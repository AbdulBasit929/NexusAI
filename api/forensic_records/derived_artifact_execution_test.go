package main

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// A structured family resolves to forensic.records, exactly as before derived
// artifacts existed.
func TestDerivedBindingStructuredFamilyReadsRecords(t *testing.T) {
	binding, err := sourceNativeBindingForFieldIDs([]string{"cdr.call_type", "cdr.msisdn"})
	if err != nil {
		t.Fatalf("resolving a records plan failed: %v", err)
	}
	if binding.Derived || binding.Table != "forensic.records" || binding.Payload != "raw_payload" {
		t.Fatalf("records plan resolved to %+v", binding)
	}
	if binding.ArtifactType != "" {
		t.Fatalf("a records plan must carry no artifact_type, got %q", binding.ArtifactType)
	}
}

// A media family resolves to forensic.derived_artifacts, carrying the contract
// that scopes it.
func TestDerivedBindingMediaFamilyReadsDerivedArtifacts(t *testing.T) {
	binding, err := sourceNativeBindingForFieldIDs([]string{"video_anpr_plate_group.sightings_count"})
	if err != nil {
		t.Fatalf("resolving a derived plan failed: %v", err)
	}
	if !binding.Derived {
		t.Fatalf("media plan did not resolve as derived: %+v", binding)
	}
	if binding.Table != "forensic.derived_artifacts" || binding.Payload != "metadata" {
		t.Fatalf("media plan resolved to %+v", binding)
	}
	if binding.ArtifactType != "forensics.video-anpr-plate-group/v1" {
		t.Fatalf("artifact_type resolved to %q", binding.ArtifactType)
	}
}

// MIXING SOURCES IS REFUSED. One table cannot answer a plan naming both, and
// both alternatives are unacceptable: reading one silently answers about
// evidence the analyst did not ask about, and joining them asserts a
// relationship between an ingested record and a model's guess that nothing in
// the data establishes.
func TestDerivedBindingRefusesAMixedPlan(t *testing.T) {
	_, err := sourceNativeBindingForFieldIDs([]string{"cdr.call_type", "anpr_model_observation.ocr_confidence"})
	if err == nil {
		t.Fatal("a plan mixing an ingested record with a model observation was accepted")
	}
	if !strings.Contains(err.Error(), "mixes evidence sources") {
		t.Fatalf("refusal did not name the cause: %v", err)
	}
}

// The switch is OFF by default, and with it off a derived plan is refused
// rather than executed, so the records path is untouched.
func TestDerivedExecutionIsOffByDefault(t *testing.T) {
	t.Setenv(derivedArtifactExecutionEnv, "")
	if derivedArtifactExecutionEnabled() {
		t.Fatal("derived execution must default to off")
	}
	t.Setenv(derivedArtifactExecutionEnv, "true")
	if !derivedArtifactExecutionEnabled() {
		t.Fatal("the switch did not enable derived execution")
	}
}

// Nested paths are what make an observation's fields reachable:
// "observation.start_seconds" must become metadata->'observation'->>'start_seconds',
// with every segment BOUND, never interpolated.
func TestDerivedPayloadExprBuildsABoundNestedPath(t *testing.T) {
	binding := sourceNativeBinding{Table: "forensic.derived_artifacts", Payload: "metadata", Derived: true}
	var args []any
	expr := sourceNativePayloadTextExpr(binding, "d", "observation.start_seconds", &args)
	if expr != "d.metadata->$1->>$2" {
		t.Fatalf("nested path built %q", expr)
	}
	if len(args) != 2 || args[0] != "observation" || args[1] != "start_seconds" {
		t.Fatalf("path segments were not bound: %#v", args)
	}
	// A records field is ONE literal key, never split -- "Registration No."
	// ends in a period.
	records := sourceNativeRecordsBinding()
	var recordArgs []any
	if got := sourceNativePayloadTextExpr(records, "r", "Registration No.", &recordArgs); got != "r.raw_payload->>$1" {
		t.Fatalf("records path built %q", got)
	}
	if len(recordArgs) != 1 || recordArgs[0] != "Registration No." {
		t.Fatalf("records key was split: %#v", recordArgs)
	}
}

