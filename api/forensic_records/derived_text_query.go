package main

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudler/LocalAI/pkg/forensictext"
	"golang.org/x/text/unicode/norm"
)

const derivedTextEvidenceSQL = `
SELECT artifacts.artifact_id::text, artifacts.artifact_type,
 artifacts.tenant_id, artifacts.collection_id, artifacts.run_id::text,
 artifacts.metadata#>>'{observation,language}' AS language_lineage,
 artifacts.metadata->>'observation_id' AS observation_id,
 CASE WHEN artifacts.artifact_type='forensics.image-ocr-observation/v1' AND coalesce(artifacts.metadata#>>'{observation,raw_text}', '')='' THEN true ELSE false END AS raw_text_unavailable,
       coalesce(artifacts.parent_artifact_id::text, '') AS parent_artifact_id,
       artifacts.evidence_id::text, artifacts.version_id::text,
       items.original_filename AS source_file,
       artifacts.citation_locator,
       CASE WHEN artifacts.artifact_type = 'forensics.image-ocr-observation/v1'
       THEN coalesce(nullif(artifacts.metadata#>>'{observation,raw_text}', ''),
                     nullif(artifacts.metadata#>>'{observation,normalized_text}', ''))
       ELSE coalesce(
         artifacts.metadata->>'text',
         artifacts.metadata#>>'{observation,roman_urdu_text}',
         artifacts.metadata#>>'{observation,text}',
         artifacts.metadata#>>'{observation,raw_urdu_text}'
       ) END AS passage_text
FROM forensic.derived_artifacts artifacts
JOIN forensic.evidence_items items
  ON items.tenant_id=artifacts.tenant_id
 AND items.collection_id=artifacts.collection_id
 AND items.evidence_id=artifacts.evidence_id
 AND items.current_version_id=artifacts.version_id
WHERE artifacts.tenant_id=$1 AND artifacts.collection_id=$2
  AND artifacts.processing_status='completed'
  AND artifacts.artifact_type = ANY($3::text[])
  AND ($4 = '' OR artifacts.evidence_id = $4::uuid)
  AND ($5 = '' OR artifacts.version_id = $5::uuid)
  AND (cardinality($7::text[]) = 0 OR artifacts.evidence_id::text = ANY($7::text[]))
  AND (cardinality($8::text[]) = 0 OR items.original_filename = ANY($8::text[]))
  AND (cardinality($9::text[]) = 0 OR artifacts.version_id::text = ANY($9::text[]))
ORDER BY artifacts.artifact_id
LIMIT 501 OFFSET $6`

const currentEvidenceScopeSQL = `
SELECT evidence_id::text, coalesce(current_version_id::text, '') AS current_version_id,
       modality, detected_type, processing_status,
       CASE
         WHEN metadata#>>'{media_processing,version_id}' = current_version_id::text
         THEN coalesce(metadata#>>'{media_processing,metadata,result_states,asr}', '')
         ELSE ''
       END AS asr_result_state
FROM forensic.evidence_items
WHERE tenant_id=$1 AND collection_id=$2 AND evidence_id=$3::uuid
LIMIT 1`

// GROUP BY keeps the tenant-policy Result node outside Timescale SkipScan.
// DISTINCT can select a SkipScan(Result(IndexScan)) plan under the runtime RLS role.
const currentEvidenceResultFamiliesSQL = `
SELECT artifact_type
FROM forensic.derived_artifacts
WHERE tenant_id=$1 AND collection_id=$2 AND evidence_id=$3::uuid
  AND version_id=$4::uuid
  AND processing_status='completed'
GROUP BY artifact_type
ORDER BY artifact_type`

var searchableDerivedTextContracts = []string{
	"forensics.document-native-text-passage/v1",
	"forensics.audio-timestamp-segment/v1",
	"forensics.audio-roman-urdu-segment/v1",
	"forensics.image-ocr-observation/v1",
}

var familyDerivedTextContracts = map[string][]string{
	"document_search":         {"forensics.document-native-text-passage/v1"},
	"image_ocr_search":        {"forensics.image-ocr-observation/v1"},
	"audio_transcript_search": {"forensics.audio-timestamp-segment/v1", "forensics.audio-roman-urdu-segment/v1"},
}

func derivedTextContractsForTemplate(template string) []string {
	if contracts := familyDerivedTextContracts[normalize(template)]; len(contracts) > 0 {
		return append([]string(nil), contracts...)
	}
	return append([]string(nil), searchableDerivedTextContracts...)
}

