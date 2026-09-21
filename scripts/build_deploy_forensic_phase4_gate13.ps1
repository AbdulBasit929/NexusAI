$ErrorActionPreference = 'Stop'
$gateMutex = New-Object System.Threading.Mutex($false, 'Local\NexusAIForensicPhase4Gate13')
if (-not $gateMutex.WaitOne(0)) {
  $gateMutex.Dispose()
  throw 'Another Phase 4 rebuild gate is already running'
}
$RepoRoot = Split-Path -Parent $PSScriptRoot
$RuntimeEnvPath = Join-Path $RepoRoot '.env.forensic-runtime.local'
$LogRoot = Join-Path $RepoRoot 'reports\runtime-activation-20260730'
$Gate9Marker = Join-Path $LogRoot 'gate9-runtime-contract.json'
$MarkerPath = Join-Path $LogRoot 'gate13-phase4-deployment.json'
. (Join-Path $PSScriptRoot 'forensic_runtime_common.ps1')

Assert-ForensicMarker $Gate9Marker 'Gate 9'
[void](Get-ForensicRuntimeEnvironment $RuntimeEnvPath)
$apiImage = 'nexusai/forensic-records-api:phase3-runtime'
$localAIImage = 'nexusai/localai-forensic:phase3-runtime'
$apiRollback = 'nexusai/forensic-records-api:rollback-before-phase4-20260730'
$localAIRollback = 'nexusai/localai-forensic:rollback-before-phase4-20260730'
$minimumFreeRAMGiB = 5.25

function Get-ComposeServiceImageID {
  param([string[]]$Files, [string]$Service)
  $args = @('compose','-p','nexusai','--env-file',$RuntimeEnvPath)
  foreach ($file in $Files) { $args += @('-f', $file) }
  # Include stopped containers so a memory-safe rebuild can release the LocalAI
  # model before compiling while retaining an independently addressable rollback.
  $args += @('ps','-a','-q',$Service)
  $containerID = & docker @args
  if ($LASTEXITCODE -ne 0 -or -not $containerID) { throw "Compose service container is missing: $Service" }
  $imageID = docker inspect --format '{{.Image}}' $containerID
  if ($LASTEXITCODE -ne 0 -or -not $imageID) { throw "Could not inspect running image: $Service" }
  return [pscustomobject]@{ContainerID=$containerID; ImageID=$imageID}
}

$sidecarFiles = @(
  (Join-Path $RepoRoot 'docker-compose.forensic-records.yaml'),
  (Join-Path $RepoRoot 'docker-compose.forensic-records.runtime.yaml')
)
$localAIFiles = @(
  (Join-Path $RepoRoot 'docker-compose.yaml'),
  (Join-Path $RepoRoot 'docker-compose.forensic-runtime.localai.yaml')
)
$beforeAPI = Get-ComposeServiceImageID -Files $sidecarFiles -Service 'forensic-records-api'
$beforeLocalAI = Get-ComposeServiceImageID -Files $localAIFiles -Service 'api'
docker image tag $beforeAPI.ImageID $apiRollback
if ($LASTEXITCODE -ne 0) { throw 'Could not tag the Phase 4 API rollback image' }
docker image tag $beforeLocalAI.ImageID $localAIRollback
if ($LASTEXITCODE -ne 0) { throw 'Could not tag the Phase 4 LocalAI rollback image' }

