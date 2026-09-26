package main

import (
	"testing"
)

// ROUTING AND CATALOGUE SELECTION MUST AGREE.
//
// Step 4 (2026-09-26) measured the case where they did not. "What is the average
// OCR confidence of the PLATE READS?" resolved `anpr_model_observation`
// correctly, and the catalogue then issued the curated derived fields PLUS the
// schema-inferred RECORDS column `ocr_confidence`. The model picked the records
// field, the binding followed the plan's fields to forensic.records exactly as
// designed, and a question about 307 MODEL plate reads was answered 0.89 from
// the INGESTED camera sightings. Truth: 0.955573.
//
// Nothing refused it. The mixed-source refusal only fires on a plan naming BOTH
// tables; this plan named only records, so it was a perfectly valid plan over
// the wrong evidence class. **The mixture has to not be offered.**
//
// These tests hold that boundary. The live suite cannot: it needs a model, a
// deploy and twenty minutes, and it only catches the questions someone thought
// to write down.

func derivedCatalogRequest(question string) hybridQueryRequest {
	return hybridQueryRequest{
		Query:        question,
		TenantID:     "default",
		CollectionID: "nexusai-multimodal-product-acceptance",
		QueryScope:   queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)},
	}
}

// recordsShapedInferred stands in for schema discovery over forensic.records,
// using the column names that actually collide with the media vocabulary.
func recordsShapedInferred(req hybridQueryRequest) []FieldDescriptorV1 {
	names := []string{"ocr_confidence", "plate_text", "camera_id", "detection_confidence", "msisdn"}
	out := make([]FieldDescriptorV1, 0, len(names))
	for _, name := range names {
		out = append(out, FieldDescriptorV1{
			ContractVersion: fieldDescriptorContractV1,
			FieldID:         sourceNativeFieldID(req, name),
			SourceName:      name,
			SourceNames:     []string{name},
			NormalizedName:  normalizeSourceNativeName(name),
			EffectiveType:   "STRING",
		})
	}
	return out
}

