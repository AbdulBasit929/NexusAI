#!/usr/bin/env python3
"""Free-form video questions with answer keys computed straight from the stored readings, and a judge for the live Ask engine.

    python -I -X utf8 tools/video_plates/video_question_bank.py build  COLLECTION KEY.json [--set dev|sealed]
    python -I -X utf8 tools/video_plates/video_question_bank.py run    KEY.json RESULTS.jsonl [--only ID,ID] [--url http://127.0.0.1:8091]
    python -I -X utf8 tools/video_plates/video_question_bank.py report RESULTS.jsonl

`build` asks the database (read-only, through `docker exec ... psql`) for the plate groups, the frame reads, the video files, the speech segments and the text regions of the collection and writes, for
each question, the wording and the expected answer. The key is computed from `forensic.derived_artifacts` and `forensic.evidence_items` directly, NOT through the analyst views the engine reads, so the
two can disagree. The key holds the plate strings of the footage: it is written where the caller says (keep it out of the repository for real footage) and never printed.
`run` puts every question to the running API (/query/hybrid) one at a time and judges the answer against the key; the results file keeps the verdict, the engine's state and the answer and the query
with plate-like strings masked. `report` prints the counts.
"""
import argparse
import json
import os
import re
import subprocess
import sys
import time

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.join(HERE, "..", "showcase"))

PLATE = re.compile(r"\b(?=[A-Z0-9]*\d)(?=[A-Z0-9]*[A-Z])[A-Z0-9]{5,9}\b")


def mask(text):
    return PLATE.sub("<PLATE>", text or "")


def psql_rows(collection, sql):
    wrapped = "select coalesce(json_agg(q), '[]'::json) from (%s) q" % sql.replace("{C}", collection.replace("'", "''"))
    out = subprocess.run(["docker", "exec", "nexusai-forensic-postgres-1", "psql", "-U", "localrecall", "-d", "localrecall", "-At", "-c", wrapped], capture_output=True, text=True, check=True).stdout
    return json.loads(out.strip() or "[]")


GROUPS = """select e.original_filename as f, d.metadata->'observation'->>'normalized_plate_text' as plate, (d.metadata->'observation'->>'first_seen_seconds')::numeric as first,
 (d.metadata->'observation'->>'last_seen_seconds')::numeric as last, (d.metadata->'observation'->>'sightings_count')::int as n
 from forensic.derived_artifacts d join forensic.evidence_items e on e.evidence_id = d.evidence_id
 where d.collection_id = '{C}' and d.artifact_type = 'forensics.video-anpr-plate-group/v1' and d.processing_status = 'completed'"""
FRAMES = """select e.original_filename as f, (d.citation_locator->>'frame_number')::bigint as fn, (d.citation_locator->>'timestamp_seconds')::numeric as t,
 d.metadata->'observation'->>'normalized_plate_text' as plate, (d.metadata->'observation'->>'ocr_confidence')::numeric as conf
 from forensic.derived_artifacts d join forensic.evidence_items e on e.evidence_id = d.evidence_id
 where d.collection_id = '{C}' and d.artifact_type = 'forensics.anpr-observation/v1' and d.processing_status = 'completed' and e.modality = 'video'"""
FILES = """select e.original_filename as f, e.size_bytes as size,
 round(((jsonb_path_query_first(e.media_metadata, '$.tracks[*] ? (@.handler_type == "vide")') ->> 'duration_ms')::numeric) / 1000, 2) as dur,
 (jsonb_path_query_first(e.media_metadata, '$.tracks[*] ? (@.handler_type == "vide")') ->> 'height_pixels')::int as h,
 jsonb_path_exists(e.media_metadata, '$.tracks[*] ? (@.handler_type == "soun")') as audio
 from forensic.evidence_items e where e.collection_id = '{C}' and e.modality = 'video'"""
SPEECH = """select e.original_filename as f, (d.citation_locator->>'start_seconds')::numeric as s, (d.citation_locator->>'end_seconds')::numeric as e, d.metadata->'observation'->>'text' as text
 from forensic.derived_artifacts d join forensic.evidence_items e on e.evidence_id = d.evidence_id
 where d.collection_id = '{C}' and d.artifact_type = 'forensics.audio-timestamp-segment/v1' and d.processing_status = 'completed'"""
