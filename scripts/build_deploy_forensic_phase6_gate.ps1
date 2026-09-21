$ErrorActionPreference = 'Stop'

$gateMutex = New-Object System.Threading.Mutex($false, 'Local\NexusAIForensicPhase6Gate')
if (-not $gateMutex.WaitOne(0)) {
  $gateMutex.Dispose()
  throw 'Another Phase 6 rebuild gate is already running'
}

$RepoRoot = Split-Path -Parent $PSScriptRoot
$RuntimeEnvPath = Join-Path $RepoRoot '.env.forensic-runtime.local'
$LogRoot = Join-Path $RepoRoot 'reports\runtime-activation-20260731'
$Gate9Marker = Join-Path $RepoRoot 'reports\runtime-activation-20260730\gate9-runtime-contract.json'
$MarkerPath = Join-Path $LogRoot 'phase6.3-live-activation.json'
$minimumFreeRAMGiB = 6.0
. (Join-Path $PSScriptRoot 'forensic_runtime_common.ps1')

Assert-ForensicMarker $Gate9Marker 'Gate 9'
$runtimeEnvironment = Get-ForensicRuntimeEnvironment $RuntimeEnvPath
Set-ForensicAgentHistoryDatabaseEnvironment -RuntimeEnvironment $runtimeEnvironment
New-Item -ItemType Directory -Force -Path $LogRoot | Out-Null

$sidecarFiles = @(
  (Join-Path $RepoRoot 'docker-compose.forensic-records.yaml'),
  (Join-Path $RepoRoot 'docker-compose.forensic-records.runtime.yaml')
)
$localAIFiles = @(
  (Join-Path $RepoRoot 'docker-compose.yaml'),
  (Join-Path $RepoRoot 'docker-compose.forensic-runtime.localai.yaml')
)

$apiImage = 'nexusai/forensic-records-api:phase3-runtime'
$workerImage = 'nexusai/forensic-records-worker:phase3-runtime'
$localAIImage = 'nexusai/localai-forensic:phase3-runtime'
$apiRollback = 'nexusai/forensic-records-api:rollback-before-phase6.3-20260731'
$workerRollback = 'nexusai/forensic-records-worker:rollback-before-phase6.3-20260731'
$localAIRollback = 'nexusai/localai-forensic:rollback-before-phase6.3-20260731'

function Get-ComposeServiceImageID {
  param([string[]]$Files, [string]$Service)
  $args = @('compose', '-p', 'nexusai', '--env-file', $RuntimeEnvPath)
  foreach ($file in $Files) { $args += @('-f', $file) }
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
  param(
    [Parameter(Mandatory=$true)][double]$MinimumGiB,
    [int]$TimeoutSeconds = 45
  )
  $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
  do {
    $freeGiB = Get-FreeRAMGiB
    Write-Host "RAMGate=WAITING FreeGiB=$freeGiB RequiredGiB=$MinimumGiB"
    if ($freeGiB -ge $MinimumGiB) { return $freeGiB }
    Start-Sleep -Seconds 3
  } until ((Get-Date) -gt $deadline)
  return $freeGiB
}

function Assert-DockerLinuxEngineReady {
  $serverVersion = & docker info --format '{{.ServerVersion}}' 2>$null
  if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($serverVersion)) {
    throw 'Docker Desktop Linux engine is not ready. Start Docker Desktop, wait until it reports Engine running, verify `docker info`, and rerun this gate.'
  }
  Write-Host "DockerEngine=READY ServerVersion=$serverVersion"
}

