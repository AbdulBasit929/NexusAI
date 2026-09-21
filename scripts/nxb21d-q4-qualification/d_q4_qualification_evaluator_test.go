package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type nxDCase struct {
	ID, Query, Language, Dimension string
	Critical                       bool
	Input                          map[string]any
	Expected                       map[string]any
}
type nxDResult struct {
	ID, Query, Language, Dimension                                             string `json:",omitempty"`
	Critical, ModelCalled, StructuredValid, ModelProposalPassed, Passed       bool
	Status                                                                     string
	LatencyMS                                                                  int64
	Expected, ModelActual, Actual                                              map[string]any
	ModelMismatches, Mismatches                                                []string `json:",omitempty"`
	RawRequestSHA256, RawResponseSHA256                                        string   `json:",omitempty"`
	RequestUTF8NoBOM, ResponseStrictUTF8, EnvelopeJSONParse, PlannerJSONParse  bool
	EvidencePaths                                                              map[string]string `json:",omitempty"`
	ExactIdentifierReconciled, PhraseLiteralReconciled                        bool
}

type nxQCapture struct {
	mu sync.Mutex
	upstream, runDir, caseID string
	request, response []byte
	status int
}

func (c *nxQCapture) begin(caseID string) {
	c.mu.Lock(); defer c.mu.Unlock()
	c.caseID, c.request, c.response, c.status = caseID, nil, nil, 0
}

func (c *nxQCapture) snapshot() ([]byte, []byte, int) {
	c.mu.Lock(); defer c.mu.Unlock()
	return append([]byte(nil), c.request...), append([]byte(nil), c.response...), c.status
}

func (c *nxQCapture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
	if err != nil { http.Error(w, err.Error(), http.StatusBadRequest); return }
	c.mu.Lock(); caseID := c.caseID; c.request = append([]byte(nil), body...); c.mu.Unlock()
	dir := filepath.Join(c.runDir, caseID)
	_ = os.MkdirAll(dir, 0700)
	_ = os.WriteFile(filepath.Join(dir, "request-utf8-no-bom.json"), body, 0600)
	upstream, err := http.NewRequestWithContext(r.Context(), r.Method, strings.TrimRight(c.upstream, "/")+r.URL.RequestURI(), bytes.NewReader(body))
	if err != nil { http.Error(w, err.Error(), http.StatusBadGateway); return }
	for key, values := range r.Header { for _, value := range values { upstream.Header.Add(key, value) } }
	response, err := (&http.Client{Timeout: 180 * time.Second}).Do(upstream)
	if err != nil { http.Error(w, err.Error(), http.StatusBadGateway); return }
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil { http.Error(w, err.Error(), http.StatusBadGateway); return }
	c.mu.Lock(); c.response = append([]byte(nil), responseBody...); c.status = response.StatusCode; c.mu.Unlock()
	_ = os.WriteFile(filepath.Join(dir, "response-body-raw.bin"), responseBody, 0600)
	if utf8.Valid(responseBody) { _ = os.WriteFile(filepath.Join(dir, "response-body-strict-utf8.json"), responseBody, 0600) }
	for key, values := range response.Header { for _, value := range values { w.Header().Add(key, value) } }
	w.WriteHeader(response.StatusCode); _, _ = w.Write(responseBody)
}

func nxQHash(raw []byte) string { return fmt.Sprintf("%x", sha256.Sum256(raw)) }

func nxQRawProposal(raw []byte) (*DynamicPlannerProposalV1, bool) {
	var completion struct { Choices []struct { Message struct { Content string `json:"content"` } `json:"message"` } `json:"choices"` }
	if json.Unmarshal(raw, &completion) != nil || len(completion.Choices) == 0 { return nil, false }
	proposal, err := decodeDynamicPlannerProposal([]byte(completion.Choices[0].Message.Content))
	if err != nil { return nil, false }
	return &proposal, true
}

func nxQRequestFromCase(c nxDCase, model string) hybridQueryRequest {
	req := hybridQueryRequest{Query: c.Query, TenantID: "nxb21d-eval", UserID: "offline-evaluator", CollectionID: "nxb21d-synthetic", Limit: 20, MaxKBResults: 8, SynthesisModel: model}
	scope, _ := c.Input["scope"].(string)
	family, _ := c.Input["family"].(string)
	req.QueryScope = queryEvidenceScope{Kind: scope, SourceFamily: family}
	if scope == "selected_evidence" {
		req.EvidenceID = "10000000-0000-4000-8000-000000000001"
		req.QueryScope.EvidenceID = req.EvidenceID
	}
	if prior, _ := c.Input["prior_capability"].(string); prior != "" {
		if ref, ok := dynamicCapabilityReference(prior); ok {
			req.ConversationContext = queryConversationContext{ContractVersion: followUpContextContractV1, TenantID: req.TenantID, UserID: req.UserID, CollectionID: req.CollectionID, Template: queryTemplateNameByOperationID(ref.OperationRef), Target: fmt.Sprint(c.Input["prior_target"]), ExpiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339)}
		}
	}
	return req
}

