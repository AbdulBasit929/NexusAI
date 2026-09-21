package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const dynamicQueryDevelopmentCorpusV1 = "forensics.dynamic-query-development-corpus/v1"

//go:embed contracts/dynamic-query-development-corpus-v1.json
var dynamicQueryDevelopmentCorpusJSON []byte

type dynamicQueryDevelopmentCaseV1 struct {
	ID                    string `json:"id"`
	Question              string `json:"question"`
	ScopeFamily           string `json:"scope_family,omitempty"`
	ExpectedExecutor      string `json:"expected_executor"`
	ExpectedOperation     string `json:"expected_operation,omitempty"`
	ExpectedFamily        string `json:"expected_family"`
	ExpectedGoal          string `json:"expected_goal"`
	ExpectedRetrievalMode string `json:"expected_retrieval_mode,omitempty"`
	ExpectedMeasure       string `json:"expected_measure,omitempty"`
	ExpectedPlanAggregate string `json:"expected_plan_aggregate,omitempty"`
	ExpectedGroupCount    *int   `json:"expected_group_count,omitempty"`
	ExpectedLimit         *int   `json:"expected_limit,omitempty"`
}

type dynamicQueryDevelopmentCorpusDocumentV1 struct {
	ContractVersion string                          `json:"contract_version"`
	OracleAuthority string                          `json:"oracle_authority"`
	Cases           []dynamicQueryDevelopmentCaseV1 `json:"cases"`
}

type dynamicQueryDevelopmentRowV1 struct {
	ID                    string `json:"id"`
	Question              string `json:"question"`
	Correct               bool   `json:"correct"`
	ExpectedExecutor      string `json:"expected_executor"`
	ActualExecutor        string `json:"actual_executor"`
	ExpectedOperation     string `json:"expected_operation,omitempty"`
	ActualOperation       string `json:"actual_operation,omitempty"`
	ExpectedFamily        string `json:"expected_family"`
	ActualFamily          string `json:"actual_family"`
	ExpectedGoal          string `json:"expected_goal"`
	ActualGoal            string `json:"actual_goal"`
	ExpectedRetrievalMode string `json:"expected_retrieval_mode,omitempty"`
	ActualRetrievalMode   string `json:"actual_retrieval_mode"`
	ReasonCode            string `json:"reason_code"`
	Mismatch              string `json:"mismatch,omitempty"`
}

type dynamicQueryDevelopmentReceiptV1 struct {
	ContractVersion string                         `json:"contract_version"`
	CorpusContract  string                         `json:"corpus_contract"`
	EvaluatedCount  int                            `json:"evaluated_count"`
	CorrectCount    int                            `json:"correct_count"`
	Accuracy        float64                        `json:"accuracy"`
	Rows            []dynamicQueryDevelopmentRowV1 `json:"rows"`
}

func loadDynamicQueryDevelopmentCorpus() (dynamicQueryDevelopmentCorpusDocumentV1, error) {
	var corpus dynamicQueryDevelopmentCorpusDocumentV1
	decoder := json.NewDecoder(bytes.NewReader(dynamicQueryDevelopmentCorpusJSON))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&corpus); err != nil {
		return corpus, err
	}
	if corpus.ContractVersion != dynamicQueryDevelopmentCorpusV1 || len(corpus.Cases) == 0 {
		return corpus, fmt.Errorf("invalid dynamic query development corpus")
	}
	return corpus, nil
}

func dynamicQueryDevelopmentRequest(testCase dynamicQueryDevelopmentCaseV1) hybridQueryRequest {
	req := hybridQueryRequest{
		TenantID: "development-tenant", UserID: "development-analyst", CollectionID: "development-case",
		Query: testCase.Question, QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)},
	}
	if testCase.ScopeFamily != "" {
		req.EvidenceID = "10000000-0000-4000-8000-000000000001"
		req.EvidenceVersionID = "20000000-0000-4000-8000-000000000001"
		req.QueryScope = queryEvidenceScope{
			Kind: string(EvidenceScopeSelected), EvidenceID: req.EvidenceID,
			EvidenceVersionID: req.EvidenceVersionID, SourceFamily: testCase.ScopeFamily,
		}
	}
	return req
}

