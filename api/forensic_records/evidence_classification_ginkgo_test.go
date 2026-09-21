package main

import (
	"bytes"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Universal forensic evidence classification", func() {
	It("propagates deterministic registration warnings without duplicating classification warnings", func() {
		classification := evidenceClassification{Warnings: []string{"classification warning"}}
		registration := evidenceRegistration{Warnings: []string{"classification warning", "archive extraction blocked"}}

		classification = applyEvidenceRegistrationWarnings(classification, registration)

		Expect(classification.Warnings).To(ConsistOf("classification warning", "archive extraction blocked"))
	})

	DescribeTable("routes real-world formats without sending unsupported formats to the records worker",
		func(filename, contentType string, headers []string, hints map[string]string, modality, detectedType, route string, queueRecords bool) {
			classification := classifyEvidenceItem(filename, contentType, detectRecordType(headers), headers, hints)
			Expect(classification.Modality).To(Equal(modality))
			Expect(classification.DetectedType).To(Equal(detectedType))
			Expect(classification.ProcessingRoute).To(Equal(route))
			Expect(classification.QueueRecords).To(Equal(queueRecords))
		},
		Entry("CDR CSV", "calls.csv", "text/csv", []string{"timestamp", "source_number", "target_number", "duration_seconds"}, nil, "structured_records", "cdr", "forensic_records_worker", true),
		Entry("IPDR JSONL", "sessions.jsonl", "application/x-ndjson", []string{"timestamp", "source_ip", "destination_ip", "bytes"}, nil, "structured_records", "ipdr", "forensic_records_worker", true),
		Entry("ANPR CSV", "sightings.csv", "text/csv", []string{"timestamp", "plate_number", "camera_id"}, nil, "structured_records", "anpr", "forensic_records_worker", true),
		Entry("Pakistan ANPR vendor CSV", "pakistan-sightings.csv", "text/csv", []string{"Registration No.", "Camera Code", "Captured At", "Camera Location"}, nil, "structured_records", "anpr", "forensic_records_worker", true),
		Entry("subscriber CSV", "subscribers.csv", "text/csv", []string{"msisdn", "subscriber_id", "name"}, nil, "structured_records", "subscriber", "forensic_records_worker", true),
		Entry("Pakistan subscriber vendor JSONL", "pakistan-subscribers.jsonl", "application/x-ndjson", []string{"mobile_no", "full_name", "cnic_no", "customer_no"}, nil, "structured_records", "subscriber", "forensic_records_worker", true),
		Entry("tower CSV", "towers.csv", "text/csv", []string{"cell_id", "latitude", "longitude"}, nil, "structured_records", "tower_location", "forensic_records_worker", true),
		Entry("Pakistan tower-sector CSV", "pakistan-towers.csv", "text/csv", []string{"Site Code", "Sector ID", "Latitude WGS84", "Longitude WGS84"}, nil, "structured_records", "tower_location", "forensic_records_worker", true),
		Entry("financial transaction CSV", "transactions.csv", "text/csv", []string{"timestamp", "amount", "from_account", "to_account"}, nil, "structured_records", "transaction", "forensic_records_worker", true),
		Entry("Pakistan PKR transaction JSONL", "pakistan-transactions.jsonl", "application/x-ndjson", []string{"txn_id", "txn_time", "debit_account", "debit_amount", "currency_code"}, nil, "structured_records", "transaction", "forensic_records_worker", true),
		Entry("access log", "access.log", "text/plain", []string{"timestamp", "source_ip", "username", "path"}, nil, "structured_records", "access_log", "forensic_records_worker", true),
		Entry("Pakistan security event JSONL", "pakistan-security.jsonl", "application/x-ndjson", []string{"event_timestamp", "remote_addr", "principal", "event_action"}, nil, "structured_records", "access_log", "forensic_records_worker", true),
		Entry("generic CSV", "observations.csv", "text/csv", []string{"event_time", "custom_value"}, nil, "structured_records", "generic", "forensic_records_worker", true),
		Entry("schema-less JSON awaits validation", "broken.json", "application/json", nil, nil, "structured_data", "json_records", "structured_validation_pending", false),
		Entry("CDR TSV uses accepted delimiter adapter", "calls.tsv", "text/tab-separated-values", []string{"timestamp", "source_number", "target_number"}, nil, "structured_records", "cdr", "forensic_records_worker", true),
		Entry("financial workbook uses accepted read-only XLSX adapter", "transactions.xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", []string{"amount", "account_number"}, nil, "structured_records", "transaction", "forensic_records_worker", true),
		Entry("workbook schema detection is safely deferred to the worker", "unknown-columns.xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", nil, nil, "tabular", "generic", "forensic_records_worker", true),
		Entry("PDF document", "policy.pdf", "application/pdf", nil, nil, "document", "pdf", "native_document_worker", true),
		Entry("Office document", "statement.docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", nil, nil, "document", "document", "native_document_worker", true),
		Entry("image evidence", "scene.jpg", "image/jpeg", nil, nil, "image", "image", "unified_media_worker", true),
		Entry("audio evidence", "interview.wav", "audio/wav", nil, nil, "audio", "audio", "unified_media_worker", true),
		Entry("TTS output artifact", "narration.wav", "audio/wav", nil, map[string]string{"evidence_role": "tts_output", "parent_evidence_id": "parent-1"}, "audio", "tts_output", "tts_artifact_registry", false),
		Entry("STT transcript artifact", "interview.vtt", "text/vtt", nil, map[string]string{"evidence_role": "stt_transcript", "parent_evidence_id": "parent-1"}, "text", "stt_transcript", "transcript_index_pending", false),
		Entry("video evidence", "camera.mp4", "video/mp4", nil, nil, "video", "video", "unified_media_worker", true),
		Entry("packet capture", "traffic.pcapng", "application/octet-stream", nil, nil, "network_or_system_capture", "capture", "capture_adapter_pending", false),
		Entry("SQLite database", "device.sqlite", "application/vnd.sqlite3", nil, nil, "database", "database", "database_adapter_pending", false),
		Entry("evidence archive", "export.zip", "application/zip", nil, nil, "container", "archive", "archive_inventory_pending", false),
		Entry("unknown binary", "device.dump", "application/octet-stream", nil, nil, "unknown", "unknown", "manual_review", false),
	)

	It("queues KB-linked media through the unified worker while preserving hybrid storage", func() {
		classification := classifyEvidenceItem("interview.wav", "audio/wav", "generic", nil)
		classification = finalizeEvidenceClassification(classification, "entry/interview.wav")
		Expect(classification.QueueRecords).To(BeTrue())
		Expect(classification.StorageMode).To(Equal("hybrid"))
		Expect(classification.ProcessingStatus).To(Equal("queued"))
	})

	It("marks only a completed KB text route as completed", func() {
		classification := classifyEvidenceItem("notes.md", "text/markdown", "generic", nil)
		classification = finalizeEvidenceClassification(classification, "entry/notes.md")
		Expect(classification.QueueRecords).To(BeFalse())
		Expect(classification.StorageMode).To(Equal("rag_only"))
		Expect(classification.ProcessingRoute).To(Equal("kb_text_index"))
		Expect(classification.ProcessingStatus).To(Equal("completed"))
	})

	DescribeTable("reports whether a duplicate upload created a distinct KB source link",
		func(linked bool, expected string) {
			Expect(duplicateEvidenceMessage(linked)).To(Equal(expected))
		},
		Entry("source linked", true, "same evidence hash already exists; the distinct Knowledge Base source entry was linked to the existing evidence"),
		Entry("concurrent duplicate", false, "evidence was registered concurrently by another request"),
	)

	It("builds PostgreSQL-safe and unambiguous evidence advisory lock keys", func() {
		key := evidenceAdvisoryLockKey("tenant-a", "case-a", "abc123")
		Expect(key).NotTo(ContainSubstring("\x00"))
		Expect(key).To(MatchJSON(`["tenant-a","case-a","abc123"]`))
		Expect(evidenceAdvisoryLockKey("a", "bc", "d")).NotTo(Equal(evidenceAdvisoryLockKey("ab", "c", "d")))
	})

	It("preserves Pakistan jurisdiction and source timezone in evidence metadata", func() {
		job := ingestJob{
			RequestID:  "request-1",
			FileID:     "file-1",
			JobID:      "job-1",
			RecordType: "cdr",
			Metadata: map[string]string{
				"jurisdiction":    "PK",
				"source_timezone": "Asia/Karachi",
			},
		}
		metadata := evidenceRegistrationMetadata(job, evidenceClassification{
			ProcessingRoute: "forensic_records_worker",
			StorageMode:     "hybrid",
		}, "version-1")
		Expect(metadata).To(HaveKeyWithValue("jurisdiction", "PK"))
		Expect(metadata).To(HaveKeyWithValue("source_timezone", "Asia/Karachi"))
	})

	It("keeps structured evidence queued and hybrid when a KB entry exists", func() {
		headers := []string{"timestamp", "source_number", "target_number"}
		classification := classifyEvidenceItem("calls.csv", "text/csv", detectRecordType(headers), headers)
		classification = finalizeEvidenceClassification(classification, "entry/calls.csv")
		Expect(classification.QueueRecords).To(BeTrue())
		Expect(classification.StorageMode).To(Equal("hybrid"))
		Expect(classification.ProcessingStatus).To(Equal("queued"))
	})

	It("spools binary evidence without attempting tabular header detection", func() {
		dir := GinkgoT().TempDir()
		store, err := newContentAddressedStore(filepath.Join(dir, "spool"))
		Expect(err).NotTo(HaveOccurred())
		stored, err := store.Retain(bytes.NewReader([]byte{'"', 0x89, 'P', 'N', 'G', '\r', '\n'}), "default", "binary-case")
		Expect(err).NotTo(HaveOccurred())
		headers, err := readEvidenceHeaders(stored.Path, "evidence.bin")
		Expect(err).NotTo(HaveOccurred())
		Expect(stored.SHA256).To(HaveLen(64))
		Expect(headers).To(BeEmpty())
		Expect(stored.Path).To(BeAnExistingFile())
	})

	It("does not place binary workbook bytes in JSONB header metadata", func() {
		path := filepath.Join(GinkgoT().TempDir(), "transactions.xlsx")
		Expect(os.WriteFile(path, []byte{'P', 'K', 3, 4, 0, 0, 0, 0, 0xff}, 0o600)).To(Succeed())
		headers, err := readCSVHeaders(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(headers).To(BeEmpty())
	})

	It("rejects NUL-bearing CSV headers before they can reach PostgreSQL JSONB", func() {
		path := filepath.Join(GinkgoT().TempDir(), "unsafe.csv")
		Expect(os.WriteFile(path, []byte("timestamp,source\x00_number,target_number\n"), 0o600)).To(Succeed())
		headers, err := readCSVHeaders(path)
		Expect(err).To(MatchError(ContainSubstring("NUL")))
		Expect(headers).To(BeEmpty())
	})

	It("recognizes the ready access-log fixture client_ip alias", func() {
		path := filepath.Join(GinkgoT().TempDir(), "access_log_sample.jsonl")
		Expect(os.WriteFile(path, []byte("{\"timestamp\":\"2026-07-24T00:00:00Z\",\"client_ip\":\"192.0.2.10\",\"user\":\"analyst\",\"path\":\"/api/records\"}\n"), 0o600)).To(Succeed())
		headers, err := readCSVHeaders(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(detectRecordType(headers)).To(Equal("access_log"))
	})

	It("detects fields from pretty-printed JSON instead of treating the opening brace as a CSV header", func() {
		path := filepath.Join(GinkgoT().TempDir(), "records.json")
		Expect(os.WriteFile(path, []byte("{\n  \"timestamp\": \"2026-07-24T00:00:00Z\",\n  \"source_ip\": \"10.0.0.1\",\n  \"destination_ip\": \"8.8.8.8\"\n}\n"), 0o600)).To(Succeed())
		headers, err := readCSVHeaders(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(headers).To(ConsistOf("timestamp", "source_ip", "destination_ip"))
		Expect(headers).NotTo(ContainElement("{"))
	})
})

var _ = Describe("Knowledge Base evidence lineage enrichment", func() {
	It("adds tenant, evidence version, stable retrieval ID, and chunk digest", func() {
		req := hybridQueryRequest{TenantID: "tenant-a", CollectionID: "case-a"}
		evidence := map[string]any{
			"mode": "vector_search",
			"results": []map[string]any{{
				"id":      "chunk-7",
				"content": "retrieved policy evidence",
				"metadata": map[string]any{
					"file_name": "policy.pdf",
					"source":    "entry/policy.pdf",
				},
			}},
		}
		lineage := map[string]evidenceLineage{
			"entry:entry/policy.pdf": {
				EvidenceID:  "evidence-1",
				VersionID:   "version-1",
				SourceFile:  "policy.pdf",
				SourceEntry: "entry/policy.pdf",
			},
		}

		enriched := applyEvidenceLineage(req, evidence, lineage)
		results := evidenceResults(enriched)
		Expect(results).To(HaveLen(1))
		metadata := results[0]["metadata"].(map[string]any)
		Expect(metadata).To(HaveKeyWithValue("tenant_id", "tenant-a"))
		Expect(metadata).To(HaveKeyWithValue("collection_id", "case-a"))
		Expect(metadata).To(HaveKeyWithValue("evidence_id", "evidence-1"))
		Expect(metadata).To(HaveKeyWithValue("version_id", "version-1"))
		Expect(metadata).To(HaveKeyWithValue("chunk_id", "chunk-7"))
		Expect(metadata["chunk_sha256"]).To(HaveLen(64))
	})
})
