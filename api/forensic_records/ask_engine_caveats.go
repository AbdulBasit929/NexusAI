package main

// ASK ENGINE V2: THE CAVEATS OF AN ANALYSIS (link analysis stage 3, docs/data/link-analysis-stage3-preregistration-20261022.md section 6).
//
// The engine, not the model, adds these lines to the response's limitations, keyed by the view the query reads, so the model can neither omit nor reword them. The texts are
// fixed in the pre-registration: a gate checks that every answer that returns a result of the analysis carries them, and an edit here is an amendment there.
var askViewCaveatText = map[string]string{
	// A5: co-location within the stated window
	"v_colocation_windows": "Proximity is not association: two numbers at the same cell site within the stated window of each other were in the same coverage area, which is not evidence that they met or know each other. " +
		"No chance baseline is computed here: how often two unrelated numbers would be at one site by chance is not shown.",
	// A7: possible meetings
	"v_meeting_candidates": "A candidate, not a finding: proximity and contact are not proof of a meeting. " +
		"The score is a documented formula (half from the number of co-located pairs, half from how short the gap to the last contact is), not a probability.",
}

// askViewCaveats returns the caveat lines of the analysis views a validated query reads, in the order of its sorted views.
func askViewCaveats(v *askValidated) []string {
	if v == nil {
		return nil
	}
	var out []string
	for _, view := range v.Views {
		if text, ok := askViewCaveatText[view]; ok {
			out = append(out, text)
		}
	}
	return out
}
