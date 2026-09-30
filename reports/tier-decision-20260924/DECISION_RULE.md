# Model-tier decision rule — WRITTEN BEFORE ANY MEASUREMENT

Date: 2026-09-24 · Owner: claude (compiler track)
Status at writing: 45 CORRECT · 16 CLARIFIED · 1 MANUAL · 0 WRONG.
P3 gate fails on ONE criterion: abstention 16/62 = 25.8% against ≤20%.

## Why this document exists before the work

`ANTI-MODEL-ROULETTE RULE (absolute)`: do not evaluate, benchmark, download or
swap LLMs except against a threshold written down in advance and surfaced to the
owner. Two operation selectors already failed at 0/5, at a task the architecture
deleted. A model benchmark is never a product milestone.

## The finding that reframes the question

The request to measure a model tier is answered first by asking whether the
model was ever TOLD. It was not.

The IR system prompt (`semantic_dynamic_plan.go`, clean-schema branch)
enumerates the aggregates:

> Count rows with `{"measure_id":"m1","op":"COUNT","field_id":""}` — COUNT is
> the only aggregate allowed an empty field_id. A total or sum uses SUM, the
> largest MAX, the smallest MIN; never answer "how much" or "largest" with
> COUNT.

**`COUNT_DISTINCT` is not mentioned anywhere in it.** Every other aggregate has
an instruction; the one CDR-08 and ANPR-05 need has none. The legacy branch is
worse — it says *"Count means source rows, never distinct entities"* with no
alternative offered.

Captured under `FORENSIC_IR_SHADOW=true` on 2026-09-24, CDR-08's plan is a plain
`COUNT` of rows. That is the behaviour the prompt describes.

Escalating a model tier to recover an aggregate the prompt never taught would be
exactly the roulette the rule forbids: gigabytes and hours to work around a
one-sentence omission.

## Scope of the affected questions

Of the 16 clarifications, five are bucket A (family resolved, full curated enum,
no verifiable plan). Their measured causes:

    CDR-08   plain COUNT for a distinct question   prompt omits COUNT_DISTINCT
    ANPR-05  same shape, same omission             prompt omits COUNT_DISTINCT
    CDR-11   invented filter cdr.direction=outbound
    TWR-02   invented filters site_location CONTAINS "where", district "area"
    SUB-03   PII withheld from the enum BY DESIGN; its gold demands a raw CNIC

SUB-03 is excluded from every threshold below: satisfying its gold means leaking
a CNIC, and the clarification is correct behaviour.

## STEP 1 — prompt adequacy, measured before any tier question

Two sentences are added to the clean-schema system prompt, each targeting one
measured failure and nothing else:

1. distinct counting maps to `COUNT_DISTINCT` over the named field;
2. a filter value must appear in `literal_values`, so a question word cannot
   become a filter.

Measured on the full 62 with `--analyst-text`, cache off for the affected
questions.

**PRE-DECLARED THRESHOLDS — binding:**

- **Any question moves to WRONG → REVERT IMMEDIATELY.** A confident wrong answer
  is never traded for a conversion, at any ratio. This has been refused four
  times in this project and is refused here.
- **≥2 of {CDR-08, ANPR-05, CDR-11, TWR-02} convert to CORRECT, 0 lost → KEEP.**
  The prompt was the defect; the tier question does not arise.
- **1 converts, 0 lost → KEEP** (a real gain at no cost) **and proceed to STEP 2**
  for the remainder.
- **0 convert, 0 lost → REVERT the prompt change** (no measured benefit, and
  prompt length costs latency on a 4 tok/s CPU) **and proceed to STEP 2.**

## STEP 2 — the tier question, only if STEP 1 leaves it open

Not started without the owner's explicit approval, because it requires
downloading a model — an ASK-FIRST action — onto a host with ~15.7 GiB RAM and a
6 GiB deployment floor.

Ladder, unchanged: Qwen3-4B-Instruct Q4_K_M (current) → Qwen2.5-Coder-7B or
Qwen3-8B Q4 → 14B Q4 (GPU) → 32B (GPU).

**PRE-DECLARED THRESHOLD for any tier change:** on the remaining bucket-A
questions plus the full 62 as a regression set, the candidate tier must convert
**≥3 questions to CORRECT with 0 new WRONG and 0 CORRECT lost**, at a p95 within
the P3 gate on the cached path. Anything less is not a tier problem and the tier
is not changed.

**Explicitly NOT a reason to escalate:** a question failing for a cause already
attributed to the prompt, the schema, a guard, the catalogue or the gold set.
Five of this session's defects had exactly those causes and none was the model.

## What is NOT on the table

Accepting unverified answers; loosening a guard to raise the conversion count
(measured and rejected — Fix B created two confident-wrong); or adding an
operation template (the catalogue is frozen).
