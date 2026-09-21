# NexusAI R4 active-case contract

Status: accepted source architecture for `R4-ARCH-01`  
Date: 2026-08-06  
Scope: case identity and navigation only; no runtime, database, evidence or model changes

## Decision

NexusAI has exactly one active case for every case-aware UI operation. The
canonical identity is an authorized URL case, represented to components by a
shared active-case context. Component-local collection values, browser storage,
agent configuration and historical demo defaults are never authoritative for a
case-aware request.

The current V1 implementation may map `case_id` and `collection_id` to the same
value. That equality is a compatibility detail at the API adapter boundary, not
an application-wide domain rule. UI code consumes a case object containing at
least `caseId`, `collectionId`, display metadata and access state.

## Current-source inventory

| Surface | Current identity source | Existing fallback/coupling | R4 disposition |
| --- | --- | --- | --- |
| Case routes | `/app/cases/:caseId/:section` in `router.jsx` | None for the route itself | Remains canonical for Case Workspace. |
| `CaseWorkspace.jsx` | `useParams().caseId` | Independently fetches case list, info, status, capabilities, manifest and evidence; status/capability calls pass the case as a collection | Migrate to shared context while preserving path authority and case-scoped APIs. |
| `RecordsRedirect.jsx` | Independently fetched default or first accessible case | Re-resolves the default on each mount | Consume the provider's single resolved default, then navigate to its case path. |
| `shellContext.js` / header | Parses the pathname independently | Display-only parsing can diverge from loaded/authorized state | Consume shared case metadata and access state. |
| Embedded `RecordsIntelligence.jsx` | `caseId` prop, then route param | Retains editable collection state and `records-demo` fallbacks; advanced ingestion can target another collection while displayed inside a case | In bound mode, lock every query/upload/list/report action to the active case and hide or disable alternate collection selection. Keep unbound/admin compatibility explicitly separate. |
| `AgentChat.jsx` | `?case=` query parameter | Falls back to API default and then persisted `forensic_collection_id`; resolves cases independently | Consume shared context and send an explicit case on every case-aware request. Persisted agent configuration is compatibility-only outside case-aware UI. |
| `useAgentChat.js` | Requested case scopes browser conversation keys | No server-side case key in the SSE channel | Preserve case-scoped storage; require message/request correlation and explicit case metadata for concurrency/audit work. |
| Generic Collections pages | Collection route/name for administration; exact provider registry mapping for optional case transition | No implicit Records/default link remains | R4-ARCH-05 accepted: administration stays collection-scoped; Open Case appears only for an exact selectable own-user mapping. |
| V1 case API | URL `/api/v1/forensics/cases/:caseId/...` | Case and collection are intentionally identical today | Keep equality and mismatch rejection inside the adapter. Return a typed case contract to clients. |
| Agent endpoint | Explicit request scope | Rejects mismatched IDs, but can fall back to stored agent configuration when scope is absent | Case-aware clients must always supply the active case. Missing case fails closed for case-aware operations. |
| Report path | Provider-bound V1 case URL plus distinct adapter case/collection IDs | Sidecar remains collection-scoped behind the compatibility boundary | R4-ARCH-05 accepted: both body IDs are locked/validated, Reports is on-demand and non-retained, and mismatches fail closed. |

## Resolution and navigation rules

The active case resolves in this strict order:

1. an explicit case path parameter;
2. an explicit, authorized `?case=` parameter on a compatibility route;
3. the current authorized case held by the provider during in-app navigation;
4. the default case returned by the case-list API, resolved once when entering a
   case-aware surface.

An inaccessible, deleted or malformed explicit case fails closed. It must not
silently switch to a default or previously active case. Browser storage, agent
configuration, component state and names such as `records-demo` are excluded
from the resolution chain.

Changing case is a navigation event. It updates the canonical URL and causes all
case-derived requests, results, selections and pending work to be cancelled or
invalidated. A late response for the former case cannot update the current case
view.

## Provider contract

The bounded implementation target is an `ActiveCaseProvider` with a read-only
value shaped conceptually as:

```text
activeCase: { caseId, collectionId, displayName, status, access }
state: loading | ready | inaccessible | error
setActiveCase(caseId, destination)
refreshActiveCase()
```

The provider validates identity through the governed case-list/detail API. It
does not cache evidence, analytic results or agent conversations. Those modules
remain responsible for their data, keyed by `activeCase.caseId` and invalidated
on a case transition.

## API invariants

- A case-aware request uses a case-scoped V1 endpoint where one exists.
- If a compatibility request needs both IDs, the adapter supplies both from the
  active case; components cannot override either value.
- URL/body identity mismatches return a client error and perform no work.
- Upload, query, report and agent operations enforce authorization for the same
  case before work begins.
- An omitted case is an error on a case-aware surface. No demo, local-storage or
  persisted-agent fallback is allowed.
- Logs and asynchronous events retain case and request/message correlation
  without exposing evidence contents.

## Migration slices

1. **R4-ARCH-02 — Provider and route adapter.** Introduce the read-only provider
   and migrate App Shell context, Records redirect and Case Workspace identity.
   Do not change backend behavior or module layout.
2. **R4-ARCH-03 — Bound Records Intelligence.** Remove alternate collection
   authority from embedded mode and reset/cancel case-derived state on switches.
3. **R4-ARCH-04 — Case-aware Agent Chat.** Use provider identity, explicit scope
   on every send and case-visible response correlation; remove UI reliance on
   persisted agent scope.
4. **R4-ARCH-05 — Administrative transitions and reports.** Separate generic
   collection administration, add explicit case transitions and close the
   case-scoped report contract.
5. **R4-ARCH-06 — Modular workspace acceptance.** Complete the approved
   Overview/Ask/Evidence/Relationships/Timeline/Media/Reports/Admin composition
   through bounded modules while preserving deterministic operations.

Each slice requires focused source tests before any guarded rebuild/deployment.

## R4 source completion

R4-ARCH-02 through R4-ARCH-06 are source accepted as of 2026-08-07. The shared
provider is authoritative across the shell, deterministic Ask/Records,
Evidence, Relationships, Timeline, Media inventory, Reports, Admin and forensic
Agent Chat. The eight-module workspace and compatibility redirects pass the
complete protected browser matrix. Deployment/runtime acceptance remains a
separate guarded gate; source acceptance does not claim that the running
container already serves these modules.

## Required acceptance tests

- Path/query/provider/default precedence is deterministic.
- An inaccessible explicit case renders a closed error state without fallback.
- Switching case cancels or ignores stale list, detail, evidence and analytic
  responses from the previous case.
- Shell, selector and module requests expose the same case identity.
- Embedded Records Intelligence cannot upload, query or report against a
  different collection.
- Agent Chat sends the active case on every request and keeps conversation state
  isolated by case.
- URL/body case or collection mismatch is rejected without side effects.
- Collection administration cannot accidentally open the default case when a
  different explicit case transition was intended.
- Existing authorized-case, deterministic-query, R3 shell and R3.1 visual-system
  contracts remain green.

## Safety and non-goals

`R4-ARCH-01` is a source/design checkpoint only. It does not rename or migrate
collections, change tenant access, ingest or reprocess evidence, rebuild images,
alter agent configuration, or deploy services. The accepted live R3.1 runtime
and all named volumes remain untouched.
