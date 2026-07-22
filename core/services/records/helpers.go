package records

import (
	"fmt"
	"strings"
)

func (s *Service) RunHelper(userID string, req QueryRequest) (QueryResponse, error) {
	helper := strings.ToLower(strings.TrimSpace(req.Helper))
	args := req.HelperArgs
	if args == nil {
		args = map[string]any{}
	}
	base := req
	base.Helper = ""
	base.HelperArgs = nil

	switch helper {
	case "shortest_call":
		base.RecordType = defaultRecordType(base.RecordType, RecordTypeCDR)
		base.Sort = []SortField{{Field: "duration_seconds", Direction: "asc"}}
		base.Limit = 1
		return s.Query(userID, base)
	case "longest_call":
		base.RecordType = defaultRecordType(base.RecordType, RecordTypeCDR)
		base.Sort = []SortField{{Field: "duration_seconds", Direction: "desc"}}
		base.Limit = 1
		return s.Query(userID, base)
	case "target_numbers_by_source":
		base.RecordType = defaultRecordType(base.RecordType, RecordTypeCDR)
		if source := argString(args, "source_number", "source", "from"); source != "" {
			base.Filters = append(base.Filters, Filter{Field: "source_number", Op: "eq", Value: source})
		}
		return s.aggregateAsQuery(userID, AggregateRequest{QueryRequest: base, Operation: "top_by_field", Field: "target_number", TopN: argInt(args, "top_n", 20)})
	case "calls_by_area":
		base.RecordType = defaultRecordType(base.RecordType, RecordTypeCDR)
		if area := argString(args, "area", "location"); area != "" {
			base.Filters = append(base.Filters, Filter{Field: "area", Op: "contains", Value: area})
		}
		return s.Query(userID, base)
	case "calls_by_date_range":
		base.RecordType = defaultRecordType(base.RecordType, RecordTypeCDR)
		base.TimeRange = &TimeRange{Field: "timestamp", From: argString(args, "from", "start"), To: argString(args, "to", "end")}
		return s.Query(userID, base)
	case "calls_between_numbers":
		base.RecordType = defaultRecordType(base.RecordType, RecordTypeCDR)
		a := argString(args, "a", "source_number", "from")
		b := argString(args, "b", "target_number", "to")
		if a == "" || b == "" {
			return QueryResponse{}, fmt.Errorf("calls_between_numbers requires source and target numbers")
		}
		return s.callsBetweenNumbers(userID, base, a, b)
	case "calls_by_cell_id":
		base.RecordType = defaultRecordType(base.RecordType, RecordTypeCDR)
		if cellID := argString(args, "cell_id", "cell"); cellID != "" {
			base.Filters = append(base.Filters, Filter{Field: "cell_id", Op: "eq", Value: cellID})
		}
		return s.Query(userID, base)
	case "failed_calls":
		base.RecordType = defaultRecordType(base.RecordType, RecordTypeCDR)
		base.Filters = append(base.Filters, Filter{Field: "status", Op: "contains", Value: "fail"})
		return s.Query(userID, base)
	case "roaming_calls":
		base.RecordType = defaultRecordType(base.RecordType, RecordTypeCDR)
		base.Filters = append(base.Filters, Filter{Field: "roaming", Op: "eq", Value: true})
		return s.Query(userID, base)
	case "imei_to_msisdns":
		base.RecordType = defaultRecordType(base.RecordType, RecordTypeCDR)
		return s.aggregateAsQuery(userID, AggregateRequest{QueryRequest: base, Operation: "count", GroupBy: []string{"imei", "source_number"}, TopN: argInt(args, "top_n", 100)})
	case "imsi_to_msisdns":
		base.RecordType = defaultRecordType(base.RecordType, RecordTypeCDR)
		return s.aggregateAsQuery(userID, AggregateRequest{QueryRequest: base, Operation: "count", GroupBy: []string{"imsi", "source_number"}, TopN: argInt(args, "top_n", 100)})
	case "plate_search", "sightings_by_plate":
		base.RecordType = defaultRecordType(base.RecordType, RecordTypeANPR)
		if plate := argString(args, "plate", "plate_number"); plate != "" {
			base.Filters = append(base.Filters, Filter{Field: "plate_number", Op: "contains", Value: plate})
		}
		return s.Query(userID, base)
	case "sightings_by_location", "plates_seen_near_location":
		base.RecordType = defaultRecordType(base.RecordType, RecordTypeANPR)
		if location := argString(args, "location"); location != "" {
			base.Filters = append(base.Filters, Filter{Field: "location", Op: "contains", Value: location})
		}
		return s.Query(userID, base)
	case "sightings_by_time_range":
		base.RecordType = defaultRecordType(base.RecordType, RecordTypeANPR)
		base.TimeRange = &TimeRange{Field: "timestamp", From: argString(args, "from", "start"), To: argString(args, "to", "end")}
		return s.Query(userID, base)
	case "repeat_plate_locations":
		base.RecordType = defaultRecordType(base.RecordType, RecordTypeANPR)
		resp, err := s.aggregateAsQuery(userID, AggregateRequest{QueryRequest: base, Operation: "count", GroupBy: []string{"plate_number", "location"}, TopN: argInt(args, "top_n", 100)})
		if err != nil {
			return resp, err
		}
		records := resp.Records[:0]
		for _, record := range resp.Records {
			if count, ok := numericValue(record["count"]); ok && count > 1 {
				records = append(records, record)
			}
		}
		resp.Records = records
		resp.Count = len(records)
		resp.Total = len(records)
		return resp, nil
	case "records_by_field":
		field := argString(args, "field")
		if field == "" {
			return QueryResponse{}, fmt.Errorf("records_by_field requires field")
		}
		base.Filters = append(base.Filters, Filter{Field: field, Op: "eq", Value: args["value"]})
		return s.Query(userID, base)
	case "distinct_values":
		return s.aggregateAsQuery(userID, AggregateRequest{QueryRequest: base, Operation: "distinct", Field: argString(args, "field")})
	case "count_by_field":
		return s.aggregateAsQuery(userID, AggregateRequest{QueryRequest: base, Operation: "count_by_field", Field: argString(args, "field"), TopN: argInt(args, "top_n", 50)})
	case "min_by_field":
		return s.aggregateAsQuery(userID, AggregateRequest{QueryRequest: base, Operation: "min", Field: argString(args, "field")})
	case "max_by_field":
		return s.aggregateAsQuery(userID, AggregateRequest{QueryRequest: base, Operation: "max", Field: argString(args, "field")})
	case "top_by_field":
		return s.aggregateAsQuery(userID, AggregateRequest{QueryRequest: base, Operation: "top_by_field", Field: argString(args, "field"), TopN: argInt(args, "top_n", 50)})
	case "timeline_by_entity":
		field := argString(args, "field", "entity_field")
		if field == "" {
			field = "msisdn"
		}
		if value := argString(args, "value", "entity"); value != "" {
			base.Filters = append(base.Filters, Filter{Field: field, Op: "eq", Value: value})
		}
		base.Sort = []SortField{{Field: "timestamp", Direction: "asc"}}
		return s.Query(userID, base)
	default:
		return QueryResponse{}, fmt.Errorf("unsupported records helper %q", req.Helper)
	}
}

