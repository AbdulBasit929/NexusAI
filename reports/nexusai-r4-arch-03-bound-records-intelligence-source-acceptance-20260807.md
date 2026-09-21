# NexusAI R4-ARCH-03 bound Records Intelligence source acceptance

Date: 2026-08-07  
Verdict: PASS at source and local production-browser boundary  
Deployment: not performed

## Outcome

Embedded Records Intelligence now has one authoritative case and collection:
the identity validated by the shared `ActiveCaseProvider`. It no longer exposes
or consults an alternate collection selector while operating inside Case
Workspace. Query, evidence, ingestion, batch, status, capability, deletion and
export behavior is consistently bound to that scope.

The UI presents a visible **Active case boundary enforced** strip and a read-
only Active Case Collection field. The boundary remains compact on mobile while
the accepted specialist shortcuts and deterministic result surfaces remain
available to analysts.

## Safety and correctness controls

- Embedded mode fails closed when provider case identity is unavailable or
  conflicts with a requested case.
- Case-aware forensic queries send explicit `case_id` and `collection_id` to
  the governed case endpoint.
- Structured queries force the provider collection even if a helper or custom
  request body attempts to supply another collection.
- Evidence uploads target only the provider collection and carry the explicit
  case ID. Legacy records ingestion carries the same collection identity.
- Embedded mode neither lists nor creates alternate agent collections.
- Returned batches are defensively filtered to the active collection, and
  deletion is blocked for an out-of-scope batch.
- Case evidence uses the case-scoped v1 endpoint. Detail, status and capability
  requests use the canonical active collection.
- Case-derived results, rows, evidence, batches, previews and ingestion state
  are cleared when scope changes. Versioned request guards discard late
  responses from the previous case.
- Result exports include sanitized case/scope identity in their filenames.
- A completed request cannot overwrite a question the analyst edited while the
  request was in flight.
- Unbound administrative compatibility remains separate and unchanged.

## Verification

- Focused ESLint: PASS with zero errors. The existing JSX/no-unused warning
  behavior remains warnings-only; no rule or baseline was weakened.
- Vite 8.0.16 production build: PASS, 664 transformed modules.
- Production assets include `index-EQ1bHPe7.js`,
  `RecordsIntelligence-CWIm_HaS.js`, `CaseWorkspace-BJUyV7Jj.js` and the
  accepted `index-pJX-gsGx.css` stylesheet.
- Focused Records Intelligence browser suite: PASS, 11/11.
- Protected R3/R3.1 plus R4 browser matrix: PASS, 46/46 in installed system
  Chrome, using the production preview and no browser download.
- Changed-file whitespace validation: PASS.

The browser contracts cover canonical/raw-field workflows, planner behavior,
typed CDR/IPDR/ANPR/subscriber visual results, exact provider-bound structured
queries, multipart evidence upload, legacy ingestion, absence of collection
list/create calls and rejection of a late answer after an active-case switch.
The carried protected matrix covers shell semantics, keyboard access,
truthful/fail-closed case loading, desktop/tablet/mobile overflow, navigation,
authentication and visual-system behavior.

## Preservation

No API/backend schema, database, case/evidence data, collection, agent, model,
image, container, volume or retained configuration was changed. No Docker
rebuild/redeployment, dependency or browser download, staging, commit, push or
publication occurred. The accepted live R3.1 plus R4-PRE runtime remains
unchanged; R4-ARCH-02 and R4-ARCH-03 are source-only until a separately approved
guarded deployment.

## Next bounded slice

`R4-ARCH-04 — Bind Agent Chat to the active case` makes case-aware Agent Chat
consume provider identity, send explicit case/collection scope on every request
and isolate or clear conversation-derived state on case changes. Generic/admin
chat compatibility must remain explicit. The slice must not expand into the
future R6 streaming-lifecycle program.
