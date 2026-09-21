package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestEvidenceContentHandlerServesVerifiedRangeWithinScope(t *testing.T) {
	store, err := newContentAddressedStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	payload := "0123456789abcdef"
	stored, err := store.Retain(strings.NewReader(payload), "default", "case-demo")
	if err != nil {
		t.Fatal(err)
	}
	resolve := func(_ context.Context, scope forensicScope, evidenceID string) (evidenceContentDescriptor, error) {
		if scope.TenantID != "default" || scope.CollectionID != "case-demo" || scope.CaseID != "case-demo" {
			t.Fatalf("unexpected scope: %#v", scope)
		}
		if evidenceID != "11111111-1111-1111-1111-111111111111" {
			t.Fatalf("unexpected evidence id: %s", evidenceID)
		}
		return evidenceContentDescriptor{
			StorageURI: stored.StorageURI, SourceFile: "clip.mp4", ContentType: "video/mp4",
			Modality: "video", SHA256: stored.SHA256, SizeBytes: int64(len(payload)),
		}, nil
	}
	handler := evidenceContentHandlerWithResolver(resolve, store)
	req := httptest.NewRequest(http.MethodGet, "/evidence/id/content?tenant_id=default&collection_id=case-demo&case_id=case-demo", nil)
	req.SetPathValue("evidence_id", "11111111-1111-1111-1111-111111111111")
	req.Header.Set("Range", "bytes=2-5")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusPartialContent {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Accept-Ranges"); got != "bytes" {
		t.Fatalf("Accept-Ranges=%q", got)
	}
	if got := recorder.Header().Get("Content-Range"); got != "bytes 2-5/16" {
		t.Fatalf("Content-Range=%q", got)
	}
	if got := recorder.Header().Get("Content-Type"); got != "video/mp4" {
		t.Fatalf("Content-Type=%q", got)
	}
	if got := recorder.Body.String(); got != "2345" {
		t.Fatalf("body=%q", got)
	}
}

func TestEvidenceBrowserContentPolicy(t *testing.T) {
	tests := []struct {
		name, modality, file, declared, wantType, wantDisposition string
		wantError                                                 bool
	}{
		{name: "image", modality: "image", file: "photo.jpg", declared: "image/jpeg", wantType: "image/jpeg", wantDisposition: "inline"},
		{name: "text", modality: "document", file: "notes.txt", declared: "text/html", wantType: "text/plain; charset=utf-8", wantDisposition: "inline"},
		{name: "native text with legacy structured modality", modality: "structured_records", file: "notes.txt", declared: "text/plain", wantType: "text/plain; charset=utf-8", wantDisposition: "inline"},
		{name: "pdf", modality: "document", file: "report.pdf", declared: "application/octet-stream", wantType: "application/pdf", wantDisposition: "inline"},
		{name: "docx", modality: "document", file: "report.docx", declared: "application/octet-stream", wantType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document", wantDisposition: "attachment"},
		{name: "unsupported document", modality: "document", file: "page.html", declared: "text/html", wantError: true},
		{name: "structured", modality: "structured", file: "records.csv", declared: "text/csv", wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotType, gotDisposition, err := evidenceBrowserContentPolicy(test.modality, test.file, test.declared)
			if test.wantError {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if gotType != test.wantType || gotDisposition != test.wantDisposition {
				t.Fatalf("got (%q,%q), want (%q,%q)", gotType, gotDisposition, test.wantType, test.wantDisposition)
			}
		})
	}
}

func TestEvidenceContentHandlerHidesMissingScopedEvidence(t *testing.T) {
	store, err := newContentAddressedStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	handler := evidenceContentHandlerWithResolver(func(context.Context, forensicScope, string) (evidenceContentDescriptor, error) {
		return evidenceContentDescriptor{}, pgx.ErrNoRows
	}, store)
	req := httptest.NewRequest(http.MethodGet, "/evidence/id/content?collection_id=case-demo&case_id=case-demo", nil)
	req.SetPathValue("evidence_id", "00000000-0000-0000-0000-000000000000")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
