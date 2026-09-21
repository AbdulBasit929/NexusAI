# NexusAI R6.2-D governed timeout-state source acceptance

Date: 2026-08-10  
Status: source accepted; deployment pending separate approval

## Outcome

Local native, distributed worker and distributed deterministic Agent Chat now
share one 210-second whole-request deadline. Distributed dispatch carries the
absolute deadline so queue time is included; legacy events receive the same
bounded fallback when dequeued. Context termination is classified centrally:
an elapsed deadline emits the request-correlated terminal state `timed_out`,
while an analyst Stop remains `cancelled`.

Deterministic forensic execution suppresses intermediate generic error status
for both terminal context outcomes, leaving the owning execution path to emit
one authoritative lifecycle result. Downstream forensic HTTP requests inherit
the governed request context.

The analyst UI clears only the matching request, displays `Request timed out.
No automatic retry was sent.`, and returns to a send-ready state. This slice
does not retry, replay or mutate retained state.

## Verification

- Focused request-lifecycle Go specs: PASS in 16.068 seconds, including dispatch
  deadline propagation, wrapped
  deadline, analyst cancellation, ordinary-error classification and terminal
  callback ownership.
- Agentpool compile gate: PASS (clean cached rerun after a transient Windows
  temporary-binary cleanup denial).
- Focused ESLint: zero errors; 14 existing warnings in `AgentChat.jsx`.
- React production build: PASS, 669 modules, 1.52 seconds.
- In-app production-preview inspection: PASS; governed demo case selected,
  analyst query enabled, assurance/model indicators visible and zero console
  errors.
- Agent Chat browser suite: PASS, 11/11 in 13.8 seconds.
- Combined Agent Chat and Case Workspace production-preview matrix: PASS,
  28/28 in 26.0 seconds.
- No deployment, retained-data/configuration mutation, schema change, model
  change, staging, commit, push or publication.

## Next

`R6.2-E — Reconnect reconciliation and status lookup` will define how a client
reconciles an in-flight request after SSE reconnect without adding retry,
schema, deployment or retained-state scope.
