package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// References carry semantic and prerequisite metadata only. Execution, argument
// schemas, certification and suggestions remain owned by their existing registries.
//
//go:embed contracts/query-capability-references-v1.json
var queryCapabilityReferencesJSON []byte

// Only immutable embedded registry metadata is indexed. Evidence/readiness is
// never cached, so no authorization, scope or current-version state is reused.
var semanticMetadataOnce sync.Once
var semanticTemplates map[string]queryTemplateCatalogEntry
var semanticCertifications map[string]OperationCertificationEntryV1

func semanticTemplate(operation string) (queryTemplateCatalogEntry, bool) {
	semanticMetadataOnce.Do(func() {
		semanticTemplates = map[string]queryTemplateCatalogEntry{}
		for _, t := range supportedQueryTemplates() {
			semanticTemplates[t.OperationID] = t
		}
		semanticCertifications = operationCertificationByID()
	})
	t, ok := semanticTemplates[operation]
	return t, ok
}

type QueryCapabilityReferenceV1 struct {
	ID                         string            `json:"query_capability_id"`
	OperationRef               string            `json:"operation_ref"`
	ReferenceKind              string            `json:"reference_kind"`
	Family                     string            `json:"family"`
	Authority                  string            `json:"authority"`
	Semantics                  []string          `json:"semantics"`
	Representations            []string          `json:"representations"`
	ArtifactTypes              []string          `json:"artifact_types"`
	ProcessorRole              string            `json:"processor_role"`
	ResultStateKey             string            `json:"result_state_key"`
	ReadinessKind              string            `json:"readiness_kind"`
	InputTypes                 []string          `json:"input_types"`
	ScopeModes                 []string          `json:"scope_modes"`
	LocatorTypes               []string          `json:"locator_types"`
	ProofRefs                  []string          `json:"proof_refs"`
	Limitations                []string          `json:"limitations"`
	LanguageCoverage           map[string]string `json:"language_coverage"`
	ProofRequirements          []string          `json:"proof_requirements"`
	PerformanceClass           string            `json:"performance_class"`
	SemanticContract           string            `json:"semantic_contract,omitempty"`
	SupportsExhaustiveNegative bool              `json:"supports_exhaustive_negative,omitempty"`
	NegativeRequires           string            `json:"negative_requires,omitempty"`
	CrossSegmentSupported      bool              `json:"cross_segment_supported,omitempty"`
	BoundedWindow              []int             `json:"bounded_window,omitempty"`
	AdjacencyAuthority         string            `json:"adjacency_authority,omitempty"`
}

type CapabilityInputBoundaryV1 struct {
	Category   string `json:"category"`
	InputType  string `json:"input_type"`
	State      string `json:"state"`
	Pipeline   string `json:"pipeline"`
	Limitation string `json:"limitation"`
}

type QueryCapabilityReferencesV1 struct {
	SchemaVersion       string                       `json:"schema_version"`
	ImplementationState string                       `json:"implementation_state"`
	Capabilities        []QueryCapabilityReferenceV1 `json:"capabilities"`
	InputBoundaries     []CapabilityInputBoundaryV1  `json:"input_boundaries"`
	UnavailableIntents  []struct {
		ID                   string `json:"id"`
		Category             string `json:"category"`
		Reason               string `json:"reason"`
		QueryExistingResults bool   `json:"query_existing_results"`
		ProcessNewInput      bool   `json:"process_new_input"`
		GenerateNewOutput    bool   `json:"generate_new_output"`
	} `json:"unavailable_intents"`
}

func loadQueryCapabilityReferences() (QueryCapabilityReferencesV1, error) {
	var refs QueryCapabilityReferencesV1
	if err := json.Unmarshal(queryCapabilityReferencesJSON, &refs); err != nil {
		return refs, err
	}
	return refs, validateQueryCapabilityReferences(refs)
}

