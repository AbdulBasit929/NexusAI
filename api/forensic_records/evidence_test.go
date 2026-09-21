package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClampEvidenceLimit(t *testing.T) {
	tests := []struct {
		name string
		in   int
		want int
	}{
		{"default below one", 0, defaultEvidenceLimit},
		{"preserve normal", 75, 75},
		{"cap excessive", maxEvidenceLimit + 1, maxEvidenceLimit},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := clampEvidenceLimit(tt.in); got != tt.want {
				t.Fatalf("clampEvidenceLimit(%d) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

func TestEvidenceListHandlerRequiresDatabase(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/evidence?collection_id=case-1", nil)
	rec := httptest.NewRecorder()
	evidenceListHandler(nil).ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestEvidenceListHandlerRejectsUnsupportedMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/evidence", nil)
	rec := httptest.NewRecorder()
	evidenceListHandler(nil).ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestEvidenceCatalogIncludesLegacyRowsInTheBoundCollection(t *testing.T) {
	if !strings.Contains(evidenceWhereSQL, "coalesce(evidence.case_id, '') IN ('', $3)") {
		t.Fatal("evidence catalog must include pre-case-id rows while rejecting rows bound to another case")
	}
}

func TestEvidenceCatalogReturnsOneRowPerEvidenceItem(t *testing.T) {
	for _, fragment := range []string{
		"LEFT JOIN LATERAL (",
		"FROM forensic.records_ingest_jobs job",
		"FROM forensic.kb_collection_assets asset",
		"ORDER BY job.queued_at DESC, job.id DESC",
		"ORDER BY asset.updated_at DESC, asset.id DESC",
	} {
		if !strings.Contains(evidenceListSQL, fragment) {
			t.Fatalf("evidence catalog query must select one current related row; missing %q", fragment)
		}
	}
	if strings.Contains(evidenceListSQL, "LEFT JOIN forensic.records_ingest_jobs jobs") ||
		strings.Contains(evidenceListSQL, "LEFT JOIN forensic.kb_collection_assets assets") {
		t.Fatal("evidence catalog must not multiply evidence rows with one-to-many joins")
	}
}
