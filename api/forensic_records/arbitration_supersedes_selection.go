package main

import (
	"os"
	"strings"
)

// ARBITRATION SUPERSEDES THE REGISTERED SELECTION.
//
// IR arbitration swaps a verified typed plan into a request that the compiler
// had first matched to a registered operation. It clears BindingState, but the
// audit keeps `Selected` -- correctly, as a record of what was chosen first --
// and the handler then recomputed binding readiness FROM THAT SELECTION, just
// before deciding whether to clarify.
//
// Measured on the ladder-off arm, 2026-09-27: "Which cell site handled the most
// calls?" (CDR-12) had an accepted plan -- GROUP BY cell site, COUNT, DESC,
// limit 1 -- and still asked "Which target identifier should I analyze?",
// because the superseded `cdr.tower_activity` requires a target. The operation
// that needed the target was never going to run.
//
// Narrow on purpose: only a request that IS the arbitrated typed plan skips the
// recompute. That plan has already passed S6, S9 SHAPE and CONSTRAINT_APPLIED in
// resolveSemanticIRFallback. Every other request keeps the readiness check.
// See reports/arbitration-supersedes-20260928/.
const arbitrationSupersedesSelectionEnv = "FORENSIC_ARBITRATION_SUPERSEDES_SELECTION"

// bindingSupersededByArbitration is written to the audit instead of a readiness
// state computed for an operation that will not execute.
const bindingSupersededByArbitration = "SUPERSEDED_BY_ARBITRATED_PLAN"

func arbitrationSupersedesSelectionEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(arbitrationSupersedesSelectionEnv)), "true") //nolint:forbidigo // forensic records product API reads its feature switches from the environment by design (docker compose); not LocalAI server config
}

// registeredSelectionSuperseded reports whether the request about to execute is
// a typed plan adopted by IR arbitration, so the registered operation recorded
// in the audit will not run and its required parameters bind nothing.
func registeredSelectionSuperseded(req hybridQueryRequest) bool {
	return arbitrationSupersedesSelectionEnabled() &&
		req.SemanticPlannerAudit != nil &&
		req.SemanticPlannerAudit.IRArbitrated &&
		req.SourceNative != nil
}
