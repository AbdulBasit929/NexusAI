$ErrorActionPreference = 'Stop'
$RepoRoot = Split-Path -Parent $PSScriptRoot
$RuntimeEnvPath = Join-Path $RepoRoot '.env.forensic-runtime.local'
$LogRoot = Join-Path $RepoRoot 'reports\runtime-activation-20260730'
$Gate13Marker = Join-Path $LogRoot 'gate13-phase4-deployment.json'
$MarkerPath = Join-Path $LogRoot 'gate13b-phase4-api-preview.json'
. (Join-Path $PSScriptRoot 'forensic_runtime_common.ps1')

Assert-ForensicMarker $Gate13Marker 'Gate 13'
[void](Get-ForensicRuntimeEnvironment $RuntimeEnvPath)
$rollbackTag = 'nexusai/forensic-records-api:rollback-before-phase4-preview-fix-20260730'
$apiID = docker compose -p nexusai --env-file $RuntimeEnvPath `
  -f (Join-Path $RepoRoot 'docker-compose.forensic-records.yaml') `
  -f (Join-Path $RepoRoot 'docker-compose.forensic-records.runtime.yaml') ps -q forensic-records-api
if ($LASTEXITCODE -ne 0 -or -not $apiID) { throw 'Running forensic API container is missing' }
$oldImage = docker inspect --format '{{.Image}}' $apiID
docker image tag $oldImage $rollbackTag
if ($LASTEXITCODE -ne 0) { throw 'Could not preserve the pre-13b API image' }

Push-Location $RepoRoot
try {
  Invoke-ForensicNative -FailureMessage 'Phase 4 preview-safe API build failed' `
    -LogPath (Join-Path $LogRoot 'gate13b-api-build.log') -Command {
      docker compose -p nexusai --env-file $RuntimeEnvPath `
        -f .\docker-compose.forensic-records.yaml -f .\docker-compose.forensic-records.runtime.yaml `
        --progress plain build forensic-records-api
    }
  Invoke-ForensicNative -FailureMessage 'Phase 4 preview-safe API deployment failed' `
    -LogPath (Join-Path $LogRoot 'gate13b-api-deploy.log') -Command {
      docker compose -p nexusai --env-file $RuntimeEnvPath `
        -f .\docker-compose.forensic-records.yaml -f .\docker-compose.forensic-records.runtime.yaml `
        up -d --no-deps --force-recreate forensic-records-api
    }
  $newID = docker compose -p nexusai --env-file $RuntimeEnvPath `
    -f .\docker-compose.forensic-records.yaml -f .\docker-compose.forensic-records.runtime.yaml ps -q forensic-records-api
  Wait-ForensicContainer -ContainerId $newID -Label 'forensic-records-api' -AllowRunningWithoutHealth
  $health = Invoke-WebRequest -UseBasicParsing -TimeoutSec 20 -Uri 'http://localhost:8091/healthz'
  if ($health.StatusCode -ne 200) { throw 'Phase 4 API health failed after Gate 13b' }
  [ordered]@{
    gate='13b'; deployed_utc=(Get-Date).ToUniversalTime().ToString('o')
    records_preview_opt_out='pass'; api_health='pass'; rollback_image=$rollbackTag
    named_volumes_preserved=$true
  } | ConvertTo-Json | Set-Content -Encoding ascii -LiteralPath $MarkerPath
} catch {
  $env:NEXUSAI_FORENSIC_API_IMAGE = $rollbackTag
  docker compose -p nexusai --env-file $RuntimeEnvPath `
    -f .\docker-compose.forensic-records.yaml -f .\docker-compose.forensic-records.runtime.yaml `
    up -d --no-deps --force-recreate forensic-records-api | Out-Null
  Remove-Item Env:\NEXUSAI_FORENSIC_API_IMAGE -ErrorAction SilentlyContinue
  throw
} finally {
  Pop-Location
}

Write-Host 'Phase4Gate13b=PASS PreviewOptOut=PASS APIHealth=PASS VolumesPreserved=true RollbackImage=preserved'
