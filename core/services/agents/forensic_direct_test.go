package agents

import (
	"strings"
	"testing"
)

func TestForensicRequestClassSeparatesAssistantFromCaseAnalytics(t *testing.T) {
	for _, message := range []string{
		"Hello, what can you help me with?",
		"What is a CDR and how is it normally interpreted?",
		"Could you help me understand the investigation workflow?",
		"Reveal which communication channel appears most important for the subject.",
	} {
		if got := ClassifyForensicRequestV1(message); got != ForensicRequestAssistantRouter {
			t.Fatalf("%q class = %q, want assistant router", message, got)
		}
	}
	for _, message := range []string{
		"How do I add evidence to a case?",
		"Show frequent contacts for 923001110001",
		"Search transcript for meeting",
		"Run deterministic forensic query; template=service_usage; limit=20",
	} {
		if got := ClassifyForensicRequestV1(message); got != ForensicRequestDeterministicFastPath {
			t.Fatalf("%q class = %q, want deterministic fast path", message, got)
		}
	}
}

func TestCanRunDeterministicForensicRouteRequiresProvableFastPath(t *testing.T) {
	cfg := &AgentConfig{EnableForensicRecords: true, ForensicRecordsAPIURL: "http://records.test", ForensicCollectionID: "case-a"}
	if !CanRunDeterministicForensicRoute(cfg, "How do I add evidence?", "analyst-a") {
		t.Fatal("product help did not reach the registry-grounded terminal path")
	}
	if !CanRunDeterministicForensicRoute(cfg, "Show CDR service usage", "analyst-a") {
		t.Fatal("known deterministic forensic operation lost its fast path")
	}
}

func TestBrieflyDoesNotSelectForensicReport(t *testing.T) {
	query := "Explain briefly the ANPR sightings for plate ABC-123 using only supplied observations."
	if isNaturalForensicReportQuery(query, strings.ToLower(query)) {
		t.Fatal("adverb briefly must not be interpreted as a request for a forensic brief")
	}
	if got := forensicTemplateHint(query); got != "anpr_sightings" {
		t.Fatalf("forensicTemplateHint(%q) = %q, want anpr_sightings", query, got)
	}
}

func TestGroupedVideoUUIDIsNotExtractedAsPlateParameter(t *testing.T) {
	query := "اس ویڈیو میں کون سی نمبر پلیٹیں ہیں evidence 50057921-4f1f-4ab8-bab0-47bcdc957822"
	args := buildForensicHybridQueryArgs(query, ForensicRecordsToolConfig{})

	if args.Template != "video_anpr_grouped_timeline" {
		t.Fatalf("template = %q, want video_anpr_grouped_timeline", args.Template)
	}
	if args.EvidenceID != "50057921-4f1f-4ab8-bab0-47bcdc957822" {
		t.Fatalf("evidence_id = %q, want the complete source UUID", args.EvidenceID)
	}
	if args.Target != args.EvidenceID {
		t.Fatalf("target = %q, want evidence UUID %q", args.Target, args.EvidenceID)
	}
	if args.Plate != "" {
		t.Fatalf("plate = %q, want no plate filter derived from the UUID", args.Plate)
	}
	if got := extractForensicTarget(query); got != "" {
		t.Fatalf("extractForensicTarget(query) = %q, want no analytical target", got)
	}
}

func TestGroupedVideoExplicitPlateRemainsSeparateFromEvidenceUUID(t *testing.T) {
	query := "show plates in this video for plate MN1367 evidence 50057921-4f1f-4ab8-bab0-47bcdc957822"
	args := buildForensicHybridQueryArgs(query, ForensicRecordsToolConfig{})

	if args.Template != "video_anpr_grouped_timeline" {
		t.Fatalf("template = %q, want video_anpr_grouped_timeline", args.Template)
	}
	if args.Target != "50057921-4f1f-4ab8-bab0-47bcdc957822" {
		t.Fatalf("target = %q, want the complete evidence UUID", args.Target)
	}
	if args.Plate != "MN1367" {
		t.Fatalf("plate = %q, want explicit plate MN1367", args.Plate)
	}
}

