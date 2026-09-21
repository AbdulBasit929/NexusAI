package main

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/mudler/LocalAI/pkg/forensictext"
)

const (
	queryUnderstandingContractV1 = "forensics.query-understanding/v1"
	executionPlanContractV1      = "forensics.execution-plan/v1"
)

type QueryValueOrigin string

const (
	QueryOriginCurrentQuestion        QueryValueOrigin = "current_question"
	QueryOriginDeterministicExtractor QueryValueOrigin = "deterministic_extractor"
	QueryOriginValidatedModelProposal QueryValueOrigin = "validated_model_proposal"
	QueryOriginExplicitScope          QueryValueOrigin = "explicit_scope"
	QueryOriginConversationContext    QueryValueOrigin = "conversation_context"
)

type QueryIntentClass string

const (
	QueryIntentLookup        QueryIntentClass = "lookup"
	QueryIntentRank          QueryIntentClass = "rank"
	QueryIntentAggregate     QueryIntentClass = "aggregate"
	QueryIntentTimeline      QueryIntentClass = "timeline"
	QueryIntentCompare       QueryIntentClass = "compare"
	QueryIntentCorrelate     QueryIntentClass = "correlate"
	QueryIntentSummarize     QueryIntentClass = "summarize"
	QueryIntentRetrieve      QueryIntentClass = "retrieve"
	QueryIntentExtract       QueryIntentClass = "extract"
	QueryIntentSearch        QueryIntentClass = "search"
	QueryIntentInspect       QueryIntentClass = "inspect"
	QueryIntentValidate      QueryIntentClass = "validate"
	QueryIntentReadiness     QueryIntentClass = "readiness"
	QueryIntentRelationship  QueryIntentClass = "relationship"
	QueryIntentMediaAnalysis QueryIntentClass = "media_analysis"
)

type QueryLanguageV1 struct {
	Tag        string           `json:"tag"`
	Source     QueryValueOrigin `json:"source"`
	Confidence float64          `json:"confidence"`
}

type QueryFamilySetV1 struct {
	Kind      string   `json:"kind"`
	Primary   string   `json:"primary,omitempty"`
	Secondary []string `json:"secondary"`
}

type QueryEntityV2 struct {
	Type       string           `json:"type"`
	Original   string           `json:"original"`
	Normalized string           `json:"normalized"`
	Origin     QueryValueOrigin `json:"origin"`
	Normalizer string           `json:"normalizer"`
}

type QueryTargetV1 struct {
	EntityIndex int              `json:"entity_index"`
	Parameter   string           `json:"parameter"`
	Value       string           `json:"value"`
	Origin      QueryValueOrigin `json:"origin"`
}

type QueryTimeScopeV1 struct {
	From     string           `json:"from,omitempty"`
	To       string           `json:"to,omitempty"`
	Timezone string           `json:"timezone"`
	Origin   QueryValueOrigin `json:"origin"`
}

type QueryFilterV2 struct {
	Field  string           `json:"field"`
	Op     string           `json:"op"`
	Value  string           `json:"value,omitempty"`
	Origin QueryValueOrigin `json:"origin"`
}

type RequestedOutputV1 struct {
	ResultKind       string `json:"result_kind"`
	PresentationType string `json:"presentation_type"`
	Limit            int    `json:"limit"`
}

type QueryClarificationStateV1 struct {
	Required      bool     `json:"required"`
	ReasonCode    string   `json:"reason_code,omitempty"`
	Question      string   `json:"question,omitempty"`
	MissingFields []string `json:"missing_fields"`
}

type QueryContextProvenanceV1 struct {
	Used            bool                       `json:"used"`
	ConversationID  string                     `json:"conversation_id,omitempty"`
	SourceAnalysis  string                     `json:"source_analysis,omitempty"`
	SourceTurn      string                     `json:"source_turn,omitempty"`
	Fields          []string                   `json:"fields"`
	InheritedFields []FollowUpInheritedFieldV1 `json:"inherited_fields"`
}

