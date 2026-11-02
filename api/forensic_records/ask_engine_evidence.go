package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	pg "github.com/pganalyze/pg_query_go/v6"
)

// ASK ENGINE V2: THE EVIDENCE BEHIND A FIGURE (docs/data/answer-evidence-and-interpretation-20261102.md, part A).
//
// An answer used to be a figure and a restatement of the query. The analyst could not see which rows the figure rests on, in which files, and whether the rows really add up to it. This file finds
// them, by code and never by the model:
//
//  1. THE PLAN comes from the answer query's own parse tree (a query that reads one view of events, with its WHERE, its ORDER BY and its aggregates). Anything else (a join, a recursive search, a
//     summary view such as v_contact_pairs, a set operation) is not guessed at: the answer says that its rows are not itemised, and why.
//  2. THE ROWS are read with the same walls as the answer (the read-only analyst role, the same scope, the planner's price ceiling): the keys of the matching events, the extreme row for a maximum or a
//     minimum, the rows of the top group for a ranking, and the number of matching rows in each source file.
//  3. THE FIGURE IS RECOMPUTED FROM THOSE PIECES (a count is the sum of the counts of the files, a maximum the largest of the maxima, an average the sum over the count). When the pieces do not give
//     the figure, the answer says so; rows that do not prove a figure are never shown as if they did.
//  4. THE PLACE IN THE SOURCE is looked up by code from the stored rows: the file, its version and the row of the sheet as the evidence viewer shows it. A typed row counts its own position in the
//     statement, which is not the sheet's row (a statement has title and header rows above its data), so the sheet's row is found by content and checked, never assumed. A row that cannot be found
//     is listed without a location and is not offered as an openable source.
//
// Nothing here can fail an answer: any error ends in "not itemised" with the reason.

const (
	askEvidenceContract = "forensics.answer-evidence/v1"
	askEvidenceRowCap   = 25
	askEvidenceFileCap  = 100
	askEvidenceBudget   = 12 * time.Second
	askEvidenceEnv      = "FORENSIC_ASK_V2_EVIDENCE"
)

// askEvidenceSource says where the rows of a view live: the base table, its key, and whether the typed row keeps the source row's own cells (so that its place in the sheet can be found by content).
type askEvidenceSource struct {
	Base      string
	Key       string
	HasRow    bool
	Locatable bool
	Preview   []string // the columns of the view that say what the row is, in the order they are shown; never a column that holds a withheld identifier
}

var askEvidenceViews = map[string]askEvidenceSource{
	"v_comm_events":              {Base: "comm_event", Key: "event_id", HasRow: true, Locatable: true, Preview: []string{"start_utc", "event_type", "direction", "src", "dst", "duration_s"}},
	"v_bank_transactions":        {Base: "bank_transaction", Key: "txn_id", HasRow: true, Locatable: true, Preview: []string{"txn_utc", "account", "direction", "amount", "currency", "counterparty_account", "counterparty_msisdn"}},
	"v_data_sessions":            {Base: "data_session", Key: "session_id", HasRow: true, Locatable: true, Preview: []string{"start_utc", "src", "app", "host", "bytes_up", "bytes_down"}},
	"v_app_usage":                {Base: "app_usage", Key: "usage_id", HasRow: true, Locatable: true, Preview: []string{"start_utc", "src", "app", "app_category"}},
	"v_plate_sightings":          {Base: "plate_sighting", Key: "sighting_id", HasRow: true, Locatable: true, Preview: []string{"observed_utc", "plate_norm", "camera_id", "camera_label"}},
	"v_subscriber_registrations": {Base: "subscriber_registration", Key: "registration_id", HasRow: true, Preview: []string{"msisdn", "imsi", "status", "valid_from", "valid_to"}},
	"v_document_mentions":        {Base: "document_mention", Key: "mention_id", Preview: []string{"document", "kind", "char_start"}},
}

type askEvidenceAgg struct {
	Fn       string // count, sum, min, max, avg
	Text     string // the call as SQL, without an alias
	Index    int    // the position of its column in the answer
	Column   string // the plain column it reads (for the extreme row); empty for count(*) and expressions
	Distinct bool
	Plain    bool // false when a FILTER, an ORDER or a DISTINCT keeps the pieces from adding up
}

