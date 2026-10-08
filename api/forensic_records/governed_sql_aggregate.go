package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	pg "github.com/pganalyze/pg_query_go/v6"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// GOVERNED SQL: THE AGGREGATE A QUESTION ASKS FOR, AND THE DAY IT ASKS WHICH OF (RUN 15).
//
// Two more classes the fresh-seed check and the probes found (pre-registration, "Run 15 plan", classes H and J):
//
//	"What is the highest total in the transactions?"       SELECT sum(amount): 129,700 for 75,000. The rules say
//	                                                        "total" is SUM and "largest" is MAX, and the model took
//	                                                        the first word it met.
//	"On which day does X have the most call records?"      "Day with most calls: 3". The query grouped by the day
//	                                                        of the month, so the answer was right only because
//	                                                        every record is in April.
//
// Both are read from the question and from the query's parse tree. A question that asks for the highest (or the
// lowest, or the average) needs the function that gives it somewhere in the query, and a question that asks which
// day needs the date, not its number in the month. Neither check asks a model anything.

var (
	govSQLRxWantsMax = regexp.MustCompile(`(?i)\b(?:highest|largest|biggest|greatest|maximum|longest|latest|newest)\b`)
	govSQLRxWantsMin = regexp.MustCompile(`(?i)\b(?:lowest|smallest|minimum|shortest|earliest|oldest)\b`)
	govSQLRxWantsAvg = regexp.MustCompile(`(?i)\b(?:average|mean)\b`)
	govSQLRxWhichDay = regexp.MustCompile(`(?i)\b(?:which|what)\s+(?:day|date)\b`)
	// "which day of the week" and "which weekday" ask for a day name or number, and are answered by it.
	govSQLRxDayPart = regexp.MustCompile(`(?i)\bdays?\s+of\s+(?:the\s+)?(?:week|month)\b|\bweekdays?\b|\bday-of-(?:week|month)\b`)
)

// govSQLAggregates is what a query computes: the functions it calls, and whether it orders rows and keeps the first.
type govSQLAggregates struct {
	funcs    map[string]bool
	sortDesc bool // some ORDER BY runs downwards
	sortAsc  bool // some ORDER BY runs upwards
	limited  bool // a LIMIT, or a ranking window function
	divides  bool // an expression divides (SUM(x) / COUNT(*))
}

func govSQLReadAggregates(tree *pg.ParseResult) govSQLAggregates {
	a := govSQLAggregates{funcs: map[string]bool{}}
	for _, stmt := range tree.Stmts {
		govSQLWalkTree(stmt.ProtoReflect(), func(msg protoreflect.Message) bool {
			switch n := msg.Interface().(type) {
			case *pg.FuncCall:
				name := govSQLFuncName(n)
				a.funcs[name] = true
				switch name {
				case "rank", "dense_rank", "row_number", "ntile", "first_value", "last_value":
					a.limited = true
				}
			case *pg.SortBy:
				switch n.SortbyDir {
				case pg.SortByDir_SORTBY_DESC:
					a.sortDesc = true
				case pg.SortByDir_SORTBY_DEFAULT, pg.SortByDir_SORTBY_ASC:
					a.sortAsc = true
				}
			case *pg.SelectStmt:
				if n.LimitCount != nil {
					a.limited = true
				}
			case *pg.A_Expr:
				if govSQLOpName(n) == "/" {
					a.divides = true
				}
			}
			return true
		})
	}
	return a
}

