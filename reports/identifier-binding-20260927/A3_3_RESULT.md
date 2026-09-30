# A3.3 IDENTIFIER BINDING + DETERMINISTIC ARBITRATION — RESULT

Measured 2026-09-27. Thresholds pre-registered in `THRESHOLD_PREREGISTRATION.md` before the image
was built and before any arm was scored. Switch state asserted from `docker inspect` before every
arm and saved as `<arm>-switches.txt`.

**Outcome:**

    identifier binding          MEASURED CLEAN (0 of 103 moved) -- kept, held OFF
    deterministic arbitration   THRESHOLD FIRED -- REVERTED and removed

---

## 1. THE THREE ARMS

    A  control   binding=false  arbitration=false   CORRECT 67 | CLARIFIED 28 | WRONG 4 | NOT_STATED 2 | MANUAL 1 | ROWCOUNT_ONLY 1
    B  binding   binding=true   arbitration=false   CORRECT 67 | CLARIFIED 28 | WRONG 4 | NOT_STATED 2 | MANUAL 1 | ROWCOUNT_ONLY 1
    C  both      binding=true   arbitration=true    CORRECT 64 | CLARIFIED 30 | WRONG 4 | NOT_STATED 3 | MANUAL 1 | ROWCOUNT_ONLY 1

Arm A reproduced the shipped posture exactly. Golden p95 latency: 4,487 / 5,205 / 7,473 ms — all
inside the 15 s gate, but arm C added ~3 s at p95.

## 2. ARM B — BINDING ALONE: PREDICTION HELD

**Zero movement**, exactly as pre-registered. Binding only reaches questions the keyword ladder has
abstained on, and none of those in the corpus names a single-field identifier.

The binding logic is correct and was proven offline through the real entry point:

    TWR-02   typed plan   tower.site_code EQ PK-LHR-SYN-001
    ANPR-02  typed plan   anpr.plate_number EQ LHR-2026
    SUB-03   refuses      a WHO question; the identity (name, CNIC) is PII-withheld
    H11      refuses      the phone sits in caller AND callee; binding one would narrow the question
    NEG-02   refuses      the value exists in no field

**Kept, held OFF.** It is safe but has no route of its own to the questions it would help.

## 3. ARM C — ROUTING: THRESHOLD FIRED

    CDR-12  CORRECT   -> CLARIFIED   "Which target identifier should I analyze?"
    CDR-14  CORRECT   -> WRONG       "There are 4 records from 2026-08-01 ... in this case."  (answer is 2)
    CDR-16  CORRECT   -> CLARIFIED   "I can count this, but I did not compute a count here"
    TWR-02  CLARIFIED -> NOT_STATED  "1 tower record matched this question."
    H11     WRONG     -> CLARIFIED   withheld by answerNamesUnfilteredTarget

Three correct answers lost, one of them to a confident wrong. **Reverted immediately**; the shipped
posture was restored and asserted before any analysis was done.

### 3.1 Why — the deterministic compiler pre-empted a working rescue

In the control arm all three lost questions were **CORRECT via `records_sql`** — the LLM IR
arbitration was already earning them a verified plan. Arm C ran the deterministic compiler
**first**; its plan was adopted, so the IR never ran.

The deterministic compiler then failed in exactly the shapes already on the A3 backlog:

    CDR-12  rank over all rows with no target       = gap A3.2
    CDR-16  GROUP BY a provenance column             = gap A3.1
    CDR-14  counted 4 generic records, not the 2 CDR records -- a family-scoping defect

**The IR is better than the deterministic compiler on these shapes, which is why it was rescuing
them.** Putting the weaker generator first replaced good plans with bad ones.

### 3.2 The pre-registered safety argument was WRONG

The threshold document said:

> *"a clarification makes NO claim, so replacing one with a plan that passed S9 SHAPE and
> CONSTRAINT_APPLIED cannot turn a right answer wrong."*

That argument silently assumed the baseline was the clarification. **It is not.** The baseline is
whatever the *next* rescuer in line would have produced — and here that was a correct IR plan.
The argument had been copied from the arbitration site, where it IS true because the IR is the last
stage. Moved one step earlier, it stopped being true.

**Lesson, recorded for every future arbitration change: a claim-free state is only claim-free if
nothing downstream would have filled it. Compare against the pipeline's actual next outcome, not
against the state at the point of insertion.**

The thresholds caught it anyway, because "any CORRECT lost" is written against the measured
control, not against the argument. That is the reason thresholds are written against outcomes.

### 3.3 TWR-02 is a narration gap, not a data gap

With routing on, TWR-02 returned **the right row** — one tower, filtered on its site code — and
stated *"1 tower record matched this question."* The coordinates were in the result grid and absent
from the sentence, so it scored NOT_STATED. That is the same weak sentence the ad-hoc probe
identified: a lookup result is narrated as a row count.

## 4. WHAT REMAINS, PRECISELY

To make TWR-02 answer correctly, two things are needed, and both are now exactly located:

1. **Routing as a LAST resort, after the IR arbitration** — only when the IR also failed to earn a
   plan. It then cannot pre-empt a working rescue. TWR-02 is such a case: the IR fails on it today.
2. **Lookup narration** — a single-row projection should state its key attributes rather than
   "1 record matched". Without this, correct routing still yields NOT_STATED.

Separately, the three deterministic-compiler defects exposed here are the A3.1 / A3.2 items plus a
new one (CDR-14 family scoping). They matter even though routing is reverted: they are why the
deterministic compiler cannot yet replace the IR or the ladder.

## 5. WHAT WAS REMOVED

The deterministic-arbitration block in `query.go` (replaced by a comment pointing here), its switch,
and its compose entry. Binding code, its tests, and the offline probes remain. Full package tests
pass; the image was rebuilt from the reverted source and the shipped posture re-asserted:

    CONSTRAINT_OBLIGATIONS=true · RANGE_FILTERS=true · VERIFIED_ONLY=true
    LADDER_ROUTING=true · IR_ARBITRATION=true · IDENTIFIER_BINDING=false

## 6. WHAT WAS NOT DONE

No `docker compose down`, `down -v`, `--remove-orphans`, `system prune`, `volume prune`. No database
write, migration, backfill or reprocessing. No auth disabled. No expectation edited. No evidence
uploaded. Rollback image: `nexusai-forensic-records-api:rollback-before-idbinding-20260927`.
