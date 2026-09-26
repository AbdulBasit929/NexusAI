package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mudler/LocalAI/pkg/httpclient"
)

const semanticOperationProposalV1 = "forensics.semantic-operation-proposal/v1"

const (
	semanticDecisionPrefix           = "OPERATION:"
	semanticClarifyAmbiguous         = "CLARIFY:AMBIGUOUS_INTENT"
	semanticClarifyInsufficientFacts = "CLARIFY:INSUFFICIENT_FACTS"
	semanticUnsupported              = "UNSUPPORTED:REQUEST_CLASS"
	semanticDynamicDecision          = "DYNAMIC_TYPED_PLAN"
)

// SemanticOperationCandidateV1 is a server-issued meaning descriptor. It lets
// the language model map open-ended wording to an existing operation without
// giving the model authority over scope, facts, parameters, tools, or SQL.
type SemanticOperationCandidateV1 struct {
	OperationID        string   `json:"operation_id"`
	FamilyID           string   `json:"family_id"`
	Intent             string   `json:"intent"`
	Description        string   `json:"description"`
	RequiredParameters []string `json:"required_parameters"`
	OptionalParameters []string `json:"optional_parameters"`
	Measures           []string `json:"measures"`
	GroupBy            []string `json:"group_by"`
	ResultKind         string   `json:"result_kind"`
	Presentation       string   `json:"presentation"`
	ScopeMode          string   `json:"scope_mode"`
	ExposureStatus     string   `json:"exposure_status"`
}

type SemanticOperationProposalV1 struct {
	Decision string `json:"decision"`
}

// semanticOperationPromptCandidateV1 is deliberately compact because the
// currently accepted local model has a 4096-token context. The complete
// descriptor stays in the server audit; the model receives only the meaning
// fields needed to choose an issued operation.
type semanticOperationPromptCandidateV1 struct {
	OperationID string   `json:"id"`
	FamilyID    string   `json:"family"`
	Intent      string   `json:"intent"`
	Description string   `json:"meaning"`
	Measures    []string `json:"measures,omitempty"`
	GroupBy     []string `json:"group_by,omitempty"`
	ResultKind  string   `json:"result_kind"`
	// Execution requirements remain in the server audit, not the semantic task.
}

type SemanticRetrievalScoreV1 struct {
	OperationID string  `json:"operation_id"`
	Rank        int     `json:"rank"`
	Score       float64 `json:"score"`
}

