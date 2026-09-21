$ErrorActionPreference = "Stop"
$RepoRoot = Split-Path -Parent $PSScriptRoot
$LogRoot = Join-Path $RepoRoot "reports\runtime-activation-20260730"
$BackupDir = Join-Path $RepoRoot ".phase2-backups"
$MarkerPath = Join-Path $LogRoot "gate3-backup.json"
. (Join-Path $PSScriptRoot "forensic_runtime_common.ps1")

New-Item -ItemType Directory -Force -Path $LogRoot,$BackupDir | Out-Null
foreach ($image in @(
  'nexusai/forensic-records-api:phase3-runtime',
  'nexusai/forensic-records-worker:phase3-runtime',
  'nexusai/localai-forensic:phase3-runtime'
)) { Assert-ForensicImage $image }
Assert-ForensicMarker (Join-Path $LogRoot 'rollback-images.txt') 'Gate 7 rollback'

Push-Location $RepoRoot
try {
  Invoke-ForensicNative -FailureMessage 'Could not start PostgreSQL and NATS' -Command {
    docker compose -p nexusai -f .\docker-compose.forensic-records.yaml up -d forensic-postgres forensic-nats
  }
  Invoke-ForensicNative -FailureMessage 'Could not stop API/worker before backup' -Command {
    docker compose -p nexusai -f .\docker-compose.forensic-records.yaml stop forensic-records-api forensic-records-worker
  }
  foreach ($service in @('forensic-postgres','forensic-nats')) {
    $id = docker compose -p nexusai -f .\docker-compose.forensic-records.yaml ps -q $service
    if ($LASTEXITCODE -ne 0 -or -not $id) { throw "$service container is missing" }
    Wait-ForensicContainer -ContainerId $id -Label $service
  }

  $stamp = (Get-Date).ToUniversalTime().ToString('yyyyMMddTHHmmssZ')
  $backupName = "phase3-pre-010-runtime-rls-$stamp.dump"
  $containerPath = "/tmp/$backupName"
  Invoke-ForensicNative -FailureMessage 'pg_dump failed' -LogPath (Join-Path $LogRoot 'gate3-pg-dump.log') -Command {
    docker compose -p nexusai -f .\docker-compose.forensic-records.yaml exec -T forensic-postgres `
      pg_dump -U localrecall -d localrecall -Fc -f $containerPath
  }
  Invoke-ForensicNative -FailureMessage 'pg_restore catalog validation failed' -LogPath (Join-Path $LogRoot 'gate3-pg-restore-list.log') -Command {
    docker compose -p nexusai -f .\docker-compose.forensic-records.yaml exec -T forensic-postgres pg_restore --list $containerPath
  }
  Invoke-ForensicNative -FailureMessage 'pg_restore full archive read failed' -LogPath (Join-Path $LogRoot 'gate3-pg-restore-read.log') -Command {
    docker compose -p nexusai -f .\docker-compose.forensic-records.yaml exec -T forensic-postgres pg_restore --file=/dev/null $containerPath
  }
  $postgresId = docker compose -p nexusai -f .\docker-compose.forensic-records.yaml ps -q forensic-postgres
  $localPath = Join-Path $BackupDir $backupName
  docker cp "${postgresId}:$containerPath" $localPath | Out-Null
  if ($LASTEXITCODE -ne 0) { throw 'Copying the verified backup to the host failed' }
  $backup = Get-Item -LiteralPath $localPath
  if ($backup.Length -le 0) { throw 'Backup is empty' }
  $localHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $localPath).Hash.ToLowerInvariant()
  $containerHashLine = docker compose -p nexusai -f .\docker-compose.forensic-records.yaml exec -T forensic-postgres sha256sum $containerPath
  if ($LASTEXITCODE -ne 0) { throw 'Could not hash the container backup copy' }
  $containerHash = ($containerHashLine -split '\s+')[0].ToLowerInvariant()
  if ($localHash -ne $containerHash) { throw 'Host/container backup hashes differ' }
  [ordered]@{
    gate = 3; created_utc = (Get-Date).ToUniversalTime().ToString('o')
    backup_file = $backup.Name; backup_bytes = $backup.Length; sha256 = $localHash
  } | ConvertTo-Json | Set-Content -Encoding ascii -LiteralPath $MarkerPath
} finally { Pop-Location }

Write-Host "ActivationGate3=PASS Backup=$($backup.Name) Bytes=$($backup.Length) SHA256=$localHash Postgres=healthy NATS=healthy"
