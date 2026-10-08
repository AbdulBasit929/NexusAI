package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/mudler/LocalAI/pkg/forensicrequest"
)

// Run 16 (pre-registration, "Run 16 plan"): the classes fresh seed 6 found. W is a filter made of the evidence's own
// name, X a direction taken from a verb, Y a calendar window that nothing held the query to; Q, R, T and Z are
// specified in governed_sql_run15_ginkgo_test.go beside the checks they extend.

var _ = Describe("Governed SQL run 16 checks", func() {
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
	kinds := func(facts govSQLQuestionFacts, sql string) []string {
		validated, rejection := govSQLValidate(sql, views)
		Expect(rejection).To(BeNil(), "validator rejected %q: %v", sql, rejection)
		out := []string{}
		for _, item := range govSQLCheck(facts, validated) {
			out = append(out, item.Kind)
		}
		return out
	}

	Describe("a filter made of the evidence's own name", func() {
		const question = "Which is the most recent request time among the web requests?"

		It("refuses a path like '/web%' for 'web requests', and takes the same question with no such filter", func() {
			facts := factsFor(question, "v_access_log")
			Expect(kinds(facts, "SELECT MAX(event_time) AS latest_request_time FROM v_access_log WHERE path LIKE '/web%'")).To(ConsistOf("LITERAL"))
			Expect(kinds(facts, "SELECT MAX(event_time) AS latest_request_time FROM v_access_log WHERE path = '/web'")).To(ConsistOf("LITERAL"))
			Expect(kinds(facts, "SELECT MAX(event_time) AS latest_request_time FROM v_access_log")).To(BeEmpty())
			validated, _ := govSQLValidate("SELECT MAX(event_time) AS latest_request_time FROM v_access_log WHERE path LIKE '/web%'", views)
			unmet := govSQLCheckLiterals(facts, validated)
			Expect(unmet).To(HaveLen(1))
			Expect(unmet[0].Message).To(ContainSubstring("the evidence's own name is not a value to look for"))
			Expect(unmet[0].Reason).To(ContainSubstring("a word of the evidence's name that the question does not ask for"))
		})

		It("leaves a literal alone when the question names the column, quotes it, or it is not a word of the evidence's name", func() {
			Expect(kinds(factsFor("How many requests hit the /login endpoint?", "v_access_log"), "SELECT COUNT(*) AS n FROM v_access_log WHERE path = '/login'")).To(BeEmpty())
			Expect(kinds(factsFor("How many web requests have a path containing web?", "v_access_log"), "SELECT COUNT(*) AS n FROM v_access_log WHERE path LIKE '%web%'")).To(BeEmpty())
			Expect(kinds(factsFor(`How many web requests mention "login" in the URL?`, "v_access_log"), "SELECT COUNT(*) AS n FROM v_access_log WHERE path LIKE '%login%'")).To(BeEmpty())
			Expect(kinds(factsFor("How many web requests came from Mozilla browsers?", "v_access_log"), "SELECT COUNT(*) AS n FROM v_access_log WHERE user_agent LIKE '%Mozilla%'")).To(BeEmpty())
			// a literal with a digit is not a word of a name
			Expect(kinds(factsFor(question, "v_access_log"), "SELECT MAX(event_time) AS latest_request_time FROM v_access_log WHERE path LIKE '/web2%'")).To(BeEmpty())
		})
	})

	Describe("a direction taken from a verb", func() {
		It("does not read 'made' or 'received' as a direction without an identifier, and does read them beside one", func() {
			plain := factsFor("How many calls were made on weekends?", "v_cdr")
			Expect(plain.WeakValues).NotTo(ContainElement(HaveField("Value", "OUTGOING")))
			Expect(factsFor("How many calls were received on weekends?", "v_cdr").WeakValues).NotTo(ContainElement(HaveField("Value", "INCOMING")))
			beside := factsFor("How many calls were made by 923001110001?", "v_cdr")
			Expect(beside.WeakValues).To(ContainElement(HaveField("Value", "OUTGOING")))
			// an everyday word that is not a verb still asks for its value
			Expect(factsFor("How many outgoing calls are there?", "v_cdr").WeakValues).To(ContainElement(HaveField("Value", "OUTGOING")))
		})

		It("refuses direction = 'OUTGOING' for calls 'made' on weekends, and takes it for calls made by a number", func() {
			facts := factsFor("How many calls were made on weekends?", "v_cdr")
			Expect(kinds(facts, "SELECT COUNT(*) AS number_of_calls FROM v_cdr WHERE direction = 'OUTGOING' AND EXTRACT(ISODOW FROM event_time) IN (6, 7)")).To(ConsistOf("FILTER"))
			Expect(kinds(facts, "SELECT COUNT(*) AS number_of_calls FROM v_cdr WHERE EXTRACT(ISODOW FROM event_time) IN (6, 7)")).To(BeEmpty())
			by := factsFor("How many calls were made by 923001110001?", "v_cdr")
			Expect(kinds(by, "SELECT COUNT(*) AS number_of_calls FROM v_cdr WHERE msisdn = '923001110001' AND direction = 'OUTGOING'")).To(BeEmpty())
			Expect(kinds(by, "SELECT COUNT(*) AS number_of_calls FROM v_cdr WHERE msisdn = '923001110001'")).To(BeEmpty())
		})

		It("does not hint the direction in the question's own message", func() {
			prompt := govSQLUserPrompt("How many calls were made on weekends?", factsFor("How many calls were made on weekends?", "v_cdr"), []*govSQLView{views["v_cdr"]})
			Expect(prompt).NotTo(ContainSubstring("OUTGOING"))
			by := govSQLUserPrompt("How many calls were made by 923001110001?", factsFor("How many calls were made by 923001110001?", "v_cdr"), []*govSQLView{views["v_cdr"]})
			Expect(by).To(ContainSubstring("OUTGOING"))
		})
	})

	Describe("a condition inside an aggregate", func() {
		It("is a time condition: FILTER and CASE carry the month of a comparison written in one row", func() {
			facts := factsFor("How many call records were there in April 2026 versus July 2026?", "v_cdr")
			Expect(kinds(facts, "SELECT COUNT(*) FILTER (WHERE event_time >= '2026-04-01' AND event_time < '2026-05-01') AS april, COUNT(*) FILTER (WHERE event_time >= '2026-07-01' AND event_time < '2026-08-01') AS july FROM v_cdr")).To(BeEmpty())
			Expect(kinds(facts, "SELECT SUM(CASE WHEN event_time >= '2026-04-01' AND event_time < '2026-05-01' THEN 1 ELSE 0 END) AS april, SUM(CASE WHEN event_time >= '2026-07-01' AND event_time < '2026-08-01' THEN 1 ELSE 0 END) AS july FROM v_cdr")).To(BeEmpty())
			Expect(kinds(facts, "SELECT COUNT(*) AS n FROM v_cdr")).To(ContainElement("TIME"))
			// the record's own time still has to be the one used
			Expect(kinds(factsFor("How many calls were there in April 2026?", "v_cdr"), "SELECT COUNT(*) FILTER (WHERE call_start >= '2026-04-01' AND call_start < '2026-05-01') AS n FROM v_cdr")).To(ContainElement("TIME_COLUMN"))
		})
	})

	Describe("a calendar window", func() {
		It("reads one month, one year or one day, and nothing that compares, bounds one side or names two", func() {
			read := func(q string) *govSQLDateRange { return govSQLReadCalendarWindow(q) }
			may := read("How many calls were there in May 2026?")
			Expect(may).NotTo(BeNil())
			Expect(may.Start).To(Equal("2026-05-01"))
			Expect(may.End).To(Equal("2026-05-31"))
			Expect(may.Calendar).To(Equal("May 2026"))
			Expect(read("How many call records are from August 2026?").End).To(Equal("2026-08-31"))
			Expect(read("How many calls were there in February 2024?").End).To(Equal("2024-02-29"))
			Expect(read("How many calls were there in Sept 2026?").Start).To(Equal("2026-09-01"))
			year := read("How many calls were there in 2025?")
			Expect(year).NotTo(BeNil())
			Expect(year.Start).To(Equal("2025-01-01"))
			Expect(year.End).To(Equal("2025-12-31"))
			day := read("How many calls were there on 2026-04-02?")
			Expect(day).NotTo(BeNil())
			Expect(day.Start).To(Equal("2026-04-02"))
			Expect(day.End).To(Equal("2026-04-02"))
			for _, q := range []string{
				"How many calls were there in May 2026 versus June 2026?",
				"How many calls were there in May 2026 and June 2026?",
				"How many calls were there since May 2026?",
				"How many calls were there before May 2026?",
				"How many calls were there between April and June 2026?",
				"How many calls were there in May?",
				"How many calls were there last May 2026?",
				"How many calls were there in May 2026 on 2026-05-03?",
				"How many calls were there?",
			} {
				Expect(read(q)).To(BeNil(), q)
			}
			// two dates given outright stay the run 14 range, without the calendar mark
			explicit := govSQLExtractFacts("How many calls were there between 2026-04-02 and 2026-04-05?", []*govSQLView{views["v_cdr"]}, all)
			Expect(explicit.Range).NotTo(BeNil())
			Expect(explicit.Range.Calendar).To(BeEmpty())
		})

		It("holds the bounds of a month to the month, and leaves other ways of naming it alone", func() {
			facts := factsFor("How many calls were there in May 2026?", "v_cdr")
			for _, where := range []string{
				"event_time >= '2026-05-01' AND event_time < '2026-06-01'",
				"event_time::date BETWEEN '2026-05-01' AND '2026-05-31'",
				"event_time >= '2026-05-01' AND event_time <= '2026-05-31 23:59:59'",
				"event_time >= '2026-05-01' AND event_time::date <= '2026-05-31'",
				"EXTRACT(MONTH FROM event_time) = 5 AND EXTRACT(YEAR FROM event_time) = 2026",
				"date_trunc('month', event_time) = '2026-05-01'",
				"to_char(event_time, 'YYYY-MM') = '2026-05'",
			} {
				Expect(kinds(facts, "SELECT COUNT(*) AS number_of_calls FROM v_cdr WHERE "+where)).To(BeEmpty(), where)
			}
			for _, where := range []string{
				"event_time >= '2026-05-01' AND event_time <= '2026-05-31'", // the 31st is cut at its midnight
				"event_time >= '2026-05-01' AND event_time < '2026-06-02'",  // a day too many
				"event_time::date BETWEEN '2026-05-01' AND '2026-06-01'",    // likewise, by day
				"event_time >= '2026-04-30' AND event_time < '2026-06-01'",  // a day early
				"event_time > '2026-05-01' AND event_time < '2026-06-01'",   // the first day cut
				"event_time BETWEEN '2026-05-01' AND '2026-05-31'",          // a timestamp BETWEEN stops at the midnight of the 31st
				"event_time >= '2026-05-01' AND event_time < '2026-05-31'",  // the 31st left out
				"event_time >= '2026-01-01' AND event_time < '2027-01-01'",  // the year, not the month
				"event_time >= '2026-05-01'",                                // no end
			} {
				Expect(kinds(facts, "SELECT COUNT(*) AS number_of_calls FROM v_cdr WHERE "+where)).To(ConsistOf("RANGE"), where)
			}
			// no time condition at all is the time check's, not the range's
			Expect(kinds(facts, "SELECT COUNT(*) AS number_of_calls FROM v_cdr")).To(ConsistOf("TIME"))
		})

		It("holds a year and a day to theirs", func() {
			year := factsFor("How many calls were there in 2026?", "v_cdr")
			Expect(kinds(year, "SELECT COUNT(*) AS n FROM v_cdr WHERE event_time >= '2026-01-01' AND event_time < '2027-01-01'")).To(BeEmpty())
			Expect(kinds(year, "SELECT COUNT(*) AS n FROM v_cdr WHERE event_time >= '2026-01-01' AND event_time <= '2026-12-31'")).To(ConsistOf("RANGE"))
			day := factsFor("How many calls were there on 2026-04-02?", "v_cdr")
			Expect(kinds(day, "SELECT COUNT(*) AS n FROM v_cdr WHERE event_time >= '2026-04-02' AND event_time < '2026-04-03'")).To(BeEmpty())
			Expect(kinds(day, "SELECT COUNT(*) AS n FROM v_cdr WHERE event_time::date = '2026-04-02'")).To(BeEmpty())
			Expect(kinds(day, "SELECT COUNT(*) AS n FROM v_cdr WHERE event_time >= '2026-04-02' AND event_time <= '2026-04-02'")).To(ConsistOf("RANGE"))
		})

		It("says what it checked, and does not change the question's own message", func() {
			facts := factsFor("How many calls were there in May 2026?", "v_cdr")
			Expect(govSQLConditionsChecked(facts)).To(ContainElement("window: May 2026, the whole of it (2026-05-01 to 2026-05-31)"))
			before := govSQLUserPrompt("How many calls were there in May 2026?", facts, []*govSQLView{views["v_cdr"]})
			facts.Range = nil
			Expect(govSQLUserPrompt("How many calls were there in May 2026?", facts, []*govSQLView{views["v_cdr"]})).To(Equal(before))
		})
	})
})

