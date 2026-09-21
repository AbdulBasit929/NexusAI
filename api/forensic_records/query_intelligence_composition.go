package main

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	maxCompositionSteps       = 3
	maxCompositionRows        = 100
	maxCompositionTotalRows   = 300
	maxCompositionKBChunks    = 12
	maxCompositionModelCalls  = 1
	maxCompositionTimeoutMS   = 60000
	compositionResultContract = "forensics.composition-result/v1"
)

type TypedIntermediateFieldV1 struct {
	Type  string `json:"type"`
	Value any    `json:"value"`
}

type ExecutionCitationV1 struct {
	EvidenceID    string `json:"evidence_id,omitempty"`
	VersionID     string `json:"version_id,omitempty"`
	SourceFile    string `json:"source_file,omitempty"`
	SourceRow     int64  `json:"source_row,omitempty"`
	SourceLocator string `json:"source_locator,omitempty"`
}

type TypedStepResultV1 struct {
	StepID         string                              `json:"step_id"`
	CapabilityID   string                              `json:"capability_id"`
	Status         string                              `json:"status"`
	ResultContract string                              `json:"result_contract"`
	Fields         map[string]TypedIntermediateFieldV1 `json:"fields"`
	Citations      []ExecutionCitationV1               `json:"citations"`
	FailureCode    string                              `json:"failure_code,omitempty"`
	Limitations    []string                            `json:"limitations"`
}

type CompositionClaimLineageV1 struct {
	ClaimID       string                `json:"claim_id"`
	SemanticState string                `json:"semantic_state"`
	StepIDs       []string              `json:"step_ids"`
	CapabilityIDs []string              `json:"capability_ids"`
	Citations     []ExecutionCitationV1 `json:"citations"`
}

type GovernedCompositionResultV1 struct {
	ContractVersion string                      `json:"contract_version"`
	PlanID          string                      `json:"plan_id"`
	Status          string                      `json:"status"`
	Steps           []TypedStepResultV1         `json:"steps"`
	Claims          []CompositionClaimLineageV1 `json:"claims"`
	Limitations     []string                    `json:"limitations"`
	ElapsedMS       int64                       `json:"elapsed_ms"`
}

type boundedCompositionSpec struct {
	Templates []string
	Reason    string
}

// detectBoundedComposition recognizes only explicit, pre-approved compositions.
// Existing cross-family operations keep the single-capability fast path.
func detectBoundedComposition(query string) boundedCompositionSpec {
	normalized := normalizeQuestion(query)
	if containsAny(normalized, []string{"correlate across", "cross-family correlation", "cross family correlation"}) {
		return boundedCompositionSpec{}
	}
	hasSubscriber := containsAny(normalized, []string{"subscriber", "sim", "imsi", "iccid", "device association"})
	hasCDRActivity := containsAny(normalized, []string{"cdr activity", "call activity", "temporal activity", "hourly calls"})
	hasFrequentContacts := containsAny(normalized, []string{"frequent contacts", "contact most", "top contacts"})
	if hasSubscriber && hasCDRActivity {
		return boundedCompositionSpec{Templates: []string{"subscriber_identity_lookup", "temporal_activity"}, Reason: "explicit_cdr_plus_subscriber"}
	}
	if hasSubscriber && hasFrequentContacts {
		return boundedCompositionSpec{Templates: []string{"subscriber_identity_lookup", "frequent_contacts"}, Reason: "explicit_cdr_contacts_plus_subscriber"}
	}
	phone, plate := compositionPhoneAndPlate(query)
	if phone != "" && plate != "" && containsAny(normalized, []string{"evidence", "related", "show", "find", "phone", "number", "plate", "anpr"}) {
		templates := []string{"cross_family_correlation", "anpr_sightings"}
		reason := "explicit_phone_plus_plate"
		if containsAny(normalized, []string{"document mention", "documents mention", "document", "documents", "ocr mention"}) {
			templates = append(templates, "document_search")
			reason = "explicit_phone_plus_plate_plus_documents"
		}
		return boundedCompositionSpec{Templates: templates, Reason: reason}
	}
	return boundedCompositionSpec{}
}

