param(
  [string]$GoPath = "C:\Program Files\Go\bin\go.exe",
  [double]$MinimumFreeRAMGiB = 6.0
)

$ErrorActionPreference = "Stop"
$RepoRoot = Split-Path -Parent $PSScriptRoot
$LogRoot = Join-Path $RepoRoot "reports\runtime-activation-20260730"
New-Item -ItemType Directory -Force -Path $LogRoot | Out-Null
New-Item -ItemType Directory -Force -Path `
  (Join-Path $RepoRoot ".tmp\go-test"), `
  (Join-Path $RepoRoot ".tmp\go-cache") | Out-Null
$env:GOTMPDIR = (Resolve-Path (Join-Path $RepoRoot ".tmp\go-test")).Path
$env:GOCACHE = (Resolve-Path (Join-Path $RepoRoot ".tmp\go-cache")).Path

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

if (-not (Test-Path -LiteralPath $GoPath)) {
  throw "Go was not found at $GoPath"
}
$os = Get-CimInstance Win32_OperatingSystem
$freeGiB = [math]::Round($os.FreePhysicalMemory / 1MB, 2)
if ($freeGiB -lt $MinimumFreeRAMGiB) {
  throw "Gate 6 requires $MinimumFreeRAMGiB GiB free RAM; only $freeGiB GiB is free. Reboot or close applications, then rerun."
}

Push-Location $RepoRoot
try {
  if (-not (Test-Path .\pkg\grpc\proto\backend.pb.go)) {
    Invoke-NativeLogged `
      -LogPath (Join-Path $LogRoot "protogen-go.log") `
      -FailureMessage "Pinned protobuf generation failed" `
      -Command {
        docker run --rm `
          --mount "type=bind,source=$RepoRoot,target=/src" `
          -w /src `
          golang@sha256:1ecb7edf62a0408027bd5729dfd6b1b8766e578e8df93995b225dfd0944eb651 `
          bash -c 'apt-get update && apt-get install -y --no-install-recommends unzip=6.0-28 && make protogen-go'
      }
  }

  Invoke-NativeLogged `
    -LogPath (Join-Path $LogRoot "go-localai-forensic-focus.log") `
    -FailureMessage "Focused LocalAI forwarding test failed" `
    -Command {
      & $GoPath test .\core\http\endpoints\localai `
        -run TestLocalAIInternalEndpoints `
        '--ginkgo.focus=forensic records KB upload forwarding' `
        -count=1
    }
} finally {
  Pop-Location
}

Write-Host "ActivationGate6=PASS FreeRAMGiB=$freeGiB ProtoReady=true"
