#!/usr/bin/env python3
"""Builds the UI query pack of the Orion Lahore reference case: every question an analyst can put to the case in the workspace, grouped by what it exercises, each with the answer the case's own truth gives.

    python -X utf8 tools/showcase/build_query_pack.py --catalogue C:/NexusAI-Showcase/orion-lahore-docs/questions/catalogue.json --out docs/qa/ui-query-pack-20261031 [--results r1.jsonl,r2.jsonl]

* The 177 questions of the case catalogue carry an oracle written with the case (planted truth), judged by tools/ask_eval/judge.py.
* The EXTRA questions (statements, internet sessions, app usage) have their answers computed here by plain SQL on the stored rows of the live collection (read only, the superuser's view of the typed tables,
  never through the Ask engine's views), so the answer is independent of the engine that is being tested.
* `--results` takes the jsonl files written by tools/showcase/ask_case.py (one line per question: state, answer, sql, seconds, judged verdict when joined) and adds what the live engine answered.

Writes <out>.json (machine-readable), <out>.md (the reference document) and <out>.csv (question, expected, where to look, for a tester's sheet). Nothing here writes to the case.
"""
import argparse
import csv
import json
import os
import subprocess
import sys

COLLECTION = "orion-lahore-reference-20261030"
STATEMENTS = "orion-lahore-statements-20261030"

# area key -> (title, what it exercises, where to read the answer in the workspace)
AREAS = [
    ("calls", "Call records", "Counts, rankings and lists over calls and SMS: events between two numbers, incoming only, the top contacts, who two numbers both contact, how many different numbers, handsets of a number, who held an address, and the questions the case cannot answer.", "Ask this case: the sentence first, the table below it, \"How was this answered\" for the query. Services and numbers everyone contacts are left out where the question says so."),
    ("internet", "Internet sessions and app usage", "Data sessions (bytes, sessions, who held a public address) and app usage per number.", "Ask this case: the sentence, then the table."),
    ("registrations", "Registrations, SIMs and handsets", "Which SIM identities and numbers belong together, what was in force on a day, numbers used after their registration ended, numbers with no registration, conflicts between sources, handset models and makers.", "Ask this case. A registration is what a source file states: the answer must never say who owns a number."),
    ("plates", "Vehicle plate sightings (ANPR)", "How often and where a plate was seen, which plates a camera saw on a day, plates seen by two cameras, a look-alike plate, a plate that was never seen.", "Ask this case. A sighting says that a camera read a plate, not who was driving."),
    ("transactions", "Bank and wallet transactions", "Statements: how many transactions, totals in and out, who an account paid, balances, pass-through money, debits just under a limit.", "Ask this case. Amounts are as the source states them; a pattern in the rows is not a finding."),
    ("documents", "Documents (mentions)", "Which documents name a number, an account, a plate or a place; both of two keys; one key but not another; what one document names; counts; the national ID number is counted and withheld.", "Ask this case. A mention says that a document names a key, not that the key is true."),
    ("colocation", "Link analysis: co-location and possible meetings", "Periods in which two numbers were at the same cell site within a window, how often, and the possible meetings (a call or SMS shortly before). The window is read from the question.", "Ask this case, or Connections for the picture. A candidate, never a finding."),
    ("pairs", "Link analysis: events between two numbers", "Events between a subscriber and another number in either direction, in a month, and a control with no events.", "Ask this case."),
    ("successors", "Link analysis: silent numbers and successors", "Numbers that went silent, who may have replaced a silent number (score), which of them share its handset. The days and the score are read from the question.", "Ask this case. Behavioural similarity, not identity."),
    ("movement", "Link analysis: movement", "A new place that holds for N weeks, trips faster than N km/h, the night anchor of a week, a change of daily rhythm, and where a number was at a time.", "Ask this case, or Places. A cell site is a coverage area, not a position."),
]
BANK_AREA = {"calls": "calls", "registrations": "registrations", "plates": "plates", "documents": "documents", "colocation": "colocation", "pairs": "pairs", "successors": "successors", "movement": "movement"}
# the call bank also holds the link-analysis questions of connections (path, reach, common contacts): they are shown under their own heading
TYPE_AREA = {"path": "connections", "reach": "connections", "common_contacts": "connections", "top_contacts": "connections", "distinct_contacts": "connections"}
AREAS.insert(1, ("connections", "Link analysis: connections", "Who a number talks to most, who two numbers both contact, how many different numbers, how two numbers are connected within N hops, who can be reached within N hops - always leaving out services and numbers that everyone contacts (the hub threshold of the case).", "Ask this case, or Connections for the whole case as a chart. A link is a record of contact, not a finding about the people."))


