package main

import (
	"encoding/json"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("STIM-5 language and grounded answer contracts", func() {
	It("normalizes composable English, Roman Urdu, Urdu, and mixed-language semantics without changing identifiers", func() {
		cases := []struct {
			query, template, language, target, direction string
		}{
			{"cdr actvty 923001234567", "temporal_activity", "en", "923001234567", ""},
			{"frequent cntcts 923001234567", "frequent_contacts", "en", "923001234567", ""},
			{"who he called most 923001234567", "frequent_contacts", "en", "923001234567", ""},
			{"923001234567 ki 10 July 2026 activity dikhao", "temporal_activity", "ur-Latn", "923001234567", ""},
			{"sirf outgoing dikhao", "", "ur-Latn", "", "OUTGOING"},
			{"10 جولائی 2026 کو 923001234567 کی سرگرمی دکھائیں", "temporal_activity", "ur", "923001234567", ""},
			{"اس نمبر کی outgoing activity show karo 923001234567", "temporal_activity", "mixed", "923001234567", "OUTGOING"},
			{"common contacts between these dono CDRs", "multi_cdr_comparison", "en", "", ""},
			{"same IMEI observation hai?", "multi_cdr_comparison", "ur-Latn", "", ""},
		}
		for _, item := range cases {
			plan := planRuntimeQuery(hybridQueryRequest{Query: item.query})
			if item.template != "" {
				Expect(plan.Template).To(Equal(item.template), item.query)
			}
			Expect(detectQueryLanguage(item.query).Tag).To(Equal(item.language), item.query)
			Expect(plan.Target).To(Equal(item.target), item.query)
			Expect(extractEventDirection(item.query)).To(Equal(item.direction), item.query)
		}
	})

	It("retains an exact comparison source set only inside valid scoped follow-up context", func() {
		now := time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC)
		sourceSet := &StructuredSourceSetV1{ContractVersion: structuredSourceSetContractV1, Sources: []StructuredSourceRefV1{
			{SourceID: "A", SourceFile: "a.csv"}, {SourceID: "B", SourceFile: "b.csv"},
		}}
		req := hybridQueryRequest{TenantID: "tenant-a", UserID: "analyst-a", CollectionID: "case-a", Query: "common contacts batao", ConversationContext: queryConversationContext{
			ContractVersion: followUpContextContractV1, TenantID: "tenant-a", UserID: "analyst-a", CollectionID: "case-a",
			ExpiresAt: now.Add(time.Minute).Format(time.RFC3339), Template: "multi_cdr_comparison", SourceSet: sourceSet,
		}}
		resolved, _ := applyAuditableConversationContext(req, planRuntimeQuery(req), now)
		Expect(resolved.SourceSet).ToNot(BeNil())
		Expect(structuredSourceIDs(resolved.SourceSet)).To(Equal([]string{"A", "B"}))

		crossScope := req
		crossScope.CollectionID = "case-b"
		blocked, _ := applyAuditableConversationContext(crossScope, planRuntimeQuery(crossScope), now)
		Expect(blocked.SourceSet).To(BeNil())
	})

	It("builds bounded request-local facts, relationships, and explicitly labeled citations", func() {
		rows := make([]map[string]any, 25)
		for index := range rows {
			rows[index] = map[string]any{"relationship_type": "observed_association", "source_entity": "MSISDN-1", "target_entity": "IMEI-1", "count": index + 1}
		}
		resp := hybridQueryResponse{
			CollectionID: "case-a", Template: "relationship_network", Route: []string{"records_sql"},
			QueryUnderstanding: QueryUnderstandingV1{Language: QueryLanguageV1{Tag: "en"}, RequestedOutput: RequestedOutputV1{ResultKind: "relationship", PresentationType: "graph"}},
			Answer:             map[string]any{"records_row_count": 25}, Telemetry: QueryTelemetry{RequestID: "request-1"},
		}
		enterprise := map[string]any{
			"status": "answered_with_limitations", "executive_answer": "25 exact relationship rows were returned.",
			"operation":   map[string]any{"operation_id": "records.relationship_network"},
			"metrics":     []map[string]any{{"label": "Results", "value": 25}},
			"data_grid":   map[string]any{"rows": rows, "count": 25, "columns": []string{"relationship_type", "source_entity", "target_entity", "count"}},
			"provenance":  []map[string]any{{"source": "records_sql", "evidence_id": "evidence-1", "version_id": "version-1", "source_file": "source.csv", "row_number": 7}},
			"limitations": []string{"Observed association is not ownership."},
		}
		packet := buildFactPacket(hybridQueryRequest{TenantID: "tenant-a", CollectionID: "case-a", Query: "show relationships", Limit: 100}, resp, enterprise)
		Expect(packet.ContractVersion).To(Equal(factPacketContractV1))
		Expect(packet.Facts[0].FactID).To(Equal("F1"))
		Expect(packet.Rows).To(HaveLen(maxFactPacketRows))
		Expect(packet.RowSetState).To(Equal("bounded"))
		Expect(packet.Relationships[0].RelationshipID).To(Equal("R1"))
		Expect(packet.Relationships[0].Strength).To(Equal("observed_association"))
		Expect(packet.Citations).To(ConsistOf(FactPacketCitationV1{CitationID: "C1", EvidenceID: "evidence-1", VersionID: "version-1", SourceFile: "source.csv", SourceRow: 7, ProofRole: "representative_evidence", Completeness: "representative"}))
	})

	It("accepts only schema-bound claims grounded in existing fact and citation IDs", func() {
		packet := FactPacketV1{ContractVersion: factPacketContractV1, Language: QueryLanguageV1{Tag: "en"}, Facts: []FactPacketFactV1{{FactID: "F1", Text: "Target 923001234567 has 4 matching CDR events.", CitationIDs: []string{"C1"}}}, Citations: []FactPacketCitationV1{{CitationID: "C1"}}}
		valid := NarrativeV1{ContractVersion: narrativeContractV1, Locale: "en", Direction: "ltr", DirectAnswer: "Target 923001234567 has 4 matching CDR events.", FactRefs: []string{"F1"}, CitationRefs: []string{"C1"}}
		Expect(validateNarrative(packet, valid)).To(Succeed())

		altered := valid
		altered.DirectAnswer = "Target 923001234567 has 400 matching CDR events."
		Expect(validateNarrative(packet, altered)).To(MatchError(ContainSubstring("ungrounded exact token")))

		omitted := valid
		omitted.DirectAnswer = "F1"
		Expect(validateNarrative(packet, omitted)).To(MatchError(ContainSubstring("ungrounded exact token")))

		fakeCitation := valid
		fakeCitation.CitationRefs = []string{"C99"}
		Expect(validateNarrative(packet, fakeCitation)).To(MatchError(ContainSubstring("unknown citation")))

		certainty := valid
		certainty.DirectAnswer = "Target 923001234567 owns the device."
		Expect(validateNarrative(packet, certainty)).To(MatchError(ContainSubstring("forbidden certainty")))

		inventedLimitation := valid
		inventedLimitation.Limitations = []string{"The source is probably incomplete."}
		Expect(validateNarrative(packet, inventedLimitation)).To(MatchError(ContainSubstring("unknown limitation")))
	})

	It("reconciles every narrative benchmark case and rejects injected evidence instructions", func() {
		payload, err := os.ReadFile("contracts/stim-answer-narrative-benchmark-v1.json")
		Expect(err).ToNot(HaveOccurred())
		var corpus struct {
			Cases []struct {
				ID, Locale string
				FactPacket struct {
					Facts       []FactPacketFactV1 `json:"facts"`
					Limitations []string           `json:"limitations"`
				} `json:"fact_packet"`
				RequiredFactRefs, RequiredCitationRefs, ForbiddenValues []string
			} `json:"cases"`
		}
		Expect(json.Unmarshal(payload, &corpus)).To(Succeed())
		Expect(corpus.Cases).To(HaveLen(8))
		for _, benchmark := range corpus.Cases {
			packet := FactPacketV1{ContractVersion: factPacketContractV1, Language: QueryLanguageV1{Tag: benchmark.Locale}, Facts: benchmark.FactPacket.Facts, Limitations: benchmark.FactPacket.Limitations}
			seen := map[string]bool{}
			for _, fact := range packet.Facts {
				for _, ref := range fact.CitationIDs {
					if !seen[ref] {
						packet.Citations = append(packet.Citations, FactPacketCitationV1{CitationID: ref})
						seen[ref] = true
					}
				}
			}
			narrative := deterministicNarrativeFallback(packet, "")
			Expect(narrative.ContractVersion).To(Equal(narrativeContractV1), benchmark.ID)
			for _, forbidden := range benchmark.ForbiddenValues {
				Expect(narrativePlainText(narrative)).ToNot(ContainSubstring(forbidden), benchmark.ID)
			}
		}
	})

	It("keeps unsupported media requests on the deterministic preflight path", func() {
		req := hybridQueryRequest{Query: "Transcribe and diarize this audio.", SynthesisModel: "qwen_qwen3-4b-instruct-2507"}
		Expect(assessQueryCapability(req.Query, req.Template).Status).To(Equal("unavailable"))
		Expect(shouldUseLanguageAssistance(req, planRuntimeQuery(req))).To(BeTrue())
		Expect(synthesisRequestTimeout(config{})).To(Equal(120 * time.Second))
	})
})
