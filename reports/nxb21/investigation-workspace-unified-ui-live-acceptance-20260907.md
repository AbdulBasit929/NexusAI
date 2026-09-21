# Unified Investigation Workspace — Live Acceptance

Date: 2026-09-07

## Result

`UNIFIED_ANALYST_UX=LIVE_VERIFIED`

`INVESTIGATION_WORKSPACE_UI=ACTIVE`

The sealed UI-only overlay is live at `http://localhost:8080/analyst` on API
image `sha256:5e056478ae0f1823cfe0fc453f1c26a60c2eb86cc173958c6537fc064b0420d9`.
The container is running and healthy. The served index SHA-256 is
`d4ba168793cd696488ce5ab76e3c68bb5b3017657435e5ed8b51d6a683bc1c60`,
which is the sealed build index recorded by activation.

## Live browser acceptance

- Canonical `/analyst` loaded the unified top bar, evidence summary, persistent
  Ask composer, answer-first retained results, citations and limitations.
- Evidence and History opened as secondary drawers without restoring the former
  four-destination navigation.
- Add data opened the real governed intake dialog and exposed its actual file
  input and drag/drop affordance. No file was uploaded.
- Dialog background sections were marked `inert` and `aria-hidden`.
- Browser console warnings/errors: **0**.
- Horizontal overflow at 390, 820, 1024 and 1440 pixels: **none**.
- No query was submitted and no retained data was changed during acceptance.

## Runtime isolation

- Only `nexusai-api-1` was recreated.
- Protected forensic API, worker, NATS and PostgreSQL container and image IDs
  still match the pre-activation receipt.
- Retained tuple before/after: `66|66|79|0|22509|850|63`.
- Database migration: **false**.
- Model change: **false**.
- D-unqualified source included: **false**.
- D activation: **BLOCKED**.

## Receipt

Path:
`reports/nxb21/investigation-workspace-unified-ui-activation-20260907/activation-receipt.json`

SHA-256:
`d386e78a651f4b658835970d71c034963d3badf3c3814eac9b987b71a42199c3`

