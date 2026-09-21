package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

//go:embed contracts/model-role-development-corpus-v1.json
var modelRoleDevelopmentCorpusJSON []byte

type modelRoleSynthesisCaseV1 struct {
	ID                   string       `json:"id"`
	Family               string       `json:"family"`
	Packet               FactPacketV1 `json:"packet"`
	RequiredFactRefs     []string     `json:"required_fact_refs"`
	RequiredCitationRefs []string     `json:"required_citation_refs"`
	RequiredExactTokens  []string     `json:"required_exact_tokens"`
	ForbiddenTerms       []string     `json:"forbidden_terms"`
}

type modelRoleSlotCaseV1 struct {
	ID, Question, Slot, ExpectedValue string
	Options                           []string `json:"options"`
}

func (value *modelRoleSlotCaseV1) UnmarshalJSON(raw []byte) error {
	type wire struct {
		ID            string   `json:"id"`
		Question      string   `json:"question"`
		Slot          string   `json:"slot"`
		Options       []string `json:"options"`
		ExpectedValue string   `json:"expected_value"`
	}
	var decoded wire
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return err
	}
	value.ID, value.Question, value.Slot, value.Options, value.ExpectedValue = decoded.ID, decoded.Question, decoded.Slot, decoded.Options, decoded.ExpectedValue
	return nil
}

type modelRoleGeneralCaseV1 struct {
	ID                 string     `json:"id"`
	Question           string     `json:"question"`
	RequiredTermGroups [][]string `json:"required_term_groups"`
	ForbiddenTerms     []string   `json:"forbidden_terms"`
}

type modelRoleDevelopmentCorpusV1 struct {
	ContractVersion string                     `json:"contract_version"`
	CorpusID        string                     `json:"corpus_id"`
	Purpose         string                     `json:"purpose"`
	ModelID         string                     `json:"model_id"`
	SynthesisCases  []modelRoleSynthesisCaseV1 `json:"synthesis_cases"`
	SlotAssistCase  modelRoleSlotCaseV1        `json:"slot_assist_case"`
	GeneralCases    []modelRoleGeneralCaseV1   `json:"general_cases"`
}

type modelRoleSynthesisResultV1 struct {
	ID                       string                    `json:"id"`
	Family                   string                    `json:"family"`
	ModelNarrativeAccepted   bool                      `json:"model_narrative_accepted"`
	FactRefsPreserved        bool                      `json:"fact_refs_preserved"`
	CitationRefsPreserved    bool                      `json:"citation_refs_preserved"`
	ExactTokensPreserved     bool                      `json:"exact_tokens_preserved"`
	UnsupportedProseRejected bool                      `json:"unsupported_prose_rejected"`
	FactualAnswerPreserved   bool                      `json:"factual_answer_preserved"`
	FallbackReason           string                    `json:"fallback_reason,omitempty"`
	Audit                    NarrativeSynthesisAuditV1 `json:"audit"`
}

type modelRoleGeneralResultV1 struct {
	ID             string `json:"id"`
	Safe           bool   `json:"safe"`
	Answer         string `json:"answer,omitempty"`
	Failure        string `json:"failure,omitempty"`
	LatencyMS      int64  `json:"latency_ms"`
	RequestSHA256  string `json:"request_sha256,omitempty"`
	ResponseSHA256 string `json:"response_sha256,omitempty"`
}

