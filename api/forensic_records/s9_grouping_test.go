package main

import (
	"strings"
	"testing"
)

// The rule under test is "a plain count must not acquire a grouping it was
// never asked for". It is D2's guard, and it was rejecting CDR-05's correct
// breakdown. Every case below carries its control: the grouping that must STILL
// be refused. That is the half that matters -- a guard that stops refusing
// correct work is only an improvement if it still refuses the wrong work.

func groupingFrame(t *testing.T, question string) (SemanticFrameV1, string) {
	t.Helper()
	req := hybridQueryRequest{Query: question, TenantID: "t", CollectionID: "c",
		QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)}}
	return extractSemanticFrame(req), question
}

func countGroupedBy(fields ...string) *SourceNativePlanV1 {
	return &SourceNativePlanV1{
		ContractVersion: sourceNativePlanContractV1,
		GroupFields:     fields,
		Measures:        []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT"}},
	}
}

// CDR-05. The generator's plan was right and S9 refused it.
func TestScalarGuardAcceptsAGroupingTheQuestionNames(t *testing.T) {
	frame, question := groupingFrame(t, "How many calls of each type are there?")
	if err := verifySourceNativePlanShape(frame, question, countGroupedBy("cdr.call_type")); err != nil {
		t.Fatalf("the analyst asked for a breakdown by type; it must not be refused: %v", err)
	}
}

// D2's signature, and the reason the rule exists: a grouping produced by
// scoring the whole question against the catalogue, on a field the analyst
// never mentioned. It must still be refused.
func TestScalarGuardStillRefusesAGroupingNobodyNamed(t *testing.T) {
	frame, question := groupingFrame(t, "How many CDR records do we have in this case?")
	err := verifySourceNativePlanShape(frame, question, countGroupedBy("cdr.cell_site_id"))
	if err == nil {
		t.Fatal("nothing in this question asks for a breakdown by cell site")
	}
	if !strings.Contains(err.Error(), "scalar question compiled a grouping") {
		t.Fatalf("unexpected rejection: %v", err)
	}
}

// A partial match is refused. Naming one of two dimensions still leaves a
// grouping nobody asked for, and the safe reading of "partly asked for" is no.
func TestScalarGuardRefusesAPartiallyNamedGrouping(t *testing.T) {
	frame, question := groupingFrame(t, "How many calls of each type are there?")
	if err := verifySourceNativePlanShape(frame, question, countGroupedBy("cdr.call_type", "cdr.cell_site_id")); err == nil {
		t.Fatal("cell site was never named; the grouping is not fully asked for")
	}
}

// An unknown field cannot be evidence that the analyst named it.
func TestScalarGuardRefusesAGroupingOnAnUncuratedField(t *testing.T) {
	frame, question := groupingFrame(t, "How many calls of each type are there?")
	if err := verifySourceNativePlanShape(frame, question, countGroupedBy("fld_b4d5a2")); err == nil {
		t.Fatal("a hashed uncurated field has no display name to have been named by")
	}
}

func TestQuestionNamesEveryGroupedFieldIsConservative(t *testing.T) {
	// No fields is not "all named" -- it is nothing to check.
	if questionNamesEveryGroupedField("How many calls of each type are there?", nil) {
		t.Error("an empty grouping must not report as named")
	}
	// A synonym counts: cdr.call_type declares "type".
	if !questionNamesEveryGroupedField("How many calls of each type are there?", []string{"cdr.call_type"}) {
		t.Error("`type` is a declared synonym of Call type")
	}
	// A field the question never mentions does not.
	if questionNamesEveryGroupedField("How many CDR records are there?", []string{"cdr.call_type"}) {
		t.Error("this question names no dimension at all")
	}
}

// The rank and breakdown branches are untouched by this change.
func TestRankAndBreakdownBranchesAreUnchanged(t *testing.T) {
	frame, question := groupingFrame(t, "Which phone number made the most calls?")
	if s9QuestionShape(frame, question) == "rank" {
		unordered := countGroupedBy("cdr.msisdn")
		if err := verifySourceNativePlanShape(frame, question, unordered); err == nil {
			t.Error("a ranking with no ordering still has an arbitrary top row")
		}
	}
}
