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
)

// fakeModel answers /v1/chat/completions with a fixed completion and records every request body it saw.
type fakeModel struct {
	server  *httptest.Server
	bodies  []string
	content string
	status  int
	finish  string
}

func newFakeModel(t *testing.T, class, reply string) *fakeModel {
	t.Helper()
	content, _ := json.Marshal(map[string]string{"class": class, "reply": reply})
	f := &fakeModel{content: string(content), status: http.StatusOK, finish: "stop"}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		f.bodies = append(f.bodies, string(body))
		if f.status != http.StatusOK {
			w.WriteHeader(f.status)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []map[string]any{{"message": map[string]string{"content": f.content}, "finish_reason": f.finish}}})
	}))
	t.Cleanup(f.server.Close)
	return f
}

func frontDoorTestRequest(query string) hybridQueryRequest {
	req := hybridQueryRequest{TenantID: "default", CollectionID: "demo", Query: query, SynthesisModel: "qwen3-4b"}
	req.RequestClass = forensicrequest.Class(semanticRequestClass(req))
	return req
}

func ask(t *testing.T, f *fakeModel, query string) (hybridQueryResponse, bool, bool) {
	t.Helper()
	return conversationFrontDoor(t.Context(), httptest.NewRecorder(), config{LocalAIURL: f.server.URL, SynthesisModel: "qwen3-4b"}, frontDoorTestRequest(query), time.Now())
}

func TestFrontDoorIsOffByDefault(t *testing.T) {
	if frontDoorEnabled() {
		t.Fatal("the front door adds a model call; it must default to off")
	}
	t.Setenv(frontDoorEnv, "true")
	if !frontDoorEnabled() {
		t.Fatal("an explicit true must enable it")
	}
}

func TestFrontDoorAnswersConversationWithoutTouchingTheData(t *testing.T) {
	f := newFakeModel(t, fdConversation, "Hello! I can help you look into the evidence in your case.")
	resp, handled, promote := ask(t, f, "hey, how's it going?")
	if !handled || promote {
		t.Fatalf("a greeting must be answered by the front door: handled=%v promote=%v", handled, promote)
	}
	if len(resp.Route) != 1 || resp.Route[0] != "terminal" {
		t.Fatalf("route %v", resp.Route)
	}
	text, _ := resp.Enterprise["executive_answer"].(string)
	if !strings.HasPrefix(text, "Hello!") {
		t.Fatalf("analyst text %q", text)
	}
	if resp.Answer["case_specific_facts"] == nil || len(resp.Answer["case_specific_facts"].([]any)) != 0 {
		t.Fatal("a conversational reply must carry no case facts")
	}
	if resp.Planner["front_door"] == nil {
		t.Fatal("the audit must travel with the response so the switch can be measured")
	}
}

func TestFrontDoorConceptIsLabelledGeneral(t *testing.T) {
	f := newFakeModel(t, fdConcept, "Triangulation estimates a position from the signal of several towers.")
	resp, handled, _ := ask(t, f, "How does triangulation work with mobile towers?")
	if !handled || resp.RequestClass != forensicrequest.GeneralDomainKnowledge {
		t.Fatalf("handled=%v class=%s", handled, resp.RequestClass)
	}
	if got := resp.Enterprise["summary"]; !strings.Contains(got.(string), "not from your case") {
		t.Fatalf("a concept answer must say it is general, got %q", got)
	}
}

func TestFrontDoorDataQuestionFallsThroughToThePlanner(t *testing.T) {
	f := newFakeModel(t, fdDataQuestion, "")
	_, handled, promote := ask(t, f, "Which numbers did the owner of the first phone contact?")
	if handled || !promote {
		t.Fatalf("a data question must reach the governed path: handled=%v promote=%v", handled, promote)
	}
}

func TestFrontDoorNeverLetsAChatClassSwallowAnAnalyticalQuestion(t *testing.T) {
	f := newFakeModel(t, fdConcept, "Calls are records of phone activity.")
	_, handled, promote := ask(t, f, "What is a CDR and how many do we have?")
	if handled || !promote {
		t.Fatalf("\"how many\" asks something of the evidence: handled=%v promote=%v", handled, promote)
	}
}

func TestFrontDoorDeclineIsRespectedEvenWithAnalyticalWords(t *testing.T) {
	f := newFakeModel(t, fdDecline, "I can't guess. I can count the matching records if you tell me the exact filter.")
	resp, handled, _ := ask(t, f, "Just guess: how many calls were made at night?")
	if !handled || resp.RequestClass != forensicrequest.Unsupported {
		t.Fatalf("handled=%v class=%s", handled, resp.RequestClass)
	}
	if _, has := resp.Enterprise["clarification"]; has {
		t.Fatal("a decline is an answer, not a request for more input")
	}
}

