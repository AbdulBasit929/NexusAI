package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

const financialTransactionSummarySQL = `
WITH scoped AS (
  SELECT r."timestamp" AS observed_at,
         coalesce(nullif(r.metadata #>> '{normalized_fields,currency}', ''), 'UNSPECIFIED') AS currency,
         coalesce(nullif(r.metadata #>> '{normalized_fields,transaction_status}', ''), 'UNSPECIFIED') AS transaction_status,
         coalesce(nullif(r.metadata #>> '{normalized_fields,amount_role}', ''), 'unspecified') AS amount_role,
         (r.metadata #>> '{normalized_fields,amount_decimal}')::numeric AS amount_decimal,
         r.source_file, r.row_number, r.row_hash,
         r.evidence_id::text AS evidence_id,
         coalesce(e.current_version_id::text, nullif(e.metadata #>> '{version_id}', '')) AS version_id
  FROM forensic.records r
  LEFT JOIN forensic.evidence_items e
    ON e.tenant_id=r.tenant_id AND e.collection_id=r.collection_id AND e.evidence_id=r.evidence_id
  WHERE r.tenant_id=$1 AND r.collection_id=$2 AND r.record_type='transaction'
    AND ($3::timestamptz IS NULL OR r."timestamp" >= $3::timestamptz)
    AND ($4::timestamptz IS NULL OR r."timestamp" < $4::timestamptz)
    AND ($5 = '' OR lower(coalesce(r.primary_target, ''))=lower($5)
         OR lower(coalesce(r.secondary_target, ''))=lower($5)
         OR lower(coalesce(r.metadata #>> '{normalized_fields,transaction_reference}', ''))=lower($5))
), grouped AS (
  SELECT currency, transaction_status, amount_role,
         count(*)::bigint AS transaction_count,
         sum(amount_decimal)::text AS total_amount_decimal,
         min(observed_at) AS first_seen, max(observed_at) AS last_seen
  FROM scoped
  GROUP BY currency, transaction_status, amount_role
)
SELECT * FROM grouped
ORDER BY transaction_count DESC, currency, transaction_status, amount_role
LIMIT $6`

const financialTransactionLineageSQL = `
WITH scoped AS (
  SELECT concat_ws('|',
           coalesce(nullif(r.metadata #>> '{normalized_fields,currency}', ''), 'UNSPECIFIED'),
           coalesce(nullif(r.metadata #>> '{normalized_fields,transaction_status}', ''), 'UNSPECIFIED'),
           coalesce(nullif(r.metadata #>> '{normalized_fields,amount_role}', ''), 'unspecified')) AS result_key,
         r.source_file, r.row_number, r.row_hash,
         coalesce(r.evidence_id::text, '') AS evidence_id,
         coalesce(e.current_version_id::text, nullif(e.metadata #>> '{version_id}', ''), '') AS version_id
  FROM forensic.records r
  LEFT JOIN forensic.evidence_items e
    ON e.tenant_id=r.tenant_id AND e.collection_id=r.collection_id AND e.evidence_id=r.evidence_id
  WHERE r.tenant_id=$1 AND r.collection_id=$2 AND r.record_type='transaction'
    AND ($3::timestamptz IS NULL OR r."timestamp" >= $3::timestamptz)
    AND ($4::timestamptz IS NULL OR r."timestamp" < $4::timestamptz)
    AND ($5 = '' OR lower(coalesce(r.primary_target, ''))=lower($5)
         OR lower(coalesce(r.secondary_target, ''))=lower($5)
         OR lower(coalesce(r.metadata #>> '{normalized_fields,transaction_reference}', ''))=lower($5))
), grouped AS (
  SELECT result_key, evidence_id, version_id, source_file,
         count(*)::bigint AS contribution_count,
         min(row_number)::bigint AS first_row,
         max(row_number)::bigint AS last_row,
         encode(digest(concat_ws('|', count(*)::text, min(row_hash), max(row_hash), min(row_number)::text, max(row_number)::text), 'sha256'), 'hex') AS row_group_digest
  FROM scoped
  GROUP BY result_key, evidence_id, version_id, source_file
), ranked AS (
  SELECT grouped.*,
         count(*) OVER (PARTITION BY result_key)::bigint AS source_group_count,
         sum(contribution_count) OVER (PARTITION BY result_key)::bigint AS total_contribution_count,
         row_number() OVER (PARTITION BY result_key ORDER BY contribution_count DESC, source_file, evidence_id, version_id) AS source_group_position
  FROM grouped
)
SELECT result_key, evidence_id, version_id, source_file, contribution_count,
       first_row, last_row, row_group_digest, source_group_count, total_contribution_count
FROM ranked
WHERE source_group_position <= 50
ORDER BY result_key, source_group_position`

