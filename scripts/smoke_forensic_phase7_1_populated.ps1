param(
  [string]$CollectionId = 'phase7-subscriber-populated-acceptance-v1',
  [string]$EvidenceId = '1b1f27f7-d8cc-4dc1-b33e-758885e1ca8a',
  [switch]$RunModelSynthesis
)

$ErrorActionPreference = 'Stop'
$RepoRoot = Split-Path -Parent $PSScriptRoot
$RuntimeEnvPath = Join-Path $RepoRoot '.env.forensic-runtime.local'
$FixturePath = Join-Path $RepoRoot 'ingestion\forensic_records\tests\fixtures\pakistan_subscriber_populated_acceptance_v1.jsonl'
$MarkerPath = Join-Path $RepoRoot 'reports\runtime-activation-20260804\phase7.1-populated-acceptance.json'
$ExpectedFixtureSHA256 = 'A5EDC8F2658B12E8564B49560FE40959BE397DE6E086EAA1BFE2AA11BAD912B0'
$SynthesisModel = 'qwen_qwen3-4b-instruct-2507'
. (Join-Path $PSScriptRoot 'forensic_runtime_common.ps1')

function Assert-Condition {
  param([bool]$Condition, [string]$Message)
  if (-not $Condition) { throw $Message }
}

Assert-Condition ($CollectionId -eq 'phase7-subscriber-populated-acceptance-v1') 'This gate is restricted to the isolated Phase 7.1 synthetic collection'
Assert-Condition ($CollectionId -notin @('records-demo', 'records-demo-verified')) 'Governed demo collections are prohibited'
$actualFixtureSHA256 = (Get-FileHash -Algorithm SHA256 -LiteralPath $FixturePath).Hash
Assert-Condition ($actualFixtureSHA256 -eq $ExpectedFixtureSHA256) "Fixture hash mismatch: $actualFixtureSHA256"

$runtime = Get-ForensicRuntimeEnvironment $RuntimeEnvPath
$headers = @{
  Authorization = "Bearer $($runtime.FORENSIC_RECORDS_API_KEY)"
  'X-Forensic-Tenant-ID' = $runtime.FORENSIC_RECORDS_TENANT_ID
  'X-Forensic-Actor-ID' = 'nexusai-phase7-operator'
  'X-Forensic-Subject-ID' = 'nexusai-phase7-operator'
  'X-Forensic-Actor-Role' = 'admin'
  'X-Forensic-Collection-ID' = $CollectionId
  'X-Forensic-Case-ID' = $CollectionId
}

function Invoke-SubscriberQuery {
  param(
    [Parameter(Mandatory=$true)][string]$Template,
    [string]$Target = '',
    [int]$Limit = 100,
    [int]$Offset = 0,
    [string]$SynthesisModelName = ''
  )
  $body = @{
    tenant_id = $runtime.FORENSIC_RECORDS_TENANT_ID
    collection_id = $CollectionId
    case_id = $CollectionId
    query = $Template
    template = $Template
    target = $Target
    record_type = 'subscriber'
    limit = $Limit
    offset = $Offset
    max_kb_results = 0
  }
  if ($SynthesisModelName) { $body.synthesis_model = $SynthesisModelName }
  $started = Get-Date
  $response = Invoke-RestMethod -Method Post -TimeoutSec 180 -Headers $headers `
    -Uri 'http://localhost:8091/query/hybrid' -ContentType 'application/json' `
    -Body ($body | ConvertTo-Json -Depth 8 -Compress)
  $elapsed = [math]::Round(((Get-Date) - $started).TotalMilliseconds, 1)
  $serialized = $response | ConvertTo-Json -Depth 30 -Compress
  Assert-Condition ($serialized -notmatch '00000-\d{7}-\d') "$Template exposed a raw CNIC value"
  Assert-Condition ($serialized -notmatch '"(?:subscriber_name|full_name|customer_name|name)"\s*:\s*"') "$Template exposed a raw subscriber name value"
  return [pscustomobject]@{ Response=$response; LatencyMS=$elapsed }
}