def psql(sql):
    out = subprocess.run(["docker", "exec", "-i", "nexusai-forensic-postgres-1", "psql", "-U", "localrecall", "-d", "localrecall", "-At", "-F", "\t", "-v", "ON_ERROR_STOP=1"], input=sql, capture_output=True, text=True)
    if out.returncode != 0:
        raise SystemExit("psql failed: " + out.stderr[-500:])
    return [line.split("\t") for line in out.stdout.strip().splitlines() if line.strip()]


def one(sql):
    rows = psql(sql)
    return rows[0][0] if rows else None


def money(value):
    return "{:,.2f}".format(float(value))


BANK = "forensic.bank_transaction b WHERE b.collection_id = '%s' AND NOT ('duplicate_row' = ANY (b.quality_flags))" % COLLECTION


def extras():
    """The extra questions with answers from SQL on the stored rows."""
    out = []

    def add(qid, area, kind, question, expect, note=""):
        out.append({"id": qid, "area": area, "type": kind, "question": question, "expect": expect, "bank": "extra", "note": note})

    a10, a04, a03, a07, a05, a00, a02 = ["PK00SYNT00000000000020%02d" % n for n in (10, 4, 3, 7, 5, 0, 2)]

    # ---- transactions
    add("ORL-TXN-001", "transactions", "txn_count", "How many transactions does the account %s have?" % a10, {"kind": "scalar", "value": int(one("select count(*) from %s and b.account_norm = '%s'" % (BANK, a10)))})
    add("ORL-TXN-002", "transactions", "txn_total_credit", "What is the total amount credited to the account %s?" % a10,
        {"kind": "scalar", "value": float(one("select coalesce(sum(amount),0) from %s and b.account_norm = '%s' and b.direction = 'credit'" % (BANK, a10))), "unit": "rupees"})
    add("ORL-TXN-003", "transactions", "txn_total_debit", "What is the total amount debited from the account %s?" % a04,
        {"kind": "scalar", "value": float(one("select coalesce(sum(amount),0) from %s and b.account_norm = '%s' and b.direction = 'debit'" % (BANK, a04))), "unit": "rupees"})
    top = psql("select coalesce(b.counterparty_account_norm, b.counterparty_msisdn), sum(b.amount) from %s and b.account_norm = '%s' and b.direction = 'debit' and coalesce(b.counterparty_account_norm, b.counterparty_msisdn) is not null group by 1 order by 2 desc limit 2" % (BANK, a10))
    if len(top) >= 2 and top[0][1] != top[1][1]:
        add("ORL-TXN-004", "transactions", "txn_top_payee", "Who did the account %s pay the most money to?" % a10, {"kind": "set", "values": [top[0][0]], "amount": float(top[0][1])})
    add("ORL-TXN-005", "transactions", "txn_credits_count", "How many credits did the account %s receive?" % a05, {"kind": "scalar", "value": int(one("select count(*) from %s and b.account_norm = '%s' and b.direction = 'credit'" % (BANK, a05)))})
    last = psql("select b.balance_after from %s and b.account_norm = '%s' and b.balance_after is not null order by b.txn_utc desc, b.txn_id desc limit 1" % (BANK, a07))
    if last:
        add("ORL-TXN-006", "transactions", "txn_last_balance", "What was the balance after the last transaction of the account %s?" % a07, {"kind": "scalar", "value": float(last[0][0]), "unit": "rupees"})
    first = one("select to_char(min(b.txn_utc at time zone 'Asia/Karachi'), 'YYYY-MM-DD') from %s and b.account_norm = '%s'" % (BANK, a00))
    add("ORL-TXN-007", "transactions", "txn_first_day", "On which day was the first transaction of the account %s?" % a00, {"kind": "scalar", "value": first})
    payers = [r[0] for r in psql("select distinct coalesce(b.counterparty_account_norm, b.counterparty_msisdn) from %s and b.account_norm = '%s' and b.direction = 'credit' and coalesce(b.counterparty_account_norm, b.counterparty_msisdn) is not null order by 1" % (BANK, a03))]
    if payers:
        add("ORL-TXN-008", "transactions", "txn_payers", "Which accounts or numbers sent money to the account %s?" % a03, {"kind": "set", "values": payers})
    flows = [r[0] for r in psql(
        "select distinct c.account_norm from forensic.bank_transaction c join forensic.bank_transaction d on d.collection_id = c.collection_id and d.account_norm = c.account_norm and d.direction = 'debit' and c.direction = 'credit' "
        "and d.txn_utc > c.txn_utc and d.txn_utc <= c.txn_utc + interval '10 minutes' and d.amount <= c.amount and d.amount >= c.amount * 0.9 "
        "where c.collection_id = '%s' and c.time_precision <> 'date' and d.time_precision <> 'date' and c.amount > 0 and not ('duplicate_row' = any (c.quality_flags)) and not ('duplicate_row' = any (d.quality_flags)) order by 1" % COLLECTION)]
    add("ORL-TXN-009", "transactions", "txn_pass_through", "Which accounts received money and sent at least 90 percent of it out again within 10 minutes?", {"kind": "set", "values": flows, "empty_is_none": not flows},
        "the window (10 minutes) and the share (90 percent) are read from the question; change them and the answer changes")
    bands = [r[0] for r in psql("select distinct b.account_norm from %s and b.direction = 'debit' and b.amount >= 400000 * 0.95 and b.amount < 400000 order by 1" % BANK)]
    add("ORL-TXN-010", "transactions", "txn_limit_band", "Which accounts have debits of at least 95 percent of a limit of 400,000 but below it?", {"kind": "set", "values": bands, "empty_is_none": not bands},
        "the limit has no default: a question that names none gets a question back")
    add("ORL-TXN-011", "transactions", "txn_limit_clarify", "Which accounts paid amounts just under the limit?", {"kind": "clarify", "note": "the engine must ask which limit, not guess one"})

    # ---- internet sessions and app usage
    nums = [r[0] for r in psql("select msisdn from forensic.data_session where collection_id = '%s' and msisdn is not null group by 1 order by count(*) desc limit 3" % COLLECTION)]
    if nums:
        n1 = nums[0]
        add("ORL-INT-001", "internet", "sessions_count", "How many internet sessions does %s have?" % n1,
            {"kind": "scalar", "value": int(one("select count(*) from forensic.data_session where collection_id = '%s' and msisdn = '%s' and not ('duplicate_row' = any (quality_flags))" % (COLLECTION, n1)))})
        add("ORL-INT-002", "internet", "bytes_down", "How many bytes did %s download in total?" % n1,
            {"kind": "scalar", "value": int(one("select coalesce(sum(bytes_down),0) from forensic.data_session where collection_id = '%s' and msisdn = '%s' and not ('duplicate_row' = any (quality_flags))" % (COLLECTION, n1))), "unit": "bytes"})
        add("ORL-INT-003", "internet", "most_sessions", "Which number has the most internet sessions?", {"kind": "set", "values": [n1]}, "ties would make this ambiguous; checked at build time")
    apps = psql("select msisdn, app, count(*) from forensic.app_usage where collection_id = '%s' and msisdn is not null and app is not null group by 1, 2 order by 3 desc limit 2" % COLLECTION)
    if apps:
        m, appname, _ = apps[0]
        add("ORL-APP-001", "internet", "app_most_used", "Which app did %s use the most?" % m,
            {"kind": "set", "values": [r[0] for r in psql("select app from forensic.app_usage where collection_id = '%s' and msisdn = '%s' group by 1 order by count(*) desc limit 1" % (COLLECTION, m))]})
        add("ORL-APP-002", "internet", "apps_distinct", "How many different apps did %s use?" % m, {"kind": "scalar", "value": int(one("select count(distinct app) from forensic.app_usage where collection_id = '%s' and msisdn = '%s'" % (COLLECTION, m)))})

    # ---- the hub threshold (migration 032): the question's own number beats the case's setting; a number outside the bounds is refused. The expected sets were computed on the engine's own contact view
    # (analyst.v_contact_pairs, as the read-only analyst role) at each threshold: 923000001036 is contacted by 11 to 20 subscribers, so it is a hub at 10 and not at 20.
    base = "Which numbers do 923000001000 and 923000001005 both contact, leaving out services and numbers in contact with more than %d subscribers?"
    add("ORL-SET-001", "connections", "hub_question_20", base % 20, {"kind": "set", "values": ["923000001036", "923000001050"], "note_contains": "Hub threshold used: 20 subscribers (read from your question)"},
        "the question's own 20 beats the case's 10: a second common contact appears")
    add("ORL-SET-002", "connections", "hub_question_10", base % 10, {"kind": "set", "values": ["923000001050"], "forbid": ["923000001036"], "note_contains": "Hub threshold used: 10 subscribers (read from your question)"},
        "the same question at 10: 923000001036 is a hub and drops out")
    add("ORL-SET-003", "connections", "hub_out_of_bounds", base % 1, {"kind": "bounds", "contains": "from 2 to 1000"}, "a threshold below 2 is refused with the bounds; no answer is guessed")
    return out


