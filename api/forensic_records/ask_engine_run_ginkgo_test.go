package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudler/LocalAI/pkg/forensicrequest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The frame of a question, the obligations, the derivation in plain words, and the run with a scripted model and a stubbed database: the whole path of
// roadmap 6.8.2 without a network. The executor and the probe are tested against a real database in ask_engine_exec_db_ginkgo_test.go.

var _ = Describe("Ask engine frame", func() {
	It("writes every number in one canonical form", func() {
		for _, written := range []string{"0300 1234567", "0300-1234567", "03001234567", "+92 300 1234567", "923001234567", "300 1234567"} {
			f := askExtractFrame("How many calls did " + written + " make?")
			Expect(f.Numbers).To(Equal([]string{"923001234567"}), written)
			Expect(f.Question).To(ContainSubstring("923001234567"), written)
		}
	})

	It("finds devices, addresses, cells, dates, months, years, limits and hops", func() {
		f := askExtractFrame("Between 2026-03-01 and 2026-03-15 which handset 356938035643809 and address 10.20.30.40 and cell 4101:2a:7f were used by 923001234567?")
		Expect(f.IMEIs).To(Equal([]string{"356938035643809"}))
		Expect(f.IPs).To(Equal([]string{"10.20.30.40"}))
		Expect(f.Cells).To(Equal([]string{"4101:2a:7f"}))
		Expect(f.Dates).To(Equal([]string{"2026-03-01", "2026-03-15"}))
		Expect(f.Numbers).To(Equal([]string{"923001234567"}))

		f = askExtractFrame("How many calls were there in March 2026?")
		Expect(f.Months).To(Equal([]askMonth{{Year: 2026, Month: 3}}))
		Expect(f.Years).To(BeEmpty())
		Expect(f.Cues["count"]).To(BeTrue())

		f = askExtractFrame("Who made calls in 2025?")
		Expect(f.Years).To(Equal([]int{2025}))

		f = askExtractFrame("Give the top five contacts of 923000000101")
		Expect(f.Limit).To(Equal(5))
		f = askExtractFrame("Which 10 numbers called 923000000101 most?")
		Expect(f.Limit).To(Equal(10))
		f = askExtractFrame("Who does 923000000101 talk to most?")
		Expect(f.Limit).To(Equal(0))
		Expect(f.Cues["rank"]).To(BeTrue())

		f = askExtractFrame("How are 923000000101 and 923000000102 connected within three hops?")
		Expect(f.Hops).To(Equal(3))
		Expect(f.Cues["path"]).To(BeTrue())
		Expect(f.Numbers).To(HaveLen(2))
	})

	It("does not take the word may for a month unless it is one", func() {
		Expect(askExtractFrame("Who may have called 923000000101?").Months).To(BeEmpty())
		Expect(askExtractFrame("Calls in May 2026").Months).To(Equal([]askMonth{{Year: 2026, Month: 5}}))
		Expect(askExtractFrame("Calls during May").Months).To(Equal([]askMonth{{Month: 5}}))
	})

	It("reads the cues of direction, kind, night, count and rank", func() {
		f := askExtractFrame("How many incoming SMS did 923000000101 get at night?")
		for _, cue := range []string{"incoming", "sms", "night", "count"} {
			Expect(f.Cues[cue]).To(BeTrue(), cue)
		}
		Expect(askExtractFrame("Which numbers did 923000000101 dial?").Cues["outgoing"]).To(BeTrue())
		Expect(askExtractFrame("Calls made by 923000000101").Cues["outgoing"]).To(BeFalse())
	})

	It("flags a relative time, which cannot be resolved without a reference day", func() {
		for _, q := range []string{"calls last week", "What happened yesterday?", "calls in the past 30 days", "recent contacts of 923000000101", "this month"} {
			Expect(askExtractFrame(q).Relative).NotTo(BeEmpty(), q)
		}
		Expect(askExtractFrame("calls in March 2026").Relative).To(BeEmpty())
	})
})

