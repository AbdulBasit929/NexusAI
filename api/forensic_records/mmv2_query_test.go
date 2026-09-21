package main

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
)

func TestNX1ProtectedTypedIdentifierExtractionAndPlans(t *testing.T) {
	plate := "KTR9084"
	phone := "923451234567"
	uuids := generatedForensicUUIDs(12)

	for index, evidenceID := range uuids {
		t.Run(fmt.Sprintf("uuid_only_%02d", index), func(t *testing.T) {
			query := fmt.Sprintf("Show the grouped ANPR timeline for retained video evidence %s", evidenceID)
			understanding, plan := nx1VideoTypedPlan(t, query)
			assertTypedEntities(t, understanding, []typedEntityExpectation{{"evidence_id", evidenceID, "evidence_id"}})
			assertVideoExecutionParameters(t, plan, evidenceID, "", nil, nil)
		})
	}

	tests := []struct {
		name       string
		query      string
		wantPlate  string
		wantStart  *float64
		wantEnd    *float64
		wantEntity []typedEntityExpectation
	}{
		{
			name:       "uuid and explicit plate",
			query:      fmt.Sprintf("For retained video evidence %s show sightings of plate %s", uuids[0], plate),
			wantPlate:  plate,
			wantEntity: []typedEntityExpectation{{"evidence_id", uuids[0], "evidence_id"}, {"plate", plate, "plate"}},
		},
		{
			name:      "uuid and source time",
			query:     fmt.Sprintf("Show grouped video ANPR for evidence %s between 11 and 37 seconds", uuids[1]),
			wantStart: float64Pointer(11), wantEnd: float64Pointer(37),
			wantEntity: []typedEntityExpectation{{"evidence_id", uuids[1], "evidence_id"}},
		},
		{
			name:      "uuid plate and source time",
			query:     fmt.Sprintf("For retained video evidence %s show plate %s between 5 and 19 seconds", uuids[2], plate),
			wantPlate: plate, wantStart: float64Pointer(5), wantEnd: float64Pointer(19),
			wantEntity: []typedEntityExpectation{{"evidence_id", uuids[2], "evidence_id"}, {"plate", plate, "plate"}},
		},
		{
			name:       "roman urdu",
			query:      fmt.Sprintf("retained video evidence %s mein plate %s ki sightings dikhao", uuids[3], plate),
			wantPlate:  plate,
			wantEntity: []typedEntityExpectation{{"evidence_id", uuids[3], "evidence_id"}, {"plate", plate, "plate"}},
		},
		{
			name:       "urdu mixed exact identifiers",
			query:      fmt.Sprintf("محفوظ ویڈیو evidence %s میں plate %s کی sightings دکھائیں", uuids[4], plate),
			wantPlate:  plate,
			wantEntity: []typedEntityExpectation{{"evidence_id", uuids[4], "evidence_id"}, {"plate", plate, "plate"}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			understanding, plan := nx1VideoTypedPlan(t, tc.query)
			assertTypedEntities(t, understanding, tc.wantEntity)
			assertVideoExecutionParameters(t, plan, tc.wantEntity[0].value, tc.wantPlate, tc.wantStart, tc.wantEnd)
		})
	}

	for _, evidenceID := range []string{
		"87654321-ab12-4cd3-8ef4-567890abcdef",
		"1a2b3c4d-ab12-4ef5-bc67-89abcdef0123",
	} {
		if got := extractTargets("grouped video ANPR evidence " + evidenceID); len(got) != 1 || got[0] != evidenceID {
			t.Fatalf("protected UUID targets = %#v, want only %q", got, evidenceID)
		}
	}
	if got := extractTargets("show calls for " + phone); len(got) != 1 || got[0] != phone || classifyTargetType(got[0]) != "phone" {
		t.Fatalf("standalone phone extraction = %#v", got)
	}
	if got := extractTargets("where was plate " + plate + " seen"); len(got) != 1 || got[0] != plate {
		t.Fatalf("standalone plate extraction = %#v", got)
	}
}