function Assert-LocalAIBuildContext {
  $dockerfilePath = Join-Path $RepoRoot 'Dockerfile'
  $dockerignorePath = Join-Path $RepoRoot '.dockerignore'
  $gitPath = Join-Path $RepoRoot '.git'
  if (-not (Test-Path -LiteralPath $gitPath)) {
    throw 'LocalAI Docker build requires the repository .git metadata, but .git is missing'
  }

  $dockerfileRequiresGit = Select-String -LiteralPath $dockerfilePath `
    -Pattern '^\s*COPY\s+(?:\./)?\.git(?:/)?\s+' -Quiet
  if (-not $dockerfileRequiresGit) { return }

  $gitIgnoreRule = Get-Content -LiteralPath $dockerignorePath | Where-Object {
    $rule = $_.Trim()
    $rule -and -not $rule.StartsWith('#') -and $rule -in @('.git', '.git/', '**/.git', '**/.git/')
  } | Select-Object -First 1
  if ($gitIgnoreRule) {
    throw ".dockerignore excludes '$gitIgnoreRule' while Dockerfile copies .git; remove that active rule before rebuilding"
  }
}

function Invoke-Phase6Native {
  param(
    [Parameter(Mandatory=$true)][scriptblock]$Command,
    [Parameter(Mandatory=$true)][string]$FailureMessage,
    [Parameter(Mandatory=$true)][string]$LogPath
  )
  $savedPreference = $ErrorActionPreference
  $nativeExitCode = 0
  $utf8WithoutBom = New-Object System.Text.UTF8Encoding($false)
  $writer = New-Object System.IO.StreamWriter($LogPath, $false, $utf8WithoutBom)
  try {
    $ErrorActionPreference = 'Continue'
    & $Command 2>&1 | ForEach-Object {
      $line = if ($_ -is [System.Management.Automation.ErrorRecord]) {
        $_.Exception.Message
      } else {
        $_.ToString()
      }
      $writer.WriteLine($line)
      $writer.Flush()
      Write-Host $line
    }
    $nativeExitCode = $LASTEXITCODE
  } finally {
    $writer.Dispose()
    $ErrorActionPreference = $savedPreference
  }
  if ($nativeExitCode -ne 0) {
    throw "$FailureMessage (native exit $nativeExitCode)"
  }
}

function Stop-ContainerIfRunning {
  param([string]$ContainerID)
  $state = docker inspect --format '{{.State.Status}}' $ContainerID
  if ($LASTEXITCODE -ne 0) { throw "Could not inspect container $ContainerID" }
  if ($state -eq 'running') {
    docker stop $ContainerID | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "Could not stop container $ContainerID" }
  }
}

function Restore-Phase6Rollback {
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

  $env:NEXUSAI_LOCALAI_RUNTIME_IMAGE = $localAIRollback
  docker compose -p nexusai --env-file $RuntimeEnvPath `
    -f $localAIFiles[0] -f $localAIFiles[1] `
    up -d --no-deps --force-recreate api | Out-Null
  Remove-Item Env:\NEXUSAI_LOCALAI_RUNTIME_IMAGE -ErrorAction SilentlyContinue
}

$apiBuildSeconds = 0
$workerBuildSeconds = 0
$localAIBuildSeconds = 0
$localAIBuildAttempts = 0
$freeRAMBeforeBuildGiB = 0
$servicesMutated = $false

Push-Location $RepoRoot
try {
  # Fail before stopping services or building sidecars when the root Docker
  # context cannot satisfy the upstream LocalAI Dockerfile.
  Assert-LocalAIBuildContext
  Assert-DockerLinuxEngineReady

  $beforeAPI = Get-ComposeServiceImageID -Files $sidecarFiles -Service 'forensic-records-api'
  $beforeWorker = Get-ComposeServiceImageID -Files $sidecarFiles -Service 'forensic-records-worker'
  $beforePostgres = Get-ComposeServiceImageID -Files $sidecarFiles -Service 'forensic-postgres'
  $beforeNATS = Get-ComposeServiceImageID -Files $sidecarFiles -Service 'forensic-nats'
  $beforeLocalAI = Get-ComposeServiceImageID -Files $localAIFiles -Service 'api'

  docker image tag $beforeAPI.ImageID $apiRollback
  if ($LASTEXITCODE -ne 0) { throw 'Could not preserve the Phase 6 API rollback image' }
  docker image tag $beforeWorker.ImageID $workerRollback
  if ($LASTEXITCODE -ne 0) { throw 'Could not preserve the Phase 6 worker rollback image' }
  docker image tag $beforeLocalAI.ImageID $localAIRollback
  if ($LASTEXITCODE -ne 0) { throw 'Could not preserve the Phase 6 LocalAI rollback image' }

  $servicesMutated = $true
  Stop-ContainerIfRunning $beforeLocalAI.ContainerID
  Stop-ContainerIfRunning $beforeAPI.ContainerID
  Stop-ContainerIfRunning $beforeWorker.ContainerID
  # Neither database nor queue is needed while images compile. Stopping them
  # gives Windows/WSL a bounded opportunity to return their resident memory;
  # the deployment or rollback path starts both with their volumes intact.
  Stop-ContainerIfRunning $beforeNATS.ContainerID
  Stop-ContainerIfRunning $beforePostgres.ContainerID

  $freeRAMBeforeBuildGiB = Wait-ForFreeRAMGiB -MinimumGiB $minimumFreeRAMGiB
  if ($freeRAMBeforeBuildGiB -lt $minimumFreeRAMGiB) {
    throw "Phase 6 sequential rebuild requires $minimumFreeRAMGiB GiB free RAM; only $freeRAMBeforeBuildGiB GiB is free"
  }

  $started = Get-Date
  Invoke-Phase6Native -FailureMessage 'Phase 6 forensic API build failed' `
    -LogPath (Join-Path $LogRoot 'phase6-api-build.log') -Command {
      docker compose -p nexusai --env-file $RuntimeEnvPath `
        -f $sidecarFiles[0] -f $sidecarFiles[1] `
        --progress plain build forensic-records-api
    }
  $apiBuildSeconds = [math]::Round(((Get-Date) - $started).TotalSeconds, 1)

  $started = Get-Date
  Invoke-Phase6Native -FailureMessage 'Phase 6 forensic worker build failed' `
    -LogPath (Join-Path $LogRoot 'phase6-worker-build.log') -Command {
      docker compose -p nexusai --env-file $RuntimeEnvPath `
        -f $sidecarFiles[0] -f $sidecarFiles[1] `
        --progress plain build forensic-records-worker
    }
  $workerBuildSeconds = [math]::Round(((Get-Date) - $started).TotalSeconds, 1)

  $started = Get-Date
  $localAIBuildSucceeded = $false
  do {
    $localAIBuildAttempts++
    try {
      Invoke-Phase6Native -FailureMessage "Phase 6 LocalAI/UI build attempt $localAIBuildAttempts failed" `
        -LogPath (Join-Path $LogRoot "phase6-localai-build-attempt-$localAIBuildAttempts.log") -Command {
          docker compose -p nexusai --env-file $RuntimeEnvPath `
            -f $localAIFiles[0] -f $localAIFiles[1] `
            --progress plain build api
        }
      $localAIBuildSucceeded = $true
    } catch {
      if ($localAIBuildAttempts -ge 3) { throw }
      Write-Warning "LocalAI build attempt $localAIBuildAttempts failed; retrying retained BuildKit layers after a bounded delay"
      Start-Sleep -Seconds 10
    }
  } until ($localAIBuildSucceeded)
  $localAIBuildSeconds = [math]::Round(((Get-Date) - $started).TotalSeconds, 1)

  Invoke-Phase6Native -FailureMessage 'Phase 6 forensic dependency deployment failed' `
    -LogPath (Join-Path $LogRoot 'phase6-dependencies-deploy.log') -Command {
      docker compose -p nexusai --env-file $RuntimeEnvPath `
        -f $sidecarFiles[0] -f $sidecarFiles[1] `
        up -d forensic-postgres forensic-nats forensic-records-worker
    }
  foreach ($service in @('forensic-postgres', 'forensic-nats', 'forensic-records-worker')) {
    $dependency = Get-ComposeServiceImageID -Files $sidecarFiles -Service $service
    Wait-ForensicContainer -ContainerId $dependency.ContainerID -Label $service -TimeoutSeconds 180
  }

  Invoke-Phase6Native -FailureMessage 'Phase 6 forensic API deployment failed' `
    -LogPath (Join-Path $LogRoot 'phase6-api-deploy.log') -Command {
      docker compose -p nexusai --env-file $RuntimeEnvPath `
        -f $sidecarFiles[0] -f $sidecarFiles[1] `
        up -d --no-deps --force-recreate forensic-records-api
    }
  $newAPI = Get-ComposeServiceImageID -Files $sidecarFiles -Service 'forensic-records-api'
  Wait-ForensicContainer -ContainerId $newAPI.ContainerID -Label 'forensic-records-api' -AllowRunningWithoutHealth

  Invoke-Phase6Native -FailureMessage 'Phase 6 LocalAI/UI deployment failed' `
    -LogPath (Join-Path $LogRoot 'phase6-localai-deploy.log') -Command {
      docker compose -p nexusai --env-file $RuntimeEnvPath `
        -f $localAIFiles[0] -f $localAIFiles[1] `
        up -d --no-deps --force-recreate api
    }
  $newLocalAI = Get-ComposeServiceImageID -Files $localAIFiles -Service 'api'
  Wait-ForensicContainer -ContainerId $newLocalAI.ContainerID -Label 'LocalAI' -TimeoutSeconds 300 -AllowRunningWithoutHealth

  $deadline = (Get-Date).AddMinutes(5)
  do {
    try { $apiHealth = Invoke-WebRequest -UseBasicParsing -TimeoutSec 10 -Uri 'http://localhost:8091/healthz' } catch { $apiHealth = $null }
    try { $workerHealth = Invoke-WebRequest -UseBasicParsing -TimeoutSec 10 -Uri 'http://localhost:9109/metrics' } catch { $workerHealth = $null }
    try { $natsHealth = Invoke-WebRequest -UseBasicParsing -TimeoutSec 10 -Uri 'http://localhost:8222/healthz' } catch { $natsHealth = $null }
    try { $localAIHealth = Invoke-WebRequest -UseBasicParsing -TimeoutSec 10 -Uri 'http://localhost:8080/readyz' } catch { $localAIHealth = $null }
    if ($apiHealth.StatusCode -eq 200 -and $workerHealth.StatusCode -eq 200 -and
        $natsHealth.StatusCode -eq 200 -and $localAIHealth.StatusCode -eq 200) { break }
    Start-Sleep -Seconds 3
  } until ((Get-Date) -gt $deadline)
  if ($apiHealth.StatusCode -ne 200 -or $workerHealth.StatusCode -ne 200 -or
      $natsHealth.StatusCode -ne 200 -or $localAIHealth.StatusCode -ne 200) {
    throw 'Phase 6 deployed services did not all become ready'
  }

  [ordered]@{
    phase = '6.1+6.2+6.3'
    deployed_utc = (Get-Date).ToUniversalTime().ToString('o')
    minimum_free_ram_gib = $minimumFreeRAMGiB
    free_ram_before_build_gib = $freeRAMBeforeBuildGiB
    api_build_seconds = $apiBuildSeconds
    worker_build_seconds = $workerBuildSeconds
    localai_build_seconds = $localAIBuildSeconds
    localai_build_attempts = $localAIBuildAttempts
    api_image = $newAPI.ImageID
    worker_image = (Get-ComposeServiceImageID -Files $sidecarFiles -Service 'forensic-records-worker').ImageID
    localai_image = $newLocalAI.ImageID
    api_health = 'pass'
    worker_health = 'pass'
    nats_health = 'pass'
    localai_ready = 'pass'
    named_volumes_preserved = $true
    rollback_images_preserved = $true
  } | ConvertTo-Json | Set-Content -Encoding ascii -LiteralPath $MarkerPath
} catch {
  if ($servicesMutated) {
    try { Restore-Phase6Rollback } catch { Write-Warning "Automatic rollback encountered an error: $($_.Exception.Message)" }
  }
  throw
} finally {
  Pop-Location
  $gateMutex.ReleaseMutex()
  $gateMutex.Dispose()
}

Write-Host "Phase6Activation=PASS APIBuildSeconds=$apiBuildSeconds WorkerBuildSeconds=$workerBuildSeconds LocalAIBuildSeconds=$localAIBuildSeconds LocalAIBuildAttempts=$localAIBuildAttempts FreeRAMBeforeBuildGiB=$freeRAMBeforeBuildGiB RollbackImages=preserved VolumesPreserved=true"
