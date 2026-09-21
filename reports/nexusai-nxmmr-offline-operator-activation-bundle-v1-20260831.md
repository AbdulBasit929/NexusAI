# Operator Activation Bundle Ready

Prepared and safely validated on 2026-08-31. **No activation was performed.**
The bundle is independent of Codex, but not guaranteed air-gapped: the current
worker Dockerfile can require Internet for uncached apt/pip build dependencies.
No model download is required or permitted. No live dependency installation was
performed during preparation.

# Files Created / Reused

Created dedicated scripts:

- `scripts/nxmmr_demo_operator_common.ps1`
- `scripts/preflight_nxmmr_demo_activation.ps1`
- `scripts/activate_nxmmr_demo.ps1`
- `scripts/verify_nxmmr_demo_activation.ps1`
- `scripts/rollback_nxmmr_demo_activation.ps1`
- `scripts/run_nxmmr_demo_activation.ps1`
- `scripts/prepare_nxmmr_demo_acceptance.ps1`
- `scripts/nxmmr_demo_readiness_probe.py`
- `scripts/test_nxmmr_demo_operator_bundle.ps1`

Reused the existing worker Dockerfile, Compose topology, source/model admission,
and `configuration/nxmmr_demo_activation_v1.json` as the sole activation authority.
Corrected that manifest to explicitly disable ASR/face/image embeddings/TTS,
declare Paddle paths/hashes and offline model behavior, and use privately staged
Paddle sources. Added `configuration/nxmmr_demo_operator_integrity.json` only as
a source/script/manifest/notice hash seal, not competing activation policy.

The older activation helper was inspected but not extended: it stops protected
services and rebuilds LocalAI/UI, conflicting with this worker-only authorization.
The dedicated [operator runbook](../docs/demo/nxmmr-offline-operator-activation-v1.md)
is the primary handoff; the existing team-lead runbook points to it.

# Preflight Script

`preflight_nxmmr_demo_activation.ps1` reads current Git HEAD and dirty summary,
Docker availability, Compose validity, exact current container identities,
worker/API/LocalAI/PostgreSQL/NATS health, active jobs, physical RAM, free disk,
source/model hashes, admission/notice seal, and rollback source identity.
It has no runtime or filesystem mutation. Standard output provides exact reasons,
measured values, requirements and meaningful exit code 2 when blocked. `-Json`
provides the full receipt for optional operator redirection. Activation reruns
this check and writes the receipt durably, even on failure.

# Activation Script

`activate_nxmmr_demo.ps1` serializes activation with a named mutex, creates a
restricted private receipt directory, repeats preflight, snapshots actual worker
configuration, validates both generated Compose files, copies only exact admitted
models, verifies destinations, rechecks RAM immediately before build, builds only
the worker, pins its immutable image ID, verifies built source/imports and absence
of Ultralytics, rechecks health/jobs/source/hash/RAM, then recreates only the worker
with `--no-deps --no-build --pull never`. It invokes verification automatically.

Build stdout/stderr is captured completely in `build.log` on command completion.
The build has a 3,600-second timeout. A failure before recreation leaves the old
worker untouched. A post-recreation failure attempts bounded rollback. Other
native commands and readiness waits are also bounded. Only an owned timed-out
command is terminated; no arbitrary app or protected service is killed.

# Verification Script

`verify_nxmmr_demo_activation.ps1` reads the durable activation state and verifies
the built immutable image, protected IDs, healthy services, zero restart count,
exact role flags, destination hashes, successful local model loading, absence of
Ultralytics, a stability window, and zero active jobs. The isolated probe blocks
network connections and performs no inference/evidence processing. Model-load
readiness is distinct from product acceptance. It writes health, role-readiness,
after-container and success/failure receipts. Exit 4 indicates failure.

# Rollback Script

`rollback_nxmmr_demo_activation.ps1` uses the hash-verified private snapshot of the
original immutable worker image and actual environment/configuration. It restores
the prior ASR=true value as well as the prior disabled Image ANPR/OCR state; it
does not substitute new manifest defaults. It recreates only the worker and
verifies restored image/environment/health plus unchanged protected services.
It handles a missing worker following failed recreation. The original container
ID cannot survive recreation; a replacement ID is recorded. Exit 5 is an explicit
failed recovery. Copied model files remain inactive, never deleted by rollback.

# Acceptance Commands

After `ACTIVATION_VERIFICATION=PASS`, reopen the browser/Codex and run:

```powershell
Set-Location 'C:\Users\sheik\Workspace\Office\Projects\NexusAI'
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\prepare_nxmmr_demo_acceptance.ps1
```

It verifies the active worker is still the verified image and creates fresh
controlled copies of six existing samples. It does not upload, score a benchmark,
or start heavy processes. Open `http://localhost:8080` manually, use a new controlled
case/collection, and follow the team-lead runbook one sample at a time. Confirm
fresh job/evidence/result identity; stop if deduplication reuses old results.
Never bulk reprocess retained evidence.

Image acceptance: classification, ANPR execution, bbox/crop, candidate OCR,
Data, Ask, citation, Activity, reopen; plus negative zero-result behavior.
Video acceptance: V3 classification/execution, actual analyzed frames, groups,
source-time timeline/seek, Ask, citation, Activity, limitations. OCR acceptance:
English/Urdu/mixed regions, Unicode, RTL/LTR and identifier order, Data, Ask,
citations and Activity. Manual widths: 390, 820, 1024 and 1440. No manual PASS is
claimed during preparation. The full video later serves only the explicitly
authorized product demo, never another reserved benchmark.

# Exact User PowerShell Sequence

Run each command separately; continue only on its printed PASS:

```powershell
Set-Location 'C:\Users\sheik\Workspace\Office\Projects\NexusAI'
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\preflight_nxmmr_demo_activation.ps1
```

