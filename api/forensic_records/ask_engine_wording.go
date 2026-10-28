package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ASK ENGINE V2: WHAT THE FIRST SENTENCE SAYS WHEN THERE IS NO ROW, A CANDIDATE, OR A PREMISE THE RECORDS DO NOT SUPPORT
// (docs/data/link-analysis-stage3-preregistration-20261022.md, Amendment 5; found by the review of the stage-3 answers, docs/data/link-analysis-stage3-review-check-20261026.md).
//
// The numbers of the answer do not change here and no model writes a word of it: every sentence is built by code from the question, the view the query reads and the values of the
// runtime parameters that were used, and every number in it is one of those. Three things were thin although right:
//  1. "No row matches what you asked." says nothing of what was searched, so it reads as "it did not happen". For the analysis views the same sentence now goes on to say what was
//     searched and with which rules ("... no change of night place that holds for 2 weeks was found for 9230..."). The plain statement stays first so that the judges and the
//     interface that read it are unchanged.
//  2. A yes or no question about two numbers ("Did A switch to B?") answered with a bare "Score: 0.99." is now answered "A candidate, not proof: ...", with the score named as a documented
//     formula and not a probability.
//  3. A question that states a conclusion the records cannot support ("which proves they know each other, doesn't it?", "Did A and B meet?") gets the one-line correction in front of the
//     answer, not only in the limitations.
// And one alert is a sentence, not "From time utc ...; To time utc ...".
// The forbidden phrases of the pre-registration ("they met", "met with", "same person", "is also", "confirmed") appear in none of them.

const askNoRowSentence = "No row matches what you asked."

// askParamValue is the value the query is run with for a runtime parameter: the one resolved for this question, else the declared default.
func askParamValue(f askFrame, name string) int {
	for _, u := range f.ParamUses {
		if u.Param.Name == name {
			return u.Value
		}
	}
	for _, p := range askParams {
		if p.Name == name {
			return p.Default
		}
	}
	return 0
}

func askReadsView(v *askValidated, view string) bool {
	if v == nil {
		return false
	}
	for _, name := range v.Views {
		if name == view {
			return true
		}
	}
	return false
}

// askWhereOf is the WHERE clause of the query, lower case, up to the next clause; "" when it has none. A sentence may say that a condition was applied only if it is in this text: a column
// of the select list is not a condition (found by reading the run on case 113: "and shares its handset" was said of a query that only listed shared_device).
func askWhereOf(v *askValidated) string {
	if v == nil {
		return ""
	}
	lower := strings.ToLower(v.SQL)
	at := strings.Index(lower, " where ")
	if at < 0 {
		return ""
	}
	rest := lower[at+len(" where "):]
	for _, next := range []string{" group by ", " order by ", " having ", " limit ", " union "} {
		if i := strings.Index(rest, next); i >= 0 {
			rest = rest[:i]
		}
	}
	return rest
}

// askSlotOf is the slot of an anchor query: "night" or "day" when its WHERE clause names exactly one, "" when it names both or neither.
func askSlotOf(v *askValidated) string {
	where := askWhereOf(v)
	night, day := strings.Contains(where, "'night'"), strings.Contains(where, "'day'")
	switch {
	case night && !day:
		return "night"
	case day && !night:
		return "day"
	}
	return ""
}

var askMonthNames = [...]string{"", "January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"}

// askPeriodPhrase says the period the question names: " on 2026-02-17", " in March 2026", or "".
func askPeriodPhrase(f askFrame) string {
	switch {
	case len(f.Dates) == 1:
		return " on " + f.Dates[0]
	case len(f.Dates) > 1:
		return " on the dates you named"
	case len(f.Months) == 1:
		m := f.Months[0]
		if m.Month < 1 || m.Month > 12 {
			return ""
		}
		if m.Year > 0 {
			return fmt.Sprintf(" in %s %d", askMonthNames[m.Month], m.Year)
		}
		return " in " + askMonthNames[m.Month]
	case len(f.Months) > 1:
		return " in the months you named"
	}
	return ""
}

