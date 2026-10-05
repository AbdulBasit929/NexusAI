package main

import (
	"sort"
	"strings"
	"testing"
)

// A3.3 CENSUS. "Where is X" and "who is X" ask for ATTRIBUTE VALUES of a named
// entity -- a projection. Before anything is wired, record what the ladder and
// the goal classifier decide for every such question today, so the change can
// be predicted question by question rather than discovered in the results.
//
// Measured on the shipped posture 2026-09-27:
//
//	TWR-02  "Where is tower PK-LHR-SYN-001 located?"                CLARIFIED  no_verified_plan
//	SUB-03  "Who is the registered subscriber for PK-SUB-SYN-ALPHA?" CLARIFIED  no_verified_plan
//	H11     "Where was 923001110001 seen according to the call records?"
//	        WRONG -- a typed plan ran and COUNTED 447 rows where locations were asked
func TestProjectionQuestionCensus(t *testing.T) {
	paths := []string{
		"../../evaluation/golden_questions_v2.json",
		"../../evaluation/holdout_questions_v1.json",
		"../../evaluation/holdout_media_v1.json",
	}
	rows := []string{}
	for _, path := range paths {
		for _, question := range loadRoutingQuestions(t, path) {
			lower := strings.ToLower(strings.TrimSpace(question.Q))
			if !strings.HasPrefix(lower, "where ") && !strings.HasPrefix(lower, "who ") &&
				!strings.HasPrefix(lower, "what is the ") && !strings.HasPrefix(lower, "list ") &&
				!strings.HasPrefix(lower, "show ") {
				continue
			}
			goal := semanticFrameGoal(semanticFrameTerms(question.Q), question.Q)
			template := chooseTemplate(question.Q, "")
			rows = append(rows, question.ID+"\n      goal="+goal+"  ladder="+orDash(template)+
				"  family="+orDash(semanticQuestionFamily(question.Q))+"\n      "+question.Q)
		}
	}
	sort.Strings(rows)
	t.Logf("PROJECTION-SHAPED QUESTIONS (%d): where / who / what is the / list / show", len(rows)) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	for _, row := range rows {
		t.Logf("   %s", row) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}

func orDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}
