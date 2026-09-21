package main

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
)

//go:embed contracts/forensic-platform-v1.json
var forensicPlatformCatalogJSON []byte

type AdapterDescriptorV1 struct {
	ID                         string   `json:"id"`
	Version                    string   `json:"version"`
	FamilyIDs                  []string `json:"family_ids"`
	Formats                    []string `json:"formats"`
	OperationIDs               []string `json:"operation_ids"`
	ImplementationKey          string   `json:"implementation_key"`
	ImplementationStatus       string   `json:"implementation_status"`
	ConfigurationSchemaVersion string   `json:"configuration_schema_version"`
	OutputContractVersion      string   `json:"output_contract_version"`
	ResourceProfile            string   `json:"resource_profile"`
	PreservesLegacyOutput      bool     `json:"preserves_legacy_output"`
}

type FamilyOperationDescriptorV1 struct {
	ID               string   `json:"id"`
	Version          string   `json:"version"`
	FamilyID         string   `json:"family_id"`
	Kind             string   `json:"kind"`
	Implementation   string   `json:"implementation"`
	RequiredTools    []string `json:"required_tools"`
	InputContract    string   `json:"input_contract"`
	OutputContract   string   `json:"output_contract"`
	CitationRequired bool     `json:"citation_required"`
	Maturity         string   `json:"maturity"`
}

type ModelRoleReferenceV1 struct {
	ID              string `json:"id"`
	Kind            string `json:"kind"`
	ConfiguredModel string `json:"configured_model"`
	Status          string `json:"status"`
	SelectionPolicy string `json:"selection_policy"`
}

type SpecialistAgentManifestV1 struct {
	ID                string   `json:"id"`
	Version           string   `json:"version"`
	FamilyIDs         []string `json:"family_ids"`
	OperationIDs      []string `json:"operation_ids"`
	ToolAllowlist     []string `json:"tool_allowlist"`
	ModelRoleIDs      []string `json:"model_role_ids"`
	ScopeSource       string   `json:"scope_source"`
	Status            string   `json:"status"`
	ResponseContract  string   `json:"response_contract"`
	RequiredCitations bool     `json:"required_citations"`
	FallbackAgentID   string   `json:"fallback_agent_id"`
}

type ForensicPlatformCatalogV1 struct {
	SchemaVersion     string                        `json:"schema_version"`
	CatalogVersion    string                        `json:"catalog_version"`
	QueryPlanContract string                        `json:"query_plan_contract"`
	ResponseContract  string                        `json:"response_contract"`
	AdapterLifecycle  []string                      `json:"adapter_lifecycle"`
	Adapters          []AdapterDescriptorV1         `json:"adapters"`
	Operations        []FamilyOperationDescriptorV1 `json:"operations"`
	ModelRoles        []ModelRoleReferenceV1        `json:"model_roles"`
	Agents            []SpecialistAgentManifestV1   `json:"agents"`
}

type QueryEntityV1 struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type QueryTimeRangeV1 struct {
	From     string `json:"from,omitempty"`
	To       string `json:"to,omitempty"`
	Timezone string `json:"timezone"`
}

type QueryFilterV1 struct {
	Field  string `json:"field"`
	Op     string `json:"op"`
	Value  any    `json:"value,omitempty"`
	Values []any  `json:"values,omitempty"`
}

type QuerySortV1 struct {
	Field     string `json:"field"`
	Direction string `json:"direction"`
}

type ClarificationV1 struct {
	Field    string   `json:"field"`
	Question string   `json:"question"`
	Options  []string `json:"options,omitempty"`
}

