package main

import (
	"fmt"
	"sort"
	"strings"
)

type CapabilityReasonCode string

const (
	CapabilityReasonMissingData          CapabilityReasonCode = "missing_data"
	CapabilityReasonUnauthorized         CapabilityReasonCode = "unauthorized"
	CapabilityReasonRuntimeUnavailable   CapabilityReasonCode = "runtime_unavailable"
	CapabilityReasonModelUnavailable     CapabilityReasonCode = "model_unavailable"
	CapabilityReasonProcessingIncomplete CapabilityReasonCode = "processing_incomplete"
	CapabilityReasonMaturityInsufficient CapabilityReasonCode = "maturity_insufficient"
	CapabilityReasonUnsupported          CapabilityReasonCode = "unsupported"
	CapabilityReasonInvalidParameter     CapabilityReasonCode = "invalid_parameter"
)

type CapabilityAvailabilityV1 struct {
	Implemented      bool                   `json:"implemented"`
	RuntimeAvailable bool                   `json:"runtime_available"`
	DataAvailable    bool                   `json:"data_available"`
	Authorized       bool                   `json:"authorized"`
	ModelAvailable   bool                   `json:"model_available"`
	Maturity         string                 `json:"maturity"`
	Queryable        bool                   `json:"queryable"`
	Executable       bool                   `json:"executable"`
	ReasonCodes      []CapabilityReasonCode `json:"reason_codes"`
}

type ResolvedCapabilityV1 struct {
	CapabilityID        string                   `json:"capability_id"`
	OperationID         string                   `json:"operation_id"`
	Template            string                   `json:"template"`
	FamilyIDs           []string                 `json:"family_ids"`
	IntentClasses       []QueryIntentClass       `json:"intent_classes"`
	RequiredParameters  []string                 `json:"required_parameters"`
	OptionalParameters  []string                 `json:"optional_parameters"`
	ScopeMode           string                   `json:"scope_mode"`
	SourceScoped        bool                     `json:"source_scoped"`
	TimeScoped          bool                     `json:"time_scoped"`
	ExecutionMode       string                   `json:"execution_mode"`
	ExtensionMode       string                   `json:"extension_mode"`
	ImplementationKey   string                   `json:"implementation_key"`
	SpecialistID        string                   `json:"specialist_id"`
	ModelRoleIDs        []string                 `json:"model_role_ids"`
	ResultContract      string                   `json:"result_contract"`
	ResultKind          string                   `json:"result_kind"`
	PresentationType    string                   `json:"presentation_type"`
	CitationRequired    bool                     `json:"citation_required"`
	CriticalityTier     string                   `json:"criticality_tier"`
	CertificationStatus string                   `json:"certification_status"`
	ExposureStatus      string                   `json:"exposure_status"`
	SuggestionEligible  bool                     `json:"suggestion_eligible"`
	Limitations         []string                 `json:"limitations"`
	Availability        CapabilityAvailabilityV1 `json:"availability"`
}

type WorkspaceCapabilityStateV1 struct {
	IndexedRecords     int64
	RegisteredEvidence int64
	KBReadyEvidence    int64
	NormalizedEvidence int64
	DerivedArtifacts   int64
	ProcessingEvidence int64
	ArtifactTypes      []string
}

type CapabilityProjectionContextV1 struct {
	Authorized                     bool
	RuntimeAvailable               bool
	FamilyStates                   map[string]WorkspaceCapabilityStateV1
	ModelRoles                     map[string]bool
	RequiredModelRolesByCapability map[string][]string
	MaturityByCapability           map[string]string
}

type CapabilityResolutionV1 struct {
	Candidates []ResolvedCapabilityV1 `json:"candidates"`
	Rejected   []ResolvedCapabilityV1 `json:"rejected"`
}