// askOf is " of 9230..." for the first number of the question, "" when it names none.
func askOf(f askFrame) string {
	if len(f.Numbers) == 0 {
		return ""
	}
	return " of " + f.Numbers[0]
}

func askFor(f askFrame) string {
	if len(f.Numbers) == 0 {
		return ""
	}
	return " for " + f.Numbers[0]
}

// askAbsenceSentence says what was searched, for the analysis views, when the result holds no row: "No row matches what you asked: ...". "" for any other view.
func askAbsenceSentence(v *askValidated, f askFrame) string {
	if v == nil {
		return ""
	}
	body := ""
	switch {
	case askReadsView(v, "v_successor_candidates"):
		body = askSuccessorAbsence(v, f)
	case askReadsView(v, "v_silent_numbers"):
		body = fmt.Sprintf("no number of this case was silent for %d days or more before the end of the case", askParamValue(f, "silence_days"))
	case askReadsView(v, "v_meeting_candidates"):
		window, gap := askParamValue(f, "colocation_minutes"), askParamValue(f, "contact_gap_minutes")
		switch len(f.Numbers) {
		case 0:
			body = fmt.Sprintf("no time at which two numbers were at the same cell site within %d minutes of each other follows a call or message between them within %d minutes before it", window, gap)
		case 1:
			body = fmt.Sprintf("no time at which %s was at the same cell site as another number within %d minutes follows a call or message between them within %d minutes before it", f.Numbers[0], window, gap)
		default:
			body = fmt.Sprintf("no time at which %s and %s were at the same cell site within %d minutes of each other follows a call or message between them within %d minutes before it", f.Numbers[0], f.Numbers[1], window, gap)
		}
		body += askPeriodPhrase(f)
	case askReadsView(v, "v_colocation_windows"):
		window := askParamValue(f, "colocation_minutes")
		switch len(f.Numbers) {
		case 0:
			body = fmt.Sprintf("no two numbers were found at the same cell site within %d minutes of each other", window)
		case 1:
			body = fmt.Sprintf("%s was not found at the same cell site as any other number within %d minutes", f.Numbers[0], window)
		default:
			body = fmt.Sprintf("%s and %s were not found at the same cell site within %d minutes of each other", f.Numbers[0], f.Numbers[1], window)
		}
		body += askPeriodPhrase(f)
	case askReadsView(v, "v_anchor_changes"):
		body = fmt.Sprintf("no change of %splace that holds for %d weeks was found%s", askSlotWord(v), askParamValue(f, "anchor_hold_weeks"), askFor(f))
	case askReadsView(v, "v_anchor_deviations"):
		body = fmt.Sprintf("no stay of fewer than %d weeks at a different %splace (a trip) was found%s", askParamValue(f, "anchor_hold_weeks"), askSlotWord(v), askFor(f))
	case askReadsView(v, "v_movement_anchors"), askReadsView(v, "v_anchor_runs"):
		body = fmt.Sprintf("no %sanchor that holds for %d weeks was found%s", askSlotWord(v), askParamValue(f, "anchor_hold_weeks"), askFor(f))
	case askReadsView(v, "v_weekly_anchors"):
		body = fmt.Sprintf("no %sanchor was found%s%s: a week needs at least %d events of the slot to have one", askSlotWord(v), askFor(f), askPeriodPhrase(f), askParamValue(f, "anchor_min_events"))
	case askReadsView(v, "v_speed_alerts"):
		body = fmt.Sprintf("no pair of consecutive events%s at different cell sites implies a speed of %d km/h or more", askOf(f), askParamValue(f, "speed_kmh"))
	case askReadsView(v, "v_rhythm_changes"):
		times := strconv.FormatFloat(float64(askParamValue(f, "rhythm_ratio_percent"))/100, 'f', -1, 64)
		body = fmt.Sprintf("no split of the daily count of events%s has |t| of 5 or more with a larger daily mean at least %s times the smaller and at least %d days on each side", askOf(f), times, askParamValue(f, "rhythm_min_side_days"))
	case askReadsView(v, "v_plate_sightings"), askReadsView(v, "v_plate_visits"):
		body = askPlateAbsence(v, f)
	case askReadsView(v, "v_subscriber_registrations"), askReadsView(v, "v_number_registration_activity"):
		body = askRegistrationAbsence(v, f)
	case askReadsView(v, "v_handsets"):
		body = askHandsetAbsence(v, f)
	case askReadsTransactionView(v):
		body = askTransactionAbsence(v, f)
	case askReadsView(v, "v_cell_visits"):
		who := "any number"
		if len(f.Numbers) > 0 {
			who = f.Numbers[0]
		}
		body = fmt.Sprintf("no cell visit of %s is recorded in the period you asked about%s", who, askPeriodPhrase(f))
	}
	if body == "" {
		return ""
	}
	return strings.TrimSuffix(askNoRowSentence, ".") + ": " + body + "."
}