func TestNX1VideoSourceTimeContract(t *testing.T) {
	random := rand.New(rand.NewSource(20260827))
	evidenceID := generatedForensicUUIDs(1)[0]
	plate := "LXR" + fmt.Sprint(1000+random.Intn(8999))
	start := float64(2 + random.Intn(12))
	end := start + float64(8+random.Intn(20))

	tests := []struct {
		name      string
		query     string
		wantPlate string
	}{
		{"from seconds to seconds", fmt.Sprintf("Show grouped ANPR for retained video evidence %s from %.0f seconds to %.0f seconds", evidenceID, start, end), ""},
		{"between seconds and seconds", fmt.Sprintf("Show grouped ANPR for retained video evidence %s between %.0f seconds and %.0f seconds", evidenceID, start, end), ""},
		{"uuid plate and time", fmt.Sprintf("For retained video evidence %s show plate %s from %.0f seconds to %.0f seconds", evidenceID, plate, start, end), plate},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			understanding, plan := nx1VideoTypedPlan(t, tc.query)
			assertSourceTimeUnderstanding(t, understanding, start, end)
			assertVideoExecutionParameters(t, plan, evidenceID, tc.wantPlate, float64Pointer(start), float64Pointer(end))
			req := requestWithExecutionParameters(hybridQueryRequest{}, plan.Steps[0].Parameters)
			records := videoANPRGroupedTimelineRecords(evidenceID, completedVideoState(), []map[string]any{}, req)
			assertFinalSourceTimeRecords(t, records, start, end, "no_match_for_filter")
		})
	}

	t.Run("time-only text with explicit operation target", func(t *testing.T) {
		req := hybridQueryRequest{
			TenantID: "default", UserID: "analyst-test", CollectionID: "case-test",
			Query: fmt.Sprintf("from %.0f seconds to %.0f seconds", start, end), Template: "video_anpr_grouped_timeline",
			Target: evidenceID, EvidenceID: evidenceID, Limit: 20, MaxKBResults: 3,
		}
		understanding, plan := nx1VideoTypedPlanFromRequest(t, req)
		assertSourceTimeUnderstanding(t, understanding, start, end)
		assertVideoExecutionParameters(t, plan, evidenceID, "", float64Pointer(start), float64Pointer(end))
	})

	t.Run("reversed range reaches existing contract rejection", func(t *testing.T) {
		reversedStart, reversedEnd := end, start
		query := fmt.Sprintf("Show grouped ANPR for retained video evidence %s from %.0f seconds to %.0f seconds", evidenceID, reversedStart, reversedEnd)
		understanding, plan := nx1VideoTypedPlan(t, query)
		assertSourceTimeUnderstanding(t, understanding, reversedStart, reversedEnd)
		req := requestWithExecutionParameters(hybridQueryRequest{}, plan.Steps[0].Parameters)
		if _, err := videoANPRGroupedTimeline(t.Context(), nil, req); err == nil || !strings.Contains(err.Error(), "end_seconds") {
			t.Fatalf("reversed source-time range error = %v", err)
		}
	})

	t.Run("outside-duration bounded miss remains truthful", func(t *testing.T) {
		duration := float64(40 + random.Intn(20))
		outsideStart, outsideEnd := duration+5, duration+15
		req := hybridQueryRequest{EvidenceID: evidenceID, StartSeconds: &outsideStart, EndSeconds: &outsideEnd}
		state := completedVideoState()
		state["duration_seconds"] = duration
		records := videoANPRGroupedTimelineRecords(evidenceID, state, []map[string]any{}, req)
		assertFinalSourceTimeRecords(t, records, outsideStart, outsideEnd, "no_match_for_filter")
	})

	t.Run("hyphenated phone is not source time", func(t *testing.T) {
		phone := fmt.Sprintf("03%02d-%07d", random.Intn(100), random.Intn(10000000))
		if gotStart, gotEnd := extractSourceSecondRange("show calls for " + phone); gotStart != nil || gotEnd != nil {
			t.Fatalf("hyphenated phone became source time: %v..%v", gotStart, gotEnd)
		}
		if targets := extractTargets("show calls for " + phone); len(targets) != 1 {
			t.Fatalf("hyphenated phone targets = %#v", targets)
		}
	})
}

