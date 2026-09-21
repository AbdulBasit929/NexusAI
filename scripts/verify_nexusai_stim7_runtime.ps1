param(
  [string]$CollectionId = 'nexusai-runtime-acceptance-20260730',
  [string]$CaseId = 'runtime-acceptance-20260730',
  [string]$RecordsAPIUrl = 'http://localhost:8091',
  [string]$ReportPath = ''
)

$ErrorActionPreference = 'Stop'

$repoRoot = Split-Path -Parent $PSScriptRoot
. (Join-Path $PSScriptRoot 'forensic_runtime_common.ps1')
$runtime = Get-ForensicRuntimeEnvironment (Join-Path $repoRoot '.env.forensic-runtime.local')
$baseUrl = $RecordsAPIUrl.TrimEnd('/')
$oracle = Get-Content -Raw -LiteralPath (Join-Path $repoRoot 'api\forensic_records\contracts\stim4-runtime-acceptance-oracle-v1.json') | ConvertFrom-Json
$results = New-Object System.Collections.Generic.List[object]

$headers = @{
  Authorization = "Bearer $($runtime.FORENSIC_RECORDS_API_KEY)"
  'X-Forensic-Tenant-ID' = $runtime.FORENSIC_RECORDS_TENANT_ID
  'X-Forensic-Actor-ID' = 'nexusai-stim7-acceptance'
  'X-Forensic-Subject-ID' = 'nexusai-stim7-acceptance'
  'X-Forensic-Actor-Role' = 'admin'
  'X-Forensic-Collection-ID' = $CollectionId
  'X-Forensic-Case-ID' = $CaseId
}

function Add-STIM7Check {
  param(
    [Parameter(Mandatory=$true)][string]$Name,
    [Parameter(Mandatory=$true)][bool]$Passed,
    [Parameter(Mandatory=$true)][string]$Detail,
    [double]$LatencyMS = 0
  )
  $script:results.Add([pscustomobject][ordered]@{
    name = $Name
    passed = $Passed
    detail = $Detail
    latency_ms = [math]::Round($LatencyMS, 1)
  })
}

function Invoke-STIM7Query {
  param(
    [Parameter(Mandatory=$true)][string]$Template,
    [Parameter(Mandatory=$true)][string]$Target,
    [object]$SourceSet = $null
  )
  $body = [ordered]@{
    collection_id = $CollectionId
    case_id = $CaseId
    query = $Target
    template = $Template
    target = $Target
    limit = 100
    max_kb_results = 0
  }
  if ($null -ne $SourceSet) { $body.source_set = $SourceSet }
  $started = Get-Date
  $response = Invoke-RestMethod -Method Post -TimeoutSec 20 -Headers $headers `
    -ContentType 'application/json; charset=utf-8' -Uri "$baseUrl/query/hybrid" `
    -Body ($body | ConvertTo-Json -Depth 20 -Compress)
  return [pscustomobject]@{
    response = $response
    latency_ms = ((Get-Date) - $started).TotalMilliseconds
  }
}

function Test-DirectCitation {
  param([Parameter(Mandatory=$true)]$Item)
  return -not [string]::IsNullOrWhiteSpace([string]$Item.evidence_id) -and
    -not [string]::IsNullOrWhiteSpace([string]$Item.version_id) -and
    -not [string]::IsNullOrWhiteSpace([string]$Item.source_file) -and
    [int]$Item.row_number -gt 0 -and
    -not [string]::IsNullOrWhiteSpace([string]$Item.row_hash)
}

