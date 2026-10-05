package main

import (
	"sort"
	"testing"
)

// A3.1 CENSUS. Which corpus questions does the `source_rows` goal capture today,
// and which of those ask for a QUANTITY -- the ones the fix would release to the
// aggregate case?
//
// CDR-16, "How many CDR records came from each source file?", contains "source"
// and "records", so `semanticFrameGoal` returns source_rows and the compiler
// projects twenty raw rows where a count per file was asked. The grouping hint
// is already right ("source"/"file" -> group by source); only the goal is wrong.
//
// Pure text classification, the same function the pipeline calls, so this
// census skips no pipeline stage.
func TestSourceRowsGoalCensus(t *testing.T) {
	paths := []string{
		"../../evaluation/golden_questions_v2.json",
		"../../evaluation/holdout_questions_v1.json",
		"../../evaluation/holdout_media_v1.json",
	}
	captured, released := []string{}, []string{}
	for _, path := range paths {
		for _, question := range loadRoutingQuestions(t, path) {
			t.Setenv(sourceRowsQuantityGuardEnv, "")
			before := semanticFrameGoal(semanticFrameTerms(question.Q), question.Q)
			if before != "source_rows" {
				continue
			}
			t.Setenv(sourceRowsQuantityGuardEnv, "true")
			after := semanticFrameGoal(semanticFrameTerms(question.Q), question.Q)
			line := question.ID + "  " + before + " -> " + after + "  " + question.Q
			captured = append(captured, line)
			if after != before {
				released = append(released, line)
			}
		}
	}
	sort.Strings(captured)
	sort.Strings(released)
	t.Logf("source_rows captures %d corpus question(s) today:", len(captured)) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	for _, line := range captured {
		t.Logf("   %s", line) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	t.Logf("the quantity guard would RELEASE %d of them:", len(released)) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	for _, line := range released {
		t.Logf("   %s", line) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}

func TestSourceRowsQuantityGuard(t *testing.T) {
	t.Setenv(sourceRowsQuantityGuardEnv, "true")
	cases := map[string]string{
		// THE DEFECT: a count per source file is an aggregate with a grouping.
		"How many CDR records came from each source file?": "aggregate",
		// A request for the raw rows stays source_rows.
		"Show me the source rows for this file": "source_rows",
		"List the source records":               "source_rows",
		"Which source records mention LHR-2026": "source_rows",
	}
	for question, want := range cases {
		if got := semanticFrameGoal(semanticFrameTerms(question), question); got != want {
			t.Errorf("%q -> %q, want %q", question, got, want) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}

	t.Setenv(sourceRowsQuantityGuardEnv, "")
	defect := "How many CDR records came from each source file?"
	if got := semanticFrameGoal(semanticFrameTerms(defect), defect); got != "source_rows" {
		t.Fatalf("with the switch off the classifier must be unchanged (source_rows), got %q", got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}
