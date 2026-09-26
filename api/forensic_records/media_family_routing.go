package main

import (
	"os"
	"sort"
	"strings"
)

// A MEDIA QUESTION MUST BE ABLE TO NAME A MEDIA FAMILY.
//
// Measured live 2026-09-26, and it is the finding that makes every other piece
// of the media work unreachable: of 28 media questions, **ZERO** reached a typed
// plan over forensic.derived_artifacts. Enabling the executor changed nothing --
// identical verdicts AND identical routes with it on and off.
//
// The cause is one line:
//
//	func semanticQuestionFamily(question string) string {
//	    return capabilityFamilyForRecordType(extractCanonicalRecordType(question))
//	}
//
// Family resolution goes through the STRUCTURED record types. A media family is
// not reachable through it at all, so a media question either resolves to a
// structured family -- wrongly -- or to nothing:
//
//	"How many plate groups used persistent object tracking?"
//	    -> resolved anpr_vehicles, because "plate" names the ANPR RECORD TYPE
//	    -> "There are 1,057 ANPR sightings in this case."
//
// Zero became 1,057, and MODEL PLATE GROUPS were answered with INGESTED SIGHTING
// counts. That is the conflation this product must never produce.
//
// THIS IS NOT A KEYWORD LADDER AND MUST NOT BECOME ONE. It adds no vocabulary of
// its own: it asks the CURATED LAYER whether the question names one of its media
// entities, using the SAME phrase matcher that already binds curated values, so
// there is one vocabulary rather than two that can disagree. Widening coverage is
// then a CURATION change -- a synonym in a YAML file -- not a code change.
//
// DEFAULT OFF. Off, `semanticQuestionFamily` behaves exactly as today.
const mediaFamilyRoutingEnv = "FORENSIC_MEDIA_FAMILY_ROUTING"

func mediaFamilyRoutingEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(mediaFamilyRoutingEnv)), "true")
}

// mediaFamilyForQuestion resolves a question to a DERIVED-ARTIFACT family, or ""
// when it names none.
//
// SAFETY PROPERTIES, each one load-bearing:
//
//  1. Only MULTI-WORD phrases match. The structured ANPR family owns the single
//     words -- "plate", "plates", "sighting", "sightings", "camera", "anpr" --
//     and the media entities are deliberately phrased "video plate group",
//     "model plate read", "scene text". A single-word match would let "plate"
//     steal every structured ANPR question, which is the defect in reverse.
//
//  2. EVERY word of the phrase must appear. "video plate group" needs video AND
//     plate AND group, so "how many ANPR sightings are in this case" cannot
//     match it. Order is not required, because analysts do not phrase questions
//     in the layer's word order.
//
//  3. AMBIGUITY RETURNS NOTHING. If two media families match, the question is
//     clarified rather than assigned to whichever sorted first. A wrong family
//     is a confident answer about the wrong evidence.
func mediaFamilyForQuestion(question string) string {
	if !mediaFamilyRoutingEnabled() {
		return ""
	}
	layer, _ := defaultSemanticLayer()
	if layer == nil {
		return ""
	}
	// ALGEBRA ONLY. A media ENTITY computes over observations; the media
	// RETRIEVAL templates cite them. Both are legitimate and they answer
	// different questions, so the goal decides which owns the question.
	//
	// Without this, media routing broke FOUR ACCEPTED routings: "Find cited OCR
	// observations in images" went to image_ocr_search, and "Show face candidate
	// observations in this image" to face_candidate_observations, and media
	// claimed both because "image ocr" and "face candidate" are declared
	// synonyms. A working capability must not be broken to gain an unproven one.
	//
	// Encoding it here rather than narrowing the synonyms is deliberate: the
	// phrases are RIGHT for both surfaces, and asking "is this a computation?"
	// is the actual distinction rather than a spelling workaround.
	if !mediaFamilyAlgebraicGoal(question) {
		return ""
	}
	words := _semanticLayerQuestionTokens(question)
	stems := map[string]bool{}
	for word := range words {
		stems[semanticStem(word)] = true
	}
	matched := map[string]bool{}
	for _, entity := range layer.Entities {
		if !entity.Source.IsDerived() {
			continue
		}
		for _, phrase := range mediaFamilyPhrases(entity) {
			if mediaFamilyPhraseMatches(phrase, words, stems) {
				matched[entity.Family] = true
				break
			}
		}
	}
	if len(matched) != 1 {
		return ""
	}
	families := make([]string, 0, 1)
	for family := range matched {
		families = append(families, family)
	}
	sort.Strings(families)
	return families[0]
}

