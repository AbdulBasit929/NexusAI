package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// tooSlowModel stands in for a model that cannot answer in time: it counts the requests it is sent and answers none of them,
// holding each until the caller gives up.
func tooSlowModel(requests *atomic.Int32) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		// The server notices that the caller gave up only after it has read the whole request.
		_, _ = io.Copy(io.Discard, r.Body)
		select {
		case <-r.Context().Done():
		case <-time.After(10 * time.Second):
		}
		http.Error(w, "too slow", http.StatusGatewayTimeout)
	}))
}

// shortenOlderPathModelLimit sets the one attempt's limit for the length of a spec.
func shortenOlderPathModelLimit(limit time.Duration) {
	previous := govSQLOlderPathModelLimit
	govSQLOlderPathModelLimit = limit
	DeferCleanup(func() { govSQLOlderPathModelLimit = previous })
}

var _ = Describe("Governed SQL lane: what the existing path may spend on the model after a decline", func() {
	declinedAfterTrying := GovSQLAuditV1{State: "declined", Attempts: 2, Reason: "the result was one value where the question compares values"}

	Describe("the allowance", func() {
		It("is not made for a request the lane did not try, and then no call is changed", func() {
			for _, audit := range []GovSQLAuditV1{
				{State: "declined", Reason: "no evidence family recognised in the question"},
				{State: "declined", Reason: "text and media evidence is answered by the retrieval path"},
				{State: "declined", Reason: "an explicit operation was requested"},
			} {
				ctx := govSQLContextAfterDecline(context.Background(), audit)
				for range 5 {
					timeout, allowed := olderPathModelAttempt(ctx, 180*time.Second)
					Expect(allowed).To(BeTrue())
					Expect(timeout).To(Equal(180 * time.Second))
				}
			}
		})

		It("changes nothing for a request that never went through the lane", func() {
			for range 5 {
				timeout, allowed := olderPathModelAttempt(context.Background(), 120*time.Second)
				Expect(allowed).To(BeTrue())
				Expect(timeout).To(Equal(120 * time.Second))
			}
		})

		It("is one attempt of at most 30 seconds once the lane has spent model time and declined", func() {
			Expect(govSQLOlderPathModelLimit).To(Equal(30 * time.Second))
			for _, attempts := range []int{1, 2} {
				ctx := govSQLContextAfterDecline(context.Background(), GovSQLAuditV1{State: "declined", Attempts: attempts})
				timeout, allowed := olderPathModelAttempt(ctx, 180*time.Second)
				Expect(allowed).To(BeTrue(), "the first attempt is allowed")
				Expect(timeout).To(Equal(30*time.Second), "and held to the limit, not to the 180 s planner ceiling")
				_, allowed = olderPathModelAttempt(ctx, 180*time.Second)
				Expect(allowed).To(BeFalse(), "a second attempt is refused")
				_, allowed = olderPathModelAttempt(ctx, 60*time.Second)
				Expect(allowed).To(BeFalse(), "and so is every later one")
			}
		})

		It("never lengthens a timeout that is already shorter than the limit", func() {
			ctx := govSQLContextAfterDecline(context.Background(), declinedAfterTrying)
			timeout, allowed := olderPathModelAttempt(ctx, 8*time.Second)
			Expect(allowed).To(BeTrue())
			Expect(timeout).To(Equal(8 * time.Second))
		})

		It("belongs to one request: another request starts with its own", func() {
			first := govSQLContextAfterDecline(context.Background(), declinedAfterTrying)
			second := govSQLContextAfterDecline(context.Background(), declinedAfterTrying)
			_, allowed := olderPathModelAttempt(first, time.Minute)
			Expect(allowed).To(BeTrue())
			_, allowed = olderPathModelAttempt(first, time.Minute)
			Expect(allowed).To(BeFalse())
			_, allowed = olderPathModelAttempt(second, time.Minute)
			Expect(allowed).To(BeTrue())
		})
	})

	Describe("at the plan generator, the call that ran to its ceiling twice", func() {
		// The planner asks for a plan and, when verified-only mode is about to withhold, the arbitration asks again: two calls
		// to resolveSemanticIRFallback for one question, which is what took 325 to 367 s in the rehearsal.
		compareMonths := func() hybridQueryRequest {
			return hybridQueryRequest{
				Query: "How many call records were there in May 2026 versus June 2026?", TenantID: "t", UserID: "u", CollectionID: "c",
				SynthesisModel: "scripted-model", QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)},
				SemanticPlannerAudit: &SemanticPlannerAuditV1{},
			}
		}

		It("sends one request after a decline, within the limit, and refuses the second", func() {
			GinkgoT().Setenv(semanticIRFallbackEnv, "true")
			var requests atomic.Int32
			server := tooSlowModel(&requests)
			DeferCleanup(server.Close)
			shortenOlderPathModelLimit(150 * time.Millisecond)
			// Without the allowance this call would wait five seconds for a model that never answers.
			cfg := config{LocalAIURL: server.URL, SemanticPlannerTimeout: 5 * time.Second}
			ctx := govSQLContextAfterDecline(context.Background(), declinedAfterTrying)

			started := time.Now()
			first := compareMonths()
			_, state := resolveSemanticIRFallback(ctx, cfg, first, "CLARIFICATION")
			Expect(state).To(Equal("CLARIFICATION"), "a plan that did not come leaves the deterministic outcome")
			Expect(first.SemanticPlannerAudit.IRFallbackOutcome).To(Equal("generator_timeout_or_unavailable"))
			Expect(time.Since(started)).To(BeNumerically("<", 2*time.Second), "held to the limit, not to the planner timeout")

			second := compareMonths()
			_, state = resolveSemanticIRFallback(ctx, cfg, second, "CLARIFICATION")
			Expect(state).To(Equal("CLARIFICATION"))
			Expect(second.SemanticPlannerAudit.IRFallbackOutcome).To(Equal("generator_model_budget_spent"))
			Expect(requests.Load()).To(Equal(int32(1)), "the model was asked once")
			Expect(time.Since(started)).To(BeNumerically("<", 2*time.Second))
		})

		It("still sends both requests, each up to the planner timeout, with no allowance in force", func() {
			GinkgoT().Setenv(semanticIRFallbackEnv, "true")
			var requests atomic.Int32
			server := tooSlowModel(&requests)
			DeferCleanup(server.Close)
			cfg := config{LocalAIURL: server.URL, SemanticPlannerTimeout: 400 * time.Millisecond}

			started := time.Now()
			for range 2 {
				audited := compareMonths()
				_, state := resolveSemanticIRFallback(context.Background(), cfg, audited, "CLARIFICATION")
				Expect(state).To(Equal("CLARIFICATION"))
				Expect(audited.SemanticPlannerAudit.IRFallbackOutcome).To(Equal("generator_timeout_or_unavailable"))
			}
			Expect(requests.Load()).To(Equal(int32(2)), "the existing path keeps both of its attempts")
			Expect(time.Since(started)).To(BeNumerically(">=", 700*time.Millisecond), "each ran to the timeout it was given")
		})
	})

	Describe("at the residual chooser", func() {
		residualRequest := func() hybridQueryRequest {
			return hybridQueryRequest{
				Query: "Inspect 198.51.100.97", TenantID: "t", UserID: "u", CollectionID: "c",
				SynthesisModel: "qwen3-4b-instruct-2507-q4km-nxb21d-dev", QueryScope: queryEvidenceScope{Kind: "authorized_workspace"},
			}
		}
		answering := func(requests *atomic.Int32) *httptest.Server {
			return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				raw, _ := json.Marshal(map[string]string{"decision": "ipdr.endpoint/AGGREGATE/authorized_workspace"})
				_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]string{"content": string(raw)}}}})
			}))
		}

		It("asks the model once after a decline and then goes on without it", func() {
			var requests atomic.Int32
			server := answering(&requests)
			DeferCleanup(server.Close)
			ctx := govSQLContextAfterDecline(context.Background(), declinedAfterTrying)

			out, status := resolveHybridPlanner(ctx, config{LocalAIURL: server.URL}, residualRequest())
			Expect(status).To(Equal("hybrid_model_plan"))
			Expect(out.HybridAudit.DecisionSource).To(Equal("MODEL_DECISION"))

			out, status = resolveHybridPlanner(ctx, config{LocalAIURL: server.URL}, residualRequest())
			Expect(status).To(Equal("model_budget_spent"))
			Expect(out.HybridAudit.DecisionSource).To(Equal("DETERMINISTIC_FACT"), "no model decision was asked for")
			Expect(out.HybridAudit.FinalPlan).To(BeNil())
			Expect(requests.Load()).To(Equal(int32(1)))
		})

		It("asks the model every time with no allowance in force", func() {
			var requests atomic.Int32
			server := answering(&requests)
			DeferCleanup(server.Close)
			for range 2 {
				_, status := resolveHybridPlanner(context.Background(), config{LocalAIURL: server.URL}, residualRequest())
				Expect(status).To(Equal("hybrid_model_plan"))
			}
			Expect(requests.Load()).To(Equal(int32(2)))
		})
	})

	Describe("through the whole handler, with the plan generator on as in the live deployment", func() {
		const question = "How many call records were there in May 2026 versus June 2026?"
		type outcome struct {
			elapsed       time.Duration
			laneCalls     int
			modelRequests int32
			body          map[string]any
			header        string
		}
		var (
			requests  atomic.Int32
			laneCalls int
			laneAudit GovSQLAuditV1
			endpoint  *httptest.Server
		)
		BeforeEach(func() {
			// FORENSIC_IR_FALLBACK asks the plan generator from the planner, FORENSIC_IR_ARBITRATION asks it again when
			// FORENSIC_VERIFIED_ONLY is about to withhold: the two calls that ran to their ceilings.
			for _, name := range []string{semanticIRFallbackEnv, semanticIRArbitrationEnv, semanticVerifiedOnlyEnv} {
				GinkgoT().Setenv(name, "true")
			}
			requests.Store(0)
			laneCalls = 0
			endpoint = tooSlowModel(&requests)
			DeferCleanup(endpoint.Close)
			previous := govSQLLaneRun
			govSQLLaneRun = func(context.Context, config, *pgxpool.Pool, hybridQueryRequest, time.Time) (*hybridQueryResponse, GovSQLAuditV1) {
				laneCalls++
				return nil, laneAudit
			}
			DeferCleanup(func() { govSQLLaneRun = previous })
		})
		// ask sends the question through the public handler, with the lane switches given, and reports what happened.
		ask := func(laneOn, laneFirst string, plannerTimeout time.Duration) outcome {
			GinkgoT().Setenv(governedSQLEnv, laneOn)
			GinkgoT().Setenv(governedSQLFirstEnv, laneFirst)
			cfg := config{LocalAIURL: endpoint.URL, SynthesisModel: "scripted-model", SemanticPlannerTimeout: plannerTimeout, SynthesisTimeout: plannerTimeout}
			body := `{"tenant_id":"default","collection_id":"records-demo","query":"` + question + `"}`
			recorder := httptest.NewRecorder()
			started := time.Now()
			hybridQueryHandler(cfg, nil).ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/query/hybrid", strings.NewReader(body)))
			result := outcome{elapsed: time.Since(started), laneCalls: laneCalls, modelRequests: requests.Load(), header: recorder.Header().Get("X-Governed-SQL")}
			Expect(recorder.Code).To(Equal(http.StatusOK))
			Expect(json.Unmarshal(recorder.Body.Bytes(), &result.body)).To(Succeed())
			return result
		}
		withheld := func(result outcome) string {
			Expect(result.body["route"]).To(ConsistOf("verified_only_withheld"))
			clarification, _ := result.body["clarification"].(map[string]any)
			return stringValueAny(clarification["reason_code"])
		}

		It("gives the existing path one short model attempt after the lane tried and declined, and the same clarification", func() {
			shortenOlderPathModelLimit(150 * time.Millisecond)
			laneAudit = GovSQLAuditV1{State: "declined", Attempts: 2, Reason: "the result was one value where the question compares values"}
			// A model that never answers, and a planner timeout of 3 s: the old behaviour is two waits of 3 s.
			result := ask("true", "true", 3*time.Second)
			Expect(result.laneCalls).To(Equal(1))
			Expect(result.header).To(ContainSubstring("state=declined; attempts=2"))
			Expect(result.modelRequests).To(Equal(int32(1)), "the existing path asked the model once")
			Expect(result.elapsed).To(BeNumerically("<", 2*time.Second), "and for the limit, not for the planner timeout")
			Expect(withheld(result)).To(Equal(withholdNoVerifiedPlan))
		})

		It("leaves the existing path both of its attempts when the lane declined before it asked the model", func() {
			shortenOlderPathModelLimit(150 * time.Millisecond)
			laneAudit = GovSQLAuditV1{State: "declined", Attempts: 0, Reason: "no evidence family recognised in the question"}
			result := ask("true", "true", 500*time.Millisecond)
			Expect(result.laneCalls).To(Equal(1))
			Expect(result.modelRequests).To(Equal(int32(2)))
			Expect(result.elapsed).To(BeNumerically(">=", 900*time.Millisecond), "each ran to the planner timeout")
			Expect(withheld(result)).To(Equal(withholdNoVerifiedPlan))
		})

		It("leaves the existing path both of its attempts when the lane runs after it, and when the lane is off", func() {
			shortenOlderPathModelLimit(150 * time.Millisecond)
			laneAudit = GovSQLAuditV1{State: "declined", Attempts: 2}
			for _, switches := range [][2]string{{"true", "false"}, {"false", "false"}} {
				requests.Store(0)
				result := ask(switches[0], switches[1], 500*time.Millisecond)
				Expect(result.laneCalls).To(BeZero(), "the lane is not entered first: %v", switches)
				Expect(result.modelRequests).To(Equal(int32(2)), "switches %v", switches)
				Expect(result.elapsed).To(BeNumerically(">=", 900*time.Millisecond), "switches %v", switches)
				Expect(withheld(result)).To(Equal(withholdNoVerifiedPlan))
			}
		})
	})

	Describe("at the narrative", func() {
		narrate := func(ctx context.Context, url string) (NarrativeV1, string) {
			req := hybridQueryRequest{SynthesisModel: "qwen3-4b-instruct-2507-q4km-nxb21d-dev"}
			return synthesizeFactPacketNarrative(ctx, config{LocalAIURL: url, SynthesisTimeout: 400 * time.Millisecond}, req, FactPacketV1{})
		}

		It("is skipped, with the deterministic result kept, once the one attempt is spent", func() {
			var requests atomic.Int32
			server := tooSlowModel(&requests)
			DeferCleanup(server.Close)
			shortenOlderPathModelLimit(100 * time.Millisecond)
			ctx := govSQLContextAfterDecline(context.Background(), declinedAfterTrying)
			_, spent := olderPathModelAttempt(ctx, time.Minute)
			Expect(spent).To(BeTrue())

			narrative, reason := narrate(ctx, server.URL)
			Expect(narrative.ContractVersion).To(BeEmpty())
			Expect(reason).To(ContainSubstring("deterministic result retained"))
			Expect(requests.Load()).To(BeZero(), "no request was sent")
		})

		It("uses the one attempt, within the limit, when nothing else has", func() {
			var requests atomic.Int32
			server := tooSlowModel(&requests)
			DeferCleanup(server.Close)
			shortenOlderPathModelLimit(100 * time.Millisecond)
			ctx := govSQLContextAfterDecline(context.Background(), declinedAfterTrying)

			started := time.Now()
			_, reason := narrate(ctx, server.URL)
			Expect(reason).To(ContainSubstring("timed out"))
			Expect(requests.Load()).To(Equal(int32(1)))
			Expect(time.Since(started)).To(BeNumerically("<", 350*time.Millisecond), "the limit (100 ms), not the configured 400 ms")
		})

		It("is attempted as before with no allowance in force", func() {
			var requests atomic.Int32
			server := tooSlowModel(&requests)
			DeferCleanup(server.Close)
			started := time.Now()
			_, reason := narrate(context.Background(), server.URL)
			Expect(reason).To(ContainSubstring("timed out"))
			Expect(requests.Load()).To(Equal(int32(1)))
			Expect(time.Since(started)).To(BeNumerically(">=", 380*time.Millisecond))
		})
	})
})
