param(
  [switch]$ApproveSchemaMigration
)

$ErrorActionPreference = 'Stop'
if (-not $ApproveSchemaMigration) {
  throw 'Migration 011 changes one custody trigger function. Rerun with -ApproveSchemaMigration after reviewing the R5 live-progress report.'
}

$gateMutex = New-Object System.Threading.Mutex($false, 'Local\NexusAIR5CustodyMigration')
if (-not $gateMutex.WaitOne(0)) {
  $gateMutex.Dispose()
  throw 'Another NexusAI R5 custody migration is already running'
}

$RepoRoot = Split-Path -Parent $PSScriptRoot
$RuntimeEnvPath = Join-Path $RepoRoot '.env.forensic-runtime.local'
$ActivationMarker = Join-Path $RepoRoot 'reports\runtime-activation-20260810\r5-live-acceptance.json'
$LogRoot = Join-Path $RepoRoot 'reports\runtime-activation-20260810'
$BackupDir = Join-Path $RepoRoot '.phase2-backups'
$MarkerPath = Join-Path $LogRoot 'r5-custody-migration-011.json'
$DbFiles = '/docker-entrypoint-initdb.d'
. (Join-Path $PSScriptRoot 'forensic_runtime_common.ps1')

Assert-ForensicMarker $ActivationMarker 'R5 activation'
[void](Get-ForensicRuntimeEnvironment $RuntimeEnvPath)
New-Item -ItemType Directory -Force -Path $LogRoot,$BackupDir | Out-Null

