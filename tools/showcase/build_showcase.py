#!/usr/bin/env python3
"""Builds a reference case into the showcase folder (docs/roadmap/reference-cases-programme-20261028.md, stage S1): the case's files in the shapes of the team lead's folder, the truth, and a catalogue of
questions each with the oracle's answer.

    python -X utf8 tools/showcase/build_showcase.py orion                 # build C:\\NexusAI-Showcase\\orion (refuses an existing folder)
    python -X utf8 tools/showcase/build_showcase.py orion --check         # rebuild into a temporary folder and compare every hash with the built case (reproducibility)
    python -X utf8 tools/showcase/build_showcase.py --list

The folder is outside the repository (NEXUSAI_SHOWCASE overrides the place). Everything in it is SYNTHETIC: every number, address, plate, account, name and national ID number is of a reserved form
(tools/real_data/reserved.py); no value of the real folder is in it. What it holds, per case:
    files\\                 the evidence: target call records in the operator layouts C1 to C4, tower dumps C5 to C8, internet records I1 to I4, application usage, camera sightings, subscriber lists of two
                           sources, the device reference list, bank statements and a wallet list
    key\\                   the truth (people, plants, oracle): never given to the engine
    questions\\             catalogue.json: every question with its oracle answer and its judge kind, by bank and type
    manifest.json          SHA-256 of every file of the case as generated; showcase-manifest.json also covers the catalogue
    README.txt             what it is
"""
import argparse
import collections
import datetime
import hashlib
import json
import os
import shutil
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.abspath(os.path.join(HERE, "..", ".."))
for sub in ("tools/case_generator", "tools/ask_eval", "tools/real_data"):
    sys.path.insert(0, os.path.join(REPO, *sub.split("/")))
sys.path.insert(0, REPO)

ROOT = os.environ.get("NEXUSAI_SHOWCASE", "C:/NexusAI-Showcase")
CASES = {
    # Orion: the full case. 12 subjects of interest in 5 communities over 75 days, 100 people in the world, every plant of the generator.
    "orion": dict(seed=5001, people=100, subjects=12, associates=24, period_days=75, communities=5, p2_pairs=5, p3_pairs=2, p7_plants=3),
    # Orion in Lahore (reference cases programme, section 9): the same world and the same plants, drawn on a real city with Pakistani names, addresses and narrations
    "orion-lahore": dict(seed=5001, people=100, subjects=12, associates=24, period_days=75, communities=5, p2_pairs=5, p3_pairs=2, p7_plants=3, locale="lahore", id_prefix="ORL"),
}
IDENTITY_NOTES = {
    "reserved": "Every number, address, plate, account, name and national ID number in this folder is invented and of a reserved form (it cannot be a real person's).",
    "locale": "Every number, address, plate, account and national ID number in this folder is invented and of a reserved form (the national ID numbers begin 00000, a code no province or district has; the accounts are PK00SYNT plus digits, an invalid IBAN). Names are common Pakistani combinations of a given name and a surname and CAN coincide with a real person's; they are not taken from any record. Cell sites are synthetic positions drawn on a real city, not real towers. Nothing here is a real person's record.",
}
README = """NexusAI reference case: %(name)s

SYNTHETIC. %(identity_note)s Built by
tools/showcase/build_showcase.py (seed %(seed)d, generator %(generator)s) of the repository; rebuilding from the seed gives the same bytes.

  files\\       the evidence, in the shapes of the files of the team lead's folder (call records C1 to C4, tower dumps C5 to C8, internet records I1 to I4) and the families built since
  key\\         the truth the generator planted and the oracle's answers: NEVER give these to the engine or the interface
  questions\\   catalogue.json: %(count)d questions with their oracle answers (the judge kind of each says how to score it)
  manifest.json, showcase-manifest.json   SHA-256 of every file

To load this case into a THROWAWAY database and put the questions to the Ask engine, see docs/roadmap/reference-cases-programme-20261028.md and tools/showcase/README.md. Nothing in this folder is
uploaded to the live service without the owner's explicit go.
"""


def sha256(path):
    digest = hashlib.sha256()
    with open(path, "rb") as handle:
        for chunk in iter(lambda: handle.read(1 << 20), b""):
            digest.update(chunk)
    return digest.hexdigest()


def _questions_of(result):
    """A bank returns its questions, or the questions and what it also computed (pair_events_bank.build)."""
    return result[0] if isinstance(result, tuple) else result


def collect_questions(params, key):
    """Every bank that exists for the families the case holds, each with its oracle. Returns [(bank, [question])]."""
    import anpr_bank
    import build_questions
    import doc_bank
    import pair_events_bank
    import stage3a_bank
    import stage3bc_bank
    import subscriber_bank

    banks = [("calls", build_questions.build(key, bank=True)), ("colocation", stage3a_bank.bank_questions(params)), ("pairs", _questions_of(pair_events_bank.build(params)))]
    world = stage3bc_bank.World(params)
    banks.append(("successors", stage3bc_bank.bank_3b(world, params.seed)))
    banks.append(("movement", stage3bc_bank.bank_3c(world, params.seed)))
    if key.get("anpr"):
        banks.append(("plates", anpr_bank.bank_questions(key["anpr"], params.seed)))
    if key.get("subscriber_truth"):
        truth, events = subscriber_bank.build_truth(params)
        banks.append(("registrations", subscriber_bank.bank_questions(truth, events, params.seed)))
    if key.get("document_truth"):
        banks.append(("documents", doc_bank.bank_questions(key["document_truth"], key, params.seed)))
    return banks


