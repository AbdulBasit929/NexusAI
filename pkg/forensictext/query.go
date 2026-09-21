// Package forensictext defines retrieval-only semantics shared by query callers
// and executors. It never normalizes or replaces retained evidence.
package forensictext

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

type Semantic string

const (
	ExactValue     Semantic = "EXACT_VALUE"
	ExactPhrase    Semantic = "EXACT_PHRASE"
	PhraseContains Semantic = "PHRASE_CONTAINS"
	TokenSearch    Semantic = "TOKEN_SEARCH"
	Source         Semantic = "SOURCE"
	TimeRange      Semantic = "TIME_RANGE"
	SourceTime     Semantic = "SOURCE_TIME"
)

const NormalizationV1 = "nfc-whitespace/v1"

type Query struct {
	LiteralText    string   `json:"literal_text,omitempty"`
	MatchSemantic  Semantic `json:"match_semantic"`
	Representation string   `json:"text_representation,omitempty"`
}

func Normalize(value string) string { return strings.Join(strings.Fields(norm.NFC.String(value)), " ") }

func (q Query) Validate() error {
	switch q.MatchSemantic {
	case ExactValue, ExactPhrase, PhraseContains, TokenSearch:
		if Normalize(q.LiteralText) == "" {
			return fmt.Errorf("INSUFFICIENT_LITERAL")
		}
	case Source, TimeRange, SourceTime:
		if q.LiteralText != "" {
			return fmt.Errorf("AMBIGUOUS_LITERAL_SEMANTIC")
		}
	default:
		return fmt.Errorf("UNSUPPORTED_TEXT_SEMANTIC")
	}
	if utf8.RuneCountInString(q.LiteralText) > 4096 {
		return fmt.Errorf("QUERY_TOO_BROAD")
	}
	// Two letters/numbers admits useful short words in both scripts, while
	// excluding one-character and punctuation-only case-wide substring scans.
	if q.MatchSemantic == PhraseContains {
		n := 0
		for _, r := range q.LiteralText {
			if unicode.IsLetter(r) || unicode.IsNumber(r) {
				n++
			}
		}
		if n < 2 {
			return fmt.Errorf("INSUFFICIENT_LITERAL")
		}
	}
	switch q.Representation {
	case "", "raw", "roman_derivative":
	default:
		return fmt.Errorf("UNSUPPORTED_TEXT_REPRESENTATION")
	}
	return nil
}

func word(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsMark(r) || r == '\u200c' || r == '\u200d'
}

func Match(text string, q Query) bool {
	if q.Validate() != nil {
		return false
	}
	haystack, needle := Normalize(text), Normalize(q.LiteralText)
	switch q.MatchSemantic {
	case ExactValue:
		return haystack == needle
	case PhraseContains:
		return strings.Contains(haystack, needle)
	case ExactPhrase:
		for offset := 0; offset <= len(haystack)-len(needle); {
			i := strings.Index(haystack[offset:], needle)
			if i < 0 {
				break
			}
			i += offset
			end := i + len(needle)
			left, _ := utf8.DecodeLastRuneInString(haystack[:i])
			right, _ := utf8.DecodeRuneInString(haystack[end:])
			first, _ := utf8.DecodeRuneInString(needle)
			last, _ := utf8.DecodeLastRuneInString(needle)
			if (i == 0 || !word(first) || !word(left)) && (end == len(haystack) || !word(last) || !word(right)) {
				return true
			}
			_, size := utf8.DecodeRuneInString(haystack[i:])
			offset = i + size
		}
		return false
	case TokenSearch:
		for _, token := range strings.Fields(needle) {
			if !Match(text, Query{LiteralText: token, MatchSemantic: ExactPhrase}) {
				return false
			}
		}
		return true
	case Source, SourceTime:
		return true
	}
	return false
}

var quoted = regexp.MustCompile(`"([^"]+)"|“([^”]+)”|'([^']+)'|‘([^’]+)’`)
var audioHint = regexp.MustCompile(`(?i)\b(transcripts?|transcriptions?|recordings?|audio)\b|ٹرانسکرپٹ|ریکارڈنگ`)
var imageHint = regexp.MustCompile(`(?i)\b(ocr|image|recognized text)\b|تصویر`)
var documentHint = regexp.MustCompile(`(?i)\b(documents?|pdf|docx|passages?)\b|دستاویز`)