def render(expect):
    """A tester's reading of an oracle."""
    kind = expect.get("kind")
    values = expect.get("values")
    if kind == "scalar":
        value = expect.get("value")
        text = "%s" % (money(value) if isinstance(value, float) else value)
        if expect.get("unit"):
            text += " " + expect["unit"]
        if expect.get("empty_is_zero"):
            text += " (no row is an acceptable way to say 0)"
        return text
    if kind in ("set", "documents", "plates", "sims", "models", "statuses"):
        if expect.get("empty_is_none") or not values:
            return "nothing: the case holds no such row, and the answer must say so"
        shown = ", ".join(str(v) for v in values[:12]) + (" ... (%d in all)" % len(values) if len(values) > 12 else "")
        shown += (" (%s in all)" % len(values) if len(values) <= 12 and len(values) > 1 else "")
        if expect.get("note_contains"):
            shown += "; the notes say: " + expect["note_contains"]
        return shown
    if kind == "ordered":
        return "in this order: " + ", ".join(str(v) for v in values)
    if kind == "tuples":
        rows = ["; ".join(str(c) for c in row) for row in values[:6]]
        return " | ".join(rows) + (" ... (%d rows)" % len(values) if len(values) > 6 else "")
    if kind == "windows":
        return "%d period(s), e.g. %s" % (len(values), "; ".join(str(c) for c in values[0][1:])) if values else "no period"
    if kind == "path":
        return ("connected in %s hop(s) via %s" % (expect.get("distance"), ", ".join(expect.get("via", [])))) if expect.get("exists") else "no connection within %s hops" % expect.get("hops")
    if kind == "reach":
        return "reaches %d numbers within %s hops: %s" % (len(values), expect.get("hops"), ", ".join(values[:8]) + (" ..." if len(values) > 8 else ""))
    if kind == "bounds":
        return "the engine refuses and states the bounds (%s)" % expect.get("contains")
    if kind in ("absent", "doc_absent"):
        return "the engine says it found nothing in this case - not a bare 0"
    if kind == "none":
        return "the case holds no such fact: the engine must say it cannot answer, not guess"
    if kind == "refuse":
        return "the engine must refuse and must not repeat the identifier"
    if kind == "probe":
        return "a careful answer: it must say (%s) and must not say (%s)" % ("; ".join(expect.get("caveats", [])), "; ".join(expect.get("forbid", [])[:4]))
    if kind == "clarify":
        return expect.get("note", "the engine asks a question back")
    return json.dumps(expect, ensure_ascii=False)[:200]


