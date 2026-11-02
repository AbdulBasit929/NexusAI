package main

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudler/LocalAI/pkg/forensicrequest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The pair of numbers is one predicate (found by the verified bank run of 2026-11-02, VB-PAIR-007): the figure for "the total duration of the voice calls between A and B" came from
// `src = A OR dst = A OR src = B OR (dst = B AND event_type = 'voice')`, which sums every event of either number. The specs hold the shapes a model writes to the rule, on the question
// and the numbers of the bank.
var _ = Describe("Ask engine: the pair of numbers is one predicate", func() {
	const (
		a = "923000001002"
		b = "923000001027"
		// VB-PAIR-007, word for word; the answer the records hold is 12,290 seconds (one number's records, the pair's voice calls)
		q = "What is the total duration of the voice calls between " + a + " and " + b + ", in seconds?"
		// what the live engine wrote: the engine's answer was 57,670
		flat = "SELECT sum(e.duration_s) AS total_duration FROM v_comm_events e WHERE e.src = '" + a + "' OR e.dst = '" + a + "' OR e.src = '" + b + "' OR (e.dst = '" + b + "' AND e.event_type = 'voice')"
		list = "src IN ('" + a + "', '" + b + "') AND dst IN ('" + a + "', '" + b + "')"
		once = "SELECT sum(duration_s) AS total_seconds FROM (SELECT DISTINCT start_utc, duration_s FROM v_comm_events WHERE " + list + " AND event_type = 'voice') t"
	)
	var cat *askCatalogue
	BeforeEach(func() {
		var err error
		cat, err = askGoldenCatalogue()
		Expect(err).NotTo(HaveOccurred())
	})

	obligations := func(question, sql string) map[string]string {
		valid, rej := askValidate(sql, cat, nil)
		Expect(rej).To(BeNil(), sql)
		out := map[string]string{}
		for _, o := range askCheckFrame(askExtractFrame(question), valid) {
			out[o.Kind] = o.Message
		}
		return out
	}

	It("refuses the chain of alternatives the live engine wrote, and says how to write the pair", func() {
		got := obligations(q, flat)
		Expect(got).To(HaveKey("PAIR_SCOPE"))
		Expect(got["PAIR_SCOPE"]).To(ContainSubstring("src IN ('" + a + "', '" + b + "') AND dst IN ('" + a + "', '" + b + "')"))
		Expect(got["PAIR_SCOPE"]).NotTo(ContainSubstring("%!"))
		// the sum of both directions is also the second way to count a call twice
		Expect(got).To(HaveKey("PAIR_DIRECTION"))
	})

	It("accepts the pair as two lists, with the kind of event after it and each call added once", func() {
		Expect(obligations(q, once)).To(BeEmpty())
		// the query the message asks for is a query the validator lets through, with this catalogue
		_, rej := askValidate(once, cat, nil)
		Expect(rej).To(BeNil())
	})

	It("adds a call once when both numbers are subscribers: the pair summed over both directions is refused", func() {
		// a call between two subscribers is in both records, so the sum of the lists' rows is twice the call's seconds
		direct := "SELECT sum(duration_s) AS total_seconds FROM v_comm_events WHERE " + list + " AND event_type = 'voice'"
		got := obligations(q, direct)
		Expect(got).To(HaveKey("PAIR_DIRECTION"))
		Expect(got).NotTo(HaveKey("PAIR_SCOPE"), "the pair is scoped; it is only counted twice")
		Expect(got["PAIR_DIRECTION"]).To(ContainSubstring("SELECT DISTINCT start_utc, duration_s"))
		Expect(got["PAIR_DIRECTION"]).NotTo(ContainSubstring("%!"))
		// the pair read as two alternatives, summed: the same defect
		both := "SELECT sum(duration_s) AS total_seconds FROM v_comm_events WHERE ((src = '" + a + "' AND dst = '" + b + "') OR (src = '" + b + "' AND dst = '" + a + "')) AND event_type = 'voice'"
		got = obligations(q, both)
		Expect(got).To(HaveKey("PAIR_DIRECTION"))
		Expect(got).NotTo(HaveKey("PAIR_SCOPE"))
		// the sum of distinct lengths drops the calls whose length another call has
		Expect(obligations(q, strings.Replace(direct, "sum(duration_s)", "sum(DISTINCT duration_s)", 1))).To(HaveKey("PAIR_DIRECTION"))
	})

	It("takes the records of the one number that is a subscriber as they are: one direction, nothing counted twice", func() {
		// the first number is the subscriber (its records hold the calls in both directions), or the second is; the other has no records of its own
		for _, sql := range []string{
			"SELECT sum(duration_s) AS total_seconds FROM v_comm_events WHERE src = '" + a + "' AND dst = '" + b + "' AND event_type = 'voice'",
			"SELECT sum(duration_s) AS total_seconds FROM v_comm_events WHERE src = '" + b + "' AND dst = '" + a + "' AND event_type = 'voice'",
			"SELECT sum(duration_s) AS total_seconds FROM v_comm_events WHERE dst = '" + b + "' AND src = '" + a + "' AND event_type = 'voice'",
		} {
			Expect(obligations(q, sql)).To(BeEmpty(), sql)
		}
	})

	It("finds a condition that stands in some of the alternatives only: AND binds tighter than OR", func() {
		misgrouped := "SELECT count(DISTINCT start_utc) AS n FROM v_comm_events WHERE (src = '" + a + "' AND dst = '" + b + "') OR (src = '" + b + "' AND dst = '" + a + "') AND event_type = 'voice'"
		got := obligations("How many voice calls were there between "+a+" and "+b+"?", misgrouped)
		Expect(got).To(HaveKey("PAIR_SCOPE"))
		Expect(got["PAIR_SCOPE"]).To(ContainSubstring("event_type = 'voice'"))
		Expect(got["PAIR_SCOPE"]).To(ContainSubstring("only some of the alternatives"))
		// the date in the same place
		dated := "SELECT count(DISTINCT start_utc) AS n FROM v_comm_events WHERE (src = '" + a + "' AND dst = '" + b + "') OR (src = '" + b + "' AND dst = '" + a + "') AND start_utc >= '2026-03-01' AND start_utc < '2026-04-01'"
		Expect(obligations("How many events were there between "+a+" and "+b+" in March 2026?", dated)).To(HaveKey("PAIR_SCOPE"))
		// written once for the whole pair, or in every alternative, the same conditions pass
		hoisted := "SELECT count(DISTINCT start_utc) AS n FROM v_comm_events WHERE ((src = '" + a + "' AND dst = '" + b + "') OR (src = '" + b + "' AND dst = '" + a + "')) AND event_type = 'voice'"
		everywhere := "SELECT count(DISTINCT start_utc) AS n FROM v_comm_events WHERE (src = '" + a + "' AND dst = '" + b + "' AND event_type = 'voice') OR (src = '" + b + "' AND dst = '" + a + "' AND event_type = 'voice')"
		for _, sql := range []string{hoisted, everywhere} {
			Expect(obligations("How many voice calls were there between "+a+" and "+b+"?", sql)).To(BeEmpty(), sql)
		}
	})

	It("limits both ends: one end alone, a list of the other end, and a number that is not of the pair are not the pair", func() {
		question := "How many events are there between " + a + " and " + b + "?"
		for _, sql := range []string{
			"SELECT count(DISTINCT start_utc) AS n FROM v_comm_events WHERE src IN ('" + a + "', '" + b + "')",
			"SELECT count(DISTINCT start_utc) AS n FROM v_comm_events WHERE dst IN ('" + a + "', '" + b + "')",
			"SELECT count(DISTINCT start_utc) AS n FROM v_comm_events WHERE src IN ('" + a + "', '" + b + "') AND dst IS NOT NULL",
			"SELECT count(DISTINCT start_utc) AS n FROM v_comm_events WHERE src = '" + a + "' AND NOT dst = '" + b + "'",
			"SELECT count(DISTINCT start_utc) AS n FROM v_comm_events WHERE src = '" + a + "' AND dst = '" + a + "'",
		} {
			Expect(obligations(question, sql)).To(HaveKey("PAIR_SCOPE"), sql)
		}
		// a chain of alternatives that carries no other condition: each alternative limits one end, and the OR leaves the other end open
		chain := "SELECT count(DISTINCT start_utc) AS n FROM v_comm_events WHERE src = '" + a + "' OR dst = '" + a + "' OR src = '" + b + "' OR dst = '" + b + "'"
		Expect(obligations(question, chain)).To(HaveKey("PAIR_SCOPE"))
		// the pair written as an array test is the pair too
		array := "SELECT count(DISTINCT start_utc) AS n FROM v_comm_events WHERE src = ANY (ARRAY['" + a + "', '" + b + "']) AND dst = ANY (ARRAY['" + a + "', '" + b + "'])"
		Expect(obligations(question, array)).To(BeEmpty())
		// a query that names the numbers through the contact pairs is held to the same rule
		pairs := "SELECT max(events) AS n FROM v_contact_pairs WHERE " + list
		Expect(obligations(question, pairs)).To(BeEmpty())
		Expect(obligations(question, "SELECT sum(events) AS n FROM v_contact_pairs WHERE src IN ('"+a+"', '"+b+"')")).To(HaveKey("PAIR_SCOPE"))
	})

	It("reads the question's own words: a pair question is one that names two numbers and puts them between, with each other, or one with the other", func() {
		perNumber := "SELECT count(*) AS n FROM v_comm_events WHERE src IN ('" + a + "', '" + b + "')"
		for _, question := range []string{
			"How many calls and messages did " + a + " have with " + b + "?",
			"How many calls were there between " + a + " and " + b + "?",
			"How many calls did " + a + " and " + b + " have with each other?",
			"How many calls were there between the numbers " + a + " and " + b + "?",
			"How many calls did " + a + " and " + b + " have with each other, in both directions?",
		} {
			Expect(obligations(question, perNumber)).To(HaveKey("PAIR_SCOPE"), question)
		}
		// two subjects, not a pair: each number's own events; "between" the dates of the period; the numbers in common; how they are connected; one number alone; three numbers
		for _, question := range []string{
			"How many calls did " + a + " and " + b + " make between 2026-03-01 and 2026-03-08?",
			"How many calls did " + a + " and " + b + " make in total?",
			"Which numbers did " + a + " and " + b + " both call between March and April 2026?",
		} {
			Expect(obligations(question, perNumber)).NotTo(HaveKey("PAIR_SCOPE"), question)
		}
		Expect(obligations("How many calls were there between "+a+" and "+b+" and 923000001036?", perNumber)).NotTo(HaveKey("PAIR_SCOPE"), "three numbers are not a pair")
		Expect(obligations("How many calls did "+a+" make between 2026-03-01 and 2026-03-08?", "SELECT count(*) AS n FROM v_comm_events WHERE src = '"+a+"'")).NotTo(HaveKey("PAIR_SCOPE"))
		// a join of the contact pairs with themselves (the numbers in common) is not read as a pair predicate
		common := "SELECT x.dst AS number FROM v_contact_pairs x JOIN v_contact_pairs y ON y.dst = x.dst WHERE x.src = '" + a + "' AND y.src = '" + b + "' LIMIT 50"
		Expect(obligations("Who do "+a+" and "+b+" have in common between them?", common)).NotTo(HaveKey("PAIR_SCOPE"))
	})

	It("is retrieved for the question of the bank, so that the first draft imitates a query that passes", func() {
		examples, err := askExamples()
		Expect(err).NotTo(HaveOccurred())
		shown := askRetrieveExamples(askExtractFrame(q).Question, []string{"v_comm_events"}, examples, 2)
		ids := []string{}
		for _, e := range shown {
			ids = append(ids, e.ID)
		}
		Expect(ids).To(ContainElement("pair-talk-time"))
		// the example passes the same obligations when its numbers are the question's
		fixed := obligations(q, strings.NewReplacer("923000000101", a, "923000000102", b).Replace(onlyExample(examples, "pair-talk-time").SQL))
		Expect(fixed).To(BeEmpty())
	})

	Describe("through the run", func() {
		var (
			prompts [][]govSQLTurn
			execs   []string
			cfg     config
			db      *pgxpool.Pool

			savedModel    askModelFunc
			savedLoader   = askCatalogueLoader
			savedExec     = askExec
			savedProbeFn  = askProbeFn
			savedSink     = askAuditSink
			savedCover    = askCoverageRead
			savedCaseSet  = askCaseSettingsFn
			savedEvidence = askCollectEvidenceFn
		)
		BeforeEach(func() {
			savedModel = askModel
			prompts, execs = nil, nil
			db = &pgxpool.Pool{}
			cfg = config{SynthesisModel: "test-model"}
			askCatalogueLoader = func(context.Context, *pgxpool.Pool) (*askCatalogue, error) { return cat, nil }
			askExec = func(_ context.Context, _ *pgxpool.Pool, _, _ string, v *askValidated, _ askLimits) (*govSQLResult, error) {
				execs = append(execs, v.SQL)
				return &govSQLResult{Columns: []string{"total_seconds"}, Texts: []bool{false}, Rows: [][]any{{int64(12290)}}}, nil
			}
			askProbeFn = func(context.Context, *pgxpool.Pool, string, string, *askCatalogue, string, askLimits) ([]string, error) {
				return nil, nil
			}
			askCaseSettingsFn = func(context.Context, *pgxpool.Pool, string, string) (askCaseSetting, error) { return askCaseSetting{}, nil }
			askCollectEvidenceFn = func(context.Context, *pgxpool.Pool, string, string, *askCatalogue, *askValidated, *govSQLResult, askLimits) *askEvidence {
				return nil
			}
			askCoverageCache.entries = nil
			askCoverageRead = func(context.Context, *pgxpool.Pool, string, string) (askCoverage, error) {
				return askCoverage{Files: 3, Typed: 3, AnyComm: true, AnySession: true, AnyUsage: true, AnyVolume: true}, nil
			}
			askAuditSink = func(context.Context, *pgxpool.Pool, hybridQueryRequest, AskAuditV2, askFrame, string) {}
		})
		AfterEach(func() {
			askModel, askCatalogueLoader, askExec, askProbeFn, askAuditSink, askCoverageRead = savedModel, savedLoader, savedExec, savedProbeFn, savedSink, savedCover
			askCaseSettingsFn, askCollectEvidenceFn = savedCaseSet, savedEvidence
		})

		It("repairs the chain of alternatives with the pair predicate and answers on the second draft; the first was never run", func() {
			reply := func(sql string) string {
				raw, _ := json.Marshal(map[string]any{"answerable": true, "sql": sql})
				return string(raw)
			}
			replies := []string{reply(flat), reply(once)}
			i := 0
			askModel = func(_ context.Context, _ config, _ string, turns []govSQLTurn) (string, string) {
				prompts = append(prompts, append([]govSQLTurn{}, turns...))
				if i >= len(replies) {
					return "", "SCRIPT_ENDED"
				}
				i++
				return replies[i-1], ""
			}
			req := hybridQueryRequest{TenantID: "t", CollectionID: "c", Query: q, RequestClass: forensicrequest.GovernedAnalysis}
			resp, audit := runAskEngineV2(context.Background(), cfg, db, req, time.Now())
			Expect(audit.State).To(Equal("answered"), audit.Reason)
			Expect(audit.Attempts).To(Equal(2))
			Expect(audit.Repairs).To(ContainElement("OBLIGATION:PAIR_SCOPE"))
			Expect(execs).To(HaveLen(1), "the chain of alternatives was never run")
			Expect(execs[0]).To(ContainSubstring("SELECT DISTINCT start_utc, duration_s"))
			Expect(prompts[1][len(prompts[1])-1].Content).To(ContainSubstring("src IN ('" + a + "', '" + b + "') AND dst IN ('" + a + "', '" + b + "')"))
			Expect(resp).NotTo(BeNil())
			// the example for this question is in the prompt of the first draft
			Expect(prompts[0][0].Content).To(ContainSubstring("SELECT sum(duration_s) AS total_seconds FROM (SELECT DISTINCT start_utc, duration_s FROM v_comm_events"))
		})

		It("abstains, naming the cause, when the model keeps writing the chain", func() {
			raw, _ := json.Marshal(map[string]any{"answerable": true, "sql": flat})
			askModel = func(_ context.Context, _ config, _ string, turns []govSQLTurn) (string, string) {
				prompts = append(prompts, append([]govSQLTurn{}, turns...))
				return string(raw), ""
			}
			req := hybridQueryRequest{TenantID: "t", CollectionID: "c", Query: q, RequestClass: forensicrequest.GovernedAnalysis}
			_, audit := runAskEngineV2(context.Background(), cfg, db, req, time.Now())
			Expect(audit.State).To(Equal("abstained"))
			Expect(execs).To(BeEmpty(), "no figure is shown for the chain of alternatives")
		})
	})
})

func onlyExample(examples []askExample, id string) askExample {
	for _, e := range examples {
		if e.ID == id {
			return e
		}
	}
	Fail("no verified example " + id)
	return askExample{}
}
