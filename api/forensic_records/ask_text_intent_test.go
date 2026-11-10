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
	if result.Kind == tiKindText {
		for value, family := range tiAboutText {
			if family == result.Family {
				about = value
			}
		}
	}
	return textIntentWire{About: about, Match: result.Match, Literal: result.Literal}
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
		t.Fatal("the step adds a model call; it must default to off")
	}
	t.Setenv(textIntentEnv, "true")
	if !textIntentEnabled() {
		t.Fatal("an explicit true must enable it")
	}
}

func TestTextIntentRoutesASearchForPhraseInDocumentsToTheStructuredQuery(t *testing.T) {
	m := newIntentModel(t, textIntentResult{Kind: tiKindText, Family: tiFamilyDocument, Match: tiMatchPhrase, Literal: "wire transfer"})
	out, family, routed, rec := routeIntent(t, m, textIntentRequest("Which documents mention wire transfer?"))
	if !routed || family != tiFamilyDocument {
		t.Fatalf("routed=%v family=%q", routed, family)
	}
	if out.TextQuery == nil || out.TextQuery.LiteralText != "wire transfer" || out.TextQuery.MatchSemantic != forensictext.PhraseContains || !out.TextQuery.CaseInsensitive {
		t.Fatalf("text query %+v", out.TextQuery)
	}
	if out.Template != "document_search" || out.RequestClass != forensicrequest.GovernedAnalysis {
		t.Fatalf("template %q class %q", out.Template, out.RequestClass)
	}
	if header := rec.Header().Get("X-Text-Intent"); !strings.HasPrefix(header, "state=ROUTED; about=written_in_documents; family=document; match=phrase; ms=") {
		t.Fatalf("header %q", header)
	}
}

func TestTextIntentMapsEachFamilyToItsOperation(t *testing.T) {
	for family, template := range map[string]string{tiFamilyAudio: "audio_transcript_search", tiFamilyImage: "image_ocr_search", tiFamilyDocument: "document_search"} {
		m := newIntentModel(t, textIntentResult{Kind: tiKindText, Family: family, Match: tiMatchExact, Literal: "invoice"})
		out, _, routed, _ := routeIntent(t, m, textIntentRequest("did anyone say invoice"))
		if !routed || out.Template != template || out.TextQuery.MatchSemantic != forensictext.ExactPhrase {
			t.Fatalf("%s: routed=%v template=%q query=%+v", family, routed, out.Template, out.TextQuery)
		}
	}
}

func TestTextIntentAnyFamilyKeepsTheLiteralAndLeavesTheOperationToTheExistingPath(t *testing.T) {
	m := newIntentModel(t, textIntentResult{Kind: tiKindText, Family: tiFamilyAny, Match: tiMatchPhrase, Literal: "اگوا شدگان"})
	out, family, routed, _ := routeIntent(t, m, textIntentRequest("کیا اگوا شدگان کا ذکر کہیں ہے؟"))
	if !routed || family != tiFamilyAny || out.Template != "" || out.TextQuery == nil || out.TextQuery.LiteralText != "اگوا شدگان" {
		t.Fatalf("routed=%v family=%q template=%q query=%+v", routed, family, out.Template, out.TextQuery)
	}
}

func TestTextIntentChangesNothingForAQuestionThatIsNotATextSearch(t *testing.T) {
	m := newIntentModel(t, textIntentResult{Kind: tiKindOther, Family: tiFamilyNone, Match: tiMatchNone})
	req := textIntentRequest("How many calls did the subscriber make in March?")
	out, _, routed, rec := routeIntent(t, m, req)
	if routed || out.TextQuery != nil || out.Template != "" || out.Query != req.Query {
		t.Fatalf("a count question must keep its path: %+v", out)
	}
	if header := rec.Header().Get("X-Text-Intent"); !strings.HasPrefix(header, "state=NOT_TEXT_SEARCH") {
		t.Fatalf("header %q", header)
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
			t.Fatalf("%s: must not route: %+v", name, out)
		}
		if header := rec.Header().Get("X-Text-Intent"); !strings.Contains(header, "state=REJECTED") || !strings.Contains(header, "LITERAL_NOT_IN_QUESTION") {
			t.Fatalf("%s: header %q", name, header)
		}
	}
}

func TestTextIntentMatchesTheLiteralCaseAndSpacingInsensitivelyButKeepsTheModelsCopy(t *testing.T) {
	m := newIntentModel(t, textIntentResult{Kind: tiKindText, Family: tiFamilyDocument, Match: tiMatchPhrase, Literal: "Wire   Transfer"})
	out, _, routed, _ := routeIntent(t, m, textIntentRequest("find wire transfer in the files"))
	if !routed || out.TextQuery.LiteralText != "Wire Transfer" {
		t.Fatalf("routed=%v query=%+v", routed, out.TextQuery)
	}
}

