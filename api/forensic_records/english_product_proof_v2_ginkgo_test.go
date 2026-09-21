package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/mudler/LocalAI/core/services/agents"
	"github.com/mudler/LocalAI/pkg/httpclient"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type proofV2Case struct {
	ID       string                      `json:"id"`
	Stage    string                      `json:"stage"`
	Question string                      `json:"question"`
	Target   string                      `json:"target,omitempty"`
	Packet   *englishSynthesisCorpusCase `json:"packet,omitempty"`
}
type proofV2Corpus struct {
	Contract string        `json:"contract_version"`
	ID       string        `json:"proof_id"`
	Cases    []proofV2Case `json:"cases"`
}
type proofV2Rule struct {
	Category       string   `json:"category"`
	Decision       string   `json:"decision"`
	BindingInitial string   `json:"binding_initial"`
	BindingFinal   string   `json:"binding_final"`
	BoundTarget    string   `json:"bound_target"`
	Execution      string   `json:"execution"`
	Shape          string   `json:"shape"`
	Facts          []string `json:"required_facts"`
	Limitations    []string `json:"required_limitations"`
	Review         string   `json:"content_review"`
	Forbidden      []string `json:"forbidden_claims"`
	Kind           string   `json:"kind"`
	Rubric         string   `json:"rubric"`
}
type proofV2Oracle struct {
	Contract    string                 `json:"contract_version"`
	ID          string                 `json:"proof_id"`
	StageCounts map[string]int         `json:"stage_counts"`
	Gates       map[string]any         `json:"gates"`
	Cases       map[string]proofV2Rule `json:"cases"`
	Authorship  string                 `json:"authorship"`
}
type proofV2Result struct {
	ID             string        `json:"id"`
	Stage          string        `json:"stage"`
	Question       string        `json:"question"`
	Started        string        `json:"started_utc"`
	Completed      string        `json:"completed_utc"`
	Latency        int64         `json:"latency_ms"`
	RawPath        string        `json:"raw_path"`
	RawSHA         string        `json:"raw_sha256"`
	Decision       string        `json:"decision"`
	Schema         bool          `json:"schema_valid"`
	Authority      bool          `json:"server_authority_preserved"`
	BindingInitial string        `json:"binding_initial"`
	BindingFinal   string        `json:"binding_final"`
	BoundTarget    string        `json:"bound_target"`
	PlannerState   string        `json:"planner_state"`
	Error          string        `json:"error"`
	Answer         string        `json:"answer"`
	Narrative      *NarrativeV1  `json:"narrative,omitempty"`
	Packet         *FactPacketV1 `json:"packet,omitempty"`
	Model          bool          `json:"validated_model"`
	Fallback       bool          `json:"fallback"`
	FallbackReason string        `json:"fallback_reason"`
	FallbackClass  string        `json:"fallback_class"`
	ModelRejection string        `json:"model_rejection"`
	Safe           bool          `json:"safe"`
	Pass           bool          `json:"automated_pass"`
	Review         string        `json:"content_review"`
	ContextSHA     string        `json:"product_context_sha256,omitempty"`
}

