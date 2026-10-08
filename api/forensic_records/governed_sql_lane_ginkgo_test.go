package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudler/LocalAI/pkg/forensicrequest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// scriptedModel stands in for the endpoint: it returns the next canned reply and
// records every prompt it was shown, so the specs can also prove what the model sees.
type scriptedModel struct {
	replies []string
	failure string
	calls   [][]govSQLTurn
}

func (m *scriptedModel) ask(_ context.Context, _ config, _ string, turns []govSQLTurn) (string, string) {
	m.calls = append(m.calls, append([]govSQLTurn(nil), turns...))
	if m.failure != "" {
		return "", m.failure
	}
	index := len(m.calls) - 1
	if index >= len(m.replies) {
		index = len(m.replies) - 1
	}
	return m.replies[index], ""
}

func sqlReply(sql string) string {
	raw, _ := json.Marshal(map[string]any{"answerable": true, "sql": sql})
	return string(raw)
}

const notAnswerable = `{"answerable": false, "sql": ""}`

var _ = Describe("Governed SQL lane (no database)", func() {
	It("renders values the way an analyst reads them", func() {
		Expect(govSQLHumanize("earliest_call")).To(Equal("Earliest call"))
		Expect(govSQLHumanize("")).To(Equal("Result"))
		Expect(govSQLFormatValue(int64(8642))).To(Equal("8,642"))
		Expect(govSQLFormatValue("5005")).To(Equal("5,005"))
		Expect(govSQLFormatValue("3905649.900000")).To(Equal("3,905,649.9"))
		Expect(govSQLFormatValue("0.123456789")).To(Equal("0.1235"))
		Expect(govSQLFormatValue("2026-04-02T00:00:02Z")).To(Equal("2026-04-02 00:00:02 UTC"))
		Expect(govSQLFormatValue(nil)).To(ContainSubstring("no value"))
		Expect(govSQLFormatValue("VOICE")).To(Equal("VOICE"))
	})

	It("builds a headline from the result's own columns", func() {
		view := &govSQLView{Display: "Call detail records"}
		one := &govSQLResult{Columns: []string{"number_of_calls"}, Rows: [][]any{{int64(5005)}}}
		Expect(govSQLHeadline(view, one, govSQLQuestionFacts{}, "", nil)).To(Equal("Number of calls: 5,005."))
		top := &govSQLResult{Columns: []string{"protocol", "sessions"}, Rows: [][]any{{"DNS", int64(1246)}, {"HTTP", int64(900)}}}
		Expect(govSQLHeadline(view, top, govSQLQuestionFacts{}, "", nil)).To(ContainSubstring("In full: DNS: 1,246; HTTP: 900"))
		Expect(govSQLHeadline(view, &govSQLResult{Columns: []string{"x"}}, govSQLQuestionFacts{}, "", nil)).To(Equal("The query matched no rows."))
		Expect(govSQLHeadline(view, one, govSQLQuestionFacts{}, "every identifier field", []string{"923000000001"})).To(ContainSubstring("No record in this case contains 923000000001"))
	})

	It("knows an empty aggregate from a real value", func() {
		Expect(govSQLResultEmpty(&govSQLResult{})).To(BeTrue())
		Expect(govSQLResultEmpty(&govSQLResult{Rows: [][]any{{int64(0)}}})).To(BeTrue())
		Expect(govSQLResultEmpty(&govSQLResult{Rows: [][]any{{nil}}})).To(BeTrue())
		Expect(govSQLResultEmpty(&govSQLResult{Rows: [][]any{{"0"}}})).To(BeTrue())
		Expect(govSQLResultEmpty(&govSQLResult{Rows: [][]any{{int64(3)}}})).To(BeFalse())
		Expect(govSQLResultEmpty(&govSQLResult{Rows: [][]any{{"VOICE"}}})).To(BeFalse())
	})

	It("reads the model's reply strictly", func() {
		answerable, sql, failure := govSQLParseOutput(`{"answerable": true, "sql": "SELECT 1"}`)
		Expect(failure).To(BeEmpty())
		Expect(answerable).To(BeTrue())
		Expect(sql).To(Equal("SELECT 1"))
		for _, bad := range []string{"", "SELECT 1", `{"answerable": true}`, `{"answerable": true, "sql": "x", "extra": 1}`, `{"answerable": true, "sql": "x"} trailing`} {
			_, _, failure := govSQLParseOutput(bad)
			if bad == `{"answerable": true}` {
				Expect(failure).To(BeEmpty())
				continue
			}
			Expect(failure).To(Equal("INVALID_JSON"), bad)
		}
	})

	It("shows the model the schema and the question and no case data", func() {
		views := govSQLTestViews()
		facts := govSQLExtractFacts("How many calls did 923001110001 make?", []*govSQLView{views["v_cdr"]}, nil)
		system := govSQLSystemPrompt([]*govSQLView{views["v_cdr"]})
		user := govSQLUserPrompt("How many calls did 923001110001 make?", facts, []*govSQLView{views["v_cdr"]})
		Expect(system).To(ContainSubstring("VIEW v_cdr"))
		Expect(system).To(ContainSubstring("call_start timestamptz"))
		Expect(system).To(ContainSubstring("msisdn text [identifier]"))
		Expect(system).To(ContainSubstring("values: VOICE, VOLTE"))
		Expect(system).NotTo(ContainSubstring("923001110001"))
		Expect(user).To(ContainSubstring("923001110001"))
		Expect(user).To(ContainSubstring("dialed_number, msisdn, originating_number, imei, imsi"))
		// the schema does not describe sensitive fields at all
		Expect(system).NotTo(ContainSubstring("cnic"))
	})

	It("declines what is not a free-text data question", func() {
		base := hybridQueryRequest{TenantID: "t", CollectionID: "c", Query: "how many calls", RequestClass: forensicrequest.GovernedAnalysis}
		ok, _ := govSQLEligible(base)
		Expect(ok).To(BeTrue())
		for _, mutate := range []func(*hybridQueryRequest){
			func(r *hybridQueryRequest) { r.RequestClass = forensicrequest.ProductHelp },
			func(r *hybridQueryRequest) { r.Template = "cdr_summary" },
			func(r *hybridQueryRequest) { r.Group = &CanonicalGroupV1{} },
			func(r *hybridQueryRequest) { r.Query = "  " },
			func(r *hybridQueryRequest) { r.CollectionID = "" },
		} {
			req := base
			mutate(&req)
			ok, reason := govSQLEligible(req)
			Expect(ok).To(BeFalse())
			Expect(reason).NotTo(BeEmpty())
		}
	})

	It("recognises an abstention in the existing path's response", func() {
		for _, body := range []string{
			`{"route":["clarification"]}`, `{"route":["verified_only_withheld"]}`, `{"intent":"clarification"}`,
			`{"clarification":{"question":"which?"}}`, `{"answer":{"result_state":"CLARIFICATION_REQUIRED"}}`,
			`{"enterprise":{"clarification":"Please restate"}}`, `{"enterprise":{"status":"needs_input"}}`,
		} {
			Expect(govSQLOldPathDeclined([]byte(body))).To(BeTrue(), body)
		}
		for _, body := range []string{
			`{"route":["records"],"answer":{"result_state":"COMPLETED"},"enterprise":{"status":"completed","executive_answer":"There are 8,642."}}`,
			`not json`, `{}`,
		} {
			Expect(govSQLOldPathDeclined([]byte(body))).To(BeFalse(), body)
		}
	})

	It("is a pass-through when its switches are off, and never touches the answer", func() {
		GinkgoT().Setenv(governedSQLEnv, "false")
		called := 0
		core := func(w http.ResponseWriter, _ *http.Request) {
			called++
			w.Header().Set("X-Test", "kept")
			writeJSON(w, http.StatusOK, map[string]any{"answer": "from the existing path"})
		}
		recorder := httptest.NewRecorder()
		governedSQLFallback(core, config{}, nil)(recorder, httptest.NewRequest(http.MethodPost, "/query/hybrid", strings.NewReader(`{}`)))
		Expect(called).To(Equal(1))
		Expect(recorder.Header().Get("X-Test")).To(Equal("kept"))
		Expect(recorder.Body.String()).To(ContainSubstring("from the existing path"))
		Expect(recorder.Header().Get("X-Governed-SQL")).To(BeEmpty())
	})
})

