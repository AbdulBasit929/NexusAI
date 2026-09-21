package main

import (
	"context"
	"encoding/json"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// These regressions execute production SQL under the actual tenant-policy
// shape on Timescale, with synthetic observations and no retained evidence.
var _ = Describe("Four P1 database contracts", Ordered, func() {
	var admin, db *pgxpool.Pool
	ctx := context.Background()
	evidence, version, stale := uuid.NewString(), uuid.NewString(), uuid.NewString()
	imageID := uuid.NewString()
	raw1, raw2, group := uuid.NewString(), uuid.NewString(), uuid.NewString()
	BeforeAll(func() {
		dsn := os.Getenv("LOCALAI_TEST_DATABASE_URL")
		if dsn == "" {
			Skip("requires explicitly isolated nxmmr_p1_test Timescale database")
		}
		config, err := pgxpool.ParseConfig(dsn)
		Expect(err).NotTo(HaveOccurred())
		Expect(config.ConnConfig.Database).To(Equal("nxmmr_p1_test"))
		admin, err = pgxpool.NewWithConfig(ctx, config)
		Expect(err).NotTo(HaveOccurred())
		var name, timescale string
		Expect(admin.QueryRow(ctx, "SELECT current_database()").Scan(&name)).To(Succeed())
		Expect(name).To(Equal("nxmmr_p1_test"))
		Expect(admin.QueryRow(ctx, "SELECT extversion FROM pg_extension WHERE extname='timescaledb'").Scan(&timescale)).To(Succeed())
		Expect(timescale).To(Equal("2.28.3"))
		_, err = admin.Exec(ctx, `CREATE SCHEMA forensic;
CREATE TABLE forensic.evidence_items(tenant_id text,collection_id text,evidence_id uuid,current_version_id uuid, original_filename text,modality text,content_type text,detected_type text,processing_status text,metadata jsonb);
CREATE TABLE forensic.derived_artifacts(artifact_id uuid PRIMARY KEY,tenant_id text,collection_id text,evidence_id uuid,version_id uuid,run_id uuid DEFAULT gen_random_uuid(),parent_artifact_id uuid,artifact_type text,processing_status text,confidence numeric(7,6),citation_locator jsonb,metadata jsonb,warnings jsonb DEFAULT '[]',created_at timestamptz DEFAULT now());
CREATE INDEX derived_artifacts_evidence_idx ON forensic.derived_artifacts(tenant_id,collection_id,evidence_id,version_id,artifact_type);
CREATE ROLE nxmmr_p1_runtime NOLOGIN NOSUPERUSER NOBYPASSRLS;
GRANT USAGE ON SCHEMA forensic TO nxmmr_p1_runtime;
GRANT SELECT ON ALL TABLES IN SCHEMA forensic TO nxmmr_p1_runtime;
ALTER TABLE forensic.evidence_items ENABLE ROW LEVEL SECURITY;
ALTER TABLE forensic.derived_artifacts ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_scope ON forensic.evidence_items USING(tenant_id=current_setting('app.tenant_id',true));
CREATE POLICY tenant_scope ON forensic.derived_artifacts USING(tenant_id=current_setting('app.tenant_id',true));`)
		Expect(err).NotTo(HaveOccurred())
		for _, scope := range [][2]string{{"test", "case"}, {"test", "other"}, {"other", "case"}} {
			_, err = admin.Exec(ctx, `INSERT INTO forensic.evidence_items VALUES($1,$2,$3,$4,'synthetic-video.mp4','video','video/mp4','video','completed','{"media_processing":{"metadata":{"result_states":{"video_anpr":"COMPLETE_RESULTS"}}}}')`, scope[0], scope[1], evidence, version)
			Expect(err).NotTo(HaveOccurred())
			_, err = admin.Exec(ctx, `INSERT INTO forensic.evidence_items VALUES($1,$2,$3,$4,'synthetic-image.png','image','image/png','image','completed','{}')`, scope[0], scope[1], imageID, version)
			Expect(err).NotTo(HaveOccurred())
		}
		insert := func(id, tenant, collection, ver, contract string, obs, locator map[string]any) {
			sourceID := evidence
			if contract == "forensics.image-ocr-observation/v1" {
				sourceID = imageID
			}
			metadata, _ := json.Marshal(map[string]any{"observation": obs, "observation_id": id})
			loc, _ := json.Marshal(locator)
			_, err := admin.Exec(ctx, `INSERT INTO forensic.derived_artifacts(artifact_id,tenant_id,collection_id,evidence_id,version_id,artifact_type,processing_status,confidence,metadata,citation_locator) VALUES($1,$2,$3,$4,$5,$6,'completed',0.9,$7,$8)`, id, tenant, collection, sourceID, ver, contract, metadata, loc)
			Expect(err).NotTo(HaveOccurred())
		}
		for i, id := range []string{raw1, raw2} {
			insert(id, "test", "case", version, "forensics.anpr-observation/v1", map[string]any{"normalized_plate_text": "AB12CDE", "raw_plate_text": "AB12 CDE"}, map[string]any{"timestamp_seconds": i + 1, "frame_number": (i + 1) * 60})
		}
		insert(group, "test", "case", version, videoANPRGroupContract, map[string]any{"normalized_plate_text": "AB12CDF", "all_observation_ids": []string{raw1, raw2}, "best_observation_id": raw1, "sightings_count": 2, "first_seen_seconds": 1, "last_seen_seconds": 2}, map[string]any{"timestamp_seconds": 1})
		for _, scope := range [][3]string{{"test", "case", version}, {"test", "other", version}, {"other", "case", version}, {"test", "case", stale}} {
			phrase := "Investigation Workspace"
			if scope[1] != "case" || scope[0] != "test" || scope[2] == stale {
				phrase = "ZZ99ZZZ"
			}
			insert(uuid.NewString(), scope[0], scope[1], scope[2], "forensics.image-ocr-observation/v1", map[string]any{"raw_text": phrase, "normalized_text": strings.ToLower(phrase), "bbox": []int{1, 2, 30, 40}}, map[string]any{"bbox": []int{1, 2, 30, 40}})
		}
		// Match the retained index cardinality sufficiently to exercise planner choice.
		insert(uuid.NewString(), "test", "case", version, "forensics.image-ocr-observation/v1", map[string]any{"raw_text": "", "normalized_text": "Normalized fallback"}, map[string]any{"bbox": []int{3, 4, 10, 20}})
		_, err = admin.Exec(ctx, `INSERT INTO forensic.derived_artifacts(artifact_id,tenant_id,collection_id,evidence_id,version_id,artifact_type,processing_status,metadata,citation_locator) SELECT gen_random_uuid(),'test','case',$1,$2,'synthetic.unrelated/'||(n%6),'completed','{}','{}' FROM generate_series(1,800) n;`, evidence, version)
		Expect(err).NotTo(HaveOccurred())
		_, err = admin.Exec(ctx, "ANALYZE forensic.derived_artifacts")
		Expect(err).NotTo(HaveOccurred())
		config.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
			_, err := conn.Exec(ctx, "SET ROLE nxmmr_p1_runtime")
			return err
		}
		db, err = pgxpool.NewWithConfig(ctx, config)
		Expect(err).NotTo(HaveOccurred())
	})
	AfterAll(func() {
		if db != nil {
			db.Close()
		}
		if admin != nil {
			var name string
			Expect(admin.QueryRow(ctx, "SELECT current_database()").Scan(&name)).To(Succeed())
			Expect(name).To(Equal("nxmmr_p1_test"))
			_, err := admin.Exec(ctx, "DROP SCHEMA IF EXISTS forensic CASCADE; DROP ROLE IF EXISTS nxmmr_p1_runtime")
			Expect(err).NotTo(HaveOccurred())
			admin.Close()
		}
	})
	It("executes scope discovery under RLS and returns exact raw readings without relabeling the selected group", func() {
		req := hybridQueryRequest{TenantID: "test", CollectionID: "case", EvidenceID: evidence, Template: "video_anpr_grouped_timeline", Plate: "AB12CDE", Limit: 10}
		Expect(bindAuthoritativeEvidenceScope(ctx, db, &req)).To(Succeed())
		rows, err := queryRows(ctx, db, "test", "EXPLAIN (ANALYZE, COSTS OFF) "+currentEvidenceResultFamiliesSQL, "test", "case", evidence, version)
		Expect(err).NotTo(HaveOccurred())
		Expect(rows).NotTo(BeEmpty())
		GinkgoWriter.Printf("RLS_GROUP_BY_PLAN=%v\n", rows)
		result, err := videoANPRGroupedTimeline(ctx, db, req)
		Expect(err).NotTo(HaveOccurred())
		readings := result["video_anpr_grouped_timeline"].([]map[string]any)
		Expect(readings).To(HaveLen(2))
		for i, row := range readings {
			Expect(row["normalized_plate_text"]).To(Equal("AB12CDE"))
			Expect(row["group_selected_plate_text"]).To(Equal("AB12CDF"))
			Expect(row["match_kind"]).To(Equal("raw_plate_observation"))
			Expect(numericFloat(row["first_seen_seconds"])).To(Equal(float64(i + 1)))
			Expect(row["citation_locator"]).NotTo(BeNil())
			Expect(stringValueAny(row["citation_ref"])).To(ContainSubstring(evidence))
		}
		req.Plate = "QZ99XYZ"
		result, err = videoANPRGroupedTimeline(ctx, db, req)
		Expect(err).NotTo(HaveOccurred())
		Expect(result["row_count"]).To(Equal(0))
		Expect(result["status"]).To(Equal("no_match_for_filter"))
		req.Plate = "AB12CDF"
		result, err = videoANPRGroupedTimeline(ctx, db, req)
		Expect(err).NotTo(HaveOccurred())
		Expect(result["row_count"]).To(Equal(1))
	})
	It("searches actual raw OCR storage with current-version, tenant and case isolation", func() {
		req := hybridQueryRequest{TenantID: "test", CollectionID: "case", EvidenceID: imageID, EvidenceVersionID: version, Template: "image_ocr_search", ExactTerm: "Investigation Workspace", MaxKBResults: 10}
		Expect(bindAuthoritativeEvidenceScope(ctx, db, &req)).To(Succeed())
		result, err := derivedTextEvidence(ctx, db, req)
		Expect(err).NotTo(HaveOccurred())
		hits := evidenceResults(result)
		Expect(hits).To(HaveLen(1))
		Expect(hits[0]["content"]).To(Equal("Investigation Workspace"))
		metadata := hits[0]["metadata"].(map[string]any)
		Expect(metadata["version_id"]).To(Equal(version))
		Expect(metadata["citation_locator"]).NotTo(BeNil())
		Expect(metadata["source_file"]).To(Equal("synthetic-image.png"))
		req.ExactTerm = "Normalized fallback"
		result, err = derivedTextEvidence(ctx, db, req)
		Expect(err).NotTo(HaveOccurred())
		Expect(evidenceResults(result)).To(HaveLen(1))
		for _, phrase := range []string{"ZZ99ZZZ", "Investigati0n Workspace"} {
			req.ExactTerm = phrase
			result, err = derivedTextEvidence(ctx, db, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(evidenceResults(result)).To(BeEmpty())
		}
	})
})
