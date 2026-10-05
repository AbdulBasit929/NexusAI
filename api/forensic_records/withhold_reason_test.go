package main

import (
	"strings"
	"testing"
)

// The corpus shapes these are reconstructed from are the real 2026-09-24
// clarifications. Offline: no model, no database, no redeploy.

func scopedWithholdRequest(query string) hybridQueryRequest {
	return hybridQueryRequest{
		Query: query, TenantID: "t", CollectionID: "c",
		QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)},
	}
}

// ---------------------------------------------------------------------------
// The five causes must be distinguishable from the recorded response alone.
// ---------------------------------------------------------------------------

// A misroute must say it is a misroute. DOC-01 and DOC-04 both reach the
// pre-execution gate with NO verifiable plan AND a structured template
// answering a document question. Before the split, the general cause won the
// short-circuit and both were told to "name the field, the grouping or the time
// range" -- advice that cannot fix either question, because nothing about a
// field or a time range makes the system read the documents.
func TestMediaMisrouteIsReportedAsAMisrouteNotAsAMissingField(t *testing.T) {
	req := scopedWithholdRequest("What does the case notes document say about plate ABC-123?")
	req.SourceNative = &SourceNativePlanV1{ContractVersion: sourceNativePlanContractV1}

	reason := preExecutionWithhold(req, "canonical_records")
	if reason == nil {
		t.Fatal("a media question answered by a structured template must be withheld")
	}
	if reason.Code != withholdMediaMisroute {
		t.Fatalf("code = %q, want %q", reason.Code, withholdMediaMisroute)
	}
	// It must name what the analyst asked about...
	if !strings.Contains(reason.Question, "documents") {
		t.Errorf("the analyst asked about documents; the sentence must say so: %q", reason.Question)
	}
	// ...and it must NOT hand back the generic advice, which is the defect.
	if strings.Contains(reason.Question, "Name the field, the grouping or the time") {
		t.Errorf("a misroute must not be reported as a missing field: %q", reason.Question)
	}
	if !strings.Contains(reason.Detail, "answered_family=") {
		t.Errorf("the audit must record which family answered: %q", reason.Detail)
	}
}

// DOC-01's shape: a SOUND plan, correctly filtered, computed over the wrong
// evidence. The guard fires and the reason must still be the misroute, because
// the plan being verifiable is exactly what made the wrong answer convincing.
func TestVerifiedPlanOverWrongEvidenceStillReportsTheMisroute(t *testing.T) {
	req := scopedWithholdRequest("Search the case documents for mentions of plate MN1367")
	req.Target = "MN1367"
	req.SourceNative = &SourceNativePlanV1{
		ContractVersion: sourceNativePlanContractV1,
		Measures:        []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT"}},
		Filters:         []SourceNativeFilterV1{{Value: "MN1367"}},
	}
	reason := preExecutionWithhold(req, "canonical_records")
	if reason == nil || reason.Code != withholdMediaMisroute {
		t.Fatalf("DOC-01 must report a misroute, got %+v", reason)
	}
}

// The WI-10 unfiltered-target guard gets its own voice: what is missing is the
// RESTRICTION, not a field name.
func TestUnfilteredTargetAsksForTheRestrictionItIsMissing(t *testing.T) {
	// No media noun, so the misroute guard does not claim this one first.
	req := scopedWithholdRequest("How many sightings involve ABC-123?")
	req.Target = "ABC-123"
	req.SourceNative = &SourceNativePlanV1{
		ContractVersion: sourceNativePlanContractV1,
		Measures:        []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT"}},
		Filters:         nil,
	}
	reason := preExecutionWithhold(req, "canonical_records")
	if reason == nil {
		t.Fatal("an answer naming a target the plan never filtered on must be withheld")
	}
	if reason.Code != withholdTargetNotFiltered {
		t.Fatalf("code = %q, want %q", reason.Code, withholdTargetNotFiltered)
	}
	if !strings.Contains(reason.Question, "ABC-123") {
		t.Errorf("the sentence must name the target it could not restrict to: %q", reason.Question)
	}
	if !strings.Contains(reason.Detail, "filters=0") {
		t.Errorf("the audit must record that the plan filtered on nothing: %q", reason.Detail)
	}
}

// ---------------------------------------------------------------------------
// What must NOT change.
// ---------------------------------------------------------------------------

