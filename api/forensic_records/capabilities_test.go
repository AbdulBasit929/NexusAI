package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestCapabilityCatalogMatchesEvaluationMatrix(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "configuration", "forensic_modality_evaluation_matrix.json"))
	if err != nil {
		t.Fatal(err)
	}
	var matrix struct {
		RequiredFamilies []string `json:"required_families"`
		Profiles         []struct {
			ID           string `json:"id"`
			SupportLevel string `json:"support_level"`
			Adapter      string `json:"adapter"`
		} `json:"profiles"`
	}
	if err := json.Unmarshal(data, &matrix); err != nil {
		t.Fatal(err)
	}
	definitions := forensicCapabilityDefinitions()
	if len(definitions) != 20 || len(definitions) != len(matrix.RequiredFamilies) {
		t.Fatalf("capability families = %d, required matrix families = %d", len(definitions), len(matrix.RequiredFamilies))
	}
	byID := make(map[string]forensicCapabilityDefinition, len(definitions))
	for _, definition := range definitions {
		if _, exists := byID[definition.ID]; exists {
			t.Fatalf("duplicate capability family %q", definition.ID)
		}
		byID[definition.ID] = definition
	}
	for _, profile := range matrix.Profiles {
		definition, exists := byID[profile.ID]
		if !exists {
			t.Errorf("matrix family %q missing from API catalog", profile.ID)
			continue
		}
		if definition.SupportLevel != profile.SupportLevel {
			t.Errorf("%s support level = %q, want %q", profile.ID, definition.SupportLevel, profile.SupportLevel)
		}
		if definition.Adapter != profile.Adapter {
			t.Errorf("%s adapter = %q, want %q", profile.ID, definition.Adapter, profile.Adapter)
		}
	}
}

func TestForensicQueryCorpusIsVersionedAndRoutesToKnownFamilies(t *testing.T) {
	corpus, err := loadForensicQueryCorpus()
	if err != nil {
		t.Fatal(err)
	}
	if corpus.ContractVersion != "forensics.family-query-answer-corpus/v1" || corpus.CorpusVersion == "" {
		t.Fatalf("unexpected corpus identity: %#v", corpus)
	}
	knownFamilies := map[string]bool{"cross_family": true}
	for _, family := range forensicCapabilityDefinitions() {
		knownFamilies[family.ID] = true
	}
	seen := map[string]bool{}
	coveredTemplates := map[string]bool{}
	for _, entry := range corpus.Entries {
		if entry.ID == "" || seen[entry.ID] {
			t.Fatalf("corpus entry id is empty or duplicated: %q", entry.ID)
		}
		seen[entry.ID] = true
		if !knownFamilies[entry.FamilyID] || entry.Query == "" || entry.ExpectedTemplate == "" {
			t.Fatalf("invalid corpus entry: %#v", entry)
		}
		if len(entry.RequiredSections) < 3 {
			t.Fatalf("answer contract is incomplete for %q", entry.ID)
		}
		coveredTemplates[entry.ExpectedTemplate] = true
		if entry.Suggested {
			if planned := chooseTemplate(entry.Query, ""); planned != entry.ExpectedTemplate {
				t.Errorf("suggested corpus query %q plans %q, want %q", entry.Query, planned, entry.ExpectedTemplate)
			}
		}
	}
	for _, template := range supportedQueryTemplates() {
		if !coveredTemplates[template.Name] {
			t.Errorf("accepted template %q has no versioned corpus entry", template.Name)
		}
	}
	if len(corpus.Scenarios) < 7 {
		t.Fatalf("corpus scenarios = %d, want ambiguity/no-data/unsupported/model/source coverage", len(corpus.Scenarios))
	}
}

