package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
)

const (
	deterministicSemanticCompilerContractV1 = "forensics.deterministic-semantic-compiler/v1"
	semanticExecutorRegistered              = "EXECUTE_REGISTERED"
	semanticExecutorDynamic                 = "EXECUTE_DYNAMIC_TYPED_PLAN"
	semanticExecutorClarify                 = "CLARIFY"
	semanticExecutorUnsupported             = "UNSUPPORTED"
)

type SemanticOperationResolutionScoreV1 struct {
	OperationID            string  `json:"operation_id"`
	Rank                   int     `json:"rank"`
	BM25Score              float64 `json:"bm25_score"`
	ExactMetadata          float64 `json:"exact_metadata"`
	FamilyCompatibility    float64 `json:"family_compatibility"`
	IntentCompatibility    float64 `json:"intent_compatibility"`
	GroupingCompatibility  float64 `json:"grouping_compatibility"`
	RetrievalCompatibility float64 `json:"retrieval_compatibility"`
	TimeCompatibility      float64 `json:"time_compatibility"`
	EmbeddingCosine        float64 `json:"embedding_cosine,omitempty"`
	EmbeddingScore         float64 `json:"embedding_score,omitempty"`
	TotalScore             float64 `json:"total_score"`
}

type SourceNativeFieldResolutionScoreV1 struct {
	Role            string  `json:"role"`
	FieldID         string  `json:"field_id"`
	Rank            int     `json:"rank"`
	ExactNameScore  float64 `json:"exact_name_score"`
	BM25Score       float64 `json:"bm25_score"`
	EmbeddingCosine float64 `json:"embedding_cosine,omitempty"`
	EmbeddingScore  float64 `json:"embedding_score,omitempty"`
	TotalScore      float64 `json:"total_score"`
}

type DeterministicSemanticCompilerResultV1 struct {
	ContractVersion  string                               `json:"contract_version"`
	Frame            SemanticFrameV1                      `json:"frame"`
	Capability       SemanticCapabilitySnapshotV1         `json:"capability_snapshot"`
	RankedOperations []SemanticOperationResolutionScoreV1 `json:"ranked_operations"`
	RankedFields     []SourceNativeFieldResolutionScoreV1 `json:"ranked_fields,omitempty"`
	EmbeddingAudit   *SemanticEmbeddingResolutionAuditV1  `json:"embedding_audit,omitempty"`
	Executor         string                               `json:"executor"`
	OperationID      string                               `json:"operation_id,omitempty"`
	DynamicPlan      *SourceNativePlanV1                  `json:"dynamic_plan,omitempty"`
	AbsoluteScore    float64                              `json:"absolute_score"`
	Top1Top2Margin   float64                              `json:"top1_top2_margin"`
	Confidence       float64                              `json:"confidence"`
	ReasonCode       string                               `json:"reason_code"`
	Degradations     []string                             `json:"degradations"`
}

var explicitTemplateNamePattern = regexp.MustCompile(`(?i)\btemplate\s*=\s*([a-z][a-z0-9_]*)\b`)

func semanticStem(term string) string {
	term = strings.ToLower(strings.TrimSpace(term))
	switch {
	case strings.HasSuffix(term, "iness") && len(term) > 6:
		return strings.TrimSuffix(term, "iness") + "y"
	case strings.HasSuffix(term, "ies") && len(term) > 4:
		return strings.TrimSuffix(term, "ies") + "y"
	case (strings.HasSuffix(term, "ses") || strings.HasSuffix(term, "xes") || strings.HasSuffix(term, "zes") || strings.HasSuffix(term, "ches") || strings.HasSuffix(term, "shes")) && len(term) > 4:
		return strings.TrimSuffix(term, "es")
	case strings.HasSuffix(term, "ed") && len(term) > 5:
		return strings.TrimSuffix(term, "ed")
	case strings.HasSuffix(term, "s") && !strings.HasSuffix(term, "ss") && len(term) > 4:
		return strings.TrimSuffix(term, "s")
	default:
		return term
	}
}

