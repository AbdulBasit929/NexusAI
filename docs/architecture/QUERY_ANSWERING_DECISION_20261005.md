# Query answering: the decision (2026-10-05)

**Status: binding addendum to `RECONCILIATION_20260921.md` §G (it amends G.5 rule 1) and the completion of `docs/work/RUNTIME_QUERY_PLAN_20260929.md` Phases 2 and 3.**
Where this file and an older planning document disagree, this file wins; code and live behaviour still outrank every document (`AGENTS.md` precedence).

## The decision

An analyst asks anything about a case in their own words. The system answers from the database, in one of **three outcomes, never a fourth**:

1. **Answered**, with a number the database computed, the query that produced it, and the conditions it was checked against.
2. **Abstained**, saying which condition of the question it could not apply, and what to change. No number is shown anyway. This is a success.
3. **Declined**, with no opinion: the request continues down the existing path exactly as it does today.

A confident answer to a different question is the one defect that must reach zero (`CONTINUATION §5`). Two lanes produce answers, behind one verification layer:

```
question ─► front door (conversation, concepts, help; never sees case data) ─► data question
         ─► LANE 1  typed plan -> deterministic compiler          exact, but says only what its algebra can say
         ─► LANE 2  governed SQL (built 2026-10-05, default off)  the model writes ONE SELECT over analyst views
                      views built by the server from the curated semantic layer (visible, non-sensitive fields only)
                      validator: parse-tree allowlist · verifier: every condition of the question is in the query
                      executor: server-built scope · read-only · timeouts · planner-cost ceiling · row cap
         ─► deterministic answer (no model narrates a number) + the query shown
```

## Why this, and why we were stuck

* **The architecture was already decided twice** (`RECONCILIATION §G`, 2026-09-21; `RUNTIME_QUERY_PLAN`, 2026-09-29) and an independent re-run of the research on 2026-10-05 reaches the same answer.
  The 2026 evidence: a bounded planner with a deterministic compiler reached 97.4% correct against 55.3% for direct SQL (arXiv 2608.16663); a semantic layer lifted frontier models from 84–90% to 98–100% and the layer
  *refuses outside its coverage* where raw SQL still reaches 70% (dbt, 2026); a rule-gated 7B agent beat a direct-prompted 32B on business correctness by abstaining (arXiv 2608.09254); and no architecture is safe from low
  scores on a hard set (SemPlan, arXiv 2608.13612). So: both lanes, one verification layer, and measurement before any claim.
* **Phase 2 and 3 of the 2026-09-29 plan were never built.** No `describe_schema`, `find_value`, checked `run_sql`, SQL parser, analyst views or value index existed in the code. Work went into guards on the keyword ladder instead.
* **The ladder answers first and pre-empts the correct components.** `identifier_binding.go` and the constraint obligations are correct and held OFF because the ladder claims the question before they run (`CONTINUATION §3`, `identifier_binding.go` STATUS).
* **Two binding documents disagreed.** `RECONCILIATION §G.5 rule 1` said the model never emits SQL; `RUNTIME_QUERY_PLAN` and the product owner's direction say the model creates the query. They are reconciled below.
* **Accuracy was judged on the 103 questions the system was tuned on.** The question factory (`evaluation/question_factory/`) generates unseen questions with an independent SQL oracle; first baseline on 82 questions: 34 correct, 23 confidently wrong, 25 abstained.

## The amendment to §G.5 rule 1

> The model **never writes SQL into the typed lane** and never names a table there. **Model-written SQL exists only in the governed SQL lane**, and only as a single `SELECT` over analyst views the server built,
> after the parse-tree validator and the verifier have accepted it. The model is never shown case data, and no model narrates a number.

Every other §G.5 rule stands: every field the model can reference is enumerated; every extracted literal is an obligation; every query is parameterized or server-scoped; abstention is a success; the deterministic answer is always produced.

## What each family gets

| Evidence | How it is answered | State |
|---|---|---|
| Structured records (CDR, IPDR, ANPR, access log, subscribers, towers, transactions) | views from the curated layer; typed lane first, SQL lane for the long tail | **built** (switch off) |
| Derived media metadata (OCR confidence, plate groups, audio segment timing, image technical metadata, face-candidate counts) | views over `forensic.derived_artifacts`, curated non-text fields only | **built** (switch off); not mixed with ingested records |
| Text evidence (OCR text, transcripts, documents, Urdu and Roman Urdu) | retrieval: full-text, trigram, vector, fused; the curated sensitivity applied to every path; "not found" only after a complete search | existing path; roadmap **R5** |
| Names of people | not answerable: no field holds one. The lane abstains and asks for a number | built |
| Questions that span two families | declined in v1: a join is only attempted where one is declared and verified (`CONTINUATION §7`: none works today) | by design |
| Conversation, concepts, product help | the front door | built (default off) |