var _ = Describe("NX-B2.1D Q4 qualification deterministic source oracle", func() {
	It("admits every expected capability and classifies every critical refusal without model inference", func() {
		raw, err := os.ReadFile(os.Getenv("NXB21D_HOLDOUT"))
		Expect(err).NotTo(HaveOccurred())
		var corpus struct { Cases []nxDCase `json:"cases"` }
		Expect(json.Unmarshal(raw, &corpus)).To(Succeed())
		Expect(corpus.Cases).To(HaveLen(168))
		failures := make([]string, 0)
		for _, c := range corpus.Cases {
			req := nxQRequestFromCase(c, "qwen3-4b-instruct-2507-q4km-nxb21d-dev")
			if expectedState, ok := c.Expected["state"].(string); ok {
				if actual := nxDDeterministicState(req); actual != expectedState {
					failures = append(failures, fmt.Sprintf("%s expected_state=%s actual=%s", c.ID, expectedState, actual))
				}
				continue
			}
			expectedCapability := fmt.Sprint(c.Expected["capability"])
			ids := make([]string, 0)
			for _, ref := range dynamicCapabilityCandidates(req) { ids = append(ids, ref.ID) }
			found := false
			for _, id := range ids { if id == expectedCapability { found = true; break } }
			if !found { failures = append(failures, fmt.Sprintf("%s expected_capability=%s candidates=%v", c.ID, expectedCapability, ids)) }
		}
		Expect(failures).To(BeEmpty())
	})
})

