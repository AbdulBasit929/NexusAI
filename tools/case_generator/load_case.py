#!/usr/bin/env python3
"""Load a generated case into a THROWAWAY PostgreSQL and compare SQL answers with the oracles (specification section 12, step 5).

    python -X utf8 tools/case_generator/load_case.py CASE_DIR [--keep]

It starts a disposable container from an image that is already on this machine (it never pulls), with no published port (unless a caller opts in) and a random
password, applies migrations 001 to 015 and the runtime-role grants as a fresh database sees them, registers the 13 layouts, and loads
the case into forensic.comm_event, forensic.data_session and forensic.app_usage. It then checks, with SQL:

  reconciliation  per export: rows read = loaded + quarantined + skipped, repeats flagged, and the quarantine and repeat counts equal the key's
  A1 contact profile of every subject      A2 common contacts of every pair      A5 co-location windows and pairs of every same-operator pair
  A5 chance baseline (expected pairs and lift)                                  A7 possible meetings (windows preceded by a direct contact)
  A6 tower-dump population                 A13 who held each probed public address and port

This loader is TEST-ONLY and independent of the product: it is the reference. The product's worker has its own shadow writer (ingestion/forensic_records/typed_ingest.py,
behind FORENSIC_INGEST_V2, off by default); test_ingest_v2.py runs these same checks on what that code writes. This proves the schema
and the oracles against real SQL until it does. It never touches the NexusAI stack, and the container and its volume are removed at the end.
"""
import argparse
import csv
import hashlib
import io
import json
import os
import subprocess
import sys
import tempfile
import time
import uuid

HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.abspath(os.path.join(HERE, "..", ".."))
sys.path.insert(0, HERE)
import audit  # noqa: E402  (also puts the repository and the vault tools on the path)
from ingestion.forensic_records.layout_registry import Policy, interpret_rows, load_registry  # noqa: E402

IMAGE = "timescale/timescaledb:latest-pg16"
MIGRATIONS = ["001_phase1_records.sql", "002_job_row_counters.sql", "003_phase2_kb_assets_and_entities.sql", "004_dynamic_records.sql",
              "005_backfill_canonical_records.sql", "006_canonical_normalized_fields.sql", "007_evidence_items.sql",
              "008_phase3_evidence_control_plane.sql", "009_phase3_queue_lifecycle.sql", "010_runtime_rls_policies.sql",
              "011_r5_custody_chain_tail.sql", "runtime_role.grants.sql", "012_typed_families.sql", "013_reference_tables.sql",
              "014_entity_relationship_layer.sql", "015_vectors_and_earth.sql", "016_effective_records.sql", "029_bank_transactions.sql"]   # 029: the table and the layout kind, which layouts need before they are registered
TENANT = "case"
TABLE_OF = {"call": "comm_event", "session": "data_session", "usage": "app_usage", "plate": "plate_sighting", "subscriber": "subscriber_registration", "transaction": "bank_transaction"}
NOT_DUPLICATE = "NOT ('duplicate_row' = ANY(quality_flags))"
LOCAL = "'YYYY-MM-DD HH24:MI:SS'"


def sql_text(value):
    return "NULL" if value is None else "'" + str(value).replace("'", "''") + "'"


