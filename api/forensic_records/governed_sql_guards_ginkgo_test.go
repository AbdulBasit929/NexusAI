package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudler/LocalAI/pkg/forensicrequest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Run 14: the guards that close the classes Phase 1 found (governed_sql_guards.go). Each class is a question
// the lane answered, or would answer, with a confident number to a different question.
var _ = Describe("Governed SQL guards (no database)", func() {
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
	factsFor := func(question, view string) govSQLQuestionFacts {
		return govSQLExtractFacts(question, []*govSQLView{views[view]}, all)
	}
	validate := func(sql string) *govSQLValidated {
		validated, rejection := govSQLValidate(sql, views)
		Expect(rejection).To(BeNil(), "validator rejected %q: %v", sql, rejection)
		return validated
	}
	rangeKinds := func(question, view, sql string) []string {
		kinds := []string{}
		for _, item := range govSQLCheckRange(factsFor(question, view), validate(sql)) {
			kinds = append(kinds, item.Kind)
		}
		return kinds
	}

	Describe("a range of two dates includes both days in full", func() {
		const question = "How many CDR records are there between 2026-04-26 and 2026-05-30?"

		It("reads a range only when it is written as two dates and says both are in", func() {
			for text, want := range map[string]*govSQLDateRange{
				"How many calls between 2026-04-26 and 2026-05-30?":     {Start: "2026-04-26", End: "2026-05-30"},
				"How many calls from 2026-04-26 to 2026-05-30?":         {Start: "2026-04-26", End: "2026-05-30"},
				"How many calls from 2026-04-26 through 2026-05-30?":    {Start: "2026-04-26", End: "2026-05-30"},
				"How many calls between 2026-05-30 and 2026-05-30?":     {Start: "2026-05-30", End: "2026-05-30"},
				"How many calls were there since 2026-04-26?":           nil,
				"How many calls before 2026-05-30?":                     nil,
				"How many calls from 2026-04-26 until 2026-05-30?":      nil,
				"How many calls between 2026-05-30 and 2026-04-26?":     nil,
				"How many calls between 2026-13-45 and 2026-14-45?":     nil,
				"How many calls between 2025 and 2026?":                 nil,
				"How many calls between 10 and 20 minutes?":             nil,
				"How many calls were made on 2026-05-30?":               nil,
				"How many calls between 04/05/2026 and 20/05/2026?":     nil,
				"How many calls between 2026-04-26 and the 30th of May": nil,
			} {
				got := govSQLReadDateRange(text)
				if want == nil {
					Expect(got).To(BeNil(), text)
				} else {
					Expect(got).To(Equal(want), text)
				}
			}
		})

		It("accepts every way of writing the whole range", func() {
			for _, sql := range []string{
				"SELECT COUNT(*) AS n FROM v_cdr WHERE event_time >= '2026-04-26' AND event_time < '2026-05-31'",
				"SELECT COUNT(*) AS n FROM v_cdr WHERE event_time::date BETWEEN '2026-04-26' AND '2026-05-30'",
				"SELECT COUNT(*) AS n FROM v_cdr WHERE event_time >= '2026-04-26' AND event_time <= '2026-05-30 23:59:59'",
				"SELECT COUNT(*) AS n FROM v_cdr WHERE event_time >= DATE '2026-04-26' AND event_time < DATE '2026-05-30' + INTERVAL '1 day'",
				"SELECT COUNT(*) AS n FROM v_cdr WHERE event_time >= '2026-04-26' AND event_time < '2026-05-30'::date + 1",
				"SELECT COUNT(*) AS n FROM v_cdr WHERE date(event_time) >= '2026-04-26' AND date(event_time) <= '2026-05-30'",
				"SELECT COUNT(*) AS n FROM v_cdr WHERE date_trunc('day', event_time) BETWEEN '2026-04-26' AND '2026-05-30'",
				"SELECT COUNT(*) AS n FROM v_cdr WHERE '2026-05-31' > event_time AND '2026-04-26' <= event_time",
				"SELECT COUNT(*) AS n FROM v_cdr WHERE event_time::date > '2026-04-25' AND event_time <= '2026-05-31'",
				"SELECT COUNT(*) AS n FROM v_cdr WHERE event_time >= '2026-04-26T00:00:00+05:00' AND event_time < '2026-05-31T00:00:00+05:00'",
			} {
				Expect(rangeKinds(question, "v_cdr", sql)).To(BeEmpty(), sql)
			}
		})

		It("refuses a range that runs past the last day or starts before the first, which counts days the question did not ask for", func() {
			for _, sql := range []string{
				"SELECT COUNT(*) AS n FROM v_cdr WHERE event_time >= '2026-04-01' AND event_time < '2026-06-01'",
				"SELECT COUNT(*) AS n FROM v_cdr WHERE event_time >= '2026-04-26' AND event_time < '2026-06-01'",
				"SELECT COUNT(*) AS n FROM v_cdr WHERE event_time > '2026-04-25' AND event_time < '2026-05-31'",
				"SELECT COUNT(*) AS n FROM v_cdr WHERE event_time >= '2026-04-26' AND event_time::date <= '2026-05-31'",
			} {
				Expect(rangeKinds(question, "v_cdr", sql)).To(ConsistOf("RANGE"), sql)
			}
		})

		It("refuses a range that stops at the midnight that begins the last day: the 914 that should have been 1,013", func() {
			for _, sql := range []string{
				"SELECT COUNT(*) AS n FROM v_cdr WHERE event_time >= '2026-04-26' AND event_time < '2026-05-30'",
				"SELECT COUNT(*) AS n FROM v_cdr WHERE event_time >= '2026-04-26' AND event_time <= '2026-05-30'",
				"SELECT COUNT(*) AS n FROM v_cdr WHERE event_time BETWEEN '2026-04-26' AND '2026-05-30'",
				"SELECT COUNT(*) AS n FROM v_cdr WHERE event_time >= '2026-04-26' AND event_time < '2026-05-30 12:00:00'",
				"SELECT COUNT(*) AS n FROM v_cdr WHERE event_time::date >= '2026-04-26' AND event_time::date < '2026-05-30'",
				"SELECT COUNT(*) AS n FROM v_cdr WHERE event_time > '2026-04-25' AND event_time < '2026-05-30'",
			} {
				Expect(rangeKinds(question, "v_cdr", sql)).To(ConsistOf("RANGE"), sql)
			}
		})

		It("refuses a range with the first day left out, with only one end, or with a bound it cannot read", func() {
			for _, sql := range []string{
				"SELECT COUNT(*) AS n FROM v_cdr WHERE event_time::date > '2026-04-26' AND event_time::date <= '2026-05-30'",
				"SELECT COUNT(*) AS n FROM v_cdr WHERE event_time >= '2026-04-27' AND event_time < '2026-05-31'",
				"SELECT COUNT(*) AS n FROM v_cdr WHERE event_time >= '2026-04-26'",
				"SELECT COUNT(*) AS n FROM v_cdr WHERE event_time < '2026-05-31'",
				"SELECT COUNT(*) AS n FROM v_cdr WHERE event_time >= '2026-04-26' AND event_time < now()",
				"SELECT COUNT(*) AS n FROM v_cdr",
			} {
				Expect(rangeKinds(question, "v_cdr", sql)).To(ConsistOf("RANGE"), sql)
			}
		})

		It("tells the model the day after and the form to use, and tells the analyst which days were not applied", func() {
			items := govSQLCheckRange(factsFor(question, "v_cdr"), validate("SELECT COUNT(*) AS n FROM v_cdr WHERE event_time >= '2026-04-26' AND event_time < '2026-05-30'"))
			Expect(items).To(HaveLen(1))
			Expect(items[0].Message).To(ContainSubstring("event_time < '2026-05-31'"))
			Expect(items[0].Message).To(ContainSubstring("event_time::date BETWEEN '2026-04-26' AND '2026-05-30'"))
			Expect(items[0].Reason).To(ContainSubstring("2026-04-26"))
			Expect(items[0].Reason).To(ContainSubstring("2026-05-30"))
		})

		It("holds a date column to the same range, by day", func() {
			const q = "How many subscribers have a activation date between 2025-03-09 and 2025-12-13?"
			for sql, want := range map[string][]string{
				"SELECT COUNT(*) AS n FROM v_subscriber WHERE activation_date >= '2025-03-09' AND activation_date < '2025-12-14'":  {},
				"SELECT COUNT(*) AS n FROM v_subscriber WHERE activation_date BETWEEN '2025-03-09' AND '2025-12-13'":               {},
				"SELECT COUNT(*) AS n FROM v_subscriber WHERE activation_date >= '2025-03-09' AND activation_date <= '2025-12-13'": {},
				"SELECT COUNT(*) AS n FROM v_subscriber WHERE activation_date >= '2025-03-09' AND activation_date < '2025-12-13'":  {"RANGE"},
				"SELECT COUNT(*) AS n FROM v_subscriber WHERE activation_date > '2025-03-09' AND activation_date <= '2025-12-13'":  {"RANGE"},
				"SELECT COUNT(*) AS n FROM v_subscriber WHERE activation_date BETWEEN '2025-03-10' AND '2025-12-13'":               {"RANGE"},
				"SELECT COUNT(*) AS n FROM v_subscriber WHERE activation_date >= '2025-03-09' AND activation_date <= '2025-12-12'": {"RANGE"},
				"SELECT COUNT(*) AS n FROM v_subscriber WHERE activation_date >= '2025-03-09' AND activation_date <= '2025-12-14'": {"RANGE"},
			} {
				kinds := rangeKinds(q, "v_subscriber", sql)
				if len(want) == 0 {
					Expect(kinds).To(BeEmpty(), sql)
				} else {
					Expect(kinds).To(ConsistOf(want), sql)
				}
			}
		})

		It("is part of the full check, next to the time condition, and not asked of a question without a range", func() {
			kinds := func(q, sql string) []string {
				out := []string{}
				for _, item := range govSQLCheck(factsFor(q, "v_cdr"), validate(sql)) {
					out = append(out, item.Kind)
				}
				return out
			}
			Expect(kinds(question, "SELECT COUNT(*) AS n FROM v_cdr WHERE event_time >= '2026-04-26' AND event_time < '2026-05-30'")).To(ConsistOf("RANGE"))
			Expect(kinds(question, "SELECT COUNT(*) AS n FROM v_cdr")).To(ConsistOf("TIME", "RANGE"))
			Expect(kinds("How many CDR records were there before 2026-05-30?", "SELECT COUNT(*) AS n FROM v_cdr WHERE event_time < '2026-05-30'")).To(BeEmpty())
			Expect(kinds("How many CDR records were there on 2026-05-30?", "SELECT COUNT(*) AS n FROM v_cdr WHERE event_time >= '2026-05-30' AND event_time < '2026-05-31'")).To(BeEmpty())
		})
	})

	Describe("a comparison is one row per value", func() {
		It("reads versus, vs and compared with, and no longer reads them as one value", func() {
			for _, q := range []string{
				"How many incoming versus outgoing calls are there?",
				"How many incoming vs outgoing calls are there?",
				"How many calls were incoming compared with outgoing?",
				"Compare incoming and outgoing calls",
			} {
				facts := factsFor(q, "v_cdr")
				Expect(facts.WantsCompare).To(BeTrue(), q)
				Expect(facts.WantsSingle).To(BeFalse(), q)
			}
			for _, q := range []string{"How many calls are there?", "How many calls were made from the cell site?", "What is the average call duration?", "How many calls lasted longer than 600 seconds?"} {
				facts := factsFor(q, "v_cdr")
				Expect(facts.WantsCompare).To(BeFalse(), q)
			}
			Expect(factsFor("How many calls are there?", "v_cdr").WantsSingle).To(BeTrue())
		})

		It("asks the model for one row per value, and only for a comparison", func() {
			v := []*govSQLView{views["v_cdr"]}
			q := "How many incoming versus outgoing calls are there?"
			user := govSQLUserPrompt(q, factsFor(q, "v_cdr"), v)
			Expect(user).To(ContainSubstring("compares values"))
			Expect(user).NotTo(ContainSubstring("return one row, with no GROUP BY"))
			plain := govSQLUserPrompt("How many calls are there?", factsFor("How many calls are there?", "v_cdr"), v)
			Expect(plain).To(ContainSubstring("return one row, with no GROUP BY"))
			Expect(plain).NotTo(ContainSubstring("compares values"))
		})

		It("refuses one total for a comparison and accepts one row per value", func() {
			facts := factsFor("How many incoming versus outgoing calls are there?", "v_cdr")
			Expect(govSQLCheckCompare(facts, nil, &govSQLResult{Columns: []string{"n"}, Rows: [][]any{{int64(5500)}}})).NotTo(BeNil())
			Expect(govSQLCheckCompare(facts, nil, &govSQLResult{Columns: []string{"direction", "n"}, Rows: [][]any{{"INCOMING", int64(2906)}, {"OUTGOING", int64(2592)}}})).To(BeNil())
			Expect(govSQLCheckCompare(factsFor("How many calls are there?", "v_cdr"), nil, &govSQLResult{Columns: []string{"n"}, Rows: [][]any{{int64(5500)}}})).To(BeNil())
		})
	})

	Describe("a ranking of contacts never ranks the number the question names", func() {
		const question = "Who did 923001110001 contact most frequently?"
		const grouped = "SELECT dialed_number, COUNT(*) AS contacts FROM v_cdr WHERE msisdn = '923001110001' OR dialed_number = '923001110001' GROUP BY dialed_number ORDER BY contacts DESC LIMIT 5"

		It("finds the subject among the groups, in whatever form the number is written", func() {
			facts := factsFor(question, "v_cdr")
			Expect(facts.Identifiers).To(HaveLen(1))
			Expect(facts.WantsRanking).To(BeTrue())
			validated := validate(grouped)
			rows := func(first string) *govSQLResult {
				return &govSQLResult{Columns: []string{"dialed_number", "contacts"}, Rows: [][]any{{first, int64(425)}, {"923009998887", int64(68)}}}
			}
			for _, written := range []string{"923001110001", "03001110001", "+92 300 1110001", "92-300-1110001"} {
				Expect(govSQLCheckSubject(facts, validated, rows(written))).NotTo(BeNil(), written)
			}
			Expect(govSQLCheckSubject(facts, validated, rows("923009998887"))).To(BeNil())
			Expect(govSQLCheckSubject(facts, validated, rows("923001110002"))).To(BeNil())
		})

		It("says what a contact is, and what was wrong, in the words each reader needs", func() {
			facts := factsFor(question, "v_cdr")
			item := govSQLCheckSubject(facts, validate(grouped), &govSQLResult{Columns: []string{"d", "c"}, Rows: [][]any{{"923001110001", int64(425)}}})
			Expect(item).NotTo(BeNil())
			Expect(item.Message).To(ContainSubstring("OTHER party"))
			Expect(item.Reason).To(ContainSubstring("923001110001"))
		})

		It("asks only a ranking over a group in a question that names one number", func() {
			facts := factsFor(question, "v_cdr")
			subject := &govSQLResult{Columns: []string{"d", "c"}, Rows: [][]any{{"923001110001", int64(425)}}}
			// no GROUP BY: a listing of the number's own records is not a ranking
			Expect(govSQLCheckSubject(facts, validate("SELECT dialed_number FROM v_cdr WHERE msisdn = '923001110001'"), subject)).To(BeNil())
			// no ranking in the question
			plain := factsFor("List the numbers 923001110001 dialed", "v_cdr")
			Expect(plain.WantsRanking).To(BeFalse())
			Expect(govSQLCheckSubject(plain, validate(grouped), subject)).To(BeNil())
			// two numbers named: the winner may be one of them
			two := factsFor("Which of 923001110001 and 923001110002 made the most calls?", "v_cdr")
			Expect(two.Identifiers).To(HaveLen(2))
			Expect(govSQLCheckSubject(two, validate(grouped), subject)).To(BeNil())
		})
	})

	Describe("a search of text evidence is the retrieval path's", func() {
		It("recognises a search of what a recording, an image or a document says", func() {
			for _, q := range []string{
				"Search the audio transcripts for Japanese cuisine",
				"Which recording mentions coconut sugar and at what time?",
				"Find OCR text mentioning Investigation Workspace",
				"Is the number 03001234567 mentioned in any audio?",
				"Which document mentions contact number 03001234567?",
				"What does the audio transcript say about the delivery?",
				"Search documents for the phrase \"quantum encryption key\"",
				"Look for the keyword invoice in the scanned documents",
			} {
				Expect(govSQLSearchesText(q)).To(BeTrue(), q)
			}
		})

		It("leaves a question about media metadata, and every structured question, to the lane", func() {
			for _, q := range []string{
				"How many audio segments are there?",
				"What is the smallest face crop dimension in pixels?",
				"Which model produced the face vectors?",
				"What is the average OCR confidence of the plate reads?",
				"How many plate groups used persistent object tracking?",
				"How many faces were detected in the evidence?",
				"How many calls were made at night?",
				"How many CDR records contain 923001110001?",
				"Which cell site said the most calls?",
				"How many sessions mention example.test?",
			} {
				Expect(govSQLSearchesText(q)).To(BeFalse(), q)
			}
		})
	})

	Describe("a comparison with letter case ignored is part of the check", func() {
		It("marks a column compared through lower() or upper() as already folded", func() {
			byOp := func(sql string) []govSQLComparison {
				return govSQLComparisons(validate(sql).Tree)
			}
			for sql, folded := range map[string]bool{
				"SELECT COUNT(*) FROM v_cdr WHERE call_type = 'VOICE'":               false,
				"SELECT COUNT(*) FROM v_cdr WHERE LOWER(call_type) = 'voice'":        true,
				"SELECT COUNT(*) FROM v_cdr WHERE UPPER(call_type) = 'VOICE'":        true,
				"SELECT COUNT(*) FROM v_cdr WHERE LOWER(call_type) = LOWER('VOICE')": true,
				"SELECT COUNT(*) FROM v_cdr WHERE 'VOICE' = call_type":               false,
				"SELECT COUNT(*) FROM v_cdr WHERE call_type::text = 'VOICE'":         false,
			} {
				found := false
				for _, cmp := range byOp(sql) {
					if cmp.Column == "call_type" && cmp.Op == "=" {
						found = true
						Expect(cmp.Folded).To(Equal(folded), sql)
					}
				}
				Expect(found).To(BeTrue(), sql)
			}
		})

		It("says to compare ignoring case, to the model, and why nothing was counted, to the analyst", func() {
			item := govSQLCaseMismatch{Column: "status", Literal: "ACTIVE"}.obligation()
			Expect(item.Kind).To(Equal("CASE"))
			Expect(item.Message).To(ContainSubstring("LOWER(status) = LOWER('ACTIVE')"))
			Expect(item.Reason).To(ContainSubstring("different letter case"))
		})
	})

	Describe("what an answer says about a number and a count of distinct values", func() {
		It("never rounds a value that is not whole into a whole number", func() {
			for in, want := range map[string]string{
				"0.9999594807624816": "0.99996",
				"0.00004":            "0.00004",
				"-0.00004":           "-0.00004",
				"1.00001":            "1.00001",
				"0.99999":            "0.99999",
				"0.123456789":        "0.1235",
				"0.88921234":         "0.8892",
				"3905649.900000":     "3,905,649.9",
				"2.0000":             "2",
				"5005":               "5,005",
				"1234.5678901":       "1,234.5679",
			} {
				Expect(govSQLFormatNumber(in)).To(Equal(want), in)
			}
			Expect(govSQLFormatValue(0.9999594807624816)).To(Equal("0.99996"))
			Expect(govSQLWholeText("1.0000")).To(BeTrue())
			Expect(govSQLWholeText("1.0001")).To(BeFalse())
			Expect(govSQLWholeText("12")).To(BeTrue())
		})

		It("says distinct when the count is of distinct values", func() {
			Expect(govSQLDistinctLabel("Number of server errors")).To(Equal("Number of distinct server errors"))
			Expect(govSQLDistinctLabel("Count of numbers")).To(Equal("Count of distinct numbers"))
			Expect(govSQLDistinctLabel("Servers")).To(Equal("Servers (distinct)"))
			Expect(govSQLDistinctLabel("Number of distinct servers")).To(Equal("Number of distinct servers"))
			Expect(govSQLDistinctLabel("Number of unique callers")).To(Equal("Number of unique callers"))

			targets := func(sql string) []bool { return govSQLDistinctTargets(validate(sql).Tree) }
			Expect(targets("SELECT COUNT(DISTINCT status) AS n FROM v_access_log")).To(Equal([]bool{true}))
			Expect(targets("SELECT COUNT(status) AS n FROM v_access_log")).To(Equal([]bool{false}))
			Expect(targets("SELECT COUNT(*) AS n FROM v_access_log")).To(Equal([]bool{false}))
			Expect(targets("SELECT msisdn, COUNT(DISTINCT dialed_number) AS d FROM v_cdr GROUP BY msisdn")).To(Equal([]bool{false, true}))
			Expect(targets("SELECT COUNT(DISTINCT status)::int AS n FROM v_access_log")).To(Equal([]bool{true}))
		})

		It("labels the headline and the metrics with it, and leaves every other result as it was", func() {
			view := &govSQLView{Display: "System access logs"}
			validated := validate("SELECT COUNT(DISTINCT status) AS number_of_server_errors FROM v_access_log")
			result := &govSQLResult{Columns: []string{"number_of_server_errors"}, Rows: [][]any{{int64(6)}}}
			govSQLLabelResult(validated, result)
			Expect(govSQLHeadline(view, result, govSQLQuestionFacts{}, "", nil)).To(Equal("Number of distinct server errors: 6."))
			Expect(govSQLMetrics(view, result)[0]["label"]).To(Equal("Number of distinct server errors"))

			plain := &govSQLResult{Columns: []string{"number_of_server_errors"}, Rows: [][]any{{int64(46)}}}
			govSQLLabelResult(validate("SELECT COUNT(*) AS number_of_server_errors FROM v_access_log WHERE status = '500'"), plain)
			Expect(govSQLHeadline(view, plain, govSQLQuestionFacts{}, "", nil)).To(Equal("Number of server errors: 46."))
			Expect(plain.Labels).To(BeNil())
		})
	})
})

