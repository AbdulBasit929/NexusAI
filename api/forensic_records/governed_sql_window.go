package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	pg "github.com/pganalyze/pg_query_go/v6"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// GOVERNED SQL: A WINDOW OF THE DAY, AND THE DAYS OF THE WEEK (RUN 15).
//
// Two classes the fresh-seed probes found (pre-registration, "Run 15 plan", classes K and N):
//
//	"How many calls were made between 11 pm and 3 am?"  abstained twice: the shared detector read "between 11" as a
//	                                                     quantity the query had to compare with 11
//	"How many calls were made on weekends?"              3,041 for 2,980: the query kept Friday and Saturday
//
// Both are answered by looking at what the query keeps, not at the numbers it contains. The question gives a
// set of hours or of days; the query's condition on the hour of the day (or on the day of the week) is read as a
// function of that one variable and evaluated over all of its values; the set it keeps must be the set the
// question names. A condition the check cannot read (a CASE, arithmetic on the hour, a subquery) is left alone,
// so the check only ever rejects a query it can show keeps another set. Nothing here asks the model anything.
//
// A window runs from its first hour up to, and not including, its last: "between 11 pm and 3 am" is 23:00 to
// 03:00, the hours 23, 0, 1 and 2; "between 9 am and 5 pm" is the hours 9 to 16. A record at 3:30 am is not
// between 11 pm and 3 am.

// ---------------------------------------------------------------- reading the question

// govSQLClockWindow is a window of the day the question gives outright, in whole hours.
type govSQLClockWindow struct {
	Phrase     string // as written
	Start, End int    // hours; the window runs from Start up to, and not including, End (End may be 24)
}

// govSQLClockSpan is one "from X to Y" of clock times, in minutes of the day, wherever it stands in the text.
type govSQLClockSpan struct {
	Phrase     string
	Start, End int // minutes of the day; End may be 1440
	From, To   int // byte range in the text
	whole      bool
}

const govSQLClockTime = `(\d{1,2})(?::(\d{2}))?\s*(?:([ap])\.?m\b\.?)?`

var govSQLRxClockWindow = regexp.MustCompile(`(?i)\b(?:(between|from)\s+)?` + govSQLClockTime + `(?:\s+(?:and|to|until|till|through)\s+|\s*[-–—]\s*)` + govSQLClockTime)

// govSQLClockMinutes reads one clock time: "11 pm", "3:30 am", "23:00". A bare number is not a clock time.
func govSQLClockMinutes(hourText, minuteText, meridiem string) (int, bool) {
	hour, err := strconv.Atoi(hourText)
	if err != nil {
		return 0, false
	}
	minute := 0
	if minuteText != "" {
		if minute, err = strconv.Atoi(minuteText); err != nil || minute > 59 {
			return 0, false
		}
	}
	meridiem = strings.ToLower(meridiem)
	switch {
	case meridiem != "":
		if hour < 1 || hour > 12 {
			return 0, false
		}
		hour %= 12
		if meridiem == "p" {
			hour += 12
		}
	case minuteText != "":
		if hour > 24 || (hour == 24 && minute != 0) {
			return 0, false
		}
	default:
		return 0, false
	}
	return hour*60 + minute, true
}

// govSQLClockSpans finds every pair of clock times the text joins with "between ... and", "from ... to" or a dash.
// Both ends must be clock times (an am/pm or a colon): "between 9 and 5" is a pair of numbers, not a window.
func govSQLClockSpans(text string) []govSQLClockSpan {
	var out []govSQLClockSpan
	for _, loc := range govSQLRxClockWindow.FindAllStringSubmatchIndex(text, -1) {
		group := func(i int) string {
			if loc[2*i] < 0 {
				return ""
			}
			return text[loc[2*i]:loc[2*i+1]]
		}
		start, okStart := govSQLClockMinutes(group(2), group(3), group(4))
		end, okEnd := govSQLClockMinutes(group(5), group(6), group(7))
		if !okStart || !okEnd {
			continue
		}
		// Without "between" or "from" the pair must say am/pm on both ends, or have a colon on both.
		bothMeridiem, bothColon := group(4) != "" && group(7) != "", group(3) != "" && group(6) != ""
		if group(1) == "" && !bothMeridiem && !bothColon {
			continue
		}
		out = append(out, govSQLClockSpan{
			Phrase: strings.TrimSpace(text[loc[0]:loc[1]]), Start: start, End: end, From: loc[0], To: loc[1],
			whole: start%60 == 0 && end%60 == 0,
		})
	}
	return out
}

