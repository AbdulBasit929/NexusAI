package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	pg "github.com/pganalyze/pg_query_go/v6"
)

// ASK ENGINE V2: THE RECORDS BEHIND THE LINES OF A SUMMARY (part A2; the rules are in ask_engine_trace.go).
//
// A query over a summary view (or over an inner join of two, as the common contacts of two numbers) answers in lines: a contact, an edge of a path, an account and its counterparty, a pair of numbers
// and a cell site. This file finds the lines the answer stands on (the answer's own rows when it lists them, the summary rows its conditions select when it counts or groups them), and for each of
// the first ten the records behind it: how many, in how many files, and whether the line's own numbers (its events, its first and last time, its total) are what those records give back.
// The first line's records are listed at once; every other line is opened on request (GET /query/evidence-line) from a session the answer leaves in memory, bound to the scope that asked.
//
// The line is selected inside the database (a CTE of the answer's own FROM, WHERE and ORDER BY) and the records are joined to it there, so that no value of a line is ever written back into SQL as a
// literal. Everything runs as the read-only analyst role, through the same walls as the answer.

const (
	askEvidenceLineCap  = 10
	askLineSessionTTL   = 30 * time.Minute
	askLineSessionMax   = 300
	askLineSessionRows  = 25
	askLineQueryTimeout = 20 * time.Second
	askLinesEagerWindow = 9 * time.Second // lines are counted at once only while this lasts, counted from the start of the collection
)

type askEvidenceLine struct {
	Index      int    `json:"index"` // from 1
	Label      string `json:"label"`
	Records    int    `json:"records"`
	Files      int    `json:"files"`
	Reconciled string `json:"reconciled,omitempty"` // "yes", "no" or "not_checked"
	Check      string `json:"check,omitempty"`
	Pending    bool   `json:"pending,omitempty"` // not counted yet: the time ran out before this line; it is counted when it is opened
}

// ---- the plan -------------------------------------------------------------------------------------------------------------------------------------

// askFromAliases reads the FROM clause as summary views under their names: one view, or inner joins of views. Anything else (a subquery, an outer join, a function) is not a plan.
func askFromAliases(n *pg.Node) ([]askEvidenceAlias, bool) {
	if rv := n.GetRangeVar(); rv != nil {
		trace, ok := askTraces[rv.Relname]
		if !ok {
			return nil, false
		}
		name := rv.Relname
		if rv.Alias != nil && rv.Alias.Aliasname != "" {
			name = rv.Alias.Aliasname
		}
		if !askPlainIdent.MatchString(name) || name == "l" || name == "r" || name == "w" {
			return nil, false
		}
		return []askEvidenceAlias{{Name: name, View: rv.Relname, Trace: trace}}, true
	}
	if je := n.GetJoinExpr(); je != nil && je.Jointype == pg.JoinType_JOIN_INNER && je.Larg != nil && je.Rarg != nil {
		left, okLeft := askFromAliases(je.Larg)
		right, okRight := askFromAliases(je.Rarg)
		if !okLeft || !okRight {
			return nil, false
		}
		return append(left, right...), true
	}
	return nil, false
}

