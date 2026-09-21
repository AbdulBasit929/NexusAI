package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Phase 3 durable queue lifecycle", func() {
	It("uses persistent work-queue and bounded DLQ streams", func() {
		configs := forensicQueueStreamConfigs()
		Expect(configs).To(HaveLen(2))
		ingest := configs[0]
		Expect(ingest.Name).To(Equal(ingestStreamName))
		Expect(ingest.Subjects).To(Equal([]string{ingestSubject}))
		Expect(ingest.Retention).To(Equal(nats.WorkQueuePolicy))
		Expect(ingest.Storage).To(Equal(nats.FileStorage))
		Expect(ingest.MaxAge).To(Equal(30 * 24 * time.Hour))
		Expect(ingest.Duplicates).To(Equal(10 * time.Minute))

		dlq := configs[1]
		Expect(dlq.Name).To(Equal(ingestDLQStreamName))
		Expect(dlq.Subjects).To(Equal([]string{ingestDLQSubject}))
		Expect(dlq.Retention).To(Equal(nats.LimitsPolicy))
		Expect(dlq.Storage).To(Equal(nats.FileStorage))
		Expect(dlq.MaxAge).To(Equal(90 * 24 * time.Hour))
		Expect(dlq.MaxMsgs).To(Equal(int64(100_000)))
		Expect(dlq.MaxBytes).To(Equal(int64(1 << 30)))
	})

	It("uses stable publication identity and bounded producer backoff", func() {
		Expect(queueMessageID("job-a")).To(Equal("ingest-job:job-a"))
		Expect(queueBackoff(0)).To(Equal(2 * time.Second))
		Expect(queueBackoff(1)).To(Equal(2 * time.Second))
		Expect(queueBackoff(2)).To(Equal(4 * time.Second))
		Expect(queueBackoff(20)).To(Equal(5 * time.Minute))
		Expect(truncateQueueError(strings.Repeat("x", 3000))).To(HaveLen(2000))
	})

	It("persists and recovers a deduplicated message across a disposable NATS restart", func() {
		url := strings.TrimSpace(os.Getenv("FORENSIC_NATS_TEST_URL"))
		if url == "" {
			Skip("FORENSIC_NATS_TEST_URL is not configured")
		}
		mode := strings.TrimSpace(os.Getenv("FORENSIC_NATS_TEST_MODE"))
		nc, err := nats.Connect(url, nats.Timeout(5*time.Second))
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(nc.Close)
		publisher, err := newJetStreamIngestPublisher(nc)
		Expect(err).NotTo(HaveOccurred())
		job := ingestJob{
			JobID: "00000000-0000-0000-0000-000000000042", TenantID: "phase3-test",
			CollectionID: "queue-acceptance", FileID: "file-42", SourceFile: "calls.csv",
			SpoolPath: "/data/forensic/spool/object-42", SHA256: strings.Repeat("4", 64),
			RecordType: "cdr", QueuedAt: time.Now().UTC(), Metadata: map[string]string{"request_id": "queue-restart-test"},
		}
		if mode == "publish" {
			Expect(publisher.Publish(context.Background(), job)).To(Succeed())
			Expect(publisher.Publish(context.Background(), job)).To(Succeed())
			info, err := publisher.js.StreamInfo(ingestStreamName)
			Expect(err).NotTo(HaveOccurred())
			Expect(info.State.Msgs).To(Equal(uint64(1)))
			return
		}
		if mode != "verify" {
			Skip("FORENSIC_NATS_TEST_MODE must be publish or verify")
		}
		info, err := publisher.js.StreamInfo(ingestStreamName)
		Expect(err).NotTo(HaveOccurred())
		Expect(info.State.Msgs).To(Equal(uint64(1)))
		sub, err := publisher.js.PullSubscribe(
			ingestSubject, "phase3-queue-restart-acceptance",
			nats.BindStream(ingestStreamName), nats.AckExplicit(),
		)
		Expect(err).NotTo(HaveOccurred())
		messages, err := sub.Fetch(1, nats.MaxWait(5*time.Second))
		Expect(err).NotTo(HaveOccurred())
		Expect(messages).To(HaveLen(1))
		var recovered ingestJob
		Expect(json.Unmarshal(messages[0].Data, &recovered)).To(Succeed())
		Expect(recovered.JobID).To(Equal(job.JobID))
		Expect(messages[0].Header.Get("Nats-Msg-Id")).To(Equal(queueMessageID(job.JobID)))
		Expect(messages[0].AckSync(nats.Context(context.Background()))).To(Succeed())
		Eventually(func() uint64 {
			info, err := publisher.js.StreamInfo(ingestStreamName)
			Expect(err).NotTo(HaveOccurred())
			return info.State.Msgs
		}, 5*time.Second, 100*time.Millisecond).Should(BeZero())
	})

	It("requires safe reprocessing idempotency keys", func() {
		Expect(reprocessIdempotencyKeyRE.MatchString("case-123:attempt-01")).To(BeTrue())
		Expect(reprocessIdempotencyKeyRE.MatchString("short")).To(BeFalse())
		Expect(reprocessIdempotencyKeyRE.MatchString("unsafe key with spaces")).To(BeFalse())
		Expect(reprocessIdempotencyKeyRE.MatchString(strings.Repeat("a", 129))).To(BeFalse())
	})

	It("enforces the additive migration, terminal immutability, and gated rollback contracts", func() {
		root := filepath.Clean(filepath.Join("..", ".."))
		migrationRoot := filepath.Join(root, "db", "forensic_records")
		forward := readQueueContract(filepath.Join(migrationRoot, "009_phase3_queue_lifecycle.sql"))
		preflight := readQueueContract(filepath.Join(migrationRoot, "009_phase3_queue_lifecycle.preflight.sql"))
		verify := readQueueContract(filepath.Join(migrationRoot, "009_phase3_queue_lifecycle.verify.sql"))
		smoke := readQueueContract(filepath.Join(migrationRoot, "009_phase3_queue_lifecycle.smoke.sql"))
		rollback := readQueueContract(filepath.Join(migrationRoot, "009_phase3_queue_lifecycle.rollback.sql"))

		for _, column := range []string{
			"queue_message_id", "queue_published_at", "queue_publish_lease_token",
			"worker_lease_token", "worker_lease_expires_at", "next_attempt_at",
			"dead_lettered_at", "acknowledged_at", "reprocess_of_job_id",
			"reprocess_generation", "reprocess_request_key",
		} {
			Expect(forward).To(ContainSubstring("ADD COLUMN IF NOT EXISTS "+column), column)
		}
		Expect(forward).To(ContainSubstring("terminal ingest job status is immutable"))
		Expect(forward).To(ContainSubstring("attempt_count cannot decrease"))
		Expect(forward).To(ContainSubstring("records_ingest_jobs_reprocess_request_uidx"))
		Expect(preflight).To(ContainSubstring("migration 008 must be applied"))
		Expect(preflight).To(ContainSubstring("requires a drained legacy worker"))
		Expect(verify).To(ContainSubstring("queue lifecycle history trigger is missing"))
		Expect(smoke).To(ContainSubstring("terminal queue state mutation was not rejected"))
		Expect(smoke).To(ContainSubstring("did not synchronize to processing_runs"))
		Expect(rollback).To(ContainSubstring("phase3_allow_destructive_rollback"))
		Expect(strings.TrimSpace(forward)).To(HaveSuffix("COMMIT;"))
	})

	It("wires JetStream ack-after-commit, leases, DLQ and persistent NATS storage", func() {
		root := filepath.Clean(filepath.Join("..", ".."))
		worker := readQueueContract(filepath.Join(root, "ingestion", "forensic_records", "worker.py"))
		compose := readQueueContract(filepath.Join(root, "docker-compose.forensic-records.yaml"))
		mainSource := readQueueContract(filepath.Join(root, "api", "forensic_records", "main.go"))

		for _, contract := range []string{
			"manual_ack=True", "ack_sync(timeout=5.0)", "msg.nak(delay=",
			"await msg.term()", "worker_lease_token", "FOR UPDATE",
			"reprocess_generation", "Nats-Msg-Id", "validate_queue_schema",
			`job_status = "dead_letter" if terminal else "failed"`,
		} {
			Expect(worker).To(ContainSubstring(contract), contract)
		}
		Expect(compose).To(ContainSubstring(`command: ["-js", "-sd", "/data/jetstream"`))
		Expect(compose).To(ContainSubstring("forensic_nats_data:/data/jetstream"))
		Expect(mainSource).To(ContainSubstring("persistQueuedJobTx(ctx, tx, job)"))
		Expect(mainSource).To(MatchRegexp(`(?s)persistQueuedJobTx\(ctx, tx, job\).*tx.Commit\(ctx\)`))
		Expect(mainSource).To(ContainSubstring("publishQueuedJob"))
		Expect(mainSource).To(ContainSubstring("validateQueueSchema"))
		Expect(mainSource).To(ContainSubstring("runQueueOutbox"))
	})
})

func readQueueContract(path string) string {
	data, err := os.ReadFile(path)
	Expect(err).NotTo(HaveOccurred())
	return strings.ReplaceAll(string(data), "\r\n", "\n")
}
