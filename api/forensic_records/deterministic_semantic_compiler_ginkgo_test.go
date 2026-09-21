package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func semanticEmbeddingTestServer(vectorFor func(string) []float64, descriptorCalls, questionCalls *atomic.Int32) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/embeddings" {
			http.NotFound(writer, request)
			return
		}
		var payload struct {
			Model string   `json:"model"`
			Input []string `json:"input"`
		}
		if json.NewDecoder(request.Body).Decode(&payload) != nil || len(payload.Input) == 0 {
			http.Error(writer, "invalid embedding request", http.StatusBadRequest)
			return
		}
		if len(payload.Input) == 1 && payload.Input[0] == "Which department has the highest average invoice total?" {
			questionCalls.Add(1)
		} else {
			descriptorCalls.Add(1)
		}
		data := make([]map[string]any, len(payload.Input))
		for index, input := range payload.Input {
			data[index] = map[string]any{"index": index, "embedding": vectorFor(input)}
		}
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{"data": data})
	}))
}

func deterministicCompilerFixture() (hybridQueryRequest, []FieldDescriptorV1, []map[string]any) {
	req := hybridQueryRequest{
		TenantID: "tenant-a", UserID: "analyst-a", CollectionID: "case-a",
		EvidenceID:        "10000000-0000-4000-8000-000000000001",
		EvidenceVersionID: "20000000-0000-4000-8000-000000000001",
		QueryScope:        queryEvidenceScope{Kind: string(EvidenceScopeSelected), SourceFamily: "generic"},
	}
	rows := []map[string]any{
		{"tenant_id": req.TenantID, "collection_id": req.CollectionID, "evidence_id": req.EvidenceID, "evidence_version_id": req.EvidenceVersionID, "record_id": "r1", "record_type": "generic", "raw_payload": map[string]any{"department": "Sales", "invoice_total": 300, "event_time": "2026-09-01T10:00:00Z"}},
		{"tenant_id": req.TenantID, "collection_id": req.CollectionID, "evidence_id": req.EvidenceID, "evidence_version_id": req.EvidenceVersionID, "record_id": "r2", "record_type": "generic", "raw_payload": map[string]any{"department": "Sales", "invoice_total": 500, "event_time": "2026-09-02T11:00:00Z"}},
		{"tenant_id": req.TenantID, "collection_id": req.CollectionID, "evidence_id": req.EvidenceID, "evidence_version_id": req.EvidenceVersionID, "record_id": "r3", "record_type": "generic", "raw_payload": map[string]any{"department": "Operations", "invoice_total": 150, "event_time": "2026-09-03T12:00:00Z"}},
	}
	return req, buildSourceNativeFieldCatalog(req, rows), rows
}

