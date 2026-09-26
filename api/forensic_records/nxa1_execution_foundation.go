package main

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mudler/LocalAI/pkg/forensictext"
)

const (
	investigationContextContractV1 = "forensics.investigation-context/v1"
	capabilitySnapshotContractV1   = "forensics.capability-snapshot/v1"
	planValidationContractV1       = "forensics.plan-validation/v1"
	toolInvocationContractV1       = "forensics.tool-invocation/v1"
	toolResultContractV1           = "forensics.tool-result/v1"
	observationPacketContractV1    = "forensics.observation-packet/v1"
	// v2 changes `options` from bare strings to {label, query} objects so a
	// clarification can be acted on in one click. Bumped rather than extended:
	// the field's TYPE changed, and a consumer reading v1 would silently see an
	// array of objects where it expected strings.
	clarificationRequestContractV1 = "forensics.clarification-request/v2"
)

// NX-A1 reuses the accepted APF-3 contracts as the authoritative durable
// concepts. These aliases deliberately prevent a second plan, operation,
// entity, citation, budget, or answer-envelope hierarchy from emerging.
type EntityReferenceV1 = QueryEntityV2
type QueryPlanV1 = GovernedExecutionPlanV1
type PlanStepV1 = ExecutionStepV1
type OperationContractV1 = ResolvedCapabilityV1
type ExecutionBudgetV1 = ExecutionResourcePolicyV1
type AnswerEnvelopeV1 = EnterpriseResponseV1
type Citation = CitationV1

type QueryIntentV1 struct {
	Class      QueryIntentClass `json:"class"`
	Qualifiers []string         `json:"qualifiers"`
	Origin     QueryValueOrigin `json:"origin"`
}

type EvidenceScopeKind string

const (
	EvidenceScopeWorkspace   EvidenceScopeKind = "current_workspace"
	EvidenceScopeSelected    EvidenceScopeKind = "selected_evidence"
	EvidenceScopeExplicitIDs EvidenceScopeKind = "explicit_evidence_ids"
	EvidenceScopeFamily      EvidenceScopeKind = "family_limited"
	EvidenceScopePriorResult EvidenceScopeKind = "prior_result"
)

type EvidenceScopeV1 struct {
	Kind             EvidenceScopeKind `json:"kind"`
	CollectionID     string            `json:"collection_id"`
	EvidenceIDs      []string          `json:"evidence_ids"`
	FamilyIDs        []string          `json:"family_ids"`
	SourceFiles      []string          `json:"source_files"`
	PriorResultID    string            `json:"prior_result_id,omitempty"`
	EvidenceVersions map[string]string `json:"evidence_versions,omitempty"`
}

type TimeDomain string

const (
	TimeDomainRecordTimestamp TimeDomain = "record_timestamp"
	TimeDomainMediaSource     TimeDomain = "media_source_time"
	TimeDomainEventRelative   TimeDomain = "event_relative"
)

type TimeScopeV1 struct {
	Domain          TimeDomain       `json:"domain"`
	From            string           `json:"from,omitempty"`
	To              string           `json:"to,omitempty"`
	Timezone        string           `json:"timezone,omitempty"`
	SourceTimezone  string           `json:"source_timezone,omitempty"`
	DateOrder       string           `json:"date_order,omitempty"`
	Precision       string           `json:"precision,omitempty"`
	EvidenceID      string           `json:"evidence_id,omitempty"`
	StartSeconds    *float64         `json:"start_seconds,omitempty"`
	EndSeconds      *float64         `json:"end_seconds,omitempty"`
	RelativeEventID string           `json:"relative_event_id,omitempty"`
	Origin          QueryValueOrigin `json:"origin"`
	Normalizer      string           `json:"normalizer,omitempty"`
}

type LocationScopeV1 struct {
	ReferenceType string           `json:"reference_type,omitempty"`
	ReferenceID   string           `json:"reference_id,omitempty"`
	Latitude      *float64         `json:"latitude,omitempty"`
	Longitude     *float64         `json:"longitude,omitempty"`
	RadiusMeters  *float64         `json:"radius_meters,omitempty"`
	Datum         string           `json:"datum,omitempty"`
	UncertaintyM  *float64         `json:"uncertainty_meters,omitempty"`
	Origin        QueryValueOrigin `json:"origin"`
}

type ConversationReferenceV1 struct {
	ConversationID string `json:"conversation_id,omitempty"`
	AnalysisID     string `json:"analysis_id,omitempty"`
	TurnID         string `json:"turn_id,omitempty"`
}

type InvestigationContextV1 struct {
	ContractVersion      string                    `json:"contract_version"`
	Workspace            ExecutionWorkspaceScopeV1 `json:"workspace"`
	Evidence             EvidenceScopeV1           `json:"evidence"`
	ActiveEntities       []EntityReferenceV1       `json:"active_entities"`
	CurrentQuestion      string                    `json:"current_question"`
	PreviousQuestion     string                    `json:"previous_question,omitempty"`
	PreviousResult       ConversationReferenceV1   `json:"previous_result"`
	Time                 TimeScopeV1               `json:"time"`
	Location             *LocationScopeV1          `json:"location,omitempty"`
	AvailableFamilies    []string                  `json:"available_families"`
	CapabilitySnapshotID string                    `json:"capability_snapshot_id,omitempty"`
	Permissions          []string                  `json:"permissions"`
	Conversation         ConversationReferenceV1   `json:"conversation"`
	ContextProvenance    QueryContextProvenanceV1  `json:"context_provenance"`
}

type DataReadinessState string

const (
	DataReadinessNotRun              DataReadinessState = "NOT_RUN"
	DataReadinessProcessing          DataReadinessState = "PROCESSING"
	DataReadinessCompleteZeroResults DataReadinessState = "COMPLETE_ZERO_RESULTS"
	DataReadinessCompleteResults     DataReadinessState = "COMPLETE_RESULTS"
	DataReadinessFailed              DataReadinessState = "FAILED"
	DataReadinessUnavailable         DataReadinessState = "UNAVAILABLE"
)

