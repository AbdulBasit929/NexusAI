package main

import (
	"fmt"
	"strings"
)

// S9 verification — the checks that run AFTER a plan is compiled and BEFORE an
// answer is shown. They are what make expanding coverage safe: every check that
// fails converts a would-be wrong answer into a clarification, which an analyst
// can act on, where a confident wrong number is indistinguishable from a right
// one.
//
// CONSTRAINT_APPLIED and FAMILY already exist (deterministicUnboundConstraints,
// filterRankedByQuestionFamily). This file adds the two that were missing and
// that the 2026-09-21 live runs proved were needed:
//
//	SHAPE  a rank must be ordered and bounded; a count must be scalar.
//	       "1 CDR record matched this question" was returned for a question
//	       asking WHICH phone number made the most calls.
//	SCOPE  a filtered answer equal to the unfiltered total is a red flag, not a
//	       result. Every 12,912 the baseline produced was this.

const (
	s9ShapeViolation = "S9_SHAPE_VIOLATION:"
	s9ScopeViolation = "S9_SCOPE_VIOLATION:"
)

// s9QuestionShape classifies what shape of answer a question demands. It is
// deterministic and lexical; an unknown shape asserts nothing.
func s9QuestionShape(frame SemanticFrameV1, question string) string {
	switch frame.Goal {
	case "rank":
		// A ranking needs something to rank BY. "Which camera recorded the most
		// sightings" names a dimension; "what was the largest transaction" names
		// none — it is a scalar MAX over the whole set, and demanding a grouping
		// there refuses a correct answer. The frame marks both as "rank".
		if len(frame.GroupByHints) > 0 || semanticFrameWhichGroup.MatchString(question) {
			return "rank"
		}
		if _, magnitude := sourceNativeMagnitudeSuperlative(question); magnitude {
			return "scalar"
		}
		return "rank"
	case "aggregate":
		if len(frame.GroupByHints) > 0 {
			return "breakdown"
		}
		// A question naming several values of one curated field is a breakdown
		// even though it carries no "by X" phrasing: "incoming versus outgoing"
		// asks for both. S9 must read the same signal the compiler used, or it
		// refuses the very grouping the compiler correctly derived.
		if semanticQuestionNamesMultipleCuratedValues(question) {
			return "breakdown"
		}
		return "scalar"
	case "breakdown":
		// Unconditional, unlike `aggregate`. The goal only fires on EXPLICIT
		// breakdown language, so there is nothing to infer: the question said
		// it wanted a grouping. `verifySourceNativePlanShape` refuses a plan
		// with no GroupFields.
		return "breakdown"
	case "source_rows", "lookup":
		return "rows"
	}
	if sourceNativeSuperlativeCountsRows(question) {
		return "rank"
	}
	return ""
}