func semanticMetadataTerms(value string) map[string]bool {
	terms := map[string]bool{}
	for _, term := range semanticWords.FindAllString(strings.ToLower(value), -1) {
		terms[semanticStem(term)] = true
	}
	return terms
}

func semanticFamilyCompatible(frameFamily, operationFamily string) bool {
	if frameFamily == "" || frameFamily == "none" {
		return false
	}
	if frameFamily == operationFamily {
		return true
	}
	switch frameFamily {
	case "cross_family":
		return operationFamily == "case_cross_family"
	case "device_sim":
		return operationFamily == "communications_cdr" || operationFamily == "subscriber_identity"
	case "document_intelligence":
		return operationFamily == "knowledge_evidence"
	}
	return false
}

func semanticGoalMatchesIntent(goal, intent string) bool {
	switch goal {
	case "readiness":
		return intent == "readiness"
	case "rank":
		// Ranking an aggregate (for example, cameras by observation count) is
		// still covered by an aggregate operation when it exposes the requested
		// grouping and deterministic ordering semantics.
		return intent == "rank" || intent == "aggregate"
	case "aggregate", "distinct":
		return intent == "aggregate" || intent == "summarize"
	case "summarize":
		// Symmetric with the "rank" case above: "summarize repeated
		// communications between counterparties" is legitimately answered by
		// a rank operation (e.g. cdr.frequent_contacts) that exposes the
		// requested grouping, not only a strict aggregate/summarize op.
		return intent == "summarize" || intent == "aggregate" || intent == "rank"
	case "compare":
		return intent == "compare" || intent == "correlate" || intent == "relationship"
	case "timeline":
		return intent == "timeline"
	case "search":
		return intent == "retrieve" || intent == "search"
	case "extract":
		return intent == "extract" || intent == "retrieve"
	case "source_rows":
		return intent == "inspect"
	case "existence":
		return intent == "lookup" || intent == "inspect"
	default:
		return false
	}
}

func semanticGroupingAliasTerms(value string) map[string]bool {
	terms := semanticMetadataTerms(value)
	aliases := map[string][]string{
		"counterparty": {"contact", "caller", "callee", "party", "called"},
		"camera":       {"capture", "point"},
		"source":       {"file", "dataset"},
		"hour":         {"hourly"},
		"day":          {"daily"},
		"week":         {"weekly"},
		"month":        {"monthly"},
		"status":       {"active", "inactive", "suspended", "review"},
	}
	for canonical, values := range aliases {
		if terms[canonical] {
			for _, alias := range values {
				terms[alias] = true
			}
		}
		for _, alias := range values {
			if terms[alias] {
				terms[canonical] = true
			}
		}
	}
	return terms
}

func semanticGroupingCompatible(hints, dimensions []string) bool {
	if len(hints) == 0 || len(dimensions) == 0 {
		return false
	}
	left := semanticGroupingAliasTerms(strings.Join(hints, " "))
	right := semanticGroupingAliasTerms(strings.Join(dimensions, " "))
	for term := range left {
		if right[term] {
			return true
		}
	}
	return false
}

func semanticRetrievalCompatible(mode string, modes []string) bool {
	if mode == "" || mode == semanticRetrievalNone {
		return false
	}
	return containsString(modes, mode)
}

func semanticExactMetadataScore(question string, template queryTemplateCatalogEntry) float64 {
	if match := explicitTemplateNamePattern.FindStringSubmatch(question); len(match) > 1 && strings.EqualFold(match[1], template.Name) {
		return 100
	}
	normalizedQuestion := semanticMetadataTerms(question)
	nameTerms := semanticMetadataTerms(template.Name)
	matched := 0
	for term := range nameTerms {
		if normalizedQuestion[term] {
			matched++
		}
	}
	if len(nameTerms) >= 2 && matched == len(nameTerms) {
		return 12
	}
	return 0
}

