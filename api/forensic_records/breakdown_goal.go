package main

import (
	"os"
	"regexp"
	"strings"

	"github.com/mudler/LocalAI/pkg/forensictext"
)

// THE BREAKDOWN GOAL — slice 1 of the taxonomy fix.
//
// The goal taxonomy has no `breakdown`, and the gap is not cosmetic. An
// explicit breakdown question currently lands on the `lookup` DEFAULT, which
// means "unrecognised" and skips shape verification entirely.
//
// WI-31 tried to close this by mapping "breakdown" onto the EXISTING
// `aggregate` goal and produced a confident wrong answer: ACC-02, "show the
// breakdown of HTTP status codes in the access logs", went CORRECT -> WRONG and
// answered "There are 1,000 access log entries in this case". The cause is
// recorded in `semanticFrameGoal` and it is structural — `aggregate` with no
// group-by hints resolves to shape `scalar`, so calling a breakdown `aggregate`
// ACTIVELY ASSERTS it is a single number.
//
// A goal whose shape is `breakdown` inverts the failure direction. That shape
// already carries a grouping obligation in `verifySourceNativePlanShape`: a
// plan with no GroupFields is REFUSED. So where WI-31 turned a breakdown into a
// confident total, this turns an ungrouped plan into a clarification.
//
// DEFAULT OFF, like every switch that gates a NEW behaviour. Off reproduces
// today bit-for-bit: `breakdownGoal` is the only caller of this vocabulary and
// it returns "" when the switch is off.
//
// Measured against a threshold written BEFORE the run:
// reports/breakdown-taxonomy-20260925/BREAKDOWN_GOAL_RULE.md
const breakdownGoalEnv = "FORENSIC_BREAKDOWN_GOAL"

func breakdownGoalEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(breakdownGoalEnv)), "true")
}

// EXPLICIT breakdown language only. "break down" is two tokens, so it cannot be
// matched through the term set the rest of `semanticFrameGoal` uses, and the
// plural is included because "show the breakdowns" is the same request.
//
// "for each" / "of each" is deliberately NOT here. CDR-05 ("how many calls of
// each type are there?") already answers CORRECT as `aggregate`/`scalar`,
// because the S9 scalar guard was relaxed to consult the curated layer for
// exactly that case. Moving it on the same run that introduces a new goal would
// leave neither effect attributable. It is a separate slice.
var breakdownGoalPhrase = regexp.MustCompile(`(?i)\bbreak\s*downs?\b`)

// breakdownGoal reports the goal for an explicit breakdown question, or "" when
// the question is not one or the switch is off.
func breakdownGoal(question string) string {
	if !breakdownGoalEnabled() {
		return ""
	}
	if breakdownGoalPhrase.MatchString(forensictext.PlanningText(question)) {
		return "breakdown"
	}
	return ""
}
