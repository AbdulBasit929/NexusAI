# BREAKDOWN GOAL — decision rule, written BEFORE the run

`FORENSIC_BREAKDOWN_GOAL`, default **OFF**. Slice 1 of the taxonomy fix.

## Why this is being attempted a second time

WI-31 mapped "breakdown" onto the EXISTING `aggregate` goal. ACC-02 went
CORRECT -> CONFIDENT-WRONG: "show the breakdown of HTTP status codes" answered
"There are 1,000 access log entries in this case". The cause was recorded at the
time and is not a vocabulary problem — `aggregate` with no group-by hints
resolves to shape `scalar` in `s9QuestionShape`, so classifying an explicit
breakdown as `aggregate` ACTIVELY ASSERTS it is a single number.

This change does the opposite: it introduces a goal whose shape is `breakdown`,
which carries a GROUPING OBLIGATION already implemented at
`verification_s9.go` — a plan with no `GroupFields` is REFUSED. The failure
direction therefore inverts. Where WI-31 turned a breakdown into a confident
total, this turns an ungrouped plan into a clarification.

## What the census measured (offline, no model)

`TestGoalTaxonomyCensus` over golden62 + holdout, 75 questions:

    aggregate  31 (41%)   lookup 17 (23%)   rank 12 (16%)   search 8 (11%)
    distinct    3         range   2         source_rows 1   summarize 1

**The 17 on the `lookup` default are SIX different kinds of question:**

    explicit breakdown   CDR-04 IPDR-03 ACC-02 CASE-04      <- THIS SLICE
    MIN/MAX over time    ANPR-03 "first and last seen"      -> `range`, slice 2
    superlative scalar   CDR-15 "longest call"              -> `rank`, slice 3
    genuine row lookup   SUB-03 TWR-02 NEG-02 X-01 H11      -> `lookup` is CORRECT
    media retrieval      DOC-04 DOC-05 IMG-02 IMG-03 VID-01 -> not algebraic
    out of scope         NEG-03 "blood type"                -> neither

**This is why an S9 `rows` case cannot be added first**: it would assert a row
shape over all seventeen, including four breakdowns and five media questions.
The taxonomy has to be split before any shape can be asserted on the remainder.

## The change

Vocabulary: EXPLICIT breakdown language only -- "breakdown" / "break down" --
resolving to a new `breakdown` goal placed ABOVE `aggregate` in
`semanticFrameGoal`. This targets exactly the four questions the census found,
all of them currently on the `lookup` default.

