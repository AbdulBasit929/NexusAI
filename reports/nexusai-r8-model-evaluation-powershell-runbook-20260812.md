# NexusAI R8 local model-evaluation runbook

This workflow downloads candidates into an isolated Docker volume and runs the
actual benchmark with networking disabled. It does not rebuild, restart or
mount the production NexusAI API, worker, database, NATS, evidence volumes or
LocalAI model library. Model promotion remains a separate evidence-backed step.

## Recommended command

Close memory-heavy applications, keep Docker Desktop running, then use Windows
PowerShell from the repository root:

```powershell
Set-Location C:\Users\sheik\Workspace\Office\Projects\NexusAI
powershell -ExecutionPolicy Bypass -File .\scripts\run_nexusai_r8_model_evaluation.ps1 -PreflightOnly
powershell -ExecutionPolicy Bypass -File .\scripts\run_nexusai_r8_model_evaluation.ps1
```

The default BOS source avoids dependence on Hugging Face availability. To use
the official Hugging Face source instead:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\run_nexusai_r8_model_evaluation.ps1 -ModelSource HuggingFace
```

The first run builds the evaluator, downloads the official Paddle models and
can take substantial time. The named cache volume makes reruns resumable. After
a successful first run, repeat without rebuilding or downloading:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\run_nexusai_r8_model_evaluation.ps1 -SkipImageBuild -SkipModelDownload
```

If the evaluator source was corrected after the models downloaded, rebuild the
small source layers while retaining the downloaded model cache:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\run_nexusai_r8_model_evaluation.ps1 -SkipModelDownload
```

## Expected result

The last line must be `R8ModelEvaluation=PASS`. The evidence bundle is written
under `reports\runtime-evaluation-r8`. Raw synthetic fixtures and intermediate
predictions stay under ignored `.tmp\r8-model-evaluation` storage. A PASS means
the benchmark completed; it does not mean a model passed the production gates.

If the script stops for low RAM or disk, free the stated resource and rerun. Do
not add `--remove-orphans`, remove named production volumes, or copy the OCR
packages into the forensic worker. The isolated evaluation must be reviewed
before any governed production role is assigned.
