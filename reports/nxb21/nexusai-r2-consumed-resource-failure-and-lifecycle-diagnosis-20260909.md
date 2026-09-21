# NexusAI r2 consumed resource failure and lifecycle diagnosis — 2026-09-09

The r2 English corpus is `CONSUMED_DO_NOT_RERUN`. Its qualification is
`CONSUMED_INCOMPLETE_RESOURCE_FAILURE`; current-4B semantic quality remains
`INSUFFICIENT_EVIDENCE`. The model passed stable load, three generic requests,
transport, and post-soak RAM. This is not a semantic-model failure.

## Exact consumed-run result

Run `run-20260909T110254961Z` created the qualification dispatch lock at
11:06:42.611Z and entered `r2-sem-001`. No qualification response completed,
so no intermediate result file was written: total completed calls 0, semantic
0, router 0, synthesis 0, and no last completed case. `r2-sem-001` is the last
dispatched case, not a completed case.

The last post-soak reading was 4.221 GiB. The first external qualification
sample 51 ms after lock creation was 4.160 GiB. Readings then declined with
small fluctuations through 4.124, 4.122, 4.073, 4.061, 4.040, 4.051, 4.032,
4.027, and finally 3.966 GiB after 46.584 seconds. The runner stopped its owned
evaluator; LocalAI logged the active POST as HTTP 500 / proxy context canceled.
This was a resource-guard cancellation, not a recorded timeout.

Evaluator working set rose from 10.012 to 22.137 MiB and stayed approximately
22.125 MiB, ruling it out as the main 194–255 MiB host-availability decline.
R2 captured only one API-memory snapshot (1.335 GiB), one vmmemWSL snapshot
(5,698.8 MiB), and invalid zero backend rows due to a telemetry parser defect.
Consequently API, backend RSS, WSL, KV/context cache, and runtime-leak trends
cannot be truthfully claimed from this run.

Generic request latencies were 8,493, 1,930, and 1,902 ms (P50 1,930 ms,
nearest-rank P95 8,493 ms, maximum 8,493 ms). Qualification P50/P95/max are not
defined because zero calls completed; the first request was canceled after
about 46.6 seconds.

## Request footprint and classification

A non-inference diagnostic reproduced the exact r2 serialization. Every
semantic case carried 66 candidates. Prompt bytes were 11,111 / 11,150 / 11,197
(min/P50/max); whole request bodies were 14,798 / 14,837 / 14,884 bytes. Rough
`bytes/4` token estimates were 2,778 / 2,788 / 2,800 and are explicitly
approximate.

The evidence supports `A_HOST_RAM_DRIFT` as observed and
`I_LARGE_PROMPT_LARGE_CANDIDATE_PAYLOAD` as the likely immediate trigger. The
gate admitted only 0.177 GiB minimum post-soak headroom before sending the first
much larger request. `K_OTHER` also applies because qualification lacked
per-call API/backend/WSL telemetry. KV-cache accumulation, context-cache
accumulation, backend/API growth, and runtime leakage remain unproven: zero
qualification calls completed. Request concurrency and result buffering did not
occur, and evaluator growth was too small to explain the breach.

This means completed-case batching is not yet an evidence-backed fix. The
failure happened inside the first request, before any batch boundary could
help. A future qualification lifecycle must nevertheless append immutable
`case_started` and `case_completed` records, sample all memory surfaces before
and after each call, use predetermined batch boundaries, and never replay a
completed case. Whether a model-only unload/reload is required remains pending
resource-only proof.

## Disposable lifecycle proof

The frozen, non-semantic soak uses 20 tiny calls in batches of 1, 2, 4, 8, and
5, with model-only unload/reload between batches, followed by three generic
payload-shaped calls after another reset. It records host RAM, API container
memory, corrected backend RSS, vmmemWSL working set, latency, and per-call
drift. It contains no r2/r3 questions, corpus, oracle, operation IDs, or semantic
decision work and is explicitly not model-quality evidence.

- Runner SHA-256: `18f27c767982e4e078ad3280b69aa44f7af830a7d7642378e2a70e71df3c95a2`
- Freeze SHA-256: `5e206207dc49737056481eb3f4c510d417e2540271340e075ba1dcc62a66c0c2`
- Static lifecycle/process-safety/hash checks: PASS
- Read-only `-ValidateOnly`: PASS
- Live soak: `PREPARED_FROZEN_NOT_EXECUTED`

No r3 corpus may be created until this soak passes and its immutable receipt is
adjudicated. If repeated generic or payload-shaped calls cannot hold the 4 GiB
floor despite deterministic model-only resets, current-4B sustained local fit
is resource-insufficient and smaller-model selection requires separate owner
approval.
