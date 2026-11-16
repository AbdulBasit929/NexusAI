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
sys.path.insert(0, os.path.join(HERE, "..", "video_plates"))
import video_question_bank as vqb  # noqa: E402  (its judge scores the video questions; its mask hides plates)

PRIVATE_BANK = os.environ.get("ORION_VIDEO_PRIVATE_BANK", "C:/NexusAI-Showcase/orion/key/video-private-bank.json")  # plates read from the owner's real footage: this machine only
VIDEO_SUFFIXES = (".mp4", ".mov", ".avi", ".mkv")
VIDEO_AREAS = ("videos", "vsearch")
PLATE_ROUTE_AREAS = ("imgplate", "vsearch")     # questions with a typed plate: answered by the plate-in-images route, not by the Ask engine's SQL
TEXT_ROUTE_TYPES = ("audio_word_search",)       # a word to find in the recordings: answered by the governed text search (the text-intent step routes it before the engine), not by the engine's SQL
INCOMPLETE_SENTENCE = re.compile(r"search is incomplete|absence of further matches has not been established", re.I)

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
    ("speech", "Speech recordings (Pashto, Urdu, English)", "The ten audio recordings of the case, public speech of real speakers: how many there are and how long, which are in Pashto, Urdu or English, how many segments a recording has, which recordings say a word, and what a recording says. The text is a machine transcript; Pashto is a search lead, not a transcript.", "Ask this case; open a recording to hear it and read the transcript beside the player."),
    ("imgplate", "Plates in photos", "Which photos show a plate, and what is said when no photo shows it. The photos are constructed (a real car photo with an Orion plate written on it).", "Ask this case; a reading is a model's candidate, shown as an exact match or as a similar reading to check by eye."),
    ("videos", "Videos, speech and text read from them", "The five real video clips of the case: how many plates were read in a clip, when the first appears, which plate stays in view longest, how long a clip is, which clips have sound, speech segments and pieces of text read from the pictures. The plates themselves are masked here.", "Ask this case; under \"Behind this answer\" each reading opens the video at its second."),
    ("vsearch", "Typed plate search over the videos", "The plate that stays in view longest in each clip, typed as an analyst would type it: every clip that holds a reading of exactly that plate must be named. The plate is masked here.", "Ask this case: \"Which photos or videos show the plate ...?\" The answer names each clip and the moments, and lists a reading that differs by one or two characters as \"check by eye\"."),
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
    if kind == "files":
        return ", ".join(str(v) for v in expect["values"]) if expect["values"] else "none"
    if kind == "words":
        parts = []
        if expect.get("any"):
            parts.append("one of: " + ", ".join(expect["any"]))
        if expect.get("all"):
            parts.append("all of: " + ", ".join(expect["all"]))
        if expect.get("forbid"):
            parts.append("and not: " + ", ".join(expect["forbid"]))
        return "; ".join(parts)
    if kind == "top":
        extra = (" (before " + ", ".join(expect["rivals"]) + ")") if expect.get("rivals") else ""
        return str(expect["value"]) + extra + ((" - %s" % (("{:,.2f}".format(expect["count"]) if isinstance(expect["count"], float) else "{:,}".format(expect["count"])))) if expect.get("count") else "")
    if kind == "video":
        if expect.get("private"):
            return "%d plate%s read from the footage (kept in a private file)" % (expect["n"], "" if expect["n"] == 1 else "s")
        value = expect.get("value")
        if isinstance(value, list):
            return ", ".join(str(v) for v in value) if value else "none"
        return "{:,}".format(value) if isinstance(value, int) else str(value)
    return json.dumps(expect, ensure_ascii=False)


def video_verdict(item, result, private):
    """A question about the video clips, judged as tools/video_plates/video_question_bank.py judges it: a number within its tolerance, a set that is exactly the expected files or plates (no extra),
    the first row of a ranking among the acceptable labels, or (typed plate search) every expected clip named. An answer that matches but whose query does not read the view the question calls for is
    'lucky' and does not count as correct. Without the private bank on this machine a question that expects plates is not run."""
    expect = item["expect"]
    values = private.get(item["id"]) if expect.get("private") else expect.get("value")
    if values is None:
        return "not run"
    answer = result.get("answer") or ""
    rows = result.get("rows") or []
    if expect["vkind"] == "named":
        blob = answer + " " + json.dumps(rows, ensure_ascii=False)
        return "correct" if all(str(v) in blob for v in values) else "wrong"
    verdict_, _ = vqb.judge({"kind": expect["vkind"], "expected": values, "tolerance": expect.get("tolerance", 0.0), "what": expect.get("what")}, answer, rows)
    if verdict_ == "correct":
        wanted = [v for v in (expect.get("expect_view") or "").split("|") if v]
        if wanted and not any(v in (result.get("sql") or "") for v in wanted):
            return "lucky"
    return verdict_