// With a database: the whole loop, with a scripted model, against rows that reproduce the Phase 1 failures
// (subscribers stored as 'active' where the layer says ACTIVE; a number that is its own dialed number on 425
// of its 872 records).
var _ = Describe("Governed SQL guards (with a database)", func() {
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

	const records = "FROM forensic.records WHERE tenant_id='default' AND collection_id='records-demo' AND record_type="
	ask := func(question string) (*hybridQueryResponse, GovSQLAuditV1) {
		req := hybridQueryRequest{TenantID: "default", CollectionID: "records-demo", Query: question, RequestClass: forensicrequest.GovernedAnalysis, SynthesisModel: "scripted"}
		return runGovernedSQLLane(ctx, config{}, db, req, timeNow())
	}
	headline := func(resp *hybridQueryResponse) string { return resp.Enterprise["executive_answer"].(string) }
	lastPrompt := func(call int) string {
		turns := model.calls[call]
		return turns[len(turns)-1].Content
	}

	Describe("a range of two dates", func() {
		const question = "How many CDR records are there between 2026-04-02 and 2026-04-03?"
		var want string
		BeforeEach(func() {
			want = govSQLFormatNumber(oracle("SELECT count(*) " + records + "'cdr' AND (" + govSQLTestCaseClock("raw_payload->>'CALL_START_DT_TM'") + ")::date BETWEEN '2026-04-02' AND '2026-04-03'"))
			Expect(want).NotTo(Equal("0"))
		})

		It("repairs a range that stops before the last day, once, naming the day after, and the count is the key's", func() {
			model.replies = []string{
				sqlReply("SELECT COUNT(*) AS number_of_call_records FROM v_cdr WHERE event_time >= '2026-04-02' AND event_time < '2026-04-03'"),
				sqlReply("SELECT COUNT(*) AS number_of_call_records FROM v_cdr WHERE event_time >= '2026-04-02' AND event_time < '2026-04-04'"),
			}
			resp, audit := ask(question)
			Expect(resp).NotTo(BeNil(), audit.Reason)
			Expect(audit.State).To(Equal("answered"))
			Expect(audit.Attempts).To(Equal(2))
			Expect(headline(resp)).To(Equal("Number of call records: " + want + "."))
			Expect(lastPrompt(1)).To(ContainSubstring("event_time < '2026-04-04'"))
			Expect(resp.Enterprise["limitations"]).To(ContainElement(ContainSubstring("range: 2026-04-02 to 2026-04-03, both days in full")))
		})

		It("abstains, and shows no number, when the model keeps leaving the last day out", func() {
			model.replies = []string{sqlReply("SELECT COUNT(*) AS number_of_call_records FROM v_cdr WHERE event_time BETWEEN '2026-04-02' AND '2026-04-03'")}
			resp, audit := ask(question)
			Expect(resp).NotTo(BeNil())
			Expect(audit.State).To(Equal("abstained"))
			Expect(audit.Attempts).To(Equal(2))
			text := resp.Answer["clarification"].(string)
			Expect(text).To(ContainSubstring("did not include both of the days 2026-04-02 and 2026-04-03"))
			Expect(text).NotTo(ContainSubstring("Number of call records"))
			Expect(text).NotTo(MatchRegexp(`\d,\d{3}`))
		})

		It("takes a range written correctly on the first attempt, without a second", func() {
			model.replies = []string{sqlReply("SELECT COUNT(*) AS number_of_call_records FROM v_cdr WHERE event_time::date BETWEEN '2026-04-02' AND '2026-04-03'")}
			resp, audit := ask(question)
			Expect(audit.State).To(Equal("answered"), audit.Reason)
			Expect(audit.Attempts).To(Equal(1))
			Expect(headline(resp)).To(Equal("Number of call records: " + want + "."))
		})
	})

	Describe("a category stored in another letter case", func() {
		const question = "How many subscriber records have account status active?"
		It("does not show a zero that matching ignoring case refutes: one retry says to compare ignoring case", func() {
			Expect(oracle("SELECT count(*) " + records + "'subscriber' AND raw_payload->>'status' = 'ACTIVE'")).To(Equal("0"))
			model.replies = []string{
				sqlReply("SELECT COUNT(*) AS number_of_subscribers FROM v_subscriber WHERE status = 'ACTIVE'"),
				sqlReply("SELECT COUNT(*) AS number_of_subscribers FROM v_subscriber WHERE LOWER(status) = LOWER('ACTIVE')"),
			}
			resp, audit := ask(question)
			Expect(resp).NotTo(BeNil(), audit.Reason)
			Expect(audit.State).To(Equal("answered"))
			Expect(audit.Attempts).To(Equal(2))
			Expect(headline(resp)).To(Equal("Number of subscribers: " + oracle("SELECT count(*) "+records+"'subscriber' AND lower(raw_payload->>'status') = 'active'") + "."))
			Expect(headline(resp)).NotTo(ContainSubstring(": 0"))
			Expect(lastPrompt(1)).To(ContainSubstring("LOWER(status) = LOWER('ACTIVE')"))
			// the model was told that a match exists, never what the stored value is
			Expect(lastPrompt(1)).NotTo(ContainSubstring("'active'"))
		})

		It("abstains, and shows no zero, when the model keeps the exact spelling", func() {
			model.replies = []string{sqlReply("SELECT COUNT(*) AS number_of_subscribers FROM v_subscriber WHERE status = 'ACTIVE'")}
			resp, audit := ask(question)
			Expect(resp).NotTo(BeNil())
			Expect(audit.State).To(Equal("abstained"))
			text := resp.Answer["clarification"].(string)
			Expect(text).To(ContainSubstring("different letter case"))
			Expect(text).NotTo(MatchRegexp(`: 0\b`))
		})

		It("leaves a spelling that matches alone, a real zero, and a comparison already ignoring case", func() {
			model.replies = []string{sqlReply("SELECT COUNT(*) AS number_of_subscribers FROM v_subscriber WHERE status = 'active'")}
			resp, audit := ask(question)
			Expect(audit.State).To(Equal("answered"), audit.Reason)
			Expect(audit.Attempts).To(Equal(1))
			Expect(headline(resp)).To(Equal("Number of subscribers: 4."))

			model.calls = nil
			model.replies = []string{sqlReply("SELECT COUNT(*) AS number_of_subscribers FROM v_subscriber WHERE status = 'closed'")}
			resp, audit = ask("How many subscriber records have account status closed?")
			Expect(audit.State).To(Equal("answered"), audit.Reason)
			Expect(audit.Attempts).To(Equal(1))
			Expect(headline(resp)).To(Equal("Number of subscribers: 0."))
		})
	})

	Describe("a comparison", func() {
		const question = "How many incoming versus outgoing calls are there?"
		const total = "SELECT COUNT(*) AS number_of_calls FROM v_cdr WHERE direction IN ('INCOMING', 'OUTGOING')"

		It("is answered with one row per value, after one retry when the model gave one total", func() {
			model.replies = []string{sqlReply(total), sqlReply("SELECT direction, COUNT(*) AS number_of_calls FROM v_cdr WHERE direction IN ('INCOMING', 'OUTGOING') GROUP BY direction ORDER BY direction")}
			resp, audit := ask(question)
			Expect(resp).NotTo(BeNil(), audit.Reason)
			Expect(audit.State).To(Equal("answered"))
			Expect(audit.Attempts).To(Equal(2))
			incoming := govSQLFormatNumber(oracle("SELECT count(*) " + records + "'cdr' AND raw_payload->>'INBOUND_OUTBOUND_IND'='INCOMING'"))
			outgoing := govSQLFormatNumber(oracle("SELECT count(*) " + records + "'cdr' AND raw_payload->>'INBOUND_OUTBOUND_IND'='OUTGOING'"))
			Expect(headline(resp)).To(ContainSubstring(incoming))
			Expect(headline(resp)).To(ContainSubstring(outgoing))
			Expect(lastPrompt(1)).To(ContainSubstring("one row per value"))
			Expect(resp.Enterprise["limitations"]).To(ContainElement(ContainSubstring("comparison: one row per value")))
		})

		It("is declined to the existing path when the model keeps giving one total", func() {
			model.replies = []string{sqlReply(total)}
			resp, audit := ask(question)
			Expect(resp).To(BeNil())
			Expect(audit.State).To(Equal("declined"))
			Expect(audit.Reason).To(ContainSubstring("compares values"))
		})
	})

	Describe("a contact ranking", func() {
		const question = "Who did 923001110001 contact most frequently?"
		const withSubject = "SELECT dialed_number, COUNT(*) AS contacts FROM v_cdr WHERE msisdn = '923001110001' OR dialed_number = '923001110001' GROUP BY dialed_number ORDER BY contacts DESC LIMIT 5"
		const otherParty = "SELECT dialed_number, COUNT(*) AS contacts FROM v_cdr WHERE msisdn = '923001110001' AND dialed_number <> '923001110001' GROUP BY dialed_number ORDER BY contacts DESC LIMIT 5"

		It("does not answer with the number itself: one retry names the other party", func() {
			Expect(oracle("SELECT count(*) " + records + "'cdr' AND raw_payload->>'MSISDN'='923001110001' AND raw_payload->>'CALL_DIALED_NUM'='923001110001'")).To(Equal("425"))
			model.replies = []string{sqlReply(withSubject), sqlReply(otherParty)}
			resp, audit := ask(question)
			Expect(resp).NotTo(BeNil(), audit.Reason)
			Expect(audit.State).To(Equal("answered"))
			Expect(audit.Attempts).To(Equal(2))
			Expect(lastPrompt(1)).To(ContainSubstring("OTHER party"))
			Expect(headline(resp)).To(ContainSubstring("923009998887"))
			Expect(headline(resp)).NotTo(ContainSubstring("Contacts: 425"))
		})

		It("is declined to the path that ranks contacts across both directions when the subject stays on top", func() {
			model.replies = []string{sqlReply(withSubject)}
			resp, audit := ask(question)
			Expect(resp).To(BeNil())
			Expect(audit.State).To(Equal("declined"))
			Expect(audit.Reason).To(ContainSubstring("a contact ranking is left to the path"))
			Expect(audit.Attempts).To(Equal(2))
		})
	})

	It("declines a search of text evidence before the model and before the name check", func() {
		for _, q := range []string{
			"Search the audio transcripts for Japanese cuisine",
			"Which recording mentions coconut sugar and at what time?",
			"What does the audio transcript say about Nadia Farooqui?",
		} {
			resp, audit := ask(q)
			Expect(resp).To(BeNil(), q)
			Expect(audit.State).To(Equal("declined"), q)
			Expect(audit.Reason).NotTo(BeEmpty())
		}
		Expect(model.calls).To(BeEmpty(), "none of them reaches the model")
	})

	It("says distinct when the query counts distinct values", func() {
		model.replies = []string{sqlReply("SELECT COUNT(DISTINCT status) AS number_of_status_codes FROM v_access_log")}
		resp, audit := ask("How many distinct status codes appear in the access log?")
		Expect(audit.State).To(Equal("answered"), audit.Reason)
		Expect(headline(resp)).To(Equal("Number of distinct status codes: " + oracle("SELECT count(DISTINCT raw_payload->>'status') "+records+"'access_log'") + "."))
	})

	Describe("a server error from the existing path in stage 1", func() {
		run := func(status int, body any) *httptest.ResponseRecorder {
			GinkgoT().Setenv(governedSQLEnv, "true")
			GinkgoT().Setenv(governedSQLFirstEnv, "false")
			core := func(w http.ResponseWriter, r *http.Request) {
				if holder := govSQLHolderFrom(r.Context()); holder != nil {
					held := hybridQueryRequest{TenantID: "default", CollectionID: "records-demo", Query: "How many CDR records are there?", RequestClass: forensicrequest.GovernedAnalysis}
					holder.req = &held
				}
				writeJSON(w, status, body)
			}
			recorder := httptest.NewRecorder()
			governedSQLFallback(core, config{}, db)(recorder, httptest.NewRequest(http.MethodPost, "/query/hybrid", strings.NewReader(`{}`)))
			return recorder
		}

		It("is offered to the lane, and the lane's answer replaces the error", func() {
			model.replies = []string{sqlReply("SELECT COUNT(*) AS number_of_records FROM v_cdr")}
			recorder := run(http.StatusInternalServerError, map[string]any{"error": "source-native group cardinality exceeds 100"})
			Expect(recorder.Code).To(Equal(http.StatusOK))
			Expect(recorder.Header().Get("X-Governed-SQL")).To(ContainSubstring("state=answered"))
			Expect(recorder.Body.String()).To(ContainSubstring("Number of records"))
		})

		It("keeps the original error when the lane declines too", func() {
			model.failure = "TIMEOUT_OR_UNAVAILABLE"
			recorder := run(http.StatusInternalServerError, map[string]any{"error": "source-native group cardinality exceeds 100"})
			Expect(recorder.Code).To(Equal(http.StatusInternalServerError))
			Expect(recorder.Body.String()).To(ContainSubstring("cardinality exceeds 100"))
		})

		It("still passes a client error through untouched, without asking the model", func() {
			recorder := run(http.StatusBadRequest, map[string]any{"error": "bad request"})
			Expect(recorder.Code).To(Equal(http.StatusBadRequest))
			Expect(model.calls).To(BeEmpty())
		})
	})
})