func buildResolvedCapabilityProjection(ctx CapabilityProjectionContextV1) ([]ResolvedCapabilityV1, error) {
	templates := supportedQueryTemplates()
	certifications := operationCertificationByID()
	projection := make([]ResolvedCapabilityV1, 0, len(templates))
	seen := map[string]string{}
	for _, template := range templates {
		if template.OperationID == "" || !validCapabilityID(template.OperationID) {
			return nil, fmt.Errorf("template %q has invalid operation ID %q", template.Name, template.OperationID)
		}
		if previous, exists := seen[template.OperationID]; exists {
			return nil, fmt.Errorf("templates %q and %q share operation ID %q", previous, template.Name, template.OperationID)
		}
		seen[template.OperationID] = template.Name
		capability := capabilityFromTemplateWithCertification(template, ctx, certifications[template.OperationID])
		projection = append(projection, capability)
	}
	sort.Slice(projection, func(i, j int) bool { return projection[i].CapabilityID < projection[j].CapabilityID })
	return projection, nil
}

func queryTemplateByName(name string) (queryTemplateCatalogEntry, bool) {
	for _, template := range supportedQueryTemplates() {
		if template.Name == name {
			return template, true
		}
	}
	return queryTemplateCatalogEntry{}, false
}

func queryTemplateNameByOperationID(operationID string) string {
	for _, template := range supportedQueryTemplates() {
		if template.OperationID == operationID {
			return template.Name
		}
	}
	return ""
}

func capabilityFromTemplate(template queryTemplateCatalogEntry, ctx CapabilityProjectionContextV1) ResolvedCapabilityV1 {
	return capabilityFromTemplateWithCertification(template, ctx, operationCertificationByID()[template.OperationID])
}

func capabilityFromTemplateWithCertification(template queryTemplateCatalogEntry, ctx CapabilityProjectionContextV1, certification OperationCertificationEntryV1) ResolvedCapabilityV1 {
	required := []string{}
	optional := []string{}
	for _, input := range template.Inputs {
		if input.Required {
			required = append(required, input.Name)
		} else {
			optional = append(optional, input.Name)
		}
	}
	// Source-file and batch predicates are currently enforced only by the
	// canonical-records executor. Do not advertise them for analytical
	// templates whose SQL does not consume those request fields.
	if template.Name == "canonical_records" {
		optional = append(optional, "source_file", "batch_id")
	}
	families := queryUnderstandingFamilies(template)
	familyIDs := uniqueStrings(append([]string{families.Primary}, families.Secondary...))
	modelRoles := append([]string(nil), ctx.RequiredModelRolesByCapability[template.OperationID]...)
	maturity := defaultString(ctx.MaturityByCapability[template.OperationID], "operational")
	availability := resolveCapabilityAvailability(template, ctx, modelRoles, maturity)
	if certification.ExposureStatus == "engineering_only" {
		availability.Queryable = false
	}
	return ResolvedCapabilityV1{
		CapabilityID: template.OperationID, OperationID: template.OperationID, Template: template.Name,
		FamilyIDs: familyIDs, IntentClasses: []QueryIntentClass{durableIntentForTemplate(template)},
		RequiredParameters: required, OptionalParameters: optional,
		ScopeMode: template.ScopeMode, SourceScoped: containsString(optional, "source_file") || containsString(optional, "source_set") || containsString(required, "source_set"), TimeScoped: containsString(optional, "date_from") || containsString(required, "date_from"),
		ExecutionMode: executionModeForRoute(template.Route), ExtensionMode: extensionExecutionMode(template.Route), ImplementationKey: implementationKeyForRoute(template.Route),
		SpecialistID: specialistForCapabilityFamily(template.FamilyID), ModelRoleIDs: modelRoles,
		ResultContract: "forensics.enterprise-response/v1", ResultKind: resultKindForTemplate(template), PresentationType: template.Presentation,
		CitationRequired: true, Limitations: append([]string(nil), template.Limitations...), Availability: availability,
		CriticalityTier: certification.CriticalityTier, CertificationStatus: certification.OverallStatus,
		ExposureStatus: certification.ExposureStatus, SuggestionEligible: certification.SuggestionEligible,
	}
}