type askEvidencePlan struct {
	View       string
	Source     askEvidenceSource
	Kind       string // "aggregate", "rows" or "group"
	Alias      string
	Where      string
	Order      string
	Limit      int
	Aggs       []askEvidenceAgg
	GroupCols  []string // the plain columns of GROUP BY
	GroupIndex []int    // the position of each in the answer's columns
}

var askPlainIdent = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

// askEvidencePlanOf reads the plan from a validated query; the second value is the reason when the rows cannot be itemised.
func askEvidencePlanOf(v *askValidated, columns []string) (*askEvidencePlan, string) {
	if v == nil || v.Tree == nil || len(v.Tree.Stmts) != 1 {
		return nil, "the query could not be read back"
	}
	root := v.Tree.Stmts[0].Stmt.GetSelectStmt()
	switch {
	case root == nil:
		return nil, "the query could not be read back"
	case v.Recursive:
		return nil, "the answer follows contacts from number to number, so no single set of records stands behind it"
	case root.Op != pg.SetOperation_SETOP_NONE:
		return nil, "the answer combines two result sets, so no single set of records stands behind it"
	case root.WithClause != nil:
		return nil, "the answer is built from intermediate steps, so its rows are not itemised"
	case len(root.FromClause) != 1 || root.FromClause[0].GetRangeVar() == nil:
		return nil, "the answer joins or nests several sources, so its rows are not itemised"
	case len(root.DistinctClause) > 0:
		return nil, "the answer removes repeated values, so one row does not stand for one result"
	case root.LimitOffset != nil || len(root.WindowClause) > 0 || len(root.LockingClause) > 0:
		return nil, "the answer skips rows or uses a window, so its rows are not itemised"
	}
	rv := root.FromClause[0].GetRangeVar()
	source, ok := askEvidenceViews[rv.Relname]
	if !ok {
		return nil, "the answer reads " + rv.Relname + ", which summarises many records; the records behind a summary are not itemised yet"
	}
	plan := &askEvidencePlan{View: rv.Relname, Source: source, Limit: v.Limit}
	if rv.Alias != nil && rv.Alias.Aliasname != "" {
		if !askPlainIdent.MatchString(rv.Alias.Aliasname) {
			return nil, "the query could not be read back"
		}
		plan.Alias = rv.Alias.Aliasname
	}
	if root.WhereClause != nil {
		if plan.Where = askDeparseFragment("where", []*pg.Node{root.WhereClause}); plan.Where == "" {
			return nil, "the conditions of the query could not be read back"
		}
	}
	if len(root.SortClause) > 0 {
		plan.Order = askDeparseFragment("order", root.SortClause)
	}
	aggregates := 0
	for i, t := range root.TargetList {
		rt := t.GetResTarget()
		if rt == nil || rt.Val == nil {
			return nil, "the query could not be read back"
		}
		call := rt.Val.GetFuncCall()
		if call == nil || call.Over != nil {
			if call != nil {
				return nil, "the answer uses a window, so its rows are not itemised"
			}
			continue
		}
		name := ""
		if len(call.Funcname) > 0 {
			name = strings.ToLower(call.Funcname[len(call.Funcname)-1].GetString_().GetSval())
		}
		switch name {
		case "count", "sum", "min", "max", "avg":
		default:
			continue
		}
		aggregates++
		agg := askEvidenceAgg{Fn: name, Index: i, Distinct: call.AggDistinct, Plain: !call.AggDistinct && call.AggFilter == nil && len(call.AggOrder) == 0}
		agg.Text = askDeparseFragment("target", []*pg.Node{{Node: &pg.Node_ResTarget{ResTarget: &pg.ResTarget{Val: rt.Val}}}})
		if agg.Text == "" {
			return nil, "the query could not be read back"
		}
		if len(call.Args) == 1 {
			agg.Column = askColumnName(call.Args[0])
		}
		plan.Aggs = append(plan.Aggs, agg)
	}
	switch {
	case len(root.GroupClause) > 0:
		if aggregates == 0 {
			return nil, "the answer groups rows without counting them, so its rows are not itemised"
		}
		plan.Kind = "group"
		for _, g := range root.GroupClause {
			column := askColumnName(g)
			if column == "" || g.GetColumnRef() == nil {
				return nil, "the answer groups by a computed value (a day, an hour), so the rows of a group are not itemised"
			}
			index := -1
			for i, t := range root.TargetList {
				if rt := t.GetResTarget(); rt != nil && rt.Val != nil && askColumnName(rt.Val) == column && rt.Val.GetColumnRef() != nil {
					index = i
					break
				}
			}
			if index < 0 {
				return nil, "the answer does not show the value it groups by, so the rows of a group are not itemised"
			}
			plan.GroupCols = append(plan.GroupCols, column)
			plan.GroupIndex = append(plan.GroupIndex, index)
		}
	case aggregates > 0:
		if aggregates != len(root.TargetList) {
			return nil, "the answer mixes a total with single values, so its rows are not itemised"
		}
		plan.Kind = "aggregate"
	default:
		plan.Kind = "rows"
	}
	if len(columns) != len(root.TargetList) {
		return nil, "the columns of the answer do not match the query"
	}
	return plan, ""
}

