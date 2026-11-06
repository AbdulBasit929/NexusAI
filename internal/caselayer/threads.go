package caselayer

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// ForkRef says which turn of which thread a conversation was forked from.
type ForkRef struct {
	ThreadID string `json:"thread_id"`
	TurnID   string `json:"turn_id"`
}

// Match is a turn of a thread that a search found.
type Match struct {
	TurnID   string  `json:"turn_id"`
	Index    int     `json:"idx"`
	Question string  `json:"question"`
	Headline *string `json:"headline"`
}

// Thread is one conversation about a case, or across several (CrossCase: then CaseID is empty and CaseIDs lists them, sorted). Turns is filled by Get only.
type Thread struct {
	ID            string     `json:"thread_id"`
	CaseID        string     `json:"case_id"`
	CaseIDs       []string   `json:"case_ids"`
	CrossCase     bool       `json:"cross_case"`
	Title         *string    `json:"title"`
	DisplayTitle  string     `json:"display_title"`
	TitleSource   string     `json:"title_source"`
	Pinned        bool       `json:"pinned"`
	Archived      bool       `json:"archived"`
	Visibility    string     `json:"visibility"`
	SharedWith    []string   `json:"shared_with"`
	Owner         string     `json:"owner"`
	IsOwner       bool       `json:"is_owner"`
	TurnCount     int        `json:"turn_count"`
	FirstQuestion *string    `json:"first_question"`
	LastQuestion  *string    `json:"last_question"`
	LastHeadline  *string    `json:"last_headline"`
	LastOutcome   *string    `json:"last_outcome"`
	LastTurnAt    *time.Time `json:"last_turn_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	ForkedFrom    *ForkRef   `json:"forked_from"`
	Hidden        bool       `json:"hidden"`
	HiddenAt      *time.Time `json:"hidden_at,omitempty"`
	HiddenBy      *string    `json:"hidden_by,omitempty"`
	HideReason    *string    `json:"hide_reason,omitempty"`
	ChangedSince  bool       `json:"changed_since"`
	Tags          []string   `json:"tags"`
	Matches       []Match    `json:"matches,omitempty"`
	ActiveTurnID  *string    `json:"active_turn_id,omitempty"`
	Turns         []Turn     `json:"turns,omitempty"`

	// roles is what the caller is in each case of the thread, read when the thread was loaded; never sent.
	roles map[string]Role
}

// threadColumns are read in this order by threadDest.
const threadColumns = `t.thread_id::text, coalesce(t.collection_id, ''), t.scope_collection_ids, t.title, t.title_source, t.pinned, t.archived, t.visibility, t.shared_with, t.owner_subject, t.turn_count,
  t.first_question, t.last_question, t.last_headline, t.last_outcome, t.last_turn_at, t.created_at, t.updated_at, t.forked_from_thread_id::text, t.forked_from_turn_id::text,
  t.hidden_at, t.hidden_by, t.hide_reason,
  EXISTS (SELECT 1 FROM (SELECT DISTINCT ON (r.root_turn_id) (r.diff_summary ->> 'changed')::boolean AS changed FROM forensic.conversation_turn r
          WHERE r.thread_id = t.thread_id AND r.kind = 'rerun' AND r.state = 'completed' ORDER BY r.root_turn_id, r.idx DESC) x WHERE x.changed)`

// threadScan holds what a row of threadColumns is read into before decorateThread puts it in a Thread.
type threadScan struct {
	forkThread, forkTurn *string
	scope                []string
}

// threadDest lists where each column of threadColumns is read, in order.
func threadDest(t *Thread, s *threadScan) []any {
	return []any{&t.ID, &t.CaseID, &s.scope, &t.Title, &t.TitleSource, &t.Pinned, &t.Archived, &t.Visibility, &t.SharedWith, &t.Owner, &t.TurnCount,
		&t.FirstQuestion, &t.LastQuestion, &t.LastHeadline, &t.LastOutcome, &t.LastTurnAt, &t.CreatedAt, &t.UpdatedAt, &s.forkThread, &s.forkTurn,
		&t.HiddenAt, &t.HiddenBy, &t.HideReason, &t.ChangedSince}
}

func scanThread(row pgx.Row, sc Scope) (Thread, error) {
	var t Thread
	var s threadScan
	if err := row.Scan(threadDest(&t, &s)...); err != nil {
		return t, err
	}
	decorateThread(&t, sc, s)
	return t, nil
}

// ThreadFilter selects threads for the history rail. Pinned and Archived are tri-state: nil means "do not filter" for Pinned and "not archived" for Archived (the rail shows archived threads only when asked).
type ThreadFilter struct {
	CaseID        string // a case: its own conversations, and the conversations across cases that cover it and that the caller may open
	CrossCase     *bool  // true: only conversations across cases; false: only conversations about one case; nil: both
	Q             string
	Tag           string
	Pinned        *bool
	Archived      string // "" (not archived, the default), "true" (archived only) or "all"
	Mine          bool
	SharedWithMe  bool
	IncludeHidden bool
	Since, Until  *time.Time
	Limit         int
	Cursor        string
}

type threadCursor struct {
	Pinned bool      `json:"p"`
	TS     time.Time `json:"t"`
	ID     string    `json:"i"`
}

// canSeeHidden: the owner sees their own hidden threads; a global admin sees all. An auditor member sees them through the case role at the call.
func canSeeHidden(sc Scope, role Role) bool { return sc.IsAdmin() || role.SeesPrivateThreads() }

// ListThreads is the history rail: pinned first, then newest first, keyset-paged. Hidden threads are left out unless asked for, and then only those the caller may see.
func (s *Service) ListThreads(ctx context.Context, sc Scope, f ThreadFilter) (Page[Thread], error) {
	page := Page[Thread]{Items: []Thread{}}
	limit := clampLimit(f.Limit)
	err := s.Store.InTx(ctx, sc, false, func(ctx context.Context, tx pgx.Tx) error {
		args := []any{sc.TenantID}
		arg := func(v any) string { args = append(args, v); return "$" + strconv.Itoa(len(args)) }
		var where []string
		where = append(where, "t.tenant_id = $1")
		if sc.BoundCase != "" {
			// a credential bound to one case lists that case and nothing else, and has a role in no other case: it sees no conversation across cases
			if f.CaseID == "" {
				f.CaseID = sc.BoundCase
			}
			where = append(where, "t.collection_id IS NOT NULL")
		}
		if f.CaseID != "" {
			if _, err := requireRole(ctx, tx, sc, f.CaseID); err != nil {
				return err
			}
			c := arg(f.CaseID)
			where = append(where, "(t.collection_id = "+c+" OR (t.collection_id IS NULL AND "+c+" = ANY (t.scope_collection_ids)))")
		}
		if f.CrossCase != nil {
			if *f.CrossCase {
				where = append(where, "t.collection_id IS NULL")
			} else {
				where = append(where, "t.collection_id IS NOT NULL")
			}
		}
		// who sees hidden conversations: their owner, a global admin everywhere, an auditor of the case asked for among that case's own conversations (a conversation across cases is
		// read by an auditor of all of its cases when it is opened by its id, and is not offered to anyone else here)
		admin := sc.IsAdmin()
		auditor := false
		if f.CaseID != "" && !admin {
			role, _ := roleIn(ctx, tx, sc, f.CaseID)
			auditor = canSeeHidden(sc, role)
		}
		if f.IncludeHidden {
			where = append(where, fmt.Sprintf("(t.hidden_at IS NULL OR t.owner_subject = %s OR %s OR (t.collection_id IS NOT NULL AND %s))", arg(sc.Subject), arg(admin), arg(auditor)))
		} else {
			where = append(where, "t.hidden_at IS NULL")
		}
		if f.Pinned != nil {
			where = append(where, "t.pinned = "+arg(*f.Pinned))
		}
		switch f.Archived {
		case "all":
		case "true":
			where = append(where, "t.archived")
		default:
			where = append(where, "NOT t.archived")
		}
		if f.Mine {
			where = append(where, "t.owner_subject = "+arg(sc.Subject))
		}
		if f.SharedWithMe {
			where = append(where, "t.owner_subject <> "+arg(sc.Subject))
		}
		if f.Since != nil {
			where = append(where, "coalesce(t.last_turn_at, t.created_at) >= "+arg(*f.Since))
		}
		if f.Until != nil {
			where = append(where, "coalesce(t.last_turn_at, t.created_at) < "+arg(*f.Until))
		}
		if f.Tag != "" {
			where = append(where, `EXISTS (SELECT 1 FROM forensic.tag_assignment a JOIN forensic.tag g ON g.tag_id = a.tag_id
				WHERE a.object_kind = 'thread' AND a.object_id = t.thread_id AND g.name_key = forensic.search_normalize(btrim(`+arg(f.Tag)+`)))`)
		}
		var matchSQL string
		words := searchWords(f.Q)
		if len(words) > 0 {
			var parts []string
			for _, w := range words {
				parts = append(parts, "m.search_text LIKE '%' || forensic.search_normalize("+arg(w)+") || '%'")
			}
			matchSQL = strings.Join(parts, " AND ")
			where = append(where, "EXISTS (SELECT 1 FROM forensic.conversation_turn m WHERE m.thread_id = t.thread_id AND "+matchSQL+")")
		}
		if f.Cursor != "" {
			var c threadCursor
			if err := decodeCursor(f.Cursor, &c); err != nil {
				return err
			}
			where = append(where, fmt.Sprintf("(t.pinned, coalesce(t.last_turn_at, t.created_at), t.thread_id) < (%s::boolean, %s::timestamptz, %s::uuid)", arg(c.Pinned), arg(c.TS), arg(c.ID)))
		}
		query := "SELECT " + threadColumns + ", coalesce(t.last_turn_at, t.created_at) AS sort_ts FROM forensic.conversation_thread t WHERE " + strings.Join(where, " AND ") +
			" ORDER BY t.pinned DESC, sort_ts DESC, t.thread_id DESC LIMIT " + strconv.Itoa(limit+1)
		rows, err := tx.Query(ctx, query, args...)
		if err != nil {
			return err
		}
		var sorts []time.Time
		for rows.Next() {
			var t Thread
			var s threadScan
			var sortTS time.Time
			if err := rows.Scan(append(threadDest(&t, &s), &sortTS)...); err != nil {
				rows.Close()
				return err
			}
			decorateThread(&t, sc, s)
			page.Items = append(page.Items, t)
			sorts = append(sorts, sortTS)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		if len(page.Items) > limit {
			last := page.Items[limit-1]
			page.NextCursor = encodeCursor(threadCursor{Pinned: last.Pinned, TS: sorts[limit-1], ID: last.ID})
			page.Items = page.Items[:limit]
		}
		if len(page.Items) == 0 {
			return nil
		}
		ids := make([]string, len(page.Items))
		index := map[string]int{}
		for i, t := range page.Items {
			ids[i] = t.ID
			index[t.ID] = i
		}
		tagRows, err := tx.Query(ctx, `SELECT a.object_id::text, g.name FROM forensic.tag_assignment a JOIN forensic.tag g ON g.tag_id = a.tag_id
			WHERE a.object_kind = 'thread' AND a.object_id = ANY ($1::uuid[]) ORDER BY g.name_key`, ids)
		if err != nil {
			return err
		}
		for tagRows.Next() {
			var id, name string
			if err := tagRows.Scan(&id, &name); err != nil {
				tagRows.Close()
				return err
			}
			page.Items[index[id]].Tags = append(page.Items[index[id]].Tags, name)
		}
		tagRows.Close()
		if err := tagRows.Err(); err != nil {
			return err
		}
		if matchSQL == "" {
			return nil
		}
		// the matching turns of each thread found, at most three each, newest first: the rail shows why a thread matched
		margs := []any{ids}
		for _, w := range words {
			margs = append(margs, w)
		}
		var mp []string
		for i := range words {
			mp = append(mp, fmt.Sprintf("m.search_text LIKE '%%' || forensic.search_normalize($%d) || '%%'", i+2))
		}
		mrows, err := tx.Query(ctx, `SELECT thread_id, turn_id, idx, question_text, headline FROM (
			SELECT m.thread_id::text AS thread_id, m.turn_id::text AS turn_id, m.idx, m.question_text, m.headline, row_number() OVER (PARTITION BY m.thread_id ORDER BY m.idx DESC) AS rn
			FROM forensic.conversation_turn m WHERE m.thread_id = ANY ($1::uuid[]) AND `+strings.Join(mp, " AND ")+`) z WHERE rn <= 3 ORDER BY thread_id, idx DESC`, margs...)
		if err != nil {
			return err
		}
		for mrows.Next() {
			var tid string
			var m Match
			if err := mrows.Scan(&tid, &m.TurnID, &m.Index, &m.Question, &m.Headline); err != nil {
				mrows.Close()
				return err
			}
			m.Question = clip(m.Question, 200)
			page.Items[index[tid]].Matches = append(page.Items[index[tid]].Matches, m)
		}
		mrows.Close()
		return mrows.Err()
	})
	return page, err
}

func decorateThread(t *Thread, sc Scope, s threadScan) {
	t.CreatedAt, t.UpdatedAt, t.LastTurnAt, t.HiddenAt = utc(t.CreatedAt), utc(t.UpdatedAt), utcPtr(t.LastTurnAt), utcPtr(t.HiddenAt)
	if s.forkThread != nil && s.forkTurn != nil {
		t.ForkedFrom = &ForkRef{ThreadID: *s.forkThread, TurnID: *s.forkTurn}
	}
	t.CrossCase = t.CaseID == ""
	if t.CrossCase {
		t.CaseIDs = append([]string{}, s.scope...)
	} else {
		t.CaseIDs = []string{t.CaseID}
	}
	if t.SharedWith == nil {
		t.SharedWith = []string{}
	}
	t.Tags = []string{}
	t.IsOwner = t.Owner == sc.Subject
	t.Hidden = t.HiddenAt != nil
	switch {
	case t.Title != nil && *t.Title != "":
		t.DisplayTitle = *t.Title
	case t.FirstQuestion != nil && *t.FirstQuestion != "":
		t.DisplayTitle = clip(*t.FirstQuestion, 80)
	default:
		t.DisplayTitle = "New conversation"
	}
}

// loadThread reads one thread the caller may see, with the caller's role in each of its cases (t.roles) and the weakest of them (the returned Role). A thread of a case the caller has no role in
// does not exist for them, which is also how a credential bound to one case sees the threads of every other. A hidden thread is found only by its owner, a global admin, or an auditor of the
// case (of EVERY case, for a thread across cases); for anyone else it does not exist.
func loadThread(ctx context.Context, tx pgx.Tx, sc Scope, threadID string) (Thread, Role, error) {
	t, err := scanThread(tx.QueryRow(ctx, `SELECT `+threadColumns+` FROM forensic.conversation_thread t WHERE t.tenant_id = $1 AND t.thread_id = $2::uuid`, sc.TenantID, threadID), sc)
	if err != nil {
		return t, RoleNone, asError(err)
	}
	roles, err := caseRoles(ctx, tx, sc, t.CaseIDs)
	if err != nil {
		return t, RoleNone, err
	}
	t.roles = roles
	role := weakestRole(roles)
	if !role.CanRead() {
		return t, role, NotFound("There is no such conversation, or you may not see it.")
	}
	if t.Hidden && !t.IsOwner && !(sc.IsAdmin() || allRoles(roles, RoleAuditor)) {
		return t, role, NotFound("There is no such conversation, or you may not see it.")
	}
	return t, role, nil
}

// CreateThread starts an empty conversation in a case.
func (s *Service) CreateThread(ctx context.Context, sc Scope, caseID, title string) (*Thread, error) {
	caseID = strings.TrimSpace(caseID)
	if caseID == "" {
		return nil, BadRequest("case_id is required.")
	}
	var clean *string
	if strings.TrimSpace(title) != "" {
		c, err := cleanLine("The title", title, 1, 200)
		if err != nil {
			return nil, err
		}
		clean = &c
	}
	var out Thread
	err := s.Store.InTx(ctx, sc, true, func(ctx context.Context, tx pgx.Tx) error {
		role, err := requireRole(ctx, tx, sc, caseID)
		if err != nil {
			return err
		}
		if !role.CanContribute() {
			return Forbidden("Your role in this case (%s) does not allow starting a conversation.", role)
		}
		known, err := caseKnown(ctx, tx, sc.TenantID, caseID)
		if err != nil {
			return err
		}
		if !known {
			return NotFound("There is no such case.")
		}
		source := "auto"
		if clean != nil {
			source = "user"
		}
		var id string
		if err := tx.QueryRow(ctx, `INSERT INTO forensic.conversation_thread (tenant_id, collection_id, owner_subject, title, title_source) VALUES ($1, $2, $3, $4, $5) RETURNING thread_id::text`,
			sc.TenantID, caseID, sc.Subject, clean, source).Scan(&id); err != nil {
			return err
		}
		if err := audit(ctx, tx, sc, role, caseID, "thread.created", "thread", id, map[string]any{"titled": clean != nil}); err != nil {
			return err
		}
		out, _, err = loadThread(ctx, tx, sc, id)
		return err
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// GetOptions say how much of the answers to include with a thread: "latest" (default) the answer of the last finished turn, "all" every stored answer, "none".
type GetOptions struct{ Answers string }

// GetThread reads a thread with the digests of its turns (and, as asked, their answers).
func (s *Service) GetThread(ctx context.Context, sc Scope, threadID string, opt GetOptions) (*Thread, error) {
	var out Thread
	err := s.Store.InTx(ctx, sc, false, func(ctx context.Context, tx pgx.Tx) error {
		t, _, err := loadThread(ctx, tx, sc, threadID)
		if err != nil {
			return err
		}
		turns, err := loadTurns(ctx, tx, sc, `t.thread_id = $2::uuid`, []any{threadID}, "t.idx", 0)
		if err != nil {
			return err
		}
		t.Turns = turns
		if n := len(turns); n > 0 {
			id := turns[n-1].ID
			t.ActiveTurnID = &id
		}
		attach := func(i int) error {
			ans, err := loadAnswer(ctx, tx, t.Turns[i].ID, s.Config.Snapshot)
			if err != nil {
				return err
			}
			t.Turns[i].Answer = ans
			t.Turns[i].LiveTokens = s.tokensLive(t.Turns[i])
			return nil
		}
		switch opt.Answers {
		case "none":
		case "all":
			for i := range t.Turns {
				if t.Turns[i].State == "completed" {
					if err := attach(i); err != nil {
						return err
					}
				}
			}
		default:
			for i := len(t.Turns) - 1; i >= 0; i-- {
				if t.Turns[i].State == "completed" && t.Turns[i].SnapshotState != SnapshotNone {
					if err := attach(i); err != nil {
						return err
					}
					break
				}
			}
		}
		tags, err := objectTags(ctx, tx, "thread", t.ID)
		if err != nil {
			return err
		}
		t.Tags = tags
		out = t
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// ThreadPatch is what a client may change in a thread. A nil field is left alone.
type ThreadPatch struct {
	Title      *string
	ClearTitle bool
	Pinned     *bool
	Archived   *bool
	Visibility *string
	SharedWith *[]string
}

// PatchThread renames, pins, archives or shares a thread. Only its owner (or a global admin) may.
func (s *Service) PatchThread(ctx context.Context, sc Scope, threadID string, p ThreadPatch) (*Thread, error) {
	if p.Title == nil && !p.ClearTitle && p.Pinned == nil && p.Archived == nil && p.Visibility == nil && p.SharedWith == nil {
		return nil, BadRequest("Nothing to change: send title, pinned, archived, visibility or shared_with.")
	}
	if p.Visibility != nil && *p.Visibility != "private" && *p.Visibility != "selected" && *p.Visibility != "case" {
		return nil, BadRequest("visibility must be private, selected or case.")
	}
	var title *string
	if p.Title != nil {
		c, err := cleanLine("The title", *p.Title, 1, 200)
		if err != nil {
			return nil, err
		}
		title = &c
	}
	var shared []string
	if p.SharedWith != nil {
		seen := map[string]bool{}
		for _, v := range *p.SharedWith {
			v, err := cleanLine("A name in shared_with", v, 1, 256)
			if err != nil {
				return nil, err
			}
			if !seen[v] {
				seen[v] = true
				shared = append(shared, v)
			}
		}
		if len(shared) > 100 {
			return nil, BadRequest("A conversation can be shared with at most 100 people.")
		}
	}
	var out Thread
	err := s.Store.InTx(ctx, sc, true, func(ctx context.Context, tx pgx.Tx) error {
		t, _, err := loadThread(ctx, tx, sc, threadID)
		if err != nil {
			return err
		}
		if !t.IsOwner && !sc.IsAdmin() {
			return Forbidden("Only the owner of a conversation can change it.")
		}
		if t.Hidden {
			return Conflict("hidden", "This conversation is hidden; restore it first.")
		}
		var sets []string
		args := []any{threadID, sc.TenantID}
		arg := func(v any) string { args = append(args, v); return "$" + strconv.Itoa(len(args)) }
		changed := []string{}
		switch {
		case title != nil:
			sets = append(sets, "title = "+arg(*title), "title_source = 'user'")
			changed = append(changed, "title")
		case p.ClearTitle:
			sets = append(sets, "title = NULL", "title_source = 'auto'")
			changed = append(changed, "title")
		}
		if p.Pinned != nil {
			sets = append(sets, "pinned = "+arg(*p.Pinned))
			changed = append(changed, "pinned")
		}
		if p.Archived != nil {
			sets = append(sets, "archived = "+arg(*p.Archived))
			changed = append(changed, "archived")
		}
		if p.Visibility != nil || p.SharedWith != nil {
			visibility := t.Visibility
			if p.Visibility != nil {
				visibility = *p.Visibility
			}
			list := t.SharedWith
			if p.SharedWith != nil {
				list = shared
			}
			if visibility != "selected" {
				list = []string{}
			} else if len(list) == 0 {
				return BadRequest("A conversation shared with selected people needs at least one name in shared_with.")
			}
			if visibility == "selected" {
				// in a controlled case a conversation is shared only with its members; across cases, with people who are members of every controlled case it covers
				for _, c := range t.CaseIDs {
					if t.roles[c] == RoleOpen {
						continue
					}
					if err := requireMembers(ctx, tx, sc.TenantID, c, list); err != nil {
						return err
					}
				}
			}
			sets = append(sets, "visibility = "+arg(visibility), "shared_with = "+arg(list))
			changed = append(changed, "visibility")
		}
		sets = append(sets, "updated_at = now()")
		if _, err := tx.Exec(ctx, "UPDATE forensic.conversation_thread SET "+strings.Join(sets, ", ")+" WHERE thread_id = $1::uuid AND tenant_id = $2", args...); err != nil {
			return err
		}
		action := "thread.updated"
		if len(changed) == 1 && changed[0] == "visibility" {
			action = "thread.shared"
		}
		details := map[string]any{"changed": changed}
		if p.Visibility != nil {
			details["visibility"] = *p.Visibility
		}
		if err := auditThread(ctx, tx, sc, t, action, details); err != nil {
			return err
		}
		out, _, err = loadThread(ctx, tx, sc, threadID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// requireMembers refuses a share list that names someone who is not a member of the (controlled) case.
func requireMembers(ctx context.Context, tx pgx.Tx, tenantID, caseID string, subjects []string) error {
	var missing []string
	rows, err := tx.Query(ctx, `SELECT s FROM unnest($3::text[]) AS s WHERE NOT EXISTS (SELECT 1 FROM forensic.case_member m WHERE m.tenant_id = $1 AND m.collection_id = $2 AND m.subject_id = s)`, tenantID, caseID, subjects)
	if err != nil {
		return err
	}
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			rows.Close()
			return err
		}
		missing = append(missing, s)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	if len(missing) > 0 {
		return BadRequest("These people are not members of the case, so a conversation cannot be shared with them: %s.", strings.Join(missing, ", "))
	}
	return nil
}

// HideThread is "delete": the thread leaves the lists, its turns stay, and the audit row keeps who hid it and why.
func (s *Service) HideThread(ctx context.Context, sc Scope, threadID, reason string) error {
	reason = strings.TrimSpace(reason)
	if reason != "" {
		var err error
		if reason, err = cleanLine("The reason", reason, 1, 500); err != nil {
			return err
		}
	}
	return s.Store.InTx(ctx, sc, true, func(ctx context.Context, tx pgx.Tx) error {
		t, _, err := loadThread(ctx, tx, sc, threadID)
		if err != nil {
			return err
		}
		if !t.IsOwner && !sc.IsAdmin() {
			return Forbidden("Only the owner of a conversation can delete it.")
		}
		if t.Hidden {
			return nil
		}
		if err := auditThread(ctx, tx, sc, t, "thread.hidden", map[string]any{"reason": reason, "turns": t.TurnCount}); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE forensic.conversation_thread SET hidden_at = now(), hidden_by = $3, hide_reason = nullif($4, ''), updated_at = now() WHERE thread_id = $1::uuid AND tenant_id = $2`,
			threadID, sc.TenantID, sc.Subject, reason)
		return err
	})
}

