package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestAPF31TypedContractsAndLegacyAdapter(t *testing.T) {
	req := hybridQueryRequest{
		TenantID: "default", UserID: "analyst-1", CollectionID: "case-1",
		Query:    "Show temporal CDR activity for 923001234567 on 2026-07-10.",
		Template: "temporal_activity",
		Limit:    20, MaxKBResults: 3,
	}
	planner := planRuntimeQuery(req)
	req.Target, req.Targets = planner.Target, planner.Targets
	req.DateFrom, req.DateTo = planner.DateFrom, planner.DateTo
	template, ok := queryTemplateByName(planner.Template)
	if !ok {
		t.Fatalf("legacy router selected unknown template %q", planner.Template)
	}
	understanding := adaptLegacyRuntimePlan(req, planner, template)
	if err := understanding.Validate(); err != nil {
		t.Fatalf("validate query understanding: %v", err)
	}
	if understanding.ContractVersion != queryUnderstandingContractV1 || understanding.Intent != QueryIntentTimeline {
		t.Fatalf("unexpected understanding contract: %#v", understanding)
	}
	if understanding.Families.Primary != "communications_cdr" || len(understanding.Entities) == 0 || understanding.Entities[0].Normalized != "923001234567" {
		t.Fatalf("legacy adapter lost family/entity resolution: %#v", understanding)
	}
	if understanding.TimeScope.From == "" || understanding.TimeScope.To == "" {
		t.Fatalf("legacy adapter lost date bounds: %#v", understanding.TimeScope)
	}
	payload, err := json.Marshal(understanding)
	if err != nil || !strings.Contains(string(payload), queryUnderstandingContractV1) {
		t.Fatalf("serialize query understanding: %v %s", err, payload)
	}
}

func TestAPF31ContractValidationRejectsUnsafeShapes(t *testing.T) {
	valid := QueryUnderstandingV1{
		ContractVersion: queryUnderstandingContractV1, OriginalQuestion: "show evidence", NormalizedQuestion: "show evidence",
		Language: QueryLanguageV1{Tag: "en", Source: QueryOriginDeterministicExtractor}, Intent: QueryIntentRetrieve,
		CandidateCapabilities: []string{"forensics.evidence"},
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid understanding rejected: %v", err)
	}
	for _, unsafe := range []string{"", "run sql", "https://example.test/tool", "cdr.timeline;drop", "../tool"} {
		candidate := valid
		candidate.CandidateCapabilities = []string{unsafe}
		if err := candidate.Validate(); err == nil {
			t.Fatalf("unsafe capability ID %q was accepted", unsafe)
		}
	}
}

func TestAPF31ExecutableOperationDescriptorParity(t *testing.T) {
	templates := supportedQueryTemplates()
	if len(templates) != 79 {
		t.Fatalf("executable operation catalog has %d entries, want 79", len(templates))
	}
	catalog, err := loadForensicPlatformCatalog()
	if err != nil {
		t.Fatalf("load reconciled platform catalog: %v", err)
	}
	descriptors := map[string]FamilyOperationDescriptorV1{}
	for _, descriptor := range catalog.Operations {
		descriptors[descriptor.ID] = descriptor
	}
	for _, template := range templates {
		if resolved := queryTemplateNameByOperationID(template.OperationID); resolved != template.Name {
			t.Errorf("typed case API reverse lookup for %s = %q, want %q", template.OperationID, resolved, template.Name)
		}
		descriptor, exists := descriptors[template.OperationID]
		if !exists {
			t.Errorf("operation %s (%s) has no descriptor", template.OperationID, template.Name)
			continue
		}
		if descriptor.FamilyID == "" || descriptor.Implementation == "" || descriptor.InputContract == "" || descriptor.OutputContract == "" || !descriptor.CitationRequired {
			t.Errorf("operation %s has incomplete APF-3 metadata: %#v", template.OperationID, descriptor)
		}
	}
	if len(descriptors) != 104 {
		t.Fatalf("reconciled catalog has %d total descriptors, want the authoritative 104-operation platform catalog", len(descriptors))
	}
	canonical, _ := queryTemplateByName("canonical_records")
	canonicalCapability := capabilityFromTemplate(canonical, CapabilityProjectionContextV1{})
	if !canonicalCapability.SourceScoped || !containsString(canonicalCapability.OptionalParameters, "batch_id") {
		t.Fatalf("canonical records must advertise enforced source/batch predicates: %#v", canonicalCapability)
	}
	frequent, _ := queryTemplateByName("frequent_contacts")
	frequentCapability := capabilityFromTemplate(frequent, CapabilityProjectionContextV1{})
	if frequentCapability.SourceScoped || containsString(frequentCapability.OptionalParameters, "batch_id") {
		t.Fatalf("analytical templates must not advertise ignored source/batch predicates: %#v", frequentCapability)
	}
}