func isFamilyDerivedTextTemplate(template string) bool {
	return len(familyDerivedTextContracts[normalize(template)]) > 0
}

func derivedTextPreview(text string, max int) string {
	if len(text) <= max {
		return text
	}
	for max > 0 && !utf8.RuneStart(text[max]) {
		max--
	}
	return text[:max]
}

const derivedTextMaxPages = 20
const derivedTextPageSize = 500
const derivedTextMaxBytes = 16 << 20

func derivedTextEvidence(ctx context.Context, db *pgxpool.Pool, req hybridQueryRequest) (map[string]any, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(ctx, "SELECT set_config('app.tenant_id',$1,true)", req.TenantID); err != nil {
		return nil, err
	}
	rows := []map[string]any{}
	complete := false
	pages := 0
	bytes := 0
	selectedEvidenceIDs, selectedSourceFiles, selectedVersionIDs := derivedTextSourceSelection(req)
	for pages < derivedTextMaxPages {
		cursor, err := tx.Query(ctx, derivedTextEvidenceSQL, req.TenantID, req.CollectionID, derivedTextContractsForTemplate(req.Template), strings.TrimSpace(req.EvidenceID), strings.TrimSpace(req.EvidenceVersionID), pages*derivedTextPageSize, selectedEvidenceIDs, selectedSourceFiles, selectedVersionIDs)
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			return nil, err
		}
		page, err := rowsToMaps(cursor)
		cursor.Close()
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			return nil, err
		}
		pages++
		count := len(page)
		if count > derivedTextPageSize {
			page = page[:derivedTextPageSize]
		}
		budget := false
		for _, row := range page {
			bytes += len(stringValueAny(row["passage_text"]))
			if bytes > derivedTextMaxBytes {
				budget = true
				break
			}
			rows = append(rows, row)
		}
		if budget {
			break
		}
		if count <= derivedTextPageSize {
			complete = true
			break
		}
	}
	result := governedDerivedTextEvidence(rows, req)
	if documentComparisonRequested(req) {
		result = annotateDocumentComparison(result, req)
	}
	result["candidate_pages"] = pages
	result["candidates_inspected"] = len(rows)
	if !complete {
		result["search_completeness"] = "INCOMPLETE"
		result["result_state"] = "SEARCH_INCOMPLETE"
	}
	return result, nil
}

func annotateDocumentComparison(evidence map[string]any, req hybridQueryRequest) map[string]any {
	if req.SourceSet == nil {
		return evidence
	}
	counts := map[string]int{}
	for _, result := range evidenceResults(evidence) {
		metadata, _ := result["metadata"].(map[string]any)
		counts[strings.ToLower(stringValueAny(metadata["evidence_id"]))]++
		counts["file:"+strings.ToLower(stringValueAny(metadata["source_file"]))]++
	}
	sources := make([]map[string]any, 0, len(req.SourceSet.Sources))
	missing := []string{}
	for _, source := range req.SourceSet.Sources {
		count := counts[strings.ToLower(source.EvidenceID)]
		if source.EvidenceID == "" {
			count = counts["file:"+strings.ToLower(source.SourceFile)]
		}
		sources = append(sources, map[string]any{
			"source_id": source.SourceID, "evidence_id": source.EvidenceID,
			"version_id": source.VersionID, "source_file": source.SourceFile,
			"passages_returned": count,
		})
		if count == 0 {
			missing = append(missing, source.SourceID)
		}
	}
	evidence["document_comparison"] = map[string]any{
		"source_count": len(sources), "sources": sources,
		"all_sources_represented": len(missing) == 0, "sources_without_native_text": missing,
		"comparison_basis": "bounded_current_version_native_text_passages",
	}
	if len(missing) > 0 {
		evidence["comparison_limitation"] = "One or more selected documents returned no current-version native-text passages; scanned or unsupported content was not inferred."
	}
	return evidence
}

func derivedTextSourceSelection(req hybridQueryRequest) ([]string, []string, []string) {
	if !documentComparisonRequested(req) || req.SourceSet == nil {
		return []string{}, []string{}, []string{}
	}
	return structuredSourceEvidenceIDs(req.SourceSet), structuredSourceFiles(req.SourceSet), structuredSourceVersionIDs(req.SourceSet)
}

