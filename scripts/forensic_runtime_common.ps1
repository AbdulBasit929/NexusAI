$ErrorActionPreference = "Stop"

function Get-ForensicRuntimeEnvironment {
  param([Parameter(Mandatory=$true)][string]$Path)
  if (-not (Test-Path -LiteralPath $Path)) {
    throw "Runtime environment is missing; complete Gate 2 first"
  }
  $values = ((Get-Content -LiteralPath $Path |
    Where-Object { $_ -and -not $_.StartsWith('#') }) -join "`n") |
    ConvertFrom-StringData
  if ($values.FORENSIC_RUNTIME_DB_PASSWORD -notmatch '^[0-9a-f]{64}$') {
    throw "Runtime DB secret has an invalid shape"
  }
  if ($values.FORENSIC_RECORDS_API_KEY -notmatch '^[0-9a-f]{64}$') {
    throw "Runtime API secret has an invalid shape"
  }
  if (-not $values.FORENSIC_RECORDS_TENANT_ID) {
    throw "Runtime tenant is missing"
  }
  return $values
}

function Set-ForensicAgentHistoryDatabaseEnvironment {
  param(
    [Parameter(Mandatory=$true)]$RuntimeEnvironment,
    [string]$ComposeProject = 'nexusai',
    [string]$ComposeService = 'api'
  )

  $configuredURL = [string]$env:NEXUSAI_AGENT_HISTORY_DATABASE_URL
  if ([string]::IsNullOrWhiteSpace($configuredURL)) {
    $configuredURL = [string]$RuntimeEnvironment.NEXUSAI_AGENT_HISTORY_DATABASE_URL
  }
  if ([string]::IsNullOrWhiteSpace($configuredURL)) {
    # An accepted deployment retains the same credential in its container
    # environment. Reuse it only in this PowerShell process so rebuild gates
    # remain restart-safe without printing or duplicating the secret on disk.
    $retainedContainer = docker ps -a `
      --filter "label=com.docker.compose.project=$ComposeProject" `
      --filter "label=com.docker.compose.service=$ComposeService" `
      --format '{{.ID}}' 2>$null | Select-Object -First 1
    if (-not [string]::IsNullOrWhiteSpace($retainedContainer)) {
      $containerLine = docker inspect --format '{{range .Config.Env}}{{println .}}{{end}}' $retainedContainer 2>$null |
        Where-Object {
          $_ -match '^(NEXUSAI_AGENT_HISTORY_DATABASE_URL|LOCALAI_AGENT_POOL_DATABASE_URL)='
        } |
        Select-Object -Last 1
      if ($containerLine) {
        $configuredURL = ($containerLine -split '=', 2)[1].Trim()
      }
    }
  }
  if ([string]::IsNullOrWhiteSpace($configuredURL) -or $configuredURL -match 'CHANGE_ME') {
    throw 'Set NEXUSAI_AGENT_HISTORY_DATABASE_URL to the retained PostgreSQL database before activation'
  }
  if ($configuredURL -notmatch '^postgres(ql)?://') {
    throw 'NEXUSAI_AGENT_HISTORY_DATABASE_URL must be a PostgreSQL URL'
  }

  $env:NEXUSAI_AGENT_HISTORY_DATABASE_URL = $configuredURL
}

function Invoke-ForensicNative {
  param(
    [Parameter(Mandatory=$true)][scriptblock]$Command,
    [Parameter(Mandatory=$true)][string]$FailureMessage,
    [string]$LogPath = ""
  )
  $savedPreference = $ErrorActionPreference
  $nativeExitCode = 0
  try {
    $ErrorActionPreference = "Continue"
    $output = & $Command 2>&1 | ForEach-Object {
      if ($_ -is [System.Management.Automation.ErrorRecord]) {
        $_.Exception.Message
      } else {
        $_
      }
    }
    $nativeExitCode = $LASTEXITCODE
    if ($LogPath) {
      $output | Tee-Object -FilePath $LogPath
    } else {
      $output
    }
  } finally {
    $ErrorActionPreference = $savedPreference
  }
  if ($nativeExitCode -ne 0) {
    throw "$FailureMessage (native exit $nativeExitCode)"
  }
}

function Assert-ForensicImage {
  param([Parameter(Mandatory=$true)][string]$Image)
  docker image inspect $Image --format '{{.Id}}' | Out-Null
  if ($LASTEXITCODE -ne 0) { throw "Required image is missing: $Image" }
}

function Wait-ForensicContainer {
  param(
    [Parameter(Mandatory=$true)][string]$ContainerId,
    [Parameter(Mandatory=$true)][string]$Label,
    [int]$TimeoutSeconds = 180,
    [switch]$AllowRunningWithoutHealth
  )
  $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
  do {
    $state = docker inspect --format '{{.State.Status}}|{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' $ContainerId
    if ($LASTEXITCODE -ne 0) { throw "Could not inspect $Label" }
    if ($state -eq 'running|healthy' -or ($AllowRunningWithoutHealth -and $state -eq 'running|none')) {
      return
    }
    if ($state -match '^(exited|dead)\|') { throw "$Label stopped before becoming ready: $state" }
    Start-Sleep -Seconds 2
  } until ((Get-Date) -gt $deadline)
  throw "$Label did not become ready in $TimeoutSeconds seconds; last state: $state"
}

function Assert-ForensicMarker {
  param([Parameter(Mandatory=$true)][string]$Path, [Parameter(Mandatory=$true)][string]$Gate)
  if (-not (Test-Path -LiteralPath $Path)) { throw "$Gate marker is missing: $Path" }
}
