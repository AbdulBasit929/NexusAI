package main

import (
	"strings"
	"testing"
)

func TestDerivedTextEvidenceIsScopedAndContractBounded(t *testing.T) {
	for _, expected := range []string{
		"artifacts.tenant_id=$1", "artifacts.collection_id=$2",
		"artifacts.processing_status='completed'", "artifacts.artifact_type = ANY($3::text[])",
		"($4 = '' OR artifacts.evidence_id = $4::uuid)",
	} {
		if !strings.Contains(derivedTextEvidenceSQL, expected) {
			t.Fatalf("derived text SQL is missing %q", expected)
		}
	}
	if len(searchableDerivedTextContracts) != 4 {
		t.Fatalf("searchable contract count = %d, want 4", len(searchableDerivedTextContracts))
	}
	if !containsString(searchableDerivedTextContracts, "forensics.image-ocr-observation/v1") {
		t.Fatal("general image OCR observations must be included in governed derived-text search")
	}
}

func TestMergeEvidencePreservesKBThenAddsDerivedCitations(t *testing.T) {
	primary := map[string]any{"mode": "vector_search", "results": []map[string]any{{"id": "kb-1"}}}
	derived := map[string]any{"mode": "derived_text_lexical", "results": []map[string]any{{"id": "artifact-1", "citation": "nexusai://evidence/e-1/artifacts/artifact-1"}}}
	merged := mergeEvidence(primary, derived, 5)
	results := evidenceResults(merged)
	if merged["mode"] != "kb_plus_derived_text" || len(results) != 2 {
		t.Fatalf("merged evidence = %#v", merged)
	}
	if results[1]["citation"] != "nexusai://evidence/e-1/artifacts/artifact-1" {
		t.Fatalf("derived citation was not preserved: %#v", results[1])
	}
}

func TestQueryTermsPreserveUrduAndRomanUrduTokens(t *testing.T) {
	terms := queryTerms("یہ پاکستان ki clear audio hai")
	for _, expected := range []string{"یہ", "پاکستان", "ki", "clear", "audio", "hai"} {
		if !containsString(terms, expected) {
			t.Fatalf("query terms %#v missing %q", terms, expected)
		}
	}
}

func TestTranscriptSourceTimePreservesObservedRange(t *testing.T) {
	results := []map[string]any{
		{"metadata": map[string]any{"start_seconds": 4.5, "end_seconds": 8.0}},
		{"metadata": map[string]any{"start_seconds": 8.0, "end_seconds": 10.25}},
	}
	start, end, ok := transcriptResultTimeRange(results)
	if !ok || start != 4.5 || end != 10.25 {
		t.Fatalf("time range = %v..%v (%v), want 4.5..10.25", start, end, ok)
	}
	if got := normalizeTranscriptMode("source_time"); got != "source_time" {
		t.Fatalf("mode = %q, want source_time", got)
	}
	response := hybridQueryResponse{Template: "audio_transcript_search", Answer: map[string]any{
		"transcript_state": "COMPLETE_RESULTS", "transcript_start_seconds": 4.5, "transcript_end_seconds": 10.25,
	}}
	answer := enterpriseExecutiveAnswer(hybridQueryRequest{TranscriptMode: "source_time"}, response, nil)
	if answer != "The cited transcript observation runs from 4.50 seconds to 10.25 seconds in the recording." {
		t.Fatalf("source-time answer = %q", answer)
	}
}

func TestSelectedImageOCRExactMatchIsCurrentEvidenceAndRegionBound(t *testing.T) {
	req := hybridQueryRequest{TenantID: "tenant-a", CollectionID: "case-a", EvidenceID: "evidence-a", EvidenceVersionID: "version-current", Template: "image_ocr_search", ExactTerm: "REF-7788", MaxKBResults: 5}
	rows := []map[string]any{
		{"artifact_id": "ocr-current", "artifact_type": "forensics.image-ocr-observation/v1", "evidence_id": "evidence-a", "version_id": "version-current", "source_file": "selected.png", "passage_text": "Invoice REF-7788", "citation_locator": map[string]any{"region": 2, "x": 10}},
		{"artifact_id": "ocr-unrelated", "artifact_type": "forensics.image-ocr-observation/v1", "evidence_id": "evidence-other", "version_id": "version-current", "source_file": "other.png", "passage_text": "Invoice REF-7788", "citation_locator": map[string]any{"region": 1}},
	}
	result := governedDerivedTextEvidence(rows, req)
	items := evidenceResults(result)
	if result["result_state"] != "COMPLETE_RESULTS" || len(items) != 1 {
		t.Fatalf("exact OCR result = %#v", result)
	}
	metadata := items[0]["metadata"].(map[string]any)
	if metadata["evidence_id"] != req.EvidenceID || metadata["version_id"] != req.EvidenceVersionID || metadata["source_family"] != "image_ocr" {
		t.Fatalf("exact OCR scope/authority lost: %#v", metadata)
	}
	if metadata["citation_locator"].(map[string]any)["region"] != 2 {
		t.Fatalf("OCR region citation lost: %#v", metadata)
	}
}

func TestSelectedImageOCRExactNoMatchDoesNotReturnRelatedText(t *testing.T) {
	req := hybridQueryRequest{Template: "image_ocr_search", ExactTerm: "ABSENT-999", MaxKBResults: 5}
	result := governedDerivedTextEvidence([]map[string]any{{"artifact_id": "ocr-1", "artifact_type": "forensics.image-ocr-observation/v1", "passage_text": "related invoice text"}}, req)
	if result["result_state"] != "NO_EXACT_MATCH" || len(evidenceResults(result)) != 0 {
		t.Fatalf("exact OCR no-match was not clean: %#v", result)
	}
}

func TestTranscriptResultCarriesTypedTimeCitationMetadata(t *testing.T) {
	req := hybridQueryRequest{Template: "audio_transcript_search", TranscriptMode: "time_range", StartSeconds: float64Pointer(4), EndSeconds: float64Pointer(6), MaxKBResults: 5}
	result := governedDerivedTextEvidence([]map[string]any{{"artifact_id": "segment-1", "artifact_type": "forensics.audio-timestamp-segment/v1", "evidence_id": "audio-1", "version_id": "version-1", "source_file": "speech.wav", "passage_text": "bounded speech", "citation_locator": map[string]any{"start_seconds": 3.5, "end_seconds": 5.25}}}, req)
	metadata := evidenceResults(result)[0]["metadata"].(map[string]any)
	if metadata["start_seconds"] != 3.5 || metadata["end_seconds"] != 5.25 || metadata["source_family"] != "audio_transcript" {
		t.Fatalf("typed transcript citation metadata lost: %#v", metadata)
	}
}
