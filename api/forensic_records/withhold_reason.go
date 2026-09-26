package main

import (
	"fmt"
	"strings"
)

// Why a withheld answer must say WHICH reason it was withheld for.
//
// Five distinct events shared one machine code (`no_verified_plan`) and one
// route label (`verified_only_withheld`), and three of them shared one
// sentence. Measured on the 62-question corpus, 2026-09-24: 12 of the 18
// clarifications returned the identical text "I can only state this from a plan
// I can verify... Name the field, the grouping or the time range you want" --
// covering at least three unrelated causes.
//
// That is wrong twice over.
//
// For the ANALYST it is wrong because the advice does not fit the question.
// DOC-04 asks "What does the case notes document say about plate ABC-123?" and
// was told to name a field, a grouping or a time range. There is no field,
// grouping or time range that fixes DOC-04: it was matched to VEHICLE records,
// and the honest thing to say is that the system looked at the wrong kind of
// evidence.
//
// For the INSTRUMENT it is wrong because a recorded run cannot be classified.
// The Phase 3 question is "which coverage gap produces these clarifications",
// and the recorded corpus answers it only for the two post-execution causes
// that happened to carry their own prose. This project's twice-paid lesson is
// that the instrument is fixed FIRST -- WI-6 found three runs mis-measured,
// WI-14 found 119 s attributed to nothing -- and this is the same shape.
//
// Nothing here changes WHETHER an answer is withheld. Every guard and its
// trigger are unchanged; only the code, the sentence and the audit detail
// differ.
const (
	withholdNoVerifiedPlan       = "no_verified_plan"
	withholdMediaMisroute        = "media_question_structured_template"
	withholdTargetNotFiltered    = "target_not_filtered"
	withholdUncomputedQuantity   = "uncomputed_quantity"
	withholdSingleFamilyNegative = "single_family_negative"
)

// withholdReason is one withholding event: what the analyst is told, why
// nothing was stated, and what the audit records for later classification.
type withholdReason struct {
	// Code is the machine-readable cause, carried on the clarification
	// contract's `reason_code` and in the planner audit.
	Code string
	// Question is the analyst-facing sentence. It names the actual obstacle
	// and asks for the thing that would actually remove it.
	Question string
	// Guardrail states why no result was stated, for the audit reader.
	Guardrail string
	// Detail is diagnostic only and never shown: the planner state that
	// explains the cause. Never contains evidence values.
	Detail string
}

// preExecutionWithhold reports the reason this request must not state an
// answer, or nil to proceed.
//
// SPECIFIC CAUSES ARE TESTED BEFORE THE GENERAL ONE. Several can hold at once
// -- DOC-01 has no verifiable plan AND is a media question answered by a
// structured template -- and the general cause, being general, tells the
// analyst the least. The old short-circuit ran the general test first, which is
// why every misroute in the corpus was reported as "name the field".
//
// The SET of withheld requests is identical either way: each branch already
// returned a withhold, so ordering selects the explanation, never the outcome.
func preExecutionWithhold(req hybridQueryRequest, template string) *withholdReason {
	if structuredTemplateAnsweringMediaQuestion(req, template) {
		return mediaMisrouteReason(req, template)
	}
	if answerNamesUnfilteredTarget(req) {
		return targetNotFilteredReason(req)
	}
	if verifiedOnlyWithholds(req) {
		return &withholdReason{
			Code: withholdNoVerifiedPlan,
			Question: "I can only state this from a plan I can verify, and I " +
				"could not derive one for this question. Name the field, the grouping or the time " +
				"range you want and I will compute it.",
			Guardrail: "No result was stated because no verifiable analytical plan " +
				"backed it: a keyword-matched template answered, and nothing checks that route.",
			Detail: noVerifiedPlanDetail(req),
		}
	}
	return nil
}

// mediaMisrouteReason names BOTH sides of the mismatch: the evidence the
// analyst asked about, and the evidence that actually answered. Naming only one
// leaves the analyst unable to tell a coverage gap from a misroute.
func mediaMisrouteReason(req hybridQueryRequest, template string) *withholdReason {
	artifact := namedMediaArtifact(req.Query)
	answered := "a different kind of evidence"
	familyID := ""
	if entry, ok := queryTemplateByName(template); ok {
		familyID = entry.FamilyID
		if layer, _ := defaultSemanticLayer(); layer != nil {
			if entity, resolved := layer.EntityByFamily(entry.FamilyID); resolved {
				answered = entityLabel(entity)
			}
		}
	}
	return &withholdReason{
		Code: withholdMediaMisroute,
		Question: fmt.Sprintf("You asked about %s, but this question matched %s -- a different kind "+
			"of evidence, which I have not searched for this. Tell me which evidence to search and "+
			"I will look there.", artifact, answered),
		Guardrail: fmt.Sprintf("No result was stated because the question names %s while a "+
			"structured analytical template answered from %s. A plan can be arithmetically sound "+
			"and still be computed over evidence the question is not about.", artifact, answered),
		Detail: fmt.Sprintf("artifact=%s answered_family=%s template=%s", artifact, familyOrNone(familyID), template),
	}
}