// askEvidenceLinesPlan plans a query over summary views. root has passed the checks of askEvidencePlanOf (no set operation, no CTE, no DISTINCT, no OFFSET, no window).
func askEvidenceLinesPlan(root *pg.SelectStmt, v *askValidated, columns []string) (*askEvidencePlan, string) {
	aliases, ok := askFromAliases(root.FromClause[0])
	if !ok {
		if rv := root.FromClause[0].GetRangeVar(); rv != nil {
			return nil, "the answer reads " + rv.Relname + ", which has no records behind it that can be listed"
		}
		return nil, "the answer joins or nests several sources of a kind that cannot be traced to records"
	}
	plan := &askEvidencePlan{Kind: "lines", Aliases: aliases, Limit: v.Limit}
	names := make([]string, len(aliases))
	for i, a := range aliases {
		names[i] = a.View
	}
	plan.View = strings.Join(names, ", ")
	if plan.FromSQL = askDeparseFragment("from", root.FromClause); plan.FromSQL == "" {
		return nil, "the sources of the query could not be read back"
	}
	if root.WhereClause != nil {
		if plan.Where = askDeparseFragment("where", []*pg.Node{root.WhereClause}); plan.Where == "" {
			return nil, "the conditions of the query could not be read back"
		}
	}
	aggregates := 0
	var only *pg.FuncCall
	for _, t := range root.TargetList {
		rt := t.GetResTarget()
		if rt == nil || rt.Val == nil {
			return nil, "the query could not be read back"
		}
		call := rt.Val.GetFuncCall()
		if call == nil || call.Over != nil {
			continue
		}
		switch strings.ToLower(call.Funcname[len(call.Funcname)-1].GetString_().GetSval()) {
		case "count", "sum", "min", "max", "avg":
			aggregates++
			only = call
		}
	}
	switch {
	case aggregates == 0 && len(root.GroupClause) == 0:
		plan.RowsAreLines = true
		if len(root.SortClause) > 0 {
			plan.Order = askDeparseFragment("order", root.SortClause)
		}
	case aggregates == 1 && len(root.GroupClause) == 0 && len(root.TargetList) == 1 && only != nil && only.AggStar && !only.AggDistinct && only.AggFilter == nil &&
		strings.EqualFold(only.Funcname[len(only.Funcname)-1].GetString_().GetSval(), "count"):
		plan.CountStar = true
	}
	if len(columns) != len(root.TargetList) {
		return nil, "the columns of the answer do not match the query"
	}
	return plan, ""
}

// ---- the statements -----------------------------------------------------------------------------------------------------------------------------

func (p *askEvidencePlan) lineColumns(a askEvidenceAlias) []string {
	seen := map[string]bool{}
	var cols []string
	for _, c := range append(append([]string{}, a.Trace.Grain...), a.Trace.Extra...) {
		if !seen[c] {
			seen[c] = true
			cols = append(cols, c)
		}
	}
	return cols
}

func (p *askEvidencePlan) lineSelect(withComputed bool) string {
	var parts []string
	for _, a := range p.Aliases {
		for _, c := range p.lineColumns(a) {
			parts = append(parts, fmt.Sprintf("%s.%s AS %s__%s", a.Name, c, a.Name, c))
		}
		if withComputed {
			for _, c := range a.Trace.Computed {
				parts = append(parts, fmt.Sprintf("%s AS %s__%s", strings.ReplaceAll(c.Expr, "{a}", a.Name), a.Name, c.Name))
			}
		}
	}
	return strings.Join(parts, ", ")
}

func (p *askEvidencePlan) lineOrder(fallback bool) string {
	var grain []string
	for _, a := range p.Aliases {
		for _, c := range a.Trace.Grain {
			grain = append(grain, a.Name+"."+c)
		}
	}
	if p.RowsAreLines && p.Order != "" && !fallback {
		return p.Order + ", " + strings.Join(grain, ", ")
	}
	return strings.Join(grain, ", ")
}

func (p *askEvidencePlan) linesSQL(fallback bool, limit int) string {
	return fmt.Sprintf("SELECT %s FROM %s%s ORDER BY %s LIMIT %d", p.lineSelect(false), p.FromSQL, p.where(""), p.lineOrder(fallback), limit)
}

func (p *askEvidencePlan) lineCountSQL() string {
	return fmt.Sprintf("SELECT count(*) AS n FROM %s%s", p.FromSQL, p.where(""))
}

func (p *askEvidencePlan) lineCTE(fallback bool, n int) string {
	return fmt.Sprintf("WITH l AS (SELECT %s FROM %s%s ORDER BY %s LIMIT 1 OFFSET %d)", p.lineSelect(true), p.FromSQL, p.where(""), p.lineOrder(fallback), n)
}

// askLineSteps picks, for each alias, the records behind this line.
func askLineSteps(p *askEvidencePlan, values []map[string]string) ([]askTraceStep, string) {
	steps := make([]askTraceStep, len(p.Aliases))
	for i, a := range p.Aliases {
		step, ok := a.Trace.Pick(values[i])
		if !ok {
			return nil, "no records are known to stand behind this kind of line"
		}
		steps[i] = step
	}
	for _, s := range steps[1:] {
		if s.Raw != steps[0].Raw {
			return nil, "the line joins two kinds of records, which are not itemised together"
		}
	}
	return steps, ""
}

