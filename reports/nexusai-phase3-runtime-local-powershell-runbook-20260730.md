# NexusAI Phase 3 runtime activation — local PowerShell runbook

Date: 2026-07-30 (Asia/Karachi)

This runbook moves the retained Phase 3 database into a non-owner, authenticated
application runtime and performs the expensive validation/build work in the
operator's PowerShell session. It is resumable by section. It never uses
`docker compose down -v`, never restores or deletes retained data, and uses only
the small synthetic acceptance fixture for the first write smoke.

Do not paste `.env.forensic-runtime.local`, passwords, API keys, raw database
rows, evidence identifiers, or filenames into chat or a report. Stop on the first
non-zero exit code.

Required execution order is `0 → 1 → 2 → 5 → 6 → 7 → 3 → 4 → 8 → 9 → 10`.
This deliberately completes configuration checks, the higher-memory LocalAI test
and all image builds before the pre-010 backup and retained schema/role mutation.
The section numbers group related operations; the execution order above is the
activation authority.

## Gate 0 — shell, repository and resource preflight (read only)

Open a normal PowerShell window and run:

```powershell
Set-Location "C:\Users\sheik\Workspace\Office\Projects\NexusAI"
$ErrorActionPreference = "Stop"
$RepoRoot = (Get-Location).Path
$Python = "C:\Users\sheik\AppData\Local\Programs\Python\Python313\python.exe"
$Go = "C:\Program Files\Go\bin\go.exe"
$RuntimeEnvPath = Join-Path $RepoRoot ".env.forensic-runtime.local"
$LogRoot = Join-Path $RepoRoot "reports\runtime-activation-20260730"
New-Item -ItemType Directory -Force -Path $LogRoot | Out-Null

# Windows PowerShell 5 can promote native stderr to NativeCommandError when it
# is merged into Tee-Object under ErrorActionPreference=Stop. This helper keeps
# visible logs while treating the native process exit code as authoritative.
function Invoke-NativeLogged {
  param(
    [Parameter(Mandatory=$true)][scriptblock]$Command,
    [Parameter(Mandatory=$true)][string]$LogPath,
    [Parameter(Mandatory=$true)][string]$FailureMessage
  )
  $savedPreference = $ErrorActionPreference
  $nativeExitCode = 0
  try {
    $ErrorActionPreference = "Continue"
    & $Command 2>&1 | ForEach-Object {
      if ($_ -is [System.Management.Automation.ErrorRecord]) {
        $_.Exception.Message
      } else {
        $_
      }
    } | Tee-Object -FilePath $LogPath
    $nativeExitCode = $LASTEXITCODE
  } finally {
    $ErrorActionPreference = $savedPreference
  }
  if ($nativeExitCode -ne 0) {
    throw "$FailureMessage (native exit $nativeExitCode)"
  }
}

$os = Get-CimInstance Win32_OperatingSystem
$freeGiB = [math]::Round($os.FreePhysicalMemory / 1MB, 2)
$disk = Get-PSDrive -Name C
[pscustomobject]@{
  FreeRAMGiB = $freeGiB
  FreeDiskGiB = [math]::Round($disk.Free / 1GB, 2)
  Docker = (docker version --format '{{.Server.Version}}')
  Compose = (docker compose version --short)
  Python = (& $Python --version)
  Go = (& $Go version)
} | Format-List

docker compose -f .\docker-compose.forensic-records.yaml ps
git status --short
```

Close browsers, IDE indexing and other memory-heavy programs before the LocalAI
test/build. Do not begin the LocalAI compilation with less than 6 GiB free RAM;
reboot first if necessary. The API/worker tests can be run before that threshold.

## Gate 1 — fast source verification (read only)

Use repository-local Go cache/temp directories to avoid the Windows
`AppData\Local\go-build ... Access is denied` failure:

Preferred one-command runner (do not copy the Markdown fence lines):

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\run_forensic_activation_gate1.ps1
```

The runner is Windows PowerShell 5-safe: expected Python/Go diagnostic stderr is
logged but only a non-zero native exit code fails the gate. The expanded commands
below are retained for auditability and troubleshooting.

```powershell
New-Item -ItemType Directory -Force -Path .\.tmp\go-test,.\.tmp\go-cache | Out-Null
$env:GOTMPDIR = (Resolve-Path .\.tmp\go-test).Path
$env:GOCACHE = (Resolve-Path .\.tmp\go-cache).Path

