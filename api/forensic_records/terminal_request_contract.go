package main

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mudler/LocalAI/pkg/forensicrequest"
)

const productHelpGroundingContractV1 = "forensics.product-help-grounding/v1"

func terminalRequestResponse(req hybridQueryRequest, startedAt time.Time) (hybridQueryResponse, bool) {
	class := req.RequestClass
	if class == forensicrequest.GovernedAnalysis || class == forensicrequest.ContextualFollowUp {
		return hybridQueryResponse{}, false
	}
	answer := map[string]any{"result_state": "COMPLETED"}
	intent := intentSemantic
	switch class {
	case forensicrequest.GeneralDomainKnowledge:
		answer["answer"] = generalDomainAnswer(req.Query)
		answer["grounding"] = "GENERAL_DOMAIN_DEFINITION"
		answer["case_specific_facts"] = []any{}
	case forensicrequest.ProductHelp:
		answer = groundedProductHelp(req.Query)
	case forensicrequest.Clarify:
		intent = intentClarify
		answer["result_state"] = "CLARIFICATION_REQUIRED"
		answer["clarification"] = "Please restate the request with the evidence, entity, result, or operation you want to use. A follow-up can continue only from unexpired context issued for this workspace."
	case forensicrequest.Unsupported:
		answer["result_state"] = "UNSUPPORTED"
		answer["answer"] = "That request is outside the governed analyst query contract. Use an authorized evidence question or a supported product-help request."
	}
	resp := hybridQueryResponse{
		CollectionID: req.CollectionID, RequestClass: class, Intent: intent,
		Route: []string{"terminal"}, Policy: "server_authoritative_terminal_contract",
		Planner:   map[string]any{"contract_version": forensicrequest.ContractV1, "request_class": class},
		QueryPlan: QueryPlan{}, Capability: queryCapabilityAssessment{Status: "terminal"},
		Answer: answer, GeneratedAt: time.Now().UTC(),
		Telemetry: QueryTelemetry{TotalLatencyMS: time.Since(startedAt).Milliseconds()},
	}
	resp.Enterprise = terminalEnterprisePayload(req, class, answer)
	return resp, true
}

// SILENCE IS NOT ABSTENTION.
//
// Every terminal class returned HTTP 200 with an `answer` map and NO enterprise
// payload. The analyst-facing surface — and the evaluation harness, which reads
// the same contract — takes its text from `enterprise.executive_answer`,
// `enterprise.narrative.direct_answer` and `enterprise.summary`, so all four
// classes rendered as a successful response containing nothing at all.
//
// MEASURED, Step 4b, 2026-09-26. H2 "What does the audio transcript say?" and
// M15 "What is the latest end offset in the audio transcripts?" both returned
// 200 in ~20ms with no analyst text, in EVERY arm and with media routing off.
// They scored WRONG for stating nothing. The server had in fact produced an
// answer — "A bounded general definition is unavailable for that term" — and
// then dropped it on the floor.
//
// An analyst who asks a question and is shown an empty successful response
// cannot tell refusal from failure from an empty result set. That is worse than
// a refusal: a refusal at least says which. **Abstention has to be SAID.**
//
// This payload deliberately does NOT go through finalizeEnterprisePayload. That
// builds a fact packet, a narrative and a proof state, all of which assert an
// EVIDENCE shape — and these four classes make no claim about the case at all.
// Manufacturing an empty fact packet here would dress a non-evidence answer as a
// governed analytical result, which is the conflation this product exists to
// prevent, pointed at a different surface. Empty provenance, empty grid, and a
// limitation that says so in words.
func terminalEnterprisePayload(req hybridQueryRequest, class forensicrequest.Class, answer map[string]any) map[string]any {
	text := strings.TrimSpace(stringValueAny(answer["answer"]))
	if text == "" {
		text = strings.TrimSpace(stringValueAny(answer["clarification"]))
	}
	status, limitation := "completed", ""
	switch class {
	case forensicrequest.GeneralDomainKnowledge:
		limitation = "This is a general definition. It states nothing about this case or its evidence, and no evidence was searched."
	case forensicrequest.ProductHelp:
		limitation = "This describes the product's registered capabilities. No evidence was searched and no case fact is asserted."
	case forensicrequest.Clarify:
		status = "needs_input"
		limitation = "More information is required before any analysis can run. No query was executed and no result was inferred."
	case forensicrequest.Unsupported:
		status = "unsupported"
		limitation = "This request is outside the governed analyst query contract. No evidence was searched."
	}
	if text == "" {
		// Never return an empty headline. A class with no text is a defect, and
		// saying so is better than saying nothing.
		text = limitation
	}
	payload := map[string]any{
		"status":           status,
		"result_state":     stringValueAny(answer["result_state"]),
		"executive_answer": text,
		"summary":          limitation,
		"limitations":      []string{limitation},
		"data_grid":        enterpriseDataGrid(nil),
		"provenance":       []map[string]any{},
		"row_count":        0,
		"metrics": []map[string]any{
			{"label": "Request Class", "value": string(class), "source": "planner"},
			{"label": "Route", "value": "terminal", "source": "planner"},
		},
		"operation": map[string]any{
			"template": "terminal", "operation_id": "forensics.terminal_request",
			"family_id": "none", "source_access": "none",
		},
		"coverage": map[string]any{
			"tenant_id": req.TenantID, "collection_id": req.CollectionID,
			"route": []string{"terminal"}, "queried_record_sql": false, "queried_kb": false,
		},
		// Everything the class already computed stays reachable rather than being
		// flattened into the headline: product help carries its family and
		// operation registries, and a caller that wants them should not have to
		// parse prose.
		"terminal_answer": answer,
	}
	// `clarification` is the contract's field for "I cannot answer this as
	// asked", and both the analyst surface and the grader key on it. It is set
	// ONLY where that is what happened: CLARIFY needs more from the analyst and
	// UNSUPPORTED refuses. A general definition and a product-help answer are
	// ANSWERS, and labelling them clarifications would misreport what the server
	// did — the inverse of the silence this whole change exists to fix.
	switch class {
	case forensicrequest.Clarify, forensicrequest.Unsupported:
		payload["clarification"] = text
	}
	return payload
}

