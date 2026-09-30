# Latency — the serving path was wrong, and the instrument was blind

**Date:** 2026-09-24 · Rollbacks: `nexusai-forensic-records-api:rollback-before-irtiming-20260924`,
`…rollback-before-embedbatch-20260924` · stale LocalAI retained as a stopped container.

## 1. All inference was going to a stale binary

`nexusai-api-1` (the container rebuilt 2026-09-22 for the GBNF `maxItems` fix) had **no
network attached and no published port**. It could not bind 8080 because
`nexusai-api-1-prev-gbnf` — kept as a rollback — still held it. The forensic API calls
`host.docker.internal:8080`, so **every inference since 2026-09-22 went to the pre-fix
binary**:

    nexusai-api-1            /local-ai  208,173,354 B  2026-09-22  md5 521f7d9a…
    nexusai-api-1-prev-gbnf  /local-ai  165,585,624 B  2026-09-03  md5 807a45b8…

The stale container also held **5.037 GiB of the 7.611 GiB Docker limit** while the active
one sat at 53 MiB with no model loaded.

Fixed by stopping the stale container (retained, not removed) and attaching the current one,
which then bound 8080 on its own existing mapping. `LOCALAI_KB_URL` needed no change.

**Correction to the record:** WI-7 attributed part of its 16% → 68% plan-exact improvement to
the GBNF `maxItems` fix. That fix was never in the serving path. The improvement came from
the five Go-side defects fixed in the same work item.

## 2. The instrument was blind — twice

`llm_latency_ms` is assigned ONLY inside the bounded-synthesis block, so:

    CDR-02   total=119,835 ms   db=988 ms   llm=0   kb=0

119 seconds attributed to nothing, on a question whose SQL took under a second.

`ir_generation_ms` now records enum-constrained generation inside
`resolveSemanticDynamicPlan` — the one place it is spent, covering all three callers.
Timing the *wrapper* first was wrong and the measurement said so: three of four probes
reported 0 ms while taking 29–75 s.

Where generation runs it is **~93% of the request** (SUB-01: 31.6 s of 34.0 s).

## 3. Catalogue embedding issued one HTTP call per descriptor

`semanticEmbeddingCatalogBatch = 1`, undocumented. A 66-descriptor catalogue issued **66
sequential `/v1/embeddings` calls** at ~2 s apiece — ~17 s per question, invisible to every
telemetry field.

LocalAI batches correctly: 3 inputs returned 3 order-preserved 1024-dim vectors in 0.9 s
against ~6 s issued singly. Raised to 32.

    embedding calls per 4 minutes of probing:   dozens per request  ->  1 total

## Measured result

| Question | Golden run 2026-09-23 | Now |
|---|---:|---:|
| CDR-01 | 5.7 s | **4.4 s** |
| IPDR-01 | ~4 s | **3.7 s** |
| ANPR-01 | ~4 s | **3.6 s** |
| ACC-01 | ~4 s | **3.7 s** |
| SUB-02 | ~3 s | **3.4 s** |
| AUD-02 | 3.3 s | **2.6 s** |
| SUB-01 | 94.7 s | **29.1 s** |
| CDR-16 | 63.8 s | **30.3 s** |
| TWR-01 | 83.9 s | **78.0 s** |

**Correctness unchanged:** 14-question regression sample returned 11 CORRECT · 3 CLARIFIED ·
0 WRONG, with CASE-01, IMG-04 and X-01 clarifying by design exactly as before.

Most questions now answer in 3–4 s, inside the P3 gate of p95 < 15 s. The remainder are the
questions that trigger IR generation.

## Next lever — a third untimed call

Five questions produced five `/v1/chat/completions` calls while reporting `llm=0` and
`ir_generation_ms=0`. Roughly 16 s each, timed nowhere. Candidate callers:
`query_hybrid_planner.go:497`, `open_ended_semantic_planner.go:361`,
`semantic_slot_assist.go`. Instrument it before optimising it — that order has now paid off
twice in one session.

---

## CORRECTION — the latency claim above was measured in a favourable window

An earlier draft of this report claimed "most questions now answer in 3-4 s,
inside P3's p95 < 15 s". That is **only true of the deterministic path**, and the
numbers that supported it were taken in one window that did not reproduce.

Re-measured later the same session, the SAME queries gave:

    ANPR-04   16.8s  ->  150s / 91s / 74s   (three consecutive runs, identical query)
    CDR-16    18.8s  ->  154s
    IPDR-04   17.8s  ->  152s

A 10x swing on identical input. `docker stats` showed LocalAI at **791% CPU**
with memory healthy (4.0 of 7.6 GiB) and the host at 23% of 16 logical cores, so
it is not starvation by another process. LocalAI logs show
`http: proxy error: context canceled` — calls exceeding the client timeout and
being abandoned, after the time was already spent.

**What IS reliably true:**

| Path | Latency | Inside P3 gate? |
|---|---:|---|
| Deterministic (no model call) | 0.3 - 4.2 s | YES |
| Model path (generation / residual choice) | 74 - 152 s | NO, by 5-10x |

CDR-01 4.2s · SUB-02 3.6s · DOC-02 0.5s · AUD-02 2.9s · CASE-01 0.6s ·
IMG-04 0.3s · X-01 0.9s — all deterministic, all fast, all correct.

**So the lever is not "make generation faster" but "generate less often."** Every
structured question the ladder cannot verify currently pays for a full
enum-constrained generation via arbitration. That is a policy question, not a
tuning one.

## The instrument is better but still incomplete

`ir_generation_ms` records only when `req.SemanticPlannerAudit != nil`. TWR-01
reported 149.9 s of 152.5 s (98% accounted); ANPR-04, CDR-16 and IPDR-04
reported 0 while taking 74-154 s, because no audit was attached on their path.

`decision_latency_ms` (hybrid residual choice) reported 0 on every probe, so
that call is NOT the cost — one hypothesis eliminated.

**Before any further latency work:** attach the audit on every path so timing is
unconditional, then benchmark with a fixed methodology — N repetitions, warm
model, recorded host load — because single samples in this environment are not
trustworthy to within an order of magnitude.

## Correctness is unaffected by all of today's changes

Two independent samples after the changes landed:
  14 questions -> 11 CORRECT · 3 CLARIFIED · 0 WRONG
   8 questions ->  5 CORRECT · 3 CLARIFIED · 0 WRONG
CASE-01, IMG-04 and X-01 clarify by design, exactly as in the full run.