$expectedEvidence = @(
  @{ file='cdr_alpha.csv'; family='cdr'; source='b010666a-944d-4ab7-887a-e81f01f14622'; evidence='e6397cf8-68e4-4cc4-a3e6-d393003edb68'; version='78fc038b-b2c4-4a40-85f5-77f07fa2db5e'; total=4; accepted=3; duplicate=1; rejected=0 },
  @{ file='cdr_bravo.csv'; family='cdr'; source='26e360b8-b72c-42ed-9779-5f8d20ec497f'; evidence='5eea84ae-fa94-4300-8863-49dad4f11e9b'; version='762afcb9-291a-4033-9678-c2609ec7f02b'; total=2; accepted=2; duplicate=0; rejected=0 },
  @{ file='cdr_charlie.tsv'; family='cdr'; source='6f746d3a-1c21-4b80-ae09-3da7928f7e83'; evidence='3e09a86b-1e82-4226-ab31-2e9dd8901a12'; version='bbaa6878-1135-4045-929b-691b0e4680fa'; total=2; accepted=2; duplicate=0; rejected=0 },
  @{ file='ipdr.csv'; family='ipdr'; source='b263d097-4153-4aa2-a91b-48269b0e8805'; evidence='aab440fe-3d03-4f30-a894-4b169b449c39'; version='fdbce0ec-3f41-4989-9854-923972eaac2e'; total=3; accepted=3; duplicate=0; rejected=0 },
  @{ file='subscriber.jsonl'; family='subscriber'; source='21cdd3f0-b832-4da1-9931-254013e634bc'; evidence='5fdbc11c-c5dd-4b08-9d8a-397db861ca9e'; version='af1e2f57-d696-474b-92b2-fd493fe43897'; total=4; accepted=4; duplicate=0; rejected=0 },
  @{ file='anpr_east.csv'; family='anpr'; source='1e44404d-0666-41ef-861b-98677a9719f6'; evidence='4c679639-f3e3-4d97-b830-990af202382e'; version='600aa04c-7876-4e59-90f9-8a5f7172c6cc'; total=3; accepted=3; duplicate=0; rejected=0 },
  @{ file='anpr_west.csv'; family='anpr'; source='88f46b4d-a669-48eb-8cf6-a37795e33176'; evidence='15762aea-3db3-4a96-950c-667714fed474'; version='06f1fdcf-f830-4353-9f29-688bcb79e62c'; total=3; accepted=3; duplicate=0; rejected=0 }
)

try {
  $started = Get-Date
  $inventory = Invoke-RestMethod -TimeoutSec 20 -Headers $headers -Uri "$baseUrl/evidence?collection_id=$([uri]::EscapeDataString($CollectionId))&limit=100"
  $completed = @($inventory.items | Where-Object { $_.processing_status -eq 'completed' })
  $identityProblems = New-Object System.Collections.Generic.List[string]
  foreach ($expected in $expectedEvidence) {
    $matches = @($completed | Where-Object {
      $_.source_file -eq $expected.file -and $_.detected_type -eq $expected.family -and
      $_.evidence_id -eq $expected.evidence -and $_.metadata.file_id -eq $expected.source -and
      $_.metadata.version_id -eq $expected.version -and [int]$_.total_rows -eq $expected.total -and
      [int]$_.accepted_rows -eq $expected.accepted -and
      [int]$_.metadata.structured_summary.duplicate_rows -eq $expected.duplicate -and
      [int]$_.metadata.structured_summary.rejected_rows -eq $expected.rejected
    })
    if ($matches.Count -ne 1) { $identityProblems.Add($expected.file) }
  }
  $totalRows = 0
  $acceptedRows = 0
  $duplicateRows = 0
  $rejectedRows = 0
  foreach ($expected in $expectedEvidence) {
    $totalRows += [int]$expected.total
    $acceptedRows += [int]$expected.accepted
    $duplicateRows += [int]$expected.duplicate
    $rejectedRows += [int]$expected.rejected
  }
  $passed = $identityProblems.Count -eq 0 -and $completed.Count -eq 7 -and $totalRows -eq 21 -and $acceptedRows -eq 20 -and $duplicateRows -eq 1 -and $rejectedRows -eq 0
  Add-STIM7Check 'retained_evidence.identity_and_accounting' $passed "completed=$($completed.Count); total=21; accepted=20; duplicate=1; rejected=0; mismatches=$($identityProblems -join ',')" (((Get-Date) - $started).TotalMilliseconds)
} catch {
  Add-STIM7Check 'retained_evidence.identity_and_accounting' $false $_.Exception.Message
}

$cdrSourceSet = [ordered]@{
  contract_version = 'forensics.structured-source-set/v1'
  sources = @($expectedEvidence | Where-Object { $_.family -eq 'cdr' } | ForEach-Object {
    [ordered]@{ source_id=$_.source; source_file=$_.file; evidence_id=$_.evidence; version_id=$_.version }
  })
}

