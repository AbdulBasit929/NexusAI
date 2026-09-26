package main

// S4 CATALOG NARROWING — cut what the generator is offered, deterministically,
// so that the correct plan is the only one it can express.
//
// This is the stage the architecture calls "what makes a small model work". It
// is NOT an embedding ranker and must never become one: two live defects (D2,
// and the measure-hint defect found 2026-09-21) came from scoring a whole
// question against a catalogue to make a structural choice.
//
// The narrowing implemented here is the aggregate, not the field set. CDR-08
// asks "how many unique phone numbers appear as callers" and the generator
// answers with a plain COUNT of rows -- 8,642 where the truth is 10. S9 already
// refuses that, which is why the question clarifies rather than lying. So the
// schema and the verifier currently DISAGREE: one offers a plan the other will
// always reject, and the model spends 60-150 s discovering it.
//
// Measured before use, across the full 62: `frame.Goal == "distinct"` fires on
// exactly CDR-08 and ANPR-05, with zero false positives on the other sixty.
//
// Deliberately NOT attempted here, each recorded with its reason in
// reports/tier-decision-20260924/S4_NARROWING_RULE.md: narrowing the FIELD set
// for CDR-11 (21 questions share its goal, several correct today) and
// withholding measures for a `lookup` goal for TWR-02 (16 questions share it).
// Bundling those in would make the result unattributable, which is exactly what
// made the previous four regressions expensive to read.

// semanticRequestFrameGoal reads the goal the compiler already derived, and
// derives it only if the audit does not carry it.
//
// Extraction is deterministic, so recomputing yields the same answer; reading
// the recorded frame simply avoids paying for it twice on the hottest path.
func semanticRequestFrameGoal(req hybridQueryRequest) string {
	if req.SemanticPlannerAudit != nil && req.SemanticPlannerAudit.SemanticFrame != nil {
		return req.SemanticPlannerAudit.SemanticFrame.Goal
	}
	return extractSemanticFrame(req).Goal
}

// distinctOnlyFieldIDs returns the fields a DISTINCT question may count, or nil
// when the question is not a distinct question.
//
// nil means "do not narrow". Returning an empty non-nil slice would be far
// worse than not narrowing: an empty enum compiles to a GBNF alternation with
// no alternatives and llama.cpp answers the whole request with "failed to parse
// grammar" -- HTTP 500 before any inference. So a distinct question over a
// catalogue with nothing distinct-capable falls back to the unnarrowed schema
// and clarifies through S9 exactly as it does today.
func distinctOnlyFieldIDs(fields []FieldDescriptorV1, goal string) []string {
	if goal != "distinct" {
		return nil
	}
	ids := []string{}
	for _, field := range fields {
		if containsString(field.AllowedAggregates, "COUNT_DISTINCT") {
			ids = append(ids, field.FieldID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	return ids
}