type SemanticPlannerAuditV1 struct {
	FamilyDecision         string                         `json:"family_decision,omitempty"`
	FamilyRaw              string                         `json:"family_raw,omitempty"`
	FamilyLatencyMS        int64                          `json:"family_latency_ms"`
	OperationLatencyMS     int64                          `json:"operation_latency_ms"`
	RegistryCandidateCount int                            `json:"registry_candidate_count"`
	EligibleOperationCount int                            `json:"eligible_operation_count"`
	RetrievedCandidates    []SemanticOperationCandidateV1 `json:"retrieved_candidates,omitempty"`
	RetrievalScores        []SemanticRetrievalScoreV1     `json:"retrieval_scores,omitempty"`
	TopK                   int                            `json:"top_k"`
	SelectedDecision       string                         `json:"selected_decision,omitempty"`
	DynamicSelected        bool                           `json:"dynamic_selected"`
	RequestClass           string                         `json:"request_class"`
	ScopeFilterReason      string                         `json:"scope_filter_reason"`
	ContractVersion        string                         `json:"contract_version"`
	Language               string                         `json:"language"`
	CandidateCount         int                            `json:"candidate_count"`
	Candidates             []SemanticOperationCandidateV1 `json:"candidates"`
	RawProposal            string                         `json:"raw_model_proposal"`
	ModelProposal          *SemanticOperationProposalV1   `json:"model_proposal,omitempty"`
	Selected               *SemanticOperationCandidateV1  `json:"selected_operation,omitempty"`
	FactAuthority          string                         `json:"fact_authority"`
	State                  string                         `json:"state"`
	DynamicPlan            any                            `json:"dynamic_plan,omitempty"`
	// IRFallbackOutcome records what the enum-constrained generator did when the
	// deterministic compiler could not answer: accepted, or the specific check
	// that discarded its plan. In a forensic product "why did the system answer
	// this way" must be reconstructable months later, so this is an audit
	// record, not debug output.
	IRFallbackOutcome string `json:"ir_fallback_outcome,omitempty"`
	// IRGenerationMS is how long the enum-constrained generation actually took.
	//
	// It was invisible. `llm_latency_ms` is assigned ONLY inside the bounded
	// synthesis block, so a request that spent two minutes generating a plan
	// reported `llm=0`. Measured 2026-09-24: CDR-02 reported
	// total=119,835 ms · db=988 ms · llm=0 · kb=0 -- 119 seconds attributed to
	// nothing at all, on a question whose SQL took under a second.
	//
	// This project's own hardest lesson is that the INSTRUMENT is fixed first:
	// WI-6 found three runs had been mis-measured. Latency cannot be worked on
	// until the dominant cost is on the record.
	IRGenerationMS int64 `json:"ir_generation_ms,omitempty"`
	// IRPlanCacheHit records that the plan came from a memoized completion
	// rather than a fresh model call. "Why did the system answer this way"
	// must be reconstructable months later, and "the model was not consulted
	// for this answer" is part of that account -- without it, an audit reader
	// cannot tell a 0 ms generation from a generation that never happened.
	IRPlanCacheHit bool `json:"ir_plan_cache_hit,omitempty"`
	// IRShadowOutcome/IRShadowPlan hold what the generator produced when shadow
	// mode is on. Evidence for measuring field-selection accuracy against gold
	// plans; never used to answer.
	IRShadowOutcome string `json:"ir_shadow_outcome,omitempty"`
	// IRArbitrated records that a generated plan REPLACED a clarification.
	// "Why did the system answer this way" must be reconstructable months
	// later, and an arbitrated answer has different provenance from a
	// compiled one.
	IRArbitrated bool `json:"ir_arbitrated,omitempty"`
	// VerifiedOnlyWithheld marks an answer that existed and was NOT stated,
	// because no verifiable plan backed it. Distinct from a question the
	// compiler could not resolve.
	VerifiedOnlyWithheld bool `json:"verified_only_withheld,omitempty"`
	// WithholdCode/WithholdDetail say WHICH of the five withholding causes
	// fired and what planner state explains it.
	//
	// The flag above is a bit: it records THAT an answer was held back, which
	// was enough while there was one cause, and is not enough now that there
	// are five. Measured 2026-09-24, 12 of 18 clarifications in the corpus were
	// indistinguishable from each other in the recorded response, so "which
	// coverage gap produces these" could not be answered from a completed run
	// -- only by re-running each question and reading the prose.
	WithholdCode   string `json:"withhold_code,omitempty"`
	WithholdDetail string `json:"withhold_detail,omitempty"`
	// DroppedInventedFilters names the fields whose filters were removed
	// because the analyst never supplied their values. "Why did the system
	// answer this way" must be reconstructable months later, and a plan that
	// was PRUNED before execution has different provenance from one the
	// generator produced whole.
	DroppedInventedFilters []string `json:"dropped_invented_filters,omitempty"`
	// CrossCheck records the second, independent derivation and whether it
	// agreed. "Why did the system decline" must be answerable later, and a
	// disagreement is a different event from an unresolvable question.
	CrossCheck *crossCheckOutcome `json:"cross_check,omitempty"`
	// IRShadowIssuedFields is the exact enum handed to the generator. Without
	// it the safety property -- that a field outside the authorized set is
	// structurally impossible -- cannot be checked from a recorded run at all:
	// a hashed uncurated ID like fld_b4d5a2... is a legitimate enum member, and
	// judging IDs by shape flags it as a violation it is not.
	IRShadowIssuedFields []string            `json:"ir_shadow_issued_fields,omitempty"`
	IRShadowPlan         *SourceNativePlanV1 `json:"ir_shadow_plan,omitempty"`
	// IRShadowRejectedPlan is what the generator proposed when validation threw
	// the plan away. Without it a rejection reason cannot be attributed.
	IRShadowRejectedPlan      *SourceNativePlanV1                  `json:"ir_shadow_rejected_plan,omitempty"`
	BindingState              string                               `json:"binding_state,omitempty"`
	BindingInitialState       string                               `json:"binding_initial_state,omitempty"`
	MissingFactKinds          []string                             `json:"missing_fact_kinds,omitempty"`
	FieldCatalogCount         int                                  `json:"field_catalog_count,omitempty"`
	RetrievedFields           []FieldDescriptorV1                  `json:"retrieved_fields,omitempty"`
	FieldRetrievalScores      []SourceNativeFieldScoreV1           `json:"field_retrieval_scores,omitempty"`
	SemanticFrame             *SemanticFrameV1                     `json:"semantic_frame,omitempty"`
	CapabilitySnapshot        *SemanticCapabilitySnapshotV1        `json:"capability_snapshot,omitempty"`
	EmbeddingResolution       *SemanticEmbeddingResolutionAuditV1  `json:"embedding_resolution,omitempty"`
	OperationResolutionScores []SemanticOperationResolutionScoreV1 `json:"operation_resolution_scores,omitempty"`
	ResolutionScore           float64                              `json:"resolution_score,omitempty"`
	ResolutionMargin          float64                              `json:"resolution_margin,omitempty"`
}

