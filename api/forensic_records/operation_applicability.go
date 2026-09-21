package main

import (
	"sort"
	"strings"
)

const (
	operationApplicabilityContractV1 = "forensics.operation-applicability/v1"
	semanticCapabilitySnapshotV1     = "forensics.semantic-capability-snapshot/v1"
)

type OperationApplicabilityV1 struct {
	ContractVersion           string   `json:"contract_version"`
	OperationID               string   `json:"operation_id"`
	Family                    string   `json:"family"`
	AcceptedEvidenceFamilies  []string `json:"accepted_evidence_families"`
	IntentClass               string   `json:"intent_class"`
	Measures                  []string `json:"measures"`
	Aggregations              []string `json:"aggregations"`
	GroupingDimensions        []string `json:"grouping_dimensions"`
	OrderingSemantics         []string `json:"ordering_semantics"`
	RetrievalModes            []string `json:"retrieval_modes"`
	ResultKind                string   `json:"result_kind"`
	RequiredSemanticSlots     []string `json:"required_semantic_slots"`
	OptionalSlots             []string `json:"optional_slots"`
	ServerBindableSlots       []string `json:"server_bindable_slots"`
	UserSuppliedRequiredSlots []string `json:"user_supplied_required_slots"`
	DynamicEquivalent         bool     `json:"dynamic_equivalent"`
}

type IssuedFieldCapabilityV1 struct {
	FieldID           string   `json:"field_id"`
	SemanticType      string   `json:"semantic_type"`
	PrivacyState      string   `json:"privacy_state"`
	AllowedFilters    []string `json:"allowed_filters"`
	AllowedAggregates []string `json:"allowed_aggregates"`
	Projectable       bool     `json:"projectable"`
	Groupable         bool     `json:"groupable"`
	Sortable          bool     `json:"sortable"`
}

type SemanticFamilyCapabilityV1 struct {
	Family    string `json:"family"`
	Readiness string `json:"readiness"`
}

type SemanticCapabilitySnapshotV1 struct {
	ContractVersion string                       `json:"contract_version"`
	Families        []SemanticFamilyCapabilityV1 `json:"families"`
	IssuedFields    []IssuedFieldCapabilityV1    `json:"issued_fields"`
	RetrievalModes  []string                     `json:"retrieval_modes"`
	OutputForms     []string                     `json:"output_forms"`
	Limits          map[string]int               `json:"limits"`
	Operations      []OperationApplicabilityV1   `json:"operations"`
}

func operationAggregations(template queryTemplateCatalogEntry) []string {
	values := []string{}
	text := strings.ToLower(strings.Join(append(append([]string{}, template.Measures...), template.Calculation), " "))
	for _, item := range []struct {
		token string
		name  string
	}{
		{"count", "COUNT"}, {"sum", "SUM"}, {"total", "SUM"}, {"average", "AVG"},
		{"minimum", "MIN"}, {"shortest", "MIN"}, {"maximum", "MAX"}, {"longest", "MAX"},
	} {
		if strings.Contains(text, item.token) {
			values = append(values, item.name)
		}
	}
	return capabilityUniqueSortedStrings(values)
}

func operationRetrievalModes(template queryTemplateCatalogEntry) []string {
	if template.Name == "document_search" {
		return []string{semanticRetrievalExactTerm, semanticRetrievalExactPhrase, semanticRetrievalFullText, semanticRetrievalSemantic, semanticRetrievalHybrid, semanticRetrievalMetadata, semanticRetrievalSourceScoped}
	}
	switch template.Route {
	case "derived":
		return []string{semanticRetrievalExactTerm, semanticRetrievalExactPhrase, semanticRetrievalFullText, semanticRetrievalSourceScoped}
	case "kb":
		return []string{semanticRetrievalSemantic, semanticRetrievalHybrid, semanticRetrievalMetadata, semanticRetrievalSourceScoped}
	case "hybrid":
		return []string{semanticRetrievalExactTerm, semanticRetrievalSemantic, semanticRetrievalHybrid, semanticRetrievalMetadata, semanticRetrievalSourceScoped}
	default:
		return []string{semanticRetrievalNone}
	}
}

func operationOrderingSemantics(template queryTemplateCatalogEntry) []string {
	values := []string{"stable"}
	intent := string(durableIntentForTemplate(template))
	if intent == "rank" {
		values = append(values, "ascending", "descending", "top_k")
	}
	if intent == "timeline" {
		values = append(values, "chronological")
	}
	return values
}

