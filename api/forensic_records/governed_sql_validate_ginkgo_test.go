package main

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The validator is the security gate of the governed SQL lane, so it is tested the
// way a gate is: a list of queries that must pass, and a longer list of hostile
// ones that must not, each with the reason code it must be refused for.
func govSQLTestViews() map[string]*govSQLView {
	layer, _ := defaultSemanticLayer()
	Expect(layer).NotTo(BeNil(), "the curated semantic layer must load for these tests")
	out := map[string]*govSQLView{}
	for _, view := range govSQLViews(layer, hybridQueryRequest{TenantID: "t", CollectionID: "c"}) {
		out[view.Name] = view
	}
	Expect(out).To(HaveKey("v_cdr"))
	return out
}

var _ = Describe("Governed SQL validator", func() {
	var views map[string]*govSQLView
	BeforeEach(func() { views = govSQLTestViews() })

	accepted := []string{
		"SELECT COUNT(*) AS total FROM v_cdr",
		"SELECT MIN(call_start) AS earliest_call FROM v_cdr",
		"SELECT call_type, COUNT(*) AS calls FROM v_cdr GROUP BY call_type ORDER BY calls DESC LIMIT 5",
		"SELECT COUNT(DISTINCT msisdn) AS numbers FROM v_cdr WHERE call_type = 'VOICE'",
		"SELECT COUNT(*) FROM v_cdr WHERE EXTRACT(HOUR FROM call_start) BETWEEN 0 AND 5",
		"SELECT AVG(call_duration_seconds) FROM v_cdr WHERE msisdn = '923001110001'",
		"SELECT msisdn, COUNT(*) c, RANK() OVER (ORDER BY COUNT(*) DESC) r FROM v_cdr GROUP BY msisdn ORDER BY r LIMIT 3",
		"WITH t AS (SELECT msisdn, COUNT(*) c FROM v_cdr GROUP BY msisdn) SELECT * FROM t WHERE c > 5",
		"SELECT COUNT(*) FROM v_cdr WHERE msisdn IN (SELECT msisdn FROM v_cdr WHERE call_type = 'SMS')",
		"SELECT CASE WHEN call_duration_seconds > 60 THEN 'long' ELSE 'short' END AS bucket, COUNT(*) FROM v_cdr GROUP BY 1",
		"SELECT DATE_TRUNC('month', call_start) AS month, COUNT(*) FROM v_cdr GROUP BY 1 ORDER BY 1",
		"SELECT COUNT(*) FROM v_cdr WHERE call_start >= '2026-04-01' AND call_start < '2026-05-01'",
		"SELECT COUNT(*) FROM v_cdr WHERE msisdn = '923001110001' OR dialed_number = '923001110001'",
		"SELECT * FROM v_subscriber LIMIT 5",
		"SELECT status, COUNT(*) FROM v_subscriber GROUP BY status",
		"SELECT COUNT(*) FROM v_ipdr WHERE domain ILIKE '%example%'",
		"SELECT SUM(bytes_down) FROM v_ipdr",
		"SELECT MAX(event_time) FROM v_access_log",
		"SELECT source_ip, COUNT(*) FROM v_access_log WHERE status = '403' GROUP BY source_ip ORDER BY 2 DESC LIMIT 3",
		"SELECT COUNT(*) FROM v_cdr WHERE call_start::date = '2026-04-02'",
		"SELECT COUNT(*) FROM v_cdr WHERE call_start AT TIME ZONE 'Asia/Karachi' > now() - interval '30 days'",
		"SELECT ROUND(AVG(network_volume)::numeric, 2) FROM v_cdr",
		"SELECT COUNT(*) FILTER (WHERE call_type = 'SMS') AS sms, COUNT(*) FILTER (WHERE call_type = 'VOICE') AS voice FROM v_cdr",
		"SELECT msisdn FROM v_cdr GROUP BY msisdn HAVING COUNT(*) > 100",
		"SELECT COALESCE(location, 'unknown') AS loc, COUNT(*) FROM v_cdr GROUP BY 1",
		"SELECT STRING_AGG(DISTINCT call_type, ', ') FROM v_cdr",
		"SELECT COUNT(*) FROM v_cdr WHERE imei IS NOT NULL AND imei <> ''",
		"SELECT LOWER(city), COUNT(*) FROM v_subscriber GROUP BY 1",
		"SELECT COUNT(*) FROM v_cdr WHERE EXISTS (SELECT 1 FROM v_cdr c2 WHERE c2.imsi = '1')",
		"SELECT msisdn FROM v_cdr WHERE call_type='SMS' INTERSECT SELECT msisdn FROM v_cdr WHERE call_type='VOICE'",
		"SELECT msisdn, COUNT(*) AS n FROM v_cdr WHERE msisdn LIKE '9230%' GROUP BY msisdn ORDER BY n DESC, msisdn LIMIT 10 OFFSET 0",
		"select count(*) from v_cdr where call_start between '2026-04-01' and '2026-04-30'",
		"SELECT PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY call_duration_seconds) FROM v_cdr",
		"SELECT COUNT(*) FROM v_cdr WHERE direction = 'INCOMING' AND call_type <> 'SMS'",
		"SELECT MAX(call_duration_seconds) AS longest FROM v_cdr",
		"SELECT COUNT(*) AS \"total calls\" FROM v_cdr",
		"SELECT COUNT(*) FROM v_cdr;",
		"SELECT COUNT(*) FROM v_cdr /* ; DROP TABLE forensic.records; */",
		"SELECT COUNT(*) FROM v_cdr -- ; DROP TABLE forensic.records",
		"SELECT $$; DROP TABLE x$$ AS text FROM v_cdr LIMIT 1",
		"SELECT COUNT(*) FROM v_cdr WHERE msisdn = ANY(ARRAY['1','2'])",
		"SELECT msisdn FROM v_cdr WHERE call_start > CURRENT_DATE - 7",
	}
	for _, query := range accepted {
		query := query
		It("accepts "+query, func() {
			result, rejection := govSQLValidate(query, views)
			Expect(rejection).To(BeNil(), "rejected: %v", rejection)
			Expect(result.View).NotTo(BeNil())
			// The canonical text is what runs, so it must itself be one accepted statement,
			// and validating it again must change nothing (no comment or second statement survives).
			again, second := govSQLValidate(result.SQL, views)
			Expect(second).To(BeNil(), "the canonical text was rejected: %v", second)
			Expect(again.SQL).To(Equal(result.SQL))
		})
	}

	type hostile struct{ sql, code string }
	rejected := []hostile{
		// more than one statement, or not a SELECT
		{"SELECT 1; SELECT 2", "ONE_STATEMENT"},
		{"SELECT COUNT(*) FROM v_cdr; DROP TABLE forensic.records", "ONE_STATEMENT"},
		{"DROP TABLE forensic.records", "SELECT_ONLY"},
		{"DELETE FROM v_cdr", "SELECT_ONLY"},
		{"UPDATE v_cdr SET msisdn = '1'", "SELECT_ONLY"},
		{"INSERT INTO v_cdr VALUES (1)", "SELECT_ONLY"},
		{"TRUNCATE forensic.records", "SELECT_ONLY"},
		{"ALTER TABLE forensic.records DISABLE ROW LEVEL SECURITY", "SELECT_ONLY"},
		{"CREATE TABLE x AS SELECT * FROM v_cdr", "SELECT_ONLY"},
		{"GRANT ALL ON forensic.records TO PUBLIC", "SELECT_ONLY"},
		{"COPY forensic.records TO '/tmp/x'", "SELECT_ONLY"},
		{"COPY (SELECT 1) TO PROGRAM 'curl http://x'", "SELECT_ONLY"},
		{"SET ROLE postgres", "SELECT_ONLY"},
		{"SET search_path = pg_catalog", "SELECT_ONLY"},
		{"RESET ALL", "SELECT_ONLY"},
		{"SHOW all", "SELECT_ONLY"},
		{"EXPLAIN SELECT * FROM v_cdr", "SELECT_ONLY"},
		{"EXPLAIN ANALYZE SELECT * FROM v_cdr", "SELECT_ONLY"},
		{"VACUUM", "SELECT_ONLY"},
		{"ANALYZE forensic.records", "SELECT_ONLY"},
		{"DO $$ BEGIN PERFORM pg_sleep(10); END $$", "SELECT_ONLY"},
		{"CALL some_proc()", "SELECT_ONLY"},
		{"PREPARE p AS SELECT 1", "SELECT_ONLY"},
		{"EXECUTE p", "SELECT_ONLY"},
		{"LISTEN x", "SELECT_ONLY"},
		{"NOTIFY x", "SELECT_ONLY"},
		{"LOCK TABLE forensic.records", "SELECT_ONLY"},
		{"BEGIN", "SELECT_ONLY"},
		{"COMMIT", "SELECT_ONLY"},
		{"CREATE EXTENSION dblink", "SELECT_ONLY"},
		{"REFRESH MATERIALIZED VIEW x", "SELECT_ONLY"},
		// SELECT forms that write, lock, recurse or sample
		{"SELECT * INTO newt FROM v_cdr", "NO_INTO"},
		{"SELECT * FROM v_cdr FOR UPDATE", "NO_LOCKING"},
		{"SELECT * FROM v_cdr FOR SHARE", "NO_LOCKING"},
		{"VALUES (1), (2)", "NO_VALUES"},
		{"SELECT * FROM v_cdr TABLESAMPLE SYSTEM (10)", "NOT_ALLOWED"},
		{"WITH RECURSIVE t(n) AS (SELECT 1 UNION ALL SELECT n + 1 FROM t) SELECT n FROM t", "NO_RECURSION"},
		{"WITH d AS (DELETE FROM v_cdr RETURNING *) SELECT * FROM d", "NOT_ALLOWED"},
		{"WITH i AS (INSERT INTO v_cdr VALUES (1) RETURNING *) SELECT * FROM i", "NOT_ALLOWED"},
		// relations the model was not issued
		{"SELECT * FROM forensic.records", "NO_SCHEMA"},
		{"SELECT * FROM records", "UNKNOWN_RELATION"},
		{"SELECT * FROM pg_catalog.pg_class", "NO_SCHEMA"},
		{"SELECT * FROM pg_class", "UNKNOWN_RELATION"},
		{"SELECT * FROM pg_user", "UNKNOWN_RELATION"},
		{"SELECT * FROM pg_shadow", "UNKNOWN_RELATION"},
		{"SELECT * FROM information_schema.tables", "NO_SCHEMA"},
		{"SELECT * FROM pg_stat_activity", "UNKNOWN_RELATION"},
		{"SELECT * FROM public.v_cdr", "NO_SCHEMA"},
		{"SELECT * FROM forensic.derived_artifacts", "NO_SCHEMA"},
		{"SELECT * FROM v_does_not_exist", "UNKNOWN_RELATION"},
		{"SELECT (SELECT COUNT(*) FROM pg_class) FROM v_cdr", "UNKNOWN_RELATION"},
		{"SELECT * FROM v_cdr, v_ipdr", "NO_JOIN"},
		{"SELECT * FROM v_cdr JOIN v_ipdr ON true", "NOT_ALLOWED"},
		{"SELECT * FROM v_cdr CROSS JOIN v_cdr", "NOT_ALLOWED"},
		{"SELECT * FROM v_cdr a LEFT JOIN v_cdr b ON a.msisdn = b.msisdn", "NOT_ALLOWED"},
		{"SELECT 1", "ONE_VIEW"},
		{"SELECT COUNT(*) FROM v_cdr WHERE msisdn IN (SELECT subscriber_id FROM v_ipdr)", "ONE_VIEW"},
		{"SELECT * FROM generate_series(1, 10)", "NOT_ALLOWED"},
		{"SELECT * FROM unnest(ARRAY[1, 2])", "NOT_ALLOWED"},
		{"SELECT * FROM (SELECT * FROM pg_class) x", "UNKNOWN_RELATION"},
		{"WITH v_cdr AS (SELECT 1 AS x) SELECT * FROM v_cdr", "CTE_NAME"},
		{"WITH pg_x AS (SELECT * FROM v_cdr) SELECT * FROM pg_x", "CTE_NAME"},
		// functions that touch the server, the clock source, the filesystem or the network
		{"SELECT pg_sleep(30) FROM v_cdr", "FUNCTION"},
		{"SELECT pg_read_file('/etc/passwd') FROM v_cdr", "FUNCTION"},
		{"SELECT pg_ls_dir('.') FROM v_cdr", "FUNCTION"},
		{"SELECT current_setting('server_version') FROM v_cdr", "FUNCTION"},
		{"SELECT set_config('app.tenant_id', 'other', false) FROM v_cdr", "FUNCTION"},
		{"SELECT version() FROM v_cdr", "FUNCTION"},
		{"SELECT current_database() FROM v_cdr", "FUNCTION"},
		{"SELECT dblink('host=x', 'select 1') FROM v_cdr", "FUNCTION"},
		{"SELECT lo_import('/etc/passwd') FROM v_cdr", "FUNCTION"},
		{"SELECT pg_terminate_backend(1) FROM v_cdr", "FUNCTION"},
		{"SELECT pg_cancel_backend(1) FROM v_cdr", "FUNCTION"},
		{"SELECT random() FROM v_cdr", "FUNCTION"},
		{"SELECT repeat('a', 1000000000) FROM v_cdr", "FUNCTION"},
		{"SELECT lpad('a', 1000000000) FROM v_cdr", "FUNCTION"},
		{"SELECT pg_catalog.pg_read_file('x') FROM v_cdr", "FUNCTION"},
		{"SELECT pg_catalog.version()", "FUNCTION"},
		{"SELECT public.myfunc(1) FROM v_cdr", "FUNCTION"},
		{"SELECT txid_current() FROM v_cdr", "FUNCTION"},
		{"SELECT inet_server_addr() FROM v_cdr", "FUNCTION"},
		{"SELECT to_regclass('pg_class') FROM v_cdr", "FUNCTION"},
		{"SELECT query_to_xml('select 1', true, true, '') FROM v_cdr", "FUNCTION"},
		{"SELECT pg_advisory_lock(1) FROM v_cdr", "FUNCTION"},
		{"SELECT generate_series(1, 100000000) FROM v_cdr", "FUNCTION"},
		{"SELECT unnest(ARRAY[1, 2]) FROM v_cdr", "FUNCTION"},
		{"SELECT current_user FROM v_cdr", "VALUE_FUNCTION"},
		{"SELECT session_user FROM v_cdr", "VALUE_FUNCTION"},
		{"SELECT user FROM v_cdr", "VALUE_FUNCTION"},
		{"SELECT current_catalog FROM v_cdr", "VALUE_FUNCTION"},
		{"SELECT current_schema FROM v_cdr", "VALUE_FUNCTION"},
		// operators, casts and expression forms
		{"SELECT msisdn::regclass FROM v_cdr", "TYPE"},
		{"SELECT 'pg_class'::regclass FROM v_cdr", "TYPE"},
		{"SELECT 1::oid FROM v_cdr", "TYPE"},
		{"SELECT msisdn::text[] FROM v_cdr", "TYPE"},
		{"SELECT msisdn FROM v_cdr WHERE msisdn OPERATOR(pg_catalog.=) '1'", "OPERATOR"},
		{"SELECT msisdn FROM v_cdr WHERE msisdn @@ 'x'", "OPERATOR"},
		{"SELECT metadata->>'x' FROM v_cdr", "OPERATOR"},
		{"SELECT * FROM v_cdr WHERE msisdn = $1", "NOT_ALLOWED"},
		{"SELECT (ARRAY[1, 2])[1] FROM v_cdr", "NOT_ALLOWED"},
		{"SELECT msisdn COLLATE \"C\" FROM v_cdr", "NOT_ALLOWED"},
		{"SELECT B'101' FROM v_cdr", "CONSTANT"},
		{"SELECT ROW(1, 2) FROM v_cdr", "NOT_ALLOWED"},
		{"SELECT GROUPING(msisdn) FROM v_cdr GROUP BY ROLLUP (msisdn)", "NOT_ALLOWED"},
		{"SELECT a.b.c.d FROM v_cdr", "NO_QUALIFIED_COLUMN"},
		{"SELECT '" + strings.Repeat("a", 500) + "' FROM v_cdr", "CONSTANT"},
		// columns the view does not have (the raw tables are not reachable by name)
		{"SELECT raw_payload FROM v_cdr", "UNKNOWN_COLUMN"},
		{"SELECT tenant_id FROM v_cdr", "UNKNOWN_COLUMN"},
		{"SELECT collection_id FROM v_cdr", "UNKNOWN_COLUMN"},
		{"SELECT metadata FROM v_cdr", "UNKNOWN_COLUMN"},
		{"SELECT cnic FROM v_subscriber", "UNKNOWN_COLUMN"},
		{"SELECT subscriber_name FROM v_subscriber", "UNKNOWN_COLUMN"},
		{"SELECT * FROM v_cdr WHERE not_a_column = 1", "UNKNOWN_COLUMN"},
		// size and shape
		{"", "EMPTY"},
		{"   ", "EMPTY"},
		{"-- only a comment", "ONE_STATEMENT"},
		{"SELECT 1 FROM v_cdr WHERE msisdn = '" + strings.Repeat("9", 4100) + "'", "TOO_LONG"},
		{"SELECT COUNT(*) FROM v_cdr WHERE msisdn IN (" + strings.TrimSuffix(strings.Repeat("'1',", 700), ",") + ")", "TOO_COMPLEX"},
		{"SELECT " + strings.Repeat("abs(", 40) + "1" + strings.Repeat(")", 40) + " FROM v_cdr", "TOO_COMPLEX"},
		{"SELECT 1 FROM v_cdr WHERE msisdn = 'a\x00b'", "BAD_TEXT"},
	}
	for _, item := range rejected {
		item := item
		name := item.sql
		if len(name) > 90 {
			name = name[:90] + "..."
		}
		It("rejects ("+item.code+") "+name, func() {
			result, rejection := govSQLValidate(item.sql, views)
			Expect(result).To(BeNil(), "this query must never be accepted")
			Expect(rejection).NotTo(BeNil())
			Expect(rejection.Code).To(Equal(item.code), "message: %s", rejection.Message)
			Expect(rejection.Message).NotTo(BeEmpty())
		})
	}

	It("keeps the number of hostile queries well above the gate", func() {
		Expect(len(rejected)).To(BeNumerically(">=", 100))
	})

	It("records the literals a query contains, for the verifier", func() {
		result, rejection := govSQLValidate("SELECT COUNT(*) FROM v_cdr WHERE msisdn = '923001110001' AND call_duration_seconds > 60", views)
		Expect(rejection).To(BeNil())
		var texts []string
		for _, c := range result.Consts {
			texts = append(texts, c.Text)
		}
		Expect(texts).To(ContainElements("923001110001", "60"))
	})

	It("reports which view columns a query references", func() {
		result, rejection := govSQLValidate("SELECT call_type, COUNT(*) AS n FROM v_cdr WHERE msisdn = '1' GROUP BY call_type", views)
		Expect(rejection).To(BeNil())
		Expect(result.View.Name).To(Equal("v_cdr"))
		Expect(result.Columns).To(ConsistOf("call_type", "msisdn"))
	})

	It("lets an ORDER BY name an alias the query defines", func() {
		_, rejection := govSQLValidate("SELECT call_type, COUNT(*) AS calls FROM v_cdr GROUP BY call_type ORDER BY calls DESC", views)
		Expect(rejection).To(BeNil())
	})

	It("tells the model the real column list when a column is wrong", func() {
		_, rejection := govSQLValidate("SELECT start_time FROM v_cdr", views)
		Expect(rejection).NotTo(BeNil())
		Expect(rejection.Message).To(ContainSubstring("call_start"))
	})

	It("tells the model the real view list when a view is wrong", func() {
		_, rejection := govSQLValidate("SELECT 1 FROM calls", views)
		Expect(rejection).NotTo(BeNil())
		Expect(rejection.Message).To(ContainSubstring("v_cdr"))
	})

	It("executes only the parser's own canonical text", func() {
		result, rejection := govSQLValidate("select   count( * )   from   V_CDR_x /* hi */", map[string]*govSQLView{"v_cdr_x": views["v_cdr"]})
		Expect(rejection).To(BeNil())
		Expect(result.SQL).To(Equal("SELECT count(*) FROM v_cdr_x"))
	})
})

