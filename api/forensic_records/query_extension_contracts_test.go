package main

import "testing"

func TestAPF36AuthorizedKnowledgeRetrievalCitationAndAbstention(t *testing.T) {
	scope := ExecutionWorkspaceScopeV1{TenantID: "default", SubjectID: "analyst-1", CaseID: "case-a", CollectionID: "case-a"}
	passage := KnowledgePassageV1{PassageID: "p-1", ChunkID: "chunk-1", ChunkHash: "sha256:abc", EvidenceID: "evidence-1", SourceFile: "policy.pdf", Locator: "page 3", Version: "v1", Score: 0.91}
	execution := KnowledgeExecutionV1{
		ContractVersion: knowledgeExecutionContractV1, TenantID: "default", UserID: "analyst-1", CollectionID: "case-a",
		Query: "find the applicable policy", RetrievalLimit: 3, Passages: []KnowledgePassageV1{passage}, CitationRequired: true,
	}
	if err := execution.Validate(scope, map[string]bool{"evidence-1": true}); err != nil {
		t.Fatalf("authorized cited retrieval rejected: %v", err)
	}
	if err := execution.Validate(scope, map[string]bool{}); err == nil {
		t.Fatal("unauthorized evidence scope was accepted")
	}
	execution.CollectionID = "case-b"
	if err := execution.Validate(scope, map[string]bool{"evidence-1": true}); err == nil {
		t.Fatal("cross-workspace retrieval was accepted")
	}

	abstention := KnowledgeExecutionV1{
		ContractVersion: knowledgeExecutionContractV1, TenantID: "default", UserID: "analyst-1", CollectionID: "case-a",
		Query: "unsupported question", RetrievalLimit: 3, CitationRequired: true, Abstained: true, AbstentionReason: "no supporting evidence found",
	}
	if err := abstention.Validate(scope, map[string]bool{}); err != nil {
		t.Fatalf("truthful no-result abstention rejected: %v", err)
	}
	abstention.Abstained = false
	if err := abstention.Validate(scope, map[string]bool{}); err == nil {
		t.Fatal("empty retrieval was allowed to synthesize without evidence")
	}
}

func TestAPF36ModelRoleRuntimeMaturityAndArtifactResolution(t *testing.T) {
	contract := ModelCapabilityContractV1{
		ContractVersion: modelCapabilityContractV1, ModelRole: "document_ocr", InputModalities: []string{"document_image"},
		OutputContract: "forensics.document-observations/v1", RequiredArtifacts: []string{"registered_document_pages"},
		MaturityRequirement: "operational", RuntimeRequirement: "localai_worker", TimeoutMS: 30000, ResourceBudget: "single_page",
		ConfidenceSemantics: "per observation", AbstentionSemantics: "no derived text when confidence is insufficient", CitationSemantics: "page and region required",
	}
	ready := ModelRuntimeStateV1{RuntimeAvailable: true, AvailableRoles: map[string]bool{"document_ocr": true}, MaturityByRole: map[string]string{"document_ocr": "operational"}, DerivedArtifacts: map[string]bool{"registered_document_pages": true}}
	if err := contract.Validate(ready); err != nil {
		t.Fatalf("ready role rejected: %v", err)
	}
	for _, state := range []ModelRuntimeStateV1{
		{RuntimeAvailable: false, AvailableRoles: ready.AvailableRoles, MaturityByRole: ready.MaturityByRole, DerivedArtifacts: ready.DerivedArtifacts},
		{RuntimeAvailable: true, AvailableRoles: map[string]bool{}, MaturityByRole: ready.MaturityByRole, DerivedArtifacts: ready.DerivedArtifacts},
		{RuntimeAvailable: true, AvailableRoles: ready.AvailableRoles, MaturityByRole: map[string]string{"document_ocr": "evaluation"}, DerivedArtifacts: ready.DerivedArtifacts},
		{RuntimeAvailable: true, AvailableRoles: ready.AvailableRoles, MaturityByRole: ready.MaturityByRole, DerivedArtifacts: map[string]bool{}},
	} {
		if err := contract.Validate(state); err == nil {
			t.Fatalf("unavailable model capability accepted: %#v", state)
		}
	}
	contract.ModelRole = "vendor/model-file.gguf"
	if err := contract.Validate(ready); err == nil {
		t.Fatal("query capability hardcoded a model implementation instead of a role")
	}
}

func TestAPF36CurrentKBHybridAndDeterministicCompatibility(t *testing.T) {
	counts := map[string]int{}
	for _, template := range supportedQueryTemplates() {
		counts[template.Route]++
		mode := extensionExecutionMode(template.Route)
		if mode != "deterministic_operation" && mode != "knowledge_retrieval" && mode != "existing_hybrid" {
			t.Fatalf("operation %s has unknown extension mode %q", template.OperationID, mode)
		}
	}
	if counts["kb"] != 1 || counts["derived"] != 3 || counts["hybrid"] != 2 || counts["records"] != 73 {
		t.Fatalf("current execution surface changed: %#v", counts)
	}
	projection, err := buildResolvedCapabilityProjection(CapabilityProjectionContextV1{})
	if err != nil || len(projection) != 79 {
		t.Fatalf("extension contract broke capability projection: len=%d err=%v", len(projection), err)
	}
	for _, capability := range projection {
		if capability.ExtensionMode == "" {
			t.Fatalf("capability %s has no extension execution mode", capability.CapabilityID)
		}
	}
}
