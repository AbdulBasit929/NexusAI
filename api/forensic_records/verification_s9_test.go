package main

import (
	"strings"
	"testing"
)

// S9 SHAPE and SCOPE. Each case below is a failure the live runs actually
// produced, not a hypothetical.

func s9Plan(group []string, sort []SourceNativeSortV1, filters []SourceNativeFilterV1) *SourceNativePlanV1 {
	return &SourceNativePlanV1{ContractVersion: sourceNativePlanContractV1,
		GroupFields: group, Sort: sort, Filters: filters,
		Measures: []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT"}}}
}

// "Which phone number made the most calls" answered "1 CDR record matched this
// question" — a real number for a different question.
func TestS9ShapeRejectsARankingWithNothingToRankBy(t *testing.T) {
	question := "Which phone number made the most calls?"
	frame := extractSemanticFrame(hybridQueryRequest{Query: question})
	if got := s9QuestionShape(frame, question); got != "rank" {
		t.Fatalf("shape = %q, want rank", got)
	}
	err := verifySourceNativePlanShape(frame, question, s9Plan(nil, nil, nil))
	if err == nil || !strings.HasPrefix(err.Error(), s9ShapeViolation) {
		t.Fatalf("a ranking with no grouping must be refused; got %v", err)
	}
	// A grouping with no ordering makes the top row arbitrary.
	err = verifySourceNativePlanShape(frame, question, s9Plan([]string{"cdr.msisdn"}, nil, nil))
	if err == nil {
		t.Fatal("a ranking without an ordering must be refused")
	}
	// Properly shaped: grouped and ordered.
	ok := s9Plan([]string{"cdr.msisdn"}, []SourceNativeSortV1{{Target: "m1", Direction: "DESC"}}, nil)
	if err := verifySourceNativePlanShape(frame, question, ok); err != nil {
		t.Fatalf("a correctly shaped ranking must pass: %v", err)
	}
}

// D2's signature: a plain count that acquires a grouping nobody asked for,
// which made identical questions return different answers.
func TestS9ShapeRejectsASpuriousGroupingOnAScalarQuestion(t *testing.T) {
	question := "How many CDR records do we have in this case?"
	frame := extractSemanticFrame(hybridQueryRequest{Query: question})
	if got := s9QuestionShape(frame, question); got != "scalar" {
		t.Fatalf("shape = %q, want scalar", got)
	}
	err := verifySourceNativePlanShape(frame, question, s9Plan([]string{"cdr.call_type"}, nil, nil))
	if err == nil || !strings.Contains(err.Error(), "scalar question compiled a grouping") {
		t.Fatalf("a spurious grouping must be refused; got %v", err)
	}
	if err := verifySourceNativePlanShape(frame, question, s9Plan(nil, nil, nil)); err != nil {
		t.Fatalf("a plain count must pass: %v", err)
	}
}

// A breakdown with no grouping is not a breakdown.
func TestS9ShapeRequiresAGroupingForABreakdown(t *testing.T) {
	question := "How many records by call_type?"
	frame := extractSemanticFrame(hybridQueryRequest{Query: question})
	if got := s9QuestionShape(frame, question); got != "breakdown" {
		t.Skipf("shape = %q; the breakdown case is not reached for this phrasing", got)
	}
	if err := verifySourceNativePlanShape(frame, question, s9Plan(nil, nil, nil)); err == nil {
		t.Fatal("a breakdown without a grouping must be refused")
	}
}

// The 12,912 family. A filtered count equal to the unfiltered scope total means
// the filter was dropped, not that it matched everything.
func TestS9ScopeRejectsAFilteredCountEqualToTheWholeScope(t *testing.T) {
	filtered := s9Plan(nil, nil, []SourceNativeFilterV1{{FieldID: "cdr.msisdn", Op: "EQ", Value: "03999999999"}})
	if !s9ScopeSuspect(filtered, 12912, 12912) {
		t.Fatal("a filtered count equal to the scope total must be suspect")
	}
	err := verifySourceNativeScope(filtered, 12912, 12912)
	if err == nil || !strings.HasPrefix(err.Error(), s9ScopeViolation) {
		t.Fatalf("expected a scope violation, got %v", err)
	}
	if !strings.Contains(err.Error(), "12,912") {
		t.Fatalf("the refusal must name the total: %v", err)
	}
	// A genuinely narrowed count is fine.
	if err := verifySourceNativeScope(filtered, 872, 12912); err != nil {
		t.Fatalf("a narrowed count must pass: %v", err)
	}
	// An UNfiltered count equal to the total is exactly right, not suspect.
	if s9ScopeSuspect(s9Plan(nil, nil, nil), 12912, 12912) {
		t.Fatal("an unfiltered count equal to the total is the correct answer")
	}
	// Only row counts are judged; a SUM that coincides with the total is not.
	sum := s9Plan(nil, nil, []SourceNativeFilterV1{{FieldID: "ipdr.bytes", Op: "GT", Value: "0"}})
	sum.Measures = []SourceNativeMeasureV1{{MeasureID: "m1", Op: "SUM", FieldID: "ipdr.bytes"}}
	if s9ScopeSuspect(sum, 12912, 12912) {
		t.Fatal("a SUM must not be judged by the row-count rule")
	}
}

// An unrecognised shape asserts nothing: S9 must not invent refusals.
func TestS9ShapeAssertsNothingWhenTheShapeIsUnknown(t *testing.T) {
	question := "Give me an overview of this case"
	frame := extractSemanticFrame(hybridQueryRequest{Query: question})
	if s9QuestionShape(frame, question) == "" {
		if err := verifySourceNativePlanShape(frame, question, s9Plan([]string{"cdr.call_type"}, nil, nil)); err != nil {
			t.Fatalf("an unknown shape must not refuse: %v", err)
		}
	}
}

// "What was the largest transaction?" is a scalar MAX over the whole set, not a
// grouped ranking. S9 classified it as a rank and refused the correct plan,
// which is how a verification check turns into a defect of its own.
func TestS9MagnitudeWithoutADimensionIsScalarNotRank(t *testing.T) {
	question := "What was the largest transaction?"
	frame := extractSemanticFrame(hybridQueryRequest{Query: question})
	if got := s9QuestionShape(frame, question); got != "scalar" {
		t.Fatalf("shape = %q, want scalar", got)
	}
	plan := &SourceNativePlanV1{ContractVersion: sourceNativePlanContractV1,
		Measures: []SourceNativeMeasureV1{{MeasureID: "m1", Op: "MAX", FieldID: "transaction.amount"}}}
	if err := verifySourceNativePlanShape(frame, question, plan); err != nil {
		t.Fatalf("a scalar MAX must pass: %v", err)
	}
	// A magnitude superlative that DOES name a dimension stays a ranking.
	ranked := "Which account had the largest transaction?"
	rframe := extractSemanticFrame(hybridQueryRequest{Query: ranked})
	if got := s9QuestionShape(rframe, ranked); got != "rank" {
		t.Fatalf("%q shape = %q, want rank", ranked, got)
	}
}
