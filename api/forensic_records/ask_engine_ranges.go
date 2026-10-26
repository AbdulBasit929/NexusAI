package main

import (
	"regexp"
	"strings"
	"time"
)

// ASK ENGINE V2: A RANGE OF DAYS THAT INCLUDES ITS LAST DAY (found by the ANPR development check on case 2: "from 2026-02-10 through 2026-03-13 (local time)" was answered with
// observed_utc < '2026-03-13 00:00:00+05', which leaves the whole of the 13th out and counted 9 sightings where the records hold 12).
//
// When the question says that the last day is in (through, up to and including, both days included, inclusive), the query's upper bound is the start of the NEXT day, or an end-of-day
// time on the last day. A question that merely says "between D1 and D2" is left alone: it is ambiguous, and the answer states the dates used.

var (
	askThroughDateRE = regexp.MustCompile(`\b(?:through|thru|up to and including|until and including|to and including|and including)\s+(?:the end of\s+)?(\d{4}-\d{2}-\d{2})\b`)
	askBothIncluded  = regexp.MustCompile(`\bboth (?:days|dates|ends)(?: are| were)? included\b|\binclusive\b|\bincluding both\b`)
)

// askInclusiveEnd is the last day of a range that the question says is included ("" when it says nothing of the kind).
func askInclusiveEnd(lowerQuestion string, dates []string) string {
	if m := askThroughDateRE.FindStringSubmatch(lowerQuestion); m != nil {
		for _, d := range dates {
			if d == m[1] {
				return d
			}
		}
	}
	if len(dates) >= 2 && askBothIncluded.MatchString(lowerQuestion) {
		last := dates[0]
		for _, d := range dates {
			if d > last {
				last = d
			}
		}
		return last
	}
	return ""
}

// askNextDay is the day after an ISO date ("" when it is not one).
func askNextDay(day string) string {
	t, err := time.Parse("2006-01-02", day)
	if err != nil {
		return ""
	}
	return t.AddDate(0, 0, 1).Format("2006-01-02")
}

// askDateConstant is whether a text constant of the query bounds the day from above or at it: the day itself, the start of the next day, or an end-of-day time of the day.
func askDateConstant(texts []string, day string, inclusiveEnd bool) bool {
	next := ""
	if inclusiveEnd {
		next = askNextDay(day)
	}
	for _, t := range texts {
		t = strings.TrimSpace(t)
		if strings.HasPrefix(t, day) || (next != "" && strings.HasPrefix(t, next)) {
			return true
		}
	}
	return false
}
