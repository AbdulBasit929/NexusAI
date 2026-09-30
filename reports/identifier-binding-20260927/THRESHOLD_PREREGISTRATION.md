# A3.3 IDENTIFIER BINDING + DETERMINISTIC ARBITRATION — THRESHOLDS, WRITTEN BEFORE THE RUN

Written 2026-09-27, after the offline probe and before the image was built or any arm was scored.
**Not editable once the first arm starts.**

---

## 1. WHAT WAS MEASURED BEFORE BUILDING

Run directly against the live catalogue with no model (`projection_compiler_probe_test.go`):

    SUB-03  CONSTRAINT_UNBOUND:identifier PK-SUB-SYN-ALPHA
    H11     CONSTRAINT_UNBOUND:phone 923001110001
    NEG-02  CONSTRAINT_UNBOUND:identifier ZZZ-0000
    every frame: identifiers=1 filters=0

**The deterministic compiler had no path from an extracted identifier to a plan filter.** And for
TWR-02 in production it never ran at all (`operation_latency_ms 0`, `registry_candidate_count 0`):
the ladder claims it with `canonical_records`, VERIFIED_ONLY is about to withhold it, and
arbitration only tries the LLM IR generator.

`TestBucketAGoldPlanConformance` confirms the correct TWR-02 plan **validates** today — "the gap is
generation".

## 2. THE TWO CHANGES, SEPARATELY SWITCHED

    FORENSIC_IDENTIFIER_BINDING          bind an identifier to the ONE curated string field holding it
    FORENSIC_DETERMINISTIC_ARBITRATION   let arbitration try the deterministic compiler first

Binding is narrow. Routing is broad — it touches every question about to be withheld or clarified.
Two switches keep a red result attributable.

**The binding rule** — bind only when the value occurs in **exactly one** curated, non-sensitive,
EQ-filterable string field, checked over the full authorized scope with the executor's own
expression and literal cast. Zero fields: nothing binds. Two or more: nothing binds.

**Three hazards found and closed offline, before any deploy:**

1. **Case parity.** The first presence check compared `lower(btrim(...))`; the executor renders EQ
   as `(typedExpr) = castLiteral($v)` — case-sensitive. A lowercase-typed identifier would have
   been "found" and then matched zero rows at execution: a confident "none found" about a tower
   that exists. The check now uses the executor's own expression and cast.
2. **The catalogue was never built.** `applyDeterministicSemanticCompiler` skips the catalogue when
   a registered operation matches, so binding received an empty catalogue and TWR-02 stayed
   registered. It now builds the catalogue when a bindable identifier is present.
3. **SUB-03 would have become a non-answer.** Binding matched `subscriber_id` and projected the row
   — while name and CNIC, the actual answer to "who", are PII and excluded. A WHO question about an
   entity that declares PII now binds nothing and refuses as it does today.

**Probe after the fixes, through the real entry point:**

    TWR-02   semantic_deterministic_dynamic   tower.site_code EQ PK-LHR-SYN-001
    ANPR-02  semantic_deterministic_dynamic   anpr.plate_number EQ LHR-2026
    SUB-03   AMBIGUOUS_INTENT                 identity withheld, refuses
    H11      AMBIGUOUS_INTENT                 phone in caller AND callee, ambiguous, refuses
    NEG-02   AMBIGUOUS_INTENT                 value exists nowhere, refuses

## 3. THE ARMS

    A  control   binding=false  arbitration=false
    B  binding   binding=true   arbitration=false
    C  both      binding=true   arbitration=true

Same image, same container, both switches asserted from `docker inspect` before each arm.

## 4. THRESHOLDS

### 4.1 Arm A must reproduce the shipped posture exactly

    CORRECT 67 | CLARIFIED 28 | WRONG 4 | NOT_STATED 2 | MANUAL 1 | ROWCOUNT_ONLY 1

### 4.2 Revert the arm if, in B or C

- **Any question CORRECT in arm A is not CORRECT.** This protects every honesty and provenance
  probe too: a probe that correctly refuses is scored CORRECT, so a probe that starts answering
  fires this rule.
- **Any question becomes WRONG.**
- **p95 latency over the golden suite exceeds 15 s**, the P3 gate. Arm C adds a deterministic
  compile to every question about to be refused.

### 4.3 Predictions, written now

    arm B   ZERO movement. Binding only reaches questions the ladder has abstained on, and no
            corpus question that the ladder abstains on names a single-field identifier.
    arm C   TWR-02  CLARIFIED -> CORRECT, stating 31.5204 and 74.3587
            SUB-03  stays CLARIFIED   (identity guard)
            H11     stays WRONG       (ambiguity guard; binding cannot fix it)
            CDR-11  stays CLARIFIED   (phone in two fields; any plan compiled without the filter
                                       is caught by `answerNamesUnfilteredTarget` pre-execution)

**Any other movement in arm C must be named and explained before anything ships**, including an
improvement. Arm C's surface is every refused question, so movement beyond TWR-02 is plausible —
media aggregates in particular may compile once the deterministic compiler is reached. Each such
movement is judged on its own verified answer, not accepted as a bonus.

## 5. DECISION RULE

    arm A does not reproduce                      -> void, investigate
    B moves anything                              -> the arm-B prediction is wrong; explain before C
    any CORRECT lost, or any new WRONG            -> REVERT that arm
    p95 > 15 s                                    -> REVERT that arm
    C clean, TWR-02 correct, every move explained -> SHIP both, default ON
    C clean but TWR-02 did not move               -> the routing does not reach it; do not ship C

## 6. WHAT WILL NOT BE DONE

No `docker compose down`, `down -v`, `--remove-orphans`, `system prune`, `volume prune`. No database
write, migration, backfill or reprocessing. No auth disabled. **No expectation edited.** No evidence
uploaded.
