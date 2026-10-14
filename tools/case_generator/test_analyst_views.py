"""Migration 017, the analyst views and the read-only analyst role (roadmap 6.8.3 and 6.8.9, decision D19), on a THROWAWAY PostgreSQL with a generated case
written by the PRODUCTION typed ingest. Plain unittest, about four minutes (skipped when Docker or the image is absent):

    python -X utf8 tools/case_generator/test_analyst_views.py

What it proves:
  * the ops scripts pass on a database built the way a fresh one is; the migration applies twice and rolls back and forward;
  * the views, written to as a person (or the model) would write the question, give the planted-truth keys: A1 (contact profile) from v_contact_pairs,
    A2 (common contacts, hubs and services left out by the views' own flags) and A13 (who held an address and port at an instant) from v_data_sessions;
  * scope is structural: through the read-only role the views return the case's rows for the right settings, nothing for another tenant, another collection
    or unset settings (fail closed);
  * the role reaches the views and nothing else: no base table, no other schema, no write; and the application's login role can SET LOCAL ROLE to it;
  * verify.sql FAILS when the role is given a base table, a description is missing, a view loses its tenant filter, or an unreviewed relation appears.
"""
import os
import sys
import tempfile
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
import generate  # noqa: E402
import load_case  # noqa: E402
import world as w  # noqa: E402
from ingestion.forensic_records.layout_registry import load_registry  # noqa: E402
from test_ingest_v2 import PsqlSink, ingest_case  # noqa: E402
from test_load_case import docker_ready  # noqa: E402

OPS = "/tmp/mig/ops/017_analyst_views/"
MIGRATION = "/tmp/mig/017_analyst_views.sql"
LOCAL = "'YYYY-MM-DD HH24:MI:SS'"


def as_analyst(settings, query):
    """A script that runs one query the way the application will: a read-only transaction, the role, the scope settings."""
    lines = ["BEGIN READ ONLY;", "SET LOCAL ROLE forensic_analyst_ro;", "SET LOCAL statement_timeout = '20s';"]
    for name, value in settings.items():
        lines.append("SELECT set_config('%s', %s, true);" % (name, load_case.sql_text(value)))
    lines += ["SELECT coalesce(jsonb_agg(t), '[]'::jsonb) FROM (%s) t;" % query.strip().rstrip(";"), "COMMIT;"]
    return "\n".join(lines)


