package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// GOVERNED SQL LANE: THE FALLBACK ENTRY.
//
// FORENSIC_GOVERNED_SQL=true alone runs the lane AFTER the existing path: the request
// is answered exactly as it is today, and only when that answer is an abstention or a
// refusal is the lane tried. An answer the existing path gave is never replaced, so
// turning the switch on cannot change a single answer that was already given.
//
// (FORENSIC_GOVERNED_SQL_FIRST=true runs the lane before the existing path instead;
// that is handled inline in query.go. It is what the measurement of the lane's own
// accuracy uses, and the route to retiring the ladder if it earns it.)
//
// The wrapper buffers the handler's response. The handler hands back the authorised,
// bound request through a holder in the context, so the lane sees the same tenant,
// case and evidence scope the old path used and never re-derives them.
type govSQLHolder struct{ req *hybridQueryRequest }

type govSQLHolderKey struct{}

func govSQLHolderFrom(ctx context.Context) *govSQLHolder {
	holder, _ := ctx.Value(govSQLHolderKey{}).(*govSQLHolder)
	return holder
}

type govSQLBuffered struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (b *govSQLBuffered) Header() http.Header { return b.header }
func (b *govSQLBuffered) WriteHeader(status int) {
	if b.status == 0 {
		b.status = status
	}
}
func (b *govSQLBuffered) Write(data []byte) (int, error) {
	if b.status == 0 {
		b.status = http.StatusOK
	}
	return b.body.Write(data)
}

func (b *govSQLBuffered) flushTo(w http.ResponseWriter) {
	for key, values := range b.header {
		w.Header()[key] = values
	}
	status := b.status
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	_, _ = w.Write(b.body.Bytes())
}

// hybridQueryHandler is the public handler. With the lane off it is the core handler
// and nothing else.
func hybridQueryHandler(cfg config, db *pgxpool.Pool) http.HandlerFunc {
	return governedSQLFallback(hybridQueryHandlerCore(cfg, db), cfg, db)
}

// governedSQLFallback wraps a handler so that, when only FORENSIC_GOVERNED_SQL is on,
// the lane is tried after the handler declines. It takes the handler as a parameter so
// the behaviour can be tested without the whole query pipeline.
func governedSQLFallback(core http.HandlerFunc, cfg config, db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !governedSQLEnabled() || governedSQLFirstEnabled() || db == nil || r.Method != http.MethodPost {
			core(w, r)
			return
		}
		startedAt := time.Now()
		holder := &govSQLHolder{}
		buffered := &govSQLBuffered{header: http.Header{}}
		core(buffered, r.WithContext(context.WithValue(r.Context(), govSQLHolderKey{}, holder)))
		if holder.req == nil || buffered.status != http.StatusOK || !govSQLOldPathDeclined(buffered.body.Bytes()) {
			buffered.flushTo(w)
			return
		}
		laneResponse, laneAudit := runGovernedSQLLane(r.Context(), cfg, db, *holder.req, startedAt)
		setGovernedSQLHeader(buffered, laneAudit)
		if laneResponse == nil {
			buffered.flushTo(w)
			return
		}
		for key, values := range buffered.header {
			w.Header()[key] = values
		}
		writeJSON(w, http.StatusOK, *laneResponse)
	}
}

// govSQLOldPathDeclined reads the existing path's response and says whether it
// declined to answer: a clarification, a refusal, a withheld result.
func govSQLOldPathDeclined(body []byte) bool {
	var resp struct {
		Route         []string       `json:"route"`
		Intent        string         `json:"intent"`
		Clarification any            `json:"clarification"`
		Answer        map[string]any `json:"answer"`
		Enterprise    map[string]any `json:"enterprise"`
	}
	if json.Unmarshal(body, &resp) != nil {
		return false
	}
	for _, route := range resp.Route {
		if route == "clarification" || route == "verified_only_withheld" {
			return true
		}
	}
	if resp.Intent == string(intentClarify) || resp.Clarification != nil {
		return true
	}
	declinedStates := map[string]bool{"CLARIFICATION_REQUIRED": true, "UNSUPPORTED": true, "REFUSED": true, "WITHHELD": true, "NOT_ENOUGH_INFORMATION": true}
	if declinedStates[strings.ToUpper(stringValueAny(resp.Answer["result_state"]))] {
		return true
	}
	if resp.Enterprise != nil {
		if text := strings.TrimSpace(stringValueAny(resp.Enterprise["clarification"])); text != "" {
			return true
		}
		switch strings.ToLower(stringValueAny(resp.Enterprise["status"])) {
		case "needs_input", "unsupported", "withheld", "refused":
			return true
		}
	}
	return false
}