func TestNaturalGroupedVideoRoutingPreservesTypedOperationParameters(t *testing.T) {
	const evidenceID = "50057921-4f1f-4ab8-bab0-47bcdc957822"
	tests := []struct {
		name      string
		query     string
		wantPlate string
		wantStart *float64
		wantEnd   *float64
	}{
		{name: "video evidence", query: "Show plates in video evidence " + evidenceID},
		{name: "natural grouped timeline", query: "Show the grouped ANPR timeline for retained video evidence " + evidenceID},
		{name: "detected plates", query: "Which plates were detected in video " + evidenceID + "?"},
		{name: "retained video detections", query: "Show plate detections for this retained video " + evidenceID},
		{name: "UUID and plate", query: "Show grouped video ANPR for evidence " + evidenceID + " and plate MN1367", wantPlate: "MN1367"},
		{name: "UUID and seconds", query: "Show grouped video ANPR for evidence " + evidenceID + " between 10 and 30 seconds", wantStart: float64Pointer(10), wantEnd: float64Pointer(30)},
		{name: "UUID plate and seconds", query: "Show grouped video ANPR for evidence " + evidenceID + " and plate MN1367 between 10 and 30 seconds", wantPlate: "MN1367", wantStart: float64Pointer(10), wantEnd: float64Pointer(30)},
		{name: "Roman Urdu", query: "is retained video " + evidenceID + " ki grouped ANPR timeline dikhao"},
		{name: "Urdu", query: "اس ویڈیو " + evidenceID + " کی نمبر پلیٹ ٹائم لائن دکھائیں"},
	}
	toolConfig := ForensicRecordsToolConfig{TenantID: "tenant-test", UserID: "analyst-test", CollectionID: "case-test"}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			args := toolConfig.defaultsHybridArgs(buildForensicHybridQueryArgs(tc.query, toolConfig))
			if args.Template != "video_anpr_grouped_timeline" || args.EvidenceID != evidenceID || args.Target != evidenceID {
				t.Fatalf("typed operation identity = template %q evidence %q target %q", args.Template, args.EvidenceID, args.Target)
			}
			if args.Plate != tc.wantPlate {
				t.Fatalf("plate = %q, want %q", args.Plate, tc.wantPlate)
			}
			if !equalOptionalFloat(args.StartSeconds, tc.wantStart) || !equalOptionalFloat(args.EndSeconds, tc.wantEnd) {
				t.Fatalf("source-second range = %v..%v, want %v..%v", args.StartSeconds, args.EndSeconds, tc.wantStart, tc.wantEnd)
			}
			if args.TenantID != "tenant-test" || args.UserID != "analyst-test" || args.CollectionID != "case-test" {
				t.Fatalf("authorized scope changed: tenant=%q user=%q collection=%q", args.TenantID, args.UserID, args.CollectionID)
			}
		})
	}
}

func float64Pointer(value float64) *float64 { return &value }

func equalOptionalFloat(left, right *float64) bool {
	return (left == nil && right == nil) || (left != nil && right != nil && *left == *right)
}