func semanticOperationCandidates(req hybridQueryRequest) []SemanticOperationCandidateV1 {
	candidates := make([]SemanticOperationCandidateV1, 0, len(supportedQueryTemplates()))
	for _, template := range supportedQueryTemplates() {
		if template.OperationID == "" || template.ExposureStatus == "engineering_only" {
			continue
		}
		if !semanticTemplateMatchesScope(template, req) {
			continue
		}
		required, optional := []string{}, []string{}
		for _, input := range template.Inputs {
			if input.Required {
				required = append(required, input.Name)
			} else {
				optional = append(optional, input.Name)
			}
		}
		candidates = append(candidates, SemanticOperationCandidateV1{
			OperationID: template.OperationID, FamilyID: template.FamilyID,
			Intent: string(durableIntentForTemplate(template)), Description: template.Description,
			RequiredParameters: required, OptionalParameters: optional,
			Measures: append([]string(nil), template.Measures...), GroupBy: append([]string(nil), template.GroupBy...),
			ResultKind: resultKindForTemplate(template), Presentation: template.Presentation,
			ScopeMode: template.ScopeMode, ExposureStatus: template.ExposureStatus,
		})
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].OperationID < candidates[j].OperationID })
	return candidates
}

func semanticTemplateMatchesScope(template queryTemplateCatalogEntry, req hybridQueryRequest) bool {
	selected := req.QueryScope.Kind == string(EvidenceScopeSelected) || req.EvidenceID != ""
	if !selected {
		return template.ScopeMode != "evidence_required"
	}
	family := normalize(req.QueryScope.SourceFamily)
	if family == "" {
		return true
	}
	if template.ScopeMode == "source_set_and_target_required" {
		return false
	}
	for _, recordType := range template.RecordTypes {
		if recordType == "all" || normalize(recordType) == family {
			return true
		}
		if (family == "text" || family == "document") && (recordType == "text" || recordType == "document") {
			return true
		}
	}
	return false
}

func semanticOperationSchema(candidates []SemanticOperationCandidateV1) map[string]any {
	decisions := []string{semanticDynamicDecision, semanticClarifyAmbiguous, semanticUnsupported}
	for _, candidate := range candidates {
		decisions = append(decisions, semanticDecisionPrefix+candidate.OperationID)
	}
	return map[string]any{
		"type": "object", "additionalProperties": false,
		"required":   []string{"decision"},
		"properties": map[string]any{"decision": map[string]any{"type": "string", "enum": decisions}},
	}
}

func semanticOperationPromptCandidates(candidates []SemanticOperationCandidateV1) []semanticOperationPromptCandidateV1 {
	prompt := make([]semanticOperationPromptCandidateV1, 0, len(candidates))
	for _, candidate := range candidates {
		prompt = append(prompt, semanticOperationPromptCandidateV1{
			OperationID: candidate.OperationID, FamilyID: candidate.FamilyID,
			Intent: candidate.Intent, Description: candidate.Description,
			Measures: candidate.Measures, GroupBy: candidate.GroupBy,
			ResultKind: candidate.ResultKind,
		})
	}
	return prompt
}

func decodeSemanticOperationProposal(payload []byte, candidates []SemanticOperationCandidateV1) (SemanticOperationProposalV1, error) {
	var proposal SemanticOperationProposalV1
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&proposal); err != nil {
		return proposal, err
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return proposal, errors.New("semantic operation proposal contains trailing content")
	}
	if proposal.Decision == semanticDynamicDecision || proposal.Decision == semanticClarifyAmbiguous || proposal.Decision == semanticUnsupported {
		return proposal, nil
	}
	for _, candidate := range candidates {
		if proposal.Decision == semanticDecisionPrefix+candidate.OperationID {
			return proposal, nil
		}
	}
	return proposal, errors.New("semantic operation proposal selected a non-issued operation")
}

