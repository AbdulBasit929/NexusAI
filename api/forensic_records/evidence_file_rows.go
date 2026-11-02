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

// THE FILE AS THE ANALYST WAS GIVEN IT, A PAGE AT A TIME (2026-11-02).
//
//	GET /evidence/{evidence_id}/file-rows?from=93&limit=50   the stored rows from row 93, under the file's own column names and in the file's own column order
//
// The evidence viewer listed 25 canonical records (primary target, secondary target, timestamp) of a file that holds hundreds or thousands of rows, so a row an answer cited was usually not on the page.
// This route pages through the rows of one file by their row number in the file: the headings are the file's (the header list the ingest kept for a CSV; the workbook's column letters for a
// spreadsheet), the cells are the values as the file has them, and a row the system did not read stays out. Subscriber rows are projected the way every other record surface projects them: a name is
// never returned, a national identity number only in its masked form, and a cell the projection removed says so instead of showing as empty.
const (
	fileRowsContract     = "forensics.file-rows/v1"
	fileRowsDefaultLimit = 50
	fileRowsMaxLimit     = 200
	fileRowsWithheld     = "withheld"
)

type fileRowsColumn struct {
	Name   string `json:"name"`
	Letter string `json:"letter,omitempty"` // the column of the workbook (H), when the file is a spreadsheet
}

type fileRowsRow struct {
	RowNumber int      `json:"row_number"`
	Cells     []string `json:"cells"`
}

type fileRowsResponse struct {
	Contract      string           `json:"contract"`
	EvidenceID    string           `json:"evidence_id"`
	SourceFile    string           `json:"source_file,omitempty"`
	Columns       []fileRowsColumn `json:"columns"`
	Rows          []fileRowsRow    `json:"rows"`
	From          int              `json:"from"`
	Limit         int              `json:"limit"`
	Total         int              `json:"total"`
	FirstRow      int              `json:"first_row"`
	LastRow       int              `json:"last_row"`
	DuplicateRows int              `json:"duplicate_rows,omitempty"`
	RejectedRows  int              `json:"rejected_rows,omitempty"`
	RowLabel      string           `json:"row_label,omitempty"` // "Record" when the numbers are the typed rows' own order, not rows of a sheet
}

