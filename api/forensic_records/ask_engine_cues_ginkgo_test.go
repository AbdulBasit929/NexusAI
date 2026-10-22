package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Ask engine cues of the analyses of stages 3b and 3c", func() {
	view := func(q string) string {
		normalised, _ := askNormalise(q)
		return askCuesOf(normalised).view()
	}

	It("points each analysis at its own view, however it is worded", func() {
		for q, want := range map[string]string{
			"Which numbers may have taken over from 923001110001?":                        "v_successor_candidates",
			"Which silent number did 923001110002 replace?":                               "v_successor_candidates",
			"Did 923001110003 get a new number?":                                          "v_successor_candidates",
			"Which numbers went silent for 12 days before the case ended?":                "v_silent_numbers",
			"List the dormant numbers and when each stopped":                              "v_silent_numbers",
			"Did 923001110004 move house, and how far?":                                   "v_anchor_changes",
			"Has 923001110004 relocated since January?":                                   "v_anchor_changes",
			"Did 923001110005 make a short trip away from the usual night place?":         "v_anchor_deviations",
			"Which events of 923001110006 are faster than 150 km/h?":                      "v_speed_alerts",
			"Show impossible travel for 923001110006":                                     "v_speed_alerts",
			"Did the daily activity of 923001110007 change for good, and from which day?": "v_rhythm_changes",
			"Did the activity of 923001110007 change at least double?":                    "v_rhythm_changes",
			"Was there a change in the daily activity of 923001110007?":                   "v_rhythm_changes",
			"Did the usage of 923001110007 drop for good?":                                "v_rhythm_changes",
			"Has 923001110007 become less active?":                                        "v_rhythm_changes",
			"Where does 923001110008 sleep, and for which weeks?":                         "v_movement_anchors",
			"Show the anchors of 923001110008":                                            "v_movement_anchors",
			"Has the rate of calls of 923001110007 gone up for good?":                     "v_rhythm_changes",
			"Has the rate of calls of 923001110007 jumped for good?":                      "v_rhythm_changes",
			"Did 923001110003 switch to 923001110004?":                                    "v_successor_candidates",
			"Is 923001110004 the same person as 923001110003?":                            "v_successor_candidates",
			"Why did the calls of 923001110007 drop?":                                     "v_rhythm_changes",
			"Where did 923001110008 sleep in the week starting 2026-03-09?":               "v_weekly_anchors",
			"Where did 923001110008 sleep in week 4?":                                     "v_weekly_anchors",
			"Which numbers went silent and which number replaced them?":                   "v_successor_candidates",
			"Did 923001110004 move house and make a trip?":                                "v_anchor_changes",
		} {
			Expect(view(q)).To(Equal(want), q)
		}
	})

	It("reads \"count\" as a count of rows only when it asks for one", func() {
		counts := func(q string) bool { return askExtractFrame(q).Cues["count"] }
		Expect(counts("From which silent number may 923001110004 have taken over? Only count a score of at least 0.90.")).To(BeFalse())
		Expect(counts("Count 923001110004 as silent only after 10 days. Who may have replaced it?")).To(BeFalse())
		Expect(counts("Which numbers may have taken over from 923001110004, counting a score of at least 0.5?")).To(BeFalse())
		Expect(counts("Count the cell sites that 923001110001 and 923001110002 both used.")).To(BeTrue())
		Expect(counts("How many windows do 923001110001 and 923001110002 have?")).To(BeTrue())
		Expect(counts("What is the total number of calls of 923001110001?")).To(BeTrue())
		Expect(counts("Just count the calls of 923001110001 in March.")).To(BeTrue())
	})

	It("leaves the questions of the earlier stages to their own views", func() {
		for _, q := range []string{
			"Whom does 923001110005 phone the most often?",
			"How are 923001110008 and 923001110009 connected within three hops?",
			"Which cell sites did 923001110010 use most between 2026-03-01 and 2026-03-08?",
			"Did 923001110011 change phone or SIM? Show each handset and SIM with first and last seen.",
			"When were 923001110001 and 923001110002 at the same cell within 15 minutes of each other?",
			"How many events per day did 923001110001 have in March 2026?",
			"Which applications did 923001110001 use most in March 2026?",
			"How many images are in this case?",
			"Who held the public address 203.0.113.7 on port 40000 at 2026-03-04 10:15?",
		} {
			Expect(view(q)).To(Equal(""), q)
		}
	})
})
