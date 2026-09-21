param(
  [string]$CollectionId = 'nexusai-forensic-demo',
  [string]$LocalAIUrl = 'http://localhost:8080',
  [int]$TimeoutSeconds = 30,
  [string]$TemplateFilter = '',
  [string]$SourcePath = '',
  [string]$ReportPath = '',
  [switch]$ModelAssisted
)

$ErrorActionPreference = 'Stop'
if ($CollectionId -ne 'nexusai-forensic-demo') {
  throw 'The full Agent Chat matrix is restricted to nexusai-forensic-demo'
}

$repoRoot = Split-Path -Parent $PSScriptRoot
$sourcePath = if ($SourcePath) { [System.IO.Path]::GetFullPath($SourcePath) } else { Join-Path $repoRoot 'reports\runtime-activation-20260805\phase7.2-natural-query-matrix.json' }
if (-not (Test-Path -LiteralPath $sourcePath)) {
  throw "Natural-language matrix fixture is missing: $sourcePath"
}
$parsedTests = Get-Content -Raw -LiteralPath $sourcePath | ConvertFrom-Json
$tests = @($parsedTests | ForEach-Object { $_ })
if ($tests.Count -ne 65) { throw "Agent Chat matrix fixture contains $($tests.Count) tests, expected 65" }
if ($TemplateFilter) {
  $selectedTemplates = @($TemplateFilter.Split(',') | ForEach-Object { $_.Trim() } | Where-Object { $_ })
  $tests = @($tests | Where-Object { $selectedTemplates -contains [string]$_.template })
  if ($tests.Count -ne $selectedTemplates.Count) {
    throw "Template filter matched $($tests.Count) of $($selectedTemplates.Count) requested templates"
  }
}

$agentByTemplate = @{}
@('collection_overview','entity_activity','relationship_network','cross_family_correlation','entity_timeline','source_records','canonical_records','schema_profile','data_quality','evidence','first_seen_last_seen','activity_by_day','co_travel_or_co_presence','suspicious_patterns','anomaly_summary','cross_dataset_entity_summary','source_file_audit','duplicate_upload_audit','case_readiness','evidence_package_summary','executive_case_brief','court_ready_source_summary','limitations_and_data_quality') | ForEach-Object { $agentByTemplate[$_] = 'Forensic_Records_Analyst' }
@('frequent_contacts','call_type_breakdown','service_usage','device_identity_changes','temporal_activity','top_locations','geospatial_movement','shortest_call','longest_call','duration_extremes','activity_by_hour','night_activity','repeated_location_visits','subscriber_profile','imei_imsi_usage','tower_activity') | ForEach-Object { $agentByTemplate[$_] = 'Communications_CDR_Analyst' }
@('ipdr_endpoint_summary','ipdr_domain_summary','ipdr_protocol_breakdown','ipdr_session_volume','ipdr_subscriber_sessions','ipdr_concurrent_sessions','ipdr_timeline') | ForEach-Object { $agentByTemplate[$_] = 'Network_IPDR_Capture_Analyst' }
@('anpr_sightings','anpr_camera_sequence','anpr_camera_activity','anpr_co_travel','anpr_route_timing','anpr_plate_variants','anpr_timeline') | ForEach-Object { $agentByTemplate[$_] = 'Vehicle_ANPR_Geospatial_Analyst' }
@('subscriber_identity_lookup','subscriber_validity_timeline','subscriber_device_links','subscriber_status_summary','subscriber_conflict_audit','subscriber_reuse_candidates') | ForEach-Object { $agentByTemplate[$_] = 'Subscriber_Identity_Analyst' }
@('tower_site_lookup','tower_reference_timeline','tower_coordinate_audit','tower_status_summary','tower_alias_conflicts','tower_cdr_join') | ForEach-Object { $agentByTemplate[$_] = 'Tower_Location_Reference_Analyst' }

