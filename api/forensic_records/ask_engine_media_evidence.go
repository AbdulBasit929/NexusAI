package main

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	pg "github.com/pganalyze/pg_query_go/v6"
)

// ASK ENGINE V2: THE PLACES BEHIND AN ANSWER ABOUT A VIDEO, A RECORDING OR A PICTURE (docs/data/video-questions-20261112.md, the phase after the closure).
//
// The records behind a figure were spreadsheet rows (ask_engine_evidence.go). A reading a model made from a video frame, a recording or a photo has no spreadsheet row: it has a file, a version and a
// place in it (the second of the video, the box on the frame). This file builds, for a query that reads ONE media view, the readings behind the answer: the same conditions as the answer's own query,
// read again for the evidence of each reading, earliest first, at most 25. Each is attached to the answer as an openable source in the shape the typed-plate search uses (a locator with the second and the
// box, never a plate), so the workspace opens the video at that second with the box drawn. Nothing here can fail an answer: anything it cannot read back ends in "not itemised" with the reason.
//
// What is not done, and said: the figure is not recomputed from the listed readings (the readings are a sample of the answer's, capped), a query that joins two views or builds intermediate steps is not
// itemised, and a query with no condition that names no file lists nothing (it stands for every reading of the case).

// askMediaEvidenceColumns are, for each media view, the columns that say where in the file a reading lies (the first is the one the readings are ordered by).
var askMediaEvidenceColumns = map[string][]string{
	"v_plate_reads":          {"first_seen_seconds", "last_seen_seconds"},
	"v_video_plate_frames":   {"seen_at_seconds", "frame_number", "x", "y", "width", "height"},
	"v_speech_segments":      {"start_seconds", "end_seconds"},
	"v_ocr_regions":          {"seen_at_seconds", "x", "y", "width", "height"},
	"v_ocr_evidence_summary": {},
	"v_video_files":          {},
	"v_audio_files":          {},
}

// askMediaPlace is where in its file a reading lies: the second it was read at (or began at), the second it ended at, the frame and the box on the frame.
type askMediaPlace struct {
	Seconds     *float64       `json:"seconds,omitempty"`
	EndSeconds  *float64       `json:"end_seconds,omitempty"`
	FrameNumber *int64         `json:"frame_number,omitempty"`
	Box         map[string]any `json:"box,omitempty"`
}

type askMediaEvidencePlan struct {
	View  string
	Alias string
	Where string
}

// askMediaEvidencePlanOf reads the plan from a validated query; nil when the query does not read one media view (the caller then takes the ordinary plan).
func askMediaEvidencePlanOf(v *askValidated) *askMediaEvidencePlan {
	if v == nil || v.Tree == nil || len(v.Tree.Stmts) != 1 || v.Recursive {
		return nil
	}
	root := v.Tree.Stmts[0].Stmt.GetSelectStmt()
	if root == nil || root.Op != pg.SetOperation_SETOP_NONE || root.WithClause != nil || len(root.FromClause) != 1 {
		return nil
	}
	rv := root.FromClause[0].GetRangeVar()
	if rv == nil || !askMediaViewSet[rv.Relname] {
		return nil
	}
	plan := &askMediaEvidencePlan{View: rv.Relname}
	if rv.Alias != nil && rv.Alias.Aliasname != "" {
		if !askPlainIdent.MatchString(rv.Alias.Aliasname) {
			return nil
		}
		plan.Alias = rv.Alias.Aliasname
	}
	if root.WhereClause != nil {
		if plan.Where = askDeparseFragment("where", []*pg.Node{root.WhereClause}); plan.Where == "" {
			return nil
		}
	}
	return plan
}

var askMediaFileCell = regexp.MustCompile(`^[^\s/\\]+\.[A-Za-z][A-Za-z0-9]{1,4}$`)

