package main

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	pg "github.com/pganalyze/pg_query_go/v6"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// GOVERNED SQL: THE GUARDS OF RUN 14.
//
// Phase 1 (pre-registration, "Phase 1 results") read the lane first against 161 unseen questions and the
// 103-question corpus. The lane was right on 146 of the 161, and the questions it got wrong, and three it got
// wrong on the corpus, fell into a few classes the factory's question templates do not produce. Each guard
// below closes one class with a check that is read from the question and the query, never from a model, and
// none changes the shared prompt rules:
//
//	range end     "between 2026-04-26 and 2026-05-30" includes all of the 30th; a bound at its midnight does not
//	case          a zero that matching ignoring letter case refutes ('ACTIVE' asked, 'active' stored) is not shown
//	comparison    "incoming versus outgoing" asks for one row per value, not one total
//	subject       a ranking of contacts whose winner is the number the question names ranks the wrong thing
//	text search   a search of transcripts, images or documents is the retrieval path's, and no word in it is a name
//
// plus two things the answer says: a rounded figure that would look whole keeps its digits, and a count of
// distinct values says "distinct".

// ---------------------------------------------------------------- the range of two dates

// govSQLDateRange is a range of two calendar dates the question gives outright. Both days are in it.
type govSQLDateRange struct {
	Start, End string // ISO dates, as written
	// Calendar is set when the question names a calendar unit, not two dates ("in May 2026", "in 2025", "on 2026-04-02"):
	// the window is the whole unit, and it is judged leniently (govSQLCheckRange).
	Calendar string
}

// "until" and "before" are left out on purpose: they do not say whether the last day is in.
var govSQLRxDateRange = regexp.MustCompile(`(?i)\b(?:between|from)\s+(\d{4}-\d{2}-\d{2})\s+(?:and|to|through|thru)\s+(\d{4}-\d{2}-\d{2})\b`)

var govSQLMonthNumber = map[string]time.Month{
	"january": time.January, "jan": time.January, "february": time.February, "feb": time.February, "march": time.March, "mar": time.March,
	"april": time.April, "apr": time.April, "may": time.May, "june": time.June, "jun": time.June, "july": time.July, "jul": time.July,
	"august": time.August, "aug": time.August, "september": time.September, "sep": time.September, "sept": time.September,
	"october": time.October, "oct": time.October, "november": time.November, "nov": time.November, "december": time.December, "dec": time.December,
}

var (
	govSQLRxMonthYear = regexp.MustCompile(`(?i)(?:\b(in|during|for|from|of|within)\s+)?\b(january|february|march|april|may|june|july|august|september|october|november|december|jan|feb|mar|apr|jun|jul|aug|sept|sep|oct|nov|dec)\.?\s+((?:19|20)\d{2})\b`)
	govSQLRxYearIn    = regexp.MustCompile(`(?i)\b(?:in|during|for|of)\s+((?:19|20)\d{2})\b`)
	govSQLRxOnDate    = regexp.MustCompile(`(?i)\b(?:on|during)\s+(\d{4}-\d{2}-\d{2})\b`)
	// A question that compares, or bounds one side, does not name a window.
	govSQLRxNotAWindow = regexp.MustCompile(`(?i)\b(?:between|since|before|after|until|till|through|thru|versus|vs|compared|compare|last|past|next|previous|this|every|each)\b`)
)