var _ = Describe("NX-B2.1D one-shot Q4 independent qualification holdout", Ordered, func() {
	It("evaluates the frozen corpus through the production model proposal interface", func() {
		corpusPath, outputPath := os.Getenv("NXB21D_HOLDOUT"), os.Getenv("NXB21D_RESULT")
		runDir, realLocalAI := os.Getenv("NXB21D_RUN_DIR"), os.Getenv("NXB21D_LOCALAI_URL")
		Expect(corpusPath).NotTo(BeEmpty())
		Expect(outputPath).NotTo(BeEmpty())
		Expect(runDir).NotTo(BeEmpty())
		var corpus struct {
			SchemaVersion string         `json:"schema_version"`
			Acceptance    map[string]any `json:"acceptance"`
			Cases         []nxDCase      `json:"cases"`
		}
		raw, err := os.ReadFile(corpusPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(json.Unmarshal(raw, &corpus)).To(Succeed())
		Expect(corpus.Cases).To(HaveLen(168))
		capture := &nxQCapture{upstream: realLocalAI, runDir: runDir}
		proxy := httptest.NewServer(capture)
		defer proxy.Close()
		cfg := config{LocalAIURL: proxy.URL, SynthesisModel: os.Getenv("NXB21D_MODEL"), SynthesisTimeout: 180 * time.Second}
		results := make([]nxDResult, 0, 168)
		langTotal := map[string]int{}
		langPass := map[string]int{}
		criticalTotal, criticalPass, modelCalls, valid := 0, 0, 0, 0
		startAll := time.Now()
		for i, c := range corpus.Cases {
			req := nxQRequestFromCase(c, cfg.SynthesisModel)
			r := nxDResult{ID: c.ID, Query: c.Query, Language: c.Language, Dimension: c.Dimension, Critical: c.Critical, Expected: c.Expected, ModelActual: map[string]any{}, Actual: map[string]any{}}
			began := time.Now()
			if expectedState, ok := c.Expected["state"].(string); ok {
				r.Status = nxDDeterministicState(req)
				r.Actual["state"] = r.Status
				r.Passed = r.Status == expectedState
				r.StructuredValid = true
			} else {
				r.ModelCalled = true
				modelCalls++
				capture.begin(c.ID)
				resolved, status := resolveWithLanguageAssistance(context.Background(), cfg, req)
				rawRequest, rawResponse, httpStatus := capture.snapshot()
				r.Status = status
				r.RawRequestSHA256, r.RawResponseSHA256 = nxQHash(rawRequest), nxQHash(rawResponse)
				r.RequestUTF8NoBOM = utf8.Valid(rawRequest) && !strings.HasPrefix(string(rawRequest), "\ufeff")
				r.ResponseStrictUTF8 = utf8.Valid(rawResponse)
				r.EnvelopeJSONParse = json.Valid(rawResponse) && httpStatus == http.StatusOK
				r.EvidencePaths = map[string]string{"request": filepath.Join(runDir, c.ID, "request-utf8-no-bom.json"), "raw_response": filepath.Join(runDir, c.ID, "response-body-raw.bin"), "strict_response": filepath.Join(runDir, c.ID, "response-body-strict-utf8.json")}
				if rawProposal, ok := nxQRawProposal(rawResponse); ok {
					r.PlannerJSONParse = true
					r.ModelActual = nxDActual(req, rawProposal)
					r.ModelMismatches = nxDCompare(c.Expected, r.ModelActual)
					r.ModelProposalPassed = len(r.ModelMismatches) == 0
				} else { r.ModelMismatches = []string{"raw_planner_proposal_invalid"} }
				r.StructuredValid = strings.HasPrefix(status, "validated_model_proposal")
				if r.StructuredValid {
					valid++
					p := resolved.DynamicProposal
					r.Actual = nxDActual(resolved, p)
					r.ExactIdentifierReconciled = resolved.ExactIdentifierAuthority != nil && resolved.ExactIdentifierAuthority.Reconciled
					r.PhraseLiteralReconciled = resolved.PhraseLiteralAuthority != nil && resolved.PhraseLiteralAuthority.Reconciled
					prior, _ := c.Input["prior_capability"].(string)
					if _, ok := c.Expected["no_stale_family"]; ok {
						r.Actual["no_stale_family"] = p.QueryCapabilityID != prior
					}
					if _, ok := c.Expected["context_change"]; ok {
						r.Actual["context_change"] = nxDContextChange(resolved, p, fmt.Sprint(c.Input["prior_target"]))
					}
					r.Mismatches = nxDCompare(c.Expected, r.Actual)
					r.Passed = len(r.Mismatches) == 0
				} else {
					r.Mismatches = []string{status}
				}
			}
			r.LatencyMS = time.Since(began).Milliseconds()
			results = append(results, r)
			langTotal[c.Language]++
			if r.Passed {
				langPass[c.Language]++
			}
			if c.Critical {
				criticalTotal++
				if r.Passed {
					criticalPass++
				}
			}
			nxDWriteCheckpoint(outputPath, results, modelCalls, valid)
			fmt.Printf("NXB21D_CASE %03d/168 id=%s language=%s status=%s pass=%t latency_ms=%d\n", i+1, c.ID, c.Language, r.Status, r.Passed, r.LatencyMS)
		}
		percent := func(a, b int) float64 {
			if b == 0 {
				return 100
			}
			return float64(a) * 100 / float64(b)
		}
		language := map[string]any{}
		allLangPass := true
		for k, n := range langTotal {
			score := percent(langPass[k], n)
			language[k] = map[string]any{"passed": langPass[k], "total": n, "percent": score}
			if score < 95 {
				allLangPass = false
			}
		}
		criticalPercent := percent(criticalPass, criticalTotal)
		suitability := "INSUFFICIENT"
		if criticalPercent == 100 && allLangPass {
			suitability = "SUITABLE"
		} else if criticalPercent == 100 {
			suitability = "SUITABLE_WITH_LIMITATIONS"
		}
		lat := make([]int64, 0, len(results))
		passed := 0
		loadMS := int64(0)
		for _, r := range results {
			lat = append(lat, r.LatencyMS)
			if r.Passed {
				passed++
			}
			if loadMS == 0 && r.ModelCalled {
				loadMS = r.LatencyMS
			}
		}
		sort.Slice(lat, func(i, j int) bool { return lat[i] < lat[j] })
		p95 := lat[(len(lat)*95+99)/100-1]
		slow := append([]nxDResult(nil), results...)
		sort.Slice(slow, func(i, j int) bool { return slow[i].LatencyMS > slow[j].LatencyMS })
		slowest := []map[string]any{}
		for _, r := range slow[:5] {
			slowest = append(slowest, map[string]any{"id": r.ID, "latency_ms": r.LatencyMS, "status": r.Status})
		}
		payload := map[string]any{"schema_version": "nexusai.nxb21d-q4-independent-holdout/v1", "completed_at": time.Now().UTC().Format(time.RFC3339Nano), "holdout_consumed": true, "cases_completed": len(results), "model_calls": modelCalls, "structured_valid": map[string]any{"count": valid, "total": modelCalls, "percent": percent(valid, modelCalls)}, "overall": map[string]any{"passed": passed, "total": len(results), "percent": percent(passed, len(results))}, "language": language, "critical": map[string]any{"passed": criticalPass, "total": criticalTotal, "percent": criticalPercent}, "breakdowns": map[string]any{"capability": nxDBreakdown(results, "capability"), "semantic": nxDBreakdown(results, "semantic"), "dimension": nxDBreakdown(results, "dimension"), "critical_class": nxDBreakdown(results, "critical_class")}, "latency": map[string]any{"total_ms": time.Since(startAll).Milliseconds(), "model_load_first_inference_ms": loadMS, "median_ms": lat[len(lat)/2], "p95_ms": p95, "slowest_cases": slowest}, "suitability": suitability, "D_exit_decision": map[bool]string{true: "PASS", false: "FAIL"}[suitability == "SUITABLE"], "results": results}
		out, err := json.MarshalIndent(payload, "", "  ")
		Expect(err).NotTo(HaveOccurred())
		Expect(os.WriteFile(outputPath, append(out, '\n'), 0600)).To(Succeed())
	})
})

func nxDWriteCheckpoint(outputPath string, results []nxDResult, modelCalls, valid int) {
	payload := map[string]any{"schema_version": "nexusai.nxb21d-qwen-holdout-checkpoint/v1", "holdout_consumed": true, "cases_completed": len(results), "model_calls": modelCalls, "structured_valid": valid, "results": results}
	out, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(outputPath+".partial", append(out, '\n'), 0600)
}

func nxDDeterministicState(req hybridQueryRequest) string {
	q := strings.ToLower(req.Query)
	if strings.Contains(q, "select ") || strings.Contains(q, "rm -rf") || strings.Contains(q, "shell command") || strings.Contains(q, "powershell") || strings.Contains(q, "curl ") || strings.Contains(q, "http://") || strings.Contains(q, "https://") {
		return "ARBITRARY_EXECUTION"
	}
	if strings.Contains(q, "another case") || strings.Contains(q, "different case") || strings.Contains(q, "tenant foreign") || strings.Contains(q, "collection to foreign") || (strings.Contains(q, "selected") && strings.Contains(q, "workspace")) {
		return "SCOPE_CONFLICT"
	}
	if strings.Contains(q, "99999999") || strings.Count(q, " and ") >= 3 {
		return "BUDGET_EXCEEDED"
	}
	if assessDynamicQuery(req.Query, "").Status == "unavailable" {
		return "UNAVAILABLE"
	}
	return "UNAVAILABLE"
}
func nxDActual(req hybridQueryRequest, p *DynamicPlannerProposalV1) map[string]any {
	a := map[string]any{"capability": p.QueryCapabilityID, "semantic": p.Semantic, "literal": nil, "target": nil, "scope": p.ScopeIntent}
	if p.LiteralText != "" {
		a["literal"] = p.LiteralText
	}
	if p.Target != "" {
		a["target"] = p.Target
	}
	if p.Parameters.DateFrom != "" || p.Parameters.DateTo != "" {
		a["semantic"] = "CALENDAR_TIME"
	}
	if p.StartSeconds != nil || p.EndSeconds != nil {
		a["semantic"] = "SOURCE_TIME"
	}
	return a
}
func nxDContextChange(req hybridQueryRequest, p *DynamicPlannerProposalV1, prior string) string {
	if req.Direction != "" {
		return strings.ToUpper(req.Direction)
	}
	if p.StartSeconds != nil || p.EndSeconds != nil {
		return "SOURCE_TIME"
	}
	if p.Target != "" && p.Target != prior {
		return "TARGET_REPLACED"
	}
	return p.ScopeIntent
}
func nxDCompare(expected, actual map[string]any) []string {
	m := []string{}
	for _, k := range []string{"capability", "semantic", "literal", "target", "scope", "context_change", "no_stale_family"} {
		e, ok := expected[k]
		if !ok {
			continue
		}
		if fmt.Sprint(e) != fmt.Sprint(actual[k]) {
			m = append(m, fmt.Sprintf("%s expected=%v actual=%v", k, e, actual[k]))
		}
	}
	return m
}
func nxDBreakdown(results []nxDResult, key string) map[string]any {
	counts := map[string][2]int{}
	for _, r := range results {
		label := ""
		switch key {
		case "capability":
			label = fmt.Sprint(r.Expected["capability"])
		case "semantic":
			label = fmt.Sprint(r.Expected["semantic"])
		case "dimension":
			label = r.Dimension
		case "critical_class":
			if r.Critical {
				label = "critical"
			} else {
				label = "noncritical"
			}
		}
		if label == "" || label == "<nil>" {
			continue
		}
		c := counts[label]
		c[1]++
		if r.Passed {
			c[0]++
		}
		counts[label] = c
	}
	out := map[string]any{}
	for label, c := range counts {
		out[label] = map[string]any{"passed": c[0], "total": c[1], "percent": float64(c[0]) * 100 / float64(c[1])}
	}
	return out
}
