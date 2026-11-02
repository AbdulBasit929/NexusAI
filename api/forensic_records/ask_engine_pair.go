package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	pg "github.com/pganalyze/pg_query_go/v6"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// ASK ENGINE V2: THE PAIR OF NUMBERS IS ONE PREDICATE (the verified bank run of 2026-11-02, VB-PAIR-007).
//
// "What is the total duration of the voice calls between A and B?" was answered 57,670 where the records hold 12,290. The model wrote
//
//	WHERE src = A OR dst = A OR src = B OR (dst = B AND event_type = 'voice')
//
// which reads every event of A and of B with everyone, and binds the voice filter to its last alternative only. The check on two directions (PAIR_DIRECTION) looks for the pair read both ways
// and never saw this shape, so a confident wrong figure was shown.
//
// A question about the events BETWEEN two numbers is limited, at both ends, to those two numbers: every row it reads has src and dst among them. The check reads that off the parse tree, not
// the text: what a WHERE says about src and about dst is worked out through AND (both limits hold: the narrower wins) and OR (an alternative that limits an end only does so for itself: the end
// is limited only if every alternative limits it). And every condition that is not about the two numbers (the kind of event, the dates) must be in every alternative or stand outside the
// OR: AND binds tighter than OR, so a condition written after the last alternative belongs to that alternative alone.

// askPairCue is a question about the events between two numbers: "between A and B", "with each other", "A had with B". Dates are also written with "between", so the word counts
// only when a number follows it.
var askPairCue = regexp.MustCompile(`\bbetween\b(?:\s+[a-z]+){0,3}?\s+\+?\d{10,15}\b|\b(?:with|to|from) (?:each other|one another)\b|\b\d{10,15}\b[^?.!]{0,80}?\bwith\b[^?.!]{0,12}?\b\d{10,15}\b`)

// askPairNotCue marks a question that names two numbers and is about something else than the events between them (who they have in common, how they are connected).
var askPairNotCue = regexp.MustCompile(`\b(?:common|mutual|shared|connected|connection|hops?|path)\b`)

// askPairViews are the event views whose rows carry the two ends of an event as src and dst.
var askPairViews = map[string]bool{"v_comm_events": true, "v_contact_pairs": true}

// askEventReach is what a predicate says about the two ends of an event: the numbers src can be and the numbers dst can be. nil means the predicate does not limit that end.
type askEventReach struct {
	src, dst map[string]bool
}

// askSetEnd returns the reach with one end replaced.
func askSetEnd(r askEventReach, column string, set map[string]bool) askEventReach {
	if column == "src" {
		r.src = set
	} else {
		r.dst = set
	}
	return r
}

func askIntersect(a, b map[string]bool) map[string]bool {
	switch {
	case a == nil:
		return b
	case b == nil:
		return a
	}
	out := map[string]bool{}
	for k := range a {
		if b[k] {
			out[k] = true
		}
	}
	return out
}

func askUnion(a, b map[string]bool) map[string]bool {
	if a == nil || b == nil {
		return nil // an alternative that does not limit this end leaves it unlimited
	}
	out := map[string]bool{}
	for k := range a {
		out[k] = true
	}
	for k := range b {
		out[k] = true
	}
	return out
}

// askConstString is the text of a string constant (a cast of one included).
func askConstString(n *pg.Node) (string, bool) {
	if n == nil {
		return "", false
	}
	if c := n.GetTypeCast(); c != nil {
		return askConstString(c.Arg)
	}
	c := n.GetAConst()
	if c == nil || c.GetSval() == nil {
		return "", false
	}
	return strings.TrimPrefix(c.GetSval().GetSval(), "+"), true
}

// askConstStrings is the text constants of a list or an ARRAY[...] (one constant is a list of one); false when anything in it is not a constant.
func askConstStrings(n *pg.Node) ([]string, bool) {
	if n == nil {
		return nil, false
	}
	if c := n.GetTypeCast(); c != nil {
		return askConstStrings(c.Arg)
	}
	var items []*pg.Node
	switch {
	case n.GetList() != nil:
		items = n.GetList().Items
	case n.GetAArrayExpr() != nil:
		items = n.GetAArrayExpr().Elements
	default:
		s, ok := askConstString(n)
		return []string{s}, ok
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		s, ok := askConstString(item)
		if !ok {
			return nil, false
		}
		out = append(out, s)
	}
	return out, true
}

