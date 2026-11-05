package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudler/LocalAI/pkg/forensicrequest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Engine round 9e: the faults the full verified-bank run of 2026-11-02 left (docs/data/engine-round-9e-20261103.md). Every spec here fails when the fix it belongs to is taken out; the
// mutation checks that prove it are in that note.

const (
	r9eAccount = "PK00SYNT0000000000002005"
	r9eNumber  = "923000001038"
)

var _ = Describe("Ask engine round 9e: the direction is relative to the identifier the question names", func() {
	var cat *askCatalogue
	BeforeEach(func() {
		var err error
		cat, err = askGoldenCatalogue()
		Expect(err).NotTo(HaveOccurred())
	})
	check := func(question, sql string, views ...string) []string {
		valid, rej := askValidate(sql, cat, views)
		Expect(rej).To(BeNil(), sql)
		out := []string{}
		for _, o := range askCheckFrame(askExtractFrame(question), valid) {
			out = append(out, o.Kind)
		}
		return out
	}

	It("reads an identifier after 'to' as the receiving end and one after 'from' as the sending end", func() {
		for question, want := range map[string]string{
			"Which accounts or numbers sent money to the account " + r9eAccount + "?":  "incoming",
			"Who paid money to " + r9eAccount + "?":                                    "incoming",
			"Which numbers sent the most text messages to " + r9eNumber + "?":           "incoming",
			"How many calls were made to " + r9eNumber + "?":                           "incoming",
			"Which accounts received money from " + r9eAccount + "?":                   "outgoing",
			"How many calls from " + r9eNumber + " are there?":                          "outgoing",
		} {
			f := askExtractFrame(question)
			other := map[string]string{"incoming": "outgoing", "outgoing": "incoming"}[want]
			Expect(f.DirectionRole).To(Equal(want), question)
			Expect(f.Cues[want]).To(BeTrue(), question)
			Expect(f.Cues[other]).To(BeFalse(), question)
			Expect(f.RoleIDs).To(HaveLen(1), question)
		}
	})

	It("keeps the reading of the verbs when the question names the doer, or names both ends, or names no verb", func() {
		for question, cue := range map[string]string{
			"Which number did 923000001006 send the most text messages to?":            "outgoing",
			"How many times did 923000001001 get a call or message from 923000001036?": "incoming",
			"How many calls did 923000001001 make to 923000001036?":                    "outgoing",
			"Messages from 923000001036 to 923000001001":                               "incoming",
			"Which accounts did " + r9eAccount + " send money to?":                      "outgoing",
			"Did 923001110001 send money to the account " + r9eAccount + "?":            "outgoing",
		} {
			f := askExtractFrame(question)
			Expect(f.DirectionRole).To(BeEmpty(), question)
			Expect(f.Cues[cue]).To(BeTrue(), question)
		}
		for _, question := range []string{"Who does 923000000101 talk to most?", "Which numbers may have taken over from 923001110001?", "Calls made by 923000000101"} {
			Expect(askExtractFrame(question).DirectionRole).To(BeEmpty(), question)
		}
	})

	It("accepts the query that reads the credits of the account for 'who sent money to it', and refuses the one that reads what it paid", func() {
		q := "Which accounts or numbers sent money to the account " + r9eAccount + "?"
		credits := "SELECT counterparty, transactions, total_amount FROM v_account_counterparties WHERE account = '" + r9eAccount + "' AND direction = 'credit' ORDER BY total_amount DESC LIMIT 200"
		debits := strings.Replace(credits, "'credit'", "'debit'", 1)
		Expect(check(q, credits, "v_account_counterparties")).To(BeEmpty(), "the verified example's own reading must meet every obligation of its question")
		Expect(check(q, debits, "v_account_counterparties")).To(ContainElement("DIRECTION_ROLE"))
		// the same events read from the senders' side: the account in the column of the other end, and the senders' own direction
		senders := "SELECT account, sum(amount) AS total FROM v_bank_transactions WHERE counterparty_account = '" + r9eAccount + "' AND direction = 'debit' GROUP BY account ORDER BY total DESC LIMIT 200"
		Expect(check(q, senders, "v_bank_transactions")).To(BeEmpty())
		Expect(check(q, strings.Replace(senders, "'debit'", "'credit'", 1), "v_bank_transactions")).To(ContainElement("DIRECTION_ROLE"))
	})

	It("holds a question about messages to the same reading, in both columns", func() {
		q := "Which numbers sent the most text messages to " + r9eNumber + "?"
		head := "SELECT %s AS label, count(*) AS value FROM v_comm_events WHERE %s = '" + r9eNumber + "' AND event_type = 'sms' AND direction = '%s' GROUP BY %s ORDER BY value DESC LIMIT 5"
		query := func(other, column, direction string) string { return fmt.Sprintf(head, other, column, direction, other) }
		Expect(check(q, query("dst", "src", "incoming"), "v_comm_events")).To(BeEmpty(), "its own incoming messages, the other party of each")
		Expect(check(q, query("src", "dst", "outgoing"), "v_comm_events")).To(BeEmpty(), "the senders' own outgoing messages to it")
		Expect(check(q, query("dst", "src", "outgoing"), "v_comm_events")).To(ContainElement("DIRECTION_ROLE"), "what it sent")
		Expect(check(q, query("src", "dst", "incoming"), "v_comm_events")).To(ContainElement("DIRECTION_ROLE"), "what the subscribers received from it")
	})

	It("tells the model which end of the event the identifier is, in the question turn and in the repair", func() {
		f := askExtractFrame("Which accounts or numbers sent money to the account " + r9eAccount + "?")
		Expect(askUserPrompt(f)).To(ContainSubstring("is the receiving end"))
		Expect(askUserPrompt(f)).NotTo(ContainSubstring("Outgoing events only."))
		message, _ := askRoleDirectionMessage("incoming", []string{r9eAccount}, false)
		Expect(message).To(ContainSubstring("direction = 'credit'"))
		flipped, _ := askRoleDirectionMessage("incoming", []string{r9eAccount}, true)
		Expect(flipped).To(ContainSubstring("'outgoing' ('debit' for money)"))
	})

	It("holds every verified example to the obligations of its own question", func() {
		examples, err := askExamples()
		Expect(err).NotTo(HaveOccurred())
		refused := []string{}
		for _, e := range examples {
			valid, rej := askValidate(e.SQL, cat, e.Views)
			if rej != nil {
				refused = append(refused, e.ID+": validator "+rej.Code)
				continue
			}
			kinds := []string{}
			for _, o := range askCheckFrame(askExtractFrameWith(e.Question, cat.View("v_plate_sightings") != nil), valid) {
				kinds = append(kinds, o.Kind)
			}
			if len(kinds) > 0 {
				refused = append(refused, e.ID+": "+strings.Join(kinds, ","))
			}
		}
		Expect(refused).To(BeEmpty(), "verified examples that are refused by the checks they are meant to pass")
	})

	It("reads the direction of data from its own columns: what was uploaded is outgoing, what was downloaded is incoming", func() {
		q := "Which hosts did 923000000101 send the most data to?"
		up := "SELECT host AS label, sum(bytes_up) AS value FROM v_data_sessions WHERE src = '923000000101' AND host IS NOT NULL GROUP BY host ORDER BY value DESC LIMIT 5"
		Expect(check(q, up, "v_data_sessions")).To(BeEmpty())
		Expect(check(q, strings.Replace(up, "bytes_up", "bytes_down", 1), "v_data_sessions")).To(ContainElement("DIRECTION"))
	})
})

