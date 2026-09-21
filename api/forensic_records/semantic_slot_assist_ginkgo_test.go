package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Bounded semantic slot assist", func() {
	It("is feature-flagged off and clarifies without a model call", func() {
		var calls atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls.Add(1) }))
		defer server.Close()
		decision, audit := assistSemanticSlot(context.Background(), config{LocalAIURL: server.URL}, SemanticSlotAssistRequestV1{
			Question: "Use the seven day range", Slot: "date_range", Options: []string{"last_7_days", "last_30_days"},
		})
		Expect(decision.Status).To(Equal("CLARIFY"))
		Expect(audit.FallbackReason).To(Equal("FEATURE_DISABLED"))
		Expect(calls.Load()).To(Equal(int32(0)))
	})

	It("uses one flat four-key schema and accepts only a server-issued value", func() {
		var captured map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			Expect(json.NewDecoder(request.Body).Decode(&captured)).To(Succeed())
			content := `{"contract_version":"forensics.slot-assist/v1","status":"SELECTED","slot":"date_range","value":"last_7_days"}`
			_ = json.NewEncoder(writer).Encode(map[string]any{"choices": []map[string]any{{"message": map[string]string{"content": content}}}})
		}))
		defer server.Close()
		decision, audit := assistSemanticSlot(context.Background(), config{
			LocalAIURL: server.URL, SynthesisModel: "qwen_qwen3-4b-instruct-2507", SemanticSlotAssistEnabled: true,
		}, SemanticSlotAssistRequestV1{Question: "Use the seven day range", Slot: "date_range", Options: []string{"last_7_days", "last_30_days"}})
		Expect(decision.Status).To(Equal("SELECTED"))
		Expect(decision.Value).To(Equal("last_7_days"))
		Expect(audit.State).To(Equal("SELECTED"))
		Expect(audit.PromptBytes).To(BeNumerically("<=", semanticSlotAssistPromptBytes))
		Expect(captured["max_tokens"]).To(BeEquivalentTo(semanticSlotAssistMaxTokens))
		schema := captured["response_format"].(map[string]any)["json_schema"].(map[string]any)["schema"].(map[string]any)
		Expect(schema["additionalProperties"]).To(BeFalse())
		Expect(schema["required"]).To(HaveLen(4))
	})

	It("clarifies on an invented value or a bounded timeout", func() {
		invalid := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			content := `{"contract_version":"forensics.slot-assist/v1","status":"SELECTED","slot":"date_range","value":"all_time"}`
			_ = json.NewEncoder(writer).Encode(map[string]any{"choices": []map[string]any{{"message": map[string]string{"content": content}}}})
		}))
		defer invalid.Close()
		cfg := config{LocalAIURL: invalid.URL, SynthesisModel: "qwen_qwen3-4b-instruct-2507", SemanticSlotAssistEnabled: true}
		decision, audit := assistSemanticSlot(context.Background(), cfg, SemanticSlotAssistRequestV1{Question: "Use all time", Slot: "date_range", Options: []string{"last_7_days"}})
		Expect(decision.Status).To(Equal("CLARIFY"))
		Expect(audit.FallbackReason).To(Equal("INVALID_SERVER_VALUE"))

		slow := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			select {
			case <-request.Context().Done():
			case <-time.After(time.Second):
			}
		}))
		defer slow.Close()
		cfg.LocalAIURL, cfg.SemanticSlotAssistTimeout = slow.URL, 20*time.Millisecond
		decision, audit = assistSemanticSlot(context.Background(), cfg, SemanticSlotAssistRequestV1{Question: "Use seven days", Slot: "date_range", Options: []string{"last_7_days"}})
		Expect(decision.Status).To(Equal("CLARIFY"))
		Expect(audit.FallbackReason).To(Equal("TIMEOUT_OR_UNAVAILABLE"))
		Expect(audit.TimeoutMS).To(Equal(20))
	})
})
