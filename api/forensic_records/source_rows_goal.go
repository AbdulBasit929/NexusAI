package main

import (
	"os"
	"strings"
)

// semanticAggregateMarkers is the ONE list of words that make a question ask for
// a quantity. It was inline in the `aggregate` case of `semanticFrameGoal`; it is
// lifted here unchanged so the A3.1 guard reads the same list rather than a
// second one that could drift from it -- two lists that disagree about one
// sentence is the defect pattern this project keeps paying for.
var semanticAggregateMarkers = []string{"average", "sum", "count", "total", "minimum", "maximum", "many"}

// A3.1 — a question asking HOW MANY records came from each source is a grouped
// count, not a request for the source rows. See the `source_rows` case in
// semanticFrameGoal. Default OFF until measured.
const sourceRowsQuantityGuardEnv = "FORENSIC_SOURCE_ROWS_QUANTITY_GUARD"

func sourceRowsQuantityGuardEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(sourceRowsQuantityGuardEnv)), "true") //nolint:forbidigo // forensic records product API reads its feature switches from the environment by design (docker compose); not LocalAI server config
}
