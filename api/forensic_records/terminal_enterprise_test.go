package main

import (
	"strings"
	"testing"
	"time"

	"github.com/mudler/LocalAI/pkg/forensicrequest"
)

// SILENCE IS NOT ABSTENTION.
//
// Measured live, Step 4b, 2026-09-26: H2 "What does the audio transcript say?"
// and M15 "What is the latest end offset in the audio transcripts?" returned
// HTTP 200 in ~20ms with NO analyst-facing text, in every arm and with media
// routing OFF. They scored WRONG for stating nothing.
//
// The server had produced an answer and dropped it: `terminalRequestResponse`
// filled `resp.Answer` and never built `resp.Enterprise`, which is where the
// analyst surface and the harness both read their text from. All FOUR terminal
// classes were affected, so greetings, product help, clarification and
// out-of-scope requests were equally silent -- the §9 A classes the product
// owner is waiting on.
//
// An analyst shown an empty successful response cannot tell a refusal from a
// failure from an empty result set.

func terminalRequestFor(class forensicrequest.Class, query string) hybridQueryRequest {
	return hybridQueryRequest{
		Query: query, TenantID: "default", CollectionID: "nexusai-multimodal-product-acceptance",
		RequestClass: class,
	}
}

// EVERY TERMINAL CLASS SAYS SOMETHING. Table-driven over all four, because the
// defect was in the shared path and fixing only the two that were measured
// would leave the other two silent.
func TestEveryTerminalClassProducesAnalystText(t *testing.T) {
	cases := []struct {
		class forensicrequest.Class
		query string
	}{
		{forensicrequest.GeneralDomainKnowledge, "What does the audio transcript say?"},
		{forensicrequest.GeneralDomainKnowledge, "What is an IMSI?"},
		{forensicrequest.ProductHelp, "What can you do?"},
		{forensicrequest.Clarify, "that one"},
		{forensicrequest.Unsupported, "delete all the evidence"},
	}
	for _, tc := range cases {
		resp, ok := terminalRequestResponse(terminalRequestFor(tc.class, tc.query), time.Now())
		if !ok {
			t.Errorf("%s: no terminal response was produced", tc.class)
			continue
		}
		if len(resp.Enterprise) == 0 {
			t.Errorf("%s: NO enterprise payload — the analyst is shown an empty 200", tc.class)
			continue
		}
		// Exactly what the harness and the analyst surface read.
		headline := strings.TrimSpace(stringValueAny(resp.Enterprise["executive_answer"]))
		if headline == "" {
			t.Errorf("%s: empty executive_answer — silence is not abstention", tc.class)
		}
		if summary := strings.TrimSpace(stringValueAny(resp.Enterprise["summary"])); summary == "" {
			t.Errorf("%s: empty summary", tc.class)
		}
		limitations, _ := resp.Enterprise["limitations"].([]string)
		if len(limitations) == 0 || strings.TrimSpace(limitations[0]) == "" {
			t.Errorf("%s: states no limitation, so the analyst is not told what was NOT done", tc.class)
		}
		t.Logf("%-26s %s", tc.class, headline[:min(len(headline), 78)])
	}
}

// A TERMINAL ANSWER ASSERTS NO EVIDENCE. These classes make no claim about the
// case, so the payload must not carry rows or citations. A non-evidence answer
// dressed as a governed analytical result is the product's central conflation
// pointed at a different surface.
func TestTerminalPayloadAssertsNoEvidence(t *testing.T) {
	for _, class := range []forensicrequest.Class{
		forensicrequest.GeneralDomainKnowledge, forensicrequest.ProductHelp,
		forensicrequest.Clarify, forensicrequest.Unsupported,
	} {
		resp, ok := terminalRequestResponse(terminalRequestFor(class, "anything"), time.Now())
		if !ok {
			continue
		}
		provenance, _ := resp.Enterprise["provenance"].([]map[string]any)
		if len(provenance) != 0 {
			t.Errorf("%s: cites %d items for an answer that searched no evidence", class, len(provenance))
		}
		grid, _ := resp.Enterprise["data_grid"].(map[string]any)
		if rows := mapsFromAny(grid["rows"]); len(rows) != 0 {
			t.Errorf("%s: carries %d result rows", class, len(rows))
		}
		if count := numericFloat(resp.Enterprise["row_count"]); count != 0 {
			t.Errorf("%s: row_count = %v", class, count)
		}
		// It must NOT carry a fact packet or proof state: those assert an
		// evidence shape, and finalizeEnterprisePayload is deliberately not used.
		for _, key := range []string{"fact_packet", "proof_state"} {
			if _, present := resp.Enterprise[key]; present {
				t.Errorf("%s: carries %q, which asserts an evidence shape it does not have", class, key)
			}
		}
		if access := stringValueAny(mapFromAny(resp.Enterprise["operation"])["source_access"]); access != "none" {
			t.Errorf("%s: declares source_access %q, want none", class, access)
		}
	}
}

