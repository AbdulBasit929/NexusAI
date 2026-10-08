#!/usr/bin/env python3
"""Question shapes the factory has no template for, with answer keys computed from SQL over the raw records.

    python shape_probes.py --psql "docker exec -i nexusai-forensic-postgres-1 psql -U localrecall -d localrecall -At -F '|'" [--set 2]

The factory's questions are templates filled with values, so a new seed gives new values and the same shapes. These are shapes it never
asks: a top 3, a percentage, a window of the day that crosses midnight, weekends, a threshold in minutes or in words, a comparison of two
months, a median, an aggregate of aggregates. They are exploratory: each key is a plain SQL reading of the question on the case clock, and
a disagreement is read by hand before it is called a defect (a key can be wrong; the question can be ambiguous).

The file has the factory's format, so `factory.py run` and `Run-Arm` ask and judge it like any other set. Kinds: number, date, and `all`
(the answer must contain every value the key query returns). Nothing here writes to the database.
"""
import argparse
import datetime
import io
import json
import sys

import factory as f

DEMO = "nexusai-forensic-demo"


def probes_v1(db):
    """(id, family, intent, question, key SQL, kind) for each probe of the first set. Typed expressions come from the layer, as the factory's own keys do."""
    L = f.load_layer()
    fld = {x["id"]: x for fam in L.values() for x in fam["fields"]}
    raw, typed, S = f.raw_expr, f.typed_expr, lambda fam: f.scope(db, fam)
    t0, t1 = typed(fld["cdr.call_start"]), typed(fld["cdr.call_end"])
    msisdn, ctype, direction = raw(fld["cdr.msisdn"]), raw(fld["cdr.call_type"]), raw(fld["cdr.direction"])
    cdr = "FROM forensic.records WHERE %s" % S("cdr")
    dur = "extract(epoch from (%s) - (%s))" % (t1, t0)
    amount, account, tstatus = typed(fld["transaction.amount"]), raw(fld["transaction.account"]), raw(fld["transaction.status"])
    tx = "FROM forensic.records WHERE %s" % S("transaction")
    ipdr = "FROM forensic.records WHERE %s" % S("ipdr")
    log = "FROM forensic.records WHERE %s" % S("access_log")
    anpr = "FROM forensic.records WHERE %s" % S("anpr")
    nums = "923001110001"
    return [
        ("SHAPE-topn-01", "cdr", "topn", "Which 3 subscriber numbers appear in the most call records?",
         "SELECT %s %s GROUP BY 1 ORDER BY count(*) DESC, 1 LIMIT 3" % (msisdn, cdr), "all"),
        ("SHAPE-avg_filtered-01", "cdr", "avg_filtered", "What is the average duration in seconds of VoLTE calls?",
         "SELECT avg(%s) %s AND %s = 'VOLTE'" % (dur, cdr, ctype), "number"),
        ("SHAPE-distinct_for-01", "cdr", "distinct_for", "How many different cell sites did %s use?" % nums,
         "SELECT count(DISTINCT %s) %s AND %s = '%s'" % (raw(fld["cdr.cell_site_id"]), cdr, msisdn, nums), "number"),
        ("SHAPE-percent-01", "cdr", "percent", "What percentage of the call records are SMS?",
         "SELECT 100.0 * count(*) FILTER (WHERE %s = 'SMS') / count(*) %s" % (ctype, cdr), "number"),
        ("SHAPE-window_midnight-01", "cdr", "window_midnight", "How many calls were made between 11 pm and 3 am?",
         "SELECT count(*) %s AND extract(hour from %s) IN (23, 0, 1, 2)" % (cdr, t0), "number"),
        ("SHAPE-weekend-01", "cdr", "weekend", "How many calls were made on weekends?",
         "SELECT count(*) %s AND extract(isodow from %s) IN (6, 7)" % (cdr, t0), "number"),
        ("SHAPE-first_for-01", "cdr", "first_for", "When was the first call record of %s?" % nums,
         "SELECT to_char(min(%s), 'YYYY-MM-DD') %s AND %s = '%s'" % (t0, cdr, msisdn, nums), "date"),
        ("SHAPE-compare_months-01", "cdr", "compare_months", "How many call records were there in May 2026 versus June 2026?",
         "SELECT c FROM (SELECT to_char(%s, 'YYYY-MM') m, count(*) c %s GROUP BY 1) t WHERE m IN ('2026-05', '2026-06') ORDER BY m" % (t0, cdr), "all"),
        ("SHAPE-having-01", "cdr", "having", "Which subscriber numbers have more than 850 call records?",
         "SELECT %s %s GROUP BY 1 HAVING count(*) > 850 ORDER BY count(*) DESC" % (msisdn, cdr), "all"),
        ("SHAPE-nested_max-01", "cdr", "nested_max", "How many call records started in the busiest hour of the day?",
         "SELECT max(c) FROM (SELECT count(*) c %s GROUP BY extract(hour from %s)) t" % (cdr, t0), "number"),
        ("SHAPE-two_values-01", "cdr", "two_values", "How many incoming SMS records are there?",
         "SELECT count(*) %s AND %s = 'SMS' AND %s = 'INCOMING'" % (cdr, ctype, direction), "number"),
        ("SHAPE-negation-01", "cdr", "negation", "How many call records are not SMS?",
         "SELECT count(*) %s AND %s <> 'SMS'" % (cdr, ctype), "number"),
        ("SHAPE-minutes-01", "cdr", "minutes", "How many calls lasted more than 15 minutes?",
         "SELECT count(*) %s AND %s > 900" % (cdr, dur), "number"),
        ("SHAPE-words-01", "cdr", "words", "How many calls lasted longer than ten minutes?",
         "SELECT count(*) %s AND %s > 600" % (cdr, dur), "number"),
        ("SHAPE-busiest_day_for-01", "cdr", "busiest_day_for", "On which day does %s have the most call records?" % nums,
         "SELECT to_char(%s, 'YYYY-MM-DD') %s AND %s = '%s' GROUP BY 1 ORDER BY count(*) DESC, 1 LIMIT 1" % (t0, cdr, msisdn, nums), "date"),
        ("SHAPE-by_month-01", "cdr", "by_month", "How many call records were there in each month?",
         "SELECT count(*) %s GROUP BY to_char(%s, 'YYYY-MM') ORDER BY to_char(%s, 'YYYY-MM')" % (cdr, t0, t0), "all"),
        ("SHAPE-count_in_month_for-01", "cdr", "count_in_month_for", "How many call records does %s have in April 2026?" % nums,
         "SELECT count(*) %s AND %s = '%s' AND to_char(%s, 'YYYY-MM') = '2026-04'" % (cdr, msisdn, nums, t0), "number"),
        ("SHAPE-top_sum-01", "ipdr", "top_sum", "Which domain transferred the most bytes in total?",
         "SELECT %s %s GROUP BY 1 ORDER BY sum(%s) DESC LIMIT 1" % (raw(fld["ipdr.domain"]), ipdr, typed(fld["ipdr.bytes"])), "all"),
        ("SHAPE-value_list-01", "ipdr", "value_list", "Which domains were accessed in the internet sessions?",
         "SELECT DISTINCT %s %s ORDER BY 1" % (raw(fld["ipdr.domain"]), ipdr), "all"),
        ("SHAPE-two_conditions-01", "access_log", "two_conditions", "How many POST requests returned a 403 status?",
         "SELECT count(*) %s AND %s = 'POST' AND %s = '403'" % (log, raw(fld["access_log.http_method"]), raw(fld["access_log.status"])), "number"),
        ("SHAPE-value_list-02", "access_log", "value_list", "Which HTTP methods appear in the access log?",
         "SELECT DISTINCT %s %s ORDER BY 1" % (raw(fld["access_log.http_method"]), log), "all"),
        ("SHAPE-sum_filtered-01", "transaction", "sum_filtered", "What is the total amount of approved transactions?",
         "SELECT sum(%s) %s AND %s = 'approved'" % (amount, tx, tstatus), "number"),
        ("SHAPE-median-01", "transaction", "median", "What is the median transaction amount?",
         "SELECT percentile_cont(0.5) WITHIN GROUP (ORDER BY %s) %s" % (amount, tx), "number"),
        ("SHAPE-max_account-01", "transaction", "max_account", "Which account made the largest transaction?",
         "SELECT %s %s ORDER BY %s DESC LIMIT 1" % (account, tx, amount), "all"),
        ("SHAPE-threshold_comma-01", "transaction", "threshold_comma", "How many transactions are above 10,000?",
         "SELECT count(*) %s AND %s > 10000" % (tx, amount), "number"),
        ("SHAPE-night_for-01", "anpr", "night_for", "How many sightings of plate LHR-2026 were at night?",
         "SELECT count(*) %s AND %s = 'LHR-2026' AND extract(hour from %s) < 6" % (anpr, raw(fld["anpr.plate_number"]), typed(fld["anpr.event_time"])), "number"),
        ("SHAPE-first_last-01", "anpr", "first_last", "What are the first and last times a vehicle was sighted?",
         "SELECT to_char(min(%s), 'YYYY-MM-DD HH24:MI:SS') %s UNION ALL SELECT to_char(max(%s), 'YYYY-MM-DD HH24:MI:SS') %s"
         % (typed(fld["anpr.event_time"]), anpr, typed(fld["anpr.event_time"]), anpr), "all"),
        ("SHAPE-year_only-01", "subscriber", "year_only", "How many subscribers were activated in 2025?",
         "SELECT count(*) FROM forensic.records WHERE %s AND extract(year from %s) = 2025" % (S("subscriber"), typed(fld["subscriber.activation_date"])), "number"),
    ]