func compositionPhoneAndPlate(query string) (string, string) {
	phone, plate := "", ""
	for _, target := range extractTargets(query) {
		switch {
		case phone == "" && classifyTargetType(target) == "phone":
			phone = target
		case plate == "" && looksLikeANPRPlateTarget(target):
			plate = target
		}
	}
	return phone, plate
}

func applyCompositionUnderstanding(understanding QueryUnderstandingV1, spec boundedCompositionSpec) (QueryUnderstandingV1, error) {
	if len(spec.Templates) == 0 {
		return understanding, nil
	}
	capabilities := make([]string, 0, len(spec.Templates))
	families := []string{}
	for _, name := range spec.Templates {
		template, ok := queryTemplateByName(name)
		if !ok {
			return understanding, fmt.Errorf("composition references unknown template %q", name)
		}
		capabilities = append(capabilities, template.OperationID)
		families = append(families, template.FamilyID)
	}
	understanding.CandidateCapabilities = capabilities
	understanding.Intent = QueryIntentCorrelate
	understanding.Families = QueryFamilySetV1{Kind: "multiple", Primary: families[0], Secondary: uniqueStrings(families[1:])}
	return understanding, understanding.Validate()
}

func buildBoundedCompositionPlan(requestID string, req hybridQueryRequest, understanding QueryUnderstandingV1, capabilities []ResolvedCapabilityV1) (GovernedExecutionPlanV1, error) {
	if err := understanding.Validate(); err != nil {
		return GovernedExecutionPlanV1{}, err
	}
	if len(capabilities) < 2 || len(capabilities) > maxCompositionSteps || len(understanding.CandidateCapabilities) != len(capabilities) {
		return GovernedExecutionPlanV1{}, errors.New("bounded composition requires two or three resolved capabilities")
	}
	byID := make(map[string]ResolvedCapabilityV1, len(capabilities))
	for _, capability := range capabilities {
		if !capability.Availability.Executable {
			return GovernedExecutionPlanV1{}, fmt.Errorf("capability %q is not executable: %s", capability.CapabilityID, capabilityRejectionSummary(capability))
		}
		byID[capability.CapabilityID] = capability
	}
	parameters := executionParametersFromRequest(req)
	steps := make([]ExecutionStepV1, 0, len(understanding.CandidateCapabilities))
	for index, capabilityID := range understanding.CandidateCapabilities {
		capability, ok := byID[capabilityID]
		if !ok {
			return GovernedExecutionPlanV1{}, fmt.Errorf("candidate capability %q was not resolved", capabilityID)
		}
		step := ExecutionStepV1{
			StepID: fmt.Sprintf("step-%d", index+1), CapabilityID: capability.CapabilityID, OperationID: capability.OperationID,
			Dependencies: []string{}, InputBindings: []ExecutionInputBindingV1{}, Parameters: parameters,
			EvidenceScope:          executionEvidenceScopeFromRequest(req),
			ExpectedResultContract: capability.ResultContract, ResultKind: capability.ResultKind,
			PresentationType: capability.PresentationType, CitationRequired: capability.CitationRequired,
		}
		if phone, plate := compositionPhoneAndPlate(req.Query); phone != "" && plate != "" {
			switch capability.Template {
			case "cross_family_correlation":
				step.Parameters.Target = phone
				step.Parameters.Targets = []string{phone}
			case "anpr_sightings":
				step.Parameters.Target = plate
				step.Parameters.Targets = []string{plate}
			case "document_search":
				step.Parameters.Target = phone
				step.Parameters.Targets = []string{phone, plate}
			}
		}
		if index > 0 && steps[0].CapabilityID == "subscriber.identity_lookup" && containsString([]string{"cdr.temporal_activity", "cdr.frequent_contacts"}, capability.CapabilityID) {
			step.Dependencies = []string{steps[0].StepID}
			step.InputBindings = []ExecutionInputBindingV1{{SourceStepID: steps[0].StepID, SourceField: "exact_target", TargetParameter: "target", ValueType: "string"}}
			step.Parameters.Target = ""
			step.Parameters.Targets = []string{}
		}
		steps = append(steps, step)
	}
	plan := GovernedExecutionPlanV1{
		ContractVersion: executionPlanContractV1, PlanID: requestID + ":composition", Mode: "bounded_composition",
		Workspace: ExecutionWorkspaceScopeV1{TenantID: req.TenantID, CaseID: req.CollectionID, CollectionID: req.CollectionID, SubjectID: req.UserID},
		Steps:     steps, OutputContract: compositionResultContract,
		ResourcePolicy:  ExecutionResourcePolicyV1{ReadOnly: true, MaximumSteps: maxCompositionSteps, MaximumRows: maxCompositionRows, MaximumOffset: 100000, MaximumKBResults: 20, MaximumTargets: maxCrossFamilyTargets, MaximumIntermediateRows: maxCompositionTotalRows, MaximumRetrievalChunks: maxCompositionKBChunks, MaximumModelCalls: maxCompositionModelCalls, TimeoutMilliseconds: maxCompositionTimeoutMS},
		PolicyDecisions: []string{"workspace_scope_bound", "known_capabilities_only", "read_only_executor", "full_plan_validated_before_execution", "acyclic_dependencies", "typed_intermediate_results", "exact_facts_preserved", "claim_lineage_required"},
	}
	if err := validateBoundedCompositionPlan(plan, capabilities); err != nil {
		return GovernedExecutionPlanV1{}, err
	}
	return plan, nil
}

