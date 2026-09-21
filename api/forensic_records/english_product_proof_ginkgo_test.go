package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mudler/LocalAI/core/services/agents"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type productProofCase struct {
	ID       string   `json:"id"`
	Question string   `json:"question"`
	Allowed  []string `json:"allowed"`
	Review   string   `json:"review"`
}

type productProofCorpus struct {
	Semantic  []productProofCase           `json:"semantic"`
	Router    []productProofCase           `json:"router"`
	Synthesis []englishSynthesisCorpusCase `json:"synthesis"`
	General   []productProofCase           `json:"general"`
}

// This one-shot proof runs the source planner and narrative validator against
// synthetic authority. It never executes a records query or writes case state.
var _ = Describe("Fresh English product proof", Ordered, func() {
	It("validates the fresh fixture schema, issued decisions and safe packet fallback offline", func() {
		path := os.Getenv("NEXUSAI_SMALL_ENGLISH_CORPUS")
		if path == "" {
			path = filepath.Join("..", "..", "scripts", "nxb21-english-functional-qualification", "small-english-product-corpus-v1.json")
		}
		data, err := os.ReadFile(path)
		Expect(err).NotTo(HaveOccurred())
		var corpus productProofCorpus
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		Expect(decoder.Decode(&corpus)).To(Succeed())
		Expect(decoder.Decode(&struct{}{})).To(Equal(io.EOF))
		candidates := semanticOperationCandidates(hybridQueryRequest{QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)}})
		for _, c := range corpus.Semantic {
			Expect(c.Allowed).NotTo(BeEmpty())
			for _, decision := range c.Allowed {
				Expect(semanticDecisionKnown(decision, candidates)).To(BeTrue(), "unknown frozen oracle decision for %s: %s", c.ID, decision)
			}
		}
		for _, c := range corpus.Synthesis {
			packet := synthesisPacket(c)
			Expect(qualificationFallbackSafe(packet, deterministicNarrativeFallback(packet, "offline transport failure"))).To(BeTrue(), c.ID)
		}
		for _, c := range corpus.General {
			Expect(c.Review).NotTo(BeEmpty())
		}
	})
	It("captures the four frozen stages once for independent adjudication", func() {
		if os.Getenv("NEXUSAI_SMALL_ENGLISH_LIVE") != "1" {
			Skip("standalone governed runner only")
		}
		var corpus productProofCorpus
		data, err := os.ReadFile(os.Getenv("NEXUSAI_SMALL_ENGLISH_CORPUS"))
		Expect(err).NotTo(HaveOccurred())
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		Expect(decoder.Decode(&corpus)).To(Succeed())
		Expect(decoder.Decode(&struct{}{})).To(Equal(io.EOF))
		Expect(corpus.Semantic).To(HaveLen(16))
		Expect(corpus.Router).To(HaveLen(8))
		Expect(corpus.Synthesis).To(HaveLen(13))
		Expect(corpus.General).To(HaveLen(8))
		lock, err := os.OpenFile(os.Getenv("NEXUSAI_SMALL_ENGLISH_LOCK"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		Expect(err).NotTo(HaveOccurred())
		_, err = lock.WriteString(time.Now().UTC().Format(time.RFC3339Nano))
		Expect(err).NotTo(HaveOccurred())
		Expect(lock.Close()).To(Succeed())
		model, base := os.Getenv("NEXUSAI_SMALL_ENGLISH_MODEL"), os.Getenv("NEXUSAI_SMALL_ENGLISH_URL")
		cfg := config{LocalAIURL: base, SynthesisModel: model, SemanticPlannerTimeout: 180 * time.Second, SynthesisTimeout: 120 * time.Second}
		rows := []map[string]any{}
		failures := 0
		path := filepath.Join(os.Getenv("NEXUSAI_SMALL_ENGLISH_RUN"), "product-proof-results.json")
		save := func() {
			out, marshalErr := json.MarshalIndent(map[string]any{"contract_version": "nexusai.small-english-product-proof/v1", "automated_failures": failures, "results": rows, "state": "INDEPENDENT_CONTENT_REVIEW_REQUIRED", "live_records_execution": false, "strict_certification": "PENDING"}, "", "  ")
			Expect(marshalErr).NotTo(HaveOccurred())
			Expect(os.WriteFile(path, append(out, '\n'), 0600)).To(Succeed())
		}
		appendResult := func(row map[string]any, ok bool) {
			if !ok {
				failures++
			}
			row["automated_check_pass"] = ok
			rows = append(rows, row)
			save()
			fmt.Fprintf(GinkgoWriter, "PRODUCT_CASE_COMPLETED stage=%v id=%v automated_pass=%v total=%d\n", row["stage"], row["id"], ok, len(rows))
		}
		for _, c := range corpus.Semantic {
			started := time.Now()
			req := hybridQueryRequest{TenantID: "proof-tenant", UserID: "proof-analyst", CollectionID: "proof-case", Query: c.Question, Limit: 20, MaxKBResults: 8, SynthesisModel: model, QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)}}
			resolved, state := resolveOpenEndedSemanticPlanner(context.Background(), cfg, req)
			raw, decision := "", ""
			if resolved.SemanticPlannerAudit != nil {
				raw = resolved.SemanticPlannerAudit.RawProposal
				if resolved.SemanticPlannerAudit.ModelProposal != nil {
					decision = resolved.SemanticPlannerAudit.ModelProposal.Decision
				}
			}
			_, schemaErr := decodeSemanticOperationProposal([]byte(raw), semanticOperationCandidates(req))
			allowed := false
			for _, d := range c.Allowed {
				allowed = allowed || d == decision
			}
			// The only model-authored field may be a server-issued decision enum.
			ok := schemaErr == nil && semanticDecisionKnown(decision, semanticOperationCandidates(req)) && !semanticRawHasForbiddenAuthority(raw) && allowed
			appendResult(map[string]any{"stage": "A", "id": c.ID, "question": c.Question, "raw": raw, "decision": decision, "planner_state": state, "allowed": c.Allowed, "schema_valid": schemaErr == nil, "clarification": strings.HasPrefix(decision, "CLARIFY:"), "latency_ms": time.Since(started).Milliseconds()}, ok)
		}
		for _, c := range corpus.Router {
			started := time.Now()
			classification := agents.ClassifyForensicRequestV1(c.Question)
			decision := "GOVERNED_TOOL"
			raw := ""
			var callErr error
			if classification != agents.ForensicRequestDeterministicFastPath {
				raw, callErr = productProofCompletion(base, model, agents.ForensicRecordsToolsPolicy+"\nReturn only a decision: GOVERNED_TOOL for case/evidence/follow-up work; DIRECT_GENERAL for product help or relevant general knowledge without case claims; CLARIFY for missing context; SAFE_UNSUPPORTED for unavailable or irrelevant work.", c.Question, "decision", []string{"GOVERNED_TOOL", "DIRECT_GENERAL", "CLARIFY", "SAFE_UNSUPPORTED"}, 48)
				decision = raw
			}
			allowed := false
			for _, d := range c.Allowed {
				allowed = allowed || d == decision
			}
			appendResult(map[string]any{"stage": "B", "id": c.ID, "question": c.Question, "decision": decision, "deterministic_route": classification == agents.ForensicRequestDeterministicFastPath, "error": fmt.Sprint(callErr), "latency_ms": time.Since(started).Milliseconds()}, callErr == nil && allowed)
		}
		for _, c := range corpus.Synthesis {
			started := time.Now()
			packet := synthesisPacket(c)
			narrative, reason := synthesizeFactPacketNarrative(context.Background(), cfg, hybridQueryRequest{Query: c.Question, SynthesisModel: model}, packet)
			valid := reason == "" && validateNarrative(packet, narrative) == nil
			if !valid {
				narrative = deterministicNarrativeFallback(packet, reason)
			}
			// Reference and literal invariants are checked by the source validator.
			// Content meaning, unsupported implications and paraphrases require review.
			ok := valid || qualificationFallbackSafe(packet, narrative)
			appendResult(map[string]any{"stage": "C", "id": c.ID, "shape": c.Shape, "packet": packet, "narrative": narrative, "validated_model": valid, "fallback": !valid, "fallback_reason": reason, "latency_ms": time.Since(started).Milliseconds()}, ok)
		}
		for _, c := range corpus.General {
			started := time.Now()
			answer, err := productProofCompletion(base, model, agents.ForensicRecordsToolsPolicy+"\nAnswer the general product or forensic concept question briefly. No case evidence or tools are supplied. Do not claim to have examined a case or cite invented sources. Distinguish recognition candidates from verified facts. Describe capabilities conditionally on configured processing and authorized evidence. Return JSON with only answer.", c.Question, "answer", nil, 384)
			appendResult(map[string]any{"stage": "D", "id": c.ID, "question": c.Question, "answer": answer, "review_rubric": c.Review, "error": fmt.Sprint(err), "latency_ms": time.Since(started).Milliseconds(), "content_review": "PENDING"}, err == nil && strings.TrimSpace(answer) != "")
		}
		Expect(failures).To(BeZero(), "automated failures do not erase captured results; inspect %s", path)
	})
})