func operationApplicability(template queryTemplateCatalogEntry) OperationApplicabilityV1 {
	required, optional, bindable, userRequired := []string{}, []string{}, []string{}, []string{}
	for _, input := range template.Inputs {
		if input.Required {
			required = append(required, input.Name)
			switch input.Name {
			case "evidence_id", "evidence_version_id", "date_from", "date_to", "direction", "limit":
				bindable = append(bindable, input.Name)
			default:
				userRequired = append(userRequired, input.Name)
			}
		} else {
			optional = append(optional, input.Name)
		}
	}
	accepted := []string{}
	for _, recordType := range template.RecordTypes {
		family := capabilityFamilyForRecordType(recordType)
		if family == "" {
			family = normalize(recordType)
		}
		accepted = append(accepted, family)
	}
	return OperationApplicabilityV1{
		ContractVersion: operationApplicabilityContractV1, OperationID: template.OperationID,
		Family: template.FamilyID, AcceptedEvidenceFamilies: capabilityUniqueSortedStrings(accepted),
		IntentClass: string(durableIntentForTemplate(template)), Measures: append([]string(nil), template.Measures...),
		Aggregations: operationAggregations(template), GroupingDimensions: append([]string(nil), template.GroupBy...),
		OrderingSemantics: operationOrderingSemantics(template), RetrievalModes: operationRetrievalModes(template),
		ResultKind: resultKindForTemplate(template), RequiredSemanticSlots: capabilityUniqueSortedStrings(required),
		OptionalSlots: capabilityUniqueSortedStrings(optional), ServerBindableSlots: capabilityUniqueSortedStrings(bindable),
		UserSuppliedRequiredSlots: capabilityUniqueSortedStrings(userRequired),
		DynamicEquivalent:         template.Route == "records" && template.Name != "canonical_records",
	}
}

func operationApplicabilityRegistry(req hybridQueryRequest) []OperationApplicabilityV1 {
	values := []OperationApplicabilityV1{}
	templatesByOperation := make(map[string]queryTemplateCatalogEntry)
	for _, template := range supportedQueryTemplates() {
		templatesByOperation[template.OperationID] = template
	}
	for _, candidate := range semanticOperationCandidates(req) {
		if template, ok := templatesByOperation[candidate.OperationID]; ok {
			values = append(values, operationApplicability(template))
		}
	}
	sort.Slice(values, func(i, j int) bool { return values[i].OperationID < values[j].OperationID })
	return values
}

func buildSemanticCapabilitySnapshot(req hybridQueryRequest, fields []FieldDescriptorV1) SemanticCapabilitySnapshotV1 {
	operations := operationApplicabilityRegistry(req)
	familySet, retrievalSet, outputSet := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, operation := range operations {
		familySet[operation.Family] = true
		outputSet[operation.ResultKind] = true
		for _, mode := range operation.RetrievalModes {
			retrievalSet[mode] = true
		}
	}
	families := make([]SemanticFamilyCapabilityV1, 0, len(familySet))
	for family := range familySet {
		families = append(families, SemanticFamilyCapabilityV1{Family: family, Readiness: "authorized_registry_available"})
	}
	sort.Slice(families, func(i, j int) bool { return families[i].Family < families[j].Family })
	issued := make([]IssuedFieldCapabilityV1, 0, len(fields))
	for _, field := range fields {
		issued = append(issued, IssuedFieldCapabilityV1{
			FieldID: field.FieldID, SemanticType: field.EffectiveType, PrivacyState: field.RedactionState,
			AllowedFilters: append([]string(nil), field.AllowedFilters...), AllowedAggregates: append([]string(nil), field.AllowedAggregates...),
			Projectable: field.Projectable, Groupable: field.Groupable, Sortable: field.Sortable,
		})
	}
	sort.Slice(issued, func(i, j int) bool { return issued[i].FieldID < issued[j].FieldID })
	keys := func(values map[string]bool) []string {
		out := make([]string, 0, len(values))
		for value := range values {
			out = append(out, value)
		}
		sort.Strings(out)
		return out
	}
	return SemanticCapabilitySnapshotV1{
		ContractVersion: semanticCapabilitySnapshotV1, Families: families, IssuedFields: issued,
		RetrievalModes: keys(retrievalSet), OutputForms: keys(outputSet), Operations: operations,
		Limits: map[string]int{"input_rows": sourceNativeInputRowLimit, "result_groups": sourceNativeGroupLimit, "ast_complexity": sourceNativeComplexityMax, "result_rows": maxHybridLimit},
	}
}
