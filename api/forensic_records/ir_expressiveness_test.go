package main

import (
	"strings"
	"testing"
)

// The two IR expressiveness gaps. Both were DECLARED in the curated layer and
// unrepresentable in the typed plan, so CDR-07/08, ANPR-05 and CDR-15 could not
// be answered at any model quality.

func TestCountDistinctIsExpressibleAndExecutes(t *testing.T) {
	layer, _ := defaultSemanticLayer()
	if layer == nil {
		t.Skip("layer unavailable")
	}
	entity, ok := layer.EntityByFamily("communications_cdr")
	if !ok {
		t.Skip("family missing")
	}
	var dialed FieldDescriptorV1
	for _, f := range entity.CatalogFields(hybridQueryRequest{}) {
		if f.FieldID == "cdr.dialed_number" {
			dialed = f
		}
	}
	if dialed.FieldID == "" {
		t.Fatal("cdr.dialed_number not in the curated catalogue")
	}
	if !containsString(dialed.AllowedAggregates, "COUNT_DISTINCT") {
		t.Fatalf("a distinct_capable field must offer COUNT_DISTINCT; got %v", dialed.AllowedAggregates)
	}
	// The schema must let the model choose it.
	//
	// Read through the measure VARIANTS rather than a fixed path. A measure is
	// now issued as two alternatives -- row count, and aggregate over a field --
	// so that COUNT_DISTINCT with an empty field_id, which CDR-08's generator
	// emitted and the parser refused, cannot be expressed at all. The property
	// under test is unchanged: some variant must offer COUNT_DISTINCT.
	schema := semanticSourceNativeSchema([]FieldDescriptorV1{dialed})
	offered := false
	var seen [][]string
	for _, variant := range issuedMeasureVariants(t, schema) {
		ops := enumOf(t, variant, "op")
		seen = append(seen, ops)
		if containsString(ops, "COUNT_DISTINCT") && len(enumOf(t, variant, "field_id")) > 0 {
			offered = true
		}
	}
	if !offered {
		t.Fatalf("no measure variant offers COUNT_DISTINCT over an issued field: %v", seen)
	}
	// And it must reach SQL as a distinct count, not a row count.
	args := []any{}
	sql, _, err := sourceNativeMeasureSQL(sourceNativeRecordsBinding(), 
		SourceNativeMeasureV1{MeasureID: "m1", Op: "COUNT_DISTINCT", FieldID: "cdr.dialed_number"},
		map[string]FieldDescriptorV1{"cdr.dialed_number": dialed}, args)
	if err != nil {
		t.Fatalf("COUNT_DISTINCT did not compile: %v", err)
	}
	if !strings.Contains(sql, "COUNT(DISTINCT") {
		t.Fatalf("expected a distinct count, got %s", sql)
	}
}

// COUNT_DISTINCT without a field would silently become a row count.
func TestCountDistinctRequiresItsField(t *testing.T) {
	err := validateSourceNativePlanShape(&SourceNativePlanV1{
		ContractVersion: sourceNativePlanContractV1,
		Measures:        []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT_DISTINCT"}},
	})
	if err == nil {
		t.Fatal("COUNT_DISTINCT with no field_id must be rejected, not silently counted as rows")
	}
}

// There is NO duration column in CDR data, only a start and an end.
func TestDerivedDurationMetricIsExpressibleAndExecutes(t *testing.T) {
	layer, _ := defaultSemanticLayer()
	if layer == nil {
		t.Skip("layer unavailable")
	}
	entity, ok := layer.EntityByFamily("communications_cdr")
	if !ok {
		t.Skip("family missing")
	}
	catalog := entity.CatalogFields(hybridQueryRequest{})
	byID := map[string]FieldDescriptorV1{}
	for _, f := range catalog {
		byID[f.FieldID] = f
	}
	duration, ok := byID["cdr.call_duration_seconds"]
	if !ok {
		t.Fatal("the derived duration metric is not published as a catalogue field")
	}
	if duration.DerivedOp == "" || len(duration.DerivedFields) != 2 {
		t.Fatalf("derived metric carries no resolvable expression: %+v", duration)
	}
	// A computed value has no stored column, so it must not be filterable or groupable.
	if duration.Groupable || duration.Projectable || len(duration.AllowedFilters) != 0 {
		t.Error("a derived metric must be measurable only")
	}
	args := []any{}
	sql, _, err := sourceNativeMeasureSQL(sourceNativeRecordsBinding(), 
		SourceNativeMeasureV1{MeasureID: "m1", Op: "MAX", FieldID: "cdr.call_duration_seconds"}, byID, args)
	if err != nil {
		t.Fatalf("derived metric did not compile: %v", err)
	}
	if !strings.Contains(sql, "EXTRACT(EPOCH FROM") || !strings.HasPrefix(sql, "MAX(") {
		t.Fatalf("expected MAX over an elapsed-seconds expression, got %s", sql)
	}
}

// Selecting the metric must drag in the columns it is computed from, or the
// plan validates and then fails at execution.
func TestDerivedMetricRetrievalIncludesItsDependencies(t *testing.T) {
	layer, _ := defaultSemanticLayer()
	if layer == nil {
		t.Skip("layer unavailable")
	}
	entity, ok := layer.EntityByFamily("communications_cdr")
	if !ok {
		t.Skip("family missing")
	}
	retrieved, _ := retrieveSourceNativeFields("what was the longest call", entity.CatalogFields(hybridQueryRequest{}))
	got := map[string]bool{}
	for _, f := range retrieved {
		got[f.FieldID] = true
	}
	if !got["cdr.call_duration_seconds"] {
		t.Skip("duration metric not retrieved for this phrasing; dependency rule untested here")
	}
	for _, need := range []string{"cdr.call_start", "cdr.call_end"} {
		if !got[need] {
			t.Errorf("duration was issued without %s; the plan would compile and then fail", need)
		}
	}
}
