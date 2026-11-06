package caselayer

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func fixture(name string) []byte {
	raw, err := os.ReadFile("testdata/" + name + ".json")
	Expect(err).NotTo(HaveOccurred())
	return raw
}

var _ = Describe("text rules", func() {
	It("masks dashed national ID numbers and counts them", func() {
		masked, n := MaskNationalIDs("who is 35202-1234567-1 and also 61101-7654321-9?")
		Expect(n).To(Equal(2))
		Expect(masked).To(Equal("who is #####-#######-# and also #####-#######-#?"))
		same, none := MaskNationalIDs("calls of 923001234567 on 2026-02-03")
		Expect(none).To(Equal(0))
		Expect(same).To(Equal("calls of 923001234567 on 2026-02-03"))
	})

	DescribeTable("cleanLine",
		func(in string, min, max int, want string, fragment string) {
			got, err := cleanLine("The title", in, min, max)
			if fragment == "" {
				Expect(err).NotTo(HaveOccurred())
				Expect(got).To(Equal(want))
				return
			}
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(fragment))
		},
		Entry("trims", "  Harbour Road  ", 1, 20, "Harbour Road", ""),
		Entry("counts characters, not bytes (Urdu)", "کال ریکارڈ", 1, 10, "کال ریکارڈ", ""),
		Entry("too long", strings.Repeat("x", 21), 1, 20, "", "longer than 20"),
		Entry("empty", "   ", 1, 20, "", "is required"),
		Entry("a line break", "two\nlines", 1, 20, "", "control characters"),
	)

	It("lets a body keep its line breaks and refuses other control characters", func() {
		got, err := cleanBody("The note", "line one\nline two\ttabbed", 1, 100)
		Expect(err).NotTo(HaveOccurred())
		Expect(got).To(ContainSubstring("\n"))
		_, err = cleanBody("The note", "bad\x00byte", 1, 100)
		Expect(err).To(HaveOccurred())
	})

	It("shortens without cutting a character", func() {
		Expect(clip("short", 10)).To(Equal("short"))
		got := clip(strings.Repeat("ک", 50), 10)
		Expect([]rune(got)).To(HaveLen(10))
		Expect(got).To(HaveSuffix("…"))
	})

	It("escapes LIKE characters in search words and keeps at most eight", func() {
		words := searchWords(`50% off_sale back\slash a b c d e f g h`)
		Expect(words).To(HaveLen(8))
		Expect(words[0]).To(Equal(`50\%`))
		Expect(words[1]).To(Equal(`off\_sale`))
		Expect(words[2]).To(Equal(`back\\slash`))
	})
})

