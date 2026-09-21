package forensictext

import "testing"

func TestConceptExplanationSeparatesCaseAnalysisFromConcepts(t *testing.T) {
	cases := map[string]bool{
		"What is IPDR?":                            true,
		"Explain what an IMEI is":                  true,
		"Explain the call type breakdown":          false,
		"Explain these results":                    false,
		"Explain the night activity pattern":       false,
		"What are the most frequent contacts?":     false,
		"Why are there so many calls at midnight?": false,
	}
	for question, want := range cases {
		if got := IsConceptExplanation(question, false); got != want {
			t.Errorf("%q: got %v, want %v", question, got, want)
		}
	}
}
