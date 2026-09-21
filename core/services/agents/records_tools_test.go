package agents

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("forensic records tools", func() {
	It("posts hybrid queries with tenant, user, collection, and limit defaults", func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			Expect(r.Method).To(Equal(http.MethodPost))
			Expect(r.URL.Path).To(Equal("/query/hybrid"))
			Expect(r.Header.Get("Authorization")).To(Equal("Bearer secret"))
			Expect(r.Header.Get("X-Forensic-Tenant-ID")).To(Equal("default"))
			Expect(r.Header.Get("X-Forensic-Actor-ID")).To(Equal("worker-1"))
			Expect(r.Header.Get("X-Forensic-Subject-ID")).To(Equal("user-1"))
			Expect(r.Header.Get("X-Forensic-Actor-Role")).To(Equal("agent-worker"))
			Expect(r.Header.Get("X-Forensic-Collection-ID")).To(Equal("records-demo"))

			var body ForensicHybridQueryArgs
			Expect(json.NewDecoder(r.Body).Decode(&body)).To(Succeed())
			Expect(body.TenantID).To(Equal("default"))
			Expect(body.UserID).To(Equal("user-1"))
			Expect(body.CollectionID).To(Equal("records-demo"))
			Expect(body.Query).To(Equal("show relationship network for ABC-123"))
			Expect(body.Limit).To(Equal(20))
			Expect(body.MaxKBResults).To(Equal(3))

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"template":"relationship_network","records":{"row_count":4}}`))
		}))
		defer server.Close()

		tool := ForensicHybridQueryTool{ForensicRecordsToolConfig: ForensicRecordsToolConfig{
			APIURL:       server.URL,
			APIKey:       "secret",
			UserID:       "user-1",
			CollectionID: "records-demo",
			ActorID:      "worker-1",
			ActorRole:    "agent-worker",
		}}

		text, raw, err := tool.Run(ForensicHybridQueryArgs{Query: "show relationship network for ABC-123"})
		Expect(err).ToNot(HaveOccurred())
		Expect(text).To(ContainSubstring("relationship_network"))
		Expect(raw).To(HaveKeyWithValue("template", "relationship_network"))
	})

	It("gets query templates", func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			Expect(r.Method).To(Equal(http.MethodGet))
			Expect(r.URL.Path).To(Equal("/query/templates"))
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"templates":[{"name":"data_quality"}]}`))
		}))
		defer server.Close()

		tool := ForensicQueryTemplatesTool{ForensicRecordsToolConfig: ForensicRecordsToolConfig{APIURL: server.URL}}
		text, raw, err := tool.Run(ForensicQueryTemplatesArgs{})
		Expect(err).ToNot(HaveOccurred())
		Expect(text).To(ContainSubstring("data_quality"))
		Expect(raw).To(HaveKey("templates"))
	})

	It("lists forensic evidence with configured defaults and filters", func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			Expect(r.Method).To(Equal(http.MethodGet))
			Expect(r.URL.Path).To(Equal("/evidence"))
			Expect(r.URL.Query().Get("tenant_id")).To(Equal("tenant-a"))
			Expect(r.URL.Query().Get("collection_id")).To(Equal("case-42"))
			Expect(r.URL.Query().Get("modality")).To(Equal("structured_records"))
			Expect(r.URL.Query().Get("processing_status")).To(Equal("completed"))
			Expect(r.URL.Query().Get("q")).To(Equal("calls.csv"))
			Expect(r.URL.Query().Get("limit")).To(Equal("50"))

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"items":[{"evidence_id":"11111111-1111-1111-1111-111111111111","source_file":"calls.csv"}]}`))
		}))
		defer server.Close()

		tool := ForensicListEvidenceTool{ForensicRecordsToolConfig: ForensicRecordsToolConfig{
			APIURL:       server.URL,
			TenantID:     "tenant-a",
			CollectionID: "case-42",
		}}
		text, raw, err := tool.Run(ForensicListEvidenceArgs{
			Modality:         "structured_records",
			ProcessingStatus: "completed",
			Query:            "calls.csv",
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(text).To(ContainSubstring("calls.csv"))
		Expect(raw).To(HaveKey("items"))
	})

	It("gets forensic evidence detail with tenant and row preview limit", func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			Expect(r.Method).To(Equal(http.MethodGet))
			Expect(r.URL.Path).To(Equal("/evidence/11111111-1111-1111-1111-111111111111"))
			Expect(r.URL.Query().Get("tenant_id")).To(Equal("default"))
			Expect(r.URL.Query().Get("collection_id")).To(Equal("records-demo"))
			Expect(r.URL.Query().Get("limit")).To(Equal("10"))

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"evidence_id":"11111111-1111-1111-1111-111111111111","records_preview":[{"record_type":"cdr"}]}`))
		}))
		defer server.Close()

		tool := ForensicGetEvidenceTool{ForensicRecordsToolConfig: ForensicRecordsToolConfig{APIURL: server.URL, CollectionID: "records-demo"}}
		text, raw, err := tool.Run(ForensicGetEvidenceArgs{
			EvidenceID: "11111111-1111-1111-1111-111111111111",
			Limit:      10,
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(text).To(ContainSubstring("records_preview"))
		Expect(raw).To(HaveKey("records_preview"))
	})

	It("posts canonical records controls for raw attribute queries", func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			Expect(r.Method).To(Equal(http.MethodPost))
			Expect(r.URL.Path).To(Equal("/query/hybrid"))

			var body ForensicHybridQueryArgs
			Expect(json.NewDecoder(r.Body).Decode(&body)).To(Succeed())
			Expect(body.Template).To(Equal("canonical_records"))
			Expect(body.RecordType).To(Equal("cdr"))
			Expect(body.Targets).To(Equal([]string{"923001110001", "923009998885"}))
			Expect(body.SourceFile).To(Equal("seed_cdr_large.csv"))
			Expect(body.BatchID).To(Equal("batch-1"))
			Expect(body.RawPayloadFilters).To(HaveLen(1))
			Expect(body.RawPayloadFilters[0].Field).To(Equal("call_type"))
			Expect(body.RawPayloadFilters[0].Op).To(Equal("eq"))
			Expect(body.FieldExists).To(Equal([]string{"imei"}))
			Expect(body.SortBy).To(Equal("timestamp"))
			Expect(body.SortDirection).To(Equal("asc"))
			Expect(body.Offset).To(Equal(10))

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"template":"canonical_records","records":{"row_count":1,"canonical_records":[{"record_type":"cdr"}]}}`))
		}))
		defer server.Close()

		tool := ForensicHybridQueryTool{ForensicRecordsToolConfig: ForensicRecordsToolConfig{APIURL: server.URL, CollectionID: "records-demo"}}
		_, raw, err := tool.Run(ForensicHybridQueryArgs{
			Query:         "show canonical CDR rows where call_type is GPRS",
			Template:      "canonical_records",
			RecordType:    "cdr",
			Targets:       []string{"923001110001", "923009998885"},
			SourceFile:    "seed_cdr_large.csv",
			BatchID:       "batch-1",
			FieldExists:   []string{"imei"},
			SortBy:        "timestamp",
			SortDirection: "asc",
			Offset:        10,
			RawPayloadFilters: []ForensicCanonicalPayloadFilter{
				{Field: "call_type", Op: "eq", Value: "GPRS"},
			},
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(raw).To(HaveKeyWithValue("template", "canonical_records"))
	})

	It("posts report generation requests with configured defaults", func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			Expect(r.Method).To(Equal(http.MethodPost))
			Expect(r.URL.Path).To(Equal("/reports/generate"))

			var body ForensicReportArgs
			Expect(json.NewDecoder(r.Body).Decode(&body)).To(Succeed())
			Expect(body.TenantID).To(Equal("tenant-a"))
			Expect(body.UserID).To(Equal("user-1"))
			Expect(body.CollectionID).To(Equal("case-42"))
			Expect(body.Target).To(Equal("ABC-123"))
			Expect(body.IncludeEvidence).To(BeTrue())

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"markdown":"# Forensic Intelligence Report"}`))
		}))
		defer server.Close()

		tool := ForensicGenerateReportTool{ForensicRecordsToolConfig: ForensicRecordsToolConfig{
			APIURL:       server.URL,
			UserID:       "user-1",
			TenantID:     "tenant-a",
			CollectionID: "case-42",
		}}

		text, raw, err := tool.Run(ForensicReportArgs{Target: "ABC-123", IncludeEvidence: true})
		Expect(err).ToNot(HaveOccurred())
		Expect(text).To(ContainSubstring("Forensic Intelligence Report"))
		Expect(raw).To(HaveKey("markdown"))
	})

	It("resolves agent forensic tool config only when enabled", func() {
		Expect(forensicRecordsToolConfig(&AgentConfig{Name: "case-agent"}, "user-1").APIURL).To(BeEmpty())

		cfg := &AgentConfig{
			Name:                  "Forensic_Records_Analyst",
			EnableForensicRecords: true,
			ForensicRecordsAPIURL: "http://records-api:8091",
		}
		resolved := forensicRecordsToolConfig(cfg, "user-1")
		Expect(resolved.APIURL).To(Equal("http://records-api:8091"))
		Expect(resolved.UserID).To(Equal("user-1"))
		Expect(resolved.TenantID).To(Equal("default"))
		Expect(resolved.CollectionID).To(Equal("records-demo"))
		Expect(resolved.ActorID).To(Equal("user-1"))
		Expect(resolved.ActorRole).To(Equal("agent-worker"))
		Expect(agentKnowledgeBaseCollection(cfg)).To(Equal("records-demo"))
		Expect(normalizeForensicCollectionID("forensic_records_analyst")).To(Equal("records-demo"))
		Expect(normalizeForensicCollectionID("records-demo")).To(Equal("records-demo"))
	})

	It("uses the explicit local proxy identity when authentication is disabled", func() {
		previousActor, actorWasSet := os.LookupEnv("FORENSIC_RECORDS_PROXY_ACTOR_ID")
		previousRole, roleWasSet := os.LookupEnv("FORENSIC_RECORDS_PROXY_ACTOR_ROLE")
		DeferCleanup(func() {
			if actorWasSet {
				_ = os.Setenv("FORENSIC_RECORDS_PROXY_ACTOR_ID", previousActor)
			} else {
				_ = os.Unsetenv("FORENSIC_RECORDS_PROXY_ACTOR_ID")
			}
			if roleWasSet {
				_ = os.Setenv("FORENSIC_RECORDS_PROXY_ACTOR_ROLE", previousRole)
			} else {
				_ = os.Unsetenv("FORENSIC_RECORDS_PROXY_ACTOR_ROLE")
			}
		})

		Expect(os.Setenv("FORENSIC_RECORDS_PROXY_ACTOR_ID", "local-operator")).To(Succeed())
		Expect(os.Setenv("FORENSIC_RECORDS_PROXY_ACTOR_ROLE", "admin")).To(Succeed())
		resolved := forensicRecordsToolConfig(&AgentConfig{
			Name:                  "Forensic_Records_Analyst",
			EnableForensicRecords: true,
			ForensicRecordsAPIURL: "http://records-api:8091",
		}, "")

		Expect(resolved.UserID).To(Equal("local-operator"))
		Expect(resolved.ActorID).To(Equal("local-operator"))
		Expect(resolved.ActorRole).To(Equal("admin"))
	})

	It("directly routes explicit forensic template requests to the templates endpoint", func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			Expect(r.Method).To(Equal(http.MethodGet))
			Expect(r.URL.Path).To(Equal("/query/templates"))
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"templates":[{"name":"call_type_breakdown","route":"records"}]}`))
		}))
		defer server.Close()

		result, ok, err := runDeterministicForensicRoute(
			"Use forensic_query_templates and list the deterministic forensic templates for records-demo.",
			ForensicRecordsToolConfig{APIURL: server.URL, CollectionID: "records-demo"},
		)

		Expect(err).ToNot(HaveOccurred())
		Expect(ok).To(BeTrue())
		Expect(result.ToolName).To(Equal("forensic_query_templates"))
		Expect(result.Text).To(ContainSubstring("call_type_breakdown"))
	})

	It("directly routes explicit forensic hybrid requests with target and defaults", func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			Expect(r.Method).To(Equal(http.MethodPost))
			Expect(r.URL.Path).To(Equal("/query/hybrid"))

			var body ForensicHybridQueryArgs
			Expect(json.NewDecoder(r.Body).Decode(&body)).To(Succeed())
			Expect(body.TenantID).To(Equal("default"))
			Expect(body.CollectionID).To(Equal("records-demo"))
			Expect(body.Target).To(Equal("923461678183"))
			Expect(body.Limit).To(Equal(20))
			Expect(body.MaxKBResults).To(Equal(3))

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"template":"call_type_breakdown","records":{"row_count":5}}`))
		}))
		defer server.Close()

		result, ok, err := runDeterministicForensicRoute(
			"Use forensic_hybrid_query to show call type breakdown for 923461678183 with KB evidence.",
			ForensicRecordsToolConfig{APIURL: server.URL, CollectionID: "records-demo"},
		)

		Expect(err).ToNot(HaveOccurred())
		Expect(ok).To(BeTrue())
		Expect(result.ToolName).To(Equal("forensic_hybrid_query"))
		Expect(result.ArgsJSON).To(ContainSubstring("923461678183"))
		Expect(result.Text).To(ContainSubstring("call_type_breakdown"))
	})

	It("directly routes evidence inventory questions to the evidence catalog", func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			Expect(r.Method).To(Equal(http.MethodGet))
			Expect(r.URL.Path).To(Equal("/evidence"))
			Expect(r.URL.Query().Get("collection_id")).To(Equal("records-demo"))

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"summary":{"evidence_total":1,"completed":1},"items":[{"evidence_id":"11111111-1111-1111-1111-111111111111","source_file":"calls.csv","detected_type":"cdr","processing_status":"completed","processing_route":"forensic_records_worker"}]}`))
		}))
		defer server.Close()

		result, ok, err := runDeterministicForensicRoute(
			"what evidence exists in this collection?",
			ForensicRecordsToolConfig{APIURL: server.URL, CollectionID: "records-demo"},
		)

		Expect(err).ToNot(HaveOccurred())
		Expect(ok).To(BeTrue())
		Expect(result.ToolName).To(Equal("forensic_list_evidence"))
		Expect(result.Text).To(ContainSubstring("Evidence inventory"))
		Expect(result.Text).To(ContainSubstring("calls.csv"))
	})

	It("directly computes natural forensic runtime questions before bounded LLM presentation", func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			Expect(r.Method).To(Equal(http.MethodPost))
			Expect(r.URL.Path).To(Equal("/query/hybrid"))

			var body ForensicHybridQueryArgs
			Expect(json.NewDecoder(r.Body).Decode(&body)).To(Succeed())
			Expect(body.Query).To(Equal("shortest call duration of 923461678183"))
			Expect(body.Target).To(Equal("923461678183"))
			Expect(body.Template).To(Equal("shortest_call"))
			Expect(body.CollectionID).To(Equal("records-demo"))
			Expect(body.Limit).To(Equal(20))
			Expect(body.MaxKBResults).To(Equal(3))
			Expect(body.SynthesisModel).To(Equal("local-explainer"))

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"template":"temporal_activity","records":{"row_count":1,"duration_stats":[{"shortest_nonzero_duration":1}]}}`))
		}))
		defer server.Close()

		result, ok, err := runDeterministicForensicRoute(
			"shortest call duration of 923461678183",
			ForensicRecordsToolConfig{APIURL: server.URL, CollectionID: "records-demo", Model: "local-explainer"},
		)

		Expect(err).ToNot(HaveOccurred())
		Expect(ok).To(BeTrue())
		Expect(result.ToolName).To(Equal("forensic_hybrid_query"))
		Expect(result.ArgsJSON).To(ContainSubstring("923461678183"))
		Expect(result.ArgsJSON).To(ContainSubstring("shortest_call"))
		Expect(result.Text).To(ContainSubstring("temporal_activity"))
	})

	It("uses bounded model synthesis only when narrative explanation is requested", func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			Expect(r.Method).To(Equal(http.MethodPost))
			Expect(r.URL.Path).To(Equal("/query/hybrid"))

			var body ForensicHybridQueryArgs
			Expect(json.NewDecoder(r.Body).Decode(&body)).To(Succeed())
			Expect(body.Template).To(Equal("frequent_contacts"))
			Expect(body.SynthesisModel).To(Equal("local-explainer"))

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"template":"frequent_contacts","records":{"row_count":2}}`))
		}))
		defer server.Close()

		result, ok, err := runDeterministicForensicRoute(
			"explain and summarize the frequent contacts",
			ForensicRecordsToolConfig{APIURL: server.URL, CollectionID: "records-demo", Model: "local-explainer"},
		)

		Expect(err).ToNot(HaveOccurred())
		Expect(ok).To(BeTrue())
		Expect(result.ToolName).To(Equal("forensic_hybrid_query"))
	})

	It("directly routes natural raw-attribute questions to canonical records", func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			Expect(r.Method).To(Equal(http.MethodPost))
			Expect(r.URL.Path).To(Equal("/query/hybrid"))

			var body ForensicHybridQueryArgs
			Expect(json.NewDecoder(r.Body).Decode(&body)).To(Succeed())
			Expect(body.Query).To(Equal("show CDR records where call type is GPRS"))
			Expect(body.Template).To(Equal("canonical_records"))
			Expect(body.CollectionID).To(Equal("records-demo"))

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"template":"canonical_records","records":{"row_count":3,"total_count":5861,"canonical_records":[{"record_type":"cdr"}]}}`))
		}))
		defer server.Close()

		result, ok, err := runDeterministicForensicRoute(
			"show CDR records where call type is GPRS",
			ForensicRecordsToolConfig{APIURL: server.URL, CollectionID: "records-demo"},
		)

		Expect(err).ToNot(HaveOccurred())
		Expect(ok).To(BeTrue())
		Expect(result.ToolName).To(Equal("forensic_hybrid_query"))
		Expect(result.ArgsJSON).To(ContainSubstring("canonical_records"))
		Expect(result.Text).To(ContainSubstring("canonical_records"))
	})

	It("routes source file and quality questions but leaves upload help for normal chat", func() {
		Expect(isNaturalForensicQuery("which files were ingested?", strings.ToLower("which files were ingested?"))).To(BeTrue())
		Expect(forensicTemplateHint("which files were ingested?")).To(Equal("source_file_audit"))
		Expect(forensicTemplateHint("show duplicate uploads")).To(Equal("duplicate_upload_audit"))
		Expect(forensicTemplateHint("what limitations and missing data exist?")).To(Equal("limitations_and_data_quality"))
		Expect(forensicTemplateHint("show available entities")).To(Equal("entity_activity"))
		Expect(forensicTemplateHint("what should I investigate next?")).To(Equal("suspicious_patterns"))
		Expect(forensicTemplateHint("compare these numbers")).To(Equal("relationship_network"))
		Expect(isNaturalForensicQuery("show canonical records where raw_payload call_type is GPRS", strings.ToLower("show canonical records where raw_payload call_type is GPRS"))).To(BeTrue())
		Expect(forensicTemplateHint("show canonical records where raw_payload call_type is GPRS")).To(Equal("canonical_records"))
		Expect(isNaturalForensicQuery("show CDR records where duration seconds > 60", strings.ToLower("show CDR records where duration seconds > 60"))).To(BeTrue())
		Expect(forensicTemplateHint("show CDR records where duration seconds > 60")).To(Equal("canonical_records"))
		Expect(isNaturalForensicQuery("who are the frequent contacts?", strings.ToLower("who are the frequent contacts?"))).To(BeTrue())
		Expect(forensicTemplateHint("who are the frequent contacts?")).To(Equal("frequent_contacts"))
		Expect(isNaturalForensicQuery("what happened on 2026-07-10?", strings.ToLower("what happened on 2026-07-10?"))).To(BeTrue())
		Expect(forensicTemplateHint("what happened on 2026-07-10?")).To(Equal("entity_timeline"))
		Expect(forensicTemplateHint("923001234567 cdr actvty 10 july 2026 pls")).To(Equal("temporal_activity"))
		Expect(isNaturalForensicReportQuery("generate a report", strings.ToLower("generate a report"))).To(BeTrue())
		Expect(isNaturalForensicQuery("how do I upload evidence?", strings.ToLower("how do I upload evidence?"))).To(BeFalse())
	})

	DescribeTable("extracts diverse forensic targets without binding to demo identifiers",
		func(query string, expected string) {
			Expect(extractForensicTarget(query)).To(Equal(expected))
		},
		Entry("spaced phone number", "shortest call for +92 346 167 8183", "923461678183"),
		Entry("email", "show evidence for analyst.one@example.org", "analyst.one@example.org"),
		Entry("ipv4", "activity for 192.168.100.45", "192.168.100.45"),
		Entry("plate", "where was LEB 4321 seen", "LEB-4321"),
		Entry("plate with one trailing letter", "where was plate ZZ99Z seen", "ZZ99Z"),
		Entry("plate with two trailing letters", "where was plate ZZ99ZZ seen", "ZZ99ZZ"),
		Entry("plate with three trailing letters", "where was plate ZZ99ZZZ seen", "ZZ99ZZZ"),
		Entry("cell site", "tower activity for LHR-GUL-014", "LHR-GUL-014"),
		Entry("long segmented identifier", "show exact ANPR sightings for ZZZ-SYNTHETIC-NO-MATCH", "ZZZ-SYNTHETIC-NO-MATCH"),
		Entry("phone after named date", "10 july 2026 ko 923001234567 ki CDR activity dikhao", "923001234567"),
		Entry("date is not a target", "what happened on 2026-07-10", ""),
		Entry("compact date is not a target", "what happened on 20260710", ""),
		Entry("ignores canonical collection", "summarize records-demo", ""),
	)

	It("defers bounded subscriber and CDR compositions to the forensic sidecar", func() {
		query := "Show subscriber identity and CDR activity for 923001234567."
		Expect(deterministicForensicRouteRequested(query)).To(BeTrue())
		Expect(extractForensicTarget(query)).To(Equal("923001234567"))
		Expect(forensicTemplateHint(query)).To(BeEmpty())
		Expect(forensicTemplateHint("Show IPDR endpoint activity for 10.20.1.7.")).To(Equal("ipdr_endpoint_summary"))
	})

	It("directly routes natural report requests before LLM planning", func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			Expect(r.Method).To(Equal(http.MethodPost))
			Expect(r.URL.Path).To(Equal("/reports/generate"))

			var body ForensicReportArgs
			Expect(json.NewDecoder(r.Body).Decode(&body)).To(Succeed())
			Expect(body.Target).To(Equal("923461678183"))
			Expect(body.IncludeEvidence).To(BeTrue())

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"markdown":"# Forensic Intelligence Report\n\nExact records aggregates only."}`))
		}))
		defer server.Close()

		result, ok, err := runDeterministicForensicRoute(
			"generate report for 923461678183 using records evidence",
			ForensicRecordsToolConfig{APIURL: server.URL, CollectionID: "records-demo"},
		)

		Expect(err).ToNot(HaveOccurred())
		Expect(ok).To(BeTrue())
		Expect(result.ToolName).To(Equal("forensic_generate_report"))
		Expect(result.ArgsJSON).To(ContainSubstring("923461678183"))
		Expect(result.Text).To(ContainSubstring("Forensic Intelligence Report"))
	})

	It("formats direct hybrid results as safe analyst markdown", func() {
		text := formatDirectForensicResult("forensic_hybrid_query", "", map[string]any{
			"collection_id": "records-demo",
			"template":      "call_type_breakdown",
			"planner": map[string]any{
				"confidence": 0.919999999999,
			},
			"records": map[string]any{
				"row_count": 1,
				"call_type_breakdown": []any{
					map[string]any{"call_type": "SMS", "direction": "INCOMING", "event_count": 5, "ignored_column": "not rendered"},
				},
			},
			"evidence": map[string]any{
				"mode": "vector_search",
				"results": []any{
					map[string]any{"content": "# Records Handling Policy\nExact records analytics only.", "similarity": 0.8468971},
				},
			},
		})

		visibleSummary := strings.Split(text, "<details>")[0]
		Expect(visibleSummary).To(ContainSubstring("Planner confidence:** 0.92"))
		Expect(visibleSummary).To(ContainSubstring("| Call Type | Direction | Event Count |"))
		Expect(visibleSummary).ToNot(ContainSubstring("ignored_column"))
		Expect(visibleSummary).To(ContainSubstring("1. Records Handling Policy Exact records analytics only."))
		Expect(visibleSummary).ToNot(ContainSubstring("1. # Records"))
	})

	It("formats clarification responses without running records output", func() {
		text := formatDirectForensicResult("forensic_hybrid_query", "", map[string]any{
			"collection_id": "records-demo",
			"template":      "top_locations",
			"route":         []any{"clarification"},
			"answer": map[string]any{
				"clarification_required": true,
				"clarification":          "Which number, plate, IP, IMSI/IMEI, or cell-site ID should I analyze for location activity?",
				"limitations": []any{
					"A target-specific query was detected, but no target identifier was provided or extractable from the request.",
				},
			},
		})

		visibleSummary := strings.Split(text, "<details>")[0]
		Expect(visibleSummary).To(ContainSubstring("Clarification needed"))
		Expect(visibleSummary).To(ContainSubstring("Which number, plate, IP"))
		Expect(visibleSummary).To(ContainSubstring("Route:** clarification"))
		Expect(visibleSummary).ToNot(ContainSubstring("Structured Records"))
	})

	It("extracts KB search retry caps from collection-size errors", func() {
		Expect(kbSearchRetryCapFromError("nResults must be <= number of documents in the collection (2)", 10)).To(Equal(2))
		Expect(kbSearchRetryCapFromError("nResults must be <= the number of documents in the collection", 10)).To(Equal(0))
	})
})
