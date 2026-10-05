package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// A1a — A STATED CONDITION IS AN OBLIGATION, exactly as a named identifier is.
//
// Measured 2026-09-27 (reports/adhoc-runtime-20260927/): "Show me all the calls
// that lasted longer than ten minutes" answered "20 CDR records matched this
// question" with `"filters": []` in the plan. The duration condition never
// reached the SQL, and a bounded page of rows was stated as the answer.
//
// S9 CONSTRAINT_APPLIED already refuses a plan that drops a named IDENTIFIER
// (`answerNamesUnfilteredTarget`, ir_crosscheck.go) or an explicit DATE RANGE
// (`deterministicUnboundConstraints`). A magnitude condition is the same
// obligation in a third shape and was simply never covered.
//
// WHY THE GUARD LIVES HERE, AND NOT IN THE COMPILER. There is more than one
// producer of a `SourceNativePlanV1` -- the deterministic compiler
// (deterministic_semantic_compiler.go:707) and the dynamic planner
// (semantic_dynamic_plan.go:691), plus the follow-up contract path. A check
// inside one of them is silently absent for questions that took another route.
// `preExecutionWithhold` is where every route converges before execution, and
// it already hosts three guards of exactly this shape.
//
// SCOPE IS DELIBERATELY NARROW. This covers a comparison against a NUMBER.
// It does NOT cover relational conditions such as "do any subscribers share the
// same handset", which need a different detector and their own measurement;
// widening this one to reach them would make it vague, and a vague guard on the
// answer path costs correct answers. That case is named in the report as
// uncovered rather than half-handled.
const constraintObligationsEnv = "FORENSIC_CONSTRAINT_OBLIGATIONS"

func constraintObligationsEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(constraintObligationsEnv)), "true") //nolint:forbidigo // forensic records product API reads its feature switches from the environment by design (docker compose); not LocalAI server config
}

// Comparison phrases that bound a quantity. Each must be followed by a number
// for the condition to count: "longer than ten minutes" is an obligation,
// "longer" alone is a superlative and belongs to `rank`, which already works.
//
// `between X and Y` is included because it is the one two-sided form the
// curated layer declares (`allowed_filters: [..., BETWEEN, ...]`).
var magnitudeConditionPhrase = regexp.MustCompile(`(?i)\b(` +
	`longer than|shorter than|greater than|less than|fewer than|more than|` +
	`larger than|smaller than|older than|newer than|` +
	`at least|at most|no more than|no fewer than|no less than|` +
	`exceeding|exceeds|exceed|above|below|over|under|` +
	`between` +
	`)\s+(?:\$)?([0-9][0-9,.]*|` + spelledNumberAlternation + `)\b`)

// Spelled numbers, because an analyst writes "ten minutes" as readily as "10".
// Kept to the values a person actually types in a threshold.
const spelledNumberAlternation = `one|two|three|four|five|six|seven|eight|nine|ten|` +
	`eleven|twelve|fifteen|twenty|thirty|forty|fifty|sixty|` +
	`hundred|thousand|million`

// questionStatesMagnitudeCondition returns the condition the analyst wrote, or
// "" when the question states none. The returned text is echoed back to the
// analyst, so it is the ORIGINAL casing and wording, never a normalisation.
func questionStatesMagnitudeCondition(query string) string {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return ""
	}
	match := magnitudeConditionPhrase.FindStringIndex(trimmed)
	if match == nil {
		return ""
	}
	condition := strings.TrimSpace(trimmed[match[0]:match[1]])
	// Carry the unit word when there is one ("longer than ten minutes"), so the
	// refusal repeats what was asked rather than a truncated fragment.
	rest := trimmed[match[1]:]
	if unit := leadingUnitWord(rest); unit != "" {
		condition += " " + unit
	}
	return condition
}

var unitWord = regexp.MustCompile(`^\s+([A-Za-z]{2,18})\b`)

func leadingUnitWord(rest string) string {
	found := unitWord.FindStringSubmatch(rest)
	if found == nil {
		return ""
	}
	// A conjunction or verb is not a unit; "between 10 and 20" must not become
	// "between 10 and".
	switch strings.ToLower(found[1]) {
	case "and", "or", "of", "in", "on", "at", "to", "from", "the", "a", "an",
		"was", "were", "is", "are", "that", "which", "who", "with", "for":
		return ""
	}
	return found[1]
}

// planBindsARangeComparison reports whether the executed plan carries a filter
// that actually compares against a bound.
//
// EQ and CONTAINS do not satisfy a magnitude condition: a plan that filtered
// `call_type EQ SMS` has bound something, but not "longer than ten minutes".
func planBindsARangeComparison(plan *SourceNativePlanV1) bool {
	if plan == nil {
		return false
	}
	for _, filter := range plan.Filters {
		switch strings.ToUpper(strings.TrimSpace(filter.Op)) {
		case "GT", "GTE", "LT", "LTE", "BETWEEN":
			return true
		}
	}
	for _, having := range plan.Having {
		switch strings.ToUpper(strings.TrimSpace(having.Op)) {
		case "GT", "GTE", "LT", "LTE", "BETWEEN":
			return true
		}
	}
	return false
}

// answerIgnoresStatedCondition is the guard: the analyst stated a magnitude
// condition and the plan that ran compares against nothing.
//
// Requires a typed plan. Where none executed there is nothing to inspect, and
// a different guard already covers the unverified-plan case.
func answerIgnoresStatedCondition(req hybridQueryRequest) bool {
	if !constraintObligationsEnabled() {
		return false
	}
	if req.SourceNative == nil {
		return false
	}
	if questionStatesMagnitudeCondition(req.Query) == "" {
		return false
	}
	return !planBindsARangeComparison(req.SourceNative)
}

// conditionNotAppliedReason names the condition back to the analyst. Naming it
// is the whole point: a person can only correct what they are told was not used.
func conditionNotAppliedReason(req hybridQueryRequest) *withholdReason {
	condition := questionStatesMagnitudeCondition(req.Query)
	return &withholdReason{
		Code: withholdConditionNotApplied,
		Question: fmt.Sprintf("I did not apply the condition %q to a curated field, so this result "+
			"counts every record of its kind rather than only those you asked for. Name the field "+
			"the condition applies to and I will compute it.", condition),
		Guardrail: fmt.Sprintf("No result was stated because the question states the condition %q "+
			"while the executed plan compares against nothing. A total presented as though it were "+
			"filtered is a confident wrong answer, not an approximation.", condition),
		Detail: fmt.Sprintf("condition=%q plan_filters=%d", condition, len(req.SourceNative.Filters)),
	}
}
