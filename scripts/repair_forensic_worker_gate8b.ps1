$ErrorActionPreference = 'Stop'
$RepoRoot = Split-Path -Parent $PSScriptRoot
$RuntimeEnvPath = Join-Path $RepoRoot '.env.forensic-runtime.local'
$LogRoot = Join-Path $RepoRoot 'reports\runtime-activation-20260730'
$Gate8Marker = Join-Path $LogRoot 'gate8-deployment.json'
$MarkerPath = Join-Path $LogRoot 'gate8b-worker-repair.json'
. (Join-Path $PSScriptRoot 'forensic_runtime_common.ps1')

Assert-ForensicMarker $Gate8Marker 'Gate 8'
[void](Get-ForensicRuntimeEnvironment $RuntimeEnvPath)
$rollbackTag = 'nexusai/forensic-records-worker:rollback-before-xlsx-package-fix-20260730'
$workerId = docker compose -p nexusai --env-file $RuntimeEnvPath `
  -f (Join-Path $RepoRoot 'docker-compose.forensic-records.yaml') `
  -f (Join-Path $RepoRoot 'docker-compose.forensic-records.runtime.yaml') ps -aq forensic-records-worker
if ($LASTEXITCODE -ne 0 -or -not $workerId) { throw 'Existing worker container is missing' }
$oldWorkerImage = docker inspect --format '{{.Image}}' $workerId
if ($LASTEXITCODE -ne 0 -or -not $oldWorkerImage) { throw 'Could not inspect existing worker image' }
docker image tag $oldWorkerImage $rollbackTag
if ($LASTEXITCODE -ne 0) { throw 'Could not preserve worker rollback image' }

Push-Location $RepoRoot
try {
  Invoke-ForensicNative -FailureMessage 'Corrected worker image build failed' `
    -LogPath (Join-Path $LogRoot 'gate8b-worker-build.log') -Command {
      docker compose -p nexusai --env-file $RuntimeEnvPath `
        -f .\docker-compose.forensic-records.yaml `
        -f .\docker-compose.forensic-records.runtime.yaml `
        --progress plain build forensic-records-worker
    }
  Invoke-ForensicNative -FailureMessage 'Corrected worker import smoke failed' -Command {
    docker run --rm --entrypoint python nexusai/forensic-records-worker:phase3-runtime `
      -c 'import xlsx_reader; import worker'
  }
  Invoke-ForensicNative -FailureMessage 'Corrected worker deployment failed' `
    -LogPath (Join-Path $LogRoot 'gate8b-worker-deploy.log') -Command {
      docker compose -p nexusai --env-file $RuntimeEnvPath `
        -f .\docker-compose.forensic-records.yaml `
        -f .\docker-compose.forensic-records.runtime.yaml `
        up -d --no-deps --force-recreate forensic-records-worker
    }
  $newWorkerId = docker compose -p nexusai --env-file $RuntimeEnvPath `
    -f .\docker-compose.forensic-records.yaml `
    -f .\docker-compose.forensic-records.runtime.yaml ps -q forensic-records-worker
  if ($LASTEXITCODE -ne 0 -or -not $newWorkerId) { throw 'Corrected worker container is missing' }
  Wait-ForensicContainer -ContainerId $newWorkerId -Label 'forensic-records-worker' -TimeoutSeconds 180
  [ordered]@{
    gate = '8b'; repaired_utc = (Get-Date).ToUniversalTime().ToString('o')
    defect = 'xlsx_reader_missing_from_worker_image'; worker_health = 'pass'
    rollback_image = $rollbackTag; named_volumes_preserved = $true
  } | ConvertTo-Json | Set-Content -Encoding ascii -LiteralPath $MarkerPath
} catch {
  # Preserve the prior image tag, but do not restore it when its import smoke is
  # known to fail; doing so would hide the corrected container's startup error.
  docker run --rm --entrypoint python $rollbackTag -c 'import xlsx_reader; import worker' 2>$null
  $rollbackImportExit = $LASTEXITCODE
  if ($rollbackImportExit -eq 0) {
    $env:NEXUSAI_FORENSIC_WORKER_IMAGE = $rollbackTag
    docker compose -p nexusai --env-file $RuntimeEnvPath `
      -f .\docker-compose.forensic-records.yaml `
      -f .\docker-compose.forensic-records.runtime.yaml `
      up -d --no-deps --force-recreate forensic-records-worker | Out-Null
    Remove-Item Env:\NEXUSAI_FORENSIC_WORKER_IMAGE -ErrorAction SilentlyContinue
  }
  throw
} finally { Pop-Location }

Write-Host 'ActivationGate8b=PASS WorkerImagePackaging=PASS WorkerHealth=PASS VolumesPreserved=true RollbackImage=preserved'