var _ = Describe("Ask engine round 9e: a ranking is ordered, and the largest value is a MAX", func() {
	var cat *askCatalogue
	BeforeEach(func() {
		var err error
		cat, err = askGoldenCatalogue()
		Expect(err).NotTo(HaveOccurred())
	})
	check := func(question, sql string, views ...string) []string {
		valid, rej := askValidate(sql, cat, views)
		Expect(rej).To(BeNil(), sql)
		out := []string{}
		for _, o := range askCheckFrame(askExtractFrame(question), valid) {
			out = append(out, o.Kind)
		}
		return out
	}

	It("takes the extreme value of a column with MAX or MIN, and still asks for the ranking when the answer is the winner", func() {
		value := "What is the largest single debit of the account PK00SYNT0000000000002010?"
		Expect(check(value, "SELECT max(amount) AS largest FROM v_bank_transactions WHERE account = 'PK00SYNT0000000000002010' AND direction = 'debit'", "v_bank_transactions")).To(BeEmpty())
		Expect(check(value, "SELECT amount FROM v_bank_transactions WHERE account = 'PK00SYNT0000000000002010' AND direction = 'debit' LIMIT 5", "v_bank_transactions")).To(ContainElement("ORDER"))
		which := "Which plate was seen the most?"
		Expect(check(which, "SELECT max(sightings) AS value FROM v_plate_visits", "v_plate_visits")).To(ContainElement("ORDER"), "the winner is a label, and MAX alone drops it")
		Expect(check(which, "SELECT plate_norm AS label, sum(sightings) AS value FROM v_plate_visits GROUP BY plate_norm ORDER BY value DESC LIMIT 1", "v_plate_visits")).To(BeEmpty())
	})
})