// With a database: the whole loop, with a scripted model, on the questions of fresh seed 6 and the probes.
var _ = Describe("Governed SQL run 16 (with a database)", func() {
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

	const cdr = "FROM forensic.records WHERE tenant_id='default' AND collection_id='records-demo' AND record_type='cdr'"
	started := govSQLTestCaseClock("raw_payload->>'CALL_START_DT_TM'")
	ask := func(question string) (*hybridQueryResponse, GovSQLAuditV1) {
		req := hybridQueryRequest{TenantID: "default", CollectionID: "records-demo", Query: question, RequestClass: forensicrequest.GovernedAnalysis, SynthesisModel: "scripted"}
		return runGovernedSQLLane(ctx, config{}, db, req, timeNow())
	}
	headline := func(resp *hybridQueryResponse) string { return resp.Enterprise["executive_answer"].(string) }
	lastPrompt := func(call int) string {
		turns := model.calls[call]
		return turns[len(turns)-1].Content
	}

	It("repairs a path invented from 'web requests' and answers the latest request time", func() {
		clock := govSQLTestCaseClock("raw_payload->>'timestamp'")
		want := oracle("SELECT to_char(max(" + clock + "), 'YYYY-MM-DD') FROM forensic.records WHERE tenant_id='default' AND collection_id='records-demo' AND record_type='access_log'")
		model.replies = []string{
			sqlReply("SELECT MAX(event_time) AS latest_request_time FROM v_access_log WHERE path LIKE '/web%'"),
			sqlReply("SELECT MAX(event_time) AS latest_request_time FROM v_access_log"),
		}
		resp, audit := ask("Which is the most recent request time among the web requests?")
		Expect(resp).NotTo(BeNil(), audit.Reason)
		Expect(audit.State).To(Equal("answered"))
		Expect(audit.Attempts).To(Equal(2))
		Expect(audit.Unmet).To(Equal([]string{"LITERAL"}))
		Expect(headline(resp)).To(ContainSubstring(want))
		Expect(headline(resp)).NotTo(ContainSubstring("no value"))
		Expect(lastPrompt(1)).To(ContainSubstring("the evidence's own name is not a value to look for"))
	})

	It("repairs a direction taken from 'made', and the count is the key's", func() {
		want := govSQLFormatNumber(oracle("SELECT count(*) " + cdr + " AND extract(isodow from " + started + ") IN (6, 7)"))
		Expect(want).NotTo(Equal("0"))
		model.replies = []string{
			sqlReply("SELECT COUNT(*) AS number_of_calls FROM v_cdr WHERE direction = 'OUTGOING' AND EXTRACT(ISODOW FROM event_time) IN (6, 7)"),
			sqlReply("SELECT COUNT(*) AS number_of_calls FROM v_cdr WHERE EXTRACT(ISODOW FROM event_time) IN (6, 7)"),
		}
		resp, audit := ask("How many calls were made on weekends?")
		Expect(resp).NotTo(BeNil(), audit.Reason)
		Expect(audit.State).To(Equal("answered"))
		Expect(audit.Attempts).To(Equal(2))
		Expect(headline(resp)).To(Equal("Number of calls: " + want + "."))
		Expect(model.calls[0][len(model.calls[0])-1].Content).NotTo(ContainSubstring("OUTGOING"))
		Expect(lastPrompt(1)).To(ContainSubstring("does not ask for direction = 'OUTGOING'"))
	})

	It("repairs a month window one day too wide", func() {
		want := govSQLFormatNumber(oracle("SELECT count(*) " + cdr + " AND to_char(" + started + ", 'YYYY-MM') = '2026-04'"))
		model.replies = []string{
			sqlReply("SELECT COUNT(*) AS number_of_calls FROM v_cdr WHERE event_time::date BETWEEN '2026-04-01' AND '2026-05-01'"),
			sqlReply("SELECT COUNT(*) AS number_of_calls FROM v_cdr WHERE event_time >= '2026-04-01' AND event_time < '2026-05-01'"),
		}
		resp, audit := ask("How many calls were there in April 2026?")
		Expect(resp).NotTo(BeNil(), audit.Reason)
		Expect(audit.State).To(Equal("answered"))
		Expect(audit.Attempts).To(Equal(2))
		Expect(audit.Unmet).To(Equal([]string{"RANGE"}))
		Expect(headline(resp)).To(Equal("Number of calls: " + want + "."))
		Expect(resp.Enterprise["limitations"]).To(ContainElement(ContainSubstring("window: April 2026, the whole of it (2026-04-01 to 2026-04-30)")))
	})

	It("shows an identifier exactly as stored when the query renames its column", func() {
		top := oracle("SELECT v FROM (SELECT raw_payload->>'MSISDN' v, count(*) c " + cdr + " GROUP BY 1 ORDER BY 2 DESC, 1 LIMIT 1) t")
		model.replies = []string{sqlReply("SELECT msisdn AS most_common_caller FROM v_cdr GROUP BY msisdn ORDER BY COUNT(*) DESC, msisdn LIMIT 1")}
		resp, audit := ask("What is the most common caller in the CDR records?")
		Expect(audit.State).To(Equal("answered"), audit.Reason)
		Expect(headline(resp)).To(Equal("Most common caller: " + top + "."))
		Expect(headline(resp)).NotTo(MatchRegexp(`\d,\d{3}`))
	})

	It("answers a comparison in one row whose columns each count one side", func() {
		april := govSQLFormatNumber(oracle("SELECT count(*) " + cdr + " AND to_char(" + started + ", 'YYYY-MM') = '2026-04'"))
		july := govSQLFormatNumber(oracle("SELECT count(*) " + cdr + " AND to_char(" + started + ", 'YYYY-MM') = '2026-07'"))
		model.replies = []string{sqlReply("SELECT COUNT(*) FILTER (WHERE event_time >= '2026-04-01' AND event_time < '2026-05-01') AS april_records, COUNT(*) FILTER (WHERE event_time >= '2026-07-01' AND event_time < '2026-08-01') AS july_records FROM v_cdr")}
		resp, audit := ask("How many call records were there in April 2026 versus July 2026?")
		Expect(resp).NotTo(BeNil(), audit.Reason)
		Expect(audit.State).To(Equal("answered"), func() string {
			if resp != nil && resp.Answer != nil {
				return fmt.Sprint(resp.Answer["clarification"])
			}
			return audit.Reason
		}())
		Expect(audit.Attempts).To(Equal(1))
		Expect(headline(resp)).To(Equal("April records: " + april + "; July records: " + july + "."))
	})

	It("offers the access log for 'log entries' and answers", func() {
		want := govSQLFormatNumber(oracle("SELECT count(*) FROM forensic.records WHERE tenant_id='default' AND collection_id='records-demo' AND record_type='access_log'"))
		model.replies = []string{sqlReply("SELECT COUNT(*) AS number_of_log_entries FROM v_access_log")}
		resp, audit := ask("How many log entries are there?")
		Expect(resp).NotTo(BeNil(), audit.Reason)
		Expect(audit.State).To(Equal("answered"))
		Expect(headline(resp)).To(Equal("Number of log entries: " + want + "."))
		Expect(strings.Join(audit.Views, ",")).To(ContainSubstring("v_access_log"))
	})
})
