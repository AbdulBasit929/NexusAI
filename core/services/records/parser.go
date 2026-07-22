package records

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const maxJSONLLineBytes = 16 * 1024 * 1024

type ParsedRow struct {
	RowNumber int            `json:"row_number"`
	Fields    map[string]any `json:"fields"`
}

type ParsedData struct {
	Rows       []ParsedRow   `json:"rows"`
	Fields     []FieldSchema `json:"fields"`
	RecordType string        `json:"record_type"`
	Errors     []IngestError `json:"errors,omitempty"`
}

func Parse(sourceFile string, r io.Reader) (*ParsedData, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	ext := strings.ToLower(filepath.Ext(sourceFile))

	var parsed *ParsedData
	switch ext {
	case ".csv":
		parsed, err = parseDelimited(data, ',')
	case ".tsv":
		parsed, err = parseDelimited(data, '\t')
	case ".json":
		parsed, err = parseJSONArray(data)
	case ".jsonl", ".ndjson":
		parsed, err = parseJSONLines(data)
	case ".txt", ".log":
		parsed, err = parseLogLines(data)
	default:
		parsed, err = parseByContent(data)
	}
	if err != nil {
		return nil, err
	}
	parsed.Fields = InferSchema(parsed.Rows)
	parsed.RecordType = inferRecordTypeFromFields(parsed.Fields)
	return parsed, nil
}

func parseByContent(data []byte) (*ParsedData, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return &ParsedData{}, nil
	}
	switch trimmed[0] {
	case '[':
		return parseJSONArray(data)
	case '{':
		if parsed, err := parseJSONLines(data); err == nil && len(parsed.Rows) > 0 {
			return parsed, nil
		}
	}
	firstLine := string(trimmed)
	if idx := strings.IndexAny(firstLine, "\r\n"); idx >= 0 {
		firstLine = firstLine[:idx]
	}
	delimiter := ','
	if strings.Count(firstLine, "\t") > strings.Count(firstLine, ",") {
		delimiter = '\t'
	}
	if strings.Count(firstLine, string(delimiter)) > 0 {
		return parseDelimited(data, rune(delimiter))
	}
	return parseLogLines(data)
}

func parseDelimited(data []byte, delimiter rune) (*ParsedData, error) {
	reader := csv.NewReader(bytes.NewReader(data))
	reader.Comma = delimiter
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true

	headers, err := reader.Read()
	if err == io.EOF {
		return &ParsedData{}, nil
	}
	if err != nil {
		return nil, err
	}
	headers = normalizeHeaders(headers)

	var rows []ParsedRow
	var errs []IngestError
	rowNumber := 1
	for {
		rowNumber++
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			errs = append(errs, IngestError{RowNumber: rowNumber, Message: err.Error()})
			continue
		}
		fields := map[string]any{}
		for i, header := range headers {
			if i < len(record) {
				fields[header] = strings.TrimSpace(record[i])
			} else {
				fields[header] = ""
			}
		}
		for i := len(headers); i < len(record); i++ {
			fields[fmt.Sprintf("extra_%d", i-len(headers)+1)] = strings.TrimSpace(record[i])
		}
		rows = append(rows, ParsedRow{RowNumber: rowNumber, Fields: fields})
	}
	return &ParsedData{Rows: rows, Errors: errs}, nil
}

func normalizeHeaders(headers []string) []string {
	seen := map[string]int{}
	out := make([]string, len(headers))
	for i, h := range headers {
		h = strings.TrimSpace(strings.TrimPrefix(h, "\ufeff"))
		if h == "" {
			h = fmt.Sprintf("column_%d", i+1)
		}
		key := NormalizeFieldName(h)
		seen[key]++
		if seen[key] > 1 {
			h = fmt.Sprintf("%s_%d", h, seen[key])
		}
		out[i] = h
	}
	return out
}

func parseJSONArray(data []byte) (*ParsedData, error) {
	var arr []map[string]any
	if err := json.Unmarshal(data, &arr); err != nil {
		var one map[string]any
		if oneErr := json.Unmarshal(data, &one); oneErr != nil {
			return nil, err
		}
		arr = []map[string]any{one}
	}
	rows := make([]ParsedRow, 0, len(arr))
	for i, obj := range arr {
		rows = append(rows, ParsedRow{RowNumber: i + 1, Fields: flattenMap(obj)})
	}
	return &ParsedData{Rows: rows}, nil
}

func parseJSONLines(data []byte) (*ParsedData, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 64*1024), maxJSONLLineBytes)
	var rows []ParsedRow
	var errs []IngestError
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			errs = append(errs, IngestError{RowNumber: lineNumber, Message: err.Error(), Raw: line})
			continue
		}
		rows = append(rows, ParsedRow{RowNumber: lineNumber, Fields: flattenMap(obj)})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(rows) == 0 && len(errs) > 0 {
		return nil, fmt.Errorf("no valid JSONL records found")
	}
	return &ParsedData{Rows: rows, Errors: errs}, nil
}

func parseLogLines(data []byte) (*ParsedData, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 64*1024), maxJSONLLineBytes)
	var rows []ParsedRow
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := parseKeyValueLine(line)
		if len(fields) == 0 {
			fields = map[string]any{"line": line}
			if ts, ok := leadingTimestamp(line); ok {
				fields["timestamp"] = ts
			}
		}
		rows = append(rows, ParsedRow{RowNumber: lineNumber, Fields: fields})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return &ParsedData{Rows: rows}, nil
}

