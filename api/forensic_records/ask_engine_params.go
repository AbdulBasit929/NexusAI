package main

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// ASK ENGINE V2: RUNTIME PARAMETERS (link analysis stage 3, docs/data/link-analysis-stage3-preregistration-20261022.md, Amendment 2).
//
// A number an analyst asks for ("within 20 minutes", "in the previous 30 minutes") must not be a constant written into a view. Each parameter here is declared once: its name, the
// transaction setting the views read, its default, its bounds, the views that read it and how it is found in the question. The CODE finds the number in the question, checks it
// against the bounds and sets it as a transaction setting before the query runs (the way the hub threshold is set); the language model never writes or chooses it, and the analyst's
// SQL cannot set it (set_config is not an allowed function). The answer states the value used and where it came from. A question that names a parameter twice with different values,
// or minutes without a number, gets a question back instead of a guess; a value outside the bounds is refused with the bounds. Adding the parameters of a later analysis is one
// entry in askParams and the setting read by its view, nothing else.

type askParam struct {
	Name    string // key in the frame
	Setting string // the transaction setting the views read
	Label   string // what the answer calls it
	Unit    string // "minutes"
	Default int
	Min     int
	Max     int
	Views   []string // the views that read the setting
	Role    string   // "window" or "gap": which number of the question it is (see askMinuteSpans)
	// Find reads the values the question names for this parameter (in the unit of the parameter); nil for a parameter found through Role.
	Find func(lower string) []int
	// Quiet parameters are rules of the definition that a question rarely changes: the answer names them only when the question gave them.
	Quiet bool
}

var askParams = []askParam{
	{Name: "colocation_minutes", Setting: "app.colocation_minutes", Label: "Co-location window", Unit: "minutes", Default: 15, Min: 1, Max: 180,
		Views: []string{"v_colocation_windows", "v_meeting_candidates"}, Role: "window"},
	{Name: "contact_gap_minutes", Setting: "app.contact_gap_minutes", Label: "Longest gap between a contact and the window", Unit: "minutes", Default: 60, Min: 1, Max: 180,
		Views: []string{"v_meeting_candidates"}, Role: "gap"},
	// 3b: A9 successor candidates
	{Name: "silence_days", Setting: "app.silence_days", Label: "Days of silence", Unit: "days", Default: 7, Min: 1, Max: 365,
		Views: []string{"v_silent_numbers", "v_successor_candidates"}, Find: askFindSilenceDays},
	{Name: "successor_window_days", Setting: "app.successor_window_days", Label: "Days after the old number stopped within which the new number started", Unit: "days", Default: 14, Min: 0, Max: 365,
		Views: []string{"v_successor_candidates"}, Find: askFindSuccessorWindow},
	{Name: "successor_overlap_days", Setting: "app.successor_overlap_days", Label: "Days before the old number stopped from which the new number may have started", Unit: "days", Default: 2, Min: 0, Max: 90,
		Views: []string{"v_successor_candidates"}, Find: askFindSuccessorOverlap, Quiet: true},
	{Name: "successor_history_days", Setting: "app.successor_history_days", Label: "Days of contacts and places compared", Unit: "days", Default: 30, Min: 1, Max: 365,
		Views: []string{"v_successor_candidates"}, Find: askFindSuccessorHistory, Quiet: true},
	{Name: "successor_new_after_days", Setting: "app.successor_new_after_days", Label: "Days at the start of the case in which the new number must not have been in use", Unit: "days", Default: 7, Min: 0, Max: 365,
		Views: []string{"v_successor_candidates"}, Find: askFindSuccessorNewAfter, Quiet: true},
	{Name: "successor_score_percent", Setting: "app.successor_score_percent", Label: "Lowest similarity score listed", Unit: "percent", Default: 35, Min: 0, Max: 100,
		Views: []string{"v_successor_candidates"}, Find: askFindScorePercent},
	// 3c: A10 movement
	{Name: "anchor_min_events", Setting: "app.anchor_min_events", Label: "Fewest events of the slot a week needs to have an anchor", Unit: "events", Default: 3, Min: 1, Max: 100,
		Views: []string{"v_weekly_anchors", "v_anchor_runs", "v_movement_anchors", "v_anchor_changes", "v_anchor_deviations"}, Find: askFindAnchorMinEvents, Quiet: true},
	{Name: "anchor_hold_weeks", Setting: "app.anchor_hold_weeks", Label: "Weeks a new anchor must hold to count", Unit: "weeks", Default: 2, Min: 1, Max: 52,
		Views: []string{"v_anchor_runs", "v_movement_anchors", "v_anchor_changes", "v_anchor_deviations"}, Find: askFindHoldWeeks},
	{Name: "speed_kmh", Setting: "app.speed_kmh", Label: "Lowest speed listed", Unit: "km/h", Default: 200, Min: 1, Max: 2000,
		Views: []string{"v_speed_alerts"}, Find: askFindSpeedKmh},
	// 3c: A11 change of rhythm
	{Name: "rhythm_min_side_days", Setting: "app.rhythm_min_side_days", Label: "Fewest days on each side of a change point", Unit: "days", Default: 10, Min: 2, Max: 180,
		Views: []string{"v_rhythm_changes"}, Find: askFindRhythmSideDays, Quiet: true},
	{Name: "rhythm_ratio_percent", Setting: "app.rhythm_ratio_percent", Label: "Least ratio of the larger daily mean to the smaller", Unit: "percent", Default: 150, Min: 101, Max: 10000,
		Views: []string{"v_rhythm_changes"}, Find: askFindRhythmRatio},
}

