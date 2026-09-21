package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/mudler/LocalAI/pkg/httpclient"
)

const (
	semanticEmbeddingAuditContractV1 = "forensics.semantic-embedding-resolution/v1"
	semanticEmbeddingModelDefault    = "qwen3-embedding-0.6b"
	semanticEmbeddingTimeoutMax      = 2 * time.Second
	semanticEmbeddingCatalogTimeout  = 3 * time.Minute
	semanticEmbeddingCatalogBatch    = 1
	semanticEmbeddingDescriptorRunes = 1536
	semanticEmbeddingResponseMax     = 8 << 20
)

type SemanticEmbeddingResolutionAuditV1 struct {
	ContractVersion        string `json:"contract_version"`
	ModelID                string `json:"model_id"`
	CallPath               string `json:"call_path"`
	CatalogHash            string `json:"catalog_hash"`
	DescriptorCount        int    `json:"descriptor_count"`
	Dimensions             int    `json:"dimensions,omitempty"`
	DescriptorCacheHit     bool   `json:"descriptor_cache_hit"`
	QuestionEmbeddingCalls int    `json:"question_embedding_calls"`
	State                  string `json:"state"`
	RankingSignalOnly      bool   `json:"ranking_signal_only"`
}

type semanticEmbeddingDescriptor struct {
	ID   string
	Text string
}

type semanticEmbeddingProvider interface {
	Embed(context.Context, []string) ([][]float64, error)
}

type localAISemanticEmbeddingProvider struct {
	baseURL string
	apiKey  string
	model   string
	timeout time.Duration
}

type semanticEmbeddingCacheEntry struct {
	ready   chan struct{}
	vectors map[string][]float64
	err     error
}

var semanticEmbeddingDescriptorCache = struct {
	sync.Mutex
	entries map[string]*semanticEmbeddingCacheEntry
}{entries: map[string]*semanticEmbeddingCacheEntry{}}

func semanticEmbeddingTimeout(cfg config) time.Duration {
	timeout := cfg.SemanticEmbeddingTimeout
	if timeout <= 0 || timeout > semanticEmbeddingTimeoutMax {
		return semanticEmbeddingTimeoutMax
	}
	return timeout
}

func semanticEmbeddingModel(cfg config) string {
	if model := strings.TrimSpace(cfg.SemanticEmbeddingModel); model != "" {
		return model
	}
	return semanticEmbeddingModelDefault
}

