#!/usr/bin/env python3
"""Builds the VERIFIED QUESTION BANK of the Orion Lahore reference case: many questions for every family and every usage, each with the answer computed by plain SQL on the stored rows of the live collection
(read only, the superuser's view of the typed tables, never through the Ask engine's views), so that the answer is independent of the engine that is being tested.

    python -X utf8 tools/showcase/build_verified_bank.py --out docs/qa/verified-bank-20261102.json

The questions are templates filled with the case's own entities (its subscribers, accounts, plates, cameras, documents), chosen from the data (the busiest first), and written so that one reading is possible:
"voice calls" and "text messages" are never mixed, a direction is said, a time is on the case clock (Asia/Karachi), a question whose top answer is tied is not asked. Nothing here writes to the case.

Expectation kinds: scalar (a number, with its unit), text (a time or a day that must occur in the answer), set (every value must occur), top (the first row of the answer must be the value, and it must come
before its rivals). tools/showcase/judge_verified_bank.py reads the engine's answers and scores them.
"""
import argparse
import datetime
import json
import subprocess
import sys

COLLECTION = "orion-lahore-reference-20261030"
TZ = "Asia/Karachi"
SUBS_VOICE_ONLY = ["923000001005"]
COMM = "forensic.comm_event c WHERE c.collection_id = '%s' AND c.dataset_kind = 'target_cdr' AND NOT ('duplicate_row' = ANY (c.quality_flags))" % COLLECTION
SUB = "coalesce(c.a_norm, c.a_raw)"
BANK = "forensic.bank_transaction b WHERE b.collection_id = '%s' AND NOT ('duplicate_row' = ANY (b.quality_flags))" % COLLECTION
SESS = "forensic.data_session s WHERE s.collection_id = '%s' AND NOT ('duplicate_row' = ANY (s.quality_flags))" % COLLECTION
USAGE = "forensic.app_usage u WHERE u.collection_id = '%s' AND NOT ('duplicate_row' = ANY (u.quality_flags))" % COLLECTION
PLATE = "forensic.plate_sighting p WHERE p.collection_id = '%s' AND NOT ('duplicate_row' = ANY (p.quality_flags))" % COLLECTION


def psql(sql):
    out = subprocess.run(["docker", "exec", "-i", "nexusai-forensic-postgres-1", "psql", "-U", "localrecall", "-d", "localrecall", "-At", "-F", "\t", "-v", "ON_ERROR_STOP=1"], input=sql, capture_output=True, text=True, encoding="utf-8")
    if out.returncode != 0:
        raise SystemExit("psql failed: " + out.stderr[-500:] + "\n" + sql[:300])
    return [line.split("\t") for line in out.stdout.strip().splitlines() if line.strip()]


def one(sql):
    rows = psql(sql)
    return rows[0][0] if rows and rows[0] else None


def unique_top(rows):
    """The first row of (value, count) rows when its count is alone at the top, else None."""
    if not rows or (len(rows) > 1 and rows[0][1] == rows[1][1]):
        return None
    return rows


class Bank:
    def __init__(self):
        self.items = []
        self.seq = {}

    def add(self, area, kind, question, expect, note=""):
        self.seq[area] = self.seq.get(area, 0) + 1
        self.items.append({"id": "VB-%s-%03d" % (area.upper()[:4], self.seq[area]), "area": area, "type": kind, "question": question, "expect": expect, "bank": "extra", "note": note})


