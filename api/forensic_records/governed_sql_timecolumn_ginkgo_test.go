package main

import (
	"crypto/sha256"
	"encoding/hex"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Run 11 (pre-registration) missed two gates, and neither miss was a clock error:
//
//	T1  "How many call records were there at night?" was answered 1,771 against a key of 1,768: the model
//	    applied "night" to call_start OR call_end, and the question named neither.
//	T3  a tower question the lane had answered was declined, 6 asks in 6, after rule 6 of the shared prompt
//	    was reworded for the case clock. That the reword caused it is suspected, not proven.
//
// These specs hold the two changes made for them: a time condition belongs on the record's own time unless the
// question names another, and the shared rules are the run 10 text again, so a question without a time cue is
// shown exactly what it was shown before.
var _ = Describe("Governed SQL: which time a time condition means", func() {
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

	Describe("the shared rules", func() {
		It("are byte for byte the run 10 text, so a question with no time cue is shown what it was shown then", func() {
			sum := sha256.Sum256([]byte(govSQLRules))
			// sha256 of the rules constant of the image that ran run 10 (commit b111b585^), 2,581 bytes.
			Expect(hex.EncodeToString(sum[:])).To(Equal("e6004f75f9d8fd26e46cbef7d9b4cdeabae539747093f5fad743e74641244c7f"))
			Expect(govSQLRules).To(ContainSubstring("6. Times are as recorded at the source."))
			Expect(govSQLRules).NotTo(ContainSubstring("case clock"))
		})

		It("leave a question with no time cue with nothing about time or the clock in what it is shown", func() {
			for question, view := range map[string]string{
				"What is the smallest position among the towers?": "v_tower",
				"How many calls did 923001110001 make?":           "v_cdr",
				"Which plate number appears most often?":          "v_anpr",
			} {
				facts := factsFor(question, view)
				Expect(facts.TimeCues).To(BeEmpty(), question)
				offered := []*govSQLView{views[view]}
				user := govSQLUserPrompt(question, facts, offered)
				Expect(user).NotTo(ContainSubstring("restricts time"), question)
				Expect(user).NotTo(ContainSubstring("case clock"), question)
				Expect(govSQLSystemPrompt(offered)).NotTo(ContainSubstring("case clock"), question)
			}
		})
	})

	Describe("the record's own time", func() {
		It("is event_time in every dated family, and there is none where the layer has no such field", func() {
			for _, name := range []string{"v_cdr", "v_ipdr", "v_anpr", "v_access_log", "v_transaction"} {
				primary := govSQLPrimaryTime(views[name])
				Expect(primary).NotTo(BeNil(), name)
				Expect(primary.Name).To(Equal("event_time"), name)
			}
			Expect(govSQLPrimaryTime(views["v_subscriber"])).To(BeNil())
			Expect(govSQLPrimaryTime(views["v_tower"])).To(BeNil())
			Expect(govSQLPrimaryTime(nil)).To(BeNil())
		})

		It("is named in the hint when the question names no other time of the record", func() {
			question := "How many call records were there at night?"
			offered := []*govSQLView{views["v_cdr"]}
			user := govSQLUserPrompt(question, factsFor(question, "v_cdr"), offered)
			Expect(user).To(ContainSubstring("on event_time, the time of the record"))
			Expect(user).NotTo(ContainSubstring("call_end, call_start"))
			Expect(user).To(ContainSubstring("Times are on the case clock"))
			Expect(user).To(ContainSubstring("never convert time zones"))
		})

		It("leaves the choice open, with the case clock, when the question names another time of the record", func() {
			question := "How many calls ended after 23:00?"
			offered := []*govSQLView{views["v_cdr"]}
			user := govSQLUserPrompt(question, factsFor(question, "v_cdr"), offered)
			Expect(user).To(ContainSubstring("on a time column (call_end, call_start, event_time)"))
			Expect(user).To(ContainSubstring("Times are on the case clock"))
		})

		It("is left alone in a family that has none: the hint lists the time columns", func() {
			question := "How many registered numbers were activated in 2023?"
			offered := []*govSQLView{views["v_subscriber"]}
			user := govSQLUserPrompt(question, factsFor(question, "v_subscriber"), offered)
			Expect(user).To(ContainSubstring("on a time column ("))
			Expect(user).To(ContainSubstring("activation_date"))
		})
	})

	Describe("the check", func() {
		const night = "How many call records were there at night?"
		const startOrEnd = "SELECT COUNT(*) AS number_of_call_records FROM v_cdr WHERE EXTRACT(HOUR FROM call_start) BETWEEN 0 AND 5 OR EXTRACT(HOUR FROM call_end) BETWEEN 0 AND 5"

		It("refuses the call's start OR end for a question that named neither (the run 11 answer, 1,771)", func() {
			Expect(unmetKinds(night, "v_cdr", startOrEnd)).To(ConsistOf("TIME_COLUMN"))
		})

		It("says what to do, to the model and to the analyst", func() {
			validated, _ := govSQLValidate(startOrEnd, views)
			unmet := govSQLCheck(factsFor(night, "v_cdr"), validated)
			Expect(unmet).To(HaveLen(1))
			Expect(unmet[0].Message).To(ContainSubstring("belongs on event_time"))
			Expect(unmet[0].Message).To(ContainSubstring("call_end, call_start"))
			Expect(unmet[0].Reason).To(ContainSubstring("which the question does not name"))
		})

		It("accepts the record's own time", func() {
			Expect(unmetKinds(night, "v_cdr", "SELECT COUNT(*) AS n FROM v_cdr WHERE EXTRACT(HOUR FROM event_time) BETWEEN 0 AND 5")).To(BeEmpty())
			Expect(unmetKinds("How many calls were made in June 2026?", "v_cdr", "SELECT COUNT(*) AS n FROM v_cdr WHERE event_time >= '2026-06-01' AND event_time < '2026-07-01'")).To(BeEmpty())
		})

		It("refuses a time the question did not name even next to the record's own", func() {
			Expect(unmetKinds(night, "v_cdr", "SELECT COUNT(*) AS n FROM v_cdr WHERE EXTRACT(HOUR FROM event_time) BETWEEN 0 AND 5 AND call_end IS NOT NULL")).To(ConsistOf("TIME_COLUMN"))
		})

		It("accepts the call's end when the question says it ended, and the start when it says it started", func() {
			Expect(unmetKinds("How many calls ended at night?", "v_cdr", "SELECT COUNT(*) AS n FROM v_cdr WHERE EXTRACT(HOUR FROM call_end) BETWEEN 0 AND 5")).To(BeEmpty())
			Expect(unmetKinds("How many calls started in April 2026?", "v_cdr", "SELECT COUNT(*) AS n FROM v_cdr WHERE call_start >= '2026-04-01' AND call_start < '2026-05-01'")).To(BeEmpty())
			Expect(unmetKinds("How many calls started in April 2026?", "v_cdr", "SELECT COUNT(*) AS n FROM v_cdr WHERE call_end >= '2026-04-01' AND call_end < '2026-05-01'")).To(ConsistOf("TIME_COLUMN"))
		})

		It("does not mind a time that is only selected, which asks for a time and constrains none", func() {
			Expect(unmetKinds("When was the first call in June 2026?", "v_cdr", "SELECT MIN(call_start) AS first_call FROM v_cdr WHERE event_time >= '2026-06-01' AND event_time < '2026-07-01'")).To(BeEmpty())
			Expect(unmetKinds("What is the earliest call in the data?", "v_cdr", "SELECT MIN(call_start) AS earliest_call FROM v_cdr")).To(BeEmpty())
		})

		It("asks nothing of a family with no record time of its own", func() {
			Expect(unmetKinds("How many registered numbers were activated in 2023?", "v_subscriber", "SELECT COUNT(*) AS n FROM v_subscriber WHERE activation_date >= '2023-01-01' AND activation_date < '2024-01-01'")).To(BeEmpty())
			Expect(unmetKinds("How many registered numbers were there in 2023?", "v_subscriber", "SELECT COUNT(*) AS n FROM v_subscriber WHERE activation_date >= '2023-01-01' AND activation_date < '2024-01-01'")).To(BeEmpty())
		})

		It("still sends the model to the record's own time when it applied no time condition at all", func() {
			validated, _ := govSQLValidate("SELECT COUNT(*) AS n FROM v_cdr", views)
			unmet := govSQLCheck(factsFor(night, "v_cdr"), validated)
			Expect(unmet).NotTo(BeEmpty())
			Expect(unmet[0].Kind).To(Equal("TIME"))
			Expect(unmet[0].Message).To(ContainSubstring("event_time, the time of the record"))
		})
	})

	Describe("reading which time a question names", func() {
		It("takes a word that begins the way a name of the column does, and nothing the times share", func() {
			for _, pair := range [][2]string{{"end", "ended"}, {"start", "started"}, {"start", "starts"}, {"activation", "activated"}, {"finished", "finish"}} {
				Expect(govSQLWordsAlike(pair[0], pair[1])).To(BeTrue(), pair[0]+" / "+pair[1])
			}
			for _, pair := range [][2]string{{"end", "energy"}, {"start", "stat"}, {"end", "call"}, {"start", "end"}} {
				Expect(govSQLWordsAlike(pair[0], pair[1])).To(BeFalse(), pair[0]+" / "+pair[1])
			}
		})

		It("names the column the question talks about and not the others", func() {
			cdr := views["v_cdr"]
			column := func(name string) govSQLColumn {
				for _, c := range cdr.Columns {
					if c.Name == name {
						return c
					}
				}
				Fail("no column " + name)
				return govSQLColumn{}
			}
			Expect(govSQLNamesTimeColumn("How many calls ended at night?", cdr, column("call_end"))).To(BeTrue())
			Expect(govSQLNamesTimeColumn("How many calls ended at night?", cdr, column("call_start"))).To(BeFalse())
			Expect(govSQLNamesTimeColumn("calls that began in June", cdr, column("call_start"))).To(BeTrue())
			Expect(govSQLNamesTimeColumn("How many call records were there at night?", cdr, column("call_start"))).To(BeFalse())
			Expect(govSQLNamesTimeColumn("How many call records were there at night?", cdr, column("call_end"))).To(BeFalse())
		})
	})
})
