#!/usr/bin/env python3
"""Tables of the speech lab results (docs/data/speech-lab-preregistration-20261112.md) from the result files of tools/media_data/speech_lab.py.

    python -I -X utf8 tools/media_data/speech_report.py [RESULTS_DIR] [--partition dev|sealed] [--include-smoke]

One line per run: set, partition, model, language told, clips and minutes of audio, corpus word error (N2 for Pashto and Punjabi, N1 otherwise, with the 95% interval), character error, content-word
recall (the share of the distinct reference words of at least three letters that occur among the words of the reading, after N2: how often a spoken word could be FOUND in the text), script share,
loops, empty readings, real-time factor, and the lowest free memory of the VM. Smoke runs (label "smoke") are left out unless asked for. Later runs of the same set, partition, model, told and
options replace earlier ones in the table only when they are listed; nothing is deleted.
"""
import glob
import json
import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)

import speech_lab as sl  # noqa: E402

DEFAULT_DIR = os.path.join(os.environ.get("NEXUSAI_MEDIA_DATA", "C:/tmp/nexusai-media-data"), "results", "asr-lab")


def content_words(text):
    return {w for w in sl.normalise(text, "N2").split() if len(w) >= 3}


def recall(rows):
    """Share of the distinct reference content words (summed over clips) that occur among the words of the reading of the same clip."""
    found = total = 0
    for r in rows:
        ref = content_words(r["reference"])
        hyp = set(sl.normalise(r["hypothesis"], "N2").split())
        total += len(ref)
        found += len(ref & hyp)
    return found / total if total else 0.0


def primary_version(set_name):
    return "N2" if set_name in ("ps_af", "pst_Arab", "pbt_Arab", "pbu_Arab", "pnb_Arab", "pa_in", "sd_in") else "N1"


def load(results_dir, partition=None, include_smoke=False):
    runs = []
    for path in sorted(glob.glob(os.path.join(results_dir, "*.json"))):
        with open(path, encoding="utf-8") as handle:
            data = json.load(handle)
        if "smoke" in os.path.basename(path) and not include_smoke:
            continue
        if partition and data["meta"]["partition"] != partition:
            continue
        data["file"] = os.path.basename(path)
        runs.append(data)
    return runs


def line(run):
    meta, s, rows = run["meta"], run["summary"], run["rows"]
    v = primary_version(meta["set"])
    e = s[v]
    return {"set": meta["set"], "part": meta["partition"], "model": meta["model"], "told": meta["told"] + (" (no timestamps)" if meta.get("without_timestamps") else "") + (", %g s pieces" % meta["chunk_seconds"] if meta.get("chunk_seconds") else ""),
            "clips": s["clips"], "min": s["audio_seconds"] / 60, "norm": v, "wer": e["wer"], "wer_ci": e["wer_ci"], "cer": e["cer"], "recall": recall(rows), "script": s["script_share"],
            "loops": s["loops"], "empty": s["empty"], "rtf": s["rtf"], "free": meta.get("vm_free_mib_lowest")}


def table(runs):
    out = ["| Set | Part | Model | Told | Clips | Minutes | Word error [95%] | Char error | Word recall | Script | Loops | Empty | RTF |", "|---|---|---|---|---|---|---|---|---|---|---|---|---|"]
    for run in runs:
        x = line(run)
        out.append("| %s | %s | %s | %s | %d | %.1f | %.1f%% [%.1f, %.1f] (%s) | %.1f%% | %.1f%% | %.0f%% | %d | %d | %.2f |" % (
            x["set"], x["part"], x["model"], x["told"], x["clips"], x["min"], 100 * x["wer"], 100 * x["wer_ci"][0], 100 * x["wer_ci"][1], x["norm"], 100 * x["cer"], 100 * x["recall"], 100 * x["script"],
            x["loops"], x["empty"], x["rtf"]))
    return "\n".join(out)


def main(argv):
    args = [a for a in argv if not a.startswith("--")]
    partition = argv[argv.index("--partition") + 1] if "--partition" in argv else None
    if partition and partition in args:
        args.remove(partition)
    runs = load(args[0] if args else DEFAULT_DIR, partition, "--include-smoke" in argv)
    print(table(runs) if runs else "no results yet")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
