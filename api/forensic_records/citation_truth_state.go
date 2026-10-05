package main

import (
	"os"
	"strings"
)

// A2. A CITATION SAYS WHAT KIND OF EVIDENCE IT IS.
//
// Every derived artifact records `metadata.source_truth_state`:
// derived_model_observation for transcripts, image text and plate reads,
// derived_native_text for text extracted from a document, and
// derived_text_representation for a Roman-Urdu rendering. The typed-plan path
// already carried it into provenance. The document, image-text and transcript
// SEARCH path did not, so the workspace could only label a speech-to-text
// segment "Source record" -- a model's output shown as if it were ingested
// evidence (found in the 2026-09-28 demo check; backend request 7).
//
// The value is copied from the stored observation, never inferred. When it is
// absent, nothing is added. See reports/citation-truth-state-20260929/.
const citationTruthStateEnv = "FORENSIC_CITATION_TRUTH_STATE"

func citationTruthStateEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(citationTruthStateEnv)), "true") //nolint:forbidigo // forensic records product API reads its feature switches from the environment by design (docker compose); not LocalAI server config
}

// citationTruthState returns the stored truth state to carry, or "" when the
// switch is off or the observation recorded none.
func citationTruthState(value any) string {
	if !citationTruthStateEnabled() {
		return ""
	}
	return strings.TrimSpace(stringValueAny(value))
}

// withCitationTruthState copies a non-empty truth state into a provenance item.
func withCitationTruthState(item map[string]any, value any) map[string]any {
	if state := citationTruthState(value); state != "" {
		item["source_truth_state"] = state
	}
	return item
}