type ForensicQueryPlanV1 struct {
	ContractVersion string              `json:"contract_version"`
	TenantID        string              `json:"tenant_id"`
	CaseID          string              `json:"case_id"`
	CollectionID    string              `json:"collection_id"`
	Intent          string              `json:"intent"`
	Families        []string            `json:"families"`
	Entities        []QueryEntityV1     `json:"entities"`
	TimeRange       QueryTimeRangeV1    `json:"time_range"`
	Filters         []QueryFilterV1     `json:"filters"`
	Measures        []string            `json:"measures"`
	GroupBy         []string            `json:"group_by"`
	Group           *CanonicalGroupV1   `json:"group,omitempty"`
	Compare         *CanonicalCompareV1 `json:"compare,omitempty"`
	Sort            []QuerySortV1       `json:"sort"`
	Limit           int                 `json:"limit"`
	RequiredTools   []string            `json:"required_tools"`
	Clarifications  []ClarificationV1   `json:"clarifications"`
}

type FindingV1 struct {
	ID          string         `json:"id"`
	Statement   string         `json:"statement"`
	Measures    map[string]any `json:"measures"`
	CitationIDs []string       `json:"citation_ids"`
}

type CitationV1 struct {
	ArtifactID   string `json:"artifact_id,omitempty"`
	RunID        string `json:"run_id,omitempty"`
	ID           string `json:"id"`
	TenantID     string `json:"tenant_id"`
	CollectionID string `json:"collection_id"`
	EvidenceID   string `json:"evidence_id"`
	VersionID    string `json:"version_id"`
	Locator      string `json:"locator"`
	SourceName   string `json:"source_name"`
}

type InferredRelationshipV1 struct {
	SourceEntity string   `json:"source_entity"`
	Relation     string   `json:"relation"`
	TargetEntity string   `json:"target_entity"`
	Confidence   float64  `json:"confidence"`
	CitationIDs  []string `json:"citation_ids"`
}

type ModelInterpretationV1 struct {
	Text          string   `json:"text"`
	ModelRoleID   string   `json:"model_role_id"`
	ModelID       string   `json:"model_id"`
	ModelRevision string   `json:"model_revision"`
	CitationIDs   []string `json:"citation_ids"`
	Fallback      bool     `json:"fallback"`
}

type UnsupportedOperationV1 struct {
	OperationID        string `json:"operation_id"`
	Reason             string `json:"reason"`
	RequiredCapability string `json:"required_capability"`
}

type TableColumnV1 struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Type  string `json:"type"`
}

type TableResultV1 struct {
	ID      string           `json:"id"`
	Title   string           `json:"title"`
	Columns []TableColumnV1  `json:"columns"`
	Rows    []map[string]any `json:"rows"`
}

type VisualizationV1 struct {
	ID          string         `json:"id"`
	Type        string         `json:"type"`
	Title       string         `json:"title"`
	Spec        map[string]any `json:"spec"`
	CitationIDs []string       `json:"citation_ids"`
}

type ExecutionTraceV1 struct {
	Route      []string         `json:"route"`
	ToolIDs    []string         `json:"tool_ids"`
	AdapterIDs []string         `json:"adapter_ids"`
	ModelRoles []string         `json:"model_roles"`
	Parameters map[string]any   `json:"parameters"`
	LatencyMS  map[string]int64 `json:"latency_ms"`
}

type NextActionV1 struct {
	Label string `json:"label"`
	Query string `json:"query,omitempty"`
	Href  string `json:"href,omitempty"`
}

const (
	EnterpriseResultStateResultsPresent   = "results_present"
	EnterpriseResultStateCompleteZero     = "complete_zero_results"
	EnterpriseResultStateNoMatchForFilter = "no_match_for_filter"
	EnterpriseResultStateNotProcessed     = "not_processed"
	EnterpriseResultStateProcessing       = "processing"
	EnterpriseResultStateFailed           = "failed"
	EnterpriseResultStateUnavailable      = "unavailable"
	EnterpriseResultStateUnauthorized     = "unauthorized"
	EnterpriseResultStateInvalidRequest   = "invalid_request"
)

