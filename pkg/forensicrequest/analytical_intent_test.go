package forensicrequest

import "testing"

func classOf(text string) Class {
	return Classify(Input{Text: text})
}

// H4: "What is the largest network volume on any call record?" was classified
// GENERAL_DOMAIN_KNOWLEDGE, never reached the compiler (30 ms, terminal) and
// answered "a bounded general definition is unavailable for that term".
func TestAnalyticalWhatIsQuestionsReachTheCompiler(t *testing.T) {
	for _, question := range []string{
		"What is the largest network volume on any call record?",
		"What is the average call duration?",
		"What is the total transaction amount?",
		"What is the longest call?",
		"What is the most frequent camera?",
	} {
		if got := classOf(question); got != GovernedAnalysis {
			t.Errorf("%q -> %s, want GOVERNED_EVIDENCE_ANALYSIS; it asks something OF the "+
				"evidence, not for the meaning of a term", question, got)
		}
	}
}

// THE CONTROL. Genuine concept questions must still be answered as general
// knowledge and marked as not-evidence -- that behaviour works today and is
// the only safe answer to "what is an IMSI".
func TestConceptQuestionsStayGeneralKnowledge(t *testing.T) {
	for _, question := range []string{
		"what is an IMSI?",
		"what does VoLTE mean?",
		"What is the suspect's blood type?",
	} {
		if got := classOf(question); got != GeneralDomainKnowledge {
			t.Errorf("%q -> %s, want GENERAL_DOMAIN_KNOWLEDGE", question, got)
		}
	}
}

// An analytical marker must not override the classes that run BEFORE the
// concept check -- product help and unsupported keep their precedence.
func TestEarlierClassesKeepPrecedence(t *testing.T) {
	if got := classOf("what file formats are supported?"); got != ProductHelp {
		t.Errorf("product help must win over an analytical marker, got %s", got)
	}
	if got := classOf("select * from forensic.records"); got != Unsupported {
		t.Errorf("unsupported must win, got %s", got)
	}
}