type QueryConfidenceV1 struct {
	Overall   float64 `json:"overall"`
	Routing   float64 `json:"routing"`
	Entities  float64 `json:"entities"`
	TimeScope float64 `json:"time_scope"`
}

type QueryUnderstandingV1 struct {
	ContractVersion       string            `json:"contract_version"`
	OriginalQuestion      string            `json:"original_question"`
	NormalizedQuestion    string            `json:"normalized_question"`
	Language              QueryLanguageV1   `json:"language"`
	Intent                QueryIntentClass  `json:"intent"`
	Families              QueryFamilySetV1  `json:"families"`
	Entities              []QueryEntityV2   `json:"entities"`
	Targets               []QueryTargetV1   `json:"targets"`
	TimeScope             QueryTimeScopeV1  `json:"time_scope"`
	Filters               []QueryFilterV2   `json:"filters"`
	RequestedOutput       RequestedOutputV1 `json:"requested_output"`
	CandidateCapabilities []string          `json:"candidate_capabilities"`
	// SemanticCapabilityIDs use the thin C planner vocabulary. CandidateCapabilities
	// remains the executable-operation vocabulary for backward-compatible execution.
	SemanticCapabilityIDs []string                  `json:"semantic_capability_ids"`
	SemanticOperationRefs []string                  `json:"semantic_operation_refs"`
	SemanticReferenceKind string                    `json:"semantic_reference_kind,omitempty"`
	RequestedSemantic     string                    `json:"requested_semantic,omitempty"`
	LiteralSpan           string                    `json:"literal_span,omitempty"`
	ScopeIntent           string                    `json:"scope_intent,omitempty"`
	Clarification         QueryClarificationStateV1 `json:"clarification"`
	Context               QueryContextProvenanceV1  `json:"context"`
	Confidence            QueryConfidenceV1         `json:"confidence"`
}

type ExecutionWorkspaceScopeV1 struct {
	TenantID     string `json:"tenant_id"`
	CaseID       string `json:"case_id"`
	CollectionID string `json:"collection_id"`
	SubjectID    string `json:"subject_id"`
}

type ExecutionEvidenceScopeV1 struct {
	CollectionID            string   `json:"collection_id"`
	EvidenceID              string   `json:"evidence_id,omitempty"`
	EvidenceVersionID       string   `json:"evidence_version_id,omitempty"`
	SourceFamily            string   `json:"source_family,omitempty"`
	AvailableResultFamilies []string `json:"available_result_families,omitempty"`
	SourceFile              string   `json:"source_file,omitempty"`
	SourceFiles             []string `json:"source_files,omitempty"`
	BatchID                 string   `json:"batch_id,omitempty"`
}

type ExecutionParametersV1 struct {
	Target            string                   `json:"target,omitempty"`
	Targets           []string                 `json:"targets"`
	EvidenceID        string                   `json:"evidence_id,omitempty"`
	EvidenceVersionID string                   `json:"evidence_version_id,omitempty"`
	TextQuery         *forensictext.Query      `json:"text_query,omitempty"`
	Group             *CanonicalGroupV1        `json:"group,omitempty"`
	Compare           *CanonicalCompareV1      `json:"compare,omitempty"`
	Projection        []string                 `json:"projection,omitempty"`
	SourceNative      *SourceNativePlanV1      `json:"source_native,omitempty"`
	TranscriptMode    string                   `json:"transcript_mode,omitempty"`
	ExactTerm         string                   `json:"exact_term,omitempty"`
	QueryLanguage     string                   `json:"query_language,omitempty"`
	Plate             string                   `json:"plate,omitempty"`
	StartSeconds      *float64                 `json:"start_seconds,omitempty"`
	EndSeconds        *float64                 `json:"end_seconds,omitempty"`
	RecordType        string                   `json:"record_type,omitempty"`
	DateFrom          string                   `json:"date_from,omitempty"`
	DateTo            string                   `json:"date_to,omitempty"`
	Direction         string                   `json:"direction,omitempty"`
	SourceFile        string                   `json:"source_file,omitempty"`
	SourceSet         *StructuredSourceSetV1   `json:"source_set,omitempty"`
	BatchID           string                   `json:"batch_id,omitempty"`
	RawPayloadFilters []CanonicalPayloadFilter `json:"raw_payload_filters"`
	FieldFilters      []CanonicalFieldFilter   `json:"field_filters"`
	FieldExists       []string                 `json:"field_exists"`
	FieldNotExists    []string                 `json:"field_not_exists"`
	Limit             int                      `json:"limit"`
	Offset            int                      `json:"offset"`
	SortBy            string                   `json:"sort_by,omitempty"`
	SortDirection     string                   `json:"sort_direction,omitempty"`
	MaxKBResults      int                      `json:"max_kb_results"`
	RetrievalMode     string                   `json:"retrieval_mode,omitempty"`
}