func TestAPF31LegacyRoutingAndLanguagesRemainEquivalent(t *testing.T) {
	tests := []struct {
		name, query, template, language string
	}{
		{"English", "who does 923001110001 contact most?", "frequent_contacts", "en"},
		{"Roman Urdu", "subscriber ki tafseel 0300-1234567 ke liye dikhao", "subscriber_identity_lookup", "ur-Latn"},
		{"Urdu", "اس نمبر 0300-1234567 کی سبسکرائبر کی تفصیل دکھائیں", "subscriber_identity_lookup", "ur"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := hybridQueryRequest{Query: tc.query, Limit: 20, MaxKBResults: 3}
			planner := planRuntimeQuery(req)
			if planner.Template != tc.template {
				t.Fatalf("legacy route changed: got %q want %q", planner.Template, tc.template)
			}
			req.Target, req.Targets = planner.Target, planner.Targets
			template, _ := queryTemplateByName(planner.Template)
			understanding := adaptLegacyRuntimePlan(req, planner, template)
			if understanding.Language.Tag != tc.language || len(understanding.CandidateCapabilities) != 1 || understanding.CandidateCapabilities[0] != template.OperationID {
				t.Fatalf("typed adapter changed route/language: %#v", understanding)
			}
		})
	}
}

func TestAPF32ResolvedCapabilityAvailabilityMatrix(t *testing.T) {
	ready := CapabilityProjectionContextV1{
		Authorized: true, RuntimeAvailable: true,
		FamilyStates: map[string]WorkspaceCapabilityStateV1{"cdr": {IndexedRecords: 10, RegisteredEvidence: 1, NormalizedEvidence: 1}},
		ModelRoles:   map[string]bool{},
	}
	projection, err := buildResolvedCapabilityProjection(ready)
	if err != nil || len(projection) != 79 {
		t.Fatalf("build 79-capability projection: len=%d err=%v", len(projection), err)
	}
	understanding := understandingForOperation(t, "frequent_contacts", "923001110001")
	resolution := resolveCapabilities(understanding, projection)
	if len(resolution.Candidates) != 1 || !resolution.Candidates[0].Availability.Executable || !resolution.Candidates[0].Availability.ModelAvailable {
		t.Fatalf("ready deterministic capability was not executable: %#v", resolution)
	}

	cases := []struct {
		name   string
		ctx    CapabilityProjectionContextV1
		reason CapabilityReasonCode
	}{
		{"missing data", CapabilityProjectionContextV1{Authorized: true, RuntimeAvailable: true, FamilyStates: map[string]WorkspaceCapabilityStateV1{}}, CapabilityReasonMissingData},
		{"unauthorized", CapabilityProjectionContextV1{Authorized: false, RuntimeAvailable: true, FamilyStates: ready.FamilyStates}, CapabilityReasonUnauthorized},
		{"runtime unavailable", CapabilityProjectionContextV1{Authorized: true, RuntimeAvailable: false, FamilyStates: ready.FamilyStates}, CapabilityReasonRuntimeUnavailable},
		{"processing incomplete", CapabilityProjectionContextV1{Authorized: true, RuntimeAvailable: true, FamilyStates: map[string]WorkspaceCapabilityStateV1{"cdr": {RegisteredEvidence: 1, ProcessingEvidence: 1}}}, CapabilityReasonProcessingIncomplete},
		{"required model absent", CapabilityProjectionContextV1{Authorized: true, RuntimeAvailable: true, FamilyStates: ready.FamilyStates, ModelRoles: map[string]bool{}, RequiredModelRolesByCapability: map[string][]string{"cdr.frequent_contacts": {"text_reasoning"}}}, CapabilityReasonModelUnavailable},
		{"maturity insufficient", CapabilityProjectionContextV1{Authorized: true, RuntimeAvailable: true, FamilyStates: ready.FamilyStates, MaturityByCapability: map[string]string{"cdr.frequent_contacts": "experimental"}}, CapabilityReasonMaturityInsufficient},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			projection, err := buildResolvedCapabilityProjection(tc.ctx)
			if err != nil {
				t.Fatal(err)
			}
			resolution := resolveCapabilities(understanding, projection)
			if len(resolution.Candidates) != 0 || len(resolution.Rejected) != 1 || !containsReason(resolution.Rejected[0].Availability.ReasonCodes, tc.reason) {
				t.Fatalf("unexpected rejection: %#v", resolution)
			}
		})
	}

	unknown := understanding
	unknown.CandidateCapabilities = []string{"cdr.not_registered"}
	resolution = resolveCapabilities(unknown, projection)
	if len(resolution.Candidates) != 0 || len(resolution.Rejected) != 0 {
		t.Fatalf("unknown capability should not resolve: %#v", resolution)
	}
}