If `ACTIVATION_PREFLIGHT=PASS`:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\activate_nxmmr_demo.ps1
```

Then:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\verify_nxmmr_demo_activation.ps1
```

If recreation occurred and recovery is needed:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\rollback_nxmmr_demo_activation.ps1
```

Optional combined workflow: `scripts/run_nxmmr_demo_activation.ps1` requires a
fresh PASS and explicit `YES`; it does not bypass or downgrade any gate.

# What User Should Close

Codex, Chrome, VS Code/IDE, and other unnecessary applications, manually.

# What User Should Keep Open

Docker Desktop and one PowerShell terminal. Do not stop protected containers.
Retain Internet access if uncached build packages are needed.

# RAM Gate

At least 6 GiB physical RAM available. It is checked at preflight, before asset
placement, immediately before build, and before recreation. Latest full safe
validation measured 4.963467 GiB and correctly blocked. It was not lowered.
After successful activation/health/readiness, browser acceptance uses normal
workload-aware resource policy and does not require keeping Chrome closed.

# Disk Gate

At least 12 GiB free workspace disk. Latest safe validation measured
1,648.368206 GiB and passed.

# Job Gate

Zero active ingest jobs. SQL treats every status other than completed/dead-letter
as active. Live validation found zero. No role or database change was performed.

# Model / Hash Gate

Three exact ANPR files plus three exact Paddle directory trees are checked.
An existing exact destination is reused; a different file/tree stops activation
before build/recreate. The whole media directory is never cleaned or overwritten.
The worker media bind stays read-only; new Windows files are marked read-only.
Source, scripts, configuration, notice and compiled-image Python sources are
checked against the integrity seal. Unexpected edits require review, not seal
regeneration by the operator.

# License / NOTICE Gate

Existing bounded internal-demo `ADMITTED_WITH_NOTICE` is required and sealed.
Ultralytics/V2 `.pt` assets remain blocked. External distribution/customer delivery
needs its separate review. This bundle does not widen admission.

# Service Scope

Only `forensic-records-worker` may be built/recreated. Forensic API, LocalAI/UI,
PostgreSQL and NATS remain protected. Generated plans contain only one service,
with existing external network/volume references. No `compose down`, orphan
removal, prune, volume cleanup, database migration or broad role activation.

# Assets To Be Activated

- Existing 384 ONNX plate detector.
- Existing FastPlate ONNX and matching configuration.
- PP-OCRv5 mobile detector.
- English mobile recognizer.
- Arabic/Urdu mobile recognizer.

The three Paddle trees were staged privately from the read-only existing cache;
digests remained `1771a1ca...2bdd74`, `9580601f...5673a9` and
`3ee6f226...31d4cd1f`. Neither the source volume nor live media mount changed.
No Tesseract model files, V2 `.pt` checkpoints or additional models are placed.

# Roles To Be Enabled

Image ANPR; Video ANPR V3; Paddle printed English, Urdu and mixed OCR.

# Roles Kept Disabled

ASR, TTS, image embeddings/semantic search, face and Video V2. The new image
must have Ultralytics unavailable. These changes occur only when the operator
later activates; the current live ASR state was preserved during preparation.

# Rollback Verification

The original worker identity is `296e0881d891...`; original immutable image is
`sha256:be13dbec4bd9bb62c824140df3777b898d7ad2801a0e68cd512c00dafaf4f9ef`.
Read-only inspection proved it exists. The actual rollback Compose render passed,
and comparison proved the prior environment is preserved byte-for-byte.
Rollback execution itself was not run, because no activation occurred.

# Dry-Run / Parse Tests

Windows PowerShell 5.1 suite: **40 checks passed**, including parser/static safety,
hash parsing, all-gates PASS simulation, below-threshold RAM BLOCKED with zero
mutations, exact resource floors, each boolean gate failure, disk/job failure,
worker-only plan, prior-role restoration, read-only mounts, external network/volume,
path traversal rejection, and restricted durable logging-path creation.

Real read-only checks: Compose validity, source/models/notices, rollback and
health pass; RAM blocks. Both actual worker activation and rollback plans render.
Current environment and protected container/image/health/restart snapshots match.
All six acceptance samples pass hash-only inventory; no fresh copies/uploads were
made during that test. Python probe AST parsing passes. Standalone blocked
preflight returns exit 2. No activation script, build, recreation, rollback,
model readiness load or manual browser acceptance was executed.

Primary private validation receipt:
`local-acceptance-models/nxmmr/private-activation/20260831T093852083Z/`.
Private Compose snapshots include credentials: do not commit or paste them.

# Runtime Impact

`RuntimeMutated=false`, `LiveModelsChanged=false`, `DeploymentPerformed=false`,
`NXB2ActivationPerformed=false`. All five protected live IDs/images/health states
are unchanged. Only a disposable network-disabled helper read the existing
Paddle cache into private staging; it did not deploy a service or alter the cache.
No arbitrary process was terminated. No STT/TTS or new model/data acquisition.

# Retained Impact

`RetainedStateMutated=false`, `ActivityMutated=false`, `DatabaseMigration=false`,
`VolumesChanged=false`. No retained evidence processing. Consumed V2.2 benchmark
receipts and reserved authorization remain untouched. Product certification
remains false and all live/manual acceptance gates remain open.

# Exact User Action

Close unnecessary applications manually, retain Docker Desktop + one PowerShell,
then run the exact preflight/activation/verification sequence above. Do not continue
past any failed command. Preserve its receipt path or output and return it.
After verification PASS, reopen Chrome/Codex and run acceptance preparation.
Only after actual browser acceptance passes should STT/TTS and later multimodal
breadth proceed. Preparation is complete; activation is intentionally left to the
operator.