func rankSemanticOperationsForFrame(question string, frame SemanticFrameV1, candidates []SemanticOperationCandidateV1) ([]SemanticOperationCandidateV1, []SemanticOperationResolutionScoreV1) {
	ranked, baseScores := rankSemanticOperationsAllWithScores(question, candidates)
	baseByID := make(map[string]float64, len(baseScores))
	for _, score := range baseScores {
		baseByID[score.OperationID] = score.Score
	}
	templatesByOperation := make(map[string]queryTemplateCatalogEntry, len(candidates))
	for _, template := range supportedQueryTemplates() {
		templatesByOperation[template.OperationID] = template
	}
	type scored struct {
		candidate SemanticOperationCandidateV1
		score     SemanticOperationResolutionScoreV1
	}
	values := make([]scored, 0, len(ranked))
	for _, candidate := range ranked {
		template := templatesByOperation[candidate.OperationID]
		applicability := operationApplicability(template)
		score := SemanticOperationResolutionScoreV1{OperationID: candidate.OperationID, BM25Score: baseByID[candidate.OperationID]}
		score.ExactMetadata = semanticExactMetadataScore(question, template)
		if semanticFamilyCompatible(frame.FamilyHint, candidate.FamilyID) || frame.Goal == "source_rows" && candidate.FamilyID == "case_cross_family" {
			score.FamilyCompatibility = 6
		} else if frame.FamilyHint != "none" && frame.FamilyHint != "cross_family" {
			score.FamilyCompatibility = -1.5
		}
		if semanticGoalMatchesIntent(frame.Goal, applicability.IntentClass) {
			score.IntentCompatibility = 5
		}
		if semanticGroupingCompatible(frame.GroupByHints, applicability.GroupingDimensions) {
			score.GroupingCompatibility = 6
		}
		if semanticRetrievalCompatible(frame.RetrievalMode, applicability.RetrievalModes) {
			score.RetrievalCompatibility = 8
		}
		if frame.TimeIntent.Kind != "none" && applicability.IntentClass == "timeline" {
			score.TimeCompatibility = 3
		}
		score.TotalScore = score.BM25Score + score.ExactMetadata + score.FamilyCompatibility + score.IntentCompatibility + score.GroupingCompatibility + score.RetrievalCompatibility + score.TimeCompatibility
		values = append(values, scored{candidate: candidate, score: score})
	}
	sort.SliceStable(values, func(i, j int) bool {
		if values[i].score.TotalScore == values[j].score.TotalScore {
			return values[i].candidate.OperationID < values[j].candidate.OperationID
		}
		return values[i].score.TotalScore > values[j].score.TotalScore
	})
	outCandidates := make([]SemanticOperationCandidateV1, len(values))
	outScores := make([]SemanticOperationResolutionScoreV1, len(values))
	for index := range values {
		outCandidates[index] = values[index].candidate
		values[index].score.Rank = index + 1
		outScores[index] = values[index].score
	}
	return outCandidates, outScores
}

