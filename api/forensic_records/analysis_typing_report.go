package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// WHAT THE TYPED TABLES HOLD FOR A CASE, AND WHERE AN ANSWER COULD MISLEAD (roadmap 7.8.3 rows 6 and 10; written 2026-10-21; the interface form of tools/dev/typing_report.py).
//
//	GET /analysis/typing-report?collection_id=...
//
// Counts and shares only, never a value, a number of the case, a name or a question. It says how many of the case's call and internet files are in the typed tables, how many call rows
// are exact repeats left out of the views, how the call rows split by event type and direction, which columns are filled, on what basis the times were read, what the internet
// records state (a volume, a public address), and how many evidence files the case holds by modality; and it says in plain words what to read before trusting an answer (findings).
// The rows are the tenant's own (row-level security, the tenant set for the read) and only the collection asked for.
const (
	typingReportContract = "forensics.typing-report/v1"

	typingOtherShare   = 0.10 // event types read as "other"
	typingRepeatShare  = 0.05 // exact repeats left out of the views
	typingUnknownShare = 0.20 // rows with no known direction
)

type typingFinding struct {
	Code string `json:"code"`
	Text string `json:"text"`
}

type typingShare struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

type typingLayoutRow struct {
	Layout      string `json:"layout"`
	Kind        string `json:"kind,omitempty"`
	TimeBasis   string `json:"time_basis"`
	Rows        int64  `json:"rows"`
	Repeats     int64  `json:"repeats"`
	VolumeRows  int64  `json:"volume_rows,omitempty"`
	PublicRows  int64  `json:"public_address_rows,omitempty"`
	ServerRows  int64  `json:"server_port_rows,omitempty"`
	Tower       bool   `json:"tower_dump,omitempty"`
}

type typingEvidenceRow struct {
	Modality string `json:"modality"`
	Files    int64  `json:"files"`
	Failed   int64  `json:"failed"`
}

type typingReport struct {
	Contract   string `json:"contract"`
	Available  bool   `json:"available"`
	Collection string `json:"collection_id"`
	Files      struct {
		Counted int64 `json:"counted"`
		Typed   int64 `json:"typed"`
	} `json:"files"`
	Calls struct {
		Typed      int64              `json:"typed"`
		Repeats    int64              `json:"repeats"`
		Effective  int64              `json:"effective"`
		Layouts    []typingLayoutRow  `json:"layouts"`
		EventTypes []typingShare      `json:"event_types"`
		Directions []typingShare      `json:"directions"`
		Populated  map[string]float64 `json:"populated"`
	} `json:"calls"`
	Sessions struct {
		Typed   int64             `json:"typed"`
		Repeats int64             `json:"repeats"`
		Layouts []typingLayoutRow `json:"layouts"`
	} `json:"sessions"`
	Usage struct {
		Typed int64 `json:"typed"`
	} `json:"usage"`
	Evidence []typingEvidenceRow `json:"evidence"`
	Findings []typingFinding     `json:"findings"`
}

func typingTimeBasisDeclared(basis string) bool {
	switch basis {
	case "declared", "explicit_offset", "confirmed_by_analyst":
		return true
	}
	return false
}

func typingPercent(part, whole int64) string {
	if whole == 0 {
		return "0%"
	}
	return fmt.Sprintf("%.0f%%", 100*float64(part)/float64(whole))
}

