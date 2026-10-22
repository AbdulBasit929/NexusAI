package main

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Runtime parameters (link analysis stage 3, Amendment 2): the number an analyst asks for is read by code, checked against its bounds and set as a transaction setting; the model never
// chooses it. These specs need no database and no model.
var _ = Describe("Ask engine runtime parameters", func() {
	read := func(question string) map[string]askParamRead {
		return askReadParams(strings.ToLower(question))
	}
	frameOf := func(question string) askFrame {
		return askExtractFrame(question)
	}
	windowViews := []string{"v_colocation_windows"}
	meetingViews := []string{"v_meeting_candidates"}

	DescribeTable("reads the window from the question, whatever the number",
		func(question string, want int) {
			got := read(question)["colocation_minutes"]
			Expect(got.Found).To(BeTrue(), question)
			Expect(got.Ambiguous).To(BeFalse(), question)
			Expect(got.Value).To(Equal(want), question)
		},
		Entry("15", "When were 923001110001 and 923001110002 at the same cell within 15 minutes of each other?", 15),
		Entry("20", "When were 923001110001 and 923001110002 at the same cell within 20 minutes of each other?", 20),
		Entry("22", "At which cell sites were 923001110001 and 923001110002 together within 22 minutes?", 22),
		Entry("hyphen", "Show the 5-minute windows of 923001110001 and 923001110002 at one cell", 5),
		Entry("abbreviation", "Same cell, at most 45 mins apart: 923001110001 and 923001110002", 45),
		Entry("number word", "Were 923001110001 and 923001110002 at the same cell within twenty minutes?", 20),
		Entry("half an hour", "Were 923001110001 and 923001110002 together at one cell within half an hour?", 30),
		Entry("quarter of an hour", "Were 923001110001 and 923001110002 together at one cell within a quarter of an hour?", 15),
	)

	It("tells the window from the gap before it in a meeting question", func() {
		q := "List the possible meetings of 923001110001 and 923001110002: windows at the same cell within 20 minutes, preceded by a call or SMS between them in the previous 30 minutes."
		got := read(q)
		Expect(got["colocation_minutes"].Value).To(Equal(20))
		Expect(got["contact_gap_minutes"].Value).To(Equal(30))
		Expect(got["colocation_minutes"].Ambiguous).To(BeFalse())
		Expect(got["contact_gap_minutes"].Ambiguous).To(BeFalse())
	})

	It("reads a gap written after the number, and the previous hour as 60", func() {
		Expect(read("meetings at one cell within 10 minutes, after a call 25 minutes before the window")["contact_gap_minutes"].Value).To(Equal(25))
		Expect(read("Which windows of 923001110001 and 923001110002 within 15 minutes follow a call in the previous hour?")["contact_gap_minutes"].Value).To(Equal(60))
		Expect(read("meetings within 15 minutes with a call half an hour before")["contact_gap_minutes"].Value).To(Equal(30))
	})

	It("finds nothing when the question names no minutes, and uses the defaults", func() {
		q := "Did 923001110001 and 923001110002 meet?"
		Expect(read(q)["colocation_minutes"].Found).To(BeFalse())
		Expect(read(q)["contact_gap_minutes"].Found).To(BeFalse())
		uses, issue := askResolveParams(frameOf(q), meetingViews)
		Expect(issue).To(BeNil())
		Expect(uses).To(HaveLen(2))
		values := map[string]int{}
		for _, u := range uses {
			Expect(u.FromQuestion).To(BeFalse())
			values[u.Param.Name] = u.Value
		}
		Expect(values).To(Equal(map[string]int{"colocation_minutes": 15, "contact_gap_minutes": 60}))
	})

	It("sets the value the question names, as a transaction setting, and says where it came from", func() {
		uses, issue := askResolveParams(frameOf("When were 923001110001 and 923001110002 at the same cell within 22 minutes of each other?"), windowViews)
		Expect(issue).To(BeNil())
		Expect(askParamSettings(uses)).To(Equal(map[string]string{"app.colocation_minutes": "22"}))
		notes := strings.Join(askParamNotes(uses), " ")
		Expect(notes).To(ContainSubstring("22 minutes"))
		Expect(notes).To(ContainSubstring("read from your question"))
		defaults, _ := askResolveParams(frameOf("Did 923001110001 and 923001110002 meet?"), windowViews)
		Expect(strings.Join(askParamNotes(defaults), " ")).To(ContainSubstring("the default"))
	})

	It("asks instead of guessing when the question names two windows, or minutes with no number", func() {
		_, issue := askResolveParams(frameOf("When were 923001110001 and 923001110002 at the same cell within 10 minutes, or 20 minutes, of each other?"), windowViews)
		Expect(issue).NotTo(BeNil())
		Expect(issue.Kind).To(Equal("clarify"))
		Expect(issue.Message).To(ContainSubstring("10 and 20"))
		_, issue = askResolveParams(frameOf("When were 923001110001 and 923001110002 at the same cell within a few minutes of each other?"), windowViews)
		Expect(issue).NotTo(BeNil())
		Expect(issue.Kind).To(Equal("clarify"))
		Expect(issue.Message).To(ContainSubstring("without saying how many"))
	})

	It("refuses a value outside the bounds, with the bounds", func() {
		for _, q := range []string{
			"When were 923001110001 and 923001110002 at the same cell within 400 minutes of each other?",
			"When were 923001110001 and 923001110002 at the same cell within 0 minutes of each other?",
		} {
			_, issue := askResolveParams(frameOf(q), windowViews)
			Expect(issue).NotTo(BeNil(), q)
			Expect(issue.Kind).To(Equal("range"))
			Expect(issue.Message).To(ContainSubstring("1 to 180"))
		}
	})

	It("does nothing for a question whose views read no parameter", func() {
		uses, issue := askResolveParams(frameOf("What was the longest call of 923001110001 in minutes? It lasted 40 minutes at most."), []string{"v_comm_events"})
		Expect(issue).To(BeNil())
		Expect(uses).To(BeEmpty())
		Expect(askParamSettings(uses)).To(BeEmpty())
	})

	It("does not take a length of time for the N of a top list", func() {
		Expect(frameOf("List the periods in which 923001110001 and 923001110002 were at the same cell site, at most 22 minutes apart.").Limit).To(Equal(0))
		Expect(frameOf("Which calls of 923001110001 were longest 30 minutes?").Limit).To(Equal(0))
		Expect(frameOf("Which are the top 5 numbers 923001110001 called?").Limit).To(Equal(5))
		Expect(frameOf("Show the 10 most frequent contacts of 923001110001").Limit).To(Equal(10))
	})

	It("owns only the settings it declares", func() {
		Expect(askKnownSetting("app.colocation_minutes")).To(BeTrue())
		Expect(askKnownSetting("app.contact_gap_minutes")).To(BeTrue())
		Expect(askKnownSetting("app.tenant_id")).To(BeFalse())
		Expect(askKnownSetting("app.collection_id")).To(BeFalse())
		Expect(askKnownSetting("role")).To(BeFalse())
	})

	It("reads the parameters of every view the registry names from the catalogue's own views", func() {
		cat, err := askGoldenCatalogue()
		Expect(err).NotTo(HaveOccurred())
		for _, p := range askParams {
			for _, v := range p.Views {
				Expect(cat.View(v)).NotTo(BeNil(), "%s reads %s, which the catalogue must list", v, p.Setting)
			}
		}
	})
})
