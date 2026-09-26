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
	case "breakdown":
		// A breakdown is an aggregate that must keep its grouping. Registered
		// operations that already expose a grouping (intent "aggregate" or
		// "summarize") answer it; the obligation that the grouping SURVIVES is
		// enforced at S9, not here.
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
	if len(catalog) > 0 && frame.FamilyHint == "generic_tabular" && containsString([]string{"aggregate", "breakdown", "rank", "distinct", "existence"}, frame.Goal) {
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

// sourceNativeCountingSuperlative captures the object of a counting
// superlative: "the most calls", "the most internet sessions", "most often".
var sourceNativeCountingSuperlative = regexp.MustCompile(`(?i)\b(?:most|least|fewest)\s+((?:[a-z]+\s+){0,2}[a-z]+)\b`)

// sourceNativeRecordNouns are plural nouns that name ROWS rather than a
// measurable quantity. "The most calls" asks how many rows, not the maximum of
// any field. "Often" and "frequently" are included because "most often" is
// always a frequency, which is a row count.
var sourceNativeRecordNouns = map[string]bool{
	"call": true, "calls": true, "record": true, "records": true,
	"transaction": true, "transactions": true, "session": true, "sessions": true,
	"request": true, "requests": true, "sighting": true, "sightings": true,
	"event": true, "events": true, "entry": true, "entries": true,
	"message": true, "messages": true, "log": true, "logs": true,
	"contact": true, "contacts": true, "connection": true, "connections": true,
	"often": true, "frequently": true, "times": true,
}

// sourceNativeMagnitudeSuperlativePattern matches superlatives that rank by
// the SIZE of a value rather than by how many rows there are.
var sourceNativeMagnitudeSuperlativePattern = regexp.MustCompile(`(?i)\b(largest|biggest|greatest|highest|longest|smallest|lowest|shortest)\b`)

// sourceNativeMagnitudeSuperlative maps "the largest transaction" onto MAX and
// "the shortest call" onto MIN. semanticFrameMeasure does not recognise these
// words at all, so they fell through to the default and "what was the largest
// transaction" was answered with a COUNT.
func sourceNativeMagnitudeSuperlative(question string) (string, bool) {
	match := sourceNativeMagnitudeSuperlativePattern.FindStringSubmatch(question)
	if match == nil {
		return "", false
	}
	switch strings.ToLower(match[1]) {
	case "smallest", "lowest", "shortest":
		return "MIN", true
	}
	return "MAX", true
}

// sourceNativeRangeTimeField picks the field a date range is measured over.
// The curated normalised timestamp wins when the layer describes one; otherwise
// any issued TIMESTAMP that admits MIN and MAX will do. Never resolved from the
// question text — that is the whole-question embedding resolver D2 was fixed
// for.
func sourceNativeRangeTimeField(catalog []FieldDescriptorV1) (FieldDescriptorV1, bool) {
	bounded := func(field FieldDescriptorV1) bool {
		return (field.EffectiveType == fieldTypeTimestamp || field.EffectiveType == fieldTypeDate) &&
			containsString(field.AllowedAggregates, "MIN") && containsString(field.AllowedAggregates, "MAX")
	}
	for _, field := range catalog {
		if field.Curated && bounded(field) && strings.HasSuffix(field.FieldID, ".event_time") {
			return field, true
		}
	}
	for _, field := range catalog {
		if field.Curated && bounded(field) {
			return field, true
		}
	}
	for _, field := range catalog {
		if bounded(field) {
			return field, true
		}
	}
	return FieldDescriptorV1{}, false
}

// sourceNativeMeasureAggregate decides the aggregate deterministically and in
// a fixed order: a counting superlative beats a magnitude one ("the most
// transactions" counts rows even though "most" also reads as a maximum), and
// both beat the frame's coarse measure slot.
func sourceNativeMeasureAggregate(frame SemanticFrameV1, question string) string {
	if sourceNativeSuperlativeCountsRows(question) {
		return "COUNT"
	}
	// A magnitude superlative only decides the aggregate when the question
	// named none. Where an aggregate IS named (an average, a sum), a
	// superlative beside it expresses the ranking direction instead, and
	// reading it as a maximum would answer something else entirely.
	if frame.Measure == "none" {
		if aggregate, ok := sourceNativeMagnitudeSuperlative(question); ok {
			return aggregate
		}
	}
	return sourceNativeAggregateForMeasure(frame.Measure)
}

// sourceNativeSuperlativeCountsRows reports whether the question's superlative
// ranks by how MANY rows there are rather than by the size of some value.
func sourceNativeSuperlativeCountsRows(question string) bool {
	match := sourceNativeCountingSuperlative.FindStringSubmatch(question)
	if match == nil {
		return false
	}
	for _, word := range strings.Fields(strings.ToLower(match[1])) {
		if sourceNativeRecordNouns[word] {
			return true
		}
	}
	return false
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

// semanticConstraintUnboundPrefix marks a refusal caused by a literal the
// analyst supplied that nothing in the compiled request binds. S9
// CONSTRAINT_APPLIED: an unbound literal must abstain and name itself, never
// silently widen the answer to the whole case. That widening was defect D1 —
// "how many calls did <number> make", and even a number present in no record,
// returned a count of every row in the case.
const semanticConstraintUnboundPrefix = "CONSTRAINT_UNBOUND:"

// sourceNativeFilterOpForFrame maps the frame's extracted operator vocabulary
// onto the algebra's allowlist. An operator with no mapping is refused rather
// than downgraded to equality: a silently weakened comparison answers a
// different question than the one asked.
func sourceNativeFilterOpForFrame(op string) (string, bool) {
	switch strings.ToUpper(strings.TrimSpace(op)) {
	case "EQ":
		return "EQ", true
	case "NE", "NEQ":
		return "NEQ", true
	case "GT":
		return "GT", true
	case "GTE":
		return "GTE", true
	case "LT":
		return "LT", true
	case "LTE":
		return "LTE", true
	case "IN":
		return "IN", true
	case "CONTAINS":
		return "CONTAINS", true
	}
	return "", false
}

// sourceNativeFieldByExactName resolves a field hint against the authorized
// catalog by exact normalized name or declared source-name alias only.
//
// It deliberately accepts no embedding signal. The analyst named this field
// explicitly ("where call_type is SMS"), so there is nothing to infer; and a
// filter bound to a field the analyst did not name produces a confident answer
// to a different question, which is how D2 and the measure-hint defect behaved.
func sourceNativeFieldByExactName(hint string, catalog []FieldDescriptorV1) (FieldDescriptorV1, bool) {
	normalized := normalizeSourceNativeName(hint)
	if normalized == "" {
		return FieldDescriptorV1{}, false
	}
	for _, field := range catalog {
		if field.NormalizedName == normalized {
			return field, true
		}
	}
	for _, field := range catalog {
		for _, name := range field.SourceNames {
			if normalizeSourceNativeName(name) == normalized {
				return field, true
			}
		}
	}
	return FieldDescriptorV1{}, false
}

// compileSourceNativeFrameFilters turns every extracted payload filter into a
// typed plan filter. It returns a non-empty reason naming the first literal
// that cannot be bound, and the caller must then refuse rather than compile a
// plan that quietly ignores it.
func compileSourceNativeFrameFilters(frame SemanticFrameV1, catalog []FieldDescriptorV1) ([]SourceNativeFilterV1, string) {
	refuse := func(detail string) string { return semanticConstraintUnboundPrefix + detail }
	filters := []SourceNativeFilterV1{}
	for _, extracted := range frame.Filters {
		// Free-text retrieval belongs to the document path, not the source-row
		// algebra, and names no catalog field.
		if normalizeSourceNativeName(extracted.FieldHint) == "derived_text" {
			continue
		}
		literal := strings.TrimSpace(extracted.Literal)
		field, ok := sourceNativeFieldByExactName(extracted.FieldHint, catalog)
		if !ok {
			return nil, refuse("no authorized field named " + extracted.FieldHint + " for literal " + literal)
		}
		op, mapped := sourceNativeFilterOpForFrame(extracted.Operator)
		if !mapped || !containsString(field.AllowedFilters, op) {
			return nil, refuse("operator " + extracted.Operator + " is not allowed on " + extracted.FieldHint + " for literal " + literal)
		}
		filter := SourceNativeFilterV1{FieldID: field.FieldID, Op: op, Values: []string{}}
		if op == "IN" {
			for _, value := range extracted.Literals {
				if trimmed := strings.TrimSpace(value); trimmed != "" {
					filter.Values = append(filter.Values, trimmed)
				}
			}
			if len(filter.Values) == 0 {
				return nil, refuse("no values supplied for " + extracted.FieldHint)
			}
		} else {
			if literal == "" {
				return nil, refuse("empty literal for " + extracted.FieldHint)
			}
			filter.Value = literal
		}
		filters = append(filters, filter)
	}
	if len(filters) > sourceNativeFilterLimit {
		return nil, refuse("more extracted filters than the algebra permits")
	}
	return filters, ""
}

// sourceNativeDigits reduces an identifier to its digits so that formatting
// differences ("+92 300 1110001" vs "923001110001") do not read as a different
// number. Mirrors the normalization the executor applies in SQL.
func sourceNativeDigits(value string) string {
	return strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, value)
}

// deterministicUnboundConstraints returns every literal the analyst supplied
// that no part of the compiled request binds.
//
// This is the S9 CONSTRAINT_APPLIED check. Identifiers are satisfied by the
// request-level target constraint, which the executor applies against both
// party columns with digit normalization
// (sourceNativeRequestConstraintsSQL); they are deliberately NOT bound to a
// guessed payload field, because an inferred catalog cannot tell the
// subscriber's own number from the number they dialled, and picking wrong is
// the confident-wrong failure this work removes. Once the semantic layer
// carries descriptions and roles, that binding becomes safe to add.
func deterministicUnboundConstraints(req hybridQueryRequest, frame SemanticFrameV1, plan *SourceNativePlanV1) []string {
	unbound := []string{}
	boundTargets := map[string]bool{}
	for _, candidate := range append([]string{req.Target}, req.Targets...) {
		if digits := sourceNativeDigits(candidate); digits != "" {
			boundTargets[digits] = true
		}
		if trimmed := strings.TrimSpace(candidate); trimmed != "" {
			boundTargets[strings.ToLower(trimmed)] = true
		}
	}
	if plan != nil {
		for _, filter := range plan.Filters {
			for _, value := range append([]string{filter.Value}, filter.Values...) {
				if digits := sourceNativeDigits(value); digits != "" {
					boundTargets[digits] = true
				}
				if trimmed := strings.TrimSpace(value); trimmed != "" {
					boundTargets[strings.ToLower(trimmed)] = true
				}
			}
		}
	}
	for _, identifier := range frame.Identifiers {
		canonical := strings.TrimSpace(identifier.Canonical)
		if canonical == "" {
			canonical = strings.TrimSpace(identifier.Raw)
		}
		if canonical == "" {
			continue
		}
		if boundTargets[strings.ToLower(canonical)] {
			continue
		}
		if digits := sourceNativeDigits(canonical); digits != "" && boundTargets[digits] {
			continue
		}
		unbound = append(unbound, identifier.Type+" "+canonical)
	}
	// An explicit date range is an obligation exactly like an identifier: a
	// month that never reaches the request counts every month in the case.
	if frame.TimeIntent.Kind == "explicit_range" {
		if from := strings.TrimSpace(frame.TimeIntent.From); from != "" && strings.TrimSpace(req.DateFrom) == "" {
			unbound = append(unbound, "date range from "+from)
		}
		if to := strings.TrimSpace(frame.TimeIntent.To); to != "" && strings.TrimSpace(req.DateTo) == "" {
			unbound = append(unbound, "date range to "+to)
		}
	}
	return unbound
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
	// Every literal the analyst named becomes a typed filter before anything
	// else is decided. If one cannot bind, the whole plan is refused: a plan
	// that drops a constraint answers a broader question than was asked.
	boundFilters, unboundReason := compileSourceNativeFrameFilters(frame, catalog)
	if unboundReason != "" {
		return nil, unboundReason, fieldScores
	}
	// A value literal the curated layer declares ("active", "500", "SMS") is an
	// obligation exactly like an identifier. It carries no identifier pattern, so
	// literal extraction never saw it, the CONSTRAINT_APPLIED guard had nothing to
	// check, and the answer silently widened to the whole family — SUB-02 counted
	// all 11 subscribers instead of the 6 that are active. Only values the layer
	// declares can bind, so nothing is invented.
	plan.Filters = appendSourceNativeValueFilters(boundFilters, question, catalog)
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
	groupable := func(field FieldDescriptorV1) bool { return field.Groupable }
	// "Which X ..." names the grouping dimension grammatically, and X resolved
	// against the curated synonyms is the most reliable signal available — more
	// reliable than the general hint list, which for "which phone number made the
	// most calls" also captured "counterparty" and would group by the wrong party.
	if match := semanticFrameWhichGroup.FindStringSubmatch(question); len(match) > 1 {
		if field, ok := sourceNativeFieldByCuratedName(match[1], catalog, groupable); ok {
			group, hasGroup = field, true
		}
	}
	// Otherwise take the first hint that exactly names a curated field.
	if !hasGroup {
		for _, hint := range frame.GroupByHints {
			if field, ok := sourceNativeFieldByCuratedName(hint, catalog, groupable); ok {
				group, hasGroup = field, true
				break
			}
		}
	}
	// Grouping is only resolved when the question names a grouping dimension.
	// The embedding signal is question-level, so resolving an empty hint let
	// it pick an arbitrary groupable field (e.g. manual_review_required) for a
	// plain "how many" question — and only when the embedding runtime
	// happened to answer, making identical questions return different results.
	if !hasGroup && strings.TrimSpace(groupHint) != "" {
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
	// A question naming several values of one curated field is asking for a
	// breakdown by that field ("incoming versus outgoing"). The explicit group
	// hint still wins; this only supplies one where the phrasing carries no
	// "by X" at all.
	if !hasGroup {
		if fieldID, ok := semanticLayerGroupFieldForQuestion(question, catalog); ok {
			group, hasGroup = sourceNativeFieldMap(catalog)[fieldID], true
		}
	}
	if hasGroup {
		plan.GroupFields = []string{group.FieldID}
	}
	// "the most <record noun>" counts rows; it is not a maximum over a field.
	// semanticFrameMeasure maps "most" to max, which turned "which account has
	// the most transactions" into MAX(amount) — ranking accounts by value
	// instead of by how many transactions they have. The object of the
	// superlative is what decides, and it is decided lexically, never by
	// similarity.
	// A RANGE needs two measures over one time field. Everything below builds a
	// single measure, which is why a range question could only ever come back as
	// a row listing.
	if frame.Goal == "range" {
		field, ok := sourceNativeRangeTimeField(catalog)
		if !ok {
			return nil, "MEASURE_FIELD_UNRESOLVED:no time field to bound", fieldScores
		}
		plan.Measures = []SourceNativeMeasureV1{
			{MeasureID: "m1", Op: "MIN", FieldID: field.FieldID},
			{MeasureID: "m2", Op: "MAX", FieldID: field.FieldID},
		}
		plan.Project = []string{}
		return plan, "", fieldScores
	}
	aggregate := sourceNativeMeasureAggregate(frame, question)
	measure := SourceNativeMeasureV1{MeasureID: "m1", Op: aggregate}
	if aggregate != "COUNT" || frame.MeasureFieldHint != "" {
		hint := strings.TrimSpace(frame.MeasureFieldHint)
		if hint == "" {
			// Never resolve a measure field from the whole question. Doing so
			// ran the same question-level embedding resolver that D2 was fixed
			// for, and it picked an arbitrary numeric field — "what was the
			// largest transaction" became a count, and a count became a
			// maximum over an amount. With no explicit hint and no declared
			// default measure (that arrives with the semantic layer),
			// refusing is the only honest option.
			return nil, "MEASURE_FIELD_UNRESOLVED", fieldScores
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
	// S9 SHAPE. A plan can be perfectly valid and still answer a different
	// question than the one asked — a ranking with nothing to rank by returns a
	// real number for the wrong question, which is the least detectable failure
	// available to an analyst.
	if err := verifySourceNativePlanShape(frame, question, plan); err != nil {
		return nil, err.Error(), fieldScores
	}
	return plan, "COMPILED_AGGREGATE", fieldScores
}

func compileDeterministicSemanticRequest(ctx context.Context, cfg config, req hybridQueryRequest, catalog []FieldDescriptorV1) (DeterministicSemanticCompilerResultV1, error) {
	frame := extractSemanticFrame(req)
	candidates := semanticOperationCandidates(req)
	// D4: rank against the whole authorized pool so the scoring statistics stay
	// stable, then drop anything from a family the question did not ask about.
	ranked, scores := rankSemanticOperationsForFrame(req.Query, frame, candidates)
	ranked, scores, questionFamily := filterRankedByQuestionFamily(req.Query, ranked, scores)
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
	if questionFamily != "" && len(ranked) == 0 {
		// Audit record: the question named a family no authorized operation
		// serves. Kept so "why did it not use operation X" is answerable later.
		result.Degradations = append(result.Degradations, "NO_OPERATION_IN_QUESTION_FAMILY:"+questionFamily)
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
	// With no operation left in the requested family there is nothing to match
	// against; the typed plan below is built from the case's own catalog and is
	// family-correct by construction. Answering from another family is the
	// defect, not the fallback.
	registeredSelected, registeredMatched, _ := deterministicRegisteredMatch(frame, ranked, scores, catalog)
	// A curated value literal ("active", "500", "SMS") is a request to FILTER.
	// A registered operation produces its own fixed breakdown and cannot carry
	// that filter, which is why "how many subscribers are active" answered 11:
	// the template returned every subscriber grouped by status. When the layer
	// binds a value the analyst named, the typed plan — which can express the
	// filter — is the better answer, and the registered match stays as the
	// fallback if no plan compiles.
	preferTypedPlan := len(appendSourceNativeValueFilters(nil, req.Query, catalog)) > 0
	if registeredMatched && !preferTypedPlan {
		result.Executor, result.OperationID, result.ReasonCode = semanticExecutorRegistered, registeredSelected.OperationID, "REGISTERED_APPLICABILITY_MATCH"
		result.Confidence = 1
		return result, nil
	}
	if plan, reason, fieldScores := compileDeterministicSourceNativePlan(frame, req.Query, catalog, fieldEmbeddings); plan != nil {
		result.RankedFields = fieldScores
		// S9 CONSTRAINT_APPLIED. The plan is only usable if every literal the
		// analyst supplied is actually carried by it or by the request scope.
		if unbound := deterministicUnboundConstraints(req, frame, plan); len(unbound) > 0 {
			result.Executor = semanticExecutorClarify
			result.ReasonCode = semanticConstraintUnboundPrefix + strings.Join(unbound, "; ")
			return result, nil
		}
		result.Executor, result.DynamicPlan, result.ReasonCode = semanticExecutorDynamic, plan, reason
		result.Confidence = frame.Confidence.Overall
		return result, nil
	} else if strings.HasPrefix(reason, s9ShapeViolation) {
		// The plan would have answered a different shape than was asked for.
		// Clarifying names the mismatch; answering would hide it.
		result.RankedFields = fieldScores
		result.Executor, result.ReasonCode = semanticExecutorClarify, reason
		return result, nil
	} else if strings.HasPrefix(reason, semanticConstraintUnboundPrefix) {
		// Naming the literal that failed to bind is the whole point: the
		// analyst can only correct what the system tells them it could not use.
		result.RankedFields = fieldScores
		result.Executor, result.ReasonCode = semanticExecutorClarify, reason
		return result, nil
	} else if reason != "FIELD_CATALOG_UNAVAILABLE" {
		result.RankedFields = fieldScores
		result.Degradations = append(result.Degradations, reason)
	}
	// The typed plan did not compile. A registered operation that matched is
	// still better than a clarification, even when a value literal suggested a
	// filter: a broader true answer beats no answer at all.
	if registeredMatched {
		result.Executor, result.OperationID, result.ReasonCode = semanticExecutorRegistered, registeredSelected.OperationID, "REGISTERED_APPLICABILITY_MATCH"
		result.Confidence = 1
		return result, nil
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
	catalogSource := ""
	frame := extractSemanticFrame(req)
	candidates := semanticOperationCandidates(req)
	ranked, scores := rankSemanticOperationsForFrame(req.Query, frame, candidates)
	ranked, scores, _ = filterRankedByQuestionFamily(req.Query, ranked, scores)
	// A breakdown MUST reach the catalogue: without issued fields it has no
	// curated field to group BY, and S9 would then refuse the plan it forced.
	needsIssuedFields := frame.FamilyHint == "generic_tabular" && containsString([]string{"aggregate", "breakdown", "rank", "distinct", "existence"}, frame.Goal)
	// A question naming a curated value ("active", "status 500") needs the
	// catalogue even when a registered operation matches: the template returns a
	// fixed breakdown and cannot carry the filter that was asked for. Skipping
	// the catalogue here is why SUB-02 kept answering 11 instead of 6.
	namesCuratedValue := semanticLayerQuestionNamesValue(req.Query)
	if _, matched, _ := deterministicRegisteredMatch(frame, ranked, scores, nil); (!matched || needsIssuedFields || namesCuratedValue) && cfg.QueryDB != nil {
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
		// The curated layer describes what the inferred catalogue only observes:
		// aliases that make one concept one field, value domains, display names
		// and descriptions. It is merged in, never substituted, so a field
		// present in the data but absent from the YAML stays queryable.
		catalog, catalogSource = semanticLayerCatalogForRequest(req, req.Query, catalog)
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
	if catalogSource != "" {
		result.Degradations = append(result.Degradations, catalogSource)
	}
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
		// A typed plan over source rows answers a question about STRUCTURED
		// evidence. When the question names neither a family nor a record type,
		// it is not about structured rows at all and this executor has no
		// competence over it -- measured 2026-09-22, it answered "which image
		// says Stay Positive Work Hard" and "what time period does this case
		// cover" from canonical_records, both wrong.
		//
		// Naming a record type is enough, even without a family: "how many
		// emails are in this case" resolves record_type=email with no family and
		// correctly answers zero. Requiring a family would withhold that.
		if semanticQuestionFamily(req.Query) == "" && extractCanonicalRecordType(req.Query) == "" {
			return req, "UNSUPPORTED_REQUEST_CLASS"
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
