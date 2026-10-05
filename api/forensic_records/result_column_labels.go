package main

import (
	"os"
	"strings"
	"unicode"
)

// A1. RESULT COLUMNS ARE LABELLED WITH WHAT THEY HOLD.
//
// The data grid labelled columns from raw keys, so 54 typed-plan answers on the
// deployed build showed a column headed "M1", plus internal "Metadata" and
// "Section" columns, and grouped fields under raw source names ("Msisdn",
// "Inbound Outbound IND") although the curated layer names them "Subscriber
// number" and "Call direction". The UX contract forbids a header like "M1": the
// analyst cannot tell what the number is.
//
// Headers only. Keys and values are untouched, so nothing a client keys on and
// nothing stated in an answer changes. See reports/column-labels-20260929/.
const resultColumnLabelsEnv = "FORENSIC_RESULT_COLUMN_LABELS"

func resultColumnLabelsEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(resultColumnLabelsEnv)), "true") //nolint:forbidigo // forensic records product API reads its feature switches from the environment by design (docker compose); not LocalAI server config
}

// sourceNativeColumnLabels maps data-grid keys to analyst-facing headers for
// the executed typed plan, or nil when there is no plan or the switch is off.
func sourceNativeColumnLabels(req hybridQueryRequest, resp hybridQueryResponse) map[string]string {
	if !resultColumnLabelsEnabled() {
		return nil
	}
	plan := executedSourceNativePlan(resp)
	if plan == nil {
		return nil
	}
	fields := map[string]FieldDescriptorV1{}
	for _, field := range catalogFromAny(resp.Records["field_catalog"]) {
		fields[field.FieldID] = field
	}
	display := func(fieldID string) string {
		field := fields[fieldID]
		if name := strings.TrimSpace(field.DisplayName); name != "" {
			return name
		}
		if field.NormalizedName != "" {
			return humanFieldName(field.NormalizedName)
		}
		return humanFieldName(fieldID)
	}
	// Lower-case the first letter so the label reads as one phrase ("Count of
	// call type"), but never break an acronym: "HTTP status code" stays as is.
	// Measured 2026-09-29 on the A1 arm: the first version wrote "hTTP".
	lower := func(text string) string {
		if text == "" {
			return text
		}
		if first := strings.Fields(text)[0]; len(first) > 1 && first == strings.ToUpper(first) {
			return text
		}
		return strings.ToLower(text[:1]) + text[1:]
	}
	labels := map[string]string{"metadata": "Source rows"}
	for _, measure := range plan.Measures {
		var label string
		switch op := strings.ToUpper(measure.Op); {
		case op == "COUNT" && strings.TrimSpace(measure.FieldID) == "":
			label = capitalizeFirst(plateReadNounOr(plan, resultNoun(req, resp, 2), 2))
		case op == "COUNT":
			label = "Count of " + lower(display(measure.FieldID))
		case op == "COUNT_DISTINCT":
			label = "Distinct " + lower(display(measure.FieldID))
		case op == "SUM":
			label = "Total " + lower(display(measure.FieldID))
		case op == "AVG":
			label = "Average " + lower(display(measure.FieldID))
		case op == "MIN":
			label = "Minimum " + lower(display(measure.FieldID))
		case op == "MAX":
			label = "Maximum " + lower(display(measure.FieldID))
		default:
			continue
		}
		labels[measure.MeasureID] = label
		labels[measureDenominatorKey(measure.MeasureID)] = label + " (values counted)"
	}
	for _, fieldID := range plan.GroupFields {
		if field, ok := fields[fieldID]; ok && field.NormalizedName != "" {
			labels[field.NormalizedName] = display(fieldID)
		}
	}
	return labels
}

// applyColumnLabels rewrites the header of every column whose key has a
// label. Keys, order and rows are untouched.
func applyColumnLabels(grid map[string]any, labels map[string]string) {
	if len(labels) == 0 || grid == nil {
		return
	}
	columns, ok := grid["columns"].([]map[string]any)
	if !ok {
		return
	}
	for _, column := range columns {
		if label, found := labels[stringValueAny(column["key"])]; found && label != "" {
			column["header"] = label
		}
	}
}

func capitalizeFirst(text string) string {
	for index, r := range text {
		return string(unicode.ToUpper(r)) + text[index+len(string(r)):]
	}
	return text
}
