package main

import (
	"os"
	"regexp"
	"strings"
)

// U2a. A SEARCH THE QUESTION AIMED AT ONE KIND OF EVIDENCE STAYS THERE.
//
// Measured 2026-09-28 on the deployed build:
//
//	"Find OCR text mentioning SINDH"    -> image_ocr_search -> "DSC_1105.JPG contains the text SINDH"
//	"Find OCR text mentioning BCX-567"  -> withheld: "You asked about images, but this question
//	                                        matched a different kind of evidence"
//
// The keyword router chose the image-text search for BOTH. For BCX-567 the
// one-identifier hybrid shortcut then overrode it with cross-evidence
// correlation, because a plate-shaped term is an identifier; the media misroute
// guard correctly withheld the result. The withhold's own clarification option
// for images is "Find OCR text mentioning %s" -- which therefore also failed.
//
// DOC-01 "Search the case documents for mentions of plate MN1367" fails one
// step later: the router chose document search, but the word "plate" gives the
// question the ANPR family, so verified-only demanded a typed plan, arbitration
// swapped in an ANPR records plan, and the misroute guard withheld it.
//
// Both gates key on one fact: the chosen template is a text search over one
// kind of evidence AND the question itself names that kind of evidence. Then
// the search runs where the question asked. The search is cited retrieval, and
// the text-browse guard and the relevance checks still apply to it.
// See reports/evidence-search-first-20260928/.
const evidenceSearchFirstEnv = "FORENSIC_EVIDENCE_SEARCH_FIRST"

func evidenceSearchFirstEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(evidenceSearchFirstEnv)), "true") //nolint:forbidigo // forensic records product API reads its feature switches from the environment by design (docker compose); not LocalAI server config
}

// derivedTextSearchEvidence maps each text-search template to the evidence
// scope (mediaEvidenceScopes.Noun) whose words must appear in the question.
var derivedTextSearchEvidence = map[string]string{
	"document_search":         "documents",
	"image_ocr_search":        "images",
	"audio_transcript_search": "audio recordings",
}

// derivedTextSearchNamedByQuestion reports that template is a text search over
// one kind of evidence and the question names that same kind of evidence.
func derivedTextSearchNamedByQuestion(question, template string) bool {
	if !evidenceSearchFirstEnabled() {
		return false
	}
	noun, ok := derivedTextSearchEvidence[template]
	if !ok {
		return false
	}
	lowered := strings.ToLower(question)
	for _, media := range mediaEvidenceScopes {
		if media.Noun != noun {
			continue
		}
		for _, keyword := range media.Keywords {
			if regexp.MustCompile(`\b` + regexp.QuoteMeta(keyword) + `\b`).MatchString(lowered) {
				return true
			}
		}
	}
	return false
}
