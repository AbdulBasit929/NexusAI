package main

import (
	"strings"
	"testing"
)

// MEASURED AND REVERTED 2026-09-24. Evidence:
// reports/tier-decision-20260924/step1/, rule in ../DECISION_RULE.md.
//
// Two sentences were added to this prompt, each aimed at one captured failure:
// COUNT_DISTINCT guidance for CDR-08/ANPR-05, and a rule binding every filter
// value to `literal_values` for CDR-11/TWR-02. The pre-declared threshold was
// "any question moves to WRONG -> REVERT IMMEDIATELY, at any ratio".
//
//	45 CORRECT · 16 CLARIFIED · 0 WRONG   ->   43 · 14 · 4 WRONG
//	in-scope conversions: ZERO
//
// Reverted on the rule, not on judgement.
//
// WHY IT FAILED, AND IT IS THE SAME LESSON TWICE FROM OPPOSITE DIRECTIONS.
// CDR-11 previously carried an invented filter (`cdr.direction = "outbound"`)
// beside a correct one, and the CONSTRAINT_APPLIED allowlist refused the plan.
// Told not to invent filters, the model emitted ONE clean filter on the WRONG
// FIELD -- `cdr.originating_number` rather than `cdr.msisdn` -- which passes
// every guard and answered 447 where the truth is 872. NEG-02, a negative
// control asking about a plate that does not exist, went from a correct "no
// sightings" to "1 ANPR sighting matched this question".
//
// The invented filter was LOAD-BEARING AS A SIGNAL. It marked a plan the model
// had not understood, and the wrong-field error underneath it was always there.
// Suppressing the symptom let the disease reach the analyst.
//
// WI-23's Fix B reached the same conclusion from the other side: DROPPING
// invented filters after generation created two confident-wrong answers.
// Preventing them at generation created four. **Do not attempt to remove
// generator-invented filters by any means.** They are diagnostic.
//
// STILL SEPARABLE AND NEVER MEASURED ALONE: the COUNT_DISTINCT sentence. It
// converted nothing here, but it was measured only alongside the filter
// sentence that caused the damage. If it is retried it must be measured by
// itself, against the same threshold.

func TestIRSystemPromptDoesNotBindEveryFilterToALiteral(t *testing.T) {
	prompt := irCleanPlanSystemPrompt()
	// The reverted sentence. Its presence is a regression, not an improvement.
	if strings.Contains(prompt, "Every filter value MUST be one of literal_values") {
		t.Fatal("this instruction was MEASURED at 4 confident-wrong and reverted; " +
			"re-adding it needs a fresh measurement against the declared threshold")
	}
	// What must remain: the empty-filters rule, which is a different case and
	// was never implicated.
	if !strings.Contains(prompt, "filters MUST be []") {
		t.Error("the no-literal rule is unrelated to the revert and must stay")
	}
}

// The instructions the prompt DOES carry, each still earning its place.
func TestIRSystemPromptKeepsItsMeasuredInstructions(t *testing.T) {
	prompt := irCleanPlanSystemPrompt()
	for _, instruction := range []string{
		`"op":\"COUNT\"`,   // count rows
		"SUM",              // a total
		"MAX",              // the largest
		"MIN",              // the smallest
		"group_fields",     // a breakdown
		"m1 DESC",          // which X has the most
		"Never write SQL",  // the standing prohibition
	} {
		if !strings.Contains(prompt, strings.ReplaceAll(instruction, `\"`, `"`)) {
			t.Errorf("the prompt no longer carries %q", instruction)
		}
	}
}

// The prompt ships on every generating request and generation is ~93% of a slow
// request at ~4 tok/s, so its length is a real cost. A tripwire, not a target.
func TestIRSystemPromptStaysBounded(t *testing.T) {
	if size := len(irCleanPlanSystemPrompt()); size > 1800 {
		t.Errorf("system prompt is %d bytes; every generating request pays for it", size)
	}
}
