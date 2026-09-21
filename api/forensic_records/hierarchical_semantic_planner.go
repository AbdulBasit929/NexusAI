package main

import (
	"context"
	"sort"
	"time"
)

type semanticFamilyGroup struct {
	ID      string `json:"id"`
	Meaning string `json:"meaning"`
}

// Groups partition the already scope-filtered registry, never question keywords.
func semanticFamilyGroups(candidates []SemanticOperationCandidateV1) []semanticFamilyGroup {
	seen := map[string]bool{}
	out := []semanticFamilyGroup{}
	for _, c := range candidates {
		if c.FamilyID != "" && !seen[c.FamilyID] {
			seen[c.FamilyID] = true
			out = append(out, semanticFamilyGroup{ID: c.FamilyID, Meaning: semanticFamilyMeaning(c.FamilyID, candidates)})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
func semanticFamilyCandidates(all []SemanticOperationCandidateV1, family string) []SemanticOperationCandidateV1 {
	out := []SemanticOperationCandidateV1{}
	for _, c := range all {
		if c.FamilyID == family {
			out = append(out, c)
		}
	}
	return out
}

func resolveOpenEndedSemanticPlanner(ctx context.Context, cfg config, req hybridQueryRequest) (hybridQueryRequest, string) {
	eligible := semanticOperationCandidates(req)
	retrieved, scores := rankSemanticOperationsWithScores(req.Query, eligible)
	audit := &SemanticPlannerAuditV1{
		ContractVersion:        "forensics.retrieval-first-semantic-planner/v1",
		Language:               detectQueryLanguage(req.Query).Tag,
		RegistryCandidateCount: len(eligible),
		EligibleOperationCount: len(eligible),
		RetrievedCandidates:    retrieved,
		RetrievalScores:        scores,
		TopK:                   len(retrieved),
		CandidateCount:         len(retrieved),
		Candidates:             retrieved,
		RequestClass:           semanticRequestClass(req),
		ScopeFilterReason:      semanticScopeFilterReason(req),
		FactAuthority:          "SERVER_DETERMINISTIC_ONLY",
	}
	req.SemanticPlannerAudit = audit
	finish := func(state string) (hybridQueryRequest, string) { audit.State = state; return req, state }
	if audit.RequestClass == semanticRequestProductHelp || audit.RequestClass == semanticRequestGeneralDomainKnowledge {
		audit.SelectedDecision = "GENERAL_PRODUCT_HELP"
		return finish("GENERAL_PRODUCT_HELP")
	}
	if audit.Language != "en" || len(retrieved) == 0 {
		return finish("unavailable")
	}
	start := time.Now()
	resolved, state := applyDeterministicSemanticCompiler(ctx, cfg, req)
	if resolved.SemanticPlannerAudit != nil {
		resolved.SemanticPlannerAudit.OperationLatencyMS = time.Since(start).Milliseconds()
		resolved.SemanticPlannerAudit.State = state
	}
	// Re-enabling the bounded model fallback here was attempted twice this
	// session. The first attempt picked forensics.top_locations for "top 5
	// phone numbers by number of calls made" (locations instead of phone
	// numbers) with high confidence; a grouping-alignment grounding check
	// was added and verified against that exact case. The second broad
	// verification batch immediately found the SAME failure mode recur with
	// different wording — "Which number talked to the most other people in
	// this case?" also resolved to forensics.top_locations — because the
	// grounding check only rejects when semanticFrameGroupHints' CURATED
	// vocabulary recognizes a grouping dimension in the question, and this
	// phrasing ("number", "talked to", "people") isn't covered by that
	// curated list (no "phone"+"number", no "who...call", no
	// contact/caller/callee). Narrowing the check to one phrasing at a time
	// is not a substitute for a grounding signal that actually generalizes
	// across free-form wording. Left disabled again pending that; see
	// NEXUSAI_CONTINUATION.md for both reproductions.
	return resolved, state
}
