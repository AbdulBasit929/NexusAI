package caselayer

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// AuditEvent is one row of a case's hash chain.
type AuditEvent struct {
	Seq        int64           `json:"seq"`
	Actor      string          `json:"actor"`
	ActorRole  string          `json:"actor_role"`
	Action     string          `json:"action"`
	ObjectKind string          `json:"object_kind"`
	ObjectID   string          `json:"object_id"`
	Details    json.RawMessage `json:"details"`
	OccurredAt time.Time       `json:"occurred_at"`
	Hash       string          `json:"event_sha256"`
	Previous   *string         `json:"previous_sha256"`
}

// AuditFilter selects events, newest first.
type AuditFilter struct {
	Action     string
	ObjectKind string
	ObjectID   string
	Limit      int
	Cursor     string
}

type auditCursor struct {
	Seq int64 `json:"s"`
}

// ListAudit reads the audit trail of a case. Owners, leads, auditors and administrators read it (anyone, while the case is open).
func (s *Service) ListAudit(ctx context.Context, sc Scope, caseID string, f AuditFilter) (Page[AuditEvent], error) {
	page := Page[AuditEvent]{Items: []AuditEvent{}}
	limit := clampLimit(f.Limit)
	err := s.Store.InTx(ctx, sc, false, func(ctx context.Context, tx pgx.Tx) error {
		role, err := requireRole(ctx, tx, sc, caseID)
		if err != nil {
			return err
		}
		if !role.CanReadAudit() {
			return Forbidden("Your role in this case (%s) does not allow reading its audit trail.", role)
		}
		args := []any{sc.TenantID, caseID}
		where := []string{"tenant_id = $1", "collection_id = $2"}
		arg := func(v any) string { args = append(args, v); return "$" + strconv.Itoa(len(args)) }
		if f.Action != "" {
			if strings.HasSuffix(f.Action, ".") {
				where = append(where, "action LIKE "+arg(likePattern(f.Action)+"%"))
			} else {
				where = append(where, "action = "+arg(f.Action))
			}
		}
		if f.ObjectKind != "" {
			where = append(where, "object_kind = "+arg(f.ObjectKind))
		}
		if f.ObjectID != "" {
			where = append(where, "object_id = "+arg(f.ObjectID))
		}
		if f.Cursor != "" {
			var c auditCursor
			if err := decodeCursor(f.Cursor, &c); err != nil {
				return err
			}
			where = append(where, "seq < "+arg(c.Seq))
		}
		rows, err := tx.Query(ctx, `SELECT seq, actor_subject, actor_role, action, object_kind, object_id, details, occurred_at, event_sha256, previous_sha256
			FROM forensic.case_audit_event WHERE `+strings.Join(where, " AND ")+` ORDER BY seq DESC LIMIT `+strconv.Itoa(limit+1), args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var e AuditEvent
			var details []byte
			if err := rows.Scan(&e.Seq, &e.Actor, &e.ActorRole, &e.Action, &e.ObjectKind, &e.ObjectID, &details, &e.OccurredAt, &e.Hash, &e.Previous); err != nil {
				return err
			}
			e.Details = details
			e.OccurredAt = utc(e.OccurredAt)
			page.Items = append(page.Items, e)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		if len(page.Items) > limit {
			page.Items = page.Items[:limit]
			page.NextCursor = encodeCursor(auditCursor{Seq: page.Items[limit-1].Seq})
		}
		return nil
	})
	return page, err
}

// ChainStatus is the result of recomputing a case's audit chain.
type ChainStatus struct {
	CaseID         string `json:"case_id"`
	Events         int64  `json:"events"`
	OK             bool   `json:"ok"`
	FirstBrokenSeq *int64 `json:"first_broken_seq"`
}

// VerifyAudit recomputes every hash of the case's chain and says whether each row still follows from the one before it.
func (s *Service) VerifyAudit(ctx context.Context, sc Scope, caseID string) (*ChainStatus, error) {
	st := ChainStatus{CaseID: caseID}
	err := s.Store.InTx(ctx, sc, false, func(ctx context.Context, tx pgx.Tx) error {
		role, err := requireRole(ctx, tx, sc, caseID)
		if err != nil {
			return err
		}
		if !role.CanReadAudit() {
			return Forbidden("Your role in this case (%s) does not allow reading its audit trail.", role)
		}
		return tx.QueryRow(ctx, `SELECT events, ok, first_broken_seq FROM forensic.case_audit_chain_status($1, $2)`, sc.TenantID, caseID).Scan(&st.Events, &st.OK, &st.FirstBrokenSeq)
	})
	if err != nil {
		return nil, err
	}
	return &st, nil
}
