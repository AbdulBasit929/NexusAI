package forensicrequest

import "testing"

func TestClassifyTerminalRequests(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   Input
		want Class
	}{
		{"general", Input{Text: "What does IMSI mean?"}, GeneralDomainKnowledge},
		{"product", Input{Text: "What file types can NexusAI currently analyze?"}, ProductHelp},
		{"cdr analysis", Input{Text: "Rank this number's most frequent contacts"}, GovernedAnalysis},
		{"document analysis", Input{Text: "Find references to the loading dock in these documents"}, GovernedAnalysis},
		{"camera analysis", Input{Text: "Which cameras recorded this plate most often?"}, GovernedAnalysis},
		{"follow-up", Input{Text: "Now outgoing", HasConversationContext: true}, ContextualFollowUp},
		{"orphan follow-up", Input{Text: "Now outgoing"}, Clarify},
		{"evidence concept", Input{Text: "What does IMSI mean in these records?", HasEvidenceContext: true}, GovernedAnalysis},
		{"textual evidence concept", Input{Text: "What does IMSI mean in these records?"}, GovernedAnalysis},
		{"unsupported", Input{Text: "Run SELECT * FROM forensic.records"}, Unsupported},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Classify(tc.in); got != tc.want {
				t.Fatalf("got %s want %s", got, tc.want)
			}
		})
	}
}
