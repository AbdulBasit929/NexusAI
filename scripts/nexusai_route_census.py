#!/usr/bin/env python3
"""Census of HOW each golden question was answered, from a recorded run.

Phase 3's remaining deliverable is deleting the template catalogue and the
`query.go` routing ladder. That is the largest open change on the backend track
and it cannot be planned from the verdict column alone: a question can be
CORRECT because the compiler produced a typed plan, or CORRECT because a
template answered it, and only the second kind is at risk from the deletion.

So this reads a completed run and reports, per question, the execution strategy
that produced the answer. Run it BEFORE the deletion to know what is at risk,
and AFTER to prove nothing that mattered was lost.

    python scripts/nexusai_route_census.py reports/phase3-close-20260925
    python scripts/nexusai_route_census.py reports/<before> reports/<after>

No model calls, no database, no network. It only reads artifacts already on
disk, so it costs nothing to run and can be run as often as needed.

Measured 2026-09-25 on reports/phase3-close-20260925 (47 CORRECT · 14 CLARIFIED
· 1 MANUAL · 0 WRONG):

    bounded_source_native_algebra  39  (36 CORRECT)   the compiler's typed plan
    sql_deterministic              16  ( 9 CORRECT)   mostly media/derived-text
    cross_family_exact_sql          2  ( 1 CORRECT)
    canonical_sql_query             2  ( 0 CORRECT)
    clarification                   2  ( 0 CORRECT)

**63% of the corpus already answers from the compiler**, and the largest
non-compiler group is the media/derived-text executors, which are a DIFFERENT
path from the ladder and must survive the deletion untouched.

WHAT THIS CENSUS CANNOT TELL YOU — measured the hard way, 2026-09-25.

Read as "the risk surface is 5 questions", this census UNDER-PREDICTED BADLY.
Disabling the ladder produced SIX regressions including THREE confident-wrong
answers, and most were not on the predicted surface at all:

    CASE-01  CLARIFIED -> WRONG       X-01  CLARIFIED -> WRONG
    TWR-02   CLARIFIED -> WRONG       CDR-12, CDR-16, DOC-02  CORRECT -> CLARIFIED

The reason is a limit in what `execution_strategy` records. It names the
executor that PRODUCED the answer; it does not capture what the ladder
contributed UPSTREAM. `chooseTemplate` binds `req.Template`, and that template
then scopes family and record type for the compiler. TWR-02 went CLARIFIED ->
WRONG **while still reporting `bounded_source_native_algebra`**: the compiler
ran either way, but without the template's scoping it ran wider and asserted
something false.

So: a question answered by the compiler is NOT thereby independent of the
ladder. Use this census to see the shape of the corpus and to diff two runs —
its before/after comparison is sound and is what caught the six regressions.
Do NOT use its "risk surface" line as a prediction of what a deletion costs.
"""

import collections
import io
import json
import os
import sys

# Strategies that are the COMPILER answering, not a template.
COMPILER = {"bounded_source_native_algebra"}

# Strategies served by the governed media / derived-text executors. These are
# not the keyword ladder and must survive its deletion; conflating them with it
# is the mistake this census exists to prevent.
MEDIA_OR_TEXT = {"sql_deterministic", "cross_family_exact_sql"}


def census(run_dir):
    """Return {question_id: (verdict, execution_strategy)} for one run."""
    results_path = os.path.join(run_dir, "results.json")
    with io.open(results_path, encoding="utf-8") as handle:
        results = {row["id"]: row for row in json.load(handle)}

    out = {}
    for qid, row in results.items():
        raw_path = os.path.join(run_dir, "raw", qid + ".json")
        strategy = "(no raw response)"
        if os.path.exists(raw_path):
            with io.open(raw_path, encoding="utf-8") as handle:
                payload = json.load(handle)
            plan = payload.get("query_plan") or {}
            strategy = str(plan.get("execution_strategy") or "(none)")
        out[qid] = (row.get("verdict", "?"), strategy)
    return out


def report(run_dir, data):
    counts = collections.Counter()
    correct = collections.Counter()
    members = collections.defaultdict(list)
    for qid, (verdict, strategy) in sorted(data.items()):
        counts[strategy] += 1
        members[strategy].append((qid, verdict))
        if verdict == "CORRECT":
            correct[strategy] += 1

    print("=== %s : %d questions ===" % (run_dir, len(data)))
    for strategy, total in counts.most_common():
        print("  %-32s %2d  (CORRECT %2d)" % (strategy, total, correct[strategy]))

    compiler = sum(n for s, n in counts.items() if s in COMPILER)
    media = sum(n for s, n in counts.items() if s in MEDIA_OR_TEXT)
    other = len(data) - compiler - media
    print()
    print("  compiler (typed plan)        %2d  survives the deletion by construction" % compiler)
    print("  media / derived-text         %2d  DIFFERENT executor; must survive untouched" % media)
    print("  everything else              %2d  <- the deletion's actual risk surface" % other)

    print()
    print("  risk surface, by question:")
    for strategy, rows in sorted(members.items()):
        if strategy in COMPILER or strategy in MEDIA_OR_TEXT:
            continue
        print("    %-30s %s" % (strategy, " ".join("%s/%s" % (q, v[:4]) for q, v in rows)))
    print()
    print("  NOTE: this line is NOT a prediction of what a deletion costs. It names the")
    print("  executor that produced each answer, not what the ladder contributed upstream")
    print("  via req.Template's family/record-type scoping. Measured 2026-09-25: disabling")
    print("  the ladder caused 6 regressions, 3 of them confident-wrong, mostly OFF this")
    print("  surface. Use the before/after diff to measure; use this only to see shape.")


def compare(before_dir, after_dir):
    before, after = census(before_dir), census(after_dir)
    changed = []
    for qid in sorted(set(before) | set(after)):
        was = before.get(qid, ("(absent)", "(absent)"))
        now = after.get(qid, ("(absent)", "(absent)"))
        if was != now:
            changed.append((qid, was, now))

    print()
    print("=== before -> after ===")
    if not changed:
        print("  no question changed verdict or execution strategy")
        return 0
    regressed = 0
    for qid, (wv, ws), (nv, ns) in changed:
        flag = ""
        if wv == "CORRECT" and nv != "CORRECT":
            flag, regressed = "   <-- LOST A CORRECT ANSWER", regressed + 1
        if nv == "WRONG":
            flag, regressed = "   <-- CONFIDENT WRONG", regressed + 1
        print("  %-9s %-10s %-30s -> %-10s %-30s%s" % (qid, wv, ws, nv, ns, flag))
    print()
    print("  regressions: %d" % regressed)
    return regressed


def main(argv):
    if len(argv) == 2:
        run = argv[1]
        report(run, census(run))
        return 0
    if len(argv) == 3:
        before, after = argv[1], argv[2]
        report(before, census(before))
        print()
        report(after, census(after))
        return 1 if compare(before, after) else 0
    print(__doc__)
    return 2


if __name__ == "__main__":
    sys.exit(main(sys.argv))
