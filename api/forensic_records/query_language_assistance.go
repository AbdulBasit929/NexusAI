package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/mudler/LocalAI/pkg/forensictext"
)

const languageAssistanceProposalV1 = "forensics.language-assistance-proposal/v1"
const dynamicPlannerProposalV1 = "forensics.dynamic-capability-proposal/v1"

const languageAssistanceTimeout = 60 * time.Second

// Preserve the accepted production budget while the hybrid model response
// contains only the residual decision. Full plans are assembled by the server.
const dynamicPlannerMaxCompletionTokens = 512

const dynamicPlannerSystemPrompt = "Select one allowlisted forensic query capability and semantic. Return only schema-valid compact JSON. Copy every literal and identifier exactly from the user. Obey the server-bounded constants for literals, counts, time, direction, and nested parameters. Keep clarification empty unless the request cannot be resolved. Calendar time and recording source-time are distinct. Never widen scope or add filters. The server owns authorization and derives the operation."

type LanguageAssistanceParametersV1 struct {
	TextQuery *forensictext.Query `json:"text_query,omitempty"`
	Target    string              `json:"target,omitempty"`
	Targets   []string            `json:"targets,omitempty"`
	DateFrom  string              `json:"date_from,omitempty"`
	DateTo    string              `json:"date_to,omitempty"`
	Direction string              `json:"direction,omitempty"`
}

type LanguageAssistanceProposalV1 struct {
	ContractVersion string                         `json:"contract_version"`
	OperationID     string                         `json:"operation_id"`
	FamilyID        string                         `json:"family_id"`
	Parameters      LanguageAssistanceParametersV1 `json:"parameters"`
	OutputContract  string                         `json:"output_contract"`
	Confidence      float64                        `json:"confidence"`
}

type DynamicPlannerProposalV1 struct {
	ContractVersion   string                         `json:"contract_version"`
	QueryCapabilityID string                         `json:"query_capability_id"`
	Semantic          string                         `json:"semantic"`
	LiteralText       string                         `json:"literal_text,omitempty"`
	Target            string                         `json:"target,omitempty"`
	ScopeIntent       string                         `json:"scope_intent"`
	TopK              int                            `json:"top_k,omitempty"`
	StartSeconds      *float64                       `json:"start_seconds,omitempty"`
	EndSeconds        *float64                       `json:"end_seconds,omitempty"`
	Clarification     string                         `json:"clarification,omitempty"`
	Parameters        LanguageAssistanceParametersV1 `json:"parameters"`
	Confidence        float64                        `json:"confidence"`
}

const exactIdentifierAuthorityV1 = "forensics.exact-identifier-authority/v1"
const phraseLiteralAuthorityV1 = "forensics.phrase-literal-authority/v1"

// ExactIdentifierAuthorityV1 keeps the model proposal and deterministic final
// value separate so a safe correction never hides model weakness.
type ExactIdentifierAuthorityV1 struct {
	ContractVersion     string   `json:"contract_version"`
	CapabilityID        string   `json:"capability_id"`
	Semantic            string   `json:"semantic"`
	Cardinality         string   `json:"cardinality"`
	AuthoritativeSpans  []string `json:"authoritative_spans"`
	ModelLiteralText    string   `json:"model_literal_text"`
	ModelEntities       []string `json:"model_entities"`
	ModelProposalStatus string   `json:"model_proposal_status"`
	MismatchReasons     []string `json:"mismatch_reasons"`
	Reconciled          bool     `json:"reconciled"`
	FinalLiteralText    string   `json:"final_literal_text"`
	FinalEntities       []string `json:"final_entities"`
}

// PhraseLiteralAuthorityV1 records the raw model fields separately from the
// exact quoted source span used by the final typed plan.
type PhraseLiteralAuthorityV1 struct {
	ContractVersion       string   `json:"contract_version"`
	CapabilityID          string   `json:"capability_id"`
	Semantic              string   `json:"semantic"`
	Cardinality           string   `json:"cardinality"`
	SourceLiteral         string   `json:"source_literal"`
	SourceCodepoints      []string `json:"source_codepoints"`
	SourceNormalized      string   `json:"source_normalized"`
	ModelLiteral          string   `json:"model_literal"`
	ModelCodepoints       []string `json:"model_codepoints"`
	ModelNormalized       string   `json:"model_normalized"`
	ModelClarification    string   `json:"model_clarification"`
	ModelProposalStatus   string   `json:"model_proposal_status"`
	MismatchReasons       []string `json:"mismatch_reasons"`
	ScriptFamilyMismatch  bool     `json:"script_family_mismatch"`
	Reconciled            bool     `json:"reconciled"`
	ReconciliationReason  string   `json:"reconciliation_reason"`
	FinalLiteral          string   `json:"final_literal"`
	FinalCodepoints       []string `json:"final_codepoints"`
	FinalClarification    string   `json:"final_clarification"`
	NormalizationContract string   `json:"normalization_contract"`
}

func unicodeCodepoints(value string) []string {
	result := make([]string, 0, len([]rune(value)))
	for _, r := range value {
		result = append(result, fmt.Sprintf("U+%04X", r))
	}
	return result
}