// ExecutionInputBindingV1 permits only declared typed fields from a completed
// dependency to populate a downstream parameter. It never carries SQL, URLs,
// tool names, or an unvalidated JSON expression.
type ExecutionInputBindingV1 struct {
	SourceStepID    string `json:"source_step_id"`
	SourceField     string `json:"source_field"`
	TargetParameter string `json:"target_parameter"`
	ValueType       string `json:"value_type"`
}

type ExecutionStepV1 struct {
	StepID                 string                    `json:"step_id"`
	CapabilityID           string                    `json:"capability_id"`
	OperationID            string                    `json:"operation_id"`
	Dependencies           []string                  `json:"dependencies"`
	InputBindings          []ExecutionInputBindingV1 `json:"input_bindings"`
	Parameters             ExecutionParametersV1     `json:"parameters"`
	EvidenceScope          ExecutionEvidenceScopeV1  `json:"evidence_scope"`
	ExpectedResultContract string                    `json:"expected_result_contract"`
	ResultKind             string                    `json:"result_kind"`
	PresentationType       string                    `json:"presentation_type"`
	CitationRequired       bool                      `json:"citation_required"`
}

type ExecutionResourcePolicyV1 struct {
	ReadOnly                bool `json:"read_only"`
	MaximumSteps            int  `json:"maximum_steps"`
	MaximumRows             int  `json:"maximum_rows"`
	MaximumOffset           int  `json:"maximum_offset"`
	MaximumKBResults        int  `json:"maximum_kb_results"`
	MaximumTargets          int  `json:"maximum_targets"`
	MaximumIntermediateRows int  `json:"maximum_intermediate_rows"`
	MaximumRetrievalChunks  int  `json:"maximum_retrieval_chunks"`
	MaximumModelCalls       int  `json:"maximum_model_calls"`
	TimeoutMilliseconds     int  `json:"timeout_milliseconds"`
}

type GovernedExecutionPlanV1 struct {
	ContractVersion string                    `json:"contract_version"`
	PlanID          string                    `json:"plan_id"`
	Mode            string                    `json:"mode"`
	Workspace       ExecutionWorkspaceScopeV1 `json:"workspace"`
	Steps           []ExecutionStepV1         `json:"steps"`
	OutputContract  string                    `json:"output_contract"`
	ResourcePolicy  ExecutionResourcePolicyV1 `json:"resource_policy"`
	PolicyDecisions []string                  `json:"policy_decisions"`
}