// askParamRead is what the question says about one parameter.
type askParamRead struct {
	Found     bool
	Value     int
	Ambiguous bool  // the question names it more than once, with different values
	Values    []int // every value named, in order
}

// askParamUse is a parameter as it will be used for one query.
type askParamUse struct {
	Param        askParam
	Value        int
	FromQuestion bool
}

// askParamIssue is a question that cannot be run as it stands: it says what to ask the analyst, or which bound was passed.
type askParamIssue struct {
	Kind    string // "clarify" or "range"
	Message string
}

// ---- how the numbers of the other rules are found in a question (lower case). Each finder returns the values it finds, in the unit of its parameter.

var (
	askDaysOrWeeks = `(\d{1,3})\s*(?:-\s*)?(days?|weeks?)`
	askSilenceA    = regexp.MustCompile(`(?:silent|inactive|idle|dormant|dark|quiet|stopped|went quiet|no (?:activity|events|calls|records|use))\s+(?:for\s+|during\s+)?(?:at least\s+|more than\s+|over\s+|longer than\s+|a minimum of\s+)?` + askDaysOrWeeks)
	askSilenceB    = regexp.MustCompile(askDaysOrWeeks + `\s+(?:of\s+)?(?:silence|inactivity|no activity|without (?:any )?(?:activity|events|calls))`)
	askWindowA     = regexp.MustCompile(`(?:started|start|began|begin|appeared|appears|first (?:event|activity|call|seen)|new number|successor|replacement)\s+(?:[a-z,]+\s+){0,6}?within\s+` + askDaysOrWeeks)
	askWindowB     = regexp.MustCompile(`within\s+` + askDaysOrWeeks + `\s+(?:after|of)\s+(?:it|the old number|the number|that number|they|the silent number)?\s*(?:went silent|stopped|last (?:event|activity|call)|going silent)`)
	askOverlapA    = regexp.MustCompile(`started\s+(?:[a-z,]+\s+){0,4}?(?:up to\s+|as much as\s+)?` + askDaysOrWeeks + `\s+(?:before|earlier than|prior to)\b`)
	askHistoryA    = regexp.MustCompile(`(?:last|first|previous)\s+` + askDaysOrWeeks + `\s+of\s+(?:its |their |the )?(?:contacts?|activity|calls?|records|places|cells?)`)
	askNewAfterA   = regexp.MustCompile(`(?:not|never)\s+(?:in use|active|used|seen|present|around)\s+(?:[a-z,]+\s+){0,4}?first\s+` + askDaysOrWeeks)
	askScoreA      = regexp.MustCompile(`score\D{0,20}?(?:of\s+)?(?:at least|above|over|more than|greater than|minimum of|min|>=?)\s*(\d*\.\d+|\d{1,3})\s*(%)?`)
	askScoreB      = regexp.MustCompile(`(\d{1,3})\s*%\s*(?:or more\s+)?(?:similar|similarity|match)`)
	askMinEventsA  = regexp.MustCompile(`(?:at least|minimum of|min(?:imum)?|no fewer than|not fewer than)\s*(\d{1,3})\s+(?:events|calls|records)\b`)
	askHoldWeeksA  = regexp.MustCompile(`(?:for|held|holds?|holding|stayed|stays|lasting|lasts|at least|least|minimum of|min(?:imum)?)\s+(\d{1,2})\s*(?:-\s*)?(?:consecutive\s+|straight\s+|full\s+|whole\s+)?weeks?`)
	askHoldWeeksB  = regexp.MustCompile(`(\d{1,2})\s*-\s*week\b`)
	askSpeedA      = regexp.MustCompile(`(\d{2,4})\s*(?:km\s*/\s*h(?:r|our)?|kmh|kph|km per hour|kilomet(?:re|er)s? (?:an|per) hour)`)
	askSideDaysA   = regexp.MustCompile(`(\d{1,3})\s+days?\s+(?:on each side|on either side|either side|each side|on both sides|before and after)`)
	askRatioTimes  = regexp.MustCompile(`(\d+(?:\.\d+)?)\s*(?:x|times)\b`)
	askRatioTwice  = regexp.MustCompile(`\b(?:double|doubled|doubling|twice|two-fold|twofold)\b`)
	askRatioTriple = regexp.MustCompile(`\b(?:triple|tripled|tripling|thrice|three-fold|threefold)\b`)
)

