package main

import (
	"strings"
	"testing"
	"time"
)

func TestAPF37SingleCapabilityFastPathRemainsSingleStep(t *testing.T) {
	if spec := detectBoundedComposition("Who did 923001110001 contact most?"); len(spec.Templates) != 0 {
		t.Fatalf("simple query was promoted to composition: %#v", spec)
	}
	if spec := detectBoundedComposition("Correlate 923001110001 across record families"); len(spec.Templates) != 0 {
		t.Fatalf("existing governed cross-family operation was duplicated: %#v", spec)
	}
}

func TestPhoneAndPlateQuestionPlansIndependentAuthoritativeFamilies(t *testing.T) {
	query := "Show evidence related to phone 923001110001 and plate AB123CD."
	spec := detectBoundedComposition(query)
	if strings.Join(spec.Templates, ",") != "cross_family_correlation,anpr_sightings" {
		t.Fatalf("phone+plate composition = %#v", spec)
	}
	phone, plate := compositionPhoneAndPlate(query)
	if phone != "923001110001" || plate != "AB123CD" {
		t.Fatalf("typed composition targets = %q / %q", phone, plate)
	}

	req := hybridQueryRequest{TenantID: "default", UserID: "analyst-1", CollectionID: "case-1", Query: query, Target: phone, Targets: []string{phone, plate}, Limit: 20, MaxKBResults: 3}
	primary, _ := queryTemplateByName(spec.Templates[0])
	understanding := adaptLegacyRuntimePlan(req, runtimePlan{Template: primary.Name, Intent: intentRecords, Confidence: 1, Target: phone, Targets: req.Targets}, primary)
	understanding, _ = applyCompositionUnderstanding(understanding, spec)
	projection, err := buildResolvedCapabilityProjection(CapabilityProjectionContextV1{Authorized: true, RuntimeAvailable: true, FamilyStates: map[string]WorkspaceCapabilityStateV1{"cdr": {IndexedRecords: 4}, "anpr_vehicle_sightings": {IndexedRecords: 2}}})
	if err != nil {
		t.Fatal(err)
	}
	resolution := resolveCapabilities(understanding, projection)
	if len(resolution.Candidates) != 2 {
		t.Fatalf("phone+plate capabilities = %#v", resolution)
	}
	plan, err := buildBoundedCompositionPlan("request-phone-plate", req, understanding, resolution.Candidates)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Steps[0].Parameters.Target != phone || plan.Steps[1].Parameters.Target != plate || len(plan.Steps[1].Dependencies) != 0 {
		t.Fatalf("phone+plate parameters were not independently bound: %#v", plan.Steps)
	}
}

func TestPhonePlateAndDocumentQuestionAddsBoundedRetrievalStep(t *testing.T) {
	spec := detectBoundedComposition("Show evidence related to phone 923001110001 and plate AB123CD, including document mentions.")
	if strings.Join(spec.Templates, ",") != "cross_family_correlation,anpr_sightings,document_search" || spec.Reason != "explicit_phone_plus_plate_plus_documents" {
		t.Fatalf("phone+plate+document composition = %#v", spec)
	}
	step := ExecutionStepV1{StepID: "step-3", CapabilityID: "document.search", ExpectedResultContract: "forensics.enterprise-response/v1"}
	evidence := map[string]any{"results": []map[string]any{{"citation": "nexusai://evidence/synthetic/artifacts/passage", "metadata": map[string]any{"evidence_id": "synthetic", "version_id": "version", "source_file": "note.txt", "source_locator": `{"page":1}`}}}}
	result := typedCompositionStepFromEvidence(step, evidence)
	if result.Status != "completed" || len(result.Citations) != 1 || result.Citations[0].SourceFile != "note.txt" {
		t.Fatalf("document composition result lost citation authority: %#v", result)
	}
}