func TestDeterministicForensicRouteCoverage(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		template string
	}{
		{name: "source audit", query: "which files were ingested?", template: "source_file_audit"},
		{name: "case readiness", query: "is this case ready for production?", template: "case_readiness"},
		{name: "CDR calculation", query: "show CDR call type breakdown", template: "call_type_breakdown"},
		{name: "CDR screenshot wording", query: "How are the CDR events divided by call type?", template: "call_type_breakdown"},
		{name: "IPDR calculation", query: "show network protocol breakdown", template: "ipdr_protocol_breakdown"},
		{name: "ANPR calculation", query: "show ANPR camera activity", template: "anpr_camera_activity"},
		{name: "Urdu ANPR sighting", query: "نمبر پلیٹ PKX4821 کب نظر آئی دکھائیں", template: "anpr_sightings"},
		{name: "Urdu plate sighting without plate noun", query: "MN1367 کہاں نظر آئی؟", template: "anpr_sightings"},
		{name: "Urdu plate found in case", query: "اس کیس میں MN1367 کہاں ملی؟", template: "anpr_sightings"},
		{name: "Urdu image ANPR", query: "اس تصویر میں کون سی نمبر پلیٹ ہے؟", template: "anpr_sightings"},
		{name: "Urdu grouped video ANPR", query: "اس ویڈیو میں کون سی نمبر پلیٹیں ہیں evidence 50057921-4f1f-4ab8-bab0-47bcdc957822", template: "video_anpr_grouped_timeline"},
		{name: "CDR service usage", query: "show CDR service usage", template: "service_usage"},
		{name: "CDR device changes", query: "show IMEI changes for 923001234567", template: "device_identity_changes"},
		{name: "geospatial movement", query: "show geospatial movement for ABC-123", template: "geospatial_movement"},
		{name: "collection overview", query: "show collection overview", template: "collection_overview"},
		{name: "case overview screenshot wording", query: "Show me an overview of this case", template: "collection_overview"},
		{name: "cross-family screenshot wording", query: "Correlate 923001110001 across the available evidence families", template: "cross_family_correlation"},
		{name: "timeline screenshot wording", query: "What happened involving 923001110001 over time?", template: "entity_timeline"},
		{name: "schema profile", query: "show detected fields and schema", template: "schema_profile"},
		{name: "limitations", query: "what limitations and missing data exist?", template: "limitations_and_data_quality"},
		{name: "subscriber lookup", query: "find subscriber identity for 923001234567", template: "subscriber_identity_lookup"},
		{name: "subscriber validity", query: "show subscriber validity timeline for 923001234567", template: "subscriber_validity_timeline"},
		{name: "subscriber device links", query: "show subscriber SIM and device links for 923001234567", template: "subscriber_device_links"},
		{name: "subscriber status", query: "show subscriber status summary", template: "subscriber_status_summary"},
		{name: "subscriber conflicts", query: "audit subscriber identity conflicts", template: "subscriber_conflict_audit"},
		{name: "subscriber reuse", query: "show subscriber identifier reuse candidates", template: "subscriber_reuse_candidates"},
		{name: "tower lookup", query: "look up tower site PK-LHR-SYN-001", template: "tower_site_lookup"},
		{name: "tower slash lookup", query: "Look up tower/site reference observations; target=PK-LHR-SYN-001; limit=2", template: "tower_site_lookup"},
		{name: "tower history", query: "show tower reference history for PK-LHR-SYN-001", template: "tower_reference_timeline"},
		{name: "tower coordinates", query: "audit tower coordinates and uncertainty", template: "tower_coordinate_audit"},
		{name: "tower status", query: "show tower status summary", template: "tower_status_summary"},
		{name: "tower conflicts", query: "show tower alias conflicts", template: "tower_alias_conflicts"},
		{name: "tower CDR join", query: "run a time-aware tower join for PK-LHR-SYN-001", template: "tower_cdr_join"},
		{name: "Roman Urdu subscriber lookup", query: "subscriber ki tafseel 0300-1234567 ke liye dikhao", template: "subscriber_identity_lookup"},
		{name: "Urdu subscriber lookup", query: "اس نمبر 0300-1234567 کی سبسکرائبر کی تفصیل دکھائیں", template: "subscriber_identity_lookup"},
		{name: "Roman Urdu subscriber validity", query: "03001234567 kab active hua aur kab band hua", template: "subscriber_validity_timeline"},
		{name: "Urdu subscriber status", query: "فعال اور معطل سبسکرائبرز کی حیثیت دکھائیں", template: "subscriber_status_summary"},
		{name: "Roman Urdu subscriber conflicts", query: "subscriber record mein ikhtilaf dikhao", template: "subscriber_conflict_audit"},
		{name: "Urdu subscriber reuse", query: "شناختی نمبر کا دوبارہ استعمال دکھائیں", template: "subscriber_reuse_candidates"},
		{name: "financial breadth", query: "show transaction totals by currency", template: "financial_transaction_summary"},
		{name: "access breadth", query: "show failed access events", template: "access_failed_events"},
		{name: "document breadth", query: "documents mein dhoondo agreement", template: "document_search"},
		{name: "image breadth", query: "tasveer ka matn dhoondo ABC123", template: "image_ocr_search"},
		{name: "face breadth", query: "tasveer mein chehray ke candidates dikhao", template: "face_candidate_observations"},
		{name: "audio breadth", query: "transcript mein dhoondo meeting", template: "audio_transcript_search"},
		{name: "video breadth", query: "video ka timeline dikhao", template: "video_timeline"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !deterministicForensicRouteRequested(tt.query) {
				t.Fatalf("query %q would fall through to the model instead of the deterministic forensic route", tt.query)
			}
			if got := forensicTemplateHint(tt.query); got != tt.template {
				t.Fatalf("forensicTemplateHint(%q) = %q, want %q", tt.query, got, tt.template)
			}
			if got := forensicSynthesisModel(tt.query, "local-model"); got != "local-model" {
				t.Fatalf("natural query selected synthesis model %q, want local-model", got)
			}
		})
	}
}

