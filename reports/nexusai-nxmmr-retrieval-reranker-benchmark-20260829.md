# NX-MMR retrieval baseline and reranker benchmark

## Decision

Keep the current Qwen3-Embedding-0.6B Q8 baseline. Reject the Qwen3-Reranker-
0.6B Q8 challenger for admission on this evidence: it is worse at rank 1,
roughly 2.9x slower end to end, reaches the same 3 GiB cap, and required one
bounded timeout retry. Its perfect fixture no-answer abstention is useful
diagnostic evidence but does not offset the losses.

## Sealed fixture

Both models saw the identical eight-document candidate set and 13 frozen
queries: 11 answerable and two no-answer across English, Roman Urdu, native
Urdu and mixed language. Thresholds were sealed at 0.5 before inference.
Evidence tier is `FIXTURE`; no result is a real-world or product promotion.

| Metric | Embedding baseline | Reranker challenger |
|---|---:|---:|
| Recall@1 / Precision@1 / citation precision@1 | 0.8182 | 0.7273 |
| Recall@3 | 0.9091 | 1.0000 |
| Recall@5 | 1.0000 | 1.0000 |
| MRR | 0.8667 | 0.8636 |
| nDCG@1 | 0.8182 | 0.7273 |
| no-answer abstention | 0.0000 | 1.0000 |
| full wall time | 136.836 s | 397.425 s |
| sampled model-container peak | 3,072 MiB | 3,072 MiB |

## Language/error analysis

- Embedding: English 6/6 and mixed 1/1 at rank 1; native Urdu 1/2 and
  Roman Urdu 1/2. It missed the Roman-Urdu target-number query and native-Urdu
  roaming query. Both no-answer queries scored above the frozen threshold.
- Reranker: English 6/6 and native Urdu 2/2 at rank 1; Roman Urdu 0/2 and mixed
  0/1. It ranked the opposite-duration document for the Roman-Urdu shortest-
  call query and unrelated documents for the other two misses. Both no-answer
  controls abstained.
- The reranker's very high relevance scores include wrong rank-one results;
  they must not be treated as calibrated truth.

## Execution record

The 5-query gates both passed 5/5 at rank one. The first full reranker attempt
timed out at the fixed 120-second per-request bound; after a fresh 4.89 GiB
preflight, the single permitted retry completed with an explicit 240-second
bound. All runs used one model, CPU only, a 3 GiB container cap, isolated copied
backend files, an internal-only Docker network and no retained state.

Private detailed rows are in
`local-acceptance-models/nxmmr/private-benchmarks/retrieval/`.