// askSlotWord is "night " or "day " for an anchor query that names one slot, "" otherwise (the trailing space is part of the word).
func askSlotWord(v *askValidated) string {
	if slot := askSlotOf(v); slot != "" {
		return slot + " "
	}
	return ""
}

// askSuccessorAbsence reads the roles of the numbers from the query's own comparisons (old_number = ..., new_number = ...), not from the order of the question.
func askSuccessorAbsence(v *askValidated, f askFrame) string {
	silence, window, score := askParamValue(f, "silence_days"), askParamValue(f, "successor_window_days"), askParamValue(f, "successor_score_percent")
	var old, next string
	for _, n := range f.Numbers {
		compared := askComparedColumns(v.SQL, n)
		switch {
		case compared["old_number"] && old == "":
			old = n
		case compared["new_number"] && next == "":
			next = n
		}
	}
	shares := ""
	if strings.Contains(askWhereOf(v), "shared_device") {
		shares = " and shares its handset"
	}
	switch {
	case old != "":
		return fmt.Sprintf("no number that started within %d days after %s went silent (for at least %d days) reaches a similarity score of %d percent%s", window, old, silence, score, shares)
	case next != "":
		return fmt.Sprintf("no number that went silent (for at least %d days) within %d days before %s started reaches a similarity score of %d percent%s", silence, window, next, score, shares)
	}
	return fmt.Sprintf("no pair of a silent number (silent for at least %d days) and a number that started within %d days afterwards reaches a similarity score of %d percent%s", silence, window, score, shares)
}

// askScoreLead is the first sentence for the score of one pair of numbers ("Did A switch to B?"): a candidate, not proof, with the score as a documented formula. The roles are read
// from the query's own comparisons, so the sentence is the same whichever way round the question is asked. Any other columns of the row follow, as in the plain headline.
func askScoreLead(r *govSQLResult, v *askValidated, f askFrame) string {
	score := askColumnIndex(r, "score")
	if !askReadsView(v, "v_successor_candidates") || len(r.Rows) != 1 || score < 0 || len(f.Numbers) < 2 {
		return ""
	}
	var old, next string
	for _, n := range f.Numbers {
		compared := askComparedColumns(v.SQL, n)
		switch {
		case compared["old_number"] && old == "":
			old = n
		case compared["new_number"] && next == "":
			next = n
		}
	}
	value := askCell(r, score, r.Rows[0][score])
	lead := fmt.Sprintf("A candidate, not proof: similarity score %s (a documented formula from 0 to 1, not a probability).", value)
	if old != "" && next != "" {
		lead = fmt.Sprintf("A candidate, not proof: %s may have taken over from %s; similarity score %s (a documented formula from 0 to 1, not a probability).", next, old, value)
	}
	var others []string
	for i := range r.Columns {
		if i == score || strings.EqualFold(r.Columns[i], "old_number") || strings.EqualFold(r.Columns[i], "new_number") {
			continue
		}
		others = append(others, askLabel(r, i)+" "+askCell(r, i, r.Rows[0][i]))
	}
	if len(others) > 0 {
		lead += " " + strings.Join(others, "; ") + "."
	}
	return lead
}

