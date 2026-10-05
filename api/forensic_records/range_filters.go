package main

import (
	"os"
	"regexp"
	"strings"
)

// A1b — BIND A NUMERIC COMPARISON THE ANALYST STATED TO THE FIELD THEY NAMED.
//
// A1a made a dropped condition REFUSE. This makes it ANSWER, for the questions
// where the curated layer can justify the filter.
//
// The whole downstream path already exists and is untouched:
//   - the layer declares `allowed_filters: [..., GT, GTE, LT, LTE, ...]` on
//     numeric fields,
//   - `compileSourceNativeFrameFilters` resolves the field and enforces
//     `allowed_filters`,
//   - `sourceNativeFilterOpForFrame` maps the operator,
//   - the source-native executor runs it.
//
// Only the extraction was missing. This is the extraction, and it is modelled
// on `semanticLayerValueFilters`, which does the same job for declared VALUE
// literals ("active", "SMS").
//
// WHAT THIS DELIBERATELY DOES NOT DO, and why
//
//  1. NO UNIT CONVERSION, EVER. Surveyed 2026-09-27: of every range-filterable
//     field in the curated layer, NOT ONE declares a `unit`. The only `unit:`
//     in the layer is on `cdr.call_duration_seconds`, which is a derived METRIC
//     (`expression: timestamp_diff_seconds(...)`) with no `allowed_filters` at
//     all. So there is no declared basis anywhere for turning "ten minutes"
//     into 600, and inventing one would be a fabricated number.
//
//     Therefore: a condition that carries a UNIT WORD binds nothing and A1a
//     withholds it. "Calls longer than ten minutes" stays refused -- honestly,
//     and with its reason named -- until duration filtering exists as its own
//     work item.
//
//  2. NO EMBEDDING SIGNAL. The field is resolved from the layer's own declared
//     synonyms and display name. A filter bound to a field the analyst did not
//     name answers a different question confidently, which is the defect
//     `sourceNativeFieldByExactName` was written to prevent.
//
//  3. AMBIGUITY BINDS NOTHING. If two curated fields match the question, the
//     filter is not guessed. A1a then withholds, which is the correct outcome.
const rangeFiltersEnv = "FORENSIC_RANGE_FILTERS"

func rangeFiltersEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(rangeFiltersEnv)), "true") //nolint:forbidigo // forensic records product API reads its feature switches from the environment by design (docker compose); not LocalAI server config
}

// The comparison, captured with its operator and its number. `between` is
// absent on purpose: it needs two bounds and a different shape, and a
// half-applied BETWEEN is a wrong answer rather than a missing one.
var rangeComparisonPhrase = regexp.MustCompile(`(?i)\b(` +
	`greater than or equal to|less than or equal to|` +
	`no fewer than|no less than|no more than|at least|at most|` +
	`longer than|shorter than|greater than|less than|fewer than|more than|` +
	`larger than|smaller than|older than|newer than|` +
	`exceeding|exceeds|exceed|above|below|over|under` +
	`)\s+(?:\$)?([0-9][0-9,]*(?:\.[0-9]+)?)\b`)

func rangeOperatorFor(phrase string) string {
	switch strings.ToLower(strings.TrimSpace(phrase)) {
	case "at least", "no fewer than", "no less than", "greater than or equal to":
		return "GTE"
	case "at most", "no more than", "less than or equal to":
		return "LTE"
	case "longer than", "greater than", "more than", "larger than", "newer than",
		"exceeding", "exceeds", "exceed", "above", "over":
		return "GT"
	case "shorter than", "less than", "fewer than", "smaller than", "older than",
		"below", "under":
		return "LT"
	}
	return ""
}

// statedComparison is one numeric bound the analyst wrote.
type statedComparison struct {
	Op      string // GT, GTE, LT, LTE
	Literal string // the number, commas stripped
	Unit    string // a unit word following the number, or "" when there is none
}

// questionStatedComparison returns the single comparison the question states.
// Two comparisons in one question is a range, which is not this item's shape,
// so it returns nothing and the question is withheld rather than half-applied.
func questionStatedComparison(question string) (statedComparison, bool) {
	trimmed := strings.TrimSpace(question)
	matches := rangeComparisonPhrase.FindAllStringSubmatchIndex(trimmed, -1)
	if len(matches) != 1 {
		return statedComparison{}, false
	}
	match := matches[0]
	op := rangeOperatorFor(trimmed[match[2]:match[3]])
	if op == "" {
		return statedComparison{}, false
	}
	return statedComparison{
		Op:      op,
		Literal: strings.ReplaceAll(trimmed[match[4]:match[5]], ",", ""),
		Unit:    leadingUnitWord(trimmed[match[1]:]),
	}, true
}

// semanticLayerRangeFilters binds the stated comparison to the one curated
// field the question names, or returns nothing.
//
// Mirrors `semanticLayerValueFilters`: the layer's own vocabulary decides, the
// catalogue descriptor enforces what is permitted, and nothing is inferred.
func semanticLayerRangeFilters(question string, entity SemanticLayerEntityV1, catalog []FieldDescriptorV1) []SourceNativeFilterV1 {
	comparison, ok := questionStatedComparison(question)
	if !ok {
		return nil
	}
	// A unit word means the number is not in the field's own terms, and no
	// filterable field in the layer declares a unit to convert against.
	if comparison.Unit != "" {
		return nil
	}
	descriptors := map[string]FieldDescriptorV1{}
	for _, field := range catalog {
		descriptors[field.FieldID] = field
	}
	stems := _semanticLayerQuestionTokens(question)
	matched := []SourceNativeFilterV1{}
	for _, field := range entity.Fields {
		descriptor, present := descriptors[field.ID]
		if !present || !containsString(descriptor.AllowedFilters, comparison.Op) {
			continue
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
			continue
		}
		matched = append(matched, SourceNativeFilterV1{
			FieldID: field.ID, Op: comparison.Op, Value: comparison.Literal, Values: []string{},
		})
	}
	// Exactly one, or nothing. Two curated fields matching the question is an
	// ambiguity the analyst must resolve, not one this code may pick from.
	if len(matched) != 1 {
		return nil
	}
	return matched
}

// appendSourceNativeRangeFilters is the A1b entry point, shaped exactly like
// `appendSourceNativeValueFilters` so both enter the plan by the same door.
func appendSourceNativeRangeFilters(existing []SourceNativeFilterV1, question string, catalog []FieldDescriptorV1) []SourceNativeFilterV1 {
	if !rangeFiltersEnabled() {
		return existing
	}
	layer, _ := defaultSemanticLayer()
	if layer == nil {
		return existing
	}
	entity, ok := layer.EntityByFamily(semanticQuestionFamily(question))
	if !ok {
		return existing
	}
	bound := map[string]bool{}
	for _, filter := range existing {
		bound[filter.FieldID] = true
	}
	out := append([]SourceNativeFilterV1(nil), existing...)
	for _, filter := range semanticLayerRangeFilters(question, entity, catalog) {
		if bound[filter.FieldID] || len(out) >= sourceNativeFilterLimit {
			continue
		}
		out = append(out, filter)
		bound[filter.FieldID] = true
	}
	return out
}
