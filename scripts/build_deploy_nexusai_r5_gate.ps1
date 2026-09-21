param(
  [string]$CaseID = 'nexusai-forensic-demo',
  [string]$EvidenceID = '',
  [string]$ApiKey = $env:LOCALAI_API_KEY
)

$ErrorActionPreference = 'Stop'
$gateMutex = New-Object System.Threading.Mutex($false, 'Local\NexusAIR5ActivationGate')
if (-not $gateMutex.WaitOne(0)) {
  $gateMutex.Dispose()
  throw 'Another NexusAI R5 activation gate is already running'
}

$RepoRoot = Split-Path -Parent $PSScriptRoot
$LogRoot = Join-Path $RepoRoot 'reports\runtime-activation-20260810'
$MarkerPath = Join-Path $LogRoot 'r5-live-acceptance.json'
$CustodyMigrationMarker = Join-Path $LogRoot 'r5-custody-migration-011.json'
$CombinedGate = Join-Path $PSScriptRoot 'build_deploy_forensic_phase6_gate.ps1'
$SwaggerPath = Join-Path $RepoRoot 'swagger\swagger.json'
$ExpectedRoute = '/api/v1/forensics/cases/{case_id}/evidence/{evidence_id}/reprocess-plan'
$ExpectedContract = 'forensics.evidence-reprocess-plan/v1'
$headers = @{}
if (-not [string]::IsNullOrWhiteSpace($ApiKey)) {
  $headers['Authorization'] = "Bearer $ApiKey"
}

function Invoke-R5JSONGet {
  param([Parameter(Mandatory=$true)][string]$Uri)
  return Invoke-RestMethod -Method Get -UseBasicParsing -TimeoutSec 30 -Headers $headers -Uri $Uri
}

function Assert-R5SourceContract {
  if (-not (Test-Path -LiteralPath $SwaggerPath)) {
    throw "Generated Swagger contract is missing: $SwaggerPath"
  }
  $swagger = Get-Content -Raw -LiteralPath $SwaggerPath | ConvertFrom-Json
  if (-not $swagger.paths.PSObject.Properties[$ExpectedRoute]) {
    throw "Generated Swagger does not publish the R5 reprocess-plan route: $ExpectedRoute"
  }
  $routeSource = Join-Path $RepoRoot 'core\http\routes\records.go'
  if (-not (Select-String -LiteralPath $routeSource -SimpleMatch 'evidence/:evidence_id/reprocess-plan' -Quiet)) {
    throw 'The LocalAI R5 reprocess-plan route registration is missing'
  }
}

