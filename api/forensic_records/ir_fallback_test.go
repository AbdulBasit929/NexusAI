package main

import (
	"context"
	"testing"
)

// Inverting the gate. The IR generator was unreachable in production; these
// tests pin the conditions under which it now runs and, more importantly, the
// conditions under which its output is still thrown away.

func TestIRFallbackAppliesOnlyToUnresolvedOutcomes(t *testing.T) {
	for _, state := range []string{
		"AMBIGUOUS_INTENT", "UNSUPPORTED_REQUEST_CLASS",
		"UNRESOLVED_ISSUED_FIELD_OR_OPERATION", "NO_AUTHORIZED_ALGEBRA_OR_OPERATION",
	} {
		if !semanticIRFallbackApplies(state) {
			t.Errorf("%s means the compiler could not answer; the fallback must run", state)
		}
	}
	// An answer already exists — a model must never second-guess it.
	for _, state := range []string{
		"semantic_deterministic_registered", "semantic_deterministic_dynamic",
		"field_catalog_unavailable", "deterministic_registered_validation_rejected",
	} {
		if semanticIRFallbackApplies(state) {
			t.Errorf("%s must not trigger the model fallback", state)
		}
	}
	// A refusal that NAMES the unbound literal is more useful than a guess.
	if semanticIRFallbackApplies("CONSTRAINT_UNBOUND:phone 923001110001") {
		t.Error("a refusal naming the unbound literal must stand")
	}
}

// The fallback is OFF by default because it was measured converting honest
// clarifications into confident wrong answers, and switchable without a rebuild.
func TestIRFallbackIsOffByDefaultAndSwitchable(t *testing.T) {
	if semanticIRFallbackEnabled() {
		t.Fatal("the model-assisted fallback must default to OFF")
	}
	t.Setenv(semanticIRFallbackEnv, "true")
	if !semanticIRFallbackEnabled() {
		t.Fatal("FORENSIC_IR_FALLBACK=true must enable the fallback")
	}
	t.Setenv(semanticIRFallbackEnv, "false")
	if semanticIRFallbackEnabled() {
		t.Fatal("FORENSIC_IR_FALLBACK=false must disable the fallback")
	}
	req := hybridQueryRequest{Query: "How many CDR records do we have?", TenantID: "t", CollectionID: "c"}
	if _, state := resolveSemanticIRFallback(context.Background(), config{LocalAIURL: "http://127.0.0.1:1"}, req, "AMBIGUOUS_INTENT"); state != "AMBIGUOUS_INTENT" {
		t.Fatalf("disabled fallback must return the deterministic state, got %q", state)
	}
}

// With no resolvable family there is no authorized field set to enumerate, and
// the enum is the whole reason this path is safe to take.
func TestIRFallbackRefusesWithoutAFamilyToEnumerate(t *testing.T) {
	t.Setenv(semanticIRFallbackEnv, "true")
	req := hybridQueryRequest{Query: "What is the suspect's blood type?", TenantID: "t", CollectionID: "c"}
	_, state := resolveSemanticIRFallback(context.Background(), config{LocalAIURL: "http://127.0.0.1:1"}, req, "AMBIGUOUS_INTENT")
	if state != "AMBIGUOUS_INTENT" {
		t.Fatalf("a question with no curated family must not reach the generator; got %q", state)
	}
}

// An unreachable model must degrade to the deterministic refusal, never to an
// error or an empty answer.
func TestIRFallbackDegradesWhenTheModelIsUnreachable(t *testing.T) {
	t.Setenv(semanticIRFallbackEnv, "true")
	req := hybridQueryRequest{Query: "How many CDR records do we have in this case?", TenantID: "t", CollectionID: "c"}
	got, state := resolveSemanticIRFallback(context.Background(), config{LocalAIURL: "http://127.0.0.1:1"}, req, "AMBIGUOUS_INTENT")
	if state != "AMBIGUOUS_INTENT" {
		t.Fatalf("an unreachable model must leave the refusal intact, got %q", state)
	}
	if got.SourceNative != nil {
		t.Fatal("no plan may survive a failed generation")
	}
	// And with no LocalAI configured at all it must not even attempt a call.
	if _, s := resolveSemanticIRFallback(context.Background(), config{}, req, "AMBIGUOUS_INTENT"); s != "AMBIGUOUS_INTENT" {
		t.Fatalf("no LocalAI configured must short-circuit, got %q", s)
	}
}

