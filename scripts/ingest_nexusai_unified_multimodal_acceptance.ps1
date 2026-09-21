param(
  [switch]$Approved,
  [string]$ManifestPath = 'local-acceptance-inputs/NEXUSAI_MULTIMODAL_PRODUCT_ACCEPTANCE_MANIFEST.json',
  [int]$TimeoutMinutesPerSource = 20
)

$ErrorActionPreference = 'Stop'
if (-not $Approved) {
  throw 'The exact unified multimodal manifest requires explicit retained-mutation approval.'
}

$repoRoot = Split-Path -Parent $PSScriptRoot
$manifestFullPath = Join-Path $repoRoot $ManifestPath
$runtimeEnvPath = Join-Path $repoRoot '.env.forensic-runtime.local'
$reportRoot = Join-Path $repoRoot 'reports\unified-multimodal-product-acceptance-20260823'
$sourceCopyRoot = Join-Path $repoRoot 'local-acceptance-inputs\unified-source-copies'
. (Join-Path $PSScriptRoot 'forensic_runtime_common.ps1')
$runtime = Get-ForensicRuntimeEnvironment $runtimeEnvPath

if (-not (Test-Path -LiteralPath $manifestFullPath -PathType Leaf)) {
  throw "Manifest is missing: $manifestFullPath"
}
$manifest = Get-Content -LiteralPath $manifestFullPath -Raw | ConvertFrom-Json
if ($manifest.status -ne 'APPROVED_PENDING_EXECUTION') {
  throw "Manifest status is not executable: $($manifest.status)"
}
if ([string]::IsNullOrWhiteSpace([string]$manifest.actor_id)) {
  throw 'Manifest actor_id is required.'
}
if (@($manifest.sources).Count -ne 21 -or [int]$manifest.execution_ceiling.maximum_new_evidence -ne 21 -or
    [int]$manifest.execution_ceiling.maximum_new_ingest_jobs -ne 21 -or
    [int]$manifest.execution_ceiling.maximum_reprocess_jobs -ne 0) {
  throw 'Manifest ceilings do not match the approved 21-source/no-reprocess scope.'
}

$tenantId = [string]$manifest.tenant_id
$collectionId = [string]$manifest.collection_id
$caseId = [string]$manifest.case_id
$actorId = [string]$manifest.actor_id
$recordsApiUrl = 'http://localhost:8091'
$localAIUrl = 'http://localhost:8080'

$headers = @{
  Authorization = "Bearer $($runtime.FORENSIC_RECORDS_API_KEY)"
  'X-Forensic-Tenant-ID' = $tenantId
  'X-Forensic-Actor-ID' = $actorId
  'X-Forensic-Subject-ID' = $actorId
  'X-Forensic-Actor-Role' = 'admin'
  'X-Forensic-Collection-ID' = $collectionId
  'X-Forensic-Case-ID' = $caseId
}