func validateQueryCapabilityReferences(refs QueryCapabilityReferencesV1) error {
	if refs.SchemaVersion != "forensics.query-capability-references/v1" || len(refs.Capabilities) == 0 {
		return fmt.Errorf("invalid capability reference contract")
	}
	catalog, err := loadForensicPlatformCatalog()
	if err != nil {
		return err
	}
	descriptors := map[string]FamilyOperationDescriptorV1{}
	for _, d := range catalog.Operations {
		descriptors[d.ID] = d
	}
	seen := map[string]bool{}
	aliases := map[string]bool{}
	for _, c := range refs.Capabilities {
		if !validCapabilityID(c.ID) || seen[c.ID] {
			return fmt.Errorf("invalid/duplicate capability %q", c.ID)
		}
		seen[c.ID] = true
		d, ok := descriptors[c.OperationRef]
		if !ok || d.FamilyID != c.Family {
			return fmt.Errorf("unresolved operation/family for %s", c.ID)
		}
		t, _ := semanticTemplate(c.OperationRef)
		template := t.Name
		if c.ReferenceKind == "QUERY_OPERATION" {
			if template == "" {
				return fmt.Errorf("query operation missing: %s", c.ID)
			}
		} else if c.ReferenceKind != "DIRECT_DESCRIPTOR" || template != "" || !strings.HasPrefix(d.Implementation, "GET /") {
			return fmt.Errorf("invalid direct descriptor: %s", c.ID)
		}
		if !containsString([]string{"DETERMINISTIC_STRUCTURED_FACT", "SOURCE_TEXT", "MODEL_OBSERVATION", "DERIVED_REPRESENTATION", "SEMANTIC_RETRIEVAL", "MODEL_SIMILARITY", "COMPOSED_VALIDATED_RESULT"}, c.Authority) {
			return fmt.Errorf("invalid authority: %s", c.ID)
		}
		if (c.Family == "audio_intelligence" || c.Family == "image_intelligence" || c.Family == "face_intelligence" || c.Family == "video_intelligence") && c.Authority == "DETERMINISTIC_STRUCTURED_FACT" {
			return fmt.Errorf("observation promoted to fact: %s", c.ID)
		}
		if c.Authority == "SOURCE_TEXT" && c.Family != "document_intelligence" || c.Authority == "DERIVED_REPRESENTATION" && !containsString(c.Representations, "roman_derivative") {
			return fmt.Errorf("authority representation mismatch: %s", c.ID)
		}
		if !containsString([]string{"records", "artifacts", "records_and_artifacts", "kb", "similarity", "composition", "inventory"}, c.ReadinessKind) {
			return fmt.Errorf("unknown readiness: %s", c.ID)
		}
		expectedAuthority := map[string]string{"records": "DETERMINISTIC_STRUCTURED_FACT", "kb": "SEMANTIC_RETRIEVAL", "similarity": "MODEL_SIMILARITY", "composition": "COMPOSED_VALIDATED_RESULT", "inventory": "COMPOSED_VALIDATED_RESULT"}[c.ReadinessKind]
		if expectedAuthority != "" && c.Authority != expectedAuthority {
			return fmt.Errorf("readiness/authority mismatch: %s", c.ID)
		}
		if (c.ReadinessKind == "artifacts" || c.ReadinessKind == "records_and_artifacts") && !containsString([]string{"SOURCE_TEXT", "MODEL_OBSERVATION", "DERIVED_REPRESENTATION"}, c.Authority) {
			return fmt.Errorf("artifact authority mismatch: %s", c.ID)
		}
		if c.ReadinessKind == "records_and_artifacts" && (len(c.ScopeModes) != 1 || c.ScopeModes[0] != "selected_evidence") {
			return fmt.Errorf("mixed-authority executor requires selected source: %s", c.ID)
		}
		for _, kind := range c.ArtifactTypes {
			if !containsString([]string{"forensics.audio-timestamp-segment/v1", "forensics.audio-roman-urdu-segment/v1", "forensics.image-ocr-observation/v1", "forensics.document-native-text-passage/v1", videoANPRGroupContract, "forensics.anpr-observation/v1", imageEmbeddingContract, faceObservationContract}, kind) {
				return fmt.Errorf("unknown artifact contract: %s", kind)
			}
		}
		if !containsString([]string{"", "native_document", "asr", "ocr", "image_anpr", "video_anpr", "image_embedding", "face"}, c.ProcessorRole) {
			return fmt.Errorf("unknown role: %s", c.ID)
		}
		if !containsString([]string{"", "asr", "ocr", "image_anpr", "video_anpr"}, c.ResultStateKey) {
			return fmt.Errorf("unknown processor state: %s", c.ID)
		}
		for _, s := range c.Semantics {
			if !containsString([]string{"EXACT_VALUE", "EXACT_PHRASE", "PHRASE_CONTAINS", "TOKEN_SEARCH", "SOURCE", "TIME_RANGE", "SOURCE_TIME", "FILTER", "COUNT", "AGGREGATE", "GROUP", "TOP_K", "RANK", "SEMANTIC_RETRIEVAL", "SIMILARITY", "SUMMARY", "COMPARE", "COMPOSE"}, s) {
				return fmt.Errorf("unknown semantic: %s", s)
			}
			key := c.OperationRef + ":" + strings.Join(c.Representations, ",") + ":" + s
			if aliases[key] {
				return fmt.Errorf("duplicate semantic alias: %s", key)
			}
			aliases[key] = true
		}
		if len(c.Semantics) == 0 || len(c.InputTypes) == 0 || len(c.ScopeModes) == 0 {
			return fmt.Errorf("missing capability boundary: %s", c.ID)
		}
		for _, p := range t.Inputs {
			if p.Name == "" {
				return fmt.Errorf("operation argument unavailable: %s", c.ID)
			}
		}
		if c.ReadinessKind == "similarity" && c.ReferenceKind != "DIRECT_DESCRIPTOR" {
			return fmt.Errorf("similarity needs existing direct input contract")
		}
		for _, p := range c.ProofRequirements {
			if !containsString([]string{"positive", "zero", "not_run", "unavailable", "incomplete", "clarification", "citation", "navigation", "activity"}, p) {
				return fmt.Errorf("unknown proof requirement: %s", p)
			}
		}
		for _, v := range c.InputTypes {
			found := false
			for _, b := range refs.InputBoundaries {
				if b.InputType == v && b.State == "SUPPORTED_INGEST_AND_ANALYSIS" {
					found = true
				}
			}
			if !found {
				return fmt.Errorf("unqualified capability input: %s", v)
			}
		}
		for _, v := range c.ScopeModes {
			if !containsString([]string{"selected_evidence", "authorized_workspace"}, v) {
				return fmt.Errorf("unknown scope: %s", v)
			}
		}
		for _, v := range c.LocatorTypes {
			if !containsString([]string{"row", "page", "region", "source_time", "frame", "artifact", "evidence"}, v) {
				return fmt.Errorf("unknown locator: %s", v)
			}
		}
		for _, v := range c.Representations {
			if !containsString([]string{"raw", "roman_derivative", "native_extracted", "normalized_search"}, v) {
				return fmt.Errorf("unknown representation: %s", v)
			}
		}
		for _, v := range c.ProofRefs {
			if !strings.HasPrefix(v, "reports/") || strings.Contains(v, "..") || strings.ContainsAny(v, "\\:") {
				return fmt.Errorf("invalid proof reference: %s", v)
			}
		}
		for _, l := range []string{"english", "urdu", "roman_urdu", "mixed"} {
			if !containsString([]string{"APPLICABLE", "NOT_APPLICABLE", "UNTESTED", "SOURCE_VALIDATED", "LIVE_VALIDATED"}, c.LanguageCoverage[l]) {
				return fmt.Errorf("missing language state: %s", l)
			}
		}
		if len(c.LanguageCoverage) != 4 {
			return fmt.Errorf("unknown language")
		}
		if c.SupportsExhaustiveNegative && c.NegativeRequires != "EXHAUSTIVE" {
			return fmt.Errorf("unbounded negative claim")
		}
	}
	inputKeys := map[string]bool{}
	categories := map[string]bool{}
	for _, f := range forensicCapabilityDefinitions() {
		categories[f.ID] = true
	}
	for _, b := range refs.InputBoundaries {
		key := b.Category + ":" + b.InputType
		if inputKeys[key] || !categories[b.Category] || b.InputType == "" || !containsString([]string{"SUPPORTED_INGEST_AND_ANALYSIS", "SUPPORTED_INGEST_INVENTORY_ONLY", "MANUAL_REVIEW", "NOT_QUALIFIED", "UNAVAILABLE"}, b.State) {
			return fmt.Errorf("invalid input boundary: %s", key)
		}
		inputKeys[key] = true
	}
	for _, template := range []string{"audio_transcript_search", "document_search", "image_ocr_search"} {
		found := false
		for _, c := range refs.Capabilities {
			t, _ := semanticTemplate(c.OperationRef)
			if t.Name == template && c.SemanticContract == "derived-text-search/v1" {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("missing B1 semantic reference: %s", template)
		}
	}
	return nil
}

// This shape consumes the existing worker readiness receipt, never a models
// list or a client-supplied enabled flag. Absence of a current receipt fails closed.
type CapabilityProcessorReceiptV1 struct {
	ContractVersion         string `json:"contract_version"`
	Role                    string `json:"role"`
	State                   string `json:"state"`
	CodePresent             bool   `json:"code_present"`
	RoleEnabled             bool   `json:"role_enabled"`
	RoleAdmitted            bool   `json:"role_admitted"`
	WorkerHealthy           bool   `json:"worker_healthy"`
	ResourcePolicySatisfied bool   `json:"resource_policy_satisfied"`
	BackendAvailable        bool   `json:"backend_available"`
	ModelAssetsVerified     bool   `json:"model_assets_verified"`
}

type CapabilityEvidenceStateV1 struct {
	Eligible     bool
	Results      bool
	CompleteZero bool
	Failed       bool
	Incomplete   bool
}

type SemanticReadinessContextV1 struct {
	Authorized        bool
	ExecutorAvailable bool
	Origin            string
	SemanticContracts map[string]bool
	ProcessorReceipts map[string]CapabilityProcessorReceiptV1
	// Set only after the existing direct executor validates the query observation,
	// candidate scope and compatible vector population. Discovery does not run it.
	ValidatedSimilarity map[string]bool
	RetrievalAvailable  bool
	ScopeMode           string
}

type SemanticCapabilityReadinessV1 struct {
	CapabilityID           string   `json:"query_capability_id"`
	OperationRef           string   `json:"operation_ref"`
	Registered             bool     `json:"registered"`
	QueryExistingResults   bool     `json:"query_existing_results"`
	ProcessNewInput        bool     `json:"process_new_input"`
	GenerateNewOutput      bool     `json:"generate_new_output"`
	ResultState            string   `json:"result_state"`
	ProcessingState        string   `json:"processing_state"`
	Reason                 string   `json:"reason"`
	CanonicalCertification string   `json:"canonical_certification"`
	SuggestionEligible     bool     `json:"suggestion_eligible"`
	Origin                 string   `json:"origin"`
	SearchCompleteness     string   `json:"search_completeness"`
	RequiredParameters     []string `json:"required_parameters"`
}

func projectSemanticReadiness(c QueryCapabilityReferenceV1, s CapabilityEvidenceStateV1, ctx SemanticReadinessContextV1) SemanticCapabilityReadinessV1 {
	r := SemanticCapabilityReadinessV1{CapabilityID: c.ID, OperationRef: c.OperationRef, Registered: true, ResultState: "UNAVAILABLE", ProcessingState: "UNAVAILABLE", Reason: "NO_ELIGIBLE_EVIDENCE", Origin: ctx.Origin}
	r.SearchCompleteness = "REQUIRES_EXECUTION"
	if t, ok := semanticTemplate(c.OperationRef); ok {
		for _, p := range t.Inputs {
			if p.Required {
				r.RequiredParameters = append(r.RequiredParameters, p.Name)
			}
		}
	}
	if len(c.ScopeModes) == 1 && c.ScopeModes[0] == "selected_evidence" && !containsString(r.RequiredParameters, "evidence_id") {
		r.RequiredParameters = append(r.RequiredParameters, "evidence_id")
	}
	cert := semanticCertifications[c.OperationRef]
	r.CanonicalCertification = cert.CertificationLevel
	if r.CanonicalCertification == "" {
		r.CanonicalCertification = "NOT_IN_QUERY_CERTIFICATION_REGISTRY"
	}
	if !ctx.Authorized {
		r.Reason = "UNAUTHORIZED"
		return r
	}
	if !ctx.ExecutorAvailable {
		r.Reason = "EXECUTOR_UNAVAILABLE"
		return r
	}
	if cert.ExposureStatus == "engineering_only" {
		r.Reason = "ENGINEERING_ONLY"
		return r
	}
	if c.ReferenceKind == "DIRECT_DESCRIPTOR" {
		catalog, _ := loadForensicPlatformCatalog()
		for _, d := range catalog.Operations {
			if d.ID == c.OperationRef && d.Maturity == "ENGINEERING_ONLY" {
				r.Reason = "ENGINEERING_ONLY"
				return r
			}
		}
	}
	if c.SemanticContract != "" && !ctx.SemanticContracts[c.SemanticContract] {
		r.Reason = "SEMANTIC_CONTRACT_UNDEPLOYED"
		return r
	}
	if !containsString(c.ScopeModes, ctx.ScopeMode) {
		r.Reason = "SCOPE_NOT_SUPPORTED"
		return r
	}
	receipt, ok := ctx.ProcessorReceipts[c.ProcessorRole]
	if ok && receipt.ContractVersion == "forensics.processor-readiness/v1" && receipt.Role == c.ProcessorRole {
		switch {
		case !receipt.RoleEnabled || !receipt.RoleAdmitted:
			r.ProcessingState = "RUNTIME_ROLE_DISABLED"
		case !receipt.ModelAssetsVerified:
			r.ProcessingState = "MODEL_REQUIRED"
		case receipt.State == "READY" && receipt.CodePresent && receipt.WorkerHealthy && receipt.ResourcePolicySatisfied && receipt.BackendAvailable:
			r.ProcessingState = "READY"
			r.ProcessNewInput = s.Eligible
		}
	}
	if !s.Eligible {
		r.Reason = "INPUT_NOT_SUPPORTED_OR_NO_ELIGIBLE_EVIDENCE"
		return r
	}
	switch {
	case c.ReadinessKind == "similarity" && !ctx.ValidatedSimilarity[c.ID]:
		r.ResultState = "CLARIFICATION_REQUIRED"
		r.Reason = "AUTHORIZED_QUERY_AND_COMPATIBLE_CANDIDATES_REQUIRED"
	case c.ReadinessKind == "kb" && !ctx.RetrievalAvailable:
		r.Reason = "RETRIEVAL_RUNTIME_UNVERIFIED"
	case c.ReadinessKind == "composition":
		r.ResultState = "CLARIFICATION_REQUIRED"
		r.Reason = "VALIDATE_COMPONENT_PREREQUISITES"
	case s.Results:
		r.QueryExistingResults = true
		r.ResultState = "COMPLETE_RESULTS"
		r.Reason = "READY_EXISTING_RESULTS"
	case s.Failed:
		r.ResultState = "FAILED"
		r.Reason = "PROCESSOR_FAILED"
	case s.CompleteZero:
		r.QueryExistingResults = true
		r.ResultState = "COMPLETE_ZERO_RESULTS"
		r.Reason = "RESULTS_ZERO"
	case s.Incomplete:
		r.ResultState = "INCOMPLETE"
		r.Reason = "PROCESSING_INCOMPLETE"
	default:
		r.ResultState = "NOT_RUN"
		r.Reason = "NOT_RUN"
	}
	r.SuggestionEligible = r.QueryExistingResults && cert.SuggestionEligible && cert.CertificationLevel == certificationLevelProductCertified
	return r
}

// The same tenant/case/collection/current-version predicates apply to positive,
// zero and failed states. A zero receipt is tied through an artifact run to its
// ingest job; unversioned evidence metadata is never enough to prove zero.
const semanticCapabilityStateSQL = `
WITH scoped AS MATERIALIZED (
 SELECT e.* FROM forensic.evidence_items e
 WHERE e.tenant_id=$1 AND e.collection_id=$2 AND ($3='' OR e.case_id=$3)
 AND ($4='' OR e.evidence_id=NULLIF($4,'')::uuid)
 AND ($5='' OR e.current_version_id=NULLIF($5,'')::uuid)
), requirements AS (SELECT * FROM jsonb_to_recordset($6::jsonb)
 AS x(id text,kind text,inputs text[],contracts text[],record_types text[],state_key text,representation text))
SELECT q.id,
 EXISTS(SELECT 1 FROM scoped e WHERE lower(ltrim(e.extension,'.'))=ANY(q.inputs)) AS eligible,
 EXISTS(SELECT 1 FROM scoped e WHERE lower(ltrim(e.extension,'.'))=ANY(q.inputs) AND (
  (q.kind='inventory') OR (q.kind='kb' AND nullif(e.kb_entry_ref,'') IS NOT NULL) OR
  (q.kind='records' AND EXISTS(SELECT 1 FROM forensic.records r WHERE r.tenant_id=e.tenant_id AND r.collection_id=e.collection_id AND r.evidence_id=e.evidence_id AND r.batch_id=e.records_batch_id AND (r.record_type::text=ANY(q.record_types) OR 'all'=ANY(q.record_types)))) OR
  (q.kind IN ('artifacts','records_and_artifacts','similarity')
   AND (q.kind<>'records_and_artifacts' OR EXISTS(SELECT 1 FROM forensic.records r WHERE r.tenant_id=e.tenant_id AND r.collection_id=e.collection_id AND r.evidence_id=e.evidence_id AND r.batch_id=e.records_batch_id AND r.record_type::text=ANY(q.record_types)))
   AND EXISTS(SELECT 1 FROM forensic.derived_artifacts a
   WHERE a.tenant_id=e.tenant_id AND a.collection_id=e.collection_id AND a.evidence_id=e.evidence_id
   AND a.version_id=e.current_version_id AND a.run_id IS NOT NULL AND a.processing_status='completed'
   AND a.artifact_type=ANY(q.contracts)
   AND (q.representation<>'raw' OR a.artifact_type<>'forensics.image-ocr-observation/v1' OR nullif(a.metadata #>> '{observation,raw_text}','') IS NOT NULL)))
 )) AS results,
 NOT EXISTS(SELECT 1 FROM scoped e WHERE lower(ltrim(e.extension,'.'))=ANY(q.inputs) AND NOT EXISTS(
 SELECT 1 FROM forensic.derived_artifacts a
 JOIN forensic.records_ingest_jobs j ON j.tenant_id=a.tenant_id AND j.collection_id=a.collection_id AND j.evidence_id=a.evidence_id AND j.id=a.run_id
 WHERE a.tenant_id=e.tenant_id AND a.collection_id=e.collection_id AND a.evidence_id=e.evidence_id AND a.version_id=e.current_version_id
 AND q.state_key<>'' AND j.status='completed' AND a.processing_status='completed'
 AND j.metadata->'media_result_states'->>q.state_key='COMPLETE_ZERO_RESULTS')) AS complete_zero,
 EXISTS(SELECT 1 FROM scoped e JOIN forensic.derived_artifacts a
 ON a.tenant_id=e.tenant_id AND a.collection_id=e.collection_id AND a.evidence_id=e.evidence_id AND a.version_id=e.current_version_id
 JOIN forensic.records_ingest_jobs j ON j.tenant_id=a.tenant_id AND j.collection_id=a.collection_id AND j.evidence_id=a.evidence_id AND j.id=a.run_id
 WHERE lower(ltrim(e.extension,'.'))=ANY(q.inputs) AND q.state_key<>'' AND j.status='completed' AND a.processing_status='completed'
 AND (j.metadata->'media_result_states'->>q.state_key IN ('COMPLETE_ZERO_RESULTS','INCOMPLETE','PROCESSING')
 OR a.artifact_type=ANY(q.contracts))) AS incomplete,
 EXISTS(SELECT 1 FROM scoped e JOIN forensic.records_ingest_jobs j ON j.tenant_id=e.tenant_id AND j.collection_id=e.collection_id AND j.evidence_id=e.evidence_id
 WHERE lower(ltrim(e.extension,'.'))=ANY(q.inputs) AND j.metadata->>'version_id'=e.current_version_id::text AND j.status IN ('failed','dead_letter')) AS failed
FROM requirements q`

func loadSemanticCapabilityStates(ctx context.Context, db *pgxpool.Pool, scope forensicScope, evidenceID, versionID string, refs QueryCapabilityReferencesV1) (map[string]CapabilityEvidenceStateV1, error) {
	states := map[string]CapabilityEvidenceStateV1{}
	if db == nil {
		return states, nil
	}
	requirements := []map[string]any{}
	for _, c := range refs.Capabilities {
		recordTypes := []string{}
		if t, ok := semanticTemplate(c.OperationRef); ok {
			recordTypes = t.RecordTypes
		}
		representation := ""
		if len(c.Representations) > 0 {
			representation = c.Representations[0]
		}
		requirements = append(requirements, map[string]any{"id": c.ID, "kind": c.ReadinessKind, "inputs": c.InputTypes, "contracts": c.ArtifactTypes, "record_types": recordTypes, "state_key": c.ResultStateKey, "representation": representation})
	}
	payload, err := json.Marshal(requirements)
	if err != nil {
		return nil, err
	}
	bounded, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	rows, err := queryCapabilityExistenceRows(bounded, db, scope.TenantID, semanticCapabilityStateSQL, scope.TenantID, scope.CollectionID, scope.CaseID, evidenceID, versionID, string(payload))
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		states[stringValueAny(row["id"])] = CapabilityEvidenceStateV1{Eligible: row["eligible"] == true, Results: row["results"] == true, CompleteZero: row["complete_zero"] == true, Failed: row["failed"] == true, Incomplete: row["incomplete"] == true}
	}
	return states, nil
}

