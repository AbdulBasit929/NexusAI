$ErrorActionPreference = 'Stop'
$RepoRoot = Split-Path -Parent $PSScriptRoot
$RuntimeEnvPath = Join-Path $RepoRoot '.env.forensic-runtime.local'
$LogRoot = Join-Path $RepoRoot 'reports\runtime-activation-20260730'
$Gate8Marker = Join-Path $LogRoot 'gate8-deployment.json'
$MarkerPath = Join-Path $LogRoot 'gate9-runtime-contract.json'
. (Join-Path $PSScriptRoot 'forensic_runtime_common.ps1')

Assert-ForensicMarker $Gate8Marker 'Gate 8'
$runtime = Get-ForensicRuntimeEnvironment $RuntimeEnvPath
$health = Invoke-RestMethod -TimeoutSec 15 -Uri 'http://localhost:8091/healthz'
if (-not $health) { throw 'Forensic API health failed' }
$workerMetrics = Invoke-WebRequest -UseBasicParsing -TimeoutSec 15 -Uri 'http://localhost:9109/metrics'
if ($workerMetrics.StatusCode -ne 200 -or $workerMetrics.Content -notmatch 'forensic_records_jobs_total') {
  throw 'Forensic worker metrics/liveness failed'
}
$localAIReady = Invoke-WebRequest -UseBasicParsing -TimeoutSec 15 -Uri 'http://localhost:8080/readyz'
if ($localAIReady.StatusCode -ne 200) { throw 'LocalAI readiness failed' }

try {
  Invoke-WebRequest -UseBasicParsing -TimeoutSec 15 -Uri 'http://localhost:8091/query/templates' | Out-Null
  throw 'Unauthenticated forensic API request unexpectedly succeeded'
} catch {
  if ($_.Exception.Message -eq 'Unauthenticated forensic API request unexpectedly succeeded') { throw }
  $status = [int]$_.Exception.Response.StatusCode
  if ($status -ne 401) { throw "Expected unauthenticated 401; received $status" }
}

$collection = 'nexusai-runtime-acceptance-20260730'
$headers = @{
  Authorization = "Bearer $($runtime.FORENSIC_RECORDS_API_KEY)"
  'X-Forensic-Tenant-ID' = $runtime.FORENSIC_RECORDS_TENANT_ID
  'X-Forensic-Actor-ID' = 'nexusai-runtime-operator'
  'X-Forensic-Subject-ID' = 'nexusai-runtime-operator'
  'X-Forensic-Actor-Role' = 'admin'
  'X-Forensic-Collection-ID' = $collection
}
$templates = Invoke-RestMethod -TimeoutSec 15 -Uri 'http://localhost:8091/query/templates' -Headers $headers
if (@($templates.templates).Count -lt 1) { throw 'Authenticated template request returned no templates' }

$wrongTenantHeaders = $headers.Clone()
$wrongTenantHeaders['X-Forensic-Tenant-ID'] = '__nexusai_wrong_tenant__'
try {
  Invoke-WebRequest -UseBasicParsing -TimeoutSec 15 -Uri 'http://localhost:8091/query/templates' -Headers $wrongTenantHeaders | Out-Null
  throw 'Wrong-tenant request unexpectedly succeeded'
} catch {
  if ($_.Exception.Message -eq 'Wrong-tenant request unexpectedly succeeded') { throw }
  $status = [int]$_.Exception.Response.StatusCode
  if ($status -ne 403) { throw "Expected wrong-tenant 403; received $status" }
}

$js = Invoke-RestMethod -TimeoutSec 15 -Uri 'http://localhost:8222/jsz?streams=true&consumers=true&config=true'
$jsText = $js | ConvertTo-Json -Depth 30 -Compress
foreach ($required in @('FORENSIC_RECORDS_INGEST','FORENSIC_RECORDS_DLQ')) {
  if ($jsText -notmatch ('"' + [regex]::Escape($required) + '"')) { throw "Missing JetStream stream $required" }
}
[ordered]@{
  gate = 9; verified_utc = (Get-Date).ToUniversalTime().ToString('o')
  forensic_health = 'pass'; worker_liveness = 'pass'; localai_ready = 'pass'; auth_401 = 'pass'
  authenticated_scope = 'pass'; cross_tenant_403 = 'pass'; jetstream = 'pass'
} | ConvertTo-Json | Set-Content -Encoding ascii -LiteralPath $MarkerPath

Write-Host 'ActivationGate9=PASS Health=PASS Worker=PASS LocalAI=PASS Auth401=PASS AuthenticatedScope=PASS CrossTenant403=PASS JetStream=PASS'
