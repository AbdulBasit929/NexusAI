package main

import (
	"context"
	"os"
	"strings"
)

// Inverting the gate.
//
// The enum-constrained IR generator (resolveSemanticDynamicPlan) was
// UNREACHABLE in production: its only caller sat inside the LLM operation
// selector, and that selector was disabled after it twice chose a confidently
// wrong operation (forensics.top_locations for "top 5 phone numbers by calls").
// So the component WI-0 measured at 74.3% execution-equivalent with ZERO
// hallucinated fields across 86 live calls never ran at all.
//
// The selector and the IR generator fail differently, and conflating them is
// what kept the good one switched off:
//
//	the SELECTOR picks one of ~104 opaque operation IDs and cannot be checked —
//	a wrong pick is a confident wrong answer.
//	the IR GENERATOR fills a typed plan whose field slots are an enum of
//	authorized IDs. It cannot name a field that does not exist, and everything
//	it produces is then re-validated: S6 shape/aggregate/filter rules, S9 SHAPE,
//	and the CONSTRAINT_APPLIED obligations.
//
// So the selector stays disabled and the IR generator becomes the fallback the
// deterministic compiler hands off to.
//
// It is a FALLBACK, never the default: measured p95 is 130-143s on this CPU
// against ~2.5s for the deterministic path, so it runs only where the
// deterministic compiler could not resolve the question at all.

const semanticIRFallbackEnv = "FORENSIC_IR_FALLBACK"

// semanticIRFallbackEnabled gates the generator. Enabled 2026-09-22.
//
// It shipped OFF because TXN-02 answered 75,000 on one build and 9,200 on the
// next, recorded as "field selection is not stable at this model size". That
// diagnosis was WRONG. The curated layer declares transaction.amount as NUMBER,
// the SQL builder switches on INTEGER/DECIMAL, so every curated numeric column
// fell through to the raw-text path: MAX() was lexicographic and "9200" sorts
// after "75000". A type mapping, not the model. Fixed, with a test that fails
// if any curated type stops reaching the executor as an executable type.
//
// With that and the GBNF maxItems fix in, the golden-IR set measures 68%
// plan-exact (25/37) against 16% before, 0 fields outside the issued enum, and
// on the 4 questions where this fallback actually fires: +1 correct, 0
// confident-wrong. NEG-01 is the proof the guards still hold — the generator
// proposed a plan on a negative test and verification discarded it.
//
// Still a FALLBACK, never the default: ~50s against ~2.5s for the deterministic
// path, so it runs only where the compiler could not resolve the question.
func semanticIRFallbackEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(semanticIRFallbackEnv)), "true")
}

// semanticIRFallbackStates are the deterministic outcomes that mean "the
// compiler could not answer this". Anything else already has an answer and must
// not be second-guessed by a model.
func semanticIRFallbackApplies(state string) bool {
	switch state {
	case "AMBIGUOUS_INTENT", "UNSUPPORTED_REQUEST_CLASS",
		"UNRESOLVED_ISSUED_FIELD_OR_OPERATION", "NO_AUTHORIZED_ALGEBRA_OR_OPERATION":
		return true
	}
	// A refusal that already NAMES what it could not bind is a better answer
	// than a guess, so those are kept: the analyst can act on them.
	return strings.HasPrefix(state, "MEASURE_FIELD_UNRESOLVED")
}

