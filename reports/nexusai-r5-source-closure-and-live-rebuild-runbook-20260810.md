# NexusAI R5 source closure and live rebuild runbook

Date: 2026-08-10  
Status: R5 source accepted; live deployment and controlled intake acceptance pending

## Source closure

R5-EVID-01 through R5-EVID-05 are source accepted. The bounded R5 product now
provides a case-scoped evidence catalog and detail desk, exact processing and
row accounting, citations and append-only custody history, bounded bulk intake
with durable per-file results, queue observability, and read-only reprocess
eligibility review with a fail-closed approval boundary.

Swag v1.16.6 regenerated `swagger/docs.go`, `swagger/swagger.json`, and
`swagger/swagger.yaml`. All publish:

`GET /api/v1/forensics/cases/{case_id}/evidence/{evidence_id}/reprocess-plan`

The generated contract is intentionally larger than the previous July snapshot
because it reconciles all accepted annotations added since that snapshot.

## Verified source gates

- Forensic API package: PASS (`go test ./api/forensic_records -count=1`).
- Generated Swagger JSON parsing and exact R5 route: PASS.
- Generated Swagger package and LocalAI endpoint compilation: PASS; Windows
  reported a post-completion executable-unlink denial, the previously recorded
  local antivirus/file-lock condition.
- React production build: PASS, Vite 8.0.16, 669 modules.
- Current production-bundle Case Workspace browser suite: PASS, 14/14 in 18.9
  seconds with one Chromium worker.
- R5 activation script PowerShell parser: PASS.
- No Docker image/container/volume, database, evidence, model, configuration,
  Git staging, commit, push, or publication was changed by source closure.

## Safe operator rebuild

Close memory-intensive applications, start Docker Desktop, wait for its Linux
engine to report running, and open PowerShell. Then:

```powershell
Set-Location 'C:\Users\sheik\Workspace\Office\Projects\NexusAI'

docker info --format '{{.ServerVersion}}'

$tokens = $null
$errors = $null
[System.Management.Automation.Language.Parser]::ParseFile(
  (Resolve-Path '.\scripts\build_deploy_nexusai_r5_gate.ps1'),
  [ref]$tokens,
  [ref]$errors
) | Out-Null
if ($errors.Count) { $errors | Format-List; throw 'R5 gate syntax failed' }

powershell -NoProfile -ExecutionPolicy Bypass `
  -File '.\scripts\build_deploy_nexusai_r5_gate.ps1' `
  -CaseID 'nexusai-forensic-demo'
```

The gate calls the already accepted sequential combined rebuild. It requires
the existing runtime environment and Gate 9 marker, preserves the currently
deployed API, worker, and LocalAI images under rollback tags, preserves named
volumes, stops services only for the bounded memory-safe build, retries the
LocalAI build at most three times, deploys health-checked services, and rolls
back automatically if build/deployment/health fails. It never calls Docker
prune and never deletes a volume.

After deployment it sends only GET requests. It verifies LocalAI readiness,
records health, `/app`, the public workflow catalog, the case-bound evidence
catalog, and the selected evidence reprocess plan. It asserts that approval is
required and execution is not permitted. It never invokes the POST reprocess
endpoint.

If local auth is enabled, set the key only in the current process before the
same command:

```powershell
$env:LOCALAI_API_KEY = '<your-local-api-key>'
```

Do not paste the key into a transcript. Remove it afterward:

```powershell
Remove-Item Env:\LOCALAI_API_KEY -ErrorAction SilentlyContinue
```

## Required success evidence

The final line must begin with:

```text
R5Activation=PASS
```

Then collect the non-secret marker and current health state:

```powershell
Get-Content -Raw '.\reports\runtime-activation-20260810\r5-live-acceptance.json'
Invoke-WebRequest -UseBasicParsing 'http://localhost:8080/readyz' | Select-Object StatusCode
Invoke-WebRequest -UseBasicParsing 'http://localhost:8091/healthz' | Select-Object StatusCode
docker compose -p nexusai --env-file '.\.env.forensic-runtime.local' `
  -f '.\docker-compose.forensic-records.yaml' `
  -f '.\docker-compose.forensic-records.runtime.yaml' ps
```

Return the final PASS line, marker JSON, and any error tail. Do not return API
keys or `.env` contents.

## Remaining R5 live gate

The rebuild gate proves deployment and read-only operation. Final R5 program
completion still requires one explicitly acknowledged, controlled synthetic
bulk intake that proves upload-to-ready accounting and a bounded partial-failure
outcome against the deployed code. That gate will retain evidence/job/audit
rows, so it is deliberately not embedded in the rebuild command and must not be
run without the operator's explicit data-mutation acknowledgement. Reprocess
execution is not required for R5 and remains a separate governed workflow.