func governedDerivedTextEvidence(rows []map[string]any, req hybridQueryRequest) map[string]any {
	mode := normalizeDerivedTextMode(req)
	exactTerm := derivedTextLiteral(req)
	if req.TextQuery != nil {
		if err := req.TextQuery.Validate(); err != nil {
			return map[string]any{"results": []map[string]any{}, "result_state": "INVALID_REQUEST", "reason_code": err.Error(), "search_completeness": "UNKNOWN"}
		}
	}
	if mode == "invalid" {
		return map[string]any{"results": []map[string]any{}, "result_state": "INVALID_REQUEST", "reason_code": "AMBIGUOUS_LITERAL_SEMANTIC", "search_completeness": "UNKNOWN"}
	}
	windows, adjacencyUncertain := derivedTextWindows(rows, req)
	rawUnavailable := false
	terms := queryTerms(req.Query + " " + req.Target)
	results := make([]map[string]any, 0, min(len(rows), req.MaxKBResults))
	textRows, timedRows := 0, 0
	for _, row := range rows {
		if !derivedTextRowAllowed(row, req) {
			continue
		}
		if req.EvidenceID != "" && !strings.EqualFold(strings.TrimSpace(stringValueAny(row["evidence_id"])), req.EvidenceID) {
			continue
		}
		if req.EvidenceVersionID != "" && !strings.EqualFold(strings.TrimSpace(stringValueAny(row["version_id"])), req.EvidenceVersionID) {
			continue
		}
		if req.TextQuery != nil && row["raw_text_unavailable"] == true {
			rawUnavailable = true
			continue
		}
		text := strings.TrimSpace(stringValueAny(row["passage_text"]))
		if text == "" {
			continue
		}
		textRows++
		if transcriptRowIntersects(row, nil, nil) {
			timedRows++
		}
		score := lexicalScore(strings.ToLower(text), terms)
		switch mode {
		case "exact", "EXACT_VALUE", "EXACT_PHRASE", "PHRASE_CONTAINS", "TOKEN_SEARCH":
			if !derivedTextMatches(text, req) && len(windows[stringValueAny(row["artifact_id"])]) == 0 {
				continue
			}
			score = 1
		case "time_range", "TIME_RANGE":
			if !transcriptRowIntersects(row, req.StartSeconds, req.EndSeconds) {
				continue
			}
			score = 1
		case "source_time", "SOURCE_TIME":
			score = 1
		case "SOURCE":
			// Uppercase SOURCE is forensictext's own deliberate classification
			// (bindDerivedTextQuery/bindSemanticRetrievalStrategy) that this
			// request wants an exhaustive, unfiltered browse — e.g. explicit
			// multi-document comparison across a supplied source set. Keep
			// that exhaustive, unfiltered.
			score = 1
		default:
			// Lowercase "source" is different: normalizeTranscriptMode's own
			// blind fallback once nothing else (no forensictext extraction, no
			// exact-term/time-range pattern) classified the request at all.
			// It used to force score=1 unconditionally too, meaning a natural
			// question that didn't happen to match one of the rigid exact-term
			// regexes (e.g. "find OCR text mentioning X" without quotes or "in
			// this image") returned an ARBITRARY unfiltered candidate dressed
			// up as a match, with no actual relevance to the analyst's words.
			// Applying the same term-presence filter as every other mode here
			// restores real lexical relevance as the default for document,
			// image OCR, and audio transcript search alike. A genuine "browse
			// everything" question (no meaningful terms once stopwords are
			// removed) is unaffected — the filter only applies when
			// len(terms) > 0.
			// Relevance uses word boundaries and requires every identifier
			// in the question; see derivedTextRelevance for why substring
			// counting was replaced.
			relevance, relevant := derivedTextRelevance(text, req.Query+" "+req.Target)
			if !relevant {
				continue
			}
			score = relevance
		}
		if req.TextQuery != nil && (req.StartSeconds != nil || req.EndSeconds != nil) && !transcriptRowIntersects(row, req.StartSeconds, req.EndSeconds) {
			continue
		}
		locator := row["citation_locator"]
		locatorText := ""
		if payload, marshalErr := json.Marshal(locator); marshalErr == nil {
			locatorText = string(payload)
		}
		artifactID := strings.TrimSpace(stringValueAny(row["artifact_id"]))
		metadata := map[string]any{
			"tenant_id": req.TenantID, "collection_id": req.CollectionID,
			"evidence_id": row["evidence_id"], "version_id": row["version_id"],
			"source_file": row["source_file"], "source_locator": locatorText,
			"citation_locator": locator, "source_family": derivedTextSourceFamily(row["artifact_type"]),
			"artifact_id": artifactID, "artifact_type": row["artifact_type"],
			"parent_artifact_id": row["parent_artifact_id"],
		}
		metadata["raw_text"] = stringValueAny(row["passage_text"])
		metadata["observation_id"] = row["observation_id"]
		metadata["run_id"] = row["run_id"]
		metadata["match_semantic"] = mode
		metadata["text_representation"] = "raw"
		if row["raw_text_unavailable"] == true {
			metadata["raw_text"] = ""
			metadata["search_text"] = text
			metadata["text_representation"] = "ocr_normalized_fallback"
		}
		if stringValueAny(row["artifact_type"]) == "forensics.audio-roman-urdu-segment/v1" {
			metadata["text_representation"] = "roman_derivative"
		}
		if req.TextQuery != nil {
			metadata["search_normalization"] = forensictext.NormalizationV1
		} else if mode == "exact" {
			metadata["search_normalization"] = "nfkc-lower-punctuation-space-token-boundary/v0"
		}
		if group := windows[artifactID]; len(group) > 0 {
			metadata["cross_segment"] = true
			metadata["contributing_observations"] = group
			start, _, _ := segmentTimes(group[0])
			_, end, _ := segmentTimes(group[len(group)-1])
			metadata["match_start_seconds"] = start
			metadata["match_end_seconds"] = end
		}
		if locatorMap, ok := locator.(map[string]any); ok {
			metadata["start_seconds"] = locatorMap["start_seconds"]
			metadata["end_seconds"] = locatorMap["end_seconds"]
		}
		results = append(results, map[string]any{
			"id":       artifactID,
			"content":  derivedTextPreview(text, 2000),
			"preview":  derivedTextPreview(text, 2000),
			"score":    score,
			"citation": fmt.Sprintf("nexusai://evidence/%s/artifacts/%s", row["evidence_id"], artifactID),
			"metadata": metadata,
		})
	}
	sort.SliceStable(results, func(i, j int) bool {
		leftMeta, _ := results[i]["metadata"].(map[string]any)
		rightMeta, _ := results[j]["metadata"].(map[string]any)
		leftRaw := stringValueAny(leftMeta["artifact_type"]) == "forensics.audio-timestamp-segment/v1"
		rightRaw := stringValueAny(rightMeta["artifact_type"]) == "forensics.audio-timestamp-segment/v1"
		if leftRaw != rightRaw {
			return leftRaw
		}
		if leftMeta["evidence_id"] == rightMeta["evidence_id"] && leftMeta["run_id"] == rightMeta["run_id"] {
			leftStart, lok := transcriptSeconds(leftMeta["start_seconds"])
			rightStart, rok := transcriptSeconds(rightMeta["start_seconds"])
			if lok && rok && leftStart != rightStart {
				return leftStart < rightStart
			}
		}
		left, _ := results[i]["score"].(int)
		right, _ := results[j]["score"].(int)
		return left > right
	})
	truncated := false
	if req.MaxKBResults > 0 && len(results) > req.MaxKBResults {
		selected := selectDerivedTextResults(results, windows, req)
		truncated = len(selected) < len(results)
		results = selected
	}
	state := "COMPLETE_RESULTS"
	if len(results) == 0 {
		switch {
		case mode == "exact" || mode == "EXACT_VALUE" || mode == "EXACT_PHRASE" || mode == "PHRASE_CONTAINS" || mode == "TOKEN_SEARCH":
			state = "NO_EXACT_MATCH"
		case (mode == "time_range" || mode == "TIME_RANGE") && textRows > 0 && timedRows == 0:
			state = "TIMING_UNAVAILABLE"
		case mode == "time_range" || mode == "TIME_RANGE":
			state = "NO_MATCH"
		case textRows == 0:
			state = "COMPLETE_ZERO_RESULTS"
		default:
			state = "NO_MATCH"
		}
	}
	completeness := "EXHAUSTIVE"
	if adjacencyUncertain || rawUnavailable {
		completeness = "INCOMPLETE"
		state = "SEARCH_INCOMPLETE"
	}
	if truncated {
		completeness = "TRUNCATED"
		state = "RESULTS_TRUNCATED"
	}
	return map[string]any{"candidate_scope": "completed_current_version_observations", "maximum_segment_window": 4, "search_completeness": completeness, "row_count": len(results), "mode": "governed_derived_text_" + mode, "query_mode": mode, "exact_term": exactTerm, "result_state": state, "results": results}
}

