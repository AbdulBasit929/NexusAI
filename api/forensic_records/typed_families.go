package main

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TYPED FAMILIES (roadmap 4.6, steps 3 to 5; docs/data/typed-family-switch.md).
//
// FORENSIC_TYPED_FAMILIES lists the evidence families (by their semantic-layer family name, comma separated) whose rows the GOVERNED SQL LANE reads
// from the typed tables instead of forensic.records. Default empty: nothing changes. A family is switched by changing the table its view reads to a
// database view with exactly the columns of forensic.records (migration 016), so no builder of the lane's SQL changes and every answer cites the
// table it came from (the provenance carries `table`).
//
// ONLY THE LANE. The typed compiler refuses a plan whose fields come from more than one table, so giving it a second table for one family would
// refuse plans that join that family to another; it stays on forensic.records until that rule is settled. A question the lane declines is therefore
// still answered by the existing path from version 1's rows.
//
// A family is switched only when its view exists in the database (checked, and re-checked every half minute): setting the variable before migration
// 016 is applied leaves the lane on forensic.records, with one warning in the log, instead of failing every question about the family.
const typedFamiliesEnv = "FORENSIC_TYPED_FAMILIES"

// typedEffectiveTables maps a family to the view that serves it. Nothing else may reach SQL.
var typedEffectiveTables = map[string]string{
	"communications_cdr": "forensic.records_effective_cdr",
}

const typedViewRecheck = 30 * time.Second

func typedFamiliesRequested() map[string]bool {
	requested := map[string]bool{}
	for _, name := range strings.Split(os.Getenv(typedFamiliesEnv), ",") { //nolint:forbidigo // forensic records product API reads its feature switches from the environment by design (docker compose declares it)
		if name = strings.ToLower(strings.TrimSpace(name)); name != "" {
			requested[name] = true
		}
	}
	return requested
}

type typedViewState struct {
	ready     bool
	checkedAt time.Time
	warned    bool
}

var (
	typedViewMu     sync.Mutex
	typedViewStates = map[string]typedViewState{}
	// typedViewExists asks the database whether the view is there. A variable so that a test can stand in for the database.
	typedViewExists = func(ctx context.Context, db *pgxpool.Pool, qualified string) bool {
		var exists bool
		if err := db.QueryRow(ctx, "SELECT to_regclass($1) IS NOT NULL", qualified).Scan(&exists); err != nil {
			return false
		}
		return exists
	}
	typedViewNow = time.Now
)

func typedViewReady(ctx context.Context, db *pgxpool.Pool, qualified string) bool {
	typedViewMu.Lock()
	state := typedViewStates[qualified]
	typedViewMu.Unlock()
	if !state.checkedAt.IsZero() && typedViewNow().Sub(state.checkedAt) < typedViewRecheck {
		return state.ready
	}
	ready := typedViewExists(ctx, db, qualified)
	typedViewMu.Lock()
	defer typedViewMu.Unlock()
	state = typedViewStates[qualified]
	if !ready && !state.warned {
		slog.Warn("FORENSIC_TYPED_FAMILIES names a family whose view does not exist; the lane stays on forensic.records for it (apply migration 016)", "view", qualified)
		state.warned = true
	}
	if ready {
		state.warned = false
	}
	state.ready, state.checkedAt = ready, typedViewNow()
	typedViewStates[qualified] = state
	return ready
}

// govSQLApplyTypedFamilies points the views of the requested families at their typed view. Derived views (model observations) are never switched.
func govSQLApplyTypedFamilies(ctx context.Context, db *pgxpool.Pool, views []*govSQLView) {
	requested := typedFamiliesRequested()
	if len(requested) == 0 || db == nil {
		return
	}
	for _, view := range views {
		table, switchable := typedEffectiveTables[view.Family]
		if !switchable || !requested[view.Family] || view.binding.Derived {
			continue
		}
		if typedViewReady(ctx, db, table) {
			view.binding.Table = table
		}
	}
}