func executionParametersFromRequest(req hybridQueryRequest) ExecutionParametersV1 {
	parameters := canonicalExecutionParameters(req)
	parameters.Group = req.Group
	parameters.Compare = req.Compare
	parameters.Projection = append([]string(nil), req.Projection...)
	parameters.SourceNative = req.SourceNative
	if req.Group != nil || req.Compare != nil {
		parameters.Limit = canonicalGroupLimit
	}
	return parameters
}

func canonicalExecutionParameters(req hybridQueryRequest) ExecutionParametersV1 {
	return ExecutionParametersV1{Target: req.Target, Targets: nonNilStrings(req.Targets), EvidenceID: req.EvidenceID, EvidenceVersionID: req.EvidenceVersionID, TextQuery: req.TextQuery, TranscriptMode: req.TranscriptMode, ExactTerm: req.ExactTerm, QueryLanguage: req.QueryLanguage, Plate: req.Plate, StartSeconds: req.StartSeconds, EndSeconds: req.EndSeconds, RecordType: req.RecordType, DateFrom: req.DateFrom, DateTo: req.DateTo, Direction: req.Direction, SourceFile: req.SourceFile, SourceSet: req.SourceSet, BatchID: req.BatchID, RawPayloadFilters: append([]CanonicalPayloadFilter(nil), req.RawPayloadFilters...), FieldFilters: append([]CanonicalFieldFilter(nil), req.FieldFilters...), FieldExists: nonNilStrings(req.FieldExists), FieldNotExists: nonNilStrings(req.FieldNotExists), Limit: req.Limit, Offset: req.Offset, SortBy: req.SortBy, SortDirection: req.SortDirection, MaxKBResults: req.MaxKBResults, RetrievalMode: req.RetrievalMode}
}

func requestWithExecutionParameters(req hybridQueryRequest, parameters ExecutionParametersV1) hybridQueryRequest {
	req.Group = parameters.Group
	req.Compare = parameters.Compare
	req.Projection = append([]string(nil), parameters.Projection...)
	req.SourceNative = parameters.SourceNative
	req.Target = parameters.Target
	req.Targets = append([]string(nil), parameters.Targets...)
	req.EvidenceID = parameters.EvidenceID
	req.EvidenceVersionID = parameters.EvidenceVersionID
	req.TextQuery = parameters.TextQuery
	req.TranscriptMode = parameters.TranscriptMode
	req.ExactTerm = parameters.ExactTerm
	req.QueryLanguage = parameters.QueryLanguage
	req.Plate = parameters.Plate
	req.StartSeconds = parameters.StartSeconds
	req.EndSeconds = parameters.EndSeconds
	req.RecordType = parameters.RecordType
	req.DateFrom = parameters.DateFrom
	req.DateTo = parameters.DateTo
	req.Direction = parameters.Direction
	req.SourceFile = parameters.SourceFile
	req.SourceSet = parameters.SourceSet
	req.BatchID = parameters.BatchID
	req.RawPayloadFilters = append([]CanonicalPayloadFilter(nil), parameters.RawPayloadFilters...)
	req.FieldFilters = append([]CanonicalFieldFilter(nil), parameters.FieldFilters...)
	req.FieldExists = append([]string(nil), parameters.FieldExists...)
	req.FieldNotExists = append([]string(nil), parameters.FieldNotExists...)
	req.Limit = parameters.Limit
	if parameters.Group != nil || parameters.Compare != nil {
		req.Limit = 0 // The plan carries the output ceiling, not input pagination.
	}
	req.Offset = parameters.Offset
	req.SortBy = parameters.SortBy
	req.SortDirection = parameters.SortDirection
	req.MaxKBResults = parameters.MaxKBResults
	req.RetrievalMode = parameters.RetrievalMode
	return req
}