func selectDerivedTextResults(results []map[string]any, windows map[string][]map[string]any, req hybridQueryRequest) []map[string]any {
	if documentComparisonRequested(req) && req.SourceSet != nil {
		groups := make([][]map[string]any, len(req.SourceSet.Sources))
		for _, result := range results {
			metadata, _ := result["metadata"].(map[string]any)
			for index, source := range req.SourceSet.Sources {
				if source.EvidenceID != "" && strings.EqualFold(stringValueAny(metadata["evidence_id"]), source.EvidenceID) ||
					source.EvidenceID == "" && strings.EqualFold(stringValueAny(metadata["source_file"]), source.SourceFile) {
					groups[index] = append(groups[index], result)
					break
				}
			}
		}
		selected := make([]map[string]any, 0, req.MaxKBResults)
		for offset := 0; len(selected) < req.MaxKBResults; offset++ {
			added := false
			for _, group := range groups {
				if offset < len(group) {
					selected = append(selected, group[offset])
					added = true
					if len(selected) == req.MaxKBResults {
						break
					}
				}
			}
			if !added {
				break
			}
		}
		return selected
	}
	keep := map[string]bool{}
	for _, result := range results[:req.MaxKBResults] {
		id := stringValueAny(result["id"])
		keep[id] = true
		for _, row := range windows[id] {
			keep[stringValueAny(row["artifact_id"])] = true
		}
	}
	selected := []map[string]any{}
	for _, result := range results {
		if keep[stringValueAny(result["id"])] {
			selected = append(selected, result)
		}
	}
	return selected
}

