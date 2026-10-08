package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Run 12, part B: the lane abstained on three of the four date-range questions it saw with "the query did
// not apply the condition 'between 2025'". The shared quantity detector reads "between" followed by a digit,
// and the year that begins a date is a digit. A date range is a time condition, not a quantity.
var _ = Describe("Governed SQL: a date range is a time condition, not a quantity", func() {
	var (
		views map[string]*govSQLView
		all   []*govSQLView
	)
	BeforeEach(func() {
		views = govSQLTestViews()
		all = nil
		for _, v := range views {
			all = append(all, v)
		}
	})
	factsFor := func(question, view string) govSQLQuestionFacts {
		return govSQLExtractFacts(question, []*govSQLView{views[view]}, all)
	}
	unmetKinds := func(question, view, sql string) []string {
		validated, rejection := govSQLValidate(sql, views)
		Expect(rejection).To(BeNil(), "validator rejected: %v", rejection)
		kinds := []string{}
		for _, item := range govSQLCheck(factsFor(question, view), validated) {
			kinds = append(kinds, item.Kind)
		}
		return kinds
	}

	It("reads no quantity in the questions run 12 abstained on, and keeps the time condition", func() {
		for question, view := range map[string]string{
			"How many subscribers have a activation date between 2025-03-09 and 2025-12-13?":        "v_subscriber",
			"How many CDR records are there between 2026-04-26 and 2026-05-30?":                     "v_cdr",
			"How many registered numbers have a activation date between 2023-01-07 and 2024-08-21?": "v_subscriber",
			"How many calls were there between 04/05/2026 and 20/05/2026?":                          "v_cdr",
			"How many calls were there between 2025 and 2026?":                                      "v_cdr",
		} {
			facts := factsFor(question, view)
			Expect(facts.Magnitude).To(BeEmpty(), question)
			Expect(facts.Numbers).To(BeEmpty(), question)
			Expect(facts.TimeCues).NotTo(BeEmpty(), question)
		}
	})

	It("still reads a real quantity, with its numbers", func() {
		for question, want := range map[string]string{
			"Which numbers made more than 10 calls?":              "more than 10",
			"Which numbers made between 10 and 20 calls?":         "between 10",
			"How many calls lasted longer than 300 seconds?":      "longer than 300",
			"How many records have more than 2000 bytes?":         "more than 2000",
			"Which numbers made between 10 and 20 calls in 2026?": "between 10",
		} {
			Expect(factsFor(question, "v_cdr").Magnitude).To(ContainSubstring(want), question)
		}
		Expect(factsFor("Which numbers made between 10 and 20 calls?", "v_cdr").Numbers).To(ConsistOf("10"))
	})

	It("reads a quantity next to a date range as the quantity, and the range as time", func() {
		facts := factsFor("Which numbers made more than 10 calls between 2026-04-01 and 2026-04-30?", "v_cdr")
		Expect(facts.Magnitude).To(ContainSubstring("more than 10"))
		Expect(facts.TimeCues).NotTo(BeEmpty())
	})

	It("accepts a query that compares the dates, for the question that was abstained on", func() {
		Expect(unmetKinds("How many CDR records are there between 2026-04-26 and 2026-05-30?", "v_cdr",
			"SELECT COUNT(*) AS number_of_call_records FROM v_cdr WHERE event_time >= '2026-04-26' AND event_time < '2026-05-31'")).To(BeEmpty())
		Expect(unmetKinds("How many CDR records are there between 2026-04-26 and 2026-05-30?", "v_cdr",
			"SELECT COUNT(*) AS number_of_call_records FROM v_cdr WHERE event_time::date BETWEEN '2026-04-26' AND '2026-05-30'")).To(BeEmpty())
		// Phase 1: BETWEEN on a timestamp stops at the midnight that begins the last day (921 where the key is 1,013);
		// run 14 holds a range to both days in full (governed_sql_guards_ginkgo_test.go).
		Expect(unmetKinds("How many CDR records are there between 2026-04-26 and 2026-05-30?", "v_cdr",
			"SELECT COUNT(*) AS number_of_call_records FROM v_cdr WHERE event_time BETWEEN '2026-04-26' AND '2026-05-30'")).To(ConsistOf("RANGE"))
		Expect(unmetKinds("How many subscribers have a activation date between 2025-03-09 and 2025-12-13?", "v_subscriber",
			"SELECT COUNT(*) AS number_of_subscribers FROM v_subscriber WHERE activation_date >= '2025-03-09' AND activation_date < '2025-12-14'")).To(BeEmpty())
	})

	It("still demands the time condition: a date range answered without one is refused", func() {
		Expect(unmetKinds("How many CDR records are there between 2026-04-26 and 2026-05-30?", "v_cdr",
			"SELECT COUNT(*) AS number_of_call_records FROM v_cdr")).To(ConsistOf("TIME", "RANGE"))
	})

	It("still demands a comparison for a real quantity", func() {
		Expect(unmetKinds("Which numbers made more than 10 calls?", "v_cdr",
			"SELECT msisdn, COUNT(*) AS calls FROM v_cdr GROUP BY msisdn ORDER BY calls DESC")).To(ConsistOf("MAGNITUDE"))
	})

	It("removes dates and year ranges and nothing else from the text read for a quantity", func() {
		Expect(govSQLMagnitudeText("between 2025-03-09 and 2025-12-13")).NotTo(ContainSubstring("2025"))
		Expect(govSQLMagnitudeText("between 04/05/2026 and 20/05/2026")).NotTo(ContainSubstring("2026"))
		Expect(govSQLMagnitudeText("between 2025 and 2026")).NotTo(ContainSubstring("between"))
		Expect(govSQLMagnitudeText("more than 2000 calls")).To(Equal("more than 2000 calls"))
		Expect(govSQLMagnitudeText("between 10 and 20 calls")).To(Equal("between 10 and 20 calls"))
	})
})
