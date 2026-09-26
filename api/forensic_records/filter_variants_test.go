package main

import (
	"testing"
)

func filterVariantCatalog() []FieldDescriptorV1 {
	return []FieldDescriptorV1{
		{FieldID: "cdr.msisdn", EffectiveType: "STRING",
			AllowedFilters:    []string{"EQ", "NEQ", "IN", "CONTAINS", "IS_NULL", "IS_NOT_NULL"},
			AllowedAggregates: []string{"COUNT", "COUNT_DISTINCT"}},
		{FieldID: "cdr.event_time", EffectiveType: fieldTypeTimestamp,
			AllowedFilters:    []string{"EQ", "BETWEEN", "GT", "LT"},
			AllowedAggregates: []string{"COUNT", "MIN", "MAX"}},
	}
}

// CDR-11's defect, asserted on the grammar the SERVER builds.
//
// The generator wrote `IS_NULL` carrying a value and the plan was refused with
// "null filters cannot carry values". A null test and a valued comparison are
// now separate variants, so the combination is unrepresentable.
func TestGrammarCannotWriteANullTestCarryingAValue(t *testing.T) {
	rules := grammarRules(servingPathGrammar(t, filterVariantCatalog()))

	nullOps := grammarLiterals(rules["root-0-filters-item-0-op"])
	if len(nullOps) != 2 || !containsString(nullOps, "IS_NULL") || !containsString(nullOps, "IS_NOT_NULL") {
		t.Fatalf("the null-test variant must carry exactly the two null operators, got %v", nullOps)
	}
	nullValues := grammarLiterals(rules["root-0-filters-item-0-value"])
	if len(nullValues) != 1 || nullValues[0] != "" {
		t.Errorf("a null test may only carry an empty value, got %v", nullValues)
	}
	// And it must not be able to carry a values array at all.
	if body, exists := rules["root-0-filters-item-0-values"]; exists {
		t.Errorf("the null-test variant must have no values key; got rule %q", body)
	}

	// The valued variant carries every other operator and a real value.
	valuedOps := grammarLiterals(rules["root-0-filters-item-1-op"])
	for _, op := range []string{"EQ", "NEQ", "IN", "CONTAINS", "BETWEEN"} {
		if !containsString(valuedOps, op) {
			t.Errorf("%s must remain expressible, got %v", op, valuedOps)
		}
	}
	for _, op := range []string{"IS_NULL", "IS_NOT_NULL"} {
		if containsString(valuedOps, op) {
			t.Errorf("%s belongs to the null variant only, got %v", op, valuedOps)
		}
	}
}

// The field slot stays constrained in BOTH variants -- splitting the object
// must not lose the property the whole architecture rests on.
func TestFilterVariantsKeepTheFieldEnumeration(t *testing.T) {
	rules := grammarRules(servingPathGrammar(t, filterVariantCatalog()))
	for _, rule := range []string{"root-0-filters-item-0-field-id", "root-0-filters-item-1-field-id"} {
		body, ok := rules[rule]
		if !ok {
			t.Errorf("%s is absent; the field slot is unconstrained", rule)
			continue
		}
		if isGenericString(body) {
			t.Errorf("%s compiled to the generic string rule", rule)
			continue
		}
		if !containsString(grammarLiterals(body), "cdr.msisdn") {
			t.Errorf("%s does not enumerate the issued fields: %s", rule, body)
		}
	}
}

// The validator keeps its rule. A schema and a verifier agreeing is the point;
// the verifier standing down is not.
func TestNullFilterValidationSurvivesTheSplit(t *testing.T) {
	catalog := filterVariantCatalog()
	carrying := &SourceNativePlanV1{
		ContractVersion: sourceNativePlanContractV1,
		Filters:         []SourceNativeFilterV1{{FieldID: "cdr.msisdn", Op: "IS_NULL", Value: "923001110001"}},
		Measures:        []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT"}},
	}
	if err := validateSourceNativePlan(carrying, catalog); err == nil {
		t.Fatal("a null test carrying a value must still be refused by the validator")
	}

	// And the shapes that must stay legal.
	for _, filter := range []SourceNativeFilterV1{
		{FieldID: "cdr.msisdn", Op: "IS_NULL"},
		{FieldID: "cdr.msisdn", Op: "EQ", Value: "923001110001"},
		{FieldID: "cdr.msisdn", Op: "IN", Values: []string{"a", "b"}},
	} {
		plan := &SourceNativePlanV1{
			ContractVersion: sourceNativePlanContractV1,
			Filters:         []SourceNativeFilterV1{filter},
			Measures:        []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT"}},
		}
		if err := validateSourceNativePlan(plan, catalog); err != nil {
			t.Errorf("%s must stay legal: %v", filter.Op, err)
		}
	}
}

// No empty enum anywhere in the issued filter schema; that is the failure mode
// that returns HTTP 500 before any inference.
func TestFilterVariantsIssueNoEmptyEnum(t *testing.T) {
	schema := semanticSourceNativeSchema(filterVariantCatalog())
	properties, _ := schema["properties"].(map[string]any)
	filters, _ := properties["filters"].(map[string]any)
	if bad := emptyEnums(filters, "filters"); len(bad) > 0 {
		t.Fatalf("empty enums in the filter schema are fatal: %v", bad)
	}
}
