#!/usr/bin/env python3
"""Scores the live engine's answers to the verified bank (tools/showcase/build_verified_bank.py) and writes the sheet.

    python -X utf8 tools/showcase/ask_case.py BANK.json --collection orion-lahore-reference-20261030 --out RESULTS.jsonl
    python -X utf8 tools/showcase/judge_verified_bank.py docs/qa/verified-bank-20261102.json RESULTS.jsonl --out docs/qa/verified-bank-20261102 [--notes docs/qa/verified-bank-notes.json]

Writes <out>.json (the pack format, readable by tools/showcase/render_pack_html.py), <out>.csv, <out>.md and <out>.html. The verdict of a question: correct (the expected answer is in the engine's answer), wrong
(the engine answered with something else), declined or abstained (the engine said it could not, which is honest and is counted apart), not run. Numbers are compared as numbers (5,912,781.7 is 5,912,781.70);
a question whose top answer is one value is correct only when that value is in the FIRST row, before its rivals.
"""
import argparse
import csv
import json
import os
import re
import subprocess
import sys

HERE = os.path.dirname(os.path.abspath(__file__))

AREAS = [
    ("calls", "Calls of one subscriber", "How many voice calls, how long, when, to whom and on which day: counts of calls made and received, talk time, the longest and the average call, calls at night, calls over ten minutes, the first and last event, the number called most, the busiest day, how many different numbers were contacted.", "Ask this case: the sentence first, then the table and \"Behind this answer\"."),
    ("messages", "Text messages of one subscriber", "How many text messages were sent and received, to whom most, and when the first was sent.", "Ask this case."),
    ("pairs", "A subscriber and one other number", "How many calls and messages two numbers had, how many outgoing calls, the total duration, and when they last had contact.", "Ask this case; the lines of the answer open into the records."),
    ("internet", "Internet sessions", "Sessions, bytes downloaded and uploaded, the number of different apps, the app and the host used most, and which number has the most sessions.", "Ask this case."),
    ("apps", "App usage", "Records of app use per number, the number of different apps and the one used most.", "Ask this case."),
    ("bank", "Bank and wallet accounts", "Transactions of an account, the totals credited and debited, the largest debit, the last balance, the first and last day, the account paid the most, how many different accounts were paid, who sent money, debits above an amount.", "Ask this case. Amounts are as the statement gives them."),
    ("plates", "Vehicle plates and cameras", "How often a plate was seen, at which cameras, when first and last; how many sightings a camera recorded and how many different plates; the plate seen the most.", "Ask this case. A sighting says that a camera read a plate, not who was driving."),
    ("handsets", "Handsets", "Which handset (IMEI) a number was seen with and with how many.", "Ask this case."),
    ("cells", "Cell sites", "The cell site a number used most and how many different cell sites it used. A cell site is a coverage area, not a position.", "Ask this case, or Places."),
    ("registrations", "Registrations", "Which SIM identity a number is registered with and the status of the registration. A registration is what a source file states, not who holds the number.", "Ask this case."),
    ("inventory", "The evidence files", "How many files the case holds, in total and by type.", "Ask this case, or Evidence."),
]


def numbers(text):
    return [float(x) for x in re.findall(r"-?\d+(?:\.\d+)?", text.replace(",", ""))]


def render(expect):
    kind = expect.get("kind")
    if kind == "scalar":
        v = expect["value"]
        text = ("{:,.2f}".format(v) if isinstance(v, float) and not float(v).is_integer() else "{:,}".format(int(v)))
        return text + (" " + expect["unit"] if expect.get("unit") else "")
    if kind == "text":
        return str(expect["value"])
    if kind == "set":
        return ", ".join(str(v) for v in expect["values"])
    if kind == "top":
        extra = (" (before " + ", ".join(expect["rivals"]) + ")") if expect.get("rivals") else ""
        return str(expect["value"]) + extra + ((" - %s" % (("{:,.2f}".format(expect["count"]) if isinstance(expect["count"], float) else "{:,}".format(expect["count"])))) if expect.get("count") else "")
    return json.dumps(expect, ensure_ascii=False)


def verdict(item, result):
    state = result.get("state")
    if state != "answered":
        return state or "declined"
    expect = item["expect"]
    answer = result.get("answer") or ""
    rows = result.get("rows") or []
    flat = rows if isinstance(rows, list) else []
    blob = (answer + " " + json.dumps(flat, ensure_ascii=False)).replace(",", "")
    kind = expect["kind"]
    if kind == "scalar":
        value = float(expect["value"])
        found = numbers(blob)
        if any(abs(n - value) < 0.006 for n in found):
            return "correct"
        if value == 0 and re.search(r"no row matches|no record|nothing", blob, re.I):
            return "correct"
        return "wrong"
    if kind == "text":
        return "correct" if str(expect["value"]) in blob else "wrong"
    if kind == "set":
        return "correct" if all(str(v) in blob for v in expect["values"]) else "wrong"
    if kind == "top":
        value = str(expect["value"])
        first = flat[0] if flat and isinstance(flat[0], dict) else None
        if first is not None:
            cells = [str(c) for c in first.values()]
            if not any(value == c or value in c for c in cells):
                return "wrong"
            return "correct"
        positions = [blob.find(value)] + [blob.find(r) for r in expect.get("rivals", [])]
        if positions[0] < 0:
            return "wrong"
        return "correct" if all(p < 0 or positions[0] <= p for p in positions[1:]) else "wrong"
    return "check"


