package main

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

const videoANPRGroupContract = "forensics.video-anpr-plate-group/v1"

var forensicUUIDPattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
var forensicUUIDInTextPattern = regexp.MustCompile(`(?i)\b[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}\b`)

func videoANPRGroupedTimeline(ctx context.Context, db *pgxpool.Pool, req hybridQueryRequest) (map[string]any, error) {
	evidenceID := strings.TrimSpace(req.EvidenceID)
	if evidenceID == "" && forensicUUIDPattern.MatchString(strings.TrimSpace(req.Target)) {
		evidenceID = strings.TrimSpace(req.Target)
	}
	if !forensicUUIDPattern.MatchString(evidenceID) {
		return nil, errors.New("video_anpr_grouped_timeline requires an exact video evidence UUID")
	}
	if req.StartSeconds != nil && *req.StartSeconds < 0 || req.EndSeconds != nil && *req.EndSeconds < 0 {
		return nil, errors.New("video ANPR source-second bounds must be non-negative")
	}
	if req.StartSeconds != nil && req.EndSeconds != nil && *req.EndSeconds < *req.StartSeconds {
		return nil, errors.New("video ANPR end_seconds must be greater than or equal to start_seconds")
	}

	stateRows, err := queryRows(ctx, db, req.TenantID, `
SELECT evidence_id, current_version_id AS version_id, original_filename AS source_file,
       content_type, modality, detected_type, processing_status,
       metadata #>> '{media_processing,processor}' AS processor,
       metadata #>> '{media_processing,processor_revision}' AS processor_revision,
       coalesce(
         metadata #>> '{media_processing,metadata,result_states,video_anpr}',
         metadata #>> '{media_processing,metadata,role_results,video_anpr,state}',
         CASE WHEN coalesce((metadata #>> '{media_processing,metadata,anpr_sampled_frame_count}')::int, 0) > 0
              THEN 'EXECUTED' ELSE '' END
       ) AS anpr_result_state
FROM forensic.evidence_items
WHERE tenant_id = $1 AND collection_id = $2 AND evidence_id = $3::uuid
LIMIT 1`, req.TenantID, req.CollectionID, evidenceID)
	if err != nil {
		return nil, err
	}
	if len(stateRows) == 0 {
		return map[string]any{
			"video_anpr_grouped_timeline": []map[string]any{}, "row_count": 0,
			"evidence_id": evidenceID, "status": "not_found_or_not_authorized",
			"executive_state": "No authorized retained video matched the supplied evidence UUID.",
		}, nil
	}
	state := stateRows[0]
	if strings.ToLower(stringValueAny(state["modality"])) != "video" && !strings.HasPrefix(strings.ToLower(stringValueAny(state["content_type"])), "video/") {
		return nil, errors.New("the supplied evidence UUID is not retained video evidence")
	}

	groups, err := queryRows(ctx, db, req.TenantID, `
SELECT groups.artifact_id, groups.evidence_id, groups.version_id,
       evidence.original_filename AS source_file,
       groups.metadata #>> '{observation,normalized_plate_text}' AS normalized_plate_text,
       groups.metadata #>> '{observation,normalized_plate_text}' AS group_selected_plate_text,
       'selected_group_candidate' AS match_kind,
       groups.metadata #> '{observation,all_observation_ids}' AS all_observation_ids,
       groups.metadata #>> '{observation,best_observation_id}' AS best_observation_id,
       (groups.metadata #>> '{observation,sightings_count}')::int AS sightings_count,
       (groups.metadata #>> '{observation,first_seen_seconds}')::double precision AS first_seen_seconds,
       (groups.metadata #>> '{observation,last_seen_seconds}')::double precision AS last_seen_seconds,
       groups.confidence AS best_confidence,
       groups.metadata #>> '{observation,processor}' AS grouping_processor,
       groups.metadata #>> '{observation,processor_revision}' AS grouping_revision,
       groups.metadata #>> '{observation,grouping_semantics}' AS grouping_semantics,
       coalesce((groups.metadata #>> '{observation,manual_review_required}')::boolean, true) AS manual_review_required,
       groups.citation_locator AS group_locator,
       best.citation_locator AS best_observation_locator,
       best.metadata #> '{observation,bbox}' AS best_bbox,
       coalesce(best.metadata #> '{observation,crop}', best.metadata->'crop') AS best_crop,
       best.metadata #>> '{observation,raw_plate_text}' AS best_raw_plate_text,
       best.metadata #>> '{observation,processor}' AS observation_processor,
       best.metadata #>> '{observation,processor_revision}' AS observation_revision,
       groups.warnings,
       'nexusai://evidence/' || groups.evidence_id::text || '/artifacts/' || groups.artifact_id::text AS citation_ref
FROM forensic.derived_artifacts groups
JOIN forensic.evidence_items evidence
  ON evidence.tenant_id = groups.tenant_id AND evidence.collection_id = groups.collection_id
 AND evidence.evidence_id = groups.evidence_id
 AND evidence.current_version_id = groups.version_id
LEFT JOIN forensic.derived_artifacts best
  ON best.tenant_id = groups.tenant_id AND best.collection_id = groups.collection_id
 AND best.evidence_id = groups.evidence_id
 AND best.version_id = groups.version_id
 AND best.processing_status = 'completed'
 AND best.artifact_id::text = groups.metadata #>> '{observation,best_observation_id}'
WHERE groups.tenant_id = $1 AND groups.collection_id = $2
  AND groups.evidence_id = $3::uuid
  AND groups.artifact_type = $4 AND groups.processing_status = 'completed'
  AND ($5 = '' OR groups.metadata #>> '{observation,normalized_plate_text}' = regexp_replace(upper($5), '[^[:alnum:]]', '', 'g'))
  AND ($6::double precision IS NULL OR (groups.metadata #>> '{observation,last_seen_seconds}')::double precision >= $6)
  AND ($7::double precision IS NULL OR (groups.metadata #>> '{observation,first_seen_seconds}')::double precision <= $7)
ORDER BY first_seen_seconds, normalized_plate_text, groups.artifact_id
LIMIT $8`, req.TenantID, req.CollectionID, evidenceID, videoANPRGroupContract, strings.TrimSpace(req.Plate), req.StartSeconds, req.EndSeconds, req.Limit)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(req.Plate) != "" {
		readings, err := queryRows(ctx, db, req.TenantID, videoANPRExactReadingsSQL,
			req.TenantID, req.CollectionID, evidenceID, req.Plate, req.StartSeconds, req.EndSeconds, req.Limit)
		if err != nil {
			return nil, err
		}
		// Raw readings take precedence over a second display row for their group,
		// without changing that group's selected candidate or its confidence.
		represented := map[string]bool{}
		for _, reading := range readings {
			represented[stringValueAny(reading["group_artifact_id"])] = true
		}
		for _, group := range groups {
			if !represented[stringValueAny(group["artifact_id"])] {
				readings = append(readings, group)
			}
		}
		sort.SliceStable(readings, func(i, j int) bool {
			left, right := numericFloat(readings[i]["first_seen_seconds"]), numericFloat(readings[j]["first_seen_seconds"])
			if left == right {
				return stringValueAny(readings[i]["artifact_id"]) < stringValueAny(readings[j]["artifact_id"])
			}
			return left < right
		})
		if len(readings) > req.Limit {
			readings = readings[:req.Limit]
		}
		groups = readings
	}
	return videoANPRGroupedTimelineRecords(evidenceID, state, groups, req), nil
}