func (p *askEvidencePlan) from() string {
	if p.Alias != "" {
		return p.View + " AS " + p.Alias
	}
	return p.View
}

func (p *askEvidencePlan) where(extra string) string {
	switch {
	case p.Where != "" && extra != "":
		return " WHERE (" + p.Where + ") AND " + extra
	case p.Where != "":
		return " WHERE " + p.Where
	case extra != "":
		return " WHERE " + extra
	}
	return ""
}

// rowsSQL selects the keys of the rows behind the figure, with the columns that say what each row is.
func (p *askEvidencePlan) rowsSQL(extra, order string, limit int) string {
	key := p.Source.Key
	if order == "" {
		order = key
	} else {
		order += ", " + key
	}
	list := key + "::text AS k"
	if len(p.Source.Preview) > 0 {
		list += ", " + strings.Join(p.Source.Preview, ", ")
	}
	return fmt.Sprintf("SELECT %s FROM %s%s ORDER BY %s LIMIT %d", list, p.from(), p.where(extra), order, limit)
}

// filesSQL counts the matching rows in each source file and recomputes each aggregate from the file-level pieces.
func (p *askEvidencePlan) filesSQL(extra string) string {
	var pieces []string
	for i, a := range p.Aggs {
		if !a.Plain {
			continue
		}
		switch a.Fn {
		case "avg":
			inner := strings.TrimPrefix(a.Text, "avg(") // what follows the name, closing bracket included
			pieces = append(pieces, fmt.Sprintf("sum(%s AS a%d_s", inner, i), fmt.Sprintf("count(%s AS a%d_c", inner, i))
		default:
			pieces = append(pieces, fmt.Sprintf("%s AS a%d", a.Text, i))
		}
	}
	list := "evidence_id::text AS f, count(*) AS n"
	if len(pieces) > 0 {
		list += ", " + strings.Join(pieces, ", ")
	}
	return fmt.Sprintf("SELECT %s FROM %s%s GROUP BY evidence_id ORDER BY n DESC, f LIMIT %d", list, p.from(), p.where(extra), askEvidenceFileCap+1)
}

// extremeOrder puts the extreme row of a single minimum or maximum first.
func (p *askEvidencePlan) extremeOrder() string {
	if len(p.Aggs) != 1 || p.Aggs[0].Column == "" || !p.Aggs[0].Plain {
		return ""
	}
	switch p.Aggs[0].Fn {
	case "max":
		return p.Aggs[0].Column + " DESC NULLS LAST"
	case "min":
		return p.Aggs[0].Column + " ASC NULLS LAST"
	}
	return ""
}