// govSQLReadClockWindow is the first window of whole hours the question gives, or nil.
func govSQLReadClockWindow(text string) *govSQLClockWindow {
	for _, span := range govSQLClockSpans(text) {
		if span.whole && span.Start != span.End {
			return &govSQLClockWindow{Phrase: span.Phrase, Start: span.Start / 60, End: span.End / 60}
		}
	}
	return nil
}

// govSQLStripClockSpans removes the clock windows from a question before a quantity is read from it: the shared
// detector takes "between 11" in "between 11 pm and 3 am" for a number the query must compare with.
func govSQLStripClockSpans(text string) string {
	spans := govSQLClockSpans(text)
	for i := len(spans) - 1; i >= 0; i-- {
		text = text[:spans[i].From] + " " + text[spans[i].To:]
	}
	return text
}

// hours lists the hours of the window, in the order the day runs them.
func (w *govSQLClockWindow) hours() []int {
	count := w.End - w.Start
	if count <= 0 {
		count += 24
	}
	out := make([]int, 0, count)
	for i := 0; i < count; i++ {
		out = append(out, (w.Start+i)%24)
	}
	return out
}

func (w *govSQLClockWindow) crossesMidnight() bool { return w.End < w.Start }

func (w *govSQLClockWindow) holds(second int) bool {
	hour := second / 3600
	if w.crossesMidnight() {
		return hour >= w.Start || hour < w.End
	}
	return hour >= w.Start && hour < w.End
}

// govSQLDaySet is the days of the week the question names, Sunday = 0 ... Saturday = 6.
type govSQLDaySet struct {
	Phrase string
	Days   [7]bool
}

var govSQLDayNames = []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}

var (
	govSQLRxWeekend   = regexp.MustCompile(`(?i)\bweekends?\b`)
	govSQLRxWeekday   = regexp.MustCompile(`(?i)\bweekdays?\b`)
	govSQLRxDayWord   = regexp.MustCompile(`(?i)\b(sunday|monday|tuesday|wednesday|thursday|friday|saturday)s?\b`)
	govSQLRxDayBefore = regexp.MustCompile(`(?i)\b(?:which|what|each|every|per|by|any|last|this|next|past|previous|coming|between|from)\s+(?:the\s+)?$`)
	govSQLRxDaySep    = regexp.MustCompile(`(?i)^\s*(?:,|,?\s*and|,?\s*or|&)\s*(?:on\s+)?(?:an?\s+|the\s+)?$`)
)

// govSQLReadDays reads "weekends", "weekdays" and a list of day names joined by and or or. "Which weekend", "each
// Friday", "last Friday" and a day beside a date name something else (a grouping, a date) and are left alone.
func govSQLReadDays(text string) *govSQLDaySet {
	if govSQLRxISODate.MatchString(text) || govSQLRxSlashDate.MatchString(text) {
		return nil
	}
	unqualified := func(rx *regexp.Regexp) (string, bool) {
		for _, loc := range rx.FindAllStringIndex(text, -1) {
			if !govSQLRxDayBefore.MatchString(text[:loc[0]]) {
				return text[loc[0]:loc[1]], true
			}
		}
		return "", false
	}
	set := &govSQLDaySet{}
	if word, ok := unqualified(govSQLRxWeekend); ok {
		set.Phrase = word
		set.Days[0], set.Days[6] = true, true
		return set
	}
	if word, ok := unqualified(govSQLRxWeekday); ok {
		set.Phrase = word
		for d := 1; d <= 5; d++ {
			set.Days[d] = true
		}
		return set
	}
	locs := govSQLRxDayWord.FindAllStringSubmatchIndex(text, -1)
	if len(locs) == 0 {
		return nil
	}
	names := make([]string, 0, len(locs))
	for i, loc := range locs {
		if govSQLRxDayBefore.MatchString(text[:loc[0]]) {
			return nil
		}
		if i > 0 && !govSQLRxDaySep.MatchString(text[locs[i-1][1]:loc[0]]) {
			return nil
		}
		name := strings.ToLower(text[loc[2]:loc[3]])
		for d, full := range govSQLDayNames {
			if strings.ToLower(full) == name {
				set.Days[d] = true
			}
		}
		names = append(names, text[loc[0]:loc[1]])
	}
	set.Phrase = strings.Join(names, " and ")
	return set
}

