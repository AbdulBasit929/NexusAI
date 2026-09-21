# NexusAI R6.2-F Source Acceptance and R6.3-A1 Foundation

Date: 2026-08-10  
Runtime/data mutation: none  
Deployment: not performed

## R6.2-F accepted boundary

- Added explicit `POST /api/agents/{name}/chat/{message_id}/retry` for failed,
  timed-out, or cancelled forensic requests only.
- Requests are bound to user, agent, case/collection, original request, exact
  message hash, and a caller-supplied 16-128 character idempotency key.
- Distributed mode atomically reserves retries in the existing PostgreSQL
  `agent_observables` table with no schema migration. Standalone mode uses an
  equivalent zero-value-safe in-memory registry.
- Concurrent duplicates return the winning retry `message_id` with
  `idempotent_replay: true`; changed payloads under the same key conflict.
- Only the idempotency-key digest is retained; the caller key is not stored.
- `retry_of` lineage is returned and retained. No automatic retry, replay,
  model change, evidence mutation, or deployment was introduced.

## R6.3-A1 started boundary

- Browser case histories now carry versioned `savedAt`, `schemaVersion`, and
  `persistenceAuthority: browser_local` metadata.
- Forensic Agent Chat exposes a saved-analysis filter and bookmark action
  labeled **Saved analyses · This browser**.
- The target `forensics.case-analysis-history/v1` server contract and bounded
  A2-A5 sequence are documented in the R6.3 design contract.
- Enterprise/server retention is not implemented or claimed.

## Verification

- Focused retry concurrency, payload-conflict, and eligibility specs: PASS.
- Affected agentpool, endpoint, and route packages compile: PASS.
- Focused ESLint: 0 errors (18 existing/style warnings).
- Vite production build: PASS, 669 modules, 11.78 seconds.
- Agent Chat Chromium acceptance: PASS, 14/14, including explicit scoped
  retry payload/lineage and truthful browser-local saved-analysis presentation.
- Pinned Swag v1.16.6 regeneration and retry-route parse: PASS.
- `git diff --check`: PASS, line-ending warnings only.
- The broad repository run was not accepted as a pass: PostgreSQL testcontainer
  specs cannot start rootless Docker on this Windows host, and one unrelated
  backend-route fixture hit a restricted temporary directory.

## Next action

Proceed with R6.3-A2: approve and source-design the retained history schema,
tenant/case authorization indexes, retention/legal-hold policy, migration and
rollback plan. Do not activate schema or import browser history without a
separate governed approval.
