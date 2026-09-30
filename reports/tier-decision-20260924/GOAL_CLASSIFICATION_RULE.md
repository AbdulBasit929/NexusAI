# Goal classification — decision rule, WRITTEN BEFORE THE WORK

Date: 2026-09-25 · Owner: claude
State at writing: 47 CORRECT · 14 CLARIFIED · 1 MANUAL · 0 WRONG · abstention 22.6%.

## THIS IS A SAFETY ITEM. IT CANNOT CLOSE THE ABSTENTION GATE.

Stated first so the result is not read as a gate attempt. CDR-11 and TWR-02 —
the only two conversions that would reach ≤20% — are blocked by the
invented-filter guard, which two independent measurements say must not be
removed. **Their goal is irrelevant to that.** A correct goal classification
converts nothing. It closes a verification hole.

## The hole, measured

`semanticFrameGoal` returns `lookup` from its `default:` branch. `lookup` does
not mean "the analyst wants a lookup" — it means **"we did not recognise this
question"**. `s9QuestionShape` then maps it to shape `rows`, and
`verifySourceNativePlanShape` has no `rows` case.

    unclassified (default branch):            16 of 62  (26%)
    of those, CORRECT today with NO shape check:  8

    CDR-04   "Show the call type breakdown"          an explicit BREAKDOWN
    IPDR-03  "Break down IPDR sessions by protocol"  an explicit BREAKDOWN
    ACC-02   "Show the breakdown of HTTP status codes"
    CASE-04  "Explain the call type breakdown"
    CDR-15   "What was the longest call?"            a magnitude SUPERLATIVE
    ANPR-03  "When was LHR-2026 first and last seen?"
    NEG-02, NEG-03

A quarter of the corpus bypasses shape verification. That is the same class of
gap that produced D2 and DOC-04 — a check that does not run cannot catch
anything.

The cause is vocabulary, not architecture: "breakdown" and "break down" appear
in NO case of the switch, and the rank case lists "largest", "greatest",
"highest" but not "longest" or "shortest". `semanticFrameMeasure` already maps
"longest" to `max`, so the measure was right while the goal was not.

## Scope: ONE change, not two

**IN:** add the missing vocabulary so these questions classify positively —
"breakdown"/"break down" to the aggregate case, "longest"/"shortest" to the rank
case. Switch ordering is load-bearing: the aggregate case sits AFTER rank, so a
question carrying both "most" and "breakdown" stays a ranking.

**OUT, deliberately:** adding the S9 `rows` case. It depends on the goals being
right first, and bundling them would make the result unattributable — the exact
mistake that made STEP 1's four regressions expensive to read. It becomes safe
to consider only after this lands and is measured.

## Risk, stated plainly

`frame.Goal` is read by operation ranking, plan compilation (`lookup` and
`source_rows` build a PROJECTION at
`deterministic_semantic_compiler.go:719`), S9 shape, and S4 narrowing. This is
the widest-blast-radius change attempted in this session, on the hottest path,
**with no gate upside.** Five questions that are CORRECT today will change goal.

That is why the thresholds below are stricter than any used so far.

## PRE-DECLARED THRESHOLDS — binding

- **Any question moves to WRONG → REVERT IMMEDIATELY, at any ratio.** Sixth time
  written this session; fired once, honoured.
- **Any currently-CORRECT question is lost → REVERT**, with ONE exception,
  qualified below.
- **0 WRONG and 0 CORRECT lost → KEEP**, on the safety argument: questions that
  answered with no shape verification now have it available.
- A CORRECT that becomes CLARIFIED is a LOSS and triggers revert, **unless** the
  clarification is shown to be correct behaviour on inspection of the plan. That
  exception must be argued from the recorded plan, not assumed, and if it is
  used at all it is stated as a judgement call rather than a threshold pass.

## Verification order

1. Offline: goal for all 62 before and after. Any question changing goal is
   named before any live run.
2. Full suite green.
3. Live 62 with `--analyst-text`.

A goal change on a question outside the eight named above is a surprise and
stops the work for re-examination.
