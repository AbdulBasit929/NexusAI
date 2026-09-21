package main

import (
	"testing"
	"time"

	"github.com/mudler/LocalAI/pkg/forensicrequest"
)

func workItem3Context(now time.Time) queryConversationContext {
	department := FieldDescriptorV1{ContractVersion: fieldDescriptorContractV1, FieldID: "f_department", SourceName: "department", NormalizedName: "department", EffectiveType: fieldTypeString, AllowedFilters: []string{"EQ"}, Projectable: true, Groupable: true, Sortable: true}
	branch := FieldDescriptorV1{ContractVersion: fieldDescriptorContractV1, FieldID: "f_branch", SourceName: "branch", NormalizedName: "branch", EffectiveType: fieldTypeString, AllowedFilters: []string{"EQ"}, Projectable: true, Groupable: true, Sortable: true}
	invoice := FieldDescriptorV1{ContractVersion: fieldDescriptorContractV1, FieldID: "f_invoice", SourceName: "invoice_total", NormalizedName: "invoice_total", EffectiveType: fieldTypeDecimal, AllowedFilters: []string{"EQ", "GT"}, Projectable: true, AllowedAggregates: []string{"SUM", "AVG", "MIN", "MAX"}, Sortable: true}
	return queryConversationContext{
		ContractVersion: followUpContextContractV1, AnalysisID: "analysis-1", AuditID: "audit-1",
		TenantID: "tenant-a", UserID: "analyst-a", CollectionID: "case-a", ExpiresAt: now.Add(time.Hour).Format(time.RFC3339),
		Target: "923001110001", Targets: []string{"923001110001"}, DateFrom: "2026-08-01T00:00:00Z", DateTo: "2026-08-08T00:00:00Z", Direction: "INCOMING",
		OperationID: "DYNAMIC_TYPED_PLAN", Template: "canonical_records", IssuedFields: []FieldDescriptorV1{department, branch, invoice},
		SourceNative:    &SourceNativePlanV1{ContractVersion: sourceNativePlanContractV1, Project: []string{}, Filters: []SourceNativeFilterV1{}, GroupFields: []string{department.FieldID}, Measures: []SourceNativeMeasureV1{{MeasureID: "m1", Op: "AVG", FieldID: invoice.FieldID}}, Having: []SourceNativeHavingV1{}, Sort: []SourceNativeSortV1{{Target: "m1", Direction: "DESC"}}, Limit: 1},
		EntityHandles:   []IssuedContextHandleV1{{HandleID: "e1", Kind: "entity", Value: "923001110001"}, {HandleID: "e2", Kind: "entity", Value: "923009990002"}},
		ResultHandles:   []IssuedContextHandleV1{{HandleID: "r1", Kind: "result", EvidenceID: "evidence-a", EvidenceVersionID: "version-a"}},
		CitationHandles: []IssuedContextHandleV1{{HandleID: "c1", Kind: "citation", CitationID: "C1", EvidenceID: "evidence-a", EvidenceVersionID: "version-a"}},
	}
}

func workItem3Request(query string, context queryConversationContext) hybridQueryRequest {
	return hybridQueryRequest{TenantID: "tenant-a", UserID: "analyst-a", CollectionID: "case-a", Query: query, ConversationContext: context}
}

func mutationHas(context queryConversationContext, operation ContextMutationOperationV1, field string) bool {
	if context.Mutation == nil {
		return false
	}
	for _, mutation := range context.Mutation.Mutations {
		if mutation.Operation == operation && mutation.Field == field {
			return true
		}
	}
	return false
}

