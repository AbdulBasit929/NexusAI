package main

import (
	"strings"
	"testing"
)

// The CONSTRAINT_APPLIED allowlist decides which filter values a plan may carry.
// When it cannot see a literal the analyst plainly supplied, it rejects CORRECT
// plans -- measured 2026-09-22 as the largest single failure class in the
// golden-IR run (9 of 62), and not a model failure at all.

func TestLiteralExtractionSeesIdentifiers(t *testing.T) {
	cases := []struct {
		question string
		want     string
	}{
		{"How many times was plate LHR-2026 seen?", "LHR-2026"},
		{"When was LHR-2026 first and last seen?", "LHR-2026"},
		{"How many calls did 923001110001 make?", "923001110001"},
		{"Which document mentions contact number 03001234567?", "03001234567"},
		{"Sessions to 10.0.0.5 yesterday", "10.0.0.5"},
	}
	for _, tc := range cases {
		got := semanticSourceNativeLiteralValues(hybridQueryRequest{Query: tc.question})
		if !containsString(got, tc.want) {
			t.Errorf("%q\n  did not supply %q; got %v", tc.question, tc.want, got)
		}
	}
}

// The hyphen in a plate is a separator, not a minus sign. The old rule produced
// "-2026", so the ONLY value a plate question could legally carry was nonsense.
func TestLiteralExtractionDoesNotInventNegativeNumbers(t *testing.T) {
	got := semanticSourceNativeLiteralValues(hybridQueryRequest{Query: "How many times was plate LHR-2026 seen?"})
	for _, value := range got {
		if strings.HasPrefix(value, "-") {
			t.Errorf("extracted %q: a separator was read as a minus sign; got %v", value, got)
		}
	}
}

// A value the curated layer DECLARES is supplied by the analyst when they name
// it. subscriber.status declares ACTIVE with synonym "active".
func TestLiteralExtractionSeesCuratedValues(t *testing.T) {
	got := semanticSourceNativeLiteralValues(hybridQueryRequest{Query: "How many subscribers are active?"})
	if !containsString(got, "ACTIVE") {
		t.Fatalf("curated value ACTIVE not supplied for an 'active' question; got %v", got)
	}
}

// "active" must not be found inside "inactive", and a value belonging to
// another family must not widen this family's allowlist.
func TestLiteralExtractionStaysPrecise(t *testing.T) {
	got := semanticSourceNativeLiteralValues(hybridQueryRequest{Query: "How many subscribers are inactive?"})
	if containsString(got, "ACTIVE") && !containsString(got, "INACTIVE") {
		t.Fatalf("matched ACTIVE inside 'inactive'; got %v", got)
	}
	// A CDR call-type value must not become supplied by a subscriber question.
	if containsString(got, "VOICE") {
		t.Fatalf("another family's declared value leaked into the allowlist; got %v", got)
	}
}

// Nothing may be supplied by a question that names no literal at all -- that is
// what makes an invented filter detectable.
func TestLiteralExtractionSuppliesNothingWhenNothingIsNamed(t *testing.T) {
	got := semanticSourceNativeLiteralValues(hybridQueryRequest{Query: "Which phone number made the most calls?"})
	if len(got) != 0 {
		t.Fatalf("no literal is present in this question; got %v", got)
	}
}
