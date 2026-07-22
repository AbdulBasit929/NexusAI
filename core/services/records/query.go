package records

import (
	"cmp"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"
)

const (
	defaultQueryLimit     = 100
	maxQueryLimit         = 1000
	defaultTopN           = 10
	defaultCorrelateLimit = 200
)

func (s *Service) Query(userID string, req QueryRequest) (QueryResponse, error) {
	if strings.TrimSpace(req.Helper) != "" {
		return s.RunHelper(userID, req)
	}
	records, batches, err := s.loadFilteredRecords(userID, req)
	if err != nil {
		return QueryResponse{}, err
	}
	sortRecords(records, req.Sort)

	total := len(records)
	limit := normalizeLimit(req.Limit)
	offset := req.Offset
	if offset < 0 {
		offset = 0
	}
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}

	projected := make([]map[string]any, 0, end-offset)
	for _, record := range records[offset:end] {
		projected = append(projected, projectRecord(record, req.Fields))
	}

	return QueryResponse{
		Records:  projected,
		Count:    len(projected),
		Total:    total,
		Limit:    limit,
		Offset:   offset,
		BatchIDs: batchIDs(batches),
	}, nil
}

func (s *Service) Aggregate(userID string, req AggregateRequest) (AggregateResponse, error) {
	req.QueryRequest.Limit = 0
	req.QueryRequest.Offset = 0
	req.QueryRequest.Sort = nil
	req.QueryRequest.Fields = nil

	records, _, err := s.loadFilteredRecords(userID, req.QueryRequest)
	if err != nil {
		return AggregateResponse{}, err
	}

	op := strings.ToLower(strings.TrimSpace(req.Operation))
	if op == "" {
		op = "count"
	}
	resp := AggregateResponse{Operation: op, Field: req.Field, Count: len(records)}
	switch op {
	case "count":
		if len(req.GroupBy) == 0 {
			return resp, nil
		}
		resp.Groups = groupCounts(records, req.GroupBy, req.TopN)
		return resp, nil
	case "distinct", "distinct_values":
		values := distinctValues(records, req.Field)
		resp.Values = values
		resp.Count = len(values)
		return resp, nil
	case "min", "max":
		value, record, ok := extremum(records, req.Field, op == "max")
		if !ok {
			return resp, nil
		}
		resp.Value = value
		resp.Record = projectRecord(record, nil)
		return resp, nil
	case "top", "top_n", "top_by_field", "count_by_field":
		resp.Results = topValues(records, req.Field, req.TopN)
		return resp, nil
	case "min_by_field", "max_by_field":
		if len(req.GroupBy) == 0 {
			return AggregateResponse{}, fmt.Errorf("group_by is required for %s", op)
		}
		resp.Groups = groupedExtrema(records, req.GroupBy, req.Field, op == "max_by_field", req.TopN)
		return resp, nil
	default:
		return AggregateResponse{}, fmt.Errorf("unsupported aggregate operation %q", req.Operation)
	}
}

