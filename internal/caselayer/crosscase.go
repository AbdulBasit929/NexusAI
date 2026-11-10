package caselayer

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Conversations across cases (Ask across cases). One thread spans two to ten cases. The case is the unit of trust, so the rule of one case is applied to all of them: a person sees such a
// thread only if they may open EVERY one of its cases (a case a person may not open is never named to them), and each turn of it is an ordinary turn of ONE case. A question put to such a
// thread is put to each of its cases in turn, one engine call each; the turns asked together share a group id, and the group says in a sentence which cases were asked and how many answered.
// The engine does not read across cases (that is the engine's work, see docs/architecture/CONVERSATIONS_AND_CASE_LAYER_20261103.md); nothing here joins one case's rows to another's.

const (
	minCrossCases = 2
	maxCrossCases = 10
	// maxCaseListBytes is what the table's CHECK accepts for the cases of one thread together (ten names of 128 characters, counted in bytes of the names).
	maxCaseListBytes = 1280
)

// normalizeCases checks the cases a thread across cases is to span and puts them in the form the table keeps: each name trimmed, each case once, in byte order (the order the database sorts
// the column in, COLLATE "C").
func normalizeCases(ids []string) ([]string, error) {
	seen := map[string]bool{}
	out := make([]string, 0, len(ids))
	total := 0
	for _, raw := range ids {
		id := strings.TrimSpace(raw)
		switch {
		case id == "":
			return nil, BadRequest("A case name in case_ids is empty.")
		case utf8.RuneCountInString(id) > 128:
			return nil, BadRequest("A case name in case_ids is longer than 128 characters.")
		case !utf8.ValidString(id) || strings.IndexFunc(id, unicode.IsControl) >= 0:
			return nil, BadRequest("A case name in case_ids has a control character or is not valid text.")
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
		total += len(id)
	}
	sort.Strings(out)
	switch {
	case len(out) < minCrossCases:
		return nil, BadRequest("A conversation across cases needs at least %d different cases; for one case send case_id.", minCrossCases)
	case len(out) > maxCrossCases:
		return nil, BadRequest("A conversation can span at most %d cases.", maxCrossCases)
	case total > maxCaseListBytes:
		return nil, BadRequest("The names of the cases are longer than 1,280 bytes together.")
	}
	return out, nil
}

// roleRank orders the roles by what they allow, for "the weakest of a person's roles". An open case counts as an analyst (that is how it behaves).
var roleRank = map[Role]int{RoleNone: 0, RoleAuditor: 1, RoleViewer: 2, RoleReviewer: 3, RoleAnalyst: 4, RoleOpen: 4, RoleLead: 5, RoleOwner: 6}

// weakestRole is the least a person can do in any of the cases: their role in a thread across them. A role the layer does not know counts as none, and so does no case at all.
func weakestRole(roles map[string]Role) Role {
	if len(roles) == 0 {
		return RoleNone
	}
	ids := make([]string, 0, len(roles))
	for id := range roles {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	weakest, rank := RoleNone, 1<<30
	for _, id := range ids {
		if r := roleRank[roles[id]]; r < rank {
			weakest, rank = roles[id], r
		}
	}
	if rank == 0 {
		return RoleNone
	}
	return weakest
}

// allRoles is true when the person's role in EVERY one of the cases is one of those named. No case at all is false.
func allRoles(roles map[string]Role, want ...Role) bool {
	if len(roles) == 0 {
		return false
	}
	for _, have := range roles {
		ok := false
		for _, w := range want {
			if have == w {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	return true
}

// caseRoles reads the caller's role in each case of a thread, by the same function the policies use. A credential bound to one case has no role in any other.
func caseRoles(ctx context.Context, tx pgx.Tx, sc Scope, cases []string) (map[string]Role, error) {
	roles := make(map[string]Role, len(cases))
	for _, c := range cases {
		r, err := roleIn(ctx, tx, sc, c)
		if err != nil {
			return nil, err
		}
		roles[c] = r
	}
	return roles, nil
}

// auditThread writes the audit row of a change to a conversation. A thread about one case writes one row in that case's chain. A thread across cases writes one row in the chain of EACH of its
// cases, which says that it is one across cases and how many cases it spans, never which: the people who read one case's audit trail need not be allowed to know of the others.
func auditThread(ctx context.Context, tx pgx.Tx, sc Scope, t Thread, action string, details map[string]any) error {
	for _, c := range t.CaseIDs {
		d := details
		if t.CrossCase {
			d = make(map[string]any, len(details)+2)
			for k, v := range details {
				d[k] = v
			}
			d["cross_case"] = true
			d["cases"] = len(t.CaseIDs)
		}
		if err := audit(ctx, tx, sc, t.roles[c], c, action, "thread", t.ID, d); err != nil {
			return err
		}
	}
	return nil
}

// ---- starting one -------------------------------------------------------------------------------------------------------------------------------------------------------------

// CreateThreadAcross starts an empty conversation across two to ten cases. The caller needs a role that allows asking in every one of them, and every one must exist.
func (s *Service) CreateThreadAcross(ctx context.Context, sc Scope, caseIDs []string, title string) (*Thread, error) {
	cases, err := normalizeCases(caseIDs)
	if err != nil {
		return nil, err
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
	err = s.Store.InTx(ctx, sc, true, func(ctx context.Context, tx pgx.Tx) error {
		roles := make(map[string]Role, len(cases))
		for _, c := range cases {
			role, err := roleIn(ctx, tx, sc, c)
			if err != nil {
				return err
			}
			if !role.CanRead() {
				return NotFound("There is no such case as %q, or you are not a member of it.", c)
			}
			if !role.CanContribute() {
				return Forbidden("Your role in case %q (%s) does not allow starting a conversation.", c, role)
			}
			known, err := caseKnown(ctx, tx, sc.TenantID, c)
			if err != nil {
				return err
			}
			if !known {
				return NotFound("There is no such case as %q.", c)
			}
			roles[c] = role
		}
		source := "auto"
		if clean != nil {
			source = "user"
		}
		var id string
		if err := tx.QueryRow(ctx, `INSERT INTO forensic.conversation_thread (tenant_id, collection_id, scope_collection_ids, owner_subject, title, title_source) VALUES ($1, NULL, $2, $3, $4, $5) RETURNING thread_id::text`,
			sc.TenantID, cases, sc.Subject, clean, source).Scan(&id); err != nil {
			return err
		}
		if err := auditThread(ctx, tx, sc, Thread{ID: id, CaseIDs: cases, CrossCase: true, roles: roles}, "thread.created", map[string]any{"titled": clean != nil}); err != nil {
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

// ---- asking one ---------------------------------------------------------------------------------------------------------------------------------------------------------------

// GroupCase is what one case made of a question put to several: its turn (nil when none could be written) and, when no turn was written, why.
type GroupCase struct {
	CaseID   string `json:"case_id"`
	Turn     *Turn  `json:"turn"`
	Existing bool   `json:"existing"`
	Error    string `json:"error,omitempty"`
}

// Group is a question put to every case of a conversation across cases. Summary states the scope of what was asked, as the roadmap requires of an answer across cases.
type Group struct {
	GroupID     string      `json:"group_id"`
	ThreadID    string      `json:"thread_id"`
	Question    string      `json:"question"`
	Cases       []GroupCase `json:"cases"`
	Answered    int         `json:"answered"`
	NotAnswered int         `json:"not_answered"`
	Summary     string      `json:"summary"`
}

// groupSummary is the sentence that says which cases were asked and how many answered.
func groupSummary(g Group) string {
	ids := make([]string, len(g.Cases))
	for i, c := range g.Cases {
		ids[i] = c.CaseID
	}
	return fmt.Sprintf("Asked in %d cases (%s): answered in %d, not answered in %d.", len(g.Cases), strings.Join(ids, ", "), g.Answered, g.NotAnswered)
}

// countGroup counts the cases that answered (a finished turn whose outcome is "answered"), counts every other case as not answered, and writes the sentence.
func countGroup(g *Group) {
	g.Answered, g.NotAnswered = 0, 0
	for _, c := range g.Cases {
		if c.Turn != nil && c.Turn.State == "completed" && c.Turn.Outcome != nil && *c.Turn.Outcome == OutcomeAnswered {
			g.Answered++
		} else {
			g.NotAnswered++
		}
	}
	g.Summary = groupSummary(*g)
}

// caseSink marks the events of one case's turn with the case and the group, moves its clock by what had passed when the case began (so the stream of the whole question has one clock that does
// not run backwards), and drops the case's "done": the end of one case is not the end of the question.
type caseSink struct {
	inner    EventSink
	caseID   string
	groupID  string
	offsetMS int64
}

func (c caseSink) Send(e Event) {
	if e.Event == "done" {
		return
	}
	e.CaseID, e.GroupID = c.caseID, c.groupID
	e.AtMS += c.offsetMS
	c.inner.Send(e)
}

// runGroup puts a question to each case of a conversation across cases, one after another (the model answers one request at a time and the slots are shared with everyone else). Everything
// that decides whether the question may be asked is checked for ALL the cases before any turn is written, so a refusal leaves nothing behind. From then on the question is completed in every
// case even if the caller goes away, as one turn is; a case that fails is a failed turn of that case and the others go on; the whole question has the time one turn has.
func (s *Service) runGroup(ctx context.Context, sc Scope, threadID string, spec runSpec, sink EventSink) (*RunResult, error) {
	if spec.ParentSet || spec.EditOf != "" {
		return nil, BadRequest("A question across cases is asked after the last answer of each case; it cannot be put below a chosen answer or replace one. Fork the answer of one case (POST /turns/{id}/fork) to branch from it.")
	}
	if sink == nil {
		sink = nopSink{}
	}
	sink = &lockedSink{inner: sink}
	var t Thread
	var earlier *string // the group of a first try of this same request, if it wrote any turn
	err := s.Store.InTx(ctx, sc, false, func(ctx context.Context, tx pgx.Tx) error {
		var lerr error
		if t, _, lerr = loadThread(ctx, tx, sc, threadID); lerr != nil {
			return lerr
		}
		switch {
		case !t.IsOwner:
			return Forbidden("Only the owner of a conversation can add a question to it; fork it to continue in your own conversation.")
		case t.Hidden:
			return Conflict("hidden", "This conversation is hidden; restore it first.")
		case t.Archived:
			return Conflict("archived", "This conversation is archived; restore it to continue.")
		}
		for _, c := range t.CaseIDs {
			if role := t.roles[c]; !role.CanContribute() {
				return Forbidden("Your role in case %q (%s) does not allow asking questions.", c, role)
			}
		}
		if spec.ClientReqID != "" {
			err := tx.QueryRow(ctx, `SELECT group_id::text FROM forensic.conversation_turn WHERE thread_id = $1::uuid AND starts_with(client_request_id, $2) AND group_id IS NOT NULL ORDER BY idx LIMIT 1`,
				threadID, spec.ClientReqID+"@").Scan(&earlier)
			if err != nil && err != pgx.ErrNoRows {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	loopCtx := context.WithoutCancel(ctx)
	groupID := uuid.NewString()
	if earlier != nil { // a retry goes on under the group of the first try, so the turns it finds and the turns it writes are one group
		groupID = *earlier
	}
	began := time.Now()
	deadline := began.Add(s.Config.TurnTimeout)
	sink.Send(Event{Event: "phase", Phase: "cases", Label: fmt.Sprintf("Asking %d cases, one after another", len(t.CaseIDs)), ThreadID: threadID, GroupID: groupID})
	g := Group{GroupID: groupID, ThreadID: threadID, Question: spec.Stored, Cases: make([]GroupCase, 0, len(t.CaseIDs))}
	allExisting, written := true, 0
	var firstErr error
	for _, c := range t.CaseIDs {
		cs := spec
		cs.CaseID, cs.GroupID, cs.Deadline = c, groupID, deadline
		if spec.ClientReqID != "" {
			cs.ClientReqID = spec.ClientReqID + "@" + c // the characters of a client's id never include '@', so this cannot meet the id of another request
		}
		gc := GroupCase{CaseID: c}
		res, rerr := s.run(loopCtx, sc, threadID, cs, caseSink{inner: sink, caseID: c, groupID: groupID, offsetMS: time.Since(began).Milliseconds()})
		switch {
		case rerr != nil:
			allExisting = false
			gc.Error = asError(rerr).Message
			if firstErr == nil {
				firstErr = rerr
			}
		default:
			written++
			gc.Turn, gc.Existing = res.Turn, res.Existing
			if !res.Existing {
				allExisting = false
			}
		}
		g.Cases = append(g.Cases, gc)
	}
	if written == 0 && firstErr != nil {
		return nil, firstErr
	}
	countGroup(&g)
	at := time.Since(began).Milliseconds()
	sink.Send(Event{Event: "group", Group: &g, ThreadID: threadID, GroupID: groupID, AtMS: at})
	sink.Send(Event{Event: "done", ThreadID: threadID, GroupID: groupID, AtMS: at})
	return &RunResult{Group: &g, Existing: allExisting && written == len(t.CaseIDs)}, nil
}
