package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const followUpContextContractV1 = "forensics.follow-up-context/v1"
const contextMutationContractV1 = "forensics.context-mutation/v1"

type ContextMutationOperationV1 string

const (
	ContextMutationInherit ContextMutationOperationV1 = "INHERIT"
	ContextMutationReplace ContextMutationOperationV1 = "REPLACE"
	ContextMutationRemove  ContextMutationOperationV1 = "REMOVE"
	ContextMutationClarify ContextMutationOperationV1 = "CLARIFY"
)

type ContextFieldMutationV1 struct {
	Operation ContextMutationOperationV1 `json:"operation"`
	Field     string                     `json:"field"`
	From      any                        `json:"from,omitempty"`
	To        any                        `json:"to,omitempty"`
	Reason    string                     `json:"reason"`
}

type ContextMutationV1 struct {
	ContractVersion string                   `json:"contract_version"`
	AuditID         string                   `json:"audit_id,omitempty"`
	SourceAnalysis  string                   `json:"source_analysis,omitempty"`
	Mutations       []ContextFieldMutationV1 `json:"mutations"`
	ResultState     string                   `json:"result_state"`
}

type IssuedContextHandleV1 struct {
	HandleID          string `json:"handle_id"`
	Kind              string `json:"kind"`
	Value             string `json:"value,omitempty"`
	EvidenceID        string `json:"evidence_id,omitempty"`
	EvidenceVersionID string `json:"evidence_version_id,omitempty"`
	FactID            string `json:"fact_id,omitempty"`
	CitationID        string `json:"citation_id,omitempty"`
}

type FollowUpInheritedFieldV1 struct {
	Field          string `json:"field"`
	Value          string `json:"value"`
	SourceTurn     string `json:"source_turn,omitempty"`
	SourceAnalysis string `json:"source_analysis,omitempty"`
	Reason         string `json:"reason"`
	ExpiresAt      string `json:"expires_at"`
}

type queryConversationContext struct {
	ContractVersion   string                     `json:"contract_version,omitempty"`
	ConversationID    string                     `json:"conversation_id,omitempty"`
	AnalysisID        string                     `json:"analysis_id,omitempty"`
	TurnID            string                     `json:"turn_id,omitempty"`
	TenantID          string                     `json:"tenant_id,omitempty"`
	UserID            string                     `json:"user_id,omitempty"`
	CollectionID      string                     `json:"collection_id,omitempty"`
	CurrentQuestion   string                     `json:"current_question,omitempty"`
	CreatedAt         string                     `json:"created_at,omitempty"`
	ExpiresAt         string                     `json:"expires_at,omitempty"`
	Target            string                     `json:"target,omitempty"`
	Targets           []string                   `json:"targets,omitempty"`
	Template          string                     `json:"template,omitempty"`
	DateFrom          string                     `json:"date_from,omitempty"`
	DateTo            string                     `json:"date_to,omitempty"`
	Direction         string                     `json:"direction,omitempty"`
	SourceSet         *StructuredSourceSetV1     `json:"source_set,omitempty"`
	OperationID       string                     `json:"operation_id,omitempty"`
	SourceNative      *SourceNativePlanV1        `json:"source_native,omitempty"`
	IssuedFields      []FieldDescriptorV1        `json:"issued_fields,omitempty"`
	EvidenceID        string                     `json:"evidence_id,omitempty"`
	EvidenceVersionID string                     `json:"evidence_version_id,omitempty"`
	EntityHandles     []IssuedContextHandleV1    `json:"entity_handles,omitempty"`
	ResultHandles     []IssuedContextHandleV1    `json:"result_handles,omitempty"`
	FactHandles       []IssuedContextHandleV1    `json:"fact_handles,omitempty"`
	CitationHandles   []IssuedContextHandleV1    `json:"citation_handles,omitempty"`
	AuditID           string                     `json:"audit_id,omitempty"`
	Mutation          *ContextMutationV1         `json:"mutation,omitempty"`
	InheritedFields   []FollowUpInheritedFieldV1 `json:"inherited_fields,omitempty"`
}

