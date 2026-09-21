package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestConvergenceRetrievalBoundary(t *testing.T) {
	for _, tc := range []struct{ query, family, op string }{
		{"Summarize repeated communications between counterparties.", "communications_cdr", "cdr.frequent_contacts"},
		{"List camera activity from plate observations.", "anpr_vehicles", "anpr.camera_activity"},
		{"Retrieve relevant written passages from documents.", "document_intelligence", "document.search"},
		{"Search the speech transcript for a topic.", "audio_intelligence", "audio.transcript_search"},
		{"Inspect tabular rows from the worksheet.", "generic_tabular", "generic.filter_records"},
	} {
		t.Run(tc.family, func(t *testing.T) {
			calls := 0
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]any
				_ = json.NewDecoder(r.Body).Decode(&body)
				calls++
				raw, _ := json.Marshal(body)
				if strings.Contains(string(raw), "INSUFFICIENT_FACTS") || strings.Contains(string(raw), "GENERAL_PRODUCT_HELP") || strings.Contains(string(raw), "FAMILY:") {
					t.Error("operation schema contains a retired semantic decision")
				}
				if !strings.Contains(string(raw), tc.op) {
					t.Errorf("expected operation was not retrieved: %s", tc.op)
				}
				d := "OPERATION:" + tc.op
				_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]string{"content": `{"decision":"` + d + `"}`}}}})
			}))
			defer s.Close()
			got, state := resolveOpenEndedSemanticPlanner(context.Background(), config{LocalAIURL: s.URL}, hybridQueryRequest{TenantID: "t", CollectionID: "c", Query: tc.query, SynthesisModel: "qwen_qwen3-4b-instruct-2507"})
			if state != "semantic_deterministic_registered" || got.SemanticPlannerAudit.Selected == nil || got.SemanticPlannerAudit.Selected.OperationID != tc.op {
				t.Fatalf("%s %#v", state, got.SemanticPlannerAudit)
			}
			if calls != 0 {
				t.Fatal(calls)
			}
		})
	}
	for _, q := range []string{"Define an IMSI.", "What is an IMEI?", "Explain the meaning of a subscriber identifier."} {
		_, state := resolveOpenEndedSemanticPlanner(context.Background(), config{}, hybridQueryRequest{Query: q})
		if state != "GENERAL_PRODUCT_HELP" {
			t.Errorf("concept: %s %s", q, state)
		}
	}
	for _, q := range []string{"Explain these records.", "What is the total call count?", "What does this document state?"} {
		if semanticConceptHelp(hybridQueryRequest{Query: q}) {
			t.Errorf("analytical request classified help: %s", q)
		}
	}
}

func TestConvergenceDescriptorsAndRanking(t *testing.T) {
	all := semanticOperationCandidates(hybridQueryRequest{})
	for _, g := range semanticFamilyGroups(all) {
		if !strings.Contains(g.Meaning, "Data:") || !strings.Contains(g.Meaning, "Analysis:") || !strings.Contains(g.Meaning, "Results:") {
			t.Errorf("incomplete meaning: %#v", g)
		}
	}
	for _, c := range all {
		if c.ExposureStatus == "engineering_only" {
			t.Fatal("engineering operation exposed")
		}
	}
	for _, tc := range []struct{ q, op string }{
		{"Summarize communication frequency by counterparty.", "cdr.frequent_contacts"},
		{"Analyze communications by hour and day.", "cdr.temporal_activity"},
		{"Review changes in device and SIM identifiers.", "cdr.device_identity_changes"},
	} {
		cs := rankSemanticOperations(tc.q, semanticFamilyCandidates(all, "communications_cdr"))
		found := false
		for _, c := range cs {
			found = found || c.OperationID == tc.op
		}
		if len(cs) > semanticOperationTopK || !found {
			t.Errorf("recall miss %s: %+v", tc.op, cs)
		}
	}
}