// govSQLReadCalendarWindow reads the one calendar unit a question names: "in May 2026" is the whole of May, "in 2025"
// the whole year, "on 2026-04-02" that day. Anything with two units, a comparison or a one-sided bound is not a
// window and is left to the rest of the verifier.
func govSQLReadCalendarWindow(text string) *govSQLDateRange {
	if govSQLRxNotAWindow.MatchString(text) {
		return nil
	}
	months := govSQLRxMonthYear.FindAllStringSubmatch(text, -1)
	dates := govSQLRxISODate.FindAllString(text, -1)
	switch {
	case len(months) == 1 && len(dates) == 0 && !govSQLRxSlashDate.MatchString(text):
		month, ok := govSQLMonthNumber[strings.ToLower(months[0][2])]
		var year int
		_, _ = fmt.Sscanf(months[0][3], "%d", &year)
		if !ok || year == 0 {
			return nil
		}
		first := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
		last := first.AddDate(0, 1, -1)
		return &govSQLDateRange{Start: first.Format("2006-01-02"), End: last.Format("2006-01-02"), Calendar: strings.TrimSpace(months[0][2] + " " + months[0][3])}
	case len(months) == 0 && len(dates) == 0:
		years := govSQLRxYearIn.FindAllStringSubmatch(text, -1)
		if len(years) != 1 {
			return nil
		}
		return &govSQLDateRange{Start: years[0][1] + "-01-01", End: years[0][1] + "-12-31", Calendar: years[0][1]}
	case len(months) == 0 && len(dates) == 1:
		on := govSQLRxOnDate.FindStringSubmatch(text)
		if on == nil {
			return nil
		}
		if _, err := time.Parse("2006-01-02", on[1]); err != nil {
			return nil
		}
		return &govSQLDateRange{Start: on[1], End: on[1], Calendar: on[1]}
	}
	return nil
}

func govSQLReadDateRange(text string) *govSQLDateRange {
	m := govSQLRxDateRange.FindStringSubmatch(text)
	if m == nil {
		return nil
	}
	start, errStart := time.Parse("2006-01-02", m[1])
	end, errEnd := time.Parse("2006-01-02", m[2])
	if errStart != nil || errEnd != nil || end.Before(start) {
		return nil
	}
	return &govSQLDateRange{Start: m[1], End: m[2]}
}

// govSQLTimeBound is one comparison of a time column with a constant, read from the query.
type govSQLTimeBound struct {
	Column    string
	Upper     bool      // an upper bound; otherwise a lower one
	Inclusive bool      // <= and >=, and both ends of BETWEEN
	DayLevel  bool      // the column is a date, or is cast or truncated to one: the comparison is by day
	At        time.Time // the constant on the case clock (a date alone is its midnight)
	HasTime   bool      // the constant carries a time of day
}

func govSQLOpName(expr *pg.A_Expr) string {
	if len(expr.Name) == 0 {
		return ""
	}
	if s := expr.Name[len(expr.Name)-1].GetString_(); s != nil {
		return strings.ToUpper(s.Sval)
	}
	return ""
}

func govSQLFuncName(call *pg.FuncCall) string {
	if call == nil || len(call.Funcname) == 0 {
		return ""
	}
	if s := call.Funcname[len(call.Funcname)-1].GetString_(); s != nil {
		return strings.ToLower(s.Sval)
	}
	return ""
}

func govSQLCastName(cast *pg.TypeCast) string {
	if cast == nil || cast.TypeName == nil || len(cast.TypeName.Names) == 0 {
		return ""
	}
	if s := cast.TypeName.Names[len(cast.TypeName.Names)-1].GetString_(); s != nil {
		return strings.ToLower(s.Sval)
	}
	return ""
}

// govSQLTimeExpression resolves a column, a column cast to a date, date(column) or date_trunc('day', column)
// to the view's time column it reads, and says whether the comparison is by day. Anything else is not one.
func govSQLTimeExpression(node *pg.Node, view *govSQLView) (column string, dayLevel bool, ok bool) {
	if node == nil {
		return "", false, false
	}
	switch {
	case node.GetColumnRef() != nil:
		fields := node.GetColumnRef().Fields
		if len(fields) == 0 || fields[len(fields)-1].GetString_() == nil {
			return "", false, false
		}
		name := fields[len(fields)-1].GetString_().Sval
		for _, c := range view.Columns {
			if c.Name == name && (c.Type == "timestamptz" || c.Type == "date") {
				// The layer types activation_date as a timestamp, but what it holds is a day; a column the
				// layer calls a date is compared by day.
				return name, c.Type == "date" || strings.HasSuffix(c.Name, "_date"), true
			}
		}
	case node.GetTypeCast() != nil:
		inner, day, ok := govSQLTimeExpression(node.GetTypeCast().Arg, view)
		if !ok {
			return "", false, false
		}
		switch govSQLCastName(node.GetTypeCast()) {
		case "date":
			return inner, true, true
		case "timestamp", "timestamptz":
			return inner, day, true
		}
	case node.GetFuncCall() != nil:
		call := node.GetFuncCall()
		switch govSQLFuncName(call) {
		case "date":
			if len(call.Args) == 1 {
				if inner, _, ok := govSQLTimeExpression(call.Args[0], view); ok {
					return inner, true, true
				}
			}
		case "date_trunc":
			if len(call.Args) == 2 {
				if unit := call.Args[0].GetAConst(); unit != nil && unit.GetSval() != nil && strings.EqualFold(unit.GetSval().Sval, "day") {
					if inner, _, ok := govSQLTimeExpression(call.Args[1], view); ok {
						return inner, true, true
					}
				}
			}
		}
	}
	return "", false, false
}