func (s *Service) Correlate(userID string, req CorrelateRequest) (CorrelateResponse, error) {
	left, _, err := s.loadFilteredRecords(userID, req.Left)
	if err != nil {
		return CorrelateResponse{}, err
	}
	right, _, err := s.loadFilteredRecords(userID, req.Right)
	if err != nil {
		return CorrelateResponse{}, err
	}
	entityFields := req.EntityFields
	if len(entityFields) == 0 {
		entityFields = []string{"msisdn", "imei", "imsi", "plate_number", "ip", "source_ip", "target_ip"}
	}
	timeField := cmp.Or(req.TimeField, "timestamp")
	window := req.TimeWindowSeconds
	if window <= 0 {
		window = 300
	}
	limit := req.Limit
	if limit <= 0 {
		limit = defaultCorrelateLimit
	}
	if limit > maxQueryLimit {
		limit = maxQueryLimit
	}

	rightByEntity := map[string][]Record{}
	for _, record := range right {
		for _, key := range entityKeys(record, entityFields) {
			rightByEntity[key] = append(rightByEntity[key], record)
		}
	}

	var matches []CorrelationMatch
	for _, l := range left {
		leftTime, leftHasTime := fieldTime(l, timeField)
		for _, key := range entityKeys(l, entityFields) {
			for _, r := range rightByEntity[key] {
				delta := int64(0)
				if leftHasTime {
					if rightTime, ok := fieldTime(r, timeField); ok {
						delta = int64(math.Abs(leftTime.Sub(rightTime).Seconds()))
						if delta > int64(window) {
							continue
						}
					}
				}
				matches = append(matches, CorrelationMatch{
					Entity:       decodeEntityKey(key),
					TimeDeltaSec: delta,
					Left:         projectRecord(l, nil),
					Right:        projectRecord(r, nil),
				})
				if len(matches) >= limit {
					return CorrelateResponse{Matches: matches, Count: len(matches)}, nil
				}
			}
		}
	}
	return CorrelateResponse{Matches: matches, Count: len(matches)}, nil
}

func (s *Service) loadFilteredRecords(userID string, req QueryRequest) ([]Record, []Batch, error) {
	records, batches, err := s.store.LoadRecords(userID, req)
	if err != nil {
		return nil, nil, err
	}
	filtered := records[:0]
	for _, record := range records {
		if !matchesFilters(record, req.Filters) {
			continue
		}
		if !matchesTimeRange(record, req.TimeRange) {
			continue
		}
		filtered = append(filtered, record)
	}
	return filtered, batches, nil
}

