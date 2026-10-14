#!/usr/bin/env python3
"""Builds a demonstration pack: a generated case written in the exact layouts of the registered provider formats, ready to upload, with its answer key and a question list.

    python -X utf8 tools/demo_pack/build_demo_pack.py [--seed 1] [--out C:\\NexusAI-DemoPack] [--questions-only]

The files are SYNTHETIC. They imitate the real exports the owner supplied (the same column headers, formats, quirks and defects, one file per subscriber like
`MSISDN-<number>.xlsx`, tower dumps, IPDR exports in four layouts, application usage) with values from the reserved ranges only, so they can be uploaded to the
product and shown to anyone. Nothing here reads a real file.

Writes <out>/<case id>/:
  upload/            the files to upload through the interface (the answer key is NOT in this folder)
  key/               the generator's answer key and world: the truth the questions below were written from; never upload it
  MANIFEST.md        which file imitates which provider layout, with its row count
  DEMO_QUESTIONS.md  questions with the answers the key gives, in three tiers: what the product answers today, link analysis (the Ask engine of roadmap 6.8, not built
                     as a product path yet), and what to say about limits
  questions.json     the same questions, machine-readable
"""
import argparse
import json
import os
import shutil
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.abspath(os.path.join(HERE, "..", "case_generator")))
sys.path.insert(0, os.path.abspath(os.path.join(HERE, "..", "..")))
import generate  # noqa: E402
import world as w  # noqa: E402

DEFAULT_OUT = r"C:\NexusAI-DemoPack"


def top_contacts(key, msisdn, n=5):
    rows = key["oracle"]["A1"][msisdn]
    return rows[:n]


def total(row):
    return row["outgoing"] + row["incoming"] + row["unknown"]


def repeats_in(key, export_name):
    return sum(1 for m in key["expected_merge"] if m["export"] == export_name)


def fmt_number(n):
    return str(n)


