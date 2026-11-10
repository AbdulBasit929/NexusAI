package caselayer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// RunParams is a question for a conversation. Send parent_turn_id to answer below a particular earlier turn, or edit_of_turn_id to ask an earlier question again in other words (a branch);
// send neither to continue after the last turn. client_request_id makes a retry harmless: the same id returns the turn that already exists.
type RunParams struct {
	Question        string
	ClientRequestID string
	ParentTurnID    string
	EditOfTurnID    string
	Language        string
}

// RunResult is a turn that ran (or, for a repeated client_request_id, the one that already existed) and, for a re-run, the comparison with the run it re-ran. A question put to a conversation
// across cases has no single turn: Group holds the turn of each case and Turn is nil.
type RunResult struct {
	Turn     *Turn
	Diff     *Diff
	Group    *Group
	Existing bool
}

type runSpec struct {
	Kind        string // ask | edit | rerun
	Stored      string // the question as stored: national ID numbers masked
	Send        string // the question to send when nothing resolves it
	NICCount    int
	ParentID    string
	ParentSet   bool
	EditOf      string
	Rerun       *Turn
	ClientReqID string
	Language    string
	CaseID      string    // the case the turn is asked in: required in a conversation across cases (one of its cases), and then the case of the earlier turn when re-running; optional otherwise
	GroupID     string    // the group of turns of one question put to several cases
	Deadline    time.Time // when the whole group must be done; zero for a turn that has only its own limit
}

var (
	clientRequestRE = regexp.MustCompile(`^[A-Za-z0-9._:\-]{1,100}$`)
	languageRE      = regexp.MustCompile(`^[a-z][a-z_\-]{1,19}$`)
)

const staleMessage = "The service restarted or the request ended before the answer was saved. Run the question again."

func checkUUID(name, v string) error {
	if _, err := uuid.Parse(v); err != nil {
		return BadRequest("%s is not a valid identifier.", name)
	}
	return nil
}

func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// lockedSink serialises the events of one turn: the heartbeat, the engine's own phases and the runner write from different goroutines.
type lockedSink struct {
	mu    sync.Mutex
	inner EventSink
}

func (l *lockedSink) Send(e Event) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.inner.Send(e)
}

// RunTurn puts a question to the engine in a conversation and stores the answer. See the design note, section 3.
func (s *Service) RunTurn(ctx context.Context, sc Scope, threadID string, p RunParams, sink EventSink) (*RunResult, error) {
	q, err := cleanBody("The question", p.Question, 1, 4000)
	if err != nil {
		return nil, err
	}
	if p.ClientRequestID != "" && !clientRequestRE.MatchString(p.ClientRequestID) {
		return nil, BadRequest("client_request_id must be 1 to 100 letters, digits or the characters . _ : -")
	}
	lang := strings.ToLower(strings.TrimSpace(p.Language))
	if lang != "" && !languageRE.MatchString(lang) {
		return nil, BadRequest("language must be a short code such as en, ur or roman_ur.")
	}
	parent, edit := strings.TrimSpace(p.ParentTurnID), strings.TrimSpace(p.EditOfTurnID)
	if parent != "" && edit != "" {
		return nil, BadRequest("Send parent_turn_id or edit_of_turn_id, not both.")
	}
	for name, v := range map[string]string{"parent_turn_id": parent, "edit_of_turn_id": edit} {
		if v != "" {
			if err := checkUUID(name, v); err != nil {
				return nil, err
			}
		}
	}
	stored, n := MaskNationalIDs(q)
	spec := runSpec{Kind: "ask", Stored: stored, Send: q, NICCount: n, ParentID: parent, ParentSet: parent != "", EditOf: edit, ClientReqID: p.ClientRequestID, Language: lang}
	if edit != "" {
		spec.Kind = "edit"
	}
	// a conversation across cases is asked of each of its cases: peek at the thread (the same read run() makes) to see which kind this is
	if s.Engine == nil {
		return nil, Unavailable("No question engine is configured.")
	}
	if err := checkUUID("The conversation id", threadID); err != nil {
		return nil, err
	}
	cross := false
	if err := s.Store.InTx(ctx, sc, false, func(ctx context.Context, tx pgx.Tx) error {
		t, _, lerr := loadThread(ctx, tx, sc, threadID)
		cross = t.CrossCase
		return lerr
	}); err != nil {
		return nil, err
	}
	if cross {
		return s.runGroup(ctx, sc, threadID, spec, sink)
	}
	return s.run(ctx, sc, threadID, spec, sink)
}

