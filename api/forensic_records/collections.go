package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	defaultRepairLimit = 100
	maxRepairLimit     = 1000
)

type collectionScope struct {
	TenantID     string `json:"tenant_id"`
	CollectionID string `json:"collection_id"`
}

type collectionRepairRequest struct {
	collectionScope
	DryRun bool `json:"dry_run"`
	Limit  int  `json:"limit"`
}

func collectionStatusHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
			return
		}
		if db == nil {
			writeError(w, http.StatusServiceUnavailable, errors.New("FORENSIC_DATABASE_URL is not configured"))
			return
		}
		requestedScope := collectionScope{
			TenantID:     strings.TrimSpace(r.URL.Query().Get("tenant_id")),
			CollectionID: strings.TrimSpace(r.URL.Query().Get("collection_id")),
		}
		boundScope, err := bindForensicScope(r, requestedScope.TenantID, requestedScope.CollectionID, "", "")
		if err != nil {
			writeScopeError(w, err)
			return
		}
		scope := collectionScope{TenantID: boundScope.TenantID, CollectionID: boundScope.CollectionID}
		if scope.CollectionID == "" {
			writeError(w, http.StatusBadRequest, errors.New("collection_id is required"))
			return
		}
		status, err := loadCollectionStatus(r.Context(), db, scope, clampRepairLimit(queryInt(r, "limit", 20)))
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, status)
	}
}

func collectionRepairHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
			return
		}
		if db == nil {
			writeError(w, http.StatusServiceUnavailable, errors.New("FORENSIC_DATABASE_URL is not configured"))
			return
		}
		if err := requireForensicAdmin(r); err != nil {
			writeScopeError(w, err)
			return
		}
		req := collectionRepairRequest{
			collectionScope: collectionScope{
				TenantID:     strings.TrimSpace(r.URL.Query().Get("tenant_id")),
				CollectionID: strings.TrimSpace(r.URL.Query().Get("collection_id")),
			},
			DryRun: envBoolValue(r.URL.Query().Get("dry_run"), false),
			Limit:  queryInt(r, "limit", defaultRepairLimit),
		}
		if r.Body != nil {
			defer r.Body.Close()
			var body collectionRepairRequest
			if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
				writeError(w, http.StatusBadRequest, fmt.Errorf("decode repair request: %w", err))
				return
			}
			if strings.TrimSpace(body.TenantID) != "" {
				req.TenantID = strings.TrimSpace(body.TenantID)
			}
			if strings.TrimSpace(body.CollectionID) != "" {
				req.CollectionID = strings.TrimSpace(body.CollectionID)
			}
			req.DryRun = body.DryRun
			if body.Limit != 0 {
				req.Limit = body.Limit
			}
		}
		boundScope, err := bindForensicScope(r, req.TenantID, req.CollectionID, "", "")
		if err != nil {
			writeScopeError(w, err)
			return
		}
		req.TenantID = boundScope.TenantID
		req.CollectionID = boundScope.CollectionID
		req.Limit = clampRepairLimit(req.Limit)
		if req.CollectionID == "" {
			writeError(w, http.StatusBadRequest, errors.New("collection_id is required"))
			return
		}
		resp, err := repairCollectionAssets(r.Context(), db, req)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

