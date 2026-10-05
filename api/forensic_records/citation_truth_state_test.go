package main

import "testing"

func TestCitationTruthState(t *testing.T) {
	item := func() map[string]any { return map[string]any{"artifact_id": "a1"} }

	t.Run("switch off adds nothing", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		t.Setenv(citationTruthStateEnv, "")
		if got := withCitationTruthState(item(), "derived_model_observation"); got["source_truth_state"] != nil {
			t.Fatalf("truth state added with the switch off: %v", got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
		citations, _ := factPacketCitations([]map[string]any{{"artifact_id": "a1", "source_truth_state": "derived_model_observation"}})
		if citations[0].SourceTruthState != "" {
			t.Fatal("citation carried a truth state with the switch off") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})

	t.Run("switch on copies the stored value, never invents one", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		t.Setenv(citationTruthStateEnv, "true")
		if got := withCitationTruthState(item(), "derived_model_observation"); got["source_truth_state"] != "derived_model_observation" {
			t.Fatalf("a transcript segment's stored truth state must be carried: %v", got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
		if got := withCitationTruthState(item(), "derived_native_text"); got["source_truth_state"] != "derived_native_text" {
			t.Fatalf("a document passage's stored truth state must be carried: %v", got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
		// Absent in the store: nothing is added, not a guess.
		for _, absent := range []any{nil, "", "  "} {
			if got := withCitationTruthState(item(), absent); got["source_truth_state"] != nil {
				t.Fatalf("an absent truth state must not be filled in: %v", got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
			}
		}
		citations, _ := factPacketCitations([]map[string]any{
			{"artifact_id": "a1", "source_truth_state": "derived_model_observation"},
			{"artifact_id": "a2"},
		})
		if citations[0].SourceTruthState != "derived_model_observation" || citations[1].SourceTruthState != "" {
			t.Fatalf("citations: %+v", citations) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})
}
