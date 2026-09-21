package main

import (
	_ "embed"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const forensicCapabilityCatalogVersion = "2026-09-03.nxb21c.source-candidate"

//go:embed contracts/forensic-family-query-answer-corpus-v1.json
var forensicQueryAnswerCorpusJSON []byte

type forensicQueryCorpusEntry struct {
	ID               string   `json:"id"`
	FamilyID         string   `json:"family_id"`
	Locale           string   `json:"locale"`
	Query            string   `json:"query"`
	ExpectedTemplate string   `json:"expected_template"`
	RequiredSections []string `json:"required_sections"`
	OperationID      string   `json:"operation_id,omitempty"`
	SourceAccess     string   `json:"source_access,omitempty"`
	Suggested        bool     `json:"suggested"`
}

type forensicQueryCorpusScenario struct {
	ID              string   `json:"id"`
	Kind            string   `json:"kind"`
	Query           string   `json:"query"`
	ExpectedOutcome string   `json:"expected_outcome"`
	Assertions      []string `json:"assertions"`
}

type forensicQueryCorpus struct {
	ContractVersion string                        `json:"contract_version"`
	CorpusVersion   string                        `json:"corpus_version"`
	Entries         []forensicQueryCorpusEntry    `json:"entries"`
	Scenarios       []forensicQueryCorpusScenario `json:"scenarios"`
}

func loadForensicQueryCorpus() (forensicQueryCorpus, error) {
	var corpus forensicQueryCorpus
	if err := json.Unmarshal(forensicQueryAnswerCorpusJSON, &corpus); err != nil {
		return corpus, err
	}
	if corpus.ContractVersion != "forensics.family-query-answer-corpus/v1" || corpus.CorpusVersion == "" || len(corpus.Entries) == 0 {
		return corpus, errors.New("invalid forensic family query/answer corpus")
	}
	return materializeForensicQueryCorpus(corpus), nil
}

func materializeForensicQueryCorpus(corpus forensicQueryCorpus) forensicQueryCorpus {
	byTemplate := make(map[string]struct{}, len(corpus.Entries))
	for index := range corpus.Entries {
		entry := &corpus.Entries[index]
		entry.Suggested = true
		byTemplate[entry.ExpectedTemplate] = struct{}{}
	}
	for _, template := range supportedQueryTemplates() {
		if _, exists := byTemplate[template.Name]; exists {
			continue
		}
		query := strings.TrimSpace(template.ExampleQuery)
		if query == "" || strings.Contains(query, "{") {
			query = "Run deterministic forensic query; template=" + template.Name + "; limit=20"
		}
		corpus.Entries = append(corpus.Entries, forensicQueryCorpusEntry{
			ID:               "catalog-" + strings.ReplaceAll(template.Name, "_", "-"),
			FamilyID:         capabilityFamilyForTemplate(template),
			Locale:           "en",
			Query:            query,
			ExpectedTemplate: template.Name,
			RequiredSections: []string{"executive_answer", "deterministic_findings", "citations", "limitations"},
			OperationID:      template.OperationID,
			SourceAccess:     template.Route,
			Suggested:        false,
		})
	}
	sort.SliceStable(corpus.Entries, func(i, j int) bool {
		if corpus.Entries[i].Suggested != corpus.Entries[j].Suggested {
			return corpus.Entries[i].Suggested
		}
		return corpus.Entries[i].ID < corpus.Entries[j].ID
	})
	return corpus
}

func capabilityFamilyForTemplate(template queryTemplateCatalogEntry) string {
	switch template.FamilyID {
	case "communications_cdr":
		return "cdr"
	case "network_ipdr":
		return "ipdr_network_sessions"
	case "anpr_vehicles":
		return "anpr_vehicle_sightings"
	case "subscriber_identity", "tower_location":
		return template.FamilyID
	case "financial_transactions":
		return "financial_transactions"
	case "access_security_logs":
		return "logs_access_security"
	case "generic_tabular":
		return "generic_tabular"
	case "document_intelligence":
		return "pdf_office_email_documents"
	case "image_intelligence":
		return "images_and_ocr"
	case "face_intelligence":
		return "images_and_ocr"
	case "audio_intelligence":
		return "audio_and_stt"
	case "video_intelligence":
		return "video"
	}
	if len(template.RecordTypes) == 1 {
		switch template.RecordTypes[0] {
		case "cdr":
			return "cdr"
		case "ipdr":
			return "ipdr_network_sessions"
		case "anpr":
			return "anpr_vehicle_sightings"
		case "subscriber":
			return "subscriber_identity"
		case "tower_location", "transaction", "access_log", "generic":
			mapping := map[string]string{"transaction": "financial_transactions", "access_log": "logs_access_security", "generic": "generic_tabular"}
			if mapped := mapping[template.RecordTypes[0]]; mapped != "" {
				return mapped
			}
			return template.RecordTypes[0]
		}
	}
	return "cross_family"
}

type forensicCapabilityDefinition struct {
	ID                      string   `json:"id"`
	Label                   string   `json:"label"`
	Formats                 []string `json:"formats"`
	SupportLevel            string   `json:"support_level"`
	Adapter                 string   `json:"adapter"`
	RecordTypes             []string `json:"record_types"`
	EvidenceModalities      []string `json:"evidence_modalities,omitempty"`
	EvidenceTypes           []string `json:"evidence_types,omitempty"`
	DeterministicOperations []string `json:"deterministic_operations"`
	SemanticOperations      []string `json:"semantic_operations"`
	NextGate                string   `json:"next_gate"`
	SuggestedQueries        []string `json:"suggested_queries"`
}

type forensicFamilyCapability struct {
	forensicCapabilityDefinition
	Availability         string   `json:"availability"`
	IndexedRecords       int64    `json:"indexed_records"`
	RegisteredEvidence   int64    `json:"registered_evidence"`
	KBReadyEvidence      int64    `json:"kb_ready_evidence"`
	NormalizedEvidence   int64    `json:"normalized_evidence"`
	DerivedArtifacts     int64    `json:"derived_artifacts"`
	DerivedArtifactTypes []string `json:"derived_artifact_types,omitempty"`
	AvailabilityReason   string   `json:"availability_reason"`
}

type queryCapabilityAssessment struct {
	Status              string   `json:"status"`
	RequestedFamilies   []string `json:"requested_families"`
	RequiredOperations  []string `json:"required_operations"`
	MissingCapabilities []string `json:"missing_capabilities"`
	Explanation         string   `json:"explanation"`
	SuggestedQueries    []string `json:"suggested_queries"`
}

type evidenceCapabilityCount struct {
	Modality      string
	DetectedType  string
	Extension     string
	Total         int64
	KBReady       int64
	Normalized    int64
	Derived       int64
	ArtifactTypes []string
}

func forensicCapabilitiesHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
			return
		}
		collectionID := normalizeCollectionID(r.URL.Query().Get("collection_id"))
		scope, err := bindForensicScope(r, r.URL.Query().Get("tenant_id"), collectionID, r.URL.Query().Get("case_id"), "")
		if err != nil {
			writeScopeError(w, err)
			return
		}
		if scope.CollectionID == "" {
			writeError(w, http.StatusBadRequest, errors.New("collection_id is required"))
			return
		}

		evidenceID, versionID := r.URL.Query().Get("evidence_id"), r.URL.Query().Get("evidence_version_id")
		for _, id := range []string{evidenceID, versionID} {
			if id != "" {
				if _, err := uuid.Parse(id); err != nil {
					writeError(w, http.StatusBadRequest, errors.New("evidence/version must be UUIDs"))
					return
				}
			}
		}
		if versionID != "" && evidenceID == "" {
			writeError(w, http.StatusBadRequest, errors.New("evidence version requires evidence ID"))
			return
		}
		refs, err := loadQueryCapabilityReferences()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		states, err := loadSemanticCapabilityStates(r.Context(), db, scope, evidenceID, versionID, refs)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, errors.New("capability state unavailable"))
			return
		}
		semanticContext := SemanticReadinessContextV1{Authorized: true, ExecutorAvailable: db != nil, Origin: "SOURCE_CANDIDATE", SemanticContracts: map[string]bool{"derived-text-search/v1": true}, ScopeMode: "authorized_workspace"}
		if evidenceID != "" {
			semanticContext.ScopeMode = "selected_evidence"
		}
		readiness := make([]SemanticCapabilityReadinessV1, 0, len(refs.Capabilities))
		for _, c := range refs.Capabilities {
			readiness = append(readiness, projectSemanticReadiness(c, states[c.ID], semanticContext))
		}
		coverage, err := loadCapabilityCoverage(r.Context(), db, scope, evidenceID, versionID)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, errors.New("capability coverage unavailable"))
			return
		}
		evidence, err := loadEvidenceCapabilityCounts(r, db, scope.TenantID, scope.CollectionID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		families := materializeForensicCapabilities(coverage, evidence)
		corpus, err := loadForensicQueryCorpus()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		summary := map[string]int{"families_total": len(families)}
		for _, family := range families {
			summary[family.Availability]++
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"query_capability_references":   refs,
			"query_capability_readiness":    readiness,
			"readiness_scope":               semanticContext.ScopeMode,
			"processor_readiness_authority": "No current worker receipt is wired to this endpoint; process-new readiness is unverified and fails closed.",
			"deployment_attestation":        "NOT_ATTESTED: describes this executing source candidate, not the previously accepted runtime image.",
			"catalog_version":               forensicCapabilityCatalogVersion,
			"query_corpus":                  corpus,
			"tenant_id":                     scope.TenantID,
			"collection_id":                 scope.CollectionID,
			"answer_policy": map[string]any{
				"exact_facts":        "Computed only from deterministic stores and parameterized query templates.",
				"semantic_facts":     "Returned only from retrieved KB evidence with citations and explicit limitations.",
				"pending_operations": "Blocked with an unavailable result; the planner must not simulate an adapter or model.",
				"no_data":            "Reported as no collection coverage, never converted into a negative real-world finding.",
			},
			"summary":      summary,
			"families":     families,
			"coverage":     coverage,
			"generated_at": time.Now().UTC(),
		})
	}
}

