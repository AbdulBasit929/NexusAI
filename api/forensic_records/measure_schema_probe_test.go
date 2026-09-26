package main

import (
	"strings"
	"testing"

	"github.com/mudler/LocalAI/pkg/functions/grammars"
)

// Measure the candidate BEFORE building it.
//
// The CDR-08 fix splits the measure object into two variants so that
// COUNT_DISTINCT-of-nothing becomes impossible to emit rather than rejected
// after the fact. That only works if the JSON-Schema -> GBNF converter handles
// an `anyOf` inside an array's `items`, and a grammar the converter cannot
// build is not a worse plan -- it is an HTTP 500 raised before any inference,
// which is how the empty-enum defect presented ("Failed to initialize samplers:
// failed to parse grammar").
//
// So the grammar is compiled here, offline, from the REAL issued schema.
func TestMeasureVariantsCompileToGrammar(t *testing.T) {
	fields := []FieldDescriptorV1{
		{FieldID: "cdr.msisdn", EffectiveType: "STRING",
			AllowedAggregates: []string{"COUNT", "COUNT_DISTINCT"}},
		{FieldID: "cdr.event_time", EffectiveType: fieldTypeTimestamp,
			AllowedAggregates: []string{"COUNT", "MIN", "MAX"}},
	}
	schema := semanticSourceNativeSchema(fields)

	converter := grammars.NewJSONSchemaConverter("")
	grammar, err := converter.Grammar(schema)
	if err != nil {
		t.Fatalf("the issued schema must compile to a grammar, or every request 500s: %v", err)
	}
	if strings.TrimSpace(grammar) == "" {
		t.Fatal("an empty grammar constrains nothing")
	}
	t.Logf("grammar compiled, %d bytes", len(grammar))
}

// The property the split exists to create: a measure that counts distinct
// values of nothing must be unrepresentable, not merely invalid.
func TestCountDistinctWithoutAFieldIsRejected(t *testing.T) {
	plan := &SourceNativePlanV1{
		ContractVersion: sourceNativePlanContractV1,
		Measures:        []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT_DISTINCT", FieldID: ""}},
	}
	if err := validateSourceNativePlanShape(plan); err == nil {
		t.Fatal("COUNT_DISTINCT of nothing counts nothing")
	}
}

// And the shapes that must stay legal, or the split breaks every other
// question: a plain COUNT of rows, and an aggregate over a named field.
func TestLegitimateMeasureShapesSurvive(t *testing.T) {
	catalog := []FieldDescriptorV1{
		{FieldID: "cdr.msisdn", EffectiveType: "STRING",
			AllowedAggregates: []string{"COUNT", "COUNT_DISTINCT"}, Projectable: true},
	}
	for _, tc := range []struct {
		name string
		m    SourceNativeMeasureV1
	}{
		{"plain COUNT of rows", SourceNativeMeasureV1{MeasureID: "m1", Op: "COUNT"}},
		{"COUNT of a field", SourceNativeMeasureV1{MeasureID: "m1", Op: "COUNT", FieldID: "cdr.msisdn"}},
		{"COUNT_DISTINCT of a field", SourceNativeMeasureV1{MeasureID: "m1", Op: "COUNT_DISTINCT", FieldID: "cdr.msisdn"}},
	} {
		plan := &SourceNativePlanV1{
			ContractVersion: sourceNativePlanContractV1,
			Measures:        []SourceNativeMeasureV1{tc.m},
		}
		if err := validateSourceNativePlan(plan, catalog); err != nil {
			t.Errorf("%s must stay legal: %v", tc.name, err)
		}
	}
}