func TestNX1GroupedVideoRoutingLanguageMatrix(t *testing.T) {
	evidenceID := generatedForensicUUIDs(2)[1]
	variants := []struct {
		name  string
		query string
	}{
		{"english", fmt.Sprintf("Show the grouped ANPR timeline for retained video evidence %s", evidenceID)},
		{"roman urdu", fmt.Sprintf("retained video evidence %s ki grouped ANPR timeline dikhao", evidenceID)},
		{"urdu transliterated terms", fmt.Sprintf("ویڈیو ایویڈنس %s کی گروپڈ اے این پی آر ٹائم لائن دکھائیں", evidenceID)},
		{"urdu native evidence and observations", fmt.Sprintf("ویڈیو ثبوت %s کے گروپ شدہ اے این پی آر مشاہدات دکھا دیں", evidenceID)},
		{"urdu evidence plate and find", fmt.Sprintf("وڈیو شواہد %s میں اےاینپیآر نمبر پلیٹیں تلاش کریں", evidenceID)},
		{"urdu grouped timeline synonyms", fmt.Sprintf("ویڈیو ایویڈنس %s کی گروہی اے این پی آر وقت کی ترتیب بتائیں", evidenceID)},
	}
	for _, variant := range variants {
		t.Run(variant.name, func(t *testing.T) {
			if got := chooseTemplate(variant.query, ""); got != "video_anpr_grouped_timeline" {
				t.Fatalf("template = %q; normalized = %q", got, normalizeAnalystSemantics(variant.query))
			}
			understanding, plan := nx1VideoTypedPlan(t, variant.query)
			assertTypedEntities(t, understanding, []typedEntityExpectation{{"evidence_id", evidenceID, "evidence_id"}})
			if plan.Steps[0].OperationID != "video.anpr_grouped_timeline" {
				t.Fatalf("operation = %q", plan.Steps[0].OperationID)
			}
			if strings.HasPrefix(variant.name, "urdu") && understanding.Language.Tag != "mixed" {
				t.Fatalf("Urdu query with LTR UUID language = %q, want mixed", understanding.Language.Tag)
			}
		})
	}
}

type typedEntityExpectation struct {
	typeName  string
	value     string
	parameter string
}

func nx1VideoTypedPlan(t *testing.T, query string) (QueryUnderstandingV1, GovernedExecutionPlanV1) {
	t.Helper()
	req := hybridQueryRequest{TenantID: "default", UserID: "analyst-test", CollectionID: "case-test", Query: query, Limit: 20, MaxKBResults: 3}
	return nx1VideoTypedPlanFromRequest(t, req)
}

func nx1VideoTypedPlanFromRequest(t *testing.T, req hybridQueryRequest) (QueryUnderstandingV1, GovernedExecutionPlanV1) {
	t.Helper()
	planner := planRuntimeQuery(req)
	if planner.Template != "video_anpr_grouped_timeline" {
		t.Fatalf("template = %q, want video_anpr_grouped_timeline for %q", planner.Template, req.Query)
	}
	req = bindVideoANPRQueryParameters(req, planner)
	req.Target = planner.Target
	req.Targets = canonicalTargetSet(req.Target, planner.Targets)
	req = bindVideoANPRPrimaryTarget(req, planner.Template)
	template, ok := queryTemplateByName(planner.Template)
	if !ok {
		t.Fatal("video ANPR template missing from catalog")
	}
	understanding := adaptLegacyRuntimePlan(req, planner, template)
	capability := ResolvedCapabilityV1{
		CapabilityID: template.OperationID, OperationID: template.OperationID, Template: template.Name,
		RequiredParameters: []string{"target", "evidence_id"}, OptionalParameters: []string{"plate", "start_seconds", "end_seconds", "limit"},
		ExecutionMode: "deterministic", ResultContract: "forensics.enterprise-response/v1", ResultKind: resultKindForTemplate(template), PresentationType: template.Presentation,
		CitationRequired: true, Availability: CapabilityAvailabilityV1{Executable: true},
	}
	plan, err := buildSingleCapabilityPlan("request-test", req, understanding, capability)
	if err != nil {
		t.Fatalf("build governed video plan: %v", err)
	}
	return understanding, plan
}

func assertSourceTimeUnderstanding(t *testing.T, understanding QueryUnderstandingV1, start, end float64) {
	t.Helper()
	if understanding.TimeScope.From != "" || understanding.TimeScope.To != "" {
		t.Fatalf("video source time leaked into date time_scope: %#v", understanding.TimeScope)
	}
	want := map[string]string{"start_seconds": fmt.Sprint(start), "end_seconds": fmt.Sprint(end)}
	for _, filter := range understanding.Filters {
		if expected, ok := want[filter.Field]; ok && filter.Value == expected {
			delete(want, filter.Field)
		}
	}
	if len(want) != 0 {
		t.Fatalf("source-time filters missing: %#v from %#v", want, understanding.Filters)
	}
}