// A DERIVED FAMILY IS ISSUED ITS CURATED FIELDS AND NOTHING ELSE. Every media
// entity is checked, not just the one that failed, because the next collision
// will be with a different column name.
func TestDerivedFamilyCatalogIssuesNoRecordsFields(t *testing.T) {
	t.Setenv(mediaFamilyRoutingEnv, "true")
	t.Setenv(derivedArtifactExecutionEnv, "true")
	layer, _ := defaultSemanticLayer()
	if layer == nil {
		t.Skip("curated layer unavailable")
	}
	checked := 0
	for _, entity := range layer.Entities {
		if !entity.Source.IsDerived() {
			continue
		}
		req := derivedCatalogRequest("placeholder")
		curated := entity.CatalogFields(req)
		if len(curated) == 0 {
			continue
		}
		catalog := mergeOrIsolate(t, entity, req)
		if len(catalog) != len(curated) {
			t.Errorf("%s: catalogue holds %d fields but the entity curates %d — %d came from somewhere else",
				entity.Family, len(catalog), len(curated), len(catalog)-len(curated))
		}
		for _, field := range catalog {
			if !field.Curated {
				t.Errorf("%s: issued UNCURATED field %s (%s); a derived plan could bind it to forensic.records",
					entity.Family, field.FieldID, field.SourceName)
			}
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("no derived entity was checked, so this proves nothing")
	}
	t.Logf("%d derived families issue only curated fields", checked)
}

// mergeOrIsolate runs the real catalogue selection for a question that resolves
// to this entity's family, so the test exercises the PATH rather than the
// helper. A probe must reproduce the path.
func mergeOrIsolate(t *testing.T, entity SemanticLayerEntityV1, req hybridQueryRequest) []FieldDescriptorV1 {
	t.Helper()
	catalog, _ := semanticLayerCatalogForFamily(req, entity.Family, recordsShapedInferred(req))
	return catalog
}

// THE QUESTION THAT FAILED, end to end through the real resolution path.
func TestM2CatalogResolvesToDerivedFieldsOnly(t *testing.T) {
	t.Setenv(mediaFamilyRoutingEnv, "true")
	t.Setenv(derivedArtifactExecutionEnv, "true")

	question := "What is the average OCR confidence of the plate reads?"
	if family := semanticQuestionFamily(question); family != "anpr_model_observation" {
		t.Fatalf("M2 resolved family %q, so this test no longer covers the measured case", family)
	}
	req := derivedCatalogRequest(question)
	catalog, source := semanticLayerCatalogForRequest(req, question, recordsShapedInferred(req))

	if source != "semantic_layer_catalog_derived_only:anpr_model_observation" {
		t.Errorf("catalogue source = %q; the derived isolation branch did not run", source)
	}
	for _, field := range catalog {
		if !field.Curated {
			t.Errorf("M2 is issued uncurated field %s (%s) — this is the Step 4 defect",
				field.FieldID, field.SourceName)
		}
		// Every issued field must bind to derived_artifacts, which is the
		// property that actually matters: the catalogue decides the table.
		binding, err := sourceNativeBindingForFieldIDs([]string{field.FieldID})
		if err != nil {
			t.Errorf("issued field %s does not resolve a binding: %v", field.FieldID, err)
			continue
		}
		if !binding.Derived || binding.Table != "forensic.derived_artifacts" {
			t.Errorf("issued field %s binds to %s (derived=%v) — a plan using it reads the wrong table",
				field.FieldID, binding.Table, binding.Derived)
		}
	}
	t.Logf("M2 issued %d fields, every one curated and bound to forensic.derived_artifacts", len(catalog))
}

// THE NEGATIVE CONTROL. Without the isolation the records column IS issued and
// DOES bind to forensic.records, so the assertions above are not vacuous. This
// reproduces the defect from the same inputs rather than asserting a shape that
// might be true for unrelated reasons.
func TestDerivedCatalogIsolationIsWhatStopsTheLeak(t *testing.T) {
	t.Setenv(mediaFamilyRoutingEnv, "true")
	t.Setenv(derivedArtifactExecutionEnv, "true")
	layer, _ := defaultSemanticLayer()
	if layer == nil {
		t.Skip("curated layer unavailable")
	}
	entity, ok := layer.EntityByFamily("anpr_model_observation")
	if !ok {
		t.Skip("anpr_model_observation missing")
	}
	req := derivedCatalogRequest("What is the average OCR confidence of the plate reads?")
	inferred := recordsShapedInferred(req)

	// The pre-fix behaviour, reconstructed: merge, exactly as the structured
	// path still does.
	leaky := mergeSemanticLayerCatalog(entity.CatalogFields(req), inferred)
	uncurated := 0
	var records string
	for _, field := range leaky {
		if !field.Curated {
			uncurated++
			if records == "" {
				records = field.FieldID
			}
		}
	}
	if uncurated == 0 {
		t.Fatal("the merge leaked nothing, so the isolation guards nothing and these tests are vacuous")
	}
	binding, err := sourceNativeBindingForFieldIDs([]string{records})
	if err != nil {
		t.Fatalf("leaked field did not resolve: %v", err)
	}
	if binding.Derived {
		t.Fatal("the leaked field bound to derived_artifacts, so it was never the defect")
	}
	t.Logf("without isolation: %d records fields issued to a DERIVED family; %s binds to %s",
		uncurated, records, binding.Table)
}

// STRUCTURED FAMILIES ARE UNTOUCHED. Withholding uncurated fields from a
// structured family was measured and REJECTED -- it cost TWR-02 a false
// negative, "there are no tower records in this case". The isolation must not
// creep into that path, and this asserts it rather than trusting the branch.
func TestStructuredFamilyCatalogStillMergesInferredFields(t *testing.T) {
	t.Setenv(mediaFamilyRoutingEnv, "true")
	layer, _ := defaultSemanticLayer()
	if layer == nil {
		t.Skip("curated layer unavailable")
	}
	for _, family := range []string{"communications_cdr", "anpr_vehicles", "tower_location"} {
		entity, ok := layer.EntityByFamily(family)
		if !ok {
			t.Errorf("%s is not curated", family)
			continue
		}
		req := derivedCatalogRequest("placeholder")
		// A column name the layer certainly does not describe.
		inferred := []FieldDescriptorV1{{
			ContractVersion: fieldDescriptorContractV1,
			FieldID:         sourceNativeFieldID(req, "zz_uncurated_probe_column"),
			SourceName:      "zz_uncurated_probe_column",
			SourceNames:     []string{"zz_uncurated_probe_column"},
			NormalizedName:  normalizeSourceNativeName("zz_uncurated_probe_column"),
			EffectiveType:   "STRING",
		}}
		catalog, source := semanticLayerCatalogForFamily(req, family, inferred)
		if source != "semantic_layer_catalog:"+family {
			t.Errorf("%s took catalogue path %q; structured families must keep the merge", family, source)
		}
		found := false
		for _, field := range catalog {
			if field.SourceName == "zz_uncurated_probe_column" {
				found = true
			}
		}
		if !found {
			t.Errorf("%s dropped its uncurated column — this is the TWR-02 false negative returning", family)
		}
		if entity.Source.IsDerived() {
			t.Errorf("%s is derived; this test is pointed at the wrong families", family)
		}
	}
}
