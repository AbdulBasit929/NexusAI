package main

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// ASK ENGINE V2: PLATES AS IDENTIFIERS OF THE QUESTION (docs/data/anpr-typed-family-20261026.md, piece f).
//
// A plate named in a question is an identifier like a number: the query must compare it, exactly, and no other. It is compared in its SEARCH KEY (forensic.normalize_plate of migration
// 012: Unicode NFKC, Arabic-Indic digits to ASCII, separators removed, upper case), never in the form the camera wrote: LEA-1234 is 'LEA1234' in plate_norm. The key never makes two
// different plates equal (O is not 0), so a look-alike plate is another plate and is not returned.
//
// Finding a plate in free text is the risky part (a month and a year, a word and a number, a camera id look like one), so a candidate is taken only when it is written as plates are
// (capitals) or follows a word that introduces one (plate, vehicle, car, registration, ANPR, licence), is not an ordinary word or a month, and a four-digit number that could be a year
// needs the introducing word.

var (
	// letters (2 to 5), an optional separator, an optional one or two digit group (a series year), then three or four digits
	askPlateRE  = regexp.MustCompile(`\b([A-Za-z]{2,5})[ \t-]?((?:\d{1,2}[ \t-]?)?\d{3,4})\b`)
	askPlateCue = regexp.MustCompile(`(?i)\b(?:plates?|vehicles?|cars?|registrations?|reg|anpr|licen[cs]es?)(?:\s+number)?\s*(?:of|is|was|for|:)?\s*$`)
	// a plate named after a verb of naming is a plate even in lower case: "list the documents that name synt 5008" (found by the documents run on dev seed 5100)
	askPlateNameCue = regexp.MustCompile(`(?i)\b(?:names?|named|naming|mentions?|mentioned|contains?|containing|lists?|shows?|writes?|written|bears?|bearing|carr(?:y|ies)|reads?|without|except|excluding|also|not|than)\s+(?:the\s+|a\s+)?(?:plate\s+|vehicle\s+)?(?:number\s+)?$`)
	// a second plate of a question that names things is a plate in lower case too: "documents that mention both PK00 ... and synt 4024" (found by the live smoke of 2026-10-30, DOC-013)
	askPlateAndCue = regexp.MustCompile(`(?i)\b(?:and|or|nor|both|either)\s+(?:the\s+|a\s+)?(?:plate\s+|vehicle\s+)?$`)
	askPlateNaming = regexp.MustCompile(`(?i)\b(?:names?|named|naming|mentions?|mentioned|contains?|containing|writes?|written|bears?|bearing)\b`)
	askYearLike    = regexp.MustCompile(`^(?:19|20)\d{2}$`)
	// a plate that looks like another (an O for a 0, an I for a 1): capitals only, and at least two real digits and one look-alike letter among the number's characters
	askPlateLookalikeRE = regexp.MustCompile(`\b([A-Z]{3,5})[ \t-]?([0-9OI]{3,4})\b`)
)

// askPlateStop are the words that are never the letters of a plate in a question: months and days, the words that stand before a number in a question, a camera or a lane.
var askPlateStop = map[string]bool{}

func init() {
	for _, w := range []string{
		"in", "on", "of", "at", "by", "to", "is", "it", "as", "an", "or", "if", "so", "up", "no", "we", "me", "my", "do", "be",
		"cam", "camera", "cameras", "lane", "lanes", "cell", "cells", "site", "tower", "id", "num", "number", "numbers", "top", "last", "first", "next", "previous", "only", "over", "under",
		"since", "from", "until", "before", "after", "during", "between", "about", "more", "than", "case", "call", "calls", "sms", "event", "events", "day", "days", "week", "weeks", "hour",
		"hours", "min", "mins", "minutes", "km", "kmh", "limit", "year", "years", "month", "months", "within", "around", "near", "visit", "visits", "time", "times", "rows", "row",
		"january", "february", "march", "april", "may", "june", "july", "august", "september", "october", "november", "december", "jan", "feb", "mar", "apr", "jun", "jul", "aug", "sep",
		"page", "pages", "section", "para", "paragraph", "ward", "sector", "block", "street", "road", "house", "flat", "room", "line", "item", "ref", "reference",
		"sept", "oct", "nov", "dec", "monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday", "mon", "tue", "tues", "wed", "thu", "thur", "thurs", "fri", "sat", "sun",
	} {
		askPlateStop[w] = true
	}
}

