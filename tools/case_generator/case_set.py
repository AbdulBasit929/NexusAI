#!/usr/bin/env python3
"""The set of 20 planted-truth cases (specification section 9 and 12, step 6; roadmap section 8.3) and the register that pins it.

    python -X utf8 tools/case_generator/case_set.py plan
    python -X utf8 tools/case_generator/case_set.py build  --out CASES_DIR [--kind development|judgement|sealed|all] [--open-sealed]
    python -X utf8 tools/case_generator/case_set.py register --out tools/case_generator/case-set.json
    python -X utf8 tools/case_generator/case_set.py extend   --register tools/case_generator/case-set.json --why "..."   # add new cases, touching none of the entries
    python -X utf8 tools/case_generator/case_set.py verify   --register tools/case_generator/case-set.json [--kind ...]

Seed policy (G7). Development seeds 1 to 99 are for building and may be looked at freely. Judgement seeds 100 to 199 are generated and
registered BEFORE the code they judge is measured, and are then used for measurement only. Sealed seeds 900 and up are generated once
for the register, their hashes recorded, and their files not opened or scored against until a gate.

What "sealed" can and cannot mean. A case is a pure function of its parameters, so anybody with this file can regenerate a sealed case; the seed
is no secret. The seal is a rule of work, made checkable by the register: it holds the SHA-256 of every sealed case (its manifest, which in turn
lists the hash of every file, and its key), so a later run proves it measured the very case that was registered, and `build` refuses to write a
sealed case unless `--open-sealed` is given. `register` and `verify` generate, audit and hash a case in a temporary folder that is deleted at once;
they print hashes and counts, never the answers.

The register is tied to the generator: any change to the generator changes the hashes. `verify` says so, and the register is then made again
(judgement and sealed cases only before the code under test is measured, never after).
"""
from __future__ import annotations

import argparse
import datetime
import hashlib
import json
import os
import shutil
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
import audit  # noqa: E402
import generate  # noqa: E402
import world as w  # noqa: E402

KINDS = {"development": range(1, 100), "judgement": range(100, 200), "sealed": range(900, 1000)}

# 8 development, 8 judgement, 4 sealed. Each differs from the defaults (12 subjects, 120 people, 90 days, 4 events a day) in a way that
# changes the size or the shape of the case: the sets are alike in kind and unlike in scale, so a method is not tuned to one size.
CASES = [
    ("development", 1, {}),
    ("development", 2, {"period_days": 60}),
    ("development", 3, {"period_days": 120, "people": 150, "associates": 30}),
    ("development", 4, {"events_per_person_per_day": 3.0}),
    ("development", 5, {"events_per_person_per_day": 5.0}),
    ("development", 6, {"subjects": 14, "associates": 28}),
    ("development", 7, {"subjects": 16, "people": 150, "associates": 30}),
    ("development", 8, {"communities": 8}),
    ("judgement", 101, {}),
    ("judgement", 102, {"period_days": 75}),
    ("judgement", 103, {"period_days": 105, "people": 140, "associates": 28}),
    ("judgement", 104, {"events_per_person_per_day": 3.5}),
    ("judgement", 105, {"events_per_person_per_day": 4.5, "communities": 7}),
    ("judgement", 106, {"subjects": 14, "associates": 28}),
    ("judgement", 107, {"subjects": 16, "people": 150, "associates": 30, "period_days": 80}),
    ("judgement", 108, {"communities": 5}),
    ("judgement", 109, {"period_days": 90, "events_per_person_per_day": 3.5, "subjects": 13}),
    ("judgement", 110, {"communities": 6, "associates": 24}),
    ("judgement", 111, {"people": 130, "subjects": 15}),
    ("judgement", 112, {"events_per_person_per_day": 4.0, "period_days": 100}),
    # added 2026-10-26 (set_version 3), before any measurement of the work they will judge (the wording of absences and scores, and the ANPR family): the reserve of 105 to 112 is spent.
    # More subscribers and more events in several of them, because the pair questions need pairs of subscribers that contact each other.
    ("judgement", 113, {"subjects": 18, "associates": 30}),
    ("judgement", 114, {"period_days": 120, "events_per_person_per_day": 3.0}),
    ("judgement", 115, {"events_per_person_per_day": 5.5, "subjects": 10}),
    ("judgement", 116, {"people": 160, "associates": 32, "communities": 6}),
    ("judgement", 117, {"subjects": 14, "period_days": 90}),
    ("judgement", 118, {"communities": 4, "associates": 20}),
    ("judgement", 119, {"subjects": 20, "people": 170, "associates": 34, "period_days": 70}),
    ("judgement", 120, {"events_per_person_per_day": 3.5, "period_days": 110, "subjects": 12}),
    # added 2026-10-29 (set_version 4), before the third measurement of the subscriber family (cases 117 and 118 both fell short of the gate, causes fixed in the engine): one fresh case.
    ("judgement", 121, {"subjects": 13, "people": 120, "period_days": 85, "associates": 26}),
    # added 2026-10-29 (set_version 5), before the third measurement of the subscriber and transaction families (the second fresh runs on 120 and 121 fell short of the gate, causes fixed in the engine
    # and checked on development cases): one fresh case for each (122 subscribers, 123 transactions) and one more for each, to be used only if the first falls short and its cause is fixed (124, 125).
    ("judgement", 122, {"subjects": 14, "people": 130, "period_days": 90, "associates": 28}),
    ("judgement", 123, {"events_per_person_per_day": 3.0, "period_days": 95, "subjects": 11}),
    ("judgement", 124, {"subjects": 12, "people": 110, "period_days": 80, "associates": 24}),
    ("judgement", 125, {"period_days": 100, "subjects": 13, "associates": 26}),
    # added 2026-10-31 (set_version 6), before the sixth measurement of the subscriber family (programme 10.15): one fresh case, used once
    ("judgement", 126, {"subjects": 13, "people": 115, "period_days": 85, "associates": 25}),
    ("sealed", 901, {}),
    ("sealed", 902, {"period_days": 70}),
    ("sealed", 903, {"subjects": 14, "associates": 28, "events_per_person_per_day": 4.5}),
    ("sealed", 904, {"period_days": 110, "people": 140, "associates": 28, "communities": 7}),
]


