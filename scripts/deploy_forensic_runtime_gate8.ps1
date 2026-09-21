$ErrorActionPreference = 'Stop'
$RepoRoot = Split-Path -Parent $PSScriptRoot
$RuntimeEnvPath = Join-Path $RepoRoot '.env.forensic-runtime.local'
$LogRoot = Join-Path $RepoRoot 'reports\runtime-activation-20260730'
$Gate4Marker = Join-Path $LogRoot 'gate4-runtime-db.json'
$MarkerPath = Join-Path $LogRoot 'gate8-deployment.json'
. (Join-Path $PSScriptRoot 'forensic_runtime_common.ps1')

Assert-ForensicMarker $Gate4Marker 'Gate 4'
[void](Get-ForensicRuntimeEnvironment $RuntimeEnvPath)
foreach ($image in @(
  'nexusai/forensic-records-api:phase3-runtime',
  'nexusai/forensic-records-worker:phase3-runtime',
  'nexusai/localai-forensic:phase3-runtime'
)) { Assert-ForensicImage $image }
Invoke-ForensicNative -FailureMessage 'Worker image import smoke failed' -Command {
  docker run --rm --entrypoint python nexusai/forensic-records-worker:phase3-runtime `
    -c 'import xlsx_reader; import worker'
}

$oldLocalAIId = $null
$oldLocalAIOriginalName = $null
$oldLocalAIRollbackName = $null
$newLocalAIId = $null
$sidecarIds = @()
$cutoverStamp = (Get-Date).ToUniversalTime().ToString('yyyyMMddTHHmmssZ').ToLowerInvariant()

Push-Location $RepoRoot
try {
  $oldMatches = @(docker ps -q `
    --filter 'label=com.docker.compose.project=nexusai' `
    --filter 'label=com.docker.compose.service=api' `
    --filter 'status=running')
  if ($LASTEXITCODE -ne 0) { throw 'Could not discover the retained LocalAI container' }
  if ($oldMatches.Count -ne 1) { throw "Expected one running retained LocalAI container; found $($oldMatches.Count)" }
  $oldLocalAIId = $oldMatches[0]
  $oldLocalAIOriginalName = (docker inspect --format '{{.Name}}' $oldLocalAIId).TrimStart('/')
  $oldImage = docker inspect --format '{{.Image}}' $oldLocalAIId
  $rollbackValues = ((Get-Content -LiteralPath (Join-Path $LogRoot 'rollback-images.txt')) -join "`n") | ConvertFrom-StringData
  $rollbackImageId = docker image inspect $rollbackValues.localai --format '{{.Id}}'
  if ($LASTEXITCODE -ne 0 -or $oldImage -ne $rollbackImageId) {
    throw 'Retained LocalAI no longer matches the verified Gate 7 rollback tag'
  }

  $oldLocalAIRollbackName = "$oldLocalAIOriginalName-pre-phase3-$cutoverStamp"
  Invoke-ForensicNative -FailureMessage 'Could not stop retained LocalAI for cutover' -Command { docker stop $oldLocalAIId }
  Invoke-ForensicNative -FailureMessage 'Could not preserve retained LocalAI container name' -Command { docker rename $oldLocalAIId $oldLocalAIRollbackName }

  Invoke-ForensicNative -FailureMessage 'Forensic sidecar deployment failed' -LogPath (Join-Path $LogRoot 'gate8-sidecar-deploy.log') -Command {
    docker compose -p nexusai --env-file $RuntimeEnvPath `
      -f .\docker-compose.forensic-records.yaml `
      -f .\docker-compose.forensic-records.runtime.yaml `
      up -d --force-recreate forensic-records-api forensic-records-worker
  }
  foreach ($service in @('forensic-records-api','forensic-records-worker')) {
    $id = docker compose -p nexusai --env-file $RuntimeEnvPath `
      -f .\docker-compose.forensic-records.yaml -f .\docker-compose.forensic-records.runtime.yaml ps -q $service
    if ($LASTEXITCODE -ne 0 -or -not $id) { throw "$service was not created" }
    $sidecarIds += $id
    if ($service -eq 'forensic-records-worker') {
      Wait-ForensicContainer -ContainerId $id -Label $service
    } else {
      Wait-ForensicContainer -ContainerId $id -Label $service -AllowRunningWithoutHealth
    }
  }

  Invoke-ForensicNative -FailureMessage 'LocalAI deployment failed' -LogPath (Join-Path $LogRoot 'gate8-localai-deploy.log') -Command {
    docker compose -p nexusai --env-file $RuntimeEnvPath `
      -f .\docker-compose.yaml -f .\docker-compose.forensic-runtime.localai.yaml `
      up -d --force-recreate api
  }
  $newMatches = @(docker ps -q `
    --filter 'label=com.docker.compose.project=nexusai' `
    --filter 'label=com.docker.compose.service=api' `
    --filter 'status=running')
  if ($LASTEXITCODE -ne 0 -or $newMatches.Count -ne 1) { throw 'New LocalAI container discovery was not unique' }
  $newLocalAIId = $newMatches[0]
  if ($newLocalAIId -eq $oldLocalAIId) { throw 'LocalAI cutover did not create the rebuilt container' }
  $newImage = docker inspect --format '{{.Image}}' $newLocalAIId
  $expectedImage = docker image inspect nexusai/localai-forensic:phase3-runtime --format '{{.Id}}'
  if ($LASTEXITCODE -ne 0 -or $newImage -ne $expectedImage) { throw 'Running LocalAI is not the Gate 7 image' }
  Wait-ForensicContainer -ContainerId $newLocalAIId -Label 'LocalAI' -TimeoutSeconds 240 -AllowRunningWithoutHealth

  $deadline = (Get-Date).AddMinutes(4)
  do {
    try { $ready = Invoke-WebRequest -UseBasicParsing -TimeoutSec 10 -Uri 'http://localhost:8080/readyz' } catch { $ready = $null }
    if ($ready -and $ready.StatusCode -eq 200) { break }
    Start-Sleep -Seconds 3
  } until ((Get-Date) -gt $deadline)
  if (-not $ready -or $ready.StatusCode -ne 200) { throw 'Rebuilt LocalAI did not pass /readyz' }

  [ordered]@{
    gate = 8; deployed_utc = (Get-Date).ToUniversalTime().ToString('o')
    localai_rollback_container = $oldLocalAIRollbackName
    localai_image = 'nexusai/localai-forensic:phase3-runtime'
    forensic_api_image = 'nexusai/forensic-records-api:phase3-runtime'
    forensic_worker_image = 'nexusai/forensic-records-worker:phase3-runtime'
    named_volumes_preserved = $true
  } | ConvertTo-Json | Set-Content -Encoding ascii -LiteralPath $MarkerPath
} catch {
  foreach ($id in $sidecarIds) { if ($id) { docker stop $id | Out-Null } }
  if ($newLocalAIId) { docker stop $newLocalAIId | Out-Null }
  if ($oldLocalAIId) {
    $oldState = docker inspect --format '{{.State.Status}}' $oldLocalAIId 2>$null
    if ($LASTEXITCODE -eq 0 -and $oldState -ne 'running') { docker start $oldLocalAIId | Out-Null }
  }
  throw
} finally { Pop-Location }

Write-Host 'ActivationGate8=PASS ForensicAPI=running Worker=running LocalAI=ready VolumesPreserved=true RollbackContainer=preserved'