def build_questions(key):
    """Returns a list of dicts: tier, id, question, expected, say (what to point out), source (the key part it came from)."""
    qs = []
    o = key["oracle"]
    subjects = key["subjects"]

    def add(tier, qid, question, expected, say="", source=""):
        qs.append({"tier": tier, "id": qid, "question": question, "expected": expected, "say": say, "source": source})

    # ---- tier 1: asked today through the governed lane (counts and filters over the loaded call records) --------------------------------------
    s0 = subjects[0]
    p0 = s0["person"]
    r0 = s0["rows"]
    rep0 = repeats_in(key, p0)
    add("today", "T1", "How many call records are there where the phone number is %s?" % s0["msisdn"],
        "%d in the file (%d rows, less %d exact repeats that the product drops at load); confirm at the first load" % (r0 - rep0, r0, rep0),
        "The count is for one subscriber's own export; the number may appear in other files as a counterparty.", "key.subjects, key.expected_merge")
    add("today", "T2", "How many calls does %s have?" % "923000099999",
        "No record contains that number: the answer says every identifier field was searched.",
        "An honest absence, not a zero: the system searched before it said none.", "not in the case")
    add("today", "T3", "How many call records were there at night?",
        "Computed by the product on the case clock (Asia/Karachi), night = 00:00 to 05:59; compare with a SQL count over the loaded rows (factory.py explain).",
        "State the clock: stored time is UTC, the answer is on the case clock.", "factory key at load")

    # ---- tier 2: link analysis through the Ask engine (target; expected answers from the key) --------------------------------------------------
    for subject in subjects[:3]:
        contacts = top_contacts(key, subject["msisdn"], 5)
        if not contacts:
            continue
        listing = "; ".join("%s (%d events, %d to %d, first %s, last %s)" % (c["number"], total(c), c["outgoing"], c["incoming"], c["first"][:10], c["last"][:10]) for c in contacts)
        add("link", "L-A1-%s" % subject["person"], "Who does %s talk to most? Give the top 5." % subject["msisdn"],
            listing, "Contact profile (A1): counts by direction, first and last contact. Services such as short codes appear with their class.", "oracle.A1")
    shown = 0
    for a in sorted(o["A2"], key=lambda a: (not a["shared"], not a["excluded"])):
        if not a["shared"] and not a["excluded"]:
            continue
        shared = ", ".join("%s (%d and %d events)" % (s["number"], s["x_count"], s["y_count"]) for s in a["shared"]) or "none"
        excluded = "; ".join("%s (%s)" % (e["number"], e["reason"]) for e in a["excluded"]) or "none"
        add("link", "L-A2-%s" % (a["plant"] or "pair%d" % shown), "Who do %s and %s both contact?" % (a["x"], a["y"]),
            "Shared: %s. Left out: %s." % (shared, excluded),
            "Common contacts (A2). Say that services and hub numbers are excluded by rule and listed, not hidden.", "oracle.A2")
        shown += 1
        if shown >= 3:
            break
    for a in o["A5"]:
        if a["windows"]:
            add("link", "L-A5-%s-%s" % (a["x"][-4:], a["y"][-4:]), "Were %s and %s ever at the same cell within 15 minutes?" % (a["x"], a["y"]),
                "%d window(s), %d pair(s) of events observed against %.1f expected by chance (lift %s). First window: %s to %s at cell %s."
                % (len(a["windows"]), a["pairs_observed"], a["pairs_expected"], a["lift"]["value"], a["windows"][0]["start_local"], a["windows"][0]["end_local"], a["windows"][0]["cell_key"]),
                "Co-location (A5). Say: " + a["caveat"] + ". The chance baseline and lift are shown beside the windows.", "oracle.A5")
            break
    for a in o["A6"][:1]:
        add("link", "L-A6", "Which numbers appear at two or more cells in the %s tower dump?" % a["dump"],
            "%s." % ", ".join(a["at_two_or_more_cells"]), "Tower dump (A6): authority-gated in a real case; here a synthetic dump.", "oracle.A6")
    for a in o["A7"]:
        if a["candidates"]:
            c = a["candidates"][0]
            add("link", "L-A7-%s" % a["plant"], "Did %s and %s probably meet?" % (a["x"], a["y"]),
                "%d candidate meeting(s); first: %s to %s at cell %s, %d pairs, a contact at %s %.0f minutes before, score %.2f."
                % (len(a["candidates"]), c["start_local"], c["end_local"], c["cell_key"], c["pairs"], c["contacts_before"][-1], c["gap_minutes"], c["score"]),
                "Possible meeting (A7). Say: " + a["caveat"] + ".", "oracle.A7")
            break
    changed = [(n, d["changes"][0]) for n, d in o["A8"]["numbers"].items() if d["changes"]]
    if changed:
        n, c = changed[0]
        add("link", "L-A8", "Did %s change phone or SIM?" % n,
            "Yes: %s changed from IMEI %s to %s (IMSI %s to %s) between %s and %s." % (c["kind"], c["old_imei"], c["new_imei"], c["old_imsi"], c["new_imsi"], c["last_seen_old"], c["first_seen_new"]),
            "Device and SIM changes (A8). The pairing is with dates; both are kept.", "oracle.A8")
    succ = [x for x in o["A9"]["silent"] if x["candidates"]]
    if succ:
        s, c = succ[0], succ[0]["candidates"][0]
        add("link", "L-A9", "Who may have replaced %s, which went silent?" % s["number"],
            "Candidate %s: contact similarity %.2f, shared cells %.2f, %d shared contacts, started %s after the last event on %s; score %.2f."
            % (c["number"], c["contact_cosine"], c["cell_jaccard"], c["shared_contacts"], c["first_event"][:10], s["last_event"][:10], c["score"]),
            "Successor candidate (A9): " + o["A9"]["caveat"] + ".", "oracle.A9")
    for n, d in o["A10"]["numbers"].items():
        if d.get("changes"):
            c = d["changes"][0]
            add("link", "L-A10", "Did %s change where they sleep, and are there impossible speeds?" % n,
                "Night anchor moved from cell %s to %s (%.1f km) from the week starting %s; %d speed alert(s), the first %.1f km in %d s (%.0f km/h) on %s."
                % (c["from_cell"], c["to_cell"], c["distance_km"], c["first_week_start"], len(d["speed_alerts"]), d["speed_alerts"][0]["distance_km"],
                   d["speed_alerts"][0]["gap_s"], d["speed_alerts"][0]["speed_kmh"], d["speed_alerts"][0]["from_time"][:10]) if d["speed_alerts"] else "Night anchor moved (%.1f km)." % c["distance_km"],
                "Movement and anchors (A10): " + o["A10"]["caveat"] + ".", "oracle.A10")
            break
    for n, d in o["A11"]["numbers"].items():
        if d["change"]:
            c = d["change"]
            add("link", "L-A11", "Did the activity of %s change?" % n,
                "Yes: from %s activity went %s, %.1f events a day before and %.1f after (ratio %.2f)." % (c["first_day"], c["direction"], c["mean_before"], c["mean_after"], c["ratio"]),
                "Rhythm change (A11): " + o["A11"]["caveat"] + ".", "oracle.A11")
            break
    for a in o["A13"]:
        add("link", "L-A13-%s" % (a["label"].replace(" ", "-")), "Who held %s on port %d at %s?" % (a["public_ip"], a["port"], a["instant"]),
            "%s: %s." % (a["result"], ", ".join(a["holders"]) or "no holder"),
            "Address attribution (A13): ambiguous is a result; the system never guesses between two holders.", "oracle.A13")
        if len([q for q in qs if q["id"].startswith("L-A13")]) >= 3:
            break

    # ---- tier 3: limits to say out loud ----------------------------------------------------------------------------------------------------
    add("limits", "X1", "Who is the owner of %s?" % subjects[0]["msisdn"],
        "The system answers with what the records hold about the number; it does not name a person: an association is a candidate until an analyst confirms it.",
        "Associations are labelled observed, inferred or candidate; identity is never asserted.", "policy P4")
    add("limits", "X2", "Where is the person who uses %s right now?" % subjects[0]["msisdn"],
        "Declined or answered with the last cell seen, with its radius: a cell is not a position.", "Cell sites are not positions of a person.", "policy P6")
    return qs


