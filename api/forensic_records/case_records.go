package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The case record (migration 021, decision D30). A case is still a collection_id on the evidence rows; the record says what analysts call it and whether it is open.
// A case with no record is reported as open with no display name and recorded=false: nothing is backfilled and a creator is never guessed (created_by stays null).
// No membership, owner, classification or priority exists here; those wait for per-user identity.

const (
	caseNameMaxRunes = 120
	caseAuditAction  = "case.record.updated"
	// the bounds of the table's check (migration 032) and of the hub_threshold parameter of the Ask engine
	caseHubThresholdMin = 2
	caseHubThresholdMax = 1000
)

var (
	errCaseNotFound = errors.New("no such case: the collection has no evidence and no case record")
	caseStatuses    = map[string]bool{"open": true, "on_hold": true, "closed": true}
)

type caseRecord struct {
	CaseID        string     `json:"case_id"`
	DisplayName   *string    `json:"display_name"`
	Status        string     `json:"status"`
	HubThreshold  *int       `json:"hub_threshold"`
	CreatedBy     *string    `json:"created_by"`
	CreatedAt     *time.Time `json:"created_at"`
	UpdatedBy     *string    `json:"updated_by"`
	UpdatedAt     *time.Time `json:"updated_at"`
	Recorded      bool       `json:"recorded"`
	EvidenceCount int64      `json:"evidence_count"`
}

// casePatch is what a client may change. A field that is absent is left alone; display_name null (or blank) clears the name.
type casePatch struct {
	DisplayName    *string
	ClearName      bool
	HasDisplayName bool
	Status         *string
	// HubThreshold is the case's threshold for "numbers and services that everyone contacts" (migration 032); nil with HasHubThreshold clears it.
	HubThreshold    *int
	HasHubThreshold bool
}

// parseCasePatch reads the JSON body of PATCH /cases/{id} and validates it with the same rules as the table's constraints, so a client gets a sentence and not a database error.
func parseCasePatch(body io.Reader) (casePatch, error) {
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(io.LimitReader(body, 1<<16)).Decode(&raw); err != nil {
		return casePatch{}, fmt.Errorf("the request body must be a JSON object: %w", err)
	}
	var patch casePatch
	for key := range raw {
		if key != "display_name" && key != "status" && key != "hub_threshold" {
			return casePatch{}, fmt.Errorf("unknown field %q: only display_name, status and hub_threshold can be changed", key)
		}
	}
	if v, ok := raw["display_name"]; ok {
		patch.HasDisplayName = true
		if string(v) == "null" {
			patch.ClearName = true
		} else {
			var name string
			if err := json.Unmarshal(v, &name); err != nil {
				return casePatch{}, errors.New("display_name must be text or null")
			}
			name = strings.TrimSpace(name)
			switch {
			case name == "":
				patch.ClearName = true
			case utf8.RuneCountInString(name) > caseNameMaxRunes:
				return casePatch{}, fmt.Errorf("display_name is longer than %d characters", caseNameMaxRunes)
			case strings.IndexFunc(name, unicode.IsControl) >= 0:
				return casePatch{}, errors.New("display_name must not contain control characters or line breaks")
			default:
				patch.DisplayName = &name
			}
		}
	}
	if v, ok := raw["status"]; ok {
		var status string
		if err := json.Unmarshal(v, &status); err != nil || !caseStatuses[status] {
			return casePatch{}, errors.New("status must be one of open, on_hold, closed")
		}
		patch.Status = &status
	}
	if v, ok := raw["hub_threshold"]; ok {
		patch.HasHubThreshold = true
		if string(v) != "null" {
			var n int
			if err := json.Unmarshal(v, &n); err != nil || n < caseHubThresholdMin || n > caseHubThresholdMax {
				return casePatch{}, fmt.Errorf("hub_threshold must be a whole number from %d to %d, or null to use the default", caseHubThresholdMin, caseHubThresholdMax)
			}
			patch.HubThreshold = &n
		}
	}
	if !patch.HasDisplayName && patch.Status == nil && !patch.HasHubThreshold {
		return casePatch{}, errors.New("nothing to change: send display_name, status, hub_threshold or any of them")
	}
	return patch, nil
}

const listCaseRecordsSQL = `
WITH evidence AS (
  SELECT collection_id, count(*) AS evidence_count FROM forensic.evidence_items WHERE tenant_id = $1 GROUP BY collection_id
)
SELECT coalesce(e.collection_id, r.collection_id) AS case_id, r.display_name, coalesce(r.status, 'open') AS status, r.hub_threshold, r.created_by, r.created_at,
       r.updated_by, r.updated_at, (r.collection_id IS NOT NULL) AS recorded, coalesce(e.evidence_count, 0) AS evidence_count
FROM evidence e
FULL OUTER JOIN forensic.case_record r ON r.tenant_id = $1 AND r.collection_id = e.collection_id
WHERE ($2 = '' OR coalesce(e.collection_id, r.collection_id) = $2)
ORDER BY 1`

