# A1a CONSTRAINT OBLIGATIONS — RESULT

Measured 2026-09-27. Thresholds pre-registered in `THRESHOLD_PREREGISTRATION.md` **before** the
image was built and before any arm was scored.

**Outcome: SHIPPED, default ON.**

---

## 1. THE ARMS

Same image, same container, switch state asserted from `docker inspect` before each arm was scored.

    A  control   FORENSIC_CONSTRAINT_OBLIGATIONS=false    16:59 - 17:05
    B  guard     FORENSIC_CONSTRAINT_OBLIGATIONS=true     17:05 - 17:11

Image build: **0.5 min**. Each arm: **~5.5 min** for all three suites.

## 2. THRESHOLD 3.1 — ARM A REPRODUCES THE SHIPPED POSTURE

    expected   CORRECT 67 | CLARIFIED 28 | WRONG 4 | NOT_STATED 2 | MANUAL 1 | ROWCOUNT_ONLY 1
    measured   CORRECT 67 | CLARIFIED 28 | WRONG 4 | NOT_STATED 2 | MANUAL 1 | ROWCOUNT_ONLY 1

**Reproduced exactly.** The guard is inert when off, so arm B is attributable.

## 3. THRESHOLD 3.2 — CORPUS MOVEMENT

    GOLDEN  (62)   control 45/14/2/1     guard 45/14/2/1     0 moved
    HOLDOUT (13)   control 10/2/1        guard 10/2/1        0 moved
    MEDIA   (28)   control 12/12/3/1     guard 12/12/3/1     0 moved

**Zero movement across all 103 questions**, exactly as the offline census predicted (0 of 103
captured). This is the rare arm where "nothing changed" is the pass.

## 4. THRESHOLD 4 — THE PROOF OF FUNCTION

The corpus is a safety instrument only; it cannot prove the guard works, because it contains no
magnitude condition. The ad-hoc probe is the proof. Its selftest passed first
(`How many CDR records do we have in this case?` → **8,642**).

**The defect, closed:**

    asked     "Show me all the calls that lasted longer than ten minutes"
    arm A     "20 CDR records matched this question."        plan filters: []
    arm B     WITHHELD

              clarification:
                I did not apply the condition "longer than ten minutes" to a curated field,
                so this result counts every record of its kind rather than only those you
                asked for. Name the field the condition applies to and I will compute it.

              guardrail:
                No result was stated because the question states the condition "longer than
                ten minutes" while the executed plan compares against nothing. A total
                presented as though it were filtered is a confident wrong answer, not an
                approximation.

              reason_code: condition_not_applied
              route:       verified_only_withheld

**The control question still answers 8,642 in arm B**, and no other probe question changed state.

**Out of scope, unchanged as pre-registered in §1 of the threshold document** — recorded so the
result is not misread as a partial failure:

    "Do any subscribers share the same handset?"                   relational, not magnitude
    "Is there any link between the plate sightings and the call records?"   states no condition

These remain wrong. They need a relational-condition detector, which is a separate item with its
own measurement. Widening this detector to reach them would have made it vague, and a vague guard
on the answer path costs correct answers.

## 5. DECISION

    corpora flat AND the probe question withheld  ->  SHIP, default ON

**Shipped default ON**, which is the documented exception to this project's switch discipline:
default-OFF is the state that answers a filtered question with an unfiltered count, so the unsafe
state is the off state. Persisted in `.env.forensic-runtime.local` and defaulted `true` in
`docker-compose.forensic-records.yaml`. Rollback image:
`nexusai-forensic-records-api:rollback-before-constraint-20260927`.

## 6. AN INSTRUMENT DEFECT FOUND DURING THIS RUN — the fifteenth

The probe reported the withhold as:

    I did not apply the condition \

`classify()` regexed `"direct_answer": "([^"]*)"` over the serialised response. The refusal embeds
the analyst's own words with Go's `%q`, so the sentence contains escaped quotes and the match
stopped at the first one. **A working guard looked broken.**

Fixed: `classify()` now walks the parsed document instead of regexing its serialisation. The same
run also failed its selftest once because the probe defaulted to the UI dev proxy, which was down —
**it correctly reported nothing rather than reporting a working system as broken.** The probe now
targets the API directly and takes the endpoint from `NEXUSAI_PROBE_URL`.

Both are instrument fixes. Neither changed a product behaviour or a verdict in this run.

## 7. WHAT WAS NOT DONE

No `docker compose down`, `down -v`, `--remove-orphans`, `system prune`, `volume prune`. No database
write, migration, backfill or reprocessing. No auth disabled. No expectation edited. No evidence
uploaded. Both arms used the same image and the same container.
