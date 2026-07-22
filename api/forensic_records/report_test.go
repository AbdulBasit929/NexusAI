package main

import (
	"strings"
	"testing"
	"time"
)

func TestBuildForensicMarkdown(t *testing.T) {
	report := reportGenerateResponse{
		CollectionID: "records-demo",
		Target:       "ABC-123",
		GeneratedAt:  time.Date(2026, 7, 14, 6, 0, 0, 0, time.UTC),
		Sections: map[string]any{
			"overview": map[string]any{
				"jobs": []map[string]any{{"source_file": "sample.csv", "record_type": "cdr", "status": "completed", "total_rows": int64(3), "headers": []string{"a", "b"}}},
			},
			"entity_activity": map[string]any{
				"entity_activity": []map[string]any{{"entity_value": "ABC-123", "observation_count": int64(2)}},
			},
			"case_readiness": map[string]any{
				"readiness": map[string]any{"level": "usable_with_limitations", "score": 84, "accepted_rows": int64(3), "source_files": 1, "record_families": 1, "kb_assets": 1},
				"readiness_checks": []map[string]any{
					{"check": "overall", "status": "usable_with_limitations", "score": 84},
					{"check": "rejected_rows", "status": "ok", "count": 0},
				},
			},
		},
	}
	markdown := buildForensicMarkdown(report)
	for _, want := range []string{
		"## Forensic Intelligence Report",
		"Collection ID: records-demo",
		"Target: ABC-123",
		"Accuracy Guardrail",
		"Key Findings",
		"Case readiness is usable_with_limitations with score 84.",
		"Confidence",
		"Case Readiness",
		"Case Readiness - Checks",
		"Collection Overview - Jobs",
		"| source_file | record_type | status | total_rows |",
		"Entity Activity",
		"Limitations",
		"Recommended Next Steps",
	} {
		if !strings.Contains(markdown, want) {
			t.Fatalf("markdown missing %q:\n%s", want, markdown)
		}
	}
	if strings.Contains(markdown, "headers=") || strings.Contains(markdown, "[\"a\",\"b\"]") {
		t.Fatalf("markdown leaked verbose schema fields:\n%s", markdown)
	}
}