// askEvidenceLiteral writes a value of the answer as an SQL literal for the condition of a group; false when it cannot be written safely.
func askEvidenceLiteral(value any) (string, bool) {
	switch x := value.(type) {
	case string:
		if strings.ContainsRune(x, 0) {
			return "", false
		}
		return "'" + strings.ReplaceAll(x, "'", "''") + "'", true
	case int:
		return strconv.Itoa(x), true
	case int32:
		return strconv.Itoa(int(x)), true
	case int64:
		return strconv.FormatInt(x, 10), true
	}
	return "", false
}

func askEvidenceNumber(value any) (float64, bool) {
	switch x := value.(type) {
	case int:
		return float64(x), true
	case int32:
		return float64(x), true
	case int64:
		return float64(x), true
	case float32:
		return float64(x), true
	case float64:
		return x, true
	case json.Number:
		f, err := x.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(x), ",", ""), 64)
		return f, err == nil
	}
	return 0, false
}

// ---- the result ----------------------------------------------------------------------------------------------------------------------------------------

type askEvidenceRow struct {
	Key        string `json:"key"`
	EvidenceID string `json:"evidence_id"`
	SourceFile string `json:"source_file"`
	VersionID  string `json:"version_id,omitempty"`
	SourceRow  int    `json:"source_row,omitempty"` // the typed row's own position in the statement
	SheetRow   int    `json:"row_number,omitempty"` // the row of the sheet as the evidence viewer shows it; 0 when it could not be found
	RowHash    string `json:"row_hash,omitempty"`
	Preview    string `json:"preview,omitempty"`
}

type askEvidenceFile struct {
	EvidenceID string `json:"evidence_id"`
	SourceFile string `json:"source_file"`
	Rows       int    `json:"rows"`
}

type askEvidence struct {
	State      string            `json:"state"` // "itemised" or "not_itemised"
	Reason     string            `json:"reason,omitempty"`
	Kind       string            `json:"kind,omitempty"`
	View       string            `json:"view,omitempty"`
	Scope      string            `json:"scope,omitempty"` // which rows the list shows, in words
	Total      int               `json:"total_rows"`
	Files      []askEvidenceFile `json:"files,omitempty"`
	Rows       []askEvidenceRow  `json:"rows,omitempty"`
	Reconciled string            `json:"reconciled,omitempty"` // "yes", "no" or "not_checked"
	Check      string            `json:"check,omitempty"`      // the sentence that says what was recomputed
	Truncated  bool              `json:"truncated,omitempty"`
}

func askEvidenceNone(reason string) *askEvidence {
	return &askEvidence{State: "not_itemised", Reason: reason}
}

// askEvidenceEnabled: on unless the environment turns it off.
func askEvidenceEnabled() bool {
	return !strings.EqualFold(strings.TrimSpace(os.Getenv(askEvidenceEnv)), "false") //nolint:forbidigo // forensic records product API reads its settings from the environment by design (docker compose)
}

// askCollectEvidenceFn is the seam a spec replaces; the default is the real thing.
var askCollectEvidenceFn = askCollectEvidence

