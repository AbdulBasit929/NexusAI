package main

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
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

	// Found by the live smoke of 2026-10-30 (SUB-002): the registrations joined to the device pairings on a nonsense key found nothing, and the engine said that no registration of the SIM identity
	// was found although the registrations hold it.
	It("does not read an empty result as the absence of a registration when the query also reads a view the question does not ask about", func() {
		cat, err := askGoldenCatalogue()
		Expect(err).NotTo(HaveOccurred())
		saved := askProbeFn
		DeferCleanup(func() { askProbeFn = saved })
		askProbeFn = func(context.Context, *pgxpool.Pool, string, string, *askCatalogue, string, askLimits) ([]string, error) {
			return []string{"v_subscriber_registrations.imsi", "v_comm_events.src"}, nil
		}
		f := askExtractFrame("What numbers does IMSI 001010000001097 appear with in the registrations?")
		Expect(f.Identifiers()).NotTo(BeEmpty())
		joined, rej := askValidate("SELECT DISTINCT v_subscriber_registrations.msisdn FROM v_subscriber_registrations JOIN v_device_pairings ON v_subscriber_registrations.msisdn = v_device_pairings.src WHERE v_subscriber_registrations.imsi = '001010000001097' AND v_device_pairings.imei IN ('001010000001097') LIMIT 200", cat, []string{"v_device_pairings", "v_subscriber_registrations"})
		Expect(rej).To(BeNil())
		verdict, err := askReadEmpty(context.Background(), nil, hybridQueryRequest{}, cat, f, joined, askLimits{})
		Expect(err).NotTo(HaveOccurred())
		Expect(verdict.elsewhere).To(Equal("v_subscriber_registrations.imsi"))
		Expect(verdict.hint).To(ContainSubstring("v_device_pairings"))
		Expect(verdict.hint).To(ContainSubstring("v_subscriber_registrations alone"))
		Expect(verdict.reason).To(ContainSubstring("joined views the question does not ask about"))
		Expect(verdict.absence).To(BeEmpty())
		// the registrations alone, narrowed by a condition the question did not state, are not concerned: that empty result is the query's own
		alone, rej := askValidate("SELECT msisdn FROM v_subscriber_registrations WHERE imsi = '001010000001097' AND status = 'deactivated' LIMIT 200", cat, []string{"v_subscriber_registrations"})
		Expect(rej).To(BeNil())
		verdict, err = askReadEmpty(context.Background(), nil, hybridQueryRequest{}, cat, f, alone, askLimits{})
		Expect(err).NotTo(HaveOccurred())
		Expect(verdict.elsewhere).To(BeEmpty())
		Expect(verdict.absence).To(BeEmpty())
		Expect(askViewsOutside(joined, askFamilyViews(joined))).To(Equal([]string{"v_device_pairings"}))
		Expect(askHitViews([]string{"v_subscriber_registrations.imsi", "v_subscriber_registrations.msisdn", "v_comm_events.src"})).To(Equal([]string{"v_subscriber_registrations", "v_comm_events"}))
	})

	// Found by the live smoke of 2026-10-30 (SUB-008): a count of the deactivated registrations answered "how many numbers were still in use after their registration ended" with 5 where 3 were right.
	It("asks for the events of the numbers when a question about use after a registration ended is answered from the registrations alone", func() {
		cat, err := askGoldenCatalogue()
		Expect(err).NotTo(HaveOccurred())
		q := "How many numbers were still in use after their registration ended?"
		check := func(sql string, views ...string) []string {
			v, rej := askValidate(sql, cat, views)
			Expect(rej).To(BeNil(), sql)
			out := []string{}
			for _, o := range askCheckFrame(askExtractFrame(q), v) {
				out = append(out, o.Kind)
			}
			return out
		}
		Expect(check("SELECT count(*) AS count FROM v_subscriber_registrations WHERE valid_to IS NOT NULL AND status = 'deactivated'", "v_subscriber_registrations")).To(ContainElement("REGISTRATION_END"))
		Expect(check("SELECT count(*) AS count FROM v_number_registration_activity WHERE events_after_last_end > 0", "v_number_registration_activity")).NotTo(ContainElement("REGISTRATION_END"))
		// a question that is not about the end of a registration is left alone
		plain, rej := askValidate("SELECT count(*) AS count FROM v_subscriber_registrations WHERE status = 'deactivated'", cat, []string{"v_subscriber_registrations"})
		Expect(rej).To(BeNil())
		for _, o := range askCheckFrame(askExtractFrame("How many registrations are deactivated?"), plain) {
			Expect(o.Kind).NotTo(Equal("REGISTRATION_END"))
		}
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

	// Found by the subscriber judgement run on case 121 (2026-10-29): the strong word registered at 5 points crowded the call records out of a question about events after a registration ended, and
	// the words "no registration file mentions" were taken for a question about documents.
	It("keeps the call records in a question about events after a registration, the documents out of a question about registrations, and finds the handsets by their maker", func() {
		cat, err := askGoldenCatalogue()
		Expect(err).NotTo(HaveOccurred())
		Expect(cat.Shortlist("Which call events of 923000001043 came after the end of a registration of the number?", 0).Views).To(ContainElement("v_comm_events"))
		for _, view := range askDocumentViews {
			Expect(cat.Shortlist("Which numbers does no registration file mention?", 0).Views).NotTo(ContainElement(view))
		}
		Expect(cat.Shortlist("Which handsets were made by Samsung?", 0).Views).To(ContainElement("v_handsets"))
		Expect(cat.Shortlist("Which numbers used a phone manufactured by Nokia?", 0).Views).To(ContainElement("v_handsets"))
	})

	// Found by the subscriber run on case 124 (2026-10-29): "which numbers used a SYNTH handset" answered with a count.
	It("asks for the rows when a question asks which ones, and leaves a count to a question that asks how many", func() {
		cat, err := askGoldenCatalogue()
		Expect(err).NotTo(HaveOccurred())
		kinds := func(question, sql string, views ...string) []string {
			v, rej := askValidate(sql, cat, views)
			Expect(rej).To(BeNil(), sql)
			out := []string{}
			for _, o := range askCheckFrame(askExtractFrame(question), v) {
				out = append(out, o.Kind)
			}
			return out
		}
		counted := "SELECT count(DISTINCT src) AS n FROM v_handsets WHERE make = 'SYNTH'"
		listed := "SELECT DISTINCT src FROM v_handsets WHERE make = 'SYNTH' ORDER BY src"
		Expect(kinds("Which numbers used a SYNTH handset?", counted, "v_handsets")).To(ContainElement("LIST"))
		Expect(kinds("Which numbers used a SYNTH handset?", listed, "v_handsets")).NotTo(ContainElement("LIST"))
		Expect(kinds("How many numbers used a SYNTH handset?", counted, "v_handsets")).NotTo(ContainElement("LIST"))
		Expect(kinds("Which numbers used more than two handsets?", "SELECT src FROM v_handsets GROUP BY src HAVING count(DISTINCT imei) > 2", "v_handsets")).NotTo(ContainElement("LIST"))
	})

	// Found by the subscriber run on case 122 (2026-10-29): imei = 'SYNTH' for "numbers seen with a SYNTH phone", and "no row matches".
	It("finds the handsets for a make written before the word phone, and refuses a name compared with an IMEI", func() {
		cat, err := askGoldenCatalogue()
		Expect(err).NotTo(HaveOccurred())
		Expect(cat.Shortlist("List the numbers seen with a SYNTH phone.", 0).Views).To(ContainElement("v_handsets"))
		Expect(cat.Shortlist("Which numbers used an Apple handset?", 0).Views).To(ContainElement("v_handsets"))
		Expect(cat.Shortlist("Which numbers used a different phone after March?", 0).Views).NotTo(ContainElement("v_handsets"))
		kinds := func(question, sql string, views ...string) []string {
			v, rej := askValidate(sql, cat, views)
			Expect(rej).To(BeNil(), sql)
			out := []string{}
			for _, o := range askCheckFrame(askExtractFrame(question), v) {
				out = append(out, o.Kind)
			}
			return out
		}
		q := "List the numbers seen with a SYNTH phone."
		Expect(kinds(q, "SELECT DISTINCT src FROM v_device_pairings WHERE imei = 'SYNTH' ORDER BY src", "v_device_pairings")).To(ContainElement("IMEI_FORM"))
		Expect(kinds(q, "SELECT DISTINCT src FROM v_handsets WHERE make = 'SYNTH' ORDER BY src", "v_handsets")).NotTo(ContainElement("IMEI_FORM"))
		Expect(kinds("Which numbers used the handset 000000000010090?", "SELECT DISTINCT src FROM v_handsets WHERE imei = '000000000010090'", "v_handsets")).NotTo(ContainElement("IMEI_FORM"))
	})

	// Found by the subscriber run on development case 2 (2026-10-29): a comparison of valid_to with the last event, worked out by the query, lists numbers whose later registration is running.
	It("asks for events_after_last_end when a question is about events after a registration ended", func() {
		cat, err := askGoldenCatalogue()
		Expect(err).NotTo(HaveOccurred())
		check := func(question, sql string, views ...string) []string {
			v, rej := askValidate(sql, cat, views)
			Expect(rej).To(BeNil(), sql)
			out := []string{}
			for _, o := range askCheckFrame(askExtractFrame(question), v) {
				out = append(out, o.Kind)
			}
			return out
		}
		q := "Which numbers were still in use after their registration ended?"
		own := "SELECT DISTINCT a.msisdn FROM v_subscriber_registrations a JOIN v_number_registration_activity b ON a.msisdn = b.msisdn WHERE a.valid_to IS NOT NULL AND a.valid_to < b.last_event_utc LIMIT 200"
		counted := "SELECT msisdn, events_after_last_end FROM v_number_registration_activity WHERE events_after_last_end > 0 ORDER BY events_after_last_end DESC LIMIT 200"
		Expect(check(q, own, "v_subscriber_registrations", "v_number_registration_activity")).To(ContainElement("REGISTRATION_END"))
		Expect(check(q, counted, "v_number_registration_activity")).NotTo(ContainElement("REGISTRATION_END"))
		Expect(check("Which numbers have a registration that conflicts with another source?", "SELECT msisdn FROM v_number_registration_activity WHERE conflicts LIMIT 200", "v_number_registration_activity")).NotTo(ContainElement("REGISTRATION_END"))
	})

	// Found by the subscriber run on development case 2 (2026-10-29): the activity view lists numbers that HAVE a registration, so "no registration" was answered from it.
	It("shortlists the call records and the registrations for the numbers that have no registration, and leaves the activity view out of them", func() {
		cat, err := askGoldenCatalogue()
		Expect(err).NotTo(HaveOccurred())
		views := cat.Shortlist("Which numbers of the call records have no registration?", 0).Views
		Expect(views).To(ContainElements("v_comm_events", "v_subscriber_registrations"))
		Expect(views).NotTo(ContainElement("v_number_registration_activity"))
		Expect(cat.Shortlist("Which numbers were still in use after their registration ended?", 0).Views).To(ContainElement("v_number_registration_activity"))
	})
})