func generalDomainAnswer(query string) string {
	normalized := strings.ToLower(query)
	definitions := []struct {
		terms []string
		text  string
	}{
		{[]string{"imsi"}, "IMSI is the International Mobile Subscriber Identity, an identifier assigned to a mobile subscription and stored on the SIM."},
		{[]string{"imei"}, "IMEI is the International Mobile Equipment Identity, an identifier assigned to mobile equipment."},
		{[]string{"iccid"}, "ICCID is the Integrated Circuit Card Identifier, the serial identifier of a SIM card."},
		{[]string{"msisdn"}, "MSISDN is the telephone number associated with a mobile subscription."},
		{[]string{"ipdr"}, "IPDR means Internet Protocol Detail Record, a structured record describing an IP service or network session."},
		{[]string{"ocr"}, "OCR is optical character recognition, the process of turning visible text in images or documents into machine-readable text."},
		{[]string{"rag", "retrieval augmented"}, "RAG is retrieval-augmented generation: relevant source material is retrieved and supplied as context for an answer."},
		{[]string{"similarity score"}, "A similarity score measures how close two representations are under a specified method. It is not, by itself, an identity probability or confidence statement."},
	}
	for _, definition := range definitions {
		for _, term := range definition.terms {
			if strings.Contains(normalized, term) {
				return definition.text + " This definition does not assert anything about the current case."
			}
		}
	}
	return "A bounded general definition is unavailable for that term. This response makes no statement about the current case or its evidence."
}

func groundedProductHelp(query string) map[string]any {
	formats := []string{}
	families := []map[string]any{}
	for _, capability := range forensicCapabilityDefinitions() {
		if capability.SupportLevel == "operational" {
			formats = append(formats, capability.Formats...)
		}
		families = append(families, map[string]any{"family_id": capability.ID, "label": capability.Label, "support_level": capability.SupportLevel, "formats": capability.Formats})
	}
	operations := []map[string]any{}
	for _, template := range supportedQueryTemplates() {
		if template.ExposureStatus == "engineering_only" {
			continue
		}
		operations = append(operations, map[string]any{
			"operation_id": template.OperationID, "label": template.Description,
			"certification_status": template.CertificationStatus, "exposure_status": template.ExposureStatus,
		})
	}
	formats = capabilityUniqueSortedStrings(formats)
	sort.Slice(operations, func(i, j int) bool {
		return fmt.Sprint(operations[i]["operation_id"]) < fmt.Sprint(operations[j]["operation_id"])
	})
	state := "AVAILABLE"
	if len(formats) == 0 && len(operations) == 0 {
		state = "UNAVAILABLE"
	}
	return map[string]any{
		"result_state": "COMPLETED", "grounding_contract": productHelpGroundingContractV1,
		"registry_state": state, "question": query,
		"answer":              "Current product help is derived from the registered evidence families, their explicit support levels, and exposed query operations shown in this response.",
		"operational_formats": formats, "registered_families": families, "exposed_operations": operations,
	}
}
