# NexusAI Phase 6.3 Live Acceptance — 2026-08-03

## Decision

The guarded Phase 6 activation and the deterministic CDR, IPDR, and ANPR
runtime are **accepted live**. All four runtime health surfaces pass, all four
specialist profiles are applied to the governed case, the complete 23-operation
deterministic matrix passes, and the final Records UI corrections are deployed
and browser-accepted at mobile, tablet, breakpoint, and desktop widths.

The later sections retain chronological source-only and intermediate-image
findings for auditability. The final state is recorded under "Final live UI
closure".

## Guarded activation evidence

The operator's PowerShell transcript ended with:

```text
Phase6Activation=PASS APIBuildSeconds=15.1 WorkerBuildSeconds=5.7 LocalAIBuildSeconds=408.6 LocalAIBuildAttempts=1 FreeRAMBeforeBuildGiB=6.44 RollbackImages=preserved VolumesPreserved=true
```

- LocalAI manifest-list digest: `c98fb6fe74ab196ee22fbd4b56f6bfc99719fc9f8014074da56c9f7be26a30ea`
- Activation phase: `6.1+6.2+6.3`
- Deployment marker UTC: `2026-08-03T06:52:54.7081628Z`
- LocalAI build succeeded on attempt 1.
- Rollback images and all named volumes were preserved.
- Compose orphan/external-volume messages were non-fatal topology warnings.
  Do not respond to them with `--remove-orphans` or volume deletion without a
  separate, reviewed topology/retention decision.

## Live runtime acceptance

| Surface | Result |
|---|---:|
| LocalAI `/readyz` on port 8080 | PASS |
| Forensic Records API `/healthz` on port 8091 | PASS |
| Worker metrics on port 9109 | PASS |
| NATS `/healthz` on port 8222 | PASS |
| Public forensic query templates | 53 |
| Enriched guided family templates | 23 |
| Selectable governed cases | 1 |
| Governed case | `records-demo-verified` |
| Collections hidden from normal selection | 25 |

Direct access to the sidecar template endpoint rejected an unauthenticated
request as designed. The authenticated LocalAI public proxy returned the
catalog successfully.

## Specialist-profile activation

`bootstrap_forensic_specialists.ps1` was run through Windows PowerShell with
execution-policy bypass for that process only. Version `1.2.0` of all four
profiles was applied to tenant `default`, collection `records-demo-verified`,
with knowledge-base and forensic modes enabled:

- `Forensic_Records_Analyst`
- `Communications_CDR_Analyst`
- `Network_IPDR_Capture_Analyst`
- `Vehicle_ANPR_Geospatial_Analyst`

All profiles use the approved synthesis role
`qwen_qwen3-4b-instruct-2507`; deterministic forensic tools remain the
authoritative calculation layer. Live readback confirmed the distinct profile
prompts and bindings.

## Deterministic operation matrix

Exact authorized entity discovery returned usable case identifiers, including
CDR MSISDN `923001110001`, IPDR subscriber `923001110002`, IP endpoint
`10.20.2.12`, and ANPR plate `ABC-123`. With an inclusive test window spanning
2020 through 2030 and a result limit of five, **23/23 operations passed**, all
through `records_sql`, with zero unsupported operations and zero failures.

### CDR — 9/9

`frequent_contacts`, `call_type_breakdown`, `temporal_activity`,
`duration_extremes`, `entity_timeline`, `geospatial_movement`,
`device_identity_changes`, `service_usage`, and `tower_activity`.

### IPDR — 7/7

`endpoint_activity`, `domain_activity`, `protocol_breakdown`,
`session_volume`, `subscriber_sessions`, `concurrent_sessions`, and
`entity_timeline`. `concurrent_sessions` correctly returned a governed
`no_results` outcome for this fixture; this is an accepted deterministic result,
not an execution failure.

### ANPR — 7/7

`plate_sightings`, `camera_sequence`, `camera_activity`, `co_travel`,
`route_timing`, `plate_variants`, and `entity_timeline`.

## Live Records UI acceptance

The Records workspace loaded the governed case with 9,250 accepted rows, zero
duplicates, and eight knowledge-base assets. Specialist navigation was scoped
correctly. Exact-identifier discovery produced executable target chips without
turning masked coverage hints into selectable targets.

The analyst result presentation passed for entity activity and CDR call-type
breakdown: readable answer, KPIs/visualization, cited findings, exact record
preview, deterministic provenance, and one concise limitations disclosure.
The CDR example returned eight exact rows, one source reference, and a reported
22 ms deterministic execution. Browser console inspection found no errors or
warnings.

## Browser-found defects and source corrections

Live browser testing materially changed the closure by exposing three issues
that API tests alone could not reveal:

1. A selected CDR phone could be inserted into an ANPR example. The UI now
   records discovered entity type/family metadata, filters target chips by the
   chosen template, refuses known incompatible targets, and uses an explicit
   compatible-target placeholder when necessary.
2. Native `datetime-local` values could be visible in the DOM but absent from
   React state at dispatch. The query path now reads the actual input refs,
   synchronizes state, and sends those exact bounds to the API.
