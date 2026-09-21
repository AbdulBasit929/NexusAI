package main

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("C database capability isolation", Ordered, func() {
	var admin, db *pgxpool.Pool
	ctx := context.Background()
	const evidence = "11111111-1111-4111-8111-111111111111"
	const other = "22222222-2222-4222-8222-222222222222"
	const current = "33333333-3333-4333-8333-333333333333"
	const stale = "44444444-4444-4444-8444-444444444444"
	const run = "55555555-5555-4555-8555-555555555555"
	BeforeAll(func() {
		dsn := os.Getenv("NXB21_C_TEST_DATABASE_URL")
		if dsn == "" {
			Skip("requires isolated nxb21_c_test database")
		}
		cfg, err := pgxpool.ParseConfig(dsn)
		Expect(err).NotTo(HaveOccurred())
		Expect(cfg.ConnConfig.Database).To(Equal("nxb21_c_test"))
		admin, err = pgxpool.NewWithConfig(ctx, cfg)
		Expect(err).NotTo(HaveOccurred())
		_, err = admin.Exec(ctx, `CREATE SCHEMA forensic;
CREATE TABLE forensic.evidence_items(tenant_id text,collection_id text,case_id text,evidence_id uuid,current_version_id uuid,extension text,kb_entry_ref text,records_batch_id uuid);
CREATE TABLE forensic.derived_artifacts(artifact_id uuid DEFAULT gen_random_uuid(),tenant_id text,collection_id text,evidence_id uuid,version_id uuid,run_id uuid,artifact_type text,processing_status text,metadata jsonb);
CREATE INDEX ON forensic.derived_artifacts(tenant_id,collection_id,evidence_id,version_id,artifact_type);
CREATE TABLE forensic.records_ingest_jobs(id uuid,tenant_id text,collection_id text,evidence_id uuid,status text,metadata jsonb);
CREATE TABLE forensic.records(tenant_id text,collection_id text,evidence_id uuid,batch_id uuid,record_type text);
ALTER TABLE forensic.evidence_items ADD COLUMN modality text DEFAULT 'audio', ADD COLUMN detected_type text DEFAULT 'audio';
CREATE ROLE nxb21_c_runtime NOLOGIN NOSUPERUSER NOBYPASSRLS;
GRANT USAGE ON SCHEMA forensic TO nxb21_c_runtime;
GRANT SELECT ON ALL TABLES IN SCHEMA forensic TO nxb21_c_runtime;
ALTER TABLE forensic.evidence_items ENABLE ROW LEVEL SECURITY;
ALTER TABLE forensic.derived_artifacts ENABLE ROW LEVEL SECURITY;
ALTER TABLE forensic.records_ingest_jobs ENABLE ROW LEVEL SECURITY;
ALTER TABLE forensic.records ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_scope ON forensic.evidence_items USING(tenant_id=current_setting('app.tenant_id',true));
CREATE POLICY tenant_scope ON forensic.derived_artifacts USING(tenant_id=current_setting('app.tenant_id',true));
CREATE POLICY tenant_scope ON forensic.records_ingest_jobs USING(tenant_id=current_setting('app.tenant_id',true));
CREATE POLICY tenant_scope ON forensic.records USING(tenant_id=current_setting('app.tenant_id',true));`)
		Expect(err).NotTo(HaveOccurred())
		cfg.AfterConnect = func(ctx context.Context, c *pgx.Conn) error {
			_, err := c.Exec(ctx, "SET ROLE nxb21_c_runtime")
			return err
		}
		db, err = pgxpool.NewWithConfig(ctx, cfg)
		Expect(err).NotTo(HaveOccurred())
	})
	AfterAll(func() {
		if db != nil {
			db.Close()
		}
		if admin != nil {
			_, err := admin.Exec(ctx, `DROP SCHEMA forensic CASCADE; DROP ROLE nxb21_c_runtime;`)
			Expect(err).NotTo(HaveOccurred())
			admin.Close()
		}
	})
	BeforeEach(func() {
		_, err := admin.Exec(ctx, `TRUNCATE forensic.evidence_items,forensic.derived_artifacts,forensic.records_ingest_jobs,forensic.records;
INSERT INTO forensic.evidence_items(tenant_id,collection_id,case_id,evidence_id,current_version_id,extension,kb_entry_ref,records_batch_id) VALUES('test','workspace','case-a','11111111-1111-4111-8111-111111111111','33333333-3333-4333-8333-333333333333','wav',NULL,NULL);`)
		Expect(err).NotTo(HaveOccurred())
	})
	artifact := func(tenant, collection, id, version, contract, runID string) {
		_, err := admin.Exec(ctx, `INSERT INTO forensic.derived_artifacts(tenant_id,collection_id,evidence_id,version_id,run_id,artifact_type,processing_status,metadata) VALUES($1,$2,$3,$4,NULLIF($5,'')::uuid,$6,'completed','{"observation":{"raw_text":"synthetic"}}')`, tenant, collection, id, version, runID, contract)
		Expect(err).NotTo(HaveOccurred())
	}
	states := func(selected string) map[string]CapabilityEvidenceStateV1 {
		refs, err := loadQueryCapabilityReferences()
		Expect(err).NotTo(HaveOccurred())
		start := time.Now()
		s, err := loadSemanticCapabilityStates(ctx, db, forensicScope{TenantID: "test", CollectionID: "workspace", CaseID: "case-a"}, selected, "", refs)
		Expect(err).NotTo(HaveOccurred())
		GinkgoWriter.Printf("C DB existence projection: %s\n", time.Since(start))
		return s
	}
	transcript := func(version string) {
		artifact("test", "workspace", evidence, version, "forensics.audio-timestamp-segment/v1", run)
	}
	zero := func() {
		artifact("test", "workspace", evidence, current, "forensics.audio-observation/v1", run)
		_, err := admin.Exec(ctx, `INSERT INTO forensic.records_ingest_jobs VALUES($1,'test','workspace',$2,'completed','{"media_result_states":{"asr":"COMPLETE_ZERO_RESULTS"}}')`, run, evidence)
		Expect(err).NotTo(HaveOccurred())
	}
	It("queries current retained transcript without an available processing role", func() {
		transcript(current)
		s := states(evidence)["transcript.phrase"]
		Expect(s.Results).To(BeTrue())
		r := projectSemanticReadiness(cReference("transcript.phrase"), s, cContext())
		Expect(r.QueryExistingResults).To(BeTrue())
		Expect(r.ProcessNewInput).To(BeFalse())
	})
	It("does not infer transcript from ordinary audio metadata", func() {
		artifact("test", "workspace", evidence, current, "forensics.audio-observation/v1", run)
		s := states(evidence)["transcript.phrase"]
		Expect(s.Results).To(BeFalse())
		Expect(s.CompleteZero).To(BeFalse())
		Expect(projectSemanticReadiness(cReference("transcript.phrase"), s, cContext()).ResultState).To(Equal("NOT_RUN"))
	})
	It("rejects stale artifacts and missing run lineage", func() {
		transcript(stale)
		artifact("test", "workspace", evidence, current, "forensics.audio-timestamp-segment/v1", "")
		Expect(states(evidence)["transcript.phrase"].Results).To(BeFalse())
	})
	It("keeps tenant case and collection predicates independent", func() {
		for _, v := range [][3]string{{"other", "workspace", "case-a"}, {"test", "workspace", "case-b"}, {"test", "other", "case-a"}} {
			_, err := admin.Exec(ctx, `INSERT INTO forensic.evidence_items(tenant_id,collection_id,case_id,evidence_id,current_version_id,extension,kb_entry_ref,records_batch_id) VALUES($1,$2,$3,$4,$5,'wav',NULL,NULL)`, v[0], v[1], v[2], other, current)
			Expect(err).NotTo(HaveOccurred())
			artifact(v[0], v[1], other, current, "forensics.audio-timestamp-segment/v1", run)
		}
		Expect(states("")["transcript.phrase"].Results).To(BeFalse())
	})
	It("keeps workspace readiness independent of the selected source", func() {
		transcript(current)
		_, err := admin.Exec(ctx, `INSERT INTO forensic.evidence_items(tenant_id,collection_id,case_id,evidence_id,current_version_id,extension,kb_entry_ref,records_batch_id) VALUES('test','workspace','case-a',$1,$2,'wav',NULL,NULL)`, other, current)
		Expect(err).NotTo(HaveOccurred())
		Expect(states(other)["transcript.phrase"].Results).To(BeFalse())
		Expect(states("")["transcript.phrase"].Results).To(BeTrue())
	})
	It("proves complete zero through current artifact run and job", func() {
		zero()
		s := states(evidence)["transcript.phrase"]
		Expect(s.CompleteZero).To(BeTrue())
		Expect(projectSemanticReadiness(cReference("transcript.phrase"), s, cContext()).ResultState).To(Equal("COMPLETE_ZERO_RESULTS"))
	})
	It("does not inherit stale zero receipts", func() {
		zero()
		_, err := admin.Exec(ctx, `UPDATE forensic.evidence_items SET current_version_id=$1`, stale)
		Expect(err).NotTo(HaveOccurred())
		Expect(states(evidence)["transcript.phrase"].CompleteZero).To(BeFalse())
	})
	It("does not call a partially processed workspace complete zero", func() {
		zero()
		_, err := admin.Exec(ctx, `INSERT INTO forensic.evidence_items(tenant_id,collection_id,case_id,evidence_id,current_version_id,extension,kb_entry_ref,records_batch_id) VALUES('test','workspace','case-a',$1,$2,'wav',NULL,NULL)`, other, current)
		Expect(err).NotTo(HaveOccurred())
		s := states("")["transcript.phrase"]
		Expect(s.CompleteZero).To(BeFalse())
		Expect(projectSemanticReadiness(cReference("transcript.phrase"), s, cContext()).ResultState).To(Equal("INCOMPLETE"))
	})
	It("distinguishes failed current processing while preserving valid current observations", func() {
		_, err := admin.Exec(ctx, `INSERT INTO forensic.records_ingest_jobs VALUES($1,'test','workspace',$2,'dead_letter',jsonb_build_object('version_id',$3::text))`, run, evidence, current)
		Expect(err).NotTo(HaveOccurred())
		s := states(evidence)["transcript.phrase"]
		Expect(projectSemanticReadiness(cReference("transcript.phrase"), s, cContext()).ResultState).To(Equal("FAILED"))
		transcript(current)
		Expect(states(evidence)["transcript.phrase"].Results).To(BeTrue())
	})
	It("keeps OCR raw and Roman derivative artifact readiness independent", func() {
		_, err := admin.Exec(ctx, `UPDATE forensic.evidence_items SET extension='png'`)
		Expect(err).NotTo(HaveOccurred())
		artifact("test", "workspace", evidence, current, "forensics.image-ocr-observation/v1", run)
		Expect(states(evidence)["ocr.phrase"].Results).To(BeTrue())
		Expect(states(evidence)["transcript.phrase"].Results).To(BeFalse())
		_, err = admin.Exec(ctx, `UPDATE forensic.derived_artifacts SET metadata='{"observation":{"normalized_text":"synthetic"}}'`)
		Expect(err).NotTo(HaveOccurred())
		Expect(states(evidence)["ocr.phrase"].Results).To(BeFalse())
		_, err = admin.Exec(ctx, `UPDATE forensic.evidence_items SET extension='wav'`)
		Expect(err).NotTo(HaveOccurred())
		artifact("test", "workspace", evidence, current, "forensics.audio-roman-urdu-segment/v1", run)
		Expect(states(evidence)["roman_urdu.phrase"].Results).To(BeTrue())
		Expect(states(evidence)["transcript.phrase"].Results).To(BeFalse())
	})
	It("requires records from the current source batch", func() {
		_, err := admin.Exec(ctx, `UPDATE forensic.evidence_items SET extension='csv',records_batch_id=$1`, run)
		Expect(err).NotTo(HaveOccurred())
		_, err = admin.Exec(ctx, `INSERT INTO forensic.records VALUES('test','workspace',$1,$2,'cdr')`, evidence, stale)
		Expect(err).NotTo(HaveOccurred())
		Expect(states(evidence)["cdr.frequent_contacts"].Results).To(BeFalse())
		_, err = admin.Exec(ctx, `UPDATE forensic.records SET batch_id=$1`, run)
		Expect(err).NotTo(HaveOccurred())
		Expect(states(evidence)["cdr.frequent_contacts"].Results).To(BeTrue())
		coverage, err := loadCapabilityCoverage(ctx, db, forensicScope{TenantID: "test", CollectionID: "workspace", CaseID: "case-a"}, evidence, "")
		Expect(err).NotTo(HaveOccurred())
		Expect(coverage.TotalIndexedRecords).To(Equal(int64(1)))
	})
	It("rejects cross-case scope headers before reading any capability state", func() {
		r := httptest.NewRequest("GET", "/query/capabilities?collection_id=workspace&case_id=case-b", nil)
		r = r.WithContext(context.WithValue(r.Context(), forensicPrincipalContextKey{}, forensicPrincipal{Authenticated: true, TenantID: "test", CollectionID: "workspace", CaseID: "case-a"}))
		w := httptest.NewRecorder()
		forensicCapabilitiesHandler(db).ServeHTTP(w, r)
		Expect(w.Code).To(Equal(403))
		var body map[string]any
		Expect(json.Unmarshal(w.Body.Bytes(), &body)).To(Succeed())
		Expect(body).NotTo(HaveKey("query_capability_readiness"))
	})
	It("serves the complete API snapshot with selected version and case boundaries", func() {
		transcript(current)
		r := httptest.NewRequest("GET", "/query/capabilities?collection_id=workspace&case_id=case-a&tenant_id=test&evidence_id="+evidence+"&evidence_version_id="+current, nil)
		w := httptest.NewRecorder()
		forensicCapabilitiesHandler(db).ServeHTTP(w, r)
		Expect(w.Code).To(Equal(200), w.Body.String())
		var body struct {
			Readiness []SemanticCapabilityReadinessV1 `json:"query_capability_readiness"`
			Families  []forensicFamilyCapability      `json:"families"`
		}
		Expect(json.Unmarshal(w.Body.Bytes(), &body)).To(Succeed())
		for _, v := range body.Readiness {
			if v.CapabilityID == "transcript.phrase" {
				Expect(v.QueryExistingResults).To(BeTrue())
			}
		}
		for _, f := range body.Families {
			if f.ID == "tts_artifacts" {
				Expect(f.Availability).To(Equal("unavailable"))
			}
		}
	})
	It("requires both current image ANPR artifacts and canonical rows for the mixed-source executor", func() {
		_, err := admin.Exec(ctx, `UPDATE forensic.evidence_items SET extension='png',records_batch_id=$1`, run)
		Expect(err).NotTo(HaveOccurred())
		artifact("test", "workspace", evidence, current, "forensics.anpr-observation/v1", run)
		Expect(states(evidence)["image.plate"].Results).To(BeFalse())
		_, err = admin.Exec(ctx, `INSERT INTO forensic.records VALUES('test','workspace',$1,$2,'anpr')`, evidence, run)
		Expect(err).NotTo(HaveOccurred())
		s := states(evidence)["image.plate"]
		Expect(s.Results).To(BeTrue())
		c := cReference("image.plate")
		projection := cContext()
		Expect(projectSemanticReadiness(c, s, projection).QueryExistingResults).To(BeFalse())
		projection.ScopeMode = "selected_evidence"
		r := projectSemanticReadiness(c, s, projection)
		Expect(r.QueryExistingResults).To(BeTrue())
		Expect(r.RequiredParameters).To(ContainElement("evidence_id"))
	})
})