try {
  $multi = Invoke-STIM7Query 'multi_cdr_comparison' '923001234567' $cdrSourceSet
  $records = $multi.response.records
  $json = $records | ConvertTo-Json -Depth 30 -Compress
  $requiredValues = @('923110000001','923220000002','923330000003','923440000004','490154203237518','410010123456789','LHR-001','2026-08-18T05:00:00Z','observed_association','exact_event_signature','source_disagreement')
  $missing = @($requiredValues | Where-Object { $json -notmatch [regex]::Escape($_) })
  $citations = @($multi.response.enterprise.provenance | Where-Object { Test-DirectCitation $_ })
  $passed = $multi.response.template -eq 'multi_cdr_comparison' -and @($multi.response.route) -contains 'records_sql' -and
    [int]$records.source_count -eq 3 -and [int]$records.matched_event_count -eq 7 -and
    $missing.Count -eq 0 -and $citations.Count -gt 0 -and $json -match 'does not establish'
  Add-STIM7Check 'multi_cdr.independent_oracle' $passed "sources=$($records.source_count); events=$($records.matched_event_count); missing=$($missing -join ','); direct_citations=$($citations.Count)" $multi.latency_ms
} catch {
  Add-STIM7Check 'multi_cdr.independent_oracle' $false $_.Exception.Message
}

try {
  $tampered = $cdrSourceSet | ConvertTo-Json -Depth 20 | ConvertFrom-Json
  $tampered.sources[0].version_id = '00000000-0000-0000-0000-000000000000'
  $body = @{ collection_id=$CollectionId; case_id=$CaseId; query='923001234567'; template='multi_cdr_comparison'; target='923001234567'; source_set=$tampered; limit=25; max_kb_results=0 } | ConvertTo-Json -Depth 20 -Compress
  $started = Get-Date
  $status = 0
  $errorPayload = $null
  try {
    Invoke-WebRequest -UseBasicParsing -TimeoutSec 20 -Method Post -Headers $headers -ContentType 'application/json' -Uri "$baseUrl/query/hybrid" -Body $body | Out-Null
  } catch {
    $status = [int]$_.Exception.Response.StatusCode
    $errorPayload = $_.ErrorDetails.Message | ConvertFrom-Json
  }
  $passed = $status -eq 400 -and $errorPayload.error_code -eq 'invalid_source_membership' -and $errorPayload.error -eq 'selected source set is invalid for the authorized collection'
  Add-STIM7Check 'multi_cdr.tampered_membership_rejected' $passed "http=$status; code=$($errorPayload.error_code); stable_error=$($errorPayload.error)" (((Get-Date) - $started).TotalMilliseconds)
} catch {
  Add-STIM7Check 'multi_cdr.tampered_membership_rejected' $false $_.Exception.Message
}

$crossFamilyCases = @(
  @{ name='positive'; target='923001234567'; families=@('cdr','ipdr','subscriber'); matches=4 },
  @{ name='future_invalid'; target='923008888888'; families=@('ipdr','subscriber'); matches=2 },
  @{ name='wrong_entity_type'; target='10.0.0.10'; families=@('ipdr','subscriber'); matches=2 },
  @{ name='no_subscriber_match'; target='923007777777'; families=@('ipdr'); matches=1 },
  @{ name='no_ipdr_match'; target='923006666666'; families=@('subscriber'); matches=1 }
)
foreach ($case in $crossFamilyCases) {
  try {
    $query = Invoke-STIM7Query 'cross_family_correlation' $case.target
    $actualFamilies = @($query.response.records.family_coverage | ForEach-Object { [string]$_.record_type } | Sort-Object)
    $expectedFamilies = @($case.families | Sort-Object)
    $matches = @($query.response.records.target_matches)
    $citationsPassed = $matches.Count -gt 0 -and @($matches | Where-Object { -not (Test-DirectCitation $_.citation_locator) }).Count -eq 0
    $passed = $query.response.template -eq 'cross_family_correlation' -and @($query.response.route) -contains 'records_sql' -and
      ($actualFamilies -join ',') -eq ($expectedFamilies -join ',') -and
      [int]$query.response.records.matched_record_count -eq $case.matches -and $citationsPassed -and
      -not [bool]$query.response.records.relations_may_be_truncated
    Add-STIM7Check "cross_family.$($case.name)" $passed "target=$($case.target); families=$($actualFamilies -join ','); matches=$($query.response.records.matched_record_count); citations=$citationsPassed" $query.latency_ms
  } catch {
    Add-STIM7Check "cross_family.$($case.name)" $false $_.Exception.Message
  }
}

