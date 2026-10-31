package main

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Runtime parameters of stages 3b and 3c (A9 successor numbers, A10 movement, A11 change of rhythm): the number an analyst gives is read by code, checked against its bounds and set as a
// transaction setting; the model never chooses it. No database and no model.
var _ = Describe("Ask engine runtime parameters of successor numbers, movement and rhythm", func() {
	value := func(question, name string) (int, bool) {
		read := askReadParams(strings.ToLower(question))[name]
		if read.Ambiguous {
			Fail("ambiguous: " + question)
		}
		return read.Value, read.Found
	}

	DescribeTable("reads the number of days of silence",
		func(question string, want int) {
			got, found := value(question, "silence_days")
			Expect(found).To(BeTrue(), question)
			Expect(got).To(Equal(want), question)
		},
		Entry("days", "Which numbers went silent for 10 days before the end of the case?", 10),
		Entry("at least", "Which numbers have been silent for at least 12 days?", 12),
		Entry("weeks", "Which numbers were inactive for 2 weeks?", 14),
		Entry("no activity", "Which numbers had no activity for 21 days?", 21),
		Entry("noun form", "Which numbers show 5 days of silence?", 5),
	)

	It("reads a number of more than three digits, so that a value far outside the bounds is refused with them", func() {
		for _, c := range []struct {
			question, name string
			want           int
		}{
			{"Which numbers went silent for at least 4000 days?", "silence_days", 4000},
			{"Which numbers moved, staying at the new place for at least 600 weeks?", "anchor_hold_weeks", 600},
			{"Which events of 923001110001 imply travel faster than 90000 km/h?", "speed_kmh", 90000},
			{"Did the daily activity of 923001110001 change, with at least 4000 days on each side?", "rhythm_min_side_days", 4000},
		} {
			got, found := value(c.question, c.name)
			Expect(found).To(BeTrue(), c.question)
			Expect(got).To(Equal(c.want), c.question)
		}
		_, issue := askResolveParams(askExtractFrame("Which numbers went silent for at least 4000 days?"), []string{"v_silent_numbers"})
		Expect(issue).NotTo(BeNil())
		Expect(issue.Kind).To(Equal("range"))
		Expect(issue.Message).To(ContainSubstring("1 to 365"))
	})

	It("asks back when a question gives two lengths for one rule", func() {
		read := askReadParams("which numbers went silent for at least 10 days or for 20 days?")["silence_days"]
		Expect(read.Ambiguous).To(BeTrue())
		Expect(read.Values).To(Equal([]int{10, 20}))
		_, issue := askResolveParams(askExtractFrame("Which numbers went silent for at least 10 days or for 20 days?"), []string{"v_silent_numbers"})
		Expect(issue).NotTo(BeNil())
		Expect(issue.Kind).To(Equal("clarify"))
		// one length is still one length, and "or" before something that is not a length changes nothing
		Expect(askReadParams("which numbers went silent for at least 10 days or never came back?")["silence_days"].Values).To(Equal([]int{10}))
	})

	It("keeps the silence and the start window apart in one question", func() {
		q := "Which new number may have replaced 923001110001 if it went silent for 10 days and the new number started within 5 days after it stopped?"
		silence, ok1 := value(q, "silence_days")
		window, ok2 := value(q, "successor_window_days")
		Expect(ok1).To(BeTrue())
		Expect(ok2).To(BeTrue())
		Expect(silence).To(Equal(10))
		Expect(window).To(Equal(5))
	})

	It("reads the start window of the new number, in days or weeks", func() {
		got, ok := value("Who took over from 923001110001, a number that started within 3 weeks of it going silent?", "successor_window_days")
		Expect(ok).To(BeTrue())
		Expect(got).To(Equal(21))
		got, ok = value("Which numbers started within 7 days after 923001110001 stopped?", "successor_window_days")
		Expect(ok).To(BeTrue())
		Expect(got).To(Equal(7))
	})

	DescribeTable("reads the lowest similarity score as a percent",
		func(question string, want int) {
			got, found := value(question, "successor_score_percent")
			Expect(found).To(BeTrue(), question)
			Expect(got).To(Equal(want), question)
		},
		Entry("fraction", "Which numbers may have replaced 923001110001, with a score of at least 0.5?", 50),
		Entry("short fraction", "Which numbers may have replaced 923001110001, with a score above .2?", 20),
		Entry("percent", "Successors of 923001110001 with a score of at least 60%", 60),
		Entry("whole number", "Successors of 923001110001 with a score of at least 40", 40),
		Entry("similar", "Numbers 70% similar to 923001110001 that took over", 70),
	)

	It("reads the rules of the anchors", func() {
		hold, ok := value("Did 923001110001 move house, a new place that held for at least 3 weeks?", "anchor_hold_weeks")
		Expect(ok).To(BeTrue())
		Expect(hold).To(Equal(3))
		hold, ok = value("Where does 923001110001 sleep, counting a place only when it holds for 4 consecutive weeks?", "anchor_hold_weeks")
		Expect(ok).To(BeTrue())
		Expect(hold).To(Equal(4))
		events, ok := value("Where did 923001110001 sleep, counting weeks with at least 5 events at night?", "anchor_min_events")
		Expect(ok).To(BeTrue())
		Expect(events).To(Equal(5))
	})

	DescribeTable("reads the speed limit",
		func(question string, want int) {
			got, found := value(question, "speed_kmh")
			Expect(found).To(BeTrue(), question)
			Expect(got).To(Equal(want), question)
		},
		Entry("km/h", "Which consecutive events of 923001110001 imply more than 150 km/h?", 150),
		Entry("kph", "List the moves of 923001110001 faster than 300 kph", 300),
		Entry("kmh", "Any jump of 923001110001 over 120 kmh?", 120),
		Entry("per hour", "Events 250 kilometres per hour apart", 250),
	)

	DescribeTable("reads how large a change of rhythm to look for",
		func(question string, want int) {
			got, found := value(question, "rhythm_ratio_percent")
			Expect(found).To(BeTrue(), question)
			Expect(got).To(Equal(want), question)
		},
		Entry("double", "Did the activity of 923001110001 at least double?", 200),
		Entry("twice", "Did 923001110001 become twice as active?", 200),
		Entry("triple", "Did the daily calls of 923001110001 triple?", 300),
		Entry("times", "Did the activity change by 1.5 times?", 150),
	)

	It("reads the days that must be on each side of a change of rhythm", func() {
		got, ok := value("Did the activity of 923001110001 change, with at least 14 days on each side?", "rhythm_min_side_days")
		Expect(ok).To(BeTrue())
		Expect(got).To(Equal(14))
	})

	It("finds nothing in a question that names none of them, and uses the defaults", func() {
		for _, name := range []string{"silence_days", "successor_window_days", "successor_score_percent", "anchor_hold_weeks", "speed_kmh", "rhythm_ratio_percent", "rhythm_min_side_days"} {
			_, found := value("Which numbers went silent and who replaced them?", name)
			Expect(found).To(BeFalse(), name)
		}
		uses, issue := askResolveParams(askExtractFrame("Which impossible moves did 923001110001 make?"), []string{"v_speed_alerts"})
		Expect(issue).To(BeNil())
		Expect(uses).To(HaveLen(1))
		Expect(uses[0].Value).To(Equal(200))
		Expect(uses[0].FromQuestion).To(BeFalse())
	})

	It("sets the settings the views read, with the question's numbers and the defaults for the rest", func() {
		q := "Which numbers took over from numbers silent for 10 days, with a score of at least 0.5?"
		uses, issue := askResolveParams(askExtractFrame(q), []string{"v_successor_candidates"})
		Expect(issue).To(BeNil())
		settings := askParamSettings(uses)
		Expect(settings).To(HaveKeyWithValue("app.silence_days", "10"))
		Expect(settings).To(HaveKeyWithValue("app.successor_score_percent", "50"))
		Expect(settings).To(HaveKeyWithValue("app.successor_window_days", "14"))
		notes := strings.Join(askParamNotes(uses), " ")
		Expect(notes).To(ContainSubstring("Days of silence used: 10 days (read from your question)"))
		Expect(notes).To(ContainSubstring("Lowest similarity score listed used: 50 percent (read from your question)"))
		Expect(notes).NotTo(ContainSubstring("Days of contacts and places compared"), "a rule of the definition is named only when the question gave it")
	})

	It("asks back when a question names two different speeds, and refuses values outside the bounds", func() {
		_, issue := askResolveParams(askExtractFrame("Moves of 923001110001 faster than 150 km/h or 300 km/h?"), []string{"v_speed_alerts"})
		Expect(issue).NotTo(BeNil())
		Expect(issue.Kind).To(Equal("clarify"))
		_, issue = askResolveParams(askExtractFrame("Moves of 923001110001 faster than 5000 km/h?"), []string{"v_speed_alerts"})
		Expect(issue).NotTo(BeNil())
		Expect(issue.Kind).To(Equal("range"))
		Expect(issue.Message).To(ContainSubstring("1 to 2000"))
		_, issue = askResolveParams(askExtractFrame("Which numbers were silent for 900 days?"), []string{"v_silent_numbers"})
		Expect(issue).NotTo(BeNil())
		Expect(issue.Message).To(ContainSubstring("1 to 365"))
	})

	It("does not read a parameter of the minute views as one of these", func() {
		q := "When were 923001110001 and 923001110002 at the same cell within 20 minutes of each other?"
		Expect(askReadParams(strings.ToLower(q))["speed_kmh"].Found).To(BeFalse())
		Expect(askReadParams(strings.ToLower(q))["silence_days"].Found).To(BeFalse())
		Expect(askReadParams(strings.ToLower(q))["colocation_minutes"].Value).To(Equal(20))
	})

	It("owns the settings of every declared parameter and no others", func() {
		for _, p := range askParams {
			Expect(askKnownSetting(p.Setting)).To(BeTrue(), p.Name)
			if !p.Required {
				// a required parameter has no default: the question must give the value
				Expect(p.Min).To(BeNumerically("<=", p.Default), p.Name)
				Expect(p.Default).To(BeNumerically("<=", p.Max), p.Name)
			}
			Expect(p.Views).NotTo(BeEmpty(), p.Name)
		}
		// the hub threshold is a declared parameter since migration 032 (the question's number, else the case's setting, else the engine's): the executor owns its setting like any other
		Expect(askKnownSetting("app.hub_threshold")).To(BeTrue())
		Expect(askKnownSetting("app.tenant_id")).To(BeFalse())
		Expect(askKnownSetting("app.collection_id")).To(BeFalse())
	})
})
