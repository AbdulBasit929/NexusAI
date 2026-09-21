package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Phase 5A forensic platform contracts", func() {
	It("loads one valid versioned catalog with structured and bounded multimodal adapters", func() {
		catalog, err := loadForensicPlatformCatalog()
		Expect(err).NotTo(HaveOccurred())
		Expect(catalog.SchemaVersion).To(Equal("forensics.platform/v1"))
		Expect(catalog.CatalogVersion).To(Equal("2026-08-28.nxb1.1"))
		Expect(catalog.AdapterLifecycle).To(Equal([]string{
			"detect", "validate", "plan", "extract", "normalize", "derive", "persist", "index", "verify", "capabilities",
		}))
		Expect(catalog.Adapters).To(HaveLen(9))
		Expect(catalog.Operations).To(HaveLen(104))
		Expect(catalog.ModelRoles).To(HaveLen(10))
		Expect(catalog.Agents).To(HaveLen(16))

		adapterByID := map[string]AdapterDescriptorV1{}
		for _, adapter := range catalog.Adapters {
			adapterByID[adapter.ID] = adapter
		}
		Expect(adapterByID["nexusai.adapter.cdr"].ImplementationKey).To(Equal("cdr"))
		Expect(adapterByID["nexusai.adapter.generic_tabular"].ImplementationKey).To(Equal("generic"))
		Expect(adapterByID["nexusai.adapter.cdr"].PreservesLegacyOutput).To(BeTrue())
		Expect(adapterByID["nexusai.adapter.cdr"].Version).To(Equal("1.4.0"))
		Expect(adapterByID["nexusai.adapter.cdr"].ImplementationStatus).To(Equal("operational_family_slice"))
		Expect(adapterByID["nexusai.adapter.ipdr"].ImplementationKey).To(Equal("ipdr"))
		Expect(adapterByID["nexusai.adapter.ipdr"].Version).To(Equal("1.2.0"))
		Expect(adapterByID["nexusai.adapter.ipdr"].ImplementationStatus).To(Equal("operational_family_slice"))
		Expect(adapterByID["nexusai.adapter.ipdr"].PreservesLegacyOutput).To(BeTrue())
		Expect(adapterByID["nexusai.adapter.anpr"].ImplementationKey).To(Equal("anpr"))
		Expect(adapterByID["nexusai.adapter.anpr"].Version).To(Equal("1.2.0"))
		Expect(adapterByID["nexusai.adapter.anpr"].ImplementationStatus).To(Equal("operational_family_slice"))
		Expect(adapterByID["nexusai.adapter.anpr"].PreservesLegacyOutput).To(BeTrue())
		Expect(adapterByID["nexusai.adapter.subscriber_identity"].ImplementationKey).To(Equal("subscriber"))
		Expect(adapterByID["nexusai.adapter.subscriber_identity"].Version).To(Equal("1.3.0"))
		Expect(adapterByID["nexusai.adapter.subscriber_identity"].ImplementationStatus).To(Equal("operational_family_slice"))
		Expect(adapterByID["nexusai.adapter.subscriber_identity"].PreservesLegacyOutput).To(BeTrue())
		Expect(adapterByID["nexusai.adapter.tower_location"].ImplementationKey).To(Equal("tower_location"))
		Expect(adapterByID["nexusai.adapter.tower_location"].Version).To(Equal("1.3.0"))
		Expect(adapterByID["nexusai.adapter.tower_location"].ImplementationStatus).To(Equal("operational_family_slice"))
		Expect(adapterByID["nexusai.adapter.tower_location"].PreservesLegacyOutput).To(BeTrue())
		Expect(adapterByID["nexusai.adapter.generic_tabular"].PreservesLegacyOutput).To(BeTrue())
		Expect(adapterByID["nexusai.adapter.image_media"].ImplementationStatus).To(Equal("limited_runtime_active"))
		Expect(adapterByID["nexusai.adapter.native_document"].OperationIDs).To(ContainElements("document.extract", "document.search"))
		operationByID := map[string]FamilyOperationDescriptorV1{}
		for _, operation := range catalog.Operations {
			operationByID[operation.ID] = operation
		}
		Expect(operationByID["image.compare"].Maturity).To(Equal("ENGINEERING_ONLY"))
		Expect(operationByID["image.ocr_search"].Maturity).To(Equal("LIMITED"))
		Expect(operationByID["image.visual_similarity"].Maturity).To(Equal("LIMITED"))
		Expect(operationByID["face.similarity"].Maturity).To(Equal("LIMITED"))
		Expect(operationByID["audio.roman_urdu"].Maturity).To(Equal("LIMITED"))
		Expect(operationByID["audio.transcript_search"].Maturity).To(Equal("LIMITED"))
		Expect(operationByID["document.extract"].Maturity).To(Equal("LIMITED"))
		Expect(operationByID["document.search"].Maturity).To(Equal("LIMITED"))
	})

	It("declares every specialist as case-bound and prevents peer delegation", func() {
		catalog, err := loadForensicPlatformCatalog()
		Expect(err).NotTo(HaveOccurred())
		manifests := map[string]SpecialistAgentManifestV1{}
		for _, manifest := range catalog.Agents {
			manifests[manifest.ID] = manifest
			if manifest.ID == "Case_Intelligence_Orchestrator" || manifest.ID == "Forensic_Records_Analyst" {
				continue
			}
			Expect(manifest.ScopeSource).To(Equal("orchestrator_bound_case"), manifest.ID)
			Expect(manifest.ToolAllowlist).NotTo(ContainElement("specialist.delegate"), manifest.ID)
			Expect(manifest.RequiredCitations).To(BeTrue(), manifest.ID)
		}
		Expect(manifests).To(HaveKey("Communications_CDR_Analyst"))
		Expect(manifests).To(HaveKey("Network_IPDR_Capture_Analyst"))
		Expect(manifests).To(HaveKey("Vehicle_ANPR_Geospatial_Analyst"))
		Expect(manifests).To(HaveKey("Subscriber_Identity_Analyst"))
		Expect(manifests).To(HaveKey("Tower_Location_Reference_Analyst"))
		Expect(manifests).To(HaveKey("Financial_Transactions_Analyst"))
		Expect(manifests).To(HaveKey("Access_Security_Log_Analyst"))
		Expect(manifests).To(HaveKey("Document_OCR_Analyst"))
		Expect(manifests).To(HaveKey("Audio_Speech_Analyst"))
		Expect(manifests).To(HaveKey("Image_Vision_Analyst"))
		Expect(manifests).To(HaveKey("Video_Timeline_Analyst"))
		Expect(manifests).To(HaveKey("Generic_Data_Quality_Analyst"))
		Expect(manifests).To(HaveKey("Database_Archive_Unknown_Evidence_Analyst"))
		Expect(manifests).To(HaveKey("Evidence_Integrity_And_Provenance_Analyst"))
		Expect(manifests["Document_OCR_Analyst"].Status).To(Equal("limited_runtime_active_native_text"))
		Expect(manifests["Document_OCR_Analyst"].OperationIDs).To(ContainElements("document.extract", "document.search"))
		Expect(manifests["Forensic_Records_Analyst"].Status).To(Equal("configured_compatibility_alias"))
		Expect(manifests["Communications_CDR_Analyst"].Version).To(Equal("1.2.0"))
		Expect(manifests["Communications_CDR_Analyst"].Status).To(Equal("operational_family_slice"))
		Expect(manifests["Communications_CDR_Analyst"].OperationIDs).To(ContainElements(
			"cdr.call_type_breakdown", "cdr.temporal_activity", "cdr.geospatial_movement", "cdr.device_identity_changes", "cdr.service_usage",
		))
		Expect(manifests["Network_IPDR_Capture_Analyst"].Version).To(Equal("1.1.0"))
		Expect(manifests["Network_IPDR_Capture_Analyst"].Status).To(Equal("operational_family_slice"))
		Expect(manifests["Network_IPDR_Capture_Analyst"].OperationIDs).To(ContainElements(
			"ipdr.endpoint_summary", "ipdr.domain_summary", "ipdr.protocol_breakdown", "ipdr.session_volume",
			"ipdr.subscriber_sessions", "ipdr.concurrent_sessions", "ipdr.timeline",
		))
		Expect(manifests["Vehicle_ANPR_Geospatial_Analyst"].Version).To(Equal("1.1.0"))
		Expect(manifests["Vehicle_ANPR_Geospatial_Analyst"].Status).To(Equal("operational_family_slice"))
		Expect(manifests["Vehicle_ANPR_Geospatial_Analyst"].OperationIDs).To(ContainElements(
			"anpr.sightings", "anpr.camera_sequence", "anpr.camera_activity", "anpr.co_travel",
			"anpr.route_timing", "anpr.plate_variants", "anpr.timeline",
		))
		Expect(manifests["Subscriber_Identity_Analyst"].Version).To(Equal("1.2.0"))
		Expect(manifests["Subscriber_Identity_Analyst"].Status).To(Equal("operational_family_slice"))
		Expect(manifests["Subscriber_Identity_Analyst"].FallbackAgentID).To(Equal("Forensic_Records_Analyst"))
		Expect(manifests["Subscriber_Identity_Analyst"].OperationIDs).To(ContainElements(
			"subscriber.identity_lookup", "subscriber.validity_timeline", "subscriber.device_links",
			"subscriber.status_summary", "subscriber.conflict_audit", "subscriber.reuse_candidates",
		))
		Expect(manifests["Tower_Location_Reference_Analyst"].Version).To(Equal("1.2.0"))
		Expect(manifests["Tower_Location_Reference_Analyst"].Status).To(Equal("operational_family_slice"))
		Expect(manifests["Tower_Location_Reference_Analyst"].FallbackAgentID).To(Equal("Forensic_Records_Analyst"))
		Expect(manifests["Tower_Location_Reference_Analyst"].OperationIDs).To(ContainElements(
			"tower.site_lookup", "tower.reference_timeline", "tower.coordinate_audit",
			"tower.status_summary", "tower.alias_conflicts", "tower.cdr_join",
		))
	})

	DescribeTable("serves typed read-only discovery sections",
		func(section, collectionKey string, expectedCount int) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/api/v1/forensics/"+section, nil)
			forensicPlatformDiscoveryHandler(section).ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusOK))

			var payload map[string]any
			Expect(json.Unmarshal(recorder.Body.Bytes(), &payload)).To(Succeed())
			Expect(payload["schema_version"]).To(Equal("forensics.platform/v1"))
			Expect(payload["catalog_version"]).To(Equal("2026-08-28.nxb1.1"))
			if collectionKey != "" {
				Expect(payload[collectionKey]).To(HaveLen(expectedCount))
			}
		},
		Entry("adapters", "adapters", "adapters", 9),
		Entry("operations", "operations", "operations", 104),
		Entry("agents", "agents", "agents", 16),
		Entry("contracts", "contracts", "", 0),
	)

	It("keeps discovery authenticated with the existing sidecar middleware", func() {
		mux := http.NewServeMux()
		mux.HandleFunc("GET /api/v1/forensics/adapters", forensicPlatformDiscoveryHandler("adapters"))
		handler := forensicAuthMiddleware(forensicAuthConfig{
			APIKey: "phase5a-key", Required: true, TrustedTenantID: "default",
		}, mux)

		missing := httptest.NewRecorder()
		handler.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/api/v1/forensics/adapters", nil))
		Expect(missing.Code).To(Equal(http.StatusUnauthorized))

		request := httptest.NewRequest(http.MethodGet, "/api/v1/forensics/adapters", nil)
		request.Header.Set("Authorization", "Bearer phase5a-key")
		request.Header.Set(forensicTenantHeader, "default")
		request.Header.Set(forensicActorHeader, "phase5a-tester")
		request.Header.Set(forensicSubjectHeader, "phase5a-tester")
		request.Header.Set(forensicActorRoleHeader, "user")
		authorized := httptest.NewRecorder()
		handler.ServeHTTP(authorized, request)
		Expect(authorized.Code).To(Equal(http.StatusOK))
	})

	It("publishes the read-only evidence reprocess plan contract without advertising execution", func() {
		recorder := httptest.NewRecorder()
		forensicPlatformDiscoveryHandler("contracts").ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/forensics/contracts", nil))
		Expect(recorder.Code).To(Equal(http.StatusOK))
		var payload map[string]any
		Expect(json.Unmarshal(recorder.Body.Bytes(), &payload)).To(Succeed())
		contracts := payload["workflow_contracts"].([]any)
		Expect(contracts).To(HaveLen(1))
		contract := contracts[0].(map[string]any)
		Expect(contract).To(HaveKeyWithValue("version", evidenceReprocessPlanContractV1))
		Expect(contract).To(HaveKeyWithValue("execution", "read_only_approval_required"))
		Expect(contract["path"]).To(ContainSubstring("/reprocess-plan"))
	})

	It("preserves the accepted legacy QueryPlan JSON shape as a golden contract", func() {
		payload, err := json.Marshal(QueryPlan{
			InterpretedIntent: "records",
			SelectedTemplate:  "frequent_contacts",
			TargetIdentifiers: []string{"923001234567"},
			DateBounds:        TimeRange{From: "2026-07-01", To: "2026-07-30"},
			AppliedFilters:    map[string]any{"target": "923001234567"},
			Confidence:        1,
			ExecutionStrategy: "sql_deterministic",
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(string(payload)).To(Equal(`{"interpreted_intent":"records","selected_template":"frequent_contacts","target_identifiers":["923001234567"],"date_bounds":{"from":"2026-07-01","to":"2026-07-30"},"applied_filters":{"target":"923001234567"},"confidence":1,"coverage_check":{"total_indexed_records":0,"record_families_present":null,"nearest_activity":{},"valid_target_examples":null},"execution_strategy":"sql_deterministic"}`))
	})

	It("keeps exact facts, cited evidence, inference, interpretation, and unsupported work separate", func() {
		response := EnterpriseResponseV1{
			ContractVersion: "forensics.enterprise-response/v1",
			RequestID:       "request-1",
			CaseID:          "case-1",
			CollectionID:    "collection-1",
			Status:          "answered_with_limitations",
			ExecutiveAnswer: "One deterministic CDR fact was found.",
			DeterministicFindings: []FindingV1{{
				ID: "finding-1", Statement: "One call matched.", Measures: map[string]any{"call_count": 1}, CitationIDs: []string{"citation-1"},
			}},
			SemanticEvidence: []CitationV1{{
				ID: "citation-1", TenantID: "default", CollectionID: "collection-1", EvidenceID: "evidence-1", VersionID: "version-1", Locator: "row:2", SourceName: "calls.csv",
			}},
			InferredRelationships: []InferredRelationshipV1{},
			UnsupportedOperations: []UnsupportedOperationV1{{
				OperationID: "audio.transcribe", Reason: "ASR is not accepted", RequiredCapability: "accepted ASR adapter and model",
			}},
			Limitations:    []string{"Only CDR evidence was queried."},
			NextActions:    []NextActionV1{},
			Tables:         []TableResultV1{},
			Visualizations: []VisualizationV1{},
			ExecutionTrace: ExecutionTraceV1{
				Route: []string{"records_sql"}, ToolIDs: []string{"cdr.frequent_contacts"}, AdapterIDs: []string{"nexusai.adapter.cdr"}, ModelRoles: []string{}, Parameters: map[string]any{"limit": 25}, LatencyMS: map[string]int64{"db": 4},
			},
		}
		payload, err := json.Marshal(response)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(payload)).To(ContainSubstring(`"deterministic_findings"`))
		Expect(string(payload)).To(ContainSubstring(`"semantic_evidence"`))
		Expect(string(payload)).To(ContainSubstring(`"inferred_relationships"`))
		Expect(string(payload)).NotTo(ContainSubstring(`"model_interpretation"`))
		Expect(string(payload)).To(ContainSubstring(`"unsupported_operations"`))

		var decoded map[string]any
		Expect(json.Unmarshal(payload, &decoded)).To(Succeed())
		Expect(decoded).To(HaveKeyWithValue("contract_version", "forensics.enterprise-response/v1"))
		Expect(decoded["executive_answer"]).To(Equal("One deterministic CDR fact was found."))
		Expect(strings.Join([]string{decoded["status"].(string), decoded["collection_id"].(string)}, "|")).To(Equal("answered_with_limitations|collection-1"))
	})
})

