package caselayer

import (
	"encoding/json"
	"math/big"
	"sort"
	"strings"
	"time"
)

// FiguresMaxRows is how many rows of a result a turn keeps for comparison besides the snapshot. The digest of ALL rows is kept as well, so equality is exact whatever the size.
const FiguresMaxRows = 200

// diffListMax is how many added, removed and changed rows a diff lists; the totals are exact.
const diffListMax = 50

// Figures is the part of an answer that a re-run is compared on: the result table (first rows and a digest of all of them) and the labelled figures.
type Figures struct {
	Columns    []Column `json:"columns"`
	Rows       [][]any  `json:"rows"`
	RowCount   int      `json:"row_count"`
	RowsDigest string   `json:"rows_digest"`
	Metrics    []Metric `json:"metrics"`
	Truncated  bool     `json:"truncated"`
}

// Figures reads the comparable part from a digest.
func (d *Digest) Figures() Figures {
	f := Figures{Columns: d.Columns, RowCount: d.RowCount, RowsDigest: d.ResultDigest(), Metrics: d.Metrics}
	if f.Columns == nil {
		f.Columns = []Column{}
	}
	if f.Metrics == nil {
		f.Metrics = []Metric{}
	}
	rows := d.Rows
	if len(rows) > FiguresMaxRows {
		rows = rows[:FiguresMaxRows]
		f.Truncated = true
	}
	if rows == nil {
		rows = [][]any{}
	}
	f.Rows = rows
	if f.RowCount > len(rows) {
		f.Truncated = true
	}
	return f
}

// Run is one run of a question as the diff sees it.
type Run struct {
	TurnID          string        `json:"turn_id"`
	Outcome         string        `json:"outcome"`
	Headline        string        `json:"headline"`
	Figures         Figures       `json:"-"`
	Evidence        []EvidenceRef `json:"-"`
	QueryHash       string        `json:"query_hash"`
	DataVersionHash string        `json:"data_version_hash"`
	CompletedAt     *time.Time    `json:"completed_at"`
}

// Change is one text that differs.
type Change struct {
	Before  string `json:"before"`
	After   string `json:"after"`
	Changed bool   `json:"changed"`
}

// MetricChange is one labelled figure that differs. Delta is set when both values are numbers.
type MetricChange struct {
	Label  string `json:"label"`
	Before string `json:"before"`
	After  string `json:"after"`
	Delta  string `json:"delta,omitempty"`
}

// RowChange is a row present in both runs under the same first cell but with other cells.
type RowChange struct {
	Key   string       `json:"key"`
	Cells []CellChange `json:"cells"`
}

// CellChange is one cell that differs.
type CellChange struct {
	Column string `json:"column"`
	Before any    `json:"before"`
	After  any    `json:"after"`
}

// RowsDiff compares the result tables. Rows are compared as sets (by the whole row); a table whose rows only came in another order is Reordered, not changed. When the first cell is unique
// in both tables a removed row and an added row with the same first cell are one changed row.
type RowsDiff struct {
	BeforeCount    int         `json:"before_count"`
	AfterCount     int         `json:"after_count"`
	ColumnsChanged bool        `json:"columns_changed"`
	Added          [][]any     `json:"added"`
	Removed        [][]any     `json:"removed"`
	Changed        []RowChange `json:"changed"`
	AddedTotal     int         `json:"added_total"`
	RemovedTotal   int         `json:"removed_total"`
	ChangedTotal   int         `json:"changed_total"`
	Reordered      bool        `json:"reordered"`
	Listed         int         `json:"listed_per_class"`
}

// EvidenceDiff compares the files the two answers cite.
type EvidenceDiff struct {
	Added    []EvidenceRef `json:"added"`
	Removed  []EvidenceRef `json:"removed"`
	Replaced []DVReplaced  `json:"replaced"`
}

// DataDiff says whether the case's data moved between the two runs.
type DataDiff struct {
	Before  string             `json:"before"`
	After   string             `json:"after"`
	Known   bool               `json:"known"`
	Changed bool               `json:"changed"`
	Detail  *DataVersionChange `json:"detail,omitempty"`
}

// QueryDiff compares the query the model wrote each time.
type QueryDiff struct {
	BeforeHash string `json:"before_hash"`
	AfterHash  string `json:"after_hash"`
	Changed    bool   `json:"changed"`
}

// Diff is the answer to "what changed since": the outcome, the sentence, the figures, the evidence set, the data version and the query.
type Diff struct {
	Of                      string         `json:"of"`
	Against                 string         `json:"against"`
	Since                   *time.Time     `json:"since"`
	Changed                 bool           `json:"changed"`
	FiguresChanged          bool           `json:"figures_changed"`
	EvidenceChanged         bool           `json:"evidence_changed"`
	Complete                bool           `json:"complete"`
	Outcome                 Change         `json:"outcome"`
	Headline                Change         `json:"headline"`
	Metrics                 []MetricChange `json:"metrics"`
	Rows                    RowsDiff       `json:"rows"`
	Evidence                EvidenceDiff   `json:"evidence"`
	Data                    DataDiff       `json:"data_version"`
	Query                   QueryDiff      `json:"query"`
	SameDataDifferentAnswer bool           `json:"same_data_different_answer"`
}

