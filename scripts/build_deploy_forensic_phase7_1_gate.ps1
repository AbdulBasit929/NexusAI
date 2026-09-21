param(
  [ValidatePattern('^phase7\.[12]$')][string]$PhaseLabel = 'phase7.1',
  [ValidatePattern('^phase7\.[12](?:-[a-z0-9.-]+)?$')][string]$ActivationLabel = ''
)

$ErrorActionPreference = 'Stop'
if (-not $ActivationLabel) { $ActivationLabel = $PhaseLabel }

$gateMutex = New-Object System.Threading.Mutex($false, 'Local\NexusAIForensicPhase71Gate')
if (-not $gateMutex.WaitOne(0)) {
  $gateMutex.Dispose()
  throw 'Another forensic sidecar deployment gate is already running'
}

$RepoRoot = Split-Path -Parent $PSScriptRoot
$RuntimeEnvPath = Join-Path $RepoRoot '.env.forensic-runtime.local'
$LogRoot = Join-Path $RepoRoot 'reports\runtime-activation-20260804'
$Gate9Marker = Join-Path $RepoRoot 'reports\runtime-activation-20260730\gate9-runtime-contract.json'
$MarkerPath = Join-Path $LogRoot "$ActivationLabel-sidecar-activation.json"
$minimumFreeRAMGiB = 2.0
. (Join-Path $PSScriptRoot 'forensic_runtime_common.ps1')

Assert-ForensicMarker $Gate9Marker 'Gate 9'
[void](Get-ForensicRuntimeEnvironment $RuntimeEnvPath)
New-Item -ItemType Directory -Force -Path $LogRoot | Out-Null

$sidecarFiles = @(
  (Join-Path $RepoRoot 'docker-compose.forensic-records.yaml'),
  (Join-Path $RepoRoot 'docker-compose.forensic-records.runtime.yaml')
)
$apiRollback = "nexusai/forensic-records-api:rollback-before-$ActivationLabel-20260804"
$workerRollback = "nexusai/forensic-records-worker:rollback-before-$ActivationLabel-20260804"

function Get-ComposeServiceImageID {
  param([string]$Service)
  $args = @('compose', '-p', 'nexusai', '--env-file', $RuntimeEnvPath)
  foreach ($file in $sidecarFiles) { $args += @('-f', $file) }
  $args += @('ps', '-a', '-q', $Service)
  $containerID = & docker @args
  if ($LASTEXITCODE -ne 0 -or -not $containerID) {
    throw "Compose service container is missing: $Service"
  }
  $imageID = docker inspect --format '{{.Image}}' $containerID
  if ($LASTEXITCODE -ne 0 -or -not $imageID) {
    throw "Could not inspect service image: $Service"
  }
  return [pscustomobject]@{ ContainerID = $containerID; ImageID = $imageID }
}

function Get-FreeRAMGiB {
  $os = Get-CimInstance Win32_OperatingSystem
  return [math]::Round($os.FreePhysicalMemory / 1MB, 2)
}

function Wait-ForFreeRAMGiB {
  param([double]$MinimumGiB, [int]$TimeoutSeconds = 45)
  $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
  do {
    $freeGiB = Get-FreeRAMGiB
    Write-Host "RAMGate=WAITING FreeGiB=$freeGiB RequiredGiB=$MinimumGiB"
    if ($freeGiB -ge $MinimumGiB) { return $freeGiB }
    Start-Sleep -Seconds 3
  } until ((Get-Date) -gt $deadline)
  return $freeGiB
}

function Invoke-LoggedNative {
  param([scriptblock]$Command, [string]$FailureMessage, [string]$LogPath)
  $savedPreference = $ErrorActionPreference
  $nativeExitCode = 0
  $utf8WithoutBom = New-Object System.Text.UTF8Encoding($false)
  $writer = New-Object System.IO.StreamWriter($LogPath, $false, $utf8WithoutBom)
  try {
    $ErrorActionPreference = 'Continue'
    & $Command 2>&1 | ForEach-Object {
      $line = if ($_ -is [System.Management.Automation.ErrorRecord]) { $_.Exception.Message } else { $_.ToString() }
      $writer.WriteLine($line)
      $writer.Flush()
      Write-Host $line
    }
    $nativeExitCode = $LASTEXITCODE
  } finally {
    $writer.Dispose()
    $ErrorActionPreference = $savedPreference
  }
  if ($nativeExitCode -ne 0) { throw "$FailureMessage (native exit $nativeExitCode)" }
}

