#!/usr/bin/env python3
"""Typing report for one collection: what the typed tables hold and where the engine could be misled. Read-only, counts and shares only, never a value, no model, a few seconds.

    python -X utf8 tools/dev/typing_report.py nexusai-multimodal-product-acceptance
    python -X utf8 tools/dev/typing_report.py nexusai-forensic-demo --tenant default

It reads the live Postgres through `docker exec` (container nexusai-forensic-postgres-1, overridable with --container). Run it after any layout, typing or back-fill change and BEFORE putting
questions to the engine: of the surprises found on 2026-10-20 when two cases were typed (an undirected volume the views could not add up, call-type labels read as "other", 2,403 exact
repeats dropped from the effective view, a time zone assumed for one file, an unasked direction filter) the first four show here at once, in seconds, where the question runs took 30 to 70 minutes.

A line starting with "!" is a finding to read before trusting an answer. Exit status 0 always (it reports; it does not judge).
"""
import argparse
import subprocess
import sys

FLAG_OTHER_SHARE = 0.10      # event types read as "other"
FLAG_DUP_SHARE = 0.05        # exact repeats left out of the effective view
FLAG_QUARANTINE = 0.01       # rows read but not typed
FLAG_UNKNOWN_DIRECTION = 0.20


def make_psql(container):
    def psql(sql):
        out = subprocess.run(["docker", "exec", container, "psql", "-U", "localrecall", "-d", "localrecall", "-At", "-F", "|", "-c", sql], capture_output=True, text=True)
        if out.returncode:
            raise SystemExit("psql failed: " + out.stderr.strip()[:200])
        return [line.split("|") for line in out.stdout.strip().splitlines() if line]
    return psql


def pct(part, whole):
    return "%.1f%%" % (100.0 * part / whole) if whole else "-"