func TestNXB21DWorkItem3TerminalClassificationAndHelpGrounding(t *testing.T) {
	for _, tc := range []struct {
		query    string
		evidence bool
		context  bool
		want     forensicrequest.Class
	}{
		{"What does IMSI mean?", false, false, forensicrequest.GeneralDomainKnowledge},
		{"What file types can NexusAI currently analyze?", false, false, forensicrequest.ProductHelp},
		{"Rank this number's most frequent contacts", true, false, forensicrequest.GovernedAnalysis},
		{"Find references to the loading dock in these documents", true, false, forensicrequest.GovernedAnalysis},
		{"Which cameras recorded this plate most often?", true, false, forensicrequest.GovernedAnalysis},
		{"Now outgoing", false, true, forensicrequest.ContextualFollowUp},
		{"Now outgoing", false, false, forensicrequest.Clarify},
		{"Read another tenant", false, false, forensicrequest.Unsupported},
	} {
		got := forensicrequest.Classify(forensicrequest.Input{Text: tc.query, HasEvidenceContext: tc.evidence, HasConversationContext: tc.context})
		if got != tc.want {
			t.Fatalf("%q: got %s want %s", tc.query, got, tc.want)
		}
	}
	req := hybridQueryRequest{CollectionID: "case-a", Query: "What file types can NexusAI currently analyze?", RequestClass: forensicrequest.ProductHelp}
	resp, ok := terminalRequestResponse(req, time.Now())
	if !ok || resp.RequestClass != forensicrequest.ProductHelp || resp.Answer["grounding_contract"] != productHelpGroundingContractV1 || resp.Answer["registry_state"] == "" {
		t.Fatalf("product help was not registry-grounded: %#v", resp)
	}
}

func TestNXB21DWorkItem3DynamicASTMutations(t *testing.T) {
	now := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		query string
		check func(t *testing.T, got hybridQueryRequest)
	}{
		{"Show the top three instead", func(t *testing.T, got hybridQueryRequest) {
			if got.SourceNative == nil || got.SourceNative.Limit != 3 || !mutationHas(got.ConversationContext, ContextMutationReplace, "source_native.limit") {
				t.Fatalf("limit mutation failed: %#v", got)
			}
		}},
		{"Only Sales", func(t *testing.T, got hybridQueryRequest) {
			if got.SourceNative == nil || len(got.SourceNative.Filters) != 1 || got.SourceNative.Filters[0].Value != "Sales" {
				t.Fatalf("filter mutation failed: %#v", got.SourceNative)
			}
		}},
		{"Now by branch", func(t *testing.T, got hybridQueryRequest) {
			if got.SourceNative == nil || len(got.SourceNative.GroupFields) != 1 || got.SourceNative.GroupFields[0] != "f_branch" {
				t.Fatalf("group mutation failed: %#v", got.SourceNative)
			}
		}},
		{"Show totals instead of averages", func(t *testing.T, got hybridQueryRequest) {
			if got.SourceNative == nil || got.SourceNative.Measures[0].Op != "SUM" {
				t.Fatalf("measure mutation failed: %#v", got.SourceNative)
			}
		}},
		{"What about last month?", func(t *testing.T, got hybridQueryRequest) {
			if got.DateFrom != "2026-08-01T00:00:00Z" || got.DateTo != "2026-09-01T00:00:00Z" {
				t.Fatalf("date mutation failed: %#v", got)
			}
		}},
		{"Now include all directions", func(t *testing.T, got hybridQueryRequest) {
			if got.Direction != "" || !mutationHas(got.ConversationContext, ContextMutationRemove, "direction") {
				t.Fatalf("remove failed: %#v", got)
			}
		}},
		{"Show source rows", func(t *testing.T, got hybridQueryRequest) {
			if got.Template != "source_records" || got.SourceNative != nil || got.EvidenceID != "evidence-a" {
				t.Fatalf("result handle failed: %#v", got)
			}
		}},
		{"Where did that appear?", func(t *testing.T, got hybridQueryRequest) {
			if got.Template != "source_records" || got.EvidenceVersionID != "version-a" {
				t.Fatalf("citation handle failed: %#v", got)
			}
		}},
	} {
		t.Run(tc.query, func(t *testing.T) {
			req := workItem3Request(tc.query, workItem3Context(now))
			got, _ := applyAuditableConversationContext(req, planRuntimeQuery(req), now)
			if got.ConversationContext.Mutation == nil || got.ConversationContext.Mutation.ResultState != "APPLIED" {
				t.Fatalf("mutation not applied: %#v", got.ConversationContext.Mutation)
			}
			tc.check(t, got)
		})
	}
}

