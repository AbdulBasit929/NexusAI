package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ASK ENGINE V2: THE FRAME OF A QUESTION AND THE OBLIGATIONS (roadmap 6.8.6, checks 1 and 2).
//
// The defect that matters is a confident answer to a different question. The validator (ask_engine_validate.go) judges that a query is SAFE; it cannot judge that
// it is the query this question asked for. So the question is read ONCE, by code, into a frame of what it states (the numbers, devices and addresses it names, its
// dates and months, how many it wants, how many hops, the direction, the kind of event, the time of day, whether it counts, ranks or wants a first or last), and
// the validated query is held against that frame:
//
//   - every identifier the question names must appear in the query as a constant, and the query may name no identifier the question did not (a model that copies the
//     number of a retrieved example into its query gives a confident answer about somebody else);
//   - every date, month and year the question names must be in the query; a relative time ("last week") cannot be resolved without a stated reference day, so the
//     question is declined before any model time is spent;
//   - a limit or a hop count the question states is the query's limit or hop bound; a connection question is a recursive query;
//   - a direction, a kind of event, a night, a count, a ranking, a distinct count and the services-and-hubs rule each leave a mark that the query must carry.
//
// This is "back-translation by code": the plain-words restatement of the query (ask_engine_describe.go) is generated from the parse tree, so it cannot say
// something the query does not do, and the frame is read from the question, so the comparison needs no second model call, which on the CPU laptop would cost tens
// of seconds (roadmap 6.8.13). Every rule below is generic: a new view needs a description, not a new rule.

type askMonth struct {
	Year  int // 0 when the question names no year
	Month int
}

type askFrame struct {
	Question string // the question with every number written in its canonical form (923001234567)
	Numbers  []string
	IMEIs    []string
	// Serials are the SIM serials (ICCID, 18 to 20 digits) and the TACs (eight digits after the word TAC) the question names: identifiers of the registration and handset views.
	Serials []string
	IPs     []string
	Cells   []string
	// Plates are the search keys of the plates the question names (askNormalizePlate), PlateWritten how the question wrote each (ask_engine_plates.go).
	Plates       []string
	PlateWritten map[string]string
	Dates        []string // ISO yyyy-mm-dd
	Months       []askMonth
	Years        []int
	Limit        int // the N of "top five", "first 10"; 0 when none
	Hops         int
	Cues         map[string]bool
	Relative     string // the relative time words found ("last week"), which a query cannot resolve without a reference day
	// Params is what the question says about each runtime parameter (ask_engine_params.go); ParamUses is what will be used for the query, set by the run.
	Params    map[string]askParamRead
	ParamUses []askParamUse
}

// askUnitFollows is whether the text right after position end starts with a unit of time or length.
func askUnitFollows(lower string, end int) bool {
	return end <= len(lower) && askUnitAfter.MatchString(lower[end:])
}

// Identifiers are every identifier of the frame in the order they were written.
func (f askFrame) Identifiers() []string {
	out := append([]string{}, f.Numbers...)
	out = append(out, f.IMEIs...)
	out = append(out, f.Serials...)
	out = append(out, f.IPs...)
	out = append(out, f.Cells...)
	return append(out, f.Plates...)
}

var (
	askFramePKPhone  = regexp.MustCompile(`(?:\+?\s?92[\s-]?|\b0)?3\d{2}[\s-]?\d{7}\b`)
	askFrameIntl     = regexp.MustCompile(`\+\d{1,3}[\s-]?\d{6,12}\b`)
	askFrameIMEI     = regexp.MustCompile(`\b\d{15}\b`)
	askFrameICCID    = regexp.MustCompile(`\b\d{18,20}\b`)
	askFrameTAC      = regexp.MustCompile(`(?i)\btac\b(?:\s+(?:code|number))?[^0-9a-z]{0,12}(\d{8})\b`)
	askFrameBare     = regexp.MustCompile(`\b\d{10,14}\b`)
	askFrameYear     = regexp.MustCompile(`\b(20\d{2})\b`)
	askFrameMonths   = regexp.MustCompile(`\b(january|february|march|april|may|june|july|august|september|october|november|december|jan|feb|mar|apr|jun|jul|aug|sept|sep|oct|nov|dec)\b(?:\s+(?:of\s+)?(20\d{2}))?`)
	askFrameHops     = regexp.MustCompile(`\b(\d+|one|two|three|four|five)\s+(?:hops?|steps?|links?|degrees?)\b`)
	askFrameRelative = regexp.MustCompile(`\b(?:yesterday|today|tonight|this (?:week|month|year)|last (?:week|month|year|night|weekend)|past (?:\d+|few|several|one|two|three|four|five|six|seven|ten|twelve) (?:days?|weeks?|months?|years?|hours?)|(?:last|previous) (?:\d+|few|several|one|two|three|four|five|six|seven|ten|twelve) (?:days?|weeks?|months?|years?|hours?)|recent(?:ly)?|these days|so far this)\b`)
	askUnitAfter     = regexp.MustCompile(`^\s*(?:-\s*)?(?:minutes?|mins?|seconds?|secs?|hours?|hrs?|days?|weeks?|months?|years?|km|kilomet\w*|met(?:re|er)s?)\b`)
	askFrameTopN     = regexp.MustCompile(`\b(?:top|first|last|bottom|latest|earliest|most|busiest|highest|largest|biggest|longest|shortest|lowest|oldest|newest)\s+(\d+|one|two|three|four|five|six|seven|eight|nine|ten|twelve|fifteen|twenty)\b`)
	askFrameNTop     = regexp.MustCompile(`\b(\d+|one|two|three|four|five|six|seven|eight|nine|ten|twelve|fifteen|twenty)\s+(?:most|top|busiest|highest|largest|biggest|longest|frequent|common|active|recent|earliest|latest|people|persons|numbers|contacts|callers|cells|towers|sites|devices|handsets|apps|applications|hosts|domains|websites|addresses)\b`)
	askFrameWord     = regexp.MustCompile(`[a-z]+`)
)

