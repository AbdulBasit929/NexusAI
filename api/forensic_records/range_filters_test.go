package main

import (
	"sort"
	"testing"
)

// CENSUS FIRST. What would A1b capture across every corpus, and what would it
// disturb? Printed before it is trusted, exactly as the goal-taxonomy slices
// were.
func TestRangeFilterCensus(t *testing.T) {
	paths := []string{
		"../../evaluation/golden_questions_v2.json",
		"../../evaluation/holdout_questions_v1.json",
		"../../evaluation/holdout_media_v1.json",
	}
	captured, unitBlocked := []string{}, []string{}
	total := 0
	for _, path := range paths {
		for _, question := range loadRoutingQuestions(t, path) {
			total++
			comparison, ok := questionStatedComparison(question.Q)
			if !ok {
				continue
			}
			line := question.ID + "  " + comparison.Op + " " + comparison.Literal + "  " + question.Q
			if comparison.Unit != "" {
				unitBlocked = append(unitBlocked, line+"   [unit: "+comparison.Unit+"]")
				continue
			}
			captured = append(captured, line)
		}
	}
	sort.Strings(captured)
	sort.Strings(unitBlocked)
	t.Logf("RANGE-FILTER CENSUS across %d corpus questions", total) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	t.Logf("would bind a filter: %d", len(captured)) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	for _, item := range captured {
		t.Logf("   %s", item) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	t.Logf("stated a bound but carry a UNIT, so bind nothing and stay withheld: %d", len(unitBlocked)) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	for _, item := range unitBlocked {
		t.Logf("   %s", item) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}

func TestQuestionStatedComparison(t *testing.T) {
	cases := []struct {
		query   string
		ok      bool
		op      string
		literal string
		unit    string
	}{
		{"transactions above 50000", true, "GT", "50000", ""},
		{"sessions with more than 1000000 bytes", true, "GT", "1000000", "bytes"},
		{"plate reads with confidence above 0.9", true, "GT", "0.9", ""},
		{"groups with at least 5 sightings", true, "GTE", "5", "sightings"},
		{"transactions under 1,000", true, "LT", "1000", ""},
		{"records at most 20", true, "LTE", "20", ""},
		// A unit word is captured so the caller can refuse: no filterable field
		// in the layer declares a unit to convert against.
		{"calls longer than 10 minutes", true, "GT", "10", "minutes"},
		// Two bounds is a range, which this item does not express. Binding one
		// half would answer a different question than the one asked.
		{"calls longer than 10 and shorter than 60", false, "", "", ""},
		// No number is a superlative, and `rank` already answers those.
		{"What was the longest call?", false, "", "", ""},
		{"Which phone number made the most calls?", false, "", "", ""},
		{"How many CDR records do we have in this case?", false, "", "", ""},
	}
	for _, tc := range cases {
		got, ok := questionStatedComparison(tc.query)
		if ok != tc.ok {
			t.Errorf("%q: ok = %v, want %v", tc.query, ok, tc.ok) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
			continue
		}
		if !ok {
			continue
		}
		if got.Op != tc.op || got.Literal != tc.literal || got.Unit != tc.unit {
			t.Errorf("%q:\n  got  op=%q literal=%q unit=%q\n  want op=%q literal=%q unit=%q", //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
				tc.query, got.Op, got.Literal, got.Unit, tc.op, tc.literal, tc.unit)
		}
	}
}

func TestRangeOperatorMapping(t *testing.T) {
	for phrase, want := range map[string]string{
		"more than": "GT", "above": "GT", "over": "GT", "exceeding": "GT", "longer than": "GT",
		"less than": "LT", "below": "LT", "under": "LT", "fewer than": "LT", "shorter than": "LT",
		"at least": "GTE", "no fewer than": "GTE", "greater than or equal to": "GTE",
		"at most": "LTE", "no more than": "LTE", "less than or equal to": "LTE",
		"between": "", "roughly": "",
	} {
		if got := rangeOperatorFor(phrase); got != want {
			t.Errorf("rangeOperatorFor(%q) = %q, want %q", phrase, got, want) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
}

// The binder must respect its switch, the layer's `allowed_filters`, the
// requirement that the question NAME the field, and the ambiguity rule.
func TestSemanticLayerRangeFilterBinding(t *testing.T) {
	entity := SemanticLayerEntityV1{
		Fields: []SemanticLayerFieldV1{
			{ID: "transaction.amount", DisplayName: "Amount", Synonyms: []string{"amount", "value"}},
			{ID: "transaction.account", DisplayName: "Account", Synonyms: []string{"account"}},
		},
	}
	catalog := []FieldDescriptorV1{
		{FieldID: "transaction.amount", AllowedFilters: []string{"EQ", "GT", "GTE", "LT", "LTE"}},
		{FieldID: "transaction.account", AllowedFilters: []string{"EQ"}},
	}

	t.Run("binds the named range-filterable field", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		filters := semanticLayerRangeFilters("transactions with amount above 50000", entity, catalog)
		if len(filters) != 1 {
			t.Fatalf("expected exactly one filter, got %d (%+v)", len(filters), filters) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
		if filters[0].FieldID != "transaction.amount" || filters[0].Op != "GT" || filters[0].Value != "50000" {
			t.Fatalf("filter = %+v", filters[0]) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})

	t.Run("binds nothing when the question names no curated field", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		if filters := semanticLayerRangeFilters("show me everything above 50000", entity, catalog); len(filters) != 0 {
			t.Fatalf("a field the analyst did not name must never bind: %+v", filters) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})

	t.Run("binds nothing when the field forbids the operator", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		if filters := semanticLayerRangeFilters("account above 50000", entity, catalog); len(filters) != 0 {
			t.Fatalf("allowed_filters must be honoured, got %+v", filters) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})

	t.Run("binds nothing when a unit word is present", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		if filters := semanticLayerRangeFilters("amount above 50000 rupees", entity, catalog); len(filters) != 0 {
			t.Fatalf("no filterable field declares a unit, so a unit must refuse: %+v", filters) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})

	t.Run("binds nothing when two curated fields match", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		ambiguous := SemanticLayerEntityV1{Fields: []SemanticLayerFieldV1{
			{ID: "transaction.amount", DisplayName: "Amount", Synonyms: []string{"amount"}},
			{ID: "transaction.account", DisplayName: "Amount", Synonyms: []string{"amount"}},
		}}
		catalogBoth := []FieldDescriptorV1{
			{FieldID: "transaction.amount", AllowedFilters: []string{"GT"}},
			{FieldID: "transaction.account", AllowedFilters: []string{"GT"}},
		}
		if filters := semanticLayerRangeFilters("amount above 50000", ambiguous, catalogBoth); len(filters) != 0 {
			t.Fatalf("ambiguity must bind nothing, got %+v", filters) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})

	t.Run("the switch keeps it inert", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		t.Setenv(rangeFiltersEnv, "")
		if out := appendSourceNativeRangeFilters(nil, "transactions with amount above 50000", catalog); len(out) != 0 {
			t.Fatalf("must be inert with the switch off, got %+v", out) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})
}
