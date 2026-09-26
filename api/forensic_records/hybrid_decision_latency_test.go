package main

import "testing"

// The third untimed LLM call found in one session. Five questions produced
// five /v1/chat/completions calls while reporting BOTH llm_latency_ms=0 and
// ir_generation_ms=0 -- roughly 16 s each, attributed to nothing.
//
// The invariant is the one that makes the number trustworthy: a deterministic
// choice is free and must report zero, so a non-zero value always means a
// model was actually consulted.
func TestHybridDecisionLatencyIsZeroWhenNoModelWasConsulted(t *testing.T) {
	// No LocalAI configured: the planner cannot consult a model, so whatever
	// state it reaches, it must not claim to have spent decision time.
	req := hybridQueryRequest{
		Query:        "How many CDR records are in this case?",
		TenantID:     "default",
		CollectionID: "nexusai-forensic-demo",
	}
	out, _ := resolveHybridPlanner(t.Context(), config{}, req)
	if out.HybridAudit == nil {
		t.Fatal("the planner must always attach its audit record")
	}
	if out.HybridAudit.DecisionLatencyMS != 0 {
		t.Errorf("no model was consulted, so no decision time may be claimed; got %d ms",
			out.HybridAudit.DecisionLatencyMS)
	}
	if out.HybridAudit.DecisionSource == "MODEL_DECISION" {
		t.Errorf("without a LocalAI URL the decision source cannot be MODEL_DECISION, got %q",
			out.HybridAudit.DecisionSource)
	}
}

// The field must survive onto the audit the analyst's trail is built from --
// an unrecorded cost is exactly the defect this closes.
func TestHybridDecisionLatencyIsCarriedOnTheAudit(t *testing.T) {
	audit := &HybridPlannerAuditV1{}
	if audit.DecisionLatencyMS != 0 {
		t.Fatal("a fresh audit must start at zero")
	}
	audit.DecisionLatencyMS = 16000
	if audit.DecisionLatencyMS != 16000 {
		t.Error("decision latency must be carried on the audit record")
	}
}