// Rerun asks an earlier question again against the data as it is now, as a new turn of the same conversation, and compares the new answer with the one it re-ran. The question sent is the one
// the earlier run sent (a follow-up is not resolved again). A question that named a national ID number cannot be re-run: it is stored masked.
func (s *Service) Rerun(ctx context.Context, sc Scope, turnID string, sink EventSink) (*RunResult, error) {
	if err := checkUUID("turn id", turnID); err != nil {
		return nil, err
	}
	var src Turn
	err := s.Store.InTx(ctx, sc, false, func(ctx context.Context, tx pgx.Tx) error {
		turns, err := loadTurns(ctx, tx, sc, `t.turn_id = $2::uuid`, []any{turnID}, "t.idx", 1)
		if err != nil {
			return err
		}
		if len(turns) == 0 {
			return NotFound("There is no such answer, or you may not see it.")
		}
		src = turns[0]
		return nil
	})
	if err != nil {
		return nil, err
	}
	var frame struct {
		NationalIDCount int `json:"national_id_count"`
	}
	_ = json.Unmarshal(src.Frame, &frame)
	if frame.NationalIDCount > 0 {
		return nil, Conflict("withheld", "That question named a national ID number, which is withheld; ask it again without the number.")
	}
	send := src.Question
	if src.Resolved != nil && *src.Resolved != "" {
		send = *src.Resolved
	}
	parent := ""
	if src.ParentID != nil {
		parent = *src.ParentID
	}
	lang := ""
	if src.Language != nil {
		lang = *src.Language
	}
	return s.run(ctx, sc, src.ThreadID, runSpec{Kind: "rerun", Stored: src.Question, Send: send, ParentID: parent, ParentSet: true, Rerun: &src, Language: lang, CaseID: src.CaseID}, sink)
}

type prepared struct {
	thread     string
	existingID string
	turnID     string
	caseID     string
	role       Role
	idx        int
	parent     string
	history    []HistoryTurn
	dv         DataVersion
}

