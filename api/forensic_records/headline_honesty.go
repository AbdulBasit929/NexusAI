package main

import (
	"fmt"
	"os"
	"strings"
)

// TWO WAYS A TRUE NUMBER BECAME A FALSE SENTENCE, measured 2026-09-29 on eight
// questions written outside the corpus (reports/laptop-speed-20260929/RESULT.md),
// each checked against the database.
//
//	S5  "How many ANPR sightings are there for plate LHR-2026 in total?"
//	    -> "There are 87 ANPR sightings in this case."
//	       87 is right for LHR-2026; the case holds 750. The plan filtered on
//	       anpr.plate_number = LHR-2026 and the sentence never said so.
//	S7  "How many IPDR sessions used the HTTPS protocol?"
//	    -> "There are 644 IPDR sessions in this case."
//	       644 is the HTTPS count; the case holds 2,500.
//	S8  "What is the total number of bytes uploaded across all IPDR sessions?"
//	    -> "The total bytes uploaded across IPDR sessions is 0."
//	       No IPDR record carries an upload value at all: SUM returned NULL over
//	       0 counted values, and the sentence turned missing data into a zero.
//
// answerScopeQualifier states the target, range bounds and dates (A1b.1). It
// had no case for an equality, IN, BETWEEN, CONTAINS or null test inside the
// executed plan, so those shaped the number and vanished from the sentence.
// measureDenominatorNote explicitly skips a value count of zero.
//
// Both behind their own switches, default off. Off reproduces today's text.
const (
	headlineStatesFiltersEnv = "FORENSIC_HEADLINE_STATES_FILTERS"
	emptyAggregateHonestEnv  = "FORENSIC_EMPTY_AGGREGATE_HONEST"
)

func headlineStatesFiltersEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(headlineStatesFiltersEnv)), "true") //nolint:forbidigo // forensic records product API reads its feature switches from the environment by design (docker compose); not LocalAI server config
}

func emptyAggregateHonestEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(emptyAggregateHonestEnv)), "true") //nolint:forbidigo // forensic records product API reads its feature switches from the environment by design (docker compose); not LocalAI server config
}

// maxStatedFilterValues bounds how many IN values a sentence lists.
const maxStatedFilterValues = 4

