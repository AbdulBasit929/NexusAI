package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// THE CITED ROW, UNDER THE FILE'S OWN COLUMN NAMES (2026-11-02).
//
//	GET /evidence/{evidence_id}/source-row?row=93&row_hash=...   one row of a statement or log file, as the file has it
//
// The evidence viewer used to open at "Row 93" and show the stored record: batch id, file id, record id, a raw payload and a row hash. An analyst reads MSISDN, STRT_TM, BNUMBER, the headings of the
// file they were given. The typed ingest keeps, for every row it understood, the row's own cells under those headings; this route finds the typed row that belongs to the sheet row (its cells must all
// be found in the stored row, so a wrong row is never presented) and returns the cells that are filled, under their headings. The row hash given by the link must be the stored row's own, or nothing is
// returned: a link that points at another version of the file does not show a row of this one. A row the typed ingest did not read (a title row, a header row) is answered "not found", never guessed.
const (
	sourceRowContract = "forensics.source-row/v1"
	sourceRowMax      = 1_000_000
)

// sourceRowTables are the typed tables whose rows keep their source cells; the names are fixed here, never taken from the request.
var sourceRowTables = []struct{ table, key, kind string }{
	{"comm_event", "event_id", "call or message"},
	{"data_session", "session_id", "internet session"},
	{"bank_transaction", "txn_id", "bank transaction"},
	{"app_usage", "usage_id", "app usage"},
	{"plate_sighting", "sighting_id", "plate sighting"},
}

type sourceRowCell struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type sourceRowResponse struct {
	Contract   string          `json:"contract"`
	State      string          `json:"state"` // "found" or "not_found"
	Reason     string          `json:"reason,omitempty"`
	EvidenceID string          `json:"evidence_id"`
	SourceFile string          `json:"source_file,omitempty"`
	Row        int             `json:"row_number"`
	Kind       string          `json:"kind,omitempty"`
	Cells      []sourceRowCell `json:"cells,omitempty"`
}

func evidenceSourceRowHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
			return
		}
		if db == nil {
			writeError(w, http.StatusServiceUnavailable, errors.New("FORENSIC_DATABASE_URL is not configured"))
			return
		}
		evidenceID := strings.TrimSpace(r.PathValue("evidence_id"))
		row, err := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("row")))
		if evidenceID == "" || err != nil || row < 1 || row > sourceRowMax {
			writeError(w, http.StatusBadRequest, errors.New("evidence_id and a row number from 1 are required"))
			return
		}
		scope, err := bindForensicScope(r, r.URL.Query().Get("tenant_id"), r.URL.Query().Get("collection_id"), "", "")
		if err != nil {
			writeScopeError(w, err)
			return
		}
		if scope.CollectionID == "" {
			writeError(w, http.StatusBadRequest, errors.New("collection_id is required"))
			return
		}
		resp, err := loadSourceRow(r.Context(), db, scope.TenantID, scope.CollectionID, evidenceID, row, strings.TrimSpace(r.URL.Query().Get("row_hash")))
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeError(w, http.StatusNotFound, errors.New("evidence item not found"))
				return
			}
			writeError(w, http.StatusInternalServerError, errors.New("the row could not be read"))
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

func loadSourceRow(ctx context.Context, db *pgxpool.Pool, tenantID, collectionID, evidenceID string, row int, rowHash string) (*sourceRowResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	tx, err := db.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // read-only transaction
	if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenantID); err != nil {
		return nil, err
	}
	out := &sourceRowResponse{Contract: sourceRowContract, State: "not_found", EvidenceID: evidenceID, Row: row}
	err = tx.QueryRow(ctx, `SELECT coalesce(nullif(source_file, ''), original_filename, '') FROM forensic.evidence_items WHERE tenant_id = $1 AND collection_id = $2 AND evidence_id::text = $3`,
		tenantID, collectionID, evidenceID).Scan(&out.SourceFile)
	if err != nil {
		return nil, err // pgx.ErrNoRows: no such evidence in this case
	}
	var storedHash, stored string
	err = tx.QueryRow(ctx, `SELECT coalesce(row_hash, ''), (raw_payload - '_xlsx_cells')::text FROM forensic.records WHERE tenant_id = $1 AND evidence_id::text = $2 AND row_number = $3 LIMIT 1`,
		tenantID, evidenceID, row).Scan(&storedHash, &stored)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		out.Reason = "this file holds no stored row " + strconv.Itoa(row)
		return out, nil
	case err != nil:
		return nil, err
	case rowHash != "" && rowHash != storedHash:
		out.Reason = "the link points at a different version of this row than the one stored"
		return out, nil
	}
	for _, source := range sourceRowTables {
		var payload string
		// every filled cell of the typed row must be found in the stored row; the typed row with the most filled cells wins, then the first by position
		query := `SELECT t.raw_payload::text FROM forensic.` + source.table + ` t
WHERE t.tenant_id = $1 AND t.evidence_id::text = $2 AND t.raw_payload IS NOT NULL AND t.raw_payload <> '{}'::jsonb
  AND NOT EXISTS (SELECT 1 FROM jsonb_each_text(t.raw_payload) p WHERE p.value <> '' AND p.value NOT IN (SELECT value FROM jsonb_each_text($3::jsonb)))
ORDER BY (SELECT count(*) FROM jsonb_each_text(t.raw_payload) p WHERE p.value <> '') DESC, t.source_row LIMIT 1`
		if err := tx.QueryRow(ctx, query, tenantID, evidenceID, stored).Scan(&payload); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			return nil, err
		}
		cells, err := sourceRowCells(payload)
		if err != nil || len(cells) == 0 {
			continue
		}
		cells = sourceRowInFileOrder(ctx, tx, tenantID, evidenceID, row, payload, cells)
		out.State, out.Kind, out.Cells = "found", source.kind, cells
		return out, nil
	}
	out.Reason = "this row is not one of the file's records (a title or a heading row, or a row the system did not read)"
	return out, nil
}

