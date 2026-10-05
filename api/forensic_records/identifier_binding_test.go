package main

import (
	"context"
	"testing"
)

// Only curated, non-sensitive string fields that accept EQ may ever receive an
// identifier. Every exclusion below is a field an identifier must never bind to.
func TestIdentifierCandidateFields(t *testing.T) {
	catalog := []FieldDescriptorV1{
		{FieldID: "tower.site_code", Curated: true, EffectiveType: "STRING", AllowedFilters: []string{"EQ", "IN"}},
		{FieldID: "tower.latitude", Curated: true, EffectiveType: "NUMBER", AllowedFilters: []string{"EQ", "GT"}},
		{FieldID: "fld_inferred", Curated: false, EffectiveType: "STRING", AllowedFilters: []string{"EQ"}},
		{FieldID: "tower.notes", Curated: true, EffectiveType: "STRING", AllowedFilters: []string{"CONTAINS"}},
		{FieldID: "subscriber.cnic", Curated: true, EffectiveType: "STRING", AllowedFilters: []string{"EQ"}, Sensitivity: "PII"},
		{FieldID: "subscriber.secret", Curated: true, EffectiveType: "STRING", AllowedFilters: []string{"EQ"}, Sensitivity: "restricted"},
	}
	got := identifierCandidateFields(catalog)
	if len(got) != 1 || got[0].FieldID != "tower.site_code" {
		ids := []string{}
		for _, field := range got {
			ids = append(ids, field.FieldID)
		}
		t.Fatalf("expected only tower.site_code, got %v", ids) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}

// A WHO question about an entity that declares PII keeps refusing. SUB-03 is
// the measured case: binding would have answered "who" with the identity
// fields removed.
func TestAsksForWithheldIdentity(t *testing.T) {
	withPII := SemanticLayerEntityV1{Fields: []SemanticLayerFieldV1{
		{ID: "subscriber.subscriber_id", Sensitivity: "NONE"},
		{ID: "subscriber.cnic", Sensitivity: "PII"},
	}}
	withoutPII := SemanticLayerEntityV1{Fields: []SemanticLayerFieldV1{
		{ID: "tower.site_code", Sensitivity: "NONE"},
	}}
	cases := []struct {
		question string
		entity   SemanticLayerEntityV1
		want     bool
	}{
		{"Who is the registered subscriber for PK-SUB-SYN-ALPHA?", withPII, true},
		{"whose number is 923001110001?", withPII, true},
		// Not a WHO question: the identity is not what is being asked for.
		{"What is the status of subscriber PK-SUB-SYN-ALPHA?", withPII, false},
		// A WHO question about an entity with nothing withheld is unaffected.
		{"Who operates tower PK-LHR-SYN-001?", withoutPII, false},
		// "who" as a later word is not an identity question.
		{"Where is the tower who serves Gulberg?", withPII, false},
	}
	for _, tc := range cases {
		if got := asksForWithheldIdentity(tc.question, tc.entity); got != tc.want {
			t.Errorf("asksForWithheldIdentity(%q) = %v, want %v", tc.question, got, tc.want) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
}

// The switch is inert when unset. Every arm downstream is void otherwise.
func TestIdentifierBindingSwitchesAreInert(t *testing.T) {
	t.Setenv(identifierBindingEnv, "")
	if identifierBindingEnabled() {
		t.Fatal("the switch must default off") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	frame := SemanticFrameV1{FamilyHint: "tower_location", Identifiers: []HybridIdentifierV1{{Raw: "PK-LHR-SYN-001", Canonical: "PK-LHR-SYN-001"}}}
	catalog := []FieldDescriptorV1{{FieldID: "tower.site_code", Curated: true, EffectiveType: "STRING", AllowedFilters: []string{"EQ"}}}
	if got := bindIdentifiersByPresence(context.Background(), nil, hybridQueryRequest{}, frame, catalog); len(got) != 0 {
		t.Fatalf("binding must do nothing with its switch off, got %+v", got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}