type CapabilityRequirementV1 struct {
	FamilyIDs          []string `json:"family_ids"`
	RequiredParameters []string `json:"required_parameters"`
	RequiredFields     []string `json:"required_fields"`
	ModelRoleIDs       []string `json:"model_role_ids"`
	CitationRequired   bool     `json:"citation_required"`
	CertifiedRequired  bool     `json:"certified_required"`
}

type CapabilitySnapshotEntryV1 struct {
	Operation   OperationContractV1     `json:"operation"`
	Requirement CapabilityRequirementV1 `json:"requirement"`
	Readiness   DataReadinessState      `json:"readiness"`
	ReasonCodes []CapabilityReasonCode  `json:"reason_codes"`
}

type CapabilitySnapshotV1 struct {
	ContractVersion string                      `json:"contract_version"`
	SnapshotID      string                      `json:"snapshot_id"`
	TenantID        string                      `json:"tenant_id"`
	CollectionID    string                      `json:"collection_id"`
	GeneratedAt     time.Time                   `json:"generated_at"`
	Entries         []CapabilitySnapshotEntryV1 `json:"entries"`
}

type PlanValidationSeverity string

const (
	PlanValidationError   PlanValidationSeverity = "error"
	PlanValidationWarning PlanValidationSeverity = "warning"
)

type PlanValidationIssueV1 struct {
	Code     string                 `json:"code"`
	Path     string                 `json:"path,omitempty"`
	Message  string                 `json:"message"`
	Severity PlanValidationSeverity `json:"severity"`
}

type PlanValidationResultV1 struct {
	ContractVersion    string                  `json:"contract_version"`
	PlanID             string                  `json:"plan_id"`
	Valid              bool                    `json:"valid"`
	ScopeAuthorized    bool                    `json:"scope_authorized"`
	CapabilitiesReady  bool                    `json:"capabilities_ready"`
	DependenciesValid  bool                    `json:"dependencies_valid"`
	BudgetAccepted     bool                    `json:"budget_accepted"`
	AuthorityPreserved bool                    `json:"authority_preserved"`
	EstimatedCostClass string                  `json:"estimated_cost_class"`
	Issues             []PlanValidationIssueV1 `json:"issues"`
}

type AuthorityClass string

const (
	AuthorityDeterministicFact       AuthorityClass = "deterministic_fact"
	AuthorityDeterministicDerivation AuthorityClass = "deterministic_derivation"
	AuthorityModelObservation        AuthorityClass = "model_observation"
	AuthoritySemanticRetrieval       AuthorityClass = "semantic_retrieval"
	AuthorityCandidateCorrelation    AuthorityClass = "candidate_correlation"
	AuthorityLLMInterpretation       AuthorityClass = "llm_interpretation"
)

type ExecutionStatus string

const (
	ExecutionStatusPlanned   ExecutionStatus = "planned"
	ExecutionStatusRunning   ExecutionStatus = "running"
	ExecutionStatusCompleted ExecutionStatus = "completed"
	ExecutionStatusPartial   ExecutionStatus = "partial"
	ExecutionStatusSkipped   ExecutionStatus = "skipped"
	ExecutionStatusFailed    ExecutionStatus = "failed"
	ExecutionStatusCancelled ExecutionStatus = "cancelled"
	ExecutionStatusTimedOut  ExecutionStatus = "timed_out"
)

type ResultState string

const (
	ResultStateResultsPresent   ResultState = EnterpriseResultStateResultsPresent
	ResultStateCompleteZero     ResultState = EnterpriseResultStateCompleteZero
	ResultStateNoMatchForFilter ResultState = EnterpriseResultStateNoMatchForFilter
	ResultStateNotProcessed     ResultState = EnterpriseResultStateNotProcessed
	ResultStateProcessing       ResultState = EnterpriseResultStateProcessing
	ResultStateFailed           ResultState = EnterpriseResultStateFailed
	ResultStateUnavailable      ResultState = EnterpriseResultStateUnavailable
	ResultStateUnauthorized     ResultState = EnterpriseResultStateUnauthorized
	ResultStateInvalidRequest   ResultState = EnterpriseResultStateInvalidRequest
)

type ToolInvocationV1 struct {
	ContractVersion string                    `json:"contract_version"`
	InvocationID    string                    `json:"invocation_id"`
	RequestID       string                    `json:"request_id"`
	PlanID          string                    `json:"plan_id"`
	StepID          string                    `json:"step_id"`
	OperationID     string                    `json:"operation_id"`
	Workspace       ExecutionWorkspaceScopeV1 `json:"workspace"`
	EvidenceScope   ExecutionEvidenceScopeV1  `json:"evidence_scope"`
	Parameters      ExecutionParametersV1     `json:"parameters"`
	ReadOnly        bool                      `json:"read_only"`
	TimeoutMS       int                       `json:"timeout_ms"`
	IdempotencyKey  string                    `json:"idempotency_key"`
}

type ToolFactV1 struct {
	ID          string         `json:"id"`
	Value       any            `json:"value"`
	Authority   AuthorityClass `json:"authority"`
	CitationIDs []string       `json:"citation_ids"`
}

type ObservationV1 struct {
	ID            string         `json:"id"`
	Kind          string         `json:"kind"`
	Value         any            `json:"value"`
	Authority     AuthorityClass `json:"authority"`
	Confidence    *float64       `json:"confidence,omitempty"`
	ModelRoleID   string         `json:"model_role_id,omitempty"`
	ModelID       string         `json:"model_id,omitempty"`
	ModelRevision string         `json:"model_revision,omitempty"`
	CitationIDs   []string       `json:"citation_ids"`
	Limitations   []string       `json:"limitations"`
}

type ObservationPacketV1 struct {
	ContractVersion string          `json:"contract_version"`
	PacketID        string          `json:"packet_id"`
	RequestID       string          `json:"request_id"`
	CaseID          string          `json:"case_id"`
	CollectionID    string          `json:"collection_id"`
	OperationID     string          `json:"operation_id"`
	Observations    []ObservationV1 `json:"observations"`
	Citations       []CitationV1    `json:"citations"`
	Limitations     []string        `json:"limitations"`
}