func askLineCond(p *askEvidencePlan, steps []askTraceStep) string {
	parts := make([]string, len(steps))
	for i, s := range steps {
		parts[i] = "(" + askTraceCond(s.Cond, p.Aliases[i].Name) + ")"
	}
	return strings.Join(parts, " OR ")
}

func askLineViews(p *askEvidencePlan, steps []askTraceStep) []string {
	views := []string{}
	seen := map[string]bool{}
	add := func(v string) {
		if v != "" && !seen[v] {
			seen[v] = true
			views = append(views, v)
		}
	}
	for _, a := range p.Aliases {
		add(a.View)
	}
	for _, s := range steps {
		for _, v := range askTraceUses(s) {
			add(v)
		}
	}
	return views
}

// ---- a session of lines -------------------------------------------------------------------------------------------------------------------------

type askLineSession struct {
	Tenant, Collection string
	Plan               *askEvidencePlan
	Cat                *askCatalogue
	Limits             askLimits
	Fallback           bool
	Lines              [][]map[string]string // the values of each line, by alias, as read from the answer
	Labels             []string
	Created            time.Time
}

var askLineSessions = struct {
	sync.Mutex
	items map[string]*askLineSession
	order []string
}{items: map[string]*askLineSession{}}

func askLineSessionStore(s *askLineSession) string {
	token := askReadingToken()
	if token == "" {
		return ""
	}
	askLineSessions.Lock()
	defer askLineSessions.Unlock()
	now := time.Now()
	kept := askLineSessions.order[:0]
	for _, t := range askLineSessions.order {
		item := askLineSessions.items[t]
		if item == nil || now.Sub(item.Created) > askLineSessionTTL {
			delete(askLineSessions.items, t)
			continue
		}
		kept = append(kept, t)
	}
	askLineSessions.order = kept
	for len(askLineSessions.order) >= askLineSessionMax {
		delete(askLineSessions.items, askLineSessions.order[0])
		askLineSessions.order = askLineSessions.order[1:]
	}
	s.Created = now
	askLineSessions.items[token] = s
	askLineSessions.order = append(askLineSessions.order, token)
	return token
}

func askLineSessionFind(token string) *askLineSession {
	askLineSessions.Lock()
	defer askLineSessions.Unlock()
	s := askLineSessions.items[token]
	if s == nil || time.Since(s.Created) > askLineSessionTTL {
		return nil
	}
	return s
}

// ---- the collection -----------------------------------------------------------------------------------------------------------------------------