def probes_v2(db):
    """The second set: the same kinds of trouble as the first, in different questions, values and words, so that a fix made for the first can be measured
    on questions it was not made for. SHAPE2-sum_bytes-01, -fewest-01 and -share-01 are controls: the right answer to each is a sum or a ranking that
    the checks for "highest", "average" and the rest must leave alone."""
    L = f.load_layer()
    fld = {x["id"]: x for fam in L.values() for x in fam["fields"]}
    raw, typed, S = f.raw_expr, f.typed_expr, lambda fam: f.scope(db, fam)
    t0, t1 = typed(fld["cdr.call_start"]), typed(fld["cdr.call_end"])
    dur = "extract(epoch from (%s) - (%s))" % (t1, t0)
    cdr = "FROM forensic.records WHERE %s" % S("cdr")
    log = "FROM forensic.records WHERE %s" % S("access_log")
    alt = typed(fld["access_log.event_time"])
    anpr = "FROM forensic.records WHERE %s" % S("anpr")
    tx = "FROM forensic.records WHERE %s" % S("transaction")
    ipdr = "FROM forensic.records WHERE %s" % S("ipdr")
    method = raw(fld["access_log.http_method"])
    return [
        ("SHAPE2-min_amount-01", "transaction", "min_amount", "What is the lowest amount in the transactions?",
         "SELECT min(%s) %s" % (typed(fld["transaction.amount"]), tx), "number"),
        ("SHAPE2-max_volume-01", "cdr", "max_volume", "What is the biggest network volume among the call records?",
         "SELECT max(%s) %s" % (typed(fld["cdr.network_volume"]), cdr), "number"),
        ("SHAPE2-mean_duration-01", "cdr", "mean_duration", "What is the mean duration of the calls in seconds?",
         "SELECT avg(%s) %s" % (dur, cdr), "number"),
        ("SHAPE2-tod_cross-01", "cdr", "tod_cross", "How many call records started between 10 pm and 4 am?",
         "SELECT count(*) %s AND extract(hour from %s) IN (22, 23, 0, 1, 2, 3)" % (cdr, t0), "number"),
        ("SHAPE2-tod_plain-01", "access_log", "tod_plain", "How many access log entries were recorded between 9 am and 5 pm?",
         "SELECT count(*) %s AND extract(hour from %s) BETWEEN 9 AND 16" % (log, alt), "number"),
        ("SHAPE2-minutes-01", "cdr", "minutes", "How many calls lasted longer than 20 minutes?",
         "SELECT count(*) %s AND %s > 1200" % (cdr, dur), "number"),
        ("SHAPE2-minutes-02", "cdr", "minutes_at_least", "How many calls lasted at least 25 minutes?",
         "SELECT count(*) %s AND %s >= 1500" % (cdr, dur), "number"),
        ("SHAPE2-versus_methods-01", "access_log", "versus_methods", "How many GET versus POST requests are in the access log?",
         "SELECT count(*) %s AND %s IN ('GET', 'POST') GROUP BY %s ORDER BY %s" % (log, method, method, method), "all"),
        ("SHAPE2-versus_months-01", "cdr", "versus_months", "How many call records were there in April 2026 versus May 2026?",
         "SELECT c FROM (SELECT to_char(%s, 'YYYY-MM') m, count(*) c %s GROUP BY 1) t WHERE m IN ('2026-04', '2026-05') ORDER BY m" % (t0, cdr), "all"),
        ("SHAPE2-which_date-01", "anpr", "which_date", "On which date were the most vehicle sightings recorded?",
         "SELECT to_char(%s, 'YYYY-MM-DD') %s GROUP BY 1 ORDER BY count(*) DESC, 1 LIMIT 1" % (typed(fld["anpr.event_time"]), anpr), "date"),
        ("SHAPE2-weekdays-01", "cdr", "weekdays", "How many call records were made on weekdays?",
         "SELECT count(*) %s AND extract(isodow from %s) BETWEEN 1 AND 5" % (cdr, t0), "number"),
        ("SHAPE2-sat_sun-01", "access_log", "sat_sun", "How many access log entries were recorded on a Saturday or a Sunday?",
         "SELECT count(*) %s AND extract(isodow from %s) IN (6, 7)" % (log, alt), "number"),
        ("SHAPE2-fewest-01", "cdr", "fewest", "Which call type has the fewest records?",
         "SELECT %s %s GROUP BY 1 ORDER BY count(*), 1 LIMIT 1" % (raw(fld["cdr.call_type"]), cdr), "all"),
        ("SHAPE2-sum_bytes-01", "ipdr", "sum_bytes", "What is the total number of bytes transferred in the internet sessions?",
         "SELECT sum(%s) %s" % (typed(fld["ipdr.bytes"]), ipdr), "number"),
        ("SHAPE2-share-01", "access_log", "share", "What percentage of the access log entries are POST requests?",
         "SELECT 100.0 * count(*) FILTER (WHERE %s = 'POST') / count(*) %s" % (method, log), "number"),
    ]