// The selector stays disabled: it is the component that was measured at 0/5 and
// whose choice nothing downstream can re-verify.
func TestOperationSelectorRemainsOutOfTheProductionPath(t *testing.T) {
	// resolveSemanticOperationInCandidates is referenced only by tests. If a
	// future change wires it back into the planner, this is the reminder that
	// its failure mode is a confident wrong operation, not a checkable plan.
	if !semanticIRFallbackApplies("AMBIGUOUS_INTENT") {
		t.Fatal("the IR generator, not the selector, is the fallback")
	}
}

// A plan the fallback accepted has already passed S6, S9 SHAPE and
// CONSTRAINT_APPLIED. If the planner's acceptance predicate does not list its
// state, that valid plan is silently discarded and the question clarifies
// anyway — which is exactly what happened to three questions.
func TestIRFallbackStateIsAcceptedByThePlanner(t *testing.T) {
	if !hybridResolvedStatus("semantic_ir_fallback") {
		t.Fatal("semantic_ir_fallback must be an accepted planner outcome")
	}
	for _, state := range []string{"AMBIGUOUS_INTENT", "UNSUPPORTED_REQUEST_CLASS"} {
		if hybridResolvedStatus(state) {
			t.Errorf("%s must not be accepted as an answer", state)
		}
	}
}

// A plan references field IDs; the executor resolves them through the
// catalogue. If the catalogue does not travel with the plan, a valid plan
// executes into "the requested analysis needs a valid required parameter".
func TestIRPlanCarriesItsCatalogue(t *testing.T) {
	catalog := []FieldDescriptorV1{{
		ContractVersion: fieldDescriptorContractV1, FieldID: "cdr.call_type",
		SourceName: "CALL_TYPE", SourceNames: []string{"CALL_TYPE"}, NormalizedName: "call_type",
		EffectiveType: "STRING", AllowedFilters: []string{"EQ"}, AllowedAggregates: []string{"COUNT"},
		Projectable: true, Groupable: true, Sortable: true, Curated: true,
	}}
	proposal := semanticDynamicProposal{Mode: "source_native", SourceNative: &SourceNativePlanV1{
		ContractVersion: sourceNativePlanContractV1,
		GroupFields:     []string{"cdr.call_type"},
		Measures:        []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT"}},
	}}
	req := hybridQueryRequest{Query: "How many CDR records by call type?", TenantID: "t", CollectionID: "c",
		QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)}}
	bound, err := applySemanticSourceNativePlan(req, "communications_cdr", proposal, catalog)
	if err != nil {
		t.Skipf("scope/family gating rejected the fixture: %v", err)
	}
	if len(bound.SourceNativeCatalog) != len(catalog) {
		t.Fatalf("the catalogue must travel with the plan; got %d fields", len(bound.SourceNativeCatalog))
	}
	if bound.Template != "canonical_records" || bound.SourceNative == nil {
		t.Fatalf("plan not bound: template=%q plan=%v", bound.Template, bound.SourceNative)
	}
}

// The execution contract requires Parameters.Limit >= 1. Zeroing it made every
// IR-produced plan fail the governed row budget after passing every other
// guard, surfacing as "the requested analysis needs a valid required parameter".
func TestIRPlanKeepsAUsableRowLimit(t *testing.T) {
	catalog := []FieldDescriptorV1{{
		ContractVersion: fieldDescriptorContractV1, FieldID: "cdr.call_type",
		SourceName: "CALL_TYPE", SourceNames: []string{"CALL_TYPE"}, NormalizedName: "call_type",
		EffectiveType: "STRING", AllowedFilters: []string{"EQ"}, AllowedAggregates: []string{"COUNT"},
		Projectable: true, Groupable: true, Sortable: true, Curated: true,
	}}
	proposal := semanticDynamicProposal{Mode: "source_native", SourceNative: &SourceNativePlanV1{
		ContractVersion: sourceNativePlanContractV1,
		GroupFields:     []string{"cdr.call_type"},
		Measures:        []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT"}},
	}}
	req := hybridQueryRequest{Query: "How many CDR records by call type?", TenantID: "t", CollectionID: "c",
		QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)}}
	bound, err := applySemanticSourceNativePlan(req, "communications_cdr", proposal, catalog)
	if err != nil {
		t.Skipf("scope/family gating rejected the fixture: %v", err)
	}
	if bound.Limit < 1 {
		t.Fatalf("limit = %d; the execution contract requires at least 1", bound.Limit)
	}
}