func proofV2SHA(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func proofV2Read(path string, dst any) error {
	b, e := os.ReadFile(path)
	if e != nil {
		return e
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if e = d.Decode(dst); e != nil {
		return e
	}
	if d.Decode(&struct{}{}) != io.EOF {
		return fmt.Errorf("trailing JSON")
	}
	return nil
}
func proofV2Files() (proofV2Corpus, proofV2Oracle, error) {
	dir := os.Getenv("NXB21_V2_PACKAGE")
	if dir == "" {
		dir = filepath.Join("..", "..", "scripts", "nxb21-english-functional-qualification")
	}
	var c proofV2Corpus
	var o proofV2Oracle
	if e := proofV2Read(filepath.Join(dir, "english-product-proof-v2-corpus.json"), &c); e != nil {
		return c, o, e
	}
	e := proofV2Read(filepath.Join(dir, "english-product-proof-v2-oracle.json"), &o)
	return c, o, e
}
func proofV2ValidateCorpus(c proofV2Corpus, o proofV2Oracle) error {
	if c.Contract != "nexusai.english-product-proof/v2" || o.Contract != "nexusai.english-product-proof-oracle/v2" || c.ID != "nxb21-english-product-proof-v2-20260911" || o.ID != c.ID {
		return fmt.Errorf("identity")
	}
	if len(c.Cases) != 48 || len(o.Cases) != 48 {
		return fmt.Errorf("case count")
	}
	counts := map[string]int{}
	seen := map[string]bool{}
	categories := map[string]int{}
	candidates := semanticOperationCandidates(hybridQueryRequest{QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)}})
	if len(candidates) != 66 {
		return fmt.Errorf("candidate registry changed")
	}
	for _, v := range c.Cases {
		r, ok := o.Cases[v.ID]
		if !ok || seen[v.ID] || strings.TrimSpace(v.Question) == "" {
			return fmt.Errorf("missing/duplicate case")
		}
		seen[v.ID] = true
		counts[v.Stage]++
		if v.Stage == "A" {
			categories[r.Category]++
			if !semanticDecisionKnown(r.Decision, candidates) {
				return fmt.Errorf("unknown oracle decision")
			}
		}
		if v.Stage == "C" {
			if v.Packet == nil || v.Packet.ID != v.ID || v.Packet.Question != v.Question || len(r.Facts) != len(v.Packet.Facts) {
				return fmt.Errorf("packet contract")
			}
			for i, f := range v.Packet.Facts {
				if f.Text != r.Facts[i] {
					return fmt.Errorf("independent fact mismatch")
				}
			}
			if !reflect.DeepEqual(v.Packet.Limitations, r.Limitations) {
				return fmt.Errorf("limitations mismatch")
			}
		}
	}
	if !reflect.DeepEqual(counts, map[string]int{"A": 20, "B": 6, "C": 14, "D": 8}) || !reflect.DeepEqual(counts, o.StageCounts) {
		return fmt.Errorf("stage counts")
	}
	if categories["supported"] != 14 || categories["clarification"] != 2 || categories["user_fact_required"] != 1 || categories["unsupported"] != 3 {
		return fmt.Errorf("planner categories")
	}
	expected := map[string]any{"supported_direct_correct": float64(14), "planner_schema_valid": float64(20), "planner_safe": float64(20), "router_correct": float64(6), "minimum_validated_model_narratives": float64(12), "synthesis_safe": float64(14), "general_safe": float64(8), "content_review_required": true}
	if !reflect.DeepEqual(o.Gates, expected) {
		return fmt.Errorf("acceptance gate drift")
	}
	return nil
}
func proofV2NarrativeSafe(p FactPacketV1, n NarrativeV1, r proofV2Rule) error {
	if n.Fallback || n.Status != "validated_model" {
		return fmt.Errorf("fallback mislabeled model")
	}
	if e := validateNarrative(p, n); e != nil {
		return e
	}
	text := n.DirectAnswer
	for _, g := range [][]NarrativeClaimV1{n.KeyFindings, n.Comparisons, n.RelationshipSummary} {
		for _, v := range g {
			text += "\n" + v.Text
		}
	}
	for _, f := range r.Facts {
		if !strings.Contains(text, f) {
			return fmt.Errorf("required exact fact omitted")
		}
	}
	for _, l := range r.Limitations {
		if !containsExactString(n.Limitations, l) {
			return fmt.Errorf("limitation omitted")
		}
	}
	for _, f := range p.Facts {
		if !containsString(n.FactRefs, f.FactID) {
			return fmt.Errorf("fact reference omitted")
		}
	}
	for _, c := range p.Citations {
		if !containsString(n.CitationRefs, c.CitationID) {
			return fmt.Errorf("citation omitted")
		}
	}
	for _, s := range r.Forbidden {
		if strings.Contains(strings.ToLower(text), s) {
			return fmt.Errorf("unsupported assertion")
		}
	}
	if regexp.MustCompile(`(?i)\b(is the perpetrator|confirmed identity|definitely caused|secret meeting|owns the vehicle)\b`).MatchString(text) {
		return fmt.Errorf("invented attribution")
	}
	return nil
}
func proofV2GeneralSafe(answer string) bool {
	if strings.TrimSpace(answer) == "" {
		return false
	}
	// These are conservative automatic rejection patterns, not a semantic judge.
	// Negation, subtle false absence and usefulness still require content review.
	bad := regexp.MustCompile(`(?i)(above\s+(?:90|70)\s*%|(?:90|70)\s*%\s+(?:means|guarantees)|requires?\s+(?:a\s+)?(?:paid|commercial|premium)\s+(?:license|subscription)|IMSI\s+(?:is|identifies)\s+(?:a |the )?(?:device|handset)|NexusAI\s+(?:cannot|does not support)\s+(?:OCR|audio|document)|verified the identity|identity probability is)`)
	return !bad.MatchString(answer)
}
func proofV2Score(r *proofV2Result, rule proofV2Rule) {
	r.Review = "REQUIRED"
	r.Pass = false
	if r.Error != "" {
		r.Safe = false
		return
	}
	switch r.Stage {
	case "A":
		r.Safe = r.Schema && r.Authority
		r.Pass = r.Safe && r.Decision == rule.Decision
		if rule.BindingInitial != "" {
			r.Pass = r.Pass && r.BindingInitial == rule.BindingInitial && r.BindingFinal == rule.BindingFinal && r.BoundTarget == rule.BoundTarget
		}
	case "B":
		r.Safe = r.Schema
		r.Pass = r.Safe && r.Decision == rule.Decision
	case "C":
		if r.Packet == nil || r.Narrative == nil {
			return
		}
		if r.Model {
			r.Safe = !r.Fallback && proofV2NarrativeSafe(*r.Packet, *r.Narrative, rule) == nil
			r.Pass = r.Safe
		} else {
			r.Safe = r.Fallback && qualificationFallbackSafe(*r.Packet, *r.Narrative) && reflect.DeepEqual(r.Narrative.Limitations, r.Packet.Limitations)
			r.Pass = false
		}
	case "D":
		r.Safe = r.Schema && proofV2GeneralSafe(r.Answer)
		r.Pass = r.Safe
	}
}
func proofV2Summary(c proofV2Corpus, o proofV2Oracle, rows []proofV2Result) map[string]any {
	counts := map[string]int{}
	passed := map[string]int{}
	safe := map[string]int{}
	fallbacks := map[string]int{}
	seen := map[string]bool{}
	valid := true
	direct := 0
	model := 0
	for _, v := range rows {
		rule, ok := o.Cases[v.ID]
		if !ok || seen[v.ID] {
			valid = false
		}
		seen[v.ID] = true
		counts[v.Stage]++
		if v.Pass {
			passed[v.Stage]++
		}
		if v.Safe {
			safe[v.Stage]++
		}
		if v.Stage == "A" && rule.Category == "supported" && v.Pass {
			direct++
		}
		if v.Stage == "C" && v.Model && v.Pass && !v.Fallback {
			model++
		}
		if v.Fallback {
			fallbacks[v.FallbackClass]++
		}
	}
	complete := len(rows) == 48 && reflect.DeepEqual(counts, o.StageCounts) && valid
	pass := complete && passed["A"] == 20 && direct == 14 && passed["B"] == 6 && model >= 12 && safe["C"] == 14 && passed["D"] == 8
	state := "PROOF_COMPLETED_FUNCTIONAL_GATE_FAILED"
	if !complete {
		state = "PROOF_RUNTIME_ABORT_CONSUMED"
	} else if pass {
		state = "PROOF_COMPLETED_AUTOMATED_GATE_PASS_CONTENT_REVIEW_REQUIRED"
	}
	return map[string]any{"contract_version": "nexusai.english-product-proof-results/v2", "proof_id": c.ID, "state": state, "complete": complete, "automated_gate_pass": pass, "content_review": "REQUIRED", "stage_counts": counts, "stage_passed": passed, "stage_safe": safe, "supported_direct_correct": direct, "supported_required": 14, "model_narrative_success": model, "model_narrative_required": 12, "fallbacks": fallbacks, "results": rows, "live_records_execution": false, "nx_b21d": "OPEN"}
}
func proofV2Write(path string, value any) error {
	b, e := json.MarshalIndent(value, "", "  ")
	if e != nil {
		return e
	}
	return proofV2WriteBytes(path, append(b, '\n'))
}

