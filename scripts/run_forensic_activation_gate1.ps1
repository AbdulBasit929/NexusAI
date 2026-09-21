param(
  [string]$PythonPath = "C:\Users\sheik\AppData\Local\Programs\Python\Python313\python.exe",
  [string]$GoPath = "C:\Program Files\Go\bin\go.exe",
  [string]$LogRoot = "",
  [switch]$SkipGo
)

$ErrorActionPreference = "Stop"
$RepoRoot = Split-Path -Parent $PSScriptRoot
if (-not $LogRoot) {
  $LogRoot = Join-Path $RepoRoot "reports\runtime-activation-20260730"
}
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
    # Windows PowerShell 5 wraps merged native stderr as NativeCommandError.
    # Expected diagnostic stderr is not a failure; the process exit code is.
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

if (-not (Test-Path -LiteralPath $PythonPath)) {
  throw "Python was not found at $PythonPath"
}
if (-not $SkipGo -and -not (Test-Path -LiteralPath $GoPath)) {
  throw "Go was not found at $GoPath"
}

Push-Location $RepoRoot
try {
  Invoke-NativeLogged `
    -LogPath (Join-Path $LogRoot "python-forensic-tests.log") `
    -FailureMessage "Python forensic tests failed" `
    -Command {
      & $PythonPath -m unittest `
        ingestion.forensic_records.tests.test_worker `
        ingestion.forensic_records.tests.test_phase2_adapters `
        ingestion.forensic_records.tests.test_phase4_xlsx `
        ingestion.forensic_records.tests.test_phase4_structured
    }

  if (-not $SkipGo) {
    Invoke-NativeLogged `
      -LogPath (Join-Path $LogRoot "go-forensic-api-tests.log") `
      -FailureMessage "Go forensic API tests failed" `
      -Command { & $GoPath test .\api\forensic_records -count=1 }
  } else {
    Write-Host "Go rerun skipped by operator; preserve the prior passing Go log."
  }

  git diff --check
  if ($LASTEXITCODE -ne 0) {
    throw "Whitespace validation failed"
  }
} finally {
  Pop-Location
}

Write-Host "ActivationGate1=PASS"