// askCollectLines finds the lines of an answer over summary views and the records behind the first ten.
func askCollectLines(ctx context.Context, db *pgxpool.Pool, tenantID, collectionID string, cat *askCatalogue, plan *askEvidencePlan, result *govSQLResult, limits askLimits, ev *askEvidence) *askEvidence {
	started := time.Now()
	session := &askLineSession{Tenant: tenantID, Collection: collectionID, Plan: plan, Cat: cat, Limits: limits}
	runner := func(sql string, maxRows int, views []string) (*govSQLResult, error) {
		checked, rej := askValidate(sql, cat, views)
		if rej != nil {
			return nil, fmt.Errorf("%s", rej.Message)
		}
		return askRun(ctx, db, tenantID, collectionID, checked.SQL, nil, limits, maxRows, true)
	}
	views := []string{}
	for _, a := range plan.Aliases {
		views = append(views, a.View)
	}
	capLines := askEvidenceLineCap
	if plan.RowsAreLines && plan.Limit > 0 && plan.Limit < capLines {
		capLines = plan.Limit
	}
	lines, err := runner(plan.linesSQL(false, capLines), askEvidenceLineCap+1, views)
	if err != nil && plan.RowsAreLines && plan.Order != "" {
		session.Fallback = true
		lines, err = runner(plan.linesSQL(true, capLines), askEvidenceLineCap+1, views)
	}
	if err != nil {
		return askEvidenceNone("the lines of the answer could not be read back (" + govSQLClip(err.Error(), 120) + ")")
	}
	if len(lines.Rows) == 0 {
		return askEvidenceNone("the answer has no line")
	}
	total := len(lines.Rows)
	if counted, err := runner(plan.lineCountSQL(), 1, views); err == nil && len(counted.Rows) == 1 {
		if n, ok := askEvidenceNumber(counted.Rows[0][0]); ok {
			total = int(n)
		}
	}
	if plan.RowsAreLines && !result.Truncated {
		total = len(result.Rows) // the lines of an answer that lists rows are its own rows, not every row its conditions could select
	}
	// the values of each line, by alias
	index := map[string]int{}
	for i, c := range lines.Columns {
		index[c] = i
	}
	for _, row := range lines.Rows {
		perAlias := make([]map[string]string, len(plan.Aliases))
		var labels []string
		for i, a := range plan.Aliases {
			values := map[string]string{}
			for _, c := range plan.lineColumns(a) {
				if at, ok := index[a.Name+"__"+c]; ok && row[at] != nil {
					values[c] = fmt.Sprint(row[at])
				}
			}
			perAlias[i] = values
			labels = append(labels, a.Trace.Label(values))
		}
		session.Lines = append(session.Lines, perAlias)
		session.Labels = append(session.Labels, strings.Join(labels, "  +  "))
	}
	ev.Kind, ev.View, ev.LinesTotal = "lines", plan.View, total
	// the figure of the answer, when it is the number of lines
	figure := ""
	if plan.CountStar && len(result.Rows) == 1 {
		if want, ok := askEvidenceNumber(result.Rows[0][0]); ok {
			if int(want) == total {
				ev.Reconciled, figure = "yes", fmt.Sprintf("The answer counts %s lines, and the lines its conditions select are %s.", govSQLGroup(strconv.Itoa(total)), govSQLGroup(strconv.Itoa(total)))
			} else {
				ev.Reconciled, figure = "no", fmt.Sprintf("The answer counts %s lines, but the lines its conditions select are %s.", govSQLGroup(strconv.Itoa(int(want))), govSQLGroup(strconv.Itoa(total)))
			}
		}
	}
	if plan.RowsAreLines && !session.Fallback && !result.Truncated {
		if len(lines.Rows) == len(result.Rows) {
			figure = fmt.Sprintf("The %d lines listed are the %d lines of the answer.", len(lines.Rows), len(result.Rows))
		} else {
			figure = fmt.Sprintf("The first %d of the answer's %d lines are listed.", len(lines.Rows), len(result.Rows))
		}
		ev.Reconciled = "yes"
	}
	if ev.Reconciled == "" {
		ev.Reconciled = "not_checked"
	}
	// each line: how many records, in how many files, and do they give back the line's own numbers
	agree, disagree, checkable := 0, 0, 0
	for i := range session.Lines {
		line := askEvidenceLine{Index: i + 1, Label: session.Labels[i]}
		if time.Since(started) > askLinesEagerWindow {
			line.Pending, line.Reconciled, line.Check = true, "not_checked", "Counted when you open it."
			ev.Lines = append(ev.Lines, line)
			continue
		}
		counts, checks, why := askLineCounts(ctx, plan, session, i, runner)
		if why != "" {
			line.Reconciled, line.Check = "not_checked", why
		} else {
			line.Records, line.Files = counts.records, counts.files
			line.Reconciled, line.Check = checks.state, checks.sentence
			switch checks.state {
			case "yes":
				agree++
				checkable++
			case "no":
				disagree++
				checkable++
			}
		}
		ev.Lines = append(ev.Lines, line)
	}
	if plan.CountStar {
		// the figure is the number of lines: it is checked above
	} else if checkable > 0 {
		sentence := fmt.Sprintf("Each line's own numbers were recomputed from its records: %d of %d agree.", agree, checkable)
		switch {
		case disagree == 0:
			ev.Reconciled = "yes"
		default:
			ev.Reconciled = "no"
			sentence += " The lines that do not agree are marked."
		}
		figure = strings.TrimSpace(figure + " " + sentence)
	}
	ev.Check = figure
	token := askLineSessionStore(session)
	ev.LinesToken = token
	// the records of the first line, at once when there is time; otherwise the workspace opens the first line itself
	if time.Since(started) > askLinesEagerWindow+3*time.Second {
		ev.Scope = "open the first line to list its records"
		return ev
	}
	first, err := askLineEvidence(ctx, db, session, 1, runner)
	if err != nil {
		ev.Scope = "no records could be listed for the first line: " + govSQLClip(err.Error(), 120)
		return ev
	}
	ev.Say, ev.Columns, ev.Rows, ev.Files, ev.Total, ev.Scope = first.Say, first.Columns, first.Rows, first.Files, first.Total, "the records behind the first line"
	return ev
}

