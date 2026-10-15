package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ASK ENGINE V2: THE AUDIT RECORD (roadmap 6.8.9, safety prerequisite 5).
//
// Every question the engine looked at leaves one row in forensic.records_audit_log (the table the evidence paths already use; tenant-isolated, append-only for the
// application role): the question, the frame's identifiers, the views and the verified examples shown, the query, the checks that passed, what had to be repaired, the
// model and the prompt's hash, the digest of the result, the times and the outcome. A question answered wrongly can be reconstructed from it: what was asked, what
// the model was told, what it wrote, what the checks said, and what the database returned. The write is best effort: a failure is logged and never changes the answer.
const askAuditAction = "ask_engine_v2.question"

// askAuditSink is a variable so that the specs can record what would be written without a database.
var askAuditSink = askWriteAudit

// askResultDigest is a SHA-256 over the rows the engine returned, in the order returned (never the rows themselves).
func askResultDigest(r *govSQLResult) string {
	if r == nil {
		return ""
	}
	h := sha256.New()
	for _, c := range r.Columns {
		_, _ = fmt.Fprintf(h, "%s\x1f", c)
	}
	for _, row := range r.Rows {
		for _, v := range row {
			_, _ = fmt.Fprintf(h, "%v\x1f", v)
		}
		_, _ = h.Write([]byte{'\n'})
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func askAuditDetails(req hybridQueryRequest, a AskAuditV2, f askFrame, digest string) map[string]any {
	return map[string]any{
		"question": req.Query, "identifiers": f.Identifiers(),
		"state": a.State, "reason": a.Reason, "attempts": a.Attempts, "views": a.Views, "examples": a.Examples, "repairs": a.Repairs, "checks": a.Checks,
		"sql": a.SQL, "shape": a.Shape, "rows": a.Rows, "truncated": a.Truncated, "hops": a.Hops, "plan_cost": a.PlanCost, "agreement": a.Agreement,
		"model": a.Model, "prompt_id": a.PromptID, "prompt_bytes": a.PromptBytes, "result_digest": digest,
		"model_ms": a.ModelMS, "exec_ms": a.ExecMS, "total_ms": a.TotalMS, "recorded_at": time.Now().UTC().Format(time.RFC3339),
	}
}

func askWriteAudit(ctx context.Context, db *pgxpool.Pool, req hybridQueryRequest, a AskAuditV2, f askFrame, digest string) {
	if db == nil || req.TenantID == "" {
		return
	}
	details, err := json.Marshal(askAuditDetails(req, a, f, digest))
	if err != nil {
		slog.Warn("ask engine v2: the audit record could not be built", "error", err)
		return
	}
	// The question has been answered or declined; the audit must not be lost because the request's own context is done.
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	tx, err := db.Begin(writeCtx)
	if err != nil {
		slog.Warn("ask engine v2: the audit record could not be written", "error", err)
		return
	}
	defer tx.Rollback(writeCtx) //nolint:errcheck // nothing to undo after a commit
	if _, err := tx.Exec(writeCtx, "SELECT set_config('app.tenant_id', $1, true)", req.TenantID); err != nil {
		slog.Warn("ask engine v2: the audit record could not be written", "error", err)
		return
	}
	if _, err := tx.Exec(writeCtx, `INSERT INTO forensic.records_audit_log (tenant_id, user_id, action, collection_id, details) VALUES ($1, $2, $3, $4, $5::jsonb)`,
		req.TenantID, req.UserID, askAuditAction, req.CollectionID, string(details)); err != nil {
		slog.Warn("ask engine v2: the audit record could not be written", "error", err)
		return
	}
	if err := tx.Commit(writeCtx); err != nil {
		slog.Warn("ask engine v2: the audit record could not be written", "error", err)
	}
}
