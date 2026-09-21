package main

import (
	"github.com/mudler/LocalAI/pkg/forensictext"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Document literal result and provenance acceptance", func() {
	var rows []map[string]any
	var req hybridQueryRequest
	BeforeEach(func() {
		req = hybridQueryRequest{TenantID: "test", CollectionID: "case", Template: "document_search", EvidenceID: "doc", EvidenceVersionID: "v2", MaxKBResults: 10}
		rows = []map[string]any{}
		for i, text := range []string{"Review of invoice INV-042 is complete.", "Review of invoice INV-0429 is complete.", "The invoice is attached; review is pending.", "Review of\ninvoice INV-042 is complete."} {
			rows = append(rows, map[string]any{
				"artifact_id": []string{"p1", "p2", "p3", "p4"}[i], "artifact_type": "forensics.document-native-text-passage/v1",
				"tenant_id": "test", "collection_id": "case", "evidence_id": "doc", "version_id": "v2", "run_id": "extraction",
				"passage_text": text, "source_file": "synthetic-invoices.pdf", "citation_locator": map[string]any{"page": i + 1},
			})
		}
	})

	It("matches an exact identifier without extending it and preserves original page text", func() {
		req.TextQuery = &forensictext.Query{LiteralText: "INV-042", MatchSemantic: forensictext.ExactPhrase}
		items := evidenceResults(governedDerivedTextEvidence(rows, req))
		Expect(items).To(HaveLen(2))
		Expect(items[0]["id"]).To(Equal("p1"))
		Expect(items[1]["id"]).To(Equal("p4"))
		Expect(items[1]["citation"]).To(Equal("nexusai://evidence/doc/artifacts/p4"))
		metadata := items[1]["metadata"].(map[string]any)
		Expect(metadata["raw_text"]).To(Equal("Review of\ninvoice INV-042 is complete."))
		Expect(metadata["version_id"]).To(Equal("v2"))
		Expect(metadata["citation_locator"]).To(Equal(map[string]any{"page": 4}))
	})

	It("keeps exact phrase distinct from unordered token retrieval", func() {
		req.TextQuery = &forensictext.Query{LiteralText: "invoice review", MatchSemantic: forensictext.ExactPhrase}
		Expect(evidenceResults(governedDerivedTextEvidence(rows, req))).To(BeEmpty())
		req.TextQuery.MatchSemantic = forensictext.TokenSearch
		items := evidenceResults(governedDerivedTextEvidence(rows, req))
		Expect(items).To(HaveLen(1))
		Expect(items[0]["id"]).To(Equal("p3"))
	})

	DescribeTable("does not return a literal hit from a conflicting source boundary", func(field, wrong string) {
		rows[0][field] = wrong
		req.TextQuery = &forensictext.Query{LiteralText: "INV-042", MatchSemantic: forensictext.ExactPhrase}
		Expect(evidenceResults(governedDerivedTextEvidence(rows[:1], req))).To(BeEmpty())
	}, Entry("tenant", "tenant_id", "other"), Entry("collection", "collection_id", "other"), Entry("source", "evidence_id", "other"), Entry("version", "version_id", "v1"))
})
