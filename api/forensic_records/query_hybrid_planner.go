package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/mudler/LocalAI/pkg/forensictext"
	"github.com/mudler/LocalAI/pkg/httpclient"
)

const hybridResidualVersion = "forensics.hybrid-residual/v1"

type HybridIdentifierV1 struct {
	Raw       string `json:"raw_span"`
	Canonical string `json:"canonical"`
	Type      string `json:"identifier_type"`
	Start     int    `json:"start_byte"`
	End       int    `json:"end_byte"`
}

// Facts never originate in a model response. Byte offsets address the original
// UTF-8 question, while canonical identifiers are kept separately for execution.
type HybridFactsV1 struct {
	ContractVersion    string                     `json:"contract_version"`
	TenantID           string                     `json:"tenant_id"`
	UserID             string                     `json:"user_id"`
	CollectionID       string                     `json:"collection_id"`
	EvidenceID         string                     `json:"evidence_id"`
	Scope              string                     `json:"scope"`
	Identifiers        []HybridIdentifierV1       `json:"identifiers"`
	Target             string                     `json:"target"`
	Targets            []string                   `json:"targets"`
	TextQuery          *forensictext.Query        `json:"text_query,omitempty"`
	Family             string                     `json:"family"`
	DateFrom           string                     `json:"date_from"`
	DateTo             string                     `json:"date_to"`
	Direction          string                     `json:"direction"`
	TopK               int                        `json:"top_k"`
	StartSeconds       *float64                   `json:"start_seconds"`
	EndSeconds         *float64                   `json:"end_seconds"`
	Filters            []CanonicalPayloadFilter   `json:"filters"`
	PriorCapability    string                     `json:"prior_capability"`
	PriorSemantic      string                     `json:"prior_semantic"`
	PriorTargets       []string                   `json:"prior_targets"`
	Inherited          []FollowUpInheritedFieldV1 `json:"inherited_fields"`
	Transition         string                     `json:"transition"`
	ExplicitCapability string                     `json:"explicit_capability"`
	State              string                     `json:"state"`
}

type HybridTupleID string
type HybridTupleV1 struct {
	ID             HybridTupleID `json:"tuple_id"`
	Capability     string        `json:"capability_id"`
	Semantic       string        `json:"semantic"`
	Scope          string        `json:"allowed_scope"`
	Family         string        `json:"family"`
	Operation      string        `json:"operation_ref"`
	Representation string        `json:"text_representation_rule"`
	Required       []string      `json:"required_fact_types"`
	Optional       []string      `json:"optional_fact_types"`
}

type HybridResidualV1 struct {
	ContractVersion   string        `json:"contract_version"`
	SelectedTupleID   HybridTupleID `json:"selected_tuple_id"`
	AmbiguityState    string        `json:"ambiguity_state"`
	ClarificationCode string        `json:"clarification_code"`
	Confidence        float64       `json:"confidence,omitempty"`
}

type HybridPlannerAuditV1 struct {
	Facts          HybridFactsV1             `json:"deterministic_facts"`
	Candidates     []HybridTupleV1           `json:"candidate_tuples"`
	RawResidual    string                    `json:"raw_model_residual"`
	ModelDecision  *HybridResidualV1         `json:"model_decision"`
	DecisionSource string                    `json:"decision_source"`
	Reconciliation []string                  `json:"reconciliation"`
	FinalPlan      *DynamicPlannerProposalV1 `json:"final_plan"`
	ResolvedTarget string                    `json:"resolved_target"`
	State          string                    `json:"state"`
}

