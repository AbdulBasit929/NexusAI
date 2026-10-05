package main

import (
	"os"
	"strings"
)

// GOAL TAXONOMY — two slices that empty the `lookup` bucket of questions the
// switch can actually recognise.
//
// `lookup` is the DEFAULT, and the default means UNRECOGNISED: it skips shape
// verification entirely and, for a media question, the algebra goal gate
// refuses it so it never reaches the compiler at all. Seven media questions sit
// there for no reason other than that this switch has no case for them.
//
// TWO PRIOR ATTEMPTS AT THIS SWITCH COST REAL ANSWERS, and both are why these
// slices are narrow, censused and separately switched:
//
//   - WI-31 put "breakdown" inside the `aggregate` case. ACC-02 went CORRECT ->
//     CONFIDENT-WRONG, answering "there are 1,000 access log entries" where a
//     breakdown was asked for, because `aggregate` with no group hints resolves
//     to shape `scalar` and therefore ASSERTS the answer is one number.
//   - "longest"/"shortest" were added alongside it and reverted with it,
//     un-measured, because the declared threshold reverts the change and not
//     the half that looks guilty.
//
// Censused across all three corpora BEFORE being wired
// (goal_taxonomy_slice_test.go): slice A moves 2 questions and disturbs 0,
// slice B moves 5 and disturbs 0.
const goalTaxonomySlicesEnv = "FORENSIC_GOAL_TAXONOMY_SLICES"

func goalTaxonomySlicesEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(goalTaxonomySlicesEnv)), "true") //nolint:forbidigo // forensic records product API reads its feature switches from the environment by design (docker compose); not LocalAI server config
}

// goalSliceATemporalSuperlative reports whether the question asks for the
// EARLIEST or LATEST of something.
//
// These are superlatives exactly as `largest` and `smallest` are, and
// `semanticRankMarkers` already carries the rest of that family — they were
// simply never added on the time axis. `semanticFrameMeasure` already maps them
// to MIN/MAX, so today the MEASURE is right while the GOAL falls to `lookup`
// and the shape is never verified.
//
// Kept OUT of `semanticRankMarkers` itself and expressed here instead, because
// that slice is also consulted by the `distinct` guard for "different": adding
// words to the shared list would change two rules at once, and this project has
// repeatedly paid for a single edit that moved more than one thing.
func goalSliceATemporalSuperlative(terms map[string]bool) bool {
	return semanticTermsContain(terms, "earliest", "latest")
}

// goalSliceBDistinctValues reports whether the question asks WHICH VALUES of a
// field were observed — a DISTINCT question, not a lookup.
//
// "What script families were detected?" and "Which model produced the face
// vectors?" ask for the set of values that occur. That is `distinct`, and the
// curated fields involved are all COUNT_DISTINCT-capable.
//
// THE EXCLUSIONS ARE THE RULE. Each one keeps a working goal working:
//
//   - a RETRIEVAL verb (mention, contain, say, find, search, phrase, match)
//     makes it `search`. "Which document mentions 03001234567?" is DOC-02 and
//     is CORRECT today; capturing it would be WI-31 in a new place.
//   - a SUPERLATIVE makes it `rank`, and the rank case must keep those.
//   - an AGGREGATE marker makes it `aggregate`.
//
// Verbs are matched on their STEMMED forms because `semanticFrameTerms` stems
// before matching: "detected" arrives as `detect`, "produced" as `produc`.
// Matching the surface form captured only two of the five and is the H7 defect
// exactly — one matcher comparing raw words while everything else stemmed.
func goalSliceBDistinctValues(question string, terms map[string]bool) bool {
	trimmed := strings.ToLower(strings.TrimSpace(question))
	if !strings.HasPrefix(trimmed, "which") && !strings.HasPrefix(trimmed, "what") {
		return false
	}
	if semanticTermsContain(terms, "mention", "contain", "say", "find", "search", "phrase", "match") {
		return false
	}
	if semanticTermsContain(terms, semanticRankMarkers...) {
		return false
	}
	if semanticTermsContain(terms, "many", "much", "average", "sum", "count", "total") {
		return false
	}
	return semanticTermsContain(terms, "detect", "produc", "used", "recognis", "recogniz", "read", "emit")
}
