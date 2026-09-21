param(
  [string]$CollectionId = 'nexusai-forensic-demo',
  [string]$LocalAIUrl = 'http://localhost:8080',
  [string]$ReportPath = ''
)

$ErrorActionPreference = 'Stop'
if ($CollectionId -notin @('nexusai-forensic-demo', 'nexusai-multimodal-product-acceptance')) {
  throw 'The full demo query matrix is restricted to an approved deterministic demo collection'
}

$tests = @(
  @{ T='collection_overview'; Q='Show me an overview of this case' },
  @{ T='frequent_contacts'; Q='Who does 923001110001 contact most?'; X='923001110001' },
  @{ T='call_type_breakdown'; Q='How are the CDR events divided by call type?' },
  @{ T='service_usage'; Q='What services were used by 923001110001?'; X='923001110001' },
  @{ T='device_identity_changes'; Q='Did 923001110001 change IMEI or IMSI?'; X='923001110001' },
  @{ T='ipdr_endpoint_summary'; Q='Which network endpoints appear in the IPDR data?' },
  @{ T='ipdr_domain_summary'; Q='Which domains appear in the network records?' },
  @{ T='ipdr_protocol_breakdown'; Q='How is the network traffic divided by protocol?' },
  @{ T='ipdr_session_volume'; Q='What is the IPDR session and byte volume?' },
  @{ T='ipdr_subscriber_sessions'; Q='Show network sessions for subscriber 923001110002'; X='923001110002' },
  @{ T='ipdr_concurrent_sessions'; Q='Were there overlapping sessions for 923001110002?'; X='923001110002' },
  @{ T='ipdr_timeline'; Q='Build the network session timeline for 923001110002'; X='923001110002' },
  @{ T='temporal_activity'; Q='When was 923001110001 active in the CDR data?'; X='923001110001' },
  @{ T='top_locations'; Q='Which supplied locations appear most often for 923001110001?'; X='923001110001' },
  @{ T='geospatial_movement'; Q='Show the chronological supplied locations for 923001110001'; X='923001110001' },
  @{ T='anpr_sightings'; Q='Where and when was plate ABC-123 observed?'; X='ABC-123' },
  @{ T='anpr_camera_sequence'; Q='Which cameras observed ABC-123 in time order?'; X='ABC-123' },
  @{ T='anpr_camera_activity'; Q='Which ANPR cameras have the most observations?' },
  @{ T='anpr_co_travel'; Q='Which plates were observed at the same camera near ABC-123?'; X='ABC-123' },
  @{ T='anpr_route_timing'; Q='What were the time gaps between consecutive sightings of ABC-123?'; X='ABC-123' },
  @{ T='anpr_plate_variants'; Q='What plate variants were recorded for ABC-123?'; X='ABC-123' },
  @{ T='anpr_timeline'; Q='Build the ANPR timeline for ABC-123'; X='ABC-123' },
  @{ T='subscriber_identity_lookup'; Q='What subscriber observations exist for 923000000001?'; X='923000000001' },
  @{ T='subscriber_validity_timeline'; Q='When was subscriber PK-SUB-SYN-ALPHA valid?'; X='PK-SUB-SYN-ALPHA' },
  @{ T='subscriber_device_links'; Q='Which SIM and device identifiers are linked to 923000000001?'; X='923000000001' },
  @{ T='subscriber_status_summary'; Q='How many subscriber records are active, inactive or suspended?' },
  @{ T='subscriber_conflict_audit'; Q='Are there conflicting subscriber identity attributes?' },
  @{ T='subscriber_reuse_candidates'; Q='Which subscriber identifiers appear with multiple phone numbers?' },
  @{ T='entity_activity'; Q='What activity exists for 923001110001?'; X='923001110001' },
  @{ T='relationship_network'; Q='What evidence-backed relationships exist for 923001110001?'; X='923001110001' },
  @{ T='cross_family_correlation'; Q='Correlate 923001110001 across the available evidence families'; X='923001110001' },
  @{ T='entity_timeline'; Q='What happened involving 923001110001 over time?'; X='923001110001' },
  @{ T='source_records'; Q='Show the source rows involving 923001110001'; X='923001110001' },
  @{ T='canonical_records'; Q='Show normalized CDR records' },
  @{ T='schema_profile'; Q='What schemas and fields were detected?' },
  @{ T='data_quality'; Q='What data quality, rejection or duplicate issues exist?' },
  @{ T='evidence'; Q='Show the evidence inventory and lineage' },
  @{ T='shortest_call'; Q='What is the shortest call?' },
  @{ T='longest_call'; Q='What is the longest call?' },
  @{ T='duration_extremes'; Q='Show both the shortest and longest calls' },
  @{ T='first_seen_last_seen'; Q='When was 923001110001 first and last observed?'; X='923001110001' },
  @{ T='activity_by_day'; Q='Show daily activity for 923001110001'; X='923001110001' },
  @{ T='activity_by_hour'; Q='Show hourly activity for 923001110001'; X='923001110001' },
  @{ T='night_activity'; Q='Show night-time activity for 923001110001'; X='923001110001' },
  @{ T='repeated_location_visits'; Q='Which supplied locations recur for 923001110001?'; X='923001110001' },
  @{ T='co_travel_or_co_presence'; Q='Is there cross-family co-presence involving 923001110001?'; X='923001110001' },
  @{ T='subscriber_profile'; Q='What identifiers does the CDR show for 923001110001?'; X='923001110001' },
  @{ T='imei_imsi_usage'; Q='How were IMEI and IMSI identifiers used by 923001110001?'; X='923001110001' },
  @{ T='tower_activity'; Q='Which CDR cells or sites observed 923221110001?'; X='923221110001' },
  @{ T='suspicious_patterns'; Q='Which deterministic patterns should an analyst review?' },
  @{ T='anomaly_summary'; Q='Summarize the deterministic anomalies' },
  @{ T='cross_dataset_entity_summary'; Q='Summarize 923001110001 across datasets'; X='923001110001' },
  @{ T='source_file_audit'; Q='Which files were ingested and did their row counts reconcile?' },
  @{ T='duplicate_upload_audit'; Q='Were any source files uploaded more than once?' },
  @{ T='case_readiness'; Q='Is this case ready for analyst demonstration?' },
  @{ T='evidence_package_summary'; Q='What is included in the evidence package?' },
  @{ T='executive_case_brief'; Q='Prepare an executive case brief from deterministic findings' },
  @{ T='court_ready_source_summary'; Q='Prepare a court-ready source and provenance summary' },
  @{ T='limitations_and_data_quality'; Q='What limitations and missing data affect this case?' },
  @{ T='tower_site_lookup'; Q='What reference facts exist for tower PK-LHR-SYN-001?'; X='PK-LHR-SYN-001' },
  @{ T='tower_reference_timeline'; Q='Show the reference history for PK-LHR-SYN-001'; X='PK-LHR-SYN-001' },
  @{ T='tower_coordinate_audit'; Q='Which tower coordinates, datums or uncertainty values require review?' },
  @{ T='tower_status_summary'; Q='How are tower references divided by status and technology?' },
  @{ T='tower_alias_conflicts'; Q='Are any tower aliases associated with conflicting supplied facts?' },
  @{ T='tower_cdr_join'; Q='Join CDR observations to the valid tower reference for PK-LHR-SYN-001'; X='PK-LHR-SYN-001' }
)