func (c queryConversationContext) hasContinuableState() bool {
	return strings.TrimSpace(c.Template) != "" || strings.TrimSpace(c.OperationID) != "" || c.SourceNative != nil ||
		strings.TrimSpace(c.Target) != "" || len(c.Targets) > 0 || len(c.EntityHandles) > 0 || len(c.ResultHandles) > 0 || len(c.CitationHandles) > 0
}

func (c queryConversationContext) validFor(req hybridQueryRequest, now time.Time) (bool, string) {
	if c.ContractVersion != "" && c.ContractVersion != followUpContextContractV1 {
		return false, "unsupported_contract"
	}
	if c.TenantID != "" && c.TenantID != req.TenantID || c.UserID != "" && c.UserID != req.UserID || c.CollectionID != "" && c.CollectionID != req.CollectionID {
		return false, "scope_changed"
	}
	if len(c.Targets) > maxCrossFamilyTargets || len(c.IssuedFields) > 32 || len(c.EntityHandles) > 32 || len(c.ResultHandles) > 32 || len(c.FactHandles) > 32 || len(c.CitationHandles) > 32 {
		return false, "context_bounds_exceeded"
	}
	if c.EvidenceID != "" && c.EvidenceID != req.EvidenceID || c.EvidenceVersionID != "" && c.EvidenceVersionID != req.EvidenceVersionID {
		return false, "evidence_version_changed"
	}
	for _, handles := range [][]IssuedContextHandleV1{c.EntityHandles, c.ResultHandles, c.FactHandles, c.CitationHandles} {
		for _, handle := range handles {
			if c.EvidenceID != "" && handle.EvidenceID != "" && handle.EvidenceID != c.EvidenceID || c.EvidenceVersionID != "" && handle.EvidenceVersionID != "" && handle.EvidenceVersionID != c.EvidenceVersionID {
				return false, "handle_scope_changed"
			}
		}
	}
	if c.SourceNative != nil {
		if c.OperationID != "" && c.OperationID != "DYNAMIC_TYPED_PLAN" {
			return false, "competing_operation_state"
		}
		if err := validateSourceNativePlan(c.SourceNative, c.IssuedFields); err != nil {
			return false, "invalid_dynamic_state"
		}
	} else if c.OperationID == "DYNAMIC_TYPED_PLAN" {
		return false, "missing_dynamic_state"
	}
	if c.ExpiresAt != "" {
		expires, err := time.Parse(time.RFC3339, c.ExpiresAt)
		if err != nil || !now.Before(expires) {
			return false, "expired_or_invalid"
		}
	}
	return true, ""
}

func inheritFollowUpField(context queryConversationContext, field, value, reason string) FollowUpInheritedFieldV1 {
	return FollowUpInheritedFieldV1{
		Field: field, Value: value, SourceTurn: context.TurnID, SourceAnalysis: context.AnalysisID,
		Reason: reason, ExpiresAt: context.ExpiresAt,
	}
}