var askNumberWords = map[string]int{"one": 1, "two": 2, "three": 3, "four": 4, "five": 5, "six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10, "twelve": 12, "fifteen": 15, "twenty": 20}

var askMonthNumber = map[string]int{
	"january": 1, "jan": 1, "february": 2, "feb": 2, "march": 3, "mar": 3, "april": 4, "apr": 4, "may": 5, "june": 6, "jun": 6, "july": 7, "jul": 7,
	"august": 8, "aug": 8, "september": 9, "sept": 9, "sep": 9, "october": 10, "oct": 10, "november": 11, "nov": 11, "december": 12, "dec": 12,
}

func askNumberOf(token string) int {
	if n, ok := askNumberWords[token]; ok {
		return n
	}
	n, err := strconv.Atoi(token)
	if err != nil {
		return 0
	}
	return n
}

// askCanonicalNumber writes a Pakistani mobile number the way the views store it (92 and ten digits). Anything else is returned as its digits.
func askCanonicalNumber(raw string) string {
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, raw)
	switch {
	case len(digits) == 12 && strings.HasPrefix(digits, "92"):
		return digits
	case len(digits) == 11 && strings.HasPrefix(digits, "03"):
		return "92" + digits[1:]
	case len(digits) == 10 && strings.HasPrefix(digits, "3"):
		return "92" + digits
	}
	return digits
}

// askExtractFrame reads a question. The identifiers are replaced in the returned Question by their canonical forms, so the model is shown, and the checks
// compare, one form of each.
// askCountAsConsider is "count" in the sense of "take into account", on the question's words joined by single spaces.
// askCountsEvents is "how many events / calls / messages ..." (the thing counted is an event, not a contact).
var askCountsEvents = regexp.MustCompile(`\bhow many (?:\w+ )?(?:events?|calls?|messages?|sms|texts?|records?)\b|\bnumber of (?:events?|calls?|messages?|sms|texts?|records?)\b|\btotal (?:events?|calls?|messages?)\b`)

var askCountAsConsider = regexp.MustCompile(`\b(?:only|not|never|don t|dont|do not)\s+count\b|\bcount(?:s|ed|ing)?\s+(?:\S+\s+){0,2}as\b|\bcount(?:s|ed|ing)?\s+(?:if|only|when|a\s+score|scores?)\b`)

func askExtractFrame(question string) askFrame { return askExtractFrameWith(question, true) }