func (provider localAISemanticEmbeddingProvider) Embed(ctx context.Context, inputs []string) ([][]float64, error) {
	if len(inputs) == 0 {
		return nil, errors.New("embedding input is empty")
	}
	payload, err := json.Marshal(map[string]any{"model": provider.model, "input": inputs})
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(provider.baseURL, "/")+"/v1/embeddings", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	if provider.apiKey != "" {
		request.Header.Set("Authorization", "Bearer "+provider.apiKey)
	}
	response, err := httpclient.NewWithTimeout(provider.timeout).Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embedding runtime status %d", response.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, semanticEmbeddingResponseMax+1))
	if err != nil || len(raw) > semanticEmbeddingResponseMax || !utf8.Valid(raw) {
		return nil, errors.New("embedding response is malformed")
	}
	var envelope struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
			Index     int       `json:"index"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil || len(envelope.Data) != len(inputs) {
		return nil, errors.New("embedding response cardinality mismatch")
	}
	vectors := make([][]float64, len(inputs))
	dimensions := 0
	for _, item := range envelope.Data {
		if item.Index < 0 || item.Index >= len(vectors) || len(item.Embedding) == 0 || vectors[item.Index] != nil {
			return nil, errors.New("embedding response index is invalid")
		}
		if dimensions == 0 {
			dimensions = len(item.Embedding)
		}
		if len(item.Embedding) != dimensions {
			return nil, errors.New("embedding dimensions are inconsistent")
		}
		for _, value := range item.Embedding {
			if math.IsNaN(value) || math.IsInf(value, 0) {
				return nil, errors.New("embedding contains a non-finite value")
			}
		}
		vectors[item.Index] = append([]float64(nil), item.Embedding...)
	}
	return vectors, nil
}

func semanticEmbeddingDescriptors(candidates []SemanticOperationCandidateV1, fields []FieldDescriptorV1) []semanticEmbeddingDescriptor {
	descriptors := make([]semanticEmbeddingDescriptor, 0, len(candidates)+len(fields))
	for _, candidate := range candidates {
		descriptors = append(descriptors, semanticEmbeddingDescriptor{
			ID:   "operation:" + candidate.OperationID,
			Text: semanticOperationEmbeddingText(candidate),
		})
	}
	for _, field := range fields {
		descriptors = append(descriptors, semanticEmbeddingDescriptor{
			ID: "field:" + field.FieldID,
			Text: boundedSemanticEmbeddingText(strings.Join([]string{
				field.NormalizedName, field.SourceName, strings.Join(field.SourceNames, " "),
				field.EffectiveType, strings.Join(field.AllowedFilters, " "), strings.Join(field.AllowedAggregates, " "),
				strings.Join(field.FamilyProvenance, " "),
			}, " ")),
		})
	}
	sort.Slice(descriptors, func(i, j int) bool { return descriptors[i].ID < descriptors[j].ID })
	return descriptors
}

// Embedding descriptors intentionally omit the broad family metadata used by
// BM25. That metadata repeats every related family operation and makes a
// request slower while diluting the operation-specific semantic signal.
func semanticOperationEmbeddingText(candidate SemanticOperationCandidateV1) string {
	template, _ := queryTemplateByName(queryTemplateNameByOperationID(candidate.OperationID))
	inputs := make([]string, 0, len(template.Inputs)*3)
	for _, input := range template.Inputs {
		inputs = append(inputs, input.Name, input.Label, input.Description)
	}
	return boundedSemanticEmbeddingText(strings.Join([]string{
		candidate.OperationID,
		strings.NewReplacer(".", " ", "_", " ", "-", " ").Replace(candidate.OperationID),
		candidate.FamilyID, candidate.Intent, candidate.Description,
		strings.Join(candidate.Measures, " "), strings.Join(candidate.GroupBy, " "),
		candidate.ResultKind, candidate.Presentation, strings.Join(template.RecordTypes, " "),
		template.Calculation, template.OutputDescription, strings.Join(inputs, " "),
	}, " "))
}

func boundedSemanticEmbeddingText(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	runes := []rune(value)
	if len(runes) > semanticEmbeddingDescriptorRunes {
		value = strings.TrimSpace(string(runes[:semanticEmbeddingDescriptorRunes]))
	}
	return value
}

func semanticEmbeddingCatalogHash(model string, descriptors []semanticEmbeddingDescriptor) string {
	hash := sha256.New()
	_, _ = hash.Write([]byte(semanticEmbeddingAuditContractV1))
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write([]byte(model))
	for _, descriptor := range descriptors {
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write([]byte(descriptor.ID))
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write([]byte(descriptor.Text))
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func cachedSemanticDescriptorEmbeddings(ctx context.Context, provider semanticEmbeddingProvider, key string, descriptors []semanticEmbeddingDescriptor) (map[string][]float64, bool, error) {
	semanticEmbeddingDescriptorCache.Lock()
	entry, cacheHit := semanticEmbeddingDescriptorCache.entries[key]
	if !cacheHit {
		entry = &semanticEmbeddingCacheEntry{ready: make(chan struct{})}
		semanticEmbeddingDescriptorCache.entries[key] = entry
	}
	semanticEmbeddingDescriptorCache.Unlock()

	if !cacheHit {
		go populateSemanticDescriptorEmbeddings(provider, key, descriptors, entry)
	}
	select {
	case <-ctx.Done():
		return nil, cacheHit, ctx.Err()
	case <-entry.ready:
		return entry.vectors, cacheHit, entry.err
	}
}

func populateSemanticDescriptorEmbeddings(provider semanticEmbeddingProvider, key string, descriptors []semanticEmbeddingDescriptor, entry *semanticEmbeddingCacheEntry) {
	texts := make([]string, len(descriptors))
	for index, descriptor := range descriptors {
		texts[index] = descriptor.Text
	}
	ctx, cancel := context.WithTimeout(context.Background(), semanticEmbeddingCatalogTimeout)
	defer cancel()
	vectors := make([][]float64, 0, len(texts))
	var err error
	for start := 0; start < len(texts); start += semanticEmbeddingCatalogBatch {
		end := min(start+semanticEmbeddingCatalogBatch, len(texts))
		var batch [][]float64
		batch, err = provider.Embed(ctx, texts[start:end])
		if err != nil {
			break
		}
		vectors = append(vectors, batch...)
	}
	resolved := map[string][]float64{}
	if err == nil && len(vectors) == len(descriptors) {
		for index, descriptor := range descriptors {
			resolved[descriptor.ID] = vectors[index]
		}
	} else if err == nil {
		err = errors.New("embedding descriptor cardinality mismatch")
	}

	semanticEmbeddingDescriptorCache.Lock()
	entry.vectors, entry.err = resolved, err
	if current := semanticEmbeddingDescriptorCache.entries[key]; err != nil && current == entry {
		delete(semanticEmbeddingDescriptorCache.entries, key)
	}
	close(entry.ready)
	semanticEmbeddingDescriptorCache.Unlock()
}

func semanticCosineSimilarity(left, right []float64) float64 {
	if len(left) == 0 || len(left) != len(right) {
		return 0
	}
	dot, leftNorm, rightNorm := 0.0, 0.0, 0.0
	for index := range left {
		dot += left[index] * right[index]
		leftNorm += left[index] * left[index]
		rightNorm += right[index] * right[index]
	}
	if leftNorm == 0 || rightNorm == 0 {
		return 0
	}
	return dot / (math.Sqrt(leftNorm) * math.Sqrt(rightNorm))
}

func semanticEmbeddingSignals(ctx context.Context, cfg config, question string, candidates []SemanticOperationCandidateV1, fields []FieldDescriptorV1) (map[string]float64, map[string]float64, *SemanticEmbeddingResolutionAuditV1, string) {
	if !cfg.SemanticEmbeddingsEnabled || strings.TrimSpace(cfg.LocalAIURL) == "" {
		return nil, nil, nil, ""
	}
	model := semanticEmbeddingModel(cfg)
	timeout := semanticEmbeddingTimeout(cfg)
	descriptors := semanticEmbeddingDescriptors(candidates, fields)
	audit := &SemanticEmbeddingResolutionAuditV1{
		ContractVersion: semanticEmbeddingAuditContractV1, ModelID: model,
		CallPath: "/v1/embeddings", DescriptorCount: len(descriptors),
		State: "LEXICAL_FALLBACK", RankingSignalOnly: true,
	}
	if len(descriptors) == 0 || strings.TrimSpace(question) == "" {
		return nil, nil, audit, "EMBEDDING_INPUT_UNAVAILABLE"
	}
	audit.CatalogHash = semanticEmbeddingCatalogHash(model, descriptors)
	provider := localAISemanticEmbeddingProvider{baseURL: cfg.LocalAIURL, apiKey: cfg.LocalAIAPIKey, model: model, timeout: timeout}
	descriptorProvider := provider
	descriptorProvider.timeout = semanticEmbeddingCatalogTimeout
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	descriptorVectors, cacheHit, err := cachedSemanticDescriptorEmbeddings(callCtx, descriptorProvider, audit.CatalogHash, descriptors)
	audit.DescriptorCacheHit = cacheHit
	if err != nil {
		return nil, nil, audit, "EMBEDDING_DESCRIPTOR_UNAVAILABLE"
	}
	questionVectors, err := provider.Embed(callCtx, []string{question})
	audit.QuestionEmbeddingCalls = 1
	if err != nil || len(questionVectors) != 1 || len(questionVectors[0]) == 0 {
		return nil, nil, audit, "EMBEDDING_QUESTION_UNAVAILABLE"
	}
	audit.Dimensions = len(questionVectors[0])
	operationScores, fieldScores := map[string]float64{}, map[string]float64{}
	for _, descriptor := range descriptors {
		vector := descriptorVectors[descriptor.ID]
		if len(vector) != audit.Dimensions {
			return nil, nil, audit, "EMBEDDING_DIMENSION_MISMATCH"
		}
		similarity := semanticCosineSimilarity(questionVectors[0], vector)
		if strings.HasPrefix(descriptor.ID, "operation:") {
			operationScores[strings.TrimPrefix(descriptor.ID, "operation:")] = similarity
		} else {
			fieldScores[strings.TrimPrefix(descriptor.ID, "field:")] = similarity
		}
	}
	audit.State = "AVAILABLE"
	return operationScores, fieldScores, audit, ""
}

func fuseSemanticOperationEmbeddings(candidates []SemanticOperationCandidateV1, scores []SemanticOperationResolutionScoreV1, similarities map[string]float64) ([]SemanticOperationCandidateV1, []SemanticOperationResolutionScoreV1) {
	if len(similarities) == 0 || len(candidates) != len(scores) {
		return candidates, scores
	}
	thresholds, err := semanticCompilerThresholds()
	if err != nil {
		return candidates, scores
	}
	type fused struct {
		candidate SemanticOperationCandidateV1
		score     SemanticOperationResolutionScoreV1
	}
	values := make([]fused, len(candidates))
	for index := range candidates {
		score := scores[index]
		score.EmbeddingCosine = similarities[candidates[index].OperationID]
		score.EmbeddingScore = score.EmbeddingCosine * thresholds.OperationEmbeddingWeight
		score.TotalScore += score.EmbeddingScore
		values[index] = fused{candidate: candidates[index], score: score}
	}
	sort.SliceStable(values, func(i, j int) bool {
		if values[i].score.TotalScore == values[j].score.TotalScore {
			return values[i].candidate.OperationID < values[j].candidate.OperationID
		}
		return values[i].score.TotalScore > values[j].score.TotalScore
	})
	for index := range values {
		candidates[index] = values[index].candidate
		values[index].score.Rank = index + 1
		scores[index] = values[index].score
	}
	return candidates, scores
}
