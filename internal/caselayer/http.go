package caselayer

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Handler serves the case layer over HTTP. The API supplies Scope, which turns a request into who is asking (and refuses a request whose own scope forbids the case it names).
type Handler struct {
	Svc   *Service
	Scope func(r *http.Request, caseID string) (Scope, error)
}

const maxBody = 1 << 20

func requestID(r *http.Request) string {
	if id := strings.TrimSpace(r.Header.Get("X-Request-ID")); id != "" && len(id) <= 100 {
		return id
	}
	return uuid.NewString()
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// fail answers with the error's status and a sentence; the cause of an internal error is logged, never sent.
func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	e := asError(err)
	id := requestID(r)
	if e.Status >= 500 && e.Cause != nil {
		slog.Error("case layer", "request_id", id, "path", r.URL.Path, "error", e.Cause)
	}
	w.Header().Set("X-Request-ID", id)
	writeJSON(w, e.Status, map[string]any{"error": e.Message, "code": e.Code, "request_id": id})
}

func (h *Handler) scope(w http.ResponseWriter, r *http.Request, caseID string) (Scope, bool) {
	if h.Scope == nil {
		h.fail(w, r, Unavailable("The case layer has no way to tell who is asking."))
		return Scope{}, false
	}
	sc, err := h.Scope(r, caseID)
	if err != nil {
		h.fail(w, r, err)
		return Scope{}, false
	}
	return sc, true
}

// body reads a JSON object into a map so that "absent" and "null" can be told apart. An empty body is an empty object when optional.
func readMap(r *http.Request, optional bool) (map[string]json.RawMessage, error) {
	raw, err := io.ReadAll(io.LimitReader(r.Body, maxBody+1))
	if err != nil {
		return nil, BadRequest("The request body could not be read.")
	}
	if len(raw) > maxBody {
		return nil, newError(http.StatusRequestEntityTooLarge, "too_large", "The request body is larger than 1 MiB.")
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		if optional {
			return map[string]json.RawMessage{}, nil
		}
		return nil, BadRequest("The request body must be a JSON object.")
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil || m == nil {
		return nil, BadRequest("The request body must be a JSON object.")
	}
	return m, nil
}

func str(m map[string]json.RawMessage, key string) (string, bool, error) {
	raw, ok := m[key]
	if !ok {
		return "", false, nil
	}
	if string(raw) == "null" {
		return "", true, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", true, BadRequest("%s must be text.", key)
	}
	return s, true, nil
}

func boolean(m map[string]json.RawMessage, key string) (*bool, error) {
	raw, ok := m[key]
	if !ok {
		return nil, nil
	}
	var b bool
	if err := json.Unmarshal(raw, &b); err != nil {
		return nil, BadRequest("%s must be true or false.", key)
	}
	return &b, nil
}

func onlyKeys(m map[string]json.RawMessage, allowed ...string) error {
	ok := map[string]bool{}
	for _, a := range allowed {
		ok[a] = true
	}
	for k := range m {
		if !ok[k] {
			return BadRequest("%q is not a field of this request (allowed: %s).", k, strings.Join(allowed, ", "))
		}
	}
	return nil
}

// Register adds the routes to a mux.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /conversations", h.createThread)
	mux.HandleFunc("GET /conversations", h.listThreads)
	mux.HandleFunc("GET /conversations/{id}", h.getThread)
	mux.HandleFunc("PATCH /conversations/{id}", h.patchThread)
	mux.HandleFunc("DELETE /conversations/{id}", h.deleteThread)
	mux.HandleFunc("POST /conversations/{id}/restore", h.restoreThread)
	mux.HandleFunc("POST /conversations/{id}/turns", h.postTurn)
	mux.HandleFunc("GET /turns/{id}", h.getTurn)
	mux.HandleFunc("POST /turns/{id}/rerun", h.rerunTurn)
	mux.HandleFunc("POST /turns/{id}/fork", h.forkTurn)
	mux.HandleFunc("POST /turns/{id}/feedback", h.postFeedback)
	mux.HandleFunc("GET /turns/{id}/feedback", h.listFeedback)
	mux.HandleFunc("POST /turns/{id}/pin", h.pinTurn)
	mux.HandleFunc("GET /turns/{id}/diff", h.diffTurn)
	mux.HandleFunc("GET /turns/{id}/ancestry", h.ancestry)
	mux.HandleFunc("POST /conversations/{id}/export", h.exportThread)
	mux.HandleFunc("POST /cases/{case_id}/exhibits", h.exportExhibits)

	for plural := range objectSpecs {
		plural := plural
		mux.HandleFunc("GET /cases/{case_id}/"+plural, h.listObjects(plural))
		mux.HandleFunc("POST /cases/{case_id}/"+plural, h.createObject(plural))
		mux.HandleFunc("GET /"+plural+"/{id}", h.getObject(plural))
		mux.HandleFunc("PATCH /"+plural+"/{id}", h.patchObject(plural))
		if objectSpecs[plural].SoftDelete {
			mux.HandleFunc("DELETE /"+plural+"/{id}", h.deleteObject(plural))
		}
	}
	mux.HandleFunc("POST /findings/{id}/review", h.reviewFinding)
	mux.HandleFunc("POST /authority/{id}/revoke", h.revokeAuthority)

	mux.HandleFunc("GET /cases/{case_id}/tags", h.listTags)
	mux.HandleFunc("PATCH /tags/{id}", h.patchTag)
	mux.HandleFunc("GET /cases/{case_id}/objects/{kind}/{id}/tags", h.getObjectTags)
	mux.HandleFunc("PUT /cases/{case_id}/objects/{kind}/{id}/tags", h.putObjectTags)

	mux.HandleFunc("GET /cases/{case_id}/members", h.listMembers)
	mux.HandleFunc("PUT /cases/{case_id}/members/{subject}", h.putMember)
	mux.HandleFunc("DELETE /cases/{case_id}/members/{subject}", h.deleteMember)
	mux.HandleFunc("GET /cases/{case_id}/audit", h.listAudit)
	mux.HandleFunc("GET /cases/{case_id}/audit/verify", h.verifyAudit)
}

