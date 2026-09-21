package main

import (
	"strings"
	"testing"
)

func TestTypedQueryPlanRejectsUnconsumedAlgebra(t *testing.T) {
	tests := []struct {
		name string
		plan map[string]any
		want string
	}{
		{
			name: "invented grouping",
			plan: map[string]any{"contract_version": "forensics.query-plan/v1", "intent": "cdr.frequent_contacts", "entities": []any{map[string]any{"type": "msisdn", "value": "923001234567"}}, "group_by": []any{"cell site"}},
			want: "group_by does not match",
		},
		{
			name: "invented measure",
			plan: map[string]any{"contract_version": "forensics.query-plan/v1", "intent": "cdr.frequent_contacts", "entities": []any{map[string]any{"type": "msisdn", "value": "923001234567"}}, "measures": []any{"average sentiment"}},
			want: "measures do not match",
		},
		{
			name: "unconsumed analytical sort",
			plan: map[string]any{"contract_version": "forensics.query-plan/v1", "intent": "cdr.frequent_contacts", "entities": []any{map[string]any{"type": "msisdn", "value": "923001234567"}}, "sort": []any{map[string]any{"field": "count", "direction": "desc"}}},
			want: "sort is not consumed",
		},
		{
			name: "unconsumed analytical filter",
			plan: map[string]any{"contract_version": "forensics.query-plan/v1", "intent": "cdr.service_usage", "entities": []any{map[string]any{"type": "msisdn", "value": "923001234567"}}, "filters": []any{map[string]any{"field": "direction", "op": "eq", "value": "outgoing"}}},
			want: "is not consumed",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, _, err := normalizeV1QueryPlan("case-a", "request-a", test.plan)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want substring %q", err, test.want)
			}
		})
	}
}

func TestTypedCanonicalPlanAcceptsBoundedFilterSortAndTopK(t *testing.T) {
	legacy, early, err := normalizeV1QueryPlan("case-a", "request-a", map[string]any{
		"contract_version": "forensics.query-plan/v1",
		"tenant_id":        "tenant-a",
		"intent":           "forensics.canonical_records",
		"families":         []any{"case_cross_family"},
		"filters": []any{
			map[string]any{"field": "status", "op": "eq", "value": "active"},
			map[string]any{"field": "amount", "op": "gte", "value": 100},
			map[string]any{"field": "imsi", "op": "exists"},
		},
		"sort":  []any{map[string]any{"field": "source_file", "direction": "asc"}},
		"limit": 15,
	})
	if err != nil || early != nil {
		t.Fatalf("canonical typed plan failed: early=%#v err=%v", early, err)
	}
	if legacy["template"] != "canonical_records" || legacy["sort_by"] != "source_file" || legacy["sort_direction"] != "asc" || legacy["limit"] != 15 {
		t.Fatalf("unexpected lowering: %#v", legacy)
	}
	if filters := mapsFromAny(legacy["raw_payload_filters"]); len(filters) != 3 {
		t.Fatalf("lowered filters = %#v, want three", filters)
	}
}

func TestTypedCanonicalPlanRejectsUnsafeOrAmbiguousControls(t *testing.T) {
	tests := []struct {
		name string
		plan map[string]any
		want string
	}{
		{name: "unknown filter op", plan: map[string]any{"filters": []any{map[string]any{"field": "status", "op": "regex", "value": ".*"}}}, want: "not supported"},
		{name: "unknown sort field", plan: map[string]any{"sort": []any{map[string]any{"field": "raw_payload ->> 'secret'", "direction": "asc"}}}, want: "not allowlisted"},
		{name: "unknown sort direction", plan: map[string]any{"sort": []any{map[string]any{"field": "timestamp", "direction": "sideways"}}}, want: "must be asc or desc"},
		{name: "multiple sorts", plan: map[string]any{"sort": []any{map[string]any{"field": "timestamp", "direction": "desc"}, map[string]any{"field": "source_file", "direction": "asc"}}}, want: "at most one"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.plan["contract_version"] = "forensics.query-plan/v1"
			test.plan["intent"] = "forensics.canonical_records"
			_, _, err := normalizeV1QueryPlan("case-a", "request-a", test.plan)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want substring %q", err, test.want)
			}
		})
	}
}