// Compare compares a later run (after) with an earlier one (before). The data version documents are optional: with them the diff names the files and families that moved.
func Compare(before, after Run, dvBefore, dvAfter *DataVersion) *Diff {
	d := &Diff{Of: after.TurnID, Against: before.TurnID, Since: before.CompletedAt, Complete: true}
	d.Outcome = Change{Before: before.Outcome, After: after.Outcome, Changed: before.Outcome != after.Outcome}
	d.Headline = Change{Before: before.Headline, After: after.Headline, Changed: before.Headline != after.Headline}
	d.Metrics = compareMetrics(before.Figures.Metrics, after.Figures.Metrics)
	d.Rows = compareRows(before.Figures, after.Figures)
	if before.Figures.Truncated || after.Figures.Truncated {
		// only the first rows of a long table are kept: the digest of all rows still says whether anything differs, the lists below are the first rows only
		d.Complete = false
	}
	// a table whose rows only came in another order is not a changed table (Reordered is set only when every row was compared)
	rowsDiffer := before.Figures.RowsDigest != after.Figures.RowsDigest && !d.Rows.Reordered
	d.FiguresChanged = len(d.Metrics) > 0 || rowsDiffer || d.Rows.ColumnsChanged
	d.Evidence = compareEvidence(before.Evidence, after.Evidence)
	d.EvidenceChanged = len(d.Evidence.Added)+len(d.Evidence.Removed)+len(d.Evidence.Replaced) > 0
	d.Data = DataDiff{Before: before.DataVersionHash, After: after.DataVersionHash, Known: before.DataVersionHash != "" && after.DataVersionHash != ""}
	d.Data.Changed = d.Data.Known && before.DataVersionHash != after.DataVersionHash
	if d.Data.Changed && dvBefore != nil && dvAfter != nil {
		ch := CompareDataVersions(*dvBefore, *dvAfter)
		d.Data.Detail = &ch
	}
	d.Query = QueryDiff{BeforeHash: before.QueryHash, AfterHash: after.QueryHash, Changed: before.QueryHash != after.QueryHash}
	d.Changed = d.Outcome.Changed || d.Headline.Changed || d.FiguresChanged || d.EvidenceChanged
	d.SameDataDifferentAnswer = d.Data.Known && !d.Data.Changed && (d.Outcome.Changed || d.FiguresChanged)
	return d
}

func compareMetrics(before, after []Metric) []MetricChange {
	out := []MetricChange{}
	b := map[string]string{}
	for _, m := range before {
		b[m.Label] = m.Value
	}
	seen := map[string]bool{}
	for _, m := range after {
		seen[m.Label] = true
		old, ok := b[m.Label]
		if !ok || old != m.Value {
			out = append(out, MetricChange{Label: m.Label, Before: old, After: m.Value, Delta: numericDelta(old, m.Value)})
		}
	}
	for _, m := range before {
		if !seen[m.Label] {
			out = append(out, MetricChange{Label: m.Label, Before: m.Value})
		}
	}
	return out
}

// numericDelta is after minus before as text, when both are numbers (a thousands separator is allowed); empty otherwise.
func numericDelta(before, after string) string {
	parse := func(s string) (*big.Rat, bool) {
		s = strings.ReplaceAll(strings.TrimSpace(s), ",", "")
		if s == "" {
			return nil, false
		}
		return new(big.Rat).SetString(s)
	}
	a, ok1 := parse(before)
	b, ok2 := parse(after)
	if !ok1 || !ok2 {
		return ""
	}
	diff := new(big.Rat).Sub(b, a)
	if diff.IsInt() {
		return diff.Num().String()
	}
	return diff.FloatString(4)
}

func rowKey(row []any) string {
	raw, _ := json.Marshal(row)
	return string(raw)
}

func cellText(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	raw, _ := json.Marshal(v)
	return string(raw)
}

func sameColumns(a, b []Column) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Key != b[i].Key {
			return false
		}
	}
	return true
}