func TestAPF33SingleCapabilityPlanAndSecurityValidation(t *testing.T) {
	understanding := understandingForOperation(t, "frequent_contacts", "923001110001")
	ctx := CapabilityProjectionContextV1{Authorized: true, RuntimeAvailable: true, FamilyStates: map[string]WorkspaceCapabilityStateV1{"cdr": {IndexedRecords: 10}}}
	projection, _ := buildResolvedCapabilityProjection(ctx)
	resolution := resolveCapabilities(understanding, projection)
	capability := resolution.Candidates[0]
	req := hybridQueryRequest{TenantID: "default", UserID: "analyst-1", CollectionID: "case-1", Query: "who does 923001110001 contact most?", Target: "923001110001", Targets: []string{"923001110001"}, Limit: 20, MaxKBResults: 3}
	plan, err := buildSingleCapabilityPlan("request-1", req, understanding, capability)
	if err != nil {
		t.Fatalf("build single capability plan: %v", err)
	}
	if err := plan.ValidateShape(); err != nil {
		t.Fatalf("validate plan shape: %v", err)
	}
	payload, err := json.Marshal(plan)
	if err != nil || !strings.Contains(string(payload), executionPlanContractV1) {
		t.Fatalf("serialize execution plan: %v %s", err, payload)
	}
	executed := 0
	result, err := executeGovernedSingleCapability(func(template string) (map[string]any, error) {
		executed++
		return map[string]any{"template": template}, nil
	}, plan, capability)
	if err != nil || executed != 1 || result["template"] != "frequent_contacts" {
		t.Fatalf("governed executor did not dispatch exactly once: result=%v count=%d err=%v", result, executed, err)
	}

	mutations := []struct {
		name   string
		mutate func(*GovernedExecutionPlanV1)
	}{
		{"widen scope", func(p *GovernedExecutionPlanV1) { p.Workspace.CollectionID = "other-case" }},
		{"unknown operation", func(p *GovernedExecutionPlanV1) {
			p.Steps[0].CapabilityID, p.Steps[0].OperationID = "cdr.unknown", "cdr.unknown"
		}},
		{"arbitrary SQL", func(p *GovernedExecutionPlanV1) {
			p.Steps[0].CapabilityID, p.Steps[0].OperationID = "select * from records", "select * from records"
		}},
		{"arbitrary URL", func(p *GovernedExecutionPlanV1) {
			p.Steps[0].CapabilityID, p.Steps[0].OperationID = "https://example.test", "https://example.test"
		}},
		{"arbitrary tool", func(p *GovernedExecutionPlanV1) {
			p.Steps[0].CapabilityID, p.Steps[0].OperationID = "shell.exec", "shell.exec"
		}},
		{"missing required target", func(p *GovernedExecutionPlanV1) {
			p.Steps[0].Parameters.Target, p.Steps[0].Parameters.Targets = "", nil
		}},
		{"unsupported source scope", func(p *GovernedExecutionPlanV1) { p.Steps[0].Parameters.SourceFile = "other.csv" }},
		{"disable citations", func(p *GovernedExecutionPlanV1) { p.Steps[0].CitationRequired = false }},
		{"multiple steps", func(p *GovernedExecutionPlanV1) { p.Steps = append(p.Steps, p.Steps[0]) }},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			unsafe := plan
			unsafe.Steps = append([]ExecutionStepV1(nil), plan.Steps...)
			tc.mutate(&unsafe)
			if err := validateSingleCapabilityPlan(unsafe, capability); err == nil {
				t.Fatalf("unsafe plan %q was accepted", tc.name)
			}
		})
	}
}

