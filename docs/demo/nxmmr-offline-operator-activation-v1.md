# NX-MMR operator activation bundle v1

Prepared only. Nothing has been activated. This bundle needs no Codex interaction
while running. “Offline operator” means independent of Codex, not an air-gapped
Docker build: the existing Dockerfile can require Internet for uncached apt/pip
packages. It never downloads models. Keep Docker Desktop running and connected.

## 2026-08-31 dependency correction / retry

The first operator attempt at `20260831T094201498Z` passed RAM admission, copied
the exact assets, and failed at pip resolution before recreation. The worker
requirements now pin `safetensors==0.6.2` instead of `0.5.3`, which could not
satisfy PaddlePaddle 3.3.1's minimum of 0.6.0. The old receipt remains immutable.
No rollback is needed for that attempt: the original worker is still running.

Use the same preflight/activation commands below after the correction validation
handoff. A retry gets a new receipt directory and reuses existing assets only
after their hashes match. Do not delete models or edit the old BUILD_STARTED
receipt to force verification. Verification runs only after a successful new
activation. Dependency resolution is not proof of a successful image build or
model-load readiness; those remain operator-time gates.

The second attempt at `20260831T100208816Z` passed dependency resolution but
timed out downloading PaddlePaddle (125.9/194.8 MB); recreation never started.
It also needs no rollback. The build now uses a 120-second pip socket timeout,
five connection retries, and at most three whole pip attempts with five-second
pauses. A BuildKit package cache reuses completed downloads across attempts and
builds; it is not a live model/data volume. The overall build deadline remains
3,600 seconds. Repeated network failure still stops activation before recreation.

## Exact operator sequence

Close Codex, Chrome, VS Code/IDE and unnecessary applications yourself. Keep
Docker Desktop and one PowerShell terminal open. Do not stop any containers.

```powershell
Set-Location 'C:\Users\sheik\Workspace\Office\Projects\NexusAI'
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\preflight_nxmmr_demo_activation.ps1
```

Only when it prints `ACTIVATION_PREFLIGHT=PASS`:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\activate_nxmmr_demo.ps1
```

Activation reruns preflight, copies exact admitted assets, rechecks RAM immediately
before build and again before recreation, builds/recreates the worker only, and
automatically verifies it. The Docker build can take time; the script prints the
receipt directory, streams build stdout/stderr into this terminal, and flushes
each line to `build.log` as it arrives (also captured in `activation.log`). Partial
logs survive a command failure/timeout. Only build output is echoed: raw
inspect/config/environment snapshots remain private. Do not run a second
activation concurrently or interrupt the build.

Only if activation prints `ACTIVATION=PASS`, independently check:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\verify_nxmmr_demo_activation.ps1
```

If recreation failed, activation attempts rollback automatically. If it requests
manual rollback, or later verification fails:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\rollback_nxmmr_demo_activation.ps1
```

If any command fails, do not guess. Preserve the printed receipt path/output and
return it to Codex. Do not run verification or acceptance after a failed build.
Do not rerun a build after timeout without inspecting the
receipt; a timed-out Docker client does not prove a backend build has stopped.
The script does not force-close applications, prune, delete media, or stop the
protected services.

## After verification passes

Once `ACTIVATION_VERIFICATION=PASS`, reopen Chrome/Codex. The 6 GiB gate is complete
for that deployment operation; it is not a permanent browser-acceptance floor.
Actual processing must still follow normal workload-aware resource/health policy.

```powershell
Set-Location 'C:\Users\sheik\Workspace\Office\Projects\NexusAI'
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\prepare_nxmmr_demo_acceptance.ps1
```

This creates fresh, hash-verified copies under that activation receipt. It does
not upload evidence, run inference, score a benchmark, or open a browser. Open
`http://localhost:8080` yourself and follow the existing
`docs/demo/nexusai-team-lead-multimodal-demo-v3.md` runbook in a controlled new
case/collection, one file at a time. Record evidence/version/job identity and
confirm it is fresh processing under V3. If deduplication reuses older results,
stop; do not bulk reprocess retained evidence to force a fresh result.

Samples enumerated and hash-pinned by the acceptance script:

- Image positive: `C:\Users\sheik\Downloads\archive\Pakistani License Number Plates Data\Cars\DSC_1068.JPG` (previous non-holdout development sample).
- Image negative: `local-acceptance-models/nxmmr/private-benchmarks/ocr/fixtures/empty_scene.png`.
- Video: `C:\Users\sheik\Downloads\sample.mp4`.
- Printed English: `local-acceptance-models/nxmmr/private-benchmarks/anpr-ocr-vertical-v1/multilingual-ocr-fixtures/english-01.png`.
- Printed Urdu: same fixture directory, `urdu-01.png`.
- Mixed: same fixture directory, `mixed-01.png`.

The existing generated printed samples support product plumbing checks, not new
independent accuracy certification. Inspect the original printed text yourself;
no model output is an oracle and no analytical answers are encoded in this bundle.
The full video is explicitly authorized here as a fresh product demonstration,
not another development/reserved benchmark. Do not score or tune against it or
alter any benchmark receipt.