Invoke-NativeLogged `
  -LogPath (Join-Path $LogRoot "python-forensic-tests.log") `
  -FailureMessage "Python forensic tests failed" `
  -Command {
    & $Python -m unittest `
      ingestion.forensic_records.tests.test_worker `
      ingestion.forensic_records.tests.test_phase2_adapters `
      ingestion.forensic_records.tests.test_phase4_xlsx `
      ingestion.forensic_records.tests.test_phase4_structured
  }

Invoke-NativeLogged `
  -LogPath (Join-Path $LogRoot "go-forensic-api-tests.log") `
  -FailureMessage "Go forensic API tests failed" `
  -Command { & $Go test .\api\forensic_records -count=1 }

git diff --check
if ($LASTEXITCODE -ne 0) { throw "Whitespace validation failed" }
```

The first use of a fresh local Go cache can take several minutes. Let this run in
PowerShell; Codex UI timeouts do not apply there.

## Gate 2 — create local secrets (local ignored file; mutating)

This generates URL-safe 256-bit hexadecimal secrets without printing them and
restricts the file ACL to the current Windows user:

Preferred one-command runner (do not copy the Markdown fence lines):

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\prepare_forensic_runtime_secrets.ps1
```

It refuses to overwrite an existing secret file, validates both secret shapes,
restricts the ACL and proves that Git ignores the resulting file. The expanded
commands below remain for auditability only. **After the preferred runner prints
`ActivationGate2=PASS`, do not execute the expanded commands; proceed directly
to Gate 5.**

```powershell
if (Test-Path -LiteralPath $RuntimeEnvPath) {
  throw "Local runtime env already exists. Preserve it or remove it deliberately before regeneration."
}

function New-CryptographicHexSecret {
  $bytes = New-Object byte[] 32
  $rng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
  try { $rng.GetBytes($bytes) } finally { $rng.Dispose() }
  return [BitConverter]::ToString($bytes).Replace('-', '').ToLowerInvariant()
}
$dbPassword = New-CryptographicHexSecret
$apiKey = New-CryptographicHexSecret
$envText = Get-Content -Raw .\.env.forensic-runtime.example
$envText = $envText.Replace("FORENSIC_RUNTIME_DB_PASSWORD=CHANGE_ME", "FORENSIC_RUNTIME_DB_PASSWORD=$dbPassword")
$envText = $envText.Replace("FORENSIC_RECORDS_API_KEY=CHANGE_ME", "FORENSIC_RECORDS_API_KEY=$apiKey")
Set-Content -LiteralPath $RuntimeEnvPath -Value $envText -Encoding ascii
$currentIdentity = [System.Security.Principal.WindowsIdentity]::GetCurrent().Name
icacls $RuntimeEnvPath /inheritance:r /grant:r "$currentIdentity`:(R,W)" | Out-Null
Remove-Variable dbPassword,apiKey

$RuntimeEnv = ((Get-Content -LiteralPath $RuntimeEnvPath |
  Where-Object { $_ -and -not $_.StartsWith('#') }) -join "`n") |
  ConvertFrom-StringData
if ($RuntimeEnv.FORENSIC_RUNTIME_DB_PASSWORD -notmatch '^[0-9a-f]{64}$') { throw "Invalid DB secret" }
if ($RuntimeEnv.FORENSIC_RECORDS_API_KEY -notmatch '^[0-9a-f]{64}$') { throw "Invalid API secret" }
Write-Host "Local ignored runtime secrets are ready; values were not printed."
```

## Gate 3 — pre-migration recovery point (retained DB; mutating backup only)

PostgreSQL and NATS must be healthy while API/worker remain stopped:

Preferred guarded runner:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\backup_forensic_runtime_gate3.ps1
```

After `ActivationGate3=PASS`, do not run the expanded commands below.