func applySemanticOperation(req hybridQueryRequest, selected SemanticOperationCandidateV1) (hybridQueryRequest, error) {
	templateName := queryTemplateNameByOperationID(selected.OperationID)
	template, ok := queryTemplateByName(templateName)
	if !ok || template.OperationID != selected.OperationID || template.FamilyID != selected.FamilyID {
		return req, errors.New("semantic operation no longer matches the server registry")
	}
	if template.ExposureStatus == "engineering_only" || !semanticTemplateMatchesScope(template, req) {
		return req, errors.New("semantic operation is not queryable in the authorized scope")
	}
	bound, facts := extractHybridFacts(req)
	if facts.State != "" {
		return req, errors.New(facts.State)
	}
	bound.Template = template.Name
	if bound.Target == "" {
		bound.Target = facts.Target
	}
	bound.Targets = canonicalTargetSet(bound.Target, bound.Targets, facts.Targets)
	if bound.DateFrom == "" {
		bound.DateFrom = facts.DateFrom
	}
	if bound.DateTo == "" {
		bound.DateTo = facts.DateTo
	}
	if bound.Direction == "" {
		bound.Direction = facts.Direction
	}
	if facts.TopK > 0 {
		bound.Limit = facts.TopK
		bound.MaxKBResults = facts.TopK
	}
	if bound.StartSeconds == nil {
		bound.StartSeconds = facts.StartSeconds
	}
	if bound.EndSeconds == nil {
		bound.EndSeconds = facts.EndSeconds
	}
	return bound, nil
}

func semanticPlannerRequestTimeout(cfg config) time.Duration {
	if cfg.SemanticPlannerTimeout > 0 && cfg.SemanticPlannerTimeout < maximumInferenceWait {
		return cfg.SemanticPlannerTimeout
	}
	return maximumInferenceWait
}

func semanticBindingReadiness(req hybridQueryRequest, selected SemanticOperationCandidateV1) (string, []string) {
	missing := []string{}
	for _, field := range selected.RequiredParameters {
		ready := false
		switch field {
		case "target":
			ready = req.Target != "" || len(req.Targets) > 0
		case "evidence_id":
			ready = req.EvidenceID != ""
		case "source_set":
			ready = req.SourceSet != nil
		default: // Unknown execution requirements cannot silently become ready.
			ready = false
		}
		if !ready {
			missing = append(missing, field)
		}
	}
	if len(missing) > 0 {
		return "USER_FACT_REQUIRED", missing
	}
	return "READY", missing
}

const semanticOperationTask = `Interpret unrestricted English wording and choose the server-issued operation that best represents the analyst's semantic intent: evidence family, requested measure, grouping and result shape. Return only {"decision":"<allowed enum>"}.
The server alone owns scope, fact binding and execution authority.
This is intent selection, not execution or evidence inspection. The server binds and validates tenant, case, evidence, target, dates, source selection, literal text, limits and authorization AFTER selection. Missing execution values or evidence rows are not a reason to reject a recognizable operation. Never supply those values or decide authorization yourself.
Choose DYNAMIC_TYPED_PLAN when the request is analytically valid and can use the bounded typed query algebra, but none of the issued named operations fully represents it. Choose CLARIFY:AMBIGUOUS_INTENT only when the analytical meaning cannot be determined. Choose UNSUPPORTED:REQUEST_CLASS only when neither an issued operation nor the current typed algebra represents the request. Evidence availability and execution readiness are server decisions.`

