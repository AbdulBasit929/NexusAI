package main

import (
	"errors"
	"fmt"
	"strings"
)

func buildSingleCapabilityPlan(requestID string, req hybridQueryRequest, understanding QueryUnderstandingV1, capability ResolvedCapabilityV1) (GovernedExecutionPlanV1, error) {
	if err := understanding.Validate(); err != nil {
		return GovernedExecutionPlanV1{}, err
	}
	if !capability.Availability.Executable {
		return GovernedExecutionPlanV1{}, fmt.Errorf("capability %q is not executable: %s", capability.CapabilityID, capabilityRejectionSummary(capability))
	}
	if len(understanding.CandidateCapabilities) != 1 || understanding.CandidateCapabilities[0] != capability.CapabilityID {
		return GovernedExecutionPlanV1{}, errors.New("single-capability plan requires one resolved candidate")
	}
	parameters := executionParametersFromRequest(req)
	plan := GovernedExecutionPlanV1{
		ContractVersion: executionPlanContractV1,
		PlanID:          requestID + ":plan",
		Mode:            "single_operation",
		Workspace:       ExecutionWorkspaceScopeV1{TenantID: req.TenantID, CaseID: req.CollectionID, CollectionID: req.CollectionID, SubjectID: req.UserID},
		Steps: []ExecutionStepV1{{
			StepID: "step-1", CapabilityID: capability.CapabilityID, OperationID: capability.OperationID,
			Dependencies: []string{}, InputBindings: []ExecutionInputBindingV1{}, Parameters: parameters,
			EvidenceScope:          executionEvidenceScopeFromRequest(req),
			ExpectedResultContract: capability.ResultContract, ResultKind: capability.ResultKind,
			PresentationType: capability.PresentationType, CitationRequired: capability.CitationRequired,
		}},
		OutputContract:  capability.ResultContract,
		ResourcePolicy:  ExecutionResourcePolicyV1{ReadOnly: true, MaximumSteps: 1, MaximumRows: maxHybridLimit, MaximumOffset: 100000, MaximumKBResults: 20, MaximumTargets: maxCrossFamilyTargets},
		PolicyDecisions: []string{"workspace_scope_bound", "known_capability_only", "read_only_executor", "citations_capability_owned"},
	}
	if err := validateSingleCapabilityPlan(plan, capability); err != nil {
		return GovernedExecutionPlanV1{}, err
	}
	return plan, nil
}

func executionEvidenceScopeFromRequest(req hybridQueryRequest) ExecutionEvidenceScopeV1 {
	return ExecutionEvidenceScopeV1{
		CollectionID: req.CollectionID, EvidenceID: req.EvidenceID, EvidenceVersionID: req.EvidenceVersionID,
		SourceFamily: req.QueryScope.SourceFamily, AvailableResultFamilies: append([]string(nil), req.QueryScope.AvailableResultFamilies...),
		SourceFile: req.SourceFile, SourceFiles: structuredSourceFiles(req.SourceSet), BatchID: req.BatchID,
	}
}