class Throwaway:
    """A disposable PostgreSQL container: local image only, no published port, random password, removed with its volume."""

    def __init__(self, image=IMAGE, memory_mb=1536, publish_port=None):
        # publish_port: opt-in, for the one check that a program on this machine (the Go specs) must connect to; it is published on 127.0.0.1 only.
        self.image, self.memory_mb, self.publish_port = image, memory_mb, publish_port
        self.name = "nx-caseload-" + uuid.uuid4().hex[:8]
        self.password = uuid.uuid4().hex
        self.started = False

    def docker(self, *args, input=None, check=True):
        proc = subprocess.run(["docker"] + list(args), input=input, capture_output=True)
        if check and proc.returncode != 0:
            raise RuntimeError("docker %s failed: %s" % (" ".join(args[:3]), proc.stderr.decode("utf-8", "replace")[-600:]))
        return proc

    def start(self):
        if self.docker("image", "inspect", self.image, check=False).returncode != 0:
            raise RuntimeError("the image %s is not on this machine; this tool never pulls" % self.image)
        publish = ["-p", "127.0.0.1:%d:5432" % self.publish_port] if self.publish_port else []
        self.docker("run", "-d", "--rm", "--name", self.name, "--label", "nexusai.purpose=case-load-test", "--memory", "%dm" % self.memory_mb, *publish,
                    "-e", "POSTGRES_USER=localrecall", "-e", "POSTGRES_DB=localrecall", "-e", "POSTGRES_PASSWORD=" + self.password, self.image)
        self.started = True
        for _ in range(90):
            if self.docker("exec", self.name, "pg_isready", "-h", "127.0.0.1", "-U", "localrecall", "-d", "localrecall", check=False).returncode == 0:
                time.sleep(2)   # the image restarts the server once after initialisation
                if self.docker("exec", self.name, "pg_isready", "-h", "127.0.0.1", "-U", "localrecall", "-d", "localrecall", check=False).returncode == 0:
                    return
            time.sleep(2)
        raise RuntimeError("the throwaway database did not become ready")

    def psql(self, *extra, input=None):
        proc = self.docker("exec", "-i", self.name, "psql", "-v", "ON_ERROR_STOP=1", "-U", "localrecall", "-d", "localrecall", "-q", *extra, input=input, check=False)
        if proc.returncode != 0:
            raise RuntimeError("psql failed: " + (proc.stderr + proc.stdout).decode("utf-8", "replace")[-900:])
        return proc.stdout.decode("utf-8", "replace")

    def sql(self, text):
        return self.psql("-At", "-c", text).strip()

    def rows(self, text):
        """Run a SELECT and return its rows as a list of dicts."""
        out = self.psql("-At", "-c", "SELECT coalesce(json_agg(t), '[]'::json) FROM (%s) t" % text.strip().rstrip(";")).strip()
        return json.loads(out)

    def script(self, text):
        return self.psql("-f", "-", input=text.encode("utf-8"))

    def stop(self):
        """Removes the container and its anonymous volume; returns None when both are gone, else a message."""
        if not self.started:
            return None
        mounts = self.docker("inspect", "--format", "{{json .Mounts}}", self.name, check=False)
        volumes = []
        if mounts.returncode == 0 and mounts.stdout.strip():
            volumes = [m["Name"] for m in json.loads(mounts.stdout) if m.get("Type") == "volume"]
        self.docker("rm", "-f", "-v", self.name, check=False)
        self.started = False
        if self.docker("ps", "-a", "-q", "--filter", "name=" + self.name, check=False).stdout.strip():
            return "container %s is still present" % self.name
        for volume in volumes:
            if self.docker("volume", "inspect", volume, check=False).returncode == 0:
                return "volume %s is still present" % volume
        return None


def prepare(db, registry):
    """Migrations 001 to 015, the runtime-role grants and the 13 layouts, in a fresh database."""
    db.docker("cp", os.path.join(REPO, "db", "forensic_records"), "%s:/tmp/mig" % db.name)
    for name in MIGRATIONS:
        db.psql("-f", "/tmp/mig/" + name)
    db.script(registry.register_sql())


def content_of(record):
    ignored = {"source_row", "raw_payload", "quality_flags"}
    return json.dumps({k: v for k, v in record.items() if k not in ignored}, sort_keys=True, default=str)


def copy_script(table, columns, rows):
    buf = io.StringIO()
    writer = csv.writer(buf, lineterminator="\n")
    for row in rows:
        out = []
        for column in columns:
            value = row.get(column)
            if value is None:
                out.append("\\N")
            elif isinstance(value, (dict, list)) and column in ("raw_payload", "raw_payload_redacted"):
                out.append(json.dumps(value, sort_keys=True, ensure_ascii=True, default=str))
            elif isinstance(value, list):
                out.append("{" + ",".join('"%s"' % str(v).replace('"', '\\"') for v in value) + "}")
            else:
                out.append(value)
        writer.writerow(out)
    return "COPY forensic.%s (%s) FROM STDIN WITH (FORMAT csv, NULL '\\N');\n%s\\.\n" % (table, ", ".join(columns), buf.getvalue())


def export_list(key):
    """Every export of a case as (key entry, family, name): the target call records, the tower dumps, the data sessions, the address queries and the app usage."""
    return ([(item, "call", item["person"]) for item in key["subjects"]] + [(item, "call", item["name"]) for item in key["dumps"]] +
            [(item, "session", item["name"]) for item in key["sessions"]] + [(item, "session", item["name"]) for item in key["ip_queries"]] +
            [(item, "usage", item["name"]) for item in key["app_usage"]] + ([(key["anpr"], "plate", "anpr")] if key.get("anpr") else []) +
            [(item, "subscriber", item["name"]) for item in key.get("subscribers", [])] + [(item, "transaction", item["name"]) for item in key.get("transactions", [])])