func deterministicRegisteredMatch(frame SemanticFrameV1, candidates []SemanticOperationCandidateV1, scores []SemanticOperationResolutionScoreV1, catalog []FieldDescriptorV1) (SemanticOperationCandidateV1, bool, float64) {
	if len(candidates) == 0 || len(scores) == 0 {
		return SemanticOperationCandidateV1{}, false, 0
	}
	top := candidates[0]
	if len(catalog) > 0 && frame.FamilyHint == "generic_tabular" && containsString([]string{"aggregate", "rank", "distinct", "existence"}, frame.Goal) {
		return SemanticOperationCandidateV1{}, false, 0
	}
	// Embeddings may reorder an authorized candidate pool, but they do not
	// establish deterministic execution confidence. Measure the confidence
	// margin from the independently inspectable lexical and frame signals.
	margin := scores[0].TotalScore - scores[0].EmbeddingScore
	if len(scores) > 1 {
		margin -= scores[1].TotalScore - scores[1].EmbeddingScore
	}
	if frame.Goal == "source_rows" && top.Intent == "inspect" && top.FamilyID == "case_cross_family" {
		return top, true, margin
	}
	// When the question implies a specific computation shape (a count, a
	// rank, a timeline, ...), the winning candidate must actually be that
	// shape of operation — a strong family or exact-name score is not a
	// substitute for that. Without this, a highly family/name-matched but
	// wrong-shaped listing operation (e.g. anpr.sightings, whose intent is
	// "lookup") could win a "how many ANPR sightings are there" question on
	// name/family strength alone, via any of the three branches below,
	// leaving the analyst with a row list where they asked for a count.
	// "lookup"/"none" goals have no defined intent mapping in
	// semanticGoalMatchesIntent (see its default case) and are exempt: their
	// IntentCompatibility is legitimately always 0, including for correct
	// matches like a plain plate or call-type lookup.
	if frame.Goal != "" && frame.Goal != "lookup" && frame.Goal != "none" && scores[0].IntentCompatibility <= 0 {
		return SemanticOperationCandidateV1{}, false, margin
	}
	thresholds, err := semanticCompilerThresholds()
	if err != nil {
		return SemanticOperationCandidateV1{}, false, margin
	}
	structural := scores[0].ExactMetadata + scores[0].FamilyCompatibility + scores[0].IntentCompatibility + scores[0].GroupingCompatibility + scores[0].RetrievalCompatibility + scores[0].TimeCompatibility
	if scores[0].ExactMetadata >= thresholds.ExactMetadataMinimum || structural >= thresholds.StructuralScoreMinimum && margin >= thresholds.Top1Top2MarginMinimum || structural >= thresholds.StructuralOverrideMinimum {
		return top, true, margin
	}
	return SemanticOperationCandidateV1{}, false, margin
}

func semanticFieldDescriptorText(field FieldDescriptorV1) string {
	return strings.Join([]string{
		field.NormalizedName, field.SourceName, strings.Join(field.SourceNames, " "),
		field.EffectiveType, strings.Join(field.AllowedFilters, " "), strings.Join(field.AllowedAggregates, " "),
		strings.Join(field.FamilyProvenance, " "),
	}, " ")
}