// verifySourceNativePlanShape rejects a plan whose shape cannot answer the
// question that was asked. A plan that compiles and executes but answers a
// different shape is the most dangerous outcome available: it returns a real
// number for the wrong question.
func verifySourceNativePlanShape(frame SemanticFrameV1, question string, plan *SourceNativePlanV1) error {
	if plan == nil {
		return nil
	}
	// DISTINCT is an OBLIGATION, not a preference. "How many unique phone
	// numbers appear as callers" asks for COUNT(DISTINCT msisdn) = 10; a plain
	// COUNT answers 8,642 and is indistinguishable from a right answer.
	//
	// Measured 2026-09-23: this fired the moment COUNT_DISTINCT became
	// expressible. Before that the question generated no plan and safely
	// clarified; making it expressible let a WRONG plan through instead. A new
	// capability has to arrive with the check that bounds it.
	if frame.Goal == "distinct" {
		distinct := false
		for _, measure := range plan.Measures {
			if strings.EqualFold(measure.Op, "COUNT_DISTINCT") {
				distinct = true
			}
		}
		if !distinct {
			return fmt.Errorf("%sa question asking for unique values compiled a plain count, "+
				"which counts rows and not distinct values", s9ShapeViolation)
		}
	}
	// MEASURE OBLIGATION. A question asking for an AVERAGE or a TOTAL cannot be
	// answered by counting rows -- "what is the average beam width" answered
	// with a row count is a different question, and the number looks just as
	// authoritative.
	//
	// Restricted to avg/sum DELIBERATELY, and the restriction was measured. A
	// frame measure of max/min also arrives from superlatives like "most",
	// where the correct plan expresses the extreme as GROUP BY + ORDER BY +
	// LIMIT rather than as a MAX aggregate: CDR-06, IPDR-02, IPDR-05 and TXN-03
	// are all CORRECT today with measure=max and a COUNT plan. Including
	// max/min here would refuse four right answers to catch one wrong one.
	//
	// "Average" and "total" have no such ranking reading.
	if frame.Measure == "avg" || frame.Measure == "sum" {
		want := map[string]string{"avg": "AVG", "sum": "SUM"}[frame.Measure]
		satisfied := false
		for _, measure := range plan.Measures {
			if strings.EqualFold(measure.Op, want) {
				satisfied = true
			}
		}
		if !satisfied && len(plan.Measures) > 0 {
			// Named the way an analyst reads it, not by the frame's token.
			asked := map[string]string{"avg": "an average", "sum": "a total"}[frame.Measure]
			return fmt.Errorf("%sa question asking for %s compiled %s instead, which answers a "+
				"different question", s9ShapeViolation, asked, planMeasureOps(plan))
		}
	}
	// RESTRICTION OBLIGATION. A value the analyst named through a MULTI-WORD
	// phrase that the plan never filtered on is a restriction the plan dropped.
	//
	// "How many server errors are in the access log?" answered 1,000 -- the
	// whole family -- where the truth is 46. The curated layer declares status
	// "500" with the synonym "server error" and says so in its description; the
	// plan simply did not use it.
	//
	// MULTI-WORD ONLY, and that was measured too. A single-word match is the
	// value naming itself: "how many CDR records are there?" names CALL merely
	// by containing "call", and seven CORRECT questions would be refused if
	// that counted. Across both corpora exactly two questions carry a
	// multi-word literal -- ACC-04, whose plan DOES filter on it, and H7, whose
	// plan does not.
	if unused := unusedPhraseRestrictions(question, plan); len(unused) > 0 {
		return fmt.Errorf("%sthe question names %s and the plan does not filter on it, so the "+
			"result would cover everything", s9ShapeViolation, strings.Join(unused, ", "))
	}
	// BOOLEAN RESTRICTION OBLIGATION. The rule above catches a dropped VALUE
	// literal; this catches a dropped BOOLEAN FIELD, which is the same defect
	// with no value to name. Rationale, measurements and the M3 trade are in
	// boolean_restriction.go. Behind FORENSIC_BOOLEAN_RESTRICTION, default off.
	if booleanRestrictionEnabled() {
		if unused := unusedBooleanRestrictions(question, plan); len(unused) > 0 {
			return fmt.Errorf("%sthe question restricts to %s and the plan neither filters nor groups "+
				"by it, so the result would cover every row", s9ShapeViolation, strings.Join(unused, ", "))
		}
	}
	// LOCATION OBLIGATION (A1d). A question opening with "where" answered by a
	// bare aggregate reports how many, not where. See location_obligation.go.
	if locationObligationUnmet(question, plan) {
		return fmt.Errorf("%sa question asking WHERE compiled %s with no location in its result, "+
			"which answers how many rather than where", s9ShapeViolation, planMeasureOps(plan))
	}
	switch s9QuestionShape(frame, question) {
	case "rank":
		// "Which X has the most Y" must name an X. Without a grouping there is
		// nothing to rank, and without an ordering the first row is arbitrary.
		if len(plan.GroupFields) == 0 && len(plan.Project) == 0 {
			return fmt.Errorf("%sa ranking question compiled with nothing to rank by", s9ShapeViolation)
		}
		if len(plan.GroupFields) > 0 && len(plan.Sort) == 0 {
			return fmt.Errorf("%sa ranking question compiled without an ordering, so the top row would be arbitrary", s9ShapeViolation)
		}
	case "scalar":
		// A plain count must not acquire a grouping it was never asked for.
		// This is D2's signature and it produced nondeterministic answers.
		//
		// "IT WAS NEVER ASKED FOR" is the whole content of the rule, and the
		// shape classifier alone cannot establish it. CDR-05 asks "how many
		// calls of each type are there?" -- a breakdown -- and the generator
		// produced exactly the right plan, GROUP BY cdr.call_type with a COUNT.
		// Extraction does not read "of each type" as a grouping signal, so
		// GroupByHints is empty, the question classifies as "scalar", and this
		// rule refused a grouping that WAS asked for. Measured 2026-09-24 from
		// the cached completion.
		//
		// So the question is asked of the CURATED LAYER, which is the shared
		// vocabulary the compiler resolves fields through, rather than of a
		// second lexical rule that could disagree with it all over again. A
		// grouping survives only when the question names the grouped field by
		// its display name or a declared synonym.
		//
		// D2 keeps its teeth: a spurious grouping comes from an embedding
		// scoring the whole question against a field the analyst never
		// mentioned, and such a field is by definition not named.
		if len(plan.GroupFields) > 0 && !questionNamesEveryGroupedField(question, plan.GroupFields) {
			return fmt.Errorf("%sa scalar question compiled a grouping by %s", s9ShapeViolation, strings.Join(plan.GroupFields, ", "))
		}
	case "breakdown":
		if len(plan.GroupFields) == 0 {
			return fmt.Errorf("%sa breakdown question compiled without a grouping", s9ShapeViolation)
		}
	}
	return nil
}

