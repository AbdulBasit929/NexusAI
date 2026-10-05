package main

import (
	"sort"
	"strings"
	"testing"
)

// A3.2, SECOND LOOK. The frame-family fix corrected CDR-12's family and the
// verdict did not move: the ladder-off arm showed the deterministic compiler
// choosing EXECUTE_REGISTERED with forensics.top_locations -- an operation that
// needs a TARGET -- for "Which cell site handled the most calls?", which names
// none. The operation can then only ask "Which target identifier should I
// analyze?".
//
// Census: every corpus question where the registered match wins with an
// operation that requires a target while the question supplies no identifier.
// Mirrors the ranking applyDeterministicSemanticCompiler performs (rank, then
// filter by question family); the catalog is nil, as in its first call.
func TestTargetRequiredRegisteredMatchCensus(t *testing.T) {
	paths := []string{
		"../../evaluation/golden_questions_v2.json",
		"../../evaluation/holdout_questions_v1.json",
		"../../evaluation/holdout_media_v1.json",
	}
	hits := []string{}
	for _, path := range paths {
		for _, question := range loadRoutingQuestions(t, path) {
			req := hybridQueryRequest{Query: question.Q}
			frame := extractSemanticFrame(req)
			candidates := semanticOperationCandidates(req)
			ranked, scores := rankSemanticOperationsForFrame(req.Query, frame, candidates)
			ranked, scores, _ = filterRankedByQuestionFamily(req.Query, ranked, scores)
			top, matched, _ := deterministicRegisteredMatch(frame, ranked, scores, nil)
			if !matched || len(frame.Identifiers) > 0 {
				continue
			}
			if !containsString(top.RequiredParameters, "target") {
				continue
			}
			hits = append(hits, question.ID+"  op="+top.OperationID+" required="+strings.Join(top.RequiredParameters, ",")+"  "+question.Q)
		}
	}
	sort.Strings(hits)
	t.Logf("registered match needs a target the question does not supply: %d", len(hits)) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	for _, line := range hits {
		t.Logf("   %s", line) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}
