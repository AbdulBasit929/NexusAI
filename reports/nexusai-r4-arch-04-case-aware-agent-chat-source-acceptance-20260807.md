# NexusAI R4-ARCH-04 case-aware Agent Chat source acceptance

Date: 2026-08-07  
Verdict: PASS at source and local production-browser boundary  
Deployment: not performed

## Outcome

Forensic Agent Chat now consumes the shared `ActiveCaseProvider` rather than
independently resolving cases or falling back to the collection stored in agent
configuration. Every forensic send carries the provider-validated `case_id` and
`collection_id`, including when those identifiers differ.

Conversation storage and visible state are isolated by case. A case transition
immediately withholds the prior conversation until the new case-scoped store is
loaded, resets transient stream/canvas state and rejects late HTTP, SSE status,
stream, error and final-message activity from the previous case.

## Safety and product controls

- An explicit `?case=` is validated by the provider. Inaccessible cases render
  a closed error state, disable forensic input and send no agent work.
- Direct forensic-chat entry resolves the provider's current governed case or
  governed default and writes that identity into the URL.
- Persisted `forensic_collection_id` remains visible only as backend
  compatibility configuration; it cannot select scope for case-aware UI.
- The active case selector uses provider-authorized options. The ribbon shows
  the distinct active collection and links back to the matching case workspace.
- Capability discovery is collection-bound and ignores stale responses after a
  case switch.
- Pending agent requests retain both conversation and case-scope correlation.
  Events with mismatched explicit metadata or originating scope are ignored.
- The conversation hook never renders or mutates an old store while a different
  agent/case scope is loading and persists each case under its own storage key.
- Generic Agent Chat remains unscoped and preserves its original `{message}`
  compatibility payload without loading the forensic case registry.
- This slice does not redesign cancellation, retry, reconnect or the complete
  streaming state machine reserved for R6.

## Verification

- Focused ESLint: PASS with zero errors and 18 existing-rule warnings; no rule,
  coverage baseline or tolerance was weakened.
- Vite 8.0.16 production build: PASS, 664 transformed modules.
- Production assets include `index-BdEUDzHK.js`, `AgentChat-DQrXWsFw.js`,
  `api-njEEUar0.js` and the accepted `index-pJX-gsGx.css` stylesheet.
- Focused Agents/Agent Chat browser suite: PASS, 7/7.
- Protected R3/R3.1 plus R4 browser matrix: PASS, 53/53 in installed system
  Chrome against the production preview, with no browser download.
- Changed-file whitespace validation: PASS.

Browser coverage includes distinct case/collection request bodies, governed
default resolution, rejection of persisted-agent fallback, inaccessible-case
fail-closed behavior with zero chat requests, per-case conversation restoration,
late prior-case SSE suppression, generic chat compatibility and 390-pixel
horizontal-overflow acceptance. The carried matrix retains shell, keyboard,
navigation, case-workspace, Records Intelligence and visual-system acceptance.

## Preservation

No server endpoint, schema, database, evidence, collection, agent configuration,
model, image, container, volume or retained setting changed. No Docker rebuild,
deployment, download, staging, commit, push or publication occurred. The live
R3.1 plus R4-PRE runtime remains unchanged; R4-ARCH-02 through R4-ARCH-04 remain
source-only until a separately approved guarded deployment.

## Next bounded slice

`R4-ARCH-05 — Administrative transitions and reports` separates generic
collection administration from explicit authorized case transitions and closes
the case-scoped report adapter/test boundary before Reports is presented as a
case module. No retained report generation or data mutation is implied.