func loadCollectionStatus(ctx context.Context, db *pgxpool.Pool, scope collectionScope, limit int) (map[string]any, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	tx, err := db.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, fmt.Errorf("begin status query: %w", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", scope.TenantID); err != nil {
		return nil, fmt.Errorf("set tenant context: %w", err)
	}

	summary, err := rowsFromQuery(ctx, tx, `
WITH jobs AS (
  SELECT *
  FROM forensic.records_ingest_jobs
  WHERE tenant_id = $1 AND collection_id = $2
),
assets AS (
  SELECT *
  FROM forensic.kb_collection_assets
  WHERE tenant_id = $1 AND collection_id = $2
),
evidence AS (
  SELECT *
  FROM forensic.evidence_items
  WHERE tenant_id = $1 AND collection_id = $2
),
missing_assets AS (
  SELECT jobs.id
  FROM jobs
  LEFT JOIN assets
    ON assets.tenant_id = jobs.tenant_id
   AND assets.collection_id = jobs.collection_id
   AND assets.file_id = jobs.file_id
   AND assets.sha256 = jobs.sha256
  WHERE jobs.status = 'completed'
    AND assets.id IS NULL
)
SELECT
  (SELECT count(*) FROM jobs) AS jobs_total,
  (SELECT count(*) FROM jobs WHERE status = 'completed') AS completed_jobs,
  (SELECT count(*) FROM jobs WHERE status IN ('failed', 'dead_letter')) AS failed_jobs,
  (SELECT count(*) FROM assets) AS kb_assets_total,
  (SELECT count(*) FROM evidence) AS evidence_total,
  (SELECT count(*) FROM evidence WHERE processing_status = 'completed') AS evidence_completed,
  (SELECT count(*) FROM evidence WHERE processing_status = 'failed') AS evidence_failed,
  (SELECT count(*) FROM evidence WHERE processing_status IN ('queued', 'registered', 'processing')) AS evidence_in_flight,
  (SELECT count(*) FROM missing_assets) AS completed_jobs_missing_kb_asset,
  (SELECT coalesce(sum(total_rows), 0) FROM jobs) AS total_rows,
  (SELECT coalesce(sum(accepted_rows), 0) FROM jobs) AS accepted_rows,
  (SELECT coalesce(sum(duplicate_rows), 0) FROM jobs) AS duplicate_rows,
  (SELECT coalesce(sum(rejected_rows), 0) FROM jobs) AS rejected_rows`, scope.TenantID, scope.CollectionID)
	if err != nil {
		return nil, err
	}
	missing, err := rowsFromQuery(ctx, tx, `
SELECT jobs.id::text AS job_id, jobs.source_file, jobs.record_type::text AS record_type,
       jobs.total_rows, jobs.accepted_rows, jobs.duplicate_rows, jobs.rejected_rows,
       jobs.completed_at, jobs.metadata
FROM forensic.records_ingest_jobs jobs
LEFT JOIN forensic.kb_collection_assets assets
  ON assets.tenant_id = jobs.tenant_id
 AND assets.collection_id = jobs.collection_id
 AND assets.file_id = jobs.file_id
 AND assets.sha256 = jobs.sha256
WHERE jobs.tenant_id = $1
  AND jobs.collection_id = $2
  AND jobs.status = 'completed'
  AND assets.id IS NULL
ORDER BY jobs.completed_at DESC NULLS LAST, jobs.queued_at DESC
LIMIT $3`, scope.TenantID, scope.CollectionID, limit)
	if err != nil {
		return nil, err
	}
	families, err := rowsFromQuery(ctx, tx, `
SELECT record_type::text AS record_type,
       count(*) AS jobs,
       coalesce(sum(total_rows), 0) AS total_rows,
       coalesce(sum(accepted_rows), 0) AS accepted_rows,
       coalesce(sum(duplicate_rows), 0) AS duplicate_rows,
       coalesce(sum(rejected_rows), 0) AS rejected_rows
FROM forensic.records_ingest_jobs
WHERE tenant_id = $1 AND collection_id = $2
GROUP BY record_type
ORDER BY total_rows DESC`, scope.TenantID, scope.CollectionID)
	if err != nil {
		return nil, err
	}
	evidence, err := rowsFromQuery(ctx, tx, `
SELECT evidence_id::text, case_id, source_file, modality, detected_type,
       processing_status, processing_route, kb_entry_ref, records_batch_id::text,
       size_bytes, warnings, errors, created_at, updated_at
FROM forensic.evidence_items
WHERE tenant_id = $1 AND collection_id = $2
ORDER BY created_at DESC
LIMIT $3`, scope.TenantID, scope.CollectionID, limit)
	if err != nil {
		return nil, err
	}
	recentJobs, err := rowsFromQuery(ctx, tx, `
SELECT id::text AS job_id, evidence_id::text, source_file,
       record_type::text AS record_type, status::text AS status,
       attempt_count, max_attempts, total_rows, accepted_rows, duplicate_rows,
       rejected_rows, error_message, queue_published_at, queue_publish_attempts,
       queue_publish_next_at, queue_publish_error, worker_lease_owner,
       worker_lease_expires_at, next_attempt_at, last_error_class,
       dead_lettered_at, acknowledged_at, queued_at, started_at, completed_at
FROM forensic.records_ingest_jobs
WHERE tenant_id = $1 AND collection_id = $2
ORDER BY coalesce(completed_at, started_at, queued_at) DESC NULLS LAST, id DESC
LIMIT $3`, scope.TenantID, scope.CollectionID, limit)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"tenant_id":         scope.TenantID,
		"collection_id":     scope.CollectionID,
		"summary":           firstRow(summary),
		"missing_kb_assets": missing,
		"record_families":   families,
		"recent_jobs":       recentJobs,
		"recent_evidence":   evidence,
		"generated_at":      time.Now().UTC(),
	}, nil
}