var _ = Describe("Ask engine obligations", func() {
	var cat *askCatalogue
	BeforeEach(func() {
		var err error
		cat, err = askGoldenCatalogue()
		Expect(err).NotTo(HaveOccurred())
	})

	check := func(question, sql string) []string {
		valid, rej := askValidate(sql, cat, nil)
		Expect(rej).To(BeNil(), sql)
		var kinds []string
		for _, o := range askCheckFrame(askExtractFrame(question), valid) {
			kinds = append(kinds, o.Kind)
		}
		return kinds
	}

	passes := []struct{ q, sql string }{
		{"How many SMS did 923000000101 receive?", "SELECT count(*) AS n FROM v_comm_events WHERE src = '923000000101' AND event_type = 'sms' AND direction = 'incoming'"},
		{"Who does 923000000101 talk to most? Give the top five.", "SELECT dst AS label, events AS value FROM v_contact_pairs WHERE src = '923000000101' AND NOT is_service ORDER BY events DESC LIMIT 5"},
		{"How many calls did 0300 0000101 make in March 2026?", "SELECT count(*) AS n FROM v_comm_events WHERE src = '923000000101' AND event_type = 'voice' AND direction = 'outgoing' AND start_utc >= '2026-03-01' AND start_utc < '2026-04-01'"},
		{"When was the first call of 923000000101?", "SELECT min(start_utc) AS first_call FROM v_comm_events WHERE src = '923000000101'"},
		{"How many calls did 923000000101 have at night?", "SELECT count(*) AS n FROM v_comm_events WHERE src = '923000000101' AND EXTRACT(HOUR FROM start_utc) BETWEEN 0 AND 5"},
		{"How many different numbers did 923000000101 call?", "SELECT count(DISTINCT dst) AS n FROM v_comm_events WHERE src = '923000000101'"},
		{"Who do 923000000101 and 923000000102 both contact, leaving out services and numbers that everyone contacts?", "SELECT x.dst AS number FROM v_contact_pairs x JOIN v_contact_pairs y ON y.dst = x.dst WHERE x.src = '923000000101' AND y.src = '923000000102' AND NOT x.is_service AND NOT x.is_hub LIMIT 50"},
		{"Which handset 356938035643809 did 923000000101 use?", "SELECT src, imei, first_seen FROM v_device_pairings WHERE src = '923000000101' AND imei = '356938035643809'"},
	}
	for _, p := range passes {
		p := p
		It("accepts a query that does what the question asks: "+p.q, func() {
			Expect(check(p.q, p.sql)).To(BeEmpty())
		})
	}

	fails := []struct{ name, q, sql, kind string }{
		{"a dropped identifier", "How many calls did 923000000101 make?", "SELECT count(*) AS n FROM v_comm_events WHERE direction = 'outgoing'", "IDENTIFIER"},
		{"the number of an example copied into the query", "How many calls did 923000000101 make?", "SELECT count(*) AS n FROM v_comm_events WHERE src = '923000000101' OR src = '923000000999'", "FOREIGN_IDENTIFIER"},
		{"another subscriber instead of the one asked", "How many events does 923000000101 have?", "SELECT count(*) AS n FROM v_comm_events WHERE src = '923000000102'", "IDENTIFIER"},
		{"a dropped date", "How many calls did 923000000101 have on 2026-03-05?", "SELECT count(*) AS n FROM v_comm_events WHERE src = '923000000101'", "DATE"},
		{"a dropped month", "How many calls did 923000000101 have in March 2026?", "SELECT count(*) AS n FROM v_comm_events WHERE src = '923000000101'", "MONTH"},
		{"another month", "How many calls did 923000000101 have in March 2026?", "SELECT count(*) AS n FROM v_comm_events WHERE src = '923000000101' AND start_utc >= '2026-04-01' AND start_utc < '2026-05-01'", "MONTH"},
		{"a wrong limit", "Give the top five contacts of 923000000101", "SELECT dst AS label, events AS value FROM v_contact_pairs WHERE src = '923000000101' ORDER BY events DESC LIMIT 10", "LIMIT"},
		{"no ordering for a ranking", "Who does 923000000101 talk to most?", "SELECT dst AS label, events AS value FROM v_contact_pairs WHERE src = '923000000101' LIMIT 5", "ORDER"},
		{"the wrong direction dropped", "How many incoming calls did 923000000101 get?", "SELECT count(*) AS n FROM v_comm_events WHERE src = '923000000101'", "DIRECTION"},
		{"the kind of event dropped", "How many SMS did 923000000101 send?", "SELECT count(*) AS n FROM v_comm_events WHERE src = '923000000101' AND direction = 'outgoing'", "EVENT_TYPE"},
		{"the night dropped", "How many calls did 923000000101 have at night?", "SELECT count(*) AS n FROM v_comm_events WHERE src = '923000000101'", "TIME_OF_DAY"},
		{"no count", "How many calls did 923000000101 have?", "SELECT dst FROM v_comm_events WHERE src = '923000000101' LIMIT 20", "AGGREGATE"},
		{"services not left out", "Who does 923000000101 contact, leaving out services?", "SELECT dst FROM v_contact_pairs WHERE src = '923000000101'", "SERVICES"},
		{"hubs not left out", "Who does 923000000101 contact, not through hubs or numbers everyone contacts?", "SELECT dst FROM v_contact_pairs WHERE src = '923000000101'", "HUBS"},
		{"a connection question without a recursion", "How are 923000000101 and 923000000102 connected within three hops?", "SELECT dst FROM v_contact_pairs WHERE src = '923000000101' AND dst = '923000000102'", "RECURSION"},
	}
	for _, p := range fails {
		p := p
		It("refuses "+p.name, func() {
			Expect(check(p.q, p.sql)).To(ContainElement(p.kind))
		})
	}

	It("holds the hop bound to the hops asked", func() {
		q := "How are 923000000101 and 923000000102 connected within three hops?"
		walk := func(bound string) string {
			return "WITH RECURSIVE walk AS (SELECT src, dst, 1 AS depth, ARRAY[src, dst] AS path FROM v_contact_pairs WHERE src = '923000000101' " +
				"UNION ALL SELECT w.src, e.dst, w.depth + 1, w.path || e.dst FROM walk w JOIN v_contact_pairs e ON e.src = w.dst WHERE w.depth " + bound + " AND NOT e.dst = ANY (w.path)) " +
				"SELECT src, dst FROM walk WHERE dst = '923000000102' LIMIT 20"
		}
		Expect(check(q, walk("< 3"))).To(BeEmpty())
		Expect(check(q, walk("<= 3"))).To(ContainElement("HOPS"))
	})

	It("holds a count to one value and an earliest to one row", func() {
		valid, rej := askValidate("SELECT dst FROM v_comm_events WHERE src = '923000000101' LIMIT 20", cat, nil)
		Expect(rej).To(BeNil())
		bad := askCheckResult(askExtractFrame("How many calls did 923000000101 have?"), valid, []string{"dst"}, 20)
		Expect(bad).NotTo(BeNil())
		Expect(bad.Kind).To(Equal("SHAPE"))
		Expect(askCheckResult(askExtractFrame("How many calls did 923000000101 have?"), valid, []string{"n"}, 1)).To(BeNil())
		Expect(askCheckResult(askExtractFrame("How many calls did each number have?"), valid, []string{"src", "n"}, 12)).To(BeNil())
	})
})

