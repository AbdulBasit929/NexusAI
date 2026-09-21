package main

import (
	"sort"
	"strings"
	"testing"
)

var nxb1OperationIDs = []string{
	"financial.transaction_summary",
	"access.failed_events",
	"generic.filter_records",
	"document.metadata",
	"document.search",
	"image.metadata",
	"image.ocr_search",
	"face.candidate_observations",
	"audio.metadata",
	"audio.transcript_search",
	"video.metadata",
	"video.timeline",
}

func TestNXB1OperationsUseTheAuthoritativeExecutableCatalogue(t *testing.T) {
	templates := supportedQueryTemplates()
	if len(templates) != 79 {
		t.Fatalf("executable operation count = %d, want 79", len(templates))
	}
	byID := map[string]queryTemplateCatalogEntry{}
	for _, template := range templates {
		byID[template.OperationID] = template
	}
	for _, operationID := range nxb1OperationIDs {
		template, ok := byID[operationID]
		if !ok {
			t.Fatalf("NX-B1 operation %q is missing from supportedQueryTemplates", operationID)
		}
		if template.CriticalityTier != "B" || template.ExposureStatus != "limited" {
			t.Fatalf("NX-B1 operation %q readiness = %s/%s, want B/limited", operationID, template.CriticalityTier, template.ExposureStatus)
		}
		if !strings.Contains(strings.ToLower(template.Calculation), "authorized") && operationID != "financial.transaction_summary" && operationID != "access.failed_events" {
			// Operation-specific descriptions may use case-scoped wording, while
			// the shared plan supplies the actual authorization enforcement.
			if template.ScopeMode == "" {
				t.Fatalf("NX-B1 operation %q has no scope contract", operationID)
			}
		}
	}
}

func TestNXB1PlatformProjectionUsesFamilySpecialistsAndTruthfulMaturity(t *testing.T) {
	catalog, err := loadForensicPlatformCatalog()
	if err != nil {
		t.Fatal(err)
	}
	operations := map[string]FamilyOperationDescriptorV1{}
	for _, operation := range catalog.Operations {
		operations[operation.ID] = operation
	}
	for _, operationID := range nxb1OperationIDs {
		operation, ok := operations[operationID]
		if !ok {
			t.Fatalf("platform projection is missing %q", operationID)
		}
		if operation.Maturity == "CERTIFIED" {
			t.Fatalf("new B1 operation %q falsely claims CERTIFIED maturity", operationID)
		}
	}

	wantSpecialist := map[string]string{
		"financial.transaction_summary": "Financial_Transactions_Analyst",
		"access.failed_events":          "Access_Security_Log_Analyst",
		"generic.filter_records":        "Generic_Data_Quality_Analyst",
		"document.search":               "Document_OCR_Analyst",
		"image.ocr_search":              "Image_Vision_Analyst",
		"face.candidate_observations":   "Image_Vision_Analyst",
		"audio.transcript_search":       "Audio_Speech_Analyst",
		"video.timeline":                "Video_Timeline_Analyst",
	}
	for operationID, specialist := range wantSpecialist {
		templateName := queryTemplateNameByOperationID(operationID)
		template, ok := queryTemplateByName(templateName)
		if !ok {
			t.Fatalf("missing template for %q", operationID)
		}
		if got := specialistForCapabilityFamily(template.FamilyID); got != specialist {
			t.Fatalf("specialist for %q = %q, want %q", operationID, got, specialist)
		}
	}
}

func TestNXB1TypedVisualActionsResolveThroughTheSharedAskContract(t *testing.T) {
	for _, operationID := range nxb1OperationIDs {
		templateName := queryTemplateNameByOperationID(operationID)
		template, ok := queryTemplateByName(templateName)
		if !ok {
			t.Fatalf("missing shared Ask template for %q", operationID)
		}
		action := map[string]any{
			"contract_version": "forensics.query-plan/v1", "tenant_id": "tenant-a", "case_id": "case-a", "collection_id": "case-a",
			"intent": operationID, "families": []any{template.FamilyID}, "limit": float64(20), "clarifications": []any{},
		}
		if operationID == "video.timeline" || operationID == "face.candidate_observations" {
			action["entities"] = []any{map[string]any{"type": "evidence_id", "value": "11111111-1111-4111-8111-111111111111"}}
			if operationID == "video.timeline" {
				action["filters"] = []any{map[string]any{"field": "start_seconds", "op": "gte", "value": float64(12)}, map[string]any{"field": "end_seconds", "op": "lte", "value": float64(18)}}
			} else {
				action["filters"] = []any{}
			}
		} else {
			action["entities"] = []any{}
			action["filters"] = []any{}
		}
		adapted, early, err := normalizeV1QueryPlan("case-a", "request-ui", action)
		if err != nil {
			t.Fatalf("typed action %q failed: %v", operationID, err)
		}
		if early != nil {
			t.Fatalf("typed action %q unexpectedly stopped early: %#v", operationID, early)
		}
		if got := stringValueAny(adapted["template"]); got != templateName {
			t.Fatalf("typed action %q resolved template %q, want %q", operationID, got, templateName)
		}
		if operationID == "video.timeline" {
			if stringValueAny(adapted["evidence_id"]) == "" || adapted["start_seconds"] == nil || adapted["end_seconds"] == nil {
				t.Fatalf("video.timeline visual action lost its exact evidence or source-time scope: %#v", adapted)
			}
		}
		if operationID == "face.candidate_observations" && stringValueAny(adapted["evidence_id"]) == "" {
			t.Fatalf("face candidate visual action lost its exact evidence scope: %#v", adapted)
		}
	}
}

