#!/usr/bin/env python3
"""Builds the sheet of video questions that can be asked of a case and the answers each one is known to give, from an answer key and the results of a live run.

    python -I -X utf8 tools/video_plates/build_video_sheet.py KEY.json RESULTS.jsonl PUBLIC.md PRIVATE.md [--title "..."]

KEY.json is a key of tools/video_plates/video_question_bank.py (computed from the stored readings, not through the analyst views); RESULTS.jsonl is the output of its `run` against the live engine. Only
the questions that the live engine answered CORRECTLY (verdict correct, through the view the question calls for) are listed. PUBLIC.md carries no plate text: where the answer is a plate, it says so. PRIVATE.md
holds the plates read from the footage and stays on this machine (never commit it, never send it anywhere). The sheet says what was verified and when.
"""
import json
import re
import sys
import time

PLATE = re.compile(r"\b(?=[A-Z0-9]*\d)(?=[A-Z0-9]*[A-Z])[A-Z0-9]{4,10}\b")


def fmt(value):
    if isinstance(value, float):
        return ("%.2f" % value).rstrip("0").rstrip(".")
    return str(value)


def is_file(text):
    return bool(re.search(r"\.[A-Za-z0-9]{2,5}$", str(text)))


def main(argv):
    key = json.load(open(argv[0], encoding="utf-8"))
    results = {}
    for line in open(argv[1], encoding="utf-8"):
        r = json.loads(line)
        results[r["id"]] = r
    title = "Questions about the videos of the case"
    if "--title" in argv:
        title = argv[argv.index("--title") + 1]
    stamp = time.strftime("%Y-%m-%d")
    groups = {"One video": [], "Across the videos": [], "Speech and text read from the videos": []}
    private = []
    for q in key["questions"]:
        r = results.get(q["id"])
        if not r or r["verdict"] != "correct":
            continue
        kind, exp = q["kind"], q["expected"]
        if kind == "number":
            shown = fmt(exp)
        elif kind in ("set", "top1", "none"):
            items = list(exp)
            if not items:
                shown = "none"
            elif all(is_file(x) for x in items):
                shown = ", ".join(items)
            else:
                shown = "%d plate%s (read from the footage: see the private sheet)" % (len(items), "" if len(items) == 1 else "s")
                private.append((q["q"], items))
        else:
            continue
        if q["id"].startswith(("speech-", "segments-", "ocr-", "regions-")):
            group = "Speech and text read from the videos"
        elif any(q["id"].startswith(p) for p in ("most-", "plates-", "total-", "with-", "longest-video", "all-", "fewest", "videos-", "avg-")):
            group = "Across the videos"
        else:
            group = "One video"
        groups[group].append((q["q"], shown, round(r["seconds"] or 0)))
    out = ["# " + title, "", "Collection `%s`. Every question below was put to the live engine on %s in these words and answered correctly: the answer matches a key computed from the stored readings by a query that does not use the analyst views." % (key["collection"], stamp),
           "The readings are the models' (plates, speech, text): they are candidates that a person must check, never an identification. Times are seconds from the start of the video.", ""]
    for name, rows in groups.items():
        if not rows:
            continue
        out += ["## " + name, "", "| Question | The answer it gives | Seconds to answer |", "|---|---|---|"]
        out += ["| %s | %s | %d |" % (q.replace("|", "/"), a.replace("|", "/"), s) for q, a, s in rows]
        out.append("")
    open(argv[2], "w", encoding="utf-8", newline="\n").write("\n".join(out) + "\n")
    priv = ["# PRIVATE: plates read from the footage (this machine only; do not commit or send)", "", "Collection `%s`, %s." % (key["collection"], stamp), ""]
    for q, items in private:
        priv += ["**%s**" % q, "", "- " + "\n- ".join(items), ""]
    open(argv[3], "w", encoding="utf-8", newline="\n").write("\n".join(priv) + "\n")
    print("%d questions listed, %d with plates in the private sheet" % (sum(len(v) for v in groups.values()), len(private)))


if __name__ == "__main__":
    main(sys.argv[1:])