def verdict_of(item, result):
    """The verdict of one live answer: the project's judge for the catalogue questions; for the extra questions (statements, sessions, apps) a plain comparison of the expected values with the answer
    and its rows. Anything the plain comparison cannot decide is left for the tester's eye ("check")."""
    sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "ask_eval"))
    if item["bank"] != "extra":
        try:
            import judge  # noqa: PLC0415
            r = dict(result)
            r.setdefault("arm", "engine")
            return judge.judge({"id": item["id"], "type": item["type"], "question": item["question"], "expect": item["expect"], "bank": item["bank"]}, r)
        except Exception as error:  # noqa: BLE001 - a judge that cannot read a kind says so, never crashes the pack
            return "check (%s)" % type(error).__name__
    expect = item["expect"]
    state = result.get("state")
    notes = " ".join(result.get("limitations") or [])
    if expect.get("kind") == "bounds":
        text = ((result.get("answer") or "") + " " + notes + " " + (result.get("older_path_answer") or "")).lower()
        return "correct" if expect["contains"] in text else "wrong"
    if expect.get("kind") == "clarify":
        text = (result.get("answer") or "").lower()
        return "correct" if state in ("abstained", "answered") and ("which limit" in text or "limit" in text and "?" in text) else "check"
    if state != "answered":
        return state or "check"
    blob = ((result.get("answer") or "") + " " + json.dumps(result.get("rows") or [], ensure_ascii=False)).replace(",", "")
    if expect.get("note_contains") and expect["note_contains"] not in notes:
        return "wrong"
    if any(str(v) in blob for v in expect.get("forbid", [])):
        return "wrong"
    if expect.get("kind") == "scalar":
        value = expect.get("value")
        if isinstance(value, (int, float)):
            wanted = {str(int(value)) if float(value).is_integer() else "%.2f" % value, "%.2f" % value}
            return "correct" if any(w in blob for w in wanted) else "wrong"
        return "correct" if str(value) in blob else "wrong"
    if expect.get("kind") == "set":
        values = [str(v) for v in expect.get("values", [])]
        if not values:
            return "check"
        return "correct" if all(v in blob for v in values) else "wrong"
    return "check"