type modelRoleDevelopmentReceiptV1 struct {
	ContractVersion       string                       `json:"contract_version"`
	CorpusID              string                       `json:"corpus_id"`
	CorpusSHA256          string                       `json:"corpus_sha256"`
	Purpose               string                       `json:"purpose"`
	ModelID               string                       `json:"model_id"`
	SynthesisRoleVerdict  string                       `json:"synthesis_role_verdict"`
	SlotAssistRoleVerdict string                       `json:"slot_assist_role_verdict"`
	GeneralRoleVerdict    string                       `json:"general_assistant_verdict"`
	SynthesisLatencyP50MS int64                        `json:"synthesis_latency_p50_ms"`
	SynthesisLatencyP95MS int64                        `json:"synthesis_latency_p95_ms"`
	SynthesisResults      []modelRoleSynthesisResultV1 `json:"synthesis_results"`
	SlotAssistDecision    SemanticSlotAssistDecisionV1 `json:"slot_assist_decision"`
	SlotAssistAudit       SemanticSlotAssistAuditV1    `json:"slot_assist_audit"`
	GeneralResults        []modelRoleGeneralResultV1   `json:"general_results"`
	ModelDownloadRequired bool                         `json:"model_download_required"`
	HoldoutCreated        bool                         `json:"holdout_created"`
	HoldoutConsumed       bool                         `json:"holdout_consumed"`
	RawBytesCaptured      bool                         `json:"raw_bytes_captured"`
	StrictUTF8            bool                         `json:"strict_utf8"`
	UnloadState           string                       `json:"unload_state"`
	Limitations           []string                     `json:"limitations"`
}

func loadModelRoleDevelopmentCorpus() (modelRoleDevelopmentCorpusV1, error) {
	var corpus modelRoleDevelopmentCorpusV1
	decoder := json.NewDecoder(bytes.NewReader(modelRoleDevelopmentCorpusJSON))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&corpus); err != nil {
		return corpus, err
	}
	if decoder.Decode(&struct{}{}) != io.EOF || corpus.ContractVersion != "nexusai.model-role-development-corpus/v1" || corpus.Purpose != "REUSABLE_DEVELOPMENT_NOT_CERTIFICATION_NOT_HOLDOUT" {
		return corpus, fmt.Errorf("invalid model-role development corpus contract")
	}
	if len(corpus.SynthesisCases) != 5 || len(corpus.GeneralCases) != 3 || len(corpus.SlotAssistCase.Options) == 0 {
		return corpus, fmt.Errorf("model-role development corpus coverage drift")
	}
	return corpus, nil
}

func modelRoleNarrativeRefs(narrative NarrativeV1) ([]string, []string, string) {
	factRefs, citationRefs := append([]string(nil), narrative.FactRefs...), append([]string(nil), narrative.CitationRefs...)
	text := narrative.DirectAnswer
	for _, group := range [][]NarrativeClaimV1{narrative.KeyFindings, narrative.Comparisons, narrative.RelationshipSummary} {
		for _, claim := range group {
			factRefs = append(factRefs, claim.FactRefs...)
			citationRefs = append(citationRefs, claim.CitationRefs...)
			text += "\n" + claim.Text
		}
	}
	return factRefs, citationRefs, text
}

func allRequiredStrings(haystack []string, needles []string) bool {
	for _, needle := range needles {
		if !containsString(haystack, needle) {
			return false
		}
	}
	return true
}

func assessModelRoleNarrative(test modelRoleSynthesisCaseV1, narrative NarrativeV1, reason string, trace narrativeSynthesisTraceV1) modelRoleSynthesisResultV1 {
	factRefs, citationRefs, text := modelRoleNarrativeRefs(narrative)
	exact, forbidden := true, false
	for _, token := range test.RequiredExactTokens {
		exact = exact && strings.Contains(text, token)
	}
	for _, phrase := range test.ForbiddenTerms {
		forbidden = forbidden || strings.Contains(strings.ToLower(text), strings.ToLower(phrase))
	}
	accepted := reason == "" && trace.Audit.Outcome == "VALIDATED_MODEL" && validateNarrative(test.Packet, narrative) == nil
	fallbackSafe := qualificationFallbackSafe(test.Packet, deterministicNarrativeFallback(test.Packet, reason))
	return modelRoleSynthesisResultV1{
		ID: test.ID, Family: test.Family, ModelNarrativeAccepted: accepted,
		FactRefsPreserved:     allRequiredStrings(factRefs, test.RequiredFactRefs),
		CitationRefsPreserved: allRequiredStrings(citationRefs, test.RequiredCitationRefs),
		ExactTokensPreserved:  exact, UnsupportedProseRejected: accepted && !forbidden,
		FactualAnswerPreserved: accepted || fallbackSafe, FallbackReason: reason, Audit: trace.Audit,
	}
}

