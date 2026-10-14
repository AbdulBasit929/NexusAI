#!/usr/bin/env python3
"""Builds the inventory of the private real-data collection (files, numbers, towers, devices, internet sessions) from the typed tables of the LIVE database, read-only.

    python -I -X utf8 tools/real_data/build_inventory_real.py [--collection nexusai-real-private-demo] [--out C:\\NexusAI-RealDataVault\\exports]

Output goes to the vault, never into a repository (the location guard refuses a git work tree):
  <out>\\inventory.json   what the workspace's Data tab shows (served by the workspace dev server from outside the repository)
  <out>\\*.csv            the same tables and the complete typed records as UTF-8 CSV files for extraction (open them in a spreadsheet)

Authority: the vault register, extension of 2026-10-14 (local demonstration to the team lead who supplied the files, on this machine). This script prints counts only.
"""
import argparse
import collections
import csv
import io
import json
import os
import subprocess
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.abspath(os.path.join(HERE, "..", ".."))
sys.path.insert(0, REPO)
sys.path.insert(0, HERE)
import guard  # noqa: E402
from ingestion.forensic_records.layout_registry import load_registry  # noqa: E402

DEFAULT_OUT = os.path.join(os.environ.get("NEXUSAI_REAL_DATA_VAULT", r"C:\NexusAI-RealDataVault"), "exports")
PSQL = ["docker", "exec", "-i", "nexusai-forensic-postgres-1", "psql", "-v", "ON_ERROR_STOP=1", "-U", "localrecall", "-d", "localrecall", "-q"]
NAME_KEY = r"(location|site|address|area|city|cell.?name|place)"
LOCAL = "AT TIME ZONE 'Asia/Karachi'"
TS = "'YYYY-MM-DD HH24:MI:SS'"


def psql(sql):
    proc = subprocess.run(PSQL + ["-At", "-c", sql], capture_output=True)
    if proc.returncode != 0:
        raise RuntimeError("psql failed: " + proc.stderr.decode("utf-8", "replace")[-400:])
    return proc.stdout.decode("utf-8")


def rows(sql):
    out = psql("SELECT coalesce(json_agg(t), '[]'::json) FROM (%s) t" % sql.strip().rstrip(";")).strip()
    return json.loads(out)


def copy_csv(sql, path):
    proc = subprocess.run(PSQL + ["-c", "COPY (%s) TO STDOUT WITH (FORMAT csv, HEADER true)" % sql.strip().rstrip(";")], capture_output=True)
    if proc.returncode != 0:
        raise RuntimeError("psql failed: " + proc.stderr.decode("utf-8", "replace")[-400:])
    with open(path, "wb") as handle:
        handle.write(b"\xef\xbb\xbf" + proc.stdout)   # a BOM so that a spreadsheet reads Urdu and Latin text correctly
    return max(0, proc.stdout.count(b"\n") - 1)


