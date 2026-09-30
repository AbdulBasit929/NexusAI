# Ladder deletion — slicing rule, WRITTEN BEFORE THE WORK

Date: 2026-09-25 · Owner: claude
State at writing: 47 CORRECT · 14 CLARIFIED · 1 MANUAL · 0 WRONG · p95 5.2 s.

## Why this is sliced, and why slice 1 deletes nothing

`query.go` is 9,041 lines. The ladder — `choosePreciseTemplate` (253 lines of
`containsAny(q, [...]) -> template`) plus `chooseGenericFallbackTemplate` — is
P3's remaining deliverable. Cutting 250+ lines of routing in one commit and
hoping the suite catches it is exactly the move that produced seven
confident-wrong answers across five attempts earlier in this session.

**The safe order for removing code this size is DISABLE → MEASURE → CUT.**

Slice 1 therefore deletes nothing. It puts the ladder behind a switch, default
ON (current behaviour, bit-for-bit), and measures the corpus with it OFF. Only
once the measurement says what is lost does any code get removed.

## What makes this safe

`chooseTemplate` already treats an abstaining ladder as a normal outcome:

    // The ladder abstains; the semantic compiler / dynamic SQL path decides.
    return ""

That fall-through is designed, present and exercised today. Turning the ladder
off drives every question down a path the code already supports, rather than
into an unhandled state.

## What the corpus can and cannot tell us

`scripts/nexusai_route_census.py` on the closing run:

    compiler (typed plan)   39  (36 CORRECT)   survives by construction
    media / derived-text    18  (10 CORRECT)   a different executor
    everything else          5                 the risk surface
                            CASE-01/CLAR TWR-02/CLAR CASE-03/CLAR IMG-03/CLAR NEG-03/CORR

**Honest limit: the 62 CANNOT fully validate this deletion.** The ladder's whole
premise was covering phrasings nobody wrote down in advance, so the corpus
under-represents exactly what it does. A clean run is necessary, not sufficient.
That is an argument for the switch staying available after the cut, not for
skipping the measurement.

## PRE-DECLARED THRESHOLDS — binding, slice 1

- **Any question moves to WRONG → the ladder stays ON.** Seventh time this
  clause is written this session; it has fired twice and been honoured twice.
- **Any currently-CORRECT question is lost → the ladder stays ON**, and the lost
  question is characterised before any retry.
- **0 WRONG and 0 CORRECT lost → the ladder ships OFF by default**, and slice 2
  deletes the dead code.
- Latency must not regress beyond the P3 gate (p95 < 15 s on the warm path).

## Slices

    1  gate the ladder behind FORENSIC_LADDER_ROUTING, default ON. Measure OFF.   <- this one
    2  if slice 1 is clean: flip the default OFF, re-measure, then DELETE
       choosePreciseTemplate and chooseGenericFallbackTemplate
    3  remove the template catalogue entries no longer reachable, with the census
       proving reachability before and after
    4  remove `templateServesFamily` and the guards that exist only to bound the
       ladder

Each slice gets its own run and its own before/after census. No slice starts
until the previous one is measured.

## Not in scope

The media / derived-text executors (`sql_deterministic`,
`cross_family_exact_sql`) are NOT the ladder and must survive untouched — 18
questions, 10 of them CORRECT, depend on them. Conflating the two is the
specific mistake the census exists to prevent.