func applyAuditableConversationContext(req hybridQueryRequest, planner runtimePlan, now time.Time) (hybridQueryRequest, runtimePlan) {
	if !isContextualFollowUp(req.Query) {
		return req, planner
	}
	context := req.ConversationContext
	valid, reason := context.validFor(req, now)
	if !valid {
		req.ConversationContext.Mutation = clarifyContextMutation(context, reason)
		planner.Reason = fmt.Sprintf("Conversation context was not inherited (%s); %s", reason, planner.Reason)
		return req, planner
	}
	if context.SourceNative != nil {
		var ok bool
		req, ok = applyDynamicContextMutation(req, context, now)
		if !ok {
			planner.Reason = "Dynamic follow-up requires clarification; " + planner.Reason
			return req, planner
		}
		planner = planRuntimeQuery(req)
		planner.Source = "conversation_context_dynamic_ast+" + planner.Source
		planner.Reason = "Applied and revalidated a typed mutation to the prior server-issued dynamic plan; " + planner.Reason
		return req, planner
	}
	explicitMutations := make([]ContextFieldMutationV1, 0, 4)
	normalizedFollowUp := normalizeAnalystSemantics(req.Query)
	inherited := make([]FollowUpInheritedFieldV1, 0, 6)
	if req.Target == "" && isReplacementOnlyFollowUp(req.Query) {
		explicitTargets := canonicalTargetSet("", extractTargets(req.Query))
		if len(explicitTargets) > 0 && len(explicitTargets) <= maxCrossFamilyTargets {
			req.Target, req.Targets = explicitTargets[0], explicitTargets
		}
	}
	if req.Target == "" && len(extractTargets(req.Query)) == 0 {
		contextTargets := canonicalTargetSet(context.Target, context.Targets)
		if strings.Contains(normalizedFollowUp, "the other number") {
			others := make([]string, 0, len(context.EntityHandles))
			for _, handle := range context.EntityHandles {
				if handle.Kind == "entity" && handle.Value != "" && handle.Value != context.Target {
					others = append(others, handle.Value)
				}
			}
			others = uniqueStrings(others)
			if len(others) != 1 {
				req.ConversationContext.Mutation = clarifyContextMutation(context, "the other number did not resolve to exactly one issued entity handle")
				return req, planner
			}
			contextTargets = canonicalTargetSet(context.Target, []string{others[0]})
			explicitMutations = append(explicitMutations, ContextFieldMutationV1{Operation: ContextMutationReplace, Field: "targets", From: context.Targets, To: contextTargets, Reason: "resolved exactly one other server-issued entity"})
		}
		if len(contextTargets) > 0 && len(contextTargets) <= maxCrossFamilyTargets {
			req.Target, req.Targets = contextTargets[0], contextTargets
			inherited = append(inherited, inheritFollowUpField(context, "target", strings.Join(contextTargets, ","), "explicit follow-up omitted the prior authorized target"))
		}
	}
	if req.Template == "" && planner.Template == "" && (isModifierOnlyFollowUp(req.Query) || isReplacementOnlyFollowUp(req.Query)) {
		candidate := canonicalTemplate(context.Template)
		if supportedTemplate(candidate) {
			req.Template = candidate
			inherited = append(inherited, inheritFollowUpField(context, "template", candidate, "field-only follow-up retained the prior operation"))
		}
	}
	operationContinuity := planner.Template == "" || canonicalTemplate(planner.Template) == canonicalTemplate(context.Template)
	removeDates := strings.Contains(normalizedFollowUp, "don't limit it to last week") || strings.Contains(normalizedFollowUp, "do not limit it to last week")
	if operationContinuity && strings.Contains(normalizedFollowUp, "last month") {
		from, to := previousCalendarMonth(now)
		req.DateFrom, req.DateTo = from, to
		explicitMutations = append(explicitMutations, ContextFieldMutationV1{Operation: ContextMutationReplace, Field: "date_range", From: []string{context.DateFrom, context.DateTo}, To: []string{from, to}, Reason: "explicit previous calendar month"})
	} else if removeDates {
		req.DateFrom, req.DateTo = "", ""
		explicitMutations = append(explicitMutations, ContextFieldMutationV1{Operation: ContextMutationRemove, Field: "date_range", From: []string{context.DateFrom, context.DateTo}, Reason: "explicitly removed optional date restriction"})
	} else if operationContinuity && req.DateFrom == "" && planner.DateFrom == "" && strings.TrimSpace(context.DateFrom) != "" {
		req.DateFrom = strings.TrimSpace(context.DateFrom)
		inherited = append(inherited, inheritFollowUpField(context, "date_from", req.DateFrom, "follow-up omitted the prior lower time bound"))
	}
	if !removeDates && !strings.Contains(normalizedFollowUp, "last month") && operationContinuity && req.DateTo == "" && planner.DateTo == "" && strings.TrimSpace(context.DateTo) != "" {
		req.DateTo = strings.TrimSpace(context.DateTo)
		inherited = append(inherited, inheritFollowUpField(context, "date_to", req.DateTo, "follow-up omitted the prior upper time bound"))
	}
	removeDirection := strings.Contains(normalizedFollowUp, "all directions")
	if removeDirection {
		req.Direction = ""
		explicitMutations = append(explicitMutations, ContextFieldMutationV1{Operation: ContextMutationRemove, Field: "direction", From: context.Direction, Reason: "explicitly removed optional direction"})
	} else if operationContinuity && req.Direction == "" && canonicalEventDirection(context.Direction) != "" {
		req.Direction = canonicalEventDirection(context.Direction)
		inherited = append(inherited, inheritFollowUpField(context, "direction", req.Direction, "follow-up omitted the prior event direction"))
	} else if operationContinuity && req.Direction != "" && req.Direction != canonicalEventDirection(context.Direction) {
		explicitMutations = append(explicitMutations, ContextFieldMutationV1{Operation: ContextMutationReplace, Field: "direction", From: context.Direction, To: req.Direction, Reason: "explicit direction replacement"})
	}
	if operationContinuity && req.SourceSet == nil && context.SourceSet != nil && isSourceSetFollowUp(req.Query) {
		candidate := cloneStructuredSourceSet(context.SourceSet)
		if candidate != nil && validateStructuredSourceSet(*candidate) == nil {
			req.SourceSet = candidate
			inherited = append(inherited, inheritFollowUpField(context, "source_set", strings.Join(structuredSourceIDs(candidate), ","), "comparison follow-up retained the prior exact authorized source set"))
		}
	}
	if strings.Contains(normalizedFollowUp, "show source rows") || strings.Contains(normalizedFollowUp, "where did that appear") {
		handle, resolved := resolveSourceHandle(context, strings.Contains(normalizedFollowUp, "where did that appear"))
		if !resolved {
			req.ConversationContext.Mutation = clarifyContextMutation(context, "the source reference did not resolve to exactly one issued evidence/version scope")
			return req, planner
		}
		req.Template = "source_records"
		if handle.EvidenceID != "" {
			req.EvidenceID, req.EvidenceVersionID = handle.EvidenceID, handle.EvidenceVersionID
		}
		if handle.Kind == "result" {
			req.SourceFile = handle.Value
		}
		explicitMutations = append(explicitMutations, ContextFieldMutationV1{Operation: ContextMutationReplace, Field: "operation", From: context.Template, To: "source_records", Reason: "resolved source view from a server-issued handle"})
	}
	if len(inherited) == 0 && len(explicitMutations) == 0 {
		return req, planner
	}
	req.ConversationContext.InheritedFields = inherited
	req.ConversationContext.Mutation = mutationFromInherited(context, inherited)
	req.ConversationContext.Mutation.Mutations = append(req.ConversationContext.Mutation.Mutations, explicitMutations...)
	planner = planRuntimeQuery(req)
	planner.Source = "conversation_context+" + planner.Source
	planner.Reason = "Resolved bounded fields from the previous authorized answer; " + planner.Reason
	if planner.QueryPlan.AppliedFilters == nil {
		planner.QueryPlan.AppliedFilters = map[string]any{}
	}
	planner.QueryPlan.AppliedFilters["conversation_context"] = true
	planner.QueryPlan.AppliedFilters["inherited_fields"] = inherited
	return req, planner
}