function Assert-R5RuntimeContract {
  $ready = Invoke-WebRequest -Method Get -UseBasicParsing -TimeoutSec 30 -Uri 'http://localhost:8080/readyz'
  $recordsHealth = Invoke-WebRequest -Method Get -UseBasicParsing -TimeoutSec 30 -Uri 'http://localhost:8091/healthz'
  $app = Invoke-WebRequest -Method Get -UseBasicParsing -TimeoutSec 30 -Headers $headers -Uri 'http://localhost:8080/app'
  if ($ready.StatusCode -ne 200 -or $recordsHealth.StatusCode -ne 200 -or $app.StatusCode -ne 200) {
    throw 'R5 health or application-shell acceptance failed'
  }

  $contracts = Invoke-R5JSONGet 'http://localhost:8080/api/v1/forensics/contracts'
  $workflow = @($contracts.workflow_contracts | Where-Object {
    $_.id -eq 'evidence_reprocess_plan' -and
    $_.version -eq $ExpectedContract -and
    $_.method -eq 'GET' -and
    $_.execution -eq 'read_only_approval_required'
  })
  if ($workflow.Count -ne 1) {
    throw 'The deployed forensic contract catalog does not publish the exact R5 read-only workflow'
  }

  $encodedCase = [uri]::EscapeDataString($CaseID)
  $catalog = Invoke-R5JSONGet "http://localhost:8080/api/v1/forensics/cases/$encodedCase/evidence?limit=1&offset=0"
  $selectedEvidenceID = $EvidenceID
  if ([string]::IsNullOrWhiteSpace($selectedEvidenceID)) {
    $selectedEvidenceID = [string](@($catalog.items)[0].evidence_id)
  }
  if ([string]::IsNullOrWhiteSpace($selectedEvidenceID)) {
    throw "Case '$CaseID' has no registered evidence for the read-only R5 plan gate; no data was created or changed"
  }

  $encodedEvidence = [uri]::EscapeDataString($selectedEvidenceID)
  $detail = Invoke-R5JSONGet "http://localhost:8080/api/v1/forensics/cases/$encodedCase/evidence/${encodedEvidence}?limit=20"
  if ([int]$detail.custody_integrity.event_count -gt 0 -and $detail.custody_integrity.chain_valid -ne $true) {
    throw "The deployed custody-chain integrity gate failed with $($detail.custody_integrity.broken_links) reported broken links"
  }
  $plan = Invoke-R5JSONGet "http://localhost:8080/api/v1/forensics/cases/$encodedCase/evidence/$encodedEvidence/reprocess-plan"
  if ($plan.contract_version -ne $ExpectedContract -or $plan.case_id -ne $CaseID -or
      $plan.collection_id -ne $CaseID -or $plan.evidence_id -ne $selectedEvidenceID) {
    throw 'The deployed R5 plan is not bound to the requested case and evidence contract'
  }
  if ($plan.approval.required -ne $true -or $plan.approval.execution_permitted -ne $false -or
      $plan.approval.state -ne 'not_granted') {
    throw 'The deployed R5 plan did not preserve its fail-closed approval boundary'
  }

  return [ordered]@{
    case_id = $CaseID
    evidence_id = $selectedEvidenceID
    eligible = [bool]$plan.eligibility.eligible
    approval_required = [bool]$plan.approval.required
    execution_permitted = [bool]$plan.approval.execution_permitted
    custody_event_count = [int]$detail.custody_integrity.event_count
    custody_chain_valid = [bool]$detail.custody_integrity.chain_valid
  }
}

Push-Location $RepoRoot
try {
  Assert-R5SourceContract
  if (-not (Test-Path -LiteralPath $CustodyMigrationMarker)) {
    throw 'Verified migration 011 marker is missing; run apply_nexusai_r5_custody_fix.ps1 with explicit migration approval before the final rebuild'
  }
  New-Item -ItemType Directory -Force -Path $LogRoot | Out-Null

  # This proven combined gate preserves rollback images and named volumes,
  # rebuilds API/worker/LocalAI sequentially, and rolls back on build/health
  # failure. It never prunes images or volumes and never migrates retained data.
  & $CombinedGate
  if ($LASTEXITCODE -ne 0) { throw "The guarded combined rebuild failed with exit code $LASTEXITCODE" }

  $acceptance = Assert-R5RuntimeContract
  [ordered]@{
    phase = 'R5'
    status = 'live_accepted'
    deployed_utc = (Get-Date).ToUniversalTime().ToString('o')
    contract_version = $ExpectedContract
    case_id = $acceptance.case_id
    evidence_id = $acceptance.evidence_id
    eligible = $acceptance.eligible
    approval_required = $acceptance.approval_required
    execution_permitted = $acceptance.execution_permitted
    custody_event_count = $acceptance.custody_event_count
    custody_chain_valid = $acceptance.custody_chain_valid
    source_contract = 'pass'
    localai_ready = 'pass'
    records_health = 'pass'
    app_shell = 'pass'
    contract_catalog = 'pass'
    evidence_catalog = 'pass'
    reprocess_plan_get = 'pass'
    mutating_requests_sent = 0
    named_volumes_preserved = $true
    rollback_images_preserved = $true
  } | ConvertTo-Json | Set-Content -Encoding ascii -LiteralPath $MarkerPath
  Write-Host "R5Activation=PASS CaseID=$($acceptance.case_id) EvidenceID=$($acceptance.evidence_id) Contract=$ExpectedContract MutatingRequests=0 RollbackImages=preserved VolumesPreserved=true"
} finally {
  Pop-Location
  $gateMutex.ReleaseMutex()
  $gateMutex.Dispose()
}