func hybridIdentifierSpans(query string) []HybridIdentifierV1 {
	// Only recognized identifier grammars participate. Prefer a complete source
	// span over a nested plate-like suffix such as K29 within analyst-k29.
	locations := principalSourcePattern.FindAllStringIndex(query, -1)
	locations = append(locations, forensicUUIDInTextPattern.FindAllStringIndex(query, -1)...)
	for _, p := range targetPatterns {
		locations = append(locations, p.FindAllStringIndex(query, -1)...)
	}
	sort.Slice(locations, func(i, j int) bool {
		if locations[i][0] == locations[j][0] {
			return locations[i][1] > locations[j][1]
		}
		return locations[i][0] < locations[j][0]
	})
	out := []HybridIdentifierV1{}
	for _, loc := range locations {
		if len(out) > 0 && loc[0] < out[len(out)-1].End {
			continue
		}
		raw := query[loc[0]:loc[1]]
		canonical := normalizeExtractedTarget(raw)
		if canonical == "" {
			continue
		}
		// Existing extraction excludes source-time fragments and date tokens.
		if !containsString(extractTargets(query), canonical) {
			continue
		}
		kind := classifyTargetType(canonical)
		if principalSourcePattern.MatchString(raw) {
			kind = "principal"
		}
		out = append(out, HybridIdentifierV1{raw, canonical, kind, loc[0], loc[1]})
	}
	return out
}

func hybridExplicitCapability(query string, text *forensictext.Query, family string) string {
	q := strings.ToLower(forensictext.PlanningText(query))
	if text != nil {
		switch family {
		case "document", "text":
			return "document.phrase"
		case "image":
			return "ocr.phrase"
		case "audio":
			if hybridTextRepresentation(q, text.LiteralText, "audio") == "roman_derivative" {
				return "roman_urdu.phrase"
			}
			return "transcript.phrase"
		}
	}
	// These descriptors name supported operations; generic identifier shape
	// alone never decides between endpoint aggregation and session filtering.
	groups := []struct {
		id      string
		phrases []string
	}{
		{"ipdr.endpoint", []string{"endpoint summary", "network endpoint", "اینڈ پوائنٹ", "endpoint ka khulasa"}},
		{"ipdr.sessions", []string{"filter internet sessions", "filter ipdr sessions", "internet sessions filter", "session filtering"}},
		{"logs.failures", []string{"failed login", "failed access", "ناکام لاگ ان", "ناکام رسائی", "nakam rasai"}},
		{"subscriber.lookup", []string{"subscriber", "سبسکرائبر"}},
		{"cdr.frequent_contacts", []string{"contact ranking", "frequent contacts", "top contacts", "most frequent contacts", "sab se zyada rabtay", "زیادہ رابطے"}},
		{"cdr.time_activity", []string{"call activity", "hourly calls", "ghanta war"}},
		{"tower.lookup", []string{"tower lookup", "site lookup", "ٹاور"}},
		{"device.observations", []string{"device observations", "imei", "imsi"}},
		{"financial.summary", []string{"transaction summary", "financial summary", "لین دین کا خلاصہ"}},
		{"image.plate", []string{"plate", "نمبر پلیٹ"}},
		{"case.evidence_package", []string{"evidence package", "investigation summary"}},
	}
	ids := []string{}
	for _, g := range groups {
		if containsAny(q, g.phrases) {
			ids = append(ids, g.id)
		}
	}
	if len(ids) == 1 {
		return ids[0]
	}
	return ""
}

func hybridTextRepresentation(outside, literal, family string) string {
	if family != "audio" {
		return "raw"
	}
	for _, r := range literal {
		if unicode.In(r, unicode.Arabic) {
			return "raw"
		}
	}
	if containsAny(strings.ToLower(outside), []string{"roman urdu", "رومن اردو"}) {
		return "roman_derivative"
	}
	return "raw"
}

