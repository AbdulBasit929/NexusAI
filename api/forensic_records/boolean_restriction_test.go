package main

import (
	"sort"
	"strings"
	"testing"
)

// CENSUS FIRST. Before the rule exists, list every question in every corpus
// that names a curated BOOLEAN field by a declared multi-word phrase. That is
// the exact set the rule can affect, and it is small enough to read.
//
// Building the rule first and measuring after is how the ladder-deletion slice
// lost three correct answers before anyone knew the risk surface.
func TestBooleanRestrictionCensus(t *testing.T) {
	t.Setenv(mediaFamilyRoutingEnv, "true")
	layer, _ := defaultSemanticLayer()
	if layer == nil {
		t.Skip("curated layer unavailable")
	}
	paths := []string{
		"../../evaluation/golden_questions_v2.json",
		"../../evaluation/holdout_questions_v1.json",
		"../../evaluation/holdout_media_v1.json",
	}
	total := 0
	for _, path := range paths {
		hits := []string{}
		for _, q := range loadRoutingQuestions(t, path) {
			named := questionNamesBooleanRestriction(q.Q)
			if len(named) == 0 {
				continue
			}
			sort.Strings(named)
			hits = append(hits, q.ID+"  ->  "+strings.Join(named, ", ")+"\n        "+q.Q)
		}
		t.Logf("")
		t.Logf("%s : %d question(s) name a curated BOOLEAN field", path, len(hits))
		for _, line := range hits {
			t.Logf("   %s", line)
		}
		total += len(hits)
	}
	t.Logf("")
	t.Logf("TOTAL RISK SURFACE: %d questions across all three corpora", total)
}

// OFF BY DEFAULT, and off it changes nothing.
func TestBooleanRestrictionIsOffByDefault(t *testing.T) {
	t.Setenv(booleanRestrictionEnv, "")
	if booleanRestrictionEnabled() {
		t.Fatal("the switch defaults on")
	}
}

// THE TWO CONFIDENT-WRONG ANSWERS ARE REFUSED. Reproduces the plans Step 4
// actually produced: a bare COUNT with filters null.
func TestBooleanRestrictionRefusesTheDroppedFilter(t *testing.T) {
	t.Setenv(mediaFamilyRoutingEnv, "true")
	plan := &SourceNativePlanV1{
		ContractVersion: sourceNativePlanContractV1,
		Project:         []string{}, Filters: nil, GroupFields: []string{},
		Measures: []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT"}},
		Having:   []SourceNativeHavingV1{}, Sort: []SourceNativeSortV1{}, Limit: 20,
	}
	for _, tc := range []struct{ id, question, want string }{
		{"M12", "How many image text regions require manual review?", "manual_review_required"},
		{"M7", "How many plate groups used persistent object tracking?", "persistent_tracking"},
		{"M3", "How many plate reads still need manual review?", "manual_review_required"},
	} {
		unused := unusedBooleanRestrictions(tc.question, plan)
		if len(unused) == 0 {
			t.Errorf("%s: the dropped restriction was not detected", tc.id)
			continue
		}
		if unused[0] != tc.want {
			t.Errorf("%s: detected %v, want %s", tc.id, unused, tc.want)
		}
	}
	t.Log("M12 and M7 were confident-wrong (309 for 46, 24 for 0); M3 was right by coincidence " +
		"because manual_review_required is true on all 307 rows")
}

// A PLAN THAT HONOURS THE RESTRICTION IS NOT REFUSED — by filtering, or by
// grouping, because a breakdown that reports both sides dropped nothing.
func TestBooleanRestrictionAcceptsAnHonouredPlan(t *testing.T) {
	t.Setenv(mediaFamilyRoutingEnv, "true")
	question := "How many image text regions require manual review?"

	filtered := &SourceNativePlanV1{
		ContractVersion: sourceNativePlanContractV1,
		Filters: []SourceNativeFilterV1{
			{FieldID: "image_ocr_observation.manual_review_required", Op: "EQ", Value: "true"},
		},
		Measures: []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT"}},
	}
	if unused := unusedBooleanRestrictions(question, filtered); len(unused) > 0 {
		t.Errorf("a plan that FILTERS on the restriction was refused: %v", unused)
	}

	grouped := &SourceNativePlanV1{
		ContractVersion: sourceNativePlanContractV1,
		GroupFields:     []string{"image_ocr_observation.manual_review_required"},
		Measures:        []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT"}},
	}
	if unused := unusedBooleanRestrictions(question, grouped); len(unused) > 0 {
		t.Errorf("a plan that GROUPS by the restriction was refused: %v", unused)
	}

	// A DIFFERENT entity's copy of the same bare name still counts, because the
	// question names a restriction and the binding owns which entity holds it.
	otherFamily := &SourceNativePlanV1{
		ContractVersion: sourceNativePlanContractV1,
		Filters: []SourceNativeFilterV1{
			{FieldID: "anpr_model_observation.manual_review_required", Op: "EQ", Value: "true"},
		},
		Measures: []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT"}},
	}
	if unused := unusedBooleanRestrictions(question, otherFamily); len(unused) > 0 {
		t.Errorf("a plan filtering another entity's copy of the same field was refused: %v", unused)
	}
}

// INERT ON STRUCTURED EVIDENCE. The census says zero structured questions name a
// curated boolean field; this asserts it so a future curation cannot change it
// silently. A structured regression here is the TWR-02 false negative returning.
func TestBooleanRestrictionNeverTouchesTheStructuredCorpora(t *testing.T) {
	t.Setenv(mediaFamilyRoutingEnv, "true")
	bare := &SourceNativePlanV1{
		ContractVersion: sourceNativePlanContractV1,
		Measures:        []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT"}},
	}
	for _, path := range []string{
		"../../evaluation/golden_questions_v2.json",
		"../../evaluation/holdout_questions_v1.json",
	} {
		for _, q := range loadRoutingQuestions(t, path) {
			if unused := unusedBooleanRestrictions(q.Q, bare); len(unused) > 0 {
				t.Errorf("%s would be refused over %v: %q", q.ID, unused, q.Q)
			}
		}
	}
}

// A FIELD THE PLAN COULD NOT FILTER IS NEVER HELD AGAINST IT. Refusing a plan
// for failing to do something the layer forbids would be a false negative with
// no remedy available to the model.
func TestBooleanRestrictionIgnoresUnfilterableFields(t *testing.T) {
	if fieldCanBeFiltered(SemanticLayerFieldV1{AllowedFilters: []string{}}) {
		t.Error("a field with no allowed filters was treated as filterable")
	}
	if fieldCanBeFiltered(SemanticLayerFieldV1{AllowedFilters: []string{"RANGE"}}) {
		t.Error("a field allowing only RANGE was treated as equality-filterable")
	}
	if !fieldCanBeFiltered(SemanticLayerFieldV1{AllowedFilters: []string{"EQ", "NEQ"}}) {
		t.Error("an EQ-filterable field was treated as unfilterable")
	}
}
