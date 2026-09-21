package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
)

const (
	ingestStreamName      = "FORENSIC_RECORDS_INGEST"
	ingestDLQStreamName   = "FORENSIC_RECORDS_DLQ"
	ingestDLQSubject      = "forensic.records.ingest.dead_letter"
	ingestConsumerName    = "forensic-records-worker-v1"
	queueOutboxPoll       = 2 * time.Second
	queuePublishLease     = 30 * time.Second
	queuePublishBatchSize = 25
)

type ingestPublisher interface {
	Publish(context.Context, ingestJob) error
}

type jetStreamIngestPublisher struct {
	js nats.JetStreamContext
}

func newJetStreamIngestPublisher(nc *nats.Conn) (*jetStreamIngestPublisher, error) {
	js, err := nc.JetStream(nats.PublishAsyncMaxPending(256))
	if err != nil {
		return nil, fmt.Errorf("initialize JetStream: %w", err)
	}
	for _, cfg := range forensicQueueStreamConfigs() {
		if err := ensureQueueStream(js, cfg); err != nil {
			return nil, err
		}
	}
	return &jetStreamIngestPublisher{js: js}, nil
}

func forensicQueueStreamConfigs() []*nats.StreamConfig {
	return []*nats.StreamConfig{
		{
			Name: ingestStreamName, Subjects: []string{ingestSubject},
			Retention: nats.WorkQueuePolicy, Storage: nats.FileStorage,
			MaxAge: 30 * 24 * time.Hour, Duplicates: 10 * time.Minute,
		},
		{
			Name: ingestDLQStreamName, Subjects: []string{ingestDLQSubject},
			Retention: nats.LimitsPolicy, Storage: nats.FileStorage,
			MaxAge: 90 * 24 * time.Hour, MaxMsgs: 100_000, MaxBytes: 1 << 30,
			Duplicates: 10 * time.Minute,
		},
	}
}

func ensureQueueStream(js nats.JetStreamContext, expected *nats.StreamConfig) error {
	info, err := js.StreamInfo(expected.Name)
	if errors.Is(err, nats.ErrStreamNotFound) {
		if _, err := js.AddStream(expected); err != nil {
			return fmt.Errorf("create JetStream stream %s: %w", expected.Name, err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect JetStream stream %s: %w", expected.Name, err)
	}
	actual := info.Config
	if actual.Retention != expected.Retention || actual.Storage != expected.Storage ||
		len(actual.Subjects) != 1 || actual.Subjects[0] != expected.Subjects[0] {
		return fmt.Errorf("JetStream stream %s has an incompatible retention, storage, or subject contract", expected.Name)
	}
	if actual.MaxAge != expected.MaxAge || actual.MaxMsgs != expected.MaxMsgs ||
		actual.MaxBytes != expected.MaxBytes || actual.Duplicates != expected.Duplicates {
		if _, err := js.UpdateStream(expected); err != nil {
			return fmt.Errorf("reconcile JetStream stream %s limits: %w", expected.Name, err)
		}
	}
	return nil
}

func (p *jetStreamIngestPublisher) Publish(ctx context.Context, job ingestJob) error {
	payload, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal ingest job: %w", err)
	}
	msg := nats.NewMsg(ingestSubject)
	msg.Data = payload
	msg.Header.Set("Nats-Msg-Id", queueMessageID(job.JobID))
	msg.Header.Set("X-Request-ID", job.RequestID)
	ack, err := p.js.PublishMsg(msg, nats.Context(ctx))
	if err != nil {
		return fmt.Errorf("persist ingest job in JetStream: %w", err)
	}
	if ack == nil || ack.Stream != ingestStreamName {
		return errors.New("JetStream returned an invalid ingest publication acknowledgement")
	}
	return nil
}

func queueMessageID(jobID string) string {
	return "ingest-job:" + strings.TrimSpace(jobID)
}

func queueBackoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	delay := 2 * time.Second
	for i := 1; i < attempt && delay < 5*time.Minute; i++ {
		delay *= 2
	}
	if delay > 5*time.Minute {
		return 5 * time.Minute
	}
	return delay
}

