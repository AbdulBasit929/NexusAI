package main

import (
	"fmt"
	"os"
	"strings"
)

// An INVENTED filter is one whose value the analyst never supplied. It is a
// different thing from a missing literal, and conflating the two cost a day:
// four questions failed with one error string and turned out to be two
// unrelated defects.
//
// Read from the cached completions on 2026-09-24, the generator's habit is
// specific and repeatable -- it fills the schema's optional filter slots with
// material scraped from the question's own wording, bolted onto an otherwise
// correct plan:
//
//	CDR-11   cdr.originating_number EQ "923001110001"   <- correct
//	         cdr.direction          EQ "outbound"       <- INVENTED
//	TWR-02   tower.site_code        EQ "PK-LHR-SYN-001" <- correct
//	         tower.site_location    CONTAINS "where"    <- INVENTED from the
//	         tower.district         CONTAINS "area"        interrogative
//
// The allowlist is RIGHT to refuse those values. The question this file answers
// is what to do next: refusing the whole plan throws away a correct filter
// alongside the invention, and the analyst gets a clarification for a question
// the system had actually understood.
//
// THE DISTINCTION THAT MAKES REMOVAL SAFE. D1 -- the defect this project was
// founded on -- was silently dropping a constraint THE ANALYST SUPPLIED. This
// drops a constraint the analyst never supplied, which is not a constraint at
// all but an invention. Removing an invention narrows nothing and loses no
// obligation: by construction only filters whose values fail the allowlist are
// removed, so every analyst-supplied literal in the plan survives untouched.
//
// It is still a WIDENING of the result, so it ships behind its own switch,
// default off, and is measured against the 62 before it is trusted.
const semanticDropInventedFiltersEnv = "FORENSIC_DROP_INVENTED_FILTERS"

func semanticDropInventedFiltersEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(semanticDropInventedFiltersEnv)), "true")
}

// planWithoutInventedFilters returns a COPY of the plan carrying only the
// filters whose values the analyst supplied.
//
// Three guards, and a failure of any of them refuses rather than answers:
//
//  1. the pruned plan is re-validated in full -- pruning changes the plan, and
//     a changed plan is re-checked, never assumed still valid;
//  2. if the analyst named a TARGET, a remaining filter must still carry it.
//     Otherwise the plan measures every record of its kind and the answer
//     attributes the total to one subject, which is precisely DOC-04;
//  3. the plan must still compute something.
func planWithoutInventedFilters(req hybridQueryRequest, plan *SourceNativePlanV1, kept []SourceNativeFilterV1, catalog []FieldDescriptorV1) (*SourceNativePlanV1, error) {
	if plan == nil {
		return nil, fmt.Errorf("source-native plan is absent")
	}
	pruned := *plan
	pruned.Filters = kept

	if len(pruned.Project) == 0 && len(pruned.Measures) == 0 {
		return nil, fmt.Errorf("source-native plan has nothing left to compute")
	}
	if err := validateSourceNativePlan(&pruned, catalog); err != nil {
		return nil, err
	}
	if target := strings.TrimSpace(req.Target); target != "" && !filtersCarryTarget(kept, target) {
		return nil, fmt.Errorf("source-native plan no longer constrains %s", target)
	}
	return &pruned, nil
}

// filtersCarryTarget reports that the analyst's named subject survives as a
// filter. Matched case-insensitively because the generator re-cases values.
func filtersCarryTarget(filters []SourceNativeFilterV1, target string) bool {
	for _, filter := range filters {
		for _, value := range append([]string{filter.Value}, filter.Values...) {
			if strings.EqualFold(strings.TrimSpace(value), target) {
				return true
			}
		}
	}
	return false
}
