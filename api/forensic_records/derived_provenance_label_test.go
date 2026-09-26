package main

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// STEP 2 -- THE PROVENANCE LABEL. The whole "a model's plate read is not a
// camera's sighting" property rests on the citation the analyst is shown. A
// derived observation cited as `records_sql` with NULL record columns is a
// FABRICATED SIGHTING in the only place the analyst goes to check.
//
// These tests prove the citation against the live database, not against a
// struct built by hand: the 9,580 defect was invisible to code review and
// obvious to one database test, and a probe must reproduce the PATH. The path
// here is the whole one -- plan -> SQL -> executor lineage -> display rows ->
// enterprise provenance -- because the executor was already CORRECT and the
// defect lived entirely in the presentation layer downstream of it.

// derivedProvenanceFixture runs a real derived aggregate and returns the
// analyst-facing provenance for it.
func derivedProvenanceFixture(t *testing.T, family, artifactType string) []map[string]any {
	t.Helper()
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
	t.Cleanup(pool.Close)

	layer, _ := defaultSemanticLayer()
	if layer == nil {
		t.Skip("curated layer unavailable")
	}
	entity, ok := layer.EntityByFamily(family)
	if !ok {
		t.Skipf("media family %s missing", family)
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
	plan := &SourceNativePlanV1{
		ContractVersion: sourceNativePlanContractV1,
		Project:         []string{}, Filters: []SourceNativeFilterV1{}, GroupFields: []string{},
		Measures: []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT"}},
		Having:   []SourceNativeHavingV1{}, Sort: []SourceNativeSortV1{}, Limit: 1,
	}
	records, err := executeSourceNativePlanSQL(ctx, pool, req, plan, entity.CatalogFields(req))
	if err != nil {
		t.Fatalf("a derived aggregate did not execute: %v", err)
	}
	resp := hybridQueryResponse{Records: records, Route: []string{"bounded_source_native_algebra"}}
	rows := enterpriseDisplayRows(req, resp)
	if len(rows) == 0 {
		t.Fatal("the derived aggregate produced no display rows, so this proves nothing")
	}
	provenance := enterpriseProvenance(req, rows, resp.Evidence, nil)
	if len(provenance) == 0 {
		t.Fatal("a derived aggregate over real artifacts cited nothing at all")
	}
	return provenance
}

// A DERIVED CITATION NEVER CLAIMS TO BE AN INGESTED RECORD.
//
// Measured 2026-09-26 before the fix: every item read
// `"source":"records_sql"` with record_type, row_hash, row_number, source_file
// and every source_locator component NULL -- the records columns
// forensic.derived_artifacts does not have.
func TestDerivedProvenanceNeverWearsTheRecordsShape(t *testing.T) {
	provenance := derivedProvenanceFixture(t, "video_anpr_plate_group", "forensics.video-anpr-plate-group/v1")
	for index, item := range provenance {
		if got := stringValueAny(item["source"]); got != "derived_artifacts_sql" {
			t.Errorf("citation %d claims source %q; a model observation must never be cited as an ingested record", index, got)
		}
		if got := stringValueAny(item["source_table"]); got != "forensic.derived_artifacts" {
			t.Errorf("citation %d names source_table %q", index, got)
		}
		// A records column must be ABSENT, not null. A null reads as "this
		// evidence has no such value", which is a claim about the evidence
		// rather than a statement about the table it came from.
		for _, key := range []string{"record_id", "record_type", "row_hash", "row_number", "source_file", "batch_id", "file_id"} {
			if _, present := item[key]; present {
				t.Errorf("citation %d carries records column %q (value %v); derived_artifacts has no such column", index, key, item[key])
			}
		}
	}
}

// THE ANALYST CAN ATTRIBUTE THE OBSERVATION. Without artifact_id and
// artifact_type the citation identifies nothing: derived PROJECTION is refused
// precisely because rows without provenance hand the analyst evidence they
// cannot attribute, and an aggregate's citation must clear the same bar.
func TestDerivedProvenanceCarriesTheArtifactIdentity(t *testing.T) {
	provenance := derivedProvenanceFixture(t, "video_anpr_plate_group", "forensics.video-anpr-plate-group/v1")
	for index, item := range provenance {
		if stringValueAny(item["artifact_id"]) == "" {
			t.Errorf("citation %d carries no artifact_id, so the observation cannot be attributed", index)
		}
		if got := stringValueAny(item["artifact_type"]); got != "forensics.video-anpr-plate-group/v1" {
			t.Errorf("citation %d names artifact_type %q", index, got)
		}
		if stringValueAny(item["evidence_id"]) == "" {
			t.Errorf("citation %d names no evidence_id, so the media it came from is unidentified", index)
		}
		if item["citation_locator"] == nil {
			t.Errorf("citation %d carries no locator, so its position in the media is unstated", index)
		}
	}
}

// `source_truth_state` IS THE FIELD THAT KEEPS THE TWO APART. It was selected
// correctly by the executor and then dropped entirely by the presentation
// layer, so nothing the analyst could see distinguished a model's plate read
// from a camera's sighting.
func TestDerivedProvenanceSurfacesSourceTruthState(t *testing.T) {
	provenance := derivedProvenanceFixture(t, "video_anpr_plate_group", "forensics.video-anpr-plate-group/v1")
	for index, item := range provenance {
		state := stringValueAny(item["source_truth_state"])
		if state == "" {
			t.Errorf("citation %d does not surface source_truth_state", index)
			continue
		}
		if state == "ingested_source_record" {
			t.Errorf("citation %d declares itself an ingested source record", index)
		}
	}
}

// EVERY DISTINCT ARTIFACT IS CITED. The de-duplication key was built from
// row_hash, source_file, source_entry, row_number and citation -- all NULL on a
// derived row -- and one video yields one evidence_id per artifact, so all 24
// plate groups collapsed to a SINGLE citation. One unattributable item standing
// in for 24 observations is indistinguishable from a citation for evidence that
// was never examined.
func TestDerivedProvenanceDoesNotCollapseDistinctArtifacts(t *testing.T) {
	provenance := derivedProvenanceFixture(t, "video_anpr_plate_group", "forensics.video-anpr-plate-group/v1")
	ids := map[string]struct{}{}
	for _, item := range provenance {
		ids[stringValueAny(item["artifact_id"])] = struct{}{}
	}
	if len(ids) != len(provenance) {
		t.Errorf("%d citations cover only %d distinct artifacts", len(provenance), len(ids))
	}
	// The contract holds 24 completed artifacts and the lineage cap is 50, so
	// every one of them must appear. A count that silently shrinks here is the
	// original defect returning.
	if len(ids) < 24 {
		t.Errorf("only %d of the 24 plate-group artifacts were cited", len(ids))
	}
	t.Logf("%d distinct artifacts cited, one per plate group", len(ids))
}

// THE RECORDS PATH IS UNTOUCHED. This change ships without a switch, so its
// inertness on structured evidence is what bounds it: a records source row must
// produce byte-identical provenance to what it produced before, including the
// records-shaped source_locator and the `records_sql` label.
func TestRecordsProvenanceIsUnchangedByTheDerivedBranch(t *testing.T) {
	req := hybridQueryRequest{CollectionID: "case-alpha"}
	rows := []map[string]any{{"metadata": map[string]any{"source_rows": []map[string]any{{
		"source": "records_sql", "source_table": "forensic.records",
		"record_id": "rec-1", "record_type": "CDR", "source_file": "cdr.csv",
		"row_number": 7, "batch_id": "batch-1", "file_id": "file-1", "row_hash": "hash-1",
		"timestamp": "2026-04-01T00:00:00Z", "evidence_id": "ev-1", "version_id": "ver-1",
	}}}}}
	provenance := enterpriseProvenance(req, rows, nil, nil)
	if len(provenance) != 1 {
		t.Fatalf("a single records source row produced %d citations", len(provenance))
	}
	item := provenance[0]
	for key, want := range map[string]any{
		"source": "records_sql", "collection_id": "case-alpha", "evidence_id": "ev-1",
		"version_id": "ver-1", "row_hash": "hash-1", "record_type": "CDR",
		"source_file": "cdr.csv", "row_number": 7, "timestamp": "2026-04-01T00:00:00Z",
	} {
		if item[key] != want {
			t.Errorf("records citation %s = %v, want %v", key, item[key], want)
		}
	}
	locator, ok := item["source_locator"].(map[string]any)
	if !ok {
		t.Fatalf("records citation lost its source_locator: %#v", item["source_locator"])
	}
	for key, want := range map[string]any{"record_id": "rec-1", "batch_id": "batch-1", "file_id": "file-1", "row_number": 7} {
		if locator[key] != want {
			t.Errorf("records locator %s = %v, want %v", key, locator[key], want)
		}
	}
	if _, present := item["artifact_id"]; present {
		t.Error("a records citation must not carry an artifact_id")
	}
}

// THE METRICS PANEL AGREES WITH THE CITATIONS. It sits beside them on the same
// screen, so a derived answer whose panel reads `records_sql` re-asserts the
// same falsehood in the second place the analyst looks.
func TestAnswerSourceLabelFollowsTheCitations(t *testing.T) {
	derivedProv := []map[string]any{{"source": "derived_artifacts_sql", "source_table": "forensic.derived_artifacts"}}
	if got := answerSourceLabel(derivedProv); got != "derived_artifacts_sql" {
		t.Errorf("a derived answer is labelled %q", got)
	}
	recordsProv := []map[string]any{{"source": "records_sql", "source_table": "forensic.records"}}
	if got := answerSourceLabel(recordsProv); got != "records_sql" {
		t.Errorf("a records answer is labelled %q", got)
	}
	// kb_rag and aggregate-lineage items must not flip the label either way.
	for _, item := range []map[string]any{{"source": "kb_rag"}, {"source": "records_aggregate"}} {
		if got := answerSourceLabel(append(append([]map[string]any{}, derivedProv...), item)); got != "derived_artifacts_sql" {
			t.Errorf("a derived answer carrying a %v item is labelled %q", item["source"], got)
		}
	}
	// No citations at all must never be reported as derived.
	if got := answerSourceLabel(nil); got != "records_sql" {
		t.Errorf("an uncited answer is labelled %q", got)
	}
	if got := answerSourceLabel(append(append([]map[string]any{}, derivedProv...), recordsProv...)); got != "records_sql+derived_artifacts_sql" {
		t.Errorf("a mixed answer is labelled %q", got)
	}
}

// THE NEGATIVE CONTROL. A test that passes both before and after a fix proves
// nothing, and three of the suites above assert on shape rather than on a value,
// which is exactly the kind of assertion that can be vacuously true.
//
// This rebuilds the citation EXACTLY as the code built it before this change --
// from a real derived source row read out of the database -- and asserts that
// each of the four properties above rejects it. If any of these stops failing,
// the corresponding assertion above has gone blind.
func TestDerivedProvenanceAssertionsRejectTheOldShape(t *testing.T) {
	sourceRow := oneRealDerivedSourceRow(t)

	// Verbatim reconstruction of the pre-2026-09-26 construction: the label
	// hardcoded, and only records columns copied across.
	legacy := map[string]any{
		"source":        "records_sql",
		"collection_id": "nexusai-multimodal-product-acceptance",
		"evidence_id":   firstPresent(sourceRow, "evidence_id"),
		"version_id":    firstPresent(sourceRow, "version_id"),
		"row_hash":      sourceRow["row_hash"],
		"record_type":   sourceRow["record_type"],
		"source_file":   sourceRow["source_file"],
		"row_number":    sourceRow["row_number"],
		"timestamp":     sourceRow["timestamp"],
		"source_locator": map[string]any{
			"record_id": sourceRow["record_id"], "batch_id": sourceRow["batch_id"],
			"file_id": sourceRow["file_id"], "row_number": sourceRow["row_number"],
		},
	}

	// 1. It claims to be an ingested record.
	if stringValueAny(legacy["source"]) == "derived_artifacts_sql" {
		t.Error("the old shape already carried the derived label; the label assertion cannot discriminate")
	}
	// 2. It carries records columns the table does not have, as nulls.
	nulls := 0
	for _, key := range []string{"record_type", "row_hash", "row_number", "source_file"} {
		value, present := legacy[key]
		if present && value == nil {
			nulls++
		}
	}
	if nulls != 4 {
		t.Errorf("the old shape presented %d of 4 records columns as null; the defect this documents is not reproduced", nulls)
	}
	// 3. It cannot be attributed.
	if legacy["artifact_id"] != nil || legacy["artifact_type"] != nil {
		t.Error("the old shape already carried the artifact identity")
	}
	// 4. source_truth_state is absent, so nothing distinguishes a model read
	//    from a camera sighting.
	if legacy["source_truth_state"] != nil {
		t.Error("the old shape already surfaced source_truth_state")
	}

	// And the row the executor handed it DID carry all of it -- which is what
	// makes this a presentation defect rather than a missing capability.
	if stringValueAny(sourceRow["artifact_id"]) == "" {
		t.Error("the executor lineage carried no artifact_id, so the defect was not only in presentation")
	}
	if stringValueAny(sourceRow["source_truth_state"]) == "" {
		t.Error("the executor lineage carried no source_truth_state")
	}
	t.Logf("old shape: source=%v record_type=%v row_hash=%v source_file=%v artifact_id=%v",
		legacy["source"], legacy["record_type"], legacy["row_hash"], legacy["source_file"], legacy["artifact_id"])
}

// oneRealDerivedSourceRow reads a single derived lineage row straight out of the
// executor, so the negative control is built from real evidence rather than a
// hand-written map that might not match what the database produces.
func oneRealDerivedSourceRow(t *testing.T) map[string]any {
	t.Helper()
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
	t.Cleanup(pool.Close)
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
		TenantID: "default", CollectionID: collection,
		QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)}, Limit: 1,
	}
	plan := &SourceNativePlanV1{
		ContractVersion: sourceNativePlanContractV1,
		Project:         []string{}, Filters: []SourceNativeFilterV1{}, GroupFields: []string{},
		Measures: []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT"}},
		Having:   []SourceNativeHavingV1{}, Sort: []SourceNativeSortV1{}, Limit: 1,
	}
	records, err := executeSourceNativePlanSQL(ctx, pool, req, plan, entity.CatalogFields(req))
	if err != nil {
		t.Fatalf("a derived aggregate did not execute: %v", err)
	}
	results := mapsFromAny(records["source_native_results"])
	if len(results) == 0 {
		t.Fatal("no results")
	}
	metadata, _ := results[0]["metadata"].(map[string]any)
	sourceRows := mapsFromAny(metadata["source_rows"])
	if len(sourceRows) == 0 {
		t.Fatal("the derived aggregate carried no lineage at all")
	}
	return sourceRows[0]
}

// The classifier reads the LINEAGE, never the question or the family. A source
// row is derived because the SQL that produced it said so.
func TestSourceRowIsDerivedReadsTheLineage(t *testing.T) {
	derived := []map[string]any{
		{"source": "derived_artifacts_sql"},
		{"source": "derived_observation"},
		{"source": "derived_text"},
		{"source_table": "forensic.derived_artifacts"},
		{"source": " derived_artifacts_sql "},
	}
	for index, row := range derived {
		if !sourceRowIsDerived(row) {
			t.Errorf("derived case %d (%v) was not recognised as derived", index, row)
		}
	}
	records := []map[string]any{
		{"source": "records_sql", "source_table": "forensic.records"},
		{"source": "records_aggregate"},
		{"source": "kb_rag"},
		{},
	}
	for index, row := range records {
		if sourceRowIsDerived(row) {
			t.Errorf("records case %d (%v) was misread as derived", index, row)
		}
	}
}
