# Team-Lead Brief — Unified Investigation Workspace

Date: 2026-09-07

The ordinary-user product is now intentionally one workflow:

**Add evidence → Ask → Answer → Verify evidence**

`/analyst` is the canonical workspace. Separate Home, Data, Ask and Activity
navigation no longer drives the analyst experience. The old URLs remain as
compatibility entry points and open the matching unified state.

## What is complete

- One quiet top bar: workspace, current investigation, evidence count,
  **History**, **Add data**, and account preferences.
- One empty state: **Add evidence to begin**, Add data, and drag/drop.
- One evidence summary with a right-side evidence drawer and current source
  detail.
- Ask is the primary persistent interaction. Suggestions come only from ready
  evidence families and current backend capability metadata.
- Answers remain answer-first: findings, evidence and material limitations;
  technical provenance is behind **How this was determined**.
- Citations retain the backend's supported row/page/time/frame semantics and
  open the relevant evidence context.
- History remains available as a secondary investigation journal.
- English, Urdu-script, Roman Urdu and mixed-direction input render correctly.

Classification, preservation, processing, OCR, ANPR, transcription, query
planning, authorization, operation certification and citation generation all
remain behind the interface. React does not plan queries, run SQL, create
analytical values, or promote an unqualified model path.

## Validation and activation state

Source validation is complete: 45 analyst unit tests passed (one explicit
pre-existing fixture skip), 25 Playwright scenarios passed, focused lint passed,
the production build completed across 686 modules, and the live-backed local
preview passed responsive and console checks at 390, 820, 1024 and 1440 pixels.

The sealed UI-only overlay is active on the existing backend base. Only the API
container was recreated; the protected forensic API, worker, NATS and
PostgreSQL identities and the retained seven-field data tuple were preserved.
The live API is healthy, `/analyst` serves the exact sealed index, backend
acceptance passes, and real-browser checks confirm the unified workspace at all
four required breakpoints with no console warnings or errors. The activation
receipt is `LIVE_VERIFIED` and has SHA-256
`d386e78a651f4b658835970d71c034963d3badf3c3814eac9b987b71a42199c3`.

## Current limitations

- Processing-dependent document, OCR, audio and video features become usable
  only when their authoritative artifacts are ready.
- Cross-source and media operations remain bounded by the current certified
  capability contracts.
- The Q4 residual model role is retired and insufficient. Multilingual residual
  interpretation is not presented as certified.

## What comes next

No UI activation work remains. Keep the receipt and rollback image with the
deployment bundle. D remains separate: replacement-model acquisition still
requires explicit approval before any new benchmark or qualification work.