func resolveCapabilityAvailability(template queryTemplateCatalogEntry, ctx CapabilityProjectionContextV1, requiredModelRoles []string, maturity string) CapabilityAvailabilityV1 {
	availability := CapabilityAvailabilityV1{
		Implemented: true, RuntimeAvailable: ctx.RuntimeAvailable, Authorized: ctx.Authorized,
		ModelAvailable: true, Maturity: maturity, Queryable: true,
	}
	for _, role := range requiredModelRoles {
		if !ctx.ModelRoles[role] {
			availability.ModelAvailable = false
			break
		}
	}
	states := statesForTemplate(template, ctx.FamilyStates)
	var indexed, registered, kbReady, normalized, derived, processing int64
	for _, state := range states {
		indexed += state.IndexedRecords
		registered += state.RegisteredEvidence
		kbReady += state.KBReadyEvidence
		normalized += state.NormalizedEvidence
		derived += state.DerivedArtifacts
		processing += state.ProcessingEvidence
	}
	switch template.Route {
	case "kb":
		availability.DataAvailable = kbReady > 0
	case "derived":
		for _, state := range states {
			for _, contract := range derivedTextContractsForTemplate(template.Name) {
				if containsString(state.ArtifactTypes, contract) {
					availability.DataAvailable = true
				}
			}
		}
	case "hybrid":
		availability.DataAvailable = indexed > 0 || normalized > 0 || kbReady > 0
	default:
		if templateNeedsCatalogEvidence(template.Name) {
			availability.DataAvailable = indexed > 0 || normalized > 0 || registered > 0
		} else {
			availability.DataAvailable = indexed > 0 || normalized > 0
		}
	}
	if !availability.RuntimeAvailable {
		availability.ReasonCodes = append(availability.ReasonCodes, CapabilityReasonRuntimeUnavailable)
	}
	if !availability.Authorized {
		availability.ReasonCodes = append(availability.ReasonCodes, CapabilityReasonUnauthorized)
	}
	if !availability.DataAvailable {
		if processing > 0 || registered > 0 && normalized == 0 && kbReady == 0 {
			availability.ReasonCodes = append(availability.ReasonCodes, CapabilityReasonProcessingIncomplete)
		} else {
			availability.ReasonCodes = append(availability.ReasonCodes, CapabilityReasonMissingData)
		}
	}
	if !availability.ModelAvailable {
		availability.ReasonCodes = append(availability.ReasonCodes, CapabilityReasonModelUnavailable)
	}
	if availability.Maturity != "operational" {
		availability.ReasonCodes = append(availability.ReasonCodes, CapabilityReasonMaturityInsufficient)
	}
	availability.Executable = availability.Implemented && availability.RuntimeAvailable && availability.DataAvailable && availability.Authorized && availability.ModelAvailable && availability.Maturity == "operational"
	availability.Queryable = availability.Executable
	return availability
}

func resolveCapabilities(understanding QueryUnderstandingV1, projection []ResolvedCapabilityV1) CapabilityResolutionV1 {
	wanted := map[string]struct{}{}
	for _, capabilityID := range understanding.CandidateCapabilities {
		wanted[capabilityID] = struct{}{}
	}
	resolution := CapabilityResolutionV1{Candidates: []ResolvedCapabilityV1{}, Rejected: []ResolvedCapabilityV1{}}
	for _, capability := range projection {
		if _, selected := wanted[capability.CapabilityID]; !selected {
			continue
		}
		if capability.Availability.Executable {
			resolution.Candidates = append(resolution.Candidates, capability)
		} else {
			resolution.Rejected = append(resolution.Rejected, capability)
		}
	}
	return resolution
}

