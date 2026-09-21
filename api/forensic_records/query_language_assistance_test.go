package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func validLanguageProposal() LanguageAssistanceProposalV1 {
	return LanguageAssistanceProposalV1{
		ContractVersion: languageAssistanceProposalV1, OperationID: "cdr.frequent_contacts", FamilyID: "communications_cdr",
		Parameters:     LanguageAssistanceParametersV1{Target: "923001110001", Direction: "OUTGOING"},
		OutputContract: queryUnderstandingContractV1, Confidence: 0.91,
	}
}

func TestAPF35SchemaBoundProposalValidation(t *testing.T) {
	proposal := validLanguageProposal()
	req := hybridQueryRequest{TenantID: "default", UserID: "analyst", CollectionID: "case-a", Query: "923001110001 ke bahar walay rabtay", Direction: "OUTGOING"}
	resolved, err := applyLanguageAssistanceProposal(req, proposal)
	if err != nil || resolved.Template != "frequent_contacts" || resolved.Target != "923001110001" || resolved.Direction != "OUTGOING" {
		t.Fatalf("valid proposal rejected: %#v %v", resolved, err)
	}
	if resolved.TenantID != req.TenantID || resolved.UserID != req.UserID || resolved.CollectionID != req.CollectionID {
		t.Fatal("model proposal changed authoritative scope")
	}

	invalid := []LanguageAssistanceProposalV1{proposal, proposal, proposal, proposal}
	invalid[0].OperationID = "cdr.invented"
	invalid[1].FamilyID = "network_ipdr"
	invalid[2].OutputContract = "run_sql"
	invalid[3].Parameters.Direction = "BOTH_OR_SECRET"
	for _, candidate := range invalid {
		if _, err := validateLanguageAssistanceProposal(candidate); err == nil {
			t.Fatalf("unsafe proposal accepted: %#v", candidate)
		}
	}
}

func TestAPF35StrictDecoderRejectsUnknownAndMalformedOutput(t *testing.T) {
	valid, _ := json.Marshal(validLanguageProposal())
	if _, err := decodeLanguageAssistanceProposal(valid); err != nil {
		t.Fatalf("valid strict JSON rejected: %v", err)
	}
	for _, raw := range [][]byte{
		[]byte(`not-json`),
		[]byte(`{"contract_version":"forensics.language-assistance-proposal/v1","operation_id":"cdr.frequent_contacts","family_id":"communications_cdr","parameters":{},"output_contract":"forensics.query-understanding/v1","confidence":1,"collection_id":"case-b"}`),
		append(valid, []byte(` trailing`)...),
	} {
		if _, err := decodeLanguageAssistanceProposal(raw); err == nil {
			t.Fatalf("unsafe output accepted: %s", raw)
		}
	}
}

func TestAPF35LanguageProposalSchemaConstrainsOperationAndFamily(t *testing.T) {
	schema := languageProposalJSONSchema()
	properties := schema["properties"].(map[string]any)
	if _, modelControlsFamily := properties["family_id"]; modelControlsFamily {
		t.Fatal("language proposal schema lets the model control the canonical operation family")
	}
	proposal := validLanguageProposal()
	proposal.FamilyID = ""
	if _, err := validateLanguageAssistanceProposal(proposal); err != nil {
		t.Fatalf("server-derived family proposal rejected: %v", err)
	}
	for _, unsafe := range []string{"run_sql", "https://example.test", "shell.exec"} {
		proposal.OperationID = unsafe
		if _, err := validateLanguageAssistanceProposal(proposal); err == nil {
			t.Fatalf("validator accepted unsafe operation %q", unsafe)
		}
	}
}

