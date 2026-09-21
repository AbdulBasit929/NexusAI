package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"
	_ "time/tzdata"

	"github.com/jackc/pgx/v5/pgxpool"
)

const canonicalGroupRowLimit = 1000
const canonicalGroupLimit = maxHybridLimit

// Only non-identifier canonical fields are admitted in this first grouping
// slice. Source-native attributes require observed schema and privacy policy.
type CanonicalGroupV1 struct {
	Field    string `json:"field"`
	Measure  string `json:"measure"`
	Bucket   string `json:"bucket,omitempty"`
	Timezone string `json:"timezone,omitempty"`
	TopK     int    `json:"top_k,omitempty"`
}

func (g *CanonicalGroupV1) UnmarshalJSON(data []byte) error {
	type plain CanonicalGroupV1
	var decoded plain
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&decoded); err != nil {
		return err
	}
	*g = CanonicalGroupV1(decoded)
	return validateCanonicalGroup(g)
}

func validateCanonicalGroup(g *CanonicalGroupV1) error {
	if g == nil {
		return nil
	}
	if g.Field == "timestamp" {
		if g.Bucket != "hour" && g.Bucket != "day" && g.Bucket != "week" && g.Bucket != "month" {
			return fmt.Errorf("timestamp group requires hour, day, week or month")
		}
		if g.Timezone == "" || len(g.Timezone) > 128 {
			return fmt.Errorf("timestamp group requires an explicit IANA timezone")
		}
		if _, err := time.LoadLocation(g.Timezone); err != nil {
			return fmt.Errorf("unknown group timezone")
		}
	} else if g.Field != "record_type" && g.Field != "source_file" {
		return fmt.Errorf("group field is not an authorized canonical string field")
	}
	if g.Field != "timestamp" && (g.Bucket != "" || g.Timezone != "") {
		return fmt.Errorf("time bucket controls require timestamp grouping")
	}
	if g.Measure != "count" {
		return fmt.Errorf("group measure must be count")
	}
	if g.TopK < 0 || g.TopK > canonicalGroupLimit {
		return fmt.Errorf("group top_k must be between 0 and %d", canonicalGroupLimit)
	}
	return nil
}

func validateCanonicalGroupRequest(req hybridQueryRequest) error {
	if err := validateCanonicalGroup(req.Group); err != nil {
		return err
	}
	if req.Group != nil && (len(req.Projection) > 0 || req.Limit != 0 || req.Offset != 0 || req.SortBy != "" || req.SortDirection != "") {
		return fmt.Errorf("group cannot combine projection, pagination limit/offset or custom sort; order is count descending, null first, then exact value ascending")
	}
	if req.Group != nil && req.Group.Field == "timestamp" {
		from, e1 := time.Parse(time.RFC3339, req.DateFrom)
		to, e2 := time.Parse(time.RFC3339, req.DateTo)
		if e1 != nil || e2 != nil || !from.Before(to) {
			return fmt.Errorf("time buckets require an increasing explicit RFC3339 time range")
		}
	}
	return nil
}

// A single bounded SELECT supplies both counts and lineage from one snapshot.
// The extra row detects overflow without a racy count-then-fetch sequence.
func canonicalGroupedRecords(ctx context.Context, db *pgxpool.Pool, req hybridQueryRequest, built canonicalRecordsQuery) (map[string]any, error) {
	rows, err := queryRows(ctx, db, req.TenantID, `SELECT record_id::text AS record_id, tenant_id, collection_id, file_id, batch_id::text AS batch_id, record_type::text AS record_type, source_file, row_number, row_hash, timestamp, 'forensic.records'::text AS source_table FROM forensic.records `+built.WhereSQL+` ORDER BY record_id LIMIT 1001`, built.Args...)
	if err != nil {
		return nil, err
	}
	groups, err := groupCanonicalRows(req, rows)
	if err != nil {
		return nil, err
	}
	totalGroups := len(groups)
	if req.Group.TopK > 0 && len(groups) > req.Group.TopK {
		groups = groups[:req.Group.TopK]
	}
	return map[string]any{"canonical_groups": groups, "row_count": len(groups), "total_group_count": totalGroups, "selected_groups_only": len(groups) < totalGroups, "total_count": len(rows), "group": req.Group, "complete": true, "input_row_limit": canonicalGroupRowLimit, "group_limit": canonicalGroupLimit, "order_by": "count DESC, null FIRST, value ASC", "filters": built.Filters, "provenance": canonicalProvenance(rows, req.CollectionID)}, nil
}

