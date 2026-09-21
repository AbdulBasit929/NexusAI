package main

import (
	"encoding/json"
	"os"
	"sort"
	"testing"
)

// TestEnglishFunctionalPromptFootprint measures the frozen semantic request
// shape without dispatching inference. It intentionally reproduces the current
// request serialization so resource adjudication is based on bytes, not an
// estimate from question length alone.
func TestEnglishFunctionalPromptFootprint(t *testing.T) {
	corpusPath := os.Getenv("NEXUSAI_ENGLISH_CORPUS")
	if corpusPath == "" {
		t.Skip("NEXUSAI_ENGLISH_CORPUS is required for frozen-corpus diagnostics")
	}
	var corpus englishQualificationCorpus
	readStrictJSON(t, corpusPath, &corpus)
	type measurement struct {
		ID             string `json:"id"`
		CandidateCount int    `json:"candidate_count"`
		PromptBytes    int    `json:"prompt_bytes"`
		RequestBytes   int    `json:"request_bytes"`
		TokenEstimate  int    `json:"token_estimate"`
	}
	measurements := make([]measurement, 0, len(corpus.SemanticCases))
	for _, c := range corpus.SemanticCases {
		req := hybridQueryRequest{
			TenantID: "qualification-tenant", UserID: "qualification-analyst",
			CollectionID: "qualification-case", Query: c.Question, Limit: 20,
			MaxKBResults: 8, SynthesisModel: "qwen_qwen3-4b-instruct-2507",
			QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)},
		}
		candidates := semanticOperationCandidates(req)
		candidateJSON, err := json.Marshal(semanticOperationPromptCandidates(candidates))
		if err != nil {
			t.Fatal(err)
		}
		system := "Map the analyst's unrestricted English wording to exactly one server-issued semantic operation. Reason from intent, meaning, requested measure, grouping, result shape, and evidence family; do not rely on memorized sentence templates. Return only {\"decision\":\"<allowed enum>\"}. The server alone owns scope, identifiers, dates, filters, limits, facts, tools, and execution. Choose clarification when meaning or required facts are genuinely ambiguous, and unsupported when no issued operation matches."
		user := "Server-issued operation meanings:\n" + string(candidateJSON) + "\nAnalyst question:\n" + req.Query
		body, err := json.Marshal(map[string]any{
			"model": req.SynthesisModel, "temperature": 0, "max_tokens": 96,
			"response_format": map[string]any{"type": "json_schema", "json_schema": map[string]any{"name": "forensic_semantic_operation", "strict": true, "schema": semanticOperationSchema(candidates)}},
			"messages":        []map[string]string{{"role": "system", "content": system}, {"role": "user", "content": user}},
		})
		if err != nil {
			t.Fatal(err)
		}
		promptBytes := len([]byte(system)) + len([]byte(user))
		measurements = append(measurements, measurement{
			ID: c.ID, CandidateCount: len(candidates), PromptBytes: promptBytes,
			RequestBytes: len(body), TokenEstimate: (promptBytes + 3) / 4,
		})
	}
	if len(measurements) == 0 {
		t.Fatal("frozen corpus has no semantic cases")
	}
	sort.Slice(measurements, func(i, j int) bool {
		if measurements[i].PromptBytes == measurements[j].PromptBytes {
			return measurements[i].ID < measurements[j].ID
		}
		return measurements[i].PromptBytes < measurements[j].PromptBytes
	})
	report := map[string]any{
		"contract_version": "nexusai.english-functional-resource-diagnostics/v1",
		"corpus_id":        corpus.CorpusID,
		"semantic_cases":   len(measurements),
		"minimum":          measurements[0],
		"p50":              measurements[(len(measurements)-1)/2],
		"maximum":          measurements[len(measurements)-1],
	}
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(string(data))
}