func sourceNativeFieldByHint(role, hint string, catalog []FieldDescriptorV1, predicate func(FieldDescriptorV1) bool, embeddingSimilarities map[string]float64) (FieldDescriptorV1, bool, []SourceNativeFieldResolutionScoreV1) {
	type score struct {
		field FieldDescriptorV1
		audit SourceNativeFieldResolutionScoreV1
	}
	values := []score{}
	for _, field := range catalog {
		if !predicate(field) {
			continue
		}
		values = append(values, score{field: field, audit: SourceNativeFieldResolutionScoreV1{Role: role, FieldID: field.FieldID}})
	}
	if len(values) == 0 {
		return FieldDescriptorV1{}, false, []SourceNativeFieldResolutionScoreV1{}
	}
	thresholds, err := semanticCompilerThresholds()
	if err != nil {
		return FieldDescriptorV1{}, false, []SourceNativeFieldResolutionScoreV1{}
	}
	queryTerms := descriptorTerms(hint)
	documents := make([]map[string]int, len(values))
	documentFrequency := map[string]int{}
	lengths := make([]float64, len(values))
	averageLength := 0.0
	for index := range values {
		documents[index] = descriptorTerms(semanticFieldDescriptorText(values[index].field))
		for term, count := range documents[index] {
			documentFrequency[term]++
			lengths[index] += float64(count)
		}
		averageLength += lengths[index]
	}
	averageLength /= float64(len(values))
	if averageLength == 0 {
		averageLength = 1
	}
	for index := range values {
		field := values[index].field
		normalizedHint := normalizeSourceNativeName(hint)
		if normalizedHint == field.NormalizedName || normalizedHint == normalizeSourceNativeName(field.SourceName) {
			values[index].audit.ExactNameScore = thresholds.FieldExactNameWeight
		} else {
			for _, sourceName := range field.SourceNames {
				if normalizedHint == normalizeSourceNativeName(sourceName) {
					values[index].audit.ExactNameScore = thresholds.FieldExactNameWeight
					break
				}
			}
		}
		for term := range queryTerms {
			termFrequency := float64(documents[index][term])
			if termFrequency == 0 {
				continue
			}
			inverseDocumentFrequency := math.Log(1 + (float64(len(values)-documentFrequency[term])+0.5)/(float64(documentFrequency[term])+0.5))
			values[index].audit.BM25Score += inverseDocumentFrequency * termFrequency * 2.2 / (termFrequency + 1.2*(0.25+0.75*lengths[index]/averageLength))
		}
		values[index].audit.BM25Score *= thresholds.FieldBM25Weight
		values[index].audit.EmbeddingCosine = embeddingSimilarities[field.FieldID]
		values[index].audit.EmbeddingScore = values[index].audit.EmbeddingCosine * thresholds.FieldEmbeddingWeight
		values[index].audit.TotalScore = values[index].audit.ExactNameScore + values[index].audit.BM25Score + values[index].audit.EmbeddingScore
	}
	sort.SliceStable(values, func(i, j int) bool {
		if values[i].audit.TotalScore == values[j].audit.TotalScore {
			return values[i].field.FieldID < values[j].field.FieldID
		}
		return values[i].audit.TotalScore > values[j].audit.TotalScore
	})
	audit := make([]SourceNativeFieldResolutionScoreV1, len(values))
	for index := range values {
		values[index].audit.Rank = index + 1
		audit[index] = values[index].audit
	}
	margin := values[0].audit.TotalScore
	if len(values) > 1 {
		margin -= values[1].audit.TotalScore
	}
	return values[0].field, values[0].audit.TotalScore >= thresholds.FieldAbsoluteScoreMinimum && margin >= thresholds.FieldTop1Top2MarginMinimum, audit
}

func sourceNativeAggregateForMeasure(measure string) string {
	switch measure {
	case "sum", "amount", "bytes", "duration":
		return "SUM"
	case "avg":
		return "AVG"
	case "min":
		return "MIN"
	case "max":
		return "MAX"
	default:
		return "COUNT"
	}
}