// With a database: the whole loop, against the real rows, with a scripted model.
var _ = Describe("Governed SQL lane (with a database)", func() {
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
	ask := func(question string) (*hybridQueryResponse, GovSQLAuditV1) {
		req := hybridQueryRequest{TenantID: "default", CollectionID: "records-demo", Query: question, RequestClass: forensicrequest.GovernedAnalysis, SynthesisModel: "scripted"}
		return runGovernedSQLLane(ctx, config{}, db, req, timeNow())
	}
	headline := func(resp *hybridQueryResponse) string {
		return resp.Enterprise["executive_answer"].(string)
	}

	It("answers a count with the number the database computed, and shows the query", func() {
		model.replies = []string{sqlReply("SELECT COUNT(*) AS number_of_records FROM v_cdr")}
		resp, audit := ask("How many CDR records are there?")
		Expect(resp).NotTo(BeNil())
		Expect(audit.State).To(Equal("answered"))
		Expect(audit.Attempts).To(Equal(1))
		Expect(headline(resp)).To(Equal("Number of records: " + govSQLFormatNumber(oracle("SELECT count(*) "+cdr)) + "."))
		Expect(resp.Route).To(Equal([]string{"governed_sql"}))
		derivation := resp.Enterprise["derivation"].(map[string]any)
		Expect(derivation["sql"]).To(ContainSubstring("count(*)"))
		Expect(resp.Enterprise["data_grid"].(map[string]any)["count"]).To(Equal(1))
		// the model was shown the schema, and none of the case's own values
		prompt := model.calls[0][0].Content + model.calls[0][1].Content
		Expect(prompt).To(ContainSubstring("VIEW v_cdr"))
		for _, secret := range []string{"923001110001", "Gulberg", "CELL-GLB-01", "35678901123747"} {
			Expect(prompt).NotTo(ContainSubstring(secret))
		}
	})

	It("answers earliest from a typed timestamp, on the case clock", func() {
		model.replies = []string{sqlReply("SELECT MIN(call_start) AS earliest_call FROM v_cdr")}
		resp, audit := ask("What is the earliest call in the data?")
		Expect(audit.State).To(Equal("answered"))
		want := oracle("SELECT to_char(min(" + govSQLTestCaseClock("raw_payload->>'CALL_START_DT_TM'") + "), 'YYYY-MM-DD HH24:MI:SS') " + cdr)
		Expect(headline(resp)).To(Equal("Earliest call: " + want + " PKT."))
	})

	It("applies a declared value the question says as a code", func() {
		model.replies = []string{sqlReply("SELECT COUNT(*) AS number_of_calls FROM v_cdr WHERE call_type = 'VOLTE'")}
		resp, audit := ask("How many VoLTE calls are there?")
		Expect(audit.State).To(Equal("answered"))
		Expect(headline(resp)).To(Equal("Number of calls: " + govSQLFormatNumber(oracle("SELECT count(*) "+cdr+" AND raw_payload->>'CALL_TYPE'='VOLTE'")) + "."))
		Expect(audit.Checked).To(ContainElement("call_type = VOLTE"))
	})

	It("repairs a query that dropped the identifier, once, and says what it checked", func() {
		model.replies = []string{
			sqlReply("SELECT COUNT(*) AS number_of_calls FROM v_cdr"),
			sqlReply("SELECT COUNT(*) AS number_of_calls FROM v_cdr WHERE msisdn = '923001110001'"),
		}
		resp, audit := ask("How many calls did 923001110001 make?")
		Expect(audit.State).To(Equal("answered"))
		Expect(audit.Attempts).To(Equal(2))
		Expect(headline(resp)).To(Equal("Number of calls: " + govSQLFormatNumber(oracle("SELECT count(*) "+cdr+" AND raw_payload->>'MSISDN'='923001110001'")) + "."))
		Expect(model.calls[1][len(model.calls[1])-1].Content).To(ContainSubstring("923001110001"))
		Expect(resp.Enterprise["limitations"]).To(ContainElement(ContainSubstring("identifier 923001110001")))
	})

	It("abstains, and shows no number, when the identifier is still dropped on the second try", func() {
		model.replies = []string{sqlReply("SELECT COUNT(*) AS number_of_calls FROM v_cdr")}
		resp, audit := ask("How many calls did 923001110001 make?")
		Expect(audit.State).NotTo(Equal("answered"))
		Expect(resp).NotTo(BeNil())
		Expect(resp.Intent).To(Equal(intentClarify))
		text := resp.Answer["clarification"].(string)
		Expect(text).To(ContainSubstring("923001110001"))
		Expect(text).To(ContainSubstring("could not apply"))
		Expect(text).NotTo(MatchRegexp(`\d,\d{3}|\b5005\b`))
		Expect(audit.Attempts).To(Equal(2))
	})

	It("abstains at once for a name no column can hold, however the model answers", func() {
		for _, reply := range []string{sqlReply("SELECT COUNT(*) AS number_of_calls FROM v_cdr"), notAnswerable} {
			model.replies, model.calls = []string{reply}, nil
			resp, audit := ask("How many calls did Ahmed make?")
			Expect(resp).NotTo(BeNil(), reply)
			Expect(audit.Attempts).To(Equal(1))
			text := resp.Answer["clarification"].(string)
			Expect(text).To(ContainSubstring("Ahmed"))
			Expect(text).To(ContainSubstring("personal names"))
		}
	})

	It("lets the model decline a question the columns cannot answer, and hands it back", func() {
		model.replies = []string{notAnswerable}
		resp, audit := ask("How many calls were dropped?")
		Expect(resp).To(BeNil())
		Expect(audit.State).To(Equal("declined"))
		Expect(audit.Reason).To(ContainSubstring("not answerable"))
	})

	It("declines when no evidence family is recognised, without calling the model", func() {
		resp, audit := ask("qwerty asdf zxcv")
		Expect(resp).To(BeNil())
		Expect(audit.State).To(Equal("declined"))
		Expect(model.calls).To(BeEmpty())
	})

	It("declines a question that asks for an attribute the evidence withholds", func() {
		resp, audit := ask("What is the CNIC of subscriber 923461678183?")
		Expect(resp).To(BeNil())
		Expect(audit.Reason).To(ContainSubstring("withholds"))
		Expect(model.calls).To(BeEmpty())
	})

	It("tells the model why a query was rejected and accepts the corrected one", func() {
		model.replies = []string{
			sqlReply("SELECT COUNT(*) FROM forensic.records"),
			sqlReply("SELECT COUNT(*) AS number_of_records FROM v_cdr"),
		}
		resp, audit := ask("How many CDR records are there?")
		Expect(audit.State).To(Equal("answered"))
		Expect(audit.Attempts).To(Equal(2))
		Expect(model.calls[1][len(model.calls[1])-1].Content).To(ContainSubstring("schema"))
		Expect(headline(resp)).To(ContainSubstring(govSQLFormatNumber(oracle("SELECT count(*) " + cdr))))
	})

	It("declines when the model cannot produce a valid query in two tries", func() {
		model.replies = []string{sqlReply("DROP TABLE forensic.records")}
		resp, audit := ask("How many CDR records are there?")
		Expect(resp).To(BeNil())
		Expect(audit.Reason).To(ContainSubstring("rejected"))
	})

	It("declines, rather than guessing, when the model is unavailable", func() {
		model.failure = "TIMEOUT_OR_UNAVAILABLE"
		resp, audit := ask("How many CDR records are there?")
		Expect(resp).To(BeNil())
		Expect(audit.Reason).To(ContainSubstring("TIMEOUT_OR_UNAVAILABLE"))
	})

	It("feeds a database error back and takes the corrected query", func() {
		model.replies = []string{
			sqlReply("SELECT CAST(location AS integer) AS bad FROM v_cdr"),
			sqlReply("SELECT COUNT(*) AS number_of_records FROM v_cdr"),
		}
		_, audit := ask("How many CDR records are there?")
		Expect(audit.State).To(Equal("answered"))
		Expect(audit.Attempts).To(Equal(2))
		Expect(model.calls[1][len(model.calls[1])-1].Content).To(ContainSubstring("database rejected"))
	})

	It("retries a list returned where one number was asked for", func() {
		model.replies = []string{
			sqlReply("SELECT call_type, COUNT(*) AS number_of_calls FROM v_cdr GROUP BY call_type"),
			sqlReply("SELECT COUNT(*) AS number_of_calls FROM v_cdr"),
		}
		resp, audit := ask("How many calls are there?")
		Expect(audit.State).To(Equal("answered"))
		Expect(audit.Attempts).To(Equal(2))
		Expect(headline(resp)).To(HavePrefix("Number of calls: "))
	})

	It("states its assumption when 'night' is used, and reads it on the case clock", func() {
		model.replies = []string{sqlReply("SELECT COUNT(*) AS number_of_sessions FROM v_ipdr WHERE EXTRACT(HOUR FROM event_time) BETWEEN 0 AND 5")}
		resp, audit := ask("How many data sessions were there at night?")
		Expect(audit.State).To(Equal("answered"))
		want := oracle("SELECT count(*) FROM forensic.records WHERE tenant_id='default' AND collection_id='records-demo' AND record_type='ipdr' AND extract(hour from " + govSQLTestCaseClock("raw_payload->>'timestamp'") + ") BETWEEN 0 AND 5")
		Expect(headline(resp)).To(Equal("Number of sessions: " + govSQLFormatNumber(want) + "."))
		Expect(resp.Enterprise["limitations"]).To(ContainElement(ContainSubstring("'night' was taken as 00:00 to 05:59")))
		Expect(resp.Enterprise["limitations"]).To(ContainElement(ContainSubstring("case clock (Asia/Karachi, UTC+05:00)")))
	})

	It("refuses 'at night' answered with no hour in the query", func() {
		model.replies = []string{sqlReply("SELECT COUNT(*) AS number_of_sessions FROM v_ipdr")}
		resp, audit := ask("How many data sessions were there at night?")
		Expect(resp).NotTo(BeNil())
		Expect(audit.State).NotTo(Equal("answered"))
		Expect(resp.Intent).To(Equal(intentClarify))
	})

	It("finds a value that sits in a free-text column the curator did not flag as an identifier", func() {
		layer, _ := defaultSemanticLayer()
		req := hybridQueryRequest{TenantID: "default", CollectionID: "records-demo", Query: "x"}
		all := govSQLViews(layer, req)
		var target string
		var column string
		// pick any non-identifier, non-enumerated text column of a records view, and a real value from it
		for _, view := range all {
			if view.binding.Derived {
				continue
			}
			for _, c := range view.Columns {
				if c.Type == "text" && !c.Identifier && len(c.Values) == 0 && c.Name != "source_file" && c.Name != "record_id" {
					cte, args, err := govSQLCTE(view, req, nil)
					Expect(err).NotTo(HaveOccurred())
					var value string
					row := db.QueryRow(ctx, "WITH "+cte+" SELECT "+c.Name+" FROM "+view.Name+" WHERE "+c.Name+" IS NOT NULL AND length("+c.Name+") BETWEEN 4 AND 30 LIMIT 1", args...)
					if row.Scan(&value) == nil && value != "" {
						target, column = value, view.Name+"."+c.Name
						break
					}
				}
			}
			if target != "" {
				break
			}
		}
		if target == "" {
			Skip("no non-identifier text column with data in this database")
		}
		hits, err := govSQLProbeIdentifiers(ctx, db, req, all, []string{target})
		Expect(err).NotTo(HaveOccurred())
		found := false
		for _, hit := range hits[target] {
			if hit.View.Name+"."+hit.Column == column {
				found = true
			}
		}
		Expect(found).To(BeTrue(), "the probe must search "+column)
	})

	It("says which time column a time condition used", func() {
		model.replies = []string{sqlReply("SELECT COUNT(*) AS number_of_call_records FROM v_cdr WHERE event_time >= '2026-06-01' AND event_time < '2026-07-01'")}
		resp, audit := ask("How many call records were there in June 2026?")
		Expect(resp).NotTo(BeNil(), audit.Reason)
		Expect(strings.Join(toStrings(resp.Enterprise["limitations"]), " ")).To(ContainSubstring("The time condition was applied to: Event time (event_time)"))
	})

	// Run 11: "at night" was answered on the call's start OR end (1,771 against 1,768), a reading the
	// question never asked for (governed_sql_timecolumn.go).
	const startOrEnd = "SELECT COUNT(*) AS number_of_call_records FROM v_cdr WHERE EXTRACT(HOUR FROM call_start) BETWEEN 0 AND 5 OR EXTRACT(HOUR FROM call_end) BETWEEN 0 AND 5"

	It("moves a time condition from the call's start and end to the record's own time, once, and the count is the one the key gives", func() {
		model.replies = []string{
			sqlReply(startOrEnd),
			sqlReply("SELECT COUNT(*) AS number_of_call_records FROM v_cdr WHERE EXTRACT(HOUR FROM event_time) BETWEEN 0 AND 5"),
		}
		resp, audit := ask("How many call records were there at night?")
		Expect(resp).NotTo(BeNil(), audit.Reason)
		Expect(audit.State).To(Equal("answered"))
		Expect(audit.Attempts).To(Equal(2))
		Expect(model.calls[1][len(model.calls[1])-1].Content).To(ContainSubstring("belongs on event_time"))
		want := oracle("SELECT count(*) " + cdr + " AND extract(hour from " + govSQLTestCaseClock("raw_payload->>'CALL_START_DT_TM'") + ") BETWEEN 0 AND 5")
		Expect(headline(resp)).To(Equal("Number of call records: " + govSQLFormatNumber(want) + "."))
		Expect(strings.Join(toStrings(resp.Enterprise["limitations"]), " ")).To(ContainSubstring("The time condition was applied to: Event time (event_time)"))
	})

	It("tells the model the record's own time in the first message, so the first attempt is usually the only one", func() {
		model.replies = []string{sqlReply("SELECT COUNT(*) AS number_of_call_records FROM v_cdr WHERE EXTRACT(HOUR FROM event_time) BETWEEN 0 AND 5")}
		_, audit := ask("How many call records were there at night?")
		Expect(audit.State).To(Equal("answered"))
		Expect(audit.Attempts).To(Equal(1))
		Expect(model.calls[0][1].Content).To(ContainSubstring("on event_time, the time of the record"))
		Expect(model.calls[0][1].Content).To(ContainSubstring("Times are on the case clock"))
	})

	It("abstains, and says which time it would not use, when the model keeps conditioning on the call's start and end", func() {
		model.replies = []string{sqlReply(startOrEnd)}
		resp, audit := ask("How many call records were there at night?")
		Expect(audit.State).NotTo(Equal("answered"))
		Expect(resp).NotTo(BeNil())
		Expect(resp.Intent).To(Equal(intentClarify))
		text := resp.Answer["clarification"].(string)
		Expect(text).To(ContainSubstring("call_end, call_start"))
		Expect(text).To(ContainSubstring("event_time"))
		Expect(text).NotTo(MatchRegexp(`\d,\d{3}`))
		Expect(audit.Attempts).To(Equal(2))
	})

	// Run 12, part B: a date range was abstained on with "the query did not apply the condition 'between 2026'"
	// because the quantity detector read the year that begins a date as a number (governed_sql_verify.go).
	It("answers a date range between two dates, with the count the key gives, on the first attempt", func() {
		model.replies = []string{sqlReply("SELECT COUNT(*) AS number_of_call_records FROM v_cdr WHERE event_time >= '2026-04-02' AND event_time < '2026-04-04'")}
		resp, audit := ask("How many CDR records are there between 2026-04-02 and 2026-04-03?")
		Expect(resp).NotTo(BeNil(), audit.Reason)
		Expect(audit.State).To(Equal("answered"))
		Expect(audit.Attempts).To(Equal(1))
		want := oracle("SELECT count(*) " + cdr + " AND (" + govSQLTestCaseClock("raw_payload->>'CALL_START_DT_TM'") + ")::date BETWEEN '2026-04-02' AND '2026-04-03'")
		Expect(headline(resp)).To(Equal("Number of call records: " + govSQLFormatNumber(want) + "."))
		Expect(strings.Join(toStrings(resp.Enterprise["limitations"]), " ")).To(ContainSubstring("The time condition was applied to: Event time (event_time)"))
	})

	It("lets a question that says the call ended condition on the call's end", func() {
		model.replies = []string{sqlReply("SELECT COUNT(*) AS number_of_calls FROM v_cdr WHERE EXTRACT(HOUR FROM call_end) BETWEEN 0 AND 5")}
		resp, audit := ask("How many calls ended at night?")
		Expect(resp).NotTo(BeNil(), audit.Reason)
		Expect(audit.State).To(Equal("answered"))
		Expect(audit.Attempts).To(Equal(1))
		raw := "COALESCE(raw_payload->>'CALL_END_DT_TM', raw_payload->>'call_end_time', raw_payload->>'end_time')"
		want := oracle("SELECT count(*) " + cdr + " AND extract(hour from " + govSQLTestCaseClock(raw) + ") BETWEEN 0 AND 5")
		Expect(headline(resp)).To(Equal("Number of calls: " + govSQLFormatNumber(want) + "."))
		Expect(strings.Join(toStrings(resp.Enterprise["limitations"]), " ")).To(ContainSubstring("The time condition was applied to: Call end time (call_end)"))
	})

	It("leaves text and media questions to the retrieval path, and relationships to a join it does not attempt", func() {
		for _, q := range []string{"Is the number 03001234567 mentioned in any audio?", "Find OCR text mentioning Investigation Workspace", "Find plate LEB15491 in the images", "Do any subscribers share the same handset?"} {
			resp, audit := ask(q)
			Expect(resp).To(BeNil(), q)
			Expect(audit.State).To(Equal("declined"), q)
		}
		Expect(model.calls).To(BeEmpty(), "none of them reaches the model")
	})

	It("lists a short breakdown in full with the names the analyst reads", func() {
		model.replies = []string{sqlReply("SELECT call_type, COUNT(*) AS number_of_events FROM v_cdr GROUP BY call_type ORDER BY number_of_events DESC")}
		resp, audit := ask("Show the call type breakdown")
		Expect(resp).NotTo(BeNil(), audit.Reason)
		Expect(headline(resp)).To(ContainSubstring("In full:"))
		Expect(headline(resp)).To(ContainSubstring("Data session"))
		Expect(headline(resp)).NotTo(ContainSubstring("GPRS"))
		Expect(headline(resp)).NotTo(ContainSubstring("Top result"))
	})

	It("declines a question about the whole case or one that names no family, and still abstains on a name", func() {
		for _, q := range []string{"What time period does this case cover?", "Where does 03001234567 appear across all evidence?"} {
			resp, audit := ask(q)
			Expect(resp).To(BeNil(), q)
			Expect(audit.State).To(Equal("declined"), q)
		}
		// "log entries" named no family until run 16 gave the lane its own word for the access log, so the example is a noun
		// no family has
		resp, audit := ask("How many widgets did Nadia Farooqui have?")
		Expect(resp).NotTo(BeNil(), audit.Reason)
		Expect(audit.State).To(Equal("abstained"))
		Expect(model.calls).To(BeEmpty())
	})

	It("describes a list as a list, never as a ranking, and shows a short single column", func() {
		model.replies = []string{sqlReply("SELECT msisdn, call_duration_seconds FROM v_cdr WHERE call_duration_seconds > 600")}
		resp, audit := ask("Show me all the calls that lasted longer than 600 seconds")
		Expect(resp).NotTo(BeNil(), audit.Reason)
		Expect(headline(resp)).To(ContainSubstring("rows"))
		Expect(headline(resp)).To(ContainSubstring("Columns:"))
		Expect(headline(resp)).NotTo(ContainSubstring("Top result"))
		model.replies = []string{sqlReply("SELECT DISTINCT call_type FROM v_cdr ORDER BY call_type")}
		resp, audit = ask("Which call types are there in the calls?")
		Expect(resp).NotTo(BeNil(), audit.Reason)
		Expect(headline(resp)).To(ContainSubstring("values (call type)"))
		Expect(headline(resp)).To(ContainSubstring("Data session"))
	})

	It("abstains on a name even when no evidence family is recognised", func() {
		model.replies = []string{sqlReply("SELECT COUNT(*) AS n FROM v_cdr")}
		resp, audit := ask("How many items did Nadia Farooqui have?")
		Expect(resp).NotTo(BeNil(), audit.Reason)
		Expect(audit.State).To(Equal("abstained"), audit.Reason)
		Expect(model.calls).To(BeEmpty(), "no model call is needed to say a name cannot be bound")
	})

	Describe("when a query for an identifier finds nothing", func() {
		It("looks in the other column of the same view before saying so", func() {
			model.replies = []string{
				sqlReply("SELECT COUNT(*) AS number_of_calls FROM v_cdr WHERE msisdn = '923009998886'"),
				sqlReply("SELECT COUNT(*) AS number_of_calls FROM v_cdr WHERE msisdn = '923009998886' OR dialed_number = '923009998886'"),
			}
			resp, audit := ask("How many calls involved 923009998886?")
			Expect(audit.State).To(Equal("answered"), audit.Reason)
			Expect(audit.Attempts).To(Equal(2))
			want := oracle("SELECT count(*) " + cdr + " AND (raw_payload->>'MSISDN'='923009998886' OR raw_payload->>'CALL_DIALED_NUM'='923009998886')")
			Expect(want).NotTo(Equal("0"))
			Expect(headline(resp)).To(Equal("Number of calls: " + govSQLFormatNumber(want) + "."))
			Expect(model.calls[1][len(model.calls[1])-1].Content).To(ContainSubstring("dialed_number"))
		})

		It("points to the family that holds it rather than reporting zero", func() {
			model.replies = []string{sqlReply("SELECT COUNT(*) AS number_of_sessions FROM v_ipdr WHERE subscriber_id = '923001112222'")}
			resp, audit := ask("How many data sessions did 923001112222 have?")
			Expect(resp).NotTo(BeNil())
			Expect(audit.State).NotTo(Equal("answered"))
			text := resp.Answer["clarification"].(string)
			Expect(text).To(ContainSubstring("923001112222"))
			Expect(text).To(ContainSubstring("does appear in"))
			Expect(resp.Enterprise["limitations"]).To(ContainElement(ContainSubstring("No count of zero is reported")))
		})

		It("reports an absence only after searching every family", func() {
			model.replies = []string{sqlReply("SELECT COUNT(*) AS number_of_calls FROM v_cdr WHERE msisdn = '923000000001' OR dialed_number = '923000000001'")}
			resp, audit := ask("How many calls did 923000000001 make?")
			Expect(audit.State).To(Equal("answered"))
			Expect(headline(resp)).To(ContainSubstring("No record in this case contains 923000000001"))
			Expect(headline(resp)).To(ContainSubstring("every identifier field of the structured evidence"))
		})
	})

	Describe("as the fallback after the existing path", func() {
		run := func(coreBody any, status int) (*httptest.ResponseRecorder, int) {
			GinkgoT().Setenv(governedSQLEnv, "true")
			GinkgoT().Setenv(governedSQLFirstEnv, "false")
			calls := 0
			core := func(w http.ResponseWriter, r *http.Request) {
				calls++
				if holder := govSQLHolderFrom(r.Context()); holder != nil {
					held := hybridQueryRequest{TenantID: "default", CollectionID: "records-demo", Query: "How many CDR records are there?", RequestClass: forensicrequest.GovernedAnalysis}
					holder.req = &held
				}
				w.Header().Set("X-Request-ID", "r1")
				writeJSON(w, status, coreBody)
			}
			recorder := httptest.NewRecorder()
			governedSQLFallback(core, config{}, db)(recorder, httptest.NewRequest(http.MethodPost, "/query/hybrid", strings.NewReader(`{}`)))
			return recorder, calls
		}

		It("answers when the existing path asked for clarification", func() {
			model.replies = []string{sqlReply("SELECT COUNT(*) AS number_of_records FROM v_cdr")}
			recorder, calls := run(map[string]any{"route": []string{"clarification"}, "intent": "clarification"}, http.StatusOK)
			Expect(calls).To(Equal(1))
			Expect(recorder.Header().Get("X-Governed-SQL")).To(ContainSubstring("state=answered"))
			Expect(recorder.Header().Get("X-Request-ID")).To(Equal("r1"))
			Expect(recorder.Body.String()).To(ContainSubstring("Number of records"))
		})

		It("never replaces an answer the existing path gave", func() {
			model.replies = []string{sqlReply("SELECT COUNT(*) AS number_of_records FROM v_cdr")}
			recorder, _ := run(map[string]any{"route": []string{"records"}, "answer": map[string]any{"result_state": "COMPLETED"}, "enterprise": map[string]any{"status": "completed", "executive_answer": "There are 8,642 CDR records."}}, http.StatusOK)
			Expect(model.calls).To(BeEmpty(), "the model must not even be asked")
			Expect(recorder.Body.String()).To(ContainSubstring("There are 8,642 CDR records."))
			Expect(recorder.Header().Get("X-Governed-SQL")).To(BeEmpty())
		})

		It("keeps the existing refusal when the lane declines too", func() {
			model.failure = "TIMEOUT_OR_UNAVAILABLE"
			recorder, _ := run(map[string]any{"route": []string{"clarification"}, "intent": "clarification", "marker": "original"}, http.StatusOK)
			Expect(recorder.Body.String()).To(ContainSubstring("original"))
			Expect(recorder.Header().Get("X-Governed-SQL")).To(ContainSubstring("state=declined"))
		})

		It("treats a terminal 'definition unavailable' reply as declined, but only an answer replaces it", func() {
			terminal := map[string]any{"route": []string{"terminal"}, "answer": map[string]any{"result_state": "COMPLETED", "grounding": "GENERAL_DOMAIN_DEFINITION", "answer": "A bounded general definition is unavailable for that term."}}
			Expect(govSQLOldPathDeclined(mustJSON(terminal))).To(BeTrue())
			Expect(govSQLOldPathDeclined(mustJSON(map[string]any{"answer": map[string]any{"grounding": "GENERAL_DOMAIN_DEFINITION", "answer": "A CDR is a call detail record."}}))).To(BeFalse())
		})

		It("passes an error status through untouched", func() {
			recorder, _ := run(map[string]any{"error": "bad request"}, http.StatusBadRequest)
			Expect(recorder.Code).To(Equal(http.StatusBadRequest))
			Expect(model.calls).To(BeEmpty())
		})
	})
})

