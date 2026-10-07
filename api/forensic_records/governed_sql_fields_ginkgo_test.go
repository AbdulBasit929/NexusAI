package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The three loose readings measured in arm C on 2026-10-06, each against the real curated layer.
var _ = Describe("Governed SQL fields and filters", func() {
	var (
		all   []*govSQLView
		views map[string]*govSQLView
	)
	BeforeEach(func() {
		views = govSQLTestViews()
		all = nil
		for _, v := range views {
			all = append(all, v)
		}
	})
	kinds := func(question, sql string, offered ...string) []string {
		picked := []*govSQLView{}
		for _, name := range offered {
			picked = append(picked, views[name])
		}
		validated, rejection := govSQLValidate(sql, views)
		Expect(rejection).To(BeNil(), "validator rejected: %v", rejection)
		out := []string{}
		for _, item := range govSQLCheck(govSQLExtractFacts(question, picked, all), validated) {
			out = append(out, item.Kind)
		}
		return out
	}

	It("holds the model to the column the layer says 'position' names", func() {
		q := "What is the smallest position among the tower records?"
		Expect(kinds(q, "SELECT min(longitude) AS smallest_position FROM v_tower", "v_tower")).To(ContainElement("FIELD"))
		Expect(kinds(q, "SELECT min(latitude) AS smallest_position FROM v_tower", "v_tower")).NotTo(ContainElement("FIELD"))
	})

	It("does not make a demand of a generic word or of the view's own name", func() {
		for _, q := range []string{"How many CDR records are there?", "What is the total number of call records?"} {
			Expect(kinds(q, "SELECT COUNT(*) AS n FROM v_cdr", "v_cdr")).To(BeEmpty(), q)
		}
	})

	It("lets the longest name win: 'call start time' is not also a demand for event_time", func() {
		q := "What is the latest call start time among the calls?"
		Expect(kinds(q, "SELECT max(call_start) AS latest FROM v_cdr", "v_cdr")).NotTo(ContainElement("FIELD"))
	})

	It("refuses a filter the question never asked for, and a value the layer does not declare", func() {
		q := "Which is the most recent call start time among the calls?"
		Expect(kinds(q, "SELECT max(call_start) AS latest FROM v_cdr WHERE call_type = 'CALL'", "v_cdr")).To(ContainElement("FILTER"))
		Expect(kinds(q, "SELECT max(call_start) AS latest FROM v_cdr WHERE call_type = 'VOICE'", "v_cdr")).To(ContainElement("FILTER"))
		Expect(kinds(q, "SELECT max(call_start) AS latest FROM v_cdr", "v_cdr")).NotTo(ContainElement("FILTER"))
	})

	It("accepts a filter the question does ask for", func() {
		Expect(kinds("How many VoLTE calls are there?", "SELECT COUNT(*) AS n FROM v_cdr WHERE call_type = 'VOLTE'", "v_cdr")).NotTo(ContainElement("FILTER"))
		Expect(kinds("How many voice calls are there?", "SELECT COUNT(*) AS n FROM v_cdr WHERE call_type = 'VOICE'", "v_cdr")).NotTo(ContainElement("FILTER"))
	})

	It("ranks the view that has the column the question names first", func() {
		offered := govSQLShortlist(all, "Which is the first activation date among the registered numbers?")
		Expect(offered).NotTo(BeEmpty())
		Expect(offered[0].Name).To(Equal("v_subscriber"))
	})

	It("does not read 'data' in 'data volume' as a request for data-type calls", func() {
		q := "What is the total data volume of all the call records?"
		Expect(kinds(q, "SELECT sum(network_volume) AS total FROM v_cdr WHERE call_type = 'GPRS'", "v_cdr")).To(ContainElement("FILTER"))
		Expect(kinds(q, "SELECT sum(network_volume) AS total FROM v_cdr", "v_cdr")).To(BeEmpty())
	})

	It("holds the query to the evidence family the question names", func() {
		q := "What is the highest data volume in the data sessions?"
		Expect(kinds(q, "SELECT max(network_volume) AS highest FROM v_cdr", "v_ipdr", "v_cdr")).To(ContainElement("VIEW"))
		Expect(kinds(q, "SELECT max(bytes) AS highest FROM v_ipdr", "v_ipdr", "v_cdr")).NotTo(ContainElement("VIEW"))
	})

	It("shows identifiers as stored and rounds a long fraction instead of cutting it", func() {
		Expect(govSQLFormatCell(views["v_cdr"], "msisdn", "923461678183")).To(Equal("923461678183"))
		Expect(govSQLFormatCell(views["v_cdr"], "number_of_records", "4454")).To(Equal("4,454"))
		Expect(govSQLFormatNumber("29.04906")).To(Equal("29.0491"))
		Expect(govSQLFormatNumber("0.86201200000000")).To(Equal("0.8620"[:5]))
		Expect(govSQLFormatNumber("-31.61180696670")).To(Equal("-31.6118"))
		Expect(govSQLFormatNumber("3905649.933600000000")).To(Equal("3,905,649.9336"))
	})

	It("reads a plural as the curated singular and lets the longest name win", func() {
		named := govSQLNamedViews("What is the average OCR confidence of the plate reads?", all)
		Expect(named).To(HaveKey("v_anpr_model_observation"))
		Expect(named).NotTo(HaveKey("v_anpr"))
		Expect(govSQLNamedViews("How many ANPR sightings are there?", all)).To(HaveKey("v_anpr"))
	})

	It("will not answer a grouping the evidence has no column for", func() {
		Expect(kinds("How many records do we have for each record type?", "SELECT call_type, COUNT(*) AS n FROM v_cdr GROUP BY call_type", "v_cdr")).To(ContainElement("GROUPING"))
		Expect(kinds("How many calls of each type are there?", "SELECT call_type, COUNT(*) AS n FROM v_cdr GROUP BY call_type", "v_cdr")).NotTo(ContainElement("GROUPING"))
		Expect(kinds("How many CDR records came from each source file?", "SELECT source_file, COUNT(*) AS n FROM v_cdr GROUP BY source_file", "v_cdr")).NotTo(ContainElement("GROUPING"))
		Expect(kinds("Break down IPDR sessions by protocol", "SELECT protocol, COUNT(*) AS n FROM v_ipdr GROUP BY protocol", "v_ipdr")).NotTo(ContainElement("GROUPING"))
	})

	It("does not read the 'call' of 'call records' as a request for call_type = CALL", func() {
		q := "What is the average event latitude of the call records?"
		Expect(kinds(q, "SELECT avg(latitude) AS average FROM v_cdr WHERE call_type = 'CALL'", "v_cdr")).To(ContainElement("FILTER"))
		Expect(kinds(q, "SELECT avg(latitude) AS average FROM v_cdr", "v_cdr")).NotTo(ContainElement("FILTER"))
	})

	It("accepts a value the question says in the layer's own words, plural or not", func() {
		Expect(kinds("How many server errors are in the access log?", "SELECT COUNT(*) AS n FROM v_access_log WHERE status = '500'", "v_access_log")).NotTo(ContainElement("FILTER"))
		Expect(kinds("How many requests were forbidden?", "SELECT COUNT(*) AS n FROM v_access_log WHERE status = '403'", "v_access_log")).NotTo(ContainElement("FILTER"))
	})

	It("counts NOT IN and LIKE as filters", func() {
		Expect(kinds("What is the most common failed in the access log entries?", "SELECT status FROM v_access_log WHERE status NOT IN ('200', '201') GROUP BY status ORDER BY count(*) DESC LIMIT 1", "v_access_log")).To(ContainElement("FILTER"))
		Expect(kinds("How many unique failed do the web requests have?", "SELECT count(DISTINCT source_ip) AS n FROM v_access_log WHERE status LIKE '4%'", "v_access_log")).To(ContainElement("FILTER"))
	})
})
