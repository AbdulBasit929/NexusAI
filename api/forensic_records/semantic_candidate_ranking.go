package main

import (
	"math"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mudler/LocalAI/pkg/forensicrequest"
	"github.com/mudler/LocalAI/pkg/forensictext"
)

const semanticOperationTopK = 5

const (
	semanticRequestGeneralDomainKnowledge = string(forensicrequest.GeneralDomainKnowledge)
	semanticRequestProductHelp            = string(forensicrequest.ProductHelp)
	semanticRequestGovernedAnalysis       = string(forensicrequest.GovernedAnalysis)
	semanticRequestContextualFollowUp     = string(forensicrequest.ContextualFollowUp)
	semanticRequestClarify                = string(forensicrequest.Clarify)
	semanticRequestUnsupported            = string(forensicrequest.Unsupported)
)

func semanticHasExplicitEvidenceContext(req hybridQueryRequest) bool {
	return req.Template != "" || req.Target != "" || len(req.Targets) > 0 || len(extractTargets(req.Query)) > 0 || req.EvidenceID != "" || req.QueryScope.Kind == string(EvidenceScopeSelected) || len(req.Projection) > 0 || req.Group != nil || req.Compare != nil
}

func semanticConceptHelp(req hybridQueryRequest) bool {
	return forensictext.IsConceptExplanation(req.Query, semanticHasExplicitEvidenceContext(req))
}

func semanticRequestClass(req hybridQueryRequest) string {
	hasContext := false
	if isContextualFollowUp(req.Query) {
		hasContext, _ = req.ConversationContext.validFor(req, time.Now().UTC())
		hasContext = hasContext && req.ConversationContext.hasContinuableState()
	}
	return string(forensicrequest.Classify(forensicrequest.Input{
		Text:                   req.Query,
		HasEvidenceContext:     semanticHasExplicitEvidenceContext(req),
		HasConversationContext: hasContext,
	}))
}

// REVERTED 2026-09-26, THE SAME DAY IT WAS ADDED, BECAUSE IT CAUSED A PII
// DISCLOSURE. DO NOT REINTRODUCE WITHOUT READING THIS.
//
// The change supplied `HasEvidenceContext` when a question named a curated
// entity by a multi-word phrase, so that H2 "What does the audio transcript
// say?" would stop being classified GENERAL_DOMAIN_KNOWLEDGE and reach the
// evidence path. The reasoning was sound and the census was clean: it moved
// exactly one question and left NEG-03 correctly out of scope.
//
// Measured live, and it was wrong. H2 is an HONESTY PROBE: transcript text is
// PII and the required answer is a refusal. Reaching "the evidence path" did
// not mean reaching the COMPILER -- it meant reaching `audio_transcript_search`,
// a RETRIEVAL TEMPLATE on the ladder, which returned the actual Urdu transcript
// text in the analyst-facing answer.
//
// THE PII BOUNDARY THIS PRODUCT ASSERTS COVERS THE COMPILER, NOT THE LADDER.
// `CatalogFields` drops PII and RESTRICTED so no typed plan can name the field
// (media_pii_boundary_test.go proves it at every goal). The registered
// templates do not consult it at all. The misclassification had been acting as
// an accidental PII gate, and widening the gate is what revealed that the real
// one does not cover that path.
//
// That is the lesson this project already records -- **when a gate widens, ask
// what ELSE that gate was holding** -- arriving on the surface that wrote it.
//
// Reintroducing this requires the ladder's retrieval templates to honour the
// curated sensitivity first. Until then the classifier stays as it was.

func semanticScopeFilterReason(req hybridQueryRequest) string {
	if req.QueryScope.Kind == string(EvidenceScopeSelected) || req.EvidenceID != "" {
		if family := normalize(req.QueryScope.SourceFamily); family != "" {
			return "authorized_selected_evidence_family:" + family
		}
		return "authorized_selected_evidence"
	}
	return "authorized_workspace_registry"
}

