package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// Characterisation of the `rejected_shape` fallback outcome, which is all the
// audit recorded for CDR-05 and CDR-08 -- it named neither the rule nor the
// stage, and the two turned out to be different rules in different files at
// different stages.
//
// Both plans are the VERBATIM cached completions read from
// /data/forensic/spool/plan-cache on 2026-09-24. No model call.
//
// The `stage` column is the LIVE-VERIFIED outcome, updated as each defect is
// fixed, so this file doubles as the regression record for both.
func TestRejectedShapeIsTwoUnrelatedRules(t *testing.T) {
	const (
		stageAccepted   = "accepted"
		stageDecode     = "decode"
		stageStructural = "structural"
		stageS9         = "s9"
	)
	cases := []struct {
		id, question, completion, stage, because string
	}{
		{
			id:       "CDR-05",
			question: "How many calls of each type are there?",
			completion: `{"contract_version":"forensics.source-native-plan/v1","filters":[],` +
				`"group_fields":["cdr.call_type"],"having":[],"limit":0,` +
				`"measures":[{"field_id":"","measure_id":"m1","op":"COUNT"}],` +
				`"project":[],"sort":[],"time_bucket":null}`,
			stage: stageAccepted,
			because: "FIXED 2026-09-24. S9 rejected this correct breakdown as `a scalar " +
				"question compiled a grouping by cdr.call_type`, because extraction does not " +
				"read `of each type` as a grouping signal. The scalar guard now asks the " +
				"curated layer whether the analyst NAMED the grouped field.",
		},
		{
			id:       "CDR-08",
			question: "How many unique phone numbers appear as callers in the CDRs?",
			completion: `{"contract_version":"forensics.source-native-plan/v1","filters":[],` +
				`"group_fields":[],"having":[],"limit":0,` +
				`"measures":[{"field_id":"","measure_id":"m1","op":"COUNT_DISTINCT"}],` +
				`"project":[],"sort":[],"time_bucket":null}`,
			stage: stageDecode,
			because: "OPEN. The issued schema lists \"\" among field_id's enum members so a " +
				"plain COUNT may omit a field, which also permits COUNT_DISTINCT of nothing. " +
				"The generator took it and the strict parser refuses it at decode.",
		},
	}

	for _, tc := range cases {
		req := hybridQueryRequest{Query: tc.question, TenantID: "t", CollectionID: "c",
			QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)}}
		frame := extractSemanticFrame(req)
		t.Logf("=== %s: %s", tc.id, tc.question)
		t.Logf("    frame.goal=%q group_by_hints=%v", frame.Goal, frame.GroupByHints)
		t.Logf("    %s", tc.because)

		reached := stageAccepted
		var reason error

		var plan SourceNativePlanV1
		if err := json.Unmarshal([]byte(tc.completion), &plan); err != nil {
			reached, reason = stageDecode, err
		} else if err := validateSourceNativePlanShape(&plan); err != nil {
			reached, reason = stageStructural, err
		} else if err := verifySourceNativePlanShape(frame, tc.question, &plan); err != nil {
			reached, reason = stageS9, err
		}

		if reason != nil {
			t.Logf("    REJECTED AT %s -> %v", strings.ToUpper(reached), reason)
		} else {
			t.Logf("    ACCEPTED -- every stage passes this plan")
		}
		if reached != tc.stage {
			t.Errorf("%s: rejected at %q, expected %q (%v)", tc.id, reached, tc.stage, reason)
		}
	}
}

// CDR-05's regression guard, stated as the property rather than as the defect.
//
// It previously asserted the MISMATCH -- that S9 refused a correct breakdown --
// because that was the behaviour being characterised. Now that the scalar guard
// consults the curated layer, the same case asserts the fix.
func TestS9AcceptsTheBreakdownTheGeneratorCorrectlyDerived(t *testing.T) {
	req := hybridQueryRequest{Query: "How many calls of each type are there?",
		TenantID: "t", CollectionID: "c",
		QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)}}
	frame := extractSemanticFrame(req)

	// The question still classifies as "scalar" -- the fix did not reclassify
	// it, it taught the guard to check whether the grouping was asked for.
	if got := s9QuestionShape(frame, req.Query); got != "scalar" {
		t.Logf("note: the question now classifies as %q rather than scalar", got)
	}
	plan := &SourceNativePlanV1{
		ContractVersion: sourceNativePlanContractV1,
		GroupFields:     []string{"cdr.call_type"},
		Measures:        []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT"}},
	}
	if err := verifySourceNativePlanShape(frame, req.Query, plan); err != nil {
		t.Fatalf("the analyst asked for a breakdown by type; S9 must not refuse it: %v", err)
	}
}