func clarifyContextMutation(context queryConversationContext, reason string) *ContextMutationV1 {
	return &ContextMutationV1{ContractVersion: contextMutationContractV1, AuditID: context.AuditID, SourceAnalysis: context.AnalysisID,
		Mutations: []ContextFieldMutationV1{{Operation: ContextMutationClarify, Field: "context", Reason: reason}}, ResultState: "CLARIFICATION_REQUIRED"}
}

func mutationFromInherited(context queryConversationContext, inherited []FollowUpInheritedFieldV1) *ContextMutationV1 {
	items := make([]ContextFieldMutationV1, 0, len(inherited))
	for _, field := range inherited {
		items = append(items, ContextFieldMutationV1{Operation: ContextMutationInherit, Field: field.Field, To: field.Value, Reason: field.Reason})
	}
	return &ContextMutationV1{ContractVersion: contextMutationContractV1, AuditID: context.AuditID, SourceAnalysis: context.AnalysisID, Mutations: items, ResultState: "APPLIED"}
}

var followUpTopLimit = regexp.MustCompile(`(?i)\b(?:top|only\s+top)\s+(\d{1,3}|one|two|three|four|five|ten)\b`)
var followUpOnlyValue = regexp.MustCompile(`(?i)^\s*only\s+([\pL\pN][\pL\pN _.-]{0,63})[.?!]?\s*$`)

