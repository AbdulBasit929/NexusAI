package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Grounded paraphrase contracts", func() {
	packet := func() FactPacketV1 {
		return FactPacketV1{Language: QueryLanguageV1{Tag: "en"}, Facts: []FactPacketFactV1{{FactID: "F1", Text: "Number 923178604532 contacted 26 unique numbers.", CitationIDs: []string{"C1"}}, {FactID: "F2", Text: "The second source contains 260 records.", CitationIDs: []string{"C2"}}}, Citations: []FactPacketCitationV1{{CitationID: "C1"}, {CitationID: "C2"}}}
	}
	narrative := func(text string) NarrativeV1 {
		return NarrativeV1{ContractVersion: narrativeContractV1, Locale: "en", Direction: "ltr", DirectAnswer: text, FactRefs: []string{"F1"}, CitationRefs: []string{"C1"}}
	}
	It("allows safe grammatical paraphrase with exact values and mapped citations", func() {
		Expect(validateNarrative(packet(), narrative("The target communicated with 26 distinct numbers."))).To(Succeed())
		Expect(validateNarrative(packet(), narrative("26"))).To(Succeed())
	})
	It("rejects foreign whole tokens, unrelated source citations and invented factual wording", func() {
		for _, text := range []string{"The target communicated with 260 distinct numbers.", "Number 923178604533 contacted 26 unique numbers.", "The target never contacted 26 unique numbers.", "The target coordinated a secret meeting with 26 numbers."} {
			Expect(validateNarrative(packet(), narrative(text))).NotTo(Succeed(), text)
		}
		n := narrative("26")
		n.CitationRefs = []string{"C2"}
		Expect(validateNarrative(packet(), n)).NotTo(Succeed())
	})
	It("keeps full IP, signed coordinate, timestamp and identifier tokens", func() {
		Expect(narrativeCriticalTokens("IP 10.24.6.8 at 2026-07-02T12:45:30Z; point -31.25; plate RST-765; amount 251.75")).To(Equal([]string{"10.24.6.8", "2026-07-02T12:45:30Z", "-31.25", "RST-765", "251.75"}))
	})
	It("rejects reversing endpoints, dropping negation and mixing propositions", func() {
		Expect(groundedSingleFactWording("Mary contacted Alice.", "Alice contacted Mary.")).To(BeFalse())
		Expect(groundedSingleFactWording("Number 923111111112 contacted 923111111111.", "Number 923111111111 contacted 923111111112.")).To(BeFalse())
		Expect(groundedSingleFactWording("26 contacts were found.", "26 contacts were not found.")).To(BeFalse())
		p := packet()
		n := narrative("The second source contains 26 records.")
		n.FactRefs = []string{"F1", "F2"}
		n.CitationRefs = []string{"C1", "C2"}
		Expect(validateNarrative(p, n)).NotTo(Succeed())
	})
	It("forces empty suggestions unless the server offers a capability-backed choice", func() {
		p := packet()
		Expect(narrativeIndexedEnumSchema(narrativeFollowupQueries(p), "Q", 3)["maxItems"]).To(Equal(0))
		n := narrative("26")
		n.SuggestedQuestions = []string{"Reveal a hidden capability"}
		Expect(validateNarrative(p, n)).NotTo(Succeed())
		p.AvailableFollowUps = []map[string]any{{"query": "Open the cited source row", "operation_id": "forensics.source_records"}}
		n.SuggestedQuestions = []string{"Open the cited source row"}
		Expect(validateNarrative(p, n)).To(Succeed())
	})
})