func parseKeyValueLine(line string) map[string]any {
	fields := map[string]any{}
	parts := strings.Fields(line)
	for _, part := range parts {
		key, value, ok := strings.Cut(part, "=")
		if !ok {
			key, value, ok = strings.Cut(part, ":")
		}
		if !ok {
			continue
		}
		key = strings.Trim(strings.TrimSpace(key), `"'[](),`)
		value = strings.Trim(strings.TrimSpace(value), `"'[](),`)
		if key == "" || value == "" {
			continue
		}
		fields[key] = value
	}
	if len(fields) < 2 {
		return nil
	}
	return fields
}

func leadingTimestamp(line string) (string, bool) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return "", false
	}
	candidates := []string{fields[0]}
	if len(fields) > 1 {
		candidates = append(candidates, fields[0]+" "+fields[1])
	}
	for _, c := range candidates {
		if t, ok := parseTimeValue(c); ok {
			return t.Format(time.RFC3339), true
		}
	}
	return "", false
}

func flattenMap(in map[string]any) map[string]any {
	out := map[string]any{}
	var walk func(prefix string, m map[string]any)
	walk = func(prefix string, m map[string]any) {
		for k, v := range m {
			name := k
			if prefix != "" {
				name = prefix + "." + k
			}
			if nested, ok := v.(map[string]any); ok {
				walk(name, nested)
				continue
			}
			out[name] = v
		}
	}
	walk("", in)
	return out
}

func InferSchema(rows []ParsedRow) []FieldSchema {
	type fieldStats struct {
		field   FieldSchema
		types   map[string]bool
		samples []string
	}
	order := []string{}
	stats := map[string]*fieldStats{}
	for _, row := range rows {
		for name, value := range row.Fields {
			if _, ok := stats[name]; !ok {
				normalized := NormalizeFieldName(name)
				canonical := CanonicalFieldName(name)
				stats[name] = &fieldStats{
					field: FieldSchema{
						OriginalName:   name,
						NormalizedName: normalized,
						CanonicalName:  canonical,
					},
					types: map[string]bool{},
				}
				if canonical != normalized {
					stats[name].field.DetectedAliases = []string{normalized}
				}
				order = append(order, name)
			}
			typ := inferFieldType(value)
			if typ == FieldTypeEmpty {
				continue
			}
			stats[name].types[typ] = true
			stats[name].field.NonEmptyCount++
			sample := valueToString(value)
			if sample != "" && len(stats[name].samples) < 5 && !slicesContains(stats[name].samples, sample) {
				stats[name].samples = append(stats[name].samples, sample)
			}
		}
	}
	fields := make([]FieldSchema, 0, len(order))
	for _, name := range order {
		st := stats[name]
		st.field.Type = collapseTypes(st.types)
		st.field.SampleValues = st.samples
		fields = append(fields, st.field)
	}
	return fields
}

func inferFieldType(value any) string {
	switch v := value.(type) {
	case nil:
		return FieldTypeEmpty
	case bool:
		return FieldTypeBool
	case int, int64, float32, float64, json.Number:
		return FieldTypeNumber
	case string:
		text := strings.TrimSpace(v)
		if text == "" {
			return FieldTypeEmpty
		}
		if shouldParseNumericString(text) {
			return FieldTypeNumber
		}
		if _, ok := strconv.ParseBool(strings.ToLower(text)); ok == nil {
			return FieldTypeBool
		}
		if _, ok := parseTimeValue(text); ok {
			return FieldTypeTime
		}
		return FieldTypeString
	default:
		return FieldTypeString
	}
}

func collapseTypes(types map[string]bool) string {
	if len(types) == 0 {
		return FieldTypeEmpty
	}
	if len(types) == 1 {
		for typ := range types {
			return typ
		}
	}
	if types[FieldTypeNumber] && len(types) == 2 && types[FieldTypeString] {
		return FieldTypeMixed
	}
	return FieldTypeMixed
}

func normalizeValue(value any) any {
	switch v := value.(type) {
	case nil:
		return nil
	case string:
		text := strings.TrimSpace(v)
		if text == "" {
			return ""
		}
		if shouldParseNumericString(text) {
			parsed, _ := strconv.ParseFloat(strings.ReplaceAll(text, ",", ""), 64)
			return parsed
		}
		if parsed, err := strconv.ParseBool(strings.ToLower(text)); err == nil {
			return parsed
		}
		if parsed, ok := parseTimeValue(text); ok {
			return parsed.Format(time.RFC3339)
		}
		return text
	case json.Number:
		if parsed, err := v.Float64(); err == nil {
			return parsed
		}
		return v.String()
	default:
		return v
	}
}

func shouldParseNumericString(text string) bool {
	cleaned := strings.ReplaceAll(strings.TrimSpace(text), ",", "")
	if cleaned == "" {
		return false
	}
	unsigned := strings.TrimPrefix(cleaned, "-")
	if len(unsigned) > 1 && strings.HasPrefix(unsigned, "0") && !strings.Contains(unsigned, ".") {
		return false
	}
	digitsOnly := true
	for _, ch := range unsigned {
		if ch < '0' || ch > '9' {
			digitsOnly = false
			break
		}
	}
	if digitsOnly && len(unsigned) > 15 {
		return false
	}
	_, err := strconv.ParseFloat(cleaned, 64)
	return err == nil
}

func parseTimeValue(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
		"02-01-2006 15:04:05",
		"02-01-2006 15:04",
		"02-01-2006",
		"01/02/2006 15:04:05",
		"01/02/2006 15:04",
		"01/02/2006",
		"02/01/2006 15:04:05",
		"02/01/2006 15:04",
		"02/01/2006",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, value); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func valueToString(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	default:
		return fmt.Sprint(v)
	}
}

func slicesContains(values []string, candidate string) bool {
	for _, v := range values {
		if v == candidate {
			return true
		}
	}
	return false
}