func adaptLegacyRuntimePlan(req hybridQueryRequest, planner runtimePlan, template queryTemplateCatalogEntry) QueryUnderstandingV1 {
	origin := QueryOriginDeterministicExtractor
	if planner.Source == "explicit_template" || req.Target != "" && !strings.Contains(planner.Source, "conversation_context") {
		origin = QueryOriginCurrentQuestion
	}
	if strings.Contains(planner.Source, "conversation_context") {
		origin = QueryOriginConversationContext
	}
	entityValues := canonicalTargetSet(req.Target, req.Targets)
	if template.Name == "video_anpr_grouped_timeline" {
		entityValues = canonicalTargetSet(req.EvidenceID, []string{req.Plate})
	}
	entities := make([]QueryEntityV2, 0, len(entityValues))
	targets := make([]QueryTargetV1, 0, len(entityValues))
	for _, value := range entityValues {
		entityType, parameter := classifyQueryEntity(value, template.Name)
		entities = append(entities, QueryEntityV2{Type: entityType, Original: value, Normalized: value, Origin: origin, Normalizer: "legacy_family_normalizer"})
		targets = append(targets, QueryTargetV1{EntityIndex: len(entities) - 1, Parameter: parameter, Value: value, Origin: origin})
	}
	families := queryUnderstandingFamilies(template)
	filters := []QueryFilterV2{}
	if req.TextQuery != nil {
		filters = append(filters, QueryFilterV2{Field: "derived_text", Op: string(req.TextQuery.MatchSemantic), Value: req.TextQuery.LiteralText, Origin: origin})
	}
	if req.Direction != "" {
		filters = append(filters, QueryFilterV2{Field: "direction", Op: "eq", Value: req.Direction, Origin: origin})
	}
	if template.Name == "video_anpr_grouped_timeline" {
		if req.Plate != "" {
			filters = append(filters, QueryFilterV2{Field: "plate", Op: "eq", Value: req.Plate, Origin: origin})
		}
		if req.StartSeconds != nil {
			filters = append(filters, QueryFilterV2{Field: "start_seconds", Op: "gte", Value: fmt.Sprint(*req.StartSeconds), Origin: origin})
		}
		if req.EndSeconds != nil {
			filters = append(filters, QueryFilterV2{Field: "end_seconds", Op: "lte", Value: fmt.Sprint(*req.EndSeconds), Origin: origin})
		}
	}
	missing := []string{}
	clarification := needsClarification(req.Query, template.Name, req.Target, req.Targets)
	if clarification {
		missing = append(missing, "target")
	}
	entityConfidence := 1.0
	if len(entities) == 0 {
		entityConfidence = 0
	}
	timeConfidence := 1.0
	if req.DateFrom == "" && req.DateTo == "" {
		timeConfidence = 0
	}
	return QueryUnderstandingV1{
		ContractVersion:       queryUnderstandingContractV1,
		OriginalQuestion:      req.Query,
		NormalizedQuestion:    normalizeQuestion(req.Query),
		Language:              detectQueryLanguage(req.Query),
		Intent:                durableIntentForTemplate(template),
		Families:              families,
		Entities:              entities,
		Targets:               targets,
		TimeScope:             QueryTimeScopeV1{From: req.DateFrom, To: req.DateTo, Timezone: "case_default", Origin: origin},
		Filters:               filters,
		RequestedOutput:       RequestedOutputV1{ResultKind: resultKindForTemplate(template), PresentationType: template.Presentation, Limit: req.Limit},
		CandidateCapabilities: nonNilStrings([]string{template.OperationID}),
		Clarification:         QueryClarificationStateV1{Required: clarification, ReasonCode: defaultString(map[bool]string{true: "missing_required_parameter"}[clarification], ""), Question: map[bool]string{true: clarificationQuestion(req.Query, template.Name)}[clarification], MissingFields: missing},
		Context: QueryContextProvenanceV1{
			Used: strings.Contains(planner.Source, "conversation_context"), ConversationID: req.ConversationContext.ConversationID,
			SourceAnalysis: req.ConversationContext.AnalysisID, SourceTurn: req.ConversationContext.TurnID,
			Fields: contextFieldsFromPlan(planner), InheritedFields: append([]FollowUpInheritedFieldV1(nil), req.ConversationContext.InheritedFields...),
		},
		Confidence: QueryConfidenceV1{Overall: planner.Confidence, Routing: planner.Confidence, Entities: entityConfidence, TimeScope: timeConfidence},
	}
}

