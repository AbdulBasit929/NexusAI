package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// GOVERNED SQL: EXECUTION.
//
// A validated query still runs inside three more walls, none of which trusts the
// validator and none of which the validator trusts:
//
//  1. THE VIEW IS BUILT BY THE SERVER. The model's text is placed inside
//     `WITH <view> AS (<server-built scope and columns>) SELECT * FROM (<model SQL>) LIMIT n`.
//     Tenant, case and family scope are in the server's CTE, so nothing the model
//     writes can widen them, and forensic.records is not nameable from the model's
//     text (the validator refuses any other relation; the view hides the rest).
//  2. THE TRANSACTION IS READ-ONLY, with a statement timeout, a lock timeout and an
//     idle timeout. A write would fail in the database even if a parse-tree rule
//     were wrong.
//  3. THE PLAN IS PRICED FIRST. EXPLAIN (no execution) returns the planner's cost;
//     a query above the ceiling is refused before it burns the laptop's CPU.
//
// The fourth wall, a database role that can read nothing but analyst views, needs a
// database change and is the owner's decision (docs/work/BACKEND_REQUESTS.md). Until
// then the first three are what stands, which is why the validator is as strict as
// it is.
const (
	govSQLStatementTimeout = 20 * time.Second
	govSQLMaxRows          = 200
	// govSQLMaxPlanCost is in planner cost units. The demo's whole CDR view costs
	// about 1,000; a 10-million-row case about 10^6. A cross product or a runaway
	// regular expression over the whole collection costs orders of magnitude more.
	govSQLMaxPlanCost = 5_000_000.0
)

type govSQLResult struct {
	Columns []string
	// Labels is what each column is called in the answer, where that is not simply the column's own name
	// (a count of distinct values says so). Empty means the humanised column name.
	Labels []string
	// Texts says, for each column, that the database returned text: a phone number, a plate or a word, never a
	// quantity, whatever alias the query gave the column.
	Texts     []bool
	Rows      [][]any
	Truncated bool
	ElapsedMS int64
	PlanCost  float64
}

// cell formats the value of column i. A value in a text column is shown exactly as stored (an identifier is not a
// quantity: 923001110001 is not "923,001,110,001"), whatever alias the query gave the column; the view's own labels
// for a value are used where the column is the view's.
func (r *govSQLResult) cell(view *govSQLView, i int, value any) string {
	if text, ok := value.(string); ok && i < len(r.Texts) && r.Texts[i] {
		if view != nil && i < len(r.Columns) {
			for _, c := range view.Columns {
				if c.Name == r.Columns[i] && c.Type == "text" {
					if label := c.Labels[text]; label != "" {
						return label
					}
				}
			}
		}
		return text
	}
	if i < len(r.Columns) {
		return govSQLFormatCell(view, r.Columns[i], value)
	}
	return govSQLFormatValue(value)
}

// label is the name the answer gives column i.
func (r *govSQLResult) label(i int) string {
	if i < len(r.Labels) && r.Labels[i] != "" {
		return r.Labels[i]
	}
	if i < len(r.Columns) {
		return govSQLHumanize(r.Columns[i])
	}
	return "Result"
}

// govSQLRunError carries what the model needs to correct itself, and what the
// analyst-facing log needs to say, without leaking server detail.
type govSQLRunError struct {
	Code    string
	Message string
	// Detail is the database's own text, for the server log only. It is never put in a
	// response or a prompt: it can name tables and settings.
	Detail string
}

func (e *govSQLRunError) Error() string { return e.Code + ": " + e.Message }

func govSQLExecute(ctx context.Context, db *pgxpool.Pool, req hybridQueryRequest, validated *govSQLValidated, maxRows int, timeout time.Duration) (*govSQLResult, error) {
	if db == nil {
		return nil, &govSQLRunError{Code: "NO_DATABASE", Message: "no database is configured"}
	}
	if validated == nil || validated.View == nil {
		return nil, &govSQLRunError{Code: "NOT_VALIDATED", Message: "only a validated query may run"}
	}
	if maxRows <= 0 || maxRows > govSQLMaxRows {
		maxRows = govSQLMaxRows
	}
	cte, args, err := govSQLCTE(validated.View, req, nil)
	if err != nil {
		return nil, &govSQLRunError{Code: "SCOPE", Message: err.Error()}
	}
	// The row cap is a literal integer the server chose, never model text.
	final := "WITH " + cte + "\nSELECT * FROM (\n" + validated.SQL + "\n) AS governed_result LIMIT " + strconv.Itoa(maxRows+1)
	return govSQLRun(ctx, db, req.TenantID, final, args, maxRows, timeout)
}