var workspaceScopePhrases = []string{
	"whole investigation", "across the investigation", "across the case", "all evidence", "entire case",
	"all recordings", "every recording", "all transcripts", "every transcript", "all transcript files", "every transcript file",
	"all documents", "every document", "all document files", "every document file",
	"poori investigation", "saray saboot", "sare saboot", "tamam evidence",
	"tamam recordings", "tamam case recordings", "tamam transcripts", "tamam transcript files", "tamam documents", "tamam document files",
	"پورے کیس", "پوری تفتیش", "تمام شواہد", "تمام ریکارڈنگ", "تمام ٹرانسکرپٹ", "تمام دستاویزات",
}

var selectedScopePhrases = []string{
	"this recording", "this audio", "this image", "this document", "this video",
	"is recording", "is audio", "is tasveer", "is document", "is video",
	"اس ریکارڈنگ", "اس تصویر", "اس دستاویز", "اس ویڈیو",
}

// ExplicitScope reads only bounded scope phrases outside quoted evidence text.
// It cannot change the authorized case; callers bind the returned intent to
// their already-authorized request context.
func ExplicitScope(input string) string {
	normalized := strings.ToLower(strings.Join(strings.Fields(PlanningText(input)), " "))
	contains := func(phrases []string) bool {
		for _, phrase := range phrases {
			if strings.Contains(normalized, phrase) {
				return true
			}
		}
		return false
	}
	workspace, selected := contains(workspaceScopePhrases), contains(selectedScopePhrases)
	if workspace && selected {
		return "ambiguous"
	}
	if workspace {
		return "authorized_workspace"
	}
	if selected {
		return "selected_evidence"
	}
	return ""
}

// Extract precedes family selection. Family hints exclude the quoted span so
// evidence text cannot accidentally steer its own operation selection.
func Extract(input string) (*Query, string) {
	matches := literalSpans(input)
	if len(matches) == 0 {
		return nil, ""
	}
	match := matches[0]
	literal := ""
	for i := 2; i < len(match); i += 2 {
		if match[i] >= 0 {
			literal = input[match[i]:match[i+1]]
			break
		}
	}
	outside := PlanningText(input)
	semantic := PhraseContains
	if len(matches) > 1 {
		return &Query{LiteralText: literal, MatchSemantic: "AMBIGUOUS_LITERAL"}, ""
	}
	if strings.Contains(strings.ToLower(outside), "exact phrase") {
		semantic = ExactPhrase
	}
	family := ""
	for name, pattern := range map[string]*regexp.Regexp{"audio": audioHint, "image": imageHint, "document": documentHint} {
		if pattern.MatchString(outside) {
			if family != "" {
				return &Query{LiteralText: literal, MatchSemantic: semantic}, ""
			}
			family = name
		}
	}
	return &Query{LiteralText: literal, MatchSemantic: semantic}, family
}

func literalSpans(input string) [][]int {
	result := [][]int{}
	for _, span := range quoted.FindAllStringSubmatchIndex(input, -1) {
		if input[span[0]] == '\'' {
			before, _ := utf8.DecodeLastRuneInString(input[:span[0]])
			after, _ := utf8.DecodeRuneInString(input[span[1]:])
			if span[0] > 0 && word(before) || span[1] < len(input) && word(after) {
				continue
			}
		}
		result = append(result, span)
	}
	return result
}

// PlanningText prevents words inside an evidence literal from becoming family
// or operation hints. The untouched input remains the literal authority.
func PlanningText(input string) string {
	spans := literalSpans(input)
	for i := len(spans) - 1; i >= 0; i-- {
		s := spans[i]
		input = input[:s[0]] + " " + input[s[1]:]
	}
	return input
}

func Template(family string) string {
	switch family {
	case "audio":
		return "audio_transcript_search"
	case "image":
		return "image_ocr_search"
	case "document", "text":
		return "document_search"
	}
	return ""
}