func phraseScriptFamilyMismatch(source, model string) bool {
	hasArabic := func(value string) bool {
		for _, r := range value {
			if unicode.In(r, unicode.Arabic) {
				return true
			}
		}
		return false
	}
	hasCyrillic := func(value string) bool {
		for _, r := range value {
			if unicode.In(r, unicode.Cyrillic) {
				return true
			}
		}
		return false
	}
	return hasArabic(source) && !hasCyrillic(source) && hasCyrillic(model)
}

func phraseCapabilityFamily(capabilityID string) string {
	switch capabilityID {
	case "document.phrase":
		return "document"
	case "transcript.phrase", "roman_urdu.phrase":
		return "audio"
	default:
		return ""
	}
}

func reconcileExplicitPhraseLiteral(question string, proposal DynamicPlannerProposalV1) (*PhraseLiteralAuthorityV1, error) {
	wantedFamily := phraseCapabilityFamily(proposal.QueryCapabilityID)
	if wantedFamily == "" || !containsString([]string{"EXACT_VALUE", "EXACT_PHRASE", "PHRASE_CONTAINS", "TOKEN_SEARCH"}, proposal.Semantic) {
		return nil, nil
	}
	extracted, family := forensictext.Extract(question)
	if extracted == nil {
		return nil, nil
	}
	if extracted.MatchSemantic == "AMBIGUOUS_LITERAL" {
		return nil, errors.New("multiple explicit phrase spans require clarification")
	}
	if family == "" {
		return nil, nil
	}
	if family != wantedFamily {
		return nil, errors.New("explicit phrase family contradicts the proposed capability")
	}
	if string(extracted.MatchSemantic) != proposal.Semantic {
		return nil, errors.New("explicit phrase semantic contradicts the proposed semantic")
	}
	source := extracted.LiteralText
	reasons := make([]string, 0, 4)
	if proposal.LiteralText != source {
		reasons = append(reasons, "literal_not_exact_source_span")
	}
	if proposal.Clarification != "" {
		reasons = append(reasons, "unjustified_model_clarification")
	}
	if proposal.Parameters.TextQuery == nil {
		reasons = append(reasons, "typed_text_query_missing")
	} else if proposal.Parameters.TextQuery.LiteralText != source || string(proposal.Parameters.TextQuery.MatchSemantic) != proposal.Semantic {
		reasons = append(reasons, "typed_text_query_not_exact_source_span")
	}
	scriptMismatch := phraseScriptFamilyMismatch(source, proposal.LiteralText)
	if scriptMismatch {
		reasons = append(reasons, "model_script_family_mismatch")
	}
	return &PhraseLiteralAuthorityV1{
		ContractVersion:       phraseLiteralAuthorityV1,
		CapabilityID:          proposal.QueryCapabilityID,
		Semantic:              proposal.Semantic,
		Cardinality:           "exactly_one_explicit_quoted_span",
		SourceLiteral:         source,
		SourceCodepoints:      unicodeCodepoints(source),
		SourceNormalized:      forensictext.Normalize(source),
		ModelLiteral:          proposal.LiteralText,
		ModelCodepoints:       unicodeCodepoints(proposal.LiteralText),
		ModelNormalized:       forensictext.Normalize(proposal.LiteralText),
		ModelClarification:    proposal.Clarification,
		ModelProposalStatus:   map[bool]string{true: "PASS", false: "FAIL_PHRASE_LITERAL"}[len(reasons) == 0],
		MismatchReasons:       reasons,
		ScriptFamilyMismatch:  scriptMismatch,
		Reconciled:            len(reasons) > 0,
		ReconciliationReason:  "one explicit quoted phrase and matching typed capability/scope context",
		FinalLiteral:          source,
		FinalCodepoints:       unicodeCodepoints(source),
		FinalClarification:    "",
		NormalizationContract: forensictext.NormalizationV1,
	}, nil
}

type exactIdentifierSpan struct {
	value string
	start int
	end   int
}

func authoritativeExactIdentifierSpans(question, capabilityID string) []string {
	spans := make([]exactIdentifierSpan, 0)
	seen := map[string]struct{}{}
	protected := protectedTargetSpans(question)
	for _, pattern := range targetPatterns {
		for _, loc := range pattern.FindAllStringIndex(question, -1) {
			if overlapsProtectedSpan(loc, protected) {
				continue
			}
			value := question[loc[0]:loc[1]]
			if capabilityID == "image.plate" && !looksLikeANPRPlateTarget(value) {
				continue
			}
			key := fmt.Sprintf("%d:%d", loc[0], loc[1])
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			spans = append(spans, exactIdentifierSpan{value: value, start: loc[0], end: loc[1]})
		}
	}
	sort.SliceStable(spans, func(i, j int) bool {
		if spans[i].start == spans[j].start {
			return spans[i].end > spans[j].end
		}
		return spans[i].start < spans[j].start
	})
	selected := make([]exactIdentifierSpan, 0, len(spans))
	for _, candidate := range spans {
		overlaps := false
		for _, existing := range selected {
			if candidate.start < existing.end && candidate.end > existing.start {
				overlaps = true
				break
			}
		}
		if !overlaps {
			selected = append(selected, candidate)
		}
	}
	values := make([]string, 0, len(selected))
	for _, span := range selected {
		values = append(values, span.value)
	}
	return values
}