func compareRows(before, after Figures) RowsDiff {
	r := RowsDiff{BeforeCount: before.RowCount, AfterCount: after.RowCount, Added: [][]any{}, Removed: [][]any{}, Changed: []RowChange{}, Listed: diffListMax}
	if !sameColumns(before.Columns, after.Columns) {
		r.ColumnsChanged = len(before.Columns) > 0 || len(after.Columns) > 0
		return r
	}
	count := map[string]int{}
	for _, row := range before.Rows {
		count[rowKey(row)]++
	}
	var addedRows, removedRows [][]any
	for _, row := range after.Rows {
		k := rowKey(row)
		if count[k] > 0 {
			count[k]--
			continue
		}
		addedRows = append(addedRows, row)
	}
	used := map[string]int{}
	for _, row := range after.Rows {
		used[rowKey(row)]++
	}
	for _, row := range before.Rows {
		k := rowKey(row)
		if used[k] > 0 {
			used[k]--
			continue
		}
		removedRows = append(removedRows, row)
	}
	if len(addedRows) == 0 && len(removedRows) == 0 {
		if before.RowsDigest != "" && after.RowsDigest != "" && before.RowsDigest != after.RowsDigest && !before.Truncated && !after.Truncated {
			r.Reordered = true
		}
		return r
	}
	// pair a removed and an added row that share their first cell, when that cell is unique in each table
	if len(before.Columns) > 1 && uniqueFirstCell(before.Rows) && uniqueFirstCell(after.Rows) {
		addedByKey := map[string]int{}
		for i, row := range addedRows {
			addedByKey[cellText(row[0])] = i
		}
		pairedAdded := map[int]bool{}
		var stillRemoved [][]any
		for _, old := range removedRows {
			i, ok := addedByKey[cellText(old[0])]
			if !ok {
				stillRemoved = append(stillRemoved, old)
				continue
			}
			pairedAdded[i] = true
			ch := RowChange{Key: cellText(old[0])}
			for c := 1; c < len(old) && c < len(addedRows[i]); c++ {
				if rowKey([]any{old[c]}) != rowKey([]any{addedRows[i][c]}) {
					ch.Cells = append(ch.Cells, CellChange{Column: before.Columns[c].Header, Before: old[c], After: addedRows[i][c]})
				}
			}
			r.ChangedTotal++
			if len(r.Changed) < diffListMax {
				r.Changed = append(r.Changed, ch)
			}
		}
		var stillAdded [][]any
		for i, row := range addedRows {
			if !pairedAdded[i] {
				stillAdded = append(stillAdded, row)
			}
		}
		addedRows, removedRows = stillAdded, stillRemoved
	}
	r.AddedTotal, r.RemovedTotal = len(addedRows), len(removedRows)
	for i := 0; i < len(addedRows) && i < diffListMax; i++ {
		r.Added = append(r.Added, addedRows[i])
	}
	for i := 0; i < len(removedRows) && i < diffListMax; i++ {
		r.Removed = append(r.Removed, removedRows[i])
	}
	return r
}

func uniqueFirstCell(rows [][]any) bool {
	seen := map[string]bool{}
	for _, row := range rows {
		if len(row) == 0 {
			return false
		}
		k := cellText(row[0])
		if k == "" || seen[k] {
			return false
		}
		seen[k] = true
	}
	return true
}

func compareEvidence(before, after []EvidenceRef) EvidenceDiff {
	d := EvidenceDiff{Added: []EvidenceRef{}, Removed: []EvidenceRef{}, Replaced: []DVReplaced{}}
	versions := func(refs []EvidenceRef) map[string]map[string]EvidenceRef {
		out := map[string]map[string]EvidenceRef{}
		for _, r := range refs {
			if out[r.EvidenceID] == nil {
				out[r.EvidenceID] = map[string]EvidenceRef{}
			}
			out[r.EvidenceID][r.VersionID] = r
		}
		return out
	}
	b, a := versions(before), versions(after)
	ids := map[string]bool{}
	for id := range b {
		ids[id] = true
	}
	for id := range a {
		ids[id] = true
	}
	sorted := make([]string, 0, len(ids))
	for id := range ids {
		sorted = append(sorted, id)
	}
	sort.Strings(sorted)
	for _, id := range sorted {
		bv, av := b[id], a[id]
		switch {
		case len(bv) == 0:
			for _, r := range av {
				d.Added = append(d.Added, r)
			}
		case len(av) == 0:
			for _, r := range bv {
				d.Removed = append(d.Removed, r)
			}
		default:
			if sameVersionSet(bv, av) {
				continue
			}
			var bver, aver, name string
			for v, r := range bv {
				bver = v
				name = r.SourceFile
			}
			for v, r := range av {
				aver = v
				if name == "" {
					name = r.SourceFile
				}
			}
			d.Replaced = append(d.Replaced, DVReplaced{EvidenceID: id, Name: name, BeforeVersion: bver, AfterVersion: aver})
		}
	}
	return d
}

func sameVersionSet(a, b map[string]EvidenceRef) bool {
	if len(a) != len(b) {
		return false
	}
	for v := range a {
		if _, ok := b[v]; !ok {
			return false
		}
	}
	return true
}
