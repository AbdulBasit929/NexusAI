package localai

import (
	"testing"
	"time"
)

func TestForensicRecordsProxyTimeout(t *testing.T) {
	tests := []struct {
		path string
		want time.Duration
	}{
		{path: "/query/hybrid", want: 405 * time.Second},
		{path: "/api/v1/cases/case-alpha/query", want: 405 * time.Second},
		{path: "/cases/case-alpha/query/", want: 405 * time.Second},
		{path: "/health", want: 30 * time.Second},
		{path: "/templates", want: 30 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := forensicRecordsProxyTimeout(tt.path); got != tt.want {
				t.Fatalf("forensicRecordsProxyTimeout(%q) = %s, want %s", tt.path, got, tt.want)
			}
		})
	}
}
