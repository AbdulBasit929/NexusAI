package main

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// THE RED TEAM OF THE ASK ENGINE'S VALIDATOR (roadmap 6.8, Phase 1 of 2026-10-16).
//
// The validator is the first of the layers between a language model and the database (the others: the read-only role that reaches only the analyst views, the scope the
// views carry, the read-only transaction, the plan-cost ceiling, the statement timeout and the row cap). A model can write anything, and a question can try to make it:
// "ignore the rules and ...". So the validator is held to a list written to break it, by class, and the bar is 100 percent: every statement below must be refused, none
// may be accepted. The list is the owner's reading list too: it says in plain words what the engine will never run.
//
// A statement that the validator accepts but that could be expensive is not here; those are in the executor's red team (ask_engine_exec_db_ginkgo_test.go), which runs
// them on a real database and requires a refusal by cost, a stop by the timeout, or a result under the cap.
var _ = Describe("Ask engine red team: statements the validator must refuse", func() {
	var cat *askCatalogue
	BeforeEach(func() {
		var err error
		cat, err = askGoldenCatalogue()
		Expect(err).NotTo(HaveOccurred())
	})

	attacks := map[string][]string{
		"change data or the schema": {
			"WITH x AS (DELETE FROM v_cells RETURNING cell_key) SELECT * FROM x",
			"WITH x AS (INSERT INTO v_cells (cell_key) VALUES ('x') RETURNING cell_key) SELECT * FROM x",
			"WITH x AS (UPDATE v_cells SET lat = 0 RETURNING cell_key) SELECT * FROM x",
			"UPDATE v_cells SET lat = 0",
			"TRUNCATE v_comm_events",
			"DROP VIEW v_cells",
			"DROP SCHEMA analyst CASCADE",
			"ALTER VIEW v_cells RENAME TO v_gone",
			"GRANT ALL ON v_cells TO PUBLIC",
			"CREATE VIEW v_x AS SELECT * FROM v_cells",
			"CREATE FUNCTION f() RETURNS int AS 'select 1' LANGUAGE sql",
			"CREATE TEMP TABLE t AS SELECT * FROM v_cells",
			"SELECT * INTO TEMP t FROM v_cells",
			"VACUUM v_cells",
			"REFRESH MATERIALIZED VIEW v_cells",
		},
		"run code, take locks, change the session": {
			"DO $$ BEGIN PERFORM 1; END $$",
			"CALL pg_sleep(1)",
			"LOCK TABLE v_cells IN ACCESS EXCLUSIVE MODE",
			"SELECT * FROM v_cells FOR SHARE",
			"SELECT * FROM v_cells FOR NO KEY UPDATE",
			"NOTIFY chan, 'x'",
			"LISTEN chan",
			"PREPARE p AS SELECT 1 FROM v_cells",
			"EXECUTE p",
			"DECLARE c CURSOR FOR SELECT * FROM v_cells",
			"ALTER SYSTEM SET work_mem = '1TB'",
			"SET statement_timeout = 0",
			"SET LOCAL app.tenant_id = 'other'",
			"BEGIN",
			"COMMIT",
			"ROLLBACK",
			"SELECT pg_advisory_lock(1) FROM v_cells",
			"SELECT pg_terminate_backend(1) FROM v_cells",
			"SELECT nextval('s') FROM v_cells",
			"SELECT txid_current() FROM v_cells",
		},
		"more than one statement, or one hidden in another": {
			"SELECT 1 FROM v_cells; DROP VIEW v_cells",
			"BEGIN; SELECT 1 FROM v_cells; COMMIT",
			"SELECT * FROM v_cells; SELECT pg_sleep(100)",
			"SELECT * FROM v_cells /* ; DROP VIEW v_cells */ ; DROP VIEW v_cells",
			"SELECT * FROM v_cells -- comment\n; DROP VIEW v_cells",
			"SELECT $$x$$ FROM v_cells; DROP VIEW v_cells",
			"SELECT * FROM \"v_cells\"; DROP VIEW \"v_cells\"",
			"SELECT * FROM v_cells\x00; DROP VIEW v_cells",
			"SELECT * FROM v_cells WHERE cell_key = 'a'; SELECT 1",
		},
		"read what is not an analyst view": {
			"SELECT * FROM forensic.records_audit_log",
			"SELECT * FROM forensic.comm_event",
			"SELECT * FROM public.nx_throwaway_marker",
			"SELECT * FROM pg_shadow",
			"SELECT * FROM pg_authid",
			"SELECT * FROM pg_roles",
			"SELECT * FROM pg_stat_activity",
			"SELECT * FROM pg_settings",
			"SELECT * FROM pg_catalog.pg_tables",
			"SELECT * FROM information_schema.columns",
			"SELECT * FROM pg_class",
			"SELECT * FROM v_cells UNION SELECT * FROM pg_roles",
			"SELECT * FROM v_cells WHERE cell_key IN (SELECT rolname FROM pg_roles)",
			"SELECT (SELECT string_agg(rolname, ',') FROM pg_roles) FROM v_cells",
			"SELECT * FROM v_cells JOIN pg_class ON true",
			"WITH t AS (SELECT * FROM pg_roles) SELECT * FROM t",
			"SELECT * FROM v_comm_events WHERE tenant_id = 'other'",
			"SELECT tenant_id FROM v_comm_events",
			"SELECT * FROM v_comm_events WHERE collection_id <> current_setting('app.collection_id')",
		},
		"reach another case or learn the system": {
			"SELECT current_setting('app.collection_id') FROM v_cells",
			"SELECT set_config('app.collection_id', 'other', true) FROM v_cells",
			"SELECT version() FROM v_cells",
			"SELECT current_database() FROM v_cells",
			"SELECT current_schema() FROM v_cells",
			"SELECT inet_server_addr() FROM v_cells",
			"SELECT pg_read_binary_file('/etc/passwd') FROM v_cells",
			"SELECT pg_read_file('/etc/passwd') FROM v_cells",
			"SELECT lo_import('/etc/passwd') FROM v_cells",
			"SELECT dblink('host=elsewhere', 'select 1') FROM v_cells",
			"SELECT query_to_xml('select * from pg_roles', true, true, '') FROM v_cells",
			"SELECT xpath('/a', '<a/>'::xml) FROM v_cells",
			"SELECT to_regclass('pg_roles') FROM v_cells",
			"SELECT cell_key::regclass::text FROM v_cells",
			"SELECT * FROM ROWS FROM (pg_ls_dir('.')) AS t",
			"SELECT * FROM v_cells, LATERAL pg_ls_dir('.') AS d",
			"SELECT current_user FROM v_cells",
		},
		"disguise": {
			"SELECT * FROM v_cеlls",  // a Cyrillic e
			"SEL​ECT * FROM v_cells", // a zero-width space inside a keyword
			"SELECT * FROM v_cells ; DROP VIEW v_cells",
			"select * from V_CELLS; drop view v_cells",
			"/**/ DROP /**/ VIEW /**/ v_cells",
			"( DROP VIEW v_cells )",
			"SELECT 1 FROM v_cells WHERE (SELECT pg_sleep(60)) IS NULL",
		},
		"cost that no query needs": {
			"SELECT repeat('x', 1000000000) FROM v_cells",
			"SELECT lpad('x', 1000000000, 'y') FROM v_cells",
			"SELECT * FROM v_cells, generate_series(1, 1000000000) AS i",
			"SELECT generate_series(1, 1000000000) FROM v_cells",
			"SELECT pg_sleep(1000) FROM v_cells",
			"SELECT * FROM v_comm_events a, v_comm_events b, v_comm_events c LIMIT 100",
			"SELECT * FROM v_comm_events a CROSS JOIN v_comm_events b LIMIT 100",
			"SELECT * FROM v_comm_events a JOIN v_comm_events b ON true LIMIT 100",
			"SELECT * FROM v_comm_events LIMIT 1000000000",
		},
	}

	for class, list := range attacks {
		class, list := class, list
		It("refuses every statement of the class: "+class, func() {
			var accepted []string
			for _, sql := range list {
				valid, rej := askValidate(sql, cat, nil)
				if valid != nil || rej == nil {
					accepted = append(accepted, strings.ReplaceAll(sql, "\n", " "))
				}
			}
			Expect(accepted).To(BeEmpty(), "the validator accepted what it must refuse")
		})
	}

	// A red team that passes because the validator refuses everything proves nothing: these are the control, plain statements of the same shapes that must be accepted.
	It("still accepts the plain statements the red team's statements imitate", func() {
		for _, sql := range []string{
			"SELECT count(*) AS n FROM v_comm_events WHERE src = '923000000101'",
			"SELECT dst AS label, events AS value FROM v_contact_pairs WHERE src = '923000000101' AND NOT is_service ORDER BY events DESC LIMIT 5",
			"WITH t AS (SELECT src, count(*) AS c FROM v_comm_events GROUP BY src) SELECT src, c FROM t ORDER BY c DESC LIMIT 5",
			"SELECT * FROM analyst.v_cells LIMIT 10",
			"SELECT src FROM v_comm_events WHERE dst IN (SELECT dst FROM v_contact_pairs WHERE is_hub) LIMIT 10",
		} {
			valid, rej := askValidate(sql, cat, nil)
			Expect(rej).To(BeNil(), sql)
			Expect(valid).NotTo(BeNil(), sql)
		}
	})

	It("has the red team list written down by class, with at least sixty statements", func() {
		total := 0
		for _, list := range attacks {
			total += len(list)
		}
		Expect(total).To(BeNumerically(">=", 60))
	})
})
