package main

import (
	"encoding/json"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func sourceNativeFixture() (hybridQueryRequest, []map[string]any, []FieldDescriptorV1) {
	req := hybridQueryRequest{TenantID: "field-tenant", CollectionID: "field-case", RecordType: "generic", QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)}}
	departments := []any{"Operations", "Operations", "Sales", "Sales", "Finance", nil}
	totals := []any{100.5, 200.5, 300.25, 499.75, 250.25, 50.0}
	models := []any{"DX-1", "DX-2", "DX-1", "DX-1", "DX-3", "DX-2"}
	branches := []any{"North", "North", "South", "South", "North", "South"}
	rows := make([]map[string]any, 0, len(departments))
	for index := range departments {
		stamp := time.Date(2026, 9, 7+index, 10, 0, 0, 0, time.UTC)
		mixed := any(fmt.Sprint(index + 1))
		if index == 5 {
			mixed = "unparseable"
		}
		rows = append(rows, map[string]any{
			"record_id": fmt.Sprintf("00000000-0000-4000-8000-%012d", index+1), "tenant_id": req.TenantID, "collection_id": req.CollectionID,
			"file_id": fmt.Sprintf("file-%d", index+1), "batch_id": "10000000-0000-4000-8000-000000000001", "record_type": "generic",
			"timestamp": stamp, "source_file": []string{"invoices-a.csv", "invoices-b.csv"}[index/3], "row_number": int64(index + 1), "row_hash": fmt.Sprintf("hash-%d", index+1), "source_table": "forensic.records",
			"raw_payload": map[string]any{"department": departments[index], "invoice_total": totals[index], "device_model": models[index], "event_type": []string{"created", "created", "paid", "paid", "review", "review"}[index], "event_time": stamp.Format(time.RFC3339), "event_date": stamp.Format("2006-01-02"), "source_branch": branches[index], "approved": index%2 == 0, "mixed_metric": mixed, "secret_token": "hidden"},
			"metadata":    map[string]any{"normalized_fields": map[string]any{}},
		})
	}
	return req, rows, buildSourceNativeFieldCatalog(req, rows)
}

func sourceNativeField(catalog []FieldDescriptorV1, name string) FieldDescriptorV1 {
	for _, field := range catalog {
		if field.NormalizedName == name {
			return field
		}
	}
	return FieldDescriptorV1{}
}