```powershell
docker compose -f .\docker-compose.forensic-records.yaml up -d forensic-postgres forensic-nats
docker compose -f .\docker-compose.forensic-records.yaml stop forensic-records-api forensic-records-worker

$stamp = (Get-Date).ToUniversalTime().ToString('yyyyMMddTHHmmssZ')
$backupName = "phase3-pre-010-runtime-rls-$stamp.dump"
$backupDir = Join-Path $RepoRoot ".phase2-backups"
New-Item -ItemType Directory -Force -Path $backupDir | Out-Null

docker compose -f .\docker-compose.forensic-records.yaml exec -T forensic-postgres `
  pg_dump -U localrecall -d localrecall -Fc -f "/tmp/$backupName"
if ($LASTEXITCODE -ne 0) { throw "pg_dump failed" }

docker compose -f .\docker-compose.forensic-records.yaml exec -T forensic-postgres `
  pg_restore --list "/tmp/$backupName" | Out-Null
if ($LASTEXITCODE -ne 0) { throw "pg_restore catalog check failed" }

docker compose -f .\docker-compose.forensic-records.yaml exec -T forensic-postgres `
  pg_restore --file=/dev/null "/tmp/$backupName"
if ($LASTEXITCODE -ne 0) { throw "pg_restore full archive read failed" }

$postgresId = docker compose -f .\docker-compose.forensic-records.yaml ps -q forensic-postgres
docker cp "${postgresId}:/tmp/$backupName" (Join-Path $backupDir $backupName)
if ($LASTEXITCODE -ne 0) { throw "docker cp backup failed" }

$backup = Get-Item -LiteralPath (Join-Path $backupDir $backupName)
$backupHash = Get-FileHash -Algorithm SHA256 -LiteralPath $backup.FullName
[pscustomobject]@{ File = $backup.Name; Bytes = $backup.Length; SHA256 = $backupHash.Hash } | Format-List
```

Keep the container copy until activation is accepted. It is small and provides a
second recovery copy.

## Gate 4 — migration 010 and non-owner role (retained DB; mutating)

Migration 010 adds no rows and rewrites no evidence. It closes the three missing
tenant policies and makes both aggregate views execute with caller/RLS rights.

Preferred guarded runner:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\apply_forensic_runtime_gate4.ps1
```

It refuses to proceed unless the Gate 3 host backup still matches its recorded
SHA-256. After `ActivationGate4=PASS`, do not run the expanded commands below.

```powershell
$DbFiles = "/docker-entrypoint-initdb.d"
foreach ($sql in @(
  "010_runtime_rls_policies.preflight.sql",
  "010_runtime_rls_policies.sql",
  "010_runtime_rls_policies.verify.sql",
  "runtime_role.grants.sql"
)) {
  Write-Host "Running $sql"
  docker compose -f .\docker-compose.forensic-records.yaml exec -T forensic-postgres `
    psql -v ON_ERROR_STOP=1 -U localrecall -d localrecall -f "$DbFiles/$sql"
  if ($LASTEXITCODE -ne 0) { throw "$sql failed" }
}

$RuntimeEnv = ((Get-Content -LiteralPath $RuntimeEnvPath |
  Where-Object { $_ -and -not $_.StartsWith('#') }) -join "`n") |
  ConvertFrom-StringData
@"
ALTER ROLE forensic_runtime PASSWORD '$($RuntimeEnv.FORENSIC_RUNTIME_DB_PASSWORD)';
"@ | docker compose -f .\docker-compose.forensic-records.yaml exec -T forensic-postgres `
  psql -v ON_ERROR_STOP=1 -U localrecall -d localrecall
if ($LASTEXITCODE -ne 0) { throw "Runtime password assignment failed" }

docker compose -f .\docker-compose.forensic-records.yaml exec -T forensic-postgres `
  psql -v ON_ERROR_STOP=1 -U localrecall -d localrecall `
  -f "$DbFiles/runtime_role.rls.verify.sql"
if ($LASTEXITCODE -ne 0) { throw "Rolled-back non-owner RLS proof failed" }

