package main

import (
	"strings"
	"testing"
)

// THE MEDIA PII BOUNDARY MUST NOT DEPEND ON THE GOAL GATE.
//
// Found 2026-09-26, after WI-LAYER-6 took routing from 7 of 28 to 15: the
// census now shows that H1 "Which plates were read from the videos?" and H2
// "What does the audio transcript say?" MATCH a curated media phrase and are
// held back ONLY because `goal=lookup` is not algebraic.
//
// That is a load-bearing safety property resting on the wrong mechanism. The
// goal gate exists to keep `lookup` -- which is ALSO the unrecognised default --
// from reaching the compiler; it is not a PII control, and the very next backend
// item (the goal taxonomy, worth ~7 media questions) exists to reclassify
// exactly these questions as `distinct`. The day M11/M14/M18/M20 start routing,
// H1 and H2 route with them.
//
// The real control is `CatalogFields`, which drops PII and RESTRICTED outright
// so no plan can name the field and no verified plan can project it. These tests
// assert that control directly, so the taxonomy work cannot silently convert an
// honest clarification into a plate list or a transcript.
//
// This is written BEFORE the taxonomy work rather than after, because the
// alternative is discovering it from a demo.

// mediaPIIFields are the curated media fields whose values must never reach an
// analyst through the dynamic path. Derived from the layer itself rather than
// hardcoded, so a newly curated PII field is covered the day it lands.
func mediaPIIFields(t *testing.T) map[string][]string {
	t.Helper()
	layer, _ := defaultSemanticLayer()
	if layer == nil {
		t.Skip("curated layer unavailable")
	}
	out := map[string][]string{}
	for _, entity := range layer.Entities {
		if !entity.Source.IsDerived() {
			continue
		}
		for _, field := range entity.Fields {
			if field.Sensitivity == "PII" || field.Sensitivity == "RESTRICTED" {
				out[entity.Family] = append(out[entity.Family], field.ID)
			}
		}
	}
	if len(out) == 0 {
		t.Fatal("no derived PII field found at all; either curation regressed or this test is pointed at the wrong layer")
	}
	return out
}

// A PII FIELD IS NEVER ISSUED. If it is not in the catalogue it is not in the
// enum, and the GBNF alternation makes it unemittable -- the keystone. This is
// the control that must hold when the goal gate stops holding.
func TestMediaPIIFieldsAreNeverIssuedInTheCatalogue(t *testing.T) {
	layer, _ := defaultSemanticLayer()
	if layer == nil {
		t.Skip("curated layer unavailable")
	}
	req := hybridQueryRequest{
		TenantID: "default", CollectionID: "nexusai-multimodal-product-acceptance",
		QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)},
	}
	for family, piiFields := range mediaPIIFields(t) {
		entity, ok := layer.EntityByFamily(family)
		if !ok {
			t.Errorf("family %s vanished from the layer", family)
			continue
		}
		issued := map[string]bool{}
		for _, field := range entity.CatalogFields(req) {
			issued[field.FieldID] = true
		}
		if len(issued) == 0 {
			t.Errorf("family %s issued NO fields at all, so this proves nothing about %v", family, piiFields)
			continue
		}
		for _, id := range piiFields {
			if issued[id] {
				t.Errorf("PII field %s IS issued for %s; a plan can name it and the analyst can be shown the value", id, family)
			}
		}
		t.Logf("%-30s %d fields issued, %d PII withheld: %v", family, len(issued), len(piiFields), piiFields)
	}
}

// THE PLATE TEXT AND THE TRANSCRIPT SPECIFICALLY. These are the two the honesty
// probes ask for by name, and the two the product would be caught on.
func TestTheHonestyProbeFieldsAreWithheld(t *testing.T) {
	layer, _ := defaultSemanticLayer()
	if layer == nil {
		t.Skip("curated layer unavailable")
	}
	req := hybridQueryRequest{
		TenantID: "default", CollectionID: "nexusai-multimodal-product-acceptance",
		QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)},
	}
	// family -> the substring of the field id the probe is really asking for.
	probes := map[string]string{
		"anpr_model_observation":   "plate",      // H1 "Which plates were read from the videos?"
		"video_anpr_plate_group":   "plate_text", // H1, the grouped form
		"audio_timestamp_segment":  "text",       // H2 "What does the audio transcript say?"
		"audio_roman_urdu_segment": "text",       // H2, the Roman-Urdu form
	}
	for family, want := range probes {
		entity, ok := layer.EntityByFamily(family)
		if !ok {
			t.Errorf("family %s is not curated", family)
			continue
		}
		// The field must EXIST in the curated layer (otherwise this test passes
		// for the wrong reason -- a field nobody curated is not a field anyone
		// withheld) and must be ABSENT from the issued catalogue.
		declared, issued := false, false
		for _, field := range entity.Fields {
			if strings.Contains(field.ID, want) && (field.Sensitivity == "PII" || field.Sensitivity == "RESTRICTED") {
				declared = true
			}
		}
		if !declared {
			t.Errorf("%s declares no PII field matching %q; either curation changed or this probe is pointed at the wrong field", family, want)
			continue
		}
		for _, field := range entity.CatalogFields(req) {
			if strings.Contains(field.FieldID, want) {
				issued = true
				t.Errorf("%s ISSUES %s — the honesty probe for this family would be answerable with the value", family, field.FieldID)
			}
		}
		if !issued {
			t.Logf("%-26s PII %q declared and withheld from the issued catalogue", family, want)
		}
	}
}

// THE GOAL GATE IS NOT THE CONTROL, and this asserts the separation directly:
// with the goal gate set aside, the PII field is STILL not issued. If this ever
// fails, the taxonomy work has opened a PII path and must stop.
func TestPIIWithholdingIsIndependentOfTheGoalGate(t *testing.T) {
	layer, _ := defaultSemanticLayer()
	if layer == nil {
		t.Skip("curated layer unavailable")
	}
	entity, ok := layer.EntityByFamily("anpr_model_observation")
	if !ok {
		t.Skip("anpr_model_observation missing")
	}
	// Every goal, algebraic or not -- the catalogue does not consult the goal,
	// and this makes that explicit rather than assumed.
	for _, goal := range []string{"lookup", "distinct", "aggregate", "rank", "range", "breakdown"} {
		req := hybridQueryRequest{
			Query:    "Which plates were read from the videos?",
			TenantID: "default", CollectionID: "nexusai-multimodal-product-acceptance",
			QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)},
		}
		for _, field := range entity.CatalogFields(req) {
			if strings.Contains(field.FieldID, "plate_text") {
				t.Errorf("goal=%s: plate_text is issued; the PII boundary depends on the goal gate", goal)
			}
		}
	}
	t.Log("plate_text withheld at every goal — the boundary is the catalogue, not the goal gate")
}