func modelRoleSynthesisPassed(result modelRoleSynthesisResultV1) bool {
	return result.ModelNarrativeAccepted && result.FactRefsPreserved && result.CitationRefsPreserved && result.ExactTokensPreserved && result.UnsupportedProseRejected
}

func modelRolePercentile(values []int64, percentile float64) int64 {
	if len(values) == 0 {
		return 0
	}
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	index := int(float64(len(values))*percentile+0.999999) - 1
	return values[max(0, min(index, len(values)-1))]
}

func modelRoleGeneralCompletion(ctx context.Context, baseURL, model, question string) (modelRoleGeneralResultV1, []byte, []byte) {
	started := time.Now()
	result := modelRoleGeneralResultV1{}
	system := "Answer the forensic concept question briefly. No case evidence is supplied. Do not claim to have examined a case or invent a source. Explain that observations and confidence scores are not identity or accuracy proof. Product capabilities depend on configured processing and authorized evidence. Return only JSON with answer."
	schema := map[string]any{"type": "object", "additionalProperties": false, "required": []string{"answer"}, "properties": map[string]any{"answer": map[string]any{"type": "string"}}}
	requestBytes, _ := json.Marshal(map[string]any{
		"model": model, "temperature": 0, "max_tokens": 256,
		"response_format": map[string]any{"type": "json_schema", "json_schema": map[string]any{"name": "forensic_general_answer", "strict": true, "schema": schema}},
		"messages":        []map[string]string{{"role": "system", "content": system}, {"role": "user", "content": question}},
	})
	result.RequestSHA256 = fmt.Sprintf("%x", sha256.Sum256(requestBytes))
	callCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	httpRequest, err := http.NewRequestWithContext(callCtx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/v1/chat/completions", bytes.NewReader(requestBytes))
	if err != nil {
		result.Failure = "REQUEST_FAILED"
		return result, requestBytes, nil
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(httpRequest)
	if err != nil {
		result.Failure = "TIMEOUT_OR_UNAVAILABLE"
		result.LatencyMS = time.Since(started).Milliseconds()
		return result, requestBytes, nil
	}
	defer response.Body.Close()
	responseBytes, err := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
	result.LatencyMS = time.Since(started).Milliseconds()
	result.ResponseSHA256 = fmt.Sprintf("%x", sha256.Sum256(responseBytes))
	if err != nil || response.StatusCode != http.StatusOK || len(responseBytes) > 1<<20 || !utf8.Valid(responseBytes) {
		result.Failure = "MALFORMED_TRANSPORT"
		return result, requestBytes, responseBytes
	}
	var envelope struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if json.Unmarshal(responseBytes, &envelope) != nil || len(envelope.Choices) != 1 {
		result.Failure = "MALFORMED_ENVELOPE"
		return result, requestBytes, responseBytes
	}
	var answer struct {
		Answer string `json:"answer"`
	}
	decoder := json.NewDecoder(strings.NewReader(envelope.Choices[0].Message.Content))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&answer) != nil || decoder.Decode(&struct{}{}) != io.EOF || strings.TrimSpace(answer.Answer) == "" {
		result.Failure = "INVALID_SCHEMA"
		return result, requestBytes, responseBytes
	}
	result.Answer = answer.Answer
	return result, requestBytes, responseBytes
}

func assessModelRoleGeneral(test modelRoleGeneralCaseV1, result modelRoleGeneralResultV1) modelRoleGeneralResultV1 {
	text := strings.ToLower(result.Answer)
	result.Safe = result.Failure == "" && !strings.Contains(text, "case evidence shows") && !forbiddenNarrativeClaim(text)
	for _, group := range test.RequiredTermGroups {
		matched := false
		for _, term := range group {
			matched = matched || strings.Contains(text, strings.ToLower(term))
		}
		result.Safe = result.Safe && matched
	}
	for _, term := range test.ForbiddenTerms {
		result.Safe = result.Safe && !strings.Contains(text, strings.ToLower(term))
	}
	return result
}

func modelRoleLoadedModels(baseURL string) ([]string, error) {
	response, err := http.Get(strings.TrimRight(baseURL, "/") + "/system")
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	var system struct {
		LoadedModels []struct {
			ID string `json:"id"`
		} `json:"loaded_models"`
	}
	if response.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&system) != nil {
		return nil, fmt.Errorf("system state unavailable")
	}
	models := make([]string, 0, len(system.LoadedModels))
	for _, model := range system.LoadedModels {
		models = append(models, model.ID)
	}
	sort.Strings(models)
	return models, nil
}

