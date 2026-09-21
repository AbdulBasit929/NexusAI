package main

import "testing"

func TestExecutableOperationCertificationRegistrationGate(t *testing.T) {
	matrix, err := loadOperationCertificationMatrix()
	if err != nil {
		t.Fatalf("load certification matrix: %v", err)
	}
	if err := validateOperationCertificationMatrix(matrix, supportedQueryTemplates()); err != nil {
		t.Fatal(err)
	}
	if len(matrix.Entries) != len(supportedQueryTemplates()) {
		t.Fatalf("certification has %d entries for %d executable operations", len(matrix.Entries), len(supportedQueryTemplates()))
	}
}

func TestCertificationCannotSelfDeclareWithoutIndependentOracle(t *testing.T) {
	matrix, err := loadOperationCertificationMatrix()
	if err != nil || len(matrix.Entries) == 0 {
		t.Fatalf("load certification matrix: %v", err)
	}
	unsafe := matrix
	unsafe.Entries = append([]OperationCertificationEntryV1(nil), matrix.Entries...)
	unsafe.Entries[0].OverallStatus = "certified"
	if err := validateOperationCertificationMatrix(unsafe, supportedQueryTemplates()); err == nil {
		t.Fatal("operation self-declared certified without independent golden proof")
	}
}

func TestRiskTierExposureAndSuggestionGate(t *testing.T) {
	matrix, err := loadOperationCertificationMatrix()
	if err != nil {
		t.Fatal(err)
	}
	tierCounts := map[string]int{}
	certifiedTierA := 0
	for _, entry := range matrix.Entries {
		tierCounts[entry.CriticalityTier]++
		if entry.CriticalityTier == "A" && (entry.OverallStatus == "certified" || entry.OverallStatus == "certified_with_limitation") {
			certifiedTierA++
		}
		if entry.SuggestionEligible && entry.ExposureStatus != "queryable" {
			t.Errorf("%s is suggested with exposure %s", entry.OperationID, entry.ExposureStatus)
		}
		if entry.SuggestionEligible && entry.CertificationLevel != certificationLevelProductCertified {
			t.Errorf("%s is suggested at certification level %s", entry.OperationID, entry.CertificationLevel)
		}
		if entry.OverallStatus == "bounded_uncertified" && (entry.DebtDisposition == "" || entry.KnownCorrectnessDefect) {
			t.Errorf("%s has unbounded or defective debt", entry.OperationID)
		}
	}
	if tierCounts["A"] != 5 || certifiedTierA != tierCounts["A"] || tierCounts["B"] != 63 || tierCounts["C"] != 11 {
		t.Fatalf("unexpected risk-tier closure: tiers=%v certifiedA=%d", tierCounts, certifiedTierA)
	}
	for _, template := range supportedQueryTemplates() {
		if template.CriticalityTier == "" || template.CertificationStatus == "" || template.ExposureStatus == "" {
			t.Errorf("template %s omits certification exposure metadata", template.OperationID)
		}
	}
}