// askMediaFilesNamed are the file names that the answer itself names (a cell that is a file name): an answer that ranks or counts per file has no condition of its own, and its evidence is that of the
// files it names.
func askMediaFilesNamed(result *govSQLResult) []string {
	seen := map[string]bool{}
	var out []string
	if result == nil {
		return nil
	}
	for _, row := range result.Rows {
		for _, cell := range row {
			s, ok := cell.(string)
			if ok && askMediaFileCell.MatchString(s) && !seen[s] && len(out) < 10 {
				seen[s] = true
				out = append(out, s)
			}
		}
	}
	sort.Strings(out)
	return out
}

// rowsSQL is the query that reads the readings behind the answer: the plan's own conditions, restricted to the files the answer names, earliest first.
func (p *askMediaEvidencePlan) rowsSQL(files []string, limit int) (string, bool) {
	var conds []string
	if p.Where != "" {
		conds = append(conds, "("+p.Where+")")
	}
	if len(files) > 0 {
		var literals []string
		for _, f := range files {
			literal, ok := askEvidenceLiteral(f)
			if !ok {
				return "", false
			}
			literals = append(literals, literal)
		}
		conds = append(conds, "original_filename IN ("+strings.Join(literals, ", ")+")")
	}
	if len(conds) == 0 {
		return "", false
	}
	cols := append([]string{"evidence_id::text AS evidence_id", "original_filename"}, askMediaEvidenceColumns[p.View]...)
	order := "original_filename, evidence_id"
	if columns := askMediaEvidenceColumns[p.View]; len(columns) > 0 {
		order = columns[0] + " NULLS LAST, original_filename, evidence_id"
	}
	from := p.View
	if p.Alias != "" {
		from += " AS " + p.Alias
	}
	return fmt.Sprintf("SELECT %s FROM %s WHERE %s ORDER BY %s LIMIT %d", strings.Join(cols, ", "), from, strings.Join(conds, " AND "), order, limit), true
}

// askMediaRowsToEvidence turns the rows that the evidence query returned into the evidence: a place for each reading, the files they lie in.
func askMediaRowsToEvidence(plan *askMediaEvidencePlan, columns []string, rows [][]any, versions map[string]string, capped bool) *askEvidence {
	ev := &askEvidence{State: "itemised", Kind: "rows", View: plan.View, Reconciled: "not_checked", Truncated: capped,
		Scope:   "the readings behind the answer, earliest first",
		Say:     "readings a model made from photos, video frames or recordings: each opens its source at the second it was read",
		Columns: []string{"File", "Second"}}
	index := map[string]int{}
	for i, c := range columns {
		index[c] = i
	}
	num := func(row []any, name string) *float64 {
		i, ok := index[name]
		if !ok || i >= len(row) || row[i] == nil {
			return nil
		}
		f, ok := askEvidenceNumber(row[i])
		if !ok {
			return nil
		}
		return &f
	}
	files := map[string]*askEvidenceFile{}
	var order []string
	for _, row := range rows {
		evidenceID, _ := row[index["evidence_id"]].(string)
		file, _ := row[index["original_filename"]].(string)
		if evidenceID == "" {
			continue
		}
		place := &askMediaPlace{}
		for _, name := range []string{"first_seen_seconds", "seen_at_seconds", "start_seconds"} {
			if f := num(row, name); f != nil && place.Seconds == nil {
				place.Seconds = f
			}
		}
		for _, name := range []string{"last_seen_seconds", "end_seconds"} {
			if f := num(row, name); f != nil && place.EndSeconds == nil {
				place.EndSeconds = f
			}
		}
		if f := num(row, "frame_number"); f != nil {
			n := int64(*f)
			place.FrameNumber = &n
		}
		if x, y, w, h := num(row, "x"), num(row, "y"), num(row, "width"), num(row, "height"); x != nil && y != nil && w != nil && h != nil {
			place.Box = map[string]any{"x": *x, "y": *y, "width": *w, "height": *h}
		}
		second := ""
		if place.Seconds != nil {
			second = fmt.Sprintf("%.2f s", *place.Seconds)
		}
		key := evidenceID
		if second != "" {
			key += "@" + second
		}
		ev.Rows = append(ev.Rows, askEvidenceRow{Key: key, EvidenceID: evidenceID, SourceFile: file, VersionID: versions[evidenceID], Preview: strings.TrimSpace(file + " " + second),
			Cells: []string{file, second}, Media: place})
		if f, ok := files[evidenceID]; ok {
			f.Rows++
		} else {
			files[evidenceID] = &askEvidenceFile{EvidenceID: evidenceID, SourceFile: file, Rows: 1}
			order = append(order, evidenceID)
		}
	}
	for _, id := range order {
		ev.Files = append(ev.Files, *files[id])
	}
	ev.Total = len(ev.Rows)
	return ev
}

