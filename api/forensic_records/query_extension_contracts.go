package main

import (
	"errors"
	"fmt"
	"strings"
)

const (
	knowledgeExecutionContractV1 = "forensics.knowledge-execution/v1"
	modelCapabilityContractV1    = "forensics.model-capability/v1"
)

type KnowledgePassageV1 struct {
	PassageID   string   `json:"passage_id"`
	ChunkID     string   `json:"chunk_id"`
	ChunkHash   string   `json:"chunk_hash"`
	EvidenceID  string   `json:"evidence_id"`
	SourceFile  string   `json:"source_file"`
	Locator     string   `json:"locator"`
	Version     string   `json:"version"`
	Score       float64  `json:"score"`
	RerankScore *float64 `json:"rerank_score,omitempty"`
}

type KnowledgeExecutionV1 struct {
	ContractVersion  string               `json:"contract_version"`
	TenantID         string               `json:"tenant_id"`
	UserID           string               `json:"user_id"`
	CollectionID     string               `json:"collection_id"`
	EvidenceIDs      []string             `json:"evidence_ids"`
	Query            string               `json:"query"`
	RetrievalLimit   int                  `json:"retrieval_limit"`
	Passages         []KnowledgePassageV1 `json:"passages"`
	RerankerRole     string               `json:"reranker_role,omitempty"`
	CitationRequired bool                 `json:"citation_required"`
	Abstained        bool                 `json:"abstained"`
	AbstentionReason string               `json:"abstention_reason,omitempty"`
}

func (k KnowledgeExecutionV1) Validate(authoritative ExecutionWorkspaceScopeV1, authorizedEvidence map[string]bool) error {
	if k.ContractVersion != knowledgeExecutionContractV1 || strings.TrimSpace(k.Query) == "" || k.RetrievalLimit < 1 || k.RetrievalLimit > 20 || !k.CitationRequired {
		return errors.New("invalid knowledge execution contract")
	}
	if k.TenantID != authoritative.TenantID || k.UserID != authoritative.SubjectID || k.CollectionID != authoritative.CollectionID {
		return errors.New("knowledge execution scope is not authorized")
	}
	if len(k.Passages) == 0 {
		if !k.Abstained || strings.TrimSpace(k.AbstentionReason) == "" {
			return errors.New("knowledge execution without passages must abstain")
		}
		return nil
	}
	if k.Abstained {
		return errors.New("knowledge execution cannot both cite passages and abstain")
	}
	for _, passage := range k.Passages {
		if passage.PassageID == "" || passage.ChunkID == "" || passage.ChunkHash == "" || passage.EvidenceID == "" || passage.SourceFile == "" || passage.Locator == "" || passage.Version == "" {
			return errors.New("retrieved passage is missing citation provenance")
		}
		if !authorizedEvidence[passage.EvidenceID] {
			return fmt.Errorf("retrieved passage references unauthorized evidence %q", passage.EvidenceID)
		}
	}
	return nil
}

type ModelCapabilityContractV1 struct {
	ContractVersion     string   `json:"contract_version"`
	ModelRole           string   `json:"model_role"`
	InputModalities     []string `json:"input_modalities"`
	OutputContract      string   `json:"output_contract"`
	RequiredArtifacts   []string `json:"required_artifacts"`
	MaturityRequirement string   `json:"maturity_requirement"`
	RuntimeRequirement  string   `json:"runtime_requirement"`
	HardwareRequirement string   `json:"hardware_requirement,omitempty"`
	TimeoutMS           int      `json:"timeout_ms"`
	ResourceBudget      string   `json:"resource_budget"`
	ConfidenceSemantics string   `json:"confidence_semantics"`
	AbstentionSemantics string   `json:"abstention_semantics"`
	CitationSemantics   string   `json:"citation_semantics"`
}

type ModelRuntimeStateV1 struct {
	RuntimeAvailable bool
	AvailableRoles   map[string]bool
	MaturityByRole   map[string]string
	DerivedArtifacts map[string]bool
}

func (m ModelCapabilityContractV1) Validate(state ModelRuntimeStateV1) error {
	if m.ContractVersion != modelCapabilityContractV1 || m.ModelRole == "" || !validModelRole(m.ModelRole) || len(m.InputModalities) == 0 || m.OutputContract == "" || m.TimeoutMS < 1 || m.ResourceBudget == "" || m.ConfidenceSemantics == "" || m.AbstentionSemantics == "" || m.CitationSemantics == "" {
		return errors.New("invalid model capability contract")
	}
	if !state.RuntimeAvailable || !state.AvailableRoles[m.ModelRole] {
		return errors.New("required model role is unavailable")
	}
	if !maturitySatisfies(state.MaturityByRole[m.ModelRole], m.MaturityRequirement) {
		return errors.New("model role maturity is insufficient")
	}
	for _, artifact := range m.RequiredArtifacts {
		if !state.DerivedArtifacts[artifact] {
			return fmt.Errorf("required governed derived artifact %q is unavailable", artifact)
		}
	}
	return nil
}

func validModelRole(role string) bool {
	switch role {
	case "text_reasoning", "embedding", "document_ocr", "anpr_detector", "anpr_ocr_latin", "anpr_ocr_urdu", "audio_asr", "image_understanding", "video_understanding", "reranker":
		return true
	default:
		return false
	}
}

func maturitySatisfies(actual, required string) bool {
	rank := map[string]int{"experimental": 1, "evaluation": 2, "operational": 3}
	return rank[actual] >= rank[required] && rank[required] > 0
}

func extensionExecutionMode(route string) string {
	switch route {
	case "kb", "derived":
		return "knowledge_retrieval"
	case "hybrid":
		return "existing_hybrid"
	default:
		return "deterministic_operation"
	}
}
