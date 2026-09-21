package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"time"
	_ "time/tzdata"
)

var _ = Describe("Governed calendar buckets", func() {
	DescribeTable("uses explicit calendar boundaries including offset changes", func(input, unit, zone, from, to string) {
		stamp, err := time.Parse(time.RFC3339, input)
		Expect(err).NotTo(HaveOccurred())
		a, b, err := canonicalTimeBucket(stamp, &CanonicalGroupV1{Field: "timestamp", Measure: "count", Bucket: unit, Timezone: zone})
		Expect(err).NotTo(HaveOccurred())
		Expect(a.Format(time.RFC3339)).To(Equal(from))
		Expect(b.Format(time.RFC3339)).To(Equal(to))
	},
		Entry("Pakistan local day", "2026-07-01T21:12:00Z", "day", "Asia/Karachi", "2026-07-02T00:00:00+05:00", "2026-07-03T00:00:00+05:00"),
		Entry("Monday week across month", "2026-08-01T21:12:00Z", "week", "UTC", "2026-07-27T00:00:00Z", "2026-08-03T00:00:00Z"),
		Entry("leap month", "2028-02-29T12:00:00Z", "month", "UTC", "2028-02-01T00:00:00Z", "2028-03-01T00:00:00Z"),
		Entry("23 hour spring day", "2026-03-08T17:00:00Z", "day", "America/New_York", "2026-03-08T00:00:00-05:00", "2026-03-09T00:00:00-04:00"),
		Entry("first repeated hour", "2026-11-01T05:35:00Z", "hour", "America/New_York", "2026-11-01T01:00:00-04:00", "2026-11-01T01:00:00-05:00"),
		Entry("second repeated hour", "2026-11-01T06:35:00Z", "hour", "America/New_York", "2026-11-01T01:00:00-05:00", "2026-11-01T02:00:00-05:00"),
	)
	It("rejects missing timezone and open/reversed time bounds", func() {
		g := &CanonicalGroupV1{Field: "timestamp", Measure: "count", Bucket: "day"}
		Expect(validateCanonicalGroup(g)).NotTo(Succeed())
		g.Timezone = "Asia/Karachi"
		Expect(validateCanonicalGroupRequest(hybridQueryRequest{Group: g})).NotTo(Succeed())
		Expect(validateCanonicalGroupRequest(hybridQueryRequest{Group: g, DateFrom: "2026-07-03T00:00:00Z", DateTo: "2026-07-02T00:00:00Z"})).NotTo(Succeed())
	})
})
