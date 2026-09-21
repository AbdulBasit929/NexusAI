package main

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/mudler/LocalAI/pkg/forensictext"
)

func bindDerivedTextQuery(req hybridQueryRequest) hybridQueryRequest {
	extracted, family := forensictext.Extract(req.Query)
	if req.TextQuery == nil && req.TranscriptMode == "" && req.ExactTerm == "" && extracted != nil && (req.Template == "" || isFamilyDerivedTextTemplate(req.Template)) {
		req.TextQuery = extracted
	}
	if req.TextQuery == nil {
		if req.ExactTerm != "" && req.TranscriptMode != "exact" && req.Template != "image_ocr_search" {
			req.TextQueryError = "AMBIGUOUS_LITERAL_SEMANTIC"
		}
		return req
	}
	if req.Template == "" {
		if family == "" && req.QueryScope.Kind == string(EvidenceScopeSelected) {
			family = req.QueryScope.SourceFamily
		}
		req.Template = forensictext.Template(family)
	}
	if err := req.TextQuery.Validate(); err != nil {
		req.TextQueryError = err.Error()
	}
	switch req.TextQuery.MatchSemantic {
	case forensictext.Source:
		req.TranscriptMode = "source"
	case forensictext.SourceTime:
		req.TranscriptMode = "source_time"
	case forensictext.TimeRange:
		req.TranscriptMode = "time_range"
	}
	if req.TextQuery.MatchSemantic == forensictext.TimeRange && req.StartSeconds == nil && req.EndSeconds == nil {
		req.TextQueryError = "SOURCE_TIME_REQUIRED"
	}
	if req.Template != "" && !isFamilyDerivedTextTemplate(req.Template) {
		req.TextQueryError = "UNSUPPORTED_TEXT_OPERATION"
	}
	if req.TextQuery.Representation == "roman_derivative" && req.Template != "audio_transcript_search" {
		req.TextQueryError = "UNSUPPORTED_TEXT_REPRESENTATION"
	}
	return req
}

func derivedTextLiteral(req hybridQueryRequest) string {
	if req.TextQuery != nil {
		return req.TextQuery.LiteralText
	}
	return strings.TrimSpace(req.ExactTerm)
}

func derivedTextRowAllowed(row map[string]any, req hybridQueryRequest) bool {
	if kind := stringValueAny(row["artifact_type"]); kind != "" && isFamilyDerivedTextTemplate(req.Template) && !containsString(derivedTextContractsForTemplate(req.Template), kind) {
		return false
	}
	for key, wanted := range map[string]string{"tenant_id": req.TenantID, "collection_id": req.CollectionID, "evidence_id": req.EvidenceID, "version_id": req.EvidenceVersionID} {
		value := stringValueAny(row[key])
		if wanted != "" && (key == "evidence_id" || key == "version_id" || value != "") && value != wanted {
			return false
		}
	}
	if req.TextQuery != nil {
		roman := stringValueAny(row["artifact_type"]) == "forensics.audio-roman-urdu-segment/v1"
		if req.TextQuery.Representation == "raw" && roman || req.TextQuery.Representation == "roman_derivative" && !roman {
			return false
		}
	}
	if documentComparisonRequested(req) && !derivedTextComparisonSourceAllowed(row, req.SourceSet) {
		return false
	}
	return true
}

func derivedTextComparisonSourceAllowed(row map[string]any, sourceSet *StructuredSourceSetV1) bool {
	if sourceSet == nil {
		return false
	}
	evidenceID := strings.TrimSpace(stringValueAny(row["evidence_id"]))
	versionID := strings.TrimSpace(stringValueAny(row["version_id"]))
	sourceFile := strings.TrimSpace(stringValueAny(row["source_file"]))
	for _, source := range sourceSet.Sources {
		if source.EvidenceID != "" {
			if strings.EqualFold(evidenceID, source.EvidenceID) && strings.EqualFold(versionID, source.VersionID) {
				return true
			}
			continue
		}
		if source.SourceFile != "" && strings.EqualFold(sourceFile, source.SourceFile) {
			return true
		}
	}
	return false
}

