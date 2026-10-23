package main

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The caveats of the stage-3 analyses (docs/data/link-analysis-stage3-preregistration-20261022.md section 6): the engine adds them, keyed by view, and the fixed phrases the gate
// looks for must be in them.
var _ = Describe("Ask engine caveats of the link-analysis views", func() {
	collapse := func(lines []string) string {
		return strings.Join(strings.Fields(strings.ToLower(strings.Join(lines, " "))), " ")
	}

	It("adds the co-location caveat, with the missing chance baseline said, for v_colocation_windows", func() {
		text := collapse(askViewCaveats(&askValidated{Views: []string{"v_colocation_windows"}}))
		Expect(text).To(ContainSubstring("proximity is not association"))
		Expect(text).To(ContainSubstring("no chance baseline is computed"))
	})

	It("adds the meeting caveat for v_meeting_candidates", func() {
		text := collapse(askViewCaveats(&askValidated{Views: []string{"v_meeting_candidates"}}))
		Expect(text).To(ContainSubstring("a candidate, not a finding"))
		Expect(text).To(ContainSubstring("proximity and contact are not proof of a meeting"))
	})

	It("adds both when a query reads both, and nothing for the other views", func() {
		Expect(askViewCaveats(&askValidated{Views: []string{"v_colocation_windows", "v_meeting_candidates"}})).To(HaveLen(2))
		Expect(askViewCaveats(&askValidated{Views: []string{"v_app_usage", "v_contact_pairs", "v_comm_events"}})).To(BeEmpty())
		cellVisits := askViewCaveats(&askValidated{Views: []string{"v_cell_visits"}})
		Expect(cellVisits).To(HaveLen(1))
		Expect(strings.ToLower(cellVisits[0])).To(ContainSubstring("cell sites, not positions of a person"))
		Expect(askViewCaveats(nil)).To(BeEmpty())
	})

	It("treats one row of NULLs (an aggregate over nothing) as no row, and a count of 0 as a row", func() {
		Expect(askRowAllNil([]any{nil})).To(BeTrue())
		Expect(askRowAllNil([]any{nil, nil})).To(BeTrue())
		Expect(askRowAllNil([]any{nil, 0})).To(BeFalse())
		Expect(askRowAllNil([]any{0})).To(BeFalse())
		Expect(askRowAllNil(nil)).To(BeFalse())
	})

	It("keeps a question about hours at the same place on the hourly view, and one about minutes on the windows view", func() {
		cat, err := askGoldenCatalogue()
		Expect(err).NotTo(HaveOccurred())
		for _, q := range []string{
			"How many different hours did 923001110005 and 923001110009 spend at the same cell site together?",
			"In how many hours were 923001110005 and 923001110009 at the same cell?",
		} {
			Expect(cat.Shortlist(q, 0).Views).NotTo(ContainElement("v_colocation_windows"), q)
			Expect(cat.Shortlist(q, 0).Views).To(ContainElement("v_cell_visits"), q)
		}
		Expect(cat.Shortlist("When were 923001110005 and 923001110009 at the same cell site together within 20 minutes of each other?", 0).Views).To(ConsistOf("v_colocation_windows"))
	})

	It("keeps a meeting question off the hourly view and a minutes question on the windows view", func() {
		cat, err := askGoldenCatalogue()
		Expect(err).NotTo(HaveOccurred())
		meeting := cat.Shortlist("Did 923001110001 and 923001110002 meet?", 3)
		Expect(meeting.Views).To(ConsistOf("v_meeting_candidates"))
		minutes := cat.Shortlist("When were 923001110001 and 923001110002 at the same place within 15 minutes of each other?", 3)
		Expect(minutes.Views).To(ConsistOf("v_colocation_windows"))
		hourly := cat.Shortlist("Which cell sites did 923001110001 and 923001110002 both use in the same hour?", 3)
		Expect(hourly.Views).To(ContainElement("v_cell_visits"))
		Expect(hourly.Views).NotTo(ContainElement("v_colocation_windows"))
		Expect(hourly.Views).NotTo(ContainElement("v_meeting_candidates"))
	})
})
