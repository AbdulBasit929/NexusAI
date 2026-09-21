# NX-MMR query-intelligence benchmark status

## Verdict

Query intelligence remains source-validated by the existing 214-entry
deterministic query-variant ledger; it is not newly real-world-certified by
NX-MMR. The live runtime still exposes the older 68-entry corpus and 67
operations versus 214/79 in source.

The NX-MMR retrieval fixture adds 13 independent retrieval questions across
English, Roman Urdu, native Urdu, mixed language and no-answer behavior. Those
results certify only model ranking on that sealed fixture. They do not certify
planner routing, parameter binding, executor truth, citation correctness,
answer synthesis, API/agent equivalence, UI presentation or Activity reopen.

Current operation ledger remains:

- 62 `REGISTERED`;
- 12 `SOURCE_VALIDATED`;
- one `FIXTURE_CERTIFIED`;
- zero `REAL_WORLD_CERTIFIED`;
- four `PRODUCT_CERTIFIED`.

Ordinary visible suggestions remain limited to exact product-certified prompt-
to-operation mappings. No self-generated expected answer was used as an oracle,
and no unrestricted SQL path was added.