"for each" / "of each" is DELIBERATELY EXCLUDED from this slice. CDR-05 ("how
many calls of each type are there?") currently classifies `aggregate`/`scalar`
and answers CORRECT only because the S9 scalar guard was relaxed to consult the
curated layer. Moving it would change a question that already works, on the same
run that introduces a new goal, and neither effect could then be attributed.
It is slice 4.

A new goal must be registered at FOUR sites or it degrades silently. Each was
read before writing:

    semanticFrameGoal              the vocabulary itself
    semanticGoalMatchesIntent      no mapping => IntentCompatibility 0 =>
                                   deterministicRegisteredMatch REJECTS every
                                   candidate (it exempts only lookup/none)
    deterministicRegisteredMatch   generic_tabular goal list, line 271
    needsIssuedFields              line 968 -- a breakdown MUST reach the
                                   catalogue, or it has no group field to use
    s9QuestionShape                case "breakdown" -> "breakdown"

## THRESHOLD — declared before the measurement

Both suites, `--analyst-text`, live. A CONTROL pass with the switch OFF runs
first on the same container, and the measurement is read against THAT, never
against an older report.

**CONFIGURATION MUST BE ASSERTED FROM THE CONTAINER BEFORE EITHER PASS AND
RECORDED HERE.** The 21:40 deploy silently ran with `FORENSIC_VERIFIED_ONLY` and
`FORENSIC_PLAN_CACHE` at compose's `:-false` defaults, because the env file
defines neither -- which is why CDR-14 crossed the 240s ceiling and why that run
cannot be compared on latency to its own baseline. The control pass exists
precisely so a config difference cannot be mistaken for the goal's effect.

    REVERT if ANY question moves to WRONG on EITHER suite.
    REVERT if net CORRECT decreases on either suite.
    ACCEPT if 0 new WRONG and net CORRECT increases by >= 1.
    ACCEPT-NEUTRAL, keep OFF: 0 new WRONG and net CORRECT unchanged --
        the goal is then structurally right but unproven, and shipping an
        unmeasured-benefit switch ON is how the abstention rate grew.

CORRECT -> CLARIFIED is a LOSS, not a revert trigger on its own, but it counts
against net CORRECT and therefore can only survive if something else converts.

A TIMEOUT (`http: -1`) is NOT a verdict. Re-measure the mover in steady state
before scoring it, both ways.

---

# CONFIGURATION, ASSERTED FROM THE CONTAINER

    FORENSIC_VERIFIED_ONLY=true      FORENSIC_PLAN_CACHE=true
    FORENSIC_IR_ARBITRATION=true     FORENSIC_PLAN_CACHE_DIR=/data/forensic/spool/plan-cache
    FORENSIC_IR_FALLBACK=true        FORENSIC_LADDER_ROUTING=true

Restored from a five-setting drift found immediately before this measurement:
[`CONFIG_DRIFT.md`](../config-drift-20260925/CONFIG_DRIFT.md).

# CONTROL PASS — goal OFF, restored configuration, same container

    47 CORRECT · 14 CLARIFIED · 1 MANUAL      latency median 2568ms

Identical to the documented 17:33 baseline (47/14/1, median 2562ms), which
confirms two things independently of the goal under test:

1. CDR-14's 240s ERROR was the MISSING PLAN CACHE, not a regression. With the
   cache restored it answers correctly: "There are 2 CDR records from 2026-08-01
   up to (not including) 2026-09-01."
2. **Restoring `FORENSIC_VERIFIED_ONLY=true` cost NOTHING on the 62.** The gate
   withholds unverified answers, and the score is the same with it on as the
   drifted run measured with it off. The compiler now answers what the ladder
   used to answer unverified. This is an observation on 62 regression questions,
   NOT a licence to run without the gate.

**This control, not any earlier report, is what the goal-on run is scored
against.**

---

# RESULT — goal ON, same container, same configuration

    golden    47 CORRECT · 14 CLARIFIED · 1 MANUAL     62/62 IDENTICAL VERDICTS
    held-out  10 CORRECT ·  2 CLARIFIED · 1 WRONG      identical
    latency   median 2572ms vs control 2568ms

Switch state asserted from the container before AND after the run.

## VERDICT AGAINST THE PRE-DECLARED THRESHOLD: ACCEPT-NEUTRAL, KEEP OFF

0 new WRONG on either suite; net CORRECT unchanged on both. The rule written
before the run says exactly this case keeps the switch OFF: "the goal is then
structurally right but unproven, and shipping an unmeasured-benefit switch ON is
how the abstention rate grew."

## WHY IT IS NEUTRAL — the ladder answers these questions first

All four target questions are byte-identical with the goal on and off: same
verdict, same template `canonical_records`, same route `records_sql`, same
answer text, and a plan-cache hit (latency within noise), which means THE MODEL
PAYLOAD NEVER CHANGED.

    CDR-04   1906ms -> 1950ms      IPDR-03  4201ms -> 4087ms
    ACC-02   3395ms -> 4514ms      CASE-04  1734ms -> 1784ms

The ladder keys on the same vocabulary:

    query.go:2418  case containsAny(q, []string{"sms", "data", "breakdown"}):
                       return "call_type_breakdown"
    query.go:2262  case containsAny(q, []string{"protocol breakdown", ...}):
                       return "ipdr_protocol_breakdown"

A template is bound before the semantic compiler's goal can influence the plan,
so the goal is correct and currently UNREACHABLE for the questions it targets.

## WHAT THIS CHANGES ABOUT THE ROADMAP

The state file ordered "breakdown taxonomy" FIRST and "delete the ladder"
FOURTH. For these four questions the dependency runs the OTHER WAY: the goal
cannot pay until the ladder stops answering them.

It is still a PREREQUISITE, not wasted: it removes 4 questions from the `lookup`
default, which is the bucket an S9 `rows` case cannot be added to while it holds
six different kinds of question. The goal should be turned ON and measured
TOGETHER with the `rows` case, not before it.

**Keep the code, keep the switch, keep it off.** Re-measure when either the
ladder stops routing these or the `rows` case lands — and re-measure both
together, because neither can be attributed alone.
