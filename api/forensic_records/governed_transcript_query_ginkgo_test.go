package main

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Governed selected-evidence transcript queries", func() {
	const (
		evidenceID = "11111111-1111-1111-8111-111111111111"
		versionID  = "22222222-2222-2222-8222-222222222222"
	)

	row := func(id, contract, text string, locator map[string]any) map[string]any {
		return map[string]any{
			"artifact_id": id, "artifact_type": contract, "parent_artifact_id": "raw-parent-1",
			"evidence_id": evidenceID, "version_id": versionID, "source_file": "selected.wav",
			"passage_text": text, "citation_locator": locator,
		}
	}
	request := func(mode, term string, start, end *float64) hybridQueryRequest {
		return hybridQueryRequest{
			TenantID: "tenant-a", CollectionID: "case-a", EvidenceID: evidenceID, EvidenceVersionID: versionID,
			Template: "audio_transcript_search", TranscriptMode: mode, ExactTerm: term,
			StartSeconds: start, EndSeconds: end, MaxKBResults: 10,
		}
	}

	It("binds retrieval to tenant, case, selected evidence, and its current version", func() {
		Expect(derivedTextEvidenceSQL).To(ContainSubstring("items.current_version_id=artifacts.version_id"))
		Expect(derivedTextEvidenceSQL).To(ContainSubstring("artifacts.tenant_id=$1 AND artifacts.collection_id=$2"))
		Expect(derivedTextEvidenceSQL).To(ContainSubstring("artifacts.evidence_id = $4::uuid"))
		Expect(derivedTextEvidenceSQL).To(ContainSubstring("artifacts.version_id = $5::uuid"))
		Expect(currentEvidenceScopeSQL).To(ContainSubstring("tenant_id=$1 AND collection_id=$2 AND evidence_id=$3::uuid"))
		Expect(currentEvidenceScopeSQL).To(ContainSubstring("media_processing,version_id"))
		Expect(currentEvidenceScopeSQL).To(ContainSubstring("media_processing,metadata,result_states,asr"))
		Expect(currentEvidenceResultFamiliesSQL).To(ContainSubstring("version_id=$4::uuid"))
		Expect(currentEvidenceResultFamiliesSQL).NotTo(ContainSubstring("$4 = ''"))
	})

	It("preserves the complete governed ASR role-state vocabulary", func() {
		for _, state := range []string{"NOT_RUN", "PROCESSING", "COMPLETE_ZERO_RESULTS", "FAILED", "MODEL_REQUIRED", "UNAVAILABLE"} {
			response := hybridQueryResponse{Answer: map[string]any{}}
			applyTranscriptAnswerState(&response, map[string]any{"results": []any{}}, state)
			Expect(response.Answer["transcript_state"]).To(Equal(state))
			Expect(response.Answer["evidence_limitation"]).NotTo(BeEmpty())
		}
	})

	It("finds exact normalized English and Urdu phrases without fuzzy inference", func() {
		Expect(containsExactNormalizedTranscript("A clear, recorded phrase!", "clear recorded phrase")).To(BeTrue())
		Expect(containsExactNormalizedTranscript("یہ پاکستان کی آواز ہے۔", "پاکستان کی آواز")).To(BeTrue())
		Expect(containsExactNormalizedTranscript("unrelated wording", "related wording")).To(BeFalse())
	})

	It("returns raw transcript evidence first and preserves Roman Urdu parent lineage", func() {
		rows := []map[string]any{
			row("roman-1", "forensics.audio-roman-urdu-segment/v1", "meeting kal hogi", map[string]any{"start_seconds": 1.0, "end_seconds": 2.0}),
			row("raw-1", "forensics.audio-timestamp-segment/v1", "Meeting, kal hogi.", map[string]any{"start_seconds": 1.0, "end_seconds": 2.0}),
		}
		evidence := governedDerivedTextEvidence(rows, request("exact", "meeting kal hogi", nil, nil))
		Expect(evidence["result_state"]).To(Equal("COMPLETE_RESULTS"))
		results := evidenceResults(evidence)
		Expect(results).To(HaveLen(2))
		firstMetadata, _ := results[0]["metadata"].(map[string]any)
		secondMetadata, _ := results[1]["metadata"].(map[string]any)
		Expect(firstMetadata["artifact_type"]).To(Equal("forensics.audio-timestamp-segment/v1"))
		Expect(secondMetadata["parent_artifact_id"]).To(Equal("raw-parent-1"))
		Expect(results[0]["citation"]).To(ContainSubstring(evidenceID))
	})

	It("distinguishes no exact match from an empty completed transcript", func() {
		noMatch := governedDerivedTextEvidence(
			[]map[string]any{row("raw-1", "forensics.audio-timestamp-segment/v1", "different words", nil)},
			request("exact", "requested phrase", nil, nil),
		)
		Expect(noMatch["result_state"]).To(Equal("NO_EXACT_MATCH"))
		empty := governedDerivedTextEvidence(nil, request("source", "", nil, nil))
		Expect(empty["result_state"]).To(Equal("COMPLETE_ZERO_RESULTS"))
	})

	It("uses inclusive source-time intersection and reports missing timing explicitly", func() {
		start, end := 9.0, 12.0
		matching := governedDerivedTextEvidence(
			[]map[string]any{row("raw-1", "forensics.audio-timestamp-segment/v1", "bounded segment", map[string]any{"start_seconds": 5.0, "end_seconds": 9.0})},
			request("time_range", "", &start, &end),
		)
		Expect(matching["result_state"]).To(Equal("COMPLETE_RESULTS"))
		missing := governedDerivedTextEvidence(
			[]map[string]any{row("raw-2", "forensics.audio-timestamp-segment/v1", "untimed segment", map[string]any{})},
			request("time_range", "", &start, &end),
		)
		Expect(missing["result_state"]).To(Equal("TIMING_UNAVAILABLE"))
	})

	It("fails closed on malformed, conflicting, or implicitly broadened scopes", func() {
		bad := hybridQueryRequest{CollectionID: "case-a", QueryScope: queryEvidenceScope{Kind: "selected_evidence", EvidenceID: "not-a-uuid"}}
		Expect(applyRequestedEvidenceScope(&bad)).To(MatchError(ContainSubstring("exact evidence UUID")))

		conflict := hybridQueryRequest{CollectionID: "case-a", EvidenceID: evidenceID, QueryScope: queryEvidenceScope{Kind: "current_workspace"}}
		Expect(applyRequestedEvidenceScope(&conflict)).To(MatchError(ContainSubstring("cannot include evidence_id")))

		selected := hybridQueryRequest{CollectionID: "case-a", QueryScope: queryEvidenceScope{Kind: "selected_evidence", EvidenceID: evidenceID, EvidenceVersionID: versionID}}
		Expect(applyRequestedEvidenceScope(&selected)).To(Succeed())
		Expect(selected.EvidenceID).To(Equal(evidenceID))
		Expect(selected.EvidenceVersionID).To(Equal(versionID))
	})

	It("rejects a typed plan whose evidence scope diverges from its parameters", func() {
		plan := GovernedExecutionPlanV1{
			ContractVersion: executionPlanContractV1, PlanID: "plan-1", Mode: "single_operation",
			Workspace: ExecutionWorkspaceScopeV1{TenantID: "tenant-a", CaseID: "case-a", CollectionID: "case-a", SubjectID: "analyst-a"},
			Steps: []ExecutionStepV1{{
				StepID: "step-1", CapabilityID: "audio.transcript_search", OperationID: "audio.transcript_search",
				Parameters:             ExecutionParametersV1{EvidenceID: evidenceID, EvidenceVersionID: versionID, Limit: 10, Targets: []string{}},
				EvidenceScope:          ExecutionEvidenceScopeV1{CollectionID: "case-a", EvidenceID: evidenceID, EvidenceVersionID: "33333333-3333-3333-8333-333333333333"},
				ExpectedResultContract: "forensics.audio-timestamp-segment/v1", ResultKind: "rows", PresentationType: "transcript", CitationRequired: true,
			}},
			OutputContract: "forensics.audio-timestamp-segment/v1",
			ResourcePolicy: ExecutionResourcePolicyV1{ReadOnly: true, MaximumSteps: 1, MaximumRows: 100, MaximumOffset: 100, MaximumKBResults: 10, MaximumTargets: 8},
		}
		Expect(plan.ValidateShape()).To(MatchError(ContainSubstring("must match the typed evidence parameters")))
	})

	It("exposes citation locators as structured presentation data", func() {
		locator := decodedCitationLocator(`{"start_seconds":12.5,"end_seconds":14}`)
		Expect(locator).To(HaveKeyWithValue("start_seconds", 12.5))
		Expect(strings.TrimSpace(stringValueAny(locator["end_seconds"]))).To(Equal("14"))
	})
})
