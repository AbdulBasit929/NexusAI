#!/usr/bin/env python3
"""Keeps the earlier live answers that still belong to a question of a rebuilt verified bank, and drops the rest.

    python -I -X utf8 tools/showcase/carry_results.py OLD_BANK.json NEW_BANK.json RESULTS.jsonl [RESULTS.jsonl ...] --out CARRIED.jsonl

The bank is rebuilt from the live database whenever the case changes (a file is added, a plate is read), and a question keeps its id while its wording or its expected answer may change (the number of
evidence files, a ranking that now shows a tie). An answer given to the old question must not be scored against the new one. This keeps, from the result files (later files win, as in the judge), only the
answers of questions whose wording AND expected answer are identical in both banks; every other question has to be asked again. Prints how many were carried, how many dropped as changed, and which ids.
"""
import argparse
import json
import sys


def load_bank(path):
    with open(path, encoding="utf-8") as handle:
        return {q["id"]: q for q in json.load(handle)["questions"]}


def load_results(paths):
    latest = {}
    for path in paths:
        with open(path, encoding="utf-8") as handle:
            for line in handle:
                if line.strip():
                    row = json.loads(line)
                    latest[row["id"]] = row
    return latest


def same_question(old, new):
    return old is not None and new is not None and old["question"] == new["question"] and old["expect"] == new["expect"]


def carry(old_bank, new_bank, results):
    """(carried result rows, ids dropped because the question changed, ids dropped because the question is gone)"""
    carried, changed, gone = [], [], []
    for qid, row in results.items():
        if qid not in new_bank:
            gone.append(qid)
        elif same_question(old_bank.get(qid), new_bank[qid]):
            carried.append(row)
        else:
            changed.append(qid)
    return carried, sorted(changed), sorted(gone)


def main(argv):
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("old_bank")
    ap.add_argument("new_bank")
    ap.add_argument("results", nargs="+")
    ap.add_argument("--out", required=True)
    args = ap.parse_args(argv)
    old_bank, new_bank = load_bank(args.old_bank), load_bank(args.new_bank)
    carried, changed, gone = carry(old_bank, new_bank, load_results(args.results))
    with open(args.out, "w", encoding="utf-8", newline="\n") as handle:
        for row in carried:
            handle.write(json.dumps(row, ensure_ascii=False) + "\n")
    new_only = sorted(set(new_bank) - set(old_bank))
    print("carried %d earlier answers; %d dropped because the question changed %s; %d dropped because it is gone; %d questions are new and %d have no earlier answer"
          % (len(carried), len(changed), changed, len(gone), len(new_only), len(set(new_bank) - {r['id'] for r in carried})))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
