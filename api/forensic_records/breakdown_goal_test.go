package main

import "testing"

// The breakdown goal must be INERT when its switch is off. Every other switch
// on this path gates a new capability the same way, and the one time a change
// altered classification unconditionally it cost a confident-wrong answer.
func TestBreakdownGoalOffChangesNothing(t *testing.T) {
	t.Setenv(breakdownGoalEnv, "")
	for _, question := range []string{
		"Show the call type breakdown",
		"Break down IPDR sessions by protocol",
		"Show the breakdown of HTTP status codes in the access logs",
	} {
		if got := breakdownGoal(question); got != "" {
			t.Fatalf("switch off: %q classified %q, want inert", question, got)
		}
		if got := semanticFrameGoal(semanticFrameTerms(question), question); got == "breakdown" {
			t.Fatalf("switch off: %q reached the breakdown goal anyway", question)
		}
	}
}

// ON, it must fire on EXPLICIT breakdown language and nothing else. The second
// list is the one that matters: each entry is a question that currently answers
// CORRECT, and capturing any of them here would move a working question on the
// same run that introduces the goal.
func TestBreakdownGoalOnFiresOnlyOnExplicitBreakdowns(t *testing.T) {
	t.Setenv(breakdownGoalEnv, "true")

	for _, question := range []string{
		"Show the call type breakdown",
		"Break down IPDR sessions by protocol",
		"Show the breakdown of HTTP status codes in the access logs",
		"Explain the call type breakdown",
		"show the breakdowns by camera",
	} {
		if got := semanticFrameGoal(semanticFrameTerms(question), question); got != "breakdown" {
			t.Fatalf("%q classified %q, want breakdown", question, got)
		}
	}

	for _, tc := range []struct{ question, want string }{
		// CDR-05. Answers CORRECT today as aggregate/scalar, because the S9
		// scalar guard consults the curated layer for exactly this phrasing.
		// "of each" is a LATER slice; capturing it here would make this run
		// unattributable.
		{"How many calls of each type are there?", "aggregate"},
		// A ranking is not a breakdown even though both group.
		{"Which camera recorded the most sightings?", "rank"},
		// Plain counts and lookups must be untouched.
		{"How many CDR records do we have in this case?", "aggregate"},
		{"Where is tower PK-LHR-SYN-001 located?", "lookup"},
		{"How many unique handsets are there?", "distinct"},
	} {
		if got := semanticFrameGoal(semanticFrameTerms(tc.question), tc.question); got != tc.want {
			t.Fatalf("%q classified %q, want %q", tc.question, got, tc.want)
		}
	}
}

// The POINT of the goal: its shape carries a grouping obligation, so an
// ungrouped plan is refused rather than presented as a single number. This is
// the exact inversion of the WI-31 failure, where ACC-02 answered "there are
// 1,000 access log entries" to a breakdown question.
func TestBreakdownGoalRefusesAnUngroupedPlan(t *testing.T) {
	t.Setenv(breakdownGoalEnv, "true")
	question := "Show the breakdown of HTTP status codes in the access logs"
	frame := SemanticFrameV1{Goal: semanticFrameGoal(semanticFrameTerms(question), question)}

	if got := s9QuestionShape(frame, question); got != "breakdown" {
		t.Fatalf("shape %q, want breakdown", got)
	}
	ungrouped := &SourceNativePlanV1{Measures: []SourceNativeMeasureV1{{Op: "COUNT"}}}
	if err := verifySourceNativePlanShape(frame, question, ungrouped); err == nil {
		t.Fatal("an ungrouped plan was accepted for an explicit breakdown question")
	}
	grouped := &SourceNativePlanV1{
		Measures:    []SourceNativeMeasureV1{{Op: "COUNT"}},
		GroupFields: []string{"access_log.status"},
	}
	if err := verifySourceNativePlanShape(frame, question, grouped); err != nil {
		t.Fatalf("a correctly grouped breakdown was refused: %v", err)
	}
}

// A new goal with no entry in semanticGoalMatchesIntent scores
// IntentCompatibility 0, and deterministicRegisteredMatch then rejects EVERY
// candidate -- it exempts only "lookup"/"none". That failure is silent, which
// is why it is pinned here rather than left to the live suite to discover.
func TestBreakdownGoalIsRegisteredWithIntentMatching(t *testing.T) {
	if !semanticGoalMatchesIntent("breakdown", "aggregate") {
		t.Fatal("breakdown does not match an aggregate operation intent")
	}
	if !semanticGoalMatchesIntent("breakdown", "summarize") {
		t.Fatal("breakdown does not match a summarize operation intent")
	}
	if semanticGoalMatchesIntent("breakdown", "timeline") {
		t.Fatal("breakdown matched an unrelated intent")
	}
}