var _ = Describe("the digest of an engine response", func() {
	It("reads an answered question: sentence, shape, rows, query, views, timings and the labelled figure", func() {
		d, err := ExtractDigest(fixture("engine_ranked"))
		Expect(err).NotTo(HaveOccurred())
		Expect(d.Outcome).To(Equal(OutcomeAnswered))
		Expect(d.Headline).To(HavePrefix("3 results"))
		Expect(d.Shape).To(Equal("ranked"))
		Expect(d.Columns).To(Equal([]Column{{Key: "label", Header: "Label"}, {Key: "value", Header: "Value"}}))
		Expect(d.Rows).To(HaveLen(3))
		Expect(fmt.Sprint(d.Rows[0][0], " ", d.Rows[0][1])).To(Equal("923000000201 14"))
		Expect(d.RowCount).To(Equal(3))
		Expect(d.Query).To(HavePrefix("SELECT dst AS label, events AS value FROM v_contact_pairs"), "the query of the derivation, not the older one under answer.governed_sql")
		Expect(d.Views).To(Equal([]string{"v_contact_pairs"}))
		Expect(d.Attempts).To(Equal(1))
		Expect(d.PromptID).To(Equal("3bb60baef6dcbf66"))
		Expect(d.TotalMS).To(BeEquivalentTo(2149))
		Expect(d.ExecMS).To(BeEquivalentTo(25))
		Expect(d.Metrics).To(Equal([]Metric{{Label: "Number of records", Value: "5,005"}}))
		Expect(QueryHash(d.Query)).To(MatchRegexp(`^[0-9a-f]{64}$`))
		Expect(QueryHash("SELECT  1\n FROM  x")).To(Equal(QueryHash("SELECT 1 FROM x")), "whitespace does not change the hash")
		Expect(d.ResultDigest()).To(MatchRegexp(`^[0-9a-f]{64}$`))
	})

	It("changes the result digest when one cell changes, and not when nothing does", func() {
		a, _ := ExtractDigest(fixture("engine_ranked"))
		b, _ := ExtractDigest(fixture("engine_ranked"))
		Expect(a.ResultDigest()).To(Equal(b.ResultDigest()))
		b.Rows[1][1] = json.Number("10")
		Expect(a.ResultDigest()).NotTo(Equal(b.ResultDigest()))
	})

	It("reads an abstention as abstained, with its own sentence, no figures and no result table", func() {
		d, err := ExtractDigest(fixture("engine_abstain"))
		Expect(err).NotTo(HaveOccurred())
		Expect(d.Outcome).To(Equal(OutcomeAbstained))
		Expect(d.Headline).To(HavePrefix("I did not give an answer"))
		Expect(d.Metrics).To(BeEmpty(), "the planner's own labels are not figures of the case")
		Expect(d.RowCount).To(Equal(0))
		Expect(d.ResultDigest()).To(Equal(""))
	})

	It("reads the governed lane's answers too", func() {
		d, err := ExtractDigest(fixture("lane_answered"))
		Expect(err).NotTo(HaveOccurred())
		Expect(d.Outcome).To(Equal(OutcomeAnswered))
		Expect(d.Headline).To(Equal("Number of records: 5,005."))
		Expect(d.Query).To(ContainSubstring("number_of_records"))
		a, err := ExtractDigest(fixture("lane_absence"))
		Expect(err).NotTo(HaveOccurred())
		Expect(a.Outcome).To(Equal(OutcomeAnswered))
		Expect(a.Headline).To(HavePrefix("No record in this case contains"))
	})

	It("finds every evidence file an answer cites, once, whatever the path put it", func() {
		a, b := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
		raw := fmt.Sprintf(`{"answer":{"answer":"x","result_state":"COMPLETED"},"enterprise":{"evidence":{"files":[{"evidence_id":"%s","source_file":"calls.csv","rows":3}],
			"rows":[{"evidence_id":"%s","version_id":"%s","source_file":"bank.xlsx"},{"evidence_id":"%s","source_file":"calls.csv"}]},
			"data_grid":{"rows":[{"citation":{"evidence_id":"not-a-uuid"}}]}}}`, a, b, b, a)
		d, err := ExtractDigest([]byte(raw))
		Expect(err).NotTo(HaveOccurred())
		Expect(d.Evidence).To(HaveLen(2))
		Expect(d.Evidence[0].EvidenceID).To(Equal(a))
		Expect(d.Evidence[0].SourceFile).To(Equal("calls.csv"))
		Expect(d.Evidence[1]).To(Equal(EvidenceRef{EvidenceID: b, VersionID: b, SourceFile: "bank.xlsx"}))
	})

	It("refuses something that is not a JSON object", func() {
		_, err := ExtractDigest([]byte(`[1,2]`))
		Expect(err).To(HaveOccurred())
		_, err = ExtractDigest([]byte(`null`))
		Expect(err).To(HaveOccurred())
	})

	It("keeps the first 200 rows for comparison and says so", func() {
		d := &Digest{Columns: []Column{{Key: "a", Header: "A"}}, RowCount: 500}
		for i := 0; i < 500; i++ {
			d.Rows = append(d.Rows, []any{json.Number(fmt.Sprint(i))})
		}
		f := d.Figures()
		Expect(f.Rows).To(HaveLen(FiguresMaxRows))
		Expect(f.Truncated).To(BeTrue())
		Expect(f.RowCount).To(Equal(500))
		Expect(f.RowsDigest).NotTo(BeEmpty())
	})
})

