package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mudler/LocalAI/pkg/forensicrequest"
	"github.com/mudler/LocalAI/pkg/forensictext"
)

// intentModel answers /v1/chat/completions with a fixed intent and records every request body it saw.
type intentModel struct {
	server *httptest.Server
	bodies []string
	status int
}

// intentWireFor is what the real model would say for a result: the "about" value that maps to the result's kind and family.
func intentWireFor(result textIntentResult) textIntentWire {
	about := tiKindOther
	if result.Kind == tiKindPlate {
		about = tiAboutPlate
	}
	if result.Kind == tiKindText {
		for value, family := range tiAboutText {
			if family == result.Family {
				about = value
			}
		}
	}
	return textIntentWire{MediaWords: result.MediaWords, About: about, Match: result.Match, Literal: result.Literal}
}

func newIntentModel(t *testing.T, result textIntentResult) *intentModel {
	t.Helper()
	content, _ := json.Marshal(intentWireFor(result))
	m := &intentModel{status: http.StatusOK}
	m.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		m.bodies = append(m.bodies, string(body))
		if m.status != http.StatusOK {
			w.WriteHeader(m.status)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []map[string]any{{"message": map[string]string{"content": string(content)}, "finish_reason": "stop"}}})
	}))
	t.Cleanup(m.server.Close)
	return m
}

func textIntentRequest(query string) hybridQueryRequest {
	req := hybridQueryRequest{TenantID: "default", CollectionID: "demo", Query: query, SynthesisModel: "qwen3-4b"}
	req.RequestClass = forensicrequest.GovernedAnalysis
	return req
}

func routeIntent(t *testing.T, m *intentModel, req hybridQueryRequest) (hybridQueryRequest, string, bool, *httptest.ResponseRecorder) {
	t.Helper()
	rec := httptest.NewRecorder()
	out, family, routed := textIntentRoute(t.Context(), rec, config{LocalAIURL: m.server.URL, SynthesisModel: "qwen3-4b"}, req, time.Now())
	return out, family, routed, rec
}