func loadEvidenceCapabilityCounts(r *http.Request, db *pgxpool.Pool, tenantID, collectionID string) ([]evidenceCapabilityCount, error) {
	if db == nil {
		return nil, nil
	}
	scope, err := bindForensicScope(r, tenantID, collectionID, r.URL.Query().Get("case_id"), "")
	if err != nil {
		return nil, err
	}
	rows, err := queryRows(r.Context(), db, tenantID, `
WITH evidence_counts AS (
  SELECT modality::text AS modality,
         detected_type::text AS detected_type,
         lower(extension) AS extension,
         count(*) AS evidence_count,
         count(*) FILTER (WHERE kb_entry_ref IS NOT NULL AND kb_entry_ref <> '') AS kb_ready_count,
         count(*) FILTER (WHERE records_batch_id IS NOT NULL) AS normalized_count
  FROM forensic.evidence_items
  WHERE tenant_id = $1 AND collection_id = $2
    AND ($3='' OR case_id=$3) AND ($4='' OR evidence_id=NULLIF($4,'')::uuid)
    AND ($5='' OR current_version_id=NULLIF($5,'')::uuid)
  GROUP BY modality, detected_type, extension
), artifact_counts AS (
  SELECT items.modality::text AS modality,
         items.detected_type::text AS detected_type,
         lower(items.extension) AS extension,
         count(*) AS derived_count,
         string_agg(DISTINCT artifacts.artifact_type, ',') AS artifact_types
  FROM forensic.evidence_items items
  JOIN forensic.derived_artifacts artifacts
    ON artifacts.tenant_id=items.tenant_id
   AND artifacts.collection_id=items.collection_id
   AND artifacts.evidence_id=items.evidence_id
   AND artifacts.version_id=items.current_version_id
  WHERE items.tenant_id = $1 AND items.collection_id = $2
    AND ($3='' OR items.case_id=$3) AND ($4='' OR items.evidence_id=NULLIF($4,'')::uuid)
    AND ($5='' OR items.current_version_id=NULLIF($5,'')::uuid)
    AND artifacts.processing_status='completed'
  GROUP BY items.modality, items.detected_type, items.extension
)
SELECT evidence_counts.*,
       coalesce(artifact_counts.derived_count, 0) AS derived_count,
       coalesce(artifact_counts.artifact_types, '') AS artifact_types
FROM evidence_counts
LEFT JOIN artifact_counts USING (modality, detected_type, extension)`, tenantID, collectionID, scope.CaseID, r.URL.Query().Get("evidence_id"), r.URL.Query().Get("evidence_version_id"))
	if err != nil {
		return nil, err
	}
	counts := make([]evidenceCapabilityCount, 0, len(rows))
	for _, row := range rows {
		counts = append(counts, evidenceCapabilityCount{
			Modality: stringValueAny(row["modality"]), DetectedType: stringValueAny(row["detected_type"]),
			Extension: stringValueAny(row["extension"]),
			Total:     valueAsInt(row["evidence_count"]), KBReady: valueAsInt(row["kb_ready_count"]),
			Normalized: valueAsInt(row["normalized_count"]), Derived: valueAsInt(row["derived_count"]),
			ArtifactTypes: capabilityUniqueSortedStrings(strings.Split(stringValueAny(row["artifact_types"]), ",")),
		})
	}
	return counts, nil
}

