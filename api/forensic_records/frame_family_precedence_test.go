package main

import (
	"sort"
	"testing"
)

// A3.2 CENSUS. Every corpus question whose frame family is cross_family today,
// and where the record-type precedence would send it. Printed before it is
// trusted; any question that moves must be one we can justify.
func TestFrameFamilyPrecedenceCensus(t *testing.T) {
	paths := []string{
		"../../evaluation/golden_questions_v2.json",
		"../../evaluation/holdout_questions_v1.json",
		"../../evaluation/holdout_media_v1.json",
	}
	crossToday, moved := []string{}, []string{}
	for _, path := range paths {
		for _, question := range loadRoutingQuestions(t, path) {
			req := hybridQueryRequest{Query: question.Q}
			t.Setenv(frameFamilyPrecedenceEnv, "")
			before := extractSemanticFrame(req).FamilyHint
			t.Setenv(frameFamilyPrecedenceEnv, "true")
			after := extractSemanticFrame(req).FamilyHint
			if before == "cross_family" {
				crossToday = append(crossToday, question.ID+"  "+question.Q)
			}
			if before != after {
				moved = append(moved, question.ID+"  "+before+" -> "+after+"  "+question.Q)
			}
		}
	}
	sort.Strings(crossToday)
	sort.Strings(moved)
	t.Logf("cross_family today: %d", len(crossToday)) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	for _, line := range crossToday {
		t.Logf("   %s", line) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	t.Logf("family would CHANGE for %d:", len(moved)) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	for _, line := range moved {
		t.Logf("   %s", line) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}

func TestFrameFamilyPrecedence(t *testing.T) {
	cdr12 := "Which cell site handled the most calls?"

	t.Setenv(frameFamilyPrecedenceEnv, "")
	if got := extractSemanticFrame(hybridQueryRequest{Query: cdr12}).FamilyHint; got != "cross_family" {
		t.Fatalf("with the switch off the family must be unchanged (cross_family), got %q", got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}

	t.Setenv(frameFamilyPrecedenceEnv, "true")
	if got := extractSemanticFrame(hybridQueryRequest{Query: cdr12}).FamilyHint; got != "communications_cdr" {
		t.Fatalf("CDR-12 is a CDR ranking, got %q", got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	// Accepted only when the precedence lands on a family the question matched.
	if got := frameFamilyByRecordTypePrecedence(cdr12, []string{"tower_location", "subscriber_identity"}); got != "" {
		t.Fatalf("a precedence answer outside the matched families must be refused, got %q", got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}

	// THE EIGHT WRONG MOVES the first census found. Each must STAY cross_family:
	// documents and model-read media are not ANPR sightings.
	for id, question := range map[string]string{
		"DOC-01": "Search the case documents for mentions of plate MN1367",
		"DOC-04": "What does the case notes document say about plate ABC-123?",
		"H1":     "Which plates were read from the videos?",
		"H3":     "Which plate was visible longest in the video?",
		"M1":     "How many plate reads were produced from the images?",
		"M2":     "What is the average OCR confidence of the plate reads?",
		"M5":     "How many plate groups were tracked across the video frames?",
		"VID-01": "What license plates appear in the videos?",
	} {
		if got := extractSemanticFrame(hybridQueryRequest{Query: question}).FamilyHint; got != "cross_family" {
			t.Errorf("%s must stay cross_family (document or model-read media, not ANPR sightings), got %q", id, got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
}