type askLineCountResult struct{ records, files int }
type askLineChecks struct{ state, sentence string }

// askLineCountStatement counts the records behind line i (from 0) and reads the line's own numbers beside the numbers its records give (the checks, for a query over one summary view).
func askLineCountStatement(plan *askEvidencePlan, fallback bool, i int, steps []askTraceStep, raw askEvidenceSource) (string, []askTraceCheck) {
	cond := askLineCond(plan, steps)
	sel := []string{"count(r." + raw.Key + ") AS n", "count(DISTINCT r.evidence_id) AS f"}
	var group []string
	var checks []askTraceCheck
	if len(plan.Aliases) == 1 {
		checks = steps[0].Checks
		for k, c := range checks {
			sel = append(sel, askTraceExpr(c.Expr, raw.Key)+fmt.Sprintf(" AS g%d", k), fmt.Sprintf("l.%s__%s AS w%d", plan.Aliases[0].Name, c.Column, k))
			group = append(group, fmt.Sprintf("l.%s__%s", plan.Aliases[0].Name, c.Column))
		}
	}
	sql := plan.lineCTE(fallback, i) + " SELECT " + strings.Join(sel, ", ") + " FROM l LEFT JOIN " + steps[0].Raw + " r ON (" + cond + ")"
	if len(group) > 0 {
		sql += " GROUP BY " + strings.Join(group, ", ")
	}
	return sql, checks
}

// askLineCounts counts the records behind line i (from 0) and checks the line's own numbers against them.
func askLineCounts(ctx context.Context, plan *askEvidencePlan, s *askLineSession, i int, runner func(string, int, []string) (*govSQLResult, error)) (askLineCountResult, askLineChecks, string) {
	steps, why := askLineSteps(plan, s.Lines[i])
	if why != "" {
		return askLineCountResult{}, askLineChecks{}, why
	}
	if steps[0].Raw == "" {
		return askLineCountResult{}, askLineChecks{state: "not_checked"}, ""
	}
	raw, ok := askEvidenceViews[steps[0].Raw]
	if !ok {
		return askLineCountResult{}, askLineChecks{}, "the records of this line are of a kind that cannot be listed"
	}
	sql, checks := askLineCountStatement(plan, s.Fallback, i, steps, raw)
	got, err := runner(sql, 2, askLineViews(plan, steps))
	if err != nil || len(got.Rows) == 0 {
		msg := "the records of this line could not be counted"
		if err != nil {
			msg += " (" + govSQLClip(err.Error(), 100) + ")"
		}
		return askLineCountResult{}, askLineChecks{}, msg
	}
	col := map[string]int{}
	for k, c := range got.Columns {
		col[c] = k
	}
	n, _ := askEvidenceNumber(got.Rows[0][col["n"]])
	f, _ := askEvidenceNumber(got.Rows[0][col["f"]])
	result := askLineCountResult{records: int(n), files: int(f)}
	if len(checks) == 0 {
		return result, askLineChecks{state: "not_checked"}, ""
	}
	var said []string
	bad := 0
	for k, c := range checks {
		want, have := got.Rows[0][col[fmt.Sprintf("w%d", k)]], got.Rows[0][col[fmt.Sprintf("g%d", k)]]
		if askLineSame(want, have) {
			said = append(said, fmt.Sprintf("%s %s", askCheckSay(c.Column), askLineShow(have)))
		} else {
			bad++
			said = append(said, fmt.Sprintf("%s is %s on the line but its records give %s", askCheckSay(c.Column), askLineShow(want), askLineShow(have)))
		}
	}
	if bad > 0 {
		return result, askLineChecks{state: "no", sentence: "The records do not give the line's numbers: " + strings.Join(said, "; ") + "."}, ""
	}
	return result, askLineChecks{state: "yes", sentence: "Recomputed from its records: " + strings.Join(said, "; ") + "."}, ""
}

