# NX-B2.1D offline operator bundle

The original Qwen holdout was consumed partially on 2026-09-04. The bundle now
refuses to run it again. Model evaluation and B1+C+D activation remain separate
operator decisions.

Bundle revision 2026-09-04 corrects all observed preflight stops (ordinal
manifest ordering, optional Docker Health shape, and the ingest-status enum
predicate), safely releases Linux clean file cache after hashing the 4.28 GB
model artifact, and adds exact activation/rollback image and retained-state checks.
Cache release requires Linux Writeback to be zero, preserves any dirty pages,
never issues `sync` or Docker cleanup, and waits up to 60 seconds for Windows to
observe reclaimed RAM before enforcing the immediate pre-load gate.
The consumed run is preserved at
`local-acceptance-models/nxb21-d/offline-runs/evaluation-20260904T053938055Z`.
It attempted 81/168 cases before Ginkgo's one-hour suite timeout. The public
receipt is `reports/nxb21/d-model-evaluation-receipt-v1.json`; it records the
configuration mismatch, zero valid structured outputs, severe memory pressure,
safe unload, and `DExitDecision=FAIL`.

## Command 1: retired one-shot Qwen evaluation

Do not run this command against the existing corpus. The script exits with
`HOLDOUT_ALREADY_CONSUMED`. A future evaluator may be enabled only after the
planner/profile passes a development corpus and a new independent holdout is
frozen. Future runs use both Go and Ginkgo four-hour timeouts, stream verbose Go
output, save a case checkpoint after every result, and use `/system` for reliable
loaded-model detection and unload verification.

```powershell
cd C:\Users\sheik\Workspace\Office\Projects\NexusAI
Set-ExecutionPolicy -Scope Process Bypass -Force
.\scripts\nxb21d-offline\run_nxb21d_qwen_evaluation.ps1
```

Stable evaluation exit codes: `0` pass, `10` RAM too low, `11` source/runtime drift, `12` active jobs or unhealthy runtime, `20` model load failure, `21` invalid evaluation, `22` model insufficient, `23` critical safety failure, `24` unload failure, `30` post-check failure.

## Command 2: future approved B1+C+D activation

Do not run this command with the current failed D receipt. It is retained here as
the exact future activation entry point. Run it only after a new independent D
evaluation passes, Codex reviews the suitable receipt, an activation source seal
exists, and the owner separately approves activation. It refuses missing,
changed, failed, incomplete, non-100%-critical, source-drifted, low-RAM,
active-job, or runtime-drifted states before build.

```powershell
cd C:\Users\sheik\Workspace\Office\Projects\NexusAI
Set-ExecutionPolicy -Scope Process Bypass -Force
.\scripts\nxb21d-offline\run_nxb21_bcd_activation.ps1 -OwnerApproved
```

Stable activation exit codes: `0` verified, `40` prerequisite failure, `41` RAM failure, `42` build failure, `43` recreation failure, `44` verification failure.

The activation scope is exactly `api` and `forensic-records-api`. It does not recreate the worker, PostgreSQL, NATS, or LocalAI. Each activation run writes an exact rollback Compose receipt in its private run directory. Use `rollback_nxb21_bcd_activation.ps1 -RunDirectory <exact-run-directory>` only when the failed activation log directs rollback; rollback restores the two captured starting images without touching retained evidence, database volumes, NATS, or models.

Detailed holdout results and resource samples are written under ignored `local-acceptance-models/nxb21-d/offline-runs/`. Public output contains aggregate scores and hashes. The evaluator never sends normal Ask requests and never rewrites planner source, prompts, schemas, expectations, or the frozen corpus.
