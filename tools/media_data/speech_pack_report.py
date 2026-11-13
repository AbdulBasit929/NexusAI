#!/usr/bin/env python3
"""How well the recordings of the Orion speech pack were read by the live worker: the stored readings (read from the artifacts table) against the text each clip was recorded from.

    python -I -X utf8 tools/media_data/speech_pack_report.py [--pack C:/NexusAI-Showcase/orion/key/speech-pack-key.json] [--out docs/qa/orion-speech-pack-report-20261113.md]

Per recording: the language the pack says, the language and reader the worker stored, the reading status, the number of segments, the length covered, word and character error (the lab's normalisations: N1 for
English, N2 for Pashto and Urdu) and word recall (the share of the reference's different words that occur in the reading). Per language: corpus word and character error and mean recall. This is a product
check of the live path on a handful of clips, not a gate: the gates are in docs/data/speech-lab-preregistration-20261112.md and were measured on 120 sealed clips.
"""
import argparse
import json
import os
import subprocess
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
import speech_lab as sl  # noqa: E402

COLLECTION = "orion-lahore-reference-20261030"
SQL = ("select e.original_filename, coalesce((d.citation_locator ->> 'start_seconds')::numeric, (d.metadata -> 'observation' ->> 'start_seconds')::numeric), "
       "coalesce((d.citation_locator ->> 'end_seconds')::numeric, (d.metadata -> 'observation' ->> 'end_seconds')::numeric), d.metadata -> 'observation' ->> 'text', "
       "d.metadata -> 'observation' ->> 'language', d.metadata -> 'observation' ->> 'model', coalesce(d.metadata -> 'observation' ->> 'reliability_state', ''), "
       "coalesce(d.metadata -> 'observation' ->> 'language_resolution', '') "
       "from forensic.derived_artifacts d join forensic.evidence_items e on e.tenant_id = d.tenant_id and e.collection_id = d.collection_id and e.evidence_id = d.evidence_id "
       "where d.collection_id = '%s' and e.modality = 'audio' and d.artifact_type = 'forensics.audio-timestamp-segment/v1' and d.processing_status = 'completed' order by 1, 2")
NAMES = {"ps": "Pashto", "ur": "Urdu", "en": "English"}


def stored_readings(collection):
    out = subprocess.run(["docker", "exec", "-i", "nexusai-forensic-postgres-1", "psql", "-U", "localrecall", "-d", "localrecall", "-At", "-F", "\x1f", "-v", "ON_ERROR_STOP=1"],
                         input=SQL % collection, capture_output=True, text=True, encoding="utf-8")
    if out.returncode != 0:
        raise SystemExit("psql failed: " + out.stderr[-400:])
    rows = {}
    current = None
    for line in out.stdout.split("\n"):
        parts = line.split("\x1f")
        if len(parts) == 8 and parts[0].startswith("voice-note-"):
            current = {"start": float(parts[1]), "end": float(parts[2]), "text": parts[3], "language": parts[4], "model": parts[5], "status": parts[6], "resolution": parts[7]}
            rows.setdefault(parts[0], []).append(current)
        elif current is not None and line:      # a text with a line break inside it
            current["text"] += "\n" + line
    return rows


def recall(reference, hypothesis):
    ref = set(reference.split())
    return len(ref & set(hypothesis.split())) / len(ref) if ref else 0.0


def main(argv):
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--pack", default="C:/NexusAI-Showcase/orion/key/speech-pack-key.json")
    ap.add_argument("--out", default="")
    ap.add_argument("--collection", default=COLLECTION)
    args = ap.parse_args(argv)
    pack = json.load(open(args.pack, encoding="utf-8"))
    stored = stored_readings(args.collection)
    lines = ["| File | Said in | Stored language | Reader | Status | Segments | Covered s of s | Word error | Character error | Word recall |", "|---|---|---|---|---|---|---|---|---|---|"]
    per = {}
    identified = 0
    for item in sorted(pack, key=lambda p: p["file"]):
        rows = stored.get(item["file"], [])
        version = "N1" if item["language"] == "en" else "N2"
        hyp = sl.normalise(" ".join(r["text"] for r in rows), version)
        ref = sl.normalise(item["reference"], version)
        we, wn, ce, cn = sl.clip_errors(ref, hyp)
        language = rows[0]["language"] if rows else "-"
        ok = language == item["language"]
        identified += ok
        reader = "Pashto route" if rows and "omni" in rows[0]["model"].lower() else ("Whisper" if rows else "-")
        covered = max((r["end"] for r in rows), default=0.0)
        status = rows[0]["status"] or "(none)" if rows else "-"
        lines.append("| %s | %s | %s%s | %s | %s | %d | %.1f of %.1f | %.1f%% | %.1f%% | %.0f%% |" % (item["file"], NAMES[item["language"]], language, "" if ok else " (WRONG)", reader, status, len(rows), covered, item["seconds"],
                                                                                                       100 * we / max(1, wn), 100 * ce / max(1, cn), 100 * recall(ref, hyp)))
        bucket = per.setdefault(item["language"], {"we": [], "wn": [], "ce": [], "cn": [], "rc": []})
        for key, value in (("we", we), ("wn", wn), ("ce", ce), ("cn", cn), ("rc", recall(ref, hyp))):
            bucket[key].append(value)
    summary = ["| Language | Recordings | Word error | Character error | Mean word recall |", "|---|---|---|---|---|"]
    for code, b in sorted(per.items()):
        summary.append("| %s | %d | %.1f%% | %.1f%% | %.0f%% |" % (NAMES[code], len(b["we"]), 100 * sl.corpus_rate(b["we"], b["wn"]), 100 * sl.corpus_rate(b["ce"], b["cn"]), 100 * sum(b["rc"]) / len(b["rc"])))
    text = ["# Orion speech pack: the live reading against the text each clip was recorded from", "",
            "Ten public clips (FLEURS read speech and one Omnilingual spontaneous Pashto clip, development partition only), uploaded through the interface with **no language chosen**. Language identified correctly: **%d of %d**." % (identified, len(pack)), "",
            *summary, "", *lines, "",
            "Word error is on normalised text (N1 English, N2 Pashto and Urdu: no punctuation, Arabic-script letter forms unified). A handful of clips is a product check, not a gate: the gates were measured on 120 sealed Pashto clips. The English and Urdu readings come from the Whisper service, the Pashto readings from the Pashto route (status SEARCH_LEAD, a search lead and not a transcript)."]
    report = "\n".join(text) + "\n"
    print(report)
    if args.out:
        with open(args.out, "w", encoding="utf-8", newline="\n") as handle:
            handle.write(report)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