var _ = Describe("Deterministic semantic frame and compiler", func() {
	It("extracts bounded slots while preserving typed identifiers and phrase literals", func() {
		req := hybridQueryRequest{Query: `Show every mention of "dock damage" in this document for 923001110001`, QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeSelected), SourceFamily: "document"}}
		frame := extractSemanticFrame(req)
		Expect(frame.ContractVersion).To(Equal(semanticFrameContractV1))
		Expect(frame.RequestClass).To(Equal(semanticRequestGovernedAnalysis))
		Expect(frame.FamilyHint).To(Equal("document_intelligence"))
		Expect(frame.Goal).To(Equal("search"))
		Expect(frame.RetrievalMode).To(Equal(semanticRetrievalSourceScoped))
		Expect(frame.Filters).To(ContainElement(SemanticFrameFilterV1{FieldHint: "derived_text", Operator: "PHRASE_CONTAINS", Literal: "dock damage"}))
		Expect(frame.Identifiers).To(HaveLen(1))
		Expect(frame.Identifiers[0].Raw).To(Equal("923001110001"))
	})

	It("publishes only bounded server-issued capability fields", func() {
		req, catalog, _ := deterministicCompilerFixture()
		snapshot := buildSemanticCapabilitySnapshot(req, catalog)
		payload, err := json.Marshal(snapshot)
		Expect(err).NotTo(HaveOccurred())
		Expect(snapshot.ContractVersion).To(Equal(semanticCapabilitySnapshotV1))
		Expect(snapshot.IssuedFields).NotTo(BeEmpty())
		Expect(string(payload)).NotTo(ContainSubstring("department"))
		Expect(string(payload)).NotTo(ContainSubstring("invoice_total"))
		Expect(string(payload)).NotTo(ContainSubstring("json_path"))
		Expect(string(payload)).NotTo(ContainSubstring("table_name"))
	})

	It("prefers a specifically applicable registered operation without a model", func() {
		req := hybridQueryRequest{Query: "Rank the call counterparties for 923146208975 by interaction frequency", QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)}}
		result, err := compileDeterministicSemanticRequest(context.Background(), config{}, req, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(validateDeterministicCompilerResult(result)).To(Succeed())
		Expect(result.Executor).To(Equal(semanticExecutorRegistered), "reason=%s frame=%+v top=%+v", result.ReasonCode, result.Frame, result.RankedOperations[:1])
		Expect(result.OperationID).To(Equal("cdr.frequent_contacts"))
	})

	It("compiles a novel aggregate against issued fields when no registered operation covers it", func() {
		req, catalog, rows := deterministicCompilerFixture()
		req.Query = "Which department has the highest average invoice total?"
		started := time.Now()
		result, err := compileDeterministicSemanticRequest(context.Background(), config{}, req, catalog)
		Expect(err).NotTo(HaveOccurred())
		Expect(time.Since(started)).To(BeNumerically("<", 300*time.Millisecond))
		Expect(validateDeterministicCompilerResult(result)).To(Succeed())
		Expect(result.Executor).To(Equal(semanticExecutorDynamic), "reason=%s degradations=%v frame=%+v", result.ReasonCode, result.Degradations, result.Frame)
		Expect(result.DynamicPlan).NotTo(BeNil())
		Expect(result.DynamicPlan.GroupFields).To(HaveLen(1))
		Expect(result.DynamicPlan.Measures).To(ConsistOf(SourceNativeMeasureV1{MeasureID: "m1", Op: "AVG", FieldID: result.DynamicPlan.Measures[0].FieldID}))
		Expect(result.DynamicPlan.Sort).To(Equal([]SourceNativeSortV1{{Target: "m1", Direction: "DESC"}}))
		Expect(result.DynamicPlan.Limit).To(Equal(1))

		answer, executeErr := executeSourceNativePlan(req, result.DynamicPlan, catalog, rows)
		Expect(executeErr).NotTo(HaveOccurred())
		results, ok := answer["source_native_results"].([]map[string]any)
		Expect(ok).To(BeTrue())
		Expect(results).To(HaveLen(1))
		Expect(results[0]["department"]).To(Equal("Sales"))
		Expect(results[0]["m1"]).To(Equal(int64(400)))
	})

	It("caches descriptor embeddings by catalog hash and embeds the question once per request", func() {
		var descriptorCalls, questionCalls atomic.Int32
		server := semanticEmbeddingTestServer(func(string) []float64 { return []float64{1, 0, 0} }, &descriptorCalls, &questionCalls)
		DeferCleanup(server.Close)
		req, catalog, _ := deterministicCompilerFixture()
		req.Query = "Which department has the highest average invoice total?"
		cfg := config{
			LocalAIURL: server.URL, SemanticEmbeddingsEnabled: true,
			SemanticEmbeddingModel: "cache-test-embedding", SemanticEmbeddingTimeout: time.Second,
		}
		first, err := compileDeterministicSemanticRequest(context.Background(), cfg, req, catalog)
		Expect(err).NotTo(HaveOccurred())
		Expect(first.Executor).To(Equal(semanticExecutorDynamic))
		Expect(first.EmbeddingAudit).NotTo(BeNil())
		Expect(first.EmbeddingAudit.State).To(Equal("AVAILABLE"))
		Expect(first.EmbeddingAudit.DescriptorCacheHit).To(BeFalse())
		Expect(first.EmbeddingAudit.QuestionEmbeddingCalls).To(Equal(1))
		Expect(first.EmbeddingAudit.Dimensions).To(Equal(3))
		firstDescriptorCalls := descriptorCalls.Load()
		Expect(firstDescriptorCalls).To(BeNumerically(">", 0))

		second, err := compileDeterministicSemanticRequest(context.Background(), cfg, req, catalog)
		Expect(err).NotTo(HaveOccurred())
		Expect(second.EmbeddingAudit.DescriptorCacheHit).To(BeTrue())
		Expect(descriptorCalls.Load()).To(Equal(firstDescriptorCalls))
		Expect(questionCalls.Load()).To(Equal(int32(2)))
	})

	It("keeps embedding descriptors operation-specific and within the runtime budget", func() {
		workspace := hybridQueryRequest{QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)}}
		descriptors := semanticEmbeddingDescriptors(semanticOperationCandidates(workspace), deterministicEvaluationFieldCatalog())
		Expect(descriptors).NotTo(BeEmpty())
		for _, descriptor := range descriptors {
			Expect(strings.TrimSpace(descriptor.Text)).NotTo(BeEmpty(), descriptor.ID)
			Expect(utf8.RuneCountInString(descriptor.Text)).To(BeNumerically("<=", semanticEmbeddingDescriptorRunes), descriptor.ID)
		}
	})

	It("changes a prior top-1 miss while preserving every existing top-1 hit under a neutral embedding signal", func() {
		ledger, err := loadQueryVariantLedger()
		Expect(err).NotTo(HaveOccurred())
		preserved, corrected := 0, false
		for _, entry := range ledger.Entries {
			if entry.Language != "english" || entry.ClarificationRequired || len(entry.FollowUpRequirements) > 0 {
				continue
			}
			req, eligible := semanticRecallRequest(entry.CanonicalOperation, entry.Text)
			if !eligible {
				continue
			}
			ranked, scores := rankSemanticOperationsForFrame(req.Query, extractSemanticFrame(req), semanticOperationCandidates(req))
			Expect(ranked).NotTo(BeEmpty())
			similarities := make(map[string]float64, len(ranked))
			for _, candidate := range ranked {
				similarities[candidate.OperationID] = 0
			}
			if ranked[0].OperationID == entry.CanonicalOperation {
				fused, _ := fuseSemanticOperationEmbeddings(ranked, scores, similarities)
				Expect(fused[0].OperationID).To(Equal(entry.CanonicalOperation), "regressed %s", entry.VariantID)
				preserved++
				continue
			}
			if entry.VariantID != "qv-083" {
				continue
			}
			similarities[ranked[0].OperationID] = -1
			similarities[entry.CanonicalOperation] = 1
			fused, fusedScores := fuseSemanticOperationEmbeddings(ranked, scores, similarities)
			Expect(fused[0].OperationID).To(Equal(entry.CanonicalOperation))
			Expect(fusedScores[0].EmbeddingScore).To(BeNumerically(">", 0))
			corrected = true
		}
		Expect(preserved).To(Equal(130))
		Expect(corrected).To(BeTrue())
	})

	It("does not turn an embedding-only lead into deterministic execution confidence", func() {
		candidates := []SemanticOperationCandidateV1{
			{OperationID: "operation.lexical_leader"},
			{OperationID: "operation.embedding_leader"},
		}
		scores := []SemanticOperationResolutionScoreV1{
			{OperationID: candidates[0].OperationID, Rank: 1, TotalScore: 10},
			{OperationID: candidates[1].OperationID, Rank: 2, TotalScore: 9.8},
		}
		ranked, fused := fuseSemanticOperationEmbeddings(candidates, scores, map[string]float64{
			"operation.lexical_leader":   0,
			"operation.embedding_leader": 1,
		})
		Expect(ranked[0].OperationID).To(Equal("operation.embedding_leader"))
		_, matched, confidenceMargin := deterministicRegisteredMatch(SemanticFrameV1{}, ranked, fused, nil)
		Expect(matched).To(BeFalse())
		Expect(confidenceMargin).To(BeNumerically("<", 0))
	})

	It("keeps embeddings ranking-only and preserves hard field type policy", func() {
		var descriptorCalls, questionCalls atomic.Int32
		server := semanticEmbeddingTestServer(func(input string) []float64 {
			if strings.Contains(input, "department") || strings.HasPrefix(input, "Which department") {
				return []float64{1, 0}
			}
			return []float64{0, 1}
		}, &descriptorCalls, &questionCalls)
		DeferCleanup(server.Close)
		req, catalog, _ := deterministicCompilerFixture()
		req.Query = "Which department has the highest average invoice total?"
		result, err := compileDeterministicSemanticRequest(context.Background(), config{
			LocalAIURL: server.URL, SemanticEmbeddingsEnabled: true,
			SemanticEmbeddingModel: "type-policy-test-embedding", SemanticEmbeddingTimeout: time.Second,
		}, req, catalog)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Executor).To(Equal(semanticExecutorDynamic))
		Expect(result.DynamicPlan).NotTo(BeNil())
		Expect(result.DynamicPlan.Measures).To(HaveLen(1))
		measureField := result.DynamicPlan.Measures[0].FieldID
		for _, field := range catalog {
			if field.FieldID == measureField {
				Expect(field.EffectiveType).To(Or(Equal(fieldTypeInteger), Equal(fieldTypeDecimal)))
				Expect(field.NormalizedName).To(Equal("invoice_total"))
			}
		}
	})

	It("degrades to lexical resolution within the configured embedding timeout", func() {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			time.Sleep(100 * time.Millisecond)
			writer.WriteHeader(http.StatusServiceUnavailable)
		}))
		DeferCleanup(server.Close)
		req, catalog, _ := deterministicCompilerFixture()
		req.Query = "Which department has the highest average invoice total?"
		started := time.Now()
		result, err := compileDeterministicSemanticRequest(context.Background(), config{
			LocalAIURL: server.URL, SemanticEmbeddingsEnabled: true,
			SemanticEmbeddingModel: "timeout-test-embedding", SemanticEmbeddingTimeout: 20 * time.Millisecond,
		}, req, catalog)
		Expect(err).NotTo(HaveOccurred())
		Expect(time.Since(started)).To(BeNumerically("<", 300*time.Millisecond))
		Expect(result.Executor).To(Equal(semanticExecutorDynamic))
		Expect(result.Degradations).To(ContainElement("EMBEDDING_DESCRIPTOR_UNAVAILABLE"))
		Expect(result.EmbeddingAudit.State).To(Equal("LEXICAL_FALLBACK"))
	})

	It("keeps the registered and dynamic anti-escape rules structural", func() {
		req, catalog, _ := deterministicCompilerFixture()
		req.Query = "Average invoice total by department"
		result, err := compileDeterministicSemanticRequest(context.Background(), config{}, req, catalog)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Executor).To(Equal(semanticExecutorDynamic), "reason=%s degradations=%v frame=%+v", result.ReasonCode, result.Degradations, result.Frame)

		registered := hybridQueryRequest{Query: "Show camera counts for vehicle observations", QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)}}
		result, err = compileDeterministicSemanticRequest(context.Background(), config{}, registered, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Executor).To(Equal(semanticExecutorRegistered), "reason=%s frame=%+v top=%+v", result.ReasonCode, result.Frame, result.RankedOperations[:1])
		Expect(result.OperationID).To(Equal("anpr.camera_activity"))
	})

	It("does not embed consumed ledger phrases in compiler source", func() {
		ledger, err := loadQueryVariantLedger()
		Expect(err).NotTo(HaveOccurred())
		sourceText := []string{}
		for _, path := range []string{"semantic_frame.go", "operation_applicability.go", "deterministic_semantic_compiler.go", "semantic_candidate_ranking.go"} {
			raw, readErr := os.ReadFile(path)
			Expect(readErr).NotTo(HaveOccurred())
			sourceText = append(sourceText, string(raw))
		}
		sources := strings.Join(semanticWords.FindAllString(strings.ToLower(strings.Join(sourceText, "\n")), -1), " ")
		for _, entry := range ledger.Entries {
			words := semanticWords.FindAllString(strings.ToLower(entry.Text), -1)
			if len(words) < 7 {
				continue
			}
			for index := 0; index+7 <= len(words); index++ {
				Expect(sources).NotTo(ContainSubstring(strings.Join(words[index:index+7], " ")))
			}
		}
	})
})
