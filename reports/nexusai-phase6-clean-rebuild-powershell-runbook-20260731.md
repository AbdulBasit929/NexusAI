# NexusAI Phase 6 clean rebuild and activation runbook

> **Current handoff:** use
> `reports/nexusai-records-ui-acceptance-runbook-20260731.md` for the next
> operator rebuild. It supersedes the Phase 6.2 discovery counts and adds the
> Phase 6.3 ANPR matrix, public discovery routes, Records redesign checks,
> collection reconciliation, live log commands, and responsive UI sequence.

Date: 2026-07-31  
Platform: Windows PowerShell, Docker Desktop, 15.71 GiB RAM reference laptop  
Repository: `C:\Users\sheik\Workspace\Office\Projects\NexusAI`

## Verified state before the planned restart

The guarded Phase 6.1/6.2 rebuild already completed successfully once:

- free RAM before build: 6.72 GiB;
- forensic API build: 6.2 seconds from retained clean layers;
- forensic worker build: 7.8 seconds from retained clean layers;
- LocalAI/UI build: 851.7 seconds;
- LocalAI build attempts: one on the successful retry;
- deployed API image: `sha256:eaa48e89507af733043a25d589a4a70db82a2afad4e25a18ff5dac4884a52d11`;
- deployed worker image: `sha256:a5edb8612aff2aaad357ec881f4953257228fc33b08ec454f0503b8a0977a1b0`;
- deployed LocalAI image: `sha256:d63879907f15e9f99a6254ee4b6eead12efe4257b9bec1c8faa55be748036ddd`;
- API, worker, NATS and LocalAI health checks passed;
- named volumes and rollback images were preserved.

The first clean LocalAI attempt failed because Docker DNS temporarily could not
resolve `proxy.golang.org`. Container DNS and HTTPS were subsequently verified,
and the successful build reused retained layers. The gate now permits up to
three bounded LocalAI build attempts and streams every Docker line both to the
terminal and its stage log.

## 1. Restart preparation

1. Save all work and restart Windows normally.
2. After sign-in, do not open browsers, VS Code, Office, ChatGPT or other large
   applications.
3. Start Docker Desktop and wait until it reports that the engine is running.
4. Open one PowerShell window. Administrator mode is normally unnecessary when
   the signed-in account already has Docker Desktop access.

Do not delete Docker volumes, models, images, `.env.forensic-runtime.local`, or
repository files. Never run `docker system prune --volumes` for this workflow.

## 2. Enter the repository and confirm Docker

```powershell
Set-Location 'C:\Users\sheik\Workspace\Office\Projects\NexusAI'

$deadline = (Get-Date).AddMinutes(3)
do {
  Start-Sleep -Seconds 3
  docker info --format '{{.ServerVersion}}' 2>$null
  $dockerReady = $LASTEXITCODE -eq 0
} until ($dockerReady -or (Get-Date) -gt $deadline)

if (-not $dockerReady) {
  throw 'Docker Desktop did not become ready within three minutes.'
}

docker compose -p nexusai --env-file .env.forensic-runtime.local `
  -f .\docker-compose.forensic-records.yaml `
  -f .\docker-compose.forensic-records.runtime.yaml config --quiet

docker compose -p nexusai --env-file .env.forensic-runtime.local `
  -f .\docker-compose.yaml `
  -f .\docker-compose.forensic-runtime.localai.yaml config --quiet
```

Both compose validation commands must return without an error.

## 3. Enforce the 6 GiB RAM gate

```powershell
$os = Get-CimInstance Win32_OperatingSystem
$freeRAMGiB = [math]::Round($os.FreePhysicalMemory / 1MB, 2)
"Free physical RAM: $freeRAMGiB GiB"
```

Continue only when the result is at least `6.00 GiB`. Do not lower the gate in
the script. If it is below 6 GiB:

1. Close every application except Docker Desktop and PowerShell.
2. Stop only the NexusAI containers if they auto-started:

```powershell
$nexusContainers = @(
  'nexusai-api-1',
  'nexusai-forensic-records-api-1',
  'nexusai-forensic-records-worker-1',
  'nexusai-forensic-nats-1',
  'nexusai-forensic-postgres-1'
)

foreach ($name in $nexusContainers) {
  $running = docker inspect --format '{{.State.Running}}' $name 2>$null
  if ($LASTEXITCODE -eq 0 -and $running -eq 'true') {
    docker stop $name
  }
}
```

