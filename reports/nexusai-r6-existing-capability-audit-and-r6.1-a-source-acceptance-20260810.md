# NexusAI R6 existing-capability audit and R6.1-A source acceptance

Date: 2026-08-10  
Status: R6 active; R6.1-A source accepted; deployment pending separate approval

## Reused baseline

- Case Workspace Ask embeds natural-language deterministic Records Intelligence.
- The live catalog publishes 65 deterministic templates across 20 registered
  families, nine currently queryable in the governed case.
- Six forensic specialists are registered and operational with 40 agent actions.
- Agent Chat already sends explicit case and collection IDs, stores conversations
  by case, correlates pending messages, rejects late prior-case events, streams
  reasoning/content/tool activity and preserves generic chat compatibility.
- Exact facts remain SQL/tool authoritative. Existing enterprise answers provide
  direct findings, bounded interpretation, tables/visualizations, limitations,
  provenance, next actions and technical trace.
- Two approved model roles remain installed; this slice needs no new model,
  backend, API, schema or retained configuration.

## Completeness gaps

The initial audit found that Ask displayed a specialist link when its evidence
family existed even if the agent was not registered or running. Other remaining
R6 gaps are the split Ask/Agent Chat route experience, incomplete server-backed
cancellation/retry/reconnect lifecycle, fragmented rather than versioned
real-world family query corpora, and non-uniform family-aware agent result
presentation.

## R6.1-A implementation

Ask now reads the existing agent registry and status map before exposing each
specialist continuation. A family-relevant operational agent receives a
case-bound link carrying the current natural-language question. An absent or
offline agent is rendered as non-interactive `unavailable` status instead of a
dead control. Registry failure produces an actionable warning while leaving the
authoritative deterministic analysis path available. Refresh revalidates case
truth, capabilities, evidence and agents together.

## Verification

- Focused ESLint: zero errors (six pre-existing warnings in the large Records
  Intelligence module).
- React production build: PASS, 669 modules, 10.55 seconds.
- Case Workspace production-preview browser suite: PASS, 17/17 in 22.3 seconds.
- New browser coverage verifies prompt/case propagation for operational CDR and
  general analyst handoff, verifies that an offline CDR specialist has no
  clickable link, and keeps deterministic Analyze enabled when registry
  discovery fails.
- No API, agent configuration, model, database, evidence, collection, container,
  image, volume, deployment, staging, commit, push or publication change.

## Next bounded work

`R6.1-B` makes the Case Workspace Ask route the canonical analysis entry and
improves continuity into and back from a chosen specialist without weakening
case authority or duplicating the agent framework. R6.2 then owns correlated
execution state, real server cancellation, timeout, retry and reconnect behavior.