func financialTransactionSummary(ctx context.Context, db *pgxpool.Pool, req hybridQueryRequest) (map[string]any, error) {
	rows, err := queryRows(ctx, db, req.TenantID, financialTransactionSummarySQL,
		req.TenantID, req.CollectionID, dateBoundArg(req.DateFrom), dateBoundArg(req.DateTo), req.Target, req.Limit)
	if err != nil {
		return nil, err
	}
	lineageRows, err := queryRows(ctx, db, req.TenantID, financialTransactionLineageSQL,
		req.TenantID, req.CollectionID, dateBoundArg(req.DateFrom), dateBoundArg(req.DateTo), req.Target)
	if err != nil {
		return nil, fmt.Errorf("load financial contribution lineage: %w", err)
	}
	return map[string]any{
		"transaction_summary": rows,
		"row_count":           len(rows),
		"target":              req.Target,
		"date_from":           req.DateFrom,
		"date_to":             req.DateTo,
		"contribution_lineage": buildContributionLineages("financial.transaction_summary", map[string]string{
			"collection_id": req.CollectionID, "target": req.Target, "date_from": req.DateFrom, "date_to": req.DateTo,
		}, lineageRows),
		"limitations": []string{
			"Amounts are totaled only within the exact supplied currency; no exchange-rate conversion or cross-currency total is inferred.",
			"Account observations do not establish ownership, beneficial control, fraud, or intent.",
		},
	}, nil
}

const accessFailedEventsSQL = `
SELECT r."timestamp" AS observed_at,
       r.metadata #>> '{normalized_fields,source_ip_raw}' AS source_ip_raw,
       r.metadata #>> '{normalized_fields,source_ip_canonical}' AS source_ip_canonical,
       r.metadata #>> '{normalized_fields,user_or_principal}' AS user_or_principal,
       r.metadata #>> '{normalized_fields,http_method}' AS http_method,
       r.metadata #>> '{normalized_fields,request_path}' AS request_path,
       nullif(r.metadata #>> '{normalized_fields,http_status}', '')::int AS http_status,
       r.metadata #>> '{normalized_fields,event_action}' AS event_action,
       r.metadata #>> '{normalized_fields,event_outcome}' AS event_outcome,
       CASE
         WHEN nullif(r.metadata #>> '{normalized_fields,http_status}', '')::int >= 400 THEN 'http_status_4xx_5xx'
         ELSE 'explicit_failure_outcome'
       END AS failure_basis,
       r.evidence_id::text AS evidence_id,
       coalesce(e.current_version_id::text, nullif(e.metadata #>> '{version_id}', '')) AS version_id,
       r.source_file, r.row_number, r.row_hash,
       jsonb_strip_nulls(jsonb_build_object(
         'evidence_id', r.evidence_id, 'version_id', coalesce(e.current_version_id::text, nullif(e.metadata #>> '{version_id}', '')),
         'source_file', r.source_file, 'row_number', r.row_number, 'row_hash', r.row_hash,
         'observed_at', r."timestamp")) AS citation_locator
FROM forensic.records r
LEFT JOIN forensic.evidence_items e
  ON e.tenant_id=r.tenant_id AND e.collection_id=r.collection_id AND e.evidence_id=r.evidence_id
WHERE r.tenant_id=$1 AND r.collection_id=$2 AND r.record_type='access_log'
  AND ($3::timestamptz IS NULL OR r."timestamp" >= $3::timestamptz)
  AND ($4::timestamptz IS NULL OR r."timestamp" < $4::timestamptz)
  AND ($5 = '' OR lower(coalesce(r.metadata #>> '{normalized_fields,source_ip_raw}', ''))=lower($5)
       OR lower(coalesce(r.metadata #>> '{normalized_fields,source_ip_canonical}', ''))=lower($5)
       OR lower(coalesce(r.metadata #>> '{normalized_fields,user_or_principal}', ''))=lower($5)
       OR lower(coalesce(r.metadata #>> '{normalized_fields,request_path}', ''))=lower($5))
  AND (
    nullif(r.metadata #>> '{normalized_fields,http_status}', '')::int >= 400
    OR lower(coalesce(r.metadata #>> '{normalized_fields,event_outcome}', '')) IN ('fail','failed','failure','denied','blocked','error','rejected')
  )
ORDER BY r."timestamp", r.source_file, r.row_number, r.row_hash
LIMIT $6`

