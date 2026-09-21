package main

import (
	"path/filepath"
	"regexp"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Forensic non-owner runtime boundary", func() {
	var preflight, forward, verify, grants, rlsProof, apiQueueSource string

	BeforeEach(func() {
		migrationRoot := filepath.Join("..", "..", "db", "forensic_records")
		preflight = readMigrationContract(filepath.Join(migrationRoot, "010_runtime_rls_policies.preflight.sql"))
		forward = readMigrationContract(filepath.Join(migrationRoot, "010_runtime_rls_policies.sql"))
		verify = readMigrationContract(filepath.Join(migrationRoot, "010_runtime_rls_policies.verify.sql"))
		grants = readMigrationContract(filepath.Join(migrationRoot, "runtime_role.grants.sql"))
		rlsProof = readMigrationContract(filepath.Join(migrationRoot, "runtime_role.rls.verify.sql"))
		apiQueueSource = readMigrationContract("queue_lifecycle.go")
	})

	It("closes every known Phase 1 policy gap transactionally", func() {
		for _, table := range []string{"records_ingest_jobs", "records_ingest_errors", "records_audit_log"} {
			Expect(forward).To(ContainSubstring("ALTER TABLE forensic." + table + " ENABLE ROW LEVEL SECURITY"))
			Expect(forward).To(MatchRegexp(`(?s)CREATE POLICY tenant_isolation_` + regexp.QuoteMeta(table) + `.*USING.*WITH CHECK`))
		}
		Expect(preflight).To(ContainSubstring("BEGIN TRANSACTION READ ONLY"))
		Expect(preflight).To(ContainSubstring("migrations 008 and 009 must be applied"))
		Expect(verify).To(ContainSubstring("RLS-enabled forensic tables without policies"))
		Expect(forward).To(ContainSubstring("ALTER VIEW forensic.cdr_frequent_contacts SET (security_invoker = true)"))
		Expect(forward).To(ContainSubstring("ALTER VIEW forensic.entity_activity_summary SET (security_invoker = true)"))
		Expect(verify).To(ContainSubstring("forensic query views are not security_invoker"))
		Expect(forward).NotTo(MatchRegexp(`(?im)^\s*(DELETE\s+FROM|TRUNCATE|DROP\s+TABLE)\s+`))
		Expect(strings.TrimSpace(forward)).To(HaveSuffix("COMMIT;"))
	})

	It("keeps the runtime role non-owner and denies destructive table operations", func() {
		Expect(grants).To(ContainSubstring("NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOINHERIT NOBYPASSRLS"))
		Expect(grants).To(ContainSubstring("REVOKE CREATE ON SCHEMA forensic FROM PUBLIC"))
		Expect(grants).To(ContainSubstring("GRANT CONNECT, TEMPORARY ON DATABASE localrecall"))
		Expect(grants).To(ContainSubstring("GRANT SELECT ON ALL TABLES IN SCHEMA forensic"))
		Expect(grants).To(MatchRegexp(`(?s)GRANT INSERT ON.*forensic\.derived_artifacts.*TO forensic_runtime`))
		Expect(grants).NotTo(MatchRegexp(`(?i)GRANT\s+(DELETE|TRUNCATE|CREATE)`))
		Expect(grants).NotTo(MatchRegexp(`(?i)\sBYPASSRLS(?:\s|;)`))
	})

	It("makes API startup fail closed until the runtime RLS contract exists", func() {
		Expect(apiQueueSource).To(ContainSubstring("tenant_isolation_records_ingest_jobs"))
		Expect(apiQueueSource).To(ContainSubstring("security_invoker=true"))
		Expect(apiQueueSource).To(ContainSubstring("migrations 009 and 010 before starting the API"))
	})

	It("proves same-tenant access and cross-tenant denial without retaining probes", func() {
		Expect(rlsProof).To(ContainSubstring("SET LOCAL ROLE forensic_runtime"))
		Expect(rlsProof).To(ContainSubstring("same-tenant runtime read/write proof failed"))
		Expect(rlsProof).To(ContainSubstring("cross-tenant runtime write unexpectedly succeeded"))
		Expect(rlsProof).To(ContainSubstring("synthetic-runtime-probe"))
		Expect(strings.TrimSpace(rlsProof)).To(HaveSuffix("ROLLBACK;"))
	})
})