@unittest.skipUnless(docker_ready(), "Docker or the image %s is not available on this machine" % load_case.IMAGE)
class AnalystViewTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.tmp = tempfile.TemporaryDirectory()
        cls.params = w.Params(seed=61, people=60, subjects=6, associates=12, period_days=20, p2_pairs=3, p3_pairs=2, p7_plants=3)
        cls.root = generate.write_case(cls.params, cls.tmp.name)
        cls.registry = load_registry()
        cls.db = load_case.Throwaway()
        try:
            cls.db.start()
            load_case.prepare(cls.db, cls.registry)
            cls.key, cls.stats, cls.summaries = ingest_case(cls.db, cls.root, cls.registry, PsqlSink(cls.db))
            # cell reference rows: the case's own list (preferred) and a tenant-wide list that disagrees, so the view's choice can be seen
            cls.db.psql("-c", "INSERT INTO forensic.cell_site (tenant_id, collection_id, cell_key, plmn, lac_tac, ci_eci, site_name, lat, lon, radius_m, source) "
                              "SELECT %s, %s, k, split_part(k, ':', 1), split_part(k, ':', 2), split_part(k, ':', 3), 'Case site ' || split_part(k, ':', 3), 33.0, 73.0, 800, 'test-case' "
                              "FROM (SELECT DISTINCT cell_key AS k FROM forensic.comm_event WHERE tenant_id = %s AND collection_id = %s AND cell_key IS NOT NULL) q"
                % (load_case.sql_text(load_case.TENANT), load_case.sql_text(cls.key["case_id"]), load_case.sql_text(load_case.TENANT), load_case.sql_text(cls.key["case_id"])))
            cls.db.psql("-c", "INSERT INTO forensic.cell_site (tenant_id, collection_id, cell_key, plmn, lac_tac, ci_eci, site_name, lat, lon, radius_m, source, source_rank) "
                              "SELECT %s, '', k, split_part(k, ':', 1), split_part(k, ':', 2), split_part(k, ':', 3), 'Tenant-wide site ' || split_part(k, ':', 3), 34.0, 74.0, 5000, 'test-tenant', 1 "
                              "FROM (SELECT DISTINCT cell_key AS k FROM forensic.comm_event WHERE tenant_id = %s AND collection_id = %s AND cell_key IS NOT NULL) q"
                % (load_case.sql_text(load_case.TENANT), load_case.sql_text(load_case.TENANT), load_case.sql_text(cls.key["case_id"])))
            cls.db.psql("-f", OPS + "preflight.sql")
            cls.db.psql("-f", MIGRATION)
            cls.settings = {"app.tenant_id": load_case.TENANT, "app.collection_id": cls.key["case_id"],
                            "app.hub_threshold": str(cls.key["oracle"]["parameters"]["hub_threshold"])}
        except Exception:
            cls.db.stop()
            cls.tmp.cleanup()
            raise

    @classmethod
    def tearDownClass(cls):
        cls.left = cls.db.stop()
        cls.tmp.cleanup()
        if cls.left:
            raise AssertionError(cls.left)

    # ---- helpers ---------------------------------------------------------------------------------------------------------------------------
    def run_ops(self, name):
        return self.db.psql("-f", OPS + name)

    def ask(self, query, settings=None):
        """Run one query as the read-only role with the case's settings (or the given ones) and return its rows."""
        import json
        out = self.db.psql("-At", "-f", "-", input=as_analyst(self.settings if settings is None else settings, query).encode())
        json_lines = [line for line in out.splitlines() if line.startswith("[")]
        self.assertEqual(len(json_lines), 1, out)
        return json.loads(json_lines[0])

    def refused(self, query, settings=None):
        with self.assertRaises(RuntimeError) as caught:
            self.db.psql("-At", "-f", "-", input=as_analyst(self.settings if settings is None else settings, query).encode())
        return str(caught.exception)

    def run_ops_expecting_failure(self, name, needle):
        with self.assertRaises(RuntimeError) as caught:
            self.run_ops(name)
        self.assertIn(needle, str(caught.exception))

    # ---- the migration -------------------------------------------------------------------------------------------------------------------------
    def test_a_the_ops_scripts_pass_and_the_migration_applies_twice(self):
        self.run_ops("verify.sql")
        self.db.psql("-f", MIGRATION)
        self.run_ops("verify.sql")

    # ---- the keys, through the views -----------------------------------------------------------------------------------------------------
    def test_b_a1_contact_profile_through_v_contact_pairs_matches_the_key(self):
        for subject in self.key["subjects"]:
            rows = self.ask("SELECT dst AS number, outgoing, incoming, unknown, coalesce(total_duration_s, 0)::int AS voice_seconds, "
                            "to_char(first_seen AT TIME ZONE 'Asia/Karachi', %s) AS first, to_char(last_seen AT TIME ZONE 'Asia/Karachi', %s) AS last, is_service "
                            "FROM analyst.v_contact_pairs WHERE src = %s" % (LOCAL, LOCAL, load_case.sql_text(subject["msisdn"])))
            got = {r["number"]: (r["outgoing"], r["incoming"], r["unknown"], r["voice_seconds"], r["first"], r["last"]) for r in rows}
            want = {r["number"]: (r["outgoing"], r["incoming"], r["unknown"], r["voice_seconds"], r["first"], r["last"]) for r in self.key["oracle"]["A1"][subject["msisdn"]]}
            self.assertGreater(len(want), 0)
            self.assertEqual({n: got.get(n) for n in want}, want, "%s: counterparties whose counts or times differ" % subject["person"])
            extras = [r["number"] for r in rows if r["number"] not in want]
            self.assertEqual(extras, [], "%s: the views must list exactly the counterparties of the key (a sentinel such as -1 is not one)" % subject["person"])

    def test_c_a2_common_contacts_through_the_views_match_the_key_hubs_and_services_left_out_by_the_flags(self):
        self.assertGreater(len(self.key["oracle"]["A2"]), 0)
        for answer in self.key["oracle"]["A2"]:
            rows = self.ask("SELECT x.dst AS number, x.events AS x_count, y.events AS y_count, "
                            "to_char(x.first_seen AT TIME ZONE 'Asia/Karachi', %s) AS x_first, to_char(x.last_seen AT TIME ZONE 'Asia/Karachi', %s) AS x_last, "
                            "to_char(y.first_seen AT TIME ZONE 'Asia/Karachi', %s) AS y_first, to_char(y.last_seen AT TIME ZONE 'Asia/Karachi', %s) AS y_last "
                            "FROM analyst.v_contact_pairs x JOIN analyst.v_contact_pairs y ON y.dst = x.dst "
                            "WHERE x.src = %s AND y.src = %s AND NOT x.is_service AND NOT x.is_hub AND x.dst NOT IN (%s, %s) ORDER BY 1"
                            % (LOCAL, LOCAL, LOCAL, LOCAL, load_case.sql_text(answer["x"]), load_case.sql_text(answer["y"]),
                               load_case.sql_text(answer["x"]), load_case.sql_text(answer["y"])))
            got = [(r["number"], r["x_count"], r["y_count"], r["x_first"], r["x_last"], r["y_first"], r["y_last"]) for r in rows]
            want = [(s["number"], s["x_count"], s["y_count"], s["x_first"], s["x_last"], s["y_first"], s["y_last"]) for s in answer["shared"]]
            self.assertEqual(got, want, "A2 %s / %s" % (answer["x"][-4:], answer["y"][-4:]))
        # the flags are what leave the decoys out: without them the planted hub or service would appear
        hubs = self.ask("SELECT count(DISTINCT dst) AS n FROM analyst.v_contact_pairs WHERE is_hub")[0]["n"]
        services = self.ask("SELECT count(DISTINCT dst) AS n FROM analyst.v_contact_pairs WHERE is_service")[0]["n"]
        self.assertGreater(hubs, 0, "the case plants a hub number")
        self.assertGreater(services, 0, "the case plants a service number")

    def test_d_a13_address_holders_through_v_data_sessions_match_the_key(self):
        self.assertGreater(len(self.key["oracle"]["A13"]), 0)
        for answer in self.key["oracle"]["A13"]:
            rows = self.ask("SELECT coalesce(json_agg(DISTINCT src ORDER BY src), '[]'::json) AS holders FROM analyst.v_data_sessions "
                            "WHERE public_ip = %s::inet AND %d BETWEEN public_port_from AND public_port_to "
                            "AND (timestamp %s AT TIME ZONE 'Asia/Karachi') BETWEEN holds_from AND holds_to"
                            % (load_case.sql_text(answer["public_ip"]), answer["port"], load_case.sql_text(answer["instant"])))
            self.assertEqual(rows[0]["holders"], answer["holders"], "A13 %s %s" % (answer["plant"], answer["label"]))

    def test_e_the_other_views_return_rows_in_the_documented_shape(self):
        for view, where in (("v_comm_events", ""), ("v_cells", ""), ("v_cell_visits", ""), ("v_device_pairings", ""), ("v_data_sessions", ""), ("v_app_usage", "")):
            n = self.ask("SELECT count(*) AS n FROM analyst.%s %s" % (view, where))[0]["n"]
            self.assertGreater(n, 0, "analyst.%s is empty for the generated case" % view)
        self.assertEqual(self.ask("SELECT count(*) AS n FROM analyst.v_comm_events")[0]["n"],
                         int(self.db.sql("SELECT count(*) FROM forensic.comm_event WHERE %s AND dataset_kind = 'target_cdr' AND %s" % (load_case.scope(self.key), load_case.NOT_DUPLICATE))))
        sites = self.ask("SELECT count(*) AS n, count(*) FILTER (WHERE site_name LIKE 'Case site%') AS own FROM analyst.v_cells")[0]
        self.assertGreater(sites["n"], 0)
        self.assertEqual(sites["n"], sites["own"], "the case's own cell list wins over the tenant-wide one, whatever the source rank, and each cell appears once")
        self.assertEqual(self.ask("SELECT count(*) - count(DISTINCT cell_key) AS d FROM analyst.v_cells")[0]["d"], 0)
        visits = self.ask("SELECT sum(events)::int AS n, count(*) FILTER (WHERE radius_m IS NULL) AS no_radius FROM analyst.v_cell_visits")[0]
        self.assertGreater(visits["n"], 0)

    def test_e2_the_address_interval_follows_the_translation_binding_when_there_is_one(self):
        """The planted sessions may all have a binding equal to the session; so a binding that differs is made inside a transaction that is rolled back."""
        script = "\n".join([
            "BEGIN;",
            "UPDATE forensic.data_session SET nat_start_utc = start_utc - interval '1 hour', nat_end_utc = coalesce(end_utc, start_utc) + interval '2 hours' "
            "WHERE session_id = (SELECT min(session_id) FROM forensic.data_session WHERE %s);" % load_case.scope(self.key),
            "SET LOCAL ROLE forensic_analyst_ro;",
            "SELECT set_config('app.tenant_id', %s, true);" % load_case.sql_text(load_case.TENANT),
            "SELECT set_config('app.collection_id', %s, true);" % load_case.sql_text(self.key["case_id"]),
            "SELECT count(*) FROM analyst.v_data_sessions v WHERE v.session_id = (SELECT min(session_id) FROM analyst.v_data_sessions) "
            "AND v.holds_from = v.start_utc - interval '1 hour' AND v.holds_to = coalesce(v.end_utc, v.start_utc) + interval '2 hours';",
            "ROLLBACK;",
        ])
        out = self.db.psql("-At", "-f", "-", input=script.encode())
        self.assertIn("1", [line.strip() for line in out.splitlines()], out)

    # ---- scope is structural ---------------------------------------------------------------------------------------------------------------
    def test_f_scope_comes_from_the_settings_and_fails_closed(self):
        own = self.ask("SELECT count(*) AS n FROM analyst.v_comm_events")[0]["n"]
        self.assertGreater(own, 0)
        other_tenant = dict(self.settings, **{"app.tenant_id": "another-tenant"})
        other_collection = dict(self.settings, **{"app.collection_id": "another-collection"})
        for name, settings in (("another tenant", other_tenant), ("another collection", other_collection), ("no settings", {}),
                               ("tenant only", {"app.tenant_id": load_case.TENANT}), ("collection only", {"app.collection_id": self.key["case_id"]})):
            for view in ("v_comm_events", "v_contact_pairs", "v_cell_visits", "v_device_pairings", "v_data_sessions", "v_app_usage"):
                n = self.ask("SELECT count(*) AS n FROM analyst.%s" % view, settings)[0]["n"]
                self.assertEqual(n, 0, "analyst.%s leaked rows for %s" % (view, name))

    def test_g_the_role_reaches_the_views_and_nothing_else(self):
        for relation in ("forensic.comm_event", "forensic.records", "forensic.evidence_items", "forensic.cell_site", "forensic.layout_versions"):
            message = self.refused("SELECT count(*) FROM %s" % relation)
            self.assertIn("permission denied", message, relation)
        with self.assertRaises(RuntimeError) as caught:
            self.db.psql("-At", "-c", "BEGIN; SET LOCAL ROLE forensic_analyst_ro; INSERT INTO analyst.v_comm_events (event_type) VALUES ('x'); COMMIT;")
        self.assertIn("permission denied", str(caught.exception))
        for statement in ("UPDATE analyst.v_comm_events SET event_type = 'x'", "DELETE FROM analyst.v_comm_events"):
            with self.assertRaises(RuntimeError) as caught:
                self.db.psql("-At", "-c", "BEGIN; SET LOCAL ROLE forensic_analyst_ro; %s; COMMIT;" % statement)
            self.assertIn("permission denied", str(caught.exception), statement)
        self.assertIn("read-only", self._write_attempt_message())

    def _write_attempt_message(self):
        with self.assertRaises(RuntimeError) as caught:
            self.db.psql("-At", "-c", "BEGIN READ ONLY; SET LOCAL ROLE forensic_analyst_ro; CREATE TABLE analyst.x (a int); COMMIT;")
        return str(caught.exception)

    def test_h_the_application_login_role_can_use_the_analyst_role_and_then_only_the_views(self):
        script = ("BEGIN READ ONLY;\nSET LOCAL ROLE forensic_analyst_ro;\nSELECT set_config('app.tenant_id', %s, true);\nSELECT set_config('app.collection_id', %s, true);\n"
                  "SELECT count(*) FROM analyst.v_comm_events;\nCOMMIT;\n" % (load_case.sql_text(load_case.TENANT), load_case.sql_text(self.key["case_id"])))
        proc = self.db.docker("exec", "-i", self.db.name, "psql", "-v", "ON_ERROR_STOP=1", "-U", "forensic_runtime", "-d", "localrecall", "-At", "-f", "-",
                              input=script.encode(), check=False)
        if proc.returncode != 0 and b"authentication" in proc.stderr + proc.stdout:
            self.skipTest("the throwaway database does not allow a socket login as forensic_runtime")
        self.assertEqual(proc.returncode, 0, (proc.stderr + proc.stdout).decode("utf-8", "replace"))
        counts = [line for line in proc.stdout.decode().split() if line.isdigit()]
        self.assertEqual(counts, [self.db.sql("SELECT count(*) FROM forensic.comm_event WHERE %s AND dataset_kind = 'target_cdr' AND %s" % (load_case.scope(self.key), load_case.NOT_DUPLICATE))])
        blocked = self.db.docker("exec", "-i", self.db.name, "psql", "-v", "ON_ERROR_STOP=1", "-U", "forensic_runtime", "-d", "localrecall", "-At", "-c",
                                 "BEGIN READ ONLY; SET LOCAL ROLE forensic_analyst_ro; SELECT count(*) FROM forensic.records; COMMIT;", check=False)
        self.assertNotEqual(blocked.returncode, 0)
        self.assertIn(b"permission denied", blocked.stderr)
        # the application's own login role has nothing to escalate to: it may not become the owner or any role it is not a member of
        for target in ("localrecall",):
            escalate = self.db.docker("exec", "-i", self.db.name, "psql", "-v", "ON_ERROR_STOP=1", "-U", "forensic_runtime", "-d", "localrecall", "-At", "-c",
                                      "BEGIN READ ONLY; SET LOCAL ROLE forensic_analyst_ro; SET LOCAL ROLE %s; COMMIT;" % target, check=False)
            self.assertNotEqual(escalate.returncode, 0, target)
            self.assertIn(b"permission denied", escalate.stderr, target)

    # ---- verify.sql must notice damage ---------------------------------------------------------------------------------------------------
    def test_y_verify_fails_for_each_kind_of_damage_and_the_migration_repairs_it(self):
        def damage(sql, needle):
            try:
                self.db.psql("-c", sql)
                self.run_ops_expecting_failure("verify.sql", needle)
            finally:
                self.db.psql("-c", "DROP TABLE IF EXISTS analyst.x_unreviewed")
                self.db.psql("-f", MIGRATION)
            self.run_ops("verify.sql")

        damage("GRANT SELECT ON forensic.comm_event TO forensic_analyst_ro", "outside the analyst schema")
        damage("GRANT USAGE ON SCHEMA forensic TO forensic_analyst_ro", "must not have USAGE on schema forensic")
        damage("GRANT INSERT ON analyst.v_cells TO forensic_analyst_ro", "can write to")
        damage("COMMENT ON COLUMN analyst.v_cells.radius_m IS NULL", "columns without a description")
        damage("COMMENT ON VIEW analyst.v_cells IS NULL", "views without a description")
        damage("CREATE TABLE analyst.x_unreviewed (a int)", "an unreviewed relation")
        # a view that lost its tenant filter
        damage("CREATE OR REPLACE VIEW analyst.v_app_usage AS SELECT usage_id, evidence_id, msisdn AS src, imei, app_category, app, app_feature, start_utc, end_utc, province, city "
               "FROM forensic.app_usage WHERE collection_id = current_setting('app.collection_id', true)", "do not filter on app.tenant_id")
        damage("ALTER ROLE forensic_analyst_ro LOGIN", "must be NOLOGIN")

    def test_z_rollback_removes_everything_and_the_migration_comes_back(self):
        self.run_ops("rollback.sql")
        self.assertEqual(self.db.sql("SELECT (to_regnamespace('analyst') IS NULL) AND NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'forensic_analyst_ro')"), "t")
        self.run_ops("preflight.sql")
        self.db.psql("-f", MIGRATION)
        self.run_ops("verify.sql")
        self.assertEqual(self.ask("SELECT count(*) > 0 AS ok FROM analyst.v_comm_events")[0]["ok"], True)


if __name__ == "__main__":
    unittest.main()
