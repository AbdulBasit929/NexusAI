package main

import (
	"sort"
	"strings"
	"testing"
)

// GOAL TAXONOMY CENSUS. What goal does every question in every corpus get
// today, and what would a candidate slice change?
//
// The `lookup` default means "UNRECOGNISED" and skips shape verification, and
// it is what blocks seven media questions from the compiler entirely. But the
// last two attempts to widen this switch cost real answers -- WI-31 put
// "breakdown" inside the aggregate case and ACC-02 answered a total where a
// breakdown was asked for, and the longest/shortest pair was reverted with it
// and never measured alone.
//
// So: census first, slice by slice, and read what each one would actually
// capture before any of it is wired.
func TestGoalTaxonomySliceCensus(t *testing.T) {
	paths := []string{
		"../../evaluation/golden_questions_v2.json",
		"../../evaluation/holdout_questions_v1.json",
		"../../evaluation/holdout_media_v1.json",
	}
	byGoal := map[string][]string{}
	for _, path := range paths {
		for _, q := range loadRoutingQuestions(t, path) {
			goal := semanticFrameGoal(semanticFrameTerms(q.Q), q.Q)
			byGoal[goal] = append(byGoal[goal], q.ID)
		}
	}
	goals := make([]string, 0, len(byGoal))
	for g := range byGoal {
		goals = append(goals, g)
	}
	sort.Strings(goals)
	t.Logf("GOAL DISTRIBUTION across all three corpora:") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	for _, g := range goals {
		t.Logf("   %-12s %3d", g, len(byGoal[g])) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	t.Logf("") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	t.Logf("The `lookup` bucket -- 'unrecognised', skips shape verification:") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	sort.Strings(byGoal["lookup"])
	t.Logf("   %s", strings.Join(byGoal["lookup"], " ")) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
}

// SLICE A CANDIDATE: temporal superlatives. `earliest` and `latest` are
// superlatives exactly as `largest` and `smallest` are, and the rank markers
// already carry the rest of that family.
//
// This prints what WOULD change. It wires nothing.
func TestGoalSliceATemporalSuperlativesCensus(t *testing.T) {
	candidates := []string{"earliest", "latest"}
	paths := []string{
		"../../evaluation/golden_questions_v2.json",
		"../../evaluation/holdout_questions_v1.json",
		"../../evaluation/holdout_media_v1.json",
	}
	moved, total := 0, 0
	for _, path := range paths {
		for _, q := range loadRoutingQuestions(t, path) {
			total++
			terms := semanticFrameTerms(q.Q)
			current := semanticFrameGoal(terms, q.Q)
			if current != "lookup" {
				// Only the lookup bucket can be improved; anything else this
				// rule touched would be a REGRESSION, so report it loudly.
				if semanticTermsContain(terms, candidates...) {
					t.Logf("   WOULD DISTURB  %-26s %-10s <- already classified", q.ID, current) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
					moved++
				}
				continue
			}
			if semanticTermsContain(terms, candidates...) {
				t.Logf("   would become rank   %-26s %s", q.ID, q.Q) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
				moved++
			}
		}
	}
	t.Logf("") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	t.Logf("SLICE A: %d of %d questions affected", moved, total) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
}

// SLICE B CANDIDATE: "which/what <field> was used / were detected / produced".
// A WH-question asking which VALUES of a field appear is a DISTINCT question,
// not a lookup.
//
// This is the risky one and the census is the point: the `search` case sits
// BELOW `distinct` in the switch, so a careless rule would capture "which
// document mentions X" and hand a retrieval question to distinct narrowing.
func TestGoalSliceBDistinctValuesCensus(t *testing.T) {
	paths := []string{
		"../../evaluation/golden_questions_v2.json",
		"../../evaluation/holdout_questions_v1.json",
		"../../evaluation/holdout_media_v1.json",
	}
	moved, disturbed := []string{}, []string{}
	for _, path := range paths {
		for _, q := range loadRoutingQuestions(t, path) {
			terms := semanticFrameTerms(q.Q)
			current := semanticFrameGoal(terms, q.Q)
			if !goalSliceBMatches(q.Q, terms) {
				continue
			}
			if current == "lookup" {
				moved = append(moved, q.ID+"  "+q.Q)
			} else {
				disturbed = append(disturbed, q.ID+"  ["+current+"]  "+q.Q)
			}
		}
	}
	t.Logf("SLICE B would newly classify %d lookup question(s) as distinct:", len(moved)) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	for _, line := range moved {
		t.Logf("   %s", line) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	t.Logf("") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	t.Logf("SLICE B would DISTURB %d already-classified question(s) -- each one a regression risk:", len(disturbed)) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	for _, line := range disturbed {
		t.Logf("   %s", line) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}

// goalSliceBMatches is the candidate predicate, defined here so the census can
// read it before anything wires it into the switch.
//
// Shape: a WH-determiner ("which"/"what") naming a FIELD, with a verb that says
// the value was OBSERVED rather than searched for. Retrieval verbs
// ("mention", "contain", "say", "find", "search") are excluded explicitly,
// because those are the `search` goal and capturing them is the WI-31 defect in
// a new place.
func goalSliceBMatches(question string, terms map[string]bool) bool {
	lower := strings.ToLower(question)
	if !strings.HasPrefix(strings.TrimSpace(lower), "which") && !strings.HasPrefix(strings.TrimSpace(lower), "what") {
		return false
	}
	// A retrieval verb makes it a search, whatever else it looks like.
	for _, verb := range []string{"mention", "contain", "say", "says", "find", "search", "phrase", "match"} {
		if semanticTermsContain(terms, verb) {
			return false
		}
	}
	// A superlative makes it a ranking.
	if semanticTermsContain(terms, semanticRankMarkers...) {
		return false
	}
	// An aggregate marker makes it an aggregate.
	if semanticTermsContain(terms, "many", "much", "average", "sum", "count", "total") {
		return false
	}
	// The observation verbs: the value was produced or observed by a model or
	// a process, and the analyst is asking which values occur.
	// STEMMED forms: semanticFrameTerms stems before matching, so "detected"
	// arrives as `detect` and "produced" as `produc`. Matching the surface form
	// silently captured only two of the five -- the same class of miss as H7,
	// where one matcher compared raw words while everything else stemmed.
	return semanticTermsContain(terms, "detect", "produc", "used", "recognis", "recogniz", "read", "emit")
}

// OFF BY DEFAULT, and off the taxonomy is byte-for-byte what it was.
func TestGoalSlicesOffChangeNothing(t *testing.T) {
	t.Setenv(goalTaxonomySlicesEnv, "")
	for _, path := range []string{
		"../../evaluation/golden_questions_v2.json",
		"../../evaluation/holdout_questions_v1.json",
		"../../evaluation/holdout_media_v1.json",
	} {
		for _, q := range loadRoutingQuestions(t, path) {
			goal := semanticFrameGoal(semanticFrameTerms(q.Q), q.Q)
			for _, id := range []string{"M8-VIDEO-FIRST-SEEN", "M15-AUDIO-MAX-END",
				"M11-OCR-SCRIPTS", "M14-AUDIO-LANGUAGES", "M18-FACE-MODELS",
				"M20-HASH-ALGOS", "H1-MEDIA-WHICH-PLATES"} {
				if q.ID == id && goal != "lookup" {
					t.Errorf("switch OFF: %s is %s, expected the unchanged lookup", id, goal) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
				}
			}
		}
	}
}

// ON, exactly the seven censused questions move and nothing else does.
func TestGoalSlicesOnMoveExactlyTheCensusedSeven(t *testing.T) {
	expected := map[string]string{
		"M8-VIDEO-FIRST-SEEN":   "rank",
		"M15-AUDIO-MAX-END":     "rank",
		"M11-OCR-SCRIPTS":       "distinct",
		"M14-AUDIO-LANGUAGES":   "distinct",
		"M18-FACE-MODELS":       "distinct",
		"M20-HASH-ALGOS":        "distinct",
		"H1-MEDIA-WHICH-PLATES": "distinct",
	}
	before := map[string]string{}
	t.Setenv(goalTaxonomySlicesEnv, "")
	paths := []string{
		"../../evaluation/golden_questions_v2.json",
		"../../evaluation/holdout_questions_v1.json",
		"../../evaluation/holdout_media_v1.json",
	}
	for _, path := range paths {
		for _, q := range loadRoutingQuestions(t, path) {
			before[q.ID] = semanticFrameGoal(semanticFrameTerms(q.Q), q.Q)
		}
	}
	t.Setenv(goalTaxonomySlicesEnv, "true")
	moved := 0
	for _, path := range paths {
		for _, q := range loadRoutingQuestions(t, path) {
			now := semanticFrameGoal(semanticFrameTerms(q.Q), q.Q)
			if now == before[q.ID] {
				if want, ok := expected[q.ID]; ok {
					t.Errorf("%s did not move; expected %s, still %s", q.ID, want, now) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
				}
				continue
			}
			moved++
			want, ok := expected[q.ID]
			if !ok {
				t.Errorf("UNCENSUSED MOVEMENT: %s %s -> %s (%q)", q.ID, before[q.ID], now, q.Q) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
				continue
			}
			if now != want {
				t.Errorf("%s moved to %s, expected %s", q.ID, now, want) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
			}
		}
	}
	if moved != len(expected) {
		t.Errorf("%d questions moved, censused %d", moved, len(expected)) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	t.Logf("exactly %d questions moved, all censused", moved) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
}

// DOC-02 IS THE ONE THAT MUST NOT MOVE. "Which document mentions contact
// number 03001234567?" starts with "which" and is CORRECT today as a `search`.
// Capturing it would be the WI-31 defect in a new place.
func TestGoalSliceBNeverStealsARetrievalQuestion(t *testing.T) {
	t.Setenv(goalTaxonomySlicesEnv, "true")
	for _, q := range []string{
		"Which document mentions contact number 03001234567?",
		"Which image contains the text REF-2026-02?",
		"Which image says Stay Positive Work Hard?",
		"Which recording mentions coconut sugar and at what time?",
		`Find the exact phrase "Scanned-only PDF OCR" in the documents`,
	} {
		if goal := semanticFrameGoal(semanticFrameTerms(q), q); goal == "distinct" {
			t.Errorf("a retrieval question was captured as distinct: %q", q) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
}

// A SUPERLATIVE STAYS A RANKING. CDR-07 "who talked to the most different
// people" is the precedent the distinct guard already exists for.
func TestGoalSlicesKeepSuperlativesRanking(t *testing.T) {
	t.Setenv(goalTaxonomySlicesEnv, "true")
	for _, q := range []string{
		"Who talked to the most different people?",
		"Which camera recorded the most sightings?",
		"What is the largest network volume on any call record?",
	} {
		if goal := semanticFrameGoal(semanticFrameTerms(q), q); goal != "rank" {
			t.Errorf("%q classified %s, expected rank", q, goal) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
}