// askCollectEvidence finds the rows behind an answer. It never fails the answer: every problem ends in a "not itemised" with the reason.
func askCollectEvidence(ctx context.Context, db *pgxpool.Pool, tenantID, collectionID string, cat *askCatalogue, v *askValidated, result *govSQLResult, limits askLimits) *askEvidence {
	if db == nil || v == nil || result == nil || !askEvidenceEnabled() {
		return nil
	}
	plan, reason := askEvidencePlanOf(v, result.Columns)
	if plan == nil {
		return askEvidenceNone(reason)
	}
	ctx, cancel := context.WithTimeout(ctx, askEvidenceBudget)
	defer cancel()
	ev := &askEvidence{State: "itemised", Kind: plan.Kind, View: plan.View}

	run := func(sql string, maxRows int) (*govSQLResult, error) {
		checked, rej := askValidate(sql, cat, []string{plan.View})
		if rej != nil {
			return nil, fmt.Errorf("%s", rej.Message)
		}
		return askRun(ctx, db, tenantID, collectionID, checked.SQL, nil, limits, maxRows, true)
	}

	extra := ""
	switch plan.Kind {
	case "group":
		if len(result.Rows) == 0 {
			return askEvidenceNone("the answer has no row")
		}
		var parts []string
		for i, column := range plan.GroupCols {
			literal, ok := askEvidenceLiteral(result.Rows[0][plan.GroupIndex[i]])
			if !ok {
				return askEvidenceNone("the value the answer groups by cannot be turned back into a condition")
			}
			parts = append(parts, column+" = "+literal)
		}
		extra = strings.Join(parts, " AND ")
		ev.Scope = "the rows of the first result (" + strings.Join(parts, ", ") + ")"
	case "aggregate":
		ev.Scope = "the rows the figure is computed over"
		if order := plan.extremeOrder(); order != "" {
			ev.Scope = "the row with the " + map[string]string{"max": "largest", "min": "smallest"}[plan.Aggs[0].Fn] + " " + plan.Aggs[0].Column + " first, then the rows that share the figure's conditions"
		}
	default:
		ev.Scope = "the rows of the answer"
	}

	// 1. the rows themselves (a handful of them: the extreme row, the rows of the answer, the first rows of the group)
	rowLimit := askEvidenceRowCap
	if plan.Kind == "rows" && plan.Limit > 0 && plan.Limit < rowLimit {
		rowLimit = plan.Limit
	}
	order := plan.Order
	if plan.Kind != "rows" {
		order = plan.extremeOrder()
	}
	rows, err := run(plan.rowsSQL(extra, order, rowLimit), askEvidenceRowCap+1)
	orderFallback := false
	if err != nil && order != "" {
		orderFallback = true
		rows, err = run(plan.rowsSQL(extra, "", rowLimit), askEvidenceRowCap+1) // the order named an output name the view does not have
	}
	if err != nil {
		return askEvidenceNone("the rows could not be read back (" + govSQLClip(err.Error(), 120) + ")")
	}
	var keys []string
	previews := map[string]string{}
	for _, r := range rows.Rows {
		key := fmt.Sprint(r[0])
		keys = append(keys, key)
		var parts []string
		for i := 1; i < len(r) && i < len(rows.Columns); i++ {
			if r[i] != nil && fmt.Sprint(r[i]) != "" {
				parts = append(parts, rows.Columns[i]+" "+fmt.Sprint(r[i]))
			}
		}
		previews[key] = govSQLClip(strings.Join(parts, "; "), 240)
	}

	// 2. the matching rows in each source file, and the figure recomputed from those pieces
	if plan.Kind != "rows" {
		files, err := run(plan.filesSQL(extra), askEvidenceFileCap+1)
		if err != nil {
			return askEvidenceNone("the files behind the figure could not be counted (" + govSQLClip(err.Error(), 120) + ")")
		}
		askEvidenceReconcile(ev, plan, result, files)
	}

	// 3. where each row lives in the source
	if err := askEvidenceLocate(ctx, db, tenantID, collectionID, plan.Source, keys, previews, ev); err != nil {
		return askEvidenceNone("the place of the rows in their files could not be read (" + govSQLClip(err.Error(), 120) + ")")
	}
	if plan.Kind == "rows" {
		ev.Total = len(result.Rows)
		ev.Truncated = result.Truncated
		ev.Reconciled, ev.Check = "not_checked", ""
		switch {
		case orderFallback:
			ev.Check = "The rows listed meet the conditions of the answer, but the answer's own order could not be repeated, so they may not be the same rows."
		case len(keys) == len(result.Rows):
			ev.Reconciled = "yes"
			ev.Check = fmt.Sprintf("The %d rows listed are the %d rows of the answer.", len(keys), len(result.Rows))
		case len(result.Rows) > 0:
			ev.Reconciled = "no"
			ev.Check = fmt.Sprintf("The answer has %d rows and the records read back for it are %d, so the list is not shown as proof of the answer.", len(result.Rows), len(keys))
		}
		position := map[string]int{}
		for _, r := range ev.Rows {
			i, seen := position[r.EvidenceID]
			if !seen {
				i = len(ev.Files)
				position[r.EvidenceID] = i
				ev.Files = append(ev.Files, askEvidenceFile{EvidenceID: r.EvidenceID, SourceFile: r.SourceFile})
			}
			ev.Files[i].Rows++
		}
	}
	return ev
}