def build():
    bank = Bank()
    add = bank.add
    subs = [r[0] for r in psql("select %s s from %s group by 1 order by count(*) desc" % (SUB, COMM))]
    sms_subs = [r[0] for r in psql("select %s from %s and c.event_type = 'sms' group by 1 order by count(*) desc" % (SUB, COMM))]

    # ------------------------------------------------------------------ calls and messages of one subscriber
    for s in subs:
        v = "%s and %s = '%s' and c.event_type = 'voice'" % (COMM, SUB, s)
        add("calls", "voice_count", "How many voice calls does %s have in this case?" % s, {"kind": "scalar", "value": int(one("select count(*) from %s" % v))}, "calls made and received, from the number's own records")
        add("calls", "voice_out", "How many outgoing voice calls did %s make?" % s, {"kind": "scalar", "value": int(one("select count(*) from %s and c.direction = 'outgoing'" % v))})
        add("calls", "voice_in", "How many incoming voice calls did %s receive?" % s, {"kind": "scalar", "value": int(one("select count(*) from %s and c.direction = 'incoming'" % v))})
        add("calls", "talk_time", "What is the total talk time of %s across all its voice calls, in seconds?" % s, {"kind": "scalar", "value": int(one("select coalesce(sum(c.duration_s), 0) from %s" % v)), "unit": "seconds"})
        add("calls", "longest", "What is the duration of the longest voice call of %s, in seconds?" % s, {"kind": "scalar", "value": int(one("select max(c.duration_s) from %s" % v)), "unit": "seconds"})
        add("calls", "average", "What is the average duration of the voice calls of %s, in seconds?" % s, {"kind": "scalar", "value": round(float(one("select avg(c.duration_s) from %s" % v)), 2), "unit": "seconds"}, "103.40 and 103.4 are the same answer")
        add("calls", "over10", "How many voice calls of %s lasted more than 10 minutes?" % s, {"kind": "scalar", "value": int(one("select count(*) from %s and c.duration_s > 600" % v))})
        add("calls", "night", "How many voice calls of %s started between 00:00 and 04:59 Karachi time?" % s, {"kind": "scalar", "value": int(one("select count(*) from %s and extract(hour from c.start_utc at time zone '%s') < 5" % (v, TZ)))}, "case clock; the hour 05:00 to 05:59 is not included")
        both = "%s and %s = '%s'" % (COMM, SUB, s)
        add("calls", "first_event", "When was the first call or message of %s?" % s, {"kind": "text", "value": one("select to_char(min(c.start_utc at time zone '%s'), 'YYYY-MM-DD HH24:MI:SS') from %s" % (TZ, both))}, "on the case clock")
        add("calls", "last_event", "When was the last call or message of %s?" % s, {"kind": "text", "value": one("select to_char(max(c.start_utc at time zone '%s'), 'YYYY-MM-DD HH24:MI:SS') from %s" % (TZ, both))}, "on the case clock")
        top = unique_top(psql("select c.dst_key, count(*) from (select coalesce(c.b_norm, c.b_raw) dst_key from %s and c.direction = 'outgoing' and c.b_norm is not null) c group by 1 order by 2 desc, 1 limit 4" % v.replace("forensic.comm_event c WHERE", "forensic.comm_event c WHERE") ))
        if top:
            add("calls", "top_called", "Which number did %s call the most with outgoing voice calls?" % s, {"kind": "top", "value": top[0][0], "rivals": [r[0] for r in top[1:]], "count": int(top[0][1])}, "%s calls; the next number has %s" % (top[0][1], top[1][1] if len(top) > 1 else "none"))
        day = unique_top(psql("select to_char(c.start_utc at time zone '%s', 'YYYY-MM-DD') d, count(*) from %s group by 1 order by 2 desc, 1 limit 3" % (TZ, v)))
        if day:
            add("calls", "busiest_day", "Which day had the most voice calls for %s?" % s, {"kind": "top", "value": day[0][0], "rivals": [r[0] for r in day[1:]], "count": int(day[0][1])}, "%s voice calls that day" % day[0][1])
        distinct = int(one("select count(distinct c.b_norm) from %s and c.b_kind in ('person_like', 'international')" % both))
        add("calls", "distinct_contacts", "How many different numbers did %s contact, leaving out services?" % s, {"kind": "scalar", "value": distinct}, "person-like and international numbers; short codes and sender names are services")

    for s in sms_subs[:6]:
        m = "%s and %s = '%s' and c.event_type = 'sms'" % (COMM, SUB, s)
        add("messages", "sms_count", "How many text messages does %s have in this case?" % s, {"kind": "scalar", "value": int(one("select count(*) from %s" % m))}, "sent and received, from the number's own records")
        add("messages", "sms_out", "How many text messages did %s send?" % s, {"kind": "scalar", "value": int(one("select count(*) from %s and c.direction = 'outgoing'" % m))})
        add("messages", "sms_in", "How many text messages did %s receive?" % s, {"kind": "scalar", "value": int(one("select count(*) from %s and c.direction = 'incoming'" % m))})
        top = unique_top(psql("select coalesce(c.b_norm, c.b_raw), count(*) from %s and c.direction = 'outgoing' and coalesce(c.b_norm, c.b_raw) is not null group by 1 order by 2 desc, 1 limit 4" % m))
        if top:
            add("messages", "sms_top", "Which number did %s send the most text messages to?" % s, {"kind": "top", "value": top[0][0], "rivals": [r[0] for r in top[1:]], "count": int(top[0][1])})
        add("messages", "sms_first", "When was the first text message of %s?" % s, {"kind": "text", "value": one("select to_char(min(c.start_utc at time zone '%s'), 'YYYY-MM-DD HH24:MI:SS') from %s" % (TZ, m))}, "on the case clock")

    # ------------------------------------------------------------------ pairs: one subscriber and one other number
    for s in subs[:8]:
        pairs = unique_top(psql("select c.b_norm, count(*) from %s and %s = '%s' and c.b_norm is not null and c.b_kind in ('person_like', 'international') group by 1 order by 2 desc, 1 limit 3" % (COMM, SUB, s)))
        if not pairs:
            continue
        other = pairs[0][0]
        p = "%s and %s = '%s' and c.b_norm = '%s'" % (COMM, SUB, s, other)
        add("pairs", "pair_events", "How many calls and messages did %s have with %s?" % (s, other), {"kind": "scalar", "value": int(one("select count(*) from %s" % p))}, "in the records of the first number, both directions")
        add("pairs", "pair_voice_out", "How many outgoing voice calls did %s make to %s?" % (s, other), {"kind": "scalar", "value": int(one("select count(*) from %s and c.event_type = 'voice' and c.direction = 'outgoing'" % p))})
        add("pairs", "pair_talk", "What is the total duration of the voice calls between %s and %s, in seconds?" % (s, other), {"kind": "scalar", "value": int(one("select coalesce(sum(c.duration_s), 0) from %s and c.event_type = 'voice'" % p)), "unit": "seconds"})
        add("pairs", "pair_last", "When was the last contact between %s and %s?" % (s, other), {"kind": "text", "value": one("select to_char(max(c.start_utc at time zone '%s'), 'YYYY-MM-DD HH24:MI:SS') from %s" % (TZ, p))}, "on the case clock")

    # ------------------------------------------------------------------ internet sessions and app usage
    for s, n in [(r[0], r[1]) for r in psql("select s.msisdn, count(*) from %s and s.msisdn is not null group by 1 order by 2 desc limit 6" % SESS)]:
        q = "%s and s.msisdn = '%s'" % (SESS, s)
        add("internet", "sessions", "How many internet sessions does %s have?" % s, {"kind": "scalar", "value": int(n)})
        add("internet", "bytes_down", "How many bytes did %s download in total?" % s, {"kind": "scalar", "value": int(one("select coalesce(sum(s.bytes_down), 0) from %s" % q)), "unit": "bytes"})
        add("internet", "bytes_up", "How many bytes did %s upload in total?" % s, {"kind": "scalar", "value": int(one("select coalesce(sum(s.bytes_up), 0) from %s" % q)), "unit": "bytes"})
        add("internet", "apps_distinct", "How many different apps did %s use in its internet sessions?" % s, {"kind": "scalar", "value": int(one("select count(distinct s.app) from %s and s.app is not null" % q))})
        top = unique_top(psql("select s.app, count(*) from %s and s.app is not null group by 1 order by 2 desc, 1 limit 3" % q))
        if top:
            add("internet", "top_app", "Which app did %s use the most in its internet sessions?" % s, {"kind": "top", "value": top[0][0], "rivals": [r[0] for r in top[1:]], "count": int(top[0][1])}, "by number of sessions")
        host = unique_top(psql("select s.host, count(*) from %s and s.host is not null group by 1 order by 2 desc, 1 limit 3" % q))
        if host:
            add("internet", "top_host", "Which host did %s connect to most often?" % s, {"kind": "top", "value": host[0][0], "rivals": [r[0] for r in host[1:]], "count": int(host[0][1])})
    add("internet", "most_sessions", "Which number has the most internet sessions?", {"kind": "top", "value": psql("select s.msisdn from %s and s.msisdn is not null group by 1 order by count(*) desc, 1 limit 1" % SESS)[0][0], "rivals": [], "count": 0})
    for s, n in [(r[0], r[1]) for r in psql("select u.msisdn, count(*) from %s and u.msisdn is not null group by 1 order by 2 desc limit 3" % USAGE)]:
        q = "%s and u.msisdn = '%s'" % (USAGE, s)
        add("apps", "usage_count", "How many app usage records does %s have?" % s, {"kind": "scalar", "value": int(n)})
        add("apps", "usage_distinct", "How many different apps did %s use?" % s, {"kind": "scalar", "value": int(one("select count(distinct u.app) from %s and u.app is not null" % q))})
        top = unique_top(psql("select u.app, count(*) from %s and u.app is not null group by 1 order by 2 desc, 1 limit 3" % q))
        if top:
            add("apps", "usage_top", "Which app did %s use the most?" % s, {"kind": "top", "value": top[0][0], "rivals": [r[0] for r in top[1:]], "count": int(top[0][1])})

    # ------------------------------------------------------------------ bank and wallet
    accounts = [r[0] for r in psql("select b.account_norm from %s group by 1 order by count(*) desc limit 12" % BANK)]
    for a in accounts:
        q = "%s and b.account_norm = '%s'" % (BANK, a)
        add("bank", "txn_count", "How many transactions does the account %s have?" % a, {"kind": "scalar", "value": int(one("select count(*) from %s" % q))})
        add("bank", "credits_total", "What is the total amount credited to the account %s?" % a, {"kind": "scalar", "value": round(float(one("select coalesce(sum(b.amount), 0) from %s and b.direction = 'credit'" % q)), 2), "unit": "rupees"})
        add("bank", "debits_total", "What is the total amount debited from the account %s?" % a, {"kind": "scalar", "value": round(float(one("select coalesce(sum(b.amount), 0) from %s and b.direction = 'debit'" % q)), 2), "unit": "rupees"})
        add("bank", "largest_debit", "What is the largest single debit of the account %s?" % a, {"kind": "scalar", "value": round(float(one("select max(b.amount) from %s and b.direction = 'debit'" % q)), 2), "unit": "rupees"})
        last = psql("select b.balance_after from %s and b.balance_after is not null order by b.txn_utc desc, b.txn_id desc limit 1" % q)
        if last:
            add("bank", "last_balance", "What was the balance after the last transaction of the account %s?" % a, {"kind": "scalar", "value": round(float(last[0][0]), 2), "unit": "rupees"})
        add("bank", "first_day", "On which day was the first transaction of the account %s?" % a, {"kind": "text", "value": one("select to_char(min(b.txn_utc at time zone '%s'), 'YYYY-MM-DD') from %s" % (TZ, q))}, "on the case clock")
        add("bank", "last_day", "On which day was the last transaction of the account %s?" % a, {"kind": "text", "value": one("select to_char(max(b.txn_utc at time zone '%s'), 'YYYY-MM-DD') from %s" % (TZ, q))}, "on the case clock")
        pay = unique_top(psql("select coalesce(b.counterparty_account_norm, b.counterparty_msisdn), sum(b.amount) from %s and b.direction = 'debit' and coalesce(b.counterparty_account_norm, b.counterparty_msisdn) is not null group by 1 order by 2 desc, 1 limit 3" % q))
        if pay:
            add("bank", "top_payee", "Who did the account %s pay the most money to?" % a, {"kind": "top", "value": pay[0][0], "rivals": [r[0] for r in pay[1:]], "count": float(pay[0][1])}, "by total amount debited")
        add("bank", "payees_distinct", "How many different accounts or numbers did the account %s pay?" % a, {"kind": "scalar", "value": int(one("select count(distinct coalesce(b.counterparty_account_norm, b.counterparty_msisdn)) from %s and b.direction = 'debit' and coalesce(b.counterparty_account_norm, b.counterparty_msisdn) is not null" % q))})
        senders = [r[0] for r in psql("select distinct coalesce(b.counterparty_account_norm, b.counterparty_msisdn) from %s and b.direction = 'credit' and coalesce(b.counterparty_account_norm, b.counterparty_msisdn) is not null order by 1" % q)]
        if 1 <= len(senders) <= 8:
            add("bank", "senders", "Which accounts or numbers sent money to the account %s?" % a, {"kind": "set", "values": senders})
        biggest = psql("select b.amount from %s and b.direction = 'debit' order by b.amount desc limit 1" % q)
        if biggest:
            limit = int(float(biggest[0][0]) // 2)
            add("bank", "debits_over", "How many debits of the account %s were above %s rupees?" % (a, limit), {"kind": "scalar", "value": int(one("select count(*) from %s and b.direction = 'debit' and b.amount > %d" % (q, limit)))}, "strictly above")

    # ------------------------------------------------------------------ vehicle plates
    plates = psql("select p.plate_norm, count(*), count(distinct p.camera_id) from %s group by 1 order by 2 desc limit 8" % PLATE)
    for plate, n, cams in plates:
        q = "%s and p.plate_norm = '%s'" % (PLATE, plate)
        add("plates", "plate_count", "How many times was the plate %s seen?" % plate, {"kind": "scalar", "value": int(n)})
        add("plates", "plate_cameras", "At which cameras was the plate %s seen?" % plate, {"kind": "set", "values": [r[0] for r in psql("select distinct p.camera_id from %s order by 1" % q)]})
        add("plates", "plate_first", "When was the plate %s first seen?" % plate, {"kind": "text", "value": one("select to_char(min(p.observed_utc at time zone '%s'), 'YYYY-MM-DD HH24:MI:SS') from %s" % (TZ, q))}, "on the case clock")
        add("plates", "plate_last", "When was the plate %s last seen?" % plate, {"kind": "text", "value": one("select to_char(max(p.observed_utc at time zone '%s'), 'YYYY-MM-DD HH24:MI:SS') from %s" % (TZ, q))}, "on the case clock")
    for cam, n, distinct in psql("select p.camera_id, count(*), count(distinct p.plate_norm) from %s group by 1 order by 1" % PLATE):
        add("plates", "camera_count", "How many plate sightings did the camera %s record?" % cam, {"kind": "scalar", "value": int(n)})
        add("plates", "camera_plates", "How many different plates did the camera %s see?" % cam, {"kind": "scalar", "value": int(distinct)})
    add("plates", "most_seen", "Which plate was seen the most?", {"kind": "top", "value": plates[0][0], "rivals": [r[0] for r in plates[1:3] if r[1] == plates[0][1]], "count": int(plates[0][1])}, "tied plates are all correct" if plates[0][1] == plates[1][1] else "")

    # ------------------------------------------------------------------ handsets
    for s in subs:
        imeis = [r[0] for r in psql("select distinct c.imei from %s and %s = '%s' and c.imei is not null order by 1" % (COMM, SUB, s))]
        if imeis:
            add("handsets", "imei_of", "Which handset (IMEI) was %s seen with?" % s, {"kind": "set", "values": imeis})
            add("handsets", "imei_count", "How many different handsets was %s seen with?" % s, {"kind": "scalar", "value": len(imeis)})

    # ------------------------------------------------------------------ cell sites
    for s in subs[:8]:
        cells = unique_top(psql("select c.cell_key, count(*) from %s and %s = '%s' and c.cell_key is not null group by 1 order by 2 desc, 1 limit 3" % (COMM, SUB, s)))
        if cells:
            add("cells", "top_cell", "Which cell site did %s use the most?" % s, {"kind": "top", "value": cells[0][0], "rivals": [r[0] for r in cells[1:]], "count": int(cells[0][1])}, "by number of events")
        add("cells", "cells_distinct", "How many different cell sites did %s use?" % s, {"kind": "scalar", "value": int(one("select count(distinct c.cell_key) from %s and %s = '%s' and c.cell_key is not null" % (COMM, SUB, s)))})

    # ------------------------------------------------------------------ registrations
    regs = psql("select r.msisdn, count(*), min(r.imsi), min(r.status) from forensic.subscriber_registration r where r.collection_id = '%s' and r.msisdn is not null and r.imsi is not null group by 1 having count(*) = 1 order by 1 limit 8" % COLLECTION)
    for n, _, imsi, status in regs:
        add("registrations", "reg_imsi", "Which SIM identity (IMSI) is registered for the number %s?" % n, {"kind": "set", "values": [imsi]}, "a registration is what a source file states, not who holds the number")
        if status:
            add("registrations", "reg_status", "What is the status of the registration of the number %s?" % n, {"kind": "set", "values": [status]})

    # ------------------------------------------------------------------ the evidence files
    for modality, n in psql("select i.modality, count(*) from forensic.evidence_items i where i.collection_id = '%s' group by 1 order by 1" % COLLECTION):
        add("inventory", "files_by_modality", "How many %s files are in this case?" % {"structured_records": "structured record", "text": "text", "document": "document", "tabular": "tabular"}.get(modality, modality), {"kind": "scalar", "value": int(n)}, "the evidence files of the case, by their type")
    add("inventory", "files_total", "How many evidence files are in this case?", {"kind": "scalar", "value": int(one("select count(*) from forensic.evidence_items i where i.collection_id = '%s'" % COLLECTION))})

    return bank.items


def main(argv):
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--out", required=True)
    args = ap.parse_args(argv)
    items = build()
    areas = {}
    for i in items:
        areas[i["area"]] = areas.get(i["area"], 0) + 1
    with open(args.out, "w", encoding="utf-8", newline="\n") as handle:
        json.dump({"collection": COLLECTION, "built": datetime.date.today().isoformat(), "questions": items}, handle, indent=1, ensure_ascii=False)
        handle.write("\n")
    print("%d questions -> %s" % (len(items), args.out))
    for area, n in sorted(areas.items()):
        print("  %-14s %d" % (area, n))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
