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
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mudler/LocalAI/pkg/httpclient"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type nxHybridCase struct {
	ID        string `json:"id"`
	Query     string `json:"query"`
	Language  string `json:"language"`
	ModelCall bool   `json:"model_call"`
	Critical  bool   `json:"critical"`
	Input     struct {
		Scope           string `json:"scope"`
		Family          string `json:"family"`
		PriorCapability string `json:"prior_capability"`
		PriorTarget     string `json:"prior_target"`
	} `json:"input"`
	Expected                  map[string]any `json:"expected"`
	ExpectedFinalTypedOutcome map[string]any `json:"expected_final_typed_outcome"`
}

func nxHybridProjection(req hybridQueryRequest, status string) map[string]any {
	a := map[string]any{"state": status}
	if req.HybridAudit != nil && req.HybridAudit.FinalPlan != nil {
		p := req.HybridAudit.FinalPlan
		a = map[string]any{"capability": p.QueryCapabilityID, "semantic": p.Semantic, "scope": p.ScopeIntent, "target": req.Target, "targets": req.Targets, "literal": p.LiteralText, "date_from": req.DateFrom, "date_to": req.DateTo, "direction": req.Direction, "top_k": p.TopK, "transition": req.HybridAudit.Facts.Transition, "representation": ""}
		if req.TextQuery != nil {
			a["representation"] = req.TextQuery.Representation
		}
		if req.StartSeconds != nil {
			a["start_seconds"] = *req.StartSeconds
		}
		if req.EndSeconds != nil {
			a["end_seconds"] = *req.EndSeconds
		}
	}
	return a
}

func nxHybridCompare(expected, actual map[string]any) []string {
	out := []string{}
	for k, v := range expected {
		e, _ := json.Marshal(v)
		a, _ := json.Marshal(actual[k])
		if !bytes.Equal(e, a) {
			out = append(out, fmt.Sprintf("%s expected=%s actual=%s", k, e, a))
		}
	}
	return out
}

