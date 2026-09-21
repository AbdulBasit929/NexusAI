param(
  [string]$TenantId = "default"
)

$ErrorActionPreference = "Stop"
$RepoRoot = Split-Path -Parent $PSScriptRoot
$TemplatePath = Join-Path $RepoRoot ".env.forensic-runtime.example"
$RuntimeEnvPath = Join-Path $RepoRoot ".env.forensic-runtime.local"

if (-not (Test-Path -LiteralPath $TemplatePath)) {
  throw "Runtime environment template is missing: $TemplatePath"
}
if (Test-Path -LiteralPath $RuntimeEnvPath) {
  throw "Runtime environment already exists. This script will not overwrite it: $RuntimeEnvPath"
}
if ($TenantId -notmatch '^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$') {
  throw "TenantId contains unsupported characters"
}

function New-CryptographicHexSecret {
  $bytes = New-Object byte[] 32
  $rng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
  try {
    $rng.GetBytes($bytes)
  } finally {
    $rng.Dispose()
  }
  return [BitConverter]::ToString($bytes).Replace('-', '').ToLowerInvariant()
}

$dbPassword = New-CryptographicHexSecret
$apiKey = New-CryptographicHexSecret
try {
  $envText = Get-Content -Raw -LiteralPath $TemplatePath
  $envText = $envText.Replace(
    "FORENSIC_RUNTIME_DB_PASSWORD=CHANGE_ME",
    "FORENSIC_RUNTIME_DB_PASSWORD=$dbPassword"
  )
  $envText = $envText.Replace(
    "FORENSIC_RECORDS_API_KEY=CHANGE_ME",
    "FORENSIC_RECORDS_API_KEY=$apiKey"
  )
  $envText = $envText.Replace(
    "FORENSIC_RECORDS_TENANT_ID=default",
    "FORENSIC_RECORDS_TENANT_ID=$TenantId"
  )
  Set-Content -LiteralPath $RuntimeEnvPath -Value $envText -Encoding ascii
} finally {
  Remove-Variable dbPassword,apiKey -ErrorAction SilentlyContinue
}

$currentIdentity = [System.Security.Principal.WindowsIdentity]::GetCurrent().Name
icacls $RuntimeEnvPath /inheritance:r /grant:r "$currentIdentity`:(R,W)" | Out-Null
if ($LASTEXITCODE -ne 0) {
  throw "Could not restrict the runtime environment ACL"
}

$runtimeEnv = ((Get-Content -LiteralPath $RuntimeEnvPath |
  Where-Object { $_ -and -not $_.StartsWith('#') }) -join "`n") |
  ConvertFrom-StringData
if ($runtimeEnv.FORENSIC_RUNTIME_DB_PASSWORD -notmatch '^[0-9a-f]{64}$') {
  throw "Generated database secret failed validation"
}
if ($runtimeEnv.FORENSIC_RECORDS_API_KEY -notmatch '^[0-9a-f]{64}$') {
  throw "Generated service API secret failed validation"
}
if ($runtimeEnv.FORENSIC_RECORDS_TENANT_ID -ne $TenantId) {
  throw "Generated tenant setting failed validation"
}

Push-Location $RepoRoot
try {
  git check-ignore -q -- .env.forensic-runtime.local
  if ($LASTEXITCODE -ne 0) {
    throw "Runtime environment is not ignored by Git"
  }
} finally {
  Pop-Location
}

Write-Host "ActivationGate2=PASS SecretValuesPrinted=false GitIgnored=true Tenant=$TenantId"
