package main

import "testing"

func goalFor(t *testing.T, question string) string {
	t.Helper()
	req := hybridQueryRequest{Query: question, TenantID: "t", CollectionID: "c",
		QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)}}
	return extractSemanticFrame(req).Goal
}

// H1: "how many different handsets" answered 8,642 (all rows) where the truth
// is 4,997 distinct IMEIs, because the distinct vocabulary was only
// "distinct"/"unique".
func TestDifferentAsksForADistinctCount(t *testing.T) {
	if got := goalFor(t, "How many different handsets show up in the call records?"); got != "distinct" {
		t.Fatalf("goal = %q, want distinct", got)
	}
}

// THE CONTROL THAT MATTERS. "Different" modifies a ranking as often as it asks
// for a distinct count, and the distinct case sits ABOVE rank in the switch.
// Unguarded, this question would be handed to S4's distinct narrowing, which
// withdraws the row-count variant and offers no grouping -- turning a ranking
// into a single number.
func TestMostDifferentStaysARanking(t *testing.T) {
	if got := goalFor(t, "Who talked to the most different people?"); got != "rank" {
		t.Fatalf("goal = %q, want rank -- a superlative makes this a ranking, not a "+
			"distinct count", got)
	}
}

// Every superlative must veto, not just "most".
func TestEverySuperlativeVetoesTheDistinctReading(t *testing.T) {
	for _, question := range []string{
		"Which subscriber contacted the most different numbers?",
		"Which camera saw the highest number of different plates?",
		"Which tower handled the greatest number of different devices?",
		"Which number had the fewest different contacts?",
	} {
		if got := goalFor(t, question); got == "distinct" {
			t.Errorf("%q classified distinct; a superlative makes it a ranking", question)
		}
	}
}

// The unambiguous markers are unaffected by the guard -- they mean distinct
// even alongside a superlative, because they name the operation outright.
func TestUnambiguousDistinctMarkersStillFire(t *testing.T) {
	for _, question := range []string{
		"How many unique phone numbers appear as callers in the CDRs?",
		"How many distinct license plates were captured?",
	} {
		if got := goalFor(t, question); got != "distinct" {
			t.Errorf("%q goal = %q, want distinct", question, got)
		}
	}
}

// The shared marker list is the point: one sentence must not be read as a
// ranking by one rule and a distinct count by another.
func TestRankMarkersAreOneList(t *testing.T) {
	if len(semanticRankMarkers) == 0 {
		t.Fatal("the rank markers must be a shared, named list")
	}
	for _, marker := range []string{"most", "top", "highest", "least", "greatest"} {
		if !containsString(semanticRankMarkers, marker) {
			t.Errorf("%q must be in the shared rank marker list", marker)
		}
	}
}