func (s *govSQLDaySet) names() []string {
	var out []string
	for d := 1; d <= 7; d++ { // Monday first, Sunday last
		if s.Days[d%7] {
			out = append(out, govSQLDayNames[d%7])
		}
	}
	return out
}

// isodow lists the days as EXTRACT(ISODOW ...) numbers them (Monday 1 ... Sunday 7).
func (s *govSQLDaySet) isodow() []string {
	var out []string
	for d := 1; d <= 7; d++ {
		if s.Days[d%7] {
			out = append(out, strconv.Itoa(d))
		}
	}
	return out
}

func govSQLJoinWords(words []string) string {
	switch len(words) {
	case 0:
		return ""
	case 1:
		return words[0]
	}
	return strings.Join(words[:len(words)-1], ", ") + " and " + words[len(words)-1]
}

// ---------------------------------------------------------------- reading the query

type govSQLTerm int

const (
	govSQLTermNone    govSQLTerm = iota
	govSQLTermHour               // EXTRACT(HOUR FROM x), date_part('hour', x)
	govSQLTermClock              // x::time, compared with 'HH:MM:SS'
	govSQLTermDOW                // EXTRACT(DOW FROM x): Sunday 0 ... Saturday 6
	govSQLTermISODOW             // EXTRACT(ISODOW FROM x): Monday 1 ... Sunday 7
	govSQLTermDayName            // to_char(x, 'Dy') or 'Day'
	govSQLTermDayD               // to_char(x, 'D'): Sunday 1 ... Saturday 7, as text
	govSQLTermDayID              // to_char(x, 'ID'): Monday 1 ... Sunday 7, as text
)

var govSQLNumericCasts = map[string]bool{
	"int": true, "int2": true, "int4": true, "int8": true, "integer": true, "smallint": true, "bigint": true,
	"numeric": true, "decimal": true, "float4": true, "float8": true, "real": true, "float": true,
}

// govSQLTermOf says what a node reads off a time column, looking through numeric casts and through the case and
// trimming functions that stand around a day name.
func govSQLTermOf(node *pg.Node) govSQLTerm {
	for node != nil {
		switch {
		case node.GetTypeCast() != nil:
			cast := node.GetTypeCast()
			name := govSQLCastName(cast)
			if name == "time" {
				return govSQLTermClock
			}
			if !govSQLNumericCasts[name] {
				return govSQLTermNone
			}
			node = cast.Arg
		case node.GetFuncCall() != nil:
			call := node.GetFuncCall()
			switch govSQLFuncName(call) {
			case "extract", "date_part":
				if len(call.Args) != 2 {
					return govSQLTermNone
				}
				_, field, _, isText, ok := govSQLConstOf(call.Args[0])
				if !ok || !isText {
					return govSQLTermNone
				}
				switch strings.ToLower(field) {
				case "hour":
					return govSQLTermHour
				case "dow":
					return govSQLTermDOW
				case "isodow":
					return govSQLTermISODOW
				}
				return govSQLTermNone
			case "to_char":
				if len(call.Args) != 2 {
					return govSQLTermNone
				}
				_, format, _, isText, ok := govSQLConstOf(call.Args[1])
				if !ok || !isText {
					return govSQLTermNone
				}
				switch strings.TrimPrefix(strings.ToLower(strings.TrimSpace(format)), "fm") {
				case "dy", "day":
					return govSQLTermDayName
				case "d":
					return govSQLTermDayD
				case "id":
					return govSQLTermDayID
				}
				return govSQLTermNone
			case "trim", "btrim", "ltrim", "rtrim", "lower", "upper":
				if len(call.Args) == 0 {
					return govSQLTermNone
				}
				node = call.Args[0]
			default:
				return govSQLTermNone
			}
		default:
			return govSQLTermNone
		}
	}
	return govSQLTermNone
}