var _ = Describe("NXB21D hybrid standalone development", func() {
	It("evaluates the frozen gate only on explicit standalone invocation", func() {
		if os.Getenv("NXB21D_HYBRID_LIVE") != "1" {
			Skip("operator-only development gate; no inference in source tests")
		}
		root := os.Getenv("NXB21D_HYBRID_RUN_DIR")
		Expect(root).NotTo(BeEmpty())
		raw, err := os.ReadFile(os.Getenv("NXB21D_HYBRID_CORPUS"))
		Expect(err).NotTo(HaveOccurred())
		var corpus struct {
			Cases []nxHybridCase `json:"cases"`
		}
		Expect(json.Unmarshal(raw, &corpus)).To(Succeed())
		Expect(corpus.Cases).To(HaveLen(32))
		results := []map[string]any{}
		passed, calls, modelCorrect := 0, 0, 0
		write := func(path string, value any) {
			b, e := json.MarshalIndent(value, "", "  ")
			Expect(e).NotTo(HaveOccurred())
			Expect(os.WriteFile(path, append(b, '\n'), 0600)).To(Succeed())
		}
		seal := func() {
			gate := "FAIL"
			if passed == 32 && len(results) == 32 && calls == 6 && modelCorrect == 6 {
				gate = "PASS"
			}
			write(filepath.Join(root, "hybrid-development-intermediate-v1.json"), map[string]any{"development_gate": gate, "development_only": true, "qualification_holdout": false, "cases_completed": len(results), "passed": passed, "model_calls": calls, "raw_tuple_correct": modelCorrect, "results": results, "D_status": "OPEN", "activation": "BLOCKED"})
		}
		defer seal()
		for i, c := range corpus.Cases {
			dir := filepath.Join(root, c.ID)
			Expect(os.MkdirAll(dir, 0700)).To(Succeed())
			modelCalled := false
			transportOK := true
			requestHash, responseHash := "", ""
			var metrics map[string]any
			proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				modelCalled = true
				body, e := io.ReadAll(io.LimitReader(r.Body, 1<<20))
				if e != nil {
					http.Error(w, "request read", 502)
					return
				}
				requestHash = fmt.Sprintf("%x", sha256.Sum256(body))
				_ = os.WriteFile(filepath.Join(dir, "request-utf8-no-bom.json"), body, 0600)
				transportOK = utf8.Valid(body) && !bytes.HasPrefix(body, []byte{0xef, 0xbb, 0xbf})
				req, e := http.NewRequestWithContext(r.Context(), "POST", os.Getenv("NXB21D_LOCALAI_URL")+"/v1/chat/completions", bytes.NewReader(body))
				if e != nil {
					http.Error(w, "request build", 502)
					return
				}
				req.Header.Set("Content-Type", "application/json")
				res, e := httpclient.NewWithTimeout(180 * time.Second).Do(req)
				if e != nil {
					transportOK = false
					http.Error(w, e.Error(), 502)
					return
				}
				defer res.Body.Close()
				response, e := io.ReadAll(io.LimitReader(res.Body, (1<<20)+1))
				if e != nil {
					transportOK = false
					http.Error(w, "response read", 502)
					return
				}
				responseHash = fmt.Sprintf("%x", sha256.Sum256(response))
				_ = os.WriteFile(filepath.Join(dir, "response-body-raw.bin"), response, 0600)
				transportOK = transportOK && utf8.Valid(response) && json.Valid(response) && res.StatusCode == 200
				if transportOK {
					_ = os.WriteFile(filepath.Join(dir, "response-body-strict-utf8.json"), response, 0600)
				}
				_ = json.Unmarshal(response, &metrics)
				w.WriteHeader(res.StatusCode)
				_, _ = w.Write(response)
			}))
			req := hybridQueryRequest{Query: c.Query, TenantID: "hybrid-eval", UserID: "operator", CollectionID: "synthetic", Limit: 20, MaxKBResults: 8, SynthesisModel: os.Getenv("NXB21D_MODEL"), QueryScope: queryEvidenceScope{Kind: c.Input.Scope, SourceFamily: c.Input.Family}}
			if c.Input.Scope == "selected_evidence" {
				req.EvidenceID = "10000000-0000-4000-8000-000000000001"
			}
			if c.Input.PriorCapability != "" {
				ref, ok := dynamicCapabilityReference(c.Input.PriorCapability)
				Expect(ok).To(BeTrue())
				req.ConversationContext = queryConversationContext{ContractVersion: followUpContextContractV1, TenantID: req.TenantID, UserID: req.UserID, CollectionID: req.CollectionID, Template: queryTemplateNameByOperationID(ref.OperationRef), Target: c.Input.PriorTarget, ExpiresAt: time.Now().Add(time.Hour).Format(time.RFC3339)}
			}
			// RAM is checked only for actual residual candidates, before dispatch.
			fq, f := extractHybridFacts(req)
			if len(buildHybridTuples(fq, f)) > 1 {
				args := []string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-File", os.Getenv("NXB21D_HYBRID_RAM_SCRIPT"), "-CompletedCalls", fmt.Sprint(calls)}
				output, e := exec.Command("powershell", args...).CombinedOutput()
				_ = os.WriteFile(filepath.Join(dir, "ram-precheck.txt"), output, 0600)
				if e != nil {
					proxy.Close()
					results = append(results, map[string]any{"id": c.ID, "passed": false, "state": "RAM_GATE_FAILED", "error": string(output)})
					return
				}
			}
			fmt.Printf("NXB21D_HYBRID_CASE %02d/32 id=%s language=%s\n", i+1, c.ID, c.Language)
			start := time.Now()
			out, status := resolveHybridPlanner(context.Background(), config{LocalAIURL: proxy.URL}, req)
			latency := time.Since(start).Milliseconds()
			proxy.Close()
			if modelCalled {
				calls++
			}
			actual := nxHybridProjection(out, status)
			mismatches := nxHybridCompare(c.Expected, actual)
			if modelCalled != c.ModelCall {
				mismatches = append(mismatches, "model-call contract mismatch")
			}
			if !transportOK {
				mismatches = append(mismatches, "transport failed")
			}
			if out.HybridAudit == nil {
				mismatches = append(mismatches, "audit missing")
			}
			pass := len(mismatches) == 0
			if pass {
				passed++
			}
			// Score residual decisions separately from final assembly correctness.
			rawCorrect := false
			if modelCalled && out.HybridAudit != nil && out.HybridAudit.ModelDecision != nil {
				d := out.HybridAudit.ModelDecision
				if c.Expected["state"] == "AMBIGUOUS_INTENT" {
					rawCorrect = d.AmbiguityState == "NEEDS_CLARIFICATION" && d.ClarificationCode == "AMBIGUOUS_INTENT"
				} else {
					want := fmt.Sprintf("%s/%s/%s", c.Expected["capability"], c.Expected["semantic"], c.Expected["scope"])
					rawCorrect = d.AmbiguityState == "RESOLVED" && string(d.SelectedTupleID) == want
				}
				if rawCorrect {
					modelCorrect++
				}
			}
			write(filepath.Join(dir, "three-layer-audit.json"), out.HybridAudit)
			result := map[string]any{"id": c.ID, "language": c.Language, "passed": pass, "status": status, "expected": c.Expected, "actual": actual, "mismatches": mismatches, "model_called": modelCalled, "transport_pass": transportOK, "request_sha256": requestHash, "response_sha256": responseHash, "latency_ms": latency, "completion": metrics}
			write(filepath.Join(dir, "case-result.json"), result)
			results = append(results, result)
			seal()
			fmt.Printf("NXB21D_HYBRID_RESULT id=%s pass=%t state=%s model_called=%t latency_ms=%d\n", c.ID, pass, status, modelCalled, latency)
			if !transportOK || strings.HasPrefix(status, "malformed") || status == "incomplete_completion" {
				return
			}
		}
	})
})