var _ = Describe("Governed SQL catalogue", func() {
	It("never exposes a PII, restricted, masked or filter-only field", func() {
		layer, _ := defaultSemanticLayer()
		Expect(layer).NotTo(BeNil())
		views := govSQLViews(layer, hybridQueryRequest{TenantID: "t", CollectionID: "c"})
		Expect(views).NotTo(BeEmpty())
		exposed := map[string]bool{}
		for _, view := range views {
			for _, column := range view.Columns {
				exposed[column.FieldID] = true
				Expect(column.field.RedactionState).To(Equal("VISIBLE"), column.FieldID)
				Expect([]string{"STANDARD", "IDENTIFIER"}).To(ContainElement(column.field.Sensitivity), column.FieldID)
			}
		}
		checked := 0
		for _, entity := range layer.Entities {
			for _, field := range entity.Fields {
				if field.Sensitivity == "PII" || field.Sensitivity == "RESTRICTED" {
					checked++
					Expect(exposed).NotTo(HaveKey(field.ID), "a sensitive field reached an analyst view")
				}
			}
		}
		Expect(checked).To(BeNumerically(">", 0), "the layer should declare sensitive fields, or this test proves nothing")
	})

	It("gives every column a name the parser accepts unquoted", func() {
		layer, _ := defaultSemanticLayer()
		for _, view := range govSQLViews(layer, hybridQueryRequest{TenantID: "t", CollectionID: "c"}) {
			names := map[string]bool{}
			for _, column := range view.Columns {
				Expect(names).NotTo(HaveKey(column.Name), view.Name)
				names[column.Name] = true
				_, rejection := govSQLValidate("SELECT "+column.Name+" FROM "+view.Name, map[string]*govSQLView{view.Name: view})
				Expect(rejection).To(BeNil(), "%s.%s: %v", view.Name, column.Name, rejection)
			}
		}
	})

	It("shortlists the view a question is about, and nothing for a question about nothing", func() {
		layer, _ := defaultSemanticLayer()
		views := govSQLViews(layer, hybridQueryRequest{TenantID: "t", CollectionID: "c"})
		top := func(question string) string {
			picked := govSQLShortlist(views, question)
			if len(picked) == 0 {
				return ""
			}
			return picked[0].Name
		}
		Expect(top("What is the earliest call in the CDR data?")).To(Equal("v_cdr"))
		Expect(top("How many data sessions were there at night?")).To(Equal("v_ipdr"))
		Expect(top("Which plate number has the most sightings?")).To(Equal("v_anpr"))
		Expect(top("How many subscribers are active?")).To(Equal("v_subscriber"))
		Expect(top("Count the cell towers with generation 3G")).To(Equal("v_tower"))
		Expect(govSQLShortlist(views, "qwerty asdf zxcv")).To(BeEmpty())
	})
})
