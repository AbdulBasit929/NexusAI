# NexusAI R8 replacement-model acquisition and T2-V evaluation

Date: 2026-08-13  
Disposition: acquisition complete; three challengers rejected; T3 not acquired; Boundary A/B blocked; no production mutation

## Outcome

Four official candidate packages were pinned, acquired outside Git under
`C:\NexusAI-Evaluation\R8\Artifacts\1.0.0`, licensed and integrity-receipted.
The machine receipt is `acquisition-receipt.json` in that directory. No
production role, rebuild, retained ingest or promotion was authorized.

| Candidate | Integrity | Decision |
| --- | --- | --- |
| OMZ 0106 FP32 | publisher SHA-384 on XML/BIN | measured; reject |
| OMZ 0123 source | publisher SHA-384 | conversion required; not accepted |
| PARSeq-tiny v1.0.0 | checkpoint SHA-256 `e7a21b543c98e67414a584c93b1dbb71c26e9463ae4aaa391d54e833660a4711`; exact source revision | measured; reject |
| EasyOCR `arabic_g1` | sole weight member publisher MD5 `993074555550e4e06a6077d55ff0449a` | measured; reject |

All measurements used the unchanged T2-V manifest SHA-256
`180889ad2dc78736cba6617ab49b3b910ddfd29e8dfb77a143fc9ce26f50e510`,
network-disabled containers and separate partitions. Holdout was never tuning
input.

| Candidate / partition | Accuracy | Error | P95 | Peak RSS | Gate |
| --- | ---: | ---: | ---: | ---: | --- |
| OMZ 0106 development | P 0.0000 / R 0.0000 | 3 FP / 45 FN | 9.645 ms | 134.359 MiB | blocked |
| OMZ 0106 validation | P undefined / R 0.0000 | 0 FP / 16 FN | 7.271 ms | 134.359 MiB | blocked |
| OMZ 0106 sealed holdout | P undefined / R 0.0000 | 0 FP / 16 FN | 10.307 ms | 134.359 MiB | blocked |
| EasyOCR development | exact 0.1795 | CER 0.4057 | 988.646 ms | 725.574 MiB | blocked |
| EasyOCR validation | exact 0.0714 | CER 0.4950 | 1,066.572 ms | 725.574 MiB | blocked |
| EasyOCR sealed holdout | exact 0.0714 | CER 0.4257 | 880.108 ms | 725.574 MiB | blocked |
| PARSeq development | exact 0.7436 | CER 0.2028 | 94.339 ms | 416.109 MiB | blocked |
| PARSeq validation | exact 0.7857 | CER 0.2079 | 153.617 ms | 416.109 MiB | blocked |
| PARSeq sealed holdout | exact 0.7857 | CER 0.2079 | 88.601 ms | 416.109 MiB | blocked |

PARSeq required a disclosed evaluator-only compatibility lane: Torch 2.2.2 CPU,
Lightning 1.9.5 and the publisher-declared timm 0.4.12. Its historical Torch
1.10.2 wheel is unavailable for the current Python runtime. The checkpoint and
source revision remained exact.

## T3 decision and next path

No real-image dataset was downloaded. CCPD has an official MIT statement and
Google Drive object but no authoritative byte size/checksum; it also needs a
written isolated-processing/non-display privacy disposition. Artificial
Mercosur remains supplemental and lacks a single archive checksum plus resolved
mixed-background rights/privacy. A multi-GB dataset transfer should be run in a
local terminal only after these fields are complete, using a resumable generated
command. Small model acquisitions are safe for Codex to execute directly.

Next: reject OMZ 0106 and EasyOCR from further work; retain PARSeq only as a
measured Latin reference; authorize one closer-domain plate detector and one
current Urdu OCR candidate; complete a fail-closed T3-D/T3-O artifact/privacy
gate. Do not start R8.5 or fine-tuning before the separate approvals and every
unchanged gate passes.

## Reproducible evaluator commands

```powershell
Set-Location C:\Users\sheik\Workspace\Office\Projects\NexusAI
powershell -ExecutionPolicy Bypass -File .\scripts\acquire_nexusai_r8_evaluation_artifacts.ps1 -PreflightOnly
powershell -ExecutionPolicy Bypass -File .\scripts\acquire_nexusai_r8_evaluation_artifacts.ps1
powershell -ExecutionPolicy Bypass -File .\scripts\run_nexusai_r8_omz0106_evaluation.ps1 -SkipImageBuild
powershell -ExecutionPolicy Bypass -File .\scripts\run_nexusai_r8_easyocr_evaluation.ps1 -SkipImageBuild
powershell -ExecutionPolicy Bypass -File .\scripts\run_nexusai_r8_parseq_evaluation.ps1 -SkipImageBuild
```

Verification: acquisition contract 4 candidates/6 artifacts; 5 acquisition and
archive tests; 6 scorer tests; 2 OMZ adapter tests; sealed 96-fixture T2-V
validation; three offline bundles; production LocalAI healthy, forensic API
running and worker healthy with unchanged container identities.
