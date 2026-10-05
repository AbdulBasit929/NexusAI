package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// A1c. RELATIONAL CONDITIONS.
//
// Measured 2026-09-27 (reports/adhoc-runtime-20260927/): two questions in a
// person's own words, each answered confidently and wrongly.
//
//	"Do any subscribers share the same handset?"
//	    -> "11 subscriber records matched this question."
//	"Is there any link between the plate sightings and the call records?"
//	    -> "There are 750 ANPR sightings in this case."
//
// Each question asks for a RELATIONSHIP between entities -- two subscribers
// with one handset, a plate and a phone tied together -- and each was answered
// with a count of one kind of record. A count is not "no", and it is not "yes";
// presented as the answer it is a confident wrong answer.
//
// A1a's magnitude guard deliberately does not reach these: its trigger is a
// comparison against a NUMBER. This is the separate detector that report named.
//
// NONE OF THE 103 CORPUS QUESTIONS STATES A RELATIONSHIP, which is why the
// defect was invisible to every measurement until an ad-hoc probe asked one.
//
// SCOPE IS DELIBERATELY NARROW. Only explicit relational forms trigger:
//
//	share / shares / shared / sharing + the same | a | an | any | one | common
//	in common
//	common <noun> between
//	link | connection | relationship | overlap | correlation + between
//
// "the shared account" (an adjective) and "the share of SMS" (a noun) do not
// trigger. "linked to <number>" does not trigger: it names a target and is a
// lookup the target guards already police.
//
// WHAT SATISFIES ONE. A "share" relationship is computed by grouping on the
// shared thing and keeping groups holding more than one entity -- GROUP BY with
// a HAVING over a COUNT or COUNT_DISTINCT. The typed algebra can express that
// (source_native_algebra.go), so a plan of that shape satisfies it. A "between"
// relationship spans record families, which a single-family typed plan cannot
// compute; only the registered relational operations can.
const relationalConditionsEnv = "FORENSIC_RELATIONAL_CONDITIONS"

const withholdRelationshipNotComputed = "relationship_not_computed"

func relationalConditionsEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(relationalConditionsEnv)), "true") //nolint:forbidigo // forensic records product API reads its feature switches from the environment by design (docker compose); not LocalAI server config
}

type relationalKind string

const (
	relationalNone    relationalKind = ""
	relationalShared  relationalKind = "shared"
	relationalBetween relationalKind = "between"
)

var (
	sharedRelationPhrase  = regexp.MustCompile(`(?i)\b(?:share|shares|shared|sharing)\s+(?:the\s+same|a|an|any|one|common)\b(?:\s+[a-z][a-z-]{1,30}){0,2}|\bin\s+common\b`)
	betweenRelationPhrase = regexp.MustCompile(`(?i)\b(?:link|links|connection|connections|relationship|relationships|overlap|correlation|common\s+[a-z]+)\s+between\b[^?.!]{0,80}`)
)

// questionStatesRelationalCondition returns the relationship the analyst
// wrote, in their own wording, and its kind; ("", relationalNone) when the
// question states none. "between" is tested first: "common contacts between A
// and B" is a cross-entity relationship, not a grouping.
func questionStatesRelationalCondition(query string) (string, relationalKind) {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return "", relationalNone
	}
	if phrase := betweenRelationPhrase.FindString(trimmed); phrase != "" {
		return strings.TrimSpace(phrase), relationalBetween
	}
	for _, span := range sharedRelationPhrase.FindAllStringIndex(trimmed, -1) {
		phrase := trimmed[span[0]:span[1]]
		if !strings.HasPrefix(strings.ToLower(phrase), "in") && sharingIsARequest(trimmed[:span[0]]) {
			continue
		}
		return trimTrailingConnective(phrase), relationalShared
	}
	return "", relationalNone
}

// trimTrailingConnective drops a connective the two-word capture can pick up
// ("share the same handset across ..." -> "share the same handset"), so the
// phrase echoed to the analyst ends on the thing shared.
func trimTrailingConnective(phrase string) string {
	words := strings.Fields(phrase)
	for len(words) > 1 {
		switch strings.ToLower(words[len(words)-1]) {
		case "across", "in", "with", "as", "from", "on", "at", "for", "between", "and", "or", "during", "over":
			words = words[:len(words)-1]
			continue
		}
		break
	}
	return strings.Join(words, " ")
}