func classifyQueryEntity(value, template string) (string, string) {
	if forensicUUIDPattern.MatchString(value) {
		return "evidence_id", "evidence_id"
	}
	if template == "video_anpr_grouped_timeline" && looksLikeANPRPlateTarget(value) {
		return "plate", "plate"
	}
	return classifyTargetType(value), "target"
}

func (q QueryUnderstandingV1) Validate() error {
	if q.ContractVersion != queryUnderstandingContractV1 {
		return fmt.Errorf("unsupported query understanding contract %q", q.ContractVersion)
	}
	if strings.TrimSpace(q.OriginalQuestion) == "" || strings.TrimSpace(q.NormalizedQuestion) == "" {
		return errors.New("query understanding requires original and normalized questions")
	}
	if !validIntentClass(q.Intent) {
		return fmt.Errorf("invalid query intent %q", q.Intent)
	}
	for _, capabilityID := range q.CandidateCapabilities {
		if !validCapabilityID(capabilityID) {
			return fmt.Errorf("invalid candidate capability ID %q", capabilityID)
		}
	}
	for _, target := range q.Targets {
		if target.EntityIndex < 0 || target.EntityIndex >= len(q.Entities) || strings.TrimSpace(target.Value) == "" {
			return errors.New("query target does not reference a valid typed entity")
		}
	}
	return nil
}

func (p GovernedExecutionPlanV1) ValidateShape() error {
	if p.ContractVersion != executionPlanContractV1 {
		return fmt.Errorf("unsupported execution plan contract %q", p.ContractVersion)
	}
	if p.PlanID == "" || p.Workspace.TenantID == "" || p.Workspace.CaseID == "" || p.Workspace.CollectionID == "" || p.Workspace.SubjectID == "" {
		return errors.New("execution plan requires a complete workspace scope")
	}
	if p.Workspace.CaseID != p.Workspace.CollectionID {
		return errors.New("execution plan case and collection scope must match")
	}
	if !p.ResourcePolicy.ReadOnly {
		return errors.New("execution plans must be read-only")
	}
	switch p.Mode {
	case "single_operation":
		if len(p.Steps) != 1 || p.ResourcePolicy.MaximumSteps != 1 {
			return errors.New("APF-3.3 permits exactly one read-only operation step")
		}
	case "bounded_composition":
		if len(p.Steps) < 2 || len(p.Steps) > 3 || p.ResourcePolicy.MaximumSteps < len(p.Steps) || p.ResourcePolicy.MaximumSteps > 3 {
			return errors.New("APF-3.7 permits two or three read-only operation steps")
		}
		if p.ResourcePolicy.MaximumIntermediateRows < 1 || p.ResourcePolicy.MaximumIntermediateRows > 300 ||
			p.ResourcePolicy.MaximumRetrievalChunks < 0 || p.ResourcePolicy.MaximumRetrievalChunks > 12 ||
			p.ResourcePolicy.MaximumModelCalls < 0 || p.ResourcePolicy.MaximumModelCalls > 1 ||
			p.ResourcePolicy.TimeoutMilliseconds < 1 || p.ResourcePolicy.TimeoutMilliseconds > 60000 {
			return errors.New("APF-3.7 resource policy exceeds bounded composition limits")
		}
	default:
		return fmt.Errorf("unsupported execution plan mode %q", p.Mode)
	}
	seen := map[string]bool{}
	for _, step := range p.Steps {
		if step.StepID == "" || seen[step.StepID] || !validCapabilityID(step.CapabilityID) || step.OperationID != step.CapabilityID {
			return errors.New("execution steps require unique IDs and one valid governed capability")
		}
		seen[step.StepID] = true
		if p.Mode == "single_operation" && (len(step.Dependencies) != 0 || len(step.InputBindings) != 0) {
			return errors.New("single-capability execution cannot declare dependencies or bindings")
		}
		if step.EvidenceScope.CollectionID != p.Workspace.CollectionID || step.ExpectedResultContract == "" || p.OutputContract == "" {
			return errors.New("execution step scope and result contracts are required")
		}
		if step.EvidenceScope.EvidenceID != step.Parameters.EvidenceID || step.EvidenceScope.EvidenceVersionID != step.Parameters.EvidenceVersionID {
			return errors.New("execution evidence scope must match the typed evidence parameters")
		}
		if step.EvidenceScope.EvidenceVersionID != "" && step.EvidenceScope.EvidenceID == "" {
			return errors.New("execution evidence version requires an evidence identifier")
		}
		if step.Parameters.Limit < 1 || step.Parameters.Limit > p.ResourcePolicy.MaximumRows || step.Parameters.Offset < 0 || step.Parameters.Offset > p.ResourcePolicy.MaximumOffset {
			return errors.New("execution parameters exceed the governed row budget")
		}
		if len(step.Parameters.Targets) > p.ResourcePolicy.MaximumTargets || step.Parameters.MaxKBResults > p.ResourcePolicy.MaximumKBResults {
			return errors.New("execution parameters exceed the governed target or retrieval budget")
		}
	}
	return nil
}