func TestAPF37BuildsAndExecutesBoundedTwoStepPlan(t *testing.T) {
	req, understanding, capabilities := compositionFixture(t)
	plan, err := buildBoundedCompositionPlan("request-37", req, understanding, capabilities)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Mode != "bounded_composition" || len(plan.Steps) != 2 || plan.ResourcePolicy.MaximumSteps != 3 || plan.ResourcePolicy.MaximumModelCalls != 1 {
		t.Fatalf("unexpected plan bounds: %#v", plan)
	}
	if !containsString(plan.Steps[1].Dependencies, "step-1") || len(plan.Steps[1].InputBindings) != 1 || plan.Steps[1].Parameters.Target != "" {
		t.Fatalf("subscriber-to-CDR dependency and exact binding were not planned: %#v", plan.Steps[1])
	}
	result, err := executeGovernedComposition(plan, capabilities, func(step ExecutionStepV1, _ ResolvedCapabilityV1) TypedStepResultV1 {
		return completedCompositionStep(step, 4)
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "completed" || len(result.Steps) != 2 || len(result.Claims) != 1 || result.Claims[0].SemanticState != "candidate_correlation" {
		t.Fatalf("unexpected composed result: %#v", result)
	}
}

func TestAPF37RejectsUnsafeOrInvalidPlansBeforeExecution(t *testing.T) {
	req, understanding, capabilities := compositionFixture(t)
	base, err := buildBoundedCompositionPlan("request-unsafe", req, understanding, capabilities)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		mutate func(*GovernedExecutionPlanV1)
	}{
		{"unknown capability", func(p *GovernedExecutionPlanV1) {
			p.Steps[0].CapabilityID, p.Steps[0].OperationID = "invented.capability", "invented.capability"
		}},
		{"arbitrary sql", func(p *GovernedExecutionPlanV1) {
			p.Steps[0].CapabilityID, p.Steps[0].OperationID = "select.from.records", "select.from.records"
		}},
		{"arbitrary url", func(p *GovernedExecutionPlanV1) {
			p.Steps[0].CapabilityID, p.Steps[0].OperationID = "https.example.test", "https.example.test"
		}},
		{"scope widening", func(p *GovernedExecutionPlanV1) { p.Steps[1].EvidenceScope.CollectionID = "other-case" }},
		{"citation disabled", func(p *GovernedExecutionPlanV1) { p.Steps[1].CitationRequired = false }},
		{"self dependency", func(p *GovernedExecutionPlanV1) { p.Steps[1].Dependencies = []string{"step-2"} }},
		{"unknown dependency", func(p *GovernedExecutionPlanV1) { p.Steps[1].Dependencies = []string{"step-99"} }},
		{"cycle", func(p *GovernedExecutionPlanV1) {
			p.Steps[0].Dependencies = []string{"step-2"}
			p.Steps[1].Dependencies = []string{"step-1"}
		}},
		{"bad binding", func(p *GovernedExecutionPlanV1) {
			p.Steps[1].Dependencies = []string{"step-1"}
			p.Steps[1].InputBindings = []ExecutionInputBindingV1{{SourceStepID: "step-1", SourceField: "target", TargetParameter: "sql", ValueType: "string"}}
		}},
		{"too many steps", func(p *GovernedExecutionPlanV1) { p.Steps = append(p.Steps, p.Steps[0], p.Steps[0]) }},
		{"duplicate capability", func(p *GovernedExecutionPlanV1) {
			p.Steps[1].CapabilityID, p.Steps[1].OperationID = p.Steps[0].CapabilityID, p.Steps[0].OperationID
		}},
		{"excessive model calls", func(p *GovernedExecutionPlanV1) { p.ResourcePolicy.MaximumModelCalls = 2 }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			unsafe := base
			unsafe.Steps = append([]ExecutionStepV1(nil), base.Steps...)
			tc.mutate(&unsafe)
			if err := validateBoundedCompositionPlan(unsafe, capabilities); err == nil {
				t.Fatal("unsafe plan accepted")
			}
		})
	}
}

