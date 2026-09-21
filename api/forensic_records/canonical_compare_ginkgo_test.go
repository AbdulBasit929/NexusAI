package main

import (
	"encoding/json"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Bounded source comparisons", func() {
	value := func(v string) *string { return &v }
	request := func() hybridQueryRequest {
		return hybridQueryRequest{TenantID: "t", CollectionID: "c", Compare: &CanonicalCompareV1{Field: "source_file", Measure: "count", A: CanonicalComparisonOperandV1{Value: value("A.csv")}, B: CanonicalComparisonOperandV1{Value: value("B.csv")}}}
	}
	rows := func() []map[string]any {
		return []map[string]any{
			{"record_id": "r1", "tenant_id": "t", "collection_id": "c", "source_file": "A.csv", "row_number": 1, "row_hash": "h1", "timestamp": "2026-06-02T00:00:00Z"},
			{"record_id": "r2", "tenant_id": "t", "collection_id": "c", "source_file": "A.csv", "row_number": 2, "row_hash": "h2", "timestamp": "2026-06-03T00:00:00Z"},
			{"record_id": "r3", "tenant_id": "t", "collection_id": "c", "source_file": "B.csv", "row_number": 3, "row_hash": "h3", "timestamp": "2026-06-04T00:00:00Z"},
		}
	}
	It("computes A/B counts and signed difference with exact lineage", func() {
		out, err := compareCanonicalRows(request(), rows())
		Expect(err).NotTo(HaveOccurred())
		Expect(out["difference_b_minus_a"]).To(Equal(-1))
		Expect(out["overlapping_source_rows"]).To(Equal(0))
		groups := out["canonical_comparison"].([]map[string]any)
		Expect(groups[0]["count"]).To(Equal(2))
		Expect(groups[1]["count"]).To(Equal(1))
		Expect(flattenEnterpriseRows(out, 100)).To(HaveLen(2))
		line := groups[0]["metadata"].(map[string]any)["source_rows"].([]map[string]any)
		Expect(line[1]["row_hash"]).To(Equal("h2"))
		empty, err := compareCanonicalRows(request(), nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(empty["difference_b_minus_a"]).To(Equal(0))
	})
	It("uses half-open periods and explicitly counts overlapping contributions", func() {
		req := request()
		req.Compare = &CanonicalCompareV1{Field: "timestamp", Measure: "count", A: CanonicalComparisonOperandV1{From: "2026-06-02T00:00:00Z", To: "2026-06-04T00:00:00Z"}, B: CanonicalComparisonOperandV1{From: "2026-06-03T00:00:00Z", To: "2026-06-05T00:00:00Z"}}
		out, err := compareCanonicalRows(req, rows())
		Expect(err).NotTo(HaveOccurred())
		Expect(out["difference_b_minus_a"]).To(Equal(0))
		Expect(out["overlapping_source_rows"]).To(Equal(1))
		Expect(out["total_count"]).To(Equal(3))
	})
	It("rejects scope substitution, unknown values, unbounded input and authored SQL", func() {
		r := rows()
		r[0]["tenant_id"] = "foreign"
		_, err := compareCanonicalRows(request(), r)
		Expect(err).To(HaveOccurred())
		r = rows()
		r[0]["source_file"] = nil
		_, err = compareCanonicalRows(request(), r)
		Expect(err).To(HaveOccurred())
		_, err = compareCanonicalRows(request(), make([]map[string]any, 1001))
		Expect(err).To(HaveOccurred())
		var c CanonicalCompareV1
		Expect(json.Unmarshal([]byte(`{"field":"source_file","measure":"count","a":{"value":"A","sql":"select 1"},"b":{"value":"B"}}`), &c)).NotTo(Succeed())
	})
})
