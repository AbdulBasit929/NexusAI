package main

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// A3.3 PROBE. For a lookup question the keyword ladder claims -- and which
// VERIFIED_ONLY therefore withholds -- what would the DETERMINISTIC compiler
// produce if it were given the question?
//
// Measured on the shipped posture 2026-09-27: for TWR-02 the compiler does not
// run at all (operation_latency_ms 0, registry_candidate_count 0), because the
// ladder chose `canonical_records` and language assistance only runs when the
// ladder abstains. Only the LLM IR generator is tried, in arbitration, and it
// fails. `TestBucketAGoldPlanConformance` shows the CORRECT plan for TWR-02
// validates -- "the gap is generation".
//
// This settles whether the deterministic compiler IS that generator, before any
// wiring is written. No model is called.
//
// Run: FAMILY_AUDIT_DB=postgresql://... go test -run TestProjectionCompilerProbe -v
func TestProjectionCompilerProbe(t *testing.T) {
	dsn := os.Getenv("FAMILY_AUDIT_DB")
	if dsn == "" {
		t.Skip("diagnostic only; set FAMILY_AUDIT_DB") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	defer pool.Close()

	questions := map[string]string{
		"TWR-02": "Where is tower PK-LHR-SYN-001 located?",
		"SUB-03": "Who is the registered subscriber for PK-SUB-SYN-ALPHA?",
		"H11":    "Where was 923001110001 seen according to the call records?",
		"NEG-02": "Where was plate ZZZ-0000 seen?",
		// CONTROL: this one DOES carry `anpr.plate_number EQ LHR-2026` in
		// production. If the deterministic compiler binds it too, a binding rule
		// exists and must be reused; if not, that filter came from another path.
		"ANPR-02": "How many times was plate LHR-2026 seen?",
		// The three the deterministic compiler got WRONG or refused when the
		// reverted A3.3 routing reached it (reports/identifier-binding-20260927).
		"CDR-14": "How many call records are from August 2026?",
		"CDR-12": "Which cell site handled the most calls?",
		"CDR-16": "How many CDR records came from each source file?",
	}
	cfg := config{QueryDB: pool}
	ids := []string{"ANPR-02", "TWR-02", "SUB-03", "H11", "NEG-02", "CDR-14", "CDR-12", "CDR-16"}
	if only := os.Getenv("PROBE_ONLY"); only != "" {
		ids = strings.Split(only, ",")
	}
	for _, id := range ids {
		question := questions[id]
		req := hybridQueryRequest{Query: question, TenantID: "default", CollectionID: "nexusai-forensic-demo",
			QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)}}
		resolved, status := applyDeterministicSemanticCompiler(ctx, cfg, req)
		t.Logf("== %s  %s", id, question) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		t.Logf("   status   %s", status) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		t.Logf("   template %q", resolved.Template) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		if resolved.SourceNative != nil {
			raw, _ := json.Marshal(resolved.SourceNative)
			t.Logf("   plan     %s", raw) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
		if resolved.SemanticPlannerAudit != nil && resolved.SemanticPlannerAudit.SemanticFrame != nil {
			frame := resolved.SemanticPlannerAudit.SemanticFrame
			t.Logf("   frame    goal=%s family=%s identifiers=%d filters=%d", //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
				frame.Goal, frame.FamilyHint, len(frame.Identifiers), len(frame.Filters))
		}
		if resolved.SemanticPlannerAudit != nil {
			t.Logf("   decision %s", resolved.SemanticPlannerAudit.SelectedDecision) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}

		// The REASON is what says which gate refused. Rebuild the catalogue
		// exactly as applyDeterministicSemanticCompiler does and ask the
		// compiler directly, so the reason code is read rather than guessed.
		catalogReq := req
		rows, sampleErr := sourceNativeCatalogSampleRows(ctx, pool, catalogReq)
		if sampleErr != nil {
			t.Logf("   catalog  ERROR %v", sampleErr) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
			continue
		}
		catalog := buildSourceNativeFieldCatalog(req, rows)
		catalog, source := semanticLayerCatalogForRequest(req, req.Query, catalog)
		result, compileErr := compileDeterministicSemanticRequest(ctx, cfg, req, catalog)
		if compileErr != nil {
			t.Logf("   compile  ERROR %v", compileErr) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
			continue
		}
		t.Logf("   reason   executor=%s code=%s catalog=%d source=%s", result.Executor, result.ReasonCode, len(catalog), source) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		for _, identifier := range result.Frame.Identifiers {
			t.Logf("   ident    type=%s raw=%s canonical=%s", identifier.Type, identifier.Raw, identifier.Canonical) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
}