func productProofCompletion(base, model, system, question, key string, enum []string, maxTokens int) (string, error) {
	property := map[string]any{"type": "string"}
	if len(enum) > 0 {
		property["enum"] = enum
	}
	body, err := json.Marshal(map[string]any{"model": model, "temperature": 0, "max_tokens": maxTokens, "response_format": map[string]any{"type": "json_schema", "json_schema": map[string]any{"name": "product_proof", "strict": true, "schema": map[string]any{"type": "object", "additionalProperties": false, "required": []string{key}, "properties": map[string]any{key: property}}}}, "messages": []map[string]string{{"role": "system", "content": system}, {"role": "user", "content": question}}})
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(base, "/")+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return "", fmt.Errorf("HTTP %d", response.StatusCode)
	}
	var envelope struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
	}
	if err = json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&envelope); err != nil {
		return "", err
	}
	if len(envelope.Choices) != 1 || envelope.Choices[0].FinishReason != "stop" {
		return "", fmt.Errorf("incomplete completion")
	}
	var object map[string]json.RawMessage
	decoder := json.NewDecoder(strings.NewReader(envelope.Choices[0].Message.Content))
	if err = decoder.Decode(&object); err != nil {
		return "", err
	}
	if decoder.Decode(&struct{}{}) != io.EOF || len(object) != 1 {
		return "", fmt.Errorf("invalid response shape")
	}
	var value string
	if err = json.Unmarshal(object[key], &value); err != nil {
		return "", err
	}
	if len(enum) > 0 {
		for _, allowed := range enum {
			if value == allowed {
				return value, nil
			}
		}
		return "", fmt.Errorf("unknown decision")
	}
	return value, nil
}