def verdict(item, result, private=None):
    state = result.get("state")
    if state == "declined" and item.get("area") in PLATE_ROUTE_AREAS and not result.get("header") and result.get("older_path_answer"):
        # a plate typed into a question is answered by the plate-in-images route, which comes before the Ask engine and sets no engine header: ask_case.py calls that "declined"; it is the designed answer
        result = dict(result, state="answered", answer=result["older_path_answer"])
        state = "answered"
    elif state == "declined" and item.get("type") in TEXT_ROUTE_TYPES and not result.get("header") and result.get("older_path_answer") and not INCOMPLETE_SENTENCE.search(result["older_path_answer"]):
        # owner's decision 2026-11-16: the governed text search is a designed answer route, so its answer is judged like any other. An answer that itself says the search is incomplete stays "declined":
        # it makes no claim to be the full list, and the file check below is exact (a missing or an extra recording is wrong)
        result = dict(result, state="answered", answer=result["older_path_answer"])
        state = "answered"
    if result.get("timed_out") or result.get("error") == "TimeoutError":
        return "timed out"          # the engine said it could not finish in time: no verdict about the content (a timeout is never a verdict)
    if state != "answered":
        return state or "declined"
    expect = item["expect"]
    if expect.get("kind") == "video":
        return video_verdict(item, result, private or {})
    answer = result.get("answer") or ""
    rows = result.get("rows") or []
    flat = rows if isinstance(rows, list) else []
    blob = (answer + " " + json.dumps(flat, ensure_ascii=False)).replace(",", "")
    kind = expect["kind"]
    if kind == "scalar":
        value = float(expect["value"])
        found = numbers(blob)
        if any(abs(n - value) <= float(expect.get("tolerance", 0.006)) for n in found):
            return "correct"
        if value == 0 and re.search(r"no row matches|no record|nothing", blob, re.I):
            return "correct"
        return "wrong"
    if kind == "text":
        return "correct" if str(expect["value"]) in blob else "wrong"
    if kind == "set":
        return "correct" if all(str(v) in blob for v in expect["values"]) else "wrong"
    if kind == "files":
        # the files the answer names must be exactly the expected ones among the files of its universe (a pattern): a missing file or an extra one is wrong
        named = set(re.findall(expect.get("universe", r"[\w.-]+\.\w{2,4}"), blob))
        wanted = set(str(v) for v in expect["values"])
        if not wanted:
            return "correct" if not named or re.search(r"no row matches|no record|no recording|none|nothing|0 ", blob, re.I) else "wrong"
        return "correct" if named == wanted else "wrong"
    if kind == "words":
        low = blob.lower()
        def has(word):
            return re.search(r"(?<![\w])" + re.escape(str(word).lower()) + r"(?![\w])", low, re.U) is not None
        ok = (not expect.get("any") or any(has(w) for w in expect["any"])) and all(has(w) for w in expect.get("all", [])) and not any(has(w) for w in expect.get("forbid", []))
        return "correct" if ok else "wrong"
    if kind == "top":
        value = str(expect["value"])
        ties = [str(t) for t in expect.get("ties", [])]
        if ties:
            # a tie for the top place: every value that shares it must be among the leading rows, in any order; an answer that shows one of them hides the tie
            wanted = [value] + ties
            lead = flat[:len(wanted)] if flat and isinstance(flat[0], dict) else None
            if lead is not None:
                shown = " ".join(" ".join(str(c) for c in row.values()) for row in lead)
                return "correct" if all(w in shown for w in wanted) else "wrong"
            return "correct" if all(w in blob for w in wanted) else "wrong"
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


def load_private(path):
    """The private bank (plates read from the real footage), or empty when it is not on this machine."""
    if path and os.path.exists(path):
        data = json.load(open(path, encoding="utf-8"))
        return data.get("values") or {}, data.get("questions") or []
    return {}, []


def known_plates(private_values, typed):
    plates = {str(v) for values in private_values.values() for v in values if not str(v).lower().endswith(VIDEO_SUFFIXES)}
    for q in typed:
        plates.add(q["question"].rsplit(" ", 1)[1].rstrip("?"))
    return plates


def masked(text, plates):
    """The text with every plate of the private bank and every plate-like string replaced."""
    text = text or ""
    for p in sorted(plates, key=len, reverse=True):
        text = text.replace(p, "<PLATE>")
    return vqb.mask(text)