// askComparison reads column = 'x', 'x' = column, column IN ('x', 'y') and column = ANY (ARRAY['x', 'y']): the column's name (without its alias) and the constants it is
// compared with. ok is false for every other expression.
func askComparison(n *pg.Node) (column string, values []string, ok bool) {
	e := n.GetAExpr()
	if e == nil || len(e.Name) != 1 || e.Name[0].GetString_().GetSval() != "=" {
		return "", nil, false
	}
	switch e.Kind {
	case pg.A_Expr_Kind_AEXPR_OP:
		if column = askColumnName(e.Lexpr); column != "" {
			values, ok = askConstStrings(e.Rexpr)
		} else if column = askColumnName(e.Rexpr); column != "" {
			values, ok = askConstStrings(e.Lexpr)
		}
	case pg.A_Expr_Kind_AEXPR_IN, pg.A_Expr_Kind_AEXPR_OP_ANY:
		if column = askColumnName(e.Lexpr); column != "" {
			values, ok = askConstStrings(e.Rexpr)
		}
	}
	return column, values, ok && column != ""
}

// askReach is what a predicate says about the two ends of an event. Only comparisons of src and dst with constants limit an end; everything else says nothing about it.
func askReach(n *pg.Node) askEventReach {
	if n == nil {
		return askEventReach{}
	}
	if b := n.GetBoolExpr(); b != nil {
		switch b.Boolop {
		case pg.BoolExprType_AND_EXPR:
			var r askEventReach
			for _, arg := range b.Args {
				a := askReach(arg)
				r.src, r.dst = askIntersect(r.src, a.src), askIntersect(r.dst, a.dst)
			}
			return r
		case pg.BoolExprType_OR_EXPR:
			if len(b.Args) == 0 {
				return askEventReach{}
			}
			r := askReach(b.Args[0])
			for _, arg := range b.Args[1:] {
				a := askReach(arg)
				r.src, r.dst = askUnion(r.src, a.src), askUnion(r.dst, a.dst)
			}
			return r
		}
		return askEventReach{} // NOT says nothing about a number
	}
	column, values, ok := askComparison(n)
	if !ok || (column != "src" && column != "dst") {
		return askEventReach{}
	}
	set := map[string]bool{}
	for _, v := range values {
		set[v] = true
	}
	return askSetEnd(askEventReach{}, column, set)
}

// askWalkTree visits every message of a parse tree, parents before children; fn returns false to leave a message's children out.
func askWalkTree(m protoreflect.Message, fn func(protoreflect.Message) bool) {
	if !fn(m) {
		return
	}
	m.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		switch {
		case fd.IsMap():
		case fd.IsList() && fd.Message() != nil:
			list := v.List()
			for i := 0; i < list.Len(); i++ {
				askWalkTree(list.Get(i).Message(), fn)
			}
		case fd.Message() != nil:
			askWalkTree(v.Message(), fn)
		}
		return true
	})
}

// askTouchesEnds is whether an expression compares src or dst anywhere inside it.
func askTouchesEnds(n *pg.Node) bool {
	found := false
	if n != nil {
		askWalkTree(n.ProtoReflect(), func(m protoreflect.Message) bool {
			if ref, ok := m.Interface().(*pg.ColumnRef); ok && len(ref.Fields) > 0 {
				name := ref.Fields[len(ref.Fields)-1].GetString_().GetSval()
				found = found || name == "src" || name == "dst"
				return false
			}
			return !found
		})
	}
	return found
}

