# WI-0 — IR-generation feasibility spike: PRE-REGISTRATION

**Written 2026-09-21, before any live inference call was made.** The decision rule in
§5 is fixed by this document. It was not chosen after seeing results, and it is not
revisable by the agent that runs the spike. If the outcome is disliked, the outcome
stands and the escalation in §5 fires.

Owner: claude · Work item: **WI-0** · Files owned: `reports/ir-spike-20260921/**`,
`scripts/ir_spike/**`. **No production source file is modified by this work item.**

---

## 1. The single question this spike answers

> Can `qwen3-4b-instruct-2507-q4km-nxb21d-dev` (Q4_K_M, CPU, ~4 tok/s) fill an
> **enum-constrained analytical IR** (`SourceNativePlanV1`) over a **curated** semantic
> catalogue of ~25–30 described fields, accurately enough to be the S5 stage of the
> Governed Semantic Compiler?

No published number exists for this task at this model size. Every adjacent published
number is for a model **writing SQL**, which this architecture never asks it to do.
The spike exists because that substitution is not safe to assume.

**It is not a model comparison.** Exactly one model is called. Escalation to a larger
tier is permitted only by the rule in §5, only after this measurement, and only by
surfacing the numbers to the user.

---

## 2. What was verified in source before designing the spike

Confirmed by reading the tree on 2026-09-21, not taken from any report:

| Claim | Verdict | Evidence |
|---|---|---|
| `enum` compiles to a GBNF alternation | **TRUE** | `pkg/functions/grammars/json_schema.go:129-138` — `strings.Join(enumRules, " \| ")` |
| `response_format: json_schema` reaches the grammar | **TRUE** | `core/http/endpoints/openai/chat.go:289-303` builds `input.Grammar` from the schema |
| `frame.Filters` has no consumer (D1) | **TRUE** | `plan.Filters` initialised `deterministic_semantic_compiler.go:418`, never read again; only reader of `frame.Filters` in the repo is a test assertion |
| `FieldDescriptorV1` has no description | **TRUE** | `grep -c Description source_native_algebra.go` = 0; zero `Synonym`/`DisplayName` in the non-test tree |
| Demo-case anchors | **TRUE** | SQL: CDR 8,642 · IPDR 2,500 · ANPR 750 · access log 1,000 · subscribers 11 · towers 5 · transactions 4 = **12,912** |

### 2.1 Two findings that change the spike's design

**(a) The enum-constrained IR path already exists in production code.**
`semanticSourceNativeSchema` (`api/forensic_records/semantic_dynamic_plan.go:269-310`)
already builds a request-time JSON Schema whose `field_id` slots are an `enum` of
catalogue IDs, and `resolveSemanticDynamicPlan` already calls LocalAI with
`response_format {type: json_schema, strict: true}`, parses with
`DisallowUnknownFields`, validates and applies. **WI-0 is therefore not "can this be
built" — it is "how well does the mechanism already in the tree actually perform".**

**(b) That path is gated behind the component known to fail.** It is only reached when
the LLM *operation selector* first returns `semanticDynamicDecision` from among ~104
opaque operation IDs (`open_ended_semantic_planner.go:361-368`) — the selector measured
at **0/5**. The sound machinery sits downstream of the broken gate. §6 of the master
prompt deletes that gate and makes the IR path primary, so the spike measures the IR
generator **unsuppressed**, which is the configuration the architecture proposes.

### 2.2 Three constraints the GBNF converter imposes on schema design

Discovered by reading `pkg/functions/grammars/json_schema.go`; all three shape the
schema and are not negotiable:

1. **Every declared object property is emitted unconditionally**, joined by `","`
   (`json_schema.go:177-195`). `required` is never read by the converter. There is no
   optionality: a declared scalar property **cannot** be omitted by the model.
   *Consequence:* anything legitimately absent must be an **array** (which may be empty:
   `"[" (item ("," item)*)? "]"`) or an explicit `null` union — never an absent key.
   `SourceNativePlanV1` is already array-shaped, so this is satisfiable.
2. **Property order is fixed** — by `properties_order` if supplied, otherwise
   alphabetical (`json_schema.go:154-173`). The grammar forces that order, so plan
   comparison must be order-insensitive *after* parsing.
3. **`JsonSchema.Schema` is typed `functions.Item` = `{Type, Properties}`**
   (`pkg/functions/function_structure.go:9-12`), so top-level `required` and
   `additionalProperties` are **dropped on the wire**. Nested constraints inside
   `Properties` survive (it is `map[string]any`). `strict: true` is parsed and then
   never read — the grammar, not the flag, is what enforces the shape.

### 2.3 The field-ID decision (a real design finding, recorded before measuring)

`sourceNativeFieldID` (`source_native_algebra.go:150-154`) mints IDs as
`"fld_" + sha256(scope…)[:24]` — **opaque content hashes**. Asking a 4B model to select
`fld_a3f21c…` from an enum of twenty hashes is a materially harder task than selecting
`cdr.duration_seconds`, and it is a task the model has no way to reason about.