func materializeForensicCapabilities(coverage CoverageSummary, evidence []evidenceCapabilityCount) []forensicFamilyCapability {
	recordCounts := map[string]int64{}
	for _, family := range coverage.RecordFamiliesPresent {
		recordCounts[normalize(family.RecordType)] += family.Count
	}
	result := make([]forensicFamilyCapability, 0, len(forensicCapabilityDefinitions()))
	var referencePolicy QueryCapabilityReferencesV1
	_ = json.Unmarshal(queryCapabilityReferencesJSON, &referencePolicy)
	for _, definition := range forensicCapabilityDefinitions() {
		family := forensicFamilyCapability{forensicCapabilityDefinition: definition}
		for _, recordType := range definition.RecordTypes {
			family.IndexedRecords += recordCounts[normalize(recordType)]
		}
		for _, count := range evidence {
			if containsNormalized(definition.EvidenceModalities, count.Modality) || containsNormalized(definition.EvidenceTypes, count.DetectedType) || containsNormalized(definition.Formats, count.Extension) {
				family.RegisteredEvidence += count.Total
				family.KBReadyEvidence += count.KBReady
				// A media job also has a records batch. Only actual structured
				// input can supply this legacy deterministic readiness dimension.
				if count.Modality == "structured_records" {
					family.NormalizedEvidence += count.Normalized
				}
				family.DerivedArtifacts += count.Derived
				family.DerivedArtifactTypes = append(family.DerivedArtifactTypes, count.ArtifactTypes...)
			}
		}
		switch {
		case family.IndexedRecords > 0 || family.NormalizedEvidence > 0:
			family.Availability = "queryable"
			family.AvailabilityReason = "This collection has normalized records available for deterministic queries."
		case family.KBReadyEvidence > 0:
			family.Availability = "semantic_only"
			family.AvailabilityReason = "Evidence is retrievable from the Knowledge Base, but typed deterministic extraction is not available."
		case family.DerivedArtifacts > 0:
			family.Availability = "limited"
			family.AvailabilityReason = "Completed source-bound derived observations are available with review requirements and capability-specific limitations."
		case family.RegisteredEvidence > 0:
			family.Availability = "registered_pending"
			family.AvailabilityReason = "Raw evidence is registered and preserved, but the required accepted adapter or model is not operational."
		case definition.SupportLevel == "planned":
			family.Availability = "manual_review"
			family.AvailabilityReason = "The format requires deterministic inspection and operator disposition."
		default:
			family.Availability = "no_data"
			family.AvailabilityReason = "No matching evidence or normalized records are present in this collection."
		}
		family.DerivedArtifactTypes = capabilityUniqueSortedStrings(family.DerivedArtifactTypes)
		for _, unavailable := range referencePolicy.UnavailableIntents {
			if unavailable.Category == family.ID {
				family.Availability = "unavailable"
				family.AvailabilityReason = unavailable.Reason
				family.SuggestedQueries = nil
			}
		}
		family.SuggestedQueries = productCertifiedCapabilitySuggestions(availableCapabilitySuggestions(family))
		result = append(result, family)
	}
	return result
}