func compileDeterministicSourceNativePlan(frame SemanticFrameV1, question string, catalog []FieldDescriptorV1, embeddingSimilarities map[string]float64) (*SourceNativePlanV1, string, []SourceNativeFieldResolutionScoreV1) {
	fieldScores := []SourceNativeFieldResolutionScoreV1{}
	if len(catalog) == 0 {
		return nil, "FIELD_CATALOG_UNAVAILABLE", fieldScores
	}
	plan := &SourceNativePlanV1{ContractVersion: sourceNativePlanContractV1, Project: []string{}, Filters: []SourceNativeFilterV1{}, GroupFields: []string{}, Measures: []SourceNativeMeasureV1{}, Having: []SourceNativeHavingV1{}, Sort: []SourceNativeSortV1{}, Limit: frame.LimitHint}
	if plan.Limit == 0 {
		plan.Limit = defaultHybridLimit
	}
	if frame.Goal == "source_rows" || frame.Goal == "lookup" {
		for _, field := range catalog {
			if field.Projectable && len(plan.Project) < sourceNativeProjectLimit {
				plan.Project = append(plan.Project, field.FieldID)
			}
		}
		if len(plan.Project) == 0 {
			return nil, "NO_PROJECTABLE_FIELD", fieldScores
		}
		return plan, "COMPILED_SOURCE_ROWS", fieldScores
	}
	groupHint := strings.Join(frame.GroupByHints, " ")
	var group FieldDescriptorV1
	hasGroup := false
	// Grouping is only resolved when the question names a grouping dimension.
	// The embedding signal is question-level, so resolving an empty hint let
	// it pick an arbitrary groupable field (e.g. manual_review_required) for a
	// plain "how many" question — and only when the embedding runtime
	// happened to answer, making identical questions return different results.
	if strings.TrimSpace(groupHint) != "" {
		var scores []SourceNativeFieldResolutionScoreV1
		group, hasGroup, scores = sourceNativeFieldByHint("group", groupHint, catalog, func(field FieldDescriptorV1) bool { return field.Groupable }, embeddingSimilarities)
		fieldScores = append(fieldScores, scores...)
	}
	if !hasGroup && frame.Goal == "rank" {
		if match := semanticFrameWhichGroup.FindStringSubmatch(question); len(match) > 1 && strings.TrimSpace(match[1]) != "" {
			var scores []SourceNativeFieldResolutionScoreV1
			group, hasGroup, scores = sourceNativeFieldByHint("group", match[1], catalog, func(field FieldDescriptorV1) bool { return field.Groupable }, embeddingSimilarities)
			fieldScores = append(fieldScores, scores...)
		}
	}
	if hasGroup {
		plan.GroupFields = []string{group.FieldID}
	}
	aggregate := sourceNativeAggregateForMeasure(frame.Measure)
	measure := SourceNativeMeasureV1{MeasureID: "m1", Op: aggregate}
	if aggregate != "COUNT" || frame.MeasureFieldHint != "" {
		hint := frame.MeasureFieldHint
		if hint == "" {
			hint = question
		}
		field, ok, scores := sourceNativeFieldByHint("measure", hint, catalog, func(field FieldDescriptorV1) bool { return containsString(field.AllowedAggregates, aggregate) }, embeddingSimilarities)
		fieldScores = append(fieldScores, scores...)
		if !ok {
			return nil, "MEASURE_FIELD_UNRESOLVED", fieldScores
		}
		measure.FieldID = field.FieldID
	}
	plan.Measures = []SourceNativeMeasureV1{measure}
	if frame.Goal == "distinct" && !hasGroup {
		return nil, "DISTINCT_FIELD_UNRESOLVED", fieldScores
	}
	if frame.OrderIntent == "descending" || frame.Goal == "rank" {
		plan.Sort = []SourceNativeSortV1{{Target: "m1", Direction: "DESC"}}
		if frame.LimitHint == 0 {
			plan.Limit = 1
		}
	} else if frame.OrderIntent == "ascending" {
		plan.Sort = []SourceNativeSortV1{{Target: "m1", Direction: "ASC"}}
	}
	for _, hint := range frame.GroupByHints {
		if !containsString([]string{"hour", "day", "week", "month"}, hint) {
			continue
		}
		field, ok, scores := sourceNativeFieldByHint("time_bucket", question, catalog, func(field FieldDescriptorV1) bool {
			return field.EffectiveType == fieldTypeTimestamp || field.EffectiveType == fieldTypeDate
		}, embeddingSimilarities)
		fieldScores = append(fieldScores, scores...)
		if ok {
			plan.TimeBucket = &SourceNativeTimeBucketV1{FieldID: field.FieldID, Bucket: hint, Timezone: "Asia/Karachi"}
		}
		break
	}
	if err := validateSourceNativePlan(plan, catalog); err != nil {
		return nil, "PLAN_VALIDATION_REJECTED:" + err.Error(), fieldScores
	}
	return plan, "COMPILED_AGGREGATE", fieldScores
}

