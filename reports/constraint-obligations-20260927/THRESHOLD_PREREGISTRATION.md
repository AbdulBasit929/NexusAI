# A1a CONSTRAINT OBLIGATIONS — THRESHOLDS, WRITTEN BEFORE THE RUN

Written 2026-09-27, after the offline census and before the image was built, before any arm was
deployed and before anything was scored. **Not editable once the first arm starts.**

---

## 1. WHAT THIS CHANGE IS, AND WHAT IT DELIBERATELY IS NOT

A magnitude condition the analyst states is a CONSTRAINT_APPLIED obligation, exactly as a named
identifier already is. When the executed plan compares against nothing, the answer is **withheld
and the condition is named back**.

**It adds no capability.** Parsing the condition into a real SQL filter is A1b, is a separate
switch, and is measured separately. If a question becomes newly *answerable* in this arm, that is a
defect in this change, not a bonus.

**Scope is narrow on purpose.** It covers a comparison against a number
(`longer than 10 minutes`, `more than 1000 bytes`, `at least 30`, `between 100 and 200`). It does
**not** cover relational conditions such as *"do any subscribers share the same handset?"*, which
need a different detector and their own measurement. Two of the three measured defects in
`reports/adhoc-runtime-20260927/` are therefore **out of scope and expected to stay wrong** — named
here so that is a recorded decision rather than a surprise in the results.

## 2. THE CENSUS, RUN BEFORE WIRING

`TestConstraintObligationCensus`, across all three corpora:

    103 corpus questions examined
      0 captured

**The guard cannot disturb a single existing answer.** It exists for freely-typed questions, which
is precisely where the defect was measured and precisely what the corpora do not contain.

This makes the corpus a **safety instrument only**. It cannot prove the guard works; §4 is what
proves that.

## 3. THE THRESHOLDS — structured and media

Two arms, same image, same container, switch state asserted from `docker inspect` before scoring.

    A  CONTROL    shipped posture                              (FORENSIC_CONSTRAINT_OBLIGATIONS=false)
    B  GUARD      + FORENSIC_CONSTRAINT_OBLIGATIONS=true

### 3.1 Arm A must reproduce the shipped result exactly

67 CORRECT · 28 CLARIFIED · 4 WRONG · 2 NOT_STATED · 1 MANUAL · 1 ROWCOUNT_ONLY across the three
suites. If arm A does not reproduce this, the guard is not inert when off and **everything
downstream is void**.

### 3.2 Arm B — any of these reverts the whole arm

- **Any golden-62 question moves at all.** The census says zero are captured; one movement means the
  detector is firing where it was measured not to, and the census is wrong.
- **Any held-out-13 question moves at all.** Same reasoning.
- **Any media-28 question moves at all.** Same reasoning.
- **Any question becomes WRONG.** A different wrong answer is still a wrong answer.

**The predicted movement on all three corpora is ZERO.** This is the rare arm where "nothing
changed" is the pass, not a disappointment.

## 4. THE PROOF THAT IT WORKS — the ad-hoc probe, not the corpus

`reports/adhoc-runtime-20260927/adhoc_probe.py`, re-run against both arms. Its selftest already
refuses to report unless a known-good control (`How many CDR records do we have in this case?` →
**8,642**) comes back ANSWERED.

**PASS requires, in arm B:**

    "Show me all the calls that lasted longer than ten minutes"
        arm A   "20 CDR records matched this question."          <- the defect
        arm B   WITHHELD, and the sentence contains
                "longer than ten minutes"                        <- the fix

**Explicitly NOT expected to change** (out of scope, §1), and recorded now so the result is not
read as a failure:

    "Do any subscribers share the same handset?"            relational, not magnitude
    "Is there any link between the plate sightings and the call records?"   no condition stated

**Also required in arm B:** the control question still answers **8,642**, and no question that
answered in arm A is withheld in arm B other than the one named above.

## 5. DECISION RULE

    arm A does not reproduce the shipped result        -> void, investigate, nothing ships
    any corpus movement in arm B                       -> REVERT the whole arm
    a new WRONG anywhere                               -> REVERT the whole arm
    the probe question is not withheld in arm B        -> the guard is in the wrong place; do not
                                                          ship, and re-locate it before retrying
    corpora flat AND the probe question withheld       -> SHIP, default ON
                                                          (see 5.1)

### 5.1 Why this one ships default ON if it passes

Every other switch on this project defaults OFF because its default-off state is the safe one. Here
it is inverted: **default-off is the state that answers a filtered question with an unfiltered
count.** A safety fix whose off-state is the unsafe one ships on, which is the exception already
recorded in the project's switch discipline.

## 6. WHAT WILL NOT BE DONE

No `docker compose down`, `down -v`, `--remove-orphans`, `system prune`, `volume prune`. No database
write, migration, backfill or reprocessing. No auth disabled. **No expectation edited.** No evidence
uploaded. The two arms use the same image and the same container.
