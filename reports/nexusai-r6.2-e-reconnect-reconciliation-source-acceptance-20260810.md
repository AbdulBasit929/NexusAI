# NexusAI R6.2-E reconnect reconciliation source acceptance

Date: 2026-08-10  
Status: source accepted; deployment pending separate approval

## Outcome

Forensic Agent Chat now publishes and retains a bounded lifecycle snapshot for
the exact user, agent, case and request. The read-only endpoint
`GET /api/agents/{name}/chat/{message_id}/status` requires matching authorized
`case_id` and `collection_id` under the current v1 case contract and returns no
conversation content.

Local execution records lifecycle state directly. Distributed status events
carry case scope, and each frontend tracks them independently of connected SSE
clients. The in-memory registry expires entries after 30 minutes and caps them
at 4,096 entries; no schema or retained-state dependency was introduced.

When SSE reconnects, the browser performs one lookup for only its current
correlated request. Processing remains active; completed, failed, cancelled or
timed-out state releases the matching UI. A completion missed while disconnected
is labeled `Request completed offline. Replay unavailable.` Unknown/expired state
is also released with an explicit no-retry message. No work is replayed or retried.

## Verification

- Focused Go correlation/lifecycle specs: PASS in 26.124 seconds.
- Agentpool, LocalAI endpoint, route and application compile gates: PASS.
- Pinned Swag v1.16.6 generation: PASS; generated route/schema parse check PASS.
- Focused ESLint: zero errors; existing warnings only.
- React production build: PASS, 669 modules, 9.36 seconds.
- Agent Chat browser suite: PASS, 12/12 in 11.7 seconds.
- Combined Agent Chat and Case Workspace matrix: PASS, 29/29 with the deterministic
  one-worker local acceptance profile in 33.3 seconds.
- Two-worker diagnostic runs each passed 28/29 but timed out in different unchanged
  five-second UI assertions; this concurrency instability is recorded and is not
  represented as a passing two-worker gate.
- In-app rebuilt preview: governed demo case selected, query enabled, assurance
  visible and zero console errors.
- No deployment, retained-data/configuration mutation, schema/model change,
  staging, commit, push or publication.

## Next

`R6.2-F — Idempotent explicit retry semantics` will add analyst-initiated retry
lineage and duplicate-submit protection. Automatic retry remains prohibited.
