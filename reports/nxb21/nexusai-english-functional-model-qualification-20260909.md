# NexusAI current-4B English functional qualification adjudication — 2026-09-09

The `nxb21-english-functional-current-4b-fresh-r1-20260909` corpus is consumed and must never be rerun. The run is `INCOMPLETE_RESOURCE_RUNTIME_FAILURE`; it is not evidence of a semantic-model failure. Current 4B semantic quality remains `INSUFFICIENT_EVIDENCE_FROM_THIS_RUN`.

The frozen corpus contained 149 English cases: 101 semantic cases, including 86 supported cases and all 66 workspace operations; 32 router cases; and 16 grounded-synthesis cases. The resource gate admitted the run at 6.72 GiB available RAM. The first semantic call took 60,056 ms, returned `timeout_or_unavailable` with no decision, and the immediate loaded-state measurement was 2.093 GiB against the frozen 4 GiB floor. Evaluation stopped after that one call and the current 4B model was safely unloaded.

No direct-selection, safe-outcome, router, or synthesis rate is reportable from one timed-out semantic call. The correct classification is insufficient evidence, not zero-percent quality and not a model semantic failure. The oracle and consumed questions must not be edited or used for prompt tuning.

Runtime integrity passed: retained tuple remained `68|68|81|22511|870|68`, Activity remained 353, active jobs remained zero, all container identities/images/restart counts remained unchanged, and the accepted analyst index SHA-256 remained `a0f1e4288c1b9ee46b4fa3c3c94870fef46b12d7c1718c36eee2e905d14ae7de`.

Immutable evidence:

- Run directory: `local-acceptance-models/nxb21-english-functional-qualification/run-20260909T092931616Z`
- Dispatch lock SHA-256: `d91999fcd4159b0402d7a65ad3abf688a9328cb8625937873f0dc2065745557f`
- Operator receipt SHA-256: `93fe4c7358da8bd381367e05cd9e29efea9eb9aa655a008fce582cf99eeae10d`
- Partial result SHA-256: `af9757ff7ebf13baa054fea0fff95a50ee95f2b1220d2f03293408257da8c488`
- Resource observations SHA-256: `bc9fc2ea251f71d18ca3e4d534017302cdd734d9d87ccc0cde547b826fb78c37`
- Freeze receipt SHA-256: `16ce46fa946c22bf3a576473a47143c193770671113df85dcfd2292e2843bd25`

The next gate is a standalone, non-benchmark resource and transport proof for the unchanged current 4B artifact/profile. A new English corpus may be dispatched only after that proof passes stable preload, stable loaded-state, generic transport-soak, and post-soak RAM gates.