func TestAPF37RejectsUnauthorizedAndMissingDataCapabilities(t *testing.T) {
	req, understanding, capabilities := compositionFixture(t)
	for _, reason := range []CapabilityReasonCode{CapabilityReasonUnauthorized, CapabilityReasonMissingData} {
		blocked := append([]ResolvedCapabilityV1(nil), capabilities...)
		blocked[1].Availability.Executable = false
		blocked[1].Availability.ReasonCodes = []CapabilityReasonCode{reason}
		if _, err := buildBoundedCompositionPlan("request-blocked", req, understanding, blocked); err == nil || !strings.Contains(err.Error(), string(reason)) {
			t.Fatalf("reason %s was not rejected truthfully: %v", reason, err)
		}
	}
}

func TestAPF37TypedIntermediateAndCitationValidation(t *testing.T) {
	req, understanding, capabilities := compositionFixture(t)
	plan, _ := buildBoundedCompositionPlan("request-types", req, understanding, capabilities)
	badType := completedCompositionStep(plan.Steps[0], 4)
	badType.Fields["row_count"] = TypedIntermediateFieldV1{Type: "integer", Value: "four"}
	if err := validateTypedStepResult(plan.Steps[0], badType); err == nil {
		t.Fatal("untyped intermediate value accepted")
	}
	missingCitation := completedCompositionStep(plan.Steps[0], 4)
	missingCitation.Citations = nil
	if err := validateTypedStepResult(plan.Steps[0], missingCitation); err == nil {
		t.Fatal("uncited factual result accepted")
	}
}

func TestAPF37DependencyOrderingAndExactTypedBinding(t *testing.T) {
	req, understanding, capabilities := compositionFixture(t)
	plan, _ := buildBoundedCompositionPlan("request-binding", req, understanding, capabilities)
	order := []string{}
	result, err := executeGovernedComposition(plan, capabilities, func(step ExecutionStepV1, _ ResolvedCapabilityV1) TypedStepResultV1 {
		order = append(order, step.StepID)
		completed := completedCompositionStep(step, 1)
		if step.StepID == "step-1" {
			completed.Fields["exact_target"] = TypedIntermediateFieldV1{Type: "string", Value: "923001234567"}
		} else if step.Parameters.Target != "923001234567" {
			t.Fatalf("exact binding changed target: %#v", step.Parameters)
		}
		return completed
	})
	if err != nil || result.Status != "completed" || strings.Join(order, ",") != "step-1,step-2" {
		t.Fatalf("dependency execution failed: order=%v result=%#v err=%v", order, result, err)
	}
}