var govSQLTimeLayouts = []struct {
	layout  string
	hasTime bool
	zoned   bool
}{
	{"2006-01-02", false, false},
	{"2006-01-02 15:04:05", true, false}, // a fractional second is accepted when parsing
	{"2006-01-02T15:04:05", true, false},
	{"2006-01-02 15:04", true, false},
	{"2006-01-02 15:04:05Z07:00", true, true},
	{"2006-01-02T15:04:05Z07:00", true, true},
	{"2006-01-02 15:04:05Z07", true, true},
}

func govSQLParseInstant(text string, loc *time.Location) (time.Time, bool, bool) {
	text = strings.TrimSpace(text)
	for _, l := range govSQLTimeLayouts {
		var t time.Time
		var err error
		if l.zoned {
			t, err = time.Parse(l.layout, text)
		} else {
			t, err = time.ParseInLocation(l.layout, text, loc)
		}
		if err == nil {
			return t.In(loc), l.hasTime, true
		}
	}
	return time.Time{}, false, false
}

var govSQLRxInterval = regexp.MustCompile(`(?i)^\s*(\d{1,4})\s*(day|days|hour|hours)\s*$`)

// govSQLConstInstant evaluates a literal date or timestamp, optionally cast, optionally plus or minus an interval
// of days or hours or a number of days. Anything it cannot evaluate is not a bound the check can read.
func govSQLConstInstant(node *pg.Node, loc *time.Location) (at time.Time, hasTime bool, ok bool) {
	if node == nil {
		return time.Time{}, false, false
	}
	switch {
	case node.GetAConst() != nil:
		if s := node.GetAConst().GetSval(); s != nil {
			return govSQLParseInstant(s.Sval, loc)
		}
	case node.GetTypeCast() != nil:
		return govSQLConstInstant(node.GetTypeCast().Arg, loc)
	case node.GetAExpr() != nil:
		expr := node.GetAExpr()
		op := govSQLOpName(expr)
		if op != "+" && op != "-" {
			return time.Time{}, false, false
		}
		base, baseTime, ok := govSQLConstInstant(expr.Lexpr, loc)
		if !ok {
			return time.Time{}, false, false
		}
		sign := 1
		if op == "-" {
			sign = -1
		}
		right := expr.Rexpr
		if right != nil && right.GetAConst() != nil && right.GetAConst().GetIval() != nil {
			return base.AddDate(0, 0, sign*int(right.GetAConst().GetIval().Ival)), baseTime, true
		}
		if right != nil && right.GetTypeCast() != nil && govSQLCastName(right.GetTypeCast()) == "interval" {
			if s := right.GetTypeCast().Arg.GetAConst(); s != nil && s.GetSval() != nil {
				if m := govSQLRxInterval.FindStringSubmatch(s.GetSval().Sval); m != nil {
					var n int
					_, _ = fmt.Sscanf(m[1], "%d", &n)
					if strings.HasPrefix(strings.ToLower(m[2]), "day") {
						return base.AddDate(0, 0, sign*n), baseTime, true
					}
					return base.Add(time.Duration(sign*n) * time.Hour), true, true
				}
			}
		}
	}
	return time.Time{}, false, false
}

