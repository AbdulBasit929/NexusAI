package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

var typedCanonicalSortFields = map[string]struct{}{
	"timestamp": {}, "observed_at": {}, "record_type": {}, "primary_target": {},
	"secondary_target": {}, "source_file": {}, "row_number": {}, "ingested_at": {},
}

// validateTypedQueryPlanV1 keeps the typed plan declarative. A plan may select a
// registered operation and bind parameters that its executor actually consumes;
// it cannot silently replace that operation's certified calculation or grouping.
func validateTypedQueryPlanV1(operationID, templateName string, plan map[string]any) error {
	if err := validateTypedAlgebraShape(plan); err != nil {
		return err
	}
	if value, supplied := plan["source_native"]; supplied {
		if templateName != "canonical_records" || value == nil {
			return fmt.Errorf("source-native algebra requires canonical_records")
		}
		data, err := json.Marshal(value)
		if err != nil {
			return err
		}
		var sourceNative SourceNativePlanV1
		if err := json.Unmarshal(data, &sourceNative); err != nil {
			return err
		}
		for _, key := range []string{"compare", "group", "projection", "sort", "sort_by", "sort_direction", "group_by", "measures", "offset"} {
			if _, present := plan[key]; present {
				return fmt.Errorf("source-native algebra cannot combine %s", key)
			}
		}
	}
	if value, supplied := plan["compare"]; supplied {
		if templateName != "canonical_records" || value == nil {
			return fmt.Errorf("typed comparison requires canonical_records")
		}
		data, err := json.Marshal(value)
		if err != nil {
			return err
		}
		var comparison CanonicalCompareV1
		if err := json.Unmarshal(data, &comparison); err != nil {
			return err
		}
		for _, key := range []string{"group", "projection", "sort", "sort_by", "sort_direction", "group_by", "measures", "offset", "limit"} {
			if _, present := plan[key]; present {
				return fmt.Errorf("typed comparison cannot combine %s", key)
			}
		}
	}
	if value, supplied := plan["group"]; supplied {
		if templateName != "canonical_records" || value == nil {
			return fmt.Errorf("typed group requires a canonical_records group object")
		}
		data, err := json.Marshal(value)
		if err != nil {
			return err
		}
		var group CanonicalGroupV1
		if err := json.Unmarshal(data, &group); err != nil {
			return err
		}
		for _, key := range []string{"projection", "sort", "sort_by", "sort_direction", "group_by", "measures", "offset", "limit"} {
			if _, supplied := plan[key]; supplied {
				return fmt.Errorf("typed group cannot combine %s", key)
			}
		}
	}
	if value, supplied := plan["projection"]; supplied {
		if templateName != "canonical_records" {
			return fmt.Errorf("typed projection is not consumed by operation %q", operationID)
		}
		if err := validateCanonicalProjection(stringSliceAny(value)); err != nil {
			return err
		}
	}
	template, ok := queryTemplateByName(templateName)
	if !ok || template.OperationID != operationID {
		return fmt.Errorf("typed query operation %q does not resolve to a registered executor", operationID)
	}

	if measures := stringSliceAny(plan["measures"]); len(measures) > 0 && !sameNormalizedStrings(measures, template.Measures) {
		return fmt.Errorf("typed query measures do not match registered operation %q; requested=%v registered=%v", operationID, measures, template.Measures)
	}
	if groupBy := stringSliceAny(plan["group_by"]); len(groupBy) > 0 && !sameNormalizedStrings(groupBy, template.GroupBy) {
		return fmt.Errorf("typed query group_by does not match registered operation %q; requested=%v registered=%v", operationID, groupBy, template.GroupBy)
	}

	sorts := mapsFromAny(plan["sort"])
	if len(sorts) > 1 {
		return fmt.Errorf("typed query operation %q supports at most one deterministic sort", operationID)
	}
	if len(sorts) == 1 {
		if templateName != "canonical_records" && templateName != "generic_filter_records" {
			return fmt.Errorf("typed query sort is not consumed by registered operation %q", operationID)
		}
		field := normalize(strings.TrimSpace(stringValueAny(sorts[0]["field"])))
		if _, allowed := typedCanonicalSortFields[field]; !allowed {
			return fmt.Errorf("typed query sort field %q is not allowlisted for operation %q", stringValueAny(sorts[0]["field"]), operationID)
		}
		direction := strings.ToLower(strings.TrimSpace(stringValueAny(sorts[0]["direction"])))
		if direction != "asc" && direction != "desc" {
			return fmt.Errorf("typed query sort direction must be asc or desc")
		}
	}

	consumedFields := map[string]bool{}
	for _, filter := range mapsFromAny(plan["filters"]) {
		if err := validateTypedQueryFilter(operationID, templateName, filter); err != nil {
			return err
		}
		field := normalize(strings.TrimSpace(stringValueAny(filter["field"])))
		if templateName == "canonical_records" && (field == "record_type" || field == "source_file") {
			if consumedFields[field] {
				return fmt.Errorf("typed canonical query repeats scalar field %q", field)
			}
			consumedFields[field] = true
		}
		// These executors lower a filter to one scalar parameter. Accepting a
		// second predicate would discard part of the requested conjunction.
		if templateName != "canonical_records" && templateName != "generic_filter_records" {
			field := normalize(strings.TrimSpace(stringValueAny(filter["field"])))
			switch field {
			case "source_start_seconds":
				field = "start_seconds"
			case "source_end_seconds":
				field = "end_seconds"
			case "plate_search_key":
				field = "plate"
			}
			if consumedFields[field] {
				return fmt.Errorf("typed query repeats scalar filter %q; executor cannot consume both predicates", field)
			}
			consumedFields[field] = true
		}
	}
	return nil
}