func TestTextIntentAllWordsMayBeApartButAPhraseMayNot(t *testing.T) {
	question := "find files with invoice and also payment in them"
	words := newIntentModel(t, textIntentResult{Kind: tiKindText, Family: tiFamilyDocument, Match: tiMatchWords, Literal: "invoice payment"})
	if out, _, routed, _ := routeIntent(t, words, textIntentRequest(question)); !routed || out.TextQuery.MatchSemantic != forensictext.TokenSearch {
		t.Fatalf("all_words must accept words found apart in the question: %+v", out.TextQuery)
	}
	phrase := newIntentModel(t, textIntentResult{Kind: tiKindText, Family: tiFamilyDocument, Match: tiMatchPhrase, Literal: "invoice payment"})
	if _, _, routed, _ := routeIntent(t, phrase, textIntentRequest(question)); routed {
		t.Fatal("a phrase the analyst did not type as one phrase must be refused")
	}
}

func TestTextIntentNeverRoutesANationalIDNumber(t *testing.T) {
	for _, literal := range []string{"35202-1234567-1", "3520212345671", "35202 1234567 1"} {
		m := newIntentModel(t, textIntentResult{Kind: tiKindText, Family: tiFamilyDocument, Match: tiMatchExact, Literal: literal})
		out, _, routed, rec := routeIntent(t, m, textIntentRequest("which documents contain "+literal))
		if routed || out.TextQuery != nil {
			t.Fatalf("%q was routed", literal)
		}
		if !strings.Contains(rec.Header().Get("X-Text-Intent"), "NATIONAL_ID_LITERAL") {
			t.Fatalf("%q header %q", literal, rec.Header().Get("X-Text-Intent"))
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
			t.Fatalf("%s must be refused", name)
		}
	}
}

func TestTextIntentChangesNothingWhenTheModelFails(t *testing.T) {
	m := newIntentModel(t, textIntentResult{})
	m.status = http.StatusInternalServerError
	req := textIntentRequest("Which documents mention wire transfer?")
	out, _, routed, rec := routeIntent(t, m, req)
	if routed || out.TextQuery != nil || out.Template != "" {
		t.Fatalf("a failed call must change nothing: %+v", out)
	}
	if header := rec.Header().Get("X-Text-Intent"); !strings.Contains(header, "state=UNAVAILABLE") || !strings.Contains(header, "HTTP_500") {
		t.Fatalf("header %q", header)
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
			t.Fatalf("%s: a request already tied to an operation must not be rerouted", name)
		}
	}
	if len(m.bodies) != 0 {
		t.Fatalf("no model call is made for a request that cannot be rerouted; saw %d", len(m.bodies))
	}
	for _, class := range []forensicrequest.Class{forensicrequest.GeneralDomainKnowledge, forensicrequest.ProductHelp, forensicrequest.Unsupported, forensicrequest.ContextualFollowUp} {
		req := textIntentRequest("what is an invoice")
		req.RequestClass = class
		if _, ok := textIntentEligible(config{LocalAIURL: m.server.URL, SynthesisModel: "qwen3-4b"}, req); ok {
			t.Fatalf("class %s must not reach the step", class)
		}
	}
}

func TestTextIntentSendsOnlyTheQuestionAndAStablePrompt(t *testing.T) {
	m := newIntentModel(t, textIntentResult{Kind: tiKindOther, Family: tiFamilyNone, Match: tiMatchNone})
	question := "Which documents mention wire transfer?"
	_, _, _, _ = routeIntent(t, m, textIntentRequest(question))
	_, _, _, _ = routeIntent(t, m, textIntentRequest("another question altogether"))
	if len(m.bodies) != 2 {
		t.Fatalf("bodies %d", len(m.bodies))
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
		t.Fatal("request bodies are not JSON")
	}
	if len(first.Messages) != 2 || first.Messages[0].Role != "system" || first.Messages[1].Role != "user" || first.Messages[1].Content != question {
		t.Fatalf("messages %+v", first.Messages)
	}
	if first.Messages[0].Content != second.Messages[0].Content {
		t.Fatal("the system prompt must be identical for every message so a server can reuse it")
	}
	if strings.Contains(first.Messages[0].Content, question) {
		t.Fatal("the system prompt must not carry the question")
	}
	if first.Format.Type != "json_schema" || !first.Format.JSONSchema.Strict || first.MaxTok != textIntentMaxTokens {
		t.Fatalf("format %+v max_tokens %d", first.Format, first.MaxTok)
	}
}