func TestAllAcceptedForensicTemplatesAreExplicitlyRoutableFromAgentChat(t *testing.T) {
	templates := []string{
		"collection_overview", "frequent_contacts", "call_type_breakdown", "service_usage",
		"device_identity_changes", "ipdr_endpoint_summary", "ipdr_domain_summary",
		"ipdr_protocol_breakdown", "ipdr_session_volume", "ipdr_subscriber_sessions",
		"ipdr_concurrent_sessions", "ipdr_timeline", "temporal_activity", "top_locations",
		"geospatial_movement", "anpr_sightings", "anpr_camera_sequence", "anpr_camera_activity",
		"anpr_co_travel", "anpr_route_timing", "anpr_plate_variants", "anpr_timeline", "video_anpr_grouped_timeline",
		"subscriber_identity_lookup", "subscriber_validity_timeline", "subscriber_device_links",
		"subscriber_status_summary", "subscriber_conflict_audit", "subscriber_reuse_candidates",
		"entity_activity", "relationship_network", "cross_family_correlation", "entity_timeline",
		"source_records", "canonical_records", "schema_profile", "data_quality", "evidence",
		"shortest_call", "longest_call", "duration_extremes", "first_seen_last_seen",
		"activity_by_day", "activity_by_hour", "night_activity", "repeated_location_visits",
		"co_travel_or_co_presence", "subscriber_profile", "imei_imsi_usage", "tower_activity",
		"suspicious_patterns", "anomaly_summary", "cross_dataset_entity_summary",
		"source_file_audit", "duplicate_upload_audit", "case_readiness",
		"evidence_package_summary", "executive_case_brief", "court_ready_source_summary",
		"limitations_and_data_quality", "tower_site_lookup", "tower_reference_timeline",
		"tower_coordinate_audit", "tower_status_summary", "tower_alias_conflicts", "tower_cdr_join",
		"financial_transaction_summary", "access_failed_events", "generic_filter_records",
		"document_metadata", "document_search", "image_metadata", "image_ocr_search", "face_candidate_observations",
		"audio_metadata", "audio_transcript_search", "video_metadata", "video_timeline",
	}
	if len(templates) != 78 {
		t.Fatalf("accepted Agent Chat template fixture contains %d templates, want 78", len(templates))
	}
	for _, template := range templates {
		t.Run(template, func(t *testing.T) {
			query := "Run deterministic forensic query; template=" + template + "; limit=20"
			if !deterministicForensicRouteRequested(query) {
				t.Fatalf("explicit template %q did not select the deterministic route", template)
			}
			if got := forensicTemplateHint(query); got != template {
				t.Fatalf("forensicTemplateHint(%q) = %q, want %q", query, got, template)
			}
			if got := forensicSynthesisModel(query, "local-model"); got != "" {
				t.Fatalf("exact query unexpectedly selected synthesis model %q", got)
			}
		})
	}
}

