package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Configured narrative lifetime", func() {
	DescribeTable("shares bounded configuration", func(value, want time.Duration) {
		Expect(synthesisRequestTimeout(config{SynthesisTimeout: value})).To(Equal(want))
	}, Entry("default", time.Duration(0), 120*time.Second), Entry("fast", 5*time.Second, 5*time.Second), Entry("minute", 60*time.Second, 60*time.Second), Entry("local", 120*time.Second, 120*time.Second), Entry("maximum", 300*time.Second, 180*time.Second))
	It("cancels the server around a configured five-second deadline", func() {
		canceled := make(chan struct{})
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.Copy(io.Discard, r.Body)
			select {
			case <-r.Context().Done():
				close(canceled)
			case <-time.After(7 * time.Second):
			}
		}))
		defer server.Close()
		start := time.Now()
		_, reason := synthesizeFactPacketNarrative(context.Background(), config{LocalAIURL: server.URL, SynthesisTimeout: 5 * time.Second}, hybridQueryRequest{SynthesisModel: "qwen_qwen3-4b-instruct-2507"}, FactPacketV1{})
		Expect(reason).To(ContainSubstring("timed out"))
		Expect(time.Since(start)).To(BeNumerically(">=", 4800*time.Millisecond))
		Expect(time.Since(start)).To(BeNumerically("<", 6500*time.Millisecond))
		Eventually(canceled, time.Second).Should(BeClosed())
	})
	It("allows a timely valid answer beyond the removed eight-second cap", func() {
		packet := FactPacketV1{Language: QueryLanguageV1{Tag: "en"}, Facts: []FactPacketFactV1{{FactID: "F1", Text: "The authorized count is 31.", CitationIDs: []string{"C1"}}}, Citations: []FactPacketCitationV1{{CitationID: "C1"}}}
		narrative := NarrativeV1{ContractVersion: narrativeContractV1, Locale: "en", Direction: "ltr", DirectAnswer: "The authorized count is 31.", FactRefs: []string{"F1"}, CitationRefs: []string{"C1"}}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.Copy(io.Discard, r.Body)
			select {
			case <-r.Context().Done():
				return
			case <-time.After(8200 * time.Millisecond):
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]any{"content": mustNarrativeJSON(narrative)}}}})
		}))
		defer server.Close()
		got, reason := synthesizeFactPacketNarrative(context.Background(), config{LocalAIURL: server.URL, SynthesisTimeout: 60 * time.Second}, hybridQueryRequest{SynthesisModel: "qwen_qwen3-4b-instruct-2507"}, packet)
		Expect(reason).To(BeEmpty())
		Expect(got.DirectAnswer).To(Equal(narrative.DirectAnswer))
	})
	DescribeTable("respects a shorter parent lifetime", func(explicitCancel bool) {
		ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
		defer cancel()
		canceled := make(chan struct{})
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.Copy(io.Discard, r.Body)
			if explicitCancel {
				cancel()
			}
			select {
			case <-r.Context().Done():
				close(canceled)
			case <-time.After(time.Second):
			}
		}))
		defer server.Close()
		start := time.Now()
		_, reason := synthesizeFactPacketNarrative(ctx, config{LocalAIURL: server.URL, SynthesisTimeout: 120 * time.Second}, hybridQueryRequest{SynthesisModel: "qwen_qwen3-4b-instruct-2507"}, FactPacketV1{})
		Expect(reason).NotTo(BeEmpty())
		Expect(time.Since(start)).To(BeNumerically("<", time.Second))
		Eventually(canceled, time.Second).Should(BeClosed())
	}, Entry("deadline", false), Entry("cancellation", true))
	DescribeTable("retains deterministic facts after rejected HTTP model output", func(kind string) {
		packet := FactPacketV1{Language: QueryLanguageV1{Tag: "en"}, Facts: []FactPacketFactV1{{FactID: "F1", Text: "There are 31 authorized records.", CitationIDs: []string{"C1"}}}, Citations: []FactPacketCitationV1{{CitationID: "C1"}}, Limitations: []string{"Only the selected source was counted."}}
		before, _ := json.Marshal(packet)
		candidate := NarrativeV1{ContractVersion: narrativeContractV1, Locale: "en", Direction: "ltr", DirectAnswer: "There are 31 authorized records.", FactRefs: []string{"F1"}, CitationRefs: []string{"C1"}}
		switch kind {
		case "invented count":
			candidate.DirectAnswer = "There are 31 authorized records. Another 99999 records exist."
		case "foreign reference":
			candidate.CitationRefs = []string{"C-foreign"}
		}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if kind == "malformed envelope" {
				_, _ = w.Write([]byte("{broken"))
				return
			}
			content := mustNarrativeJSON(candidate)
			if kind == "malformed narrative" {
				content = `{"sql":"SELECT secret"}`
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": content}}}})
		}))
		defer server.Close()
		got, reason := synthesizeFactPacketNarrative(context.Background(), config{LocalAIURL: server.URL, SynthesisTimeout: 60 * time.Second}, hybridQueryRequest{SynthesisModel: "qwen_qwen3-4b-instruct-2507"}, packet)
		Expect(reason).NotTo(BeEmpty())
		Expect(got).To(Equal(NarrativeV1{}))
		fallback := deterministicNarrativeFallback(packet, reason)
		Expect(fallback.Fallback).To(BeTrue())
		Expect(fallback.DirectAnswer).To(ContainSubstring("31 authorized records"))
		Expect(fallback.Limitations).To(ContainElement("Only the selected source was counted."))
		Expect(fallback.FactRefs).To(ContainElement("F1"))
		Expect(fallback.CitationRefs).To(ContainElement("C1"))
		after, _ := json.Marshal(packet)
		Expect(after).To(Equal(before))
	}, Entry("malformed envelope", "malformed envelope"), Entry("malformed narrative", "malformed narrative"), Entry("invented count", "invented count"), Entry("foreign reference", "foreign reference"))
})

func mustNarrativeJSON(n NarrativeV1) string { b, _ := json.Marshal(n); return string(b) }