def lineage_ids(case_id, rel):
    """The evidence id and the batch id the loaders give an export: fixed by the case and the file, so a second load meets the same keys."""
    return (str(uuid.uuid5(uuid.NAMESPACE_URL, "nexusai-case/%s/%s" % (case_id, rel))), str(uuid.uuid5(uuid.NAMESPACE_URL, "nexusai-case-batch/%s/%s" % (case_id, rel))))


def evidence_statement(evidence_id, case_id, filename, extension, size, sha):
    """The evidence item the typed rows point at (the tables' foreign key)."""
    return ("INSERT INTO forensic.evidence_items (evidence_id, tenant_id, collection_id, case_id, user_id, original_filename, source_file, content_type, extension, "
            "size_bytes, sha256, modality, detected_type, classifier_confidence, processing_route, processing_status, raw_storage_ref, metadata) VALUES "
            "(%s, %s, %s, %s, 'case-loader', %s, %s, 'application/octet-stream', %s, %d, %s, 'structured_records', 'cdr', 1, 'forensic_records_worker', 'completed', %s, "
            "jsonb_build_object('request_id', 'case-load'));" % (sql_text(evidence_id), sql_text(TENANT), sql_text(case_id), sql_text("SYNTHETIC-" + case_id), sql_text(filename),
                                                               sql_text(filename), sql_text(extension), size, sql_text(sha), sql_text("immutable://case/sha256/" + sha)))


def load_case(db, case_dir, registry):
    """Read every export through the registry and load the accepted rows. Returns (key, per-export statistics)."""
    with open(os.path.join(case_dir, "key", "key.json"), encoding="utf-8") as handle:
        key = json.load(handle)
    with open(os.path.join(case_dir, "manifest.json"), encoding="utf-8") as handle:
        manifest = json.load(handle)
    case_id = key["case_id"]
    columns = {}
    for table in TABLE_OF.values():
        columns[table] = [r["column_name"] for r in db.rows(
            "SELECT column_name FROM information_schema.columns WHERE table_schema = 'forensic' AND table_name = %s "
            "AND is_generated = 'NEVER' AND column_default IS DISTINCT FROM 'nextval' AND identity_generation IS NULL ORDER BY ordinal_position" % sql_text(table))]
    exports = export_list(key)
    stats, bulk, evidence = {}, {t: [] for t in TABLE_OF.values()}, []
    for item, family, name in exports:
        rel = item["file"]
        rows = audit.read_export(os.path.join(case_dir, *rel.split("/")))
        report = registry.find_header(rows)
        entry = registry.get(item["layout_id"])
        # the registration and statement exports carry dd/mm/yyyy dates in some rows: the job says so for those families (as test_subscriber_oracle and test_txn_oracle do)
        policy = Policy(source_tz=audit.POLICY.source_tz, tz_basis=audit.POLICY.tz_basis, date_order="DMY") if family in ("subscriber", "transaction") else audit.POLICY
        results = list(interpret_rows(entry, rows[report.header_row], rows[report.header_row + 1:], policy=policy))
        evidence_id, batch_id = lineage_ids(case_id, rel)
        sha = manifest["files"][rel]["sha256"]
        evidence.append((evidence_id, os.path.basename(rel), os.path.splitext(rel)[1].lstrip("."), manifest["files"][rel]["bytes"], sha))
        counts = {"name": name, "family": family, "evidence_id": evidence_id, "table": TABLE_OF[family], "read": len(results), "loaded": 0, "quarantined": 0,
                  "skipped": 0, "duplicates": 0}
        seen = set()
        table = TABLE_OF[family]
        for result in results:
            if result.status == "quarantined":
                counts["quarantined"] += 1
                continue
            if result.status != "accepted":
                counts["skipped"] += 1
                continue
            record = dict(result.record)
            flags = list(record.get("quality_flags") or [])
            content = content_of(record)
            content_hash = hashlib.sha256(content.encode("utf-8")).hexdigest()
            if content_hash in seen:
                flags.append("duplicate_row")
                counts["duplicates"] += 1
            seen.add(content_hash)
            row = {k: v for k, v in record.items() if k in columns[table]}
            row.update({"tenant_id": TENANT, "collection_id": case_id, "evidence_id": evidence_id, "file_id": rel, "batch_id": batch_id,
                        "content_hash": content_hash, "row_hash": hashlib.sha256(("%s|%s|%s" % (rel, record["source_row"], content)).encode("utf-8")).hexdigest(),
                        "quality_flags": flags, "raw_payload": record["raw_payload"]})
            bulk[table].append(row)
            counts["loaded"] += 1
        stats[name] = counts
    statements = ["SELECT set_config('app.tenant_id', %s, false);" % sql_text(TENANT)]
    for evidence_id, filename, extension, size, sha in evidence:
        statements.append(evidence_statement(evidence_id, case_id, filename, extension, size, sha))
    for table, rows_ in bulk.items():
        if rows_:
            cols = [c for c in columns[table] if any(c in r for r in rows_)]
            statements.append(copy_script(table, cols, rows_))
    db.script("\n".join(statements))
    return key, stats


