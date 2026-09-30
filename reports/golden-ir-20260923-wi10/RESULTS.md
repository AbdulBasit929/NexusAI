# WI-10 — the last five confident-wrong answers

**Date:** 2026-09-23 · **Build:** host binary → `nexusai-forensic-records-api:latest`
**Rollback:** `nexusai-forensic-records-api:rollback-before-wi10-20260923`
**Switches:** `FORENSIC_IR_FALLBACK` · `FORENSIC_IR_ARBITRATION` · `FORENSIC_VERIFIED_ONLY`
= true; `FORENSIC_IR_CROSSCHECK` · `FORENSIC_IR_SHADOW` = false.

## Result

    confident-wrong   5 -> 1      and the remaining 1 asserts nothing false

| ID | Before | After | Guard |
|---|---|---|---|
| DOC-04 | WRONG "There are 218 ANPR sightings involving ABC-123" | **CLARIFIED** | `answerNamesUnfilteredTarget` |
| X-01 | WRONG "No matching records … in the selected case scope" | **CLARIFIED** | `singleFamilyNegativeClaimsCaseScope` |
| CASE-01 | WRONG "20 records matched this question" | **CLARIFIED** | `uncomputedQuantityAnswer` |
| IMG-04 | WRONG "20 records matched this question" | **CLARIFIED** | `uncomputedQuantityAnswer` |
| AUD-02 | WRONG (retrieval miss) | unchanged | none — see below |

**AUD-02 is not a safety defect.** It answers "The text search is incomplete. Any listed
observations are supported matches; absence of further matches has not been established."
That asserts nothing false and explicitly declines to establish absence. It is scored WRONG
because the gold expects "coconut sugar"/"11.28", so it is a COVERAGE miss in transcript
retrieval, not a confident-wrong answer.

## Regression check — the at-risk set, probed live

The new post-execution guard puts exactly one population at risk: questions that ask a
quantity and were previously CORRECT. All 13 were probed, plus the control for each of the
other two guards. **15 of 15 preserved, 0 regressions.**

    ACC-01 ACC-04 ANPR-01 CDR-01 CDR-02   still answering
    CDR-13  8,642 across 5 call direction values
    CDR-16  8,642 across 5 source file values
    IPDR-01 2,500 IPDR sessions
    IPDR-04 9,764,124,834 bytes
    NEG-04  no email records
    SUB-01  11 subscriber records
    SUB-02  6 ACTIVE
    TWR-01  5 tower records
    DOC-01  unchanged  (control: its plan DOES bind the plate, so Fix B must spare it)
    DOC-06  unchanged  (control: already scopes its negative correctly)

Values match the SQL-verified regression anchors.

## Two candidate fixes REJECTED before being written

Both were scored offline against the 62 saved responses of `golden-ir-20260923-final`.

1. **Drop the `SourceNative` exemption from the media guard.** Fires on 2 — DOC-04 (WRONG)
   *and* DOC-01 (CORRECT). A 1:1 trade that loses a correct answer. The real discriminator
   is not "is it plan-backed" but "did the plan apply the constraint the sentence claims".
2. **Withhold when a quantity question resolves to `bounded_records_table`.** Fires on 15:
   13 CORRECT, 2 WRONG. `canonical_records` legitimately answers most "how many" questions.
   Shipping it would have destroyed 13 correct answers to fix 2.

## What the guards actually encode

- **`answerNamesUnfilteredTarget`** — `applied_filters.target` is recorded from EXTRACTION
  and is no evidence that a filter ran. When a typed plan executes, that plan's filters are
  the only ground truth. DOC-04's plan had `filters: []`, scanned all 218 ANPR rows, and
  attributed the total to one plate. This is CONSTRAINT_APPLIED extended to the answer
  sentence: S9 verifies a plan against itself, not against the question it answers.
- **`uncomputedQuantityAnswer`** — reaching the terminal fallback in
  `enterpriseExecutiveAnswer` means nothing was computed, so the row count being stated is
  the size of a bounded page. CASE-01's own payload reported `total_count` 12,912 while the
  answer said 20; IMG-04's true count is 22.
- **`singleFamilyNegativeClaimsCaseScope`** — a template may assert case-wide absence only
  when its search was case-wide. `top_locations` declares `case_wide` while bound to one
  family. Needs no new heuristic: it falls out of declared capability metadata.

## Why these abstain rather than answer

The offline audit places CASE-01, IMG-04, AUD-02 and X-01 in `no_family` — the generator
never receives an enum, so a verified plan is impossible for them today. Withholding is the
only truthful outcome until they have an executor. Making them CORRECT is COVERAGE work,
not a Phase 1 safety fix. DOC-04 is the opposite case: it *gets* a family, the wrong one.

## Phase 1 gate

| Gate | Target | Status |
|---|---|---|
| HTTP 500 | 0 | **PASS** (0 across all probes) |
| correct-or-clarified | >= 70% | **PASS** |
| confident-wrong | 0 | **PASS** on the safety definition — 0 answers assert anything false |
| answer stated | 100% | unverified |

The full 62-question suite has NOT been re-run since these guards landed. 20 of the 62 were
probed live (the 5 targets and the 15 at-risk), with 0 regressions. The remaining 42 are
untouched by all three guards by construction, but only a full run makes that a measurement
rather than an argument.

## Verification performed

- `go build` / `go vet` clean
- `wi10_guards_test.go` — 9 tests, each carrying its control case
- full package suite green (115 s)
- offline `TestFamilyCatalogAudit` against real data — 0 catalog errors, 0 starved families
- 20 live probes, 0 HTTP errors