// targetNotFilteredReason is the WI-10 guard's own voice. The old text asked
// for a field or a time range; what is actually missing is the RESTRICTION to
// the named target, which the executed plan never applied.
func targetNotFilteredReason(req hybridQueryRequest) *withholdReason {
	target := strings.TrimSpace(req.Target)
	return &withholdReason{
		Code: withholdTargetNotFiltered,
		Question: fmt.Sprintf("This analysis measured every record of its kind, not only %s, so I "+
			"cannot report the result as a finding about %s. Ask me to restrict it to %s and I will "+
			"compute it.", target, target, target),
		Guardrail: fmt.Sprintf("No result was stated because the executed plan applied no filter on "+
			"%s while the answer would have named it. An unfiltered total attributed to one subject "+
			"is a confident wrong answer, not a rounding error.", target),
		Detail: fmt.Sprintf("target_named=yes filters=%d", sourceNativeFilterCount(req)),
	}
}

// namedMediaArtifact returns the analyst's own noun for the evidence they asked
// about, drawn from the SAME scope vocabulary the clarification options offer,
// so the sentence and the buttons under it agree.
func namedMediaArtifact(question string) string {
	lowered := strings.ToLower(question)
	for _, media := range mediaEvidenceScopes {
		for _, keyword := range media.Keywords {
			if strings.Contains(lowered, keyword) {
				return media.Noun
			}
		}
	}
	// mediaArtifactNouns is the broader set the guard keys on. If it fired and
	// no scope claimed the word, say so plainly rather than naming a scope the
	// question did not ask for.
	return "evidence of a kind these records do not hold"
}

// noVerifiedPlanDetail records the planner state that explains why no verified
// plan exists, so a recorded run can be classified WITHOUT re-running it.
//
// Deliberately field names and outcomes only. A withhold detail is written to
// every audited response, and evidence values belong in the plan, not in a
// diagnostic string.
func noVerifiedPlanDetail(req hybridQueryRequest) string {
	parts := []string{"family=" + familyOrNone(semanticQuestionFamily(req.Query))}
	audit := req.SemanticPlannerAudit
	if audit == nil {
		return strings.Join(append(parts, "audit=absent"), " ")
	}
	parts = append(parts, fmt.Sprintf("issued_fields=%d", len(audit.IRShadowIssuedFields)))
	if audit.BindingState != "" {
		parts = append(parts, "binding="+audit.BindingState)
	}
	if audit.IRFallbackOutcome != "" {
		parts = append(parts, "fallback="+audit.IRFallbackOutcome)
	}
	if audit.IRShadowOutcome != "" {
		parts = append(parts, "shadow="+audit.IRShadowOutcome)
	}
	if audit.IRShadowRejectedPlan != nil {
		parts = append(parts, "rejected_plan=yes")
	}
	if audit.IRArbitrated {
		parts = append(parts, "arbitrated=yes")
	}
	if audit.IRPlanCacheHit {
		parts = append(parts, "plan_cache=hit")
	}
	if len(audit.MissingFactKinds) > 0 {
		parts = append(parts, "missing_facts="+strings.Join(audit.MissingFactKinds, ","))
	}
	return strings.Join(parts, " ")
}

func familyOrNone(family string) string {
	if strings.TrimSpace(family) == "" {
		return "(none)"
	}
	return family
}

func sourceNativeFilterCount(req hybridQueryRequest) int {
	if req.SourceNative == nil {
		return 0
	}
	return len(req.SourceNative.Filters)
}

// applyWithhold turns a response into a clarification. One place, so the five
// causes cannot drift apart in HOW they withhold -- only in what they say.
func applyWithhold(req hybridQueryRequest, resp *hybridQueryResponse, reason withholdReason) {
	resp.Intent = intentClarify
	// The route label is UNCHANGED. Recorded runs, the live evaluation harness
	// and the reports all key on `verified_only_withheld`; the cause now lives
	// in `reason_code`, which is additive.
	resp.Route = []string{"verified_only_withheld"}
	resp.Answer["clarification_required"] = true
	resp.Answer["clarification"] = reason.Question
	resp.Answer["guardrail"] = reason.Guardrail
	resp.Clarification = &ClarificationRequestV1{
		ContractVersion: clarificationRequestContractV1,
		ReasonCode:      reason.Code,
		Question:        reason.Question,
		MissingFields:   []string{},
		Options:         clarificationOptions(req, nil),
		ExecutionHeld:   true,
	}
	if req.SemanticPlannerAudit != nil {
		req.SemanticPlannerAudit.VerifiedOnlyWithheld = true
		req.SemanticPlannerAudit.WithholdCode = reason.Code
		req.SemanticPlannerAudit.WithholdDetail = reason.Detail
	}
}