PROBE_SETS = {"1": probes_v1, "2": probes_v2}


def build(db, collection, which="1"):
    items = []
    for qid, family, intent, question, sql, kind in PROBE_SETS[which](db):
        item = f.spec(qid, family, intent, question, sql, kind)
        item["collection"] = collection
        item["expected"] = f.expected_of(db, item)
        items.append(item)
    return items


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--psql", required=True, help="a command that runs psql against the case database, as factory.py takes it")
    parser.add_argument("--collection", default=DEMO)
    parser.add_argument("--set", default="1", choices=sorted(PROBE_SETS), help="1: the first 28 probes; 2: the second set (different questions, same kinds of trouble)")
    parser.add_argument("--out", default=None, help="default questions-shapes.json (set 1) or questions-shapes2.json (set 2)")
    args = parser.parse_args(argv)
    db = f.Db(args.psql, args.collection)
    out = args.out or ("questions-shapes.json" if args.set == "1" else "questions-shapes2.json")
    items = build(db, args.collection, args.set)
    empty = [i["id"] for i in items if i["expected"] in (None, "", [])]
    with io.open(out, "w", encoding="utf-8") as handle:
        json.dump({"seed": "shapes" + args.set, "collection": args.collection, "generated": datetime.datetime.now().isoformat(timespec="seconds"), "questions": items}, handle, indent=1, ensure_ascii=False)
    print("%d probes written to %s" % (len(items), out))
    for i in items:
        print("  %-28s %-6s key: %s" % (i["id"], i["kind"], str(i["expected"])[:90]))
    if empty:
        print("NO KEY for: %s (the question would test nothing)" % ", ".join(empty))
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