func normalizeQuestion(value string) string {
	return normalizeAnalystSemantics(value)
}

func detectQueryLanguage(value string) QueryLanguageV1 {
	if queryHasMixedScript(value) {
		return QueryLanguageV1{Tag: "mixed", Source: QueryOriginDeterministicExtractor, Confidence: 1}
	}
	if regexp.MustCompile(`[\x{0600}-\x{06ff}]`).MatchString(value) {
		return QueryLanguageV1{Tag: "ur", Source: QueryOriginDeterministicExtractor, Confidence: 1}
	}
	normalized := normalizeQuestion(value)
	original := strings.ToLower(value)
	if containsAny(original, []string{"dikhao", "batao", "khulasa", "maloomat", "tafseel", "kab active hua", "kab band hua", "ikhtilaf", "dobara istemal", " aur ", " ko ", " ke liye", " hai", " hua"}) {
		return QueryLanguageV1{Tag: "ur-Latn", Source: QueryOriginDeterministicExtractor, Confidence: 0.9}
	}
	_ = normalized
	return QueryLanguageV1{Tag: "en", Source: QueryOriginDeterministicExtractor, Confidence: 0.9}
}

func queryUnderstandingFamilies(template queryTemplateCatalogEntry) QueryFamilySetV1 {
	primary := template.FamilyID
	secondarySet := map[string]struct{}{}
	for _, recordType := range template.RecordTypes {
		family := capabilityFamilyForRecordType(recordType)
		if family != "" && family != primary && recordType != "all" {
			secondarySet[family] = struct{}{}
		}
	}
	secondary := make([]string, 0, len(secondarySet))
	for family := range secondarySet {
		secondary = append(secondary, family)
	}
	sort.Strings(secondary)
	kind := "single"
	if template.Route == "kb" || template.Route == "derived" {
		kind = "knowledge_only"
	} else if primary == "case_cross_family" || len(secondary) > 0 || len(template.RecordTypes) > 1 {
		kind = "multiple"
	} else if primary == "" {
		kind = "family_neutral"
	}
	return QueryFamilySetV1{Kind: kind, Primary: primary, Secondary: secondary}
}

func capabilityFamilyForRecordType(recordType string) string {
	switch normalize(recordType) {
	case "cdr":
		return "communications_cdr"
	case "ipdr":
		return "network_ipdr"
	case "anpr":
		return "anpr_vehicles"
	case "subscriber":
		return "subscriber_identity"
	case "tower_location":
		return "tower_location"
	case "transaction":
		return "financial_transactions"
	case "access_log":
		return "access_security_logs"
	case "generic":
		return "generic_tabular"
	case "document", "text":
		return "document_intelligence"
	case "image":
		return "image_intelligence"
	case "audio":
		return "audio_intelligence"
	case "video":
		return "video_intelligence"
	default:
		return ""
	}
}