$reportName = if ($TemplateFilter) { 'r6.6-predeployment-agent-chat-regression.json' } else { 'r6.6-predeployment-agent-chat-full-matrix.json' }
$reportPath = if ($ReportPath) { [System.IO.Path]::GetFullPath($ReportPath) } else { Join-Path $repoRoot "reports\runtime-activation-20260811\$reportName" }
$reportParent = Split-Path -Parent $reportPath
if ($reportParent) { New-Item -ItemType Directory -Force -Path $reportParent | Out-Null }
$results = @()
foreach ($test in $tests) {
  $template = [string]$test.template
  $agentName = [string]$agentByTemplate[$template]
  if (-not $agentName) { throw "No specialist agent mapping exists for $template" }
  Write-Host "Testing Agent Chat [$agentName] $template"
  $agent = [uri]::EscapeDataString($agentName)
  $sseURL = "$($LocalAIUrl.TrimEnd('/'))/api/agents/$agent/sse"
  $sseJob = Start-Job -ScriptBlock {
    param($url, $timeout)
    curl.exe -sS -N --max-time $timeout $url
  } -ArgumentList $sseURL, ($TimeoutSeconds + 5)

  $started = Get-Date
  $ack = $null
  $answer = $null
  $errorText = ''
  try {
    Start-Sleep -Milliseconds 700
	$submittedQuestion = [string]$test.question
	if (-not $ModelAssisted) {
	  # This matrix validates all deterministic computations and presentations
	  # without serially invoking the CPU model 65 times. The separate LLM
	  # matrix validates automatic model assistance for every specialist family.
	  $submittedQuestion += ' Use deterministic only mode.'
	}
    $body = @{
      message = $submittedQuestion
      case_id = $CollectionId
      collection_id = $CollectionId
    } | ConvertTo-Json -Compress
    $ack = Invoke-RestMethod -Method Post -TimeoutSec 15 `
      -Uri "$($LocalAIUrl.TrimEnd('/'))/api/agents/$agent/chat" `
      -ContentType 'application/json' -Body $body
    $requestID = [string]$ack.message_id
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
      Start-Sleep -Milliseconds 250
      foreach ($line in @(Receive-Job -Job $sseJob -Keep)) {
        if ($line -notlike 'data: *') { continue }
        try { $event = $line.Substring(6) | ConvertFrom-Json } catch { continue }
        $eventID = [string]$event.message_id
        $baseID = $eventID -replace '-agent$',''
        if ($event.sender -eq 'agent' -and $event.content -and $baseID -eq $requestID) {
          $answer = $event
          break
        }
        if ($event.error -and $baseID -eq $requestID) { $errorText = [string]$event.error }
      }
    } while (-not $answer -and -not $errorText -and (Get-Date) -lt $deadline)
  } catch {
    $errorText = $_.Exception.Message
  } finally {
    Stop-Job $sseJob -ErrorAction SilentlyContinue
    Remove-Job $sseJob -Force -ErrorAction SilentlyContinue
  }

  $content = if ($answer) { [string]$answer.content } else { '' }
  $results += [pscustomobject]@{
    agent = $agentName
    template = $template
    question = [string]$test.question
	submitted_question = $submittedQuestion
    message_id = if ($ack) { [string]$ack.message_id } else { '' }
    completed = [bool]$answer
    latency_ms = [math]::Round(((Get-Date) - $started).TotalMilliseconds, 1)
    selected_template_visible = $content -match [regex]::Escape($template)
    selected_collection_visible = $content -match [regex]::Escape($CollectionId)
	professional_heading_visible = $content -match '(?m)^##\s+\S'
	internal_fields_hidden = $content -notmatch 'Operating Metrics|Sourced Claims|Data Grid Preview|\*\*Template:\*\*'
    error = $errorText
    answer_length = $content.Length
  }
  $results | ConvertTo-Json -Depth 7 | Set-Content -Encoding utf8 -LiteralPath $reportPath
}

$failures = @($results | Where-Object {
  -not $_.completed -or -not $_.selected_collection_visible -or
  -not $_.professional_heading_visible -or -not $_.internal_fields_hidden -or $_.error
})
$p95Index = [math]::Max(0, [math]::Ceiling($results.Count * 0.95) - 1)
$p95 = @($results.latency_ms | Sort-Object)[$p95Index]
if ($failures.Count -gt 0) {
  $failed = ($failures | ForEach-Object { "$($_.agent):$($_.template)" }) -join ', '
  throw "Full Agent Chat matrix failed for $($failures.Count) queries: $failed"
}
Write-Host "AgentChatFullMatrix=PASS Queries=$($results.Count) P95MS=$p95 Report=$reportPath"
