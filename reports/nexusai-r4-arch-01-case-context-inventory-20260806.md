# NexusAI R4-ARCH-01 case-context inventory acceptance

Date: 2026-08-06  
Disposition: source architecture accepted; runtime unchanged

## Outcome

The existing case-context sources, routes and collection coupling were traced
across the React router, Case Workspace, Records Intelligence, Agent Chat,
generic Collections pages, LocalAI forensic-case proxy and forensic sidecar.
The resulting single active-case contract is recorded in
`docs/design/nexusai-r4-active-case-contract.md`.

## Material findings

- Case Workspace already uses `/app/cases/:caseId/:section` as its primary
  identity and the V1 query adapter rejects URL/body identity mismatch.
- Records redirect, shell display context and Agent Chat each resolve case state
  independently instead of consuming a single validated source.
- Bound Records Intelligence synchronizes to the route case but retains editable
  collection state and historical `records-demo` fallbacks in the same component.
  This can expose an alternate collection target from inside a case-aware view.
- Agent Chat correctly scopes browser conversations by requested case and sends
  explicit case/collection IDs, but its UI still permits fallback through the
  default case and persisted agent configuration.
- Generic Collections is an administrative collection surface, not a case
  context. Its transition to Records Intelligence can resolve a default case
  unrelated to the collection being viewed.
- V1 case=collection equality is an intentional compatibility mapping today and
  must remain isolated in adapters so future case/collection separation does not
  require another UI-wide rewrite.
- The sidecar report contract remains collection-shaped and needs an explicit
  case-scoped adapter and mismatch test before Reports becomes a case module.

## Evidence inspected

- `core/http/react-ui/src/router.jsx`
- `core/http/react-ui/src/pages/RecordsRedirect.jsx`
- `core/http/react-ui/src/pages/CaseWorkspace.jsx`
- `core/http/react-ui/src/pages/RecordsIntelligence.jsx`
- `core/http/react-ui/src/pages/AgentChat.jsx`
- `core/http/react-ui/src/hooks/useAgentChat.js`
- `core/http/react-ui/src/pages/Collections.jsx`
- `core/http/react-ui/src/utils/api.js`
- `core/http/endpoints/localai/forensic_cases.go`
- `core/http/endpoints/localai/agents.go`
- `core/services/agents/`
- `api/forensic_records/cases.go`
- `api/forensic_records/report.go`

## Acceptance boundary

No application behavior, database state, evidence, collections, models, images,
containers or volumes were changed. The next bounded action is R4-ARCH-02: add a
read-only active-case provider/route adapter and migrate only shell context,
Records redirect and Case Workspace identity with focused tests.
