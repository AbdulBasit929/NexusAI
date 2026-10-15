package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudler/LocalAI/pkg/forensicrequest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The executor, the probe and the whole run against a real database that holds a GENERATED case written by the production typed ingest, as the application role.
// They need the throwaway database of tools/case_generator/check_ask_engine.py (which also hands over the generator's planted-truth keys) and skip without one. The harness
// leaves a marker table in the database; the specs refuse any database that has none, so they cannot be pointed at a case database.
type askTruth struct {
	CaseID   string `json:"case_id"`
	Tenant   string `json:"tenant"`
	Subjects []struct {
		MSISDN string `json:"msisdn"`
	} `json:"subjects"`
	Oracle struct {
		A1 map[string][]struct {
			Number   string `json:"number"`
			Kind     string `json:"kind"`
			First    string `json:"first"`
			Last     string `json:"last"`
			Incoming int    `json:"incoming"`
			Outgoing int    `json:"outgoing"`
			Unknown  int    `json:"unknown"`
		} `json:"A1"`
	} `json:"oracle"`
}

var _ = Describe("Ask engine executor on a case database", Ordered, func() {
	var (
		pool   *pgxpool.Pool
		truth  askTruth
		ctx    = context.Background()
		limits = askDefaultLimits()
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
	})

	AfterAll(func() {
		if pool != nil {
			pool.Close()
		}
	})

	validated := func(sql string) *askValidated {
		cat, err := loadAskCatalogue(ctx, pool)
		Expect(err).NotTo(HaveOccurred())
		v, rej := askValidate(sql, cat, nil)
		Expect(rej).To(BeNil(), sql)
		return v
	}
	scoped := func(sql string) (*govSQLResult, error) {
		return askExecute(ctx, pool, truth.Tenant, truth.CaseID, validated(sql), limits)
	}
	pair := func() (subject, other string, events int, first string) {
		for _, s := range truth.Subjects {
			for _, p := range truth.Oracle.A1[s.MSISDN] {
				if p.Kind == "person_like" && p.Outgoing+p.Incoming >= 3 {
					return s.MSISDN, p.Number, p.Outgoing + p.Incoming + p.Unknown, p.First
				}
			}
		}
		Fail("the case has no pair with three events")
		return
	}

	It("returns, through the analyst role, the counts of the planted-truth key", func() {
		for _, s := range truth.Subjects {
			for _, p := range truth.Oracle.A1[s.MSISDN] {
				if p.Kind != "person_like" {
					continue
				}
				result, err := scoped(fmt.Sprintf("SELECT outgoing, incoming, unknown FROM v_contact_pairs WHERE src = '%s' AND dst = '%s'", s.MSISDN, p.Number))
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Rows).To(HaveLen(1))
				Expect(fmt.Sprint(result.Rows[0][0], result.Rows[0][1], result.Rows[0][2])).To(Equal(fmt.Sprint(p.Outgoing, p.Incoming, p.Unknown)), s.MSISDN+" with "+p.Number)
			}
		}
	})

	It("reads and shows every time on the case clock", func() {
		subject, other, _, first := pair()
		result, err := scoped(fmt.Sprintf("SELECT first_seen FROM v_contact_pairs WHERE src = '%s' AND dst = '%s'", subject, other))
		Expect(err).NotTo(HaveOccurred())
		shown := fmt.Sprint(result.Rows[0][0])
		Expect(shown).To(ContainSubstring(first[:10]), "the key's first contact is written on the case clock: "+first)
		_, loc := govSQLTimeZone()
		Expect(loc.String()).To(Equal("Asia/Karachi"))
		hour, err := scoped(fmt.Sprintf("SELECT EXTRACT(HOUR FROM first_seen) AS h FROM v_contact_pairs WHERE src = '%s' AND dst = '%s'", subject, other))
		Expect(err).NotTo(HaveOccurred())
		wantHour, convErr := strconv.Atoi(first[11:13])
		Expect(convErr).NotTo(HaveOccurred())
		Expect(fmt.Sprint(hour.Rows[0][0])).To(Equal(fmt.Sprint(wantHour)), "an hour of day is the case clock's hour: "+first)
	})

	It("shows nothing of another tenant or another collection, and nothing when no scope is set", func() {
		v := validated("SELECT count(*) AS n FROM v_comm_events")
		own, err := askExecute(ctx, pool, truth.Tenant, truth.CaseID, v, limits)
		Expect(err).NotTo(HaveOccurred())
		Expect(fmt.Sprint(own.Rows[0][0])).NotTo(Equal("0"))
		for _, scope := range [][2]string{{"another-tenant", truth.CaseID}, {truth.Tenant, "another-collection"}, {"", ""}, {truth.Tenant, ""}} {
			other, err := askExecute(ctx, pool, scope[0], scope[1], v, limits)
			Expect(err).NotTo(HaveOccurred())
			Expect(fmt.Sprint(other.Rows[0][0])).To(Equal("0"), fmt.Sprintf("scope %q %q", scope[0], scope[1]))
		}
	})

	// The system catalogue is readable by every role (docs/data/analyst-views.md); the validator, not the role, keeps the model out of it.
	It("cannot reach a base table, whatever text reaches it", func() {
		for _, sql := range []string{"SELECT count(*) FROM forensic.comm_event", "SELECT count(*) FROM forensic.records", "SELECT count(*) FROM comm_event", "SELECT count(*) FROM public.nx_throwaway_marker"} {
			_, err := askRun(ctx, pool, truth.Tenant, truth.CaseID, sql, nil, limits, 10, false)
			Expect(err).To(HaveOccurred(), sql)
		}
	})

	It("refuses a query the planner prices above the ceiling, before it runs", func() {
		tight := limits
		tight.MaxPlanCost = 1
		_, err := askExecute(ctx, pool, truth.Tenant, truth.CaseID, validated("SELECT src, count(*) AS n FROM v_comm_events GROUP BY src"), tight)
		run, ok := err.(*govSQLRunError)
		Expect(ok).To(BeTrue())
		Expect(run.Code).To(Equal("TOO_EXPENSIVE"))
		Expect(run.Message).To(ContainSubstring("narrow it"))
	})

	It("caps the rows and says so, never silently", func() {
		small := limits
		small.RowCap = 5
		result, err := askExecute(ctx, pool, truth.Tenant, truth.CaseID, validated("SELECT event_id FROM v_comm_events"), small)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Rows).To(HaveLen(5))
		Expect(result.Truncated).To(BeTrue())
		exact, err := askExecute(ctx, pool, truth.Tenant, truth.CaseID, validated("SELECT event_id FROM v_comm_events LIMIT 5"), small)
		Expect(err).NotTo(HaveOccurred())
		Expect(exact.Rows).To(HaveLen(5))
		Expect(exact.Truncated).To(BeFalse())
	})

	It("stops a query that runs too long and reports it as too broad, not as an error", func() {
		quick := limits
		quick.Timeout = 300 * time.Millisecond
		_, err := askRun(ctx, pool, truth.Tenant, truth.CaseID, "SELECT pg_sleep(3)", nil, quick, 10, false)
		run, ok := err.(*govSQLRunError)
		Expect(ok).To(BeTrue(), fmt.Sprint(err))
		Expect(run.Code).To(Equal("TIMEOUT"))
	})

	It("tells a mistake the model can repair from a failure of the system", func() {
		_, err := askRun(ctx, pool, truth.Tenant, truth.CaseID, "SELECT nope FROM v_comm_events", nil, limits, 10, false)
		run, ok := err.(*govSQLRunError)
		Expect(ok).To(BeTrue())
		Expect(run.Code).To(Equal("QUERY_ERROR"))
		Expect(run.Message).To(ContainSubstring("nope"))
	})

	It("finds where an identifier occurs in the case, and that an unknown one occurs nowhere", func() {
		cat, err := loadAskCatalogue(ctx, pool)
		Expect(err).NotTo(HaveOccurred())
		subject := truth.Subjects[0].MSISDN
		hits, err := askProbe(ctx, pool, truth.Tenant, truth.CaseID, cat, subject, limits)
		Expect(err).NotTo(HaveOccurred())
		sort.Strings(hits)
		Expect(hits).To(ContainElement("v_comm_events.src"))
		none, err := askProbe(ctx, pool, truth.Tenant, truth.CaseID, cat, "923000099999", limits)
		Expect(err).NotTo(HaveOccurred())
		Expect(none).To(BeEmpty())
		other, err := askProbe(ctx, pool, "another-tenant", truth.CaseID, cat, subject, limits)
		Expect(err).NotTo(HaveOccurred())
		Expect(other).To(BeEmpty(), "the probe is scoped like every other statement")
	})

	It("answers a question end to end with a scripted model, and the answer is the planted truth", func() {
		subject, other, events, _ := pair()
		savedModel := askModel
		defer func() { askModel = savedModel }()
		sql := fmt.Sprintf("SELECT events AS n FROM v_contact_pairs WHERE src = '%s' AND dst = '%s'", subject, other)
		askModel = func(context.Context, config, string, []govSQLTurn) (string, string) {
			raw, _ := json.Marshal(map[string]any{"answerable": true, "sql": sql})
			return string(raw), ""
		}
		req := hybridQueryRequest{TenantID: truth.Tenant, CollectionID: truth.CaseID, RequestClass: forensicrequest.GovernedAnalysis,
			Query: fmt.Sprintf("How many events are there between %s and %s?", subject, other)}
		resp, audit := runAskEngineV2(ctx, config{SynthesisModel: "scripted"}, pool, req, time.Now())
		Expect(audit.State).To(Equal("answered"), audit.Reason)
		Expect(resp).NotTo(BeNil())
		Expect(resp.Answer["answer"]).To(Equal(fmt.Sprintf("N: %d.", events)))
		rows := resp.Enterprise["data_grid"].(map[string]any)["rows"].([]map[string]any)
		Expect(fmt.Sprint(rows[0]["n"])).To(Equal(fmt.Sprint(events)))
	})

	It("says that a number the case does not hold has no record, and does not answer zero", func() {
		savedModel := askModel
		defer func() { askModel = savedModel }()
		askModel = func(context.Context, config, string, []govSQLTurn) (string, string) {
			raw, _ := json.Marshal(map[string]any{"answerable": true, "sql": "SELECT count(*) AS n FROM v_comm_events WHERE src = '923000099999' OR dst = '923000099999'"})
			return string(raw), ""
		}
		req := hybridQueryRequest{TenantID: truth.Tenant, CollectionID: truth.CaseID, RequestClass: forensicrequest.GovernedAnalysis, Query: "How many calls does 923000099999 have?"}
		resp, audit := runAskEngineV2(ctx, config{SynthesisModel: "scripted"}, pool, req, time.Now())
		Expect(audit.State).To(Equal("answered"), audit.Reason)
		Expect(resp.Answer["answer"]).To(Equal("No record of this case holds 923000099999 in any number, device, SIM, address or cell column."))
	})

	It("leaves the case unchanged: the engine wrote nothing", func() {
		var writes int
		Expect(pool.QueryRow(ctx, "SELECT coalesce(sum(n_tup_ins + n_tup_upd + n_tup_del), 0)::int FROM pg_stat_user_tables WHERE schemaname = 'analyst'").Scan(&writes)).To(Succeed())
		Expect(writes).To(Equal(0))
	})
})
