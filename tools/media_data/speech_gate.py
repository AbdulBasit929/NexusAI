#!/usr/bin/env python3
"""Applies the gates of docs/data/speech-lab-preregistration-20261112.md (section 6 and amendment A4) to the SEALED result files of one model and prints the status each language earns.

    python -I -X utf8 tools/media_data/speech_gate.py MODEL [RESULTS_DIR]

Pashto (the three spontaneous sets pooled, plus the read set):
  MACHINE_TRANSCRIPT  read set word error (N2) <= 30%; pooled spontaneous word error (N2) <= 45% and character error (N2) <= 20%; script share >= 99%; loops <= 1% of clips; real-time factor <= 1.5
  SEARCH_LEAD         pooled spontaneous word recall >= 50%; script share >= 99%; loops <= 1% of clips; real-time factor <= 1.5
  otherwise           LOW_RESOURCE_LANGUAGE
Every check is printed with its figure, so that the status can be audited line by line. Only partition "sealed" files are read; the newest file of each set and model wins (a repeat is a deliberate act and is said in the results).
"""
import glob
import json
import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)

import speech_lab as sl  # noqa: E402
import speech_report as sr  # noqa: E402

DEFAULT_DIR = sr.DEFAULT_DIR
SPONTANEOUS = ("pst_Arab", "pbt_Arab", "pbu_Arab")
READ = "ps_af"


def pooled(runs):
    rows = [r for run in runs for r in run["rows"]]
    out = {"clips": len(rows)}
    for version in ("N2",):
        out["wer"] = sl.corpus_rate([r[version]["w_edits"] for r in rows], [r[version]["w_len"] for r in rows])
        out["cer"] = sl.corpus_rate([r[version]["c_edits"] for r in rows], [r[version]["c_len"] for r in rows])
    out["recall"] = sr.recall(rows)
    out["script"] = sum(r["script"] for r in rows) / len(rows) if rows else 0.0
    out["loop_share"] = sum(1 for r in rows if r["loop"]) / len(rows) if rows else 1.0
    out["rtf"] = sum(r["compute"] for r in rows) / (sum(r["seconds"] for r in rows) or 1.0)
    return out


def latest(runs, set_name):
    mine = [r for r in runs if r["meta"]["set"] == set_name]
    return mine[-1] if mine else None


def evaluate(runs):
    """(status, [(check, figure text, passed)]) for Pashto from the sealed runs of ONE model."""
    spont = [latest(runs, s) for s in SPONTANEOUS]
    read = latest(runs, READ)
    checks = []
    if any(r is None for r in spont) or read is None:
        return "NOT EVALUATED", [("all four sealed sets present", "missing: %s" % [s for s, r in zip(SPONTANEOUS + (READ,), spont + [read]) if r is None], False)]
    p, q = pooled(spont), pooled([read])
    machine = [
        ("read set word error (N2) <= 30%", "%.1f%%" % (100 * q["wer"]), q["wer"] <= 0.30),
        ("pooled spontaneous word error (N2) <= 45%", "%.1f%%" % (100 * p["wer"]), p["wer"] <= 0.45),
        ("pooled spontaneous character error (N2) <= 20%", "%.1f%%" % (100 * p["cer"]), p["cer"] <= 0.20),
    ]
    common = [
        ("script share >= 99% (pooled spontaneous and read)", "%.1f%% / %.1f%%" % (100 * p["script"], 100 * q["script"]), min(p["script"], q["script"]) >= 0.99),
        ("loops <= 1% of clips (pooled spontaneous)", "%.1f%%" % (100 * p["loop_share"]), p["loop_share"] <= 0.01),
        ("real-time factor <= 1.5", "%.2f" % max(p["rtf"], q["rtf"]), max(p["rtf"], q["rtf"]) <= 1.5),
    ]
    lead = [("pooled spontaneous word recall >= 50%", "%.1f%%" % (100 * p["recall"]), p["recall"] >= 0.50)]
    machine_ok = all(c[2] for c in machine + common)
    lead_ok = all(c[2] for c in lead + common)
    status = sl_status(machine_ok, lead_ok)
    return status, machine + lead + common


def sl_status(machine_ok, lead_ok):
    return "MACHINE_TRANSCRIPT" if machine_ok else ("SEARCH_LEAD" if lead_ok else "LOW_RESOURCE_LANGUAGE")


def main(argv):
    if not argv:
        raise SystemExit(__doc__)
    model, results_dir = argv[0], (argv[1] if len(argv) > 1 else DEFAULT_DIR)
    runs = [r for r in sr.load(results_dir, "sealed") if r["meta"]["model"] == model and r["meta"].get("backend", "whisper") in ("omni", "whisper")]
    status, checks = evaluate(runs)
    print("model %s, sealed runs: %s" % (model, sorted({r["meta"]["set"] for r in runs})))
    for name, figure, ok in checks:
        print("  %-52s %-16s %s" % (name, figure, "met" if ok else "NOT MET"))
    print("STATUS for Pashto: %s" % status)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