var _ = Describe("NXB21D hybrid corpus source admission", func() {
	It("checks frozen oracles and dispatch expectations without HTTP or inference", func() {
		path := os.Getenv("NXB21D_HYBRID_CORPUS_CHECK")
		if path == "" {
			Skip("optional read-only corpus admission")
		}
		raw, err := os.ReadFile(path)
		Expect(err).NotTo(HaveOccurred())
		var corpus struct {
			Cases []nxHybridCase `json:"cases"`
		}
		Expect(json.Unmarshal(raw, &corpus)).To(Succeed())
		failures := []string{}
		for _, c := range corpus.Cases {
			req := hybridQueryRequest{Query: c.Query, TenantID: "t", UserID: "u", CollectionID: "c", QueryScope: queryEvidenceScope{Kind: c.Input.Scope, SourceFamily: c.Input.Family}}
			if c.Input.Scope == "selected_evidence" {
				req.EvidenceID = "10000000-0000-4000-8000-000000000001"
			}
			if c.Input.PriorCapability != "" {
				r, ok := dynamicCapabilityReference(c.Input.PriorCapability)
				Expect(ok).To(BeTrue())
				req.ConversationContext = queryConversationContext{ContractVersion: followUpContextContractV1, TenantID: "t", UserID: "u", CollectionID: "c", Template: queryTemplateNameByOperationID(r.OperationRef), Target: c.Input.PriorTarget}
			}
			req, f := extractHybridFacts(req)
			tuples := buildHybridTuples(req, f)
			if (len(tuples) > 1) != c.ModelCall {
				failures = append(failures, c.ID+": model-call mismatch")
			}
			if c.ModelCall && c.Expected["state"] != nil {
				continue
			}
			if c.Expected["state"] != nil {
				if f.State != c.Expected["state"] {
					failures = append(failures, fmt.Sprintf("%s state expected=%s actual=%s", c.ID, c.Expected["state"], f.State))
				}
				continue
			}
			found := false
			for _, tuple := range tuples {
				if tuple.Capability != c.Expected["capability"] || tuple.Semantic != c.Expected["semantic"] {
					continue
				}
				found = true
				out, e := assembleHybridPlan(req, f, tuple)
				if e != nil {
					failures = append(failures, c.ID+": "+e.Error())
					continue
				}
				out.HybridAudit = &HybridPlannerAuditV1{Facts: f, FinalPlan: out.DynamicProposal}
				for _, m := range nxHybridCompare(c.Expected, nxHybridProjection(out, "")) {
					failures = append(failures, c.ID+": "+m)
				}
			}
			if !found {
				failures = append(failures, fmt.Sprintf("%s expected tuple absent; facts=%+v", c.ID, f))
			}
		}
		Expect(failures).To(BeEmpty())
	})
})
