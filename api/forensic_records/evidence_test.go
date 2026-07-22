package main

import (
	"net/http"
	"net/http/httptest"
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
