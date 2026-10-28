package main

import (
	"regexp"
	"strings"
)

// ASK ENGINE V2: DOCUMENTS (docs/roadmap/reference-cases-programme-20261028.md, stage S3b). The mention index of text documents (migration 031) says which documents name which numbers,
// accounts, plates, SIM serials, handsets and places. What it says is that a document NAMES a key: not who holds it, that its holder took part in anything, or that the document is true.
// National ID numbers are identity data: the index counts them and keeps neither the number nor its text until the masking design is approved (R9), so a question that names one is answered
// with that and not run.

var askDocumentViews = []string{"v_document_mentions", "v_document_values", "v_document_links"}

// askDocPremiseCue is a question that treats a mention as proof of who holds a number or who took part.
var askDocPremiseCue = regexp.MustCompile(`\b(?:prove[sd]?|proof|owner|owns?|belongs?|accused|guilty|involved|culprit|suspect\w*|criminal)\b`)

// askNationalIDAnswer is what a question that names a national ID number is told.
const askNationalIDAnswer = "National ID numbers are identity data. The documents' mentions of them are counted, and the numbers themselves are withheld until the masking design is approved, so a question cannot search for one or show one. You can ask which documents contain a national ID number, or search the documents by a mobile number, an account or a plate."

// askReadsDocumentView is whether the query reads one of the document views.
func askReadsDocumentView(v *askValidated) bool {
	for _, view := range askDocumentViews {
		if askReadsView(v, view) {
			return true
		}
	}
	return false
}

// askDocumentAbsence says what was searched when a document view returns no row: the keys of the question, or the kind of link asked for.
func askDocumentAbsence(v *askValidated, f askFrame) string {
	named := append(append(append([]string{}, f.Numbers...), f.Accounts...), f.Plates...)
	named = append(named, f.IMEIs...)
	named = append(named, f.Serials...)
	switch {
	case askReadsView(v, "v_document_links") && len(named) == 0:
		return "no key named in the documents of this case is found in the records of the case"
	case askReadsView(v, "v_document_links"):
		return strings.Join(named, ", ") + " is not both named in a document and found in the records of the case"
	case len(named) == 1:
		return "no document of this case names " + named[0]
	case len(named) > 1:
		return "no document of this case names " + strings.Join(named, " and ")
	}
	return "no document of this case matches your question"
}

// askDocumentLead is the correction put in front of a question that treats a mention as proof of who holds a number or took part. "" when it does not.
func askDocumentLead(f askFrame, stated bool) string {
	if stated || askDocPremiseCue.MatchString(strings.ToLower(f.Question)) {
		return "A mention in a document says that the document names it. It does not say who holds the number, that its holder took part in anything, or that the document is true."
	}
	return ""
}