def main(argv):
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("collection")
    parser.add_argument("--tenant", default="default")
    parser.add_argument("--container", default="nexusai-forensic-postgres-1")
    args = parser.parse_args(argv)
    psql = make_psql(args.container)
    c, t = args.collection.replace("'", ""), args.tenant.replace("'", "")
    where = "tenant_id = '%s' AND collection_id = '%s'" % (t, c)
    findings = []

    print("== %s (tenant %s)" % (c, t))
    jobs = psql("SELECT record_type::text, status, count(*), coalesce(sum(total_rows), 0) FROM forensic.records_ingest_jobs WHERE %s AND record_type::text IN ('cdr', 'ipdr') GROUP BY 1, 2 ORDER BY 1, 2" % where)
    print("version-1 jobs (call and internet records): " + ("; ".join("%s %s x%s (%s rows)" % tuple(j) for j in jobs) or "none"))

    cov = psql("SELECT count(*) FILTER (WHERE e.detected_type IN ('cdr', 'ipdr') AND coalesce(e.processing_status, '') <> 'failed'), "
               "count(*) FILTER (WHERE e.detected_type IN ('cdr', 'ipdr') AND coalesce(e.processing_status, '') <> 'failed' AND (EXISTS (SELECT 1 FROM forensic.comm_event c WHERE c.tenant_id = e.tenant_id AND c.evidence_id = e.evidence_id) "
               "OR EXISTS (SELECT 1 FROM forensic.data_session s WHERE s.tenant_id = e.tenant_id AND s.evidence_id = e.evidence_id))) "
               "FROM forensic.evidence_items e WHERE e.%s AND e.modality = 'structured_records'" % where)
    files, typed = int(cov[0][0]), int(cov[0][1])
    print("files the engine's guard counts: %d, with typed rows: %d" % (files, typed))
    if files and typed < files:
        findings.append("only %d of %d call and internet files are typed: the engine declines this case (coverage guard)" % (typed, files))

    comm = psql("SELECT layout_id || ' v' || layout_version, dataset_kind, time_basis, count(*), count(*) FILTER (WHERE 'duplicate_row' = ANY (quality_flags)) FROM forensic.comm_event WHERE %s GROUP BY 1, 2, 3 ORDER BY 1, 3" % where)
    total = sum(int(r[3]) for r in comm)
    dups = sum(int(r[4]) for r in comm)
    if comm:
        print("\ncall records (comm_event): %d typed, %d flagged as exact repeats (%s), %d in the effective view" % (total, dups, pct(dups, total), total - dups))
        for layout, kind, basis, n, d in comm:
            print("  %-34s %-10s time %-18s %6s rows  repeats %s" % (layout, kind, basis, n, pct(int(d), int(n))))
            if basis not in ("declared", "explicit_offset", "confirmed_by_analyst"):
                findings.append("%s: %s rows carry the time basis '%s' (a zone was assumed, not declared)" % (layout, n, basis))
        if total and dups / total > FLAG_DUP_SHARE:
            findings.append("%s of the call rows are exact repeats and are left out of the effective view: counts differ from version 1 and from any key built on it" % pct(dups, total))
        kinds = psql("SELECT event_type, count(*) FROM forensic.comm_event WHERE %s AND NOT ('duplicate_row' = ANY (quality_flags)) GROUP BY 1 ORDER BY 2 DESC" % where)
        eff = sum(int(r[1]) for r in kinds)
        print("  event types: " + ", ".join("%s %s (%s)" % (k, n, pct(int(n), eff)) for k, n in kinds))
        other = sum(int(n) for k, n in kinds if k == "other")
        if eff and other / eff > FLAG_OTHER_SHARE:
            raw = psql("SELECT coalesce(upper(raw_payload->>'CALL_TYPE'), upper(raw_payload->>'Call_Type'), upper(raw_payload->>'call_type'), '(no label column)'), count(*) FROM forensic.comm_event WHERE %s AND event_type = 'other' AND NOT ('duplicate_row' = ANY (quality_flags)) GROUP BY 1 ORDER BY 2 DESC LIMIT 6" % where)
            findings.append("%s of the call rows are event type 'other': a question about voice or sms leaves them out. Labels behind it: %s" % (pct(other, eff), ", ".join("%s %s" % (a, b) for a, b in raw)))
        dirs = psql("SELECT direction, count(*) FROM forensic.comm_event WHERE %s AND NOT ('duplicate_row' = ANY (quality_flags)) GROUP BY 1 ORDER BY 2 DESC" % where)
        print("  directions: " + ", ".join("%s %s (%s)" % (k, n, pct(int(n), eff)) for k, n in dirs))
        unknown = sum(int(n) for k, n in dirs if k in ("unknown", "none"))
        if eff and unknown / eff > FLAG_UNKNOWN_DIRECTION:
            findings.append("%s of the call rows have no known direction: incoming and outgoing counts are partial" % pct(unknown, eff))
        pop = psql("SELECT count(*) FILTER (WHERE imei IS NOT NULL), count(*) FILTER (WHERE imsi IS NOT NULL), count(*) FILTER (WHERE cell_key IS NOT NULL), count(*) FILTER (WHERE lat IS NOT NULL) FROM forensic.comm_event WHERE %s AND NOT ('duplicate_row' = ANY (quality_flags))" % where)[0]
        print("  populated: imei %s, imsi %s, cell %s, position %s" % tuple(pct(int(x), eff) for x in pop))
        q = psql("SELECT count(*) FROM forensic.records_audit_log WHERE %s AND action = 'records.ingest.typed_shadow' AND (details->'typed_ingest'->>'rows_quarantined')::int > 0" % where)
        print("  typed-ingest audit rows that report quarantined rows: %s" % q[0][0])

    sess = psql("SELECT layout_id || ' v' || layout_version, time_basis, count(*), count(*) FILTER (WHERE bytes_up IS NOT NULL OR bytes_down IS NOT NULL), count(*) FILTER (WHERE public_ip IS NOT NULL), "
                "count(*) FILTER (WHERE server_port IS NOT NULL), count(*) FILTER (WHERE 'duplicate_row' = ANY (quality_flags)) FROM forensic.data_session WHERE %s GROUP BY 1, 2 ORDER BY 1" % where)
    if sess:
        print("\ninternet records (data_session):")
        for layout, basis, n, vol, pub, port, dup in sess:
            print("  %-34s time %-18s %6s rows  volume stated %s  public address %s  server port %s  repeats %s" % (layout, basis, n, pct(int(vol), int(n)), pct(int(pub), int(n)), pct(int(port), int(n)), pct(int(dup), int(n))))
            if int(n) and int(vol) == 0:
                findings.append("%s: no row states a byte count (the export has one undirected volume or none): volume questions are declined to the older path" % layout)
            if int(n) and int(pub) == 0:
                findings.append("%s: no public address: 'who held this address' cannot be answered from this case" % layout)
            if basis not in ("declared", "explicit_offset", "confirmed_by_analyst"):
                findings.append("%s: %s rows carry the time basis '%s'" % (layout, n, basis))

    print("")
    if findings:
        for f in findings:
            print("! " + f)
    else:
        print("no finding")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