// Every field a plan touches must be counted, or a mixed plan slips through the
// source check on the field it forgot to look at.
func TestDerivedPlanFieldIDsCoversEveryReference(t *testing.T) {
	plan := &SourceNativePlanV1{
		Project:     []string{"a.one"},
		GroupFields: []string{"a.two"},
		Filters:     []SourceNativeFilterV1{{FieldID: "a.three"}},
		Measures: []SourceNativeMeasureV1{
			{MeasureID: "m1", Op: "COUNT"},
			{MeasureID: "m2", Op: "AVG", FieldID: "a.four"},
		},
		TimeBucket: &SourceNativeTimeBucketV1{FieldID: "a.five"},
	}
	got := map[string]bool{}
	for _, id := range sourceNativePlanFieldIDs(plan) {
		got[id] = true
	}
	for _, want := range []string{"a.one", "a.two", "a.three", "a.four", "a.five"} {
		if !got[want] {
			t.Errorf("%s was not counted as a referenced field", want)
		}
	}
	// A COUNT(*) measure names no field and must not produce an empty ID.
	if got[""] {
		t.Error("an empty field ID was counted")
	}
}

// The discriminator narrows a contract carrying more than one kind of row, and
// is meaningless for forensic.records.
func TestSemanticSourceObservationTypeIsDerivedOnly(t *testing.T) {
	good := SemanticLayerSourceV1{
		Table: semanticSourceTableDerived, ArtifactType: "forensics.image-observation/v1",
		Payload: "metadata", ObservationType: "video_sampled_frame_observation",
	}
	if err := validateSemanticSource("entity x", good); err != nil {
		t.Fatalf("a discriminated derived source was rejected: %v", err)
	}
	onRecords := SemanticLayerSourceV1{ObservationType: "anything"}
	if err := validateSemanticSource("entity y", onRecords); err == nil {
		t.Fatal("observation_type was accepted on forensic.records, where it means nothing")
	}
	blank := SemanticLayerSourceV1{
		Table: semanticSourceTableDerived, ArtifactType: "forensics.x/v1", ObservationType: "   ",
	}
	if err := validateSemanticSource("entity z", blank); err == nil {
		t.Fatal("a blank discriminator was accepted; it would match the empty string")
	}
}

