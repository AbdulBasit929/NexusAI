$ErrorActionPreference = 'Stop'
$RepoRoot = Split-Path -Parent $PSScriptRoot
$RuntimeEnvPath = Join-Path $RepoRoot '.env.forensic-runtime.local'
$LogRoot = Join-Path $RepoRoot 'reports\runtime-activation-20260730'
$Gate10Marker = Join-Path $LogRoot 'gate10-synthetic-success.json'
$MarkerPath = Join-Path $LogRoot 'gate11-redelivery.json'
. (Join-Path $PSScriptRoot 'forensic_runtime_common.ps1')

Assert-ForensicMarker $Gate10Marker 'Gate 10'
$runtime = Get-ForensicRuntimeEnvironment $RuntimeEnvPath
$workerId = docker compose -p nexusai --env-file $RuntimeEnvPath `
  -f (Join-Path $RepoRoot 'docker-compose.forensic-records.yaml') `
  -f (Join-Path $RepoRoot 'docker-compose.forensic-records.runtime.yaml') ps -q forensic-records-worker
if ($LASTEXITCODE -ne 0 -or -not $workerId) { throw 'Healthy worker container is missing' }
Wait-ForensicContainer -ContainerId $workerId -Label 'forensic-records-worker'

$scriptText = Get-Content -Raw -LiteralPath (Join-Path $PSScriptRoot 'verify_forensic_redelivery.py')
$savedPreference = $ErrorActionPreference
try {
  $ErrorActionPreference = 'Continue'
  $result = $scriptText | docker exec -i $workerId python - $runtime.FORENSIC_RECORDS_TENANT_ID 2>&1
  $nativeExit = $LASTEXITCODE
} finally {
  $ErrorActionPreference = $savedPreference
}
if ($nativeExit -ne 0) { throw "Controlled completed-redelivery proof failed (native exit $nativeExit)" }
if (@($result) -notcontains 'ActivationGate11=PASS CompletedRedeliveryAck=PASS DuplicateEffects=0 StreamDrained=true RealEvidenceUsed=false') {
  throw 'Controlled completed-redelivery proof returned no PASS marker'
}
[ordered]@{
  gate = 11; completed_utc = (Get-Date).ToUniversalTime().ToString('o')
  completed_redelivery_ack = 'pass'; duplicate_effects = 0
  stream_drained = $true; real_evidence_used = $false
} | ConvertTo-Json | Set-Content -Encoding ascii -LiteralPath $MarkerPath
Write-Host 'ActivationGate11=PASS CompletedRedeliveryAck=PASS DuplicateEffects=0 StreamDrained=true RealEvidenceUsed=false'