func typingReportHandler(db *pgxpool.Pool) http.HandlerFunc {
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
		if scope.CollectionID == "" {
			writeError(w, http.StatusBadRequest, errors.New("collection_id is required"))
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		out, err := readTypingReport(ctx, db, scope.TenantID, scope.CollectionID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, errors.New("the typing report could not be read"))
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func readTypingReport(ctx context.Context, db *pgxpool.Pool, tenantID, collectionID string) (*typingReport, error) {
	out := &typingReport{Contract: typingReportContract, Available: true, Collection: collectionID}
	out.Calls.Layouts, out.Calls.EventTypes, out.Calls.Directions = []typingLayoutRow{}, []typingShare{}, []typingShare{}
	out.Calls.Populated = map[string]float64{}
	out.Sessions.Layouts, out.Evidence, out.Findings = []typingLayoutRow{}, []typingEvidenceRow{}, []typingFinding{}
	tx, err := db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // a read only
	if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenantID); err != nil {
		return nil, err
	}

	// the files the engine's guard counts, and how many of them have typed rows
	if err := tx.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE e.detected_type IN ('cdr', 'ipdr') AND coalesce(e.processing_status, '') <> 'failed'),
		       count(*) FILTER (WHERE e.detected_type IN ('cdr', 'ipdr') AND coalesce(e.processing_status, '') <> 'failed' AND (
		         EXISTS (SELECT 1 FROM forensic.comm_event c WHERE c.tenant_id = e.tenant_id AND c.evidence_id = e.evidence_id)
		         OR EXISTS (SELECT 1 FROM forensic.data_session s WHERE s.tenant_id = e.tenant_id AND s.evidence_id = e.evidence_id)))
		FROM forensic.evidence_items e WHERE e.tenant_id = $1 AND e.collection_id = $2 AND e.modality = 'structured_records'`, tenantID, collectionID).Scan(&out.Files.Counted, &out.Files.Typed); err != nil {
		return nil, err
	}

	// call records by layout and time basis
	rows, err := tx.Query(ctx, `
		SELECT layout_id || ' v' || layout_version, dataset_kind, time_basis, count(*), count(*) FILTER (WHERE 'duplicate_row' = ANY (quality_flags))
		FROM forensic.comm_event WHERE tenant_id = $1 AND collection_id = $2 GROUP BY 1, 2, 3 ORDER BY 1, 3`, tenantID, collectionID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var row typingLayoutRow
		if err := rows.Scan(&row.Layout, &row.Kind, &row.TimeBasis, &row.Rows, &row.Repeats); err != nil {
			rows.Close()
			return nil, err
		}
		row.Tower = row.Kind == "tower_dump"
		out.Calls.Layouts = append(out.Calls.Layouts, row)
		if !row.Tower { // the views leave tower dumps out
			out.Calls.Typed += row.Rows
			out.Calls.Repeats += row.Repeats
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out.Calls.Effective = out.Calls.Typed - out.Calls.Repeats

	if out.Calls.Effective > 0 {
		shares := func(column string) ([]typingShare, error) {
			list := []typingShare{}
			r, err := tx.Query(ctx, fmt.Sprintf(`SELECT %s, count(*) FROM forensic.comm_event WHERE tenant_id = $1 AND collection_id = $2 AND dataset_kind = 'target_cdr'
				AND NOT ('duplicate_row' = ANY (quality_flags)) GROUP BY 1 ORDER BY 2 DESC, 1`, column), tenantID, collectionID)
			if err != nil {
				return nil, err
			}
			defer r.Close()
			for r.Next() {
				var s typingShare
				if err := r.Scan(&s.Name, &s.Count); err != nil {
					return nil, err
				}
				list = append(list, s)
			}
			return list, r.Err()
		}
		if out.Calls.EventTypes, err = shares("event_type"); err != nil {
			return nil, err
		}
		if out.Calls.Directions, err = shares("direction"); err != nil {
			return nil, err
		}
		var imei, imsi, cell, position int64
		if err := tx.QueryRow(ctx, `SELECT count(imei), count(imsi), count(cell_key), count(lat) FROM forensic.comm_event WHERE tenant_id = $1 AND collection_id = $2
			AND dataset_kind = 'target_cdr' AND NOT ('duplicate_row' = ANY (quality_flags))`, tenantID, collectionID).Scan(&imei, &imsi, &cell, &position); err != nil {
			return nil, err
		}
		for name, n := range map[string]int64{"imei": imei, "imsi": imsi, "cell": cell, "position": position} {
			out.Calls.Populated[name] = float64(n) / float64(out.Calls.Effective)
		}
	}

	// internet records by layout
	srows, err := tx.Query(ctx, `
		SELECT layout_id || ' v' || layout_version, time_basis, count(*), count(*) FILTER (WHERE 'duplicate_row' = ANY (quality_flags)),
		       count(*) FILTER (WHERE bytes_up IS NOT NULL OR bytes_down IS NOT NULL), count(*) FILTER (WHERE public_ip IS NOT NULL), count(*) FILTER (WHERE server_port IS NOT NULL)
		FROM forensic.data_session WHERE tenant_id = $1 AND collection_id = $2 GROUP BY 1, 2 ORDER BY 1, 2`, tenantID, collectionID)
	if err != nil {
		return nil, err
	}
	for srows.Next() {
		var row typingLayoutRow
		if err := srows.Scan(&row.Layout, &row.TimeBasis, &row.Rows, &row.Repeats, &row.VolumeRows, &row.PublicRows, &row.ServerRows); err != nil {
			srows.Close()
			return nil, err
		}
		out.Sessions.Layouts = append(out.Sessions.Layouts, row)
		out.Sessions.Typed += row.Rows
		out.Sessions.Repeats += row.Repeats
	}
	srows.Close()
	if err := srows.Err(); err != nil {
		return nil, err
	}
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM forensic.app_usage WHERE tenant_id = $1 AND collection_id = $2`, tenantID, collectionID).Scan(&out.Usage.Typed); err != nil {
		return nil, err
	}
	if err := typingEvidence(ctx, tx, tenantID, collectionID, out); err != nil {
		return nil, err
	}
	out.Findings = typingFindings(out)
	return out, nil
}

