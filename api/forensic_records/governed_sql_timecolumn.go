package main

import (
	"fmt"
	"sort"
	"strings"

	pg "github.com/pganalyze/pg_query_go/v6"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// GOVERNED SQL: WHICH TIME A TIME CONDITION MEANS.
//
// Measured 2026-10-07 (pre-registration, run 11): "How many call records were there at night?" was
// answered 1,771 against a key of 1,768. A call record holds three times, its own, the call's start and
// the call's end, and the model applied "night" to start OR end; the three extra records are calls that
// began before midnight and ended after it. The answer said so, but the question had named neither.
//
// The curated layer already says which time a question about the records means: cdr.event_time is the
// "normalised record timestamp; all case-level time filtering uses this". The hint listed every time column
// of the view in alphabetical order (call_end, call_start, event_time) and the checker accepted any of them.
//
// So a time condition belongs on the record's own time, the normalised timestamp that the dated evidence
// families carry (event_time), unless the question names another time of that record by one of its names
// ("started", "ended", "activation"). A family whose layer has no such field (subscribers, towers) and the
// derived media views are left as they were.

// govSQLTimeNameSkip are the words that say "a time" without saying which one. The evidence's own name
// ("call", "session") is skipped too, read from the view.
var govSQLTimeNameSkip = map[string]bool{
	"time": true, "date": true, "timestamp": true, "datetime": true, "when": true, "period": true, "month": true,
	"year": true, "day": true, "number": true, "type": true, "record": true, "event": true, "data": true,
	"the": true, "of": true, "at": true, "on": true, "in": true, "for": true, "and": true,
}

// govSQLPrimaryTime is the column that holds a record's own time: the normalised timestamp of a records
// view, the one govSQLCTE reads with the no-time rule. A derived view and a family without one have none.
func govSQLPrimaryTime(view *govSQLView) *govSQLColumn {
	if view == nil || view.binding.Derived {
		return nil
	}
	for i := range view.Columns {
		if c := &view.Columns[i]; c.Type == "timestamptz" && c.field.NormalizedName == "timestamp" {
			return c
		}
	}
	return nil
}

// govSQLWordsAlike is true for the same word, or two forms of it that begin the same way: "end" and
// "ended", "start" and "started", "activation" and "activated". A doubtful case is decided in favour of
// letting the query use the column, which is what happened before this check existed.
func govSQLWordsAlike(a, b string) bool {
	if a == b {
		return true
	}
	common := 0
	for common < len(a) && common < len(b) && a[common] == b[common] {
		common++
	}
	need := len(a)
	if len(b) < need {
		need = len(b)
	}
	if need > 5 {
		need = 5
	}
	return need >= 3 && common >= need
}

// govSQLNamesTimeColumn is true when the question uses a word that names this column and not the record's
// other times: "started" for call_start, "ended" for call_end, "activation" for activation_date. The words
// every time shares ("time", "date") and the evidence's own name do not count.
func govSQLNamesTimeColumn(question string, view *govSQLView, column govSQLColumn) bool {
	entity := map[string]bool{}
	for _, phrase := range append([]string{view.Display}, view.Synonyms...) {
		for _, w := range govSQLPhraseWords(phrase) {
			entity[w] = true
		}
	}
	asked := govSQLPhraseWords(question)
	for _, phrase := range append([]string{column.Display}, column.Synonyms...) {
		for _, w := range govSQLPhraseWords(phrase) {
			if len(w) < 3 || govSQLTimeNameSkip[w] || entity[w] {
				continue
			}
			for _, q := range asked {
				if govSQLWordsAlike(w, q) {
					return true
				}
			}
		}
	}
	return false
}

// govSQLNamesOtherTime is true when the question names a time of the record other than its own.
func govSQLNamesOtherTime(question string, view *govSQLView, primary *govSQLColumn) bool {
	for _, c := range view.Columns {
		if c.Name != primary.Name && (c.Type == "timestamptz" || c.Type == "date") && govSQLNamesTimeColumn(question, view, c) {
			return true
		}
	}
	return false
}

// govSQLTimeTarget says where a time condition goes: the record's own time when the question names no other
// time of the record, otherwise any time column of the view.
func govSQLTimeTarget(question string, view *govSQLView) string {
	if primary := govSQLPrimaryTime(view); primary != nil && !govSQLNamesOtherTime(question, view, primary) {
		return primary.Name + ", the time of the record"
	}
	return "a time column (" + govSQLTimeColumns(view) + ")"
}

// govSQLTimeHint is what the question's own message says about a time condition. It carries the case clock
// too, so the shared rules (the same for every question and cached with the schema) never have to.
func govSQLTimeHint(facts govSQLQuestionFacts, view *govSQLView) string {
	return fmt.Sprintf("The question restricts time (%s). Put it in the WHERE clause on %s. Times are on the case clock: the hour, day and month of a time column already mean the case's local time, so never convert time zones.",
		strings.Join(facts.TimeCues, ", "), govSQLTimeTarget(facts.Question, view))
}

// govSQLPredicateTimeColumns lists the time columns that a query's conditions (WHERE, HAVING) refer to.
// MIN(call_start) in a SELECT list asks for a time; it does not constrain one.
func govSQLPredicateTimeColumns(tree *pg.ParseResult, view *govSQLView) []string {
	timeColumns := map[string]bool{}
	for _, c := range view.Columns {
		if c.Type == "timestamptz" || c.Type == "date" {
			timeColumns[c.Name] = true
		}
	}
	found := map[string]bool{}
	scan := func(node *pg.Node) {
		if node == nil {
			return
		}
		govSQLWalkTree(node.ProtoReflect(), func(msg protoreflect.Message) bool {
			if ref, ok := msg.Interface().(*pg.ColumnRef); ok && len(ref.Fields) > 0 {
				if s := ref.Fields[len(ref.Fields)-1].GetString_(); s != nil && timeColumns[s.Sval] {
					found[s.Sval] = true
				}
			}
			return true
		})
	}
	for _, stmt := range tree.Stmts {
		govSQLWalkTree(stmt.ProtoReflect(), func(msg protoreflect.Message) bool {
			switch n := msg.Interface().(type) {
			case *pg.SelectStmt:
				scan(n.WhereClause)
				scan(n.HavingClause)
			case *pg.FuncCall:
				if n.AggFilter != nil {
					scan(n.AggFilter)
				}
			case *pg.CaseWhen:
				scan(n.Expr)
			}
			return true
		})
	}
	names := make([]string, 0, len(found))
	for name := range found {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// govSQLCheckTimeColumn holds a time condition to the record's own time unless the question names another.
// A query that conditions on a time the question never named answers a different question than the one
// asked (here, calls that started OR ended at night, for calls at night).
func govSQLCheckTimeColumn(facts govSQLQuestionFacts, validated *govSQLValidated) []govSQLObligation {
	if len(facts.TimeCues) == 0 {
		return nil
	}
	view := validated.View
	primary := govSQLPrimaryTime(view)
	if primary == nil {
		return nil
	}
	var off []string
	for _, name := range govSQLPredicateTimeColumns(validated.Tree, view) {
		if name == primary.Name {
			continue
		}
		for _, c := range view.Columns {
			if c.Name == name && !govSQLNamesTimeColumn(facts.Question, view, c) {
				off = append(off, name)
			}
		}
	}
	if len(off) == 0 {
		return nil
	}
	return []govSQLObligation{{
		Kind: "TIME_COLUMN", Subject: strings.Join(off, ", "),
		Message: fmt.Sprintf("The question names no time other than the record's own, so the time condition belongs on %s. Remove %s from the conditions and put the condition on %s.", primary.Name, strings.Join(off, ", "), primary.Name),
		Reason:  fmt.Sprintf("the query applied the time condition to %s, which the question does not name; the time of a record is %s", strings.Join(off, ", "), primary.Name),
	}}
}
