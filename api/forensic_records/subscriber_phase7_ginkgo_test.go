package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type subscriberQueryGoldenPack struct {
	SchemaVersion string                      `json:"schema_version"`
	QueryCases    []subscriberQueryGoldenCase `json:"query_cases"`
}

type subscriberQueryGoldenCase struct {
	Language              string `json:"language"`
	Query                 string `json:"query"`
	Template              string `json:"template"`
	Target                string `json:"target"`
	ClarificationRequired bool   `json:"clarification_required"`
}

var _ = Describe("Phase 7.3 subscriber, SIM, device, and service semantics", func() {
	loadQueryGoldens := func() subscriberQueryGoldenPack {
		path := filepath.Join("..", "..", "tests", "fixtures", "forensic_modalities", "pakistan_subscriber_query_goldens_v1.json")
		data, err := os.ReadFile(path)
		Expect(err).NotTo(HaveOccurred())
		var pack subscriberQueryGoldenPack
		Expect(json.Unmarshal(data, &pack)).To(Succeed())
		Expect(pack.SchemaVersion).To(Equal("1.0.0"))
		Expect(pack.QueryCases).To(HaveLen(24))
		return pack
	}

	It("routes the governed English, Roman Urdu, and Urdu analyst corpus", func() {
		for _, golden := range loadQueryGoldens().QueryCases {
			plan := planRuntimeQuery(hybridQueryRequest{Query: golden.Query})
			Expect(plan.Template).To(Equal(golden.Template), "%s: %s", golden.Language, golden.Query)
			Expect(plan.Target).To(Equal(golden.Target), "%s: %s", golden.Language, golden.Query)
			Expect(needsClarification(golden.Query, plan.Template, plan.Target)).To(Equal(golden.ClarificationRequired), "%s: %s", golden.Language, golden.Query)
		}
	})

	DescribeTable("routes real-world raw questions to bounded subscriber operations",
		func(query, expected string) {
			Expect(chooseTemplate(query, "")).To(Equal(expected))
		},
		Entry("exact lookup", "look up subscriber identity for PK-SUB-SYN-001", "subscriber_identity_lookup"),
		Entry("validity", "show subscriber activation and deactivation for 923001234567", "subscriber_validity_timeline"),
		Entry("device links", "show subscriber SIM and device links for 410010123456789", "subscriber_device_links"),
		Entry("status", "show active and suspended subscriber statuses", "subscriber_status_summary"),
		Entry("conflicts", "audit conflicting subscriber identity attributes", "subscriber_conflict_audit"),
		Entry("reuse", "show subscriber identifier reuse candidates", "subscriber_reuse_candidates"),
	)

	It("requires an exact non-CNIC target for identity, validity, and device workflows", func() {
		for _, template := range []string{"subscriber_identity_lookup", "subscriber_validity_timeline", "subscriber_device_links"} {
			Expect(needsClarification("execute subscriber workflow", template, "")).To(BeTrue(), template)
		}
		Expect(clarificationQuestion("", "subscriber_identity_lookup")).To(ContainSubstring("MSISDN"))
		Expect(clarificationQuestion("", "subscriber_identity_lookup")).To(ContainSubstring("Full CNIC"))
		Expect(isAllowedSubscriberRawTarget("0300-1234567")).To(BeTrue())
		Expect(isAllowedSubscriberRawTarget("+92 300 1234567")).To(BeTrue())
		Expect(isAllowedSubscriberRawTarget("PK-SUB-SYN-001")).To(BeTrue())
		Expect(isAllowedSubscriberRawTarget("410010123456789")).To(BeTrue())
		Expect(isAllowedSubscriberRawTarget("490154203237518")).To(BeTrue())
		Expect(isAllowedSubscriberRawTarget("999999999999")).To(BeTrue())
		Expect(isAllowedSubscriberRawTarget("00000-1234567-1")).To(BeFalse())
		Expect(needsClarification("look up subscriber identity for 00000-1234567-1", "subscriber_identity_lookup", "00000-1234567-1")).To(BeTrue())
	})

	It("rejects typed CNIC and name targets without echoing their values", func() {
		for _, entity := range []map[string]any{
			{"type": "cnic", "value": "00000-1234567-1"},
			{"type": "subscriber_name", "value": "Synthetic Person"},
		} {
			_, response, err := normalizeV1QueryPlan("case-alpha", "request-subscriber-target-type", map[string]any{
				"contract_version": "forensics.query-plan/v1", "intent": "subscriber.identity_lookup",
				"families": []any{"subscriber_identity"}, "entities": []any{entity},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(response).NotTo(BeNil())
			Expect(response.Status).To(Equal("needs_input"))
			Expect(response.ExecutionTrace.Route).To(Equal([]string{"clarification"}))
			encoded, marshalErr := json.Marshal(response)
			Expect(marshalErr).NotTo(HaveOccurred())
			Expect(string(encoded)).NotTo(ContainSubstring(stringValueAny(entity["value"])))
		}
		Expect(subscriberTypedEntitiesAllowed([]map[string]any{{"type": "msisdn", "value": "03001234567"}})).To(BeTrue())
		Expect(subscriberTypedEntitiesAllowed([]map[string]any{{"type": "iccid", "value": "8992410000000000001"}})).To(BeTrue())
		Expect(subscriberTypedEntitiesAllowed([]map[string]any{{"type": "service_reference", "value": "PK-SVC-SYN-001"}})).To(BeTrue())
		Expect(subscriberTypedEntitiesAllowed([]map[string]any{{"type": "cnic", "value": "0000012345671"}})).To(BeFalse())
		Expect(isAllowedSubscriberRawTarget("8992410000000000001")).To(BeTrue())
	})

	It("removes a raw CNIC-shaped target before response and audit export construction", func() {
		payload := []byte(`{"tenant_id":"default","collection_id":"case-alpha","query":"look up subscriber identity","template":"subscriber_identity_lookup","target":"00000-1234567-1"}`)
		request := httptest.NewRequest(http.MethodPost, "/query/hybrid", bytes.NewReader(payload))
		recorder := httptest.NewRecorder()
		hybridQueryHandler(config{}, nil).ServeHTTP(recorder, request)

		Expect(recorder.Code).To(Equal(http.StatusOK))
		Expect(recorder.Body.String()).NotTo(ContainSubstring("00000-1234567-1"))
		Expect(recorder.Body.String()).NotTo(ContainSubstring("0000012345671"))
		Expect(recorder.Body.String()).To(ContainSubstring("rejected sensitive target was removed"))
		Expect(recorder.Body.String()).To(ContainSubstring(`"route":["clarification"]`))
	})

	It("publishes executable inputs, calculations, and ownership boundaries", func() {
		entries := supportedQueryTemplates()
		byName := map[string]queryTemplateCatalogEntry{}
		for _, entry := range entries {
			byName[entry.Name] = entry
		}
		lookup := byName["subscriber_identity_lookup"]
		Expect(lookup.OperationID).To(Equal("subscriber.identity_lookup"))
		Expect(lookup.FamilyID).To(Equal("subscriber_identity"))
		Expect(lookup.Inputs[0].Required).To(BeTrue())
		Expect(lookup.Inputs[0].AcceptedKinds).To(ConsistOf("MSISDN", "subscriber reference", "service reference", "IMSI", "ICCID", "IMEI"))
		Expect(lookup.Calculation).To(ContainSubstring("CNIC is masked"))
		Expect(lookup.Limitations).To(ContainElement(ContainSubstring("not proof")))

		reuse := byName["subscriber_reuse_candidates"]
		Expect(reuse.OperationID).To(Equal("subscriber.reuse_candidates"))
		Expect(reuse.Inputs[0].Required).To(BeFalse())
		Expect(reuse.Limitations).To(ContainElement(ContainSubstring("do not prove")))
	})

	It("keeps default query output privacy-safe and exact", func() {
		normalizedSQL := strings.ToLower(subscriberObservationScopeSQL)
		Expect(normalizedSQL).To(ContainSubstring("cnic_masked"))
		Expect(normalizedSQL).To(ContainSubstring("subscriber_name_present"))
		Expect(normalizedSQL).To(ContainSubstring("iccid_raw"))
		Expect(normalizedSQL).To(ContainSubstring("service_identifier"))
		Expect(normalizedSQL).To(ContainSubstring("association_basis"))
		Expect(normalizedSQL).NotTo(ContainSubstring(" as subscriber_name,"))
		Expect(subscriberExactTargetSQL).To(ContainSubstring("lower(coalesce(msisdn"))
		Expect(subscriberExactTargetSQL).NotTo(ContainSubstring("cnic"))
		Expect(subscriberExactTargetSQL).NotTo(ContainSubstring("subscriber_name"))
		Expect(subscriberExactTargetSQL).To(ContainSubstring("coalesce(iccid"))
		Expect(subscriberExactTargetSQL).To(ContainSubstring("coalesce(service_identifier"))
	})

	It("applies bounded limit and offset pagination to every subscriber operation", func() {
		Expect(subscriberPaginationSQL).To(ContainSubstring("LIMIT $6 OFFSET $7"))
		data, err := os.ReadFile("query.go")
		Expect(err).NotTo(HaveOccurred())
		Expect(strings.Count(string(data), "+subscriberPaginationSQL")).To(Equal(6))
	})

	It("redacts protected subscriber fields from canonical API and export rows", func() {
		rows := []map[string]any{{
			"record_type":      "subscriber",
			"secondary_target": "00000-1234567-1",
			"raw_payload": map[string]any{
				"mobile_no": "03001234567", "full_name": "Synthetic Person", "cnic_no": "00000-1234567-1",
			},
			"metadata": map[string]any{"normalized_fields": map[string]any{
				"subscriber_reference": "PK-SUB-SYN-001", "subscriber_name": "Synthetic Person",
				"subscriber_name_search_key": "synthetic person", "subscriber_name_script": "latin",
				"cnic_raw": "00000-1234567-1", "cnic_digits": "0000012345671", "cnic_masked": "*********5671",
			}},
		}}
		redactSubscriberCanonicalRows(rows)
		encoded, err := json.Marshal(rows)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(encoded)).NotTo(ContainSubstring("00000-1234567-1"))
		Expect(string(encoded)).NotTo(ContainSubstring("0000012345671"))
		Expect(string(encoded)).NotTo(ContainSubstring("Synthetic Person"))
		Expect(rows[0]).To(HaveKeyWithValue("secondary_target", "PK-SUB-SYN-001"))
		Expect(string(encoded)).To(ContainSubstring("*********5671"))
		Expect(string(encoded)).To(ContainSubstring("subscriber_name_present"))
	})

	It("masks CNIC values derived during cross-family correlation", func() {
		relations := aggregateCrossFamilyRelations([]map[string]any{{
			"requested_target": "923001234567", "related_entity_type": "identity",
			"related_entity_value": "35202-1234567-1", "related_source_field": "cnic_raw",
		}}, 1)
		Expect(relations).To(HaveLen(1))
		Expect(relations[0]).To(HaveKeyWithValue("related_entity_value", "*********5671"))
		Expect(fmt.Sprint(relations)).NotTo(ContainSubstring("3520212345671"))
	})

	It("uses a dedicated bounded synthesis policy", func() {
		prompt := forensicSynthesisSystemPrompt("subscriber_conflict_audit")
		Expect(prompt).To(ContainSubstring("never expose a full CNIC"))
		Expect(prompt).To(ContainSubstring("never expose"))
		Expect(prompt).To(ContainSubstring("fraud"))
		Expect(prompt).To(ContainSubstring("touching or consecutive windows are not overlap"))
		Expect(validateForensicSynthesisSummary("subscriber_identity_lookup", "Model Interpretation: Supplied rows share an explicit reference. Limitations: This does not establish attribution.")).To(BeEmpty())
		Expect(validateForensicSynthesisSummary("subscriber_identity_lookup", "Model Interpretation: Identical subscriber identity with overlapping validity. Limitations: Review source facts.")).To(ContainSubstring("identity inference"))
		Expect(validateForensicSynthesisSummary("subscriber_identity_lookup", "Model Interpretation: The CNIC is 00000-1000001-1. Limitations: Review source facts.")).To(ContainSubstring("numeric"))
		Expect(validateForensicSynthesisSummary("subscriber_status_summary", "Model Interpretation: Review supplied status rows. Limitations: This is a review of")).To(ContainSubstring("incomplete ending"))
		Expect(validateForensicSynthesisSummary("subscriber_status_summary", "Model Interpretation: The summary supports source review. Limitations: It does not establish ownership or validity overlap.")).To(BeEmpty())
		Expect(validateForensicSynthesisSummary("anpr_sightings", "Model Interpretation: The observations support bounded review. Limitations: They do not establish a continuous route.")).To(BeEmpty())
	})

	DescribeTable("maps typed subscriber operations to exact deterministic templates",
		func(operationID, template string) {
			legacy, early, err := normalizeV1QueryPlan("case-alpha", "request-subscriber", map[string]any{
				"contract_version": "forensics.query-plan/v1", "intent": operationID,
				"families": []any{"subscriber_identity"}, "entities": []any{map[string]any{"type": "subscriber_reference", "value": "PK-SUB-SYN-001"}}, "limit": 25,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(early).To(BeNil())
			Expect(legacy).To(HaveKeyWithValue("template", template))
			Expect(legacy).To(HaveKeyWithValue("record_type", "subscriber"))
		},
		Entry("identity", "subscriber.identity_lookup", "subscriber_identity_lookup"),
		Entry("validity", "subscriber.validity_timeline", "subscriber_validity_timeline"),
		Entry("devices", "subscriber.device_links", "subscriber_device_links"),
		Entry("status", "subscriber.status_summary", "subscriber_status_summary"),
		Entry("conflicts", "subscriber.conflict_audit", "subscriber_conflict_audit"),
		Entry("reuse", "subscriber.reuse_candidates", "subscriber_reuse_candidates"),
	)

	It("returns typed clarification before executing a target-specific subscriber operation", func() {
		_, response, err := normalizeV1QueryPlan("case-alpha", "request-subscriber-target", map[string]any{
			"contract_version": "forensics.query-plan/v1", "intent": "subscriber.identity_lookup", "families": []any{"subscriber_identity"},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(response).NotTo(BeNil())
		Expect(response.Status).To(Equal("needs_input"))
		Expect(response.ExecutionTrace.Route).To(Equal([]string{"clarification"}))
		Expect(response.Limitations).To(ContainElement(ContainSubstring("Full CNIC")))
	})

	It("adds subscriber traceability, limitations, finding, and visualization", func() {
		legacy := hybridQueryResponse{
			CollectionID: "case-alpha", Template: "subscriber_status_summary", Route: []string{"records_sql"},
			Enterprise: map[string]any{
				"status": "answered", "summary": "Two explicit subscriber statuses were counted.",
				"data_grid": map[string]any{
					"columns": []map[string]any{{"key": "subscriber_status", "label": "Status", "type": "string"}, {"key": "row_count", "label": "Rows", "type": "integer"}},
					"rows":    []map[string]any{{"subscriber_status": "ACTIVE", "row_count": 7}, {"subscriber_status": "SUSPENDED", "row_count": 2}},
				},
			},
		}
		response := legacyHybridToEnterpriseV1("case-alpha", "request-subscriber", map[string]any{"tenant_id": "default", "limit": 20}, legacy)
		Expect(response.ExecutionTrace.AdapterIDs).To(ContainElement("nexusai.adapter.subscriber_identity"))
		Expect(response.ExecutionTrace.ToolIDs).To(ContainElement("subscriber.status_summary"))
		Expect(response.SemanticEvidence).To(HaveLen(1))
		Expect(response.SemanticEvidence[0].Locator).To(ContainSubstring("record_type:subscriber"))
		Expect(response.DeterministicFindings).To(HaveLen(1))
		Expect(response.Visualizations).To(HaveLen(1))
		Expect(response.Visualizations[0].Spec).To(HaveKeyWithValue("value_field", "row_count"))
		Expect(response.Limitations).To(ContainElement(ContainSubstring("masking CNIC")))
	})
})