func TestAllAcceptedForensicTemplatesHaveNaturalAgentChatQuestions(t *testing.T) {
	tests := []struct {
		query    string
		template string
	}{
		{"Show the collection overview", "collection_overview"},
		{"Who are the frequent contacts?", "frequent_contacts"},
		{"Show the CDR call type breakdown", "call_type_breakdown"},
		{"Show CDR service usage", "service_usage"},
		{"Show IMEI changes for 923001110001", "device_identity_changes"},
		{"Summarize IPDR endpoints", "ipdr_endpoint_summary"},
		{"Show the IPDR domain summary", "ipdr_domain_summary"},
		{"Show the network protocol breakdown", "ipdr_protocol_breakdown"},
		{"Show network session volume", "ipdr_session_volume"},
		{"List subscriber network sessions for 923001110001", "ipdr_subscriber_sessions"},
		{"Find concurrent sessions for 923001110001", "ipdr_concurrent_sessions"},
		{"Build the IPDR timeline for 923001110001", "ipdr_timeline"},
		{"Show temporal CDR activity for 923001110001", "temporal_activity"},
		{"Show top locations for 923001110001", "top_locations"},
		{"Show geospatial movement for 923001110001", "geospatial_movement"},
		{"List ANPR sightings for ABC-123", "anpr_sightings"},
		{"Show the camera sequence for ABC-123", "anpr_camera_sequence"},
		{"Show ANPR camera activity", "anpr_camera_activity"},
		{"Find same-camera co-travel observations for ABC-123", "anpr_co_travel"},
		{"Show route timing for ABC-123", "anpr_route_timing"},
		{"Show plate variants for ABC-123", "anpr_plate_variants"},
		{"Build the ANPR timeline for ABC-123", "anpr_timeline"},
		{"Look up subscriber identity for 923000000001", "subscriber_identity_lookup"},
		{"Show subscriber validity for PK-SUB-SYN-ALPHA", "subscriber_validity_timeline"},
		{"Show subscriber device links for 923000000001", "subscriber_device_links"},
		{"Show subscriber status", "subscriber_status_summary"},
		{"Audit subscriber conflicts", "subscriber_conflict_audit"},
		{"Show subscriber reuse candidates", "subscriber_reuse_candidates"},
		{"Show entity activity for 923001110001", "entity_activity"},
		{"Build a relationship network for 923001110001", "relationship_network"},
		{"Run cross-family correlation for 923001110001", "cross_family_correlation"},
		{"What happened on 2026-07-14 for 923001110001?", "entity_timeline"},
		{"Show source records for 923001110001", "source_records"},
		{"Show canonical records where record type equals cdr", "canonical_records"},
		{"Show the detected schema", "schema_profile"},
		{"Show data quality issues", "data_quality"},
		{"Show forensic evidence lineage", "evidence"},
		{"Show the shortest call", "shortest_call"},
		{"Show the longest call", "longest_call"},
		{"Show call duration extremes", "duration_extremes"},
		{"When was 923001110001 first and last observed?", "first_seen_last_seen"},
		{"Show activity by day for 923001110001", "activity_by_day"},
		{"Show activity by hour for 923001110001", "activity_by_hour"},
		{"Show night activity for 923001110001", "night_activity"},
		{"Show repeated location visits for 923001110001", "repeated_location_visits"},
		{"Find cross-family co-presence for 923001110001", "co_travel_or_co_presence"},
		{"Show the CDR subscriber profile for 923001110001", "subscriber_profile"},
		{"Show IMEI IMSI usage for 923001110001", "imei_imsi_usage"},
		{"Show tower activity for 923001110001", "tower_activity"},
		{"Show suspicious patterns", "suspicious_patterns"},
		{"Show the anomaly summary", "anomaly_summary"},
		{"Show the entity across datasets for 923001110001", "cross_dataset_entity_summary"},
		{"Which source files were ingested?", "source_file_audit"},
		{"Audit duplicate uploads", "duplicate_upload_audit"},
		{"Is this case ready for production?", "case_readiness"},
		{"Show the evidence package", "evidence_package_summary"},
		{"Prepare an executive brief", "executive_case_brief"},
		{"Prepare a court-ready source summary", "court_ready_source_summary"},
		{"What limitations and missing data exist?", "limitations_and_data_quality"},
		{"Look up tower site PK-LHR-SYN-001", "tower_site_lookup"},
		{"Show tower reference history for PK-LHR-SYN-001", "tower_reference_timeline"},
		{"Audit tower coordinates and uncertainty", "tower_coordinate_audit"},
		{"Show tower status", "tower_status_summary"},
		{"Show tower alias conflicts", "tower_alias_conflicts"},
		{"Run a time-aware tower join for PK-LHR-SYN-001", "tower_cdr_join"},
		{"Show transaction totals by currency", "financial_transaction_summary"},
		{"Show failed access events", "access_failed_events"},
		{"Show generic structured rows", "generic_filter_records"},
		{"Show registered document metadata", "document_metadata"},
		{"Search documents for agreement", "document_search"},
		{"Show image processing status", "image_metadata"},
		{"Search image OCR for ABC123", "image_ocr_search"},
		{"Show face candidate observations", "face_candidate_observations"},
		{"Show registered audio metadata", "audio_metadata"},
		{"Search transcript for meeting", "audio_transcript_search"},
		{"Show registered video metadata", "video_metadata"},
		{"Show the video observation timeline", "video_timeline"},
	}
	if len(tests) != 77 {
		t.Fatalf("natural Agent Chat fixture contains %d questions, want 77", len(tests))
	}
	for _, tt := range tests {
		t.Run(tt.template, func(t *testing.T) {
			if !deterministicForensicRouteRequested(tt.query) {
				t.Fatalf("natural question %q did not select the deterministic forensic route", tt.query)
			}
			if got := forensicTemplateHint(tt.query); got != tt.template {
				t.Fatalf("forensicTemplateHint(%q) = %q, want %q", tt.query, got, tt.template)
			}
		})
	}
}