// govSQLTimeBounds lists every bound the query puts on a time column of its view.
func govSQLTimeBounds(tree *pg.ParseResult, view *govSQLView, loc *time.Location) []govSQLTimeBound {
	var out []govSQLTimeBound
	add := func(column string, day bool, upper, inclusive bool, constant *pg.Node) {
		at, hasTime, ok := govSQLConstInstant(constant, loc)
		if !ok {
			return
		}
		out = append(out, govSQLTimeBound{Column: column, Upper: upper, Inclusive: inclusive, DayLevel: day, At: at, HasTime: hasTime})
	}
	for _, stmt := range tree.Stmts {
		govSQLWalkTree(stmt.ProtoReflect(), func(msg protoreflect.Message) bool {
			expr, ok := msg.Interface().(*pg.A_Expr)
			if !ok {
				return true
			}
			switch op := govSQLOpName(expr); op {
			case "<", "<=", ">", ">=":
				column, day, ok := govSQLTimeExpression(expr.Lexpr, view)
				constant := expr.Rexpr
				if !ok {
					// the constant is on the left: 'x' > column is column < 'x'
					column, day, ok = govSQLTimeExpression(expr.Rexpr, view)
					constant = expr.Lexpr
					if !ok {
						return true
					}
					op = map[string]string{"<": ">", "<=": ">=", ">": "<", ">=": "<="}[op]
				}
				add(column, day, op == "<" || op == "<=", op == "<=" || op == ">=", constant)
			case "BETWEEN":
				column, day, ok := govSQLTimeExpression(expr.Lexpr, view)
				if !ok || expr.Rexpr == nil || expr.Rexpr.GetList() == nil || len(expr.Rexpr.GetList().Items) != 2 {
					return true
				}
				items := expr.Rexpr.GetList().Items
				add(column, day, false, true, items[0])
				add(column, day, true, true, items[1])
			}
			return true
		})
	}
	return out
}

func govSQLDayStart(t time.Time, loc *time.Location) time.Time {
	t = t.In(loc)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
}

// govSQLRangeSlack is how far from the edge of the range a bound may be and still count as the edge: a bound
// written at the stroke of a minute or a second is the edge, one a whole day away is another range.
const govSQLRangeSlack = 12 * time.Hour

// edge is where the bound reaches. For a lower bound it is the first instant the bound admits, for an upper
// bound the first instant it no longer admits. A date compared by day admits whole days; a timestamp admits up
// to its own instant, so "<= '2026-05-30'" stops at the midnight that begins the 30th.
func (b govSQLTimeBound) edge(loc *time.Location) time.Time {
	if b.DayLevel {
		day := govSQLDayStart(b.At, loc)
		if b.Upper == b.Inclusive { // date <= D reaches the end of D; date > D starts the day after D
			return day.AddDate(0, 0, 1)
		}
		return day
	}
	if b.Upper == b.Inclusive { // <= t admits t itself; > t starts just after t
		return b.At.Add(time.Second)
	}
	return b.At
}

// matches says whether the bound is the edge of the range: a lower bound at the start of the first day, an
// upper bound at the end of the last. A bound short of the edge leaves days out; one well past it adds days.
func (b govSQLTimeBound) matches(first, endExclusive time.Time, loc *time.Location) bool {
	edge := b.edge(loc)
	if !b.Upper {
		return !edge.After(first) && first.Sub(edge) < govSQLRangeSlack
	}
	return !edge.Before(endExclusive) && edge.Sub(endExclusive) < govSQLRangeSlack
}

