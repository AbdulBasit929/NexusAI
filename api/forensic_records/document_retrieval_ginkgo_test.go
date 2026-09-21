package main

import (
	"strings"

	"github.com/mudler/LocalAI/pkg/forensictext"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func plannedDocumentRequest(question string) hybridQueryRequest {
	req := hybridQueryRequest{Query: question, CollectionID: "case-a", MaxKBResults: 10}
	req = bindDerivedTextQuery(req)
	if req.Template == "" {
		req.Template = chooseTemplate(question, "")
	}
	return bindSemanticRetrievalStrategy(req)
}

func documentPassageRow(id, evidenceID, versionID, file, text string, locator map[string]any) map[string]any {
	return map[string]any{
		"artifact_id": id, "artifact_type": "forensics.document-native-text-passage/v1",
		"tenant_id": "tenant-a", "collection_id": "case-a", "evidence_id": evidenceID,
		"version_id": versionID, "run_id": "run-a", "parent_artifact_id": "",
		"source_file": file, "passage_text": text, "citation_locator": locator,
	}
}

var _ = Describe("Document retrieval strategy execution", func() {
	DescribeTable("routes natural document questions through the SemanticFrame mode",
		func(question, mode string, kb, derived bool) {
			req := plannedDocumentRequest(question)
			Expect(req.Template).To(Equal("document_search"))
			Expect(req.RetrievalMode).To(Equal(mode))
			Expect(retrievalUsesKnowledgeBase(req)).To(Equal(kb))
			Expect(retrievalUsesDerivedText(req)).To(Equal(derived))
		},
		Entry("conceptual document question", "What does this document say about retention policy?", semanticRetrievalHybrid, true, true),
		Entry("documents that mention a term", "Which documents mention REF-7788?", semanticRetrievalFullText, false, true),
		Entry("exact quoted phrase", `Find the exact phrase "retention schedule" in the document.`, semanticRetrievalExactPhrase, false, true),
		Entry("every mention", "Show every mention of REF-7788 in the document.", semanticRetrievalFullText, false, true),
		Entry("grounded summary", "Summarize the relevant document sections.", semanticRetrievalHybrid, true, true),
		Entry("source passage", "Show source passages from the document.", semanticRetrievalHybrid, true, true),
		Entry("exact location", "Where exactly is this stated in the document?", semanticRetrievalHybrid, true, true),
	)

	It("never silently changes an exact phrase into semantic retrieval", func() {
		req := plannedDocumentRequest(`Find the exact phrase "Annual Report" in the document.`)
		Expect(req.TextQuery).NotTo(BeNil())
		Expect(req.TextQuery.MatchSemantic).To(Equal(forensictext.ExactPhrase))
		Expect(req.RetrievalMode).To(Equal(semanticRetrievalExactPhrase))
		audit := retrievalStrategyAudit(req)
		Expect(audit).To(HaveKeyWithValue("kb_retrieval", false))
		Expect(audit).To(HaveKeyWithValue("derived_text", true))
		Expect(audit).To(HaveKeyWithValue("exact_is_semantic", false))
	})

	It("preserves real extractor locators and citations for exact matches", func() {
		req := hybridQueryRequest{
			TenantID: "tenant-a", CollectionID: "case-a", Template: "document_search", MaxKBResults: 10,
			TextQuery: &forensictext.Query{LiteralText: "retention schedule", MatchSemantic: forensictext.ExactPhrase},
		}
		locator := map[string]any{"page": 4, "passage": 2, "source_file": "policy.pdf"}
		evidence := governedDerivedTextEvidence([]map[string]any{
			documentPassageRow("passage-1", "evidence-1", "version-1", "policy.pdf", "The retention schedule applies for seven years.", locator),
		}, req)
		results := evidenceResults(evidence)
		Expect(results).To(HaveLen(1))
		metadata := results[0]["metadata"].(map[string]any)
		Expect(metadata["citation_locator"]).To(Equal(locator))
		Expect(results[0]["citation"]).To(Equal("nexusai://evidence/evidence-1/artifacts/passage-1"))
		Expect(evidence).To(HaveKeyWithValue("result_state", "COMPLETE_RESULTS"))
	})

	It("performs a bounded fair comparison over explicitly selected documents", func() {
		sourceSet := &StructuredSourceSetV1{ContractVersion: structuredSourceSetContractV1, Sources: []StructuredSourceRefV1{
			{SourceID: "doc-a", EvidenceID: "evidence-a", VersionID: "version-a", SourceFile: "a.pdf"},
			{SourceID: "doc-b", EvidenceID: "evidence-b", VersionID: "version-b", SourceFile: "b.pdf"},
		}}
		req := hybridQueryRequest{TenantID: "tenant-a", CollectionID: "case-a", Query: "Compare these two documents.", Template: "document_search", SourceSet: sourceSet, MaxKBResults: 3}
		req = bindSemanticRetrievalStrategy(req)
		Expect(req.RetrievalMode).To(Equal(semanticRetrievalSourceScoped))
		Expect(req.TextQuery).NotTo(BeNil())
		Expect(req.TextQuery.MatchSemantic).To(Equal(forensictext.Source))
		rows := []map[string]any{
			documentPassageRow("a-1", "evidence-a", "version-a", "a.pdf", "Alpha one", map[string]any{"page": 1}),
			documentPassageRow("a-2", "evidence-a", "version-a", "a.pdf", "Alpha two", map[string]any{"page": 2}),
			documentPassageRow("b-1", "evidence-b", "version-b", "b.pdf", "Beta one", map[string]any{"page": 1}),
			documentPassageRow("b-2", "evidence-b", "version-b", "b.pdf", "Beta two", map[string]any{"page": 2}),
			documentPassageRow("outside", "evidence-c", "version-c", "c.pdf", "Outside", map[string]any{"page": 1}),
		}
		evidence := annotateDocumentComparison(governedDerivedTextEvidence(rows, req), req)
		results := evidenceResults(evidence)
		Expect(results).To(HaveLen(3))
		ids := []string{stringValueAny(results[0]["id"]), stringValueAny(results[1]["id"]), stringValueAny(results[2]["id"])}
		Expect(ids).To(Equal([]string{"a-1", "b-1", "a-2"}))
		Expect(strings.Join(ids, ",")).NotTo(ContainSubstring("outside"))
		comparison := evidence["document_comparison"].(map[string]any)
		Expect(comparison).To(HaveKeyWithValue("source_count", 2))
		Expect(comparison).To(HaveKeyWithValue("all_sources_represented", true))
		Expect(evidence).To(HaveKeyWithValue("search_completeness", "TRUNCATED"))
	})

	It("fails a comparison closed until its exact source set is supplied", func() {
		req := plannedDocumentRequest("Compare these two documents.")
		Expect(documentComparisonRequested(req)).To(BeTrue())
		Expect(req.SourceSet).To(BeNil())
		Expect(clarificationQuestion(req.Query, req.Template)).To(Equal("Which two to eight exact documents should I compare?"))
	})

	It("carries selected document filters into the bounded derived-text SQL", func() {
		Expect(derivedTextEvidenceSQL).To(ContainSubstring("artifacts.evidence_id::text = ANY($7::text[])"))
		Expect(derivedTextEvidenceSQL).To(ContainSubstring("items.original_filename = ANY($8::text[])"))
		Expect(derivedTextEvidenceSQL).To(ContainSubstring("artifacts.version_id::text = ANY($9::text[])"))
	})

	It("presents document passages as grounded answers, findings, tables, citations, and methodology", func() {
		req := plannedDocumentRequest(`Find the exact phrase "retention schedule" in the document.`)
		req.TenantID, req.CollectionID = "tenant-a", "case-a"
		locator := map[string]any{"page": 4, "paragraph": 2, "source_file": "policy.pdf"}
		evidence := governedDerivedTextEvidence([]map[string]any{
			documentPassageRow("passage-1", "evidence-1", "version-1", "policy.pdf", "The retention schedule applies for seven years.", locator),
		}, req)
		evidence = annotateRetrievalEvidence(req, evidence)
		resp := hybridQueryResponse{
			CollectionID: req.CollectionID, Template: "document_search", Route: []string{"derived_text_lexical"},
			Evidence: evidence, Answer: map[string]any{"evidence_count": 1, "evidence_status": "matched"},
			Planner: map[string]any{"retrieval_strategy": retrievalStrategyAudit(req)},
		}
		resp.Enterprise = buildEnterprisePayload(req, resp)
		answer := stringValueAny(resp.Enterprise["executive_answer"])
		Expect(answer).To(ContainSubstring("retention schedule applies for seven years"))
		Expect(answer).NotTo(ContainSubstring("cited evidence result"))
		grid := resp.Enterprise["data_grid"].(map[string]any)
		Expect(grid).To(HaveKeyWithValue("title", "Cited document passages"))
		Expect(mapsFromAny(grid["rows"])).To(HaveLen(1))
		Expect(mapsFromAny(grid["rows"])[0]).To(HaveKeyWithValue("source_location", "Page 4 · Paragraph 2"))
		packet := resp.Enterprise["fact_packet"].(FactPacketV1)
		Expect(packet.NormalizedParameters).To(HaveKeyWithValue("retrieval_mode", semanticRetrievalExactPhrase))
		Expect(packet.Facts).To(ContainElement(And(
			HaveField("Kind", "source_passage"),
			HaveField("CitationIDs", ConsistOf("C1")),
		)))

		public := legacyHybridToEnterpriseV1(req.CollectionID, "request-document", map[string]any{"tenant_id": req.TenantID, "limit": 10}, resp)
		Expect(public.Tables).To(HaveLen(1))
		Expect(public.DeterministicFindings).To(HaveLen(1))
		Expect(public.DeterministicFindings[0].CitationIDs).To(HaveLen(1))
		Expect(public.SemanticEvidence).To(HaveLen(1))
		Expect(public.SemanticEvidence[0].Locator).To(ContainSubstring(`"page":4`))
		Expect(public.ExecutionTrace.Parameters).To(HaveKeyWithValue("retrieval_mode", semanticRetrievalExactPhrase))
	})

	It("reports bounded or incomplete document coverage as a limitation", func() {
		resp := hybridQueryResponse{Template: "document_search", Evidence: map[string]any{
			"search_completeness":   "TRUNCATED",
			"comparison_limitation": "One selected scanned document has no native-text passages.",
		}}
		limitations := enterpriseLimitations(resp)
		Expect(limitations).To(ContainElement(ContainSubstring("bounded result limit")))
		Expect(limitations).To(ContainElement(ContainSubstring("scanned document")))
	})
})
