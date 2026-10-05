package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/mudler/LocalAI/pkg/forensicrequest"
)

// B1. THE FIRST MINUTE OF REAL USE: A GREETING, "WHAT CAN YOU DO", "WHAT IS A CDR".
//
// Measured 2026-09-29 on the deployed build (reports/template-states-20260929/on/adhoc):
//
//	"hello"                     -> "I could not map that request to a deterministic forensic workflow. ..."
//	"What can this system do?"  -> the same refusal
//	"What is a CDR?"            -> "A bounded general definition is unavailable for that term."
//
// A new user's first three messages were all refusals. None of them asks about the
// case, so none of them should reach the query engine at all.
//
// What this changes, and only when the switch is on:
//
//   - A message that is ONLY a greeting, or ONLY a question about what the system
//     can do, is routed to the existing product-help class. The whole message
//     must match, so "hello, how many CDR records are there?" and "What can you
//     tell me about the video evidence?" still go to the query engine. A message
//     carrying any evidence context (a number, a plate, a selected file) is never
//     rerouted, and neither is anything the classifier already placed elsewhere.
//   - The product-help answer describes the registered families by their support
//     level, from forensicCapabilityDefinitions -- not from prose that could drift.
//   - The glossary covers the terms the product's own screens use (CDR, ANPR,
//     LAC, CGI, CNIC, ...) and matches whole words. It matched substrings, so
//     "rag" answered for "storage" and "paragraph".
//
// No answer here states anything about a case.
//
// DEFAULT OFF, like every switch gating new behaviour. Off reproduces today's
// classification and text exactly.
const conversationRouterEnv = "FORENSIC_CONVERSATION_ROUTER"

func conversationRouterEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(conversationRouterEnv)), "true") //nolint:forbidigo // forensic records product API reads its feature switches from the environment by design (docker compose); not LocalAI server config
}

var (
	greetingOnly   = regexp.MustCompile(`(?i)^\s*(?:hi|hello|hey|hiya|greetings|good\s+(?:morning|afternoon|evening)|sala+m|as+alam(?:u|o)?[\s-]*(?:alaikum|alaykum|o\s*alaikum|u\s*alaikum))(?:\s+(?:there|everyone|team|nexus(?:ai)?))?[\s!.,?]*$`)
	capabilityOnly = regexp.MustCompile(`(?i)^\s*(?:(?:what|how)\s+(?:can|could)\s+(?:you|this(?:\s+(?:system|tool|app|application|platform|product))?|it|nexus(?:ai)?)\s+(?:do|help(?:\s+me)?(?:\s+with)?|answer)(?:\s+for\s+me)?|what\s+(?:kinds?|types?|sorts?)\s+of\s+questions\s+(?:can|should|could)\s+i\s+ask(?:\s+you)?|what\s+(?:questions|things)\s+can\s+i\s+ask(?:\s+you)?|what\s+do\s+you\s+do|who\s+are\s+you|what\s+are\s+you|help(?:\s+me)?|how\s+do\s+i\s+(?:start|begin|use\s+(?:this|you|it)))[\s!.,?]*$`)
)

// routedConversationClass returns ProductHelp for a greeting or capability
// question that the classifier sent to analysis without evidence context (or to
// a definition, as it does for "How do I start?"), and the classifier's own
// answer otherwise. Both source classes stay as they were for anything else.
func routedConversationClass(req hybridQueryRequest, class string) string {
	if !conversationRouterEnabled() || semanticHasExplicitEvidenceContext(req) {
		return class
	}
	if class != string(forensicrequest.GovernedAnalysis) && class != string(forensicrequest.GeneralDomainKnowledge) {
		return class
	}
	if greetingOnly.MatchString(req.Query) || capabilityOnly.MatchString(req.Query) {
		return string(forensicrequest.ProductHelp)
	}
	return class
}

// productHelpAnswer is the analyst-facing text of a product-help response.
func productHelpAnswer(query string) string {
	operational, reviewed := []string{}, []string{}
	for _, capability := range forensicCapabilityDefinitions() {
		label := capabilityPhrase(capability.Label)
		switch capability.SupportLevel {
		case "operational":
			operational = append(operational, label)
		case "limited":
			reviewed = append(reviewed, label)
		}
	}
	var b strings.Builder
	if greetingOnly.MatchString(query) {
		b.WriteString("Hello. ")
	}
	b.WriteString("I answer questions about the evidence in the selected case by running exact queries and citing the source records.")
	if len(operational) > 0 {
		fmt.Fprintf(&b, " Supported: %s.", statedSeries(operational))
	}
	if len(reviewed) > 0 {
		fmt.Fprintf(&b, " Supported with limits, where results are model observations to review: %s.", statedSeries(reviewed))
	}
	b.WriteString(" For example: \"How many CDR records are in this case?\", \"Who did <phone number> contact most frequently?\", \"When was plate <plate> first and last seen?\" or \"Find documents that mention <phrase>\".")
	b.WriteString(" When a question cannot be computed exactly, I say so instead of guessing.")
	return b.String()
}