func TestAPF35QueryBoundedSchemaPreventsInventedOperationAndDirection(t *testing.T) {
	schema := languageProposalJSONSchema("923001234567 cdr actvty 10 july 2026 pls")
	properties := schema["properties"].(map[string]any)
	operations := properties["operation_id"].(map[string]any)["enum"].([]string)
	if len(operations) != 1 || operations[0] != "cdr.temporal_activity" {
		t.Fatalf("CDR activity schema was not bounded to temporal activity: %#v", operations)
	}
	parameterProperties := properties["parameters"].(map[string]any)["properties"].(map[string]any)
	directions := parameterProperties["direction"].(map[string]any)["enum"].([]string)
	if len(directions) != 1 || directions[0] != "" {
		t.Fatalf("schema allowed an unrequested direction: %#v", directions)
	}
	targets := parameterProperties["target"].(map[string]any)["enum"].([]string)
	if len(targets) != 2 || targets[0] != "" || targets[1] != "923001234567" {
		t.Fatalf("schema allowed a model-controlled target: %#v", targets)
	}
	if allowlist := languageAssistanceAllowlist("923001234567 cdr actvty 10 july 2026 pls"); !strings.Contains(allowlist, "cdr.temporal_activity") || strings.Contains(allowlist, "cdr.frequent_contacts") {
		t.Fatalf("query-bounded allowlist is incorrect: %s", allowlist)
	}
}

func TestAPF35DeterministicFastPathAndAssistedFallback(t *testing.T) {
	if languageAssistanceTimeout < 30*time.Second || languageAssistanceTimeout > 60*time.Second {
		t.Fatalf("language-assistance timeout %s is outside the bounded local-model acceptance window", languageAssistanceTimeout)
	}
	clean := hybridQueryRequest{Query: "who does 923001110001 contact most?", SynthesisModel: "qwen_qwen3-4b-instruct-2507"}
	if planner := planRuntimeQuery(clean); shouldUseLanguageAssistance(clean, planner) {
		t.Fatal("canonical deterministic query would call the model")
	}

	proposalJSON, _ := json.Marshal(map[string]string{"decision": "cdr.frequent_contacts/TOP_K/authorized_workspace"})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request map[string]any
		_ = json.NewDecoder(r.Body).Decode(&request)
		content := string(proposalJSON)
		if raw, err := json.Marshal(request["response_format"]); err == nil && strings.Contains(string(raw), "OPERATION:cdr.frequent_contacts") {
			content = `{"decision":"OPERATION:cdr.frequent_contacts"}`
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []map[string]any{{"finish_reason": "stop", "message": map[string]string{"content": content}}}})
	}))
	defer server.Close()
	ambiguous := hybridQueryRequest{TenantID: "default", UserID: "analyst", CollectionID: "case-a", Query: "923001110001 ke sirf bahar walay rabtay dikhao", Direction: "OUTGOING", SynthesisModel: "qwen_qwen3-4b-instruct-2507"}
	resolved, status := resolveWithLanguageAssistance(context.Background(), config{LocalAIURL: server.URL}, ambiguous)
	if !hybridResolvedStatus(status) || resolved.Template != "frequent_contacts" {
		t.Fatalf("schema-bound assisted route failed: %s %#v", status, resolved)
	}

	badServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"content":"not-json"}}]}`))
	}))
	defer badServer.Close()
	resolved, status = resolveWithLanguageAssistance(context.Background(), config{LocalAIURL: badServer.URL}, ambiguous)
	if status != "malformed_residual" || resolved.Template != "" {
		t.Fatalf("malformed proposal did not fall back safely: %s %#v", status, resolved)
	}
}

func TestAPF35ExistingMultilingualDeterministicSemantics(t *testing.T) {
	for _, query := range []string{
		"subscriber ki tafseel 0300-1234567 ke liye dikhao",
		"اس نمبر 0300-1234567 کی سبسکرائبر کی تفصیل دکھائیں",
		"subscriber ki tafseel 0300-1234567 ke liye dikhao please",
	} {
		plan := planRuntimeQuery(hybridQueryRequest{Query: query})
		if plan.Template != "subscriber_identity_lookup" {
			t.Fatalf("multilingual deterministic semantics changed for %q: %#v", query, plan)
		}
	}
}

func TestNXB21DLanguageAssistanceUsesCompleteCompactResponseEnvelope(t *testing.T) {
	var captured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("request body was not JSON: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"content":"not-json"}}]}`))
	}))
	defer server.Close()

	req := hybridQueryRequest{
		Query:          "list communications linked with 03175550123",
		SynthesisModel: "qwen_qwen3-4b-instruct-2507",
		QueryScope:     queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)},
	}
	_, _ = resolveHybridPlanner(context.Background(), config{LocalAIURL: server.URL}, req)

	if dynamicPlannerMaxCompletionTokens != 512 {
		t.Fatalf("dynamic planner completion constant = %d, want 512", dynamicPlannerMaxCompletionTokens)
	}
	if got := int(captured["max_tokens"].(float64)); got != dynamicPlannerMaxCompletionTokens {
		t.Fatalf("dynamic planner completion budget = %d, want %d", got, dynamicPlannerMaxCompletionTokens)
	}
	messages := captured["messages"].([]any)
	system := messages[0].(map[string]any)["content"].(string)
	for _, instruction := range []string{"one-field JSON object", "Never generate facts", "CLARIFY:AMBIGUOUS_INTENT"} {
		if !strings.Contains(system, instruction) {
			t.Fatalf("compact response instruction %q is missing", instruction)
		}
	}
}