if ($tests.Count -ne 65) { throw "Query matrix contains $($tests.Count) tests, expected 65" }
$results = @()
foreach ($test in $tests) {
  Write-Host "Testing $($test.T): $($test.Q)"
  $body = @{
    collection_id = $CollectionId
    case_id = $CollectionId
    query = $test.Q
    target = if ($test.X) { $test.X } else { '' }
    limit = 5
    max_kb_results = 0
  } | ConvertTo-Json -Depth 6 -Compress
  $started = Get-Date
  try {
    $response = Invoke-RestMethod -Method Post -TimeoutSec 45 `
      -Uri "$($LocalAIUrl.TrimEnd('/'))/api/records/forensic/query" `
      -ContentType 'application/json' -Body $body
  } catch {
    $results += [pscustomobject]@{
      template = $test.T
      question = $test.Q
      target = $test.X
      rows = 0
      latency_ms = [math]::Round(((Get-Date) - $started).TotalMilliseconds, 1)
      model_used = $false
      selected_template = ''
      outcome = 'request_failed'
      error = $_.Exception.Message
    }
    continue
  }
  $elapsed = [math]::Round(((Get-Date) - $started).TotalMilliseconds, 1)
  $routeMatches = $response.template -eq $test.T
  $completed = -not $response.error -and -not $response.needs_input
  $rowCount = 0
  if ($response.records) {
    foreach ($property in $response.records.PSObject.Properties) {
      $rowCount += @($property.Value).Count
    }
  }
  $results += [pscustomobject]@{
    template = $test.T
    question = $test.Q
    target = $test.X
    rows = $rowCount
    latency_ms = $elapsed
    model_used = [bool]$response.model_used
    selected_template = $response.template
    outcome = if (-not $routeMatches) { 'route_mismatch' } elseif (-not $completed) { 'incomplete' } elseif ($rowCount -gt 0) { 'answered' } else { 'accepted_no_results' }
  }
}

$answered = @($results | Where-Object { $_.outcome -eq 'answered' }).Count
$noResults = @($results | Where-Object { $_.outcome -eq 'accepted_no_results' }).Count
$failures = @($results | Where-Object { $_.outcome -in @('route_mismatch', 'incomplete', 'request_failed') })
$p95Index = [math]::Max(0, [math]::Ceiling($results.Count * 0.95) - 1)
$p95 = @($results.latency_ms | Sort-Object)[$p95Index]
if (-not $ReportPath) {
  $reportRoot = Join-Path (Split-Path -Parent $PSScriptRoot) 'reports\runtime-activation-20260811'
  New-Item -ItemType Directory -Force -Path $reportRoot | Out-Null
  $ReportPath = Join-Path $reportRoot 'r6.6-predeployment-65-operation-matrix.json'
}
$reportPath = [System.IO.Path]::GetFullPath($ReportPath)
$reportParent = Split-Path -Parent $reportPath
if ($reportParent) { New-Item -ItemType Directory -Force -Path $reportParent | Out-Null }
$results | ConvertTo-Json -Depth 5 | Set-Content -Encoding utf8 -LiteralPath $reportPath
$results | ConvertTo-Json -Depth 5
if ($failures.Count -gt 0) {
  throw "Full demo query matrix has $($failures.Count) routing/completion failures"
}
Write-Host "FullDemoQueryMatrix=PASS Queries=$($results.Count) Answered=$answered AcceptedNoResults=$noResults P95MS=$p95 Report=$reportPath"