// govSQLCheckRange holds a range of two dates to both days in full, and to those days only. The query's bounds
// on one time column must start at the first day and end at the end of the last. A bound at the midnight that
// begins the last day (< '2026-05-30', <= '2026-05-30', BETWEEN on a timestamp) leaves that day out: the
// measured case answered 914 for 1,013 (Phase 1, demo CDR-date_range-01). A bound a day too far adds one.
func govSQLCheckRange(facts govSQLQuestionFacts, validated *govSQLValidated) []govSQLObligation {
	if facts.Range == nil || validated == nil || validated.View == nil {
		return nil
	}
	view := validated.View
	if govSQLTimeColumns(view) == "" {
		return nil
	}
	_, loc := govSQLTimeZone()
	first, _ := time.ParseInLocation("2006-01-02", facts.Range.Start, loc)
	last, _ := time.ParseInLocation("2006-01-02", facts.Range.End, loc)
	endExclusive := last.AddDate(0, 0, 1)
	byColumn := map[string][]govSQLTimeBound{}
	for _, b := range govSQLTimeBounds(validated.Tree, view, loc) {
		byColumn[b.Column] = append(byColumn[b.Column], b)
	}
	// A calendar unit is judged only by the bounds the query puts on a time column: a month read through EXTRACT,
	// date_trunc or to_char is not a pair of bounds, and a query with no bounds at all is the time check's to flag.
	if facts.Range.Calendar != "" && (len(byColumn) == 0 || govSQLReadsTimeByFunction(validated.Tree)) {
		return nil
	}
	column := ""
	for name, bounds := range byColumn {
		lower, upper := false, false
		for _, b := range bounds {
			if b.matches(first, endExclusive, loc) {
				lower = lower || !b.Upper
				upper = upper || b.Upper
			}
		}
		if lower && upper {
			return nil
		}
		if column == "" || name < column {
			column = name
		}
	}
	if column == "" {
		if primary := govSQLPrimaryTime(view); primary != nil {
			column = primary.Name
		} else {
			for _, c := range view.Columns {
				if c.Type == "timestamptz" || c.Type == "date" {
					column = c.Name
					break
				}
			}
		}
	}
	return []govSQLObligation{{
		Kind: "RANGE", Subject: facts.Range.Start + " to " + facts.Range.End,
		Message: fmt.Sprintf("The question asks for the range from %s to %s, and a range of dates includes ALL of both days and no other day. The query's range does not match it (for example it stops at the midnight that begins %s, or runs past %s). Write the range as %s >= '%s' AND %s < '%s' (the day after %s), or compare %s::date BETWEEN '%s' AND '%s'.",
			facts.Range.Start, facts.Range.End, facts.Range.End, facts.Range.End, column, facts.Range.Start, column, endExclusive.Format("2006-01-02"), facts.Range.End, column, facts.Range.Start, facts.Range.End),
		Reason: fmt.Sprintf("the query did not include both of the days %s and %s in full, and only those days", facts.Range.Start, facts.Range.End),
	}}
}

// ---------------------------------------------------------------- a zero that letter case refutes

// govSQLCaseMismatch is a text column the query compared with a string literal that matches no row exactly
// but matches rows when letter case is ignored.
type govSQLCaseMismatch struct {
	Column  string
	Literal string
}

func (m govSQLCaseMismatch) obligation() govSQLObligation {
	return govSQLObligation{
		Kind: "CASE", Subject: m.Literal,
		Message: fmt.Sprintf("Nothing matched %s = '%s', but records do match when letter case is ignored, so the stored value is written in a different case. Compare ignoring case: LOWER(%s) = LOWER('%s').", m.Column, m.Literal, m.Column, m.Literal),
		Reason:  fmt.Sprintf("the query compared %s with %q, but the records write that value with different letter case, so a count of zero would be wrong", m.Column, m.Literal),
	}
}

// govSQLIsFolded is true when a comparison's column side is wrapped in lower() or upper().
func govSQLIsFolded(node *pg.Node) bool {
	for node != nil {
		switch {
		case node.GetTypeCast() != nil:
			node = node.GetTypeCast().Arg
		case node.GetFuncCall() != nil:
			switch govSQLFuncName(node.GetFuncCall()) {
			case "lower", "upper":
				return true
			}
			if len(node.GetFuncCall().Args) == 0 {
				return false
			}
			node = node.GetFuncCall().Args[0]
		default:
			return false
		}
	}
	return false
}

