package main

import (
	"testing"

	"github.com/mudler/LocalAI/pkg/forensicrequest"
)

// CENSUS. Which questions never reach the compiler because the request
// classifier sends them somewhere terminal?
//
// Four do, and 2026-09-26 an attempt was made to fix three of them. IT WAS
// REVERTED THE SAME DAY BECAUSE IT CAUSED A PII DISCLOSURE: reclassifying them
// did not route them to the COMPILER, it routed them to `audio_transcript_search`,
// a retrieval template on the ladder, which returned actual PII transcript text
// for H2 and a FALSE ABSENCE for M15.
//
// The census is kept because it is the instrument that will measure the fix
// when the ladder stops claiming these questions. Reasoning:
// semantic_candidate_ranking.go and pkg/forensicrequest/classification.go.
func TestRequestClassCensus(t *testing.T) {
	paths := []string{
		"../../evaluation/golden_questions_v2.json",
		"../../evaluation/holdout_questions_v1.json",
		"../../evaluation/holdout_media_v1.json",
	}
	total := 0
	for _, path := range paths {
		diverted := []string{}
		for _, q := range loadRoutingQuestions(t, path) {
			class := forensicrequest.Classify(forensicrequest.Input{Text: q.Q})
			if class == forensicrequest.GovernedAnalysis {
				continue
			}
			diverted = append(diverted, string(class)+"  "+q.ID+"\n        "+q.Q)
		}
		t.Logf("")
		t.Logf("%s : %d question(s) NEVER REACH THE COMPILER", path, len(diverted))
		for _, line := range diverted {
			t.Logf("   %s", line)
		}
		total += len(diverted)
	}
	t.Logf("")
	t.Logf("TOTAL DIVERTED FROM THE EVIDENCE PATH: %d", total)
	t.Logf("They are VISIBLE now rather than silent (terminal_enterprise_test.go);")
	t.Logf("routing them correctly needs the ladder fixed first.")
}

// THE H4 SUPERLATIVE FIX OF 2026-09-25 STILL HOLDS, and the reverted temporal
// words are asserted ABSENT so a future change re-adds them deliberately rather
// than by accident.
func TestClassifierSuperlativeBoundary(t *testing.T) {
	governed := forensicrequest.Classify(forensicrequest.Input{
		Text: "What is the largest network volume on any call record?"})
	if governed != forensicrequest.GovernedAnalysis {
		t.Errorf("the H4 superlative fix regressed: got %s", governed)
	}
	// Out of scope, and it must stay that way.
	if got := forensicrequest.Classify(forensicrequest.Input{
		Text: "What is the suspect's blood type?"}); got != forensicrequest.GeneralDomainKnowledge {
		t.Errorf("NEG-03 classified %s, want GENERAL_DOMAIN_KNOWLEDGE", got)
	}
	// REVERTED, deliberately. Re-adding these routes M15 to a retrieval template
	// that answers a FALSE ABSENCE, and H2 to one that returns PII transcript
	// text. Re-add only with the ladder fixed and a control.
	for _, q := range []string{
		"What is the latest end offset in the audio transcripts?",
		"What is the earliest offset at which a plate group was first seen?",
	} {
		if got := forensicrequest.Classify(forensicrequest.Input{Text: q}); got == forensicrequest.GovernedAnalysis {
			t.Errorf("temporal superlative re-enabled without the ladder fix: %q -> %s", q, got)
		}
	}
}
