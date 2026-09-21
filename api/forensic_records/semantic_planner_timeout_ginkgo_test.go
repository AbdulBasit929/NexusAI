package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Open-ended semantic request lifetime", func() {
	DescribeTable("uses the frozen bound while allowing shorter operator limits", func(setting, want time.Duration) {
		Expect(semanticPlannerRequestTimeout(config{SemanticPlannerTimeout: setting})).To(Equal(want))
	}, Entry("default", time.Duration(0), 180*time.Second), Entry("shorter", 30*time.Second, 30*time.Second), Entry("cannot widen", 300*time.Second, 180*time.Second))
	It("cancels an in-flight backend request when the configured deadline expires", func() {
		canceled := make(chan struct{})
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.Copy(io.Discard, r.Body)
			select {
			case <-r.Context().Done():
				close(canceled)
			case <-time.After(3 * time.Second):
			}
		}))
		defer server.Close()
		req := hybridQueryRequest{Query: "Summarize recorded call service categories", SynthesisModel: "qwen_qwen3-4b-instruct-2507", QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)}}
		_, state := resolveSemanticOperationInCandidates(context.Background(), config{LocalAIURL: server.URL, SemanticPlannerTimeout: 50 * time.Millisecond}, req, semanticOperationCandidates(req))
		Expect(state).To(Equal("timeout_or_unavailable"))
		Eventually(canceled, 2*time.Second).Should(BeClosed())
	})
})
