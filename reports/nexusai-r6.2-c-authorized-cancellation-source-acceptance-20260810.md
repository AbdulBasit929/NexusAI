# NexusAI R6.2-C authorized cancellation source acceptance

Date: 2026-08-10  
Status: source accepted; deployment pending separate approval

## Outcome

Forensic Agent Chat now exposes an explicit Stop action backed by
`POST /api/agents/{name}/chat/{message_id}/cancel`. The endpoint requires
matching authorized `case_id` and `collection_id`. Cancellation keys bind user,
agent, case and request, preventing cross-scope cancellation.

Local native, distributed worker and distributed deterministic executions
register cancellable contexts. Forensic downstream HTTP calls inherit the chat
context. Cancellation emits a correlated `cancelled` terminal status; the UI
waits for that status, records `Request stopped by analyst.`, and then clears
only the matching request state. Generic Agent Chat remains unchanged.

## Verification

- Focused agent correlation/cancellation Go specs: PASS.
- Agentpool, endpoint and route compile gates: PASS.
- Pinned Swag v1.16.6 regenerated `docs.go`, `swagger.json` and `swagger.yaml`.
- API instructions registry and agent documentation include the cancellation contract.
- ESLint: zero errors; existing warnings only.
- React production build: PASS, 669 modules, 8.19 seconds.
- Agent Chat browser suite: PASS, 10/10.
- Combined Agent Chat and Case Workspace production-preview matrix: PASS,
  27/27 in 26.5 seconds.
- No deployment, retained-data mutation, staging, commit, push or publication.

## Next

`R6.2-D — Consistent whole-request deadlines and timeout state` will define one
bounded deadline contract and a correlated `timed_out` terminal state without
adding retry or reconnect scope.