def main(argv):
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("bank")
    ap.add_argument("results", nargs="+", help="one or more result files of ask_case.py; the latest line of a question wins, later files after earlier ones")
    ap.add_argument("--out", required=True)
    ap.add_argument("--notes", default="")
    ap.add_argument("--private-bank", default=PRIVATE_BANK)
    ap.add_argument("--run-note", default="", help="a sentence for the head of the sheet: which questions were run when, on which engine")
    args = ap.parse_args(argv)
    bank = json.load(open(args.bank, encoding="utf-8"))
    results = {}
    for path in args.results:
        results.update(load_results(path))
    private_values, typed = load_private(args.private_bank)
    plates = known_plates(private_values, typed)
    notes = json.load(open(args.notes, encoding="utf-8")) if args.notes and os.path.exists(args.notes) else {}
    questions = []
    for q in bank["questions"] + typed:
        item = dict(q)
        item["expected_text"] = render(q["expect"])
        item["kind_of_check"] = {"scalar": "a number", "text": "a time or a day", "set": "a list of values", "top": "the first of a ranking", "video": "what the stored readings of the clips say", "files": "exactly these recordings", "words": "words the answer must contain"}.get(q["expect"]["kind"], "")
        r = results.get(q["id"])
        if r:
            v = verdict(item, r, private_values)
            row_text = ""
            rows = r.get("rows")
            if isinstance(rows, list) and rows and isinstance(rows[0], dict):
                row_text = " | first row: " + "; ".join("%s %s" % (k, val) for k, val in list(rows[0].items())[:6])
            item["live"] = {"state": r.get("state"), "answer": ((r.get("answer") or r.get("older_path_answer") or "") + row_text)[:420], "seconds": round((r.get("total_ms") or 0) / 1000, 1), "verdict": v,
                            "sql": (r.get("sql") or "")[:600], "views": r.get("views") or []}
        if q["id"] in notes:
            item["review_note"] = notes[q["id"]]
        item.pop("bank", None)
        if q["area"] in VIDEO_AREAS:      # real footage: no plate string leaves this function
            for field in ("question", "expected_text", "note"):
                item[field] = masked(item.get(field), plates)
            if item.get("live"):
                item["live"]["answer"] = masked(item["live"]["answer"], plates)
                item["live"]["sql"] = masked(item["live"]["sql"], plates)
        questions.append(item)
    pack = {"collection": bank.get("collection"), "areas": [{"key": k, "title": t, "exercises": e, "where": w} for k, t, e, w in AREAS], "questions": questions}
    if any(q["area"] in VIDEO_AREAS for q in questions):
        pack["scope"] = "the Orion Lahore reference case (synthetic records, and five real video clips whose plates are masked)"
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
    has_video = any(q["area"] in VIDEO_AREAS for q in questions)
    lines = ["# Verified question bank: the Orion Lahore reference case", "",
             "%d questions, each with the answer computed from the stored rows or readings (never through the engine) and the live engine's answer scored against it. %s" % (len(questions),
                 "The records are synthetic; the five video clips are real footage of the owner's, so the plates read from them are masked here and kept in a private file outside the repository. A reading is a model's candidate, never an identification." if has_video else "Synthetic data only."), ""]
    if args.run_note:
        lines += [args.run_note, ""]
    lines += ["| Family | Questions | Correct | Wrong | Right number, wrong view | Declined or abstained | Not run |", "|---|---|---|---|---|---|---|"]
    total = {"correct": 0, "wrong": 0, "lucky": 0, "other": 0, "none": 0}
    for key, title, _, _ in AREAS:
        c = counts.get(key, {})
        n = sum(c.values())
        if not n:
            continue
        other = sum(v for k, v in c.items() if k not in ("correct", "wrong", "lucky", "not run"))
        lines.append("| %s | %d | %d | %d | %d | %d | %d |" % (title, n, c.get("correct", 0), c.get("wrong", 0), c.get("lucky", 0), other, c.get("not run", 0)))
        total["correct"] += c.get("correct", 0)
        total["wrong"] += c.get("wrong", 0)
        total["lucky"] += c.get("lucky", 0)
        total["other"] += other
        total["none"] += c.get("not run", 0)
    lines += ["| **All** | **%d** | **%d** | **%d** | **%d** | **%d** | **%d** |" % (len(questions), total["correct"], total["wrong"], total["lucky"], total["other"], total["none"]), ""]
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
    written = [args.out + ext for ext in (".json", ".csv", ".md", ".html")]
    written_text = []
    for path in written:
        with open(path, encoding="utf-8") as handle:
            written_text.append(handle.read())
    leaked = sorted(p for p in plates if any(p in text for text in written_text))
    if leaked:
        for path in written:
            os.remove(path)
        raise SystemExit("%d plate(s) of the private bank occur in the sheet; the sheet was removed" % len(leaked))
    timed_out = sum(1 for q in questions if (q.get("live") or {}).get("verdict") == "timed out")
    print("%d questions: correct %d, wrong %d, right number but wrong view %d, declined or abstained %d (of which timed out %d), not run %d -> %s.{json,csv,md,html}" % (len(questions), total["correct"], total["wrong"], total["lucky"], total["other"], timed_out, total["none"], args.out))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
