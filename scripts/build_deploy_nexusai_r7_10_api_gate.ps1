$ErrorActionPreference = 'Stop'

$gateMutex = New-Object System.Threading.Mutex($false, 'Local\NexusAIR710APIGate')
if (-not $gateMutex.WaitOne(0)) {
  $gateMutex.Dispose()
  throw 'Another NexusAI R7.10 API activation is already running'
}

$repoRoot = Split-Path -Parent $PSScriptRoot
$runtimeEnvPath = Join-Path $repoRoot '.env.forensic-runtime.local'
$composeFiles = @(
  (Join-Path $repoRoot 'docker-compose.forensic-records.yaml'),
  (Join-Path $repoRoot 'docker-compose.forensic-records.runtime.yaml')
)
$apiImage = 'nexusai/forensic-records-api:phase3-runtime'
$rollbackImage = 'nexusai/forensic-records-api:rollback-before-r7.10-20260812'
$reportRoot = Join-Path $repoRoot 'reports\runtime-activation-20260812'
$markerPath = Join-Path $reportRoot 'r7.10-api-activation.json'
. (Join-Path $PSScriptRoot 'forensic_runtime_common.ps1')

function Invoke-R710Compose {
  param([Parameter(Mandatory=$true)][string[]]$Arguments)
  $composeArguments = @('compose', '-p', 'nexusai', '--env-file', $runtimeEnvPath)
  foreach ($file in $composeFiles) { $composeArguments += @('-f', $file) }
  $composeArguments += $Arguments
  & docker @composeArguments
  if ($LASTEXITCODE -ne 0) { throw "Docker Compose failed: $($Arguments -join ' ')" }
}

function Get-R710APIContainer {
  $composeArguments = @('compose', '-p', 'nexusai', '--env-file', $runtimeEnvPath)
  foreach ($file in $composeFiles) { $composeArguments += @('-f', $file) }
  $composeArguments += @('ps', '-a', '-q', 'forensic-records-api')
  $containerID = & docker @composeArguments
  if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($containerID)) {
    throw 'The forensic records API container is missing'
  }
  return $containerID.Trim()
}

Push-Location $repoRoot
try {
  [void](Get-ForensicRuntimeEnvironment $runtimeEnvPath)
  $dockerVersion = docker info --format '{{.ServerVersion}}'
  if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($dockerVersion)) {
    throw 'Docker Desktop Linux engine is not ready'
  }
  New-Item -ItemType Directory -Force -Path $reportRoot | Out-Null

  $beforeContainer = Get-R710APIContainer
  $beforeImageID = docker inspect --format '{{.Image}}' $beforeContainer
  if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($beforeImageID)) {
    throw 'Could not inspect the current forensic API image'
  }
  docker image tag $beforeImageID $rollbackImage
  if ($LASTEXITCODE -ne 0) { throw 'Could not preserve the R7.10 API rollback image' }

  $buildStarted = Get-Date
  try {
    Invoke-R710Compose @('--progress', 'plain', 'build', 'forensic-records-api')
    $buildSeconds = [math]::Round(((Get-Date) - $buildStarted).TotalSeconds, 1)
    Invoke-R710Compose @('up', '-d', '--no-deps', '--force-recreate', 'forensic-records-api')
    $newContainer = Get-R710APIContainer
    Wait-ForensicContainer -ContainerId $newContainer -Label 'forensic-records-api' -AllowRunningWithoutHealth -TimeoutSeconds 180
    $health = Invoke-WebRequest -UseBasicParsing -TimeoutSec 20 -Uri 'http://localhost:8091/healthz'
    if ($health.StatusCode -ne 200) { throw 'Forensic records API health verification failed' }
  } catch {
    docker image tag $rollbackImage $apiImage | Out-Null
    if ($LASTEXITCODE -eq 0) {
      try { Invoke-R710Compose @('up', '-d', '--no-deps', '--force-recreate', 'forensic-records-api') } catch {}
    }
    throw
  }

  $newImageID = docker inspect --format '{{.Image}}' $newContainer
  [ordered]@{
    phase = 'R7.10'
    status = 'api_live_accepted'
    deployed_utc = (Get-Date).ToUniversalTime().ToString('o')
    api_build_seconds = $buildSeconds
    prior_image_id = $beforeImageID
    api_image_id = $newImageID
    api_health = 'pass'
    services_recreated = @('forensic-records-api')
    profiles_changed = $false
    named_volumes_preserved = $true
    rollback_image = $rollbackImage
  } | ConvertTo-Json | Set-Content -Encoding ascii -LiteralPath $markerPath
  Write-Host "R7.10APIActivation=PASS BuildSeconds=$buildSeconds ServicesRecreated=forensic-records-api ProfilesChanged=false VolumesPreserved=true RollbackImage=preserved"
} finally {
  Pop-Location
  $gateMutex.ReleaseMutex()
  $gateMutex.Dispose()
}