func resolveSemanticOperationInCandidates(ctx context.Context, cfg config, req hybridQueryRequest, candidates []SemanticOperationCandidateV1) (hybridQueryRequest, string) {
	audit := req.SemanticPlannerAudit
	if audit == nil {
		audit = &SemanticPlannerAuditV1{ContractVersion: semanticOperationProposalV1}
	}
	audit.Language = detectQueryLanguage(req.Query).Tag
	audit.CandidateCount = len(candidates)
	audit.Candidates = candidates
	audit.FactAuthority = "SERVER_DETERMINISTIC_ONLY"
	audit.State = "unavailable"
	req.SemanticPlannerAudit = audit
	finish := func(state string) (hybridQueryRequest, string) { audit.State = state; return req, state }
	if audit.Language != "en" || len(candidates) == 0 || cfg.LocalAIURL == "" || !isForensicSynthesisModelName(req.SynthesisModel) {
		return finish("unavailable")
	}
	candidateJSON, _ := json.Marshal(semanticOperationPromptCandidates(candidates))
	body, _ := json.Marshal(map[string]any{
		"model": req.SynthesisModel, "temperature": 0, "max_tokens": 96,
		"response_format": map[string]any{"type": "json_schema", "json_schema": map[string]any{"name": "forensic_semantic_operation", "strict": true, "schema": semanticOperationSchema(candidates)}},
		"messages": []map[string]string{
			{"role": "system", "content": semanticOperationTask},
			// Keep the stable registry prefix before the varying question so a
			// compatible local runtime may reuse prompt-prefix work across a
			// bounded sequential qualification or analyst session.
			{"role": "user", "content": "Server-issued operation meanings:\n" + string(candidateJSON) + "\nAnalyst question:\n" + req.Query},
		},
	})
	// The frozen current-4B envelope measured a 131-second first full-context
	// request. Keep this path bounded without inheriting the tiny-residual cap.
	timeout := semanticPlannerRequestTimeout(cfg)
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(callCtx, http.MethodPost, strings.TrimRight(cfg.LocalAIURL, "/")+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return finish("request_build_failed")
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if cfg.LocalAIAPIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+cfg.LocalAIAPIKey)
	}
	response, err := httpclient.New().Do(httpReq)
	if err != nil {
		return finish("timeout_or_unavailable")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return finish("runtime_rejected")
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
	if err != nil || len(raw) > 1<<20 || !utf8.Valid(raw) {
		return finish("malformed_completion")
	}
	var envelope struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
	}
	if json.Unmarshal(raw, &envelope) != nil || len(envelope.Choices) != 1 {
		return finish("malformed_completion")
	}
	audit.RawProposal = envelope.Choices[0].Message.Content
	if envelope.Choices[0].FinishReason != "stop" {
		return finish("incomplete_completion")
	}
	proposal, err := decodeSemanticOperationProposal([]byte(audit.RawProposal), candidates)
	if err != nil {
		return finish("malformed_semantic_proposal")
	}
	audit.ModelProposal = &proposal
	audit.SelectedDecision = proposal.Decision
	switch proposal.Decision {
	case semanticClarifyAmbiguous:
		return finish("AMBIGUOUS_INTENT")
	case semanticUnsupported:
		return finish("UNSUPPORTED_REQUEST_CLASS")
	case semanticDynamicDecision:
		audit.DynamicSelected = true
		family := semanticDynamicFamilyForRequest(req, candidates)
		resolved, state := resolveSemanticDynamicPlan(callCtx, cfg, req, family)
		audit.State = state
		resolved.SemanticPlannerAudit = audit
		return resolved, state
	}
	operationID := strings.TrimPrefix(proposal.Decision, semanticDecisionPrefix)
	for index := range candidates {
		if candidates[index].OperationID != operationID {
			continue
		}
		// Grounding check reusing the exact same grouping-alignment signal
		// the deterministic compiler itself scores with (semanticGroupingCompatible),
		// not a new validator: a live test proved the model can pick a
		// schema-valid, correctly-shaped operation that groups by the wrong
		// dimension entirely (e.g. "top 5 phone numbers by call count"
		// resolved to forensics.top_locations, grouped by cell site/location,
		// not by phone number) and present it as a confident, non-clarifying
		// answer. Only the curated, controlled-vocabulary hints are trusted
		// for this hard rejection — frame.GroupByHints also contains raw
		// free-text captured from "by X"/"which X" phrasing (e.g. "rank
		// partners by event frequency" captures "event frequency", which
		// describes the rank/sort criterion, not a real grouping dimension —
		// rejecting on that would wrongly break a correct match).
		frame := extractSemanticFrame(req)
		curatedHints := curatedGroupHints(frame.GroupByHints)
		if len(curatedHints) > 0 && len(candidates[index].GroupBy) > 0 && !semanticGroupingCompatible(curatedHints, candidates[index].GroupBy) {
			return finish("semantic_grounding_rejected:grouping_mismatch")
		}
		audit.Selected = &candidates[index]
		resolved, err := applySemanticOperation(req, candidates[index])
		if err != nil {
			return finish("semantic_validation_rejected:" + err.Error())
		}
		audit.State = "semantic_model_plan"
		audit.BindingState, audit.MissingFactKinds = semanticBindingReadiness(resolved, candidates[index])
		audit.BindingInitialState, _ = semanticBindingReadiness(req, candidates[index])
		if audit.BindingInitialState == "USER_FACT_REQUIRED" && audit.BindingState == "READY" {
			audit.BindingInitialState = "SERVER_BINDABLE"
		}
		resolved.SemanticPlannerAudit = audit
		return resolved, "semantic_model_plan"
	}
	return finish("malformed_semantic_proposal")
}