// askEvidenceReconcile recomputes the figure(s) from the per-file pieces and records whether they agree.
func askEvidenceReconcile(ev *askEvidence, plan *askEvidencePlan, result *govSQLResult, files *govSQLResult) {
	index := map[string]int{}
	for i, c := range files.Columns {
		index[c] = i
	}
	if len(files.Rows) > askEvidenceFileCap {
		ev.Reconciled, ev.Check = "not_checked", "More than 100 files hold matching rows, so the figure was not recomputed from them."
		files.Rows = files.Rows[:askEvidenceFileCap]
		ev.Truncated = true
	}
	total := 0
	for _, r := range files.Rows {
		n, _ := askEvidenceNumber(r[index["n"]])
		total += int(n)
		ev.Files = append(ev.Files, askEvidenceFile{EvidenceID: fmt.Sprint(r[index["f"]]), Rows: int(n)})
	}
	ev.Total = total
	if ev.Reconciled == "not_checked" {
		return
	}
	if len(result.Rows) == 0 {
		ev.Reconciled = "not_checked"
		return
	}
	rowOfAnswer := result.Rows[0]
	checked, bad := 0, 0
	var said []string
	for i, a := range plan.Aggs {
		if !a.Plain {
			continue
		}
		figure, ok := askEvidenceNumber(rowOfAnswer[a.Index])
		if !ok {
			continue
		}
		var got float64
		have := false
		switch a.Fn {
		case "count", "sum":
			for _, r := range files.Rows {
				if x, ok := askEvidenceNumber(r[index[fmt.Sprintf("a%d", i)]]); ok {
					got += x
					have = true
				}
			}
			have = have || len(files.Rows) == 0
		case "min", "max":
			for _, r := range files.Rows {
				if x, ok := askEvidenceNumber(r[index[fmt.Sprintf("a%d", i)]]); ok {
					if !have || (a.Fn == "min" && x < got) || (a.Fn == "max" && x > got) {
						got = x
					}
					have = true
				}
			}
		case "avg":
			var sum, count float64
			for _, r := range files.Rows {
				s, ok1 := askEvidenceNumber(r[index[fmt.Sprintf("a%d_s", i)]])
				c, ok2 := askEvidenceNumber(r[index[fmt.Sprintf("a%d_c", i)]])
				if ok1 && ok2 {
					sum += s
					count += c
				}
			}
			if count > 0 {
				got, have = sum/count, true
			}
		}
		if !have {
			continue
		}
		checked++
		if math.Abs(got-figure) > 1e-6*math.Max(1, math.Abs(figure)) {
			bad++
			said = append(said, fmt.Sprintf("%s gives %s, the answer says %s", a.Text, askEvidenceFormat(got), askEvidenceFormat(figure)))
		} else {
			said = append(said, fmt.Sprintf("%s = %s", a.Text, askEvidenceFormat(figure)))
		}
	}
	switch {
	case checked == 0:
		ev.Reconciled, ev.Check = "not_checked", "The figure was not recomputed from the rows (it is a count of different values or a filtered total)."
	case bad == 0:
		ev.Reconciled = "yes"
		ev.Check = fmt.Sprintf("Recomputed from the %s matching rows in %s: %s.", askEvidenceFormat(float64(total)), askEvidencePlural(len(files.Rows), "file"), strings.Join(said, "; "))
	default:
		ev.Reconciled = "no"
		ev.Check = "The rows do not give the figure: " + strings.Join(said, "; ") + ". The rows are listed for inspection only."
	}
}

func askEvidencePlural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return strconv.Itoa(n) + " " + word + "s"
}

func askEvidenceFormat(f float64) string {
	if f == math.Trunc(f) && math.Abs(f) < 1e15 {
		return govSQLGroup(strconv.FormatInt(int64(f), 10))
	}
	return strconv.FormatFloat(f, 'f', 4, 64)
}