var _ = Describe("governed SQL reach", func() {
	It("lets a computed-value question labelled a concept or clarification reach the lane", func() {
		for _, class := range []forensicrequest.Class{forensicrequest.GeneralDomainKnowledge, forensicrequest.Clarify} {
			for _, q := range []string{"What is the earliest request time in the access logs?", "What is the latest transaction?", "How many CDR records are there?", "What is the total amount?"} {
				Expect(govSQLReclassifiable(hybridQueryRequest{Query: q, RequestClass: class})).To(BeTrue(), q)
			}
		}
	})
	It("leaves concepts, greetings, help and unsupported requests alone", func() {
		for _, q := range []string{"What is a CDR?", "hello", "What does IPDR stand for?", "How do I upload a file?"} {
			Expect(govSQLReclassifiable(hybridQueryRequest{Query: q, RequestClass: forensicrequest.GeneralDomainKnowledge})).To(BeFalse(), q)
		}
		Expect(govSQLReclassifiable(hybridQueryRequest{Query: "How many records?", RequestClass: forensicrequest.ProductHelp})).To(BeFalse())
		Expect(govSQLReclassifiable(hybridQueryRequest{Query: "How many records?", RequestClass: forensicrequest.Unsupported})).To(BeFalse())
	})
	It("is eligible for the lane only with a scope and no explicit operation", func() {
		ok, _ := govSQLEligible(hybridQueryRequest{Query: "latest transaction?", RequestClass: forensicrequest.GeneralDomainKnowledge, TenantID: "t", CollectionID: "c"})
		Expect(ok).To(BeTrue())
		ok, _ = govSQLEligible(hybridQueryRequest{Query: "What is a CDR?", RequestClass: forensicrequest.GeneralDomainKnowledge, TenantID: "t", CollectionID: "c"})
		Expect(ok).To(BeFalse())
	})
})

func timeNow() time.Time { return time.Now() }

func toStrings(value any) []string {
	switch v := value.(type) {
	case []string:
		return v
	case []any:
		out := []string{}
		for _, item := range v {
			if text, ok := item.(string); ok {
				out = append(out, text)
			}
		}
		return out
	}
	return nil
}
