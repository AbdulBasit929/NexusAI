package caselayer

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// PinResult is a finding made from an answer. AlreadyPinned is true when the answer already had one (pinning twice returns it, it does not make a second).
type PinResult struct {
	Finding       json.RawMessage
	AlreadyPinned bool
}

// PinTurn makes a finding of an answered question. The finding keeps a digest of the answer as it was when pinned (its sentence, its figures, the query hash, the engine and the data version), so it stays
// reproducible even if the conversation is hidden. Only an answered question can be pinned: an abstention or a clarification is not a claim.
func (s *Service) PinTurn(ctx context.Context, sc Scope, turnID, title, note, basis string) (*PinResult, error) {
	if basis == "" {
		basis = "observed"
	}
	if basis != "observed" && basis != "inferred" {
		return nil, BadRequest("basis must be observed or inferred.")
	}
	var cleanTitle string
	if strings.TrimSpace(title) != "" {
		var err error
		if cleanTitle, err = cleanLine("The title", title, 1, 200); err != nil {
			return nil, err
		}
	}
	var body string
	if strings.TrimSpace(note) != "" {
		var err error
		if body, err = cleanBody("The note", note, 1, 20000); err != nil {
			return nil, err
		}
	}
	var res PinResult
	err := s.Store.InTx(ctx, sc, true, func(ctx context.Context, tx pgx.Tx) error {
		turns, err := loadTurns(ctx, tx, sc, `t.turn_id = $2::uuid`, []any{turnID}, "t.idx", 1)
		if err != nil {
			return err
		}
		if len(turns) == 0 {
			return NotFound("There is no such answer, or you may not see it.")
		}
		t := turns[0]
		if _, _, err := loadThread(ctx, tx, sc, t.ThreadID); err != nil {
			return err
		}
		role, err := requireRole(ctx, tx, sc, t.CaseID)
		if err != nil {
			return err
		}
		if !role.CanContribute() {
			return Forbidden("Your role in this case (%s) does not allow making findings.", role)
		}
		var existing string
		err = tx.QueryRow(ctx, `SELECT finding_id::text FROM forensic.finding WHERE tenant_id = $1 AND collection_id = $2 AND source_turn_id = $3::uuid AND deleted_at IS NULL`, sc.TenantID, t.CaseID, turnID).Scan(&existing)
		if err == nil {
			res.AlreadyPinned = true
			res.Finding, _, _, err = findingSpec.readObject(ctx, tx, sc, existing)
			return err
		}
		if err != pgx.ErrNoRows {
			return err
		}
		if t.State != "completed" || t.Outcome == nil || *t.Outcome != OutcomeAnswered {
			return Conflict("not_an_answer", "Only an answered question can be pinned to the findings; this one is %s.", describeTurnState(t))
		}
		var figures []byte
		if err := tx.QueryRow(ctx, `SELECT figures_json FROM forensic.conversation_turn WHERE turn_id = $1::uuid`, turnID).Scan(&figures); err != nil {
			return err
		}
		headline := ""
		if t.Headline != nil {
			headline = *t.Headline
		}
		if cleanTitle == "" {
			cleanTitle = clip(headline, 120)
		}
		if cleanTitle == "" {
			cleanTitle = clip(t.Question, 120)
		}
		pinned := map[string]any{
			"turn_id": t.ID, "thread_id": t.ThreadID, "question": t.Question, "headline": headline, "outcome": derefString(t.Outcome), "shape": derefString(t.Shape),
			"row_count": t.RowCount, "result_digest": derefString(t.ResultDigest), "query_hash": derefString(t.QueryHash), "engine_version": derefString(t.EngineVersion),
			"model": derefString(t.ModelID), "data_version_hash": derefString(t.DataVersionHash), "answered_at": t.CompletedAt, "pinned_at": time.Now().UTC(),
		}
		if len(figures) > 0 {
			pinned["figures"] = trimFigures(figures, 50)
		}
		pinnedJSON, err := json.Marshal(pinned)
		if err != nil {
			return err
		}
		evidence := t.Evidence
		if len(evidence) == 0 {
			evidence = json.RawMessage("[]")
		}
		var id string
		if err := tx.QueryRow(ctx, `INSERT INTO forensic.finding (tenant_id, collection_id, title, body, basis, source_turn_id, pinned_json, evidence_refs, data_version_hash, created_by)
			VALUES ($1, $2, $3, $4, $5, $6::uuid, $7::jsonb, $8::jsonb, $9, $10) RETURNING finding_id::text`,
			sc.TenantID, t.CaseID, cleanTitle, body, basis, turnID, string(pinnedJSON), string(evidence), t.DataVersionHash, sc.Subject).Scan(&id); err != nil {
			return err
		}
		if err := audit(ctx, tx, sc, role, t.CaseID, "finding.pinned", "finding", id, map[string]any{"turn": turnID, "basis": basis}); err != nil {
			return err
		}
		res.Finding, _, _, err = findingSpec.readObject(ctx, tx, sc, id)
		return err
	})
	if err != nil {
		return nil, err
	}
	return &res, nil
}

