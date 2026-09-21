package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("R6.6 complete operation acceptance matrix", func() {
	It("publishes complete execution and presentation metadata for all 79 operations", func() {
		entries := supportedQueryTemplates()
		Expect(entries).To(HaveLen(79))
		seenNames := map[string]bool{}
		seenOperations := map[string]bool{}
		validScopes := map[string]bool{"case_wide": true, "case_or_target": true, "target_required": true, "evidence_required": true, "source_set_and_target_required": true}
		validPresentations := map[string]bool{"bounded_records_table": true, "cited_evidence_results": true, "executive_brief_with_sources": true, "multi_source_relationship_comparison": true, "source_time_observation_timeline": true, "candidate_observation_table": true}

		for _, entry := range entries {
			Expect(entry.Name).ToNot(BeEmpty())
			Expect(seenNames).ToNot(HaveKey(entry.Name), entry.Name)
			seenNames[entry.Name] = true
			Expect(entry.OperationID).ToNot(BeEmpty(), entry.Name)
			Expect(seenOperations).ToNot(HaveKey(entry.OperationID), entry.OperationID)
			seenOperations[entry.OperationID] = true
			Expect(entry.FamilyID).ToNot(BeEmpty(), entry.Name)
			Expect(entry.Route).To(BeElementOf("records", "kb", "derived", "hybrid"), entry.Name)
			Expect(entry.RecordTypes).ToNot(BeEmpty(), entry.Name)
			Expect(entry.ExampleQuery).ToNot(BeEmpty(), entry.Name)
			Expect(entry.Calculation).ToNot(BeEmpty(), entry.Name)
			Expect(entry.OutputDescription).ToNot(BeEmpty(), entry.Name)
			Expect(entry.Limitations).ToNot(BeEmpty(), entry.Name)
			Expect(validScopes[entry.ScopeMode]).To(BeTrue(), entry.Name)
			Expect(validPresentations[entry.Presentation]).To(BeTrue(), entry.Name)
			Expect(chooseTemplate(entry.ExampleQuery, "")).To(Equal(entry.Name), entry.ExampleQuery)

			inputNames := map[string]bool{}
			for _, input := range entry.Inputs {
				Expect(input.Name).ToNot(BeEmpty(), entry.Name)
				Expect(input.Label).ToNot(BeEmpty(), entry.Name+":"+input.Name)
				Expect(input.Description).ToNot(BeEmpty(), entry.Name+":"+input.Name)
				Expect(inputNames).ToNot(HaveKey(input.Name), entry.Name+":"+input.Name)
				inputNames[input.Name] = true
			}
			if (entry.Route == "records" || entry.Route == "hybrid") && entry.ScopeMode != "evidence_required" {
				Expect(inputNames).To(HaveKey("date_from"), entry.Name)
				Expect(inputNames).To(HaveKey("date_to"), entry.Name)
				Expect(inputNames).To(HaveKey("limit"), entry.Name)
			}
			if entry.ScopeMode == "target_required" || entry.ScopeMode == "source_set_and_target_required" {
				Expect(entry.Inputs).To(ContainElement(SatisfyAll(
					WithTransform(func(input queryTemplateInput) string { return input.Name }, Equal("target")),
					WithTransform(func(input queryTemplateInput) bool { return input.Required }, BeTrue()),
					WithTransform(func(input queryTemplateInput) []string { return input.AcceptedKinds }, Not(BeEmpty())),
				)), entry.Name)
				Expect(needsClarification("", entry.Name, "")).To(BeTrue(), entry.Name)
			}
			if entry.ScopeMode == "evidence_required" {
				Expect(entry.Inputs).To(ContainElement(SatisfyAll(
					WithTransform(func(input queryTemplateInput) string { return input.Name }, Equal("evidence_id")),
					WithTransform(func(input queryTemplateInput) bool { return input.Required }, BeTrue()),
				)), entry.Name)
				Expect(inputNames).To(HaveKey("limit"), entry.Name)
				Expect(inputNames).ToNot(HaveKey("date_from"), entry.Name)
				Expect(inputNames).ToNot(HaveKey("date_to"), entry.Name)
			}
		}
	})

	It("rejects model-authored identities, locations, and citations", func() {
		Expect(validateForensicSynthesisSummary("frequent_contacts", "Model Interpretation: The subject belongs to Alpha. Limitations: Review exact records.")).To(ContainSubstring("identity or location"))
		Expect(validateForensicSynthesisSummary("anpr_sightings", "Model Interpretation: The vehicle is located at Sector Blue. Limitations: Review observations.")).To(ContainSubstring("identity or location"))
		Expect(validateForensicSynthesisSummary("data_quality", "Model Interpretation: Citation confirms the result. Limitations: Review sources.")).To(ContainSubstring("citation authority"))
	})
})
