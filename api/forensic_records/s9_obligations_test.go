package main

import (
	"strings"
	"testing"
)

func frameFor(t *testing.T, question string) SemanticFrameV1 {
	t.Helper()
	return extractSemanticFrame(hybridQueryRequest{Query: question, TenantID: "t", CollectionID: "c",
		QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)}})
}

func countPlan(filters ...SourceNativeFilterV1) *SourceNativePlanV1 {
	return &SourceNativePlanV1{
		ContractVersion: sourceNativePlanContractV1,
		Filters:         filters,
		Measures:        []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT"}},
	}
}

// H12: "average beam width" answered with a row count.
func TestAverageQuestionCannotBeAnsweredByCounting(t *testing.T) {
	question := "What is the average beam width of the towers?"
	err := verifySourceNativePlanShape(frameFor(t, question), question, countPlan())
	if err == nil {
		t.Fatal("an average question answered by COUNT must be refused")
	}
	if !strings.Contains(err.Error(), "average") {
		t.Errorf("the refusal must name the mismatch: %v", err)
	}
}

// THE CONTROL. A frame measure of max/min also comes from superlatives like
// "most", where the correct plan is GROUP BY + ORDER BY + LIMIT, not a MAX
// aggregate. Four CORRECT questions look exactly like this.
func TestSuperlativeRankingsAreNotRefused(t *testing.T) {
	for _, question := range []string{
		"Which phone number made the most calls?",
		"Which subscriber has the most internet sessions?",
		"Which vehicle was seen most often?",
	} {
		frame := frameFor(t, question)
		plan := &SourceNativePlanV1{
			ContractVersion: sourceNativePlanContractV1,
			GroupFields:     []string{"cdr.msisdn"},
			Measures:        []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT"}},
			Sort:            []SourceNativeSortV1{{Target: "m1", Direction: "DESC"}},
			Limit:           1,
		}
		if err := verifySourceNativePlanShape(frame, question, plan); err != nil {
			t.Errorf("%q is a ranking expressed as COUNT+ORDER+LIMIT and must not be refused: %v",
				question, err)
		}
	}
}

// H7: the layer declares status "500" with the synonym "server error" and the
// plan filtered on nothing, so the answer covered all 1,000 rows.
func TestNamedRestrictionMustBeFiltered(t *testing.T) {
	question := "How many server errors are in the access log?"
	err := verifySourceNativePlanShape(frameFor(t, question), question, countPlan())
	if err == nil {
		t.Fatal("a named restriction the plan never applied must be refused")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("the refusal must name the dropped restriction: %v", err)
	}
}

// ...and is satisfied once the plan actually filters on it.
func TestNamedRestrictionSatisfiedByTheFilter(t *testing.T) {
	question := "How many server errors are in the access log?"
	plan := countPlan(SourceNativeFilterV1{FieldID: "access_log.status", Op: "EQ", Value: "500"})
	if err := verifySourceNativePlanShape(frameFor(t, question), question, plan); err != nil {
		t.Fatalf("the plan filters on 500; nothing was dropped: %v", err)
	}
}

// THE CONTROL THAT MATTERS MOST. A single-word value name is not a restriction:
// "how many CDR records are there?" names CALL merely by containing "call", and
// seven CORRECT questions would be refused if that counted.
func TestSingleWordValueNamesAreNotRestrictions(t *testing.T) {
	for _, question := range []string{
		"How many CDR records do we have in this case?",
		"What was the longest call?",
		"How many call records are from August 2026?",
		"Show the call type breakdown",
	} {
		if err := verifySourceNativePlanShape(frameFor(t, question), question, countPlan()); err != nil {
			if strings.Contains(err.Error(), "does not filter") {
				t.Errorf("%q: a single-word value name is not a restriction: %v", question, err)
			}
		}
	}
}
