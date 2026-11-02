package main

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The evidence behind a figure (ask_engine_evidence.go): the plan is read from the answer query's own parse tree, the statements it writes pass the engine's own validator, a figure is
// recomputed from the pieces, and what lands in the response is what the workspace already knows how to show.
var _ = Describe("Ask engine evidence behind a figure", func() {
	var cat *askCatalogue

	BeforeEach(func() {
		var err error
		cat, err = askGoldenCatalogue()
		Expect(err).NotTo(HaveOccurred())
	})

	plan := func(sql string, columns ...string) (*askEvidencePlan, string, *askValidated) {
		v, rej := askValidate(sql, cat, nil)
		Expect(rej).To(BeNil(), sql)
		p, reason := askEvidencePlanOf(v, columns)
		return p, reason, v
	}
	passes := func(p *askEvidencePlan, sql string) {
		_, rej := askValidate(sql, cat, []string{p.View})
		Expect(rej).To(BeNil(), sql)
	}

	It("plans a maximum as an aggregate whose extreme row comes first, and the statements pass the validator", func() {
		p, reason, _ := plan("SELECT max(duration_s) AS longest FROM v_comm_events WHERE event_type = 'voice'", "longest")
		Expect(reason).To(BeEmpty())
		Expect(p.Kind).To(Equal("aggregate"))
		Expect(p.View).To(Equal("v_comm_events"))
		Expect(p.Aggs).To(HaveLen(1))
		Expect(p.Aggs[0].Fn).To(Equal("max"))
		Expect(p.Aggs[0].Column).To(Equal("duration_s"))
		Expect(p.extremeOrder()).To(Equal("duration_s DESC NULLS LAST"))
		rows := p.rowsSQL("", p.extremeOrder(), 25)
		Expect(rows).To(ContainSubstring("SELECT event_id::text AS k, start_utc, event_type, direction, src, dst, duration_s FROM v_comm_events WHERE"))
		Expect(rows).To(ContainSubstring("event_type = 'voice'"))
		Expect(rows).To(ContainSubstring("ORDER BY duration_s DESC NULLS LAST, event_id LIMIT 25"))
		passes(p, rows)
		files := p.filesSQL("")
		Expect(files).To(ContainSubstring("max(duration_s) AS a0"))
		Expect(files).To(ContainSubstring("GROUP BY evidence_id"))
		passes(p, files)
	})

	It("plans a count and an average, and writes the pieces that add up to them", func() {
		p, reason, _ := plan("SELECT count(*) AS n FROM v_comm_events WHERE src = '923000000101'", "n")
		Expect(reason).To(BeEmpty())
		Expect(p.Aggs[0].Fn).To(Equal("count"))
		Expect(p.extremeOrder()).To(BeEmpty(), "a count has no extreme row")
		passes(p, p.filesSQL(""))
		passes(p, p.rowsSQL("", "", 25))

		avg, reason, _ := plan("SELECT avg(duration_s) AS mean FROM v_comm_events WHERE event_type = 'voice'", "mean")
		Expect(reason).To(BeEmpty())
		files := avg.filesSQL("")
		Expect(files).To(ContainSubstring("sum(duration_s) AS a0_s"))
		Expect(files).To(ContainSubstring("count(duration_s) AS a0_c"))
		passes(avg, files)
	})

	It("plans a list as the rows of the answer, in the answer's order, within its limit", func() {
		p, reason, _ := plan("SELECT src AS subscriber, dst AS other_party, duration_s FROM v_comm_events WHERE event_type = 'voice' ORDER BY duration_s DESC, start_utc LIMIT 5", "subscriber", "other_party", "duration_s")
		Expect(reason).To(BeEmpty())
		Expect(p.Kind).To(Equal("rows"))
		Expect(p.Limit).To(Equal(5))
		Expect(p.Order).To(ContainSubstring("duration_s DESC"))
		rows := p.rowsSQL("", p.Order, 5)
		Expect(rows).To(ContainSubstring("LIMIT 5"))
		passes(p, rows)
	})

	It("plans a ranking as the rows of its first group, and writes the group's value as a safe condition", func() {
		p, reason, _ := plan("SELECT src AS number, count(*) AS calls FROM v_comm_events WHERE event_type = 'voice' AND direction = 'outgoing' GROUP BY src ORDER BY calls DESC LIMIT 5", "number", "calls")
		Expect(reason).To(BeEmpty())
		Expect(p.Kind).To(Equal("group"))
		Expect(p.GroupCols).To(Equal([]string{"src"}))
		Expect(p.GroupIndex).To(Equal([]int{0}))
		literal, ok := askEvidenceLiteral("923000000101")
		Expect(ok).To(BeTrue())
		rows := p.rowsSQL("src = "+literal, "", 25)
		Expect(rows).To(ContainSubstring("(event_type = 'voice' AND direction = 'outgoing') AND src = '923000000101'"))
		passes(p, rows)
		passes(p, p.filesSQL("src = "+literal))
		quoted, _ := askEvidenceLiteral("O'Brien")
		Expect(quoted).To(Equal("'O''Brien'"))
		_, ok = askEvidenceLiteral(1.5)
		Expect(ok).To(BeFalse(), "a value that cannot be written safely is not turned into a condition")
	})

	DescribeTable("says why it does not itemise, never guesses",
		func(sql, reason string, columns ...string) {
			p, got, _ := plan(sql, columns...)
			Expect(p).To(BeNil())
			Expect(got).To(ContainSubstring(reason))
		},
		Entry("an outer join of summaries", "SELECT x.dst AS number FROM v_contact_pairs x LEFT JOIN v_contact_pairs y ON y.dst = x.dst WHERE x.src = '923000000101' LIMIT 5", "cannot be traced", "number"),
		Entry("a join of a summary with a record view", "SELECT x.dst AS number FROM v_contact_pairs x JOIN v_comm_events e ON e.dst = x.dst WHERE x.src = '923000000101' LIMIT 5", "cannot be traced", "number"),
		Entry("a step-by-step query", "WITH a AS (SELECT dst FROM v_contact_pairs WHERE src = '923000000101') SELECT dst AS number FROM a LIMIT 5", "intermediate steps", "number"),
		Entry("a set operation", "SELECT dst AS number FROM v_comm_events WHERE src = '923000000101' INTERSECT SELECT dst FROM v_comm_events WHERE src = '923000000102' LIMIT 5", "combines two result sets", "number"),
		Entry("distinct values", "SELECT DISTINCT dst AS number FROM v_comm_events WHERE src = '923000000101' LIMIT 50", "removes repeated values", "number"),
		Entry("a group by a computed day", "SELECT date_trunc('day', start_utc) AS t, count(*) AS value FROM v_comm_events GROUP BY 1 ORDER BY value DESC LIMIT 5", "computed value", "t", "value"),
		Entry("a total mixed with a single value", "SELECT src, count(*) AS n FROM v_comm_events WHERE event_type = 'voice' LIMIT 5", "mixes a total", "src", "n"),
	)

	It("recomputes a count, a maximum and an average from the pieces, and says when they do not agree", func() {
		p, _, _ := plan("SELECT count(*) AS n, max(duration_s) AS longest FROM v_comm_events WHERE event_type = 'voice'", "n", "longest")
		files := &govSQLResult{Columns: []string{"f", "n", "a0", "a1"}, Rows: [][]any{{"file-1", int64(30), int64(30), int64(900)}, {"file-2", int64(12), int64(12), int64(1800)}}}
		agree := &askEvidence{State: "itemised"}
		askEvidenceReconcile(agree, p, &govSQLResult{Columns: []string{"n", "longest"}, Rows: [][]any{{int64(42), int64(1800)}}}, files)
		Expect(agree.Reconciled).To(Equal("yes"))
		Expect(agree.Total).To(Equal(42))
		Expect(agree.Check).To(ContainSubstring("the number of records is 42"))
		Expect(agree.Check).To(ContainSubstring("the largest duration (seconds) is 1,800"), "what was recomputed is said in an analyst's words, not as max(duration_s)")
		Expect(agree.Check).NotTo(ContainSubstring("duration_s"))
		Expect(agree.Files).To(HaveLen(2))

		disagree := &askEvidence{State: "itemised"}
		askEvidenceReconcile(disagree, p, &govSQLResult{Columns: []string{"n", "longest"}, Rows: [][]any{{int64(43), int64(1800)}}}, files)
		Expect(disagree.Reconciled).To(Equal("no"))
		Expect(disagree.Check).To(ContainSubstring("do not give the figure"))

		mean, _, _ := plan("SELECT avg(duration_s) AS mean FROM v_comm_events WHERE event_type = 'voice'", "mean")
		pieces := &govSQLResult{Columns: []string{"f", "n", "a0_s", "a0_c"}, Rows: [][]any{{"file-1", int64(2), int64(10), int64(2)}, {"file-2", int64(2), int64(20), int64(2)}}}
		avgOK := &askEvidence{State: "itemised"}
		askEvidenceReconcile(avgOK, mean, &govSQLResult{Columns: []string{"mean"}, Rows: [][]any{{float64(7.5)}}}, pieces)
		Expect(avgOK.Reconciled).To(Equal("yes"))
	})

	It("does not check a figure it cannot recompute, and says so", func() {
		p, _, _ := plan("SELECT count(DISTINCT dst) AS n FROM v_comm_events WHERE src = '923000000101'", "n")
		Expect(p.Aggs[0].Plain).To(BeFalse())
		ev := &askEvidence{State: "itemised"}
		askEvidenceReconcile(ev, p, &govSQLResult{Columns: []string{"n"}, Rows: [][]any{{int64(7)}}}, &govSQLResult{Columns: []string{"f", "n"}, Rows: [][]any{{"file-1", int64(9)}}})
		Expect(ev.Reconciled).To(Equal("not_checked"))
		Expect(ev.Total).To(Equal(9))
	})

	It("puts what the workspace shows into the response: an openable source for a located row, none for a row that could not be found, the files and one plain sentence", func() {
		resp := &hybridQueryResponse{Enterprise: map[string]any{"limitations": []string{"first note"}, "provenance": []map[string]any{{"source": "analyst_views_sql"}}}}
		ev := &askEvidence{State: "itemised", Kind: "aggregate", Total: 42, Reconciled: "yes", Check: "Recomputed from the 42 matching rows in 2 files: count(*) = 42.",
			Files: []askEvidenceFile{{EvidenceID: "e1", SourceFile: "a.xlsx", Rows: 30}, {EvidenceID: "e2", SourceFile: "b.xlsx", Rows: 12}},
			Rows: []askEvidenceRow{
				{Key: "1", EvidenceID: "e1", SourceFile: "a.xlsx", VersionID: "v1", SourceRow: 1, SheetRow: 5, RowHash: "abc", Preview: "MSISDN 923000000101"},
				{Key: "2", EvidenceID: "e2", SourceFile: "b.xlsx", VersionID: "v2", SourceRow: 3},
			}}
		askAttachEvidence(resp, ev)
		Expect(resp.Enterprise["evidence"]).To(Equal(ev))
		provenance := resp.Enterprise["provenance"].([]map[string]any)
		Expect(provenance).To(HaveLen(3), "the engine's own item, one openable row, one lineage")
		Expect(provenance[1]["row_number"]).To(Equal(5))
		Expect(provenance[1]["row_hash"]).To(Equal("abc"))
		Expect(provenance[1]["version_id"]).To(Equal("v1"))
		Expect(provenance[1]["source_truth_state"]).To(Equal("ingested_source_record"))
		Expect(provenance[2]["contract_version"]).To(Equal("forensics.contribution-lineage/v1"))
		Expect(provenance[2]["contribution_count"]).To(Equal(42))
		notes := resp.Enterprise["limitations"].([]string)
		Expect(notes).To(HaveLen(2))
		Expect(notes[1]).To(HavePrefix("42 rows in 2 source files are behind this answer."))

		none := &hybridQueryResponse{Enterprise: map[string]any{"limitations": []string{"first note"}}}
		askAttachEvidence(none, askEvidenceNone("the answer reads v_contact_pairs, which summarises many records"))
		Expect(strings.Join(none.Enterprise["limitations"].([]string), " ")).To(ContainSubstring("not itemised: the answer reads v_contact_pairs"))
		askAttachEvidence(none, nil)
		askAttachEvidence(nil, ev)
	})

	It("shows what each row is from the columns of the engine's own view, and withholds no identifier through it", func() {
		for view, source := range askEvidenceViews {
			Expect(source.Preview).NotTo(BeEmpty(), view)
			for _, column := range source.Preview {
				Expect(column).NotTo(Equal("value"), view+": a document value may be a withheld identifier")
				Expect(column).NotTo(Equal("subscriber_ref"), view+": a subscriber reference is an identity column")
			}
			known := map[string]bool{}
			for _, c := range cat.View(view).Columns {
				known[c.Name] = true
			}
			for _, column := range source.Preview {
				Expect(known[column]).To(BeTrue(), view+" has no column "+column)
			}
		}
	})
})