func assertFinalSourceTimeRecords(t *testing.T, records map[string]any, start, end float64, status string) {
	t.Helper()
	if !equalOptionalFloat(records["start_seconds"].(*float64), &start) || !equalOptionalFloat(records["end_seconds"].(*float64), &end) {
		t.Fatalf("final records source time = %#v..%#v", records["start_seconds"], records["end_seconds"])
	}
	if records["status"] != status || records["row_count"] != 0 {
		t.Fatalf("final bounded records contract = %#v", records)
	}
}

func completedVideoState() map[string]any {
	return map[string]any{"processing_status": "completed", "anpr_result_state": "COMPLETE_ZERO_RESULTS", "version_id": "version-test", "source_file": "video-test.mp4"}
}

func assertTypedEntities(t *testing.T, understanding QueryUnderstandingV1, want []typedEntityExpectation) {
	t.Helper()
	if len(understanding.Entities) != len(want) || len(understanding.Targets) != len(want) {
		t.Fatalf("typed entities/targets = %#v / %#v, want %#v", understanding.Entities, understanding.Targets, want)
	}
	for index, expected := range want {
		entity, target := understanding.Entities[index], understanding.Targets[index]
		if entity.Type != expected.typeName || entity.Original != expected.value || entity.Normalized != expected.value || target.EntityIndex != index || target.Parameter != expected.parameter || target.Value != expected.value {
			t.Fatalf("typed entity/target[%d] = %#v / %#v, want %#v", index, entity, target, expected)
		}
	}
}

func assertVideoExecutionParameters(t *testing.T, plan GovernedExecutionPlanV1, evidenceID, plate string, start, end *float64) {
	t.Helper()
	parameters := plan.Steps[0].Parameters
	if parameters.Target != evidenceID || parameters.EvidenceID != evidenceID || len(parameters.Targets) != 1 || parameters.Targets[0] != evidenceID || parameters.Plate != plate {
		t.Fatalf("video execution identifiers = %#v", parameters)
	}
	if !equalOptionalFloat(parameters.StartSeconds, start) || !equalOptionalFloat(parameters.EndSeconds, end) {
		t.Fatalf("video execution source seconds = %v..%v, want %v..%v", parameters.StartSeconds, parameters.EndSeconds, start, end)
	}
}

func generatedForensicUUIDs(count int) []string {
	random := rand.New(rand.NewSource(20260826))
	result := make([]string, 0, count)
	for range count {
		value := make([]byte, 16)
		_, _ = random.Read(value)
		value[6] = value[6]&0x0f | byte(1+random.Intn(5))<<4
		value[8] = value[8]&0x3f | 0x80
		result = append(result, fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
			value[0:4], value[4:6], value[6:8], value[8:10], value[10:16]))
	}
	return result
}

func float64Pointer(value float64) *float64 { return &value }

func equalOptionalFloat(got, want *float64) bool {
	return got == nil && want == nil || got != nil && want != nil && *got == *want
}

func TestMMV2UrduANPRRoutingPreservesASCIIPlateTarget(t *testing.T) {
	query := "نمبر پلیٹ PKX4821 کب نظر آئی دکھائیں"
	if got := chooseTemplate(query, ""); got != "anpr_sightings" {
		t.Fatalf("Urdu ANPR template = %q, want anpr_sightings", got)
	}
	if got := extractTarget(query); got != "PKX4821" {
		t.Fatalf("ASCII plate target = %q, want PKX4821", got)
	}
}