var _ = Describe("Server-issued source-native typed algebra", func() {
	It("derives a stable privacy-safe typed field catalog and bounded retrieval", func() {
		req, rows, catalog := sourceNativeFixture()
		Expect(catalog).To(HaveLen(13))
		Expect(sourceNativeField(catalog, "secret_token").FieldID).To(BeEmpty())
		types := map[string]bool{}
		for _, field := range catalog {
			Expect(field.ContractVersion).To(Equal(fieldDescriptorContractV1))
			Expect(field.FieldID).To(MatchRegexp(`^fld_[a-f0-9]{24}$`))
			Expect(field.Sensitivity).To(Equal("STANDARD"))
			Expect(field.RedactionState).To(Equal("VISIBLE"))
			types[field.EffectiveType] = true
		}
		for _, kind := range []string{fieldTypeString, fieldTypeInteger, fieldTypeDecimal, fieldTypeBoolean, fieldTypeTimestamp, fieldTypeDate} {
			Expect(types[kind]).To(BeTrue(), kind)
		}
		mixed := sourceNativeField(catalog, "mixed_metric")
		Expect(mixed.IncompatibleValueCount).To(Equal(1))
		Expect(mixed.AllowedAggregates).To(BeEmpty())

		question := "Which department has the highest average invoice total?"
		first, firstScores := retrieveSourceNativeFields(question, catalog)
		second, secondScores := retrieveSourceNativeFields(question, buildSourceNativeFieldCatalog(req, rows))
		Expect(first).To(Equal(second))
		Expect(firstScores).To(Equal(secondScores))
		retrievedNames := []string{}
		for _, field := range first {
			retrievedNames = append(retrievedNames, field.NormalizedName)
		}
		Expect(retrievedNames).To(ContainElements("department", "invoice_total"))
		Expect(first).To(HaveLen(sourceNativeFieldTopK))
	})

	It("answers the representative dynamic AVG query with top-one ordering and exact lineage", func() {
		req, rows, catalog := sourceNativeFixture()
		department := sourceNativeField(catalog, "department")
		invoice := sourceNativeField(catalog, "invoice_total")
		plan := &SourceNativePlanV1{ContractVersion: sourceNativePlanContractV1, Project: []string{}, Filters: []SourceNativeFilterV1{}, GroupFields: []string{department.FieldID}, Measures: []SourceNativeMeasureV1{{MeasureID: "m1", Op: "AVG", FieldID: invoice.FieldID}}, Having: []SourceNativeHavingV1{}, Sort: []SourceNativeSortV1{{Target: "m1", Direction: "DESC"}}, Limit: 1}
		result, err := executeSourceNativePlan(req, plan, catalog, rows)
		Expect(err).NotTo(HaveOccurred())
		out := result["source_native_results"].([]map[string]any)
		Expect(out).To(HaveLen(1))
		Expect(out[0]["department"]).To(Equal("Sales"))
		Expect(out[0]["m1"]).To(BeNumerically("==", 400))
		metadata := out[0]["metadata"].(map[string]any)
		Expect(metadata["contributing_row_count"]).To(Equal(2))
		Expect(metadata["source_rows"]).To(HaveLen(2))
		Expect(result["scanned_source_rows"]).To(Equal(6))
		Expect(result["total_count"]).To(Equal(6))
	})

	It("issues only opaque field IDs in the source-native schema and binds analyst literals", func() {
		req, _, catalog := sourceNativeFixture()
		req.Query = "Which department has average invoice total above 350?"
		req.SemanticPlannerAudit = &SemanticPlannerAuditV1{}
		department := sourceNativeField(catalog, "department")
		invoice := sourceNativeField(catalog, "invoice_total")
		retrieved, _ := retrieveSourceNativeFields(req.Query, catalog)
		schemaJSON, err := json.Marshal(semanticSourceNativeSchema(retrieved))
		Expect(err).NotTo(HaveOccurred())
		Expect(string(schemaJSON)).To(ContainSubstring(department.FieldID))
		Expect(string(schemaJSON)).To(ContainSubstring(invoice.FieldID))
		Expect(string(schemaJSON)).NotTo(ContainSubstring("department"))
		Expect(string(schemaJSON)).NotTo(ContainSubstring("invoice_total"))
		Expect(string(schemaJSON)).NotTo(ContainSubstring("raw_payload"))
		Expect(string(schemaJSON)).NotTo(ContainSubstring("SELECT"))

		plan := &SourceNativePlanV1{ContractVersion: sourceNativePlanContractV1, Project: []string{}, Filters: []SourceNativeFilterV1{}, GroupFields: []string{department.FieldID}, Measures: []SourceNativeMeasureV1{{MeasureID: "m1", Op: "AVG", FieldID: invoice.FieldID}}, Having: []SourceNativeHavingV1{{MeasureID: "m1", Op: "GT", Value: "350"}}, Sort: []SourceNativeSortV1{{Target: "m1", Direction: "DESC"}}, Limit: 1}
		bound, err := applySemanticSourceNativePlan(req, "generic_tabular", semanticDynamicProposal{Mode: "source_native", SourceNative: plan}, retrieved)
		Expect(err).NotTo(HaveOccurred())
		Expect(bound.Template).To(Equal("canonical_records"))
		Expect(bound.SourceNative).To(Equal(plan))
		Expect(bound.SemanticPlannerAudit.DynamicPlan).To(Equal(plan))

		invented := *plan
		invented.Having = []SourceNativeHavingV1{{MeasureID: "m1", Op: "GT", Value: "351"}}
		_, err = applySemanticSourceNativePlan(req, "generic_tabular", semanticDynamicProposal{Mode: "source_native", SourceNative: &invented}, retrieved)
		Expect(err).To(MatchError(ContainSubstring("was not supplied")))
	})

	It("executes project, filters, two-key grouping, all aggregates, HAVING, sort, limit and zero results", func() {
		req, rows, catalog := sourceNativeFixture()
		department := sourceNativeField(catalog, "department")
		branch := sourceNativeField(catalog, "source_branch")
		invoice := sourceNativeField(catalog, "invoice_total")
		device := sourceNativeField(catalog, "device_model")

		projection := &SourceNativePlanV1{ContractVersion: sourceNativePlanContractV1, Project: []string{department.FieldID, device.FieldID}, Filters: []SourceNativeFilterV1{{FieldID: branch.FieldID, Op: "EQ", Value: "North", Values: []string{}}}, GroupFields: []string{}, Measures: []SourceNativeMeasureV1{}, Having: []SourceNativeHavingV1{}, Sort: []SourceNativeSortV1{{Target: department.FieldID, Direction: "ASC"}}, Limit: 2}
		projected, err := executeSourceNativePlan(req, projection, catalog, rows)
		Expect(err).NotTo(HaveOccurred())
		Expect(projected["source_native_results"]).To(HaveLen(2))

		aggregate := &SourceNativePlanV1{ContractVersion: sourceNativePlanContractV1, Project: []string{}, Filters: []SourceNativeFilterV1{}, GroupFields: []string{department.FieldID, branch.FieldID}, Measures: []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT"}, {MeasureID: "m2", Op: "SUM", FieldID: invoice.FieldID}, {MeasureID: "m3", Op: "MIN", FieldID: invoice.FieldID}, {MeasureID: "m4", Op: "MAX", FieldID: invoice.FieldID}}, Having: []SourceNativeHavingV1{{MeasureID: "m1", Op: "GTE", Value: "2"}}, Sort: []SourceNativeSortV1{{Target: "m2", Direction: "DESC"}}, Limit: 5}
		grouped, err := executeSourceNativePlan(req, aggregate, catalog, rows)
		Expect(err).NotTo(HaveOccurred())
		groups := grouped["source_native_results"].([]map[string]any)
		Expect(groups).To(HaveLen(2))
		Expect(groups[0]["department"]).To(Equal("Sales"))
		Expect(groups[0]["m1"]).To(Equal(2))
		Expect(groups[0]["m2"]).To(BeNumerically("==", 800))
		Expect(groups[0]["m3"]).To(BeNumerically("==", 300.25))
		Expect(groups[0]["m4"]).To(BeNumerically("==", 499.75))

		zero := *projection
		zero.Filters = []SourceNativeFilterV1{{FieldID: branch.FieldID, Op: "EQ", Value: "Absent", Values: []string{}}}
		empty, err := executeSourceNativePlan(req, &zero, catalog, rows)
		Expect(err).NotTo(HaveOccurred())
		Expect(empty["source_native_results"]).To(BeEmpty())
		Expect(empty["total_count"]).To(BeZero())
	})

	It("reuses calendar buckets and preserves a deterministic null group", func() {
		req, rows, catalog := sourceNativeFixture()
		eventTime := sourceNativeField(catalog, "event_time")
		department := sourceNativeField(catalog, "department")
		bucketed := &SourceNativePlanV1{ContractVersion: sourceNativePlanContractV1, Project: []string{}, Filters: []SourceNativeFilterV1{}, GroupFields: []string{}, Measures: []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT"}}, TimeBucket: &SourceNativeTimeBucketV1{FieldID: eventTime.FieldID, Bucket: "week", Timezone: "Asia/Karachi"}, Having: []SourceNativeHavingV1{}, Sort: []SourceNativeSortV1{{Target: eventTime.FieldID, Direction: "ASC"}}, Limit: 10}
		result, err := executeSourceNativePlan(req, bucketed, catalog, rows)
		Expect(err).NotTo(HaveOccurred())
		Expect(result["source_native_results"]).To(HaveLen(1))

		nulls := &SourceNativePlanV1{ContractVersion: sourceNativePlanContractV1, Project: []string{}, Filters: []SourceNativeFilterV1{}, GroupFields: []string{department.FieldID}, Measures: []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT"}}, Having: []SourceNativeHavingV1{}, Sort: []SourceNativeSortV1{{Target: department.FieldID, Direction: "ASC"}}, Limit: 10}
		result, err = executeSourceNativePlan(req, nulls, catalog, rows)
		Expect(err).NotTo(HaveOccurred())
		foundNull := false
		for _, row := range result["source_native_results"].([]map[string]any) {
			foundNull = foundNull || row["department"] == nil
		}
		Expect(foundNull).To(BeTrue())
	})

	It("fails closed on privacy, unissued/cross-scope fields, unsafe types, injection and caps", func() {
		req, rows, catalog := sourceNativeFixture()
		department := sourceNativeField(catalog, "department")
		mixed := sourceNativeField(catalog, "mixed_metric")
		Expect(sourceNativeField(catalog, "secret_token").FieldID).To(BeEmpty())
		base := SourceNativePlanV1{ContractVersion: sourceNativePlanContractV1, Project: []string{department.FieldID}, Filters: []SourceNativeFilterV1{}, GroupFields: []string{}, Measures: []SourceNativeMeasureV1{}, Having: []SourceNativeHavingV1{}, Sort: []SourceNativeSortV1{}, Limit: 10}
		for _, id := range []string{"fld_nonexistent", "fld_x');DROP TABLE forensic.records;--"} {
			invalid := base
			invalid.Project = []string{id}
			_, err := executeSourceNativePlan(req, &invalid, catalog, rows)
			Expect(err).To(HaveOccurred())
		}
		crossScope := req
		crossScope.CollectionID = "other-case"
		crossCatalog := buildSourceNativeFieldCatalog(crossScope, rows)
		_, err := executeSourceNativePlan(crossScope, &base, crossCatalog, rows)
		Expect(err).To(HaveOccurred())

		unsafe := base
		unsafe.Project = []string{}
		unsafe.Measures = []SourceNativeMeasureV1{{MeasureID: "m1", Op: "AVG", FieldID: mixed.FieldID}}
		_, err = executeSourceNativePlan(req, &unsafe, catalog, rows)
		Expect(err).To(HaveOccurred())

		overflow := base
		overflow.Project = make([]string, sourceNativeProjectLimit+1)
		Expect(validateSourceNativePlanShape(&overflow)).To(HaveOccurred())
		var decoded SourceNativePlanV1
		Expect(json.Unmarshal([]byte(`{"contract_version":"forensics.source-native-plan/v1","project":["x"],"filters":[],"group_fields":[],"measures":[],"time_bucket":null,"having":[],"sort":[],"limit":1,"sql":"select 1"}`), &decoded)).To(HaveOccurred())
	})
})