var _ = Describe("Ask engine round 9e: the view that can carry a kind of event", func() {
	var cat *askCatalogue
	BeforeEach(func() {
		var err error
		cat, err = askGoldenCatalogue()
		Expect(err).NotTo(HaveOccurred())
	})

	It("says which view has the event_type column when the query reads the contact pairs for one kind of message", func() {
		q := "Which number did 923000001006 send the most text messages to?"
		pairs := "SELECT dst AS label, outgoing AS value FROM v_contact_pairs WHERE src = '923000001006' AND NOT is_service ORDER BY outgoing DESC LIMIT 5"
		valid, rej := askValidate(pairs, cat, []string{"v_comm_events", "v_contact_pairs"})
		Expect(rej).To(BeNil())
		message := ""
		for _, o := range askCheckFrame(askExtractFrame(q), valid) {
			if o.Kind == "EVENT_TYPE" {
				message = o.Message
			}
		}
		Expect(message).To(ContainSubstring("v_contact_pairs counts every kind of event together and has no event_type column"))
		Expect(message).To(ContainSubstring("read v_comm_events"))
		// the view that has the column needs only the plain instruction
		events := "SELECT dst AS label, count(*) AS value FROM v_comm_events WHERE src = '923000001006' AND direction = 'outgoing' GROUP BY dst ORDER BY value DESC LIMIT 5"
		valid, rej = askValidate(events, cat, []string{"v_comm_events", "v_contact_pairs"})
		Expect(rej).To(BeNil())
		for _, o := range askCheckFrame(askExtractFrame(q), valid) {
			if o.Kind == "EVENT_TYPE" {
				Expect(o.Message).NotTo(ContainSubstring("counts every kind of event together"))
			}
		}
		right := "SELECT dst AS label, count(*) AS value FROM v_comm_events WHERE src = '923000001006' AND event_type = 'sms' AND direction = 'outgoing' GROUP BY dst ORDER BY value DESC LIMIT 5"
		valid, rej = askValidate(right, cat, []string{"v_comm_events", "v_contact_pairs"})
		Expect(rej).To(BeNil())
		Expect(askCheckFrame(askExtractFrame(q), valid)).To(BeEmpty())
	})

	It("says where a column lives when the query asks the wrong view for it, and says nothing when no other offered view has it", func() {
		offered := []string{"v_comm_events", "v_contact_pairs"}
		_, rej := askValidate("SELECT dst, event_type FROM v_contact_pairs WHERE src = '923000001006'", cat, offered)
		Expect(rej).NotTo(BeNil())
		Expect(rej.Code).To(Equal("UNKNOWN_COLUMN"))
		Expect(rej.Message).To(ContainSubstring("event_type is a column of v_comm_events, which this query does not read"))
		_, rej = askValidate("SELECT nonsense FROM v_contact_pairs", cat, offered)
		Expect(rej).NotTo(BeNil())
		Expect(rej.Message).NotTo(ContainSubstring("is a column of"))
	})
})

