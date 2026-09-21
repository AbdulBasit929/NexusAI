param(
  [string]$CaseID = 'nexusai-forensic-demo',
  [string]$AgentName = 'Forensic_Records_Analyst',
  [string]$ApiKey = $env:LOCALAI_API_KEY,
  [int]$MaxP95LatencyMS = 5000
)

$ErrorActionPreference = 'Stop'
$gateMutex = New-Object System.Threading.Mutex($false, 'Local\NexusAIR65LiveAcceptance')
if (-not $gateMutex.WaitOne(0)) {
  $gateMutex.Dispose()
  throw 'Another NexusAI R6.5 live acceptance is already running'
}

$RepoRoot = Split-Path -Parent $PSScriptRoot
$LogRoot = Join-Path $RepoRoot 'reports\runtime-activation-20260810'
$R64MarkerPath = Join-Path $LogRoot 'r6.4-live-acceptance.json'
$MarkerPath = Join-Path $LogRoot 'r6.5-live-acceptance.json'
$headers = @{}
if (-not [string]::IsNullOrWhiteSpace($ApiKey)) {
  $headers['Authorization'] = "Bearer $ApiKey"
}

function Invoke-TimedGet {
  param([Parameter(Mandatory)][string]$Uri, [switch]$Raw)
  $timer = [System.Diagnostics.Stopwatch]::StartNew()
  if ($Raw) {
    $response = Invoke-WebRequest -Method Get -UseBasicParsing -TimeoutSec 30 -Headers $headers -Uri $Uri
    $body = $response.Content
    $statusCode = $response.StatusCode
  } else {
    $body = Invoke-RestMethod -Method Get -UseBasicParsing -TimeoutSec 30 -Headers $headers -Uri $Uri
    $statusCode = 200
  }
  $timer.Stop()
  return [pscustomobject]@{
    Body = $body
    StatusCode = $statusCode
    ElapsedMS = [math]::Round($timer.Elapsed.TotalMilliseconds, 2)
  }
}

function Get-Percentile95 {
  param([Parameter(Mandatory)][double[]]$Values)
  $sorted = @($Values | Sort-Object)
  if ($sorted.Count -eq 0) { return 0 }
  $index = [math]::Max(0, [math]::Ceiling($sorted.Count * 0.95) - 1)
  return [math]::Round([double]$sorted[$index], 2)
}

