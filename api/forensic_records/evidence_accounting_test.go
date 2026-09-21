package main

import (
	"strings"
	"testing"
)

func TestEvidenceProvenanceQueriesRemainScopeBoundAndStable(t *testing.T) {
	queries := map[string]string{
		"processing runs":   evidenceProcessingRunsSQL,
		"processing events": evidenceProcessingEventsSQL,
		"derived artifacts": evidenceDerivedArtifactsSQL,
		"custody events":    evidenceCustodyEventsSQL,
		"custody integrity": evidenceCustodySummarySQL,
	}
	for name, query := range queries {
		for _, required := range []string{"tenant_id = $1", "collection_id = $2", "evidence_id = $3::uuid"} {
			if !strings.Contains(query, required) {
				t.Fatalf("%s query is missing scope predicate %q", name, required)
			}
		}
	}
	if !strings.Contains(evidenceDerivedArtifactsSQL, "nexusai://evidence/") || !strings.Contains(evidenceDerivedArtifactsSQL, "citation_ref") {
		t.Fatal("derived artifact query does not expose the stable evidence/artifact citation reference")
	}
	for _, fragment := range []string{
		"JOIN walk parent ON child.previous_event_sha256 = parent.event_sha256",
		"missing_predecessors = 0",
		"forked_links = 0",
		"reachable_count = event_count",
	} {
		if !strings.Contains(evidenceCustodySummarySQL, fragment) {
			t.Fatalf("custody summary does not verify chain topology; missing %q", fragment)
		}
	}
	if !strings.Contains(evidenceCustodyEventsSQL, "ORDER BY walk.chain_depth DESC NULLS LAST") {
		t.Fatal("custody event presentation is not ordered by the recorded hash chain")
	}
}

func TestEvidenceAccountingViewOmitsSourcePayloads(t *testing.T) {
	detail := map[string]any{
		"tenant_id": "tenant-a", "collection_id": "case-a",
		"item": map[string]any{
			"processing_status": "completed",
			"metadata":          map[string]any{"CNIC": "one", "cnic": "two"},
		},
		"records_preview": []map[string]any{{"CNIC": "one", "cnic": "two"}},
		"ingest_jobs": []map[string]any{{
			"job_id": "hidden", "status": "completed", "record_type": "subscriber",
			"total_rows": int64(9), "accepted_rows": int64(5), "duplicate_rows": int64(1), "rejected_rows": int64(3),
		}},
	}

	view := evidenceAccountingView(detail)
	if view["view"] != "accounting" || view["processing_status"] != "completed" {
		t.Fatalf("accounting view = %#v", view)
	}
	if _, exists := view["records_preview"]; exists {
		t.Fatal("accounting view exposed records_preview")
	}
	jobs := view["ingest_jobs"].([]map[string]any)
	if len(jobs) != 1 || jobs[0]["accepted_rows"] != int64(5) {
		t.Fatalf("accounting jobs = %#v", jobs)
	}
	if _, exists := jobs[0]["job_id"]; exists {
		t.Fatal("accounting view exposed job identity")
	}
}

func TestEvidencePreviewRedactsSubscriberIdentityFields(t *testing.T) {
	rows := []map[string]any{
		{
			"record_type":      "subscriber",
			"secondary_target": "00000-1000001-1",
			"raw_payload": map[string]any{
				"full_name": "Synthetic Person",
				"cnic_no":   "00000-1000001-1",
				"mobile_no": "03000000001",
			},
			"metadata": map[string]any{
				"normalized_fields": map[string]any{
					"subscriber_name":            "Synthetic Person",
					"subscriber_name_search_key": "synthetic person",
					"subscriber_name_script":     "latin",
					"subscriber_reference":       "PK-SUB-SYN-ALPHA",
					"cnic_raw":                   "00000-1000001-1",
					"cnic_digits":                "0000010000011",
					"cnic_masked":                "*********0011",
				},
			},
		},
	}

	redactSubscriberCanonicalRows(rows)

	metadata := rows[0]["metadata"].(map[string]any)
	fields := metadata["normalized_fields"].(map[string]any)
	for _, key := range []string{"subscriber_name", "subscriber_name_search_key", "cnic_raw", "cnic_digits"} {
		if _, exists := fields[key]; exists {
			t.Fatalf("subscriber evidence preview retained protected field %q", key)
		}
	}
	if fields["subscriber_name_present"] != true {
		t.Fatal("subscriber evidence preview did not retain the safe name-presence indicator")
	}
	payload := rows[0]["raw_payload"].(map[string]any)
	if _, exists := payload["full_name"]; exists {
		t.Fatal("subscriber evidence preview retained a raw name")
	}
	if payload["cnic_no"] != "*********0011" {
		t.Fatalf("subscriber evidence preview did not mask CNIC: %#v", payload["cnic_no"])
	}
	if rows[0]["secondary_target"] != "PK-SUB-SYN-ALPHA" {
		t.Fatalf("subscriber evidence preview retained an unsafe secondary target: %#v", rows[0]["secondary_target"])
	}
}