type ToolResultV1 struct {
	TextTimeScope      *TimeScopeV1                        `json:"text_time_scope,omitempty"`
	TextQuery          *forensictext.Query                 `json:"text_query,omitempty"`
	TextScope          *ExecutionEvidenceScopeV1           `json:"text_scope,omitempty"`
	TenantID           string                              `json:"tenant_id,omitempty"`
	ContractVersion    string                              `json:"contract_version"`
	InvocationID       string                              `json:"invocation_id"`
	PlanID             string                              `json:"plan_id"`
	StepID             string                              `json:"step_id"`
	OperationID        string                              `json:"operation_id"`
	ResultContract     string                              `json:"result_contract"`
	ExecutionStatus    ExecutionStatus                     `json:"execution_status"`
	ResultState        ResultState                         `json:"result_state"`
	Authority          AuthorityClass                      `json:"authority"`
	Fields             map[string]TypedIntermediateFieldV1 `json:"fields"`
	Facts              []ToolFactV1                        `json:"facts"`
	Observations       []ObservationPacketV1               `json:"observations"`
	Rows               []map[string]any                    `json:"rows"`
	Citations          []CitationV1                        `json:"citations"`
	Limitations        []string                            `json:"limitations"`
	FailureCode        string                              `json:"failure_code,omitempty"`
	RowCount           int                                 `json:"row_count"`
	SearchCompleteness string                              `json:"search_completeness,omitempty"`
	Truncated          bool                                `json:"truncated"`
	DurationMS         int64                               `json:"duration_ms"`
}

type ClarificationRequestV1 struct {
	ContractVersion string   `json:"contract_version"`
	ReasonCode      string   `json:"reason_code"`
	Question        string   `json:"question"`
	MissingFields   []string `json:"missing_fields"`
	// Options are 2-4 re-runnable choices drawn from the curated semantic
	// layer, or empty when the layer can offer none. v2 carries {label, query}
	// per option; v1 carried bare label strings and could not be clicked.
	Options       []ClarificationOptionV1 `json:"options"`
	ExecutionHeld bool                    `json:"execution_held"`
}

func (s EvidenceScopeV1) Validate(collectionID string) error {
	if strings.TrimSpace(s.CollectionID) == "" || s.CollectionID != collectionID {
		return fmt.Errorf("evidence scope must remain in the authorized collection")
	}
	switch s.Kind {
	case EvidenceScopeWorkspace:
	case EvidenceScopeSelected, EvidenceScopeExplicitIDs:
		if len(s.EvidenceIDs) == 0 {
			return fmt.Errorf("%s scope requires exact evidence IDs", s.Kind)
		}
	case EvidenceScopeFamily:
		if len(s.FamilyIDs) == 0 {
			return fmt.Errorf("family-limited scope requires at least one family")
		}
	case EvidenceScopePriorResult:
		if strings.TrimSpace(s.PriorResultID) == "" {
			return fmt.Errorf("prior-result scope requires an authoritative result reference")
		}
	default:
		return fmt.Errorf("unsupported evidence scope kind %q", s.Kind)
	}
	return nil
}

func (s TimeScopeV1) Validate() error {
	switch s.Domain {
	case TimeDomainRecordTimestamp:
		if s.StartSeconds != nil || s.EndSeconds != nil || s.EvidenceID != "" {
			return fmt.Errorf("record timestamp scope cannot contain media source-time fields")
		}
		if (s.From != "" || s.To != "") && strings.TrimSpace(s.Timezone) == "" {
			return fmt.Errorf("record timestamp bounds require an explicit timezone")
		}
	case TimeDomainMediaSource:
		if s.From != "" || s.To != "" || strings.TrimSpace(s.EvidenceID) == "" {
			return fmt.Errorf("media source-time scope requires evidence_id and cannot contain calendar bounds")
		}
		if s.StartSeconds != nil && s.EndSeconds != nil && *s.StartSeconds > *s.EndSeconds {
			return fmt.Errorf("media source-time start must not exceed end")
		}
	case TimeDomainEventRelative:
		if strings.TrimSpace(s.RelativeEventID) == "" {
			return fmt.Errorf("event-relative time scope requires an authoritative event reference")
		}
	default:
		return fmt.Errorf("unsupported time domain %q", s.Domain)
	}
	return nil
}

func (c InvestigationContextV1) Validate() error {
	if c.ContractVersion != investigationContextContractV1 {
		return fmt.Errorf("unsupported investigation context contract %q", c.ContractVersion)
	}
	if c.Workspace.TenantID == "" || c.Workspace.CaseID == "" || c.Workspace.CollectionID == "" || c.Workspace.SubjectID == "" || c.Workspace.CaseID != c.Workspace.CollectionID {
		return fmt.Errorf("investigation context requires one complete authoritative workspace scope")
	}
	if err := c.Evidence.Validate(c.Workspace.CollectionID); err != nil {
		return err
	}
	if err := c.Time.Validate(); err != nil {
		return err
	}
	if c.Location != nil {
		location := c.Location
		if (location.Latitude == nil) != (location.Longitude == nil) {
			return fmt.Errorf("location coordinates require both latitude and longitude")
		}
		if location.Latitude != nil && (*location.Latitude < -90 || *location.Latitude > 90 || *location.Longitude < -180 || *location.Longitude > 180) {
			return fmt.Errorf("location coordinates are outside valid bounds")
		}
		if location.RadiusMeters != nil && *location.RadiusMeters < 0 {
			return fmt.Errorf("location radius cannot be negative")
		}
	}
	for _, entity := range c.ActiveEntities {
		if strings.TrimSpace(entity.Type) == "" || strings.TrimSpace(entity.Original) == "" || strings.TrimSpace(entity.Normalized) == "" || strings.TrimSpace(entity.Normalizer) == "" {
			return fmt.Errorf("active entities must preserve original, normalized, type, and normalizer")
		}
	}
	return nil
}

