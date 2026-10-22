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
	Name    string   // key in the frame
	Setting string   // the transaction setting the views read
	Label   string   // what the answer calls it
	Unit    string   // "minutes"
	Default int
	Min     int
	Max     int
	Views   []string // the views that read the setting
	Role    string   // "window" or "gap": which number of the question it is (see askMinuteSpans)
}

var askParams = []askParam{
	{Name: "colocation_minutes", Setting: "app.colocation_minutes", Label: "Co-location window", Unit: "minutes", Default: 15, Min: 1, Max: 180,
		Views: []string{"v_colocation_windows", "v_meeting_candidates"}, Role: "window"},
	{Name: "contact_gap_minutes", Setting: "app.contact_gap_minutes", Label: "Longest gap between a contact and the window", Unit: "minutes", Default: 60, Min: 1, Max: 180,
		Views: []string{"v_meeting_candidates"}, Role: "gap"},
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
		for _, s := range spans {
			if s.Role != p.Role {
				continue
			}
			read.Found = true
			if len(read.Values) > 0 && read.Values[len(read.Values)-1] != s.Minutes {
				read.Ambiguous = true
			}
			if !askContainsInt(read.Values, s.Minutes) {
				read.Values = append(read.Values, s.Minutes)
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
		case p.Role == "window" && f.Cues["minutes_without_number"]:
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
		source := "the default, because the question names none"
		if u.FromQuestion {
			source = "read from your question"
		}
		out = append(out, fmt.Sprintf("%s used: %d %s (%s).", u.Param.Label, u.Value, u.Param.Unit, source))
	}
	return out
}