def case_id(kind, seed):
    return "%s-%d" % (kind, seed)


def validate(cases=CASES):
    """The set is well formed: seeds unique and inside the range of their kind, the right number of each kind."""
    problems = []
    seeds = [seed for _, seed, _ in cases]
    if len(set(seeds)) != len(seeds):
        problems.append("a seed is used twice")
    for kind, seed, overrides in cases:
        if kind not in KINDS or seed not in KINDS[kind]:
            problems.append("%s: seed %d is outside the range of its kind" % (case_id(kind, seed), seed))
        unknown = set(overrides) - set(w.Params.__dataclass_fields__)
        if unknown:
            problems.append("%s: unknown parameters %s" % (case_id(kind, seed), sorted(unknown)))
    return problems


def params_for(seed, overrides):
    return w.Params(seed=seed, **overrides)


def sha256_file(path):
    with open(path, "rb") as handle:
        return hashlib.sha256(handle.read()).hexdigest()


def generate_and_describe(kind, seed, overrides, out_dir, check=True):
    """Writes one case under out_dir; returns its register entry (hashes, counts and the audit result)."""
    params = params_for(seed, overrides)
    root = generate.write_case(params, out_dir)
    with open(os.path.join(root, "manifest.json"), encoding="utf-8") as handle:
        manifest = json.load(handle)
    with open(os.path.join(root, "key", "key.json"), encoding="utf-8") as handle:
        key = json.load(handle)
    plants = {}
    for plant in key["plants"]:
        plants[plant["type"]] = plants.get(plant["type"], 0) + 1
    problems = audit.audit(root) if check else None
    return {
        "id": case_id(kind, seed), "kind": kind, "seed": seed, "overrides": overrides,
        "generator_version": manifest["generator_version"], "spec_version": manifest["spec_version"],
        "manifest_sha256": sha256_file(os.path.join(root, "manifest.json")), "key_sha256": sha256_file(os.path.join(root, "key", "key.json")),
        "files": len(manifest["files"]), "bytes": sum(info["bytes"] for info in manifest["files"].values()),
        "plants": dict(sorted(plants.items())), "audit_problems": None if problems is None else len(problems)}, problems


def build_register(cases=CASES, progress=None):
    entries, failures = [], []
    for kind, seed, overrides in cases:
        with tempfile.TemporaryDirectory() as scratch:
            entry, problems = generate_and_describe(kind, seed, overrides, scratch)
        entries.append(entry)
        if problems:
            failures.append((entry["id"], problems))
        if progress:
            progress(entry)
    return {"set_version": 1, "generator_version": w.GENERATOR_VERSION, "spec_version": w.SPEC_VERSION, "cases": entries,
            "policy": "development 1-99 free; judgement 100-199 registered before the code they judge is measured; sealed 900+ registered once, not opened or scored until a gate"}, failures


def verify_register(register, kinds=("development", "judgement"), progress=None):
    """Regenerates the chosen kinds and compares with the register. Returns a list of differences (empty: the register still describes the generator)."""
    differences = []
    if register.get("generator_version") != w.GENERATOR_VERSION or register.get("spec_version") != w.SPEC_VERSION:
        differences.append("the register was made by generator %s / spec %s, this is %s / %s" % (register.get("generator_version"), register.get("spec_version"), w.GENERATOR_VERSION, w.SPEC_VERSION))
    for entry in register["cases"]:
        if entry["kind"] not in kinds:
            continue
        with tempfile.TemporaryDirectory() as scratch:
            now, problems = generate_and_describe(entry["kind"], entry["seed"], entry["overrides"], scratch, check=False)
        for field in ("manifest_sha256", "key_sha256", "files", "bytes", "plants"):
            if now[field] != entry[field]:
                differences.append("%s: %s differs from the register" % (entry["id"], field))
        if progress:
            progress(now)
    return differences


