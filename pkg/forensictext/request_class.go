package forensictext

import "regexp"

var conceptSpeechAct = regexp.MustCompile(`(?i)^\s*(?:explain|define|describe the meaning|what (?:is|are|does)|how (?:does|do)|why (?:does|do|is|are|can))\b`)
// Analysis-result nouns ("the call type breakdown", "the results", "the night
// activity", "the calls") refer to this case's output, not to a concept, so
// "Explain the call type breakdown" is case analysis. Bare family acronyms
// stay conceptual ("What is IPDR?").
var evidenceSpeechAct = regexp.MustCompile(`(?i)\b(?:this|these|my|our|selected|uploaded|case|investigation|collection|records|documents|recordings|sightings|total|count|most|least|highest|lowest|yesterday|today|breakdown|results?|findings?|activity|patterns?|timeline|calls|sessions|contacts|transactions|subscribers|towers|plates|numbers)\b`)

// Concept requests are separated before evidence-family selection. An explicit
// target or evidence context always wins over an explanatory opening verb.
func IsConceptExplanation(question string, explicitEvidenceContext bool) bool {
	return !explicitEvidenceContext && conceptSpeechAct.MatchString(question) && !evidenceSpeechAct.MatchString(question)
}
