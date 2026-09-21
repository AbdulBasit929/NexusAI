$ErrorActionPreference = 'Stop'
$RepoRoot = Split-Path -Parent $PSScriptRoot
$RuntimeEnvPath = Join-Path $RepoRoot '.env.forensic-runtime.local'
$LogRoot = Join-Path $RepoRoot 'reports\runtime-activation-20260730'
$Gate9Marker = Join-Path $LogRoot 'gate9-runtime-contract.json'
$MarkerPath = Join-Path $LogRoot 'gate10-synthetic-success.json'
. (Join-Path $PSScriptRoot 'forensic_runtime_common.ps1')

Assert-ForensicMarker $Gate9Marker 'Gate 9'
$runtime = Get-ForensicRuntimeEnvironment $RuntimeEnvPath
$collection = 'nexusai-runtime-acceptance-20260730'
$case = 'runtime-acceptance-20260730'
$fixture = Join-Path $RepoRoot 'ingestion\forensic_records\tests\fixtures\pakistan_cdr_messy_synthetic.csv'
if (-not (Test-Path -LiteralPath $fixture)) { throw 'Synthetic CDR fixture is missing' }
if ($fixture -match '923461678183|records-demo-verified') { throw 'Gate 10 cannot use retained or real evidence' }
$headers = @{
  Authorization = "Bearer $($runtime.FORENSIC_RECORDS_API_KEY)"
  'X-Forensic-Tenant-ID' = $runtime.FORENSIC_RECORDS_TENANT_ID
  'X-Forensic-Actor-ID' = 'nexusai-runtime-operator'
  'X-Forensic-Subject-ID' = 'nexusai-runtime-operator'
  'X-Forensic-Actor-Role' = 'admin'
  'X-Forensic-Collection-ID' = $collection
  'X-Forensic-Case-ID' = $case
}

function Submit-SyntheticCDR {
  $rawWithStatus = @(& curl.exe -sS -X POST 'http://localhost:8091/webhooks/records/upload' `
    -H "Authorization: Bearer $($runtime.FORENSIC_RECORDS_API_KEY)" `
    -H "X-Forensic-Tenant-ID: $($runtime.FORENSIC_RECORDS_TENANT_ID)" `
    -H 'X-Forensic-Actor-ID: nexusai-runtime-operator' `
    -H 'X-Forensic-Subject-ID: nexusai-runtime-operator' `
    -H 'X-Forensic-Actor-Role: admin' `
    -H "X-Forensic-Collection-ID: $collection" `
    -H "X-Forensic-Case-ID: $case" `
    -F "tenant_id=$($runtime.FORENSIC_RECORDS_TENANT_ID)" `
    -F "collection_id=$collection" -F "case_id=$case" -F 'record_type=auto' `
    -F 'skip_kb_mirror=true' -F "file=@$fixture" -w "`n%{http_code}")
  if ($LASTEXITCODE -ne 0) { throw 'Synthetic upload transport failed' }
  if ($rawWithStatus.Count -lt 2) { throw 'Synthetic upload returned no HTTP status' }
  $statusCode = [int]$rawWithStatus[-1]
  if ($statusCode -lt 200 -or $statusCode -ge 300) {
    throw "Synthetic upload HTTP request failed with status $statusCode"
  }
  $raw = ($rawWithStatus[0..($rawWithStatus.Count - 2)] -join "`n")
  return ($raw | ConvertFrom-Json)
}

