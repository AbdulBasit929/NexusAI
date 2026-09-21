package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var reprocessIdempotencyKeyRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{7,127}$`)

var errEvidenceReprocessInProgress = errors.New("evidence already has a non-terminal processing job")

type evidenceReprocessRequest struct {
	TenantID       string `json:"tenant_id"`
	CollectionID   string `json:"collection_id"`
	CaseID         string `json:"case_id,omitempty"`
	Reason         string `json:"reason"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
	MaxAttempts    int    `json:"max_attempts,omitempty"`
}

type evidenceReprocessResult struct {
	Status              string    `json:"status"`
	EvidenceID          string    `json:"evidence_id"`
	JobID               string    `json:"job_id"`
	ReprocessOfJobID    string    `json:"reprocess_of_job_id"`
	ReprocessGeneration int       `json:"reprocess_generation"`
	IdempotencyKey      string    `json:"idempotency_key"`
	QueuedAt            time.Time `json:"queued_at"`
	PublicationPending  bool      `json:"publication_pending"`
	IdempotentReplay    bool      `json:"idempotent_replay"`
}

const evidenceReprocessPlanContractV1 = "forensics.evidence-reprocess-plan/v1"

// evidenceReprocessPlanHandler exposes the exact, read-only prerequisites for
// a future immutable reprocess request. It deliberately never reserves a job,
// changes evidence state, writes audit history, or publishes to the queue.
func evidenceReprocessPlanHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if db == nil {
			writeError(w, http.StatusServiceUnavailable, errors.New("FORENSIC_DATABASE_URL is not configured"))
			return
		}
		evidenceID := strings.TrimSpace(r.PathValue("evidence_id"))
		if _, err := uuid.Parse(evidenceID); err != nil {
			writeError(w, http.StatusBadRequest, errors.New("a valid evidence_id is required"))
			return
		}
		scope, err := bindForensicScope(r, r.URL.Query().Get("tenant_id"), r.URL.Query().Get("collection_id"), r.URL.Query().Get("case_id"), "")
		if err != nil {
			writeScopeError(w, err)
			return
		}
		if scope.CollectionID == "" {
			writeError(w, http.StatusBadRequest, errors.New("collection_id is required"))
			return
		}

		plan, err := loadEvidenceReprocessPlan(r.Context(), db, scope, evidenceID)
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, errors.New("evidence was not found in the authorized collection"))
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Errorf("load evidence reprocess plan: %w", err))
			return
		}
		writeJSON(w, http.StatusOK, plan)
	}
}