func typingEvidence(ctx context.Context, tx pgx.Tx, tenantID, collectionID string, out *typingReport) error {
	rows, err := tx.Query(ctx, `SELECT modality, count(*), count(*) FILTER (WHERE processing_status = 'failed') FROM forensic.evidence_items
		WHERE tenant_id = $1 AND collection_id = $2 GROUP BY 1 ORDER BY 2 DESC, 1`, tenantID, collectionID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var row typingEvidenceRow
		if err := rows.Scan(&row.Modality, &row.Files, &row.Failed); err != nil {
			return err
		}
		out.Evidence = append(out.Evidence, row)
	}
	return rows.Err()
}

// typingFindings says, in plain words and from the counts alone, what to read before trusting an answer over this case.
func typingFindings(r *typingReport) []typingFinding {
	var out []typingFinding
	add := func(code, text string) { out = append(out, typingFinding{Code: code, Text: text}) }
	if r.Files.Counted > 0 && r.Files.Typed < r.Files.Counted {
		add("files_not_typed", fmt.Sprintf("Only %d of %d call and internet files are in the typed records, so the engine does not answer questions about this case from them.", r.Files.Typed, r.Files.Counted))
	}
	assumed := map[string]int64{}
	for _, l := range r.Calls.Layouts {
		if !l.Tower && !typingTimeBasisDeclared(l.TimeBasis) {
			assumed[l.TimeBasis] += l.Rows
		}
	}
	for _, l := range r.Sessions.Layouts {
		if !typingTimeBasisDeclared(l.TimeBasis) {
			assumed[l.TimeBasis] += l.Rows
		}
	}
	keys := make([]string, 0, len(assumed))
	for k := range assumed {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		add("time_assumed", fmt.Sprintf("%d rows were read with a time zone that was assumed (%s), not declared by the file: times on them depend on that choice.", assumed[k], strings.ReplaceAll(k, "_", " ")))
	}
	if r.Calls.Typed > 0 && float64(r.Calls.Repeats)/float64(r.Calls.Typed) > typingRepeatShare {
		add("repeats", fmt.Sprintf("%s of the call rows (%d) are exact repeats of another row and are left out of counts, so a count here is lower than the number of lines in the files.", typingPercent(r.Calls.Repeats, r.Calls.Typed), r.Calls.Repeats))
	}
	var other, unknown int64
	for _, s := range r.Calls.EventTypes {
		if s.Name == "other" {
			other = s.Count
		}
	}
	for _, s := range r.Calls.Directions {
		if s.Name == "unknown" || s.Name == "none" {
			unknown += s.Count
		}
	}
	if r.Calls.Effective > 0 && float64(other)/float64(r.Calls.Effective) > typingOtherShare {
		add("other_events", fmt.Sprintf("%s of the call rows are of a kind the layout does not name (event type other): a question about calls or messages only leaves them out.", typingPercent(other, r.Calls.Effective)))
	}
	if r.Calls.Effective > 0 && float64(unknown)/float64(r.Calls.Effective) > typingUnknownShare {
		add("unknown_direction", fmt.Sprintf("%s of the call rows have no known direction: counts of incoming and outgoing events are partial.", typingPercent(unknown, r.Calls.Effective)))
	}
	for _, l := range r.Sessions.Layouts {
		if l.Rows > 0 && l.VolumeRows == 0 {
			add("no_volume", fmt.Sprintf("The internet records read with %s state no byte count per direction: a question about volume is answered by the older paths, not the engine.", l.Layout))
		}
		if l.Rows > 0 && l.PublicRows == 0 {
			add("no_public_address", fmt.Sprintf("The internet records read with %s state no public address: who held an address at a time cannot be answered from them.", l.Layout))
		}
	}
	return out
}