// govSQLConstOf is the number or the text a constant node holds, looking through casts ('23:00'::time).
func govSQLConstOf(node *pg.Node) (num float64, text string, isNum, isText, ok bool) {
	for node != nil && node.GetTypeCast() != nil {
		node = node.GetTypeCast().Arg
	}
	if node == nil || node.GetAConst() == nil {
		return 0, "", false, false, false
	}
	c := node.GetAConst()
	switch {
	case c.GetIval() != nil:
		return float64(c.GetIval().Ival), "", true, false, true
	case c.GetFval() != nil:
		f, err := strconv.ParseFloat(c.GetFval().Fval, 64)
		if err != nil {
			return 0, "", false, false, false
		}
		return f, "", true, false, true
	case c.GetSval() != nil:
		return 0, c.GetSval().Sval, false, true, true
	}
	return 0, "", false, false, false
}

var govSQLRxClockText = regexp.MustCompile(`^\s*(\d{1,2}):(\d{2})(?::(\d{2}))?(?:\.\d+)?\s*$`)

// govSQLClockSeconds reads 'HH:MM' or 'HH:MM:SS' as seconds of the day.
func govSQLClockSeconds(text string) (float64, bool) {
	m := govSQLRxClockText.FindStringSubmatch(text)
	if m == nil {
		return 0, false
	}
	h, _ := strconv.Atoi(m[1])
	mi, _ := strconv.Atoi(m[2])
	s := 0
	if m[3] != "" {
		s, _ = strconv.Atoi(m[3])
	}
	if h > 24 || mi > 59 || s > 59 {
		return 0, false
	}
	return float64(h*3600 + mi*60 + s), true
}

// govSQLDayKey folds a day name to its three letters, so 'Saturday ' (to_char pads) and 'SAT' are one day.
func govSQLDayKey(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	if len(name) > 3 {
		name = name[:3]
	}
	return name
}

// govSQLPredicate is a condition read as a function of one variable: whether the variable's value is kept, and
// whether the condition says nothing about the variable at all (neutral: any other condition of the query).
type govSQLPredicate func(point int) (kept, neutral bool)

// govSQLEvaluation compiles a query's condition into a function of the hour of the day, or of the day of the week.
type govSQLEvaluation struct {
	reads       func(govSQLTerm) bool
	value       func(term govSQLTerm, point int) (float64, string)
	involved    bool // some part of the condition reads the variable
	unsupported bool // some part reads it in a way this evaluation cannot follow
}

func (e *govSQLEvaluation) mentions(node *pg.Node) bool {
	if node == nil {
		return false
	}
	found := false
	govSQLWalkTree(node.ProtoReflect(), func(msg protoreflect.Message) bool {
		switch n := msg.Interface().(type) {
		case *pg.FuncCall:
			found = found || e.reads(govSQLTermOf(&pg.Node{Node: &pg.Node_FuncCall{FuncCall: n}}))
		case *pg.TypeCast:
			found = found || e.reads(govSQLTermOf(&pg.Node{Node: &pg.Node_TypeCast{TypeCast: n}}))
		}
		return !found
	})
	return found
}

func govSQLFlipOperator(op string) string {
	switch op {
	case "<":
		return ">"
	case "<=":
		return ">="
	case ">":
		return "<"
	case ">=":
		return "<="
	}
	return op
}

func (e *govSQLEvaluation) compile(node *pg.Node) govSQLPredicate {
	neutral := func(int) (bool, bool) { return true, true }
	if node == nil {
		return neutral
	}
	switch {
	case node.GetBoolExpr() != nil:
		b := node.GetBoolExpr()
		children := make([]govSQLPredicate, 0, len(b.Args))
		for _, arg := range b.Args {
			children = append(children, e.compile(arg))
		}
		switch b.Boolop {
		case pg.BoolExprType_AND_EXPR:
			return func(point int) (bool, bool) {
				kept, allNeutral := true, true
				for _, child := range children {
					if v, n := child(point); !n {
						kept, allNeutral = kept && v, false
					}
				}
				return kept, allNeutral
			}
		case pg.BoolExprType_OR_EXPR:
			// "reads the hour OR something else" keeps every hour: the other condition can hold at any of them.
			return func(point int) (bool, bool) {
				kept, allNeutral, anyNeutral := false, true, false
				for _, child := range children {
					v, n := child(point)
					if n {
						anyNeutral = true
					} else {
						kept, allNeutral = kept || v, false
					}
				}
				return kept || anyNeutral, allNeutral
			}
		case pg.BoolExprType_NOT_EXPR:
			if len(children) == 1 {
				return func(point int) (bool, bool) {
					v, n := children[0](point)
					if n {
						return true, true
					}
					return !v, false
				}
			}
		}
		e.unsupported = true
		return neutral
	case node.GetAExpr() != nil:
		return e.atom(node.GetAExpr())
	}
	if e.mentions(node) {
		e.unsupported = true
	}
	return neutral
}