type EnterpriseResponseV1 struct {
	ContractVersion       string                   `json:"contract_version"`
	RequestID             string                   `json:"request_id"`
	CaseID                string                   `json:"case_id"`
	CollectionID          string                   `json:"collection_id"`
	Status                string                   `json:"status"`
	ResultState           string                   `json:"result_state"`
	ProcessingState       string                   `json:"processing_state,omitempty"`
	RowCount              int                      `json:"row_count"`
	OperationID           string                   `json:"operation_id,omitempty"`
	ExecutiveAnswer       string                   `json:"executive_answer"`
	DeterministicFindings []FindingV1              `json:"deterministic_findings"`
	SemanticEvidence      []CitationV1             `json:"semantic_evidence"`
	InferredRelationships []InferredRelationshipV1 `json:"inferred_relationships"`
	ModelInterpretation   *ModelInterpretationV1   `json:"model_interpretation,omitempty"`
	ObservationPackets    []ObservationPacketV1    `json:"observation_packets,omitempty"`
	ToolResults           []ToolResultV1           `json:"tool_results,omitempty"`
	PlanValidation        *PlanValidationResultV1  `json:"plan_validation,omitempty"`
	Clarification         *ClarificationRequestV1  `json:"clarification,omitempty"`
	UnsupportedOperations []UnsupportedOperationV1 `json:"unsupported_operations"`
	Limitations           []string                 `json:"limitations"`
	NextActions           []NextActionV1           `json:"next_actions"`
	Tables                []TableResultV1          `json:"tables"`
	Visualizations        []VisualizationV1        `json:"visualizations"`
	ExecutionTrace        ExecutionTraceV1         `json:"execution_trace"`
}

var (
	forensicPlatformCatalogOnce sync.Once
	forensicPlatformCatalog     ForensicPlatformCatalogV1
	forensicPlatformCatalogErr  error
)

func loadForensicPlatformCatalog() (ForensicPlatformCatalogV1, error) {
	forensicPlatformCatalogOnce.Do(func() {
		if err := json.Unmarshal(forensicPlatformCatalogJSON, &forensicPlatformCatalog); err != nil {
			forensicPlatformCatalogErr = fmt.Errorf("decode forensic platform catalog: %w", err)
			return
		}
		forensicPlatformCatalog, forensicPlatformCatalogErr = reconciledForensicPlatformCatalog(forensicPlatformCatalog)
		if forensicPlatformCatalogErr != nil {
			return
		}
		forensicPlatformCatalogErr = validateForensicPlatformCatalog(forensicPlatformCatalog)
	})
	return forensicPlatformCatalog, forensicPlatformCatalogErr
}

