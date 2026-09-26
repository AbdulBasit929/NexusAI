package main

import (
	"os"
	"sort"
	"strings"
)

// A NAMED BOOLEAN FIELD IS A RESTRICTION, AND A PLAN THAT DROPS IT MUST REFUSE.
//
// MEASURED LIVE, Step 4, 2026-09-26. Every derived plan in the run carried
// `filters: null`, and the system answered anyway:
//
//	M12  "How many image text regions require manual review?"
//	     truth 46 (manual_review_required = true, 46 of 309)   answered 309
//	M7   "How many plate groups used persistent object tracking?"
//	     truth 0  (persistent_tracking is false on all 24)     answered 24
//
// Both bound to the RIGHT table. Both fields are curated correctly -- BOOLEAN,
// sensitivity NONE, allowed_filters [EQ, NEQ, IS_NULL, IS_NOT_NULL], with the
// synonyms an analyst would use. The curation is not the problem.
//
// The plan is VALID: it executes over the right contract and verifies, because
// verification proves a plan against ITSELF, never against the question. An
// INVENTED filter is a symptom this system already watches for; a MISSING one
// has no symptom at all. "How many X require review" silently widened to "how
// many X", and the answer looks exactly as authoritative.
//
// M3 is why this is worth a refusal rather than a shrug. "How many plate reads
// still need manual review?" answered 307 and scored CORRECT -- because
// manual_review_required is true on ALL 307 rows, so the dropped filter happened
// not to matter. **It is right by coincidence, not by capability**, which is the
// same thing "the 47 was never real" was about. Refusing it costs one lucky
// answer and removes two confident-wrong ones.
//
// BOOLEAN ONLY, and the type is the whole argument. Naming a boolean field in a
// question IS the restriction: "require manual review", "used persistent
// tracking". Naming a NUMBER field is usually the measure ("average ocr
// confidence") and naming a STRING field is usually the grouping ("which script
// families"), so neither implies a filter and both would refuse correct plans.
//
// MULTI-WORD ONLY, for the reason the value-literal rule already records: a
// single-word match is the field naming itself by accident.
//
// DEFAULT OFF, its own switch, measured against a control.
const booleanRestrictionEnv = "FORENSIC_BOOLEAN_RESTRICTION"

func booleanRestrictionEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(booleanRestrictionEnv)), "true")
}

// questionNamesBooleanRestriction returns the BARE names of the curated BOOLEAN
// fields a question names through a declared multi-word phrase.
//
// BARE NAMES, not entity-qualified ids, and that matters. Five entities curate
// `manual_review_required`, so "still need manual review" names all five. A rule
// keyed on the full id would then refuse a plan that DID filter on its own
// family's copy, because the other four would still look unhonoured. The
// question names a RESTRICTION; which entity owns it is the binding's business.
//
// It reads the CURATED LAYER and nothing else, so widening it is a curation
// change and there is one vocabulary rather than two that can disagree. It does
// NOT consult the switch: the census must see the risk surface with the rule off.
func questionNamesBooleanRestriction(question string) []string {
	layer, _ := defaultSemanticLayer()
	if layer == nil {
		return nil
	}
	words := _semanticLayerQuestionTokens(question)
	stems := map[string]bool{}
	for word := range words {
		stems[semanticStem(word)] = true
	}
	named := map[string]bool{}
	for _, entity := range layer.Entities {
		for _, field := range entity.Fields {
			if !strings.EqualFold(strings.TrimSpace(field.Type), "BOOLEAN") {
				continue
			}
			if !fieldCanBeFiltered(field) {
				continue
			}
			for _, phrase := range append(append([]string{}, field.Synonyms...), field.DisplayName) {
				if mediaFamilyPhraseMatches(phrase, words, stems) {
					named[bareFieldName(field.ID)] = true
					break
				}
			}
		}
	}
	out := make([]string, 0, len(named))
	for id := range named {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// fieldCanBeFiltered reports whether the layer lets a plan restrict on this
// field at all. A field the plan COULD NOT filter must never be held against it.
func fieldCanBeFiltered(field SemanticLayerFieldV1) bool {
	for _, op := range field.AllowedFilters {
		switch strings.ToUpper(strings.TrimSpace(op)) {
		case "EQ", "NEQ", "IN", "IS_NULL", "IS_NOT_NULL":
			return true
		}
	}
	return false
}

// unusedBooleanRestrictions lists the curated BOOLEAN fields a question names
// that the plan neither FILTERS on nor GROUPS by.
//
// Grouping counts as honouring the restriction: "how many of each review state"
// is a breakdown, and a breakdown that reports both sides has not dropped
// anything. Only a plan that does neither has silently widened the question.
func unusedBooleanRestrictions(question string, plan *SourceNativePlanV1) []string {
	if plan == nil {
		return nil
	}
	named := questionNamesBooleanRestriction(question)
	if len(named) == 0 {
		return nil
	}
	// Honoured by BARE name, so a plan filtering on its own family's copy of a
	// field five entities curate is never held to the other four.
	honoured := map[string]bool{}
	for _, filter := range plan.Filters {
		honoured[bareFieldName(filter.FieldID)] = true
	}
	for _, id := range plan.GroupFields {
		honoured[bareFieldName(id)] = true
	}
	unused := []string{}
	for _, name := range named {
		if !honoured[name] {
			unused = append(unused, name)
		}
	}
	return unused
}

// bareFieldName drops the entity qualifier from a curated field id.
func bareFieldName(id string) string {
	trimmed := strings.TrimSpace(id)
	if idx := strings.LastIndex(trimmed, "."); idx >= 0 {
		return trimmed[idx+1:]
	}
	return trimmed
}