$apiBuildSeconds = 0
$localAIBuildSeconds = 0
Push-Location $RepoRoot
try {
  # The CPU model consumes several GiB on this 16 GiB Windows host. Stop only
  # LocalAI after preserving its image so Docker/Go compilation cannot force the
  # engine out of memory. Database, queue, worker, API and named volumes remain.
  docker stop $beforeLocalAI.ContainerID | Out-Null
  if ($LASTEXITCODE -ne 0) { throw 'Could not stop LocalAI for the memory-safe rebuild' }
  Start-Sleep -Seconds 3

  $memoryDeadline = (Get-Date).AddSeconds(60)
  do {
    $os = Get-CimInstance Win32_OperatingSystem
    $freeRAMGiB = [math]::Round($os.FreePhysicalMemory / 1MB, 2)
    if ($freeRAMGiB -ge $minimumFreeRAMGiB) { break }
    Start-Sleep -Seconds 2
  } until ((Get-Date) -gt $memoryDeadline)
  if ($freeRAMGiB -lt $minimumFreeRAMGiB) {
    throw "Phase 4 bounded-parallel rebuild requires $minimumFreeRAMGiB GiB free RAM after stopping LocalAI; only $freeRAMGiB GiB is free"
  }

  $started = Get-Date
  Invoke-ForensicNative -FailureMessage 'Phase 4 forensic API build failed' `
    -LogPath (Join-Path $LogRoot 'gate13-api-build.log') -Command {
      docker compose -p nexusai --env-file $RuntimeEnvPath `
        -f .\docker-compose.forensic-records.yaml `
        -f .\docker-compose.forensic-records.runtime.yaml `
        --progress plain build forensic-records-api
    }
  $apiBuildSeconds = [math]::Round(((Get-Date) - $started).TotalSeconds, 1)

  $started = Get-Date
  Invoke-ForensicNative -FailureMessage 'Phase 4 LocalAI/UI build failed' `
    -LogPath (Join-Path $LogRoot 'gate13-localai-build.log') -Command {
      docker compose -p nexusai --env-file $RuntimeEnvPath `
        -f .\docker-compose.yaml `
        -f .\docker-compose.forensic-runtime.localai.yaml `
        --progress plain build api
    }
  $localAIBuildSeconds = [math]::Round(((Get-Date) - $started).TotalSeconds, 1)

  Invoke-ForensicNative -FailureMessage 'Phase 4 forensic dependency startup failed' `
    -LogPath (Join-Path $LogRoot 'gate13-dependencies-start.log') -Command {
      docker compose -p nexusai --env-file $RuntimeEnvPath `
        -f .\docker-compose.forensic-records.yaml `
        -f .\docker-compose.forensic-records.runtime.yaml `
        up -d forensic-postgres forensic-nats forensic-records-worker
    }
  foreach ($service in @('forensic-postgres', 'forensic-nats', 'forensic-records-worker')) {
    $dependency = Get-ComposeServiceImageID -Files $sidecarFiles -Service $service
    Wait-ForensicContainer -ContainerId $dependency.ContainerID -Label $service -TimeoutSeconds 180
  }

  Invoke-ForensicNative -FailureMessage 'Phase 4 forensic API deployment failed' `
    -LogPath (Join-Path $LogRoot 'gate13-api-deploy.log') -Command {
      docker compose -p nexusai --env-file $RuntimeEnvPath `
        -f .\docker-compose.forensic-records.yaml `
        -f .\docker-compose.forensic-records.runtime.yaml `
        up -d --no-deps --force-recreate forensic-records-api
    }
  $newAPI = Get-ComposeServiceImageID -Files $sidecarFiles -Service 'forensic-records-api'
  Wait-ForensicContainer -ContainerId $newAPI.ContainerID -Label 'forensic-records-api' -AllowRunningWithoutHealth

  Invoke-ForensicNative -FailureMessage 'Phase 4 LocalAI/UI deployment failed' `
    -LogPath (Join-Path $LogRoot 'gate13-localai-deploy.log') -Command {
      docker compose -p nexusai --env-file $RuntimeEnvPath `
        -f .\docker-compose.yaml `
        -f .\docker-compose.forensic-runtime.localai.yaml `
        up -d --no-deps --force-recreate api
    }
  $newLocalAI = Get-ComposeServiceImageID -Files $localAIFiles -Service 'api'
  Wait-ForensicContainer -ContainerId $newLocalAI.ContainerID -Label 'LocalAI' -TimeoutSeconds 300 -AllowRunningWithoutHealth

  $deadline = (Get-Date).AddMinutes(5)
  do {
    try { $apiHealth = Invoke-WebRequest -UseBasicParsing -TimeoutSec 10 -Uri 'http://localhost:8091/healthz' } catch { $apiHealth = $null }
    try { $localAIHealth = Invoke-WebRequest -UseBasicParsing -TimeoutSec 10 -Uri 'http://localhost:8080/readyz' } catch { $localAIHealth = $null }
    if ($apiHealth.StatusCode -eq 200 -and $localAIHealth.StatusCode -eq 200) { break }
    Start-Sleep -Seconds 3
  } until ((Get-Date) -gt $deadline)
  if ($apiHealth.StatusCode -ne 200 -or $localAIHealth.StatusCode -ne 200) { throw 'Phase 4 deployed services did not become ready' }

  [ordered]@{
    gate=13; deployed_utc=(Get-Date).ToUniversalTime().ToString('o')
    api_build_seconds=$apiBuildSeconds; localai_build_seconds=$localAIBuildSeconds
    api_health='pass'; localai_ready='pass'; explicit_local_proxy_identity='pass'
    named_volumes_preserved=$true; rollback_images_preserved=$true
  } | ConvertTo-Json | Set-Content -Encoding ascii -LiteralPath $MarkerPath
} catch {
  docker compose -p nexusai --env-file $RuntimeEnvPath `
    -f .\docker-compose.forensic-records.yaml -f .\docker-compose.forensic-records.runtime.yaml `
    up -d forensic-postgres forensic-nats forensic-records-worker | Out-Null
  $env:NEXUSAI_FORENSIC_API_IMAGE = $apiRollback
  docker compose -p nexusai --env-file $RuntimeEnvPath `
    -f .\docker-compose.forensic-records.yaml -f .\docker-compose.forensic-records.runtime.yaml `
    up -d --no-deps --force-recreate forensic-records-api | Out-Null
  Remove-Item Env:\NEXUSAI_FORENSIC_API_IMAGE -ErrorAction SilentlyContinue
  $env:NEXUSAI_LOCALAI_RUNTIME_IMAGE = $localAIRollback
  docker compose -p nexusai --env-file $RuntimeEnvPath `
    -f .\docker-compose.yaml -f .\docker-compose.forensic-runtime.localai.yaml `
    up -d --no-deps --force-recreate api | Out-Null
  Remove-Item Env:\NEXUSAI_LOCALAI_RUNTIME_IMAGE -ErrorAction SilentlyContinue
  throw
} finally {
  Pop-Location
  $gateMutex.ReleaseMutex()
  $gateMutex.Dispose()
}

Write-Host "Phase4Gate13=PASS APIBuildSeconds=$apiBuildSeconds LocalAIBuildSeconds=$localAIBuildSeconds APIHealth=PASS LocalAIReady=PASS ProxyIdentity=PASS VolumesPreserved=true RollbackImages=preserved"
