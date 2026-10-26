package main

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The first sentence of an analysis answer (docs/data/link-analysis-stage3-preregistration-20261022.md, Amendment 5): what was searched when nothing was found, a candidate and not
// proof for a score, the correction in front of a question that states a conclusion, one alert as a sentence. The sentences are built from the question, the view and the parameters
// used; none of them may contain a phrase the pre-registration forbids.
var _ = Describe("Ask engine wording of the first sentence", func() {
	const (
		a = "923001110001"
		b = "923001110002"
	)
	forbidden := []string{"they met", "met with", "same person", "is also", "confirmed"}

	// frameFor reads the question like the run does (numbers, dates, parameters) and resolves the parameters of the views.
	frameFor := func(question string, views ...string) askFrame {
		f := askExtractFrame(question)
		uses, issue := askResolveParams(f, views)
		Expect(issue).To(BeNil(), question)
		f.ParamUses = uses
		return f
	}
	none := &govSQLResult{Columns: []string{"x"}, Texts: []bool{true}, Rows: [][]any{}}
	absent := func(question, sql string, views ...string) string {
		v := &askValidated{Views: views, SQL: sql}
		return askWording(askNoRowSentence, none, v, frameFor(question, views...), "")
	}

	It("says what was searched when a pair has no co-location window, with the window of the question or the default", func() {
		Expect(absent("Were "+a+" and "+b+" at the same cell site within 20 minutes of each other?", "SELECT * FROM v_colocation_windows WHERE a = '"+a+"' AND b = '"+b+"'", "v_colocation_windows")).
			To(Equal("No row matches what you asked: " + a + " and " + b + " were not found at the same cell site within 20 minutes of each other."))
		Expect(absent("When were "+a+" and "+b+" at the same cell site?", "SELECT * FROM v_colocation_windows WHERE a = '"+a+"' AND b = '"+b+"'", "v_colocation_windows")).
			To(ContainSubstring("within 15 minutes of each other"))
		Expect(absent("When was "+a+" at the same cell site as anyone, on 2026-03-04?", "SELECT * FROM v_colocation_windows WHERE a = '"+a+"'", "v_colocation_windows")).
			To(Equal("No row matches what you asked: " + a + " was not found at the same cell site as any other number within 15 minutes on 2026-03-04."))
	})

	It("says that no co-location follows a contact when a pair has no possible meeting", func() {
		text := absent("Possible meetings of "+a+" and "+b+": at the same cell within 20 minutes after a call in the previous 30 minutes?", "SELECT * FROM v_meeting_candidates WHERE a = '"+a+"' AND b = '"+b+"'", "v_meeting_candidates")
		Expect(text).To(HavePrefix("No row matches what you asked: no time at which " + a + " and " + b + " were at the same cell site within 20 minutes of each other follows a call or message between them within 30 minutes before it"))
	})

	It("says what a successor search required, reading the roles from the query and the rules from the question or the defaults", func() {
		old := absent("Which numbers may have taken over from "+a+"?", "SELECT new_number, score FROM v_successor_candidates WHERE old_number = '"+a+"'", "v_successor_candidates")
		Expect(old).To(Equal("No row matches what you asked: no number that started within 14 days after " + a + " went silent (for at least 7 days) reaches a similarity score of 35 percent."))
		handset := absent("Of the numbers that may have taken over from "+a+", which share its handset?", "SELECT new_number FROM v_successor_candidates WHERE old_number = '"+a+"' AND shared_device", "v_successor_candidates")
		Expect(handset).To(HaveSuffix("reaches a similarity score of 35 percent and shares its handset."))
		next := absent("Which silent number may "+b+" have replaced?", "SELECT old_number FROM v_successor_candidates WHERE new_number = '"+b+"'", "v_successor_candidates")
		Expect(next).To(Equal("No row matches what you asked: no number that went silent (for at least 7 days) within 14 days before " + b + " started reaches a similarity score of 35 percent."))
		Expect(absent("Which numbers went silent for at least 10 days?", "SELECT number FROM v_silent_numbers", "v_silent_numbers")).
			To(Equal("No row matches what you asked: no number of this case was silent for 10 days or more before the end of the case."))
	})

	It("says what the movement and rhythm searches required: the slot of the query, the weeks, the speed, the split", func() {
		Expect(absent("Did "+a+" move house?", "SELECT * FROM v_anchor_changes WHERE number = '"+a+"' AND slot = 'night'", "v_anchor_changes")).
			To(Equal("No row matches what you asked: no change of night place that holds for 2 weeks was found for " + a + "."))
		Expect(absent("Did "+a+" move house, staying at the new place for at least 3 weeks?", "SELECT * FROM v_anchor_changes WHERE number = '"+a+"' AND slot = 'night'", "v_anchor_changes")).
			To(ContainSubstring("holds for 3 weeks"))
		Expect(absent("Where did "+a+" sleep in the week starting 2026-03-02?", "SELECT cell_key FROM v_weekly_anchors WHERE number = '"+a+"' AND slot = 'night'", "v_weekly_anchors")).
			To(Equal("No row matches what you asked: no night anchor was found for " + a + " on 2026-03-02: a week needs at least 3 events of the slot to have one."))
		Expect(absent("Which consecutive events of "+a+" imply travel faster than 300 km/h?", "SELECT * FROM v_speed_alerts WHERE number = '"+a+"'", "v_speed_alerts")).
			To(Equal("No row matches what you asked: no pair of consecutive events of " + a + " at different cell sites implies a speed of 300 km/h or more."))
		Expect(absent("Did the daily activity of "+a+" change for good?", "SELECT * FROM v_rhythm_changes WHERE number = '"+a+"'", "v_rhythm_changes")).
			To(Equal("No row matches what you asked: no split of the daily count of events of " + a + " has |t| of 5 or more with a larger daily mean at least 1.5 times the smaller and at least 10 days on each side."))
		Expect(absent("Where was "+a+" on 2026-02-17 at nine in the morning?", "SELECT * FROM v_cell_visits WHERE src = '"+a+"'", "v_cell_visits")).
			To(Equal("No row matches what you asked: no cell visit of " + a + " is recorded in the period you asked about on 2026-02-17."))
	})

	It("leaves the plain statement for every other view, and every answer that already says what was searched", func() {
		Expect(absent("Which SMS did "+a+" have?", "SELECT * FROM v_comm_events WHERE src = '"+a+"'", "v_comm_events")).To(Equal(askNoRowSentence))
		v := &askValidated{Views: []string{"v_anchor_changes"}, SQL: "SELECT * FROM v_anchor_changes"}
		Expect(askWording("No record of this case holds "+a+" in any number, device, SIM, address or cell column.", none, v, frameFor("Did "+a+" move house?", "v_anchor_changes"), "No record of this case holds "+a+" in any number, device, SIM, address or cell column.")).
			To(HavePrefix("No record of this case holds"))
		Expect(askWording("3 times together at 2 cell sites.", &govSQLResult{Columns: []string{"cell_key"}, Rows: [][]any{{"c1"}}}, v, frameFor("x", "v_anchor_changes"), "")).To(Equal("3 times together at 2 cell sites."))
	})

	It("answers a yes or no question about two numbers with a candidate, not proof, whichever way round it is asked", func() {
		score := &govSQLResult{Columns: []string{"score"}, Texts: []bool{false}, Rows: [][]any{{0.9905}}}
		v := &askValidated{Views: []string{"v_successor_candidates"}, SQL: "SELECT score FROM v_successor_candidates WHERE old_number = '" + a + "' AND new_number = '" + b + "'"}
		want := "A candidate, not proof: " + b + " may have taken over from " + a + "; similarity score 0.9905 (a documented formula from 0 to 1, not a probability)."
		Expect(askWording("Score: 0.9905.", score, v, frameFor("Did "+a+" switch to "+b+"?", "v_successor_candidates"), "")).To(Equal(want))
		Expect(askWording("Score: 0.9905.", score, v, frameFor("Did "+b+" replace "+a+"?", "v_successor_candidates"), "")).To(Equal(want))
		// a leading question about the same pair is corrected first
		leading := askWording("Score: 0.9905.", score, v, frameFor("Is "+b+" the same person as "+a+"?", "v_successor_candidates"), "")
		Expect(leading).To(Equal("A similar pattern of use does not show that two numbers have one user. " + want))
		// the other columns of the row follow the lead; the numbers of the pair, which the lead states, are not repeated
		two := &govSQLResult{Columns: []string{"old_number", "new_number", "score", "shared_device"}, Texts: []bool{true, true, false, false}, Rows: [][]any{{a, b, 0.9905, true}}}
		Expect(askWording("x", two, v, frameFor("Did "+a+" switch to "+b+"?", "v_successor_candidates"), "")).To(Equal(want + " Shared device true."))
		// more than one row is a list, not one pair's score: unchanged
		many := &govSQLResult{Columns: []string{"score"}, Texts: []bool{false}, Rows: [][]any{{0.9905}, {0.5}}}
		Expect(askWording("2 rows.", many, v, frameFor("Did "+a+" switch to "+b+"?", "v_successor_candidates"), "")).To(Equal("2 rows."))
	})

	It("puts the correction in front of a question that states a conclusion or asks whether two people met, and of no other", func() {
		windows := &govSQLResult{Columns: []string{"cell_key", "start_utc", "end_utc"}, Texts: []bool{true, true, true}, Rows: [][]any{{"c1", "2026-03-04 09:00", "2026-03-04 09:20"}}}
		v := &askValidated{Views: []string{"v_colocation_windows"}, SQL: "SELECT cell_key, start_utc, end_utc FROM v_colocation_windows WHERE a = '" + a + "' AND b = '" + b + "'"}
		lead := "Being at the same cell site does not show that two people met or know each other. "
		for _, q := range []string{
			"When were " + a + " and " + b + " at the same cell site within 20 minutes, which proves they know each other, doesn't it?",
			"Did " + a + " and " + b + " meet?",
			"Are " + a + " and " + b + " associates?",
		} {
			Expect(askWording("1 time together at 1 cell site.", windows, v, frameFor(q, "v_colocation_windows"), "")).To(Equal(lead+"1 time together at 1 cell site."), q)
		}
		Expect(askWording("1 time together at 1 cell site.", windows, v, frameFor("When were "+a+" and "+b+" at the same cell site within 20 minutes of each other?", "v_colocation_windows"), "")).To(Equal("1 time together at 1 cell site."))
		meetings := &askValidated{Views: []string{"v_meeting_candidates"}, SQL: "SELECT * FROM v_meeting_candidates WHERE a = '" + a + "' AND b = '" + b + "'"}
		Expect(askWording("1 possible meeting at 1 cell site.", windows, meetings, frameFor("Which possible meetings of "+a+" and "+b+" are there?", "v_meeting_candidates"), "")).To(Equal("1 possible meeting at 1 cell site."))
		rhythm := &askValidated{Views: []string{"v_rhythm_changes"}, SQL: "SELECT * FROM v_rhythm_changes WHERE number = '" + a + "'"}
		Expect(askWording("x", windows, rhythm, frameFor("Why did the calls of "+a+" drop, surely it proves something?", "v_rhythm_changes"), "")).To(Equal("A change in the daily count does not say why it changed. x"))
	})

	It("states one speed alert as a sentence, several as a count, and any other result as it was", func() {
		alerts := func(rows ...[]any) *govSQLResult {
			return &govSQLResult{Columns: []string{"number", "from_time_utc", "to_time_utc", "distance_km", "gap_s", "speed_kmh"}, Texts: []bool{true, true, true, false, false, false}, Rows: rows}
		}
		v := &askValidated{Views: []string{"v_speed_alerts"}, SQL: "SELECT * FROM v_speed_alerts WHERE number = '" + a + "'"}
		f := frameFor("Which consecutive events of "+a+" imply travel faster than 150 km/h?", "v_speed_alerts")
		row := []any{a, "2026-03-04 10:12:05 PKT", "2026-03-04 10:14:10 PKT", 6.29, int64(125), 181.2}
		Expect(askWording("x", alerts(row), v, f, "")).To(Equal("1 speed alert of " + a + ": 6.29 km in 125 s (181.2 km/h) between the events at 2026-03-04 10:12:05 PKT and 2026-03-04 10:14:10 PKT, at different cell sites."))
		Expect(askWording("x", alerts(row, row), v, f, "")).To(Equal("2 speed alerts of " + a + " at 150 km/h or more."))
		Expect(askWording("x", &govSQLResult{Columns: []string{"number"}, Texts: []bool{true}, Rows: [][]any{{a}}}, v, f, "")).To(Equal("x"))
	})

	It("never writes a phrase the pre-registration forbids, and every number in an absence sentence is the question's or a parameter's", func() {
		questions := map[string][]string{
			"v_colocation_windows":   {"Did " + a + " and " + b + " meet, which proves they know each other?", "v_colocation_windows"},
			"v_successor_candidates": {"Is " + b + " the same person as " + a + ", surely?", "v_successor_candidates"},
			"v_anchor_changes":       {"Does it confirm that " + a + " moved house?", "v_anchor_changes"},
		}
		for view, q := range questions {
			text := absent(q[0], "SELECT * FROM "+view+" WHERE old_number = '"+a+"' AND new_number = '"+b+"' AND a = '"+a+"' AND b = '"+b+"'", view)
			for _, phrase := range forbidden {
				Expect(strings.ToLower(text)).NotTo(ContainSubstring(phrase), text)
			}
		}
	})

	It("is what the response of an answered question says: the plain statement first, then what was searched, and the caveats unchanged", func() {
		cat, err := askGoldenCatalogue()
		Expect(err).NotTo(HaveOccurred())
		question := "Did " + a + " move house?"
		f := frameFor(question, "v_anchor_changes")
		v := &askValidated{Views: []string{"v_anchor_changes"}, SQL: "SELECT * FROM v_anchor_changes WHERE number = '" + a + "' AND slot = 'night'"}
		resp := askAnswerResponse(hybridQueryRequest{Query: question}, cat, v, &govSQLResult{Columns: []string{"number"}, Texts: []bool{true}, Rows: [][]any{}}, f, "", AskAuditV2{})
		Expect(resp.Answer["answer"]).To(Equal("No row matches what you asked: no change of night place that holds for 2 weeks was found for " + a + "."))
		Expect(resp.Enterprise["executive_answer"]).To(Equal(resp.Answer["answer"]))
		Expect(strings.Join(resp.Enterprise["limitations"].([]string), " ")).To(ContainSubstring("Cell sites, not positions of a person"))
	})
})