// askNormalizePlate is forensic.normalize_plate (migration 012) in Go; the two must agree on every token (a spec runs the Python port and the SQL function on one fixture, and this one on
// the same list).
func askNormalizePlate(raw string) string {
	var b strings.Builder
	for _, r := range norm.NFKC.String(raw) {
		switch {
		case r >= 0x0660 && r <= 0x0669:
			b.WriteRune('0' + (r - 0x0660))
		case r >= 0x06F0 && r <= 0x06F9:
			b.WriteRune('0' + (r - 0x06F0))
		case strings.ContainsRune("·•‐‑‒–—―−", r):
			// deleted, as `translate` deletes the characters that have no target
		case r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\f' || r == '\v' || r == '-' || r == '_' || r == '.' || r == '/':
			// a separator
		default:
			b.WriteRune(unicode.ToUpper(r))
		}
	}
	return b.String()
}

// askFindPlates takes the plates out of the text: each is replaced by hold(key) and returned with the way it was written. The text is returned with the replacements.
func askFindPlates(text string, hold func(string) string) (string, []string, map[string]string) {
	text, keys, written := askFindPlatesBy(text, hold)
	text, more, extra := askFindLookalikePlates(text, hold)
	if len(more) > 0 {
		if written == nil {
			written = map[string]string{}
		}
		for _, key := range more {
			if _, seen := written[key]; !seen {
				keys = append(keys, key)
				written[key] = extra[key]
			}
		}
	}
	return text, keys, written
}

// askFindLookalikePlates takes out the plates written with an O or an I among their digits (the plates the generator plants to tell a search key from a guess).
func askFindLookalikePlates(text string, hold func(string) string) (string, []string, map[string]string) {
	var keys []string
	written := map[string]string{}
	var out strings.Builder
	last := 0
	for _, m := range askPlateLookalikeRE.FindAllStringSubmatchIndex(text, -1) {
		letters, number := text[m[2]:m[3]], text[m[4]:m[5]]
		digits := strings.Count(number, "0") + strings.Count(number, "1") + strings.Count(number, "2") + strings.Count(number, "3") + strings.Count(number, "4") +
			strings.Count(number, "5") + strings.Count(number, "6") + strings.Count(number, "7") + strings.Count(number, "8") + strings.Count(number, "9")
		if askPlateStop[strings.ToLower(letters)] || digits < 2 || digits == len(number) || strings.Trim(number, "OI") == "" {
			continue
		}
		key := askNormalizePlate(text[m[0]:m[1]])
		if _, seen := written[key]; !seen {
			keys = append(keys, key)
			written[key] = strings.TrimSpace(text[m[0]:m[1]])
		}
		out.WriteString(text[last:m[0]])
		out.WriteString(hold(key))
		last = m[1]
	}
	if last == 0 {
		return text, nil, nil
	}
	out.WriteString(text[last:])
	return out.String(), keys, written
}

func askFindPlatesBy(text string, hold func(string) string) (string, []string, map[string]string) {
	var keys []string
	written := map[string]string{}
	var out strings.Builder
	last := 0
	for _, m := range askPlateRE.FindAllStringSubmatchIndex(text, -1) {
		start, end := m[0], m[1]
		letters, digits := text[m[2]:m[3]], text[m[4]:m[5]]
		before := text[max(0, start-40):start]
		cue := askPlateCue.MatchString(before) || askPlateNameCue.MatchString(before) || (askPlateAndCue.MatchString(before) && askPlateNaming.MatchString(text[:start]))
		if askPlateStop[strings.ToLower(letters)] {
			continue
		}
		if letters != strings.ToUpper(letters) && !cue {
			continue
		}
		only := strings.NewReplacer(" ", "", "\t", "", "-", "").Replace(digits)
		// a number that could be a year (found by the G4 run on case 115: SYNT-2020 was not read as a plate and the question was refused) is a plate when it is joined to its capitals by a
		// dash or follows four or more capital letters; "ABC 2024" and "MARCH 2026" still need the word that introduces a plate, or are a month
		joined := strings.Contains(text[m[3]:m[4]], "-") || len(letters) >= 4
		if len(only) == 4 && askYearLike.MatchString(only) && !cue && !joined {
			continue
		}
		key := askNormalizePlate(text[start:end])
		if _, seen := written[key]; !seen {
			keys = append(keys, key)
			written[key] = strings.TrimSpace(text[start:end])
		}
		out.WriteString(text[last:start])
		out.WriteString(hold(key))
		last = end
	}
	if last == 0 {
		return text, nil, nil
	}
	out.WriteString(text[last:])
	return out.String(), keys, written
}

// askPlateObligations is what a query owes the plates of the question: each is compared, as its search key, and the query names no plate the question did not.
func askPlateObligations(f askFrame, texts []string, sql string, add func(kind, message, reason string)) {
	asked := map[string]bool{}
	for _, key := range f.Plates {
		asked[strings.ToLower(key)] = true
		found := false
		for _, t := range texts {
			found = found || strings.EqualFold(strings.TrimSpace(t), key)
		}
		if !found {
			add("IDENTIFIER", fmt.Sprintf("The question names the plate %s. Compare plate_norm with '%s' exactly (the plate in capitals, without separators); never compare plate_raw.", f.PlateWritten[key], key), "the question names a plate that the query does not compare")
		}
	}
	for _, re := range askPlateConstants {
		for _, m := range re.FindAllStringSubmatch(sql, -1) {
			for _, c := range strings.Split(m[1], ",") {
				constant := strings.Trim(strings.TrimSpace(c), "'")
				if constant != "" && !asked[strings.ToLower(constant)] {
					add("FOREIGN_IDENTIFIER", fmt.Sprintf("The query names the plate %s, which the question does not mention. Use only the plates of the question.", strings.ToUpper(constant)), "the query names a plate that the question does not")
				}
			}
		}
	}
}