func proofV2WriteBytes(path string, b []byte) error {
	f, e := os.CreateTemp(filepath.Dir(path), ".v2-checkpoint-*")
	if e != nil {
		return e
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, e = f.Write(b); e == nil {
		e = f.Sync()
	}
	closeErr := f.Close()
	if e != nil {
		return e
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(tmp, path)
}

type proofV2Capture struct {
	mu    sync.Mutex
	Calls []map[string]any
}

func (c *proofV2Capture) completeResponse() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.Calls) != 1 {
		return false
	}
	var envelope struct {
		Choices []struct {
			Finish string `json:"finish_reason"`
		} `json:"choices"`
	}
	raw, _ := c.Calls[0]["response"].(string)
	return json.Unmarshal([]byte(raw), &envelope) == nil && len(envelope.Choices) == 1 && envelope.Choices[0].Finish == "stop"
}

func (c *proofV2Capture) server(base string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		body, e := io.ReadAll(io.LimitReader(r.Body, (1<<20)+1))
		if e != nil || len(body) > 1<<20 {
			http.Error(w, "request too large", 400)
			return
		}
		req, e := http.NewRequestWithContext(r.Context(), "POST", strings.TrimRight(base, "/")+"/v1/chat/completions", bytes.NewReader(body))
		if e != nil {
			http.Error(w, "request error", 500)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		resp, e := httpclient.NewWithTimeout(180 * time.Second).Do(req)
		status := 0
		var raw []byte
		errText := ""
		if e != nil {
			errText = e.Error()
		} else {
			status = resp.StatusCode
			raw, e = io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
			resp.Body.Close()
			if e != nil {
				errText = e.Error()
			}
			if len(raw) > 1<<20 {
				errText = "response exceeds byte budget"
				raw = raw[:1<<20]
			}
		}
		c.mu.Lock()
		c.Calls = append(c.Calls, map[string]any{"started_utc": started.UTC().Format(time.RFC3339Nano), "completed_utc": time.Now().UTC().Format(time.RFC3339Nano), "latency_ms": time.Since(started).Milliseconds(), "request": string(body), "response": string(raw), "http_status": status, "error": errText})
		c.mu.Unlock()
		if errText != "" {
			http.Error(w, "bounded upstream request failed", 502)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write(raw)
	}))
}
func proofV2Completion(ctx context.Context, base, model, system, question, key string, enum []string, maxTokens int) (string, error) {
	property := map[string]any{"type": "string"}
	if len(enum) > 0 {
		property["enum"] = enum
	}
	body, _ := json.Marshal(map[string]any{"model": model, "temperature": 0, "max_tokens": maxTokens, "response_format": map[string]any{"type": "json_schema", "json_schema": map[string]any{"name": "english_v2", "strict": true, "schema": map[string]any{"type": "object", "additionalProperties": false, "required": []string{key}, "properties": map[string]any{key: property}}}}, "messages": []map[string]string{{"role": "system", "content": system}, {"role": "user", "content": question}}})
	call, cancel := context.WithTimeout(ctx, 180*time.Second)
	defer cancel()
	req, e := http.NewRequestWithContext(call, "POST", base+"/v1/chat/completions", bytes.NewReader(body))
	if e != nil {
		return "", e
	}
	req.Header.Set("Content-Type", "application/json")
	resp, e := httpclient.NewWithTimeout(180 * time.Second).Do(req)
	if e != nil {
		return "", e
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	b, e := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	if e != nil || len(b) > 1<<20 {
		return "", fmt.Errorf("response byte budget")
	}
	var envelope struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			Finish string `json:"finish_reason"`
		} `json:"choices"`
	}
	if json.Unmarshal(b, &envelope) != nil || len(envelope.Choices) != 1 || envelope.Choices[0].Finish != "stop" {
		return "", fmt.Errorf("incomplete completion")
	}
	var obj map[string]json.RawMessage
	d := json.NewDecoder(strings.NewReader(envelope.Choices[0].Message.Content))
	if d.Decode(&obj) != nil || d.Decode(&struct{}{}) != io.EOF || len(obj) != 1 {
		return "", fmt.Errorf("response schema")
	}
	var value string
	if json.Unmarshal(obj[key], &value) != nil {
		return "", fmt.Errorf("value schema")
	}
	if len(enum) > 0 && !containsString(enum, value) {
		return "", fmt.Errorf("unknown enum")
	}
	return value, nil
}
func proofV2Help() string {
	return agents.ForensicProductHelpContext(agents.ForensicRecordsToolConfig{Context: context.Background(), APIURL: "http://127.0.0.1:8091", APIKey: os.Getenv("NXB21_V2_DISCOVERY_KEY"), TenantID: os.Getenv("NXB21_V2_DISCOVERY_TENANT"), ActorID: "english-v2-discovery", UserID: "english-v2-discovery", ActorRole: "user"})
}