$accounting = Invoke-RestMethod -TimeoutSec 30 -Headers $headers `
  -Uri "http://localhost:8091/evidence/${EvidenceId}?limit=25&include_records_preview=false&view=accounting"
$job = @($accounting.ingest_jobs | Where-Object { $_.status -eq 'completed' }) | Select-Object -First 1
Assert-Condition ([bool]$job) 'No completed Phase 7.1 ingest job was found'
Assert-Condition ([int]$job.total_rows -eq 15) 'Total-row accounting mismatch'
Assert-Condition ([int]$job.accepted_rows -eq 11) 'Accepted-row accounting mismatch'
Assert-Condition ([int]$job.duplicate_rows -eq 1) 'Duplicate-row accounting mismatch'
Assert-Condition ([int]$job.rejected_rows -eq 3) 'Rejected-row accounting mismatch'

$detailRaw = (Invoke-WebRequest -UseBasicParsing -TimeoutSec 30 -Headers $headers `
  -Uri "http://localhost:8091/evidence/${EvidenceId}?limit=25&include_records_preview=true").Content
Assert-Condition ($detailRaw -notmatch '00000-\d{7}-\d') 'Evidence preview exposed a raw CNIC value'
Assert-Condition ($detailRaw -notmatch '"(?:subscriber_name|full_name|customer_name|name)"\s*:\s*"') 'Evidence preview exposed a raw subscriber name value'
$maskedPreviewCount = [regex]::Matches($detailRaw, '\*{9}\d{4}').Count
Assert-Condition ($maskedPreviewCount -ge 11) 'Evidence preview omitted expected masked CNIC values'
$versionID = [regex]::Match($detailRaw, '"version_id":"([0-9a-f-]{36})"').Groups[1].Value
$batchID = [regex]::Match($detailRaw, '"records_batch_id":"([0-9a-f-]{36})"').Groups[1].Value
$jobID = [regex]::Match($detailRaw, '"job_id":"([0-9a-f-]{36})"').Groups[1].Value
$sourceSHA256 = [regex]::Match($detailRaw, '"sha256":"([0-9a-f]{64})"').Groups[1].Value
Assert-Condition ([bool]$versionID -and [bool]$batchID -and [bool]$jobID) 'Evidence lineage identifiers are incomplete'
Assert-Condition ($sourceSHA256.ToUpperInvariant() -eq $ExpectedFixtureSHA256) 'Retained evidence hash mismatch'

$identity = Invoke-SubscriberQuery -Template 'subscriber_identity_lookup' -Target 'PK-SUB-SYN-ALPHA'
$identityRows = @($identity.Response.records.subscriber_identity_lookup)
Assert-Condition ($identityRows.Count -eq 2) 'Identity lookup row count mismatch'
Assert-Condition ((@($identityRows.row_number | Sort-Object) -join ',') -eq '2,3') 'Identity lookup source-row mismatch'

$timeline = Invoke-SubscriberQuery -Template 'subscriber_validity_timeline' -Target 'PK-SUB-SYN-ALPHA'
$timelineRows = @($timeline.Response.records.subscriber_validity_timeline)
Assert-Condition ($timelineRows.Count -eq 2) 'Validity timeline row count mismatch'
Assert-Condition (@($timelineRows | Where-Object { $_.valid_to_at }).Count -eq 1) 'Validity timeline closed-window mismatch'
Assert-Condition (@($timelineRows | Where-Object { -not $_.valid_to_at }).Count -eq 1) 'Validity timeline open-window mismatch'

$devices = Invoke-SubscriberQuery -Template 'subscriber_device_links' -Target 'PK-SUB-SYN-ALPHA'
$deviceRows = @($devices.Response.records.subscriber_device_links)
Assert-Condition ($deviceRows.Count -eq 2) 'Device-link row count mismatch'
Assert-Condition (@($deviceRows.imsi | Sort-Object -Unique).Count -eq 1) 'Device-link IMSI count mismatch'
Assert-Condition (@($deviceRows.imei | Sort-Object -Unique).Count -eq 2) 'Device-link IMEI count mismatch'

$statuses = Invoke-SubscriberQuery -Template 'subscriber_status_summary'
$statusRows = @($statuses.Response.records.subscriber_status_summary)
$statusActual = @($statusRows | ForEach-Object { "$($_.subscriber_status)|$($_.manual_review_required)|$($_.row_count)" } | Sort-Object)
$statusExpected = @('ACTIVE|False|4','ACTIVE|True|2','CLOSED|False|1','INACTIVE|False|1','PENDING|True|1','SUSPENDED|False|2') | Sort-Object
Assert-Condition (($statusActual -join ',') -eq ($statusExpected -join ',')) 'Subscriber status groups mismatch'
Assert-Condition ((($statusRows | Measure-Object row_count -Sum).Sum) -eq 11) 'Subscriber status total mismatch'

