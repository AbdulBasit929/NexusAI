package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The operator harness creates and later destroys the only database touched by
// this suite. The role is NOSUPERUSER/NOBYPASSRLS and production handlers run
// the typed plan against real PostgreSQL rows.
var _ = Describe("Disposable source-native SQL API", Ordered, func() {
	var db *pgxpool.Pool
	ctx := context.Background()
	exec := func(sql string, args ...any) { _, err := db.Exec(ctx, sql, args...); Expect(err).NotTo(HaveOccurred()) }
	BeforeAll(func() {
		dsn := os.Getenv("NXB21_SOURCE_NATIVE_TEST_DATABASE_URL")
		if dsn == "" {
			Skip("requires owned disposable source-native database")
		}
		cfg, err := pgxpool.ParseConfig(dsn)
		Expect(err).NotTo(HaveOccurred())
		name := os.Getenv("NXB21_SOURCE_NATIVE_TEST_DATABASE_NAME")
		Expect(name).To(MatchRegexp(`^nxb21_fields_[a-f0-9]{16}$`))
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
INSERT INTO forensic.evidence_items VALUES('field-tenant','field-case','field-case','20000000-0000-4000-8000-000000000001','30000000-0000-4000-8000-000000000001','structured','generic','csv','', '10000000-0000-4000-8000-000000000001');
INSERT INTO forensic.records(record_id,tenant_id,collection_id,file_id,batch_id,record_type,timestamp,source_file,row_number,row_hash,raw_payload) VALUES
('00000000-0000-4000-8000-000000000001','field-tenant','field-case','file-1','10000000-0000-4000-8000-000000000001','generic','2026-09-07T10:00:00Z','invoices-a.csv',1,'hash-1','{"department":"Operations","invoice_total":100.5,"device_model":"DX-1","event_time":"2026-09-07T10:00:00Z","source_branch":"North","secret_token":"hidden"}'),
('00000000-0000-4000-8000-000000000002','field-tenant','field-case','file-2','10000000-0000-4000-8000-000000000001','generic','2026-09-08T10:00:00Z','invoices-a.csv',2,'hash-2','{"department":"Operations","invoice_total":200.5,"device_model":"DX-2","event_time":"2026-09-08T10:00:00Z","source_branch":"North","secret_token":"hidden"}'),
('00000000-0000-4000-8000-000000000003','field-tenant','field-case','file-3','10000000-0000-4000-8000-000000000001','generic','2026-09-09T10:00:00Z','invoices-b.csv',3,'hash-3','{"department":"Sales","invoice_total":300.25,"device_model":"DX-1","event_time":"2026-09-09T10:00:00Z","source_branch":"South","secret_token":"hidden"}'),
('00000000-0000-4000-8000-000000000004','field-tenant','field-case','file-4','10000000-0000-4000-8000-000000000001','generic','2026-09-10T10:00:00Z','invoices-b.csv',4,'hash-4','{"department":"Sales","invoice_total":499.75,"device_model":"DX-1","event_time":"2026-09-10T10:00:00Z","source_branch":"South","secret_token":"hidden"}'),
('00000000-0000-4000-8000-000000000005','field-tenant','field-case','file-5','10000000-0000-4000-8000-000000000001','generic','2026-09-11T10:00:00Z','invoices-b.csv',5,'hash-5','{"department":null,"invoice_total":50,"device_model":"DX-3","event_time":"2026-09-11T10:00:00Z","source_branch":"South","secret_token":"hidden"}'),
('00000000-0000-4000-8000-000000000006','foreign','field-case','file-x','10000000-0000-4000-8000-000000000001','generic','2026-09-11T10:00:00Z','foreign.csv',6,'foreign-hash','{"department":"Foreign","invoice_total":999999}');
ALTER TABLE forensic.records ENABLE ROW LEVEL SECURITY;
ALTER TABLE forensic.records FORCE ROW LEVEL SECURITY;
CREATE POLICY scope ON forensic.records USING(tenant_id=current_setting('app.tenant_id',true));`)
	})
	AfterAll(func() {
		if db != nil {
			db.Close()
		}
	})

	req := func() hybridQueryRequest {
		return hybridQueryRequest{TenantID: "field-tenant", UserID: "analyst", CollectionID: "field-case", RecordType: "generic", Query: "Which department has the highest average invoice total?", QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)}}
	}
	plan := func(catalog []FieldDescriptorV1) *SourceNativePlanV1 {
		return &SourceNativePlanV1{ContractVersion: sourceNativePlanContractV1, Project: []string{}, Filters: []SourceNativeFilterV1{}, GroupFields: []string{sourceNativeField(catalog, "department").FieldID}, Measures: []SourceNativeMeasureV1{{MeasureID: "m1", Op: "AVG", FieldID: sourceNativeField(catalog, "invoice_total").FieldID}}, Having: []SourceNativeHavingV1{}, Sort: []SourceNativeSortV1{{Target: "m1", Direction: "DESC"}}, Limit: 1}
	}
	api := func(body map[string]any) *httptest.ResponseRecorder {
		data, err := json.Marshal(body)
		Expect(err).NotTo(HaveOccurred())
		mux := http.NewServeMux()
		mux.HandleFunc("POST /cases/{case_id}/query", forensicCaseQueryV1Handler(config{}, db))
		h := forensicAuthMiddleware(forensicAuthConfig{APIKey: "disposable-key", Required: true, TrustedTenantID: "field-tenant"}, mux)
		r := httptest.NewRequest(http.MethodPost, "/cases/field-case/query", bytes.NewReader(data))
		for key, value := range map[string]string{"Authorization": "Bearer disposable-key", forensicTenantHeader: "field-tenant", forensicActorHeader: "analyst", forensicSubjectHeader: "analyst", forensicActorRoleHeader: "user", forensicCollectionHeader: "field-case", forensicCaseHeader: "field-case"} {
			r.Header.Set(key, value)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}

	It("discovers fields from the authorized SQL scope and executes the representative AST through the public API", func() {
		request := req()
		catalog, rows, err := discoverSourceNativeFieldCatalog(ctx, db, request)
		Expect(err).NotTo(HaveOccurred())
		Expect(rows).To(HaveLen(5))
		Expect(sourceNativeField(catalog, "secret_token").FieldID).To(BeEmpty())
		retrieved, _ := retrieveSourceNativeFields(request.Query, catalog)
		names := []string{}
		for _, field := range retrieved {
			names = append(names, field.NormalizedName)
		}
		Expect(names).To(ContainElements("department", "invoice_total"))

		request.SourceNative = plan(catalog)
		out, err := canonicalRecords(ctx, db, request)
		Expect(err).NotTo(HaveOccurred())
		result := out["source_native_results"].([]map[string]any)
		Expect(result).To(HaveLen(1))
		Expect(result[0]["department"]).To(Equal("Sales"))
		Expect(result[0]["m1"]).To(BeNumerically("==", 400))
		Expect(result[0]["metadata"].(map[string]any)["source_rows"]).To(HaveLen(2))

		body := map[string]any{"contract_version": "forensics.query-plan/v1", "intent": "forensics.canonical_records", "families": []string{"generic_tabular"}, "source_native": request.SourceNative}
		response := api(body)
		Expect(response.Code).To(Equal(http.StatusOK), response.Body.String())
		Expect(response.Body.String()).To(ContainSubstring("Sales"))
		Expect(response.Body.String()).To(ContainSubstring(`"m1":400`))
		Expect(response.Body.String()).To(ContainSubstring("hash-3"))
		Expect(response.Body.String()).To(ContainSubstring("hash-4"))
		Expect(response.Body.String()).NotTo(ContainSubstring("foreign-hash"))
		Expect(response.Body.String()).NotTo(ContainSubstring("secret_token"))
		Expect(response.Body.String()).To(ContainSubstring(`"semantic_evidence":[`))
		Expect(response.Body.String()).To(ContainSubstring(`"evidence_id":"20000000-0000-4000-8000-000000000001"`))
		Expect(response.Body.String()).To(ContainSubstring(`"version_id":"30000000-0000-4000-8000-000000000001"`))
	})

	It("executes project, count/sum/avg/min/max, HAVING, aggregate sort, limit and source-native time bucket over SQL rows", func() {
		request := req()
		catalog, _, err := discoverSourceNativeFieldCatalog(ctx, db, request)
		Expect(err).NotTo(HaveOccurred())
		department := sourceNativeField(catalog, "department")
		invoice := sourceNativeField(catalog, "invoice_total")
		eventTime := sourceNativeField(catalog, "event_time")
		device := sourceNativeField(catalog, "device_model")

		request.SourceNative = &SourceNativePlanV1{ContractVersion: sourceNativePlanContractV1, Project: []string{department.FieldID, device.FieldID}, Filters: []SourceNativeFilterV1{}, GroupFields: []string{}, Measures: []SourceNativeMeasureV1{}, Having: []SourceNativeHavingV1{}, Sort: []SourceNativeSortV1{{Target: department.FieldID, Direction: "ASC"}}, Limit: 2}
		projected, err := canonicalRecords(ctx, db, request)
		Expect(err).NotTo(HaveOccurred())
		Expect(projected["source_native_results"]).To(HaveLen(2))

		request.SourceNative = &SourceNativePlanV1{ContractVersion: sourceNativePlanContractV1, Project: []string{}, Filters: []SourceNativeFilterV1{}, GroupFields: []string{department.FieldID}, Measures: []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT"}, {MeasureID: "m2", Op: "SUM", FieldID: invoice.FieldID}, {MeasureID: "m3", Op: "MIN", FieldID: invoice.FieldID}, {MeasureID: "m4", Op: "MAX", FieldID: invoice.FieldID}}, Having: []SourceNativeHavingV1{{MeasureID: "m1", Op: "GTE", Value: "2"}}, Sort: []SourceNativeSortV1{{Target: "m2", Direction: "DESC"}}, Limit: 1}
		aggregated, err := canonicalRecords(ctx, db, request)
		Expect(err).NotTo(HaveOccurred())
		result := aggregated["source_native_results"].([]map[string]any)
		Expect(result).To(HaveLen(1))
		Expect(result[0]["department"]).To(Equal("Sales"))
		Expect(result[0]["m1"]).To(Equal(2))
		Expect(result[0]["m2"]).To(BeNumerically("==", 800))
		Expect(result[0]["m3"]).To(BeNumerically("==", 300.25))
		Expect(result[0]["m4"]).To(BeNumerically("==", 499.75))

		request.SourceNative = &SourceNativePlanV1{ContractVersion: sourceNativePlanContractV1, Project: []string{}, Filters: []SourceNativeFilterV1{}, GroupFields: []string{}, Measures: []SourceNativeMeasureV1{{MeasureID: "m1", Op: "AVG", FieldID: invoice.FieldID}}, TimeBucket: &SourceNativeTimeBucketV1{FieldID: eventTime.FieldID, Bucket: "day", Timezone: "Asia/Karachi"}, Having: []SourceNativeHavingV1{}, Sort: []SourceNativeSortV1{{Target: eventTime.FieldID, Direction: "ASC"}}, Limit: 10}
		bucketed, err := canonicalRecords(ctx, db, request)
		Expect(err).NotTo(HaveOccurred())
		Expect(bucketed["source_native_results"]).To(HaveLen(5))
	})

	It("binds field IDs to selected evidence/version and rejects foreign IDs, unsafe types and injection", func() {
		request := req()
		request.EvidenceID = "20000000-0000-4000-8000-000000000001"
		request.EvidenceVersionID = "30000000-0000-4000-8000-000000000001"
		request.QueryScope = queryEvidenceScope{Kind: string(EvidenceScopeSelected), EvidenceID: request.EvidenceID, EvidenceVersionID: request.EvidenceVersionID, SourceFamily: "generic"}
		catalog, rows, err := discoverSourceNativeFieldCatalog(ctx, db, request)
		Expect(err).NotTo(HaveOccurred())
		Expect(rows).To(HaveLen(5))
		Expect(sourceNativeField(catalog, "department").EvidenceID).To(Equal(request.EvidenceID))

		workspace := req()
		workspaceCatalog, _, err := discoverSourceNativeFieldCatalog(ctx, db, workspace)
		Expect(err).NotTo(HaveOccurred())
		Expect(sourceNativeField(workspaceCatalog, "department").FieldID).NotTo(Equal(sourceNativeField(catalog, "department").FieldID))
		bad := plan(workspaceCatalog)
		request.SourceNative = bad
		_, err = canonicalRecords(ctx, db, request)
		Expect(err).To(HaveOccurred())

		bad.Project = []string{"fld_');DROP TABLE forensic.records;--"}
		bad.GroupFields, bad.Measures = []string{}, []SourceNativeMeasureV1{}
		_, err = canonicalRecords(ctx, db, request)
		Expect(err).To(HaveOccurred())
		Expect(strings.ToLower(err.Error())).To(ContainSubstring("field"))
	})
})