func TestMMV2RequiredUrduANPRExamplesRouteGenerically(t *testing.T) {
	tests := map[string]string{
		"MN1367 کہاں نظر آئی؟":                      "anpr_sightings",
		"اس کیس میں MN1367 کہاں ملی؟":               "anpr_sightings",
		"اس تصویر میں کون سی نمبر پلیٹ ہے؟":         "anpr_sightings",
		"اس ویڈیو میں کون سی نمبر پلیٹیں نظر آئیں؟": "video_anpr_grouped_timeline",
		"اس ویڈیو کی نمبر پلیٹ ٹائم لائن دکھائیں":   "video_anpr_grouped_timeline",
	}
	for query, want := range tests {
		if got := chooseTemplate(query, ""); got != want {
			t.Errorf("query %q planned %q, want %q", query, got, want)
		}
	}
	if got := extractTarget("اس کیس میں MN1367 کہاں ملی؟"); got != "MN1367" {
		t.Fatalf("ASCII plate target = %q, want MN1367", got)
	}
}

func TestMMV2VideoANPRSourceSecondRangeExtraction(t *testing.T) {
	for _, query := range []string{
		"Show video plates between 10 and 30 seconds",
		"is video mein 10 aur 30 second ke darmiyan plates dikhao",
		"اس ویڈیو میں 10 سے 30 سیکنڈ کے درمیان پلیٹیں دکھائیں",
	} {
		start, end := extractSourceSecondRange(query)
		if start == nil || end == nil || *start != 10 || *end != 30 {
			t.Errorf("query %q range = %v..%v, want 10..30", query, start, end)
		}
	}
}

func TestMMV2VideoANPROutcomeSemantics(t *testing.T) {
	status, message := videoANPROutcome("completed", "COMPLETE_ZERO_RESULTS", 0)
	if status != "complete_zero" || message != "ANPR processing complete, 0 plate groups detected." {
		t.Fatalf("completed-zero outcome = %q / %q", status, message)
	}
	status, _ = videoANPROutcome("processing", "PROCESSING", 0)
	if status != "processing_not_complete" {
		t.Fatalf("incomplete zero outcome = %q", status)
	}
	status, _ = videoANPROutcome("completed", "MODEL_REQUIRED", 0)
	if status != "model_required" {
		t.Fatalf("missing model outcome = %q", status)
	}
	status, _ = videoANPROutcome("completed", "", 0)
	if status != "not_run" {
		t.Fatalf("unexecuted ANPR outcome = %q", status)
	}
}

func TestMMV2VideoANPRRoutesAcrossLanguages(t *testing.T) {
	for _, query := range []string{
		"Which plates appear in this video?",
		"Show the grouped ANPR timeline for retained video evidence 50057921-4f1f-4ab8-bab0-47bcdc957822.",
		"Show the grouped ANPR timeline for retained video evidence 50057921-4f1f-4ab8-bab0-47bcdc957822 and plate MN1367.",
		"Show the grouped ANPR timeline for retained video evidence 50057921-4f1f-4ab8-bab0-47bcdc957822 between 10 and 30 seconds.",
		"is video mein konsi number plates hain",
		"اس ویڈیو میں کون سی نمبر پلیٹیں ہیں",
	} {
		if got := chooseTemplate(query, ""); got != "video_anpr_grouped_timeline" {
			t.Errorf("query %q planned %q", query, got)
		}
	}
}

func TestMMV2VideoANPRInputValidation(t *testing.T) {
	negative := -1.0
	_, err := videoANPRGroupedTimeline(t.Context(), nil, hybridQueryRequest{EvidenceID: "not-a-uuid"})
	if err == nil {
		t.Fatal("invalid evidence UUID was accepted")
	}
	_, err = videoANPRGroupedTimeline(t.Context(), nil, hybridQueryRequest{EvidenceID: "50057921-4f1f-4ab8-bab0-47bcdc957822", StartSeconds: &negative})
	if err == nil {
		t.Fatal("negative source-second bound was accepted")
	}
}

