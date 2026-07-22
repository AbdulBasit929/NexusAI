package main

import (
	"net/http/httptest"
	"testing"
)

func TestClampRepairLimit(t *testing.T) {
	tests := []struct {
		name string
		in   int
		want int
	}{
		{"default below one", 0, defaultRepairLimit},
		{"preserve normal", 50, 50},
		{"cap excessive", maxRepairLimit + 1, maxRepairLimit},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := clampRepairLimit(tt.in); got != tt.want {
				t.Fatalf("clampRepairLimit(%d) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

func TestQueryInt(t *testing.T) {
	req := httptest.NewRequest("GET", "/collections/status?limit=25", nil)
	if got := queryInt(req, "limit", 10); got != 25 {
		t.Fatalf("queryInt(limit) = %d, want 25", got)
	}
	if got := queryInt(req, "missing", 10); got != 10 {
		t.Fatalf("queryInt(missing) = %d, want 10", got)
	}
	req = httptest.NewRequest("GET", "/collections/status?limit=bad", nil)
	if got := queryInt(req, "limit", 10); got != 10 {
		t.Fatalf("queryInt(bad) = %d, want 10", got)
	}
}
