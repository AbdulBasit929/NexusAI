package main

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	pg "github.com/pganalyze/pg_query_go/v6"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// ASK ENGINE V2: A TIE FOR THE TOP PLACE (the verified bank run of 2026-11-02, VB-PLAT-051).
//
// "Which plate was seen the most?" was answered with `... ORDER BY value DESC LIMIT 1`: one plate, SYNT4024 with 50 sightings, where SYNT2905 was seen 50 times too. The query is what the
// question asks for, and the answer is still wrong, because "the most" is a place that two plates can share. The model cannot see the data, so it cannot know to write the tie; the code can.
//
// After a ranking that keeps ONE row (ORDER BY ... LIMIT 1) has run, the same query is run once more with room for fifty rows more (the parser's own text with a larger LIMIT, validated again
// like any query), and every row whose figure equals the first row's is kept. Two or more are a tie, and the answer says so ("2 plates tie for the top place at 50: SYNT2905, SYNT4024").
// A ranking that shows several rows already shows a tie at the top; "the top five" is five by the question's own count and is left as it is.

// askTieWindow is how many rows past the LIMIT are looked at for a tie.
const askTieWindow = 50

// askTrailingLimit is the LIMIT at the end of the parser's own text of a query (the outermost one).
var askTrailingLimit = regexp.MustCompile(`\s+LIMIT\s+(\d+)\s*$`)

// askRootSelect is the outermost SELECT of a validated query.
func askRootSelect(v *askValidated) *pg.SelectStmt {
	if v == nil || v.Tree == nil || len(v.Tree.Stmts) != 1 {
		return nil
	}
	return v.Tree.Stmts[0].GetStmt().GetSelectStmt()
}

// askNodeKey is a comparable form of an expression that ignores where in the text it stood.
func askNodeKey(n *pg.Node) string {
	if n == nil {
		return ""
	}
	clone, ok := proto.Clone(n).(*pg.Node)
	if !ok {
		return ""
	}
	var all []protoreflect.Message
	askWalkTree(clone.ProtoReflect(), func(m protoreflect.Message) bool {
		all = append(all, m)
		return true
	})
	for _, m := range all {
		if fd := m.Descriptor().Fields().ByName("location"); fd != nil && fd.Kind() == protoreflect.Int32Kind {
			m.Clear(fd)
		}
	}
	out, err := proto.MarshalOptions{Deterministic: true}.Marshal(clone)
	if err != nil {
		return ""
	}
	return string(out)
}

// askRankKey is the index of the result column that the first key of the ORDER BY sorts by: a position (ORDER BY 2), a name (an output column, or the column an output column is made of),
// or an expression that an output column is made of (ORDER BY count(*) against count(*) AS value). ok is false when it cannot be told, and then no tie is looked for.
func askRankKey(v *askValidated, columns []string) (int, bool) {
	root := askRootSelect(v)
	if root == nil || len(root.SortClause) == 0 {
		return 0, false
	}
	sortBy := root.SortClause[0].GetSortBy()
	if sortBy == nil || sortBy.Node == nil {
		return 0, false
	}
	node := sortBy.Node
	if c := node.GetAConst(); c != nil && c.GetIval() != nil {
		if k := int(c.GetIval().Ival); k >= 1 && k <= len(columns) {
			return k - 1, true
		}
		return 0, false
	}
	// a star in the select list shifts the output columns away from the targets
	for _, target := range root.TargetList {
		if rt := target.GetResTarget(); rt != nil && rt.Val != nil && rt.Val.GetColumnRef() != nil && askColumnName(rt.Val) == "" {
			return 0, false
		}
	}
	if name := askColumnName(node); name != "" {
		for i, c := range columns {
			if strings.EqualFold(c, name) {
				return i, true
			}
		}
		for i, target := range root.TargetList {
			if rt := target.GetResTarget(); rt != nil && rt.Val != nil && strings.EqualFold(askColumnName(rt.Val), name) && i < len(columns) {
				return i, true
			}
		}
		return 0, false
	}
	want := askNodeKey(node)
	if want == "" {
		return 0, false
	}
	for i, target := range root.TargetList {
		if rt := target.GetResTarget(); rt != nil && rt.Val != nil && i < len(columns) && askNodeKey(rt.Val) == want {
			return i, true
		}
	}
	return 0, false
}

