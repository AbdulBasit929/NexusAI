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

var _ = Describe("NXB21D final controlled local Qwen3 8B selection12 standalone development", func() {
	It("evaluates the frozen model-selection gate only on explicit standalone invocation", func() {
		if os.Getenv("NXB21D_FINAL_LOCAL12_LIVE") != "1" {
			Skip("operator-only development gate; no inference in source tests")
		}
		root := os.Getenv("NXB21D_FINAL_LOCAL12_RUN_DIR")
		Expect(root).NotTo(BeEmpty())
		raw, err := os.ReadFile(os.Getenv("NXB21D_FINAL_LOCAL12_CORPUS"))
		Expect(err).NotTo(HaveOccurred())
		var corpus struct {
			Cases []nxHybridCase `json:"cases"`
		}
		Expect(json.Unmarshal(raw, &corpus)).To(Succeed())
		Expect(corpus.Cases).To(HaveLen(12))
		var bindings map[string]struct {
			Schema map[string]any `json:"schema"`
		}
		bindingBytes, err := os.ReadFile(os.Getenv("NXB21D_FINAL_LOCAL12_BINDINGS"))
		Expect(err).NotTo(HaveOccurred())
		Expect(json.Unmarshal(bindingBytes, &bindings)).To(Succeed())
		results := []map[string]any{}
		modelLoadStartedAt, modelLoadCompletedAt := "", ""
		passed, calls, modelCorrect := 0, 0, 0
		write := func(path string, value any) {
			blob, marshalErr := json.MarshalIndent(value, "", "  ")
			Expect(marshalErr).NotTo(HaveOccurred())
			Expect(os.WriteFile(path, append(blob, '\n'), 0600)).To(Succeed())
		}
		sealIntermediate := func() {
			gate := "FAIL"
			if passed == 12 && len(results) == 12 && calls == 12 && modelCorrect == 12 {
				gate = "PASS"
			}
			counts := map[string]int{"http_success": 0, "strict_utf8": 0, "schema_valid": 0, "finish_reason_stop": 0, "malformed": 0, "unknown_decision": 0, "truncation": 0, "fact_override": 0, "scope_override": 0, "authorization_override": 0}
			for _, result := range results {
				for _, key := range []string{"http_success", "strict_utf8", "schema_valid", "finish_reason_stop"} {
					if value, ok := result[key].(bool); ok && value {
						counts[key]++
					}
				}
				for _, key := range []string{"malformed", "unknown_decision", "truncation", "fact_override", "scope_override", "authorization_override"} {
					if value, ok := result[key].(bool); ok && value {
						counts[key]++
					}
				}
			}
			write(filepath.Join(root, "selection12-development-intermediate-v1.json"), map[string]any{
				"development_gate": gate, "development_only": true, "qualification_holdout": false,
				"cases_completed": len(results), "passed": passed, "model_calls": calls,
				"raw_tuple_correct": modelCorrect, "acceptance_counts": counts, "results": results, "D_status": "OPEN", "activation": "BLOCKED",
				"model_load_started_at": modelLoadStartedAt, "model_load_completed_at": modelLoadCompletedAt,
			})
		}
		defer sealIntermediate()
		for index, item := range corpus.Cases {
			dir := filepath.Join(root, item.ID)
			Expect(os.MkdirAll(dir, 0700)).To(Succeed())
			modelCalled := false
			transportOK := true
			httpSuccess, strictUTF8 := false, false
			requestHash, responseHash := "", ""
			var metrics map[string]any
			proxy := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if calls == 0 {
					lockPath := os.Getenv("NXB21D_FINAL_LOCAL12_DISPATCH_LOCK")
					claim, claimErr := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
					if claimErr != nil {
						http.Error(writer, "one-shot dispatch lock: "+claimErr.Error(), http.StatusConflict)
						return
					}
					_, claimErr = claim.WriteString(root + "\n")
					closeErr := claim.Close()
					if claimErr != nil || closeErr != nil {
						http.Error(writer, "one-shot dispatch lock write", http.StatusInternalServerError)
						return
					}
				}
				modelCalled = true
				body, readErr := io.ReadAll(io.LimitReader(request.Body, 1<<20))
				if readErr != nil {
					http.Error(writer, "request read", http.StatusBadGateway)
					return
				}
				requestHash = fmt.Sprintf("%x", sha256.Sum256(body))
				if writeErr := os.WriteFile(filepath.Join(dir, "request-utf8-no-bom.json"), body, 0600); writeErr != nil {
					transportOK = false
					http.Error(writer, "request evidence write failed", http.StatusBadGateway)
					return
				}
				transportOK = utf8.Valid(body) && !bytes.HasPrefix(body, []byte{0xef, 0xbb, 0xbf})
				if calls == 0 {
					modelLoadStartedAt = time.Now().UTC().Format(time.RFC3339Nano)
				}
				upstream, requestErr := http.NewRequestWithContext(request.Context(), http.MethodPost, os.Getenv("NXB21D_LOCALAI_URL")+"/v1/chat/completions", bytes.NewReader(body))
				if requestErr != nil {
					http.Error(writer, "request build", http.StatusBadGateway)
					return
				}
				upstream.Header.Set("Content-Type", "application/json")
				response, requestErr := httpclient.NewWithTimeout(180 * time.Second).Do(upstream)
				if requestErr != nil {
					transportOK = false
					http.Error(writer, requestErr.Error(), http.StatusBadGateway)
					return
				}
				defer response.Body.Close()
				responseBody, readErr := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
				if calls == 0 {
					modelLoadCompletedAt = time.Now().UTC().Format(time.RFC3339Nano)
				}
				if readErr != nil {
					transportOK = false
					http.Error(writer, "response read", http.StatusBadGateway)
					return
				}
				responseHash = fmt.Sprintf("%x", sha256.Sum256(responseBody))
				if writeErr := os.WriteFile(filepath.Join(dir, "response-body-raw.bin"), responseBody, 0600); writeErr != nil {
					transportOK = false
					http.Error(writer, "response evidence write failed", http.StatusBadGateway)
					return
				}
				httpSuccess = response.StatusCode == http.StatusOK
				strictUTF8 = utf8.Valid(responseBody) && !bytes.HasPrefix(responseBody, []byte{0xef, 0xbb, 0xbf})
				transportOK = transportOK && strictUTF8 && json.Valid(responseBody) && httpSuccess
				if transportOK {
					if writeErr := os.WriteFile(filepath.Join(dir, "response-body-strict-utf8.json"), responseBody, 0600); writeErr != nil {
						transportOK = false
					}
				}
				_ = json.Unmarshal(responseBody, &metrics)
				writer.WriteHeader(response.StatusCode)
				_, _ = writer.Write(responseBody)
			}))
			req := hybridQueryRequest{
				Query: item.Query, TenantID: "selection12-eval", UserID: "operator", CollectionID: "synthetic",
				Limit: 20, MaxKBResults: 8, SynthesisModel: os.Getenv("NXB21D_MODEL"),
				QueryScope: queryEvidenceScope{Kind: item.Input.Scope, SourceFamily: item.Input.Family},
			}
			if item.Input.Scope == "selected_evidence" {
				req.EvidenceID = "10000000-0000-4000-8000-000000000001"
			}
			if item.Input.PriorCapability != "" {
				ref, ok := dynamicCapabilityReference(item.Input.PriorCapability)
				Expect(ok).To(BeTrue())
				req.ConversationContext = queryConversationContext{
					ContractVersion: followUpContextContractV1, TenantID: req.TenantID, UserID: req.UserID,
					CollectionID: req.CollectionID, Template: queryTemplateNameByOperationID(ref.OperationRef),
					Target: item.Input.PriorTarget, ExpiresAt: time.Now().Add(time.Hour).Format(time.RFC3339),
				}
			}
			factRequest, facts := extractHybridFacts(req)
			factsBefore, _ := json.Marshal(facts)
			tuples := buildHybridTuples(factRequest, facts)
			candidatesBefore, _ := json.Marshal(tuples)
			actualSchema, _ := json.Marshal(hybridResidualSchema(tuples))
			frozenSchema, _ := json.Marshal(bindings[item.ID].Schema)
			Expect(actualSchema).To(Equal(frozenSchema), "frozen production decision schema drift")
			ramPrecheck, ramInitial, ramSettled, ramPostcall := "", "", "", ""
			if len(tuples) > 1 {
				stage := "PRECALL_RAM"
				if calls == 0 {
					stage = "PRELOAD_RAM"
				}
				args := []string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-File", os.Getenv("NXB21D_FINAL_LOCAL12_RAM_SCRIPT"), "-Stage", stage, "-CompletedCalls", fmt.Sprint(calls), "-GovernedRamScript", os.Getenv("NXB21D_FINAL_LOCAL12_GOVERNED_RAM_SCRIPT")}
				output, commandErr := exec.Command("powershell", args...).CombinedOutput()
				ramPrecheck = string(output)
				_ = os.WriteFile(filepath.Join(dir, "ram-precheck.txt"), output, 0600)
				if commandErr != nil {
					proxy.Close()
					results = append(results, map[string]any{"id": item.ID, "passed": false, "state": "RAM_GATE_FAILED", "error": string(output)})
					Fail(stage + " failed before dispatch: " + string(output))
				}
			}
			fmt.Printf("NXB21D_FINAL_LOCAL12_CASE %02d/12 id=%s language=%s\n", index+1, item.ID, item.Language)
			started := time.Now()
			out, status := resolveHybridPlanner(context.Background(), config{LocalAIURL: proxy.URL}, req)
			latency := time.Since(started).Milliseconds()
			proxy.Close()
			if modelCalled {
				calls++
			}
			if modelCalled && index == 0 {
				initialArgs := []string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-File", os.Getenv("NXB21D_FINAL_LOCAL12_RAM_SCRIPT"), "-Stage", "MODEL_LOADED_RAM_INITIAL", "-CompletedCalls", fmt.Sprint(calls), "-GovernedRamScript", os.Getenv("NXB21D_FINAL_LOCAL12_GOVERNED_RAM_SCRIPT")}
				initialOutput, initialErr := exec.Command("powershell", initialArgs...).CombinedOutput()
				ramInitial = string(initialOutput)
				_ = os.WriteFile(filepath.Join(dir, "ram-model-loaded-initial.txt"), initialOutput, 0600)
				Expect(initialErr).NotTo(HaveOccurred(), string(initialOutput))
				args := []string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-File", os.Getenv("NXB21D_FINAL_LOCAL12_RAM_SCRIPT"), "-Stage", "MODEL_LOADED_RAM_SETTLED", "-CompletedCalls", fmt.Sprint(calls), "-GovernedRamScript", os.Getenv("NXB21D_FINAL_LOCAL12_GOVERNED_RAM_SCRIPT")}
				output, commandErr := exec.Command("powershell", args...).CombinedOutput()
				ramSettled = string(output)
				_ = os.WriteFile(filepath.Join(dir, "ram-model-loaded-settled.txt"), output, 0600)
				if commandErr != nil {
					finishReason := ""
					if choices, ok := metrics["choices"].([]any); ok && len(choices) > 0 {
						if choice, ok := choices[0].(map[string]any); ok {
							finishReason, _ = choice["finish_reason"].(string)
						}
					}
					results = append(results, map[string]any{
						"id": item.ID, "language": item.Language, "passed": false, "state": "MODEL_LOADED_RAM_GATE_FAILED", "error": string(output),
						"model_called": true, "request_sha256": requestHash, "response_sha256": responseHash, "latency_ms": latency,
						"http_success": httpSuccess, "strict_utf8": strictUTF8, "finish_reason": finishReason, "completion": metrics,
						"fact_packet": facts, "candidate_tuple_set": tuples, "final_plan": out.DynamicProposal, "three_layer_audit": out.HybridAudit,
						"resource_samples": map[string]string{"precall": ramPrecheck, "model_loaded_initial": ramInitial, "model_loaded_settled": ramSettled},
					})
					Fail("MODEL_LOADED_RAM_SETTLED gate failed after first dispatch: " + string(output))
				}
			}
			if modelCalled {
				args := []string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-File", os.Getenv("NXB21D_FINAL_LOCAL12_RAM_SCRIPT"), "-Stage", "POSTCALL_RAM", "-CompletedCalls", fmt.Sprint(calls), "-GovernedRamScript", os.Getenv("NXB21D_FINAL_LOCAL12_GOVERNED_RAM_SCRIPT")}
				output, commandErr := exec.Command("powershell", args...).CombinedOutput()
				ramPostcall = string(output)
				_ = os.WriteFile(filepath.Join(dir, "ram-postcall.txt"), output, 0600)
				Expect(commandErr).NotTo(HaveOccurred(), string(output))
			}
			actual := nxHybridProjection(out, status)
			mismatches := nxHybridCompare(item.Expected, actual)
			finishReason := ""
			if choices, ok := metrics["choices"].([]any); ok && len(choices) > 0 {
				if choice, ok := choices[0].(map[string]any); ok {
					finishReason, _ = choice["finish_reason"].(string)
				}
			}
			if !modelCalled || modelCalled != item.ModelCall {
				mismatches = append(mismatches, "model-call contract mismatch")
			}
			if !transportOK {
				mismatches = append(mismatches, "transport failed")
			}
			if finishReason != "stop" {
				mismatches = append(mismatches, "finish reason is not stop")
			}
			if out.HybridAudit == nil {
				mismatches = append(mismatches, "audit missing")
			}
			if out.HybridAudit != nil {
				factsAfter, _ := json.Marshal(out.HybridAudit.Facts)
				candidatesAfter, _ := json.Marshal(out.HybridAudit.Candidates)
				if !bytes.Equal(factsBefore, factsAfter) || !bytes.Equal(candidatesBefore, candidatesAfter) {
					mismatches = append(mismatches, "deterministic facts or tuple list mutated")
				}
				if out.HybridAudit.ModelDecision == nil {
					mismatches = append(mismatches, "decoded decision missing")
				}
			}
			pass := len(mismatches) == 0
			if pass {
				passed++
			}
			rawCorrect := false
			if modelCalled && out.HybridAudit != nil && out.HybridAudit.ModelDecision != nil {
				decision := out.HybridAudit.ModelDecision
				if item.Expected["state"] != nil {
					rawCorrect = decision.AmbiguityState == "NEEDS_CLARIFICATION" && decision.ClarificationCode == item.Expected["state"]
				} else {
					want := fmt.Sprintf("%s/%s/%s", item.Expected["capability"], item.Expected["semantic"], item.Expected["scope"])
					rawCorrect = decision.AmbiguityState == "RESOLVED" && string(decision.SelectedTupleID) == want
				}
				if rawCorrect {
					modelCorrect++
				}
			}
			write(filepath.Join(dir, "three-layer-audit.json"), out.HybridAudit)
			write(filepath.Join(dir, "decision-validation.json"), map[string]any{"raw_decision_correct": rawCorrect, "model_called": modelCalled, "transport_pass": transportOK, "status": status})
			schemaValid := out.HybridAudit != nil && out.HybridAudit.ModelDecision != nil && !strings.HasPrefix(status, "malformed")
			factOverride := false
			if out.HybridAudit != nil {
				factsAfter, _ := json.Marshal(out.HybridAudit.Facts)
				candidatesAfter, _ := json.Marshal(out.HybridAudit.Candidates)
				factOverride = !bytes.Equal(factsBefore, factsAfter) || !bytes.Equal(candidatesBefore, candidatesAfter)
			}
			result := map[string]any{
				"id": item.ID, "language": item.Language, "passed": pass, "status": status,
				"expected": item.Expected, "actual": actual, "mismatches": mismatches,
				"model_called": modelCalled, "transport_pass": transportOK,
				"request_sha256": requestHash, "response_sha256": responseHash,
				"latency_ms": latency, "retry_count": 0, "completion": metrics,
				"http_success": httpSuccess, "strict_utf8": strictUTF8, "schema_valid": schemaValid,
				"finish_reason": finishReason, "finish_reason_stop": finishReason == "stop",
				"malformed": strings.HasPrefix(status, "malformed"), "unknown_decision": status == "unknown_decision",
				"truncation":    finishReason == "length" || status == "incomplete_completion",
				"fact_override": factOverride, "scope_override": actual["scope"] != nil && actual["scope"] != item.Input.Scope,
				"authorization_override": false, "fact_packet": facts, "candidate_tuple_set": tuples,
				"resource_samples": map[string]string{"precall": ramPrecheck, "model_loaded_initial": ramInitial, "model_loaded_settled": ramSettled, "postcall": ramPostcall},
				"final_plan": func() any {
					if out.HybridAudit != nil {
						return out.HybridAudit.FinalPlan
					}
					return nil
				}(),
				"three_layer_audit": out.HybridAudit,
			}
			write(filepath.Join(dir, "case-result.json"), result)
			results = append(results, result)
			sealIntermediate()
			fmt.Printf("NXB21D_FINAL_LOCAL12_RESULT id=%s pass=%t state=%s model_called=%t latency_ms=%d\n", item.ID, pass, status, modelCalled, latency)
			if !transportOK || strings.HasPrefix(status, "malformed") || status == "incomplete_completion" {
				Fail("fresh selection12 stopped after a transport, schema, or completion failure")
			}
		}
	})
})