func durableIntentForTemplate(template queryTemplateCatalogEntry) QueryIntentClass {
	name := template.Name
	switch {
	case template.Route == "kb" || template.Route == "derived":
		return QueryIntentRetrieve
	case strings.Contains(name, "readiness"):
		return QueryIntentReadiness
	case strings.Contains(name, "timeline") || strings.Contains(name, "temporal") || strings.Contains(name, "first_seen") || strings.Contains(name, "activity_by") || name == "night_activity":
		return QueryIntentTimeline
	case strings.Contains(name, "correlation") || strings.Contains(name, "co_travel") || strings.Contains(name, "co_presence"):
		return QueryIntentCorrelate
	case strings.Contains(name, "relationship") || strings.Contains(name, "device_links"):
		return QueryIntentRelationship
	case strings.Contains(name, "compare") || strings.Contains(name, "comparison") || strings.Contains(name, "conflict") || strings.Contains(name, "variants"):
		return QueryIntentCompare
	case strings.Contains(name, "frequent") || strings.Contains(name, "top_") || strings.Contains(name, "extremes") || strings.Contains(name, "shortest") || strings.Contains(name, "longest"):
		return QueryIntentRank
	case strings.Contains(name, "summary") || strings.Contains(name, "brief") || strings.Contains(name, "package"):
		return QueryIntentSummarize
	case strings.Contains(name, "audit") || strings.Contains(name, "quality") || strings.Contains(name, "schema") || strings.Contains(name, "source_records") || name == "canonical_records":
		return QueryIntentInspect
	case strings.Contains(name, "breakdown") || strings.Contains(name, "usage") || strings.Contains(name, "overview") || strings.Contains(name, "activity") || strings.Contains(name, "patterns"):
		return QueryIntentAggregate
	default:
		return QueryIntentLookup
	}
}

func resultKindForTemplate(template queryTemplateCatalogEntry) string {
	name := template.Name
	switch {
	case template.Route == "kb" || template.Route == "derived":
		return "document"
	case strings.Contains(name, "timeline") || strings.Contains(name, "temporal") || strings.Contains(name, "first_seen") || strings.Contains(name, "activity_by"):
		return "timeline"
	case strings.Contains(name, "location") || strings.Contains(name, "movement") || strings.Contains(name, "tower_cdr") || strings.Contains(name, "route_timing"):
		return "geo"
	case strings.Contains(name, "frequent") || strings.Contains(name, "top_") || strings.Contains(name, "extremes"):
		return "ranked"
	case strings.Contains(name, "summary") || strings.Contains(name, "overview") || strings.Contains(name, "breakdown") || strings.Contains(name, "activity"):
		return "aggregate"
	default:
		return "table"
	}
}

func contextFieldsFromPlan(planner runtimePlan) []string {
	if !strings.Contains(planner.Source, "conversation_context") {
		return []string{}
	}
	fields := []string{}
	if planner.Target != "" {
		fields = append(fields, "target")
	}
	if planner.Template != "" {
		fields = append(fields, "template")
	}
	if planner.DateFrom != "" || planner.DateTo != "" {
		fields = append(fields, "time_scope")
	}
	return fields
}

func validIntentClass(intent QueryIntentClass) bool {
	switch intent {
	case QueryIntentLookup, QueryIntentRank, QueryIntentAggregate, QueryIntentTimeline, QueryIntentCompare,
		QueryIntentCorrelate, QueryIntentSummarize, QueryIntentRetrieve, QueryIntentExtract, QueryIntentSearch,
		QueryIntentInspect, QueryIntentValidate, QueryIntentReadiness, QueryIntentRelationship, QueryIntentMediaAnalysis:
		return true
	default:
		return false
	}
}

var capabilityIDPattern = regexp.MustCompile(`^[a-z][a-z0-9_.-]{2,127}$`)

func validCapabilityID(value string) bool {
	return capabilityIDPattern.MatchString(strings.TrimSpace(value)) &&
		!strings.ContainsAny(value, "/\\:;\n\r\t ")
}
