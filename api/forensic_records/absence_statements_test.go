package main

import (
	"strings"
	"testing"
)

func transcriptSearch(state, mode, term string) hybridQueryResponse {
	return hybridQueryResponse{
		Template: "audio_transcript_search",
		Evidence: map[string]any{"result_state": state, "query_mode": mode, "exact_term": term},
		Answer:   map[string]any{},
	}
}

func TestRelevanceSearchMissIsNotAbsence(t *testing.T) {
	// P2's recorded search: relevance mode, no term, nothing found.
	p2 := transcriptSearch("NO_MATCH", "source", "")

	t.Run("switch off changes nothing", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		t.Setenv(textSearchNotAbsenceEnv, "")
		if relevanceSearchMissIsNotAbsence(p2) {
			t.Fatal("withheld with the switch off") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})

	t.Run("switch on", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		t.Setenv(textSearchNotAbsenceEnv, "true")
		if !relevanceSearchMissIsNotAbsence(p2) {
			t.Fatal("an empty word search must not be reported as absence") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
		spared := map[string]hybridQueryResponse{
			// DOC-06 is CORRECT: an exact phrase, searched and not found, scoped to what was searched.
			"exact phrase (DOC-06)":    {Template: "document_search", Evidence: map[string]any{"result_state": "NO_EXACT_MATCH", "query_mode": "PHRASE_CONTAINS", "exact_term": "quantum encryption key"}},
			"exact term supplied":      transcriptSearch("NO_MATCH", "source", "03001234567"),
			"a requested time range":   transcriptSearch("NO_MATCH", "time_range", ""),
			"results were found":       transcriptSearch("COMPLETE_RESULTS", "source", ""),
			"not a text search":        {Template: "canonical_records", Evidence: map[string]any{"result_state": "NO_MATCH", "query_mode": "source"}},
			"no evidence block at all": {Template: "audio_transcript_search"},
		}
		for name, resp := range spared {
			if relevanceSearchMissIsNotAbsence(resp) {
				t.Errorf("%s: must not be withheld", name) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
			}
		}
		reason := textSearchNotAbsenceReason(p2)
		if reason.Code != withholdTextSearchNotAbsence || !strings.Contains(reason.Question, "the transcripts") ||
			strings.Contains(reason.Question, "source-time range") {
			t.Fatalf("reason must name what was searched and never claim a time range: %+v", reason) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})
}

func TestClarifyingResponseNeverDescribesAResult(t *testing.T) {
	// R5: held for clarification, nothing ran.
	r5 := hybridQueryResponse{
		Template: "multi_cdr_comparison",
		Intent:   intentClarify,
		Answer: map[string]any{
			"clarification_required": true,
			"clarification":          "Which exact MSISDN and two to eight exact CDR source identities should I compare?",
		},
		Records: map[string]any{},
	}
	req := hybridQueryRequest{Query: "What do the two busiest numbers have in common?"}

	t.Run("switch off keeps today's behaviour", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		t.Setenv(clarificationNotResultEnv, "")
		if clarifyingInsteadOfResult(r5) {
			t.Fatal("changed behaviour with the switch off") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})

	t.Run("switch on states the question, not a result", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		t.Setenv(clarificationNotResultEnv, "true")
		payload := buildEnterprisePayload(req, r5)
		answer := stringValueAny(payload["executive_answer"])
		if strings.Contains(answer, "were found") || strings.Contains(answer, "No qualifying") {
			t.Fatalf("a clarification stated a result: %q", answer) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
		if !strings.Contains(answer, "Which exact MSISDN") {
			t.Fatalf("a clarification must state its question, got %q", answer) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
		answered := hybridQueryResponse{Template: "multi_cdr_comparison", Answer: map[string]any{}}
		if clarifyingInsteadOfResult(answered) {
			t.Fatal("an executed comparison must keep its own payload builder") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})
}
