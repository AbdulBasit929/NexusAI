package main

import "fmt"

// Projection is restricted to already-authorized canonical columns. Source-native
// payloads need their own observed-schema and field-policy contract first.
var canonicalProjectionTypes = map[string]string{
	"record_type": "string", "timestamp": "timestamp", "primary_target": "identifier",
	"secondary_target": "identifier", "source_file": "string", "row_number": "integer",
	"ingested_at": "timestamp",
}

var canonicalProjectionProvenance = []string{
	"record_id", "tenant_id", "collection_id", "file_id", "batch_id", "record_type",
	"source_file", "row_number", "row_hash", "timestamp", "source_table",
}

func validateCanonicalProjection(fields []string) error {
	if len(fields) > len(canonicalProjectionTypes) {
		return fmt.Errorf("projection exceeds canonical field budget")
	}
	seen := map[string]bool{}
	for _, field := range fields {
		if _, ok := canonicalProjectionTypes[field]; !ok {
			return fmt.Errorf("projection field %q is not an authorized typed canonical field", field)
		}
		if seen[field] {
			return fmt.Errorf("projection repeats field %q", field)
		}
		seen[field] = true
	}
	return nil
}

func projectCanonicalRows(rows []map[string]any, fields []string) []map[string]any {
	if len(fields) == 0 {
		return rows
	}
	projected := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out := map[string]any{}
		for _, field := range canonicalProjectionProvenance {
			out[field] = row[field]
		}
		for _, field := range fields {
			out[field] = row[field]
		}
		projected = append(projected, out)
	}
	return projected
}
