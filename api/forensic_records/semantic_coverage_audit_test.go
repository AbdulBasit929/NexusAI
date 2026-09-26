package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// THE ANSWERABLE SURFACE: how much of the data the product can actually be
// asked about.
//
// Every measurement in this project so far scores 62 questions someone wrote
// down. The product's promise is UNBOUNDED questions, and the 62 cannot measure
// that — they can only tell us we have not regressed on what we already knew to
// ask. This audit measures the thing that actually bounds open-ended questions:
// **how many of the columns present in the evidence are described well enough
// for a verified plan to reference them.**
//
// The mechanism is the keystone working as designed. A field the curated layer
// does not describe is never issued in the enum, so the generator physically
// cannot name it, so no plan can filter, group or measure on it. That is
// correct and safe — it is also, precisely, the ceiling on what an analyst can
// ask. **Coverage of the layer IS coverage of the product.**
//
// An uncovered column is not a bug in the compiler, the model, the guards or
// the gates. It is a curation gap, and it is invisible to the golden suite
// unless someone happened to write a question about that column.
//
// Read-only: one SELECT of payload keys. No model, no writes.
//
// Run: FAMILY_AUDIT_DB=postgresql://... go test -run TestSemanticLayerCoverage -v
func TestSemanticLayerCoverage(t *testing.T) {
	dsn := os.Getenv("FAMILY_AUDIT_DB")
	if dsn == "" {
		t.Skip("diagnostic only; set FAMILY_AUDIT_DB")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	layer, source := defaultSemanticLayer()
	if layer == nil {
		t.Fatalf("semantic layer unavailable (%s)", source)
	}

	// Every raw column name the curated layer can resolve, per record type.
	// A field aliases several source names, and matching must ignore case,
	// spaces and separators the way the executor does -- `Site Code` and
	// `site_code` are the same column.
	// Only SOURCE NAMES resolve a raw column. The field ID (`cdr.msisdn`) is
	// the layer's internal handle and never appears in a payload -- an earlier
	// version of this audit put IDs in here too, which made every curated field
	// report as "absent from the data" including ones that demonstrably answer
	// questions today. An instrument that indicts working fields is worse than
	// no instrument.
	known := map[string]map[string]string{}   // recordType -> normalizedColumn -> fieldID
	aliases := map[string]map[string][]string{} // recordType -> fieldID -> source names
	for _, entity := range layer.Entities {
		recordType := entity.RecordType
		if recordType == "" {
			continue
		}
		if known[recordType] == nil {
			known[recordType] = map[string]string{}
			aliases[recordType] = map[string][]string{}
		}
		for _, field := range entity.Fields {
			aliases[recordType][field.ID] = append([]string(nil), field.SourceNames...)
			for _, name := range field.SourceNames {
				if trimmed := normalizeSourceNativeName(name); trimmed != "" {
					known[recordType][trimmed] = field.ID
				}
			}
		}
	}

	// Scope to ONE collection. Different collections were ingested from
	// different source schemas -- the demo CDRs use `MSISDN`/`CALL_TYPE` while
	// another set uses `a_number`/`call_time` -- so counting across all of them
	// reports columns as "not askable" that simply belong to other datasets.
	collection := os.Getenv("FAMILY_AUDIT_COLLECTION")
	if collection == "" {
		collection = "nexusai-forensic-demo"
	}
	t.Logf("collection: %s", collection)

	rows, err := pool.Query(ctx, `
		SELECT record_type, k, count(*)
		FROM (SELECT record_type, jsonb_object_keys(raw_payload) AS k
		      FROM forensic.records WHERE collection_id = $1) s
		GROUP BY record_type, k ORDER BY record_type, 3 DESC`, collection)
	if err != nil {
		t.Fatalf("payload keys: %v", err)
	}
	defer rows.Close()

	type column struct {
		name  string
		count int64
	}
	present := map[string][]column{}
	for rows.Next() {
		var recordType, key string
		var count int64
		if err := rows.Scan(&recordType, &key, &count); err != nil {
			t.Fatalf("scan: %v", err)
		}
		present[recordType] = append(present[recordType], column{key, count})
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}

	recordTypes := make([]string, 0, len(present))
	for recordType := range present {
		recordTypes = append(recordTypes, recordType)
	}
	sort.Strings(recordTypes)

	totalPresent, totalCovered := 0, 0
	t.Log("=== ANSWERABLE SURFACE: curated coverage of the columns actually present ===")
	for _, recordType := range recordTypes {
		columns := present[recordType]
		curated := known[recordType]
		covered, uncovered := 0, []string{}
		for _, col := range columns {
			if _, ok := curated[normalizeSourceNativeName(col.name)]; ok {
				covered++
				continue
			}
			uncovered = append(uncovered, fmt.Sprintf("%s(%d)", col.name, col.count))
		}
		totalPresent += len(columns)
		totalCovered += covered

		share := 0.0
		if len(columns) > 0 {
			share = 100 * float64(covered) / float64(len(columns))
		}
		flag := ""
		if share < 60 {
			flag = "   <-- most of this family cannot be asked about"
		}
		t.Logf("  %-16s %2d/%2d columns curated (%.0f%%)%s", recordType, covered, len(columns), share, flag)
		if len(uncovered) > 0 {
			t.Logf("      NOT ASKABLE: %s", strings.Join(uncovered, " "))
		}

		// A curated field describing a column that is not in the data is a
		// declaration that can never bind -- the mirror image of the gap, and
		// just as invisible to the golden suite.
		inData := map[string]bool{}
		for _, col := range columns {
			inData[normalizeSourceNativeName(col.name)] = true
		}
		orphans := []string{}
		for fieldID, sourceNames := range aliases[recordType] {
			bound := false
			for _, name := range sourceNames {
				if inData[normalizeSourceNativeName(name)] {
					bound = true
					break
				}
			}
			if !bound {
				orphans = append(orphans, fieldID)
			}
		}
		if len(orphans) > 0 {
			sort.Strings(orphans)
			// KNOWN FALSE-POSITIVE CLASS, stated so this line is not read as a
			// defect list: some curated fields bind to CANONICAL TABLE COLUMNS
			// (`timestamp`, `source_file`, `primary_target`) rather than to a
			// payload key. `cdr.event_time` is one -- it is the normalised
			// record timestamp and it demonstrably answers time questions
			// today. This audit reads `raw_payload` only, so those appear here
			// with no column behind them.
			t.Logf("      NO PAYLOAD COLUMN (may bind a canonical table column): %s",
				strings.Join(orphans, " "))
		}
	}

	share := 0.0
	if totalPresent > 0 {
		share = 100 * float64(totalCovered) / float64(totalPresent)
	}
	t.Logf("=== TOTAL: %d/%d columns curated (%.0f%%) ===", totalCovered, totalPresent, share)
	t.Log("An uncovered column is never issued in the enum, so no verified plan can " +
		"reference it and no question about it can be answered. This is the ceiling on " +
		"open-ended questions, and the golden suite cannot see it.")
}
