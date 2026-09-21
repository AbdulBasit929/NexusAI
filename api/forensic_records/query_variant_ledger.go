package main

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const queryVariantLedgerContractV1 = "forensics.query-variant-ledger/v1"

//go:embed contracts/query-variant-ledger-v1.json
var queryVariantLedgerJSON []byte

type QueryVariantLedgerEntryV1 struct {
	VariantID             string            `json:"variant_id"`
	Text                  string            `json:"text"`
	Language              string            `json:"language"`
	SourceCorpus          string            `json:"source_corpus"`
	CanonicalOperation    string            `json:"canonical_operation"`
	ExpectedIntent        string            `json:"expected_intent"`
	ExpectedFamily        string            `json:"expected_family"`
	ExpectedTarget        string            `json:"expected_target,omitempty"`
	ExpectedEntities      []string          `json:"expected_entities"`
	ExpectedTime          map[string]string `json:"expected_time"`
	ExpectedDirection     string            `json:"expected_direction,omitempty"`
	ExpectedFilters       map[string]string `json:"expected_filters"`
	FollowUpRequirements  map[string]string `json:"follow_up_requirements"`
	ClarificationRequired bool              `json:"clarification_required"`
	DuplicateOf           string            `json:"duplicate_of,omitempty"`
	RoutingPass           bool              `json:"routing_pass"`
	ParameterPass         bool              `json:"parameter_pass"`
	SemanticEquivalent    bool              `json:"semantic_equivalence_pass"`
	Status                string            `json:"status"`
}

type QueryVariantLedgerV1 struct {
	ContractVersion string                      `json:"contract_version"`
	GeneratedFrom   []string                    `json:"generated_from"`
	Entries         []QueryVariantLedgerEntryV1 `json:"entries"`
}

func loadQueryVariantLedger() (QueryVariantLedgerV1, error) {
	var ledger QueryVariantLedgerV1
	if err := json.Unmarshal(queryVariantLedgerJSON, &ledger); err != nil {
		return ledger, err
	}
	if ledger.ContractVersion != queryVariantLedgerContractV1 {
		return ledger, fmt.Errorf("unsupported query-variant ledger contract %q", ledger.ContractVersion)
	}
	return ledger, nil
}

func validateQueryVariantLedger(ledger QueryVariantLedgerV1, templates []queryTemplateCatalogEntry) error {
	if ledger.ContractVersion != queryVariantLedgerContractV1 || len(ledger.Entries) == 0 {
		return errors.New("invalid or empty query-variant ledger")
	}
	operations := make(map[string]queryTemplateCatalogEntry, len(templates))
	for _, template := range templates {
		operations[template.OperationID] = template
	}
	ids := map[string]struct{}{}
	firstByText := map[string]string{}
	canonicalCoverage := map[string]bool{}
	for _, entry := range ledger.Entries {
		if entry.VariantID == "" || strings.TrimSpace(entry.Text) == "" || entry.Language == "" || entry.SourceCorpus == "" || entry.CanonicalOperation == "" || entry.ExpectedIntent == "" || entry.ExpectedFamily == "" || entry.Status == "" {
			return fmt.Errorf("variant %q has an incomplete semantic contract", entry.VariantID)
		}
		if _, exists := ids[entry.VariantID]; exists {
			return fmt.Errorf("duplicate variant ID %q", entry.VariantID)
		}
		ids[entry.VariantID] = struct{}{}
		template, exists := operations[entry.CanonicalOperation]
		if !exists || template.FamilyID != entry.ExpectedFamily {
			return fmt.Errorf("variant %q references unknown or mismatched operation %q", entry.VariantID, entry.CanonicalOperation)
		}
		if entry.SourceCorpus == "operation_catalog" {
			canonicalCoverage[entry.CanonicalOperation] = true
		}
		if first, duplicate := firstByText[entry.Text]; duplicate {
			if entry.DuplicateOf != first {
				return fmt.Errorf("variant %q must declare duplicate_of %q", entry.VariantID, first)
			}
		} else {
			if entry.DuplicateOf != "" {
				return fmt.Errorf("variant %q declares a non-existent earlier duplicate", entry.VariantID)
			}
			firstByText[entry.Text] = entry.VariantID
		}
		if entry.Status == "pass" && (!entry.RoutingPass || !entry.ParameterPass || !entry.SemanticEquivalent) {
			return fmt.Errorf("variant %q claims PASS without routing, parameter, and semantic-equivalence proof", entry.VariantID)
		}
	}
	if len(canonicalCoverage) != len(operations) {
		return fmt.Errorf("canonical variant coverage is %d operations, want %d", len(canonicalCoverage), len(operations))
	}
	return nil
}