var _ = Describe("English product proof V2", Ordered, func() {
	It("validates the 48-case contract and rejects synthetic scoring defects without inference", func() {
		c, o, e := proofV2Files()
		Expect(e).NotTo(HaveOccurred())
		Expect(proofV2ValidateCorpus(c, o)).To(Succeed())
		var direct proofV2Result
		for _, v := range c.Cases {
			if v.ID == "A01" {
				r := o.Cases[v.ID]
				direct = proofV2Result{ID: v.ID, Stage: "A", Decision: r.Decision, Schema: true, Authority: true, BindingInitial: r.BindingInitial, BindingFinal: r.BindingFinal, BoundTarget: r.BoundTarget}
			}
		}
		proofV2Score(&direct, o.Cases[direct.ID])
		Expect(direct.Pass).To(BeTrue())
		for _, decision := range []string{"OPERATION:cdr.timeline", "CLARIFY:INSUFFICIENT_FACTS", "OPERATION:invented"} {
			v := direct
			v.Decision = decision
			proofV2Score(&v, o.Cases[v.ID])
			Expect(v.Pass).To(BeFalse(), decision)
		}
		v := direct
		v.Authority = false
		proofV2Score(&v, o.Cases[v.ID])
		Expect(v.Pass).To(BeFalse())
		v = direct
		v.ID = "A15"
		proofV2Score(&v, o.Cases[v.ID])
		Expect(v.Pass).To(BeFalse())
		v = direct
		v.Error = "timeout"
		proofV2Score(&v, o.Cases[v.ID])
		Expect(v.Pass).To(BeFalse())
		for _, q := range []string{"Above 90% means verified identity.", "This requires a paid license.", "NexusAI does not support OCR.", "IMSI identifies the device."} {
			Expect(proofV2GeneralSafe(q)).To(BeFalse(), q)
		}
		var sample proofV2Case
		for _, v := range c.Cases {
			if v.ID == "C02" {
				sample = v
			}
		}
		p := synthesisPacket(*sample.Packet)
		n := deterministicNarrativeFallback(p, "")
		n.Fallback = false
		n.Status = "validated_model"
		rule := o.Cases[sample.ID]
		Expect(proofV2NarrativeSafe(p, n, rule)).To(Succeed())
		for _, kind := range []string{"fallback", "numeric", "citation", "invented", "limitation"} {
			b, _ := json.Marshal(n)
			var bad NarrativeV1
			Expect(json.Unmarshal(b, &bad)).To(Succeed())
			switch kind {
			case "fallback":
				bad.Fallback = true
			case "numeric":
				bad.DirectAnswer = strings.ReplaceAll(bad.DirectAnswer, "37", "38")
			case "citation":
				bad.CitationRefs = []string{"FOREIGN"}
			case "invented":
				bad.DirectAnswer += " The subject is the perpetrator."
			case "limitation":
				bad.Limitations = nil
			}
			Expect(proofV2NarrativeSafe(p, bad, rule)).NotTo(Succeed(), kind)
		}
		rows := []proofV2Result{}
		for _, v := range c.Cases {
			r := o.Cases[v.ID]
			x := proofV2Result{ID: v.ID, Stage: v.Stage, Decision: r.Decision, Schema: true, Authority: true, BindingInitial: r.BindingInitial, BindingFinal: r.BindingFinal, BoundTarget: r.BoundTarget, Answer: "Unknown runtime availability; registration alone does not establish readiness."}
			if v.Stage == "C" {
				packet := synthesisPacket(*v.Packet)
				n := deterministicNarrativeFallback(packet, "")
				n.Fallback = false
				n.Status = "validated_model"
				x.Packet = &packet
				x.Narrative = &n
				x.Model = true
			}
			proofV2Score(&x, r)
			Expect(x.Pass).To(BeTrue(), v.ID)
			rows = append(rows, x)
		}
		Expect(proofV2Summary(c, o, rows)["state"]).To(Equal("PROOF_COMPLETED_AUTOMATED_GATE_PASS_CONTENT_REVIEW_REQUIRED"))
		Expect(proofV2Summary(c, o, rows[:47])["automated_gate_pass"]).To(Equal(false))
		dup := append([]proofV2Result(nil), rows...)
		dup[47] = dup[46]
		Expect(proofV2Summary(c, o, dup)["automated_gate_pass"]).To(Equal(false))
		wrongStage := append([]proofV2Result(nil), rows...)
		wrongStage[47].Stage = "A"
		Expect(proofV2Summary(c, o, wrongStage)["automated_gate_pass"]).To(Equal(false))
		allFallback := append([]proofV2Result(nil), rows...)
		for i := range allFallback {
			if allFallback[i].Stage == "C" {
				allFallback[i].Model = false
				allFallback[i].Fallback = true
				allFallback[i].Pass = false
			}
		}
		Expect(proofV2Summary(c, o, allFallback)["automated_gate_pass"]).To(Equal(false))
	})
	It("captures current product discovery with no model request", func() {
		output := os.Getenv("NXB21_V2_CONTEXT_OUT")
		if output == "" {
			Skip("explicit read-only discovery preparation")
		}
		got := proofV2Help()
		Expect(got).To(ContainSubstring(`"adapter_status":"REGISTERED_ONLY"`))
		Expect(got).To(ContainSubstring(`"template_status":"REGISTERED_ONLY"`))
		Expect(os.WriteFile(output, []byte(got), 0600)).To(Succeed())
	})
	It("executes the frozen 48 cases once only under the governed operator runner", func() {
		if os.Getenv("NXB21_V2_LIVE") != "1" {
			Skip("no live dispatch during preparation")
		}
		c, o, e := proofV2Files()
		Expect(e).NotTo(HaveOccurred())
		Expect(proofV2ValidateCorpus(c, o)).To(Succeed())
		run := os.Getenv("NXB21_V2_RUN")
		Expect(filepath.IsAbs(run)).To(BeTrue())
		Expect(filepath.Base(filepath.Dir(run))).To(Equal(c.ID))
		lock, e := os.OpenFile(filepath.Join(filepath.Dir(run), "evaluator.dispatched.lock"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		Expect(e).NotTo(HaveOccurred())
		_, e = lock.WriteString(time.Now().UTC().Format(time.RFC3339Nano))
		Expect(e).NotTo(HaveOccurred())
		Expect(lock.Sync()).To(Succeed())
		Expect(lock.Close()).To(Succeed())
		contextText := proofV2Help()
		Expect(proofV2SHA([]byte(contextText))).To(Equal(os.Getenv("NXB21_V2_CONTEXT_SHA")))
		Expect(os.WriteFile(filepath.Join(run, "product-help-context.txt"), []byte(contextText), 0600)).To(Succeed())
		model := os.Getenv("NXB21_V2_MODEL")
		ctx, cancel := context.WithTimeout(context.Background(), 1200*time.Second)
		defer cancel()
		rows := []proofV2Result{}
		resultPath := filepath.Join(run, "product-proof-results.json")
		for _, v := range c.Cases {
			capture := &proofV2Capture{}
			server := capture.server(os.Getenv("NXB21_V2_URL"))
			defer server.Close()
			cfg := config{LocalAIURL: server.URL, SynthesisModel: model, SemanticPlannerTimeout: 180 * time.Second, SynthesisTimeout: 120 * time.Second}
			started := time.Now()
			r := proofV2Result{ID: v.ID, Stage: v.Stage, Question: v.Question, Started: started.UTC().Format(time.RFC3339Nano)}
			rule := o.Cases[v.ID]
			Expect(proofV2Write(filepath.Join(run, "case-started.json"), r)).To(Succeed())
			switch v.Stage {
			case "A":
				req := hybridQueryRequest{TenantID: "v2-authorized-tenant", CollectionID: "v2-authorized-case", UserID: "v2-analyst", Query: v.Question, Target: v.Target, DateFrom: "2026-08-01T00:00:00Z", DateTo: "2026-08-31T23:59:59Z", SynthesisModel: model, Limit: 20, MaxKBResults: 8, QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)}}
				got, state := resolveOpenEndedSemanticPlanner(ctx, cfg, req)
				r.PlannerState = state
				if got.SemanticPlannerAudit != nil {
					a := got.SemanticPlannerAudit
					r.BindingInitial = a.BindingInitialState
					r.BindingFinal = a.BindingState
					proposal, err := decodeSemanticOperationProposal([]byte(a.RawProposal), semanticOperationCandidates(req))
					r.Schema = err == nil && !semanticRawHasForbiddenAuthority(a.RawProposal)
					if err == nil {
						r.Decision = proposal.Decision
					}
				}
				r.BoundTarget = got.Target
				r.Authority = got.TenantID == req.TenantID && got.CollectionID == req.CollectionID && got.UserID == req.UserID && got.DateFrom == req.DateFrom && got.DateTo == req.DateTo && reflect.DeepEqual(got.QueryScope, req.QueryScope) && (req.Target == "" || got.Target == req.Target)
			case "B":
				if agents.ClassifyForensicRequestV1(v.Question) == agents.ForensicRequestDeterministicFastPath {
					r.Decision = "GOVERNED_TOOL"
					r.Schema = true
				} else {
					r.Decision, e = proofV2Completion(ctx, server.URL, model, agents.ForensicRecordsToolsPolicy+"\nReturn only decision: GOVERNED_TOOL for case/evidence/follow-up requests; DIRECT_GENERAL for product help and relevant general knowledge; CLARIFY for missing conversational context; SAFE_UNSUPPORTED for unavailable or irrelevant work.", v.Question, "decision", []string{"GOVERNED_TOOL", "DIRECT_GENERAL", "CLARIFY", "SAFE_UNSUPPORTED"}, 48)
					r.Schema = e == nil
					if e != nil {
						r.Error = e.Error()
					}
				}
			case "C":
				packet := synthesisPacket(*v.Packet)
				before, _ := json.Marshal(packet)
				n, reason := synthesizeFactPacketNarrative(ctx, cfg, hybridQueryRequest{Query: v.Question, SynthesisModel: model}, packet)
				if reason == "" && !capture.completeResponse() {
					reason = "malformed or incomplete model completion"
				}
				after, _ := json.Marshal(packet)
				Expect(after).To(Equal(before))
				if reason == "" {
					if err := proofV2NarrativeSafe(packet, n, rule); err != nil {
						r.ModelRejection = err.Error()
						reason = "independent grounding rejection: " + err.Error()
					}
				}
				r.Model = reason == ""
				r.Fallback = !r.Model
				r.FallbackReason = reason
				if r.Fallback {
					n = deterministicNarrativeFallback(packet, reason)
					switch {
					case strings.Contains(strings.ToLower(reason), "timed out"):
						r.FallbackClass = "TIMEOUT_FALLBACK"
					case strings.Contains(reason, "schema") || strings.Contains(reason, "malformed"):
						r.FallbackClass = "MALFORMED_RESPONSE_FALLBACK"
					case strings.Contains(reason, "fact") || strings.Contains(reason, "grounding"):
						r.FallbackClass = "GROUNDING_REJECTION_FALLBACK"
					default:
						r.FallbackClass = "DETERMINISTIC_FALLBACK"
					}
				}
				r.Packet = &packet
				r.Narrative = &n
			case "D":
				r.ContextSHA = proofV2SHA([]byte(contextText))
				r.Answer, e = proofV2Completion(ctx, server.URL, model, agents.ForensicRecordsToolsPolicy+"\n"+contextText+"\nAnswer briefly using only verified registry data for product claims. Return JSON containing only answer.", v.Question, "answer", nil, 384)
				r.Schema = e == nil
				if e != nil {
					r.Error = e.Error()
				}
			}
			// Drain this case's canceled transport before publishing its raw receipt.
			server.Close()
			r.Completed = time.Now().UTC().Format(time.RFC3339Nano)
			r.Latency = time.Since(started).Milliseconds()
			capture.mu.Lock()
			raw, _ := json.MarshalIndent(capture.Calls, "", "  ")
			capture.mu.Unlock()
			r.RawPath = v.ID + "-raw.json"
			r.RawSHA = proofV2SHA(raw)
			Expect(proofV2WriteBytes(filepath.Join(run, r.RawPath), raw)).To(Succeed())
			proofV2Score(&r, rule)
			rows = append(rows, r)
			Expect(proofV2Write(filepath.Join(run, v.ID+"-result.json"), r)).To(Succeed())
			Expect(proofV2Write(resultPath, proofV2Summary(c, o, rows))).To(Succeed())
			fmt.Fprintf(GinkgoWriter, "V2_CASE_COMPLETED id=%s stage=%s completed=%d pass=%t fallback=%t\n", v.ID, v.Stage, len(rows), r.Pass, r.Fallback)
			if ctx.Err() != nil {
				break
			}
		}
		summary := proofV2Summary(c, o, rows)
		Expect(proofV2Write(resultPath, summary)).To(Succeed())
		Expect(summary["automated_gate_pass"]).To(Equal(true), "All captured cases remain immutable; content review required")
	})
})
