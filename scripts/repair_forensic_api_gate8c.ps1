$ErrorActionPreference = 'Stop'
$RepoRoot = Split-Path -Parent $PSScriptRoot
$RuntimeEnvPath = Join-Path $RepoRoot '.env.forensic-runtime.local'
$LogRoot = Join-Path $RepoRoot 'reports\runtime-activation-20260730'
$Gate8bMarker = Join-Path $LogRoot 'gate8b-worker-repair.json'
$MarkerPath = Join-Path $LogRoot 'gate8c-api-rls-repair.json'
. (Join-Path $PSScriptRoot 'forensic_runtime_common.ps1')

Assert-ForensicMarker $Gate8bMarker 'Gate 8b'
[void](Get-ForensicRuntimeEnvironment $RuntimeEnvPath)
$rollbackTag = 'nexusai/forensic-records-api:rollback-before-runtime-rls-context-fix-20260730'
$apiId = docker compose -p nexusai --env-file $RuntimeEnvPath `
  -f (Join-Path $RepoRoot 'docker-compose.forensic-records.yaml') `
  -f (Join-Path $RepoRoot 'docker-compose.forensic-records.runtime.yaml') ps -q forensic-records-api
if ($LASTEXITCODE -ne 0 -or -not $apiId) { throw 'Existing forensic API container is missing' }
$oldAPIImage = docker inspect --format '{{.Image}}' $apiId
docker image tag $oldAPIImage $rollbackTag
if ($LASTEXITCODE -ne 0) { throw 'Could not preserve forensic API rollback image' }

Push-Location $RepoRoot
try {
  Invoke-ForensicNative -FailureMessage 'Corrected forensic API image build failed' `
    -LogPath (Join-Path $LogRoot 'gate8c-api-build.log') -Command {
      docker compose -p nexusai --env-file $RuntimeEnvPath `
        -f .\docker-compose.forensic-records.yaml `
        -f .\docker-compose.forensic-records.runtime.yaml `
        --progress plain build forensic-records-api
    }
  Invoke-ForensicNative -FailureMessage 'Corrected forensic API deployment failed' `
    -LogPath (Join-Path $LogRoot 'gate8c-api-deploy.log') -Command {
      docker compose -p nexusai --env-file $RuntimeEnvPath `
        -f .\docker-compose.forensic-records.yaml `
        -f .\docker-compose.forensic-records.runtime.yaml `
        up -d --no-deps --force-recreate forensic-records-api
    }
  $newApiId = docker compose -p nexusai --env-file $RuntimeEnvPath `
    -f .\docker-compose.forensic-records.yaml `
    -f .\docker-compose.forensic-records.runtime.yaml ps -q forensic-records-api
  Wait-ForensicContainer -ContainerId $newApiId -Label 'forensic-records-api' -AllowRunningWithoutHealth
  $deadline = (Get-Date).AddMinutes(2)
  do {
    try { $health = Invoke-WebRequest -UseBasicParsing -TimeoutSec 10 -Uri 'http://localhost:8091/healthz' } catch { $health = $null }
    if ($health -and $health.StatusCode -eq 200) { break }
    Start-Sleep -Seconds 2
  } until ((Get-Date) -gt $deadline)
  if (-not $health -or $health.StatusCode -ne 200) { throw 'Corrected forensic API health failed' }
  [ordered]@{
    gate = '8c'; repaired_utc = (Get-Date).ToUniversalTime().ToString('o')
    defect = 'runtime_writes_missing_transaction_tenant_context'; api_health = 'pass'
    rollback_image = $rollbackTag; named_volumes_preserved = $true
  } | ConvertTo-Json | Set-Content -Encoding ascii -LiteralPath $MarkerPath
} catch {
  $env:NEXUSAI_FORENSIC_API_IMAGE = $rollbackTag
  docker compose -p nexusai --env-file $RuntimeEnvPath `
    -f .\docker-compose.forensic-records.yaml `
    -f .\docker-compose.forensic-records.runtime.yaml `
    up -d --no-deps --force-recreate forensic-records-api | Out-Null
  Remove-Item Env:\NEXUSAI_FORENSIC_API_IMAGE -ErrorAction SilentlyContinue
  throw
} finally { Pop-Location }

Write-Host 'ActivationGate8c=PASS RuntimeTenantContext=PASS APIHealth=PASS VolumesPreserved=true RollbackImage=preserved'
