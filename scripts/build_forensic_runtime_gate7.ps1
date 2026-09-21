param(
  [double]$MinimumFreeRAMGiB = 6.0
)

$ErrorActionPreference = "Stop"
$RepoRoot = Split-Path -Parent $PSScriptRoot
$RuntimeEnvPath = Join-Path $RepoRoot ".env.forensic-runtime.local"
$LogRoot = Join-Path $RepoRoot "reports\runtime-activation-20260730"
$Gate6Log = Join-Path $LogRoot "go-localai-forensic-focus.log"
New-Item -ItemType Directory -Force -Path $LogRoot | Out-Null

function Invoke-NativeLogged {
  param(
    [Parameter(Mandatory=$true)][scriptblock]$Command,
    [Parameter(Mandatory=$true)][string]$LogPath,
    [Parameter(Mandatory=$true)][string]$FailureMessage
  )
  $savedPreference = $ErrorActionPreference
  $nativeExitCode = 0
  try {
    $ErrorActionPreference = "Continue"
    & $Command 2>&1 | ForEach-Object {
      if ($_ -is [System.Management.Automation.ErrorRecord]) {
        $_.Exception.Message
      } else {
        $_
      }
    } | Tee-Object -FilePath $LogPath
    $nativeExitCode = $LASTEXITCODE
  } finally {
    $ErrorActionPreference = $savedPreference
  }
  if ($nativeExitCode -ne 0) {
    throw "$FailureMessage (native exit $nativeExitCode)"
  }
}

if (-not (Test-Path -LiteralPath $RuntimeEnvPath)) {
  throw "Runtime environment is missing; Gate 2 is not complete"
}
if (-not (Test-Path -LiteralPath $Gate6Log) -or
    -not (Select-String -LiteralPath $Gate6Log -Pattern '^ok\s+github.com/mudler/LocalAI/core/http/endpoints/localai' -Quiet)) {
  throw "Passing Gate 6 LocalAI focus log is missing"
}
$os = Get-CimInstance Win32_OperatingSystem
$freeGiB = [math]::Round($os.FreePhysicalMemory / 1MB, 2)
if ($freeGiB -lt $MinimumFreeRAMGiB) {
  throw "Gate 7 requires $MinimumFreeRAMGiB GiB free RAM; only $freeGiB GiB is free."
}

Push-Location $RepoRoot
try {
  $rollbackStamp = (Get-Date).ToUniversalTime().ToString('yyyyMMddTHHmmssZ').ToLowerInvariant()
  $rollback = @{}
  foreach ($service in @('forensic-records-api','forensic-records-worker')) {
    $id = docker compose -f .\docker-compose.forensic-records.yaml ps -aq $service
    if ($LASTEXITCODE -ne 0) { throw "Could not inspect $service container" }
    if ($id) {
      $imageId = docker inspect --format '{{.Image}}' $id
      if ($LASTEXITCODE -ne 0) { throw "Could not inspect $service image" }
      $tag = "nexusai/$service`:rollback-$rollbackStamp"
      docker image tag $imageId $tag
      if ($LASTEXITCODE -ne 0) { throw "Could not tag $service rollback image" }
      $rollback[$service] = $tag
    }
  }
  # This laptop's retained LocalAI container uses the explicit historical
  # Compose project label `nexusai`; implicit discovery can omit it even from the
  # same workspace. Pin the project identity for rollback and later deployment.
  $localAIId = docker compose -p nexusai -f .\docker-compose.yaml ps -aq api
  if ($LASTEXITCODE -ne 0) { throw "Could not inspect LocalAI container" }
  if (-not $localAIId) {
    $localAIId = @(docker ps -q `
      --filter "label=com.docker.compose.project=nexusai" `
      --filter "label=com.docker.compose.service=api" `
      --filter "status=running")
    if ($LASTEXITCODE -ne 0) { throw "Could not discover running LocalAI container by labels" }
    if (@($localAIId).Count -gt 1) { throw "Multiple running LocalAI containers match the retained project" }
  }
  if ($localAIId) {
    $localAIState = docker inspect --format '{{.State.Status}}|{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' $localAIId
    if ($LASTEXITCODE -ne 0 -or $localAIState -ne 'running|healthy') {
      throw "Existing LocalAI rollback source is not running and healthy: $localAIState"
    }
    $imageId = docker inspect --format '{{.Image}}' $localAIId
    if ($LASTEXITCODE -ne 0) { throw "Could not inspect LocalAI image" }
    $tag = "nexusai/localai`:rollback-$rollbackStamp"
    docker image tag $imageId $tag
    if ($LASTEXITCODE -ne 0) { throw "Could not tag LocalAI rollback image" }
    $rollback['localai'] = $tag
  }
  if (-not $rollback.ContainsKey('localai')) {
    throw "No existing LocalAI container/image was found to rollback-tag; stop before rebuilding"
  }
  $rollback.GetEnumerator() | Sort-Object Name |
    ForEach-Object { "$($_.Name)=$($_.Value)" } |
    Set-Content -Encoding ascii (Join-Path $LogRoot "rollback-images.txt")

  $sidecarStart = Get-Date
  Invoke-NativeLogged `
    -LogPath (Join-Path $LogRoot "build-forensic-sidecars.log") `
    -FailureMessage "Forensic sidecar build failed" `
    -Command {
      docker compose -p nexusai --env-file $RuntimeEnvPath `
        -f .\docker-compose.forensic-records.yaml `
        -f .\docker-compose.forensic-records.runtime.yaml `
        --progress plain build forensic-records-api forensic-records-worker
    }
  $sidecarElapsed = [math]::Round(((Get-Date) - $sidecarStart).TotalSeconds, 1)

  $localAIStart = Get-Date
  Invoke-NativeLogged `
    -LogPath (Join-Path $LogRoot "build-localai.log") `
    -FailureMessage "LocalAI build failed" `
    -Command {
      docker compose -p nexusai --env-file $RuntimeEnvPath `
        -f .\docker-compose.yaml `
        -f .\docker-compose.forensic-runtime.localai.yaml `
        --progress plain build api
    }
  $localAIElapsed = [math]::Round(((Get-Date) - $localAIStart).TotalSeconds, 1)
} finally {
  Pop-Location
}

Write-Host "ActivationGate7=PASS SidecarBuildSeconds=$sidecarElapsed LocalAIBuildSeconds=$localAIElapsed RollbackTags=$($rollback.Count)"