func describeTurnState(t Turn) string {
	switch {
	case t.State != "completed":
		return "not finished (" + t.State + ")"
	case t.Outcome != nil:
		return *t.Outcome
	}
	return "without an outcome"
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// trimFigures keeps the first n rows of a stored figures document, for the digest a finding carries.
func trimFigures(raw []byte, n int) json.RawMessage {
	var f map[string]any
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil
	}
	if rows, ok := f["rows"].([]any); ok && len(rows) > n {
		f["rows"] = rows[:n]
		f["truncated"] = true
	}
	out, _ := json.Marshal(f)
	return out
}

// ReviewFinding records a second person's verdict on a finding: confirmed, rejected, or unreviewed (to withdraw a verdict). In a controlled case the reviewer must not be the person who made the
// finding. A case that is still open has one shared identity, so there the verdict is a label, not a control (the answer says so through the case's mode).
func (s *Service) ReviewFinding(ctx context.Context, sc Scope, id, state, note string) (json.RawMessage, error) {
	if state != "confirmed" && state != "rejected" && state != "unreviewed" {
		return nil, BadRequest("state must be confirmed, rejected or unreviewed.")
	}
	var notePtr *string
	if strings.TrimSpace(note) != "" {
		n, err := cleanBody("The note", note, 1, 2000)
		if err != nil {
			return nil, err
		}
		notePtr = &n
	}
	var out json.RawMessage
	err := s.Store.InTx(ctx, sc, true, func(ctx context.Context, tx pgx.Tx) error {
		_, caseID, author, err := findingSpec.readObject(ctx, tx, sc, id)
		if err != nil {
			return err
		}
		role, err := requireRole(ctx, tx, sc, caseID)
		if err != nil {
			return err
		}
		if !role.CanReview() {
			return Forbidden("Your role in this case (%s) does not allow reviewing findings.", role)
		}
		if state == "unreviewed" && !(role == RoleOwner || role == RoleLead || role == RoleOpen || sc.IsAdmin()) {
			return Forbidden("Only an owner or a lead can withdraw a review.")
		}
		if state != "unreviewed" && role != RoleOpen && author == sc.Subject && !sc.IsAdmin() {
			return Forbidden("A finding must be reviewed by someone other than the person who made it.")
		}
		if err := audit(ctx, tx, sc, role, caseID, "finding.reviewed", "finding", id, map[string]any{"state": state, "with_note": notePtr != nil}); err != nil {
			return err
		}
		if state == "unreviewed" {
			_, err = tx.Exec(ctx, `UPDATE forensic.finding SET review_state = 'unreviewed', reviewed_by = NULL, reviewed_at = NULL, review_note = NULL, updated_by = $3, updated_at = now()
				WHERE tenant_id = $1 AND finding_id = $2::uuid AND deleted_at IS NULL`, sc.TenantID, id, sc.Subject)
		} else {
			_, err = tx.Exec(ctx, `UPDATE forensic.finding SET review_state = $4, reviewed_by = $3, reviewed_at = now(), review_note = $5, updated_by = $3, updated_at = now()
				WHERE tenant_id = $1 AND finding_id = $2::uuid AND deleted_at IS NULL`, sc.TenantID, id, sc.Subject, state, notePtr)
		}
		if err != nil {
			return err
		}
		out, _, _, err = findingSpec.readObject(ctx, tx, sc, id)
		return err
	})
	return out, err
}
