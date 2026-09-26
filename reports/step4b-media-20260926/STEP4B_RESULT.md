# STEP 4b — RESULT. Measured 2026-09-26. **Both Step 4 defects are fixed. One decision is owed.**

    A  CONTROL     media off, boolean off    GATE: reproduced Step 4's control EXACTLY
    B  CATALOGUE   media on,  boolean off    the derived-catalogue isolation
    C  +BOOLEAN    media on,  boolean on     + the boolean restriction obligation

Thresholds and per-question predictions written before the rebuild:
[`THRESHOLD_PREREGISTRATION.md`](THRESHOLD_PREREGISTRATION.md).

---

## 1. The numbers

| Suite | A control | B catalogue | C +boolean |
|---|---|---|---|
| golden 62 | 45 · 14 CLAR · 2 NS · 1 MAN · **0 WRONG** | **identical** | **identical** |
| held-out 13 | 10 · 2 CLAR · 1 WRONG | **identical** | **identical** |
| media 28 | **5** CORRECT · 15 CLAR · **7 WRONG** | **14** CORRECT · 8 CLAR · **5 WRONG** | **13** CORRECT · 11 CLAR · **3 WRONG** |

**ZERO structured transitions across 75 questions in every arm.** Both changes are inert on
structured evidence, measured three times rather than argued.

**THE GATE PASSED.** Arm A reproduced Step 4's control identically across all 103 questions, on a
rebuilt image. That is what makes arms B and C attributable: the catalogue isolation is genuinely
unreachable with media routing off, live, not merely by unit test.

## 2. Confident-wrong, which is the bar

    control (media OFF)   7   H2 · H4 · M15 · M16 · M21 · M7 · P2
    arm B                 5   H2 · M15 · P2 · M7 · M12          <- M12 is NEW
    arm C                 3   H2 · M15 · P2                     <- a strict SUBSET of the control

    NEW confident-wrong introduced by arm C:  NONE
    Removed by arm C:                         H4 · M16 · M21 · M7   (four)

**Arm C's three remaining wrong answers are wrong with the switches OFF as well.** They are
unrelated to this work and are listed in §5.

## 3. Defect 1 — the binding blocker. FIXED.

M2 "average OCR confidence of the **plate reads**" was answered **0.89 from `forensic.records`** in
Step 4, with 50 records citations and zero derived ones. Root cause: `mergeSemanticLayerCatalog`
appended schema-inferred RECORDS columns into a DERIVED family's catalogue, the model chose one,
and the binding followed the plan's fields to the wrong table exactly as designed. Nothing refused
it, because the plan did not MIX sources — it was purely records.

Reproduced offline before any change was made: the leaked field id came back
**`fld_22a22183f7a9e0024f970ca6`, byte-identical to the one in M2's live plan.** That made the
diagnosis conclusive instead of plausible.

A derived family is now issued its curated fields and nothing else. Live result, both arms:

    M2  verdict CORRECT   truth 0.955573   answered 0.96
        50 citations, every one derived_artifacts_sql, ZERO records citations
        "The average plate-text recognition confidence across ANPR sightings is 0.96."

The answer even reads better: it now uses the curated display name rather than the records
column's generic one, because the curated field is the only one on offer.

**Structured families keep the merge.** Withholding uncurated fields there was measured and
rejected — it cost TWR-02 a false negative, "there are no tower records in this case" — and
`TestStructuredFamilyCatalogStillMergesInferredFields` asserts it rather than trusting the branch.

## 4. Defect 2 — the dropped filter. FIXED, behind `FORENSIC_BOOLEAN_RESTRICTION`.

Every derived plan in Step 4 carried `filters: null` and the system answered anyway. A plan that
silently widens "how many X require review" to "how many X" is valid, executes over the right
contract, and verifies — **verification proves a plan against ITSELF, never against the question.**
An INVENTED filter is a symptom this system already watches for; a MISSING one has none.

**The census came first.** Before the rule existed, exactly three questions in all three corpora
name a curated BOOLEAN field by a declared multi-word phrase, and **zero structured questions do.**
Predictions were written into the threshold file before the run. Arm C moved **exactly those three
and nothing else**:

| | before | after | why |
|---|---|---|---|
| **M12** "how many image text regions require manual review" | **WRONG** — answered 309, truth 46 | **CLARIFIED** | a confident-wrong removed |
| **M7** "how many plate groups used persistent object tracking" | **WRONG** — answered 24, truth 0 | **CLARIFIED** | a confident-wrong removed |
| **M3** "how many plate reads still need manual review" | CORRECT — 307 | **CLARIFIED** | **a LUCKY answer removed** |

**M3 was right by coincidence and losing it is the point.** `manual_review_required` is true on all
307 ANPR observations, so the unfiltered count happened to equal the truth. The plan never applied
the restriction. **An answer that is right by accident is not a capability** — that is exactly what
"the 47 was never real" was about, and it was accepted in advance in §3.3 of the threshold file.

## 5. The three that remain, and none is ours

All three are WRONG in the control, with media routing OFF. Reverting does not fix them.

    H2   "What does the audio transcript say?"       HTTP 200, NO analyst text, route=[terminal], ~20ms
    M15  "latest end offset in the audio transcripts" HTTP 200, NO analyst text, route=[terminal], ~25ms
    P2   "were any transcribed segments from video?"  "No transcript segment intersects the requested
                                                      source-time range" — a false absence from a time
                                                      filter nobody asked for

**H2 and M15 return success with nothing in it.** Silence is not abstention: the analyst is told
neither an answer nor why there is none. It is not media-specific and it is the next defect to fix.