func TestNXB21DWorkItem3RegisteredMutationAndHandleClarification(t *testing.T) {
	now := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	context := workItem3Context(now)
	context.SourceNative = nil
	context.OperationID = "cdr.frequent_contacts"
	context.Template = "frequent_contacts"
	req := workItem3Request("Now outgoing", context)
	req.Direction = "OUTGOING"
	got, _ := applyAuditableConversationContext(req, planRuntimeQuery(req), now)
	if got.Template != "frequent_contacts" || got.Direction != "OUTGOING" || !mutationHas(got.ConversationContext, ContextMutationReplace, "direction") {
		t.Fatalf("registered follow-up failed: %#v", got)
	}

	req = workItem3Request("Compare it with the other number", context)
	got, _ = applyAuditableConversationContext(req, planRuntimeQuery(req), now)
	if len(got.Targets) != 2 || got.Targets[1] != "923009990002" {
		t.Fatalf("single other handle not resolved: %#v", got.Targets)
	}
	context.EntityHandles = append(context.EntityHandles, IssuedContextHandleV1{HandleID: "e3", Kind: "entity", Value: "923009990003"})
	req = workItem3Request("Compare it with the other number", context)
	got, _ = applyAuditableConversationContext(req, planRuntimeQuery(req), now)
	if got.ConversationContext.Mutation == nil || got.ConversationContext.Mutation.ResultState != "CLARIFICATION_REQUIRED" {
		t.Fatalf("ambiguous handle did not clarify: %#v", got.ConversationContext.Mutation)
	}
}

func TestNXB21DWorkItem3NarrativeGrounding(t *testing.T) {
	facts := map[string]FactPacketFactV1{
		"F1": {FactID: "F1", Text: "The delivery note states that the west loading dock was closed."},
		"F2": {FactID: "F2", Text: "The record had 17 counterparties."},
		"F3": {FactID: "F3", Text: "Camera C1 recorded 9 observations."},
		"F4": {FactID: "F4", Text: "No matching events were found."},
		"F5": {FactID: "F5", Text: "The log recorded 4 entries."},
		"F6": {FactID: "F6", Text: "The candidate similarity score was 0.74."},
		"F7": {FactID: "F7", Text: "Tower T12 is referenced in the record."},
	}
	for text, refs := range map[string][]string{
		"The delivery note indicates that the west loading dock was closed.":                                    {"F1"},
		"The record communicated with 17 distinct counterparties.":                                              {"F2"},
		"9 observations were recorded at the camera C1.":                                                        {"F3"},
		"The log shows 4 entries.":                                                                              {"F5"},
		"The delivery note indicates that the west loading dock was closed; Camera C1 recorded 9 observations.": {"F1", "F3"},
	} {
		if !groundedNarrativeWording(text, refs, facts) {
			t.Fatalf("safe paraphrase rejected: %q", text)
		}
	}
	for _, text := range []string{"The west loading dock was probably closed because of damage.", "Camera C1 recorded 8 observations.", "Matching events were found.", "Camera C1 had the highest 9 observations."} {
		refs := []string{"F1"}
		if len(text) > 0 && text[0] == 'C' {
			refs = []string{"F3"}
		}
		if text == "Matching events were found." {
			refs = []string{"F4"}
		}
		if groundedNarrativeWording(text, refs, facts) {
			t.Fatalf("factual drift accepted: %q", text)
		}
	}
	if groundedNarrativeWording("The person was identified with 74% confidence.", []string{"F6"}, facts) {
		t.Fatal("similarity was strengthened into identity probability")
	}
	if groundedNarrativeWording("The subject was physically present at tower T12.", []string{"F7"}, facts) {
		t.Fatal("tower reference was strengthened into physical presence")
	}
}