3. If memory remains low, reset only Docker/WSL memory, then reopen Docker:

```powershell
docker desktop stop
wsl.exe --shutdown
Start-Sleep -Seconds 8
Start-Process -FilePath `
  'C:\Users\sheik\AppData\Local\Programs\DockerDesktop\Docker Desktop.exe' `
  -WindowStyle Hidden
```

Repeat the Docker-ready loop and RAM measurement. On the verified laptop, a
Docker/WSL reset with other applications closed provided more than 6 GiB.

## 4. Start a full transcript and run the guarded rebuild

```powershell
Set-Location 'C:\Users\sheik\Workspace\Office\Projects\NexusAI'

$stamp = Get-Date -Format 'yyyyMMdd-HHmmss'
$transcriptPath = Join-Path $PWD "reports\runtime-activation-20260731\manual-phase6-$stamp.transcript.log"
Start-Transcript -LiteralPath $transcriptPath -Force

try {
  & powershell.exe -NoProfile -ExecutionPolicy Bypass `
    -File '.\scripts\build_deploy_forensic_phase6_gate.ps1'
  if ($LASTEXITCODE -ne 0) {
    throw "Phase 6 gate returned exit code $LASTEXITCODE"
  }
} finally {
  Stop-Transcript
}

"Complete terminal transcript: $transcriptPath"
```

The gate performs the following actions in order:

1. validates the retained runtime contract and secrets shape;
2. preserves API, worker and LocalAI rollback tags;
3. stops application containers if they are running;
4. refuses to build below 6 GiB free RAM;
5. builds API, worker and LocalAI/UI sequentially;
6. retries only the LocalAI build up to three times for transient failures;
7. starts PostgreSQL, NATS and the worker and waits for health;
8. recreates the API and LocalAI containers;
9. verifies API, worker, NATS and LocalAI readiness;
10. writes the immutable activation summary marker;
11. restores all three rollback images automatically if activation fails.

## 5. Watch live logs from a second PowerShell window

```powershell
Set-Location 'C:\Users\sheik\Workspace\Office\Projects\NexusAI'

Get-ChildItem '.\reports\runtime-activation-20260731' |
  Sort-Object LastWriteTime -Descending |
  Select-Object Name, Length, LastWriteTime
```

Follow whichever stage is active:

```powershell
Get-Content '.\reports\runtime-activation-20260731\phase6-api-build.log' -Tail 80 -Wait
Get-Content '.\reports\runtime-activation-20260731\phase6-worker-build.log' -Tail 80 -Wait
Get-Content '.\reports\runtime-activation-20260731\phase6-localai-build-attempt-1.log' -Tail 80 -Wait
```

Use `Ctrl+C` only in the second log-watching window. Do not interrupt the first
PowerShell window that owns the build gate.

Optional read-only resource monitoring:

```powershell
docker stats --no-stream

$os = Get-CimInstance Win32_OperatingSystem
[pscustomobject]@{
  FreeRAMGiB = [math]::Round($os.FreePhysicalMemory / 1MB, 2)
  Time = Get-Date
}
```

## 6. Verify successful activation

```powershell
Get-Content -Raw '.\reports\runtime-activation-20260731\phase6-live-activation.json'

docker ps --format '{{.Names}}|{{.Status}}|{{.Image}}|{{.ID}}'

$healthURLs = @(
  'http://localhost:8080/readyz',
  'http://localhost:8091/healthz',
  'http://localhost:9109/metrics',
  'http://localhost:8222/healthz'
)

foreach ($url in $healthURLs) {
  $response = Invoke-WebRequest -UseBasicParsing -TimeoutSec 15 -Uri $url
  "$url -> $($response.StatusCode)"
}
```

All four URLs must return HTTP 200. Verify deployed and rollback images remain
independently addressable:

```powershell
$images = @(
  'nexusai/forensic-records-api:phase3-runtime',
  'nexusai/forensic-records-worker:phase3-runtime',
  'nexusai/localai-forensic:phase3-runtime',
  'nexusai/forensic-records-api:rollback-before-phase6-20260731',
  'nexusai/forensic-records-worker:rollback-before-phase6-20260731',
  'nexusai/localai-forensic:rollback-before-phase6-20260731'
)

