package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudler/LocalAI/pkg/forensicrequest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// THE EVALUATION OF THE ASK ENGINE AGAINST THE GOVERNED SQL LANE (roadmap 6.8.10, the arms of milestone M2).
//
// Not a unit spec: it puts the questions of tools/ask_eval (built from the planted-truth keys of a GENERATED case) to each arm with the REAL local model and writes one JSON
// line per question and arm, which tools/ask_eval/judge.py scores against the keys. It runs only when tools/case_generator/check_ask_engine.py hands it a throwaway database
// and a questions file (ASK_EVAL_QUESTIONS), and skips otherwise. Nothing here is a gate: the gates are written before the run (docs/data/ask-engine-v2.md) and read from the judge.
var _ = Describe("Ask engine evaluation", Ordered, func() {
	var (
		pool *pgxpool.Pool
		ctx  = context.Background()
	)

	BeforeAll(func() {
		dsn := os.Getenv("ASK_ENGINE_CASE_DSN")                 //nolint:forbidigo // opt-in switch for an evaluation run
		if dsn == "" || os.Getenv("ASK_EVAL_QUESTIONS") == "" { //nolint:forbidigo // set by the harness only
			Skip("requires tools/case_generator/check_ask_engine.py with --questions")
		}
		cfg, err := pgxpool.ParseConfig(dsn)
		Expect(err).NotTo(HaveOccurred())
		Expect(cfg.ConnConfig.Host).To(Equal("127.0.0.1"))
		pool, err = pgxpool.NewWithConfig(ctx, cfg)
		Expect(err).NotTo(HaveOccurred())
		var marker bool
		Expect(pool.QueryRow(ctx, "SELECT to_regclass('public.nx_throwaway_marker') IS NOT NULL").Scan(&marker)).To(Succeed())
		Expect(marker).To(BeTrue(), "this is not a throwaway database")
	})

	AfterAll(func() {
		if pool != nil {
			pool.Close()
		}
	})

	It("puts each question to each arm and records what came back", func() {
		raw, err := os.ReadFile(os.Getenv("ASK_EVAL_QUESTIONS")) //nolint:forbidigo // set by the harness only
		Expect(err).NotTo(HaveOccurred())
		var questions []struct {
			ID       string `json:"id"`
			Question string `json:"question"`
		}
		Expect(json.Unmarshal(raw, &questions)).To(Succeed())
		only := map[string]bool{}
		for _, id := range strings.Split(os.Getenv("ASK_EVAL_IDS"), ",") { //nolint:forbidigo // set by the harness only
			if id = strings.TrimSpace(id); id != "" {
				only[id] = true
			}
		}
		arms := strings.Split(os.Getenv("ASK_EVAL_ARMS"), ",")                                         //nolint:forbidigo // set by the harness only
		out, err := os.OpenFile(os.Getenv("ASK_EVAL_OUT"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600) //nolint:forbidigo // set by the harness only
		Expect(err).NotTo(HaveOccurred())
		defer func() { _ = out.Close() }()

		cfg := config{
			LocalAIURL:       os.Getenv("ASK_EVAL_MODEL_URL"), //nolint:forbidigo // set by the harness only
			SynthesisModel:   os.Getenv("ASK_EVAL_MODEL"),     //nolint:forbidigo // set by the harness only
			SynthesisTimeout: 10 * time.Minute,
		}
		Expect(cfg.LocalAIURL).NotTo(BeEmpty())
		tenant, collection := os.Getenv("ASK_ENGINE_CASE_TENANT"), os.Getenv("ASK_ENGINE_CASE_COLLECTION") //nolint:forbidigo // set by the harness only

		for _, q := range questions {
			if len(only) > 0 && !only[q.ID] {
				continue
			}
			for _, arm := range arms {
				req := hybridQueryRequest{TenantID: tenant, CollectionID: collection, Query: q.Question, RequestClass: forensicrequest.GovernedAnalysis}
				callCtx, cancel := context.WithTimeout(ctx, 12*time.Minute)
				started := time.Now()
				line := map[string]any{"id": q.ID, "arm": strings.TrimSpace(arm)}
				var resp *hybridQueryResponse
				switch strings.TrimSpace(arm) {
				case "engine":
					var audit AskAuditV2
					resp, audit = runAskEngineV2(callCtx, cfg, pool, req, started)
					line["state"], line["reason"], line["attempts"], line["sql"], line["views"], line["repairs"], line["model_ms"], line["exec_ms"] =
						audit.State, audit.Reason, audit.Attempts, audit.SQL, audit.Views, audit.Repairs, audit.ModelMS, audit.ExecMS
					line["examples"], line["shape"] = audit.Examples, audit.Shape
				case "lane":
					var audit GovSQLAuditV1
					resp, audit = runGovernedSQLLane(callCtx, cfg, pool, req, started)
					line["state"], line["reason"], line["attempts"], line["sql"], line["views"], line["model_ms"], line["exec_ms"] =
						audit.State, audit.Reason, audit.Attempts, audit.SQL, audit.Views, audit.ModelMS, audit.ExecMS
				default:
					Fail("unknown arm " + arm)
				}
				cancel()
				line["total_ms"] = time.Since(started).Milliseconds()
				answer, rows := "", []map[string]any{}
				if resp != nil {
					answer = strings.TrimSpace(stringValueAny(resp.Answer["answer"]))
					if answer == "" {
						answer = strings.TrimSpace(stringValueAny(resp.Answer["clarification"]))
					}
					if grid, ok := resp.Enterprise["data_grid"].(map[string]any); ok {
						if all, ok := grid["rows"].([]map[string]any); ok {
							rows = all
						}
					}
					if len(rows) > 80 {
						rows = rows[:80]
					}
					if resp.Answer["result_state"] != "COMPLETED" && line["state"] == "answered" {
						line["state"] = "abstained"
					}
					if limits, ok := resp.Enterprise["limitations"].([]string); ok {
						line["limitations"] = limits // the engine's own caveat lines (link-analysis stage 3: gate 4 checks them by string)
					}
				}
				line["answer"], line["rows"] = answer, rows
				encoded, err := json.Marshal(line)
				Expect(err).NotTo(HaveOccurred())
				_, err = fmt.Fprintln(out, string(encoded))
				Expect(err).NotTo(HaveOccurred())
				_ = out.Sync()
			}
		}
	})
})
