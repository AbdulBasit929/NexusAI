# NexusAI frontend source-truth reconciliation — 2026-09-17

Status: `FRONTEND_SOURCE_VERIFIED`, `UI_ACTIVATION=NOT_STARTED_RAM_GATE`,
`BACKEND_QUERY_SEMANTIC_SCOPE=UNCHANGED`.

## Scope

This slice is limited to the active React UI in `core/http/react-ui/`, its
browser upload transport, presentation tests, and the API-container UI-asset
overlay needed to serve those assets. It does not include query planning,
semantic selection, forensic operations, model roles, database schema,
retained evidence, ingestion workers, or reprocessing.

## Gap and acceptance reconciliation

| Stable ID | Severity | Source evidence and root cause | Decision and files | Independent acceptance | Final status |
| --- | --- | --- | --- | --- | --- |
| NXB21-UI-P1-INTAKE-20260917 | P1 | Add Data exposed analyst-selected timezone/language/type controls, relied on browser extension policy, and could report completion after cancellation. | Keep the normal flow to drop/choose plus a truthful queue; render server capability formats; preserve unknown time/language; make browser checks advisory; connect `AbortSignal` to XHR; expose retry for failed/cancelled items. `AnalystAddData.jsx`, `AnalystData.jsx`, `AnalystWorkspace.jsx`, `analystIntakePolicy.js`, `uploadPolicy.js`, `api.js`. | Focused unit suite; Analyst Playwright; production build. | fixed_source_verified_activation_pending |
| NXB21-UI-P1-EVIDENCE-TOOLBAR-20260917 | P1 | The evidence toolbar could be clipped by an ancestor using horizontal overflow clipping, and request races could replace current state. | Use one responsive sticky toolbar with coherent filters/chips/count/reset; remove the clipping ancestor; sequence requests; keep loading/error/empty/filtered-empty distinct. `EvidenceWorkspace.jsx`, `EvidenceWorkspace.css`, `CaseWorkspaceChrome.css`. | Case Workspace Playwright at 390/820/1024/1440/1600 in light/dark, including sticky geometry and no page overflow. | fixed_source_verified_activation_pending |
| NXB21-UI-P2-MODALITY-20260917 | P2 | Modality icons and labels were resolved in several places with inconsistent filename-first fallbacks; audio lacked the required waveform treatment. | Centralize authoritative-field-first modality resolution with MIME and filename fallbacks; retain the specialized ANPR treatment. `evidenceModality.js` and all analyst/case/evidence presentation callers. | Modality unit tests plus browser assertions for audio list/detail/history surfaces. | fixed_source_verified_activation_pending |
| NXB21-UI-P2-HISTORY-REUSE-20260917 | P2 | Reusing a retained result was visually ambiguous with reopening and could imply execution. | Keep stored-result reopen separate from explicit `Use again`/`Use question again`; prefill the exact retained prompt without sending it. `AnalystHistory.jsx` and presentation helpers. | Analyst Playwright covers retained reopen and non-executing prompt reuse. | fixed_source_verified_activation_pending |
| NXB21-UI-P2-ACTIVATION-RAM-20260917 | P2 | The fresh UI-only bundle passed read-only preflight, but the live attempt observed twelve RAM samples from 4.57 to 4.836 GiB, below the unchanged 6 GiB floor. | Fail before first mutation. Do not unload the embedding model, reclaim host cache, weaken the gate, or alter backend/query/semantic state. | Runner reported `UI_ACTIVATION=NOT_STARTED`, `SERVICE_RECREATION_PERFORMED=false`, and `RETAINED_DATA_MUTATED=false`. | open_resource_gate |

## Verification

- Focused JavaScript unit tests: 49 passed.
- Analyst Portal Playwright: 31 passed.
- Case Workspace Playwright: 20 passed.
- Production Vite build: passed, 689 modules transformed. Vite reported only
  the inherited inline-dynamic-import and large-chunk advisories.
- Focused ESLint: zero errors and zero new warnings; 55 warnings remain in the
  already-modified large frontend files (56 before this slice).
- Browser coverage includes 390, 820, 1024, 1440, and 1600 pixels, light/dark,
  toolbar geometry, horizontal overflow, and audio modality presentation.

## Activation candidate

- Runner: `scripts/activate_nexusai_source_truth_ui_20260917.ps1`.
- Bundle: `reports/nxb21/nxb21d-source-truth-ui-activation-20260917/`.
- Manifest SHA-256:
  `0ae4dae2af4f45e2a05cf7f49eb2febeff37533987a2c02655e85a9656f240a6`.
- UI source tree: 391 files,
  `c82530503b60b7190a2ac6627c83faacccca822598eaa1ca4cf46d4c7eafd4df`.
- UI distribution tree: 213 files,
  `c848483ea2907d69bff75af48b2d03765eadf9bbe0f9878cf2160132b28ca085`.
- Exact preserved backend base image:
  `sha256:327754b7fe958c9d3f33f4ad2188f3c29cac7d9576f775b8eaca4c60b3b10006`.
- Preflight baseline: retained tuple `69|69|82|0|22512|876|69`, Activity
  count 355, active jobs 0, loaded model `qwen3-embedding-0.6b`, Q8 unloaded.

The bundle includes no backend source. The live attempt did not tag/build an
image, recreate a service, migrate data, change a model/profile, or mutate
retained evidence. The historical premium-UI bundle remains unchanged and is
not reused.

## Remaining limitations

- The corrected assets are not live until three consecutive RAM samples meet
  the 6 GiB gate and the same runner completes live hash and preservation
  checks.
- History search/filtering remains truthful to the currently loaded paginated
  result set; expanding it to server-side search would require an authorized
  backend contract change and is outside this frontend-only slice.
- This slice does not certify any forensic operation, query planner, semantic
  model, family adapter, or cross-family result.

## Exact next action

Free enough physical memory for three consecutive samples at or above 6 GiB
while keeping Docker Desktop running, then execute:

`powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\activate_nexusai_source_truth_ui_20260917.ps1`

Do not pass the model-unload or cache-reclaim switches for this frontend-only
activation.
