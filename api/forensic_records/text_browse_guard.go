package main

import (
	"os"
	"strings"
)

// AN UNTARGETED TEXT REQUEST MUST NOT DUMP THE CORPUS.
//
// `derivedTextRelevance` returns `(0, true)` when a question carries no
// identifier and no content term — "nothing to filter on: a browse request".
// Every passage then scores as relevant and is returned WITH ITS FULL TEXT.
//
// Those passages are `observation.raw_text`, `normalized_text`, `text`,
// `roman_urdu_text` and `raw_urdu_text`: the exact five curated fields marked
// PII that `CatalogFields` withholds from every typed plan, proven at every goal
// in media_pii_boundary_test.go. **The retrieval path never consults that
// sensitivity.** The compiler cannot name these fields; the ladder reads them
// straight out of the JSON.
//
// MEASURED 2026-09-26. Reclassifying H2 "What does the audio transcript say?"
// onto this path returned the actual Urdu transcript text in the analyst-facing
// answer. That reclassification was reverted the same day, which makes this
// path LATENT rather than live — and latent only because a misclassification is
// accidentally standing in front of it. This guard is what makes fixing the
// classification safe.
//
// SEARCH WITH A TERM IS NOT THE DEFECT AND MUST NOT BE BROKEN. AUD-01, AUD-02,
// AUD-03, DOC-02, DOC-03 and IMG-01 are CORRECT in the golden 62 and every one
// of them shows a snippet: the analyst supplies an identifier they already hold
// ("is 03001234567 mentioned in any audio?") and the snippet is the CITATION
// that proves the match. Withholding it would destroy six working answers and
// protect nothing — the analyst already had the identifier.
//
// The distinction is SEARCH versus BULK DISCLOSURE, and it is exactly the
// distinction "what does the transcript say?" fails.
//
// WHAT IT DOES, per the accepted §9 B ruling: withhold the TEXT and keep
// everything else — the count, the files, the languages, the timestamps and the
// locators — and say that the text was withheld. An analyst still learns that 11
// segments exist across 7 files in 2 languages, which is a real answer, and is
// told plainly why the words are not shown.
//
// Censused across all three corpora before it was written: exactly ONE question
// is untargeted, H2, and it is the honesty probe that is supposed to refuse.
// Zero structured questions.
//
// DEFAULT ON. Default-off would leave a bulk-PII path open as the shipped state,
// and this project has one precedent for that already: the derived provenance
// label shipped without a switch for the same reason. The switch exists so the
// guard can be turned OFF for a control measurement, not so it can be omitted.
const textBrowseGuardEnv = "FORENSIC_TEXT_BROWSE_GUARD"

func textBrowseGuardEnabled() bool {
	return !strings.EqualFold(strings.TrimSpace(os.Getenv(textBrowseGuardEnv)), "false")
}

// derivedTextRequestIsUntargeted reports whether the request carries nothing to
// match on, so every passage would be returned.
//
// It asks `derivedTextRelevance` itself rather than re-deriving the rule, so
// there is ONE definition of "untargeted" and this cannot drift from the filter
// it guards. A string that shares no content with any real passage scores zero;
// if such a string is still judged RELEVANT, the question filtered on nothing.
func derivedTextRequestIsUntargeted(req hybridQueryRequest) bool {
	// An explicit text query, an exact-phrase request or a supplied target are
	// all targeting, whatever the surrounding prose looks like.
	if req.TextQuery != nil || strings.TrimSpace(req.Target) != "" {
		return false
	}
	if req.StartSeconds != nil || req.EndSeconds != nil {
		// A time window is a filter: "what was said between 0:30 and 0:45" is
		// targeted at a bounded span the analyst named.
		return false
	}
	const sentinel = "zzzz-no-passage-shares-this-token-zzzz"
	score, relevant := derivedTextRelevance(sentinel, req.Query+" "+req.Target)
	return relevant && score == 0
}

// withholdBrowsePassageText strips the passage text from one result while
// leaving its identity, locator and lineage intact.
//
// The text is REMOVED, not blanked to "", and a reason is stated in its place.
// An empty string reads as "this passage has no text", which is a claim about
// the evidence; "withheld" is a claim about the request.
func withholdBrowsePassageText(result map[string]any) {
	result["content"] = ""
	result["preview"] = ""
	result["text_withheld"] = true
	result["text_withheld_reason"] = "PII_UNTARGETED_BROWSE"
	metadata, ok := result["metadata"].(map[string]any)
	if !ok {
		return
	}
	metadata["raw_text"] = ""
	metadata["search_text"] = ""
	metadata["text_withheld"] = true
	metadata["text_withheld_reason"] = "PII_UNTARGETED_BROWSE"
	// The contributing observations carry their own copies of the passage.
	for _, observation := range mapsFromAny(metadata["contributing_observations"]) {
		for _, key := range []string{"passage_text", "text", "raw_text", "preview", "content"} {
			if _, present := observation[key]; present {
				observation[key] = ""
			}
		}
		observation["text_withheld"] = true
	}
}

// textBrowseWithheldLimitation is what the analyst is told. It names what was
// withheld, why, and what they CAN still see, so the response is an answer with
// a stated boundary rather than a refusal.
const textBrowseWithheldLimitation = "Transcript and OCR text is withheld for this request: it names no term, " +
	"identifier or time range to search for, so answering it would disclose every retained passage in full. " +
	"The passage count, source files, languages, timestamps and citation locators are shown and are unaffected. " +
	"Name a term, an identifier or a time range and the matching passage will be quoted with its citation."