$upload = Submit-SyntheticCDR
$evidenceID = if ($upload.evidence_id) { $upload.evidence_id } elseif ($upload.existing.evidence_id) { $upload.existing.evidence_id } else { $null }
if (-not $evidenceID) { throw 'Synthetic upload did not return an evidence identity' }
$deadline = (Get-Date).AddMinutes(3)
do {
  Start-Sleep -Seconds 2
  $detail = Invoke-RestMethod -TimeoutSec 15 -Uri "http://localhost:8091/evidence/$evidenceID" -Headers $headers
  $state = $detail.item.processing_status
} until ($state -in @('completed','failed') -or (Get-Date) -gt $deadline)
$linkedReprocessing = $false
if ($state -eq 'failed') {
  $failedJob = @($detail.ingest_jobs)[0]
  if ($failedJob.status -ne 'dead_letter' -or [int]$failedJob.attempt_count -lt 2) {
    throw 'Synthetic failure did not preserve bounded retry/dead-letter history'
  }
  $reprocessHeaders = $headers.Clone()
  $reprocessHeaders['Idempotency-Key'] = 'runtime-acceptance-repair-20260730-v1'
  $reprocess = Invoke-RestMethod -Method Post -TimeoutSec 30 `
    -Uri "http://localhost:8091/evidence/$evidenceID/reprocess" `
    -Headers $reprocessHeaders -ContentType 'application/json' -Body (@{
      tenant_id = $runtime.FORENSIC_RECORDS_TENANT_ID
      collection_id = $collection
      case_id = $case
      reason = 'runtime acceptance reprocess after corrected disposable failure path'
      idempotency_key = 'runtime-acceptance-repair-20260730-v1'
      max_attempts = 5
    } | ConvertTo-Json -Compress)
  if (-not $reprocess.job_id -or [int]$reprocess.reprocess_generation -lt 1) {
    throw 'Linked reprocessing did not return a new processing generation'
  }
  $linkedReprocessing = $true
  $deadline = (Get-Date).AddMinutes(3)
  do {
    Start-Sleep -Seconds 2
    $detail = Invoke-RestMethod -TimeoutSec 15 -Uri "http://localhost:8091/evidence/$evidenceID" -Headers $headers
    $state = $detail.item.processing_status
  } until ($state -eq 'completed' -or (Get-Date) -gt $deadline)
}
if ($state -ne 'completed') { throw "Synthetic evidence ended in state '$state'" }
$job = @($detail.ingest_jobs)[0]
if ([int]$job.total_rows -ne 5 -or [int]$job.accepted_rows -ne 4 -or [int]$job.duplicate_rows -ne 1 -or [int]$job.rejected_rows -ne 0) {
  throw 'Synthetic CDR accounting did not match the five-row golden fixture'
}

$duplicate = Submit-SyntheticCDR
if ($duplicate.status -ne 'duplicate') { throw 'Repeat synthetic upload did not take the idempotent duplicate path' }

Push-Location $RepoRoot
try {
  & .\scripts\smoke_forensic_records.ps1 `
    -CollectionId $collection -TenantId $runtime.FORENSIC_RECORDS_TENANT_ID `
    -ApiKey $runtime.FORENSIC_RECORDS_API_KEY -ActorId 'nexusai-runtime-operator' `
    -SubjectId 'nexusai-runtime-operator' -ActorRole admin -CaseId $case `
    -Target '923001234567' -SkipReport
  if ($LASTEXITCODE -ne 0) { throw 'Authenticated forensic query smoke failed' }
} finally { Pop-Location }

[ordered]@{
  gate = 10; completed_utc = (Get-Date).ToUniversalTime().ToString('o')
  collection = $collection; fixture = 'pakistan_cdr_messy_synthetic.csv'
  total_rows = 5; accepted_unique_rows = 4; duplicate_rows = 1; rejected_rows = 0
  idempotent_repeat = 'pass'; authenticated_query_smoke = 'pass'; real_evidence_used = $false
  linked_reprocessing = if ($linkedReprocessing) { 'pass' } else { 'not_required' }
} | ConvertTo-Json | Set-Content -Encoding ascii -LiteralPath $MarkerPath

$reprocessResult = if ($linkedReprocessing) { 'PASS' } else { 'NOT_REQUIRED' }
Write-Host "ActivationGate10=PASS SyntheticRows=5 AcceptedUnique=4 DuplicateRows=1 Rejected=0 Idempotency=PASS LinkedReprocess=$reprocessResult QuerySmoke=PASS RealEvidenceUsed=false"
