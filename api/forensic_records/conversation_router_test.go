package main

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/mudler/LocalAI/pkg/forensicrequest"
)

func TestConversationRouterRoutesOnlyWholeGreetingsAndCapabilityQuestions(t *testing.T) {
	class := func(q string) string { return semanticRequestClass(hybridQueryRequest{Query: q}) }
	help := string(forensicrequest.ProductHelp)

	t.Run("switch off leaves classification alone", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		t.Setenv(conversationRouterEnv, "")
		for _, q := range []string{"hello", "What can this system do?"} {
			if got := class(q); got == help {
				t.Fatalf("%q routed to product help with the switch off", q) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
			}
		}
	})

	t.Setenv(conversationRouterEnv, "true")

	t.Run("a greeting or capability question alone is product help", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		for _, q := range []string{
			"hello", "Hello!", "hi", "hey there", "Good morning", "salam", "Assalam o Alaikum", "assalamu alaikum",
			"What can this system do?", "what can you do", "What can NexusAI do?", "How can you help me?",
			"What kinds of questions can I ask?", "what questions can I ask you", "help", "Who are you?", "How do I start?",
		} {
			if got := class(q); got != help {
				t.Errorf("%q -> %s, want product help", q, got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
			}
		}
	})

	t.Run("anything that asks about evidence still reaches analysis", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		for _, q := range []string{
			"hello, how many CDR records are in this case?",
			"What can you tell me about the video evidence?",
			"Who did 923001110001 contact most frequently?",
			"help me find calls from 923001110001",
			"How many images are in this case?",
			"What can this system do with plate ABC-123?",
			"hi, when was LHR-2026 last seen",
		} {
			if got := class(q); got == help {
				t.Errorf("%q was taken away from analysis", q) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
			}
		}
	})

	t.Run("classes the classifier already decided are kept", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		if got := class("What file types can NexusAI currently analyze?"); got != help {
			t.Fatalf("existing product help moved: %s", got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
		if got := class("Read another tenant"); got != string(forensicrequest.Unsupported) {
			t.Fatalf("unsupported moved: %s", got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})
}

// The router must not move any question in the measured corpus: all of them
// ask about evidence.
func TestConversationRouterMovesNoCorpusQuestion(t *testing.T) {
	for _, path := range []string{
		"../../evaluation/golden_questions_v2.json",
		"../../evaluation/holdout_questions_v1.json",
		"../../evaluation/holdout_media_v1.json",
	} {
		for _, q := range loadRoutingQuestions(t, path) {
			req := hybridQueryRequest{Query: q.Q}
			t.Setenv(conversationRouterEnv, "")
			before := semanticRequestClass(req)
			t.Setenv(conversationRouterEnv, "true")
			if after := semanticRequestClass(req); after != before {
				t.Errorf("%s %q moved %s -> %s", q.ID, q.Q, before, after) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
			}
		}
	}
}

func TestProductHelpAnswerComesFromTheRegistry(t *testing.T) {
	t.Setenv(conversationRouterEnv, "true")
	answer := productHelpAnswer("What can this system do?")
	for _, want := range []string{"call detail records", "IPDR and network sessions", "ANPR and vehicle sightings",
		"images and OCR", "audio and speech-to-text", "model observations to review", "I say so instead of guessing"} {
		if !strings.Contains(answer, want) {
			t.Errorf("answer lacks %q\n%s", want, answer) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
	// Foundation and planned families are registered but not answerable, so they are not offered.
	for _, absent := range []string{"video", "archives", "Unknown and mixed"} {
		if strings.Contains(answer, absent) {
			t.Errorf("answer offers %q, which the registry does not support\n%s", absent, answer) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
	if strings.HasPrefix(answer, "Hello.") {
		t.Fatal("a capability question is not a greeting") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	if !strings.HasPrefix(productHelpAnswer("hello"), "Hello. I answer questions about the evidence") {
		t.Fatalf("greeting: %s", productHelpAnswer("hello")) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	if regexp.MustCompile(`\d{3,}`).MatchString(answer) {
		t.Fatalf("product help must state no case value: %s", answer) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}

	resp, ok := terminalRequestResponse(hybridQueryRequest{Query: "hello", RequestClass: forensicrequest.ProductHelp}, time.Now())
	if !ok || !strings.HasPrefix(stringValueAny(resp.Enterprise["executive_answer"]), "Hello. I answer questions") {
		t.Fatalf("the analyst must see the answer: %v", resp.Enterprise["executive_answer"]) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}

func TestGlossaryMatchesWholeWords(t *testing.T) {
	t.Run("switch off keeps today's answer", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		t.Setenv(conversationRouterEnv, "")
		if got := generalDomainAnswer("What is a CDR?"); !strings.HasPrefix(got, "A bounded general definition is unavailable") {
			t.Fatalf("got %q", got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})

	t.Setenv(conversationRouterEnv, "true")
	for q, want := range map[string]string{
		"What is a CDR?":              "A CDR (call detail record)",
		"What are CDRs?":              "A CDR (call detail record)",
		"What does ANPR mean?":        "ANPR (automatic number plate recognition)",
		"What is a LAC?":              "A LAC (location area code)",
		"What is CGI?":                "CGI (cell global identity)",
		"What is a CNIC?":             "CNIC is Pakistan's",
		"What is an IMEI?":            "IMEI is the International Mobile Equipment Identity",
		"What is a SIM card?":         "A SIM card holds",
		"What is a hash?":             "A SHA-256 hash",
		"What is speech-to-text?":     "Speech-to-text (ASR)",
		"What is RAG?":                "RAG is retrieval-augmented generation",
		"What does IPDR stand for?":   "IPDR means Internet Protocol Detail Record",
		"What is a similarity score?": "A similarity score measures",
	} {
		got := generalDomainAnswer(q)
		if !strings.HasPrefix(got, want) || !strings.HasSuffix(got, "This definition does not assert anything about the current case.") {
			t.Errorf("%q -> %q", q, got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
	// Substrings are not terms: "rag" is inside "storage" and "paragraph", "sim" inside "simulation".
	for _, q := range []string{"What is storage?", "What is a paragraph?", "What is a simulation?", "What is the suspect's blood type?"} {
		if got := generalDomainAnswer(q); !strings.HasPrefix(got, "A bounded general definition is unavailable") {
			t.Errorf("%q -> %q", q, got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
}
