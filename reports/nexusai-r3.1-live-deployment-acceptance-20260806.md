# NexusAI R3.1 live deployment acceptance

Date: 2026-08-06  
Verdict: PASS  
Deployment method: existing guarded Phase 6 combined gate, executed by the user

## Guarded deployment evidence

The supplied transcript ended with:

`Phase6Activation=PASS APIBuildSeconds=4.6 WorkerBuildSeconds=4.8 LocalAIBuildSeconds=73.5 LocalAIBuildAttempts=1 FreeRAMBeforeBuildGiB=6.3 RollbackImages=preserved VolumesPreserved=true`

The React production stage transformed 663 modules and embedded the accepted
R3.1 assets. The historical gate script was not modified.

## Verified live state

- `nexusai-api-1`: running and healthy on image
  `sha256:961070c6432ee2a1dc16bc53b638781215ff6d42171b58b778fe0b43a0106303`.
- `nexusai-forensic-records-api-1`: running on image
  `sha256:0dca5b52fd2bf9265ceb50ba9074e7a1ac32084d0fdf14b187ff577c82123a42`.
- `nexusai-forensic-records-worker-1`: running and healthy on image
  `sha256:54c46f5784f6b90fb36f26c9d3b81f979cb290ded5791613bbb9f640af37dc11`.
- PostgreSQL and NATS are running and healthy; the spool initializer exited 0.
- LocalAI `/readyz`, forensic API `/healthz`, and LocalAI `/app` returned HTTP 200.
- Named model, backend, data, image, configuration, PostgreSQL, NATS and spool
  volumes remain present.

## Live browser acceptance

- Deployed Home served `index-CYZD-OEG.js` and `index-pJX-gsGx.css`, with no
  Vite client and no console entries.
- NexusAI title, 248 px rail, 64 px context header and dark theme rendered at
  1440 px without horizontal overflow.
- The governed route `/app/cases/nexusai-forensic-demo/analyze` resolved the
  URL-backed case and active collection `nexusai-forensic-demo`, displayed
  9,272 accepted rows and produced no error state or console entry.
- Settings rendered five code-native NexusAI identity elements and requested no
  legacy LocalAI logo image.
- Home passed at 390 px with the compact mobile header and no page-level
  horizontal overflow.

## Warnings and preservation

Docker reported existing orphan-container and externally-created-volume
warnings. No `--remove-orphans`, volume deletion, cleanup, migration, upload,
reprocess, model change, staging, commit, push or publication was performed.
Rollback containers/images and retained volumes were preserved.

