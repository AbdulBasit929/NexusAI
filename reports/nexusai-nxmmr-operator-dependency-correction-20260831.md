# NX-MMR operator dependency correction

Scope: source correction and non-installing validation only. No image build,
activation, rollback or retained processing is authorized in this correction.

## Cause and correction

The operator receipt
`local-acceptance-models/nxmmr/private-activation/20260831T094201498Z/build.log`
records pip's unsatisfiable requirements: the worker pinned safetensors 0.5.3,
while PaddlePaddle 3.3.1 requires safetensors >=0.6.0. Transformers 4.49.0 allows
safetensors >=0.4.1. The single dependency change is the exact pin **0.6.2**, whose
published CPython-compatible Linux wheel is available in the
[official PyPI release](https://pypi.org/project/safetensors/0.6.2/).

PaddlePaddle, Transformers, the V3 processor, all ANPR/OCR models and benchmark
receipts remain unchanged. The operator had already copied exact admitted model
assets before the failed build; those are preserved and will be reused only when
hashes match. No worker recreation occurred, so this failure does not need rollback.

## Validation method and limitation

`scripts/validate_nxmmr_worker_dependencies.py` runs in the already-cached image
`sha256:90744cff8f32887f075c47d747a173ff333e9e98801667af93c357fa9f5e28ff`, Linux
x86-64, CPython 3.11.15, pip 24.0. The disposable container has a read-only root,
1,536 MiB memory limit, two CPUs, no live data/model mounts and a bounded timeout.
Only requirements and validator source are mounted read-only; package metadata,
wheels and reports go to the private validation directory.

Pip uses `--dry-run --ignore-installed --no-build-isolation`. Binary packages are
required except NATS 2.12.0, for which existing setuptools/wheel prepare source
metadata without installing build dependencies. The installed distribution list
is compared before/after. The first diagnostic's all-wheel restriction rejected
that NATS release and is retained separately; no runtime pin was changed for it.

The final resolver outcome is recorded in
`local-acceptance-models/nxmmr/private-activation/dependency-repair-20260831/attempt2/validation.json`.
Dependency resolution does not prove image building, imports, model loading, live
service health or browser acceptance; all remain mandatory operator-time gates.

## Regression protection

Three PowerShell tests require the exact new pin, reject the incompatible old
0.5.x pin, and preserve PaddlePaddle/Transformers versions. Existing resource,
scope, rollback, source/model-hash and no-mutation tests remain in the bundle.
The integrity seal is refreshed only for reviewed corrections, not unrelated
worktree changes. The operator's historical failed receipt is not rewritten.

## Completed validation results

- Full dependency resolution: **PASS**, exit 0, 477.352 seconds, no timeout.
- Installed distributions unchanged; no package installation, image build,
  model loading/download, or live-service mutation.
- Requirements SHA-256:
  `21f457b8f1a6dcabe79b7f7b52d46de55144a57008b63b8e4ee9efac81415a4c`.
- Resolution report SHA-256:
  `32f14f37fb54fc60f2468f1f7b1a7cedf0008848e8ec36069e04119d90dd86a9`.
- Operator bundle: **43 checks PASS**, including read-only live checks;
  receipt `private-activation/20260831T095545498Z`. Integrity seal: **PASS**.
- Available RAM during validation: 3.715 GiB, correctly BLOCKED below 6 GiB;
  disk 1649.330 GiB and active jobs 0. Retry must pass a fresh resource check.
- Original worker and protected service identities, images, health and restart
  counts unchanged. No benchmark or consumed-holdout rerun occurred.

The resolver proves satisfiable package metadata for this captured Linux/Python
environment, not successful image construction or runtime compatibility. Build,
imports, model-health and fresh API/Ask/Data/citation/Activity/UI/manual product
acceptance remain outstanding. Nothing is promoted to PRODUCT_CERTIFIED.

## Retry sequence

After the completed correction validation is reported, manually close Codex,
Chrome/IDE and unnecessary applications; keep Docker Desktop + PowerShell open.
Run each command separately and continue only when its gate passes:

```powershell
Set-Location 'C:\Users\sheik\Workspace\Office\Projects\NexusAI'
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\preflight_nxmmr_demo_activation.ps1
```

Only on `ACTIVATION_PREFLIGHT=PASS`:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\activate_nxmmr_demo.ps1
```

Only after successful activation:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\verify_nxmmr_demo_activation.ps1
```

No cleanup or rollback is needed before retrying this pre-recreation failure.
A retry creates a new receipt. If it fails, stop and return that receipt path;
do not edit the old state file to make verification pass. Reopen Codex/Chrome
only after activation verification PASS, then prepare fresh browser acceptance
using the existing runbook. RAM >=6 GiB, disk >=12 GiB, jobs=0 and protected
service/hash/license gates are unchanged. Product certification remains false.
