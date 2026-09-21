param([switch]$PreflightOnly)

$ErrorActionPreference = 'Stop'

$gateMutex = New-Object System.Threading.Mutex($false, 'Local\NexusAIR8UIAPIGate')
if (-not $gateMutex.WaitOne(0)) {
  $gateMutex.Dispose()
  throw 'Another NexusAI R8 UI/API activation is already running'
}

$repoRoot = Split-Path -Parent $PSScriptRoot
$runtimeEnvPath = Join-Path $repoRoot '.env.forensic-runtime.local'
$sidecarFiles = @(
  (Join-Path $repoRoot 'docker-compose.forensic-records.yaml'),
  (Join-Path $repoRoot 'docker-compose.forensic-records.runtime.yaml')
)
$localAIFiles = @(
  (Join-Path $repoRoot 'docker-compose.yaml'),
  (Join-Path $repoRoot 'docker-compose.forensic-runtime.localai.yaml')
)
$apiImage = 'nexusai/forensic-records-api:phase3-runtime'
$localAIImage = 'nexusai/localai-forensic:phase3-runtime'
$stamp = (Get-Date).ToUniversalTime().ToString('yyyyMMddTHHmmssZ')
$apiRollback = "nexusai/forensic-records-api:rollback-before-r8-ui-api-$stamp"
$localAIRollback = "nexusai/localai-forensic:rollback-before-r8-ui-api-$stamp"
$reportRoot = Join-Path $repoRoot 'reports\runtime-activation-20260812'
$markerPath = Join-Path $reportRoot "r8-ui-api-activation-$stamp.json"
. (Join-Path $PSScriptRoot 'forensic_runtime_common.ps1')

function Invoke-R8Compose {
  param([string[]]$Files, [Parameter(Mandatory=$true)][string[]]$Arguments)
  $composeArguments = @('compose', '-p', 'nexusai', '--env-file', $runtimeEnvPath)
  foreach ($file in $Files) { $composeArguments += @('-f', $file) }
  $composeArguments += $Arguments
  & docker @composeArguments
  if ($LASTEXITCODE -ne 0) { throw "Docker Compose failed: $($Arguments -join ' ')" }
}

function Get-R8Container {
  param(
    [Parameter(Mandatory=$true)][string]$ContainerName,
    [Parameter(Mandatory=$true)][string]$Service
  )
  $containerID = docker inspect --format '{{.Id}}' $ContainerName 2>$null
  if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($containerID)) {
    throw "The $Service container is missing"
  }
  $inspection = docker inspect $containerID | ConvertFrom-Json
  if ($LASTEXITCODE -ne 0 -or $inspection.Count -ne 1) {
    throw "The $Service container cannot be inspected"
  }
  $labels = $inspection[0].Config.Labels
  $projectLabel = [string]$labels.'com.docker.compose.project'
  $serviceLabel = [string]$labels.'com.docker.compose.service'
  if ($projectLabel -ne 'nexusai' -or $serviceLabel -ne $Service) {
    throw "The $Service container has an unexpected Compose identity"
  }
  return $containerID.Trim()
}

function Restore-R8Images {
  $env:NEXUSAI_FORENSIC_API_IMAGE = $apiRollback
  try { Invoke-R8Compose -Files $sidecarFiles -Arguments @('up', '-d', '--no-deps', '--force-recreate', 'forensic-records-api') } finally { Remove-Item Env:\NEXUSAI_FORENSIC_API_IMAGE -ErrorAction SilentlyContinue }
  $env:NEXUSAI_LOCALAI_RUNTIME_IMAGE = $localAIRollback
  try { Invoke-R8Compose -Files $localAIFiles -Arguments @('up', '-d', '--no-deps', '--force-recreate', 'api') } finally { Remove-Item Env:\NEXUSAI_LOCALAI_RUNTIME_IMAGE -ErrorAction SilentlyContinue }
}