// ---- conversations ---------------------------------------------------------------------------------------------------------------------------------------------------------

// strList reads a list of text from a body: absent is an empty list; anything but a list of text is refused.
func strList(m map[string]json.RawMessage, key string) ([]string, bool, error) {
	raw, ok := m[key]
	if !ok || string(raw) == "null" {
		return nil, ok, nil
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, true, BadRequest("%s must be a list of text.", key)
	}
	return out, true, nil
}

func (h *Handler) createThread(w http.ResponseWriter, r *http.Request) {
	m, err := readMap(r, false)
	if err == nil {
		err = onlyKeys(m, "case_id", "case_ids", "title")
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	caseID, _, err := str(m, "case_id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	caseIDs, across, err := strList(m, "case_ids")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	title, _, err := str(m, "title")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if across && strings.TrimSpace(caseID) != "" {
		h.fail(w, r, BadRequest("Send case_id for a conversation about one case, or case_ids for one across several cases, not both."))
		return
	}
	// a credential bound to one case is checked against the case it names; for a list of cases the layer checks each (a bound credential has a role in no other case)
	named := strings.TrimSpace(caseID)
	sc, ok := h.scope(w, r, named)
	if !ok {
		return
	}
	var t *Thread
	if across {
		t, err = h.Svc.CreateThreadAcross(r.Context(), sc, caseIDs, title)
	} else {
		t, err = h.Svc.CreateThread(r.Context(), sc, caseID, title)
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, t)
}

func parseTime(name, v string) (*time.Time, error) {
	if v == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return nil, BadRequest("%s must be a time such as 2026-11-03T09:30:00Z.", name)
	}
	return &t, nil
}

