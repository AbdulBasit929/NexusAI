package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// This suite only owns the database explicitly created by its operator harness.
// The production tables and handlers are not replaced with query mocks.
var _ = Describe("Disposable canonical group SQL API", Ordered, func() {
	var db *pgxpool.Pool
	ctx := context.Background()
	exec := func(sql string, args ...any) { _, err := db.Exec(ctx, sql, args...); Expect(err).NotTo(HaveOccurred()) }
	BeforeAll(func() {
		dsn := os.Getenv("NXB21_GROUP_TEST_DATABASE_URL")
		if dsn == "" {
			Skip("requires owned disposable group database")
		}
		cfg, err := pgxpool.ParseConfig(dsn)
		Expect(err).NotTo(HaveOccurred())
		name := os.Getenv("NXB21_GROUP_TEST_DATABASE_NAME")
		Expect(name).To(MatchRegexp(`^nxb21_group_v2_[a-f0-9]{16}$`))
		Expect(cfg.ConnConfig.Database).To(Equal(name))
		Expect(cfg.ConnConfig.User).To(Equal(name))
		db, err = pgxpool.NewWithConfig(ctx, cfg)
		Expect(err).NotTo(HaveOccurred())
		var super, bypass bool
		Expect(db.QueryRow(ctx, "SELECT rolsuper,rolbypassrls FROM pg_roles WHERE rolname=current_user").Scan(&super, &bypass)).To(Succeed())
		Expect(super || bypass).To(BeFalse())
		exec(`CREATE SCHEMA forensic;
CREATE TABLE forensic.records(record_id uuid PRIMARY KEY,tenant_id text NOT NULL,collection_id text NOT NULL,file_id text,batch_id uuid,record_type text NOT NULL,timestamp timestamptz NOT NULL,primary_target text,secondary_target text,source_file text NOT NULL,row_number bigint,row_hash text,raw_payload jsonb DEFAULT '{}',metadata jsonb DEFAULT '{}',ingested_at timestamptz DEFAULT now());
CREATE TABLE forensic.cdr_records(tenant_id text,collection_id text,call_start_ts timestamptz);
CREATE TABLE forensic.generic_records(tenant_id text,collection_id text,observed_at timestamptz,record_type text);
CREATE TABLE forensic.kb_active_metadata(tenant_id text,collection_id text,record_type text,inserted_rows bigint);
CREATE TABLE forensic.record_entities(tenant_id text,collection_id text,entity_value text,observed_at timestamptz);
CREATE TABLE forensic.evidence_items(tenant_id text,collection_id text,case_id text,evidence_id uuid,current_version_id uuid,modality text,detected_type text,extension text,kb_entry_ref text,records_batch_id uuid);
CREATE TABLE forensic.derived_artifacts(tenant_id text,collection_id text,evidence_id uuid,version_id uuid,artifact_type text,processing_status text);
ALTER TABLE forensic.records ENABLE ROW LEVEL SECURITY;
ALTER TABLE forensic.records FORCE ROW LEVEL SECURITY;
CREATE POLICY scope ON forensic.records USING(tenant_id=current_setting('app.tenant_id',true));`)
	})
	AfterAll(func() {
		if db != nil {
			db.Close()
		}
	})
	BeforeEach(func() {
		exec(`ALTER TABLE forensic.records NO FORCE ROW LEVEL SECURITY; TRUNCATE forensic.records; ALTER TABLE forensic.records ALTER COLUMN source_file SET NOT NULL;
INSERT INTO forensic.records(record_id,tenant_id,collection_id,file_id,batch_id,record_type,timestamp,source_file,row_number,row_hash,raw_payload)
SELECT ('00000000-0000-4000-8000-'||lpad(i::text,12,'0'))::uuid,
 CASE WHEN i=5 THEN 'foreign' ELSE 'group-tenant' END,CASE WHEN i=6 THEN 'foreign-case' ELSE 'group-case' END,
 'file-'||i,'10000000-0000-4000-8000-000000000001',CASE WHEN i IN (2,4) THEN 'generic' ELSE 'cdr' END,
 '2026-08-01T10:00:00Z'::timestamptz,CASE WHEN i IN (1,3) THEN 'A.csv' ELSE 'B.csv' END,i,'hash-'||i,jsonb_build_object('included',i<3)
FROM generate_series(1,6) i;
ALTER TABLE forensic.records FORCE ROW LEVEL SECURITY;`)
	})
	request := func(field string) hybridQueryRequest {
		return hybridQueryRequest{TenantID: "group-tenant", CollectionID: "group-case", Group: &CanonicalGroupV1{Field: field, Measure: "count"}}
	}
	query := func(req hybridQueryRequest) map[string]any {
		out, err := canonicalRecords(ctx, db, req)
		Expect(err).NotTo(HaveOccurred())
		return out
	}
	api := func(body map[string]any) *httptest.ResponseRecorder {
		data, err := json.Marshal(body)
		Expect(err).NotTo(HaveOccurred())
		mux := http.NewServeMux()
		mux.HandleFunc("POST /cases/{case_id}/query", forensicCaseQueryV1Handler(config{}, db))
		h := forensicAuthMiddleware(forensicAuthConfig{APIKey: "disposable-key", Required: true, TrustedTenantID: "group-tenant"}, mux)
		r := httptest.NewRequest("POST", "/cases/group-case/query", bytes.NewReader(data))
		for k, v := range map[string]string{"Authorization": "Bearer disposable-key", forensicTenantHeader: "group-tenant", forensicActorHeader: "analyst", forensicSubjectHeader: "analyst", forensicActorRoleHeader: "user", forensicCollectionHeader: "group-case", forensicCaseHeader: "group-case"} {
			r.Header.Set(k, v)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	body := func(field string) map[string]any {
		return map[string]any{"contract_version": "forensics.query-plan/v1", "intent": "forensics.canonical_records", "group": map[string]any{"field": field, "measure": "count"}}
	}
	It("executes a validated dynamic grouped plan through SQL, authorization and presentation", func() {
		req := hybridQueryRequest{TenantID: "group-tenant", UserID: "analyst", CollectionID: "group-case", Query: "Count tabular rows by source."}
		resolved, err := applySemanticDynamicPlan(req, "generic_tabular", semanticDynamicProposal{Mode: "group", Group: &CanonicalGroupV1{Field: "source_file", Measure: "count"}})
		Expect(err).NotTo(HaveOccurred())
		Expect(resolved.RecordType).To(Equal("generic"))
		out := query(resolved)
		Expect(out["total_count"]).To(Equal(2))
		Expect(out["row_count"]).To(Equal(1))
		raw, err := json.Marshal(resolved)
		Expect(err).NotTo(HaveOccurred())
		var payload map[string]any
		Expect(json.Unmarshal(raw, &payload)).To(Succeed())
		w := api(payload)
		Expect(w.Code).To(Equal(200), w.Body.String())
		Expect(w.Body.String()).To(ContainSubstring("2 authorized source rows into 1 groups"))
		Expect(w.Body.String()).To(ContainSubstring("hash-2"))
		Expect(w.Body.String()).NotTo(ContainSubstring("hash-1"))
		Expect(w.Body.String()).NotTo(ContainSubstring("hash-5"))
		Expect(w.Body.String()).NotTo(ContainSubstring("hash-6"))
	})
	It("executes typed record-type and source-file grouping through SQL and presentation", func() {
		for _, field := range []string{"record_type", "source_file"} {
			out := query(request(field))
			Expect(out["total_count"]).To(Equal(4))
			Expect(out["row_count"]).To(Equal(2))
			Expect(canonicalAnswerRowCount("canonical_records", out)).To(Equal(2))
			Expect(flattenEnterpriseRows(out, 100)).To(HaveLen(2))
			Expect(summarizeRecords("canonical_records", out)).To(ContainSubstring("4 authorized source rows into 2 groups"))
			w := api(body(field))
			Expect(w.Code).To(Equal(200), w.Body.String())
			Expect(w.Body.String()).To(ContainSubstring("hash-1"), w.Body.String())
			Expect(w.Body.String()).NotTo(ContainSubstring("hash-5"))
			Expect(w.Body.String()).NotTo(ContainSubstring("hash-6"))
			Expect(w.Body.String()).To(ContainSubstring("4 authorized source rows"), w.Body.String())
		}
	})
	It("retains exact ordered contribution lineage and deterministic ties", func() {
		groups := query(request("source_file"))["canonical_groups"].([]map[string]any)
		Expect(groups[0]["value"]).To(Equal("A.csv"))
		Expect(groups[1]["value"]).To(Equal("B.csv"))
		for i, g := range groups {
			Expect(g["count"]).To(Equal(2))
			rows := g["metadata"].(map[string]any)["source_rows"].([]map[string]any)
			for j, row := range rows {
				n := i + 1 + 2*j
				Expect(row["record_id"]).To(Equal(fmt.Sprintf("00000000-0000-4000-8000-%012d", n)))
				Expect(row["row_hash"]).To(Equal(fmt.Sprint("hash-", n)))
				Expect(row["row_number"]).To(BeNumerically("==", n))
				Expect(row["source_table"]).To(Equal("forensic.records"))
				Expect(row["tenant_id"]).To(Equal("group-tenant"))
			}
		}
	})
	It("executes calendar buckets through the typed API with exact source lineage", func() {
		b := body("timestamp")
		b["group"] = map[string]any{"field": "timestamp", "measure": "count", "bucket": "day", "timezone": "Asia/Karachi"}
		b["time_range"] = map[string]any{"from": "2026-08-01T00:00:00Z", "to": "2026-08-02T00:00:00Z"}
		w := api(b)
		Expect(w.Code).To(Equal(200), w.Body.String())
		Expect(w.Body.String()).To(ContainSubstring("2026-08-01T00:00:00+05:00"))
		Expect(w.Body.String()).To(ContainSubstring("bucket_end_exclusive"))
		Expect(w.Body.String()).To(ContainSubstring("hash-4"))
		Expect(w.Body.String()).NotTo(ContainSubstring("hash-5"))
	})
	It("executes source comparison through the typed API and preserves source citations", func() {
		b := map[string]any{"contract_version": "forensics.query-plan/v1", "intent": "forensics.canonical_records", "compare": map[string]any{"field": "source_file", "measure": "count", "a": map[string]any{"value": "A.csv"}, "b": map[string]any{"value": "B.csv"}}}
		w := api(b)
		Expect(w.Code).To(Equal(200), w.Body.String())
		Expect(w.Body.String()).To(ContainSubstring(`"difference_from_a":0`))
		Expect(w.Body.String()).To(ContainSubstring("hash-4"))
		Expect(w.Body.String()).NotTo(ContainSubstring("hash-6"))
		for _, field := range []string{"projection", "group", "sort", "offset", "limit"} {
			b[field] = 1
			Expect(api(b).Code).To(Equal(400))
			delete(b, field)
		}
	})
	It("runs filter then group then count then stable sort then top-k without losing total-group truth", func() {
		b := body("source_file")
		b["group"] = map[string]any{"field": "source_file", "measure": "count", "top_k": 1}
		w := api(b)
		Expect(w.Code).To(Equal(200), w.Body.String())
		Expect(w.Body.String()).To(ContainSubstring("showing the top 1 groups"))
		Expect(w.Body.String()).To(ContainSubstring("into 2 groups"))
		b["filters"] = []map[string]any{{"field": "record_type", "op": "eq", "value": "cdr"}}
		w = api(b)
		Expect(w.Code).To(Equal(200), w.Body.String())
		Expect(w.Body.String()).To(ContainSubstring("into 1 groups"))
		Expect(w.Body.String()).NotTo(ContainSubstring("hash-4"))
	})
	It("handles explicit NULL defensively in a deliberately nullable scratch fixture", func() {
		exec(`ALTER TABLE forensic.records NO FORCE ROW LEVEL SECURITY; ALTER TABLE forensic.records ALTER COLUMN source_file DROP NOT NULL; UPDATE forensic.records SET source_file=NULL WHERE row_number IN(1,3); ALTER TABLE forensic.records FORCE ROW LEVEL SECURITY;`)
		groups := query(request("source_file"))["canonical_groups"].([]map[string]any)
		Expect(groups[0]["is_null"]).To(Equal(true))
		Expect(groups[0]["value"]).To(BeNil())
		Expect(groups[0]["count"]).To(Equal(2))
		w := api(body("source_file"))
		Expect(w.Code).To(Equal(200))
		Expect(w.Body.String()).To(ContainSubstring(`"is_null":true`))
	})
	It("preserves zero, filters and source isolation", func() {
		req := request("record_type")
		req.SourceFile = "absent.csv"
		out := query(req)
		Expect(out["total_count"]).To(Equal(0))
		Expect(out["row_count"]).To(Equal(0))
		req.SourceFile = "A.csv"
		out = query(req)
		Expect(out["total_count"]).To(Equal(2))
		Expect(out["row_count"]).To(Equal(1))
		req.RecordType = "generic"
		Expect(query(req)["total_count"]).To(Equal(0))
		req.SourceFile = "B.csv"
		Expect(query(req)["total_count"]).To(Equal(2))
		for _, value := range []bool{true, false} {
			b := body("record_type")
			b["filters"] = []map[string]any{{"field": "included", "op": "eq", "value": fmt.Sprint(value)}}
			w := api(b)
			Expect(w.Code).To(Equal(200), w.Body.String())
			Expect(w.Body.String()).To(ContainSubstring("2 authorized source rows"), w.Body.String())
		}
		b := body("record_type")
		b["filters"] = []map[string]any{{"field": "included", "op": "eq", "value": "absent"}}
		w := api(b)
		Expect(w.Code).To(Equal(200), w.Body.String())
		Expect(w.Body.String()).NotTo(ContainSubstring("hash-"))
		Expect(w.Body.String()).To(ContainSubstring("0 authorized source rows"), w.Body.String())
	})
	It("enforces tenant and collection authority including hostile typed requests", func() {
		req := request("record_type")
		req.CollectionID = "absent-case"
		Expect(query(req)["total_count"]).To(Equal(0))
		for k, v := range map[string]string{"tenant_id": "foreign", "collection_id": "foreign-case", "case_id": "foreign-case"} {
			b := body("record_type")
			b[k] = v
			w := api(b)
			Expect(w.Code).To(BeNumerically(">=", 400), w.Body.String())
			Expect(w.Body.String()).NotTo(ContainSubstring("hash-"))
		}
	})
	It("accepts exactly 100 groups and 1000 rows and fails closed on either overflow", func() {
		for _, v := range []struct {
			rows, groups int
			pass         bool
		}{{100, 100, true}, {1000, 100, true}, {101, 101, false}, {1001, 1, false}} {
			exec(`ALTER TABLE forensic.records NO FORCE ROW LEVEL SECURITY; TRUNCATE forensic.records;`)
			exec(`INSERT INTO forensic.records(record_id,tenant_id,collection_id,record_type,timestamp,source_file,row_number,row_hash) SELECT ('00000000-0000-4000-8000-'||lpad(i::text,12,'0'))::uuid,'group-tenant','group-case','cdr',now(),'source-'||(i%$2),i,'bound-'||i FROM generate_series(1,$1::int) i`, v.rows, v.groups)
			exec(`ALTER TABLE forensic.records FORCE ROW LEVEL SECURITY`)
			out, err := canonicalRecords(ctx, db, request("source_file"))
			w := api(body("source_file"))
			if v.pass {
				Expect(err).NotTo(HaveOccurred())
				Expect(out["total_count"]).To(Equal(v.rows))
				Expect(out["row_count"]).To(Equal(v.groups))
				Expect(w.Code).To(Equal(200))
				Expect(w.Body.String()).To(ContainSubstring("bound-1"), w.Body.String())
			} else {
				Expect(err).To(HaveOccurred())
				Expect(out).To(BeNil())
				Expect(w.Body.String()).NotTo(ContainSubstring("bound-1"))
				Expect(strings.ToLower(w.Body.String())).To(Or(ContainSubstring("exceed"), ContainSubstring("cardinality")), w.Body.String())
			}
		}
	})
	It("rejects unsupported and malformed group controls without silently executing", func() {
		for _, g := range []any{map[string]any{"field": "primary_target", "measure": "count"}, map[string]any{"field": "source_file", "measure": "sum"}, map[string]any{"field": "source_file", "measure": "count", "sql": "select 1"}, "source_file", map[string]any{}} {
			b := body("source_file")
			b["group"] = g
			w := api(b)
			Expect(w.Code).To(Equal(400), w.Body.String())
			Expect(w.Body.String()).NotTo(ContainSubstring("hash-"))
		}
		for _, key := range []string{"limit", "offset", "projection", "sort_by"} {
			b := body("source_file")
			b[key] = 1
			Expect(api(b).Code).To(Equal(400), key)
		}
	})
})