func extractHybridFacts(req hybridQueryRequest) (hybridQueryRequest, HybridFactsV1) {
	f := HybridFactsV1{ContractVersion: "forensics.hybrid-facts/v1", Transition: "NONE", Identifiers: hybridIdentifierSpans(req.Query), Inherited: []FollowUpInheritedFieldV1{}}
	var state string
	req, state = applyExplicitQueryScopeIntent(req)
	if state == "AMBIGUOUS_SCOPE" || state == "SELECTED_EVIDENCE_REQUIRED" {
		f.State = state
	}
	q := forensictext.PlanningText(req.Query)
	a := assessDynamicQuery(q, req.Template)
	if a.Status == "unavailable" {
		f.State = "UNAVAILABLE"
		if len(a.MissingCapabilities) > 0 {
			f.State = a.MissingCapabilities[0]
		}
	}
	if containsAny(strings.ToLower(q), []string{"powershell", "execute shell", "run shell"}) {
		f.State = "ARBITRARY_EXECUTION_REJECTED"
	}
	plan := planRuntimeQuery(req)
	contextValid, _ := req.ConversationContext.validFor(req, time.Now().UTC())
	contextValid = contextValid && req.ConversationContext.TenantID == req.TenantID && req.ConversationContext.UserID == req.UserID && req.ConversationContext.CollectionID == req.CollectionID
	if !contextValid {
		req.ConversationContext = queryConversationContext{}
	}
	if direction := extractEventDirection(req.Query); direction != "" {
		req.Direction = direction
	}
	req, plan = applyAuditableConversationContext(req, plan, time.Now().UTC())
	f.Inherited = append(f.Inherited, req.ConversationContext.InheritedFields...)
	if contextValid {
		refs, _ := loadQueryCapabilityReferences()
		if len(refs.Capabilities) > 0 {
			for _, r := range refs.Capabilities {
				if queryTemplateNameByOperationID(r.OperationRef) == req.ConversationContext.Template {
					f.PriorCapability = r.ID
					if len(r.Semantics) == 1 {
						f.PriorSemantic = r.Semantics[0]
					}
					break
				}
			}
		}
		f.PriorTargets = canonicalTargetSet(req.ConversationContext.Target, req.ConversationContext.Targets)
	}
	f.TenantID, f.UserID, f.CollectionID, f.EvidenceID = req.TenantID, req.UserID, req.CollectionID, req.EvidenceID
	f.Scope = plannerScopeIntent(req)
	f.Targets = extractTargets(req.Query)
	if len(f.Targets) == 0 {
		f.Targets = canonicalTargetSet(req.Target, req.Targets)
	}
	f.Target = firstTarget(f.Targets)
	f.TextQuery, f.Family = forensictext.Extract(req.Query)
	if f.Family == "" {
		f.Family = strings.TrimSuffix(req.QueryScope.SourceFamily, "_intelligence")
	}
	if f.TextQuery != nil {
		if err := f.TextQuery.Validate(); err != nil {
			f.State = "AMBIGUOUS_LITERAL"
		} else {
			f.TextQuery.Representation = hybridTextRepresentation(q, f.TextQuery.LiteralText, f.Family)
		}
	} else if req.TextQuery != nil {
		copyText := *req.TextQuery
		f.TextQuery = &copyText
	}
	f.DateFrom, f.DateTo = extractDateRange(req.Query)
	f.DateFrom = defaultString(f.DateFrom, req.DateFrom)
	f.DateTo = defaultString(f.DateTo, req.DateTo)
	f.Direction = canonicalEventDirection(defaultString(extractEventDirection(req.Query), req.Direction))
	f.TopK, _ = extractPlannerTopK(req.Query)
	f.StartSeconds, f.EndSeconds, _ = extractPlannerSourceTime(req.Query)
	if f.TextQuery == nil && f.StartSeconds != nil && (f.Family == "audio" || containsAny(strings.ToLower(q), []string{"recording", "transcript", "audio", "ریکارڈنگ"})) {
		f.Family = "audio"
		f.TextQuery = &forensictext.Query{MatchSemantic: forensictext.TimeRange, Representation: "raw"}
	}
	f.Filters = applyCanonicalQueryHints(req, plan.Template).RawPayloadFilters
	f.ExplicitCapability = hybridExplicitCapability(req.Query, f.TextQuery, f.Family)
	if isContextualFollowUp(req.Query) && f.PriorCapability != "" {
		if f.ExplicitCapability == "" && (isReplacementOnlyFollowUp(req.Query) || isModifierOnlyFollowUp(req.Query)) {
			f.ExplicitCapability = f.PriorCapability
		}
		if f.ExplicitCapability != "" && f.ExplicitCapability != f.PriorCapability {
			f.Transition = "CAPABILITY_REPLACED"
		} else if f.Target != "" && !containsString(f.PriorTargets, f.Target) {
			f.Transition = "TARGET_REPLACED"
		} else {
			f.Transition = "CONTEXT_RETAINED"
		}
	}
	if f.TopK > maxHybridLimit || len(f.Targets) > maxCrossFamilyTargets {
		f.State = "TOP_K_BUDGET_EXCEEDED"
	}
	return req, f
}