// planFilterQualifiers names every filter of the executed plan that the scope
// qualifier does not already state: not the target (already "involving X") and
// not a range bound (already "with amount above N").
//
// A value is repeated only when the analyst could have typed it: never for a
// field the curated layer marks PII or withheld, and never for a filter-only
// field such as the plate-read search, which has its own sentence.
func planFilterQualifiers(req hybridQueryRequest, resp hybridQueryResponse, plan *SourceNativePlanV1) string {
	if !headlineStatesFiltersEnabled() || plan == nil || len(plan.Filters) == 0 {
		return ""
	}
	fields := map[string]FieldDescriptorV1{}
	for _, field := range catalogFromAny(resp.Records["field_catalog"]) {
		fields[strings.ToLower(field.FieldID)] = field
	}
	target := compactFilterValue(req.Target)
	dated := strings.TrimSpace(req.DateFrom) != "" || strings.TrimSpace(req.DateTo) != ""
	grouped := map[string]bool{}
	for _, fieldID := range plan.GroupFields {
		grouped[strings.ToLower(fieldID)] = true
	}
	parts := []string{}
	for _, filter := range plan.Filters {
		op := strings.ToUpper(strings.TrimSpace(filter.Op))
		if rangeComparisonWording(op) != "" {
			continue
		}
		descriptor, known := fields[strings.ToLower(filter.FieldID)]
		if known && strings.EqualFold(descriptor.RedactionState, "FILTER_ONLY") {
			continue
		}
		// Said once, not twice. Measured on the first arm: "from 2026-08-01 up to (not
		// including) 2026-09-01 with event time between 2026-08-01T00:00:00Z and ...";
		// "involving QQQ-9999 with plate number containing "QQQ-9999""; and "with account
		// status ACTIVE across 1 account status value. In full: Active: 6."
		if grouped[strings.ToLower(filter.FieldID)] {
			continue
		}
		if op == "BETWEEN" && dated && (timeLikeField(descriptor) || timeLikeValues(filter.Values)) {
			continue
		}
		if target != "" && (op == "CONTAINS" || op == "IN") {
			single := filter.Value
			if op == "IN" && len(filter.Values) == 1 {
				single = filter.Values[0]
			}
			if single != "" && compactFilterValue(single) == target {
				continue
			}
		}
		name := statedFieldName(resp, filter.FieldID)
		if name == "" {
			name = filter.FieldID
			if _, tail, found := strings.Cut(name, "."); found {
				name = tail
			}
			name = humanFieldName(name)
		}
		hidden := known && (strings.EqualFold(descriptor.Sensitivity, "PII") || strings.EqualFold(descriptor.Sensitivity, "WITHHELD"))
		value := strings.TrimSpace(filter.Value)
		switch op {
		case "EQ":
			if value == "" || (target != "" && compactFilterValue(filter.Value) == target) {
				continue
			}
			if hidden {
				parts = append(parts, "with "+name+" matching the value you gave")
				continue
			}
			parts = append(parts, "with "+name+" "+value)
		case "NEQ":
			if value == "" {
				continue
			}
			if hidden {
				parts = append(parts, "with "+name+" other than the value you gave")
				continue
			}
			parts = append(parts, "with "+name+" other than "+value)
		case "IN":
			values := filter.Values
			if len(values) == 0 && filter.Value != "" {
				values = []string{filter.Value}
			}
			if len(values) == 0 {
				continue
			}
			if hidden {
				parts = append(parts, "with "+name+" matching the values you gave")
				continue
			}
			shown := values
			more := ""
			if len(shown) > maxStatedFilterValues {
				more = fmt.Sprintf(" and %d more", len(shown)-maxStatedFilterValues)
				shown = shown[:maxStatedFilterValues]
			}
			parts = append(parts, "with "+name+" "+strings.Join(shown, ", ")+more)
		case "BETWEEN":
			if len(filter.Values) == 2 && !hidden {
				parts = append(parts, fmt.Sprintf("with %s between %s and %s", name, filter.Values[0], filter.Values[1]))
			} else {
				parts = append(parts, "with "+name+" in the range you gave")
			}
		case "CONTAINS":
			if value != "" {
				if hidden {
					parts = append(parts, "with "+name+" containing the text you gave")
				} else {
					parts = append(parts, fmt.Sprintf("with %s containing %q", name, value))
				}
			}
		case "IS_NULL":
			parts = append(parts, "with no "+name)
		case "IS_NOT_NULL":
			parts = append(parts, "with a "+name)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return " " + strings.Join(parts, " ")
}

// timeLikeField reports a timestamp or date field.
func timeLikeField(field FieldDescriptorV1) bool {
	kind := strings.ToUpper(field.EffectiveType)
	return strings.Contains(kind, "TIME") || strings.Contains(kind, "DATE")
}

// timeLikeValues reports a BETWEEN whose two bounds are both timestamps.
func timeLikeValues(values []string) bool {
	if len(values) != 2 {
		return false
	}
	for _, value := range values {
		if _, err := parseAnswerTimestamp(value); err != nil {
			return false
		}
	}
	return true
}

// compactFilterValue compares a filter value with the target the way an
// analyst reads them: case and punctuation do not make a different plate.
func compactFilterValue(value string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(value) {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// emptyAggregateHeadline states that a SUM, AVG, MIN or MAX had no values to
// work on, instead of the zero a NULL aggregate otherwise becomes. It fires only
// when the result proves it: the measure is NULL, or the executor's own count
// of values (measureDenominatorKey) is zero.
func emptyAggregateHeadline(req hybridQueryRequest, resp hybridQueryResponse, measure SourceNativeMeasureV1, result map[string]any, noun, scope string) (string, bool) {
	if !emptyAggregateHonestEnabled() || result == nil {
		return "", false
	}
	var word string
	switch strings.ToUpper(measure.Op) {
	case "SUM":
		word = "total"
	case "AVG":
		word = "average"
	case "MIN":
		word = "minimum"
	case "MAX":
		word = "maximum"
	default:
		return "", false
	}
	raw, present := result[measure.MeasureID]
	counted, hasCount := answerNumber(result[measureDenominatorKey(measure.MeasureID)])
	nullAggregate := present && raw == nil
	if !nullAggregate && !(hasCount && counted == 0) {
		return "", false
	}
	if field := statedFieldName(resp, measure.FieldID); field != "" {
		return fmt.Sprintf("No %s values are recorded in these %s%s, so there is no %s to state.", field, noun, scope, word), true
	}
	return fmt.Sprintf("No values are recorded in these %s%s for this field, so there is no %s to state.", noun, scope, word), true
}