// atom compiles one comparison. A comparison that does not read the variable is neutral; one that reads it in a
// form this does not follow marks the whole reading unsupported.
func (e *govSQLEvaluation) atom(x *pg.A_Expr) govSQLPredicate {
	neutral := func(int) (bool, bool) { return true, true }
	left, right := govSQLTermOf(x.Lexpr), govSQLTermOf(x.Rexpr)
	otherNode, term, flipped := x.Rexpr, left, false
	if !e.reads(left) {
		otherNode, term, flipped = x.Lexpr, right, true
	}
	if !e.reads(term) {
		if e.mentions(x.Lexpr) || e.mentions(x.Rexpr) {
			e.unsupported = true
		}
		return neutral
	}
	e.involved = true
	unsupported := func() govSQLPredicate {
		e.unsupported = true
		return neutral
	}
	// one constant of the right kind for the term, as a number or as a day key
	constant := func(node *pg.Node) (num float64, key string, ok bool) {
		n, text, isNum, isText, found := govSQLConstOf(node)
		switch {
		case !found:
			return 0, "", false
		case term == govSQLTermDayName:
			return 0, govSQLDayKey(text), isText
		case term == govSQLTermClock:
			if !isText {
				return 0, "", false
			}
			seconds, good := govSQLClockSeconds(text)
			return seconds, "", good
		case (term == govSQLTermDayD || term == govSQLTermDayID) && isText:
			// to_char returns text, so the day number is compared with '1', '7'
			f, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
			return f, "", err == nil
		default:
			return n, "", isNum
		}
	}
	read := func(point int) (float64, string) { return e.value(term, point) }

	switch x.Kind {
	case pg.A_Expr_Kind_AEXPR_OP:
		op := govSQLOpName(x)
		if flipped {
			op = govSQLFlipOperator(op)
		}
		num, key, ok := constant(otherNode)
		if !ok {
			return unsupported()
		}
		if term == govSQLTermDayName {
			if op != "=" && op != "<>" && op != "!=" {
				return unsupported()
			}
			return func(point int) (bool, bool) {
				_, got := read(point)
				if op == "=" {
					return got == key, false
				}
				return got != key, false
			}
		}
		switch op {
		case "=", "<>", "!=", "<", "<=", ">", ">=":
		default:
			return unsupported()
		}
		return func(point int) (bool, bool) {
			got, _ := read(point)
			switch op {
			case "=":
				return got == num, false
			case "<>", "!=":
				return got != num, false
			case "<":
				return got < num, false
			case "<=":
				return got <= num, false
			case ">":
				return got > num, false
			}
			return got >= num, false
		}
	case pg.A_Expr_Kind_AEXPR_IN:
		if flipped || x.Rexpr == nil || x.Rexpr.GetList() == nil {
			return unsupported()
		}
		var nums []float64
		var keys []string
		for _, item := range x.Rexpr.GetList().Items {
			num, key, ok := constant(item)
			if !ok {
				return unsupported()
			}
			nums, keys = append(nums, num), append(keys, key)
		}
		negate := govSQLOpName(x) == "<>"
		return func(point int) (bool, bool) {
			got, gotKey := read(point)
			in := false
			for i := range nums {
				if term == govSQLTermDayName {
					in = in || keys[i] == gotKey
				} else {
					in = in || nums[i] == got
				}
			}
			return in != negate, false
		}
	case pg.A_Expr_Kind_AEXPR_BETWEEN, pg.A_Expr_Kind_AEXPR_NOT_BETWEEN:
		if flipped || term == govSQLTermDayName || x.Rexpr == nil || x.Rexpr.GetList() == nil || len(x.Rexpr.GetList().Items) != 2 {
			return unsupported()
		}
		low, _, okLow := constant(x.Rexpr.GetList().Items[0])
		high, _, okHigh := constant(x.Rexpr.GetList().Items[1])
		if !okLow || !okHigh {
			return unsupported()
		}
		negate := x.Kind == pg.A_Expr_Kind_AEXPR_NOT_BETWEEN
		return func(point int) (bool, bool) {
			got, _ := read(point)
			return (got >= low && got <= high) != negate, false
		}
	}
	return unsupported()
}