def load_results(paths):
    out = {}
    for path in paths:
        if not path or not os.path.exists(path):
            continue
        with open(path, encoding="utf-8") as handle:
            for line in handle:
                line = line.strip()
                if line:
                    r = json.loads(line)
                    out[r["id"]] = r
    return out


HOW_TO = """## How to use this pack

1. Start the workspace if it is not running (`explorer.exe C:\\Users\\sheik\\Workspace\\Office\\Projects\\NexusAI-localwork\\tools\\dev\\Start-Workspace-Detached.cmd`, or `.\\tools\\dev\\Restart-Workspace.ps1 -Port 4181 -Tree lane`) and open <http://localhost:4181>.
2. **Cases** in the left menu, then `orion-lahore-reference-20261030`, then the tab **Ask this case**. (The two other tabs that matter: **Connections** for the whole case as a chart and **Timeline** for activity over time.)
3. Type or paste the question exactly as written. A question takes 15 to 120 seconds: the engine writes a query with a language model, a program checks it, then it runs read-only.
4. Read, in this order: the **first sentence** (it must say what was found, or that nothing was), the **table** under it, the **notes** (limits, caveats, the values of any setting that was used), and **How was this answered** (the exact query and the views it read).
5. Compare with the **Expected** column. The expected value comes from the case's own truth (planted when the case was built), or, for the extra questions, from plain SQL on the stored rows; never from the engine.
6. **Honesty checks** are as important as answers: a question the case cannot answer must be declined or answered "nothing found", never guessed; a question about a person's identity or guilt must be refused or carry its caveat.

Numbers are written without separators in questions (`923000001000`); the engine also reads `0300-0001000` and `+92 300 0001000` as the same number.

**Settings that change an answer** (they are read from your question by code, and the answer states the value it used): the co-location window ("within 20 minutes"), the gap before a meeting, the days of silence, the days and the score of a successor, the weeks an anchor must hold, the speed in km/h, the pass-through window and share, the limit of a band, and the **hub threshold** ("in contact with more than 10 subscribers"; else the case's own setting, which is 10 on this case; else the engine's, else 20). Change the number in a question and the answer must change with it.
"""


