# NexusAI R4-DEPLOY-01 runtime gate progress

Date: 2026-08-07  
Status: complete; corrected deployment and runtime acceptance passed

## Guarded deployment result

The user ran `scripts/build_deploy_forensic_phase6_gate.ps1`. The retained
transcript is
`reports/runtime-activation-20260731/manual-phase6-20260807-111543.transcript.log`.
The gate emitted:

```text
Phase6Activation=PASS APIBuildSeconds=3.7 WorkerBuildSeconds=7.5 LocalAIBuildSeconds=114.5 LocalAIBuildAttempts=1 FreeRAMBeforeBuildGiB=6.12 RollbackImages=preserved VolumesPreserved=true
```

LocalAI, worker, PostgreSQL and NATS were running healthy after activation. The
forensic API was running and its documented `/healthz` route returned 200;
LocalAI `/readyz`, NATS `/healthz`, and LocalAI `/app` also returned 200. The
new LocalAI image is
`sha256:fd4f3bfb5269da963b03d8a87edbe11b7b2194c61f7ce1b4c0911c87d0595d14`.

## Live acceptance result

The deployed application served `index-BCL3QJ6m.js`. Read-only browser
inspection verified the governed `nexusai-forensic-demo` case, its matching
collection, 9,272 queryable rows, all eight exact workspace modules, zero
captured console errors, and zero page-level horizontal overflow at 1440 and
390. The mobile module navigator remained internally contained.

The protected deployed-bundle matrix passed 60/61. The single failure was the
App Shell keyboard skip link: activation reached `#main-route-content` but did
not reliably transfer DOM focus. An isolated deployed rerun reproduced the
failure, so it was not waived as contention.

## Bounded correction

`core/http/react-ui/src/App.jsx` now explicitly focuses and scrolls the existing
`#main-route-content` target when the skip link is activated. Focused ESLint has
zero errors, Vite 8.0.16 builds 667 modules, the isolated production-preview
focus contract passes 1/1, and the complete corrected production-bundle matrix
passes 61/61 with four workers.

## Disposition

The user ran the corrected rollback-preserving gate. The retained transcript is
`reports/runtime-activation-20260731/manual-phase6-20260807-113954.transcript.log`.
It reported PASS with 7.22 GiB free RAM, one LocalAI build attempt, rollback
images preserved and volumes preserved. The corrected LocalAI image is
`sha256:87c306c0fb8bfa9d220e5a7200a9575f392db0a6716788484d41df73e61e9067`
and the deployed UI serves `/assets/index-BY8o0UmF.js`.

LocalAI `/readyz`, records `/healthz`, NATS `/healthz` and `/app` returned 200.
The isolated deployed skip-focus contract passed 1/1 and the complete protected
deployed matrix passed 61/61 with two workers. R4 is therefore complete at its
bounded source, deployment and live-runtime boundary, and R5 may proceed.

No evidence upload/reprocessing, report retention, database migration, model or
configuration change, collection cleanup, staging, commit, push or publication
was performed during this acceptance work.
