package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The model's reading of a result (ask_engine_reading.go, decision D33): what the model is shown, the checks that every number and word of the reading must pass, the one retry, and the token that
// only the asking scope can use.
var _ = Describe("Ask engine reading of a result", func() {
	shown := "Question: What is the average duration of voice calls?\nFigure: Average duration: 105.3568.\nResult: 1 rows\ncolumns: average_duration\n105.3568\n" +
		"Records behind it: 3,871 rows in 12 source files; largest: export-P-000.xlsx (402 rows). Recomputed from the 3,871 matching rows in 12 files.\n"

	Describe("the checks", func() {
		It("passes a reading that uses only what it was shown, and accepts a rounding of a decimal", func() {
			Expect(askReadingCheck("The average voice call lasts 105.3568 seconds, over 3,871 calls in 12 files. This is a mean, not a typical call.", shown)).To(BeEmpty())
			Expect(askReadingCheck("The average voice call lasts about 105.36 seconds.", shown)).To(BeEmpty())
		})
		It("refuses a number, a date or an identifier that was not shown", func() {
			Expect(askReadingCheck("The average is 105.3568 seconds across 4,000 calls.", shown)).To(ContainSubstring("4,000"))
			Expect(askReadingCheck("The longest call was on 2026-02-17.", shown)).To(ContainSubstring("2026-02-17"))
			Expect(askReadingCheck("Number 923000001005 made most of them.", shown)).To(ContainSubstring("923000001005"))
		})
		It("refuses words that accuse or overstate", func() {
			for _, text := range []string{"This looks suspicious.", "That proves the number was used by the owner.", "The caller is a criminal.", "It is definitely a pattern."} {
				Expect(askReadingCheck(text, shown)).To(ContainSubstring("not allowed"), text)
			}
		})
		It("refuses a name of the engine's own (a view, a column, a condition) and asks for everyday words", func() {
			Expect(askReadingCheck("This rests on 3,871 rows of the v_comm_events table.", shown)).To(ContainSubstring("internal"))
			Expect(askReadingCheck("Calls where event_type = 'voice' lasted 105.3568 seconds.", shown)).To(ContainSubstring("internal"))
			Expect(askReadingCheck("The average voice call lasts 105.3568 seconds.", shown)).To(BeEmpty())
		})
		It("refuses an empty or a long reading", func() {
			Expect(askReadingCheck("  ", shown)).To(ContainSubstring("empty"))
			Expect(askReadingCheck(strings.Repeat("The average is 105.3568 seconds. ", 30), shown)).To(ContainSubstring("longer than"))
		})
	})

	Describe("what the model is shown", func() {
		It("is the question, the figure, at most 20 rows, the steps, the evidence and the notes, and says when it sees only some of the rows", func() {
			r := &govSQLResult{Columns: []string{"number", "calls"}}
			for i := 0; i < 25; i++ {
				r.Rows = append(r.Rows, []any{"92300000" + string(rune('A'+i%20)), int64(i)})
			}
			ev := &askEvidence{State: "itemised", Total: 300, Files: []askEvidenceFile{{SourceFile: "a.xlsx", Rows: 200}, {SourceFile: "b.xlsx", Rows: 100}}, Check: "Recomputed from the 300 matching rows in 2 files."}
			text := askReadingText("Who made the most calls?", "25 rows.", r, []string{"Read: v_comm_events."}, []string{"Computed by a model-written query over this case's analyst views ...", "Times are read on the case clock."}, ev)
			Expect(text).To(ContainSubstring("Question: Who made the most calls?"))
			Expect(text).To(ContainSubstring("only the first 20 are shown to you"))
			Expect(strings.Count(text, "92300000")).To(Equal(20))
			Expect(text).To(ContainSubstring("How it was found: Read: v_comm_events."))
			Expect(text).To(ContainSubstring("300 rows in 2 source files; largest: a.xlsx (200 rows) b.xlsx (100 rows)"))
			Expect(text).To(ContainSubstring("Note: Times are read on the case clock."))
			Expect(text).NotTo(ContainSubstring("Computed by a model-written query"), "the first note is about the query model, not for the reading")
		})
	})

	Describe("asking the model", func() {
		var saved askReadingModelFunc
		BeforeEach(func() { saved = askReadingModel })
		AfterEach(func() { askReadingModel = saved })
		scripted := func(texts ...string) *int {
			calls := 0
			askReadingModel = func(_ context.Context, _ config, _, _, _ string) (string, string) {
				calls++
				raw, _ := json.Marshal(map[string]string{"reading": texts[min(calls-1, len(texts)-1)]})
				return string(raw), ""
			}
			return &calls
		}
		in := func() *askReadingInput { return &askReadingInput{Tenant: "t", Collection: "c", Shown: shown} }

		It("returns a reading that passes the checks at once", func() {
			calls := scripted("The average voice call lasts 105.3568 seconds, computed from 3,871 records in 12 files.")
			got := askReadingMake(context.Background(), config{SynthesisModel: "m"}, in())
			Expect(got.State).To(Equal("ready"))
			Expect(got.Text).To(ContainSubstring("105.3568"))
			Expect(got.Disclosure).To(ContainSubstring("Check it against the rows"))
			Expect(*calls).To(Equal(1))
		})
		It("asks once more with the fault named, and keeps the second reading when it passes", func() {
			var seen []string
			calls := 0
			askReadingModel = func(_ context.Context, _ config, _, _, user string) (string, string) {
				calls++
				seen = append(seen, user)
				text := "The average is 110 seconds."
				if calls == 2 {
					text = "The average is 105.3568 seconds."
				}
				raw, _ := json.Marshal(map[string]string{"reading": text})
				return string(raw), ""
			}
			got := askReadingMake(context.Background(), config{SynthesisModel: "m"}, in())
			Expect(got.State).To(Equal("ready"))
			Expect(calls).To(Equal(2))
			Expect(seen[1]).To(ContainSubstring("Your last reading was refused: the number 110 does not occur"))
		})
		It("asks again, shorter, when the model ran out of room before it ended its reading", func() {
			var seen []string
			askReadingModel = func(_ context.Context, _ config, _, _, user string) (string, string) {
				seen = append(seen, user)
				if len(seen) == 1 {
					return "", "NOT_FINISHED"
				}
				return `{"reading":"The average is 105.3568 seconds."}`, ""
			}
			got := askReadingMake(context.Background(), config{SynthesisModel: "m"}, in())
			Expect(got.State).To(Equal("ready"))
			Expect(seen).To(HaveLen(2))
			Expect(seen[1]).To(ContainSubstring("cut off. Write at most two short sentences"))
		})
		It("drops a reading that fails twice, and says so", func() {
			scripted("This looks suspicious.", "It proves the point.")
			got := askReadingMake(context.Background(), config{SynthesisModel: "m"}, in())
			Expect(got.State).To(Equal("unavailable"))
			Expect(got.Text).To(BeEmpty())
			Expect(got.Reason).To(ContainSubstring("did not pass the check against the rows"))
		})
		It("says when the model did not answer", func() {
			askReadingModel = func(context.Context, config, string, string, string) (string, string) {
				return "", "TIMEOUT_OR_UNAVAILABLE"
			}
			got := askReadingMake(context.Background(), config{SynthesisModel: "m"}, in())
			Expect(got.State).To(Equal("unavailable"))
			Expect(got.Reason).To(ContainSubstring("did not answer in time"))
		})
	})

	Describe("the token and the route", func() {
		var saved askReadingModelFunc
		BeforeEach(func() { saved = askReadingModel })
		AfterEach(func() { askReadingModel = saved })

		get := func(token, tenant, collection string) *httptest.ResponseRecorder {
			rec := httptest.NewRecorder()
			askReadingHandler(config{SynthesisModel: "m"})(rec, httptest.NewRequest(http.MethodGet, "/query/reading?token="+token+"&tenant_id="+tenant+"&collection_id="+collection, nil))
			return rec
		}

		It("is given only to an answer that has rows, and not to an absence, an empty result or when the reading is switched off", func() {
			r := &govSQLResult{Columns: []string{"n"}, Rows: [][]any{{int64(7)}}}
			req := hybridQueryRequest{TenantID: "t", CollectionID: "c", Query: "How many?"}
			fresh := func() *hybridQueryResponse {
				return &hybridQueryResponse{Answer: map[string]any{"answer": "N: 7."}, Enterprise: map[string]any{"limitations": []string{"first"}}}
			}
			ok := fresh()
			askAttachReading(ok, req, r, nil, nil, "")
			pending := ok.Enterprise["reading"].(map[string]any)
			Expect(pending["state"]).To(Equal("pending"))
			Expect(pending["token"]).To(HaveLen(32))
			Expect(pending["path"]).To(Equal("/query/reading?token=" + pending["token"].(string)))
			absence := fresh()
			askAttachReading(absence, req, r, nil, nil, "No record of this case holds 923000099999.")
			Expect(absence.Enterprise).NotTo(HaveKey("reading"))
			empty := fresh()
			askAttachReading(empty, req, &govSQLResult{Columns: []string{"n"}}, nil, nil, "")
			Expect(empty.Enterprise).NotTo(HaveKey("reading"))
			GinkgoT().Setenv(askReadingEnv, "false")
			off := fresh()
			askAttachReading(off, req, r, nil, nil, "")
			Expect(off.Enterprise).NotTo(HaveKey("reading"))
		})

		It("serves the reading to the scope that asked, keeps a settled reading, and tries again after a model that did not answer", func() {
			token := askReadingStore(&askReadingInput{Tenant: "t", Collection: "c", Shown: shown})
			Expect(token).To(HaveLen(32))
			calls := 0
			failing := true
			askReadingModel = func(context.Context, config, string, string, string) (string, string) {
				calls++
				if failing {
					return "", "TIMEOUT_OR_UNAVAILABLE"
				}
				return `{"reading":"The average is 105.3568 seconds."}`, ""
			}
			first := get(token, "t", "c")
			Expect(first.Code).To(Equal(http.StatusOK))
			Expect(first.Body.String()).To(ContainSubstring(`"state":"unavailable"`))
			failing = false
			second := get(token, "t", "c")
			Expect(second.Body.String()).To(ContainSubstring(`"state":"ready"`))
			Expect(second.Body.String()).To(ContainSubstring("105.3568"))
			before := calls
			third := get(token, "t", "c")
			Expect(third.Body.String()).To(ContainSubstring(`"state":"ready"`))
			Expect(calls).To(Equal(before), "a settled reading is not asked for again")
		})

		It("does not serve another tenant, another case, or a token that is unknown", func() {
			token := askReadingStore(&askReadingInput{Tenant: "t", Collection: "c", Shown: shown})
			Expect(get(token, "other", "c").Code).To(Equal(http.StatusNotFound))
			Expect(get(token, "t", "other-case").Code).To(Equal(http.StatusNotFound))
			Expect(get("0123456789abcdef0123456789abcdef", "t", "c").Code).To(Equal(http.StatusNotFound))
		})
	})
})