def main(argv):
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--collection", default="nexusai-real-private-demo")
    parser.add_argument("--out", default=DEFAULT_OUT)
    args = parser.parse_args(argv)
    guard.require_safe_location(args.out if os.path.isdir(args.out) else os.path.dirname(args.out), role="output")
    os.makedirs(args.out, exist_ok=True)
    c = "tenant_id = 'default' AND collection_id = '%s'" % args.collection.replace("'", "")
    registry = load_registry()
    label_of = {e["layout_id"]: (e.get("label") or "") for e in registry.entries}
    family_of = {e["layout_id"]: e.get("family") for e in registry.entries}

    # ---- files -----------------------------------------------------------------------------------------------------------------------------------
    files = []
    evidence = {r["evidence_id"]: r for r in rows("SELECT evidence_id, original_filename, extension, size_bytes, left(sha256, 12) AS sha12, created_at FROM forensic.evidence_items WHERE %s" % c)}
    stats = rows(
        "SELECT evidence_id, 'comm_event' AS tbl, layout_id, min(dataset_kind) AS dataset_kind, count(*) AS n, count(DISTINCT a_norm) AS subscribers, "
        "to_char(min(start_utc) %(l)s, %(t)s) AS first, to_char(max(start_utc) %(l)s, %(t)s) AS last, count(*) FILTER (WHERE 'duplicate_row' = ANY (quality_flags)) AS repeats "
        "FROM forensic.comm_event WHERE %(c)s GROUP BY evidence_id, layout_id "
        "UNION ALL SELECT evidence_id, 'data_session', layout_id, 'sessions', count(*), count(DISTINCT msisdn), to_char(min(start_utc) %(l)s, %(t)s), to_char(max(start_utc) %(l)s, %(t)s), 0 "
        "FROM forensic.data_session WHERE %(c)s GROUP BY evidence_id, layout_id "
        "UNION ALL SELECT evidence_id, 'app_usage', layout_id, 'app usage', count(*), count(DISTINCT msisdn), to_char(min(start_utc) %(l)s, %(t)s), to_char(max(start_utc) %(l)s, %(t)s), 0 "
        "FROM forensic.app_usage WHERE %(c)s GROUP BY evidence_id, layout_id" % {"c": c, "l": LOCAL, "t": TS})
    keys = {r["evidence_id"]: r["keys"] for r in rows(
        "SELECT evidence_id, (SELECT json_agg(k ORDER BY k) FROM (SELECT DISTINCT jsonb_object_keys(raw_payload) AS k FROM forensic.%s s2 WHERE s2.evidence_id = s.evidence_id LIMIT 80) q) AS keys "
        "FROM (SELECT DISTINCT evidence_id FROM forensic.comm_event WHERE %s) s" % ("comm_event", c))}
    for tbl in ("data_session", "app_usage"):
        keys.update({r["evidence_id"]: r["keys"] for r in rows(
            "SELECT evidence_id, (SELECT json_agg(k ORDER BY k) FROM (SELECT DISTINCT jsonb_object_keys(raw_payload) AS k FROM forensic.%s s2 WHERE s2.evidence_id = s.evidence_id LIMIT 80) q) AS keys "
            "FROM (SELECT DISTINCT evidence_id FROM forensic.%s WHERE %s) s" % (tbl, tbl, c))})
    for index, s in enumerate(sorted(stats, key=lambda s: (family_of.get(s["layout_id"]) or "", s["layout_id"], s["first"] or "")), 1):
        e = evidence.get(s["evidence_id"], {})
        files.append({"id": "F%02d" % index, "name": e.get("original_filename"), "extension": e.get("extension"), "size_bytes": e.get("size_bytes"), "sha256_prefix": e.get("sha12"),
                      "layout": s["layout_id"], "layout_label": label_of.get(s["layout_id"], ""), "family": family_of.get(s["layout_id"]), "kind": s["dataset_kind"],
                      "rows": s["n"], "repeats": s["repeats"], "subscribers": s["subscribers"], "first": s["first"], "last": s["last"], "columns": keys.get(s["evidence_id"]) or []})

    # ---- subscribers (the number each call-record file belongs to), their counterparties, devices and cells ---------------------------------------------
    base = "FROM forensic.comm_event WHERE %s AND dataset_kind = 'target_cdr' AND NOT ('duplicate_row' = ANY (quality_flags)) AND a_norm IS NOT NULL" % c
    subscribers = rows(
        "SELECT a_norm AS number, min(layout_id) AS layout, count(*) AS events, count(*) FILTER (WHERE event_type = 'voice') AS calls, count(*) FILTER (WHERE event_type = 'sms') AS sms, "
        "count(*) FILTER (WHERE direction = 'outgoing') AS outgoing, count(*) FILTER (WHERE direction = 'incoming') AS incoming, count(DISTINCT coalesce(b_norm, b_raw)) AS counterparties, "
        "coalesce(sum(duration_s), 0) AS total_duration_s, count(DISTINCT cell_key) AS cells, to_char(min(start_utc) %s, %s) AS first, to_char(max(start_utc) %s, %s) AS last "
        "%s GROUP BY a_norm ORDER BY count(*) DESC" % (LOCAL, TS, LOCAL, TS, base))
    for sub in subscribers:
        sub["label"] = label_of.get(sub["layout"], "")
        sub["top_counterparties"] = rows(
            "SELECT coalesce(b_norm, b_raw) AS number, min(b_kind) AS kind, count(*) AS events, count(*) FILTER (WHERE direction = 'outgoing') AS outgoing, count(*) FILTER (WHERE direction = 'incoming') AS incoming, "
            "coalesce(sum(duration_s), 0) AS duration_s, to_char(min(start_utc) %s, %s) AS first, to_char(max(start_utc) %s, %s) AS last %s AND a_norm = '%s' AND coalesce(b_norm, b_raw) IS NOT NULL "
            "GROUP BY 1 ORDER BY count(*) DESC LIMIT 25" % (LOCAL, TS, LOCAL, TS, base, sub["number"]))
        sub["devices"] = rows(
            "SELECT imsi, imei, count(*) AS events, to_char(min(start_utc) %s, %s) AS first, to_char(max(start_utc) %s, %s) AS last %s AND a_norm = '%s' AND (imsi IS NOT NULL OR imei IS NOT NULL) "
            "GROUP BY imsi, imei ORDER BY min(start_utc)" % (LOCAL, TS, LOCAL, TS, base, sub["number"]))

    # ---- towers: every cell seen in any file, with the name the files give it, its position, the traffic and the numbers -------------------------------
    names = collections.defaultdict(collections.Counter)
    for r in rows(
        "SELECT cell_key, value, count(*) AS n FROM forensic.comm_event, jsonb_each_text(raw_payload) AS kv(key, value) WHERE %s AND cell_key IS NOT NULL AND key ~* '%s' AND value <> '' "
        "GROUP BY cell_key, value" % (c, NAME_KEY)):
        names[r["cell_key"]][r["value"]] += r["n"]
    towers = rows(
        "SELECT cell_key, min(layout_id) AS layout, avg(lat) AS lat, avg(lon) AS lon, count(*) AS events, count(DISTINCT a_norm) AS numbers, "
        "count(*) FILTER (WHERE dataset_kind = 'target_cdr') AS call_events, count(*) FILTER (WHERE dataset_kind = 'tower_dump') AS dump_events, count(DISTINCT evidence_id) AS files, "
        "min(lac_tac) AS lac, to_char(min(start_utc) %s, %s) AS first, to_char(max(start_utc) %s, %s) AS last FROM forensic.comm_event WHERE %s AND cell_key IS NOT NULL "
        "GROUP BY cell_key ORDER BY count(*) DESC" % (LOCAL, TS, LOCAL, TS, c))
    for t in towers:
        top = names[t["cell_key"]].most_common(3)
        t["name"] = top[0][0] if top else None
        t["names_seen"] = [n for n, _ in top]

    # ---- internet sessions ------------------------------------------------------------------------------------------------------------------------------
    sessions = {
        "total": int(psql("SELECT count(*) FROM forensic.data_session WHERE %s" % c).strip() or 0),
        "subscribers": rows("SELECT msisdn AS number, min(layout_id) AS layout, count(*) AS sessions, count(DISTINCT public_ip) AS public_addresses, coalesce(sum(bytes_up), 0) AS bytes_up, coalesce(sum(bytes_down), 0) AS bytes_down, "
                            "to_char(min(start_utc) %s, %s) AS first, to_char(max(start_utc) %s, %s) AS last FROM forensic.data_session WHERE %s AND msisdn IS NOT NULL GROUP BY msisdn ORDER BY count(*) DESC LIMIT 200" % (LOCAL, TS, LOCAL, TS, c)),
        "top_public_addresses": rows("SELECT host(public_ip) AS address, count(*) AS sessions, count(DISTINCT msisdn) AS subscribers, min(public_port_from) AS port_from, max(public_port_to) AS port_to FROM forensic.data_session "
                                     "WHERE %s AND public_ip IS NOT NULL GROUP BY public_ip ORDER BY count(*) DESC LIMIT 100" % c),
        "top_applications": rows("SELECT coalesce(app, '(none)') AS application, count(*) AS sessions FROM forensic.data_session WHERE %s GROUP BY 1 ORDER BY 2 DESC LIMIT 40" % c),
        "top_hosts": rows("SELECT host AS host, count(*) AS sessions FROM forensic.data_session WHERE %s AND host IS NOT NULL GROUP BY host ORDER BY 2 DESC LIMIT 60" % c),
    }
    app_usage = rows("SELECT coalesce(app, '(none)') AS application, coalesce(app_category, '') AS category, count(*) AS records, count(DISTINCT msisdn) AS numbers FROM forensic.app_usage WHERE %s GROUP BY 1, 2 ORDER BY 3 DESC LIMIT 60" % c)

    inventory = {"collection": args.collection, "generated_utc": psql("SELECT to_char(now() AT TIME ZONE 'UTC', 'YYYY-MM-DD HH24:MI:SS')").strip(),
                 "note": "real call and internet records supplied for testing: shown to the team lead on this machine only", "files": files, "subscribers": subscribers, "towers": towers[:3000],
                 "towers_total": len(towers), "sessions": sessions, "app_usage": app_usage}
    with open(os.path.join(args.out, "inventory.json"), "w", encoding="utf-8", newline="\n") as handle:
        json.dump(inventory, handle, separators=(",", ":"))

    # ---- CSV extracts ---------------------------------------------------------------------------------------------------------------------------------------
    def write_csv(name, header, data):
        with open(os.path.join(args.out, name), "wb") as handle:
            buf = io.StringIO()
            w = csv.writer(buf, lineterminator="\r\n")
            w.writerow(header)
            w.writerows(data)
            handle.write(b"\xef\xbb\xbf" + buf.getvalue().encode("utf-8"))
        return len(data)

    counts = {}
    counts["files.csv"] = write_csv("files.csv", ["id", "file name", "layout", "layout description", "family", "kind", "rows", "repeats", "subscribers", "first (local)", "last (local)", "columns in the file"],
                                    [[f["id"], f["name"], f["layout"], f["layout_label"], f["family"], f["kind"], f["rows"], f["repeats"], f["subscribers"], f["first"], f["last"], "; ".join(f["columns"])] for f in files])
    counts["subscribers.csv"] = write_csv("subscribers.csv", ["number", "layout", "layout description", "events", "calls", "sms", "outgoing", "incoming", "counterparties", "total duration (s)", "cells", "first (local)", "last (local)"],
                                          [[s["number"], s["layout"], s["label"], s["events"], s["calls"], s["sms"], s["outgoing"], s["incoming"], s["counterparties"], s["total_duration_s"], s["cells"], s["first"], s["last"]] for s in subscribers])
    counts["towers.csv"] = write_csv("towers.csv", ["cell key", "name in the files", "other names", "latitude", "longitude", "layout", "LAC/TAC", "events", "call events", "tower dump events", "distinct numbers", "files", "first (local)", "last (local)"],
                                     [[t["cell_key"], t["name"], "; ".join(t["names_seen"][1:]), t["lat"], t["lon"], t["layout"], t["lac"], t["events"], t["call_events"], t["dump_events"], t["numbers"], t["files"], t["first"], t["last"]] for t in towers])
    counts["devices.csv"] = write_csv("devices.csv", ["number", "IMSI", "IMEI", "events", "first (local)", "last (local)"], [[s["number"], d["imsi"], d["imei"], d["events"], d["first"], d["last"]] for s in subscribers for d in s["devices"]])
    counts["call_records.csv"] = copy_csv(
        "SELECT a_norm AS subscriber, coalesce(b_norm, b_raw) AS other_party, b_kind AS other_party_class, event_type, direction, to_char(start_utc %s, %s) AS start_local, duration_s, imsi, imei, cell_key, lac_tac, lat, lon, "
        "layout_id, source_row + 1 AS source_row, file_id FROM forensic.comm_event WHERE %s AND dataset_kind = 'target_cdr' AND NOT ('duplicate_row' = ANY (quality_flags)) ORDER BY a_norm, start_utc" % (LOCAL, TS, c),
        os.path.join(args.out, "call_records.csv"))
    counts["tower_dump_rows.csv"] = copy_csv(
        "SELECT a_norm AS number, coalesce(b_norm, b_raw) AS other_party, event_type, to_char(start_utc %s, %s) AS time_local, imsi, imei, cell_key, lac_tac, lat, lon, layout_id, source_row + 1 AS source_row "
        "FROM forensic.comm_event WHERE %s AND dataset_kind = 'tower_dump' ORDER BY layout_id, start_utc" % (LOCAL, TS, c), os.path.join(args.out, "tower_dump_rows.csv"))
    counts["internet_sessions.csv"] = copy_csv(
        "SELECT msisdn AS subscriber, imsi, imei, to_char(start_utc %s, %s) AS start_local, to_char(end_utc %s, %s) AS end_local, host(ue_ip) AS private_address, host(public_ip) AS public_address, public_port_from, public_port_to, "
        "host(server_ip) AS server_address, server_port, app_category, app, host, bytes_up, bytes_down, cell_key, layout_id, source_row + 1 AS source_row FROM forensic.data_session WHERE %s ORDER BY msisdn, start_utc" % (LOCAL, TS, LOCAL, TS, c),
        os.path.join(args.out, "internet_sessions.csv"))
    counts["app_usage.csv"] = copy_csv(
        "SELECT msisdn AS subscriber, imei, app_category, app, app_feature, to_char(start_utc %s, %s) AS start_local, province, city, layout_id, source_row + 1 AS source_row FROM forensic.app_usage WHERE %s ORDER BY msisdn, start_utc" % (LOCAL, TS, c),
        os.path.join(args.out, "app_usage.csv"))
    counts["numbers_seen.csv"] = copy_csv(
        "SELECT number, bool_or(is_subscriber) AS is_subscriber, sum(events) AS events, string_agg(DISTINCT role, '; ') AS roles FROM ("
        "SELECT a_norm AS number, true AS is_subscriber, count(*) AS events, 'subscriber or dump population' AS role FROM forensic.comm_event WHERE %(c)s AND a_norm IS NOT NULL GROUP BY a_norm "
        "UNION ALL SELECT b_norm, false, count(*), 'other party' FROM forensic.comm_event WHERE %(c)s AND b_norm IS NOT NULL GROUP BY b_norm "
        "UNION ALL SELECT msisdn, true, count(*), 'internet subscriber' FROM forensic.data_session WHERE %(c)s AND msisdn IS NOT NULL GROUP BY msisdn) u GROUP BY number ORDER BY sum(events) DESC" % {"c": c},
        os.path.join(args.out, "numbers_seen.csv"))
    print("inventory: %d files, %d subscribers, %d towers (%d in the view), %d internet subscribers, %d applications" % (len(files), len(subscribers), len(towers), min(len(towers), 3000), len(sessions["subscribers"]), len(app_usage)))
    print("towers with a name in the files: %d" % sum(1 for t in towers if t["name"]))
    print("CSV extracts (rows): " + ", ".join("%s=%d" % kv for kv in counts.items()))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