Push-Location $RepoRoot
try {
  if (-not (Test-Path -LiteralPath $R64MarkerPath)) {
    throw 'R6.4 live acceptance marker is missing'
  }
  $r64 = Get-Content -Raw -LiteralPath $R64MarkerPath | ConvertFrom-Json
  if ($r64.status -ne 'live_accepted' -or $r64.template_count -ne 65 -or
      $r64.covered_template_count -ne 65 -or $r64.governance_scenario_count -lt 7 -or
      $r64.mutating_requests_sent -ne 0 -or -not $r64.named_volumes_preserved -or
      -not $r64.rollback_images_preserved) {
    throw 'R6.4 live acceptance prerequisite is invalid'
  }

  $encodedCase = [uri]::EscapeDataString($CaseID)
  $encodedAgent = [uri]::EscapeDataString($AgentName)
  $latencies = New-Object System.Collections.Generic.List[double]

  $ready = Invoke-TimedGet -Raw -Uri 'http://localhost:8080/readyz'
  $recordsHealth = Invoke-TimedGet -Raw -Uri 'http://localhost:8091/healthz'
  $app = Invoke-TimedGet -Raw -Uri 'http://localhost:8080/app'
  $cases = Invoke-TimedGet -Uri 'http://localhost:8080/api/v1/forensics/cases'
  $agents = Invoke-TimedGet -Uri 'http://localhost:8080/api/v1/forensics/agents'
  $templates = Invoke-TimedGet -Uri 'http://localhost:8080/api/records/forensic/templates'
  $capabilities = Invoke-TimedGet -Uri "http://localhost:8080/api/records/forensic/capabilities?collection_id=$encodedCase"
  $history = Invoke-TimedGet -Uri "http://localhost:8080/api/agents/$encodedAgent/history?case_id=$encodedCase&collection_id=$encodedCase&limit=1"
  foreach ($probe in @($ready, $recordsHealth, $app, $cases, $agents, $templates, $capabilities, $history)) {
    $latencies.Add([double]$probe.ElapsedMS)
  }

  if ($ready.StatusCode -ne 200 -or $recordsHealth.StatusCode -ne 200 -or $app.StatusCode -ne 200) {
    throw 'R6.5 health or application-shell acceptance failed'
  }
  if (@($cases.Body.cases | Where-Object { $_.case_id -eq $CaseID -and $_.selectable -eq $true }).Count -ne 1) {
    throw 'The governed case is not uniquely selectable'
  }
  $requiredSpecialists = @(
    'Communications_CDR_Analyst',
    'Network_IPDR_Capture_Analyst',
    'Vehicle_ANPR_Geospatial_Analyst',
    'Subscriber_Identity_Analyst',
    'Tower_Location_Reference_Analyst',
    'Forensic_Records_Analyst'
  )
  $publishedAgents = @($agents.Body.agents | ForEach-Object { [string]$_.id })
  foreach ($specialist in $requiredSpecialists) {
    if ($publishedAgents -notcontains $specialist) {
      throw "Required forensic specialist is not published: $specialist"
    }
  }
  if ($templates.Body.contract_version -ne 'forensics.query-template-catalog/v1' -or
      @($templates.Body.templates).Count -ne 65) {
    throw 'The deployed query-template catalog is incomplete'
  }
  $corpus = $capabilities.Body.query_corpus
  $coveredTemplates = @($corpus.entries | ForEach-Object { [string]$_.expected_template } | Sort-Object -Unique)
  if ($corpus.contract_version -ne 'forensics.family-query-answer-corpus/v1' -or
      $corpus.corpus_version -ne '2026-08-11.r6.4' -or
      $coveredTemplates.Count -ne 65 -or @($corpus.scenarios).Count -lt 7) {
    throw 'The deployed family query/answer corpus is incomplete'
  }
  if ($history.Body.contract_version -ne 'forensics.case-analysis-history/v1' -or
      $null -eq $history.Body.items -or $null -eq $history.Body.has_more) {
    throw 'The retained-history GET contract is invalid'
  }

  $p95LatencyMS = Get-Percentile95 -Values $latencies.ToArray()
  if ($p95LatencyMS -gt $MaxP95LatencyMS) {
    throw "R6.5 GET latency p95 $p95LatencyMS ms exceeds the $MaxP95LatencyMS ms local acceptance bound"
  }
  $appAsset = [regex]::Match([string]$app.Body, '/assets/index-[A-Za-z0-9_-]+\.js').Value
  if ([string]::IsNullOrWhiteSpace($appAsset)) {
    throw 'The deployed application shell does not reference a versioned production asset'
  }

  New-Item -ItemType Directory -Force -Path $LogRoot | Out-Null
  [ordered]@{
    phase = 'R6'
    status = 'live_accepted'
    completed_utc = (Get-Date).ToUniversalTime().ToString('o')
    case_id = $CaseID
    agent_name = $AgentName
    presentation_contract_version = 'forensics.agent-presentation/v1'
    query_template_contract_version = 'forensics.query-template-catalog/v1'
    query_corpus_contract_version = 'forensics.family-query-answer-corpus/v1'
    query_corpus_version = [string]$corpus.corpus_version
    history_contract_version = [string]$history.Body.contract_version
    template_count = @($templates.Body.templates).Count
    corpus_entry_count = @($corpus.entries).Count
    covered_template_count = $coveredTemplates.Count
    suggested_entry_count = @($corpus.entries | Where-Object { $_.suggested -eq $true }).Count
    governance_scenario_count = @($corpus.scenarios).Count
    required_specialist_count = $requiredSpecialists.Count
    app_asset = $appAsset
    get_probe_count = $latencies.Count
    get_latency_p95_ms = $p95LatencyMS
    get_latency_max_bound_ms = $MaxP95LatencyMS
    localai_ready = 'pass'
    records_health = 'pass'
    application_shell = 'pass'
    governed_case_scope = 'pass'
    specialist_registry = 'pass'
    retained_history_get = 'pass'
    template_catalog_get = 'pass'
    query_corpus_get = 'pass'
    mutating_requests_sent = 0
    schema_drop_attempted = $false
    named_volumes_preserved = [bool]$r64.named_volumes_preserved
    rollback_images_preserved = [bool]$r64.rollback_images_preserved
  } | ConvertTo-Json | Set-Content -Encoding ascii -LiteralPath $MarkerPath
  Write-Host "R6Acceptance=PASS CaseID=$CaseID Templates=65 Corpus=$($corpus.corpus_version) Specialists=$($requiredSpecialists.Count) GETP95ms=$p95LatencyMS MutatingRequests=0 RollbackImages=preserved VolumesPreserved=true"
} finally {
  Pop-Location
  $gateMutex.ReleaseMutex()
  $gateMutex.Dispose()
}