func groupCanonicalRows(req hybridQueryRequest, rows []map[string]any) ([]map[string]any, error) {
	if req.Group == nil {
		return nil, fmt.Errorf("group is required")
	}
	if err := validateCanonicalGroupRequest(req); err != nil {
		return nil, err
	}
	if req.TenantID == "" || req.CollectionID == "" {
		return nil, fmt.Errorf("group requires authorized tenant and collection")
	}
	if len(rows) > canonicalGroupRowLimit {
		return nil, fmt.Errorf("group input exceeds %d rows; narrow authorized filters", canonicalGroupRowLimit)
	}
	type key struct {
		Null  bool
		Value string
	}
	buckets := map[key][]map[string]any{}
	seen := map[string]bool{}
	for _, row := range rows {
		if row["tenant_id"] != req.TenantID || row["collection_id"] != req.CollectionID {
			return nil, fmt.Errorf("group row is outside authorized scope")
		}
		id, ok := row["record_id"].(string)
		if !ok || id == "" || seen[id] {
			return nil, fmt.Errorf("group requires unique source record identities")
		}
		seen[id] = true
		value, exists := row[req.Group.Field]
		if !exists {
			return nil, fmt.Errorf("group field missing from canonical row")
		}
		k := key{Null: value == nil}
		if value != nil {
			if req.Group.Field == "timestamp" {
				stamp, valid := value.(time.Time)
				if serialized, ok := value.(string); ok {
					var parseErr error
					stamp, parseErr = time.Parse(time.RFC3339, serialized)
					valid = parseErr == nil
				}
				if !valid {
					return nil, fmt.Errorf("timestamp violates canonical time type")
				}
				start, _, err := canonicalTimeBucket(stamp, req.Group)
				if err != nil {
					return nil, err
				}
				k.Value, ok = start.Format(time.RFC3339), true
			} else {
				k.Value, ok = value.(string)
			}
			if !ok {
				return nil, fmt.Errorf("group field violates canonical string type")
			}
		}
		locator := map[string]any{}
		for _, field := range canonicalProjectionProvenance {
			locator[field] = row[field]
		}
		buckets[k] = append(buckets[k], locator)
		if len(buckets) > canonicalGroupLimit {
			return nil, fmt.Errorf("group cardinality exceeds %d", canonicalGroupLimit)
		}
	}
	keys := make([]key, 0, len(buckets))
	for k := range buckets {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := keys[i], keys[j]
		if len(buckets[a]) != len(buckets[b]) {
			return len(buckets[a]) > len(buckets[b])
		}
		if a.Null != b.Null {
			return a.Null
		}
		return a.Value < b.Value
	})
	groups := make([]map[string]any, 0, len(keys))
	for _, k := range keys {
		lineage := buckets[k]
		sort.Slice(lineage, func(i, j int) bool { return lineage[i]["record_id"].(string) < lineage[j]["record_id"].(string) })
		var value any = k.Value
		if k.Null {
			value = nil
		}
		group := map[string]any{"field": req.Group.Field, "value": value, "is_null": k.Null, "count": len(lineage), "metadata": map[string]any{"source_rows": lineage}}
		if req.Group.Field == "timestamp" && !k.Null {
			stamp, _ := time.Parse(time.RFC3339, k.Value)
			_, end, _ := canonicalTimeBucket(stamp, req.Group)
			group["bucket_end_exclusive"] = end.Format(time.RFC3339)
			group["timezone"] = req.Group.Timezone
		}
		groups = append(groups, group)
	}
	return groups, nil
}

// Calendar buckets preserve civil-time boundaries. Hour buckets retain the
// offset of a repeated DST hour; days/weeks/months use calendar arithmetic.
func canonicalTimeBucket(stamp time.Time, group *CanonicalGroupV1) (time.Time, time.Time, error) {
	zone, err := time.LoadLocation(group.Timezone)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	t := stamp.In(zone)
	start := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, zone)
	switch group.Bucket {
	case "hour":
		start = t.Add(-time.Duration(t.Minute())*time.Minute - time.Duration(t.Second())*time.Second - time.Duration(t.Nanosecond()))
		return start, start.Add(time.Hour), nil
	case "day":
		return start, start.AddDate(0, 0, 1), nil
	case "week":
		start = start.AddDate(0, 0, -(int(start.Weekday())+6)%7)
		return start, start.AddDate(0, 0, 7), nil
	case "month":
		start = time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, zone)
		return start, start.AddDate(0, 1, 0), nil
	default:
		return time.Time{}, time.Time{}, fmt.Errorf("unsupported bucket")
	}
}
