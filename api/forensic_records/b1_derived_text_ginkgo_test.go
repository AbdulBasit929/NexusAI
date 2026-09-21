package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mudler/LocalAI/pkg/forensictext"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func b1Row(id, text string, start, end float64) map[string]any {
	return map[string]any{"artifact_id": id, "observation_id": "observation-" + id, "artifact_type": "forensics.audio-timestamp-segment/v1", "tenant_id": "test", "collection_id": "case", "evidence_id": "source", "version_id": "current", "run_id": "run", "language_lineage": "ur", "passage_text": text, "source_file": "synthetic.wav", "citation_locator": map[string]any{"start_seconds": start, "end_seconds": end}}
}

func b1Request(literal string) hybridQueryRequest {
	return hybridQueryRequest{TenantID: "test", CollectionID: "case", Template: "audio_transcript_search", MaxKBResults: 3, TextQuery: &forensictext.Query{LiteralText: literal, MatchSemantic: forensictext.PhraseContains}}
}

var _ = Describe("B1 shared derived text", func() {
	It("holds multiple literals and preserves multiline literals without treating contractions as quotes", func() {
		q, _ := forensictext.Extract("Doesn't this recording include what's needed?")
		Expect(q).To(BeNil())
		q, _ = forensictext.Extract("Find transcript \"clear\nphrase\"")
		Expect(q.LiteralText).To(Equal("clear\nphrase"))
		q, _ = forensictext.Extract(`Find transcript "clear" or "phrase"`)
		Expect(q.Validate()).NotTo(Succeed())
	})
	It("does not relabel a normalized OCR fallback as raw evidence for explicit literal search", func() {
		row := b1Row("a", "Report", 0, 1)
		row["artifact_type"] = "forensics.image-ocr-observation/v1"
		row["raw_text_unavailable"] = true
		req := b1Request("Report")
		req.Template = "image_ocr_search"
		result := governedDerivedTextEvidence([]map[string]any{row}, req)
		Expect(evidenceResults(result)).To(BeEmpty())
		Expect(result["result_state"]).To(Equal("SEARCH_INCOMPLETE"))
		req.TextQuery = nil
		req.ExactTerm = "Report"
		result = governedDerivedTextEvidence([]map[string]any{row}, req)
		Expect(evidenceResults(result)).To(HaveLen(1))
		metadata := evidenceResults(result)[0]["metadata"].(map[string]any)
		Expect(metadata["raw_text"]).To(BeEmpty())
		Expect(metadata["text_representation"]).To(Equal("ocr_normalized_fallback"))
	})
	It("resolves actual workspace results independently of selected-source readiness", func() {
		req := b1Request("clear")
		result := governedDerivedTextEvidence([]map[string]any{b1Row("a", "clear", 0, 1)}, req)
		Expect(derivedTranscriptResultState(req, result)).To(Equal("COMPLETE_RESULTS"))
		req.QueryScope.Kind = string(EvidenceScopeSelected)
		req.EvidenceASRResultState = "NOT_RUN"
		Expect(derivedTranscriptResultState(req, result)).To(Equal("COMPLETE_RESULTS"))
		Expect(derivedTranscriptResultState(req, map[string]any{"results": []map[string]any{}, "result_state": "COMPLETE_ZERO_RESULTS"})).To(Equal("NOT_RUN"))
		Expect(derivedTranscriptResultState(req, map[string]any{"results": []map[string]any{}, "result_state": "FAILED"})).To(Equal("FAILED"))
		resp := hybridQueryResponse{Answer: map[string]any{}}
		result["result_state"] = "SEARCH_INCOMPLETE"
		applyTranscriptAnswerState(&resp, result, "SEARCH_INCOMPLETE")
		Expect(resp.Answer["evidence_status"]).To(Equal("search_incomplete"))
		Expect(resp.Answer["processing_status"]).NotTo(Equal("not_processed"))
	})
	It("carries every supporting locator into the existing typed tool-result validator", func() {
		req := b1Request("clear phrase")
		req.MaxKBResults = 1
		rows := []map[string]any{b1Row("a", "clear", 0, 1), b1Row("b", "phrase", 1, 2)}
		evidence := governedDerivedTextEvidence(rows, req)
		plan := GovernedExecutionPlanV1{PlanID: "plan-text", Steps: []ExecutionStepV1{{StepID: "text", OperationID: "audio.transcript_search", ExpectedResultContract: "forensics.enterprise-response/v1", CitationRequired: true}}, ResourcePolicy: ExecutionResourcePolicyV1{MaximumRows: 20}}
		resp := hybridQueryResponse{Template: req.Template, ExecutionPlan: &plan, Evidence: evidence, Enterprise: map[string]any{"result_state": string(ResultStateResultsPresent), "provenance": enterpriseProvenance(req, nil, evidence, nil)}}
		results, warnings := buildSharedToolResults(req, resp, nil)
		Expect(warnings).To(BeEmpty())
		Expect(results).To(HaveLen(1))
		result := results[0]
		Expect(result.RowCount).To(Equal(2))
		Expect(result.Citations).To(HaveLen(2))
		Expect(result.Observations).To(HaveLen(1))
		Expect(result.Validate(true, "case")).To(Succeed())
		for _, c := range result.Citations {
			Expect(c.ArtifactID).NotTo(BeEmpty())
			Expect(c.Locator).To(ContainSubstring("start_seconds"))
		}
		result.Citations = result.Citations[:1]
		Expect(result.Validate(true, "case")).NotTo(Succeed())
		result = results[0]
		result.ResultState = ResultStateNotProcessed
		Expect(result.Validate(true, "case")).NotTo(Succeed())
		result = results[0]
		result.SearchCompleteness = "INCOMPLETE"
		result.ResultState = ResultStateNoMatchForFilter
		result.Rows = nil
		result.RowCount = 0
		Expect(result.Validate(true, "case")).NotTo(Succeed())
	})
	It("keeps overlapping third segments and out-of-range contributors from creating a phrase", func() {
		rows := []map[string]any{b1Row("a", "clear", 0, 1), b1Row("b", "phrase", 1, 2), b1Row("c", "different", 1, 2)}
		Expect(evidenceResults(governedDerivedTextEvidence(rows, b1Request("clear phrase")))).To(BeEmpty())
		req := b1Request("clear phrase")
		start, end := 1.5, 2.0
		req.StartSeconds = &start
		req.EndSeconds = &end
		Expect(evidenceResults(governedDerivedTextEvidence(rows[:2], req))).To(BeEmpty())
	})
	It("keeps multibyte previews valid without changing raw text", func() {
		text := strings.Repeat("رپورٹ ", 500)
		Expect(utf8.ValidString(derivedTextPreview(text, 2000))).To(BeTrue())
		Expect(len(derivedTextPreview(text, 2000))).To(BeNumerically("<=", 2000))
	})
	DescribeTable("conservative semantics have independently specified outcomes", func(text, literal string, semantic forensictext.Semantic, want bool) {
		Expect(forensictext.Match(text, forensictext.Query{LiteralText: literal, MatchSemantic: semantic})).To(Equal(want))
	},
		Entry("whole field", "ABC DEF", "ABC DEF", forensictext.ExactValue, true),
		Entry("whole field is not contains", "ABC DEF", "ABC", forensictext.ExactValue, false),
		Entry("phrase", "a clear phrase here", "clear phrase", forensictext.ExactPhrase, true),
		Entry("phrase boundary", "unrelated wording", "related", forensictext.ExactPhrase, false),
		Entry("inside token", "unrelated wording", "related", forensictext.PhraseContains, true),
		Entry("punctuation preserved", "clear, phrase", "clear phrase", forensictext.PhraseContains, false),
		Entry("newlines and Unicode space", "clear\n\u00a0phrase", "clear phrase", forensictext.PhraseContains, true),
		Entry("Urdu", "یہ ہماری نئی رپورٹ ہے۔", "نئی رپورٹ", forensictext.PhraseContains, true),
		Entry("Urdu absent", "یہ ہماری نئی رپورٹ ہے۔", "پرانی رپورٹ", forensictext.PhraseContains, false),
		Entry("NFC", "cafe\u0301 notes", "café", forensictext.PhraseContains, true),
		Entry("case preserved", "Report", "report", forensictext.PhraseContains, false),
		Entry("joiner preserved", "ab\u200dcd", "abcd", forensictext.PhraseContains, false),
		Entry("all tokens unordered", "alpha beta gamma", "gamma alpha", forensictext.TokenSearch, true),
		Entry("missing token", "alpha beta", "alpha gamma", forensictext.TokenSearch, false),
		Entry("one letter", "پاکستان", "پ", forensictext.PhraseContains, false),
		Entry("two letter Urdu word", "یہ رپورٹ", "یہ", forensictext.PhraseContains, true),
		Entry("punctuation only", "a!?b", "!?", forensictext.PhraseContains, false),
	)
	DescribeTable("preserves exact identifiers", func(value string) {
		Expect(forensictext.Match(value, forensictext.Query{LiteralText: value, MatchSemantic: forensictext.ExactValue})).To(BeTrue())
		Expect(forensictext.Match("prefix "+value, forensictext.Query{LiteralText: value, MatchSemantic: forensictext.ExactValue})).To(BeFalse())
	}, Entry("phone", "03001234567"), Entry("plate", "AB12CDE"), Entry("UUID", "11111111-1111-4111-8111-111111111111"), Entry("IP", "192.0.2.1"), Entry("structured ID", "CASE-0042"))
	DescribeTable("routes literal intent without inference", func(question, template, literal string) {
		start := time.Now()
		req := bindDerivedTextQuery(hybridQueryRequest{Query: question, SynthesisModel: "qwen_qwen3-4b-instruct-2507"})
		plan := planRuntimeQuery(req)
		Expect(req.TextQuery).NotTo(BeNil())
		Expect(req.TextQuery.LiteralText).To(Equal(literal))
		Expect(plan.Template).To(Equal(template))
		Expect(shouldUseLanguageAssistance(req, plan)).To(BeFalse())
		Expect(time.Since(start)).To(BeNumerically("<", time.Second))
	}, Entry("short Urdu", `Find this transcript phrase "نئی رپورٹ"`, "audio_transcript_search", "نئی رپورٹ"), Entry("full Urdu", `Find the transcript phrase "یہ ہماری نئی رپورٹ ہے۔"`, "audio_transcript_search", "یہ ہماری نئی رپورٹ ہے۔"), Entry("English", `Does this recording contain "clear phrase"?`, "audio_transcript_search", "clear phrase"), Entry("OCR", `Find OCR text "Clear Report"`, "image_ocr_search", "Clear Report"), Entry("document", `Find document phrase "Annual Report"`, "document_search", "Annual Report"))
	It("clarifies an unresolvable literal without source retrieval or inference", func() {
		req := bindDerivedTextQuery(hybridQueryRequest{Query: `Search for "new report"`, SynthesisModel: "qwen_qwen3-4b-instruct-2507"})
		Expect(planRuntimeQuery(req).Intent).To(Equal(intentClarify))
		Expect(shouldUseLanguageAssistance(req, planRuntimeQuery(req))).To(BeFalse())
	})
	It("never treats an unclassified typed literal as source", func() {
		req := hybridQueryRequest{Template: "document_search", ExactTerm: "absent", MaxKBResults: 3}
		result := governedDerivedTextEvidence([]map[string]any{b1Row("a", "unrelated", 0, 1)}, req)
		Expect(result["result_state"]).To(Equal("INVALID_REQUEST"))
		Expect(evidenceResults(result)).To(BeEmpty())
	})
	It("preserves legacy normalization and explicit caller semantics", func() {
		req := bindDerivedTextQuery(hybridQueryRequest{Query: `Find transcript "clear phrase"`, Template: "audio_transcript_search", TranscriptMode: "exact", ExactTerm: "clear phrase", MaxKBResults: 3})
		Expect(req.TextQuery).To(BeNil())
		Expect(evidenceResults(governedDerivedTextEvidence([]map[string]any{b1Row("a", "CLEAR, phrase!", 0, 1)}, req))).To(HaveLen(1))
	})
	DescribeTable("joins two through four touching raw observations", func(parts []string, literal string) {
		rows := []map[string]any{}
		for i, part := range parts {
			rows = append(rows, b1Row(fmt.Sprint(i), part, float64(i), float64(i+1)))
		}
		req := b1Request(literal)
		req.MaxKBResults = 1
		result := governedDerivedTextEvidence(rows, req)
		Expect(result["result_state"]).To(Equal("COMPLETE_RESULTS"))
		Expect(evidenceResults(result)).To(HaveLen(len(parts)))
		for _, r := range evidenceResults(result) {
			m := r["metadata"].(map[string]any)
			Expect(m["contributing_observations"]).To(HaveLen(len(parts)))
			Expect(m["match_start_seconds"]).To(Equal(0.0))
			Expect(m["match_end_seconds"]).To(Equal(float64(len(parts))))
			Expect(m["raw_text"]).NotTo(BeEmpty())
			Expect(r["citation"]).NotTo(BeEmpty())
		}
		encoded, err := json.Marshal(result)
		Expect(err).NotTo(HaveOccurred())
		var reopened map[string]any
		Expect(json.Unmarshal(encoded, &reopened)).To(Succeed())
		Expect(evidenceResults(reopened)).To(HaveLen(len(parts)))
	}, Entry("English", []string{"a clear", "phrase here"}, "clear phrase"), Entry("Urdu", []string{"یہ نئی", "رپورٹ ہے"}, "نئی رپورٹ"), Entry("three", []string{"one", "two", "three"}, "one two three"), Entry("four", []string{"one", "two", "three", "four"}, "one two three four"))
	DescribeTable("rejects cross-scope and cross-representation joins", func(field, value string) {
		a, b := b1Row("a", "clear", 0, 1), b1Row("b", "phrase", 1, 2)
		b[field] = value
		result := governedDerivedTextEvidence([]map[string]any{a, b}, b1Request("clear phrase"))
		Expect(evidenceResults(result)).To(BeEmpty())
	}, Entry("run", "run_id", "other"), Entry("evidence", "evidence_id", "other"), Entry("version", "version_id", "old"), Entry("tenant", "tenant_id", "other"), Entry("case", "collection_id", "other"), Entry("representation", "artifact_type", "forensics.audio-roman-urdu-segment/v1"), Entry("language", "language_lineage", "en"))
	It("rejects ambiguous timing and does not assert exhaustive absence", func() {
		a, b := b1Row("a", "clear", 0, 2), b1Row("b", "phrase", 1, 3)
		result := governedDerivedTextEvidence([]map[string]any{a, b}, b1Request("clear phrase"))
		Expect(evidenceResults(result)).To(BeEmpty())
		Expect(result["result_state"]).To(Equal("SEARCH_INCOMPLETE"))
		b["citation_locator"] = map[string]any{}
		Expect(governedDerivedTextEvidence([]map[string]any{a, b}, b1Request("clear phrase"))["result_state"]).To(Equal("SEARCH_INCOMPLETE"))
	})
	It("keeps OCR and native document literals on raw observations and preserves locators", func() {
		for template, contract := range map[string]string{"image_ocr_search": "forensics.image-ocr-observation/v1", "document_search": "forensics.document-native-text-passage/v1"} {
			req := b1Request("Annual Report")
			req.Template = template
			row := b1Row("a", "Annual Report 2026", 0, 1)
			row["artifact_type"] = contract
			row["citation_locator"] = map[string]any{"page": 2, "bbox": []int{1, 2, 3, 4}}
			result := governedDerivedTextEvidence([]map[string]any{row}, req)
			Expect(evidenceResults(result)).To(HaveLen(1))
			Expect(result["result_state"]).To(Equal("COMPLETE_RESULTS"))
			req.TextQuery.LiteralText = "Absent Phrase"
			Expect(governedDerivedTextEvidence([]map[string]any{row}, req)["result_state"]).To(Equal("NO_EXACT_MATCH"))
		}
	})
	It("preserves raw authority and explicit Roman derivative lineage", func() {
		raw := b1Row("a", "یہ نئی رپورٹ ہے", 0, 1)
		roman := b1Row("b", "yeh nai report hai", 0, 1)
		roman["artifact_type"] = "forensics.audio-roman-urdu-segment/v1"
		roman["parent_artifact_id"] = "a"
		req := b1Request("nai report")
		req.TextQuery.Representation = "roman_derivative"
		result := governedDerivedTextEvidence([]map[string]any{raw, roman}, req)
		Expect(evidenceResults(result)).To(HaveLen(1))
		m := evidenceResults(result)[0]["metadata"].(map[string]any)
		Expect(m["parent_artifact_id"]).To(Equal("a"))
		Expect(m["text_representation"]).To(Equal("roman_derivative"))
		Expect(raw["passage_text"]).To(Equal("یہ نئی رپورٹ ہے"))
	})
	It("discloses positive result truncation and incomplete zero", func() {
		req := b1Request("clear")
		req.MaxKBResults = 1
		result := governedDerivedTextEvidence([]map[string]any{b1Row("a", "clear", 0, 1), b1Row("b", "clear", 1, 2)}, req)
		Expect(result["result_state"]).To(Equal("RESULTS_TRUNCATED"))
		Expect(result["search_completeness"]).To(Equal("TRUNCATED"))
		for _, rows := range [][]map[string]any{nil, evidenceResults(result)} {
			r := hybridQueryResponse{Answer: map[string]any{}, Evidence: map[string]any{"result_state": "SEARCH_INCOMPLETE", "results": rows}}
			Expect(enterpriseResultState(r, nil)).NotTo(Equal(EnterpriseResultStateNoMatchForFilter))
			Expect(enterpriseExecutiveAnswer(req, r, nil)).To(ContainSubstring("incomplete"))
		}
	})
	It("serializes typed semantics through execution parameters", func() {
		req := b1Request("نئی رپورٹ")
		parameters := executionParametersFromRequest(req)
		bytes, err := json.Marshal(parameters)
		Expect(err).NotTo(HaveOccurred())
		var decoded ExecutionParametersV1
		Expect(json.Unmarshal(bytes, &decoded)).To(Succeed())
		replayed := requestWithExecutionParameters(hybridQueryRequest{}, decoded)
		Expect(replayed.TextQuery).To(Equal(req.TextQuery))
	})
	It("rejects model literal rewriting and unknown schema fields", func() {
		template, _ := queryTemplateByName("audio_transcript_search")
		p := LanguageAssistanceProposalV1{ContractVersion: languageAssistanceProposalV1, OperationID: template.OperationID, FamilyID: template.FamilyID, OutputContract: queryUnderstandingContractV1, Confidence: 0.9, Parameters: LanguageAssistanceParametersV1{TextQuery: &forensictext.Query{LiteralText: "نئی رپورٹ", MatchSemantic: forensictext.PhraseContains}}}
		req := hybridQueryRequest{Query: "اس ریکارڈنگ میں نئی رپورٹ تلاش کریں"}
		accepted, err := applyLanguageAssistanceProposal(req, p)
		Expect(err).NotTo(HaveOccurred())
		Expect(accepted.TextQuery.LiteralText).To(Equal("نئی رپورٹ"))
		p.Parameters.TextQuery.LiteralText = "رپورٹ نئی"
		_, err = applyLanguageAssistanceProposal(req, p)
		Expect(err).To(HaveOccurred())
		_, err = decodeLanguageAssistanceProposal([]byte(`{"parameters":{"text_query":{"match_semantic":"PHRASE_CONTAINS","literal_text":"ok","unknown":true}}}`))
		Expect(err).To(HaveOccurred())
	})
	It("does not interpret source-time anaphora as literals", func() {
		q, _ := forensictext.Extract("When did they say that?")
		Expect(q).To(BeNil())
		Expect(strings.Contains(string(forensictext.SourceTime), "SOURCE")).To(BeTrue())
	})
})
