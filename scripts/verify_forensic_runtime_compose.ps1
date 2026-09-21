param()

$ErrorActionPreference = "Stop"
$RepoRoot = Split-Path -Parent $PSScriptRoot
$RuntimeEnvPath = Join-Path $RepoRoot ".env.forensic-runtime.local"

if (-not (Test-Path -LiteralPath $RuntimeEnvPath)) {
  throw "Runtime environment is missing. Complete activation Gate 2 first."
}

$runtimeEnv = ((Get-Content -LiteralPath $RuntimeEnvPath |
  Where-Object { $_ -and -not $_.StartsWith('#') }) -join "`n") |
  ConvertFrom-StringData
if ($runtimeEnv.FORENSIC_RUNTIME_DB_PASSWORD -notmatch '^[0-9a-f]{64}$') {
  throw "Runtime DB secret shape is invalid"
}
if ($runtimeEnv.FORENSIC_RECORDS_API_KEY -notmatch '^[0-9a-f]{64}$') {
  throw "Runtime API secret shape is invalid"
}
if ($runtimeEnv.FORENSIC_RECORDS_TENANT_ID -notmatch '^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$') {
  throw "Runtime tenant setting is invalid"
}

Push-Location $RepoRoot
try {
  docker compose --env-file $RuntimeEnvPath `
    -f .\docker-compose.forensic-records.yaml `
    -f .\docker-compose.forensic-records.runtime.yaml `
    config --quiet
  if ($LASTEXITCODE -ne 0) {
    throw "Forensic runtime Compose config failed"
  }

  docker compose --env-file $RuntimeEnvPath `
    -f .\docker-compose.yaml `
    -f .\docker-compose.forensic-runtime.localai.yaml `
    config --quiet
  if ($LASTEXITCODE -ne 0) {
    throw "LocalAI runtime Compose config failed"
  }
} finally {
  Pop-Location
}

Write-Host "ActivationGate5=PASS ComposeExpanded=false SecretValuesPrinted=false"