## What exists, and what is next

| Step | What | State |
|---|---|---|
| R0 | the laptop's backend work pushed to a branch | **done** 2026-10-05 |
| Lane 2 | catalogue, validator (172 specs, 100+ hostile queries), executor (14 live-database specs), verifier, prompt, loop, two entry modes, 31 end-to-end specs, 0 lint issues | **built** 2026-10-05 |
| M1 | measure it: arms A/B/C on the factory set against the gates in `reports/governed-sql-20261005/PREREGISTRATION.md` | **next** (owner runs; one paste per arm) |
| R2 | give the typed lane the same verifier, so a dropped condition abstains there too | after M1 |
| R3 | add earliest/latest, time-of-day and others to the typed algebra **only** where M1 shows lane 2 is too slow or too error-prone for them | data-driven |
| R5 | retrieval path with the curated sensitivity; then, and only then, the front door may promote | after M1 |
| R6 | retire the ladder family by family, once lane 1 + lane 2 equal or beat it on the factory set, the 103 and the pre-flight | after M1, M2 |
| R7 | speed (shared prompt start; thread tuning; a second RAM stick for dual channel) | parallel |
| R8 | model bake-off under a decision rule written first | only if M1 shows failures the model causes |
| R9 | the read-only role that can read only analyst views; case registry and identity | **owner decision** (database change) |

## Hardware and model, as measured

This laptop is an i7-1260P with 15.7 GiB of RAM, CPU only, about 4 tokens per second, so a model-written query is tens of seconds and a retry doubles it. A mixture-of-experts model with 3 billion active
parameters decodes about twice as fast as a dense 4B at the same memory bandwidth, but its 4-bit weights need about 18 GiB and do not fit here. The levers that do apply, each measured before it is kept:
the shared prompt start (the schema is in the system message so a second question about the same family pays only for its own tokens), a second RAM stick (dual channel), and shorter outputs.
**No model is swapped without a decision rule written first** (`CONTINUATION §5`).

## What we will not do

* add operation templates, or extend the keyword ladder (frozen; deleted at the end of Phase 3);
* use embeddings for structural decisions (family, group, measure, field role): the lane's view shortlist is lexical and curated;
* let the model see case data, or narrate a number;
* run a model-written query outside the server-built scope, outside a read-only transaction, or unpriced;
* answer a question by silently dropping a condition it cannot bind;
* say "none found" before the all-families identifier search;
* iterate on prompt wording after a gate fails: two failures on one item mean a failure taxonomy and a smaller scope.

## Superseded where they conflict (kept for provenance)

| Document | Status |
|---|---|
| `RECONCILIATION_20260921.md` §G.5 rule 1 | amended above; the rest of §G stands |
| `docs/work/RUNTIME_QUERY_PLAN_20260929.md` | Phases 2 and 3 are implemented by the lane and the roadmap below; its decisions list still applies |
| `docs/work/ROADMAP_FREE_QUESTION_20261005.md` | consistent; its R-steps are the table above |
| `docs/work/ROADMAP_20260929.md` | superseded by the two above |
| `docs/work/FREE_QUESTION_GOAL_20261002.md` | the goal, unchanged |
| everything under `docs/work/CODEX_*`, `UI_*`, `WI-*` | not about query answering |

## Evidence

Internal: `reports/free-question-baseline-20261002/`, `reports/governed-sql-20261005/PREREGISTRATION.md`, `evaluation/question_factory/`, `docs/research/ARCHITECTURE_20260929.md`.
External (2026): dbt Labs, *Semantic Layer vs. Text-to-SQL* · arXiv 2608.16663 (bounded semantic planning) · 2608.09254 (rule-gated 7B) · 2608.13612 (SemPlan) · 2606.31041 (semantic-layer-mediated agent) ·
2501.10858 (adaptive abstention) · CIDR 2026, *Text-to-SQL Benchmarks are Broken* · Arctic-Text2SQL-R1 (arXiv 2505.20315) · ReFoRCE (2502.00675) · `wasilibs/go-pgquery` (MIT, pure Go).