func queryCapabilityExistenceRows(ctx context.Context, db *pgxpool.Pool, tenantID, query string, args ...any) ([]map[string]any, error) {
	tx, err := db.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	// The retained plan compiled 414 JIT functions for tiny EXISTS results and
	// exhausted the two-second budget. Disable JIT only in this transaction;
	// preserve the budget and the same tenant RLS context as queryRows.
	if _, err = tx.Exec(ctx, "SELECT set_config('app.tenant_id',$1,true),set_config('jit','off',true),set_config('statement_timeout','2000',true)", tenantID); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result, err := rowsToMaps(rows)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return result, nil
}

func loadCapabilityCoverage(ctx context.Context, db *pgxpool.Pool, scope forensicScope, evidenceID, versionID string) (CoverageSummary, error) {
	coverage := CoverageSummary{}
	if db == nil {
		return coverage, nil
	}
	bounded, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	rows, err := queryRows(bounded, db, scope.TenantID, `
SELECT r.record_type::text AS record_type,count(*) AS count
FROM forensic.records r JOIN forensic.evidence_items e
ON r.tenant_id=e.tenant_id AND r.collection_id=e.collection_id AND r.evidence_id=e.evidence_id AND r.batch_id=e.records_batch_id
WHERE e.tenant_id=$1 AND e.collection_id=$2 AND ($3='' OR e.case_id=$3)
AND ($4='' OR e.evidence_id=NULLIF($4,'')::uuid) AND ($5='' OR e.current_version_id=NULLIF($5,'')::uuid)
GROUP BY r.record_type`, scope.TenantID, scope.CollectionID, scope.CaseID, evidenceID, versionID)
	if err != nil {
		return coverage, err
	}
	for _, row := range rows {
		coverage.RecordFamiliesPresent = append(coverage.RecordFamiliesPresent, RecordFamilyCoverage{RecordType: stringValueAny(row["record_type"]), Count: valueAsInt(row["count"])})
		coverage.TotalIndexedRecords += valueAsInt(row["count"])
	}
	return coverage, nil
}