// askExtractFrameWith reads the frame of a question; plates are looked for only when the catalogue the engine runs against has the plate views (a database without migration 027
// has none, and a capitalised word and a number must not become a plate it cannot answer about).
func askExtractFrameWith(question string, plates bool) askFrame {
	f := askFrame{Cues: map[string]bool{}}
	text := strings.TrimSpace(question)
	// Identifiers first, longest first, so that a date or an address is not taken for a number. Each found item is replaced by a placeholder and restored.
	var held []string
	hold := func(canonical string) string {
		held = append(held, canonical)
		return fmt.Sprintf(" \x00%d\x00 ", len(held)-1)
	}
	text = askIPv4RE.ReplaceAllStringFunc(text, func(s string) string { f.IPs = append(f.IPs, s); return hold(s) })
	text = askCellRE.ReplaceAllStringFunc(text, func(s string) string { f.Cells = append(f.Cells, strings.ToLower(s)); return hold(strings.ToLower(s)) })
	text = askDateRE.ReplaceAllStringFunc(text, func(s string) string {
		f.Dates = append(f.Dates, s[:10])
		return hold(s)
	})
	text = askFrameIMEI.ReplaceAllStringFunc(text, func(s string) string { f.IMEIs = append(f.IMEIs, s); return hold(s) })
	text = askFrameICCID.ReplaceAllStringFunc(text, func(s string) string { f.Serials = append(f.Serials, s); return hold(s) })
	text = askFrameTAC.ReplaceAllStringFunc(text, func(s string) string {
		m := askFrameTAC.FindStringSubmatch(s)
		f.Serials = append(f.Serials, m[1])
		return "TAC " + hold(m[1])
	})
	text = askFramePKPhone.ReplaceAllStringFunc(text, func(s string) string {
		c := askCanonicalNumber(s)
		if len(c) != 12 {
			return s
		}
		f.Numbers = append(f.Numbers, c)
		return hold(c)
	})
	text = askFrameIntl.ReplaceAllStringFunc(text, func(s string) string { c := askCanonicalNumber(s); f.Numbers = append(f.Numbers, c); return hold(c) })
	text = askFrameBare.ReplaceAllStringFunc(text, func(s string) string { c := askCanonicalNumber(s); f.Numbers = append(f.Numbers, c); return hold(c) })
	if plates {
		text, f.Plates, f.PlateWritten = askFindPlates(text, hold)
	}
	f.Numbers, f.IMEIs, f.IPs, f.Cells, f.Dates = askUnique(f.Numbers), askUnique(f.IMEIs), askUnique(f.IPs), askUnique(f.Cells), askUnique(f.Dates)
	f.Serials = askUnique(f.Serials)

	// The words, read from the text with the identifiers held back.
	lower := strings.ToLower(text)
	for _, m := range askFrameMonths.FindAllStringSubmatch(lower, -1) {
		name := m[1]
		year := 0
		if m[2] != "" {
			year, _ = strconv.Atoi(m[2])
		}
		if name == "may" && year == 0 && !regexp.MustCompile(`\b(?:in|of|during|on|since|until|before|after|from|through) may\b`).MatchString(lower) {
			continue
		}
		f.Months = append(f.Months, askMonth{Year: year, Month: askMonthNumber[name]})
	}
	named := map[int]bool{}
	for _, m := range f.Months {
		if m.Year != 0 {
			named[m.Year] = true
		}
	}
	for _, m := range askFrameYear.FindAllStringSubmatch(lower, -1) {
		year, _ := strconv.Atoi(m[1])
		if !named[year] {
			f.Years = append(f.Years, year)
			named[year] = true
		}
	}
	if m := askFrameHops.FindStringSubmatch(lower); m != nil {
		f.Hops = askNumberOf(m[1])
		f.Cues["path"] = true
	}
	f.Params = askReadParams(lower)
	f.Cues["minutes_without_number"] = len(askMinuteSpans(lower)) == 0 && askMinuteWord.MatchString(lower)
	if m := askFrameRelative.FindString(lower); m != "" {
		f.Relative = m
	}
	// "at most 22 minutes apart", "longest 30 seconds": a number followed by a unit of time or length is a length, not the N of a top list (found by the stage-3a development check,
	// where "at most 22 minutes" made the model write LIMIT 22).
	if m := askFrameTopN.FindStringSubmatchIndex(lower); m != nil && !askUnitFollows(lower, m[1]) {
		f.Limit = askNumberOf(lower[m[2]:m[3]])
	} else if m := askFrameNTop.FindStringSubmatchIndex(lower); m != nil && !askUnitFollows(lower, m[3]) {
		f.Limit = askNumberOf(lower[m[2]:m[3]])
	}
	if f.Limit > 100 {
		f.Limit = 0
	}
	words := map[string]bool{}
	for _, w := range askFrameWord.FindAllString(lower, -1) {
		words[w] = true
	}
	anyWord := func(list ...string) bool {
		for _, w := range list {
			if words[w] {
				return true
			}
		}
		return false
	}
	phrase := func(p string) bool {
		return strings.Contains(" "+strings.Join(askFrameWord.FindAllString(lower, -1), " ")+" ", " "+p+" ")
	}
	words2 := strings.Join(askFrameWord.FindAllString(lower, -1), " ")
	f.Cues["incoming"] = anyWord("incoming", "inbound", "received") || regexp.MustCompile(`\b(?:get|gets|got|getting|receive|receives|receiving)\b.*\bfrom\b`).MatchString(words2) ||
		regexp.MustCompile(`\b(?:calls?|messages?|sms|texts?)\s+from\b`).MatchString(words2)
	f.Cues["outgoing"] = anyWord("outgoing", "outbound", "dial", "dials", "dialled", "dialed", "dialling", "dialing", "made") || regexp.MustCompile(`\b(?:send|sends|sent|sending|call|calls|called|calling|message|messages|messaged|text|texted)\b.*\bto\b`).MatchString(words2) && !f.Cues["incoming"]
	f.Cues["outgoing"] = f.Cues["outgoing"] && !phrase("made by")
	// any word that gives a question a direction, wider than the incoming and outgoing cues (which need the right phrasing): the obligation below refuses a direction filter only when none is there
	f.Cues["direction"] = anyWord("direction", "directions", "incoming", "outgoing", "inbound", "outbound", "received", "receive", "receives", "receiving", "get", "gets", "got", "getting", "make", "makes", "made", "making",
		"send", "sends", "sent", "sending", "dial", "dials", "dialled", "dialed", "dialling", "dialing", "called", "calling", "from", "to")
	f.Cues["volume"] = anyWord("bytes", "volume", "volumes", "transferred", "megabytes", "gigabytes", "kilobytes", "upload", "uploaded", "download", "downloaded") || phrase("data used") || phrase("data usage")
	f.Cues["sms"] = anyWord("sms", "texts") || phrase("text message") || phrase("text messages")
	f.Cues["mms"] = anyWord("mms")
	f.Cues["ussd"] = anyWord("ussd")
	f.Cues["night"] = anyWord("night", "overnight", "midnight")
	// "count" asks for a number of rows; "only count a score of at least 0.9" and "count it as silent after 10 days" use the verb for "take into account" (found by the stage-3b development
	// check, where the former made the model answer "Count: 1" to "from which silent number may X have taken over").
	f.Cues["count"] = phrase("how many") || phrase("how often") || phrase("number of") || anyWord("count", "total") && !anyWord("duration", "minutes", "seconds", "hours") && !askCountAsConsider.MatchString(words2)
	f.Cues["distinct"] = anyWord("different", "distinct", "unique")
	f.Cues["rank"] = anyWord("top", "busiest", "highest", "largest", "biggest", "frequently") || anyWord("often") && !phrase("how often") || anyWord("most") && !phrase("most recent")
	// "first" and "last" ask for one earliest or latest value only when they qualify an event ("the first call", "when was the last message"); "the first week of March"
	// is a period. "earliest", "oldest", "latest", "newest" and "most recent" always do.
	followed := func(word string) bool {
		return regexp.MustCompile(`\b` + word + `\s+(?:\w+\s+)?(?:call|calls|event|events|contact|contacts|message|messages|sms|time|record|records|seen|appearance|activity|connection|session|sessions|use)\b`).MatchString(lower)
	}
	f.Cues["earliest"] = anyWord("earliest", "oldest") || followed("first") || phrase("when was the first") || phrase("when did it first")
	f.Cues["latest"] = (anyWord("latest", "newest") || phrase("most recent") || followed("last") || phrase("when was the last")) && !f.Cues["rank"]
	f.Cues["services"] = anyWord("services", "shortcodes") || phrase("short codes") || phrase("service numbers") || phrase("sender names")
	f.Cues["hubs"] = anyWord("hubs", "hub") || phrase("everyone contacts") || phrase("everybody contacts")
	f.Cues["exclude"] = anyWord("without", "excluding", "except", "ignoring") || phrase("leave out") || phrase("leaving out") || phrase("not through") || phrase("not counting") || phrase("other than")
	f.Cues["path"] = f.Cues["path"] || (len(f.Numbers) >= 2 && (anyWord("connected", "connection", "connections", "path", "chain", "route", "linked") || phrase("how are")) && !phrase("in contact"))
	f.Cues["duration"] = anyWord("duration", "minutes", "seconds", "longest", "shortest", "talk") && !anyWord("call") || phrase("how long") || phrase("longest call")
	f.Cues["time"] = len(f.Dates) > 0 || len(f.Months) > 0 || len(f.Years) > 0 || anyWord("between", "since", "before", "after", "during")
	// A "first" or "last" that is a number of the question's ("the first five") is a ranking, not a request for one earliest value.
	if f.Limit > 0 {
		f.Cues["earliest"], f.Cues["latest"] = false, false
		f.Cues["rank"] = true
	}

	for i, value := range held {
		text = strings.Replace(text, fmt.Sprintf(" \x00%d\x00 ", i), " "+value+" ", 1)
	}
	f.Question = strings.Join(strings.Fields(text), " ")
	return f
}

