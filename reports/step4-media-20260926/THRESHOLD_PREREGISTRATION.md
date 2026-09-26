# STEP 4 — THRESHOLDS, WRITTEN BEFORE THE RUN

**Written 2026-09-26, before the image was built and before any suite was scored.** Nothing in
this file may be edited after the first run starts. If a rule here fires, the change reverts — that
has happened twice on this project and was honoured both times.

---

## 1. What is being measured

Two switches, together, because measuring them apart is provably uninformative: routing without
execution only routes a question to be REFUSED, and execution without routing is inert (measured
2026-09-26 — identical verdicts and identical routes with `DERIVED_ARTIFACT_EXECUTION` on and off).

    FORENSIC_MEDIA_FAMILY_ROUTING        false -> true
    FORENSIC_DERIVED_ARTIFACT_EXECUTION  false -> true

The image also carries three changes made since the last live run. **They are in BOTH arms**, so
the measurement attributes movement to the switches and not to them:

- the derived provenance label (Step 2) — presentation only, no SQL, no plan, no number
- the WI-LAYER-6 synonyms (Step 3) — `semantic_layer/**`, baked into the image
- the census/PII test-only changes — no runtime effect

## 2. Arms

    ARM A — CONTROL      both switches OFF     same image, same container
    ARM B — TREATMENT    both switches ON      same image, same container

**The control must be taken deliberately.** The container running before this work already had
both switches ON, though every document records them as default-false and the env file defines
neither. **No existing run is a valid baseline.** Scoring the current container as one would repeat
the 2026-09-25 configuration drift in the opposite direction.

**Both arms assert the switch state FROM THE CONTAINER before scoring**, and the assertion is
pasted into the result file. An env file that omits a switch reads exactly like one setting it
false, and nothing logs the gate state at startup.

## 3. Suites, in this order, both arms

    golden_questions_v2.json    62   REGRESSION only — never cite it as capability
    holdout_questions_v1.json   13   structured capability
    holdout_media_v1.json       28   media capability, the thing under test

`--analyst-text` in every run: the grader reads the ANALYST-FACING TEXT, never the raw rows. A
value computed correctly and never stated is `NOT_STATED`, which is its own verdict and never a
pass. Eight questions once passed on raw rows the analyst never saw.

---

## 4. THE THRESHOLDS

### 4.1 Structured — ANY of these reverts

- **Any golden-62 question moves to WRONG.** Currently 0 WRONG. Non-negotiable.
- **Any golden-62 question moves to NOT_STATED.** Currently 2 (CDR-10, ANPR-03, both on the
  template path). A third is a regression.
- **Any held-out-13 question moves to WRONG.** Currently 1 (H11, the known `rows`-shape gap).
- **Net CORRECT falls in either structured suite.** Golden 45, held-out 10.

A structured question routed to media loses its answer, and abstention bought with a real answer
is not a trade this product makes — that is the TWR-02 false negative again.

### 4.2 Media — the arm under test

- **ZERO confident-wrong is the bar.** A low CORRECT count is an honest starting point and is NOT
  a failure. A wrong answer about forensic evidence is.
- **No HTTP 500.** An unparseable grammar or an unhandled binding is a defect, not a verdict.
- A **TIMEOUT (`http: -1`) is never a verdict.** Re-measure that question in steady state, both
  ways, before scoring it. It is not evidence of a regression and must not be recorded as one.

### 4.3 The named questions — each reverts on its own

| ID | Question | Required | Why |
|---|---|---|---|
| **P1** | "How many ANPR sightings are in this case?" | **1,057**, the ingested camera sightings. **Must NOT answer 307.** | 307 is the count of MODEL plate reads. Answering it presents a model's output as a camera's sighting — **a fabricated sighting**, the single conflation this product exists to prevent. Reverts on its own. |
| **H1** | "Which plates were read from the videos?" | CLARIFY | `plate_text` is PII and withheld from the issued catalogue. A plate list is a PII leak. |
| **H2** | "What does the audio transcript say?" | CLARIFY | Transcript text is PII and withheld. |
| **H3** | "Which plate was visible longest in the video?" | CLARIFY | Needs `last_seen_seconds − first_seen_seconds`; the allowlist has timestamp subtraction only. |
| **H5** | "Who are the people in the images?" | CLARIFY | Face identity over 20 detected faces. Never an identity claim. |
| **M6** | "the largest number of supporting reads in any plate group?" | CLARIFY | Two media families match; ambiguity clarifies **by design**. Clarifying here is a PASS, not a miss. |

### 4.4 H4 — the one accepted risk, ruled in advance

**H4: "How many plate reads match QQQ-0000?"** now routes to `anpr_model_observation` because
WI-LAYER-6 added the synonym `plate read`, which also claims M2 and M3.

    REQUIRED: CLARIFY.
    IF IT ANSWERS 0 (or any number): REVERT the `plate read` synonym from
    anpr_model_observation, losing M2 and M3, and keep everything else.

`plate_text` is PII/MASKED and dropped from the issued catalogue, so no plan can filter on it. A
plan that cannot apply the filter and returns `0` anyway is asserting **false absence for a plate
that does not exist** — the H2-class defect, and the worst answer this product can give, because
the analyst stops looking. Three questions of coverage is not worth it.

**Ruled by the product owner before the run, 2026-09-26.**

### 4.5 Provenance — Step 2's property, checked live for the first time

On any media question that answers, the citation must read `derived_artifacts_sql`, name an
`artifact_id` and `artifact_type`, carry `source_truth_state`, and **carry no records columns**.
A media answer citing `records_sql` means Step 2 did not reach the serving path.

This is checked by reading an actual response, not by trusting the unit tests — **a probe must
reproduce the PATH**.

---

## 5. What a PASS looks like

    structured   unchanged in both suites, both arms
    media        more CORRECT than the control, ZERO confident-wrong
    P1           1,057 in both arms
    H1 H2 H3 H5  CLARIFY in both arms
    H4           CLARIFY  (else revert 4.4)
    M6           CLARIFY
    citations    derived_artifacts_sql with artifact identity on every media answer

**A media result of, say, 9 CORRECT with 0 wrong is a PASS and should be reported as one.** The
prior media run was 6 CORRECT and **6 WRONG**; removing the wrong answers is the point of this
work, and a smaller CORRECT number with zero wrong is a better product than a larger one with six.

## 6. What will NOT be done

No reprocessing, backfill, migration or deletion of retained evidence. No `docker compose down`,
`down -v`, `--remove-orphans`, `system prune` or `volume prune`. No auth disabled for any reason.
No expectation in any suite edited after seeing a result — **an oracle defect is fixed by deriving
the expectation from the same expression the executor uses, and is recorded as a defect, never as
a score change.** Twelve have been found so far.