func repairCollectionAssets(ctx context.Context, db *pgxpool.Pool, req collectionRepairRequest) (map[string]any, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	tx, err := db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin repair transaction: %w", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", req.TenantID); err != nil {
		return nil, fmt.Errorf("set tenant context: %w", err)
	}
	candidates, err := rowsFromQuery(ctx, tx, repairCandidatesSQL, req.TenantID, req.CollectionID, req.Limit)
	if err != nil {
		return nil, err
	}
	if req.DryRun {
		return map[string]any{
			"tenant_id":     req.TenantID,
			"collection_id": req.CollectionID,
			"dry_run":       true,
			"repairable":    len(candidates),
			"candidates":    candidates,
			"generated_at":  time.Now().UTC(),
		}, nil
	}
	tag, err := tx.Exec(ctx, repairAssetsSQL, req.TenantID, req.CollectionID, req.Limit)
	if err != nil {
		return nil, fmt.Errorf("repair kb asset links: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit repair transaction: %w", err)
	}
	return map[string]any{
		"tenant_id":       req.TenantID,
		"collection_id":   req.CollectionID,
		"dry_run":         false,
		"repairable":      len(candidates),
		"repaired_assets": tag.RowsAffected(),
		"candidates":      candidates,
		"generated_at":    time.Now().UTC(),
	}, nil
}

const repairCandidatesSQL = `
SELECT jobs.id::text AS job_id, jobs.source_file, jobs.file_id, jobs.sha256,
       jobs.record_type::text AS record_type, jobs.status::text AS status,
       jobs.total_rows, jobs.accepted_rows, jobs.duplicate_rows, jobs.rejected_rows,
       jobs.metadata->>'kb_source_entry' AS source_entry,
       coalesce(jobs.metadata->>'kb_mirror_status', 'disabled') AS kb_mirror_status
FROM forensic.records_ingest_jobs jobs
LEFT JOIN forensic.kb_collection_assets assets
  ON assets.tenant_id = jobs.tenant_id
 AND assets.collection_id = jobs.collection_id
 AND assets.file_id = jobs.file_id
 AND assets.sha256 = jobs.sha256
WHERE jobs.tenant_id = $1
  AND jobs.collection_id = $2
  AND jobs.status = 'completed'
  AND assets.id IS NULL
ORDER BY jobs.completed_at DESC NULLS LAST, jobs.queued_at DESC
LIMIT $3`