func (s *Service) run(ctx context.Context, sc Scope, threadID string, spec runSpec, sink EventSink) (*RunResult, error) {
	if s.Engine == nil {
		return nil, Unavailable("No question engine is configured.")
	}
	if err := checkUUID("The conversation id", threadID); err != nil {
		return nil, err
	}
	if sink == nil {
		sink = nopSink{}
	}
	sink = &lockedSink{inner: sink}
	ph := newPhases(sink, threadID, "")

	var pr prepared
	err := s.Store.InTx(ctx, sc, true, func(ctx context.Context, tx pgx.Tx) error {
		t, role, err := loadThread(ctx, tx, sc, threadID)
		if err != nil {
			return err
		}
		// the case of this turn: the one the thread is about; in a conversation across cases the one it is asked in, and then the role that counts is the role in that case
		caseID := t.CaseID
		switch {
		case t.CrossCase:
			found := false
			for _, c := range t.CaseIDs {
				found = found || c == spec.CaseID
			}
			if !found {
				return BadRequest("This conversation is across cases; the question must be asked in one of them.")
			}
			caseID, role = spec.CaseID, t.roles[spec.CaseID]
		case spec.CaseID != "" && spec.CaseID != t.CaseID:
			return BadRequest("That answer belongs to another case than this conversation.")
		}
		pr.thread, pr.caseID, pr.role = threadID, caseID, role
		switch {
		case !t.IsOwner:
			return Forbidden("Only the owner of a conversation can add a question to it; fork it to continue in your own conversation.")
		case t.Hidden:
			return Conflict("hidden", "This conversation is hidden; restore it first.")
		case t.Archived:
			return Conflict("archived", "This conversation is archived; restore it to continue.")
		case !role.CanContribute():
			return Forbidden("Your role in this case (%s) does not allow asking questions.", role)
		}
		if spec.ClientReqID != "" {
			err := tx.QueryRow(ctx, `SELECT turn_id::text FROM forensic.conversation_turn WHERE thread_id = $1::uuid AND client_request_id = $2`, threadID, spec.ClientReqID).Scan(&pr.existingID)
			if err == nil {
				return nil
			}
			if err != pgx.ErrNoRows {
				return err
			}
		}
		// a turn that has been "running" for longer than a turn may take was lost with its process
		if _, err := tx.Exec(ctx, `UPDATE forensic.conversation_turn SET state = 'interrupted', error_reason = $2, completed_at = now()
			WHERE thread_id = $1::uuid AND state = 'running' AND started_at < now() - interval '`+staleRunning+`'`, threadID, staleMessage); err != nil {
			return err
		}
		parent := spec.ParentID
		switch {
		case spec.Kind == "rerun":
		case spec.EditOf != "":
			var inThread string
			var up *string
			if err := tx.QueryRow(ctx, `SELECT thread_id::text, parent_turn_id::text FROM forensic.conversation_turn WHERE tenant_id = $1 AND turn_id = $2::uuid`, sc.TenantID, spec.EditOf).Scan(&inThread, &up); err != nil {
				return NotFound("There is no such question to edit.")
			}
			if inThread != threadID {
				return BadRequest("The question to edit belongs to another conversation.")
			}
			parent = ""
			if up != nil {
				parent = *up
			}
		case spec.ParentSet:
			var ok bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM forensic.conversation_turn WHERE tenant_id = $1 AND turn_id = $2::uuid AND (thread_id = $3::uuid OR turn_id = $4::uuid))`,
				sc.TenantID, parent, threadID, nilIfEmpty(forkTurnID(t))).Scan(&ok); err != nil {
				return err
			}
			if !ok {
				return BadRequest("parent_turn_id must be a turn of this conversation.")
			}
		default:
			// after the last turn of the conversation; in a conversation across cases, after the last turn of THIS case (the context of a follow-up is the answers of the same case)
			onlyCase := ""
			if t.CrossCase {
				onlyCase = caseID
			}
			err := tx.QueryRow(ctx, `SELECT turn_id::text FROM forensic.conversation_turn WHERE thread_id = $1::uuid AND ($2 = '' OR collection_id = $2) ORDER BY idx DESC LIMIT 1`,
				threadID, onlyCase).Scan(&parent)
			if err == pgx.ErrNoRows {
				parent = forkTurnID(t)
			} else if err != nil {
				return err
			}
		}
		pr.parent = parent
		if pr.history, err = loadHistory(ctx, tx, sc, parent, s.Config.HistoryTurns); err != nil {
			return err
		}
		if pr.dv, err = readDataVersion(ctx, tx, sc.TenantID, caseID); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `UPDATE forensic.conversation_thread SET turn_count = turn_count + 1, first_question = coalesce(first_question, $3), last_question = $3, last_turn_at = now(), updated_at = now()
			WHERE thread_id = $1::uuid AND tenant_id = $2 RETURNING turn_count`, threadID, sc.TenantID, clip(spec.Stored, 500)).Scan(&pr.idx); err != nil {
			return err
		}
		pr.turnID = uuid.NewString()
		root := pr.turnID
		var rerunOf string
		if spec.Rerun != nil {
			root, rerunOf = spec.Rerun.RootID, spec.Rerun.ID
		}
		if _, err := tx.Exec(ctx, `INSERT INTO forensic.conversation_turn (turn_id, tenant_id, collection_id, thread_id, idx, parent_turn_id, kind, edit_of_turn_id, rerun_of_turn_id, root_turn_id,
				question_text, language, state, created_by, client_request_id, group_id)
			VALUES ($1::uuid, $2, $3, $4::uuid, $5, $6::uuid, $7, $8::uuid, $9::uuid, $10::uuid, $11, $12, 'running', $13, $14, $15::uuid)`,
			pr.turnID, sc.TenantID, caseID, threadID, pr.idx, nilIfEmpty(parent), spec.Kind, nilIfEmpty(spec.EditOf), nilIfEmpty(rerunOf), root, spec.Stored, nilIfEmpty(spec.Language), sc.Subject, nilIfEmpty(spec.ClientReqID), nilIfEmpty(spec.GroupID)); err != nil {
			return err
		}
		details := map[string]any{"thread": threadID, "kind": spec.Kind, "idx": pr.idx, "rerun_of": rerunOf, "edit_of": spec.EditOf}
		if spec.GroupID != "" {
			details["group"] = spec.GroupID
		}
		return audit(ctx, tx, sc, role, caseID, "turn.started", "turn", pr.turnID, details)
	})
	if err != nil {
		return nil, err
	}
	if pr.existingID != "" {
		turn, err := s.GetTurn(ctx, sc, pr.existingID, true)
		if err != nil {
			return nil, err
		}
		return &RunResult{Turn: turn, Existing: true}, nil
	}

	ph.turn = pr.turnID
	ph.phase("accepted", "Question received")
	finished := false
	defer func() {
		if !finished { // an early return or a panic must not leave the turn "running"
			s.failTurn(sc, pr, "The answer could not be completed.")
		}
	}()

	runCtx, cancel := turnContext(ctx, s.Config.TurnTimeout, spec.Deadline)
	defer cancel()

	// the follow-up: the question the engine will answer. resolvedStored is what is kept of it: only a question the layer itself holds in clean form (never one that may carry a national ID number).
	send, resolvedStored, resolution := spec.Send, "", Resolution{Method: "none"}
	switch {
	case spec.Rerun != nil:
		resolution = Resolution{Method: "reused"}
		if send != spec.Stored {
			resolvedStored = send
		}
	case s.FollowUp != nil && len(pr.history) > 0 && spec.NICCount == 0:
		ph.phase("context", "Reading the earlier questions of this conversation")
		var resolved string
		resolved, resolution = applyFollowUp(runCtx, s.FollowUp, spec.Stored, pr.history)
		if resolution.Method == "model" {
			send, resolvedStored = resolved, resolved
		}
	}

	queuedAt := time.Now()
	if !s.slots.acquire(runCtx, ph.queued) {
		return s.finishTurn(ctx, sc, pr, spec, resolution, resolvedStored, EngineResult{}, fmt.Errorf("timed out waiting for the model"), time.Since(queuedAt), 0, sink, ph, &finished)
	}
	queued := time.Since(queuedAt)
	engineStart := time.Now()
	ph.phase("engine", "Answering")
	stop, beatsDone := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(beatsDone)
		tick := time.NewTicker(s.Config.Heartbeat)
		defer tick.Stop()
		for {
			select {
			case <-tick.C:
				sink.Send(Event{Event: "heartbeat", TurnID: pr.turnID, ThreadID: threadID, ElapsedMS: time.Since(engineStart).Milliseconds(), AtMS: ph.at()})
			case <-stop:
				return
			}
		}
	}()
	engineCtx := WithPhaseEmitter(runCtx, func(phase, label string) { ph.phase("engine."+phase, label) })
	res, askErr := s.Engine.Ask(engineCtx, EngineRequest{Scope: sc, CaseID: pr.caseID, ThreadID: threadID, TurnID: pr.turnID, Question: send, Original: spec.Send, History: pr.history, RequestID: pr.turnID})
	close(stop)
	<-beatsDone // no heartbeat after the answer: the last events of the stream are the turn and done
	s.slots.release()
	return s.finishTurn(ctx, sc, pr, spec, resolution, resolvedStored, res, askErr, queued, time.Since(engineStart), sink, ph, &finished)
}

// turnContext is the context a turn runs on: detached from the request (a client that went away does not stop it), limited to the time a turn may take, and, in a question put to several cases,
// to the time left of the whole question.
func turnContext(ctx context.Context, limit time.Duration, deadline time.Time) (context.Context, context.CancelFunc) {
	end := time.Now().Add(limit)
	if !deadline.IsZero() && deadline.Before(end) {
		end = deadline
	}
	return context.WithDeadline(context.WithoutCancel(ctx), end)
}

func forkTurnID(t Thread) string {
	if t.ForkedFrom != nil {
		return t.ForkedFrom.TurnID
	}
	return ""
}

// failTurn marks a turn failed when the runner ended without finishing it. Best effort.
func (s *Service) failTurn(sc Scope, pr prepared, reason string) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = s.Store.InTx(ctx, sc, true, func(ctx context.Context, tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE forensic.conversation_turn SET state = 'failed', outcome = 'failed', error_reason = $2, completed_at = now() WHERE turn_id = $1::uuid AND state = 'running'`, pr.turnID, reason)
		if err != nil || tag.RowsAffected() == 0 {
			return err
		}
		return audit(ctx, tx, sc, pr.role, pr.caseID, "turn.failed", "turn", pr.turnID, map[string]any{"reason": reason})
	})
}

