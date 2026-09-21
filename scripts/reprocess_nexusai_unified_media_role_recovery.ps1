param(
  [switch]$Approved,
  [string]$ManifestPath = 'reports/unified-multimodal-product-acceptance-20260823/P1_MEDIA_ROLE_RECOVERY_APPROVAL_MANIFEST.json',
  [int]$TimeoutMinutesPerEvidence = 30
)

$ErrorActionPreference = 'Stop'
if (-not $Approved) {
  throw 'The exact 12-job retained recovery requires explicit operator approval.'
}

$repoRoot = Split-Path -Parent $PSScriptRoot
$manifestFullPath = Join-Path $repoRoot $ManifestPath
$runtimeEnvPath = Join-Path $repoRoot '.env.forensic-runtime.local'
$reportRoot = Join-Path $repoRoot 'reports\unified-multimodal-product-acceptance-20260823'
$resultPath = Join-Path $reportRoot 'media-role-reprocess-results.json'
$verdictPath = Join-Path $reportRoot 'media-role-reprocess-verdict.json'
$originalManifestPath = Join-Path $repoRoot 'local-acceptance-inputs\NEXUSAI_MULTIMODAL_PRODUCT_ACCEPTANCE_MANIFEST.json'

. (Join-Path $PSScriptRoot 'forensic_runtime_common.ps1')
$runtime = Get-ForensicRuntimeEnvironment $runtimeEnvPath
$manifest = Get-Content -LiteralPath $manifestFullPath -Raw | ConvertFrom-Json
$originalManifest = Get-Content -LiteralPath $originalManifestPath -Raw | ConvertFrom-Json

if ($manifest.status -ne 'APPROVED_PENDING_EXECUTION') {
  throw "Recovery manifest is not executable: $($manifest.status)"
}
if (@($manifest.reprocess_plan).Count -ne 12 -or
    [int]$manifest.execution_ceiling.maximum_new_reprocess_jobs -ne 12 -or
    [int]$manifest.execution_ceiling.maximum_new_evidence -ne 0 -or
    [int]$manifest.execution_ceiling.maximum_new_versions -ne 0) {
  throw 'Recovery ceilings do not match the approved exact 12-job/no-new-evidence scope.'
}
if (Test-Path -LiteralPath $resultPath) {
  throw "A recovery result already exists; refusing to issue any POST: $resultPath"
}

$tenantId = [string]$manifest.tenant_id
$collectionId = [string]$manifest.collection_id
$caseId = [string]$manifest.case_id
$actorId = [string]$manifest.actor_id
$recordsApiUrl = 'http://localhost:8091'
$headers = @{
  Authorization = "Bearer $($runtime.FORENSIC_RECORDS_API_KEY)"
  'X-Forensic-Tenant-ID' = $tenantId
  'X-Forensic-Actor-ID' = $actorId
  'X-Forensic-Subject-ID' = $actorId
  'X-Forensic-Actor-Role' = 'admin'
  'X-Forensic-Collection-ID' = $collectionId
  'X-Forensic-Case-ID' = $caseId
}

function Get-EvidenceDetail([string]$EvidenceId) {
  Invoke-RestMethod -TimeoutSec 120 -Headers $headers -Uri "$recordsApiUrl/evidence/$([uri]::EscapeDataString($EvidenceId))?limit=300"
}

function Get-LatestJob($Detail) {
  @($Detail.ingest_jobs) |
    Sort-Object @{Expression={[int]$_.reprocess_generation};Descending=$true}, queued_at -Descending |
    Select-Object -First 1
}

function Assert-ExactEvidence($Plan, $Detail) {
  if ([string]$Detail.item.evidence_id -ne [string]$Plan.evidence_id -or
      [string]$Detail.item.sha256 -ne [string]$Plan.sha256 -or
      [string]$Detail.item.collection_id -ne $collectionId -or
      [string]$Detail.item.case_id -ne $caseId) {
    throw "Evidence identity or scope mismatch for $($Plan.source_id)"
  }
}

$inventory = Invoke-RestMethod -TimeoutSec 120 -Headers $headers -Uri "$recordsApiUrl/evidence?collection_id=$([uri]::EscapeDataString($collectionId))&limit=100"
if ([int]$inventory.pagination.evidence_total -ne 21 -or @($inventory.items).Count -ne 21) {
  throw 'Target evidence count changed from the approved 21-source manifest.'
}
$approvedHashes = @($originalManifest.sources.sha256)
$unexpected = @($inventory.items | Where-Object { [string]$_.sha256 -notin $approvedHashes })
if ($unexpected.Count -gt 0) {
  throw "Target contains unmanifested evidence: $(@($unexpected.evidence_id) -join ', ')"
}

foreach ($item in $inventory.items) {
  $detail = Get-EvidenceDetail ([string]$item.evidence_id)
  $latest = Get-LatestJob $detail
  if ($latest -and [string]$latest.status -in @('queued', 'running', 'failed')) {
    throw "Target has an active or retryable job before recovery: evidence=$($item.evidence_id) status=$($latest.status)"
  }
}