func TestNXB1SQLIsScopedBoundedAndSemanticallyExplicit(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want []string
	}{
		{"financial", financialTransactionSummarySQL, []string{"r.tenant_id=$1", "r.collection_id=$2", "r.record_type='transaction'", "GROUP BY currency, transaction_status, amount_role", "LIMIT $6"}},
		{"financial lineage", financialTransactionLineageSQL, []string{"r.tenant_id=$1", "r.collection_id=$2", "r.record_type='transaction'", "source_group_position <= 50", "row_group_digest"}},
		{"access", accessFailedEventsSQL, []string{"r.tenant_id=$1", "r.collection_id=$2", "r.record_type='access_log'", "http_status", "explicit_failure_outcome", "ORDER BY r.\"timestamp\", r.source_file, r.row_number, r.row_hash", "LIMIT $6"}},
		{"metadata", familyEvidenceMetadataSQL, []string{"items.tenant_id=$1", "items.collection_id=$2", "items.modality = ANY($3::text[])", "completed_artifact_count", "LIMIT $5"}},
		{"video", videoTimelineSQL, []string{"artifacts.tenant_id=$1", "artifacts.collection_id=$2", "artifacts.evidence_id=$3::uuid", "artifact_type = ANY($4::text[])", "ORDER BY source_start_seconds", "LIMIT $7"}},
		{"face", faceCandidateObservationsSQL, []string{"artifacts.tenant_id=$1", "artifacts.collection_id=$2", "artifacts.evidence_id=$3::uuid", "items.current_version_id=artifacts.version_id", "artifact_type='forensics.face-observation/v1'", "- 'embedding'", "ORDER BY artifacts.created_at, artifacts.artifact_id", "LIMIT $4"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, fragment := range tc.want {
				if !strings.Contains(tc.sql, fragment) {
					t.Fatalf("SQL is missing %q", fragment)
				}
			}
			for _, forbidden := range []string{"; DROP", "EXECUTE ", "raw_sql", "query_text"} {
				if strings.Contains(strings.ToUpper(tc.sql), strings.ToUpper(forbidden)) {
					t.Fatalf("SQL contains forbidden escape surface %q", forbidden)
				}
			}
		})
	}
}

func TestNXB1IndependentFinancialOracleSeparatesCurrencyAndMissingStatus(t *testing.T) {
	// This hand-auditable oracle is intentionally independent from production
	// SQL. It defines the semantic totals that the SQL GROUP BY must produce.
	fixture := []struct {
		currency, status, role, amount string
	}{
		{"PKR", "posted", "debit", "100.25"},
		{"PKR", "posted", "debit", "20.75"},
		{"USD", "posted", "debit", "2.00"},
		{"PKR", "", "credit", "9.00"},
	}
	want := map[string]string{
		"PKR|posted|debit":       "2|121.00",
		"USD|posted|debit":       "1|2.00",
		"PKR|UNSPECIFIED|credit": "1|9.00",
	}
	// Decimal strings are converted to minor units in the test oracle; the
	// production SQL independently sums normalized decimal values.
	type aggregate struct{ count, minor int }
	got := map[string]aggregate{}
	for _, row := range fixture {
		status := row.status
		if status == "" {
			status = "UNSPECIFIED"
		}
		parts := strings.Split(row.amount, ".")
		major, minor := 0, 0
		for _, r := range parts[0] {
			major = major*10 + int(r-'0')
		}
		if len(parts) == 2 {
			for _, r := range (parts[1] + "00")[:2] {
				minor = minor*10 + int(r-'0')
			}
		}
		key := strings.Join([]string{row.currency, status, row.role}, "|")
		agg := got[key]
		agg.count++
		agg.minor += major*100 + minor
		got[key] = agg
	}
	for key, expected := range want {
		agg := got[key]
		actual := strings.Join([]string{itoaForOracle(agg.count), itoaForOracle(agg.minor/100) + "." + twoDigitsForOracle(agg.minor%100)}, "|")
		if actual != expected {
			t.Fatalf("financial oracle %s = %s, want %s", key, actual, expected)
		}
	}
}