def load_results(path):
    out = {}
    if path and os.path.exists(path):
        for line in open(path, encoding="utf-8"):
            if line.strip():
                r = json.loads(line)
                out[r["id"]] = r
    return out


def main(argv):
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("bank")
    ap.add_argument("results")
    ap.add_argument("--out", required=True)
    ap.add_argument("--notes", default="")
    args = ap.parse_args(argv)
    bank = json.load(open(args.bank, encoding="utf-8"))
    results = load_results(args.results)
    notes = json.load(open(args.notes, encoding="utf-8")) if args.notes and os.path.exists(args.notes) else {}
    questions = []
    for q in bank["questions"]:
        item = dict(q)
        item["expected_text"] = render(q["expect"])
        item["kind_of_check"] = {"scalar": "a number", "text": "a time or a day", "set": "a list of values", "top": "the first of a ranking"}.get(q["expect"]["kind"], "")
        r = results.get(q["id"])
        if r:
            v = verdict(item, r)
            row_text = ""
            rows = r.get("rows")
            if isinstance(rows, list) and rows and isinstance(rows[0], dict):
                row_text = " | first row: " + "; ".join("%s %s" % (k, val) for k, val in list(rows[0].items())[:6])
            item["live"] = {"state": r.get("state"), "answer": ((r.get("answer") or r.get("older_path_answer") or "") + row_text)[:420], "seconds": round((r.get("total_ms") or 0) / 1000, 1), "verdict": v,
                            "sql": (r.get("sql") or "")[:600], "views": r.get("views") or []}
        if q["id"] in notes:
            item["review_note"] = notes[q["id"]]
        item.pop("bank", None)
        questions.append(item)
    pack = {"collection": bank.get("collection"), "areas": [{"key": k, "title": t, "exercises": e, "where": w} for k, t, e, w in AREAS], "questions": questions}
    with open(args.out + ".json", "w", encoding="utf-8", newline="\n") as handle:
        json.dump(pack, handle, indent=1, ensure_ascii=False)
        handle.write("\n")
    with open(args.out + ".csv", "w", encoding="utf-8", newline="") as handle:
        w = csv.writer(handle)
        w.writerow(["id", "family", "question", "expected", "live verdict", "live answer", "seconds"])
        for q in questions:
            live = q.get("live") or {}
            w.writerow([q["id"], q["area"], q["question"], q["expected_text"], live.get("verdict", "not run"), live.get("answer", ""), live.get("seconds", "")])
    counts = {}
    for q in questions:
        key = (q.get("live") or {}).get("verdict", "not run")
        counts.setdefault(q["area"], {}).setdefault(key, 0)
        counts[q["area"]][key] += 1
    lines = ["# Verified question bank: the Orion Lahore reference case", "",
             "%d questions, each with the answer computed by plain SQL on the stored rows (never through the engine) and the live engine's answer scored against it. Synthetic data only." % len(questions), "",
             "| Family | Questions | Correct | Wrong | Declined or abstained | Not run |", "|---|---|---|---|---|---|"]
    total = {"correct": 0, "wrong": 0, "other": 0, "none": 0}
    for key, title, _, _ in AREAS:
        c = counts.get(key, {})
        n = sum(c.values())
        if not n:
            continue
        other = sum(v for k, v in c.items() if k not in ("correct", "wrong", "not run"))
        lines.append("| %s | %d | %d | %d | %d | %d |" % (title, n, c.get("correct", 0), c.get("wrong", 0), other, c.get("not run", 0)))
        total["correct"] += c.get("correct", 0)
        total["wrong"] += c.get("wrong", 0)
        total["other"] += other
        total["none"] += c.get("not run", 0)
    lines += ["| **All** | **%d** | **%d** | **%d** | **%d** | **%d** |" % (len(questions), total["correct"], total["wrong"], total["other"], total["none"]), ""]
    for key, title, exercises, where in AREAS:
        group = [q for q in questions if q["area"] == key]
        if not group:
            continue
        lines += ["## %s" % title, "", exercises, "", "| ID | Question | Expected | Live engine |", "|---|---|---|---|"]
        for q in group:
            live = q.get("live")
            cell = lambda s: str(s).replace("|", "\\|").replace("\n", " ")
            lines.append("| %s | %s | %s | %s |" % (q["id"], cell(q["question"]), cell(q["expected_text"]), cell("%s: %s" % (live["verdict"], live["answer"][:110])) if live else "not run"))
        lines.append("")
    with open(args.out + ".md", "w", encoding="utf-8", newline="\n") as handle:
        handle.write("\n".join(lines) + "\n")
    subprocess.run([sys.executable, "-X", "utf8", os.path.join(HERE, "render_pack_html.py"), args.out + ".json", args.out + ".html"], check=True, capture_output=True)
    html = open(args.out + ".html", encoding="utf-8").read().replace("<title>Orion Query Pack</title>", "<title>Orion Verified Questions</title>")
    open(args.out + ".html", "w", encoding="utf-8", newline="\n").write(html)
    print("%d questions: correct %d, wrong %d, declined or abstained %d, not run %d -> %s.{json,csv,md,html}" % (len(questions), total["correct"], total["wrong"], total["other"], total["none"], args.out))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