func unloadDevelopmentModel(baseURL, model string) error {
	payload, _ := json.Marshal(map[string]string{"model": model})
	request, _ := http.NewRequest(http.MethodPost, strings.TrimRight(baseURL, "/")+"/backend/shutdown", bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("shutdown status %d", response.StatusCode)
	}
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		models, stateErr := modelRoleLoadedModels(baseURL)
		if stateErr == nil && !containsString(models, model) {
			return nil
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("model remained loaded")
}

func writeModelRoleRawEvidence(directory, id string, requestBytes, responseBytes []byte) error {
	if directory == "" {
		return nil
	}
	directory = filepath.Clean(directory)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(directory, id+"-request.json"), requestBytes, 0o600); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(directory, id+"-response.json"), responseBytes, 0o600)
}

var _ = Describe("NX-B2.1D reduced model-role development adjudication", func() {
	It("freezes five multi-family Fact Packets without creating a holdout", func() {
		corpus, err := loadModelRoleDevelopmentCorpus()
		Expect(err).NotTo(HaveOccurred())
		Expect(corpus.ModelID).To(Equal("qwen_qwen3-4b-instruct-2507"))
		seenFamilies := map[string]bool{}
		for _, test := range corpus.SynthesisCases {
			seenFamilies[test.Family] = true
			Expect(test.Packet.ContractVersion).To(Equal(factPacketContractV1))
			Expect(test.Packet.ExecutionMode).To(Equal("frozen_authoritative_packet"))
			Expect(test.Packet.Facts).NotTo(BeEmpty())
			Expect(test.Packet.Citations).NotTo(BeEmpty())
		}
		Expect(seenFamilies).To(HaveLen(5))
	})

	It("accepts grounded model prose and captures byte-safe transport evidence offline", func() {
		corpus, err := loadModelRoleDevelopmentCorpus()
		Expect(err).NotTo(HaveOccurred())
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			var payload struct {
				Messages []struct {
					Content string `json:"content"`
				} `json:"messages"`
			}
			Expect(json.NewDecoder(request.Body).Decode(&payload)).To(Succeed())
			var wrapped struct {
				FactPacket FactPacketV1 `json:"fact_packet"`
			}
			parts := strings.SplitN(payload.Messages[1].Content, "\n", 2)
			Expect(parts).To(HaveLen(2))
			Expect(json.Unmarshal([]byte(parts[1]), &wrapped)).To(Succeed())
			packet := wrapped.FactPacket
			narrative := NarrativeV1{
				ContractVersion: narrativeContractV1, Locale: "en", Direction: "ltr", Status: "validated_model",
				DirectAnswer: packet.Facts[0].Text, FactRefs: []string{packet.Facts[0].FactID}, CitationRefs: packet.Facts[0].CitationIDs,
				KeyFindings: []NarrativeClaimV1{}, Comparisons: []NarrativeClaimV1{}, RelationshipSummary: []NarrativeClaimV1{},
				// The mock model must comply with the current schema contract,
				// which asks for short IDs (see narrativeIndexedEnumSchema), not
				// the literal limitation text — the server translates IDs back
				// to real text before validation runs.
				Limitations: narrativeIndexedIDs("L", len(packet.Limitations)), SuggestedQuestions: []string{}, Fallback: false,
			}
			for _, fact := range packet.Facts[1:] {
				narrative.KeyFindings = append(narrative.KeyFindings, NarrativeClaimV1{Text: fact.Text, ClaimType: fact.Kind, FactRefs: []string{fact.FactID}, CitationRefs: fact.CitationIDs})
			}
			content, _ := json.Marshal(narrative)
			_ = json.NewEncoder(writer).Encode(map[string]any{"choices": []map[string]any{{"message": map[string]string{"content": string(content)}}}})
		}))
		defer server.Close()
		for _, test := range corpus.SynthesisCases {
			narrative, reason, trace := synthesizeFactPacketNarrativeWithTrace(context.Background(), config{LocalAIURL: server.URL, SynthesisModel: corpus.ModelID, SynthesisTimeout: time.Second}, hybridQueryRequest{}, test.Packet)
			result := assessModelRoleNarrative(test, narrative, reason, trace)
			Expect(modelRoleSynthesisPassed(result)).To(BeTrue(), test.ID)
			Expect(trace.Audit.RequestSHA256).NotTo(BeEmpty())
			Expect(trace.Audit.ResponseSHA256).NotTo(BeEmpty())
			Expect(utf8.Valid(trace.RequestBytes)).To(BeTrue())
			Expect(utf8.Valid(trace.ResponseBytes)).To(BeTrue())
		}
	})

	It("runs one opt-in installed-model development adjudication", func() {
		if os.Getenv("NXB21_MODEL_ROLE_LIVE") != "1" {
			Skip("set NXB21_MODEL_ROLE_LIVE=1 for the reusable development adjudication")
		}
		corpus, err := loadModelRoleDevelopmentCorpus()
		Expect(err).NotTo(HaveOccurred())
		baseURL := strings.TrimSpace(os.Getenv("NXB21_MODEL_ROLE_BASE_URL"))
		if baseURL == "" {
			baseURL = "http://127.0.0.1:8080"
		}
		initialModels, err := modelRoleLoadedModels(baseURL)
		Expect(err).NotTo(HaveOccurred())
		initiallyLoaded := containsString(initialModels, corpus.ModelID)
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
		defer cancel()
		evidenceDir := strings.TrimSpace(os.Getenv("NXB21_MODEL_ROLE_EVIDENCE_DIR"))
		receipt := modelRoleDevelopmentReceiptV1{
			ContractVersion: "nexusai.model-role-development-adjudication/v1", CorpusID: corpus.CorpusID,
			CorpusSHA256: fmt.Sprintf("%x", sha256.Sum256(modelRoleDevelopmentCorpusJSON)), Purpose: corpus.Purpose, ModelID: corpus.ModelID,
			SynthesisResults: []modelRoleSynthesisResultV1{}, GeneralResults: []modelRoleGeneralResultV1{},
			ModelDownloadRequired: false, HoldoutCreated: false, HoldoutConsumed: false, StrictUTF8: true,
			Limitations: []string{"Development evidence is reusable and is not product certification.", "CPU latency is measured on one local run and is not a throughput benchmark."},
		}
		latencies := []int64{}
		allRaw := true
		for _, test := range corpus.SynthesisCases {
			narrative, reason, trace := synthesizeFactPacketNarrativeWithTrace(ctx, config{LocalAIURL: baseURL, SynthesisModel: corpus.ModelID, SynthesisTimeout: 120 * time.Second}, hybridQueryRequest{}, test.Packet)
			result := assessModelRoleNarrative(test, narrative, reason, trace)
			receipt.SynthesisResults = append(receipt.SynthesisResults, result)
			latencies = append(latencies, trace.Audit.LatencyMS)
			allRaw = allRaw && len(trace.RequestBytes) > 0 && len(trace.ResponseBytes) > 0 && utf8.Valid(trace.RequestBytes) && utf8.Valid(trace.ResponseBytes)
			Expect(writeModelRoleRawEvidence(evidenceDir, test.ID, trace.RequestBytes, trace.ResponseBytes)).To(Succeed())
		}
		cfg := config{LocalAIURL: baseURL, SynthesisModel: corpus.ModelID, SemanticSlotAssistModel: corpus.ModelID, SemanticSlotAssistEnabled: true, SemanticSlotAssistTimeout: semanticSlotAssistTimeoutMax}
		receipt.SlotAssistDecision, receipt.SlotAssistAudit = assistSemanticSlot(ctx, cfg, SemanticSlotAssistRequestV1{Question: corpus.SlotAssistCase.Question, Slot: corpus.SlotAssistCase.Slot, Options: corpus.SlotAssistCase.Options})
		for _, test := range corpus.GeneralCases {
			result, requestBytes, responseBytes := modelRoleGeneralCompletion(ctx, baseURL, corpus.ModelID, test.Question)
			result.ID = test.ID
			result = assessModelRoleGeneral(test, result)
			receipt.GeneralResults = append(receipt.GeneralResults, result)
			allRaw = allRaw && len(requestBytes) > 0 && len(responseBytes) > 0 && utf8.Valid(requestBytes) && utf8.Valid(responseBytes)
			Expect(writeModelRoleRawEvidence(evidenceDir, test.ID, requestBytes, responseBytes)).To(Succeed())
		}
		passedSynthesis, factualPreserved := 0, true
		for _, result := range receipt.SynthesisResults {
			if modelRoleSynthesisPassed(result) {
				passedSynthesis++
			}
			factualPreserved = factualPreserved && result.FactualAnswerPreserved
		}
		switch {
		case passedSynthesis == len(receipt.SynthesisResults):
			receipt.SynthesisRoleVerdict = "PASS"
		case passedSynthesis > 0 && factualPreserved:
			receipt.SynthesisRoleVerdict = "PASS_WITH_LIMITATIONS"
		default:
			receipt.SynthesisRoleVerdict = "FAIL"
		}
		if receipt.SlotAssistDecision.Status == "SELECTED" && receipt.SlotAssistDecision.Value == corpus.SlotAssistCase.ExpectedValue {
			receipt.SlotAssistRoleVerdict = "PASS"
		} else {
			receipt.SlotAssistRoleVerdict = "FAIL"
		}
		safeGeneral := 0
		for _, result := range receipt.GeneralResults {
			if result.Safe {
				safeGeneral++
			}
		}
		switch {
		case safeGeneral == len(receipt.GeneralResults):
			receipt.GeneralRoleVerdict = "PASS"
		case safeGeneral > 0:
			receipt.GeneralRoleVerdict = "PASS_WITH_LIMITATIONS"
		default:
			receipt.GeneralRoleVerdict = "FAIL"
		}
		receipt.SynthesisLatencyP50MS = modelRolePercentile(append([]int64(nil), latencies...), 0.50)
		receipt.SynthesisLatencyP95MS = modelRolePercentile(append([]int64(nil), latencies...), 0.95)
		receipt.RawBytesCaptured = allRaw
		if initiallyLoaded {
			receipt.UnloadState = "PRESERVED_PREEXISTING"
		} else if err := unloadDevelopmentModel(baseURL, corpus.ModelID); err != nil {
			receipt.UnloadState = "FAIL: " + err.Error()
		} else {
			receipt.UnloadState = "PASS"
		}
		if output := strings.TrimSpace(os.Getenv("NXB21_MODEL_ROLE_OUTPUT")); output != "" {
			output = filepath.Clean(output)
			payload, marshalErr := json.MarshalIndent(receipt, "", "  ")
			Expect(marshalErr).NotTo(HaveOccurred())
			Expect(os.MkdirAll(filepath.Dir(output), 0o755)).To(Succeed())
			Expect(os.WriteFile(output, append(payload, '\n'), 0o600)).To(Succeed())
		}
		Expect(receipt.RawBytesCaptured).To(BeTrue())
		Expect(receipt.UnloadState).NotTo(HavePrefix("FAIL"))
		Expect(factualPreserved).To(BeTrue())
	})
})
