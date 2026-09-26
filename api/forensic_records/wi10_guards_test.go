package main

import "testing"

// WI-10. The last five confident-wrong answers were all media/cross-family.
// Each guard below is reconstructed from the SHAPE of a real 2026-09-23
// response, so a regression fails here rather than 40 minutes into a live run.
//
// Every case carries its control: the question that must NOT be withheld. That
// is the part that matters. Two earlier candidates for these same defects were
// rejected precisely because they fired on the control -- see the rejection
// notes in ir_crosscheck.go.

// ---------------------------------------------------------------------------
// DOC-04: the sentence claimed a constraint the executed plan never applied.
// ---------------------------------------------------------------------------

func TestAnswerNamesUnfilteredTargetCatchesDOC04Shape(t *testing.T) {
	// "What does the case notes document say about plate ABC-123?" compiled a
	// bare COUNT over ANPR with no filters, scanned all 218 rows, and stated
	// "There are 218 ANPR sightings involving ABC-123 in this case."
	req := hybridQueryRequest{
		Query:  "What does the case notes document say about plate ABC-123?",
		Target: "ABC-123",
		SourceNative: &SourceNativePlanV1{
			ContractVersion: sourceNativePlanContractV1,
			Measures:        []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT"}},
			Filters:         nil,
		},
	}
	if !answerNamesUnfilteredTarget(req) {
		t.Fatal("a plan that filters on NOTHING must not answer a question about one plate")
	}
}

func TestAnswerNamesUnfilteredTargetSparesDOC01Control(t *testing.T) {
	// DOC-01 is the control and the reason the cruder fix was rejected: same
	// template, same family, same bare-COUNT shape, but the plan really does
	// constrain to the plate, so the sentence is entitled to name it.
	req := hybridQueryRequest{
		Query:  "Search the case documents for mentions of plate MN1367",
		Target: "MN1367",
		SourceNative: &SourceNativePlanV1{
			ContractVersion: sourceNativePlanContractV1,
			Measures:        []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT"}},
			Filters: []SourceNativeFilterV1{{
				FieldID: "anpr.plate_number", Op: "CONTAINS",
				Value: "MN1367", Values: []string{"MN1367"},
			}},
		},
	}
	if answerNamesUnfilteredTarget(req) {
		t.Fatal("DOC-01's plan binds the plate; withholding it loses a correct answer")
	}
}

func TestAnswerNamesUnfilteredTargetIgnoresUnplannedAndUntargetedAnswers(t *testing.T) {
	// No typed plan means the template's own filters are the authority, and
	// this guard has no standing to judge them.
	if answerNamesUnfilteredTarget(hybridQueryRequest{Query: "How many CDR records?", Target: "ABC-123"}) {
		t.Error("without an executed plan there is no plan to check")
	}
	// A question with no target identifier asserts no constraint to violate.
	noTarget := hybridQueryRequest{
		Query:        "How many ANPR sightings are there in total?",
		SourceNative: &SourceNativePlanV1{ContractVersion: sourceNativePlanContractV1},
	}
	if answerNamesUnfilteredTarget(noTarget) {
		t.Error("an unfiltered COUNT is the right answer to an unfiltered question")
	}
}

// ---------------------------------------------------------------------------
// CASE-01 / IMG-04: the size of a bounded page reported as a count.
// ---------------------------------------------------------------------------

func TestUncomputedQuantityAnswerCatchesPageSizeAsCount(t *testing.T) {
	for _, question := range []string{
		"How many records do we have for each record type?", // CASE-01, true answer 8642/2500/750
		"How many images are in this case?",                 // IMG-04, true answer 22
	} {
		resp := hybridQueryResponse{Answer: map[string]any{uncomputedAnswerMarker: true}}
		if !uncomputedQuantityAnswer(hybridQueryRequest{Query: question}, resp) {
			t.Errorf("%q: a page length is not a count", question)
		}
	}
}

func TestUncomputedQuantityAnswerSparesComputedAndNonQuantityAnswers(t *testing.T) {
	// The marker is absent whenever the answer builder computed a headline --
	// which is every one of the 13 CORRECT "how many" answers in the corpus.
	computed := hybridQueryResponse{Answer: map[string]any{}}
	if uncomputedQuantityAnswer(hybridQueryRequest{Query: "How many CDR records do we have in this case?"}, computed) {
		t.Error("a computed count must be delivered, not withheld")
	}
	// A listing question is ANSWERED by a listing; its length is legitimate.
	listing := hybridQueryResponse{Answer: map[string]any{uncomputedAnswerMarker: true}}
	if uncomputedQuantityAnswer(hybridQueryRequest{Query: "Show me the call records for 03001234567"}, listing) {
		t.Error("a listing question is not a quantity question")
	}
}

