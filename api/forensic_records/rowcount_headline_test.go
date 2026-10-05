package main

import (
	"strings"
	"testing"
)

// The ABC-123 plan captured on 2026-09-28: plate filter, COUNT(*) among four
// measures, one aggregate row.
func abc123Plan() *SourceNativePlanV1 {
	return &SourceNativePlanV1{
		ContractVersion: "forensics.source-native-plan/v1",
		Filters:         []SourceNativeFilterV1{{FieldID: "anpr.plate_number", Op: "EQ", Value: "ABC-123", Values: []string{"ABC-123"}}},
		Measures: []SourceNativeMeasureV1{
			{MeasureID: "m1", Op: "COUNT"},
			{MeasureID: "m2", Op: "COUNT", FieldID: "anpr.plate_script"},
			{MeasureID: "m3", Op: "COUNT", FieldID: "anpr.province"},
			{MeasureID: "m4", Op: "AVG", FieldID: "anpr.confidence"},
		},
	}
}

func TestMultiMeasureRecordCountHeadline(t *testing.T) {
	req := hybridQueryRequest{Query: "Show me everything you have about plate ABC-123", RecordType: "anpr", Target: "ABC-123", Targets: []string{"ABC-123"}}
	results := []map[string]any{{"m1": 218, "m2": 0, "m3": 0, "m4": "0.862188"}}

	t.Run("switch off changes nothing", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		t.Setenv(rowCountHeadlineGuardEnv, "")
		if _, ok := multiMeasureRecordCountHeadline(req, hybridQueryResponse{}, abc123Plan(), results); ok {
			t.Fatal("stated a headline with the switch off") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})

	t.Run("switch on states the record count, never the row count", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		t.Setenv(rowCountHeadlineGuardEnv, "true")
		answer, ok := multiMeasureRecordCountHeadline(req, hybridQueryResponse{}, abc123Plan(), results)
		if !ok {
			t.Fatal("the COUNT(*) of an ungrouped multi-measure plan must be stated") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
		if !strings.Contains(answer.Headline, "218") || strings.Contains(answer.Headline, "1 ANPR sighting") {
			t.Fatalf("headline must state 218, never the one result row: %q", answer.Headline) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
		if answer.Value != 218.0 {
			t.Fatalf("value = %v, want 218", answer.Value) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}

		declined := map[string]*SourceNativePlanV1{
			"grouped":   func() *SourceNativePlanV1 { p := abc123Plan(); p.GroupFields = []string{"anpr.camera_id"}; return p }(),
			"projected": func() *SourceNativePlanV1 { p := abc123Plan(); p.Project = []string{"anpr.plate_number"}; return p }(),
			// COUNT(field) counts non-empty values of that field, not records.
			"no COUNT(*)": {Measures: []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT", FieldID: "anpr.province"}, {MeasureID: "m2", Op: "AVG", FieldID: "anpr.confidence"}}},
			"one measure": {Measures: []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT"}}},
		}
		for name, plan := range declined {
			if _, ok := multiMeasureRecordCountHeadline(req, hybridQueryResponse{}, plan, results); ok {
				t.Errorf("%s: must decline", name) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
			}
		}
		if _, ok := multiMeasureRecordCountHeadline(req, hybridQueryResponse{}, abc123Plan(), nil); ok {
			t.Error("no result row: must decline") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})

	t.Run("a MIN/MAX range still answers through the range path", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		t.Setenv(rowCountHeadlineGuardEnv, "true")
		t.Setenv(answerStatesValuesEnv, "true")
		plan := &SourceNativePlanV1{ContractVersion: "forensics.source-native-plan/v1", Measures: []SourceNativeMeasureV1{
			{MeasureID: "m1", Op: "MIN", FieldID: "cdr.event_time"}, {MeasureID: "m2", Op: "MAX", FieldID: "cdr.event_time"}}}
		resp := hybridQueryResponse{Records: map[string]any{"source_native_results": []map[string]any{{"m1": "2026-04-01T00:00:00Z", "m2": "2026-08-10T00:00:00Z"}}}}
		answer, ok := sourceNativeResultAnswer(hybridQueryRequest{RecordType: "cdr"}, resp, plan)
		if !ok || !strings.Contains(answer.Headline, "2026-04-01") || !strings.Contains(answer.Headline, "2026-08-10") {
			t.Fatalf("range path must still answer CDR-09's shape, got ok=%v %q", ok, answer.Headline) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})
}

func TestRowCountFallbackSentence(t *testing.T) {
	t.Run("switch off keeps today's sentence", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		t.Setenv(rowCountHeadlineGuardEnv, "")
		if got := rowCountFallbackSentence(hybridQueryResponse{Template: "suspicious_patterns"}, 14, "records"); got != "" {
			t.Fatalf("changed the sentence with the switch off: %q", got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})

	t.Run("switch on", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		t.Setenv(rowCountHeadlineGuardEnv, "true")
		// The captured "Summarise the suspicious activity" case: 14 summary rows.
		composite := rowCountFallbackSentence(hybridQueryResponse{Template: "suspicious_patterns"}, 14, "records")
		if composite == "" || strings.Contains(composite, "14") || strings.Contains(composite, "matched") {
			t.Fatalf("a composite summary must state no row count: %q", composite) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
		aggregate := rowCountFallbackSentence(hybridQueryResponse{Template: "canonical_records", Records: map[string]any{"plan": abc123Plan()}}, 1, "ANPR sighting")
		if aggregate == "" || strings.Contains(aggregate, "1 ANPR") {
			t.Fatalf("aggregate results must state no row count: %q", aggregate) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
		page := rowCountFallbackSentence(hybridQueryResponse{Template: "canonical_records", Records: map[string]any{"total_count": 87}}, 20, "ANPR sightings")
		if page != "Showing 20 of 87 ANPR sightings that matched this question." {
			t.Fatalf("a page of a longer listing must say so: %q", page) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
		unknown := rowCountFallbackSentence(hybridQueryResponse{Template: "canonical_records", Records: map[string]any{}}, 20, "CDR records")
		if !strings.Contains(unknown, "not computed") {
			t.Fatalf("a listing with no known total must say the total was not computed: %q", unknown) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
		// The one case where today's sentence is true: the whole total is shown.
		if got := rowCountFallbackSentence(hybridQueryResponse{Template: "canonical_records", Records: map[string]any{"total_count": 5}}, 5, "tower records"); got != "" {
			t.Fatalf("a complete listing keeps today's sentence, got %q", got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})
}
