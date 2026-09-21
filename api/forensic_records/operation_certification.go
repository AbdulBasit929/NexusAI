package main

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
)

const operationCertificationContractV1 = "forensics.operation-certification/v1"

const (
	certificationLevelRegistered         = "REGISTERED"
	certificationLevelSourceValidated    = "SOURCE_VALIDATED"
	certificationLevelFixtureCertified   = "FIXTURE_CERTIFIED"
	certificationLevelRealWorldCertified = "REAL_WORLD_CERTIFIED"
	certificationLevelProductCertified   = "PRODUCT_CERTIFIED"
)

//go:embed contracts/operation-certification-v1.json
var operationCertificationJSON []byte

type OperationCertificationEntryV1 struct {
	OperationID            string         `json:"operation_id"`
	CertificationLevel     string         `json:"certification_level"`
	CriticalityTier        string         `json:"criticality_tier"`
	ExposureStatus         string         `json:"exposure_status"`
	SuggestionEligible     bool           `json:"suggestion_eligible"`
	SecurityPass           bool           `json:"security_pass"`
	KnownCorrectnessDefect bool           `json:"known_correctness_defect"`
	DebtDisposition        string         `json:"debt_disposition,omitempty"`
	CertificationBasis     []string       `json:"certification_basis"`
	Family                 string         `json:"family"`
	Intent                 string         `json:"intent"`
	AnalystQuestion        string         `json:"analyst_question"`
	SupportedInputShapes   []string       `json:"supported_input_shapes"`
	RequiredParameters     []string       `json:"required_parameters"`
	OptionalParameters     []string       `json:"optional_parameters"`
	NormalizationRules     []string       `json:"normalization_rules"`
	DataPrerequisites      []string       `json:"data_prerequisites"`
	CalculationDefinition  string         `json:"calculation_definition"`
	GroupingSemantics      []string       `json:"grouping_semantics"`
	FilterSemantics        []string       `json:"filter_semantics"`
	TimeSemantics          string         `json:"time_semantics"`
	DuplicateHandling      string         `json:"duplicate_handling"`
	SentinelHandling       string         `json:"sentinel_handling"`
	NullHandling           string         `json:"null_handling"`
	SortingRankingRules    string         `json:"sorting_ranking_rules"`
	TieBehavior            string         `json:"tie_behavior"`
	ResultLimits           string         `json:"result_limits"`
	ExpectedResultContract string         `json:"expected_result_contract"`
	PresentationContract   string         `json:"presentation_contract"`
	CitationPolicy         string         `json:"citation_policy"`
	KnownLimitations       []string       `json:"known_limitations"`
	GoldenFixture          string         `json:"golden_fixture"`
	GoldenOracle           string         `json:"golden_oracle"`
	CanonicalQuery         string         `json:"canonical_query"`
	QueryVariants          []string       `json:"query_variants"`
	ExpectedUnderstanding  map[string]any `json:"expected_understanding"`
	ExpectedParameters     map[string]any `json:"expected_parameters"`
	ExpectedResult         any            `json:"expected_result"`
	ActualUnderstanding    map[string]any `json:"actual_understanding"`
	ActualParameters       map[string]any `json:"actual_parameters"`
	ActualResult           any            `json:"actual_result"`
	CalculationPass        bool           `json:"calculation_pass"`
	RoutingPass            bool           `json:"routing_pass"`
	ParameterPass          bool           `json:"parameter_pass"`
	CitationPass           bool           `json:"citation_pass"`
	PresentationPass       bool           `json:"presentation_pass"`
	OverallStatus          string         `json:"overall_status"`
	IssueIDs               []string       `json:"issue_ids"`
}

type OperationCertificationMatrixV1 struct {
	ContractVersion string                          `json:"contract_version"`
	GeneratedFrom   string                          `json:"generated_from"`
	Entries         []OperationCertificationEntryV1 `json:"entries"`
}

func loadOperationCertificationMatrix() (OperationCertificationMatrixV1, error) {
	var matrix OperationCertificationMatrixV1
	if err := json.Unmarshal(operationCertificationJSON, &matrix); err != nil {
		return matrix, err
	}
	if matrix.ContractVersion != operationCertificationContractV1 {
		return matrix, fmt.Errorf("unsupported operation certification contract %q", matrix.ContractVersion)
	}
	return matrix, nil
}