func (h *Handler) listThreads(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	caseID := strings.TrimSpace(q.Get("case_id"))
	sc, ok := h.scope(w, r, caseID)
	if !ok {
		return
	}
	f := ThreadFilter{CaseID: caseID, Q: q.Get("q"), Tag: q.Get("tag"), Archived: q.Get("archived"), Mine: q.Get("mine") == "true", SharedWithMe: q.Get("shared") == "true",
		IncludeHidden: q.Get("include_hidden") == "true", Limit: atoiDefault(q.Get("limit"), 0), Cursor: q.Get("cursor")}
	if f.CaseID == "" && sc.BoundCase != "" {
		f.CaseID = sc.BoundCase
	}
	if v := q.Get("pinned"); v == "true" || v == "false" {
		b := v == "true"
		f.Pinned = &b
	}
	if v := q.Get("cross_case"); v == "true" || v == "false" {
		b := v == "true"
		f.CrossCase = &b
	}
	var err error
	if f.Since, err = parseTime("since", q.Get("since")); err == nil {
		f.Until, err = parseTime("until", q.Get("until"))
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	page, err := h.Svc.ListThreads(r.Context(), sc, f)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (h *Handler) getThread(w http.ResponseWriter, r *http.Request) {
	sc, ok := h.scope(w, r, "")
	if !ok {
		return
	}
	t, err := h.Svc.GetThread(r.Context(), sc, r.PathValue("id"), GetOptions{Answers: r.URL.Query().Get("answers")})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (h *Handler) patchThread(w http.ResponseWriter, r *http.Request) {
	m, err := readMap(r, false)
	if err == nil {
		err = onlyKeys(m, "title", "pinned", "archived", "visibility", "shared_with")
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var p ThreadPatch
	if raw, present := m["title"]; present {
		if string(raw) == "null" {
			p.ClearTitle = true
		} else {
			title, _, err := str(m, "title")
			if err != nil {
				h.fail(w, r, err)
				return
			}
			p.Title = &title
		}
	}
	if p.Pinned, err = boolean(m, "pinned"); err == nil {
		p.Archived, err = boolean(m, "archived")
	}
	if err == nil {
		if v, present, serr := str(m, "visibility"); serr != nil {
			err = serr
		} else if present {
			p.Visibility = &v
		}
	}
	if err == nil {
		if raw, present := m["shared_with"]; present {
			var list []string
			if jerr := json.Unmarshal(raw, &list); jerr != nil {
				err = BadRequest("shared_with must be a list of names.")
			} else {
				if list == nil {
					list = []string{}
				}
				p.SharedWith = &list
			}
		}
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	sc, ok := h.scope(w, r, "")
	if !ok {
		return
	}
	t, err := h.Svc.PatchThread(r.Context(), sc, r.PathValue("id"), p)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

// reasonOf reads an optional reason from the body or the query string (some proxies drop the body of a DELETE).
func reasonOf(r *http.Request) (string, error) {
	if v := r.URL.Query().Get("reason"); v != "" {
		return v, nil
	}
	m, err := readMap(r, true)
	if err != nil {
		return "", err
	}
	if err := onlyKeys(m, "reason"); err != nil {
		return "", err
	}
	v, _, err := str(m, "reason")
	return v, err
}

func (h *Handler) deleteThread(w http.ResponseWriter, r *http.Request) {
	reason, err := reasonOf(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	sc, ok := h.scope(w, r, "")
	if !ok {
		return
	}
	if err := h.Svc.HideThread(r.Context(), sc, r.PathValue("id"), reason); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) restoreThread(w http.ResponseWriter, r *http.Request) {
	sc, ok := h.scope(w, r, "")
	if !ok {
		return
	}
	t, err := h.Svc.RestoreThread(r.Context(), sc, r.PathValue("id"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

// ---- turns -----------------------------------------------------------------------------------------------------------------------------------------------------------------------

// streamMode is how the caller wants the events: "ndjson", "sse" or "" for one JSON answer at the end.
func streamMode(r *http.Request) string {
	switch strings.ToLower(r.URL.Query().Get("stream")) {
	case "ndjson", "1", "true":
		return "ndjson"
	case "sse":
		return "sse"
	case "false", "0", "json":
		return ""
	}
	accept := strings.ToLower(r.Header.Get("Accept"))
	switch {
	case strings.Contains(accept, "application/x-ndjson"):
		return "ndjson"
	case strings.Contains(accept, "text/event-stream"):
		return "sse"
	}
	return ""
}

// streamSink writes events as NDJSON lines or server-sent events. It starts the response on the first event, so a request that fails before anything happens still gets a plain error.
type streamSink struct {
	mu      sync.Mutex
	w       http.ResponseWriter
	mode    string
	started bool
	dead    bool
}

func (s *streamSink) Send(e Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dead {
		return
	}
	if !s.started {
		h := s.w.Header()
		if s.mode == "sse" {
			h.Set("Content-Type", "text/event-stream")
		} else {
			h.Set("Content-Type", "application/x-ndjson")
		}
		h.Set("Cache-Control", "no-store")
		h.Set("X-Accel-Buffering", "no")
		s.w.WriteHeader(http.StatusOK)
		s.started = true
	}
	raw, err := json.Marshal(e)
	if err != nil {
		return
	}
	var line []byte
	if s.mode == "sse" {
		line = []byte("event: " + e.Event + "\ndata: " + string(raw) + "\n\n")
	} else {
		line = append(raw, '\n')
	}
	if _, err := s.w.Write(line); err != nil {
		s.dead = true // the client went away; the turn goes on without it
		return
	}
	if f, ok := s.w.(http.Flusher); ok {
		f.Flush()
	}
}

func (s *streamSink) hasStarted() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.started
}

// finishRun writes the result of a run: the stream's last events are already sent; a plain call gets the turn and the diff as JSON.
func (h *Handler) finishRun(w http.ResponseWriter, r *http.Request, sink *streamSink, res *RunResult, err error) {
	switch {
	case err != nil && sink != nil && sink.hasStarted():
		e := asError(err)
		sink.Send(Event{Event: "error", Code: e.Code, Message: e.Message})
	case err != nil:
		h.fail(w, r, err)
	case sink != nil && sink.hasStarted():
		// the events carried the turn
	case sink != nil && res.Group != nil:
		sink.Send(Event{Event: "group", Group: res.Group, GroupID: res.Group.GroupID, ThreadID: res.Group.ThreadID})
		sink.Send(Event{Event: "done", GroupID: res.Group.GroupID, ThreadID: res.Group.ThreadID})
	case sink != nil:
		// an existing turn (a repeated client_request_id) ran no events: send it as the single event of the stream
		sink.Send(Event{Event: "turn", Turn: res.Turn, Diff: res.Diff})
		sink.Send(Event{Event: "done"})
	default:
		status := http.StatusCreated
		if res.Existing {
			status = http.StatusOK
		}
		if res.Group != nil {
			writeJSON(w, status, map[string]any{"group": res.Group, "existing": res.Existing})
			return
		}
		writeJSON(w, status, map[string]any{"turn": res.Turn, "diff": res.Diff, "existing": res.Existing})
	}
}

func (h *Handler) newSink(w http.ResponseWriter, r *http.Request) *streamSink {
	if mode := streamMode(r); mode != "" {
		return &streamSink{w: w, mode: mode}
	}
	return nil
}

func (h *Handler) postTurn(w http.ResponseWriter, r *http.Request) {
	m, err := readMap(r, false)
	if err == nil {
		err = onlyKeys(m, "question", "client_request_id", "parent_turn_id", "edit_of_turn_id", "language")
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var p RunParams
	for key, dst := range map[string]*string{"question": &p.Question, "client_request_id": &p.ClientRequestID, "parent_turn_id": &p.ParentTurnID, "edit_of_turn_id": &p.EditOfTurnID, "language": &p.Language} {
		v, _, serr := str(m, key)
		if serr != nil {
			h.fail(w, r, serr)
			return
		}
		*dst = v
	}
	sc, ok := h.scope(w, r, "")
	if !ok {
		return
	}
	sink := h.newSink(w, r)
	var es EventSink
	if sink != nil {
		es = sink
	}
	res, err := h.Svc.RunTurn(r.Context(), sc, r.PathValue("id"), p, es)
	h.finishRun(w, r, sink, res, err)
}

func (h *Handler) rerunTurn(w http.ResponseWriter, r *http.Request) {
	sc, ok := h.scope(w, r, "")
	if !ok {
		return
	}
	sink := h.newSink(w, r)
	var es EventSink
	if sink != nil {
		es = sink
	}
	res, err := h.Svc.Rerun(r.Context(), sc, r.PathValue("id"), es)
	h.finishRun(w, r, sink, res, err)
}

func (h *Handler) getTurn(w http.ResponseWriter, r *http.Request) {
	sc, ok := h.scope(w, r, "")
	if !ok {
		return
	}
	t, err := h.Svc.GetTurn(r.Context(), sc, r.PathValue("id"), r.URL.Query().Get("answer") != "false")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (h *Handler) forkTurn(w http.ResponseWriter, r *http.Request) {
	m, err := readMap(r, true)
	if err == nil {
		err = onlyKeys(m, "title")
	}
	var title string
	if err == nil {
		title, _, err = str(m, "title")
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	sc, ok := h.scope(w, r, "")
	if !ok {
		return
	}
	t, err := h.Svc.ForkTurn(r.Context(), sc, r.PathValue("id"), title)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, t)
}

func (h *Handler) postFeedback(w http.ResponseWriter, r *http.Request) {
	m, err := readMap(r, false)
	if err == nil {
		err = onlyKeys(m, "verdict", "note")
	}
	var verdict, note string
	if err == nil {
		if verdict, _, err = str(m, "verdict"); err == nil {
			note, _, err = str(m, "note")
		}
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	sc, ok := h.scope(w, r, "")
	if !ok {
		return
	}
	info, err := h.Svc.SetFeedback(r.Context(), sc, r.PathValue("id"), verdict, note)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (h *Handler) listFeedback(w http.ResponseWriter, r *http.Request) {
	sc, ok := h.scope(w, r, "")
	if !ok {
		return
	}
	items, err := h.Svc.ListFeedback(r.Context(), sc, r.PathValue("id"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) pinTurn(w http.ResponseWriter, r *http.Request) {
	m, err := readMap(r, true)
	if err == nil {
		err = onlyKeys(m, "title", "note", "basis")
	}
	var title, note, basis string
	if err == nil {
		if title, _, err = str(m, "title"); err == nil {
			if note, _, err = str(m, "note"); err == nil {
				basis, _, err = str(m, "basis")
			}
		}
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	sc, ok := h.scope(w, r, "")
	if !ok {
		return
	}
	res, err := h.Svc.PinTurn(r.Context(), sc, r.PathValue("id"), title, note, basis)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	status := http.StatusCreated
	if res.AlreadyPinned {
		status = http.StatusOK
	}
	writeJSON(w, status, map[string]any{"finding": res.Finding, "already_pinned": res.AlreadyPinned})
}

func (h *Handler) diffTurn(w http.ResponseWriter, r *http.Request) {
	sc, ok := h.scope(w, r, "")
	if !ok {
		return
	}
	d, err := h.Svc.DiffTurns(r.Context(), sc, r.PathValue("id"), r.URL.Query().Get("against"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (h *Handler) ancestry(w http.ResponseWriter, r *http.Request) {
	sc, ok := h.scope(w, r, "")
	if !ok {
		return
	}
	items, err := h.Svc.Ancestry(r.Context(), sc, r.PathValue("id"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// ---- export to the report builder ------------------------------------------------------------------------------------------------------------------------------------------

func exportOptions(m map[string]json.RawMessage) (ExportOptions, error) {
	var o ExportOptions
	var err error
	if o.TurnIDs, _, err = strList(m, "turn_ids"); err != nil {
		return o, err
	}
	if o.FindingIDs, _, err = strList(m, "finding_ids"); err != nil {
		return o, err
	}
	only, err := boolean(m, "only_confirmed")
	if err != nil {
		return o, err
	}
	if only != nil {
		o.OnlyConfirmed = *only
	}
	if raw, ok := m["max_rows"]; ok && string(raw) != "null" {
		var n int
		if err := json.Unmarshal(raw, &n); err != nil || n < 1 || n > maxExportRows {
			return o, BadRequest("max_rows must be a whole number from 1 to %d.", maxExportRows)
		}
		o.MaxRows = n
	}
	if o.Title, _, err = str(m, "title"); err != nil {
		return o, err
	}
	return o, nil
}

// writeBundle answers with the bundle as JSON, or, with ?format=markdown, with its Markdown alone (the hash of the bundle is in the X-Bundle-SHA256 header).
func writeBundle(w http.ResponseWriter, r *http.Request, b *Bundle) {
	if strings.EqualFold(r.URL.Query().Get("format"), "markdown") {
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Bundle-SHA256", b.BundleSHA256)
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, b.Markdown)
		return
	}
	writeJSON(w, http.StatusOK, b)
}

func (h *Handler) exportThread(w http.ResponseWriter, r *http.Request) {
	m, err := readMap(r, true)
	if err == nil {
		err = onlyKeys(m, "turn_ids", "max_rows", "title")
	}
	var opt ExportOptions
	if err == nil {
		opt, err = exportOptions(m)
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	sc, ok := h.scope(w, r, "")
	if !ok {
		return
	}
	b, err := h.Svc.ExportThread(r.Context(), sc, r.PathValue("id"), opt)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeBundle(w, r, b)
}

func (h *Handler) exportExhibits(w http.ResponseWriter, r *http.Request) {
	caseID := r.PathValue("case_id")
	m, err := readMap(r, false)
	if err == nil {
		err = onlyKeys(m, "finding_ids", "turn_ids", "only_confirmed", "max_rows", "title")
	}
	var opt ExportOptions
	if err == nil {
		opt, err = exportOptions(m)
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	sc, ok := h.scope(w, r, caseID)
	if !ok {
		return
	}
	b, err := h.Svc.ExportExhibits(r.Context(), sc, caseID, opt)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeBundle(w, r, b)
}

// ---- knowledge objects -----------------------------------------------------------------------------------------------------------------------------------------------

func (h *Handler) listObjects(plural string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caseID := r.PathValue("case_id")
		sc, ok := h.scope(w, r, caseID)
		if !ok {
			return
		}
		page, err := h.Svc.ListObjects(r.Context(), sc, objectSpecs[plural], caseID, r.URL.Query())
		if err != nil {
			h.fail(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, page)
	}
}

func (h *Handler) createObject(plural string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caseID := r.PathValue("case_id")
		body, err := readMap(r, false)
		if err != nil {
			h.fail(w, r, err)
			return
		}
		sc, ok := h.scope(w, r, caseID)
		if !ok {
			return
		}
		obj, err := h.Svc.CreateObject(r.Context(), sc, objectSpecs[plural], caseID, body)
		if err != nil {
			h.fail(w, r, err)
			return
		}
		writeJSON(w, http.StatusCreated, obj)
	}
}

func (h *Handler) getObject(plural string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sc, ok := h.scope(w, r, "")
		if !ok {
			return
		}
		obj, err := h.Svc.GetObject(r.Context(), sc, objectSpecs[plural], r.PathValue("id"))
		if err != nil {
			h.fail(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, obj)
	}
}

func (h *Handler) patchObject(plural string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := readMap(r, false)
		if err != nil {
			h.fail(w, r, err)
			return
		}
		sc, ok := h.scope(w, r, "")
		if !ok {
			return
		}
		obj, err := h.Svc.PatchObject(r.Context(), sc, objectSpecs[plural], r.PathValue("id"), body)
		if err != nil {
			h.fail(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, obj)
	}
}

func (h *Handler) deleteObject(plural string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		reason, err := reasonOf(r)
		if err != nil {
			h.fail(w, r, err)
			return
		}
		sc, ok := h.scope(w, r, "")
		if !ok {
			return
		}
		if err := h.Svc.DeleteObject(r.Context(), sc, objectSpecs[plural], r.PathValue("id"), reason); err != nil {
			h.fail(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func (h *Handler) reviewFinding(w http.ResponseWriter, r *http.Request) {
	m, err := readMap(r, false)
	if err == nil {
		err = onlyKeys(m, "state", "note")
	}
	var state, note string
	if err == nil {
		if state, _, err = str(m, "state"); err == nil {
			note, _, err = str(m, "note")
		}
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	sc, ok := h.scope(w, r, "")
	if !ok {
		return
	}
	obj, err := h.Svc.ReviewFinding(r.Context(), sc, r.PathValue("id"), state, note)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, obj)
}

func (h *Handler) revokeAuthority(w http.ResponseWriter, r *http.Request) {
	m, err := readMap(r, false)
	if err == nil {
		err = onlyKeys(m, "reason")
	}
	var reason string
	if err == nil {
		reason, _, err = str(m, "reason")
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	sc, ok := h.scope(w, r, "")
	if !ok {
		return
	}
	obj, err := h.Svc.RevokeAuthority(r.Context(), sc, r.PathValue("id"), reason)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, obj)
}

// ---- tags, members, audit ---------------------------------------------------------------------------------------------------------------------------------------------

func (h *Handler) listTags(w http.ResponseWriter, r *http.Request) {
	caseID := r.PathValue("case_id")
	sc, ok := h.scope(w, r, caseID)
	if !ok {
		return
	}
	items, err := h.Svc.ListTags(r.Context(), sc, caseID)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) patchTag(w http.ResponseWriter, r *http.Request) {
	m, err := readMap(r, false)
	if err == nil {
		err = onlyKeys(m, "name", "color")
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var name, color *string
	var clearColor bool
	if v, present, serr := str(m, "name"); serr != nil {
		h.fail(w, r, serr)
		return
	} else if present {
		name = &v
	}
	if raw, present := m["color"]; present {
		if string(raw) == "null" {
			clearColor = true
		} else {
			v, _, serr := str(m, "color")
			if serr != nil {
				h.fail(w, r, serr)
				return
			}
			color = &v
		}
	}
	sc, ok := h.scope(w, r, "")
	if !ok {
		return
	}
	t, err := h.Svc.PatchTag(r.Context(), sc, r.PathValue("id"), name, color, clearColor)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (h *Handler) getObjectTags(w http.ResponseWriter, r *http.Request) {
	caseID := r.PathValue("case_id")
	sc, ok := h.scope(w, r, caseID)
	if !ok {
		return
	}
	tags, err := h.Svc.ObjectTags(r.Context(), sc, caseID, r.PathValue("kind"), r.PathValue("id"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tags": tags})
}

func (h *Handler) putObjectTags(w http.ResponseWriter, r *http.Request) {
	caseID := r.PathValue("case_id")
	m, err := readMap(r, false)
	if err == nil {
		err = onlyKeys(m, "tags")
	}
	var names []string
	if err == nil {
		if raw, present := m["tags"]; !present || json.Unmarshal(raw, &names) != nil {
			err = BadRequest("tags must be a list of names.")
		}
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	sc, ok := h.scope(w, r, caseID)
	if !ok {
		return
	}
	tags, err := h.Svc.SetObjectTags(r.Context(), sc, caseID, r.PathValue("kind"), r.PathValue("id"), names)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tags": tags})
}

func (h *Handler) listMembers(w http.ResponseWriter, r *http.Request) {
	caseID := r.PathValue("case_id")
	sc, ok := h.scope(w, r, caseID)
	if !ok {
		return
	}
	m, err := h.Svc.ListMembers(r.Context(), sc, caseID)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (h *Handler) putMember(w http.ResponseWriter, r *http.Request) {
	caseID := r.PathValue("case_id")
	m, err := readMap(r, false)
	if err == nil {
		err = onlyKeys(m, "role", "display_name")
	}
	var role, name string
	if err == nil {
		if role, _, err = str(m, "role"); err == nil {
			name, _, err = str(m, "display_name")
		}
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	sc, ok := h.scope(w, r, caseID)
	if !ok {
		return
	}
	member, err := h.Svc.PutMember(r.Context(), sc, caseID, r.PathValue("subject"), role, name)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, member)
}

func (h *Handler) deleteMember(w http.ResponseWriter, r *http.Request) {
	caseID := r.PathValue("case_id")
	sc, ok := h.scope(w, r, caseID)
	if !ok {
		return
	}
	if err := h.Svc.RemoveMember(r.Context(), sc, caseID, r.PathValue("subject")); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) listAudit(w http.ResponseWriter, r *http.Request) {
	caseID := r.PathValue("case_id")
	sc, ok := h.scope(w, r, caseID)
	if !ok {
		return
	}
	q := r.URL.Query()
	page, err := h.Svc.ListAudit(r.Context(), sc, caseID, AuditFilter{Action: q.Get("action"), ObjectKind: q.Get("object_kind"), ObjectID: q.Get("object_id"), Limit: atoiDefault(q.Get("limit"), 0), Cursor: q.Get("cursor")})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (h *Handler) verifyAudit(w http.ResponseWriter, r *http.Request) {
	caseID := r.PathValue("case_id")
	sc, ok := h.scope(w, r, caseID)
	if !ok {
		return
	}
	st, err := h.Svc.VerifyAudit(r.Context(), sc, caseID)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}