func normalizeDerivedTextMode(req hybridQueryRequest) string {
	if req.TextQuery != nil {
		return string(req.TextQuery.MatchSemantic)
	}
	if req.ExactTerm != "" && req.TranscriptMode != "exact" && req.Template != "image_ocr_search" {
		return "invalid"
	}
	if req.Template == "image_ocr_search" && strings.TrimSpace(req.ExactTerm) != "" {
		return "exact"
	}
	return normalizeTranscriptMode(req.TranscriptMode)
}

func derivedTextSourceFamily(artifactType any) string {
	switch stringValueAny(artifactType) {
	case "forensics.image-ocr-observation/v1":
		return "image_ocr"
	case "forensics.audio-timestamp-segment/v1":
		return "audio_transcript"
	case "forensics.audio-roman-urdu-segment/v1":
		return "audio_roman_urdu"
	case "forensics.document-native-text-passage/v1":
		return "document_text"
	default:
		return "derived_text"
	}
}

func queryScopeHasTranscript(families []string) bool {
	for _, family := range families {
		if family == "forensics.audio-timestamp-segment/v1" || family == "forensics.audio-roman-urdu-segment/v1" {
			return true
		}
	}
	return false
}

func queryScopeHasResultFamily(families []string, wanted string) bool {
	for _, family := range families {
		if family == wanted {
			return true
		}
	}
	return false
}