// askAlertSentence is the sentence for the rows of the speed view: one alert says its distance, time and speed; several say how many.
func askAlertSentence(r *govSQLResult, v *askValidated, f askFrame, more string) string {
	if !askReadsView(v, "v_speed_alerts") || len(r.Rows) == 0 || askColumnIndex(r, "from_time_utc") < 0 {
		return ""
	}
	if len(r.Rows) > 1 {
		return fmt.Sprintf("%s%s at %d km/h or more%s.", askCount(len(r.Rows), "speed alert", "speed alerts"), askOf(f), askParamValue(f, "speed_kmh"), more)
	}
	at := func(name string) string {
		if i := askColumnIndex(r, name); i >= 0 {
			return askCell(r, i, r.Rows[0][i])
		}
		return ""
	}
	km, seconds, speed, from, to := at("distance_km"), at("gap_s"), at("speed_kmh"), at("from_time_utc"), at("to_time_utc")
	if km == "" || seconds == "" || speed == "" || from == "" || to == "" {
		return ""
	}
	return fmt.Sprintf("1 speed alert%s: %s km in %s s (%s km/h) between the events at %s and %s, at different cell sites.", askOf(f), km, seconds, speed, from, to)
}

// askMoveSentence is the sentence for the rows of the move view: one move says where from, where to, from which week and how far; several are counted.
func askMoveSentence(r *govSQLResult, v *askValidated, f askFrame, more string) string {
	if !askReadsView(v, "v_anchor_changes") || len(r.Rows) == 0 || askColumnIndex(r, "first_week_start") < 0 {
		return ""
	}
	place := "place"
	if slot := askSlotOf(v); slot != "" {
		place = slot + " place"
	}
	if len(r.Rows) > 1 {
		return fmt.Sprintf("%s of %s%s%s.", askCount(len(r.Rows), "change", "changes"), place, askOf(f), more)
	}
	at := func(name string) string {
		if i := askColumnIndex(r, name); i >= 0 && r.Rows[0][i] != nil {
			return askCell(r, i, r.Rows[0][i])
		}
		return ""
	}
	from, to, week, km := at("from_cell_key"), at("to_cell_key"), at("first_week_start"), at("distance_km")
	if from == "" || to == "" || week == "" {
		return ""
	}
	out := fmt.Sprintf("1 change of %s%s: from cell site %s to cell site %s, from the week starting %s", place, askOf(f), from, to, week)
	if km != "" {
		out += ", " + km + " km apart"
	}
	return out + "."
}

// askVisitSentence is the headline of rows of the cell-visit view: a visit to a cell site (a coverage area), not a position of a person.
func askVisitSentence(r *govSQLResult, v *askValidated, shape string) string {
	if !askReadsView(v, "v_cell_visits") || shape != "map" || len(r.Rows) == 0 {
		return ""
	}
	head := askCount(len(r.Rows), "cell site", "cell sites")
	if askColumnIndex(r, "hour_utc") >= 0 {
		head = askCount(len(r.Rows), "cell visit", "cell visits")
		if c := askColumnIndex(r, "cell_key"); c >= 0 {
			sites := map[string]bool{}
			for _, row := range r.Rows {
				sites[fmt.Sprint(row[c])] = true
			}
			head += " at " + askCount(len(sites), "cell site", "cell sites")
		}
	}
	return head + askTruncatedNote(r) + "."
}

// askPlateSentence is the headline of rows that are sightings of plates: how many, of which plate when the question names one, at how many cameras.
func askPlateSentence(r *govSQLResult, v *askValidated, f askFrame) string {
	if !askReadsView(v, "v_plate_sightings") || len(r.Rows) == 0 || askColumnIndex(r, "observed_utc") < 0 {
		return ""
	}
	head := askCount(len(r.Rows), "sighting", "sightings")
	if len(f.Plates) == 1 {
		head += " of the plate " + f.PlateWritten[f.Plates[0]]
	}
	if c := askColumnIndex(r, "camera_id"); c >= 0 {
		cameras := map[string]bool{}
		for _, row := range r.Rows {
			cameras[fmt.Sprint(row[c])] = true
		}
		head += " at " + askCount(len(cameras), "camera", "cameras")
	}
	return head + askTruncatedNote(r) + "."
}