func buildCapabilitySnapshot(snapshotID string, workspace ExecutionWorkspaceScopeV1, projection []ResolvedCapabilityV1) CapabilitySnapshotV1 {
	entries := make([]CapabilitySnapshotEntryV1, 0, len(projection))
	for _, operation := range projection {
		entries = append(entries, CapabilitySnapshotEntryV1{
			Operation: operation,
			Requirement: CapabilityRequirementV1{
				FamilyIDs: append([]string(nil), operation.FamilyIDs...), RequiredParameters: append([]string(nil), operation.RequiredParameters...),
				ModelRoleIDs: append([]string(nil), operation.ModelRoleIDs...), CitationRequired: operation.CitationRequired,
				CertifiedRequired: operation.CriticalityTier == "A",
			},
			Readiness:   readinessFromAvailability(operation.Availability),
			ReasonCodes: append([]CapabilityReasonCode(nil), operation.Availability.ReasonCodes...),
		})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Operation.OperationID < entries[j].Operation.OperationID })
	return CapabilitySnapshotV1{ContractVersion: capabilitySnapshotContractV1, SnapshotID: snapshotID, TenantID: workspace.TenantID, CollectionID: workspace.CollectionID, GeneratedAt: time.Now().UTC(), Entries: entries}
}

func readinessFromAvailability(availability CapabilityAvailabilityV1) DataReadinessState {
	if containsCapabilityReason(availability.ReasonCodes, CapabilityReasonProcessingIncomplete) {
		return DataReadinessProcessing
	}
	if containsCapabilityReason(availability.ReasonCodes, CapabilityReasonRuntimeUnavailable) || containsCapabilityReason(availability.ReasonCodes, CapabilityReasonModelUnavailable) || !availability.Implemented {
		return DataReadinessUnavailable
	}
	if availability.DataAvailable {
		return DataReadinessCompleteResults
	}
	return DataReadinessNotRun
}

func containsCapabilityReason(values []CapabilityReasonCode, wanted CapabilityReasonCode) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func validateGovernedExecutionPlan(plan GovernedExecutionPlanV1, capabilities []ResolvedCapabilityV1, context InvestigationContextV1, ceiling ExecutionBudgetV1) PlanValidationResultV1 {
	result := PlanValidationResultV1{
		ContractVersion: planValidationContractV1, PlanID: plan.PlanID,
		ScopeAuthorized: true, CapabilitiesReady: true, DependenciesValid: true,
		BudgetAccepted: true, AuthorityPreserved: true, EstimatedCostClass: planCostClass(plan), Issues: []PlanValidationIssueV1{},
	}
	addError := func(code, path, message string) {
		result.Issues = append(result.Issues, PlanValidationIssueV1{Code: code, Path: path, Message: message, Severity: PlanValidationError})
	}
	if err := context.Validate(); err != nil {
		result.ScopeAuthorized = false
		addError("invalid_investigation_context", "context", err.Error())
	} else if plan.Workspace != context.Workspace || plan.Workspace.CollectionID != context.Evidence.CollectionID {
		result.ScopeAuthorized = false
		addError("scope_mismatch", "workspace", "plan scope does not exactly match the authenticated investigation context")
	}
	if len(context.Permissions) > 0 && !containsString(context.Permissions, "query:execute") {
		result.ScopeAuthorized = false
		addError("unauthorized", "permissions", "query:execute permission is required")
	}
	var planErr error
	switch plan.Mode {
	case "single_operation":
		if len(capabilities) != 1 {
			planErr = fmt.Errorf("single-operation validation requires exactly one resolved capability")
		} else {
			planErr = validateSingleCapabilityPlan(plan, capabilities[0])
		}
	case "bounded_composition":
		planErr = validateBoundedCompositionPlan(plan, capabilities)
	default:
		planErr = fmt.Errorf("unsupported plan mode %q", plan.Mode)
	}
	if planErr != nil {
		result.CapabilitiesReady = false
		result.DependenciesValid = false
		addError("invalid_plan", "steps", planErr.Error())
	}
	if err := validateExecutionBudget(plan.ResourcePolicy, ceiling); err != nil {
		result.BudgetAccepted = false
		addError("budget_exceeded", "resource_policy", err.Error())
	}
	for _, capability := range capabilities {
		if !capability.Availability.Executable {
			result.CapabilitiesReady = false
			addError("capability_not_ready", "steps", fmt.Sprintf("operation %s is not executable", capability.OperationID))
		}
		if !containsString([]string{"runAnalyticalTemplate", "queryKnowledgeBaseEvidence", "derivedTextEvidence", "hybridQueryHandler"}, capability.ImplementationKey) {
			result.AuthorityPreserved = false
			addError("unapproved_executor", "steps", fmt.Sprintf("operation %s has an unapproved implementation key", capability.OperationID))
		}
	}
	result.Valid = result.ScopeAuthorized && result.CapabilitiesReady && result.DependenciesValid && result.BudgetAccepted && result.AuthorityPreserved && len(result.Issues) == 0
	return result
}

func validateExecutionBudget(actual, ceiling ExecutionBudgetV1) error {
	if !actual.ReadOnly {
		return fmt.Errorf("analytical execution must be read-only")
	}
	checks := []struct {
		name   string
		actual int
		limit  int
	}{
		{"maximum_steps", actual.MaximumSteps, ceiling.MaximumSteps}, {"maximum_rows", actual.MaximumRows, ceiling.MaximumRows},
		{"maximum_offset", actual.MaximumOffset, ceiling.MaximumOffset}, {"maximum_kb_results", actual.MaximumKBResults, ceiling.MaximumKBResults},
		{"maximum_targets", actual.MaximumTargets, ceiling.MaximumTargets}, {"maximum_intermediate_rows", actual.MaximumIntermediateRows, ceiling.MaximumIntermediateRows},
		{"maximum_retrieval_chunks", actual.MaximumRetrievalChunks, ceiling.MaximumRetrievalChunks}, {"maximum_model_calls", actual.MaximumModelCalls, ceiling.MaximumModelCalls},
		{"timeout_milliseconds", actual.TimeoutMilliseconds, ceiling.TimeoutMilliseconds},
	}
	for _, check := range checks {
		if check.limit > 0 && check.actual > check.limit {
			return fmt.Errorf("%s=%d exceeds ceiling %d", check.name, check.actual, check.limit)
		}
	}
	return nil
}

