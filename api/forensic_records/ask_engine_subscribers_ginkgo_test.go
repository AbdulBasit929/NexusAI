package main

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Registrations and handsets (docs/data/subscriber-device-family-20261028.md, piece g): SIM serials and TACs are identifiers of the question, a registration is never stated as who holds a number,
// a handset's model is "listed as", and the coverage guard declines the views for a case that has no registrations.
var _ = Describe("Ask engine registrations and handsets", func() {
	DescribeTable("finds a SIM serial or a TAC in a question only where one is written",
		func(question string, want ...string) {
			f := askExtractFrame(question)
			if len(want) == 0 {
				Expect(f.Serials).To(BeEmpty(), question)
				return
			}
			Expect(f.Serials).To(Equal(want), question)
		},
		Entry("an ICCID of 19 digits", "Which numbers were registered on the SIM with ICCID 8900000000000010002?", "8900000000000010002"),
		Entry("an ICCID of 20 digits", "Is the SIM 89000000000000100021 registered to more than one number?", "89000000000000100021"),
		Entry("a TAC after the word", "Which handsets have TAC 00290611?", "00290611"),
		Entry("a TAC after the word and a colon", "Show the handsets with tac: 00290611", "00290611"),
		Entry("a TAC after 'code'", "Which numbers use a phone with TAC code 00290611?", "00290611"),
		Entry("eight digits without the word are not a TAC", "Show the events for 00290611"),
		Entry("a phone number is not a serial", "Which SIM was 923001110001 registered on?"),
		Entry("an IMSI (15 digits) is an identifier like an IMEI, not a serial", "Which numbers are registered on IMSI 001010000001000?"),
		Entry("the same serial twice is one", "Was 8900000000000010002 swapped? Is 8900000000000010002 active?", "8900000000000010002"))

	It("names the serial among the identifiers, so that the probe and the audit see it, and the IMSI is one of the IMEIs list", func() {
		f := askExtractFrame("Which numbers were registered on the SIM with ICCID 8900000000000010002?")
		Expect(f.Identifiers()).To(ContainElement("8900000000000010002"))
		Expect(f.Question).To(ContainSubstring("8900000000000010002"))
		g := askExtractFrame("Which numbers are registered on IMSI 001010000001000?")
		Expect(g.Identifiers()).To(ContainElement("001010000001000"))
		Expect(g.Serials).To(BeEmpty())
	})

	Describe("the query's duty to the serial of the question", func() {
		var cat *askCatalogue
		BeforeEach(func() {
			var err error
			cat, err = askGoldenCatalogue()
			Expect(err).NotTo(HaveOccurred())
		})
		check := func(question, sql string, views ...string) []askObligation {
			v, rej := askValidate(sql, cat, views)
			Expect(rej).To(BeNil(), sql)
			return askCheckFrame(askExtractFrame(question), v)
		}
		kinds := func(list []askObligation) []string {
			out := []string{}
			for _, o := range list {
				out = append(out, o.Kind)
			}
			return out
		}
		q := "Which numbers were registered on the SIM with ICCID 8900000000000010002?"

		It("accepts a query that compares the serial exactly", func() {
			Expect(check(q, "SELECT DISTINCT msisdn FROM v_subscriber_registrations WHERE iccid = '8900000000000010002' ORDER BY msisdn LIMIT 200", "v_subscriber_registrations")).To(BeEmpty())
		})
		It("asks for the serial when the query leaves it out, and refuses a serial the question did not name", func() {
			Expect(kinds(check(q, "SELECT DISTINCT msisdn FROM v_subscriber_registrations ORDER BY msisdn LIMIT 200", "v_subscriber_registrations"))).To(ContainElement("IDENTIFIER"))
			unmet := check(q, "SELECT DISTINCT msisdn FROM v_subscriber_registrations WHERE iccid IN ('8900000000000010002', '8900000000000010010') LIMIT 200", "v_subscriber_registrations")
			Expect(kinds(unmet)).To(ContainElement("FOREIGN_IDENTIFIER"))
		})
		It("lets a question about a number be answered from its registrations without a serial duty", func() {
			Expect(check("Which SIM was 923001110001 registered on?",
				"SELECT imsi, iccid, status, valid_from, valid_to FROM v_subscriber_registrations WHERE msisdn = '923001110001' ORDER BY valid_from NULLS FIRST LIMIT 200", "v_subscriber_registrations")).To(BeEmpty())
		})
		It("asks for the date of the question to be applied to the registration period", func() {
			unmet := check("Was 923001110001 registered and active on 2026-03-05?", "SELECT status FROM v_subscriber_registrations WHERE msisdn = '923001110001' LIMIT 200", "v_subscriber_registrations")
			Expect(kinds(unmet)).To(ContainElement("DATE"))
		})
		It("asks for both bounds of a whole day when a registration is said to be in force on a date (found by the development check on case 2)", func() {
			day := "Which registrations of the number 923001110001 were valid on 2026-02-28 (local time)? Show the status."
			instant := "SELECT msisdn, status, valid_from, valid_to FROM v_subscriber_registrations WHERE msisdn = '923001110001' AND (valid_from IS NULL OR valid_from <= '2026-02-28 00:00:00+05') AND (valid_to IS NULL OR valid_to > '2026-02-28 00:00:00+05') LIMIT 200"
			unmet := check(day, instant, "v_subscriber_registrations")
			Expect(kinds(unmet)).To(ConsistOf("DAY_COVERAGE"))
			Expect(unmet[0].Message).To(ContainSubstring("< '2026-03-01 00:00:00+05'"))
			whole := "SELECT msisdn, status, valid_from, valid_to FROM v_subscriber_registrations WHERE msisdn = '923001110001' AND (valid_from IS NULL OR valid_from < '2026-03-01 00:00:00+05') AND (valid_to IS NULL OR valid_to > '2026-02-28 00:00:00+05') LIMIT 200"
			Expect(check(day, whole, "v_subscriber_registrations")).To(BeEmpty())
			// a question that is not about being in force on a day is left alone, and so is a query that does not read the validity
			Expect(kinds(check("Which SIM was 923001110001 registered on 2026-02-28?", "SELECT imsi FROM v_subscriber_registrations WHERE msisdn = '923001110001' AND valid_from >= '2026-02-28 00:00:00+05' LIMIT 200", "v_subscriber_registrations"))).NotTo(ContainElement("DAY_COVERAGE"))
		})
	})

	It("keeps a registration or handset question to the views of its own family when the probe reads an empty result", func() {
		cat, err := askGoldenCatalogue()
		Expect(err).NotTo(HaveOccurred())
		reg, rej := askValidate("SELECT * FROM v_subscriber_registrations WHERE msisdn = '923000001006' ORDER BY valid_from DESC", cat, []string{"v_subscriber_registrations"})
		Expect(rej).To(BeNil())
		Expect(askFamilyViews(reg)).To(Equal([]string{"v_subscriber_registrations", "v_number_registration_activity"}))
		// the number occurs in the call records and nowhere among the registrations: no hit of the family, so the empty result is the answer (unregistered), not a wrong place
		Expect(askHitsIn([]string{"v_comm_events.src", "v_device_pairings.src"}, askFamilyViews(reg))).To(BeEmpty())
		Expect(askHitsIn([]string{"v_subscriber_registrations.imsi", "v_comm_events.src"}, askFamilyViews(reg))).To(Equal([]string{"v_subscriber_registrations.imsi"}))
		hand, rej := askValidate("SELECT make, model FROM v_handsets WHERE src = '923000001006' LIMIT 200", cat, []string{"v_handsets"})
		Expect(rej).To(BeNil())
		Expect(askFamilyViews(hand)).To(Equal([]string{"v_handsets", "v_device_pairings"}))
		calls, rej := askValidate("SELECT count(*) AS n FROM v_comm_events WHERE src = '923000001006'", cat, []string{"v_comm_events"})
		Expect(rej).To(BeNil())
		Expect(askFamilyViews(calls)).To(BeNil())
		Expect(askHitsIn([]string{"nodot"}, []string{"v_handsets"})).To(BeEmpty())
	})

	It("answers who owns a number from the registrations, with the correction in front", func() {
		cat, err := askGoldenCatalogue()
		Expect(err).NotTo(HaveOccurred())
		f := askExtractFrame("Who owns the number 923001110001?")
		Expect(cat.Shortlist(f.Question, 0).Views).To(ContainElement("v_subscriber_registrations"))
		reg, rej := askValidate("SELECT imsi, iccid, status, valid_from, valid_to FROM v_subscriber_registrations WHERE msisdn = '923001110001' LIMIT 200", cat, []string{"v_subscriber_registrations"})
		Expect(rej).To(BeNil())
		Expect(askPremiseLead(reg, f)).To(Equal("A registration records what one source file states, not who holds or uses the number."))
		Expect(askPremiseLead(reg, askExtractFrame("Which SIM was 923001110001 registered on?"))).To(Equal(""))
	})

	It("adds the caveats of the registration and handset views, which the model can neither omit nor reword", func() {
		Expect(askViewCaveatText).To(HaveKey("v_subscriber_registrations"))
		Expect(askViewCaveatText["v_subscriber_registrations"]).To(ContainSubstring("does not say who holds or uses the number"))
		Expect(askViewCaveatText["v_number_registration_activity"]).To(ContainSubstring("a pattern in the records, not a finding"))
		Expect(askViewCaveatText["v_handsets"]).To(ContainSubstring("a listing, not a fact about the individual handset"))
		for _, name := range []string{"v_subscriber_registrations", "v_number_registration_activity", "v_handsets"} {
			Expect(strings.ToLower(askViewCaveatText[name])).NotTo(ContainSubstring("owner"), name)
		}
		lines := askViewCaveats(&askValidated{Views: []string{"v_handsets", "v_subscriber_registrations"}})
		Expect(lines).To(HaveLen(2))
	})

	It("declines the registration views for a case with no typed registrations, and the handset view for a case with no call records", func() {
		Expect(askCoverageGap(askCoverage{AnyRegistration: false}, []string{"v_subscriber_registrations"})).To(ContainSubstring("no typed subscriber registrations"))
		Expect(askCoverageGap(askCoverage{AnyRegistration: false}, []string{"v_number_registration_activity"})).To(ContainSubstring("no typed subscriber registrations"))
		Expect(askCoverageGap(askCoverage{AnyRegistration: true}, []string{"v_subscriber_registrations", "v_number_registration_activity"})).To(Equal(""))
		Expect(askCoverageGap(askCoverage{AnyRegistration: true, Files: 3, Typed: 1}, []string{"v_subscriber_registrations"})).To(Equal(""))
		Expect(askCoverageGap(askCoverage{AnyComm: false}, []string{"v_handsets"})).To(ContainSubstring("no typed call records"))
		Expect(askCoverageGap(askCoverage{AnyComm: true, Files: 1, Typed: 1}, []string{"v_handsets"})).To(Equal(""))
	})

	It("probes the columns that can hold a registered number, a SIM serial or a TAC, and no description", func() {
		for _, name := range []string{"msisdn", "iccid", "tac"} {
			Expect(askProbeColumn(name)).To(BeTrue(), name)
		}
		for _, name := range []string{"status", "make", "model", "operator_label"} {
			Expect(askProbeColumn(name)).To(BeFalse(), name)
		}
	})
})