var _ = Describe("Ask engine derivation in plain words", func() {
	It("restates the query from its own parse tree", func() {
		cat, err := askGoldenCatalogue()
		Expect(err).NotTo(HaveOccurred())
		valid, rej := askValidate("SELECT dst AS label, events AS value FROM v_contact_pairs WHERE src = '923000000101' AND NOT is_service AND events > 3 ORDER BY events DESC LIMIT 5", cat, nil)
		Expect(rej).To(BeNil())
		text := strings.Join(askDescribe(valid, cat).Sentences(), " ")
		Expect(text).To(ContainSubstring("v_contact_pairs"))
		Expect(text).To(ContainSubstring("src = '923000000101'"))
		Expect(text).To(ContainSubstring("NOT is_service"))
		Expect(text).To(ContainSubstring("events > 3"))
		Expect(text).To(ContainSubstring("Ordered by events DESC"))
		Expect(text).To(ContainSubstring("first 5 rows"))
	})

	It("says a recursion walks, how far, and that times are on the case clock", func() {
		cat, _ := askGoldenCatalogue()
		examples, err := askExamples()
		Expect(err).NotTo(HaveOccurred())
		var path askExample
		for _, e := range examples {
			if e.ID == "connection-path" {
				path = e
			}
		}
		valid, rej := askValidate(path.SQL, cat, nil)
		Expect(rej).To(BeNil())
		text := strings.Join(askDescribe(valid, cat).Sentences(), " ")
		Expect(text).To(ContainSubstring("up to 3 hops"))
		valid, _ = askValidate("SELECT count(*) AS n FROM v_comm_events WHERE start_utc >= '2026-03-01' AND start_utc < '2026-04-01'", cat, nil)
		Expect(strings.Join(askDescribe(valid, cat).Sentences(), " ")).To(ContainSubstring("case clock"))
	})
})