try {
  $sequence = Invoke-STIM7Query 'anpr_camera_sequence' 'STIM404'
  $rows = @($sequence.response.records.anpr_camera_sequence)
  $passed = $rows.Count -eq 2 -and ($rows.camera_id -join ',') -eq 'STIM-CAM-EAST,STIM-CAM-WEST' -and
    (@($rows.source_file | Sort-Object) -join ',') -eq 'anpr_east.csv,anpr_west.csv' -and
    @($rows | Where-Object { -not (Test-DirectCitation $_) }).Count -eq 0
  Add-STIM7Check 'anpr.cross_source_sequence' $passed "rows=$($rows.Count); cameras=$($rows.camera_id -join ','); sources=$($rows.source_file -join ',')" $sequence.latency_ms
} catch {
  Add-STIM7Check 'anpr.cross_source_sequence' $false $_.Exception.Message
}

try {
  $timing = Invoke-STIM7Query 'anpr_route_timing' 'STIM404'
  $rows = @($timing.response.records.anpr_route_timing)
  $passed = $rows.Count -eq 1 -and [int]$rows[0].elapsed_seconds -eq [int]$oracle.expectations.cross_source_anpr.elapsed_seconds -and
    $rows[0].from_camera_id -eq 'STIM-CAM-EAST' -and $rows[0].to_camera_id -eq 'STIM-CAM-WEST' -and
    (Test-DirectCitation $rows[0])
  Add-STIM7Check 'anpr.route_timing' $passed "rows=$($rows.Count); elapsed_seconds=$($rows[0].elapsed_seconds); route=$($rows[0].from_camera_id)->$($rows[0].to_camera_id)" $timing.latency_ms
} catch {
  Add-STIM7Check 'anpr.route_timing' $false $_.Exception.Message
}

try {
  $east = Invoke-STIM7Query 'anpr_camera_sequence' 'LOOK777'
  $west = Invoke-STIM7Query 'anpr_camera_sequence' 'LOOK778'
  $eastRows = @($east.response.records.anpr_camera_sequence)
  $westRows = @($west.response.records.anpr_camera_sequence)
  $passed = $eastRows.Count -eq 1 -and $westRows.Count -eq 1 -and
    $eastRows[0].source_file -eq 'anpr_east.csv' -and $westRows[0].source_file -eq 'anpr_west.csv' -and
    $eastRows[0].plate_number -ne $westRows[0].plate_number
  Add-STIM7Check 'anpr.lookalike_non_match' $passed "LOOK777=$($eastRows.Count):$($eastRows[0].source_file); LOOK778=$($westRows.Count):$($westRows[0].source_file)" ($east.latency_ms + $west.latency_ms)
} catch {
  Add-STIM7Check 'anpr.lookalike_non_match' $false $_.Exception.Message
}

$failures = @($results | Where-Object { -not $_.passed })
$report = [ordered]@{
  contract_version = 'nexusai.stim7-runtime-oracles/v1'
  generated_utc = (Get-Date).ToUniversalTime().ToString('o')
  collection_id = $CollectionId
  case_id = $CaseId
  oracle_contract = 'api/forensic_records/contracts/stim4-runtime-acceptance-oracle-v1.json'
  database_mutated = $false
  models_changed = $false
  checks = $results.ToArray()
  passed = $failures.Count -eq 0
}

if (-not $ReportPath) {
  $reportRoot = Join-Path $repoRoot 'reports\runtime-activation-stim7'
  New-Item -ItemType Directory -Force -Path $reportRoot | Out-Null
  $ReportPath = Join-Path $reportRoot ("stim7-runtime-oracles-{0}.json" -f (Get-Date).ToUniversalTime().ToString('yyyyMMddTHHmmssZ'))
}
$fullReportPath = [System.IO.Path]::GetFullPath($ReportPath)
$reportParent = Split-Path -Parent $fullReportPath
if ($reportParent) { New-Item -ItemType Directory -Force -Path $reportParent | Out-Null }
$report | ConvertTo-Json -Depth 30 | Set-Content -Encoding utf8 -LiteralPath $fullReportPath
$report | ConvertTo-Json -Depth 30

if ($failures.Count -gt 0) {
  throw "STIM-7 runtime oracle verification failed $($failures.Count) check(s); report=$fullReportPath"
}
Write-Host "STIM7RuntimeOracles=PASS Checks=$($results.Count) Failures=0 DatabaseMutated=false ModelsChanged=false Report=$fullReportPath"