func productCertifiedCapabilitySuggestions(candidates []string) []string {
	operationByQuery := map[string]string{
		"show frequent contacts":             "cdr.frequent_contacts",
		"show hourly and daily CDR activity": "cdr.temporal_activity",
	}
	certificationByID := operationCertificationByID()
	visible := make([]string, 0, len(candidates))
	for _, query := range candidates {
		operationID, mapped := operationByQuery[query]
		certification, certified := certificationByID[operationID]
		if !mapped || !certified || !certification.SuggestionEligible || certification.CertificationLevel != certificationLevelProductCertified {
			continue
		}
		visible = append(visible, query)
	}
	return visible
}

func availableCapabilitySuggestions(family forensicFamilyCapability) []string {
	hasArtifact := func(contract string) bool { return containsString(family.DerivedArtifactTypes, contract) }
	switch family.ID {
	case "images_and_ocr":
		queries := []string{"show image metadata"}
		if hasArtifact("forensics.anpr-image-observation/v1") || hasArtifact("forensics.image-observation/v1") {
			queries = append(queries, "show recorded plate-region candidates")
		}
		if hasArtifact("forensics.image-ocr-observation/v1") {
			queries = append(queries, "show OCR regions with citations")
		}
		if hasArtifact("forensics.face-observation/v1") {
			queries = append(queries, "show candidate face observations")
		}
		if hasArtifact("forensics.image-embedding-observation/v1") {
			queries = append(queries, "find visually similar images within these evidence IDs")
		}
		return queries
	case "audio_and_stt":
		queries := []string{"show audio metadata"}
		if hasArtifact("forensics.audio-timestamp-segment/v1") {
			queries = append(queries, "show timestamped Urdu transcript")
		}
		if hasArtifact("forensics.audio-roman-urdu-segment/v1") {
			queries = append(queries, "find this phrase in the raw Urdu or Roman Urdu transcript")
		}
		return queries
	case "pdf_office_email_documents", "plain_text_notes":
		queries := []string{"show document metadata"}
		if family.ID == "plain_text_notes" {
			queries = []string{"which text files were ingested?"}
		}
		if hasArtifact("forensics.document-native-text-passage/v1") {
			queries = append(queries, "find cited native document evidence")
		}
		return queries
	default:
		return family.SuggestedQueries
	}
}