docker compose -f .\docker-compose.forensic-records.yaml exec -T forensic-postgres `
  psql -v ON_ERROR_STOP=1 -U localrecall -d localrecall -P pager=off -c `
  "SELECT rolname, rolsuper, rolcreatedb, rolcreaterole, rolreplication, rolinherit, rolbypassrls FROM pg_roles WHERE rolname='forensic_runtime';"
```

Expected role flags are all `false`. Never pass the actual password through
`psql -c`, because process command lines can be inspected by other local tools.

## Gate 5 — Compose merge verification (read only)

Preferred one-command runner (do not copy the Markdown fence lines):

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\verify_forensic_runtime_compose.ps1
```

After it prints `ActivationGate5=PASS`, do not execute the expanded commands;
proceed to Gate 6. The expanded commands below remain for auditability only.

```powershell
docker compose -p nexusai --env-file $RuntimeEnvPath `
  -f .\docker-compose.forensic-records.yaml `
  -f .\docker-compose.forensic-records.runtime.yaml `
  config --quiet
if ($LASTEXITCODE -ne 0) { throw "Forensic runtime Compose config failed" }

docker compose -p nexusai --env-file $RuntimeEnvPath `
  -f .\docker-compose.yaml `
  -f .\docker-compose.forensic-runtime.localai.yaml `
  config --quiet
if ($LASTEXITCODE -ne 0) { throw "LocalAI runtime Compose config failed" }
```

Do not run `docker compose config` without `--quiet`; the expanded output can
contain secrets.

## Gate 6 — protobuf and focused LocalAI test (long, read-only source test)

Preferred one-command runner after freeing at least 6 GiB RAM:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\run_forensic_activation_gate6.ps1
```

After `ActivationGate6=PASS`, skip the expanded commands and proceed to Gate 7.

Only regenerate ignored protobuf files if the package is absent:

```powershell
if (-not (Test-Path .\pkg\grpc\proto\backend.pb.go)) {
  Invoke-NativeLogged `
    -LogPath (Join-Path $LogRoot "protogen-go.log") `
    -FailureMessage "Pinned protobuf generation failed" `
    -Command {
      docker run --rm `
        --mount "type=bind,source=$RepoRoot,target=/src" `
        -w /src `
        golang@sha256:1ecb7edf62a0408027bd5729dfd6b1b8766e578e8df93995b225dfd0944eb651 `
        bash -c 'apt-get update && apt-get install -y --no-install-recommends unzip=6.0-28 && make protogen-go'
    }
}

$os = Get-CimInstance Win32_OperatingSystem
$freeGiB = [math]::Round($os.FreePhysicalMemory / 1MB, 2)
if ($freeGiB -lt 6) { throw "Only $freeGiB GiB RAM is free. Reboot/close applications before LocalAI compilation." }

Invoke-NativeLogged `
  -LogPath (Join-Path $LogRoot "go-localai-forensic-focus.log") `
  -FailureMessage "Focused LocalAI forwarding test failed" `
  -Command {
    & $Go test .\core\http\endpoints\localai `
      -run TestLocalAIInternalEndpoints `
      '--ginkgo.focus=forensic records KB upload forwarding' `
      -count=1
  }
```

Do not use `go get github.com/mudler/LocalAI/pkg/grpc/proto`; it is generated
inside this repository.

## Gate 7 — save rollback images and build locally (long, image mutation)

Preferred complete rebuild runner:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\build_forensic_runtime_gate7.ps1
```

It refuses to build without a passing Gate 6 log and at least 6 GiB free RAM,
rollback-tags every discoverable prior image, then builds both forensic sidecars
and the full LocalAI image with visible saved logs. After
`ActivationGate7=PASS`, skip the expanded commands and proceed to Gate 3.

First tag any existing container images. Missing API/worker containers are okay;
LocalAI should normally produce a rollback tag.

```powershell
$rollbackStamp = (Get-Date).ToUniversalTime().ToString('yyyyMMddTHHmmssZ').ToLowerInvariant()
$rollback = @{}
foreach ($service in @('forensic-records-api','forensic-records-worker')) {
  $id = docker compose -f .\docker-compose.forensic-records.yaml ps -aq $service
  if ($id) {
    $imageId = docker inspect --format '{{.Image}}' $id
    $tag = "nexusai/$service`:rollback-$rollbackStamp"
    docker image tag $imageId $tag
    $rollback[$service] = $tag
  }
}
$localAIId = docker compose -p nexusai -f .\docker-compose.yaml ps -aq api
if ($localAIId) {
  $imageId = docker inspect --format '{{.Image}}' $localAIId
  $tag = "nexusai/localai`:rollback-$rollbackStamp"
  docker image tag $imageId $tag
  $rollback['localai'] = $tag
}
$rollback.GetEnumerator() | Sort-Object Name | ForEach-Object { "$($_.Name)=$($_.Value)" } |
  Set-Content -Encoding ascii (Join-Path $LogRoot "rollback-images.txt")
$rollback | Format-Table -AutoSize

$buildStart = Get-Date
Invoke-NativeLogged `
  -LogPath (Join-Path $LogRoot "build-forensic-sidecars.log") `
  -FailureMessage "Forensic sidecar build failed" `
  -Command {
    docker compose -p nexusai --env-file $RuntimeEnvPath `
      -f .\docker-compose.forensic-records.yaml `
      -f .\docker-compose.forensic-records.runtime.yaml `
      --progress plain build forensic-records-api forensic-records-worker
  }
Write-Host "Sidecar build elapsed: $((Get-Date) - $buildStart)"

$buildStart = Get-Date
Invoke-NativeLogged `
  -LogPath (Join-Path $LogRoot "build-localai.log") `
  -FailureMessage "LocalAI build failed" `
  -Command {
    docker compose -p nexusai --env-file $RuntimeEnvPath `
      -f .\docker-compose.yaml `
      -f .\docker-compose.forensic-runtime.localai.yaml `
      --progress plain build api
  }
