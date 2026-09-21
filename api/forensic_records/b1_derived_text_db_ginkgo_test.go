package main

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudler/LocalAI/pkg/forensictext"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("B1 database derived text", Ordered, func() {
	var admin, db *pgxpool.Pool
	ctx := context.Background()
	const source = "11111111-1111-4111-8111-111111111111"
	const version = "22222222-2222-4222-8222-222222222222"
	const bulk = "33333333-3333-4333-8333-333333333333"
	const capped = "44444444-4444-4444-8444-444444444444"
	BeforeAll(func() {
		dsn := os.Getenv("NXB21_TEST_DATABASE_URL")
		if dsn == "" {
			Skip("requires isolated nxb21_b1_test database")
		}
		cfg, err := pgxpool.ParseConfig(dsn)
		Expect(err).NotTo(HaveOccurred())
		Expect(cfg.ConnConfig.Database).To(Equal("nxb21_b1_test"))
		admin, err = pgxpool.NewWithConfig(ctx, cfg)
		Expect(err).NotTo(HaveOccurred())
		_, err = admin.Exec(ctx, `CREATE SCHEMA forensic;
CREATE TABLE forensic.evidence_items(tenant_id text,collection_id text,evidence_id uuid,current_version_id uuid,original_filename text);
CREATE TABLE forensic.derived_artifacts(artifact_id uuid PRIMARY KEY,tenant_id text,collection_id text,evidence_id uuid,version_id uuid,run_id uuid,parent_artifact_id uuid,artifact_type text,processing_status text,citation_locator jsonb,metadata jsonb,created_at timestamptz DEFAULT now());
CREATE INDEX derived_artifacts_evidence_idx ON forensic.derived_artifacts(tenant_id,collection_id,evidence_id,version_id,artifact_type);
CREATE ROLE nxb21_b1_runtime NOLOGIN NOSUPERUSER NOBYPASSRLS;
GRANT USAGE ON SCHEMA forensic TO nxb21_b1_runtime;
GRANT SELECT ON ALL TABLES IN SCHEMA forensic TO nxb21_b1_runtime;
ALTER TABLE forensic.evidence_items ENABLE ROW LEVEL SECURITY;
ALTER TABLE forensic.derived_artifacts ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_scope ON forensic.evidence_items USING(tenant_id=current_setting('app.tenant_id',true));
CREATE POLICY tenant_scope ON forensic.derived_artifacts USING(tenant_id=current_setting('app.tenant_id',true));`)
		Expect(err).NotTo(HaveOccurred())
		for _, pair := range [][2]string{{"test", "case"}, {"test", "other"}, {"other", "case"}} {
			_, err = admin.Exec(ctx, `INSERT INTO forensic.evidence_items VALUES($1,$2,$3,$4,'synthetic.wav')`, pair[0], pair[1], source, version)
			Expect(err).NotTo(HaveOccurred())
		}
		_, err = admin.Exec(ctx, `INSERT INTO forensic.evidence_items VALUES('test','case',$1,$3,'bulk.txt'),('test','case',$2,$3,'capped.txt')`, bulk, capped, version)
		Expect(err).NotTo(HaveOccurred())
		insert := func(tenant, collection, ver, contract, text string, start, end float64) {
			metadata, _ := json.Marshal(map[string]any{"observation_id": "synthetic-observation", "observation": map[string]any{"text": text, "raw_text": text, "language": "ur"}})
			locator, _ := json.Marshal(map[string]any{"start_seconds": start, "end_seconds": end, "page": 2, "bbox": []int{1, 2, 3, 4}})
			_, err := admin.Exec(ctx, `INSERT INTO forensic.derived_artifacts VALUES(gen_random_uuid(),$1,$2,$3,$4,'55555555-5555-4555-8555-555555555555',NULL,$5,'completed',$6,$7,now())`, tenant, collection, source, ver, contract, locator, metadata)
			Expect(err).NotTo(HaveOccurred())
		}
		insert("test", "case", version, "forensics.audio-timestamp-segment/v1", "یہ نئی", 0, 1)
		insert("test", "case", version, "forensics.audio-timestamp-segment/v1", "رپورٹ ہے", 1, 2)
		insert("test", "case", version, "forensics.image-ocr-observation/v1", "Annual Report", 0, 1)
		for _, pair := range [][2]string{{"test", "other"}, {"other", "case"}} {
			insert(pair[0], pair[1], version, "forensics.audio-timestamp-segment/v1", "unauthorized phrase", 0, 1)
		}
		insert("test", "case", "66666666-6666-4666-8666-666666666666", "forensics.audio-timestamp-segment/v1", "stale phrase", 0, 1)
		for id, count := range map[string]int{bulk: 510, capped: 10005} {
			_, err = admin.Exec(ctx, `INSERT INTO forensic.derived_artifacts SELECT ('00000000-0000-4000-8000-'||lpad((n+$4)::text,12,'0'))::uuid,'test','case',$1,$2,'55555555-5555-4555-8555-555555555555',NULL,'forensics.document-native-text-passage/v1','completed','{"page":1}'::jsonb,jsonb_build_object('text',CASE WHEN n=510 THEN 'needle beyond page' ELSE 'filler content' END),now() FROM generate_series(1,$3) n`, id, version, count, map[string]int{bulk: 0, capped: 100000}[id])
			Expect(err).NotTo(HaveOccurred())
		}
		cfg.AfterConnect = func(ctx context.Context, c *pgx.Conn) error {
			_, err := c.Exec(ctx, "SET ROLE nxb21_b1_runtime")
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
			_, err := admin.Exec(ctx, `DROP SCHEMA forensic CASCADE; DROP ROLE nxb21_b1_runtime;`)
			Expect(err).NotTo(HaveOccurred())
			admin.Close()
		}
	})
	request := func(literal string) hybridQueryRequest {
		req := b1Request(literal)
		req.EvidenceID = source
		req.EvidenceVersionID = version
		return req
	}
	It("executes current-version and tenant policies and agrees with an independent SQL substring oracle", func() {
		var oracle bool
		Expect(admin.QueryRow(ctx, `SELECT position('نئی رپورٹ' in string_agg(a.metadata#>>'{observation,text}',' ' ORDER BY (a.citation_locator->>'start_seconds')::numeric))>0 FROM forensic.derived_artifacts a JOIN forensic.evidence_items e ON (e.tenant_id,e.collection_id,e.evidence_id,e.current_version_id)=(a.tenant_id,a.collection_id,a.evidence_id,a.version_id) WHERE a.tenant_id='test' AND a.collection_id='case' AND a.artifact_type='forensics.audio-timestamp-segment/v1' GROUP BY a.run_id`).Scan(&oracle)).To(Succeed())
		Expect(oracle).To(BeTrue())
		start := time.Now()
		result, err := derivedTextEvidence(ctx, db, request("نئی رپورٹ"))
		Expect(err).NotTo(HaveOccurred())
		Expect(evidenceResults(result)).To(HaveLen(2))
		Expect(result["result_state"]).To(Equal("COMPLETE_RESULTS"))
		GinkgoWriter.Printf("B1 DB cross-segment latency_ms=%d\n", time.Since(start).Milliseconds())
		for _, literal := range []string{"unauthorized phrase", "stale phrase", "absent phrase"} {
			result, err = derivedTextEvidence(ctx, db, request(literal))
			Expect(err).NotTo(HaveOccurred())
			Expect(evidenceResults(result)).To(BeEmpty())
			Expect(result["search_completeness"]).To(Equal("EXHAUSTIVE"))
		}
		rows, err := queryRows(ctx, db, "test", `SELECT count(*) AS count FROM forensic.derived_artifacts WHERE tenant_id='other'`)
		Expect(err).NotTo(HaveOccurred())
		Expect(stringValueAny(rows[0]["count"])).To(Equal("0"))
	})
	It("finds beyond the first candidate page and proves exhaustive zero", func() {
		req := request("needle beyond page")
		req.Template = "document_search"
		req.EvidenceID = bulk
		result, err := derivedTextEvidence(ctx, db, req)
		Expect(err).NotTo(HaveOccurred())
		Expect(evidenceResults(result)).To(HaveLen(1))
		Expect(result["candidate_pages"]).To(Equal(2))
		Expect(result["search_completeness"]).To(Equal("EXHAUSTIVE"))
		req.TextQuery.LiteralText = "absent phrase"
		result, err = derivedTextEvidence(ctx, db, req)
		Expect(err).NotTo(HaveOccurred())
		Expect(result["result_state"]).To(Equal("NO_EXACT_MATCH"))
	})
	It("executes workspace audio and OCR positives and exhaustive audio absence", func() {
		req := request("نئی رپورٹ")
		req.EvidenceID = ""
		req.EvidenceVersionID = ""
		result, err := derivedTextEvidence(ctx, db, req)
		Expect(err).NotTo(HaveOccurred())
		Expect(evidenceResults(result)).To(HaveLen(2))
		Expect(derivedTranscriptResultState(req, result)).To(Equal("COMPLETE_RESULTS"))
		req.TextQuery.LiteralText = "absent phrase"
		result, err = derivedTextEvidence(ctx, db, req)
		Expect(err).NotTo(HaveOccurred())
		Expect(result["result_state"]).To(Equal("NO_EXACT_MATCH"))
		Expect(result["search_completeness"]).To(Equal("EXHAUSTIVE"))
		req.Template = "image_ocr_search"
		req.TextQuery.LiteralText = "Annual Report"
		result, err = derivedTextEvidence(ctx, db, req)
		Expect(err).NotTo(HaveOccurred())
		Expect(evidenceResults(result)).To(HaveLen(1))
	})
	It("reports budget exhaustion for both zero and positive show-all searches", func() {
		for _, literal := range []string{"absent phrase", "needle beyond page"} {
			req := request(literal)
			req.Query = "Show all occurrences of the supplied literal"
			req.Template = "document_search"
			req.EvidenceID = capped
			result, err := derivedTextEvidence(ctx, db, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(result["result_state"]).To(Equal("SEARCH_INCOMPLETE"))
			Expect(result["search_completeness"]).To(Equal("INCOMPLETE"))
			Expect(result["candidates_inspected"]).To(Equal(10000))
		}
	})
	It("uses current raw OCR metadata and preserves region citations", func() {
		req := request("Annual Report")
		req.Template = "image_ocr_search"
		req.TextQuery.MatchSemantic = forensictext.ExactPhrase
		result, err := derivedTextEvidence(ctx, db, req)
		Expect(err).NotTo(HaveOccurred())
		Expect(evidenceResults(result)).To(HaveLen(1))
		Expect(evidenceResults(result)[0]["metadata"].(map[string]any)["citation_locator"].(map[string]any)["bbox"]).To(HaveLen(4))
	})
	It("propagates actual execution failure without successful results", func() {
		dead, cancel := context.WithCancel(ctx)
		cancel()
		result, err := derivedTextEvidence(dead, db, request("نئی رپورٹ"))
		Expect(err).To(HaveOccurred())
		Expect(result).To(BeNil())
	})
})
