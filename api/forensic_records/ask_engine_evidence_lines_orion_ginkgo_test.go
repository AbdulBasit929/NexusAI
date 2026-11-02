package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The records behind the lines of a summary, on the REAL data of the Orion reference case in a restored copy of the live database (tools/dev/live-window/rehearse-evidence.sh): one representative query
// for every summary view of the catalogue, each line's own numbers recomputed from the records behind it. A view whose lines the case does not have is reported, not failed. A line whose numbers do
// not come back from its records is a trace that disagrees with the definition of its view, and fails.
var _ = Describe("Ask engine evidence for summary views on the restored Orion case", Ordered, func() {
	const (
		tenant     = "default"
		collection = "orion-lahore-reference-20261030"
	)
	var (
		pool *pgxpool.Pool
		cat  *askCatalogue
		ctx  = context.Background()
	)

	BeforeAll(func() {
		dsn := os.Getenv("ASK_EVIDENCE_ORION_DSN") //nolint:forbidigo // opt-in switch for a throwaway-database spec
		if dsn == "" {
			Skip("requires the restored copy of tools/dev/live-window/rehearse-evidence.sh")
		}
		cfg, err := pgxpool.ParseConfig(dsn)
		Expect(err).NotTo(HaveOccurred())
		Expect(cfg.ConnConfig.Host).To(Equal("127.0.0.1"))
		pool, err = pgxpool.NewWithConfig(ctx, cfg)
		Expect(err).NotTo(HaveOccurred())
		var marker bool
		Expect(pool.QueryRow(ctx, "SELECT to_regclass('public.nx_throwaway_marker') IS NOT NULL").Scan(&marker)).To(Succeed())
		Expect(marker).To(BeTrue(), "this is not a throwaway database")
		cat, err = loadAskCatalogue(ctx, pool)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterAll(func() {
		if pool != nil {
			pool.Close()
		}
	})

	type summary struct {
		view, sql string
		settings  map[string]string
		count     bool // the answer is the number of lines
	}
	subscriber := "923000001005"
	account := "PK00SYNT0000000000002010"
	cases := []summary{
		{view: "v_contact_pairs", sql: "SELECT dst AS number, events FROM v_contact_pairs WHERE src = '" + subscriber + "' AND NOT is_service ORDER BY events DESC LIMIT 5"},
		{view: "v_contact_pairs", sql: "SELECT count(*) AS n FROM v_contact_pairs WHERE src = '" + subscriber + "' AND NOT is_service", count: true},
		{view: "v_contact_pairs (common)", sql: "SELECT x.dst AS number, x.events AS events_first, y.events AS events_second FROM v_contact_pairs x JOIN v_contact_pairs y ON y.dst = x.dst WHERE x.src = '923000001000' AND y.src = '" + subscriber + "' AND NOT x.is_service AND NOT x.is_hub AND x.dst NOT IN ('923000001000', '" + subscriber + "') ORDER BY x.events + y.events DESC LIMIT 20", settings: map[string]string{"app.hub_threshold": "10"}},
		{view: "v_contact_links", sql: "SELECT b AS number, events FROM v_contact_links WHERE a = '" + subscriber + "' ORDER BY events DESC LIMIT 5"},
		{view: "v_connection_edges", sql: "SELECT src, dst FROM v_connection_edges WHERE a = '923000001000' AND b = '923000001020' AND rank_any = 1 AND hops <= 3 LIMIT 200"},
		{view: "v_account_counterparties", sql: "SELECT counterparty, transactions, total_amount FROM v_account_counterparties WHERE account = '" + account + "' AND direction = 'debit' ORDER BY total_amount DESC LIMIT 5"},
		{view: "v_account_activity", sql: "SELECT day, transactions, credits, debits FROM v_account_activity WHERE account = '" + account + "' ORDER BY day LIMIT 5"},
		{view: "v_account_flows", sql: "SELECT account, credit_id, debit_id FROM v_account_flows ORDER BY credit_id LIMIT 5"},
		{view: "v_account_limit_bands", sql: "SELECT account, transactions FROM v_account_limit_bands ORDER BY account LIMIT 5", settings: map[string]string{"app.limit_amount": "400000"}},
		{view: "v_cell_visits", sql: "SELECT cell_key, hour_utc, events FROM v_cell_visits WHERE src = '" + subscriber + "' ORDER BY events DESC LIMIT 5"},
		{view: "v_cells", sql: "SELECT cell_key, site_name FROM v_cells ORDER BY cell_key LIMIT 3"},
		{view: "v_colocation_windows", sql: "SELECT a, b, cell_key, start_utc, end_utc FROM v_colocation_windows ORDER BY start_utc LIMIT 5", settings: map[string]string{"app.colocation_minutes": "15"}},
		{view: "v_meeting_candidates", sql: "SELECT a, b, cell_key, start_utc, last_contact_utc FROM v_meeting_candidates ORDER BY score DESC LIMIT 5", settings: map[string]string{"app.colocation_minutes": "15", "app.meeting_gap_minutes": "60"}},
		{view: "v_silent_numbers", sql: "SELECT number, last_event_utc FROM v_silent_numbers ORDER BY last_event_utc LIMIT 3", settings: map[string]string{"app.silence_days": "14"}},
		{view: "v_successor_candidates", sql: "SELECT old_number, new_number, score FROM v_successor_candidates ORDER BY score DESC LIMIT 3", settings: map[string]string{"app.silence_days": "14", "app.successor_window_days": "14"}},
		{view: "v_weekly_anchors", sql: "SELECT number, slot, week, cell_key FROM v_weekly_anchors WHERE number = '" + subscriber + "' ORDER BY week LIMIT 3"},
		{view: "v_movement_anchors", sql: "SELECT number, slot, cell_key, first_week FROM v_movement_anchors ORDER BY number LIMIT 3", settings: map[string]string{"app.hold_weeks": "2"}},
		{view: "v_anchor_changes", sql: "SELECT number, slot, to_cell_key FROM v_anchor_changes ORDER BY number LIMIT 3", settings: map[string]string{"app.hold_weeks": "2"}},
		{view: "v_anchor_runs", sql: "SELECT number, slot, cell_key, first_week, last_week FROM v_anchor_runs ORDER BY number LIMIT 3", settings: map[string]string{"app.hold_weeks": "2"}},
		{view: "v_anchor_deviations", sql: "SELECT number, slot, cell_key FROM v_anchor_deviations ORDER BY number LIMIT 3", settings: map[string]string{"app.hold_weeks": "2"}},
		{view: "v_rhythm_changes", sql: "SELECT number, first_day FROM v_rhythm_changes ORDER BY number LIMIT 3", settings: map[string]string{"app.rhythm_t": "5", "app.rhythm_ratio_percent": "200", "app.rhythm_min_side_days": "14"}},
		{view: "v_speed_alerts", sql: "SELECT number, from_time_utc, speed_kmh FROM v_speed_alerts ORDER BY speed_kmh DESC LIMIT 3", settings: map[string]string{"app.limit_kmh": "120"}},
		{view: "v_handsets", sql: "SELECT imei, events FROM v_handsets WHERE src = '" + subscriber + "' ORDER BY events DESC LIMIT 3"},
		{view: "v_device_pairings", sql: "SELECT imei, imsi, events FROM v_device_pairings WHERE src = '" + subscriber + "' ORDER BY events DESC LIMIT 3"},
		{view: "v_number_registration_activity", sql: "SELECT msisdn, registrations, sources FROM v_number_registration_activity ORDER BY msisdn LIMIT 3"},
		{view: "v_plate_visits", sql: "SELECT plate_norm, camera_id, sightings FROM v_plate_visits ORDER BY sightings DESC LIMIT 3"},
		{view: "v_document_values", sql: "SELECT document, kind, value, mentions FROM v_document_values ORDER BY document LIMIT 3"},
		{view: "v_document_links", sql: "SELECT kind, value, family, records FROM v_document_links ORDER BY kind, value, family LIMIT 9"},
		{view: "v_evidence_inventory", sql: "SELECT count(*) AS n FROM v_evidence_inventory WHERE modality = 'document'", count: true},
		{view: "v_evidence_inventory (list)", sql: "SELECT original_filename FROM v_evidence_inventory ORDER BY original_filename LIMIT 3"},
	}

	It("opens the records of another line of an answer through the route, for the scope that asked and no other", func() {
		limits := askDefaultLimits()
		limits.Timeout = 60 * time.Second
		v, rej := askValidate("SELECT dst AS number, events FROM v_contact_pairs WHERE src = '923000001005' AND NOT is_service ORDER BY events DESC LIMIT 5", cat, nil)
		Expect(rej).To(BeNil())
		result, err := askExecute(ctx, pool, tenant, collection, v, limits)
		Expect(err).NotTo(HaveOccurred())
		ev := askCollectEvidence(ctx, pool, tenant, collection, cat, v, result, limits)
		Expect(ev.State).To(Equal("itemised"), ev.Reason)
		Expect(ev.LinesToken).To(HaveLen(32))
		Expect(len(ev.Lines)).To(BeNumerically(">=", 2))
		second := ev.Lines[1]
		get := func(query string) *httptest.ResponseRecorder {
			rec := httptest.NewRecorder()
			askLineHandler(pool)(rec, httptest.NewRequest(http.MethodGet, "/query/evidence-line?"+query, nil))
			return rec
		}
		base := "token=" + ev.LinesToken + "&tenant_id=" + tenant + "&collection_id=" + collection
		rec := get(base + "&line=2")
		Expect(rec.Code).To(Equal(http.StatusOK), rec.Body.String())
		var body struct {
			Line     int         `json:"line"`
			Label    string      `json:"label"`
			Evidence askEvidence `json:"evidence"`
		}
		Expect(json.Unmarshal(rec.Body.Bytes(), &body)).To(Succeed())
		Expect(body.Line).To(Equal(2))
		Expect(body.Label).To(Equal(second.Label))
		Expect(body.Evidence.Total).To(Equal(second.Records), "the records listed for a line are the records counted for it")
		Expect(body.Evidence.Rows).NotTo(BeEmpty())
		located := 0
		for _, r := range body.Evidence.Rows {
			if r.SheetRow > 0 && r.RowHash != "" && r.VersionID != "" {
				located++
			}
		}
		Expect(located).To(Equal(len(body.Evidence.Rows)), "every record opened for this line is found in its sheet")
		fmt.Printf("DRILL line 2 %s: %d records, %d listed, all located\n", body.Label, body.Evidence.Total, len(body.Evidence.Rows))
		Expect(get(base + "&line=99").Code).To(Equal(http.StatusBadRequest))
		Expect(get("token="+ev.LinesToken+"&tenant_id="+tenant+"&collection_id=another-case&line=2").Code).To(Equal(http.StatusNotFound), "another case cannot open these lines")
		Expect(get("token=00000000000000000000000000000000&tenant_id=" + tenant + "&collection_id=" + collection + "&line=1").Code).To(Equal(http.StatusNotFound))
	})

	for _, c := range cases {
		c := c
		It("traces the lines of "+c.view+" to their records and gets each line's own numbers back", func() {
			limits := askDefaultLimits()
			limits.Timeout = 60 * time.Second
			limits.Settings = c.settings
			v, rej := askValidate(c.sql, cat, nil)
			Expect(rej).To(BeNil(), c.sql)
			result, err := askExecute(ctx, pool, tenant, collection, v, limits)
			Expect(err).NotTo(HaveOccurred())
			if len(result.Rows) == 0 || (len(result.Rows) == 1 && askRowAllNil(result.Rows[0])) {
				fmt.Printf("LINES %-34s the case has no line for this query\n", c.view)
				return
			}
			ev := askCollectEvidence(ctx, pool, tenant, collection, cat, v, result, limits)
			Expect(ev).NotTo(BeNil())
			Expect(ev.State).To(Equal("itemised"), c.view+": "+ev.Reason)
			Expect(ev.Kind).To(Equal("lines"))
			Expect(ev.Lines).NotTo(BeEmpty())
			var shown []string
			disagree := 0
			for _, l := range ev.Lines {
				shown = append(shown, fmt.Sprintf("%s=%d/%df:%s", l.Label, l.Records, l.Files, l.Reconciled))
				if l.Reconciled == "no" {
					disagree++
					fmt.Printf("DISAGREE %s line %d %s: %s\n", c.view, l.Index, l.Label, l.Check)
				}
			}
			first := ev.Lines[0]
			fmt.Printf("LINES %-34s lines=%d total=%d reconciled=%s first=%d records in %d files, %d listed | %s\n", c.view, len(ev.Lines), ev.LinesTotal, ev.Reconciled, first.Records, first.Files, len(ev.Rows), govSQLClip(strings.Join(shown, "; "), 260))
			Expect(disagree).To(BeZero(), c.view+": a line's own numbers do not come back from the records behind it")
			Expect(ev.Reconciled).NotTo(Equal("no"), c.view+": "+ev.Check)
			if c.count {
				Expect(ev.Reconciled).To(Equal("yes"), ev.Check)
			}
			if ev.Lines[0].Records > 0 || ev.Lines[0].Pending {
				rows, say := ev.Rows, ev.Say
				if len(rows) == 0 {
					// the time ran out before the first line was opened (a heavy view on a busy machine): the workspace opens it itself, through the route
					rec := httptest.NewRecorder()
					askLineHandler(pool)(rec, httptest.NewRequest(http.MethodGet, "/query/evidence-line?token="+ev.LinesToken+"&line=1&tenant_id="+tenant+"&collection_id="+collection, nil))
					Expect(rec.Code).To(Equal(http.StatusOK), rec.Body.String())
					var body struct {
						Evidence askEvidence `json:"evidence"`
					}
					Expect(json.Unmarshal(rec.Body.Bytes(), &body)).To(Succeed())
					rows, say = body.Evidence.Rows, body.Evidence.Say
					fmt.Printf("      (%s: line 1 opened through the route)\n", c.view)
				}
				Expect(rows).NotTo(BeEmpty(), c.view+": the first line has records, so they are listed")
				located := 0
				for _, r := range rows {
					if r.SheetRow > 0 {
						located++
					}
				}
				fmt.Printf("      first line: %d of %d listed records found in their sheet; says: %s\n", located, len(rows), say)
			}
		})
	}
})
