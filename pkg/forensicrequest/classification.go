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
	if forensictext.IsConceptExplanation(text, false) {
		return GeneralDomainKnowledge
	}
	return GovernedAnalysis
}
