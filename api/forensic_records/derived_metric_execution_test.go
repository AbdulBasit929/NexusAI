package main

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Executes the derived-metric plan against a REAL database.
//
// TestDerivedDurationMetricIsExpressibleAndExecutes asserted the SQL STRING
// contained EXTRACT and never ran it. The name said "AndExecutes"; it executed
// nothing. Shadow mode generates plans without executing them, so the defect
// stayed invisible until verified-only made the plan actually run, and then it
// was an HTTP-500-class error on a gate criterion:
//
//	ERROR: could not determine data type of parameter $4 (SQLSTATE 42P18)
//
// The typed expression was built and then discarded for a derived field,
// leaving its placeholders in args with nothing referencing them.
//
// Run: DERIVED_EXEC_DB=postgresql://... go test -run TestDerivedMetricExecutes
func TestDerivedMetricExecutesAgainstPostgres(t *testing.T) {
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

	layer, _ := defaultSemanticLayer()
	if layer == nil {
		t.Skip("curated layer unavailable")
	}
	entity, ok := layer.EntityByFamily("communications_cdr")
	if !ok {
		t.Skip("family missing")
	}
	req := hybridQueryRequest{
		Query: "What was the longest call?", RecordType: "cdr",
		TenantID:     os.Getenv("DERIVED_EXEC_TENANT"),
		CollectionID: os.Getenv("DERIVED_EXEC_COLLECTION"),
		QueryScope:   queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)},
		Limit:        1,
	}
	if req.TenantID == "" {
		req.TenantID = "default"
	}
	if req.CollectionID == "" {
		req.CollectionID = "nexusai-forensic-demo"
	}
	catalog := entity.CatalogFields(req)
	plan := &SourceNativePlanV1{
		ContractVersion: sourceNativePlanContractV1,
		Project:         []string{}, Filters: []SourceNativeFilterV1{}, GroupFields: []string{},
		Measures: []SourceNativeMeasureV1{{MeasureID: "m1", Op: "MAX", FieldID: "cdr.call_duration_seconds"}},
		Having:   []SourceNativeHavingV1{}, Sort: []SourceNativeSortV1{}, Limit: 1,
	}
	records, err := executeSourceNativePlanSQL(ctx, pool, req, plan, catalog)
	if err != nil {
		t.Fatalf("derived metric did not EXECUTE: %v", err)
	}
	if records == nil {
		t.Fatal("no result from a derived metric plan")
	}
	t.Logf("derived duration executed; result contract = %v", records["contract_version"])
}

// DISTINCT is an obligation. "How many unique phone numbers appear as callers"
// wants COUNT(DISTINCT msisdn) = 10; a plain COUNT answers 8,642 and is
// indistinguishable from a right answer.
//
// This regressed the moment COUNT_DISTINCT became expressible: before that the
// question generated no plan and safely clarified, and making it expressible
// let a WRONG plan through instead. A new capability must arrive with the check
// that bounds it.
func TestDistinctQuestionRejectsAPlainCount(t *testing.T) {
	question := "How many unique phone numbers appear as callers in the CDRs?"
	frame := extractSemanticFrame(hybridQueryRequest{Query: question})
	if frame.Goal != "distinct" {
		t.Fatalf("goal = %q, want distinct — the obligation keys off it", frame.Goal)
	}
	plain := &SourceNativePlanV1{
		ContractVersion: sourceNativePlanContractV1,
		Measures:        []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT"}},
	}
	if err := verifySourceNativePlanShape(frame, question, plain); err == nil {
		t.Fatal("a plain COUNT must be rejected for a distinct question, not stated as fact")
	}
	distinct := &SourceNativePlanV1{
		ContractVersion: sourceNativePlanContractV1,
		Measures: []SourceNativeMeasureV1{
			{MeasureID: "m1", Op: "COUNT_DISTINCT", FieldID: "cdr.msisdn"}},
	}
	if err := verifySourceNativePlanShape(frame, question, distinct); err != nil {
		t.Fatalf("a correct COUNT_DISTINCT plan must pass, got %v", err)
	}
	// An ordinary count question must not be dragged into the obligation.
	ordinary := "How many CDR records do we have in this case?"
	of := extractSemanticFrame(hybridQueryRequest{Query: ordinary})
	if err := verifySourceNativePlanShape(of, ordinary, plain); err != nil {
		t.Fatalf("a plain count question must still accept COUNT, got %v", err)
	}
}