// engineFailure is the sentence for a turn whose engine call failed.
func engineFailure(err error, res EngineResult) string {
	switch {
	case err != nil:
		var e *Error
		if errors.As(err, &e) {
			return e.Message
		}
		if strings.Contains(err.Error(), "timed out") || strings.Contains(err.Error(), "deadline") {
			return "The question took longer than the time allowed, so it was stopped. Narrow it by a number or a period and ask again."
		}
		return "The question engine could not be reached."
	case len(res.Response) > 0:
		var body struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(res.Response, &body) == nil && strings.TrimSpace(body.Error) != "" {
			return fmt.Sprintf("The engine refused the question: %s", clip(body.Error, 300))
		}
	}
	return fmt.Sprintf("The question engine answered with an error (HTTP %d).", res.Status)
}

// mergeFrame adds to the engine's frame what the layer knows: the count of national ID numbers the question named (never the numbers).
func mergeFrame(raw json.RawMessage, nicCount int) json.RawMessage {
	m := map[string]any{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &m); err != nil || m == nil {
			m = map[string]any{}
		}
	}
	if nicCount > 0 {
		if have, ok := m["national_id_count"].(float64); !ok || int(have) < nicCount {
			m["national_id_count"] = nicCount
		}
	}
	out, _ := json.Marshal(m)
	return out
}