func reconcileSingleExactIdentifier(question, capabilityID, semantic, modelLiteral string, modelEntities []string) (*ExactIdentifierAuthorityV1, error) {
	if capabilityID != "image.plate" || semantic != "EXACT_VALUE" {
		return nil, nil
	}
	spans := authoritativeExactIdentifierSpans(question, capabilityID)
	if len(spans) == 0 {
		return nil, errors.New("exact identifier authority found no bounded image.plate span")
	}
	if len(spans) != 1 {
		return nil, errors.New("image.plate accepts one exact target; multiple candidate spans require clarification")
	}
	authoritative := spans[0]
	reasons := make([]string, 0, 3)
	if modelLiteral != authoritative {
		reasons = append(reasons, "literal_not_exact_source_span")
	}
	if len(modelEntities) != 1 {
		reasons = append(reasons, "entity_count_not_one")
	}
	if len(modelEntities) != 1 || modelEntities[0] != authoritative {
		reasons = append(reasons, "atomic_entity_not_exact_source_span")
	}
	for _, entity := range modelEntities {
		if !strings.Contains(question, entity) {
			reasons = append(reasons, "model_entity_not_in_question")
			break
		}
	}
	return &ExactIdentifierAuthorityV1{
		ContractVersion:     exactIdentifierAuthorityV1,
		CapabilityID:        capabilityID,
		Semantic:            semantic,
		Cardinality:         "exactly_one",
		AuthoritativeSpans:  spans,
		ModelLiteralText:    modelLiteral,
		ModelEntities:       append([]string(nil), modelEntities...),
		ModelProposalStatus: map[bool]string{true: "PASS", false: "FAIL_ATOMIC_IDENTIFIER"}[len(reasons) == 0],
		MismatchReasons:     reasons,
		Reconciled:          len(reasons) > 0,
		FinalLiteralText:    authoritative,
		FinalEntities:       []string{authoritative},
	}, nil
}

func proposalIdentifierEntities(proposal DynamicPlannerProposalV1) []string {
	entities := make([]string, 0, 1+len(proposal.Parameters.Targets))
	if proposal.Target != "" {
		entities = append(entities, proposal.Target)
	}
	entities = append(entities, proposal.Parameters.Targets...)
	return entities
}

func applyExactIdentifierAuthority(req hybridQueryRequest, proposal DynamicPlannerProposalV1) (DynamicPlannerProposalV1, *ExactIdentifierAuthorityV1, error) {
	authority, err := reconcileSingleExactIdentifier(req.Query, proposal.QueryCapabilityID, proposal.Semantic, proposal.LiteralText, proposalIdentifierEntities(proposal))
	if err != nil || authority == nil {
		return proposal, authority, err
	}
	proposal.LiteralText = authority.FinalLiteralText
	proposal.Target = authority.FinalEntities[0]
	proposal.Parameters.Target = authority.FinalEntities[0]
	proposal.Parameters.Targets = nil
	return proposal, authority, nil
}

func applyPhraseLiteralAuthority(req hybridQueryRequest, proposal DynamicPlannerProposalV1) (DynamicPlannerProposalV1, *PhraseLiteralAuthorityV1, error) {
	authority, err := reconcileExplicitPhraseLiteral(req.Query, proposal)
	if err != nil || authority == nil {
		return proposal, authority, err
	}
	proposal.LiteralText = authority.FinalLiteral
	proposal.Clarification = authority.FinalClarification
	representation := ""
	if proposal.Parameters.TextQuery != nil {
		representation = proposal.Parameters.TextQuery.Representation
	}
	proposal.Parameters.TextQuery = &forensictext.Query{LiteralText: authority.FinalLiteral, MatchSemantic: forensictext.Semantic(proposal.Semantic), Representation: representation}
	return proposal, authority, nil
}

func decodeDynamicPlannerProposal(raw []byte) (DynamicPlannerProposalV1, error) {
	var proposal DynamicPlannerProposalV1
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&proposal); err != nil {
		return proposal, fmt.Errorf("decode dynamic planner proposal: %w", err)
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return proposal, errors.New("dynamic planner proposal contains trailing content")
	}
	return proposal, nil
}