Write-Host "LocalAI build elapsed: $((Get-Date) - $buildStart)"
```

These are the intentionally long commands. PowerShell can continue even when a
chat frontend would time out. Docker layer output is saved under `reports`.

## Gate 8 — deploy with volumes preserved (runtime mutation)

Preferred guarded runner:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\deploy_forensic_runtime_gate8.ps1
```

It preserves the stopped prior LocalAI container as an additional rollback
artifact, deploys only the Gate 7 images, waits for readiness and attempts to
restart the prior LocalAI container if cutover fails. It never removes a volume.

```powershell
docker compose -p nexusai --env-file $RuntimeEnvPath `
  -f .\docker-compose.forensic-records.yaml `
  -f .\docker-compose.forensic-records.runtime.yaml `
  up -d --no-deps --force-recreate forensic-records-api forensic-records-worker
if ($LASTEXITCODE -ne 0) { throw "Forensic sidecar deployment failed" }

docker compose -p nexusai --env-file $RuntimeEnvPath `
  -f .\docker-compose.yaml `
  -f .\docker-compose.forensic-runtime.localai.yaml `
  up -d --no-deps --force-recreate api
if ($LASTEXITCODE -ne 0) { throw "LocalAI deployment failed" }

Start-Sleep -Seconds 10
docker compose -p nexusai --env-file $RuntimeEnvPath `
  -f .\docker-compose.forensic-records.yaml `
  -f .\docker-compose.forensic-records.runtime.yaml ps
docker compose -p nexusai --env-file $RuntimeEnvPath `
  -f .\docker-compose.yaml `
  -f .\docker-compose.forensic-runtime.localai.yaml ps

Invoke-NativeLogged `
  -LogPath (Join-Path $LogRoot "startup-sidecars.log") `
  -FailureMessage "Reading sidecar startup logs failed" `
  -Command {
    docker compose -p nexusai --env-file $RuntimeEnvPath `
      -f .\docker-compose.forensic-records.yaml `
      -f .\docker-compose.forensic-records.runtime.yaml `
      logs --tail 100 forensic-records-api forensic-records-worker
  }
```

Do not use `down -v`; the named PostgreSQL, NATS, spool, model, KB and LocalAI
volumes are the retained state.

## Gate 9 — health, fail-closed auth and JetStream contract (read only)

Preferred guarded runner:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\verify_forensic_runtime_gate9.ps1
```

It verifies LocalAI readiness, API health, unauthenticated 401, authenticated
scope, wrong-tenant 403 and both required JetStream streams.

```powershell
$RuntimeEnv = ((Get-Content -LiteralPath $RuntimeEnvPath |
  Where-Object { $_ -and -not $_.StartsWith('#') }) -join "`n") |
  ConvertFrom-StringData
$health = Invoke-RestMethod http://localhost:8091/healthz
if (-not $health) { throw "Forensic API health failed" }

try {
  Invoke-WebRequest http://localhost:8091/query/templates -UseBasicParsing | Out-Null
  throw "Unauthenticated sidecar request unexpectedly succeeded"
} catch {
  $status = [int]$_.Exception.Response.StatusCode
  if ($status -ne 401) { throw "Expected 401, received $status" }
}

$headers = @{
  Authorization = "Bearer $($RuntimeEnv.FORENSIC_RECORDS_API_KEY)"
  'X-Forensic-Tenant-ID' = $RuntimeEnv.FORENSIC_RECORDS_TENANT_ID
  'X-Forensic-Actor-ID' = 'nexusai-runtime-operator'
  'X-Forensic-Subject-ID' = 'nexusai-runtime-operator'
  'X-Forensic-Actor-Role' = 'admin'
  'X-Forensic-Collection-ID' = 'nexusai-runtime-acceptance-20260730'
}
$templates = Invoke-RestMethod http://localhost:8091/query/templates -Headers $headers
if (-not $templates.templates) { throw "Authenticated template request failed" }

$js = Invoke-RestMethod 'http://localhost:8222/jsz?streams=true&consumers=true'
$streamNames = @($js.account_details.stream_detail.name)
foreach ($required in @('FORENSIC_RECORDS_INGEST','FORENSIC_RECORDS_DLQ')) {
  if ($streamNames -notcontains $required) { throw "Missing JetStream stream $required" }
}
Write-Host "Health=PASS Auth401=PASS AuthenticatedScope=PASS JetStream=PASS"
```

## Gate 10 — one isolated synthetic success and read/query smoke (mutating)

This uses the five-row synthetic Pakistan CDR fixture, not the supplied real CDR
or `records-demo-verified`:

Preferred guarded runner:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\smoke_forensic_runtime_gate10.ps1
```

It also verifies the exact 5 total / 4 uniquely inserted / 1 duplicate-row / 0
rejected accounting, repeat-upload idempotency and authenticated deterministic
queries.
After `ActivationGate10=PASS`, do not run the expanded commands below.

```powershell
$collection = 'nexusai-runtime-acceptance-20260730'
$fixture = Join-Path $RepoRoot 'ingestion\forensic_records\tests\fixtures\pakistan_cdr_messy_synthetic.csv'
$raw = & curl.exe -sS -X POST 'http://localhost:8091/webhooks/records/upload' `
  -H "Authorization: Bearer $($RuntimeEnv.FORENSIC_RECORDS_API_KEY)" `
  -H "X-Forensic-Tenant-ID: $($RuntimeEnv.FORENSIC_RECORDS_TENANT_ID)" `
  -H 'X-Forensic-Actor-ID: nexusai-runtime-operator' `
  -H 'X-Forensic-Subject-ID: nexusai-runtime-operator' `
  -H 'X-Forensic-Actor-Role: admin' `
  -H "X-Forensic-Collection-ID: $collection" `
  -F "tenant_id=$($RuntimeEnv.FORENSIC_RECORDS_TENANT_ID)" `
  -F "collection_id=$collection" `
  -F 'case_id=runtime-acceptance-20260730' `
  -F 'record_type=auto' `
  -F 'skip_kb_mirror=true' `
  -F "file=@$fixture"
if ($LASTEXITCODE -ne 0) { throw "Synthetic upload transport failed" }
$upload = $raw | ConvertFrom-Json
if (-not $upload.evidence_id) { throw "Synthetic upload did not return evidence identity" }

$deadline = (Get-Date).AddMinutes(3)
do {
  Start-Sleep -Seconds 2
  $item = Invoke-RestMethod "http://localhost:8091/evidence/$($upload.evidence_id)" -Headers $headers
  $state = $item.item.processing_status
} until ($state -in @('completed','failed') -or (Get-Date) -gt $deadline)
if ($state -ne 'completed') { throw "Synthetic evidence ended in state '$state'" }
Write-Host "SyntheticSuccess=PASS"

& .\scripts\smoke_forensic_records.ps1 `
  -CollectionId $collection `
  -TenantId $RuntimeEnv.FORENSIC_RECORDS_TENANT_ID `
  -ApiKey $RuntimeEnv.FORENSIC_RECORDS_API_KEY `
  -ActorId 'nexusai-runtime-operator' `
  -SubjectId 'nexusai-runtime-operator' `
  -ActorRole admin `
  -CaseId 'runtime-acceptance-20260730' `
  -Target '923001234567' `
  -SkipReport
if ($LASTEXITCODE -ne 0) { throw "Authenticated forensic query smoke failed" }
```

Never deliberately corrupt or interrupt the retained registry for fault
injection. During activation, the isolated synthetic job naturally exercised the
bounded retry and DLQ path after a worker SQL defect, preserving a 5/5 terminal
parent. After the fix, Gate 10 immutably reprocessed that same synthetic evidence
as generation 1, and Gate 11 proved completed-message redelivery. This accepted
the live failure path without involving real evidence or manufactured retained
database corruption.

## Gate 11 — controlled completed-message redelivery (synthetic only)

Run only after Gate 10 passes:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\verify_forensic_runtime_gate11.ps1
```

This republishes only the completed isolated synthetic job, waits for synchronous
acknowledgement, requires the work stream to drain, and proves that canonical-row
and job counts do not change. It does not read or publish real evidence.

## Rollback — stop the new sidecars and restore prior LocalAI image

Use this if Gate 8–10 fails. Migration 010 can safely remain because it is
additive and the prior owner connection bypasses RLS. The pre-activation state had
the forensic API/worker stopped, so rollback restores that truthful state:

```powershell
  docker compose -p nexusai --env-file $RuntimeEnvPath `
  -f .\docker-compose.forensic-records.yaml `
  -f .\docker-compose.forensic-records.runtime.yaml `
  stop forensic-records-api forensic-records-worker

$rollbackFile = Join-Path $LogRoot 'rollback-images.txt'
$rollbackValues = ((Get-Content $rollbackFile) -join "`n") | ConvertFrom-StringData
if ($rollbackValues.localai) {
  $env:NEXUSAI_LOCALAI_RUNTIME_IMAGE = $rollbackValues.localai
  docker compose -p nexusai --env-file $RuntimeEnvPath `
    -f .\docker-compose.yaml `
    -f .\docker-compose.forensic-runtime.localai.yaml `
    up -d --no-deps --force-recreate api
  if ($LASTEXITCODE -ne 0) { throw "LocalAI rollback failed" }
}
```

Do not run a database restore merely because an application container failed.
A restore is a separately approved disaster-recovery operation and would discard
post-backup history.

## What to return for review

Return only these non-sensitive summaries:

- free RAM/disk and tool versions;
- final `Ran N tests ... OK` and Go `ok` lines;
- backup filename, byte size and SHA-256;
- migration/preflight/verify exit status and the all-false role flags;
- build elapsed times and final image names (not environment output);
- Compose service health/status and sanitized last error lines;
- the single `Health=...` and `SyntheticSuccess=...` pass lines.

Never return the runtime env file, headers, bearer value, database URL, upload
response IDs, raw evidence, or expanded Compose configuration.
# Phase 4 structured closure (run after Phase 3 Gate 11)

The Phase 4 API/UI deployment is complete through Gate 13. To deploy the final
capability materialization fix and close the nine-path structured acceptance,
run:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass `
  -File .\scripts\repair_forensic_phase4_api_gate13b.ps1

powershell.exe -NoProfile -ExecutionPolicy Bypass `
  -File .\scripts\smoke_forensic_phase4_structured_gate12.ps1
```

Require both PASS summaries. Gate 12 is synthetic-only and hard-bound to the
neutral `nexusai-structured-demo-v2-20260730` collection. It does not use the
supplied real CDR or `records-demo-verified`.