func buildHybridTuples(req hybridQueryRequest, f HybridFactsV1) []HybridTupleV1 {
	out := []HybridTupleV1{}
	if f.State != "" {
		return out
	}
	refs, err := loadQueryCapabilityReferences()
	if err != nil {
		return out
	}
	for _, r := range refs.Capabilities {
		if !containsString(r.ScopeModes, f.Scope) {
			continue
		}
		if f.Scope == "selected_evidence" && !capabilityMatchesSelectedFamily(r, strings.TrimSuffix(req.QueryScope.SourceFamily, "_intelligence")) {
			continue
		}
		if f.ExplicitCapability != "" && r.ID != f.ExplicitCapability {
			continue
		}
		if f.ExplicitCapability == "" && len(f.Targets) > 0 && !capabilityMatchesQuery(r, normalizeAnalystSemantics(forensictext.PlanningText(req.Query)), f.Targets) {
			continue
		}
		for _, s := range r.Semantics {
			if f.TextQuery != nil && s != string(f.TextQuery.MatchSemantic) {
				continue
			}
			if f.TextQuery == nil && strings.HasSuffix(r.ID, ".phrase") {
				continue
			}
			if f.TopK > 0 && s != "TOP_K" && s != "SIMILARITY" {
				continue
			}
			if f.StartSeconds != nil && s != "TIME_RANGE" && s != "SOURCE_TIME" {
				continue
			}
			representation := ""
			required := []string{"scope"}
			if f.TextQuery != nil {
				representation = f.TextQuery.Representation
				required = append(required, "text_query")
			}
			if containsString([]string{"cdr.identifier_lookup", "cdr.frequent_contacts", "ipdr.endpoint", "ipdr.sessions", "subscriber.lookup", "tower.lookup", "device.observations"}, r.ID) {
				required = append(required, "identifiers")
				if len(f.Targets) == 0 {
					continue
				}
			}
			if r.ID == "roman_urdu.phrase" && representation != "roman_derivative" {
				continue
			}
			if r.ID == "document.phrase" && representation == "roman_derivative" {
				continue
			}
			out = append(out, HybridTupleV1{ID: HybridTupleID(r.ID + "/" + s + "/" + f.Scope), Capability: r.ID, Semantic: s, Scope: f.Scope, Family: r.Family, Operation: r.OperationRef, Representation: representation, Required: required, Optional: []string{"identifiers", "dates", "direction", "top_k", "source_time", "filters"}})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// One exclusive wire decision prevents schema-valid but contradictory state fields.
// Keep the richer server-derived audit record; it is not model-owned output.
func hybridResidualSchema(tuples []HybridTupleV1) map[string]any {
	ids := []string{"CLARIFY:AMBIGUOUS_INTENT", "CLARIFY:INSUFFICIENT_FACTS"}
	for _, t := range tuples {
		ids = append(ids, string(t.ID))
	}
	return map[string]any{"type": "object", "additionalProperties": false,
		"required": []string{"decision"}, "properties": map[string]any{
			"decision": map[string]any{"type": "string", "enum": ids},
		}}
}

func decodeHybridResidual(raw []byte, tuples []HybridTupleV1) (HybridResidualV1, error) {
	p := HybridResidualV1{ContractVersion: "forensics.hybrid-decision/v2"}
	if !utf8.Valid(raw) {
		return p, errors.New("residual is not UTF-8")
	}
	// Token-level decoding rejects duplicate keys as well as unknown fields.
	d := json.NewDecoder(bytes.NewReader(raw))
	token, err := d.Token()
	if err != nil || token != json.Delim('{') {
		return p, errors.New("residual requires decision object")
	}
	key, err := d.Token()
	if err != nil || key != "decision" {
		return p, errors.New("residual requires exactly one decision field")
	}
	var decision string
	if err := d.Decode(&decision); err != nil || decision == "" {
		return p, errors.New("invalid residual decision")
	}
	token, err = d.Token()
	if err != nil || token != json.Delim('}') {
		return p, errors.New("extra or duplicate residual field")
	}
	if d.Decode(&struct{}{}) != io.EOF {
		return p, errors.New("trailing residual data")
	}
	switch decision {
	case "CLARIFY:AMBIGUOUS_INTENT", "CLARIFY:INSUFFICIENT_FACTS":
		p.AmbiguityState = "NEEDS_CLARIFICATION"
		p.ClarificationCode = strings.TrimPrefix(decision, "CLARIFY:")
		return p, nil
	}
	for _, t := range tuples {
		if string(t.ID) == decision {
			p.SelectedTupleID = t.ID
			p.AmbiguityState = "RESOLVED"
			return p, nil
		}
	}
	return p, errors.New("unknown tuple or clarification decision")
}

func assembleHybridPlan(req hybridQueryRequest, f HybridFactsV1, tuple HybridTupleV1) (hybridQueryRequest, error) {
	if req.TenantID != f.TenantID || req.UserID != f.UserID || req.CollectionID != f.CollectionID || req.EvidenceID != f.EvidenceID || plannerScopeIntent(req) != f.Scope {
		return req, errors.New("fact authorization does not match request")
	}
	// Reconstruct membership from server facts; callers cannot smuggle a new
	// operation through a forged tuple with a familiar ID.
	valid := false
	for _, t := range buildHybridTuples(req, f) {
		if t.ID == tuple.ID && t.Capability == tuple.Capability && t.Semantic == tuple.Semantic && t.Operation == tuple.Operation && t.Scope == tuple.Scope && t.Representation == tuple.Representation {
			valid = true
		}
	}
	if !valid || f.State != "" {
		return req, errors.New("tuple not authorized by deterministic facts")
	}
	p := DynamicPlannerProposalV1{ContractVersion: dynamicPlannerProposalV1, QueryCapabilityID: tuple.Capability, Semantic: tuple.Semantic, ScopeIntent: f.Scope, Target: f.Target, TopK: f.TopK, StartSeconds: f.StartSeconds, EndSeconds: f.EndSeconds, Confidence: 1, Parameters: LanguageAssistanceParametersV1{Target: f.Target, Targets: f.Targets, DateFrom: f.DateFrom, DateTo: f.DateTo, Direction: f.Direction, TextQuery: f.TextQuery}}
	if f.TextQuery != nil {
		p.LiteralText = f.TextQuery.LiteralText
	}
	if tuple.Capability == "image.plate" {
		spans := authoritativeExactIdentifierSpans(req.Query, "image.plate")
		if len(spans) != 1 {
			return req, errors.New("plate requires one exact source span")
		}
		p.LiteralText = spans[0]
		p.Target = spans[0]
		p.Parameters.Target = spans[0]
		p.Parameters.Targets = nil
	}
	// The existing full-proposal validator validates current-turn facts. For
	// authorized inherited fields, validate against the current turn and then
	// attach only context already checked by validFor, never model fields.
	check := p
	if !strings.Contains(req.Query, p.Target) {
		check.Target = ""
		check.Parameters.Target = ""
		check.Parameters.Targets = nil
	}
	from, to := extractDateRange(req.Query)
	check.Parameters.DateFrom = from
	check.Parameters.DateTo = to
	if _, err := validateDynamicPlannerProposal(req, check); err != nil {
		return req, err
	}
	if tuple.Representation == "roman_derivative" && tuple.Capability != "roman_urdu.phrase" && tuple.Capability != "transcript.phrase" {
		return req, errors.New("unsupported text representation")
	}
	req.Template = queryTemplateNameByOperationID(tuple.Operation)
	req.Target, req.Targets = p.Target, canonicalTargetSet(p.Target, p.Parameters.Targets)
	req.DateFrom, req.DateTo, req.Direction = f.DateFrom, f.DateTo, f.Direction
	req.StartSeconds, req.EndSeconds = f.StartSeconds, f.EndSeconds
	req.RawPayloadFilters = f.Filters
	req.TextQuery = f.TextQuery
	if f.TopK > 0 {
		req.Limit = f.TopK
		req.MaxKBResults = f.TopK
	}
	req.DynamicProposal = &p
	return req, nil
}

func hybridResolvedStatus(status string) bool {
	return status == "hybrid_deterministic_plan" || status == "hybrid_model_plan" || status == "semantic_model_plan" || status == "semantic_dynamic_plan" || status == "semantic_deterministic_registered" || status == "semantic_deterministic_dynamic"
}

func resolveHybridPlanner(ctx context.Context, cfg config, req hybridQueryRequest) (hybridQueryRequest, string) {
	req, f := extractHybridFacts(req)
	audit := &HybridPlannerAuditV1{Facts: f, Candidates: buildHybridTuples(req, f), DecisionSource: "DETERMINISTIC_FACT", Reconciliation: []string{}}
	req.HybridAudit = audit
	finish := func(state string) (hybridQueryRequest, string) { audit.State = state; return req, state }
	if f.State != "" {
		return finish(f.State)
	}
	if len(audit.Candidates) == 0 {
		return finish("INSUFFICIENT_FACTS")
	}
	selected := audit.Candidates[0]
	status := "hybrid_deterministic_plan"
	if len(audit.Candidates) > 1 {
		if cfg.LocalAIURL == "" || !isForensicSynthesisModelName(req.SynthesisModel) {
			return finish("unavailable")
		}
		audit.DecisionSource = "MODEL_DECISION"
		choices, _ := json.Marshal(audit.Candidates)
		body, _ := json.Marshal(map[string]any{"model": req.SynthesisModel, "temperature": 0, "max_tokens": dynamicPlannerMaxCompletionTokens, "response_format": map[string]any{"type": "json_schema", "json_schema": map[string]any{"name": "forensic_hybrid_decision_v2", "strict": true, "schema": hybridResidualSchema(audit.Candidates)}}, "messages": []map[string]string{{"role": "system", "content": "Choose one server-issued tuple matching the analyst's residual intent. Return only a one-field JSON object: {\"decision\":\"<one allowed enum value>\"}. Never generate facts, parameters, scope, tools or operations. Choose CLARIFY:AMBIGUOUS_INTENT for ambiguous intent or CLARIFY:INSUFFICIENT_FACTS for insufficient facts; otherwise choose one tuple ID."}, {"role": "user", "content": req.Query + "\nValid tuples:\n" + string(choices)}}})
		timeout := languageAssistanceTimeout
		if cfg.SynthesisTimeout > 0 && cfg.SynthesisTimeout < timeout {
			timeout = cfg.SynthesisTimeout
		}
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
		response, err := httpclient.NewWithTimeout(timeout).Do(httpReq)
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
		audit.RawResidual = envelope.Choices[0].Message.Content
		if envelope.Choices[0].FinishReason != "stop" {
			return finish("incomplete_completion")
		}
		p, err := decodeHybridResidual([]byte(audit.RawResidual), audit.Candidates)
		if err != nil {
			return finish("malformed_residual")
		}
		audit.ModelDecision = &p
		if p.AmbiguityState == "NEEDS_CLARIFICATION" {
			return finish(p.ClarificationCode)
		}
		for _, t := range audit.Candidates {
			if t.ID == p.SelectedTupleID {
				selected = t
				break
			}
		}
		status = "hybrid_model_plan"
	}
	resolved, err := assembleHybridPlan(req, f, selected)
	if err != nil {
		return finish("hybrid_validation_rejected:" + fmt.Sprint(err))
	}
	audit.FinalPlan = resolved.DynamicProposal
	audit.ResolvedTarget = resolved.Target
	audit.State = status
	resolved.HybridAudit = audit
	return resolved, status
}