func accessFailedEvents(ctx context.Context, db *pgxpool.Pool, req hybridQueryRequest) (map[string]any, error) {
	rows, err := queryRows(ctx, db, req.TenantID, accessFailedEventsSQL,
		req.TenantID, req.CollectionID, dateBoundArg(req.DateFrom), dateBoundArg(req.DateTo), req.Target, req.Limit)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"failed_access_events": rows, "row_count": len(rows), "target": req.Target,
		"date_from": req.DateFrom, "date_to": req.DateTo,
		"limitations": []string{
			"A failed HTTP status or explicit failure outcome is a source event, not proof of compromise, malicious intent, or user identity.",
		},
	}, nil
}

func genericFilteredRecords(ctx context.Context, db *pgxpool.Pool, req hybridQueryRequest) (map[string]any, error) {
	req.RecordType = "generic"
	result, err := canonicalRecords(ctx, db, req)
	if err != nil {
		return nil, err
	}
	result["limitations"] = []string{
		"Generic rows preserve source fields and provenance but do not acquire family-specific semantics merely because a field name looks familiar.",
	}
	return result, nil
}

const familyEvidenceMetadataSQL = `
SELECT items.evidence_id::text AS evidence_id,
       coalesce(items.current_version_id::text, nullif(items.metadata #>> '{version_id}', '')) AS version_id,
       items.original_filename AS source_file,
       items.modality, items.detected_type, items.content_type, items.extension,
       items.size_bytes, items.processing_status,
       count(artifacts.artifact_id) FILTER (WHERE artifacts.processing_status='completed') AS completed_artifact_count,
       string_agg(DISTINCT artifacts.artifact_type, ', ' ORDER BY artifacts.artifact_type)
         FILTER (WHERE artifacts.processing_status='completed') AS completed_artifact_types,
       items.created_at, items.updated_at,
       jsonb_strip_nulls(jsonb_build_object(
         'evidence_id', items.evidence_id,
         'version_id', coalesce(items.current_version_id::text, nullif(items.metadata #>> '{version_id}', '')),
         'source_file', items.original_filename)) AS citation_locator
FROM forensic.evidence_items items
LEFT JOIN forensic.derived_artifacts artifacts
  ON artifacts.tenant_id=items.tenant_id AND artifacts.collection_id=items.collection_id
 AND artifacts.evidence_id=items.evidence_id
WHERE items.tenant_id=$1 AND items.collection_id=$2
  AND items.modality = ANY($3::text[])
  AND ($4 = '' OR items.evidence_id::text=$4 OR lower(items.original_filename)=lower($4))
GROUP BY items.evidence_id, items.current_version_id, items.metadata, items.original_filename,
         items.modality, items.detected_type, items.content_type, items.extension,
         items.size_bytes, items.processing_status, items.created_at, items.updated_at
ORDER BY items.updated_at DESC, items.original_filename, items.evidence_id
LIMIT $5`

func familyEvidenceMetadata(ctx context.Context, db *pgxpool.Pool, req hybridQueryRequest, family string) (map[string]any, error) {
	modalities := map[string][]string{
		"document": {"document", "text"},
		"image":    {"image"},
		"audio":    {"audio"},
		"video":    {"video"},
	}[family]
	if len(modalities) == 0 {
		return nil, fmt.Errorf("unsupported evidence metadata family %q", family)
	}
	rows, err := queryRows(ctx, db, req.TenantID, familyEvidenceMetadataSQL,
		req.TenantID, req.CollectionID, modalities, firstNonempty(req.EvidenceID, req.Target), req.Limit)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		family + "_metadata": rows, "row_count": len(rows), "target": firstNonempty(req.EvidenceID, req.Target),
		"limitations": []string{"Metadata reports retained registry and completed-artifact state; it does not imply that every family processor ran or that an absent observation is a negative real-world finding."},
	}, nil
}

func firstNonempty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

var videoTimelineArtifactContracts = []string{
	"forensics.video-observation/v1",
	"forensics.video-anpr-plate-group/v1",
	"forensics.image-observation/v1",
	"forensics.image-ocr-observation/v1",
	"forensics.anpr-observation/v1",
	"forensics.face-observation/v1",
	"forensics.audio-timestamp-segment/v1",
	"forensics.audio-roman-urdu-segment/v1",
}

