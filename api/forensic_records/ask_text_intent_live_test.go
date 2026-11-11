package main

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

// TestTextIntentLive is the Tier 1 measurement of docs/roadmap/text-intent-preregistration-20261110.md: the real model is called with the production request for every question of a bank,
// and the server's validator is applied. It writes one decision per question and asserts nothing about them (evaluation/text_intent/score_tier1.py applies the gates). Skipped unless
// TEXT_INTENT_BANK names the bank file.
//
//	TEXT_INTENT_BANK=evaluation/text_intent/bank-20261110.json TEXT_INTENT_OUT=... TEXT_INTENT_MODEL_URL=http://127.0.0.1:8080 go test -run TestTextIntentLive -count=1 -timeout 3h .
func TestTextIntentLive(t *testing.T) {
	bankPath := os.Getenv("TEXT_INTENT_BANK") //nolint:forbidigo // measurement harness reads its inputs from the environment
	if bankPath == "" {
		t.Skip("TEXT_INTENT_BANK not set") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	raw, err := os.ReadFile(bankPath)
	if err != nil {
		t.Fatal(err) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	var bank struct {
		Positives []struct{ ID, Question string } `json:"positives"`
		Negatives []struct{ ID, Question string } `json:"negatives"`
	}
	if err := json.Unmarshal(raw, &bank); err != nil {
		t.Fatal(err) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	cfg := config{LocalAIURL: os.Getenv("TEXT_INTENT_MODEL_URL"), SynthesisModel: os.Getenv("TEXT_INTENT_MODEL")} //nolint:forbidigo // measurement harness
	if cfg.LocalAIURL == "" {
		cfg.LocalAIURL = "http://127.0.0.1:8080"
	}
	if cfg.SynthesisModel == "" {
		cfg.SynthesisModel = "qwen3-4b-instruct-2507-q4km-nxb21d-dev"
	}
	type decision struct {
		ID       string `json:"id"`
		Question string `json:"question"`
		State    string `json:"state"` // ROUTED | NOT_TEXT_SEARCH | REJECTED | UNAVAILABLE
		About    string `json:"about"`
		Reason   string `json:"reason,omitempty"`
		Kind     string `json:"kind"`
		Family   string `json:"family"`
		Match    string `json:"match"`
		Proposed string `json:"proposed_literal"`
		Media    string `json:"media_words,omitempty"`
		Literal  string `json:"literal,omitempty"`
		MS       int64  `json:"ms"`
	}
	var out []decision
	run := func(id, question string) {
		started := time.Now()
		audit := TextIntentAuditV1{}
		result, failure := textIntentModel(t.Context(), cfg, cfg.SynthesisModel, question, &audit)
		d := decision{ID: id, Question: question, MS: time.Since(started).Milliseconds()}
		if failure != "" {
			d.State, d.Reason = "UNAVAILABLE", failure
		} else {
			d.About, d.Kind, d.Family, d.Match, d.Proposed = result.About, result.Kind, result.Family, result.Match, result.Literal
			d.Media = result.MediaWords
			if result.Kind == tiKindPlate { // Amendment 3: the plate is checked by the same rules the route applies
				key, plateWhy := textIntentPlateKey(question, result)
				if plateWhy != "" {
					d.State, d.Reason = "REJECTED", plateWhy
				} else {
					d.State, d.Literal, d.Family = "ROUTED", key, tiFamilyPlate
				}
				out = append(out, d)
				return
			}
			text, family, why := textIntentValidate(question, result)
			switch {
			case why == "NOT_TEXT_SEARCH":
				d.State = "NOT_TEXT_SEARCH"
			case why != "":
				d.State, d.Reason = "REJECTED", why
			default:
				d.State, d.Literal, d.Family = "ROUTED", text.LiteralText, family
			}
		}
		out = append(out, d)
	}
	for _, q := range bank.Positives {
		run(q.ID, q.Question)
	}
	for _, q := range bank.Negatives {
		run(q.ID, q.Question)
	}
	encoded, _ := json.MarshalIndent(out, "", " ")
	path := os.Getenv("TEXT_INTENT_OUT") //nolint:forbidigo // measurement harness
	if path == "" {
		path = "text-intent-live.json"
	}
	if err := os.WriteFile(path, encoded, 0o644); err != nil {
		t.Fatal(err) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	t.Logf("%d decisions written to %s", len(out), path) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
}
