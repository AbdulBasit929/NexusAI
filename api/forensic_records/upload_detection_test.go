package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadJSONHeadersFromJSONL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "seed_ipdr.jsonl")
	err := os.WriteFile(path, []byte(`{"timestamp":"2026-07-14T00:00:00Z","source_ip":"10.0.0.1","destination_ip":"8.8.8.8","bytes":123}`+"\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	headers, err := readCSVHeaders(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := detectRecordType(headers); got != "ipdr" {
		t.Fatalf("detectRecordType(%v) = %q, want ipdr", headers, got)
	}
}

func TestDetectCanonicalCDRHeaders(t *testing.T) {
	headers := []string{"timestamp", "source_number", "target_number", "duration_seconds", "call_type", "cell_id", "latitude", "longitude"}
	if got := detectRecordType(headers); got != "cdr" {
		t.Fatalf("detectRecordType() = %q, want cdr", got)
	}
}

func TestClassifyEvidenceItemPrefersStructuredRecordSignals(t *testing.T) {
	headers := []string{"timestamp", "source_number", "target_number", "duration_seconds"}
	classification := classifyEvidenceItem("calls.csv", "text/csv", detectRecordType(headers), headers)
	if classification.Modality != "structured_records" {
		t.Fatalf("Modality = %q, want structured_records", classification.Modality)
	}
	if classification.DetectedType != "cdr" {
		t.Fatalf("DetectedType = %q, want cdr", classification.DetectedType)
	}
	if classification.ProcessingRoute != "forensic_records_worker" {
		t.Fatalf("ProcessingRoute = %q, want forensic_records_worker", classification.ProcessingRoute)
	}
}

func TestClassifyEvidenceItemRoutesDocumentsToKB(t *testing.T) {
	classification := classifyEvidenceItem("case-notes.pdf", "application/pdf", "", nil)
	if classification.Modality != "document" {
		t.Fatalf("Modality = %q, want document", classification.Modality)
	}
	if classification.DetectedType != "pdf" {
		t.Fatalf("DetectedType = %q, want pdf", classification.DetectedType)
	}
	if classification.ProcessingRoute != "kb_document_pipeline" {
		t.Fatalf("ProcessingRoute = %q, want kb_document_pipeline", classification.ProcessingRoute)
	}
}

func TestReadTextHeadersFallback(t *testing.T) {
	path := filepath.Join(t.TempDir(), "case_notes.txt")
	if err := os.WriteFile(path, []byte("Case notes for records-demo\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	headers, err := readCSVHeaders(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(headers) != 1 || headers[0] != "line" {
		t.Fatalf("headers = %#v, want [line]", headers)
	}
}
