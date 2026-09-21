package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sync"
)

const semanticCompilerThresholdsContractV1 = "forensics.semantic-compiler-thresholds/v1"

//go:embed contracts/semantic-compiler-thresholds-v1.json
var semanticCompilerThresholdsJSON []byte

type SemanticCompilerThresholdsV1 struct {
	ContractVersion            string  `json:"contract_version"`
	ExactMetadataMinimum       float64 `json:"exact_metadata_minimum"`
	StructuralScoreMinimum     float64 `json:"structural_score_minimum"`
	Top1Top2MarginMinimum      float64 `json:"top1_top2_margin_minimum"`
	StructuralOverrideMinimum  float64 `json:"structural_override_minimum"`
	FieldAbsoluteScoreMinimum  float64 `json:"field_absolute_score_minimum"`
	FieldTop1Top2MarginMinimum float64 `json:"field_top1_top2_margin_minimum"`
	OperationEmbeddingWeight   float64 `json:"operation_embedding_weight"`
	FieldExactNameWeight       float64 `json:"field_exact_name_weight"`
	FieldBM25Weight            float64 `json:"field_bm25_weight"`
	FieldEmbeddingWeight       float64 `json:"field_embedding_weight"`
}

var semanticCompilerThresholdsCache struct {
	sync.Once
	value SemanticCompilerThresholdsV1
	err   error
}

func semanticCompilerThresholds() (SemanticCompilerThresholdsV1, error) {
	semanticCompilerThresholdsCache.Do(func() {
		semanticCompilerThresholdsCache.err = json.Unmarshal(semanticCompilerThresholdsJSON, &semanticCompilerThresholdsCache.value)
		if semanticCompilerThresholdsCache.err == nil && semanticCompilerThresholdsCache.value.ContractVersion != semanticCompilerThresholdsContractV1 {
			semanticCompilerThresholdsCache.err = fmt.Errorf("unsupported semantic compiler thresholds contract %q", semanticCompilerThresholdsCache.value.ContractVersion)
		}
	})
	return semanticCompilerThresholdsCache.value, semanticCompilerThresholdsCache.err
}
