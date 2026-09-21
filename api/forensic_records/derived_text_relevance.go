package main

import (
	"regexp"
	"strings"
)

// Natural-language relevance for document, OCR and transcript search when no
// exact/phrase/time mode was classified. The previous scorer counted
// substrings of every non-stopword term, so function words such as "is",
// "in" and "any" matched inside unrelated text and an arbitrary segment was
// returned as a match (live: "Is the number 03001234567 mentioned in any
// audio?" returned a segment about Japanese cuisine).

var derivedTextIdentifier = regexp.MustCompile(`(?i)\b(?:\+?\d[\d\s-]{5,}\d|[a-z]{1,4}-?\d{2,}(?:-\d+)*|\d+[a-z]{1,4}\d*)\b`)

var derivedTextQuestionWords = map[string]bool{
	"a": true, "an": true, "the": true, "is": true, "are": true, "was": true, "were": true, "be": true, "been": true,
	"in": true, "on": true, "at": true, "of": true, "to": true, "for": true, "from": true, "by": true, "with": true, "about": true,
	"and": true, "or": true, "any": true, "all": true, "some": true, "it": true, "its": true, "this": true, "that": true, "these": true, "those": true,
	"what": true, "which": true, "who": true, "whom": true, "where": true, "when": true, "why": true, "how": true, "does": true, "do": true, "did": true,
	"there": true, "here": true, "time": true, "times": true, "say": true, "says": true, "said": true, "mention": true, "mentions": true, "mentioned": true,
	"mentioning": true, "contain": true, "contains": true, "containing": true, "search": true, "find": true, "show": true, "me": true, "text": true,
	"word": true, "words": true, "phrase": true, "audio": true, "recording": true, "recordings": true, "transcript": true, "transcripts": true,
	"document": true, "documents": true, "doc": true, "docs": true, "pdf": true, "file": true, "files": true, "image": true, "images": true,
	"photo": true, "photos": true, "picture": true, "ocr": true, "video": true, "videos": true, "case": true, "evidence": true, "number": true,
	"source": true, "sources": true, "anything": true, "something": true, "somewhere": true, "appear": true, "appears": true, "describe": true, "describes": true,
}

func derivedTextDigits(value string) string {
	return strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, value)
}

// derivedTextRelevance returns a relevance score and whether the passage is
// relevant: every identifier in the question must appear (digits compared
// without formatting), and at least half of the remaining content words must
// start a word in the passage.
func derivedTextRelevance(text, question string) (int, bool) {
	lower := strings.ToLower(text)
	textDigits := derivedTextDigits(lower)
	compact := strings.NewReplacer("-", "", " ", "").Replace(lower)
	score := 0
	identifiers := derivedTextIdentifier.FindAllString(question, -1)
	for _, identifier := range identifiers {
		id := strings.ToLower(strings.TrimSpace(identifier))
		digits := derivedTextDigits(id)
		switch {
		case len(digits) >= 6 && len(digits) == len(strings.NewReplacer(" ", "", "-", "", "+", "").Replace(id)):
			if !strings.Contains(textDigits, digits) {
				return 0, false
			}
		default:
			if !strings.Contains(compact, strings.NewReplacer("-", "", " ", "").Replace(id)) {
				return 0, false
			}
		}
		score += 10
	}
	remaining := derivedTextIdentifier.ReplaceAllString(question, " ")
	content := []string{}
	for _, term := range rawQueryTerms(remaining) {
		if len(term) < 3 || derivedTextQuestionWords[term] {
			continue
		}
		content = append(content, term)
	}
	matched := 0
	for _, term := range content {
		pattern := regexp.MustCompile(`(?:^|[^\p{L}\p{N}])` + regexp.QuoteMeta(term))
		if hits := len(pattern.FindAllStringIndex(lower, -1)); hits > 0 {
			matched++
			score += hits
		}
	}
	if len(content) > 0 && matched*2 < len(content) {
		return 0, false
	}
	if len(identifiers) == 0 && len(content) == 0 {
		return 0, true // nothing to filter on: a browse request
	}
	return score + matched*5, true
}