func TestMMV2EnterpriseResultStateMatrix(t *testing.T) {
	positive := hybridQueryResponse{
		Template: "anpr_sightings", Route: []string{"records_sql"},
		Records: map[string]any{"anpr_sightings": []map[string]any{{
			"normalized_plate_text": "TEST123", "source_file": "plate.jpg", "row_number": 3,
			"evidence_id": "evidence-test", "version_id": "version-test", "row_hash": "hash-test", "observed_at": "2026-08-25T08:00:00Z",
		}}},
		Answer: map[string]any{"records_row_count": 1, "records_status": "matched", "records_summary": "Retrieved exact ANPR observations."},
	}
	positivePayload := buildEnterprisePayload(hybridQueryRequest{CollectionID: "case-test", Target: "TEST123"}, positive)
	if positivePayload["result_state"] != EnterpriseResultStateResultsPresent || positivePayload["row_count"] != 1 {
		t.Fatalf("positive result contract = %#v", positivePayload)
	}
	if strings.Contains(strings.ToLower(stringValueAny(positivePayload["summary"])), "no matching") {
		t.Fatalf("positive summary contradicted authoritative row: %q", positivePayload["summary"])
	}
	positive.Enterprise = positivePayload
	typedPositive := legacyHybridToEnterpriseV1("case-test", "request-positive", map[string]any{"tenant_id": "default"}, positive)
	if typedPositive.ResultState != EnterpriseResultStateResultsPresent || len(typedPositive.SemanticEvidence) != 1 {
		t.Fatalf("positive typed response lost state or citation: %#v", typedPositive)
	}
	if typedPositive.SemanticEvidence[0].EvidenceID != "evidence-test" || !strings.Contains(typedPositive.SemanticEvidence[0].Locator, "row_number:3") {
		t.Fatalf("positive citation changed: %#v", typedPositive.SemanticEvidence[0])
	}

	multi := positive
	multi.Records = map[string]any{"anpr_sightings": []map[string]any{{"normalized_plate_text": "TEST123"}, {"normalized_plate_text": "TEST123"}}}
	multi.Answer = map[string]any{"records_row_count": 2, "records_status": "matched"}
	if got := enterpriseResultState(multi, flattenEnterpriseRows(multi.Records, 100)); got != EnterpriseResultStateResultsPresent {
		t.Fatalf("multi-row state = %q", got)
	}

	completeZero := hybridQueryResponse{
		Template: "video_anpr_grouped_timeline", Route: []string{"records_sql"},
		Records: map[string]any{"video_anpr_grouped_timeline": []map[string]any{}, "row_count": 0, "status": "complete_zero", "processing_status": "completed", "executive_state": "ANPR processing complete, 0 plate groups detected."},
		Answer:  map[string]any{"records_row_count": 0, "records_status": "no_matching_records"},
	}
	zeroPayload := buildEnterprisePayload(hybridQueryRequest{CollectionID: "case-test"}, completeZero)
	if zeroPayload["result_state"] != EnterpriseResultStateCompleteZero || zeroPayload["status"] == "no_results" {
		t.Fatalf("complete-zero contract = %#v", zeroPayload)
	}
	if !strings.Contains(stringValueAny(zeroPayload["summary"]), "ANPR processing complete, 0 plate groups detected.") {
		t.Fatalf("complete-zero summary = %q", zeroPayload["summary"])
	}

	tests := []struct {
		name string
		resp hybridQueryResponse
		want string
	}{
		{"filter miss", hybridQueryResponse{Answer: map[string]any{"records_row_count": 0, "records_status": "no_matching_records", "evidence_count": 1}}, EnterpriseResultStateNoMatchForFilter},
		{"video plate filter miss", hybridQueryResponse{Records: map[string]any{"status": "no_match_for_filter", "processing_status": "completed", "plate_filter": "TEST999"}, Answer: map[string]any{"records_row_count": 0}}, EnterpriseResultStateNoMatchForFilter},
		{"not processed", hybridQueryResponse{Records: map[string]any{"status": "processing_not_complete", "processing_status": "registered"}, Answer: map[string]any{}}, EnterpriseResultStateNotProcessed},
		{"processing", hybridQueryResponse{Records: map[string]any{"processing_status": "running"}, Answer: map[string]any{}}, EnterpriseResultStateProcessing},
		{"failed", hybridQueryResponse{Records: map[string]any{"processing_status": "dead_letter"}, Answer: map[string]any{}}, EnterpriseResultStateFailed},
		{"unavailable", hybridQueryResponse{Answer: map[string]any{"failure_semantics": string(CapabilityReasonRuntimeUnavailable)}}, EnterpriseResultStateUnavailable},
		{"unauthorized", hybridQueryResponse{Answer: map[string]any{"failure_semantics": string(CapabilityReasonUnauthorized)}}, EnterpriseResultStateUnauthorized},
		{"invalid", hybridQueryResponse{Answer: map[string]any{"failure_semantics": string(CapabilityReasonInvalidParameter)}}, EnterpriseResultStateInvalidRequest},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := enterpriseResultState(tc.resp, nil); got != tc.want {
				t.Fatalf("state = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestMMV2PublicEnterpriseAdapterPreservesCompleteZeroAndOperation(t *testing.T) {
	legacy := hybridQueryResponse{
		CollectionID: "case-test", Template: "video_anpr_grouped_timeline", Route: []string{"records_sql"},
		Records: map[string]any{"video_anpr_grouped_timeline": []map[string]any{}, "row_count": 0, "status": "complete_zero", "processing_status": "completed", "executive_state": "ANPR processing complete, 0 plate groups detected."},
		Answer:  map[string]any{"records_row_count": 0, "records_status": "no_matching_records"},
	}
	legacy.Enterprise = buildEnterprisePayload(hybridQueryRequest{CollectionID: "case-test"}, legacy)
	response := legacyHybridToEnterpriseV1("case-test", "request-test", map[string]any{"tenant_id": "default", "limit": 20}, legacy)
	if response.ResultState != EnterpriseResultStateCompleteZero || response.ProcessingState != "completed" || response.RowCount != 0 {
		t.Fatalf("typed complete-zero response = %#v", response)
	}
	if response.OperationID != "video.anpr_grouped_timeline" || !containsString(response.ExecutionTrace.ToolIDs, "video.anpr_grouped_timeline") {
		t.Fatalf("video operation identity was lost: %#v", response.ExecutionTrace)
	}
	if !strings.Contains(response.ExecutiveAnswer, "ANPR processing complete, 0 plate groups detected.") {
		t.Fatalf("typed complete-zero answer = %q", response.ExecutiveAnswer)
	}
}

func TestMMV2PublicEnterpriseRowCountUsesAuthoritativeResultCount(t *testing.T) {
	metadataWrapper := map[string]any{
		"status": "complete_zero", "processing_status": "completed",
		"evidence_id": "synthetic-evidence", "limitations": []any{"bounded synthetic limitation"},
	}
	tests := []struct {
		name        string
		resultState string
		records     map[string]any
		answer      map[string]any
		want        int
	}{
		{name: "positive one", resultState: EnterpriseResultStateResultsPresent, records: map[string]any{"rows": []map[string]any{{"value": "one"}}, "row_count": 1}, want: 1},
		{name: "positive multi-row", resultState: EnterpriseResultStateResultsPresent, records: map[string]any{"rows": []map[string]any{{"value": "one"}, {"value": "two"}, {"value": "three"}}, "row_count": "3"}, want: 3},
		{name: "no match metadata wrapper", resultState: EnterpriseResultStateNoMatchForFilter, records: map[string]any{"status": "no_match_for_filter", "processing_status": "completed", "row_count": 0, "limitations": metadataWrapper["limitations"]}, answer: map[string]any{"records_row_count": 0}, want: 0},
		{name: "complete zero metadata wrapper", resultState: EnterpriseResultStateCompleteZero, records: map[string]any{"rows": []map[string]any{}, "row_count": 0, "status": metadataWrapper["status"], "processing_status": metadataWrapper["processing_status"], "evidence_id": metadataWrapper["evidence_id"], "limitations": metadataWrapper["limitations"]}, answer: map[string]any{"records_row_count": 0}, want: 0},
		{name: "not processed", resultState: EnterpriseResultStateNotProcessed, records: map[string]any{"processing_status": "registered"}, want: 0},
		{name: "failed", resultState: EnterpriseResultStateFailed, records: map[string]any{"processing_status": "dead_letter"}, want: 0},
		{name: "unavailable", resultState: EnterpriseResultStateUnavailable, answer: map[string]any{"failure_semantics": string(CapabilityReasonRuntimeUnavailable)}, want: 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			legacy := hybridQueryResponse{CollectionID: "case-test", Template: "anpr_sightings", Route: []string{"records_sql"}, Records: tc.records, Answer: tc.answer}
			legacy.Enterprise = map[string]any{"result_state": tc.resultState}
			response := legacyHybridToEnterpriseV1("case-test", "request-test", map[string]any{"tenant_id": "default"}, legacy)
			if response.RowCount != tc.want {
				t.Fatalf("public row_count = %d, want %d; flattened rows = %#v", response.RowCount, tc.want, flattenEnterpriseRows(tc.records, 100))
			}
		})
	}
}