For each image verify classification, processing, bbox/crop, candidate text,
confidence, Data, Ask, citation, Activity and reopen identity. Verify the negative
zero-result state. For video also verify V3 processor identity, actual analyzed
frames, groups, first/last source time, timeline/seek and limitations. For OCR
verify Unicode, source regions, RTL/LTR and identifier order. Test widths 390,
820, 1024 and 1440, keyboard navigation, zero-result, failed and model-required
states. Record manual PASS/FAIL; do not infer acceptance from this bundle's tests.

## Gates and scope

Preflight reads current Git HEAD/dirty summary, Docker, Compose validity, exact
container identities, service/HTTP health, active jobs, physical RAM, disk,
source hashes, local asset hashes, notice/license admission and rollback identity.
It intentionally does not write files; it prints diagnostics (or structured
JSON with `-Json`). Activation repeats the exact check and stores `preflight.json`
inside its private receipt, even if it fails. To capture a standalone JSON check:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\preflight_nxmmr_demo_activation.ps1 -Json > .\local-acceptance-models\nxmmr\operator-bundle-v1\operator-preflight.json
```

The thresholds are unchanged: RAM at least 6 GiB; disk at least 12 GiB; active jobs
zero; protected services healthy; every hash and admission gate verified.
There is no runtime override or flag to skip a failing gate. A named mutex blocks
concurrent activation/rollback. All external commands have finite timeouts.

Enabled only: Image ANPR, Video V3, Paddle printed English/Urdu/mixed OCR.
Disabled: Video V2, ASR, TTS, face and image embeddings. Ultralytics must not be
installed/importable in the new worker. Tesseract model files are not placed.
No API recreation is required by the current manifest. LocalAI/UI, forensic API,
PostgreSQL and NATS are protected from recreation.

Model source inventory: three exact ANPR files plus three exact Paddle trees.
Paddle was copied from the pre-existing cache to a private local source directory
during preparation, with identical tree digests. The cache and live model mount
were not changed. Destination conflicts stop activation before build/recreate;
matching files are reused. New files have the Windows read-only attribute; the
worker's whole media bind is read-only. Windows attributes are not represented
as POSIX 0444/0555 ACLs.

The sole activation authority remains `configuration/nxmmr_demo_activation_v1.json`.
`configuration/nxmmr_demo_operator_integrity.json` is only an integrity seal for
scripts/source/manifest/notice, not a second policy manifest. Do not regenerate
the seal to bypass unexpected edits. Return the changed-file failure for review.

## Rollback and logs

Before build, the operator snapshot captures the original immutable worker image,
full environment (including the currently enabled ASR), entrypoint/command, user,
working directory, health check, ports, mounts, logging, restart policy and
external network/volume references. Unsupported topology fails preflight.
Rollback restores those values rather than new defaults; the original container
ID cannot be restored after recreation, so the replacement ID is recorded.
The original image remains locally available; the activation build uses a separate
tag and switches to its immutable ID before recreation.

A failed build does not recreate the old live worker. If the new worker fails
health/model-load readiness, automatic rollback is attempted. Rollback recreates
only the worker with `--no-deps --no-build --pull never`. It changes no PostgreSQL
data, NATS state, Activity, retained evidence, named volume, or LocalAI/UI service.
Copied models are left inactive; rollback never deletes them. Rollback is a
recovery operation, not another build, and does not require the 6 GiB build floor.

Private receipts are under `local-acceptance-models/nxmmr/private-activation/`.
Each run is timestamped; `latest.json` points to the latest prepared activation.
Never commit or paste raw Compose snapshots: they contain live credentials and
inherit a restricted operator/SYSTEM/Administrators ACL. Share sanitized status
and the receipt path with Codex, not the credentials.

Successful runs include `preflight.json`, `activation.log`, `build.log`,
`container-before.json`, `container-after.json`, `health.json`, `model-hashes.json`,
`role-readiness.json`, `rollback.json`, immutable build/source receipts, and private
activation/rollback Compose snapshots. Failure may stop before later files exist;
that is a failure marker, not a missing PASS. Readiness means local model loading
only; fresh product acceptance remains separate.

Exit codes: 0 PASS; 1 cancelled; 2 preflight blocked; 3 activation failed;
4 verification failed; 5 rollback failed; 6 acceptance preparation blocked;
7 self-test failed. No permission or configuration decision requires Codex during
execution: stop on any failure and return the receipt/output.

Optional combined command (same gates, explicit YES/NO):

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\run_nxmmr_demo_activation.ps1
```

No STT/TTS or later breadth starts now. After actual activation **and** browser
acceptance pass, continue English STT, Urdu STT, Roman Urdu, English/Urdu TTS,
image similarity/search and safe face candidate comparison. Do not reopen ANPR
optimization. `ProductCertification=false`; NX-B2.1 remains unstarted.