// askCountsSightings finds a question that asks how many times a plate was seen (the number of sightings), not how many cameras or plates.
var askCountsSightings = regexp.MustCompile(`\b(?:how many times|how many sightings|how often|number of (?:times|sightings)|times (?:at|by) each|times each|sightings (?:at|by) each|sightings each)\b`)

// askBareSightings finds the sightings column selected as itself; an alias of that name (count(*) AS sightings) is not the column, and the model's alias is what hid the defect.
var askBareSightings = regexp.MustCompile(`(?:select|,)\s*(?:distinct\s+)?(?:\w+\.)?sightings\s*(?:,|\bfrom\b)`)

// askPlateConstants find the constants a query compares with the plate column, in the parser's lower-case canonical text.
var askPlateConstants = []*regexp.Regexp{
	regexp.MustCompile(`plate_norm\s*(?:=|<>|!=)\s*('[^']*')`),
	regexp.MustCompile(`plate_norm\s+(?:not\s+)?in\s*\(([^)]*)\)`),
}

// askAbsenceOf is the sentence for identifiers that occur nowhere in the case. A plate is said as the question wrote it, and as a plate: no camera sighting holds it.
func askAbsenceOf(absent []string, f askFrame) string {
	shown := make([]string, 0, len(absent))
	plates := 0
	for _, id := range absent {
		if written, ok := f.PlateWritten[id]; ok {
			shown = append(shown, written)
			plates++
			continue
		}
		shown = append(shown, id)
	}
	switch {
	case plates == len(absent):
		return "No record of this case holds " + askJoinPlates(shown) + " in any camera sighting."
	case plates > 0:
		return "No record of this case holds " + strings.Join(shown, ", ") + " in any number, device, SIM, address, cell or plate column."
	}
	return "No record of this case holds " + strings.Join(shown, ", ") + " in any number, device, SIM, address or cell column."
}

func askJoinPlates(shown []string) string {
	if len(shown) == 1 {
		return "the plate " + shown[0]
	}
	return "the plates " + strings.Join(shown, ", ")
}

// askQueryConstants are the text constants a query compares with a column, with the case the query wrote (the validated text keeps it).
func askQueryConstants(sql, column string) []string {
	var out []string
	eq := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(column) + `\s*=\s*'([^']*)'`)
	in := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(column) + `\s+in\s*\(([^)]*)\)`)
	for _, m := range eq.FindAllStringSubmatch(sql, -1) {
		out = append(out, m[1])
	}
	for _, m := range in.FindAllStringSubmatch(sql, -1) {
		for _, c := range strings.Split(m[1], ",") {
			if c = strings.Trim(strings.TrimSpace(c), "'"); c != "" {
				out = append(out, c)
			}
		}
	}
	return out
}

// askPlateAbsence says what a search of the camera sightings required when it found no row: the plate, the camera, the period.
func askPlateAbsence(v *askValidated, f askFrame) string {
	who := ""
	switch len(f.Plates) {
	case 0:
	case 1:
		who = "the plate " + f.PlateWritten[f.Plates[0]]
	default:
		written := make([]string, 0, len(f.Plates))
		for _, key := range f.Plates {
			written = append(written, f.PlateWritten[key])
		}
		who = "the plates " + strings.Join(written, ", ")
	}
	where := ""
	if cameras := askQueryConstants(v.SQL, "camera_id"); len(cameras) == 1 {
		where = " at camera " + cameras[0]
	} else if len(cameras) > 1 {
		where = " at cameras " + strings.Join(cameras, ", ")
	}
	// "which plates were seen at both camera A and camera B" is a search for plates at several cameras: the sentence says that, not that a camera saw nothing
	if cameras := askQueryConstants(v.SQL, "camera_id"); who == "" && len(cameras) > 1 && strings.Contains(strings.ToLower(v.SQL), "count(distinct camera_id)") {
		return "no plate was seen at all of the cameras " + strings.Join(cameras, ", ") + askPeriodPhrase(f)
	}
	switch {
	case who != "":
		return "no sighting of " + who + where + askPeriodPhrase(f) + " was found"
	case where != "":
		return "no sighting" + where + askPeriodPhrase(f) + " was found"
	}
	return "no sighting of a plate" + askPeriodPhrase(f) + " was found"
}