// THE CLASS'S OWN CONTENT SURVIVES. Product help computes family and operation
// registries; flattening them into prose would lose them for any caller that
// wants the structure.
func TestProductHelpKeepsItsRegistries(t *testing.T) {
	resp, ok := terminalRequestResponse(terminalRequestFor(forensicrequest.ProductHelp, "what can you do?"), time.Now())
	if !ok {
		t.Fatal("no terminal response for product help")
	}
	carried := mapFromAny(resp.Enterprise["terminal_answer"])
	if len(carried) == 0 {
		t.Fatal("the class's own answer was not carried through")
	}
	for _, key := range []string{"registered_families", "exposed_operations", "grounding_contract"} {
		if _, present := carried[key]; !present {
			t.Errorf("product help lost %q", key)
		}
	}
}

// THE GOVERNED PATH IS UNTOUCHED, asserted rather than assumed. This change has
// no switch, so its bound is that `terminalRequestResponse` refuses to handle an
// evidence question at all — every governed answer still goes through
// buildEnterprisePayload exactly as before.
func TestTerminalPathNeverHandlesAGovernedQuestion(t *testing.T) {
	for _, class := range []forensicrequest.Class{
		forensicrequest.GovernedAnalysis, forensicrequest.ContextualFollowUp,
	} {
		if _, ok := terminalRequestResponse(terminalRequestFor(class, "How many CDR records are there?"), time.Now()); ok {
			t.Errorf("%s was answered by the TERMINAL path, bypassing the compiler and the evidence contract", class)
		}
	}
}

// THE HARNESS CAN NOW READ IT. Reproduces `answer_text()` from
// scripts/nexusai_live_eval.py exactly -- executive_answer, then
// narrative.direct_answer, then summary. A probe must reproduce the PATH: the
// defect was invisible precisely because the answer existed somewhere the
// reader never looked.
func TestTerminalTextIsReachableTheWayTheHarnessReadsIt(t *testing.T) {
	resp, ok := terminalRequestResponse(
		terminalRequestFor(forensicrequest.GeneralDomainKnowledge, "What does the audio transcript say?"), time.Now())
	if !ok {
		t.Fatal("no terminal response")
	}
	parts := []string{
		stringValueAny(resp.Enterprise["executive_answer"]),
		stringValueAny(mapFromAny(resp.Enterprise["narrative"])["direct_answer"]),
		stringValueAny(resp.Enterprise["summary"]),
	}
	text := strings.TrimSpace(strings.Join(parts, "\n"))
	if text == "" {
		t.Fatal("the harness would still read an empty analyst answer — this is the defect unfixed")
	}
	// It must say it is NOT about the case, or the analyst may read a general
	// definition as a finding about their evidence.
	lowered := strings.ToLower(text)
	if !strings.Contains(lowered, "case") && !strings.Contains(lowered, "evidence") {
		t.Errorf("the answer does not say it makes no claim about the case: %q", text)
	}
	t.Logf("analyst now reads: %s", text)
}

// `clarification` IS SET ONLY WHERE THAT IS WHAT HAPPENED. It is the contract's
// field for "I cannot answer this as asked", and the grader keys on it. A
// general definition and a product-help reply are ANSWERS; labelling them
// clarifications would misreport what the server did, which is the inverse of
// the silence this change exists to fix.
func TestTerminalClarificationIsSetOnlyWhereItApplies(t *testing.T) {
	wants := map[forensicrequest.Class]bool{
		forensicrequest.Clarify:                true,
		forensicrequest.Unsupported:            true,
		forensicrequest.GeneralDomainKnowledge: false,
		forensicrequest.ProductHelp:            false,
	}
	for class, want := range wants {
		resp, ok := terminalRequestResponse(terminalRequestFor(class, "anything"), time.Now())
		if !ok {
			t.Errorf("%s: no terminal response", class)
			continue
		}
		got := strings.TrimSpace(stringValueAny(resp.Enterprise["clarification"])) != ""
		if got != want {
			t.Errorf("%s: clarification present = %v, want %v", class, got, want)
		}
	}
}