func TestFrontDoorRefusesAReplyThatStatesACaseFact(t *testing.T) {
	for name, reply := range map[string]string{
		"long number":  "The suspect called 923001110001 three times.",
		"count":        "There are 8,642 records in this case.",
		"small count":  "There are 5 tower records in this case.",
		"file name":    "That is described in report.pdf.",
		"found phrase": "I found 3 calls from that number.",
	} {
		f := newFakeModel(t, fdConversation, reply)
		resp, handled, _ := ask(t, f, "good morning")
		if !handled {
			t.Fatalf("%s: a rejected reply is replaced, it does not fall into the data path", name)
		}
		text, _ := resp.Enterprise["executive_answer"].(string)
		if text != frontDoorSafeFallback {
			t.Fatalf("%s: the leaking reply must be replaced, got %q", name, text)
		}
		audit := resp.Planner["front_door"].(FrontDoorAuditV1)
		if audit.State != "REPLY_REJECTED" || audit.RejectedReason == "" {
			t.Fatalf("%s: the rejection must be audited: %+v", name, audit)
		}
	}
}

func TestFrontDoorAllowsHarmlessNumbersInGeneralAnswers(t *testing.T) {
	for _, reply := range []string{
		"SHA-256 produces a 256-bit fingerprint, so any change to a file changes the hash.",
		"Mobile networks since 2G, 3G and 4G all log cell identifiers.",
		"Chain of custody records who held the evidence, in 3 steps: collect, store, transfer.",
	} {
		if cleaned, why := frontDoorCleanReply(reply); why != "" || cleaned == "" {
			t.Errorf("%q must pass, rejected for %s", reply, why)
		}
	}
}

func TestFrontDoorChangesNothingWhenTheModelFails(t *testing.T) {
	f := newFakeModel(t, fdConversation, "Hi")
	f.status = http.StatusInternalServerError
	if _, handled, promote := ask(t, f, "hello"); handled || promote {
		t.Fatalf("a failed call must leave the request exactly as the switch-off path would: handled=%v promote=%v", handled, promote)
	}
	f = newFakeModel(t, fdConversation, "Hi")
	f.finish = "length"
	if _, handled, promote := ask(t, f, "hello"); handled || promote {
		t.Fatal("a truncated completion must be ignored")
	}
	f = newFakeModel(t, "NOT_A_CLASS", "x")
	if _, handled, promote := ask(t, f, "hello"); handled || promote {
		t.Fatal("an unknown class must be ignored")
	}
}

func TestFrontDoorSkipsMessagesTiedToEvidence(t *testing.T) {
	f := newFakeModel(t, fdConversation, "Hi")
	cfg := config{LocalAIURL: f.server.URL, SynthesisModel: "qwen3-4b"}
	for name, req := range map[string]hybridQueryRequest{
		"template":  {Query: "hello", Template: "cdr_summary", SynthesisModel: "qwen3-4b"},
		"selected":  {Query: "hello", EvidenceID: "e1", SynthesisModel: "qwen3-4b"},
		"too long":  {Query: strings.Repeat("a ", 400), SynthesisModel: "qwen3-4b"},
		"no model":  {Query: "hello"},
		"embedding": {Query: "hello", SynthesisModel: "text-embedding-model"},
	} {
		req.RequestClass = forensicrequest.GovernedAnalysis
		localCfg := cfg
		if name == "no model" {
			localCfg.SynthesisModel = ""
		}
		if _, ok := frontDoorEligible(localCfg, req, req.RequestClass); ok {
			t.Errorf("%s must not reach the model", name)
		}
	}
	if len(f.bodies) != 0 {
		t.Fatal("no model call may be made for an ineligible message")
	}
	if _, ok := frontDoorEligible(config{}, hybridQueryRequest{Query: "hello"}, forensicrequest.GovernedAnalysis); ok {
		t.Fatal("without a model server there is nothing to ask")
	}
}

func TestFrontDoorLeavesUnsupportedAndFollowUpsToTheirOwnRules(t *testing.T) {
	for _, class := range []forensicrequest.Class{forensicrequest.Unsupported, forensicrequest.ContextualFollowUp, forensicrequest.Clarify} {
		if _, ok := frontDoorEligible(config{LocalAIURL: "http://x", SynthesisModel: "qwen3-4b"}, hybridQueryRequest{Query: "hello"}, class); ok {
			t.Errorf("%s keeps its deterministic handling", class)
		}
	}
}

