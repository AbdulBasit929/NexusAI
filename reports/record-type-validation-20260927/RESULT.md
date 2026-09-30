# RECORD-TYPE VALIDATION — RESULT

Measured 2026-09-27. Thresholds in `THRESHOLD_PREREGISTRATION.md`, written before the run.

**Outcome: SHIPPED, default ON.**

## How it was found

Not by any suite. Found while verifying Codex's redesigned analyst workspace in a browser: the
**Tower** evidence filter returned *"No evidence items in the current scope"* for a case whose
overview lists 5 cell-tower rows. The chip was defined as `recordType: 'tower'`; the stored and
curated value is `tower_location`.

The same value is sent to the question API when an analyst picks the Tower scope in Investigate:

    record_type=tower            "There are no tower records in this case."    <- case holds 5
    record_type=tower_location   "There are 5 tower records in this case."

**A confident false negative**, live in the product. The eval harness never sends `record_type`, so
no suite could ever have caught it.

Every other chip was checked against the stored record types; only Tower was wrong.

## Two fixes

1. **Client** (`apps/investigation-workspace/src/components/ScopeChips.jsx`): `tower` →
   `tower_location`. Verified in the browser: the Tower filter now lists `tower_reference.csv`, and
   Investigate with the Tower scope answers **5**.
2. **Service** (`record_type_validation.go`): an explicit `record_type` is validated before it can
   scope anything. Known → kept; documented alias → canonicalised through the same mapping
   questions already use; unknown → refused with 400, stating nothing was counted.

## Measurement

    arm A (validation off)  reproduced the shipped posture exactly   67 | 28 | 4 | 2 | 1 | 1
    arm B (validation on)   GOLDEN 0 moved · HOLDOUT 0 moved · MEDIA 0 moved

Direct checks against the running service, arm B:

    record_type=tower            HTTP 200   "There are 5 tower records in this case."   PASS
    record_type=tower_location   HTTP 200   "There are 5 tower records in this case."   PASS
    record_type=xyz              HTTP 400   "...no scope was applied and nothing was counted"  PASS
    record_type=email            HTTP 200   accepted, not refused                        PASS (see below)

### A prediction error, recorded

The threshold predicted `record_type=email` would answer *"zero email records"*. It returned a
clarification instead. **Checked against the control rather than assumed:** with validation OFF the
response is identical. The clarification is pre-existing behaviour for an explicit
`record_type=email`; the change does not touch it. The property the check existed to protect —
`email` accepted, never refused, no false claim — holds. The wording of the prediction was wrong
because the baseline was not measured before writing it.

## Rollback

`nexusai-forensic-records-api:rollback-before-rtvalidation-20260927`, or set
`FORENSIC_RECORD_TYPE_VALIDATION=false`.