OCR = """select e.original_filename as f, (d.citation_locator->>'timestamp_seconds')::numeric as t, d.metadata->'observation'->>'raw_text' as text
 from forensic.derived_artifacts d join forensic.evidence_items e on e.evidence_id = d.evidence_id
 where d.collection_id = '{C}' and d.artifact_type = 'forensics.image-ocr-observation/v1' and d.processing_status = 'completed'"""


def mmss(seconds):
    return "%d:%02d" % (int(seconds) // 60, int(seconds) % 60)


def build(collection, which):
    groups, frames, files, speech, ocr = (psql_rows(collection, q) for q in (GROUPS, FRAMES, FILES, SPEECH, OCR))
    names = sorted({g["f"] for g in groups})
    by_file = lambda rows, f: [r for r in rows if r["f"] == f]  # noqa: E731
    qs = []

    def view_for(qid):
        """The view a question calls for; an answer that matches the key through another view is reported as lucky, not correct."""
        if qid.startswith("frames-"):
            return "v_video_plate_frames"
        if qid.startswith(("length-", "total-", "with-audio", "longest-video")):
            return "v_video_files"
        if qid.startswith("speech-"):
            return "v_speech_segments"
        if qid.startswith("ocr-"):
            return "v_ocr_regions|v_ocr_evidence_summary"
        return "v_plate_reads"

    def add(qid, text, kind, expected, **extra):
        qs.append(dict(id=qid, q=text, kind=kind, expected=expected, expect_view=view_for(qid), **extra))

    # (kind) number: expected is a number; set: a list of strings; top1: a list of acceptable first labels; none: nothing is expected
    wording = {
        "dev": {
            "count": "How many different number plates were read in {f}?",
            "first": "At what second of {f} does the first plate appear?",
            "window": "Which plates were read between {a} and {b} of {f}?",
            "longest": "Which plate stays in view the longest in {f}?",
            "frames": "On how many frames of {f} was a plate read?",
            "length": "How long is the video {f}?",
        },
        "sealed": {
            "count": "How many distinct plates can be made out in {f}?",
            "first": "When in {f} is a plate first picked up? Give the second.",
            "window": "List the plates that show up from {a} to {b} in {f}.",
            "longest": "In {f}, which plate is visible for the greatest length of time?",
            "frames": "How many frames in {f} have a plate reading?",
            "length": "What is the duration of {f} in seconds?",
        },
    }[which]
    for f in names:
        gs, fr = by_file(groups, f), by_file(frames, f)
        key = re.sub(r"[^a-z0-9]+", "-", f.lower()).strip("-")
        add("count-" + key, wording["count"].format(f=f), "number", len({g["plate"] for g in gs}))
        add("first-" + key, wording["first"].format(f=f), "number", min(float(g["first"]) for g in gs), tolerance=0.15)
        dur = next((float(x["dur"]) for x in files if x["f"] == f and x["dur"] is not None), None)
        if dur and dur >= 20:
            hit = sorted({g["plate"] for g in gs if float(g["first"]) <= 20 and float(g["last"]) >= 10})
            add("window-" + key, wording["window"].format(f=f, a=mmss(10), b=mmss(20)), "set", hit, what="plates")
        spans = [(float(g["last"]) - float(g["first"]), g["plate"]) for g in gs]
        top = max(s for s, _ in spans)
        add("longest-" + key, wording["longest"].format(f=f), "top1", sorted({p for s, p in spans if abs(s - top) < 1e-6}))
        add("frames-" + key, wording["frames"].format(f=f), "number", len({x["fn"] for x in fr}))
        add("length-" + key, wording["length"].format(f=f), "number", dur, tolerance=0.06)
    # across videos
    distinct = {f: len({g["plate"] for g in by_file(groups, f)}) for f in names}
    top = max(distinct.values())
    cross = {
        "dev": ("Which video shows the most different plates?", "Which plates were read in more than one video?", "How many minutes of video does the case hold in total?",
                "How many of the videos have an audio track?", "Which video is the longest?", "How many different plates were read across all the videos?"),
        "sealed": ("Which clip has the largest number of distinct plates?", "Is any plate seen in two or more of the videos? Which?", "What is the total running time of all the videos, in minutes?",
                   "How many videos carry sound?", "What is the longest video in the case?", "Across every video, how many distinct plates were read?"),
    }[which]
    add("most-plates", cross[0], "top1", sorted(f for f, n in distinct.items() if n == top))
    multi = {}
    for g in groups:
        multi.setdefault(g["plate"], set()).add(g["f"])
    several = sorted(p for p, fs in multi.items() if len(fs) > 1)
    add("plates-several-videos", cross[1], "set" if several else "none", several, what="plates")
    add("total-minutes", cross[2], "number", round(sum(float(x["dur"]) for x in files if x["dur"] is not None) / 60, 1), tolerance=0.06)
    add("with-audio", cross[3], "number", sum(1 for x in files if x["audio"]))
    longest = max(float(x["dur"]) for x in files if x["dur"] is not None)
    add("longest-video", cross[4], "top1", sorted(x["f"] for x in files if x["dur"] is not None and abs(float(x["dur"]) - longest) < 1e-6))
    add("all-distinct", cross[5], "number", len({g["plate"] for g in groups}))
    # speech and text in the pictures: the key is what the model transcribed or read, whatever its quality
    with_speech = sorted({s["f"] for s in speech})
    if with_speech:
        add("speech-which", {"dev": "Which videos have speech in them?", "sealed": "In which of the videos can anyone be heard speaking?"}[which], "set", with_speech, what="files")
    if ocr:
        counts = {}
        for r in ocr:
            counts[r["f"]] = counts.get(r["f"], 0) + 1
        top = max(counts.values())
        add("ocr-most", {"dev": "Which video has the most text visible in it?", "sealed": "Which clip shows the largest amount of readable text?"}[which], "top1", sorted(f for f, n in counts.items() if n == top))
    return qs


def cells(grid_rows):
    out = []
    for row in grid_rows or []:
        out.append(list(row.values()) if isinstance(row, dict) else list(row))
    return out


def numbers_of(text):
    return [float(x) for x in re.findall(r"-?\d+(?:\.\d+)?", (text or "").replace(",", ""))]


def judge(q, answer, grid):
    rows = cells(grid)
    flat = [str(c) for r in rows for c in r if c is not None]
    exp = q["expected"]
    kind = q["kind"]
    if kind == "number":
        tol = q.get("tolerance", 0.0)
        cand = []
        if len(rows) == 1 and len(rows[0]) == 1:
            try:
                cand = [float(rows[0][0])]
            except (TypeError, ValueError):
                cand = numbers_of(str(rows[0][0]))
        else:
            for r in rows[:3]:
                for c in r:
                    try:
                        cand.append(float(c))
                    except (TypeError, ValueError):
                        pass
        if not cand:
            cand = numbers_of(answer)[:6]
        return ("correct", "") if any(abs(x - float(exp)) <= tol + 1e-9 for x in cand) else ("wrong", "expected a number the answer does not give")
    if kind == "top1":
        first = None
        if rows:
            first = [str(c) for c in rows[0] if c is not None]
        if first is not None and any(x in exp for x in first):
            return "correct", ""
        if not rows and any(x in (answer or "") for x in exp) and len(exp) == 1:
            return "correct", ""
        return "wrong", "first row is not an acceptable label"
    if kind in ("set", "none"):
        pool = set(flat) | set(PLATE.findall(answer or "")) | {w for w in re.findall(r"[\w.\-]+\.(?:mp4|mov|avi|mkv)", answer or "")}
        wanted = set(exp)
        if kind == "none":
            return ("correct", "") if not rows or not any(PLATE.fullmatch(c) for c in flat) else ("wrong", "plates listed where none are expected")
        found = {x for x in pool if x in wanted}
        extra = set()
        if q.get("what") == "plates":
            extra = {x for x in pool if PLATE.fullmatch(x) and x not in wanted}
        elif q.get("what") == "files":
            extra = {x for x in pool if re.fullmatch(r"[\w.\-]+\.(?:mp4|mov|avi|mkv)", x) and x not in wanted}
        if found == wanted and not extra:
            return "correct", ""
        return "wrong", "%d of %d expected found, %d extra" % (len(found), len(wanted), len(extra))
    return "wrong", "unknown kind"


def run(keyfile, out_path, only, url):
    import ask_case
    key = json.load(open(keyfile, encoding="utf-8"))
    questions = key["questions"]
    if only:
        questions = [q for q in questions if q["id"] in only]
    api_key = ask_case.api_key()
    with open(out_path, "a", encoding="utf-8") as out:
        for q in questions:
            reply = ask_case.ask(url, api_key, key["collection"], q["q"], 300)
            headers = reply.get("headers") or {}
            data = reply.get("data") or {}
            ent = data.get("enterprise") or {}
            state = re.search(r"state=(\w+)", headers.get("X-Ask-Engine-V2", ""))
            state = state.group(1) if state else "none"
            answer = ent.get("executive_answer") or data.get("answer") or ""
            grid = (ent.get("data_grid") or {}).get("rows") or []
            verdict, why = judge(q, answer, grid) if state == "answered" else ("declined", "engine state " + state)
            sql_text = (ent.get("derivation") or {}).get("sql") or ""
            wanted = [v for v in (q.get("expect_view") or "").split("|") if v]
            if verdict == "correct" and wanted and not any(v in sql_text for v in wanted):
                verdict, why = "lucky", "the answer matches the key but the query does not read " + " or ".join(wanted)
            row = {"id": q["id"], "q": q["q"], "state": state, "verdict": verdict, "why": why, "seconds": reply.get("seconds"), "answer": mask(answer)[:600],
                   "sql": mask(((ent.get("derivation") or {}).get("sql")) or "")[:500], "views": (ent.get("derivation") or {}).get("views")}
            out.write(json.dumps(row, ensure_ascii=False) + "\n")
            out.flush()
            print("%-26s %-9s %-9s %5.0fs  %s" % (q["id"], state, verdict, reply.get("seconds") or 0, why), flush=True)


def report(path):
    rows = [json.loads(line) for line in open(path, encoding="utf-8")]
    latest = {}
    for r in rows:
        latest[r["id"]] = r
    counts = {}
    for r in latest.values():
        counts[r["verdict"]] = counts.get(r["verdict"], 0) + 1
    print("questions %d: %s" % (len(latest), ", ".join("%s %d" % kv for kv in sorted(counts.items()))))
    for r in latest.values():
        if r["verdict"] != "correct":
            print("  %-26s %-9s %s | %s" % (r["id"], r["verdict"], r["why"], mask(r["answer"])[:140].replace("\n", " ")))


def main(argv):
    p = argparse.ArgumentParser()
    sub = p.add_subparsers(dest="cmd", required=True)
    b = sub.add_parser("build")
    b.add_argument("collection")
    b.add_argument("key")
    b.add_argument("--set", default="dev", choices=["dev", "sealed"])
    r = sub.add_parser("run")
    r.add_argument("key")
    r.add_argument("results")
    r.add_argument("--only", default="")
    r.add_argument("--url", default="http://127.0.0.1:8091")
    rp = sub.add_parser("report")
    rp.add_argument("results")
    a = p.parse_args(argv)
    if a.cmd == "build":
        qs = build(a.collection, a.set)
        json.dump({"collection": a.collection, "set": a.set, "built": time.strftime("%Y-%m-%d %H:%M:%S"), "questions": qs}, open(a.key, "w", encoding="utf-8"), ensure_ascii=False, indent=1)
        print("%d questions written to %s (the key holds plate strings: do not print or commit it)" % (len(qs), a.key))
    elif a.cmd == "run":
        run(a.key, a.results, set(filter(None, a.only.split(","))), a.url)
    else:
        report(a.results)


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
