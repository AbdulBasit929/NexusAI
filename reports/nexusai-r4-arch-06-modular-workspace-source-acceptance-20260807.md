# NexusAI R4-ARCH-06 modular workspace source acceptance

Date: 2026-08-07  
Status: R4 source accepted; deployment/runtime acceptance pending approval

## Product outcome

Case Workspace is now a coherent enterprise investigation desk rather than a
row of loosely related tabs. A reusable command-center chrome presents the
active investigation, readiness, classification, selectable authorized case,
distinct case and collection identifiers, and a concise authorization state.
Desktop uses a sticky descriptive module rail; tablet and mobile use a compact
contained horizontal navigator without creating page-level overflow.

The approved modules are complete at their R4 boundaries:

- Overview: readiness, inventory, quality notes, coverage and manifest.
- Ask: existing deterministic Records Intelligence under an evidence-first
  assurance boundary; no premature R6 orchestration claim.
- Evidence: case-bound registration and provenance catalog.
- Relationships: source-backed networks, contacts and cross-family workflows.
- Timeline: recorded-event chronology, temporal activity, movement and ANPR
  workflows with an explicit no-interpolation rule.
- Media: truthful registered image/audio/video inventory; registration is not
  presented as OCR, transcription, detection or validated analysis.
- Reports: existing R4-ARCH-05 on-demand, non-retained deterministic reporting.
- Admin: processing history, governance, resource accounting and protected
  advanced controls with destructive actions disabled.

Legacy `/analyze`, `/jobs` and `/settings` URLs redirect explicitly to `/ask`
or `/admin` and preserve query context. The Records default and forensic Agent
Chat workspace action now use `/ask` directly.

## Verification

- Focused ESLint: PASS with zero errors.
- Vite 8.0.16 production build: PASS, 667 modules transformed; workspace chrome
  CSS is emitted as its own production asset.
- Complete protected system-Chrome matrix: PASS, 61/61 in 46 seconds using four
  workers. Coverage includes login, navigation, App Shell, R3.1 visual system,
  Case Workspace, deterministic Records Intelligence, forensic/generic Agent
  Chat and Knowledge administration.
- Case Workspace suite: 11/11, including 1440/820/390 overflow, keyboard
  navigation, exact eight-module composition, truthful Timeline/Media/Admin,
  legacy redirects, report export, inaccessible-case fail-closed behavior,
  default resolution and stale-case clearing.
- Interactive production preview against current local APIs: full eight-module
  desk rendered for `nexusai-forensic-demo`; desktop rail was sticky; mobile
  navigator scrolled internally; page scroll width equaled client width at
  1440 and 390; captured console errors were zero.

An initial eight-worker protected run recorded 60/61 because one desktop render
exceeded its five-second assertion timeout under contention. That exact test
passed immediately in isolation, and the final bounded four-worker run passed
all 61 contracts cleanly.

## Preservation and remaining gate

No API, schema, database, evidence, collection, model, backend, agent, retained
configuration, container, image or volume state was changed. No deployment,
staging, commit, push, pull request or publication occurred.

R4 is source accepted, not fully deployed/runtime accepted. The next and sole
active item is `R4-DEPLOY-01 — Guarded combined refresh and runtime acceptance`.
It requires explicit approval to run
`scripts/build_deploy_forensic_phase6_gate.ps1`, preserve rollback state, and
verify deployed hashes, all eight modules, one-case scope, deterministic Ask/
Reports behavior, responsive layouts, keyboard access and a clean console.
R5 must not begin until that gate is accepted.
