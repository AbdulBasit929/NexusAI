package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	deterministicSemanticEvaluationContractV1 = "forensics.deterministic-semantic-evaluation/v1"
	deterministicSemanticBaselineScorerV1     = "bm25-operation-descriptor/v1"
)

type deterministicSemanticScorer interface {
	Name() string
	Rank(req hybridQueryRequest, candidates []SemanticOperationCandidateV1) ([]SemanticOperationCandidateV1, []SemanticRetrievalScoreV1)
}

type deterministicSemanticExpectedOperationAware interface {
	SetExpectedOperation(string)
}

type bm25OperationDescriptorScorer struct{}

func (bm25OperationDescriptorScorer) Name() string {
	return deterministicSemanticBaselineScorerV1
}

func (bm25OperationDescriptorScorer) Rank(req hybridQueryRequest, candidates []SemanticOperationCandidateV1) ([]SemanticOperationCandidateV1, []SemanticRetrievalScoreV1) {
	return rankSemanticOperationsAllWithScores(req.Query, candidates)
}

type semanticFrameOperationScorer struct{}

func (semanticFrameOperationScorer) Name() string {
	return "semantic-frame-applicability/v1"
}

func (semanticFrameOperationScorer) Rank(req hybridQueryRequest, candidates []SemanticOperationCandidateV1) ([]SemanticOperationCandidateV1, []SemanticRetrievalScoreV1) {
	frame := extractSemanticFrame(req)
	ranked, scores := rankSemanticOperationsForFrame(req.Query, frame, candidates)
	out := make([]SemanticRetrievalScoreV1, len(scores))
	for index, score := range scores {
		out[index] = SemanticRetrievalScoreV1{OperationID: score.OperationID, Rank: score.Rank, Score: score.TotalScore}
	}
	return ranked, out
}

type semanticEmbeddingOperationScorer struct {
	ctx                  context.Context
	cfg                  config
	availableCount       int
	lexicalFallbackCount int
	expectedOperation    string
	confidentCount       int
	confidentCorrect     int
	confidentWrong       int
}

func (scorer *semanticEmbeddingOperationScorer) Name() string {
	return "semantic-frame-applicability+qwen3-embedding-0.6b/v1"
}

func (scorer *semanticEmbeddingOperationScorer) SetExpectedOperation(operationID string) {
	scorer.expectedOperation = operationID
}

func (scorer *semanticEmbeddingOperationScorer) Rank(req hybridQueryRequest, candidates []SemanticOperationCandidateV1) ([]SemanticOperationCandidateV1, []SemanticRetrievalScoreV1) {
	frame := extractSemanticFrame(req)
	ranked, scores := rankSemanticOperationsForFrame(req.Query, frame, candidates)
	similarities, _, audit, _ := semanticEmbeddingSignals(scorer.ctx, scorer.cfg, req.Query, candidates, nil)
	if audit != nil && audit.State == "AVAILABLE" {
		scorer.availableCount++
	} else {
		scorer.lexicalFallbackCount++
	}
	ranked, scores = fuseSemanticOperationEmbeddings(ranked, scores, similarities)
	if selected, matched, _ := deterministicRegisteredMatch(frame, ranked, scores, nil); matched {
		scorer.confidentCount++
		if selected.OperationID == scorer.expectedOperation {
			scorer.confidentCorrect++
		} else {
			scorer.confidentWrong++
		}
	}
	out := make([]SemanticRetrievalScoreV1, len(scores))
	for index, score := range scores {
		out[index] = SemanticRetrievalScoreV1{OperationID: score.OperationID, Rank: score.Rank, Score: score.TotalScore}
	}
	return ranked, out
}

type deterministicSemanticEvaluationRowV1 struct {
	VariantID           string                     `json:"variant_id"`
	ExpectedOperation   string                     `json:"expected_operation"`
	ExpectedFamily      string                     `json:"expected_family"`
	ExpectedTarget      string                     `json:"expected_target,omitempty"`
	RankedCandidates    []SemanticRetrievalScoreV1 `json:"ranked_candidates"`
	Top1Top2Margin      float64                    `json:"top1_top2_margin"`
	ExpectedRank        int                        `json:"expected_rank"`
	Top1Hit             bool                       `json:"top1_hit"`
	Top3Hit             bool                       `json:"top3_hit"`
	Top5Hit             bool                       `json:"top5_hit"`
	Top1MissCause       string                     `json:"top1_miss_cause,omitempty"`
	Top1MissExplanation string                     `json:"top1_miss_explanation,omitempty"`
}