// Extend the existing tool-result validator with text ownership and match
// proofs. A model cannot repair a packet rejected here.
func (r ToolResultV1) validateTextResult(collectionID string) error {
	if r.RowCount != len(r.Rows) {
		return fmt.Errorf("text row accounting inconsistent")
	}
	if err := r.TextQuery.Validate(); err != nil {
		return err
	}
	if r.TextScope == nil || r.TextScope.CollectionID != collectionID || r.TenantID == "" {
		return fmt.Errorf("text result scope is missing")
	}
	if r.RowCount == 0 && r.ResultState == ResultStateResultsPresent {
		return fmt.Errorf("text results-present packet has no observations")
	}
	if !isFamilyDerivedTextTemplate(queryTemplateNameByOperationID(r.OperationID)) {
		return fmt.Errorf("text result operation mismatch")
	}
	req := hybridQueryRequest{Template: queryTemplateNameByOperationID(r.OperationID), TenantID: r.TenantID, CollectionID: collectionID, EvidenceID: r.TextScope.EvidenceID, EvidenceVersionID: r.TextScope.EvidenceVersionID, TextQuery: r.TextQuery}
	if r.TextTimeScope != nil {
		if err := r.TextTimeScope.Validate(); err != nil {
			return err
		}
		req.StartSeconds = r.TextTimeScope.StartSeconds
		req.EndSeconds = r.TextTimeScope.EndSeconds
	}
	citations := map[string]CitationV1{}
	for _, c := range r.Citations {
		if c.TenantID != r.TenantID || c.CollectionID != collectionID {
			return fmt.Errorf("text citation outside scope")
		}
		citations[c.ArtifactID] = c
	}
	for _, row := range r.Rows {
		m, ok := row["metadata"].(map[string]any)
		if !ok {
			return fmt.Errorf("text observation metadata missing")
		}
		if !derivedTextRowAllowed(m, req) || stringValueAny(m["match_semantic"]) != string(r.TextQuery.MatchSemantic) {
			return fmt.Errorf("text observation scope or semantic mismatch")
		}
		supports := mapsFromAny(m["contributing_observations"])
		if len(supports) == 0 {
			supports = []map[string]any{{"artifact_id": m["artifact_id"], "artifact_type": m["artifact_type"], "tenant_id": m["tenant_id"], "collection_id": m["collection_id"], "evidence_id": m["evidence_id"], "version_id": m["version_id"], "run_id": m["run_id"], "passage_text": m["raw_text"], "citation_locator": m["citation_locator"]}}
		}
		if len(supports) > 4 {
			return fmt.Errorf("text window exceeds segment budget")
		}
		texts := []string{}
		for i, s := range supports {
			if !derivedTextRowAllowed(s, req) {
				return fmt.Errorf("text support outside scope")
			}
			for _, key := range []string{"evidence_id", "version_id", "run_id", "artifact_type"} {
				if stringValueAny(s[key]) == "" || stringValueAny(s[key]) != stringValueAny(m[key]) {
					return fmt.Errorf("text support lineage mismatch")
				}
			}
			c, ok := citations[stringValueAny(s["artifact_id"])]
			if !ok || c.EvidenceID != stringValueAny(s["evidence_id"]) || c.VersionID != stringValueAny(s["version_id"]) || c.RunID != stringValueAny(s["run_id"]) || c.Locator == "" {
				return fmt.Errorf("text support citation missing or mismatched")
			}
			if i > 0 {
				_, previousEnd, previousOK := segmentTimes(supports[i-1])
				start, _, currentOK := segmentTimes(s)
				if !previousOK || !currentOK || math.Abs(previousEnd-start) > 0.000001 {
					return fmt.Errorf("text support adjacency unproven")
				}
			}
			if (req.StartSeconds != nil || req.EndSeconds != nil) && !transcriptRowIntersects(s, req.StartSeconds, req.EndSeconds) {
				return fmt.Errorf("text support outside source-time filter")
			}
			texts = append(texts, stringValueAny(s["passage_text"]))
		}
		if r.TextQuery.MatchSemantic != forensictext.TimeRange && !forensictext.Match(strings.Join(texts, " "), *r.TextQuery) {
			return fmt.Errorf("text support does not satisfy literal semantic")
		}
	}
	return nil
}

func derivedTextMatches(text string, req hybridQueryRequest) bool {
	if req.TextQuery != nil {
		return forensictext.Match(text, *req.TextQuery)
	}
	return containsExactNormalizedTranscript(text, req.ExactTerm)
}

func segmentTimes(row map[string]any) (float64, float64, bool) {
	loc, _ := row["citation_locator"].(map[string]any)
	a, okA := transcriptSeconds(loc["start_seconds"])
	b, okB := transcriptSeconds(loc["end_seconds"])
	return a, b, okA && okB && !math.IsNaN(a) && !math.IsNaN(b) && !math.IsInf(a, 0) && !math.IsInf(b, 0) && b > a
}