func TestNXB21DCompactOutputContractPreservesRequiredSemantics(t *testing.T) {
	req := hybridQueryRequest{Query: `search this selected document for "quartz dispatch folio"`, QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeSelected)}, EvidenceID: "10000000-0000-4000-8000-000000000099"}
	schema := dynamicPlannerJSONSchema(req)
	required := schema["required"].([]string)
	if len(required) != 12 {
		t.Fatalf("production proposal requires %d top-level fields, want 12", len(required))
	}
	for _, field := range []string{"query_capability_id", "semantic", "literal_text", "scope_intent", "parameters", "confidence"} {
		if !containsString(required, field) {
			t.Fatalf("semantic field %q is no longer required", field)
		}
	}
	parameters := schema["properties"].(map[string]any)["parameters"].(map[string]any)
	requiredParameters, ok := parameters["required"].([]string)
	if !ok || len(requiredParameters) != 1 || requiredParameters[0] != "text_query" {
		t.Fatalf("phrase request required parameters = %#v, want text_query", parameters["required"])
	}
	properties := parameters["properties"].(map[string]any)
	for _, field := range []string{"target", "targets", "text_query", "date_from", "date_to", "direction"} {
		if _, ok := properties[field]; !ok {
			t.Fatalf("nested semantic parameter %q was lost", field)
		}
	}
}

func TestNXB21DProductionSchemaBindsServerDerivedFields(t *testing.T) {
	req := hybridQueryRequest{
		Query:      "List the five most frequent contacts for 03215587042 between 2026-11-03 and 2026-11-05.",
		QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)},
	}
	schema := dynamicPlannerJSONSchema(req)
	properties := schema["properties"].(map[string]any)
	if got := properties["top_k"].(map[string]any)["const"]; got != 5 {
		t.Fatalf("top_k const = %#v, want 5", got)
	}
	if got := properties["start_seconds"].(map[string]any)["const"]; got != nil {
		t.Fatalf("unused start_seconds const = %#v, want nil", got)
	}
	parameters := properties["parameters"].(map[string]any)
	parameterProperties := parameters["properties"].(map[string]any)
	if _, ok := parameterProperties["text_query"]; ok {
		t.Fatal("non-text request schema permits an unsolicited text_query")
	}
	if got := parameterProperties["date_from"].(map[string]any)["const"]; got != "2026-11-03T00:00:00Z" {
		t.Fatalf("date_from const = %#v", got)
	}
	if got := parameterProperties["date_to"].(map[string]any)["const"]; got != "2026-11-06T00:00:00Z" {
		t.Fatalf("date_to const = %#v", got)
	}
}