const videoTimelineSQL = `
SELECT artifacts.artifact_id::text AS artifact_id,
       artifacts.artifact_type,
       artifacts.evidence_id::text AS evidence_id,
       artifacts.version_id::text AS version_id,
       items.original_filename AS source_file,
       artifacts.processing_status,
       artifacts.confidence,
       coalesce(
         nullif(artifacts.citation_locator->>'timestamp_seconds', '')::double precision,
         nullif(artifacts.citation_locator->>'start_seconds', '')::double precision,
         nullif(artifacts.metadata #>> '{observation,frame_timestamp_seconds}', '')::double precision,
         nullif(artifacts.metadata #>> '{observation,first_seen_seconds}', '')::double precision,
         0
       ) AS source_start_seconds,
       coalesce(
         nullif(artifacts.citation_locator->>'end_seconds', '')::double precision,
         nullif(artifacts.metadata #>> '{observation,last_seen_seconds}', '')::double precision,
         nullif(artifacts.citation_locator->>'timestamp_seconds', '')::double precision,
         nullif(artifacts.citation_locator->>'start_seconds', '')::double precision,
         0
       ) AS source_end_seconds,
       artifacts.citation_locator,
       artifacts.metadata->'observation' AS observation,
       artifacts.warnings,
       'nexusai://evidence/' || artifacts.evidence_id::text || '/artifacts/' || artifacts.artifact_id::text AS citation
FROM forensic.derived_artifacts artifacts
JOIN forensic.evidence_items items
  ON items.tenant_id=artifacts.tenant_id AND items.collection_id=artifacts.collection_id
 AND items.evidence_id=artifacts.evidence_id
WHERE artifacts.tenant_id=$1 AND artifacts.collection_id=$2
  AND artifacts.evidence_id=$3::uuid
  AND artifacts.processing_status='completed'
  AND artifacts.artifact_type = ANY($4::text[])
  AND ($5::double precision IS NULL OR coalesce(
        nullif(artifacts.citation_locator->>'end_seconds', '')::double precision,
        nullif(artifacts.metadata #>> '{observation,last_seen_seconds}', '')::double precision,
        nullif(artifacts.citation_locator->>'timestamp_seconds', '')::double precision, 0) >= $5)
  AND ($6::double precision IS NULL OR coalesce(
        nullif(artifacts.citation_locator->>'timestamp_seconds', '')::double precision,
        nullif(artifacts.citation_locator->>'start_seconds', '')::double precision,
        nullif(artifacts.metadata #>> '{observation,frame_timestamp_seconds}', '')::double precision,
        nullif(artifacts.metadata #>> '{observation,first_seen_seconds}', '')::double precision, 0) <= $6)
ORDER BY source_start_seconds, source_end_seconds, artifacts.artifact_type, artifacts.artifact_id
LIMIT $7`

func videoObservationTimeline(ctx context.Context, db *pgxpool.Pool, req hybridQueryRequest) (map[string]any, error) {
	evidenceID := firstNonempty(req.EvidenceID, req.Target)
	if !forensicUUIDPattern.MatchString(evidenceID) {
		return nil, errors.New("video timeline requires an exact video evidence UUID")
	}
	if req.StartSeconds != nil && *req.StartSeconds < 0 || req.EndSeconds != nil && *req.EndSeconds < 0 {
		return nil, errors.New("video timeline source-second bounds must be non-negative")
	}
	if req.StartSeconds != nil && req.EndSeconds != nil && *req.EndSeconds < *req.StartSeconds {
		return nil, errors.New("video timeline end_seconds must be greater than or equal to start_seconds")
	}
	stateRows, err := queryRows(ctx, db, req.TenantID, `
SELECT evidence_id::text AS evidence_id,
       coalesce(current_version_id::text, nullif(metadata #>> '{version_id}', '')) AS version_id,
       original_filename AS source_file, modality, content_type, processing_status
FROM forensic.evidence_items
WHERE tenant_id=$1 AND collection_id=$2 AND evidence_id=$3::uuid
LIMIT 1`, req.TenantID, req.CollectionID, evidenceID)
	if err != nil {
		return nil, err
	}
	if len(stateRows) == 0 {
		return map[string]any{"video_timeline": []map[string]any{}, "row_count": 0, "status": "not_found_or_not_authorized", "evidence_id": evidenceID}, nil
	}
	state := stateRows[0]
	if !strings.EqualFold(stringValueAny(state["modality"]), "video") && !strings.HasPrefix(strings.ToLower(stringValueAny(state["content_type"])), "video/") {
		return nil, errors.New("the supplied evidence UUID is not retained video evidence")
	}
	rows, err := queryRows(ctx, db, req.TenantID, videoTimelineSQL,
		req.TenantID, req.CollectionID, evidenceID, videoTimelineArtifactContracts, req.StartSeconds, req.EndSeconds, req.Limit)
	if err != nil {
		return nil, err
	}
	processingStatus := strings.ToLower(stringValueAny(state["processing_status"]))
	status := "observations_present"
	if len(rows) == 0 {
		if processingStatus == "completed" {
			status = "complete_zero"
		} else {
			status = "processing_not_complete"
		}
	}
	return map[string]any{
		"video_timeline": rows, "row_count": len(rows), "status": status,
		"processing_status": processingStatus, "evidence_id": evidenceID,
		"version_id": state["version_id"], "source_file": state["source_file"],
		"start_seconds": req.StartSeconds, "end_seconds": req.EndSeconds,
		"limitations": []string{
			"The timeline contains sampled or derived observations only; it is not a continuous account of the video.",
			"OCR, ANPR, face, ASR, and visual outputs remain model observations requiring analyst review and do not prove identity, ownership, association, or intent.",
		},
	}, nil
}