$conflicts = Invoke-SubscriberQuery -Template 'subscriber_conflict_audit'
$conflictRows = @($conflicts.Response.records.subscriber_conflict_audit)
Assert-Condition ($conflictRows.Count -eq 2) 'Subscriber conflict row count mismatch'
Assert-Condition ((@($conflictRows.stable_identifier | Sort-Object) -join ',') -eq 'PK-SUB-SYN-ALPHA,PK-SUB-SYN-CONFLICT') 'Subscriber conflict identifiers mismatch'

$reuse = Invoke-SubscriberQuery -Template 'subscriber_reuse_candidates'
$reuseRows = @($reuse.Response.records.subscriber_reuse_candidates)
$reuseActual = @($reuseRows | ForEach-Object { "$($_.identifier_kind)|$($_.identifier_value)|$($_.distinct_msisdn_count)" } | Sort-Object)
$reuseExpected = @('imei|490154203237518|3','imsi|410030000000099|2','subscriber_reference|PK-SUB-SYN-CONFLICT|2') | Sort-Object
Assert-Condition (($reuseActual -join ',') -eq ($reuseExpected -join ',')) 'Subscriber reuse candidates mismatch'

$pageOne = Invoke-SubscriberQuery -Template 'subscriber_identity_lookup' -Target 'PK-SUB-SYN-ALPHA' -Limit 1 -Offset 0
$pageTwo = Invoke-SubscriberQuery -Template 'subscriber_identity_lookup' -Target 'PK-SUB-SYN-ALPHA' -Limit 1 -Offset 1
$pageOneRows = @($pageOne.Response.records.subscriber_identity_lookup)
$pageTwoRows = @($pageTwo.Response.records.subscriber_identity_lookup)
Assert-Condition ($pageOneRows.Count -eq 1 -and $pageTwoRows.Count -eq 1) 'Subscriber pagination did not enforce limit=1'
Assert-Condition ($pageOneRows[0].row_hash -ne $pageTwoRows[0].row_hash) 'Subscriber pagination returned the same row twice'

$rawCNIC = '00000-1000001-1'
$rejectedBody = @{
  tenant_id=$runtime.FORENSIC_RECORDS_TENANT_ID; collection_id=$CollectionId; case_id=$CollectionId
  query='subscriber identity lookup'; template='subscriber_identity_lookup'; target=$rawCNIC
  record_type='subscriber'; limit=25; max_kb_results=0; synthesis_model=$SynthesisModel
} | ConvertTo-Json -Depth 8 -Compress
$rejected = Invoke-RestMethod -Method Post -TimeoutSec 30 -Headers $headers `
  -Uri 'http://localhost:8091/query/hybrid' -ContentType 'application/json' -Body $rejectedBody
$rejectedJSON = $rejected | ConvertTo-Json -Depth 30 -Compress
Assert-Condition ($rejected.intent -eq 'clarification') 'Raw CNIC target did not require clarification'
Assert-Condition (@($rejected.route) -notcontains 'records_sql') 'Raw CNIC target executed SQL'
Assert-Condition (@($rejected.route) -notcontains 'llm_synthesis') 'Raw CNIC target executed model synthesis'
Assert-Condition ([int]$rejected.telemetry.db_latency_ms -eq 0 -and [int]$rejected.telemetry.llm_latency_ms -eq 0) 'Raw CNIC target consumed SQL or model latency'
Assert-Condition ($rejectedJSON -notmatch [regex]::Escape($rawCNIC)) 'Raw CNIC target was echoed into the response'

$typedBody = @{
  query_plan = @{
    contract_version='forensics.query-plan/v1'; case_id=$CollectionId; collection_id=$CollectionId
    intent='subscriber.identity_lookup'; families=@('subscriber_identity')
    entities=@(@{type='cnic';value=$rawCNIC}); filters=@(); limit=25
  }
} | ConvertTo-Json -Depth 12 -Compress
$typed = Invoke-RestMethod -Method Post -TimeoutSec 30 -Headers $headers `
  -Uri "http://localhost:8091/api/v1/forensics/cases/$CollectionId/query" -ContentType 'application/json' -Body $typedBody
