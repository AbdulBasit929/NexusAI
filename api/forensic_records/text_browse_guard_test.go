package main

import (
	"strings"
	"testing"

	"github.com/mudler/LocalAI/pkg/forensictext"
)

// CENSUS FIRST. Which questions reach the derived-text retrieval path with
// NOTHING TO FILTER ON?
//
// `derivedTextRelevance` returns (0, true) when a question carries no
// identifier and no content term -- "nothing to filter on: a browse request".
// Every passage then scores as relevant and is returned WITH ITS FULL TEXT.
//
// Those passages are `observation.raw_text`, `normalized_text`, `text`,
// `roman_urdu_text` and `raw_urdu_text` -- the exact five curated fields marked
// PII that `CatalogFields` withholds from every typed plan
// (media_pii_boundary_test.go proves it at every goal). The retrieval path never
// consults that sensitivity, so an untargeted question dumps them.
//
// Measured 2026-09-26: reclassifying H2 "What does the audio transcript say?"
// onto this path returned the actual Urdu transcript in the analyst answer.
//
// SEARCH WITH A TERM IS NOT THE DEFECT and must not be broken. AUD-01, AUD-02,
// AUD-03, DOC-02, DOC-03 and IMG-01 all supply a term the analyst already holds
// and are CORRECT in the golden 62: the snippet is the citation that proves the
// match. The defect is the UNTARGETED case, where there is no term and the
// system returns everything.
func TestTextBrowseCensus(t *testing.T) {
	paths := []string{
		"../../evaluation/golden_questions_v2.json",
		"../../evaluation/holdout_questions_v1.json",
		"../../evaluation/holdout_media_v1.json",
	}
	total := 0
	for _, path := range paths {
		untargeted := []string{}
		for _, q := range loadRoutingQuestions(t, path) {
			// The real call: relevance of ANY text against this question.
			if _, relevant := derivedTextRelevance("any passage text at all", q.Q); relevant {
				if score, _ := derivedTextRelevance("zzzz-nothing-matches-zzzz", q.Q); score == 0 {
					untargeted = append(untargeted, q.ID+"  "+q.Q)
				}
			}
		}
		t.Logf("")
		t.Logf("%s : %d question(s) would return EVERY passage in full", path, len(untargeted))
		for _, line := range untargeted {
			t.Logf("   %s", line)
		}
		total += len(untargeted)
	}
	t.Logf("")
	t.Logf("TOTAL UNTARGETED TEXT-BROWSE SURFACE: %d", total)
}

// THE SIX WORKING SEARCH ANSWERS MUST STAY TARGETED. These are CORRECT in the
// golden 62 and every one shows a snippet as its citation. If the guard claims
// any of them, it has destroyed six working answers to protect nothing — the
// analyst supplied the identifier in the question.
func TestTargetedSearchesAreNeverTreatedAsBrowse(t *testing.T) {
	for _, tc := range []struct{ id, question string }{
		{"AUD-01", "Search the audio transcripts for Japanese cuisine"},
		{"AUD-02", "Which recording mentions coconut sugar and at what time?"},
		{"AUD-03", "Is the number 03001234567 mentioned in any audio?"},
		{"DOC-02", "Which document mentions contact number 03001234567?"},
		{"DOC-03", `Find the exact phrase "Scanned-only PDF OCR" in the documents`},
		{"IMG-01", "Find OCR text mentioning Investigation Workspace"},
		{"IMG-02", "Which image contains the text REF-2026-02?"},
		{"DOC-06", `Search documents for the phrase "quantum encryption key"`},
	} {
		req := hybridQueryRequest{Query: tc.question}
		if derivedTextRequestIsUntargeted(req) {
			t.Errorf("%s was treated as an untargeted browse; its snippet is the citation: %q", tc.id, tc.question)
		}
	}
}

// THE UNTARGETED CASE IS CLAIMED. H2 is the honesty probe.
func TestUntargetedBrowseIsClaimed(t *testing.T) {
	if !derivedTextRequestIsUntargeted(hybridQueryRequest{Query: "What does the audio transcript say?"}) {
		t.Error("H2 was not recognised as an untargeted browse — the PII path is open")
	}
}

// ANY FORM OF TARGETING DISQUALIFIES IT, including ones that carry no content
// words at all. A time window is a filter the analyst named; a supplied target
// and an explicit TextQuery are targeting whatever the prose looks like.
func TestAnyTargetingDisqualifiesTheGuard(t *testing.T) {
	start := 30.0
	for name, req := range map[string]hybridQueryRequest{
		"time window":     {Query: "What does the transcript say?", StartSeconds: &start},
		"explicit target": {Query: "What does the transcript say?", Target: "03001234567"},
		"text query":      {Query: "What does the transcript say?", TextQuery: &forensictext.Query{}},
	} {
		if derivedTextRequestIsUntargeted(req) {
			t.Errorf("%s was treated as untargeted", name)
		}
	}
}

// WITHHOLDING REMOVES THE WORDS AND KEEPS THE EVIDENCE IDENTITY. The analyst
// must still learn which file, when, and how to cite it — that is the accepted
// §9 B ruling: withhold the text, keep counts, timestamps, language and locators.
func TestWithholdingKeepsLocatorsAndSaysWhy(t *testing.T) {
	result := map[string]any{
		"id": "artifact-1", "content": "SECRET TRANSCRIPT", "preview": "SECRET TRANSCRIPT",
		"citation": "nexusai://evidence/ev-1/artifacts/artifact-1",
		"metadata": map[string]any{
			"raw_text": "SECRET TRANSCRIPT", "search_text": "SECRET TRANSCRIPT",
			"source_file": "urdu-english-identifier.wav", "start_seconds": 2.84, "end_seconds": 4.1,
			"evidence_id": "ev-1", "citation_locator": map[string]any{"start_seconds": 2.84},
			"contributing_observations": []map[string]any{{"passage_text": "SECRET", "text": "SECRET"}},
		},
	}
	withholdBrowsePassageText(result)

	blob := marshalJSONString(result)
	if strings.Contains(blob, "SECRET") {
		t.Fatalf("passage text survived withholding: %s", blob)
	}
	if result["text_withheld"] != true {
		t.Error("the response does not say the text was withheld")
	}
	if result["text_withheld_reason"] != "PII_UNTARGETED_BROWSE" {
		t.Error("no reason given for the withholding")
	}
	metadata := result["metadata"].(map[string]any)
	for key, want := range map[string]any{
		"source_file": "urdu-english-identifier.wav", "start_seconds": 2.84,
		"end_seconds": 4.1, "evidence_id": "ev-1",
	} {
		if metadata[key] != want {
			t.Errorf("withholding destroyed %s: got %v, want %v", key, metadata[key], want)
		}
	}
	if metadata["citation_locator"] == nil {
		t.Error("withholding destroyed the citation locator")
	}
	if result["citation"] == "" {
		t.Error("withholding destroyed the citation")
	}
}

// THE GUARD DEFAULTS ON. Default-off would ship a bulk-PII path.
func TestTextBrowseGuardDefaultsOn(t *testing.T) {
	t.Setenv(textBrowseGuardEnv, "")
	if !textBrowseGuardEnabled() {
		t.Error("the guard defaults OFF, which ships the PII path")
	}
	t.Setenv(textBrowseGuardEnv, "false")
	if textBrowseGuardEnabled() {
		t.Error("the guard cannot be disabled for a control measurement")
	}
}