// resolveSemanticIRFallback asks the enum-constrained generator for a typed plan
// when the deterministic compiler produced none, then re-verifies the result
// exactly as if it had been compiled locally. A plan that fails any check is
// discarded and the original refusal stands — the model widens coverage, it
// never relaxes a guard.
func resolveSemanticIRFallback(ctx context.Context, cfg config, req hybridQueryRequest, deterministic string) (hybridQueryRequest, string) {
	if !semanticIRFallbackEnabled() || cfg.LocalAIURL == "" {
		return req, deterministic
	}
	family := semanticQuestionFamily(req.Query)
	if family == "" {
		// With no family there is no authorized field set to enumerate, and an
		// enum is the whole reason this path is safe.
		return req, deterministic
	}
	if !semanticDynamicFamilyAllowed(req, family) {
		return req, deterministic
	}
	// Why the fallback did or did not produce an answer is an AUDIT RECORD, not
	// debug output: "why did the system answer this way" must be reconstructable
	// months later. Record the outcome on the request that is actually returned.
	// Generation time is recorded inside resolveSemanticDynamicPlan, which is
	// the single place it is actually spent and covers all three callers.
	// Timing it here instead missed every path that returns without noting an
	// outcome -- measured 2026-09-24, three of four probes reported 0 ms while
	// taking 29-75 s.
	note := func(target hybridQueryRequest, outcome string) {
		if target.SemanticPlannerAudit != nil {
			target.SemanticPlannerAudit.IRFallbackOutcome = outcome
		}
	}
	resolved, state := resolveSemanticDynamicPlan(ctx, cfg, req, family)
	if state != "semantic_dynamic_plan" || resolved.SourceNative == nil {
		note(req, "generator_"+state)
		return req, deterministic
	}
	// Re-verify. The generator is constrained, not trusted.
	frame := extractSemanticFrame(req)
	if err := verifySourceNativePlanShape(frame, req.Query, resolved.SourceNative); err != nil {
		note(req, "rejected_shape")
		return req, deterministic
	}
	if unbound := deterministicUnboundConstraints(resolved, frame, resolved.SourceNative); len(unbound) > 0 {
		note(req, "rejected_unbound:"+strings.Join(unbound, ","))
		return req, deterministic
	}
	note(resolved, "accepted")
	if resolved.SemanticPlannerAudit != nil {
		resolved.SemanticPlannerAudit.State = "semantic_ir_fallback"
		resolved.SemanticPlannerAudit.DynamicSelected = true
	}
	return resolved, "semantic_ir_fallback"
}

const semanticIRArbitrationEnv = "FORENSIC_IR_ARBITRATION"

// semanticIRArbitrationEnabled lets a re-verified generated plan REPLACE a
// clarification, from any route -- including the keyword ladder and the
// registered templates, which the fallback never reaches because they do not
// report an unresolved state.
//
// Scoped deliberately to clarifications. A clarification asserts nothing, so
// substituting a plan that passed S6, S9 SHAPE and CONSTRAINT_APPLIED cannot
// turn a correct answer into a wrong one. Arbitrating against a CONFIDENT
// answer was measured on the same 62 questions and rejected: the shape-mismatch
// signal fired on 3 wrong answers and 2 correct ones, and converting a correct
// answer into a wrong one is the one failure this product does not accept.
//
// Its own switch, so it can be measured and withdrawn independently of the
// fallback.
func semanticIRArbitrationEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(semanticIRArbitrationEnv)), "true")
}

// --- shadow mode ------------------------------------------------------------

const semanticIRShadowEnv = "FORENSIC_IR_SHADOW"

// semanticIRShadowEnabled runs the generator on EVERY question and records what
// it produced without ever using it. Off by default.
//
// This exists because the fallback could not be turned on: field selection was
// unstable, and "unstable" is not something you can fix. Shadow mode measures
// the generator against hand-written gold plans on the real production path, so
// the failure becomes specific — which field, which question, which aggregate —
// instead of anecdotal. It is measurement, so it must never touch the answer.
func semanticIRShadowEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(semanticIRShadowEnv)), "true")
}

