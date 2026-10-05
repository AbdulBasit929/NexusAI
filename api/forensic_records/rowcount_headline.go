package main

import (
	"fmt"
	"os"
	"strings"
)

// S1. A RESULT-ROW COUNT IS NOT A FINDING.
//
// Captured 2026-09-28 on the deployed build:
//
//	"Show me everything you have about plate ABC-123"
//	    -> "1 ANPR sighting matched this question."      truth: 218
//	"Summarise the suspicious activity in this case"
//	    -> "14 records matched this question."           14 summary rows
//
// Both reach the terminal fallback of enterpriseExecutiveAnswer, which states
// the number of RESULT ROWS as "N <family> matched". That is true only for a
// list of source records whose N is the whole total. For ABC-123 the plan was
// right -- it filtered the plate and computed COUNT(*) = 218 among four
// measures -- but sourceNativeResultAnswer declines any plan with more than one
// measure, so the one aggregate row was reported as one sighting. For the
// summary, the 14 rows are findings, not matching records.
//
// uncomputedQuantityAnswer already withholds the fallback, but only when the
// question says "how many"; neither of these does. Its marker is left exactly
// as it is, so it keeps withholding what it withholds today.
//
// See reports/rowcount-headline-20260928/.
const rowCountHeadlineGuardEnv = "FORENSIC_ROW_COUNT_HEADLINE_GUARD"

func rowCountHeadlineGuardEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(rowCountHeadlineGuardEnv)), "true") //nolint:forbidigo // forensic records product API reads its feature switches from the environment by design (docker compose); not LocalAI server config
}

// Part A. multiMeasureRecordCountHeadline states the COUNT(*) of an ungrouped,
// multi-measure plan -- the number of matching records -- in the sentence the
// single-COUNT path uses, and says the other computed values are shown with it.
// Only COUNT with no field counts records; COUNT(field) counts non-empty values
// and is not a record count. Runs only after the MIN/MAX range path declined.
func multiMeasureRecordCountHeadline(req hybridQueryRequest, resp hybridQueryResponse, plan *SourceNativePlanV1, results []map[string]any) (resultAnswer, bool) {
	if !rowCountHeadlineGuardEnabled() || plan == nil || len(plan.Measures) < 2 {
		return resultAnswer{}, false
	}
	if len(plan.GroupFields) > 0 || plan.TimeBucket != nil || len(plan.Project) > 0 || len(results) != 1 {
		return resultAnswer{}, false
	}
	for _, measure := range plan.Measures {
		if !strings.EqualFold(measure.Op, "COUNT") || strings.TrimSpace(measure.FieldID) != "" {
			continue
		}
		value, ok := answerNumber(results[0][measure.MeasureID])
		if !ok {
			return resultAnswer{}, false
		}
		scope := answerScopeQualifier(req)
		others := " The other values computed for this question are shown with this answer."
		if value == 0 {
			return resultAnswer{Headline: fmt.Sprintf("There are no %s%s in this case.", resultNoun(req, resp, 0), scope), Value: 0.0}, true
		}
		verb := "are"
		if value == 1 {
			verb = "is"
		}
		return resultAnswer{Headline: fmt.Sprintf("There %s %s %s%s in this case.%s", verb, formatAnswerNumber(value), resultNoun(req, resp, value), scope, others), Value: value}, true
	}
	return resultAnswer{}, false
}

// Part B. rowCountFallbackSentence replaces "N <family> matched this question"
// wherever that sentence would not be true. It returns "" to keep today's
// sentence: a record listing whose full total equals the rows shown.
func rowCountFallbackSentence(resp hybridQueryResponse, count int, noun string) string {
	if !rowCountHeadlineGuardEnabled() {
		return ""
	}
	plan := executedSourceNativePlan(resp)
	listing := resp.Template == "canonical_records" && (plan == nil || len(plan.Measures) == 0)
	if !listing {
		if plan != nil && len(plan.Measures) > 0 {
			return "The computed values are shown with this answer. No single figure answers this question, so none is stated."
		}
		if entry, ok := queryTemplateByName(resp.Template); ok && strings.TrimSpace(entry.Description) != "" {
			return fmt.Sprintf("The results shown come from: %s No single figure answers this question, so none is stated.",
				strings.TrimSpace(entry.Description))
		}
		return "The results are shown with this answer. No single figure answers this question, so none is stated."
	}
	total, known := answerNumber(resp.Records["total_count"])
	switch {
	case known && int(total) == count:
		return ""
	case known && int(total) > count:
		return fmt.Sprintf("Showing %s of %s %s that matched this question.", formatAnswerNumber(float64(count)), formatAnswerNumber(total), noun)
	default:
		return fmt.Sprintf("Showing %s %s; the full number that matched was not computed.", formatAnswerNumber(float64(count)), noun)
	}
}

// executedSourceNativePlan reads the typed plan the records path attached to
// the response, in either of the two forms buildResultAnswer accepts.
func executedSourceNativePlan(resp hybridQueryResponse) *SourceNativePlanV1 {
	if plan, ok := resp.Records["plan"].(*SourceNativePlanV1); ok {
		return plan
	}
	if planMap, ok := resp.Records["plan"].(map[string]any); ok && planMap != nil {
		var plan SourceNativePlanV1
		if decodeViaJSON(planMap, &plan) == nil && plan.ContractVersion != "" {
			return &plan
		}
	}
	return nil
}