def main(argv=None):
    ap = argparse.ArgumentParser(description="The set of planted-truth cases and its register.")
    sub = ap.add_subparsers(dest="command", required=True)
    sub.add_parser("plan")
    b = sub.add_parser("build")
    b.add_argument("--out", required=True)
    b.add_argument("--kind", default="development", choices=["development", "judgement", "sealed", "all"])
    b.add_argument("--open-sealed", action="store_true", help="allow writing sealed cases (a gate)")
    b.add_argument("--no-audit", action="store_true")
    r = sub.add_parser("register")
    r.add_argument("--out", required=True)
    x = sub.add_parser("extend", help="add the cases of CASES that the register does not hold yet; the entries already in it are not touched")
    x.add_argument("--register", required=True)
    x.add_argument("--why", required=True, help="one sentence for the history of the register")
    v = sub.add_parser("verify")
    v.add_argument("--register", required=True)
    v.add_argument("--kind", default="development,judgement", help="comma separated kinds (sealed only at a gate)")
    args = ap.parse_args(argv)

    problems = validate()
    if problems:
        print("\n".join(problems))
        return 2
    if args.command == "plan":
        for kind, seed, overrides in CASES:
            print("%-12s %4d  %s" % (kind, seed, json.dumps(overrides, sort_keys=True) if overrides else "defaults"))
        counts = {k: sum(1 for kind, _, _ in CASES if kind == k) for k in KINDS}
        print("%d cases: %s" % (len(CASES), ", ".join("%d %s" % (n, k) for k, n in counts.items())))
        return 0
    if args.command == "build":
        chosen = [c for c in CASES if args.kind == "all" or c[0] == args.kind]
        if any(kind == "sealed" for kind, _, _ in chosen) and not args.open_sealed:
            print("sealed cases are not written without --open-sealed (specification G7: not opened until a gate)")
            return 3
        os.makedirs(args.out, exist_ok=True)
        status = 0
        for kind, seed, overrides in chosen:
            entry, found = generate_and_describe(kind, seed, overrides, os.path.join(args.out, kind), check=not args.no_audit)
            print("%-18s %2d files %6.1f MB  audit %s" % (entry["id"], entry["files"], entry["bytes"] / 1e6, "skipped" if found is None else ("ok" if not found else "%d PROBLEMS" % len(found))), flush=True)
            for line in (found or [])[:10]:
                print("   ", line)
            status |= 1 if found else 0
        return status
    if args.command == "register":
        register, failures = build_register(progress=lambda e: print("%-18s %2d files %6.1f MB  audit %s" % (e["id"], e["files"], e["bytes"] / 1e6, e["audit_problems"]), flush=True))
        with open(args.out, "w", encoding="utf-8", newline="\n") as handle:
            json.dump(register, handle, indent=1, sort_keys=True)
            handle.write("\n")
        for name, found in failures:
            print("AUDIT FAILED", name, found[:5])
        print("wrote", args.out)
        return 1 if failures else 0
    if args.command == "extend":
        with open(args.register, encoding="utf-8") as handle:
            register = json.load(handle)
        have = {entry["id"] for entry in register["cases"]}
        missing = [c for c in CASES if case_id(c[0], c[1]) not in have]
        if not missing:
            print("the register already holds every case of the set")
            return 0
        if register.get("generator_version") != w.GENERATOR_VERSION or register.get("spec_version") != w.SPEC_VERSION:
            print("the generator is not the one that made the register: make the register again instead")
            return 2
        entries, failures = [], []
        for kind, seed, overrides in missing:
            with tempfile.TemporaryDirectory() as scratch:
                entry, problems = generate_and_describe(kind, seed, overrides, scratch)
            entries.append(entry)
            print("%-18s %2d files %6.1f MB  audit %s" % (entry["id"], entry["files"], entry["bytes"] / 1e6, entry["audit_problems"]), flush=True)
            if problems:
                failures.append((entry["id"], problems))
        if failures:
            for name, found in failures:
                print("AUDIT FAILED", name, found[:5])
            return 1
        order = {c[:2]: i for i, c in enumerate(CASES)}
        register["cases"] = sorted(register["cases"] + entries, key=lambda e: order[(e["kind"], e["seed"])])
        register["set_version"] += 1
        register.setdefault("history", []).append("set_version %d (%s): %s" % (register["set_version"], datetime.date.today().isoformat(), args.why))
        with open(args.register, "w", encoding="utf-8", newline="\n") as handle:
            json.dump(register, handle, indent=1, sort_keys=True)
            handle.write("\n")
        print("added %d cases; set_version %d" % (len(entries), register["set_version"]))
        return 0
    if args.command == "verify":
        with open(args.register, encoding="utf-8") as handle:
            register = json.load(handle)
        differences = verify_register(register, tuple(args.kind.split(",")), progress=lambda e: print("checked", e["id"], flush=True))
        for line in differences:
            print("DIFFERS:", line)
        print("the register still describes the generator" if not differences else "%d differences" % len(differences))
        return 1 if differences else 0


if __name__ == "__main__":
    sys.exit(main())