func TestNXB21DProductionSchemaBindsPhraseAndSourceTime(t *testing.T) {
	req := hybridQueryRequest{
		Query:      `Within this selected recording, find the exact phrase "copper ledger annex".`,
		EvidenceID: "10000000-0000-4000-8000-000000000099",
		QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeSelected), SourceFamily: "audio_intelligence"},
	}
	schema := dynamicPlannerJSONSchema(req)
	properties := schema["properties"].(map[string]any)
	if got := properties["semantic"].(map[string]any)["enum"].([]string); len(got) != 1 || got[0] != "EXACT_PHRASE" {
		t.Fatalf("phrase semantic enum = %#v", got)
	}
	if got := properties["literal_text"].(map[string]any)["const"]; got != "copper ledger annex" {
		t.Fatalf("literal const = %#v", got)
	}
	parameters := properties["parameters"].(map[string]any)
	if required := parameters["required"].([]string); len(required) != 1 || required[0] != "text_query" {
		t.Fatalf("phrase parameter requirements = %#v", required)
	}
	timeReq := hybridQueryRequest{
		Query:      "Show this selected recording from 00:10 to 00:20.",
		EvidenceID: req.EvidenceID,
		QueryScope: req.QueryScope,
	}
	timeProperties := dynamicPlannerJSONSchema(timeReq)["properties"].(map[string]any)
	if got := timeProperties["start_seconds"].(map[string]any)["const"]; got != float64(10) {
		t.Fatalf("start_seconds const = %#v", got)
	}
	if got := timeProperties["end_seconds"].(map[string]any)["const"]; got != float64(20) {
		t.Fatalf("end_seconds const = %#v", got)
	}
}

func TestNXB21DPlannerTopKUsesBoundedMultilingualSourceWords(t *testing.T) {
	cases := []struct {
		query string
		want  int
		ok    bool
	}{
		{"List the five most frequent contacts", 5, true},
		{"top 7 nearest results", 7, true},
		{"paanch sab se zyada rabtay dikhao", 5, true},
		{"پانچ زیادہ رابطے دکھائیں", 5, true},
		{"observations within five minutes", 0, false},
	}
	for _, tc := range cases {
		got, ok := extractPlannerTopK(tc.query)
		if got != tc.want || ok != tc.ok {
			t.Fatalf("extractPlannerTopK(%q) = (%d,%t), want (%d,%t)", tc.query, got, ok, tc.want, tc.ok)
		}
	}
}

func TestNXB21DRequestBoundProposalsPassProductionValidation(t *testing.T) {
	dateFrom, dateTo := extractDateRange("Summarize call activity for 03350812764 from 2026-12-07 through 2026-12-09.")
	cases := []struct {
		name     string
		req      hybridQueryRequest
		proposal DynamicPlannerProposalV1
	}{
		{
			name:     "identifier without synthetic optional fields",
			req:      hybridQueryRequest{Query: "Locate case communications tied to 03097645218 across all authorized evidence.", QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)}},
			proposal: DynamicPlannerProposalV1{ContractVersion: dynamicPlannerProposalV1, QueryCapabilityID: "cdr.identifier_lookup", Semantic: "EXACT_VALUE", Target: "03097645218", ScopeIntent: "authorized_workspace", Parameters: LanguageAssistanceParametersV1{Target: "03097645218"}, Confidence: 1},
		},
		{
			name:     "word bounded top k",
			req:      hybridQueryRequest{Query: "Rank the seven most frequent contacts of 03248715069 in the whole investigation.", QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)}},
			proposal: DynamicPlannerProposalV1{ContractVersion: dynamicPlannerProposalV1, QueryCapabilityID: "cdr.frequent_contacts", Semantic: "TOP_K", Target: "03248715069", ScopeIntent: "authorized_workspace", TopK: 7, Parameters: LanguageAssistanceParametersV1{Target: "03248715069"}, Confidence: 1},
		},
		{
			name:     "calendar bounds without recording bounds",
			req:      hybridQueryRequest{Query: "Summarize call activity for 03350812764 from 2026-12-07 through 2026-12-09.", QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)}},
			proposal: DynamicPlannerProposalV1{ContractVersion: dynamicPlannerProposalV1, QueryCapabilityID: "cdr.time_activity", Semantic: "AGGREGATE", Target: "03350812764", ScopeIntent: "authorized_workspace", Parameters: LanguageAssistanceParametersV1{Target: "03350812764", DateFrom: dateFrom, DateTo: dateTo}, Confidence: 1},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := validateDynamicPlannerProposal(tc.req, tc.proposal); err != nil {
				t.Fatalf("request-bound proposal rejected: %v", err)
			}
		})
	}
}