func compileDeterministicSemanticRequest(ctx context.Context, cfg config, req hybridQueryRequest, catalog []FieldDescriptorV1) (DeterministicSemanticCompilerResultV1, error) {
	frame := extractSemanticFrame(req)
	candidates := semanticOperationCandidates(req)
	ranked, scores := rankSemanticOperationsForFrame(req.Query, frame, candidates)
	operationEmbeddings, fieldEmbeddings, embeddingAudit, embeddingDegradation := semanticEmbeddingSignals(ctx, cfg, req.Query, candidates, catalog)
	ranked, scores = fuseSemanticOperationEmbeddings(ranked, scores, operationEmbeddings)
	result := DeterministicSemanticCompilerResultV1{
		ContractVersion: deterministicSemanticCompilerContractV1, Frame: frame,
		Capability: buildSemanticCapabilitySnapshot(req, catalog), RankedOperations: scores,
		EmbeddingAudit: embeddingAudit,
		Executor:       semanticExecutorClarify, ReasonCode: "AMBIGUOUS_SEMANTIC_FRAME", Degradations: []string{},
	}
	if embeddingDegradation != "" {
		result.Degradations = append(result.Degradations, embeddingDegradation)
	}
	if len(scores) > 0 {
		result.AbsoluteScore = scores[0].TotalScore
		result.Top1Top2Margin = scores[0].TotalScore
		if len(scores) > 1 {
			result.Top1Top2Margin -= scores[1].TotalScore
		}
	}
	if frame.RequestClass == semanticRequestProductHelp || frame.RequestClass == semanticRequestGeneralDomainKnowledge {
		result.Executor, result.ReasonCode = semanticExecutorUnsupported, "TERMINAL_REQUEST_CLASS"
		return result, nil
	}
	if frame.Confidence.Overall == 0 {
		result.Executor, result.ReasonCode = semanticExecutorClarify, "FRAME_EXTRACTION_DEGRADED"
		return result, nil
	}
	if selected, matched, _ := deterministicRegisteredMatch(frame, ranked, scores, catalog); matched {
		result.Executor, result.OperationID, result.ReasonCode = semanticExecutorRegistered, selected.OperationID, "REGISTERED_APPLICABILITY_MATCH"
		result.Confidence = 1
		return result, nil
	}
	if plan, reason, fieldScores := compileDeterministicSourceNativePlan(frame, req.Query, catalog, fieldEmbeddings); plan != nil {
		result.RankedFields = fieldScores
		result.Executor, result.DynamicPlan, result.ReasonCode = semanticExecutorDynamic, plan, reason
		result.Confidence = frame.Confidence.Overall
		return result, nil
	} else if reason != "FIELD_CATALOG_UNAVAILABLE" {
		result.RankedFields = fieldScores
		result.Degradations = append(result.Degradations, reason)
	}
	if frame.RequestClass == semanticRequestGovernedAnalysis || frame.RequestClass == semanticRequestContextualFollowUp {
		result.Executor, result.ReasonCode = semanticExecutorClarify, "UNRESOLVED_ISSUED_FIELD_OR_OPERATION"
		return result, nil
	}
	result.Executor, result.ReasonCode = semanticExecutorUnsupported, "NO_AUTHORIZED_ALGEBRA_OR_OPERATION"
	return result, nil
}