3. The Records workspace overflowed horizontally at 390 px and 1024 px. The
   flex parents now use `min-width: 0`, allowing the workspace to shrink inside
   the responsive shell.

The follow-up source changes are in:

- `core/http/react-ui/src/pages/RecordsIntelligence.jsx`
- `core/http/react-ui/src/App.css`
- `core/http/react-ui/e2e/records-intelligence.spec.js`

Targeted ESLint reports zero errors and only the five existing unused-symbol
warnings. The production React build passes at 658 modules, and bounded diff
checks pass.

## State and next gate

- No collection, evidence, upload, database row, Docker volume, or rollback
  image was deleted.
- Backend, profiles, public templates, entity discovery, and the 23-operation
  matrix are accepted live.
- The three UI corrections are source-verified but not deployed in the image
  identified above.
- The next bounded action is one guarded LocalAI image refresh followed by only
  these UI rechecks: type-compatible template targets, start/end date retention
  and request payload, 390/1024/desktop horizontal overflow, console errors,
  and one deterministic result render. The full 23-operation matrix does not
  need to be repeated unless backend source changes before that refresh.

## 2026-08-03 UI-refresh retry disposition

The first UI-refresh retry stopped before any image build because only 5.9 GiB
was free after the application containers stopped. The subsequent Compose
`up` messages were the automatic rollback restoring the prior healthy
deployment, not preflight startup. Read-only verification after rollback
returned HTTP 200 from LocalAI, the forensic API, worker metrics, and NATS.

The guarded script now also stops PostgreSQL and NATS during the compilation
window and samples free RAM for up to 45 seconds before deciding the 6 GiB
gate. Their named volumes remain attached and both the normal deployment and
rollback paths restart them. The mandatory threshold was not lowered.
PowerShell AST and diff checks pass. No image build, data mutation, or volume
deletion occurred during the failed retry.

## 2026-08-03 post-refresh live UI acceptance

The corrected retry passed after the bounded RAM wait moved from 5.89 to 6.00
GiB. The activation marker records API 3.1 seconds, worker 3.9 seconds, LocalAI
410.3 seconds, one LocalAI attempt, preserved rollback images/volumes, and
LocalAI image
`sha256:9e0e9c47edfead2824d14aa5cd955a112753821362a577d22e6ddec555d5841c`.
All four health surfaces returned HTTP 200 after deployment.

Focused live browser acceptance produced these results:

- **PASS — compatible targets:** selecting ANPR Camera Sequence left the target
  empty and offered only exact authorized plates (`ABC-123`, `XYZ-789`,
  `KHI-777`, `LEA-447`, `ICT-404`, and `LHR-2026`). No CDR phone was
  materialized or offered.
- **PASS — date dispatch:** the visible interval
  `2026-07-19T00:00`–`2026-07-20T00:00` was retained through dispatch. The UI
  reported the corresponding UTC interval and returned six exact `ABC-123`
  sightings, all inside that scope, with six source references.
- **OPEN — responsive root:** the earlier parent-flex correction reduced the
  390 px document width from 1,234 px to 930 px, but browser measurement still
  found overflow. The remaining cause is `.case-workspace`: horizontal auto
  margins on a column-flex child allow max-content sizing. At 1024 px it also
  extended from x=200 to x=1178. Source now sets `width: 100%` on that root so
  it is bounded by the already-shrinking parent.

The final one-line responsive source correction passes targeted ESLint with
zero errors (12 existing unused-symbol warnings), production React build at 658
modules, and diff validation. The existing E2E covers page-level overflow at
1440, 820, and 390 px, but its Windows web-server process could not start in
this environment (`The system cannot find the path specified`). The new root
correction is therefore source-verified and requires one last image refresh and
live 390/820/1024 measurement before UI closure. Backend and deterministic
operation acceptance remain unchanged and do not need repetition.

## Final live UI closure

The final guarded refresh ended with:

```text
Phase6Activation=PASS APIBuildSeconds=3.1 WorkerBuildSeconds=3.4 LocalAIBuildSeconds=505 LocalAIBuildAttempts=1 FreeRAMBeforeBuildGiB=6.31 RollbackImages=preserved VolumesPreserved=true
```

The activation marker records deployment at
`2026-08-03T08:45:30.0496109Z` with LocalAI image
`sha256:a283931706c2e428fa1b813ab29f81e3b2274bb4e290a4d846aafdd6bf39d6c8`.
LocalAI, forensic API, worker, and NATS each returned HTTP 200 after deployment.

Live browser measurements on the deployed Records workspace:

| Viewport | Document client width | Document scroll width | Overflow |
|---|---:|---:|---:|
| 390 x 844 mobile | 380 | 380 | 0 px |
| 820 x 900 tablet | 810 | 810 | 0 px |
| 1024 x 900 breakpoint | 1,014 | 1,014 | 0 px |
| 1440 x 1000 desktop | 1,430 | 1,430 | 0 px |

The browser console contained zero warnings and zero errors. Together with the
previously accepted compatible-target and date-bound query tests, this closes
the three UI defects against the live image. Phase 6.3 is fully live-accepted;
no further rebuild or repeat of the backend/profile/23-operation matrix is
required for this phase.