func askUnique(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// askObligation is one thing the question demands that the query does not do. Kind is a fixed word (the audit keeps it), Message is what the model is told
// so that it can repair the query, Reason is what the analyst is told when the question is abstained.
type askObligation struct {
	Kind    string
	Message string
	Reason  string
}

var (
	askSQLIdentRE = regexp.MustCompile(`^\+?\d{9,20}$`)
	askSQLIPRE    = regexp.MustCompile(`^\d{1,3}(?:\.\d{1,3}){3}(?:/\d+)?$`)
)

// askConstText returns the string constants of the query and the numeric ones.
func askConstants(v *askValidated) (texts []string, numbers map[int]bool) {
	numbers = map[int]bool{}
	for _, c := range v.Consts {
		if c.Numeric {
			if n, err := strconv.Atoi(c.Text); err == nil {
				numbers[n] = true
			}
			continue
		}
		texts = append(texts, c.Text)
	}
	return texts, numbers
}

// askNameAsNumber finds src or dst compared with a constant of two or more capitalised words, which is a person's name (a single token such as a sender id is not matched).
var askNameAsNumber = regexp.MustCompile(`(?i:\b(?:src|dst)\s*=\s*)'[A-Z][a-z]+(?: [A-Z][a-z]+)+'`)

// askDirectionFilter finds a condition on the direction column (a comparison, a list or a pattern); grouping or selecting it is not a filter.
var askDirectionFilter = regexp.MustCompile(`\bdirection\s*(?:=|<>|!=|in\s*\(|like\b)`)

func askSQLMentions(sql string, words ...string) bool {
	for _, w := range words {
		if strings.Contains(sql, w) {
			return true
		}
	}
	return false
}

// askUsesView is whether the query reads the named analyst view.
func askUsesView(v *askValidated, name string) bool {
	for _, view := range v.Views {
		if view == name {
			return true
		}
	}
	return false
}

var askHopsBound = regexp.MustCompile(`\bhops\s*(<=|<|=)\s*(\d+)`)

// askHopsBounded is whether the query limits the number of links to at most n (hops <= n, hops < n+1 or hops = k with k <= n).
func askHopsBounded(sql string, n int) bool {
	for _, m := range askHopsBound.FindAllStringSubmatch(sql, -1) {
		k, err := strconv.Atoi(m[2])
		if err != nil {
			continue
		}
		switch m[1] {
		case "<=", "=":
			if k <= n {
				return true
			}
		case "<":
			if k-1 <= n {
				return true
			}
		}
	}
	return false
}

var askInList = regexp.MustCompile(`\b(src|dst)\s+in\s*\(([^)]*)\)`)

// askBothDirections finds the first pair of numbers of the question that the query reads in both directions: src = A and dst = B as well as src = B and dst = A (or src IN (A, B) and
// dst IN (A, B)). The numbers are returned in the order the question names them.
func askBothDirections(sql string, numbers []string) (string, string, bool) {
	mentioned := func(column, id string) bool {
		return regexp.MustCompile(`\b` + column + `\s*=\s*'\+?` + regexp.QuoteMeta(strings.ToLower(id)) + `'`).MatchString(sql)
	}
	listed := map[string]map[string]bool{"src": {}, "dst": {}}
	for _, m := range askInList.FindAllStringSubmatch(sql, -1) {
		for _, id := range numbers {
			if strings.Contains(m[2], strings.ToLower(id)) {
				listed[m[1]][id] = true
			}
		}
	}
	for i := 0; i < len(numbers); i++ {
		for j := i + 1; j < len(numbers); j++ {
			a, b := numbers[i], numbers[j]
			if mentioned("src", a) && mentioned("dst", b) && mentioned("src", b) && mentioned("dst", a) {
				return a, b, true
			}
			if listed["src"][a] && listed["src"][b] && listed["dst"][a] && listed["dst"][b] {
				return a, b, true
			}
		}
	}
	return "", "", false
}

func askUsesColumn(v *askValidated, names ...string) bool {
	for _, c := range v.Columns {
		for _, n := range names {
			if c == n {
				return true
			}
		}
	}
	return false
}

// askCheckFrame holds a validated query against the frame of its question. It returns what is unmet, in a fixed order.
func askCheckFrame(f askFrame, v *askValidated) []askObligation {
	var unmet []askObligation
	add := func(kind, message, reason string) {
		unmet = append(unmet, askObligation{Kind: kind, Message: message, Reason: reason})
	}
	texts, numbers := askConstants(v)
	sql := strings.ToLower(v.SQL)
	has := func(identifier string) bool {
		plain := strings.TrimPrefix(identifier, "+")
		for _, t := range texts {
			t = strings.TrimPrefix(strings.TrimSpace(t), "+")
			if t == plain || strings.EqualFold(t, plain) || strings.Contains(strings.ReplaceAll(t, "%", ""), plain) && strings.ContainsAny(t, "%") {
				return true
			}
			if strings.HasPrefix(strings.ToLower(t), strings.ToLower(plain)+"/") { // an address written with its mask
				return true
			}
		}
		return false
	}

	// 1. Every identifier the question names is in the query, and the query names none the question did not.
	for _, id := range f.Identifiers() {
		if _, plate := f.PlateWritten[id]; plate {
			continue // a plate is compared as its search key, which askPlateObligations says
		}
		if !has(id) {
			add("IDENTIFIER", fmt.Sprintf("The question names %s but the query does not use it as a text constant. Compare against it exactly as written.", id), "the question names an identifier that the query does not use")
		}
	}
	askPlateObligations(f, texts, sql, add)
	asked := map[string]bool{}
	for _, id := range f.Identifiers() {
		asked[strings.TrimPrefix(id, "+")] = true
		asked[strings.ToLower(id)] = true
	}
	// An IMEI is 15 digits with its check digit; some exports store the 14 digits without it. The 14-digit form of an IMEI of the question is that IMEI, not another identifier.
	for _, imei := range f.IMEIs {
		asked[askIMEI14(imei)] = true
	}
	for _, t := range texts {
		t = strings.TrimPrefix(strings.TrimSpace(t), "+")
		bare := strings.ToLower(strings.Split(t, "/")[0])
		if (askSQLIdentRE.MatchString(t) || askSQLIPRE.MatchString(t)) && !asked[t] && !asked[bare] {
			add("FOREIGN_IDENTIFIER", fmt.Sprintf("The query names %s, which the question does not mention. Use only the identifiers of the question.", t), "the query names an identifier that the question does not")
		}
	}

	// 2. Every date, month and year of the question is in the query. A last day that the question says is included may be bounded by the start of the next day.
	inclusiveEnd := askInclusiveEnd(strings.ToLower(f.Question), f.Dates)
	for _, d := range f.Dates {
		if !askDateConstant(texts, d, d == inclusiveEnd) {
			add("DATE", fmt.Sprintf("The question names the date %s but the query does not compare against it.", d), "the question names a date that the query does not apply")
		}
	}
	if inclusiveEnd != "" {
		// found by the ANPR development check on case 2: "from 2026-02-10 through 2026-03-13" was bounded by observed_utc < '2026-03-13 00:00:00+05', which leaves the 13th out
		next := askNextDay(inclusiveEnd)
		bounded := false
		for _, t := range texts {
			t = strings.TrimSpace(t)
			bounded = bounded || (next != "" && strings.HasPrefix(t, next)) || strings.HasPrefix(t, inclusiveEnd+" 23:59") || strings.HasPrefix(t, inclusiveEnd+"T23:59")
		}
		if !bounded && next != "" {
			add("DATE_END", fmt.Sprintf("The question includes the whole of %[1]s (the range runs through the end of that day, local time): the upper bound is the start of the next day, < '%[2]s 00:00:00+05', not %[1]s itself.", inclusiveEnd, next), "the question includes its last day and the query stops before it")
		}
	}
	for _, m := range f.Months {
		found := false
		for _, t := range texts {
			t = strings.TrimSpace(t)
			if len(t) >= 7 && t[4] == '-' {
				year, _ := strconv.Atoi(t[:4])
				month, _ := strconv.Atoi(t[5:7])
				if month == m.Month && (m.Year == 0 || year == m.Year) {
					found = true
				}
			}
		}
		if !found && numbers[m.Month] && askSQLMentions(sql, "month") {
			found = true
		}
		if !found {
			add("MONTH", fmt.Sprintf("The question names month %d%s but the query has no date in it. Write the first day of the month and the first day of the next month as explicit dates.", m.Month, askYearNote(m.Year)), "the question names a month that the query does not apply")
		}
	}
	for _, year := range f.Years {
		found := numbers[year]
		for _, t := range texts {
			found = found || strings.HasPrefix(strings.TrimSpace(t), strconv.Itoa(year))
		}
		if !found {
			add("YEAR", fmt.Sprintf("The question names the year %d but the query does not apply it.", year), "the question names a year that the query does not apply")
		}
	}

	// 3. A limit or a hop count the question states is the query's.
	if f.Limit > 0 && v.Limit != f.Limit && !numbers[f.Limit] {
		add("LIMIT", fmt.Sprintf("The question asks for %d, so the query must keep exactly %d rows (LIMIT %d).", f.Limit, f.Limit, f.Limit), "the question asks for a number of results that the query does not return")
	}
	links := askUsesView(v, "v_connection_edges")
	if f.Hops > 0 {
		switch {
		case links && f.Hops > 3:
			add("RECURSION", fmt.Sprintf("v_connection_edges covers at most three hops and the question asks for %d: use a bounded recursive query over v_contact_pairs (depth < %d).", f.Hops, f.Hops), "the view reaches three hops and the question asks for more")
		case links && f.Hops < 3 && !askHopsBounded(sql, f.Hops):
			add("HOPS", fmt.Sprintf("The question asks for %d hops: add hops <= %d.", f.Hops, f.Hops), "the query does not limit the number of links to the one asked")
		case links:
			// v_connection_edges walks up to three links itself
		case !v.Recursive:
			add("RECURSION", fmt.Sprintf("The question asks about connections within %d hops, so the query must be a bounded recursive query (WITH RECURSIVE) with depth < %d.", f.Hops, f.Hops), "a connection question needs a path search that the query does not do")
		case v.MaxHops != f.Hops:
			add("HOPS", fmt.Sprintf("The question asks for %d hops but the query's depth bound allows %d.", f.Hops, v.MaxHops), "the query searches a different number of hops from the one asked")
		}
	} else if f.Cues["path"] && len(f.Numbers) >= 2 && !v.Recursive && !links {
		add("RECURSION", "The question asks how two numbers are connected, so the query must walk the contacts with a bounded WITH RECURSIVE (depth <= 3).", "a connection question needs a path search that the query does not do")
	}

	// 4. Marks that the question's words leave in the query.
	if f.Cues["incoming"] && !askSQLMentions(sql, "'incoming'") && !askUsesColumn(v, "incoming") {
		add("DIRECTION", "The question is about incoming events: filter direction = 'incoming' (or use the incoming column).", "the question is about incoming events and the query does not say so")
	}
	if f.Cues["outgoing"] && !askSQLMentions(sql, "'outgoing'") && !askUsesColumn(v, "outgoing") {
		add("DIRECTION", "The question is about outgoing events: filter direction = 'outgoing' (or use the outgoing column).", "the question is about outgoing events and the query does not say so")
	}
	// A person's name compared with a number column. src and dst hold phone numbers and no view maps a name to one, so the query counts nothing and the answer would be a confident 0
	// about a person the case may well hold (found 2026-10-20 on forensic-demo: "How many call records did Hamza Ghauri have?" answered 0).
	if askNameAsNumber.MatchString(v.SQL) {
		add("NAME", "src and dst hold phone numbers; a person's name is not one, and no view maps names to numbers. If the question names a person and no number, set answerable to false instead of comparing a number column with a name.", "the query compares a number column with a person's name")
	}
	// A filter on direction that the question never asked for narrows the answer without saying so (found 2026-10-20: "the latest call start time in the calls" was answered from
	// outgoing voice calls only, and the answer was a day early).
	if !f.Cues["incoming"] && !f.Cues["outgoing"] && !f.Cues["direction"] && askDirectionFilter.MatchString(sql) {
		add("DIRECTION", "The question does not ask about incoming or outgoing events: remove the filter on direction.", "the query filters on direction and the question does not ask about it")
	}
	for _, kind := range []string{"sms", "mms", "ussd"} {
		// v_meeting_candidates is defined on calls and SMS between the two numbers, so "a call or SMS between them" is carried by the view itself
		if f.Cues[kind] && !askSQLMentions(sql, "'"+kind+"'") && !askUsesView(v, "v_meeting_candidates") {
			add("EVENT_TYPE", fmt.Sprintf("The question is about %s: filter event_type = '%s'.", strings.ToUpper(kind), kind), "the question is about one kind of event and the query does not say so")
		}
	}
	// the anchor views carry the night as a column (slot = 'night'), so a night question asked of them is carried by that filter, not by an hour of day (found by the stage-3c judgement run:
	// "which cell site was the night anchor of X in the week starting ..." was refused three times for a missing hour)
	if f.Cues["night"] && !askSQLMentions(sql, "hour", "::time", "time '") && !(askUsesAnchorView(v) && askSQLMentions(sql, "'night'")) {
		add("TIME_OF_DAY", "The question is about the night: restrict the hour of day (EXTRACT(HOUR FROM start_utc) BETWEEN 0 AND 5 on the case clock).", "the question is about the night and the query does not restrict the hour")
	}
	// The days of silence, the start window and the lowest score of the successor view are set by the application from the question. A query that works out a window of its own from the two
	// times (found by the first live question that named a silence and a window: the model compared the new number's first event with the old one's last event plus an interval of 10 + 21
	// days, and the answer was "No row matches" for a pair the view lists) answers a different question than the one asked.
	if askUsesView(v, "v_successor_candidates") && askSQLMentions(sql, "interval") {
		add("PARAMETER", "The days of silence, the start window and the lowest score are set by the application from the question: do not compute a window from new_first_event_utc and old_last_event_utc or use an interval; select from v_successor_candidates and filter only on old_number, new_number or the score.", "the query works out a window of its own on the successor view")
	}
	// A row of v_contact_pairs is one contact (a number, in one direction), not one event: "how many events are there between A and B" is the events column of the pair's row, not a count of rows. A count of its rows said 1
	// where the events were 76 (found by the stage-3 milestone run on case 110).
	if askUsesView(v, "v_contact_pairs") && !askUsesView(v, "v_comm_events") && askCountsEvents.MatchString(strings.ToLower(f.Question)) && askSQLMentions(sql, "count(") && !askSQLMentions(sql, "sum(") {
		add("AGGREGATE", "A row of v_contact_pairs is one contact, not one event: for how many events, calls or messages there are between TWO numbers, take MAX(events) over the rows of the pair (WHERE src IN (A, B) AND dst IN (A, B); the incoming, outgoing and unknown events are already in that column). For the events of ONE number with others, SUM its events column. Do not COUNT the rows.", "the question counts events and the query counts the contacts")
	}
	// A call between two subscribers is in the records of both: once as outgoing in one export and once as incoming in the other, at the same time. The events of A with B and of B with A are the
	// same events, so a query that adds both directions counts every call twice (found 2026-10-26 on the live service: "How many events are there between A and B?" answered 8 where the records hold 4;
	// the repair message of the stage-3 milestone run had said "both directions"). Reading both directions is right when only one of the two is a subscriber (there is one row), so the answer is the
	// pair's rows taken ONCE: MAX of the events column, or the distinct start times of the events.
	if a, b, both := askBothDirections(sql, f.Numbers); both {
		switch {
		case askUsesView(v, "v_contact_pairs") && askSQLMentions(sql, "sum(", "count("):
			add("PAIR_DIRECTION", fmt.Sprintf("The events of %[1]s with %[2]s and of %[2]s with %[1]s are the same events: a call between two subscribers is in both of their records. Take the pair once: SELECT MAX(events) ... WHERE src IN ('%[1]s', '%[2]s') AND dst IN ('%[1]s', '%[2]s') (when both are subscribers the two rows hold the same calls, when only one is there is one row). Write the pair as two IN lists joined by AND, so that any other condition is one more AND. Do not SUM or COUNT both directions, which counts every call twice.", a, b), "the query adds the events of the two numbers in both directions, which counts every call between two subscribers twice")
		case askUsesView(v, "v_comm_events") && askSQLMentions(sql, "count(") && !askSQLMentions(sql, "count(distinct", "count( distinct"):
			add("PAIR_DIRECTION", fmt.Sprintf("The events of %[1]s with %[2]s and of %[2]s with %[1]s are the same events: a call between two subscribers is in both of their records at the same time. Count them once: SELECT COUNT(DISTINCT start_utc) ... WHERE src IN ('%[1]s', '%[2]s') AND dst IN ('%[1]s', '%[2]s') AND (the dates or any other condition), not COUNT(*). Write the pair as two IN lists joined by AND, so that every other condition applies to both directions.", a, b), "the query counts the events of the two numbers in both directions, which counts every call between two subscribers twice")
		}
	}
	// A row of v_plate_visits is one plate at one camera, not one sighting: "how many times was it seen at each camera" is its sightings column. A count of its rows said 1 for a plate seen
	// five times at a camera (found by the ANPR development check on case 2).
	if askUsesView(v, "v_plate_visits") && !askUsesView(v, "v_plate_sightings") && askCountsSightings.MatchString(strings.ToLower(f.Question)) && askSQLMentions(sql, "count(") && !askSQLMentions(sql, "sum(") && !askBareSightings.MatchString(sql) {
		add("AGGREGATE", "A row of v_plate_visits is one plate at one camera, not one sighting: the number of sightings is its sightings column. SELECT sightings (or SUM(sightings) when you group); do not COUNT its rows.", "the question counts sightings and the query counts the plate-camera pairs")
	}
	if f.Cues["count"] && !askSQLMentions(sql, "count(", "sum(") && !askUsesColumn(v, "events", "incoming", "outgoing", "sightings") {
		add("AGGREGATE", "The question asks how many: count the rows (COUNT) or sum the counts of the view.", "the question asks for a count that the query does not compute")
	}
	if f.Cues["distinct"] && !askSQLMentions(sql, "distinct") && !askSQLMentions(sql, "group by") {
		add("DISTINCT", "The question asks for different values: count DISTINCT values or group by them.", "the question asks for different values and the query does not take them")
	}
	if f.Cues["rank"] && !f.Cues["count"] && !askSQLMentions(sql, "order by") {
		add("ORDER", "The question asks for the most or the top: ORDER BY the measure descending and LIMIT.", "the question asks for a ranking and the query does not order")
	}
	if f.Cues["earliest"] && f.Limit == 0 && !askSQLMentions(sql, "min(", "order by", "first_seen") {
		add("EARLIEST", "The question asks for the earliest: use MIN of the time, or ORDER BY the time ascending with LIMIT 1.", "the question asks for the earliest and the query does not take it")
	}
	if f.Cues["latest"] && f.Limit == 0 && !askSQLMentions(sql, "max(", "order by", "last_seen") {
		add("LATEST", "The question asks for the latest: use MAX of the time, or ORDER BY the time descending with LIMIT 1.", "the question asks for the latest and the query does not take it")
	}
	if f.Cues["services"] && f.Cues["exclude"] && !askUsesColumn(v, "is_service", "dst_is_person_like", "ordinary") && !askSQLMentions(sql, "dst_kind", "is_service") {
		add("SERVICES", "The question leaves out services and short codes: use NOT is_service (or dst_is_person_like).", "the question leaves out services and the query does not")
	}
	if f.Cues["hubs"] && f.Cues["exclude"] && !askUsesColumn(v, "is_hub", "ordinary") {
		add("HUBS", "The question leaves out numbers that everyone contacts: use NOT is_hub.", "the question leaves out hubs and the query does not")
	}
	return unmet
}

func askYearNote(year int) string {
	if year == 0 {
		return ""
	}
	return fmt.Sprintf(" of %d", year)
}

// askCheckResult holds the result against the frame: a count is one value, and a question that names an identifier is not answered by a list of
// everything. It returns nil when the result is acceptable. (Emptiness is handled by the probe in the run, because an empty result is sometimes the truth.)
// askIMEI14 is an IMEI without its check digit: the first 14 digits of a 15-digit IMEI (the type approval code and the serial number).
func askIMEI14(imei string) string {
	if len(imei) == 15 {
		return imei[:14]
	}
	return imei
}

func askCheckResult(f askFrame, v *askValidated, columns []string, rows int) *askObligation {
	// A connection question (decision D22) is answered with the edges of the connections found, so that the answer can name the numbers between the two. Measured on seed
	// 101: asked "is there a link between A and B within three hops", the model wrote an existence test and the answer was a bare "Has link: true", which is true and names
	// nothing. The result must carry src and dst, as the verified example's does.
	if (f.Hops > 0 || f.Cues["path"]) && len(f.Numbers) >= 2 && (v.Recursive || askUsesView(v, "v_connection_edges")) {
		has := map[string]bool{}
		for _, c := range columns {
			has[strings.ToLower(c)] = true
		}
		if !has["src"] || !has["dst"] {
			return &askObligation{Kind: "SHAPE", Message: "A connection question is answered with the edges of the connections found: return the columns src and dst, one row per edge of the shortest routes between the two numbers, as in the verified example, not a yes or no.", Reason: "the question asks how two numbers are connected and the result is not the connection itself"}
		}
	}
	grouped := askSQLMentions(strings.ToLower(v.SQL), "group by", "partition by") || askSQLMentions(strings.ToLower(v.SQL), " per ")
	asksPer := strings.Contains(" "+strings.ToLower(f.Question)+" ", " per ") || strings.Contains(strings.ToLower(f.Question), "each ") || strings.Contains(strings.ToLower(f.Question), " by ") ||
		strings.Contains(strings.ToLower(f.Question), "which ") || strings.Contains(strings.ToLower(f.Question), "who ") || strings.Contains(strings.ToLower(f.Question), "list")
	if f.Cues["count"] && !f.Cues["rank"] && !asksPer && !grouped && (rows != 1 || len(columns) != 1) {
		return &askObligation{Kind: "SHAPE", Message: "The question asks for one number, but the query returns several rows or columns. Return one row with one value.", Reason: "the question asks for one value and the result is a list"}
	}
	if rows > 1 && f.Cues["earliest"] && f.Limit == 0 && !asksPer && !grouped && len(columns) == 1 {
		return &askObligation{Kind: "SHAPE", Message: "The question asks for the earliest one; return only that one.", Reason: "the question asks for one value and the result is a list"}
	}
	return nil
}