func cloneSourceNativePlan(plan *SourceNativePlanV1) *SourceNativePlanV1 {
	if plan == nil {
		return nil
	}
	copyPlan := *plan
	copyPlan.Project = append([]string(nil), plan.Project...)
	copyPlan.Filters = append([]SourceNativeFilterV1(nil), plan.Filters...)
	copyPlan.GroupFields = append([]string(nil), plan.GroupFields...)
	copyPlan.Measures = append([]SourceNativeMeasureV1(nil), plan.Measures...)
	copyPlan.Having = append([]SourceNativeHavingV1(nil), plan.Having...)
	copyPlan.Sort = append([]SourceNativeSortV1(nil), plan.Sort...)
	if plan.TimeBucket != nil {
		bucket := *plan.TimeBucket
		copyPlan.TimeBucket = &bucket
	}
	return &copyPlan
}

func contextField(fields []FieldDescriptorV1, id string) (FieldDescriptorV1, bool) {
	for _, field := range fields {
		if field.FieldID == id {
			return field, true
		}
	}
	return FieldDescriptorV1{}, false
}

func contextFieldByName(fields []FieldDescriptorV1, name string) (FieldDescriptorV1, bool) {
	want := normalizeSourceNativeName(name)
	for _, field := range fields {
		if field.NormalizedName == want || normalizeSourceNativeName(field.SourceName) == want {
			return field, true
		}
	}
	return FieldDescriptorV1{}, false
}

