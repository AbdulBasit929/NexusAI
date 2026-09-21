# NexusAI R6 final source and runtime acceptance

Date: 2026-08-11  
Phase: R6 — Ask NexusAI and Specialist Orchestration Experience  
Disposition: complete; source and live-runtime accepted

## Operator activation evidence

The guarded rebuild passed in one LocalAI attempt with 6.33 GiB free RAM.
API, worker and LocalAI build times were 12.6, 3.3 and 81.5 seconds.
Rollback images and named volumes were preserved. The R6.4 gate reported exact
65-template coverage, corpus `2026-08-11.r6.4`, seven governance scenarios and
zero mutating requests. LocalAI readiness and forensic records health returned
HTTP 200.

The accepted R6.4 marker records:

- `forensics.agent-presentation/v1`;
- `forensics.query-template-catalog/v1`;
- `forensics.family-query-answer-corpus/v1`;
- 65 templates, 66 corpus entries and 65 uniquely covered templates;
- nine curated suggestions and seven governed scenarios;
- zero mutation and no schema-drop attempt; and
- preserved rollback images and named volumes.

## R6.5 closure

### API, authorization and security

- The complete forensic sidecar package passes.
- LocalAI forensic proxy, governed case and per-user Agent isolation run cleanly
  in the isolated internal suite.
- Agent request correlation, lifecycle, retry, retained history and typed
  presentation suites pass.
- The full auth package and focused auth/usage route suite pass.
- Agent routes remain protected by the `FeatureAgents` group middleware;
  forensic records and V1 routes remain protected by `FeatureRecords` and
  trusted actor/tenant/case/collection scope binding.
- The final live gate uses eight GET requests and sends no chat, history,
  evidence, report, upload, reprocess or configuration mutation.

An exploratory full routes suite passed 18/19. Its unrelated backend-upgrade
fixture attempted a blocked external OCI lookup and then lacked its temporary
`run.sh`; this is outside the R6 surface. The focused auth/usage route suite
passes and no R6 failure is waived.

### Accessibility and browser acceptance

The complete Agent Chat browser suite passes 17/17. The newly deployed bundle
was independently inspected at desktop and 390×844 mobile widths:

- the governed case and evidence-first workspace render;
- exact-fact, citation and model-role assurance remains visible;
- corrected `longest_call` corpus guidance is present;
- obsolete `duration_extrema`, `ipdr_temporal_activity` and
  `duplicate_transactions` suggestions are absent;
- page-level horizontal overflow is zero; and
- captured browser warning/error logs are empty.

Typed result tables retain a caption, column scopes and keyboard-scrollable
container; processing status is politely announced; raw trace remains
collapsed by default.

### Performance and operational acceptance

`scripts/verify_nexusai_r6_5_live.ps1` produced
`r6.5-live-acceptance.json`. It verifies health, application shell, governed
case, specialist registry, template catalog, query corpus and retained-history
contracts. Eight local GET probes measured p95 latency of 231.99 ms against a
5,000 ms acceptance bound. The deployed production asset is
`/assets/index-4Nowm5K2.js`.

The gate records six required specialist identities, all 65 templates, 66
corpus entries, nine curated suggestions, seven scenarios, zero mutating
requests, no schema-drop attempt and preserved rollback images/volumes.

## Final disposition

R6 exit criteria are satisfied at the bounded product/runtime boundary:
request state is correlated and case-isolated; cancellation, timeout,
reconnect and retry behavior is explicit; retained history is governed;
deterministic execution authority and model fallback are disclosed; sources
and specialist ownership are accessible; and the family operation surface is
complete and versioned.

R6 is complete. No further R6 rebuild or acceptance run is required unless its
source changes. R7 is next in sequence but remains not started until explicitly
authorized as a bounded work item.
