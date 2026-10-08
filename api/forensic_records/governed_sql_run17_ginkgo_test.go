package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/mudler/LocalAI/pkg/forensicrequest"
)

// Run 17: a comparison of calendar units. "May 2026 versus June 2026" asks for a row per month, and the months are not a
// column the hint "GROUP BY the column that holds them" could name: the question was declined four times in four
// (run 15 and run 16, probes SHAPE-compare_months-01 and SHAPE2-versus_months-01).

var _ = Describe("Governed SQL run 17 checks", func() {
	var (
		views map[string]*govSQLView
		all   []*govSQLView
	)
	BeforeEach(func() {
		views = govSQLTestViews()
		all = nil
		for _, v := range views {
			all = append(all, v)
		}
	})
	factsFor := func(question string, names ...string) govSQLQuestionFacts {
		picked := []*govSQLView{}
		for _, name := range names {
			picked = append(picked, views[name])
		}
		return govSQLExtractFacts(question, picked, all)
	}

	Describe("a comparison of months, years or days", func() {
		It("reads the units that are set side by side", func() {
			months := govSQLReadCompareUnits("How many call records were there in May 2026 versus June 2026?")
			Expect(months).NotTo(BeNil())
			Expect(months.Unit).To(Equal("month"))
			Expect(months.Keys).To(Equal([]string{"2026-05", "2026-06"}))
			Expect(months.Format).To(Equal("YYYY-MM"))
			Expect(govSQLReadCompareUnits("How many calls in April 2026 compared with July 2026 and August 2026?").Keys).To(Equal([]string{"2026-04", "2026-07", "2026-08"}))
			years := govSQLReadCompareUnits("How many calls were there in 2025 versus 2026?")
			Expect(years).NotTo(BeNil())
			Expect(years.Unit).To(Equal("year"))
			Expect(years.Keys).To(Equal([]string{"2025", "2026"}))
			days := govSQLReadCompareUnits("How many calls on 2026-04-02 vs 2026-04-03?")
			Expect(days).NotTo(BeNil())
			Expect(days.Unit).To(Equal("day"))
			Expect(days.Format).To(Equal("YYYY-MM-DD"))
			for _, q := range []string{
				"How many incoming versus outgoing calls are there?",
				"How many calls were there in May 2026 and June 2026?",
				"How many calls were there in May 2026?",
				"How many calls compared with the year 2026?",
			} {
				Expect(govSQLReadCompareUnits(q)).To(BeNil(), q)
			}
		})

		It("says what the rows are, in the question's own message and in the retry", func() {
			question := "How many call records were there in May 2026 versus June 2026?"
			facts := factsFor(question, "v_cdr")
			Expect(facts.CompareUnits).NotTo(BeNil())
			prompt := govSQLUserPrompt(question, facts, []*govSQLView{views["v_cdr"]})
			Expect(prompt).To(ContainSubstring("The question compares months (2026-05, 2026-06). Return one row per month: SELECT to_char(column, 'YYYY-MM') AS month"))
			plain := govSQLUserPrompt("How many incoming versus outgoing calls are there?", factsFor("How many incoming versus outgoing calls are there?", "v_cdr"), []*govSQLView{views["v_cdr"]})
			Expect(plain).NotTo(ContainSubstring("to_char"))
			validated, rejection := govSQLValidate("SELECT COUNT(*) AS number_of_call_records FROM v_cdr WHERE event_time >= '2026-05-01' AND event_time < '2026-07-01'", views)
			Expect(rejection).To(BeNil())
			unmet := govSQLCheckCompare(facts, validated, &govSQLResult{Columns: []string{"number_of_call_records"}, Rows: [][]any{{int64(3457)}}})
			Expect(unmet).NotTo(BeNil())
			Expect(unmet.Message).To(ContainSubstring("The question compares months (2026-05, 2026-06)"))
			Expect(unmet.Message).To(ContainSubstring("event_time, the time of the record"))
			// two rows are the answer, whatever the units
			Expect(govSQLCheckCompare(facts, validated, &govSQLResult{Columns: []string{"month", "n"}, Rows: [][]any{{"2026-05", int64(994)}, {"2026-06", int64(2463)}}})).To(BeNil())
		})
	})
})

// With a database: one total is refused once, with the way to write it, and the rows are the answer.
var _ = Describe("Governed SQL run 17 (with a database)", func() {
	var (
		ctx    context.Context
		db     *pgxpool.Pool
		model  *scriptedModel
		oracle func(sql string) string
	)
	BeforeEach(func() {
		dsn := os.Getenv("GOVSQL_TEST_DB") //nolint:forbidigo // tests gate live-database specs on an environment variable
		if dsn == "" {
			Skip("needs a live database; set GOVSQL_TEST_DB")
		}
		ctx = context.Background()
		var err error
		db, err = pgxpool.New(ctx, dsn)
		Expect(err).NotTo(HaveOccurred())
		model = &scriptedModel{}
		previous := govSQLModel
		govSQLModel = model.ask
		DeferCleanup(func() {
			govSQLModel = previous
			db.Close()
		})
		oracle = func(sql string) string {
			var out string
			Expect(db.QueryRow(ctx, "SELECT ("+sql+")::text").Scan(&out)).To(Succeed())
			return out
		}
	})

	It("repairs one total of two months into a row per month", func() {
		const cdr = "FROM forensic.records WHERE tenant_id='default' AND collection_id='records-demo' AND record_type='cdr'"
		started := govSQLTestCaseClock("raw_payload->>'CALL_START_DT_TM'")
		april := govSQLFormatNumber(oracle("SELECT count(*) " + cdr + " AND to_char(" + started + ", 'YYYY-MM') = '2026-04'"))
		july := govSQLFormatNumber(oracle("SELECT count(*) " + cdr + " AND to_char(" + started + ", 'YYYY-MM') = '2026-07'"))
		model.replies = []string{
			sqlReply("SELECT COUNT(*) AS number_of_call_records FROM v_cdr WHERE event_time >= '2026-04-01' AND event_time < '2026-08-01'"),
			sqlReply("SELECT to_char(event_time, 'YYYY-MM') AS month, COUNT(*) AS number_of_call_records FROM v_cdr WHERE (event_time >= '2026-04-01' AND event_time < '2026-05-01') OR (event_time >= '2026-07-01' AND event_time < '2026-08-01') GROUP BY 1 ORDER BY 1"),
		}
		req := hybridQueryRequest{TenantID: "default", CollectionID: "records-demo", Query: "How many call records were there in April 2026 versus July 2026?", RequestClass: forensicrequest.GovernedAnalysis, SynthesisModel: "scripted"}
		resp, audit := runGovernedSQLLane(ctx, config{}, db, req, timeNow())
		Expect(resp).NotTo(BeNil(), audit.Reason)
		Expect(audit.State).To(Equal("answered"), fmt.Sprint(audit.Reason))
		Expect(audit.Attempts).To(Equal(2))
		text := resp.Enterprise["executive_answer"].(string)
		Expect(text).To(ContainSubstring("2026-04: " + april))
		Expect(text).To(ContainSubstring("2026-07: " + july))
		turns := model.calls[1]
		Expect(turns[len(turns)-1].Content).To(ContainSubstring("The question compares months (2026-04, 2026-07)"))
	})
})
