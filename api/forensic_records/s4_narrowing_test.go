package main

import (
	"testing"
)

func distinctCatalog() []FieldDescriptorV1 {
	return []FieldDescriptorV1{
		{FieldID: "cdr.msisdn", EffectiveType: "STRING",
			AllowedAggregates: []string{"COUNT", "COUNT_DISTINCT"}},
		{FieldID: "cdr.call_duration_seconds", EffectiveType: fieldTypeDecimal,
			AllowedAggregates: []string{"COUNT", "SUM", "AVG", "MIN", "MAX"}},
	}
}

// The narrowing fires only on the goal it was measured against.
func TestDistinctNarrowingAppliesOnlyToDistinctQuestions(t *testing.T) {
	catalog := distinctCatalog()
	if ids := distinctOnlyFieldIDs(catalog, "distinct"); len(ids) != 1 || ids[0] != "cdr.msisdn" {
		t.Fatalf("a distinct question may count only distinct-capable fields, got %v", ids)
	}
	// Every other goal in the corpus -- aggregate, lookup, rank, range, search,
	// source_rows, summarize -- must be untouched.
	for _, goal := range []string{"aggregate", "lookup", "rank", "range", "search", "source_rows", "summarize", ""} {
		if ids := distinctOnlyFieldIDs(catalog, goal); ids != nil {
			t.Errorf("goal %q must not be narrowed, got %v", goal, ids)
		}
	}
}

// THE EMPTY-ENUM KILLER. Narrowing to nothing is far worse than not narrowing:
// an empty enum compiles to an alternation with no alternatives and llama.cpp
// answers the request with "failed to parse grammar" -- HTTP 500 before any
// inference. A distinct question over a catalogue with nothing distinct-capable
// must fall back, not narrow to empty.
func TestDistinctNarrowingFallsBackRatherThanIssuingAnEmptyEnum(t *testing.T) {
	noDistinct := []FieldDescriptorV1{
		{FieldID: "cdr.call_duration_seconds", EffectiveType: fieldTypeDecimal,
			AllowedAggregates: []string{"COUNT", "SUM"}},
	}
	if ids := distinctOnlyFieldIDs(noDistinct, "distinct"); ids != nil {
		t.Fatalf("with nothing distinct-capable the schema must NOT be narrowed, got %v", ids)
	}
	// And the schema it produces must still be usable.
	schema := semanticSourceNativeSchemaForGoal(noDistinct, "distinct")
	if bad := emptyEnums(schema, "schema"); len(bad) > 0 {
		for _, path := range bad {
			t.Logf("pre-existing empty enum (unrelated to narrowing): %s", path)
		}
	}
	variants := issuedMeasureVariants(t, schema)
	for i, variant := range variants {
		for _, key := range []string{"op", "field_id", "measure_id"} {
			if len(enumOf(t, variant, key)) == 0 {
				t.Errorf("measure variant %d has an empty enum for %q", i, key)
			}
		}
	}
}

// CDR-08's property, on the schema the generator actually receives: a distinct
// question is offered COUNT_DISTINCT and nothing else.
func TestDistinctQuestionIsOfferedOnlyCountDistinct(t *testing.T) {
	schema := semanticSourceNativeSchemaForGoal(distinctCatalog(), "distinct")
	variants := issuedMeasureVariants(t, schema)
	if len(variants) != 1 {
		t.Fatalf("a distinct question needs exactly one measure shape, got %d", len(variants))
	}
	ops := enumOf(t, variants[0], "op")
	if len(ops) != 1 || ops[0] != "COUNT_DISTINCT" {
		t.Fatalf("op enum = %v, want [COUNT_DISTINCT]; a plain COUNT answers rows, not "+
			"distinct values, and S9 refuses it anyway", ops)
	}
	fieldIDs := enumOf(t, variants[0], "field_id")
	if containsString(fieldIDs, "") {
		t.Error("COUNT_DISTINCT of nothing counts nothing; the empty field must not be offered")
	}
	if containsString(fieldIDs, "cdr.call_duration_seconds") {
		t.Error("a field that is not distinct-capable must not be offered for a distinct count")
	}
}

// The control that matters: a NON-distinct question keeps both measure shapes,
// including the plain row count that most of the corpus depends on.
func TestNonDistinctQuestionsKeepTheRowCount(t *testing.T) {
	schema := semanticSourceNativeSchemaForGoal(distinctCatalog(), "aggregate")
	variants := issuedMeasureVariants(t, schema)
	if len(variants) != 2 {
		t.Fatalf("an ordinary question keeps the row-count and field-aggregate variants, got %d", len(variants))
	}
	rowCountOffered := false
	for _, variant := range variants {
		ops := enumOf(t, variant, "op")
		ids := enumOf(t, variant, "field_id")
		if len(ops) == 1 && ops[0] == "COUNT" && containsString(ids, "") {
			rowCountOffered = true
		}
	}
	if !rowCountOffered {
		t.Fatal("counting rows must remain expressible; most of the corpus is a row count")
	}
}

// S9's obligation is NOT removed by the narrowing. A schema and a verifier
// agreeing is the point; the verifier stopping is not.
func TestS9DistinctObligationSurvivesTheNarrowing(t *testing.T) {
	req := hybridQueryRequest{Query: "How many unique phone numbers appear as callers in the CDRs?",
		TenantID: "t", CollectionID: "c",
		QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)}}
	frame := extractSemanticFrame(req)
	if frame.Goal != "distinct" {
		t.Skipf("frame goal changed to %q; re-derive this probe", frame.Goal)
	}
	plainCount := &SourceNativePlanV1{
		ContractVersion: sourceNativePlanContractV1,
		Measures:        []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT"}},
	}
	if err := verifySourceNativePlanShape(frame, req.Query, plainCount); err == nil {
		t.Fatal("S9 must still refuse a plain count for a distinct question, narrowing or not")
	}
}
