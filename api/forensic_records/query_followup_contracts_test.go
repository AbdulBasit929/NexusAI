package main

import (
	"testing"
	"time"
)

func TestAPF34AuditableFollowUpInheritanceAndExplicitPrecedence(t *testing.T) {
	now := time.Date(2026, 8, 17, 10, 0, 0, 0, time.UTC)
	context := queryConversationContext{
		ContractVersion: followUpContextContractV1, ConversationID: "conversation-1", AnalysisID: "analysis-1", TurnID: "turn-1",
		TenantID: "default", UserID: "analyst-1", CollectionID: "case-a", ExpiresAt: now.Add(30 * time.Minute).Format(time.RFC3339),
		Target: "923001110001", Template: "frequent_contacts", DateFrom: "2026-07-10T00:00:00Z", DateTo: "2026-07-11T00:00:00Z", Direction: "OUTGOING",
	}
	req := hybridQueryRequest{TenantID: "default", UserID: "analyst-1", CollectionID: "case-a", Query: "Only outgoing.", ConversationContext: context}
	resolved, plan := applyAuditableConversationContext(req, planRuntimeQuery(req), now)
	if resolved.Target != context.Target || resolved.Template != context.Template || resolved.DateFrom != context.DateFrom || resolved.DateTo != context.DateTo || resolved.Direction != "OUTGOING" {
		t.Fatalf("bounded context was not inherited: %#v", resolved)
	}
	if len(resolved.ConversationContext.InheritedFields) != 5 || plan.QueryPlan.AppliedFilters["conversation_context"] != true {
		t.Fatalf("follow-up provenance missing: %#v %#v", resolved.ConversationContext.InheritedFields, plan.QueryPlan.AppliedFilters)
	}
	for _, inherited := range resolved.ConversationContext.InheritedFields {
		if inherited.SourceAnalysis != "analysis-1" || inherited.SourceTurn != "turn-1" || inherited.Reason == "" || inherited.ExpiresAt == "" {
			t.Fatalf("inherited field is not auditable: %#v", inherited)
		}
	}

	override := req
	override.Query = "Now show only incoming for 923009999999."
	override.Target = "923009999999"
	override.Direction = "INCOMING"
	resolved, _ = applyAuditableConversationContext(override, planRuntimeQuery(override), now)
	if resolved.Target != "923009999999" || resolved.Direction != "INCOMING" {
		t.Fatalf("current turn did not override context: %#v", resolved)
	}
}

func TestAPF34ContextExpiryScopeIsolationAndStandaloneReset(t *testing.T) {
	now := time.Date(2026, 8, 17, 10, 0, 0, 0, time.UTC)
	base := queryConversationContext{
		ContractVersion: followUpContextContractV1, TenantID: "default", UserID: "analyst-a", CollectionID: "case-a",
		ExpiresAt: now.Add(time.Minute).Format(time.RFC3339), Target: "923001110001", Template: "frequent_contacts",
	}
	for _, tc := range []struct {
		name string
		req  hybridQueryRequest
		ctx  queryConversationContext
	}{
		{"case change", hybridQueryRequest{TenantID: "default", UserID: "analyst-a", CollectionID: "case-b", Query: "Only outgoing."}, base},
		{"user change", hybridQueryRequest{TenantID: "default", UserID: "analyst-b", CollectionID: "case-a", Query: "Only outgoing."}, base},
		{"expired", hybridQueryRequest{TenantID: "default", UserID: "analyst-a", CollectionID: "case-a", Query: "Only outgoing."}, func() queryConversationContext {
			c := base
			c.ExpiresAt = now.Add(-time.Second).Format(time.RFC3339)
			return c
		}()},
		{"standalone reset", hybridQueryRequest{TenantID: "default", UserID: "analyst-a", CollectionID: "case-a", Query: "show collection overview"}, base},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.req.ConversationContext = tc.ctx
			resolved, plan := applyAuditableConversationContext(tc.req, planRuntimeQuery(tc.req), now)
			if resolved.Target != "" || len(resolved.ConversationContext.InheritedFields) != 0 || plan.Source == "conversation_context" {
				t.Fatalf("isolated/stale context leaked: %#v %#v", resolved, plan)
			}
		})
	}
}

func TestAPF34ReplacementOnlyFollowUpPreservesIntentAndExplicitTargetWins(t *testing.T) {
	now := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
	context := queryConversationContext{
		ContractVersion: followUpContextContractV1, AnalysisID: "analysis-outgoing", TurnID: "turn-outgoing",
		TenantID: "default", UserID: "analyst-1", CollectionID: "case-a", ExpiresAt: now.Add(30 * time.Minute).Format(time.RFC3339),
		Target: "923001110001", Template: "frequent_contacts", Direction: "OUTGOING",
	}
	for _, query := range []string{
		"For 923001234567 instead.",
		"Switch to 923001234567.",
		"Use 923001234567 instead.",
		"Same analysis for 923001234567.",
		"Now do it for 923001234567.",
		"Do that for 923001234567 instead.",
	} {
		t.Run(query, func(t *testing.T) {
			req := hybridQueryRequest{TenantID: "default", UserID: "analyst-1", CollectionID: "case-a", Query: query, ConversationContext: context}
			resolved, plan := applyAuditableConversationContext(req, planRuntimeQuery(req), now)
			if resolved.Target != "923001234567" || resolved.Template != "frequent_contacts" || resolved.Direction != "OUTGOING" {
				t.Fatalf("replacement did not preserve bounded intent: %#v", resolved)
			}
			if plan.Template != "frequent_contacts" || plan.Target != "923001234567" || plan.QueryPlan.AppliedFilters["conversation_context"] != true {
				t.Fatalf("replacement plan is not executable/auditable: %#v", plan)
			}
		})
	}
}

func TestAPF34ReplacementMarkerDoesNotOverrideStandaloneOperation(t *testing.T) {
	now := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
	context := queryConversationContext{
		ContractVersion: followUpContextContractV1, TenantID: "default", UserID: "analyst-1", CollectionID: "case-a",
		ExpiresAt: now.Add(30 * time.Minute).Format(time.RFC3339), Template: "frequent_contacts", Direction: "OUTGOING",
	}
	req := hybridQueryRequest{TenantID: "default", UserID: "analyst-1", CollectionID: "case-a", Query: "Show temporal CDR activity for 923001234567 instead.", ConversationContext: context}
	resolved, plan := applyAuditableConversationContext(req, planRuntimeQuery(req), now)
	if resolved.Template != "" || plan.Template != "temporal_activity" || resolved.Direction != "" {
		t.Fatalf("standalone operation was overwritten by conversation context: resolved=%#v plan=%#v", resolved, plan)
	}
}

func TestAPF34ClarificationRemainsSpecificAndNonExecuting(t *testing.T) {
	for template, tc := range map[string]struct{ query, want string }{
		"frequent_contacts": {"show frequent contacts for this number", "Which number or subscriber should I analyze for frequent contacts?"},
		"anpr_sightings":    {"show sightings for this plate", "Which plate number, camera, or location should I search ANPR sightings for?"},
		"tower_cdr_join":    {"join records for this entity", "Which exact cell or site identifier should I use for the time-aware CDR-to-reference join?"},
	} {
		if got := clarificationQuestion(tc.query, template); got != tc.want || !needsClarification(tc.query, template, "") {
			t.Fatalf("unsafe or vague clarification for %s: %q", template, got)
		}
	}
}