// sharingIsARequest reports "share" used as "give me": "Share a list of the
// calls", "Can you share a summary", "I want you to share the same report".
// That is a request for output, not a relationship between records. Judged by
// the word before "share": none (an imperative), or a word that makes the
// analyst or the system its subject. Precision over recall -- a miss here
// leaves today's behaviour, a false hit withholds an answer.
func sharingIsARequest(before string) bool {
	fields := strings.Fields(strings.ToLower(before))
	if len(fields) == 0 {
		return true
	}
	switch strings.Trim(fields[len(fields)-1], ",;:") {
	case "you", "please", "pls", "kindly", "me", "us", "i", "we", "to":
		return true
	}
	return false
}

// relationalOperationTemplates compute a relationship between entities rather
// than a count or a listing of one kind of record.
var relationalOperationTemplates = map[string]bool{
	"cross_family_correlation": true,
	"relationship_network":     true,
	"tower_cdr_join":           true,
	"anpr_co_travel":           true,
	"multi_cdr_comparison":     true,
	"subscriber_device_links":  true,
}

// planComputesSharedRelationship reports a typed plan of the one shape that
// answers "do any X share the same Y": grouped, with a HAVING bound on a count.
func planComputesSharedRelationship(plan *SourceNativePlanV1) bool {
	if plan == nil || len(plan.GroupFields) == 0 {
		return false
	}
	counted := map[string]bool{}
	for _, measure := range plan.Measures {
		switch strings.ToUpper(strings.TrimSpace(measure.Op)) {
		case "COUNT", "COUNT_DISTINCT":
			counted[measure.MeasureID] = true
		}
	}
	for _, having := range plan.Having {
		switch strings.ToUpper(strings.TrimSpace(having.Op)) {
		case "GT", "GTE":
			if counted[having.MeasureID] {
				return true
			}
		}
	}
	return false
}

// answerIgnoresStatedRelationship is the guard: the analyst asked for a
// relationship and what is about to execute cannot compute one.
func answerIgnoresStatedRelationship(req hybridQueryRequest, template string) bool {
	if !relationalConditionsEnabled() {
		return false
	}
	_, kind := questionStatesRelationalCondition(req.Query)
	if kind == relationalNone {
		return false
	}
	if relationalOperationTemplates[template] && req.SourceNative == nil {
		return false
	}
	if kind == relationalShared && planComputesSharedRelationship(req.SourceNative) {
		return false
	}
	return true
}

// relationshipNotComputedReason names the relationship back to the analyst and
// says what would compute it. Naming it is the point: "11 records matched"
// hid that the question was never asked of the data.
func relationshipNotComputedReason(req hybridQueryRequest, template string) *withholdReason {
	phrase, kind := questionStatesRelationalCondition(req.Query)
	groups, having := 0, 0
	if req.SourceNative != nil {
		groups, having = len(req.SourceNative.GroupFields), len(req.SourceNative.Having)
	}
	question := fmt.Sprintf("I did not compute the relationship you asked about (%q). The plan I "+
		"found only counts or lists one kind of record, which cannot show whether records share "+
		"anything. Tell me which field they would share, and I will group on it and keep only the "+
		"values held by more than one record.", phrase)
	if kind == relationalBetween {
		question = fmt.Sprintf("I did not compute the relationship you asked about (%q). The plan I "+
			"found looks at one kind of record only. Name one identifier, such as a phone number, a "+
			"plate or a device, and I will correlate it across the record types.", phrase)
	}
	return &withholdReason{
		Code:     withholdRelationshipNotComputed,
		Question: question,
		Guardrail: fmt.Sprintf("No result was stated because the question asks for the relationship %q "+
			"while the executed plan computes none. A count of one kind of record presented as the "+
			"answer to a relationship is a confident wrong answer.", phrase),
		Detail: fmt.Sprintf("relation=%s template=%s plan_group_fields=%d plan_having=%d", kind, template, groups, having),
	}
}