def write_manifest(case_root, key, out_path):
    registry = None
    try:
        from ingestion.forensic_records.layout_registry import load_registry
        registry = load_registry()
    except Exception:  # the manifest is still useful without the registry's labels
        registry = None
    labels = {}
    if registry is not None:
        for entry in getattr(registry, "entries", []) or []:
            labels[entry["layout_id"]] = entry.get("label") or entry.get("operator_label") or ""
    lines = ["# Demo pack manifest", "", "Synthetic files in the layouts of the registered provider formats. Upload the `upload/` folder; never upload `key/`.", "",
             "| File | Layout | Operator label | Rows |", "|---|---|---|---|"]
    groups = [("subjects", key["subjects"]), ("dumps", key["dumps"]), ("sessions", key["sessions"]), ("ip_queries", key["ip_queries"]), ("app_usage", key["app_usage"])]
    for _, items in groups:
        for item in items:
            lines.append("| %s | %s | %s | %d |" % (os.path.basename(item["file"]), item["layout_id"], labels.get(item["layout_id"], "") or item.get("operator", ""), item["rows"]))
    with open(out_path, "w", encoding="utf-8", newline="\n") as handle:
        handle.write("\n".join(lines) + "\n")


def write_questions_md(qs, key, out_path):
    tiers = [("today", "Tier 1: what the product answers today", "Counts and filters over the loaded call records, through the governed lane. Expected values come from the key and must be confirmed at the first load (the product drops exact repeats at ingest; times are on the case clock)."),
             ("link", "Tier 2: link analysis (the Ask engine, roadmap 6.8)", "The questions an analyst asks about connections. **Not answered through the Ask module yet**: the engine is being built (validator, catalogue, shortlist done; model call and executor next). The expected answers come from the generator's key and are what the engine will be scored against."),
             ("limits", "Tier 3: limits to say out loud", "What the system declines or qualifies, and why.")]
    lines = ["# Demo questions for case %s (seed %s)" % (key["case_id"], key["seed"]), "",
             "Generated from the answer key by `tools/demo_pack/build_demo_pack.py`. All numbers are synthetic (the project's reserved range). The case clock is %s." % key["clock"]["local_zone"], ""]
    for tier, title, intro in tiers:
        items = [q for q in qs if q["tier"] == tier]
        if not items:
            continue
        lines += ["## " + title, "", intro, ""]
        for q in items:
            lines += ["### %s. %s" % (q["id"], q["question"]), "", "- **Expected:** " + q["expected"]]
            if q["say"]:
                lines.append("- **Say:** " + q["say"])
            lines += ["- **From:** " + q["source"], ""]
    with open(out_path, "w", encoding="utf-8", newline="\n") as handle:
        handle.write("\n".join(lines))


def main(argv):
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--seed", type=int, default=1)
    parser.add_argument("--out", default=DEFAULT_OUT)
    parser.add_argument("--questions-only", action="store_true", help="write only DEMO_QUESTIONS.md and questions.json into --out (no files)")
    args = parser.parse_args(argv)
    tmp = tempfile.mkdtemp(prefix="nx-demo-pack-")
    try:
        params = w.Params(seed=args.seed)
        case_root = generate.write_case(params, tmp)
        with open(os.path.join(case_root, "key", "key.json"), encoding="utf-8") as handle:
            key = json.load(handle)
        target = os.path.join(args.out, key["case_id"])
        os.makedirs(target, exist_ok=True)
        qs = build_questions(key)
        write_questions_md(qs, key, os.path.join(target, "DEMO_QUESTIONS.md"))
        with open(os.path.join(target, "questions.json"), "w", encoding="utf-8", newline="\n") as handle:
            json.dump(qs, handle, indent=2, ensure_ascii=False)
            handle.write("\n")
        if not args.questions_only:
            for name in ("upload", "key"):
                if os.path.isdir(os.path.join(target, name)):
                    shutil.rmtree(os.path.join(target, name))
            shutil.copytree(os.path.join(case_root, "files"), os.path.join(target, "upload"))
            shutil.copytree(os.path.join(case_root, "key"), os.path.join(target, "key"))
            write_manifest(case_root, key, os.path.join(target, "MANIFEST.md"))
        print("demo pack: %s (%d files to upload, %d questions)" % (target, len(os.listdir(os.path.join(target, "upload"))) if not args.questions_only else 0, len(qs)))
        return 0
    finally:
        shutil.rmtree(tmp, ignore_errors=True)


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