var _ = Describe("snapshots", func() {
	small := SnapshotLimits{RawMax: 64 << 10, StoredMax: 16 << 10, TrimRows: 5, TrimEvidenceRows: 2}

	bigResponse := func(rows int) []byte {
		var grid []map[string]any
		for i := 0; i < rows; i++ {
			grid = append(grid, map[string]any{"label": fmt.Sprintf("row %d %x", i, i*2654435761), "value": i})
		}
		raw, err := json.Marshal(map[string]any{"answer": map[string]any{"answer": "many rows"}, "enterprise": map[string]any{"data_grid": map[string]any{"columns": []map[string]string{{"key": "label", "header": "Label"}, {"key": "value", "header": "Value"}}, "rows": grid}}})
		Expect(err).NotTo(HaveOccurred())
		return raw
	}

	It("stores a response whole when it fits, and the SHA-256 is of the JSON", func() {
		raw := fixture("engine_ranked")
		snap, err := BuildSnapshot(raw, small)
		Expect(err).NotTo(HaveOccurred())
		Expect(snap.State).To(Equal(SnapshotFull))
		back, err := DecodeSnapshot(snap.Blob, small)
		Expect(err).NotTo(HaveOccurred())
		Expect(back).To(Equal(raw))
		Expect(snap.SHA256).To(Equal(sha256Hex(raw)))
		Expect(snap.Bytes).To(Equal(len(raw)))
	})

	It("cuts the long lists, notes it, and says trimmed, when the whole is too big", func() {
		raw := bigResponse(3000)
		Expect(len(raw)).To(BeNumerically(">", small.RawMax/4))
		tight := SnapshotLimits{RawMax: 1 << 20, StoredMax: 8 << 10, TrimRows: 5, TrimEvidenceRows: 2}
		snap, err := BuildSnapshot(raw, tight)
		Expect(err).NotTo(HaveOccurred())
		Expect(snap.State).To(Equal(SnapshotTrimmed))
		back, err := DecodeSnapshot(snap.Blob, tight)
		Expect(err).NotTo(HaveOccurred())
		var m map[string]any
		Expect(json.Unmarshal(back, &m)).To(Succeed())
		rows := m["enterprise"].(map[string]any)["data_grid"].(map[string]any)["rows"].([]any)
		Expect(rows).To(HaveLen(5))
		note := m["snapshot_trimmed"].(map[string]any)
		Expect(note["rows_total"]).To(BeEquivalentTo(3000))
		Expect(snap.SHA256).To(Equal(sha256Hex(back)))
	})

	It("keeps nothing when even the trimmed copy is too big, or the response is not JSON", func() {
		snap, err := BuildSnapshot(bigResponse(3000), SnapshotLimits{RawMax: 1 << 20, StoredMax: 64, TrimRows: 5, TrimEvidenceRows: 2})
		Expect(err).NotTo(HaveOccurred())
		Expect(snap.State).To(Equal(SnapshotNone))
		Expect(snap.Blob).To(BeNil())
		snap, err = BuildSnapshot([]byte("not json"), SnapshotLimits{RawMax: 4, StoredMax: 4, TrimRows: 1, TrimEvidenceRows: 1})
		Expect(err).NotTo(HaveOccurred())
		Expect(snap.State).To(Equal(SnapshotNone))
		empty, err := BuildSnapshot(nil, small)
		Expect(err).NotTo(HaveOccurred())
		Expect(empty.State).To(Equal(SnapshotNone))
	})

	It("refuses to unzip more than the limit", func() {
		var buf bytes.Buffer
		zw := gzip.NewWriter(&buf)
		_, _ = zw.Write(bytes.Repeat([]byte{0}, 2<<20))
		Expect(zw.Close()).To(Succeed())
		_, err := DecodeSnapshot(buf.Bytes(), SnapshotLimits{RawMax: 1 << 20})
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("comparing two runs", func() {
	cols := []Column{{Key: "label", Header: "Number"}, {Key: "value", Header: "Calls"}}
	figures := func(rows ...[]any) Figures {
		d := &Digest{Columns: cols, Rows: rows, RowCount: len(rows)}
		return d.Figures()
	}
	n := func(s string) json.Number { return json.Number(s) }
	run := func(id, headline string, f Figures) Run {
		at := time.Date(2026, 11, 3, 9, 0, 0, 0, time.UTC)
		return Run{TurnID: id, Outcome: OutcomeAnswered, Headline: headline, Figures: f, QueryHash: "q1", DataVersionHash: "d1", CompletedAt: &at}
	}

	It("finds nothing when nothing differs", func() {
		f := figures([]any{"A", n("1")}, []any{"B", n("2")})
		d := Compare(run("t1", "2 numbers", f), run("t2", "2 numbers", f), nil, nil)
		Expect(d.Changed).To(BeFalse())
		Expect(d.FiguresChanged).To(BeFalse())
		Expect(d.Rows.AddedTotal + d.Rows.RemovedTotal + d.Rows.ChangedTotal).To(Equal(0))
		Expect(d.Complete).To(BeTrue())
		Expect(d.SameDataDifferentAnswer).To(BeFalse())
	})

	It("pairs a removed and an added row that share their first cell as one changed row, cell by cell", func() {
		before := figures([]any{"A", n("1")}, []any{"B", n("2")}, []any{"C", n("3")})
		after := figures([]any{"A", n("1")}, []any{"B", n("5")}, []any{"D", n("4")})
		d := Compare(run("t1", "3 numbers", before), run("t2", "3 numbers", after), nil, nil)
		Expect(d.Changed).To(BeTrue())
		Expect(d.Rows.ChangedTotal).To(Equal(1))
		Expect(d.Rows.Changed[0].Key).To(Equal("B"))
		Expect(d.Rows.Changed[0].Cells).To(Equal([]CellChange{{Column: "Calls", Before: n("2"), After: n("5")}}))
		Expect(d.Rows.AddedTotal).To(Equal(1))
		Expect(d.Rows.RemovedTotal).To(Equal(1))
		Expect(d.Rows.Added[0][0]).To(Equal("D"))
		Expect(d.Rows.Removed[0][0]).To(Equal("C"))
	})

	It("does not call a table changed because its rows came in another order", func() {
		a := figures([]any{"A", n("1")}, []any{"B", n("2")})
		b := figures([]any{"B", n("2")}, []any{"A", n("1")})
		d := Compare(run("t1", "x", a), run("t2", "x", b), nil, nil)
		Expect(d.Rows.Reordered).To(BeTrue())
		Expect(d.FiguresChanged).To(BeFalse())
		Expect(d.Changed).To(BeFalse())
	})

	It("reports a different sentence, a different outcome and a different column set", func() {
		a := run("t1", "14 calls", figures([]any{"A", n("1")}))
		b := run("t2", "15 calls", figures([]any{"A", n("1")}))
		b.Outcome = OutcomeAbstained
		Expect(Compare(a, b, nil, nil).Headline.Changed).To(BeTrue())
		Expect(Compare(a, b, nil, nil).Outcome).To(Equal(Change{Before: OutcomeAnswered, After: OutcomeAbstained, Changed: true}))
		other := &Digest{Columns: []Column{{Key: "x", Header: "X"}}, Rows: [][]any{{"A"}}, RowCount: 1}
		c := run("t3", "14 calls", other.Figures())
		Expect(Compare(a, c, nil, nil).Rows.ColumnsChanged).To(BeTrue())
		Expect(Compare(a, c, nil, nil).FiguresChanged).To(BeTrue())
	})

	It("compares the labelled figures and gives the difference of numbers, thousands separators included", func() {
		a := run("t1", "x", Figures{Metrics: []Metric{{Label: "Number of records", Value: "5,005"}, {Label: "Cells", Value: "n/a"}}})
		b := run("t2", "x", Figures{Metrics: []Metric{{Label: "Number of records", Value: "5,010"}, {Label: "Cells", Value: "n/a"}, {Label: "Files", Value: "3"}}})
		d := Compare(a, b, nil, nil)
		Expect(d.Metrics).To(Equal([]MetricChange{{Label: "Number of records", Before: "5,005", After: "5,010", Delta: "5"}, {Label: "Files", Before: "", After: "3", Delta: ""}}))
		Expect(numericDelta("1.5", "2")).To(Equal("0.5000"))
		Expect(numericDelta("a", "2")).To(Equal(""))
	})

	It("lists added, removed and replaced evidence files", func() {
		a, b, c := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
		v1, v2 := "11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222"
		x := run("t1", "x", figures())
		x.Evidence = []EvidenceRef{{EvidenceID: a, VersionID: v1, SourceFile: "calls.csv"}, {EvidenceID: b, SourceFile: "bank.xlsx"}}
		y := run("t2", "x", figures())
		y.Evidence = []EvidenceRef{{EvidenceID: a, VersionID: v2, SourceFile: "calls.csv"}, {EvidenceID: c, SourceFile: "new.csv"}}
		d := Compare(x, y, nil, nil)
		Expect(d.EvidenceChanged).To(BeTrue())
		Expect(d.Evidence.Added).To(Equal([]EvidenceRef{{EvidenceID: c, SourceFile: "new.csv"}}))
		Expect(d.Evidence.Removed).To(Equal([]EvidenceRef{{EvidenceID: b, SourceFile: "bank.xlsx"}}))
		Expect(d.Evidence.Replaced).To(Equal([]DVReplaced{{EvidenceID: a, Name: "calls.csv", BeforeVersion: v1, AfterVersion: v2}}))
		Expect(d.Changed).To(BeTrue())
	})

	It("says when the same data gave a different answer, and compares the query that was written", func() {
		a := run("t1", "14 calls", figures([]any{"A", n("14")}))
		b := run("t2", "15 calls", figures([]any{"A", n("15")}))
		b.QueryHash = "q2"
		d := Compare(a, b, nil, nil)
		Expect(d.Data.Known).To(BeTrue())
		Expect(d.Data.Changed).To(BeFalse())
		Expect(d.SameDataDifferentAnswer).To(BeTrue())
		Expect(d.Query.Changed).To(BeTrue())
		b.DataVersionHash = "d2"
		Expect(Compare(a, b, nil, nil).SameDataDifferentAnswer).To(BeFalse())
		Expect(Compare(a, b, nil, nil).Data.Changed).To(BeTrue())
	})

	It("names what moved between two versions of the data", func() {
		before := DataVersion{
			Evidence: []DVEvidence{{EvidenceID: "e1", VersionID: "v1", SHA256: "h1", Name: "calls.csv"}, {EvidenceID: "e2", VersionID: "v1", SHA256: "h2", Name: "old.csv"}},
			Families: []DVFamily{{Family: "comm_event", Rows: 100, LastIngestedAt: "2026-11-01T00:00:00Z"}},
			Settings: map[string]any{"status": "open", "hub_threshold": 10},
		}
		after := DataVersion{
			Evidence: []DVEvidence{{EvidenceID: "e1", VersionID: "v2", SHA256: "h1b", Name: "calls.csv"}, {EvidenceID: "e3", VersionID: "v1", SHA256: "h3", Name: "new.csv"}},
			Families: []DVFamily{{Family: "comm_event", Rows: 120, LastIngestedAt: "2026-11-02T00:00:00Z"}, {Family: "bank_transaction", Rows: 5}},
			Settings: map[string]any{"status": "open", "hub_threshold": 12},
		}
		Expect(before.Hash()).NotTo(Equal(after.Hash()))
		Expect(before.Hash()).To(Equal(before.Hash()))
		ch := CompareDataVersions(before, after)
		Expect(ch.EvidenceAdded).To(HaveLen(1))
		Expect(ch.EvidenceAdded[0].Name).To(Equal("new.csv"))
		Expect(ch.EvidenceRemoved[0].Name).To(Equal("old.csv"))
		Expect(ch.EvidenceReplaced[0].EvidenceID).To(Equal("e1"))
		Expect(ch.Families).To(HaveLen(2))
		Expect(ch.Settings).To(Equal([]string{"hub_threshold"}))
	})
})

var _ = Describe("the follow-up lock", func() {
	history := []HistoryTurn{{TurnID: "t1", Question: "How many calls did 923001234567 make in February 2026?", Frame: json.RawMessage(`{"numbers":["923001234567"],"months":[{"year":2026,"month":2}]}`)}}

	DescribeTable("numberLock",
		func(original, resolved string, offending []string) {
			Expect(numberLock(original, history, resolved)).To(Equal(offending))
		},
		Entry("a question that carries over the number and the month", "and for March?", "How many calls did 923001234567 make in March 2026?", nil),
		Entry("adds a number nobody wrote", "and for March?", "How many calls did 923009999999 make in March?", []string{"923009999999"}),
		Entry("adds a date nobody wrote", "and for March?", "How many calls did 923001234567 make on 2026-03-05?", []string{"2026-03-05"}),
		Entry("words alone are never offending", "and for last week?", "How many calls did that number make last week?", nil),
		Entry("is not fooled by case", "ok", "HOW MANY calls did 923001234567 make?", nil),
	)

	type scripted struct {
		result FollowUpResult
		err    error
		called bool
	}
	resolver := func(s *scripted) FollowUpResolver {
		return resolverFunc(func(_ context.Context, _ FollowUpInput) (FollowUpResult, error) {
			s.called = true
			return s.result, s.err
		})
	}

	It("uses a standalone question that passes the lock", func() {
		s := &scripted{result: FollowUpResult{Question: "How many calls did 923001234567 make in March 2026?", Inherited: []string{"number"}, Model: "m"}}
		got, res := applyFollowUp(context.Background(), resolver(s), "and for March?", history)
		Expect(got).To(Equal("How many calls did 923001234567 make in March 2026?"))
		Expect(res).To(Equal(Resolution{Method: "model", Inherited: []string{"number"}, Model: "m"}))
	})

	It("drops a proposal that adds a value, and says why", func() {
		s := &scripted{result: FollowUpResult{Question: "How many calls did 923007777777 make in March?"}}
		got, res := applyFollowUp(context.Background(), resolver(s), "and for March?", history)
		Expect(got).To(Equal("and for March?"))
		Expect(res.Method).To(Equal("rejected"))
		Expect(res.Reason).To(ContainSubstring("923007777777"))
	})

	It("leaves the message as typed when the resolver fails, returns it unchanged, or there is no history or no resolver", func() {
		got, res := applyFollowUp(context.Background(), resolver(&scripted{err: errors.New("down")}), "and for March?", history)
		Expect(got).To(Equal("and for March?"))
		Expect(res.Method).To(Equal("failed"))
		_, res = applyFollowUp(context.Background(), resolver(&scripted{result: FollowUpResult{Question: "and for March?"}}), "and for March?", history)
		Expect(res.Method).To(Equal("none"))
		s := &scripted{}
		_, res = applyFollowUp(context.Background(), resolver(s), "first question", nil)
		Expect(res.Method).To(Equal("none"))
		Expect(s.called).To(BeFalse(), "with no history there is nothing to resolve against")
		_, res = applyFollowUp(context.Background(), nil, "and for March?", history)
		Expect(res.Method).To(Equal("none"))
	})

	It("renders the history and the message for the model, and parses its answer", func() {
		prompt := FollowUpUserPrompt(FollowUpInput{Question: "and for March?", History: history})
		Expect(prompt).To(ContainSubstring("1. How many calls did 923001234567"))
		Expect(prompt).To(ContainSubstring("New message: and for March?"))
		res, err := ParseFollowUp(`{"question":"q","inherited":["number"]}`)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Inherited).To(Equal([]string{"number"}))
		_, err = ParseFollowUp(`nonsense`)
		Expect(err).To(HaveOccurred())
	})
})

type resolverFunc func(context.Context, FollowUpInput) (FollowUpResult, error)

func (f resolverFunc) Resolve(ctx context.Context, in FollowUpInput) (FollowUpResult, error) {
	return f(ctx, in)
}

var _ = Describe("roles", func() {
	It("lets each role do what the roadmap gives it", func() {
		Expect(RoleOpen.CanContribute()).To(BeTrue())
		Expect(RoleAnalyst.CanContribute()).To(BeTrue())
		Expect(RoleReviewer.CanContribute()).To(BeFalse())
		Expect(RoleReviewer.CanReview()).To(BeTrue())
		Expect(RoleViewer.CanContribute()).To(BeFalse())
		Expect(RoleViewer.CanGiveFeedback()).To(BeTrue())
		Expect(RoleAuditor.CanGiveFeedback()).To(BeFalse())
		Expect(RoleAuditor.CanReadAudit()).To(BeTrue())
		Expect(RoleAnalyst.CanReadAudit()).To(BeFalse())
		Expect(RoleLead.CanManage()).To(BeTrue())
		Expect(RoleAnalyst.CanManage()).To(BeFalse())
		Expect(RoleNone.CanRead()).To(BeFalse())
		Expect(RoleAuditor.SeesPrivateThreads()).To(BeTrue())
		Expect(RoleLead.SeesPrivateThreads()).To(BeFalse())
		Expect(Scope{ActorRole: "admin"}.IsAdmin()).To(BeTrue())
	})
})

var _ = Describe("the stream of a turn", func() {
	It("chooses the encoding from the query string or the Accept header", func() {
		get := func(url, accept string) string {
			r := httptest.NewRequest(http.MethodPost, url, nil)
			if accept != "" {
				r.Header.Set("Accept", accept)
			}
			return streamMode(r)
		}
		Expect(get("/conversations/x/turns", "")).To(Equal(""))
		Expect(get("/conversations/x/turns", "application/json")).To(Equal(""))
		Expect(get("/conversations/x/turns", "application/x-ndjson")).To(Equal("ndjson"))
		Expect(get("/conversations/x/turns", "text/event-stream")).To(Equal("sse"))
		Expect(get("/conversations/x/turns?stream=ndjson", "")).To(Equal("ndjson"))
		Expect(get("/conversations/x/turns?stream=sse", "application/x-ndjson")).To(Equal("sse"))
		Expect(get("/conversations/x/turns?stream=false", "application/x-ndjson")).To(Equal(""))
	})

	It("writes one JSON object per line, starting the response on the first event", func() {
		rec := httptest.NewRecorder()
		sink := &streamSink{w: rec, mode: "ndjson"}
		Expect(sink.hasStarted()).To(BeFalse())
		sink.Send(Event{Event: "phase", Phase: "accepted", Label: "Question received", AtMS: 3})
		sink.Send(Event{Event: "heartbeat", ElapsedMS: 2000, AtMS: 2010})
		Expect(sink.hasStarted()).To(BeTrue())
		Expect(rec.Header().Get("Content-Type")).To(Equal("application/x-ndjson"))
		lines := strings.Split(strings.TrimSpace(rec.Body.String()), "\n")
		Expect(lines).To(HaveLen(2))
		var first Event
		Expect(json.Unmarshal([]byte(lines[0]), &first)).To(Succeed())
		Expect(first.Phase).To(Equal("accepted"))
		Expect(lines[1]).To(ContainSubstring(`"elapsed_ms":2000`))
	})

	It("writes the same events as server-sent events", func() {
		rec := httptest.NewRecorder()
		sink := &streamSink{w: rec, mode: "sse"}
		sink.Send(Event{Event: "phase", Phase: "engine", Label: "Answering"})
		Expect(rec.Header().Get("Content-Type")).To(Equal("text/event-stream"))
		Expect(rec.Body.String()).To(HavePrefix("event: phase\ndata: {"))
		Expect(rec.Body.String()).To(HaveSuffix("}\n\n"))
	})

	It("carries the engine's own phases to the stream, and does nothing without a stream", func() {
		EmitPhase(context.Background(), "running", "Running")
		var got []string
		ctx := WithPhaseEmitter(context.Background(), func(phase, label string) { got = append(got, phase+"|"+label) })
		EmitPhase(ctx, "running", "Running it")
		Expect(got).To(Equal([]string{"running|Running it"}))
	})

	It("queues turns for the model and tells the second one its place", func() {
		s := newSlots(1)
		Expect(s.acquire(context.Background(), nil)).To(BeTrue())
		position := make(chan int, 1)
		done := make(chan bool, 1)
		go func() { done <- s.acquire(context.Background(), func(p int) { position <- p }) }()
		Eventually(position).Should(Receive(Equal(1)))
		s.release()
		Eventually(done).Should(Receive(BeTrue()))
		s.release()
		ctx, cancel := context.WithCancel(context.Background())
		Expect(s.acquire(ctx, nil)).To(BeTrue())
		cancel()
		Expect(s.acquire(ctx, nil)).To(BeFalse(), "a turn that gives up waiting does not take a slot")
	})
})

var _ = Describe("request bodies of the knowledge objects", func() {
	body := func(s string) map[string]json.RawMessage {
		var m map[string]json.RawMessage
		Expect(json.Unmarshal([]byte(s), &m)).To(Succeed())
		return m
	}

	It("accepts a note, fills the default category, and refuses what is not a field", func() {
		_, vals, err := noteSpec.parseBody(body(`{"body":"check the second handset"}`), true)
		Expect(err).NotTo(HaveOccurred())
		Expect(vals["category"]).To(Equal("note"))
		_, _, err = noteSpec.parseBody(body(`{"body":"x","owner":"me"}`), true)
		Expect(err).To(MatchError(ContainSubstring(`"owner" is not a field`)))
		_, _, err = noteSpec.parseBody(body(`{"category":"gossip","body":"x"}`), true)
		Expect(err).To(MatchError(ContainSubstring("category must be one of")))
		_, _, err = noteSpec.parseBody(body(`{"category":"lead"}`), true)
		Expect(err).To(MatchError(ContainSubstring("body is required")))
		_, _, err = noteSpec.parseBody(body(`{"source_turn_id":"nope","body":"x"}`), true)
		Expect(err).To(MatchError(ContainSubstring("source_turn_id")))
	})

	It("refuses to change what is fixed once made, and an empty patch", func() {
		_, _, err := savedViewSpec.parseBody(body(`{"kind":"timeline"}`), false)
		Expect(err).To(MatchError(ContainSubstring("kind cannot be changed")))
		_, _, err = noteSpec.parseBody(body(`{}`), false)
		Expect(err).To(MatchError(ContainSubstring("Nothing to change")))
		_, _, err = noteSpec.parseBody(body(`{"body":null}`), false)
		Expect(err).To(MatchError(ContainSubstring("cannot be null")))
	})

	It("keeps a task's completion time in step with its status", func() {
		args := []any{"tenant", "case", "analyst-1"}
		sets := taskComplete(map[string]any{"status": "done"}, "analyst-1", &args)
		Expect(sets).To(Equal([]string{"completed_at = now()", "completed_by = $4"}))
		Expect(args).To(HaveLen(4))
		Expect(taskComplete(map[string]any{"status": "open"}, "analyst-1", &args)).To(Equal([]string{"completed_at = NULL", "completed_by = NULL"}))
		Expect(taskComplete(map[string]any{"title": "x"}, "analyst-1", &args)).To(BeNil())
	})

	It("checks the dates and the attachments of an authority record", func() {
		ok := `{"kind":"court_order","reference":"ATC-2026/114","issuing_body":"Anti-Terrorism Court Lahore","valid_from":"2026-10-01","valid_to":"2027-03-31","attachments":[{"evidence_id":"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"}]}`
		_, _, err := authoritySpec.parseBody(body(ok), true)
		Expect(err).NotTo(HaveOccurred())
		_, _, err = authoritySpec.parseBody(body(`{"kind":"court_order","reference":"r","issuing_body":"b","valid_from":"2026-10-01","valid_to":"2026-09-01"}`), true)
		Expect(err).To(MatchError(ContainSubstring("valid_to is before valid_from")))
		_, _, err = authoritySpec.parseBody(body(`{"kind":"court_order","reference":"r","issuing_body":"b","attachments":[{"evidence_id":"x"}]}`), true)
		Expect(err).To(MatchError(ContainSubstring("evidence_id")))
		_, _, err = authoritySpec.parseBody(body(`{"kind":"verbal","reference":"r","issuing_body":"b"}`), true)
		Expect(err).To(MatchError(ContainSubstring("kind must be one of")))
		_, _, err = authoritySpec.parseBody(body(`{"kind":"consent","reference":"r","issuing_body":"b","valid_from":"3 November"}`), true)
		Expect(err).To(MatchError(ContainSubstring("valid_from must be a date")))
	})

	It("validates a saved view's kind and size", func() {
		_, _, err := savedViewSpec.parseBody(body(`{"name":"Links in February","kind":"connections","params_json":{"from":"2026-02-01"}}`), true)
		Expect(err).NotTo(HaveOccurred())
		_, _, err = savedViewSpec.parseBody(body(`{"name":"x","kind":"Connections!"}`), true)
		Expect(err).To(MatchError(ContainSubstring("not in the form allowed")))
		_, _, err = savedViewSpec.parseBody(body(`{"name":"x","kind":"links","params_json":[1]}`), true)
		Expect(err).To(MatchError(ContainSubstring("JSON object")))
	})

	It("selects the row as JSON without the tenant, with the case id and the tags", func() {
		sql := noteSpec.jsonSelect("x")
		Expect(sql).To(ContainSubstring("- 'tenant_id'"))
		Expect(sql).To(ContainSubstring("'case_id', x.collection_id"))
		Expect(sql).To(ContainSubstring("'tags'"))
		Expect(annotationSpec.jsonSelect("x")).NotTo(ContainSubstring("'tags'"))
		Expect(authoritySpec.jsonSelect("x")).To(ContainSubstring("'status', CASE WHEN x.revoked_at"))
	})

	It("has a route for every spec", func() {
		for plural, spec := range objectSpecs {
			Expect(spec.Plural).To(Equal(plural))
			Expect(spec.Table).NotTo(BeEmpty())
			Expect(spec.IDCol).NotTo(BeEmpty())
		}
	})
})

var _ = Describe("errors and cursors", func() {
	It("turns database errors into sentences and never shows the cause of an unknown one", func() {
		Expect(asError(&pgconn.PgError{Code: "23505"}).Status).To(Equal(http.StatusConflict))
		Expect(asError(&pgconn.PgError{Code: "42501"}).Status).To(Equal(http.StatusForbidden))
		Expect(asError(&pgconn.PgError{Code: "55000"}).Code).To(Equal("immutable"))
		Expect(asError(&pgconn.PgError{Code: "23514", ConstraintName: "case_note_body_chk"}).Message).To(ContainSubstring("case_note_body_chk"))
		unknown := asError(errors.New("password authentication failed for user forensic_runtime"))
		Expect(unknown.Status).To(Equal(http.StatusInternalServerError))
		Expect(unknown.Message).NotTo(ContainSubstring("password"))
		typed := NotFound("nothing here")
		Expect(asError(fmt.Errorf("wrapped: %w", typed))).To(BeIdenticalTo(typed))
		Expect(asError(nil)).To(BeNil())
	})

	It("round-trips a cursor and refuses a forged one", func() {
		in := threadCursor{Pinned: true, TS: time.Date(2026, 11, 3, 9, 0, 0, 0, time.UTC), ID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"}
		var out threadCursor
		Expect(decodeCursor(encodeCursor(in), &out)).To(Succeed())
		Expect(out.ID).To(Equal(in.ID))
		Expect(out.TS.Equal(in.TS)).To(BeTrue())
		Expect(decodeCursor("%%%", &out)).To(HaveOccurred())
		Expect(decodeCursor("e30", &out)).To(Succeed(), "an empty object is a valid cursor of nothing")
		Expect(clampLimit(0)).To(Equal(30))
		Expect(clampLimit(1000)).To(Equal(100))
		Expect(clampLimit(7)).To(Equal(7))
	})

	It("reads request bodies with absent and null told apart", func() {
		r := httptest.NewRequest(http.MethodPatch, "/x", strings.NewReader(`{"title":null,"pinned":true}`))
		m, err := readMap(r, false)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(m["title"])).To(Equal("null"))
		_, present := m["archived"]
		Expect(present).To(BeFalse())
		Expect(onlyKeys(m, "title", "pinned", "archived")).To(Succeed())
		Expect(onlyKeys(m, "title")).To(MatchError(ContainSubstring(`"pinned" is not a field`)))
		_, err = readMap(httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(`[1]`)), false)
		Expect(err).To(HaveOccurred())
		empty, err := readMap(httptest.NewRequest(http.MethodDelete, "/x", strings.NewReader(``)), true)
		Expect(err).NotTo(HaveOccurred())
		Expect(empty).To(BeEmpty())
	})

	It("answers an error as JSON with the code and a request id", func() {
		h := &Handler{}
		rec := httptest.NewRecorder()
		h.fail(rec, httptest.NewRequest(http.MethodGet, "/x", nil), Forbidden("no"))
		Expect(rec.Code).To(Equal(http.StatusForbidden))
		var body map[string]string
		Expect(json.Unmarshal(rec.Body.Bytes(), &body)).To(Succeed())
		Expect(body["error"]).To(Equal("no"))
		Expect(body["code"]).To(Equal("forbidden"))
		Expect(body["request_id"]).NotTo(BeEmpty())
		Expect(rec.Header().Get("X-Request-ID")).To(Equal(body["request_id"]))
	})

	It("says so when the handler has no way to tell who is asking", func() {
		rec := httptest.NewRecorder()
		(&Handler{}).getThread(rec, httptest.NewRequest(http.MethodGet, "/conversations/x", nil))
		Expect(rec.Code).To(Equal(http.StatusServiceUnavailable))
	})
})