// askDaysFrom reads the days of the first group pair of a match (a number and "days" or "weeks").
func askDaysFrom(match []string, numberAt int) int {
	n, err := strconv.Atoi(match[numberAt])
	if err != nil {
		return -1
	}
	if strings.HasPrefix(match[numberAt+1], "week") {
		return n * 7
	}
	return n
}

func askFindAllDays(lower string, patterns ...*regexp.Regexp) []int {
	var out []int
	for _, re := range patterns {
		for _, m := range re.FindAllStringSubmatch(lower, -1) {
			if v := askDaysFrom(m, 1); v >= 0 {
				out = append(out, v)
			}
		}
	}
	return out
}

func askFindSilenceDays(lower string) []int       { return askFindAllDays(lower, askSilenceA, askSilenceB) }
func askFindSuccessorWindow(lower string) []int   { return askFindAllDays(lower, askWindowA, askWindowB) }
func askFindSuccessorOverlap(lower string) []int  { return askFindAllDays(lower, askOverlapA) }
func askFindSuccessorHistory(lower string) []int  { return askFindAllDays(lower, askHistoryA) }
func askFindSuccessorNewAfter(lower string) []int { return askFindAllDays(lower, askNewAfterA) }

// askFindScorePercent reads a similarity score given as 0.5, .5, 50, 50% or "50% similar" as a percent.
func askFindScorePercent(lower string) []int {
	var out []int
	for _, m := range askScoreA.FindAllStringSubmatch(lower, -1) {
		f, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			continue
		}
		switch {
		case m[2] == "%":
			out = append(out, int(f+0.5))
		case f <= 1:
			out = append(out, int(f*100+0.5))
		default:
			out = append(out, int(f+0.5))
		}
	}
	for _, m := range askScoreB.FindAllStringSubmatch(lower, -1) {
		if n, err := strconv.Atoi(m[1]); err == nil {
			out = append(out, n)
		}
	}
	return out
}

func askFindInts(lower string, patterns ...*regexp.Regexp) []int {
	var out []int
	for _, re := range patterns {
		for _, m := range re.FindAllStringSubmatch(lower, -1) {
			if n, err := strconv.Atoi(m[1]); err == nil {
				out = append(out, n)
			}
		}
	}
	return out
}

func askFindAnchorMinEvents(lower string) []int { return askFindInts(lower, askMinEventsA) }
func askFindHoldWeeks(lower string) []int       { return askFindInts(lower, askHoldWeeksA, askHoldWeeksB) }
func askFindSpeedKmh(lower string) []int        { return askFindInts(lower, askSpeedA) }
func askFindRhythmSideDays(lower string) []int  { return askFindInts(lower, askSideDaysA) }