type deterministicSemanticEvaluationReceiptV1 struct {
	ContractVersion        string                                 `json:"contract_version"`
	Scorer                 string                                 `json:"scorer"`
	LedgerContract         string                                 `json:"ledger_contract"`
	LedgerPath             string                                 `json:"ledger_path"`
	EligibleOperationCount int                                    `json:"eligible_operation_count"`
	IssuedFieldCount       int                                    `json:"issued_field_count"`
	EvaluatedCount         int                                    `json:"evaluated_count"`
	Top1                   int                                    `json:"top1"`
	Top3                   int                                    `json:"top3"`
	Top5                   int                                    `json:"top5"`
	Top1MissCount          int                                    `json:"top1_miss_count"`
	RuntimeBudgetMS        int                                    `json:"runtime_budget_ms"`
	Rows                   []deterministicSemanticEvaluationRowV1 `json:"rows"`
}

type semanticThresholdCurvePointV1 struct {
	ExactMetadataMinimum      float64 `json:"exact_metadata_minimum"`
	StructuralScoreMinimum    float64 `json:"structural_score_minimum"`
	Top1Top2MarginMinimum     float64 `json:"top1_top2_margin_minimum"`
	StructuralOverrideMinimum float64 `json:"structural_override_minimum"`
	ConfidentCount            int     `json:"confident_count"`
	CorrectCount              int     `json:"correct_count"`
	WrongCount                int     `json:"wrong_count"`
	Coverage                  float64 `json:"coverage"`
	Precision                 float64 `json:"precision"`
}

type semanticThresholdCalibrationReceiptV1 struct {
	ContractVersion string                          `json:"contract_version"`
	EvaluatedCount  int                             `json:"evaluated_count"`
	Selected        semanticThresholdCurvePointV1   `json:"selected"`
	Curve           []semanticThresholdCurvePointV1 `json:"curve"`
}

type semanticEmbeddingEvaluationReceiptV1 struct {
	ContractVersion     string                                   `json:"contract_version"`
	ModelID             string                                   `json:"model_id"`
	CallPath            string                                   `json:"call_path"`
	Before              deterministicSemanticEvaluationReceiptV1 `json:"before"`
	After               deterministicSemanticEvaluationReceiptV1 `json:"after"`
	EmbeddingAvailable  int                                      `json:"embedding_available_count"`
	LexicalFallback     int                                      `json:"lexical_fallback_count"`
	CorrectedVariantIDs []string                                 `json:"corrected_variant_ids"`
	RegressedVariantIDs []string                                 `json:"regressed_variant_ids"`
	QuestionTimeoutMS   int                                      `json:"question_timeout_ms"`
	RankingSignalOnly   bool                                     `json:"ranking_signal_only"`
	ConfidentBefore     semanticThresholdCurvePointV1            `json:"confident_before"`
	ConfidentAfter      semanticThresholdCurvePointV1            `json:"confident_after"`
}

func deterministicEvaluationFieldCatalog() []FieldDescriptorV1 {
	req := hybridQueryRequest{
		TenantID: "evaluation-tenant", CollectionID: "evaluation-case",
		EvidenceID:        "10000000-0000-4000-8000-000000000001",
		EvidenceVersionID: "20000000-0000-4000-8000-000000000001",
	}
	rows := []map[string]any{
		{"tenant_id": req.TenantID, "collection_id": req.CollectionID, "evidence_id": req.EvidenceID, "evidence_version_id": req.EvidenceVersionID, "record_type": "generic", "raw_payload": map[string]any{"department": "Sales", "invoice_total": 400, "event_time": "2026-09-01T10:00:00Z"}},
		{"tenant_id": req.TenantID, "collection_id": req.CollectionID, "evidence_id": req.EvidenceID, "evidence_version_id": req.EvidenceVersionID, "record_type": "generic", "raw_payload": map[string]any{"department": "Operations", "invoice_total": 150, "event_time": "2026-09-02T11:00:00Z"}},
	}
	return buildSourceNativeFieldCatalog(req, rows)
}

