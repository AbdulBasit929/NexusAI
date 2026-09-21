package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Phase 3 evidence control-plane migration", func() {
	var forward string
	var rollback string
	var preflight string
	var verify string
	var idempotencyVerify string
	var rlsSmoke string

	BeforeEach(func() {
		repositoryRoot := filepath.Clean(filepath.Join("..", ".."))
		migrationRoot := filepath.Join(repositoryRoot, "db", "forensic_records")
		forward = readMigrationContract(filepath.Join(migrationRoot, "008_phase3_evidence_control_plane.sql"))
		rollback = readMigrationContract(filepath.Join(migrationRoot, "008_phase3_evidence_control_plane.rollback.sql"))
		preflight = readMigrationContract(filepath.Join(migrationRoot, "008_phase3_evidence_control_plane.preflight.sql"))
		verify = readMigrationContract(filepath.Join(migrationRoot, "008_phase3_evidence_control_plane.verify.sql"))
		idempotencyVerify = readMigrationContract(filepath.Join(migrationRoot, "008_phase3_evidence_control_plane.idempotency.verify.sql"))
		rlsSmoke = readMigrationContract(filepath.Join(migrationRoot, "008_phase3_evidence_control_plane.rls.smoke.sql"))
	})

	It("normalizes every required control-plane object and tenant boundary", func() {
		for _, table := range []string{
			"evidence_storage_objects",
			"evidence_versions",
			"evidence_source_links",
			"processing_runs",
			"processing_events",
			"derived_artifacts",
			"evidence_custody_events",
		} {
			Expect(forward).To(ContainSubstring("CREATE TABLE IF NOT EXISTS forensic."+table), table)
			Expect(forward).To(ContainSubstring("ALTER TABLE forensic."+table+" ENABLE ROW LEVEL SECURITY"), table)
			Expect(forward).To(MatchRegexp(`(?s)CREATE POLICY tenant_isolation_`+regexp.QuoteMeta(table)+`.*WITH CHECK`), table)
		}
		Expect(forward).To(ContainSubstring("ADD COLUMN IF NOT EXISTS current_version_id uuid"))
		Expect(forward).To(ContainSubstring("evidence_items_current_version_fk"))
		Expect(forward).To(ContainSubstring("idempotency_key text NOT NULL"))
		Expect(forward).To(ContainSubstring("model_revision text"))
		Expect(forward).To(ContainSubstring("adapter_revision text"))
	})

	It("enforces immutable identities and append-only hash-chained events", func() {
		Expect(forward).To(ContainSubstring("reject_append_only_mutation"))
		Expect(forward).To(ContainSubstring("protect_storage_identity"))
		Expect(forward).To(ContainSubstring("protect_evidence_identity"))
		Expect(forward).To(ContainSubstring("prepare_custody_event"))
		Expect(forward).To(ContainSubstring("pg_advisory_xact_lock"))
		Expect(forward).To(ContainSubstring("previous_event_sha256"))
		Expect(forward).To(ContainSubstring("event_sha256"))
		Expect(forward).To(ContainSubstring("digest(convert_to"))
		Expect(forward).To(ContainSubstring("evidence_versions_append_only"))
		Expect(forward).To(ContainSubstring("processing_events_append_only"))
		Expect(forward).To(ContainSubstring("evidence_custody_events_append_only"))
	})

	It("backfills legacy evidence, KB links, jobs, and custody without deleting Phase 2 data", func() {
		Expect(forward).To(ContainSubstring("phase3_legacy_backfill"))
		Expect(forward).To(ContainSubstring("FROM forensic.evidence_items evidence"))
		Expect(forward).To(ContainSubstring("FROM forensic.kb_collection_assets assets"))
		Expect(forward).To(ContainSubstring("FROM forensic.records_ingest_jobs jobs"))
		Expect(forward).To(ContainSubstring("legacy_snapshot"))
		Expect(forward).NotTo(MatchRegexp(`(?im)^\s*(DROP\s+TABLE|TRUNCATE|DELETE\s+FROM)\s+`))
		Expect(forward).NotTo(ContainSubstring("ON DELETE CASCADE"))
		Expect(strings.TrimSpace(forward)).To(HavePrefix("-- Phase 3:"))
		Expect(forward).To(ContainSubstring("BEGIN;"))
		Expect(strings.TrimSpace(forward)).To(HaveSuffix("COMMIT;"))
	})

	It("keeps new registrations and ingest status changes synchronized", func() {
		Expect(forward).To(ContainSubstring("sync_new_evidence_control_plane"))
		Expect(forward).To(ContainSubstring("sync_kb_asset_source_link"))
		Expect(forward).To(ContainSubstring("sync_ingest_job_processing_run"))
		Expect(forward).To(ContainSubstring("sync_evidence_status_custody"))
		Expect(forward).To(ContainSubstring("ON CONFLICT (tenant_id, collection_id, content_sha256, storage_uri)"))
		Expect(forward).To(ContainSubstring("ON CONFLICT (run_id) DO UPDATE"))
		Expect(forward).To(ContainSubstring("phase3_native_registration"))
		Expect(idempotencyVerify).To(ContainSubstring("phase3_control_plane_idempotency_passed"))
		Expect(idempotencyVerify).To(ContainSubstring("native source-link count drifted"))
	})

	It("promotes verified content-addressed receipts without trusting inconsistent metadata", func() {
		Expect(forward).To(ContainSubstring("forensic_spool_content_addressed"))
		Expect(forward).To(ContainSubstring("sha256-full-read-after-retain"))
		Expect(forward).To(ContainSubstring("storage_receipt_uri"))
		Expect(forward).To(ContainSubstring("write_once = forensic.evidence_storage_objects.write_once OR EXCLUDED.write_once"))
		Expect(preflight).To(ContainSubstring("unsupported storage layout"))
		Expect(preflight).To(ContainSubstring("content-addressed evidence item(s) have inconsistent storage identity metadata"))
		Expect(preflight).To(ContainSubstring("forensic-spool-receipt://sha256-scope-v1/"))
		Expect(verify).To(ContainSubstring("content-addressed storage object(s) are not verified write-once objects"))
	})

	It("gates destructive rollback and verifies every backfill relationship", func() {
		Expect(rollback).To(ContainSubstring("app.phase3_allow_destructive_rollback"))
		Expect(rollback).To(ContainSubstring("IS DISTINCT FROM 'true'"))
		Expect(rollback).To(ContainSubstring("DROP COLUMN IF EXISTS current_version_id"))
		Expect(rollback).To(ContainSubstring("BEGIN;"))
		Expect(strings.TrimSpace(rollback)).To(HaveSuffix("COMMIT;"))
		Expect(preflight).To(ContainSubstring("BEGIN TRANSACTION READ ONLY"))
		Expect(preflight).To(ContainSubstring("non-monotonic lifecycle times"))
		Expect(preflight).To(ContainSubstring("point across tenant/collection scope"))
		Expect(preflight).To(ContainSubstring("normalized evidence version ID collision"))
		Expect(preflight).To(ContainSubstring("phase3_control_plane_preflight_passed"))
		Expect(verify).To(ContainSubstring("evidence item(s) have no current version"))
		Expect(verify).To(ContainSubstring("KB asset(s) have no normalized source link"))
		Expect(verify).To(ContainSubstring("ingest job(s) have no processing run"))
		Expect(verify).To(ContainSubstring("custody event(s) have invalid hashes"))
		Expect(verify).To(ContainSubstring("expected 7 Phase 3 tenant policies"))
		Expect(rlsSmoke).To(ContainSubstring("NOBYPASSRLS"))
		Expect(rlsSmoke).To(ContainSubstring("SET ROLE phase3_rls_test"))
		Expect(rlsSmoke).To(ContainSubstring("RLS allowed a cross-tenant insert"))
		Expect(rlsSmoke).To(ContainSubstring("phase3_non_owner_rls_smoke_passed"))
		Expect(strings.TrimSpace(rlsSmoke)).To(HaveSuffix("ROLLBACK;"))
	})
})

func readMigrationContract(path string) string {
	data, err := os.ReadFile(path)
	Expect(err).NotTo(HaveOccurred())
	return strings.ReplaceAll(string(data), "\r\n", "\n")
}
