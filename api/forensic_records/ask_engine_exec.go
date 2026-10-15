package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ASK ENGINE V2: THE EXECUTOR (roadmap 6.8.2 step 5, 6.8.9).
//
// A validated query runs inside walls that neither trust the validator nor are trusted by it:
//
//  1. THE ROLE. `SET LOCAL ROLE forensic_analyst_ro` (migration 017): it can read the analyst views and nothing else, so even a query the validator wrongly let
//     through cannot name a base table. It is NOLOGIN; the application's login role is made a member and switches inside the transaction.
//  2. THE SCOPE. The views carry the tenant and the collection themselves (`app.tenant_id`, `app.collection_id`, set per transaction). Unset, they return nothing.
//  3. THE TRANSACTION IS READ-ONLY, with a statement timeout, a lock timeout and an idle timeout.
//  4. THE PLAN IS PRICED FIRST. EXPLAIN (no execution) returns the planner's total cost; a query above the ceiling is refused before it spends the laptop's CPU.
//  5. THE ROWS ARE CAPPED. The validated text is wrapped as `SELECT * FROM (<query>) LIMIT cap+1` with a cap the server chose, so that a result larger than the cap is
//     noticed and reported, never silently cut.
//
// The execution recipe is the one in docs/data/analyst-views.md. Role-level settings are not used (they apply at login, not at SET ROLE); everything is set per
// transaction. The case clock is the session time zone, so a time the query reads, compares or returns is on the one clock of governed_sql_zone.go.
const (
	askAnalystRole       = "forensic_analyst_ro"
	askStatementTimeout  = 15 * time.Second
	askRowCap            = 1000
	askDefaultPlanCost   = 5_000_000.0
	askRowCapEnv         = "FORENSIC_ASK_V2_ROW_CAP"
	askPlanCostEnv       = "FORENSIC_ASK_V2_MAX_COST"
	askStatementTimeEnv  = "FORENSIC_ASK_V2_TIMEOUT_S"
	askHubThresholdEnv   = "FORENSIC_ASK_V2_HUB_THRESHOLD"
	askProbeRowsPerQuery = 1
)

// askLimits are the executor's ceilings. The defaults are the proposal of roadmap 6.8.4; each can be changed by an environment variable, which is how the
// laptop's own measurement (6.8.13) sets them.
type askLimits struct {
	Timeout      time.Duration
	RowCap       int
	MaxPlanCost  float64
	HubThreshold string // the case's threshold for "a number everyone contacts"; empty leaves the view's default of 20
}