var _ = Describe("Ask engine round 9e: a tie for the top place", func() {
	var (
		cat       *askCatalogue
		savedExec = askExec
	)
	const ranking = "SELECT plate_norm AS label, sum(sightings) AS value FROM v_plate_visits GROUP BY plate_norm ORDER BY value DESC LIMIT 1"
	tied := func() *govSQLResult {
		return &govSQLResult{Columns: []string{"label", "value"}, Texts: []bool{true, false}, Rows: [][]any{{"SYNT2905", int64(50)}, {"SYNT4024", int64(50)}, {"SYNT0001", int64(41)}}}
	}
	alone := func() *govSQLResult {
		return &govSQLResult{Columns: []string{"label", "value"}, Texts: []bool{true, false}, Rows: [][]any{{"SYNT4024", int64(50)}, {"SYNT0001", int64(41)}}}
	}
	one := func() *govSQLResult {
		return &govSQLResult{Columns: []string{"label", "value"}, Texts: []bool{true, false}, Rows: [][]any{{"SYNT4024", int64(50)}}}
	}
	BeforeEach(func() {
		var err error
		cat, err = askGoldenCatalogue()
		Expect(err).NotTo(HaveOccurred())
	})
	AfterEach(func() { askExec = savedExec })

	It("lists every result that shares the top place of a ranking that kept one row, from the same query with room for fifty more", func() {
		valid, rej := askValidate(ranking, cat, []string{"v_plate_visits"})
		Expect(rej).To(BeNil())
		var asked []string
		askExec = func(_ context.Context, _ *pgxpool.Pool, _, _ string, v *askValidated, _ askLimits) (*govSQLResult, error) {
			asked = append(asked, v.SQL)
			return tied(), nil
		}
		got, n := askExtendTies(context.Background(), nil, "t", "c", cat, []string{"v_plate_visits"}, askExtractFrame("Which plate was seen the most?"), valid, one(), askDefaultLimits())
		Expect(n).To(Equal(2))
		Expect(got.Rows).To(HaveLen(2))
		Expect([]any{got.Rows[0][0], got.Rows[1][0]}).To(ConsistOf("SYNT2905", "SYNT4024"))
		Expect(asked).To(HaveLen(1))
		Expect(asked[0]).To(HaveSuffix("LIMIT 51"))
		Expect(asked[0]).To(ContainSubstring("ORDER BY value DESC"))
	})

	It("leaves a ranking with a single winner as it is, and runs nothing for a ranking that already shows several rows", func() {
		valid, _ := askValidate(ranking, cat, []string{"v_plate_visits"})
		askExec = func(context.Context, *pgxpool.Pool, string, string, *askValidated, askLimits) (*govSQLResult, error) {
			return alone(), nil
		}
		first := one()
		got, n := askExtendTies(context.Background(), nil, "t", "c", cat, []string{"v_plate_visits"}, askExtractFrame("Which plate was seen the most?"), valid, first, askDefaultLimits())
		Expect(n).To(BeZero())
		Expect(got).To(BeIdenticalTo(first))

		askExec = func(context.Context, *pgxpool.Pool, string, string, *askValidated, askLimits) (*govSQLResult, error) {
			Fail("a ranking that shows several rows, or that is no ranking, runs nothing more")
			return nil, nil
		}
		five, _ := askValidate(strings.Replace(ranking, "LIMIT 1", "LIMIT 5", 1), cat, []string{"v_plate_visits"})
		_, n = askExtendTies(context.Background(), nil, "t", "c", cat, []string{"v_plate_visits"}, askExtractFrame("Which plate was seen the most?"), five, tied(), askDefaultLimits())
		Expect(n).To(BeZero())
		_, n = askExtendTies(context.Background(), nil, "t", "c", cat, []string{"v_plate_visits"}, askExtractFrame("How many sightings has the camera CAM-01 recorded?"), valid, one(), askDefaultLimits())
		Expect(n).To(BeZero(), "a question that ranks nothing")
	})

	DescribeTable("finds the column the ranking is sorted by",
		func(sql string, columns []string, want int, found bool) {
			valid, rej := askValidate(sql, cat, nil)
			Expect(rej).To(BeNil(), sql)
			got, ok := askRankKey(valid, columns)
			Expect(ok).To(Equal(found), sql)
			if found {
				Expect(got).To(Equal(want), sql)
			}
		},
		Entry("an output name", ranking, []string{"label", "value"}, 1, true),
		Entry("the column an output is made of", "SELECT dst AS label, events AS value FROM v_contact_pairs WHERE src = '923000000101' ORDER BY events DESC LIMIT 1", []string{"label", "value"}, 1, true),
		Entry("a position", "SELECT plate_norm AS label, sum(sightings) AS value FROM v_plate_visits GROUP BY plate_norm ORDER BY 2 DESC LIMIT 1", []string{"label", "value"}, 1, true),
		Entry("an expression an output is made of", "SELECT plate_norm AS label, count(*) AS value FROM v_plate_sightings GROUP BY plate_norm ORDER BY count(*) DESC LIMIT 1", []string{"label", "value"}, 1, true),
		Entry("a key that is no output", "SELECT plate_norm AS label FROM v_plate_visits GROUP BY plate_norm ORDER BY sum(sightings) DESC LIMIT 1", []string{"label"}, 0, false),
		Entry("a star", "SELECT * FROM v_plate_visits ORDER BY sightings DESC LIMIT 1", []string{"plate_norm", "camera_id", "sightings"}, 0, false),
	)

	It("says the top place is shared, with the noun of the label and only figures of the result", func() {
		valid, _ := askValidate(ranking, cat, []string{"v_plate_visits"})
		r := tied()
		r.Rows = r.Rows[:2]
		Expect(askTieHeadline(r, valid)).To(Equal("2 plates tie for the top place at 50: SYNT2905, SYNT4024."))
		r.Truncated = true
		Expect(askTieHeadline(r, valid)).To(HavePrefix("At least 2 plates tie"))
		numbers, _ := askValidate("SELECT dst AS label, events AS value FROM v_contact_pairs WHERE src = '923000000101' ORDER BY value DESC LIMIT 1", cat, []string{"v_contact_pairs"})
		Expect(askTieHeadline(r, numbers)).To(ContainSubstring("numbers tie"))
	})
})

