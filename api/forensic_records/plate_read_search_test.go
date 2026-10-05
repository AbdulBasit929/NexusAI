package main

import (
	"strings"
	"testing"
)

func TestAnalystPlateLiterals(t *testing.T) {
	cases := map[string][]string{
		"Which image shows plate MN1367?":         {"MN1367"},
		"Is car BCX-567 in any photo?":            {"BCX567"}, // stored without the hyphen
		"Is the number 03001234567 in any video?": nil,        // digits only: a phone, not a plate
		"Which plates were read from the videos?": nil,
		// A file name is not a plate: no search, so no nonsense "no reads of DSC1105JPG".
		"Which plates appear in DSC_1105.JPG?": nil,
	}
	for question, want := range cases {
		got := analystPlateLiterals(question)
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("%q: got %v, want %v", question, got, want) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
}

func TestPlateReadSearchIsSearchOnly(t *testing.T) {
	layer, _ := defaultSemanticLayer()
	if layer == nil {
		t.Skip("semantic layer not available") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	entity, ok := layer.EntityByFamily(plateReadFamily)
	if !ok {
		t.Skip("plate-read entity not curated") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	plateField := func(catalog []FieldDescriptorV1) (FieldDescriptorV1, bool) {
		for _, field := range catalog {
			if field.FieldID == plateReadTextField {
				return field, true
			}
		}
		return FieldDescriptorV1{}, false
	}

	t.Run("switch off: the PII field is never issued", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		t.Setenv(plateSearchOnlyEnv, "")
		if _, found := plateField(entity.CatalogFields(hybridQueryRequest{Query: "Which image shows plate MN1367?"})); found {
			t.Fatal("plate text issued with the switch off") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})

	t.Run("switch on", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		t.Setenv(plateSearchOnlyEnv, "true")
		// No plate supplied: the privacy probes' shape. Never issued.
		for _, question := range []string{"Which plates were read from the videos?", "What license plates appear in the videos?"} {
			if _, found := plateField(entity.CatalogFields(hybridQueryRequest{Query: question})); found {
				t.Fatalf("%q names no plate; plate text must not be issued", question) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
			}
		}
		catalog := entity.CatalogFields(hybridQueryRequest{Query: "Which image shows plate MN1367?"})
		field, found := plateField(catalog)
		if !found {
			t.Fatal("a supplied plate must make the field available as a filter") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
		if field.Projectable || field.Groupable || field.Sortable || len(field.AllowedAggregates) != 0 || field.RedactionState != "FILTER_ONLY" {
			t.Fatalf("the field must be filter-only: %+v", field) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
		for _, op := range field.AllowedFilters {
			if op != "EQ" && op != "IN" {
				t.Fatalf("only exact filters may be allowed, found %s", op) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
			}
		}
		// The validator must refuse every way of listing or counting plates.
		count := []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT"}}
		leaks := map[string]*SourceNativePlanV1{
			"project the plate":  {ContractVersion: sourceNativePlanContractV1, Project: []string{plateReadTextField}},
			"group by the plate": {ContractVersion: sourceNativePlanContractV1, GroupFields: []string{plateReadTextField}, Measures: count},
			"count distinct plates": {ContractVersion: sourceNativePlanContractV1,
				Measures: []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT_DISTINCT", FieldID: plateReadTextField}}},
			"a partial-text filter": {ContractVersion: sourceNativePlanContractV1, Measures: count,
				Filters: []SourceNativeFilterV1{{FieldID: plateReadTextField, Op: "CONTAINS", Value: "MN"}}},
		}
		for name, plan := range leaks {
			if err := validateSourceNativePlan(plan, catalog); err == nil {
				t.Errorf("%s: the validator must reject it", name) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
			}
		}
		searched, ok := plateReadSearchRequest(hybridQueryRequest{Query: "Which image shows plate MN1367?"})
		if !ok || searched.SourceNative == nil || searched.Target != "MN1367" {
			t.Fatalf("a qualifying question must become a verified plate-read count: ok=%v %+v", ok, searched.SourceNative) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
		if len(searched.SourceNative.Project) != 0 || len(searched.SourceNative.GroupFields) != 0 {
			t.Fatal("the search plan must not project or group") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})
}

func TestPlateReadSearchNote(t *testing.T) {
	t.Setenv(plateSearchOnlyEnv, "true")
	plan := &SourceNativePlanV1{Filters: []SourceNativeFilterV1{{FieldID: plateReadTextField, Op: "EQ", Value: "MN1367"}}}
	resp := hybridQueryResponse{Records: map[string]any{"provenance": []map[string]any{
		{"artifact_type": "forensics.anpr-observation/v1", "citation_locator": map[string]any{"source_file": "image-test-plate-test_plate.jpg"}},
	}}}
	found := plateReadSearchNote(resp, plan, 1)
	if !strings.Contains(found, "image-test-plate-test_plate.jpg") || !strings.Contains(found, "model reads") {
		t.Fatalf("a match must name its file and say it is a model read: %q", found) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	none := plateReadSearchNote(hybridQueryResponse{}, plan, 0)
	if !strings.Contains(none, "misread") || strings.Contains(none, "absent from the images.") && !strings.Contains(none, "does not show") {
		t.Fatalf("no match must not be stated as absence: %q", none) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	if plateReadSearchNote(resp, &SourceNativePlanV1{}, 1) != "" {
		t.Fatal("any other plan must get no note") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}

// A search with no match has no provenance to label from; it must still say
// it searched plate reads, never "no ANPR sightings" (camera records).
func TestPlateReadSearchZeroMatchNamesPlateReads(t *testing.T) {
	t.Setenv(plateSearchOnlyEnv, "true")
	plan := &SourceNativePlanV1{ContractVersion: sourceNativePlanContractV1,
		Filters:  []SourceNativeFilterV1{{FieldID: plateReadTextField, Op: "EQ", Value: "ZZ9999", Values: []string{"ZZ9999"}}},
		Measures: []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT"}}}
	resp := hybridQueryResponse{Records: map[string]any{"source_native_results": []map[string]any{{"m1": 0}}}}
	answer, ok := sourceNativeResultAnswer(hybridQueryRequest{RecordType: "anpr", Target: "ZZ9999"}, resp, plan)
	if !ok || !strings.Contains(answer.Headline, "no plate reads") || strings.Contains(answer.Headline, "sighting") && !strings.Contains(answer.Headline, "not camera sightings") {
		t.Fatalf("zero-match headline must name plate reads: %q", answer.Headline) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	if !strings.Contains(answer.Headline, "misread") {
		t.Fatalf("zero-match headline must not read as absence: %q", answer.Headline) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}
