param(
  [string]$ComposeFile = "docker-compose.forensic-records.yaml",
  [string]$BackupDir = "D:\LocalAI-runtime\backups",
  [string]$MigrationPath = "/docker-entrypoint-initdb.d/005_backfill_canonical_records.sql"
)

$ErrorActionPreference = "Stop"

function Invoke-Compose {
  param([string[]]$Arguments)
  docker compose -f $ComposeFile @Arguments
}

$timestamp = Get-Date -Format "yyyyMMdd-HHmmss"
$backupPath = Join-Path $BackupDir "forensic-before-canonical-backfill-$timestamp.sql"

if (-not (Test-Path -LiteralPath $BackupDir)) {
  New-Item -ItemType Directory -Force -Path $BackupDir | Out-Null
}

Write-Host "Creating PostgreSQL backup: $backupPath"
Invoke-Compose @("exec", "-T", "forensic-postgres", "pg_dump", "-U", "localrecall", "-d", "localrecall") |
  Out-File -LiteralPath $backupPath -Encoding utf8

$backup = Get-Item -LiteralPath $backupPath
if ($backup.Length -le 0) {
  throw "Backup file is empty: $backupPath"
}

Write-Host "Applying canonical records backfill migration..."
Invoke-Compose @("exec", "-T", "forensic-postgres", "psql", "-v", "ON_ERROR_STOP=1", "-U", "localrecall", "-d", "localrecall", "-f", $MigrationPath)

Write-Host "Canonical records counts:"
Invoke-Compose @(
  "exec", "-T", "forensic-postgres", "psql", "-U", "localrecall", "-d", "localrecall",
  "-c",
  "SELECT tenant_id, collection_id, record_type, count(*) AS rows FROM forensic.records GROUP BY tenant_id, collection_id, record_type ORDER BY tenant_id, collection_id, record_type;"
)

Write-Host "Backup verified: $backupPath ($($backup.Length) bytes)"
