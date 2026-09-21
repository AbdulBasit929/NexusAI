package main

import (
	"context"
	"encoding/json"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
)

var _ = Describe("Semantic intent separate from server binding", func() {
	DescribeTable("retains a selected intent with explicit execution readiness", func(target, question, want, initial, boundTarget string) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer GinkgoRecover()
			b, _ := io.ReadAll(r.Body)
			Expect(string(b)).NotTo(ContainSubstring(`\"requires\"`))
			Expect(string(b)).To(ContainSubstring("AFTER selection"))
			_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]string{"content": `{"decision":"OPERATION:cdr.frequent_contacts"}`}}}})
		}))
		defer server.Close()
		req := hybridQueryRequest{TenantID: "t-contract", UserID: "u-contract", CollectionID: "c-contract", Target: target, Query: question, SynthesisModel: "qwen_qwen3-4b-instruct-2507", QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)}, DateFrom: "2026-06-01T00:00:00Z"}
		got, status := resolveSemanticOperationInCandidates(context.Background(), config{LocalAIURL: server.URL}, req, semanticOperationCandidates(req))
		Expect(status).To(Equal("semantic_model_plan"))
		Expect(got.Template).To(Equal("frequent_contacts"))
		Expect(got.SemanticPlannerAudit.BindingState).To(Equal(want))
		Expect(got.SemanticPlannerAudit.BindingInitialState).To(Equal(initial))
		Expect(got.Target).To(Equal(boundTarget))
		Expect(got.CollectionID).To(Equal(req.CollectionID))
		Expect(got.DateFrom).To(Equal(req.DateFrom))
	}, Entry("server supplied target", "923219876543", "Rank communication partners by event frequency", "READY", "READY", "923219876543"), Entry("user fact required", "", "Rank communication partners by event frequency", "USER_FACT_REQUIRED", "USER_FACT_REQUIRED", ""), Entry("server can bind a literal supplied by the user", "", "Rank communication partners for 923219876543 by event frequency", "READY", "SERVER_BINDABLE", "923219876543"))
	It("keeps the full workspace registry and rejects model-authored facts", func() {
		req := hybridQueryRequest{QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)}}
		candidates := semanticOperationCandidates(req)
		Expect(candidates).To(HaveLen(66))
		for _, raw := range []string{`{"decision":"OPERATION:invented"}`, `{"decision":"OPERATION:cdr.frequent_contacts","target":"000999"}`, `{"decision":"OPERATION:cdr.frequent_contacts","date_from":"2000-01-01"}`, `{"decision":"OPERATION:cdr.frequent_contacts","collection_id":"elsewhere"}`} {
			_, err := decodeSemanticOperationProposal([]byte(raw), candidates)
			Expect(err).To(HaveOccurred())
		}
		for _, c := range candidates {
			Expect(c.ExposureStatus).NotTo(Equal("engineering_only"))
			Expect(c.Description).NotTo(BeEmpty())
			Expect(c.FamilyID).NotTo(BeEmpty())
			Expect(c.ResultKind).NotTo(BeEmpty())
			for _, field := range c.RequiredParameters {
				Expect([]string{"target", "evidence_id", "source_set"}).To(ContainElement(field), c.OperationID)
			}
		}
		if output := os.Getenv("NX_B21_DESCRIPTOR_AUDIT_OUT"); output != "" {
			data, err := json.MarshalIndent(map[string]any{"contract": "nxb21.semantic-descriptor-source-audit/v1", "candidate_policy": "FULL_WORKSPACE_REGISTRY_NO_QUESTION_FILTER", "candidate_count": len(candidates), "candidates": candidates, "live_inference": false, "model_effectiveness": "UNPROVEN"}, "", "  ")
			Expect(err).NotTo(HaveOccurred())
			Expect(os.WriteFile(output, append(data, '\n'), 0600)).To(Succeed())
		}
	})
})
