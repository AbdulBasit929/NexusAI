package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// These specs run the lane's OWN Go-generated CTE against the effective call-record view on a real database. They need a throwaway database built by
// tools/case_generator/check_lane_on_typed_view.py (migrations 001 to 016 and a generated case written by the production typed ingest) and skip without one.
// The harness leaves a marker table in the database; the specs refuse any database that has none, so they cannot be pointed at a case database.
var _ = Describe("Typed families on a real database", Ordered, func() {
	var (
		pool       *pgxpool.Pool
		tenant     = "case"
		collection string
		subject    string
		ctx        = context.Background()
	)

	BeforeAll(func() {
		dsn := os.Getenv("TYPED_FAMILIES_TEST_DSN") //nolint:forbidigo // opt-in switch for a throwaway-database spec
		if dsn == "" {
			Skip("requires the throwaway database of tools/case_generator/check_lane_on_typed_view.py")
		}
		collection = os.Getenv("TYPED_FAMILIES_TEST_COLLECTION") //nolint:forbidigo // set by the same harness
		subject = os.Getenv("TYPED_FAMILIES_TEST_SUBJECT")       //nolint:forbidigo // set by the same harness
		Expect(collection).NotTo(BeEmpty())
		Expect(subject).NotTo(BeEmpty())
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

	// lane builds the lane's view of the call-record family exactly as runGovernedSQLLane does, and returns its CTE with the arguments it needs.
	lane := func() (*govSQLView, string, []any) {
		layer, _ := defaultSemanticLayer()
		Expect(layer).NotTo(BeNil())
		req := hybridQueryRequest{TenantID: tenant, CollectionID: collection}
		all := govSQLViews(layer, req)
		govSQLApplyTypedFamilies(ctx, pool, all)
		for _, view := range all {
			if view.Family == "communications_cdr" {
				cte, args, err := govSQLCTE(view, req, nil)
				Expect(err).NotTo(HaveOccurred())
				return view, cte, args
			}
		}
		Fail("the curated layer has no call-record view")
		return nil, "", nil
	}
	column := func(view *govSQLView, fieldID string) string {
		for _, c := range view.Columns {
			if c.FieldID == fieldID {
				return c.Name
			}
		}
		Fail("the view has no column for " + fieldID)
		return ""
	}
	// run executes one query as the lane would: a read-only transaction with the tenant set, the CTE in front.
	run := func(cte string, args []any, tail string, extra ...any) [][]any {
		tx, err := pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
		Expect(err).NotTo(HaveOccurred())
		defer func() { _ = tx.Rollback(ctx) }()
		_, err = tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenant)
		Expect(err).NotTo(HaveOccurred())
		sql := "WITH " + cte + " " + strings.ReplaceAll(tail, "$$", fmt.Sprintf("$%d", len(args)+1))
		rows, err := tx.Query(ctx, sql, append(append([]any{}, args...), extra...)...)
		Expect(err).NotTo(HaveOccurred(), sql)
		defer rows.Close()
		var out [][]any
		for rows.Next() {
			values, err := rows.Values()
			Expect(err).NotTo(HaveOccurred())
			out = append(out, values)
		}
		Expect(rows.Err()).NotTo(HaveOccurred())
		return out
	}
	direct := func(sql string, args ...any) [][]any {
		tx, err := pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
		Expect(err).NotTo(HaveOccurred())
		defer func() { _ = tx.Rollback(ctx) }()
		_, err = tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenant)
		Expect(err).NotTo(HaveOccurred())
		rows, err := tx.Query(ctx, sql, args...)
		Expect(err).NotTo(HaveOccurred(), sql)
		defer rows.Close()
		var out [][]any
		for rows.Next() {
			values, err := rows.Values()
			Expect(err).NotTo(HaveOccurred())
			out = append(out, values)
		}
		Expect(rows.Err()).NotTo(HaveOccurred())
		return out
	}

	const typedWhere = "tenant_id = 'case' AND collection_id = $1 AND dataset_kind = 'target_cdr' AND NOT ('duplicate_row' = ANY (quality_flags))"

	It("reads exactly the non-repeated target call records of the typed table", func() {
		GinkgoT().Setenv(typedFamiliesEnv, "communications_cdr")
		view, cte, args := lane()
		Expect(view.binding.Table).To(Equal("forensic.records_effective_cdr"))
		got := run(cte, args, "SELECT count(*) FROM "+view.Name)
		want := direct("SELECT count(*) FROM forensic.comm_event WHERE "+typedWhere, collection)
		Expect(want[0][0]).To(BeNumerically(">", 0))
		Expect(got[0][0]).To(Equal(want[0][0]))
	})

	It("reads a subject's counterparties, counts and first and last instants through the builders' own casts", func() {
		GinkgoT().Setenv(typedFamiliesEnv, "communications_cdr")
		view, cte, args := lane()
		msisdn, dialed, start := column(view, "cdr.msisdn"), column(view, "cdr.dialed_number"), column(view, "cdr.call_start")
		got := run(cte, args, fmt.Sprintf("SELECT %s, count(*), min(%s)::text, max(%s)::text FROM %s WHERE %s = $$ AND %s IS NOT NULL GROUP BY 1 ORDER BY 1", dialed, start, start, view.Name, msisdn, dialed), subject)
		want := direct("SELECT coalesce(b_norm, b_raw), count(*), min(start_utc)::text, max(start_utc)::text FROM forensic.comm_event WHERE "+typedWhere+
			" AND a_norm = $2 AND coalesce(b_norm, b_raw) IS NOT NULL GROUP BY 1 ORDER BY 1", collection, subject)
		Expect(want).NotTo(BeEmpty())
		Expect(got).To(Equal(want))
	})

	It("reads the event time column and the derived duration metric", func() {
		GinkgoT().Setenv(typedFamiliesEnv, "communications_cdr")
		view, cte, args := lane()
		eventTime, duration := column(view, "cdr.event_time"), column(view, "cdr.call_duration_seconds")
		got := run(cte, args, fmt.Sprintf("SELECT min(%s)::text, max(%s)::text, coalesce(sum(%s), 0)::bigint, count(%s) FROM %s", eventTime, eventTime, duration, duration, view.Name))
		want := direct("SELECT min(start_utc)::text, max(start_utc)::text, coalesce(sum(extract(epoch FROM end_utc - start_utc)), 0)::bigint, count(*) FILTER (WHERE end_utc IS NOT NULL) FROM forensic.comm_event WHERE "+typedWhere, collection)
		Expect(got).To(Equal(want))
		Expect(want[0][3]).To(BeNumerically(">", 0), "the case has calls with an end time, so the metric is exercised")
	})

	It("carries the provenance every cited answer needs", func() {
		GinkgoT().Setenv(typedFamiliesEnv, "communications_cdr")
		view, cte, args := lane()
		// (source_file is left out: the curated cdr.source_file field and the provenance column share the name, over forensic.records as well.)
		got := run(cte, args, "SELECT count(*), count(record_id), count(row_number), count(DISTINCT record_id), min(row_number) FROM "+view.Name)
		Expect(got[0][0]).To(BeNumerically(">", 0))
		Expect(got[0][1]).To(Equal(got[0][0]))
		Expect(got[0][2]).To(Equal(got[0][0]))
		Expect(got[0][3]).To(Equal(got[0][0]), "record ids are unique")
		Expect(got[0][4]).To(BeNumerically(">=", 2), "row numbers count the header as row 1, as version 1 does")
	})

	It("with the switch off, the same lane reads forensic.records and sees none of the typed rows", func() {
		GinkgoT().Setenv(typedFamiliesEnv, "")
		view, cte, args := lane()
		Expect(view.binding.Table).To(Equal("forensic.records"))
		got := run(cte, args, "SELECT count(*) FROM "+view.Name)
		Expect(got[0][0]).To(BeEquivalentTo(0))
	})
})
