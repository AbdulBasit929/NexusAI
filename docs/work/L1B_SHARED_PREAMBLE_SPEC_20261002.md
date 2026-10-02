# L1b: a shared fixed preamble for every model call (spec and pre-registered thresholds)

Written 2026-10-02, before any measurement of this change. Status: **proposal; nothing implemented, nothing run.** It builds
on `reports/laptop-speed-20260929/` (L1a failed; its RESULT.md named L1b as the next lever) and on the M0 measurements below.

## What was measured (2026-10-02, this laptop, `scripts/diagnose_cpu_inference_m0.ps1`)

i7-1260P, one 16 GB DDR4-3200 module (single channel, estimated peak 25.6 GB/s), Qwen3-4B Q4_K_M (2.33 GB file), 8 threads,
`parallel:1`, context 4096, no container limits, Docker VM memory not under pressure (about 3.3 GB available, swap unused).

| Quantity | Measured | Note |
|---|---|---|
| Writing speed (decode) | 6.9 to 9.0 tok/s over four passes | 63% to 82% of the single-channel peak: memory-bound |
| Reading speed (prefill), 172-token prompt | 44 to 48 tok/s | |
| Reading speed, 2,301-token prompt | 60.67 s, about 38 tok/s | slower per token on longer prompts |
| Same 2,301-token prefix, second to fourth request | 1.04 s each | the prompt cache works through the live stack |
| First request after a gap | 7.5 to 9.6 s against 3.5 to 3.9 s | cold effect |
| Threads, power plan, container limits, memory pressure | not limiting | threads 8 matches the four fast cores; High performance did not help |

The 2026-09-29 and 2026-09-24 reports say the same from the other side: generating questions took a median 72 to 138 s,
99.8% of it plan (IR) generation, and an identical-start prompt dropped from 45 s to 0.7 s.

## The problem in the code

Each call builds `system` plus a user message whose start differs from question to question, so only the system prompt
(about 200 to 250 tokens) can ever be reused:

- `api/forensic_records/semantic_dynamic_plan.go` ~line 560: the user message is `json.Marshal(map[string]any{"question",
  "family", "literal_values", "bound_from", "bound_to", "issued_fields"})`. Go writes map keys alphabetically, so the
  message starts `{"bound_from":...,"bound_to":...,"family":...,"issued_fields":[...retrieved for THIS question...]`. Everything
  after the first differing token (the dates) is read cold, and `issued_fields` is the largest part.
- The selection, arbitration, slot-assist and narration calls (`query_hybrid_planner.go` ~510, `semantic_slot_assist.go` ~122,
  `query.go` ~6346, `open_ended_semantic_planner.go` ~371) each start with a different system prompt, so on a one-slot server
  each evicts the others' cached start.

Estimated size of a family's full field catalogue in the same JSON shape (field id, type, operators, name, one-sentence
description, synonyms; characters divided by 3.6, so about plus or minus 30%): CDR about 1,550 tokens, IPDR 1,500, tower
1,430, ANPR 1,330, subscriber 1,000, transaction 730, access log 480.

## Change

1. **One preamble, identical for every call type and every question about a family:** the family's complete field catalogue in
   a fixed order, serialised the same way each time (sorted keys, no dates, no per-request ids, no timestamps). It goes
   **first** in the user message of every call that concerns that family. Nothing question-specific appears before it.
2. **After it, the call-specific part:** the call's own instructions, then the retrieved subset **as field ids only** (the
   descriptions are already in the preamble), the literals, the date bounds, and the question **last**.
3. The grammar (`response_format` schema) still enumerates only the retrieved field ids, so the model still cannot emit a
   field outside the retrieved subset. The preamble is context, not permission.
4. A keep-alive request that sends only the preamble for the active family (one token out) when the case is opened, so the
   first question does not pay the cold read.

Expected effect, from measured numbers (an estimate until run): a plan call that read about 1,300 tokens cold (about 35 to
45 s) reads about 150 to 300 tokens after the cached preamble (about 4 to 8 s). Across the 2 to 4 calls of a generating
question that is the largest single saving available without new hardware, and it does not change what the model is allowed
to say. It does nothing for writing speed (see "Other levers").

## Risks to measure, not assume

- **Accuracy:** a longer context with more distractor fields. WI-0's 74.3% was measured with retrieval narrowing the
  fields; the point of the grammar is that the model cannot pick an unissued field, but it could still be steered by the
  extra text.
- **First call per family is slower** (the preamble read cold once: 20 to 40 s for CDR). Switching families on a one-slot
  server evicts it.
- **Prefix stability:** any byte that differs early (a reordered field, a changed description, a request id) silently turns
  the cache off. A test must fail if the preamble bytes change without a deliberate catalogue change.
- **Memory:** a larger prompt grows the key-value cache within the same 4,096 context; check the longest prompt still fits
  with room for the answer.
- **The plan cache** (`semantic_plan_cache.go`) is keyed on the payload; the payload changes, so every cached plan is
  invalidated once and must be regenerated by the new layout.

## Thresholds (fixed now, before the arms run)

Same corpus and probes as the 2026-09-29 arms: the full 103, the 38-question pre-flight, 14 everyday, 16 relational, the plate
probe and the 8 new questions, plan cache off in both arms so every plan calls the model, other model unloaded, model warmed
before each arm, runner records per-question wall time, prompt tokens and cache hits.

| Line | Pass condition |
|---|---|
| Answers | the text of every answer is identical between arm A (as deployed) and arm B (L1b); any difference is listed and judged by the owner |
| Accuracy | 0 corpus questions lose CORRECT; 0 new WRONG; pre-flight 38/38; plate 8/8 with no leaks; none of the 8 new questions gets worse |
| Speed, warm | median wall time per generating question in B at least 30% lower than A, with the spread (min, median, max) reported; a single run is not evidence |
| Speed, cold | the first question per family in B no more than 20 s slower than in A |
| Prefix stability | the preamble is byte-identical across 20 different questions of one family (checked by hash in the runner) |
| Memory | no OOM or restart; peak LocalAI memory recorded and within the 7.6 GB Docker limit |
| Rollback | any failed line: restore the previous layout, record why |

## Order of work

1. Add a prompt-layout test first (preamble hash stable across questions; question last). No behaviour change.
2. Implement behind a switch (`FORENSIC_PLAN_PREAMBLE=true`), default off.
3. Run arms A and B with the thresholds above on the machine that has the database; only then decide the default.
4. Then, and only then, re-examine a second slot (L1a) if different call types still evict each other.

## Other levers, for completeness (all need their own measurement)

- **Writing speed is memory-bound.** Dual-channel memory (a second matching DDR4-3200 module, if the laptop has a free
  slot) is the one change that could roughly double it; Q4_0 saves only a few percent there. A shorter plan output (fewer
  generated tokens) helps in the same proportion on any hardware: at 7 tok/s each 100 tokens is about 14 s.
- **Q4_0** (the pre-registered 2026-09-29 test, files downloaded, result not in this checkout): published tests give about
  40% faster prompt reading on AVX2 CPUs. It only matters for the cold reads that L1b removes, so run L1b first and judge
  Q4_0 on what remains.
- **A thread-count test** (8 against 12 or 16) is cheap but touches the model config; only for prompt reading, since decode
  is memory-bound. Needs approval.
