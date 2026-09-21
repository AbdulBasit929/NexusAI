package main

import (
	"context"
	"encoding/json"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Bounded canonical grouping", func() {
	req := func() hybridQueryRequest {
		return hybridQueryRequest{TenantID: "t-group", CollectionID: "c-group", Group: &CanonicalGroupV1{Field: "source_file", Measure: "count"}}
	}
	row := func(id string, value any) map[string]any {
		return map[string]any{"record_id": id, "tenant_id": "t-group", "collection_id": "c-group", "record_type": "cdr", "source_file": value, "file_id": "f-" + id, "batch_id": "b", "row_number": 7, "row_hash": "h-" + id, "source_table": "forensic.records", "raw_payload": map[string]any{"private": "hidden"}}
	}
	It("matches an independently stated count, null, tie and lineage oracle", func() {
		rows := []map[string]any{row("r4", "B.csv"), row("r2", nil), row("r3", "A.csv"), row("r1", "A.csv"), row("r5", "B.csv"), row("r6", nil), row("r7", "")}
		groups, err := groupCanonicalRows(req(), rows)
		Expect(err).NotTo(HaveOccurred())
		Expect(groups).To(HaveLen(4))
		values := []any{}
		counts := []int{}
		ids := [][]string{}
		for _, g := range groups {
			values = append(values, g["value"])
			counts = append(counts, g["count"].(int))
			groupIDs := []string{}
			for _, source := range g["metadata"].(map[string]any)["source_rows"].([]map[string]any) {
				groupIDs = append(groupIDs, source["record_id"].(string))
				Expect(source).NotTo(HaveKey("raw_payload"))
				Expect(source["row_hash"]).To(Equal("h-" + source["record_id"].(string)))
			}
			ids = append(ids, groupIDs)
		}
		Expect(values).To(Equal([]any{nil, "A.csv", "B.csv", ""}))
		Expect(counts).To(Equal([]int{2, 2, 2, 1}))
		Expect(ids).To(Equal([][]string{{"r2", "r6"}, {"r1", "r3"}, {"r4", "r5"}, {"r7"}}))
		Expect(flattenEnterpriseRows(map[string]any{"canonical_groups": groups}, 100)).To(HaveLen(4))
		empty, err := groupCanonicalRows(req(), nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(empty).To(BeEmpty())
	})
	It("fails closed for overflow, foreign scope, duplicate identity and invalid types", func() {
		for _, kind := range []string{"rows", "groups", "tenant", "collection", "duplicate", "type", "missing"} {
			rows := []map[string]any{row("r1", "a")}
			switch kind {
			case "rows":
				for i := 1; i <= canonicalGroupRowLimit; i++ {
					rows = append(rows, row(fmt.Sprint(i), "a"))
				}
			case "groups":
				for i := 1; i <= canonicalGroupLimit; i++ {
					rows = append(rows, row(fmt.Sprint(i), fmt.Sprint(i)))
				}
			case "tenant":
				rows[0]["tenant_id"] = "foreign"
			case "collection":
				rows[0]["collection_id"] = "foreign"
			case "duplicate":
				rows = append(rows, row("r1", "b"))
			case "type":
				rows[0]["source_file"] = 42
			case "missing":
				delete(rows[0], "source_file")
			}
			got, err := groupCanonicalRows(req(), rows)
			Expect(err).To(HaveOccurred(), kind)
			Expect(got).To(BeNil(), kind)
		}
	})
	It("lowers only the typed allowlisted group and preserves authorized SQL predicates", func() {
		legacy, early, err := normalizeV1QueryPlan("case-group", "request-group", map[string]any{"contract_version": "forensics.query-plan/v1", "intent": "forensics.canonical_records", "group": map[string]any{"field": "record_type", "measure": "count"}})
		Expect(err).NotTo(HaveOccurred())
		Expect(early).To(BeNil())
		Expect(legacy["group"]).To(Equal(map[string]any{"field": "record_type", "measure": "count"}))
		request := req()
		request.SourceFile = "supplied.csv"
		request.BatchID = "batch-input"
		built, err := buildCanonicalRecordsQuery(request)
		Expect(err).NotTo(HaveOccurred())
		Expect(built.WhereSQL).To(ContainSubstring("tenant_id = $1"))
		Expect(built.WhereSQL).To(ContainSubstring("collection_id = $2"))
		Expect(built.Args).To(ContainElements("t-group", "c-group", "supplied.csv", "batch-input"))
		Expect(built.WhereSQL).NotTo(ContainSubstring("supplied.csv"))
	})
	It("rejects expressions, silent extra controls and mismatched executors", func() {
		for _, raw := range []string{`{"field":"count(*)","measure":"count"}`, `{"field":"primary_target","measure":"count"}`, `{"field":"record_type","measure":"sum"}`, `{"field":"record_type","measure":"count","sql":"DROP TABLE x"}`} {
			var group CanonicalGroupV1
			Expect(json.Unmarshal([]byte(raw), &group)).NotTo(Succeed())
		}
		request := req()
		request.Projection = []string{"timestamp"}
		_, err := buildCanonicalRecordsQuery(request)
		Expect(err).To(HaveOccurred())
		_, err = runAnalyticalTemplate(context.Background(), nil, req(), "frequent_contacts")
		Expect(err).To(MatchError(ContainSubstring("not consumed")))
	})
	It("passes the governed plan and preserves grouping through execution and presentation", func() {
		request := req()
		request.UserID = "analyst-group"
		request.Query = "Count authorized source rows by source file."
		request.Template = "canonical_records"
		request.MaxKBResults = 3
		template, ok := queryTemplateByName("canonical_records")
		Expect(ok).To(BeTrue())
		understanding := adaptLegacyRuntimePlan(request, runtimePlan{Intent: intentRecords, Template: template.Name, Source: "explicit_typed_plan", Confidence: 1}, template)
		capability := capabilityFromTemplate(template, CapabilityProjectionContextV1{Authorized: true, RuntimeAvailable: true, FamilyStates: map[string]WorkspaceCapabilityStateV1{"cdr": {IndexedRecords: 2}}})
		Expect(capability.Availability.Executable).To(BeTrue())
		plan, err := buildSingleCapabilityPlan("group-governed", request, understanding, capability)
		Expect(err).NotTo(HaveOccurred())
		Expect(plan.Steps[0].Parameters.Group).To(Equal(request.Group))
		Expect(plan.Steps[0].Parameters.Limit).To(Equal(100))
		snapshot := buildCapabilitySnapshot("group-snapshot", plan.Workspace, []ResolvedCapabilityV1{capability})
		validation := validateGovernedExecutionPlan(plan, []ResolvedCapabilityV1{capability}, investigationContextFromRequest(request, understanding, snapshot), nxa1ExecutionBudgetCeiling())
		Expect(validation.Valid).To(BeTrue(), "%+v", validation.Issues)
		result, err := executeGovernedRecordsCapability(func(selected string) (map[string]any, error) {
			Expect(selected).To(Equal("canonical_records"))
			bound := requestWithExecutionParameters(request, plan.Steps[0].Parameters)
			groups, err := groupCanonicalRows(bound, []map[string]any{row("g1", "a.csv"), row("g2", "a.csv")})
			return map[string]any{"canonical_groups": groups, "total_count": 2}, err
		}, plan, capability)
		Expect(err).NotTo(HaveOccurred())
		Expect(canonicalAnswerRowCount("canonical_records", result)).To(Equal(1))
		Expect(summarizeRecords("canonical_records", result)).To(ContainSubstring("2 authorized source rows into 1 groups"))
		Expect(flattenEnterpriseRows(result, 100)).To(HaveLen(1))
	})
})
