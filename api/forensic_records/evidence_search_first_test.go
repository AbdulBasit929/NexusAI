package main

import "testing"

func TestDerivedTextSearchNamedByQuestion(t *testing.T) {
	t.Run("switch off changes nothing", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		t.Setenv(evidenceSearchFirstEnv, "")
		if derivedTextSearchNamedByQuestion("Find OCR text mentioning BCX-567", "image_ocr_search") {
			t.Fatal("gate opened with the switch off") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})

	t.Run("switch on", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		t.Setenv(evidenceSearchFirstEnv, "true")
		named := map[string]string{
			"Find OCR text mentioning BCX-567":                       "image_ocr_search",
			"Search image OCR for BCX-567":                           "image_ocr_search",
			"Search the case documents for mentions of plate MN1367": "document_search",
			"Is 03001234567 mentioned in any audio recording?":       "audio_transcript_search",
		}
		for question, template := range named {
			if !derivedTextSearchNamedByQuestion(question, template) {
				t.Errorf("%q with %s: the question names this evidence, the search must stay there", question, template) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
			}
		}
		notNamed := map[string]string{
			// A text search the question never aimed at stays under every existing guard.
			"How many times was plate LHR-2026 seen?":    "image_ocr_search",
			"Find OCR text mentioning BCX-567":           "document_search",
			"Search the case documents for plate MN1367": "anpr_sightings",
			"Which camera recorded the most sightings?":  "canonical_records",
			// Word boundaries: "photograph" is not a scope keyword, "photos" is.
			"Show the demographic breakdown": "image_ocr_search",
		}
		for question, template := range notNamed {
			if derivedTextSearchNamedByQuestion(question, template) {
				t.Errorf("%q with %s: must not open the gate", question, template) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
			}
		}
	})

	t.Run("verified-only no longer withholds a named text search", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		t.Setenv(evidenceSearchFirstEnv, "true")
		t.Setenv("FORENSIC_VERIFIED_ONLY", "true")
		doc01 := hybridQueryRequest{Query: "Search the case documents for mentions of plate MN1367", Template: "document_search"}
		if verifiedOnlyWithholds(doc01) {
			t.Fatal("DOC-01's document search must not be withheld for want of a typed plan") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
		// The same question with a structured template is still held to a verified plan.
		structured := hybridQueryRequest{Query: "Search the case documents for mentions of plate MN1367", Template: "anpr_sightings"}
		if !verifiedOnlyWithholds(structured) {
			t.Fatal("a structured template answering this question must still need a verified plan") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})
}
