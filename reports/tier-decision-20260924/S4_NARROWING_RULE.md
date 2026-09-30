# S4 narrowing — decision rule, WRITTEN BEFORE THE WORK

Date: 2026-09-24 · Owner: claude · Follows STEP 1's revert (4 confident-wrong).

## What S4 can and cannot reach

S4 narrows what the generator is OFFERED. Measured against the four remaining
bucket-A failures, it reaches some and not others, and saying so first prevents
the change being credited with more than it does:

    CDR-08   aggregate selection: plain COUNT where COUNT_DISTINCT is required   REACHABLE
    ANPR-05  same shape, same cause                                              REACHABLE
    CDR-11   FIELD selection: cdr.originating_number instead of cdr.msisdn       reachable, NOT attempted
    TWR-02   SHAPE: a measure where a projection is required                     reachable, NOT attempted

Only the first two are attempted here. The other two are recorded below with
why they are being left alone.

## The signal, measured before use

`frame.Goal == "distinct"` across the full 62:

    distinct   2   CDR-08, ANPR-05

**Two questions, zero false positives on the other sixty.** It is also a signal
the system ALREADY treats as authoritative: S9 refuses a plain COUNT whenever
the goal is `distinct`, which is exactly why these two clarify today.

## The change

When the frame's goal is `distinct`, the issued measure schema stops offering
the row-count variant and offers `COUNT_DISTINCT` over the distinct-capable
issued fields. The grammar then makes the correct plan the only expressible one.

This is the keystone applied to the AGGREGATE rather than to the field, and it
is consistent with S9 rather than a new judgement: **everything the narrowing
forbids, S9 already refuses.** The narrowing stops the generator spending a
60-150 s generation on a plan that was going to be rejected.

S9's obligation is NOT removed. It remains as defence in depth; a schema and a
verifier agreeing is the point.

## PRE-DECLARED THRESHOLDS — binding

- **Any question moves to WRONG → REVERT IMMEDIATELY, at any ratio.** Fourth
  time this clause is written this session; it has fired once and was honoured.
- **Both CDR-08 and ANPR-05 convert to CORRECT, 0 lost → KEEP.**
- **1 converts, 0 lost → KEEP** and record the other's remaining cause.
- **0 convert, 0 lost → REVERT.** A narrowing that changes nothing is added
  surface on the hottest path in the product.
- **Any currently-CORRECT question is lost → REVERT**, regardless of conversions.

## Not attempted, and why

**CDR-11 (field selection).** The fix would be removing `cdr.originating_number`
from the issued set for questions about what a number DID. 21 questions carry
the `aggregate` goal and several are CORRECT today; narrowing a field set that
broadly, to fix one question, is the shape of change that produced STEP 1's four
regressions. It needs its own measurement, not a ride on this one.

**TWR-02 (shape).** Would mean not offering measures for a `lookup` goal. 16
questions carry that goal and most are CORRECT. Same reasoning.

Both are separable work items with their own thresholds. Bundling them here
would make an unattributable result — which is precisely what made STEP 1's
four regressions expensive to read.
