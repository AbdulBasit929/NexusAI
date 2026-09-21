# NexusAI R4-ARCH-05 administrative transitions and reports source acceptance

Date: 2026-08-07  
Status: source accepted; live read-only production-preview accepted; not deployed

## Accepted outcome

Generic Collections and Collection Details are explicitly administrative
knowledge surfaces. They preserve collection management, but no longer point
an analyst through the implicit Records default. An Open Case action is shown
only when the shared active-case registry contains an exact selectable mapping
for the current own-user collection. Unmapped and other-user collections never
derive case identity from their names.

Case Workspace now includes a production-grade on-demand Reports module. It
shows the locked authorized case and bound collection independently, accepts an
optional target and evidence-preview choice, presents warnings and generated
Markdown, and exports only from the current browser session. The UI explicitly
states that the output is not retained as a case artifact.

The browser API adapter overwrites any caller-supplied scope with the provider
case and collection identifiers. The V1 compatibility endpoint validates both
`case_id` and `collection_id` against the URL-authoritative case before writing
either value into the proxied body. The current same-ID mapping remains isolated
to that adapter boundary.

## Verification evidence

- Focused ESLint: PASS with zero errors.
- Vite 8.0.16 production build: PASS, 664 modules transformed.
- Focused internal Go governance assertion: PASS, 1/1; verifies absent/matching
  scope binding plus fail-closed case and collection mismatch behavior without
  partial body mutation.
- Playwright specs added for exact non-default administrative transition and
  locked report request/preview/export. CLI execution was infrastructure-
  blocked before test logic because pinned Chromium headless shell 1208 is not
  installed; this is not recorded as an application test failure.
- In-app production preview: PASS. The current local stack exposed three active
  administrative collections and exactly one authorized Open Case action. That
  action navigated to `/app/cases/nexusai-forensic-demo/overview`, never an
  implicit default.
- Live report path: PASS. The non-retained report action returned 13 analytical
  sections with deterministic accuracy language and an export control.
- Responsive acceptance: PASS. Page scroll width equaled client width at both
  820 and 390 pixels; the mobile scope grid collapsed to one column.
- Browser console: zero captured errors.

## Preservation and boundary

No Docker image or container was rebuilt or deployed. No database/schema,
evidence, collection, model, backend, agent, retained configuration or volume
state was changed. The live report was a read-only computed response and was
not retained or downloaded. No staging, commit, push, pull request or external
publication occurred.

## Next bounded slice

`R4-ARCH-06 — Modular workspace acceptance` will reconcile the approved
Overview/Ask/Evidence/Relationships/Timeline/Media/Reports/Admin composition,
preserve deterministic operations and complete protected module-route,
responsive, keyboard, truthful unavailable-state and no-cross-case acceptance.
It must not implement R5 ingestion, R6 orchestration or R8+ media intelligence
ahead of their owning phases.