var _ = Describe("Phase 5B case-bound compatibility API", func() {
	It("rejects a body collection that conflicts with the authoritative URL case", func() {
		mux := http.NewServeMux()
		mux.HandleFunc("POST /api/v1/forensics/cases/{case_id}/query", forensicCaseQueryV1Handler(config{}, nil))
		request := httptest.NewRequest(http.MethodPost, "/api/v1/forensics/cases/case-alpha/query", strings.NewReader(`{"query":"which files were ingested?","collection_id":"case-beta"}`))
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, request)
		Expect(recorder.Code).To(Equal(http.StatusConflict))
		Expect(recorder.Body.String()).To(ContainSubstring("does not match URL case"))
	})

	It("maps a capability-guarded legacy result into enterprise-response/v1 without querying a database", func() {
		mux := http.NewServeMux()
		mux.HandleFunc("POST /api/v1/forensics/cases/{case_id}/query", forensicCaseQueryV1Handler(config{}, nil))
		request := httptest.NewRequest(http.MethodPost, "/api/v1/forensics/cases/case-alpha/query", strings.NewReader(`{"query":"diarize this audio recording","limit":20}`))
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, request)
		Expect(recorder.Code).To(Equal(http.StatusOK), recorder.Body.String())
		var response EnterpriseResponseV1
		Expect(json.Unmarshal(recorder.Body.Bytes(), &response)).To(Succeed())
		Expect(response.ContractVersion).To(Equal("forensics.enterprise-response/v1"))
		Expect(response.CaseID).To(Equal("case-alpha"))
		Expect(response.CollectionID).To(Equal("case-alpha"))
		Expect(response.UnsupportedOperations).NotTo(BeEmpty())
		Expect(response.ModelInterpretation).To(BeNil())
		Expect(response.ExecutionTrace.Route).To(ContainElement("capability_guard"))
	})

	It("enforces both authenticated collection and case headers against the URL", func() {
		mux := http.NewServeMux()
		mux.HandleFunc("POST /api/v1/forensics/cases/{case_id}/query", forensicCaseQueryV1Handler(config{}, nil))
		handler := forensicAuthMiddleware(forensicAuthConfig{APIKey: "phase5b-key", Required: true, TrustedTenantID: "default"}, mux)
		request := httptest.NewRequest(http.MethodPost, "/api/v1/forensics/cases/case-alpha/query", strings.NewReader(`{"query":"transcribe this audio recording"}`))
		request.Header.Set("Authorization", "Bearer phase5b-key")
		request.Header.Set(forensicTenantHeader, "default")
		request.Header.Set(forensicActorHeader, "phase5b-tester")
		request.Header.Set(forensicSubjectHeader, "phase5b-tester")
		request.Header.Set(forensicActorRoleHeader, "user")
		request.Header.Set(forensicCollectionHeader, "case-alpha")
		request.Header.Set(forensicCaseHeader, "case-beta")
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		Expect(recorder.Code).To(Equal(http.StatusForbidden))
		Expect(recorder.Body.String()).To(ContainSubstring("case mismatch"))
	})

	It("keeps synthesis fallback separate from successful model interpretation", func() {
		legacy := hybridQueryResponse{
			CollectionID: "case-alpha", Template: "collection_overview", Route: []string{"records_sql"},
			Answer:     map[string]any{"records_summary": "One row found.", "llm_fallback_reason": "model unavailable"},
			Enterprise: map[string]any{"status": "answered_with_limitations", "summary": "One row found.", "limitations": []string{"model unavailable"}},
			Telemetry:  QueryTelemetry{RequestID: "request-1", DBLatencyMS: 4},
		}
		response := legacyHybridToEnterpriseV1("case-alpha", "request-1", map[string]any{"limit": 20}, legacy)
		Expect(response.ModelInterpretation).To(BeNil())
		Expect(response.Limitations).To(ContainElement("model unavailable"))
		Expect(response.ExecutionTrace.LatencyMS).To(HaveKeyWithValue("db", int64(4)))
	})

	It("adapts an accepted typed query plan to the existing deterministic template", func() {
		legacy, early, err := normalizeV1QueryPlan("case-alpha", "request-typed", map[string]any{
			"contract_version": "forensics.query-plan/v1",
			"tenant_id":        "default",
			"case_id":          "case-alpha",
			"collection_id":    "case-alpha",
			"intent":           "cdr.frequent_contacts",
			"families":         []any{"communications_cdr"},
			"entities":         []any{map[string]any{"type": "msisdn", "value": "923001234567"}},
			"time_range":       map[string]any{"from": "2026-07-01", "to": "2026-07-30", "timezone": "Asia/Karachi"},
			"filters":          []any{map[string]any{"field": "direction", "op": "eq", "value": "outgoing"}},
			"measures":         []any{"total interactions", "incoming count", "outgoing count", "first contact", "last contact"},
			"group_by":         []any{"counterparty"},
			"limit":            25,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(early).To(BeNil())
		Expect(legacy).To(HaveKeyWithValue("template", "frequent_contacts"))
		Expect(legacy).To(HaveKeyWithValue("record_type", "cdr"))
		Expect(legacy).To(HaveKeyWithValue("target", "923001234567"))
		Expect(legacy).To(HaveKeyWithValue("date_from", "2026-07-01"))
		Expect(legacy).To(HaveKeyWithValue("direction", "OUTGOING"))
		Expect(legacy).NotTo(HaveKey("raw_payload_filters"))
	})

	DescribeTable("maps every Phase 6.1 CDR query operation to a bounded deterministic template",
		func(operationID, template string) {
			legacy, early, err := normalizeV1QueryPlan("case-alpha", "request-cdr", map[string]any{
				"contract_version": "forensics.query-plan/v1", "intent": operationID,
				"families": []any{"communications_cdr"}, "entities": []any{map[string]any{"type": "msisdn", "value": "923001234567"}}, "limit": 25,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(early).To(BeNil())
			Expect(legacy).To(HaveKeyWithValue("template", template))
			Expect(legacy).To(HaveKeyWithValue("record_type", "cdr"))
		},
		Entry("frequent contacts", "cdr.frequent_contacts", "frequent_contacts"),
		Entry("call types", "cdr.call_type_breakdown", "call_type_breakdown"),
		Entry("temporal activity", "cdr.temporal_activity", "temporal_activity"),
		Entry("durations", "cdr.duration_extremes", "duration_extremes"),
		Entry("timeline", "cdr.timeline", "entity_timeline"),
		Entry("movement", "cdr.geospatial_movement", "geospatial_movement"),
		Entry("identity changes", "cdr.device_identity_changes", "device_identity_changes"),
		Entry("service usage", "cdr.service_usage", "service_usage"),
		Entry("tower activity", "cdr.tower_activity", "tower_activity"),
	)

	DescribeTable("maps every Phase 6.2 IPDR query operation to a bounded deterministic template",
		func(operationID, template string) {
			legacy, early, err := normalizeV1QueryPlan("case-alpha", "request-ipdr", map[string]any{
				"contract_version": "forensics.query-plan/v1", "intent": operationID,
				"families": []any{"network_ipdr"}, "entities": []any{map[string]any{"type": "subscriber_identifier", "value": "923001234567"}}, "limit": 25,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(early).To(BeNil())
			Expect(legacy).To(HaveKeyWithValue("template", template))
			Expect(legacy).To(HaveKeyWithValue("record_type", "ipdr"))
		},
		Entry("endpoints", "ipdr.endpoint_summary", "ipdr_endpoint_summary"),
		Entry("domains", "ipdr.domain_summary", "ipdr_domain_summary"),
		Entry("protocols", "ipdr.protocol_breakdown", "ipdr_protocol_breakdown"),
		Entry("volume", "ipdr.session_volume", "ipdr_session_volume"),
		Entry("subscriber", "ipdr.subscriber_sessions", "ipdr_subscriber_sessions"),
		Entry("overlap", "ipdr.concurrent_sessions", "ipdr_concurrent_sessions"),
		Entry("timeline", "ipdr.timeline", "ipdr_timeline"),
	)

	DescribeTable("maps every Phase 6.3 ANPR query operation to a bounded deterministic template",
		func(operationID, template string) {
			legacy, early, err := normalizeV1QueryPlan("case-alpha", "request-anpr", map[string]any{
				"contract_version": "forensics.query-plan/v1", "intent": operationID,
				"families": []any{"anpr_vehicles"}, "entities": []any{map[string]any{"type": "plate", "value": "ABC-123"}}, "limit": 25,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(early).To(BeNil())
			Expect(legacy).To(HaveKeyWithValue("template", template))
			Expect(legacy).To(HaveKeyWithValue("record_type", "anpr"))
		},
		Entry("sightings", "anpr.sightings", "anpr_sightings"),
		Entry("camera sequence", "anpr.camera_sequence", "anpr_camera_sequence"),
		Entry("camera activity", "anpr.camera_activity", "anpr_camera_activity"),
		Entry("co-travel", "anpr.co_travel", "anpr_co_travel"),
		Entry("route timing", "anpr.route_timing", "anpr_route_timing"),
		Entry("plate variants", "anpr.plate_variants", "anpr_plate_variants"),
		Entry("timeline", "anpr.timeline", "anpr_timeline"),
	)

	It("requires an explicit plate for bounded ANPR sequence operations", func() {
		for _, operationID := range []string{"anpr.camera_sequence", "anpr.co_travel", "anpr.route_timing", "anpr.plate_variants", "anpr.timeline"} {
			_, response, err := normalizeV1QueryPlan("case-alpha", "request-anpr-target", map[string]any{
				"contract_version": "forensics.query-plan/v1", "intent": operationID, "families": []any{"anpr_vehicles"},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(response).NotTo(BeNil())
			Expect(response.Status).To(Equal("needs_input"))
			Expect(response.ExecutionTrace.Route).To(Equal([]string{"clarification"}))
			Expect(response.Limitations).To(ContainElement(ContainSubstring("does not infer vehicle ownership")))
		}
	})

	It("does not convert a target-free ANPR operation ID into a plate filter", func() {
		legacy, response, err := normalizeV1QueryPlan("case-alpha", "request-anpr-activity", map[string]any{
			"contract_version": "forensics.query-plan/v1",
			"tenant_id":        "default",
			"case_id":          "case-alpha",
			"collection_id":    "case-alpha",
			"intent":           "anpr.camera_activity",
			"families":         []any{"anpr_vehicles"},
			"entities":         []any{},
			"limit":            float64(20),
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(response).To(BeNil())
		Expect(legacy).To(HaveKeyWithValue("template", "anpr_camera_activity"))
		Expect(legacy).NotTo(HaveKey("target"))
		Expect(stringValueAny(legacy["query"])).To(Equal("execute typed forensic operation anpr camera activity"))
	})

	It("requires an explicit target for bounded subscriber, overlap, and IPDR timeline operations", func() {
		for _, operationID := range []string{"ipdr.subscriber_sessions", "ipdr.concurrent_sessions", "ipdr.timeline"} {
			_, response, err := normalizeV1QueryPlan("case-alpha", "request-ipdr-target", map[string]any{
				"contract_version": "forensics.query-plan/v1", "intent": operationID, "families": []any{"network_ipdr"},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(response).NotTo(BeNil())
			Expect(response.Status).To(Equal("needs_input"))
			Expect(response.ExecutionTrace.Route).To(Equal([]string{"clarification"}))
			Expect(response.Limitations).To(ContainElement(ContainSubstring("does not infer subscriber-to-IP ownership")))
		}
	})

	It("requires an originating subscriber/device target before comparing IMEI or IMSI", func() {
		_, response, err := normalizeV1QueryPlan("case-alpha", "request-device", map[string]any{
			"contract_version": "forensics.query-plan/v1", "intent": "cdr.device_identity_changes", "families": []any{"communications_cdr"},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(response).NotTo(BeNil())
		Expect(response.Status).To(Equal("needs_input"))
		Expect(response.ExecutionTrace.Route).To(Equal([]string{"clarification"}))
		Expect(response.Limitations).To(ContainElement(ContainSubstring("Dialed counterparties")))
	})

	It("adds a CDR specialist trace, deterministic citation, finding, and typed visualization", func() {
		legacy := hybridQueryResponse{
			CollectionID: "case-alpha", Template: "frequent_contacts", Route: []string{"records_sql"},
			Enterprise: map[string]any{
				"status": "answered", "summary": "Two contacts were ranked.",
				"data_grid": map[string]any{
					"columns": []map[string]any{{"key": "dialed_number", "label": "Dialed number", "type": "string"}, {"key": "total_interactions", "label": "Interactions", "type": "integer"}},
					"rows":    []map[string]any{{"dialed_number": "923001111111", "total_interactions": 5}, {"dialed_number": "923002222222", "total_interactions": 3}},
				},
			},
		}
		response := legacyHybridToEnterpriseV1("case-alpha", "request-cdr", map[string]any{"tenant_id": "default", "limit": 20}, legacy)
		Expect(response.ExecutionTrace.AdapterIDs).To(ContainElement("nexusai.adapter.cdr"))
		Expect(response.ExecutionTrace.ToolIDs).To(ContainElement("cdr.frequent_contacts"))
		Expect(response.SemanticEvidence).To(HaveLen(1))
		Expect(response.SemanticEvidence[0].Locator).To(ContainSubstring("table:forensic.cdr_records"))
		Expect(response.DeterministicFindings).To(HaveLen(1))
		Expect(response.DeterministicFindings[0].CitationIDs).To(ContainElement(response.SemanticEvidence[0].ID))
		Expect(response.Visualizations).To(HaveLen(1))
		Expect(response.Visualizations[0].Type).To(Equal("bar"))
		Expect(response.Visualizations[0].Spec).To(HaveKeyWithValue("value_field", "total_interactions"))
	})

	It("adds an IPDR specialist trace, deterministic citation, finding, and typed visualization", func() {
		legacy := hybridQueryResponse{
			CollectionID: "case-alpha", Template: "ipdr_protocol_breakdown", Route: []string{"records_sql"},
			Enterprise: map[string]any{
				"status": "answered", "summary": "Two explicit protocols were counted.",
				"data_grid": map[string]any{
					"columns": []map[string]any{{"key": "protocol", "label": "Protocol", "type": "string"}, {"key": "session_count", "label": "Sessions", "type": "integer"}},
					"rows":    []map[string]any{{"protocol": "TCP", "session_count": 7}, {"protocol": "UDP", "session_count": 3}},
				},
			},
		}
		response := legacyHybridToEnterpriseV1("case-alpha", "request-ipdr", map[string]any{"tenant_id": "default", "limit": 20}, legacy)
		Expect(response.ExecutionTrace.AdapterIDs).To(ContainElement("nexusai.adapter.ipdr"))
		Expect(response.ExecutionTrace.ToolIDs).To(ContainElement("ipdr.protocol_breakdown"))
		Expect(response.SemanticEvidence).To(HaveLen(1))
		Expect(response.SemanticEvidence[0].Locator).To(ContainSubstring("record_type:ipdr"))
		Expect(response.DeterministicFindings).To(HaveLen(1))
		Expect(response.Visualizations).To(HaveLen(1))
		Expect(response.Visualizations[0].Type).To(Equal("bar"))
		Expect(response.Visualizations[0].Spec).To(HaveKeyWithValue("value_field", "session_count"))
	})

	It("does not emit an empty IPDR visualization when an exact operation returns no rows", func() {
		visualization := ipdrVisualizationForTemplate("ipdr_concurrent_sessions", []map[string]any{}, []string{"citation-1"})
		Expect(visualization.ID).To(BeEmpty())
		Expect(visualization.Type).To(BeEmpty())
	})

	It("adds an ANPR specialist trace, bounded limitation, citation, and typed visualization", func() {
		legacy := hybridQueryResponse{
			CollectionID: "case-alpha", Template: "anpr_camera_activity", Route: []string{"records_sql"},
			Enterprise: map[string]any{
				"status": "answered", "summary": "Two cameras were counted.",
				"data_grid": map[string]any{
					"columns": []map[string]any{{"key": "camera_id", "label": "Camera", "type": "string"}, {"key": "sighting_count", "label": "Sightings", "type": "integer"}},
					"rows":    []map[string]any{{"camera_id": "CAM-7", "sighting_count": 9}, {"camera_id": "CAM-9", "sighting_count": 6}},
				},
			},
		}
		response := legacyHybridToEnterpriseV1("case-alpha", "request-anpr", map[string]any{"tenant_id": "default", "limit": 20}, legacy)
		Expect(response.ExecutionTrace.AdapterIDs).To(ContainElement("nexusai.adapter.anpr"))
		Expect(response.ExecutionTrace.ToolIDs).To(ContainElement("anpr.camera_activity"))
		Expect(response.SemanticEvidence).To(HaveLen(1))
		Expect(response.SemanticEvidence[0].Locator).To(ContainSubstring("record_type:anpr"))
		Expect(response.DeterministicFindings).To(HaveLen(1))
		Expect(response.Visualizations).To(HaveLen(1))
		Expect(response.Visualizations[0].Type).To(Equal("bar"))
		Expect(response.Visualizations[0].Spec).To(HaveKeyWithValue("value_field", "sighting_count"))
		Expect(response.Limitations).To(ContainElement(ContainSubstring("vehicle ownership")))
	})

	It("does not emit an empty ANPR visualization", func() {
		visualization := anprVisualizationForTemplate("anpr_timeline", []map[string]any{}, []string{"citation-1"})
		Expect(visualization.ID).To(BeEmpty())
	})

	It("publishes synchronized uncertainty-aware tower join map and timeline views", func() {
		rows := []map[string]any{{
			"cdr_observed_at": "2026-08-01T10:15:00Z", "cell_site_id": "PK-LHR-SYN-001",
			"latitude": 31.5204, "longitude": 74.3587, "coordinate_datum": "WGS84",
			"uncertainty_radius_m": 125, "uncertainty_class": "provider_supplied",
			"match_status": "ambiguous_overlapping_references", "cdr_source_file": "synthetic-cdr.csv", "cdr_row_number": 4,
		}}
		views := towerVisualizationsForTemplate("tower_cdr_join", rows, []string{"citation-tower-1"})
		Expect(views).To(HaveLen(2))
		Expect(views[0].Type).To(Equal("map"))
		Expect(views[1].Type).To(Equal("timeline"))
		Expect(views[0].Spec).To(HaveKeyWithValue("uncertainty_field", "uncertainty_radius_m"))
		Expect(views[0].Spec).To(HaveKeyWithValue("match_status_field", "match_status"))
		Expect(views[0].Spec).To(HaveKeyWithValue("route_inference", false))
		Expect(views[0].Spec).To(HaveKeyWithValue("rf_presence_inference", false))
		Expect(views[1].CitationIDs).To(Equal([]string{"citation-tower-1"}))
	})

	It("does not emit empty telecom visualization shells", func() {
		Expect(cdrVisualizationForTemplate("tower_activity", nil, []string{"citation-1"}).ID).To(BeEmpty())
		Expect(towerVisualizationsForTemplate("tower_cdr_join", nil, []string{"citation-1"})).To(BeNil())
	})

	It("returns typed unsupported and clarification responses without executing tools", func() {
		_, unsupported, err := normalizeV1QueryPlan("case-alpha", "request-unsupported", map[string]any{
			"contract_version": "forensics.query-plan/v1", "intent": "cdr.normalize_records",
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(unsupported.Status).To(Equal("unsupported"))
		Expect(unsupported.UnsupportedOperations).To(HaveLen(1))
		Expect(unsupported.ExecutionTrace.Route).To(Equal([]string{"capability_guard"}))

		_, needsInput, err := normalizeV1QueryPlan("case-alpha", "request-input", map[string]any{
			"contract_version": "forensics.query-plan/v1", "intent": "cdr.timeline",
			"clarifications": []any{map[string]any{"field": "target", "question": "Which subscriber identifier?"}},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(needsInput.Status).To(Equal("needs_input"))
		Expect(needsInput.Limitations).To(ContainElement("Which subscriber identifier?"))
		Expect(needsInput.ExecutionTrace.Route).To(Equal([]string{"clarification"}))
	})

	It("declares every reconciliation surface and zero deletion in the sidecar manifest type", func() {
		zero := int64(0)
		manifest := collectionManifestV1{
			ContractVersion: caseGovernanceContractV1, CaseID: "case-alpha", CollectionID: "case-alpha", ReadOnly: true, DeletionCount: 0,
			Resources: []manifestItemV1{
				{Resource: "kb_entries", Status: "external"}, {Resource: "vectors", Status: "external"},
				{Resource: "evidence_items", Count: &zero, Status: "counted"}, {Resource: "canonical_records", Count: &zero, Status: "counted"},
				{Resource: "ingest_jobs", Count: &zero, Status: "counted"}, {Resource: "agents", Status: "external"},
				{Resource: "reports", Status: "not_persisted"}, {Resource: "sources", Status: "external"},
				{Resource: "audit_events", Count: &zero, Status: "counted"},
			},
		}
		resourceNames := []string{}
		for _, resource := range manifest.Resources {
			resourceNames = append(resourceNames, resource.Resource)
		}
		Expect(resourceNames).To(ConsistOf("kb_entries", "vectors", "evidence_items", "canonical_records", "ingest_jobs", "agents", "reports", "sources", "audit_events"))
		Expect(manifest.ReadOnly).To(BeTrue())
		Expect(manifest.DeletionCount).To(BeZero())
	})

	It("publishes executable family-template inputs, calculations, and inference limits", func() {
		entries := supportedQueryTemplates()
		byName := map[string]queryTemplateCatalogEntry{}
		for _, entry := range entries {
			byName[entry.Name] = entry
		}

		cdr := byName["device_identity_changes"]
		Expect(cdr.OperationID).To(Equal("cdr.device_identity_changes"))
		Expect(cdr.FamilyID).To(Equal("communications_cdr"))
		Expect(cdr.Inputs).NotTo(BeEmpty())
		Expect(cdr.Inputs[0].Name).To(Equal("target"))
		Expect(cdr.Inputs[0].Required).To(BeTrue())
		Expect(cdr.Inputs[0].AcceptedKinds).To(ContainElement("MSISDN"))
		Expect(cdr.Calculation).To(ContainSubstring("preceding cited row"))

		ipdr := byName["ipdr_domain_summary"]
		Expect(ipdr.OperationID).To(Equal("ipdr.domain_summary"))
		Expect(ipdr.Measures).To(ContainElement("session count"))
		Expect(ipdr.ExampleQuery).NotTo(BeEmpty())
		Expect(ipdr.Limitations).To(ContainElement(ContainSubstring("ownership")))

		anpr := byName["anpr_route_timing"]
		Expect(anpr.OperationID).To(Equal("anpr.route_timing"))
		Expect(anpr.Inputs[0].Required).To(BeTrue())
		Expect(anpr.OutputDescription).To(ContainSubstring("straight-line distance"))
		Expect(anpr.Limitations).To(ContainElement(ContainSubstring("No road route")))
	})
})
