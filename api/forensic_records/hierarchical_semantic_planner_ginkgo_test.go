package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Retrieval-first semantic contracts", func() {
	It("preserves family metadata without using a family model decision", func() {
		all := semanticOperationCandidates(hybridQueryRequest{})
		groups := semanticFamilyGroups(all)
		Expect(groups).To(HaveLen(14))
		seen := map[string]bool{}
		for _, group := range groups {
			for _, candidate := range semanticFamilyCandidates(all, group.ID) {
				Expect(candidate.FamilyID).To(Equal(group.ID))
				Expect(seen[candidate.OperationID]).To(BeFalse())
				seen[candidate.OperationID] = true
			}
		}
		Expect(seen).To(HaveLen(len(all)))
	})

	DescribeTable("binds server facts after one deterministic registered decision", func(operation, question, target string) {
		var calls atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			Fail("the deterministic compiler must not call the model-based fallback selector")
		}))
		defer server.Close()

		req := hybridQueryRequest{TenantID: "development-t", CollectionID: "development-c", UserID: "development-u", Target: target, DateFrom: "2026-07-01T00:00:00Z", Query: question, SynthesisModel: "qwen_qwen3-4b-instruct-2507", QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)}}
		got, state := resolveOpenEndedSemanticPlanner(context.Background(), config{LocalAIURL: server.URL}, req)
		Expect(state).To(Equal("semantic_deterministic_registered"))
		Expect(calls.Load()).To(BeZero())
		Expect(got.SemanticPlannerAudit.Selected.OperationID).To(Equal(operation))
		Expect(got.SemanticPlannerAudit.SelectedDecision).To(Equal(semanticExecutorRegistered))
		Expect(got.SemanticPlannerAudit.FamilyDecision).To(BeEmpty())
		Expect(got.SemanticPlannerAudit.EligibleOperationCount).To(Equal(66))
		Expect(got.SemanticPlannerAudit.RetrievedCandidates).To(HaveLen(semanticOperationTopK))
		Expect(got.SemanticPlannerAudit.RetrievalScores).To(HaveLen(semanticOperationTopK))
		Expect(got.SemanticPlannerAudit.CandidateCount).To(BeNumerically("<=", semanticOperationTopK))
		Expect(got.SemanticPlannerAudit.TopK).To(Equal(semanticOperationTopK))
		Expect(got.SemanticPlannerAudit.RequestClass).To(Equal(semanticRequestGovernedAnalysis))
		Expect(got.SemanticPlannerAudit.ScopeFilterReason).To(Equal("authorized_workspace_registry"))
		Expect([]string{"READY", "SERVER_BINDABLE", "USER_FACT_REQUIRED"}).To(ContainElement(got.SemanticPlannerAudit.BindingState))
		Expect(got.TenantID).To(Equal(req.TenantID))
		Expect(got.CollectionID).To(Equal(req.CollectionID))
		Expect(got.DateFrom).To(Equal(req.DateFrom))
		Expect(got.Target).To(Equal(target))
	},
		Entry("communications", "cdr.frequent_contacts", "Rank the counterparties of 923146208975 by how often they exchanged calls.", "923146208975"),
		Entry("documents", "document.search", "Find passages discussing damage to the loading dock in the case documents.", ""),
		Entry("camera observations", "anpr.camera_activity", "Which capture points recorded the greatest number of vehicle sightings?", ""),
		Entry("audio", "audio.transcript_search", "Look through recorded speech for mentions of the missed supply delivery.", ""),
		Entry("generic", "generic.filter_records", "Display the imported spreadsheet entries for inspection.", ""),
	)

	It("bypasses analytical retrieval for domain and product help", func() {
		var calls atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls.Add(1) }))
		defer server.Close()
		for _, test := range []struct{ question, class string }{
			{"Explain what an equipment identifier represents in a cellular network.", semanticRequestGeneralDomainKnowledge},
			{"What NexusAI capabilities are available?", semanticRequestProductHelp},
		} {
			got, state := resolveOpenEndedSemanticPlanner(context.Background(), config{LocalAIURL: server.URL}, hybridQueryRequest{Query: test.question, SynthesisModel: "qwen_qwen3-4b-instruct-2507"})
			Expect(state).To(Equal("GENERAL_PRODUCT_HELP"))
			Expect(got.SemanticPlannerAudit.RequestClass).To(Equal(test.class))
		}
		Expect(calls.Load()).To(BeZero())
	})

	DescribeTable("holds execution before the model-based fallback selector can return nonissued or authority-bearing decisions", func(raw string) {
		var calls atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			calls.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]string{"content": raw}}}})
		}))
		defer server.Close()
		got, state := resolveOpenEndedSemanticPlanner(context.Background(), config{LocalAIURL: server.URL}, hybridQueryRequest{Query: "Review the current investigation.", SynthesisModel: "qwen_qwen3-4b-instruct-2507", TenantID: "t"})
		Expect(state).To(Equal("AMBIGUOUS_INTENT"))
		Expect(got.Template).To(BeEmpty())
		Expect(got.TenantID).To(Equal("t"))
		// The model-based fallback selector is disabled again after a second
		// live reproduction of a confident wrong answer under different
		// wording than the first — see hierarchical_semantic_planner.go. The
		// model must never be called here at all until a grounding signal
		// that generalizes across free-form wording exists.
		Expect(calls.Load()).To(BeZero())
		Expect(raw).NotTo(BeEmpty())
	},
		Entry("unknown family", `{"decision":"FAMILY:invented"}`),
		Entry("authority field", `{"decision":"OPERATION:cdr.frequent_contacts","tenant_id":"foreign"}`),
		Entry("unissued operation", `{"decision":"OPERATION:shell.exec"}`),
		Entry("old insufficient terminal", `{"decision":"CLARIFY:INSUFFICIENT_FACTS"}`),
	)

	It("uses DYNAMIC_TYPED_PLAN exactly once and never retries after unsupported", func() {
		proposal := semanticDynamicProposal{Mode: "group", Group: &CanonicalGroupV1{Field: "source_file", Measure: "count"}}
		var calls atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer GinkgoRecover()
			body, err := io.ReadAll(r.Body)
			Expect(err).NotTo(HaveOccurred())
			call := calls.Add(1)
			content := `{"decision":"DYNAMIC_TYPED_PLAN"}`
			if call == 1 {
				var payload map[string]any
				Expect(json.Unmarshal(body, &payload)).To(Succeed())
				responseFormat := payload["response_format"].(map[string]any)
				jsonSchema := responseFormat["json_schema"].(map[string]any)
				schema := jsonSchema["schema"].(map[string]any)
				properties := schema["properties"].(map[string]any)
				decision := properties["decision"].(map[string]any)
				enums := decision["enum"].([]any)
				issued := 0
				for _, value := range enums {
					if value == semanticDynamicDecision {
						issued++
					}
				}
				Expect(issued).To(Equal(1))
			} else {
				encoded, err := json.Marshal(proposal)
				Expect(err).NotTo(HaveOccurred())
				content = string(encoded)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]string{"content": content}}}})
		}))
		defer server.Close()
		req := hybridQueryRequest{TenantID: "t", UserID: "u", CollectionID: "c", Query: "Count generic rows by source file.", RecordType: "generic", SynthesisModel: "qwen_qwen3-4b-instruct-2507", QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)}}
		got, state := resolveSemanticOperationInCandidates(context.Background(), config{LocalAIURL: server.URL}, req, semanticOperationCandidates(req))
		Expect(state).To(Equal("semantic_dynamic_plan"))
		Expect(got.Group).NotTo(BeNil())
		Expect(got.SemanticPlannerAudit.DynamicSelected).To(BeTrue())
		Expect(got.SemanticPlannerAudit.SelectedDecision).To(Equal(semanticDynamicDecision))
		Expect(calls.Load()).To(Equal(int32(2)))

		calls.Store(0)
		unsupported := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			calls.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]string{"content": `{"decision":"UNSUPPORTED:REQUEST_CLASS"}`}}}})
		}))
		defer unsupported.Close()
		_, state = resolveSemanticOperationInCandidates(context.Background(), config{LocalAIURL: unsupported.URL}, req, semanticOperationCandidates(req))
		Expect(state).To(Equal("UNSUPPORTED_REQUEST_CLASS"))
		Expect(calls.Load()).To(Equal(int32(1)))
	})
})