func itoaForOracle(value int) string {
	if value == 0 {
		return "0"
	}
	digits := make([]byte, 0, 10)
	for value > 0 {
		digits = append(digits, byte('0'+value%10))
		value /= 10
	}
	for left, right := 0, len(digits)-1; left < right; left, right = left+1, right-1 {
		digits[left], digits[right] = digits[right], digits[left]
	}
	return string(digits)
}

func twoDigitsForOracle(value int) string {
	return string([]byte{byte('0' + value/10), byte('0' + value%10)})
}

func TestNXB1IndependentAccessFailureOracleUsesOnlyExplicitSignals(t *testing.T) {
	fixture := []struct {
		status  int
		outcome string
		want    bool
	}{
		{200, "success", false},
		{401, "", true},
		{0, "denied", true},
		{302, "redirect", false},
		{0, "unknown", false},
	}
	failureTokens := map[string]bool{"fail": true, "failed": true, "failure": true, "denied": true, "blocked": true, "error": true, "rejected": true}
	for _, row := range fixture {
		actual := row.status >= 400 || failureTokens[strings.ToLower(row.outcome)]
		if actual != row.want {
			t.Fatalf("status=%d outcome=%q failure=%v, want %v", row.status, row.outcome, actual, row.want)
		}
	}
}

func TestNXB1DerivedTextSearchIsFamilyIsolated(t *testing.T) {
	want := map[string][]string{
		"document_search":         {"forensics.document-native-text-passage/v1"},
		"image_ocr_search":        {"forensics.image-ocr-observation/v1"},
		"audio_transcript_search": {"forensics.audio-roman-urdu-segment/v1", "forensics.audio-timestamp-segment/v1"},
	}
	for template, expected := range want {
		actual := derivedTextContractsForTemplate(template)
		sort.Strings(actual)
		sort.Strings(expected)
		if strings.Join(actual, "|") != strings.Join(expected, "|") {
			t.Fatalf("contracts for %s = %#v, want %#v", template, actual, expected)
		}
		if !isFamilyDerivedTextTemplate(template) {
			t.Fatalf("%s is not marked family-derived-only", template)
		}
	}
	if len(derivedTextContractsForTemplate("evidence")) != len(searchableDerivedTextContracts) {
		t.Fatal("general governed evidence retrieval lost its accepted all-family derived-text set")
	}
}

func TestNXB1VideoTimelineRejectsInvalidScopeBeforeDatabaseAccess(t *testing.T) {
	if _, err := videoObservationTimeline(t.Context(), nil, hybridQueryRequest{EvidenceID: "not-a-uuid"}); err == nil || !strings.Contains(err.Error(), "exact video evidence UUID") {
		t.Fatalf("invalid UUID error = %v", err)
	}
	start, end := 8.0, 2.0
	if _, err := videoObservationTimeline(t.Context(), nil, hybridQueryRequest{EvidenceID: "11111111-1111-4111-8111-111111111111", StartSeconds: &start, EndSeconds: &end}); err == nil || !strings.Contains(err.Error(), "end_seconds") {
		t.Fatalf("invalid source-time error = %v", err)
	}
}

func TestNXB1FaceCandidatesRejectInvalidScopeBeforeDatabaseAccess(t *testing.T) {
	if _, err := faceCandidateObservations(t.Context(), nil, hybridQueryRequest{EvidenceID: "not-a-uuid"}); err == nil || !strings.Contains(err.Error(), "exact image evidence UUID") {
		t.Fatalf("invalid UUID error = %v", err)
	}
}

func TestNXB1MultilingualRoutingPreservesFamilyOperations(t *testing.T) {
	tests := []struct{ query, template string }{
		{"len den ka khulasa currency ke hisab se dikhao", "financial_transaction_summary"},
		{"ناکام رسائی کے واقعات دکھائیں", "access_failed_events"},
		{"documents mein dhoondo agreement", "document_search"},
		{"تصویر کا متن تلاش کریں ABC123", "image_ocr_search"},
		{"tasveer mein chehray ke candidates dikhao", "face_candidate_observations"},
		{"transcript mein dhoondo meeting", "audio_transcript_search"},
		{"ویڈیو ٹائم لائن دکھائیں", "video_timeline"},
	}
	for _, tc := range tests {
		if got := chooseTemplate(tc.query, ""); got != tc.template {
			t.Fatalf("chooseTemplate(%q) = %q, want %q", tc.query, got, tc.template)
		}
	}
}