// govSQLPredicateRoots lists every condition a query applies to its rows: each WHERE, each HAVING, each FILTER.
func govSQLPredicateRoots(tree *pg.ParseResult) []*pg.Node {
	var roots []*pg.Node
	for _, stmt := range tree.Stmts {
		govSQLWalkTree(stmt.ProtoReflect(), func(msg protoreflect.Message) bool {
			switch n := msg.Interface().(type) {
			case *pg.SelectStmt:
				if n.WhereClause != nil {
					roots = append(roots, n.WhereClause)
				}
				if n.HavingClause != nil {
					roots = append(roots, n.HavingClause)
				}
			case *pg.FuncCall:
				if n.AggFilter != nil {
					roots = append(roots, n.AggFilter)
				}
			}
			return true
		})
	}
	return roots
}

// govSQLReading is what the query's conditions keep of one variable.
type govSQLReading struct {
	mismatch   []bool // the set the query keeps, when it is not the set the question names
	verified   bool   // a condition was read, and it is the right set
	cannotTell bool   // a condition reads the variable in a form this does not follow
	absent     bool   // nothing in the query reads the variable at all
}

// govSQLReadConditions evaluates every condition of the query over the points 0..points-1 and compares what it
// keeps with want. tolerance is how many points may differ (the instant a '<= 03:00' comparison adds).
func govSQLReadConditions(tree *pg.ParseResult, e *govSQLEvaluation, points int, want func(point int) bool, tolerance int) govSQLReading {
	var reading govSQLReading
	for _, root := range govSQLPredicateRoots(tree) {
		e.involved, e.unsupported = false, false
		predicate := e.compile(root)
		if !e.involved {
			if e.unsupported {
				reading.cannotTell = true
			}
			continue
		}
		if e.unsupported {
			reading.cannotTell = true
			continue
		}
		kept := make([]bool, points)
		differ := 0
		for p := 0; p < points; p++ {
			v, _ := predicate(p)
			kept[p] = v
			if v != want(p) {
				differ++
			}
		}
		if differ > tolerance {
			if reading.mismatch == nil {
				reading.mismatch = kept
			}
			continue
		}
		reading.verified = true
	}
	if reading.mismatch == nil && !reading.verified && !reading.cannotTell {
		anywhere := false
		for _, stmt := range tree.Stmts {
			govSQLWalkTree(stmt.ProtoReflect(), func(msg protoreflect.Message) bool {
				anywhere = anywhere || e.mentions(nodeOfMessage(msg))
				return !anywhere
			})
		}
		reading.absent = !anywhere
	}
	return reading
}

// nodeOfMessage wraps the two message kinds mentions looks for.
func nodeOfMessage(msg protoreflect.Message) *pg.Node {
	switch n := msg.Interface().(type) {
	case *pg.FuncCall:
		return &pg.Node{Node: &pg.Node_FuncCall{FuncCall: n}}
	case *pg.TypeCast:
		return &pg.Node{Node: &pg.Node_TypeCast{TypeCast: n}}
	}
	return nil
}

// govSQLTimeOfDayLike is true when the query reads the time of day in some form (to_char, date_trunc to a unit
// below the day, a cast to time, a clock literal, EXTRACT of a unit below the day). A question about the hour is
// not shown to be unanswered by a query that reads it in a form this check does not follow.
func govSQLTimeOfDayLike(tree *pg.ParseResult) bool {
	found := false
	for _, stmt := range tree.Stmts {
		govSQLWalkTree(stmt.ProtoReflect(), func(msg protoreflect.Message) bool {
			switch n := msg.Interface().(type) {
			case *pg.FuncCall:
				switch govSQLFuncName(n) {
				case "to_char", "timezone", "age":
					found = true
				case "extract", "date_part", "date_trunc":
					if len(n.Args) > 0 {
						if _, field, _, isText, ok := govSQLConstOf(n.Args[0]); ok && isText {
							switch strings.ToLower(field) {
							case "hour", "minute", "second", "epoch", "milliseconds", "microseconds":
								found = true
							}
						}
					}
				}
			case *pg.TypeCast:
				found = found || govSQLCastName(n) == "time"
			case *pg.A_Const:
				if s := n.GetSval(); s != nil && govSQLRxTimeOfDay.MatchString(s.Sval) {
					found = true
				}
			}
			return !found
		})
	}
	return found
}