The spike therefore uses **readable, stable, curated IDs** (`cdr.call_type`,
`subscriber.status`). This is pre-registered as a *design recommendation for the semantic
layer (WI-4)*, not a thumb on the scale: the semantic layer is curated and persisted, so
it is free to choose its own ID convention, and the enum mechanism is identical either
way. **A separate arm (§4.3) measures the hashed-ID form**, so the cost of this choice is
quantified rather than assumed.

---

## 3. Corpus and gold standard

**Source:** `reports/nexusai-tl-audit-20260918/golden_questions_v0.json` (62 questions,
every expected value computed by direct SQL independent of the API).

**Selection — 40 governed-analysis questions.** Fixed before any run:

| Family | IDs | n |
|---|---|---:|
| CDR | 01, 02, 04–16 | 15 |
| IPDR | 01–05 | 5 |
| ANPR | 01–06 | 6 |
| Access log | 01–04 | 4 |
| Subscriber | 01–03 | 3 |
| Tower | 01, 02 | 2 |
| Transaction | 01–03 | 3 |
| Negative | NEG-01, NEG-04 | 2 |
| **Total** | | **40** |

Excluded, with reasons: all DOC/IMG/AUD/VID items (20) are `evidence_retrieval`, not
governed analysis — a different compiler branch; **NEG-03** ("blood type") is
`out_of_scope`; **CASE-02** is qualitative; **CASE-04** ("Explain…") is the D7 classifier
defect, not an IR defect; **CDR-03** is a third verbatim paraphrase of CDR-01/02;
**NEG-02** duplicates the NEG-01 abstention shape; **CASE-01/CASE-03** need a
cross-family record entity the semantic layer does not yet model (WI-4).

**Gold plans** are hand-written from the question text against the curated layer, by
reading the question only — never by running the harness and blessing its output, and
never by reading a model response first. Gold plans are committed **before** the live run
and their file hash is recorded in the results report.

### 3.1 Questions the IR provably cannot express

`SourceNativePlanV1` allows aggregates `COUNT · SUM · AVG · MIN · MAX` only
(`source_native_algebra.go:538`). **There is no `COUNT DISTINCT`.** Therefore:

- **CDR-07** "Who talked to the most different people?"
- **CDR-08** "How many unique phone numbers appear as callers?"
- **ANPR-05** "How many distinct license plates were captured?"

are **not expressible** in the IR as it stands, at any model quality. Their gold entry is
`INEXPRESSIBLE`. They are **kept in the corpus** and **scored separately** (§4.4): they
measure an IR gap, not a model failure, and hiding them would flatter both. A model that
*abstains* on these is behaving correctly; a model that confidently emits plain `COUNT`
is reproducing the exact confident-wrong failure this architecture exists to eliminate.

---

### 3.2 AMENDMENT 2026-09-21, before any live call — a fourth inexpressible item

Recorded here rather than silently edited into §3.1, because it changes a
pre-registered denominator.

While hand-writing the gold plans I sampled the real CDR payload and found it has **no
duration column** — the keys are `CALL_START_DT_TM` and `CALL_END_DT_TM` only. So:

- **CDR-15** "What was the longest call?" requires `MAX(call_end - call_start)`. The IR
  has no derived or computed expression, so this is **inexpressible** — a *different*
  gap from the `COUNT DISTINCT` gap, and one the semantic layer must close with derived
  **metrics**, not merely field descriptors. (SQL confirms the oracle 1798 is exactly
  `MAX(end - start)` in seconds.)

**Expressible items therefore drop from 37 to 36**, and the §5 rule is computed over
**36**, not 37. Nothing else changes. This was found by reading data, before any model
was called, and is disclosed for that reason.

This is itself a finding for WI-4: two distinct classes of question — distinct-counts
and derived measures — are unanswerable by *any* model until the IR or the layer gains
those capabilities. That is an architecture gap, not a model-quality gap, and no amount
of model escalation addresses it.

---

## 4. Method

One call per question per arm, `temperature: 0`, `max_tokens: 512`, against live LocalAI
at `http://localhost:8080/v1/chat/completions`, model
`qwen3-4b-instruct-2507-q4km-nxb21d-dev`.

**S4 narrowing is deterministic** — family tag plus lexical overlap of question terms,
synonyms and literal types against the curated layer. It is **not** an embedding ranker
(hard rule; D2 and the measure defect both came from embedding signals on structural
roles). The working set is capped at 30 fields and always contains the question's family.

**The schema is the clean §6 IR** — `contract_version · project · filters · group_fields ·
measures · time_bucket · having · sort · limit` — *not* the legacy
`semanticDynamicSchemaWithFields` wrapper, which additionally makes the model fill
`mode`, `projection`, `sort_by`, `group` and `compare`. Measuring the legacy wrapper would
measure accumulated legacy surface, not the architecture under test.