// capabilityPhrase lowers a registry label for use mid-sentence, keeping
// acronyms ("IPDR and network sessions", "PDF, office, email, and ebook documents").
func capabilityPhrase(label string) string {
	label = strings.TrimSpace(label)
	if label == "" {
		return label
	}
	if first := strings.Fields(label)[0]; len(first) > 1 && strings.Trim(first, ",") == strings.ToUpper(strings.Trim(first, ",")) {
		return label
	}
	return strings.ToLower(label[:1]) + label[1:]
}

// statedSeries joins with semicolons, since the labels themselves contain
// commas and "and".
func statedSeries(values []string) string {
	return strings.Join(values, "; ")
}

// glossary is the general-knowledge definition list. Each entry states what a
// term means in general and nothing about a case.
var glossary = []struct {
	pattern *regexp.Regexp
	text    string
}{
	{regexp.MustCompile(`(?i)\b(?:cdrs?|call\s+detail\s+records?)\b`), "A CDR (call detail record) is the record a telecom operator keeps for each call, SMS or data event. It typically notes the numbers involved, the time, the duration and the cell that handled it."},
	{regexp.MustCompile(`(?i)\b(?:ipdrs?|internet\s+protocol\s+detail\s+records?)\b`), "IPDR means Internet Protocol Detail Record, a structured record describing an IP service or network session."},
	{regexp.MustCompile(`(?i)\b(?:anpr|automatic\s+number\s+plate\s+recognition)\b`), "ANPR (automatic number plate recognition) is camera-based reading of vehicle number plates. An ANPR sighting records a plate read by a camera at a time and place. Reads can be wrong, especially from poor images."},
	{regexp.MustCompile(`(?i)\bimsi\b`), "IMSI is the International Mobile Subscriber Identity, an identifier assigned to a mobile subscription and stored on the SIM."},
	{regexp.MustCompile(`(?i)\bimei\b`), "IMEI is the International Mobile Equipment Identity, an identifier assigned to mobile equipment."},
	{regexp.MustCompile(`(?i)\biccid\b`), "ICCID is the Integrated Circuit Card Identifier, the serial identifier of a SIM card."},
	{regexp.MustCompile(`(?i)\bmsisdn\b`), "MSISDN is the telephone number associated with a mobile subscription."},
	{regexp.MustCompile(`(?i)\b(?:lac|location\s+area\s+code)\b`), "A LAC (location area code) identifies a group of cells in a mobile network. Together with a cell ID it identifies the cell that handled an event."},
	{regexp.MustCompile(`(?i)\b(?:cgi|cell\s+global\s+identity)\b`), "CGI (cell global identity) identifies one mobile network cell worldwide: the country code, network code, location area code and cell ID together."},
	{regexp.MustCompile(`(?i)\bcell\s*(?:id|identity)\b`), "A cell ID identifies one cell, usually one sector of a mobile tower, within a location area."},
	{regexp.MustCompile(`(?i)\b(?:bts|base\s+transceiver\s+station)\b`), "A BTS (base transceiver station) is the radio equipment at a mobile tower site that phones connect to."},
	{regexp.MustCompile(`(?i)\bcnic\b`), "CNIC is Pakistan's Computerised National Identity Card, issued by NADRA. Its 13-digit number identifies a person and is treated as sensitive personal data."},
	{regexp.MustCompile(`(?i)\bsim(?:\s+card)?s?\b`), "A SIM card holds a mobile subscription's identity (IMSI) and its own serial number (ICCID). It can be moved between handsets."},
	{regexp.MustCompile(`(?i)\bussd\b`), "USSD is a mobile network service for short codes such as *123#, often used for balance checks and mobile banking."},
	{regexp.MustCompile(`(?i)\bexif\b`), "EXIF is metadata embedded in image files, such as capture time, camera model and sometimes GPS position. It can be missing or edited."},
	{regexp.MustCompile(`(?i)\b(?:sha-?256|hash(?:es)?)\b`), "A SHA-256 hash is a fixed-length fingerprint of a file's content. Any change to the file changes the hash, which is how evidence integrity is checked."},
	{regexp.MustCompile(`(?i)\b(?:ocr|optical\s+character\s+recognition)\b`), "OCR is optical character recognition, the process of turning visible text in images or documents into machine-readable text."},
	{regexp.MustCompile(`(?i)\b(?:asr|speech[\s-]+to[\s-]+text|transcription)\b`), "Speech-to-text (ASR) converts recorded speech into text with a model. A transcript produced this way is a model observation and can contain errors."},
	{regexp.MustCompile(`(?i)\b(?:rag|retrieval[\s-]+augmented(?:\s+generation)?)\b`), "RAG is retrieval-augmented generation: relevant source material is retrieved and supplied as context for an answer."},
	{regexp.MustCompile(`(?i)\bsimilarity\s+scores?\b`), "A similarity score measures how close two representations are under a specified method. It is not, by itself, an identity probability or confidence statement."},
}

// glossaryAnswer returns the definition of the first glossary term the query
// names as a whole word, or ok=false.
func glossaryAnswer(query string) (string, bool) {
	for _, entry := range glossary {
		if entry.pattern.MatchString(query) {
			return entry.text + " This definition does not assert anything about the current case.", true
		}
	}
	return "", false
}