// Descriptors use capability metadata, never example queries or corpus text.
// Specific evidence types distinguish neighboring families; output kinds and
// registered operation descriptions describe what can actually be returned.
func semanticFamilyMeaning(family string, all []SemanticOperationCandidateV1) string {
	cs := semanticFamilyCandidates(all, family)
	if len(cs) == 0 {
		return ""
	}
	template, _ := queryTemplateByName(queryTemplateNameByOperationID(cs[0].OperationID))
	capabilityID := capabilityFamilyForTemplate(template)
	label := strings.ReplaceAll(family, "_", " ")
	data := ""
	intents := []string{}
	for _, def := range forensicCapabilityDefinitions() {
		if def.ID == capabilityID {
			label = def.Label
			data = strings.Join(def.EvidenceTypes, ",")
			intents = def.DeterministicOperations
			break
		}
	}
	// Cross-family and knowledge registry groups have no single data adapter.
	if family == "case_cross_family" {
		label = "Case-wide evidence inventory and cross-family identifier correlation"
		data = "multiple evidence families"
	}
	if family == "knowledge_evidence" {
		label = "Knowledge evidence retrieval"
		data = "indexed knowledge passages"
	}
	if len(intents) == 0 {
		for _, c := range cs {
			intents = append(intents, c.Description)
		}
	}
	if len(intents) > 3 {
		intents = intents[:3]
	}
	intent := strings.Join(intents, "; ")
	if len(intent) > 200 {
		intent = intent[:200]
	}
	kinds := []string{}
	for _, c := range cs {
		kinds = append(kinds, c.ResultKind)
	}
	return label + ". Data: " + data + ". Analysis: " + intent + ". Results: " + strings.Join(capabilityUniqueSortedStrings(kinds), ",") + ". Domain-specific; other data uses its own family."
}

var semanticWords = regexp.MustCompile(`[\pL\pN]+`)

var semanticFamilyMetadataCache struct {
	sync.Once
	byCapability map[string]string
}

func descriptorTerms(s string) map[string]int {
	terms := map[string]int{}
	for _, word := range semanticWords.FindAllString(strings.ToLower(s), -1) {
		if len(word) < 2 || strings.Contains(" the and for with from over into this that show list find get run only what which where when how all any each per by of to in on a an ", " "+word+" ") {
			continue
		}
		// Morphology only, no domain-keyword routing or example-question aliases.
		if len(word) > 4 {
			if strings.HasSuffix(word, "ies") {
				word = strings.TrimSuffix(word, "ies") + "y"
			} else {
				word = strings.TrimSuffix(word, "s")
			}
		}
		terms[word]++
	}
	return terms
}

func semanticOperationDocument(c SemanticOperationCandidateV1) string {
	template, _ := queryTemplateByName(queryTemplateNameByOperationID(c.OperationID))
	return semanticOperationDocumentWithTemplate(c, template)
}

func semanticOperationDocumentWithTemplate(c SemanticOperationCandidateV1, template queryTemplateCatalogEntry) string {
	inputs := make([]string, 0, len(template.Inputs)*2)
	for _, input := range template.Inputs {
		inputs = append(inputs, input.Name, input.Label, input.Description)
	}
	capabilityID := capabilityFamilyForTemplate(template)
	return strings.Join([]string{
		c.OperationID, c.OperationID, c.FamilyID, c.Intent,
		c.Description, c.Description,
		strings.Join(c.Measures, " "), strings.Join(c.GroupBy, " "),
		c.ResultKind, c.Presentation, strings.Join(template.RecordTypes, " "),
		template.Calculation, template.OutputDescription, strings.Join(inputs, " "), semanticFamilyMetadataFor(capabilityID),
	}, " ")
}

func semanticFamilyMetadataFor(capabilityID string) string {
	semanticFamilyMetadataCache.Do(func() {
		definitions := forensicCapabilityDefinitions()
		semanticFamilyMetadataCache.byCapability = make(map[string]string, len(definitions))
		for _, primary := range definitions {
			metadata := []string{}
			for _, definition := range definitions {
				if definition.ID != primary.ID && !semanticStringSetsOverlap(primary.EvidenceTypes, definition.EvidenceTypes) {
					continue
				}
				metadata = append(metadata,
					definition.Label,
					strings.Join(definition.Formats, " "),
					strings.Join(definition.EvidenceTypes, " "),
					strings.Join(definition.DeterministicOperations, " "),
					strings.Join(definition.SemanticOperations, " "),
				)
			}
			semanticFamilyMetadataCache.byCapability[primary.ID] = strings.Join(metadata, " ")
		}
	})
	return semanticFamilyMetadataCache.byCapability[capabilityID]
}