// askCheckSay names a number of a line as an analyst says it.
func askCheckSay(column string) string {
	if say, ok := askCheckWords[column]; ok {
		return say
	}
	return strings.ReplaceAll(column, "_", " ")
}

var askCheckWords = map[string]string{
	"events": "events:", "outgoing": "outgoing:", "incoming": "incoming:", "unknown": "unknown direction:", "total_duration_s": "total duration (seconds):",
	"first_seen": "first contact:", "last_seen": "last contact:", "first_event_utc": "first event:", "last_event_utc": "last event:", "events_at_anchor": "events at the anchor:",
	"transactions": "transactions:", "total_amount": "total amount:", "first_utc": "first:", "last_utc": "last:", "credits": "credits:", "debits": "debits:",
	"sightings": "sightings:", "registrations": "registrations:", "sources": "source files:", "mentions": "mentions:", "records": "records:",
}

func askLineSame(a, b any) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	x, okX := askEvidenceNumber(a)
	y, okY := askEvidenceNumber(b)
	if okX && okY {
		diff := x - y
		if diff < 0 {
			diff = -diff
		}
		scale := x
		if scale < 0 {
			scale = -scale
		}
		return diff <= 1e-6*(1+scale)
	}
	return fmt.Sprint(a) == fmt.Sprint(b)
}

func askLineShow(v any) string {
	if v == nil {
		return "nothing"
	}
	if f, ok := v.(float64); ok {
		return askEvidenceFormat(f)
	}
	if n, ok := askEvidenceNumber(v); ok {
		return askEvidenceFormat(n)
	}
	return fmt.Sprint(v)
}

// askLineRecordStatements are the two statements that list the records behind line i (from 0): the first records, and the records of each file.
func askLineRecordStatements(plan *askEvidencePlan, fallback bool, i int, steps []askTraceStep, raw askEvidenceSource) (rowsSQL, filesSQL string) {
	cond := askLineCond(plan, steps)
	cte := plan.lineCTE(fallback, i)
	order := steps[0].Order
	if order == "" {
		order = "r." + raw.Key
	} else {
		order += ", r." + raw.Key
	}
	list := "r." + raw.Key + "::text AS k"
	for _, c := range raw.Preview {
		list += ", r." + c
	}
	rowsSQL = fmt.Sprintf("%s SELECT %s FROM l JOIN %s r ON (%s) ORDER BY %s LIMIT %d", cte, list, steps[0].Raw, cond, order, askLineSessionRows)
	filesSQL = fmt.Sprintf("%s SELECT r.evidence_id::text AS f, count(*) AS n FROM l JOIN %s r ON (%s) GROUP BY 1 ORDER BY 2 DESC, 1 LIMIT %d", cte, steps[0].Raw, cond, askEvidenceFileCap+1)
	return rowsSQL, filesSQL
}