func planCostClass(plan GovernedExecutionPlanV1) string {
	if plan.ResourcePolicy.MaximumModelCalls > 0 || plan.ResourcePolicy.MaximumRetrievalChunks > 0 {
		return "bounded_model_or_retrieval"
	}
	if len(plan.Steps) > 1 {
		return "bounded_multi_operation"
	}
	return "direct_operation"
}

func toolInvocationFromStep(requestID string, plan GovernedExecutionPlanV1, step ExecutionStepV1) ToolInvocationV1 {
	return ToolInvocationV1{
		ContractVersion: toolInvocationContractV1, InvocationID: plan.PlanID + ":" + step.StepID, RequestID: requestID,
		PlanID: plan.PlanID, StepID: step.StepID, OperationID: step.OperationID, Workspace: plan.Workspace,
		EvidenceScope: step.EvidenceScope, Parameters: step.Parameters, ReadOnly: plan.ResourcePolicy.ReadOnly,
		TimeoutMS: plan.ResourcePolicy.TimeoutMilliseconds, IdempotencyKey: requestID + ":" + step.StepID,
	}
}

func (p ObservationPacketV1) Validate() error {
	if p.ContractVersion != observationPacketContractV1 || p.PacketID == "" || p.RequestID == "" || p.CaseID == "" || p.CollectionID == "" || p.CaseID != p.CollectionID || p.OperationID == "" {
		return fmt.Errorf("observation packet identity and scope are incomplete")
	}
	citationIDs := map[string]struct{}{}
	for _, citation := range p.Citations {
		if citation.ID == "" || citation.CollectionID != p.CollectionID {
			return fmt.Errorf("observation citation is missing or outside packet scope")
		}
		citationIDs[citation.ID] = struct{}{}
	}
	for _, observation := range p.Observations {
		if observation.ID == "" || observation.Kind == "" || !containsAuthority([]AuthorityClass{AuthorityModelObservation, AuthoritySemanticRetrieval, AuthorityCandidateCorrelation}, observation.Authority) {
			return fmt.Errorf("observation packet cannot contain deterministic facts or unlabeled authority")
		}
		if len(observation.CitationIDs) == 0 {
			return fmt.Errorf("observation %s requires at least one citation", observation.ID)
		}
		for _, citationID := range observation.CitationIDs {
			if _, ok := citationIDs[citationID]; !ok {
				return fmt.Errorf("observation %s references unknown citation %s", observation.ID, citationID)
			}
		}
	}
	return nil
}