// askFindRhythmRatio reads "double", "twice", "triple" and "N times" as the least ratio of the two daily means, in percent.
func askFindRhythmRatio(lower string) []int {
	var out []int
	if askRatioTwice.MatchString(lower) {
		out = append(out, 200)
	}
	if askRatioTriple.MatchString(lower) {
		out = append(out, 300)
	}
	for _, m := range askRatioTimes.FindAllStringSubmatch(lower, -1) {
		if f, err := strconv.ParseFloat(m[1], 64); err == nil && f >= 1 && f <= 100 {
			out = append(out, int(f*100+0.5))
		}
	}
	return out
}

var (
	askMinuteNumber = regexp.MustCompile(`\b(\d{1,4}|one|two|three|four|five|six|seven|eight|nine|ten|twelve|fifteen|twenty|thirty|forty[- ]five|forty|fifty|sixty|ninety)(?:\s|-)?(?:minutes?|mins?)\b`)
	askHalfHour     = regexp.MustCompile(`\bhalf (?:an |a )?hour\b`)
	askQuarterHour  = regexp.MustCompile(`\ba quarter(?: of an| of a)? hour\b|\bquarter(?: of an| of a)? hour\b`)
	askGapBefore    = regexp.MustCompile(`(?:previous|preceding|prior|last|past|earlier|before)(?:\s+\w+){0,2}\s*$`)
	askGapAfter     = regexp.MustCompile(`^\s*(?:before|earlier|prior|ahead|ago)\b`)
	askHourGap      = regexp.MustCompile(`\b(?:previous|preceding|prior|last|past)\s+hour\b|\bhour\s+(?:before|earlier|prior)\b|\bthe hour before\b`)
	askMinuteWord   = regexp.MustCompile(`\bmin(?:ute)?s?\b`)
)

var askMinuteWords = map[string]int{"one": 1, "two": 2, "three": 3, "four": 4, "five": 5, "six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10, "twelve": 12,
	"fifteen": 15, "twenty": 20, "thirty": 30, "forty": 40, "forty five": 45, "forty-five": 45, "fifty": 50, "sixty": 60, "ninety": 90}

// askMinuteSpan is one length of time in minutes named in the question and which number of an analysis it is.
type askMinuteSpan struct {
	Minutes int
	Role    string // "gap" when it sits next to "previous", "before", "earlier"...; otherwise "window"
}

// askMinuteSpans finds every length of time of the question (digits or a few number words followed by minutes; half an hour; a quarter of an hour; the previous hour). A length
// that sits next to "previous", "preceding", "before", "earlier", "ago" is the gap before a window; any other is the window itself.
func askMinuteSpans(lower string) []askMinuteSpan {
	var spans []askMinuteSpan
	type found struct {
		start, end, minutes int
	}
	var hits []found
	for _, m := range askMinuteNumber.FindAllStringSubmatchIndex(lower, -1) {
		token := lower[m[2]:m[3]]
		value, ok := askMinuteWords[token]
		if !ok {
			n, err := strconv.Atoi(token)
			if err != nil {
				continue
			}
			value = n
		}
		hits = append(hits, found{m[0], m[1], value})
	}
	for _, m := range askHalfHour.FindAllStringIndex(lower, -1) {
		hits = append(hits, found{m[0], m[1], 30})
	}
	for _, m := range askQuarterHour.FindAllStringIndex(lower, -1) {
		hits = append(hits, found{m[0], m[1], 15})
	}
	for _, m := range askHourGap.FindAllStringIndex(lower, -1) {
		hits = append(hits, found{m[0], m[1], 60})
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].start < hits[j].start })
	for _, h := range hits {
		before := lower[max(0, h.start-30):h.start]
		after := lower[h.end:min(len(lower), h.end+16)]
		role := "window"
		if askGapBefore.MatchString(before) || askGapAfter.MatchString(after) || askHourGap.MatchString(lower[h.start:h.end]) {
			role = "gap"
		}
		spans = append(spans, askMinuteSpan{Minutes: h.minutes, Role: role})
	}
	return spans
}