func projectionContextFromWorkspace(families []forensicFamilyCapability, authorized, runtimeAvailable bool) CapabilityProjectionContextV1 {
	ctx := CapabilityProjectionContextV1{
		Authorized: authorized, RuntimeAvailable: runtimeAvailable,
		FamilyStates: map[string]WorkspaceCapabilityStateV1{}, ModelRoles: map[string]bool{},
		RequiredModelRolesByCapability: map[string][]string{}, MaturityByCapability: map[string]string{},
	}
	for _, family := range families {
		ctx.FamilyStates[family.ID] = WorkspaceCapabilityStateV1{
			IndexedRecords: family.IndexedRecords, RegisteredEvidence: family.RegisteredEvidence,
			KBReadyEvidence: family.KBReadyEvidence, NormalizedEvidence: family.NormalizedEvidence,
			DerivedArtifacts: family.DerivedArtifacts,
			ArtifactTypes:    family.DerivedArtifactTypes,
		}
	}
	return ctx
}

func statesForTemplate(template queryTemplateCatalogEntry, states map[string]WorkspaceCapabilityStateV1) []WorkspaceCapabilityStateV1 {
	keys := []string{capabilityFamilyForTemplate(template)}
	switch template.FamilyID {
	case "document_intelligence":
		keys = []string{"plain_text_notes", "pdf_office_email_documents"}
	case "image_intelligence":
		keys = []string{"images_and_ocr"}
	case "audio_intelligence":
		keys = []string{"audio_and_stt"}
	case "video_intelligence":
		keys = []string{"video"}
	}
	if template.FamilyID == "case_cross_family" || containsString(template.RecordTypes, "all") || len(template.RecordTypes) > 1 {
		keys = keys[:0]
		for _, recordType := range template.RecordTypes {
			if key := familyStateKeyForRecordType(recordType); key != "" {
				keys = append(keys, key)
			}
		}
		if len(keys) == 0 || containsString(template.RecordTypes, "all") {
			keys = make([]string, 0, len(states))
			for key := range states {
				keys = append(keys, key)
			}
		}
	}
	result := make([]WorkspaceCapabilityStateV1, 0, len(keys))
	for _, key := range uniqueStrings(keys) {
		if state, exists := states[key]; exists {
			result = append(result, state)
		}
	}
	return result
}

func familyStateKeyForRecordType(value string) string {
	switch normalize(value) {
	case "cdr":
		return "cdr"
	case "ipdr":
		return "ipdr_network_sessions"
	case "anpr":
		return "anpr_vehicle_sightings"
	case "subscriber":
		return "subscriber_identity"
	case "tower_location":
		return "tower_location"
	case "transaction":
		return "financial_transactions"
	case "access_log":
		return "logs_access_security"
	case "generic":
		return "generic_tabular"
	case "document", "text":
		return "pdf_office_email_documents"
	case "image":
		return "images_and_ocr"
	case "audio":
		return "audio_and_stt"
	case "video":
		return "video"
	default:
		return ""
	}
}

func templateNeedsCatalogEvidence(name string) bool {
	return containsString([]string{"collection_overview", "schema_profile", "data_quality", "source_file_audit", "duplicate_upload_audit", "case_readiness", "evidence_package_summary", "executive_case_brief", "court_ready_source_summary", "limitations_and_data_quality", "document_metadata", "image_metadata", "face_candidate_observations", "audio_metadata", "video_metadata", "video_timeline"}, name)
}

func executionModeForRoute(route string) string {
	switch route {
	case "kb", "derived":
		return "retrieval"
	case "hybrid":
		return "legacy_hybrid"
	default:
		return "deterministic"
	}
}

func implementationKeyForRoute(route string) string {
	switch route {
	case "kb":
		return "queryKnowledgeBaseEvidence"
	case "derived":
		return "derivedTextEvidence"
	case "hybrid":
		return "hybridQueryHandler"
	default:
		return "runAnalyticalTemplate"
	}
}

