package main

import (
	"bytes"
	"context"
	"os"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// These specs read the analyst views of migration 017 on a real database: the catalogue the engine builds from the database's own comments must equal the
// pinned snapshot, every verified example must pass the validator and must run, as the read-only analyst role, in a read-only transaction. They need the
// throwaway database of tools/case_generator/check_ask_catalogue.py (which also writes the snapshot when asked) and skip without one. The harness leaves a
// marker table in the database; the specs refuse any database that has none, so they cannot be pointed at a case database.
var _ = Describe("Ask engine on a real database", Ordered, func() {
	var (
		pool *pgxpool.Pool
		ctx  = context.Background()
	)

	BeforeAll(func() {
		dsn := os.Getenv("ASK_ENGINE_TEST_DSN") //nolint:forbidigo // opt-in switch for a throwaway-database spec
		if dsn == "" {
			Skip("requires the throwaway database of tools/case_generator/check_ask_catalogue.py")
		}
		cfg, err := pgxpool.ParseConfig(dsn)
		Expect(err).NotTo(HaveOccurred())
		Expect(cfg.ConnConfig.Host).To(Equal("127.0.0.1"))
		pool, err = pgxpool.NewWithConfig(ctx, cfg)
		Expect(err).NotTo(HaveOccurred())
		var marker bool
		Expect(pool.QueryRow(ctx, "SELECT to_regclass('public.nx_throwaway_marker') IS NOT NULL").Scan(&marker)).To(Succeed())
		Expect(marker).To(BeTrue(), "this is not a throwaway database")
	})

	AfterAll(func() {
		if pool != nil {
			pool.Close()
		}
	})

	It("builds a complete catalogue from the database's own descriptions", func() {
		cat, err := loadAskCatalogue(ctx, pool)
		Expect(err).NotTo(HaveOccurred())
		names := []string{}
		for _, v := range cat.Views {
			names = append(names, v.Name)
		}
		Expect(names).To(Equal([]string{"v_account_activity", "v_account_counterparties", "v_account_flows", "v_account_limit_bands", "v_anchor_changes", "v_anchor_deviations", "v_anchor_runs", "v_app_usage", "v_bank_transactions", "v_cell_visits", "v_cells", "v_colocation_windows", "v_comm_events", "v_connection_edges", "v_contact_links", "v_contact_pairs", "v_data_sessions", "v_device_pairings", "v_evidence_inventory", "v_handsets", "v_meeting_candidates", "v_movement_anchors", "v_number_registration_activity", "v_plate_sightings", "v_plate_visits", "v_rhythm_changes", "v_silent_numbers", "v_speed_alerts", "v_subscriber_registrations", "v_successor_candidates", "v_weekly_anchors"}))
		text := cat.Render(nil)
		Expect(text).To(ContainSubstring("is_hub"))
		Expect(text).To(ContainSubstring("holds_from"))
	})

	It("matches the pinned snapshot, or writes it when asked", func() {
		cat, err := loadAskCatalogue(ctx, pool)
		Expect(err).NotTo(HaveOccurred())
		live, err := cat.JSON()
		Expect(err).NotTo(HaveOccurred())
		if target := os.Getenv("ASK_ENGINE_WRITE_GOLDEN"); target != "" { //nolint:forbidigo // set by the harness only
			Expect(os.WriteFile(target, live, 0o600)).To(Succeed())
			return
		}
		pinned, err := askGoldenCatalogue()
		Expect(err).NotTo(HaveOccurred())
		pinnedJSON, err := pinned.JSON()
		Expect(err).NotTo(HaveOccurred())
		Expect(bytes.Equal(live, pinnedJSON)).To(BeTrue(), "the analyst views or their descriptions changed: regenerate ask_engine_catalogue.golden.json with check_ask_catalogue.py --write-golden")
	})

	It("accepts every verified example and runs it as the read-only analyst role", func() {
		cat, err := loadAskCatalogue(ctx, pool)
		Expect(err).NotTo(HaveOccurred())
		examples, err := askExamples()
		Expect(err).NotTo(HaveOccurred())
		Expect(len(examples)).To(BeNumerically(">=", 6))
		run := func(example askExample, sql string) (columns []string, n int, runErr error) {
			tx, err := pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
			if err != nil {
				return nil, 0, err
			}
			defer func() { _ = tx.Rollback(ctx) }()
			for _, statement := range []string{
				"SET LOCAL ROLE forensic_analyst_ro",
				"SET LOCAL statement_timeout = '20s'",
				"SET LOCAL search_path = analyst, pg_catalog",
				"SELECT set_config('app.tenant_id', 'case', true)",
				"SELECT set_config('app.collection_id', 'ask-engine-spec', true)",
			} {
				if _, err = tx.Exec(ctx, statement); err != nil {
					return nil, 0, err
				}
			}
			rows, err := tx.Query(ctx, sql)
			if err != nil {
				return nil, 0, err
			}
			defer rows.Close()
			for _, fd := range rows.FieldDescriptions() {
				columns = append(columns, fd.Name)
			}
			for rows.Next() {
				n++
			}
			return columns, n, rows.Err()
		}
		for _, example := range examples {
			valid, rej := askValidate(example.SQL, cat, example.Views)
			Expect(rej).To(BeNil(), example.ID)
			columns, n, err := run(example, valid.SQL)
			Expect(err).NotTo(HaveOccurred(), example.ID)
			Expect(askShape(columns, n)).To(Equal(example.Shape), example.ID+" columns "+strings.Join(columns, ","))
		}
	})

	It("refuses what the role cannot reach, whatever the validator says", func() {
		tx, err := pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
		Expect(err).NotTo(HaveOccurred())
		defer func() { _ = tx.Rollback(ctx) }()
		_, err = tx.Exec(ctx, "SET LOCAL ROLE forensic_analyst_ro")
		Expect(err).NotTo(HaveOccurred())
		_, err = tx.Exec(ctx, "SELECT count(*) FROM forensic.comm_event")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("permission denied"))
	})
})
