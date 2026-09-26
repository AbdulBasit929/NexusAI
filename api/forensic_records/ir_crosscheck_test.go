package main

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Cross-verification abstains when two independent derivations disagree.
// Measured on the 62: 12 wrong answers withheld, 2 correct ones withheld.
// The alternatives measured worse -- shape override 3/2 (and it CREATES
// confident-wrong), blanket withholding of unverified structured answers 12/9.

func TestCrossCheckIsOffByDefaultAndSwitchable(t *testing.T) {
	if semanticIRCrossCheckEnabled() {
		t.Fatal("cross-check must default to OFF; it costs a generation per question")
	}
	t.Setenv(semanticIRCrossCheckEnv, "true")
	if !semanticIRCrossCheckEnabled() {
		t.Fatal("FORENSIC_IR_CROSSCHECK=true must enable it")
	}
}

// The same number is the same answer, whatever its formatting.
func TestCrossCheckTreatsEqualNumbersAsAgreement(t *testing.T) {
	for _, tc := range []struct{ a, b any }{
		{8642, 8642.0}, {"8642", 8642}, {75000.0, 75000}, {0, 0.0},
	} {
		if !crossCheckValuesAgree(tc.a, tc.b) {
			t.Errorf("%v and %v are the same answer", tc.a, tc.b)
		}
	}
	// And a real difference is a real disagreement -- 9,200 vs 75,000 is the
	// lexicographic-MAX bug this product shipped for weeks.
	for _, tc := range []struct{ a, b any }{
		{9200, 75000}, {8642, 872}, {"149631808", "149631809"},
	} {
		if crossCheckValuesAgree(tc.a, tc.b) {
			t.Errorf("%v and %v must be a disagreement", tc.a, tc.b)
		}
	}
}

// A missing value is never agreement: silence must not corroborate.
func TestCrossCheckNeverAgreesWithNothing(t *testing.T) {
	if crossCheckValuesAgree(nil, 5) || crossCheckValuesAgree(5, nil) || crossCheckValuesAgree(nil, nil) {
		t.Fatal("a nil value must never count as corroboration")
	}
}

// It must not run where it cannot judge. Document, image and audio questions
// have their own governed executors and no typed plan to compare against;
// withholding them would suppress a competence this path does not have.
func TestCrossCheckSkipsWhereItCannotJudge(t *testing.T) {
	t.Setenv(semanticIRCrossCheckEnv, "true")
	var db *pgxpool.Pool // nil: no execution possible
	cfg := config{LocalAIURL: "http://127.0.0.1:1"}
	resp := hybridQueryResponse{Records: map[string]any{}, Answer: map[string]any{}}

	if out := crossCheckAgainstGeneratedPlan(context.Background(), cfg, db,
		hybridQueryRequest{Query: "Find OCR text mentioning Investigation Workspace"}, resp); out.Ran {
		t.Error("a media question has no typed plan to compare against")
	}
	// An answer already produced BY a verified plan needs no second opinion.
	planBacked := hybridQueryRequest{Query: "How many CDR records do we have?",
		SourceNative: &SourceNativePlanV1{ContractVersion: sourceNativePlanContractV1}}
	if out := crossCheckAgainstGeneratedPlan(context.Background(), cfg, db, planBacked, resp); out.Ran {
		t.Error("a plan-backed answer must not be compared with itself")
	}
	if out := crossCheckAgainstGeneratedPlan(context.Background(), cfg, db, planBacked, resp); out.Skipped != "already_plan_backed" {
		t.Errorf("skip reason must be recorded, got %q", out.Skipped)
	}
}

// Disabled, it must be inert.
func TestCrossCheckDisabledIsInert(t *testing.T) {
	out := crossCheckAgainstGeneratedPlan(context.Background(), config{LocalAIURL: "x"}, nil,
		hybridQueryRequest{Query: "How many CDR records do we have?"},
		hybridQueryResponse{Records: map[string]any{}, Answer: map[string]any{}})
	if out.Ran || out.Skipped != "disabled" {
		t.Fatalf("must be inert when disabled, got %+v", out)
	}
	// Enabled but with no database, the reason must say so rather than "disabled".
	t.Setenv(semanticIRCrossCheckEnv, "true")
	out = crossCheckAgainstGeneratedPlan(context.Background(), config{LocalAIURL: "x"}, nil,
		hybridQueryRequest{Query: "How many CDR records do we have in this case?", TenantID: "t", CollectionID: "c",
			QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)}},
		hybridQueryResponse{Records: map[string]any{}, Answer: map[string]any{}})
	if out.Ran {
		t.Fatalf("cannot run without a database, got %+v", out)
	}
}