func classifyDeterministicTop1Miss(entry QueryVariantLedgerEntryV1, ranked []SemanticOperationCandidateV1, expectedRank int) (string, string) {
	if len(ranked) == 0 {
		return "INSUFFICIENT_OPERATION_METADATA", "the authorized candidate pool was empty"
	}
	top := ranked[0]
	if top.FamilyID != entry.ExpectedFamily {
		return "FAMILY_OR_SCOPE_UNCERTAINTY", fmt.Sprintf("top family %s differed from expected family %s", top.FamilyID, entry.ExpectedFamily)
	}
	if top.Intent == entry.ExpectedIntent {
		return "OVERLAPPING_REGISTERED_OPERATIONS", fmt.Sprintf("%s and %s share family %s and intent %s", top.OperationID, entry.CanonicalOperation, top.FamilyID, top.Intent)
	}
	if expectedRank == 0 || expectedRank > semanticOperationTopK {
		return "DESCRIPTOR_RANKING_DEFICIENCY", fmt.Sprintf("expected operation ranked %d against overlapping descriptor vocabulary", expectedRank)
	}
	return "INSUFFICIENT_OPERATION_METADATA", fmt.Sprintf("top intent %s displaced expected intent %s", top.Intent, entry.ExpectedIntent)
}

func buildDeterministicSemanticEvaluation(scorer deterministicSemanticScorer) (deterministicSemanticEvaluationReceiptV1, error) {
	ledger, err := loadQueryVariantLedger()
	if err != nil {
		return deterministicSemanticEvaluationReceiptV1{}, err
	}
	workspace := hybridQueryRequest{QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)}}
	receipt := deterministicSemanticEvaluationReceiptV1{
		ContractVersion: deterministicSemanticEvaluationContractV1,
		Scorer:          scorer.Name(), LedgerContract: ledger.ContractVersion,
		LedgerPath:             "api/forensic_records/contracts/query-variant-ledger-v1.json",
		EligibleOperationCount: len(semanticOperationCandidates(workspace)),
		IssuedFieldCount:       len(deterministicEvaluationFieldCatalog()), RuntimeBudgetMS: 60000,
		Rows: []deterministicSemanticEvaluationRowV1{},
	}
	for _, entry := range ledger.Entries {
		if entry.Language != "english" || entry.ClarificationRequired || len(entry.FollowUpRequirements) > 0 {
			continue
		}
		req, eligible := semanticRecallRequest(entry.CanonicalOperation, entry.Text)
		if !eligible {
			continue
		}
		if aware, ok := scorer.(deterministicSemanticExpectedOperationAware); ok {
			aware.SetExpectedOperation(entry.CanonicalOperation)
		}
		ranked, scores := scorer.Rank(req, semanticOperationCandidates(req))
		expectedRank := semanticOperationRank(ranked, entry.CanonicalOperation)
		row := deterministicSemanticEvaluationRowV1{
			VariantID: entry.VariantID, ExpectedOperation: entry.CanonicalOperation,
			ExpectedFamily: entry.ExpectedFamily, ExpectedTarget: entry.ExpectedTarget,
			RankedCandidates: scores, ExpectedRank: expectedRank,
			Top1Hit: expectedRank == 1, Top3Hit: expectedRank > 0 && expectedRank <= 3,
			Top5Hit: expectedRank > 0 && expectedRank <= 5,
		}
		if len(scores) > 1 {
			row.Top1Top2Margin = scores[0].Score - scores[1].Score
		}
		if !row.Top1Hit {
			row.Top1MissCause, row.Top1MissExplanation = classifyDeterministicTop1Miss(entry, ranked, expectedRank)
			receipt.Top1MissCount++
		}
		receipt.EvaluatedCount++
		if row.Top1Hit {
			receipt.Top1++
		}
		if row.Top3Hit {
			receipt.Top3++
		}
		if row.Top5Hit {
			receipt.Top5++
		}
		receipt.Rows = append(receipt.Rows, row)
	}
	return receipt, nil
}

