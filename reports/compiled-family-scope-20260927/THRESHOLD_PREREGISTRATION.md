# CDR-14 COMPILED FAMILY SCOPE — THRESHOLDS, WRITTEN BEFORE THE RUN

Written 2026-09-27, after the offline proof and before the image was built or any arm was scored.

## The defect, proven offline through the real executor

`TestCompiledFamilyScopeExecutesCDR14`, live database, *"How many call records are from August
2026?"*:

    switch OFF   record_type=""     m1=4   scanned=4   (2 cdr + 2 subscriber)   <- defect
    switch ON    record_type="cdr"  m1=2   scanned=2                           <- oracle

The deterministic compiler classified the family correctly and compiled the date range, but handed
the plan on with `RecordType` empty. The date filter is on the top-level `timestamp` column every
family carries, so the count ran over the whole case.

## The change

`FORENSIC_COMPILED_FAMILY_SCOPE`, **default ON** (the off-state over-counts across families). A plan
compiled for one curated **records** family is scoped to that family's `record_type`. Never
overrides an explicit client scope; never scopes a cross-family question; never touches a derived
family.

## Where it can reach in the shipped posture

The deterministic dynamic path runs for questions the keyword ladder abstains on, via language
assistance. **CDR-14 itself is CORRECT today via the IR arbitration and is not predicted to move.**

## Arms

    A  control   FORENSIC_COMPILED_FAMILY_SCOPE=false
    B  scoped    FORENSIC_COMPILED_FAMILY_SCOPE=true

## Thresholds

- **Arm A reproduces the shipped posture** (67 / 28 / 4 / 2 / 1 / 1), or the run is void.
- **Any CORRECT lost → revert.** Covers every honesty probe, which scores CORRECT when it refuses.
- **Any new WRONG → revert.**
- **Predicted movement: none.** Any movement must be named and explained before shipping,
  including an improvement.
