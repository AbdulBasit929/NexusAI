# STEP 4b — THRESHOLDS, WRITTEN BEFORE THE REBUILD

Written 2026-09-26 after Step 4 fired and reverted, and **before the image carrying the fixes was
built**. Step 4's thresholds ([`step4-media-20260926`](../step4-media-20260926/)) carry over
unchanged; this file adds only what is new.

---

## 1. What changed since Step 4, and why each is separable

| Change | Switch | Scope |
|---|---|---|
| **Derived catalogue isolation** — a derived family is issued its curated fields and nothing else | **none** | Unreachable unless a derived family resolves, which needs `MEDIA_FAMILY_ROUTING`. It is part of the thing under test, not a second change. |
| **Boolean restriction obligation** — a plan that neither filters nor groups on a named curated BOOLEAN field is refused | `FORENSIC_BOOLEAN_RESTRICTION`, **default off** | Its own switch, so it gets its own arm. |

**Do not bundle two changes into one measurement** — so there are THREE arms, not two.

## 2. Arms. Same image, same container, switches asserted from the container each time.

    A  CONTROL      MEDIA_FAMILY_ROUTING=false  DERIVED_ARTIFACT_EXECUTION=false  BOOLEAN_RESTRICTION=false
    B  CATALOGUE    MEDIA_FAMILY_ROUTING=true   DERIVED_ARTIFACT_EXECUTION=true   BOOLEAN_RESTRICTION=false
    C  +BOOLEAN     MEDIA_FAMILY_ROUTING=true   DERIVED_ARTIFACT_EXECUTION=true   BOOLEAN_RESTRICTION=true

**Arm A is re-run even though Step 4's control exists**, because the IMAGE changed. The catalogue
isolation is proven inert with media routing off by test, and a test is not a live run. Arm A must
reproduce Step 4's control exactly; **if it does not, the isolation is not inert and everything
downstream is void.**

All three suites in every arm: golden 62, held-out 13, media 28, `--analyst-text`.

---

## 3. THE THRESHOLDS

### 3.1 Carried over from Step 4, unchanged

- Any golden-62 question to WRONG or NOT_STATED → **revert**. Any held-out-13 question to WRONG →
  **revert**. Net CORRECT must not fall in either structured suite.
- **Media: ZERO confident-wrong is the bar.** A low CORRECT count is an honest start.
- No HTTP 500. A TIMEOUT (`http: -1`) is never a verdict.
- **P1 must answer 1,057 and never 307.** H1, H2, H3, H5 and M6 must clarify.
- Every media citation reads `derived_artifacts_sql` with artifact identity and
  `source_truth_state`, and carries no records columns.

### 3.2 New — Arm B, the catalogue isolation. THE DEFECT IT FIXES:

**M2 must not be answered from `forensic.records`.**

    REQUIRED  M2 answers 0.955573 from forensic.derived_artifacts, OR clarifies.
    FAILS     M2 answers 0.89, or any value whose provenance is records_sql.

M2 answering a number from records is the cross-evidence-class defect that reverted Step 4. If it
recurs, the isolation did not reach the serving path and media routing does not ship.

**No media answer may carry a `records_sql` CITATION.** Route and `execution_path` strings are not
citations and do not count.

### 3.3 New — Arm C, the boolean restriction. Predicted from the census, BEFORE the run:

The census says exactly three questions name a curated BOOLEAN field, and **zero structured
questions do**. Predictions, written now:

    M12  WRONG (309 for a truth of 46)  ->  CLARIFIED     a confident-wrong removed
    M7   WRONG (24  for a truth of 0)   ->  CLARIFIED     a confident-wrong removed
    M3   CORRECT (307)                  ->  CLARIFIED     a LUCKY answer removed
    every other question                ->  UNCHANGED

**M3 is expected to be lost and that is accepted in advance.** `manual_review_required` is true on
all 307 ANPR observations, so the unfiltered count coincidentally equals the truth: the plan never
applied the restriction and was right by accident. **An answer that is right by coincidence is not
a capability** — that is what "the 47 was never real" was about. Trading one lucky answer for two
removed confident-wrong is the trade the media threshold explicitly endorses.

**Arm C FAILS and the switch stays off if:** any question other than M3, M7, M12 changes verdict ·
any structured question changes at all · M7 or M12 does not become CLARIFIED.

### 3.4 The decision rule, written before the run

    Arm A does not reproduce Step 4's control            -> everything void, investigate
    Arm B has ZERO media confident-wrong                 -> media routing SHIPS on B
    Arm B still has confident-wrong, Arm C has ZERO      -> media routing SHIPS on C
    Arm C still has confident-wrong                      -> media stays OFF, report the remainder
    any structured regression in any arm                 -> that arm reverts, no exceptions

## 4. What will NOT be done

No `docker compose down`, `down -v`, `--remove-orphans`, `system prune`, `volume prune`. No
database write, migration, backfill or reprocessing. No auth disabled. **No expectation in any
suite edited after seeing a result** — an oracle defect is re-derived from the same expression the
executor uses and recorded as a defect, never as a score change. Thirteen found so far, the last
in the Step 4 comparator itself.