func validateBoundedCompositionPlan(plan GovernedExecutionPlanV1, capabilities []ResolvedCapabilityV1) error {
	if err := plan.ValidateShape(); err != nil {
		return err
	}
	if plan.Mode != "bounded_composition" {
		return errors.New("multi-capability validator requires bounded_composition mode")
	}
	byCapability := map[string]ResolvedCapabilityV1{}
	for _, capability := range capabilities {
		byCapability[capability.CapabilityID] = capability
	}
	steps := map[string]ExecutionStepV1{}
	usedCapabilities := map[string]bool{}
	for _, step := range plan.Steps {
		capability, ok := byCapability[step.CapabilityID]
		if !ok || !capability.Availability.Executable || step.OperationID != capability.OperationID {
			return fmt.Errorf("step %q does not reference one authorized executable capability", step.StepID)
		}
		if usedCapabilities[step.CapabilityID] {
			return fmt.Errorf("capability %q is duplicated in one composition", step.CapabilityID)
		}
		usedCapabilities[step.CapabilityID] = true
		if step.ExpectedResultContract != capability.ResultContract || step.ResultKind != capability.ResultKind || step.PresentationType != capability.PresentationType || step.CitationRequired != capability.CitationRequired {
			return fmt.Errorf("step %q overrides governed capability metadata", step.StepID)
		}
		for _, required := range capability.RequiredParameters {
			if !executionParameterPresent(step.Parameters, required) && !bindingTargetsParameter(step.InputBindings, required) {
				return fmt.Errorf("step %q is missing required parameter %q", step.StepID, required)
			}
		}
		steps[step.StepID] = step
	}
	for _, step := range plan.Steps {
		for _, dependency := range step.Dependencies {
			if dependency == step.StepID || steps[dependency].StepID == "" {
				return fmt.Errorf("step %q has an invalid dependency %q", step.StepID, dependency)
			}
		}
		for _, binding := range step.InputBindings {
			if !containsString(step.Dependencies, binding.SourceStepID) || !validBinding(binding) {
				return fmt.Errorf("step %q has an invalid typed input binding", step.StepID)
			}
		}
	}
	if compositionHasCycle(plan.Steps) {
		return errors.New("composition dependency graph contains a cycle")
	}
	return nil
}

func validBinding(binding ExecutionInputBindingV1) bool {
	return binding.SourceStepID != "" && validCapabilityID(strings.ReplaceAll(binding.SourceStepID, "step-", "step.")) &&
		containsString([]string{"target", "targets", "date_from", "date_to"}, binding.TargetParameter) &&
		containsString([]string{"string", "string_list", "timestamp"}, binding.ValueType) &&
		binding.SourceField != "" && validCapabilityID("field."+strings.ReplaceAll(binding.SourceField, "_", "."))
}

func bindingTargetsParameter(bindings []ExecutionInputBindingV1, parameter string) bool {
	for _, binding := range bindings {
		if binding.TargetParameter == parameter {
			return true
		}
	}
	return false
}

