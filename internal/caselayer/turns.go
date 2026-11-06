package caselayer

import (
	"bytes"
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// RerunInfo tells a turn about the later runs of the same question: how many, the latest, and whether the latest one differed from the run before it.
type RerunInfo struct {
	Count         int     `json:"count"`
	LatestTurnID  *string `json:"latest_turn_id"`
	LatestChanged *bool   `json:"latest_changed"`
}

// Turn is one question and its answer. Answer is the engine's response exactly as returned; it is present on the call that ran the turn and on a single-turn read, and left out of lists.
type Turn struct {
	ID              string          `json:"turn_id"`
	ThreadID        string          `json:"thread_id"`
	CaseID          string          `json:"case_id"`
	Index           int             `json:"idx"`
	GroupID         *string         `json:"group_id"`
	ParentID        *string         `json:"parent_turn_id"`
	Kind            string          `json:"kind"`
	EditOf          *string         `json:"edit_of_turn_id"`
	RerunOf         *string         `json:"rerun_of_turn_id"`
	RootID          string          `json:"root_turn_id"`
	Question        string          `json:"question"`
	Resolved        *string         `json:"resolved_question"`
	Language        *string         `json:"language"`
	Frame           json.RawMessage `json:"frame"`
	Resolution      json.RawMessage `json:"resolution"`
	State           string          `json:"state"`
	Outcome         *string         `json:"outcome"`
	Headline        *string         `json:"headline"`
	Shape           *string         `json:"shape"`
	RowCount        *int            `json:"row_count"`
	Query           *string         `json:"query,omitempty"`
	QueryHash       *string         `json:"query_hash"`
	ResultDigest    *string         `json:"result_digest"`
	Evidence        json.RawMessage `json:"evidence_refs"`
	SnapshotState   string          `json:"snapshot_state"`
	SnapshotBytes   *int            `json:"snapshot_bytes"`
	EngineVersion   *string         `json:"engine_version"`
	ModelID         *string         `json:"model"`
	PromptID        *string         `json:"prompt_id"`
	DataVersionHash *string         `json:"data_version_hash"`
	Timings         json.RawMessage `json:"timings"`
	DiffSummary     json.RawMessage `json:"diff_summary"`
	Failure         *string         `json:"error"`
	CreatedBy       string          `json:"created_by"`
	CreatedAt       time.Time       `json:"created_at"`
	StartedAt       time.Time       `json:"started_at"`
	CompletedAt     *time.Time      `json:"completed_at"`
	Reruns          RerunInfo       `json:"reruns"`
	Feedback        *FeedbackInfo   `json:"feedback,omitempty"`
	PinnedAs        []string        `json:"pinned_as,omitempty"`
	LiveTokens      bool            `json:"live_tokens"`
	Answer          json.RawMessage `json:"answer,omitempty"`
}

// A turn that has been "running" for longer than this was lost with the process that ran it (the longest a turn may take is ten minutes); it is read as interrupted.
const staleRunning = "15 minutes"

const turnColumns = `t.turn_id::text, t.thread_id::text, t.collection_id, t.idx, t.group_id::text, t.parent_turn_id::text, t.kind, t.edit_of_turn_id::text, t.rerun_of_turn_id::text, t.root_turn_id::text,
  t.question_text, t.resolved_text, t.language, t.frame_json, t.resolution_json,
  CASE WHEN t.state = 'running' AND t.started_at < now() - interval '` + staleRunning + `' THEN 'interrupted' ELSE t.state END,
  t.outcome, t.headline, t.shape, t.row_count, t.query_hash, t.result_digest, t.evidence_refs, t.snapshot_state, t.snapshot_bytes,
  t.engine_version, t.model_id, t.prompt_id, t.data_version_hash, t.timings_json, t.diff_summary, t.error_reason,
  t.created_by, t.created_at, t.started_at, t.completed_at, rr.n, rr.latest, rr.latest_changed`

const turnJoin = `LEFT JOIN LATERAL (SELECT count(*)::int AS n, (array_agg(r.turn_id::text ORDER BY r.idx DESC))[1] AS latest,
    (array_agg((r.diff_summary ->> 'changed')::boolean ORDER BY r.idx DESC))[1] AS latest_changed
  FROM forensic.conversation_turn r WHERE r.root_turn_id = t.root_turn_id AND r.kind = 'rerun' AND r.state = 'completed' AND r.idx > t.idx) rr ON true`

// loadTurns reads turns by a condition on alias t; $1 is the tenant, the caller's arguments start at $2.
func loadTurns(ctx context.Context, tx pgx.Tx, sc Scope, where string, args []any, order string, limit int) ([]Turn, error) {
	q := `SELECT ` + turnColumns + ` FROM forensic.conversation_turn t ` + turnJoin + ` WHERE t.tenant_id = $1 AND ` + where + ` ORDER BY ` + order
	if limit > 0 {
		q += ` LIMIT ` + strconv.Itoa(limit)
	}
	rows, err := tx.Query(ctx, q, append([]any{sc.TenantID}, args...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Turn{}
	for rows.Next() {
		var t Turn
		var frame, resolution, evidence, timings, diff []byte
		if err := rows.Scan(&t.ID, &t.ThreadID, &t.CaseID, &t.Index, &t.GroupID, &t.ParentID, &t.Kind, &t.EditOf, &t.RerunOf, &t.RootID,
			&t.Question, &t.Resolved, &t.Language, &frame, &resolution,
			&t.State, &t.Outcome, &t.Headline, &t.Shape, &t.RowCount, &t.QueryHash, &t.ResultDigest, &evidence, &t.SnapshotState, &t.SnapshotBytes,
			&t.EngineVersion, &t.ModelID, &t.PromptID, &t.DataVersionHash, &timings, &diff, &t.Failure,
			&t.CreatedBy, &t.CreatedAt, &t.StartedAt, &t.CompletedAt, &t.Reruns.Count, &t.Reruns.LatestTurnID, &t.Reruns.LatestChanged); err != nil {
			return nil, err
		}
		t.Frame, t.Resolution, t.Evidence, t.Timings, t.DiffSummary = frame, resolution, evidence, timings, diff
		t.CreatedAt, t.StartedAt, t.CompletedAt = utc(t.CreatedAt), utc(t.StartedAt), utcPtr(t.CompletedAt)
		if t.State == "interrupted" && t.Failure == nil {
			msg := "The service restarted or the request ended before the answer was saved. Run the question again."
			t.Failure = &msg
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// loadAnswer decodes the stored snapshot of a turn; nil if it has none.
func loadAnswer(ctx context.Context, tx pgx.Tx, turnID string, lim SnapshotLimits) (json.RawMessage, error) {
	var blob []byte
	if err := tx.QueryRow(ctx, `SELECT snapshot FROM forensic.conversation_turn WHERE turn_id = $1::uuid`, turnID).Scan(&blob); err != nil {
		return nil, err
	}
	if len(blob) == 0 {
		return nil, nil
	}
	raw, err := DecodeSnapshot(blob, lim)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(raw), nil
}

// objectTags are the names of the tags on one object, in alphabetical order.
func objectTags(ctx context.Context, tx pgx.Tx, kind, id string) ([]string, error) {
	rows, err := tx.Query(ctx, `SELECT g.name FROM forensic.tag_assignment a JOIN forensic.tag g ON g.tag_id = a.tag_id WHERE a.object_kind = $1 AND a.object_id = $2::uuid ORDER BY g.name_key`, kind, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}

// loadHistory reads the finished turns on the chain of parents above a turn, oldest first, at most n of them: the context a follow-up is read in.
func loadHistory(ctx context.Context, tx pgx.Tx, sc Scope, parentID string, n int) ([]HistoryTurn, error) {
	if parentID == "" || n <= 0 {
		return nil, nil
	}
	rows, err := tx.Query(ctx, `WITH RECURSIVE a AS (
		SELECT t.turn_id, t.parent_turn_id, 1 AS depth FROM forensic.conversation_turn t WHERE t.tenant_id = $1 AND t.turn_id = $2::uuid
		UNION ALL
		SELECT p.turn_id, p.parent_turn_id, a.depth + 1 FROM forensic.conversation_turn p JOIN a ON p.turn_id = a.parent_turn_id WHERE a.depth < 20)
		SELECT t.turn_id::text, t.question_text, coalesce(t.resolved_text, ''), coalesce(t.headline, ''), coalesce(t.outcome, ''), t.frame_json
		FROM a JOIN forensic.conversation_turn t ON t.turn_id = a.turn_id WHERE t.state = 'completed' ORDER BY a.depth LIMIT $3`, sc.TenantID, parentID, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var newestFirst []HistoryTurn
	for rows.Next() {
		var h HistoryTurn
		var frame []byte
		if err := rows.Scan(&h.TurnID, &h.Question, &h.Resolved, &h.Headline, &h.Outcome, &frame); err != nil {
			return nil, err
		}
		h.Frame = frame
		newestFirst = append(newestFirst, h)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]HistoryTurn, len(newestFirst))
	for i, h := range newestFirst {
		out[len(newestFirst)-1-i] = h
	}
	return out, nil
}

// GetTurn reads one turn with its answer, its feedback and the findings it was pinned to.
func (s *Service) GetTurn(ctx context.Context, sc Scope, turnID string, withAnswer bool) (*Turn, error) {
	var out Turn
	err := s.Store.InTx(ctx, sc, false, func(ctx context.Context, tx pgx.Tx) error {
		turns, err := loadTurns(ctx, tx, sc, `t.turn_id = $2::uuid`, []any{turnID}, "t.idx", 1)
		if err != nil {
			return err
		}
		if len(turns) == 0 {
			return NotFound("There is no such answer, or you may not see it.")
		}
		t := turns[0]
		if _, _, err := loadThread(ctx, tx, sc, t.ThreadID); err != nil { // a hidden conversation hides its turns from those who may not see it
			return err
		}
		var query *string
		if err := tx.QueryRow(ctx, `SELECT query_text FROM forensic.conversation_turn WHERE turn_id = $1::uuid`, turnID).Scan(&query); err != nil {
			return err
		}
		t.Query = query
		if withAnswer && t.State == "completed" {
			ans, err := loadAnswer(ctx, tx, turnID, s.Config.Snapshot)
			if err != nil {
				return err
			}
			t.Answer = ans
			t.LiveTokens = s.tokensLive(t)
		}
		if t.Feedback, err = feedbackInfo(ctx, tx, sc, turnID); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `SELECT finding_id::text FROM forensic.finding WHERE tenant_id = $1 AND source_turn_id = $2::uuid AND deleted_at IS NULL ORDER BY created_at`, sc.TenantID, turnID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			t.PinnedAs = append(t.PinnedAs, id)
		}
		rows.Close()
		out = t
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// Ancestry is the chain of turns above a turn (across the conversation it was forked from), oldest first, at most 20. Turns the caller may not see are left out.
func (s *Service) Ancestry(ctx context.Context, sc Scope, turnID string) ([]Turn, error) {
	var out []Turn
	err := s.Store.InTx(ctx, sc, false, func(ctx context.Context, tx pgx.Tx) error {
		turns, err := loadTurns(ctx, tx, sc, `t.turn_id IN (WITH RECURSIVE a AS (
			SELECT x.turn_id, x.parent_turn_id, 1 AS depth FROM forensic.conversation_turn x WHERE x.tenant_id = $1 AND x.turn_id = $2::uuid
			UNION ALL
			SELECT p.turn_id, p.parent_turn_id, a.depth + 1 FROM forensic.conversation_turn p JOIN a ON p.turn_id = a.parent_turn_id WHERE a.depth < 20) SELECT turn_id FROM a)`,
			[]any{turnID}, "t.created_at, t.idx", 0)
		if err != nil {
			return err
		}
		if len(turns) == 0 {
			return NotFound("There is no such answer, or you may not see it.")
		}
		// the answer asked about must be in a conversation the caller may open (a hidden one, or one of a case a bound credential is not bound to, is not); the chain above it is of the same case
		thread := ""
		for _, t := range turns {
			if t.ID == strings.ToLower(turnID) {
				thread = t.ThreadID
			}
		}
		if thread == "" {
			return NotFound("There is no such answer, or you may not see it.")
		}
		if _, _, err := loadThread(ctx, tx, sc, thread); err != nil {
			return err
		}
		out = turns
		return nil
	})
	return out, err
}

// ---- feedback ---------------------------------------------------------------------------------------------------------------------------------------------------------------

// Feedback is one person's verdict on one answer.
type Feedback struct {
	TurnID    string    `json:"turn_id"`
	Subject   string    `json:"subject"`
	Verdict   string    `json:"verdict"`
	Note      *string   `json:"note"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// FeedbackInfo is the caller's own verdict and the count of each verdict given by anyone.
type FeedbackInfo struct {
	Mine   *Feedback      `json:"mine"`
	Counts map[string]int `json:"counts"`
}

var feedbackVerdicts = map[string]bool{"correct": true, "wrong": true, "incomplete": true, "unclear": true}

func feedbackInfo(ctx context.Context, tx pgx.Tx, sc Scope, turnID string) (*FeedbackInfo, error) {
	info := &FeedbackInfo{Counts: map[string]int{}}
	rows, err := tx.Query(ctx, `SELECT verdict, count(*)::int FROM forensic.turn_feedback WHERE tenant_id = $1 AND turn_id = $2::uuid GROUP BY verdict`, sc.TenantID, turnID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var v string
		var n int
		if err := rows.Scan(&v, &n); err != nil {
			rows.Close()
			return nil, err
		}
		info.Counts[v] = n
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var f Feedback
	err = tx.QueryRow(ctx, `SELECT turn_id::text, subject_id, verdict, note, created_at, updated_at FROM forensic.turn_feedback WHERE tenant_id = $1 AND turn_id = $2::uuid AND subject_id = $3`,
		sc.TenantID, turnID, sc.Subject).Scan(&f.TurnID, &f.Subject, &f.Verdict, &f.Note, &f.CreatedAt, &f.UpdatedAt)
	switch {
	case err == nil:
		f.CreatedAt, f.UpdatedAt = utc(f.CreatedAt), utc(f.UpdatedAt)
		info.Mine = &f
	case err != pgx.ErrNoRows:
		return nil, err
	}
	return info, nil
}

// SetFeedback records (or changes) the caller's verdict on an answer.
func (s *Service) SetFeedback(ctx context.Context, sc Scope, turnID, verdict, note string) (*FeedbackInfo, error) {
	if !feedbackVerdicts[verdict] {
		return nil, BadRequest("verdict must be one of correct, wrong, incomplete, unclear.")
	}
	var notePtr *string
	if strings.TrimSpace(note) != "" {
		n, err := cleanBody("The note", note, 1, 2000)
		if err != nil {
			return nil, err
		}
		notePtr = &n
	}
	var out *FeedbackInfo
	err := s.Store.InTx(ctx, sc, true, func(ctx context.Context, tx pgx.Tx) error {
		var caseID, threadID string
		if err := tx.QueryRow(ctx, `SELECT collection_id, thread_id::text FROM forensic.conversation_turn WHERE tenant_id = $1 AND turn_id = $2::uuid`, sc.TenantID, turnID).Scan(&caseID, &threadID); err != nil {
			return asError(err)
		}
		if _, _, err := loadThread(ctx, tx, sc, threadID); err != nil {
			return err
		}
		role, err := requireRole(ctx, tx, sc, caseID)
		if err != nil {
			return err
		}
		if !role.CanGiveFeedback() {
			return Forbidden("Your role in this case (%s) does not allow giving feedback.", role)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO forensic.turn_feedback (turn_id, tenant_id, collection_id, subject_id, verdict, note) VALUES ($1::uuid, $2, $3, $4, $5, $6)
			ON CONFLICT (turn_id, subject_id) DO UPDATE SET verdict = EXCLUDED.verdict, note = EXCLUDED.note, updated_at = now()`, turnID, sc.TenantID, caseID, sc.Subject, verdict, notePtr); err != nil {
			return err
		}
		if err := audit(ctx, tx, sc, role, caseID, "turn.feedback", "turn", turnID, map[string]any{"verdict": verdict, "with_note": notePtr != nil}); err != nil {
			return err
		}
		out, err = feedbackInfo(ctx, tx, sc, turnID)
		return err
	})
	return out, err
}

// ListFeedback lists everyone's feedback on an answer to those who run the case (owner, lead, auditor, an open case) and to the owner of the conversation; anyone else gets nothing here and sees the counts on the turn.
func (s *Service) ListFeedback(ctx context.Context, sc Scope, turnID string) ([]Feedback, error) {
	out := []Feedback{}
	err := s.Store.InTx(ctx, sc, false, func(ctx context.Context, tx pgx.Tx) error {
		var caseID, threadID string
		if err := tx.QueryRow(ctx, `SELECT collection_id, thread_id::text FROM forensic.conversation_turn WHERE tenant_id = $1 AND turn_id = $2::uuid`, sc.TenantID, turnID).Scan(&caseID, &threadID); err != nil {
			return asError(err)
		}
		t, _, err := loadThread(ctx, tx, sc, threadID)
		if err != nil {
			return err
		}
		if !(t.IsOwner || t.roles[caseID].CanReadAudit() || sc.IsAdmin()) { // the role in the case of the answer, which in a conversation across cases is one of its cases
			return Forbidden("Only the owner of the conversation and those who run the case can read everyone's feedback.")
		}
		rows, err := tx.Query(ctx, `SELECT turn_id::text, subject_id, verdict, note, created_at, updated_at FROM forensic.turn_feedback WHERE tenant_id = $1 AND turn_id = $2::uuid ORDER BY created_at`, sc.TenantID, turnID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var f Feedback
			if err := rows.Scan(&f.TurnID, &f.Subject, &f.Verdict, &f.Note, &f.CreatedAt, &f.UpdatedAt); err != nil {
				return err
			}
			f.CreatedAt, f.UpdatedAt = utc(f.CreatedAt), utc(f.UpdatedAt)
			out = append(out, f)
		}
		return rows.Err()
	})
	return out, err
}

// ---- comparing two runs ------------------------------------------------------------------------------------------------------------------------------------------------------

// loadRun reads what the diff needs of a finished turn.
func loadRun(ctx context.Context, tx pgx.Tx, sc Scope, turnID string) (Run, string, error) {
	var r Run
	var caseID, state string
	var fig, ev []byte
	err := tx.QueryRow(ctx, `SELECT turn_id::text, collection_id, state, coalesce(outcome, ''), coalesce(headline, ''), figures_json, evidence_refs, coalesce(query_hash, ''), coalesce(data_version_hash, ''), completed_at
		FROM forensic.conversation_turn WHERE tenant_id = $1 AND turn_id = $2::uuid`, sc.TenantID, turnID).
		Scan(&r.TurnID, &caseID, &state, &r.Outcome, &r.Headline, &fig, &ev, &r.QueryHash, &r.DataVersionHash, &r.CompletedAt)
	if err != nil {
		return r, "", asError(err)
	}
	r.CompletedAt = utcPtr(r.CompletedAt)
	if state != "completed" {
		return r, caseID, Conflict("not_finished", "That answer is not finished (it is %s), so it cannot be compared.", state)
	}
	if len(fig) > 0 {
		dec := json.NewDecoder(bytes.NewReader(fig))
		dec.UseNumber() // a long number in a cell must not lose digits on the way back
		_ = dec.Decode(&r.Figures)
	}
	if len(ev) > 0 {
		_ = json.Unmarshal(ev, &r.Evidence)
	}
	return r, caseID, nil
}

// DiffTurns compares a turn with another run of the same case: by default the run it re-ran, or the run before it in the same chain.
func (s *Service) DiffTurns(ctx context.Context, sc Scope, turnID, againstID string) (*Diff, error) {
	var out *Diff
	err := s.Store.InTx(ctx, sc, false, func(ctx context.Context, tx pgx.Tx) error {
		after, caseID, err := loadRun(ctx, tx, sc, turnID)
		if err != nil {
			return err
		}
		var threadID string
		if err := tx.QueryRow(ctx, `SELECT thread_id::text FROM forensic.conversation_turn WHERE turn_id = $1::uuid`, turnID).Scan(&threadID); err != nil {
			return err
		}
		if _, _, err := loadThread(ctx, tx, sc, threadID); err != nil {
			return err
		}
		if againstID == "" {
			err := tx.QueryRow(ctx, `SELECT coalesce(t.rerun_of_turn_id, (SELECT p.turn_id FROM forensic.conversation_turn p WHERE p.root_turn_id = t.root_turn_id AND p.idx < t.idx AND p.state = 'completed' ORDER BY p.idx DESC LIMIT 1))::text
				FROM forensic.conversation_turn t WHERE t.turn_id = $1::uuid`, turnID).Scan(&againstID)
			if err != nil || againstID == "" {
				return BadRequest("There is no earlier run of this question to compare with; name one with against=.")
			}
		}
		before, otherCase, err := loadRun(ctx, tx, sc, againstID)
		if err != nil {
			return err
		}
		if otherCase != caseID {
			return BadRequest("The two answers are from different cases and cannot be compared.")
		}
		dvBefore, err := loadDataVersion(ctx, tx, sc.TenantID, caseID, before.DataVersionHash)
		if err != nil {
			return err
		}
		dvAfter, err := loadDataVersion(ctx, tx, sc.TenantID, caseID, after.DataVersionHash)
		if err != nil {
			return err
		}
		out = Compare(before, after, dvBefore, dvAfter)
		return nil
	})
	return out, err
}
