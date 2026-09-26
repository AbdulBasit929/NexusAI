package main

import (
	"strings"
	"testing"
)

// WI-12. Every audio question in the corpus looked like broken retrieval.
// Retrieval was working the whole time: the match was found, cited and
// located, and then discarded by the completeness caveat before the analyst
// saw it.
//
// The invariant is simple and it is a truthfulness invariant in BOTH
// directions: a search that found something must say what it found, and a
// search that found nothing must not imply that it did.

// Built from the real 2026-09-23 response to
// `Find the exact phrase "coconut sugar" in the audio transcripts`.
func coconutSugarResponse() hybridQueryResponse {
	return hybridQueryResponse{
		Template: "audio_transcript_search",
		Evidence: map[string]any{
			"result_state": "SEARCH_INCOMPLETE",
			"results": []map[string]any{{
				"content": "especially Japanese coconut sugar, and various aromatic spices.",
				"metadata": map[string]any{
					"artifact_type": "forensics.audio-timestamp-segment/v1",
					"citation_locator": map[string]any{
						"source_file":   "fleurs-en_us-validation-row-01.wav",
						"start_seconds": 11.28,
						"end_seconds":   15.08,
					},
				},
			}},
		},
		Answer:  map[string]any{},
		Records: map[string]any{},
	}
}

func TestIncompleteSearchStillStatesWhatItFound(t *testing.T) {
	resp := coconutSugarResponse()
	req := hybridQueryRequest{Query: "Which recording mentions coconut sugar and at what time?"}
	answer := enterpriseExecutiveAnswer(req, resp, nil)

	// AUD-02 asks WHICH recording and AT WHAT TIME. Both are in the locator.
	for _, want := range []string{"fleurs-en_us-validation-row-01.wav", "coconut sugar", "11.28"} {
		if !strings.Contains(answer, want) {
			t.Errorf("answer omits %q -- the analyst asked for it and it is in the citation\n got: %s", want, answer)
		}
	}
	if strings.Contains(answer, "The text search is incomplete") {
		t.Errorf("a found, cited match was discarded by the completeness caveat: %s", answer)
	}
}

// The caveat is still exactly right when nothing was found. Removing it there
// would let an empty search imply absence, which is the failure X-01 was
// withheld for.
func TestIncompleteSearchWithNoResultsStillDeclinesToAssertAbsence(t *testing.T) {
	resp := coconutSugarResponse()
	resp.Evidence["results"] = []map[string]any{}
	answer := enterpriseExecutiveAnswer(hybridQueryRequest{Query: "Which recording mentions coconut sugar?"}, resp, nil)
	if !strings.Contains(answer, "absence of further matches has not been established") {
		t.Fatalf("an empty incomplete search must not imply absence; got: %s", answer)
	}
}

// A segment with no usable start time contributes no time claim rather than a
// guessed one -- but the recording it came from is still worth stating.
func TestTranscriptAnswerOmitsTimeWhenTheLocatorHasNone(t *testing.T) {
	resp := coconutSugarResponse()
	results := resp.Evidence["results"].([]map[string]any)
	metadata := results[0]["metadata"].(map[string]any)
	metadata["citation_locator"] = map[string]any{"source_file": "fleurs-en_us-validation-row-01.wav"}

	answer := transcriptExecutiveAnswer(hybridQueryRequest{}, resp)
	if !strings.Contains(answer, "fleurs-en_us-validation-row-01.wav") {
		t.Errorf("the recording is known and must still be named: %s", answer)
	}
	if strings.Contains(answer, " at ") {
		t.Errorf("no start time was recorded, so none may be claimed: %s", answer)
	}
}

// Without a source file there is no citable recording to name, so this builds
// nothing and the caller falls back rather than emitting a half-claim.
func TestTranscriptAnswerDeclinesWithoutACitableSource(t *testing.T) {
	resp := coconutSugarResponse()
	results := resp.Evidence["results"].([]map[string]any)
	results[0]["metadata"] = map[string]any{"citation_locator": map[string]any{"start_seconds": 11.28}}
	if answer := transcriptExecutiveAnswer(hybridQueryRequest{}, resp); answer != "" {
		t.Errorf("a match with no citable recording must not be stated as one: %s", answer)
	}
}

// Several recordings matching is a different fact from one, and the analyst
// needs to know the first is not the only one.
func TestTranscriptAnswerReportsWhenSeveralRecordingsMatch(t *testing.T) {
	resp := coconutSugarResponse()
	results := resp.Evidence["results"].([]map[string]any)
	resp.Evidence["results"] = append(results, map[string]any{
		"content": "kal 03001234567 at 1035",
		"metadata": map[string]any{
			"citation_locator": map[string]any{
				"source_file": "urdu-english-identifier.wav", "start_seconds": 1.0,
			},
		},
	})
	answer := transcriptExecutiveAnswer(hybridQueryRequest{}, resp)
	if !strings.Contains(answer, "2 recordings") {
		t.Errorf("two recordings matched and the answer must say so: %s", answer)
	}
}

// A citation that does not carry the claim is not a citation. Three segments
// of one recording matched AUD-02; the answer must cite the one containing
// the phrase the analyst asked about, not whichever came back first.
func TestTranscriptAnswerCitesTheSegmentThatCarriesTheClaim(t *testing.T) {
	seg := func(text, file string, start float64) map[string]any {
		return map[string]any{"content": text, "metadata": map[string]any{
			"citation_locator": map[string]any{"source_file": file, "start_seconds": start},
		}}
	}
	resp := hybridQueryResponse{
		Template: "audio_transcript_search",
		Answer:   map[string]any{},
		Evidence: map[string]any{
			"result_state": "SEARCH_INCOMPLETE",
			"results": []map[string]any{
				seg("Now widely available throughout the archipelago, Japanese cuisines features an array of simply", "fleurs-en_us-validation-row-01.wav", 0),
				seg("seasoned dishes, the predominant flavorings, the Japanese favor being peanut chilies, sugar,", "fleurs-en_us-validation-row-01.wav", 5.56),
				seg("especially Japanese coconut sugar, and various aromatic spices.", "fleurs-en_us-validation-row-01.wav", 11.28),
			},
		},
	}
	req := hybridQueryRequest{Query: "Which recording mentions coconut sugar and at what time?"}
	answer := transcriptExecutiveAnswer(req, resp)
	if !strings.Contains(answer, "coconut sugar") {
		t.Errorf("cited a segment that does not contain the phrase asked about: %s", answer)
	}
	if !strings.Contains(answer, "11.28") {
		t.Errorf("reported the wrong timestamp; want the matching segment's 11.28: %s", answer)
	}
}