// The point of the split is the EXPLANATION, never the outcome. A question
// nothing objects to must still pass the gate untouched.
func TestAStructuredQuestionWithNoObjectionIsNotWithheld(t *testing.T) {
	req := scopedWithholdRequest("How many CDR records do we have in this case?")
	req.SourceNative = &SourceNativePlanV1{ContractVersion: sourceNativePlanContractV1}
	if reason := preExecutionWithhold(req, "canonical_records"); reason != nil {
		t.Fatalf("nothing objects to this question; it must be answered, got %+v", reason)
	}
}

// A media question answered by its OWN media executor is correct routing, and
// reordering the guards must not have turned it into a misroute.
func TestMediaExecutorAnsweringAMediaQuestionIsNotAMisroute(t *testing.T) {
	req := scopedWithholdRequest("How many images are in this case?")
	req.SourceNative = &SourceNativePlanV1{ContractVersion: sourceNativePlanContractV1}
	if reason := preExecutionWithhold(req, "image_metadata"); reason != nil {
		t.Fatalf("image_metadata is the right executor for an image question, got %+v", reason)
	}
}

// Every cause must carry a code, and the codes must be distinct -- otherwise a
// recorded run still cannot be classified, which is the whole point.
func TestEveryWithholdCodeIsDistinct(t *testing.T) {
	codes := []string{
		withholdNoVerifiedPlan, withholdMediaMisroute, withholdTargetNotFiltered,
		withholdUncomputedQuantity, withholdSingleFamilyNegative,
		withholdConditionNotApplied, withholdRelationshipNotComputed, withholdTextSearchNotAbsence,
	}
	seen := map[string]bool{}
	for _, code := range codes {
		if strings.TrimSpace(code) == "" {
			t.Fatal("a withhold cause with no code cannot be classified from a recorded run")
		}
		if seen[code] {
			t.Fatalf("duplicate withhold code %q", code)
		}
		seen[code] = true
	}
}

// The diagnostic detail is written to every audited response. It records
// planner STATE -- field counts, binding outcomes -- and must never carry the
// question's literal values, which belong in the plan.
func TestWithholdDetailCarriesPlannerStateNotEvidenceValues(t *testing.T) {
	req := scopedWithholdRequest("How many calls did 923001110001 make?")
	req.Target = "923001110001"
	detail := noVerifiedPlanDetail(req)
	if strings.Contains(detail, "923001110001") {
		t.Fatalf("a withhold detail must not carry evidence values: %q", detail)
	}
	if !strings.Contains(detail, "family=") {
		t.Errorf("the detail must record which family resolved, if any: %q", detail)
	}
	// And the WI-10 detail is subject to the same rule.
	unfiltered := targetNotFilteredReason(req)
	if strings.Contains(unfiltered.Detail, "923001110001") {
		t.Fatalf("a withhold detail must not carry evidence values: %q", unfiltered.Detail)
	}
}

// applyWithhold is the one place a response becomes a clarification. The route
// label is depended on by the live evaluation harness and every recorded run,
// so the split must leave it exactly where it was.
func TestApplyWithholdKeepsTheEstablishedRouteLabel(t *testing.T) {
	req := scopedWithholdRequest("How many images are in this case?")
	audit := &SemanticPlannerAuditV1{}
	req.SemanticPlannerAudit = audit
	resp := hybridQueryResponse{Answer: map[string]any{}}

	applyWithhold(req, &resp, withholdReason{
		Code: withholdUncomputedQuantity, Question: "q", Guardrail: "g", Detail: "d",
	})

	if len(resp.Route) != 1 || resp.Route[0] != "verified_only_withheld" {
		t.Fatalf("route = %v, want [verified_only_withheld]", resp.Route)
	}
	if resp.Intent != intentClarify {
		t.Fatalf("intent = %q, want %q", resp.Intent, intentClarify)
	}
	if resp.Clarification == nil || resp.Clarification.ReasonCode != withholdUncomputedQuantity {
		t.Fatalf("the specific cause must reach the contract, got %+v", resp.Clarification)
	}
	if !resp.Clarification.ExecutionHeld {
		t.Error("a withheld answer must say execution was held")
	}
	if !audit.VerifiedOnlyWithheld || audit.WithholdCode != withholdUncomputedQuantity || audit.WithholdDetail != "d" {
		t.Fatalf("the audit must record the cause and its detail, got %+v", audit)
	}
}