// THE SQL SEMANTICS, proven against the table rather than by reading the string.
// forensics.image-observation/v1 holds 26 rows: 20 technical observations and 6
// sampled video frames. The predicate must split them exactly, and an EXACT
// match matters -- a prefix match would pull
// video_embedded_audio_transcript_segment into audio_transcript_segment.
func TestDerivedObservationTypePredicateNarrowsExactly(t *testing.T) {
	dsn := os.Getenv("DERIVED_EXEC_DB")
	if dsn == "" {
		t.Skip("needs a live database; set DERIVED_EXEC_DB")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()
	collection := os.Getenv("DERIVED_EXEC_COLLECTION")
	if collection == "" {
		collection = "nexusai-multimodal-product-acceptance"
	}

	count := func(observationType string) int64 {
		var n int64
		sql := `SELECT COUNT(*) FROM forensic.derived_artifacts d
		        WHERE d.tenant_id=$1 AND d.collection_id=$2 AND d.processing_status=$3 AND d.artifact_type=$4`
		args := []any{"default", collection, "completed", "forensics.image-observation/v1"}
		if observationType != "" {
			sql += ` AND d.metadata->>'observation_type'=$5`
			args = append(args, observationType)
		}
		if err := pool.QueryRow(ctx, sql, args...).Scan(&n); err != nil {
			t.Fatalf("count(%q): %v", observationType, err)
		}
		return n
	}

	whole := count("")
	technical := count("image_technical_observation")
	frames := count("video_sampled_frame_observation")
	if whole == 0 {
		t.Skip("this collection holds no image-observation rows")
	}
	if technical+frames != whole {
		t.Fatalf("the discriminator does not partition the contract: %d + %d != %d", technical, frames, whole)
	}
	if technical == 0 || frames == 0 {
		t.Fatalf("one side of the split is empty (%d technical, %d frames), so it proves nothing", technical, frames)
	}
	t.Logf("forensics.image-observation/v1 partitions exactly: %d technical + %d sampled frames = %d", technical, frames, whole)
}

// THE 9,580 BUG. A plain COUNT(*) carries NO field ID, and "how many plate
// groups were read from the videos?" is exactly that plan. Resolving the binding
// from the plan's fields alone left nothing to resolve, so it fell back to
// forensic.records and counted 9,580 structured rows where the truth is 24.
//
// Found by the database test below, not by reading the code.
func TestDerivedBindingResolvesACountStarFromTheIssuedCatalogue(t *testing.T) {
	layer, _ := defaultSemanticLayer()
	if layer == nil {
		t.Skip("curated layer unavailable")
	}
	entity, ok := layer.EntityByFamily("video_anpr_plate_group")
	if !ok {
		t.Skip("media family missing")
	}
	catalog := entity.CatalogFields(hybridQueryRequest{})
	if len(catalog) == 0 {
		t.Fatal("the media entity issued no catalogue fields")
	}
	countStar := &SourceNativePlanV1{
		ContractVersion: sourceNativePlanContractV1,
		Measures:        []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT"}},
	}
	if ids := sourceNativePlanFieldIDs(countStar); len(ids) != 0 {
		t.Fatalf("a COUNT(*) plan should name no field, got %v", ids)
	}
	binding, err := sourceNativeBindingForPlan("How many plate groups were tracked across the video frames?", countStar, catalog)
	if err != nil {
		t.Fatalf("scoping a COUNT(*) plan failed: %v", err)
	}
	if !binding.Derived || binding.ArtifactType != "forensics.video-anpr-plate-group/v1" {
		t.Fatalf("a COUNT(*) over a media catalogue resolved to %+v — this is the 9,580 defect", binding)
	}

	// And the records case must still resolve to records.
	cdr, ok := layer.EntityByFamily("communications_cdr")
	if !ok {
		t.Skip("cdr family missing")
	}
	recordsBinding, err := sourceNativeBindingForPlan("How many CDR records do we have in this case?", countStar, cdr.CatalogFields(hybridQueryRequest{}))
	if err != nil {
		t.Fatalf("scoping a records COUNT(*) failed: %v", err)
	}
	if recordsBinding.Derived {
		t.Fatalf("a COUNT(*) over a CDR catalogue resolved to derived: %+v", recordsBinding)
	}
}

// THE REAL PROOF: run a derived aggregate against Postgres and check the number
// against the contract's own row count. A binding that compiles but selects the
// wrong table, or drops the artifact_type filter, still returns A number -- and
// a wrong count over forensic evidence reads exactly like a right one.
//
// Run: DERIVED_EXEC_DB=postgresql://... go test -run TestDerivedArtifactAggregateExecutes
func TestDerivedArtifactAggregateExecutesAgainstPostgres(t *testing.T) {
	dsn := os.Getenv("DERIVED_EXEC_DB")
	if dsn == "" {
		t.Skip("needs a live database; set DERIVED_EXEC_DB")
	}
	t.Setenv(derivedArtifactExecutionEnv, "true")
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
	entity, ok := layer.EntityByFamily("video_anpr_plate_group")
	if !ok {
		t.Skip("media family missing")
	}
	collection := os.Getenv("DERIVED_EXEC_COLLECTION")
	if collection == "" {
		collection = "nexusai-multimodal-product-acceptance"
	}
	req := hybridQueryRequest{
		Query:        "How many plate groups were read from the videos?",
		TenantID:     "default",
		CollectionID: collection,
		QueryScope:   queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)},
		Limit:        1,
	}
	catalog := entity.CatalogFields(req)
	plan := &SourceNativePlanV1{
		ContractVersion: sourceNativePlanContractV1,
		Project:         []string{}, Filters: []SourceNativeFilterV1{}, GroupFields: []string{},
		Measures: []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT"}},
		Having:   []SourceNativeHavingV1{}, Sort: []SourceNativeSortV1{}, Limit: 1,
	}
	records, err := executeSourceNativePlanSQL(ctx, pool, req, plan, catalog)
	if err != nil {
		t.Fatalf("a derived aggregate did not execute: %v", err)
	}

	// The independent oracle: count the contract's completed rows directly.
	var want int64
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM forensic.derived_artifacts
		 WHERE tenant_id=$1 AND collection_id=$2 AND artifact_type=$3 AND processing_status='completed'`,
		req.TenantID, collection, "forensics.video-anpr-plate-group/v1").Scan(&want); err != nil {
		t.Fatalf("oracle query: %v", err)
	}

	results := mapsFromAny(records["source_native_results"])
	if len(results) == 0 {
		t.Fatal("a derived aggregate returned no rows at all")
	}
	got, ok := answerNumber(results[0]["m1"])
	if !ok {
		t.Fatalf("no measure value in %#v", results[0])
	}
	if int64(got) != want {
		t.Fatalf("derived COUNT returned %v, the table holds %d — the binding read the wrong rows", got, want)
	}
	if want == 0 {
		t.Fatal("the oracle found no rows, so this proves nothing; point it at a collection with media")
	}
	t.Logf("derived COUNT over %s = %d, matching the table", "forensics.video-anpr-plate-group/v1", want)
}

// NESTED EXTRACTION, proven in real SQL rather than by string comparison. The
// analytical fields of an observation live under metadata->'observation', so an
// AVG over one of them exercises the whole path: nested jsonb, the numeric
// guard, and the artifact_type scope together.
//
// A probe must reproduce the PATH, not just the function -- calling the right
// builder with the right arguments once concluded, confidently and wrongly, that
// the GBNF keystone was broken.
func TestDerivedArtifactNestedAverageMatchesTheTable(t *testing.T) {
	dsn := os.Getenv("DERIVED_EXEC_DB")
	if dsn == "" {
		t.Skip("needs a live database; set DERIVED_EXEC_DB")
	}
	t.Setenv(derivedArtifactExecutionEnv, "true")
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
	entity, ok := layer.EntityByFamily("anpr_model_observation")
	if !ok {
		t.Skip("media family missing")
	}
	collection := os.Getenv("DERIVED_EXEC_COLLECTION")
	if collection == "" {
		collection = "nexusai-multimodal-product-acceptance"
	}
	req := hybridQueryRequest{
		Query: "What is the average OCR confidence of the plate reads?", TenantID: "default",
		CollectionID: collection,
		QueryScope:   queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)},
		Limit:        1,
	}
	catalog := entity.CatalogFields(req)

	// Find a NUMBER field of this entity that resolves through a nested path.
	var fieldID, sourceName string
	for _, field := range catalog {
		if field.EffectiveType != fieldTypeDecimal && field.EffectiveType != fieldTypeInteger {
			continue
		}
		if len(field.SourceNames) == 0 || !strings.Contains(field.SourceNames[0], ".") {
			continue
		}
		fieldID, sourceName = field.FieldID, field.SourceNames[0]
		break
	}
	if fieldID == "" {
		t.Skip("no nested numeric field is curated on this entity")
	}

	plan := &SourceNativePlanV1{
		ContractVersion: sourceNativePlanContractV1,
		Project:         []string{}, Filters: []SourceNativeFilterV1{}, GroupFields: []string{},
		Measures: []SourceNativeMeasureV1{{MeasureID: "m1", Op: "AVG", FieldID: fieldID}},
		Having:   []SourceNativeHavingV1{}, Sort: []SourceNativeSortV1{}, Limit: 1,
	}
	records, err := executeSourceNativePlanSQL(ctx, pool, req, plan, catalog)
	if err != nil {
		t.Fatalf("a nested derived AVG did not execute: %v", err)
	}

	segments := strings.SplitN(sourceName, ".", 2)
	if len(segments) != 2 {
		t.Fatalf("expected a nested source name, got %q", sourceName)
	}
	var want float64
	if err := pool.QueryRow(ctx,
		`SELECT AVG((metadata->$4->>$5)::numeric) FROM forensic.derived_artifacts
		 WHERE tenant_id=$1 AND collection_id=$2 AND artifact_type=$3 AND processing_status='completed'
		   AND (metadata->$4->>$5) ~ '^-?[0-9]+(\.[0-9]+)?$'`,
		req.TenantID, collection, "forensics.anpr-observation/v1", segments[0], segments[1]).Scan(&want); err != nil {
		t.Fatalf("oracle query: %v", err)
	}

	results := mapsFromAny(records["source_native_results"])
	if len(results) == 0 {
		t.Fatal("a nested derived AVG returned no rows")
	}
	got, ok := answerNumber(results[0]["m1"])
	if !ok {
		t.Fatalf("no measure value in %#v", results[0])
	}
	if diff := got - want; diff > 1e-6 || diff < -1e-6 {
		t.Fatalf("AVG over %s returned %v, the table says %v", sourceName, got, want)
	}
	t.Logf("nested AVG over %s = %v, matching the table", sourceName, got)
}

// THE HTTP 500 THIS BINDING CAUSED, measured live 2026-09-26.
//
// "How many faces were detected in the evidence?" is a COUNT(*) naming no field.
// The issued catalogue legitimately mixes CURATED media fields with INFERRED
// forensic.records fields, so resolving the fallback by "every catalogue field
// must agree" refused the question -- and surfaced as an HTTP 500. The Phase 1
// gate is ZERO HTTP 500s.
//
// The question's own family is the precise answer to "what evidence is this
// COUNT about", and it is what the compiler used to choose the catalogue.
func TestDerivedBindingScopesACountStarByTheQuestionsFamily(t *testing.T) {
	t.Setenv(mediaFamilyRoutingEnv, "true")
	layer, _ := defaultSemanticLayer()
	if layer == nil {
		t.Skip("curated layer unavailable")
	}
	entity, ok := layer.EntityByFamily("face_model_observation")
	if !ok {
		t.Skip("media family missing")
	}
	// A MIXED catalogue: curated media fields plus an inferred records field,
	// which is exactly what the compiler issues.
	catalog := append(entity.CatalogFields(hybridQueryRequest{}), FieldDescriptorV1{
		FieldID: "inferred.some_column", NormalizedName: "some_column", SourceNames: []string{"some_column"},
	})
	countStar := &SourceNativePlanV1{
		ContractVersion: sourceNativePlanContractV1,
		Measures:        []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT"}},
	}
	binding, err := sourceNativeBindingForPlan("How many faces were detected in the evidence?", countStar, catalog)
	if err != nil {
		t.Fatalf("a mixed catalogue must not error -- that is the HTTP 500: %v", err)
	}
	if !binding.Derived || binding.ArtifactType != "forensics.face-observation/v1" {
		t.Fatalf("the question's family did not scope the COUNT: %+v", binding)
	}

	// And a question naming NO family with a mixed catalogue must still not
	// error; it falls back to records and is declined downstream.
	if _, err := sourceNativeBindingForPlan("qqq zzz", countStar, catalog); err != nil {
		t.Fatalf("an unscopeable COUNT must not error: %v", err)
	}
}