// CDR-08, stated as the property rather than the defect.
//
// This previously asserted the MISMATCH -- that the schema permitted
// `COUNT_DISTINCT` with an empty field_id while the validator refused it,
// which is how CDR-08's plan came to be emitted and then rejected at decode.
// The measure is now issued as two variants, so the combination is no longer
// representable and the schema and the validator agree.
//
// The assertion is structural, not a substring search: `""` still appears in
// the schema, legitimately, inside the row-count variant.
func TestSchemaCannotExpressCountDistinctWithoutAField(t *testing.T) {
	fields := []FieldDescriptorV1{
		{FieldID: "cdr.msisdn", EffectiveType: "STRING", AllowedAggregates: []string{"COUNT", "COUNT_DISTINCT"}},
	}
	measures := issuedMeasureVariants(t, semanticSourceNativeSchema(fields))
	if len(measures) < 2 {
		t.Fatalf("expected a row-count variant and a field-aggregate variant, got %d", len(measures))
	}
	for i, variant := range measures {
		ops := enumOf(t, variant, "op")
		fieldIDs := enumOf(t, variant, "field_id")
		t.Logf("variant %d: op=%v field_id=%v", i, ops, fieldIDs)

		// The offending combination: a variant that allows an empty field_id
		// alongside any aggregate that needs one.
		if containsString(fieldIDs, "") {
			for _, op := range ops {
				if !strings.EqualFold(op, "COUNT") {
					t.Errorf("variant %d permits %s with an empty field_id, which counts nothing", i, op)
				}
			}
		}
	}

	// And the validator still refuses it, so the two agree rather than one
	// silently carrying the other.
	plan := &SourceNativePlanV1{
		ContractVersion: sourceNativePlanContractV1,
		Measures:        []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT_DISTINCT", FieldID: ""}},
	}
	if err := validateSourceNativePlanShape(plan); err == nil {
		t.Fatal("COUNT_DISTINCT with no field counts nothing; it must still be rejected")
	}
}

// An EMPTY ENUM is the known killer on this path: the GBNF converter turns
// `enum: []` into an alternation with no alternatives and llama.cpp answers
// with "failed to parse grammar" -- HTTP 500 before any inference. Retrieval
// can legitimately issue no fields, so the aggregate variant must disappear
// rather than be offered empty.
func TestMeasureSchemaOffersNoEmptyEnumWhenNoFieldsAreIssued(t *testing.T) {
	schema := semanticSourceNativeSchema(nil)
	measures := issuedMeasureVariants(t, schema)
	if len(measures) != 1 {
		t.Fatalf("with nothing to aggregate only the row-count variant may be offered, got %d", len(measures))
	}
	for _, key := range []string{"op", "field_id", "measure_id"} {
		if values := enumOf(t, measures[0], key); len(values) == 0 {
			t.Errorf("the row-count variant has an EMPTY enum for %q; the grammar will not parse", key)
		}
	}

	// PRE-EXISTING EXPOSURE, recorded here rather than silently passed over.
	// It is NOT introduced by the measure split and this test does not fail on
	// it, because it is not reachable today: `retrieveSourceNativeFields`
	// returns the top-K fields for a question and every family question in the
	// corpus is issued 12-14. A question that reached this path with zero
	// fields would 500 before inference, the same way an empty `timeIDs` did
	// until `time_bucket` was made null-only for exactly this reason.
	if bad := emptyEnums(schema, "schema"); len(bad) > 0 {
		t.Logf("LATENT: with zero issued fields these enums are empty and the grammar "+
			"would not parse: %v -- guard them the way time_bucket already is if a "+
			"path is ever found that issues none", bad)
	}
}

// issuedMeasureVariants returns each alternative shape a measure may take:
// the anyOf branches when there are several, or the single object otherwise.
func issuedMeasureVariants(t *testing.T, schema map[string]any) []map[string]any {
	t.Helper()
	props, _ := schema["properties"].(map[string]any)
	measures, _ := props["measures"].(map[string]any)
	items, _ := measures["items"].(map[string]any)
	if items == nil {
		t.Fatal("the issued schema must describe measure items")
	}
	alternatives, ok := items["anyOf"].([]any)
	if !ok {
		return []map[string]any{items}
	}
	out := []map[string]any{}
	for _, alternative := range alternatives {
		if variant, ok := alternative.(map[string]any); ok {
			out = append(out, variant)
		}
	}
	return out
}

func enumOf(t *testing.T, variant map[string]any, property string) []string {
	t.Helper()
	props, _ := variant["properties"].(map[string]any)
	spec, _ := props[property].(map[string]any)
	return schemaEnumValues(spec["enum"])
}