func assessQueryCapability(query, template string) queryCapabilityAssessment {
	q := strings.ToLower(strings.TrimSpace(query))
	assessment := queryCapabilityAssessment{
		Status:      "supported_by_selected_route",
		Explanation: "The selected deterministic or evidence route may run; collection coverage is reported separately.",
	}
	type pendingRule struct {
		family, operation, missing string
		keywords                   []string
		suggestions                []string
	}
	rules := []pendingRule{
		{"audio_and_stt", "speaker diarization", "accepted diarization adapter and speaker policy", []string{"diarize", "diarization", "speaker separation", "identify speaker"}, []string{"show audio metadata", "show timestamped transcript observations"}},
		{"images_and_ocr", "unbounded visual interpretation or identity recognition", "accepted object/event adapter or separately governed identity capability", []string{"describe image", "what is in the image", "recognize face", "identify person in image", "identify this face", "who is this person", "یہ شخص کون ہے", "ye shakhs kon hai"}, []string{"show image metadata", "show OCR observations", "show candidate face observations"}},
		{"video", "video content analysis", "accepted frame/audio timeline adapter and promoted models", []string{"analyze video", "describe video", "what happens in the video", "extract frames", "video timeline"}, []string{"show video metadata", "show registered video evidence"}},
		{"tts_artifacts", "text-to-speech generation", "approved voice policy and accepted TTS backend", []string{"text to speech", "tts", "speak this", "read this aloud", "generate voice", "generate speech", "synthesize audio", "پڑھ کر سنائیں", "parh kar sunao"}, []string{"show registered TTS artifacts"}},
		{"network_system_captures", "packet/session reconstruction", "bounded capture-to-session adapter", []string{"reconstruct session", "analyze pcap", "parse pcap", "packet payload"}, []string{"show capture inventory", "show capture metadata"}},
		{"databases_and_sql", "database row querying", "accepted read-only database query adapter", []string{"query sqlite", "query database", "run sql", "select from database"}, []string{"show database schema inventory", "show registered database evidence"}},
		{"archives_and_bundles", "archive extraction", "bounded archive child-registration adapter", []string{"extract archive", "unzip", "open rar", "recurse archive"}, []string{"show archive member inventory", "show registered archive evidence"}},
		{"spreadsheets_and_columnar", "unsafe or unsupported spreadsheet execution", "accepted non-executing adapter for this operation or format", []string{"evaluate formula", "recalculate formula", "recalculate workbook", "execute macro", "run macro", "open ods", "query ods", "legacy xls workbook", "query arrow", "query feather", "query avro", "query orc", "xml spreadsheet"}, []string{"read XLSX workbook rows", "show spreadsheet inventory", "query accepted Parquet records"}},
		{"pdf_office_email_documents", "scanned/table/unsupported document extraction", "accepted scanned-document OCR, table, or format adapter", []string{"read scanned pdf", "extract document table", "parse msg file", "extract ppt", "extract rtf"}, []string{"show document metadata", "find cited native document evidence"}},
		{"access_security_logs", "unsupported event-log decoding", "accepted EVTX decoder and source-validation contract", []string{"analyze evtx", "parse evtx", "read evtx"}, []string{"show registered log evidence"}},
		{"financial_transactions", "unsupported financial-message decoding", "accepted OFX or MT940 decoder", []string{"parse ofx", "read ofx", "analyze ofx", "parse mt940", "read mt940"}, []string{"show accepted structured transaction records"}},
		{"images_and_ocr", "unsupported image decoding", "accepted HEIC, raw-image, or SVG rasterization path", []string{"analyze heic", "read heic", "process svg", "analyze svg", "raw camera image"}, []string{"show registered image evidence"}},
		{"transcripts_and_subtitles", "unsupported standalone subtitle decoding", "accepted SRT/VTT/ASS cue parser", []string{"standalone srt", "parse srt", "parse vtt", "parse ass subtitles"}, []string{"show registered transcript evidence"}},
	}
	for _, rule := range rules {
		if containsAny(q, rule.keywords) {
			assessment.RequestedFamilies = append(assessment.RequestedFamilies, rule.family)
			assessment.RequiredOperations = append(assessment.RequiredOperations, rule.operation)
			assessment.MissingCapabilities = append(assessment.MissingCapabilities, rule.missing)
			assessment.SuggestedQueries = append(assessment.SuggestedQueries, rule.suggestions...)
		}
	}
	if len(assessment.MissingCapabilities) > 0 {
		assessment.Status = "unavailable"
		assessment.Explanation = "The request requires a processing capability that has not passed its acceptance and model-promotion gates. No substitute answer was generated."
	}
	assessment.RequestedFamilies = capabilityUniqueSortedStrings(assessment.RequestedFamilies)
	assessment.RequiredOperations = capabilityUniqueSortedStrings(assessment.RequiredOperations)
	assessment.MissingCapabilities = capabilityUniqueSortedStrings(assessment.MissingCapabilities)
	assessment.SuggestedQueries = capabilityUniqueSortedStrings(assessment.SuggestedQueries)
	return assessment
}

func containsNormalized(values []string, candidate string) bool {
	candidate = normalize(candidate)
	for _, value := range values {
		if normalize(value) == candidate && candidate != "" {
			return true
		}
	}
	return false
}

