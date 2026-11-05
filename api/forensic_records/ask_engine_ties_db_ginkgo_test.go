package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// A tie for the top place of a one-row ranking, against a real case database (engine round 9e, VB-PLAT-051): the LIMIT of the parser's own text is widened, validated again and run as the
// analyst role, and every row that shares the first row's figure is kept. The name starts with the harness's focus so that tools/case_generator/check_ask_engine.py runs it.
var _ = Describe("Ask engine executor on a case database: a tie for the top place", Ordered, func() {
	var (
		pool   *pgxpool.Pool
		truth  askTruth
		ctx    = context.Background()
		limits = askDefaultLimits()
		cat    *askCatalogue
	)

	BeforeAll(func() {
		dsn := os.Getenv("ASK_ENGINE_CASE_DSN") //nolint:forbidigo // opt-in switch for a throwaway-database spec
		if dsn == "" {
			Skip("requires the throwaway database of tools/case_generator/check_ask_engine.py")
		}
		cfg, err := pgxpool.ParseConfig(dsn)
		Expect(err).NotTo(HaveOccurred())
		Expect(cfg.ConnConfig.Host).To(Equal("127.0.0.1"))
		pool, err = pgxpool.NewWithConfig(ctx, cfg)
		Expect(err).NotTo(HaveOccurred())
		var marker bool
		Expect(pool.QueryRow(ctx, "SELECT to_regclass('public.nx_throwaway_marker') IS NOT NULL").Scan(&marker)).To(Succeed())
		Expect(marker).To(BeTrue(), "this is not a throwaway database")
		raw, err := os.ReadFile(os.Getenv("ASK_ENGINE_CASE_TRUTH")) //nolint:forbidigo // set by the harness only
		Expect(err).NotTo(HaveOccurred())
		Expect(json.Unmarshal(raw, &truth)).To(Succeed())
		Expect(truth.Subjects).NotTo(BeEmpty())
		limits.Timeout = 20 * time.Second
		cat, err = loadAskCatalogue(ctx, pool)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterAll(func() {
		if pool != nil {
			pool.Close()
		}
	})

	validated := func(sql string) *askValidated {
		v, rej := askValidate(sql, cat, nil)
		Expect(rej).To(BeNil(), sql)
		return v
	}

	It("keeps every contact that shares the top figure, from the same query with a wider LIMIT", func() {
		subject := truth.Subjects[0].MSISDN
		// every contact has the figure 1, so the whole contact list ties for the top place
		ranking := validated(fmt.Sprintf("SELECT dst AS label, 1 AS value FROM v_contact_pairs WHERE src = '%s' AND NOT is_service GROUP BY dst ORDER BY value DESC LIMIT 1", subject))
		first, err := askExecute(ctx, pool, truth.Tenant, truth.CaseID, ranking, limits)
		Expect(err).NotTo(HaveOccurred())
		Expect(first.Rows).To(HaveLen(1))
		got, tied := askExtendTies(ctx, pool, truth.Tenant, truth.CaseID, cat, nil, askExtractFrame("Which number did "+subject+" talk to the most?"), ranking, first, limits)
		count, err := askExecute(ctx, pool, truth.Tenant, truth.CaseID, validated(fmt.Sprintf("SELECT count(DISTINCT dst) AS n FROM v_contact_pairs WHERE src = '%s' AND NOT is_service", subject)), limits)
		Expect(err).NotTo(HaveOccurred())
		contacts, convErr := strconv.Atoi(fmt.Sprint(count.Rows[0][0]))
		Expect(convErr).NotTo(HaveOccurred())
		Expect(contacts).To(BeNumerically(">=", 2), "the subject has more than one contact")
		want := min(contacts, 1+askTieWindow)
		Expect(tied).To(Equal(want))
		Expect(got.Rows).To(HaveLen(want))
		Expect(got.Truncated).To(Equal(want == 1+askTieWindow))
	})

	It("leaves a ranking with a single winner as it is", func() {
		for _, s := range truth.Subjects {
			two, err := askExecute(ctx, pool, truth.Tenant, truth.CaseID, validated(fmt.Sprintf("SELECT dst AS label, events AS value FROM v_contact_pairs WHERE src = '%s' AND NOT is_service ORDER BY value DESC LIMIT 2", s.MSISDN)), limits)
			Expect(err).NotTo(HaveOccurred())
			if len(two.Rows) < 2 || fmt.Sprint(two.Rows[0][1]) == fmt.Sprint(two.Rows[1][1]) {
				continue // a subject whose top two contacts tie is the other spec's case
			}
			ranking := validated(fmt.Sprintf("SELECT dst AS label, events AS value FROM v_contact_pairs WHERE src = '%s' AND NOT is_service ORDER BY events DESC LIMIT 1", s.MSISDN))
			first, err := askExecute(ctx, pool, truth.Tenant, truth.CaseID, ranking, limits)
			Expect(err).NotTo(HaveOccurred())
			got, tied := askExtendTies(ctx, pool, truth.Tenant, truth.CaseID, cat, nil, askExtractFrame("Who does "+s.MSISDN+" talk to most?"), ranking, first, limits)
			Expect(tied).To(BeZero())
			Expect(got).To(BeIdenticalTo(first))
			return
		}
		Fail("no subject of the case has a single top contact")
	})
})