var _ = Describe("Ask engine round 9e: the run", func() {
	var (
		cat      *askCatalogue
		db       *pgxpool.Pool
		prompts  [][]govSQLTurn
		execBody func(v *askValidated) (*govSQLResult, error)

		savedModel    = askModel
		savedLoader   = askCatalogueLoader
		savedExec     = askExec
		savedProbeFn  = askProbeFn
		savedCaseSet  = askCaseSettingsFn
		savedEvidence = askCollectEvidenceFn
		savedSink     = askAuditSink
		savedCover    = askCoverageRead
	)
	scalar := func(n int64) (*govSQLResult, error) {
		return &govSQLResult{Columns: []string{"n"}, Texts: []bool{false}, Rows: [][]any{{n}}}, nil
	}
	script := func(replies ...string) {
		i := 0
		askModel = func(_ context.Context, _ config, _ string, turns []govSQLTurn) (string, string) {
			prompts = append(prompts, append([]govSQLTurn{}, turns...))
			if i >= len(replies) {
				return "", "SCRIPT_ENDED"
			}
			i++
			return replies[i-1], ""
		}
	}
	reply := func(sql string) string {
		raw, _ := json.Marshal(map[string]any{"answerable": true, "sql": sql})
		return string(raw)
	}
	run := func(q string) (*hybridQueryResponse, AskAuditV2) {
		req := hybridQueryRequest{TenantID: "t", CollectionID: "c", Query: q, RequestClass: forensicrequest.GovernedAnalysis}
		return runAskEngineV2(context.Background(), config{SynthesisModel: "test-model"}, db, req, time.Now())
	}
	BeforeEach(func() {
		var err error
		cat, err = askGoldenCatalogue()
		Expect(err).NotTo(HaveOccurred())
		prompts = nil
		db = &pgxpool.Pool{}
		askCatalogueLoader = func(context.Context, *pgxpool.Pool) (*askCatalogue, error) { return cat, nil }
		execBody = func(*askValidated) (*govSQLResult, error) { return scalar(42) }
		askExec = func(_ context.Context, _ *pgxpool.Pool, _, _ string, v *askValidated, _ askLimits) (*govSQLResult, error) { return execBody(v) }
		askProbeFn = func(context.Context, *pgxpool.Pool, string, string, *askCatalogue, string, askLimits) ([]string, error) { return nil, nil }
		askCaseSettingsFn = func(context.Context, *pgxpool.Pool, string, string) (askCaseSetting, error) { return askCaseSetting{}, nil }
		askCollectEvidenceFn = func(context.Context, *pgxpool.Pool, string, string, *askCatalogue, *askValidated, *govSQLResult, askLimits) *askEvidence { return nil }
		askCoverageCache.entries = nil
		askCoverageRead = func(context.Context, *pgxpool.Pool, string, string) (askCoverage, error) {
			return askCoverage{Files: 3, Typed: 3, AnyComm: true, AnySession: true, AnyUsage: true, AnyVolume: true, AnyPlate: true, AnyTransaction: true}, nil
		}
		askAuditSink = func(context.Context, *pgxpool.Pool, hybridQueryRequest, AskAuditV2, askFrame, string) {}
	})
	AfterEach(func() {
		askModel, askCatalogueLoader, askExec, askProbeFn, askAuditSink, askCoverageRead = savedModel, savedLoader, savedExec, savedProbeFn, savedSink, savedCover
		askCaseSettingsFn = savedCaseSet
		askCollectEvidenceFn = savedEvidence
	})

	It("answers 'who sent money to the account' at the first attempt from its credits, and repairs the debits once", func() {
		const q = "Which accounts or numbers sent money to the account " + r9eAccount + "?"
		credits := "SELECT counterparty, transactions, total_amount FROM v_account_counterparties WHERE account = '" + r9eAccount + "' AND direction = 'credit' ORDER BY total_amount DESC LIMIT 200"
		script(reply(credits))
		_, audit := run(q)
		Expect(audit.State).To(Equal("answered"), audit.Reason)
		Expect(audit.Attempts).To(Equal(1))
		Expect(audit.Repairs).To(BeEmpty())
		Expect(prompts[0][1].Content).To(ContainSubstring("is the receiving end"))

		prompts = nil
		script(reply(strings.Replace(credits, "'credit'", "'debit'", 1)), reply(credits))
		_, audit = run(q)
		Expect(audit.State).To(Equal("answered"), audit.Reason)
		Expect(audit.Repairs).To(Equal([]string{"OBLIGATION:DIRECTION_ROLE"}))
		Expect(prompts[1][len(prompts[1])-1].Content).To(ContainSubstring("receiving end"))
	})

	It("answers the largest single debit at the first attempt with MAX: the example's own query is not refused three times", func() {
		script(reply("SELECT max(amount) AS largest FROM v_bank_transactions WHERE account = 'PK00SYNT0000000000002010' AND direction = 'debit'"))
		_, audit := run("What is the largest single debit of the account PK00SYNT0000000000002010?")
		Expect(audit.State).To(Equal("answered"), audit.Reason)
		Expect(audit.Attempts).To(Equal(1))
		Expect(audit.Repairs).To(BeEmpty())
	})

	It("shows every plate of a tie for the most seen, says so, and records how many", func() {
		script(reply("SELECT plate_norm AS label, sum(sightings) AS value FROM v_plate_visits GROUP BY plate_norm ORDER BY value DESC LIMIT 1"))
		execBody = func(v *askValidated) (*govSQLResult, error) {
			if strings.HasSuffix(v.SQL, "LIMIT 51") {
				return &govSQLResult{Columns: []string{"label", "value"}, Texts: []bool{true, false}, Rows: [][]any{{"SYNT2905", int64(50)}, {"SYNT4024", int64(50)}, {"SYNT0001", int64(41)}}}, nil
			}
			return &govSQLResult{Columns: []string{"label", "value"}, Texts: []bool{true, false}, Rows: [][]any{{"SYNT4024", int64(50)}}}, nil
		}
		resp, audit := run("Which plate was seen the most?")
		Expect(audit.State).To(Equal("answered"), audit.Reason)
		Expect(audit.Tied).To(Equal(2))
		Expect(audit.Rows).To(Equal(2))
		Expect(resp.Answer["answer"]).To(Equal("2 plates tie for the top place at 50: SYNT2905, SYNT4024."))
		Expect(fmt.Sprint(resp.Enterprise["limitations"])).To(ContainSubstring("2 results share the top place with the same figure"))
		Expect(resp.Enterprise["row_count"]).To(Equal(2))
	})

	It("takes the byte-order mark off the first heading the upload reader detects", func() {
		bom := string(rune(0xFEFF))
		headers, err := validateDetectedHeaders([]string{bom + "UFDR start time", "Msisdn"})
		Expect(err).NotTo(HaveOccurred())
		Expect(headers).To(Equal([]string{"UFDR start time", "Msisdn"}))
	})

	It("tells a call that ran out of time from a service that is not there", func() {
		Expect(askModelTimedOut("TIMEOUT_OR_UNAVAILABLE", true, 0)).To(BeTrue(), "the call ran into its deadline")
		Expect(askModelTimedOut("TIMEOUT_OR_UNAVAILABLE", false, 30*time.Second)).To(BeTrue(), "the call ran for longer than the shortest call the engine starts")
		Expect(askModelTimedOut("TIMEOUT_OR_UNAVAILABLE", false, 5*time.Millisecond)).To(BeFalse(), "a connection refused at once is a decline")
		Expect(askModelTimedOut("INCOMPLETE_COMPLETION", true, time.Minute)).To(BeFalse(), "a runaway completion is not a timeout")
		audit := AskAuditV2{}
		resp := askTimedOut(hybridQueryRequest{TenantID: "t", CollectionID: "c", Query: "x"}, &audit, "the model did not answer in the time allowed")
		Expect(audit.TimedOut).To(BeTrue())
		Expect(audit.Reason).To(HavePrefix("time: "))
		Expect(resp.Answer["clarification"]).To(ContainSubstring("This says nothing about what the records hold"))
		Expect(resp.Answer["clarification"]).NotTo(ContainSubstring("No record"))
	})
})

var _ = Describe("Ask engine round 9e: the reading of a word that names a value of a column", func() {
	It("says which value the files were counted by, and says nothing when the query counts them another way", func() {
		cat, err := askGoldenCatalogue()
		Expect(err).NotTo(HaveOccurred())
		f := askExtractFrame("How many text files are in this case?")
		f.Domains = map[string][]string{"v_evidence_inventory.modality": {"audio", "document", "image", "structured_records", "tabular", "text", "video"}}
		byKind, rej := askValidate("SELECT count(*) AS n FROM v_evidence_inventory WHERE modality = 'text'", cat, nil)
		Expect(rej).To(BeNil())
		notes := askDomainNotes(f, byKind)
		Expect(notes).To(HaveLen(1))
		Expect(notes[0]).To(ContainSubstring("modality = 'text'"))
		Expect(notes[0]).To(ContainSubstring("ending of a file's name"))
		byEnding, rej := askValidate("SELECT count(*) AS n FROM v_evidence_inventory WHERE extension = 'txt'", cat, nil)
		Expect(rej).To(BeNil())
		Expect(askDomainNotes(f, byEnding)).To(BeEmpty())
	})
})