func evaluateDynamicQueryDevelopmentCorpus() (dynamicQueryDevelopmentReceiptV1, error) {
	corpus, err := loadDynamicQueryDevelopmentCorpus()
	if err != nil {
		return dynamicQueryDevelopmentReceiptV1{}, err
	}
	_, catalog, _ := deterministicCompilerFixture()
	receipt := dynamicQueryDevelopmentReceiptV1{
		ContractVersion: "forensics.dynamic-query-development-evaluation/v1",
		CorpusContract:  corpus.ContractVersion, Rows: []dynamicQueryDevelopmentRowV1{},
	}
	for _, testCase := range corpus.Cases {
		req := dynamicQueryDevelopmentRequest(testCase)
		issuedCatalog := []FieldDescriptorV1(nil)
		if testCase.ScopeFamily == "generic" {
			issuedCatalog = catalog
		}
		result, compileErr := compileDeterministicSemanticRequest(context.Background(), config{}, req, issuedCatalog)
		if compileErr != nil {
			return receipt, fmt.Errorf("%s: %w", testCase.ID, compileErr)
		}
		row := dynamicQueryDevelopmentRowV1{
			ID: testCase.ID, Question: testCase.Question,
			ExpectedExecutor: testCase.ExpectedExecutor, ActualExecutor: result.Executor,
			ExpectedOperation: testCase.ExpectedOperation, ActualOperation: result.OperationID,
			ExpectedFamily: testCase.ExpectedFamily, ActualFamily: result.Frame.FamilyHint,
			ExpectedGoal: testCase.ExpectedGoal, ActualGoal: result.Frame.Goal,
			ExpectedRetrievalMode: testCase.ExpectedRetrievalMode, ActualRetrievalMode: result.Frame.RetrievalMode,
			ReasonCode: result.ReasonCode, Correct: true,
		}
		mismatches := []string{}
		compare := func(label string, expected any, actual any) {
			if fmt.Sprint(expected) != fmt.Sprint(actual) {
				mismatches = append(mismatches, fmt.Sprintf("%s expected=%v actual=%v", label, expected, actual))
			}
		}
		compare("executor", testCase.ExpectedExecutor, result.Executor)
		compare("family", testCase.ExpectedFamily, result.Frame.FamilyHint)
		compare("goal", testCase.ExpectedGoal, result.Frame.Goal)
		if testCase.ExpectedOperation != "" {
			compare("operation", testCase.ExpectedOperation, result.OperationID)
		}
		if testCase.ExpectedRetrievalMode != "" {
			compare("retrieval_mode", testCase.ExpectedRetrievalMode, result.Frame.RetrievalMode)
		}
		if testCase.ExpectedMeasure != "" {
			compare("measure", testCase.ExpectedMeasure, result.Frame.Measure)
		}
		if testCase.ExpectedPlanAggregate != "" {
			if result.DynamicPlan == nil || len(result.DynamicPlan.Measures) != 1 {
				mismatches = append(mismatches, "dynamic plan measure missing")
			} else {
				compare("plan_aggregate", testCase.ExpectedPlanAggregate, result.DynamicPlan.Measures[0].Op)
			}
		}
		if testCase.ExpectedGroupCount != nil {
			if result.DynamicPlan == nil {
				mismatches = append(mismatches, "dynamic plan missing for group count")
			} else {
				compare("group_count", *testCase.ExpectedGroupCount, len(result.DynamicPlan.GroupFields))
			}
		}
		if testCase.ExpectedLimit != nil {
			if result.DynamicPlan == nil {
				mismatches = append(mismatches, "dynamic plan missing for limit")
			} else {
				compare("limit", *testCase.ExpectedLimit, result.DynamicPlan.Limit)
			}
		}
		row.Correct = len(mismatches) == 0
		row.Mismatch = strings.Join(mismatches, "; ")
		receipt.Rows = append(receipt.Rows, row)
		receipt.EvaluatedCount++
		if row.Correct {
			receipt.CorrectCount++
		}
	}
	receipt.Accuracy = float64(receipt.CorrectCount) / float64(receipt.EvaluatedCount)
	return receipt, nil
}

var _ = Describe("NX-B2.1D fresh dynamic query corpus", func() {
	It("is independent of the consumed ledger and compiler source", func() {
		corpus, err := loadDynamicQueryDevelopmentCorpus()
		Expect(err).NotTo(HaveOccurred())
		ledger, err := loadQueryVariantLedger()
		Expect(err).NotTo(HaveOccurred())
		consumed := map[string]bool{}
		for _, entry := range ledger.Entries {
			consumed[strings.ToLower(strings.Join(strings.Fields(entry.Text), " "))] = true
		}
		seen := map[string]bool{}
		compilerText := []string{}
		for _, path := range []string{"semantic_frame.go", "operation_applicability.go", "deterministic_semantic_compiler.go", "semantic_candidate_ranking.go"} {
			raw, readErr := os.ReadFile(path)
			Expect(readErr).NotTo(HaveOccurred())
			compilerText = append(compilerText, strings.ToLower(string(raw)))
		}
		joinedCompiler := strings.Join(compilerText, "\n")
		for _, testCase := range corpus.Cases {
			normalized := strings.ToLower(strings.Join(strings.Fields(testCase.Question), " "))
			Expect(consumed).NotTo(HaveKey(normalized), testCase.ID)
			Expect(seen).NotTo(HaveKey(normalized), testCase.ID)
			seen[normalized] = true
			words := semanticWords.FindAllString(normalized, -1)
			for index := 0; index+7 <= len(words); index++ {
				Expect(joinedCompiler).NotTo(ContainSubstring(strings.Join(words[index:index+7], " ")), testCase.ID)
			}
		}
	})

	It("meets the fresh-corpus correctness and latency gate deterministically", func() {
		started := time.Now()
		first, err := evaluateDynamicQueryDevelopmentCorpus()
		Expect(err).NotTo(HaveOccurred())
		second, err := evaluateDynamicQueryDevelopmentCorpus()
		Expect(err).NotTo(HaveOccurred())
		firstJSON, err := json.MarshalIndent(first, "", "  ")
		Expect(err).NotTo(HaveOccurred())
		secondJSON, err := json.MarshalIndent(second, "", "  ")
		Expect(err).NotTo(HaveOccurred())
		Expect(bytes.Equal(firstJSON, secondJSON)).To(BeTrue())
		Expect(first.EvaluatedCount).To(Equal(20))
		Expect(first.CorrectCount).To(BeNumerically(">=", 18), "rows=%+v", first.Rows)
		Expect(first.Accuracy).To(BeNumerically(">=", 0.90))
		// Two complete 20-query passes include byte-determinism verification;
		// the five-second suite budget remains far below the two-second
		// per-request runtime target.
		Expect(time.Since(started)).To(BeNumerically("<", 5*time.Second))

		if output := os.Getenv("NXB21_DYNAMIC_QUERY_EVAL_OUTPUT"); output != "" {
			firstJSON = append(firstJSON, '\n')
			Expect(filepath.Clean(output)).To(Equal(output))
			Expect(os.MkdirAll(filepath.Dir(output), 0o755)).To(Succeed())
			Expect(os.WriteFile(output, firstJSON, 0o600)).To(Succeed())
		}
	})
})
