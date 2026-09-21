package main

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const maxInlineEvidenceBytes int64 = 256 << 20

type evidenceContentDescriptor struct {
	StorageURI  string
	SourceFile  string
	ContentType string
	Modality    string
	SHA256      string
	SizeBytes   int64
}

type evidenceContentResolveFunc func(context.Context, forensicScope, string) (evidenceContentDescriptor, error)

// evidenceContentHandler serves only verified retained source bytes that have
// a deliberately bounded browser presentation policy.
// Scope is resolved before the storage URI is verified, so a caller cannot use
// this route to probe another tenant, collection, or case.
func evidenceContentHandler(db *pgxpool.Pool, store contentAddressedStore) http.HandlerFunc {
	return evidenceContentHandlerWithResolver(func(ctx context.Context, scope forensicScope, evidenceID string) (evidenceContentDescriptor, error) {
		if db == nil {
			return evidenceContentDescriptor{}, errors.New("forensic database is unavailable")
		}
		return resolveEvidenceContent(ctx, db, scope, evidenceID)
	}, store)
}

func evidenceContentHandlerWithResolver(resolve evidenceContentResolveFunc, store contentAddressedStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if resolve == nil {
			writeError(w, http.StatusServiceUnavailable, errors.New("forensic database is unavailable"))
			return
		}
		evidenceID := strings.TrimSpace(r.PathValue("evidence_id"))
		if evidenceID == "" {
			writeError(w, http.StatusBadRequest, errors.New("evidence_id is required"))
			return
		}
		scope, err := bindForensicScope(
			r,
			strings.TrimSpace(r.URL.Query().Get("tenant_id")),
			strings.TrimSpace(r.URL.Query().Get("collection_id")),
			strings.TrimSpace(r.URL.Query().Get("case_id")),
			"",
		)
		if err != nil {
			writeScopeError(w, err)
			return
		}
		descriptor, err := resolve(r.Context(), scope, evidenceID)
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, errors.New("evidence was not found in the authorized scope"))
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Errorf("resolve evidence content: %w", err))
			return
		}
		contentType, disposition, err := evidenceBrowserContentPolicy(descriptor.Modality, descriptor.SourceFile, descriptor.ContentType)
		if err != nil {
			writeError(w, http.StatusUnsupportedMediaType, err)
			return
		}
		if descriptor.SizeBytes < 0 || descriptor.SizeBytes > maxInlineEvidenceBytes {
			writeError(w, http.StatusRequestEntityTooLarge, errors.New("evidence exceeds the bounded inline-content limit"))
			return
		}
		verified, err := store.Verify(descriptor.StorageURI, descriptor.SizeBytes)
		if err != nil {
			writeError(w, http.StatusConflict, fmt.Errorf("verify retained evidence before serving: %w", err))
			return
		}
		if verified.SHA256 != descriptor.SHA256 {
			writeError(w, http.StatusConflict, errors.New("retained evidence identity does not match the evidence record"))
			return
		}
		file, err := os.Open(verified.Path)
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Errorf("open verified evidence: %w", err))
			return
		}
		defer file.Close()
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": filepath.Base(descriptor.SourceFile)}))
		w.Header().Set("Cache-Control", "private, no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Evidence-SHA256", descriptor.SHA256)
		http.ServeContent(w, r, filepath.Base(descriptor.SourceFile), time.Time{}, file)
	}
}

func resolveEvidenceContent(ctx context.Context, db *pgxpool.Pool, scope forensicScope, evidenceID string) (evidenceContentDescriptor, error) {
	tx, err := db.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return evidenceContentDescriptor{}, fmt.Errorf("begin evidence content query: %w", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", scope.TenantID); err != nil {
		return evidenceContentDescriptor{}, fmt.Errorf("set tenant context: %w", err)
	}
	descriptor := evidenceContentDescriptor{}
	err = tx.QueryRow(ctx, `
SELECT raw_storage_ref, source_file, coalesce(content_type, ''), modality, sha256, size_bytes
FROM forensic.evidence_items
WHERE tenant_id=$1 AND collection_id=$2 AND evidence_id=$3::uuid
  AND ($4 = '' OR case_id::text=$4)
`, scope.TenantID, scope.CollectionID, evidenceID, scope.CaseID).Scan(
		&descriptor.StorageURI, &descriptor.SourceFile, &descriptor.ContentType,
		&descriptor.Modality, &descriptor.SHA256, &descriptor.SizeBytes,
	)
	if err != nil {
		return evidenceContentDescriptor{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return evidenceContentDescriptor{}, fmt.Errorf("commit evidence content query: %w", err)
	}
	return descriptor, nil
}

func evidenceBrowserContentPolicy(modality, sourceFile, contentType string) (string, string, error) {
	modality = strings.ToLower(strings.TrimSpace(modality))
	extension := strings.ToLower(filepath.Ext(strings.TrimSpace(sourceFile)))
	contentType = strings.TrimSpace(contentType)
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = mime.TypeByExtension(extension)
	}
	// Native text can retain a legacy structured modality from its original
	// admission while the authoritative worker route correctly produces native
	// passages. The allowlist stays extension-bound so CSV/tabular sources do
	// not become browser-readable through this compatibility path.
	if extension == ".txt" || extension == ".md" || extension == ".log" {
		return "text/plain; charset=utf-8", "inline", nil
	}
	switch modality {
	case "image", "audio", "video":
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		return contentType, "inline", nil
	case "document", "text":
		switch extension {
		case ".pdf":
			return "application/pdf", "inline", nil
		case ".docx":
			return "application/vnd.openxmlformats-officedocument.wordprocessingml.document", "attachment", nil
		default:
			return "", "", errors.New("browser source access is unavailable for this document type")
		}
	default:
		return "", "", errors.New("inline content is limited to image, audio, video, TXT, PDF, and DOCX evidence")
	}
}
