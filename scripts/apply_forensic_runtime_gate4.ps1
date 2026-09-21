$ErrorActionPreference = "Stop"
$RepoRoot = Split-Path -Parent $PSScriptRoot
$RuntimeEnvPath = Join-Path $RepoRoot ".env.forensic-runtime.local"
$LogRoot = Join-Path $RepoRoot "reports\runtime-activation-20260730"
$Gate3Marker = Join-Path $LogRoot "gate3-backup.json"
$MarkerPath = Join-Path $LogRoot "gate4-runtime-db.json"
. (Join-Path $PSScriptRoot "forensic_runtime_common.ps1")

Assert-ForensicMarker $Gate3Marker 'Gate 3'
$gate3 = Get-Content -Raw -LiteralPath $Gate3Marker | ConvertFrom-Json
$backupPath = Join-Path (Join-Path $RepoRoot '.phase2-backups') $gate3.backup_file
if (-not (Test-Path -LiteralPath $backupPath)) { throw 'Gate 3 backup is missing' }
$actualHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $backupPath).Hash.ToLowerInvariant()
if ($actualHash -ne $gate3.sha256) { throw 'Gate 3 backup hash no longer matches its marker' }
$runtime = Get-ForensicRuntimeEnvironment $RuntimeEnvPath
$DbFiles = '/docker-entrypoint-initdb.d'

Push-Location $RepoRoot
try {
  Invoke-ForensicNative -FailureMessage 'Could not stop API/worker before database mutation' -Command {
    docker compose -p nexusai -f .\docker-compose.forensic-records.yaml stop forensic-records-api forensic-records-worker
  }
  foreach ($sql in @(
    '010_runtime_rls_policies.preflight.sql',
    '010_runtime_rls_policies.sql',
    '010_runtime_rls_policies.verify.sql',
    'runtime_role.grants.sql'
  )) {
    Invoke-ForensicNative -FailureMessage "$sql failed" -LogPath (Join-Path $LogRoot "gate4-$sql.log") -Command {
      docker compose -p nexusai -f .\docker-compose.forensic-records.yaml exec -T forensic-postgres `
        psql -v ON_ERROR_STOP=1 -U localrecall -d localrecall -f "$DbFiles/$sql"
    }
  }

  $passwordSql = "ALTER ROLE forensic_runtime PASSWORD '$($runtime.FORENSIC_RUNTIME_DB_PASSWORD)';"
  $savedPreference = $ErrorActionPreference
  try {
    $ErrorActionPreference = 'Continue'
    $passwordSql | docker compose -p nexusai -f .\docker-compose.forensic-records.yaml exec -T forensic-postgres `
      psql -q -v ON_ERROR_STOP=1 -U localrecall -d localrecall
    $passwordExit = $LASTEXITCODE
  } finally {
    $ErrorActionPreference = $savedPreference
    $passwordSql = $null
  }
  if ($passwordExit -ne 0) { throw 'Runtime password assignment failed' }

  Invoke-ForensicNative -FailureMessage 'Rolled-back non-owner RLS proof failed' -LogPath (Join-Path $LogRoot 'gate4-rls-proof.log') -Command {
    docker compose -p nexusai -f .\docker-compose.forensic-records.yaml exec -T forensic-postgres `
      psql -v ON_ERROR_STOP=1 -U localrecall -d localrecall -f "$DbFiles/runtime_role.rls.verify.sql"
  }
  $roleFlags = docker compose -p nexusai -f .\docker-compose.forensic-records.yaml exec -T forensic-postgres `
    psql -At -v ON_ERROR_STOP=1 -U localrecall -d localrecall -c `
    "SELECT rolname,rolsuper,rolcreatedb,rolcreaterole,rolreplication,rolinherit,rolbypassrls FROM pg_roles WHERE rolname='forensic_runtime';"
  if ($LASTEXITCODE -ne 0) { throw 'Runtime role flag verification failed' }
  $roleFlags = $roleFlags.Trim()
  if ($roleFlags -ne 'forensic_runtime|f|f|f|f|f|f') { throw "Unsafe runtime role flags: $roleFlags" }
  [ordered]@{
    gate = 4; completed_utc = (Get-Date).ToUniversalTime().ToString('o')
    backup_file = $gate3.backup_file; backup_sha256 = $actualHash
    migration = '010_runtime_rls_policies'; runtime_role = 'forensic_runtime'; role_flags = 'all_false'
  } | ConvertTo-Json | Set-Content -Encoding ascii -LiteralPath $MarkerPath
} finally { Pop-Location }

Write-Host 'ActivationGate4=PASS Migration010=verified RuntimeRole=nonowner RoleFlags=all_false RLSProof=PASS'
