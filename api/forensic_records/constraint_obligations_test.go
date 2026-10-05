package main

import (
	"sort"
	"strings"
	"testing"
)

// CENSUS FIRST. What does the magnitude-condition detector capture across every
// corpus, and what does it disturb?
//
// This prints what WOULD change. It wires nothing. Slice B of the goal taxonomy
// moved 5 questions and disturbed 0 because it was censused before it was
// wired; WI-31 skipped this step and cost a correct answer.
func TestConstraintObligationCensus(t *testing.T) {
	paths := []string{
		"../../evaluation/golden_questions_v2.json",
		"../../evaluation/holdout_questions_v1.json",
		"../../evaluation/holdout_media_v1.json",
	}
	captured := []string{}
	total := 0
	for _, path := range paths {
		for _, question := range loadRoutingQuestions(t, path) {
			total++
			if condition := questionStatesMagnitudeCondition(question.Q); condition != "" {
				captured = append(captured, question.ID+"  ["+condition+"]  "+question.Q)
			}
		}
	}
	sort.Strings(captured)
	t.Logf("MAGNITUDE-CONDITION CENSUS across %d corpus questions", total) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	t.Logf("captured: %d", len(captured)) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	for _, item := range captured {
		t.Logf("   %s", item) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	if len(captured) == 0 {
		t.Logf("   (none -- the corpora contain no magnitude condition, so this guard") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		t.Logf("    cannot disturb a single existing answer. It exists for TYPED questions.)") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}

// The detector must fire on the measured defects and stay silent on the
// phrasings that already work. Each negative here is a real corpus question or
// a real shape that must not be captured.
func TestMagnitudeConditionDetector(t *testing.T) {
	fires := map[string]string{
		"Show me all the calls that lasted longer than ten minutes": "longer than ten minutes",
		"calls longer than 10 minutes":                              "longer than 10 minutes",
		"Which sessions transferred more than 1000 bytes?":          "more than 1000 bytes",
		"transactions above 50000":                                  "above 50000",
		"sessions lasting at least 30 seconds":                      "at least 30 seconds",
		"records between 100 and 200":                               "between 100",
	}
	for query, expected := range fires {
		if got := questionStatesMagnitudeCondition(query); got != expected {
			t.Errorf("detector on %q\n  got      %q\n  expected %q", query, got, expected) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}

	// SUPERLATIVES ARE NOT CONDITIONS. These already resolve as `rank` and are
	// CORRECT today; capturing one would be the WI-31 defect in a new place.
	silent := []string{
		"What was the longest call?",
		"Which phone number made the most calls?",
		"What is the largest network volume on any call record?",
		"How many calls of each type are there?",
		"Which cell site handled the most calls?",
		"How many CDR records do we have in this case?",
		"Which document mentions contact number 03001234567?",
		"How many times was plate LHR-2026 seen?",
		// A bare comparative with no number is a superlative phrasing, not a
		// bound: there is nothing to compare against.
		"Show me the longer calls",
		"Which sessions were larger?",
	}
	for _, query := range silent {
		if got := questionStatesMagnitudeCondition(query); got != "" {
			t.Errorf("detector must stay silent on %q, captured %q", query, got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
}

// A plan that compares against nothing does not satisfy a stated condition; a
// plan that compares does. EQ is explicitly not enough.
func TestPlanBindsARangeComparison(t *testing.T) {
	if planBindsARangeComparison(nil) {
		t.Error("a nil plan binds nothing") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	empty := &SourceNativePlanV1{Filters: []SourceNativeFilterV1{}}
	if planBindsARangeComparison(empty) {
		t.Error("an empty filter list binds no comparison -- this is the measured defect") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	equality := &SourceNativePlanV1{Filters: []SourceNativeFilterV1{{FieldID: "cdr.call_type", Op: "EQ", Value: "SMS"}}}
	if planBindsARangeComparison(equality) {
		t.Error("EQ is not a magnitude comparison: it binds a value, not a bound") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	for _, op := range []string{"GT", "GTE", "LT", "LTE", "BETWEEN", "gt"} {
		bound := &SourceNativePlanV1{Filters: []SourceNativeFilterV1{{FieldID: "cdr.duration_seconds", Op: op, Value: "600"}}}
		if !planBindsARangeComparison(bound) {
			t.Errorf("op %q must count as a magnitude comparison", op) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
}

// The guard is INERT while its switch is off, and refuses only the measured
// shape when it is on.
func TestAnswerIgnoresStatedConditionRespectsItsSwitch(t *testing.T) {
	req := hybridQueryRequest{
		Query:        "Show me all the calls that lasted longer than ten minutes",
		SourceNative: &SourceNativePlanV1{Filters: []SourceNativeFilterV1{}},
	}

	t.Setenv(constraintObligationsEnv, "")
	if answerIgnoresStatedCondition(req) {
		t.Fatal("the guard must be inert with the switch unset; every arm downstream is void otherwise") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}

	t.Setenv(constraintObligationsEnv, "true")
	if !answerIgnoresStatedCondition(req) {
		t.Fatal("the guard must fire on a stated condition the plan never bound") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}

	bound := req
	bound.SourceNative = &SourceNativePlanV1{Filters: []SourceNativeFilterV1{{FieldID: "cdr.duration_seconds", Op: "GT", Value: "600"}}}
	if answerIgnoresStatedCondition(bound) {
		t.Fatal("a plan that DOES compare must pass untouched") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}

	noPlan := req
	noPlan.SourceNative = nil
	if answerIgnoresStatedCondition(noPlan) {
		t.Fatal("with no typed plan there is nothing to inspect; another guard covers that case") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}

	noCondition := req
	noCondition.Query = "How many CDR records do we have in this case?"
	if answerIgnoresStatedCondition(noCondition) {
		t.Fatal("a question stating no condition must never be withheld by this guard") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}

// The refusal must NAME the condition. A refusal that does not say what was
// dropped leaves the analyst unable to correct it.
func TestConditionNotAppliedReasonNamesTheCondition(t *testing.T) {
	t.Setenv(constraintObligationsEnv, "true")
	req := hybridQueryRequest{
		Query:        "Show me all the calls that lasted longer than ten minutes",
		SourceNative: &SourceNativePlanV1{Filters: []SourceNativeFilterV1{}},
	}
	reason := conditionNotAppliedReason(req)
	if reason.Code != withholdConditionNotApplied {
		t.Errorf("code = %q", reason.Code) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	if !strings.Contains(reason.Question, "longer than ten minutes") {
		t.Errorf("the analyst sentence must repeat the condition, got %q", reason.Question) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	if !strings.Contains(reason.Guardrail, "longer than ten minutes") {
		t.Errorf("the guardrail must name the condition, got %q", reason.Guardrail) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}
