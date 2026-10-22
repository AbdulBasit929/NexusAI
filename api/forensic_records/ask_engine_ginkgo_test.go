package main

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The Ask engine's validator is the security gate for model-written SQL, so it is tested the way a gate is: queries that must pass, and a longer list of
// hostile or malformed ones that must not, each with the reason code it must be refused for. The catalogue is the pinned snapshot of the analyst views.
var _ = Describe("Ask engine validator", func() {
	var cat *askCatalogue
	BeforeEach(func() {
		var err error
		cat, err = askGoldenCatalogue()
		Expect(err).NotTo(HaveOccurred())
	})

	recursive := func(where, target, tail string) string {
		return "WITH RECURSIVE walk AS (SELECT src, dst, 1 AS depth, ARRAY[src, dst] AS path FROM v_contact_pairs WHERE src = '923000000101' " +
			"UNION ALL SELECT w.src, e.dst, " + target + ", w.path || e.dst FROM walk w JOIN v_contact_pairs e ON e.src = w.dst WHERE " + where + ") " +
			"SELECT src, dst FROM walk WHERE dst = '923000000102' " + tail
	}

	accepted := []string{
		"SELECT count(*) AS n FROM v_comm_events",
		"SELECT * FROM analyst.v_cells LIMIT 10",
		"SELECT dst FROM v_contact_pairs WHERE src = '923000000101' INTERSECT SELECT dst FROM v_contact_pairs WHERE src = '923000000102'",
		"SELECT v.src, c.site_name, sum(v.events) AS visits FROM v_cell_visits v JOIN v_cells c ON c.cell_key = v.cell_key GROUP BY v.src, c.site_name LIMIT 100",
		"SELECT a.src, b.imei FROM v_comm_events a LEFT JOIN v_device_pairings b USING (src) LIMIT 20",
		"SELECT a.src FROM v_comm_events a, v_device_pairings b WHERE a.src = b.src LIMIT 20",
		"SELECT src FROM v_comm_events WHERE dst IN (SELECT dst FROM v_contact_pairs WHERE is_hub) LIMIT 10",
		"WITH t AS (SELECT src, count(*) AS c FROM v_comm_events GROUP BY src) SELECT src, c FROM t WHERE c > 5 ORDER BY c DESC LIMIT 5",
		"WITH t AS (SELECT src, count(*) FROM v_comm_events GROUP BY src) SELECT src, count FROM t",
		"SELECT i, cell_key FROM v_cells, generate_series(1, 3) AS i LIMIT 9",
		"SELECT src FROM v_comm_events WHERE dst = ANY (ARRAY['923000000101', '923000000102']) LIMIT 10",
		"SELECT src, row_number() OVER (PARTITION BY src ORDER BY start_utc) AS n FROM v_comm_events LIMIT 50",
		"SELECT src FROM v_comm_events LIMIT 5000",
		"SELECT coalesce(site_name, 'unknown') AS site, greatest(lat, 0) AS la FROM v_cells",
		"SELECT src, nullif(dst, '') AS other FROM v_comm_events LIMIT 5",
		recursive("w.depth <= 3 AND NOT e.dst = ANY (w.path)", "w.depth + 1", "LIMIT 20"),
	}
	for i, q := range accepted {
		q := q
		It("accepts query "+string(rune('A'+i)), func() {
			valid, rej := askValidate(q, cat, nil)
			Expect(rej).To(BeNil(), q)
			Expect(valid.SQL).NotTo(BeEmpty())
			Expect(valid.Views).NotTo(BeEmpty())
		})
	}

	rejected := []struct{ code, sql string }{
		{"EMPTY", "   "},
		{"SYNTAX", "selec * fromm v_cells"},
		{"ONE_STATEMENT", "SELECT 1 FROM v_cells; SELECT 2 FROM v_cells"},
		{"SELECT_ONLY", "DELETE FROM v_cells"},
		{"SELECT_ONLY", "INSERT INTO v_cells (cell_key) VALUES ('x')"},
		{"SELECT_ONLY", "SET ROLE forensic_runtime"},
		{"SELECT_ONLY", "RESET ROLE"},
		{"SELECT_ONLY", "EXPLAIN ANALYZE SELECT * FROM v_cells"},
		{"SELECT_ONLY", "COPY v_cells TO '/tmp/x'"},
		{"SELECT_ONLY", "CREATE TABLE x (a int)"},
		{"NO_SCHEMA", "SELECT * FROM forensic.comm_event"},
		{"NO_SCHEMA", "SELECT * FROM pg_catalog.pg_roles"},
		{"NO_SCHEMA", "SELECT * FROM information_schema.tables"},
		{"UNKNOWN_RELATION", "SELECT * FROM pg_roles"},
		{"UNKNOWN_RELATION", "SELECT * FROM comm_event"},
		{"UNKNOWN_RELATION", "SELECT * FROM v_missing"},
		{"NO_VIEW", "SELECT 1"},
		{"NO_VIEW", "SELECT * FROM generate_series(1, 10)"},
		{"CROSS_PRODUCT", "SELECT * FROM v_comm_events a, v_cells b LIMIT 5"},
		{"CROSS_PRODUCT", "SELECT * FROM v_comm_events a CROSS JOIN v_cells b LIMIT 5"},
		{"CROSS_PRODUCT", "SELECT * FROM v_comm_events a NATURAL JOIN v_cells b LIMIT 5"},
		{"CROSS_PRODUCT", "SELECT * FROM v_comm_events a JOIN v_cells b ON true LIMIT 5"},
		{"CROSS_PRODUCT", "SELECT * FROM v_comm_events a JOIN v_cells b ON 1 = 1 LIMIT 5"},
		{"CROSS_PRODUCT", "SELECT * FROM v_comm_events a, v_cells b WHERE 1 = 1 LIMIT 5"},
		{"FUNCTION", "SELECT pg_sleep(5) FROM v_cells"},
		{"FUNCTION", "SELECT pg_read_file('/etc/passwd') FROM v_cells"},
		{"FUNCTION", "SELECT current_setting('app.tenant_id') FROM v_cells"},
		{"FUNCTION", "SELECT set_config('app.tenant_id', 'x', false) FROM v_cells"},
		{"FUNCTION", "SELECT generate_series(1, 5) FROM v_cells"},
		{"FUNCTION", "SELECT pg_catalog.pg_backend_pid() FROM v_cells"},
		{"FUNCTION", "SELECT lo_import('/etc/passwd') FROM v_cells"},
		{"RANGE_FUNCTION", "SELECT * FROM v_cells, generate_series(1, 1000000) AS i LIMIT 5"},
		{"RANGE_FUNCTION", "SELECT * FROM v_cells, pg_ls_dir('.') AS d LIMIT 5"},
		{"OPERATOR", "SELECT cell_key OPERATOR(pg_catalog.||) 'x' FROM v_cells"},
		{"OPERATOR", "SELECT cell_key @@ 'x' FROM v_cells"},
		{"TYPE", "SELECT cell_key::regclass FROM v_cells"},
		{"TYPE", "SELECT ARRAY[cell_key]::text[] FROM v_cells"},
		{"VALUE_FUNCTION", "SELECT current_user FROM v_cells"},
		{"VALUE_FUNCTION", "SELECT session_user FROM v_cells"},
		{"NO_INTO", "SELECT * INTO x FROM v_cells"},
		{"NO_LOCKING", "SELECT * FROM v_cells FOR UPDATE"},
		{"NO_VALUES", "SELECT * FROM (VALUES (1), (2)) t"},
		{"NOT_ALLOWED", "SELECT * FROM v_cells TABLESAMPLE BERNOULLI (10)"},
		{"LIMIT", "SELECT * FROM v_cells LIMIT 5001"},
		{"LIMIT", "SELECT * FROM v_cells LIMIT 0"},
		{"LIMIT", "SELECT * FROM v_cells LIMIT (SELECT 5)"},
		{"UNKNOWN_COLUMN", "SELECT nonsense FROM v_cells"},
		{"UNKNOWN_COLUMN", "SELECT x.nonsense FROM v_cells x"},
		{"NO_QUALIFIED_COLUMN", "SELECT analyst.v_cells.cell_key FROM v_cells"},
		{"CTE_NAME", "WITH v_cells AS (SELECT cell_key FROM v_cells) SELECT * FROM v_cells"},
		{"CTE_NAME", "WITH pg_x AS (SELECT cell_key FROM v_cells) SELECT * FROM pg_x"},
		{"CONSTANT", "SELECT B'101' FROM v_cells"},
		{"CONSTANT", "SELECT '" + strings.Repeat("a", 401) + "' FROM v_cells"},
		{"TOO_LONG", "SELECT '" + strings.Repeat("a", 7000) + "' FROM v_cells"},
		{"TOO_COMPLEX", "SELECT " + strings.Repeat("(SELECT ", 14) + "1" + strings.Repeat(")", 14) + " FROM v_cells"},
		// recursion
		{"REC_NO_DEPTH_BOUND", recursive("NOT e.dst = ANY (w.path)", "w.depth + 1", "LIMIT 20")},
		{"REC_DEPTH", recursive("w.depth < 5 AND NOT e.dst = ANY (w.path)", "w.depth + 1", "LIMIT 20")},
		{"REC_DEPTH", recursive("w.depth <= 4 AND NOT e.dst = ANY (w.path)", "w.depth + 1", "LIMIT 20")},
		{"REC_DEPTH_COLUMN", recursive("w.depth < 3 AND NOT e.dst = ANY (w.path)", "w.depth", "LIMIT 20")},
		{"REC_DEPTH_COLUMN", recursive("w.depth < 3 AND NOT e.dst = ANY (w.path)", "w.depth - 1", "LIMIT 20")},
		{"REC_DEPTH_COLUMN", recursive("w.depth < 3 AND NOT e.dst = ANY (w.path)", "w.depth + 0", "LIMIT 20")},
		{"REC_CYCLE_GUARD", recursive("w.depth < 3", "w.depth + 1", "LIMIT 20")},
		{"REC_LIMIT", recursive("w.depth < 3 AND NOT e.dst = ANY (w.path)", "w.depth + 1", "")},
		{"REC_SHAPE", "WITH RECURSIVE walk AS (SELECT src, dst, 1 AS depth FROM walk UNION ALL SELECT w.src, e.dst, w.depth + 1 FROM walk w JOIN v_contact_pairs e ON e.src = w.dst WHERE w.depth < 3) SELECT * FROM walk LIMIT 5"},
		{"LIMIT", recursive("w.depth < 3 AND NOT e.dst = ANY (w.path)", "w.depth + 1", "LIMIT 9999")},
	}
	for i, r := range rejected {
		r := r
		It("refuses query "+string(rune('a'+i%26))+string(rune('0'+i/26))+" with "+r.code, func() {
			valid, rej := askValidate(r.sql, cat, nil)
			Expect(valid).To(BeNil(), r.sql)
			Expect(rej).NotTo(BeNil(), r.sql)
			Expect(rej.Code).To(Equal(r.code), r.sql+" -> "+rej.Message)
		})
	}

	It("offers only the views of the shortlist", func() {
		_, rej := askValidate("SELECT src FROM v_comm_events LIMIT 5", cat, []string{"v_cells"})
		Expect(rej).NotTo(BeNil())
		Expect(rej.Code).To(Equal("UNKNOWN_RELATION"))
		Expect(rej.Message).To(ContainSubstring("v_cells"))
		Expect(rej.Message).NotTo(ContainSubstring("v_comm_events,"))
	})

	It("tells the model the real columns when one is wrong, so the repair needs no database round trip", func() {
		_, rej := askValidate("SELECT telephone FROM v_cells", cat, nil)
		Expect(rej).NotTo(BeNil())
		Expect(rej.Code).To(Equal("UNKNOWN_COLUMN"))
		Expect(rej.Message).To(ContainSubstring("radius_m"))
	})

	It("reports what a recursive query will do", func() {
		valid, rej := askValidate(recursive("w.depth < 3 AND NOT e.dst = ANY (w.path)", "w.depth + 1", "LIMIT 20"), cat, nil)
		Expect(rej).To(BeNil())
		Expect(valid.Recursive).To(BeTrue())
		Expect(valid.MaxHops).To(Equal(3))
		Expect(valid.Limit).To(Equal(20))
		Expect(valid.Views).To(Equal([]string{"v_contact_pairs"}))
		hops4, rej := askValidate(recursive("w.depth <= 3 AND NOT e.dst = ANY (w.path)", "w.depth + 1", "LIMIT 20"), cat, nil)
		Expect(rej).To(BeNil())
		Expect(hops4.MaxHops).To(Equal(4))
	})

	It("executes only the parser's own canonical text", func() {
		valid, rej := askValidate("select   src  /* hidden */ from v_cells_x_does_not_matter_when_it_fails", cat, nil)
		Expect(valid).To(BeNil())
		Expect(rej).NotTo(BeNil())
		valid, rej = askValidate("select   count(*)  /* a comment */  from   v_cells   -- trailing", cat, nil)
		Expect(rej).To(BeNil())
		Expect(valid.SQL).NotTo(ContainSubstring("comment"))
		Expect(valid.SQL).NotTo(ContainSubstring("trailing"))
	})

	It("records the constants of the query so that the checks can compare them with the question", func() {
		valid, rej := askValidate("SELECT src FROM v_comm_events WHERE dst = '923000000101' AND duration_s > 60 LIMIT 5", cat, nil)
		Expect(rej).To(BeNil())
		texts := []string{}
		for _, c := range valid.Consts {
			texts = append(texts, c.Text)
		}
		Expect(texts).To(ContainElements("923000000101", "60", "5"))
	})
})