func videoANPRGroupedTimelineRecords(evidenceID string, state map[string]any, groups []map[string]any, req hybridQueryRequest) map[string]any {
	processingStatus := strings.ToLower(stringValueAny(state["processing_status"]))
	anprResultState := strings.ToUpper(strings.TrimSpace(stringValueAny(state["anpr_result_state"])))
	status, executiveState := videoANPROutcome(processingStatus, anprResultState, len(groups))
	if (status == "complete_zero" || status == "complete_results_without_groups") && videoANPRHasExplicitFilter(req) {
		status = "no_match_for_filter"
		switch {
		case strings.TrimSpace(req.Plate) != "" && (req.StartSeconds != nil || req.EndSeconds != nil):
			executiveState = fmt.Sprintf("ANPR processing complete; 0 plate groups matched plate filter %s within the requested source-time bounds.", strings.TrimSpace(req.Plate))
		case strings.TrimSpace(req.Plate) != "":
			executiveState = fmt.Sprintf("ANPR processing complete; 0 plate groups matched filter %s.", strings.TrimSpace(req.Plate))
		default:
			executiveState = "ANPR processing complete; 0 plate groups matched the requested source-time bounds."
		}
	}
	if len(groups) > 0 && strings.TrimSpace(req.Plate) != "" {
		executiveState = fmt.Sprintf("%s appears in %d retained plate reading or selected-candidate result(s) in this video. Review the cited frames; the group-selected candidate may differ.", req.Plate, len(groups))
	}
	return map[string]any{
		"video_anpr_grouped_timeline": groups,
		"row_count":                   len(groups),
		"evidence_id":                 evidenceID,
		"version_id":                  state["version_id"],
		"source_file":                 state["source_file"],
		"processing_status":           processingStatus,
		"anpr_result_state":           anprResultState,
		"status":                      status,
		"executive_state":             executiveState,
		"plate_filter":                strings.TrimSpace(req.Plate),
		"start_seconds":               req.StartSeconds,
		"end_seconds":                 req.EndSeconds,
		"grouping_label":              "Grouped observations",
		"limitations": []string{
			"Equal normalized OCR output is grouped for presentation; this is not tracking, identity, ownership, association, or continuous movement.",
			"Model observations require analyst review and do not establish population accuracy.",
		},
	}
}