func applyDynamicContextMutation(req hybridQueryRequest, context queryConversationContext, now time.Time) (hybridQueryRequest, bool) {
	plan := cloneSourceNativePlan(context.SourceNative)
	mutations := []ContextFieldMutationV1{}
	add := func(op ContextMutationOperationV1, field string, from, to any, reason string) {
		mutations = append(mutations, ContextFieldMutationV1{Operation: op, Field: field, From: from, To: to, Reason: reason})
	}
	normalized := normalizeAnalystSemantics(req.Query)

	if match := followUpTopLimit.FindStringSubmatch(req.Query); len(match) == 2 {
		limit, err := strconv.Atoi(match[1])
		if err != nil {
			limit = map[string]int{"one": 1, "two": 2, "three": 3, "four": 4, "five": 5, "ten": 10}[strings.ToLower(match[1])]
		}
		old := plan.Limit
		plan.Limit = limit
		add(ContextMutationReplace, "source_native.limit", old, limit, "explicit bounded result limit")
	}
	if strings.Contains(normalized, "now by branch") || strings.Contains(normalized, "group by branch") {
		field, found := contextFieldByName(context.IssuedFields, "branch")
		if !found || !field.Groupable {
			req.ConversationContext.Mutation = clarifyContextMutation(context, "branch is not one issued groupable field")
			return req, false
		}
		old := append([]string(nil), plan.GroupFields...)
		plan.GroupFields = []string{field.FieldID}
		add(ContextMutationReplace, "source_native.group_fields", old, plan.GroupFields, "explicit grouping replacement using an issued field")
	}
	if strings.Contains(normalized, "totals instead") || strings.Contains(normalized, "sum instead") {
		changed := false
		for index := range plan.Measures {
			if strings.EqualFold(plan.Measures[index].Op, "AVG") {
				field, found := contextField(context.IssuedFields, plan.Measures[index].FieldID)
				if !found || !containsString(field.AllowedAggregates, "SUM") {
					req.ConversationContext.Mutation = clarifyContextMutation(context, "SUM is not allowed for the issued measure field")
					return req, false
				}
				plan.Measures[index].Op = "SUM"
				changed = true
			}
		}
		if !changed {
			req.ConversationContext.Mutation = clarifyContextMutation(context, "the issued plan has no AVG measure to replace")
			return req, false
		}
		add(ContextMutationReplace, "source_native.measures", "AVG", "SUM", "explicit issued aggregate replacement")
	}
	if match := followUpOnlyValue.FindStringSubmatch(req.Query); len(match) == 2 && !strings.HasPrefix(strings.ToLower(match[1]), "top ") && !containsString([]string{"incoming", "outgoing", "all directions"}, strings.ToLower(match[1])) {
		if len(plan.GroupFields) != 1 {
			req.ConversationContext.Mutation = clarifyContextMutation(context, "the filter value does not map to exactly one issued grouping field")
			return req, false
		}
		field, found := contextField(context.IssuedFields, plan.GroupFields[0])
		if !found || !containsString(field.AllowedFilters, "EQ") {
			req.ConversationContext.Mutation = clarifyContextMutation(context, "the issued grouping field is not filterable")
			return req, false
		}
		value := strings.TrimSpace(match[1])
		filters := make([]SourceNativeFilterV1, 0, len(plan.Filters)+1)
		for _, existing := range plan.Filters {
			if existing.FieldID != field.FieldID {
				filters = append(filters, existing)
			}
		}
		plan.Filters = append(filters, SourceNativeFilterV1{FieldID: field.FieldID, Op: "EQ", Value: value, Values: []string{}})
		add(ContextMutationReplace, "source_native.filters", nil, value, "explicit filter on the single issued grouping field")
	}
	if strings.Contains(normalized, "last month") {
		from, to := previousCalendarMonth(now)
		req.DateFrom, req.DateTo = from, to
		add(ContextMutationReplace, "date_range", []string{context.DateFrom, context.DateTo}, []string{from, to}, "explicit previous calendar month")
	} else {
		if context.DateFrom != "" {
			req.DateFrom = context.DateFrom
			add(ContextMutationInherit, "date_from", nil, context.DateFrom, "follow-up omitted prior lower bound")
		}
		if context.DateTo != "" {
			req.DateTo = context.DateTo
			add(ContextMutationInherit, "date_to", nil, context.DateTo, "follow-up omitted prior upper bound")
		}
	}
	if strings.Contains(normalized, "all directions") {
		req.Direction = ""
		add(ContextMutationRemove, "direction", context.Direction, nil, "explicitly removed optional direction")
	} else if req.Direction == "" && context.Direction != "" {
		req.Direction = context.Direction
		add(ContextMutationInherit, "direction", nil, context.Direction, "follow-up omitted prior direction")
	} else if req.Direction != "" && req.Direction != context.Direction {
		add(ContextMutationReplace, "direction", context.Direction, req.Direction, "explicit direction replacement")
	}
	if strings.Contains(normalized, "don't limit it to last week") || strings.Contains(normalized, "do not limit it to last week") {
		req.DateFrom, req.DateTo = "", ""
		add(ContextMutationRemove, "date_range", []string{context.DateFrom, context.DateTo}, nil, "explicitly removed optional date restriction")
	}
	if strings.Contains(normalized, "show source rows") || strings.Contains(normalized, "where did that appear") {
		handle, resolved := resolveSourceHandle(context, strings.Contains(normalized, "where did that appear"))
		if !resolved {
			req.ConversationContext.Mutation = clarifyContextMutation(context, "the source reference did not resolve to exactly one issued evidence/version scope")
			return req, false
		}
		req.Template, req.SourceNative = "source_records", nil
		if handle.EvidenceID != "" {
			req.EvidenceID = handle.EvidenceID
			req.EvidenceVersionID = handle.EvidenceVersionID
		}
		if handle.Kind == "result" {
			req.SourceFile = handle.Value
		}
		add(ContextMutationReplace, "operation", "DYNAMIC_TYPED_PLAN", "source_records", "resolved source view from a server-issued handle")
	} else {
		req.Template, req.SourceNative = "canonical_records", plan
		if err := validateSourceNativePlan(plan, context.IssuedFields); err != nil {
			req.SourceNative = nil
			req.ConversationContext.Mutation = clarifyContextMutation(context, err.Error())
			return req, false
		}
	}
	if len(mutations) == 0 {
		req.ConversationContext.Mutation = clarifyContextMutation(context, "no typed follow-up mutation was recognized")
		return req, false
	}
	req.ConversationContext.Mutation = &ContextMutationV1{ContractVersion: contextMutationContractV1, AuditID: context.AuditID, SourceAnalysis: context.AnalysisID, Mutations: mutations, ResultState: "APPLIED"}
	return req, true
}

func resolveSourceHandle(context queryConversationContext, citationOnly bool) (IssuedContextHandleV1, bool) {
	handles := context.ResultHandles
	if citationOnly {
		handles = context.CitationHandles
	}
	if len(handles) == 0 {
		return IssuedContextHandleV1{}, false
	}
	chosen := handles[0]
	for _, handle := range handles[1:] {
		if handle.EvidenceID != chosen.EvidenceID || handle.EvidenceVersionID != chosen.EvidenceVersionID {
			return IssuedContextHandleV1{}, false
		}
	}
	return chosen, true
}

