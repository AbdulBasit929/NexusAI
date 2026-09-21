# NexusAI R6.2-B correlated lifecycle source acceptance

Date: 2026-08-10  
Status: source accepted; deployment pending separate approval

## Outcome

Agent lifecycle events now preserve one request identity from HTTP 202
submission through processing, live content/reasoning/tool activity, terminal
messages and errors.

- Distributed `EventBridge` status and stream publishers require the originating
  `message_id` and include it in both the NATS event envelope and SSE metadata.
- NATS worker callbacks and the distributed deterministic forensic path propagate
  the dispatch ID through every status and stream event.
- Local legacy Agent Chat now includes request identity in user, status, final
  and error events; native local events retain their existing correlated path.
- The browser normalizes `-user`, `-agent` and `-error` terminal suffixes, treats
  the HTTP 202 response as authoritative, and mutates live stream/spinner/error
  state only when both request identity and governed case scope match.
- An older request can still finish in its originating conversation, but it
  cannot overwrite a newer conversation's stream or clear its spinner.
- Uncorrelated connection-level SSE errors remain visible but cannot terminate
  an unrelated request.

## Verification

- New focused Go request-correlation specs: PASS.
- Agentpool compile-only gate: PASS (`go test ./core/services/agentpool -run '^$'`).
- A broad agents-suite attempt compiled and ran 69 non-database specs; its 13
  database-backed specs could not start because rootless Docker test containers
  are unsupported on this Windows host. This is an environment limitation, not
  a changed-code assertion failure.
- Focused ESLint: zero errors; 14 existing warnings in `AgentChat.jsx`.
- React production build: PASS, 669 modules, 6.17 seconds.
- Production-preview Agent Chat suite: PASS, 9/9.
- Combined Agent Chat and Case Workspace browser acceptance: PASS, 26/26 in
  28.8 seconds.
- New browser coverage runs two conversations in one governed case, interleaves
  their stream and final events, verifies only the current request owns live
  state, and verifies each final answer reaches its original conversation.
- `git diff --check`: required before final handoff.

## Governance and next work

No route, schema, database, evidence, model, agent configuration, collection,
container, image, volume or runtime state changed. No deployment, staging,
commit, push or publication occurred.

`R6.2-C — Authorized cancellation endpoint and Stop action` is next. It must
reuse the now-correlated request ID, enforce agent/user/case ownership, cover
local native, distributed worker and deterministic execution modes, publish a
correlated terminal cancellation event, and remain explicit rather than
silently retrying work.