func videoANPRHasExplicitFilter(req hybridQueryRequest) bool {
	return strings.TrimSpace(req.Plate) != "" || req.StartSeconds != nil || req.EndSeconds != nil
}

func videoANPROutcome(processingStatus, anprResultState string, groupCount int) (string, string) {
	if groupCount > 0 {
		return "groups_detected", "ANPR processing complete; grouped plate observations detected."
	}
	switch strings.ToUpper(strings.TrimSpace(anprResultState)) {
	case "MODEL_REQUIRED":
		return "model_required", "Video ANPR requires an approved local model; zero groups is not an absence finding."
	case "UNAVAILABLE":
		return "processor_unavailable", "Video ANPR is unavailable; zero groups is not an absence finding."
	case "NOT_RUN", "":
		return "not_run", "Video ANPR was not run; zero groups is not an absence finding."
	case "FAILED":
		return "failed", "Video ANPR failed; zero groups is not an absence finding."
	case "PROCESSING":
		return "processing_not_complete", "Video ANPR processing is not complete; zero groups is not an absence finding."
	case "COMPLETE_ZERO_RESULTS", "EXECUTED":
		if strings.EqualFold(strings.TrimSpace(processingStatus), "completed") {
			return "complete_zero", "ANPR processing complete, 0 plate groups detected."
		}
	case "COMPLETE_RESULTS":
		return "complete_results_without_groups", "Video ANPR observations exist but no completed presentation groups are available."
	}
	if strings.EqualFold(strings.TrimSpace(processingStatus), "completed") && strings.EqualFold(anprResultState, "EXECUTED") {
		return "complete_zero", "ANPR processing complete, 0 plate groups detected."
	}
	return "processing_not_complete", "Video ANPR processing is not complete; zero groups is not an absence finding."
}