func loadEvidenceReprocessPlan(ctx context.Context, db *pgxpool.Pool, scope forensicScope, evidenceID string) (map[string]any, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	tx, err := db.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", scope.TenantID); err != nil {
		return nil, err
	}
	row := map[string]any{}
	rows, err := rowsFromQuery(ctx, tx, `
SELECT evidence.evidence_id::text AS evidence_id,
       evidence.processing_status::text AS evidence_status,
       jobs.id::text AS latest_job_id,
       jobs.status::text AS latest_job_status,
       jobs.reprocess_generation,
       jobs.attempt_count,
       jobs.max_attempts,
       jobs.completed_at,
       jobs.dead_lettered_at
FROM forensic.evidence_items evidence
LEFT JOIN LATERAL (
  SELECT id, status, reprocess_generation, attempt_count, max_attempts, completed_at, dead_lettered_at
  FROM forensic.records_ingest_jobs
  WHERE tenant_id = evidence.tenant_id
    AND collection_id = evidence.collection_id
    AND evidence_id = evidence.evidence_id
  ORDER BY reprocess_generation DESC, queued_at DESC
  LIMIT 1
) jobs ON true
WHERE evidence.tenant_id = $1
  AND evidence.collection_id = $2
  AND evidence.evidence_id = $3::uuid`, scope.TenantID, scope.CollectionID, evidenceID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, pgx.ErrNoRows
	}
	row = rows[0]
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	jobStatus := strings.ToLower(strings.TrimSpace(fmt.Sprint(row["latest_job_status"])))
	eligible := jobStatus == "completed" || jobStatus == "dead_letter"
	eligibilityReason := "A terminal processing job is required before an immutable reprocess can be considered."
	if eligible {
		eligibilityReason = "The latest job is terminal; an approved immutable reprocess would create a new linked generation."
	}
	if jobStatus == "" {
		eligibilityReason = "No ingest job is linked to this evidence item, so reprocess cannot be planned."
	}
	return map[string]any{
		"contract_version": evidenceReprocessPlanContractV1,
		"tenant_id":        scope.TenantID,
		"case_id":          scope.CaseID,
		"collection_id":    scope.CollectionID,
		"evidence_id":      evidenceID,
		"current_state":    row,
		"eligibility": map[string]any{
			"eligible": eligible,
			"reason":   eligibilityReason,
		},
		"approval": map[string]any{
			"required":            true,
			"state":               "not_granted",
			"execution_permitted": false,
			"notice":              "This plan is read-only. A separately authorized operator workflow is required before any reprocess request can be submitted.",
		},
		"request_contract": map[string]any{
			"method":          http.MethodPost,
			"endpoint":        "/api/records/forensic/evidence/{evidence_id}/reprocess",
			"idempotency_key": "required; 8-128 safe characters",
			"reason":          "required; 3-1000 characters",
			"max_attempts":    "optional; integer 1-10; default 5",
			"effects":         []string{"creates a new immutable linked job generation", "preserves prior jobs and derived outputs", "updates queue state only after approved execution"},
		},
		"generated_at": time.Now().UTC(),
	}, nil
}

// evidenceReprocessHandler creates a new immutable job linked to an earlier job.
// It never resets terminal state or deletes prior records/artifacts.
func evidenceReprocessHandler(db *pgxpool.Pool, publisher ingestPublisher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if db == nil {
			writeError(w, http.StatusServiceUnavailable, errors.New("FORENSIC_DATABASE_URL is not configured"))
			return
		}
		evidenceID := strings.TrimSpace(r.PathValue("evidence_id"))
		if _, err := uuid.Parse(evidenceID); err != nil {
			writeError(w, http.StatusBadRequest, errors.New("a valid evidence_id is required"))
			return
		}
		var req evidenceReprocessRequest
		decoder := json.NewDecoder(io.LimitReader(r.Body, 64<<10))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("decode reprocess request: %w", err))
			return
		}
		scope, err := bindForensicScope(r, req.TenantID, req.CollectionID, req.CaseID, "")
		if err != nil {
			writeScopeError(w, err)
			return
		}
		if scope.CollectionID == "" {
			writeError(w, http.StatusBadRequest, errors.New("collection_id is required"))
			return
		}
		reason := strings.TrimSpace(req.Reason)
		if len(reason) < 3 || len(reason) > 1000 {
			writeError(w, http.StatusBadRequest, errors.New("reason must contain 3 to 1000 characters"))
			return
		}
		key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		if key == "" {
			key = strings.TrimSpace(req.IdempotencyKey)
		}
		if !reprocessIdempotencyKeyRE.MatchString(key) {
			writeError(w, http.StatusBadRequest, errors.New("Idempotency-Key must contain 8 to 128 safe characters"))
			return
		}
		if req.MaxAttempts == 0 {
			req.MaxAttempts = 5
		}
		if req.MaxAttempts < 1 || req.MaxAttempts > 10 {
			writeError(w, http.StatusBadRequest, errors.New("max_attempts must be between 1 and 10"))
			return
		}

		result, job, err := createReprocessJob(r.Context(), db, scope, evidenceID, reason, key, req.MaxAttempts, requestIDFromHTTP(r))
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeError(w, http.StatusNotFound, errors.New("structured evidence or a source ingest job was not found in this collection"))
				return
			}
			if errors.Is(err, errEvidenceReprocessInProgress) {
				writeError(w, http.StatusConflict, err)
				return
			}
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		if result.IdempotentReplay {
			if result.PublicationPending {
				published, publishErr := publishQueuedJob(r.Context(), db, publisher, scope.TenantID, result.JobID)
				result.PublicationPending = publishErr != nil || !published
			}
			if result.PublicationPending {
				result.Status = "publication_pending"
			}
			writeJSON(w, http.StatusAccepted, result)
			return
		}
		published, publishErr := publishQueuedJob(r.Context(), db, publisher, scope.TenantID, job.JobID)
		result.PublicationPending = publishErr != nil || !published
		if result.PublicationPending {
			result.Status = "publication_pending"
		}
		writeJSON(w, http.StatusAccepted, result)
	}
}