func specialistForCapabilityFamily(family string) string {
	switch family {
	case "communications_cdr":
		return "Communications_CDR_Analyst"
	case "network_ipdr":
		return "Network_IPDR_Capture_Analyst"
	case "anpr_vehicles":
		return "Vehicle_ANPR_Geospatial_Analyst"
	case "subscriber_identity":
		return "Subscriber_Identity_Analyst"
	case "tower_location":
		return "Tower_Location_Reference_Analyst"
	case "knowledge_evidence":
		return "Evidence_Integrity_And_Provenance_Analyst"
	case "financial_transactions":
		return "Financial_Transactions_Analyst"
	case "access_security_logs":
		return "Access_Security_Log_Analyst"
	case "generic_tabular":
		return "Generic_Data_Quality_Analyst"
	case "document_intelligence":
		return "Document_OCR_Analyst"
	case "image_intelligence":
		return "Image_Vision_Analyst"
	case "face_intelligence":
		return "Image_Vision_Analyst"
	case "audio_intelligence":
		return "Audio_Speech_Analyst"
	case "video_intelligence":
		return "Video_Timeline_Analyst"
	default:
		return "Forensic_Records_Analyst"
	}
}

func reconciledForensicPlatformCatalog(catalog ForensicPlatformCatalogV1) (ForensicPlatformCatalogV1, error) {
	byID := make(map[string]struct{}, len(catalog.Operations))
	for index, operation := range catalog.Operations {
		if catalog.Operations[index].Maturity == "" {
			catalog.Operations[index].Maturity = defaultOperationMaturity(operation.FamilyID)
		}
		byID[operation.ID] = struct{}{}
	}
	for _, template := range supportedQueryTemplates() {
		if _, exists := byID[template.OperationID]; exists {
			continue
		}
		catalog.Operations = append(catalog.Operations, FamilyOperationDescriptorV1{
			ID: template.OperationID, Version: "1.0.0", FamilyID: template.FamilyID,
			Kind: executionModeForRoute(template.Route), Implementation: implementationKeyForRoute(template.Route),
			RequiredTools: []string{sourceToolForRoute(template.Route)}, InputContract: queryUnderstandingContractV1,
			OutputContract: "forensics.enterprise-response/v1", CitationRequired: true,
			Maturity: operationMaturityForTemplate(template),
		})
		byID[template.OperationID] = struct{}{}
	}
	sort.Slice(catalog.Operations, func(i, j int) bool { return catalog.Operations[i].ID < catalog.Operations[j].ID })
	catalog.CatalogVersion = "2026-08-28.nxb1.1"
	for _, template := range supportedQueryTemplates() {
		if _, exists := byID[template.OperationID]; !exists {
			return catalog, fmt.Errorf("executable operation %q has no platform descriptor", template.OperationID)
		}
	}
	return catalog, nil
}

func operationMaturityForTemplate(template queryTemplateCatalogEntry) string {
	if template.CertificationStatus == "certified" || template.CertificationStatus == "certified_with_limitation" {
		return "CERTIFIED"
	}
	return "LIMITED"
}

func defaultOperationMaturity(familyID string) string {
	if containsString([]string{
		"communications_cdr", "network_ipdr", "anpr_vehicles", "subscriber_identity",
		"tower_location", "generic_tabular", "case_cross_family", "knowledge_evidence",
	}, familyID) {
		return "CERTIFIED"
	}
	return "LIMITED"
}

func sourceToolForRoute(route string) string {
	switch route {
	case "kb":
		return "kb_retrieval"
	case "derived":
		return "derived_text_lexical"
	case "hybrid":
		return "records_sql+kb_retrieval"
	default:
		return "records_sql"
	}
}

func capabilityRejectionSummary(capability ResolvedCapabilityV1) string {
	parts := make([]string, 0, len(capability.Availability.ReasonCodes))
	for _, reason := range capability.Availability.ReasonCodes {
		parts = append(parts, string(reason))
	}
	return strings.Join(parts, ",")
}
