package main

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func nxa1DirectFixture() (hybridQueryRequest, QueryUnderstandingV1, ResolvedCapabilityV1, GovernedExecutionPlanV1, InvestigationContextV1) {
	template, ok := queryTemplateByName("frequent_contacts")
	Expect(ok).To(BeTrue(), "frequent_contacts operation fixture is missing")
	req := hybridQueryRequest{TenantID: "tenant-a", UserID: "analyst-a", CollectionID: "case-a", Query: "Who did 923001110001 contact most?", Target: "923001110001", Limit: 25, MaxKBResults: 5}
	understanding := adaptLegacyRuntimePlan(req, runtimePlan{Intent: intentRecords, Template: template.Name, Target: req.Target, Source: "deterministic_identifier_first", Confidence: 1}, template)
	capability := capabilityFromTemplate(template, CapabilityProjectionContextV1{
		Authorized: true, RuntimeAvailable: true,
		FamilyStates: map[string]WorkspaceCapabilityStateV1{"cdr": {IndexedRecords: 10, RegisteredEvidence: 1, NormalizedEvidence: 1}},
		ModelRoles:   map[string]bool{}, RequiredModelRolesByCapability: map[string][]string{}, MaturityByCapability: map[string]string{template.OperationID: "operational"},
	})
	Expect(capability.Availability.Executable).To(BeTrue())
	plan, err := buildSingleCapabilityPlan("request-a", req, understanding, capability)
	Expect(err).NotTo(HaveOccurred())
	snapshot := buildCapabilitySnapshot("snapshot-a", plan.Workspace, []ResolvedCapabilityV1{capability})
	return req, understanding, capability, plan, investigationContextFromRequest(req, understanding, snapshot)
}

