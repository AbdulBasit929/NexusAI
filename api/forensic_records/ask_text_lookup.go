package main

import (
	"regexp"
	"strings"
	"unicode"
)

// THE VALUE LOOKUP: when the records route has nothing to say about a message that holds a number or a code, the text search says where that value is written.
//
// Measured 2026-11-19/20 (docs/data/ask-exhaustive-search-results-20261119.md): a message such as "Where does 042-35761234 appear in this case?" is placed by the intent step with the records, and the
// records route answers "no row matches" or asks which MSISDN to compare. Two ways of getting such a message to the text search through the model were measured and rejected: an added instruction in
// the intent prompt moved plain "how many records does <number> have" questions to the text search (4 to 5 false routes of 50, gate 0) and lowered the routing of the real text searches (49 to 46
// of 57 on development bank 2); a separate question to the model ("does this message ask in which files a value is written?") said yes to 47 of 72 plain structured questions (65%), although it
// said yes to all 25 real lookups. A 4-billion-parameter model does not make that distinction, so none is asked of it.
//
// What is done instead needs no model and cannot touch an answer that has rows: after the records route has answered, and only when its answer is EMPTY (no rows, or a request for more input), the
// numbers and codes the analyst typed are searched in the text of the case and the files that hold them are added to the answer (textLookupAugment, ask_text_search_all.go). The value is a word of the
// message as typed; the server's checks are the same as the intent step's (a national identity number is never searched, the value is never the whole message). Switch: FORENSIC_TEXT_SEARCH_ALL.

// tiLookupDate is a date written with separators; a date names no value that a file could be asked for.
var tiLookupDate = regexp.MustCompile(`^\d{1,2}[/.-]\d{1,2}[/.-]\d{2,4}$|^\d{4}[/.-]\d{1,2}[/.-]\d{1,2}$`)

// textLookupValue is the first word of the message that looks like a value a file could be asked for: six digits or more (a phone number, an account, a reference), or four digits with a letter in
// the same word (a plate), and not a date. It is the word as the analyst typed it, without the punctuation around it.
func textLookupValue(message string) string {
	for _, word := range strings.Fields(message) {
		word = strings.Trim(word, "?!,;:()[]{}\"'")
		if tiLookupDate.MatchString(word) {
			continue
		}
		digits, letters := 0, 0
		for _, r := range word {
			switch {
			case unicode.IsDigit(r):
				digits++
			case unicode.IsLetter(r):
				letters++
			}
		}
		if digits >= 6 || digits >= 4 && letters > 0 {
			return word
		}
	}
	return ""
}

// textLookupCandidate is the value of the message to look up if the records route turns out to have nothing, or "" when the request is tied to an operation or to selected evidence (the text of the
// whole case is then not what was asked), or when the server's checks refuse the value.
func textLookupCandidate(req hybridQueryRequest) string {
	if req.Template != "" || req.TextQuery != nil || req.ExactTerm != "" || req.TranscriptMode != "" || req.Target != "" || len(req.Targets) > 0 || req.EvidenceID != "" ||
		req.QueryScope.Kind == string(EvidenceScopeSelected) || len(req.Projection) > 0 || req.Group != nil || req.Compare != nil || req.SourceSet != nil {
		return ""
	}
	query := strings.TrimSpace(req.Query)
	value := textLookupValue(query)
	if value == "" {
		return ""
	}
	lookup, _, why := textIntentValidate(query, textIntentResult{Kind: tiKindText, Family: tiFamilyAny, Match: tiMatchPhrase, Literal: value})
	if why != "" || lookup == nil {
		return ""
	}
	return lookup.LiteralText
}

// textLookupAnswerEmpty says whether the records route had nothing to say: the answer has no rows, or it asks for more input before it can run.
func textLookupAnswerEmpty(resp hybridQueryResponse) bool {
	if resp.Enterprise == nil {
		return false
	}
	switch status := strings.ToLower(stringValueAny(resp.Enterprise["status"])); status {
	case "needs_input", "unsupported":
		return true
	}
	switch rows := resp.Enterprise["row_count"].(type) {
	case int:
		return rows == 0
	case int64:
		return rows == 0
	case float64:
		return rows == 0
	}
	return false
}