// askReadParams reads every declared parameter from the question (already lower-cased).
func askReadParams(lower string) map[string]askParamRead {
	spans := askMinuteSpans(lower)
	out := map[string]askParamRead{}
	for _, p := range askParams {
		read := askParamRead{}
		var named []int
		if p.Find != nil {
			named = p.Find(lower)
		} else {
			for _, s := range spans {
				if s.Role == p.Role {
					named = append(named, s.Minutes)
				}
			}
		}
		for _, v := range named {
			read.Found = true
			if len(read.Values) > 0 && read.Values[len(read.Values)-1] != v {
				read.Ambiguous = true
			}
			if !askContainsInt(read.Values, v) {
				read.Values = append(read.Values, v)
			}
		}
		if read.Found {
			read.Value = read.Values[0]
		}
		out[p.Name] = read
	}
	return out
}

func askContainsInt(list []int, n int) bool {
	for _, x := range list {
		if x == n {
			return true
		}
	}
	return false
}

// askParamsFor returns the parameters the views of a query read (in declaration order).
func askParamsFor(views []string) []askParam {
	var out []askParam
	for _, p := range askParams {
		for _, v := range views {
			if askContains(p.Views, v) {
				out = append(out, p)
				break
			}
		}
	}
	return out
}

// askResolveParams decides the value of each parameter the views read: the question's number when it names one within the bounds, the default when it names none. It refuses
// (an issue, which the engine turns into a question or a refusal, never a guess) when the question names the same parameter with different values, names minutes without a
// number for the window, or names a value outside the bounds.
func askResolveParams(f askFrame, views []string) ([]askParamUse, *askParamIssue) {
	var uses []askParamUse
	for _, p := range askParamsFor(views) {
		read := f.Params[p.Name]
		switch {
		case read.Ambiguous:
			parts := make([]string, 0, len(read.Values))
			for _, v := range read.Values {
				parts = append(parts, strconv.Itoa(v))
			}
			return nil, &askParamIssue{Kind: "clarify", Message: fmt.Sprintf("The question names more than one %s (%s %s). Which one should I use?", strings.ToLower(p.Label), strings.Join(parts, " and "), p.Unit)}
		case read.Found && (read.Value < p.Min || read.Value > p.Max):
			return nil, &askParamIssue{Kind: "range", Message: fmt.Sprintf("The %s can be from %d to %d %s; the question asks for %d.", strings.ToLower(p.Label), p.Min, p.Max, p.Unit, read.Value)}
		case read.Found:
			uses = append(uses, askParamUse{Param: p, Value: read.Value, FromQuestion: true})
		case p.Role == "window" && p.Find == nil && f.Cues["minutes_without_number"]:
			return nil, &askParamIssue{Kind: "clarify", Message: fmt.Sprintf("The question speaks of minutes without saying how many. How many %s should the %s be (%d to %d)?", p.Unit, strings.ToLower(p.Label), p.Min, p.Max)}
		default:
			uses = append(uses, askParamUse{Param: p, Value: p.Default})
		}
	}
	return uses, nil
}

// askParamSettings is the transaction settings of the resolved parameters: only names declared in askParams, only integers inside the bounds.
func askParamSettings(uses []askParamUse) map[string]string {
	out := map[string]string{}
	for _, u := range uses {
		if u.Value >= u.Param.Min && u.Value <= u.Param.Max {
			out[u.Param.Setting] = strconv.Itoa(u.Value)
		}
	}
	return out
}

// askKnownSetting is whether a setting name is one a declared parameter owns: the executor sets nothing else from the limits.
func askKnownSetting(name string) bool {
	for _, p := range askParams {
		if p.Setting == name {
			return true
		}
	}
	return false
}

// askParamNotes is the line of the answer that says which values were used and where each came from.
func askParamNotes(uses []askParamUse) []string {
	var out []string
	for _, u := range uses {
		if u.Param.Quiet && !u.FromQuestion {
			continue
		}
		source := "the default, because the question names none"
		if u.FromQuestion {
			source = "read from your question"
		}
		out = append(out, fmt.Sprintf("%s used: %d %s (%s).", u.Param.Label, u.Value, u.Param.Unit, source))
	}
	return out
}