// govSQLDayOfWeekLike is true when the query reads the day of the week in some form this check may not follow.
func govSQLDayOfWeekLike(tree *pg.ParseResult) bool {
	found := false
	for _, stmt := range tree.Stmts {
		govSQLWalkTree(stmt.ProtoReflect(), func(msg protoreflect.Message) bool {
			if n, ok := msg.Interface().(*pg.FuncCall); ok {
				switch govSQLFuncName(n) {
				case "to_char", "age":
					found = true
				case "extract", "date_part", "date_trunc":
					if len(n.Args) > 0 {
						if _, field, _, isText, ok := govSQLConstOf(n.Args[0]); ok && isText {
							switch strings.ToLower(field) {
							case "dow", "isodow", "doy", "week", "epoch", "isoyear", "julian":
								found = true
							}
						}
					}
				}
			}
			return !found
		})
	}
	return found
}

// ---------------------------------------------------------------- the checks

// govSQLHoursText writes the hours a set of seconds touches as runs, wrapping past midnight: the hours 23, 0, 1
// and 2 are "23 to 2".
func govSQLHoursText(kept []bool) string {
	var inSet [24]bool
	count := 0
	for second, ok := range kept {
		if ok && second/3600 < 24 && !inSet[second/3600] {
			inSet[second/3600] = true
			count++
		}
	}
	switch count {
	case 0:
		return "none"
	case 24:
		return "every hour"
	}
	var runs []string
	for h := 0; h < 24; h++ {
		if !inSet[h] || inSet[(h+23)%24] { // a run starts where the hour before it is not in the set
			continue
		}
		end := h
		for inSet[(end+1)%24] {
			end = (end + 1) % 24
		}
		if end == h {
			runs = append(runs, strconv.Itoa(h))
		} else {
			runs = append(runs, fmt.Sprintf("%d to %d", h, end))
		}
	}
	return strings.Join(runs, ", ")
}

