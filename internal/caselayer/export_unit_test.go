package caselayer

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func sp(s string) *string { return &s }

// answeredTurn is a finished, answered turn of the case orion as the stored columns have it.
func answeredTurn(id, root string, idx int, kind string) Turn {
	at := time.Date(2026, 11, 4, 8, 0, idx, 0, time.UTC)
	return Turn{
		ID: id, ThreadID: "th-1", CaseID: "orion", Index: idx, Kind: kind, RootID: root, Question: "Which numbers made the most calls?",
		State: "completed", Outcome: sp(OutcomeAnswered), Headline: sp("3 numbers made calls: 923000000201 (14), 923000000202 (9), 923000000203 (4)."), Shape: sp("ranked"),
		RowCount: func() *int { n := 3; return &n }(), QueryHash: sp(strings.Repeat("a", 64)), ResultDigest: sp(strings.Repeat("b", 64)),
		Evidence:      json.RawMessage(`[{"evidence_id":"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa","version_id":"bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb","source_file":"calls.csv"}]`),
		SnapshotState: "full", EngineVersion: sp("engine-9d"), ModelID: sp("qwen-7b"), PromptID: sp("ask-v2.4"), DataVersionHash: sp(strings.Repeat("c", 64)),
		CreatedBy: "alice", CreatedAt: at, StartedAt: at, CompletedAt: &at,
	}
}

func sampleFigures() *Figures {
	return &Figures{
		Columns:    []Column{{Key: "label", Header: "Number"}, {Key: "value", Header: "Calls"}},
		Rows:       [][]any{{"923000000201", json.Number("14")}, {"923000000202", json.Number("9")}, {"923000000203", json.Number("4")}},
		RowCount:   3,
		RowsDigest: strings.Repeat("b", 64),
		Metrics:    []Metric{{Label: "Number of records", Value: "5,005"}},
	}
}

func sampleData() *DataVersion {
	return &DataVersion{
		Evidence: []DVEvidence{{EvidenceID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", VersionID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", SHA256: strings.Repeat("d", 64), Name: "calls.csv"}},
		Families: []DVFamily{{Family: "comm_event", Rows: 5005, LastIngestedAt: "2026-11-01T10:00:00Z"}},
		Settings: map[string]any{"status": "open"},
	}
}

