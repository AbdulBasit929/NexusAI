package main

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ASK ENGINE V2: THE FALLBACK ENTRY (the same shape as governedSQLFallback).
//
// FORENSIC_ASK_V2=true alone runs the engine AFTER the existing path: the request is answered exactly as it is today, and only when that answer is an abstention, a
// refusal or a server error is the engine tried. An answer the existing path gave is never replaced, so turning the switch on cannot change a single answer that was
// already given. When the engine declines too, the original reply goes out unchanged (and, if the governed SQL lane is on, the lane gets its turn after this).
// FORENSIC_ASK_V2_FIRST=true runs it before the existing path instead; that is handled inline in query.go.
//
// The wrapper buffers the handler's response. The handler hands back the authorised, bound request through the lane's holder in the context (one holder serves
// both entries), so the engine sees the same tenant, case and evidence scope as the old path and never re-derives them.
func askEngineFallback(core http.HandlerFunc, cfg config, db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !askV2Enabled() || askV2FirstEnabled() || db == nil || r.Method != http.MethodPost {
			core(w, r)
			return
		}
		startedAt := time.Now()
		holder := govSQLHolderFrom(r.Context())
		ctx := r.Context()
		if holder == nil {
			holder = &govSQLHolder{}
			ctx = context.WithValue(ctx, govSQLHolderKey{}, holder)
		}
		buffered := &govSQLBuffered{header: http.Header{}}
		core(buffered, r.WithContext(ctx))
		failed := buffered.status >= http.StatusInternalServerError && buffered.status < 600
		declined := buffered.status == http.StatusOK && govSQLOldPathDeclined(buffered.body.Bytes())
		if holder.req == nil || !(failed || declined) {
			buffered.flushTo(w)
			return
		}
		response, audit := askV2Run(r.Context(), cfg, db, *holder.req, startedAt)
		setAskV2Header(buffered, audit)
		if response == nil || (holder.reclassified && audit.State != "answered") {
			buffered.flushTo(w)
			return
		}
		for key, values := range buffered.header {
			w.Header()[key] = values
		}
		writeJSON(w, http.StatusOK, *response)
	}
}
