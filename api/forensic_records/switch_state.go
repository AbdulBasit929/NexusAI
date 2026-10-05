package main

import (
	"log/slog"
	"os"
	"sort"
	"strings"
)

// P3. THE RUNNING SERVER SAYS WHICH SWITCHES IT IS RUNNING WITH.
//
// Measured 2026-09-25 (reports/config-drift-20260925/): five settings,
// including VERIFIED_ONLY, ran at compose's `:-false` defaults for five hours
// and a golden suite was scored that way before anyone noticed. An omitted
// switch is indistinguishable from one set false, and one omitted from compose
// never reaches the container at all. Every measurement since has asserted the
// switch state from `docker inspect`; this puts the same fact in the service's
// own startup log, where anyone can read it.
//
// Logging only: it changes no answer. The registry below lists the behaviour
// switches this service reads that are declared in docker-compose;
// TestBehaviourSwitchRegistryMatchesCompose keeps the two in step.
var behaviourSwitches = []string{
	"FORENSIC_ANSWER_STATES_VALUES", "FORENSIC_API_AUTH_REQUIRED",
	"FORENSIC_ARBITRATION_SUPERSEDES_SELECTION", "FORENSIC_BOOLEAN_RESTRICTION",
	"FORENSIC_BREAKDOWN_GOAL", "FORENSIC_CITATION_TRUTH_STATE", "FORENSIC_CLARIFICATION_NOT_RESULT",
	"FORENSIC_COUNT_NAMES_FIELD",
	"FORENSIC_CONSTRAINT_OBLIGATIONS", "FORENSIC_CONVERSATION_FRONT_DOOR", "FORENSIC_CONVERSATION_ROUTER", "FORENSIC_DERIVED_ARTIFACT_EXECUTION",
	"FORENSIC_DERIVED_RESULT_LABELS", "FORENSIC_DROP_INVENTED_FILTERS", "FORENSIC_EMPTY_AGGREGATE_HONEST",
	"FORENSIC_EVIDENCE_SEARCH_FIRST", "FORENSIC_HEADLINE_STATES_FILTERS",
	"FORENSIC_FRAME_FAMILY_PRECEDENCE", "FORENSIC_GOAL_TAXONOMY_SLICES", "FORENSIC_GOVERNED_SQL", "FORENSIC_GOVERNED_SQL_FIRST",
	"FORENSIC_IDENTIFIER_BINDING", "FORENSIC_IR_ARBITRATION", "FORENSIC_IR_CROSSCHECK",
	"FORENSIC_IR_FALLBACK", "FORENSIC_IR_SHADOW", "FORENSIC_LADDER_ROUTING",
	"FORENSIC_LOCATION_OBLIGATION", "FORENSIC_MEASURE_DENOMINATOR",
	"FORENSIC_MEDIA_FAMILY_ROUTING", "FORENSIC_PII_MASKED_PROJECTION", "FORENSIC_PLAN_CACHE",
	"FORENSIC_PLATE_SEARCH_ONLY",
	"FORENSIC_RANGE_FILTERS", "FORENSIC_RECORD_TYPE_VALIDATION",
	"FORENSIC_RELATIONAL_CONDITIONS", "FORENSIC_RESULT_COLUMN_LABELS", "FORENSIC_ROW_COUNT_HEADLINE_GUARD",
	"FORENSIC_SOURCE_ROWS_QUANTITY_GUARD", "FORENSIC_TABLE_HEADER_CASING", "FORENSIC_TEMPLATE_STATES_RESULT",
	"FORENSIC_TEXT_BROWSE_GUARD",
	"FORENSIC_TEXT_SEARCH_NOT_ABSENCE", "FORENSIC_VERIFIED_ONLY",
}

// switchState splits the registry into switches set true, set false, set to
// something else, and not set at all. Values are only ever booleans here, so
// nothing secret can be logged.
func switchState(lookup func(string) (string, bool)) (on, off, other, unset []string) {
	for _, name := range behaviourSwitches {
		value, present := lookup(name)
		switch normalized := strings.ToLower(strings.TrimSpace(value)); {
		case !present:
			unset = append(unset, name)
		case normalized == "true":
			on = append(on, name)
		case normalized == "false" || normalized == "":
			off = append(off, name)
		default:
			other = append(other, name)
		}
	}
	for _, list := range [][]string{on, off, other, unset} {
		sort.Strings(list)
	}
	return on, off, other, unset
}

// logSwitchState writes one line with the full switch state, and a warning
// naming any switch that is not set, since its code default then applies silently.
func logSwitchState() {
	on, off, other, unset := switchState(os.LookupEnv) //nolint:forbidigo // forensic records product API reads its feature switches from the environment by design (docker compose); not LocalAI server config
	slog.Info("behaviour switches", "on", strings.Join(on, ","), "off", strings.Join(off, ","),
		"on_count", len(on), "off_count", len(off))
	if len(other) > 0 {
		slog.Warn("behaviour switches set to a value other than true or false; treated as off",
			"switches", strings.Join(other, ","))
	}
	if len(unset) > 0 {
		slog.Warn("behaviour switches not set in the environment; their code defaults apply",
			"switches", strings.Join(unset, ","))
	}
}
