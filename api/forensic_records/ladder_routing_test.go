package main

import "testing"

// The gate's contract. Every other switch on this path is off-by-default
// because it adds a capability; this one is ON by default because it gates an
// EXISTING behaviour, and default-off would silently reroute every question the
// moment it merged.
func TestLadderRoutingIsOnByDefault(t *testing.T) {
	if !ladderRoutingEnabled() {
		t.Fatal("the ladder gates existing behaviour; default must reproduce today bit-for-bit")
	}
	t.Setenv(ladderRoutingEnv, "false")
	if ladderRoutingEnabled() {
		t.Fatal("FORENSIC_LADDER_ROUTING=false must disable the ladder")
	}
	// Anything that is not an explicit "false" leaves it on. A typo must not
	// silently reroute the product.
	for _, value := range []string{"", "true", "0", "off", "no", "disabled"} {
		t.Setenv(ladderRoutingEnv, value)
		if !ladderRoutingEnabled() {
			t.Errorf("%q must not disable the ladder; only an explicit \"false\" does", value)
		}
	}
}

// With the ladder off, template selection must ABSTAIN rather than guess, so
// the compiler decides. That fall-through already exists -- this asserts the
// switch actually reaches it.
func TestLadderOffAbstainsInsteadOfGuessing(t *testing.T) {
	// The phrase must be one the LADDER routes, or this test passes whether or
	// not the switch does anything. The first attempt used "show the breakdown
	// of HTTP status codes", which resolves to "" with the ladder both on and
	// off -- it never reached the ladder at all, so the assertion was vacuous.
	//
	// These are lifted verbatim from `choosePreciseTemplate`'s own keyword
	// lists, so each one provably enters the ladder.
	routed := map[string]string{
		"cdr events by call type":          "call_type_breakdown",
		"traffic by protocol":              "ipdr_protocol_breakdown",
		"busiest anpr cameras":             "anpr_camera_activity",
		"most frequent supplied locations": "top_locations",
	}

	discriminating := 0
	for question, want := range routed {
		t.Setenv(ladderRoutingEnv, "true")
		withLadder := chooseTemplate(question, "")

		t.Setenv(ladderRoutingEnv, "false")
		withoutLadder := chooseTemplate(question, "")

		t.Logf("%-34s on -> %-26q off -> %q", question, withLadder, withoutLadder)

		if withLadder != want {
			// The ladder's own keyword did not route here: family guards can
			// still veto it. Not a failure of the switch, but it cannot
			// discriminate either, so it does not count.
			continue
		}
		discriminating++
		if withoutLadder != "" {
			t.Errorf("%q: ladder off must abstain so the compiler decides, got %q",
				question, withoutLadder)
		}
	}
	if discriminating == 0 {
		t.Fatal("no phrase actually routed through the ladder; this test proves nothing " +
			"and must be re-derived from choosePreciseTemplate's current keywords")
	}
	t.Logf("discriminating cases: %d", discriminating)
}

// An EXPLICIT template from the caller is not the ladder and must still be
// honoured with routing off -- the API contract allows a caller to name one.
func TestExplicitTemplateSurvivesTheLadderBeingOff(t *testing.T) {
	t.Setenv(ladderRoutingEnv, "false")
	if got := chooseTemplate("anything at all", "canonical_records"); got != "canonical_records" {
		t.Fatalf("an explicitly requested template must still bind, got %q", got)
	}
}

// The exact-example-query match is also not the ladder: it is an identity
// lookup against the catalogue, and it runs before the ladder. It must survive
// too, or the catalogue's own examples stop working while the ladder is off.
func TestCatalogueExampleMatchSurvivesTheLadderBeingOff(t *testing.T) {
	templates := supportedQueryTemplates()
	if len(templates) == 0 {
		t.Skip("no templates registered")
	}
	var example, want string
	for _, candidate := range templates {
		if candidate.ExampleQuery != "" {
			example, want = candidate.ExampleQuery, candidate.Name
			break
		}
	}
	if example == "" {
		t.Skip("no catalogue example query to test with")
	}
	t.Setenv(ladderRoutingEnv, "false")
	if got := chooseTemplate(example, ""); got != want {
		t.Fatalf("catalogue example %q resolved to %q, want %q", example, got, want)
	}
}