// ---------------------------------------------------------------------------
// X-01: one family searched, absence asserted for the whole case.
// ---------------------------------------------------------------------------

func noMatchResponse() hybridQueryResponse {
	return hybridQueryResponse{Enterprise: map[string]any{"result_state": "no_match_for_filter"}}
}

func TestSingleFamilyNegativeClaimsCaseScopeCatchesX01Shape(t *testing.T) {
	// "Where does 03001234567 appear across all evidence?" searched CDR
	// locations only and reported absence "in the selected case scope". The
	// gold shows the number appears in this case, in a PDF and a WAV.
	entry := queryTemplateCatalogEntry{ScopeMode: "case_wide", FamilyID: "communications_cdr"}
	if !singleFamilyNegativeClaimsCaseScope(noMatchResponse(), entry) {
		t.Fatal("a one-family search may not assert case-wide absence")
	}
}

func TestSingleFamilyNegativeClaimsCaseScopeSparesLegitimateNegatives(t *testing.T) {
	// A genuinely case-wide template found nothing case-wide. That negative is
	// earned -- 35 of the 62 responses resolve to this shape.
	crossFamily := queryTemplateCatalogEntry{ScopeMode: "case_wide", FamilyID: "case_cross_family"}
	if singleFamilyNegativeClaimsCaseScope(noMatchResponse(), crossFamily) {
		t.Error("a case-wide search is entitled to report case-wide absence")
	}
	// DOC-06 is the model of a correctly scoped negative: it says "in the
	// searched current source scope" and is scored CORRECT.
	docSearch := queryTemplateCatalogEntry{ScopeMode: "case_or_target", FamilyID: "document_intelligence"}
	if singleFamilyNegativeClaimsCaseScope(noMatchResponse(), docSearch) {
		t.Error("DOC-06 already scopes its negative correctly and must not be withheld")
	}
	// Nothing to withhold when the search actually matched something.
	found := hybridQueryResponse{Enterprise: map[string]any{"result_state": "results_present"}}
	cdr := queryTemplateCatalogEntry{ScopeMode: "case_wide", FamilyID: "communications_cdr"}
	if singleFamilyNegativeClaimsCaseScope(found, cdr) {
		t.Error("this guard only governs assertions of ABSENCE")
	}
}

// ---------------------------------------------------------------------------
// The withhold itself must produce a real clarification, not a reworded answer.
// ---------------------------------------------------------------------------

func TestWithholdPostExecutionProducesAClarification(t *testing.T) {
	resp := hybridQueryResponse{
		Intent:     "records",
		Answer:     map[string]any{uncomputedAnswerMarker: true},
		Enterprise: map[string]any{"result_state": "results_present"},
	}
	req := hybridQueryRequest{Query: "How many images are in this case?"}
	if !withholdPostExecution(req, &resp, queryTemplateCatalogEntry{}) {
		t.Fatal("an uncomputed count must be withheld")
	}
	if resp.Intent != intentClarify {
		t.Errorf("intent = %q, want %q", resp.Intent, intentClarify)
	}
	// EXPECTATION NARROWED 2026-09-24. This asserted the blanket
	// `no_verified_plan`, which every withholding cause shared. It is now the
	// SPECIFIC cause, because a shared code cannot classify a recorded run --
	// see the reasoning in `withhold_reason.go`. The property under test is
	// unchanged: withholding carries a typed clarification.
	if resp.Clarification == nil || resp.Clarification.ReasonCode != withholdUncomputedQuantity {
		t.Fatal("withholding must carry a typed clarification naming its cause: the scorer and the UI both read it")
	}
	if !resp.Clarification.ExecutionHeld {
		t.Error("execution_held must be true when an answer is withheld")
	}
	if resp.Clarification.Question == "" {
		t.Error("a clarification with no question tells the analyst nothing")
	}
}

func TestWithholdPostExecutionLeavesGoodAnswersAlone(t *testing.T) {
	resp := hybridQueryResponse{
		Intent:     "records",
		Answer:     map[string]any{},
		Enterprise: map[string]any{"result_state": "results_present"},
	}
	req := hybridQueryRequest{Query: "How many CDR records do we have in this case?"}
	if withholdPostExecution(req, &resp, queryTemplateCatalogEntry{}) {
		t.Fatal("a computed answer must reach the analyst")
	}
	if resp.Intent != "records" || resp.Clarification != nil {
		t.Error("a non-withheld response must be left exactly as it was")
	}
}
