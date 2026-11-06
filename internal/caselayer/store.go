package caselayer

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	// queryTimeout bounds every transaction that is not the engine call; statementTimeoutMS is the same bound inside the database.
	queryTimeout       = 20 * time.Second
	statementTimeoutMS = "15000"
	readyPositiveTTL   = 60 * time.Second
	readyNegativeTTL   = 5 * time.Second
)

// Store runs the layer's transactions. Every one sets the three settings that the row-level policies read, so a forgotten setting fails closed (no rows) instead of open.
type Store struct {
	pool *pgxpool.Pool

	mu        sync.Mutex
	readyAt   time.Time
	readyGood bool
}

// NewStore wraps a pool. A nil pool is allowed: every call then answers "not available".
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// Ready says whether migration 034 is applied. The answer is cached for a minute when good and for five seconds when not, so a migration applied while the API runs is noticed.
func (s *Store) Ready(ctx context.Context) error {
	if s == nil || s.pool == nil {
		return Unavailable("The database is not configured (FORENSIC_DATABASE_URL).")
	}
	s.mu.Lock()
	checkedAt, good := s.readyAt, s.readyGood
	s.mu.Unlock()
	age := time.Since(checkedAt)
	if (good && age < readyPositiveTTL) || (!good && !checkedAt.IsZero() && age < readyNegativeTTL) {
		if good {
			return nil
		}
		return newError(503, "not_installed", "The case layer is not installed in this database (migration 034).")
	}
	var present bool
	cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := s.pool.QueryRow(cctx, `SELECT to_regclass('forensic.conversation_turn') IS NOT NULL AND to_regclass('forensic.case_audit_event') IS NOT NULL`).Scan(&present); err != nil {
		return Unavailable("The database could not be asked whether the case layer is installed.")
	}
	s.mu.Lock()
	s.readyAt, s.readyGood = time.Now(), present
	s.mu.Unlock()
	if !present {
		return newError(503, "not_installed", "The case layer is not installed in this database (migration 034).")
	}
	return nil
}

// InTx runs fn in one transaction with the scope's settings. A write transaction commits when fn returns nil; any error rolls it back and is returned as an *Error.
func (s *Store) InTx(ctx context.Context, sc Scope, write bool, fn func(ctx context.Context, tx pgx.Tx) error) error {
	if err := s.Ready(ctx); err != nil {
		return err
	}
	if sc.TenantID == "" || sc.Subject == "" {
		return Forbidden("The request carries no tenant or no subject.")
	}
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	opts := pgx.TxOptions{}
	if !write {
		opts.AccessMode = pgx.ReadOnly
	}
	tx, err := s.pool.BeginTx(ctx, opts)
	if err != nil {
		return asError(err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // nothing to undo after a commit
	if _, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true), set_config('app.subject_id', $2, true), set_config('app.actor_role', $3, true), set_config('statement_timeout', $4, true), set_config('timezone', 'UTC', true)`,
		sc.TenantID, sc.Subject, sc.ActorRole, statementTimeoutMS); err != nil {
		return asError(err)
	}
	if err := fn(ctx, tx); err != nil {
		return asError(err)
	}
	if !write {
		return nil
	}
	if err := tx.Commit(ctx); err != nil {
		return asError(err)
	}
	return nil
}

// utc and utcPtr put a time scanned from the database in UTC, so that every time the service returns (the ones the database renders as JSON and the ones Go encodes) has the same offset.
func utc(t time.Time) time.Time { return t.UTC() }

func utcPtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

// roleIn is the caller's role in a case, from the same function the policies use.
func roleIn(ctx context.Context, tx pgx.Tx, sc Scope, caseID string) (Role, error) {
	if sc.BoundCase != "" && sc.BoundCase != caseID {
		return RoleNone, nil
	}
	var role string
	if err := tx.QueryRow(ctx, `SELECT forensic.case_role($1, $2, $3)`, sc.TenantID, caseID, sc.Subject).Scan(&role); err != nil {
		return RoleNone, err
	}
	return Role(role), nil
}

// requireRole returns the caller's role, or a "not found" for a case the caller may not read (the layer does not say that a case exists).
func requireRole(ctx context.Context, tx pgx.Tx, sc Scope, caseID string) (Role, error) {
	role, err := roleIn(ctx, tx, sc, caseID)
	if err != nil {
		return RoleNone, err
	}
	if !role.CanRead() {
		return RoleNone, NotFound("There is no such case, or you are not a member of it.")
	}
	return role, nil
}

// caseKnown is whether a case exists: it holds evidence or has a case record. A thread cannot be started in a case that does not.
func caseKnown(ctx context.Context, tx pgx.Tx, tenantID, caseID string) (bool, error) {
	var known bool
	err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM forensic.evidence_items WHERE tenant_id = $1 AND collection_id = $2)
		OR EXISTS (SELECT 1 FROM forensic.case_record WHERE tenant_id = $1 AND collection_id = $2)
		OR EXISTS (SELECT 1 FROM forensic.conversation_thread WHERE tenant_id = $1 AND collection_id = $2)`, tenantID, caseID).Scan(&known)
	return known, err
}

// ---- audit ------------------------------------------------------------------------------------------------------------------------------------------------------------------

// audit writes one row of the case's hash chain in the caller's transaction. The hashes are set by the database trigger. Details name what changed, never the text of a note or a question.
func audit(ctx context.Context, tx pgx.Tx, sc Scope, role Role, caseID, action, kind, objectID string, details map[string]any) error {
	if details == nil {
		details = map[string]any{}
	}
	raw, err := json.Marshal(details)
	if err != nil {
		return err
	}
	actor := string(role)
	if sc.IsAdmin() {
		actor = "admin"
	}
	_, err = tx.Exec(ctx, `INSERT INTO forensic.case_audit_event (tenant_id, collection_id, actor_subject, actor_role, action, object_kind, object_id, details)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8::jsonb)`, sc.TenantID, caseID, sc.Subject, actor, action, kind, objectID, string(raw))
	return err
}

// ---- cursors ----------------------------------------------------------------------------------------------------------------------------------------------------------------

// Page is a page of a list with the cursor of the next one (empty when there is none).
type Page[T any] struct {
	Items      []T    `json:"items"`
	NextCursor string `json:"next_cursor"`
}

func encodeCursor(v any) string {
	raw, _ := json.Marshal(v)
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decodeCursor(s string, v any) error {
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(s))
	if err != nil {
		return BadRequest("The cursor is not one this service issued.")
	}
	if err := json.Unmarshal(raw, v); err != nil {
		return BadRequest("The cursor is not one this service issued.")
	}
	return nil
}

// clampLimit applies the page size rules of every list: 30 by default, 1 to 100.
func clampLimit(limit int) int {
	switch {
	case limit <= 0:
		return 30
	case limit > 100:
		return 100
	default:
		return limit
	}
}