func persistQueuedJob(ctx context.Context, db *pgxpool.Pool, job ingestJob) error {
	if db == nil {
		return nil
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin queue outbox job: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`, job.TenantID); err != nil {
		return fmt.Errorf("set queue outbox tenant: %w", err)
	}
	if err := persistQueuedJobTx(ctx, tx, job); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit queue outbox job: %w", err)
	}
	return nil
}

func persistQueuedJobTx(ctx context.Context, tx pgx.Tx, job ingestJob) error {
	metadata, err := json.Marshal(job.Metadata)
	if err != nil {
		return fmt.Errorf("marshal queued job metadata: %w", err)
	}
	_, err = tx.Exec(ctx, `
INSERT INTO forensic.records_ingest_jobs (
  id, tenant_id, user_id, collection_id, file_id, source_file, spool_path,
  sha256, record_type, status, evidence_id, metadata, queued_at,
  queue_message_id, queue_publish_next_at
) VALUES (
  $1::uuid, $2, $3, $4, $5, $6, $7, $8, $9::forensic_record_type, 'queued',
  NULLIF($10, '')::uuid, $11::jsonb, $12, $13, now()
)
ON CONFLICT (id) DO NOTHING`,
		job.JobID, job.TenantID, job.UserID, job.CollectionID, job.FileID,
		job.SourceFile, job.SpoolPath, job.SHA256, job.RecordType, job.EvidenceID,
		metadata, job.QueuedAt, queueMessageID(job.JobID),
	)
	if err != nil {
		return fmt.Errorf("persist queue outbox job: %w", err)
	}
	return nil
}

func validateQueueSchema(ctx context.Context, db *pgxpool.Pool) error {
	if db == nil {
		return nil
	}
	var ready bool
	err := db.QueryRow(ctx, `
SELECT to_regclass('forensic.records_ingest_jobs') IS NOT NULL
   AND to_regprocedure('forensic.protect_ingest_job_history()') IS NOT NULL
   AND NOT EXISTS (
     SELECT required.column_name
     FROM (VALUES
       ('queue_message_id'), ('queue_published_at'), ('queue_publish_attempts'),
       ('queue_publish_next_at'), ('queue_publish_lease_token'),
       ('worker_lease_token'), ('worker_lease_expires_at'), ('next_attempt_at'),
       ('dead_lettered_at'), ('acknowledged_at'), ('reprocess_of_job_id'),
       ('reprocess_generation'), ('reprocess_request_key')
     ) AS required(column_name)
     WHERE NOT EXISTS (
       SELECT 1 FROM information_schema.columns actual
       WHERE actual.table_schema='forensic'
         AND actual.table_name='records_ingest_jobs'
       AND actual.column_name=required.column_name
     )
   )
   AND NOT EXISTS (
     SELECT required.table_name
     FROM (VALUES
       ('records_ingest_jobs', 'tenant_isolation_records_ingest_jobs'),
       ('records_ingest_errors', 'tenant_isolation_records_ingest_errors'),
       ('records_audit_log', 'tenant_isolation_records_audit_log')
     ) AS required(table_name, policy_name)
     WHERE NOT EXISTS (
       SELECT 1 FROM pg_policies actual
       WHERE actual.schemaname='forensic'
         AND actual.tablename=required.table_name
         AND actual.policyname=required.policy_name
     )
   )
   AND NOT EXISTS (
     SELECT required.view_name
     FROM (VALUES ('cdr_frequent_contacts'), ('entity_activity_summary')) AS required(view_name)
     LEFT JOIN pg_class actual
       ON actual.relnamespace='forensic'::regnamespace
      AND actual.relname=required.view_name
      AND actual.relkind='v'
     WHERE actual.oid IS NULL
        OR NOT coalesce(actual.reloptions, ARRAY[]::text[]) @> ARRAY['security_invoker=true']
   )`).Scan(&ready)
	if err != nil {
		return fmt.Errorf("inspect Phase 3 queue schema: %w", err)
	}
	if !ready {
		return errors.New("Phase 3 runtime schema is not ready; apply and verify migrations 009 and 010 before starting the API")
	}
	return nil
}

type claimedQueuePublication struct {
	Job        ingestJob
	LeaseToken string
	Attempt    int
}

func claimQueuePublication(ctx context.Context, db *pgxpool.Pool, tenantID, jobID string) (*claimedQueuePublication, error) {
	token := uuid.NewString()
	tx, err := db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin queue publication claim: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`, tenantID); err != nil {
		return nil, fmt.Errorf("set queue publication tenant: %w", err)
	}
	row := tx.QueryRow(ctx, `
UPDATE forensic.records_ingest_jobs
SET queue_publish_lease_token = $3::uuid,
    queue_publish_lease_expires_at = now() + $4::interval,
    queue_publish_attempts = queue_publish_attempts + 1,
    queue_publish_error = NULL
WHERE tenant_id = $1
  AND id = $2::uuid
  AND status = 'queued'
  AND queue_published_at IS NULL
  AND coalesce(queue_publish_next_at, now()) <= now()
  AND (queue_publish_lease_expires_at IS NULL OR queue_publish_lease_expires_at <= now())
RETURNING id::text, coalesce(evidence_id::text, ''), tenant_id, user_id, collection_id,
          file_id, source_file, spool_path, sha256, record_type::text,
          queued_at, metadata, queue_publish_attempts`,
		tenantID, jobID, token, queuePublishLease.String(),
	)
	var job ingestJob
	var metadata []byte
	var attempt int
	if err := row.Scan(
		&job.JobID, &job.EvidenceID, &job.TenantID, &job.UserID, &job.CollectionID,
		&job.FileID, &job.SourceFile, &job.SpoolPath, &job.SHA256, &job.RecordType,
		&job.QueuedAt, &metadata, &attempt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			if err := tx.Commit(ctx); err != nil {
				return nil, fmt.Errorf("commit empty queue publication claim: %w", err)
			}
			return nil, nil
		}
		return nil, fmt.Errorf("claim queue publication: %w", err)
	}
	job.Metadata, err = decodeIngestMetadata(metadata)
	if err != nil {
		return nil, fmt.Errorf("decode queued job metadata: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit queue publication claim: %w", err)
	}
	job.RequestID = job.Metadata["request_id"]
	return &claimedQueuePublication{Job: job, LeaseToken: token, Attempt: attempt}, nil
}

