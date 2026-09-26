package main

import "testing"

// OMISSION MEANS RECORDS. The seven structured entities carry no source block
// and must keep binding to forensic.records exactly as before -- changing the
// issued field set changes plans for questions that already work, proven twice
// and it cost a false negative both times.
func TestSemanticSourceDefaultsToRecords(t *testing.T) {
	var absent SemanticLayerSourceV1
	if absent.resolvedTable() != semanticSourceTableRecords {
		t.Fatalf("absent source resolved to %q", absent.resolvedTable())
	}
	if absent.resolvedPayload() != "raw_payload" {
		t.Fatalf("absent source payload resolved to %q", absent.resolvedPayload())
	}
	if absent.IsDerived() {
		t.Fatal("an absent source must not read as derived")
	}
	if err := validateSemanticSource("entity x", absent); err != nil {
		t.Fatalf("an absent source must validate: %v", err)
	}
}

// THE REGRESSION THIS FILE CAUSED ON ITS FIRST RUN. `anpr.plate_number` binds
// to the CSV header "Registration No.", which ENDS IN A PERIOD. Treating the
// dot as a path separator turned a working field into an empty path and broke
// the shipped layer. A records source name is one literal key, never split.
func TestSemanticSourceRecordsNameWithADotIsLiteral(t *testing.T) {
	path := semanticSourceFieldPath("Registration No.", false)
	if len(path) != 1 || path[0] != "Registration No." {
		t.Fatalf("records source name was split: %#v", path)
	}
	if err := validateSemanticSourceFieldName("entity anpr", "Registration No.", false); err != nil {
		t.Fatalf("a literal header ending in a period must validate: %v", err)
	}
}

// A derived entity keeps its analytical fields under one object, so it may name
// a nested path.
func TestSemanticSourceDerivedNameIsAPath(t *testing.T) {
	path := semanticSourceFieldPath("observation.normalized_plate_text", true)
	if len(path) != 2 || path[0] != "observation" || path[1] != "normalized_plate_text" {
		t.Fatalf("derived path not resolved: %#v", path)
	}
	if err := validateSemanticSourceFieldName("entity anpr_obs", "observation.start_seconds", true); err != nil {
		t.Fatalf("a nested derived path must validate: %v", err)
	}
	if semanticSourceFieldPath("observation..text", true) != nil {
		t.Fatal("an empty path segment must not resolve")
	}
}

// AN UNVERSIONED OR MISSING artifact_type IS THE WHOLE SAFETY PROPERTY. Without
// one the entity matches every contract in the table -- an OCR entity counting
// face detections -- and the number looks entirely authoritative.
func TestSemanticSourceDerivedRequiresAVersionedArtifactType(t *testing.T) {
	for _, tc := range []struct {
		name   string
		source SemanticLayerSourceV1
	}{
		{"no artifact type", SemanticLayerSourceV1{Table: semanticSourceTableDerived}},
		{"unversioned", SemanticLayerSourceV1{Table: semanticSourceTableDerived, ArtifactType: "forensics.anpr-observation"}},
		{"not a forensics contract", SemanticLayerSourceV1{Table: semanticSourceTableDerived, ArtifactType: "anpr/v1"}},
		{"unknown table", SemanticLayerSourceV1{Table: "evidence_items", ArtifactType: "forensics.x/v1"}},
		{"payload not readable", SemanticLayerSourceV1{Table: semanticSourceTableDerived, ArtifactType: "forensics.x/v1", Payload: "citation_locator"}},
		{"artifact type on records", SemanticLayerSourceV1{Table: semanticSourceTableRecords, ArtifactType: "forensics.x/v1"}},
	} {
		if err := validateSemanticSource("entity x", tc.source); err == nil {
			t.Errorf("%s: accepted", tc.name)
		}
	}

	good := SemanticLayerSourceV1{
		Table: semanticSourceTableDerived, ArtifactType: "forensics.anpr-observation/v1", Payload: "metadata",
	}
	if err := validateSemanticSource("entity x", good); err != nil {
		t.Fatalf("a well-formed derived source was rejected: %v", err)
	}
	if !good.IsDerived() {
		t.Fatal("a derived source must read as derived")
	}
}

// A dotted name against forensic.records resolves to a key that does not exist,
// so the field reads NULL for every row: an entity that validates, loads, and
// answers every question about it with "no value".
func TestSemanticSourceRejectsNestedPathOnRecords(t *testing.T) {
	if err := validateSemanticSourceFieldName("entity cdr", "observation.msisdn", true); err != nil {
		t.Fatalf("derived should accept it: %v", err)
	}
	if err := validateSemanticSourceFieldName("entity cdr", "a.b.c.d.e", true); err == nil {
		t.Fatal("a path deeper than the contracts nest was accepted")
	}
}
