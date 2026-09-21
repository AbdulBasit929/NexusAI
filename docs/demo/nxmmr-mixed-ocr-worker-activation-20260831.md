# NX-MMR mixed Urdu/English OCR worker-only activation

This run installs only the tested mixed-script OCR source correction. It does
not rebuild or recreate LocalAI/UI, download packages or models,
upload/reprocess evidence, or change the database, Activity, volumes, model
configuration, upload limits, forensic API, PostgreSQL or NATS. To make the
unchanged 6 GiB gate attainable, it temporarily stops the existing worker and
LocalAI containers, then restarts LocalAI with the same container ID and image.

## Run from standalone Windows PowerShell

1. Save all work. Keep Docker Desktop running.
2. Open an ordinary Windows PowerShell window outside Codex.
3. Paste this one command block:

```powershell
Set-Location 'C:\Users\sheik\Workspace\Office\Projects\NexusAI'
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\deploy_nxmmr_mixed_ocr_worker.ps1 -WaitForRamMinutes 30
```

If the drained host remains just below 6 GiB, use the explicitly bounded RAM
recovery mode instead:

```powershell
Set-Location 'C:\Users\sheik\Workspace\Office\Projects\NexusAI'
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\deploy_nxmmr_mixed_ocr_worker.ps1 -RecoverRam -WaitForRamMinutes 30
```

`-RecoverRam` stops only the recoverable Phone Link and idle lock-screen user
processes. After worker/LocalAI drain it releases Linux page cache only when
both Dirty and Writeback are exactly zero. It uses `drop_caches=1`, never calls
`sync`, and does not stop Defender, Memory Compression, Explorer, Docker,
PostgreSQL or NATS. It deletes no file, image, build cache, model or volume and
does not lower the 6 GiB gate.

The command first verifies exact current service identities, health, zero jobs,
retained counts, sealed sources and 19 OCR/ANPR tests. It then prints:

```text
OPERATOR_ACTION=Close Codex, ChatGPT and browsers now.
```

At that point close Codex, every ChatGPT process, and browsers. Do not close the
PowerShell window, stop Docker, run `wsl --shutdown`, press Ctrl+C, upload files,
or reopen Codex while the script is working. Terminal output is also preserved
under the printed `MixedOCRReceiptDirectory`.

The script waits for the applications-closed condition, arms exact worker
rollback, temporarily stops the old worker and LocalAI, and then waits for the
unchanged 6 GiB RAM floor. It builds one network-disabled source-only worker
layer, runs a generated-pixel model smoke without retained evidence, rechecks
6 GiB, creates only the candidate worker, and restarts the original LocalAI
container unchanged. Forensic API, PostgreSQL and NATS remain running. All
non-worker identities and retained counts must remain unchanged.

Success ends with:

```text
MIXED_OCR_WORKER_DEPLOYMENT=PASS
```

Only after that marker should you reopen Codex and Chrome. Hard-refresh with
`Ctrl+Shift+R`. Do not reuse the old `20260831T115207942Z` completed-build
receipt: it contains the previous OCR revision.

## Optional non-deploying preflight

This requires 6 GiB at the instant it runs and intentionally reports BLOCKED
while Codex consumes too much RAM:

```powershell
Set-Location 'C:\Users\sheik\Workspace\Office\Projects\NexusAI'
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\deploy_nxmmr_mixed_ocr_worker.ps1 -PreflightOnly
```

## Recovery after an interrupted or failed run

The script automatically attempts worker-only rollback after the old worker has
been marked touched. If automatic rollback cannot complete, keep Docker and the
receipt intact. Replace `YOUR_RECEIPT` below with the exact directory printed by
the failed run:

```powershell
Set-Location 'C:\Users\sheik\Workspace\Office\Projects\NexusAI'
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\deploy_nxmmr_mixed_ocr_worker.ps1 -RollbackDirectory 'C:\Users\sheik\Workspace\Office\Projects\NexusAI\local-acceptance-models\nxmmr\private-activation\YOUR_RECEIPT'
```

Do not share `private-container-snapshot.json`, candidate Compose files, or
rollback Compose files because they contain runtime environment values. Share
only the final marker, receipt path and relevant redacted logs.

Deployment success is not product certification. A fresh mixed-script fixture
with a locked human oracle must subsequently pass Data, overlay annotation,
citation, Activity and manual acceptance. The retained failed example is not
automatically retried.
