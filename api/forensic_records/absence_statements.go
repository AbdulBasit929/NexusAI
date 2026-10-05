package main

import (
	"fmt"
	"os"
	"strings"
)

// STATEMENTS OF ABSENCE THAT WERE NOT FINDINGS. Two defects of one kind: the
// analyst was told "nothing was found" where nothing established absence.
// A false absence is the worst answer this product can give, because the
// analyst stops looking. See reports/absence-text-20260928/.

// S2. A word search that found nothing is not evidence of absence.
//
// P2 "Were any of the transcribed audio segments taken from video?" ran a
// relevance search of the transcripts for the question's own words, found no
// match among 18 observations, and answered "No transcript segment intersects
// the requested source-time range." No time range was requested -- that is the
// headline mapped to NO_MATCH whatever the search mode -- and one segment DID
// come from a video's audio track. A word search cannot show that something is
// absent. Exact-phrase and time-range searches are untouched: their empty
// results are scoped to exactly what was searched.
const textSearchNotAbsenceEnv = "FORENSIC_TEXT_SEARCH_NOT_ABSENCE"

const withholdTextSearchNotAbsence = "text_search_not_absence"

func textSearchNotAbsenceEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(textSearchNotAbsenceEnv)), "true") //nolint:forbidigo // forensic records product API reads its feature switches from the environment by design (docker compose); not LocalAI server config
}

var textSearchSubjects = map[string]string{
	"audio_transcript_search": "the transcripts",
	"image_ocr_search":        "the text read from the images",
	"document_search":         "the documents",
}

// relevanceSearchMissIsNotAbsence reports an empty relevance-mode text search:
// no exact term, not a time range, result NO_MATCH.
func relevanceSearchMissIsNotAbsence(resp hybridQueryResponse) bool {
	if !textSearchNotAbsenceEnabled() {
		return false
	}
	if _, ok := textSearchSubjects[resp.Template]; !ok {
		return false
	}
	evidence := resp.Evidence
	if evidence == nil || stringValueAny(evidence["result_state"]) != "NO_MATCH" {
		return false
	}
	if strings.TrimSpace(stringValueAny(evidence["exact_term"])) != "" {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(stringValueAny(evidence["query_mode"]))) {
	case "time_range", "exact", "exact_value", "exact_phrase", "phrase_contains", "token_search":
		return false
	}
	return true
}

func textSearchNotAbsenceReason(resp hybridQueryResponse) withholdReason {
	subject := textSearchSubjects[resp.Template]
	return withholdReason{
		Code: withholdTextSearchNotAbsence,
		Question: fmt.Sprintf("I searched %s for the words in your question and found no match. A word "+
			"search cannot show that something is absent, so I have not answered no. Give me a specific "+
			"word, phrase or identifier to look for, or name the detail you want counted.", subject),
		Guardrail: "No absence was stated because the only search run was a word search for the " +
			"question's own terms, and an empty word search does not establish that nothing exists.",
		Detail: fmt.Sprintf("template=%s mode=%s", resp.Template, stringValueAny(resp.Evidence["query_mode"])),
	}
}

// S3. A clarification states its question, never a result.
//
// R5 "What do the two busiest numbers have in common?" was correctly held for
// clarification -- nothing ran -- but the analyst read "No qualifying
// phone-counterparty events were found for the selected target in the exact
// source set": buildEnterprisePayload sends every multi_cdr_comparison
// response, clarification included, to the comparison's own payload builder,
// which sees zero rows and writes a no-results sentence.
const clarificationNotResultEnv = "FORENSIC_CLARIFICATION_NOT_RESULT"

func clarificationNotResultEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(clarificationNotResultEnv)), "true") //nolint:forbidigo // forensic records product API reads its feature switches from the environment by design (docker compose); not LocalAI server config
}

// clarifyingInsteadOfResult reports a response that is asking, not answering,
// so no result-specific payload builder may describe it.
func clarifyingInsteadOfResult(resp hybridQueryResponse) bool {
	if !clarificationNotResultEnabled() {
		return false
	}
	return resp.Intent == intentClarify || resp.Answer["clarification_required"] == true
}