func matchesFilters(record Record, filters []Filter) bool {
	for _, filter := range filters {
		if strings.TrimSpace(filter.Field) == "" {
			return false
		}
		value, exists := recordField(record, filter.Field)
		op := strings.ToLower(strings.TrimSpace(filter.Op))
		if op == "" {
			op = "eq"
		}
		switch op {
		case "exists":
			if !exists {
				return false
			}
		case "not_exists":
			if exists {
				return false
			}
		case "eq", "=", "equals":
			if !exists || !valuesEqual(value, filter.Value) {
				return false
			}
		case "ne", "!=", "not_equals":
			if exists && valuesEqual(value, filter.Value) {
				return false
			}
		case "contains":
			if !exists || !strings.Contains(strings.ToLower(valueToString(value)), strings.ToLower(valueToString(filter.Value))) {
				return false
			}
		case "gt", ">", "gte", ">=", "lt", "<", "lte", "<=":
			if !exists || !compareWithOp(value, filter.Value, op) {
				return false
			}
		case "in":
			values := filter.Values
			if len(values) == 0 && filter.Value != nil {
				if arr, ok := filter.Value.([]any); ok {
					values = arr
				}
			}
			found := false
			for _, candidate := range values {
				if valuesEqual(value, candidate) {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func matchesTimeRange(record Record, tr *TimeRange) bool {
	if tr == nil || (tr.From == "" && tr.To == "") {
		return true
	}
	field := cmp.Or(tr.Field, "timestamp")
	valueTime, ok := fieldTime(record, field)
	if !ok {
		return false
	}
	if tr.From != "" {
		from, ok := parseTimeValue(tr.From)
		if !ok || valueTime.Before(from) {
			return false
		}
	}
	if tr.To != "" {
		to, ok := parseTimeValue(tr.To)
		if !ok || valueTime.After(to) {
			return false
		}
	}
	return true
}

func sortRecords(records []Record, sortFields []SortField) {
	if len(sortFields) == 0 {
		return
	}
	slices.SortStableFunc(records, func(a, b Record) int {
		for _, sf := range sortFields {
			av, _ := recordField(a, sf.Field)
			bv, _ := recordField(b, sf.Field)
			result := compareValues(av, bv)
			if strings.EqualFold(sf.Direction, "desc") {
				result = -result
			}
			if result != 0 {
				return result
			}
		}
		return cmp.Compare(a.ID, b.ID)
	})
}

func recordField(record Record, field string) (any, bool) {
	switch NormalizeFieldName(field) {
	case "id", "record_id":
		return record.ID, true
	case "batch_id":
		return record.BatchID, true
	case "collection_name", "collection":
		return record.CollectionName, true
	case "record_type":
		return record.RecordType, true
	case "source_file":
		return record.SourceFile, true
	case "source_entry":
		return record.SourceEntry, record.SourceEntry != ""
	case "row_number", "row":
		return record.RowNumber, true
	case "timestamp":
		if record.Timestamp != "" {
			return record.Timestamp, true
		}
	case "raw_fields":
		return record.RawFields, true
	}
	normalized := CanonicalFieldName(field)
	if value, ok := record.NormalizedFields[normalized]; ok {
		return value, true
	}
	plain := NormalizeFieldName(field)
	if value, ok := record.NormalizedFields[plain]; ok {
		return value, true
	}
	if value, ok := record.RawFields[field]; ok {
		return value, true
	}
	for key, value := range record.RawFields {
		if NormalizeFieldName(key) == plain || CanonicalFieldName(key) == normalized {
			return value, true
		}
	}
	return nil, false
}

func projectRecord(record Record, fields []string) map[string]any {
	out := map[string]any{}
	if len(fields) > 0 {
		for _, field := range fields {
			if value, ok := recordField(record, field); ok {
				out[field] = value
			}
		}
		return out
	}
	out["id"] = record.ID
	out["batch_id"] = record.BatchID
	out["record_type"] = record.RecordType
	out["collection_name"] = record.CollectionName
	out["source_file"] = record.SourceFile
	out["source_entry"] = record.SourceEntry
	out["row_number"] = record.RowNumber
	if record.Timestamp != "" {
		out["timestamp"] = record.Timestamp
	}
	for key, value := range record.NormalizedFields {
		out[key] = value
	}
	return out
}

func valuesEqual(a, b any) bool {
	if af, ok := numericValue(a); ok {
		if bf, ok := numericValue(b); ok {
			return af == bf
		}
	}
	if at, ok := anyTime(a); ok {
		if bt, ok := anyTime(b); ok {
			return at.Equal(bt)
		}
	}
	return strings.EqualFold(valueToString(a), valueToString(b))
}

func compareWithOp(a, b any, op string) bool {
	result := compareValues(a, b)
	switch op {
	case "gt", ">":
		return result > 0
	case "gte", ">=":
		return result >= 0
	case "lt", "<":
		return result < 0
	case "lte", "<=":
		return result <= 0
	default:
		return false
	}
}

func compareValues(a, b any) int {
	if af, ok := numericValue(a); ok {
		if bf, ok := numericValue(b); ok {
			return cmp.Compare(af, bf)
		}
	}
	if at, ok := anyTime(a); ok {
		if bt, ok := anyTime(b); ok {
			return at.Compare(bt)
		}
	}
	return cmp.Compare(strings.ToLower(valueToString(a)), strings.ToLower(valueToString(b)))
}

func numericValue(value any) (float64, bool) {
	switch v := value.(type) {
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case float32:
		return float64(v), true
	case float64:
		return v, true
	case json.Number:
		f, err := v.Float64()
		return f, err == nil
	case string:
		if !shouldParseNumericString(v) {
			return 0, false
		}
		f, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(v), ",", ""), 64)
		return f, err == nil
	default:
		return 0, false
	}
}

func anyTime(value any) (time.Time, bool) {
	if t, ok := value.(time.Time); ok {
		return t, true
	}
	return parseTimeValue(valueToString(value))
}

func fieldTime(record Record, field string) (time.Time, bool) {
	if value, ok := recordField(record, field); ok {
		return anyTime(value)
	}
	return time.Time{}, false
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return defaultQueryLimit
	}
	if limit > maxQueryLimit {
		return maxQueryLimit
	}
	return limit
}

func batchIDs(batches []Batch) []string {
	out := make([]string, 0, len(batches))
	for _, batch := range batches {
		out = append(out, batch.ID)
	}
	return out
}

func groupCounts(records []Record, fields []string, topN int) []GroupResult {
	counts := map[string]GroupResult{}
	for _, record := range records {
		key := groupKey(record, fields)
		encoded := encodeKey(key)
		result := counts[encoded]
		result.Key = key
		result.Count++
		counts[encoded] = result
	}
	return sortedGroups(counts, topN)
}

func distinctValues(records []Record, field string) []any {
	seen := map[string]any{}
	for _, record := range records {
		value, ok := recordField(record, field)
		if !ok || valueToString(value) == "" {
			continue
		}
		seen[strings.ToLower(valueToString(value))] = value
	}
	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	values := make([]any, 0, len(keys))
	for _, key := range keys {
		values = append(values, seen[key])
	}
	return values
}

func extremum(records []Record, field string, max bool) (any, Record, bool) {
	var bestValue any
	var bestRecord Record
	found := false
	for _, record := range records {
		value, ok := recordField(record, field)
		if !ok || valueToString(value) == "" {
			continue
		}
		if !found {
			bestValue = value
			bestRecord = record
			found = true
			continue
		}
		result := compareValues(value, bestValue)
		if (max && result > 0) || (!max && result < 0) {
			bestValue = value
			bestRecord = record
		}
	}
	return bestValue, bestRecord, found
}

func topValues(records []Record, field string, topN int) []GroupResult {
	if topN <= 0 {
		topN = defaultTopN
	}
	counts := map[string]GroupResult{}
	for _, record := range records {
		value, ok := recordField(record, field)
		if !ok || valueToString(value) == "" {
			continue
		}
		key := map[string]any{field: value}
		encoded := strings.ToLower(valueToString(value))
		result := counts[encoded]
		result.Key = key
		result.Value = value
		result.Count++
		counts[encoded] = result
	}
	return sortedGroups(counts, topN)
}

func groupedExtrema(records []Record, groupBy []string, field string, max bool, topN int) []GroupResult {
	groups := map[string]GroupResult{}
	for _, record := range records {
		value, ok := recordField(record, field)
		if !ok || valueToString(value) == "" {
			continue
		}
		key := groupKey(record, groupBy)
		encoded := encodeKey(key)
		current, exists := groups[encoded]
		if !exists || (max && compareValues(value, current.Value) > 0) || (!max && compareValues(value, current.Value) < 0) {
			groups[encoded] = GroupResult{
				Key:    key,
				Value:  value,
				Count:  1,
				Record: projectRecord(record, nil),
			}
		} else {
			current.Count++
			groups[encoded] = current
		}
	}
	return sortedGroups(groups, topN)
}

func groupKey(record Record, fields []string) map[string]any {
	key := map[string]any{}
	for _, field := range fields {
		if value, ok := recordField(record, field); ok {
			key[field] = value
		}
	}
	return key
}

func encodeKey(key map[string]any) string {
	data, _ := json.Marshal(key)
	return string(data)
}

func sortedGroups(groups map[string]GroupResult, topN int) []GroupResult {
	results := make([]GroupResult, 0, len(groups))
	for _, result := range groups {
		results = append(results, result)
	}
	slices.SortFunc(results, func(a, b GroupResult) int {
		if a.Count != b.Count {
			return cmp.Compare(b.Count, a.Count)
		}
		return cmp.Compare(encodeKey(a.Key), encodeKey(b.Key))
	})
	if topN > 0 && len(results) > topN {
		results = results[:topN]
	}
	return results
}

func entityKeys(record Record, fields []string) []string {
	keys := []string{}
	for _, field := range fields {
		value, ok := recordField(record, field)
		if !ok || valueToString(value) == "" {
			continue
		}
		keys = append(keys, encodeKey(map[string]any{"value": value}))
	}
	return keys
}

func decodeEntityKey(key string) map[string]any {
	var out map[string]any
	if err := json.Unmarshal([]byte(key), &out); err != nil {
		return map[string]any{"value": key}
	}
	return out
}
