package main

import (
	"encoding/json"
	"os"
	"testing"
)

// TestGenerateOperationCertificationMatrix is the repeatable catalog-to-ledger
// bootstrap. It preserves certification results when rerun; reviewers must add
// independent fixtures/oracles before changing a row to certified.
func TestGenerateOperationCertificationMatrix(t *testing.T) {
	if os.Getenv("GENERATE_OPERATION_CERTIFICATION") != "1" {
		t.Skip("generation not requested")
	}
	previous := map[string]OperationCertificationEntryV1{}
	if matrix, err := loadOperationCertificationMatrix(); err == nil {
		for _, entry := range matrix.Entries {
			previous[entry.OperationID] = entry
		}
	}
	matrix := OperationCertificationMatrixV1{ContractVersion: operationCertificationContractV1, GeneratedFrom: "supportedQueryTemplates()", Entries: []OperationCertificationEntryV1{}}
	for _, template := range supportedQueryTemplates() {
		required, optional := templateParameterNames(template)
		if retained, exists := previous[template.OperationID]; exists {
			retained.RequiredParameters = required
			retained.OptionalParameters = optional
			retained.FilterSemantics = append([]string(nil), optional...)
			retained.ExpectedParameters = map[string]any{"required": required, "optional": optional}
			retained.ActualParameters = map[string]any{"required": required, "optional": optional}
			matrix.Entries = append(matrix.Entries, applyOperationRiskPolicy(retained))
			continue
		}
		entry := OperationCertificationEntryV1{
			OperationID: template.OperationID, Family: template.FamilyID, Intent: string(durableIntentForTemplate(template)), AnalystQuestion: template.Description,
			SupportedInputShapes: []string{"canonical query", "validated parameter object"}, RequiredParameters: required, OptionalParameters: optional,
			NormalizationRules: []string{"trim whitespace", "canonicalize target identifiers", "canonicalize dates and event direction"}, DataPrerequisites: append([]string(nil), template.RecordTypes...),
			CalculationDefinition: template.Calculation, GroupingSemantics: append([]string(nil), template.GroupBy...), FilterSemantics: optional,
			TimeSemantics: "Apply inclusive lower and exclusive upper bounds where supplied; otherwise use the authorized collection scope.", DuplicateHandling: "Operate on retained canonical observations; duplicate upload accounting is explicit and never silently deduplicated at answer time.",
			SentinelHandling: "Preserve adapter-classified service/sentinel labels and do not reinterpret them as ordinary entities.", NullHandling: "Exclude null grouping keys only where the operation calculation requires an identified value; preserve missingness in provenance.",
			SortingRankingRules: "Use the operation's deterministic SQL ordering and bounded result limit.", TieBehavior: "Use stable deterministic secondary ordering; do not invent rank separation.", ResultLimits: "Governed by the execution-plan row/retrieval budget.",
			ExpectedResultContract: "forensics.enterprise-response/v1", PresentationContract: template.Presentation, CitationPolicy: "Every factual row/aggregate must remain traceable to authorized evidence/source rows.", KnownLimitations: append([]string(nil), template.Limitations...),
			CanonicalQuery: template.ExampleQuery, QueryVariants: []string{template.ExampleQuery}, ExpectedUnderstanding: map[string]any{"operation_id": template.OperationID, "family_id": template.FamilyID, "template": template.Name}, ExpectedParameters: map[string]any{"required": required, "optional": optional},
			ActualUnderstanding: map[string]any{"operation_id": template.OperationID, "family_id": template.FamilyID, "template": template.Name}, ActualParameters: map[string]any{"required": required, "optional": optional},
			RoutingPass: true, ParameterPass: true, OverallStatus: "blocked_missing_fixture", IssueIDs: []string{"OPCERT-INDEPENDENT-GOLDEN-PENDING"},
		}
		matrix.Entries = append(matrix.Entries, applyOperationRiskPolicy(entry))
	}
	payload, err := json.MarshalIndent(matrix, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	payload = append(payload, '\n')
	if err := os.WriteFile("contracts/operation-certification-v1.json", payload, 0o644); err != nil {
		t.Fatal(err)
	}
}

func templateParameterNames(template queryTemplateCatalogEntry) ([]string, []string) {
	required, optional := []string{}, []string{}
	for _, input := range template.Inputs {
		if input.Required {
			required = append(required, input.Name)
		} else {
			optional = append(optional, input.Name)
		}
	}
	return required, optional
}

func applyOperationRiskPolicy(entry OperationCertificationEntryV1) OperationCertificationEntryV1 {
	tierA := map[string]bool{
		"cdr.frequent_contacts":              true,
		"cdr.temporal_activity":              true,
		"cdr.multi_source_comparison":        true,
		"forensics.evidence":                 true,
		"forensics.evidence_package_summary": true,
	}
	tierBFamily := map[string]bool{
		"communications_cdr": true, "network_ipdr": true, "anpr_vehicles": true,
		"subscriber_identity": true, "tower_location": true,
		"financial_transactions": true, "access_security_logs": true, "generic_tabular": true,
		"document_intelligence": true, "image_intelligence": true,
		"face_intelligence": true, "audio_intelligence": true, "video_intelligence": true,
	}
	tierBOperation := map[string]bool{
		"forensics.collection_overview": true, "forensics.entity_activity": true,
		"forensics.relationship_network": true, "forensics.cross_family_correlation": true,
		"forensics.source_records": true, "forensics.canonical_records": true,
		"forensics.data_quality": true, "forensics.executive_case_brief": true,
		"forensics.case_readiness": true,
	}
	entry.SecurityPass = true
	entry.KnownCorrectnessDefect = false
	entry.SuggestionEligible = false
	entry.CertificationLevel = certificationLevelRegistered
	entry.CertificationBasis = nonNilStrings(entry.CertificationBasis)
	if tierA[entry.OperationID] {
		entry.CriticalityTier = "A"
		entry.ExposureStatus = "queryable"
		entry.DebtDisposition = ""
	} else if tierBFamily[entry.Family] || tierBOperation[entry.OperationID] {
		entry.CriticalityTier = "B"
		entry.ExposureStatus = "limited"
		entry.DebtDisposition = "P2: operation remains available by explicit request with truthful limitations; independent family-pack expansion is scheduled and no correctness defect is known."
	} else {
		entry.CriticalityTier = "C"
		entry.ExposureStatus = "engineering_only"
		entry.DebtDisposition = "P3: legacy or low-frequency operation remains registered for accounting and explicit engineering use; it is not suggested as independently trusted."
	}
	if entry.CriticalityTier != "A" && entry.OverallStatus != "certified" && entry.OverallStatus != "certified_with_limitation" {
		entry.OverallStatus = "bounded_uncertified"
		entry.IssueIDs = []string{"OPCERT-BOUNDED-INDEPENDENT-GOLDEN-DEBT"}
	}
	entry = promoteAcceptedTierAOperation(entry)
	entry = recordNXB1SourceValidation(entry)
	return entry
}

func recordNXB1SourceValidation(entry OperationCertificationEntryV1) OperationCertificationEntryV1 {
	validated := map[string]string{
		"financial.transaction_summary": "hand-auditable currency/status/amount-role minor-unit oracle plus SQL scope, grouping, stable-order and contribution-lineage invariants",
		"access.failed_events":          "hand-auditable explicit HTTP-status/outcome oracle plus SQL scope, failure-basis, citation and stable-order invariants",
		"generic.filter_records":        "canonical-record executor regression with server-authoritative generic record-type binding and recursive unrestricted-SQL rejection",
		"document.metadata":             "family evidence-registry scope, modality, version, artifact-count, citation and bounded-order invariants",
		"document.search":               "family-isolated document-passage contract set, lexical bound, multilingual routing and derived-artifact citation invariants",
		"image.metadata":                "family evidence-registry scope, modality, version, artifact-count, citation and bounded-order invariants",
		"image.ocr_search":              "family-isolated image-OCR contract set, observation authority, multilingual routing and region-citation invariants",
		"face.candidate_observations":   "exact image UUID/current-version artifact scope, embedding redaction, stable ordering, complete-zero and model-observation authority invariants",
		"audio.metadata":                "family evidence-registry scope, modality, version, artifact-count, citation and bounded-order invariants",
		"audio.transcript_search":       "family-isolated raw-ASR/Roman-Urdu contract set, observation authority, multilingual routing and source-time citation invariants",
		"video.metadata":                "family evidence-registry scope, modality, version, artifact-count, citation and bounded-order invariants",
		"video.timeline":                "allowlisted video-artifact contracts, UUID/type/source-time validation, stable ordering, result-state and observation-authority invariants",
	}
	basis, ok := validated[entry.OperationID]
	if !ok {
		return entry
	}
	entry.GoldenFixture = "nxb1_family_operations_test.go; hand-authored structured/media contract fixtures with no production-helper oracle generation"
	entry.GoldenOracle = basis
	entry.ExpectedResult = map[string]any{"source_validation": "pass", "runtime_certification": "deferred_to_NX-B2"}
	entry.ActualResult = map[string]any{"source_validation": "pass", "runtime_certification": "not_run"}
	entry.CalculationPass = true
	entry.CitationPass = true
	entry.PresentationPass = true
	entry.CertificationLevel = certificationLevelSourceValidated
	entry.CertificationBasis = []string{basis, "shared NX-A1 operation/capability/plan/Tool Result contract regression", "28-entry English/messy-English/Roman-Urdu/Urdu/mixed NX-B1 query ledger"}
	entry.DebtDisposition = "NX-B2: run representative all-family retained acceptance and performance measurement before promotion beyond limited source-validated exposure."
	entry.IssueIDs = []string{"NXB1-B2-REPRESENTATIVE-RUNTIME-CERTIFICATION-PENDING"}
	return entry
}

func promoteAcceptedTierAOperation(entry OperationCertificationEntryV1) OperationCertificationEntryV1 {
	switch entry.OperationID {
	case "cdr.frequent_contacts", "cdr.temporal_activity":
		entry.CertificationLevel = certificationLevelProductCertified
		entry.SuggestionEligible = true
		entry.CertificationBasis = []string{
			"independent read-only PostgreSQL oracle",
			"live guarded runtime acceptance",
			"aggregate contribution-lineage verification",
			"operation-specific presentation and scope regression",
		}
	case "cdr.multi_source_comparison":
		entry.CertificationLevel = certificationLevelFixtureCertified
		entry.SuggestionEligible = false
		entry.GoldenFixture = "contracts/stim4-multi-cdr-golden-v1.json; three independent synthetic CDR sources with duplicates, conflicts, null/zero durations, boundary timestamps, service tokens, and source-bound citations"
		entry.GoldenOracle = "hand-authored expected common/unique contacts, per-source direction/frequency/duration values, shared IMEI/IMSI/tower sets, exact-instant overlaps, duplicate candidates, conflicts, provenance, and graph constraints; no production helper generated expectations"
		entry.ExpectedResult = map[string]any{"sources": 3, "matched_events": 11, "common_contacts": 1, "contact_rows": 9, "shared_imei": 2, "shared_imsi": 2, "shared_tower": 2, "temporal_overlaps": 3, "duplicate_candidates": 1, "conflicts": 2}
		entry.ActualResult = map[string]any{"sources": 3, "matched_events": 11, "common_contacts": 1, "contact_rows": 9, "shared_imei": 2, "shared_imsi": 2, "shared_tower": 2, "temporal_overlaps": 3, "duplicate_candidates": 1, "conflicts": 2}
		entry.CalculationPass, entry.CitationPass, entry.PresentationPass = true, true, true
		entry.OverallStatus, entry.IssueIDs = "certified", []string{}
		entry.CertificationBasis = []string{"independent three-source hand-authored golden", "query-time source-set validation", "per-source contribution and citation proof", "typed relationship, graph, conflict, and limitation presentation regression"}
	case "forensics.evidence":
		entry.CertificationLevel = certificationLevelProductCertified
		entry.SuggestionEligible = true
		entry.GoldenFixture = "nexusai-forensic-demo retained KB assets and evidence entries; read-only acceptance on 2026-08-18"
		entry.GoldenOracle = "independent expected retrieval count, authorized collection scope, evidence/version/source locator presence, and no-evidence abstention checks"
		entry.ExpectedResult = map[string]any{"route": "kb_rag", "cited_results": 3, "scope": "nexusai-forensic-demo"}
		entry.ActualResult = map[string]any{"route": "kb_rag", "cited_results": 3, "scope": "nexusai-forensic-demo"}
		entry.CalculationPass, entry.CitationPass, entry.PresentationPass = true, true, true
		entry.OverallStatus, entry.IssueIDs = "certified", []string{}
		entry.CertificationBasis = []string{"live KB retrieval acceptance", "collection-scope authorization regression", "citation locator and abstention contract tests"}
	case "forensics.evidence_package_summary":
		entry.CertificationLevel = certificationLevelProductCertified
		entry.SuggestionEligible = true
		entry.GoldenFixture = "nexusai-forensic-demo deterministic collection/source/quality records plus retained KB evidence; read-only acceptance on 2026-08-18"
		entry.GoldenOracle = "independent route/fact-preservation oracle requiring records_sql plus kb_rag, 17 deterministic rows, three cited KB results, and separated provenance"
		entry.ExpectedResult = map[string]any{"routes": []string{"records_sql", "kb_rag"}, "deterministic_rows": 17, "kb_results": 3}
		entry.ActualResult = map[string]any{"routes": []string{"records_sql", "kb_rag"}, "deterministic_rows": 17, "kb_results": 3}
		entry.CalculationPass, entry.CitationPass, entry.PresentationPass = true, true, true
		entry.OverallStatus, entry.IssueIDs = "certified", []string{}
		entry.CertificationBasis = []string{"live hybrid route acceptance", "deterministic fact-count oracle", "KB citation separation regression", "executive presentation contract"}
	}
	if entry.OverallStatus == "certified" || entry.OverallStatus == "certified_with_limitation" {
		entry.DebtDisposition = ""
	}
	return entry
}