// Validate before permissive compatibility helpers can drop malformed controls.
func validateTypedAlgebraShape(plan map[string]any) error {
	for _, key := range []string{"project", "time_bucket", "comparison", "steps"} {
		if _, supplied := plan[key]; supplied {
			return fmt.Errorf("typed query %q is not supported by this query-plan version", key)
		}
	}
	for _, key := range []string{"filters", "sort"} {
		value, supplied := plan[key]
		if !supplied {
			continue
		}
		var rows []map[string]any
		switch typed := value.(type) {
		case []map[string]any:
			rows = typed
		case []any:
			for _, item := range typed {
				row, ok := item.(map[string]any)
				if !ok || row == nil {
					return fmt.Errorf("typed query %s must contain only objects", key)
				}
				rows = append(rows, row)
			}
		default:
			return fmt.Errorf("typed query %s must be an array of objects", key)
		}
		for _, row := range rows {
			for field := range row {
				if field != "field" && !(key == "sort" && field == "direction") && !(key == "filters" && (field == "op" || field == "value")) {
					return fmt.Errorf("typed query %s field %q is not consumed", key, field)
				}
			}
			fields := []string{"field", "op"}
			if key == "sort" {
				fields = []string{"field", "direction"}
			}
			for _, field := range fields {
				if text, ok := row[field].(string); !ok || strings.TrimSpace(text) == "" {
					return fmt.Errorf("typed query %s %s must be a non-empty string", key, field)
				}
			}
		}
	}
	for _, key := range []string{"measures", "group_by", "projection"} {
		value, supplied := plan[key]
		if !supplied {
			continue
		}
		var values []string
		switch typed := value.(type) {
		case []string:
			values = typed
		case []any:
			for _, item := range typed {
				text, ok := item.(string)
				if !ok {
					return fmt.Errorf("typed query %s must contain only strings", key)
				}
				values = append(values, text)
			}
		default:
			return fmt.Errorf("typed query %s must be an array of strings", key)
		}
		for _, text := range values {
			if strings.TrimSpace(text) == "" {
				return fmt.Errorf("typed query %s cannot contain an empty string", key)
			}
		}
	}
	return nil
}

func validateTypedQueryFilter(operationID, templateName string, filter map[string]any) error {
	field := normalize(strings.TrimSpace(stringValueAny(filter["field"])))
	op := normalize(strings.TrimSpace(stringValueAny(filter["op"])))
	if field == "" || op == "" {
		return fmt.Errorf("typed query filters require non-empty field and op")
	}
	if templateName == "canonical_records" && (field == "record_type" || field == "source_file") {
		value, ok := filter["value"].(string)
		if !ok || strings.TrimSpace(value) == "" || !containsString([]string{"eq", "equals", "equal"}, op) {
			return fmt.Errorf("canonical scalar filter %q requires one exact non-empty string", field)
		}
		return nil
	}
	switch templateName {
	case "canonical_records", "generic_filter_records":
		if !containsString([]string{"eq", "equals", "equal", "ne", "not_eq", "not_equal", "contains", "ilike", "in", "gt", "gte", "lt", "lte", "exists", "not_exists", "missing"}, op) {
			return fmt.Errorf("typed query filter op %q is not supported by operation %q", stringValueAny(filter["op"]), operationID)
		}
		return nil
	case "frequent_contacts":
		if field != "direction" || !containsString([]string{"eq", "equals", "equal"}, op) {
			return fmt.Errorf("typed query filter %q/%q is not consumed by operation %q", field, op, operationID)
		}
		if canonicalEventDirection(stringValueAny(filter["value"])) == "" {
			return fmt.Errorf("typed query direction filter requires incoming or outgoing")
		}
		return nil
	case "video_anpr_grouped_timeline":
		if field == "plate" || field == "plate_search_key" {
			if containsString([]string{"eq", "equals", "equal"}, op) && strings.TrimSpace(stringValueAny(filter["value"])) != "" {
				return nil
			}
		} else if field == "start_seconds" || field == "source_start_seconds" {
			if op == "gte" {
				return nil
			}
		} else if field == "end_seconds" || field == "source_end_seconds" {
			if op == "lte" {
				return nil
			}
		}
	case "video_timeline":
		if (field == "start_seconds" || field == "source_start_seconds") && op == "gte" {
			return nil
		}
		if (field == "end_seconds" || field == "source_end_seconds") && op == "lte" {
			return nil
		}
	}
	return fmt.Errorf("typed query filter %q/%q is not consumed by operation %q", field, op, operationID)
}

func sameNormalizedStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	counts := make(map[string]int, len(left))
	for _, value := range left {
		counts[normalize(value)]++
	}
	for _, value := range right {
		key := normalize(value)
		if counts[key] == 0 {
			return false
		}
		counts[key]--
	}
	return true
}