// govSQLCheckWindow holds a clock window and a set of days to what the query's conditions keep.
func govSQLCheckWindow(facts govSQLQuestionFacts, validated *govSQLValidated) []govSQLObligation {
	if validated == nil || validated.Tree == nil || validated.View == nil {
		return nil
	}
	var out []govSQLObligation
	target := govSQLTimeTarget(facts.Question, validated.View)
	if w := facts.Window; w != nil {
		e := &govSQLEvaluation{
			reads: func(t govSQLTerm) bool { return t == govSQLTermHour || t == govSQLTermClock },
			value: func(t govSQLTerm, second int) (float64, string) {
				if t == govSQLTermHour {
					return float64(second / 3600), ""
				}
				return float64(second), ""
			},
		}
		reading := govSQLReadConditions(validated.Tree, e, 86400, func(second int) bool { return w.holds(second) }, 59)
		unmet := reading.mismatch != nil || (reading.absent && !govSQLTimeOfDayLike(validated.Tree))
		if unmet {
			shows := "The query has no condition on the hour of the day."
			if reading.mismatch != nil {
				shows = "The query keeps the hours " + govSQLHoursText(reading.mismatch) + "."
			}
			out = append(out, govSQLObligation{
				Kind: "WINDOW", Subject: w.Phrase,
				Message: fmt.Sprintf("The question asks for the window %q: from %02d:00 up to, but not including, %02d:00, which is the hours %s. %s %s",
					w.Phrase, w.Start, w.End%24, govSQLJoinWords(govSQLIntWords(w.hours())), shows, govSQLWindowAdvice(w, target)),
				Reason: fmt.Sprintf("the query did not keep exactly the hours of the window the question gives (%s)", w.Phrase),
			})
		}
	}
	if s := facts.Days; s != nil {
		e := &govSQLEvaluation{
			reads: func(t govSQLTerm) bool {
				return t == govSQLTermDOW || t == govSQLTermISODOW || t == govSQLTermDayName || t == govSQLTermDayD || t == govSQLTermDayID
			},
			value: func(t govSQLTerm, day int) (float64, string) {
				switch t {
				case govSQLTermDOW:
					return float64(day), ""
				case govSQLTermDayD:
					return float64(day + 1), ""
				case govSQLTermISODOW, govSQLTermDayID:
					if day == 0 {
						return 7, ""
					}
					return float64(day), ""
				}
				return 0, govSQLDayKey(govSQLDayNames[day])
			},
		}
		reading := govSQLReadConditions(validated.Tree, e, 7, func(day int) bool { return s.Days[day] }, 0)
		unmet := reading.mismatch != nil || (reading.absent && !govSQLDayOfWeekLike(validated.Tree))
		if unmet {
			shows := "The query has no condition on the day of the week."
			if reading.mismatch != nil {
				var kept []string
				for d := 1; d <= 7; d++ {
					if reading.mismatch[d%7] {
						kept = append(kept, govSQLDayNames[d%7])
					}
				}
				if len(kept) == 0 {
					shows = "The query keeps no day at all."
				} else {
					shows = "The query keeps " + govSQLJoinWords(kept) + "."
				}
			}
			out = append(out, govSQLObligation{
				Kind: "DAYS", Subject: s.Phrase,
				Message: fmt.Sprintf("The question asks for %s, which is %s. %s EXTRACT(DOW ...) numbers Sunday 0 to Saturday 6, EXTRACT(ISODOW ...) numbers Monday 1 to Sunday 7. Use EXTRACT(ISODOW FROM column) IN (%s) on %s.",
					s.Phrase, govSQLJoinWords(s.names()), shows, strings.Join(s.isodow(), ", "), target),
				Reason: fmt.Sprintf("the query did not keep exactly the days the question names (%s: %s)", s.Phrase, govSQLJoinWords(s.names())),
			})
		}
	}
	return out
}

func govSQLIntWords(values []int) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		out = append(out, strconv.Itoa(v))
	}
	return out
}

// govSQLWindowAdvice writes the condition for a window, where it goes, and why a window across midnight is not a BETWEEN.
func govSQLWindowAdvice(w *govSQLClockWindow, target string) string {
	const hour = "EXTRACT(HOUR FROM column)"
	switch {
	case w.crossesMidnight():
		return fmt.Sprintf("The window crosses midnight, so write %s >= %d OR %s < %d on %s (BETWEEN %d AND %d keeps no hour, because the first is larger than the second).",
			hour, w.Start, hour, w.End, target, w.Start, w.End)
	case w.End >= 24:
		return fmt.Sprintf("Write %s >= %d on %s.", hour, w.Start, target)
	}
	return fmt.Sprintf("Write %s >= %d AND %s < %d on %s (the hour %d is not in the window).", hour, w.Start, hour, w.End, target, w.End)
}

// govSQLWindowHints are what the question's own message says about a window of the day and about days of the week.
func govSQLWindowHints(facts govSQLQuestionFacts, view *govSQLView) []string {
	var hints []string
	target := "the record's time column"
	if view != nil {
		target = govSQLTimeTarget(facts.Question, view)
	}
	if w := facts.Window; w != nil {
		hints = append(hints, fmt.Sprintf("The question gives a window of the day, %q: from %02d:00 up to, but not including, %02d:00, which is the hours %s. %s",
			w.Phrase, w.Start, w.End%24, govSQLJoinWords(govSQLIntWords(w.hours())), govSQLWindowAdvice(w, target)))
	}
	if s := facts.Days; s != nil {
		hints = append(hints, fmt.Sprintf("The question names the days %s. Day numbers differ: EXTRACT(ISODOW FROM column) is 1 for Monday to 7 for Sunday (EXTRACT(DOW ...) is 0 for Sunday to 6 for Saturday, and to_char(column, 'D') is 1 for Sunday to 7 for Saturday). Use EXTRACT(ISODOW FROM column) IN (%s) on %s.",
			govSQLJoinWords(s.names()), strings.Join(s.isodow(), ", "), target))
	}
	return hints
}