const faceCandidateObservationsSQL = `
SELECT artifacts.artifact_id::text AS artifact_id,
       artifacts.artifact_type,
       artifacts.evidence_id::text AS evidence_id,
       artifacts.version_id::text AS version_id,
       items.original_filename AS source_file,
       artifacts.metadata->>'observation_id' AS observation_id,
       (artifacts.metadata->'observation') - 'embedding' AS observation,
       artifacts.citation_locator,
       artifacts.confidence,
       artifacts.created_at,
       'nexusai://evidence/' || artifacts.evidence_id::text || '/artifacts/' || artifacts.artifact_id::text AS citation
FROM forensic.derived_artifacts artifacts
JOIN forensic.evidence_items items
  ON items.tenant_id=artifacts.tenant_id AND items.collection_id=artifacts.collection_id
 AND items.evidence_id=artifacts.evidence_id AND items.current_version_id=artifacts.version_id
WHERE artifacts.tenant_id=$1 AND artifacts.collection_id=$2
  AND artifacts.evidence_id=$3::uuid
  AND artifacts.artifact_type='forensics.face-observation/v1'
  AND artifacts.processing_status='completed'
ORDER BY artifacts.created_at, artifacts.artifact_id
LIMIT $4`

func faceCandidateObservations(ctx context.Context, db *pgxpool.Pool, req hybridQueryRequest) (map[string]any, error) {
	evidenceID := firstNonempty(req.EvidenceID, req.Target)
	if !forensicUUIDPattern.MatchString(evidenceID) {
		return nil, errors.New("face candidate observations require an exact image evidence UUID")
	}
	stateRows, err := queryRows(ctx, db, req.TenantID, `
SELECT evidence_id::text AS evidence_id,
       coalesce(current_version_id::text, nullif(metadata #>> '{version_id}', '')) AS version_id,
       original_filename AS source_file, modality, content_type, processing_status
FROM forensic.evidence_items
WHERE tenant_id=$1 AND collection_id=$2 AND evidence_id=$3::uuid
LIMIT 1`, req.TenantID, req.CollectionID, evidenceID)
	if err != nil {
		return nil, err
	}
	if len(stateRows) == 0 {
		return map[string]any{"face_candidate_observations": []map[string]any{}, "row_count": 0, "status": "not_found_or_not_authorized", "evidence_id": evidenceID}, nil
	}
	state := stateRows[0]
	if !strings.EqualFold(stringValueAny(state["modality"]), "image") && !strings.HasPrefix(strings.ToLower(stringValueAny(state["content_type"])), "image/") {
		return nil, errors.New("the supplied evidence UUID is not retained image evidence")
	}
	rows, err := queryRows(ctx, db, req.TenantID, faceCandidateObservationsSQL, req.TenantID, req.CollectionID, evidenceID, req.Limit)
	if err != nil {
		return nil, err
	}
	processingStatus := strings.ToLower(stringValueAny(state["processing_status"]))
	status := "observations_present"
	if len(rows) == 0 {
		if processingStatus == "completed" {
			status = "complete_zero"
		} else {
			status = "processing_not_complete"
		}
	}
	return map[string]any{
		"face_candidate_observations": rows, "row_count": len(rows), "status": status,
		"processing_status": processingStatus, "evidence_id": evidenceID,
		"version_id": state["version_id"], "source_file": state["source_file"],
		"limitations": []string{
			"These are retained face-model candidates for analyst review, not identity determinations.",
			"No identity, name, ownership, association, intent, demographic attribute, or real-world presence is inferred.",
		},
	}, nil
}
