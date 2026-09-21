package main

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("R7.4 tower history and R7.5 governed telecom operations", func() {
	It("publishes six governed tower workflows", func() {
		byName := map[string]queryTemplateCatalogEntry{}
		for _, entry := range supportedQueryTemplates() {
			byName[entry.Name] = entry
		}
		expected := map[string]string{
			"tower_site_lookup":        "tower.site_lookup",
			"tower_reference_timeline": "tower.reference_timeline",
			"tower_coordinate_audit":   "tower.coordinate_audit",
			"tower_status_summary":     "tower.status_summary",
			"tower_alias_conflicts":    "tower.alias_conflicts",
			"tower_cdr_join":           "tower.cdr_join",
		}
		for template, operation := range expected {
			entry, ok := byName[template]
			Expect(ok).To(BeTrue(), template)
			Expect(entry.OperationID).To(Equal(operation))
			Expect(entry.FamilyID).To(Equal("tower_location"))
			Expect(entry.Limitations).NotTo(BeEmpty())
		}
		Expect(byName["tower_site_lookup"].Inputs[0].Required).To(BeTrue())
		Expect(byName["tower_cdr_join"].Inputs[0].Required).To(BeTrue())
	})

	DescribeTable("routes accepted tower language",
		func(query, template string) { Expect(chooseTemplate(query, "")).To(Equal(template)) },
		Entry("lookup", "look up tower site PK-LHR-SYN-001", "tower_site_lookup"),
		Entry("history", "show tower reference history for PK-LHR-SYN-001", "tower_reference_timeline"),
		Entry("coordinates", "audit tower coordinates and uncertainty", "tower_coordinate_audit"),
		Entry("status", "show tower status summary", "tower_status_summary"),
		Entry("conflicts", "show tower alias conflicts", "tower_alias_conflicts"),
		Entry("join", "run a time-aware tower join for PK-LHR-SYN-001", "tower_cdr_join"),
	)

	It("requires exact targets for lookup, timeline, and CDR joins", func() {
		for _, template := range []string{"tower_site_lookup", "tower_reference_timeline", "tower_cdr_join"} {
			Expect(needsClarification("show tower reference", template, "")).To(BeTrue(), template)
		}
		Expect(clarificationQuestion("", "tower_site_lookup")).To(ContainSubstring("site"))
		Expect(clarificationQuestion("", "tower_cdr_join")).To(ContainSubstring("time-aware"))
	})

	It("keeps tower matching exact, time-aware, parameterized, and bounded", func() {
		scope := strings.Join(strings.Fields(towerReferenceScopeSQL), " ")
		join := strings.Join(strings.Fields(towerExactTargetSQL), " ")
		Expect(scope).To(ContainSubstring("record_type = 'tower_location'"))
		Expect(scope).To(ContainSubstring("reference_valid_from_at"))
		Expect(scope).To(ContainSubstring("uncertainty_radius_m"))
		Expect(scope).To(ContainSubstring("history_key"))
		Expect(scope).To(ContainSubstring("reference_version"))
		Expect(scope).To(ContainSubstring("coordinate_datum_basis"))
		Expect(scope).To(ContainSubstring("validity_status"))
		Expect(join).To(ContainSubstring("lower($3)"))
		Expect(join).To(ContainSubstring("provider_code"))
		Expect(join).To(ContainSubstring("reference_identifier"))
		Expect(towerPaginationSQL).To(ContainSubstring("LIMIT $6 OFFSET $7"))
	})

	It("detects half-open provider/site/sector history overlaps for review", func() {
		source := strings.Join(strings.Fields(towerHistoryConflictSQL), " ")
		Expect(source).To(ContainSubstring("a.history_key = b.history_key"))
		Expect(source).To(ContainSubstring("b.valid_from_at < a.valid_to_at"))
		Expect(source).To(ContainSubstring("a.valid_from_at < b.valid_to_at"))
		Expect(source).To(ContainSubstring("overlapping_pair_count"))
		Expect(source).To(ContainSubstring("distinct_datum_count"))
		Expect(source).To(ContainSubstring("distinct_uncertainty_count"))
	})

	DescribeTable("routes realistic R7.5 telecom language",
		func(query, template string) { Expect(chooseTemplate(query, "")).To(Equal(template)) },
		Entry("provider history", "show provider tower history for PK-LHR-HIST-001", "tower_reference_timeline"),
		Entry("sector history", "show sector reference history for PK-LHR-HIST-001", "tower_reference_timeline"),
		Entry("overlap review", "show overlapping tower references for PK-LHR-HIST-001", "tower_alias_conflicts"),
		Entry("validity conflict", "audit tower validity conflicts", "tower_alias_conflicts"),
		Entry("ICCID links", "show ICCID device links for 8992410000000000001", "subscriber_device_links"),
		Entry("service links", "show subscriber service links for PK-SVC-SYN-001", "subscriber_device_links"),
		Entry("service classification", "classify packet data and USSD usage for 923001234567", "service_usage"),
		Entry("device history", "show SIM identity changes for 923001234567", "device_identity_changes"),
	)

	It("keeps target-required telecom requests safe on missing input", func() {
		for _, template := range []string{"frequent_contacts", "device_identity_changes", "subscriber_identity_lookup", "subscriber_validity_timeline", "subscriber_device_links", "tower_site_lookup", "tower_reference_timeline", "tower_cdr_join"} {
			Expect(needsClarification("run telecom analysis", template, "")).To(BeTrue(), template)
		}
		Expect(clarificationQuestion("", "subscriber_device_links")).To(ContainSubstring("ICCID"))
		Expect(clarificationQuestion("", "tower_reference_timeline")).To(ContainSubstring("site"))
	})

	It("returns an analyst-safe no-result contract without inferred telecom facts", func() {
		resp := hybridQueryResponse{
			Intent:   intentRecords,
			Template: "tower_reference_timeline",
			Answer: map[string]any{
				"records_status":    "no_matching_records",
				"records_row_count": 0,
			},
			Records: map[string]any{"tower_reference_timeline": []map[string]any{}},
		}
		Expect(enterpriseOutcomeStatus(resp, nil)).To(Equal("no_results"))
		Expect(enterpriseExecutiveAnswer(hybridQueryRequest{}, resp, nil)).To(ContainSubstring("No matching records"))
		Expect(forensicSynthesisSystemPrompt("tower_reference_timeline")).To(ContainSubstring("Never infer RF coverage"))
	})

	It("classifies matched, ambiguous, and unmatched time-valid joins", func() {
		joinSource := strings.Join(strings.Fields(towerCDRJoinSQL), " ")
		Expect(joinSource).To(ContainSubstring("reference_candidate_count"))
		Expect(joinSource).To(ContainSubstring("eligible_reference_count"))
		Expect(joinSource).To(ContainSubstring("ambiguous_overlapping_references"))
		Expect(joinSource).To(ContainSubstring("unmatched_no_reference"))
		Expect(joinSource).To(ContainSubstring("unmatched_outside_validity_window"))

		summary := summarizeTowerCDRJoinRows([]map[string]any{
			{"match_status": "matched"},
			{"match_status": "ambiguous_overlapping_references"},
			{"match_status": "unmatched_no_reference"},
			{"match_status": "unmatched_outside_validity_window"},
		})
		Expect(summary).To(Equal(map[string]any{
			"observations":                      4,
			"matched_observations":              1,
			"ambiguous_observations":            1,
			"unmatched_without_reference":       1,
			"unmatched_outside_validity_window": 1,
		}))
		Expect(isResultMetadataKey("join_summary")).To(BeTrue())

		metrics := enterpriseMetrics(hybridQueryRequest{}, hybridQueryResponse{
			Template: "tower_cdr_join",
			Records:  map[string]any{"join_summary": summary},
		}, nil, nil)
		labels := map[string]any{}
		for _, metric := range metrics {
			labels[stringValueAny(metric["label"])] = metric["value"]
		}
		Expect(labels).To(HaveKeyWithValue("Matched Observations", 1))
		Expect(labels).To(HaveKeyWithValue("Ambiguous Observations", 1))
		Expect(labels).To(HaveKeyWithValue("Unmatched Without Reference", 1))
		Expect(labels).To(HaveKeyWithValue("Unmatched Outside Validity Window", 1))
	})

	It("gives model synthesis the RF and location inference boundary", func() {
		prompt := forensicSynthesisSystemPrompt("tower_cdr_join")
		Expect(prompt).To(ContainSubstring("Never infer RF coverage"))
		Expect(prompt).To(ContainSubstring("handset position"))
		Expect(prompt).To(ContainSubstring("exact identifier-based location context only"))
	})
})
