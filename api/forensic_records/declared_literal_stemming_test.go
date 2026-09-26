package main

import "testing"

func declared(t *testing.T, question string) []string {
	t.Helper()
	return semanticLayerDeclaredLiterals(question)
}

// H7: "How many server errors are in the access log?" answered 1,000 -- the
// whole family -- where the truth is 46. The curation was already correct:
// access_log.status declares the value "500" with the synonym "server error".
// The matcher compared RAW words, so the question's "errorS" never met the
// synonym's "error".
func TestMultiWordSynonymsMatchAcrossPlurals(t *testing.T) {
	got := declared(t, "How many server errors are in the access log?")
	if !containsString(got, "500") {
		t.Fatalf("declared = %v, want 500 -- the layer says a server error IS status 500", got)
	}
}

// THE CONTROL, and it is why stemming is confined to multi-word synonyms.
// "calls" stems to "call", which IS the value CALL of cdr.call_type. Stemming
// the value-name match made five ordinary questions start supplying a
// call-type filter nobody asked for.
func TestOrdinaryCallQuestionsSupplyNoCallTypeFilter(t *testing.T) {
	for _, question := range []string{
		"How many calls did 923001110001 make?",
		"Which phone number made the most calls?",
		"How many calls of each type are there?",
		"How many calls did 03999999999 make?",
	} {
		if got := declared(t, question); containsString(got, "CALL") {
			t.Errorf("%q supplied CALL as a literal (%v); \"calls\" is the noun for CDR rows, "+
				"not a request to filter call_type", question, got)
		}
	}
}

// A single word still only supplies a value when it NAMES that value, so a
// question that says "call" in the type sense keeps working.
func TestSingleWordValueNamingIsUnchanged(t *testing.T) {
	if got := declared(t, "How many VoLTE calls are there?"); !containsString(got, "VOLTE") {
		t.Errorf("VOLTE names the value outright; declared = %v", got)
	}
	if got := declared(t, "How many subscribers are currently active?"); !containsString(got, "ACTIVE") {
		t.Errorf("ACTIVE names the value outright; declared = %v", got)
	}
}

// The loose-synonym protection the original rule existed for must survive:
// cdr.direction declares OUTGOING with the one-word synonym "made".
func TestLooseOneWordSynonymsStillDoNotSupply(t *testing.T) {
	if got := declared(t, "Which phone number made the most calls?"); containsString(got, "OUTGOING") {
		t.Errorf("\"made\" is a loose one-word synonym of OUTGOING and must not supply it: %v", got)
	}
}