func capabilityUniqueSortedStrings(values []string) []string {
	set := map[string]struct{}{}
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			set[value] = struct{}{}
		}
	}
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func forensicCapabilityDefinitions() []forensicCapabilityDefinition {
	return []forensicCapabilityDefinition{
		{ID: "plain_text_notes", Label: "Plain text and notes", Formats: []string{"txt", "md", "yaml", "yml"}, SupportLevel: "limited", Adapter: "native_document_worker_txt_v1", EvidenceModalities: []string{"text"}, EvidenceTypes: []string{"text"}, DeterministicOperations: []string{"hash equality", "byte size", "bounded native-text extraction", "section-passage citation", "source and version lookup"}, SemanticOperations: []string{"retrieval", "citation", "bounded summary", "no-answer abstention"}, NextGate: "Run separately approved retained multilingual/noisy-note extraction, retrieval, citation and abstention acceptance.", SuggestedQueries: []string{"find cited case-note evidence", "which text files were ingested?"}},
		{ID: "logs_access_security", Label: "Logs and access/security events", Formats: []string{"log", "txt", "csv", "jsonl", "ndjson", "evtx"}, SupportLevel: "operational", Adapter: "access_log", RecordTypes: []string{"access_log"}, EvidenceTypes: []string{"access_log"}, DeterministicOperations: []string{"event counts", "user/IP/path filters", "time ranges", "failed-access sequences", "exact cross-family correlation with cited source rows"}, SemanticOperations: []string{"explain returned events", "correlate with case notes"}, NextGate: "EVTX, clock-drift, late-arrival, redaction, and incident-timeline goldens.", SuggestedQueries: []string{"show failed access events", "show access-log activity by hour", "correlate this IP across record families"}},
		{ID: "cdr", Label: "Call detail records", Formats: []string{"csv", "json", "jsonl", "ndjson", "parquet"}, SupportLevel: "operational", Adapter: "cdr", RecordTypes: []string{"cdr"}, EvidenceTypes: []string{"cdr"}, DeterministicOperations: []string{"raw and canonical Pakistan number roles", "source-timezone provenance", "service and sentinel classification", "exact call counts", "duration extrema", "frequent contacts", "time and cell filters", "direction and disposition", "exact cross-family correlation with cited source rows"}, SemanticOperations: []string{"bounded pattern explanation", "policy-context retrieval"}, NextGate: "Provider/sector reference history and deeper time-valid tower semantics.", SuggestedQueries: []string{"show frequent contacts", "show hourly and daily CDR activity", "show longest call duration", "show CDR records where call type is GPRS", "correlate this number across record families"}},
		{ID: "ipdr_network_sessions", Label: "IPDR and network sessions", Formats: []string{"csv", "json", "jsonl", "ndjson", "parquet", "pcap", "pcapng"}, SupportLevel: "operational", Adapter: "ipdr", RecordTypes: []string{"ipdr"}, EvidenceTypes: []string{"ipdr"}, DeterministicOperations: []string{"IPv4/IPv6 and NAT normalization", "session counts and durations", "IP/domain/port filters", "exact non-negative byte totals", "time overlap", "subscriber correlation", "exact cross-family correlation with cited source rows"}, SemanticOperations: []string{"bounded traffic-pattern explanation"}, NextGate: "Add provider-specific DNS/NAT schema packs, clock-drift/late-arrival cases, and accepted capture-to-session derivation.", SuggestedQueries: []string{"show IPDR records for this IP", "show network-session activity by hour", "correlate this IP across record families"}},
		{ID: "anpr_vehicle_sightings", Label: "ANPR and vehicle sightings", Formats: []string{"csv", "json", "jsonl", "ndjson", "parquet", "tsv", "xlsx", "png", "jpg", "jpeg", "webp", "bmp", "tif", "tiff"}, SupportLevel: "operational", Adapter: "anpr_and_optional_fastalpr_image_observer", RecordTypes: []string{"anpr"}, EvidenceTypes: []string{"anpr", "image"}, DeterministicOperations: []string{"exact cited sightings", "exact plate/camera/time filters", "camera sequence and activity", "five-minute same-camera co-observations", "consecutive-sighting elapsed time and straight-line distance", "observed plate variants", "formatting-only plate normalization", "image/version/crop provenance", "exact cross-family correlation with cited source rows"}, SemanticOperations: []string{"bounded movement narrative with explicit limitations"}, NextGate: "Deploy the opt-in FastALPR worker with explicitly mounted approved local models, then run representative adverse-condition imagery, camera clock-drift, throughput, and review-workflow goldens. Image OCR remains a review-required derived observation, not source truth.", SuggestedQueries: []string{"where was plate ABC-123 seen?", "show camera sequence for ABC-123", "show co-travel for ABC-123", "show route timing for ABC-123", "show plate variants for ABC-123"}},
		{ID: "subscriber_identity", Label: "Subscriber and identity records", Formats: []string{"csv", "json", "jsonl", "ndjson", "parquet", "tsv", "xlsx"}, SupportLevel: "operational", Adapter: "subscriber", RecordTypes: []string{"subscriber"}, EvidenceTypes: []string{"subscriber"}, DeterministicOperations: []string{"privacy-safe exact identifier lookup", "explicit validity timeline", "role-separated MSISDN/subscriber, IMSI/ICCID SIM, IMEI device, and service observations", "status and review summary", "conflict audit", "identifier reuse candidates", "exact cross-family correlation with cited source rows"}, SemanticOperations: []string{"bounded explanation of returned identity observations and conflicts"}, NextGate: "R7.6 adds synchronized family presentation; authorized provider-format benchmarks and field-level authorization remain before expanded PII display.", SuggestedQueries: []string{"look up subscriber identity observations for this MSISDN", "show subscriber validity timeline", "show subscriber device and service links", "show subscriber status summary", "show subscriber identity conflicts", "show subscriber identifier reuse candidates"}},
		{ID: "tower_location", Label: "Tower and location reference data", Formats: []string{"csv", "json", "jsonl", "ndjson", "geojson", "parquet", "tsv", "xlsx"}, SupportLevel: "operational", Adapter: "tower_location", RecordTypes: []string{"tower_location"}, EvidenceTypes: []string{"tower_location"}, DeterministicOperations: []string{"exact site/sector/LAC/TAC lookup", "time-aware reference timeline", "coordinate/datum/uncertainty audit", "status summary", "alias conflict audit", "exact time-aware CDR join", "exact cross-family correlation with cited source rows"}, SemanticOperations: []string{"bounded location-context explanation with RF and physical-presence limitations"}, NextGate: "Authorized provider drift packs, non-WGS84 transform validation, retained-data joins, map UI, and live specialist acceptance.", SuggestedQueries: []string{"look up tower/site reference observations", "show tower reference history", "audit tower coordinates and uncertainty", "show tower status summary", "audit tower alias conflicts", "join CDR observations to eligible tower references"}},
		{ID: "financial_transactions", Label: "Financial transactions", Formats: []string{"csv", "json", "jsonl", "parquet", "xlsx", "ofx", "mt940"}, SupportLevel: "operational", Adapter: "transaction", RecordTypes: []string{"transaction"}, EvidenceTypes: []string{"transaction"}, DeterministicOperations: []string{"exact sums", "currency-separated totals", "account flows", "time filters", "counterparty ranking", "duplicate transaction checks", "exact cross-family correlation with cited source rows"}, SemanticOperations: []string{"bounded anomaly explanation based on returned transactions"}, NextGate: "Authorized bank/wallet profiles, multi-currency provenance, reconciliation, OFX, MT940, and spreadsheet goldens.", SuggestedQueries: []string{"show transactions for this account", "show duplicate transaction records", "correlate this account across record families"}},
		{ID: "generic_tabular", Label: "Generic tabular records", Formats: []string{"csv", "json", "jsonl", "ndjson", "parquet", "tsv", "xlsx", "xml", "avro", "orc", "arrow", "feather"}, SupportLevel: "foundation", Adapter: "generic", RecordTypes: []string{"generic"}, EvidenceTypes: []string{"generic", "tabular_data"}, DeterministicOperations: []string{"schema discovery", "typed filters", "exact counts", "null profiling", "drift comparison", "exact cross-family correlation with cited source rows"}, SemanticOperations: []string{"schema explanation", "clarification"}, NextGate: "Schema-drift, encoding, delimiter, nested JSON, Parquet, and unknown-column goldens.", SuggestedQueries: []string{"show detected headers and schema", "show generic records where field exists", "correlate this identifier across record families"}},
		{ID: "spreadsheets_and_columnar", Label: "Spreadsheets and columnar files", Formats: []string{"tsv", "xls", "xlsx", "xlsm", "ods", "parquet", "arrow", "feather", "avro", "orc", "xml"}, SupportLevel: "foundation", Adapter: "tsv_xlsx_per_sheet_read_only_adapter_v1_then_spreadsheet_columnar_pending", EvidenceModalities: []string{"tabular"}, EvidenceTypes: []string{"delimited_text", "tabular_data", "spreadsheet"}, DeterministicOperations: []string{"TSV typed row extraction", "bounded read-only XLSX row extraction", "per-sheet typed mapping with generic preservation", "multi-sheet inventory and provenance", "hidden row/column/sheet markers", "merged-range inventory", "formula text and cached-value distinction", "exact source-value preservation"}, SemanticOperations: []string{"workbook structure explanation over normalized evidence"}, NextGate: "Add multi-provider schema-drift goldens, benchmark ODS and columnar formats, and keep formula recalculation/macros prohibited.", SuggestedQueries: []string{"show XLSX records with sheet and row provenance", "show each worksheet's mapping decision", "show spreadsheet inventory", "show formulas without evaluating them"}},
		{ID: "pdf_office_email_documents", Label: "PDF, office, email, and ebook documents", Formats: []string{"pdf", "doc", "docx", "rtf", "odt", "ppt", "pptx", "odp", "html", "htm", "epub", "eml", "msg"}, SupportLevel: "limited", Adapter: "native_document_worker_txt_pdf_docx_v1", EvidenceModalities: []string{"document"}, EvidenceTypes: []string{"pdf", "office_document", "email"}, DeterministicOperations: []string{"file/page count", "metadata", "attachment inventory", "bounded native-text extraction", "page/paragraph/section passage citation"}, SemanticOperations: []string{"retrieval", "grounded summary", "document comparison"}, NextGate: "Run separately approved retained TXT/PDF/DOCX extraction and cited retrieval. Scanned-document OCR, tables, geometry, active content, attachments, and other office formats remain unavailable.", SuggestedQueries: []string{"show document metadata", "find cited document evidence"}},
		{ID: "images_and_ocr", Label: "Images and OCR", Formats: []string{"png", "jpg", "jpeg", "webp", "gif", "bmp", "tif", "tiff", "heic", "heif", "dng", "raw", "svg"}, SupportLevel: "limited", Adapter: "unified_media_worker_with_optional_fastalpr_face_siglip_tesseract_roles", EvidenceModalities: []string{"image"}, EvidenceTypes: []string{"image"}, DeterministicOperations: []string{"immutable hash and byte accounting", "bounded signature and header validation", "dimensions and pixel budget", "orientation-only EXIF projection", "authorized retained-source preview", "exact SHA-256 comparison", "bounded dHash near-duplicate comparison", "typed original-pixel plate/face/OCR regions", "lossless crop hash and parent/version provenance", "explicit manual-review state"}, SemanticOperations: []string{"retrieval over persisted ANPR and OCR observations", "bounded explicit-candidate semantic image similarity", "English text-image evaluation with Urdu limitation", "questions over stored derived artifacts"}, NextGate: "Run separately approved retained image/OCR/semantic acceptance. General OCR and SigLIP remain LIMITED; Urdu text-image retrieval failed its first top-1 check. HEIC, raw and SVG decoding remains manual-review only.", SuggestedQueries: []string{"show image metadata", "show recorded plate-region candidates", "show OCR regions with citations", "compare these two images", "find visually similar images within these evidence IDs"}},
		{ID: "audio_and_stt", Label: "Audio and speech-to-text", Formats: []string{"wav", "mp3", "m4a", "flac", "ogg", "aac", "opus", "wma", "amr"}, SupportLevel: "limited", Adapter: "unified_media_worker_ffprobe_localai_asr_roman_urdu_v1", EvidenceModalities: []string{"audio"}, EvidenceTypes: []string{"audio"}, DeterministicOperations: []string{"duration", "codec", "sample rate", "channel count", "authorized retained-source playback", "timestamp-segment lineage", "raw-Urdu-authoritative Roman Urdu derivative", "identifier-preservation audit", "derived-artifact lineage"}, SemanticOperations: []string{"retrieval over persisted timestamped ASR and Roman Urdu observations", "grounded summary with time-range citations"}, NextGate: "Run separately approved retained Roman Urdu search/Ask/citation acceptance. Identifier speech remains an M2 limitation and diarization remains unavailable.", SuggestedQueries: []string{"show audio metadata", "show timestamped Urdu transcript", "find this phrase in the raw Urdu or Roman Urdu transcript"}},
		{ID: "tts_artifacts", Label: "Text-to-speech artifacts", Formats: []string{"wav", "mp3", "m4a", "flac", "ogg", "opus"}, SupportLevel: "foundation", Adapter: "tts_artifact_registry_with_deterministic_wav_metadata_v1", EvidenceTypes: []string{"tts_artifact"}, DeterministicOperations: []string{"parent linkage", "text hash", "duration", "codec", "backend/version lookup"}, SemanticOperations: []string{"optional accessibility playback"}, NextGate: "Consent/voice policy, pronunciation goldens, round-trip intelligibility, CPU latency, and accessible playback.", SuggestedQueries: []string{"show registered TTS artifacts"}},
		{ID: "transcripts_and_subtitles", Label: "Transcripts and subtitles", Formats: []string{"txt", "json", "srt", "vtt", "ass", "ssa"}, SupportLevel: "foundation", Adapter: "transcript_index_pending", EvidenceTypes: []string{"transcript", "subtitle"}, DeterministicOperations: []string{"cue count", "timestamp range", "speaker label filter", "parent evidence lookup"}, SemanticOperations: []string{"retrieval", "citation by cue and time range", "bounded summary"}, NextGate: "Subtitle parsing and cue-level KB indexing with parent evidence and time-range citations.", SuggestedQueries: []string{"show registered transcript evidence"}},
		{ID: "video", Label: "Video", Formats: []string{"mp4", "mov", "mkv", "avi", "webm", "m4v", "mpg", "mpeg", "mts", "m2ts", "3gp"}, SupportLevel: "foundation", Adapter: "unified_media_worker_ffprobe_bounded_frames", EvidenceModalities: []string{"video"}, EvidenceTypes: []string{"video"}, DeterministicOperations: []string{"duration", "stream inventory", "bounded frame sampling", "frame/time/evidence mapping", "optional image and ANPR observations", "authorized retained-source playback", "artifact timeline query"}, SemanticOperations: []string{"persisted observation retrieval; no event or transcript inference"}, NextGate: "Deploy and validate real-codec, corrupt and long-video goldens; add separately governed ASR, tracking/deduplication and timeline-query contracts.", SuggestedQueries: []string{"show video metadata", "show registered video evidence"}},
		{ID: "network_system_captures", Label: "Network and system captures", Formats: []string{"pcap", "pcapng", "cap", "evtx"}, SupportLevel: "foundation", Adapter: "deterministic_pcap_pcapng_inventory_v1_4_then_capture_adapter_pending", EvidenceModalities: []string{"capture"}, EvidenceTypes: []string{"pcap", "pcapng", "evtx"}, DeterministicOperations: []string{"safe metadata inventory", "packet/event counts", "session reconstruction", "protocol and time filters"}, SemanticOperations: []string{"bounded explanation over normalized sessions/events"}, NextGate: "Real capture goldens and bounded Ethernet/IP/TCP/UDP/DNS session derivatives; separate EVTX adapter.", SuggestedQueries: []string{"show capture inventory", "show registered capture evidence"}},
		{ID: "databases_and_sql", Label: "Databases and SQL dumps", Formats: []string{"sqlite", "sqlite3", "db", "sql"}, SupportLevel: "foundation", Adapter: "deterministic_sqlite_inventory_v1_3_then_database_adapter_pending", EvidenceModalities: []string{"database"}, EvidenceTypes: []string{"sqlite", "sql_dump"}, DeterministicOperations: []string{"read-only schema inventory", "table counts", "bounded queries", "foreign-key relationship extraction"}, SemanticOperations: []string{"schema explanation", "query clarification"}, NextGate: "Real SQLite encoding/WAL/schema/corruption goldens before a query adapter or SQL dump support.", SuggestedQueries: []string{"show database schema inventory", "show registered database evidence"}},
		{ID: "archives_and_bundles", Label: "Archives and bundles", Formats: []string{"zip", "7z", "rar", "tar", "gz", "tgz", "bz2", "xz"}, SupportLevel: "foundation", Adapter: "deterministic_zip_tar_inventory_v1_2_then_archive_extraction_pending", EvidenceModalities: []string{"archive"}, EvidenceTypes: []string{"archive", "zip", "tar"}, DeterministicOperations: []string{"member inventory", "member hash", "size and compression checks", "recursive child registration"}, SemanticOperations: []string{"bundle overview over registered children"}, NextGate: "ZIP64/PAX/GNU fixtures, bounded member hashing, and safe child registration before more formats.", SuggestedQueries: []string{"show archive member inventory", "show registered archive evidence"}},
		{ID: "unknown_and_mixed", Label: "Unknown and mixed evidence", Formats: []string{"unknown extension", "binary dump", "mixed export", "unsupported MIME"}, SupportLevel: "planned", Adapter: "manual_review", EvidenceModalities: []string{"unknown"}, EvidenceTypes: []string{"unknown"}, DeterministicOperations: []string{"hash", "size", "signature sampling", "duplicate detection", "operator disposition"}, SemanticOperations: []string{"classification explanation after deterministic inspection"}, NextGate: "Ambiguous-signature, renamed-extension, polyglot, corrupt, and mixed-bundle safety fixtures.", SuggestedQueries: []string{"show unclassified evidence", "show evidence requiring manual review"}},
	}
}
