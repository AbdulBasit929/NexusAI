package main

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

func loadCoverageSummary(ctx context.Context, db *pgxpool.Pool, req hybridQueryRequest) CoverageSummary {
	if db == nil {
		return CoverageSummary{}
	}
	summary := CoverageSummary{}
	if rows, err := queryRows(ctx, db, req.TenantID, `
WITH canonical AS (
  SELECT "timestamp" AS event_time, record_type::text AS record_type
  FROM forensic.records
  WHERE tenant_id = $1 AND collection_id = $2
),
legacy AS (
  SELECT call_start_ts AS event_time, 'cdr'::text AS record_type
  FROM forensic.cdr_records
  WHERE tenant_id = $1 AND collection_id = $2
  UNION ALL
  SELECT observed_at AS event_time, record_type::text AS record_type
  FROM forensic.generic_records
  WHERE tenant_id = $1 AND collection_id = $2
),
events AS (
  SELECT event_time, record_type FROM canonical
  UNION ALL
  SELECT event_time, record_type FROM legacy
  WHERE NOT EXISTS (SELECT 1 FROM canonical)
)
SELECT min(event_time) AS collection_min_timestamp,
       max(event_time) AS collection_max_timestamp,
       count(*) AS total_indexed_records
FROM events`, req.TenantID, req.CollectionID); err == nil && len(rows) > 0 {
		row := rows[0]
		summary.CollectionMinTimestamp = stringValueAny(row["collection_min_timestamp"])
		summary.CollectionMaxTimestamp = stringValueAny(row["collection_max_timestamp"])
		summary.TotalIndexedRecords = valueAsInt(row["total_indexed_records"])
	}
	if rows, err := queryRows(ctx, db, req.TenantID, `
WITH metadata_counts AS (
  SELECT record_type::text AS record_type, coalesce(sum(inserted_rows), 0) AS count
  FROM forensic.kb_active_metadata
  WHERE tenant_id = $1 AND collection_id = $2
  GROUP BY record_type
),
canonical_counts AS (
  SELECT record_type::text AS record_type, count(*) AS count
  FROM forensic.records
  WHERE tenant_id = $1 AND collection_id = $2
  GROUP BY record_type
),
family_counts AS (
  SELECT record_type, count FROM metadata_counts
  UNION ALL
  SELECT c.record_type, c.count FROM canonical_counts c
  WHERE NOT EXISTS (SELECT 1 FROM metadata_counts m WHERE m.record_type = c.record_type)
)
SELECT record_type, count FROM family_counts
ORDER BY count DESC, record_type`, req.TenantID, req.CollectionID); err == nil {
		for _, row := range rows {
			summary.RecordFamiliesPresent = append(summary.RecordFamiliesPresent, RecordFamilyCoverage{
				RecordType: stringValueAny(row["record_type"]),
				Count:      valueAsInt(row["count"]),
			})
		}
	}
	summary.ValidTargetExamples = validTargetExamples(ctx, db, req)
	summary.NearestActivity = nearestActivity(ctx, db, req)
	return summary
}

func validTargetExamples(ctx context.Context, db *pgxpool.Pool, req hybridQueryRequest) []string {
	rows, err := queryRows(ctx, db, req.TenantID, `
SELECT entity_value, max(observed_at) AS last_seen, count(*) AS observations
FROM forensic.record_entities
WHERE tenant_id = $1 AND collection_id = $2
  AND entity_value IS NOT NULL AND entity_value <> ''
GROUP BY entity_value
ORDER BY observations DESC, last_seen DESC NULLS LAST
LIMIT 5`, req.TenantID, req.CollectionID)
	if err != nil {
		return nil
	}
	examples := make([]string, 0, len(rows))
	for _, row := range rows {
		if masked := maskTarget(stringValueAny(row["entity_value"])); masked != "" {
			examples = append(examples, masked)
		}
	}
	return examples
}

func nearestActivity(ctx context.Context, db *pgxpool.Pool, req hybridQueryRequest) NearestActivity {
	if strings.TrimSpace(req.DateFrom) == "" && strings.TrimSpace(req.DateTo) == "" {
		return NearestActivity{}
	}
	beforeBound := req.DateFrom
	afterBound := req.DateTo
	if beforeBound == "" {
		beforeBound = afterBound
	}
	if afterBound == "" {
		afterBound = beforeBound
	}
	rows, err := queryRows(ctx, db, req.TenantID, `
WITH events AS (
  SELECT call_start_ts AS event_time,
         array_remove(ARRAY[msisdn, call_org_num, call_dialed_num, imsi, imei, cell_site_id, location], NULL) AS targets
  FROM forensic.cdr_records
  WHERE tenant_id = $1 AND collection_id = $2
  UNION ALL
  SELECT observed_at AS event_time,
         array_remove(ARRAY[primary_entity, secondary_entity, location], NULL) AS targets
  FROM forensic.generic_records
  WHERE tenant_id = $1 AND collection_id = $2
),
filtered AS (
  SELECT event_time
  FROM events
  WHERE event_time IS NOT NULL
    AND (
      $3 = ''
      OR EXISTS (
        SELECT 1 FROM unnest(targets) AS target
        WHERE target ILIKE '%' || $3 || '%'
      )
    )
)
SELECT
  (SELECT max(event_time) FROM filtered WHERE event_time < $4::timestamptz) AS before,
  (SELECT min(event_time) FROM filtered WHERE event_time >= $5::timestamptz) AS after`,
		req.TenantID, req.CollectionID, req.Target, beforeBound, afterBound)
	if err != nil || len(rows) == 0 {
		return NearestActivity{}
	}
	return NearestActivity{
		Before: stringValueAny(rows[0]["before"]),
		After:  stringValueAny(rows[0]["after"]),
	}
}

func maskTarget(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.Contains(value, "@") {
		parts := strings.SplitN(value, "@", 2)
		if len(parts[0]) <= 2 {
			return "***@" + parts[1]
		}
		return parts[0][:2] + "***@" + parts[1]
	}
	digits := regexp.MustCompile(`\D`).ReplaceAllString(value, "")
	if len(digits) >= 8 {
		return digits[:minInt(4, len(digits))] + strings.Repeat("*", maxInt(0, len(digits)-6)) + digits[len(digits)-2:]
	}
	if len(value) <= 4 {
		return value
	}
	return fmt.Sprintf("%s***%s", value[:2], value[len(value)-2:])
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