var _ = Describe("exporting answers to the report builder (no database)", func() {
	Describe("the canonical form a hash is taken over", func() {
		It("sorts keys, writes no spaces, does not escape < > &, and keeps a long number exactly", func() {
			raw, err := canonicalJSON(map[string]any{"b": 1, "a": []any{"x<y & z>", json.Number("12345678901234567890"), nil, true}, "é": "ق"})
			Expect(err).NotTo(HaveOccurred())
			Expect(string(raw)).To(Equal(`{"a":["x<y & z>",12345678901234567890,null,true],"b":1,"é":"ق"}`))
		})

		It("is the same for a struct and for the map it decodes to, and for two calls", func() {
			type row struct {
				Z int    `json:"z"`
				A string `json:"a"`
			}
			one, err := canonicalJSON(row{Z: 2, A: "x"})
			Expect(err).NotTo(HaveOccurred())
			two, err := canonicalJSON(map[string]any{"z": 2, "a": "x"})
			Expect(err).NotTo(HaveOccurred())
			Expect(string(one)).To(Equal(`{"a":"x","z":2}`))
			Expect(one).To(Equal(two))
			again, _ := canonicalJSON(row{Z: 2, A: "x"})
			Expect(again).To(Equal(one))
		})
	})

	Describe("an exhibit of an answer", func() {
		It("carries the question, the sentence, the table, the cited files, how it was produced and what the case held", func() {
			turn := answeredTurn("turn-1", "turn-1", 1, "ask")
			ex := answerExhibit(turnExport{Turn: turn, AnswerText: "Three numbers made calls in the period.", Query: "SELECT src AS label, count(*) AS value FROM v_comm_events GROUP BY 1",
				Figures: sampleFigures(), Data: sampleData(), CurrentHash: strings.Repeat("c", 64)}, 50)
			Expect(ex.ID).To(Equal("turn:turn-1"))
			Expect(ex.Kind).To(Equal("answer"))
			Expect(ex.CaseID).To(Equal("orion"))
			Expect(ex.Title).To(HavePrefix("3 numbers made calls"))
			Expect(ex.Question).To(Equal("Which numbers made the most calls?"))
			Expect(ex.Answer.Outcome).To(Equal("answered"))
			Expect(ex.Answer.Headline).To(HavePrefix("3 numbers"))
			Expect(ex.Answer.Text).To(Equal("Three numbers made calls in the period."))
			Expect(ex.Answer.Shape).To(Equal("ranked"))
			Expect(ex.Answer.Figures).To(Equal([]Metric{{Label: "Number of records", Value: "5,005"}}))
			Expect(ex.Answer.Table.RowsShown).To(Equal(3))
			Expect(ex.Answer.Table.Truncated).To(BeFalse())
			Expect(ex.Evidence).To(Equal([]EvidenceRef{{EvidenceID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", VersionID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", SourceFile: "calls.csv"}}))
			p := ex.Provenance
			Expect(p.TurnID).To(Equal("turn-1"))
			Expect(p.ThreadID).To(Equal("th-1"))
			Expect(p.RunKind).To(Equal("ask"))
			Expect(p.AskedBy).To(Equal("alice"))
			Expect(p.EngineVersion).To(Equal("engine-9d"))
			Expect(p.Model).To(Equal("qwen-7b"))
			Expect(p.PromptID).To(Equal("ask-v2.4"))
			Expect(p.Query).To(HavePrefix("SELECT src AS label"))
			Expect(p.QueryHash).To(Equal(strings.Repeat("a", 64)))
			Expect(p.ResultDigest).To(Equal(strings.Repeat("b", 64)))
			Expect(p.DataVersionHash).To(Equal(strings.Repeat("c", 64)))
			Expect(p.SnapshotState).To(Equal("full"))
			Expect(ex.Data.Recorded).To(BeTrue())
			Expect(ex.Data.Evidence).To(HaveLen(1))
			Expect(ex.Data.Families[0].Family).To(Equal("comm_event"))
			Expect(ex.Reproduce.Rerun).To(Equal("POST /turns/turn-1/rerun"))
			Expect(*ex.Context.DataCurrent).To(BeTrue())
		})

		It("says the data has changed when the case no longer holds what the answer was computed on, and says nothing when that was not read", func() {
			turn := answeredTurn("turn-1", "turn-1", 1, "ask")
			moved := answerExhibit(turnExport{Turn: turn, Figures: sampleFigures(), Data: sampleData(), CurrentHash: strings.Repeat("e", 64)}, 50)
			Expect(*moved.Context.DataCurrent).To(BeFalse())
			unknown := answerExhibit(turnExport{Turn: turn, Figures: sampleFigures(), Data: sampleData()}, 50)
			Expect(unknown.Context.DataCurrent).To(BeNil())
		})

		It("says that the version of the data was not recorded, rather than leaving the block out", func() {
			turn := answeredTurn("turn-1", "turn-1", 1, "ask")
			ex := answerExhibit(turnExport{Turn: turn, Figures: sampleFigures()}, 50)
			Expect(ex.Data.Recorded).To(BeFalse())
			Expect(ex.Data.Hash).To(Equal(strings.Repeat("c", 64)), "the hash is still the one the turn stored")
			Expect(ex.Data.Evidence).NotTo(BeNil())
			Expect(ex.Data.Families).NotTo(BeNil())
			raw, _ := json.Marshal(ex)
			Expect(string(raw)).To(ContainSubstring(`"evidence":[]`))
		})

		It("leaves the table out of an answer that has none and still has no null lists", func() {
			turn := answeredTurn("turn-1", "turn-1", 1, "ask")
			ex := answerExhibit(turnExport{Turn: turn}, 50)
			Expect(ex.Answer.Table).To(BeNil())
			Expect(ex.Answer.Figures).NotTo(BeNil())
			Expect(ex.Answer.Figures).To(BeEmpty())
		})

		It("keeps a question that named a national ID number as it was stored, masked", func() {
			turn := answeredTurn("turn-1", "turn-1", 1, "ask")
			turn.Question, _ = MaskNationalIDs("whose calls are on 35202-1234567-1?")
			ex := answerExhibit(turnExport{Turn: turn}, 50)
			Expect(ex.Question).To(Equal("whose calls are on #####-#######-#?"))
		})
	})

	Describe("the table of an exhibit", func() {
		cols := []Column{{Key: "a", Header: "A"}}
		rows := func(n int) [][]any {
			out := make([][]any, n)
			for i := range out {
				out[i] = []any{json.Number("1")}
			}
			return out
		}

		It("shows the rows asked for and says what was cut", func() {
			t := exhibitTable(cols, rows(5), 5, "d", 3)
			Expect(t.RowsShown).To(Equal(3))
			Expect(t.Rows).To(HaveLen(3))
			Expect(t.RowCount).To(Equal(5))
			Expect(t.Truncated).To(BeTrue())
			Expect(t.RowsDigest).To(Equal("d"))
		})

		It("says a result is cut when the layer kept fewer rows than the result has", func() {
			t := exhibitTable(cols, rows(200), 1234, "d", 500)
			Expect(t.RowsShown).To(Equal(200))
			Expect(t.RowCount).To(Equal(1234))
			Expect(t.Truncated).To(BeTrue())
		})

		It("is not cut when every row is shown", func() {
			t := exhibitTable(cols, rows(2), 2, "", 50)
			Expect(t.Truncated).To(BeFalse())
			Expect(t.RowsShown).To(Equal(2))
		})

		It("has no table when there are no columns, and never has null lists", func() {
			Expect(exhibitTable(nil, nil, 0, "", 50)).To(BeNil())
			t := exhibitTable(cols, nil, 0, "", 50)
			raw, _ := json.Marshal(t)
			Expect(string(raw)).To(ContainSubstring(`"rows":[]`))
		})
	})

	Describe("the hash of an exhibit and of a bundle", func() {
		build := func() Exhibit {
			turn := answeredTurn("turn-1", "turn-1", 1, "ask")
			return answerExhibit(turnExport{Turn: turn, AnswerText: "text", Figures: sampleFigures(), Data: sampleData(), CurrentHash: strings.Repeat("c", 64)}, 50)
		}

		It("covers the content but not the day of the export", func() {
			a := build()
			ha, err := a.ComputeHash()
			Expect(err).NotTo(HaveOccurred())
			Expect(ha).To(MatchRegexp(`^[0-9a-f]{64}$`))
			b := build()
			stale := false
			b.Context = &ExhibitContext{DataCurrent: &stale}
			b.SHA256 = "ignored"
			hb, err := b.ComputeHash()
			Expect(err).NotTo(HaveOccurred())
			Expect(hb).To(Equal(ha), "the context and the stored hash are not part of what is hashed")
		})

		It("changes with the question, the figures, the sentence, the model and the data version", func() {
			base := build()
			h0, _ := base.ComputeHash()
			mutations := map[string]func(*Exhibit){
				"question":  func(e *Exhibit) { e.Question += " " },
				"headline":  func(e *Exhibit) { e.Answer.Headline += "." },
				"a cell":    func(e *Exhibit) { e.Answer.Table.Rows[1][1] = json.Number("10") },
				"a figure":  func(e *Exhibit) { e.Answer.Figures[0].Value = "5,006" },
				"the model": func(e *Exhibit) { e.Provenance.Model = "other" },
				"the data":  func(e *Exhibit) { e.Provenance.DataVersionHash = strings.Repeat("e", 64) },
				"evidence":  func(e *Exhibit) { e.Evidence[0].SourceFile = "other.csv" },
				"the case":  func(e *Exhibit) { e.CaseID = "meridian" },
				"the query": func(e *Exhibit) { e.Provenance.Query += " " },
			}
			for name, mutate := range mutations {
				e := build()
				mutate(&e)
				h, err := e.ComputeHash()
				Expect(err).NotTo(HaveOccurred())
				Expect(h).NotTo(Equal(h0), "changing %s must change the hash", name)
			}
		})

		It("can be checked by anyone from the JSON alone: drop sha256 and context, write the keys in order, hash", func() {
			e := build()
			h, err := e.ComputeHash()
			Expect(err).NotTo(HaveOccurred())
			e.SHA256 = h
			raw, err := json.Marshal(e)
			Expect(err).NotTo(HaveOccurred())
			var back map[string]any
			dec := json.NewDecoder(strings.NewReader(string(raw)))
			dec.UseNumber()
			Expect(dec.Decode(&back)).To(Succeed())
			Expect(back["sha256"]).To(Equal(h))
			delete(back, "sha256")
			delete(back, "context")
			canon, err := canonicalJSON(back)
			Expect(err).NotTo(HaveOccurred())
			sum := sha256.Sum256(canon)
			Expect(hex.EncodeToString(sum[:])).To(Equal(h))
		})

		It("hashes a bundle from the format, the cases and the hashes of its exhibits in order", func() {
			h1, h2 := strings.Repeat("1", 64), strings.Repeat("2", 64)
			a := bundleHash([]string{"orion"}, []string{h1, h2})
			Expect(a).To(MatchRegexp(`^[0-9a-f]{64}$`))
			Expect(bundleHash([]string{"orion"}, []string{h1, h2})).To(Equal(a))
			Expect(bundleHash([]string{"orion"}, []string{h2, h1})).NotTo(Equal(a), "the order of the exhibits is part of the bundle")
			Expect(bundleHash([]string{"orion", "meridian"}, []string{h1, h2})).NotTo(Equal(a))
			Expect(bundleHash([]string{"orion"}, []string{h1})).NotTo(Equal(a))
		})
	})

	Describe("which answers of a conversation go into an export", func() {
		It("takes the latest answered run of each question, in the order the questions were asked, and says what it left out", func() {
			r1, r3, r4 := "t1", "t3", "t4"
			abstained := answeredTurn("t3", r3, 3, "ask")
			abstained.Outcome = sp(OutcomeAbstained)
			failedRerun := answeredTurn("t5", r4, 5, "rerun")
			failedRerun.State, failedRerun.Outcome = "failed", sp(OutcomeFailed)
			turns := []Turn{
				answeredTurn("t1", r1, 1, "ask"),
				answeredTurn("t2", r1, 2, "rerun"),
				abstained,
				answeredTurn("t4", r4, 4, "ask"),
				failedRerun,
			}
			selected, skipped := selectDefaultTurns(turns)
			var ids []string
			for _, t := range selected {
				ids = append(ids, t.ID)
			}
			Expect(ids).To(Equal([]string{"t2", "t4"}), "the re-run of the first question replaces its first run; a failed re-run does not replace an answer")
			Expect(skipped).To(HaveLen(1))
			Expect(skipped[0].Ref).To(Equal("turn:t3"))
			Expect(skipped[0].Reason).To(ContainSubstring("abstained"))
		})

		It("says a question that is still running was not answered yet", func() {
			running := answeredTurn("t1", "t1", 1, "ask")
			running.State, running.Outcome, running.CompletedAt = "running", nil, nil
			selected, skipped := selectDefaultTurns([]Turn{running})
			Expect(selected).To(BeEmpty())
			Expect(skipped).To(HaveLen(1))
			Expect(skipped[0].Reason).To(ContainSubstring("not finished (running)"))
		})

		It("keeps the branches: an edited question is another question", func() {
			a := answeredTurn("t1", "t1", 1, "ask")
			b := answeredTurn("t2", "t2", 2, "edit")
			selected, _ := selectDefaultTurns([]Turn{a, b})
			Expect(selected).To(HaveLen(2))
		})
	})

	Describe("an exhibit of a finding", func() {
		pinned := `{"turn_id":"turn-1","thread_id":"th-1","question":"Which numbers made the most calls?","headline":"3 numbers made calls.","outcome":"answered","shape":"ranked","row_count":3,` +
			`"result_digest":"` + "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" + `","query_hash":"` + "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" +
			`","engine_version":"engine-9d","model":"qwen-7b","data_version_hash":"` + "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc" + `","answered_at":"2026-11-04T08:00:01Z",` +
			`"figures":{"columns":[{"key":"label","header":"Number"},{"key":"value","header":"Calls"}],"rows":[["923000000201",14],["923000000202",9]],"row_count":3,"rows_digest":"x","metrics":[],"truncated":false}}`

		row := func(state string) findingRow {
			at := time.Date(2026, 11, 4, 9, 0, 0, 0, time.UTC)
			f := findingRow{ID: "f-1", CaseID: "orion", Title: "Three numbers carry the traffic", Body: "The top number is the suspect's second handset.", Basis: "observed",
				ReviewState: state, SourceTurnID: sp("turn-1"), Pinned: json.RawMessage(pinned), Evidence: json.RawMessage(`[{"evidence_id":"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa","source_file":"calls.csv"}]`),
				DataVersionHash: sp(strings.Repeat("c", 64)), CreatedBy: "alice", CreatedAt: at}
			if state != "unreviewed" {
				f.ReviewedBy, f.ReviewedAt, f.ReviewNote = sp("bob"), &at, sp("checked against the file")
			}
			return f
		}

		It("carries the text of the finding, who reviewed it, and the answer as it was when it was pinned", func() {
			ex := findingExhibit(row("confirmed"), sampleData(), strings.Repeat("c", 64), 50)
			Expect(ex.ID).To(Equal("finding:f-1"))
			Expect(ex.Kind).To(Equal("finding"))
			Expect(ex.Title).To(Equal("Three numbers carry the traffic"))
			Expect(ex.Finding.Body).To(ContainSubstring("second handset"))
			Expect(ex.Finding.ReviewState).To(Equal("confirmed"))
			Expect(*ex.Finding.ReviewedBy).To(Equal("bob"))
			Expect(ex.Question).To(Equal("Which numbers made the most calls?"))
			Expect(ex.Answer.Headline).To(Equal("3 numbers made calls."))
			Expect(ex.Answer.Table.RowsShown).To(Equal(2))
			Expect(ex.Answer.Table.RowCount).To(Equal(3))
			Expect(ex.Answer.Table.Truncated).To(BeTrue(), "the finding keeps two of three rows, and says so")
			Expect(ex.Provenance.TurnID).To(Equal("turn-1"))
			Expect(ex.Provenance.EngineVersion).To(Equal("engine-9d"))
			Expect(ex.Provenance.QueryHash).To(Equal(strings.Repeat("a", 64)))
			Expect(ex.Provenance.DataVersionHash).To(Equal(strings.Repeat("c", 64)))
			Expect(ex.Evidence).To(HaveLen(1))
			Expect(ex.Reproduce.Rerun).To(Equal("POST /turns/turn-1/rerun"))
			Expect(*ex.Context.DataCurrent).To(BeTrue())
		})

		It("is an exhibit of the finding alone when it was not made from an answer", func() {
			f := row("unreviewed")
			f.SourceTurnID, f.Pinned = nil, json.RawMessage(`{}`)
			ex := findingExhibit(f, nil, "", 50)
			Expect(ex.Answer).To(BeNil())
			Expect(ex.Reproduce).To(BeNil())
			Expect(ex.Finding.ReviewState).To(Equal("unreviewed"))
			Expect(ex.Data.Recorded).To(BeFalse())
			h, err := ex.ComputeHash()
			Expect(err).NotTo(HaveOccurred())
			Expect(h).To(MatchRegexp(`^[0-9a-f]{64}$`))
		})
	})

	Describe("who may export", func() {
		It("is anyone who works on the case, not someone who only reads or audits it", func() {
			for _, r := range []Role{RoleOwner, RoleLead, RoleAnalyst, RoleReviewer, RoleOpen} {
				Expect(r.CanExport()).To(BeTrue(), string(r))
			}
			for _, r := range []Role{RoleViewer, RoleAuditor, RoleNone, Role("")} {
				Expect(r.CanExport()).To(BeFalse(), string(r))
			}
		})
	})

	Describe("the Markdown of a bundle", func() {
		bundle := func() *Bundle {
			turn := answeredTurn("turn-1", "turn-1", 1, "ask")
			turn.Question = "کس نمبر نے سب سے زیادہ کالیں کیں؟ <b>bold</b>"
			fig := sampleFigures()
			fig.Rows = [][]any{{"a|b\nc", json.Number("14")}, {"<script>alert(1)</script>", json.Number("9")}}
			fig.RowCount = 14
			ex1 := answerExhibit(turnExport{Turn: turn, AnswerText: "Three numbers made calls.", Figures: fig, Data: sampleData(), CurrentHash: strings.Repeat("e", 64)}, 50)
			ex1.SHA256, _ = ex1.ComputeHash()
			f := findingExhibit(findingRow{ID: "f-1", CaseID: "orion", Title: "A finding", Body: "line one\nline two", Basis: "inferred", ReviewState: "unreviewed", CreatedBy: "alice",
				CreatedAt: time.Date(2026, 11, 4, 9, 0, 0, 0, time.UTC), Pinned: json.RawMessage(`{}`), Evidence: json.RawMessage(`[]`)}, nil, "", 50)
			f.SHA256, _ = f.ComputeHash()
			return &Bundle{Format: ExportFormat, Title: "Calls of the number", CaseIDs: []string{"orion"}, ExportedBy: "alice", ExportedAt: time.Date(2026, 11, 5, 9, 30, 0, 0, time.UTC),
				Exhibits: []Exhibit{ex1, f}, Skipped: []Skipped{{Ref: "turn:t9", Reason: "The latest run of this question is abstained, not an answer."}}, BundleSHA256: strings.Repeat("9", 64)}
		}

		It("is a document: a title, the bundle hash, one section per exhibit with its question, sentence, table, files, provenance and hash", func() {
			md := renderMarkdown(bundle())
			Expect(md).To(HavePrefix("# Calls of the number\n"))
			Expect(md).To(ContainSubstring("alice"))
			Expect(md).To(ContainSubstring("2026-11-05T09:30:00Z"))
			Expect(md).To(ContainSubstring("Bundle SHA-256: " + strings.Repeat("9", 64)))
			Expect(md).To(ContainSubstring("## Exhibit 1. "))
			Expect(md).To(ContainSubstring("## Exhibit 2. A finding"))
			Expect(md).To(ContainSubstring("کس نمبر نے سب سے زیادہ کالیں کیں؟"), "Urdu is written as it was typed")
			Expect(md).To(ContainSubstring("| Number | Calls |"))
			Expect(md).To(ContainSubstring("| --- | --- |"))
			Expect(md).To(ContainSubstring("Showing 2 of 14 rows"))
			Expect(md).To(ContainSubstring("calls.csv"))
			Expect(md).To(ContainSubstring("Engine: engine-9d"))
			Expect(md).To(ContainSubstring("Model: qwen-7b"))
			Expect(md).To(ContainSubstring("Query hash: " + strings.Repeat("a", 64)))
			Expect(md).To(ContainSubstring("Data version: " + strings.Repeat("c", 64)))
			Expect(md).To(ContainSubstring("SHA-256 of this exhibit: "))
			Expect(md).To(ContainSubstring("Review: not yet reviewed"))
			Expect(md).To(ContainSubstring("Basis: inferred"))
			Expect(md).To(ContainSubstring("The case's data has changed since this answer was computed."))
			Expect(md).To(ContainSubstring("## Not exported"))
			Expect(md).To(ContainSubstring("turn:t9"))
			Expect(md).To(ContainSubstring("abstained"))
			Expect(md).To(HaveSuffix("\n"))
		})

		It("cannot be broken by what a cell or a question contains: no raw markup, a pipe or a line break stays in its cell", func() {
			md := renderMarkdown(bundle())
			Expect(md).NotTo(ContainSubstring("<script>"))
			Expect(md).NotTo(ContainSubstring("<b>"))
			Expect(md).To(ContainSubstring("&lt;script&gt;alert(1)&lt;/script&gt;"))
			Expect(md).To(ContainSubstring(`a\|b c`), "the pipe is escaped and the line break is a space")
			for _, line := range strings.Split(md, "\n") {
				if strings.HasPrefix(line, "| a") {
					Expect(strings.Count(line, "|") - strings.Count(line, `\|`)).To(Equal(3), "a table row has its two cells: %q", line)
				}
			}
		})

		It("is the same every time, and says nothing about the data being changed when it was not", func() {
			b := bundle()
			Expect(renderMarkdown(b)).To(Equal(renderMarkdown(b)))
			current := true
			b.Exhibits[0].Context = &ExhibitContext{DataCurrent: &current}
			Expect(renderMarkdown(b)).NotTo(ContainSubstring("has changed since this answer was computed"))
		})
	})
})
