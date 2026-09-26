package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"testing"
)

type routingQuestion struct {
	ID string `json:"id"`
	Q  string `json:"q"`
}

func loadRoutingQuestions(t *testing.T, path string) []routingQuestion {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var doc struct {
		Questions []routingQuestion `json:"questions"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return doc.Questions
}

// OFF, nothing changes. Family resolution is consulted by the compiler, the
// catalogue selection, verified-only and the ladder, so an unconditional change
// here would move plans for every question at once.
func TestMediaFamilyRoutingOffChangesNothing(t *testing.T) {
	t.Setenv(mediaFamilyRoutingEnv, "")
	for _, q := range loadRoutingQuestions(t, "../../evaluation/holdout_media_v1.json") {
		if family := mediaFamilyForQuestion(q.Q); family != "" {
			t.Fatalf("switch off: %s resolved %q", q.ID, family)
		}
		if mediaFamilyRoutingClaims(q.Q) {
			t.Fatalf("switch off: %s claimed the ladder", q.ID)
		}
	}
}

// THE SAFETY TEST, and the one that matters most. Not one of the 62 structured
// questions may resolve to a media family. A structured question routed to media
// loses its answer, and abstention bought with a real answer is not a trade this
// product makes -- that is the TWR-02 false negative all over again.
func TestMediaFamilyRoutingNeverClaimsAStructuredQuestion(t *testing.T) {
	t.Setenv(mediaFamilyRoutingEnv, "true")
	for _, path := range []string{
		"../../evaluation/golden_questions_v2.json",
		"../../evaluation/holdout_questions_v1.json",
	} {
		for _, q := range loadRoutingQuestions(t, path) {
			if family := mediaFamilyForQuestion(q.Q); family != "" {
				t.Errorf("%s was claimed by media family %q: %q", q.ID, family, q.Q)
			}
		}
	}
}

// THE COLLISION THIS CHANGE CAUSED, caught as an assertion rather than as a
// stale generated file.
//
// The query-variant ledger holds every ACCEPTED routing in the product -- far
// more than the 62 -- and three of them broke: "Show face candidate observations
// in this image" and its Urdu variants routed to the working template
// `face_candidate_observations` (family face_intelligence), and media routing
// claimed them because Codex declared "face candidate" as a synonym of
// face_model_observation.
//
// **A working capability must not be broken to gain an unproven one.** The
// resolution is CURATION -- the media entity should not claim a phrase an
// existing template already owns -- so this test names the rule rather than
// patching around it in code.
func TestMediaFamilyRoutingNeverClaimsAnAcceptedVariantRouting(t *testing.T) {
	t.Setenv(mediaFamilyRoutingEnv, "true")
	seeds := acceptedQueryVariantSeedsForTest(t)
	if len(seeds) == 0 {
		t.Skip("no accepted variant seeds available")
	}
	// EMPTY, AND IT MUST STAY EMPTY. Four collisions existed when this test was
	// written -- "Find cited OCR observations in images" and three "Show face
	// candidate observations" variants -- and all four are now excluded by the
	// ALGEBRAIC GOAL gate rather than by narrowing a synonym: those phrases are
	// right for both surfaces, and "is this a computation?" is the real
	// distinction.
	//
	// A non-empty list here means a collision is being tolerated. Prefer fixing
	// it; if one must be recorded, record why, because a stale allowlist hides
	// the next one.
	known := map[string]bool{}
	unexpected := 0
	for _, seed := range seeds {
		family := mediaFamilyForQuestion(seed.Text)
		if family == "" {
			continue
		}
		if known[seed.Text] {
			t.Logf("owed curation: %q (%s -> %s)", seed.Text, seed.Template, family)
			continue
		}
		unexpected++
		t.Errorf("a NEW accepted routing was claimed by media: %q (%s -> %s)", seed.Text, seed.Template, family)
	}
	if unexpected > 0 {
		t.Log("FIX IN CURATION: remove the colliding phrase from the media entity's synonyms.")
		t.Log("Do NOT special-case it here -- a code exception would hide the next collision.")
	}
}

// The ANPR conflation, stated as a test. The structured family owns "sighting";
// the media families own the multi-word phrases. These two questions differ by
// almost nothing in wording and must route to different evidence.
func TestMediaFamilyRoutingKeepsSightingsStructured(t *testing.T) {
	t.Setenv(mediaFamilyRoutingEnv, "true")
	if family := mediaFamilyForQuestion("How many ANPR sightings are in this case?"); family != "" {
		t.Fatalf("a question about ingested sightings was claimed by %q", family)
	}
	if family := mediaFamilyForQuestion("How many model plate reads came from the images?"); family == "" {
		t.Fatal("a question naming a model plate read resolved no media family")
	}
}

// A single-word synonym must never bind, even if one is added to the layer
// later. "plate" alone would steal every structured ANPR question.
func TestMediaFamilyPhrasesRejectSingleWords(t *testing.T) {
	entity := SemanticLayerEntityV1{
		DisplayName: "Plates",
		Synonyms:    []string{"plate", "plates", "video plate group"},
	}
	phrases := mediaFamilyPhrases(entity)
	if len(phrases) != 1 || phrases[0] != "video plate group" {
		t.Fatalf("single-word phrases were not dropped: %#v", phrases)
	}
}

// AMBIGUITY CLARIFIES. Two media families matching means the question does not
// establish which evidence it is about, and picking one would be a confident
// answer about possibly the wrong thing.
func TestMediaFamilyRoutingRefusesAnAmbiguousQuestion(t *testing.T) {
	t.Setenv(mediaFamilyRoutingEnv, "true")
	// Names both a model plate read and a scene-text read.
	ambiguous := "Compare the model plate read against the scene text read for this image"
	if family := mediaFamilyForQuestion(ambiguous); family != "" {
		t.Fatalf("an ambiguous question was assigned to %q", family)
	}
}

// CENSUS, not an assertion. Reports which media questions the CURRENT curated
// synonyms reach, so coverage is a visible curation number rather than a
// surprise in a live run. Coverage below 100% is expected and is closed by adding
// synonyms in the layer, not by changing this code.
// mediaRoutingBlocker names WHICH GATE stops an unclaimed question.
//
// The census used to report every unclaimed question as "needs a curated
// synonym, or is a structured/honesty probe" -- a GUESS, and a costly one: it
// framed the remaining 21 as pure curation work and a synonym brief was written
// on that basis. Forecasting the proposed synonyms through the real matcher
// 2026-09-26 showed the true split: EIGHT are vocabulary, and SEVEN already
// match a curated phrase and are refused by the ALGEBRA GOAL GATE, which no
// amount of YAML can move.
//
// This is the §6 lesson in the instrument itself: list every gate a question can
// die at, and report which one actually fired. A census that guesses at the
// cause sends the next cycle of work to the wrong track.
func mediaRoutingBlocker(question string) string {
	layer, _ := defaultSemanticLayer()
	if layer == nil {
		return "LAYER UNAVAILABLE"
	}
	words := _semanticLayerQuestionTokens(question)
	stems := map[string]bool{}
	for word := range words {
		stems[semanticStem(word)] = true
	}
	matched := []string{}
	for _, entity := range layer.Entities {
		if !entity.Source.IsDerived() {
			continue
		}
		for _, phrase := range mediaFamilyPhrases(entity) {
			if mediaFamilyPhraseMatches(phrase, words, stems) {
				matched = append(matched, entity.Family)
				break
			}
		}
	}
	sort.Strings(matched)
	goal := semanticFrameGoal(semanticFrameTerms(question), question)
	switch {
	case len(matched) == 0 && !mediaFamilyAlgebraicGoals[goal]:
		// BOTH gates. A synonym is NECESSARY BUT NOT SUFFICIENT here: adding one
		// moves the question from this line to the GOAL GATE line and it still
		// does not route. Reporting only the first gate is what sent the last
		// cycle of work entirely to curation.
		return fmt.Sprintf("VOCABULARY+GOAL  no phrase matches AND goal=%s is not algebraic; a synonym alone will NOT route it", goal)
	case len(matched) == 0:
		return fmt.Sprintf("VOCABULARY    no curated phrase matches (goal=%s is algebraic, so a synonym is enough)", goal)
	case !mediaFamilyAlgebraicGoals[goal]:
		return fmt.Sprintf("GOAL GATE     %v matches, but goal=%s is not algebraic; curation cannot fix this", matched, goal)
	case len(matched) > 1:
		return fmt.Sprintf("AMBIGUOUS     %v both match, so it clarifies (goal=%s)", matched, goal)
	default:
		return fmt.Sprintf("UNEXPLAINED   %v matches and goal=%s is algebraic", matched, goal)
	}
}

func TestMediaFamilyRoutingCensus(t *testing.T) {
	t.Setenv(mediaFamilyRoutingEnv, "true")
	questions := loadRoutingQuestions(t, "../../evaluation/holdout_media_v1.json")
	resolved, unresolved := 0, []string{}
	for _, q := range questions {
		family := mediaFamilyForQuestion(q.Q)
		if family == "" {
			unresolved = append(unresolved, fmt.Sprintf("%-26s %s\n      %s", q.ID, mediaRoutingBlocker(q.Q), q.Q))
			continue
		}
		resolved++
		t.Logf("  %-26s -> %s", q.ID, family)
	}
	t.Logf("")
	t.Logf("MEDIA ROUTING COVERAGE: %d of %d (%.0f%%)", resolved, len(questions),
		100*float64(resolved)/float64(len(questions)))
	t.Logf("NOT CLAIMED -- with the gate that actually refused each one:")
	t.Logf("  VOCABULARY is curation (semantic_layer/**). GOAL GATE is backend (goal taxonomy).")
	for _, line := range unresolved {
		t.Logf("  %s", line)
	}
	if resolved == 0 {
		t.Fatal("no media question resolved a family; the mechanism is not working at all")
	}
}
