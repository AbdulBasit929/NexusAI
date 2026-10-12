package main

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// GOVERNED SQL LANE: WHAT THE EXISTING PATH MAY SPEND ON THE MODEL AFTER THE LANE HAS DECLINED.
//
// With the lane first (FORENSIC_GOVERNED_SQL_FIRST) a question the lane tried and could not answer goes on to the existing path,
// and for a question the ladder cannot answer that path asks the model again: the enum-constrained plan generator, first from the
// planner (FORENSIC_IR_FALLBACK) and then again from the arbitration that verified-only mode asks for (FORENSIC_IR_ARBITRATION),
// each allowed up to the 180 s planner ceiling. A generation takes 50 to 150 s on this CPU, so on a question the model cannot plan
// in that time both calls wait out their ceilings. The rehearsal of 2026-10-09 measured it: "How many call records were there in May
// 2026 versus June 2026?" took 458 s, and two probes like it 443 s and 409 s, all ending in the same "I can only state this from a
// plan I can verify" clarification. In the saved results of the last two, the lane's own time is 76 s and 83 s, so the existing path
// took the other 367 s and 325 s, which is two calls of up to 180 s.
//
// So a question the lane spent model time on and still declined gets the existing path ONE model attempt, of at most
// govSQLOlderPathModelLimit. After that the path goes on without a model and reaches the clarification (or the answer) it reaches
// without one. The allowance travels in the request's context and every place the existing path sends a request to the model asks
// for it first (olderPathModelAttempt), so a call site added later cannot skip the limit by being a different function.
//
// WHAT IS NOT CHANGED, on purpose:
//   - a request with no allowance in its context: the lane off, stage 1 (the lane after the existing path), and every question the
//     lane did not try (a dashboard operation, a text or media question, a family it does not recognise). Same timeouts, same calls.
//   - an answer already in the plan cache: a cached completion is not a model attempt and is served as before.
//   - the embedding call (it is capped at 2 s on its own) and the conversation front door (it runs before the lane).
//   - the lane's own model calls (two attempts of up to 240 s each, governed_sql_prompt.go).
//
// WHAT IT COSTS, measured on the 36 saved lane-first arms (1,754 answers): the lane tried and declined 29 times. The existing path
// then ended with a clarification 18 times (12 of them took more than 30 s, up to 367 s: those come sooner now), with a wrong
// answer once (SUBS-count_distinct-01, 58 s: it becomes a clarification) and with a correct answer 10 times. The ten are four
// question wordings (two of CDR-sum-01, one each of TOWE-count_distinct-01 and TOWE-top_group-01): five askings rescued by a model
// generation of 70 to 140 s (one took 3,059 s), and five repeats of those wordings in later arms, served by the plan cache in 5 to 9 s.
// A generation cut at 30 s is never cached, so the limit gives up all ten. Raising govSQLOlderPathModelLimit to 90 s would keep the
// two TOWE askings (70 s and 77 s) and cost 90 s on every question that gets no plan.
const govSQLOlderPathModelAttempts = 1

// govSQLOlderPathModelLimit is the longest the one attempt may take. A variable only so that a spec can shorten it.
var govSQLOlderPathModelLimit = 30 * time.Second

type govSQLModelBudget struct {
	mu       sync.Mutex
	attempts int
	limit    time.Duration
}

type govSQLModelBudgetKey struct{}

// govSQLOlderPathIsRescue says whether the existing path, running after the lane, is rescuing a question the lane tried: the lane
// spent model time on it (an attempt was made) and answered nothing. A decline before the model, because the question is not for
// the lane, leaves the existing path as the lane's contract promises: exactly as if the lane did not exist.
func govSQLOlderPathIsRescue(audit GovSQLAuditV1) bool {
	return audit.Attempts > 0
}

// govSQLContextAfterDecline returns the context the existing path runs in once the lane has given no answer.
func govSQLContextAfterDecline(ctx context.Context, audit GovSQLAuditV1) context.Context {
	if !govSQLOlderPathIsRescue(audit) {
		return ctx
	}
	return withGovSQLModelBudget(ctx, govSQLOlderPathModelAttempts, govSQLOlderPathModelLimit)
}

func withGovSQLModelBudget(ctx context.Context, attempts int, limit time.Duration) context.Context {
	return context.WithValue(ctx, govSQLModelBudgetKey{}, &govSQLModelBudget{attempts: attempts, limit: limit})
}

// olderPathModelAttempt is asked by the existing path just before it sends a request to the model, with the timeout it would use.
// It returns the timeout to use and whether to send the request at all. Without an allowance in the context it returns the
// timeout it was given and true, and records nothing. With one, the first request gets the shorter of the two timeouts and uses
// up the allowance; every later request is refused, and its caller takes the same road as for a model that did not answer.
func olderPathModelAttempt(ctx context.Context, timeout time.Duration) (time.Duration, bool) {
	budget, _ := ctx.Value(govSQLModelBudgetKey{}).(*govSQLModelBudget)
	if budget == nil {
		return timeout, true
	}
	budget.mu.Lock()
	defer budget.mu.Unlock()
	if budget.attempts <= 0 {
		slog.Info("governed sql older path", "model_attempt", "refused", "reason", "the one attempt after a lane decline is spent")
		return 0, false
	}
	budget.attempts--
	if budget.limit > 0 && (timeout <= 0 || budget.limit < timeout) {
		timeout = budget.limit
	}
	slog.Info("governed sql older path", "model_attempt", "granted", "limit_ms", timeout.Milliseconds())
	return timeout, true
}
