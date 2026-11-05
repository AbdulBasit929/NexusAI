package main

import (
	"fmt"
	"time"
)

// ASK ENGINE V2: A QUESTION THAT RAN OUT OF TIME (the verified bank run of 2026-11-02, VB-BANK-004).
//
// "What is the largest single debit of the account X?" was asked twice. Both times the engine spent its 300 seconds of model time on three drafts (the first two were refused by an
// obligation that the verified example's own query did not meet, see ask_engine_rankings.go), the third call was cut by the budget, and the request was DECLINED, which sends the question on
// to the governed lane and the older path, each with model calls of its own. The client gave up at 420 seconds with no answer and no reason.
//
// Running out of time is not a finding about the case, and it is not a question the engine cannot answer. It is its own outcome, and it is final for the request: the paths that follow need
// the same model, on the same processor, and would only spend the rest of the analyst's wait. The response says that the time ran out, that nothing was concluded, and what to do.
// A timeout is never a verdict (NEXUSAI_CONTINUATION.md section 2).

// askModelTimedOut is whether a failed model call was a call that did not answer in time: it ran into the deadline it was given, or it ran for at least the shortest call the engine
// starts. A connection that is refused at once is the service being unavailable, which the following paths may be able to work around, and stays a decline.
func askModelTimedOut(failure string, expired bool, elapsed time.Duration) bool {
	return failure == "TIMEOUT_OR_UNAVAILABLE" && (expired || elapsed >= askMinModelCall)
}

// askTimedOut marks the audit of a question that ran out of time and returns its response.
func askTimedOut(req hybridQueryRequest, audit *AskAuditV2, why string) *hybridQueryResponse {
	audit.TimedOut = true
	audit.Reason = "time: " + why
	return askTimeoutResponse(req, *audit)
}

// askTimeoutResponse is the answer "I could not finish this in time". It shows no number, names no cause in the case, and says what to do.
func askTimeoutResponse(req hybridQueryRequest, audit AskAuditV2) *hybridQueryResponse {
	seconds := int(askQuestionBudget / time.Second)
	text := fmt.Sprintf("I could not finish this question in time, so there is no answer. The language model that writes the query is slow on this computer and has %d seconds for one question; it did not finish. "+
		"This says nothing about what the records hold. Ask again, or narrow the question by a number, a date or a place.", seconds)
	response := askClarification(req, text, "Nothing was run to completion, so no result is shown. A question that runs out of time is not a finding about the case.", audit, nil)
	response.Answer["timed_out"] = true
	response.Enterprise["timed_out"] = true
	return response
}