func TestTextIntentSchemaAndPromptAgree(t *testing.T) {
	schema, _ := json.Marshal(textIntentSchema())
	prompt := textIntentSystemPrompt()
	words := []string{tiAboutSpoken, tiAboutImages, tiAboutDocument, tiAboutAny, tiMatchPhrase, tiMatchExact, tiMatchWords, tiMatchNone}
	words = append(words, tiAboutOther...)
	for _, word := range words {
		if !strings.Contains(string(schema), `"`+word+`"`) {
			t.Fatalf("schema lacks %q", word)
		}
		if !strings.Contains(prompt, word+" -") && !strings.Contains(prompt, word+":") {
			t.Fatalf("prompt does not explain %q", word)
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
			t.Fatalf("%s: %q in %q gave %q", name, c[1], c[0], why)
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
			t.Fatalf("%s: %q in %q refused with %q", name, c[1], c[0], why)
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
			t.Fatalf("%q gave %q", c[0], why)
		}
	}
	result := textIntentResult{Kind: tiKindText, Family: tiFamilyAudio, Match: tiMatchPhrase, Literal: "wire transfer"}
	if _, _, why := textIntentValidate("did anyone say wire transfer?", result); why != "" {
		t.Fatalf("a literal inside a longer sentence must be accepted, got %q", why)
	}
}

func TestTextIntentPromptNamesWhatIsNotATextSearchWithoutExamples(t *testing.T) {
	prompt := textIntentSystemPrompt()
	for _, need := range []string{"call_records", "message_records", "bank_transactions", "whom or what a question is about", "greeting", "Never the whole message", "unsure"} {
		if !strings.Contains(prompt, need) {
			t.Fatalf("the instruction no longer says %q", need)
		}
	}
	if strings.Contains(prompt, "?") {
		t.Fatal("the instruction must not carry an example question")
	}
}

func TestTextIntentAboutValuesMapToFamiliesAndEverythingElseIsNotASearch(t *testing.T) {
	for about, family := range map[string]string{tiAboutSpoken: tiFamilyAudio, tiAboutImages: tiFamilyImage, tiAboutDocument: tiFamilyDocument, tiAboutAny: tiFamilyAny} {
		got := textIntentWire{About: about, Match: tiMatchPhrase, Literal: "x"}.result()
		if got.Kind != tiKindText || got.Family != family || got.About != about {
			t.Fatalf("%s: %+v", about, got)
		}
	}
	for _, about := range tiAboutOther {
		got := textIntentWire{About: about, Match: tiMatchNone}.result()
		if got.Kind != tiKindOther || got.Family != tiFamilyNone {
			t.Fatalf("%s must not be a search: %+v", about, got)
		}
		if _, _, why := textIntentValidate("How many voice calls does 923000001005 have?", textIntentWire{About: about, Match: tiMatchPhrase, Literal: "923000001005"}.result()); why != "NOT_TEXT_SEARCH" {
			t.Fatalf("%s with a literal still must not route, got %q", about, why)
		}
	}
	if got := (textIntentWire{About: "invented", Literal: "x"}).result(); got.Kind != "" {
		t.Fatalf("a value outside the list must have no kind: %+v", got)
	}
	if _, _, why := textIntentValidate("find x", textIntentWire{About: "invented", Match: tiMatchPhrase, Literal: "x"}.result()); why != "UNKNOWN_KIND" {
		t.Fatalf("a value outside the list is refused, got %q", why)
	}
}

func TestTextIntentWholeMessageIsRefusedEvenWithItsOwnQuestionMark(t *testing.T) {
	for _, c := range [][2]string{
		{"تصویر میں بہت حیرانی لکھا ہے؟", "تصویر میں بہت حیرانی لکھا ہے؟"},
		{"did anyone say wire transfer?", "did anyone say wire transfer?"},
	} {
		result := textIntentResult{Kind: tiKindText, Family: tiFamilyImage, Match: tiMatchPhrase, Literal: c[1]}
		if _, _, why := textIntentValidate(c[0], result); why != "LITERAL_IS_WHOLE_MESSAGE" {
			t.Fatalf("%q gave %q", c[0], why)
		}
	}
}

func TestTextIntentSearchWithNoMatchTypeUsesThePhraseDefault(t *testing.T) {
	for _, match := range []string{tiMatchNone, ""} {
		got := textIntentWire{About: tiAboutDocument, Match: match, Literal: "50429"}.result()
		if got.Match != tiMatchPhrase {
			t.Fatalf("match %q must become the default, got %q", match, got.Match)
		}
		text, family, why := textIntentValidate("search the documents for 50429", got)
		if why != "" || family != tiFamilyDocument || text.MatchSemantic != forensictext.PhraseContains {
			t.Fatalf("a bare number must route as a phrase search: %v %v %v", text, family, why)
		}
	}
	other := textIntentWire{About: "call_records", Match: tiMatchNone, Literal: "923000001005"}.result()
	if other.Kind != tiKindOther || other.Match != tiMatchNone {
		t.Fatalf("a value that is not a search keeps its match: %+v", other)
	}
}