# ---- the SQL answers --------------------------------------------------------------------------------------------------------------------------
def scope(key):
    return "tenant_id = %s AND collection_id = %s" % (sql_text(TENANT), sql_text(key["case_id"]))


def sql_a1(key, msisdn):
    return ("SELECT coalesce(b_norm, b_raw) AS number, b_kind AS kind, "
            "count(*) FILTER (WHERE direction = 'outgoing') AS outgoing, count(*) FILTER (WHERE direction = 'incoming') AS incoming, "
            "count(*) FILTER (WHERE direction = 'unknown') AS unknown, coalesce(sum(duration_s), 0)::int AS voice_seconds, "
            "to_char(min(start_utc) AT TIME ZONE 'Asia/Karachi', %s) AS first, to_char(max(start_utc) AT TIME ZONE 'Asia/Karachi', %s) AS last "
            "FROM forensic.comm_event WHERE %s AND dataset_kind = 'target_cdr' AND a_norm = %s AND %s "
            "AND (b_norm IS NOT NULL OR b_kind IN ('short_code', 'sender_id', 'service_label')) GROUP BY 1, 2"
            % (LOCAL, LOCAL, scope(key), sql_text(msisdn), NOT_DUPLICATE))


def sql_a2(key, x, y, threshold):
    subjects = ", ".join(sql_text(s["msisdn"]) for s in key["subjects"])
    return ("WITH contacts AS (SELECT a_norm AS subject, b_norm AS number, count(*) AS n, min(start_utc) AS f, max(start_utc) AS l "
            "FROM forensic.comm_event WHERE %s AND dataset_kind = 'target_cdr' AND %s AND b_kind IN ('person_like', 'international') AND b_norm IS NOT NULL "
            "AND a_norm IN (%s) GROUP BY 1, 2), "
            "hubs AS (SELECT number FROM contacts GROUP BY number HAVING count(DISTINCT subject) > %d) "
            "SELECT x.number, x.n AS x_count, y.n AS y_count, to_char(x.f AT TIME ZONE 'Asia/Karachi', %s) AS x_first, to_char(x.l AT TIME ZONE 'Asia/Karachi', %s) AS x_last, "
            "to_char(y.f AT TIME ZONE 'Asia/Karachi', %s) AS y_first, to_char(y.l AT TIME ZONE 'Asia/Karachi', %s) AS y_last "
            "FROM contacts x JOIN contacts y ON y.number = x.number WHERE x.subject = %s AND y.subject = %s AND x.number NOT IN (%s, %s) "
            "AND x.number NOT IN (SELECT number FROM hubs) ORDER BY 1"
            % (scope(key), NOT_DUPLICATE, subjects, threshold, LOCAL, LOCAL, LOCAL, LOCAL, sql_text(x), sql_text(y), sql_text(x), sql_text(y)))


def sql_a5_pairs(key, x, y, minutes):
    return ("SELECT least(a.start_utc, b.start_utc) AS early, greatest(a.start_utc, b.start_utc) AS late, a.cell_key AS cell_a, b.cell_key AS cell_b "
            "FROM forensic.comm_event a JOIN forensic.comm_event b ON b.tenant_id = a.tenant_id AND b.collection_id = a.collection_id "
            "AND b.start_utc BETWEEN a.start_utc - interval '%d minutes' AND a.start_utc + interval '%d minutes' "
            "WHERE a.%s AND a.dataset_kind = 'target_cdr' AND b.dataset_kind = 'target_cdr' AND a.a_norm = %s AND b.a_norm = %s "
            "AND a.cell_key IS NOT NULL AND b.cell_key IS NOT NULL AND %s AND %s"
            % (minutes, minutes, scope(key).replace(" AND ", " AND a."), sql_text(x), sql_text(y), NOT_DUPLICATE.replace("quality_flags", "a.quality_flags"), NOT_DUPLICATE.replace("quality_flags", "b.quality_flags")))


