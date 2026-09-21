package main

import (
	"context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Canonical field projection", func() {
	It("lowers an explicit typed projection without losing the case", func() {
		legacy, early, err := normalizeV1QueryPlan("case-p", "request-p", map[string]any{
			"contract_version": "forensics.query-plan/v1", "intent": "forensics.canonical_records",
			"projection": []any{"primary_target", "secondary_target"},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(early).To(BeNil())
		Expect(legacy["projection"]).To(Equal([]string{"primary_target", "secondary_target"}))
		Expect(legacy["case_id"]).To(Equal("case-p"))
	})
	It("retains exact identifiers, nulls and locators while excluding payload and unrequested fields", func() {
		row := map[string]any{"record_id": "r7", "tenant_id": "t", "collection_id": "c", "file_id": "f", "batch_id": "b", "record_type": "cdr", "source_file": "calls.csv", "row_number": 17, "row_hash": "digest", "timestamp": "2026-09-02T09:00:00Z", "source_table": "forensic.records", "primary_target": "000871", "secondary_target": nil, "raw_payload": map[string]any{"secret": "hidden"}, "metadata": map[string]any{}, "ingested_at": "later"}
		want := map[string]any{"record_id": "r7", "tenant_id": "t", "collection_id": "c", "file_id": "f", "batch_id": "b", "record_type": "cdr", "source_file": "calls.csv", "row_number": 17, "row_hash": "digest", "timestamp": "2026-09-02T09:00:00Z", "source_table": "forensic.records", "primary_target": "000871", "secondary_target": nil}
		out := projectCanonicalRows([]map[string]any{row}, []string{"primary_target", "secondary_target"})
		Expect(out).To(Equal([]map[string]any{want}))
		Expect(row).To(HaveKey("raw_payload"))
		Expect(projectCanonicalRows(nil, []string{"timestamp"})).To(BeEmpty())
	})
	DescribeTable("rejects unsafe projection before querying", func(fields []string) {
		_, err := buildCanonicalRecordsQuery(hybridQueryRequest{Projection: fields})
		Expect(err).To(HaveOccurred())
	}, Entry("unknown", []string{"imaginary"}), Entry("payload", []string{"raw_payload"}), Entry("expression", []string{"count(*)"}), Entry("privacy", []string{"cnic"}), Entry("duplicate", []string{"timestamp", "timestamp"}), Entry("budget", []string{"a", "b", "c", "d", "e", "f", "g", "h"}))
	It("rejects projection on an executor that cannot consume it", func() {
		_, err := runAnalyticalTemplate(context.Background(), nil, hybridQueryRequest{Projection: []string{"timestamp"}}, "frequent_contacts")
		Expect(err).To(MatchError(ContainSubstring("not consumed")))
	})
})