func (s *Service) aggregateAsQuery(userID string, req AggregateRequest) (QueryResponse, error) {
	resp, err := s.Aggregate(userID, req)
	if err != nil {
		return QueryResponse{}, err
	}
	records := []map[string]any{}
	for _, group := range append(resp.Groups, resp.Results...) {
		row := map[string]any{}
		for k, v := range group.Key {
			row[k] = v
		}
		if group.Value != nil {
			row["value"] = group.Value
		}
		if group.Count != 0 {
			row["count"] = group.Count
		}
		if group.Record != nil {
			row["record"] = group.Record
		}
		records = append(records, row)
	}
	if len(resp.Values) > 0 {
		for _, value := range resp.Values {
			records = append(records, map[string]any{"value": value})
		}
	}
	if resp.Record != nil || resp.Value != nil {
		row := map[string]any{"value": resp.Value}
		if resp.Record != nil {
			row["record"] = resp.Record
		}
		records = append(records, row)
	}
	if len(records) == 0 && resp.Operation == "count" {
		records = append(records, map[string]any{"count": resp.Count})
	}
	return QueryResponse{Records: records, Count: len(records), Total: len(records), Limit: len(records)}, nil
}

func (s *Service) callsBetweenNumbers(userID string, req QueryRequest, a, b string) (QueryResponse, error) {
	req.Filters = nil
	records, batches, err := s.loadFilteredRecords(userID, req)
	if err != nil {
		return QueryResponse{}, err
	}
	filtered := records[:0]
	for _, record := range records {
		src, _ := recordField(record, "source_number")
		dst, _ := recordField(record, "target_number")
		forward := valuesEqual(src, a) && valuesEqual(dst, b)
		reverse := valuesEqual(src, b) && valuesEqual(dst, a)
		if forward || reverse {
			filtered = append(filtered, record)
		}
	}
	sortRecords(filtered, []SortField{{Field: "timestamp", Direction: "asc"}})
	limit := normalizeLimit(req.Limit)
	if len(filtered) < limit {
		limit = len(filtered)
	}
	out := make([]map[string]any, 0, limit)
	for _, record := range filtered[:limit] {
		out = append(out, projectRecord(record, req.Fields))
	}
	return QueryResponse{Records: out, Count: len(out), Total: len(filtered), Limit: limit, BatchIDs: batchIDs(batches)}, nil
}

func defaultRecordType(current, fallback string) string {
	if current != "" {
		return current
	}
	return fallback
}

func argString(args map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := args[key]; ok {
			return strings.TrimSpace(valueToString(value))
		}
	}
	return ""
}

func argInt(args map[string]any, key string, fallback int) int {
	if value, ok := args[key]; ok {
		if f, ok := numericValue(value); ok {
			return int(f)
		}
	}
	return fallback
}