// used names the aggregates the query calls, for the message that tells the model what it did.
func (a govSQLAggregates) used() string {
	var names []string
	for _, name := range []string{"sum", "count", "avg", "min", "max"} {
		if a.funcs[name] {
			names = append(names, strings.ToUpper(name))
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		return "no aggregate"
	}
	return strings.Join(names, " and ")
}

func (a govSQLAggregates) takesMax() bool { return a.funcs["max"] || (a.sortDesc && a.limited) }
func (a govSQLAggregates) takesMin() bool { return a.funcs["min"] || (a.sortAsc && a.limited) }
func (a govSQLAggregates) takesAvg() bool {
	return a.funcs["avg"] || (a.divides && (a.funcs["sum"] || a.funcs["count"])) || (a.funcs["sum"] && a.funcs["count"])
}

// govSQLCheckAggregate holds a question that asks for the highest, the lowest or the average to a query that
// computes it. A SUM answers "the total", and "the highest total" is not that.
func govSQLCheckAggregate(facts govSQLQuestionFacts, validated *govSQLValidated) []govSQLObligation {
	if validated == nil || validated.Tree == nil || !(facts.WantsMax || facts.WantsMin || facts.WantsAvg) {
		return nil
	}
	a := govSQLReadAggregates(validated.Tree)
	var out []govSQLObligation
	add := func(subject, message, reason string) {
		out = append(out, govSQLObligation{Kind: "AGGREGATE", Subject: subject, Message: message, Reason: reason})
	}
	if facts.WantsMax && !a.takesMax() {
		add("highest", fmt.Sprintf("The question asks for the highest value (highest, largest, longest, latest). Take the MAX of the matching column, or ORDER BY it DESC with LIMIT 1 to get the row; the query uses %s, which does not give the highest value.", a.used()),
			"the query did not take the highest value the question asks for")
	}
	if facts.WantsMin && !a.takesMin() {
		add("lowest", fmt.Sprintf("The question asks for the lowest value (lowest, smallest, shortest, earliest). Take the MIN of the matching column, or ORDER BY it ASC with LIMIT 1 to get the row; the query uses %s, which does not give the lowest value.", a.used()),
			"the query did not take the lowest value the question asks for")
	}
	if facts.WantsAvg && !a.takesAvg() {
		add("average", fmt.Sprintf("The question asks for an average. Use AVG of the matching column, or SUM divided by COUNT; the query uses %s, which does not give an average.", a.used()),
			"the query did not compute the average the question asks for")
	}
	return out
}

// govSQLAggregateHints say, in the question's own message, which function the question needs.
func govSQLAggregateHints(facts govSQLQuestionFacts) []string {
	var hints []string
	if facts.WantsMax {
		hints = append(hints, "The question asks for the highest value: use MAX of the matching column (or ORDER BY it DESC LIMIT 1), even when the column is called a total.")
	}
	if facts.WantsMin {
		hints = append(hints, "The question asks for the lowest value: use MIN of the matching column (or ORDER BY it ASC LIMIT 1).")
	}
	if facts.WantsAvg {
		hints = append(hints, "The question asks for an average: use AVG of the matching column.")
	}
	if facts.WantsDate {
		hints = append(hints, "The question asks which day: return the calendar date, grouped as date_trunc('day', column)::date (or column::date), not EXTRACT(DAY ...), which is only the day of the month.")
	}
	return hints
}

// ---------------------------------------------------------------- which day

// govSQLDayExpressions reports, for the expressions a SELECT returns or groups by, whether one is the day of
// the month and whether one is a date.
func govSQLDayExpressions(nodes []*pg.Node) (dayOfMonth, date bool) {
	for _, node := range nodes {
		if node == nil {
			continue
		}
		govSQLWalkTree(node.ProtoReflect(), func(msg protoreflect.Message) bool {
			switch n := msg.Interface().(type) {
			case *pg.FuncCall:
				switch govSQLFuncName(n) {
				case "extract", "date_part":
					if len(n.Args) == 2 {
						if _, field, _, isText, ok := govSQLConstOf(n.Args[0]); ok && isText && strings.EqualFold(field, "day") {
							dayOfMonth = true
						}
					}
				case "date":
					date = true
				case "date_trunc":
					if len(n.Args) == 2 {
						if _, unit, _, isText, ok := govSQLConstOf(n.Args[0]); ok && isText {
							switch strings.ToLower(unit) {
							case "day", "hour", "minute", "second":
								date = true
							}
						}
					}
				case "to_char":
					if len(n.Args) == 2 {
						if _, format, _, isText, ok := govSQLConstOf(n.Args[1]); ok && isText {
							f := strings.ToLower(format)
							switch {
							case strings.Contains(f, "yyyy") && strings.Contains(f, "dd"), strings.Contains(f, "yy-mm-dd"):
								date = true
							case strings.TrimPrefix(strings.TrimSpace(f), "fm") == "dd":
								dayOfMonth = true
							}
						}
					}
				}
			case *pg.TypeCast:
				if govSQLCastName(n) == "date" {
					date = true
				}
			}
			return true
		})
	}
	return dayOfMonth, date
}

// govSQLCheckDay holds "which day" to a query that returns the date. A SELECT that returns or groups by the day
// of the month and nowhere by a date answers with a number between 1 and 31.
func govSQLCheckDay(facts govSQLQuestionFacts, validated *govSQLValidated) []govSQLObligation {
	if validated == nil || validated.Tree == nil || !facts.WantsDate {
		return nil
	}
	flagged := false
	for _, stmt := range validated.Tree.Stmts {
		govSQLWalkTree(stmt.ProtoReflect(), func(msg protoreflect.Message) bool {
			sel, ok := msg.Interface().(*pg.SelectStmt)
			if !ok {
				return true
			}
			nodes := append(append([]*pg.Node{}, sel.TargetList...), sel.GroupClause...)
			if dayOfMonth, date := govSQLDayExpressions(nodes); dayOfMonth && !date {
				flagged = true
			}
			return true
		})
	}
	if !flagged {
		return nil
	}
	return []govSQLObligation{{
		Kind: "DATE", Subject: "which day",
		Message: "The question asks which day, and the answer is a date. The query returns the day of the month (EXTRACT(DAY ...)), a number from 1 to 31 that is the same in every month. Group by the date: date_trunc('day', column)::date, or column::date, and return it.",
		Reason:  "the query returned the day of the month where the question asks which day (a date)",
	}}
}
