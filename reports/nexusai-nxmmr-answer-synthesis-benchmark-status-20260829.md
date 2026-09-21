# NX-MMR answer-synthesis benchmark status

## Verdict

No new model-specific answer-synthesis promotion is justified. Qwen3 4B
remains the installed synthesis baseline; NX-MMR did not download or compare a
new synthesis model. Existing source contracts continue to require cited Fact
Packets/Observation Packets, authority separation, explicit limitations and
truthful zero/no-match/unavailable states.

The retrieval/reranker fixture cannot certify generated answers because it
tests ranking only. A synthesis model may summarize accepted packets but may
not label its own evaluation data, invent missing evidence, turn retrieval
scores into facts, or hide a missing processor behind a zero-result answer.

State: `SOURCE_VALIDATED_EXISTING_BASELINE`; representative independent
real-world synthesis and the live API/Ask/citation/source/Activity journey
remain required before any new `REAL_WORLD_CERTIFIED` or `PRODUCT_CERTIFIED`
decision.

