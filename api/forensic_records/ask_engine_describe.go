package main

import (
	"fmt"
	"strings"

	pg "github.com/pganalyze/pg_query_go/v6"
	pgq "github.com/wasilibs/go-pgquery"
)

// ASK ENGINE V2: "HOW THIS WAS DERIVED", IN PLAIN WORDS (roadmap 6.8.7).
//
// The restatement of what a query does is generated from the query's own parse tree, never written by the model, so it cannot claim a filter the query does not
// have or leave one out: it is the "back-translation" of roadmap 6.8.6, by code. Each fragment (a condition, a computed value, a grouping, an ordering, a limit)
// is turned back into SQL text by PostgreSQL's own deparser and put in a sentence; the analyst sees the sentences and an administrator the query itself.

type askDerivation struct {
	Reads      []string // the views read, with their one-sentence descriptions
	Conditions []string // the conditions of the query, as written
	Computes   []string // what it returns
	GroupedBy  string
	OrderedBy  string
	Limit      int
	Recursive  bool
	Hops       int
	Combines   string // INTERSECT, UNION or EXCEPT when the query combines two result sets
	CaseClock  bool   // a time is compared or extracted: it is read on the case clock
}

// Sentences is the derivation as the sentences the analyst reads.
func (d askDerivation) Sentences() []string {
	var out []string
	if len(d.Reads) > 0 {
		out = append(out, "Read: "+strings.Join(d.Reads, "; ")+".")
	}
	if len(d.Conditions) > 0 {
		out = append(out, "Kept only the rows where "+strings.Join(d.Conditions, ", and ")+".")
	}
	if d.Recursive {
		out = append(out, fmt.Sprintf("Followed contacts from contact to contact, up to %d hops, never revisiting a number.", d.Hops))
	}
	if d.Combines != "" {
		out = append(out, "Combined two result sets with "+d.Combines+".")
	}
	if len(d.Computes) > 0 {
		out = append(out, "Returned: "+strings.Join(d.Computes, ", ")+".")
	}
	if d.GroupedBy != "" {
		out = append(out, "Grouped by "+d.GroupedBy+".")
	}
	if d.OrderedBy != "" {
		out = append(out, "Ordered by "+d.OrderedBy+".")
	}
	if d.Limit > 0 {
		out = append(out, fmt.Sprintf("Kept the first %d rows.", d.Limit))
	}
	if d.CaseClock {
		zone, _ := govSQLTimeZone()
		out = append(out, "Times are read and shown on the case clock ("+zone+").")
	}
	return out
}

// askDeparseFragment turns a piece of a parse tree back into SQL text. The piece is put into a one-line template SELECT and the deparser's output is cut
// back to the piece.
func askDeparseFragment(kind string, nodes []*pg.Node) string {
	if len(nodes) == 0 {
		return ""
	}
	tree, err := pgq.Parse("SELECT 1")
	if err != nil || len(tree.Stmts) != 1 {
		return ""
	}
	sel := tree.Stmts[0].Stmt.GetSelectStmt()
	if sel == nil {
		return ""
	}
	prefix := ""
	switch kind {
	case "where":
		sel.WhereClause = nodes[0]
		prefix = "SELECT 1 WHERE "
	case "group":
		sel.GroupClause = nodes
		prefix = "SELECT 1 GROUP BY "
	case "order":
		sel.SortClause = nodes
		prefix = "SELECT 1 ORDER BY "
	case "target":
		sel.TargetList = nodes
		prefix = "SELECT "
	default:
		return ""
	}
	text, err := pgq.Deparse(tree)
	if err != nil || !strings.HasPrefix(text, prefix) {
		return ""
	}
	return strings.Join(strings.Fields(strings.TrimPrefix(text, prefix)), " ")
}

func askAppendUnique(list []string, item string, max int) []string {
	if item == "" || len(list) >= max {
		return list
	}
	for _, have := range list {
		if have == item {
			return list
		}
	}
	return append(list, item)
}

// askDescribe builds the derivation of a validated query.
func askDescribe(v *askValidated, cat *askCatalogue) askDerivation {
	d := askDerivation{Recursive: v.Recursive, Hops: v.MaxHops, Limit: v.Limit}
	for _, name := range v.Views {
		if view := cat.View(name); view != nil {
			d.Reads = append(d.Reads, name+" ("+govSQLFirstSentence(view.Description, 110)+")")
		} else {
			d.Reads = append(d.Reads, name)
		}
	}
	if v.Tree == nil || len(v.Tree.Stmts) == 0 {
		return d
	}
	root := v.Tree.Stmts[0].Stmt.GetSelectStmt()
	if root == nil {
		return d
	}
	selects := []*pg.SelectStmt{root}
	if root.WithClause != nil {
		for _, c := range root.WithClause.Ctes {
			if cte := c.GetCommonTableExpr(); cte != nil && cte.Ctequery.GetSelectStmt() != nil {
				selects = append(selects, cte.Ctequery.GetSelectStmt())
			}
		}
	}
	main := root
	switch root.Op {
	case pg.SetOperation_SETOP_INTERSECT:
		d.Combines = "INTERSECT (what both have in common)"
	case pg.SetOperation_SETOP_UNION:
		d.Combines = "UNION"
	case pg.SetOperation_SETOP_EXCEPT:
		d.Combines = "EXCEPT (what the first has and the second lacks)"
	}
	if root.Op != pg.SetOperation_SETOP_NONE && root.Larg != nil {
		main = root.Larg
		selects = append(selects, root.Larg, root.Rarg)
	}
	for _, sel := range selects {
		if sel == nil {
			continue
		}
		for _, c := range askConjuncts(sel.WhereClause) {
			d.Conditions = askAppendUnique(d.Conditions, askDeparseFragment("where", []*pg.Node{c}), 8)
		}
	}
	for _, t := range main.TargetList {
		d.Computes = askAppendUnique(d.Computes, askDeparseFragment("target", []*pg.Node{t}), 8)
	}
	d.GroupedBy = askDeparseFragment("group", root.GroupClause)
	if d.GroupedBy == "" {
		d.GroupedBy = askDeparseFragment("group", main.GroupClause)
	}
	d.OrderedBy = askDeparseFragment("order", root.SortClause)
	sql := strings.ToLower(v.SQL)
	d.CaseClock = strings.Contains(sql, "extract(") || strings.Contains(sql, "date_part(") || strings.Contains(sql, "date_trunc(") || strings.Contains(sql, "time zone") || strings.Contains(sql, "::date")
	for _, c := range v.Consts {
		if !c.Numeric && len(c.Text) >= 10 && c.Text[4] == '-' && c.Text[7] == '-' {
			d.CaseClock = true
		}
	}
	return d
}
