package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The verifier is tested on the failure classes the 2026-10-05 baseline measured:
// a constraint the question states and the query silently drops. Each case pairs a
// question with a query that ignores part of it (which must be flagged) and a query
// that honours it (which must pass).
var _ = Describe("Governed SQL verification", func() {
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
	factsFor := func(question string, names ...string) govSQLQuestionFacts {
		picked := []*govSQLView{}
		for _, name := range names {
			picked = append(picked, views[name])
		}
		return govSQLExtractFacts(question, picked, all)
	}
	unmetKinds := func(facts govSQLQuestionFacts, sql string) []string {
		validated, rejection := govSQLValidate(sql, views)
		Expect(rejection).To(BeNil(), "validator rejected: %v", rejection)
		kinds := []string{}
		for _, item := range govSQLCheck(facts, validated) {
			kinds = append(kinds, item.Kind)
		}
		return kinds
	}

	Describe("reading the question", func() {
		It("finds identifiers of every kind and ignores dates", func() {
			Expect(factsFor("How many calls did 923001110001 make?", "v_cdr").Identifiers).To(ConsistOf("923001110001"))
			Expect(factsFor("calls from +92 300 1110001", "v_cdr").Identifiers).To(HaveLen(1))
			Expect(factsFor("Count the vehicle sightings with plate number XYZ-789", "v_anpr").Identifiers).To(ConsistOf("XYZ-789"))
			Expect(factsFor("How many sessions have host mail.example.test?", "v_ipdr").Identifiers).To(ConsistOf("mail.example.test"))
			Expect(factsFor("requests from 192.168.1.10", "v_access_log").Identifiers).To(ConsistOf("192.168.1.10"))
			Expect(factsFor("calls between 2026-04-01 and 2026-04-30", "v_cdr").Identifiers).To(BeEmpty())
			Expect(factsFor("What is the status PK-SUB-SYN-ALPHA has?", "v_subscriber").Identifiers).To(ConsistOf("PK-SUB-SYN-ALPHA"))
		})

		It("enforces a value the question says as a code or names by its column", func() {
			facts := factsFor("How many VoLTE calls are there?", "v_cdr")
			Expect(facts.ValueFacts).To(ContainElement(HaveField("Column", "call_type")))
			Expect(facts.ValueFacts[0].Value).To(Equal("VOLTE"))
			named := factsFor("How many calls are there where the call direction is incoming?", "v_cdr")
			Expect(named.ValueFacts).To(ContainElement(HaveField("Value", "INCOMING")))
			Expect(factsFor("How many log entries have status 403?", "v_access_log").ValueFacts).To(ContainElement(HaveField("Value", "403")))
		})

		It("only hints at a value when the question uses an everyday word for it", func() {
			for _, question := range []string{"What is the earliest call in the data?", "How many calls were made in April 2026?", "Which numbers made more than 10 calls?"} {
				facts := factsFor(question, "v_cdr")
				Expect(facts.ValueFacts).To(BeEmpty(), question)
			}
			voice := factsFor("How many voice calls are there?", "v_cdr")
			Expect(voice.ValueFacts).To(BeEmpty())
			Expect(voice.WeakValues).To(ContainElement(HaveField("Value", "VOICE")))
		})

		It("reads a stated magnitude with its number", func() {
			facts := factsFor("Which numbers made more than 10 calls?", "v_cdr")
			Expect(facts.Magnitude).To(ContainSubstring("more than"))
			Expect(facts.Numbers).To(ConsistOf("10"))
		})

		It("reads time constraints and a time of day, and not a bare 'per month'", func() {
			Expect(factsFor("How many calls were made in April 2026?", "v_cdr").TimeCues).NotTo(BeEmpty())
			night := factsFor("How many data sessions were there at night?", "v_ipdr")
			Expect(night.TimeOfDay).To(BeTrue())
			Expect(factsFor("How many calls per month?", "v_cdr").TimeCues).To(BeEmpty())
			Expect(factsFor("How many calls are there?", "v_cdr").TimeCues).To(BeEmpty())
		})

		It("finds a name the layer does not know, and nothing in ordinary questions", func() {
			Expect(factsFor("How many calls did Ahmed make?", "v_cdr").Names).To(ConsistOf("Ahmed"))
			Expect(factsFor("How many log entries did Nadia Farooqui have?", "v_access_log").Names).To(ConsistOf("Nadia Farooqui"))
			Expect(factsFor("What is the earliest call in the data?", "v_cdr").Names).To(BeEmpty())
			Expect(factsFor("How many CDR records are there in April?", "v_cdr").Names).To(BeEmpty())
			Expect(factsFor("How many VoLTE calls are there?", "v_cdr").Names).To(BeEmpty())
			Expect(factsFor("Count the ANPR records", "v_anpr").Names).To(BeEmpty())
		})

		It("tells a single-value question from a list question", func() {
			Expect(factsFor("How many calls are there?", "v_cdr").WantsSingle).To(BeTrue())
			Expect(factsFor("What is the average data volume?", "v_cdr").WantsSingle).To(BeTrue())
			Expect(factsFor("Which number made the most calls?", "v_cdr").WantsSingle).To(BeFalse())
			Expect(factsFor("How many calls per call type?", "v_cdr").WantsSingle).To(BeFalse())
		})

		It("notices a request for an attribute the evidence withholds", func() {
			Expect(factsFor("What is the CNIC of subscriber 923461678183?", "v_subscriber").WithheldAttr).NotTo(BeEmpty())
			Expect(factsFor("How many subscribers are active?", "v_subscriber").WithheldAttr).To(BeEmpty())
		})
	})

	Describe("checking the query against the question", func() {
		It("flags a query that drops the identifier the question names", func() {
			facts := factsFor("How many calls did 923001110001 make?", "v_cdr")
			Expect(unmetKinds(facts, "SELECT COUNT(*) AS calls FROM v_cdr")).To(ConsistOf("IDENTIFIER"))
			Expect(unmetKinds(facts, "SELECT COUNT(*) AS calls FROM v_cdr WHERE msisdn = '923001110001'")).To(BeEmpty())
			Expect(unmetKinds(facts, "SELECT COUNT(*) AS calls FROM v_cdr WHERE msisdn = '923001110001' OR dialed_number = '923001110001'")).To(BeEmpty())
			Expect(unmetKinds(facts, "SELECT COUNT(*) AS calls FROM v_cdr WHERE msisdn LIKE '%3001110001%'")).To(BeEmpty())
			Expect(unmetKinds(facts, "SELECT COUNT(*) AS calls FROM v_cdr WHERE msisdn = '923009998885'")).To(ConsistOf("IDENTIFIER"))
		})

		It("treats 0300... and 92300... as the same number", func() {
			facts := factsFor("How many calls did 03001110001 make?", "v_cdr")
			Expect(unmetKinds(facts, "SELECT COUNT(*) FROM v_cdr WHERE msisdn = '923001110001'")).To(BeEmpty())
		})

		It("flags a query that drops a declared value", func() {
			facts := factsFor("How many VoLTE calls are there?", "v_cdr")
			Expect(unmetKinds(facts, "SELECT COUNT(*) AS calls FROM v_cdr")).To(ConsistOf("VALUE"))
			Expect(unmetKinds(facts, "SELECT COUNT(*) AS calls FROM v_cdr WHERE call_type = 'VOLTE'")).To(BeEmpty())
			Expect(unmetKinds(facts, "SELECT COUNT(*) AS calls FROM v_cdr WHERE direction = 'VOLTE'")).To(ConsistOf("VALUE", "FILTER"))
		})

		It("flags a query that drops a stated magnitude", func() {
			facts := factsFor("Which numbers made more than 10 calls?", "v_cdr")
			Expect(unmetKinds(facts, "SELECT msisdn, COUNT(*) AS calls FROM v_cdr GROUP BY msisdn ORDER BY calls DESC")).To(ConsistOf("MAGNITUDE"))
			Expect(unmetKinds(facts, "SELECT msisdn, COUNT(*) AS calls FROM v_cdr GROUP BY msisdn HAVING COUNT(*) > 10")).To(BeEmpty())
			Expect(unmetKinds(facts, "SELECT msisdn, COUNT(*) AS calls FROM v_cdr GROUP BY msisdn HAVING COUNT(*) = 10")).To(ConsistOf("MAGNITUDE"))
		})

		It("flags a query that drops a time restriction, and does not count a time column that is only selected", func() {
			facts := factsFor("How many calls were made in April 2026?", "v_cdr")
			Expect(unmetKinds(facts, "SELECT COUNT(*) AS calls FROM v_cdr")).To(ConsistOf("TIME"))
			Expect(unmetKinds(facts, "SELECT MIN(call_start) AS first_call FROM v_cdr")).To(ConsistOf("TIME"))
			Expect(unmetKinds(facts, "SELECT COUNT(*) AS calls FROM v_cdr WHERE call_start >= '2026-04-01' AND call_start < '2026-05-01'")).To(BeEmpty())
			Expect(unmetKinds(facts, "SELECT COUNT(*) AS calls FROM v_cdr WHERE EXTRACT(MONTH FROM call_start) = 4 AND EXTRACT(YEAR FROM call_start) = 2026")).To(BeEmpty())
		})

		It("flags 'at night' answered without the hour", func() {
			facts := factsFor("How many data sessions were there at night?", "v_ipdr")
			Expect(unmetKinds(facts, "SELECT COUNT(*) AS sessions FROM v_ipdr")).To(ConsistOf("TIME"))
			Expect(unmetKinds(facts, "SELECT COUNT(*) AS sessions FROM v_ipdr WHERE event_time >= '2026-01-01'")).To(ConsistOf("TIME_OF_DAY"))
			Expect(unmetKinds(facts, "SELECT COUNT(*) AS sessions FROM v_ipdr WHERE EXTRACT(HOUR FROM event_time) BETWEEN 0 AND 5")).To(BeEmpty())
		})

		It("flags a name, whether the query ignores it or pretends it is an identifier", func() {
			facts := factsFor("How many calls did Ahmed make?", "v_cdr")
			Expect(unmetKinds(facts, "SELECT COUNT(*) AS calls FROM v_cdr")).To(ConsistOf("NAME"))
			Expect(unmetKinds(facts, "SELECT COUNT(*) AS calls FROM v_cdr WHERE msisdn = 'Ahmed'")).To(ConsistOf("NAME"))
		})

		It("lets a value the layer does not enumerate be used on a descriptive column", func() {
			facts := factsFor("Count the registered numbers with city Lahore", "v_subscriber")
			Expect(facts.Names).To(ConsistOf("Lahore"))
			Expect(unmetKinds(facts, "SELECT COUNT(*) AS numbers FROM v_subscriber")).To(ConsistOf("NAME"))
			Expect(unmetKinds(facts, "SELECT COUNT(*) AS numbers FROM v_subscriber WHERE city = 'Lahore'")).To(BeEmpty())
			Expect(unmetKinds(facts, "SELECT COUNT(*) AS numbers FROM v_subscriber WHERE LOWER(city) = 'lahore'")).To(BeEmpty())
		})

		It("passes a plain question with nothing to demand", func() {
			facts := factsFor("How many CDR records are there?", "v_cdr")
			Expect(unmetKinds(facts, "SELECT COUNT(*) AS records FROM v_cdr")).To(BeEmpty())
			facts = factsFor("What is the earliest call in the data?", "v_cdr")
			Expect(unmetKinds(facts, "SELECT MIN(call_start) AS earliest_call FROM v_cdr")).To(BeEmpty())
		})

		It("flags a list where one value was asked for", func() {
			facts := factsFor("How many calls are there?", "v_cdr")
			Expect(govSQLCheckShape(facts, &govSQLResult{Rows: [][]any{{"a", 1}, {"b", 2}}})).NotTo(BeNil())
			Expect(govSQLCheckShape(facts, &govSQLResult{Rows: [][]any{{5005}}})).To(BeNil())
			listFacts := factsFor("Which number made the most calls?", "v_cdr")
			Expect(govSQLCheckShape(listFacts, &govSQLResult{Rows: [][]any{{"a", 1}, {"b", 2}}})).To(BeNil())
		})
	})
})