func validateSingleCapabilityPlan(plan GovernedExecutionPlanV1, capability ResolvedCapabilityV1) error {
	if err := plan.ValidateShape(); err != nil {
		return err
	}
	step := plan.Steps[0]
	if step.Parameters.Group != nil || step.Parameters.Compare != nil || len(step.Parameters.Projection) > 0 {
		if capability.Template != "canonical_records" {
			return errors.New("canonical algebra requires the canonical_records executor")
		}
		request := requestWithExecutionParameters(hybridQueryRequest{}, step.Parameters)
		if err := validateCanonicalProjection(request.Projection); err != nil {
			return err
		}
		if err := validateCanonicalGroupRequest(request); err != nil {
			return err
		}
		if err := validateCanonicalCompareRequest(request); err != nil {
			return err
		}
		if (request.Group != nil || request.Compare != nil) && (plan.Mode != "single_operation" || step.Parameters.Limit != canonicalGroupLimit) {
			return errors.New("group requires one bounded step with the canonical group ceiling")
		}
	}
	if step.Parameters.TextQuery != nil {
		if err := step.Parameters.TextQuery.Validate(); err != nil {
			return err
		}
		if !isFamilyDerivedTextTemplate(queryTemplateNameByOperationID(step.OperationID)) {
			return errors.New("text semantic operation mismatch")
		}
	}
	if step.CapabilityID != capability.CapabilityID || step.OperationID != capability.OperationID || !capability.Availability.Executable {
		return errors.New("execution plan capability does not match one executable resolved capability")
	}
	if step.ExpectedResultContract != capability.ResultContract || step.ResultKind != capability.ResultKind || step.PresentationType != capability.PresentationType {
		return errors.New("execution plan result contract does not match capability metadata")
	}
	if step.CitationRequired != capability.CitationRequired {
		return errors.New("execution plan cannot override capability citation policy")
	}
	for _, required := range capability.RequiredParameters {
		if !executionParameterPresent(step.Parameters, required) {
			return fmt.Errorf("required execution parameter %q is missing", required)
		}
	}
	if step.Parameters.Direction != "" && canonicalEventDirection(step.Parameters.Direction) == "" {
		return errors.New("execution direction is invalid")
	}
	if step.Parameters.SortDirection != "" && canonicalSortDirection(step.Parameters.SortDirection) == "" {
		return errors.New("execution sort direction is invalid")
	}
	if step.Parameters.SourceFile != "" && !capability.SourceScoped {
		return errors.New("execution source-file scope is not supported by the resolved capability")
	}
	if step.Parameters.SourceSet != nil {
		if !capability.SourceScoped {
			return errors.New("execution source-set scope is not supported by the resolved capability")
		}
		if err := validateStructuredSourceSet(*step.Parameters.SourceSet); err != nil {
			return err
		}
	}
	if step.Parameters.BatchID != "" && !containsString(capability.OptionalParameters, "batch_id") && !containsString(capability.RequiredParameters, "batch_id") {
		return errors.New("execution batch scope is not supported by the resolved capability")
	}
	return nil
}

func executionParameterPresent(parameters ExecutionParametersV1, name string) bool {
	switch name {
	case "target":
		return strings.TrimSpace(parameters.Target) != "" || len(parameters.Targets) > 0
	case "evidence_id":
		return strings.TrimSpace(parameters.EvidenceID) != ""
	case "plate":
		return strings.TrimSpace(parameters.Plate) != ""
	case "start_seconds":
		return parameters.StartSeconds != nil
	case "end_seconds":
		return parameters.EndSeconds != nil
	case "date_from":
		return strings.TrimSpace(parameters.DateFrom) != ""
	case "date_to":
		return strings.TrimSpace(parameters.DateTo) != ""
	case "limit":
		return parameters.Limit > 0
	case "source_file":
		return strings.TrimSpace(parameters.SourceFile) != ""
	case "source_set":
		return parameters.SourceSet != nil && validateStructuredSourceSet(*parameters.SourceSet) == nil
	case "batch_id":
		return strings.TrimSpace(parameters.BatchID) != ""
	default:
		return false
	}
}

func executeGovernedSingleCapability(ctxQuery func(string) (map[string]any, error), plan GovernedExecutionPlanV1, capability ResolvedCapabilityV1) (map[string]any, error) {
	if err := validateSingleCapabilityPlan(plan, capability); err != nil {
		return nil, err
	}
	if capability.ExecutionMode != "deterministic" {
		return nil, errors.New("single-capability deterministic executor received a non-deterministic capability")
	}
	return ctxQuery(capability.Template)
}

// executeGovernedRecordsCapability keeps one typed plan and one allowlisted
// implementation key for both exact-record and retained hybrid operations. A
// hybrid capability may orchestrate its existing SQL/KB/model stages, but it
// must never be forced through the deterministic-only executor.
func executeGovernedRecordsCapability(ctxQuery func(string) (map[string]any, error), plan GovernedExecutionPlanV1, capability ResolvedCapabilityV1) (map[string]any, error) {
	if err := validateSingleCapabilityPlan(plan, capability); err != nil {
		return nil, err
	}
	switch capability.ExecutionMode {
	case "deterministic":
		return executeGovernedSingleCapability(ctxQuery, plan, capability)
	case "legacy_hybrid":
		return ctxQuery(capability.Template)
	default:
		return nil, fmt.Errorf("records executor does not support capability mode %q", capability.ExecutionMode)
	}
}

// Retrieval-only capabilities use the bounded KB and derived-text executors;
// they must not be sent through the records-SQL dispatcher merely because the
// legacy intent classifier labeled the natural-language question as records.
func shouldExecuteRecordsCapability(intent queryIntent, executionMode string) bool {
	return (intent == intentRecords || intent == intentHybrid) && executionMode != "retrieval"
}

func shouldExecuteEvidenceRetrieval(intent queryIntent, executionMode string) bool {
	return intent == intentSemantic || intent == intentHybrid || executionMode == "retrieval"
}