function Restore-SidecarRollback {
  docker compose -p nexusai --env-file $RuntimeEnvPath `
    -f $sidecarFiles[0] -f $sidecarFiles[1] `
    up -d forensic-postgres forensic-nats | Out-Null

  $env:NEXUSAI_FORENSIC_WORKER_IMAGE = $workerRollback
  docker compose -p nexusai --env-file $RuntimeEnvPath `
    -f $sidecarFiles[0] -f $sidecarFiles[1] `
    up -d --no-deps --force-recreate forensic-records-worker | Out-Null
  Remove-Item Env:\NEXUSAI_FORENSIC_WORKER_IMAGE -ErrorAction SilentlyContinue

  $env:NEXUSAI_FORENSIC_API_IMAGE = $apiRollback
  docker compose -p nexusai --env-file $RuntimeEnvPath `
    -f $sidecarFiles[0] -f $sidecarFiles[1] `
    up -d --no-deps --force-recreate forensic-records-api | Out-Null
  Remove-Item Env:\NEXUSAI_FORENSIC_API_IMAGE -ErrorAction SilentlyContinue
}

$servicesMutated = $false
$apiBuildSeconds = 0
$workerBuildSeconds = 0
$freeRAMBeforeBuildGiB = 0

Push-Location $RepoRoot
try {
  $serverVersion = docker info --format '{{.ServerVersion}}' 2>$null
  if ($LASTEXITCODE -ne 0 -or -not $serverVersion) { throw 'Docker Desktop Linux engine is not ready' }
  Write-Host "DockerEngine=READY ServerVersion=$serverVersion"

  $beforeAPI = Get-ComposeServiceImageID 'forensic-records-api'
  $beforeWorker = Get-ComposeServiceImageID 'forensic-records-worker'
  docker image tag $beforeAPI.ImageID $apiRollback
  if ($LASTEXITCODE -ne 0) { throw "Could not preserve the $PhaseLabel API rollback image" }
  docker image tag $beforeWorker.ImageID $workerRollback
  if ($LASTEXITCODE -ne 0) { throw "Could not preserve the $PhaseLabel worker rollback image" }

  docker stop $beforeAPI.ContainerID $beforeWorker.ContainerID | Out-Null
  if ($LASTEXITCODE -ne 0) { throw "Could not stop $PhaseLabel sidecars before build" }
  $servicesMutated = $true

  $freeRAMBeforeBuildGiB = Wait-ForFreeRAMGiB -MinimumGiB $minimumFreeRAMGiB
  if ($freeRAMBeforeBuildGiB -lt $minimumFreeRAMGiB) {
    throw "$PhaseLabel sidecar rebuild requires $minimumFreeRAMGiB GiB free RAM; only $freeRAMBeforeBuildGiB GiB is free"
  }

  $started = Get-Date
  Invoke-LoggedNative -FailureMessage "$PhaseLabel forensic API build failed" `
    -LogPath (Join-Path $LogRoot "$ActivationLabel-api-build.log") -Command {
      docker compose -p nexusai --env-file $RuntimeEnvPath `
        -f $sidecarFiles[0] -f $sidecarFiles[1] `
        --progress plain build forensic-records-api
    }
  $apiBuildSeconds = [math]::Round(((Get-Date) - $started).TotalSeconds, 1)

  $started = Get-Date
  Invoke-LoggedNative -FailureMessage "$PhaseLabel forensic worker build failed" `
    -LogPath (Join-Path $LogRoot "$ActivationLabel-worker-build.log") -Command {
      docker compose -p nexusai --env-file $RuntimeEnvPath `
        -f $sidecarFiles[0] -f $sidecarFiles[1] `
        --progress plain build forensic-records-worker
    }
  $workerBuildSeconds = [math]::Round(((Get-Date) - $started).TotalSeconds, 1)

  docker compose -p nexusai --env-file $RuntimeEnvPath `
    -f $sidecarFiles[0] -f $sidecarFiles[1] `
    up -d forensic-postgres forensic-nats forensic-records-worker
  if ($LASTEXITCODE -ne 0) { throw "$PhaseLabel dependency deployment failed" }

  $worker = Get-ComposeServiceImageID 'forensic-records-worker'
  Wait-ForensicContainer -ContainerId $worker.ContainerID -Label 'forensic-records-worker' -TimeoutSeconds 180

  docker compose -p nexusai --env-file $RuntimeEnvPath `
    -f $sidecarFiles[0] -f $sidecarFiles[1] `
    up -d --no-deps --force-recreate forensic-records-api
  if ($LASTEXITCODE -ne 0) { throw "$PhaseLabel API deployment failed" }

  $api = Get-ComposeServiceImageID 'forensic-records-api'
  Wait-ForensicContainer -ContainerId $api.ContainerID -Label 'forensic-records-api' -AllowRunningWithoutHealth

  $deadline = (Get-Date).AddMinutes(3)
  do {
    try { $apiHealth = Invoke-WebRequest -UseBasicParsing -TimeoutSec 10 -Uri 'http://localhost:8091/healthz' } catch { $apiHealth = $null }
    try { $workerHealth = Invoke-WebRequest -UseBasicParsing -TimeoutSec 10 -Uri 'http://localhost:9109/metrics' } catch { $workerHealth = $null }
    try { $natsHealth = Invoke-WebRequest -UseBasicParsing -TimeoutSec 10 -Uri 'http://localhost:8222/healthz' } catch { $natsHealth = $null }
    try { $localAIHealth = Invoke-WebRequest -UseBasicParsing -TimeoutSec 10 -Uri 'http://localhost:8080/readyz' } catch { $localAIHealth = $null }
    if ($apiHealth.StatusCode -eq 200 -and $workerHealth.StatusCode -eq 200 -and $natsHealth.StatusCode -eq 200 -and $localAIHealth.StatusCode -eq 200) { break }
    Start-Sleep -Seconds 3
  } until ((Get-Date) -gt $deadline)
  if ($apiHealth.StatusCode -ne 200 -or $workerHealth.StatusCode -ne 200 -or $natsHealth.StatusCode -ne 200 -or $localAIHealth.StatusCode -ne 200) {
    throw "$PhaseLabel deployed services did not all become ready"
  }

  [ordered]@{
    phase = "$PhaseLabel-sidecars"
    activation_label = $ActivationLabel
    deployed_utc = (Get-Date).ToUniversalTime().ToString('o')
    minimum_free_ram_gib = $minimumFreeRAMGiB
    free_ram_before_build_gib = $freeRAMBeforeBuildGiB
    api_build_seconds = $apiBuildSeconds
    worker_build_seconds = $workerBuildSeconds
    api_image = $api.ImageID
    worker_image = $worker.ImageID
    localai_image_unchanged = $true
    api_health = 'pass'
    worker_health = 'pass'
    nats_health = 'pass'
    localai_ready = 'pass'
    named_volumes_preserved = $true
    rollback_images_preserved = $true
  } | ConvertTo-Json | Set-Content -Encoding ascii -LiteralPath $MarkerPath
} catch {
  if ($servicesMutated) {
    try { Restore-SidecarRollback } catch { Write-Warning "Automatic rollback encountered an error: $($_.Exception.Message)" }
  }
  throw
} finally {
  Pop-Location
  $gateMutex.ReleaseMutex()
  $gateMutex.Dispose()
}

Write-Host "SidecarActivation=PASS Phase=$PhaseLabel Activation=$ActivationLabel APIBuildSeconds=$apiBuildSeconds WorkerBuildSeconds=$workerBuildSeconds FreeRAMBeforeBuildGiB=$freeRAMBeforeBuildGiB LocalAIImage=unchanged RollbackImages=preserved VolumesPreserved=true"