func TestAllAcceptedTemplatesPublishFamilyOperationAndSourceAccess(t *testing.T) {
	templates := supportedQueryTemplates()
	if len(templates) != 79 {
		t.Fatalf("template count = %d, want 79", len(templates))
	}
	for _, template := range templates {
		if template.OperationID == "" || template.FamilyID == "" || template.ExampleQuery == "" {
			t.Errorf("template %q lacks complete specialist guidance: %#v", template.Name, template)
		}
		if template.Route != "records" && template.Route != "kb" && template.Route != "derived" && template.Route != "hybrid" {
			t.Errorf("template %q has unsupported source access %q", template.Name, template.Route)
		}
		if template.OutputDescription == "" || template.Calculation == "" || len(template.Limitations) == 0 {
			t.Errorf("template %q lacks presentation or limitations", template.Name)
		}
	}
}

func TestEnterprisePayloadCarriesGovernedOperationMetadata(t *testing.T) {
	payload := buildEnterprisePayload(hybridQueryRequest{
		TenantID:     "tenant-a",
		CollectionID: "case-a",
	}, hybridQueryResponse{
		Template: "call_type_breakdown",
		Route:    []string{"records_sql"},
		Answer:   map[string]any{},
	})
	operation, ok := payload["operation"].(map[string]any)
	if !ok {
		t.Fatalf("enterprise operation metadata = %#v", payload["operation"])
	}
	if operation["operation_id"] != "cdr.call_type_breakdown" ||
		operation["family_id"] != "communications_cdr" ||
		operation["source_access"] != "records" {
		t.Fatalf("unexpected governed operation metadata: %#v", operation)
	}
}

func TestCorpusGovernanceScenariosMatchPlannerAndModelPolicy(t *testing.T) {
	corpus, err := loadForensicQueryCorpus()
	if err != nil {
		t.Fatal(err)
	}
	for _, scenario := range corpus.Scenarios {
		switch scenario.Kind {
		case "unsupported_operation":
			if assessment := assessQueryCapability(scenario.Query, ""); assessment.Status != scenario.ExpectedOutcome {
				t.Errorf("scenario %q outcome = %q, want %q", scenario.ID, assessment.Status, scenario.ExpectedOutcome)
			}
		case "model_rejection":
			if isForensicSynthesisModelName("whisper-large-v3") {
				t.Errorf("scenario %q accepted an incompatible model", scenario.ID)
			}
		case "source_access":
			templateName := chooseTemplate(scenario.Query, "")
			found := false
			for _, template := range supportedQueryTemplates() {
				if template.Name == templateName {
					found = template.Route == scenario.ExpectedOutcome
				}
			}
			if !found {
				t.Errorf("scenario %q did not select %q source access (template %q)", scenario.ID, scenario.ExpectedOutcome, templateName)
			}
		}
	}
}

func TestAssessQueryCapabilityBlocksUnavailableProcessing(t *testing.T) {
	tests := []struct {
		query  string
		family string
	}{
		{"transcribe and diarize this audio", "audio_and_stt"},
		{"identify person in image evidence", "images_and_ocr"},
		{"analyze video and make a timeline", "video"},
		{"extract archive recursively", "archives_and_bundles"},
		{"query sqlite for all users", "databases_and_sql"},
		{"evaluate formula in xlsx workbook", "spreadsheets_and_columnar"},
	}
	for _, test := range tests {
		assessment := assessQueryCapability(test.query, "")
		if assessment.Status != "unavailable" {
			t.Errorf("%q status = %q", test.query, assessment.Status)
		}
		if !containsString(assessment.RequestedFamilies, test.family) {
			t.Errorf("%q families = %#v, want %q", test.query, assessment.RequestedFamilies, test.family)
		}
	}
}

func TestAssessQueryCapabilityAllowsAcceptedDeterministicQueries(t *testing.T) {
	for _, query := range []string{
		"show frequent contacts for 923461678183",
		"where was plate ABC-123 seen?",
		"show exact financial transaction rows",
		"show image metadata",
		"show capture inventory",
		"read xlsx workbook rows",
		"query parquet records",
		"read text from image evidence",
		"find this phrase in the Roman Urdu transcript",
		"extract pdf text",
	} {
		if assessment := assessQueryCapability(query, ""); assessment.Status != "supported_by_selected_route" {
			t.Errorf("%q unexpectedly blocked: %#v", query, assessment)
		}
	}
}

