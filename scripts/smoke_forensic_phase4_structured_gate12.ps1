param(
  [string]$CollectionId = 'nexusai-structured-demo-v2-20260730',
  [string]$CaseId = 'structured-demo-20260730',
  [string]$PythonPath = "$env:LOCALAPPDATA\Programs\Python\Python313\python.exe"
)

$ErrorActionPreference = 'Stop'
$RepoRoot = Split-Path -Parent $PSScriptRoot
$RuntimeEnvPath = Join-Path $RepoRoot '.env.forensic-runtime.local'
$LogRoot = Join-Path $RepoRoot 'reports\runtime-activation-20260730'
$Gate11Marker = Join-Path $LogRoot 'gate11-redelivery.json'
$MarkerPath = Join-Path $LogRoot 'gate12-phase4-structured.json'
$FixtureRoot = Join-Path $RepoRoot 'ingestion\forensic_records\tests\fixtures'
$GoldenPath = Join-Path $FixtureRoot 'pakistan_phase4_structured_goldens_v1.json'
$XlsxPath = Join-Path $RepoRoot '.tmp\phase4\pakistan_mixed_provider_synthetic.xlsx'
. (Join-Path $PSScriptRoot 'forensic_runtime_common.ps1')

Assert-ForensicMarker $Gate11Marker 'Gate 11'
$runtime = Get-ForensicRuntimeEnvironment $RuntimeEnvPath
if (-not (Test-Path -LiteralPath $PythonPath)) { throw 'Python 3.13 is required for the synthetic XLSX generator' }
& $PythonPath (Join-Path $PSScriptRoot 'generate_forensic_phase4_xlsx_fixture.py') $XlsxPath
if ($LASTEXITCODE -ne 0 -or -not (Test-Path -LiteralPath $XlsxPath)) { throw 'Synthetic XLSX fixture generation failed' }

$headers = @{
  Authorization = "Bearer $($runtime.FORENSIC_RECORDS_API_KEY)"
  'X-Forensic-Tenant-ID' = $runtime.FORENSIC_RECORDS_TENANT_ID
  'X-Forensic-Actor-ID' = 'nexusai-phase4-operator'
  'X-Forensic-Subject-ID' = 'nexusai-phase4-operator'
  'X-Forensic-Actor-Role' = 'admin'
  'X-Forensic-Collection-ID' = $CollectionId
  'X-Forensic-Case-ID' = $CaseId
}

function Assert-Condition {
  param([bool]$Condition, [string]$Message)
  if (-not $Condition) { throw $Message }
}

