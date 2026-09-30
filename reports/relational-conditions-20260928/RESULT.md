# A1c RELATIONAL CONDITIONS + A1d LOCATION OBLIGATION -- RESULT

Measured 2026-09-28. Thresholds in `THRESHOLD_PREREGISTRATION.md` (this folder) and
`reports/location-obligation-20260928/THRESHOLD_PREREGISTRATION.md`, both written before the image
was built. Scored by `scripts/nexusai_guards_compare.py`.

    FORENSIC_RELATIONAL_CONDITIONS   SHIPPED, default ON
    FORENSIC_LOCATION_OBLIGATION     SHIPPED, default ON

## Corpus, ladder ON (arms in reports/guards-20260928/)

    control   CORRECT 67 | CLARIFIED 28 | WRONG 4 | NOT_STATED 2 | MANUAL 1 | ROWCOUNT_ONLY 1
    a1c       identical -- 0 of 103 moved (census predicted 0: no corpus question states a relationship)
    a1d       H11 WRONG -> CLARIFIED, nothing else moved, NEG-02 stays CORRECT
    both      identical to a1d
    shipped   identical to both -- the deployed build reproduces the measured arm exactly
              CORRECT 67 | CLARIFIED 29 | WRONG 3

## Relational probe (16 questions, relational_probe.py)

    R1 R2 R3 R4 R6 R8   answered with a count in control -> WITHHELD, reason relationship_not_computed,
                        the relationship named back ("share the same handset", "shared a cell tower")
    R5 R7               already withheld in control, unchanged
    N1-N8               unchanged in state and text ("share of SMS", "Can you share a summary",
                        "Share a list", "linked to <number>", four ordinary questions)

## An instrument defect, found and fixed before the verdict

The comparator's first run fired 12 thresholds: every withheld R question scored as "answered without
a relationship". The probe's extractor reads `direct_answer`, and a withhold carries its refusal
sentence there. Read against the contract (clarification reason code, withheld route), all six were
correct withholds with 0 rows. The pre-registered criterion names "withheld with
relationship_not_computed" as the pass, so the comparator was corrected to read the contract, not the
threshold. `withheld()` in the comparator records why.

## Wording, changed after the arms and re-verified on the shipped build

The echoed phrase was cut at one word ("shared a cell") and one sentence read "about a overlap". The
capture now takes up to two words and drops a trailing connective, and both sentences were rewritten.
Text only: the set of withheld requests is unchanged. Re-verified on the deployed build (corpus
identical, probe identical in state).

## Found in passing, not in scope

R5 "What do the two busiest numbers have in common?" is correctly routed to clarification, but its
analyst text reads "No qualifying phone-counterparty events were found for the selected target" --
an absence statement on a request that was never executed. Pre-existing, `multi_cdr_comparison`.

Rollback image: `nexusai-forensic-records-api:rollback-before-guards-20260928`.