func compositionHasCycle(steps []ExecutionStepV1) bool {
	dependencies := map[string][]string{}
	for _, step := range steps {
		dependencies[step.StepID] = step.Dependencies
	}
	visiting, visited := map[string]bool{}, map[string]bool{}
	var visit func(string) bool
	visit = func(id string) bool {
		if visiting[id] {
			return true
		}
		if visited[id] {
			return false
		}
		visiting[id] = true
		for _, dependency := range dependencies[id] {
			if visit(dependency) {
				return true
			}
		}
		delete(visiting, id)
		visited[id] = true
		return false
	}
	for id := range dependencies {
		if visit(id) {
			return true
		}
	}
	return false
}

func validateTypedStepResult(step ExecutionStepV1, result TypedStepResultV1) error {
	if result.StepID != step.StepID || result.CapabilityID != step.CapabilityID || result.ResultContract != step.ExpectedResultContract {
		return errors.New("intermediate result identity or contract mismatch")
	}
	if !containsString([]string{"completed", "no_results", "failed", "skipped_dependency"}, result.Status) {
		return fmt.Errorf("invalid intermediate result status %q", result.Status)
	}
	for name, field := range result.Fields {
		if name == "" || !containsString([]string{"string", "integer", "number", "boolean", "timestamp", "string_list"}, field.Type) || !typedValueMatches(field.Type, field.Value) {
			return fmt.Errorf("intermediate field %q does not match declared type %q", name, field.Type)
		}
	}
	if step.CitationRequired && result.Status == "completed" && len(result.Fields) > 0 && len(result.Citations) == 0 {
		return errors.New("completed factual result is missing required citations")
	}
	return nil
}

func typedValueMatches(kind string, value any) bool {
	switch kind {
	case "string", "timestamp":
		_, ok := value.(string)
		return ok
	case "integer":
		switch value.(type) {
		case int, int32, int64, uint, uint32, uint64:
			return true
		}
		return false
	case "number":
		switch value.(type) {
		case int, int32, int64, uint, uint32, uint64, float32, float64:
			return true
		}
		return false
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "string_list":
		_, ok := value.([]string)
		return ok
	default:
		return false
	}
}

func executeGovernedComposition(plan GovernedExecutionPlanV1, capabilities []ResolvedCapabilityV1, execute func(ExecutionStepV1, ResolvedCapabilityV1) TypedStepResultV1) (GovernedCompositionResultV1, error) {
	if err := validateBoundedCompositionPlan(plan, capabilities); err != nil {
		return GovernedCompositionResultV1{}, err
	}
	started := time.Now()
	byCapability := map[string]ResolvedCapabilityV1{}
	for _, capability := range capabilities {
		byCapability[capability.CapabilityID] = capability
	}
	ordered, err := topologicallyOrderedCompositionSteps(plan.Steps)
	if err != nil {
		return GovernedCompositionResultV1{}, err
	}
	completed := map[string]bool{}
	byStepResult := map[string]TypedStepResultV1{}
	results := make([]TypedStepResultV1, 0, len(ordered))
	failed := 0
	for _, step := range ordered {
		if time.Since(started) > time.Duration(plan.ResourcePolicy.TimeoutMilliseconds)*time.Millisecond {
			result := failedCompositionStep(step, "timeout", "The bounded composition timeout expired before this step could execute.")
			results = append(results, result)
			byStepResult[step.StepID] = result
			failed++
			continue
		}
		blocked := false
		for _, dependency := range step.Dependencies {
			if !completed[dependency] {
				blocked = true
			}
		}
		var result TypedStepResultV1
		if blocked {
			result = TypedStepResultV1{StepID: step.StepID, CapabilityID: step.CapabilityID, Status: "skipped_dependency", ResultContract: step.ExpectedResultContract, Fields: map[string]TypedIntermediateFieldV1{}, Citations: []ExecutionCitationV1{}, FailureCode: "dependency_failed", Limitations: []string{"A required upstream capability did not complete."}}
		} else {
			boundStep, bindErr := applyCompositionBindings(step, byStepResult)
			if bindErr != nil {
				return GovernedCompositionResultV1{}, fmt.Errorf("apply %s bindings: %w", step.StepID, bindErr)
			}
			step = boundStep
			result = execute(step, byCapability[step.CapabilityID])
		}
		if err := validateTypedStepResult(step, result); err != nil {
			return GovernedCompositionResultV1{}, fmt.Errorf("validate %s: %w", step.StepID, err)
		}
		if result.Status == "completed" {
			completed[step.StepID] = true
		} else {
			failed++
		}
		results = append(results, result)
		byStepResult[step.StepID] = result
	}
	status := "completed"
	limitations := []string{"Cross-family co-observation does not establish identity, ownership, causation, or intent."}
	noResults := 0
	for _, result := range results {
		if result.Status == "no_results" {
			noResults++
		}
	}
	if noResults == len(results) {
		status = "no_results"
	} else if failed == len(results) && noResults == 0 {
		status = "execution_failed"
	} else if failed > 0 {
		status = "partial_analysis"
	}
	claims := compositionClaims(results)
	return GovernedCompositionResultV1{ContractVersion: compositionResultContract, PlanID: plan.PlanID, Status: status, Steps: results, Claims: claims, Limitations: limitations, ElapsedMS: time.Since(started).Milliseconds()}, nil
}