// Found by the subscriber judgement run on case 117 (2026-10-28): "made or received" asks for both ways, and the query cannot filter on both.
var _ = Describe("Ask engine direction cues that name both ways", func() {
	It("asks for neither filter when the question says made or received, and still asks for the one it names alone", func() {
		cat, err := askGoldenCatalogue()
		Expect(err).NotTo(HaveOccurred())
		check := func(question, sql string) []string {
			v, rej := askValidate(sql, cat, []string{"v_comm_events"})
			Expect(rej).To(BeNil(), sql)
			out := []string{}
			for _, o := range askCheckFrame(askExtractFrame(question), v) {
				out = append(out, o.Kind)
			}
			return out
		}
		both := "SELECT DISTINCT src FROM v_comm_events ORDER BY src"
		Expect(check("Which numbers made or received calls after their registration ended?", both)).NotTo(ContainElement("DIRECTION"))
		Expect(check("Which numbers made or received messages in March?", both)).NotTo(ContainElement("DIRECTION"))
		Expect(check("Which numbers received calls?", both)).To(ContainElement("DIRECTION"))
		Expect(check("Which numbers made calls?", both)).To(ContainElement("DIRECTION"))
	})
})

// Found by the subscriber judgement run on case 118 (2026-10-28): the words imsi and sim outweighed "registered", and "seen" brought the camera view into a question about phones.
var _ = Describe("Ask engine retrieval of registrations and phones", func() {
	It("shortlists the registrations for a question that says registered, and keeps the camera view out of a question about who was seen with a phone", func() {
		cat, err := askGoldenCatalogue()
		Expect(err).NotTo(HaveOccurred())
		Expect(cat.Shortlist("Which numbers were registered on the SIM with IMSI 001010000001018?", 0).Views).To(ContainElement("v_subscriber_registrations"))
		Expect(cat.Shortlist("List the numbers seen with a SYNTH phone.", 0).Views).NotTo(ContainElement("v_plate_sightings"))
		Expect(cat.Shortlist("At which cameras was the plate SYNT-4024 seen?", 0).Views).To(ContainElement("v_plate_sightings"))
		Expect(cat.Shortlist("Which calls did 923000001043 make in March?", 0).Views).NotTo(ContainElement("v_subscriber_registrations"))
	})
})
