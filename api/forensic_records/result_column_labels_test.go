package main

import (
	"reflect"
	"testing"
)

func TestSourceNativeColumnLabels(t *testing.T) {
	catalog := []FieldDescriptorV1{
		{FieldID: "cdr.msisdn", NormalizedName: "msisdn", DisplayName: "Subscriber number"},
		{FieldID: "cdr.call_duration_seconds", NormalizedName: "call_duration_seconds", DisplayName: "Call duration"},
	}
	plan := &SourceNativePlanV1{
		ContractVersion: sourceNativePlanContractV1,
		GroupFields:     []string{"cdr.msisdn"},
		Measures: []SourceNativeMeasureV1{
			{MeasureID: "m1", Op: "COUNT"},
			{MeasureID: "m2", Op: "AVG", FieldID: "cdr.call_duration_seconds"},
		},
	}
	rows := []map[string]any{{"msisdn": "923001110001", "m1": 447, "m2": 61.5, "metadata": map[string]any{}, "section": "source_native"}}
	newGrid := func() map[string]any { return enterpriseDataGrid(rows) }
	resp := hybridQueryResponse{Records: map[string]any{"plan": plan, "field_catalog": catalog}}
	req := hybridQueryRequest{RecordType: "cdr"}

	t.Run("switch off leaves the grid exactly as today", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		t.Setenv(resultColumnLabelsEnv, "")
		grid, before := newGrid(), newGrid()
		applyColumnLabels(grid, sourceNativeColumnLabels(req, resp))
		if !reflect.DeepEqual(grid, before) {
			t.Fatal("grid changed with the switch off") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})

	t.Run("switch on names every column, changes no key or value", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		t.Setenv(resultColumnLabelsEnv, "true")
		grid, before := newGrid(), newGrid()
		applyColumnLabels(grid, sourceNativeColumnLabels(req, resp))
		want := map[string]string{
			"m1": "CDR records", "m2": "Average call duration", "msisdn": "Subscriber number",
			"metadata": "Source rows", "section": "Section",
		}
		for _, column := range grid["columns"].([]map[string]any) {
			key := column["key"].(string)
			if w, ok := want[key]; ok && column["header"] != w {
				t.Errorf("%s: header %q, want %q", key, column["header"], w) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
			}
		}
		if !reflect.DeepEqual(grid["rows"], before["rows"]) || grid["count"] != before["count"] {
			t.Fatal("rows or count changed; only headers may change") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
		keys := func(g map[string]any) []string {
			out := []string{}
			for _, column := range g["columns"].([]map[string]any) {
				out = append(out, column["key"].(string))
			}
			return out
		}
		if !reflect.DeepEqual(keys(grid), keys(before)) {
			t.Fatal("column keys or order changed; only headers may change") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})

	t.Run("an acronym at the start of a field name is never broken", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		t.Setenv(resultColumnLabelsEnv, "true")
		acronym := hybridQueryResponse{Records: map[string]any{
			"plan": &SourceNativePlanV1{ContractVersion: sourceNativePlanContractV1,
				Measures: []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT", FieldID: "access_log.status"}}},
			"field_catalog": []FieldDescriptorV1{{FieldID: "access_log.status", NormalizedName: "status", DisplayName: "HTTP status code"}},
		}}
		if got := sourceNativeColumnLabels(req, acronym)["m1"]; got != "Count of HTTP status code" {
			t.Fatalf("label = %q, want %q (measured: the first version wrote \"hTTP\")", got, "Count of HTTP status code") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})

	t.Run("no typed plan, no change", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		t.Setenv(resultColumnLabelsEnv, "true")
		if labels := sourceNativeColumnLabels(req, hybridQueryResponse{Records: map[string]any{}}); labels != nil {
			t.Fatalf("labels without a plan: %v", labels) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})
}
