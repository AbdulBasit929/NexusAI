package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The evidence behind a figure, on the REAL data of the Orion reference case (synthetic by construction), in a throwaway restored copy of the live database
// (tools/dev/live-window/rehearse-evidence.sh). The generated case of the executor's database spec has no statement files with title rows, so only this copy can show that the sheet row of a
// typed row is found by content and not by its position. The spec skips without ASK_EVIDENCE_ORION_DSN and refuses a database without the throwaway marker.
var _ = Describe("Ask engine evidence on the restored Orion case", Ordered, func() {
	const (
		tenant     = "default"
		collection = "orion-lahore-reference-20261030"
	)
	var (
		pool   *pgxpool.Pool
		cat    *askCatalogue
		ctx    = context.Background()
		limits = askDefaultLimits()
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
		limits.Timeout = 60 * time.Second
	})

	AfterAll(func() {
		if pool != nil {
			pool.Close()
		}
	})

	// each case: the query the engine wrote (or the verified example), what the evidence must show
	type orionCase struct {
		id, sql, kind string
		total         int    // the rows behind the figure, 0 = not asserted
		figure        string // text the rows must give back (a preview of the first row), "" = not asserted
		minLocated    float64
	}
	cases := []orionCase{
		{"longest", "SELECT max(duration_s) AS duration FROM v_comm_events WHERE event_type = 'voice'", "aggregate", 3871, "1800", 1},
		{"shortest", "SELECT min(duration_s) AS duration FROM v_comm_events WHERE event_type = 'voice'", "aggregate", 3871, "", 1},
		{"average", "SELECT avg(duration_s) AS mean FROM v_comm_events WHERE event_type = 'voice'", "aggregate", 3871, "", 1},
		{"over ten minutes", "SELECT count(*) AS n FROM v_comm_events WHERE event_type = 'voice' AND duration_s > 600", "aggregate", 19, "", 1},
		{"night", "SELECT count(*) AS n FROM v_comm_events WHERE event_type = 'voice' AND extract(hour FROM start_utc AT TIME ZONE 'Asia/Karachi') < 5", "aggregate", 197, "", 1},
		{"talk time", "SELECT sum(duration_s) AS total_talk_time FROM v_comm_events WHERE src = '923000001005' AND event_type = 'voice'", "aggregate", 0, "", 1},
		{"five longest", "SELECT src AS subscriber, dst AS other_party, direction, duration_s FROM v_comm_events WHERE event_type = 'voice' ORDER BY duration_s DESC, start_utc LIMIT 5", "rows", 5, "1800", 1},
		{"most outgoing", "SELECT src AS number, count(*) AS calls FROM v_comm_events WHERE event_type = 'voice' AND direction = 'outgoing' GROUP BY src ORDER BY calls DESC LIMIT 5", "group", 520, "", 1},
		{"all events", "SELECT count(*) AS n FROM v_comm_events", "aggregate", 5092, "", 1},
	}

	for _, c := range cases {
		c := c
		It("itemises '"+c.id+"', recomputes the figure from the rows and finds the sheet row of what it lists", func() {
			v, rej := askValidate(c.sql, cat, nil)
			Expect(rej).To(BeNil(), c.sql)
			result, err := askExecute(ctx, pool, tenant, collection, v, limits)
			Expect(err).NotTo(HaveOccurred())
			ev := askCollectEvidence(ctx, pool, tenant, collection, cat, v, result, limits)
			Expect(ev).NotTo(BeNil())
			Expect(ev.State).To(Equal("itemised"), ev.Reason)
			Expect(ev.Kind).To(Equal(c.kind))
			Expect(ev.Reconciled).To(Equal("yes"), ev.Check)
			if c.total > 0 {
				Expect(ev.Total).To(Equal(c.total))
			}
			located := 0
			for _, r := range ev.Rows {
				Expect(r.SourceFile).NotTo(BeEmpty())
				if r.SheetRow > 0 && r.RowHash != "" {
					located++
				}
			}
			Expect(ev.Rows).NotTo(BeEmpty())
			share := float64(located) / float64(len(ev.Rows))
			fmt.Printf("EVIDENCE %-17s kind=%-9s total=%-5d files=%-2d rows=%-2d located=%d (%.0f%%) reconciled=%s | %s\n", c.id, ev.Kind, ev.Total, len(ev.Files), len(ev.Rows), located, share*100, ev.Reconciled, ev.Check)
			Expect(share).To(BeNumerically(">=", c.minLocated), "every listed row is found in its sheet")
			if c.figure != "" {
				Expect(strings.Join([]string{ev.Rows[0].Preview}, " ")).To(ContainSubstring(c.figure), "the first row listed is the one the figure comes from")
			}
		})
	}

	It("says that a summary view is not itemised, instead of guessing", func() {
		v, rej := askValidate("SELECT dst AS number, events FROM v_contact_pairs WHERE src = '923000001005' ORDER BY events DESC LIMIT 5", cat, nil)
		Expect(rej).To(BeNil())
		result, err := askExecute(ctx, pool, tenant, collection, v, limits)
		Expect(err).NotTo(HaveOccurred())
		ev := askCollectEvidence(ctx, pool, tenant, collection, cat, v, result, limits)
		Expect(ev.State).To(Equal("not_itemised"))
		Expect(ev.Reason).To(ContainSubstring("summarises many records"))
	})

	It("never shows the rows of another collection", func() {
		v, rej := askValidate("SELECT count(*) AS n FROM v_comm_events", cat, nil)
		Expect(rej).To(BeNil())
		result, err := askExecute(ctx, pool, tenant, "no-such-collection", v, limits)
		Expect(err).NotTo(HaveOccurred())
		ev := askCollectEvidence(ctx, pool, tenant, "no-such-collection", cat, v, result, limits)
		Expect(ev).NotTo(BeNil())
		Expect(ev.Rows).To(BeEmpty())
		Expect(ev.Total).To(BeZero())
	})
})