func TestConvergenceDynamicAlgebra(t *testing.T) {
	req := hybridQueryRequest{TenantID: "t", UserID: "u", CollectionID: "c", Query: "Count generic rows by source file.", RecordType: "generic"}
	p := semanticDynamicProposal{Mode: "group", Group: &CanonicalGroupV1{Field: "source_file", Measure: "count"}}
	got, err := applySemanticDynamicPlan(req, "generic_tabular", p)
	if err != nil || got.Template != "canonical_records" || got.Group == nil || got.TenantID != req.TenantID || got.CollectionID != req.CollectionID {
		t.Fatalf("dynamic group: %+v %v", got, err)
	}
	if !hybridResolvedStatus("semantic_dynamic_plan") {
		t.Fatal("dynamic plan cannot reach execution")
	}
	for _, bad := range []semanticDynamicProposal{
		{Mode: "rows", Projection: []string{"password"}},
		{Mode: "rows", Filters: []semanticDynamicFilter{{Field: "source_file", Op: "eq", Value: "foreign"}}},
		{Mode: "group", Group: &CanonicalGroupV1{Field: "primary_target", Measure: "count"}},
		{Mode: "group", Group: &CanonicalGroupV1{Field: "timestamp", Measure: "count", Bucket: "day", Timezone: "UTC"}},
		{Mode: "rows", Filters: []semanticDynamicFilter{{Field: "record_type", Op: "eq", Value: "cdr"}}},
	} {
		if _, err := applySemanticDynamicPlan(req, "generic_tabular", bad); err == nil {
			t.Errorf("unsafe plan accepted: %+v", bad)
		}
	}
	if _, err := applySemanticDynamicPlan(req, "image_intelligence", p); err == nil {
		t.Fatal("media truth boundary bypass")
	}
	// No named operation fits: the issued typed-plan decision enters the dynamic
	// planner once. Unsupported is terminal and cannot hide another model call.
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		d := `{"decision":"DYNAMIC_TYPED_PLAN"}`
		if calls == 2 {
			b, _ := json.Marshal(p)
			d = string(b)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]string{"content": d}}}})
	}))
	defer s.Close()
	req.SynthesisModel = "qwen_qwen3-4b-instruct-2507"
	got, state := resolveSemanticOperationInCandidates(context.Background(), config{LocalAIURL: s.URL}, req, semanticOperationCandidates(req))
	if state != "semantic_dynamic_plan" || got.Group == nil || calls != 2 {
		t.Fatalf("fallback: %s calls=%d", state, calls)
	}
	unsupportedCalls := 0
	unsupported := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		unsupportedCalls++
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]string{"content": `{"decision":"UNSUPPORTED:REQUEST_CLASS"}`}}}})
	}))
	defer unsupported.Close()
	_, state = resolveSemanticOperationInCandidates(context.Background(), config{LocalAIURL: unsupported.URL}, req, semanticOperationCandidates(req))
	if state != "UNSUPPORTED_REQUEST_CLASS" || unsupportedCalls != 1 {
		t.Fatalf("unsupported hidden retry: %s calls=%d", state, unsupportedCalls)
	}
}

func TestConvergenceAttributionParaphrase(t *testing.T) {
	for _, tc := range []struct {
		source, claim string
		want          bool
	}{
		{"The delivery note states that the west loading dock was closed.", "The delivery note indicates the west loading dock was closed.", true},
		{"The log states that camera CP-217 recorded 12 observations.", "The log indicates camera CP-217 recorded 12 observations.", true},
		{"The log indicates camera CP-217 recorded 12 observations.", "The log states camera CP-217 recorded 12 observations.", false},
		{"The log states camera CP-217 recorded 12 observations.", "The log indicates camera CP-217 recorded 21 observations.", false},
		{"The log states camera CP-217 recorded no observations.", "The log indicates camera CP-217 recorded observations.", false},
		{"The note states Alpha contacted Beta.", "The note indicates Beta contacted Alpha.", false},
	} {
		if got := groundedSingleFactWording(tc.claim, tc.source); got != tc.want {
			t.Errorf("%q got %v", tc.claim, got)
		}
	}
}