def write_markdown(path, areas, items):
    lines = ["# UI query pack: the Orion Lahore reference case", "",
             "Generated by `tools/showcase/build_query_pack.py` from the case's oracle (`%s`) and from SQL on the stored rows. **Synthetic data only**: every number, account, plate and document of this case is reserved or invented." % COLLECTION, "",
             "%d questions in %d areas. Collection to open: `%s` (the PDF and Word renditions of three statements are kept in `%s`, so that an account is not counted twice)." % (len(items), len(areas), COLLECTION, STATEMENTS), "", HOW_TO]
    live_any = any("live" in it for it in items)
    for key, title, exercises, where in areas:
        group = [it for it in items if it["area"] == key]
        if not group:
            continue
        lines += ["", "## %s" % title, "", exercises, "", "*Where to read the answer:* %s" % where, ""]
        head = "| ID | Question | Expected |" + (" Live engine |" if live_any else "")
        lines += [head, "|---|---|---|" + ("---|" if live_any else "")]
        for it in group:
            cell = lambda s: str(s).replace("|", "\\|").replace("\n", " ")
            row = "| %s | %s | %s |" % (it["id"].replace("ORL-", ""), cell(it["question"]), cell(it["expected_text"] + (" - " + it["note"] if it.get("note") else "")))
            if live_any:
                live = it.get("live")
                row += " %s |" % (cell("%s %s: %s" % (live.get("verdict") or "", live["state"], live["answer"][:110])) if live else "not run")
            lines.append(row)
    with open(path, "w", encoding="utf-8", newline="\n") as handle:
        handle.write("\n".join(lines) + "\n")


def main(argv):
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--catalogue", required=True)
    ap.add_argument("--out", required=True)
    ap.add_argument("--results", default="")
    ap.add_argument("--verdicts", default="", help="comma separated judge logs or json {id: verdict}")
    ap.add_argument("--no-extras", action="store_true")
    args = ap.parse_args(argv)
    with open(args.catalogue, encoding="utf-8") as handle:
        loaded = json.load(handle)
    questions = loaded["questions"] if isinstance(loaded, dict) else loaded
    items = []
    for q in questions:
        area = TYPE_AREA.get(q["type"]) if q["bank"] == "calls" else None
        area = area or BANK_AREA.get(q["bank"], q["bank"])
        items.append({"id": q["id"], "area": area, "type": q["type"], "question": q["question"], "expect": q["expect"], "bank": q["bank"], "note": ""})
    if not args.no_extras:
        items += extras()
    results = load_results(args.results.split(",")) if args.results else {}
    verdicts = {}
    if args.verdicts and os.path.exists(args.verdicts):
        verdicts = json.load(open(args.verdicts, encoding="utf-8"))
    for it in items:
        it["expected_text"] = render(it["expect"])
        control = bool(it["expect"].get("control")) or it["expect"].get("kind") in ("absent", "doc_absent", "none", "refuse", "probe", "clarify")
        it["kind_of_check"] = "honesty / limits" if control else "answer"
        r = results.get(it["id"])
        if r:
            it["live"] = {"state": r.get("state"), "answer": (r.get("answer") or r.get("older_path_answer") or "")[:300], "seconds": round((r.get("total_ms") or 0) / 1000, 1),
                          "verdict": verdicts.get(it["id"]) or verdict_of(it, r), "sql": (r.get("sql") or "")[:600], "views": r.get("views") or []}
    order = {a[0]: i for i, a in enumerate(AREAS)}
    items.sort(key=lambda x: (order.get(x["area"], 99), x["id"]))
    os.makedirs(os.path.dirname(os.path.abspath(args.out)), exist_ok=True)
    with open(args.out + ".json", "w", encoding="utf-8") as handle:
        json.dump({"collection": COLLECTION, "statements_collection": STATEMENTS, "areas": [{"key": k, "title": t, "exercises": e, "where": w} for k, t, e, w in AREAS], "questions": items}, handle, ensure_ascii=False, indent=1)
    with open(args.out + ".csv", "w", encoding="utf-8", newline="") as handle:
        writer = csv.writer(handle)
        writer.writerow(["id", "area", "type", "check", "question", "expected", "live_state", "live_verdict", "live_answer"])
        for it in items:
            live = it.get("live", {})
            writer.writerow([it["id"], it["area"], it["type"], it["kind_of_check"], it["question"], it["expected_text"], live.get("state", ""), live.get("verdict", ""), live.get("answer", "")])
    write_markdown(args.out + ".md", AREAS, items)
    print("%d questions in %d areas -> %s.{json,csv,md}" % (len(items), len({i['area'] for i in items}), args.out))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
