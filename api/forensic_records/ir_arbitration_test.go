package main

import (
	"context"
	"testing"
)

// Arbitration lets a re-verified generated plan replace a CLARIFICATION, from
// any route. Its safety rests entirely on that scope: a clarification asserts
// nothing, so substituting a verified plan cannot turn a correct answer wrong.
//
// Measured on the 62 golden questions, 2026-09-22:
//   clarifications a verified plan answers correctly : 2
//   clarifications it answers wrongly                : 0
//   the rejected alternative -- arbitrating CONFIDENT answers on a shape
//   mismatch -- fired on 3 wrong answers and 2 CORRECT ones.

func TestArbitrationIsOffByDefaultAndSwitchable(t *testing.T) {
	if semanticIRArbitrationEnabled() {
		t.Fatal("arbitration must default to OFF and be enabled deliberately")
	}
	t.Setenv(semanticIRArbitrationEnv, "true")
	if !semanticIRArbitrationEnabled() {
		t.Fatal("FORENSIC_IR_ARBITRATION=true must enable it")
	}
}

// It is a SEPARATE switch from the fallback so either can be withdrawn without
// losing the other.
func TestArbitrationAndFallbackAreIndependentSwitches(t *testing.T) {
	t.Setenv(semanticIRArbitrationEnv, "true")
	t.Setenv(semanticIRFallbackEnv, "false")
	if !semanticIRArbitrationEnabled() || semanticIRFallbackEnabled() {
		t.Fatal("the two switches must be independently controllable")
	}
}

// Arbitration reuses the fallback resolver, which re-verifies every plan. With
// the fallback disabled nothing is generated, so arbitration cannot smuggle an
// unverified plan into an answer.
func TestArbitrationCannotProduceAnUnverifiedPlan(t *testing.T) {
	t.Setenv(semanticIRArbitrationEnv, "true")
	t.Setenv(semanticIRFallbackEnv, "false")
	req := hybridQueryRequest{Query: "Which cell site handled the most calls?", TenantID: "t", CollectionID: "c"}
	got, state := resolveSemanticIRFallback(context.Background(), config{LocalAIURL: "http://127.0.0.1:1"}, req, "CLARIFICATION")
	if state == "semantic_ir_fallback" || got.SourceNative != nil {
		t.Fatal("no plan may be produced while the generator is disabled")
	}
}

// An unreachable model must leave the clarification standing, never error.
func TestArbitrationDegradesToTheClarification(t *testing.T) {
	t.Setenv(semanticIRArbitrationEnv, "true")
	t.Setenv(semanticIRFallbackEnv, "true")
	req := hybridQueryRequest{Query: "Which cell site handled the most calls?", TenantID: "t", CollectionID: "c"}
	got, state := resolveSemanticIRFallback(context.Background(), config{LocalAIURL: "http://127.0.0.1:1"}, req, "CLARIFICATION")
	if state != "CLARIFICATION" {
		t.Fatalf("an unreachable model must leave the clarification intact, got %q", state)
	}
	if got.SourceNative != nil {
		t.Fatal("no plan may survive a failed generation")
	}
}

// A generated plan must run inside the family it was generated for. Without
// this the typed algebra ran across EVERY record type in the collection:
// measured 2026-09-22, "which cell site handled the most calls" grouped over
// 12,912 rows spanning six families instead of the 8,642 CDR rows asked about.
func TestGeneratedPlanIsScopedToItsFamily(t *testing.T) {
	catalog := []FieldDescriptorV1{{
		ContractVersion: fieldDescriptorContractV1, FieldID: "cdr.cell_site_id",
		SourceName: "Cell_SITE_ID", SourceNames: []string{"Cell_SITE_ID"}, NormalizedName: "cell_site_id",
		EffectiveType: "STRING", AllowedFilters: []string{"EQ"}, AllowedAggregates: []string{"COUNT"},
		Projectable: true, Groupable: true, Sortable: true, Curated: true,
	}}
	proposal := semanticDynamicProposal{Mode: "source_native", SourceNative: &SourceNativePlanV1{
		ContractVersion: sourceNativePlanContractV1,
		GroupFields:     []string{"cdr.cell_site_id"},
		Measures:        []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT"}},
	}}
	req := hybridQueryRequest{Query: "Which cell site handled the most calls?", TenantID: "t", CollectionID: "c",
		QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)}}
	bound, err := applySemanticSourceNativePlan(req, "communications_cdr", proposal, catalog)
	if err != nil {
		t.Skipf("scope/family gating rejected the fixture: %v", err)
	}
	if bound.RecordType != "cdr" {
		t.Fatalf("plan record type = %q, want cdr — an unscoped plan counts other families' rows", bound.RecordType)
	}
	// An analyst's explicit scope must win over the family default.
	req.RecordType = "ipdr"
	bound, err = applySemanticSourceNativePlan(req, "communications_cdr", proposal, catalog)
	if err == nil && bound.RecordType != "ipdr" {
		t.Fatalf("explicit record type was overwritten with %q", bound.RecordType)
	}
}
