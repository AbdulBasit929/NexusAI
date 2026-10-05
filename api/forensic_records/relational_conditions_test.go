package main

import (
	"sort"
	"testing"
)

func TestQuestionStatesRelationalCondition(t *testing.T) {
	positives := map[string]relationalKind{
		// The two measured confident-wrong answers.
		"Do any subscribers share the same handset?":                          relationalShared,
		"Is there any link between the plate sightings and the call records?": relationalBetween,
		// Other explicit forms of the same two kinds.
		"Which numbers shared a tower last week?":                         relationalShared,
		"Are there subscribers sharing one IMEI?":                         relationalShared,
		"What do these two numbers have in common?":                       relationalShared,
		"Show the common contacts between 923001110001 and 923451112233":  relationalBetween,
		"Is there any overlap between the two CDR files?":                 relationalBetween,
		"What is the connection between this plate and the phone number?": relationalBetween,
	}
	for question, want := range positives {
		phrase, kind := questionStatesRelationalCondition(question)
		if kind != want || phrase == "" {
			t.Errorf("%q: got kind=%q phrase=%q, want kind=%q", question, kind, phrase, want) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}

	// The phrase is echoed to the analyst, so it must end on the thing shared.
	// Measured 2026-09-28: a one-word capture echoed "shared a cell".
	echoes := map[string]string{
		"Which phone numbers shared a cell tower?":                    "shared a cell tower",
		"Do any subscribers share the same handset?":                  "share the same handset",
		"Do any subscribers share the same handset across the cases?": "share the same handset",
		"Are there subscribers sharing one IMEI?":                     "sharing one IMEI",
	}
	for question, want := range echoes {
		if phrase, _ := questionStatesRelationalCondition(question); phrase != want {
			t.Errorf("%q: echoed %q, want %q", question, phrase, want) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}

	negatives := []string{
		// "share" as a noun, an adjective, or a request for output.
		"What is the share of SMS calls?",
		"Share a list of all calls",
		"Can you share a summary of the case?",
		"I want you to share the same report as yesterday",
		"What does the note say about the shared account?",
		// A named target is a lookup, policed by the target guards.
		"Which subscribers are linked to 923001110001?",
		// Neighbouring shapes owned by other guards or by rank.
		"How many calls lasted longer than ten minutes?",
		"Which cell site handled the most calls?",
		"How many CDR records came from each source file?",
		"hello",
		"",
	}
	for _, question := range negatives {
		if phrase, kind := questionStatesRelationalCondition(question); kind != relationalNone {
			t.Errorf("%q: must not state a relationship, got kind=%q phrase=%q", question, kind, phrase) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
}

func TestPlanComputesSharedRelationship(t *testing.T) {
	sharedHandset := &SourceNativePlanV1{
		GroupFields: []string{"subscriber.imei"},
		Measures:    []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT_DISTINCT", FieldID: "subscriber.msisdn"}},
		Having:      []SourceNativeHavingV1{{MeasureID: "m1", Op: "GT", Value: "1"}},
	}
	if !planComputesSharedRelationship(sharedHandset) {
		t.Fatal("GROUP BY handset HAVING COUNT_DISTINCT subscriber > 1 computes the relationship") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	cases := map[string]*SourceNativePlanV1{
		"nil plan": nil,
		"count with no grouping": {
			Measures: []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT"}},
			Having:   []SourceNativeHavingV1{{MeasureID: "m1", Op: "GT", Value: "1"}},
		},
		"grouping with no having": {
			GroupFields: []string{"subscriber.imei"},
			Measures:    []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT"}},
		},
		"having on a sum": {
			GroupFields: []string{"subscriber.imei"},
			Measures:    []SourceNativeMeasureV1{{MeasureID: "m1", Op: "SUM", FieldID: "cdr.call_duration_seconds"}},
			Having:      []SourceNativeHavingV1{{MeasureID: "m1", Op: "GT", Value: "1"}},
		},
		"having an upper bound": {
			GroupFields: []string{"subscriber.imei"},
			Measures:    []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT"}},
			Having:      []SourceNativeHavingV1{{MeasureID: "m1", Op: "LT", Value: "2"}},
		},
		"project only": {Project: []string{"subscriber.msisdn"}},
	}
	for name, plan := range cases {
		if planComputesSharedRelationship(plan) {
			t.Errorf("%s: must not count as computing a shared relationship", name) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
}

func TestAnswerIgnoresStatedRelationship(t *testing.T) {
	countOnly := &SourceNativePlanV1{Measures: []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT"}}}
	sharedPlan := &SourceNativePlanV1{
		GroupFields: []string{"subscriber.imei"},
		Measures:    []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT_DISTINCT", FieldID: "subscriber.msisdn"}},
		Having:      []SourceNativeHavingV1{{MeasureID: "m1", Op: "GT", Value: "1"}},
	}
	handset := "Do any subscribers share the same handset?"
	link := "Is there any link between the plate sightings and the call records?"

	t.Run("switch off changes nothing", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		t.Setenv(relationalConditionsEnv, "")
		if answerIgnoresStatedRelationship(hybridQueryRequest{Query: handset, SourceNative: countOnly}, "canonical_records") {
			t.Fatal("guard fired with the switch off") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})

	t.Run("switch on", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		t.Setenv(relationalConditionsEnv, "true")
		fires := map[string]struct {
			req      hybridQueryRequest
			template string
		}{
			"shared, answered by a count":              {hybridQueryRequest{Query: handset, SourceNative: countOnly}, "canonical_records"},
			"shared, answered by a one-family listing": {hybridQueryRequest{Query: handset}, "subscriber_status_summary"},
			"between, answered by a count":             {hybridQueryRequest{Query: link, SourceNative: countOnly}, "canonical_records"},
			// A single-family typed plan cannot span families, whatever its shape.
			"between, answered by a grouped typed plan": {hybridQueryRequest{Query: link, SourceNative: sharedPlan}, "canonical_records"},
		}
		for name, c := range fires {
			if !answerIgnoresStatedRelationship(c.req, c.template) {
				t.Errorf("%s: guard must fire", name) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
			}
		}
		spared := map[string]struct {
			req      hybridQueryRequest
			template string
		}{
			"shared, computed by GROUP BY + HAVING": {hybridQueryRequest{Query: handset, SourceNative: sharedPlan}, "canonical_records"},
			"shared, a relational operation":        {hybridQueryRequest{Query: handset}, "subscriber_device_links"},
			"between, a relational operation":       {hybridQueryRequest{Query: link}, "cross_family_correlation"},
			"no relationship stated":                {hybridQueryRequest{Query: "How many subscribers are there?", SourceNative: countOnly}, "canonical_records"},
		}
		for name, c := range spared {
			if answerIgnoresStatedRelationship(c.req, c.template) {
				t.Errorf("%s: guard must not fire", name) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
			}
		}
		reason := relationshipNotComputedReason(hybridQueryRequest{Query: handset, SourceNative: countOnly}, "canonical_records")
		if reason.Code != withholdRelationshipNotComputed || reason.Question == "" || reason.Guardrail == "" {
			t.Fatalf("incomplete withhold reason: %+v", reason) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})
}

// Census: which corpus questions state a relationship. Recorded 2026-09-28:
// none of the 103, which is why the defect was invisible until an ad-hoc probe
// asked one. Logged rather than asserted, so adding relational questions to the
// corpus -- which it needs -- does not break the build.
func TestRelationalConditionCorpusCensus(t *testing.T) {
	paths := []string{
		"../../evaluation/golden_questions_v2.json",
		"../../evaluation/holdout_questions_v1.json",
		"../../evaluation/holdout_media_v1.json",
	}
	hits := []string{}
	total := 0
	for _, path := range paths {
		for _, question := range loadRoutingQuestions(t, path) {
			total++
			if phrase, kind := questionStatesRelationalCondition(question.Q); kind != relationalNone {
				hits = append(hits, question.ID+"  "+string(kind)+"  "+phrase+"  "+question.Q)
			}
		}
	}
	sort.Strings(hits)
	t.Logf("corpus questions stating a relationship: %d of %d", len(hits), total) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	for _, line := range hits {
		t.Logf("   %s", line) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}