func TestExplicitForensicTemplateCanRequestModelAssistance(t *testing.T) {
	query := "Explain the deterministic result; template=tower_status_summary; limit=20"
	if got := forensicTemplateHint(query); got != "tower_status_summary" {
		t.Fatalf("explicit model-assisted template = %q", got)
	}
	if got := forensicSynthesisModel(query, "qwen-forensic-4b"); got != "qwen-forensic-4b" {
		t.Fatalf("model-assisted query selected %q", got)
	}
}

func TestExplicitDeterministicModeOverridesModelWordsInTemplateName(t *testing.T) {
	query := "Run deterministic forensic query; template=executive_case_brief; limit=20"
	if got := forensicSynthesisModel(query, "qwen-forensic-4b"); got != "" {
		t.Fatalf("explicit deterministic mode selected synthesis model %q", got)
	}
}

func TestEmptyEvidenceCatalogHasGovernedAnalystSummary(t *testing.T) {
	result := formatForensicEvidenceListSummary(map[string]any{
		"summary": map[string]any{"evidence_total": 0, "completed": 0, "failed": 0},
		"items":   []any{},
	})
	if !strings.Contains(result, "No evidence items matched") {
		t.Fatalf("empty evidence result did not explain the governed no-data outcome: %q", result)
	}
	if strings.Contains(result, "did not include an analyst summary") {
		t.Fatalf("empty evidence result fell back to an internal formatter message: %q", result)
	}
}

func TestEnterpriseForensicSummaryUsesReadableDefaultPresentation(t *testing.T) {
	result := formatEnterpriseForensicSummary(map[string]any{
		"collection_id": "case-alpha",
		"template":      "call_type_breakdown",
		"route":         []any{"records_sql"},
	}, map[string]any{
		"summary": "**Deterministic Findings**\n\nComputed CDR event counts by call type and direction. It returned 2 exact structured result rows.",
		"metrics": []map[string]any{
			{"label": "Template", "value": "call_type_breakdown"},
			{"label": "Route", "value": "records_sql"},
			{"label": "Records Row Count", "value": 2},
		},
		"data_grid": map[string]any{"rows": []map[string]any{
			{"call_type": "VOICE", "direction": "outgoing", "event_count": 12},
			{"call_type": "SMS", "direction": "incoming", "event_count": 4},
		}},
		"provenance":  []map[string]any{{"source": "records_sql", "source_file": "cdr.csv", "row_number": 7}},
		"limitations": []string{"A call record does not establish communication content."},
	})

	for _, expected := range []string{
		"## Call activity by type", "### Answer", "### At a glance", "### Key results",
		"| Call Type | Direction | Event Count |", "### Important context",
		"<summary>Technical details and source traceability</summary>",
	} {
		if !strings.Contains(result, expected) {
			t.Fatalf("readable presentation missing %q:\n%s", expected, result)
		}
	}
	for _, internal := range []string{"Operating Metrics", "Sourced Claims", "Data Grid Preview", "**Template:**"} {
		if strings.Contains(result, internal) {
			t.Fatalf("internal presentation label %q leaked into the main answer:\n%s", internal, result)
		}
	}
}