// govSQLProbeCase asks the database, once and with every literal bound as a parameter, whether the string
// literals the query compared text columns with match exactly and whether they match ignoring letter case. It
// returns the first literal that matches only the second way. No model text is in the SQL.
func govSQLProbeCase(ctx context.Context, db *pgxpool.Pool, req hybridQueryRequest, validated *govSQLValidated) (*govSQLCaseMismatch, error) {
	textColumns := map[string]bool{}
	for _, c := range validated.View.Columns {
		if c.Type == "text" {
			textColumns[c.Name] = true
		}
	}
	type slot struct{ column, literal string }
	var slots []slot
	seen := map[string]bool{}
	for _, cmp := range govSQLComparisons(validated.Tree) {
		if cmp.Op != "=" || cmp.Folded || !textColumns[cmp.Column] {
			continue
		}
		for _, literal := range cmp.Consts {
			key := cmp.Column + "\x00" + literal
			// A phone-like number is searched by its digits (govSQLProbeIdentifiers); letter case is not its problem.
			if literal == "" || seen[key] || len(govSQLDigits(literal)) >= 7 {
				continue
			}
			seen[key] = true
			slots = append(slots, slot{cmp.Column, literal})
		}
	}
	if len(slots) == 0 {
		return nil, nil
	}
	cte, args, err := govSQLCTE(validated.View, req, nil)
	if err != nil {
		return nil, err
	}
	unions := make([]string, 0, len(slots))
	for i, s := range slots {
		args = append(args, s.literal)
		n := len(args)
		unions = append(unions, fmt.Sprintf("SELECT %d AS slot, COUNT(*) FILTER (WHERE %s = $%d::text) AS exact, COUNT(*) FILTER (WHERE LOWER(%s) = LOWER($%d::text)) AS folded FROM %s",
			i, s.column, n, s.column, n, validated.View.Name))
	}
	final := "WITH " + cte + "\n" + strings.Join(unions, "\nUNION ALL\n")
	result, err := govSQLRun(ctx, db, req.TenantID, final, args, 100, 0)
	if err != nil {
		return nil, err
	}
	for _, row := range result.Rows {
		index := int(int64FromAny(row[0]))
		if index >= 0 && index < len(slots) && int64FromAny(row[1]) == 0 && int64FromAny(row[2]) > 0 {
			return &govSQLCaseMismatch{Column: slots[index].column, Literal: slots[index].literal}, nil
		}
	}
	return nil, nil
}

// ---------------------------------------------------------------- a comparison, and a ranking of the subject

var govSQLRxCompare = regexp.MustCompile(`(?i)\b(?:versus|vs\.?|compared\s+(?:to|with)|compare[sd]?|comparison\s+(?:of|between))\b`)

var govSQLRxRanking = regexp.MustCompile(`(?i)\b(?:most|least|top|highest|lowest|frequent(?:ly)?|common(?:ly)?|rank(?:ed|ing)?)\b`)

// govSQLCompareUnits are the calendar units a comparison sets side by side: "May 2026 versus June 2026". The values
// compared are not a column, so "one row per value" says nothing the model can act on (run 15, probes
// SHAPE-compare_months-01 and SHAPE2-versus_months-01: declined four times in four); the unit and its keys can be said.
type govSQLCompareUnits struct {
	Unit   string   // "month", "year" or "day"
	Keys   []string // 2026-05, 2026-06 / 2025, 2026 / 2026-04-02, 2026-04-03
	Format string   // the to_char format that gives such a key
}

var govSQLRxBareYear = regexp.MustCompile(`\b((?:19|20)\d{2})\b`)

// govSQLReadCompareUnits reads two or more months, years or days of a question that compares them.
func govSQLReadCompareUnits(text string) *govSQLCompareUnits {
	if !govSQLRxCompare.MatchString(text) {
		return nil
	}
	if months := govSQLRxMonthYear.FindAllStringSubmatch(text, -1); len(months) >= 2 {
		keys := make([]string, 0, len(months))
		for _, m := range months {
			month, ok := govSQLMonthNumber[strings.ToLower(m[2])]
			if !ok {
				return nil
			}
			var year int
			_, _ = fmt.Sscanf(m[3], "%d", &year)
			keys = append(keys, fmt.Sprintf("%04d-%02d", year, int(month)))
		}
		return &govSQLCompareUnits{Unit: "month", Keys: keys, Format: "YYYY-MM"}
	}
	if dates := govSQLRxISODate.FindAllString(text, -1); len(dates) >= 2 {
		return &govSQLCompareUnits{Unit: "day", Keys: dates, Format: "YYYY-MM-DD"}
	}
	if !govSQLRxMonth.MatchString(text) {
		seen := map[string]bool{}
		var keys []string
		for _, y := range govSQLRxBareYear.FindAllString(text, -1) {
			if !seen[y] {
				seen[y] = true
				keys = append(keys, y)
			}
		}
		if len(keys) >= 2 {
			return &govSQLCompareUnits{Unit: "year", Keys: keys, Format: "YYYY"}
		}
	}
	return nil
}

// advice says how to write such a comparison: one row per unit, grouped on the unit's key.
func (c *govSQLCompareUnits) advice(target string) string {
	return fmt.Sprintf("The question compares %ss (%s). Return one row per %s: SELECT to_char(column, '%s') AS %s, COUNT(*) AS number_of_records FROM the view WHERE the time column is within one of those %ss GROUP BY 1 ORDER BY 1, with the time condition on %s.",
		c.Unit, strings.Join(c.Keys, ", "), c.Unit, c.Format, c.Unit, c.Unit, target)
}