func validateDynamicPlannerProposal(req hybridQueryRequest, proposal DynamicPlannerProposalV1) (QueryCapabilityReferenceV1, error) {
	if proposal.ContractVersion != dynamicPlannerProposalV1 || proposal.Confidence < 0 || proposal.Confidence > 1 {
		return QueryCapabilityReferenceV1{}, errors.New("invalid dynamic planner proposal identity")
	}
	refs, err := loadQueryCapabilityReferences()
	if err != nil {
		return QueryCapabilityReferenceV1{}, err
	}
	var ref QueryCapabilityReferenceV1
	for _, candidate := range refs.Capabilities {
		if candidate.ID == proposal.QueryCapabilityID {
			ref = candidate
			break
		}
	}
	if ref.ID == "" {
		return ref, errors.New("proposal references an unknown query capability")
	}
	if !containsString(ref.Semantics, proposal.Semantic) {
		return ref, errors.New("proposal semantic is not supported by the capability")
	}
	if authority, err := reconcileSingleExactIdentifier(req.Query, proposal.QueryCapabilityID, proposal.Semantic, proposal.LiteralText, proposalIdentifierEntities(proposal)); err != nil {
		return ref, err
	} else if authority != nil && authority.Reconciled {
		return ref, errors.New("proposal exact identifier fields are not authoritative source spans")
	}
	if authority, err := reconcileExplicitPhraseLiteral(req.Query, proposal); err != nil {
		return ref, err
	} else if authority != nil && authority.Reconciled {
		return ref, errors.New("proposal phrase fields are not authoritative source spans")
	}
	if proposal.LiteralText != "" && !strings.Contains(req.Query, proposal.LiteralText) {
		return ref, errors.New("proposal changed literal user span")
	}
	if proposal.Parameters.TextQuery != nil {
		q := proposal.Parameters.TextQuery
		if err := q.Validate(); err != nil || q.LiteralText != proposal.LiteralText || string(q.MatchSemantic) != proposal.Semantic {
			return ref, errors.New("proposal text semantic contradicts the typed literal")
		}
	}
	if proposal.Parameters.Target != "" && normalizeExtractedTarget(proposal.Parameters.Target) != normalizeExtractedTarget(proposal.Target) {
		return ref, errors.New("proposal parameters contradict the typed target")
	}
	wantedScope := plannerScopeIntent(req)
	if proposal.ScopeIntent != wantedScope || !containsString(ref.ScopeModes, wantedScope) {
		return ref, errors.New("proposal changed or selected an unsupported scope")
	}
	if wantedScope == "selected_evidence" && strings.TrimSpace(req.EvidenceID) == "" {
		return ref, errors.New("selected capability requires authoritative evidence scope")
	}
	proposedTarget := normalizeExtractedTarget(proposal.Target)
	extracted := canonicalTargetSet("", extractTargets(req.Query))
	if proposedTarget != "" && (!strings.Contains(req.Query, proposal.Target) || !containsString(extracted, proposedTarget)) {
		return ref, errors.New("proposal invented or changed an identifier")
	}
	for _, target := range canonicalTargetSet("", proposal.Parameters.Targets) {
		if !strings.Contains(req.Query, target) || !containsString(extracted, target) {
			return ref, errors.New("proposal invented or changed a target-set identifier")
		}
	}
	wantedTopK, hasTopK := extractPlannerTopK(req.Query)
	if proposal.TopK < 0 || proposal.TopK > maxHybridLimit || hasTopK && proposal.TopK != wantedTopK || !hasTopK && proposal.TopK != 0 {
		return ref, errors.New("proposal top-k is absent or outside the execution budget")
	}
	start, end, hasSourceTime := extractPlannerSourceTime(req.Query)
	if proposal.StartSeconds != nil || proposal.EndSeconds != nil {
		if !hasSourceTime || !sameOptionalFloat(proposal.StartSeconds, start) || !sameOptionalFloat(proposal.EndSeconds, end) {
			return ref, errors.New("proposal changed recording source-time bounds")
		}
		if proposal.Semantic != "TIME_RANGE" && proposal.Semantic != "SOURCE_TIME" {
			return ref, errors.New("source-time bounds require a source-time semantic")
		}
	}
	if proposal.Parameters.DateFrom != "" || proposal.Parameters.DateTo != "" {
		from, to := extractDateRange(req.Query)
		if proposal.Parameters.DateFrom != from || proposal.Parameters.DateTo != to {
			return ref, errors.New("proposal changed calendar-time bounds")
		}
	}
	wantedDirection := canonicalEventDirection(defaultString(req.Direction, extractEventDirection(req.Query)))
	if proposal.Parameters.Direction != "" && canonicalEventDirection(proposal.Parameters.Direction) != wantedDirection {
		return ref, errors.New("proposal changed direction")
	}
	return ref, nil
}

func dynamicCapabilityReference(id string) (QueryCapabilityReferenceV1, bool) {
	refs, err := loadQueryCapabilityReferences()
	if err != nil {
		return QueryCapabilityReferenceV1{}, false
	}
	for _, ref := range refs.Capabilities {
		if ref.ID == id {
			return ref, true
		}
	}
	return QueryCapabilityReferenceV1{}, false
}

var plannerSourceTimePattern = regexp.MustCompile(`(?i)(\d{1,2}):(\d{2})(?::(\d{2}))?\s*(?:-|to|through|se|سے)\s*(\d{1,2}):(\d{2})(?::(\d{2}))?`)

func extractPlannerSourceTime(query string) (*float64, *float64, bool) {
	m := plannerSourceTimePattern.FindStringSubmatch(query)
	if len(m) == 0 {
		return nil, nil, false
	}
	seconds := func(h, minute, second string) float64 {
		a, _ := strconv.Atoi(h)
		b, _ := strconv.Atoi(minute)
		c, _ := strconv.Atoi(second)
		if second == "" {
			return float64(a*60 + b)
		}
		return float64(a*3600 + b*60 + c)
	}
	start, end := seconds(m[1], m[2], m[3]), seconds(m[4], m[5], m[6])
	if start > end {
		return nil, nil, false
	}
	return &start, &end, true
}

