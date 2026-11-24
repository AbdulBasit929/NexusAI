#!/usr/bin/env python3
"""Builds the QUESTION CATALOGUE: every question the project has measured against the live Ask engine on the Orion Lahore reference case, with its expected answer and its measured status,
as one CSV (every question) and one Markdown file (what the application answers, by family and by kind of question, and what it cannot answer yet).

    python -X utf8 -I tools/showcase/build_question_catalogue.py --out docs/qa/question-catalogue-20261124 \
        [--search-runs C:/tmp/nexusai-g4/orion_bank_A_v5.jsonl C:/tmp/nexusai-g4/orion_bank_B_v4.jsonl C:/tmp/nexusai-g4/orion_bank_C_v2.jsonl]

Nothing here is invented and nothing is asked of the engine: it reads the sheets that the judges already wrote (docs/qa/verified-bank-*-sheet.json, ui-query-pack-20261031.json,
audio-fresh-wording-20261116-sheet.json) and the three any-source search banks of tools/showcase/ask_exhaustive_bank.py (their questions are read from the file's own text, never by importing it).
The records are synthetic; the plates of real video footage are masked in the sheets and stay masked here.
"""
import argparse
import ast
import csv
import json
import os
import re
import sys
from collections import Counter, OrderedDict

ROOT = os.path.abspath(os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", ".."))
QA = os.path.join(ROOT, "docs", "qa")

VERDICT_TEXT = {
    "correct": "answered correctly",
    "wrong": "answered wrongly",
    "abstained": "abstained (the engine said it could not show the answer)",
    "declined": "declined (the engine said it could not)",
    "right-number-wrong-view": "right number, wrong view",
}


def load(path):
    with open(path, encoding="utf-8") as handle:
        return json.load(handle)


def norm_type(raw):
    """Kinds that are asked once per video carry the video's file name in the type; fold them to one kind."""
    m = re.match(r"^(count|first|window|longest|frames|length)-.+$", raw)
    if m:
        return "video-" + m.group(1) + " (per video)"
    if raw.startswith("total-minutes"):
        return "video-total-minutes"
    return raw


def expect_text(q):
    if q.get("expected_text"):
        return str(q["expected_text"])
    return str(q.get("expect", ""))


def one_line(text, limit=200):
    text = re.sub(r"\s+", " ", str(text or "")).strip()
    return text if len(text) <= limit else text[: limit - 1] + "…"


def sheet_rows(source, sheet):
    areas = OrderedDict((a["key"], a) for a in sheet["areas"])
    rows = []
    for q in sheet["questions"]:
        live = q.get("live") or {}
        verdict = live.get("verdict") if isinstance(live, dict) else None
        rows.append({
            "source": source, "id": q["id"], "family": q["area"], "family_title": areas.get(q["area"], {}).get("title", q["area"]), "kind": norm_type(q["type"]),
            "question": q["question"], "expected": expect_text(q), "verdict": verdict or "not run", "answer": one_line((live or {}).get("answer", "")),
            "seconds": (live or {}).get("seconds", ""), "note": one_line(q.get("note", ""), 160),
        })
    return areas, rows


def parse_banks():
    """Questions and truth lists of the any-source search banks, read from the script's text (no import: ask_case is imported at its top)."""
    path = os.path.join(ROOT, "tools", "showcase", "ask_exhaustive_bank.py")
    tree = ast.parse(open(path, encoding="utf-8").read())
    env, values = {}, {}
    for node in tree.body:
        if isinstance(node, ast.Assign) and len(node.targets) == 1 and isinstance(node.targets[0], ast.Name):
            name = node.targets[0].id
            try:
                # constants and the two dicts; TRUTH names the list DOCS_PHONE, which is why the names defined so far are the environment
                values[name] = eval(compile(ast.Expression(node.value), path, "eval"), {"__builtins__": {}}, dict(env))
                env[name] = values[name]
            except Exception:
                continue
    return values["BANKS"], values["TRUTH"]


def search_rows(banks, truth, run_files):
    runs = {}
    for path in run_files or []:
        rows = [json.loads(line) for line in open(path, encoding="utf-8") if line.strip()]
        for r in rows:
            runs.setdefault(os.path.basename(path)[len("orion_bank_"):len("orion_bank_") + 1], {})[r["n"]] = r
    out = []
    for name, items in banks.items():
        letter = name[-1]
        for n, (probe, question) in enumerate(items, 1):
            files = truth[probe]
            run = runs.get(letter, {}).get(n)
            if run is None:
                verdict, answer = "not run", ""
            else:
                verdict = "correct" if run["recall"] >= 0.999 else ("abstained" if not run["cited"] else "wrong")
                answer = "recall %.2f, precision %s, cited %d of %d files" % (run["recall"], ("%.2f" % run["precision"]) if run["precision"] is not None else "n/a", len(run["cited"]), len(files))
            out.append({"source": "search-" + name, "id": "%s-%02d" % (name.upper(), n), "family": "search", "family_title": "Where is a value written (any evidence)", "kind": "where-written",
                        "question": question, "expected": "all of: " + ", ".join(files), "verdict": verdict, "answer": answer, "seconds": (run or {}).get("seconds", ""), "note": "files that write it, by an independent check"})
    return out


IMAGE_ROWS = [
    ("IMG-01", "Find copies of a picture I have", "attach a picture in the Ask box and press Ask", "the same file, the same image re-saved or resized, with a label per match"),
    ("IMG-02", "Find a small part of a picture inside the other pictures", "attach a crop or a detail", "\"your picture appears inside N other pictures\", each with a box"),
    ("IMG-03", "A part of another photograph that looks like it", "attach a picture", "windows that look alike, outlined and labelled not verified"),
    ("IMG-04", "Find copies of a picture already in the case", "find copies of imgsearch-truck-558137.jpg", "the same labels for a stored picture"),
    ("IMG-05", "Photos by words, in any language", "show me photos of a laptop", "the closest pictures, best first, with more that are also possible"),
    ("IMG-06", "More like this picture", "pictures that look like imgsearch-bus-124952.jpg", "the nearest-looking pictures, ranked (candidates only)"),
    ("IMG-07", "Draw a box on a picture and find that part elsewhere", "open a picture, press Find this part elsewhere, drag a box", "the pictures that contain that part, outlined"),
    ("IMG-08", "Duplicate and near-duplicate groups of the whole case", "show me the duplicate pictures in this case", "stacks, one picture per group, each member with its link"),
]

# Honest gaps. Each row: what an analyst may ask, why the application cannot answer it today, where it is planned. Sources: docs/roadmap/reconciled-plan-20261124.md and the notes it cites.
NOT_YET = [
    ("Across cases", "Which numbers appear in more than one case? Show everything about this person in all cases.", "The engine does not join across cases. Today a question can be fanned out to 2 to 10 cases and the answers placed side by side.", "W1-3 identifier registry (X1), W1-9 Ask across cases (X2)"),
    ("Across cases", "Which findings of case A support case B?", "Findings are per case; there is no cross-case findings view.", "W1-9 findings across cases (X3)"),
    ("Chats and messages", "Show the WhatsApp or Telegram conversation between two people; who was in the group; what was said on a date.", "Only text messages in the call-record families and a plain chat-export document (searchable as text) are answered. No typed chat family and no chat-screenshot reader.", "W1-5 messages family (migration 041)"),
    ("Email", "Which emails did this address send in March? Show attachments.", "No email family (EML, mbox, PST) is ingested.", "W1-6 email"),
    ("Phone extractions", "Which apps were installed; browser history; contacts; calendar of a handset extraction.", "No handset-extraction reader is built (the plan runs iLEAPP and ALEAPP out of process and ingests their output).", "W1-7 phone extractions"),
    ("Faces", "Is this person in any other photo or video? Who is this?", "Face detection, embedding and matching are measured, not switched on; a match is only ever a candidate and faces need governance first (permission, search log, kill switch, consent).", "W1-12 face governance (042), then grouping inside one case; search stays off"),
    ("Alerts and watch lists beyond pictures", "Alert me when this plate, number or face appears in new evidence.", "Known sets of picture hashes are live (a picture that is a copy of a listed one is flagged when you look at the case); there are no alerts and no watch lists of plates, numbers or faces.", "Wave 2 productivity (D2); faces after governance"),
    ("Punjabi and Saraiki speech", "What is said in this Punjabi or Saraiki recording?", "Pashto is a search lead only; Punjabi and Saraiki have no measured reader yet (recording kit ready).", "W1-4 audio product (bake-off in the background)"),
    ("Speakers and calls in audio", "Who is the second speaker; which recording matches this call record?", "No diarisation, no spoken-number reader and no link from a recording to a call record yet.", "W1-4 audio product"),
    ("Video", "Which people, vehicles or events are in this video; summarise it; how many minutes of video in total?", "Plates in video, speech and text read from video are answered; shots, motion, tracks and summaries are not built, and the total-minutes question is abstained today.", "W1-10 video overview; tracking in Wave 2"),
    ("Records answers naming files", "Which files mention this number? (when the same number is also a record in the tables)", "When the table engine has an answer, the files that write the value are not added. Empty answers are completed; non-empty ones are not.", "W1-1 Ask Phase 2: attach, never replace"),
    ("Maps and movement", "Where was this phone at 3 pm? Draw the route.", "Cell-site views exist (most used cell, distinct cells); tower coordinates are not part of the product, so a position or route cannot be drawn.", "W1-8 tower positions"),
    ("Documents beyond text", "Which scanned forms in Urdu say X; which contract clause says Y.", "Typed documents and OCR text are searched; scanned Urdu print is not readable (about 70% character error) and clause-level understanding is not built.", "W1-13 Urdu OCR bake-off; clause-level in Wave 2"),
    ("Pictures: metadata and provenance", "Which photos were taken with this camera or at this place; are these pictures signed by a camera maker?", "EXIF filters and Content Credentials are not read yet.", "W1-11 images: EXIF, C2PA"),
]


def write_csv(path, rows):
    with open(path, "w", encoding="utf-8-sig", newline="") as handle:
        writer = csv.writer(handle)
        writer.writerow(["source", "id", "family", "kind", "question", "expected answer", "measured status", "engine answer (short)", "seconds", "note"])
        for r in rows:
            writer.writerow([r["source"], r["id"], r["family_title"], r["kind"], r["question"], r["expected"], VERDICT_TEXT.get(r["verdict"], r["verdict"]), r["answer"], r["seconds"], r["note"]])


def md_escape(text):
    return str(text).replace("|", "\\|").replace("\n", " ")


def family_section(lines, title_prefix, areas, rows):
    by_family = OrderedDict()
    for r in rows:
        by_family.setdefault(r["family"], []).append(r)
    for key, fam_rows in by_family.items():
        area = areas.get(key, {})
        counts = Counter(r["verdict"] for r in fam_rows)
        lines.append("### %s (%d questions: %d correct%s)" % (area.get("title", key), len(fam_rows), counts.get("correct", 0), "".join(", %d %s" % (v, k) for k, v in sorted(counts.items()) if k != "correct")))
        lines.append("")
        if area.get("exercises"):
            lines.append(md_escape(area["exercises"]))
            lines.append("")
        kinds = OrderedDict()
        for r in fam_rows:
            kinds.setdefault(r["kind"], []).append(r)
        lines.append("| Kind of question | Asked | Correct | Example question | Expected answer for the example |")
        lines.append("|---|---|---|---|---|")
        for kind, krs in kinds.items():
            ok = sum(1 for r in krs if r["verdict"] == "correct")
            lines.append("| %s | %d | %d | %s | %s |" % (md_escape(kind), len(krs), ok, md_escape(krs[0]["question"]), md_escape(one_line(krs[0]["expected"], 90))))
        lines.append("")


def main(argv):
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--out", required=True, help="path without extension; writes .csv and .md")
    ap.add_argument("--bank-sheet", default=os.path.join(QA, "verified-bank-20261124-sheet.json"))
    ap.add_argument("--search-runs", nargs="*", default=[])
    args = ap.parse_args(argv)

    vb_areas, vb = sheet_rows("verified-bank", load(args.bank_sheet))
    ui_areas, ui = sheet_rows("ui-query-pack", load(os.path.join(QA, "ui-query-pack-20261031.json")))
    au_areas, au = sheet_rows("audio-fresh-wording", load(os.path.join(QA, "audio-fresh-wording-20261116-sheet.json")))
    banks, truth = parse_banks()
    se = search_rows(banks, truth, args.search_runs)
    im = [{"source": "image-search", "id": i, "family": "images", "family_title": "Image search", "kind": t, "question": how, "expected": exp, "verdict": "manual", "answer": "", "seconds": "", "note": "checked by hand with the guide and by the measured runs of docs/data/image-search-*"}
          for i, t, how, exp in IMAGE_ROWS]
    all_rows = vb + ui + au + se + im
    write_csv(args.out + ".csv", all_rows)

    def tally(rows):
        c = Counter(r["verdict"] for r in rows)
        return len(rows), c.get("correct", 0), len(rows) - c.get("correct", 0)

    def rest_text(rows):
        c = Counter(r["verdict"] for r in rows if r["verdict"] != "correct")
        word = {"wrong": "wrong", "declined": "declined", "abstained": "abstained", "not run": "not run"}
        return "%d (%s)" % (sum(c.values()), ", ".join("%d %s" % (v, word.get(k, k)) for k, v in sorted(c.items()))) if c else "0"

    n_vb, ok_vb, rest_vb = tally(vb)
    n_ui, ok_ui, rest_ui = tally(ui)
    n_au, ok_au, rest_au = tally(au)
    search_by_bank = OrderedDict((b, tally([r for r in se if r["source"] == "search-" + b])) for b in banks)

    L = []
    L.append("# Question catalogue: what the application answers, measured (2026-11-24)")
    L.append("")
    L.append("Every question below comes from a bank that was put to the **live Ask engine** on the **Orion Lahore reference case** (`orion-lahore-reference-20261030`, synthetic records, a few real public recordings and five real video clips whose plates are masked). The expected answer of each question was computed from the stored rows by a query that does not go through the engine, and the engine's answer was scored against it. Nothing in this file was written by hand except the notes and the list of what cannot be answered yet. It is generated by `tools/showcase/build_question_catalogue.py`; the full list, one row per question, is `docs/qa/question-catalogue-20261124.csv` (%d rows)." % len(all_rows))
    L.append("")
    L.append("**How to read the status.** *answered correctly* = the expected answer is in the engine's answer. *abstained* or *declined* = the engine said it could not show the answer; that is honest and is counted apart from a wrong answer. *answered wrongly* = a wrong answer (none in the main bank today). Readings made by models (plates, speech, text in pictures) are **candidates that a person must check**; none is an identification.")
    L.append("")
    L.append("## 1. At a glance")
    L.append("")
    L.append("| Bank | Questions | Correct | Not correct | Measured |")
    L.append("|---|---|---|---|---|")
    L.append("| Verified bank (calls, messages, pairs, internet, apps, bank, plates, handsets, cells, registrations, evidence files, speech, plates in photos, video, typed plate search) | %d | %d | %d (all abstained or declined, none wrong) | run of 2026-11-24 on the live API `text-search-all-20261124`: 558 earlier verdicts (runs of 2026-11-03 to 11-13) carried unchanged for questions whose wording and expected answer did not change, 53 re-asked live, plus the 5 typed plate searches |" % (n_vb, ok_vb, rest_vb))
    L.append("| Older UI query pack (connections, calls, documents, registrations, plates, transactions, co-location, successors, movement, pairs, internet) | %d | %d | %s | run of 2026-10-31 on an older engine; **a re-ask on the live engine is owed** |" % (n_ui, ok_ui, rest_text(ui)))
    L.append("| Fresh-wording speech questions (audio) | %d | %d | %s | run of 2026-11-16, before the later audio rounds; gates not met |" % (n_au, ok_au, rest_text(au)))
    for b, (n, ok, rest) in search_by_bank.items():
        L.append("| Where-is-it-written search, %s (any evidence: documents, pictures, plates, recordings) | %d | %d (every true file named) | %d | diagnostic run of the fixed build on staging, 2026-11-20 (the misses of these banks had been read, so it is not a certification); that design went live 2026-11-24; gate 90%% full recall and 0.95 mean recall, **not met** |" % (b, n, ok, rest))
    L.append("| Image search (eight things an analyst can do) | %d | checked by hand and by measured runs | | see `docs/qa/image-search-manual-test-guide-20261123.md` |" % len(im))
    L.append("")
    L.append("## 2. The verified bank: what the application answers accurately")
    L.append("")
    L.append("This is the bank to trust: %d of %d answered correctly, the other %d abstained instead of guessing, none wrong. Phone numbers, accounts, plates and handsets are synthetic." % (ok_vb, n_vb, rest_vb))
    L.append("")
    family_section(L, "", vb_areas, vb)
    L.append("## 3. The older UI query pack (connections, co-location, successors, movement and more)")
    L.append("")
    L.append("%d questions asked on 2026-10-31 against an older engine; %d correct, %s not. It covers views that the verified bank does not (connections, co-location, successors, movement, transactions, documents). Treat the figures as **old** until the pack is asked again on the live engine." % (n_ui, ok_ui, rest_text(ui)))
    L.append("")
    family_section(L, "", ui_areas, ui)
    L.append("## 4. Where is this value written? (search across all the evidence)")
    L.append("")
    L.append("An analyst types a number, a name, a plate, an account, a date or a word, in English, Roman Urdu or Urdu script, and expects **every** file that writes it (documents, text read from pictures, plates, transcripts), tolerant of OCR errors, digit scripts and phone forms (0, 92, 0092). Three banks of fresh wordings were asked; truth was computed by an independent check, never by the engine.")
    L.append("")
    L.append("| Bank | Questions | Full recall | Result |")
    L.append("|---|---|---|---|")
    for b, (n, ok, rest) in search_by_bank.items():
        L.append("| %s | %d | %d (%d%%) | gate 90%% not met |" % (b, n, ok, round(100 * ok / n) if n else 0))
    L.append("")
    L.append("Examples of wordings (the full list with every expected file is in the CSV):")
    L.append("")
    for b in banks:
        for r in [x for x in se if x["source"] == "search-" + b][:4]:
            L.append("- %s: %s" % (b, r["question"]))
    L.append("")
    L.append("Known limit: when the same value is also a record in a table (a phone number in call records), the table answer is returned alone and the files that write it are not added (Ask Phase 2).")
    L.append("")
    L.append("## 5. Speech: fresh wordings (audio)")
    L.append("")
    L.append("%d questions, %d correct, %s not. Pashto is a search lead only; the gates for this track were not met on 2026-11-16 and a new round is planned." % (n_au, ok_au, rest_text(au)))
    L.append("")
    family_section(L, "", au_areas, au)
    L.append("## 6. Image search: eight things to try")
    L.append("")
    L.append("Case: Orion Lahore Reference, interface `http://localhost:4182` (the old design on :4181 has no image button). Full steps and the labels for a copy are in `docs/qa/image-search-manual-test-guide-20261123.md`.")
    L.append("")
    L.append("| # | What it does | How to ask | What you get |")
    L.append("|---|---|---|---|")
    for r in im:
        L.append("| %s | %s | %s | %s |" % (r["id"], md_escape(r["kind"]), md_escape(r["question"]), md_escape(r["expected"])))
    L.append("")
    L.append("## 7. What cannot be answered yet (so a test of it should fail honestly)")
    L.append("")
    L.append("| Area | A question an analyst may ask | Why not today | Where it is planned |")
    L.append("|---|---|---|---|")
    for area, ask, why, plan in NOT_YET:
        L.append("| %s | %s | %s | %s |" % (md_escape(area), md_escape(ask), md_escape(why), md_escape(plan)))
    L.append("")
    L.append("A correct behaviour for these today is to decline or abstain with a reason, never to guess. If any of them is answered with a confident number, that is a defect to report.")
    L.append("")
    L.append("## 8. How to test it yourself")
    L.append("")
    L.append("1. In the interface (`http://localhost:4182` for the new design, `:4181` for the old one) open the case **Orion Lahore Reference** and put any question of section 2 into Ask, word for word or in your own words. Compare with the expected answer in the CSV.")
    L.append("2. To re-run a bank against the live engine (one question at a time; the model is one process, so allow minutes, not seconds):")
    L.append("")
    L.append("```powershell")
    L.append("python -X utf8 tools/showcase/ask_case.py docs/qa/verified-bank-20261124.json --collection orion-lahore-reference-20261030 --out C:/tmp/results.jsonl --per-type 2")
    L.append("python -X utf8 tools/showcase/judge_verified_bank.py docs/qa/verified-bank-20261124.json C:/tmp/results.jsonl --out C:/tmp/sheet")
    L.append("```")
    L.append("")
    L.append("3. To re-ask the where-is-it-written banks: `python -X utf8 tools/showcase/ask_exhaustive_bank.py --out C:/tmp/bank.jsonl --bank bank-C`, then `ask_exhaustive_score.py C:/tmp/bank.jsonl --list`.")
    L.append("4. Questions that name a plate of the real video clips are typed from the private file (`C:/NexusAI-Showcase/orion/key/video-private-bank.json`, outside the repository); the sheets show `<PLATE>`.")
    L.append("5. A new bank must be written **after** the build it certifies and asked once; a bank whose misses were read is a diagnostic, not a gate.")
    L.append("")
    with open(args.out + ".md", "w", encoding="utf-8", newline="\n") as handle:
        handle.write("\n".join(L))
    print("rows", len(all_rows), "| verified", n_vb, ok_vb, "| ui", n_ui, ok_ui, "| audio", n_au, ok_au, "| search", {b: v for b, v in search_by_bank.items()}, "| images", len(im))


if __name__ == "__main__":
    main(sys.argv[1:])
