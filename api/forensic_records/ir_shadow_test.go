package main

import (
	"context"
	"testing"
)

// Shadow mode measures the enum-constrained generator on the real production
// path without letting it near an answer. Its first version hung off the
// semantic planner, so it never saw a question the keyword ladder resolved
// first, and the golden-IR scorer recorded those as `absent` — a measurement
// gap that reads exactly like a generator that produced nothing. These tests
// pin the coverage and the isolation.

func unreachableModel() config { return config{LocalAIURL: "http://127.0.0.1:1"} }

func TestIRShadowIsOffByDefaultAndSwitchable(t *testing.T) {
	if semanticIRShadowEnabled() {
		t.Fatal("shadow mode must default to OFF: it costs a model call per question")
	}
	t.Setenv(semanticIRShadowEnv, "true")
	if !semanticIRShadowEnabled() {
		t.Fatal("FORENSIC_IR_SHADOW=true must enable shadow mode")
	}
}

// THE REGRESSION. The ladder answers without ever building a planner audit. If
// shadow mode only writes to an audit that already exists, every ladder-routed
// question is invisible to the scorer.
func TestIRShadowCoversTheLadderPathThatHasNoAudit(t *testing.T) {
	t.Setenv(semanticIRShadowEnv, "true")
	req := hybridQueryRequest{
		Query: "What is the suspect's blood type?", TenantID: "t", CollectionID: "c",
		// What the ladder leaves behind: a bound template and no audit at all.
		Template: "canonical_records", RecordType: "cdr", SemanticPlannerAudit: nil,
	}
	got := ensureSemanticIRShadow(context.Background(), unreachableModel(), req, req, "deterministic_fast_path")
	if got.SemanticPlannerAudit == nil {
		t.Fatal("a ladder-routed question must still get an audit to record the shadow on")
	}
	if got.SemanticPlannerAudit.IRShadowOutcome == "" {
		t.Fatal("empty outcome is what the scorer reports as `absent`; the ladder path must be measured")
	}
	// The created audit must be inert: query.go's binding check reads these.
	if got.SemanticPlannerAudit.Selected != nil || got.SemanticPlannerAudit.BindingState != "" {
		t.Fatal("the synthesized audit must not look like a bound operation")
	}
	if got.SemanticPlannerAudit.State != "deterministic_fast_path" {
		t.Fatalf("the routing status must be recorded so the scorer can tell the paths apart, got %q",
			got.SemanticPlannerAudit.State)
	}
}

// Measurement must not become an answer.
func TestIRShadowNeverTouchesTheAnswer(t *testing.T) {
	t.Setenv(semanticIRShadowEnv, "true")
	req := hybridQueryRequest{Query: "What is the suspect's blood type?", TenantID: "t", CollectionID: "c"}
	got := ensureSemanticIRShadow(context.Background(), unreachableModel(), req, req, "AMBIGUOUS_INTENT")
	if got.SourceNative != nil || got.Template != "" || got.RecordType != "" {
		t.Fatal("shadow mode may write to the audit and nothing else")
	}
}

// Disabled, it must be a true no-op — no audit conjured into the response.
func TestIRShadowDisabledLeavesTheRequestAlone(t *testing.T) {
	req := hybridQueryRequest{Query: "How many CDR records do we have?", TenantID: "t", CollectionID: "c"}
	got := ensureSemanticIRShadow(context.Background(), unreachableModel(), req, req, "deterministic_fast_path")
	if got.SemanticPlannerAudit != nil {
		t.Fatal("shadow mode off must not add an audit to the response")
	}
	// Nor with shadow on but no model configured.
	t.Setenv(semanticIRShadowEnv, "true")
	if got := ensureSemanticIRShadow(context.Background(), config{}, req, req, "x"); got.SemanticPlannerAudit != nil {
		t.Fatal("no LocalAI configured must short-circuit before touching the request")
	}
}

// One generation per question. A second costs ~130s on this hardware and can
// only disagree with the first.
func TestIRShadowDoesNotRegenerateWhatIsAlreadyRecorded(t *testing.T) {
	t.Setenv(semanticIRShadowEnv, "true")
	audit := &SemanticPlannerAuditV1{IRShadowOutcome: "generated"}
	plan := &SourceNativePlanV1{ContractVersion: sourceNativePlanContractV1}
	audit.IRShadowPlan = plan
	req := hybridQueryRequest{Query: "How many CDR records do we have?", TenantID: "t",
		CollectionID: "c", SemanticPlannerAudit: audit}
	got := ensureSemanticIRShadow(context.Background(), unreachableModel(), req, req, "x")
	if got.SemanticPlannerAudit.IRShadowOutcome != "generated" || got.SemanticPlannerAudit.IRShadowPlan != plan {
		t.Fatal("an already-recorded shadow must not be regenerated or overwritten")
	}
}

// The fallback runs the same generator. With both switches on, the accepted
// plan is the shadow result — paying twice measures nothing extra.
func TestIRShadowReusesAnAcceptedFallbackPlan(t *testing.T) {
	t.Setenv(semanticIRShadowEnv, "true")
	plan := &SourceNativePlanV1{ContractVersion: sourceNativePlanContractV1,
		GroupFields: []string{"cdr.call_type"}}
	req := hybridQueryRequest{Query: "How many CDR records do we have?", TenantID: "t", CollectionID: "c",
		SourceNative:         plan,
		SemanticPlannerAudit: &SemanticPlannerAuditV1{IRFallbackOutcome: "accepted"},
	}
	got := ensureSemanticIRShadow(context.Background(), unreachableModel(), req, req, "semantic_ir_fallback")
	if got.SemanticPlannerAudit.IRShadowOutcome != "generated" || got.SemanticPlannerAudit.IRShadowPlan != plan {
		t.Fatal("an accepted fallback plan must be reused as the shadow record")
	}
}

// A question with no curated family has no authorized field set to enumerate.
// That is a real measurement outcome, not a missing one.
func TestIRShadowRecordsNoFamilyRatherThanNothing(t *testing.T) {
	t.Setenv(semanticIRShadowEnv, "true")
	audit := &SemanticPlannerAuditV1{}
	recordSemanticIRShadow(context.Background(), unreachableModel(),
		hybridQueryRequest{Query: "What is the suspect's blood type?", TenantID: "t", CollectionID: "c"}, audit)
	if audit.IRShadowOutcome != "no_family" {
		t.Fatalf("expected no_family, got %q", audit.IRShadowOutcome)
	}
	if audit.IRShadowPlan != nil {
		t.Fatal("no family means no plan")
	}
}