func topologicallyOrderedCompositionSteps(steps []ExecutionStepV1) ([]ExecutionStepV1, error) {
	byID := map[string]ExecutionStepV1{}
	indegree := map[string]int{}
	dependents := map[string][]string{}
	for _, step := range steps {
		byID[step.StepID] = step
		indegree[step.StepID] = len(step.Dependencies)
		for _, dependency := range step.Dependencies {
			dependents[dependency] = append(dependents[dependency], step.StepID)
		}
	}
	queue := []string{}
	for _, step := range steps {
		if indegree[step.StepID] == 0 {
			queue = append(queue, step.StepID)
		}
	}
	ordered := make([]ExecutionStepV1, 0, len(steps))
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		ordered = append(ordered, byID[id])
		for _, dependent := range dependents[id] {
			indegree[dependent]--
			if indegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}
	if len(ordered) != len(steps) {
		return nil, errors.New("composition dependency graph cannot be ordered")
	}
	return ordered, nil
}

func applyCompositionBindings(step ExecutionStepV1, results map[string]TypedStepResultV1) (ExecutionStepV1, error) {
	for _, binding := range step.InputBindings {
		source, exists := results[binding.SourceStepID]
		if !exists || (source.Status != "completed" && source.Status != "no_results") {
			return step, fmt.Errorf("binding source %q is unavailable", binding.SourceStepID)
		}
		field, exists := source.Fields[binding.SourceField]
		if !exists || field.Type != binding.ValueType {
			return step, fmt.Errorf("binding source field %q does not match declared type %q", binding.SourceField, binding.ValueType)
		}
		switch binding.TargetParameter {
		case "target":
			value, ok := field.Value.(string)
			if !ok {
				return step, errors.New("target binding requires an exact string")
			}
			step.Parameters.Target = value
		case "targets":
			value, ok := field.Value.([]string)
			if !ok {
				return step, errors.New("targets binding requires an exact string list")
			}
			step.Parameters.Targets = append([]string(nil), value...)
		case "date_from":
			value, ok := field.Value.(string)
			if !ok {
				return step, errors.New("date_from binding requires an exact timestamp string")
			}
			step.Parameters.DateFrom = value
		case "date_to":
			value, ok := field.Value.(string)
			if !ok {
				return step, errors.New("date_to binding requires an exact timestamp string")
			}
			step.Parameters.DateTo = value
		default:
			return step, fmt.Errorf("unsupported binding target %q", binding.TargetParameter)
		}
	}
	return step, nil
}

func compositionClaims(results []TypedStepResultV1) []CompositionClaimLineageV1 {
	completed, capabilities, citations := []string{}, []string{}, []ExecutionCitationV1{}
	for _, result := range results {
		if result.Status != "completed" {
			continue
		}
		completed = append(completed, result.StepID)
		capabilities = append(capabilities, result.CapabilityID)
		citations = append(citations, result.Citations...)
	}
	if len(completed) < 2 {
		return []CompositionClaimLineageV1{}
	}
	return []CompositionClaimLineageV1{{ClaimID: "claim-1", SemanticState: "candidate_correlation", StepIDs: completed, CapabilityIDs: capabilities, Citations: citations}}
}

