package forensicrequest

import (
	"regexp"
	"strings"

	"github.com/mudler/LocalAI/pkg/forensictext"
)

const ContractV1 = "forensics.request-class/v1"

type Class string

const (
	GeneralDomainKnowledge Class = "GENERAL_DOMAIN_KNOWLEDGE"
	ProductHelp            Class = "PRODUCT_HELP"
	GovernedAnalysis       Class = "GOVERNED_EVIDENCE_ANALYSIS"
	ContextualFollowUp     Class = "CONTEXTUAL_FOLLOW_UP"
	Clarify                Class = "CLARIFY"
	Unsupported            Class = "UNSUPPORTED"
)

type Input struct {
	Text                   string
	HasEvidenceContext     bool
	HasConversationContext bool
}

var (
	productHelp      = regexp.MustCompile(`(?i)\b(?:this product|(?:nexusai|product)\s+capabilities|supported\s+(?:file|format|evidence|data)|file\s+types?|file\s+formats?|what\s+can\s+(?:nexusai|the product)|how\s+do\s+i\s+(?:use|upload|add|configure|open))\b`)
	followUp         = regexp.MustCompile(`(?i)\b(?:now\s+(?:incoming|outgoing|by|show|include)|only\s+top|what\s+about|same\s+(?:analysis|query)|instead|where\s+did\s+that|show\s+(?:the\s+)?source\s+rows?|that\s+finding|compare\s+it|the\s+other\s+number|all\s+directions|don['’]?t\s+limit\s+it)\b`)
	unsupported      = regexp.MustCompile(`(?i)\b(?:select\s+\*\s+from|drop\s+table|run\s+(?:shell|powershell|cmd)|read\s+(?:another|a\s+different)\s+(?:tenant|case|collection)|bypass\s+(?:authorization|rls)|reveal\s+(?:passwords?|secrets?|tokens?))\b`)
	evidenceQuestion = regexp.MustCompile(`(?i)\b(?:(?:this|these|those|uploaded|current|case)\s+(?:records?|documents?|evidence|files?)|in\s+(?:this|the\s+current)\s+(?:case|collection|evidence)|this\s+(?:number|plate|subscriber|camera))\b`)
	// A question naming an actual filename (by extension) is asking about a
	// specific piece of case evidence, even when it uses no demonstrative word
	// ("What is report.pdf about?"). Genuine general-knowledge questions never
	// contain a filename, so this adds no false positives.
	// Aggregate and superlative markers. Their presence means the question asks
	// something OF the evidence, not for the meaning of a term.
	//
	// "mean" is deliberately ABSENT. It was included and the existing
	// classification test caught it immediately: "What does IMSI MEAN?" is the
	// canonical concept phrasing, and the verb is far more common in this
	// product than the statistic. "average" and "median" carry the statistical
	// sense without the ambiguity.
	//
	// `earliest` and `latest` were ADDED and REVERTED on 2026-09-26, the same
	// day. They are genuine temporal superlatives and the census was clean --
	// exactly M8 and M15 moved, NEG-03 stayed correctly out of scope -- but
	// measured live, reclassifying them did not send them to the COMPILER. It
	// sent them to `audio_transcript_search`, a RETRIEVAL TEMPLATE on the
	// ladder, where M15 answered "No transcript segment intersects the
	// requested source-time range": a FALSE ABSENCE, which this product holds
	// to be the worst answer it can give because the analyst stops looking.
	// Silence was bad; a confident false absence is worse. The same
	// reclassification took H2 to that template and it returned actual PII
	// transcript text -- see semantic_candidate_ranking.go.
	//
	// Re-add them ONLY once the ladder stops claiming those questions or its
	// templates honour curated sensitivity, together, with a control.
	analyticalIntent = regexp.MustCompile(`(?i)\b(?:largest|smallest|highest|lowest|longest|shortest|greatest|most|least|fewest|maximum|minimum|average|median|total|sum|count|how\s+many|how\s+much|top|per|breakdown|distinct|unique)\b`)
	evidenceFilename = regexp.MustCompile(`(?i)[\w][\w.\-]*\.(?:pdf|docx?|xlsx?|pptx?|csv|tsv|txt|jpe?g|png|gif|bmp|tiff?|mp4|mov|avi|mkv|wav|mp3|m4a|flac|json|xml|pcap|pcapng|zip|eml|msg)\b`)
)

// Classify is the sole terminal request-class authority. Callers provide only
// server-established context booleans; text can never grant evidence access.
func Classify(input Input) Class {
	text := strings.TrimSpace(input.Text)
	if text == "" {
		if input.HasEvidenceContext {
			return GovernedAnalysis
		}
		return Clarify
	}
	if unsupported.MatchString(text) {
		return Unsupported
	}
	if followUp.MatchString(text) {
		if input.HasConversationContext {
			return ContextualFollowUp
		}
		return Clarify
	}
	if productHelp.MatchString(text) {
		return ProductHelp
	}
	if input.HasEvidenceContext || evidenceQuestion.MatchString(text) || evidenceFilename.MatchString(text) {
		return GovernedAnalysis
	}
	// A DEFINITION REQUEST NEVER ASKS FOR A MAXIMUM.
	//
	// `IsConceptExplanation` keys on the shape "what is X", which also fits
	// "what is the LARGEST network volume on any call record?" -- an analytical
	// question about the case. Measured 2026-09-25: it was classified
	// GENERAL_DOMAIN_KNOWLEDGE, never reached the compiler at all (30 ms,
	// terminal route) and answered "a bounded general definition is unavailable
	// for that term". The analyst asked about their evidence and was told a
	// dictionary had failed them.
	//
	// An aggregate or superlative marker settles it: no concept explanation
	// asks for the largest, the average or a count. Measured across the
	// 75-question corpus, exactly TWO questions reach this branch -- NEG-03
	// ("what is the suspect's blood type?", correctly refused as out of scope,
	// and carrying no such marker) and the one above. Only the analytical one
	// moves.
	//
	// Erring toward GovernedAnalysis is also the safe direction: a concept
	// question misread as analytical abstains or clarifies, whereas an
	// analytical question misread as a concept silently refuses to look at the
	// evidence.
	if !analyticalIntent.MatchString(text) && forensictext.IsConceptExplanation(text, false) {
		return GeneralDomainKnowledge
	}
	return GovernedAnalysis
}
