# TWR-02 shape narrowing — MEASURED DEAD, NOT BUILT

Date: 2026-09-25 · Owner: claude · No code changed, nothing deployed.

The plan was: for a `lookup` goal, withhold measures so a "where is X" question
must project fields instead of counting. Two independent measurements killed it
before implementation, and a third finding underneath explains both.

## 1. The `lookup` goal does not mean what the narrowing needed it to mean

Probed directly against `extractSemanticFrame` / `s9QuestionShape`:

    CDR-04    goal=lookup  shape=rows   "Show the call type breakdown"
    CDR-15    goal=lookup  shape=rows   "What was the longest call?"
    IPDR-03   goal=lookup  shape=rows   "Break down IPDR sessions by protocol"
    ANPR-03   goal=lookup  shape=rows   "When was LHR-2026 first and last seen?"
    ACC-02    goal=lookup  shape=rows   "Show the breakdown of HTTP status codes"
    TWR-02    goal=lookup  shape=rows   "Where is tower PK-LHR-SYN-001 located?"

The first five are CORRECT today and every one of them **requires measures** — a
grouped COUNT, a MAX, a MIN/MAX pair. Withholding measures for this goal would
break six currently-correct answers to convert one clarification. The pattern is
the same one that produced four regressions in the prompt attempt: a signal
assumed to be clean, used without being measured.

## 2. TWR-02 does not currently fail on shape anyway

    generator_dynamic_validation_rejected:
      "source-native filter value was not supplied by the analyst"

The model emits `tower.site_location CONTAINS "where"` and
`tower.district CONTAINS "area"` beside the correct `site_code` filter — the
INVENTED-FILTER signal. Shape never gets a chance to matter. Converting TWR-02
therefore needs that signal removed, which is forbidden on two independent
measurements: WI-23 (dropping invented filters post-hoc: 2 confident-wrong) and
WI-27 (instructing the model not to produce them: 4 confident-wrong, including a
negative control turning into a false positive).

**TWR-02 and CDR-11 are the same blocked case.** Both were recorded as distinct
defects — field selection and shape — and both are actually the invented-filter
signal doing its job.

## 3. The root cause, and a verification gap that cannot be closed yet

`s9QuestionShape` maps goal `lookup` to shape `rows`, and
`verifySourceNativePlanShape` has cases for `rank`, `scalar` and `breakdown` —
**but no case for `rows`.** A question demanding rows gets no shape check at
all, so a scalar count can answer it silently.

That gap is real and worth closing. It **cannot be closed today**: a `rows` rule
requiring a projection would refuse the same six correct answers, because the
goal that feeds it is mis-assigned. The frame calls an explicit breakdown a
lookup.

**The defect is in goal classification, not in S4 or S9.** Fixing it touches the
compiler's core signal, which every downstream check reads, and it is its own
work item with its own measurement — not a tail-end change after a long session.

## Recommendation

Do not narrow on `lookup`. Do not add an S9 `rows` case until goal
classification is corrected and re-measured. Both are recorded so the next agent
does not re-derive them from scratch — and so neither is attempted as an
"obvious" improvement.