function Ensure-LocalAICollection {
  $catalog = Invoke-RestMethod -TimeoutSec 30 -Uri 'http://localhost:8080/api/agents/collections'
  if (@($catalog.collections) -notcontains $CollectionId) {
    Invoke-RestMethod -Method Post -TimeoutSec 30 -Uri 'http://localhost:8080/api/agents/collections' `
      -ContentType 'application/json' -Body (@{name=$CollectionId} | ConvertTo-Json -Compress) | Out-Null
  }
}

function Submit-SyntheticEvidence {
  param([string]$Path, [string]$RecordType)
  if (-not (Test-Path -LiteralPath $Path)) { throw "Synthetic fixture is missing: $Path" }
  if ($Path -match '923461678183|records-demo-verified') { throw 'Gate 12 cannot use retained or real evidence' }
  $rawWithStatus = @(& curl.exe -sS -X POST 'http://localhost:8091/webhooks/records/upload' `
    -H "Authorization: Bearer $($runtime.FORENSIC_RECORDS_API_KEY)" `
    -H "X-Forensic-Tenant-ID: $($runtime.FORENSIC_RECORDS_TENANT_ID)" `
    -H 'X-Forensic-Actor-ID: nexusai-phase4-operator' `
    -H 'X-Forensic-Subject-ID: nexusai-phase4-operator' `
    -H 'X-Forensic-Actor-Role: admin' `
    -H "X-Forensic-Collection-ID: $CollectionId" `
    -H "X-Forensic-Case-ID: $CaseId" `
    -F "tenant_id=$($runtime.FORENSIC_RECORDS_TENANT_ID)" `
    -F "collection_id=$CollectionId" -F "case_id=$CaseId" `
    -F "record_type=$RecordType" -F 'skip_kb_mirror=true' `
    -F 'jurisdiction=PK' -F 'source_timezone=Asia/Karachi' `
    -F "file=@$Path" -w "`n%{http_code}")
  if ($LASTEXITCODE -ne 0 -or $rawWithStatus.Count -lt 2) { throw "Upload transport failed for $([IO.Path]::GetFileName($Path))" }
  $statusCode = [int]$rawWithStatus[-1]
  Assert-Condition ($statusCode -ge 200 -and $statusCode -lt 300) "Upload HTTP $statusCode for $([IO.Path]::GetFileName($Path))"
  $payload = ($rawWithStatus[0..($rawWithStatus.Count - 2)] -join "`n") | ConvertFrom-Json
  $evidenceId = if ($payload.evidence_id) { $payload.evidence_id } else { $payload.existing.evidence_id }
  Assert-Condition ([bool]$evidenceId) "Upload returned no evidence identity for $([IO.Path]::GetFileName($Path))"
  return [pscustomobject]@{ EvidenceId=$evidenceId; Response=$payload }
}

function Wait-ForGoldenAccounting {
  param($Item)
  $deadline = (Get-Date).AddMinutes(5)
  do {
    Start-Sleep -Seconds 2
    $detailResponse = Invoke-WebRequest -UseBasicParsing -TimeoutSec 20 -Headers $headers -Uri "http://localhost:8091/evidence/$($Item.EvidenceId)?limit=1&include_records_preview=false&view=accounting"
    $detail = $detailResponse.Content | ConvertFrom-Json
    $completed = @($detail.ingest_jobs | Where-Object { $_.status -eq 'completed' -and [int]$_.total_rows -eq $Item.TotalRows })
    $dead = @($detail.ingest_jobs | Where-Object { $_.status -eq 'dead_letter' })
    if ($dead.Count -gt 0 -and $completed.Count -eq 0) { throw "Synthetic processing dead-lettered for $($Item.Name)" }
  } until ($completed.Count -gt 0 -or (Get-Date) -gt $deadline)
  Assert-Condition ($completed.Count -gt 0) "Timed out waiting for $($Item.Name)"
  $job = $completed | Select-Object -First 1
  Assert-Condition ([int]$job.accepted_rows -eq $Item.UniqueRows) "$($Item.Name) accepted-row mismatch"
  Assert-Condition ([int]$job.duplicate_rows -eq $Item.DuplicateRows) "$($Item.Name) duplicate-row mismatch"
  Assert-Condition ([int]$job.rejected_rows -eq $Item.RejectedRows) "$($Item.Name) rejected-row mismatch"
  return [pscustomobject]@{
    family=$Item.RecordType; format=$Item.Format; total_rows=[int]$job.total_rows
    accepted_unique_rows=[int]$job.accepted_rows; duplicate_rows=[int]$job.duplicate_rows
    rejected_rows=[int]$job.rejected_rows; status='pass'
  }
}

Ensure-LocalAICollection
$golden = Get-Content -Raw -LiteralPath $GoldenPath | ConvertFrom-Json
$items = @($golden.acceptance_matrix | ForEach-Object {
  [pscustomobject]@{
    Name=$_.path; Path=(Join-Path $FixtureRoot $_.path); RecordType=$_.record_type
    Format=[IO.Path]::GetExtension($_.path).TrimStart('.'); TotalRows=[int]$_.total_rows
    UniqueRows=[int]$_.unique_normalized_rows; DuplicateRows=[int]$_.duplicate_rows
    RejectedRows=[int]$_.rejected_rows
  }
})
$items += [pscustomobject]@{
  Name='pakistan_cdr_tsv_synthetic.tsv'; Path=(Join-Path $FixtureRoot 'pakistan_cdr_tsv_synthetic.tsv')
  RecordType='cdr'; Format='tsv'; TotalRows=3; UniqueRows=2; DuplicateRows=1; RejectedRows=0
}
$items += [pscustomobject]@{
  Name='pakistan_mixed_provider_synthetic.xlsx'; Path=$XlsxPath
  RecordType='auto'; Format='xlsx'; TotalRows=4; UniqueRows=4; DuplicateRows=0; RejectedRows=0
}

$submitted = foreach ($item in $items) {
  $upload = Submit-SyntheticEvidence -Path $item.Path -RecordType $item.RecordType
  Add-Member -InputObject $item -NotePropertyName EvidenceId -NotePropertyValue $upload.EvidenceId
  $item
}
$accounting = @($submitted | ForEach-Object { Wait-ForGoldenAccounting $_ })

$capabilities = Invoke-RestMethod -TimeoutSec 20 -Headers $headers `
  -Uri "http://localhost:8091/query/capabilities?collection_id=$([uri]::EscapeDataString($CollectionId))"
Assert-Condition (@($capabilities.families).Count -eq 20) 'Capability contract did not return 20 families'
$requiredFamilyIDs = @('cdr','ipdr_network_sessions','anpr_vehicle_sightings','subscriber_identity','tower_location','financial_transactions','logs_access_security','generic_tabular','spreadsheets_and_columnar')
foreach ($familyID in $requiredFamilyIDs) {
  $family = @($capabilities.families | Where-Object { $_.id -eq $familyID })
  Assert-Condition ($family.Count -eq 1) "Capability family missing: $familyID"
  Assert-Condition ($family[0].availability -eq 'queryable') "Capability family is not queryable: $familyID"
}

$queryBody = @{
  tenant_id=$runtime.FORENSIC_RECORDS_TENANT_ID; collection_id=$CollectionId
  case_id=$CaseId; query='correlate 35678901234567 across record families'
  template='cross_family_correlation'; target='35678901234567'; limit=25; max_kb_results=0
} | ConvertTo-Json -Depth 8 -Compress
$correlation = Invoke-RestMethod -Method Post -TimeoutSec 30 -Headers $headers `
  -Uri 'http://localhost:8091/query/hybrid' -ContentType 'application/json' -Body $queryBody
Assert-Condition ($correlation.template -eq 'cross_family_correlation') 'Cross-family query routed incorrectly'
Assert-Condition ($correlation.route -contains 'records_sql') 'Cross-family query did not use deterministic SQL'
Assert-Condition ([int]$correlation.answer.records_row_count -ge 2) 'Cross-family query returned fewer than two cited rows'

$sourceAuditBody = @{
  tenant_id=$runtime.FORENSIC_RECORDS_TENANT_ID; collection_id=$CollectionId
  case_id=$CaseId; query='which files were ingested?'; template='source_file_audit'; limit=25; max_kb_results=0
} | ConvertTo-Json -Depth 8 -Compress
$sourceAudit = Invoke-RestMethod -Method Post -TimeoutSec 30 -Headers $headers `
  -Uri 'http://localhost:8091/query/hybrid' -ContentType 'application/json' -Body $sourceAuditBody
Assert-Condition ($sourceAudit.template -eq 'source_file_audit') 'Source audit routed incorrectly'
Assert-Condition ([int]$sourceAudit.answer.records_row_count -ge $items.Count) 'Source audit omitted synthetic evidence files'

[ordered]@{
  gate=12; completed_utc=(Get-Date).ToUniversalTime().ToString('o'); collection=$CollectionId
  synthetic_files=$items.Count; families_exercised=9; total_rows=($accounting | Measure-Object total_rows -Sum).Sum
  accepted_unique_rows=($accounting | Measure-Object accepted_unique_rows -Sum).Sum
  duplicate_rows=($accounting | Measure-Object duplicate_rows -Sum).Sum
  rejected_rows=($accounting | Measure-Object rejected_rows -Sum).Sum
  capability_families=20; queryable_required_families=$requiredFamilyIDs.Count
  cross_family_sql='pass'; source_audit='pass'; localai_collection_registered='pass'
  real_evidence_used=$false; accounting=$accounting
} | ConvertTo-Json -Depth 8 | Set-Content -Encoding ascii -LiteralPath $MarkerPath

Write-Host "Phase4Gate12=PASS SyntheticFiles=$($items.Count) Families=9 TotalRows=$(($accounting | Measure-Object total_rows -Sum).Sum) AcceptedUnique=$(($accounting | Measure-Object accepted_unique_rows -Sum).Sum) DuplicateRows=$(($accounting | Measure-Object duplicate_rows -Sum).Sum) Rejected=$(($accounting | Measure-Object rejected_rows -Sum).Sum) Capabilities=20 CrossFamily=PASS SourceAudit=PASS RealEvidenceUsed=false"