func createReprocessJob(
	ctx context.Context,
	db *pgxpool.Pool,
	scope forensicScope,
	evidenceID, reason, idempotencyKey string,
	maxAttempts int,
	requestID string,
) (evidenceReprocessResult, ingestJob, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	tx, err := db.Begin(ctx)
	if err != nil {
		return evidenceReprocessResult{}, ingestJob{}, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", scope.TenantID); err != nil {
		return evidenceReprocessResult{}, ingestJob{}, err
	}
	lockKey := strings.Join([]string{scope.TenantID, scope.CollectionID, evidenceID}, "\x1f")
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, lockKey); err != nil {
		return evidenceReprocessResult{}, ingestJob{}, err
	}

	var existing evidenceReprocessResult
	err = tx.QueryRow(ctx, `
SELECT id::text, evidence_id::text, reprocess_of_job_id::text,
       reprocess_generation, queued_at, status::text, queue_published_at IS NULL
FROM forensic.records_ingest_jobs
WHERE tenant_id = $1 AND collection_id = $2 AND reprocess_request_key = $3`,
		scope.TenantID, scope.CollectionID, idempotencyKey,
	).Scan(&existing.JobID, &existing.EvidenceID, &existing.ReprocessOfJobID,
		&existing.ReprocessGeneration, &existing.QueuedAt, &existing.Status,
		&existing.PublicationPending)
	if err == nil {
		existing.IdempotencyKey = idempotencyKey
		existing.IdempotentReplay = true
		if err := tx.Commit(ctx); err != nil {
			return evidenceReprocessResult{}, ingestJob{}, err
		}
		return existing, ingestJob{}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return evidenceReprocessResult{}, ingestJob{}, err
	}

	var parentJobID string
	var job ingestJob
	var metadataBytes []byte
	var parentGeneration int
	var parentStatus string
	err = tx.QueryRow(ctx, `
SELECT jobs.id::text, jobs.tenant_id, jobs.user_id, jobs.collection_id,
       jobs.file_id, jobs.source_file, jobs.spool_path, jobs.sha256,
       jobs.record_type::text, jobs.metadata, jobs.reprocess_generation,
       jobs.status::text
FROM forensic.records_ingest_jobs jobs
JOIN forensic.evidence_items evidence
  ON evidence.evidence_id = jobs.evidence_id
 AND evidence.tenant_id = jobs.tenant_id
 AND evidence.collection_id = jobs.collection_id
WHERE jobs.tenant_id = $1 AND jobs.collection_id = $2
  AND jobs.evidence_id = $3::uuid
ORDER BY jobs.reprocess_generation DESC, jobs.queued_at DESC
LIMIT 1`, scope.TenantID, scope.CollectionID, evidenceID).Scan(
		&parentJobID, &job.TenantID, &job.UserID, &job.CollectionID, &job.FileID,
		&job.SourceFile, &job.SpoolPath, &job.SHA256, &job.RecordType,
		&metadataBytes, &parentGeneration, &parentStatus,
	)
	if err != nil {
		return evidenceReprocessResult{}, ingestJob{}, err
	}
	if parentStatus != "completed" && parentStatus != "dead_letter" {
		return evidenceReprocessResult{}, ingestJob{}, errEvidenceReprocessInProgress
	}
	job.Metadata, err = decodeIngestMetadata(metadataBytes)
	if err != nil {
		return evidenceReprocessResult{}, ingestJob{}, err
	}
	job.JobID = uuid.NewString()
	job.EvidenceID = evidenceID
	job.RequestID = requestID
	job.QueuedAt = time.Now().UTC()
	job.Metadata["request_id"] = requestID
	job.Metadata["reprocess_reason"] = reason
	job.Metadata["reprocess_of_job_id"] = parentJobID
	job.Metadata["reprocess_generation"] = fmt.Sprintf("%d", parentGeneration+1)
	job.Metadata["reprocess_requested_by"] = scope.ActorID
	metadataBytes, err = json.Marshal(job.Metadata)
	if err != nil {
		return evidenceReprocessResult{}, ingestJob{}, err
	}

	_, err = tx.Exec(ctx, `
INSERT INTO forensic.records_ingest_jobs (
  id, tenant_id, user_id, collection_id, file_id, source_file, spool_path,
  sha256, record_type, status, max_attempts, evidence_id, metadata, queued_at,
  queue_message_id, queue_publish_next_at, reprocess_of_job_id,
  reprocess_generation, reprocess_request_key
) VALUES (
  $1::uuid, $2, $3, $4, $5, $6, $7, $8, $9::forensic_record_type, 'queued',
  $10, $11::uuid, $12::jsonb, $13, $14, now(), $15::uuid, $16, $17
)`, job.JobID, job.TenantID, job.UserID, job.CollectionID, job.FileID,
		job.SourceFile, job.SpoolPath, job.SHA256, job.RecordType, maxAttempts,
		job.EvidenceID, metadataBytes, job.QueuedAt, queueMessageID(job.JobID),
		parentJobID, parentGeneration+1, idempotencyKey)
	if err != nil {
		return evidenceReprocessResult{}, ingestJob{}, err
	}
	_, err = tx.Exec(ctx, `
UPDATE forensic.evidence_items
SET processing_status = 'queued',
    metadata = metadata || jsonb_build_object(
      'latest_reprocess_job_id', $4::text,
      'latest_reprocess_generation', $5::integer,
      'latest_reprocess_reason', $6::text,
      'latest_reprocess_requested_at', now()
    )
WHERE tenant_id = $1 AND collection_id = $2 AND evidence_id = $3::uuid`,
		scope.TenantID, scope.CollectionID, evidenceID, job.JobID,
		parentGeneration+1, reason)
	if err != nil {
		return evidenceReprocessResult{}, ingestJob{}, err
	}
	_, err = tx.Exec(ctx, `
INSERT INTO forensic.records_audit_log (
  tenant_id, user_id, action, collection_id, file_id, batch_id, details
) VALUES ($1, $2, 'records.ingest.reprocess_requested', $3, $4, $5::uuid, $6::jsonb)`,
		scope.TenantID, scope.ActorID, scope.CollectionID, job.FileID, job.JobID,
		mustJSON(map[string]any{
			"evidence_id": evidenceID, "reprocess_of_job_id": parentJobID,
			"reprocess_generation": parentGeneration + 1, "reason": reason,
			"idempotency_key": idempotencyKey, "request_id": requestID,
		}))
	if err != nil {
		return evidenceReprocessResult{}, ingestJob{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return evidenceReprocessResult{}, ingestJob{}, err
	}
	return evidenceReprocessResult{
		Status: "queued", EvidenceID: evidenceID, JobID: job.JobID,
		ReprocessOfJobID: parentJobID, ReprocessGeneration: parentGeneration + 1,
		IdempotencyKey: idempotencyKey, QueuedAt: job.QueuedAt,
	}, job, nil
}

func mustJSON(value any) []byte {
	payload, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return payload
}
