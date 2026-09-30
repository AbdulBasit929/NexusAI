# MEDIA LAYER VISIBILITY — threshold declared BEFORE the run

## What is being measured, and what is NOT

The deploy ships eight new media entities in `semantic_layer/**` and the executor
binding that can read them. `FORENSIC_DERIVED_ARTIFACT_EXECUTION` is **OFF**, so
no derived row can be read.

**So this run measures ONE thing: whether making 8 media entities VISIBLE TO THE
COMPILER changes plans for the 62 structured questions.** It does not measure any
media capability — with the switch off, a derived plan is refused.

This is the risk worth measuring on its own. Twice before, changing the issued
vocabulary changed plans for questions that already worked:

    narrowing the issued FIELD set   a top-12 cut made CDR-11 unanswerable at
                                     any model quality
    withholding uncurated fields     stopped a field substitution AND cost TWR-02
                                     a false negative, "there are no tower
                                     records in this case"

**The enum is not a menu.** Eight new families is a larger vocabulary change than
either of those.

## Baseline

The Stage 0 run, re-graded with the corrected oracle so both sides are scored by
the same instrument:

    45 CORRECT · 2 NOT_STATED · 14 CLARIFIED · 1 MANUAL

The 2 NOT_STATED are CDR-10 and ANPR-03, both on the template path, both known.

## Configuration, asserted from the container before scoring

    FORENSIC_DERIVED_ARTIFACT_EXECUTION=false   <- the point of this run
    FORENSIC_ANSWER_STATES_VALUES=true
    FORENSIC_VERIFIED_ONLY=true
    FORENSIC_IR_ARBITRATION=true
    FORENSIC_IR_FALLBACK=true
    FORENSIC_PLAN_CACHE=true
    FORENSIC_LADDER_ROUTING=true
    FORENSIC_BREAKDOWN_GOAL=false

## THRESHOLD

    REVERT THE LAYER VISIBILITY if ANY question moves to WRONG on either suite.
    REVERT if ANY question moves CORRECT -> NOT_STATED or CORRECT -> CLARIFIED.
    ACCEPT if 62/62 and 13/13 verdicts are unchanged.

**Reverting means removing the media entity files from the shipped layer, not
just flipping the execution switch** — the switch does not hide them from the
compiler, and that is precisely what is under test.

A partial move (one question CORRECT -> CLARIFIED) still reverts. A structured
question losing an answer because a media family competed for it is the same
false-negative class as TWR-02, and abstention bought with a real answer is not
a trade this product makes.

## Not a verdict

A TIMEOUT (`http: -1`) is not a verdict. The plan cache is on and the payloads
for structured questions should be unchanged, so a cache MISS on a structured
question is itself a finding: it would mean the issued field set changed.
Re-measure any mover in steady state before scoring it.

---

# RESULT — ACCEPT. The media entities do not disturb the 62.

Configuration asserted from the container, `DERIVED_ARTIFACT_EXECUTION=false`.
Layer snapshot measured: [`layer_snapshot_measured.txt`](layer_snapshot_measured.txt).

    golden    45 CORRECT · 2 NOT_STATED · 14 CLARIFIED · 1 MANUAL   62/62 IDENTICAL
    held-out  10 CORRECT · 2 CLARIFIED · 1 WRONG                    13/13 IDENTICAL

**No verdict moved on either suite.** Nothing to revert.

## The latency evidence is the stronger half of this result

    median            2681ms -> 2502ms
    slowest question  5324ms (CDR-14), nothing within two orders of the ceiling

**Every structured question hit the plan cache.** That was declared as the check
before the run, and it is what actually proves the point: the cache is keyed on a
hash of the exact model payload, so a HIT means the payload was byte-identical,
which means **the issued field set did not change**. Eight new families entered
the layer and not one of them reached a structured question's enum.

Verdicts alone could not have established that — two different payloads can
produce the same verdict. The cache turns "the answers look the same" into "the
question the model was asked was the same".

## What this does NOT license

**The switch stays OFF.** This run measured layer visibility only; with execution
off no derived row was read, so no media capability was measured at all.

**A re-measure is owed.** Codex was mid-WI-LAYER-4 during this run (layer files
written 15 minutes before the build). That item REBINDS SYNONYMS, and synonyms
drive family selection and field binding -- precisely the mechanism this run
found to be quiet. The hashes above record exactly what was measured; when
WI-LAYER-4 lands, this run repeats against the same threshold.
