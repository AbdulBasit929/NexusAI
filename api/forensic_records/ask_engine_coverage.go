package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
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
	// AnyVolume is whether any typed internet record states a byte count in a direction. An export that gives one undirected volume keeps it in the raw row (the entry does not
	// guess a direction), so the view has nothing to add up for it (found 2026-10-20 on the multimodal case: a sum of the volume answered "no value").
	AnyVolume bool
	// Unpopulated names, as "view.column", the columns that no row of the case fills although the case has rows in the table behind the view (an export that does not state them).
	// A query that reads such a column filters or adds up nothing: the engine declines it with the reason instead of stating "no value" or a zero. nil means every column is filled
	// (a stub in a spec; a case with no rows in the table is the coverage guard's business).
	Unpopulated map[string]bool
	// Repeats is how many rows of the case's typed call and internet records are flagged as exact repeats and left out of the views; the answer says so.
	Repeats int64
}

// askPopulationColumns are the view columns whose emptiness makes a filter or an aggregate meaningless, by the typed table that holds them.
var askPopulationColumns = map[string][]string{
	"comm": {"v_comm_events.duration_s", "v_comm_events.end_utc", "v_comm_events.imsi", "v_comm_events.imei", "v_comm_events.cell_key", "v_device_pairings.imsi", "v_device_pairings.imei", "v_contact_pairs.total_duration_s"},
	"session": {"v_data_sessions.imsi", "v_data_sessions.imei", "v_data_sessions.public_ip", "v_data_sessions.public_port_from", "v_data_sessions.ue_ip", "v_data_sessions.server_ip", "v_data_sessions.server_port",
		"v_data_sessions.app_category", "v_data_sessions.app", "v_data_sessions.host", "v_data_sessions.bytes_up", "v_data_sessions.bytes_down", "v_data_sessions.duration_ms", "v_data_sessions.end_utc", "v_data_sessions.cell_key"},
	"usage": {"v_app_usage.imei", "v_app_usage.app_category", "v_app_usage.app", "v_app_usage.app_feature", "v_app_usage.end_utc", "v_app_usage.province", "v_app_usage.city"},
}

// the probes: one scan of each typed table of the case, the number of rows and, per tracked column, the number of rows that hold a value (same order as askPopulationColumns)
const askPopulationComm = `SELECT count(*), count(duration_s), count(end_utc), count(imsi), count(imei), count(cell_key), count(imsi), count(imei), count(duration_s)
FROM forensic.comm_event WHERE tenant_id = $1 AND collection_id = $2 AND dataset_kind = 'target_cdr' AND NOT ('duplicate_row' = ANY (quality_flags))`
const askPopulationSession = `SELECT count(*), count(imsi), count(imei), count(public_ip), count(public_port_from), count(ue_ip), count(server_ip), count(server_port), count(app_category), count(app), count(host),
count(bytes_up), count(bytes_down), count(duration_ms), count(end_utc), count(cell_key)
FROM forensic.data_session WHERE tenant_id = $1 AND collection_id = $2 AND NOT ('duplicate_row' = ANY (quality_flags))`
const askPopulationUsage = `SELECT count(*), count(imei), count(app_category), count(app), count(app_feature), count(end_utc), count(province), count(city)
FROM forensic.app_usage WHERE tenant_id = $1 AND collection_id = $2`
const askRepeatsQuery = `SELECT (SELECT count(*) FROM forensic.comm_event WHERE tenant_id = $1 AND collection_id = $2 AND dataset_kind = 'target_cdr' AND 'duplicate_row' = ANY (quality_flags))
 + (SELECT count(*) FROM forensic.data_session WHERE tenant_id = $1 AND collection_id = $2 AND 'duplicate_row' = ANY (quality_flags))`

// askReadPopulation fills Unpopulated and Repeats of c in the transaction of the coverage read.
func askReadPopulation(ctx context.Context, tx pgx.Tx, tenantID, collectionID string, c *askCoverage) error {
	c.Unpopulated = map[string]bool{}
	for _, probe := range []struct{ table, sql string }{{"comm", askPopulationComm}, {"session", askPopulationSession}, {"usage", askPopulationUsage}} {
		keys := askPopulationColumns[probe.table]
		counts := make([]int64, len(keys)+1)
		dest := make([]any, len(counts))
		for i := range counts {
			dest[i] = &counts[i]
		}
		if err := tx.QueryRow(ctx, probe.sql, tenantID, collectionID).Scan(dest...); err != nil {
			return err
		}
		if counts[0] == 0 {
			continue // no rows in this table: nothing to say about its columns
		}
		for i, key := range keys {
			if counts[i+1] == 0 {
				c.Unpopulated[key] = true
			}
		}
	}
	return tx.QueryRow(ctx, askRepeatsQuery, tenantID, collectionID).Scan(&c.Repeats)
}

// askPopulationGap says which column of the query no row of the case fills; "" when none. A column is named only when every view the query reads is tracked for it and empty for the case:
// a column of the same name that another view fills, or a view that is not tracked, never causes a decline.
func askPopulationGap(c askCoverage, v *askValidated) string {
	if len(c.Unpopulated) == 0 || v == nil {
		return ""
	}
	tracked := map[string]bool{}
	for _, keys := range askPopulationColumns {
		for _, k := range keys {
			tracked[k] = true
		}
	}
	for _, col := range v.Columns {
		empty, filled := []string{}, false
		for _, view := range v.Views {
			key := view + "." + col
			switch {
			case c.Unpopulated[key]:
				empty = append(empty, key)
			case tracked[key]:
				filled = true
			}
		}
		if len(empty) > 0 && !filled && len(empty) == len(v.Views) {
			return "no row of the case holds a value in " + empty[0] + " (the records do not state it), so a question that needs it cannot be answered from them"
		}
	}
	return ""
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
  EXISTS (SELECT 1 FROM forensic.app_usage u WHERE u.tenant_id = $1 AND u.collection_id = $2),
  EXISTS (SELECT 1 FROM forensic.data_session s WHERE s.tenant_id = $1 AND s.collection_id = $2 AND (s.bytes_up IS NOT NULL OR s.bytes_down IS NOT NULL))
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
	if err = tx.QueryRow(ctx, askCoverageQuery, tenantID, collectionID).Scan(&c.Files, &c.Typed, &c.AnyComm, &c.AnySession, &c.AnyUsage, &c.AnyVolume); err != nil {
		return c, err
	}
	err = askReadPopulation(ctx, tx, tenantID, collectionID, &c)
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

// askVolumeGap says why a question about volumes cannot be answered from the typed internet records of this case; "" when it can, or when the question is not about volumes.
func askVolumeGap(c askCoverage, f askFrame, views []string) string {
	if !f.Cues["volume"] || !c.AnySession || c.AnyVolume {
		return ""
	}
	for _, v := range views {
		if v == "v_data_sessions" {
			return "the case's internet records state no byte count per direction (an export with one undirected volume keeps it in the raw row)"
		}
	}
	return ""
}

// askCoverageGap says, for the views a question would read, why the analyst views cannot speak for this case; "" when they can.
func askCoverageGap(c askCoverage, views []string) string {
	for _, v := range views {
		switch v {
		case "v_comm_events", "v_contact_pairs", "v_contact_links", "v_connection_edges", "v_cell_visits", "v_colocation_windows", "v_meeting_candidates", "v_device_pairings":
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