func applyDeterministicSemanticCompiler(ctx context.Context, cfg config, req hybridQueryRequest) (hybridQueryRequest, string) {
	catalog := []FieldDescriptorV1{}
	frame := extractSemanticFrame(req)
	candidates := semanticOperationCandidates(req)
	ranked, scores := rankSemanticOperationsForFrame(req.Query, frame, candidates)
	needsIssuedFields := frame.FamilyHint == "generic_tabular" && containsString([]string{"aggregate", "rank", "distinct", "existence"}, frame.Goal)
	if _, matched, _ := deterministicRegisteredMatch(frame, ranked, scores, nil); (!matched || needsIssuedFields) && cfg.QueryDB != nil {
		// A bounded sample is enough to learn field shape/type here; this only
		// decides whether a dynamic plan CAN be compiled. The actual answer,
		// if a plan is compiled, is always executed as real SQL over the full
		// authorized scope by executeSourceNativePlanSQL, never over this
		// sample — so a large collection must not fail catalog discovery.
		rows, err := sourceNativeCatalogSampleRows(ctx, cfg.QueryDB, req)
		if err != nil {
			return req, "field_catalog_unavailable"
		}
		catalog = buildSourceNativeFieldCatalog(req, rows)
	}
	result, err := compileDeterministicSemanticRequest(ctx, cfg, req, catalog)
	if err != nil {
		return req, "deterministic_compiler_error"
	}
	if req.SemanticPlannerAudit == nil {
		req.SemanticPlannerAudit = &SemanticPlannerAuditV1{}
	}
	req.SemanticPlannerAudit.SemanticFrame = &result.Frame
	req.SemanticPlannerAudit.CapabilitySnapshot = &result.Capability
	req.SemanticPlannerAudit.EmbeddingResolution = result.EmbeddingAudit
	req.SemanticPlannerAudit.OperationResolutionScores = result.RankedOperations
	req.SemanticPlannerAudit.SelectedDecision = result.Executor
	req.SemanticPlannerAudit.ResolutionMargin = result.Top1Top2Margin
	req.SemanticPlannerAudit.ResolutionScore = result.AbsoluteScore
	switch result.Executor {
	case semanticExecutorRegistered:
		for _, candidate := range candidates {
			if candidate.OperationID != result.OperationID {
				continue
			}
			resolved, applyErr := applySemanticOperation(req, candidate)
			if applyErr != nil {
				return req, "deterministic_registered_validation_rejected"
			}
			resolved.SemanticPlannerAudit = req.SemanticPlannerAudit
			resolved.SemanticPlannerAudit.Selected = &candidate
			resolved.SemanticPlannerAudit.BindingState, resolved.SemanticPlannerAudit.MissingFactKinds = semanticBindingReadiness(resolved, candidate)
			return resolved, "semantic_deterministic_registered"
		}
		return req, "deterministic_registered_operation_missing"
	case semanticExecutorDynamic:
		if result.DynamicPlan == nil {
			return req, "deterministic_dynamic_plan_missing"
		}
		bound := req
		bound.Template = "canonical_records"
		bound.SourceNative = result.DynamicPlan
		bound.SourceNativeCatalog = catalog
		bound.Group, bound.Compare, bound.Projection = nil, nil, nil
		bound.SemanticPlannerAudit = req.SemanticPlannerAudit
		bound.SemanticPlannerAudit.DynamicSelected = true
		bound.SemanticPlannerAudit.DynamicPlan = result.DynamicPlan
		return bound, "semantic_deterministic_dynamic"
	case semanticExecutorClarify:
		return req, "AMBIGUOUS_INTENT"
	case semanticExecutorUnsupported:
		return req, "UNSUPPORTED_REQUEST_CLASS"
	default:
		return req, "deterministic_compiler_error"
	}
}

func validateDeterministicCompilerResult(result DeterministicSemanticCompilerResultV1) error {
	if result.ContractVersion != deterministicSemanticCompilerContractV1 || result.Frame.ContractVersion != semanticFrameContractV1 || result.Capability.ContractVersion != semanticCapabilitySnapshotV1 {
		return errors.New("deterministic semantic compiler contract mismatch")
	}
	if !containsString([]string{semanticExecutorRegistered, semanticExecutorDynamic, semanticExecutorClarify, semanticExecutorUnsupported}, result.Executor) {
		return fmt.Errorf("unknown semantic executor %q", result.Executor)
	}
	if result.Executor == semanticExecutorRegistered && result.OperationID == "" {
		return errors.New("registered execution requires an operation")
	}
	if result.Executor == semanticExecutorDynamic && result.DynamicPlan == nil {
		return errors.New("dynamic execution requires a typed plan")
	}
	return nil
}