// askLineEvidence lists the records behind line i (from 1): the first records, and how they divide between the files.
func askLineEvidence(ctx context.Context, db *pgxpool.Pool, s *askLineSession, line int, runner func(string, int, []string) (*govSQLResult, error)) (*askEvidence, error) {
	if line < 1 || line > len(s.Lines) {
		return nil, errors.New("this line is not one of the answer's lines")
	}
	i := line - 1
	plan := s.Plan
	steps, why := askLineSteps(plan, s.Lines[i])
	if why != "" {
		return nil, errors.New(why)
	}
	out := &askEvidence{State: "itemised", Kind: "records", View: plan.View, Say: steps[0].Say}
	if steps[0].Raw == "" {
		return out, nil
	}
	raw, ok := askEvidenceViews[steps[0].Raw]
	if !ok {
		return nil, errors.New("the records of this line are of a kind that cannot be listed")
	}
	views := askLineViews(plan, steps)
	rowsSQL, filesSQL := askLineRecordStatements(plan, s.Fallback, i, steps, raw)
	rows, err := runner(rowsSQL, askLineSessionRows+1, views)
	if err != nil {
		return nil, err
	}
	var keys []string
	previews := map[string]string{}
	cellsOf := map[string][]string{}
	if len(raw.Labels) == len(raw.Preview) {
		out.Columns = raw.Labels
	}
	for _, r := range rows.Rows {
		key := fmt.Sprint(r[0])
		keys = append(keys, key)
		cells := make([]string, 0, len(r)-1)
		var parts []string
		for k := 1; k < len(r); k++ {
			text := ""
			if r[k] != nil {
				text = fmt.Sprint(r[k])
			}
			cells = append(cells, text)
			if text != "" && k-1 < len(raw.Labels) {
				parts = append(parts, raw.Labels[k-1]+" "+text)
			}
		}
		cellsOf[key] = cells
		previews[key] = govSQLClip(strings.Join(parts, "; "), 240)
	}
	out.cells = cellsOf
	files, err := runner(filesSQL, askEvidenceFileCap+1, views)
	if err != nil {
		return nil, err
	}
	for _, r := range files.Rows {
		n, _ := askEvidenceNumber(r[1])
		out.Files = append(out.Files, askEvidenceFile{EvidenceID: fmt.Sprint(r[0]), Rows: int(n)})
		out.Total += int(n)
	}
	if err := askEvidenceLocate(ctx, db, s.Tenant, s.Collection, raw, keys, previews, out); err != nil {
		return nil, err
	}
	return out, nil
}

// ---- the route ------------------------------------------------------------------------------------------------------------------------------------

// askLineHandler: GET /query/evidence-line?token=...&line=3 returns the records behind line 3 of an answer that is on the screen.
func askLineHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
			return
		}
		if db == nil {
			writeError(w, http.StatusServiceUnavailable, errors.New("FORENSIC_DATABASE_URL is not configured"))
			return
		}
		scope, err := bindForensicScope(r, r.URL.Query().Get("tenant_id"), strings.TrimSpace(r.URL.Query().Get("collection_id")), "", "")
		if err != nil {
			writeScopeError(w, err)
			return
		}
		session := askLineSessionFind(strings.TrimSpace(r.URL.Query().Get("token")))
		if session == nil || session.Tenant != scope.TenantID || (scope.CollectionID != "" && session.Collection != scope.CollectionID) {
			writeError(w, http.StatusNotFound, errors.New("these lines are no longer available; ask the question again"))
			return
		}
		line, err := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("line")))
		if err != nil || line < 1 || line > len(session.Lines) {
			writeError(w, http.StatusBadRequest, errors.New("line must be a number of the answer's lines, from 1"))
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), askLineQueryTimeout+askEvidenceBudget)
		defer cancel()
		runner := func(sql string, maxRows int, views []string) (*govSQLResult, error) {
			checked, rej := askValidate(sql, session.Cat, views)
			if rej != nil {
				return nil, fmt.Errorf("%s", rej.Message)
			}
			return askRun(ctx, db, session.Tenant, session.Collection, checked.SQL, nil, session.Limits, maxRows, true)
		}
		ev, err := askLineEvidence(ctx, db, session, line, runner)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, errors.New("the records of this line could not be listed: "+govSQLClip(err.Error(), 160)))
			return
		}
		summary := askEvidenceLine{Index: line, Label: session.Labels[line-1], Reconciled: "not_checked"}
		if counts, checks, why := askLineCounts(ctx, session.Plan, session, line-1, runner); why == "" {
			summary.Records, summary.Files, summary.Reconciled, summary.Check = counts.records, counts.files, checks.state, checks.sentence
		} else {
			summary.Check = why
		}
		writeJSON(w, http.StatusOK, map[string]any{"line": line, "label": session.Labels[line-1], "summary": summary, "evidence": ev})
	}
}
