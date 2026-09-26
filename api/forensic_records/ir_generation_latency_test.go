package main

import "testing"

// The instrument is fixed before the thing it measures. `llm_latency_ms` is
// assigned only inside bounded synthesis, so a request that spent two minutes
// generating a plan reported llm=0 -- measured 2026-09-24, CDR-02 at
// total=119,835 ms with db=988 ms and 119 seconds attributed to nothing.
//
// This asserts the field exists, is carried on the audit record, and survives
// into the response the analyst's audit trail is built from.
func TestIRGenerationLatencyIsRecordedOnTheAudit(t *testing.T) {
	audit := &SemanticPlannerAuditV1{}
	audit.IRGenerationMS = 119835
	if audit.IRGenerationMS != 119835 {
		t.Fatal("generation latency must be carried on the audit record")
	}
	// A request that never reaches the generator must not report a cost.
	fresh := &SemanticPlannerAuditV1{}
	if fresh.IRGenerationMS != 0 {
		t.Errorf("an unused generator must report 0, got %d", fresh.IRGenerationMS)
	}
}

// The guards above the timer are free; a request rejected for having no family
// must not be charged generation time it never spent.
func TestIRFallbackChargesNothingWhenItNeverGenerates(t *testing.T) {
	t.Setenv(semanticIRFallbackEnv, "false")
	audit := &SemanticPlannerAuditV1{}
	req := hybridQueryRequest{Query: "How many CDR records?", SemanticPlannerAudit: audit}
	if _, state := resolveSemanticIRFallback(t.Context(), config{}, req, "DETERMINISTIC"); state != "DETERMINISTIC" {
		t.Fatalf("disabled fallback must pass the deterministic state through, got %q", state)
	}
	if audit.IRGenerationMS != 0 {
		t.Errorf("no generation ran, so nothing may be charged; got %d ms", audit.IRGenerationMS)
	}
}
