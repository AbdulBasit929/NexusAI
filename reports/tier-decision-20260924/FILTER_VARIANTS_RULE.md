# Filter variants — decision rule, WRITTEN BEFORE THE WORK

Date: 2026-09-25 · Owner: claude · Follows WI-28 (S4 distinct narrowing, kept).
State at writing: 47 CORRECT · 14 CLARIFIED · 1 MANUAL · 0 WRONG.

## The diagnosis was re-checked, and it had gone stale

CDR-11 was recorded as a FIELD-SELECTION failure — the generator filtering on
`cdr.originating_number` instead of `cdr.msisdn`. That was measured before the
measure-variant and S4 changes. Re-read from the current run:

    generator_dynamic_validation_rejected: "null filters cannot carry values"

A different defect entirely. The plan carries `IS_NULL` on a field **together
with a value**, which `validateSourceNativePlan` refuses. Designing a field
narrowing against the old observation would have been building for a failure
that no longer happens.

## Field narrowing is NOT the fix, and the repository already measured that

`retrieveSourceNativeFields` deliberately issues EVERY curated field for the
family. Its comment records why: a fixed top-12 cut `cdr.msisdn` — tenth
alphabetically — and made **this same question unanswerable at any model
quality**, measured 2026-09-22. Narrowing the field set is a lever this project
has already tried here and moved away from. It is not retried.

## The actual defect: the schema permits what the validator forbids

Identical in kind to CDR-08's `COUNT_DISTINCT` with an empty `field_id`. The
filter item is issued as ONE flat object — `op` spanning all eleven operators,
plus a free `value` and a `values` array — so `IS_NULL` carrying a value is a
grammatically writable sentence. The keystone says an invalid plan should be
impossible to emit, not rejected afterwards.

Fix: split the filter item into two variants.

    null test   op ∈ {IS_NULL, IS_NOT_NULL}, value "", no values key
    valued      op ∈ {EQ,NEQ,IN,CONTAINS,GT,GTE,LT,LTE,BETWEEN}, value + values

## Blast radius, measured

Filter operators across every hand-written gold plan:

    EQ 9 · NEQ 1 · BETWEEN 1 · null tests: NONE

No correct plan in the corpus uses a null test, so the null variant is a
capability the layer declares (`allowed_filters` lists IS_NULL/IS_NOT_NULL) and
the corpus never needs. It stays expressible — withdrawing a declared capability
is a separate decision — but nothing correct depends on it.

**This changes the filter schema for EVERY question**, so every plan-cache key
changes and the measurement run is fully cold (~35 min, p95 ~90 s). Budgeted.

## PRE-DECLARED THRESHOLDS — binding

- **Any question moves to WRONG → REVERT IMMEDIATELY, at any ratio.** Fifth time
  written this session; fired once, honoured.
- **Any currently-CORRECT question is lost → REVERT**, regardless of conversions.
- **≥1 converts to CORRECT, 0 lost → KEEP.**
- **0 convert, 0 lost → KEEP ONLY IF** the grammar demonstrably forbids the
  invalid shape (asserted offline through the serving path). The change is a
  safety property — an invalid plan becoming unrepresentable — and is worth
  keeping at zero conversions, unlike the prompt change, which was pure
  instruction with no structural guarantee. Recorded here so this is not a
  post-hoc rationalisation of a null result.

## Honest expectation

This removes a STRUCTURAL defect. Whether CDR-11 then converts is unknown: the
field-selection weakness may still sit underneath, and the earlier cached plan
showed the generator reaching for `cdr.originating_number`. **A null result on
CDR-11 is a plausible outcome and is not a failure of the change** — it would
mean the structural defect was masking a semantic one, which is worth knowing
and is exactly what the run will say.
