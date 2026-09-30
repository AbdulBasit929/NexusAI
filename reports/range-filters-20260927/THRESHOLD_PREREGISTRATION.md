# A1b RANGE FILTERS — THRESHOLDS, WRITTEN BEFORE THE RUN

Written 2026-09-27 after the offline census and the SQL oracle, before the image was built and
before any arm was scored. **Not editable once the first arm starts.**

---

## 1. WHAT THIS CHANGE IS

A1a made a dropped numeric condition **refuse**. A1b makes it **answer**, for the questions the
curated layer can justify: the analyst states a bound (`above 50000`) and names a field the layer
declares range-filterable, and the plan carries `GT/GTE/LT/LTE` on that field.

Nothing downstream changed. `compileSourceNativeFrameFilters` already resolves the field and
enforces `allowed_filters`; `sourceNativeFilterOpForFrame` already maps the operator; the executor
already runs it. **Only the extraction was missing.**

## 2. THE FLAGSHIP EXAMPLE DOES NOT WORK, AND THAT IS CORRECT

*"Show me all the calls that lasted longer than ten minutes"* — the question that motivated this
whole work item — **still abstains after A1b**, and must.

Surveyed 2026-09-27 across the entire curated layer: **not one range-filterable field declares a
`unit`.** The only `unit:` anywhere is on `cdr.call_duration_seconds`, which is a derived METRIC
(`expression: timestamp_diff_seconds(cdr.call_start, cdr.call_end)`) carrying **no
`allowed_filters` at all**. There is therefore no declared basis for converting "ten minutes" to
600, and no filterable duration column to apply it to.

**A1b refuses any condition carrying a unit word.** Inventing the conversion would be a fabricated
number, and filtering a computed metric needs HAVING over an expression — a separate work item.
A1a already withholds this question honestly and names its condition.

## 3. THE CENSUS, RUN BEFORE WIRING

`TestRangeFilterCensus` across all three corpora:

    103 corpus questions examined
      0 would bind a filter
      0 stated a bound but carried a unit

**Zero regression surface.** As with A1a, the corpus is a SAFETY instrument only and cannot prove
the feature works. §5 is the proof.

## 4. THE THRESHOLDS — corpus

Two arms, same image, same container, switch asserted from `docker inspect` before scoring.

    A  control   FORENSIC_RANGE_FILTERS=false
    B  ranges    FORENSIC_RANGE_FILTERS=true

- **Arm A must reproduce 67 CORRECT · 28 CLARIFIED · 4 WRONG · 2 NOT_STATED · 1 MANUAL ·
  1 ROWCOUNT_ONLY**, or the run is void.
- **Any movement of any corpus question in arm B reverts the whole arm.** The census predicts zero.
- **Any new WRONG anywhere reverts the whole arm.**

## 5. THE PROOF OF FUNCTION — an independent SQL oracle

Derived 2026-09-27 by mirroring the executor's own expression,
`COALESCE(r.raw_payload->>'<source_name>', r.metadata->'normalized_fields'->>'<source_name>')`,
**not** by reading the API's output. Collection `nexusai-forensic-demo`.

| Question | Filtered answer | Unfiltered answer |
|---|---|---|
| `How many transactions have an amount above 50000?` | **1** | 4 |
| `How many IPDR sessions have bytes above 1000000?` | **2189** | 2500 |

Retained transaction amounts are `9200, 14500, 31000, 75000` — exactly one exceeds 50000.

**PASS requires, in arm B:**

    both questions answer the FILTERED value (1 and 2189)
    the executed plan carries a GT filter on the named field

**PASS also requires, in arm A:** these two questions do **not** answer the filtered value — they
either state the unfiltered total or are withheld by A1a. If arm A already answers 1 and 2189, the
feature was not what produced it and the measurement is void.

**And, unchanged in both arms:**

    "Show me all the calls that lasted longer than ten minutes"   -> WITHHELD (unit word, §2)
    "How many CDR records do we have in this case?"                -> 8,642

## 6. DECISION RULE

    arm A does not reproduce the shipped posture      -> void, investigate
    any corpus movement in arm B                      -> REVERT the whole arm
    a new WRONG anywhere                              -> REVERT the whole arm
    either probe answers an UNFILTERED value in arm B -> the filter did not bind; do not ship
    the duration question stops being withheld        -> a unit was converted; REVERT immediately
    corpus flat AND both probes answer the oracle     -> SHIP

### 6.1 Default state if it passes

**Default OFF**, unlike A1a. A1b adds capability rather than removing a wrong answer, so its
off-state is safe — an unbound condition is still caught by A1a. It is turned on deliberately after
its own measurement, which is this project's normal discipline.

## 7. WHAT WILL NOT BE DONE

No `docker compose down`, `down -v`, `--remove-orphans`, `system prune`, `volume prune`. No database
write, migration, backfill or reprocessing. No auth disabled. No expectation edited. No evidence
uploaded. Both arms use the same image and the same container.