def sql_a5_windows(key, x, y, minutes):
    """Co-located pairs merged into windows by gaps and islands: a pair joins the open window when it starts within W of the window's latest time."""
    base = sql_a5_pairs(key, x, y, minutes)
    return ("WITH p AS (SELECT cell_a AS cell_key, early, late FROM (%s) q WHERE cell_a = cell_b), "
            "o AS (SELECT *, max(late) OVER (PARTITION BY cell_key ORDER BY early, late ROWS BETWEEN UNBOUNDED PRECEDING AND 1 PRECEDING) AS prev_end FROM p), "
            "g AS (SELECT *, sum(CASE WHEN prev_end IS NULL OR early - prev_end > interval '%d minutes' THEN 1 ELSE 0 END) OVER (PARTITION BY cell_key ORDER BY early, late) AS grp FROM o) "
            "SELECT cell_key, to_char(min(early) AT TIME ZONE 'Asia/Karachi', %s) AS start_local, to_char(max(late) AT TIME ZONE 'Asia/Karachi', %s) AS end_local, "
            "count(*)::int AS pairs FROM g GROUP BY cell_key, grp ORDER BY 2, 1" % (base, minutes, LOCAL, LOCAL))


def sql_a5_expected(key, x, y, minutes):
    """The chance baseline of A5 in SQL: over every pair of events (one of each subject, cell known, no more than W minutes apart, same cell or not),
    the chance that both are at one cell if each chooses as they usually do at that time of week. A subject's habit is its share of events per
    cell in the (weekday or weekend, hour) of the earlier event of the pair, and its share over the whole case when it has no event in that hour."""
    return ("WITH ev AS (SELECT a_norm, cell_key, (start_utc AT TIME ZONE 'Asia/Karachi') AS lt FROM forensic.comm_event "
            "WHERE %s AND dataset_kind = 'target_cdr' AND a_norm IN (%s, %s) AND cell_key IS NOT NULL AND %s), "
            "ek AS (SELECT a_norm, cell_key, extract(isodow FROM lt) >= 6 AS wk, extract(hour FROM lt)::int AS hr FROM ev), "
            "bucket AS (SELECT a_norm, wk, hr, cell_key, count(*) AS n FROM ek GROUP BY 1, 2, 3, 4), "
            "overall AS (SELECT a_norm, cell_key, count(*) AS n FROM ek GROUP BY 1, 2), "
            "need AS (SELECT extract(isodow FROM early AT TIME ZONE 'Asia/Karachi') >= 6 AS wk, extract(hour FROM early AT TIME ZONE 'Asia/Karachi')::int AS hr, "
            "count(*) AS n FROM (%s) q GROUP BY 1, 2), "
            "dist AS (SELECT need.wk, need.hr, s.a_norm, s.cell_key, s.n::float8 / sum(s.n) OVER (PARTITION BY need.wk, need.hr, s.a_norm) AS p "
            "FROM need CROSS JOIN LATERAL (SELECT b.a_norm, b.cell_key, b.n FROM bucket b WHERE b.wk = need.wk AND b.hr = need.hr "
            "UNION ALL SELECT o.a_norm, o.cell_key, o.n FROM overall o WHERE NOT EXISTS "
            "(SELECT 1 FROM bucket b2 WHERE b2.a_norm = o.a_norm AND b2.wk = need.wk AND b2.hr = need.hr)) s), "
            "same AS (SELECT dx.wk, dx.hr, sum(dx.p * dy.p) AS prob FROM dist dx JOIN dist dy ON dy.wk = dx.wk AND dy.hr = dx.hr AND dy.cell_key = dx.cell_key "
            "WHERE dx.a_norm = %s AND dy.a_norm = %s GROUP BY 1, 2) "
            "SELECT coalesce(sum(need.n * same.prob), 0)::float8 AS expected FROM need LEFT JOIN same USING (wk, hr)"
            % (scope(key), sql_text(x), sql_text(y), NOT_DUPLICATE, sql_a5_pairs(key, x, y, minutes), sql_text(x), sql_text(y)))


def sql_a7(key, x, y, minutes, gap_minutes):
    """Possible meetings in SQL: the co-location windows of the pair, kept when a direct call or SMS between the two (either direction) fell within
    `gap_minutes` before the window starts (at the start counts, after does not), with the gap to the latest such contact and the documented score."""
    before = "c.ct BETWEEN w.start_local::timestamp - interval '%d minutes' AND w.start_local::timestamp" % gap_minutes
    return ("WITH w AS (%s), "
            "c AS (SELECT DISTINCT (start_utc AT TIME ZONE 'Asia/Karachi') AS ct FROM forensic.comm_event WHERE %s AND dataset_kind = 'target_cdr' AND %s "
            "AND ((a_norm = %s AND b_norm = %s) OR (a_norm = %s AND b_norm = %s))), "
            "j AS (SELECT w.cell_key, w.start_local, w.end_local, w.pairs, "
            "(SELECT json_agg(to_char(c.ct, %s) ORDER BY c.ct) FROM c WHERE %s) AS contacts_before, "
            "extract(epoch FROM w.start_local::timestamp - (SELECT max(c.ct) FROM c WHERE %s)) / 60.0 AS gap_minutes FROM w) "
            "SELECT cell_key, start_local, end_local, pairs, contacts_before, gap_minutes::float8 AS gap_minutes, "
            "(0.5 * least(pairs, 3) / 3.0 + 0.5 * (1 - gap_minutes / %d))::float8 AS score FROM j WHERE gap_minutes IS NOT NULL ORDER BY start_local, cell_key"
            % (sql_a5_windows(key, x, y, minutes), scope(key), NOT_DUPLICATE, sql_text(x), sql_text(y), sql_text(y), sql_text(x), LOCAL, before, before, gap_minutes))