// askCollectMediaEvidence reads the readings behind an answer that reads one media view. nil when nothing can be listed (the caller then says why).
func askCollectMediaEvidence(ctx context.Context, db *pgxpool.Pool, tenantID, collectionID string, cat *askCatalogue, plan *askMediaEvidencePlan, result *govSQLResult, limits askLimits) *askEvidence {
	sql, ok := plan.rowsSQL(askMediaFilesNamed(result), askEvidenceRowCap)
	if !ok {
		return askEvidenceNone("the answer stands for every reading of the case and names no file, so no reading is listed; ask about one video to see its readings")
	}
	checked, rej := askValidate(sql, cat, []string{plan.View})
	if rej != nil {
		return askEvidenceNone("the readings behind the answer could not be read back")
	}
	got, err := askRun(ctx, db, tenantID, collectionID, checked.SQL, nil, limits, askEvidenceRowCap, true)
	if err != nil || got == nil || len(got.Rows) == 0 {
		return askEvidenceNone("no reading was found behind the answer")
	}
	ids := map[string]bool{}
	for _, row := range got.Rows {
		if id, _ := row[0].(string); id != "" {
			ids[id] = true
		}
	}
	versions := map[string]string{}
	if tx, err := db.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly}); err == nil {
		defer tx.Rollback(ctx) //nolint:errcheck // read-only transaction
		if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenantID); err == nil {
			var list []string
			for id := range ids {
				list = append(list, id)
			}
			if rows, err := tx.Query(ctx, `SELECT evidence_id::text, coalesce(current_version_id::text, '') FROM forensic.evidence_items WHERE tenant_id = $1 AND collection_id = $2 AND evidence_id::text = ANY($3)`, tenantID, collectionID, list); err == nil {
				for rows.Next() {
					var id, version string
					if rows.Scan(&id, &version) == nil {
						versions[id] = version
					}
				}
				rows.Close()
			}
		}
	}
	return askMediaRowsToEvidence(plan, got.Columns, got.Rows, versions, len(got.Rows) >= askEvidenceRowCap)
}

// askMediaProvenance is the openable source of one reading, in the shape of the typed-plate search: the evidence, its version, the file and a locator with the second and the box. A reading is a model
// candidate, never a camera sighting, and the locator holds no text that was read.
func askMediaProvenance(r askEvidenceRow) map[string]any {
	locator := map[string]any{"source_file": r.SourceFile}
	if r.Media != nil {
		if r.Media.Seconds != nil {
			locator["timestamp_seconds"] = *r.Media.Seconds
		}
		if r.Media.EndSeconds != nil {
			locator["end_seconds"] = *r.Media.EndSeconds
		}
		if r.Media.FrameNumber != nil {
			locator["frame_number"] = *r.Media.FrameNumber
		}
		if r.Media.Box != nil {
			locator["bbox"] = r.Media.Box
		}
	}
	return map[string]any{
		"source": "analyst_views_sql", "kind": "ask_engine_v2_media_evidence", "citation_id": "media-" + r.Key, "evidence_id": r.EvidenceID, "version_id": r.VersionID, "source_file": r.SourceFile,
		"preview": r.Preview, "source_truth_state": "derived_model_observation", "proof_role": "model reading, check by eye", "locator": locator,
	}
}