var _ = Describe("Ask engine run", func() {
	var (
		cat      *askCatalogue
		cfg      config
		db       *pgxpool.Pool
		prompts  [][]govSQLTurn
		execs    []string
		execBody func(v *askValidated) (*govSQLResult, error)
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
	request := func(q string) hybridQueryRequest {
		return hybridQueryRequest{TenantID: "t", CollectionID: "c", Query: q, RequestClass: forensicrequest.GovernedAnalysis}
	}
	run := func(q string) (*hybridQueryResponse, AskAuditV2) {
		return runAskEngineV2(context.Background(), cfg, db, request(q), time.Now())
	}

	var (
		savedModel   askModelFunc
		savedLoader  = askCatalogueLoader
		savedExec    = askExec
		savedProbeFn = askProbeFn
	)
	BeforeEach(func() {
		var err error
		cat, err = askGoldenCatalogue()
		Expect(err).NotTo(HaveOccurred())
		savedModel = askModel
		prompts, execs = nil, nil
		db = &pgxpool.Pool{}
		cfg = config{SynthesisModel: "test-model"}
		askCatalogueLoader = func(context.Context, *pgxpool.Pool) (*askCatalogue, error) { return cat, nil }
		execBody = func(*askValidated) (*govSQLResult, error) { return scalar(42) }
		askExec = func(_ context.Context, _ *pgxpool.Pool, tenant, collection string, v *askValidated, _ askLimits) (*govSQLResult, error) {
			Expect(tenant).To(Equal("t"))
			Expect(collection).To(Equal("c"))
			execs = append(execs, v.SQL)
			return execBody(v)
		}
		askProbeFn = func(context.Context, *pgxpool.Pool, string, string, *askCatalogue, string, askLimits) ([]string, error) {
			return nil, nil
		}
	})
	AfterEach(func() {
		askModel, askCatalogueLoader, askExec, askProbeFn = savedModel, savedLoader, savedExec, savedProbeFn
	})

	const q = "How many incoming SMS did 923000000101 receive?"
	const good = "SELECT count(*) AS n FROM v_comm_events WHERE src = '923000000101' AND event_type = 'sms' AND direction = 'incoming'"

	It("answers when the first draft passes every check, and builds the answer by code", func() {
		script(reply(good))
		resp, audit := run(q)
		Expect(audit.State).To(Equal("answered"))
		Expect(audit.Attempts).To(Equal(1))
		Expect(audit.Views).To(ContainElement("v_comm_events"))
		Expect(audit.Shape).To(Equal("scalar"))
		Expect(resp).NotTo(BeNil())
		Expect(resp.Policy).To(Equal("ask_engine_v2"))
		Expect(resp.Answer["answer"]).To(Equal("N: 42."))
		Expect(resp.Answer["grounding"]).To(Equal("ASK_ENGINE_V2"))
		Expect(resp.Enterprise["result_shape"]).To(Equal("scalar"))
		derivation := resp.Enterprise["derivation"].(map[string]any)
		Expect(derivation["sql"]).To(ContainSubstring("v_comm_events"))
		Expect(strings.Join(derivation["steps"].([]string), " ")).To(ContainSubstring("event_type = 'sms'"))
		Expect(derivation["conditions_checked"]).NotTo(BeEmpty())
		Expect(execs).To(HaveLen(1))
	})

	It("shows the model the canonical number, the hints, the fixed rules first, and no case data", func() {
		script(reply(good))
		run("How many incoming SMS did 0300 0000101 receive?")
		Expect(prompts).To(HaveLen(1))
		system, user := prompts[0][0].Content, prompts[0][1].Content
		Expect(system).To(HavePrefix(askPromptRules))
		Expect(user).To(ContainSubstring("923000000101"))
		Expect(user).NotTo(ContainSubstring("0300 0000101"))
		Expect(user).To(ContainSubstring("Incoming events only."))
		Expect(user).To(ContainSubstring("Event type sms."))
	})

	It("repairs a draft that leaves out part of the question, with the reason, and answers on the second", func() {
		script(reply("SELECT count(*) AS n FROM v_comm_events WHERE src = '923000000101' AND event_type = 'sms'"), reply(good))
		resp, audit := run(q)
		Expect(audit.State).To(Equal("answered"))
		Expect(audit.Attempts).To(Equal(2))
		Expect(audit.Repairs).To(Equal([]string{"OBLIGATION:DIRECTION"}))
		Expect(prompts[1][len(prompts[1])-1].Content).To(ContainSubstring("direction = 'incoming'"))
		Expect(resp.Answer["answer"]).To(Equal("N: 42."))
		Expect(execs).To(HaveLen(1), "the draft that failed an obligation was never run")
	})

	It("repairs a rejection of the validator, then a database error, within two repairs", func() {
		script(reply("SELECT count(*) FROM base_table"), reply("SELECT count(*) AS n FROM v_comm_events WHERE nope = 1"), reply(good))
		_, audit := run(q)
		Expect(audit.State).To(Equal("answered"))
		Expect(audit.Attempts).To(Equal(3))
		Expect(audit.Repairs).To(HaveLen(2))
		Expect(audit.Repairs[0]).To(HavePrefix("VALIDATOR:"))
	})

	It("abstains, with the reason and no number, when the last draft still leaves out part of the question", func() {
		bad := reply("SELECT count(*) AS n FROM v_comm_events WHERE src = '923000000101'")
		script(bad, bad, bad)
		resp, audit := run(q)
		Expect(audit.State).To(Equal("abstained"))
		Expect(audit.Attempts).To(Equal(3))
		Expect(resp.Answer["result_state"]).To(Equal("CLARIFICATION_REQUIRED"))
		Expect(resp.Answer["clarification"]).To(ContainSubstring("incoming"))
		Expect(resp.Answer).NotTo(HaveKey("answer"))
		Expect(execs).To(BeEmpty())
	})

	It("never lets a model copy the number of an example into its query", func() {
		bad := reply("SELECT count(*) AS n FROM v_comm_events WHERE src = '923000000101' AND event_type = 'sms' AND direction = 'incoming' AND dst <> '923000000999'")
		script(bad, bad, bad)
		_, audit := run(q)
		Expect(audit.State).To(Equal("abstained"))
		Expect(audit.Repairs).To(ContainElement("OBLIGATION:FOREIGN_IDENTIFIER"))
		Expect(execs).To(BeEmpty())
	})

	It("declines, without calling the model, a question in a relative time and a question no view fits", func() {
		script(reply(good))
		_, audit := run("How many SMS did 923000000101 receive last week?")
		Expect(audit.State).To(Equal("declined"))
		Expect(audit.Reason).To(ContainSubstring("relative time"))
		_, audit = run("What is the capital of France?")
		Expect(audit.State).To(Equal("declined"))
		Expect(prompts).To(BeEmpty())
	})

	It("declines when it is not a data question, is structured, has no scope or has no database", func() {
		script(reply(good))
		r := request(q)
		r.RequestClass = forensicrequest.GeneralDomainKnowledge
		r.Query = "What is a call detail record?"
		_, audit := runAskEngineV2(context.Background(), cfg, db, r, time.Now())
		Expect(audit.State).To(Equal("declined"))
		r = request(q)
		r.Template = "cdr_summary"
		_, audit = runAskEngineV2(context.Background(), cfg, db, r, time.Now())
		Expect(audit.Reason).To(Equal("an explicit operation was requested"))
		r = request(q)
		r.CollectionID = ""
		_, audit = runAskEngineV2(context.Background(), cfg, db, r, time.Now())
		Expect(audit.Reason).To(Equal("no authorised scope"))
		_, audit = runAskEngineV2(context.Background(), cfg, nil, request(q), time.Now())
		Expect(audit.Reason).To(Equal("no database"))
		Expect(prompts).To(BeEmpty())
	})

	It("declines when the analyst views are not installed, and when the model fails or is not answerable", func() {
		askCatalogueLoader = func(context.Context, *pgxpool.Pool) (*askCatalogue, error) { return nil, context.DeadlineExceeded }
		script(reply(good))
		_, audit := run(q)
		Expect(audit.Reason).To(HavePrefix("the analyst views are not available"))
		askCatalogueLoader = func(context.Context, *pgxpool.Pool) (*askCatalogue, error) { return cat, nil }
		askModel = func(context.Context, config, string, []govSQLTurn) (string, string) {
			return "", "TIMEOUT_OR_UNAVAILABLE"
		}
		_, audit = run(q)
		Expect(audit.Reason).To(Equal("model: TIMEOUT_OR_UNAVAILABLE"))
		script(`{"answerable": false, "sql": ""}`)
		_, audit = run(q)
		Expect(audit.State).To(Equal("declined"))
		Expect(audit.Reason).To(ContainSubstring("cannot answer"))
	})

	It("says what a result is when it is a list, a graph or nothing", func() {
		const ranked = "Who does 923000000101 talk to most? Give the top five."
		script(reply("SELECT dst AS label, events AS value FROM v_contact_pairs WHERE src = '923000000101' AND NOT is_service ORDER BY events DESC LIMIT 5"))
		execBody = func(*askValidated) (*govSQLResult, error) {
			return &govSQLResult{Columns: []string{"label", "value"}, Texts: []bool{true, false}, Rows: [][]any{{"923000000201", int64(14)}, {"923000000202", int64(9)}}}, nil
		}
		resp, audit := run(ranked)
		Expect(audit.Shape).To(Equal("ranked"))
		Expect(resp.Answer["answer"]).To(Equal("2 results: 923000000201 (14), 923000000202 (9)."))
		Expect(resp.Enterprise["row_count"]).To(Equal(2))
	})

	It("reads an empty result: nothing anywhere is a true absence; the identifier elsewhere is a repair; the right place is a true empty", func() {
		empty := func(*askValidated) (*govSQLResult, error) {
			return &govSQLResult{Columns: []string{"n"}, Texts: []bool{false}, Rows: [][]any{}}, nil
		}
		execBody = empty
		script(reply("SELECT src, dst FROM v_comm_events WHERE src = '923000000101' AND event_type = 'sms' AND direction = 'incoming' LIMIT 20"))
		resp, audit := run("Which incoming SMS did 923000000101 receive?")
		Expect(audit.State).To(Equal("answered"))
		Expect(resp.Answer["answer"]).To(Equal("No record of this case holds 923000000101 in any number, device, SIM, address or cell column."))

		askProbeFn = func(context.Context, *pgxpool.Pool, string, string, *askCatalogue, string, askLimits) ([]string, error) {
			return []string{"v_device_pairings.imei"}, nil
		}
		script(reply("SELECT src, imei FROM v_device_pairings WHERE src = '356938035643809' LIMIT 20"), reply("SELECT src, imei FROM v_device_pairings WHERE imei = '356938035643809' LIMIT 20"))
		execBody = func(v *askValidated) (*govSQLResult, error) {
			if strings.Contains(v.SQL, "imei = '356938035643809'") {
				return &govSQLResult{Columns: []string{"src", "imei"}, Texts: []bool{true, true}, Rows: [][]any{{"923000000101", "356938035643809"}}}, nil
			}
			return empty(v)
		}
		resp, audit = run("Which numbers used the handset 356938035643809?")
		Expect(audit.Repairs).To(ContainElement("RESULT:EMPTY_WRONG_COLUMN"))
		Expect(audit.State).To(Equal("answered"))
		Expect(resp.Answer["answer"]).To(ContainSubstring("923000000101"))

		askProbeFn = func(context.Context, *pgxpool.Pool, string, string, *askCatalogue, string, askLimits) ([]string, error) {
			return []string{"v_comm_events.src"}, nil
		}
		execBody = empty
		script(reply("SELECT src, dst FROM v_comm_events WHERE src = '923000000101' AND event_type = 'sms' LIMIT 20"))
		resp, audit = run("Which SMS did 923000000101 have?")
		Expect(audit.State).To(Equal("answered"))
		Expect(resp.Answer["answer"]).To(Equal("No row matches what you asked."))
	})

	It("tells a query that is too broad apart from a mistake, and asks to narrow it", func() {
		execBody = func(*askValidated) (*govSQLResult, error) {
			return nil, &govSQLRunError{Code: "TOO_EXPENSIVE", Message: "the query would read far too much data"}
		}
		bad := reply(good)
		script(bad, bad, bad)
		resp, audit := run(q)
		Expect(audit.State).To(Equal("abstained"))
		Expect(audit.Repairs).To(ContainElement("EXECUTION:TOO_EXPENSIVE"))
		Expect(resp.Answer["clarification"]).To(ContainSubstring("too broad"))
	})

	It("answers only when two independent drafts agree, when asked to", func() {
		GinkgoT().Setenv(askV2AgreeEnv, "true")
		script(reply(good), reply(strings.Replace(good, "count(*)", "count(event_id)", 1)))
		_, audit := run(q)
		Expect(audit.State).To(Equal("answered"))
		Expect(audit.Agreement).To(Equal("agreed"))

		n := int64(0)
		execBody = func(*askValidated) (*govSQLResult, error) { n++; return scalar(40 + n) }
		script(reply(good), reply(strings.Replace(good, "count(*)", "count(event_id)", 1)))
		resp, audit := run(q)
		Expect(audit.State).To(Equal("abstained"))
		Expect(audit.Agreement).To(Equal("disagreed"))
		Expect(resp.Answer["clarification"]).To(ContainSubstring("different results"))
	})

	It("compares results by their rows, not their order or column names", func() {
		a := &govSQLResult{Columns: []string{"x"}, Rows: [][]any{{"a"}, {"b"}}}
		b := &govSQLResult{Columns: []string{"y"}, Rows: [][]any{{"b"}, {"a"}}}
		Expect(askSameResult(a, b)).To(BeTrue())
		b.Rows[0] = []any{"c"}
		Expect(askSameResult(a, b)).To(BeFalse())
	})
})

var _ = Describe("Ask engine fallback entry", func() {
	var (
		savedRun = askV2Run
		cfg      config
		db       = &pgxpool.Pool{}
	)
	AfterEach(func() { askV2Run = savedRun })

	core := func(body string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if holder := govSQLHolderFrom(r.Context()); holder != nil {
				held := hybridQueryRequest{TenantID: "t", CollectionID: "c", Query: "q", RequestClass: forensicrequest.GovernedAnalysis}
				holder.req = &held
			}
			w.Header().Set("X-Core", "yes")
			writeJSON(w, http.StatusOK, json.RawMessage(body))
		}
	}
	post := func(h http.HandlerFunc) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h(rec, httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{}")))
		return rec
	}
	const declined = `{"intent":"clarify","answer":{"result_state":"CLARIFICATION_REQUIRED"}}`
	const answered = `{"intent":"records","answer":{"result_state":"COMPLETED","answer":"7"}}`
	engineAnswer := func(context.Context, config, *pgxpool.Pool, hybridQueryRequest, time.Time) (*hybridQueryResponse, AskAuditV2) {
		return &hybridQueryResponse{Answer: map[string]any{"result_state": "COMPLETED", "answer": "from the engine"}, Route: []string{"ask_engine_v2"}}, AskAuditV2{State: "answered"}
	}

	It("does nothing while the switch is off", func() {
		called := false
		askV2Run = func(context.Context, config, *pgxpool.Pool, hybridQueryRequest, time.Time) (*hybridQueryResponse, AskAuditV2) {
			called = true
			return nil, AskAuditV2{}
		}
		rec := post(askEngineFallback(core(declined), cfg, db))
		Expect(rec.Body.String()).To(ContainSubstring("CLARIFICATION_REQUIRED"))
		Expect(called).To(BeFalse())
	})

	It("answers a request the existing path declined, and never replaces an answer the existing path gave", func() {
		GinkgoT().Setenv(askV2Env, "true")
		askV2Run = engineAnswer
		rec := post(askEngineFallback(core(declined), cfg, db))
		Expect(rec.Body.String()).To(ContainSubstring("from the engine"))
		Expect(rec.Header().Get("X-Core")).To(Equal("yes"))
		Expect(rec.Header().Get(askHeaderName)).To(ContainSubstring("state=answered"))
		rec = post(askEngineFallback(core(answered), cfg, db))
		Expect(rec.Body.String()).To(ContainSubstring(`"answer":"7"`))
		Expect(rec.Body.String()).NotTo(ContainSubstring("from the engine"))
	})

	It("sends the existing path's reply unchanged when the engine declines too", func() {
		GinkgoT().Setenv(askV2Env, "true")
		askV2Run = func(context.Context, config, *pgxpool.Pool, hybridQueryRequest, time.Time) (*hybridQueryResponse, AskAuditV2) {
			return nil, AskAuditV2{State: "declined", Reason: "no analyst view fits the question closely enough"}
		}
		rec := post(askEngineFallback(core(declined), cfg, db))
		Expect(rec.Body.String()).To(ContainSubstring("CLARIFICATION_REQUIRED"))
		Expect(rec.Header().Get(askHeaderName)).To(ContainSubstring("state=declined"))
	})

	It("leaves a request alone when it is first (query.go runs it) or is not a POST", func() {
		GinkgoT().Setenv(askV2FirstEnv, "true")
		called := false
		askV2Run = func(context.Context, config, *pgxpool.Pool, hybridQueryRequest, time.Time) (*hybridQueryResponse, AskAuditV2) {
			called = true
			return nil, AskAuditV2{}
		}
		post(askEngineFallback(core(declined), cfg, db))
		Expect(called).To(BeFalse())
	})
})
