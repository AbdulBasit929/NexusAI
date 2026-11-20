package main

import (
	"testing"
)

func TestTextLookupValueIsALongNumberOrACodeAsTyped(t *testing.T) {
	for message, want := range map[string]string{
		"Where does 042-35761234 appear in this case?": "042-35761234",
		"which files mention PK00SYNT0000000000002005": "PK00SYNT0000000000002005",
		"find the plate SYNT9150":                      "SYNT9150",
		"where is the amount 1,234,567.00 written":     "1,234,567.00",
		"find Hassan Farooqi":                          "",
		"Who called 0300 most often?":                  "",
		"how many calls were made in March 2026":       "",
		"calls between 04/02/2026 and 16/04/2026":      "",
	} {
		if got := textLookupValue(message); got != want {
			t.Errorf("textLookupValue(%q) = %q, want %q", message, got, want) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
}

func TestTextLookupCandidateIsRefusedWhereTheServerRefusesAValueOrTheRequestIsTied(t *testing.T) {
	if got := textLookupCandidate(textIntentRequest("Where does 042-35761234 appear in this case?")); got != "042-35761234" {
		t.Fatalf("candidate = %q", got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	if got := textLookupCandidate(textIntentRequest("Where is 35202-1234567-1 written?")); got != "" {
		t.Fatalf("a national identity number must never be looked up: %q", got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	if got := textLookupCandidate(textIntentRequest("042-35761234")); got != "" {
		t.Fatalf("the whole message is not a value: %q", got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	tied := textIntentRequest("Where does 042-35761234 appear in this case?")
	tied.Template = "document_search"
	if got := textLookupCandidate(tied); got != "" {
		t.Fatalf("a request tied to an operation is not looked up: %q", got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	selected := textIntentRequest("Where does 042-35761234 appear in this case?")
	selected.EvidenceID = "11111111-1111-4111-8111-111111111111"
	if got := textLookupCandidate(selected); got != "" {
		t.Fatalf("a request tied to selected evidence is not looked up: %q", got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}

func TestTextLookupAnswerEmptyOnlyWhenTheRecordsRouteHadNothingToSay(t *testing.T) {
	answer := func(status string, rows any) hybridQueryResponse {
		return hybridQueryResponse{Enterprise: map[string]any{"status": status, "row_count": rows}}
	}
	for name, c := range map[string]struct {
		resp hybridQueryResponse
		want bool
	}{
		"rows":              {answer("completed", 3), false},
		"one figure":        {answer("completed", 1), false},
		"no rows":           {answer("completed", 0), true},
		"no rows as float":  {answer("completed", float64(0)), true},
		"needs input":       {answer("needs_input", 0), true},
		"needs input, rows": {answer("needs_input", 2), true},
		"unsupported":       {answer("unsupported", 0), true},
		"no enterprise":     {hybridQueryResponse{}, false},
		"no row count":      {hybridQueryResponse{Enterprise: map[string]any{"status": "completed"}}, false},
	} {
		if got := textLookupAnswerEmpty(c.resp); got != c.want {
			t.Errorf("%s: textLookupAnswerEmpty = %v, want %v", name, got, c.want) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
}

func TestTextLookupAugmentLeavesAnAnswerWithRowsAndASwitchedOffSearchUntouched(t *testing.T) {
	req := textIntentRequest("Where does 042-35761234 appear in this case?")
	req.TextLookup = "042-35761234"
	withRows := hybridQueryResponse{Enterprise: map[string]any{"status": "completed", "row_count": 4, "executive_answer": "Total calls: 694."}}
	t.Setenv(textSearchAllEnv, "true")
	if got := textLookupAugment(t.Context(), nil, req, withRows); got.Enterprise["executive_answer"] != "Total calls: 694." {
		t.Fatalf("an answer with rows must not change: %v", got.Enterprise["executive_answer"]) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	empty := hybridQueryResponse{Enterprise: map[string]any{"status": "needs_input", "row_count": 0, "executive_answer": "Which MSISDN?"}}
	if got := textLookupAugment(t.Context(), nil, req, empty); got.Enterprise["executive_answer"] != "Which MSISDN?" { // no database: nothing can be added
		t.Fatalf("without a database nothing is added: %v", got.Enterprise["executive_answer"]) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	t.Setenv(textSearchAllEnv, "false")
	if got := textLookupAugment(t.Context(), nil, req, empty); got.Enterprise["executive_answer"] != "Which MSISDN?" {
		t.Fatalf("with the switch off nothing is added: %v", got.Enterprise["executive_answer"]) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}
