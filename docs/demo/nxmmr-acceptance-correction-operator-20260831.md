# Run the OCR / 300 MiB correction with Codex closed

Prepared 2026-08-31. **Operator-run only; not deployed by Codex.** This replaces
the old worker-only activation command for this correction. Do not run
`activate_nxmmr_demo.ps1` or merge the upload fragment into a guessed Compose
command. Their historical identity/scope does not describe this deployment.

## What will happen

The new script snapshots the **currently activated** worker and LocalAI/API
configuration into an ACL-restricted receipt directory. It validates the exact
current identities, host configuration, sealed correction sources, original
model hashes, service health, zero ingest jobs and at least 12 GiB disk.
The **6 GiB free host RAM** floor remains unchanged. It never closes other apps,
stops Docker/WSL, evicts caches, changes memory limits or deletes anything to
reach it.

Builds run sequentially: first a small OCR-only layer on the existing worker
image, then LocalAI plus its embedded React UI. The latter uses existing cached
layers and Go parallelism limits (GOMAXPROCS=4, GOFLAGS=-p=2). LocalAI build
dependencies may need Internet inside build stages; no models or datasets are
downloaded by the script, and no packages are installed into the live worker.
Private `.env` files are excluded from the LocalAI build context with a
Dockerfile-specific ignore file. Do not edit source files during the build.

Both candidate images must build and pass source/version/embedded-UI checks
before replacement starts. RAM is rechecked before each build and replacement.
Only `forensic-records-worker` and `api` are recreated, with `--no-deps`,
`--no-build`, `--pull never`. LocalAI replacement briefly interrupts port 8080.
Forensic API, PostgreSQL and NATS must retain their container IDs/images.
Existing mounts and runtime environment are preserved, apart from the intended
LocalAI upload-limit variables set to 320 MiB. The UI ceiling is 300 MiB per
general/video file; image ceiling stays 64 MiB.

## 1. Prepare your desktop

Save all work. Close browsers, Codex and other nonessential apps. Keep Docker
Desktop/its engine running. Do **not** quit Docker, run `wsl --shutdown`, stop
database/NATS services, or prune images/volumes. Open a standalone Windows
PowerShell window, not a terminal hosted inside Codex. Ensure nobody uploads
files or runs Ask/processing during deployment.

The existing `docker`, `python`, and `node` commands must remain available in
that PowerShell session. No new host installation is performed.

## 2. Run a non-deploying preflight (optional)

```powershell
Set-Location 'C:\Users\sheik\Workspace\Office\Projects\NexusAI'
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\deploy_nxmmr_acceptance_correction.ps1 -PreflightOnly
```

`CORRECTION_PREFLIGHT=PASS` confirms readiness at that instant, not deployment.
Below 6 GiB, it reports BLOCKED and does not build or recreate anything. Each
run prints its exact receipt directory; preflight may create private rollback
snapshots, but does not alter services or tag/build images.

## 3. Deploy from standalone PowerShell

The first full build completed successfully but stopped before replacement when
Docker/BuildKit memory remained allocated. Do **not** rebuild it. The completed
candidate images and receipts are pinned by
`configuration/nxmmr_correction_completed_build.json`.

```powershell
Set-Location 'C:\Users\sheik\Workspace\Office\Projects\NexusAI'
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\deploy_nxmmr_acceptance_correction.ps1 -UseBuiltRunDirectory 'C:\Users\sheik\Workspace\Office\Projects\NexusAI\local-acceptance-models\nxmmr\private-activation\20260831T115207942Z' -DrainTargetsForRam -WaitForRamMinutes 30
```

You may start the command while reading these instructions. It verifies the
immutable completed images, repeats source/test/image smokes and checks zero
jobs, then prints `OPERATOR_APPS_CLOSED=WAITING`. At that point close Codex,
**every ChatGPT window/process**, and browsers. The script will not stop targets
until ChatGPT and Codex processes are actually gone. It then records both
targets as rollback-required, stops only the old
worker and LocalAI, and waits up to thirty minutes for their RAM to return.
Forensic API, PostgreSQL and NATS stay running. The six-GiB threshold is not
lowered. If the threshold is still unavailable, or either candidate fails to
start, guarded rollback restores both old targets. **Do not close this
PowerShell window or press Ctrl+C after the old targets are stopped.**

Do not paste intermediate Waiting/RAM output back into Codex. That necessarily
reopens ChatGPT/Codex (about 1.5 GiB observed) and can terminate or starve the
standalone wait. Leave the external PowerShell alone until it prints either
`CORRECTION_DEPLOYMENT=PASS` or a final `CORRECTION=FAILED` plus rollback result.

The resume prints `COMPLETED_IMAGES=VERIFIED BuildSkipped=true Downloads=false`.
It performs no new build or download. Runtime/replacement output appears in the
terminal and in the new printed receipt's `correction.log`, `drain-targets.log`
and `replace-*.log`. No old image is removed or overwritten.

Expected final marker:

```text
CORRECTION_DEPLOYMENT=PASS
```

Only then reopen Codex/browser. In Chrome, use Ctrl+Shift+R on the existing
Multimodal Product Acceptance collection. This marker confirms deployment,
health, preserved runtime configuration/mounts, model-load readiness and an
HTTP header admission probe; it is **not** an actual 300 MiB upload test or
PRODUCT_CERTIFIED status.

## 4. If it stops

- **RAM blocked before replacement:** no live services were replaced. Keep
  logs. If closing apps does not provide 6 GiB, stop and share the marker/RAM
  measurement. Do not edit the gate, restart Docker or use an unreviewed
  low-memory workaround. Cached successful build layers remain reusable.
- **Build/source/configuration failure before replacement:** original live
  services stay in place. Share the relevant build log or error, not private
  snapshots. Do not blindly reuse old activation scripts.
- **Failure after replacement starts:** automatic rollback attempts to restore
  only the touched services from their exact saved images/configurations. It
  will not interrupt active processing jobs or overwrite an unrelated newer
  deployment. If rollback is blocked/fails, preserve the receipt and stop.
- **Console/power interruption:** inspect the receipt first. A manual rollback
  command is below; it does not rebuild images or require six GiB. It still
  requires zero jobs and valid original snapshots. If a prior mutex is reported
  abandoned, investigate the interrupted run rather than launching deployment.

Replace the example receipt path below with the **CorrectionReceiptDirectory
printed by this correction run**, not an old activation/preflight receipt:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\deploy_nxmmr_acceptance_correction.ps1 -RollbackDirectory 'C:\Users\sheik\Workspace\Office\Projects\NexusAI\local-acceptance-models\nxmmr\private-activation\YOUR_CORRECTION_RUN'
```

Do not share `private-container-snapshot.json`, `candidate-*.json` or
`rollback-*.json`: they contain environment credentials. Share status markers
and relevant redacted logs only.

## 5. Product testing remains a separate next step

The script uploads no evidence, does not retry the failed negative image, does
not change Activity via analyst actions, and does not edit database records.
It performs read-only health/count checks and a model-load probe. Counts are
guardrails, not proof of byte-for-byte database identity.

After deployment PASS, report the marker and receipt directory. Confirm the
next controlled upload/retry procedure in the existing **Multimodal Product
Acceptance** collection. The 175.9 MiB `video-v3.mp4` fits the new allowance, but
actual upload/processing/UI acceptance remains untested until then. Do not
rename/edit the retained negative image to evade deduplication. No sealed
benchmark rerun, direct product certification or NX-B2.1 is authorized here.
