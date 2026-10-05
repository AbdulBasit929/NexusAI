package main

import (
	"strings"
	"testing"
)

func TestLocationObligation(t *testing.T) {
	// H11's recorded IR plan: filter on the number, location IS NOT NULL, COUNT.
	h11Plan := &SourceNativePlanV1{
		Filters: []SourceNativeFilterV1{
			{FieldID: "cdr.originating_number", Op: "EQ", Value: "923001110001"},
			{FieldID: "cdr.location", Op: "IS_NOT_NULL"},
		},
		Measures: []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT"}},
	}
	// NEG-02's recorded plan: grouped by location. CORRECT today; must stay.
	neg02Plan := &SourceNativePlanV1{
		GroupFields: []string{"anpr.location"},
		Measures:    []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT"}},
		Sort:        []SourceNativeSortV1{{Target: "m1", Direction: "DESC"}},
	}
	listing := &SourceNativePlanV1{Project: []string{"cdr.location", "cdr.start_time"}}
	h11 := "Where was 923001110001 seen according to the call records?"

	t.Run("switch off changes nothing", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		t.Setenv(locationObligationEnv, "")
		if err := verifySourceNativePlanShape(SemanticFrameV1{Goal: "lookup"}, h11, h11Plan); err != nil {
			t.Fatalf("rule fired with the switch off: %v", err) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})

	t.Run("switch on", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		t.Setenv(locationObligationEnv, "true")
		err := verifySourceNativePlanShape(SemanticFrameV1{Goal: "lookup"}, h11, h11Plan)
		if err == nil || !strings.HasPrefix(err.Error(), s9ShapeViolation) {
			t.Fatalf("H11's bare count must be an S9 shape violation, got %v", err) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
		spared := map[string]struct {
			question string
			plan     *SourceNativePlanV1
		}{
			"NEG-02 grouped by location":      {"Where was plate ZZZ-0000 seen?", neg02Plan},
			"a WHERE question listing rows":   {h11, listing},
			"a count that is not asked WHERE": {"How many calls did 923001110001 make?", h11Plan},
			"'where' not as the opening word": {"Show calls where the duration is zero", h11Plan},
			"a WHERE question with no plan":   {h11, nil},
		}
		for name, c := range spared {
			if locationObligationUnmet(c.question, c.plan) {
				t.Errorf("%s: rule must not fire", name) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
			}
		}
	})

	t.Run("interrogative detection", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		for _, question := range []string{"Where is tower PK-LHR-SYN-001 located?", "  where was it seen", "And where did he go?", "So where were the calls made?"} {
			if !questionAsksWhere(question) {
				t.Errorf("%q asks where", question) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
			}
		}
		for _, question := range []string{"Show calls where the duration is zero", "Nowhere to be found", "Whereabouts of the plate", "Which cell site handled the most calls?"} {
			if questionAsksWhere(question) {
				t.Errorf("%q does not open by asking where", question) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
			}
		}
	})
}