func finishQueuePublication(ctx context.Context, db *pgxpool.Pool, claim *claimedQueuePublication, publishErr error) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin queue publication result: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`, claim.Job.TenantID); err != nil {
		return fmt.Errorf("set queue publication result tenant: %w", err)
	}
	if publishErr == nil {
		command, err := tx.Exec(ctx, `
UPDATE forensic.records_ingest_jobs
SET queue_published_at = now(), queue_publish_next_at = NULL,
    queue_publish_error = NULL, queue_publish_lease_token = NULL,
    queue_publish_lease_expires_at = NULL
WHERE id = $1::uuid AND queue_publish_lease_token = $2::uuid`, claim.Job.JobID, claim.LeaseToken)
		if err != nil {
			return fmt.Errorf("record queue publication: %w", err)
		}
		if command.RowsAffected() != 1 {
			return errors.New("queue publication lease was lost before completion")
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit queue publication result: %w", err)
		}
		return nil
	}
	delay := queueBackoff(claim.Attempt)
	_, err = tx.Exec(ctx, `
UPDATE forensic.records_ingest_jobs
SET queue_publish_error = $3,
    queue_publish_next_at = now() + $4::interval,
    queue_publish_lease_token = NULL,
    queue_publish_lease_expires_at = NULL
WHERE id = $1::uuid AND queue_publish_lease_token = $2::uuid`,
		claim.Job.JobID, claim.LeaseToken, truncateQueueError(publishErr.Error()), delay.String(),
	)
	if err != nil {
		return fmt.Errorf("schedule queue publication retry: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit queue publication retry: %w", err)
	}
	return publishErr
}

func publishQueuedJob(ctx context.Context, db *pgxpool.Pool, publisher ingestPublisher, tenantID, jobID string) (bool, error) {
	claim, err := claimQueuePublication(ctx, db, tenantID, jobID)
	if err != nil || claim == nil {
		return false, err
	}
	publishErr := publisher.Publish(ctx, claim.Job)
	if err := finishQueuePublication(ctx, db, claim, publishErr); err != nil {
		return false, err
	}
	return true, nil
}

func runQueueOutbox(ctx context.Context, db *pgxpool.Pool, publisher ingestPublisher, tenantID string) {
	ticker := time.NewTicker(queueOutboxPoll)
	defer ticker.Stop()
	for {
		if err := publishDueQueueJobs(ctx, db, publisher, tenantID); err != nil && !errors.Is(err, context.Canceled) {
			slog.Error("queue outbox dispatch failed", "tenant_id", tenantID, "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func publishDueQueueJobs(ctx context.Context, db *pgxpool.Pool, publisher ingestPublisher, tenantID string) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin queue outbox scan: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`, tenantID); err != nil {
		return fmt.Errorf("set queue outbox tenant: %w", err)
	}
	rows, err := tx.Query(ctx, `
SELECT id::text
FROM forensic.records_ingest_jobs
WHERE tenant_id = $1 AND status = 'queued' AND queue_published_at IS NULL
  AND coalesce(queue_publish_next_at, now()) <= now()
  AND (queue_publish_lease_expires_at IS NULL OR queue_publish_lease_expires_at <= now())
ORDER BY queued_at
LIMIT $2`, tenantID, queuePublishBatchSize)
	if err != nil {
		return fmt.Errorf("list due queue publications: %w", err)
	}
	var jobIDs []string
	for rows.Next() {
		var jobID string
		if err := rows.Scan(&jobID); err != nil {
			rows.Close()
			return err
		}
		jobIDs = append(jobIDs, jobID)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit queue outbox scan: %w", err)
	}
	for _, jobID := range jobIDs {
		if _, err := publishQueuedJob(ctx, db, publisher, tenantID, jobID); err != nil {
			slog.Warn("queue publication deferred", "job_id", jobID, "error", err)
		}
	}
	return nil
}

func truncateQueueError(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 2000 {
		return value[:2000]
	}
	return value
}