// The typed-plan executor answers questions about STRUCTURED evidence. When a
// question names neither a family nor a record type it is not about structured
// rows, and answering anyway produced two confident wrong answers on
// 2026-09-22: an OCR-text question and a cross-family time range, both answered
// from canonical_records.
//
// Naming a record type is sufficient WITHOUT a family — "how many emails are in
// this case" resolves record_type=email, no family, and correctly answers zero.
// Requiring a family would withhold that correct answer.
func TestStructuredExecutorDeclinesQuestionsOutsideItsCompetence(t *testing.T) {
	outside := []string{
		"Which image says Stay Positive Work Hard?",
		"What time period does this case cover?",
		"Give me an overview of this case",
	}
	for _, question := range outside {
		if semanticQuestionFamily(question) != "" || extractCanonicalRecordType(question) != "" {
			t.Errorf("%q now resolves a family or record type; the guard no longer covers it", question)
		}
	}
	inside := map[string]string{
		"How many emails are in this case?":             "record type only, no family",
		"How many CDR records do we have in this case?": "family",
		"Which cell site handled the most calls?":       "family",
	}
	for question, why := range inside {
		if semanticQuestionFamily(question) == "" && extractCanonicalRecordType(question) == "" {
			t.Errorf("%q (%s) would be withheld; the guard is too broad", question, why)
		}
	}
}

// A date range is TWO values. Without a `range` goal the question fell through
// to "lookup" and returned a page of rows — measured 2026-09-22, "what date
// range do the CDR records cover" projected every field over 20 rows instead of
// bounding the period. The curated layer has always declared the capability:
// cdr.event_time's description says MIN/MAX of it answers exactly this.
func TestDateRangeQuestionBecomesMinAndMax(t *testing.T) {
	frame := extractSemanticFrame(hybridQueryRequest{Query: "What date range do the CDR records cover?"})
	if frame.Goal != "range" {
		t.Fatalf("goal = %q, want range — a lookup returns rows, not a period", frame.Goal)
	}
	// A superlative over periods stays a ranking; the range goal must not eat it.
	for _, q := range []string{
		"Which period had the most calls?",
		"Which camera recorded the most sightings?",
		"How many CDR records do we have in this case?",
	} {
		if g := extractSemanticFrame(hybridQueryRequest{Query: q}).Goal; g == "range" {
			t.Errorf("%q classified as range; the guard is too broad", q)
		}
	}
}

// The field is chosen from the ISSUED catalogue, never from the question text —
// resolving a measure field from the whole question is the D2 defect.
func TestRangeTimeFieldPrefersTheCuratedNormalisedTimestamp(t *testing.T) {
	catalog := []FieldDescriptorV1{
		{FieldID: "cdr.call_start", EffectiveType: fieldTypeTimestamp, Curated: true,
			AllowedAggregates: []string{"COUNT", "MIN", "MAX"}},
		{FieldID: "cdr.event_time", EffectiveType: fieldTypeTimestamp, Curated: true,
			AllowedAggregates: []string{"COUNT", "MIN", "MAX"}},
	}
	field, ok := sourceNativeRangeTimeField(catalog)
	if !ok || field.FieldID != "cdr.event_time" {
		t.Fatalf("range field = %q, want cdr.event_time (the normalised record timestamp)", field.FieldID)
	}
	// With nothing bounded, it must refuse rather than invent a field.
	if _, ok := sourceNativeRangeTimeField([]FieldDescriptorV1{
		{FieldID: "cdr.msisdn", EffectiveType: "STRING", Curated: true, AllowedAggregates: []string{"COUNT"}},
	}); ok {
		t.Fatal("a catalogue with no bounded time field must yield no range field")
	}
}

// Verified-only mode: a structured analytical question is answered from a typed
// plan that passed every check, or not at all. Measured on the 62: withholds 12
// wrong answers and 9 correct ones. Worse than cross-verification's projection,
// but cross-verification cannot judge template answers at all — they expose no
// single comparable value — so this is the mechanism that actually works.
func TestVerifiedOnlyIsOffByDefaultAndScoped(t *testing.T) {
	structured := hybridQueryRequest{Query: "How many CDR records do we have in this case?",
		TenantID: "t", CollectionID: "c", QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)}}
	if verifiedOnlyWithholds(structured) {
		t.Fatal("must default to OFF")
	}
	t.Setenv(semanticVerifiedOnlyEnv, "true")
	if !verifiedOnlyWithholds(structured) {
		t.Fatal("an unbacked structured answer must be withheld when enabled")
	}
	// A plan-backed answer has been verified and must pass through.
	backed := structured
	backed.SourceNative = &SourceNativePlanV1{ContractVersion: sourceNativePlanContractV1}
	if verifiedOnlyWithholds(backed) {
		t.Fatal("a plan-backed answer must never be withheld")
	}
	// Media and document questions have their own governed executors and no
	// typed plan exists for them; withholding them would suppress a competence
	// this mode does not judge.
	for _, q := range []string{
		"Find OCR text mentioning Investigation Workspace",
		"Search the audio transcripts for Japanese cuisine",
		"Which document mentions contact number 03001234567?",
	} {
		other := structured
		other.Query = q
		if verifiedOnlyWithholds(other) {
			t.Errorf("%q is not a structured analytical question; it must not be withheld", q)
		}
	}
}