func marshalDeterministicSemanticEvaluation(receipt deterministicSemanticEvaluationReceiptV1) ([]byte, error) {
	payload, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(payload, '\n'), nil
}

func buildSemanticThresholdCalibration() (semanticThresholdCalibrationReceiptV1, error) {
	ledger, err := loadQueryVariantLedger()
	if err != nil {
		return semanticThresholdCalibrationReceiptV1{}, err
	}
	type resolutionCase struct {
		correct    bool
		exact      float64
		structural float64
		margin     float64
	}
	cases := []resolutionCase{}
	for _, entry := range ledger.Entries {
		if entry.Language != "english" || entry.ClarificationRequired || len(entry.FollowUpRequirements) > 0 {
			continue
		}
		req, eligible := semanticRecallRequest(entry.CanonicalOperation, entry.Text)
		if !eligible {
			continue
		}
		frame := extractSemanticFrame(req)
		ranked, scores := rankSemanticOperationsForFrame(req.Query, frame, semanticOperationCandidates(req))
		if len(ranked) == 0 || len(scores) == 0 {
			continue
		}
		margin := scores[0].TotalScore
		if len(scores) > 1 {
			margin -= scores[1].TotalScore
		}
		structural := scores[0].ExactMetadata + scores[0].FamilyCompatibility + scores[0].IntentCompatibility + scores[0].GroupingCompatibility + scores[0].RetrievalCompatibility + scores[0].TimeCompatibility
		cases = append(cases, resolutionCase{correct: ranked[0].OperationID == entry.CanonicalOperation, exact: scores[0].ExactMetadata, structural: structural, margin: margin})
	}
	receipt := semanticThresholdCalibrationReceiptV1{ContractVersion: "forensics.semantic-threshold-calibration/v1", EvaluatedCount: len(cases), Curve: []semanticThresholdCurvePointV1{}}
	for _, structuralMinimum := range []float64{0, 4, 8, 10, 12, 14, 16, 18} {
		for _, marginMinimum := range []float64{0.5, 0.75, 1, 1.5, 2, 3, 5} {
			for _, overrideMinimum := range []float64{14, 16, 18, 20, 24, 100} {
				point := semanticThresholdCurvePointV1{ExactMetadataMinimum: 12, StructuralScoreMinimum: structuralMinimum, Top1Top2MarginMinimum: marginMinimum, StructuralOverrideMinimum: overrideMinimum}
				for _, item := range cases {
					confident := item.exact >= point.ExactMetadataMinimum || item.structural >= point.StructuralScoreMinimum && item.margin >= point.Top1Top2MarginMinimum || item.structural >= point.StructuralOverrideMinimum
					if !confident {
						continue
					}
					point.ConfidentCount++
					if item.correct {
						point.CorrectCount++
					} else {
						point.WrongCount++
					}
				}
				point.Coverage = float64(point.ConfidentCount) / float64(len(cases))
				if point.ConfidentCount > 0 {
					point.Precision = float64(point.CorrectCount) / float64(point.ConfidentCount)
				}
				receipt.Curve = append(receipt.Curve, point)
				if point.WrongCount == 0 && (point.ConfidentCount > receipt.Selected.ConfidentCount || point.ConfidentCount == receipt.Selected.ConfidentCount && point.Top1Top2MarginMinimum < receipt.Selected.Top1Top2MarginMinimum) {
					receipt.Selected = point
				}
			}
		}
	}
	return receipt, nil
}