var _ = Describe("NX-A1 shared typed execution foundation", func() {
	It("reuses the accepted contracts", func() {
		var plan QueryPlanV1 = GovernedExecutionPlanV1{ContractVersion: executionPlanContractV1}
		var step PlanStepV1 = ExecutionStepV1{StepID: "step-1"}
		var operation OperationContractV1 = ResolvedCapabilityV1{OperationID: "cdr.frequent_contacts"}
		var budget ExecutionBudgetV1 = ExecutionResourcePolicyV1{ReadOnly: true}
		var envelope AnswerEnvelopeV1 = EnterpriseResponseV1{ContractVersion: "forensics.enterprise-response/v1"}
		Expect(plan.ContractVersion).To(Equal(executionPlanContractV1))
		Expect(step.StepID).To(Equal("step-1"))
		Expect(operation.OperationID).To(Equal("cdr.frequent_contacts"))
		Expect(budget.ReadOnly).To(BeTrue())
		Expect(envelope.ContractVersion).To(Equal("forensics.enterprise-response/v1"))
	})

	It("preserves exact identifiers and authoritative scope", func() {
		req, understanding, _, _, context := nxa1DirectFixture()
		Expect(context.Validate()).To(Succeed())
		Expect(context.Workspace).To(Equal(ExecutionWorkspaceScopeV1{TenantID: req.TenantID, CaseID: req.CollectionID, CollectionID: req.CollectionID, SubjectID: req.UserID}))
		Expect(context.Evidence.CollectionID).To(Equal(req.CollectionID))
		Expect(context.ActiveEntities).To(HaveLen(1))
		Expect(context.ActiveEntities[0].Original).To(Equal("923001110001"))
		Expect(context.ActiveEntities[0].Normalized).To(Equal("923001110001"))
		Expect(understanding.Intent).To(Equal(QueryIntentRank))
		widened := context
		widened.Evidence.CollectionID = "case-b"
		Expect(widened.Validate()).To(MatchError(ContainSubstring("authorized collection")))
	})

	It("separates calendar timestamps from media source time", func() {
		start, end := 12.5, 18.0
		media := TimeScopeV1{Domain: TimeDomainMediaSource, EvidenceID: "evidence-a", StartSeconds: &start, EndSeconds: &end, Origin: QueryOriginExplicitScope}
		Expect(media.Validate()).To(Succeed())
		media.From = "2026-08-28T00:00:00Z"
		Expect(media.Validate()).To(MatchError(ContainSubstring("calendar bounds")))
		calendar := TimeScopeV1{Domain: TimeDomainRecordTimestamp, From: "2026-08-28T00:00:00Z", StartSeconds: &start, Timezone: "Asia/Karachi"}
		Expect(calendar.Validate()).To(MatchError(ContainSubstring("media source-time")))
	})

	It("preserves capability readiness states", func() {
		workspace := ExecutionWorkspaceScopeV1{TenantID: "tenant-a", CaseID: "case-a", CollectionID: "case-a", SubjectID: "analyst-a"}
		makeCapability := func(id string, availability CapabilityAvailabilityV1) ResolvedCapabilityV1 {
			return ResolvedCapabilityV1{CapabilityID: id, OperationID: id, Availability: availability}
		}
		snapshot := buildCapabilitySnapshot("snapshot-a", workspace, []ResolvedCapabilityV1{
			makeCapability("test.complete", CapabilityAvailabilityV1{Implemented: true, RuntimeAvailable: true, DataAvailable: true, Authorized: true, ModelAvailable: true}),
			makeCapability("test.processing", CapabilityAvailabilityV1{Implemented: true, RuntimeAvailable: true, Authorized: true, ModelAvailable: true, ReasonCodes: []CapabilityReasonCode{CapabilityReasonProcessingIncomplete}}),
			makeCapability("test.not-run", CapabilityAvailabilityV1{Implemented: true, RuntimeAvailable: true, Authorized: true, ModelAvailable: true, ReasonCodes: []CapabilityReasonCode{CapabilityReasonMissingData}}),
			makeCapability("test.unavailable", CapabilityAvailabilityV1{Implemented: true, Authorized: true, ModelAvailable: true, ReasonCodes: []CapabilityReasonCode{CapabilityReasonRuntimeUnavailable}}),
		})
		want := map[string]DataReadinessState{"test.complete": DataReadinessCompleteResults, "test.processing": DataReadinessProcessing, "test.not-run": DataReadinessNotRun, "test.unavailable": DataReadinessUnavailable}
		for _, entry := range snapshot.Entries {
			Expect(entry.Readiness).To(Equal(want[entry.Operation.OperationID]))
		}
		Expect(DataReadinessCompleteZeroResults).NotTo(Equal(DataReadinessCompleteResults))
		Expect(DataReadinessFailed).NotTo(Equal(DataReadinessUnavailable))
	})

	It("validates the direct fast path and fails closed on policy violations", func() {
		_, _, capability, plan, context := nxa1DirectFixture()
		result := validateGovernedExecutionPlan(plan, []ResolvedCapabilityV1{capability}, context, nxa1ExecutionBudgetCeiling())
		Expect(result.Valid).To(BeTrue(), "%+v", result)
		Expect(result.EstimatedCostClass).To(Equal("direct_operation"))
		unauthorized := context
		unauthorized.Permissions = []string{"query:view"}
		result = validateGovernedExecutionPlan(plan, []ResolvedCapabilityV1{capability}, unauthorized, nxa1ExecutionBudgetCeiling())
		Expect(result.Valid).To(BeFalse())
		Expect(result.ScopeAuthorized).To(BeFalse())
		overBudget := plan
		overBudget.ResourcePolicy.MaximumRows++
		result = validateGovernedExecutionPlan(overBudget, []ResolvedCapabilityV1{capability}, context, nxa1ExecutionBudgetCeiling())
		Expect(result.Valid).To(BeFalse())
		Expect(result.BudgetAccepted).To(BeFalse())
		unapproved := capability
		unapproved.ImplementationKey = "model_authored_sql"
		result = validateGovernedExecutionPlan(plan, []ResolvedCapabilityV1{unapproved}, context, nxa1ExecutionBudgetCeiling())
		Expect(result.Valid).To(BeFalse())
		Expect(result.AuthorityPreserved).To(BeFalse())
	})

	It("rejects dependency cycles", func() {
		req, understanding, first, _, _ := nxa1DirectFixture()
		second := first
		second.CapabilityID, second.OperationID, second.Template = "cdr.temporal_activity", "cdr.temporal_activity", "temporal_activity"
		understanding.CandidateCapabilities, understanding.Intent = []string{first.CapabilityID, second.CapabilityID}, QueryIntentCorrelate
		parameters := executionParametersFromRequest(req)
		plan := GovernedExecutionPlanV1{
			ContractVersion: executionPlanContractV1, PlanID: "cycle-plan", Mode: "bounded_composition",
			Workspace: ExecutionWorkspaceScopeV1{TenantID: req.TenantID, CaseID: req.CollectionID, CollectionID: req.CollectionID, SubjectID: req.UserID},
			Steps: []ExecutionStepV1{
				{StepID: "a", CapabilityID: first.CapabilityID, OperationID: first.OperationID, Dependencies: []string{"b"}, Parameters: parameters, EvidenceScope: ExecutionEvidenceScopeV1{CollectionID: req.CollectionID}, ExpectedResultContract: first.ResultContract, ResultKind: first.ResultKind, PresentationType: first.PresentationType, CitationRequired: first.CitationRequired},
				{StepID: "b", CapabilityID: second.CapabilityID, OperationID: second.OperationID, Dependencies: []string{"a"}, Parameters: parameters, EvidenceScope: ExecutionEvidenceScopeV1{CollectionID: req.CollectionID}, ExpectedResultContract: second.ResultContract, ResultKind: second.ResultKind, PresentationType: second.PresentationType, CitationRequired: second.CitationRequired},
			},
			OutputContract: compositionResultContract,
			ResourcePolicy: ExecutionResourcePolicyV1{ReadOnly: true, MaximumSteps: 2, MaximumRows: 100, MaximumOffset: 100000, MaximumKBResults: 20, MaximumTargets: 8, MaximumIntermediateRows: 200, MaximumRetrievalChunks: 12, MaximumModelCalls: 1, TimeoutMilliseconds: 60000},
		}
		context := investigationContextFromRequest(req, understanding, buildCapabilitySnapshot("snapshot-cycle", plan.Workspace, []ResolvedCapabilityV1{first, second}))
		result := validateGovernedExecutionPlan(plan, []ResolvedCapabilityV1{first, second}, context, nxa1ExecutionBudgetCeiling())
		Expect(result.Valid).To(BeFalse())
		Expect(result.DependenciesValid).To(BeFalse())
	})

	It("preserves zero, filter no-match, and not-processed separately", func() {
		base := ToolResultV1{ContractVersion: toolResultContractV1, InvocationID: "invocation-a", PlanID: "plan-a", StepID: "step-a", OperationID: "cdr.frequent_contacts", ResultContract: "forensics.enterprise-response/v1", Authority: AuthorityDeterministicFact, Fields: map[string]TypedIntermediateFieldV1{}, Facts: []ToolFactV1{}, Observations: []ObservationPacketV1{}, Rows: []map[string]any{}, Citations: []CitationV1{}, Limitations: []string{}}
		zero := base
		zero.ExecutionStatus, zero.ResultState = ExecutionStatusCompleted, ResultStateCompleteZero
		Expect(zero.Validate(true, "case-a")).To(Succeed())
		noMatch := base
		noMatch.ExecutionStatus, noMatch.ResultState = ExecutionStatusCompleted, ResultStateNoMatchForFilter
		Expect(noMatch.Validate(true, "case-a")).To(Succeed())
		notProcessed := base
		notProcessed.ExecutionStatus, notProcessed.ResultState = ExecutionStatusSkipped, ResultStateNotProcessed
		Expect(notProcessed.Validate(true, "case-a")).To(Succeed())
		Expect(zero.ResultState).NotTo(Equal(noMatch.ResultState))
		Expect(zero.ResultState).NotTo(Equal(notProcessed.ResultState))
	})

	It("prevents observations from being promoted to facts", func() {
		citation := CitationV1{ID: "citation-a", TenantID: "tenant-a", CollectionID: "case-a", EvidenceID: "evidence-a", VersionID: "version-a", Locator: "frame:12", SourceName: "video.mp4"}
		packet := ObservationPacketV1{ContractVersion: observationPacketContractV1, PacketID: "packet-a", RequestID: "request-a", CaseID: "case-a", CollectionID: "case-a", OperationID: "anpr.observe", Observations: []ObservationV1{{ID: "observation-a", Kind: "plate_candidate", Authority: AuthorityModelObservation, Value: "ABC-123", CitationIDs: []string{"citation-a"}, Limitations: []string{"OCR candidate only"}}}, Citations: []CitationV1{citation}, Limitations: []string{}}
		Expect(packet.Validate()).To(Succeed())
		packet.Observations[0].Authority = AuthorityDeterministicFact
		Expect(packet.Validate()).To(MatchError(ContainSubstring("cannot contain deterministic facts")))
	})

	It("keeps the NX-B1 video timeline in model-observation authority", func() {
		step := ExecutionStepV1{StepID: "step-video", OperationID: "video.timeline", ExpectedResultContract: "forensics.enterprise-response/v1", CitationRequired: true}
		plan := GovernedExecutionPlanV1{PlanID: "plan-video", Steps: []ExecutionStepV1{step}, ResourcePolicy: ExecutionResourcePolicyV1{MaximumRows: 20}}
		response := hybridQueryResponse{
			ExecutionPlan: &plan, Telemetry: QueryTelemetry{RequestID: "request-video"},
			Records: map[string]any{"video_timeline": []map[string]any{
				{"artifact_id": "artifact-a", "artifact_type": "forensics.image-observation/v1", "evidence_id": "evidence-a", "version_id": "version-a"},
				{"artifact_id": "artifact-b", "artifact_type": "forensics.image-observation/v1", "evidence_id": "evidence-b", "version_id": "version-b"},
			}},
			Enterprise: map[string]any{
				"result_state": string(ResultStateResultsPresent),
				"provenance": []map[string]any{
					{"evidence_id": "evidence-a", "version_id": "version-a", "source_file": "video-a.mp4", "source_time_seconds": 12.5},
					{"evidence_id": "evidence-b", "version_id": "version-b", "source_file": "video-b.mp4", "source_time_seconds": 18.5},
				},
				"limitations": []string{"Retained model observations only."},
			},
		}
		results, warnings := buildSharedToolResults(hybridQueryRequest{TenantID: "tenant-a", CollectionID: "case-a", Template: "video_timeline"}, response, nil)
		Expect(warnings).To(BeEmpty())
		Expect(results).To(HaveLen(1))
		Expect(results[0].Authority).To(Equal(AuthorityModelObservation))
		Expect(results[0].Facts).To(BeEmpty())
		Expect(results[0].Observations).To(HaveLen(1))
		Expect(results[0].Observations[0].Observations).To(HaveLen(2))
		Expect(results[0].Observations[0].Observations[0].Authority).To(Equal(AuthorityModelObservation))
		Expect(results[0].Observations[0].Observations[0].CitationIDs).To(Equal([]string{"tool-citation-1"}))
		Expect(results[0].Observations[0].Observations[1].CitationIDs).To(Equal([]string{"tool-citation-2"}))
	})

	It("rejects unrestricted SQL and arbitrary execution fields", func() {
		plan := map[string]any{"contract_version": "forensics.query-plan/v1", "case_id": "case-a", "intent": "cdr.frequent_contacts", "raw_sql": "SELECT * FROM forensic.records"}
		_, _, err := normalizeV1QueryPlan("case-a", "request-a", plan)
		Expect(err).To(MatchError(ContainSubstring("raw_sql")))
		plan = map[string]any{"contract_version": "forensics.query-plan/v1", "case_id": "case-a", "intent": "cdr.frequent_contacts", "filters": []any{map[string]any{"command": "run arbitrary tool"}}}
		_, _, err = normalizeV1QueryPlan("case-a", "request-a", plan)
		Expect(err).To(MatchError(ContainSubstring("command")))
	})

	It("maps a visual action and Ask to the same operation", func() {
		_, understanding, capability, plan, _ := nxa1DirectFixture()
		action := map[string]any{"contract_version": "forensics.query-plan/v1", "tenant_id": "tenant-a", "case_id": "case-a", "collection_id": "case-a", "intent": capability.OperationID, "families": []any{"communications_cdr"}, "entities": []any{map[string]any{"type": "phone", "value": "923001110001"}}, "limit": float64(25), "clarifications": []any{}}
		adapted, early, err := normalizeV1QueryPlan("case-a", "request-ui", action)
		Expect(err).NotTo(HaveOccurred())
		Expect(early).To(BeNil())
		Expect(stringValueAny(adapted["template"])).To(Equal(capability.Template))
		Expect(plan.Steps[0].OperationID).To(Equal(capability.OperationID))
		Expect(understanding.CandidateCapabilities).To(Equal([]string{capability.OperationID}))
	})

	It("preserves exact multilingual identifiers", func() {
		template, ok := queryTemplateByName("temporal_activity")
		Expect(ok).To(BeTrue())
		queries := []struct{ query, lang string }{{"10 july 2026 ko 923001234567 ki CDR activity dikhao", "ur-Latn"}, {"10 جولائی 2026 کو 923001234567 کی کال سرگرمی دکھائیں", "ur"}, {"10 جولائی 2026 کو 923001234567 کی CDR سرگرمی دکھائیں", "mixed"}, {"Show 923001234567 کی CDR activity", "mixed"}}
		for _, test := range queries {
			req := hybridQueryRequest{TenantID: "tenant-a", UserID: "analyst-a", CollectionID: "case-a", Query: test.query, Target: "923001234567", Limit: 20}
			understanding := adaptLegacyRuntimePlan(req, runtimePlan{Intent: intentRecords, Template: template.Name, Target: req.Target, Source: "deterministic", Confidence: 1}, template)
			Expect(understanding.Language.Tag).To(Equal(test.lang))
			Expect(understanding.Entities).To(HaveLen(1))
			Expect(understanding.Entities[0].Original).To(Equal("923001234567"))
			Expect(understanding.Entities[0].Normalized).To(Equal("923001234567"))
		}
	})

	It("bounds large data and preserves answer-envelope state", func() {
		_, _, capability, plan, context := nxa1DirectFixture()
		plan.Steps[0].Parameters.Limit, plan.ResourcePolicy.MaximumRows = maxHybridLimit, maxHybridLimit
		Expect(validateGovernedExecutionPlan(plan, []ResolvedCapabilityV1{capability}, context, nxa1ExecutionBudgetCeiling()).Valid).To(BeTrue())
		plan.Steps[0].Parameters.Limit++
		plan.ResourcePolicy.MaximumRows++
		validation := validateGovernedExecutionPlan(plan, []ResolvedCapabilityV1{capability}, context, nxa1ExecutionBudgetCeiling())
		Expect(validation.Valid).To(BeFalse())
		Expect(validation.BudgetAccepted).To(BeFalse())
		payload, err := json.Marshal(AnswerEnvelopeV1{ContractVersion: "forensics.enterprise-response/v1", RequestID: "request-a", CaseID: "case-a", CollectionID: "case-a", ResultState: EnterpriseResultStateCompleteZero, ToolResults: []ToolResultV1{}})
		Expect(err).NotTo(HaveOccurred())
		Expect(string(payload)).To(ContainSubstring(`"result_state":"complete_zero_results"`))
	})

	It("derives a future operation descriptor from one registration", func() {
		template := queryTemplateCatalogEntry{queryTemplate: queryTemplate{Name: "hypothetical_family_operation", RecordTypes: []string{"generic"}, Route: "records"}, OperationID: "generic.hypothetical_operation", FamilyID: "generic_tabular", Inputs: []queryTemplateInput{{Name: "target", Required: true}}, ScopeMode: "target_required", Presentation: "table"}
		capability := capabilityFromTemplate(template, CapabilityProjectionContextV1{Authorized: true, RuntimeAvailable: true, FamilyStates: map[string]WorkspaceCapabilityStateV1{"generic_tabular": {IndexedRecords: 1}}, ModelRoles: map[string]bool{}, RequiredModelRolesByCapability: map[string][]string{}, MaturityByCapability: map[string]string{template.OperationID: "operational"}})
		Expect(capability.OperationID).To(Equal(template.OperationID))
		Expect(capability.Template).To(Equal(template.Name))
		Expect(capability.ImplementationKey).To(Equal("runAnalyticalTemplate"))
		Expect(capability.ResultContract).To(Equal("forensics.enterprise-response/v1"))
		Expect(capability.RequiredParameters).To(Equal([]string{"target"}))
	})
})
