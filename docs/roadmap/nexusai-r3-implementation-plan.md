# NexusAI R3 implementation plan

Status: source and browser accepted, 2026-08-06  
Authority: `NEXUSAI_MASTER_DIRECTIVE.md` and the accepted R2 design contracts

R3 advances through bounded source slices. It reuses the current React router,
provider tree, responsive drawer, operations bar, route guards and public
branding/auth/features APIs. It introduces no visual-only backend API.

## R3-UI-01 — Identity, loading and navigation foundation

Source implemented; browser acceptance pending: reusable NexusAI mark/lockup/signature; existing
white-label asset override; exact primary/context tagline placement; browser
metadata; branded boot, login-status and lazy-route loading; Analyst/System
Administrator labels; analyst/admin terminology; initial `--nx-*` semantic
tokens; login/navigation/responsive/focus test contracts.

Acceptance remaining: execute the current-source browser matrix after explicit
build/preview approval. Deployment is a separate later gate.

## R3-UI-02 — Unified shell context

Source implemented; browser acceptance pending. The restrained desktop/tablet
header exposes route, authenticated identity and enforced role. Mobile has a
compact route/case header. Case routes show only the decoded URL-bound case ID;
they do not invent a case name, classification or readiness state. The existing
operations bar and route layout remain, and no API was added. A keyboard skip
link targets the route content and drawer focus-return behavior is preserved.

## R3-UI-03 — Shell state language and route white-label pass

Source implemented; browser acceptance pending. The shared state component now supports accessible empty, error,
forbidden, partial and unavailable variants while retaining legacy callers and
their child content. Authentication loading is branded rather than blank; 404
and governed-case redirect states use the shared contract. Knowledge now uses
durable loading/error/empty states and analyst-facing terminology; Talk exposes
truthful pipeline loading/error/unavailable states; Home uses the shared
unavailable state for non-admin workspaces without models. Visible administration
copy is NexusAI-contextualized while technical compatibility identifiers and
legal attribution remain intact.

## R3-UI-04 — Acceptance and handoff

Accepted. The production Vite bundle completed with 662 transformed modules.
Twenty-seven focused login/loading/error/shell/Knowledge/navigation contracts
passed against the hashed production assets in installed Chrome. Interactive
production QA passed the 390/820/1024/1440 responsive matrix, light/dark theme,
metadata/favicon, primary route presentation and browser-console checks. The
local preview was stopped after acceptance. Deployment remains a separate
approval gate; R4 may now begin.
