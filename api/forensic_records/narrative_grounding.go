package main

import (
	"fmt"
	"regexp"
	"strings"
)

var narrativeCriticalPattern = regexp.MustCompile(`(?i)\d{4}-\d{2}-\d{2}(?:T\d{2}:\d{2}(?::\d{2}(?:\.\d+)?)?(?:Z|[+-]\d{2}:\d{2})?)?|\b\d{1,2}:\d{2}(?::\d{2})?\b|[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}|(?:\d{1,3}\.){3}\d{1,3}|(?:[$€£₨]|USD|EUR|GBP|PKR)\s?[+-]?\d+(?:\.\d+)?|\b[a-z][a-z0-9]*[-_:][a-z0-9._:-]*\d[a-z0-9._:-]*|\b[a-z]+\d+[a-z0-9]*\b|[+-]?\d+(?:\.\d+)?(?:%|ms|km|mb|gb)?`)

func narrativeCriticalTokens(text string) []string {
	tokens := narrativeCriticalPattern.FindAllString(text, -1)
	for i, t := range tokens {
		tokens[i] = strings.TrimRight(t, ".,;:")
	}
	return tokens
}

// A deterministic lexical entailment guard supports conservative paraphrases;
// it is not a general natural-language truth oracle. New content words,
// polarity changes and ungrounded relationships fail closed. Values and source
// references are checked independently, before this wording check.
func groundedNarrativeWording(text string, refs []string, facts map[string]FactPacketFactV1) bool {
	// A claim must follow one referenced proposition. Pooling words and values
	// across facts would permit a value from one source to describe another.
	for _, ref := range refs {
		if fact, ok := facts[ref]; ok && groundedSingleFactWording(text, fact.Text) {
			return true
		}
	}
	if len(refs) < 2 || len(refs) > 3 {
		return false
	}
	clauses := regexp.MustCompile(`\s*(?:[.;]|\band\b)\s*`).Split(text, -1)
	used := map[string]bool{}
	for _, clause := range clauses {
		if strings.TrimSpace(clause) == "" {
			continue
		}
		matched := false
		for _, ref := range refs {
			if used[ref] {
				continue
			}
			if fact, ok := facts[ref]; ok && groundedSingleFactWording(clause, fact.Text) {
				used[ref], matched = true, true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return len(used) == len(refs)
}

func groundedSingleFactWording(text, source string) bool {
	if strings.TrimSpace(text) == "" {
		return false
	}
	normalize := func(s string) string { return strings.Join(strings.Fields(strings.ToLower(s)), " ") }
	if normalize(text) == normalize(source) {
		return true
	}
	// Normalize two bounded grammatical alternations required by the
	// presentation contract. These preserve the same subject, value and event.
	passiveCamera := regexp.MustCompile(`(?i)^\s*(\d+)\s+observations?\s+(?:was|were)\s+recorded\s+at\s+(?:the\s+)?camera\s+([a-z0-9._:-]+)[.!]?\s*$`)
	if match := passiveCamera.FindStringSubmatch(text); match != nil {
		text = "camera " + match[2] + " recorded " + match[1] + " observations"
	}
	counterparties := regexp.MustCompile(`(?i)\bcommunicated\s+with\s+(\d+)\s+distinct\s+counterparties\b`)
	text = counterparties.ReplaceAllString(text, "had $1 counterparties")
	// A source explicitly stating a proposition also indicates it. This
	// one-way attribution paraphrase cannot strengthen an indication into a
	// confirmed statement, and requires the same named reporting source.
	attribution := regexp.MustCompile(`(?i)^(.+?)\s+(states|stated)\s+`)
	if match := attribution.FindStringSubmatch(source); match != nil {
		weaker := regexp.MustCompile(`(?i)^` + regexp.QuoteMeta(match[1]) + `\s+(?:indicates|indicated)\s+`)
		text = weaker.ReplaceAllString(text, match[0])
	}
	recordedAttribution := regexp.MustCompile(`(?i)^(.+?)\s+recorded\s+`)
	if match := recordedAttribution.FindStringSubmatch(source); match != nil {
		weaker := regexp.MustCompile(`(?i)^` + regexp.QuoteMeta(match[1]) + `\s+shows\s+`)
		text = weaker.ReplaceAllString(text, match[0])
	}
	polarity := regexp.MustCompile(`(?i)\b(?:not|never|no|without|cannot|absent)\b`)
	if polarity.MatchString(text) != polarity.MatchString(source) {
		return false
	}
	textLower, sourceLower := strings.ToLower(text), strings.ToLower(source)
	for _, strengthening := range []string{"because", "due to", "probably", "confirmed", "identified", "confidence", "physically present", "owns", "owner", "caused", "continuous presence"} {
		if strings.Contains(textLower, strengthening) && !strings.Contains(sourceLower, strengthening) {
			return false
		}
	}
	for _, ranking := range []string{"highest", "lowest", "first", "last", "most frequent", "least frequent"} {
		if strings.Contains(textLower, ranking) && !strings.Contains(sourceLower, ranking) {
			return false
		}
	}
	// Preserve entity/value ordering, so reversing two endpoints cannot turn
	// an outgoing relationship into an incoming one using the same token set.
	remaining := narrativeCriticalTokens(source)
	for _, token := range narrativeCriticalTokens(text) {
		found := false
		for len(remaining) > 0 {
			candidate := remaining[0]
			remaining = remaining[1:]
			if candidate == token {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	for _, qualifier := range []string{"possible", "candidate", "suspected", "approximate", "alleged"} {
		if strings.Contains(strings.ToLower(source), qualifier) && !strings.Contains(strings.ToLower(text), qualifier) {
			return false
		}
	}
	ignored := map[string]bool{}
	for _, s := range strings.Fields("a an the this that these those of for to from by in on at as and or with is are was were be been being has have had it its their there which who whose supplied authorized cited recorded observed reported fact evidence target respectively") {
		ignored[s] = true
	}
	equivalents := map[string]string{"contacts": "contact", "contacted": "contact", "communicated": "contact", "communicates": "contact", "communication": "contact", "counterparties": "counterparty", "distinct": "unique", "reports": "report", "reported": "report", "reporting": "report", "records": "record", "recorded": "record", "observations": "observation", "numbers": "number", "entries": "entry", "contains": "contain", "containing": "contain", "returned": "return", "returns": "return", "states": "state", "stated": "state", "shows": "show", "shown": "show", "readings": "reading"}
	words := func(s string) []string {
		s = narrativeCriticalPattern.ReplaceAllString(s, " ")
		out := []string{}
		for _, w := range regexp.MustCompile(`[\pL]+`).FindAllString(strings.ToLower(s), -1) {
			if v, ok := equivalents[w]; ok {
				w = v
			}
			if !ignored[w] {
				out = append(out, w)
			}
		}
		return out
	}
	remainingWords := words(source)
	matched := len(narrativeCriticalTokens(text)) > 0
	for _, w := range words(text) {
		found := false
		for len(remainingWords) > 0 {
			candidate := remainingWords[0]
			remainingWords = remainingWords[1:]
			if candidate == w {
				found = true
				break
			}
		}
		if !found {
			return false
		}
		matched = true
	}
	return matched
}

func narrativeFollowupQueries(packet FactPacketV1) []string {
	allowed := []string{}
	for _, f := range packet.AvailableFollowUps {
		if q := fpFirstString(f, "query"); q != "" {
			allowed = append(allowed, q)
		}
	}
	return allowed
}

func narrativeStringChoices(allowed []string, maximum int) map[string]any {
	if len(allowed) == 0 {
		return map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "maxItems": 0}
	}
	return map[string]any{"type": "array", "items": map[string]any{"type": "string", "enum": allowed}, "uniqueItems": true, "maxItems": min(maximum, len(allowed))}
}

// narrativeIndexedIDs/narrativeIndexedEnumSchema/narrativeIndexedTranslate
// exist because a JSON Schema enum whose choices are full free-text
// sentences (limitations text, follow-up question text — sometimes 100+
// characters each) is extremely expensive for llama.cpp's grammar sampler
// to compile/evaluate on this CPU backend: a live test measured narration
// consistently timing out at the full 120s synthesis budget even after
// shrinking every other bound (max_tokens, per-claim ref limits, array
// sizes), while the exact same schema SHAPE with short (F1/F2-style) enum
// values generated at the hardware's normal ~4.3 tokens/sec. The fix keeps
// the exact same safety property (the model can only select from the
// server-supplied list, never invent new text) by having the model choose
// a short ID instead of the literal text, then translating IDs back to the
// real text server-side before validateNarrative runs — validateNarrative
// still requires the real text to appear verbatim, unchanged.
func narrativeIndexedIDs(prefix string, count int) []string {
	ids := make([]string, count)
	for i := range ids {
		ids[i] = fmt.Sprintf("%s%d", prefix, i+1)
	}
	return ids
}

func narrativeIndexedEnumSchema(items []string, prefix string, maximum int) map[string]any {
	return narrativeStringChoices(narrativeIndexedIDs(prefix, len(items)), maximum)
}

func narrativeIndexedTranslate(ids []string, items []string, prefix string) []string {
	lookup := make(map[string]string, len(items))
	for i, item := range items {
		lookup[fmt.Sprintf("%s%d", prefix, i+1)] = item
	}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if text, ok := lookup[id]; ok {
			out = append(out, text)
		}
	}
	return out
}