// recordSemanticIRShadow generates a plan and files it on the supplied audit.
// It writes ONLY to that audit: the shadow plan is evidence, never an answer,
// and a failure here must leave the response untouched.
//
// `probe` is the request as it stood BEFORE routing mutated it, and that is not
// a detail. The keyword ladder sets RecordType; RecordType narrows the rows
// sourceNativeCatalogSampleRows draws the field catalogue from; the catalogue
// becomes the enum handed to the generator. Measuring on whatever each route
// happened to leave behind would compare the generator against a different
// field set per question, which is not one measurement.
func recordSemanticIRShadow(ctx context.Context, cfg config, probe hybridQueryRequest, audit *SemanticPlannerAuditV1) {
	if audit == nil || !semanticIRShadowEnabled() || cfg.LocalAIURL == "" {
		return
	}
	if audit.IRShadowOutcome != "" {
		return // already measured on this request
	}
	family := semanticQuestionFamily(probe.Query)
	if family == "" || !semanticDynamicFamilyAllowed(probe, family) {
		audit.IRShadowOutcome = "no_family"
		return
	}
	// A COPY is handed to the generator so nothing it sets can reach the answer.
	probeAudit := &SemanticPlannerAuditV1{}
	probe.SemanticPlannerAudit = probeAudit
	generated, state := resolveSemanticDynamicPlan(ctx, cfg, probe, family)
	// Record the enum the generator was actually offered, whatever the outcome.
	// The catalogue is sampled per request, so it cannot be reconstructed later.
	for _, field := range probeAudit.RetrievedFields {
		audit.IRShadowIssuedFields = append(audit.IRShadowIssuedFields, field.FieldID)
	}
	if state != "semantic_dynamic_plan" || generated.SourceNative == nil {
		audit.IRShadowOutcome = state
		// A plan the validator rejected is still the best evidence of WHAT the
		// generator proposed. Filed separately so the scorer never mistakes it
		// for a plan that was accepted.
		if rejected, ok := probeAudit.DynamicPlan.(*SourceNativePlanV1); ok && rejected != nil {
			audit.IRShadowRejectedPlan = rejected
		}
		return
	}
	audit.IRShadowOutcome = "generated"
	audit.IRShadowPlan = generated.SourceNative
}

// ensureSemanticIRShadow measures the generator on EVERY question, whichever
// route answered it.
//
// The first version of shadow mode hung off resolveOpenEndedSemanticPlanner, so
// it only saw questions that reached the semantic planner. Questions the
// keyword ladder resolved first — query.go's deterministic fast path, and
// anything planRuntimeQuery matched outright — never entered the planner at
// all, and the golden-IR scorer recorded them as `absent`: indistinguishable,
// from the outside, from a generator that produced nothing. That is a defect in
// the instrument, and it under-samples exactly the questions the product
// already answers well.
//
// `routed` is the routing status that actually answered, recorded so the
// scorer can tell the paths apart instead of inferring them from latency.
func ensureSemanticIRShadow(ctx context.Context, cfg config, req hybridQueryRequest, probe hybridQueryRequest, routed string) hybridQueryRequest {
	if !semanticIRShadowEnabled() || cfg.LocalAIURL == "" {
		return req
	}
	if req.SemanticPlannerAudit == nil {
		// The ladder never builds an audit. Creating an empty one cannot change
		// an answer: query.go's binding check reads Selected and BindingState,
		// both of which stay zero, and nothing branches on State. Shadow mode is
		// measurement-only and off by default besides.
		req.SemanticPlannerAudit = &SemanticPlannerAuditV1{
			ContractVersion: "forensics.retrieval-first-semantic-planner/v1",
			FactAuthority:   "SERVER_DETERMINISTIC_ONLY",
			State:           routed,
		}
	}
	audit := req.SemanticPlannerAudit
	// The fallback runs the same generator on the same question. With both
	// switches on, reuse its accepted plan rather than pay a second ~130s
	// generation to learn what we already recorded.
	if audit.IRFallbackOutcome == "accepted" && req.SourceNative != nil {
		audit.IRShadowOutcome, audit.IRShadowPlan = "generated", req.SourceNative
		return req
	}
	recordSemanticIRShadow(ctx, cfg, probe, audit)
	return req
}

// willClarify predicts, BEFORE the request is turned into an execution plan,
// that the deterministic route is going to ask a question instead of answering
// one. It mirrors the binding half of the real clarification condition in
// query.go: a template whose required identifier the question never supplied.
//
// It is deliberately narrower than that condition. The other arms -- a rejected
// sensitive target, a missing source_set for a comparison -- are refusals with
// their own governed remedies, and a generated plan must not paper over them.
func willClarify(req hybridQueryRequest, template string) bool {
	// A template that demands a target identifier the question never supplies.
	// "Which cell site handled the most calls?" names no cell site because it is
	// ASKING which one; the route then asks the analyst to supply the answer.
	if needsClarification(req.Query, template, req.Target, req.Targets) {
		return true
	}
	if req.SemanticPlannerAudit == nil || req.SemanticPlannerAudit.Selected == nil {
		return false
	}
	state, _ := semanticBindingReadiness(req, *req.SemanticPlannerAudit.Selected)
	return state == "USER_FACT_REQUIRED"
}