// The current worker persists source times, not a usable ordinal. Only touching,
// non-overlapping ranges establish adjacency; UUIDs and creation time never do.
func derivedTextWindows(rows []map[string]any, req hybridQueryRequest) (map[string][]map[string]any, bool) {
	matches := map[string][]map[string]any{}
	mode := normalizeDerivedTextMode(req)
	if req.Template != "audio_transcript_search" || mode != "PHRASE_CONTAINS" && mode != "EXACT_PHRASE" && mode != "exact" {
		return matches, false
	}
	groups := map[string][]map[string]any{}
	uncertain := false
	for _, row := range rows {
		if !derivedTextRowAllowed(row, req) || (req.StartSeconds != nil || req.EndSeconds != nil) && !transcriptRowIntersects(row, req.StartSeconds, req.EndSeconds) {
			continue
		}
		keys := []string{}
		valid := true
		for _, field := range []string{"tenant_id", "collection_id", "evidence_id", "version_id", "run_id", "artifact_type"} {
			v := stringValueAny(row[field])
			if v == "" {
				valid = false
			}
			keys = append(keys, v)
		}
		keys = append(keys, stringValueAny(row["language_lineage"]))
		if !valid {
			uncertain = req.TextQuery != nil || uncertain
			continue
		}
		groups[strings.Join(keys, "\x00")] = append(groups[strings.Join(keys, "\x00")], row)
	}
	for _, group := range groups {
		if len(group) < 2 {
			continue
		}
		timed := []map[string]any{}
		for _, row := range group {
			if _, _, ok := segmentTimes(row); ok {
				timed = append(timed, row)
			} else {
				uncertain = true
			}
		}
		if len(timed) != len(group) {
			continue
		}
		sort.Slice(timed, func(i, j int) bool {
			a, _, _ := segmentTimes(timed[i])
			b, _, _ := segmentTimes(timed[j])
			return a < b
		})
		blocked := map[int]bool{}
		for i := 1; i < len(timed); i++ {
			_, prevEnd, _ := segmentTimes(timed[i-1])
			start, _, _ := segmentTimes(timed[i])
			if prevEnd > start {
				blocked[i-1] = true
				blocked[i] = true
				uncertain = true
			}
		}
		for i := 0; i < len(timed); i++ {
			text := stringValueAny(timed[i]["passage_text"])
			for j := i + 1; j < len(timed) && j < i+4; j++ {
				if blocked[j-1] || blocked[j] {
					break
				}
				_, end, _ := segmentTimes(timed[j-1])
				start, _, _ := segmentTimes(timed[j])
				if math.Abs(end-start) > 0.000001 {
					uncertain = true
					break
				}
				prefix := text
				text += " " + stringValueAny(timed[j]["passage_text"])
				suffix := []string{}
				for k := i + 1; k <= j; k++ {
					suffix = append(suffix, stringValueAny(timed[k]["passage_text"]))
				}
				if derivedTextMatches(text, req) && !derivedTextMatches(prefix, req) && !derivedTextMatches(strings.Join(suffix, " "), req) {
					for k := i; k <= j; k++ {
						matches[stringValueAny(timed[k]["artifact_id"])] = timed[i : j+1]
					}
				}
			}
		}
	}
	return matches, uncertain
}

func applyDerivedTextCompleteness(resp *hybridQueryResponse, evidence map[string]any) {
	if value, ok := evidence["search_completeness"]; ok {
		resp.Answer["search_completeness"] = value
	}
	state := stringValueAny(evidence["result_state"])
	if state == "SEARCH_INCOMPLETE" || state == "RESULTS_TRUNCATED" {
		resp.Answer["evidence_status"] = "search_incomplete"
		resp.Answer["evidence_limitation"] = "The search is incomplete because its budget was reached or segment adjacency could not be established. Absence of additional matches has not been established."
		resp.Warnings = append(resp.Warnings, stringValueAny(resp.Answer["evidence_limitation"]))
	}
}

func derivedTranscriptResultState(req hybridQueryRequest, evidence map[string]any) string {
	state := stringValueAny(evidence["result_state"])
	if req.QueryScope.Kind == string(EvidenceScopeSelected) && len(evidenceResults(evidence)) == 0 && state != "FAILED" && state != "SEARCH_INCOMPLETE" && !queryScopeHasTranscript(req.QueryScope.AvailableResultFamilies) {
		switch normalize(req.EvidenceASRResultState) {
		case "complete_results":
			// A completed-results role state without a current-version
			// transcript artifact is an inconsistent/unavailable result,
			// never an empty successful answer.
			state = "UNAVAILABLE"
		case "complete_zero_results":
			state = "COMPLETE_ZERO_RESULTS"
		case "model_required":
			state = "MODEL_REQUIRED"
		case "unavailable":
			state = "UNAVAILABLE"
		case "failed":
			state = "FAILED"
		case "processing":
			state = "PROCESSING"
		case "not_run":
			state = "NOT_RUN"
		default:
			switch normalize(req.EvidenceProcessingState) {
			case "processing", "queued", "running":
				state = "PROCESSING"
			case "failed", "dead_letter", "error":
				state = "FAILED"
			default:
				state = "NOT_RUN"
			}
		}
		evidence["result_state"] = state
	}
	return state
}