func listCaseRecords(ctx context.Context, db *pgxpool.Pool, tenantID, onlyCase string) ([]caseRecord, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	tx, err := db.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, fmt.Errorf("begin case list: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // read-only transaction
	if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenantID); err != nil {
		return nil, fmt.Errorf("set tenant context: %w", err)
	}
	rows, err := tx.Query(ctx, listCaseRecordsSQL, tenantID, onlyCase)
	if err != nil {
		return nil, fmt.Errorf("list cases: %w", err)
	}
	defer rows.Close()
	out := []caseRecord{}
	for rows.Next() {
		var c caseRecord
		if err := rows.Scan(&c.CaseID, &c.DisplayName, &c.Status, &c.HubThreshold, &c.CreatedBy, &c.CreatedAt, &c.UpdatedBy, &c.UpdatedAt, &c.Recorded, &c.EvidenceCount); err != nil {
			return nil, fmt.Errorf("read case row: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

const saveCaseRecordSQL = `
INSERT INTO forensic.case_record (tenant_id, collection_id, display_name, status, hub_threshold, created_by, updated_by, updated_at)
VALUES ($1, $2, $3, coalesce($4, 'open'), $8, nullif($5, ''), nullif($5, ''), now())
ON CONFLICT (tenant_id, collection_id) DO UPDATE SET
  display_name = CASE WHEN $6 THEN EXCLUDED.display_name ELSE forensic.case_record.display_name END,
  status = CASE WHEN $7 THEN EXCLUDED.status ELSE forensic.case_record.status END,
  hub_threshold = CASE WHEN $9 THEN EXCLUDED.hub_threshold ELSE forensic.case_record.hub_threshold END,
  updated_by = EXCLUDED.updated_by,
  updated_at = now()`

// saveCaseRecord applies a validated patch and writes one audit row, in one transaction. A case that has neither evidence nor a record is refused.
func saveCaseRecord(ctx context.Context, db *pgxpool.Pool, tenantID, caseID, subject string, patch casePatch) (caseRecord, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	tx, err := db.Begin(ctx)
	if err != nil {
		return caseRecord{}, fmt.Errorf("begin case save: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // nothing to undo after a commit
	if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenantID); err != nil {
		return caseRecord{}, fmt.Errorf("set tenant context: %w", err)
	}
	var known bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM forensic.evidence_items WHERE tenant_id = $1 AND collection_id = $2)
		OR EXISTS (SELECT 1 FROM forensic.case_record WHERE tenant_id = $1 AND collection_id = $2)`, tenantID, caseID).Scan(&known); err != nil {
		return caseRecord{}, fmt.Errorf("check the case exists: %w", err)
	}
	if !known {
		return caseRecord{}, errCaseNotFound
	}
	var name any
	if patch.DisplayName != nil {
		name = *patch.DisplayName
	}
	var status any
	if patch.Status != nil {
		status = *patch.Status
	}
	var hub any
	if patch.HubThreshold != nil {
		hub = *patch.HubThreshold
	}
	if _, err := tx.Exec(ctx, saveCaseRecordSQL, tenantID, caseID, name, status, subject, patch.HasDisplayName, patch.Status != nil, hub, patch.HasHubThreshold); err != nil {
		return caseRecord{}, fmt.Errorf("save the case record: %w", err)
	}
	details, err := json.Marshal(map[string]any{"display_name_changed": patch.HasDisplayName, "display_name_cleared": patch.ClearName, "status": status, "hub_threshold_changed": patch.HasHubThreshold, "hub_threshold": hub})
	if err != nil {
		return caseRecord{}, fmt.Errorf("encode the audit details: %w", err)
	}
	actor := subject
	if actor == "" {
		actor = "unknown"
	}
	if _, err := tx.Exec(ctx, `INSERT INTO forensic.records_audit_log (tenant_id, user_id, action, collection_id, details) VALUES ($1, $2, $3, $4, $5::jsonb)`,
		tenantID, actor, caseAuditAction, caseID, string(details)); err != nil {
		return caseRecord{}, fmt.Errorf("write the audit row: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return caseRecord{}, fmt.Errorf("commit the case record: %w", err)
	}
	list, err := listCaseRecords(ctx, db, tenantID, caseID)
	if err != nil || len(list) == 0 {
		return caseRecord{}, fmt.Errorf("read the case record back: %w", err)
	}
	return list[0], nil
}

func caseRecordsHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if db == nil {
			writeError(w, http.StatusServiceUnavailable, errors.New("FORENSIC_DATABASE_URL is not configured"))
			return
		}
		only := strings.TrimSpace(r.PathValue("case_id"))
		scope, err := bindForensicScope(r, strings.TrimSpace(r.URL.Query().Get("tenant_id")), only, "", "")
		if err != nil {
			writeScopeError(w, err)
			return
		}
		// A key that is bound to one collection sees only that case.
		if scope.CollectionID != "" {
			only = scope.CollectionID
		}
		list, err := listCaseRecords(r.Context(), db, scope.TenantID, only)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		if r.PathValue("case_id") != "" {
			if len(list) == 0 {
				writeError(w, http.StatusNotFound, errCaseNotFound)
				return
			}
			writeJSON(w, http.StatusOK, list[0])
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"cases": list, "count": len(list)})
	}
}

func caseRecordPatchHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if db == nil {
			writeError(w, http.StatusServiceUnavailable, errors.New("FORENSIC_DATABASE_URL is not configured"))
			return
		}
		caseID := strings.TrimSpace(r.PathValue("case_id"))
		scope, err := bindForensicScope(r, strings.TrimSpace(r.URL.Query().Get("tenant_id")), caseID, "", "")
		if err != nil {
			writeScopeError(w, err)
			return
		}
		if scope.ActorRole == "agent-worker" {
			writeScopeError(w, errForensicForbidden)
			return
		}
		if caseID == "" {
			writeError(w, http.StatusBadRequest, errors.New("case id is required"))
			return
		}
		patch, err := parseCasePatch(r.Body)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		subject := scope.SubjectID
		if subject == "" {
			subject = scope.ActorID
		}
		rec, err := saveCaseRecord(r.Context(), db, scope.TenantID, caseID, subject, patch)
		switch {
		case errors.Is(err, errCaseNotFound):
			writeError(w, http.StatusNotFound, err)
		case err != nil:
			writeError(w, http.StatusInternalServerError, err)
		default:
			writeJSON(w, http.StatusOK, rec)
		}
	}
}
