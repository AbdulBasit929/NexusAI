package main

import (
	"fmt"
	"os"
	"strings"
)

// A4. A COUNT OF A FIELD SAYS WHICH FIELD IT COUNTED.
//
// Measured 2026-09-29 on the deployed build (reports/a1-1-a2-20260929/on): three
// ungrouped distinct counts answered with a sentence that names no field.
//
//	CDR-08           "How many unique phone numbers appear as callers in the CDRs?"
//	                 -> "The count across CDR records is 10."
//	ANPR-05          "How many distinct license plates were captured?"
//	                 -> "The count across ANPR sightings is 6."
//	H1-CDR-HANDSETS  -> "The count across CDR records is 4,997."
//
// The number is right, but the sentence does not say what it is a count of, and
// for CDR-08 that hides something the analyst needs: the plan counted distinct
// SUBSCRIBER numbers, which is not obviously the same as "callers". Naming the
// field turns a hidden interpretation into a visible one.
//
// The same gap, unmeasured because no corpus question has the shape: COUNT of a
// field counts only the rows where that field is present (COUNT(expr) in
// source_native_sql_executor.go), and was stated as "There are N CDR records in
// this case", which reads as every record. It now says which field was present.
//
// COUNT(*) is untouched, and so is the plate-read search, which is a COUNT(*).
//
// DEFAULT OFF, like every switch gating new behaviour. Off reproduces today's
// sentences exactly.
const countNamesFieldEnv = "FORENSIC_COUNT_NAMES_FIELD"

func countNamesFieldEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(countNamesFieldEnv)), "true") //nolint:forbidigo // forensic records product API reads its feature switches from the environment by design (docker compose); not LocalAI server config
}

// countsFieldValues reports a COUNT of one field (not COUNT(*)) that this file
// states, so the record-count sentence does not claim it.
//
// A filter on the SAME field already guarantees the field is present, so there
// the count IS the record count and the record sentence is the right one.
// Measured 2026-09-29 on the A4 arm: "How many transactions have an amount
// above 50000?" read "1 transaction with amount above 50000 has a transaction
// amount value." -- true, and worse than what it replaced. No threshold fired;
// reading the changed texts found it.
func countsFieldValues(plan *SourceNativePlanV1, measure SourceNativeMeasureV1) bool {
	if !countNamesFieldEnabled() || !strings.EqualFold(measure.Op, "COUNT") || strings.TrimSpace(measure.FieldID) == "" {
		return false
	}
	if plan != nil {
		for _, filter := range plan.Filters {
			if strings.EqualFold(filter.FieldID, measure.FieldID) {
				return false
			}
		}
	}
	return true
}

// countedFieldHeadline states an ungrouped COUNT or COUNT_DISTINCT of a field
// with the field's curated name, or ok=false when the field cannot be named.
func countedFieldHeadline(req hybridQueryRequest, resp hybridQueryResponse, plan *SourceNativePlanV1, measure SourceNativeMeasureV1, value float64, scope string) (string, bool) {
	if !countNamesFieldEnabled() || strings.TrimSpace(measure.FieldID) == "" {
		return "", false
	}
	if strings.EqualFold(measure.Op, "COUNT") && !countsFieldValues(plan, measure) {
		return "", false
	}
	field := statedFieldName(resp, measure.FieldID)
	if field == "" {
		return "", false
	}
	records := resultNoun(req, resp, 0)
	switch strings.ToUpper(measure.Op) {
	case "COUNT_DISTINCT":
		switch value {
		case 0:
			return fmt.Sprintf("There are no %s values across %s%s.", field, records, scope), true
		case 1:
			return fmt.Sprintf("There is 1 distinct %s value across %s%s.", field, records, scope), true
		}
		return fmt.Sprintf("There are %s distinct %s values across %s%s.", formatAnswerNumber(value), field, records, scope), true
	case "COUNT":
		switch value {
		case 0:
			return fmt.Sprintf("No %s%s have a %s value.", resultNoun(req, resp, 0), scope, field), true
		case 1:
			return fmt.Sprintf("1 %s%s has a %s value.", resultNoun(req, resp, 1), scope, field), true
		}
		return fmt.Sprintf("%s %s%s have a %s value.", formatAnswerNumber(value), resultNoun(req, resp, value), scope, field), true
	}
	return "", false
}

// statedFieldName is the field's curated display name as it reads mid-sentence:
// the first letter lowered unless the first word is an acronym ("device IMEI",
// "HTTP status code"), or "" when the catalogue does not name the field.
func statedFieldName(resp hybridQueryResponse, fieldID string) string {
	for _, field := range catalogFromAny(resp.Records["field_catalog"]) {
		if !strings.EqualFold(field.FieldID, fieldID) {
			continue
		}
		name := strings.TrimSpace(field.DisplayName)
		if name == "" {
			return strings.TrimSpace(humanFieldName(field.NormalizedName))
		}
		if first := strings.Fields(name)[0]; len(first) > 1 && first == strings.ToUpper(first) {
			return name
		}
		return strings.ToLower(name[:1]) + name[1:]
	}
	return ""
}
