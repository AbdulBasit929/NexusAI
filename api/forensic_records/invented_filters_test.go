package main

import "testing"

// Reconstructed from the cached completions read on 2026-09-24.

func inventedCatalog() []FieldDescriptorV1 {
	return []FieldDescriptorV1{
		{FieldID: "cdr.originating_number", EffectiveType: "STRING", Projectable: true,
			AllowedFilters: []string{"EQ"}, AllowedAggregates: []string{"COUNT"}},
		{FieldID: "cdr.direction", EffectiveType: "STRING", Projectable: true,
			AllowedFilters: []string{"EQ"}, AllowedAggregates: []string{"COUNT"}},
	}
}

func cdr11Plan() *SourceNativePlanV1 {
	return &SourceNativePlanV1{
		ContractVersion: sourceNativePlanContractV1,
		Filters: []SourceNativeFilterV1{
			{FieldID: "cdr.originating_number", Op: "EQ", Value: "923001110001", Values: []string{"923001110001"}},
			{FieldID: "cdr.direction", Op: "EQ", Value: "outbound", Values: []string{"outbound"}},
		},
		Measures: []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT"}},
	}
}

func TestDroppingInventedFiltersIsOffByDefault(t *testing.T) {
	if semanticDropInventedFiltersEnabled() {
		t.Fatal("removing a filter WIDENS the result; it must be opt-in and measured")
	}
	t.Setenv(semanticDropInventedFiltersEnv, "true")
	if !semanticDropInventedFiltersEnabled() {
		t.Fatal("FORENSIC_DROP_INVENTED_FILTERS=true must enable it")
	}
}

// The analyst's own filter survives; only the invention is removed.
func TestPruningKeepsTheAnalystSuppliedFilter(t *testing.T) {
	req := supplyRequest("How many calls did 923001110001 make?")
	req.Target = "923001110001"
	plan := cdr11Plan()
	kept := []SourceNativeFilterV1{plan.Filters[0]}

	pruned, err := planWithoutInventedFilters(req, plan, kept, inventedCatalog())
	if err != nil {
		t.Fatalf("pruning a plan that still carries the target must succeed: %v", err)
	}
	if len(pruned.Filters) != 1 || pruned.Filters[0].Value != "923001110001" {
		t.Fatalf("the analyst's filter must survive: %+v", pruned.Filters)
	}
	// The ORIGINAL must not be mutated: it is memoized in the plan cache and
	// shared across requests.
	if len(plan.Filters) != 2 {
		t.Fatalf("pruning must not mutate the cached plan: %+v", plan.Filters)
	}
}

// D1's lesson, inverted and re-asserted. Removing an invention must never leave
// a plan that measures everything while the answer names one subject.
func TestPruningRefusesWhenTheTargetNoLongerSurvives(t *testing.T) {
	req := supplyRequest("How many calls did 923001110001 make?")
	req.Target = "923001110001"
	plan := cdr11Plan()

	// Pathological: the analyst's own filter is the one proposed for removal.
	_, err := planWithoutInventedFilters(req, plan, []SourceNativeFilterV1{plan.Filters[1]}, inventedCatalog())
	if err == nil {
		t.Fatal("a plan that no longer constrains the named target must be refused, not answered")
	}
}

func TestPruningRefusesAPlanWithNothingLeftToCompute(t *testing.T) {
	req := supplyRequest("Show the records")
	plan := &SourceNativePlanV1{
		ContractVersion: sourceNativePlanContractV1,
		Filters:         []SourceNativeFilterV1{{FieldID: "cdr.direction", Op: "EQ", Value: "outbound"}},
	}
	if _, err := planWithoutInventedFilters(req, plan, nil, inventedCatalog()); err == nil {
		t.Fatal("a plan with no projection and no measure computes nothing")
	}
}

// Pruning changes the plan, so the plan is re-validated rather than assumed
// still valid.
func TestPrunedPlanIsRevalidatedAgainstTheIssuedCatalog(t *testing.T) {
	req := supplyRequest("How many calls are there?")
	plan := &SourceNativePlanV1{
		ContractVersion: sourceNativePlanContractV1,
		Filters:         []SourceNativeFilterV1{{FieldID: "cdr.direction", Op: "EQ", Value: "outbound"}},
		Measures:        []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT"}},
		GroupFields:     []string{"cdr.not_issued"},
	}
	if _, err := planWithoutInventedFilters(req, plan, nil, inventedCatalog()); err == nil {
		t.Fatal("a pruned plan referencing an unissued field must still be rejected")
	}
}

func TestTargetMatchIsCaseInsensitive(t *testing.T) {
	filters := []SourceNativeFilterV1{{FieldID: "tower.site_code", Op: "EQ", Value: "pk-lhr-syn-001"}}
	if !filtersCarryTarget(filters, "PK-LHR-SYN-001") {
		t.Fatal("the generator re-cases values; the target check must not depend on case")
	}
	if filtersCarryTarget(filters, "PK-LHR-SYN-002") {
		t.Fatal("a different identifier is not the target")
	}
}