// govSQLConditionalAggregates counts the columns of the outermost SELECT that count or add up one side of a
// comparison inside a single row: COUNT(*) FILTER (WHERE month = 5), SUM(CASE WHEN ... THEN 1 END).
func govSQLConditionalAggregates(tree *pg.ParseResult) int {
	if tree == nil || len(tree.Stmts) == 0 || tree.Stmts[0].Stmt == nil || tree.Stmts[0].Stmt.GetSelectStmt() == nil {
		return 0
	}
	count := 0
	for _, target := range tree.Stmts[0].Stmt.GetSelectStmt().TargetList {
		node := target
		if res := target.GetResTarget(); res != nil {
			node = res.Val
		}
		for node != nil && node.GetTypeCast() != nil {
			node = node.GetTypeCast().Arg
		}
		if node == nil || node.GetFuncCall() == nil {
			continue
		}
		call := node.GetFuncCall()
		switch govSQLFuncName(call) {
		case "count", "sum", "avg", "min", "max":
		default:
			continue
		}
		conditional := call.AggFilter != nil
		for _, arg := range call.Args {
			govSQLWalkTree(arg.ProtoReflect(), func(msg protoreflect.Message) bool {
				if _, ok := msg.Interface().(*pg.CaseExpr); ok {
					conditional = true
				}
				return !conditional
			})
		}
		if conditional {
			count++
		}
	}
	return count
}

// govSQLCheckCompare: a question that compares values asks for a value for each: one row per value, or one row
// whose columns each count one side (COUNT(*) FILTER (WHERE ...)). One total answers a different question (Phase 1,
// corpus CDR-13: "incoming versus outgoing" answered with one total of calls). A one-row answer with two
// conditional columns was declined as "one value" by run 14 (run 15 probe SHAPE-compare_months-01).
func govSQLCheckCompare(facts govSQLQuestionFacts, validated *govSQLValidated, result *govSQLResult) *govSQLObligation {
	if !facts.WantsCompare || len(result.Rows) >= 2 {
		return nil
	}
	if validated != nil && len(result.Rows) == 1 && len(result.Columns) >= 2 && govSQLConditionalAggregates(validated.Tree) >= 2 {
		return nil
	}
	message := "The question compares values, so it needs one row per value: GROUP BY the column that holds them and return the count of each, not one total."
	if facts.CompareUnits != nil {
		target := "the record's time column"
		if validated != nil && validated.View != nil {
			target = govSQLTimeTarget(facts.Question, validated.View)
		}
		message = "The question compares values, so it needs one row per value, not one total. " + facts.CompareUnits.advice(target)
	}
	return &govSQLObligation{
		Kind: "SHAPE", Subject: "comparison",
		Message: message,
		Reason:  "the query returned one value where the question compares values",
	}
}

func govSQLHasGroupBy(tree *pg.ParseResult) bool {
	found := false
	for _, stmt := range tree.Stmts {
		govSQLWalkTree(stmt.ProtoReflect(), func(msg protoreflect.Message) bool {
			if sel, ok := msg.Interface().(*pg.SelectStmt); ok && len(sel.GroupClause) > 0 {
				found = true
			}
			return true
		})
	}
	return found
}

// govSQLSameIdentifier is true when a result value is the identifier the question names: a phone number by its
// last ten digits, anything else by its letters and digits.
func govSQLSameIdentifier(value, id string) bool {
	a, b := govSQLAlnum(value), govSQLAlnum(id)
	if a == "" || b == "" {
		return false
	}
	if a == b {
		return true
	}
	da, db := govSQLDigits(value), govSQLDigits(id)
	if len(da) >= 7 && len(db) >= 7 && len(govSQLDigits(value)) == len(a) {
		if len(da) > 10 {
			da = da[len(da)-10:]
		}
		if len(db) > 10 {
			db = db[len(db)-10:]
		}
		return da == db
	}
	return false
}

