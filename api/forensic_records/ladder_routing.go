package main

import (
	"os"
	"strings"
)

// The keyword routing ladder, and the switch that lets it be removed by
// measurement rather than by hope.
//
// `choosePreciseTemplate` is 253 lines of `containsAny(question, [...]) ->
// template name`, and it is the abstraction the product owner ruled out: it is
// PER-QUESTION, so it can never cover a phrasing nobody wrote in advance, and
// every miss produces a confident wrong answer. Deleting it is Phase 3's
// remaining deliverable.
//
// IT IS NOT DELETED BY THIS SWITCH. Cutting 250+ lines of routing in one change
// and trusting the suite to catch the damage is the move that produced seven
// confident-wrong answers across five attempts earlier in this session. The
// safe order for removing code this size is DISABLE -> MEASURE -> CUT, and this
// is the disable.
//
// DEFAULT ON, deliberately and unlike every other switch on this path. The
// others (FORENSIC_VERIFIED_ONLY, FORENSIC_PLAN_CACHE, FORENSIC_IR_FALLBACK)
// gate NEW capabilities, so off-by-default means "no change". This one gates an
// EXISTING behaviour, so default-off would silently change routing for every
// question the moment it merged. Default-on reproduces today bit-for-bit.
//
// What makes turning it off safe is that an abstaining ladder is already a
// designed outcome: `chooseTemplate` ends with
//
//	// The ladder abstains; the semantic compiler / dynamic SQL path decides.
//	return ""
//
// and 39 of 62 questions already reach the compiler that way.
//
// HONEST LIMIT ON THE MEASUREMENT: the 62-question corpus cannot fully validate
// this deletion. The ladder's whole premise was covering phrasings nobody wrote
// down, so the corpus under-represents exactly what it does. A clean run is
// necessary, not sufficient — which is why the switch stays after the code is
// cut, not why the measurement is skipped.
const ladderRoutingEnv = "FORENSIC_LADDER_ROUTING"

func ladderRoutingEnabled() bool {
	return !strings.EqualFold(strings.TrimSpace(os.Getenv(ladderRoutingEnv)), "false")
}