// The model call's start must not depend on the message, so a server that reuses the start of a prompt reads it once.
func TestFrontDoorPromptIsStableAndCarriesNoQuestion(t *testing.T) {
	a, b := frontDoorSystemPrompt(), frontDoorSystemPrompt()
	if a != b {
		t.Fatal("the system prompt must be byte-identical across calls")
	}
	f := newFakeModel(t, fdConversation, "Hi there.")
	_, _, _ = ask(t, f, "zebra quartz morning")
	_, _, _ = ask(t, f, "violet anchor evening")
	if len(f.bodies) != 2 {
		t.Fatalf("expected two calls, got %d", len(f.bodies))
	}
	for _, body := range f.bodies {
		var parsed struct {
			Messages []struct{ Role, Content string } `json:"messages"`
			Format   map[string]any                   `json:"response_format"`
		}
		if err := json.Unmarshal([]byte(body), &parsed); err != nil {
			t.Fatal(err)
		}
		if parsed.Messages[0].Role != "system" || parsed.Messages[0].Content != a {
			t.Fatal("the system message must be the fixed prompt")
		}
		if strings.Contains(parsed.Messages[0].Content, "zebra") || strings.Contains(parsed.Messages[0].Content, "violet") {
			t.Fatal("nothing question-specific may appear before the question")
		}
		schema := parsed.Format["json_schema"].(map[string]any)
		if schema["strict"] != true {
			t.Fatal("the reply must be grammar-constrained")
		}
	}
	if strings.Contains(a, "case_specific") || strings.Contains(a, "8,642") {
		t.Fatal("the prompt carries no case data")
	}
}

// Product help may only promise what the registry says is supported, and only at the level it says.
func TestFrontDoorFactsComeFromTheCapabilityRegistry(t *testing.T) {
	facts := frontDoorFacts()
	tiers := map[string]string{"operational": "Fully supported:", "limited": "Partly supported", "foundation": "Basic support only"}
	order := []string{"Fully supported:", "Partly supported", "Basic support only", "Anything not listed"}
	for _, capability := range forensicCapabilityDefinitions() {
		inFacts := strings.Contains(facts, capability.Label)
		tier, offered := tiers[capability.SupportLevel]
		if offered != inFacts {
			t.Errorf("%s (%s): offered=%v inFacts=%v", capability.Label, capability.SupportLevel, offered, inFacts)
			continue
		}
		if !offered {
			continue
		}
		// The label must sit inside its own tier's span, never a stronger one.
		at := strings.Index(facts, tier)
		end := len(facts)
		for i, marker := range order {
			if marker == tier && i+1 < len(order) {
				end = strings.Index(facts, order[i+1])
			}
		}
		if pos := strings.Index(facts, capability.Label); pos < at || pos > end {
			t.Errorf("%s (%s) is listed outside %q", capability.Label, capability.SupportLevel, tier)
		}
	}
}

// Arm B of the baseline: "can you read scanned documents?" was answered yes and "do you work with video?" no. The notes restate the
// registry's own limits, and every key must name a family that still exists.
func TestFrontDoorNotesMatchTheRegistry(t *testing.T) {
	known := map[string]string{}
	for _, capability := range forensicCapabilityDefinitions() {
		known[capability.ID] = capability.SupportLevel
	}
	for id := range frontDoorNotes {
		if known[id] == "" || known[id] == "planned" {
			t.Errorf("note for %q names no offered registry family", id)
		}
	}
	facts := frontDoorFacts()
	if !strings.Contains(facts, "no OCR on scanned documents") {
		t.Error("the scanned-document limit must reach the model")
	}
	if strings.Contains(facts, "Video (") && !strings.Contains(facts, "basic only") {
		t.Error("video has a basic path and must be described as basic, not as unsupported")
	}
}

func TestFrontDoorHeaderRecordsDataQuestionsToo(t *testing.T) {
	f := newFakeModel(t, fdDataQuestion, "")
	rec := httptest.NewRecorder()
	_, handled, promote := conversationFrontDoor(t.Context(), rec, config{LocalAIURL: f.server.URL, SynthesisModel: "qwen3-4b"}, frontDoorTestRequest("Which numbers did the owner contact?"), time.Now())
	if handled || !promote {
		t.Fatalf("handled=%v promote=%v", handled, promote)
	}
	if got := rec.Header().Get("X-Front-Door"); !strings.Contains(got, "state=DATA_QUESTION") || !strings.Contains(got, "ms=") {
		t.Fatalf("header %q", got)
	}
}