func TestMaterializeCapabilitiesSeparatesQueryablePendingAndNoData(t *testing.T) {
	families := materializeForensicCapabilities(
		CoverageSummary{RecordFamiliesPresent: []RecordFamilyCoverage{{RecordType: "cdr", Count: 42}}},
		[]evidenceCapabilityCount{
			{Modality: "structured_records", DetectedType: "cdr", Extension: "tsv", Total: 1, Normalized: 1},
			{Modality: "image", DetectedType: "image", Extension: "jpg", Total: 2, Derived: 3, ArtifactTypes: []string{"forensics.image-ocr-observation/v1"}},
		},
	)
	byID := map[string]forensicFamilyCapability{}
	for _, family := range families {
		byID[family.ID] = family
	}
	if byID["cdr"].Availability != "queryable" || byID["cdr"].IndexedRecords != 42 {
		t.Fatalf("CDR capability = %#v", byID["cdr"])
	}
	if byID["spreadsheets_and_columnar"].Availability != "queryable" || byID["spreadsheets_and_columnar"].NormalizedEvidence != 1 {
		t.Fatalf("TSV capability = %#v", byID["spreadsheets_and_columnar"])
	}
	if byID["images_and_ocr"].Availability != "limited" || byID["images_and_ocr"].DerivedArtifacts != 3 {
		t.Fatalf("image capability = %#v", byID["images_and_ocr"])
	}
	if len(byID["images_and_ocr"].SuggestedQueries) != 0 {
		t.Fatalf("uncertified image operations are visible as ordinary suggestions: %#v", byID["images_and_ocr"].SuggestedQueries)
	}
	if byID["audio_and_stt"].Availability != "no_data" {
		t.Fatalf("audio capability = %#v", byID["audio_and_stt"])
	}
	if got := byID["cdr"].SuggestedQueries; !containsString(got, "show frequent contacts") || !containsString(got, "show hourly and daily CDR activity") || len(got) != 2 {
		t.Fatalf("product-certified CDR suggestions = %#v", got)
	}
}

func TestCapabilitiesHandlerReturnsAllFamiliesWithoutDatabase(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/query/capabilities?collection_id=case-a&tenant_id=tenant-a", nil)
	recorder := httptest.NewRecorder()
	forensicCapabilitiesHandler(nil).ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		CollectionID string                     `json:"collection_id"`
		Families     []forensicFamilyCapability `json:"families"`
		Summary      map[string]int             `json:"summary"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.CollectionID != "case-a" || len(response.Families) != 20 || response.Summary["families_total"] != 20 {
		t.Fatalf("unexpected response: collection=%q families=%d summary=%#v", response.CollectionID, len(response.Families), response.Summary)
	}
}

func TestHybridQueryCapabilityGuardRunsBeforeDatabaseOrModel(t *testing.T) {
	body := []byte(`{"tenant_id":"tenant-a","collection_id":"case-a","query":"transcribe and diarize this audio"}`)
	req := httptest.NewRequest(http.MethodPost, "/query/hybrid", bytes.NewReader(body))
	recorder := httptest.NewRecorder()
	hybridQueryHandler(config{}, nil).ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response hybridQueryResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Capability.Status != "unavailable" || len(response.Route) != 1 || response.Route[0] != "capability_guard" {
		t.Fatalf("capability guard response = %#v", response)
	}
	if response.Telemetry.DBLatencyMS != 0 || response.Telemetry.KBLatencyMS != 0 || response.Telemetry.LLMLatencyMS != 0 {
		t.Fatalf("guard unexpectedly used a downstream runtime: %#v", response.Telemetry)
	}
}