foreach ($image in $images) {
  docker image inspect $image --format "$image|{{.Id}}|{{.Created}}"
}
```

## 7. Basic live Phase 6 discovery checks

```powershell
$contracts = Invoke-RestMethod `
  'http://localhost:8080/api/v1/forensics/contracts'
$contracts | ConvertTo-Json -Depth 20

$cases = Invoke-RestMethod `
  'http://localhost:8080/api/v1/forensics/cases'
$cases | ConvertTo-Json -Depth 20

$status = Invoke-RestMethod `
  'http://localhost:8080/api/records/forensic/status?tenant_id=default&collection_id=records-demo-verified&limit=5'
$status | ConvertTo-Json -Depth 20
```

The catalog must report version `2026-07-31.phase6.2`, three adapters and 25
operations. The governed collection must remain at 9,250 accepted structured
rows: CDR 5,000, IPDR 2,500, access log 1,000 and ANPR 750, with no new failed,
rejected or duplicate jobs caused by the rebuild.

## 8. Typed CDR and IPDR smoke requests

```powershell
$cdrPlan = @{
  contract_version = 'forensics.query-plan/v1'
  tenant_id = 'default'
  case_id = 'records-demo-verified'
  collection_id = 'records-demo-verified'
  intent = 'cdr.device_identity_changes'
  families = @('communications_cdr')
  entities = @(@{ type = 'msisdn'; value = '923001234567' })
  time_range = @{
    from = '2026-07-01'
    to = '2026-08-01'
    timezone = 'Asia/Karachi'
  }
  limit = 25
} | ConvertTo-Json -Depth 8

$cdr = Invoke-RestMethod -Method Post `
  -Uri 'http://localhost:8080/api/v1/forensics/cases/records-demo-verified/query' `
  -ContentType 'application/json' -Body $cdrPlan
$cdr | ConvertTo-Json -Depth 30

$ipdrPlan = @{
  contract_version = 'forensics.query-plan/v1'
  tenant_id = 'default'
  case_id = 'records-demo-verified'
  collection_id = 'records-demo-verified'
  intent = 'ipdr.concurrent_sessions'
  families = @('network_ipdr')
  entities = @(@{ type = 'subscriber_identifier'; value = '923001234567' })
  time_range = @{
    from = '2026-07-01'
    to = '2026-08-01'
    timezone = 'Asia/Karachi'
  }
  limit = 25
} | ConvertTo-Json -Depth 8

$ipdr = Invoke-RestMethod -Method Post `
  -Uri 'http://localhost:8080/api/v1/forensics/cases/records-demo-verified/query' `
  -ContentType 'application/json' -Body $ipdrPlan
$ipdr | ConvertTo-Json -Depth 30
```

Expected contracts are `forensics.enterprise-response/v1`. CDR traces must name
`nexusai.adapter.cdr`; IPDR traces must name `nexusai.adapter.ipdr`. Row-bearing
results must retain source locators. Empty results are acceptable when the
explicit target is absent from retained evidence; invented ownership, DNS,
route, location, payload or attribution facts are not acceptable.

## 9. Failure triage

For a Go proxy/DNS error, test from inside Docker:

```powershell
docker run --rm alpine:3.22 nslookup proxy.golang.org
docker run --rm alpine:3.22 wget -q -S -O NUL https://proxy.golang.org
```

If both pass, rerun the guarded script; retained BuildKit layers should make the
retry much faster. If DNS fails, restart Docker Desktop and WSL as shown in
section 3. Do not disable TLS verification or replace checksums.

If the gate fails for any other reason, preserve the transcript and all stage
logs and do not delete containers or volumes. The gate attempts automatic
rollback. Verify the four health URLs before any further action.

## 10. Resume point

After the post-restart rebuild and sections 6-8 pass, return to Codex with:

- the `Phase6Activation=PASS` terminal line;
- `phase6-live-activation.json`;
- the four HTTP status results;
- any unexpected CDR/IPDR response or log path.

The next work is exhaustive live Phase 6.1/6.2 acceptance, checkpoint closure,
then the approved Phase 6.3 ANPR/geospatial vertical slice. Phase 6.3 must not
start until the post-restart retained counts, provenance and typed operations
are verified.