// s9ScopeSuspect reports whether a filtered result is indistinguishable from the
// unfiltered scope, which means the filter did nothing.
//
// The 2026-09-18 baseline answered "how many calls did <number> make" with
// 12,912 — every row in the case — for a number that appears in none of them.
// A filtered count that exactly equals the scope total is not evidence that the
// filter matched everything; it is evidence that it was dropped.
func s9ScopeSuspect(plan *SourceNativePlanV1, resultValue, scopeTotal float64) bool {
	if plan == nil || len(plan.Filters) == 0 || scopeTotal <= 0 {
		return false
	}
	if len(plan.Measures) != 1 || strings.ToUpper(plan.Measures[0].Op) != "COUNT" {
		return false
	}
	return resultValue == scopeTotal
}

// verifySourceNativeScope turns that suspicion into a refusal.
func verifySourceNativeScope(plan *SourceNativePlanV1, resultValue, scopeTotal float64) error {
	if s9ScopeSuspect(plan, resultValue, scopeTotal) {
		return fmt.Errorf("%sa filtered count equals the unfiltered scope total (%s); the filter did not narrow anything",
			s9ScopeViolation, formatAnswerNumber(scopeTotal))
	}
	return nil
}

// questionNamesEveryGroupedField reports that the analyst named each field the
// plan groups by, using the curated layer's display names and synonyms.
//
// EVERY field, not any: a plan grouping by two dimensions where the question
// named one is still partly a grouping nobody asked for, and the safe reading
// of a partial match is refusal.
//
// The layer is consulted rather than the question's phrasing because the layer
// is what the COMPILER resolves fields through. A verifier that re-derives the
// signal from the question text independently is free to disagree with the
// compiler about the same sentence -- which is the defect this replaces.
func questionNamesEveryGroupedField(question string, groupFields []string) bool {
	if len(groupFields) == 0 {
		return false
	}
	layer, _ := defaultSemanticLayer()
	if layer == nil {
		// No layer means no evidence that the analyst named anything, and an
		// unverified grouping is refused rather than assumed.
		return false
	}
	stems := _semanticLayerQuestionTokens(question)
	for _, fieldID := range groupFields {
		field, ok := layer.FieldByID(fieldID)
		if !ok {
			return false
		}
		named := false
		for _, phrase := range append([]string{field.DisplayName}, field.Synonyms...) {
			if strings.TrimSpace(phrase) == "" {
				continue
			}
			if _semanticLayerPhraseInQuestion(phrase, stems) {
				named = true
				break
			}
		}
		if !named {
			return false
		}
	}
	return true
}

// planMeasureOps renders what the plan actually computes, so a refusal names
// the mismatch instead of merely asserting one.
func planMeasureOps(plan *SourceNativePlanV1) string {
	ops := make([]string, 0, len(plan.Measures))
	for _, measure := range plan.Measures {
		ops = append(ops, strings.ToUpper(measure.Op))
	}
	if len(ops) == 0 {
		return "no aggregate"
	}
	return strings.Join(ops, "+")
}

// unusedPhraseRestrictions lists values the question named through a
// multi-word curated synonym that the plan never filtered on.
func unusedPhraseRestrictions(question string, plan *SourceNativePlanV1) []string {
	declared := semanticLayerPhraseLiterals(question)
	if len(declared) == 0 || plan == nil {
		return nil
	}
	filtered := map[string]bool{}
	for _, filter := range plan.Filters {
		for _, value := range append([]string{filter.Value}, filter.Values...) {
			if trimmed := strings.TrimSpace(value); trimmed != "" {
				filtered[strings.ToUpper(trimmed)] = true
			}
		}
	}
	unused := []string{}
	for _, value := range declared {
		if !filtered[strings.ToUpper(strings.TrimSpace(value))] {
			unused = append(unused, value)
		}
	}
	return unused
}