def sql_a6(key, evidence_id, wanted):
    cells = ", ".join(sql_text(c) for c in wanted) if wanted else "NULL"
    return ("WITH seen AS (SELECT a_norm, cell_key FROM forensic.comm_event WHERE %s AND evidence_id = %s AND %s) "
            "SELECT r1.a_norm AS number, count(*)::int AS events, "
            "(SELECT coalesce(jsonb_object_agg(c.cell_key, c.n), '{}'::jsonb) FROM (SELECT r2.cell_key, count(*)::int AS n FROM seen r2 "
            "WHERE r2.a_norm = r1.a_norm AND r2.cell_key IN (%s) GROUP BY r2.cell_key) c) AS cells "
            "FROM seen r1 GROUP BY r1.a_norm ORDER BY 1" % (scope(key), sql_text(evidence_id), NOT_DUPLICATE, cells))


def sql_a13(key, probe, public_ip):
    return ("SELECT coalesce(json_agg(DISTINCT msisdn ORDER BY msisdn), '[]'::json) AS holders FROM forensic.data_session WHERE %s AND public_ip = %s::inet "
            "AND %d BETWEEN public_port_from AND public_port_to AND (timestamp %s AT TIME ZONE 'Asia/Karachi') "
            "BETWEEN coalesce(nat_start_utc, start_utc) AND coalesce(nat_end_utc, end_utc, start_utc)" % (scope(key), sql_text(public_ip), probe["port"], sql_text(probe["instant"])))


# ---- the checks --------------------------------------------------------------------------------------------------------------------------------
class Report:
    def __init__(self):
        self.items = []

    def add(self, name, ok, detail=""):
        self.items.append((name, bool(ok), detail))

    @property
    def failed(self):
        return [i for i in self.items if not i[1]]


def check_reconciliation(db, key, stats, report):
    for name, s in sorted(stats.items()):
        in_db = int(db.sql("SELECT count(*) FROM forensic.%s WHERE %s AND evidence_id = %s" % (s["table"], scope(key), sql_text(s["evidence_id"]))))
        flagged = int(db.sql("SELECT count(*) FROM forensic.%s WHERE %s AND evidence_id = %s AND NOT %s" % (s["table"], scope(key), sql_text(s["evidence_id"]), NOT_DUPLICATE)))
        report.add("reconcile %s" % name, s["read"] == in_db + s["quarantined"] + s["skipped"] and flagged == s["duplicates"],
                   "read %d, loaded %d, quarantined %d, skipped %d, repeats flagged %d (in the database %d)" % (s["read"], s["loaded"], s["quarantined"], s["skipped"], s["duplicates"], flagged))
    wanted_q, wanted_m = {}, {}
    for item in key["expected_quarantine"]:
        wanted_q[item["export"]] = wanted_q.get(item["export"], 0) + 1
    for item in key["expected_merge"]:
        wanted_m[item["export"]] = wanted_m.get(item["export"], 0) + 1
    # the key's expected_quarantine and expected_merge describe the call, session and usage exports; the families added since state their planted counts in their own truths (compared below)
    older = {n: s for n, s in stats.items() if s["family"] in ("call", "session", "usage")}
    got_q = {n: s["quarantined"] for n, s in older.items() if s["quarantined"]}
    got_m = {n: s["duplicates"] for n, s in older.items() if s["duplicates"]}
    report.add("quarantined rows equal the key's", wanted_q == got_q, "%s vs %s" % (wanted_q, got_q) if wanted_q != got_q else "%d rows" % sum(got_q.values()))
    report.add("repeated rows equal the key's", wanted_m == got_m, "%s vs %s" % (wanted_m, got_m) if wanted_m != got_m else "%d rows" % sum(got_m.values()))
    planted = {}
    for name, truth in (("anpr", (key.get("anpr") or {})), ("subscribers-A", key.get("subscriber_truth") or {}), ("bank-statement-A", key.get("transaction_truth") or {})):
        if truth and "planted" in truth and name in stats:
            planted[name] = (sum(truth["planted"].get("quarantined", {}).values()), truth["planted"].get("duplicate_rows", 0))
    newer = {n: s for n, s in stats.items() if s["family"] in ("plate", "subscriber", "transaction")}
    wanted_typed = {n: planted.get(n, (0, 0)) for n in newer}
    got_typed = {n: (s["quarantined"], s["duplicates"]) for n, s in newer.items()}
    if newer:
        report.add("quarantined and repeated rows of the newer families equal their planted counts", wanted_typed == got_typed,
                   "differs: %s" % {n: "planted %s, loaded %s" % (wanted_typed[n], got_typed[n]) for n in got_typed if wanted_typed[n] != got_typed[n]} if wanted_typed != got_typed
                   else "%d quarantined, %d repeated" % (sum(q for q, _ in got_typed.values()), sum(m for _, m in got_typed.values())))