$results = @()
foreach ($plan in @($manifest.reprocess_plan | Sort-Object order)) {
  $detail = Get-EvidenceDetail ([string]$plan.evidence_id)
  Assert-ExactEvidence $plan $detail
  $latest = Get-LatestJob $detail
  $expectedParentGeneration = [int]$plan.expected_generation - 1
  if (-not $latest -or [int]$latest.reprocess_generation -ne $expectedParentGeneration -or [string]$latest.status -ne 'completed') {
    throw "Unexpected parent generation for $($plan.source_id): expected completed generation $expectedParentGeneration"
  }

  $idempotencyKey = "unified-media-role-p1:$($plan.order):$($plan.source_id):20260823"
  $body = @{
    collection_id = $collectionId
    reason = [string]$plan.reason
    idempotency_key = $idempotencyKey
    max_attempts = 5
  } | ConvertTo-Json -Compress
  $requestHeaders = @{} + $headers
  $requestHeaders['Idempotency-Key'] = $idempotencyKey
  Write-Host "MediaRoleRecoveryPOST=$($plan.order)/12 Source=$($plan.source_id) ParentGeneration=$expectedParentGeneration"
  $started = Get-Date
  $response = Invoke-RestMethod -Method Post -TimeoutSec 120 -Headers $requestHeaders `
    -Uri "$recordsApiUrl/evidence/$([uri]::EscapeDataString([string]$plan.evidence_id))/reprocess" `
    -ContentType 'application/json' -Body $body
  if ([bool]$response.idempotent_replay -or [int]$response.reprocess_generation -ne [int]$plan.expected_generation) {
    throw "Unexpected reprocess response for $($plan.source_id)"
  }

  $deadline = (Get-Date).AddMinutes($TimeoutMinutesPerEvidence)
  do {
    Start-Sleep -Seconds 3
    $terminalDetail = Get-EvidenceDetail ([string]$plan.evidence_id)
    $terminal = Get-LatestJob $terminalDetail
    if ([int]$terminal.reprocess_generation -ne [int]$plan.expected_generation) {
      throw "Latest generation changed unexpectedly for $($plan.source_id)"
    }
    if ([string]$terminal.status -in @('failed', 'dead_letter')) {
      $failure = [ordered]@{
        verdict = 'STOPPED_ON_FAILURE'
        recorded_at = (Get-Date).ToUniversalTime().ToString('o')
        source_id = [string]$plan.source_id
        evidence_id = [string]$plan.evidence_id
        job_id = [string]$terminal.job_id
        generation = [int]$terminal.reprocess_generation
        status = [string]$terminal.status
        error_message = [string]$terminal.error_message
        completed_results = $results
      }
      $failure | ConvertTo-Json -Depth 12 | Set-Content -LiteralPath $verdictPath -Encoding utf8
      throw "$($plan.source_id) entered $($terminal.status): $($terminal.error_message)"
    }
    if ([string]$terminal.status -eq 'completed') { break }
  } while ((Get-Date) -lt $deadline)
  if ([string]$terminal.status -ne 'completed') {
    throw "Timed out waiting for $($plan.source_id)"
  }

  $result = [pscustomobject]@{
    order = [int]$plan.order
    source_id = [string]$plan.source_id
    evidence_id = [string]$plan.evidence_id
    sha256 = [string]$plan.sha256
    job_id = [string]$terminal.job_id
    reprocess_of_job_id = [string]$terminal.reprocess_of_job_id
    reprocess_generation = [int]$terminal.reprocess_generation
    status = [string]$terminal.status
    attempt_count = [int]$terminal.attempt_count
    accepted_rows = [int]$terminal.accepted_rows
    duplicate_rows = [int]$terminal.duplicate_rows
    rejected_rows = [int]$terminal.rejected_rows
    latency_ms = [math]::Round(((Get-Date) - $started).TotalMilliseconds, 1)
  }
  $results += $result
  $results | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $resultPath -Encoding utf8
  Write-Host "MediaRoleRecoveryTerminal=$($plan.order)/12 Source=$($plan.source_id) Job=$($terminal.job_id) Generation=$($terminal.reprocess_generation) Accepted=$($terminal.accepted_rows)"
}

$verdict = [ordered]@{
  verdict = 'RECOVERY_COMPLETE_PENDING_PRODUCT_ACCEPTANCE'
  recorded_at = (Get-Date).ToUniversalTime().ToString('o')
  tenant_id = $tenantId
  collection_id = $collectionId
  case_id = $caseId
  actor_id = $actorId
  reprocess_jobs = $results.Count
  evidence_ids = @($results.evidence_id)
  all_completed = @($results | Where-Object status -ne 'completed').Count -eq 0
  results_path = 'reports/unified-multimodal-product-acceptance-20260823/media-role-reprocess-results.json'
}
$verdict | ConvertTo-Json -Depth 12 | Set-Content -LiteralPath $verdictPath -Encoding utf8
Write-Host "MediaRoleRecovery=PASS Jobs=$($results.Count) Collection=$collectionId"
