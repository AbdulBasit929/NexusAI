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
	return hybridQueryResponse{
		CollectionID: req.CollectionID, RequestClass: class, Intent: intent,
		Route: []string{"terminal"}, Policy: "server_authoritative_terminal_contract",
		Planner:   map[string]any{"contract_version": forensicrequest.ContractV1, "request_class": class},
		QueryPlan: QueryPlan{}, Capability: queryCapabilityAssessment{Status: "terminal"},
		Answer: answer, GeneratedAt: time.Now().UTC(),
		Telemetry: QueryTelemetry{TotalLatencyMS: time.Since(startedAt).Milliseconds()},
	}, true
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