// sourceRowInFileOrder puts the cells in the order of the file's own columns. The header row of the sheet (the nearest row above whose cells hold every column name of this row) lists the names in the
// order of the columns; its stored keys end in the column's position. Without such a row the cells stay as they are.
func sourceRowInFileOrder(ctx context.Context, tx pgx.Tx, tenantID, evidenceID string, row int, typedPayload string, cells []sourceRowCell) []sourceRowCell {
	var header string
	err := tx.QueryRow(ctx, `SELECT (r.raw_payload - '_xlsx_cells')::text FROM forensic.records r
WHERE r.tenant_id = $1 AND r.evidence_id::text = $2 AND r.row_number < $3
  AND NOT EXISTS (SELECT 1 FROM jsonb_object_keys($4::jsonb) k WHERE k NOT IN (SELECT value FROM jsonb_each_text(r.raw_payload)))
ORDER BY r.row_number DESC LIMIT 1`, tenantID, evidenceID, row, typedPayload).Scan(&header)
	if err != nil {
		return cells
	}
	var named map[string]any
	if json.Unmarshal([]byte(header), &named) != nil {
		return cells
	}
	position := map[string]int{}
	for key, value := range named {
		name, ok := value.(string)
		if !ok || strings.HasPrefix(key, "_xlsx_") {
			continue
		}
		place := 0 // the first column carries the heading's own text as its key; the others end in their position
		if rest, found := strings.CutPrefix(key, "_extra_column_"); found {
			if n, convErr := strconv.Atoi(rest); convErr == nil {
				place = n
			}
		}
		if _, seen := position[name]; !seen {
			position[name] = place
		}
	}
	ordered := make([]sourceRowCell, len(cells))
	copy(ordered, cells)
	sort.SliceStable(ordered, func(a, b int) bool {
		pa, okA := position[ordered[a].Name]
		pb, okB := position[ordered[b].Name]
		switch {
		case okA && okB:
			return pa < pb
		case okA:
			return true
		}
		return false
	})
	return ordered
}

// sourceRowCells reads the filled cells of a typed row's own payload under their headings.
func sourceRowCells(payload string) ([]sourceRowCell, error) {
	var named map[string]any
	if err := json.Unmarshal([]byte(payload), &named); err != nil {
		return nil, err
	}
	var cells []sourceRowCell
	for _, name := range sourceRowOrder(payload) {
		raw, ok := named[name]
		if !ok || strings.HasPrefix(name, "_") {
			continue
		}
		text := strings.TrimSpace(sourceRowText(raw))
		if text != "" {
			cells = append(cells, sourceRowCell{Name: name, Value: text})
		}
	}
	return cells, nil
}

func sourceRowText(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(x)
	}
	raw, _ := json.Marshal(v)
	return string(raw)
}

// sourceRowOrder returns the keys of a JSON object in the order they were stored (the database keeps them shortest first, which follows the file's columns closely enough to read).
func sourceRowOrder(payload string) []string {
	decoder := json.NewDecoder(strings.NewReader(payload))
	if token, err := decoder.Token(); err != nil || token != json.Delim('{') {
		return nil
	}
	var keys []string
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return keys
		}
		name, ok := token.(string)
		if !ok {
			return keys
		}
		keys = append(keys, name)
		var skip json.RawMessage
		if err := decoder.Decode(&skip); err != nil {
			return keys
		}
	}
	return keys
}
