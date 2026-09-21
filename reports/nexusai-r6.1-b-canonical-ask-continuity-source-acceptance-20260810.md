# NexusAI R6.1-B canonical Ask continuity source acceptance

Date: 2026-08-10  
Status: R6.1 source accepted; deployment pending separate approval

## Outcome

Specialist analysis now has an explicit `Back to Ask NexusAI` action. The return
route is derived only from the provider-validated active case and carries the
current analyst input, latest sent user question, or original handoff prompt in
that order. It does not accept an arbitrary return URL.

The question remains available after send, and changing the governed case
updates the return destination to that authorized case. Existing case-keyed
conversation isolation and late prior-case event rejection remain intact.
Generic Agent Chat receives no forensic return control and retains its original
unscoped request payload.

## Verification

- Focused ESLint: PASS with zero errors; 14 pre-existing warnings in
  `AgentChat.jsx` remain unchanged in class.
- React production build: PASS, 669 modules.
- Production-preview browser acceptance: PASS, 25/25 in 26.1 seconds.
- Coverage includes prompt prefill, send persistence, governed case switch,
  canonical Ask return, unavailable specialist behavior, registry failure,
  stale prior-case rejection, generic compatibility and all 17 Case Workspace
  contracts.
- `git diff --check`: required before final handoff.

## Governance and next work

No API, model, agent configuration, database, evidence, collection, container,
image, volume, deployment, staging, commit, push or publication change occurred.
R6.1 is source accepted. `R6.2-A` is an implementation-free audit of the real
HTTP/SSE execution lifecycle and server support for correlation, cancellation,
timeout, retry and reconnect semantics. It must select one bounded correction
before R6.2-B changes behavior.
