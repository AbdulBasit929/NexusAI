# NX-MMR download recovery and terminal logs — 2026-08-31

## Outcome

Source changes and non-building validation complete. **57 checks PASS**:
11 focused streaming/retry tests and 46 operator bundle checks. No image was
built, no packages were installed/downloaded, and no live services were changed.

## Diagnosis

The operator's immutable receipt `private-activation/20260831T100208816Z`
records a socket read timeout from files.pythonhosted.org while downloading
PaddlePaddle 3.3.1, at 125.9/194.8 MB. The corrected safetensors 0.6.2 wheel
downloaded successfully. State is BUILD_STARTED with recreation_started=false;
no rollback is required for this failed attempt.

## Bounded changes

- Worker package installation uses a 120-second pip socket timeout, five
  connection retries, and at most three complete pip attempts with five-second
  pauses. All installation failures are subject to this finite attempt limit;
  this does not promise that permanent errors can recover.
- A BuildKit pip package cache reuses completed downloads across attempts/builds.
  No live data/model volume is mounted or changed. Runtime packages, model pins,
  benchmark source and the activation manifest are unchanged.
- The existing 3,600-second build deadline remains in force. A client timeout
  does not prove the Docker backend build has stopped: inspect before retrying.
- Only the build call opts into terminal streaming. Both stdout and stderr
  are read concurrently, with each line flushed to build.log before display.
  The activation transcript also captures displayed output. Within-stream order
  is preserved; exact ordering between simultaneous stdout/stderr is not promised.
- Nonzero exit and timeout retain partial logs and fail closed. Raw inspection,
  Compose configuration and credential-bearing environment output stay private.

## Validation

Focused receipt: `local-acceptance-models/nxmmr/private-activation/20260831T101224407Z`.
Eleven tests verified terminal/log delivery before child exit, separate stdout
return values, stderr and final lines without newline, nonzero exit, bounded
timeout with partial logging, default private-output isolation, 4,000 lines
without pipe deadlock/loss, and rejection of streaming without a log path.
The exact Dockerfile shell was exercised with fake pip in cached Linux containers
with read-only roots, no mounts and networking disabled: immediate success,
recovery on attempt two, and failure after exactly three attempts preserving exit
code 19. No real pip installation or Docker build ran.

Bundle receipt: `local-acceptance-models/nxmmr/private-activation/20260831T101327182Z`.
All 46 parser/safety/resource/model/Compose/rollback checks passed. Read-only
checks confirmed unchanged live identities, images, health and restart counts.
Available RAM was 5.024845 GiB, correctly blocking admission below 6 GiB;
disk was 1649.193764 GiB, active jobs zero. Only reviewed file hashes were
refreshed in the integrity seal; all other sealed files and adapter trees matched.

These tests do not establish a successful real build, imports/model loading or
product acceptance. No oracle, split, consumed holdout, model or retained evidence
was used or changed. ProductCertification=false; NX-B2.1 remains unstarted.

## Exact operator next action

Close unnecessary apps yourself, keeping Docker Desktop and PowerShell running.
Run commands separately from the repository:

```powershell
Set-Location 'C:\Users\sheik\Workspace\Office\Projects\NexusAI'
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\preflight_nxmmr_demo_activation.ps1
```

Only on ACTIVATION_PREFLIGHT=PASS:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\activate_nxmmr_demo.ps1
```

Build logs now appear in that terminal and are saved continuously. Only after
ACTIVATION=PASS:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\verify_nxmmr_demo_activation.ps1
```

On failure, stop and share the new receipt path; do not run verification or
acceptance. Preserve existing receipts/models; no cleanup is required. Fresh
API/Ask/Data/citation/Activity/UI/manual acceptance follows successful activation
verification under the existing operator runbook, not automatically here.