// RestoreThread brings a hidden thread back.
func (s *Service) RestoreThread(ctx context.Context, sc Scope, threadID string) (*Thread, error) {
	var out Thread
	err := s.Store.InTx(ctx, sc, true, func(ctx context.Context, tx pgx.Tx) error {
		t, _, err := loadThread(ctx, tx, sc, threadID)
		if err != nil {
			return err
		}
		if !t.IsOwner && !sc.IsAdmin() {
			return Forbidden("Only the owner of a conversation or an administrator can restore it.")
		}
		if !t.Hidden {
			out = t
			return nil
		}
		if err := auditThread(ctx, tx, sc, t, "thread.restored", nil); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE forensic.conversation_thread SET hidden_at = NULL, hidden_by = NULL, hide_reason = NULL, updated_at = now() WHERE thread_id = $1::uuid AND tenant_id = $2`, threadID, sc.TenantID); err != nil {
			return err
		}
		out, _, err = loadThread(ctx, tx, sc, threadID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// ForkTurn starts a new conversation, owned by the caller, whose context is the chain of turns up to the one forked. Nothing is copied: the first turn of the fork names the source turn as its parent.
func (s *Service) ForkTurn(ctx context.Context, sc Scope, turnID, title string) (*Thread, error) {
	var clean *string
	if strings.TrimSpace(title) != "" {
		c, err := cleanLine("The title", title, 1, 200)
		if err != nil {
			return nil, err
		}
		clean = &c
	}
	var out Thread
	err := s.Store.InTx(ctx, sc, true, func(ctx context.Context, tx pgx.Tx) error {
		var caseID, threadID string
		if err := tx.QueryRow(ctx, `SELECT collection_id, thread_id::text FROM forensic.conversation_turn WHERE tenant_id = $1 AND turn_id = $2::uuid`, sc.TenantID, turnID).Scan(&caseID, &threadID); err != nil {
			return asError(err)
		}
		if _, _, err := loadThread(ctx, tx, sc, threadID); err != nil { // a hidden conversation cannot be forked by someone who may not see it
			return err
		}
		role, err := requireRole(ctx, tx, sc, caseID)
		if err != nil {
			return err
		}
		if !role.CanContribute() {
			return Forbidden("Your role in this case (%s) does not allow starting a conversation.", role)
		}
		source := "auto"
		if clean != nil {
			source = "user"
		}
		var id string
		if err := tx.QueryRow(ctx, `INSERT INTO forensic.conversation_thread (tenant_id, collection_id, owner_subject, title, title_source, forked_from_thread_id, forked_from_turn_id)
			VALUES ($1, $2, $3, $4, $5, $6::uuid, $7::uuid) RETURNING thread_id::text`, sc.TenantID, caseID, sc.Subject, clean, source, threadID, turnID).Scan(&id); err != nil {
			return err
		}
		if err := audit(ctx, tx, sc, role, caseID, "thread.forked", "thread", id, map[string]any{"from_thread": threadID, "from_turn": turnID}); err != nil {
			return err
		}
		out, _, err = loadThread(ctx, tx, sc, id)
		return err
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}