func TestAPF37NoResultDependencySkipsDownstreamWithoutCorrelationClaim(t *testing.T) {
	req, understanding, capabilities := compositionFixture(t)
	plan, err := buildBoundedCompositionPlan("request-no-result", req, understanding, capabilities)
	if err != nil {
		t.Fatal(err)
	}
	executed := []string{}
	result, err := executeGovernedComposition(plan, capabilities, func(step ExecutionStepV1, _ ResolvedCapabilityV1) TypedStepResultV1 {
		executed = append(executed, step.StepID)
		return TypedStepResultV1{StepID: step.StepID, CapabilityID: step.CapabilityID, Status: "no_results", ResultContract: step.ExpectedResultContract, Fields: map[string]TypedIntermediateFieldV1{}, Citations: []ExecutionCitationV1{}, Limitations: []string{}}
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(executed, ",") != "step-1" || result.Status != "partial_analysis" || result.Steps[1].Status != "skipped_dependency" || len(result.Claims) != 0 {
		t.Fatalf("no-result dependency was not bounded truthfully: executed=%v result=%#v", executed, result)
	}
}

func TestAPF37ValidStructuredAndKnowledgePlan(t *testing.T) {
	req := hybridQueryRequest{TenantID: "default", UserID: "analyst-1", CollectionID: "case-1", Query: "Combine structured evidence and cited knowledge", Limit: 20, MaxKBResults: 3}
	structured, _ := queryTemplateByName("canonical_records")
	understanding := adaptLegacyRuntimePlan(req, runtimePlan{Template: structured.Name, Intent: intentRecords, Confidence: 1}, structured)
	understanding.CandidateCapabilities = []string{"forensics.canonical_records", "forensics.evidence"}
	understanding.Families = QueryFamilySetV1{Kind: "multiple", Primary: "case_cross_family", Secondary: []string{"knowledge_evidence"}}
	ctx := CapabilityProjectionContextV1{Authorized: true, RuntimeAvailable: true, FamilyStates: map[string]WorkspaceCapabilityStateV1{"cdr": {IndexedRecords: 10, KBReadyEvidence: 1}}}
	projection, _ := buildResolvedCapabilityProjection(ctx)
	resolution := resolveCapabilities(understanding, projection)
	if len(resolution.Candidates) != 2 {
		t.Fatalf("structured+KB capabilities not available: %#v", resolution)
	}
	plan, err := buildBoundedCompositionPlan("request-structured-kb", req, understanding, resolution.Candidates)
	if err != nil {
		t.Fatal(err)
	}
	result, err := executeGovernedComposition(plan, resolution.Candidates, func(step ExecutionStepV1, _ ResolvedCapabilityV1) TypedStepResultV1 {
		return completedCompositionStep(step, 1)
	})
	if err != nil || result.Status != "completed" || len(result.Claims) != 1 {
		t.Fatalf("structured+KB composition failed: %#v %v", result, err)
	}
}

func TestAPF37PartialFailureDoesNotClaimCompleteCorrelation(t *testing.T) {
	req, understanding, capabilities := compositionFixture(t)
	plan, _ := buildBoundedCompositionPlan("request-partial", req, understanding, capabilities)
	result, err := executeGovernedComposition(plan, capabilities, func(step ExecutionStepV1, _ ResolvedCapabilityV1) TypedStepResultV1 {
		if step.StepID == "step-2" {
			return TypedStepResultV1{StepID: step.StepID, CapabilityID: step.CapabilityID, Status: "failed", ResultContract: step.ExpectedResultContract, Fields: map[string]TypedIntermediateFieldV1{}, Citations: []ExecutionCitationV1{}, FailureCode: "execution_failed", Limitations: []string{"Capability execution failed."}}
		}
		return completedCompositionStep(step, 1)
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "partial_analysis" || len(result.Claims) != 0 {
		t.Fatalf("partial execution overstated conclusion: %#v", result)
	}
}

func TestAPF37EnterprisePresentationPreservesStepAndClaimLineage(t *testing.T) {
	composition := &GovernedCompositionResultV1{ContractVersion: compositionResultContract, PlanID: "plan-1", Status: "completed", Steps: []TypedStepResultV1{
		{StepID: "step-1", CapabilityID: "subscriber.identity_lookup", Status: "completed", ResultContract: "forensics.enterprise-response/v1", Fields: map[string]TypedIntermediateFieldV1{"row_count": {Type: "integer", Value: int64(1)}}, Citations: []ExecutionCitationV1{{EvidenceID: "evidence-subscriber", VersionID: "version-1", SourceFile: "subscriber.csv", SourceRow: 2}}},
		{StepID: "step-2", CapabilityID: "cdr.temporal_activity", Status: "completed", ResultContract: "forensics.enterprise-response/v1", Fields: map[string]TypedIntermediateFieldV1{"row_count": {Type: "integer", Value: int64(4)}}, Citations: []ExecutionCitationV1{{EvidenceID: "evidence-cdr", VersionID: "version-2", SourceFile: "cdr.csv", SourceRow: 5}}},
	}, Claims: []CompositionClaimLineageV1{{ClaimID: "claim-1", SemanticState: "candidate_correlation", StepIDs: []string{"step-1", "step-2"}, CapabilityIDs: []string{"subscriber.identity_lookup", "cdr.temporal_activity"}}}}
	legacy := hybridQueryResponse{CollectionID: "case-1", Template: "subscriber_identity_lookup", Route: []string{"bounded_composition", "records_sql"}, Answer: map[string]any{}, Composition: composition}
	response := legacyHybridToEnterpriseV1("case-1", "request-1", map[string]any{"tenant_id": "default", "limit": 20}, legacy)
	if response.Status != "answered_with_limitations" || len(response.Tables) != 1 || len(response.DeterministicFindings) != 2 || len(response.SemanticEvidence) != 2 {
		t.Fatalf("composition presentation lost typed results or lineage: %#v", response)
	}
	if !strings.Contains(response.ExecutiveAnswer, "candidate correlation") || !containsString(response.ExecutionTrace.ToolIDs, "subscriber.identity_lookup") || !containsString(response.ExecutionTrace.ToolIDs, "cdr.temporal_activity") {
		t.Fatalf("composition presentation overstated or omitted tools: %#v", response)
	}
}

func TestAPF37LegacyEnterprisePayloadPresentsCompositionSteps(t *testing.T) {
	composition := &GovernedCompositionResultV1{ContractVersion: compositionResultContract, PlanID: "plan-legacy", Status: "partial_analysis", Steps: []TypedStepResultV1{
		{StepID: "step-1", CapabilityID: "subscriber.identity_lookup", Status: "completed", Fields: map[string]TypedIntermediateFieldV1{"row_count": {Type: "integer", Value: int64(1)}}, Citations: []ExecutionCitationV1{{EvidenceID: "evidence-subscriber", SourceFile: "subscriber.csv", SourceRow: 2}}},
		{StepID: "step-2", CapabilityID: "cdr.temporal_activity", Status: "skipped_dependency", Fields: map[string]TypedIntermediateFieldV1{}, FailureCode: "dependency_failed", Limitations: []string{"A required upstream capability did not complete."}},
	}}
	payload := buildEnterprisePayload(hybridQueryRequest{TenantID: "default", UserID: "analyst-1", CollectionID: "case-1", Query: "compose", Target: "923001234567"}, hybridQueryResponse{Route: []string{"bounded_composition", "records_sql"}, Composition: composition, Answer: map[string]any{}})
	grid := payload["data_grid"].(map[string]any)
	if grid["title"] != "Bounded capability composition" || grid["count"] != 2 || payload["status"] != "partial_analysis" {
		t.Fatalf("legacy composition payload flattened or mislabeled steps: %#v", payload)
	}
	operation := payload["operation"].(map[string]any)
	if operation["operation_id"] != "forensics.bounded_composition" || len(payload["provenance"].([]map[string]any)) != 1 {
		t.Fatalf("legacy composition operation or provenance missing: %#v", payload)
	}
}

func TestBoundedCompositionPrioritizesOneCitationPerCompletedStep(t *testing.T) {
	phoneCitations := make([]ExecutionCitationV1, 0, 25)
	for row := 1; row <= 25; row++ {
		phoneCitations = append(phoneCitations, ExecutionCitationV1{EvidenceID: "evidence-cdr", VersionID: "version-cdr", SourceFile: "cdr.csv", SourceRow: int64(row)})
	}
	composition := &GovernedCompositionResultV1{ContractVersion: compositionResultContract, PlanID: "plan-balanced-citations", Status: "completed", Steps: []TypedStepResultV1{
		{StepID: "step-1", CapabilityID: "forensics.cross_family_correlation", Status: "completed", Fields: map[string]TypedIntermediateFieldV1{"row_count": {Type: "integer", Value: int64(25)}}, Citations: phoneCitations},
		{StepID: "step-2", CapabilityID: "anpr.sightings", Status: "completed", Fields: map[string]TypedIntermediateFieldV1{"row_count": {Type: "integer", Value: int64(1)}}, Citations: []ExecutionCitationV1{{EvidenceID: "evidence-anpr", VersionID: "version-anpr", SourceFile: "anpr.csv", SourceRow: 7}}},
	}}
	payload := buildEnterprisePayload(
		hybridQueryRequest{TenantID: "default", CollectionID: "case-1", Query: "Correlate phone 923001110001 with plate MN1367", Limit: 20},
		hybridQueryResponse{Route: []string{"bounded_composition", "records_sql"}, Composition: composition, Answer: map[string]any{}},
	)
	provenance := payload["provenance"].([]map[string]any)
	if len(provenance) != 26 || provenance[0]["source_file"] != "cdr.csv" || provenance[1]["source_file"] != "anpr.csv" {
		t.Fatalf("bounded provenance did not preserve step coverage: %#v", provenance[:min(len(provenance), 3)])
	}
	citations, _ := factPacketCitations(provenance[:20])
	if citations[0].SourceFile != "cdr.csv" || citations[1].SourceFile != "anpr.csv" {
		t.Fatalf("bounded citation set omitted a completed step: %#v", citations[:2])
	}
}

func TestAPF37DeterministicPlanningPerformance(t *testing.T) {
	req, understanding, capabilities := compositionFixture(t)
	started := time.Now()
	for i := 0; i < 1000; i++ {
		if _, err := buildBoundedCompositionPlan("request-perf", req, understanding, capabilities); err != nil {
			t.Fatal(err)
		}
	}
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("bounded deterministic planning took %s", elapsed)
	}
}

func compositionFixture(t *testing.T) (hybridQueryRequest, QueryUnderstandingV1, []ResolvedCapabilityV1) {
	t.Helper()
	req := hybridQueryRequest{TenantID: "default", UserID: "analyst-1", CollectionID: "case-1", Query: "Show subscriber identity and CDR activity for 923001234567", Target: "923001234567", Targets: []string{"923001234567"}, Limit: 20, MaxKBResults: 3}
	spec := detectBoundedComposition(req.Query)
	if len(spec.Templates) != 2 {
		t.Fatalf("composition not recognized: %#v", spec)
	}
	primary, _ := queryTemplateByName(spec.Templates[0])
	planner := planRuntimeQuery(req)
	planner.Template = primary.Name
	planner.Target = req.Target
	planner.Targets = req.Targets
	planner.Confidence = 1
	understanding := adaptLegacyRuntimePlan(req, planner, primary)
	understanding, _ = applyCompositionUnderstanding(understanding, spec)
	ctx := CapabilityProjectionContextV1{Authorized: true, RuntimeAvailable: true, FamilyStates: map[string]WorkspaceCapabilityStateV1{"cdr": {IndexedRecords: 10}, "subscriber_identity": {IndexedRecords: 5}}}
	projection, err := buildResolvedCapabilityProjection(ctx)
	if err != nil {
		t.Fatal(err)
	}
	resolution := resolveCapabilities(understanding, projection)
	if len(resolution.Candidates) != 2 {
		t.Fatalf("composition candidates: %#v", resolution)
	}
	return req, understanding, resolution.Candidates
}

func completedCompositionStep(step ExecutionStepV1, rows int64) TypedStepResultV1 {
	fields := map[string]TypedIntermediateFieldV1{"row_count": {Type: "integer", Value: rows}}
	if step.Parameters.Target != "" {
		fields["exact_target"] = TypedIntermediateFieldV1{Type: "string", Value: step.Parameters.Target}
	}
	return TypedStepResultV1{StepID: step.StepID, CapabilityID: step.CapabilityID, Status: "completed", ResultContract: step.ExpectedResultContract, Fields: fields, Citations: []ExecutionCitationV1{{EvidenceID: "evidence-1", VersionID: "version-1", SourceFile: "fixture.csv", SourceRow: 1}}, Limitations: []string{}}
}