func validateForensicPlatformCatalog(catalog ForensicPlatformCatalogV1) error {
	if catalog.SchemaVersion != "forensics.platform/v1" {
		return fmt.Errorf("unsupported forensic platform schema %q", catalog.SchemaVersion)
	}
	if catalog.QueryPlanContract != "forensics.query-plan/v1" || catalog.ResponseContract != "forensics.enterprise-response/v1" {
		return errors.New("forensic query or response contract version is invalid")
	}
	expectedLifecycle := []string{"detect", "validate", "plan", "extract", "normalize", "derive", "persist", "index", "verify", "capabilities"}
	if strings.Join(catalog.AdapterLifecycle, "|") != strings.Join(expectedLifecycle, "|") {
		return errors.New("forensic adapter lifecycle is incomplete or out of order")
	}
	operationIDs := map[string]struct{}{}
	for _, operation := range catalog.Operations {
		if operation.ID == "" || operation.Version == "" || operation.FamilyID == "" || !containsString([]string{"deterministic", "retrieval", "legacy_hybrid"}, operation.Kind) {
			return fmt.Errorf("invalid forensic operation descriptor %q", operation.ID)
		}
		if !containsString([]string{"PLANNED", "ENGINEERING_ONLY", "LIMITED", "CERTIFIED"}, operation.Maturity) {
			return fmt.Errorf("invalid forensic operation maturity %q for %q", operation.Maturity, operation.ID)
		}
		if _, exists := operationIDs[operation.ID]; exists {
			return fmt.Errorf("duplicate forensic operation %q", operation.ID)
		}
		operationIDs[operation.ID] = struct{}{}
	}
	adapterIDs := map[string]struct{}{}
	for _, adapter := range catalog.Adapters {
		if adapter.ID == "" || adapter.Version == "" || adapter.ImplementationKey == "" || !adapter.PreservesLegacyOutput {
			return fmt.Errorf("invalid forensic adapter descriptor %q", adapter.ID)
		}
		if _, exists := adapterIDs[adapter.ID]; exists {
			return fmt.Errorf("duplicate forensic adapter %q", adapter.ID)
		}
		adapterIDs[adapter.ID] = struct{}{}
		for _, operationID := range adapter.OperationIDs {
			if _, exists := operationIDs[operationID]; !exists {
				return fmt.Errorf("adapter %q references unknown operation %q", adapter.ID, operationID)
			}
		}
	}
	modelRoleIDs := map[string]struct{}{}
	for _, role := range catalog.ModelRoles {
		if role.ID == "" || role.Kind == "" || role.SelectionPolicy == "" {
			return fmt.Errorf("invalid forensic model role %q", role.ID)
		}
		modelRoleIDs[role.ID] = struct{}{}
	}
	agentIDs := map[string]struct{}{}
	for _, agent := range catalog.Agents {
		if agent.ID == "" || agent.Version == "" || agent.ScopeSource == "" || agent.ResponseContract != catalog.ResponseContract {
			return fmt.Errorf("invalid forensic agent manifest %q", agent.ID)
		}
		if _, exists := agentIDs[agent.ID]; exists {
			return fmt.Errorf("duplicate forensic agent %q", agent.ID)
		}
		agentIDs[agent.ID] = struct{}{}
		for _, operationID := range agent.OperationIDs {
			if operationID == "*" {
				continue
			}
			if _, exists := operationIDs[operationID]; !exists {
				return fmt.Errorf("agent %q references unknown operation %q", agent.ID, operationID)
			}
		}
		for _, roleID := range agent.ModelRoleIDs {
			if _, exists := modelRoleIDs[roleID]; !exists {
				return fmt.Errorf("agent %q references unknown model role %q", agent.ID, roleID)
			}
		}
	}
	return nil
}

func forensicPlatformDiscoveryHandler(section string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
			return
		}
		catalog, err := loadForensicPlatformCatalog()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		response := map[string]any{
			"schema_version":  catalog.SchemaVersion,
			"catalog_version": catalog.CatalogVersion,
		}
		switch section {
		case "adapters":
			response["adapter_lifecycle"] = catalog.AdapterLifecycle
			response["adapters"] = catalog.Adapters
		case "operations":
			response["operations"] = catalog.Operations
		case "agents":
			response["agents"] = catalog.Agents
			response["model_roles"] = catalog.ModelRoles
		case "contracts":
			response["query_plan_contract"] = catalog.QueryPlanContract
			response["response_contract"] = catalog.ResponseContract
			response["query_plan_fields"] = sortedContractFields([]string{
				"tenant_id", "case_id", "collection_id", "intent", "families", "entities", "time_range", "filters", "measures", "group_by", "projection", "group", "compare", "sort", "limit", "required_tools", "clarifications",
			})
			response["response_sections"] = sortedContractFields([]string{
				"executive_answer", "deterministic_findings", "semantic_evidence", "inferred_relationships", "model_interpretation", "unsupported_operations", "limitations", "next_actions", "tables", "visualizations", "execution_trace",
			})
			response["workflow_contracts"] = []map[string]any{{
				"id":        "evidence_reprocess_plan",
				"version":   evidenceReprocessPlanContractV1,
				"method":    http.MethodGet,
				"path":      "/api/v1/forensics/cases/{case_id}/evidence/{evidence_id}/reprocess-plan",
				"execution": "read_only_approval_required",
			}}
		default:
			writeError(w, http.StatusNotFound, errors.New("unknown forensic platform discovery section"))
			return
		}
		writeJSON(w, http.StatusOK, response)
	}
}

func sortedContractFields(fields []string) []string {
	result := append([]string(nil), fields...)
	sort.Strings(result)
	return result
}