const repairAssetsSQL = `
WITH candidates AS (
  SELECT jobs.*,
         metadata.normalized_schema,
         metadata.quality_report AS metadata_quality_report
  FROM forensic.records_ingest_jobs jobs
  LEFT JOIN forensic.kb_collection_assets assets
    ON assets.tenant_id = jobs.tenant_id
   AND assets.collection_id = jobs.collection_id
   AND assets.file_id = jobs.file_id
   AND assets.sha256 = jobs.sha256
  LEFT JOIN forensic.kb_active_metadata metadata
    ON metadata.tenant_id = jobs.tenant_id
   AND metadata.collection_id = jobs.collection_id
   AND metadata.file_id = jobs.file_id
   AND metadata.batch_id = jobs.id
  WHERE jobs.tenant_id = $1
    AND jobs.collection_id = $2
    AND jobs.status = 'completed'
    AND assets.id IS NULL
  ORDER BY jobs.completed_at DESC NULLS LAST, jobs.queued_at DESC
  LIMIT $3
)
INSERT INTO forensic.kb_collection_assets (
  tenant_id, user_id, collection_id, file_id, batch_id, source_file,
  source_entry, sha256, detected_record_type, requested_record_type, storage_mode,
  rag_status, structured_status, content_type, size_bytes, evidence_id, headers,
  routing_decision, quality_report
)
SELECT
  tenant_id,
  user_id,
  collection_id,
  file_id,
  id,
  source_file,
  nullif(metadata->>'kb_source_entry', ''),
  sha256,
  record_type,
  record_type,
  CASE WHEN nullif(metadata->>'kb_source_entry', '') IS NULL THEN 'records_only' ELSE 'hybrid' END,
  CASE
    WHEN metadata->>'kb_mirror_status' = 'mirrored' THEN 'mirrored'
    WHEN metadata->>'kb_mirror_status' = 'failed' THEN 'failed'
    ELSE 'skipped'
  END,
  'completed',
  nullif(metadata->>'content_type', ''),
  CASE
    WHEN metadata->>'size_bytes' ~ '^[0-9]+$' THEN (metadata->>'size_bytes')::bigint
    ELSE NULL
  END,
  evidence_id,
  '[]'::jsonb,
  jsonb_build_object(
    'storage_mode', CASE WHEN nullif(metadata->>'kb_source_entry', '') IS NULL THEN 'records_only' ELSE 'hybrid' END,
    'requested_record_type', record_type::text,
    'detected_record_type', record_type::text,
    'structured_store', CASE WHEN record_type = 'cdr' THEN 'cdr_records' ELSE 'generic_records' END,
    'kb_collection', collection_id,
    'routing_reason', 'asset_repair_from_completed_ingest_job'
  ),
  jsonb_build_object(
    'repair_source', 'records_ingest_jobs',
    'repaired_at', now(),
    'total_rows', total_rows,
    'accepted_rows', accepted_rows,
    'duplicate_rows', duplicate_rows,
    'rejected_rows', rejected_rows,
    'normalized_schema', coalesce(normalized_schema, '{}'::jsonb),
    'metadata_quality_report', coalesce(metadata_quality_report, '{}'::jsonb)
  )
FROM candidates
ON CONFLICT (tenant_id, collection_id, file_id, sha256)
DO UPDATE SET
  batch_id=EXCLUDED.batch_id,
  detected_record_type=EXCLUDED.detected_record_type,
  requested_record_type=EXCLUDED.requested_record_type,
  storage_mode=EXCLUDED.storage_mode,
  rag_status=EXCLUDED.rag_status,
  structured_status=EXCLUDED.structured_status,
  evidence_id=coalesce(forensic.kb_collection_assets.evidence_id, EXCLUDED.evidence_id),
  routing_decision=EXCLUDED.routing_decision,
  quality_report=forensic.kb_collection_assets.quality_report || EXCLUDED.quality_report,
  updated_at=now()`

func rowsFromQuery(ctx context.Context, q interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}, sql string, args ...any) ([]map[string]any, error) {
	rows, err := q.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("execute collection operation: %w", err)
	}
	defer rows.Close()
	values, err := rowsToMaps(rows)
	if err != nil {
		return nil, fmt.Errorf("read collection operation rows: %w", err)
	}
	return values, nil
}

func clampRepairLimit(limit int) int {
	if limit < 1 {
		return defaultRepairLimit
	}
	if limit > maxRepairLimit {
		return maxRepairLimit
	}
	return limit
}

func queryInt(r *http.Request, key string, fallback int) int {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}