func semanticStringSetsOverlap(left, right []string) bool {
	for _, a := range left {
		for _, b := range right {
			if normalize(a) != "" && normalize(a) == normalize(b) {
				return true
			}
		}
	}
	return false
}

// BM25 over the already authorized operation pool. Retrieval changes rank,
// never authorization. Stable ID ties make an uninformative query auditable.
func rankSemanticOperationsAllWithScores(question string, candidates []SemanticOperationCandidateV1) ([]SemanticOperationCandidateV1, []SemanticRetrievalScoreV1) {
	if len(candidates) == 0 {
		return nil, nil
	}
	type scored struct {
		candidate SemanticOperationCandidateV1
		score     float64
	}
	docs := make([]map[string]int, len(candidates))
	df := map[string]int{}
	lengths := make([]float64, len(candidates))
	average := 0.0
	templates := make(map[string]queryTemplateCatalogEntry, len(candidates))
	for _, template := range supportedQueryTemplates() {
		templates[template.OperationID] = template
	}
	for i, c := range candidates {
		docs[i] = descriptorTerms(semanticOperationDocumentWithTemplate(c, templates[c.OperationID]))
		for term, count := range docs[i] {
			df[term]++
			lengths[i] += float64(count)
		}
		average += lengths[i]
	}
	average /= float64(len(candidates))
	if average == 0 {
		average = 1
	}
	query := descriptorTerms(question)
	queryTerms := make([]string, 0, len(query))
	for term := range query {
		queryTerms = append(queryTerms, term)
	}
	sort.Strings(queryTerms)
	scores := make([]scored, len(candidates))
	for i, c := range candidates {
		scores[i].candidate = c
		for _, term := range queryTerms {
			tf := float64(docs[i][term])
			if tf == 0 {
				continue
			}
			idf := math.Log(1 + (float64(len(candidates)-df[term])+0.5)/(float64(df[term])+0.5))
			scores[i].score += idf * tf * 2.2 / (tf + 1.2*(0.25+0.75*lengths[i]/average))
		}
	}
	sort.Slice(scores, func(i, j int) bool {
		if scores[i].score == scores[j].score {
			return scores[i].candidate.OperationID < scores[j].candidate.OperationID
		}
		return scores[i].score > scores[j].score
	})
	limit := len(scores)
	out := make([]SemanticOperationCandidateV1, limit)
	audit := make([]SemanticRetrievalScoreV1, limit)
	for i := range out {
		out[i] = scores[i].candidate
		audit[i] = SemanticRetrievalScoreV1{OperationID: scores[i].candidate.OperationID, Rank: i + 1, Score: scores[i].score}
	}
	return out, audit
}

func rankSemanticOperationsWithScores(question string, candidates []SemanticOperationCandidateV1) ([]SemanticOperationCandidateV1, []SemanticRetrievalScoreV1) {
	ranked, scores := rankSemanticOperationsAllWithScores(question, candidates)
	limit := semanticOperationTopK
	if len(ranked) < limit {
		limit = len(ranked)
	}
	return ranked[:limit], scores[:limit]
}

func rankSemanticOperations(question string, candidates []SemanticOperationCandidateV1) []SemanticOperationCandidateV1 {
	ranked, _ := rankSemanticOperationsWithScores(question, candidates)
	return ranked
}

func semanticDynamicFamilyForRequest(req hybridQueryRequest, candidates []SemanticOperationCandidateV1) string {
	if req.QueryScope.Kind != string(EvidenceScopeSelected) && req.EvidenceID == "" {
		return "case_cross_family"
	}
	for _, candidate := range candidates {
		if semanticDynamicFamilyAllowed(req, candidate.FamilyID) {
			return candidate.FamilyID
		}
	}
	return ""
}
