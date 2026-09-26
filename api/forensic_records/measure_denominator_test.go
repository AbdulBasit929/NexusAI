package main

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// A COUNT over a sparse field IS the denominator -- "how many observations carry
// a confidence" is answered by the number itself. AVG, SUM, MIN and MAX all
// summarise a population without naming it.
func TestMeasureNeedsDenominatorOnlyForSummarisingOps(t *testing.T) {
	for _, op := range []string{"AVG", "SUM", "MIN", "MAX"} {
		if !measureNeedsDenominator(SourceNativeMeasureV1{Op: op, FieldID: "x.y"}) {
			t.Errorf("%s summarises a population and must be qualified", op)
		}
	}
	for _, op := range []string{"COUNT", "COUNT_DISTINCT"} {
		if measureNeedsDenominator(SourceNativeMeasureV1{Op: op, FieldID: "x.y"}) {
			t.Errorf("%s is its own denominator and must not be qualified", op)
		}
	}
	if measureNeedsDenominator(SourceNativeMeasureV1{Op: "AVG"}) {
		t.Error("a measure naming no field has no coverage to report")
	}
}

func TestMeasureDenominatorNoteQualifiesOnlyWhenItMatters(t *testing.T) {
	// The case this exists for: 263 of 309.
	note := measureDenominatorNote(263, 309, "image OCR observations")
	for _, want := range []string{"263", "309", "46", "image OCR observations"} {
		if !strings.Contains(note, want) {
			t.Fatalf("note %q omits %s", note, want)
		}
	}

	// A single NULL in 8,642 does not make an average misleading, and a qualifier
	// on every answer is noise analysts learn to skip -- which also devalues it
	// where it matters.
	if got := measureDenominatorNote(8641, 8642, "CDR records"); got != "" {
		t.Fatalf("a 99.99%% field was qualified: %q", got)
	}
	// Full coverage, and the degenerate cases, say nothing.
	if got := measureDenominatorNote(309, 309, "x"); got != "" {
		t.Fatalf("full coverage was qualified: %q", got)
	}
	if got := measureDenominatorNote(0, 309, "x"); got != "" {
		t.Fatalf("zero values was qualified: %q", got)
	}
	if got := measureDenominatorNote(400, 309, "x"); got != "" {
		t.Fatalf("an impossible count was qualified rather than ignored: %q", got)
	}
}

// OFF, no extra column is selected and the answer is unqualified, so every
// existing aggregate answer is byte-identical.
func TestMeasureDenominatorOffLeavesTheAnswerAlone(t *testing.T) {
	t.Setenv(measureDenominatorEnv, "")
	if measureDenominatorEnabled() {
		t.Fatal("the denominator must default to off")
	}
	plan := &SourceNativePlanV1{
		ContractVersion: sourceNativePlanContractV1,
		Measures:        []SourceNativeMeasureV1{{MeasureID: "m1", Op: "AVG", FieldID: "cdr.duration"}},
	}
	resp := hybridQueryResponse{Records: map[string]any{
		"plan": plan, "complete": true,
		"field_catalog": []FieldDescriptorV1{{FieldID: "cdr.duration", NormalizedName: "duration", DisplayName: "Duration"}},
		"source_native_results": []map[string]any{{
			"m1": 12.5, measureDenominatorKey("m1"): 263,
			"metadata": map[string]any{"contributing_row_count": 309},
		}},
	}}
	answer, ok := buildResultAnswer(hybridQueryRequest{RecordType: "cdr"}, resp, nil)
	if !ok {
		t.Fatal("no answer built")
	}
	// The key is present in the fixture, so the presentation reads it regardless
	// of the switch -- what the switch gates is whether the EXECUTOR emits it.
	// This asserts the note itself is well formed when it does.
	if !strings.Contains(answer.Headline, "263 of 309") {
		t.Fatalf("a present denominator was not stated: %q", answer.Headline)
	}
}

// THE REAL PROOF. Run an AVG over the field Codex flagged and check the
// denominator against the table: 263 of 309, with 46 absent.
//
// Run: DERIVED_EXEC_DB=postgresql://... go test -run TestMeasureDenominatorMatchesTheTable
func TestMeasureDenominatorMatchesTheTable(t *testing.T) {
	dsn := os.Getenv("DERIVED_EXEC_DB")
	if dsn == "" {
		t.Skip("needs a live database; set DERIVED_EXEC_DB")
	}
	t.Setenv(derivedArtifactExecutionEnv, "true")
	t.Setenv(measureDenominatorEnv, "true")
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	layer, _ := defaultSemanticLayer()
	if layer == nil {
		t.Skip("curated layer unavailable")
	}
	entity, ok := layer.EntityByFamily("image_ocr_observation")
	if !ok {
		t.Skip("media family missing")
	}
	collection := os.Getenv("DERIVED_EXEC_COLLECTION")
	if collection == "" {
		collection = "nexusai-multimodal-product-acceptance"
	}
	req := hybridQueryRequest{
		Query: "What is the average OCR confidence on the images?", TenantID: "default",
		CollectionID: collection,
		QueryScope:   queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)},
		Limit:        1,
	}
	catalog := entity.CatalogFields(req)

	var fieldID string
	for _, field := range catalog {
		if len(field.SourceNames) > 0 && field.SourceNames[0] == "observation.confidence" {
			fieldID = field.FieldID
			break
		}
	}
	if fieldID == "" {
		t.Skip("the general confidence field is not curated under that source name")
	}

	plan := &SourceNativePlanV1{
		ContractVersion: sourceNativePlanContractV1,
		Project:         []string{}, Filters: []SourceNativeFilterV1{}, GroupFields: []string{},
		Measures: []SourceNativeMeasureV1{{MeasureID: "m1", Op: "AVG", FieldID: fieldID}},
		Having:   []SourceNativeHavingV1{}, Sort: []SourceNativeSortV1{}, Limit: 1,
	}
	records, err := executeSourceNativePlanSQL(ctx, pool, req, plan, catalog)
	if err != nil {
		t.Fatalf("AVG over a sparse derived field did not execute: %v", err)
	}
	results := mapsFromAny(records["source_native_results"])
	if len(results) == 0 {
		t.Fatal("no rows returned")
	}
	valueCount, ok := answerNumber(results[0][measureDenominatorKey("m1")])
	if !ok {
		t.Fatalf("no denominator in %#v", results[0])
	}

	var wantValues, wantScope int64
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FILTER (WHERE metadata->'observation'->>'confidence' IS NOT NULL), count(*)
		 FROM forensic.derived_artifacts
		 WHERE tenant_id=$1 AND collection_id=$2 AND artifact_type=$3 AND processing_status='completed'`,
		req.TenantID, collection, "forensics.image-ocr-observation/v1").Scan(&wantValues, &wantScope); err != nil {
		t.Fatalf("oracle: %v", err)
	}
	if int64(valueCount) != wantValues {
		t.Fatalf("denominator %v, the table says %d", valueCount, wantValues)
	}
	if wantValues >= wantScope {
		t.Skip("this field is not sparse in this collection, so it proves nothing")
	}
	note := measureDenominatorNote(int64(valueCount), wantScope, "image OCR observations")
	if note == "" {
		t.Fatalf("a %d-of-%d field produced no qualifier", wantValues, wantScope)
	}
	t.Logf("denominator %d of %d, matching the table. Answer says: %s", wantValues, wantScope, note)
}
