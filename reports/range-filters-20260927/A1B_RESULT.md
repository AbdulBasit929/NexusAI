# A1b RANGE FILTERS — RESULT

Measured 2026-09-27. Thresholds pre-registered in `THRESHOLD_PREREGISTRATION.md` before the image
was built and before any arm was scored.

**Outcome: SHIPPED, enabled, after two measured rounds.** Round one proved the filter and was held
off on a narration defect (§5); round two fixed it and re-measured (§8).

---

## 1. THE ARMS

    A  control   FORENSIC_RANGE_FILTERS=false    17:24 - 17:30
    B  ranges    FORENSIC_RANGE_FILTERS=true     17:30 - 17:35

Same image, same container, switch asserted from `docker inspect` before each arm. Build 0.7 min.

## 2. CORPUS THRESHOLDS — PASS

    arm A reproduced the shipped posture EXACTLY
      CORRECT 67 | CLARIFIED 28 | WRONG 4 | NOT_STATED 2 | MANUAL 1 | ROWCOUNT_ONLY 1

    GOLDEN  (62)   0 moved
    HOLDOUT (13)   0 moved
    MEDIA   (28)   0 moved

Zero movement across all 103 questions, as the offline census predicted (0 of 103 captured).

## 3. PROOF OF FUNCTION — PASS, against an independent SQL oracle

Oracle derived by mirroring the executor's own expression,
`COALESCE(r.raw_payload->>'<source_name>', r.metadata->'normalized_fields'->>'<source_name>')`,
never from the API's output.

| Question | Oracle | Measured | Plan |
|---|---|---|---|
| `How many transactions have an amount above 50000?` | **1** (of 4) | **1** | carries `GT` |
| `How many IPDR sessions have bytes above 1000000?` | **2,189** (of 2,500) | **2,189** | carries `GT` |

Retained transaction amounts are `9200, 14500, 31000, 75000` — exactly one exceeds 50000.

**Invariants held:**

    "Show me all the calls that lasted longer than ten minutes"  -> STILL WITHHELD
        no unit was converted, which is the outcome section 2 of the threshold demands
    "How many CDR records do we have in this case?"              -> 8,642

## 4. THE FLAGSHIP EXAMPLE STILL ABSTAINS, CORRECTLY

*"Calls longer than ten minutes"* does not work and must not. Surveyed across the whole curated
layer: **no range-filterable field declares a `unit`**, and `cdr.call_duration_seconds` is a derived
METRIC (`timestamp_diff_seconds(...)`) with no `allowed_filters`. There is no declared basis for
600, and no filterable duration column. A1a withholds it and names the condition.

**Duration filtering is its own work item** and needs HAVING over a computed expression.

## 5. WHY IT IS NOT ENABLED — the narration drops the qualifier

The measured answer is arithmetically correct and the plan is right. The **sentence** is not:

    asked     "How many transactions have an amount above 50000?"
    answered  "There is 1 transaction in this case."

**The case holds 4 transactions.** Read in context the sentence is defensible; read standalone —
exported, quoted in a report, pasted into an email — it states something false about the case.

The product already has the correct standard and meets it for a different filter shape:

    asked     "How many times was plate LHR-2026 seen?"
    answered  "There are 87 ANPR sightings INVOLVING LHR-2026 in this case."

So narration repeats a **target** filter and not a **range** filter. That is a gap in one place, not
a design disagreement.

**This is exactly the shape of defect this project exists to refuse**: a true number carried by a
sentence that claims more than was computed. Shipping a correct filter behind a misleading sentence
would trade one confident-wrong for another.

**Held at default OFF.** The shipped posture was restored and asserted from the container after the
run: `FORENSIC_RANGE_FILTERS=false`.

## 6. NEXT — A1b.1, and it is small

Make the answer sentence name the range condition, exactly as it already names a target:

    "There is 1 transaction with an amount above 50000 in this case."

Then re-measure A1b with the same thresholds plus one addition — **the stated sentence must contain
the condition** — and enable both together.

## 8. ROUND TWO — A1b.1, THE NARRATION FIX, MEASURED

`rangeFilterQualifiers` in `answer_presentation.go` now names each range bound the **executed plan**
carries, in the same place and for the same reason the headline already named a target. It reads
the plan, never the question: the plan is what ran.

Gated on the same switch, so A1b and A1b.1 are one measurable unit.

**Arms `control2` / `ranges2`, same image, switch asserted from the container:**

    arm A reproduced the shipped posture EXACTLY   67 | 28 | 4 | 2 | 1 | 1
    GOLDEN 0 moved · HOLDOUT 0 moved · MEDIA 0 moved

**The sentence, before and after:**

    before   "There is 1 transaction in this case."                             <- case holds 4
    after    "There is 1 transaction with amount above 50000 in this case."

    before   "There are 2,189 IPDR sessions in this case."                       <- case holds 2,500
    after    "There are 2,189 IPDR sessions with bytes above 1000000 in this case."

Oracle values still matched (**1** and **2,189**), the plan still carried `GT`, the duration
question was **still withheld**, and the control still answered **8,642**.

**All thresholds passed, including the one added for round two: the stated sentence must contain
the condition.** Enabled by default in compose and persisted in `.env.forensic-runtime.local`;
shipped posture re-asserted from the container.

## 7. WHAT WAS NOT DONE

No `docker compose down`, `down -v`, `--remove-orphans`, `system prune`, `volume prune`. No database
write, migration, backfill or reprocessing. No auth disabled. No expectation edited. No evidence
uploaded. Rollback image: `nexusai-forensic-records-api:rollback-before-rangefilters-20260927`.