var _ = Describe("Ask engine catalogue, examples and prompt", func() {
	It("has a complete pinned catalogue of the twenty-one analyst views", func() {
		cat, err := askGoldenCatalogue()
		Expect(err).NotTo(HaveOccurred())
		names := []string{}
		for _, v := range cat.Views {
			names = append(names, v.Name)
		}
		Expect(names).To(Equal([]string{"v_anchor_changes", "v_anchor_deviations", "v_anchor_runs", "v_app_usage", "v_cell_visits", "v_cells", "v_colocation_windows", "v_comm_events", "v_connection_edges", "v_contact_links", "v_contact_pairs", "v_data_sessions", "v_device_pairings", "v_evidence_inventory", "v_meeting_candidates", "v_movement_anchors", "v_rhythm_changes", "v_silent_numbers", "v_speed_alerts", "v_successor_candidates", "v_weekly_anchors"}))
		Expect(cat.View("v_contact_pairs").Columns).NotTo(BeEmpty())
		Expect(cat.View("v_nothing")).To(BeNil())
	})

	It("refuses an incomplete catalogue: no views, a missing description, a duplicate", func() {
		Expect((&askCatalogue{}).Check()).NotTo(Succeed())
		Expect((*askCatalogue)(nil).Check()).NotTo(Succeed())
		col := askColumn{Name: "a", Type: "text", Description: "d"}
		Expect((&askCatalogue{Views: []askView{{Name: "v", Description: "", Columns: []askColumn{col}}}}).Check()).NotTo(Succeed())
		Expect((&askCatalogue{Views: []askView{{Name: "v", Description: "d", Columns: []askColumn{{Name: "a", Type: "text"}}}}}).Check()).NotTo(Succeed())
		Expect((&askCatalogue{Views: []askView{{Name: "v", Description: "d"}}}).Check()).NotTo(Succeed())
		Expect((&askCatalogue{Views: []askView{{Name: "v", Description: "d", Columns: []askColumn{col}}, {Name: "v", Description: "d", Columns: []askColumn{col}}}}).Check()).NotTo(Succeed())
		Expect((&askCatalogue{Views: []askView{{Name: "v", Description: "d", Columns: []askColumn{col}}}}).Check()).To(Succeed())
	})

	It("round-trips the snapshot byte for byte", func() {
		cat, err := askGoldenCatalogue()
		Expect(err).NotTo(HaveOccurred())
		again, err := cat.JSON()
		Expect(err).NotTo(HaveOccurred())
		Expect(string(again)).To(Equal(strings.ReplaceAll(string(askCatalogueGolden), "\r\n", "\n")), "a checkout may turn line endings into CRLF; the content must not differ")
	})

	It("has verified examples that all pass the validator against their own views", func() {
		cat, err := askGoldenCatalogue()
		Expect(err).NotTo(HaveOccurred())
		examples, err := askExamples()
		Expect(err).NotTo(HaveOccurred())
		Expect(len(examples)).To(BeNumerically(">=", 6))
		shapes := map[string]bool{}
		for _, e := range examples {
			valid, rej := askValidate(e.SQL, cat, e.Views)
			Expect(rej).To(BeNil(), e.ID)
			Expect(valid.Views).To(ConsistOf(e.Views), e.ID)
			shapes[e.Shape] = true
		}
		for _, shape := range []string{"ranked", "table", "graph", "map", "series"} {
			Expect(shapes).To(HaveKey(shape))
		}
	})

	It("keeps the start of the prompt identical for every question, so the model server's prompt cache is reused", func() {
		cat, err := askGoldenCatalogue()
		Expect(err).NotTo(HaveOccurred())
		examples, err := askExamples()
		Expect(err).NotTo(HaveOccurred())
		a := askPrompt(cat, []string{"v_cells", "v_comm_events"}, examples[:1])
		b := askPrompt(cat, []string{"v_app_usage"}, nil)
		Expect(strings.HasPrefix(a, askPromptRules+"\n\nRelations:\n")).To(BeTrue())
		Expect(strings.HasPrefix(b, askPromptRules+"\n\nRelations:\n")).To(BeTrue())
		Expect(a).To(ContainSubstring("- v_cells:"))
		Expect(a).To(ContainSubstring("Verified example. Question:"))
		Expect(a).NotTo(ContainSubstring("v_app_usage"))
		Expect(b).NotTo(ContainSubstring("v_cells"))
		Expect(b).NotTo(ContainSubstring("Verified example"))
		Expect(askPrompt(cat, []string{"v_comm_events", "v_cells"}, nil)).To(Equal(askPrompt(cat, []string{"v_cells", "v_comm_events"}, nil)), "the shortlist's order does not change the prompt")
	})

	It("tells the model the conventions that make a result drawable", func() {
		for _, word := range []string{"src and dst", "lat and lon", "t and value", "label and value", "depth < 3", "is_hub", "Asia/Karachi", "answerable"} {
			Expect(askPromptRules).To(ContainSubstring(word))
		}
	})

	DescribeTable("names the shape of a result from its columns",
		func(columns []string, rows int, want string) {
			Expect(askShape(columns, rows)).To(Equal(want))
		},
		Entry("graph", []string{"src", "dst", "kind"}, 40, "graph"),
		Entry("graph beats map", []string{"src", "dst", "lat", "lon"}, 3, "graph"),
		Entry("lanes", []string{"lane", "start", "end", "label"}, 8, "lanes"),
		Entry("map", []string{"cell_key", "lat", "lon", "radius_m"}, 5, "map"),
		Entry("series", []string{"t", "value"}, 30, "series"),
		Entry("ranked list", []string{"label", "value"}, 5, "ranked"),
		Entry("scalar", []string{"n"}, 1, "scalar"),
		Entry("one column, many rows is a table", []string{"dst"}, 9, "table"),
		Entry("plain table", []string{"number", "events"}, 4, "table"),
		Entry("case does not matter", []string{"SRC", "Dst"}, 2, "graph"),
		Entry("a src without a dst is a table", []string{"src", "events"}, 2, "table"),
	)
})