## 6. Everything else held

**Every honesty probe passed in both arms.** P1 answered **1,057** and never 307 — no fabricated
sighting. H1, H3, H4, H5 clarified. M6 clarified, which is correct: two media families match it and
ambiguity clarifies by design.

**Step 2's provenance property held on the serving path**, every media answer citing
`derived_artifacts_sql` with artifact identity and `source_truth_state`.

**One correct answer was lost in both B and C:** M4 "highest plate detection confidence"
CORRECT → CLARIFIED. Worth investigating, but it is a lost answer rather than a wrong one, and the
media bar is explicitly about confident-wrong, not count.

## 7. A NEAR-MISS WORTH RECORDING

`FORENSIC_BOOLEAN_RESTRICTION` was set in the shell but **not declared in
`docker-compose.forensic-records.yaml`**, and compose only forwards variables it names. **Arm C
would have run with the rule silently OFF and been scored as neutral** — the 2026-09-25
configuration drift exactly, in a new place.

It was caught by the protocol that exists because of that incident: assert the switch state FROM
THE CONTAINER before scoring. The switch is now declared in compose, with the reason recorded
beside it. **The protocol paid for itself the first time it was applied to a new switch.**

## 8. THE DECISION OWED — and a flaw in my own threshold

The pre-registered rule §3.4 reads:

    Arm C still has confident-wrong  ->  media stays OFF, report the remainder

Read literally it fires: arm C has three. **But that rule is flawed, and I wrote it.** It does not
distinguish confident-wrong CAUSED by the change from confident-wrong the change does not touch.
H2, M15 and P2 are wrong with the switches off as well, so the rule as written can never be
satisfied and, applied literally, mandates keeping a configuration that is strictly worse:

    media OFF (rule followed)    5 CORRECT · 7 WRONG
    media ON  + boolean (arm C) 13 CORRECT · 3 WRONG, none of them new

I am not reinterpreting a threshold after seeing the result — that is the failure mode this
project's discipline exists to prevent. **The two readings are stated and the call is the product
owner's:**

| Reading | Basis | Outcome |
|---|---|---|
| **Literal** | §3.4 as written: any media confident-wrong → off | Media stays OFF. Keeps 7 wrong answers to avoid 3. |
| **Causal** | Step 4 reverted on questions that MOVED to WRONG; arm C moves none | **Ship arm C.** Zero new confident-wrong, four removed, +8 correct, structured untouched. |

**The container has been returned to the control state (both switches OFF) pending that decision**,
because the conservative default is the one the literal rule names.

## 9. What was NOT done

No `docker compose down`, `down -v`, `--remove-orphans`, `system prune` or `volume prune`. No
database write, migration, backfill or reprocessing. No auth disabled. **No expectation in any
suite was edited.** Rollback images `rollback-before-step4-20260926` and
`rollback-before-step4b-20260926` were tagged before each build.

**Evidence:** `control/`, `catalogue/`, `boolean/`, each with three suites and per-question
`raw/*.json`. Reproduce with:

```bash
python scripts/nexusai_step4b_compare.py
```

---

## 10. SHIPPED — 2026-09-26, on the product owner's decision (the causal reading, §8)

Arm C is live. The three switches are **persisted in `.env.forensic-runtime.local` AND declared in
`docker-compose.forensic-records.yaml`**, because a switch set only in the shell is how the
2026-09-25 drift happened and how this run's near-miss nearly happened again.

Deployed from the env file with **no shell override**, which is what proves the persistence works
rather than the deploy command working. Asserted from the container:

    FORENSIC_MEDIA_FAMILY_ROUTING=true
    FORENSIC_DERIVED_ARTIFACT_EXECUTION=true
    FORENSIC_BOOLEAN_RESTRICTION=true
    FORENSIC_VERIFIED_ONLY=true · IR_ARBITRATION=true · IR_FALLBACK=true · PLAN_CACHE=true
    ANSWER_STATES_VALUES=true · MEASURE_DENOMINATOR=true · LADDER_ROUTING=true
    BREAKDOWN_GOAL=false · IR_SHADOW=false · IR_CROSSCHECK=false · DROP_INVENTED_FILTERS=false

**Ship confirmation, media suite re-run against the deployed configuration:**

    13 CORRECT · 11 CLARIFIED · 3 WRONG · 1 ROWCOUNT_ONLY
    IDENTICAL to arm C, question for question
    0 HTTP 500 · 0 timeouts
    the 3 WRONG are H2, M15, P2 — the pre-existing ones

Offline suite green in every switch combination, including the shipped one. DB-backed proofs green:
derived execution, measure denominator, media PII boundary, the M2 catalogue isolation, curated
coverage 93/94, `TestWI4`.

### What ships with it

- **Media families answer from a typed plan over 865 model observations**, 13 of 28 held-out media
  questions correct, with citations that name `derived_artifacts_sql`, the artifact, its locator and
  `source_truth_state`.
- **No fabricated sightings.** P1 answers 1,057 ingested camera sightings and never the 307 model
  plate reads.
- **Honest abstention on every honesty probe**, including "who are the people in the images?" over
  20 detected faces and "which plates were read from the videos?" over PII-excluded plate text.
- **A dropped restriction is refused, not widened** — and one answer that was right by coincidence
  was given up to get there.

### The gaps, stated plainly

H2 and M15 return HTTP 200 with no analyst text at all; P2 asserts a false absence. All three are
wrong with media off as well. Cross-family is still 0 of 5. The conversational classes are still
unwired. 9,041 lines of template ladder still await deletion.