func sameOptionalFloat(left, right *float64) bool {
	return left == nil && right == nil || left != nil && right != nil && *left == *right
}

func plannerScopeIntent(req hybridQueryRequest) string {
	if req.QueryScope.Kind == string(EvidenceScopeSelected) || strings.TrimSpace(req.EvidenceID) != "" {
		return "selected_evidence"
	}
	return "authorized_workspace"
}

func extractPlannerTopK(query string) (int, bool) {
	normalized := strings.ToLower(strings.Join(strings.Fields(forensictext.PlanningText(query)), " "))
	ranking := regexp.MustCompile(`(?i)\b(top|first|nearest|closest|most frequent|frequent contacts?|rank|ranking|list)\b`).MatchString(normalized) ||
		containsAny(normalized, []string{"sab se zyada", "paanch", "پانچ", "زیادہ رابط"})
	if !ranking {
		return 0, false
	}
	for _, pattern := range []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(?:top|first|nearest|closest|list)\s+(\d{1,3})\b`),
		regexp.MustCompile(`(?i)\b(\d{1,3})\s+(?:most|top|nearest|closest|frequent|contacts?|results?)\b`),
	} {
		if match := pattern.FindStringSubmatch(normalized); len(match) == 2 {
			value, _ := strconv.Atoi(match[1])
			if value >= 1 && value <= maxHybridLimit {
				return value, true
			}
		}
	}
	words := []struct {
		value int
		forms []string
	}{
		{1, []string{"one", "aik", "ek", "ایک"}}, {2, []string{"two", "do", "دو"}},
		{3, []string{"three", "teen", "تین"}}, {4, []string{"four", "char", "chaar", "چار"}},
		{5, []string{"five", "panch", "paanch", "پانچ"}}, {6, []string{"six", "chay", "چھ"}},
		{7, []string{"seven", "saat", "سات"}}, {8, []string{"eight", "aath", "آٹھ"}},
		{9, []string{"nine", "nau", "نو"}}, {10, []string{"ten", "das", "دس"}},
	}
	for _, word := range words {
		for _, form := range word.forms {
			if regexp.MustCompile(`(^|[^\pL\pN])` + regexp.QuoteMeta(form) + `([^\pL\pN]|$)`).MatchString(normalized) {
				return word.value, true
			}
		}
	}
	return 0, false
}

func decodeLanguageAssistanceProposal(raw []byte) (LanguageAssistanceProposalV1, error) {
	var proposal LanguageAssistanceProposalV1
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&proposal); err != nil {
		return proposal, fmt.Errorf("decode language proposal: %w", err)
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return proposal, errors.New("language proposal contains trailing content")
	}
	return proposal, nil
}

func validateLanguageAssistanceProposal(proposal LanguageAssistanceProposalV1) (queryTemplateCatalogEntry, error) {
	if proposal.ContractVersion != languageAssistanceProposalV1 {
		return queryTemplateCatalogEntry{}, errors.New("unsupported language proposal contract")
	}
	templateName := queryTemplateNameByOperationID(proposal.OperationID)
	template, exists := queryTemplateByName(templateName)
	if !exists || proposal.FamilyID != "" && template.FamilyID != proposal.FamilyID {
		return queryTemplateCatalogEntry{}, errors.New("proposal references an unknown operation or contradictory family")
	}
	if proposal.OutputContract != "forensics.query-understanding/v1" || proposal.Confidence < 0 || proposal.Confidence > 1 {
		return queryTemplateCatalogEntry{}, errors.New("proposal has an unknown output contract or invalid confidence")
	}
	if len(proposal.Parameters.Targets) > maxCrossFamilyTargets {
		return queryTemplateCatalogEntry{}, errors.New("proposal exceeds target budget")
	}
	if direction := proposal.Parameters.Direction; direction != "" && canonicalEventDirection(direction) == "" {
		return queryTemplateCatalogEntry{}, errors.New("proposal has an unknown direction filter")
	}
	return template, nil
}

func applyLanguageAssistanceProposal(req hybridQueryRequest, proposal LanguageAssistanceProposalV1) (hybridQueryRequest, error) {
	template, err := validateLanguageAssistanceProposal(proposal)
	if err != nil {
		return req, err
	}
	if q := proposal.Parameters.TextQuery; q != nil {
		if err := q.Validate(); err != nil {
			return req, err
		}
		if !isFamilyDerivedTextTemplate(template.Name) {
			return req, errors.New("text semantic requires a derived-text operation")
		}
		if q.LiteralText != "" && !strings.Contains(req.Query, q.LiteralText) {
			return req, errors.New("proposal changed literal user span")
		}
		if req.TextQuery != nil && *req.TextQuery != *q {
			return req, errors.New("proposal changed explicit text semantics")
		}
		if q.Representation == "roman_derivative" && template.Name != "audio_transcript_search" {
			return req, errors.New("unsupported text representation")
		}
		req.TextQuery = q
	}
	planner := planRuntimeQuery(req)
	proposedTarget := normalizeExtractedTarget(proposal.Parameters.Target)
	if planner.Target == "" && proposedTarget != "" || planner.Target != "" && proposedTarget != "" && proposedTarget != planner.Target {
		return req, errors.New("proposal invented or changed the target filter")
	}
	allowedTargets := canonicalTargetSet(planner.Target, planner.Targets)
	for _, target := range canonicalTargetSet("", proposal.Parameters.Targets) {
		if !containsString(allowedTargets, target) {
			return req, errors.New("proposal invented or changed a target-set filter")
		}
	}
	proposedDirection := canonicalEventDirection(proposal.Parameters.Direction)
	if req.Direction == "" && proposedDirection != "" || req.Direction != "" && proposedDirection != "" && proposedDirection != req.Direction {
		return req, errors.New("proposal invented or changed the direction filter")
	}
	// Scope is intentionally absent from the proposal schema. The request's
	// already-authorized tenant/user/collection values remain untouched.
	req.Template = template.Name
	req.Target = defaultString(planner.Target, proposedTarget)
	req.Targets = canonicalTargetSet(req.Target, proposal.Parameters.Targets)
	req.DateFrom = strings.TrimSpace(proposal.Parameters.DateFrom)
	req.DateTo = strings.TrimSpace(proposal.Parameters.DateTo)
	if req.Direction == "" {
		req.Direction = proposedDirection
	}
	if parsed, err := time.Parse(time.RFC3339, req.DateTo); err == nil && parsed.Hour() == 23 && parsed.Minute() == 59 && parsed.Second() == 59 {
		req.DateTo = parsed.Add(time.Second).UTC().Format(time.RFC3339)
	}
	return req, nil
}

func shouldUseLanguageAssistance(req hybridQueryRequest, planner runtimePlan) bool {
	return req.TextQuery == nil && req.TextQueryError == "" && strings.TrimSpace(req.Query) != "" && req.Template == "" && planner.Template == "" && strings.TrimSpace(req.SynthesisModel) != ""
}

func resolveWithLanguageAssistance(ctx context.Context, cfg config, req hybridQueryRequest) (hybridQueryRequest, string) {
	semantic, semanticStatus := resolveOpenEndedSemanticPlanner(ctx, cfg, req)
	if hybridResolvedStatus(semanticStatus) {
		return semantic, semanticStatus
	}
	if semantic.SemanticPlannerAudit != nil && semantic.SemanticPlannerAudit.Language == "en" && semanticStatus != "unavailable" {
		// A bounded semantic decision must not be overridden by a second,
		// question-filtered chooser with a different candidate universe.
		return semantic, semanticStatus
	}
	// Preserve the established compact residual chooser as a compatibility
	// fallback for multilingual and already-covered capability tuples.
	residual, residualStatus := resolveHybridPlanner(ctx, cfg, semantic)
	residual.SemanticPlannerAudit = semantic.SemanticPlannerAudit
	if hybridResolvedStatus(residualStatus) {
		return residual, residualStatus
	}
	if semanticStatus == "AMBIGUOUS_INTENT" || semanticStatus == "INSUFFICIENT_FACTS" || semanticStatus == "UNSUPPORTED_REQUEST_CLASS" {
		return residual, semanticStatus
	}
	return residual, residualStatus
}

func languageAssistanceAllowlist(query ...string) string {
	templates := languageAssistanceCandidates(strings.Join(query, " "))
	lines := make([]string, 0, len(templates))
	for _, template := range templates {
		lines = append(lines, template.OperationID+" | "+template.FamilyID+" | "+template.Name)
	}
	return strings.Join(lines, "\n")
}

func dynamicCapabilityAllowlist(req hybridQueryRequest) string {
	refs := dynamicCapabilityCandidates(req)
	lines := make([]string, 0, len(refs))
	for _, ref := range refs {
		lines = append(lines, ref.ID+" | "+strings.Join(ref.Semantics, ",")+" | "+ref.Authority)
	}
	return strings.Join(lines, "\n")
}

func dynamicCapabilityCandidates(req hybridQueryRequest) []QueryCapabilityReferenceV1 {
	refs, err := loadQueryCapabilityReferences()
	if err != nil {
		return nil
	}
	scope := plannerScopeIntent(req)
	query := normalizeAnalystSemantics(forensictext.PlanningText(req.Query))
	targets := extractTargets(req.Query)
	selected := make([]QueryCapabilityReferenceV1, 0, len(refs.Capabilities))
	for _, ref := range refs.Capabilities {
		if !containsString(ref.ScopeModes, scope) {
			continue
		}
		if req.QueryScope.SourceFamily != "" && scope == "selected_evidence" && !capabilityMatchesSelectedFamily(ref, req.QueryScope.SourceFamily) {
			continue
		}
		selected = append(selected, ref)
	}
	// Shape and explicit-intent filtering keeps the prompt small. If no safe
	// discriminator applies, retain every scope-compatible C reference.
	filtered := make([]QueryCapabilityReferenceV1, 0, len(selected))
	for _, ref := range selected {
		if capabilityMatchesQuery(ref, query, targets) {
			filtered = append(filtered, ref)
		}
	}
	if len(filtered) > 0 {
		return filtered
	}
	return selected
}

func capabilityMatchesSelectedFamily(ref QueryCapabilityReferenceV1, family string) bool {
	switch normalize(family) {
	case "audio":
		return ref.ID == "transcript.phrase" || ref.ID == "roman_urdu.phrase"
	case "document", "text":
		return ref.ID == "document.phrase"
	case "image":
		return ref.ID == "ocr.phrase" || ref.ID == "image.plate" || ref.ID == "image.similarity" || ref.ID == "face.candidates"
	case "video":
		return ref.ID == "video.group_plate"
	}
	return true
}

func capabilityMatchesQuery(ref QueryCapabilityReferenceV1, query string, targets []string) bool {
	words := map[string][]string{
		"cdr.identifier_lookup": {"call record", "cdr record"}, "cdr.frequent_contacts": {"frequent contact", "top contact", "most often", "sab se zyada"}, "cdr.time_activity": {"call activity", "hourly call", "ghanta war"},
		"ipdr.endpoint": {"endpoint summary", "network endpoint"}, "ipdr.sessions": {"ipdr session", "internet session"}, "subscriber.lookup": {"subscriber", "سبسکرائبر", "tafseel"}, "device.observations": {"imei", "imsi", "device", "sim identifier"}, "tower.lookup": {"tower", "site lookup", "ٹاور"},
		"financial.summary": {"transaction", "account", "لین دین"}, "logs.failures": {"failed login", "failed access", "ناکام رسائی"}, "generic.filter": {"tabular", "generic row", "جدول"},
		"document.phrase": {"document", "دستاویز"}, "knowledge.semantic": {"what do the documents", "about", "کے بارے"}, "ocr.phrase": {"ocr", "recognized text", "تصویر کے متن"}, "transcript.phrase": {"audio", "recording", "transcript", "ریکارڈنگ"}, "roman_urdu.phrase": {"roman urdu"},
		"image.plate": {"plate", "number plate", "نمبر پلیٹ"}, "video.group_plate": {"grouped", "timeline", "video", "ویڈیو"}, "image.similarity": {"similar image", "visual match", "ملتی جلتی تصاویر"}, "face.candidates": {"face candidate", "similar face", "چہرے"},
		"cross_family.identifiers": {"cross family", "across families", "مختلف اقسام"}, "case.evidence_package": {"evidence package", "investigation summary", "شواہد کا مجموعی"},
	}
	if containsAny(query, words[ref.ID]) {
		return true
	}
	for _, target := range targets {
		switch classifyTargetType(target) {
		case "phone":
			if strings.HasPrefix(ref.ID, "cdr.") || ref.ID == "subscriber.lookup" || ref.ID == "device.observations" || ref.ID == "cross_family.identifiers" {
				return true
			}
		case "ip":
			if strings.HasPrefix(ref.ID, "ipdr.") || ref.ID == "cross_family.identifiers" {
				return true
			}
		}
		if looksLikeANPRPlateTarget(target) && (ref.ID == "image.plate" || ref.ID == "video.group_plate" || ref.ID == "cross_family.identifiers") {
			return true
		}
	}
	return false
}

func dynamicPlannerJSONSchema(req hybridQueryRequest) map[string]any {
	refs := dynamicCapabilityCandidates(req)
	ids, semantics := []string{}, map[string]bool{}
	for _, ref := range refs {
		ids = append(ids, ref.ID)
		for _, semantic := range ref.Semantics {
			semantics[semantic] = true
		}
	}
	semanticValues := make([]string, 0, len(semantics))
	for semantic := range semantics {
		semanticValues = append(semanticValues, semantic)
	}
	sort.Strings(ids)
	sort.Strings(semanticValues)
	allowedTargets := append([]string{""}, extractTargets(req.Query)...)
	literalSchema := map[string]any{"type": "string", "const": ""}
	parameterProperties := map[string]any{
		"target":  map[string]any{"type": "string", "enum": allowedTargets},
		"targets": map[string]any{"type": "array", "items": map[string]any{"type": "string", "enum": allowedTargets}, "maxItems": maxCrossFamilyTargets},
	}
	dateFrom, dateTo := extractDateRange(req.Query)
	parameterProperties["date_from"] = map[string]any{"type": "string", "const": dateFrom}
	parameterProperties["date_to"] = map[string]any{"type": "string", "const": dateTo}
	parameterProperties["direction"] = map[string]any{"type": "string", "const": canonicalEventDirection(defaultString(req.Direction, extractEventDirection(req.Query)))}
	parameterSchema := map[string]any{"type": "object", "additionalProperties": false, "properties": parameterProperties}
	if textQuery, _ := forensictext.Extract(req.Query); textQuery != nil && textQuery.MatchSemantic != "AMBIGUOUS_LITERAL" {
		literalSchema = map[string]any{"type": "string", "const": textQuery.LiteralText}
		semanticValues = []string{string(textQuery.MatchSemantic)}
		parameterProperties["text_query"] = map[string]any{"type": "object", "additionalProperties": false,
			"required": []string{"literal_text", "match_semantic", "text_representation"},
			"properties": map[string]any{
				"literal_text":        map[string]any{"type": "string", "const": textQuery.LiteralText},
				"match_semantic":      map[string]any{"type": "string", "const": string(textQuery.MatchSemantic)},
				"text_representation": map[string]any{"type": "string", "enum": []string{"", "raw", "roman_derivative"}},
			},
		}
		parameterSchema["required"] = []string{"text_query"}
	}
	wantedTopK, _ := extractPlannerTopK(req.Query)
	start, end, _ := extractPlannerSourceTime(req.Query)
	optionalFloatSchema := func(value *float64) map[string]any {
		if value == nil {
			return map[string]any{"type": "null", "const": nil}
		}
		return map[string]any{"type": "number", "const": *value, "minimum": 0}
	}
	return map[string]any{"type": "object", "additionalProperties": false,
		"required": []string{"contract_version", "query_capability_id", "semantic", "literal_text", "target", "scope_intent", "top_k", "start_seconds", "end_seconds", "clarification", "parameters", "confidence"},
		"properties": map[string]any{
			"contract_version": map[string]any{"type": "string", "const": dynamicPlannerProposalV1}, "query_capability_id": map[string]any{"type": "string", "enum": ids}, "semantic": map[string]any{"type": "string", "enum": semanticValues},
			"literal_text": literalSchema, "target": map[string]any{"type": "string", "enum": allowedTargets}, "scope_intent": map[string]any{"type": "string", "const": plannerScopeIntent(req)},
			"top_k": map[string]any{"type": "integer", "const": wantedTopK}, "start_seconds": optionalFloatSchema(start), "end_seconds": optionalFloatSchema(end), "clarification": map[string]any{"type": "string", "maxLength": 256}, "confidence": map[string]any{"type": "number", "minimum": 0, "maximum": 1},
			"parameters": parameterSchema,
		},
	}
}

func languageAssistanceCandidates(query string) []queryTemplateCatalogEntry {
	all := supportedQueryTemplates()
	normalized := normalizeAnalystSemantics(query)
	selected := map[string]bool{}
	switch {
	case containsAny(normalized, []string{"rabtay", "contacts", "contact most", "frequent contact"}):
		selected["frequent_contacts"] = true
	case containsAny(normalized, []string{"cdr actvty", "cdr activity", "calls activity", "call activity", "سرگرمی"}):
		selected["temporal_activity"] = true
	case containsAny(normalized, []string{"subscriber tafseel", "subscriber detail", "سبسکرائبر"}):
		selected["subscriber_identity_lookup"] = true
	default:
		return all
	}
	bounded := make([]queryTemplateCatalogEntry, 0, len(selected))
	for _, template := range all {
		if selected[template.Name] {
			bounded = append(bounded, template)
		}
	}
	return bounded
}

func languageProposalJSONSchema(query ...string) map[string]any {
	rawQuery := strings.Join(query, " ")
	candidates := languageAssistanceCandidates(rawQuery)
	operationIDs := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		operationIDs = append(operationIDs, candidate.OperationID)
	}
	allowedTargets := append([]string{""}, extractTargets(rawQuery)...)
	directionValues := []string{""}
	if direction := canonicalEventDirection(extractEventDirection(rawQuery)); direction != "" {
		directionValues = append(directionValues, direction)
	}
	return map[string]any{
		"type": "object", "additionalProperties": false,
		"required": []string{"contract_version", "operation_id", "parameters", "output_contract", "confidence"},
		"properties": map[string]any{
			"contract_version": map[string]any{"type": "string", "const": languageAssistanceProposalV1},
			"operation_id":     map[string]any{"type": "string", "enum": operationIDs},
			"output_contract":  map[string]any{"type": "string", "const": queryUnderstandingContractV1}, "confidence": map[string]any{"type": "number", "minimum": 0, "maximum": 1},
			"parameters": map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{
				"target": map[string]any{"type": "string", "enum": allowedTargets}, "targets": map[string]any{"type": "array", "items": map[string]any{"type": "string", "enum": allowedTargets}, "maxItems": maxCrossFamilyTargets},
				"text_query": map[string]any{"type": "object", "additionalProperties": false, "required": []string{"match_semantic"}, "properties": map[string]any{
					"literal_text":        map[string]any{"type": "string", "maxLength": 4096},
					"match_semantic":      map[string]any{"type": "string", "enum": []string{"EXACT_VALUE", "EXACT_PHRASE", "PHRASE_CONTAINS", "TOKEN_SEARCH", "SOURCE", "TIME_RANGE", "SOURCE_TIME"}},
					"text_representation": map[string]any{"type": "string", "enum": []string{"", "raw", "roman_derivative"}},
				}},
				"date_from": map[string]any{"type": "string"}, "date_to": map[string]any{"type": "string"}, "direction": map[string]any{"type": "string", "enum": directionValues},
			}},
		},
	}
}