// TestForensicSummaryAttemptsRoleAThenFallsBackToDeterministicFactPacket
// proves the TL-8 wiring end to end for the live Ask NexusAI chat path: a
// validated Role A narrative (llm_summary with no llm_fallback_reason) is
// surfaced as the analyst-facing answer; a failed/timed-out attempt
// (llm_fallback_reason set) falls back to the deterministic Fact Packet
// executive summary; and a case with neither still returns a governed
// message rather than an empty or raw-error result. No case ever surfaces
// nothing.
func TestForensicSummaryAttemptsRoleAThenFallsBackToDeterministicFactPacket(t *testing.T) {
	baseResp := func(answer map[string]any) map[string]any {
		return map[string]any{
			"collection_id": "case-alpha",
			"template":      "call_type_breakdown",
			"route":         []any{"records_sql"},
			"answer":        answer,
		}
	}
	enterprise := map[string]any{
		"summary": "Computed CDR event counts by call type and direction. It returned 2 exact structured result rows.",
	}

	t.Run("validated model narrative is surfaced", func(t *testing.T) {
		resp := baseResp(map[string]any{"llm_summary": "Reasoning-rich grounded explanation of the call-type split."})
		result := formatEnterpriseForensicSummary(resp, enterprise)
		if !strings.Contains(result, "Reasoning-rich grounded explanation of the call-type split.") {
			t.Fatalf("validated Role A narrative was not surfaced:\n%s", result)
		}
		if strings.Contains(result, "Computed CDR event counts") {
			t.Fatalf("deterministic summary should not duplicate the validated model narrative:\n%s", result)
		}
	})

	t.Run("failed or timed out attempt falls back to the deterministic Fact Packet", func(t *testing.T) {
		resp := baseResp(map[string]any{
			"llm_summary":         "Stale text from a prior attempt.",
			"llm_fallback_reason": "Grounded explanation timed out; deterministic result retained",
		})
		result := formatEnterpriseForensicSummary(resp, enterprise)
		if !strings.Contains(result, "Computed CDR event counts by call type and direction.") {
			t.Fatalf("timeout fallback did not surface the deterministic Fact Packet answer:\n%s", result)
		}
		if strings.Contains(result, "Stale text from a prior attempt.") {
			t.Fatalf("an unvalidated model attempt leaked into the analyst-facing answer:\n%s", result)
		}
	})

	t.Run("no attempt still never surfaces nothing", func(t *testing.T) {
		resp := baseResp(nil)
		result := formatEnterpriseForensicSummary(resp, map[string]any{})
		if !strings.Contains(result, "The analysis completed, but no plain-language summary was returned.") {
			t.Fatalf("absent narrative and summary produced a blank/unexplained result:\n%s", result)
		}
	})
}

func TestAcceptedTemplatesHaveSpecificPresentationTitles(t *testing.T) {
	templates := []string{
		"collection_overview", "frequent_contacts", "call_type_breakdown", "service_usage", "device_identity_changes",
		"ipdr_endpoint_summary", "ipdr_domain_summary", "ipdr_protocol_breakdown", "ipdr_session_volume", "ipdr_subscriber_sessions",
		"ipdr_concurrent_sessions", "ipdr_timeline", "temporal_activity", "top_locations", "geospatial_movement",
		"anpr_sightings", "anpr_camera_sequence", "anpr_camera_activity", "anpr_co_travel", "anpr_route_timing", "anpr_plate_variants", "anpr_timeline",
		"subscriber_identity_lookup", "subscriber_validity_timeline", "subscriber_device_links", "subscriber_status_summary", "subscriber_conflict_audit", "subscriber_reuse_candidates",
		"entity_activity", "relationship_network", "cross_family_correlation", "entity_timeline", "source_records", "canonical_records", "schema_profile", "data_quality", "evidence",
		"shortest_call", "longest_call", "duration_extremes", "first_seen_last_seen", "activity_by_day", "activity_by_hour", "night_activity", "repeated_location_visits",
		"co_travel_or_co_presence", "subscriber_profile", "imei_imsi_usage", "tower_activity", "suspicious_patterns", "anomaly_summary", "cross_dataset_entity_summary",
		"source_file_audit", "duplicate_upload_audit", "case_readiness", "evidence_package_summary", "executive_case_brief", "court_ready_source_summary", "limitations_and_data_quality",
		"tower_site_lookup", "tower_reference_timeline", "tower_coordinate_audit", "tower_status_summary", "tower_alias_conflicts", "tower_cdr_join",
		"financial_transaction_summary", "access_failed_events", "generic_filter_records", "document_metadata", "document_search",
		"image_metadata", "image_ocr_search", "face_candidate_observations", "audio_metadata", "audio_transcript_search", "video_metadata", "video_timeline",
	}
	if len(templates) != 77 {
		t.Fatalf("presentation fixture contains %d templates, want 77", len(templates))
	}
	seen := map[string]string{}
	for _, template := range templates {
		title := forensicPresentationTitle(template)
		if title == "Forensic analysis" {
			t.Fatalf("template %q has only the generic presentation title", template)
		}
		if previous := seen[title]; previous != "" {
			t.Fatalf("templates %q and %q share presentation title %q", previous, template, title)
		}
		seen[title] = template
	}
}
