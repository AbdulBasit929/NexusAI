package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudler/LocalAI/pkg/forensicrequest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The executor, the probe and the whole run against a real database that holds a GENERATED case written by the production typed ingest, as the application role.
// They need the throwaway database of tools/case_generator/check_ask_engine.py (which also hands over the generator's planted-truth keys) and skip without one. The harness
// leaves a marker table in the database; the specs refuse any database that has none, so they cannot be pointed at a case database.
type askTruth struct {
	CaseID   string `json:"case_id"`
	Tenant   string `json:"tenant"`
	Subjects []struct {
		MSISDN string `json:"msisdn"`
	} `json:"subjects"`
	Oracle struct {
		A1 map[string][]struct {
			Number   string `json:"number"`
			Kind     string `json:"kind"`
			First    string `json:"first"`
			Last     string `json:"last"`
			Incoming int    `json:"incoming"`
			Outgoing int    `json:"outgoing"`
			Unknown  int    `json:"unknown"`
		} `json:"A1"`
	} `json:"oracle"`
}

var _ = Describe("Ask engine executor on a case database", Ordered, func() {
	var (
		pool   *pgxpool.Pool
		truth  askTruth
		ctx    = context.Background()
		limits = askDefaultLimits()
	)

	BeforeAll(func() {
		dsn := os.Getenv("ASK_ENGINE_CASE_DSN") //nolint:forbidigo // opt-in switch for a throwaway-database spec
		if dsn == "" {
			Skip("requires the throwaway database of tools/case_generator/check_ask_engine.py")
		}
		cfg, err := pgxpool.ParseConfig(dsn)
		Expect(err).NotTo(HaveOccurred())
		Expect(cfg.ConnConfig.Host).To(Equal("127.0.0.1"))
		pool, err = pgxpool.NewWithConfig(ctx, cfg)
		Expect(err).NotTo(HaveOccurred())
		var marker bool
		Expect(pool.QueryRow(ctx, "SELECT to_regclass('public.nx_throwaway_marker') IS NOT NULL").Scan(&marker)).To(Succeed())
		Expect(marker).To(BeTrue(), "this is not a throwaway database")
		raw, err := os.ReadFile(os.Getenv("ASK_ENGINE_CASE_TRUTH")) //nolint:forbidigo // set by the harness only
		Expect(err).NotTo(HaveOccurred())
		Expect(json.Unmarshal(raw, &truth)).To(Succeed())
		Expect(truth.Subjects).NotTo(BeEmpty())
		limits.Timeout = 20 * time.Second
	})

	AfterAll(func() {
		if pool != nil {
			pool.Close()
		}
	})

	validated := func(sql string) *askValidated {
		cat, err := loadAskCatalogue(ctx, pool)
		Expect(err).NotTo(HaveOccurred())
		v, rej := askValidate(sql, cat, nil)
		Expect(rej).To(BeNil(), sql)
		return v
	}
	scoped := func(sql string) (*govSQLResult, error) {
		return askExecute(ctx, pool, truth.Tenant, truth.CaseID, validated(sql), limits)
	}
	pair := func() (subject, other string, events int, first string) {
		for _, s := range truth.Subjects {
			for _, p := range truth.Oracle.A1[s.MSISDN] {
				if p.Kind == "person_like" && p.Outgoing+p.Incoming >= 3 {
					return s.MSISDN, p.Number, p.Outgoing + p.Incoming + p.Unknown, p.First
				}
			}
		}
		Fail("the case has no pair with three events")
		return
	}

	It("returns, through the analyst role, the counts of the planted-truth key", func() {
		for _, s := range truth.Subjects {
			for _, p := range truth.Oracle.A1[s.MSISDN] {
				if p.Kind != "person_like" {
					continue
				}
				result, err := scoped(fmt.Sprintf("SELECT outgoing, incoming, unknown FROM v_contact_pairs WHERE src = '%s' AND dst = '%s'", s.MSISDN, p.Number))
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Rows).To(HaveLen(1))
				Expect(fmt.Sprint(result.Rows[0][0], result.Rows[0][1], result.Rows[0][2])).To(Equal(fmt.Sprint(p.Outgoing, p.Incoming, p.Unknown)), s.MSISDN+" with "+p.Number)
			}
		}
	})

	It("reads and shows every time on the case clock", func() {
		subject, other, _, first := pair()
		result, err := scoped(fmt.Sprintf("SELECT first_seen FROM v_contact_pairs WHERE src = '%s' AND dst = '%s'", subject, other))
		Expect(err).NotTo(HaveOccurred())
		shown := fmt.Sprint(result.Rows[0][0])
		Expect(shown).To(ContainSubstring(first[:10]), "the key's first contact is written on the case clock: "+first)
		_, loc := govSQLTimeZone()
		Expect(loc.String()).To(Equal("Asia/Karachi"))
		hour, err := scoped(fmt.Sprintf("SELECT EXTRACT(HOUR FROM first_seen) AS h FROM v_contact_pairs WHERE src = '%s' AND dst = '%s'", subject, other))
		Expect(err).NotTo(HaveOccurred())
		wantHour, convErr := strconv.Atoi(first[11:13])
		Expect(convErr).NotTo(HaveOccurred())
		Expect(fmt.Sprint(hour.Rows[0][0])).To(Equal(fmt.Sprint(wantHour)), "an hour of day is the case clock's hour: "+first)
	})

	It("shows nothing of another tenant or another collection, and nothing when no scope is set", func() {
		v := validated("SELECT count(*) AS n FROM v_comm_events")
		own, err := askExecute(ctx, pool, truth.Tenant, truth.CaseID, v, limits)
		Expect(err).NotTo(HaveOccurred())
		Expect(fmt.Sprint(own.Rows[0][0])).NotTo(Equal("0"))
		for _, scope := range [][2]string{{"another-tenant", truth.CaseID}, {truth.Tenant, "another-collection"}, {"", ""}, {truth.Tenant, ""}} {
			other, err := askExecute(ctx, pool, scope[0], scope[1], v, limits)
			Expect(err).NotTo(HaveOccurred())
			Expect(fmt.Sprint(other.Rows[0][0])).To(Equal("0"), fmt.Sprintf("scope %q %q", scope[0], scope[1]))
		}
	})

	// The system catalogue is readable by every role (docs/data/analyst-views.md); the validator, not the role, keeps the model out of it.
	It("cannot reach a base table, whatever text reaches it", func() {
		for _, sql := range []string{"SELECT count(*) FROM forensic.comm_event", "SELECT count(*) FROM forensic.records", "SELECT count(*) FROM comm_event", "SELECT count(*) FROM public.nx_throwaway_marker"} {
			_, err := askRun(ctx, pool, truth.Tenant, truth.CaseID, sql, nil, limits, 10, false)
			Expect(err).To(HaveOccurred(), sql)
		}
	})

	It("refuses a query the planner prices above the ceiling, before it runs", func() {
		tight := limits
		tight.MaxPlanCost = 1
		_, err := askExecute(ctx, pool, truth.Tenant, truth.CaseID, validated("SELECT src, count(*) AS n FROM v_comm_events GROUP BY src"), tight)
		run, ok := err.(*govSQLRunError)
		Expect(ok).To(BeTrue())
		Expect(run.Code).To(Equal("TOO_EXPENSIVE"))
		Expect(run.Message).To(ContainSubstring("narrow it"))
	})

	It("caps the rows and says so, never silently", func() {
		small := limits
		small.RowCap = 5
		result, err := askExecute(ctx, pool, truth.Tenant, truth.CaseID, validated("SELECT event_id FROM v_comm_events"), small)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Rows).To(HaveLen(5))
		Expect(result.Truncated).To(BeTrue())
		exact, err := askExecute(ctx, pool, truth.Tenant, truth.CaseID, validated("SELECT event_id FROM v_comm_events LIMIT 5"), small)
		Expect(err).NotTo(HaveOccurred())
		Expect(exact.Rows).To(HaveLen(5))
		Expect(exact.Truncated).To(BeFalse())
	})

	It("stops a query that runs too long and reports it as too broad, not as an error", func() {
		quick := limits
		quick.Timeout = 300 * time.Millisecond
		_, err := askRun(ctx, pool, truth.Tenant, truth.CaseID, "SELECT pg_sleep(3)", nil, quick, 10, false)
		run, ok := err.(*govSQLRunError)
		Expect(ok).To(BeTrue(), fmt.Sprint(err))
		Expect(run.Code).To(Equal("TIMEOUT"))
	})

	It("tells a mistake the model can repair from a failure of the system", func() {
		_, err := askRun(ctx, pool, truth.Tenant, truth.CaseID, "SELECT nope FROM v_comm_events", nil, limits, 10, false)
		run, ok := err.(*govSQLRunError)
		Expect(ok).To(BeTrue())
		Expect(run.Code).To(Equal("QUERY_ERROR"))
		Expect(run.Message).To(ContainSubstring("nope"))
	})

	It("finds where an identifier occurs in the case, and that an unknown one occurs nowhere", func() {
		cat, err := loadAskCatalogue(ctx, pool)
		Expect(err).NotTo(HaveOccurred())
		subject := truth.Subjects[0].MSISDN
		hits, err := askProbe(ctx, pool, truth.Tenant, truth.CaseID, cat, subject, limits)
		Expect(err).NotTo(HaveOccurred())
		sort.Strings(hits)
		Expect(hits).To(ContainElement("v_comm_events.src"))
		none, err := askProbe(ctx, pool, truth.Tenant, truth.CaseID, cat, "923000099999", limits)
		Expect(err).NotTo(HaveOccurred())
		Expect(none).To(BeEmpty())
		other, err := askProbe(ctx, pool, "another-tenant", truth.CaseID, cat, subject, limits)
		Expect(err).NotTo(HaveOccurred())
		Expect(other).To(BeEmpty(), "the probe is scoped like every other statement")
	})

	It("serves the connections of the case: every subscriber's strongest counterparties with the planted-truth counts, and says so when the case is not covered", func() {
		handler := connectionsHandler(pool)
		call := func(collection string) map[string]any {
			rec := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/analysis/connections?tenant_id="+truth.Tenant+"&collection_id="+collection+"&per_subject=200", nil)
			handler(rec, request)
			Expect(rec.Code).To(Equal(http.StatusOK))
			var out map[string]any
			Expect(json.Unmarshal(rec.Body.Bytes(), &out)).To(Succeed())
			return out
		}
		out := call(truth.CaseID)
		Expect(out["available"]).To(BeTrue())
		Expect(out["contract"]).To(Equal("forensics.analysis-views/v1"))
		edges := map[[2]string]map[string]any{}
		for _, e := range out["edges"].([]any) {
			m := e.(map[string]any)
			edges[[2]string{m["source"].(string), m["target"].(string)}] = m
		}
		checked := 0
		for _, s := range truth.Subjects {
			for _, p := range truth.Oracle.A1[s.MSISDN] {
				e, found := edges[[2]string{s.MSISDN, p.Number}]
				Expect(found).To(BeTrue(), s.MSISDN+" with "+p.Number)
				Expect(int(e["outgoing"].(float64))).To(Equal(p.Outgoing))
				Expect(int(e["incoming"].(float64))).To(Equal(p.Incoming))
				Expect(int(e["events"].(float64))).To(Equal(p.Outgoing + p.Incoming + p.Unknown))
				checked++
			}
		}
		Expect(checked).To(BeNumerically(">", 10))
		subjects := out["subjects"].([]any)
		Expect(subjects).To(HaveLen(len(truth.Subjects)))
		for _, e := range out["edges"].([]any) {
			first := e.(map[string]any)["first"].(string)
			_, parseErr := time.Parse(time.RFC3339, first)
			Expect(parseErr).NotTo(HaveOccurred(), "times are RFC 3339 with the case clock's offset: "+first)
			Expect(first).To(HaveSuffix("+05:00"))
		}
		withSIM := 0
		for _, s := range subjects {
			if s.(map[string]any)["imsi"] != nil {
				withSIM++
			}
		}
		Expect(withSIM).To(BeNumerically(">", 0), "a subscriber carries the SIM it was seen with")
		// another tenant's scope sees an empty case (the coverage read finds nothing typed), never this case
		rec := httptest.NewRecorder()
		handler(rec, httptest.NewRequest(http.MethodGet, "/analysis/connections?tenant_id=another-tenant&collection_id="+truth.CaseID, nil))
		var other map[string]any
		Expect(json.Unmarshal(rec.Body.Bytes(), &other)).To(Succeed())
		Expect(other["available"]).To(BeFalse())
		Expect(other["edges"]).To(BeEmpty())
		// no collection is a refusal
		rec = httptest.NewRecorder()
		handler(rec, httptest.NewRequest(http.MethodGet, "/analysis/connections", nil))
		Expect(rec.Code).To(Equal(http.StatusBadRequest))
		rec = httptest.NewRecorder()
		handler(rec, httptest.NewRequest(http.MethodPost, "/analysis/connections?collection_id=x", nil))
		Expect(rec.Code).To(Equal(http.StatusMethodNotAllowed))
	})

	It("serves the places of the case: the cell sites with their events and the hourly visits, scoped", func() {
		handler := placesHandler(pool)
		rec := httptest.NewRecorder()
		handler(rec, httptest.NewRequest(http.MethodGet, "/analysis/places?tenant_id="+truth.Tenant+"&collection_id="+truth.CaseID, nil))
		Expect(rec.Code).To(Equal(http.StatusOK))
		var out map[string]any
		Expect(json.Unmarshal(rec.Body.Bytes(), &out)).To(Succeed())
		Expect(out["available"]).To(BeTrue())
		sites := out["sites"].([]any)
		visits := out["visits"].([]any)
		Expect(sites).NotTo(BeEmpty())
		Expect(visits).NotTo(BeEmpty())
		var fromSites, fromVisits float64
		for _, s := range sites {
			fromSites += s.(map[string]any)["events"].(float64)
		}
		for _, v := range visits {
			fromVisits += v.(map[string]any)["events"].(float64)
		}
		Expect(fromSites).To(Equal(fromVisits), "the sites' events are the visits' events")
		withPosition := 0
		for _, s := range sites {
			site := s.(map[string]any)
			if site["lat"] != nil && site["lon"] != nil {
				withPosition++
				Expect(site["lat"].(float64)).To(And(BeNumerically(">=", -90), BeNumerically("<=", 90)), "a position taken from the records is a latitude")
			}
		}
		Expect(withPosition).To(BeNumerically(">", 0), "migration 019: the positions the records carry reach the places view")
		rec = httptest.NewRecorder()
		handler(rec, httptest.NewRequest(http.MethodGet, "/analysis/places?tenant_id=another-tenant&collection_id="+truth.CaseID, nil))
		var other map[string]any
		Expect(json.Unmarshal(rec.Body.Bytes(), &other)).To(Succeed())
		Expect(other["available"]).To(BeFalse())
	})

	It("reads the coverage of a case: a fully typed case is covered, a case with a call-record file that has no typed rows, or no typed rows at all, is not", func() {
		cover, err := askCoverageRead(ctx, pool, truth.Tenant, truth.CaseID)
		Expect(err).NotTo(HaveOccurred())
		Expect(cover.AnyComm).To(BeTrue())
		Expect(cover.Files).To(BeNumerically(">", 0))
		Expect(cover.Typed).To(Equal(cover.Files))
		Expect(askCoverageGap(cover, []string{"v_comm_events", "v_contact_pairs"})).To(BeEmpty())
		if cover.AnySession {
			Expect(cover.AnyVolume).To(BeTrue(), "the planted case's internet records state their volumes")
		}

		// an older case: a call-record file, no typed row
		tx, err := pool.Begin(ctx)
		Expect(err).NotTo(HaveOccurred())
		_, err = tx.Exec(ctx, "SELECT set_config('app.tenant_id', 'cov-tenant', true)")
		Expect(err).NotTo(HaveOccurred())
		_, err = tx.Exec(ctx, `INSERT INTO forensic.evidence_items (evidence_id, tenant_id, collection_id, case_id, user_id, original_filename, source_file, content_type, extension, size_bytes, sha256,
modality, detected_type, classifier_confidence, processing_route, processing_status, raw_storage_ref, metadata) VALUES ('3a000000-0000-4000-8000-000000000001', 'cov-tenant', 'cov-old-case', 'SYNTHETIC-COV', 'u',
'old.csv', 'old.csv', 'text/csv', 'csv', 1, repeat('c', 64), 'structured_records', 'cdr', 1, 'w', 'completed', 'immutable://cov/1', jsonb_build_object('request_id', 'r'))`)
		Expect(err).NotTo(HaveOccurred())
		Expect(tx.Commit(ctx)).To(Succeed())
		old, err := askCoverageRead(ctx, pool, "cov-tenant", "cov-old-case")
		Expect(err).NotTo(HaveOccurred())
		Expect(old.Files).To(Equal(1))
		Expect(old.Typed).To(Equal(0))
		Expect(askCoverageGap(old, []string{"v_comm_events"})).To(Equal("the case has no typed call records"))
		empty, err := askCoverageRead(ctx, pool, "cov-tenant", "nothing-here")
		Expect(err).NotTo(HaveOccurred())
		Expect(askCoverageGap(empty, []string{"v_comm_events"})).NotTo(BeEmpty())
		// the engine, asked about the older case, declines before the model is called
		saved := askModel
		defer func() { askModel = saved }()
		called := false
		askModel = func(context.Context, config, string, []govSQLTurn) (string, string) { called = true; return "", "NO" }
		resp, audit := runAskEngineV2(ctx, config{SynthesisModel: "scripted"}, pool, hybridQueryRequest{TenantID: "cov-tenant", CollectionID: "cov-old-case", RequestClass: forensicrequest.GovernedAnalysis,
			Query: "How many incoming SMS did 923000000101 receive?"}, time.Now())
		Expect(resp).To(BeNil())
		Expect(audit.State).To(Equal("declined"))
		Expect(audit.Reason).To(ContainSubstring("cannot speak for this case"))
		Expect(called).To(BeFalse())
	})

	It("reads a pair in both orientations, so that swapped roles can be told from a true zero", func() {
		subject, other, events, _ := pair()
		for _, ab := range [][2]string{{subject, other}, {other, subject}} {
			pairs, err := askPairs(ctx, pool, truth.Tenant, truth.CaseID, ab[0], ab[1], limits)
			Expect(err).NotTo(HaveOccurred())
			Expect(pairs).To(HaveLen(1), "the other party is not a subscriber, so only one orientation exists")
			Expect(pairs[[2]string{subject, other}].Events).To(Equal(int64(events)))
		}
		none, err := askPairs(ctx, pool, "another-tenant", truth.CaseID, subject, other, limits)
		Expect(err).NotTo(HaveOccurred())
		Expect(none).To(BeEmpty())
	})

	It("answers a question end to end with a scripted model, and the answer is the planted truth", func() {
		subject, other, events, _ := pair()
		savedModel := askModel
		defer func() { askModel = savedModel }()
		sql := fmt.Sprintf("SELECT events AS n FROM v_contact_pairs WHERE src = '%s' AND dst = '%s'", subject, other)
		askModel = func(context.Context, config, string, []govSQLTurn) (string, string) {
			raw, _ := json.Marshal(map[string]any{"answerable": true, "sql": sql})
			return string(raw), ""
		}
		req := hybridQueryRequest{TenantID: truth.Tenant, CollectionID: truth.CaseID, RequestClass: forensicrequest.GovernedAnalysis,
			Query: fmt.Sprintf("How many events are there between %s and %s?", subject, other)}
		resp, audit := runAskEngineV2(ctx, config{SynthesisModel: "scripted"}, pool, req, time.Now())
		Expect(audit.State).To(Equal("answered"), audit.Reason)
		Expect(resp).NotTo(BeNil())
		Expect(resp.Answer["answer"]).To(Equal(fmt.Sprintf("N: %d.", events)))
		rows := resp.Enterprise["data_grid"].(map[string]any)["rows"].([]map[string]any)
		Expect(fmt.Sprint(rows[0]["n"])).To(Equal(fmt.Sprint(events)))
	})

	It("says that a number the case does not hold has no record, and does not answer zero", func() {
		savedModel := askModel
		defer func() { askModel = savedModel }()
		askModel = func(context.Context, config, string, []govSQLTurn) (string, string) {
			raw, _ := json.Marshal(map[string]any{"answerable": true, "sql": "SELECT count(*) AS n FROM v_comm_events WHERE src = '923000099999' OR dst = '923000099999'"})
			return string(raw), ""
		}
		req := hybridQueryRequest{TenantID: truth.Tenant, CollectionID: truth.CaseID, RequestClass: forensicrequest.GovernedAnalysis, Query: "How many calls does 923000099999 have?"}
		resp, audit := runAskEngineV2(ctx, config{SynthesisModel: "scripted"}, pool, req, time.Now())
		Expect(audit.State).To(Equal("answered"), audit.Reason)
		Expect(resp.Answer["answer"]).To(Equal("No record of this case holds 923000099999 in any number, device, SIM, address or cell column."))
	})

	It("writes one audit row for the question, in the tenant's own rows, and the application role cannot change or remove it", func() {
		subject, other, events, _ := pair()
		savedModel := askModel
		defer func() { askModel = savedModel }()
		sql := fmt.Sprintf("SELECT events AS n FROM v_contact_pairs WHERE src = '%s' AND dst = '%s'", subject, other)
		askModel = func(context.Context, config, string, []govSQLTurn) (string, string) {
			raw, _ := json.Marshal(map[string]any{"answerable": true, "sql": sql})
			return string(raw), ""
		}
		question := fmt.Sprintf("Count the events between %s and %s for the audit check.", subject, other)
		req := hybridQueryRequest{TenantID: truth.Tenant, UserID: "audit-spec", CollectionID: truth.CaseID, RequestClass: forensicrequest.GovernedAnalysis, Query: question}
		_, audit := runAskEngineV2(ctx, config{SynthesisModel: "scripted"}, pool, req, time.Now())
		Expect(audit.State).To(Equal("answered"))

		read := func(tenant string) (n int, details map[string]any) {
			tx, err := pool.Begin(ctx)
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = tx.Rollback(ctx) }()
			_, err = tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenant)
			Expect(err).NotTo(HaveOccurred())
			Expect(tx.QueryRow(ctx, "SELECT count(*) FROM forensic.records_audit_log WHERE action = $1 AND details->>'question' = $2", askAuditAction, question).Scan(&n)).To(Succeed())
			var raw []byte
			if n > 0 {
				Expect(tx.QueryRow(ctx, "SELECT details FROM forensic.records_audit_log WHERE action = $1 AND details->>'question' = $2 ORDER BY id DESC LIMIT 1", askAuditAction, question).Scan(&raw)).To(Succeed())
				Expect(json.Unmarshal(raw, &details)).To(Succeed())
			}
			return n, details
		}
		n, details := read(truth.Tenant)
		Expect(n).To(Equal(1))
		Expect(details["question"]).To(Equal(question))
		Expect(details["state"]).To(Equal("answered"))
		Expect(details["sql"]).To(ContainSubstring("v_contact_pairs"))
		Expect(details["result_digest"]).To(HaveLen(64))
		Expect(fmt.Sprint(details["rows"])).To(Equal("1"))
		Expect(fmt.Sprint(events)).NotTo(BeEmpty())
		other2, _ := read("another-tenant")
		Expect(other2).To(Equal(0), "another tenant sees none of it")

		tx, err := pool.Begin(ctx)
		Expect(err).NotTo(HaveOccurred())
		defer func() { _ = tx.Rollback(ctx) }()
		_, err = tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", truth.Tenant)
		Expect(err).NotTo(HaveOccurred())
		_, err = tx.Exec(ctx, "DELETE FROM forensic.records_audit_log WHERE action = $1", askAuditAction)
		Expect(err).To(HaveOccurred(), "the application role may append to the audit log, not remove from it")
	})

	// The engine's health for a case (roadmap 7.8.3 row 9): counts and times from the engine's own audit rows, only the tenant's own, only the collection asked for, and never a
	// question, an answer or a query.
	It("reports how the engine has done on a case from its audit rows: counts, times and reasons for declines, for the tenant's own case only, and no question text", func() {
		handler := engineHealthHandler(pool)
		call := func(tenant, collection string) (int, map[string]any, string) {
			rec := httptest.NewRecorder()
			handler(rec, httptest.NewRequest(http.MethodGet, "/analysis/engine-health?tenant_id="+tenant+"&collection_id="+collection+"&days=7", nil))
			var out map[string]any
			if rec.Code == http.StatusOK {
				Expect(json.Unmarshal(rec.Body.Bytes(), &out)).To(Succeed())
			}
			return rec.Code, out, rec.Body.String()
		}
		totals := func(out map[string]any) map[string]float64 {
			m := map[string]float64{}
			for k, v := range out["totals"].(map[string]any) {
				m[k] = v.(float64)
			}
			return m
		}
		code, before, _ := call(truth.Tenant, truth.CaseID)
		Expect(code).To(Equal(http.StatusOK))
		Expect(before["contract"]).To(Equal("forensics.engine-health/v1"))
		was := totals(before)

		// requests that were not questions for the engine (the interface's own template and structured requests; older rows were audited as declined questions) are not its questions
		for _, reason := range []string{"an explicit operation was requested", "a structured request"} {
			askWriteAudit(ctx, pool, hybridQueryRequest{TenantID: truth.Tenant, UserID: "health-spec", CollectionID: truth.CaseID, Query: "template request"}, AskAuditV2{State: "declined", Reason: reason}, askFrame{}, "")
		}

		subject, other, _, _ := pair()
		savedModel := askModel
		defer func() { askModel = savedModel }()
		sql := fmt.Sprintf("SELECT events AS n FROM v_contact_pairs WHERE src = '%s' AND dst = '%s'", subject, other)
		askModel = func(context.Context, config, string, []govSQLTurn) (string, string) {
			raw, _ := json.Marshal(map[string]any{"answerable": true, "sql": sql})
			return string(raw), ""
		}
		secret := "ENGINE-HEALTH-SPEC-QUESTION-TEXT"
		answered := hybridQueryRequest{TenantID: truth.Tenant, UserID: "health-spec", CollectionID: truth.CaseID, RequestClass: forensicrequest.GovernedAnalysis, Query: fmt.Sprintf("Count the events between %s and %s. %s", subject, other, secret)}
		_, audit := runAskEngineV2(ctx, config{SynthesisModel: "scripted"}, pool, answered, time.Now())
		Expect(audit.State).To(Equal("answered"))
		declined := hybridQueryRequest{TenantID: truth.Tenant, UserID: "health-spec", CollectionID: truth.CaseID, RequestClass: forensicrequest.GovernedAnalysis, Query: "How many calls were made last week? " + secret}
		_, audit = runAskEngineV2(ctx, config{SynthesisModel: "scripted"}, pool, declined, time.Now())
		Expect(audit.State).To(Equal("declined"))

		code, after, body := call(truth.Tenant, truth.CaseID)
		Expect(code).To(Equal(http.StatusOK))
		now := totals(after)
		Expect(now["questions"]).To(Equal(was["questions"] + 2))
		Expect(now["answered"]).To(Equal(was["answered"] + 1))
		Expect(now["declined"]).To(Equal(was["declined"] + 1))
		Expect(after["latency_ms"].(map[string]any)["p50"]).NotTo(BeNil())
		Expect(body).NotTo(ContainSubstring(secret), "no question text is returned")
		Expect(body).NotTo(ContainSubstring(subject), "no number of the case is returned")
		declines := after["declines"].([]any)
		Expect(declines).NotTo(BeEmpty())
		reasons := ""
		for _, d := range declines {
			reasons += d.(map[string]any)["reason"].(string) + ";"
		}
		Expect(reasons).To(ContainSubstring("the question uses a relative time (#) and no reference day is stated"), "the phrase of the question is replaced by a mark")
		Expect(body).NotTo(ContainSubstring("last week"), "no word of a question is returned")
		Expect(reasons).NotTo(ContainSubstring("explicit operation"), "a request that was not a question is not counted among the engine's declines")

		// another tenant's case and another collection see none of it; the method and the scope are checked like every analysis read
		code, none, _ := call("some-other-tenant", truth.CaseID)
		Expect(code).To(Equal(http.StatusOK))
		Expect(totals(none)["questions"]).To(BeZero())
		code, empty, _ := call(truth.Tenant, "another-collection")
		Expect(code).To(Equal(http.StatusOK))
		Expect(totals(empty)["questions"]).To(BeZero())
		rec := httptest.NewRecorder()
		handler(rec, httptest.NewRequest(http.MethodPost, "/analysis/engine-health?tenant_id="+truth.Tenant+"&collection_id="+truth.CaseID, nil))
		Expect(rec.Code).To(Equal(http.StatusMethodNotAllowed))
	})

	// Decision D22 (2026-10-16): two numbers are connected when they are in contact with each other or through others, in either direction. The verified example is the
	// reference: run on the case with the numbers of two subscribers who share a contact, it must return only edges that are real contacts of the key, join the two, and the
	// sentence built from it must name a contact that both really have.
	It("finds two subscribers who share a contact: the example's walk goes both ways, returns real edges only, and the answer names the shared number", func() {
		examples, err := askExamples()
		Expect(err).NotTo(HaveOccurred())
		var example askExample
		for _, e := range examples {
			if e.ID == "connection-path" {
				example = e
			}
		}
		Expect(example.SQL).NotTo(BeEmpty())
		contacts := map[string]map[string]bool{}
		edge := map[[2]string]bool{}
		for subject, list := range truth.Oracle.A1 {
			contacts[subject] = map[string]bool{}
			for _, p := range list {
				if p.Kind == "person_like" {
					contacts[subject][p.Number] = true
					edge[[2]string{subject, p.Number}], edge[[2]string{p.Number, subject}] = true, true
				}
			}
		}
		var s1, s2 string
		var shared []string
		for _, x := range truth.Subjects {
			for _, y := range truth.Subjects {
				if x.MSISDN >= y.MSISDN || contacts[x.MSISDN][y.MSISDN] || contacts[y.MSISDN][x.MSISDN] {
					continue
				}
				var both []string
				for c := range contacts[x.MSISDN] {
					if contacts[y.MSISDN][c] {
						both = append(both, c)
					}
				}
				if len(both) > 0 && s1 == "" {
					s1, s2, shared = x.MSISDN, y.MSISDN, both
				}
			}
		}
		Expect(s1).NotTo(BeEmpty(), "the generated case has two subscribers who share a contact and do not call each other")
		sql := strings.NewReplacer("923000000101", s1, "923000000102", s2).Replace(example.SQL)
		result, err := scoped(sql)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Rows).NotTo(BeEmpty())
		for _, row := range result.Rows {
			Expect(edge[[2]string{fmt.Sprint(row[0]), fmt.Sprint(row[1])}]).To(BeTrue(), fmt.Sprint(row)+" is not a contact of the key")
		}
		frame := askExtractFrame("How are " + s1 + " and " + s2 + " connected within three hops, not through services or hubs?")
		headline := askPathHeadline(result, frame, 3)
		Expect(headline).To(HavePrefix(s1 + " and " + s2 + " are connected"))
		named := false
		for _, c := range shared {
			named = named || strings.Contains(headline, c)
		}
		Expect(named).To(BeTrue(), headline)
	})

	// The red team of the executor (the validator's is in ask_engine_redteam_ginkgo_test.go): statements the validator ACCEPTS, because each is a valid read of the views, but that
	// would be costly on a large case. Each must be refused by the planner's cost, stopped by the statement timeout, or return a result under the row cap, and none may run past
	// the timeout. What the validator refuses is not run, and counts as refused.
	It("holds the costly statements that the validator lets through: refused by cost, stopped by the timeout or capped, never unbounded", func() {
		cat, err := loadAskCatalogue(ctx, pool)
		Expect(err).NotTo(HaveOccurred())
		bombs := []string{
			"SELECT count(*) FROM v_comm_events a JOIN v_comm_events b ON a.src <> b.src",
			"SELECT count(*) FROM v_comm_events a JOIN v_comm_events b ON a.start_utc < b.start_utc",
			"SELECT count(*) FROM v_comm_events a JOIN v_comm_events b ON a.dst <> b.dst JOIN v_comm_events c ON b.dst <> c.dst",
			"SELECT array_agg(event_id) FROM v_comm_events",
			"SELECT string_agg(a.dst, ',') FROM v_comm_events a JOIN v_comm_events b ON a.src <> b.src",
			"SELECT src, count(*) FROM v_comm_events GROUP BY src, dst, start_utc, end_utc ORDER BY 2 DESC",
			"WITH RECURSIVE walk AS (SELECT src, dst, 1 AS depth, ARRAY[src, dst] AS path FROM v_contact_pairs UNION ALL SELECT w.src, e.dst, w.depth + 1, w.path || e.dst FROM walk w JOIN v_contact_pairs e ON e.src = w.dst WHERE w.depth < 4 AND NOT e.dst = ANY (w.path)) SELECT src, dst FROM walk LIMIT 200",
			"SELECT a.src, b.src AS other, count(*) AS n FROM v_comm_events a JOIN v_comm_events b ON a.dst = b.dst GROUP BY a.src, b.src",
			"SELECT * FROM v_comm_events a JOIN v_comm_events b ON a.dst = b.dst LIMIT 5000",
			"SELECT count(*) FROM v_comm_events WHERE dst ~ '(a+)+$'",
			"SELECT count(*) FROM v_comm_events WHERE dst ~ '^(([a-z])+.)+[A-Z]([a-z])+$'",
		}
		outcomes := map[string]int{}
		for _, sql := range bombs {
			valid, rej := askValidate(sql, cat, nil)
			if rej != nil {
				outcomes["refused by the validator"]++
				continue
			}
			started := time.Now()
			result, err := askExecute(ctx, pool, truth.Tenant, truth.CaseID, valid, limits)
			elapsed := time.Since(started)
			Expect(elapsed).To(BeNumerically("<", limits.Timeout+10*time.Second), sql)
			if err != nil {
				run, ok := err.(*govSQLRunError)
				Expect(ok).To(BeTrue(), fmt.Sprint(err))
				Expect([]string{"TOO_EXPENSIVE", "TIMEOUT"}).To(ContainElement(run.Code), sql+" -> "+run.Code+" "+run.Message)
				outcomes["stopped: "+run.Code]++
				continue
			}
			Expect(len(result.Rows)).To(BeNumerically("<=", limits.RowCap), sql)
			outcomes["ran within the limits"]++
		}
		GinkgoWriter.Printf("red team of the executor, %d statements: %v\n", len(bombs), outcomes)
	})

	It("leaves the case unchanged: the engine wrote nothing", func() {
		var writes int
		Expect(pool.QueryRow(ctx, "SELECT coalesce(sum(n_tup_ins + n_tup_upd + n_tup_del), 0)::int FROM pg_stat_user_tables WHERE schemaname = 'analyst'").Scan(&writes)).To(Succeed())
		Expect(writes).To(Equal(0))
	})
})