func previousCalendarMonth(now time.Time) (string, string) {
	now = now.UTC()
	current := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	previous := current.AddDate(0, -1, 0)
	return previous.Format(time.RFC3339), current.Format(time.RFC3339)
}

func buildConversationContextPayload(req hybridQueryRequest, resp hybridQueryResponse, operationID string, provenance []map[string]any) map[string]any {
	if req.SourceNative != nil {
		operationID = "DYNAMIC_TYPED_PLAN"
	}
	fields := append([]FieldDescriptorV1(nil), req.ConversationContext.IssuedFields...)
	if raw := resp.Records["field_catalog"]; raw != nil {
		if typed, ok := raw.([]FieldDescriptorV1); ok {
			fields = append([]FieldDescriptorV1(nil), typed...)
		}
	}
	if len(fields) > 32 {
		fields = fields[:32]
	}
	entities := make([]IssuedContextHandleV1, 0, len(req.Targets)+1)
	for index, value := range canonicalTargetSet(req.Target, req.Targets) {
		entities = append(entities, IssuedContextHandleV1{HandleID: fmt.Sprintf("entity-%d", index+1), Kind: "entity", Value: value, EvidenceID: req.EvidenceID, EvidenceVersionID: req.EvidenceVersionID})
	}
	citations := make([]IssuedContextHandleV1, 0, min(len(provenance), 16))
	results := make([]IssuedContextHandleV1, 0, min(len(provenance), 16))
	for index, source := range provenance {
		if index >= 16 {
			break
		}
		evidenceID := stringValueAny(source["evidence_id"])
		versionID := stringValueAny(firstPresent(source, "version_id", "evidence_version_id"))
		citation := stringValueAny(firstPresent(source, "citation", "citation_ref", "source_locator"))
		citations = append(citations, IssuedContextHandleV1{HandleID: fmt.Sprintf("citation-%d", index+1), Kind: "citation", CitationID: citation, EvidenceID: evidenceID, EvidenceVersionID: versionID})
		results = append(results, IssuedContextHandleV1{HandleID: fmt.Sprintf("result-%d", index+1), Kind: "result", Value: stringValueAny(firstPresent(source, "source_file", "artifact_id")), EvidenceID: evidenceID, EvidenceVersionID: versionID, CitationID: citation})
	}
	now := time.Now().UTC()
	auditID := req.ConversationContext.AuditID
	if auditID == "" {
		auditID = fmt.Sprintf("ctx-%d", now.UnixNano())
	}
	return map[string]any{
		"contract_version": followUpContextContractV1, "conversation_id": req.ConversationContext.ConversationID,
		"analysis_id": req.ConversationContext.AnalysisID, "turn_id": req.ConversationContext.TurnID,
		"tenant_id": req.TenantID, "user_id": req.UserID, "collection_id": req.CollectionID,
		"current_question": req.Query, "created_at": now.Format(time.RFC3339), "expires_at": now.Add(30 * time.Minute).Format(time.RFC3339),
		"target": req.Target, "targets": req.Targets, "template": resp.Template, "operation_id": operationID,
		"date_from": req.DateFrom, "date_to": req.DateTo, "direction": req.Direction, "source_set": req.SourceSet,
		"source_native": req.SourceNative, "issued_fields": fields, "evidence_id": req.EvidenceID, "evidence_version_id": req.EvidenceVersionID,
		"entity_handles": entities, "result_handles": results, "citation_handles": citations,
		"audit_id": auditID, "mutation": req.ConversationContext.Mutation,
		"inherited_fields": req.ConversationContext.InheritedFields,
	}
}

func isSourceSetFollowUp(query string) bool {
	normalized := normalizeAnalystSemantics(query)
	return containsAny(normalized, []string{
		"common contacts", "shared imei", "shared tower", "show conflicts",
		"compare both sources", "compare these sources", "same comparison", "source contribution",
	})
}

func cloneStructuredSourceSet(sourceSet *StructuredSourceSetV1) *StructuredSourceSetV1 {
	if sourceSet == nil {
		return nil
	}
	copySet := &StructuredSourceSetV1{ContractVersion: sourceSet.ContractVersion}
	copySet.Sources = append([]StructuredSourceRefV1(nil), sourceSet.Sources...)
	return copySet
}