### 4.1 Arms

| Arm | Description |
|---|---|
| **A — baseline** | Curated layer, readable IDs, one shot, no retry |
| **B — self-correction** | Arm A, plus **exactly one** bounded retry when S6 validation fails, feeding back a typed diff (`"field_id cdr.x is not groupable; groupable fields are …"`). Never a loop. |
| **C — hashed IDs** | Arm A with `fld_<hash>` IDs, to price §2.3's design choice |
| **D — uncurated** | Arm A with descriptions and synonyms stripped, to price the semantic layer itself (this is the WI-4 business case) |

Arms C and D are diagnostic. **Only Arms A and B are scored against the §5 decision rule.**

### 4.2 Metrics (the five the directive requires, plus attribution)

1. **Exact-IR match** — parsed plan equals gold after canonical ordering.
2. **Execution-equivalent match** — plan compiles to SQL returning the same result as
   gold. *This is the metric the decision rule keys on.* Differences that cannot change
   the result set (measure ordering, limit on a scalar, sort on a one-row result) are
   equivalent; a different filter, field, aggregate or grouping is not.
3. **Abstention rate** — model declines or S6 rejects with no valid retry. A correct
   abstention on an ambiguous or inexpressible item is a **success**, reported separately
   from a wrong answer.
4. **Malformed rate** — output that is not schema-valid JSON. **Expected: 0.** A non-zero
   value means the grammar is not being enforced and is reported as a defect in its own
   right, because it falsifies §2's keystone.
5. **Latency** — p50 / p95 / max wall-clock per call.

Each failure is attributed to exactly one of: `wrong_field` · `wrong_aggregate` ·
`missing_filter` · `spurious_group` · `wrong_shape` · `inexpressible` · `malformed` ·
`timeout`. Attribution is what makes the result actionable rather than a single number.

### 4.3 The metric that actually gates the product

Recorded for every item and reported with equal prominence:

> **Confident-wrong rate** = plans that are schema-valid, pass S6, execute, and return a
> **wrong** result without abstaining.

The product baseline is **42%** (26/62). This is the number the architecture exists to
drive to zero. A high abstention rate with a near-zero confident-wrong rate is a **better**
outcome than the reverse, and §5's rule must be read with that in mind.

### 4.4 Scoring of the inexpressible three

Scored separately and never folded into the headline. For CDR-07/08 and ANPR-05:
`abstained` = correct · `emitted plain COUNT` = **confident-wrong** · `emitted anything
else` = wrong.

---

## 5. DECISION RULE — binding, fixed before execution

Computed on **execution-equivalent match**, over the **36 expressible** items of the 40 (see the §3.2 amendment),
taking the **better of Arm A and Arm B**:

| Result | Verdict | Action |
|---|---|---|
| **≥ 80%** | **PROCEED** | 4B model is sufficient. Build Phase 3 as specified in §6. |
| **60 – 79%** | **PROCEED, REINFORCED** | Keep the 4B model. Widen S4 narrowing. Add `qwen3-8b-q4km-nxb21d-selection-dev` (already present locally — no download) as a **fallback tier for low-confidence plans only**, never as the default. |
| **< 60%** | **STOP** | Do **not** try another model. Report the numbers to the user and escalate. The next step is the user's decision, not the agent's. |

**Two overriding conditions, either of which forces STOP-and-escalate regardless of the
percentage above:**

- **Malformed rate > 0 in Arm A.** The enum/GBNF keystone is the security control that
  makes a hallucinated field structurally impossible (§11 of the master prompt). If it
  leaks even once, the constraint is not what the architecture believes it is, and that
  must be resolved before anything is built on it.
- **Confident-wrong rate > 10%** on expressible items. The product's whole thesis is that
  verification plus abstention converts residual error into clarifying questions. An IR
  stage that is confidently wrong more than one time in ten hands S9 more than it can
  filter.

**Committed in advance:** the numbers are reported exactly as measured, including a STOP.
No additional model is called, no prompt is tuned after seeing scores and re-run as if it
were the first attempt, and no threshold is reinterpreted. Prompt iteration, if any,
happens **before** the scored run and is disclosed with the count of iterations.

---

## 6. Stop condition

One report containing the five metrics, the confident-wrong rate, the failure
attribution, and the rule's verdict. **Then stop.** WI-0 does not modify production
source, does not start Phase 3, and does not proceed to WI-1 without reporting first.

---

## 7. Approval required before execution

Per §11 of the master prompt, the live run (40 questions × up to 4 arms on a ~4 tok/s CPU
box, materially slower under the self-correction arm) is **explicitly gated on user
approval** and has not been started. Everything in this document, the curated layer, the
gold plans and the harness are produced first so the run is a single reviewable action.