// A question naming a media artifact answered by a STRUCTURED template is a
// misroute, not a coverage gap. Measured 2026-09-23: "What remote access setup
// does the HP guide describe?" was answered from access_failed_events because
// "access" matched the access-log family, and an image-text question was
// answered from cross_family_correlation.
//
// Keyed on the artifact noun it removes 2 wrong answers and loses NOTHING.
// Withholding every unverified structured answer instead measured 4 wrong
// removed against 3 CORRECT lost.
func TestStructuredTemplateMustNotAnswerMediaQuestions(t *testing.T) {
	scoped := func(q string) hybridQueryRequest {
		return hybridQueryRequest{Query: q, TenantID: "t", CollectionID: "c",
			QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)}}
	}
	misroutes := map[string]string{
		"What remote access setup does the HP guide describe?": "access_failed_events",
		"Which image contains the text REF-2026-02?":           "cross_family_correlation",
	}
	for question, template := range misroutes {
		if !structuredTemplateAnsweringMediaQuestion(scoped(question), template) {
			t.Errorf("%q answered by %s is a misroute and must be withheld", question, template)
		}
	}
	// A structured question must still be answered by a structured template.
	if structuredTemplateAnsweringMediaQuestion(scoped("How many CDR records do we have in this case?"), "canonical_records") {
		t.Error("a structured question names no media artifact; it must not be withheld")
	}
	// A media question answered by its OWN media executor is correct, not a misroute.
	if structuredTemplateAnsweringMediaQuestion(scoped("How many images are in this case?"), "image_metadata") {
		t.Error("image_metadata is the right executor for an image question")
	}
	// INVARIANT CHANGED 2026-09-24, and the reason matters more than the change.
	//
	// This previously asserted that a plan-backed answer must NEVER be withheld
	// here -- that verification had earned it the right to speak. DOC-01
	// disproved it: "Search the case documents for mentions of plate MN1367"
	// compiled a perfectly sound ANPR plan, correctly filtered on the plate,
	// and answered "There are no ANPR sightings involving MN1367". Sound
	// arithmetic over the wrong evidence.
	//
	// Verification proves a plan against ITSELF, never against the question it
	// answers. So being plan-backed buys no exemption from a misroute guard.
	backed := scoped("What does the case notes document say about plate ABC-123?")
	backed.SourceNative = &SourceNativePlanV1{ContractVersion: sourceNativePlanContractV1}
	if !structuredTemplateAnsweringMediaQuestion(backed, "canonical_records") {
		t.Error("a media question answered from a structured plan is still a misroute")
	}
}

// "How many CDR records came from each source file?" captured "?" as the file
// name — the trailing punctuation was the first non-space token after "source
// file". The generated plan was CORRECT (group by cdr.source_file, COUNT) and
// then ran against a filter matching nothing, so a right plan returned zero rows.
func TestSourceFileExtractionIgnoresPunctuationAndGroupings(t *testing.T) {
	if got := extractCanonicalSourceFile("How many CDR records came from each source file?"); got != "" {
		t.Errorf("a breakdown BY source file names no file; got %q", got)
	}
	for _, q := range []string{
		"records per source file?",
		"counts by source file",
		"every source file.",
	} {
		if got := extractCanonicalSourceFile(q); got != "" {
			t.Errorf("%q asks for a grouping, not a filter; got %q", q, got)
		}
	}
	// A real file name must still bind.
	for q, want := range map[string]string{
		"records from source file seed_cdr_large.csv": "seed_cdr_large.csv",
		`source file "923461678183.csv" only`:         "923461678183.csv",
	} {
		if got := extractCanonicalSourceFile(q); got != want {
			t.Errorf("%q -> %q, want %q", q, got, want)
		}
	}
}