// ---- where a row lives in its source -------------------------------------------------------------------------------------------------------------------

// askEvidenceLocate fills ev.Rows from the stored rows (file, version, row) and finds the row of the sheet by content. As the runtime role with the tenant set, like the case settings.
func askEvidenceLocate(ctx context.Context, db *pgxpool.Pool, tenantID, collectionID string, source askEvidenceSource, keys []string, previews map[string]string, ev *askEvidence) error {
	if len(keys) == 0 {
		return nil
	}
	tx, err := db.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // read-only transaction
	if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenantID); err != nil {
		return err
	}
	rowColumn, payloadColumn := "0", "''::text"
	if source.HasRow {
		rowColumn = "coalesce(t.source_row, 0)"
	}
	if source.Locatable {
		payloadColumn = "coalesce(t.raw_payload::text, '{}')"
	}
	// the names come from askEvidenceViews, never from a question
	lookup := fmt.Sprintf(`SELECT t.%[1]s::text, t.evidence_id::text, coalesce(nullif(e.source_file, ''), e.original_filename, ''), coalesce(e.current_version_id::text, ''), %[2]s, %[3]s
FROM forensic.%[4]s t
JOIN forensic.evidence_items e ON e.tenant_id = t.tenant_id AND e.evidence_id = t.evidence_id
WHERE t.tenant_id = $1 AND t.collection_id = $2 AND t.%[1]s::text = ANY($3)`, source.Key, rowColumn, payloadColumn, source.Base)
	found, err := tx.Query(ctx, lookup, tenantID, collectionID, keys)
	if err != nil {
		return err
	}
	byKey := map[string]askEvidenceRow{}
	payloads := map[string]string{}
	for found.Next() {
		var r askEvidenceRow
		var payload string
		if err := found.Scan(&r.Key, &r.EvidenceID, &r.SourceFile, &r.VersionID, &r.SourceRow, &payload); err != nil {
			found.Close()
			return err
		}
		byKey[r.Key] = r
		payloads[r.Key] = payload
	}
	found.Close()
	if err := found.Err(); err != nil {
		return err
	}
	offsets := map[string]int{}
	known := map[string]bool{}
	for _, key := range keys {
		r, ok := byKey[key]
		if !ok {
			continue
		}
		payload := payloads[key]
		if source.Locatable && payload != "" && payload != "{}" {
			// the file's own offset first (a statement's title and header rows), checked by content; the whole file's rows otherwise
			if known[r.EvidenceID] {
				if row, hash, ok := askEvidenceSheetRow(ctx, tx, tenantID, r.EvidenceID, r.SourceRow+offsets[r.EvidenceID], payload); ok {
					r.SheetRow, r.RowHash = row, hash
				}
			}
			if r.SheetRow == 0 {
				if row, hash, ok := askEvidenceSheetRow(ctx, tx, tenantID, r.EvidenceID, 0, payload); ok {
					r.SheetRow, r.RowHash = row, hash
					offsets[r.EvidenceID], known[r.EvidenceID] = row-r.SourceRow, true
				}
			}
		}
		r.Preview = previews[key]
		ev.Rows = append(ev.Rows, r)
	}
	// a file the answer counts but whose rows are not listed still needs its name
	names := map[string]string{}
	for _, r := range ev.Rows {
		names[r.EvidenceID] = r.SourceFile
	}
	var missing []string
	for _, f := range ev.Files {
		if names[f.EvidenceID] == "" && f.EvidenceID != "" {
			missing = append(missing, f.EvidenceID)
		}
	}
	if len(missing) > 0 {
		named, err := tx.Query(ctx, `SELECT evidence_id::text, coalesce(nullif(source_file, ''), original_filename, '') FROM forensic.evidence_items WHERE tenant_id = $1 AND collection_id = $2 AND evidence_id::text = ANY($3)`, tenantID, collectionID, missing)
		if err != nil {
			return err
		}
		for named.Next() {
			var id, name string
			if err := named.Scan(&id, &name); err != nil {
				named.Close()
				return err
			}
			names[id] = name
		}
		named.Close()
		if err := named.Err(); err != nil {
			return err
		}
	}
	for i := range ev.Files {
		if ev.Files[i].SourceFile == "" {
			ev.Files[i].SourceFile = names[ev.Files[i].EvidenceID]
		}
	}
	sort.SliceStable(ev.Files, func(a, b int) bool { return ev.Files[a].Rows > ev.Files[b].Rows })
	return nil
}