// askPremiseCue finds a question that states a conclusion; askMeetingCue the questions about a meeting or an association.
var (
	askPremiseCue = regexp.MustCompile(`\b(?:prove[sd]?|proof|proving|same person|knows? each other|know one another|definitely|certainly|obviously|surely|confirm(?:s|ed)? (?:that|they|he|she|it|this))\b`)
	askMeetingCue = regexp.MustCompile(`\b(?:meet|met|associates?)\b`)
)

// askPremiseLead is the correction that goes in front of an answer whose question states a conclusion the records of the view cannot support. "" when the question states none.
func askPremiseLead(v *askValidated, f askFrame) string {
	lower := strings.ToLower(f.Question)
	stated := askPremiseCue.MatchString(lower)
	switch {
	case askReadsView(v, "v_meeting_candidates"), askReadsView(v, "v_colocation_windows"):
		if stated || askMeetingCue.MatchString(lower) {
			return "Being at the same cell site does not show that two people met or know each other."
		}
	case askReadsView(v, "v_successor_candidates"):
		if stated {
			return "A similar pattern of use does not show that two numbers have one user."
		}
	case askReadsView(v, "v_speed_alerts"):
		if stated {
			return "Two distant cell sites close in time do not show that anyone travelled that fast."
		}
	case askReadsView(v, "v_weekly_anchors"), askReadsView(v, "v_anchor_runs"), askReadsView(v, "v_movement_anchors"), askReadsView(v, "v_anchor_changes"), askReadsView(v, "v_anchor_deviations"), askReadsView(v, "v_cell_visits"):
		if stated {
			return "A cell site is a coverage area, not the position of a person."
		}
	case askReadsView(v, "v_rhythm_changes"):
		if stated {
			return "A change in the daily count does not say why it changed."
		}
	case askReadsView(v, "v_plate_sightings"), askReadsView(v, "v_plate_visits"):
		if stated {
			return "A camera sighting does not show who was driving, only that a camera read the plate."
		}
	case askReadsView(v, "v_subscriber_registrations"), askReadsView(v, "v_number_registration_activity"), askReadsView(v, "v_handsets"):
		return askRegistrationLead(v, f, stated)
	case askReadsTransactionView(v):
		return askTransactionLead(f, stated)
	}
	return ""
}

// askWording turns the generic headline into the first sentence of the answer.
func askWording(headline string, r *govSQLResult, v *askValidated, f askFrame, shape, absence string) string {
	if absence != "" || v == nil {
		return headline
	}
	switch {
	case len(r.Rows) == 0 && headline == askNoRowSentence:
		if s := askAbsenceSentence(v, f); s != "" {
			headline = s
		}
	case len(r.Rows) > 0:
		if s := askScoreLead(r, v, f); s != "" {
			headline = s
		} else if s := askAlertSentence(r, v, f, askTruncatedNote(r)); s != "" {
			headline = s
		} else if s := askMoveSentence(r, v, f, askTruncatedNote(r)); s != "" {
			headline = s
		} else if s := askVisitSentence(r, v, shape); s != "" {
			headline = s
		} else if s := askPlateSentence(r, v, f); s != "" {
			headline = s
		} else if askReadsView(v, "v_successor_candidates") {
			headline = "Candidates, not proof. " + headline
		}
	}
	if lead := askPremiseLead(v, f); lead != "" {
		headline = lead + " " + headline
	}
	return headline
}

func askTruncatedNote(r *govSQLResult) string {
	if r.Truncated {
		return fmt.Sprintf(" (the first %s; there are more)", govSQLGroup(fmt.Sprint(len(r.Rows))))
	}
	return ""
}