function Invoke-R5MigrationSQL {
  param([Parameter(Mandatory=$true)][string]$FileName)
  Invoke-ForensicNative -FailureMessage "$FileName failed" `
    -LogPath (Join-Path $LogRoot "r5-$FileName.log") -Command {
      docker compose -p nexusai --env-file $RuntimeEnvPath `
        -f .\docker-compose.forensic-records.yaml `
        -f .\docker-compose.forensic-records.runtime.yaml `
        exec -T forensic-postgres psql -v ON_ERROR_STOP=1 `
        -U localrecall -d localrecall -f "$DbFiles/$FileName"
    }
}

$servicesStopped = $false
$migrationApplied = $false
$backup = $null
$backupHash = ''

Push-Location $RepoRoot
try {
  docker compose -p nexusai --env-file $RuntimeEnvPath `
    -f .\docker-compose.forensic-records.yaml `
    -f .\docker-compose.forensic-records.runtime.yaml `
    up -d forensic-postgres forensic-nats | Out-Null
  if ($LASTEXITCODE -ne 0) { throw 'Could not start PostgreSQL and NATS' }

  foreach ($service in @('forensic-postgres', 'forensic-nats')) {
    $containerID = docker compose -p nexusai --env-file $RuntimeEnvPath `
      -f .\docker-compose.forensic-records.yaml `
      -f .\docker-compose.forensic-records.runtime.yaml ps -q $service
    if ($LASTEXITCODE -ne 0 -or -not $containerID) { throw "$service container is missing" }
    Wait-ForensicContainer -ContainerId $containerID -Label $service
  }

  docker compose -p nexusai --env-file $RuntimeEnvPath `
    -f .\docker-compose.forensic-records.yaml `
    -f .\docker-compose.forensic-records.runtime.yaml `
    stop forensic-records-api forensic-records-worker | Out-Null
  if ($LASTEXITCODE -ne 0) { throw 'Could not stop API and worker before migration' }
  $servicesStopped = $true

  $stamp = (Get-Date).ToUniversalTime().ToString('yyyyMMddTHHmmssZ')
  $backupName = "r5-pre-migration-011-$stamp.dump"
  $containerBackup = "/tmp/$backupName"
  Invoke-ForensicNative -FailureMessage 'R5 pre-migration pg_dump failed' `
    -LogPath (Join-Path $LogRoot 'r5-migration-011-backup.log') -Command {
      docker compose -p nexusai --env-file $RuntimeEnvPath `
        -f .\docker-compose.forensic-records.yaml `
        -f .\docker-compose.forensic-records.runtime.yaml `
        exec -T forensic-postgres pg_dump -U localrecall -d localrecall `
        -Fc -f $containerBackup
    }
  Invoke-ForensicNative -FailureMessage 'R5 backup archive validation failed' -Command {
    docker compose -p nexusai --env-file $RuntimeEnvPath `
      -f .\docker-compose.forensic-records.yaml `
      -f .\docker-compose.forensic-records.runtime.yaml `
      exec -T forensic-postgres pg_restore --list $containerBackup
  }
  $postgresID = docker compose -p nexusai --env-file $RuntimeEnvPath `
    -f .\docker-compose.forensic-records.yaml `
    -f .\docker-compose.forensic-records.runtime.yaml ps -q forensic-postgres
  $backupPath = Join-Path $BackupDir $backupName
  docker cp "${postgresID}:$containerBackup" $backupPath | Out-Null
  if ($LASTEXITCODE -ne 0) { throw 'Could not copy the verified R5 backup to the host' }
  $backup = Get-Item -LiteralPath $backupPath
  if ($backup.Length -le 0) { throw 'R5 pre-migration backup is empty' }
  $backupHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $backupPath).Hash.ToLowerInvariant()

  Invoke-R5MigrationSQL '011_r5_custody_chain_tail.preflight.sql'
  Invoke-R5MigrationSQL '011_r5_custody_chain_tail.sql'
  $migrationApplied = $true
  Invoke-R5MigrationSQL '011_r5_custody_chain_tail.verify.sql'

  docker compose -p nexusai --env-file $RuntimeEnvPath `
    -f .\docker-compose.forensic-records.yaml `
    -f .\docker-compose.forensic-records.runtime.yaml `
    up -d forensic-records-worker forensic-records-api | Out-Null
  if ($LASTEXITCODE -ne 0) { throw 'Could not restart the forensic API and worker after migration 011' }
  foreach ($service in @('forensic-records-worker', 'forensic-records-api')) {
    $containerID = docker compose -p nexusai --env-file $RuntimeEnvPath `
      -f .\docker-compose.forensic-records.yaml `
      -f .\docker-compose.forensic-records.runtime.yaml ps -q $service
    Wait-ForensicContainer -ContainerId $containerID -Label $service `
      -AllowRunningWithoutHealth
  }
  $deadline = (Get-Date).AddMinutes(3)
  do {
    try { $apiHealth = Invoke-WebRequest -UseBasicParsing -TimeoutSec 10 -Uri 'http://localhost:8091/healthz' } catch { $apiHealth = $null }
    try { $workerHealth = Invoke-WebRequest -UseBasicParsing -TimeoutSec 10 -Uri 'http://localhost:9109/metrics' } catch { $workerHealth = $null }
    if ($apiHealth.StatusCode -eq 200 -and $workerHealth.StatusCode -eq 200) { break }
    Start-Sleep -Seconds 3
  } until ((Get-Date) -gt $deadline)
  if ($apiHealth.StatusCode -ne 200 -or $workerHealth.StatusCode -ne 200) {
    throw 'Forensic services did not recover after migration 011'
  }
  $servicesStopped = $false

  [ordered]@{
    migration = '011_r5_custody_chain_tail'
    status = 'verified'
    applied_utc = (Get-Date).ToUniversalTime().ToString('o')
    backup_file = $backup.Name
    backup_bytes = $backup.Length
    backup_sha256 = $backupHash
    retained_rows_rewritten = 0
    api_health = 'pass'
    worker_health = 'pass'
  } | ConvertTo-Json | Set-Content -Encoding ascii -LiteralPath $MarkerPath
} catch {
  if ($migrationApplied) {
    try { Invoke-R5MigrationSQL '011_r5_custody_chain_tail.rollback.sql' } catch {
      Write-Warning "Migration rollback failed; preserve the verified backup and inspect before continuing: $($_.Exception.Message)"
    }
  }
  throw
} finally {
  if ($servicesStopped) {
    docker compose -p nexusai --env-file $RuntimeEnvPath `
      -f .\docker-compose.forensic-records.yaml `
      -f .\docker-compose.forensic-records.runtime.yaml `
      up -d forensic-records-worker forensic-records-api | Out-Null
  }
  Pop-Location
  $gateMutex.ReleaseMutex()
  $gateMutex.Dispose()
}

Write-Host "R5CustodyMigration=PASS Migration=011 Backup=$($backup.Name) BackupSHA256=$backupHash RetainedRowsRewritten=0"
