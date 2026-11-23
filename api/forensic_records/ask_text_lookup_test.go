package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
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
		"calls between 04/02/2026 and 16/04/2026":      "04/02/2026",
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
	if got := textLookupAugment(t.Context(), config{}, nil, req, withRows); got.Enterprise["executive_answer"] != "Total calls: 694." {
		t.Fatalf("an answer with rows must not change: %v", got.Enterprise["executive_answer"]) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	empty := hybridQueryResponse{Enterprise: map[string]any{"status": "needs_input", "row_count": 0, "executive_answer": "Which MSISDN?"}}
	if got := textLookupAugment(t.Context(), config{}, nil, req, empty); got.Enterprise["executive_answer"] != "Which MSISDN?" { // no database: nothing can be added
		t.Fatalf("without a database nothing is added: %v", got.Enterprise["executive_answer"]) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	t.Setenv(textSearchAllEnv, "false")
	if got := textLookupAugment(t.Context(), config{}, nil, req, empty); got.Enterprise["executive_answer"] != "Which MSISDN?" {
		t.Fatalf("with the switch off nothing is added: %v", got.Enterprise["executive_answer"]) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}

func lookupServer(t *testing.T, reply textLookupWire) (*httptest.Server, *int) {
	t.Helper()
	calls := 0
	content, _ := json.Marshal(reply)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		calls++
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []map[string]any{{"message": map[string]string{"content": string(content)}, "finish_reason": "stop"}}})
	}))
	t.Cleanup(server.Close)
	return server, &calls
}

func TestTextLookupExtractAcceptsOnlyAValueTheServerAccepts(t *testing.T) {
	message := "Which chats mention Zainab Gill?"
	server, calls := lookupServer(t, textLookupWire{Literal: "Zainab Gill"})
	cfg := config{LocalAIURL: server.URL, SynthesisModel: "qwen3-4b"}
	if got := textLookupExtract(t.Context(), cfg, textIntentRequest(message)); got != "Zainab Gill" || *calls != 1 {
		t.Fatalf("value = %q after %d calls", got, *calls) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	for name, reply := range map[string]textLookupWire{
		"empty":                {Literal: ""},
		"not in the message":   {Literal: "Someone Else"},
		"the whole message":    {Literal: message},
		"national identity id": {Literal: "35202-1234567-1"},
	} {
		server, _ := lookupServer(t, reply)
		question := message
		if name == "national identity id" {
			question = "Where is 35202-1234567-1 written?"
		}
		if got := textLookupExtract(t.Context(), config{LocalAIURL: server.URL, SynthesisModel: "qwen3-4b"}, textIntentRequest(question)); got != "" {
			t.Errorf("%s: value %q must be refused", name, got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
	tied := textIntentRequest(message)
	tied.Template = "document_search"
	server, calls = lookupServer(t, textLookupWire{Literal: "Zainab Gill"})
	if got := textLookupExtract(t.Context(), config{LocalAIURL: server.URL, SynthesisModel: "qwen3-4b"}, tied); got != "" || *calls != 0 {
		t.Fatalf("a request tied to an operation is not looked at (calls %d, value %q)", *calls, got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) }))
	t.Cleanup(failing.Close)
	if got := textLookupExtract(t.Context(), config{LocalAIURL: failing.URL, SynthesisModel: "qwen3-4b"}, textIntentRequest(message)); got != "" {
		t.Fatalf("a failing model must change nothing: %q", got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}

func TestTextLookupAugmentAsksTheModelOnlyForAnEmptyAnswerWithoutANumber(t *testing.T) {
	t.Setenv(textSearchAllEnv, "true")
	server, calls := lookupServer(t, textLookupWire{Literal: "Zainab Gill"})
	cfg := config{LocalAIURL: server.URL, SynthesisModel: "qwen3-4b"}
	req := textIntentRequest("Which chats mention Zainab Gill?")
	withRows := hybridQueryResponse{Enterprise: map[string]any{"status": "completed", "row_count": 2, "executive_answer": "2 rows."}}
	if got := textLookupAugment(t.Context(), cfg, nil, req, withRows); got.Enterprise["executive_answer"] != "2 rows." || *calls != 0 {
		t.Fatalf("an answer with rows asks nothing (calls %d)", *calls) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	empty := hybridQueryResponse{Enterprise: map[string]any{"status": "needs_input", "row_count": 0, "executive_answer": "I could not tie it."}}
	_ = textLookupAugment(t.Context(), cfg, nil, req, empty) // no database: nothing is added, but the model was asked once
	if *calls != 1 {
		t.Fatalf("an empty answer without a number asks the model once (calls %d)", *calls) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	numbered := textIntentRequest("Where does 042-35761234 appear?")
	numbered.TextLookup = "042-35761234"
	_ = textLookupAugment(t.Context(), cfg, nil, numbered, empty)
	if *calls != 1 {
		t.Fatalf("a message with a number needs no model (calls %d)", *calls) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}