func typedCompositionStepFromRecords(step ExecutionStepV1, template string, records map[string]any) TypedStepResultV1 {
	rowCount := int64(canonicalAnswerRowCount(template, records))
	status := "completed"
	fields := map[string]TypedIntermediateFieldV1{"row_count": {Type: "integer", Value: rowCount}}
	if target := strings.TrimSpace(step.Parameters.Target); target != "" {
		fields["exact_target"] = TypedIntermediateFieldV1{Type: "string", Value: target}
	}
	if rowCount == 0 {
		status = "no_results"
		fields = map[string]TypedIntermediateFieldV1{}
	}
	return TypedStepResultV1{
		StepID: step.StepID, CapabilityID: step.CapabilityID, Status: status,
		ResultContract: step.ExpectedResultContract, Fields: fields,
		Citations: collectCompositionCitations(records, 50), Limitations: []string{},
	}
}

func typedCompositionStepFromEvidence(step ExecutionStepV1, evidence map[string]any) TypedStepResultV1 {
	results := evidenceResults(evidence)
	if len(results) == 0 {
		return TypedStepResultV1{StepID: step.StepID, CapabilityID: step.CapabilityID, Status: "no_results", ResultContract: step.ExpectedResultContract, Fields: map[string]TypedIntermediateFieldV1{}, Citations: []ExecutionCitationV1{}, Limitations: []string{}}
	}
	return TypedStepResultV1{
		StepID: step.StepID, CapabilityID: step.CapabilityID, Status: "completed", ResultContract: step.ExpectedResultContract,
		Fields:    map[string]TypedIntermediateFieldV1{"row_count": {Type: "integer", Value: int64(len(results))}},
		Citations: collectCompositionCitations(evidence, 50), Limitations: []string{"Document passages are retrieved narrative context, not structured facts."},
	}
}

func failedCompositionStep(step ExecutionStepV1, code, limitation string) TypedStepResultV1 {
	return TypedStepResultV1{
		StepID: step.StepID, CapabilityID: step.CapabilityID, Status: "failed",
		ResultContract: step.ExpectedResultContract, Fields: map[string]TypedIntermediateFieldV1{},
		Citations: []ExecutionCitationV1{}, FailureCode: code, Limitations: []string{limitation},
	}
}

func collectCompositionCitations(value any, limit int) []ExecutionCitationV1 {
	result := []ExecutionCitationV1{}
	seen := map[string]bool{}
	var walk func(any)
	walk = func(current any) {
		if len(result) >= limit || current == nil {
			return
		}
		switch typed := current.(type) {
		case map[string]any:
			citation := ExecutionCitationV1{
				EvidenceID: stringValueAny(typed["evidence_id"]), VersionID: stringValueAny(typed["version_id"]),
				SourceFile: stringValueAny(typed["source_file"]), SourceLocator: stringValueAny(typed["source_locator"]),
			}
			if citation.SourceLocator == "" {
				citation.SourceLocator = stringValueAny(typed["locator"])
			}
			citation.SourceRow = int64ValueAny(typed["source_row"])
			if citation.SourceRow == 0 {
				citation.SourceRow = int64ValueAny(typed["source_row_number"])
			}
			if citation.SourceRow == 0 {
				citation.SourceRow = int64ValueAny(typed["row_number"])
			}
			if citation.EvidenceID != "" || citation.VersionID != "" || citation.SourceFile != "" || citation.SourceLocator != "" || citation.SourceRow > 0 {
				key := fmt.Sprintf("%s|%s|%s|%d|%s", citation.EvidenceID, citation.VersionID, citation.SourceFile, citation.SourceRow, citation.SourceLocator)
				if !seen[key] {
					seen[key] = true
					result = append(result, citation)
				}
			}
			for _, nested := range typed {
				walk(nested)
			}
		case []map[string]any:
			for _, nested := range typed {
				walk(nested)
			}
		case []any:
			for _, nested := range typed {
				walk(nested)
			}
		}
	}
	walk(value)
	return result
}

func int64ValueAny(value any) int64 {
	switch typed := value.(type) {
	case int:
		return int64(typed)
	case int32:
		return int64(typed)
	case int64:
		return typed
	case float64:
		return int64(typed)
	default:
		return 0
	}
}