// mediaFamilyAlgebraicGoals are the goals a curated media ENTITY can serve: a
// question that COMPUTES over observations.
//
// `lookup`, `search` and `extract` are deliberately absent. `lookup` is also the
// UNRECOGNISED default, so admitting it would hand every unparsed question
// naming a media phrase to the compiler -- and "show face candidate
// observations" is exactly that shape. `source_rows` is absent because listing
// derived rows is refused anyway: derived_artifacts carries no record_id or
// row_hash, so a citation cannot be built for it yet.
var mediaFamilyAlgebraicGoals = map[string]bool{
	"aggregate": true,
	"breakdown": true,
	"rank":      true,
	"distinct":  true,
	"range":     true,
}

func mediaFamilyAlgebraicGoal(question string) bool {
	goal := semanticFrameGoal(semanticFrameTerms(question), question)
	return mediaFamilyAlgebraicGoals[goal]
}

// mediaFamilyPhraseMatches reports whether every word of a MULTI-WORD phrase
// appears in the question, as a raw word or as a stem.
//
// STEMMING IS THE DIFFERENCE BETWEEN 4% AND USEFUL. Measured: with exact-token
// matching, ONE of 28 media questions resolved a family. "faces were detected"
// could not match "face detection"; "plate reads" could not match "plate read".
//
// This is the H7 defect in a new place, and its fix is the precedent here:
// multi-word declared synonyms never bound because one matcher compared raw
// words while everything else stemmed. Stemming is confined to MULTI-WORD
// phrases for the reason recorded there -- stemming a single word made "calls"
// bind `CALL` and would have supplied a filter to five correct questions.
func mediaFamilyPhraseMatches(phrase string, words, stems map[string]bool) bool {
	parts := semanticWords.FindAllString(strings.ToLower(strings.TrimSpace(phrase)), -1)
	if len(parts) < 2 {
		return false
	}
	for _, part := range parts {
		if !words[part] && !stems[semanticStem(part)] {
			return false
		}
	}
	return true
}

// mediaFamilyPhrases are the phrases that may name a media entity: its declared
// synonyms, plus its display name.
//
// Single-word phrases are DROPPED here rather than relied on being absent. The
// layer is curated by another track and a well-meaning single-word synonym --
// "plate", "transcript" -- would silently start stealing structured questions.
// The guard belongs where it cannot be edited away.
func mediaFamilyPhrases(entity SemanticLayerEntityV1) []string {
	out := make([]string, 0, len(entity.Synonyms)+1)
	consider := append([]string{}, entity.Synonyms...)
	if display := strings.TrimSpace(entity.DisplayName); display != "" {
		consider = append(consider, display)
	}
	for _, phrase := range consider {
		if len(semanticWords.FindAllString(strings.ToLower(phrase), -1)) < 2 {
			continue
		}
		out = append(out, phrase)
	}
	return out
}

// mediaFamilyRoutingClaims reports whether a media family owns this question, so
// the keyword ladder can ABSTAIN and let the compiler build a typed plan.
//
// The ladder abstaining is already a designed outcome -- `chooseTemplate` ends
// with `return ""` and 39 of 62 questions reach the compiler that way -- so this
// adds no new state, it just routes one more class down the path that already
// exists.
func mediaFamilyRoutingClaims(question string) bool {
	return mediaFamilyForQuestion(question) != ""
}
