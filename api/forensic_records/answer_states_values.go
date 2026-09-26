package main

import (
	"fmt"
	"os"
	"strings"
)

// THE STATED ANSWER MUST CONTAIN THE ANSWER.
//
// The Phase 1 gate requires "answer stated in analyst text". Measured
// 2026-09-25 against the 62, EIGHT questions failed it while scoring CORRECT,
// because the harness graded the whole response instead of the analyst text:
//
//	CDR-09  "what date range do the CDR records cover?"
//	        -> "1 CDR record matched this question."
//	CDR-10  "who did 923001110001 contact most frequently?"
//	        -> "8 phone contacts ranked for 923001110001."
//	ACC-02  "show the breakdown of HTTP status codes"
//	        -> top THREE of six values
//
// HONEST SCOPE, established before writing this: the values are NOT hidden from
// the analyst. They are in `data_grid`, and for CDR-10 that grid is perfectly
// legible ("Counterparty", "Total Interactions"). The defect is that the DIRECT
// ANSWER does not answer, so the analyst has to reconstruct it from a table.
// For the range case the table does not help either -- its columns are headed
// `M1` and `M2`, which this file's neighbour already calls out as a defect
// class ("the same class of defect as a column headed M1").
//
// TWO CAUSES, both addressed here:
//
//	(1) sourceNativeResultAnswer DECLINES any plan with more than one measure,
//	    so a MIN/MAX range -- two measures by construction -- falls through to
//	    the generic "N records matched" narration. A date range is TWO values;
//	    one number cannot express it, which is why `semanticFrameGoal` grew a
//	    `range` case in the first place.
//	(2) a complete breakdown is truncated to the top three entries. Stating
//	    "across 6 values" and then listing three is not wrong, but it is not
//	    the breakdown that was asked for.
//
// DEFAULT OFF, like every switch gating new behaviour. Off reproduces today's
// narration exactly.
const answerStatesValuesEnv = "FORENSIC_ANSWER_STATES_VALUES"

func answerStatesValuesEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(answerStatesValuesEnv)), "true")
}

// answerBreakdownFullLimit is how many groups a breakdown states in full before
// falling back to "largest N". Twelve covers every grouped question in the
// corpus (the largest is 6) while keeping a headline readable; beyond it, a
// sentence listing every value stops being an answer and becomes the table.
const answerBreakdownFullLimit = 12

// sourceNativeRangeMeasures reports the MIN and MAX measures of a two-measure
// plan over ONE field -- the shape a range question compiles to.
//
// Both halves are required. A plan carrying MIN and MAX of DIFFERENT fields is
// not a range, and describing it as one would assert a relationship between two
// quantities that were never compared.
func sourceNativeRangeMeasures(plan *SourceNativePlanV1) (low, high SourceNativeMeasureV1, ok bool) {
	if plan == nil || len(plan.Measures) != 2 || len(plan.GroupFields) != 0 {
		return low, high, false
	}
	first, second := plan.Measures[0], plan.Measures[1]
	if !strings.EqualFold(first.FieldID, second.FieldID) {
		return low, high, false
	}
	switch {
	case strings.EqualFold(first.Op, "MIN") && strings.EqualFold(second.Op, "MAX"):
		return first, second, true
	case strings.EqualFold(first.Op, "MAX") && strings.EqualFold(second.Op, "MIN"):
		return second, first, true
	}
	return low, high, false
}

// answerRangeDisplay renders one end of a range.
//
// shortDate is applied ONLY to a value that actually parses as a timestamp.
// shortDate truncates any string of ten characters or more, so handing it a
// large number ("1730127963000") would silently chop it to ten digits and state
// a wrong quantity with full confidence -- the exact failure class this file
// exists to remove. A non-timestamp is stated unchanged.
func answerRangeDisplay(value string) string {
	if _, err := parseAnswerTimestamp(value); err == nil {
		return shortDate(value)
	}
	return value
}

// answerRangeHeadline states both ends of a range in the analyst text.
func answerRangeHeadline(fieldLabel, family, scope, lowValue, highValue string) string {
	low, high := strings.TrimSpace(lowValue), strings.TrimSpace(highValue)
	if low == "" || high == "" {
		return ""
	}
	low, high = answerRangeDisplay(low), answerRangeDisplay(high)
	label := strings.TrimSpace(fieldLabel)
	if label == "" {
		label = "values"
	}
	if low == high {
		return fmt.Sprintf("The %s%s all share one %s: %s.", family, scope, label, low)
	}
	return fmt.Sprintf("The %s%s span %s from %s to %s.", family, scope, label, low, high)
}

// sourceNativeFieldNameFor resolves a field ID to its curated display name,
// falling back to the normalized source name. Both come from the catalogue the
// executor already attached to the response, so this reads the SAME vocabulary
// the plan was constrained to rather than a second one that could disagree.
func sourceNativeFieldNameFor(resp hybridQueryResponse, fieldID string) string {
	for _, field := range catalogFromAny(resp.Records["field_catalog"]) {
		if !strings.EqualFold(field.FieldID, fieldID) {
			continue
		}
		if name := strings.TrimSpace(field.DisplayName); name != "" {
			return strings.ToLower(name)
		}
		return field.NormalizedName
	}
	return ""
}
