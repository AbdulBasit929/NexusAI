package main

import (
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The records behind the lines of a summary (ask_engine_trace.go, ask_engine_evidence_lines.go).
var _ = Describe("Ask engine evidence for the lines of a summary", func() {
	var cat *askCatalogue

	BeforeEach(func() {
		var err error
		cat, err = askGoldenCatalogue()
		Expect(err).NotTo(HaveOccurred())
	})

	It("accounts for every view of the catalogue: a record view, a traced summary, or declared untraceable with its reason", func() {
		var names []string
		for _, v := range cat.Views {
			names = append(names, v.Name)
		}
		Expect(names).NotTo(BeEmpty())
		Expect(askTraceMissing(names)).To(BeEmpty(), "a view of the catalogue has no rule for the records behind its lines (ask_engine_trace.go)")
		for name := range askTraces {
			Expect(cat.View(name)).NotTo(BeNil(), name+" is traced but is not a view of the catalogue")
			_, isRecord := askEvidenceViews[name]
			Expect(isRecord).To(BeFalse(), name+" is both a record view and a traced summary")
		}
		for name, reason := range askUntraceable {
			Expect(reason).NotTo(BeEmpty(), name)
		}
	})

	It("names only columns that exist: the grain and the extras of a view, the raw view of each step and the columns its checks compare", func() {
		for _, name := range askTraceViewNames() {
			trace := askTraces[name]
			view := cat.View(name)
			have := map[string]bool{}
			for _, c := range view.Columns {
				have[c.Name] = true
			}
			Expect(trace.Grain).NotTo(BeEmpty(), name)
			for _, c := range append(append([]string{}, trace.Grain...), trace.Extra...) {
				Expect(have[c]).To(BeTrue(), name+" has no column "+c)
			}
			Expect(trace.Pick).NotTo(BeNil(), name)
			Expect(trace.Label).NotTo(BeNil(), name)
			step, ok := trace.Pick(askSampleLine(trace))
			if name == "v_document_links" {
				step, ok = askDocumentLinkStep(map[string]string{"kind": "msisdn", "family": "calls"})
			}
			Expect(ok).To(BeTrue(), name)
			if step.Raw != "" {
				_, isRecord := askEvidenceViews[step.Raw]
				Expect(isRecord).To(BeTrue(), name+": "+step.Raw+" is not a record view")
				Expect(step.Cond).NotTo(BeEmpty(), name)
			}
			for _, check := range step.Checks {
				Expect(have[check.Column]).To(BeTrue(), name+": a check compares column "+check.Column+" which the view lacks")
			}
			Expect(step.Say).NotTo(BeEmpty(), name)
		}
	})

	It("writes statements that the engine's own validator accepts, for every traced view and every family of a document link", func() {
		type probe struct{ view, kind, family string }
		var probes []probe
		for _, name := range askTraceViewNames() {
			if name == "v_document_links" {
				for _, k := range []string{"msisdn/calls", "msisdn/own_calls", "msisdn/registrations", "msisdn/wallet", "account/statements", "plate/sightings", "iccid/registrations", "imsi/registrations", "imei/calls"} {
					parts := strings.Split(k, "/")
					probes = append(probes, probe{name, parts[0], parts[1]})
				}
				continue
			}
			probes = append(probes, probe{view: name})
		}
		for _, p := range probes {
			trace := askTraces[p.view]
			sql := "SELECT " + strings.Join(trace.Grain, ", ") + " FROM " + p.view + " LIMIT 3"
			v, rej := askValidate(sql, cat, nil)
			Expect(rej).To(BeNil(), sql)
			plan, reason := askEvidencePlanOf(v, trace.Grain)
			Expect(reason).To(BeEmpty(), p.view)
			Expect(plan.Kind).To(Equal("lines"), p.view)
			views := []string{p.view}
			_, rej = askValidate(plan.linesSQL(false, 10), cat, views)
			Expect(rej).To(BeNil(), plan.linesSQL(false, 10))
			_, rej = askValidate(plan.lineCountSQL(), cat, views)
			Expect(rej).To(BeNil(), plan.lineCountSQL())
			line := askSampleLine(trace)
			if p.view == "v_document_links" {
				line["kind"], line["family"] = p.kind, p.family
			}
			steps, why := askLineSteps(plan, []map[string]string{line})
			Expect(why).To(BeEmpty(), p.view)
			if steps[0].Raw == "" {
				continue // the files themselves: nothing to count
			}
			raw := askEvidenceViews[steps[0].Raw]
			count, _ := askLineCountStatement(plan, false, 0, steps, raw)
			_, rej = askValidate(count, cat, askLineViews(plan, steps))
			Expect(rej).To(BeNil(), p.view+" (count): "+count)
			rows, files := askLineRecordStatements(plan, false, 2, steps, raw)
			_, rej = askValidate(rows, cat, askLineViews(plan, steps))
			Expect(rej).To(BeNil(), p.view+" (records): "+rows)
			_, rej = askValidate(files, cat, askLineViews(plan, steps))
			Expect(rej).To(BeNil(), p.view+" (files): "+files)
		}
	})

	It("plans the common contacts of two numbers as lines over both sides of the join, and traces a line to the records of each side", func() {
		sql := "SELECT x.dst AS number, x.events AS events_first, y.events AS events_second FROM v_contact_pairs x JOIN v_contact_pairs y ON y.dst = x.dst WHERE x.src = '923000000101' AND y.src = '923000000102' AND NOT x.is_service AND NOT x.is_hub ORDER BY x.events + y.events DESC LIMIT 50"
		v, rej := askValidate(sql, cat, nil)
		Expect(rej).To(BeNil())
		plan, reason := askEvidencePlanOf(v, []string{"number", "events_first", "events_second"})
		Expect(reason).To(BeEmpty())
		Expect(plan.Kind).To(Equal("lines"))
		Expect(plan.RowsAreLines).To(BeTrue())
		Expect(plan.Aliases).To(HaveLen(2))
		Expect([]string{plan.Aliases[0].Name, plan.Aliases[1].Name}).To(Equal([]string{"x", "y"}))
		Expect(plan.FromSQL).To(ContainSubstring("JOIN"))
		views := []string{"v_contact_pairs"}
		_, rej = askValidate(plan.linesSQL(false, 10), cat, views)
		Expect(rej).To(BeNil(), plan.linesSQL(false, 10))
		lines := []map[string]string{{"src": "923000000101", "dst": "923000000099"}, {"src": "923000000102", "dst": "923000000099"}}
		steps, why := askLineSteps(plan, lines)
		Expect(why).To(BeEmpty())
		cond := askLineCond(plan, steps)
		Expect(cond).To(ContainSubstring("(r.src = l.x__src AND r.dst = l.x__dst) OR (r.src = l.y__src AND r.dst = l.y__dst)"))
		count, checks := askLineCountStatement(plan, false, 0, steps, askEvidenceViews["v_comm_events"])
		Expect(checks).To(BeEmpty(), "two sides have no single set of numbers to compare")
		_, rej = askValidate(count, cat, askLineViews(plan, steps))
		Expect(rej).To(BeNil(), count)
	})

	It("knows when the answer is the number of lines, and when it lists them", func() {
		counted, reason := askEvidencePlanOfSQL(cat, "SELECT count(*) AS n FROM v_contact_pairs WHERE src = '923000000101' AND NOT is_service", "n")
		Expect(reason).To(BeEmpty())
		Expect(counted.CountStar).To(BeTrue())
		Expect(counted.RowsAreLines).To(BeFalse())
		listed, _ := askEvidencePlanOfSQL(cat, "SELECT dst AS number, events FROM v_contact_pairs WHERE src = '923000000101' ORDER BY events DESC LIMIT 5", "number", "events")
		Expect(listed.RowsAreLines).To(BeTrue())
		Expect(listed.Order).To(ContainSubstring("events DESC"))
		grouped, _ := askEvidencePlanOfSQL(cat, "SELECT dst, sum(events) AS n FROM v_contact_pairs WHERE src = '923000000101' GROUP BY dst ORDER BY n DESC LIMIT 5", "dst", "n")
		Expect(grouped.CountStar).To(BeFalse())
		Expect(grouped.RowsAreLines).To(BeFalse())
		Expect(grouped.linesSQL(false, 10)).To(ContainSubstring("ORDER BY v_contact_pairs.src, v_contact_pairs.dst"), "a grouped answer lists the summary rows its conditions select, in the order of their keys")
	})

	It("still says why it does not itemise a search through several steps or a join of other kinds", func() {
		_, reason := askEvidencePlanOfSQL(cat, "SELECT x.dst AS number FROM v_contact_pairs x LEFT JOIN v_contact_pairs y ON y.dst = x.dst WHERE x.src = '923000000101' LIMIT 5", "number")
		Expect(reason).To(ContainSubstring("cannot be traced"))
		_, reason = askEvidencePlanOfSQL(cat, "SELECT x.dst AS number FROM v_contact_pairs x JOIN v_comm_events e ON e.dst = x.dst WHERE x.src = '923000000101' LIMIT 5", "number")
		Expect(reason).NotTo(BeEmpty(), "a join of a summary with a record view is not planned")
	})

	It("keeps the lines of an answer in memory for the scope that asked, and only for it", func() {
		session := &askLineSession{Tenant: "t", Collection: "c", Lines: [][]map[string]string{{{"src": "a", "dst": "b"}}}, Labels: []string{"a → b"}}
		token := askLineSessionStore(session)
		Expect(token).To(HaveLen(32))
		Expect(askLineSessionFind(token)).To(BeIdenticalTo(session))
		Expect(askLineSessionFind("00000000000000000000000000000000")).To(BeNil())
		get := func(query string) *httptest.ResponseRecorder {
			rec := httptest.NewRecorder()
			askLineHandler(nil)(rec, httptest.NewRequest(http.MethodGet, "/query/evidence-line?"+query, nil))
			return rec
		}
		Expect(get("token="+token+"&line=1&tenant_id=t&collection_id=c").Code).To(Equal(http.StatusServiceUnavailable), "no database in this spec: the scope check is not reached")
	})
})

// askSampleLine gives a line the values a spec needs to build the statements of a trace: a text for every column.
func askSampleLine(trace askTrace) map[string]string {
	line := map[string]string{}
	for _, c := range append(append([]string{}, trace.Grain...), trace.Extra...) {
		line[c] = "x"
	}
	return line
}

func askEvidencePlanOfSQL(cat *askCatalogue, sql string, columns ...string) (*askEvidencePlan, string) {
	v, rej := askValidate(sql, cat, nil)
	if rej != nil {
		return nil, "rejected: " + rej.Message
	}
	return askEvidencePlanOf(v, columns)
}

var _ = sort.Strings
