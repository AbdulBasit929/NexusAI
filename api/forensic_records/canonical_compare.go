package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"io"
	"time"
)

type CanonicalComparisonOperandV1 struct {
	Value *string `json:"value,omitempty"`
	From  string  `json:"from,omitempty"`
	To    string  `json:"to,omitempty"`
}
type CanonicalCompareV1 struct {
	Field   string                       `json:"field"`
	Measure string                       `json:"measure"`
	A       CanonicalComparisonOperandV1 `json:"a"`
	B       CanonicalComparisonOperandV1 `json:"b"`
}

func (c *CanonicalCompareV1) UnmarshalJSON(raw []byte) error {
	type plain CanonicalCompareV1
	var p plain
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&p); err != nil {
		return err
	}
	if d.Decode(&struct{}{}) != io.EOF {
		return fmt.Errorf("comparison trailing content")
	}
	*c = CanonicalCompareV1(p)
	return validateCanonicalCompare(c)
}
func validateCanonicalCompare(c *CanonicalCompareV1) error {
	if c == nil {
		return nil
	}
	if c.Measure != "count" {
		return fmt.Errorf("comparison supports exact source-row count only")
	}
	if c.Field != "source_file" && c.Field != "record_type" && c.Field != "timestamp" {
		return fmt.Errorf("comparison field is not authorized")
	}
	for _, p := range []CanonicalComparisonOperandV1{c.A, c.B} {
		if c.Field == "timestamp" {
			from, e1 := time.Parse(time.RFC3339, p.From)
			to, e2 := time.Parse(time.RFC3339, p.To)
			if p.Value != nil || e1 != nil || e2 != nil || !from.Before(to) {
				return fmt.Errorf("comparison periods require increasing explicit RFC3339 bounds")
			}
		} else if p.Value == nil || len(*p.Value) > 512 || p.From != "" || p.To != "" {
			return fmt.Errorf("comparison requires two bounded exact field values")
		}
	}
	if c.Field == "timestamp" && c.A.From == c.B.From && c.A.To == c.B.To || c.Field != "timestamp" && *c.A.Value == *c.B.Value {
		return fmt.Errorf("comparison operands must differ")
	}
	return nil
}
func validateCanonicalCompareRequest(req hybridQueryRequest) error {
	if req.Compare == nil {
		return nil
	}
	if err := validateCanonicalCompare(req.Compare); err != nil {
		return err
	}
	if req.Group != nil || len(req.Projection) > 0 || req.Limit != 0 || req.Offset != 0 || req.SortBy != "" || req.SortDirection != "" {
		return fmt.Errorf("comparison cannot combine grouping, projection, pagination or custom ordering")
	}
	return nil
}
func canonicalComparedRecords(ctx context.Context, db *pgxpool.Pool, req hybridQueryRequest, built canonicalRecordsQuery) (map[string]any, error) {
	rows, err := queryRows(ctx, db, req.TenantID, `SELECT record_id::text AS record_id,tenant_id,collection_id,file_id,batch_id::text AS batch_id,record_type::text AS record_type,source_file,row_number,row_hash,timestamp,'forensic.records'::text AS source_table FROM forensic.records `+built.WhereSQL+` ORDER BY record_id LIMIT 1001`, built.Args...)
	if err != nil {
		return nil, err
	}
	return compareCanonicalRows(req, rows)
}
func compareCanonicalRows(req hybridQueryRequest, rows []map[string]any) (map[string]any, error) {
	if req.Compare == nil {
		return nil, fmt.Errorf("comparison required")
	}
	if err := validateCanonicalCompareRequest(req); err != nil {
		return nil, err
	}
	if req.TenantID == "" || req.CollectionID == "" || len(rows) > canonicalGroupRowLimit {
		return nil, fmt.Errorf("comparison requires bounded authorized input")
	}
	operands := []CanonicalComparisonOperandV1{req.Compare.A, req.Compare.B}
	contributions := [][]map[string]any{{}, {}}
	seen := map[string]bool{}
	overlap := 0
	matched := 0
	for _, row := range rows {
		id, ok := row["record_id"].(string)
		if !ok || id == "" || seen[id] || row["tenant_id"] != req.TenantID || row["collection_id"] != req.CollectionID {
			return nil, fmt.Errorf("comparison source identity or scope violation")
		}
		seen[id] = true
		value, exists := row[req.Compare.Field]
		if !exists || value == nil {
			return nil, fmt.Errorf("comparison field unavailable; unknown is not zero")
		}
		matches := 0
		for i, p := range operands {
			include := false
			if req.Compare.Field == "timestamp" {
				stamp, err := time.Parse(time.RFC3339, stringValueAny(value))
				if err != nil {
					return nil, fmt.Errorf("comparison timestamp malformed")
				}
				from, _ := time.Parse(time.RFC3339, p.From)
				to, _ := time.Parse(time.RFC3339, p.To)
				include = !stamp.Before(from) && stamp.Before(to)
			} else {
				v, ok := value.(string)
				if !ok {
					return nil, fmt.Errorf("comparison field type mismatch")
				}
				include = v == *p.Value
			}
			if include {
				locator := map[string]any{}
				for _, field := range canonicalProjectionProvenance {
					locator[field] = row[field]
				}
				contributions[i] = append(contributions[i], locator)
				matches++
			}
		}
		if matches > 0 {
			matched++
		}
		if matches == 2 {
			overlap++
		}
	}
	a, b := len(contributions[0]), len(contributions[1])
	result := []map[string]any{}
	for i, p := range operands {
		var value any
		if p.Value != nil {
			value = *p.Value
		}
		result = append(result, map[string]any{"operand": []string{"A", "B"}[i], "count": len(contributions[i]), "field": req.Compare.Field, "value": value, "from": p.From, "to_exclusive": p.To, "metadata": map[string]any{"source_rows": contributions[i]}})
	}
	result[1]["difference_from_a"] = b - a
	result[1]["overlapping_source_rows"] = overlap
	return map[string]any{"canonical_comparison": result, "row_count": 2, "total_count": matched, "scanned_source_rows": len(rows), "measure": "source_row_count", "field": req.Compare.Field, "difference_b_minus_a": b - a, "overlapping_source_rows": overlap, "complete": true, "input_row_limit": canonicalGroupRowLimit, "provenance": canonicalProvenance(rows, req.CollectionID)}, nil
}