$typedJSON = $typed | ConvertTo-Json -Depth 30 -Compress
Assert-Condition ($typed.status -eq 'needs_input') 'Typed CNIC target did not require clarification'
Assert-Condition (@($typed.execution_trace.route) -notcontains 'records_sql') 'Typed CNIC target executed SQL'
Assert-Condition ($typedJSON -notmatch [regex]::Escape($rawCNIC)) 'Typed CNIC target was echoed into the response'

$modelResult = [ordered]@{ requested=$false; status='not_requested'; latency_ms=0; summary=''; fallback_reason='' }
if ($RunModelSynthesis) {
  $model = Invoke-SubscriberQuery -Template 'subscriber_status_summary' -SynthesisModelName $SynthesisModel
  $modelRows = @($model.Response.records.subscriber_status_summary)
  Assert-Condition ($modelRows.Count -eq 6) 'Model-enabled query changed deterministic status-group count'
  Assert-Condition ((($modelRows | Measure-Object row_count -Sum).Sum) -eq 11) 'Model-enabled query changed deterministic subscriber row total'
  $summary = [string]$model.Response.answer.llm_summary
  $fallback = [string]$model.Response.answer.llm_fallback_reason
  Assert-Condition ([bool]$summary -or [bool]$fallback) 'Model-enabled query reported neither synthesis nor explicit fallback'
  $modelResult = [ordered]@{
    requested=$true; model=$SynthesisModel; status=$(if($summary){'answered'}else{'deterministic_fallback'})
    latency_ms=$model.LatencyMS; summary=$summary; fallback_reason=$fallback
    deterministic_group_count=$modelRows.Count; deterministic_row_count=11; request_id=$model.Response.telemetry.request_id
  }
}

$operationResults = @(
  [ordered]@{operation='subscriber.identity_lookup';rows=$identityRows.Count;latency_ms=$identity.LatencyMS;request_id=$identity.Response.telemetry.request_id},
  [ordered]@{operation='subscriber.validity_timeline';rows=$timelineRows.Count;latency_ms=$timeline.LatencyMS;request_id=$timeline.Response.telemetry.request_id},
  [ordered]@{operation='subscriber.device_links';rows=$deviceRows.Count;latency_ms=$devices.LatencyMS;request_id=$devices.Response.telemetry.request_id},
  [ordered]@{operation='subscriber.status_summary';groups=$statusRows.Count;rows=11;latency_ms=$statuses.LatencyMS;request_id=$statuses.Response.telemetry.request_id},
  [ordered]@{operation='subscriber.conflict_audit';rows=$conflictRows.Count;latency_ms=$conflicts.LatencyMS;request_id=$conflicts.Response.telemetry.request_id},
  [ordered]@{operation='subscriber.reuse_candidates';rows=$reuseRows.Count;latency_ms=$reuse.LatencyMS;request_id=$reuse.Response.telemetry.request_id}
)

[ordered]@{
  phase='7.1-populated-subscriber'; completed_utc=(Get-Date).ToUniversalTime().ToString('o')
  collection_id=$CollectionId; case_id=$CollectionId; fixture_sha256=$actualFixtureSHA256.ToLowerInvariant()
  evidence_id=$EvidenceId; version_id=$versionID; batch_id=$batchID; job_id=$jobID
  accounting=[ordered]@{total_rows=15;accepted_rows=11;duplicate_rows=1;rejected_rows=3;status='pass'}
  privacy=[ordered]@{raw_cnic_value_matches=0;raw_name_value_matches=0;masked_preview_matches=$maskedPreviewCount;status='pass'}
  operations=$operationResults; pagination='pass'; raw_cnic_guard='pass'; typed_cnic_guard='pass'
  model_synthesis=$modelResult; governed_collections_touched=$false; legacy_subscriber_collections_touched=$false
} | ConvertTo-Json -Depth 12 | Set-Content -Encoding utf8 -LiteralPath $MarkerPath

Write-Host "Phase71Populated=PASS Evidence=$EvidenceId Version=$versionID Batch=$batchID Job=$jobID Accounting=15/11/1/3 Operations=6 Privacy=PASS Pagination=PASS CNICGuards=PASS Model=$($modelResult.status)"