func evidenceFileRowsHandler(db *pgxpool.Pool) http.HandlerFunc {
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
		from := 1
		if text := strings.TrimSpace(r.URL.Query().Get("from")); text != "" {
			n, err := strconv.Atoi(text)
			if err != nil || n < 1 || n > sourceRowMax {
				writeError(w, http.StatusBadRequest, errors.New("from must be a row number from 1"))
				return
			}
			from = n
		}
		limit := fileRowsDefaultLimit
		if text := strings.TrimSpace(r.URL.Query().Get("limit")); text != "" {
			n, err := strconv.Atoi(text)
			if err != nil || n < 1 {
				writeError(w, http.StatusBadRequest, errors.New("limit must be a number from 1"))
				return
			}
			limit = min(n, fileRowsMaxLimit)
		}
		if evidenceID == "" {
			writeError(w, http.StatusBadRequest, errors.New("evidence_id is required"))
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
		resp, err := loadFileRows(r.Context(), db, scope.TenantID, scope.CollectionID, evidenceID, from, limit)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeError(w, http.StatusNotFound, errors.New("evidence item not found"))
				return
			}
			writeError(w, http.StatusInternalServerError, errors.New("the rows of the file could not be read"))
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

func loadFileRows(ctx context.Context, db *pgxpool.Pool, tenantID, collectionID, evidenceID string, from, limit int) (*fileRowsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	tx, err := db.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // read-only transaction
	if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenantID); err != nil {
		return nil, err
	}
	out := &fileRowsResponse{Contract: fileRowsContract, EvidenceID: evidenceID, Columns: []fileRowsColumn{}, Rows: []fileRowsRow{}, Limit: limit}
	var headersJSON, duplicates, rejected string
	err = tx.QueryRow(ctx, `SELECT coalesce(nullif(source_file, ''), original_filename, ''), coalesce(metadata -> 'headers', 'null'::jsonb)::text,
       coalesce(nullif(metadata -> 'structured_summary' ->> 'duplicate_rows', ''), '0'), coalesce(nullif(metadata -> 'structured_summary' ->> 'rejected_rows', ''), '0')
FROM forensic.evidence_items WHERE tenant_id = $1 AND collection_id = $2 AND evidence_id::text = $3`,
		tenantID, collectionID, evidenceID).Scan(&out.SourceFile, &headersJSON, &duplicates, &rejected)
	if err != nil {
		return nil, err // pgx.ErrNoRows: no such evidence in this case
	}
	out.DuplicateRows, _ = strconv.Atoi(duplicates)
	out.RejectedRows, _ = strconv.Atoi(rejected)
	if err := tx.QueryRow(ctx, `SELECT count(*), coalesce(min(row_number), 0), coalesce(max(row_number), 0) FROM forensic.records WHERE tenant_id = $1 AND evidence_id::text = $2`,
		tenantID, evidenceID).Scan(&out.Total, &out.FirstRow, &out.LastRow); err != nil {
		return nil, err
	}
	rowQuery := `SELECT row_number, coalesce(record_type, ''), raw_payload::text, coalesce(metadata, '{}'::jsonb)::text, coalesce(secondary_target, '')
FROM forensic.records WHERE tenant_id = $1 AND evidence_id::text = $2 AND row_number >= $3 ORDER BY row_number LIMIT $4`
	if out.Total == 0 {
		// A workbook in the old XML format and a text export are typed straight into the typed tables, which keep each row's cells under the file's headings and its place among the file's rows.
		// Those places are the typed rows' own order, not a sheet row, so the page calls them records.
		for _, source := range sourceRowTables {
			if err := tx.QueryRow(ctx, `SELECT count(*), coalesce(min(source_row), 0), coalesce(max(source_row), 0) FROM forensic.`+source.table+` WHERE tenant_id = $1 AND evidence_id::text = $2 AND raw_payload IS NOT NULL`,
				tenantID, evidenceID).Scan(&out.Total, &out.FirstRow, &out.LastRow); err != nil {
				return nil, err
			}
			if out.Total > 0 {
				out.RowLabel = "Record"
				rowQuery = `SELECT source_row, '', raw_payload::text, '{}', '' FROM forensic.` + source.table + `
WHERE tenant_id = $1 AND evidence_id::text = $2 AND raw_payload IS NOT NULL AND source_row >= $3 ORDER BY source_row, ` + source.key + ` LIMIT $4`
				break
			}
		}
	}
	if out.Total == 0 {
		return out, nil
	}
	// a start before the first row begins at it; a start past the last row shows the last page
	switch {
	case from < out.FirstRow:
		from = out.FirstRow
	case from > out.LastRow:
		from = max(out.FirstRow, out.LastRow-limit+1)
	}
	out.From = from
	rows, err := tx.Query(ctx, rowQuery, tenantID, evidenceID, from, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var stored []fileRowStored
	for rows.Next() {
		var item fileRowStored
		var payload, metadata string
		if err := rows.Scan(&item.RowNumber, &item.RecordType, &payload, &metadata, &item.SecondaryTarget); err != nil {
			return nil, err
		}
		if json.Unmarshal([]byte(payload), &item.Payload) != nil {
			continue // a row whose payload is not an object cannot be shown by column
		}
		item.Order = sourceRowOrder(payload)
		_ = json.Unmarshal([]byte(metadata), &item.Metadata)
		stored = append(stored, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out.Columns, out.Rows = fileRowsTable(stored, fileRowsHeaders(headersJSON))
	return out, nil
}

// fileRowStored is one stored row before it is shown under the file's columns.
type fileRowStored struct {
	RowNumber       int
	RecordType      string
	SecondaryTarget string
	Payload         map[string]any
	Metadata        map[string]any
	Order           []string // the keys of the payload as the database kept them
}

// fileRowsHeaders reads the header list the ingest kept for a CSV: the file's column names in the file's order. A spreadsheet keeps none (null), and its column letters give the order instead.
func fileRowsHeaders(text string) []string {
	var headers []string
	if json.Unmarshal([]byte(text), &headers) != nil {
		return nil
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(headers))
	for _, name := range headers {
		if name = strings.TrimSpace(name); name != "" && !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	return out
}

// fileRowsShown says whether a key of a stored payload is a column of the file (not a marker the ingest added).
func fileRowsShown(key string) bool {
	return key != "" && !strings.HasPrefix(key, "_") && !strings.HasSuffix(key, "__xlsx_raw_value")
}

// fileRowsTable lays stored rows out under the columns of the file. The subscriber projection is applied first, and what it removed is marked, never left looking empty.
func fileRowsTable(stored []fileRowStored, headers []string) ([]fileRowsColumn, []fileRowsRow) {
	before := make([][]string, len(stored))
	projected := make([]map[string]any, len(stored))
	for i, item := range stored {
		for key := range item.Payload {
			before[i] = append(before[i], key)
		}
		projected[i] = map[string]any{"record_type": item.RecordType, "raw_payload": item.Payload, "metadata": item.Metadata, "secondary_target": item.SecondaryTarget}
	}
	redactSubscriberCanonicalRows(projected) // changes each payload in place
	removed := make([]map[string]bool, len(stored))
	for i, item := range stored {
		removed[i] = map[string]bool{}
		for _, key := range before[i] {
			if _, kept := item.Payload[key]; !kept {
				removed[i][key] = true
			}
		}
	}
	letters := map[string]int{} // column name -> position in the workbook, from the cell addresses of the rows on this page
	labels := map[string]string{}
	var first []string // the keys in the order the database kept them, first seen across the page
	seen := map[string]bool{}
	for i, item := range stored {
		for name, address := range sourceRowAddresses(fileRowsCellsText(item.Payload)) {
			if column := sourceRowColumn(address); column > 0 {
				if known, ok := letters[name]; !ok || column < known {
					letters[name] = column
					labels[name] = fileRowsLetters(column)
				}
			}
		}
		keys := append([]string{}, item.Order...)
		for key := range removed[i] {
			keys = append(keys, key)
		}
		for _, key := range keys {
			if fileRowsShown(key) && !seen[key] {
				seen[key] = true
				first = append(first, key)
			}
		}
	}
	var names []string
	switch {
	case len(headers) > 0:
		names = append(names, headers...)
		known := map[string]bool{}
		for _, name := range headers {
			known[name] = true
		}
		for _, key := range first { // a key the header list does not have still belongs to the file
			if !known[key] {
				names = append(names, key)
			}
		}
	default:
		names = first
		sort.SliceStable(names, func(a, b int) bool {
			pa, okA := letters[names[a]]
			pb, okB := letters[names[b]]
			switch {
			case okA && okB:
				return pa < pb
			case okA:
				return true
			}
			return false
		})
	}
	columns := make([]fileRowsColumn, len(names))
	for i, name := range names {
		columns[i] = fileRowsColumn{Name: name, Letter: labels[name]}
	}
	rows := make([]fileRowsRow, 0, len(stored))
	for i, item := range stored {
		cells := make([]string, len(names))
		for c, name := range names {
			switch {
			case removed[i][name]:
				cells[c] = fileRowsWithheld
			default:
				cells[c] = strings.TrimSpace(sourceRowText(item.Payload[name]))
			}
		}
		rows = append(rows, fileRowsRow{RowNumber: item.RowNumber, Cells: cells})
	}
	return columns, rows
}

// fileRowsCellsText returns the workbook markers of a stored payload as the text sourceRowAddresses reads.
func fileRowsCellsText(payload map[string]any) string {
	raw, ok := payload["_xlsx_cells"]
	if !ok {
		return ""
	}
	text, err := json.Marshal(raw)
	if err != nil {
		return ""
	}
	return string(text)
}

// fileRowsLetters is the name of a workbook column from its number (1 is A, 27 is AA).
func fileRowsLetters(column int) string {
	var name []byte
	for column > 0 {
		column--
		name = append([]byte{byte('A' + column%26)}, name...)
		column /= 26
	}
	return string(name)
}