// askEvidenceSheetRow finds the row of the file's sheet that holds every cell the typed row kept. With row 0 it searches the whole file (the first match); with a row number it checks that one.
func askEvidenceSheetRow(ctx context.Context, tx pgx.Tx, tenantID, evidenceID string, row int, payload string) (int, string, bool) {
	where := ""
	args := []any{tenantID, evidenceID, payload}
	if row > 0 {
		where = " AND r.row_number = $4"
		args = append(args, row)
	}
	var number int
	var hash string
	err := tx.QueryRow(ctx, `SELECT r.row_number, coalesce(r.row_hash, '') FROM forensic.records r
WHERE r.tenant_id = $1 AND r.evidence_id::text = $2`+where+`
  AND NOT EXISTS (SELECT 1 FROM jsonb_each_text($3::jsonb) t WHERE t.value <> '' AND t.value NOT IN (SELECT value FROM jsonb_each_text(r.raw_payload - '_xlsx_cells')))
ORDER BY r.row_number LIMIT 1`, args...).Scan(&number, &hash)
	return number, hash, err == nil && number > 0 && hash != ""
}

// ---- into the response ------------------------------------------------------------------------------------------------------------------------------------

// askAttachEvidence adds the evidence to an answer: the structured block, the openable sources the workspace already knows how to show, and one plain sentence among the notes.
func askAttachEvidence(resp *hybridQueryResponse, ev *askEvidence) {
	if resp == nil || ev == nil || resp.Enterprise == nil {
		return
	}
	resp.Enterprise["evidence"] = ev
	notes, _ := resp.Enterprise["limitations"].([]string)
	if ev.State != "itemised" {
		resp.Enterprise["limitations"] = append(notes, "The records behind this answer are not itemised: "+ev.Reason+".")
		return
	}
	provenance, _ := resp.Enterprise["provenance"].([]map[string]any)
	shown := 0
	for _, r := range ev.Rows {
		if r.SheetRow == 0 || r.RowHash == "" || r.VersionID == "" {
			continue
		}
		provenance = append(provenance, map[string]any{
			"source": "analyst_views_sql", "kind": "ask_engine_v2_evidence", "citation_id": "evidence-" + r.Key,
			"evidence_id": r.EvidenceID, "version_id": r.VersionID, "source_file": r.SourceFile,
			"row_number": r.SheetRow, "row_hash": r.RowHash, "preview": r.Preview,
			"source_truth_state": "ingested_source_record", "proof_role": "evidence row",
			"locator": map[string]any{"row_number": r.SheetRow, "row_hash": r.RowHash, "source_file": r.SourceFile, "source_truth_state": "ingested_source_record"},
		})
		shown++
	}
	if len(ev.Files) > 0 && ev.Total > 0 {
		var sources []map[string]any
		for _, f := range ev.Files {
			sources = append(sources, map[string]any{"source_file": f.SourceFile, "contribution_count": f.Rows})
		}
		provenance = append(provenance, map[string]any{"contract_version": "forensics.contribution-lineage/v1", "contribution_count": ev.Total, "sources": sources})
	}
	resp.Enterprise["provenance"] = provenance
	line := fmt.Sprintf("%s rows in %s are behind this answer", govSQLGroup(strconv.Itoa(ev.Total)), askEvidencePlural(len(ev.Files), "source file"))
	if ev.Kind == "rows" {
		line = fmt.Sprintf("The %s rows shown come from %s", govSQLGroup(strconv.Itoa(ev.Total)), askEvidencePlural(len(ev.Files), "source file"))
	}
	if ev.Check != "" {
		line += ". " + ev.Check
	} else {
		line += "."
	}
	resp.Enterprise["limitations"] = append(notes, line)
}