func askEnvInt(name string, fallback int) int {
	if raw := strings.TrimSpace(os.Getenv(name)); raw != "" { //nolint:forbidigo // forensic records product API reads its settings from the environment by design (docker compose)
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}

func askDefaultLimits() askLimits {
	limits := askLimits{Timeout: time.Duration(askEnvInt(askStatementTimeEnv, int(askStatementTimeout/time.Second))) * time.Second, RowCap: askEnvInt(askRowCapEnv, askRowCap), MaxPlanCost: askDefaultPlanCost}
	if raw := strings.TrimSpace(os.Getenv(askPlanCostEnv)); raw != "" { //nolint:forbidigo // see above
		if cost, err := strconv.ParseFloat(raw, 64); err == nil && cost > 0 {
			limits.MaxPlanCost = cost
		}
	}
	if n := askEnvInt(askHubThresholdEnv, 0); n > 0 {
		limits.HubThreshold = strconv.Itoa(n)
	}
	if limits.RowCap > askMaxLimit {
		limits.RowCap = askMaxLimit
	}
	return limits
}

// askExecute runs one validated query as the analyst role and returns its rows. The error is a *govSQLRunError whose Code is one of NO_DATABASE, NOT_VALIDATED,
// DATABASE, TOO_EXPENSIVE, TIMEOUT, QUERY_ERROR (a mistake the model can repair, with the database's own message), READ_ONLY or CANCELED.
func askExecute(ctx context.Context, db *pgxpool.Pool, tenantID, collectionID string, validated *askValidated, limits askLimits) (*govSQLResult, error) {
	if db == nil {
		return nil, &govSQLRunError{Code: "NO_DATABASE", Message: "no database is configured"}
	}
	if validated == nil || strings.TrimSpace(validated.SQL) == "" {
		return nil, &govSQLRunError{Code: "NOT_VALIDATED", Message: "only a validated query may run"}
	}
	if limits.RowCap <= 0 || limits.RowCap > askMaxLimit {
		limits.RowCap = askRowCap
	}
	// The cap is a literal integer the server chose, never model text.
	final := "SELECT * FROM (\n" + validated.SQL + "\n) AS ask_result LIMIT " + strconv.Itoa(limits.RowCap+1)
	return askRun(ctx, db, tenantID, collectionID, final, nil, limits, limits.RowCap, true)
}

func askRun(ctx context.Context, db *pgxpool.Pool, tenantID, collectionID, final string, args []any, limits askLimits, maxRows int, priced bool) (*govSQLResult, error) {
	if limits.Timeout <= 0 {
		limits.Timeout = askStatementTimeout
	}
	if limits.MaxPlanCost <= 0 {
		limits.MaxPlanCost = askDefaultPlanCost
	}
	started := time.Now()
	callCtx, cancel := context.WithTimeout(ctx, limits.Timeout+5*time.Second)
	defer cancel()
	tx, err := db.BeginTx(callCtx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, &govSQLRunError{Code: "DATABASE", Message: "could not start a read-only transaction"}
	}
	defer tx.Rollback(callCtx) //nolint:errcheck // a read-only transaction has nothing to undo

	zoneName, zone := govSQLTimeZone()
	statements := []struct {
		sql  string
		args []any
		what string
	}{
		{"SET LOCAL ROLE " + askAnalystRole, nil, "could not switch to the read-only analyst role"},
		{"SELECT set_config('app.tenant_id', $1, true)", []any{tenantID}, "could not set the tenant context"},
		{"SELECT set_config('app.collection_id', $1, true)", []any{collectionID}, "could not set the collection context"},
		{"SELECT set_config('TimeZone', $1, true)", []any{zoneName}, "could not set the case time zone"},
		{"SET LOCAL statement_timeout = " + strconv.FormatInt(limits.Timeout.Milliseconds(), 10), nil, "could not set the query limits"},
		{"SET LOCAL lock_timeout = 2000", nil, "could not set the query limits"},
		{"SET LOCAL idle_in_transaction_session_timeout = 30000", nil, "could not set the query limits"},
		{"SET LOCAL search_path = analyst, pg_catalog", nil, "could not set the search path"},
	}
	if limits.HubThreshold != "" {
		statements = append(statements, struct {
			sql  string
			args []any
			what string
		}{"SELECT set_config('app.hub_threshold', $1, true)", []any{limits.HubThreshold}, "could not set the hub threshold"})
	}
	for _, s := range statements {
		if _, err := tx.Exec(callCtx, s.sql, s.args...); err != nil {
			return nil, &govSQLRunError{Code: "DATABASE", Message: s.what, Detail: err.Error()}
		}
	}

	var cost float64
	if priced {
		cost, err = govSQLPlanCost(callCtx, tx, final, args)
		if err != nil {
			return nil, govSQLMapError(err)
		}
		if cost > limits.MaxPlanCost {
			return nil, &govSQLRunError{Code: "TOO_EXPENSIVE", Message: fmt.Sprintf("the query would read far too much data (planner cost %.0f, limit %.0f); narrow it by a number, a time or a cell first", cost, limits.MaxPlanCost)}
		}
	}

	rows, err := tx.Query(callCtx, final, args...)
	if err != nil {
		return nil, govSQLMapError(err)
	}
	defer rows.Close()
	descriptions := rows.FieldDescriptions()
	result := &govSQLResult{Columns: make([]string, len(descriptions)), PlanCost: cost}
	columnTypes := make([]uint32, len(descriptions))
	result.Texts = make([]bool, len(descriptions))
	for i, d := range descriptions {
		result.Columns[i] = string(d.Name)
		columnTypes[i] = d.DataTypeOID
		result.Texts[i] = d.DataTypeOID == pgtype.TextOID || d.DataTypeOID == pgtype.VarcharOID || d.DataTypeOID == pgtype.BPCharOID
	}
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, govSQLMapError(err)
		}
		if len(result.Rows) >= maxRows {
			result.Truncated = true
			break
		}
		row := make([]any, len(values))
		for i, value := range values {
			row[i] = govSQLCellValue(value, columnTypes[i], zone)
		}
		result.Rows = append(result.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return nil, govSQLMapError(err)
	}
	rows.Close()
	result.ElapsedMS = time.Since(started).Milliseconds()
	return result, nil
}

// askProbe says where an identifier occurs in the case: the "view.column" of every analyst relation that holds it. It is how an empty result is told from a
// wrong column: nothing anywhere means "no record of this", a hit in a column the query did not use means the query looked in the wrong place. The statements
// are built from the catalogue's own names (never from the model) and run as the analyst role with the same scope.
func askProbe(ctx context.Context, db *pgxpool.Pool, tenantID, collectionID string, cat *askCatalogue, identifier string, limits askLimits) ([]string, error) {
	var hits []string
	for _, view := range cat.Views {
		for _, col := range view.Columns {
			expr := ""
			switch {
			case strings.HasPrefix(col.Type, "inet"):
				expr = "host(" + col.Name + ")"
			case strings.HasPrefix(col.Type, "text") || strings.HasPrefix(col.Type, "character"):
				expr = col.Name
			default:
				continue
			}
			if !askProbeColumn(col.Name) {
				continue
			}
			sql := "SELECT 1 FROM " + view.Name + " WHERE " + expr + " = $1 LIMIT " + strconv.Itoa(askProbeRowsPerQuery)
			result, err := askRun(ctx, db, tenantID, collectionID, sql, []any{identifier}, limits, askProbeRowsPerQuery, false)
			if err != nil {
				return nil, err
			}
			if len(result.Rows) > 0 {
				hits = append(hits, view.Name+"."+col.Name)
			}
		}
	}
	return hits, nil
}

// askProbeColumn is true for the columns that can hold an identifier: a number, a device, a SIM, an address, a cell. Names and descriptions never match an
// identifier, so probing them would only cost time.
func askProbeColumn(name string) bool {
	switch name {
	case "src", "dst", "imsi", "imei", "cell_key", "public_ip", "ue_ip", "server_ip":
		return true
	}
	return false
}