def build(name, out_root, force=False, folder=None):
    import generate
    import world as w

    spec = dict(CASES[name])
    prefix = spec.pop("id_prefix", "ORI")
    params = w.Params(**spec)
    target = os.path.join(out_root, folder or name)
    if os.path.exists(target) and not force:
        raise SystemExit("%s exists; nothing is overwritten (use --check to compare, or remove it by hand)" % target)
    os.makedirs(out_root, exist_ok=True)
    work = tempfile.mkdtemp(prefix=".build-%s-" % name, dir=out_root)
    try:
        root = generate.write_case(params, work, with_anpr=True, with_subscribers=True, with_transactions=True, with_documents=bool(params.locale), with_statements=bool(params.locale))
        with open(os.path.join(root, "key", "key.json"), encoding="utf-8") as handle:
            key = json.load(handle)
        catalogue, per_bank = [], collections.OrderedDict()
        for bank, questions in collect_questions(params, key):
            per_bank[bank] = collections.Counter(q["type"] for q in questions)
            for q in questions:
                catalogue.append(dict(q, id="%s-%s-%s" % (prefix, bank[:3].upper(), q["id"]), bank=bank, source_id=q["id"]))
        os.makedirs(os.path.join(root, "questions"), exist_ok=True)
        with open(os.path.join(root, "questions", "catalogue.json"), "w", encoding="utf-8", newline="\n") as handle:
            json.dump(catalogue, handle, indent=1, ensure_ascii=False)
            handle.write("\n")
        with open(os.path.join(root, "README.txt"), "w", encoding="utf-8", newline="\n") as handle:
            import world as world_module
            handle.write(README % {"name": name, "seed": params.seed, "generator": world_module.GENERATOR_VERSION, "count": len(catalogue), "identity_note": IDENTITY_NOTES["locale" if params.locale else "reserved"]})
        files = {}
        for base, _, names in os.walk(root):
            for item in sorted(names):
                full = os.path.join(base, item)
                files[os.path.relpath(full, root).replace(os.sep, "/")] = {"sha256": sha256(full), "bytes": os.path.getsize(full)}
        with open(os.path.join(root, "showcase-manifest.json"), "w", encoding="utf-8", newline="\n") as handle:
            json.dump({"case": name, "seed": params.seed, "built": datetime.datetime.now().isoformat(timespec="seconds"), "files": dict(sorted(files.items()))}, handle, indent=1)
            handle.write("\n")
        if os.path.exists(target):
            shutil.rmtree(target)
        shutil.move(root, target)
        return target, catalogue, per_bank, files
    finally:
        shutil.rmtree(work, ignore_errors=True)


def summarise(target, catalogue, per_bank, files):
    by_ext = collections.Counter(os.path.splitext(name)[1] or "(none)" for name in files)
    print("built %s: %d files (%s), %d questions" % (target, len(files), ", ".join("%d %s" % (n, e) for e, n in by_ext.most_common()), len(catalogue)))
    for bank, kinds in per_bank.items():
        print("  %-14s %3d  %s" % (bank, sum(kinds.values()), dict(kinds)))
    controls = sum(1 for q in catalogue if q["expect"].get("control") or q["expect"].get("kind") in ("absent", "none", "refuse", "probe"))
    print("  controls and probes: %d (%.0f%%)" % (controls, 100.0 * controls / max(1, len(catalogue))))


def main(argv):
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("case", nargs="?")
    parser.add_argument("--check", action="store_true", help="rebuild into a temporary folder and compare every hash with the built case")
    parser.add_argument("--list", action="store_true")
    parser.add_argument("--as", dest="folder", help="the folder name under the showcase root when it is not the case name (a build never overwrites)")
    args = parser.parse_args(argv)
    if args.list or not args.case:
        for name, spec in CASES.items():
            print("%s  %s" % (name, spec))
        return 0
    if args.case not in CASES:
        raise SystemExit("unknown case %r" % args.case)
    if args.check:
        built = os.path.join(ROOT, args.folder or args.case, "showcase-manifest.json")
        with open(built, encoding="utf-8") as handle:
            wanted = {k: v["sha256"] for k, v in json.load(handle)["files"].items() if k != "showcase-manifest.json"}
        scratch = tempfile.mkdtemp(prefix="showcase-check-")
        try:
            target, _, _, files = build(args.case, scratch, folder=args.folder)
            got = {k: v["sha256"] for k, v in files.items() if k != "showcase-manifest.json"}
        finally:
            shutil.rmtree(scratch, ignore_errors=True)
        different = sorted(k for k in set(wanted) | set(got) if wanted.get(k) != got.get(k))
        print("REPRODUCIBLE" if not different else "DIFFERENT: %s" % different[:10])
        return 0 if not different else 1
    target, catalogue, per_bank, files = build(args.case, ROOT, folder=args.folder)
    summarise(target, catalogue, per_bank, files)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