// govSQLCheckSubject: in a ranking of a group, in a question that names exactly one identifier, a group that is
// that identifier ranks the subject against itself. "Who did X contact most?" answered with X and a count of the
// records that carry X (Phase 1, corpus CDR-10). A contact is the other party.
func govSQLCheckSubject(facts govSQLQuestionFacts, validated *govSQLValidated, result *govSQLResult) *govSQLObligation {
	if len(facts.Identifiers) != 1 || !facts.WantsRanking || !govSQLHasGroupBy(validated.Tree) {
		return nil
	}
	id := facts.Identifiers[0]
	for _, row := range result.Rows {
		for _, cell := range row {
			if text, ok := cell.(string); ok && govSQLSameIdentifier(text, id) {
				return &govSQLObligation{
					Kind: "SUBJECT", Subject: id,
					Message: fmt.Sprintf("The ranking contains %s itself, which is the number the question names. A contact, caller or counterpart of %s is the OTHER party: group by the other side's number (the dialed number on the calls %s made, the originating number on the calls it received), across both directions, and leave %s out.", id, id, id, id),
					Reason:  fmt.Sprintf("the ranking returned %s, the number the question names", id),
				}
			}
		}
	}
	return nil
}

// ---------------------------------------------------------------- a search of text evidence

// govSQLTextSearch is a verb that looks into the content of a recording, an image or a document.
var govSQLTextSearch = regexp.MustCompile(`(?i)\b(?:search(?:es|ed|ing)?|look(?:s|ing)?\s+(?:for|up)|mention(?:s|ed|ing)?|say|says|said|saying|spoke|spoken|contain(?:s|ed|ing)?|talk(?:s|ed|ing)?\s+about|quote[sd]?|phrase|keywords?)\b`)

// govSQLSearchesText is true for a question that searches the content of media evidence. The structured views
// hold no text from it, so no query over them can answer, and a word of the search is not a person's name.
func govSQLSearchesText(question string) bool {
	return govSQLMediaCue.MatchString(question) && govSQLTextSearch.MatchString(question)
}

// ---------------------------------------------------------------- what the answer says

// govSQLWholeText is true when a decimal text has nothing but zeros after its point.
func govSQLWholeText(text string) bool {
	i := strings.Index(text, ".")
	return i < 0 || strings.Trim(text[i+1:], "0") == ""
}

// govSQLDistinctTargets says, for each column of the top-level SELECT, whether it is count(DISTINCT ...).
func govSQLDistinctTargets(tree *pg.ParseResult) []bool {
	if tree == nil || len(tree.Stmts) != 1 || tree.Stmts[0].Stmt == nil || tree.Stmts[0].Stmt.GetSelectStmt() == nil {
		return nil
	}
	sel := tree.Stmts[0].Stmt.GetSelectStmt()
	out := make([]bool, 0, len(sel.TargetList))
	for _, target := range sel.TargetList {
		node := target
		if res := target.GetResTarget(); res != nil {
			node = res.Val
		}
		for node != nil && node.GetTypeCast() != nil {
			node = node.GetTypeCast().Arg
		}
		call := (*pg.FuncCall)(nil)
		if node != nil {
			call = node.GetFuncCall()
		}
		out = append(out, call != nil && govSQLFuncName(call) == "count" && call.AggDistinct)
	}
	return out
}

// govSQLDistinctLabel adds the word that count(DISTINCT x) is: "Number of server errors" for the six distinct
// status codes of a log reads as 46 requests.
func govSQLDistinctLabel(label string) string {
	lower := strings.ToLower(label)
	if strings.Contains(lower, "distinct") || strings.Contains(lower, "unique") {
		return label
	}
	for _, prefix := range []string{"Number of ", "Count of ", "Total number of "} {
		if strings.HasPrefix(label, prefix) {
			return prefix + "distinct " + label[len(prefix):]
		}
	}
	return label + " (distinct)"
}

// govSQLLabelResult names the columns of a result as the answer should read them.
func govSQLLabelResult(validated *govSQLValidated, result *govSQLResult) {
	targets := govSQLDistinctTargets(validated.Tree)
	if len(targets) != len(result.Columns) {
		return
	}
	for i, distinct := range targets {
		if distinct {
			if result.Labels == nil {
				result.Labels = make([]string, len(result.Columns))
			}
			result.Labels[i] = govSQLDistinctLabel(govSQLHumanize(result.Columns[i]))
		}
	}
}