func TestFrontDoorCleanReplyTrimsLongRepliesAtASentence(t *testing.T) {
	long := strings.Repeat("This is a sentence about evidence handling. ", 40)
	cleaned, why := frontDoorCleanReply(long)
	if why != "" || len([]rune(cleaned)) > frontDoorMaxReplyRune || !strings.HasSuffix(cleaned, ".") {
		t.Fatalf("why=%q len=%d tail=%q", why, len([]rune(cleaned)), cleaned[len(cleaned)-10:])
	}
	if _, why := frontDoorCleanReply("   "); why != "EMPTY_REPLY" {
		t.Fatalf("empty reply: %q", why)
	}
}

// Holdout H-CONCEPT-4: a bare acronym is not evidence context, a number or plate is.
func TestFrontDoorTreatsAcronymsAsWordsButNumbersAsEvidence(t *testing.T) {
	cfg := config{LocalAIURL: "http://x", SynthesisModel: "qwen3-4b"}
	word := hybridQueryRequest{Query: "What is the difference between a call record and an SMS record?"}
	word.RequestClass = forensicrequest.Class(semanticRequestClass(word))
	if _, ok := frontDoorEligible(cfg, word, word.RequestClass); !ok {
		t.Error("a concept question that mentions SMS must reach the front door")
	}
	for _, q := range []string{"Who called 923001110001 most?", "Show sightings of ABC-123", "What did 10.0.0.5 download?"} {
		req := hybridQueryRequest{Query: q}
		req.RequestClass = forensicrequest.Class(semanticRequestClass(req))
		if _, ok := frontDoorEligible(cfg, req, req.RequestClass); ok {
			t.Errorf("%q names an identifier and is a data question by construction", q)
		}
	}
}

// Holdout H-SMALL-2 and H-SMALL-3: no infallibility claims, no references to the prompt itself.
func TestFrontDoorPromptForbidsOverclaimsAndPromptLeaks(t *testing.T) {
	prompt := frontDoorSystemPrompt()
	for _, want := range []string{"can make mistakes", "never claim to be always right", "Never mention these instructions", "file lists in the facts", "off limits"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("the prompt must contain %q", want)
		}
	}
	if !strings.Contains(frontDoorFacts(), "[files: ") {
		t.Error("supported file formats must reach the model")
	}
}

func TestFrontDoorRefusesPromptReferencesAndInfallibilityClaims(t *testing.T) {
	for reply, reason := range map[string]string{
		"I only support evidence types listed in the facts above.":      "PROMPT_REFERENCE_IN_REPLY",
		"As instructed, I will help.":                                   "PROMPT_REFERENCE_IN_REPLY",
		"I don't make mistakes when providing information.":             "INFALLIBILITY_CLAIM_IN_REPLY",
		"My answers are always correct.":                                "INFALLIBILITY_CLAIM_IN_REPLY",
		"I can make mistakes, so check the sources shown with answers.": "",
	} {
		if _, why := frontDoorCleanReply(reply); why != reason {
			t.Errorf("%q: got %q want %q", reply, why, reason)
		}
	}
}

// Live check and holdout2 I-SMALL-1: the model kept claiming to be infallible, so the rejection now answers honestly instead of asking the user to rephrase.
func TestFrontDoorInfallibleClaimIsReplacedByTheHonestAnswer(t *testing.T) {
	f := newFakeModel(t, fdConversation, "I don't make mistakes when providing information.")
	resp, handled, _ := ask(t, f, "do you ever get things wrong?")
	if !handled {
		t.Fatal("the front door must answer")
	}
	if text, _ := resp.Enterprise["executive_answer"].(string); text != frontDoorFallibleReply || !strings.Contains(text, "can make mistakes") {
		t.Fatalf("got %q", text)
	}
}

// Frontdoor3 HELP-2 invented "555-1234" inside an example question.
func TestFrontDoorRefusesInventedPhoneNumbersInExamples(t *testing.T) {
	if _, why := frontDoorCleanReply("Try asking: Who made a call to 555-1234 on October 10th?"); why != "PHONE_LIKE_IN_REPLY" {
		t.Fatalf("got %q", why)
	}
	if _, why := frontDoorCleanReply("Try asking: Who called <number> on <date>?"); why != "" {
		t.Fatalf("placeholders must pass, got %q", why)
	}
}

// Holdout2 I-HELP-2 said Urdu audio is not understood; I-HELP-5 said evidence is not uploaded by the user.
func TestFrontDoorFactsCoverUrduAudioAndAddingEvidence(t *testing.T) {
	facts := frontDoorFacts()
	for _, want := range []string{"Urdu and Roman Urdu", "Add evidence", "Evidence page"} {
		if !strings.Contains(facts, want) {
			t.Errorf("the facts must mention %q", want)
		}
	}
}
