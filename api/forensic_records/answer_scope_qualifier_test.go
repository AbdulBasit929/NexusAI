package main

import (
	"strings"
	"testing"
)

// A1b.1 — the headline must name the range bound that shaped the result.
//
// Measured 2026-09-27 with A1b on: "How many transactions have an amount above
// 50000?" answered "There is 1 transaction in this case." The count is correct
// and the case holds FOUR transactions, so read alone the sentence is false.
func TestAnswerScopeQualifierNamesRangeBounds(t *testing.T) {
	req := hybridQueryRequest{
		SourceNative: &SourceNativePlanV1{Filters: []SourceNativeFilterV1{
			{FieldID: "transaction.amount", Op: "GT", Value: "50000"},
		}},
	}

	t.Run("silent while the switch is off", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		t.Setenv(rangeFiltersEnv, "")
		if got := answerScopeQualifier(req); got != "" {
			t.Fatalf("must be inert with the switch off, got %q", got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})

	t.Run("names the bound when the switch is on", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		t.Setenv(rangeFiltersEnv, "true")
		got := answerScopeQualifier(req)
		if !strings.Contains(got, "amount") || !strings.Contains(got, "above") || !strings.Contains(got, "50000") {
			t.Fatalf("the qualifier must name field, comparison and value, got %q", got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})

	t.Run("reads correctly in the headline", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		t.Setenv(rangeFiltersEnv, "true")
		sentence := "There is 1 transaction" + answerScopeQualifier(req) + " in this case."
		want := "There is 1 transaction with amount above 50000 in this case."
		if sentence != want {
			t.Fatalf("\n  got  %q\n  want %q", sentence, want) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})

	t.Run("a target and a bound both appear", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		t.Setenv(rangeFiltersEnv, "true")
		both := req
		both.Target = "ACCT-778812"
		got := answerScopeQualifier(both)
		if !strings.Contains(got, "involving ACCT-778812") || !strings.Contains(got, "above 50000") {
			t.Fatalf("both constraints must be named, got %q", got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})
}

func TestRangeComparisonWording(t *testing.T) {
	for op, want := range map[string]string{
		"GT": "above", "GTE": "of at least", "LT": "below", "LTE": "of at most",
		"gt": "above",
		// EQ, CONTAINS and IN are not range bounds and must not be described as
		// one; a value filter is already named by the target qualifier.
		"EQ": "", "CONTAINS": "", "IN": "", "BETWEEN": "", "": "",
	} {
		if got := rangeComparisonWording(op); got != want {
			t.Errorf("rangeComparisonWording(%q) = %q, want %q", op, got, want) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
}

// An empty value must never produce "with amount above " -- a dangling
// qualifier is worse than none.
func TestRangeFilterQualifiersSkipEmptyValues(t *testing.T) {
	t.Setenv(rangeFiltersEnv, "true")
	req := hybridQueryRequest{SourceNative: &SourceNativePlanV1{Filters: []SourceNativeFilterV1{
		{FieldID: "transaction.amount", Op: "GT", Value: "   "},
		{FieldID: "transaction.account", Op: "EQ", Value: "ACCT-1"},
	}}}
	if got := rangeFilterQualifiers(req); len(got) != 0 {
		t.Fatalf("expected no qualifier, got %v", got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}