def check_a1(db, key, report):
    for subject in key["subjects"]:
        got = {r["number"]: (r["kind"], r["outgoing"], r["incoming"], r["unknown"], r["voice_seconds"], r["first"], r["last"]) for r in db.rows(sql_a1(key, subject["msisdn"]))}
        want = {r["number"]: (r["kind"], r["outgoing"], r["incoming"], r["unknown"], r["voice_seconds"], r["first"], r["last"]) for r in key["oracle"]["A1"][subject["msisdn"]]}
        report.add("A1 contact profile of %s" % subject["person"], got == want, "%d contacts" % len(want) if got == want else "%d in SQL, %d in the key" % (len(got), len(want)))


def check_a2(db, key, report):
    threshold = key["oracle"]["parameters"]["hub_threshold"]
    for answer in key["oracle"]["A2"]:
        got = [(r["number"], r["x_count"], r["y_count"], r["x_first"], r["x_last"], r["y_first"], r["y_last"]) for r in db.rows(sql_a2(key, answer["x"], answer["y"], threshold))]
        want = [(s["number"], s["x_count"], s["y_count"], s["x_first"], s["x_last"], s["y_first"], s["y_last"]) for s in answer["shared"]]
        report.add("A2 common contacts %s / %s%s" % (answer["x"][-4:], answer["y"][-4:], " (" + answer["plant"] + ")" if answer.get("plant") else ""), got == want,
                   "%d shared" % len(want) if got == want else "SQL %s, key %s" % (got, want))


def check_a5(db, key, report):
    minutes = key["oracle"]["parameters"]["window_minutes"]
    for answer in key["oracle"]["A5"]:
        pairs = db.rows(sql_a5_pairs(key, answer["x"], answer["y"], minutes))
        windows = [(w["cell_key"], w["start_local"], w["end_local"], w["pairs"]) for w in db.rows(sql_a5_windows(key, answer["x"], answer["y"], minutes))]
        want = [(w["cell_key"], w["start_local"], w["end_local"], w["pairs"]) for w in answer["windows"]]
        observed = sum(1 for p in pairs if p["cell_a"] == p["cell_b"])
        ok = windows == want and observed == answer["pairs_observed"] and len(pairs) == answer["pairs_within_window"]
        report.add("A5 co-location %s / %s%s" % (answer["x"][-4:], answer["y"][-4:], " (" + answer["plant"] + ")" if answer.get("plant") else ""), ok,
                   "%d pairs in %d windows" % (observed, len(want)) if ok else "SQL %d pairs/%d within/%s, key %d/%d/%s" % (observed, len(pairs), windows, answer["pairs_observed"], answer["pairs_within_window"], want))


def check_a5_baseline(db, key, report):
    minutes = key["oracle"]["parameters"]["window_minutes"]
    for answer in key["oracle"]["A5"]:
        expected = float(db.sql(sql_a5_expected(key, answer["x"], answer["y"], minutes)))
        lift = round(answer["pairs_observed"] / expected, 4) if expected >= 0.5 else None
        ok = abs(expected - answer["pairs_expected"]) <= 1e-4 and ((lift is None) == (answer["lift"]["value"] is None)) \
            and (lift is None or abs(lift - answer["lift"]["value"]) <= 2e-4)
        report.add("A5 chance baseline %s / %s%s" % (answer["x"][-4:], answer["y"][-4:], " (" + answer["plant"] + ")" if answer.get("plant") else ""), ok,
                   ("expected %.2f pairs, lift %s" % (expected, "undefined" if lift is None else lift)) if ok
                   else "SQL expected %.4f lift %s, key expected %s lift %s" % (expected, lift, answer["pairs_expected"], answer["lift"]["value"]))