func TestAPF33HybridCapabilityUsesGovernedHybridDispatcher(t *testing.T) {
	understanding := understandingForOperation(t, "evidence_package_summary", "")
	ctx := CapabilityProjectionContextV1{
		Authorized: true, RuntimeAvailable: true,
		FamilyStates: map[string]WorkspaceCapabilityStateV1{"all": {IndexedRecords: 10, RegisteredEvidence: 1, NormalizedEvidence: 1, KBReadyEvidence: 1}},
	}
	projection, err := buildResolvedCapabilityProjection(ctx)
	if err != nil {
		t.Fatal(err)
	}
	resolution := resolveCapabilities(understanding, projection)
	if len(resolution.Candidates) != 1 {
		t.Fatalf("hybrid capability did not resolve: %#v", resolution)
	}
	capability := resolution.Candidates[0]
	if capability.ExecutionMode != "legacy_hybrid" {
		t.Fatalf("hybrid execution mode = %q", capability.ExecutionMode)
	}
	req := hybridQueryRequest{TenantID: "default", UserID: "analyst-1", CollectionID: "case-1", Query: "prepare an evidence package", Template: "evidence_package_summary", Limit: 20, MaxKBResults: 3}
	plan, err := buildSingleCapabilityPlan("request-hybrid", req, understanding, capability)
	if err != nil {
		t.Fatal(err)
	}
	executed := 0
	result, err := executeGovernedRecordsCapability(func(template string) (map[string]any, error) {
		executed++
		return map[string]any{"template": template}, nil
	}, plan, capability)
	if err != nil || executed != 1 || result["template"] != "evidence_package_summary" {
		t.Fatalf("governed hybrid dispatch failed: result=%#v executed=%d err=%v", result, executed, err)
	}
	if _, err := executeGovernedSingleCapability(func(string) (map[string]any, error) { return nil, nil }, plan, capability); err == nil {
		t.Fatal("deterministic-only executor accepted a hybrid capability")
	}
}

func TestAPF33RetrievalCapabilityUsesEvidenceExecutorOnly(t *testing.T) {
	if shouldExecuteRecordsCapability(intentRecords, "retrieval") {
		t.Fatal("retrieval capability was routed to the records executor")
	}
	if !shouldExecuteEvidenceRetrieval(intentRecords, "retrieval") {
		t.Fatal("retrieval capability did not reach the evidence executor")
	}
	if !shouldExecuteRecordsCapability(intentHybrid, "legacy_hybrid") {
		t.Fatal("legacy hybrid capability lost its governed records stage")
	}
	if !shouldExecuteEvidenceRetrieval(intentHybrid, "legacy_hybrid") {
		t.Fatal("legacy hybrid capability lost its governed evidence stage")
	}
}

func TestAPF33FailureSemanticsRemainDistinct(t *testing.T) {
	tests := map[CapabilityReasonCode]string{
		CapabilityReasonMissingData:          "data_unavailable",
		CapabilityReasonProcessingIncomplete: "processing_incomplete",
		CapabilityReasonUnauthorized:         "unauthorized",
		CapabilityReasonUnsupported:          "unsupported",
		CapabilityReasonRuntimeUnavailable:   "capability_unavailable",
		CapabilityReasonModelUnavailable:     "capability_unavailable",
		CapabilityReasonMaturityInsufficient: "capability_unavailable",
		CapabilityReasonInvalidParameter:     "needs_input",
	}
	for reason, expected := range tests {
		resp := hybridQueryResponse{Answer: map[string]any{"failure_semantics": string(reason)}}
		if actual := enterpriseOutcomeStatus(resp, nil); actual != expected {
			t.Errorf("failure %s compiled as %s, want %s", reason, actual, expected)
		}
	}
}

func TestAPF33EvidenceOnlyResultIsNotMislabelledNoResults(t *testing.T) {
	resp := hybridQueryResponse{
		Intent: intentSemantic,
		Answer: map[string]any{"evidence_count": 3, "evidence_status": "matched", "evidence_summary": "Retrieved cited evidence"},
	}
	if got := enterpriseOutcomeStatus(resp, nil); got == "no_results" {
		t.Fatalf("evidence-only result status = %q", got)
	}
	if got := enterpriseExecutiveAnswer(hybridQueryRequest{}, resp, nil); !strings.Contains(got, "3 cited evidence results") {
		t.Fatalf("evidence-only executive answer = %q", got)
	}
}

func TestAPF33DeterministicPlanningPerformance(t *testing.T) {
	req := hybridQueryRequest{Query: "who does 923001110001 contact most?", Limit: 20, MaxKBResults: 3}
	started := time.Now()
	for i := 0; i < 1000; i++ {
		planner := planRuntimeQuery(req)
		template, _ := queryTemplateByName(planner.Template)
		_ = adaptLegacyRuntimePlan(req, planner, template)
	}
	t.Logf("1000 deterministic understanding passes: %s", time.Since(started))
}

func understandingForOperation(t *testing.T, templateName, target string) QueryUnderstandingV1 {
	t.Helper()
	template, ok := queryTemplateByName(templateName)
	if !ok {
		t.Fatalf("unknown template %q", templateName)
	}
	req := hybridQueryRequest{Query: template.ExampleQuery, Target: target, Targets: []string{target}, Limit: 20, MaxKBResults: 3}
	planner := planRuntimeQuery(req)
	planner.Template = templateName
	planner.Target = target
	planner.Targets = []string{target}
	planner.Confidence = 1
	return adaptLegacyRuntimePlan(req, planner, template)
}

func containsReason(values []CapabilityReasonCode, want CapabilityReasonCode) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
