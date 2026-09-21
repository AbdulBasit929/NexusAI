package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Typed algebra control preservation", func() {
	DescribeTable("rejects controls that compatibility conversion would discard",
		func(key string, value any) {
			plan := map[string]any{"contract_version": "forensics.query-plan/v1", "intent": "forensics.canonical_records", key: value}
			legacy, early, err := normalizeV1QueryPlan("case-a", "request-a", plan)
			Expect(err).To(HaveOccurred())
			Expect(legacy).To(BeNil())
			Expect(early).To(BeNil())
		},
		Entry("object instead of filter array", "filters", map[string]any{"field": "status", "op": "eq", "value": "active"}),
		Entry("mixed filter array", "filters", []any{map[string]any{"field": "status", "op": "eq", "value": "active"}, "ignored"}),
		Entry("null filters", "filters", nil),
		Entry("null row", "filters", []any{nil}),
		Entry("filter extension", "filters", []any{map[string]any{"field": "status", "op": "eq", "value": "active", "negate": true}}),
		Entry("numeric field", "filters", []any{map[string]any{"field": 123, "op": "exists"}}),
		Entry("malformed sort", "sort", []any{"timestamp"}),
		Entry("sort extension", "sort", []any{map[string]any{"field": "timestamp", "direction": "asc", "nulls": "first"}}),
		Entry("scalar grouping", "group_by", "status"),
		Entry("null measure", "measures", []any{nil}),
		Entry("blank measure", "measures", []string{" "}),
		Entry("unsupported projection", "projection", []any{"status"}),
		Entry("unsupported time bucket", "time_bucket", "hour"),
		Entry("unsupported comparison", "compare", map[string]any{"period": "yesterday"}),
		Entry("unsupported composition", "steps", []any{}),
	)

	It("rejects a second direction instead of executing only the first", func() {
		_, _, err := normalizeV1QueryPlan("case-a", "request-a", map[string]any{
			"contract_version": "forensics.query-plan/v1", "intent": "cdr.frequent_contacts",
			"filters": []any{
				map[string]any{"field": "direction", "op": "eq", "value": "incoming"},
				map[string]any{"field": "direction", "op": "eq", "value": "outgoing"},
			},
		})
		Expect(err).To(MatchError(ContainSubstring("repeats scalar filter")))
	})

	It("recognizes duplicate video-bound aliases", func() {
		_, _, err := normalizeV1QueryPlan("case-a", "request-a", map[string]any{
			"contract_version": "forensics.query-plan/v1", "intent": "video.timeline",
			"filters": []any{
				map[string]any{"field": "start_seconds", "op": "gte", "value": 10},
				map[string]any{"field": "source_start_seconds", "op": "gte", "value": 2},
			},
		})
		Expect(err).To(MatchError(ContainSubstring("repeats scalar filter")))
	})

	It("preserves canonical range conjunctions and their exact values", func() {
		filters := []map[string]any{{"field": "amount", "op": "gte", "value": 12}, {"field": "amount", "op": "lt", "value": 19}}
		legacy, early, err := normalizeV1QueryPlan("case-a", "request-a", map[string]any{
			"contract_version": "forensics.query-plan/v1", "intent": "forensics.canonical_records",
			"filters": filters, "sort": []any{}, "measures": []string{}, "group_by": []any{},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(early).To(BeNil())
		Expect(legacy["raw_payload_filters"]).To(Equal(filters))
	})
})