// finishTurn stores the outcome of a turn that was accepted: the answer, or the failure. It runs on a context of its own, so a client that went away does not lose the answer.
func (s *Service) finishTurn(ctx context.Context, sc Scope, pr prepared, spec runSpec, resolution Resolution, resolvedStored string, res EngineResult, askErr error, queued, engineTook time.Duration, sink EventSink, ph *phases, finished *bool) (*RunResult, error) {
	ph.phase("saving", "Saving the answer")
	var d *Digest
	failure := ""
	switch {
	case askErr != nil:
		failure = engineFailure(askErr, res)
	case res.Status != 0 && res.Status != 200:
		failure = engineFailure(nil, res)
	default:
		var derr error
		if d, derr = ExtractDigest(res.Response); derr != nil {
			failure = "The engine's answer could not be read."
		} else {
			// what the layer derives and stores as text (the sentence, the words searched) never carries a national ID number, whatever path answered
			d.Headline, d.AnswerText = scrubText(d.Headline), scrubText(d.AnswerText)
		}
	}
	var snap Snapshot
	if failure == "" {
		var err error
		if snap, err = BuildSnapshot(res.Response, s.Config.Snapshot); err != nil {
			snap = Snapshot{State: SnapshotNone}
		}
	} else {
		snap = Snapshot{State: SnapshotNone}
	}
	frame := json.RawMessage(scrubText(string(mergeFrame(res.Frame, spec.NICCount))))
	resolutionJSON, _ := json.Marshal(resolution)
	engineVersion := res.EngineVersion
	if engineVersion == "" {
		engineVersion = s.Config.EngineVersion
	}

	var result RunResult
	finCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	err := s.Store.InTx(finCtx, sc, true, func(ctx context.Context, tx pgx.Tx) error {
		dvHash, err := storeDataVersion(ctx, tx, sc.TenantID, pr.caseID, pr.dv)
		if err != nil {
			return err
		}
		state, outcome := "completed", OutcomeFailed
		var figuresJSON, evidenceJSON, diffJSON any
		evidenceJSON = "[]"
		var headline, answerText, shape, query, queryHash, resultDigest, model, prompt any
		var rowCount any
		var diff *Diff
		if failure != "" {
			state = "failed"
		} else {
			outcome = d.Outcome
			headline, answerText, shape = nilIfEmpty(d.Headline), nilIfEmpty(d.AnswerText), nilIfEmpty(d.Shape)
			query, queryHash = nilIfEmpty(d.Query), nilIfEmpty(QueryHash(d.Query))
			resultDigest, model, prompt = nilIfEmpty(d.ResultDigest()), nilIfEmpty(d.Model), nilIfEmpty(d.PromptID)
			rowCount = d.RowCount
			if raw, err := json.Marshal(d.Figures()); err == nil {
				figuresJSON = string(raw)
			}
			if raw, err := json.Marshal(d.Evidence); err == nil && d.Evidence != nil {
				evidenceJSON = string(raw)
			}
			if spec.Rerun != nil {
				now := time.Now()
				before, _, lerr := loadRun(ctx, tx, sc, spec.Rerun.ID)
				if lerr == nil {
					dvBefore, derr := loadDataVersion(ctx, tx, sc.TenantID, pr.caseID, before.DataVersionHash)
					if derr != nil {
						return derr
					}
					after := Run{TurnID: pr.turnID, Outcome: d.Outcome, Headline: d.Headline, Figures: d.Figures(), Evidence: d.Evidence, QueryHash: QueryHash(d.Query), DataVersionHash: dvHash, CompletedAt: &now}
					diff = Compare(before, after, dvBefore, &pr.dv)
					summary, _ := json.Marshal(map[string]any{"against": before.TurnID, "changed": diff.Changed, "figures_changed": diff.FiguresChanged, "evidence_changed": diff.EvidenceChanged,
						"data_changed": diff.Data.Changed, "same_data_different_answer": diff.SameDataDifferentAnswer, "complete": diff.Complete})
					diffJSON = string(summary)
				}
			}
		}
		timings, _ := json.Marshal(map[string]any{"total_ms": (queued + engineTook).Milliseconds(), "queued_ms": queued.Milliseconds(), "engine_ms": engineTook.Milliseconds(),
			"model_ms": digestMS(d, func(x *Digest) int64 { return x.ModelMS }), "exec_ms": digestMS(d, func(x *Digest) int64 { return x.ExecMS }), "attempts": digestMS(d, func(x *Digest) int64 { return int64(x.Attempts) })})
		var failureArg any
		if failure != "" {
			failureArg = failure
		}
		var snapBlob any
		var snapBytes, snapSHA any
		if snap.State != SnapshotNone {
			snapBlob, snapBytes, snapSHA = snap.Blob, snap.Bytes, snap.SHA256
		}
		tag, err := tx.Exec(ctx, `UPDATE forensic.conversation_turn SET
			resolved_text = $2, frame_json = $3::jsonb, resolution_json = $4::jsonb, state = $5, outcome = $6, headline = $7, answer_text = $8, shape = $9, row_count = $10,
			query_text = $11, query_hash = $12, result_digest = $13, figures_json = $14::jsonb, evidence_refs = $15::jsonb, snapshot = $16, snapshot_state = $17, snapshot_bytes = $18, snapshot_sha256 = $19,
			engine_version = $20, model_id = $21, prompt_id = $22, data_version_hash = $23, timings_json = $24::jsonb, diff_summary = $25::jsonb, error_reason = $26, request_id = $27, completed_at = now()
			WHERE turn_id = $1::uuid AND state = 'running'`,
			pr.turnID, nilIfEmpty(resolvedStored), string(frame), string(resolutionJSON), state, outcome, headline, answerText, shape, rowCount,
			query, queryHash, resultDigest, figuresJSON, evidenceJSON, snapBlob, snap.State, snapBytes, snapSHA,
			nilIfEmpty(engineVersion), model, prompt, dvHash, string(timings), diffJSON, failureArg, nilIfEmpty(res.RequestID))
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return Conflict("immutable", "The turn was already finished by another request.")
		}
		var headlineText, outcomeText string
		if d != nil {
			headlineText = d.Headline
		}
		outcomeText = outcome
		if _, err := tx.Exec(ctx, `UPDATE forensic.conversation_thread SET last_headline = nullif($3, ''), last_outcome = $4, updated_at = now() WHERE thread_id = $1::uuid AND tenant_id = $2`,
			pr.thread, sc.TenantID, clip(headlineText, 500), outcomeText); err != nil {
			return err
		}
		action := "turn.completed"
		if failure != "" {
			action = "turn.failed"
		}
		if err := audit(ctx, tx, sc, pr.role, pr.caseID, action, "turn", pr.turnID, map[string]any{"outcome": outcome, "snapshot": snap.State, "data_version": dvHash, "rows": rowCount, "reason": failure}); err != nil {
			return err
		}
		turns, err := loadTurns(ctx, tx, sc, `t.turn_id = $2::uuid`, []any{pr.turnID}, "t.idx", 1)
		if err != nil {
			return err
		}
		if len(turns) == 0 {
			return NotFound("The turn disappeared.")
		}
		t := turns[0]
		if d != nil {
			q := d.Query
			if q != "" {
				t.Query = &q
			}
			t.Answer = res.Response
			t.LiveTokens = true
		}
		result = RunResult{Turn: &t, Diff: diff}
		return nil
	})
	*finished = true
	if err != nil {
		*finished = false
		return nil, err
	}
	if failure != "" {
		sink.Send(Event{Event: "error", Code: "engine_failed", Message: failure, TurnID: pr.turnID, ThreadID: pr.thread, Turn: result.Turn, AtMS: ph.at()})
	} else {
		sink.Send(Event{Event: "turn", TurnID: pr.turnID, ThreadID: pr.thread, Turn: result.Turn, Diff: result.Diff, AtMS: ph.at()})
	}
	sink.Send(Event{Event: "done", TurnID: pr.turnID, ThreadID: pr.thread, AtMS: ph.at()})
	return &result, nil
}

func digestMS(d *Digest, f func(*Digest) int64) int64 {
	if d == nil {
		return 0
	}
	return f(d)
}