func (r ToolResultV1) Validate(citationRequired bool, collectionID string) error {
	if r.ContractVersion != toolResultContractV1 || r.InvocationID == "" || r.PlanID == "" || r.StepID == "" || r.OperationID == "" || r.ResultContract == "" {
		return fmt.Errorf("tool result identity and result contract are incomplete")
	}
	if r.TextQuery != nil {
		if err := r.validateTextResult(collectionID); err != nil {
			return err
		}
	}
	if !validResultState(r.ResultState) || !validExecutionStatus(r.ExecutionStatus) || !containsAuthority([]AuthorityClass{AuthorityDeterministicFact, AuthorityDeterministicDerivation, AuthorityModelObservation, AuthoritySemanticRetrieval, AuthorityCandidateCorrelation}, r.Authority) {
		return fmt.Errorf("tool result status, result state, or authority is invalid")
	}
	if r.RowCount < 0 || r.RowCount != len(r.Rows) && !r.Truncated {
		return fmt.Errorf("tool result row accounting is inconsistent")
	}
	if r.SearchCompleteness != "" && !containsString([]string{"EXHAUSTIVE", "TRUNCATED", "INCOMPLETE", "UNKNOWN"}, r.SearchCompleteness) {
		return fmt.Errorf("invalid search completeness")
	}
	if r.SearchCompleteness != "" && r.SearchCompleteness != "EXHAUSTIVE" && (r.ResultState == ResultStateNoMatchForFilter || r.ResultState == ResultStateCompleteZero) {
		return fmt.Errorf("incomplete search cannot establish absence")
	}
	if r.RowCount > 0 && r.ResultState != ResultStateResultsPresent {
		return fmt.Errorf("non-result state contains analytical rows")
	}
	if r.ResultState == ResultStateCompleteZero && r.RowCount != 0 {
		return fmt.Errorf("complete-zero result cannot contain result rows")
	}
	if r.ResultState == ResultStateNoMatchForFilter && r.ExecutionStatus != ExecutionStatusCompleted {
		return fmt.Errorf("no-match-for-filter requires completed execution")
	}
	if r.ResultState == ResultStateNotProcessed && r.ExecutionStatus == ExecutionStatusCompleted {
		return fmt.Errorf("not-processed cannot be presented as completed execution")
	}
	if citationRequired && r.ResultState == ResultStateResultsPresent && len(r.Citations) == 0 {
		return fmt.Errorf("factual result requires citations")
	}
	for _, citation := range r.Citations {
		if citation.ID == "" || citation.CollectionID != collectionID {
			return fmt.Errorf("tool result citation is missing or outside authorized scope")
		}
	}
	for _, fact := range r.Facts {
		if !containsAuthority([]AuthorityClass{AuthorityDeterministicFact, AuthorityDeterministicDerivation}, fact.Authority) {
			return fmt.Errorf("fact %s has non-factual authority %q", fact.ID, fact.Authority)
		}
	}
	for _, packet := range r.Observations {
		if err := packet.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func validResultState(value ResultState) bool {
	return containsResultState([]ResultState{ResultStateResultsPresent, ResultStateCompleteZero, ResultStateNoMatchForFilter, ResultStateNotProcessed, ResultStateProcessing, ResultStateFailed, ResultStateUnavailable, ResultStateUnauthorized, ResultStateInvalidRequest}, value)
}

func validExecutionStatus(value ExecutionStatus) bool {
	return containsExecutionStatus([]ExecutionStatus{ExecutionStatusPlanned, ExecutionStatusRunning, ExecutionStatusCompleted, ExecutionStatusPartial, ExecutionStatusSkipped, ExecutionStatusFailed, ExecutionStatusCancelled, ExecutionStatusTimedOut}, value)
}

func containsAuthority(values []AuthorityClass, wanted AuthorityClass) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func containsResultState(values []ResultState, wanted ResultState) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func containsExecutionStatus(values []ExecutionStatus, wanted ExecutionStatus) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func investigationContextFromRequest(req hybridQueryRequest, understanding QueryUnderstandingV1, snapshot CapabilitySnapshotV1) InvestigationContextV1 {
	timeScope := TimeScopeV1{
		Domain: TimeDomainRecordTimestamp, From: req.DateFrom, To: req.DateTo, Timezone: understanding.TimeScope.Timezone,
		Origin: understanding.TimeScope.Origin, Normalizer: "forensics.time-policy/v1",
	}
	evidence := EvidenceScopeV1{Kind: EvidenceScopeWorkspace, CollectionID: req.CollectionID, EvidenceIDs: []string{}, FamilyIDs: []string{}, SourceFiles: []string{}, EvidenceVersions: map[string]string{}}
	if req.EvidenceID != "" {
		evidence.Kind = EvidenceScopeExplicitIDs
		if req.QueryScope.Kind == string(EvidenceScopeSelected) {
			evidence.Kind = EvidenceScopeSelected
		}
		evidence.EvidenceIDs = []string{req.EvidenceID}
		if req.EvidenceVersionID != "" {
			evidence.EvidenceVersions[req.EvidenceID] = req.EvidenceVersionID
		}
	}
	if files := uniqueStrings(append([]string{req.SourceFile}, structuredSourceFiles(req.SourceSet)...)); len(files) > 0 && !(len(files) == 1 && files[0] == "") {
		evidence.SourceFiles = files
	}
	if req.StartSeconds != nil || req.EndSeconds != nil {
		timeScope = TimeScopeV1{Domain: TimeDomainMediaSource, EvidenceID: req.EvidenceID, StartSeconds: req.StartSeconds, EndSeconds: req.EndSeconds, Origin: QueryOriginExplicitScope, Normalizer: "forensics.source-time/v1"}
	}
	families := uniqueStrings(append(append([]string{understanding.Families.Primary}, understanding.Families.Secondary...), req.QueryScope.SourceFamily))
	return InvestigationContextV1{
		ContractVersion: investigationContextContractV1,
		Workspace:       ExecutionWorkspaceScopeV1{TenantID: req.TenantID, CaseID: req.CollectionID, CollectionID: req.CollectionID, SubjectID: req.UserID},
		Evidence:        evidence, ActiveEntities: append([]EntityReferenceV1(nil), understanding.Entities...), CurrentQuestion: req.Query,
		Time: timeScope, AvailableFamilies: families, CapabilitySnapshotID: snapshot.SnapshotID, Permissions: []string{"query:execute"},
		Conversation:   ConversationReferenceV1{ConversationID: req.ConversationContext.ConversationID, AnalysisID: req.ConversationContext.AnalysisID, TurnID: req.ConversationContext.TurnID},
		PreviousResult: ConversationReferenceV1{AnalysisID: understanding.Context.SourceAnalysis, TurnID: understanding.Context.SourceTurn}, ContextProvenance: understanding.Context,
	}
}

func nxa1ExecutionBudgetCeiling() ExecutionBudgetV1 {
	return ExecutionBudgetV1{
		ReadOnly: true, MaximumSteps: maxCompositionSteps, MaximumRows: maxHybridLimit,
		MaximumOffset: 100000, MaximumKBResults: 20, MaximumTargets: maxCrossFamilyTargets,
		MaximumIntermediateRows: maxCompositionTotalRows, MaximumRetrievalChunks: maxCompositionKBChunks,
		MaximumModelCalls: maxCompositionModelCalls, TimeoutMilliseconds: maxCompositionTimeoutMS,
	}
}

func buildSharedToolResults(req hybridQueryRequest, resp hybridQueryResponse, warnings []string) ([]ToolResultV1, []string) {
	if resp.ExecutionPlan == nil {
		return nil, warnings
	}
	plan := *resp.ExecutionPlan
	if resp.Composition != nil {
		results := make([]ToolResultV1, 0, len(resp.Composition.Steps))
		for _, stepResult := range resp.Composition.Steps {
			step, ok := executionStepByID(plan.Steps, stepResult.StepID)
			if !ok {
				warnings = append(warnings, "tool_result_adapter: composition result references an unknown plan step")
				continue
			}
			citations := citationsFromExecution(stepResult.Citations, req.TenantID, req.CollectionID)
			rowCount := 0
			if field, exists := stepResult.Fields["row_count"]; exists {
				rowCount = int(int64ValueAny(field.Value))
			}
			state, status := sharedStatesFromCompositionStep(stepResult)
			result := ToolResultV1{
				ContractVersion: toolResultContractV1, InvocationID: plan.PlanID + ":" + step.StepID,
				PlanID: plan.PlanID, StepID: step.StepID, OperationID: step.OperationID, ResultContract: step.ExpectedResultContract,
				ExecutionStatus: status, ResultState: state, Authority: AuthorityDeterministicFact,
				Fields: stepResult.Fields, Facts: []ToolFactV1{}, Observations: []ObservationPacketV1{}, Rows: []map[string]any{}, Citations: citations,
				Limitations: append([]string(nil), stepResult.Limitations...), FailureCode: stepResult.FailureCode, RowCount: rowCount, Truncated: rowCount > 0,
			}
			if field, exists := stepResult.Fields["row_count"]; exists {
				result.Facts = append(result.Facts, ToolFactV1{ID: step.StepID + ":row_count", Value: field.Value, Authority: AuthorityDeterministicDerivation, CitationIDs: citationIDs(citations)})
			}
			if err := result.Validate(step.CitationRequired, req.CollectionID); err != nil {
				warnings = append(warnings, "tool_result_adapter: "+err.Error())
				continue
			}
			results = append(results, result)
		}
		return results, warnings
	}
	if len(plan.Steps) != 1 {
		return nil, append(warnings, "tool_result_adapter: direct execution plan does not contain exactly one step")
	}
	step := plan.Steps[0]
	rows := flattenEnterpriseRows(resp.Records, plan.ResourcePolicy.MaximumRows)
	if isFamilyDerivedTextTemplate(req.Template) {
		rows = evidenceResults(resp.Evidence)
	}
	if resp.Template == "video_anpr_grouped_timeline" {
		rows = videoResultRows(resp.Records["video_anpr_grouped_timeline"])
		if maximum := plan.ResourcePolicy.MaximumRows; maximum > 0 && len(rows) > maximum {
			rows = rows[:maximum]
		}
	}
	citations := citationsFromEnterprise(resp.Enterprise, req.TenantID, req.CollectionID)
	resultState := ResultState(stringValueAny(resp.Enterprise["result_state"]))
	if !validResultState(resultState) {
		resultState = ResultState(enterpriseResultState(resp, rows))
	}
	authority := AuthorityDeterministicFact
	if resp.ResolvedCapability != nil {
		switch resp.ResolvedCapability.ExecutionMode {
		case "retrieval":
			authority = AuthoritySemanticRetrieval
		case "legacy_hybrid":
			authority = AuthorityDeterministicDerivation
		}
	}
	result := ToolResultV1{
		ContractVersion: toolResultContractV1, InvocationID: plan.PlanID + ":" + step.StepID,
		PlanID: plan.PlanID, StepID: step.StepID, OperationID: step.OperationID, ResultContract: step.ExpectedResultContract,
		ExecutionStatus: executionStatusForResultState(resultState), ResultState: resultState, Authority: authority, Fields: map[string]TypedIntermediateFieldV1{},
		Facts:        []ToolFactV1{{ID: step.StepID + ":row_count", Value: len(rows), Authority: AuthorityDeterministicDerivation, CitationIDs: citationIDs(citations)}},
		Observations: []ObservationPacketV1{}, Rows: rows, Citations: citations, Limitations: stringSliceAny(resp.Enterprise["limitations"]),
		SearchCompleteness: stringValueAny(resp.Evidence["search_completeness"]),
		RowCount:           len(rows), Truncated: stringValueAny(resp.Evidence["search_completeness"]) == "INCOMPLETE" || stringValueAny(resp.Evidence["search_completeness"]) == "TRUNCATED" || len(rows) >= plan.ResourcePolicy.MaximumRows && plan.ResourcePolicy.MaximumRows > 0,
		DurationMS: resp.Telemetry.TotalLatencyMS,
	}
	if req.TextQuery != nil {
		result.TextQuery = req.TextQuery
		scope := executionEvidenceScopeFromRequest(req)
		result.TextScope = &scope
		result.TenantID = req.TenantID
		if req.StartSeconds != nil || req.EndSeconds != nil {
			result.TextTimeScope = &TimeScopeV1{Domain: TimeDomainMediaSource, EvidenceID: req.EvidenceID, StartSeconds: req.StartSeconds, EndSeconds: req.EndSeconds, Origin: QueryOriginCurrentQuestion}
		}
	}
	if (req.Template == "audio_transcript_search" || req.Template == "image_ocr_search" || req.Template == "video_timeline" || req.Template == "video_anpr_grouped_timeline" || req.Template == "face_candidate_observations") && resultState == ResultStateResultsPresent {
		result.Authority = AuthorityModelObservation
		result.Facts = []ToolFactV1{}
		observations := make([]ObservationV1, 0, len(rows))
		for index, row := range rows {
			if metadata, ok := row["metadata"].(map[string]any); ok {
				row = metadata
			}
			observationID := strings.TrimSpace(stringValueAny(row["artifact_id"]))
			if observationID == "" {
				observationID = fmt.Sprintf("%s:observation:%d", step.StepID, index+1)
			}
			kind := strings.TrimSpace(stringValueAny(row["artifact_type"]))
			if kind == "" {
				kind = req.Template + "_observation"
			}
			observations = append(observations, ObservationV1{
				ID: observationID, Kind: kind, Value: row, Authority: AuthorityModelObservation,
				CitationIDs: citationIDsForObservationRow(row, citations),
				Limitations: []string{"Retained model observation only; analyst review is required and the observation does not prove identity, ownership, association, causation, intent, or a continuous real-world event."},
			})
		}
		result.Observations = []ObservationPacketV1{{
			ContractVersion: observationPacketContractV1, PacketID: plan.PlanID + ":model-observations",
			RequestID: firstNonempty(resp.Telemetry.RequestID, plan.PlanID), CaseID: req.CollectionID, CollectionID: req.CollectionID,
			OperationID: step.OperationID, Observations: observations, Citations: append([]CitationV1(nil), citations...),
			Limitations: []string{"The packet contains retained model observations for analyst review, not deterministic identity or event facts."},
		}}
	}
	if resultState != ResultStateResultsPresent {
		result.Facts = []ToolFactV1{}
	}
	if err := result.Validate(step.CitationRequired, req.CollectionID); err != nil {
		return nil, append(warnings, "tool_result_adapter: "+err.Error())
	}
	return []ToolResultV1{result}, warnings
}

func executionStepByID(steps []ExecutionStepV1, stepID string) (ExecutionStepV1, bool) {
	for _, step := range steps {
		if step.StepID == stepID {
			return step, true
		}
	}
	return ExecutionStepV1{}, false
}

func citationsFromExecution(values []ExecutionCitationV1, tenantID, collectionID string) []CitationV1 {
	result := make([]CitationV1, 0, len(values))
	for index, value := range values {
		locator := value.SourceLocator
		if locator == "" && value.SourceRow > 0 {
			locator = fmt.Sprintf("row:%d", value.SourceRow)
		}
		result = append(result, CitationV1{
			ID: fmt.Sprintf("tool-citation-%d", index+1), TenantID: tenantID, CollectionID: collectionID,
			EvidenceID: value.EvidenceID, VersionID: value.VersionID, Locator: locator, SourceName: value.SourceFile,
		})
	}
	return result
}

func citationsFromEnterprise(enterprise map[string]any, tenantID, collectionID string) []CitationV1 {
	result := []CitationV1{}
	for index, value := range mapsFromAny(enterprise["provenance"]) {
		locatorParts := []string{}
		for _, key := range []string{"source_entry", "row_number", "row_hash", "timestamp", "source_time_seconds"} {
			if item := strings.TrimSpace(stringValueAny(value[key])); item != "" {
				locatorParts = append(locatorParts, key+":"+item)
			}
		}
		result = append(result, CitationV1{
			ArtifactID: stringValueAny(value["artifact_id"]), RunID: stringValueAny(value["run_id"]),

			ID: fmt.Sprintf("tool-citation-%d", index+1), TenantID: tenantID, CollectionID: collectionID,
			EvidenceID: stringValueAny(value["evidence_id"]), VersionID: stringValueAny(value["version_id"]),
			Locator: strings.Join(locatorParts, ","), SourceName: stringValueAny(value["source_file"]),
		})
		if value["artifact_id"] != nil {
			result[len(result)-1].Locator = stringValueAny(value["source_locator"])
			if result[len(result)-1].Locator == "" {
				result[len(result)-1].Locator = marshalJSONString(value["citation_locator"])
			}
		}
		if value["source_family"] == "video_anpr" {
			result[len(result)-1].Locator = marshalJSONString(value["source_locator"])
		}
	}
	return result
}

func citationIDs(citations []CitationV1) []string {
	result := make([]string, 0, len(citations))
	for _, citation := range citations {
		result = append(result, citation.ID)
	}
	return result
}

func citationIDsForObservationRow(row map[string]any, citations []CitationV1) []string {
	if row["match_semantic"] != nil {
		ids := map[string]bool{stringValueAny(row["artifact_id"]): true}
		for _, s := range mapsFromAny(row["contributing_observations"]) {
			ids[stringValueAny(s["artifact_id"])] = true
		}
		out := []string{}
		for _, c := range citations {
			if ids[c.ArtifactID] && c.EvidenceID == stringValueAny(row["evidence_id"]) && c.VersionID == stringValueAny(row["version_id"]) {
				out = append(out, c.ID)
			}
		}
		return out
	}
	if row["result_semantics"] == "video_anpr_observation" || row["result_semantics"] == "video_anpr_group" {
		matched := make([]string, 0, 1)
		for _, citation := range citations {
			locator := decodedCitationLocator(citation.Locator)
			if citation.EvidenceID == stringValueAny(row["evidence_id"]) && citation.VersionID == stringValueAny(row["version_id"]) && stringValueAny(locator["artifact_id"]) == stringValueAny(row["artifact_id"]) && row["artifact_id"] != nil {
				matched = append(matched, citation.ID)
			}
		}
		return matched
	}
	evidenceID := strings.TrimSpace(stringValueAny(row["evidence_id"]))
	versionID := strings.TrimSpace(stringValueAny(row["version_id"]))
	sourceName := strings.TrimSpace(stringValueAny(row["source_file"]))
	matched := make([]string, 0, 1)
	for _, citation := range citations {
		if evidenceID != "" && citation.EvidenceID != evidenceID {
			continue
		}
		if versionID != "" && citation.VersionID != versionID {
			continue
		}
		if sourceName != "" && citation.SourceName != sourceName {
			continue
		}
		matched = append(matched, citation.ID)
	}
	if len(matched) > 0 {
		return matched
	}
	return citationIDs(citations)
}

func sharedStatesFromCompositionStep(step TypedStepResultV1) (ResultState, ExecutionStatus) {
	switch step.Status {
	case "completed":
		return ResultStateResultsPresent, ExecutionStatusCompleted
	case "no_results":
		return ResultStateCompleteZero, ExecutionStatusCompleted
	case "skipped_dependency":
		return ResultStateNotProcessed, ExecutionStatusSkipped
	default:
		return ResultStateFailed, ExecutionStatusFailed
	}
}

func executionStatusForResultState(state ResultState) ExecutionStatus {
	switch state {
	case ResultStateResultsPresent, ResultStateCompleteZero, ResultStateNoMatchForFilter:
		return ExecutionStatusCompleted
	case ResultStateProcessing:
		return ExecutionStatusRunning
	case ResultStateNotProcessed:
		return ExecutionStatusSkipped
	case ResultStateFailed:
		return ExecutionStatusFailed
	case ResultStateUnavailable, ResultStateUnauthorized, ResultStateInvalidRequest:
		return ExecutionStatusSkipped
	default:
		return ExecutionStatusFailed
	}
}

func prohibitedExecutionField(value any) string {
	prohibited := map[string]struct{}{
		"sql": {}, "raw_sql": {}, "query_sql": {}, "statement": {}, "command": {}, "executable": {},
		"tool_url": {}, "callback_url": {}, "arbitrary_tool": {}, "implementation_key": {},
	}
	var inspect func(any) string
	inspect = func(current any) string {
		switch typed := current.(type) {
		case map[string]any:
			for key, nested := range typed {
				normalized := strings.ToLower(strings.TrimSpace(key))
				if _, blocked := prohibited[normalized]; blocked {
					return key
				}
				if found := inspect(nested); found != "" {
					return found
				}
			}
		case []any:
			for _, nested := range typed {
				if found := inspect(nested); found != "" {
					return found
				}
			}
		}
		return ""
	}
	return inspect(value)
}