var _ = Describe("NX-B2.1D deterministic semantic evaluation", func() {
	It("reproduces the frozen BM25 baseline offline and byte-deterministically", func() {
		started := time.Now()
		first, err := buildDeterministicSemanticEvaluation(bm25OperationDescriptorScorer{})
		Expect(err).NotTo(HaveOccurred())
		second, err := buildDeterministicSemanticEvaluation(bm25OperationDescriptorScorer{})
		Expect(err).NotTo(HaveOccurred())
		firstJSON, err := marshalDeterministicSemanticEvaluation(first)
		Expect(err).NotTo(HaveOccurred())
		secondJSON, err := marshalDeterministicSemanticEvaluation(second)
		Expect(err).NotTo(HaveOccurred())
		Expect(bytes.Equal(firstJSON, secondJSON)).To(BeTrue())
		Expect(first.EvaluatedCount).To(Equal(137))
		Expect(first.Top1).To(Equal(117))
		Expect(first.Top3).To(Equal(126))
		Expect(first.Top5).To(Equal(132))
		Expect(first.Top1MissCount).To(Equal(20))
		Expect(first.EligibleOperationCount).To(Equal(66))
		Expect(first.IssuedFieldCount).To(BeNumerically(">", 0))
		Expect(time.Since(started)).To(BeNumerically("<", 60*time.Second))

		if output := os.Getenv("NXB21_DETERMINISTIC_EVAL_OUTPUT"); output != "" {
			Expect(filepath.Clean(output)).To(Equal(output))
			Expect(os.MkdirAll(filepath.Dir(output), 0o755)).To(Succeed())
			Expect(os.WriteFile(output, firstJSON, 0o600)).To(Succeed())
		}
	})

	It("raises deterministic top-1 operation resolution above the D threshold", func() {
		receipt, err := buildDeterministicSemanticEvaluation(semanticFrameOperationScorer{})
		Expect(err).NotTo(HaveOccurred())
		Expect(receipt.EvaluatedCount).To(Equal(137))
		Expect(receipt.Top1).To(BeNumerically(">=", 127))
		Expect(receipt.Top5).To(BeNumerically(">=", 132))
		if output := os.Getenv("NXB21_SEMANTIC_FRAME_EVAL_OUTPUT"); output != "" {
			payload, marshalErr := marshalDeterministicSemanticEvaluation(receipt)
			Expect(marshalErr).NotTo(HaveOccurred())
			Expect(filepath.Clean(output)).To(Equal(output))
			Expect(os.MkdirAll(filepath.Dir(output), 0o755)).To(Succeed())
			Expect(os.WriteFile(output, payload, 0o600)).To(Succeed())
		}
	})

	It("calibrates confidence for zero wrong registered executions", func() {
		receipt, err := buildSemanticThresholdCalibration()
		Expect(err).NotTo(HaveOccurred())
		if output := os.Getenv("NXB21_THRESHOLD_CALIBRATION_OUTPUT"); output != "" {
			payload, marshalErr := json.MarshalIndent(receipt, "", "  ")
			Expect(marshalErr).NotTo(HaveOccurred())
			payload = append(payload, '\n')
			Expect(filepath.Clean(output)).To(Equal(output))
			Expect(os.MkdirAll(filepath.Dir(output), 0o755)).To(Succeed())
			Expect(os.WriteFile(output, payload, 0o600)).To(Succeed())
		}
		Expect(receipt.EvaluatedCount).To(Equal(137))
		Expect(receipt.Selected.WrongCount).To(Equal(0))
		Expect(receipt.Selected.Precision).To(Equal(1.0))
	})

	It("measures installed embedding assistance only on explicit live development invocation", func() {
		if os.Getenv("NXB21_EMBEDDING_LIVE") != "1" {
			Skip("set NXB21_EMBEDDING_LIVE=1 for the reusable development-ledger run")
		}
		baseURL := os.Getenv("NXB21_EMBEDDING_BASE_URL")
		if baseURL == "" {
			baseURL = "http://127.0.0.1:8080"
		}
		cfg := config{
			LocalAIURL: baseURL, SemanticEmbeddingsEnabled: true,
			SemanticEmbeddingModel: semanticEmbeddingModelDefault, SemanticEmbeddingTimeout: semanticEmbeddingTimeoutMax,
		}
		workspace := hybridQueryRequest{QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)}}
		descriptors := semanticEmbeddingDescriptors(semanticOperationCandidates(workspace), nil)
		catalogHash := semanticEmbeddingCatalogHash(semanticEmbeddingModel(cfg), descriptors)
		provider := localAISemanticEmbeddingProvider{
			baseURL: cfg.LocalAIURL, apiKey: cfg.LocalAIAPIKey,
			model: semanticEmbeddingModel(cfg), timeout: semanticEmbeddingCatalogTimeout,
		}
		prewarmContext, cancel := context.WithTimeout(context.Background(), semanticEmbeddingCatalogTimeout)
		defer cancel()
		_, _, err := cachedSemanticDescriptorEmbeddings(prewarmContext, provider, catalogHash, descriptors)
		Expect(err).NotTo(HaveOccurred())

		before, err := buildDeterministicSemanticEvaluation(semanticFrameOperationScorer{})
		Expect(err).NotTo(HaveOccurred())
		calibration, err := buildSemanticThresholdCalibration()
		Expect(err).NotTo(HaveOccurred())
		scorer := &semanticEmbeddingOperationScorer{ctx: context.Background(), cfg: cfg}
		after, err := buildDeterministicSemanticEvaluation(scorer)
		Expect(err).NotTo(HaveOccurred())
		beforeByID := map[string]bool{}
		for _, row := range before.Rows {
			beforeByID[row.VariantID] = row.Top1Hit
		}
		corrected, regressed := []string{}, []string{}
		for _, row := range after.Rows {
			switch {
			case !beforeByID[row.VariantID] && row.Top1Hit:
				corrected = append(corrected, row.VariantID)
			case beforeByID[row.VariantID] && !row.Top1Hit:
				regressed = append(regressed, row.VariantID)
			}
		}
		thresholds, err := semanticCompilerThresholds()
		Expect(err).NotTo(HaveOccurred())
		afterConfidence := semanticThresholdCurvePointV1{
			ExactMetadataMinimum: thresholds.ExactMetadataMinimum, StructuralScoreMinimum: thresholds.StructuralScoreMinimum,
			Top1Top2MarginMinimum: thresholds.Top1Top2MarginMinimum, StructuralOverrideMinimum: thresholds.StructuralOverrideMinimum,
			ConfidentCount: scorer.confidentCount, CorrectCount: scorer.confidentCorrect, WrongCount: scorer.confidentWrong,
		}
		if after.EvaluatedCount > 0 {
			afterConfidence.Coverage = float64(afterConfidence.ConfidentCount) / float64(after.EvaluatedCount)
		}
		if afterConfidence.ConfidentCount > 0 {
			afterConfidence.Precision = float64(afterConfidence.CorrectCount) / float64(afterConfidence.ConfidentCount)
		}
		receipt := semanticEmbeddingEvaluationReceiptV1{
			ContractVersion: "forensics.semantic-embedding-evaluation/v1",
			ModelID:         semanticEmbeddingModel(cfg), CallPath: "/v1/embeddings",
			Before: before, After: after,
			EmbeddingAvailable: scorer.availableCount, LexicalFallback: scorer.lexicalFallbackCount,
			CorrectedVariantIDs: corrected, RegressedVariantIDs: regressed,
			QuestionTimeoutMS: int(semanticEmbeddingTimeout(cfg).Milliseconds()), RankingSignalOnly: true,
			ConfidentBefore: calibration.Selected, ConfidentAfter: afterConfidence,
		}
		if output := os.Getenv("NXB21_EMBEDDING_EVAL_OUTPUT"); output != "" {
			payload, marshalErr := json.MarshalIndent(receipt, "", "  ")
			Expect(marshalErr).NotTo(HaveOccurred())
			payload = append(payload, '\n')
			output = filepath.Clean(output)
			Expect(output).NotTo(Equal("."))
			Expect(output).NotTo(HavePrefix(".." + string(os.PathSeparator)))
			Expect(os.MkdirAll(filepath.Dir(output), 0o755)).To(Succeed())
			Expect(os.WriteFile(output, payload, 0o600)).To(Succeed())
		}
		Expect(scorer.availableCount).To(BeNumerically(">", 0))
		Expect(scorer.availableCount + scorer.lexicalFallbackCount).To(Equal(after.EvaluatedCount))
		Expect(regressed).To(BeEmpty())
		Expect(corrected).NotTo(BeEmpty())
		Expect(after.Top1).To(BeNumerically(">", before.Top1))
		Expect(afterConfidence.WrongCount).To(Equal(0))
	})
})