function Get-SHA256([string]$Path) {
  (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant()
}

function Assert-SourceHash($Source, [string]$Path) {
  $file = Get-Item -LiteralPath $Path
  $actualHash = Get-SHA256 $Path
  if ($file.Length -ne [int64]$Source.size_bytes -or $actualHash -ne [string]$Source.sha256) {
    throw "Source verification failed for $($Source.source_id): size=$($file.Length) sha256=$actualHash"
  }
}

function Get-RecordType([string]$SourceId) {
  switch ($SourceId) {
    'structured-cdr' { 'cdr' }
    'structured-ipdr' { 'ipdr' }
    'structured-subscriber' { 'subscriber' }
    'structured-tower' { 'tower_location' }
    'structured-anpr' { 'anpr' }
    'structured-access-log' { 'access_log' }
    'structured-transactions' { 'transaction' }
    default { 'auto' }
  }
}

function Get-Phase([string]$SourceId) {
  if ($SourceId -like 'structured-*') { return 1 }
  if ($SourceId -in @('image-test-plate', 'image-islamabad-road')) { return 2 }
  if ($SourceId -like 'face-*') { return 3 }
  if ($SourceId -like 'audio-*') { return 4 }
  if ($SourceId -like 'video-*') { return 5 }
  if ($SourceId -like 'document-*') { return 6 }
  throw "No controlled phase is defined for $SourceId"
}

function Resolve-SourcePath($Source) {
  if ($Source.kind -eq 'local_path') {
    $path = Join-Path $repoRoot ([string]$Source.path)
    if (-not (Test-Path -LiteralPath $path -PathType Leaf)) {
      throw "Manifest source is missing: $($Source.source_id) $path"
    }
    Assert-SourceHash $Source $path
    return $path
  }
  if ($Source.kind -ne 'existing_retained_evidence') {
    throw "Unsupported manifest source kind: $($Source.kind)"
  }

  New-Item -ItemType Directory -Force -Path $sourceCopyRoot | Out-Null
  $safeName = ([string]$Source.filename) -replace '[^A-Za-z0-9._-]', '_'
  $path = Join-Path $sourceCopyRoot ("$($Source.source_id)-$safeName")
  if (-not (Test-Path -LiteralPath $path -PathType Leaf)) {
    $sourceCollection = [string]$Source.source_collection_id
    $sourceCase = [string]$Source.source_case_id
    $sourceHeaders = @(
      '-H', "Authorization: Bearer $($runtime.FORENSIC_RECORDS_API_KEY)",
      '-H', "X-Forensic-Tenant-ID: $tenantId",
      '-H', "X-Forensic-Actor-ID: $actorId",
      '-H', "X-Forensic-Subject-ID: $actorId",
      '-H', 'X-Forensic-Actor-Role: admin',
      '-H', "X-Forensic-Collection-ID: $sourceCollection",
      '-H', "X-Forensic-Case-ID: $sourceCase"
    )
    $url = "$recordsApiUrl/evidence/$($Source.evidence_id)/content?tenant_id=$([uri]::EscapeDataString($tenantId))&collection_id=$([uri]::EscapeDataString($sourceCollection))&case_id=$([uri]::EscapeDataString($sourceCase))"
    & curl.exe --fail-with-body -sS @sourceHeaders -o $path $url
    if ($LASTEXITCODE -ne 0) { throw "Authorized source copy failed for $($Source.source_id)" }
  }
  Assert-SourceHash $Source $path
  return $path
}

function Get-EvidenceDetail([string]$EvidenceId) {
  $uri = "$recordsApiUrl/evidence/$([uri]::EscapeDataString($EvidenceId))?limit=25&include_records_preview=false&view=accounting"
  Invoke-RestMethod -TimeoutSec 120 -Headers $headers -Uri $uri
}

function Wait-Terminal([string]$SourceId, [string]$EvidenceId, [string]$InitialStatus) {
  $deadline = (Get-Date).AddMinutes($TimeoutMinutesPerSource)
  $lastNotice = Get-Date
  do {
    $detail = Get-EvidenceDetail $EvidenceId
    $jobs = @($detail.ingest_jobs)
    $latest = $jobs | Sort-Object @{Expression={[int]$_.reprocess_generation};Descending=$true}, completed_at -Descending | Select-Object -First 1
    if ($latest.status -in @('failed', 'dead_letter')) {
      throw "$SourceId latest generation entered a terminal failure: generation=$($latest.reprocess_generation) status=$($latest.status) error=$($latest.error_message)"
    }
    if ($latest.status -eq 'completed') {
      return [pscustomobject]@{ detail=$detail; job=$latest; terminal='completed' }
    }
    if ($jobs.Count -eq 0 -and $InitialStatus -eq 'registered') {
      return [pscustomobject]@{ detail=$detail; job=$null; terminal='registered_without_job' }
    }
    if (((Get-Date) - $lastNotice).TotalSeconds -ge 15) {
      Write-Host "UnifiedAcceptanceWait=$SourceId EvidenceId=$EvidenceId"
      $lastNotice = Get-Date
    }
    Start-Sleep -Seconds 3
  } until ((Get-Date) -gt $deadline)
  throw "Timed out waiting for $SourceId ($EvidenceId)"
}

function Process-Source($Source, [string]$Path, $ExistingEvidence) {
  $recordType = Get-RecordType ([string]$Source.source_id)
  if ($ExistingEvidence) {
    Write-Host "UnifiedAcceptanceResume=$($Source.source_id) EvidenceId=$($ExistingEvidence.evidence_id)"
    $payload = [pscustomobject]@{
      status = [string]$ExistingEvidence.processing_status
      evidence_id = [string]$ExistingEvidence.evidence_id
      evidence_version_id = [string]$ExistingEvidence.current_version_id
      sha256 = [string]$ExistingEvidence.sha256
    }
  } else {
    $args = @(
      '--fail-with-body', '-sS', '--max-time', '180', '-X', 'POST', "$recordsApiUrl/webhooks/records/upload",
      '-H', "Authorization: Bearer $($runtime.FORENSIC_RECORDS_API_KEY)",
      '-H', "X-Forensic-Tenant-ID: $tenantId",
      '-H', "X-Forensic-Actor-ID: $actorId",
      '-H', "X-Forensic-Subject-ID: $actorId",
      '-H', 'X-Forensic-Actor-Role: admin',
      '-H', "X-Forensic-Collection-ID: $collectionId",
      '-H', "X-Forensic-Case-ID: $caseId",
      '-F', "tenant_id=$tenantId",
      '-F', "collection_id=$collectionId",
      '-F', "case_id=$caseId",
      '-F', "user_id=$actorId",
      '-F', "record_type=$recordType",
      '-F', 'jurisdiction=PK',
      '-F', 'source_timezone=Asia/Karachi',
      '-F', 'source_timezone_state=analyst_confirmed',
      '-F', 'evidence_role=source',
      '-F', "file=@$Path"
    )
    if ($Source.source_id -like 'audio-*' -or $Source.source_id -like 'video-*') {
      $args += @('-F', 'asr_language=ur')
    }
    $raw = (& curl.exe @args) -join "`n"
    if ($LASTEXITCODE -ne 0) { throw "Upload failed for $($Source.source_id)" }
    $payload = $raw | ConvertFrom-Json
    if ($payload.status -eq 'duplicate') {
      throw "Unexpected duplicate in target for $($Source.source_id)"
    }
    if (-not $payload.evidence_id) { throw "Upload returned no evidence ID for $($Source.source_id)" }
    if ([string]$payload.sha256 -ne [string]$Source.sha256) {
      throw "Upload hash mismatch for $($Source.source_id)"
    }
  }
  $terminal = Wait-Terminal ([string]$Source.source_id) ([string]$payload.evidence_id) ([string]$payload.status)
  [pscustomobject]@{
    phase = Get-Phase ([string]$Source.source_id)
    source_id = [string]$Source.source_id
    filename = Split-Path -Leaf $Path
    sha256 = [string]$Source.sha256
    evidence_id = [string]$payload.evidence_id
    evidence_version_id = [string]$payload.evidence_version_id
    job_id = if ($payload.job_id) { [string]$payload.job_id } elseif ($terminal.job.job_id) { [string]$terminal.job.job_id } else { $null }
    status = [string]$terminal.terminal
    record_type = if ($terminal.job) { [string]$terminal.job.record_type } else { $recordType }
    total_rows = if ($terminal.job) { [int]$terminal.job.total_rows } else { 0 }
    accepted_rows = if ($terminal.job) { [int]$terminal.job.accepted_rows } else { 0 }
    duplicate_rows = if ($terminal.job) { [int]$terminal.job.duplicate_rows } else { 0 }
    rejected_rows = if ($terminal.job) { [int]$terminal.job.rejected_rows } else { 0 }
  }
}

$targetInventory = Invoke-RestMethod -TimeoutSec 120 -Headers $headers -Uri "$recordsApiUrl/evidence?collection_id=$([uri]::EscapeDataString($collectionId))&limit=100"
$existingEvidence = @($targetInventory.items)
if ([int]$targetInventory.pagination.evidence_total -gt 21 -or $existingEvidence.Count -gt 21) {
  throw 'The target exceeds the approved evidence ceiling.'
}
$approvedHashes = @($manifest.sources.sha256)
$unmanifested = @($existingEvidence | Where-Object { [string]$_.sha256 -notin $approvedHashes })
if ($unmanifested.Count -gt 0) {
  throw "The target contains unmanifested evidence: $(@($unmanifested.evidence_id) -join ', ')"
}
$duplicateExistingHashes = @($existingEvidence | Group-Object sha256 | Where-Object Count -gt 1)
if ($duplicateExistingHashes.Count -gt 0) {
  throw "The target contains duplicate evidence hashes outside the one-source-per-hash plan."
}

$resolved = @()
foreach ($source in $manifest.sources) {
  $resolved += [pscustomobject]@{ source=$source; path=(Resolve-SourcePath $source); phase=(Get-Phase ([string]$source.source_id)) }
}
if (@($resolved).Count -ne 21) { throw 'Resolved source count changed from the approved manifest.' }

$collections = Invoke-RestMethod -TimeoutSec 30 -Uri "$localAIUrl/api/agents/collections"
if ($collectionId -notin @($collections.collections)) {
  [void](Invoke-RestMethod -Method Post -TimeoutSec 30 -Uri "$localAIUrl/api/agents/collections" -ContentType 'application/json' -Body (@{name=$collectionId} | ConvertTo-Json -Compress))
}

New-Item -ItemType Directory -Force -Path $reportRoot | Out-Null
$before = [ordered]@{
  recorded_at = (Get-Date).ToUniversalTime().ToString('o')
  tenant_id = $tenantId
  collection_id = $collectionId
  case_id = $caseId
  actor_id = $actorId
  manifest_version = [string]$manifest.manifest_version
  source_count = @($manifest.sources).Count
  source_hashes_verified = $true
}
$before | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath (Join-Path $reportRoot 'before.json') -Encoding utf8

$results = @()
for ($phase = 1; $phase -le 6; $phase++) {
  $phaseSources = @($resolved | Where-Object phase -eq $phase)
  Write-Host "UnifiedAcceptancePhase=$phase Sources=$($phaseSources.Count)"
  foreach ($item in $phaseSources) {
    Write-Host "UnifiedAcceptanceUpload=$($item.source.source_id)"
    $existing = $existingEvidence | Where-Object { [string]$_.sha256 -eq [string]$item.source.sha256 } | Select-Object -First 1
    $result = Process-Source $item.source $item.path $existing
    $results += $result
    $results | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath (Join-Path $reportRoot 'ingest-results.json') -Encoding utf8
    Write-Host "UnifiedAcceptanceTerminal=$($result.source_id) Status=$($result.status) EvidenceId=$($result.evidence_id) JobId=$($result.job_id)"
  }
  $phaseInventory = Invoke-RestMethod -TimeoutSec 30 -Headers $headers -Uri "$recordsApiUrl/evidence?collection_id=$([uri]::EscapeDataString($collectionId))&limit=100"
  $phaseCapabilities = Invoke-RestMethod -TimeoutSec 30 -Headers $headers -Uri "$recordsApiUrl/query/capabilities?collection_id=$([uri]::EscapeDataString($collectionId))"
  [ordered]@{
    phase = $phase
    recorded_at = (Get-Date).ToUniversalTime().ToString('o')
    evidence_count = [int]$phaseInventory.pagination.evidence_total
    completed_source_ids = @($results.source_id)
    capabilities = $phaseCapabilities
  } | ConvertTo-Json -Depth 30 | Set-Content -LiteralPath (Join-Path $reportRoot ("phase-$phase-reconciliation.json")) -Encoding utf8
}

if (@($results).Count -ne 21) { throw "Only $(@($results).Count) of 21 approved sources reached a terminal state." }
$finalInventory = Invoke-RestMethod -TimeoutSec 30 -Headers $headers -Uri "$recordsApiUrl/evidence?collection_id=$([uri]::EscapeDataString($collectionId))&limit=100"
if ([int]$finalInventory.pagination.evidence_total -ne 21) { throw "Final target evidence count is $($finalInventory.pagination.evidence_total), expected 21." }

[ordered]@{
  verdict = 'INGEST_COMPLETE_PENDING_PRODUCT_ACCEPTANCE'
  recorded_at = (Get-Date).ToUniversalTime().ToString('o')
  tenant_id = $tenantId
  collection_id = $collectionId
  case_id = $caseId
  actor_id = $actorId
  evidence_count = [int]$finalInventory.pagination.evidence_total
  job_count = @($results | Where-Object job_id).Count
  completed_jobs = @($results | Where-Object status -eq 'completed').Count
  registered_without_job = @($results | Where-Object status -eq 'registered_without_job').Count
  accepted_rows = [int](($results | Measure-Object accepted_rows -Sum).Sum)
  duplicate_rows = [int](($results | Measure-Object duplicate_rows -Sum).Sum)
  rejected_rows = [int](($results | Measure-Object rejected_rows -Sum).Sum)
  results = $results
} | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath (Join-Path $reportRoot 'ingest-verdict.json') -Encoding utf8

Write-Host "UnifiedAcceptanceIngest=PASS Evidence=21 Jobs=$(@($results | Where-Object job_id).Count) Collection=$collectionId"