func TestTextIntentIsOffByDefault(t *testing.T) {
	if textIntentEnabled() {
		t.Fatal("the step adds a model call; it must default to off") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	t.Setenv(textIntentEnv, "true")
	if !textIntentEnabled() {
		t.Fatal("an explicit true must enable it") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}

func TestTextIntentRoutesASearchForPhraseInDocumentsToTheStructuredQuery(t *testing.T) {
	m := newIntentModel(t, textIntentResult{Kind: tiKindText, Family: tiFamilyDocument, Match: tiMatchPhrase, Literal: "wire transfer"})
	out, family, routed, rec := routeIntent(t, m, textIntentRequest("Which documents mention wire transfer?"))
	if !routed || family != tiFamilyDocument {
		t.Fatalf("routed=%v family=%q", routed, family) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	if out.TextQuery == nil || out.TextQuery.LiteralText != "wire transfer" || out.TextQuery.MatchSemantic != forensictext.PhraseContains || !out.TextQuery.CaseInsensitive {
		t.Fatalf("text query %+v", out.TextQuery) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	if out.Template != "document_search" || out.RequestClass != forensicrequest.GovernedAnalysis {
		t.Fatalf("template %q class %q", out.Template, out.RequestClass) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	if header := rec.Header().Get("X-Text-Intent"); !strings.HasPrefix(header, "state=ROUTED; about=written_in_documents; family=document; match=phrase; ms=") {
		t.Fatalf("header %q", header) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}

func TestTextIntentMapsEachFamilyToItsOperation(t *testing.T) {
	for family, template := range map[string]string{tiFamilyAudio: "audio_transcript_search", tiFamilyImage: "image_ocr_search", tiFamilyDocument: "document_search"} {
		m := newIntentModel(t, textIntentResult{Kind: tiKindText, Family: family, Match: tiMatchExact, Literal: "invoice"})
		out, _, routed, _ := routeIntent(t, m, textIntentRequest("did anyone say invoice"))
		if !routed || out.Template != template || out.TextQuery.MatchSemantic != forensictext.ExactPhrase {
			t.Fatalf("%s: routed=%v template=%q query=%+v", family, routed, out.Template, out.TextQuery) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
}

func TestTextIntentAnyFamilyKeepsTheLiteralAndLeavesTheOperationToTheExistingPath(t *testing.T) {
	m := newIntentModel(t, textIntentResult{Kind: tiKindText, Family: tiFamilyAny, Match: tiMatchPhrase, Literal: "اگوا شدگان"})
	out, family, routed, _ := routeIntent(t, m, textIntentRequest("کیا اگوا شدگان کا ذکر کہیں ہے؟"))
	if !routed || family != tiFamilyAny || out.Template != "" || out.TextQuery == nil || out.TextQuery.LiteralText != "اگوا شدگان" {
		t.Fatalf("routed=%v family=%q template=%q query=%+v", routed, family, out.Template, out.TextQuery) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}

func TestTextIntentChangesNothingForAQuestionThatIsNotATextSearch(t *testing.T) {
	m := newIntentModel(t, textIntentResult{Kind: tiKindOther, Family: tiFamilyNone, Match: tiMatchNone})
	req := textIntentRequest("How many calls did the subscriber make in March?")
	out, _, routed, rec := routeIntent(t, m, req)
	if routed || out.TextQuery != nil || out.Template != "" || out.Query != req.Query {
		t.Fatalf("a count question must keep its path: %+v", out) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	if header := rec.Header().Get("X-Text-Intent"); !strings.HasPrefix(header, "state=NOT_TEXT_SEARCH") {
		t.Fatalf("header %q", header) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}

func TestTextIntentRefusesALiteralThatIsNotInTheQuestion(t *testing.T) {
	// A model that corrected, translated or completed the analyst's words is refused: the search is for what the analyst typed.
	cases := map[string]textIntentResult{
		"completed":  {Kind: tiKindText, Family: tiFamilyDocument, Match: tiMatchPhrase, Literal: "wire transfer receipt"},
		"translated": {Kind: tiKindText, Family: tiFamilyAudio, Match: tiMatchPhrase, Literal: "kidnapped"},
		"invented":   {Kind: tiKindText, Family: tiFamilyDocument, Match: tiMatchPhrase, Literal: "bank account"},
		"reordered":  {Kind: tiKindText, Family: tiFamilyDocument, Match: tiMatchPhrase, Literal: "transfer wire"},
	}
	for name, result := range cases {
		m := newIntentModel(t, result)
		out, _, routed, rec := routeIntent(t, m, textIntentRequest("Which documents mention wire transfer?"))
		if routed || out.TextQuery != nil || out.Template != "" {
			t.Fatalf("%s: must not route: %+v", name, out) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
		if header := rec.Header().Get("X-Text-Intent"); !strings.Contains(header, "state=REJECTED") || !strings.Contains(header, "LITERAL_NOT_IN_QUESTION") {
			t.Fatalf("%s: header %q", name, header) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
}

func TestTextIntentMatchesTheLiteralCaseAndSpacingInsensitivelyButKeepsTheModelsCopy(t *testing.T) {
	m := newIntentModel(t, textIntentResult{Kind: tiKindText, Family: tiFamilyDocument, Match: tiMatchPhrase, Literal: "Wire   Transfer"})
	out, _, routed, _ := routeIntent(t, m, textIntentRequest("find wire transfer in the files"))
	if !routed || out.TextQuery.LiteralText != "Wire Transfer" {
		t.Fatalf("routed=%v query=%+v", routed, out.TextQuery) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}

func TestTextIntentAllWordsMayBeApartButAPhraseMayNot(t *testing.T) {
	question := "find files with invoice and also payment in them"
	words := newIntentModel(t, textIntentResult{Kind: tiKindText, Family: tiFamilyDocument, Match: tiMatchWords, Literal: "invoice payment"})
	if out, _, routed, _ := routeIntent(t, words, textIntentRequest(question)); !routed || out.TextQuery.MatchSemantic != forensictext.TokenSearch {
		t.Fatalf("all_words must accept words found apart in the question: %+v", out.TextQuery) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	phrase := newIntentModel(t, textIntentResult{Kind: tiKindText, Family: tiFamilyDocument, Match: tiMatchPhrase, Literal: "invoice payment"})
	if _, _, routed, _ := routeIntent(t, phrase, textIntentRequest(question)); routed {
		t.Fatal("a phrase the analyst did not type as one phrase must be refused") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}

func TestTextIntentNeverRoutesANationalIDNumber(t *testing.T) {
	for _, literal := range []string{"35202-1234567-1", "3520212345671", "35202 1234567 1"} {
		m := newIntentModel(t, textIntentResult{Kind: tiKindText, Family: tiFamilyDocument, Match: tiMatchExact, Literal: literal})
		out, _, routed, rec := routeIntent(t, m, textIntentRequest("which documents contain "+literal))
		if routed || out.TextQuery != nil {
			t.Fatalf("%q was routed", literal) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
		if !strings.Contains(rec.Header().Get("X-Text-Intent"), "NATIONAL_ID_LITERAL") {
			t.Fatalf("%q header %q", literal, rec.Header().Get("X-Text-Intent")) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
}

func TestTextIntentRefusesProposalsTheTextValidatorRefuses(t *testing.T) {
	for name, result := range map[string]textIntentResult{
		"one letter":       {Kind: tiKindText, Family: tiFamilyDocument, Match: tiMatchPhrase, Literal: "a"},
		"empty":            {Kind: tiKindText, Family: tiFamilyDocument, Match: tiMatchPhrase, Literal: "  "},
		"unknown family":   {Kind: tiKindText, Family: "video", Match: tiMatchPhrase, Literal: "invoice"},
		"unknown match":    {Kind: tiKindText, Family: tiFamilyDocument, Match: "regex", Literal: "invoice"},
		"unknown kind":     {Kind: "sql", Family: tiFamilyDocument, Match: tiMatchPhrase, Literal: "invoice"},
		"none as the kind": {Kind: tiKindText, Family: tiFamilyNone, Match: tiMatchPhrase, Literal: "invoice"},
	} {
		if _, _, why := textIntentValidate("is a invoice here", result); why == "" {
			t.Fatalf("%s must be refused", name) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
}

func TestTextIntentChangesNothingWhenTheModelFails(t *testing.T) {
	m := newIntentModel(t, textIntentResult{})
	m.status = http.StatusInternalServerError
	req := textIntentRequest("Which documents mention wire transfer?")
	out, _, routed, rec := routeIntent(t, m, req)
	if routed || out.TextQuery != nil || out.Template != "" {
		t.Fatalf("a failed call must change nothing: %+v", out) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	if header := rec.Header().Get("X-Text-Intent"); !strings.Contains(header, "state=UNAVAILABLE") || !strings.Contains(header, "HTTP_500") {
		t.Fatalf("header %q", header) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}

func TestTextIntentSkipsRequestsAlreadyTiedToAnOperationOrToEvidence(t *testing.T) {
	m := newIntentModel(t, textIntentResult{Kind: tiKindText, Family: tiFamilyDocument, Match: tiMatchPhrase, Literal: "invoice"})
	tied := map[string]func(*hybridQueryRequest){
		"template": func(r *hybridQueryRequest) { r.Template = "cdr_summary" },
		"text query": func(r *hybridQueryRequest) {
			r.TextQuery = &forensictext.Query{LiteralText: "x", MatchSemantic: forensictext.PhraseContains}
		},
		"exact term":     func(r *hybridQueryRequest) { r.ExactTerm = "x" },
		"target":         func(r *hybridQueryRequest) { r.Target = "+923001234567" },
		"evidence":       func(r *hybridQueryRequest) { r.EvidenceID = "ev-1" },
		"selected scope": func(r *hybridQueryRequest) { r.QueryScope.Kind = string(EvidenceScopeSelected) },
		"grouping":       func(r *hybridQueryRequest) { r.Group = &CanonicalGroupV1{} },
	}
	for name, tie := range tied {
		req := textIntentRequest("is there an invoice")
		tie(&req)
		if _, _, routed, _ := routeIntent(t, m, req); routed {
			t.Fatalf("%s: a request already tied to an operation must not be rerouted", name) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
	if len(m.bodies) != 0 {
		t.Fatalf("no model call is made for a request that cannot be rerouted; saw %d", len(m.bodies)) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	for _, class := range []forensicrequest.Class{forensicrequest.GeneralDomainKnowledge, forensicrequest.ProductHelp, forensicrequest.Unsupported, forensicrequest.ContextualFollowUp} {
		req := textIntentRequest("what is an invoice")
		req.RequestClass = class
		if _, ok := textIntentEligible(config{LocalAIURL: m.server.URL, SynthesisModel: "qwen3-4b"}, req); ok {
			t.Fatalf("class %s must not reach the step", class) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
}

func TestTextIntentLeavesAQuestionAboutANamedMediaFileToTheEngine(t *testing.T) {
	m := newIntentModel(t, textIntentResult{Kind: tiKindText, Family: tiFamilyAny, Match: tiMatchPhrase, Literal: "readings"})
	cfg := config{LocalAIURL: m.server.URL, SynthesisModel: "qwen3-4b"}
	for _, q := range []string{"What was said in the first ten seconds of interview-3.mp4?", "How many readings were taken from night-1.mp4 altogether?", "Which plates are in front-gate.JPG?"} {
		if _, ok := textIntentEligible(cfg, textIntentRequest(q)); ok {
			t.Fatalf("%q names a media file and must not reach the model", q) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
	for _, q := range []string{"Which documents mention report-12.pdf?", "Which recordings mention delivery?", "find the word invoice in the statements"} {
		if _, ok := textIntentEligible(cfg, textIntentRequest(q)); !ok {
			t.Fatalf("%q names no media file and must reach the model", q) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
	if len(m.bodies) != 0 {
		t.Fatalf("the eligibility check makes no model call; saw %d", len(m.bodies)) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}

func TestTextIntentSendsOnlyTheQuestionAndAStablePrompt(t *testing.T) {
	m := newIntentModel(t, textIntentResult{Kind: tiKindOther, Family: tiFamilyNone, Match: tiMatchNone})
	question := "Which documents mention wire transfer?"
	_, _, _, _ = routeIntent(t, m, textIntentRequest(question))
	_, _, _, _ = routeIntent(t, m, textIntentRequest("another question altogether"))
	if len(m.bodies) != 2 {
		t.Fatalf("bodies %d", len(m.bodies)) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	var first, second struct {
		Messages []struct{ Role, Content string } `json:"messages"`
		MaxTok   int                              `json:"max_tokens"`
		Format   struct {
			Type       string `json:"type"`
			JSONSchema struct {
				Strict bool `json:"strict"`
			} `json:"json_schema"`
		} `json:"response_format"`
	}
	if json.Unmarshal([]byte(m.bodies[0]), &first) != nil || json.Unmarshal([]byte(m.bodies[1]), &second) != nil {
		t.Fatal("request bodies are not JSON") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	if len(first.Messages) != 2 || first.Messages[0].Role != "system" || first.Messages[1].Role != "user" || first.Messages[1].Content != question {
		t.Fatalf("messages %+v", first.Messages) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	if first.Messages[0].Content != second.Messages[0].Content {
		t.Fatal("the system prompt must be identical for every message so a server can reuse it") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	if strings.Contains(first.Messages[0].Content, question) {
		t.Fatal("the system prompt must not carry the question") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	if first.Format.Type != "json_schema" || !first.Format.JSONSchema.Strict || first.MaxTok != textIntentMaxTokens {
		t.Fatalf("format %+v max_tokens %d", first.Format, first.MaxTok) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}

func TestTextIntentSchemaAndPromptAgree(t *testing.T) {
	schema, _ := json.Marshal(textIntentSchema())
	prompt := textIntentSystemPrompt()
	words := []string{tiAboutSpoken, tiAboutImages, tiAboutDocument, tiAboutAny, tiAboutPlate, tiMatchPhrase, tiMatchExact, tiMatchWords, tiMatchNone}
	words = append(words, tiAboutOther...)
	for _, word := range words {
		if !strings.Contains(string(schema), `"`+word+`"`) {
			t.Fatalf("schema lacks %q", word) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
		if !strings.Contains(prompt, word+" -") && !strings.Contains(prompt, word+":") {
			t.Fatalf("prompt does not explain %q", word) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
}

func TestTextIntentLiteralMustStandAloneInTheQuestion(t *testing.T) {
	text := func(literal string) textIntentResult {
		return textIntentResult{Kind: tiKindText, Family: tiFamilyDocument, Match: tiMatchPhrase, Literal: literal}
	}
	refused := map[string][2]string{
		"a number with its last digit cut off": {"search the files for 73775 169562 please", "73775 16956"},
		"a number cut at the front":            {"search the files for 73775 169562 please", "3775 169562"},
		"a word cut short":                     {"documents that mention laptops", "laptop"},
		"a word inside a longer word":          {"documents that mention wiretransfer", "wire"},
		"a word with a letter added":           {"documents that mention laptop", "laptops"},
	}
	for name, c := range refused {
		if _, _, why := textIntentValidate(c[0], text(c[1])); why != "LITERAL_NOT_IN_QUESTION" {
			t.Fatalf("%s: %q in %q gave %q", name, c[1], c[0], why) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
	accepted := map[string][2]string{
		"before punctuation":       {"did anyone say laptop, or a phone", "laptop"},
		"inside quotes":            {`find "wire transfer" please`, "wire transfer"},
		"a number with a dash":     {"documents with 0300-1234567?", "0300-1234567"},
		"urdu before its question": {"کیا ریکارڈنگ میں اگوا شدگان کا ذکر ہے؟", "اگوا شدگان"},
		"stands once of two":       {"laptops and also a laptop in the files", "laptop"},
	}
	for name, c := range accepted {
		if _, _, why := textIntentValidate(c[0], text(c[1])); why != "" {
			t.Fatalf("%s: %q in %q refused with %q", name, c[1], c[0], why) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
}

func TestTextIntentNeverSearchesForTheWholeMessage(t *testing.T) {
	for _, c := range [][2]string{
		{"hello, how are you?", "hello, how are you"},
		{"what does the audio transcript say?", "what does the audio transcript say"},
		{"wire transfer", "wire transfer"},
		{"Wire Transfer.  ", "wire transfer"},
		{"کیا یہ درست ہے؟", "کیا یہ درست ہے"},
	} {
		result := textIntentResult{Kind: tiKindText, Family: tiFamilyAudio, Match: tiMatchPhrase, Literal: c[1]}
		if _, _, why := textIntentValidate(c[0], result); why != "LITERAL_IS_WHOLE_MESSAGE" {
			t.Fatalf("%q gave %q", c[0], why) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
	result := textIntentResult{Kind: tiKindText, Family: tiFamilyAudio, Match: tiMatchPhrase, Literal: "wire transfer"}
	if _, _, why := textIntentValidate("did anyone say wire transfer?", result); why != "" {
		t.Fatalf("a literal inside a longer sentence must be accepted, got %q", why) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}

func TestTextIntentPromptNamesWhatIsNotATextSearchWithoutExamples(t *testing.T) {
	prompt := textIntentSystemPrompt()
	for _, need := range []string{"call_records", "message_records", "bank_transactions", "whom or what a question is about", "greeting", "Never the whole message", "unsure"} {
		if !strings.Contains(prompt, need) {
			t.Fatalf("the instruction no longer says %q", need) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
	if strings.Contains(prompt, "?") {
		t.Fatal("the instruction must not carry an example question") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}

func TestTextIntentAboutValuesMapToFamiliesAndEverythingElseIsNotASearch(t *testing.T) {
	for about, family := range map[string]string{tiAboutSpoken: tiFamilyAudio, tiAboutImages: tiFamilyImage, tiAboutDocument: tiFamilyDocument, tiAboutAny: tiFamilyAny} {
		got := textIntentWire{About: about, Match: tiMatchPhrase, Literal: "x"}.result()
		if got.Kind != tiKindText || got.Family != family || got.About != about {
			t.Fatalf("%s: %+v", about, got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
	for _, about := range tiAboutOther {
		got := textIntentWire{About: about, Match: tiMatchNone}.result()
		if got.Kind != tiKindOther || got.Family != tiFamilyNone {
			t.Fatalf("%s must not be a search: %+v", about, got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
		if _, _, why := textIntentValidate("How many voice calls does 923000001005 have?", textIntentWire{About: about, Match: tiMatchPhrase, Literal: "923000001005"}.result()); why != "NOT_TEXT_SEARCH" {
			t.Fatalf("%s with a literal still must not route, got %q", about, why) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
	if got := (textIntentWire{About: "invented", Literal: "x"}).result(); got.Kind != "" {
		t.Fatalf("a value outside the list must have no kind: %+v", got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	if _, _, why := textIntentValidate("find x", textIntentWire{About: "invented", Match: tiMatchPhrase, Literal: "x"}.result()); why != "UNKNOWN_KIND" {
		t.Fatalf("a value outside the list is refused, got %q", why) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}

func TestTextIntentWholeMessageIsRefusedEvenWithItsOwnQuestionMark(t *testing.T) {
	for _, c := range [][2]string{
		{"تصویر میں بہت حیرانی لکھا ہے؟", "تصویر میں بہت حیرانی لکھا ہے؟"},
		{"did anyone say wire transfer?", "did anyone say wire transfer?"},
	} {
		result := textIntentResult{Kind: tiKindText, Family: tiFamilyImage, Match: tiMatchPhrase, Literal: c[1]}
		if _, _, why := textIntentValidate(c[0], result); why != "LITERAL_IS_WHOLE_MESSAGE" {
			t.Fatalf("%q gave %q", c[0], why) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
}

func TestTextIntentSearchWithNoMatchTypeUsesThePhraseDefault(t *testing.T) {
	for _, match := range []string{tiMatchNone, ""} {
		got := textIntentWire{About: tiAboutDocument, Match: match, Literal: "50429"}.result()
		if got.Match != tiMatchPhrase {
			t.Fatalf("match %q must become the default, got %q", match, got.Match) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
		text, family, why := textIntentValidate("search the documents for 50429", got)
		if why != "" || family != tiFamilyDocument || text.MatchSemantic != forensictext.PhraseContains {
			t.Fatalf("a bare number must route as a phrase search: %v %v %v", text, family, why) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
	other := textIntentWire{About: "call_records", Match: tiMatchNone, Literal: "923000001005"}.result()
	if other.Kind != tiKindOther || other.Match != tiMatchNone {
		t.Fatalf("a value that is not a search keeps its match: %+v", other) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}

// The model says the question is about a plate the investigator typed, asked about in images or video. Any plate format is accepted: no list of formats is consulted.
// The model also quotes the words that name the photos or video (a_media_words); the server checks the quote is in the question.
func TestTextIntentRoutesAPlateQuestionWithAnyPlateFormat(t *testing.T) {
	for _, c := range [][3]string{
		{"Which photos show SYNT4O24?", "SYNT4O24", "photos"},
		{"is LEA-1234 in any of the images", "LEA-1234", "images"},
		{"does the plate ABC 123 appear in the video?", "ABC 123", "video"},
		{"any picture with 7788-KHI?", "7788-KHI", "picture"},
		{"which images show the plate XQ77Z9", "XQ77Z9", "images"},
		{"Check the footage for A 12345", "A 12345", "footage"},
	} {
		m := newIntentModel(t, textIntentResult{Kind: tiKindPlate, Family: tiFamilyPlate, Match: tiMatchNone, Literal: c[1], MediaWords: c[2]})
		out, family, routed, rec := routeIntent(t, m, textIntentRequest(c[0]))
		if !routed || family != tiFamilyPlate {
			t.Fatalf("%q: routed=%v family=%q header=%q", c[0], routed, family, rec.Header().Get("X-Text-Intent")) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
		want := plateKeyStrip.ReplaceAllString(strings.ToUpper(c[1]), "")
		if out.Plate != want || out.TextQuery != nil || out.Template != "" || out.RequestClass != forensicrequest.GovernedAnalysis {
			t.Fatalf("%q: plate %q (want %q) text %+v template %q", c[0], out.Plate, want, out.TextQuery, out.Template) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
		if header := rec.Header().Get("X-Text-Intent"); !strings.HasPrefix(header, "state=ROUTED; about=plate_in_images_or_video; family=plate") {
			t.Fatalf("%q: header %q", c[0], header) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
}

// The same discipline as for words: what the model proposes is checked against what the investigator typed.
func TestTextIntentRefusesAPlateThatIsNotTypedInTheQuestion(t *testing.T) {
	cases := map[string][3]string{
		"corrected":             {"Which photos show SYNT4O24?", "SYNT4024", "photos"},
		"completed":             {"Which photos show SYNT4O24?", "SYNT4O245", "photos"},
		"cut short":             {"Which photos show SYNT4O24?", "SYNT4O2", "photos"},
		"whole message":         {"SYNT4O24", "SYNT4O24", "photos"},
		"too short":             {"which images show ab", "ab", "images"},
		"too long":              {"which images show ABCDEFGHIJKLMNOP", "ABCDEFGHIJKLMNOP", "images"},
		"nothing proposed":      {"which images show the plate", "", "images"},
		"no media named":        {"When was the plate SYNT4O24 last seen?", "SYNT4O24", ""},
		"media words invented":  {"When was the plate SYNT4O24 last seen?", "SYNT4O24", "photographs"},
		"media words are plate": {"Which photos show SYNT4O24?", "SYNT4O24", "SYNT4O24"},
	}
	for name, c := range cases {
		m := newIntentModel(t, textIntentResult{Kind: tiKindPlate, Family: tiFamilyPlate, Match: tiMatchNone, Literal: c[1], MediaWords: c[2]})
		out, _, routed, rec := routeIntent(t, m, textIntentRequest(c[0]))
		if routed || out.Plate != "" || out.TextQuery != nil {
			t.Fatalf("%s: must not route: %+v", name, out) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
		if header := rec.Header().Get("X-Text-Intent"); !strings.Contains(header, "state=REJECTED") {
			t.Fatalf("%s: header %q", name, header) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
	for name, why := range map[string]string{"no media named": "NO_MEDIA_NAMED", "media words invented": "MEDIA_WORDS_NOT_IN_QUESTION", "media words are plate": "MEDIA_WORDS_NOT_IN_QUESTION"} {
		c := cases[name]
		if _, got := textIntentPlateKey(c[0], textIntentResult{Kind: tiKindPlate, Literal: c[1], MediaWords: c[2]}); got != why {
			t.Fatalf("%s: got %q want %q", name, got, why) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
}

func TestTextIntentRefusesAPlateThatIsPartOfAFileNameButNotOneNextToIt(t *testing.T) {
	question := "On how many frames of pexels-5293898-hd.mp4 was a plate read?"
	for _, literal := range []string{"5293898-hd", "pexels-5293898-hd", "pexels-5293898-hd.mp4", "hd"} {
		if _, why := textIntentPlateKey(question, textIntentResult{Kind: tiKindPlate, Literal: literal, MediaWords: "frames"}); why == "" {
			t.Fatalf("%q is part of a file name and was accepted as a plate", literal) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
	if key, why := textIntentPlateKey("Which photos show the plate LEA-1234, taken near gate.jpg?", textIntentResult{Kind: tiKindPlate, Literal: "LEA-1234", MediaWords: "photos"}); why != "" || key != "LEA1234" {
		t.Fatalf("a plate typed next to a file name was refused: %q %q", key, why) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}

func TestTextIntentRefusesAPieceOfAFileNameAsWordsToFindButAllowsTheWholeName(t *testing.T) {
	result := func(literal string) textIntentResult {
		return textIntentResult{Kind: tiKindText, Family: tiFamilyAny, Match: tiMatchPhrase, Literal: literal}
	}
	question := "What is said in clip-12.mp4 about delivery?"
	if _, _, why := textIntentValidate(question, result("clip-12")); why != "LITERAL_IN_FILE_NAME" {
		t.Fatalf("a piece of a file name was taken as the words to find: %q", why) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	if _, _, why := textIntentValidate(question, result("delivery")); why != "" {
		t.Fatalf("a search for a word was refused: %q", why) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	if _, _, why := textIntentValidate("Which documents mention report-12.pdf and when?", result("report-12.pdf")); why != "" {
		t.Fatalf("a search of the documents for the name of a document was refused: %q", why) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	if _, _, why := textIntentValidate("On how many frames of clip-12.mp4 was a plate read?", result("clip-12.mp4")); why != "LITERAL_IN_FILE_NAME" {
		t.Fatalf("the name of a video was taken as the words to find: %q", why) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}

func TestTextIntentAsksForEveryMatchAndNeverFewerThanTheCallerAsked(t *testing.T) {
	for current, want := range map[int]int{0: 20, 1: 20, 5: 20, 19: 20, 20: 20, 50: 50} {
		if got := textIntentResultLimit(current); got != want {
			t.Fatalf("a limit of %d became %d, want %d", current, got, want) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
}

func TestTextIntentDoesNotSearchForTheNameOfALanguageAskedAsTheLanguageOfRecordings(t *testing.T) {
	text := func(literal string) textIntentResult {
		return textIntentResult{Kind: tiKindText, Family: tiFamilyAudio, Match: tiMatchPhrase, Literal: literal}
	}
	// the language of the recordings is a field of the audio files, not a word to find in what was said
	for _, c := range [][2]string{
		{"Which audio recordings are in Pashto?", "Pashto"},
		{"Which audio recordings are in Urdu?", "Urdu"},
		{"How many audio recordings are spoken in English?", "English"},
		{"Which recordings are in Pashto?", "in Pashto"},
		{"List the Punjabi recordings", "Punjabi"},
		{"Who speaks Urdu in the recordings?", "Urdu"},
		{"Which recording has an Urdu speaker?", "Urdu"},
		// measured 2026-11-16 on the fresh-wording run: a language name with no cue for words said, alone or with request words around it
		{"List the recordings where Urdu is spoken.", "Urdu"},
		{"What is the longest English recording?", "English recording"},
		{"What is the longest English recording?", "longest English recording"},
		{"How long is the shortest Pashto recording, in seconds?", "shortest Pashto recording"},
		{"How long is the shortest Pashto recording, in seconds?", "Pashto"},
		{"Which voice notes are spoken in English?", "English"},
	} {
		if _, _, why := textIntentValidate(c[0], text(c[1])); why != "LITERAL_NAMES_A_LANGUAGE" {
			t.Fatalf("%q: the language name %q was taken as words to find: %q", c[0], c[1], why) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
	// a language name the analyst asked to be SAID, or put in quotes, is a search for the word
	for _, c := range [][2]string{
		{"Which recordings mention Pashto?", "Pashto"},
		{"Which recordings have someone say the word Urdu?", "Urdu"},
		{"Which audio recordings say 'Urdu'?", "Urdu"},
		{"Find the recordings where \"English\" is said", "English"},
		{"Which recordings mention delivery?", "delivery"},
		// another word in the literal, or a cue for words said, keeps it a search
		{"find the audio where dari kam is mentioned", "dari kam"},
		{"Search the recordings for Urdu", "Urdu"},
		{"Where is Pashto mentioned in the audio?", "Pashto"},
		{"Which of the voice notes talk about English?", "English"},
		{"Is the word Urdu said in any recording?", "Urdu"},
		{"Which recordings contain the phrase Urdu newspaper?", "Urdu newspaper"},
	} {
		if _, _, why := textIntentValidate(c[0], text(c[1])); why != "" {
			t.Fatalf("%q: the search for %q was refused: %q", c[0], c[1], why) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
}

func TestTextIntentRefusesALiteralThatOnlyDescribesTheRequest(t *testing.T) {
	text := func(literal string) textIntentResult {
		return textIntentResult{Kind: tiKindText, Family: tiFamilyAudio, Match: tiMatchPhrase, Literal: literal}
	}
	for _, c := range [][2]string{
		{"Which video is the longest?", "longest"},
		{"Which videos have speech in them?", "speech"},
		{"Which video has the most text visible in it?", "text visible"},
		{"What is the longest video in the case?", "longest video"},
		{"Which clip shows the largest amount of readable text?", "clip"},
		{"What is the total running time of all the videos, in minutes?", "running time of all the videos"},
		{"Which videos are longer than 30 seconds?", "longer than 30 seconds"},
	} {
		if _, _, why := textIntentValidate(c[0], text(c[1])); why != "LITERAL_DESCRIBES_THE_REQUEST" {
			t.Fatalf("%q: the literal %q was not refused: %q", c[0], c[1], why) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
	// a literal with a word of its own in it is a search, whatever else it says
	for _, c := range [][2]string{
		{"Which videos have someone say delivery?", "delivery"},
		{"Which recordings mention wire transfer?", "wire transfer"},
		{"Which video shows the sign for the pharmacy?", "pharmacy"},
		{"Which documents mention the longest queue?", "longest queue"},
	} {
		if _, _, why := textIntentValidate(c[0], text(c[1])); why != "" {
			t.Fatalf("%q: the search for %q was refused: %q", c[0], c[1], why) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
	// a plate is never the word plate, vehicle or number
	for _, literal := range []string{"plate", "vehicle", "number plate", "plates"} {
		if _, why := textIntentPlateKey("On how many frames of the video was a plate read, photo evidence?", textIntentResult{Kind: tiKindPlate, Literal: literal, MediaWords: "video"}); why == "" {
			t.Fatalf("the word %q was accepted as a plate", literal) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
}

func TestTextIntentPlateAboutValueMapsToThePlateKind(t *testing.T) {
	got := textIntentWire{About: tiAboutPlate, Match: tiMatchNone, Literal: "SYNT9150"}.result()
	if got.Kind != tiKindPlate || got.Family != tiFamilyPlate || got.About != tiAboutPlate {
		t.Fatalf("%+v", got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	// a plate question must never fall into the words-in-text validator
	if _, _, why := textIntentValidate("which photos show SYNT9150", got); why == "" {
		t.Fatal("the text validator must not accept a plate result") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}

// "a_media_words" must be the first field of the reply: the model quotes the words that name photos or video before it chooses a subject. The grammar emits fields in alphabetical order.
func TestTextIntentSchemaPutsTheMediaQuoteBeforeTheSubject(t *testing.T) {
	encoded, _ := json.Marshal(textIntentSchema())
	text := string(encoded)
	media, about := strings.Index(text, `"a_media_words":{`), strings.Index(text, `"about":{`)
	if media < 0 || about < 0 || media > about {
		t.Fatalf("a_media_words (%d) must come before about (%d) in %s", media, about, text) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	if !strings.Contains(text, `"required":["a_media_words","about","match","literal"]`) {
		t.Fatalf("the field must be required: %s", text) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	if !strings.Contains(textIntentSystemPrompt(), "a_media_words - fill this first") {
		t.Fatal("the instruction must explain the field") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}

func TestTextIntentMediaQuoteMayBeAnInflectedFormOfAWordThatIsInTheMessage(t *testing.T) {
	grounded := [][2]string{
		{"show me every photo of the vehicle with plate x", "photos"},
		{"show me every photograph of the vehicle", "photographs"},
		{"check the footage for it", "footage"},
		{"find the image with it", "images"},
		{"which pictures show it", "pictures"},
		{"ویڈیو میں گاڑی کون سی ہے", "ویڈیو"},
	}
	for _, c := range grounded {
		if !tiQuoteGrounded(c[0], c[1]) {
			t.Fatalf("%q should be grounded in %q", c[1], c[0]) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
	refused := [][2]string{
		{"when was the plate last seen", "photographs"},
		{"when was the plate last seen", "photos"},
		{"how many times was it seen by the cameras", "images"},
		{"the word subphotography appears here", "photos"},
		{"it is a pho", "photos"},
		{"anything", ""},
	}
	for _, c := range refused {
		if tiQuoteGrounded(c[0], c[1]) {
			t.Fatalf("%q must not be grounded in %q", c[1], c[0]) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
}