// askExtendTies runs a one-row ranking again with room for a tie and returns the rows that share the top figure, and how many they are (0 or 1 when there is no tie and the first result
// stands). It never fails the answer: whatever goes wrong, the first result is returned unchanged.
func askExtendTies(ctx context.Context, db *pgxpool.Pool, tenantID, collectionID string, cat *askCatalogue, allowed []string, f askFrame, v *askValidated, first *govSQLResult, limits askLimits) (*govSQLResult, int) {
	if v == nil || first == nil || v.Limit != 1 || len(first.Rows) != 1 || first.Truncated || !f.Cues["rank"] || f.Limit > 1 {
		return first, 0
	}
	key, ok := askRankKey(v, first.Columns)
	if !ok || key >= len(first.Rows[0]) {
		return first, 0
	}
	m := askTrailingLimit.FindStringSubmatchIndex(v.SQL)
	if m == nil || v.SQL[m[2]:m[3]] != "1" {
		return first, 0
	}
	widened, rejection := askValidate(v.SQL[:m[0]]+" LIMIT "+strconv.Itoa(1+askTieWindow), cat, allowed)
	if rejection != nil {
		return first, 0
	}
	more, err := askExec(ctx, db, tenantID, collectionID, widened, limits)
	if err != nil || more == nil || len(more.Rows) == 0 || key >= len(more.Columns) || key >= len(more.Rows[0]) {
		return first, 0
	}
	top := fmt.Sprint(more.Rows[0][key])
	if top != fmt.Sprint(first.Rows[0][key]) {
		return first, 0 // the data moved between the two runs: nothing is claimed
	}
	tied := 0
	for tied < len(more.Rows) && key < len(more.Rows[tied]) && fmt.Sprint(more.Rows[tied][key]) == top {
		tied++
	}
	if tied < 2 {
		return first, 0
	}
	out := *more
	out.Rows = more.Rows[:tied]
	out.Truncated = tied == 1+askTieWindow // every row looked at ties: there may be more
	return &out, tied
}

// askLabelNouns name what a label column holds, by the column of the view the label is made of. A label made of anything else is called a result.
var askLabelNouns = map[string]string{
	"plate_norm": "plate", "plate_raw": "plate", "src": "number", "dst": "number", "msisdn": "number", "account": "account", "counterparty_account": "account", "cell_key": "cell site",
	"app": "app", "host": "host", "imei": "handset", "camera_id": "camera", "a": "number", "b": "number",
}

// askLabelNoun is the noun for the rows of a ranking, read from the column the `label` output is made of.
func askLabelNoun(v *askValidated) string {
	root := askRootSelect(v)
	if root == nil {
		return "result"
	}
	for _, target := range root.TargetList {
		rt := target.GetResTarget()
		if rt == nil || rt.Val == nil || !strings.EqualFold(rt.Name, "label") {
			continue
		}
		if noun, ok := askLabelNouns[askColumnName(rt.Val)]; ok {
			return noun
		}
	}
	return "result"
}

// askTieHeadline is the sentence for a ranking whose top place is shared: "2 plates tie for the top place at 50: SYNT2905, SYNT4024." Every figure in it is a figure of the result.
func askTieHeadline(r *govSQLResult, v *askValidated) string {
	l, value := askColumnIndex(r, "label"), askColumnIndex(r, "value")
	if l < 0 || value < 0 || len(r.Rows) < 2 {
		return ""
	}
	noun := askLabelNoun(v)
	items := make([]string, 0, askHeadlineItems)
	for i := 0; i < len(r.Rows) && i < askHeadlineItems; i++ {
		items = append(items, askCell(r, l, r.Rows[i][l]))
	}
	tail := ""
	if len(r.Rows) > askHeadlineItems {
		tail = fmt.Sprintf(", and %d more", len(r.Rows)-askHeadlineItems)
	}
	count := askCount(len(r.Rows), noun, noun+"s")
	if r.Truncated {
		count = "At least " + count
	}
	return fmt.Sprintf("%s tie for the top place at %s: %s%s.", count, askCell(r, value, r.Rows[0][value]), strings.Join(items, ", "), tail)
}