var derivedTextQuotedPhrasePattern = regexp.MustCompile(`["“”'‘’]([^"“”'‘’]{1,256})["“”'‘’]`)
var imageOCRExactPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(?:does|did)\s+(?:this|the)\s+image\s+(?:contain|show|say|include|mention)\s+(.{1,256}?)\s*[?.!]*$`),
	regexp.MustCompile(`(?i)(?:find|search|look\s+for)\s+(?:the\s+)?(?:exact\s+)?(?:ocr\s+)?(?:text|phrase|reference|value)?\s*[:=\-]?\s*(.{1,256}?)\s+(?:in|on)\s+(?:this|the|selected)\s+image\s*[?.!]*$`),
	regexp.MustCompile(`(?i)(?:is|was)\s+(.{1,256}?)\s+(?:present|visible|shown|written|mentioned)\s+(?:in|on)\s+(?:this|the|selected)\s+image\s*[?.!]*$`),
}

func exactImageOCRTerm(query string) string {
	if match := derivedTextQuotedPhrasePattern.FindStringSubmatch(query); len(match) == 2 {
		return strings.TrimSpace(match[1])
	}
	for _, pattern := range imageOCRExactPatterns {
		if match := pattern.FindStringSubmatch(strings.TrimSpace(query)); len(match) == 2 {
			return strings.TrimSpace(match[1])
		}
	}
	return ""
}

func applyImageOCRAnswerState(resp *hybridQueryResponse, evidence map[string]any) {
	state := stringValueAny(evidence["result_state"])
	if state == "SEARCH_INCOMPLETE" || state == "RESULTS_TRUNCATED" {
		resp.Answer["ocr_state"] = state
		applyDerivedTextCompleteness(resp, evidence)
		return
	}
	resp.Answer["ocr_state"] = state
	resp.Answer["evidence_count"] = len(evidenceResults(evidence))
	switch state {
	case "COMPLETE_RESULTS":
		resp.Answer["evidence_status"] = "matched"
	case "NO_EXACT_MATCH":
		resp.Answer["evidence_status"] = "no_exact_match"
		resp.Answer["evidence_limitation"] = "No exact normalized OCR observation match was found in the selected current evidence version. No collection-wide substitute was used."
	case "COMPLETE_ZERO_RESULTS":
		resp.Answer["evidence_status"] = "complete_zero_results"
		resp.Answer["evidence_limitation"] = "OCR processing completed without a nonempty observation in the selected current evidence version."
	default:
		resp.Answer["evidence_status"] = "no_match"
		resp.Answer["evidence_limitation"] = "No OCR observation matched in the selected current evidence version."
	}
}

func applyTranscriptAnswerState(resp *hybridQueryResponse, evidence map[string]any, state string) {
	results := evidenceResults(evidence)
	resp.Answer["transcript_state"] = state
	if state == "SEARCH_INCOMPLETE" || state == "RESULTS_TRUNCATED" {
		resp.Answer["evidence_count"] = len(results)
		resp.Answer["transcript_text"] = transcriptResultText(results)
		applyDerivedTextCompleteness(resp, evidence)
		return
	}
	resp.Answer["evidence_count"] = len(results)
	resp.Answer["evidence_summary"] = summarizeTranscriptEvidence(evidence)
	switch state {
	case "COMPLETE_RESULTS":
		resp.Answer["evidence_status"] = "matched"
		resp.Answer["transcript_text"] = transcriptResultText(results)
		if start, end, ok := transcriptResultTimeRange(results); ok {
			resp.Answer["transcript_start_seconds"] = start
			resp.Answer["transcript_end_seconds"] = end
		}
	case "NO_EXACT_MATCH", "NO_MATCH":
		resp.Answer["evidence_status"] = "no_exact_match"
		resp.Answer["evidence_limitation"] = "No exact normalized transcript match was found in the selected current evidence version."
	case "TIMING_UNAVAILABLE":
		resp.Answer["evidence_status"] = "timing_unavailable"
		resp.Answer["processing_status"] = "not_processed"
		resp.Answer["evidence_limitation"] = "Transcript text exists, but valid segment timing is unavailable for this source-time question."
	case "PROCESSING":
		resp.Answer["evidence_status"] = "processing"
		resp.Answer["processing_status"] = "processing"
		resp.Answer["evidence_limitation"] = "Transcription is still processing."
	case "FAILED":
		resp.Answer["evidence_status"] = "failed"
		resp.Answer["processing_status"] = "failed"
		resp.Answer["evidence_limitation"] = "Transcription failed; no speech result was inferred."
	case "COMPLETE_ZERO_RESULTS":
		resp.Answer["evidence_status"] = "complete_zero_results"
		resp.Answer["processing_status"] = "complete_zero"
		resp.Answer["evidence_limitation"] = "Transcription completed without a nonempty transcript observation."
	case "MODEL_REQUIRED":
		resp.Answer["evidence_status"] = "model_required"
		resp.Answer["processing_status"] = "unavailable"
		resp.Answer["evidence_limitation"] = "An admitted local speech model is required before transcription can run."
	case "UNAVAILABLE":
		resp.Answer["evidence_status"] = "unavailable"
		resp.Answer["processing_status"] = "unavailable"
		resp.Answer["evidence_limitation"] = "Transcription is unavailable for the selected current evidence version."
	default:
		resp.Answer["evidence_status"] = "transcription_not_run"
		resp.Answer["processing_status"] = "not_processed"
		resp.Answer["evidence_limitation"] = "Transcription has not run for the selected current evidence version."
	}
}

func summarizeTranscriptEvidence(evidence map[string]any) string {
	state := stringValueAny(evidence["result_state"])
	mode := stringValueAny(evidence["query_mode"])
	return fmt.Sprintf("Governed %s transcript query returned state %s", defaultString(mode, "source"), defaultString(state, "NOT_RUN"))
}

func transcriptResultText(results []map[string]any) string {
	parts := make([]string, 0, len(results))
	for _, item := range results {
		if content := strings.TrimSpace(stringValueAny(item["content"])); content != "" {
			parts = append(parts, content)
		}
	}
	return strings.Join(parts, " ")
}

func transcriptResultTimeRange(results []map[string]any) (float64, float64, bool) {
	var start, end float64
	found := false
	for _, item := range results {
		metadata, _ := item["metadata"].(map[string]any)
		candidateStart, startOK := transcriptSeconds(metadata["start_seconds"])
		candidateEnd, endOK := transcriptSeconds(metadata["end_seconds"])
		if !startOK || !endOK || candidateStart > candidateEnd {
			continue
		}
		if !found || candidateStart < start {
			start = candidateStart
		}
		if !found || candidateEnd > end {
			end = candidateEnd
		}
		found = true
	}
	return start, end, found
}

func applyRequestedEvidenceScope(req *hybridQueryRequest) error {
	scope := &req.QueryScope
	scope.Kind = strings.TrimSpace(scope.Kind)
	if scope.Kind == "" {
		if req.EvidenceID != "" {
			scope.Kind = string(EvidenceScopeSelected)
			scope.EvidenceID = req.EvidenceID
			scope.EvidenceVersionID = req.EvidenceVersionID
		} else {
			scope.Kind = string(EvidenceScopeWorkspace)
		}
	}
	switch scope.Kind {
	case string(EvidenceScopeWorkspace):
		if strings.TrimSpace(scope.EvidenceID) != "" || req.EvidenceID != "" {
			return fmt.Errorf("current_workspace scope cannot include evidence_id")
		}
	case string(EvidenceScopeSelected):
		scope.EvidenceID = strings.ToLower(strings.TrimSpace(scope.EvidenceID))
		if !forensicUUIDPattern.MatchString(scope.EvidenceID) {
			return fmt.Errorf("selected_evidence scope requires an exact evidence UUID")
		}
		if req.EvidenceID != "" && !strings.EqualFold(req.EvidenceID, scope.EvidenceID) {
			return fmt.Errorf("request evidence_id conflicts with selected evidence scope")
		}
		req.EvidenceID = scope.EvidenceID
		scope.EvidenceVersionID = strings.ToLower(strings.TrimSpace(scope.EvidenceVersionID))
		if scope.EvidenceVersionID != "" && !forensicUUIDPattern.MatchString(scope.EvidenceVersionID) {
			return fmt.Errorf("selected evidence version must be a UUID")
		}
		if req.EvidenceVersionID != "" && !strings.EqualFold(req.EvidenceVersionID, scope.EvidenceVersionID) {
			return fmt.Errorf("request evidence version conflicts with selected evidence scope")
		}
		req.EvidenceVersionID = scope.EvidenceVersionID
	default:
		return fmt.Errorf("unsupported evidence scope kind %q", scope.Kind)
	}
	if req.Template == "audio_transcript_search" {
		mode := normalizeTranscriptMode(req.TranscriptMode)
		if mode == "exact" && strings.TrimSpace(req.ExactTerm) == "" {
			return fmt.Errorf("exact transcript search requires exact_term")
		}
		if mode == "time_range" && req.StartSeconds == nil && req.EndSeconds == nil {
			return fmt.Errorf("time-range transcript query requires a source-time bound")
		}
		if req.StartSeconds != nil && req.EndSeconds != nil && *req.StartSeconds > *req.EndSeconds {
			return fmt.Errorf("start_seconds must not exceed end_seconds")
		}
		req.TranscriptMode = mode
	}
	if req.Template == "image_ocr_search" && strings.TrimSpace(req.ExactTerm) != "" {
		req.ExactTerm = strings.TrimSpace(req.ExactTerm)
	}
	return nil
}

func bindAuthoritativeEvidenceScope(ctx context.Context, db *pgxpool.Pool, req *hybridQueryRequest) error {
	rows, err := queryRows(ctx, db, req.TenantID, currentEvidenceScopeSQL, req.TenantID, req.CollectionID, req.EvidenceID)
	if err != nil {
		return err
	}
	if len(rows) != 1 {
		return fmt.Errorf("selected evidence is not available in the authorized case")
	}
	currentVersion := strings.ToLower(strings.TrimSpace(stringValueAny(rows[0]["current_version_id"])))
	if req.EvidenceVersionID != "" && req.EvidenceVersionID != currentVersion {
		return fmt.Errorf("selected evidence version is not the current authoritative version")
	}
	req.EvidenceVersionID = currentVersion
	req.QueryScope.EvidenceVersionID = currentVersion
	req.QueryScope.SourceFamily = normalize(defaultString(stringValueAny(rows[0]["modality"]), stringValueAny(rows[0]["detected_type"])))
	req.EvidenceProcessingState = strings.TrimSpace(stringValueAny(rows[0]["processing_status"]))
	req.EvidenceASRResultState = strings.TrimSpace(stringValueAny(rows[0]["asr_result_state"]))
	req.QueryScope.AvailableResultFamilies = req.QueryScope.AvailableResultFamilies[:0]
	if currentVersion == "" {
		return nil
	}
	families, err := queryRows(ctx, db, req.TenantID, currentEvidenceResultFamiliesSQL, req.TenantID, req.CollectionID, req.EvidenceID, currentVersion)
	if err != nil {
		return err
	}
	for _, row := range families {
		if family := strings.TrimSpace(stringValueAny(row["artifact_type"])); family != "" {
			req.QueryScope.AvailableResultFamilies = append(req.QueryScope.AvailableResultFamilies, family)
		}
	}
	return nil
}

func normalizeTranscriptMode(value string) string {
	switch normalize(value) {
	case "exact":
		return "exact"
	case "time_range":
		return "time_range"
	case "source_time":
		return "source_time"
	default:
		return "source"
	}
}

var transcriptSpacePattern = regexp.MustCompile(`\s+`)

func normalizeTranscriptExact(value string) string {
	value = strings.ToLower(norm.NFKC.String(value))
	var out strings.Builder
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsMark(r) {
			out.WriteRune(r)
		} else {
			out.WriteByte(' ')
		}
	}
	return strings.TrimSpace(transcriptSpacePattern.ReplaceAllString(out.String(), " "))
}

func containsExactNormalizedTranscript(text, term string) bool {
	haystack, needle := normalizeTranscriptExact(text), normalizeTranscriptExact(term)
	return needle != "" && strings.Contains(" "+haystack+" ", " "+needle+" ")
}

func transcriptRowIntersects(row map[string]any, start, end *float64) bool {
	locator, _ := row["citation_locator"].(map[string]any)
	segmentStart, startOK := transcriptSeconds(locator["start_seconds"])
	segmentEnd, endOK := transcriptSeconds(locator["end_seconds"])
	if !startOK || !endOK || segmentStart > segmentEnd {
		return false
	}
	return (end == nil || segmentStart <= *end) && (start == nil || segmentEnd >= *start)
}

func transcriptSeconds(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, typed >= 0
	case float32:
		return float64(typed), typed >= 0
	case int64:
		return float64(typed), typed >= 0
	case int:
		return float64(typed), typed >= 0
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil && parsed >= 0
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return parsed, err == nil && parsed >= 0
	default:
		return 0, false
	}
}

func mergeEvidence(primary, derived map[string]any, limit int) map[string]any {
	primaryResults := evidenceResults(primary)
	derivedResults := evidenceResults(derived)
	if len(derivedResults) == 0 {
		return primary
	}
	combined := make([]map[string]any, 0, len(primaryResults)+len(derivedResults))
	seen := map[string]struct{}{}
	for _, group := range [][]map[string]any{primaryResults, derivedResults} {
		for _, item := range group {
			key := strings.TrimSpace(stringValueAny(firstPresent(item, "id", "citation", "entry")))
			if key != "" {
				if _, exists := seen[key]; exists {
					continue
				}
				seen[key] = struct{}{}
			}
			combined = append(combined, item)
			if len(combined) == limit {
				return map[string]any{"mode": "kb_plus_derived_text", "results": combined}
			}
		}
	}
	return map[string]any{"mode": "kb_plus_derived_text", "results": combined}
}