// askLooseConditions finds, in every OR that compares src or dst, the conditions that are not about the numbers and are not in every alternative. Each is returned once as the text of the
// condition. (`(src = A AND dst = B) OR (src = B AND dst = A) AND start_utc >= '2026-03-01'`: AND binds tighter, so the date belongs to the second alternative only.)
func askLooseConditions(where *pg.Node) []string {
	var loose []string
	seen := map[string]bool{}
	var visit func(n *pg.Node)
	visit = func(n *pg.Node) {
		b := n.GetBoolExpr()
		if b == nil {
			return
		}
		if b.Boolop == pg.BoolExprType_OR_EXPR && askTouchesEnds(n) {
			inArms := map[string]int{}
			for _, arg := range b.Args {
				own := map[string]bool{}
				for _, c := range askConjuncts(arg) {
					if askTouchesEnds(c) {
						continue
					}
					text := askDeparseFragment("where", []*pg.Node{c})
					if text == "" {
						text = c.String()
					}
					own[text] = true
				}
				for text := range own {
					inArms[text]++
				}
			}
			for text, count := range inArms {
				if count < len(b.Args) && !seen[text] {
					seen[text] = true
					loose = append(loose, text)
				}
			}
		}
		for _, arg := range b.Args {
			visit(arg)
		}
	}
	visit(where)
	sort.Strings(loose)
	return loose
}

// askPairSelects are the SELECTs of the query that read one event view and nothing else (no join, no grouping): the shape in which a pair predicate can be read off the WHERE.
func askPairSelects(v *askValidated) []*pg.SelectStmt {
	if v == nil || v.Tree == nil {
		return nil
	}
	var out []*pg.SelectStmt
	askWalkTree(v.Tree.ProtoReflect(), func(m protoreflect.Message) bool {
		if sel, ok := m.Interface().(*pg.SelectStmt); ok && sel.WhereClause != nil && len(sel.FromClause) == 1 && len(sel.GroupClause) == 0 {
			if rv := sel.FromClause[0].GetRangeVar(); rv != nil && askPairViews[rv.Relname] && (rv.Schemaname == "" || rv.Schemaname == "analyst") {
				out = append(out, sel)
			}
		}
		return true
	})
	return out
}

// askPairScope holds a query about the events between two numbers to the rule that the pair is one predicate. It returns the repair message and the reason, or "" when the query does
// what the question asks (or is not of the shape this reads: a join, a grouping, a view without src and dst).
func askPairScope(f askFrame, v *askValidated) (message, reason string) {
	q := strings.ToLower(f.Question)
	if len(f.Numbers) != 2 || !askPairCue.MatchString(q) || askPairNotCue.MatchString(q) {
		return "", ""
	}
	a, b := f.Numbers[0], f.Numbers[1]
	pair := map[string]bool{a: true, b: true}
	for _, sel := range askPairSelects(v) {
		reach := askReach(sel.WhereClause)
		scoped := reach.src != nil && reach.dst != nil
		covered := map[string]bool{}
		for _, set := range []map[string]bool{reach.src, reach.dst} {
			for k := range set {
				if !pair[k] {
					scoped = false // an end limited to a number that is not of the pair
				}
				covered[k] = true
			}
		}
		if !scoped || !covered[a] || !covered[b] {
			return fmt.Sprintf("The question asks about the events between %[1]s and %[2]s, but the conditions on the two numbers do not limit both ends of every event to the pair: a chain such as src = '%[1]s' OR dst = '%[1]s' OR src = '%[2]s' reads every event of each number with everyone, and an AND after the last alternative belongs to that alternative only. Write the pair once, as src IN ('%[1]s', '%[2]s') AND dst IN ('%[1]s', '%[2]s'), and put every other condition (the kind of event, the dates) after it with AND, so that it applies to the whole pair.", a, b),
				"the query does not limit the events to those between the two numbers of the question"
		}
		if loose := askLooseConditions(sel.WhereClause); len(loose) > 0 {
			return fmt.Sprintf("The condition %[3]s is written inside only some of the alternatives joined by OR (AND binds tighter than OR), so the other alternatives ignore it. Write the pair once, as src IN ('%[1]s', '%[2]s') AND dst IN ('%[1]s', '%[2]s'), and put every other condition after it with AND, so that it applies to the whole pair.", a, b, strings.Join(loose, " and ")),
				"a condition of the question applies to only part of the events between the two numbers"
		}
	}
	return "", ""
}

// askPairSumOnce is whether a query that adds up the events of a pair read in both directions adds each call once: it sums over a SELECT DISTINCT of the calls, not over the rows of both
// directions and not over DISTINCT values (SUM(DISTINCT duration_s) drops every call whose length another call also has). sql is lowercase.
func askPairSumOnce(sql string) bool {
	return strings.Contains(sql, "select distinct") && !strings.Contains(sql, "sum(distinct") && !strings.Contains(sql, "sum( distinct")
}