Push-Location $repoRoot
$servicesMutated = $false
try {
  $runtimeEnvironment = Get-ForensicRuntimeEnvironment $runtimeEnvPath
  Set-ForensicAgentHistoryDatabaseEnvironment -RuntimeEnvironment $runtimeEnvironment
  $dockerVersion = docker info --format '{{.ServerVersion}}'
  if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($dockerVersion)) {
    throw 'Docker Desktop Linux engine is not ready'
  }
  New-Item -ItemType Directory -Force -Path $reportRoot | Out-Null

  # Resolve both accepted services before the first image tag or build. This
  # makes a mismatched Compose identity a read-only preflight failure.
  $apiBefore = Get-R8Container -ContainerName 'nexusai-forensic-records-api-1' -Service 'forensic-records-api'
  $localAIBefore = Get-R8Container -ContainerName 'nexusai-api-1' -Service 'api'
  if ($PreflightOnly) {
    Write-Host "R8UIAPIActivationPreflight=PASS API=$($apiBefore.Substring(0,12)) LocalAI=$($localAIBefore.Substring(0,12)) Mutation=false"
    return
  }
  $apiBeforeImage = docker inspect --format '{{.Image}}' $apiBefore
  $localAIBeforeImage = docker inspect --format '{{.Image}}' $localAIBefore
  docker image tag $apiBeforeImage $apiRollback
  if ($LASTEXITCODE -ne 0) { throw 'Could not preserve the forensic API rollback image' }
  docker image tag $localAIBeforeImage $localAIRollback
  if ($LASTEXITCODE -ne 0) { throw 'Could not preserve the LocalAI/UI rollback image' }

  $apiStarted = Get-Date
  Invoke-R8Compose -Files $sidecarFiles -Arguments @('--progress', 'plain', 'build', 'forensic-records-api')
  $apiBuildSeconds = [math]::Round(((Get-Date) - $apiStarted).TotalSeconds, 1)
  $servicesMutated = $true
  Invoke-R8Compose -Files $sidecarFiles -Arguments @('up', '-d', '--no-deps', '--force-recreate', 'forensic-records-api')
  $apiContainer = Get-R8Container -ContainerName 'nexusai-forensic-records-api-1' -Service 'forensic-records-api'
  Wait-ForensicContainer -ContainerId $apiContainer -Label 'forensic-records-api' -AllowRunningWithoutHealth -TimeoutSeconds 180
  if ((Invoke-WebRequest -UseBasicParsing -TimeoutSec 20 -Uri 'http://localhost:8091/healthz').StatusCode -ne 200) {
    throw 'Forensic records API health verification failed'
  }

  $localAIStarted = Get-Date
  Invoke-R8Compose -Files $localAIFiles -Arguments @('--progress', 'plain', 'build', 'api')
  $localAIBuildSeconds = [math]::Round(((Get-Date) - $localAIStarted).TotalSeconds, 1)
  Invoke-R8Compose -Files $localAIFiles -Arguments @('up', '-d', '--no-deps', '--force-recreate', 'api')
  $localAIContainer = Get-R8Container -ContainerName 'nexusai-api-1' -Service 'api'
  Wait-ForensicContainer -ContainerId $localAIContainer -Label 'api' -TimeoutSeconds 300
  if ((Invoke-WebRequest -UseBasicParsing -TimeoutSec 30 -Uri 'http://localhost:8080/readyz').StatusCode -ne 200) {
    throw 'LocalAI readiness verification failed'
  }
  if ((Invoke-WebRequest -UseBasicParsing -TimeoutSec 30 -Uri 'http://localhost:8080/app').StatusCode -ne 200) {
    throw 'NexusAI UI verification failed'
  }

  [ordered]@{
    phase = 'R8.2+R8.3-source-ui-api'
    status = 'ui_api_live_pending_manual_acceptance'
    deployed_utc = (Get-Date).ToUniversalTime().ToString('o')
    api_build_seconds = $apiBuildSeconds
    localai_build_seconds = $localAIBuildSeconds
    services_recreated = @('forensic-records-api', 'api')
    worker_rebuilt = $false
    models_changed = $false
    profiles_changed = $false
    named_volumes_preserved = $true
    api_rollback_image = $apiRollback
    localai_rollback_image = $localAIRollback
  } | ConvertTo-Json | Set-Content -Encoding ascii -LiteralPath $markerPath
  Write-Host "R8UIAPIActivation=PASS APIBuildSeconds=$apiBuildSeconds LocalAIBuildSeconds=$localAIBuildSeconds WorkerRebuilt=false ModelsChanged=false ProfilesChanged=false VolumesPreserved=true RollbackImages=preserved"
} catch {
  if ($servicesMutated) {
    try { Restore-R8Images } catch { Write-Warning "Automatic rollback encountered an error: $($_.Exception.Message)" }
  }
  throw
} finally {
  Remove-Item Env:\NEXUSAI_AGENT_HISTORY_DATABASE_URL -ErrorAction SilentlyContinue
  Pop-Location
  $gateMutex.ReleaseMutex()
  $gateMutex.Dispose()
}
