package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ASK ENGINE V2: COVERAGE, OR WHETHER THE ANALYST VIEWS CAN SPEAK FOR THIS CASE.
//
// The analyst views (migration 017) read the TYPED tables only (forensic.comm_event, data_session, app_usage). A case ingested before the typed writers existed holds its
// call and internet records in the version-1 table and has no typed rows at all: a query over the views would count zero, and the engine would answer "0" or "no record of
// this number" about a case that holds thousands. Found on 2026-10-15 while checking the engine on the live collections: the demonstration cases (8,642 call records, 497 ...)
// have no typed rows, and the real-data collection has them all. The same hazard exists for a case that is only partly typed.
//
// So before the engine spends a second of model time, it asks one question of the case, with the connection's own scope: for each family that a chosen view reads, does the
// case hold typed rows, and is every evidence file that was detected as that family (cdr, ipdr) among those that have typed rows? If not, the engine DECLINES, with the reason,
// and the existing path answers as it would without the engine. A decline costs nothing; a confident zero is the defect the engine exists to remove.
type askCoverage struct {
	// Files is the structured files of the case that version 1 detected as call or internet records, Typed how many of them have rows in ANY typed table. (Version 1 calls
	// every call, internet and usage export "cdr", so the typed table that holds a file cannot be read from its detected type.)
	Files, Typed        int
	AnyComm, AnySession bool
	AnyUsage            bool
}

const askCoverageQuery = `
SELECT
  count(*) FILTER (WHERE e.modality = 'structured_records' AND e.detected_type IN ('cdr', 'ipdr')),
  count(*) FILTER (WHERE e.modality = 'structured_records' AND e.detected_type IN ('cdr', 'ipdr') AND (
    EXISTS (SELECT 1 FROM forensic.comm_event c WHERE c.tenant_id = e.tenant_id AND c.evidence_id = e.evidence_id)
    OR EXISTS (SELECT 1 FROM forensic.data_session s WHERE s.tenant_id = e.tenant_id AND s.evidence_id = e.evidence_id)
    OR EXISTS (SELECT 1 FROM forensic.app_usage u WHERE u.tenant_id = e.tenant_id AND u.evidence_id = e.evidence_id))),
  EXISTS (SELECT 1 FROM forensic.comm_event c WHERE c.tenant_id = $1 AND c.collection_id = $2 AND c.dataset_kind = 'target_cdr' AND NOT ('duplicate_row' = ANY (c.quality_flags))),
  EXISTS (SELECT 1 FROM forensic.data_session s WHERE s.tenant_id = $1 AND s.collection_id = $2),
  EXISTS (SELECT 1 FROM forensic.app_usage u WHERE u.tenant_id = $1 AND u.collection_id = $2)
FROM forensic.evidence_items e
WHERE e.tenant_id = $1 AND e.collection_id = $2 AND coalesce(e.processing_status, '') <> 'failed'`

// askCoverageRead reads the coverage of one case. A variable only so that a spec can stand in for the database.
var askCoverageRead = func(ctx context.Context, db *pgxpool.Pool, tenantID, collectionID string) (askCoverage, error) {
	var c askCoverage
	tx, err := db.Begin(ctx)
	if err != nil {
		return c, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // read only
	if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenantID); err != nil {
		return c, err
	}
	err = tx.QueryRow(ctx, askCoverageQuery, tenantID, collectionID).Scan(&c.Files, &c.Typed, &c.AnyComm, &c.AnySession, &c.AnyUsage)
	return c, err
}

var askCoverageCache struct {
	sync.Mutex
	entries map[string]askCoverageEntry
}

type askCoverageEntry struct {
	c  askCoverage
	at time.Time
}

const askCoverageTTL = 30 * time.Second

func askCoverageFor(ctx context.Context, db *pgxpool.Pool, tenantID, collectionID string) (askCoverage, error) {
	key := tenantID + "\x00" + collectionID
	askCoverageCache.Lock()
	if e, ok := askCoverageCache.entries[key]; ok && time.Since(e.at) < askCoverageTTL {
		askCoverageCache.Unlock()
		return e.c, nil
	}
	askCoverageCache.Unlock()
	c, err := askCoverageRead(ctx, db, tenantID, collectionID)
	if err != nil {
		return c, err
	}
	askCoverageCache.Lock()
	if askCoverageCache.entries == nil {
		askCoverageCache.entries = map[string]askCoverageEntry{}
	}
	askCoverageCache.entries[key] = askCoverageEntry{c: c, at: time.Now()}
	askCoverageCache.Unlock()
	return c, nil
}

// AnyComm is true only for TARGET call records: the views leave tower-dump rows out, so a case whose only typed call rows are a dump has nothing for v_comm_events (found
// 2026-10-15: the demonstration files, read by the registry as a dump layout, would have passed a check for "any call row" and answered zero).

// askCoverageGap says, for the views a question would read, why the analyst views cannot speak for this case; "" when they can.
func askCoverageGap(c askCoverage, views []string) string {
	for _, v := range views {
		switch v {
		case "v_comm_events", "v_contact_pairs", "v_cell_visits", "v_device_pairings":
			if !c.AnyComm {
				return "the case has no typed call records"
			}
			if c.Typed < c.Files {
				return fmt.Sprintf("only %d of the case's %d call and internet record files are in the typed tables", c.Typed, c.Files)
			}
		case "v_data_sessions":
			if !c.AnySession {
				return "the case has no typed internet records"
			}
			if c.Typed < c.Files {
				return fmt.Sprintf("only %d of the case's %d call and internet record files are in the typed tables", c.Typed, c.Files)
			}
		case "v_app_usage":
			if !c.AnyUsage {
				return "the case has no typed application-usage records"
			}
		}
	}
	return ""
}