def check_a7(db, key, report):
    minutes = key["oracle"]["parameters"]["window_minutes"]
    gap = key["oracle"]["parameters"]["contact_gap_minutes"]
    for answer in key["oracle"]["A7"]:
        got = db.rows(sql_a7(key, answer["x"], answer["y"], minutes, gap))
        want = answer["candidates"]
        same = len(got) == len(want) and all(
            (g["cell_key"], g["start_local"], g["end_local"], g["pairs"], g["contacts_before"]) == (c["cell_key"], c["start_local"], c["end_local"], c["pairs"], c["contacts_before"])
            and abs(g["gap_minutes"] - c["gap_minutes"]) <= 1e-2 and abs(g["score"] - c["score"]) <= 1e-4 for g, c in zip(got, want))
        report.add("A7 possible meetings %s / %s%s" % (answer["x"][-4:], answer["y"][-4:], " (" + answer["plant"] + ")" if answer.get("plant") else ""), same,
                   "%d candidates" % len(want) if same else "SQL %s, key %s" % ([(g["start_local"], g["pairs"], g["gap_minutes"]) for g in got], [(c["start_local"], c["pairs"], c["gap_minutes"]) for c in want]))


def check_a6(db, key, stats, report):
    for answer in key["oracle"]["A6"]:
        dump = next(d for d in key["dumps"] if d["name"] == answer["dump"])
        wanted = [c for c in dump["cell_keys"] if c]
        got = {r["number"]: (r["events"], r["cells"]) for r in db.rows(sql_a6(key, stats[answer["dump"]]["evidence_id"], wanted))}
        want = {p["number"]: (p["events"], p["cells"]) for p in answer["population"]}
        ok = got == want   # events counts every row of the dump; the per-cell counts only the requested cells
        report.add("A6 population of %s" % answer["dump"], ok, "%d numbers" % len(want) if ok else "SQL %d numbers, key %d" % (len(got), len(want)))


def check_a13(db, key, report):
    plants = {p["id"]: p for p in key["plants"] if p["type"] == "address_translation"}
    for answer in key["oracle"]["A13"]:
        holders = db.rows(sql_a13(key, answer, answer["public_ip"]))[0]["holders"]
        verdict = "none" if not holders else "unique" if len(holders) == 1 else "ambiguous"
        ok = verdict == answer["result"] and holders == answer["holders"] and verdict == answer["intent"]
        report.add("A13 %s %s" % (answer["plant"], answer["label"]), ok, verdict if ok else "SQL %s %s, key %s %s" % (verdict, holders, answer["result"], answer["holders"]))
    return plants


def run_checks(db, key, stats):
    report = Report()
    check_reconciliation(db, key, stats, report)
    check_a1(db, key, report)
    check_a2(db, key, report)
    check_a5(db, key, report)
    check_a5_baseline(db, key, report)
    check_a7(db, key, report)
    check_a6(db, key, stats, report)
    check_a13(db, key, report)
    return report


def run(case_dir, keep=False, image=IMAGE):
    registry = load_registry()
    db = Throwaway(image)
    leftover = None
    try:
        started = time.time()
        db.start()
        prepare(db, registry)
        key, stats = load_case(db, case_dir, registry)
        loaded = {t: int(db.sql("SELECT count(*) FROM forensic.%s" % t)) for t in TABLE_OF.values()}
        report = run_checks(db, key, stats)
        report.seconds = round(time.time() - started, 1)
        report.loaded = loaded
        return report
    finally:
        if keep:
            print("kept container %s (password in this process only); remove it with: docker rm -f -v %s" % (db.name, db.name))
        else:
            leftover = db.stop()
            if leftover:
                print("WARNING: " + leftover)


def main(argv=None):
    ap = argparse.ArgumentParser(description="Load a generated case into a throwaway PostgreSQL and compare SQL answers with the key.")
    ap.add_argument("case_dir")
    ap.add_argument("--keep", action="store_true", help="leave the container running (for inspection)")
    ap.add_argument("--image", default=IMAGE)
    args = ap.parse_args(argv)
    report = run(args.case_dir, args.keep, args.image)
    for name, ok, detail in report.items:
        print("%s  %-46s %s" % ("ok  " if ok else "FAIL", name, detail[:160]))
    print("loaded rows: %s in %.1f s" % (report.loaded, report.seconds))
    print("CASELOAD=%s checks=%d failed=%d" % ("FAIL" if report.failed else "PASS", len(report.items), len(report.failed)))
    return 1 if report.failed else 0


if __name__ == "__main__":
    sys.exit(main())
