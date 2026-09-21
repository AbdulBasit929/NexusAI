# NexusAI R7.9 hardening and R7.10 runtime acceptance

Date: 2026-08-12  
Disposition: R7.9 source accepted; R7.10 live accepted; R7.8 remains approval-gated

## Corrections completed

1. The model-assisted CDR smoke prompt now supplies the exact synthetic target
   required by `frequent_contacts`. The prior result was a correct fail-closed
   clarification, not a model or query failure. The corrected focused model
   path passes.
2. The backward-compatible Ask query response now publishes the same bounded
   typed visualization descriptors as the governed case response. Before this
   bridge, the deployed answer retained correct rows but dropped the map and
   timeline metadata.
3. Tower visualization source fields now explicitly include both CDR and
   reference evidence file/row/hash identifiers.

## R7.9 acceptance

- Query limits clamp to 100 and offsets clamp to 100,000.
- Tower joins remain parameterized by tenant and collection and use stable
  event-time/source-file/source-row ordering with bounded `LIMIT/OFFSET`.
- Every tower join row carries CDR and reference evidence/source row hashes,
  coordinate datum, uncertainty and match status.
- Agent presentation bounds tables and nested visualization rows while retaining
  scalar coordinates, status and locators.
- The forensic API package and focused agent-presentation contract pass after
  the new hardening assertions.

## R7.10 live acceptance

- The user-run full rollback-preserving activation passed: API 14.9 seconds,
  worker 8.1 seconds, LocalAI 85.8 seconds, one LocalAI attempt, 7.55 GiB free
  RAM, rollback images preserved and volumes preserved.
- All six forensic profiles version 1.2 were applied to
  `nexusai-forensic-demo` / tenant `default` using the existing approved Qwen
  explanation model.
- The deterministic Agent Chat matrix passed all 65 operations with 4,149.7 ms
  P95 latency.
- Five model-family paths passed in the original run; the corrected CDR-focused
  regression then passed, completing coverage of all six specialist families.
- A bounded API-only activation rebuilt and recreated only
  `forensic-records-api` in 33 seconds. Health passed, profiles were unchanged,
  named volumes were preserved and a dedicated rollback image was retained.
- The deployed legacy Ask endpoint now returns two tower CDR visualizations:
  `map` and `timeline`, each with two bounded rows, `route_inference=false` and
  explicit reference locators.
- Live UI acceptance shows the supplied-coordinate map, source-bound timeline
  and synchronized selected-evidence drawer. The selected observation exposes
  timestamp, coordinates, WGS84 datum, 75 m uncertainty and exact source
  file/row/hash. No route, RF coverage or physical presence is inferred.
- At 390 x 844 the deployed page has 390 px client/scroll width, no horizontal
  overflow, both visualizations and the evidence drawer remain present, and no
  browser console errors were captured.
- LocalAI `/readyz`, forensic API `/healthz` and NATS `/healthz` return 200; six
  specialist profiles are registered and the approved Qwen model remains
  installed.

## Sole remaining R7 gate

R7.8 cannot be completed from synthetic evidence. It requires an explicitly
named protected dataset and authorization defining tenant/case, read-only audit
versus retained ingest, retention class, access roles and permitted outputs.
Until then no protected file is opened, copied, uploaded, normalized into
retained tables or exposed in reports. R8 must not start early.
