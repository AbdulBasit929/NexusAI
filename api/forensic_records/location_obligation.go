package main

import (
	"os"
	"regexp"
	"strings"
)

// A1d. LOCATION OBLIGATION -- an S9 SHAPE rule.
//
// "Where was 923001110001 seen according to the call records?" answered
// "There are 447 CDR records involving 923001110001 in this case." (H11,
// WRONG in every recorded arm.) The IR plan filtered on the number, required
// `cdr.location IS NOT NULL` -- it knew the question was about location -- and
// then COUNTED. A count answers "how many", not "where", and 447 is also the
// count of the wrong field (originating_number, not msisdn). The corpus note
// records the honest answer: Airport Road, DHA Phase 5, Gulberg, Liberty Market
// and Model Town.
//
// Every other S9 shape rule keys on the frame's goal. "Where" has no goal of its
// own -- the frame reads H11 as a lookup -- so this reads the interrogative.
//
// NARROW ON PURPOSE: the question must OPEN with "where", and the plan must be a
// bare aggregate -- measures, with no grouping and no projection, so it carries
// no location to report at all. A plan grouped by a location (NEG-02, "Where was
// plate ZZZ-0000 seen?", CORRECT) or listing rows is untouched. Census over the
// recorded responses of both the ladder-on and the ladder-off arms: of the four
// corpus questions that open with "where", exactly H11 compiled a bare
// aggregate.
//
// This refuses; it computes nothing. Computing H11 needs the target bound to the
// right number field, which is the A3.3 identifier-binding work.
const locationObligationEnv = "FORENSIC_LOCATION_OBLIGATION"

func locationObligationEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(locationObligationEnv)), "true") //nolint:forbidigo // forensic records product API reads its feature switches from the environment by design (docker compose); not LocalAI server config
}

var whereInterrogative = regexp.MustCompile(`(?i)^\s*(?:and\s+|so\s+)?where\b`)

// questionAsksWhere reports a question whose first word asks for a place.
func questionAsksWhere(question string) bool {
	return whereInterrogative.MatchString(question)
}

// planIsBareAggregate reports a plan that computes a measure over the whole
// filtered set and returns no dimension: nothing in its result could name a place.
func planIsBareAggregate(plan *SourceNativePlanV1) bool {
	return plan != nil && len(plan.Measures) > 0 && len(plan.GroupFields) == 0 && len(plan.Project) == 0
}

// locationObligationUnmet is the rule: a WHERE question compiled a bare aggregate.
func locationObligationUnmet(question string, plan *SourceNativePlanV1) bool {
	return locationObligationEnabled() && questionAsksWhere(question) && planIsBareAggregate(plan)
}