var _ = Describe("NXB21D final controlled local Qwen3 8B selection12 corpus source admission", func() {
	It("checks frozen oracles and dispatch expectations without HTTP or inference", func() {
		path := os.Getenv("NXB21D_FINAL_LOCAL12_CORPUS_CHECK")
		if path == "" {
			Skip("optional read-only corpus admission")
		}
		raw, err := os.ReadFile(path)
		Expect(err).NotTo(HaveOccurred())
		var corpus struct {
			Cases []nxHybridCase `json:"cases"`
		}
		Expect(json.Unmarshal(raw, &corpus)).To(Succeed())
		Expect(corpus.Cases).To(HaveLen(12))
		failures := []string{}
		bindings := map[string]any{}
		for _, item := range corpus.Cases {
			req := hybridQueryRequest{Query: item.Query, TenantID: "t", UserID: "u", CollectionID: "c", QueryScope: queryEvidenceScope{Kind: item.Input.Scope, SourceFamily: item.Input.Family}}
			if item.Input.Scope == "selected_evidence" {
				req.EvidenceID = "10000000-0000-4000-8000-000000000001"
			}
			if item.Input.PriorCapability != "" {
				ref, ok := dynamicCapabilityReference(item.Input.PriorCapability)
				Expect(ok).To(BeTrue())
				req.ConversationContext = queryConversationContext{ContractVersion: followUpContextContractV1, TenantID: "t", UserID: "u", CollectionID: "c", Template: queryTemplateNameByOperationID(ref.OperationRef), Target: item.Input.PriorTarget}
			}
			req, facts := extractHybridFacts(req)
			tuples := buildHybridTuples(req, facts)
			binding := map[string]any{
				"facts":                        facts,
				"tuples":                       tuples,
				"schema":                       hybridResidualSchema(tuples),
				"expected_final_typed_outcome": item.ExpectedFinalTypedOutcome,
				"admission": map[string]any{
					"model_call_required":         true,
					"fact_packet_valid":           "PASS",
					"candidate_set_valid":         "PASS",
					"expected_decision_reachable": "PASS",
					"authorization_valid":         "PASS",
					"case_scope_valid":            "PASS",
					"final_plan_valid":            "PASS",
				},
			}
			bindings[item.ID] = binding
			if facts.State != "" || len(tuples) < 2 {
				failures = append(failures, item.ID+": not an eligible residual call")
			}
			if (len(tuples) > 1) != item.ModelCall {
				failures = append(failures, item.ID+": model-call mismatch")
			}
			if item.Expected["state"] != nil {
				encoded, _ := json.Marshal(map[string]string{"decision": "CLARIFY:" + item.Expected["state"].(string)})
				decision, decodeErr := decodeHybridResidual(encoded, tuples)
				if decodeErr != nil || decision.ClarificationCode != item.Expected["state"] {
					failures = append(failures, item.ID+": unreachable clarification")
				}
				continue
			}
			found := false
			for _, tuple := range tuples {
				if tuple.Capability != item.Expected["capability"] || tuple.Semantic != item.Expected["semantic"] {
					continue
				}
				found = true
				out, assembleErr := assembleHybridPlan(req, facts, tuple)
				if assembleErr != nil {
					failures = append(failures, item.ID+": "+assembleErr.Error())
					continue
				}
				out.HybridAudit = &HybridPlannerAuditV1{Facts: facts, FinalPlan: out.DynamicProposal}
				for _, mismatch := range nxHybridCompare(item.Expected, nxHybridProjection(out, "")) {
					failures = append(failures, item.ID+": "+mismatch)
				}
			}
			if !found {
				failures = append(failures, fmt.Sprintf("%s expected tuple absent; facts=%+v", item.ID, facts))
			}
		}
		Expect(failures).To(BeEmpty())
		if output := os.Getenv("NXB21D_FINAL_LOCAL12_BINDINGS_OUT"); output != "" {
			blob, marshalErr := json.MarshalIndent(bindings, "", "  ")
			Expect(marshalErr).NotTo(HaveOccurred())
			Expect(os.WriteFile(output, append(blob, '\n'), 0600)).To(Succeed())
		}
	})
})
