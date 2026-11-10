package caselayer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	. "github.com/onsi/gomega"
)

// A scripted engine: it answers from a function, optionally reports phases and takes a while, and remembers what it was asked.
type scriptedEngine struct {
	mu      sync.Mutex
	calls   []EngineRequest
	respond func(EngineRequest) (int, []byte)
	phases  []string
	delay   time.Duration
}

func (e *scriptedEngine) Ask(ctx context.Context, req EngineRequest) (EngineResult, error) {
	e.mu.Lock()
	e.calls = append(e.calls, req)
	respond, phases, delay := e.respond, e.phases, e.delay
	e.mu.Unlock()
	for _, p := range phases {
		EmitPhase(ctx, p, "phase "+p)
	}
	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return EngineResult{}, ctx.Err()
		}
	}
	status, body := respond(req)
	frame, _ := json.Marshal(map[string]any{"question_seen": req.Question, "numbers": []string{"923001234567"}})
	return EngineResult{Response: body, Status: status, RequestID: "req-" + req.TurnID[:8], Frame: frame, EngineVersion: "test-1"}, nil
}

func (e *scriptedEngine) configure(respond func(EngineRequest) (int, []byte), phases []string, delay time.Duration) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.respond, e.phases, e.delay = respond, phases, delay
}

func (e *scriptedEngine) callCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.calls)
}

func (e *scriptedEngine) lastCall() EngineRequest {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.calls[len(e.calls)-1]
}

const evidenceA = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"

// answerBody builds an engine response the way the Ask engine does: a sentence, a result table of numbers and calls, one labelled figure, the query, and the evidence file it cites.
func answerBody(headline string, rows [][2]any, evidenceIDs ...string) []byte {
	grid := []map[string]any{}
	for _, r := range rows {
		grid = append(grid, map[string]any{"label": r[0], "value": r[1]})
	}
	files := []map[string]any{}
	for _, id := range evidenceIDs {
		files = append(files, map[string]any{"evidence_id": id, "source_file": "calls.csv", "rows": 3})
	}
	raw, err := json.Marshal(map[string]any{
		"collection_id": "x", "policy": "ask_engine_v2", "intent": "records", "route": []string{"ask_engine_v2"},
		"planner": map[string]any{"ask_engine_v2": map[string]any{"state": "answered", "model_ms": 10, "exec_ms": 5, "total_ms": 20, "attempts": 1, "model": "test-model", "prompt_id": "p1"}},
		"answer":  map[string]any{"answer": headline, "result_state": "COMPLETED"},
		"enterprise": map[string]any{
			"result_shape": "ranked", "row_count": len(rows),
			"data_grid":  map[string]any{"columns": []map[string]string{{"header": "Number", "key": "label"}, {"header": "Calls", "key": "value"}}, "rows": grid},
			"derivation": map[string]any{"sql": "SELECT src AS label, count(*) AS value FROM v_comm_events GROUP BY 1 ORDER BY 2 DESC", "views": []string{"v_comm_events"}, "model": "test-model", "prompt_id": "p1"},
			"metrics":    []map[string]string{{"label": "Number of records", "value": "5,005", "source": "ask_engine_v2"}},
			"evidence":   map[string]any{"files": files},
		},
	})
	Expect(err).NotTo(HaveOccurred())
	return raw
}

func abstainBody() []byte {
	raw, err := json.Marshal(map[string]any{
		"policy": "ask_engine_v2", "intent": "clarification",
		"planner":    map[string]any{"ask_engine_v2": map[string]any{"state": "abstained"}},
		"answer":     map[string]any{"clarification": "I did not give an answer, because I could not show that the query asks everything you asked.", "result_state": "CLARIFICATION_REQUIRED"},
		"enterprise": map[string]any{"row_count": 0, "data_grid": map[string]any{"rows": []any{}}},
	})
	Expect(err).NotTo(HaveOccurred())
	return raw
}

var ranked1 = func(req EngineRequest) (int, []byte) {
	return http.StatusOK, answerBody("3 numbers made calls: 923000000201 (14), 923000000202 (9), 923000000203 (4).",
		[][2]any{{"923000000201", 14}, {"923000000202", 9}, {"923000000203", 4}}, evidenceA)
}

var ranked2 = func(req EngineRequest) (int, []byte) {
	return http.StatusOK, answerBody("4 numbers made calls: 923000000201 (14), 923000000202 (10), 923000000204 (2), 923000000203 (4).",
		[][2]any{{"923000000201", 14}, {"923000000202", 10}, {"923000000203", 4}, {"923000000204", 2}}, evidenceA)
}

// ---- HTTP -----------------------------------------------------------------------------------------------------------------------------------------------------------------------

type caller struct{ subject, role, bound string }

var (
	alice = caller{subject: "alice", role: "user"}
	bob   = caller{subject: "bob", role: "user"}
	carol = caller{subject: "carol", role: "user"}
	dave  = caller{subject: "dave", role: "user"}
	erin  = caller{subject: "erin", role: "user"}
	root  = caller{subject: "root", role: "admin"}
)

var httpClient = &http.Client{Timeout: 90 * time.Second}

func (c caller) do(base, tenant, method, path string, body any, accept string) (*http.Response, []byte) {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		Expect(err).NotTo(HaveOccurred())
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, base+path, reader)
	Expect(err).NotTo(HaveOccurred())
	req.Header.Set("X-Test-Tenant", tenant)
	req.Header.Set("X-Test-Subject", c.subject)
	req.Header.Set("X-Test-Role", c.role)
	if c.bound != "" {
		req.Header.Set("X-Test-Bound", c.bound)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	resp, err := httpClient.Do(req)
	Expect(err).NotTo(HaveOccurred())
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	Expect(err).NotTo(HaveOccurred())
	return resp, raw
}

// jsonMap decodes a response body into a map (numbers as float64).
func jsonMap(raw []byte) map[string]any {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return map[string]any{"_raw": string(raw)}
	}
	return m
}

// jget walks a decoded JSON value by keys (and list positions as numbers in text form).
func jget(v any, path ...string) any {
	for _, p := range path {
		switch x := v.(type) {
		case map[string]any:
			v = x[p]
		case []any:
			var i int
			if _, err := fmt.Sscanf(p, "%d", &i); err != nil || i < 0 || i >= len(x) {
				return nil
			}
			v = x[i]
		default:
			return nil
		}
	}
	return v
}

func jstr(v any, path ...string) string {
	if s, ok := jget(v, path...).(string); ok {
		return s
	}
	return ""
}

func jlen(v any, path ...string) int {
	if l, ok := jget(v, path...).([]any); ok {
		return len(l)
	}
	return -1
}

// ndjsonEvents parses a stream body into its events.
func ndjsonEvents(raw []byte) []map[string]any {
	var out []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var m map[string]any
		Expect(json.Unmarshal([]byte(line), &m)).To(Succeed(), line)
		out = append(out, m)
	}
	return out
}