// govSQLRun executes already-assembled SQL under the database walls. It is separate
// so the tests can attack walls 2 and 3 directly, with SQL the validator would
// never have let through.
func govSQLRun(ctx context.Context, db *pgxpool.Pool, tenantID, final string, args []any, maxRows int, timeout time.Duration) (*govSQLResult, error) {
	if timeout <= 0 {
		timeout = govSQLStatementTimeout
	}
	started := time.Now()
	callCtx, cancel := context.WithTimeout(ctx, timeout+5*time.Second)
	defer cancel()
	tx, err := db.BeginTx(callCtx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, &govSQLRunError{Code: "DATABASE", Message: "could not start a read-only transaction"}
	}
	defer tx.Rollback(callCtx) //nolint:errcheck // a read-only transaction has nothing to undo
	if _, err := tx.Exec(callCtx, "SELECT set_config('app.tenant_id', $1, true)", tenantID); err != nil {
		return nil, &govSQLRunError{Code: "DATABASE", Message: "could not set the tenant context"}
	}
	// THE CASE CLOCK (governed_sql_zone.go): the hour, day and month of every time the query reads or
	// compares are evaluated in ONE zone, and every time it returns is shown in it.
	zoneName, zone := govSQLTimeZone()
	if _, err := tx.Exec(callCtx, "SELECT set_config('TimeZone', $1, true)", zoneName); err != nil {
		return nil, &govSQLRunError{Code: "DATABASE", Message: "could not set the case time zone", Detail: err.Error()}
	}
	for _, setting := range []string{
		"SET LOCAL statement_timeout = " + strconv.FormatInt(timeout.Milliseconds(), 10),
		"SET LOCAL lock_timeout = 2000",
		"SET LOCAL idle_in_transaction_session_timeout = 30000",
	} {
		if _, err := tx.Exec(callCtx, setting); err != nil {
			return nil, &govSQLRunError{Code: "DATABASE", Message: "could not set the query limits"}
		}
	}

	cost, err := govSQLPlanCost(callCtx, tx, final, args)
	if err != nil {
		return nil, govSQLMapError(err)
	}
	if cost > govSQLMaxPlanCost {
		return nil, &govSQLRunError{Code: "TOO_EXPENSIVE", Message: fmt.Sprintf("the query would read far too much data (planner cost %.0f, limit %.0f); filter on a column first", cost, govSQLMaxPlanCost)}
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

// govSQLPlanCost asks the planner for the total cost without running the query.
func govSQLPlanCost(ctx context.Context, tx pgx.Tx, final string, args []any) (float64, error) {
	var raw []byte
	if err := tx.QueryRow(ctx, "EXPLAIN (FORMAT JSON) "+final, args...).Scan(&raw); err != nil {
		return 0, err
	}
	var plan []struct {
		Plan struct {
			TotalCost float64 `json:"Total Cost"`
		} `json:"Plan"`
	}
	if err := json.Unmarshal(raw, &plan); err != nil || len(plan) == 0 {
		return 0, errors.New("the planner returned no cost")
	}
	return plan[0].Plan.TotalCost, nil
}

// govSQLMapError turns a database error into a short, correctable message. The text
// is the database's own message for the user's mistake (a bad cast, a missing
// column) and a fixed sentence for everything else, so nothing about the server
// reaches the model or the analyst.
func govSQLMapError(err error) error {
	var run *govSQLRunError
	if errors.As(err, &run) {
		return run
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return &govSQLRunError{Code: "TIMEOUT", Message: "the query took too long; filter or aggregate earlier"}
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		detail := pgErr.Code + ": " + govSQLClip(pgErr.Message, 300)
		switch pgErr.Code {
		case "57014":
			return &govSQLRunError{Code: "TIMEOUT", Message: "the query took too long; filter or aggregate earlier"}
		case "25006":
			return &govSQLRunError{Code: "READ_ONLY", Message: "this query tried to change data, which is not allowed"}
		case "42703", "42883", "42804", "42725", "22P02", "22007", "22008", "22012", "22003", "42601", "42803", "42P20", "42702", "22023", "2201X":
			return &govSQLRunError{Code: "QUERY_ERROR", Message: govSQLClip(pgErr.Message, 200)}
		}
		return &govSQLRunError{Code: "DATABASE", Message: "the database could not run this query", Detail: detail}
	}
	if strings.Contains(err.Error(), "context canceled") {
		return &govSQLRunError{Code: "CANCELED", Message: "the request was canceled"}
	}
	return &govSQLRunError{Code: "DATABASE", Message: "the database could not run this query", Detail: govSQLClip(err.Error(), 300)}
}
