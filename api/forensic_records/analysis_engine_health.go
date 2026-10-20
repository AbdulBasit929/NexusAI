package main

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// THE ENGINE'S HEALTH FOR A CASE (roadmap 7.8.3 row 9; written 2026-10-19).
//
//	GET /analysis/engine-health?collection_id=...&days=7   how the Ask engine has done on this case in the last days
//
// It reads the engine's own audit rows (one per question it looked at, `ask_engine_v2.question`) and returns only COUNTS and TIMES: how many questions, how many answered,
// abstained and declined, the median, 90th percentile and longest time of the answered ones, the repairs, and the reasons for declines. It never returns a question, an answer,
// a query or a number of the case: the audit rows hold those, and this read does not select them. A reason is clipped, its parenthesised part (the phrase of a relative time) and any run of three or more digits are replaced by a mark, so no word of a question and no number of a case leaves with it. The rows are the tenant's own (row-level security by tenant, set for the
// read), and only the collection asked for.
const (
	engineHealthContract = "forensics.engine-health/v1"
	engineHealthMaxDays  = 90
)

type engineHealthLatency struct {
	P50 *int64 `json:"p50"`
	P90 *int64 `json:"p90"`
	Max *int64 `json:"max"`
}

type engineHealthDecline struct {
	Reason string `json:"reason"`
	Count  int64  `json:"count"`
}

type engineHealth struct {
	Contract   string                `json:"contract"`
	Available  bool                  `json:"available"`
	Collection string                `json:"collection_id"`
	Days       int                   `json:"days"`
	Since      time.Time             `json:"since"`
	Totals     map[string]int64      `json:"totals"`
	LatencyMS  engineHealthLatency   `json:"latency_ms"`
	Repairs    int64                 `json:"repairs"`
	Declines   []engineHealthDecline `json:"declines"`
	Agreement  string                `json:"agreement"`
}

func asInt64Ptr(f *float64) *int64 {
	if f == nil {
		return nil
	}
	v := int64(*f + 0.5)
	return &v
}

func engineHealthHandler(db *pgxpool.Pool) http.HandlerFunc {
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
		days := queryInt(r, "days", 7)
		if days < 1 {
			days = 1
		}
		if days > engineHealthMaxDays {
			days = engineHealthMaxDays
		}
		agreement := "off"
		if askV2AgreeEnabled() {
			agreement = "on"
		}
		out := engineHealth{Contract: engineHealthContract, Available: true, Collection: scope.CollectionID, Days: days, Since: time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour),
			Totals: map[string]int64{}, Declines: []engineHealthDecline{}, Agreement: agreement}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		tx, err := db.Begin(ctx)
		if err != nil {
			writeError(w, http.StatusInternalServerError, errors.New("the engine's health could not be read"))
			return
		}
		defer tx.Rollback(ctx) //nolint:errcheck // a read only
		if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", scope.TenantID); err != nil {
			writeError(w, http.StatusInternalServerError, errors.New("the engine's health could not be read"))
			return
		}
		var questions, answered, abstained, declined, repairs int64
		var p50, p90, longest *float64
		if err := tx.QueryRow(ctx, `
			SELECT count(*),
			       count(*) FILTER (WHERE details->>'state' = 'answered'),
			       count(*) FILTER (WHERE details->>'state' = 'abstained'),
			       count(*) FILTER (WHERE details->>'state' = 'declined'),
			       coalesce(sum(greatest(coalesce((details->>'attempts')::int, 1) - 1, 0)) FILTER (WHERE details->>'state' = 'answered'), 0),
			       (percentile_cont(0.5) WITHIN GROUP (ORDER BY (details->>'total_ms')::double precision) FILTER (WHERE details->>'state' = 'answered')),
			       (percentile_cont(0.9) WITHIN GROUP (ORDER BY (details->>'total_ms')::double precision) FILTER (WHERE details->>'state' = 'answered')),
			       (max((details->>'total_ms')::double precision) FILTER (WHERE details->>'state' = 'answered'))
			FROM forensic.records_audit_log
			WHERE tenant_id = $1 AND collection_id = $2 AND action = $3 AND created_at >= $4`,
			scope.TenantID, scope.CollectionID, askAuditAction, out.Since).Scan(&questions, &answered, &abstained, &declined, &repairs, &p50, &p90, &longest); err != nil {
			writeError(w, http.StatusInternalServerError, errors.New("the engine's health could not be read"))
			return
		}
		out.Totals = map[string]int64{"questions": questions, "answered": answered, "abstained": abstained, "declined": declined}
		out.Repairs = repairs
		out.LatencyMS = engineHealthLatency{P50: asInt64Ptr(p50), P90: asInt64Ptr(p90), Max: asInt64Ptr(longest)}
		rows, err := tx.Query(ctx, `
			SELECT regexp_replace(regexp_replace(left(coalesce(details->>'reason', ''), 120), '\([^)]*\)', '(#)', 'g'), '[0-9]{3,}', '#', 'g') AS reason, count(*)
			FROM forensic.records_audit_log
			WHERE tenant_id = $1 AND collection_id = $2 AND action = $3 AND created_at >= $4 AND details->>'state' = 'declined' AND coalesce(details->>'reason', '') <> ''
			GROUP BY 1 ORDER BY 2 DESC, 1 LIMIT 8`, scope.TenantID, scope.CollectionID, askAuditAction, out.Since)
		if err != nil {
			writeError(w, http.StatusInternalServerError, errors.New("the engine's health could not be read"))
			return
		}
		defer rows.Close()
		for rows.Next() {
			var d engineHealthDecline
			if err := rows.Scan(&d.Reason, &d.Count); err != nil {
				writeError(w, http.StatusInternalServerError, errors.New("the engine's health could not be read"))
				return
			}
			out.Declines = append(out.Declines, d)
		}
		if err := rows.Err(); err != nil {
			writeError(w, http.StatusInternalServerError, errors.New("the engine's health could not be read"))
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}