func validateOperationCertificationMatrix(matrix OperationCertificationMatrixV1, templates []queryTemplateCatalogEntry) error {
	if matrix.ContractVersion != operationCertificationContractV1 {
		return errors.New("invalid operation certification contract")
	}
	byID := make(map[string]OperationCertificationEntryV1, len(matrix.Entries))
	for _, entry := range matrix.Entries {
		if entry.OperationID == "" || entry.Family == "" || entry.Intent == "" || entry.AnalystQuestion == "" || entry.CalculationDefinition == "" || entry.ExpectedResultContract == "" || entry.PresentationContract == "" || entry.CitationPolicy == "" || entry.CanonicalQuery == "" {
			return fmt.Errorf("operation %q has an incomplete correctness contract", entry.OperationID)
		}
		if !containsString([]string{"A", "B", "C"}, entry.CriticalityTier) {
			return fmt.Errorf("operation %q has invalid criticality tier %q", entry.OperationID, entry.CriticalityTier)
		}
		if !containsString([]string{
			certificationLevelRegistered,
			certificationLevelSourceValidated,
			certificationLevelFixtureCertified,
			certificationLevelRealWorldCertified,
			certificationLevelProductCertified,
		}, entry.CertificationLevel) {
			return fmt.Errorf("operation %q has invalid certification level %q", entry.OperationID, entry.CertificationLevel)
		}
		if !containsString([]string{"queryable", "limited", "engineering_only"}, entry.ExposureStatus) {
			return fmt.Errorf("operation %q has invalid exposure status %q", entry.OperationID, entry.ExposureStatus)
		}
		if entry.KnownCorrectnessDefect && entry.ExposureStatus == "queryable" {
			return fmt.Errorf("operation %q has a known correctness defect but remains queryable", entry.OperationID)
		}
		if entry.SuggestionEligible && entry.CertificationLevel != certificationLevelProductCertified {
			return fmt.Errorf("operation %q is suggested without product certification", entry.OperationID)
		}
		if entry.CertificationLevel == certificationLevelProductCertified && entry.OverallStatus != "certified" && entry.OverallStatus != "certified_with_limitation" {
			return fmt.Errorf("operation %q claims product certification without complete independent certification", entry.OperationID)
		}
		if entry.OverallStatus == "bounded_uncertified" && entry.ExposureStatus == "queryable" {
			return fmt.Errorf("bounded operation %q is falsely exposed as fully queryable", entry.OperationID)
		}
		if _, exists := byID[entry.OperationID]; exists {
			return fmt.Errorf("duplicate operation certification %q", entry.OperationID)
		}
		switch entry.OverallStatus {
		case "certified", "certified_with_limitation":
			if entry.GoldenFixture == "" || entry.GoldenOracle == "" || entry.ExpectedResult == nil || !entry.RoutingPass || !entry.ParameterPass || !entry.CalculationPass || !entry.CitationPass || !entry.PresentationPass || !entry.SecurityPass || len(entry.CertificationBasis) == 0 {
				return fmt.Errorf("operation %q claims certification without complete independent proof", entry.OperationID)
			}
		case "bounded_uncertified", "blocked_correctness", "blocked_missing_fixture", "blocked_missing_data", "blocked_citation", "blocked_presentation", "deprecated":
			if entry.DebtDisposition == "" {
				return fmt.Errorf("operation %q has uncertified debt without an explicit disposition", entry.OperationID)
			}
		default:
			return fmt.Errorf("operation %q has unknown certification status %q", entry.OperationID, entry.OverallStatus)
		}
		byID[entry.OperationID] = entry
	}
	for _, entry := range matrix.Entries {
		if entry.CriticalityTier == "A" && entry.OverallStatus != "certified" && entry.OverallStatus != "certified_with_limitation" {
			return fmt.Errorf("Tier A operation %q is not independently certified", entry.OperationID)
		}
	}
	for _, template := range templates {
		entry, exists := byID[template.OperationID]
		if !exists {
			return fmt.Errorf("executable operation %q has no correctness contract; add independent golden, routing, citation, presentation, and scope fixtures before queryability", template.OperationID)
		}
		if entry.Family != template.FamilyID || entry.ExpectedUnderstanding["operation_id"] != template.OperationID || entry.ActualUnderstanding["operation_id"] != template.OperationID {
			return fmt.Errorf("operation %q certification no longer matches registry metadata", template.OperationID)
		}
		delete(byID, template.OperationID)
	}
	if len(byID) > 0 {
		unknown := make([]string, 0, len(byID))
		for id := range byID {
			unknown = append(unknown, id)
		}
		sort.Strings(unknown)
		return fmt.Errorf("certification contains non-executable operations: %s", strings.Join(unknown, ", "))
	}
	return nil
}

func operationCertificationByID() map[string]OperationCertificationEntryV1 {
	operationCertificationIndexCache.Do(func() {
		matrix, err := loadOperationCertificationMatrix()
		if err != nil {
			operationCertificationIndexCache.value = map[string]OperationCertificationEntryV1{}
			return
		}
		operationCertificationIndexCache.value = make(map[string]OperationCertificationEntryV1, len(matrix.Entries))
		for _, entry := range matrix.Entries {
			operationCertificationIndexCache.value[entry.OperationID] = entry
		}
	})
	return operationCertificationIndexCache.value
}

var operationCertificationIndexCache struct {
	sync.Once
	value map[string]OperationCertificationEntryV1
}
