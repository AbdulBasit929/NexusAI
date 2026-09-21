package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenEndedSemanticPlannerProjectsFullQueryableRegistry(t *testing.T) {
	req := hybridQueryRequest{QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)}}
	candidates := semanticOperationCandidates(req)
	expected := 0
	for _, template := range supportedQueryTemplates() {
		if template.ExposureStatus != "engineering_only" && template.ScopeMode != "evidence_required" {
			expected++
		}
	}
	if len(candidates) != expected || expected < 60 {
		t.Fatalf("semantic planner exposes %d operation meanings; want all %d scope-compatible non-engineering operations", len(candidates), expected)
	}
	promptPayload, err := json.Marshal(candidates)
	if err != nil {
		t.Fatalf("marshal semantic candidates: %v", err)
	}
	t.Logf("workspace semantic candidate payload bytes=%d", len(promptPayload))
	retrieved := rankSemanticOperations("Summarize communication service usage", candidates)
	compactPayload, err := json.Marshal(semanticOperationPromptCandidates(retrieved))
	if err != nil {
		t.Fatalf("marshal compact semantic candidates: %v", err)
	}
	t.Logf("compact workspace semantic candidate payload bytes=%d", len(compactPayload))
	if len(compactPayload) >= 12_000 {
		t.Fatalf("compact semantic prompt payload is %d bytes; want less than 12000 for the accepted 4096-token local profile", len(compactPayload))
	}
	for index, candidate := range retrieved {
		promptCandidate := semanticOperationPromptCandidates(retrieved)[index]
		if promptCandidate.OperationID != candidate.OperationID || promptCandidate.FamilyID != candidate.FamilyID || promptCandidate.Description != candidate.Description {
			t.Fatalf("compact semantic prompt lost identity/meaning for %q", candidate.OperationID)
		}
	}
	wanted := map[string]bool{"cdr.service_usage": false, "financial.transaction_summary": false, "forensics.canonical_records": false}
	for _, candidate := range candidates {
		if candidate.ExposureStatus == "engineering_only" {
			t.Fatalf("engineering-only operation %q was exposed to the planner", candidate.OperationID)
		}
		if _, ok := wanted[candidate.OperationID]; ok {
			wanted[candidate.OperationID] = true
		}
	}
	for operationID, found := range wanted {
		if !found {
			t.Fatalf("full-registry semantic candidate %q is missing", operationID)
		}
	}
}

func TestOpenEndedSemanticPlannerSelectsMeaningAndServerBindsFacts(t *testing.T) {
	var captured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode planner request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"finish_reason": "stop", "message": map[string]string{"content": `{"decision":"OPERATION:cdr.service_usage"}`}}},
		})
	}))
	defer server.Close()

	req := hybridQueryRequest{
		TenantID: "tenant-a", UserID: "analyst-a", CollectionID: "case-a",
		Query:          "Explain how 923001110001 used the available communication channels between 2026-08-01 and 2026-08-03.",
		SynthesisModel: "qwen_qwen3-4b-instruct-2507",
		QueryScope:     queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)},
	}
	resolved, status := resolveSemanticOperationInCandidates(context.Background(), config{LocalAIURL: server.URL}, req, semanticOperationCandidates(req))
	if status != "semantic_model_plan" || resolved.Template != "service_usage" {
		t.Fatalf("open-ended semantic plan failed: status=%s request=%#v", status, resolved)
	}
	if resolved.Target != "923001110001" || resolved.DateFrom != "2026-08-01T00:00:00Z" || resolved.DateTo != "2026-08-04T00:00:00Z" {
		t.Fatalf("server did not bind deterministic target/time facts: %#v", resolved)
	}
	if resolved.TenantID != req.TenantID || resolved.UserID != req.UserID || resolved.CollectionID != req.CollectionID {
		t.Fatal("semantic proposal changed authoritative scope")
	}
	if resolved.SemanticPlannerAudit == nil || resolved.SemanticPlannerAudit.FactAuthority != "SERVER_DETERMINISTIC_ONLY" {
		t.Fatal("semantic planner audit did not preserve deterministic fact authority")
	}

	format := captured["response_format"].(map[string]any)
	schema := format["json_schema"].(map[string]any)["schema"].(map[string]any)
	properties := schema["properties"].(map[string]any)
	if len(properties) != 1 || properties["decision"] == nil {
		t.Fatalf("model schema contains authority-bearing fields: %#v", properties)
	}
	messages := captured["messages"].([]any)
	system := messages[0].(map[string]any)["content"].(string)
	for _, rule := range []string{"unrestricted English wording", "requested measure", "server alone owns scope"} {
		if !strings.Contains(strings.ToLower(system), strings.ToLower(rule)) {
			t.Fatalf("semantic planning rule %q is missing", rule)
		}
	}
}

func TestOpenEndedSemanticPlannerRejectsUnissuedAndFactBearingOutput(t *testing.T) {
	candidates := semanticOperationCandidates(hybridQueryRequest{QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)}})
	for _, raw := range []string{
		`{"decision":"OPERATION:shell.exec"}`,
		`{"decision":"OPERATION:cdr.service_usage","target":"923009999999"}`,
		`{"decision":"OPERATION:cdr.service_usage"} trailing`,
	} {
		if _, err := decodeSemanticOperationProposal([]byte(raw), candidates); err == nil {
			t.Fatalf("unsafe semantic proposal accepted: %s", raw)
		}
	}
}

func TestOpenEndedSemanticPlannerScopesSelectedEvidenceCandidates(t *testing.T) {
	req := hybridQueryRequest{
		EvidenceID: "10000000-0000-4000-8000-000000000099",
		QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeSelected), SourceFamily: "document"},
	}
	for _, candidate := range semanticOperationCandidates(req) {
		if candidate.FamilyID == "communications_cdr" || candidate.FamilyID == "video_intelligence" {
			t.Fatalf("selected document scope leaked unrelated operation %q", candidate.OperationID)
		}
	}
}
