param(
  [string]$CollectionId = 'nexusai-forensic-demo',
  [string]$LocalAIUrl = 'http://localhost:8080',
  [string]$ReportPath = ''
)

$ErrorActionPreference = 'Stop'

if ($CollectionId -ne 'nexusai-forensic-demo') {
  throw 'The STIM-5/6 runtime acceptance deck is restricted to nexusai-forensic-demo'
}

$baseUrl = $LocalAIUrl.TrimEnd('/')
$results = New-Object System.Collections.Generic.List[object]

function Add-STIM56Result {
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

function Invoke-STIM56Query {
  param(
    [Parameter(Mandatory=$true)][string]$Query,
    [hashtable]$Extra = @{},
    [int]$TimeoutSec = 20
  )
  $body = [ordered]@{
    collection_id = $CollectionId
    case_id = $CollectionId
    query = $Query
    limit = 10
    max_kb_results = 0
  }
  foreach ($key in $Extra.Keys) { $body[$key] = $Extra[$key] }
  $json = $body | ConvertTo-Json -Depth 20 -Compress
  $started = Get-Date
  $response = Invoke-RestMethod -Method Post -TimeoutSec $TimeoutSec `
    -Uri "$baseUrl/api/records/forensic/query" `
    -ContentType 'application/json; charset=utf-8' `
    -Body ([Text.Encoding]::UTF8.GetBytes($json))
  return [pscustomobject]@{
    response = $response
    latency_ms = ((Get-Date) - $started).TotalMilliseconds
  }
}

function ConvertFrom-STIM56Utf8Base64 {
  param([Parameter(Mandatory=$true)][string]$Value)
  return [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($Value))
}

function Test-STIM56Packet {
  param(
    [Parameter(Mandatory=$true)]$Response,
    [Parameter(Mandatory=$true)][string]$ExpectedLanguage,
    [string]$ExactIdentifier = ''
  )
  $packet = $Response.enterprise.fact_packet
  $narrative = $Response.enterprise.narrative
  if (-not $packet -or $packet.contract_version -ne 'forensics.fact-packet/v1') { return 'missing or invalid Fact Packet contract' }
  if (-not $narrative -or $narrative.contract_version -ne 'forensics.narrative/v1') { return 'missing or invalid narrative contract' }
  if ($packet.language.tag -ne $ExpectedLanguage) { return "Fact Packet language was $($packet.language.tag), expected $ExpectedLanguage" }
  if ($narrative.locale -ne $ExpectedLanguage) { return "narrative locale was $($narrative.locale), expected $ExpectedLanguage" }
  if (@($packet.facts).Count -gt 24) { return 'Fact Packet exceeded the 24-fact bound' }
  if (@($packet.rows).Count -gt 20) { return 'Fact Packet exceeded the 20-row bound' }
  if (@($packet.citations).Count -gt 50) { return 'Fact Packet exceeded the 50-citation bound' }
  foreach ($fact in @($packet.facts)) {
    if ([string]$fact.fact_id -notmatch '^F[1-9][0-9]*$') { return "invalid fact ID $($fact.fact_id)" }
  }
  foreach ($citation in @($packet.citations)) {
    if ([string]$citation.citation_id -notmatch '^C[1-9][0-9]*$') { return "invalid citation ID $($citation.citation_id)" }
    if ([string]::IsNullOrWhiteSpace([string]$citation.proof_role)) { return "citation $($citation.citation_id) has no proof role" }
  }
  $factIds = @($packet.facts | ForEach-Object { [string]$_.fact_id })
  $citationIds = @($packet.citations | ForEach-Object { [string]$_.citation_id })
  foreach ($ref in @($narrative.fact_refs)) {
    if ($factIds -notcontains [string]$ref) { return "narrative references unknown fact $ref" }
  }
  foreach ($ref in @($narrative.citation_refs)) {
    if ($citationIds -notcontains [string]$ref) { return "narrative references unknown citation $ref" }
  }
  if ($ExactIdentifier) {
    $packetJSON = $packet | ConvertTo-Json -Depth 30 -Compress
    if ($Response.target -ne $ExactIdentifier -or $packetJSON -notmatch [regex]::Escape($ExactIdentifier)) {
      return "exact identifier $ExactIdentifier was not preserved"
    }
  }
  return ''
}

$healthStarted = Get-Date
$ready = Invoke-WebRequest -UseBasicParsing -TimeoutSec 20 -Uri "$baseUrl/readyz"
Add-STIM56Result -Name 'runtime.readyz' -Passed ($ready.StatusCode -eq 200) -Detail "HTTP $($ready.StatusCode)" -LatencyMS (((Get-Date) - $healthStarted).TotalMilliseconds)

$models = Invoke-RestMethod -TimeoutSec 20 -Uri "$baseUrl/v1/models"
$modelIDs = @($models.data | ForEach-Object { [string]$_.id })
$modelsPassed = $modelIDs -contains 'qwen_qwen3-4b-instruct-2507' -and $modelIDs -contains 'qwen3-embedding-0.6b'
Add-STIM56Result -Name 'runtime.model_inventory' -Passed $modelsPassed -Detail ($modelIDs -join ', ')

$queryCases = @(
  @{ id='english'; query='Who does 923001110001 contact most?'; template='frequent_contacts'; language='en'; target='923001110001' },
  @{ id='messy_english'; query='frequent cntcts 923001110001'; template='frequent_contacts'; language='en'; target='923001110001' },
  @{ id='roman_urdu'; query='923001110001 ki activity dikhao'; template='temporal_activity'; language='ur-Latn'; target='923001110001' },
  # Base64 keeps Urdu literals intact when Windows PowerShell 5.1 reads this
  # UTF-8 script without relying on a host-specific default code page.
  @{ id='urdu'; query=(ConvertFrom-STIM56Utf8Base64 'OTIzMDAxMTEwMDAxINqp24wg2LPYsdqv2LHZhduMINiv2qnavtin2KbbjNq6'); template='temporal_activity'; language='ur'; target='923001110001' },
  @{ id='mixed'; query=(ConvertFrom-STIM56Utf8Base64 '2KfYsyDZhtmF2KjYsSDaqduMIG91dGdvaW5nIGFjdGl2aXR5IHNob3cga2FybyA5MjMwMDExMTAwMDE='); template='temporal_activity'; language='mixed'; target='923001110001'; direction='OUTGOING' }
)

foreach ($case in $queryCases) {
  try {
    $result = Invoke-STIM56Query -Query $case.query
    $response = $result.response
    $problem = Test-STIM56Packet -Response $response -ExpectedLanguage $case.language -ExactIdentifier $case.target
    if ($response.template -ne $case.template) { $problem = "template was $($response.template), expected $($case.template)" }
    if ($case.direction -and $response.query_plan.applied_filters.event_direction -ne $case.direction) {
      $problem = "direction was $($response.query_plan.applied_filters.event_direction), expected $($case.direction)"
    }
    Add-STIM56Result -Name "query.$($case.id)" -Passed ([string]::IsNullOrWhiteSpace($problem)) -Detail $(if ($problem) { $problem } else { "$($response.template); language=$($response.query_understanding.language.tag); target=$($response.target)" }) -LatencyMS $result.latency_ms
  } catch {
    Add-STIM56Result -Name "query.$($case.id)" -Passed $false -Detail $_.Exception.Message
  }
}

try {
  $clarification = Invoke-STIM56Query -Query 'Show frequent contacts'
  $clarificationPassed = [bool]$clarification.response.answer.clarification_required -and $clarification.response.telemetry.llm_latency_ms -eq 0
  Add-STIM56Result -Name 'query.clarification' -Passed $clarificationPassed -Detail "intent=$($clarification.response.intent); model_ms=$($clarification.response.telemetry.llm_latency_ms)" -LatencyMS $clarification.latency_ms
} catch {
  Add-STIM56Result -Name 'query.clarification' -Passed $false -Detail $_.Exception.Message
}

try {
  $expiresAt = (Get-Date).ToUniversalTime().AddMinutes(15).ToString('o')
  $context = @{
    contract_version = 'forensics.follow-up-context/v1'
    tenant_id = 'default'
    collection_id = $CollectionId
    target = '923001110001'
    template = 'frequent_contacts'
    expires_at = $expiresAt
  }
  $followUp = Invoke-STIM56Query -Query 'sirf outgoing dikhao' -Extra @{ conversation_context = $context }
  $inherited = @($followUp.response.query_plan.applied_filters.inherited_fields | ForEach-Object { $_.field })
  $followUpPassed = $followUp.response.target -eq '923001110001' -and $followUp.response.template -eq 'frequent_contacts' -and $followUp.response.query_plan.applied_filters.event_direction -eq 'OUTGOING' -and $inherited -contains 'target'
  Add-STIM56Result -Name 'query.follow_up' -Passed $followUpPassed -Detail "template=$($followUp.response.template); target=$($followUp.response.target); direction=$($followUp.response.query_plan.applied_filters.event_direction); inherited=$($inherited -join ',')" -LatencyMS $followUp.latency_ms
} catch {
  Add-STIM56Result -Name 'query.follow_up' -Passed $false -Detail $_.Exception.Message
}

try {
  $unsupported = Invoke-STIM56Query -Query 'Transcribe and diarize this audio.' -Extra @{ synthesis_model = 'qwen_qwen3-4b-instruct-2507' } -TimeoutSec 10
  $route = @($unsupported.response.route)
  $unsupportedPassed = $unsupported.response.capability.status -eq 'unavailable' -and $route.Count -eq 1 -and $route[0] -eq 'capability_guard' -and $unsupported.response.telemetry.db_latency_ms -eq 0 -and $unsupported.response.telemetry.kb_latency_ms -eq 0 -and $unsupported.response.telemetry.llm_latency_ms -eq 0 -and $unsupported.latency_ms -lt 5000
  Add-STIM56Result -Name 'query.unsupported_fast_path' -Passed $unsupportedPassed -Detail "route=$($route -join '+'); db_ms=$($unsupported.response.telemetry.db_latency_ms); kb_ms=$($unsupported.response.telemetry.kb_latency_ms); model_ms=$($unsupported.response.telemetry.llm_latency_ms)" -LatencyMS $unsupported.latency_ms
} catch {
  Add-STIM56Result -Name 'query.unsupported_fast_path' -Passed $false -Detail $_.Exception.Message
}

try {
  $assisted = Invoke-STIM56Query -Query 'Explain what the frequent contacts for 923001110001 mean using the model.' -Extra @{ synthesis_model = 'qwen_qwen3-4b-instruct-2507' } -TimeoutSec 20
  $problem = Test-STIM56Packet -Response $assisted.response -ExpectedLanguage 'en' -ExactIdentifier '923001110001'
  $narrativeStatus = [string]$assisted.response.enterprise.narrative.status
  $narrativeFallback = [bool]$assisted.response.enterprise.narrative.fallback
  $acceptedNarrativeState = $narrativeStatus -eq 'validated_model' -or ($narrativeFallback -and $narrativeStatus.EndsWith('_fallback'))
  $assistedPassed = [string]::IsNullOrWhiteSpace($problem) -and $acceptedNarrativeState -and $assisted.latency_ms -lt 15000
  Add-STIM56Result -Name 'query.assisted_validated_or_fallback' -Passed $assistedPassed -Detail $(if ($problem) { $problem } else { "narrative_status=$narrativeStatus; model_ms=$($assisted.response.telemetry.llm_latency_ms)" }) -LatencyMS $assisted.latency_ms
} catch {
  Add-STIM56Result -Name 'query.assisted_validated_or_fallback' -Passed $false -Detail $_.Exception.Message
}

foreach ($route in @('/analyst/data', '/analyst/ask', '/analyst/history')) {
  try {
    $started = Get-Date
    # Echo's SPA fallback is content-negotiated; use the same HTML Accept header
    # as a browser so a deep-link probe verifies the deployed application route.
    $page = Invoke-WebRequest -UseBasicParsing -TimeoutSec 20 -Headers @{ Accept = 'text/html' } -Uri "$baseUrl$route"
    $html = [string]$page.Content
    $passed = $page.StatusCode -eq 200 -and $html -match '<div id="root"></div>'
    Add-STIM56Result -Name "ui$route" -Passed $passed -Detail "HTTP $($page.StatusCode); app_root=$($html -match '<div id="root"></div>')" -LatencyMS (((Get-Date) - $started).TotalMilliseconds)
  } catch {
    Add-STIM56Result -Name "ui$route" -Passed $false -Detail $_.Exception.Message
  }
}

$failures = @($results | Where-Object { -not $_.passed })
$report = [ordered]@{
  contract_version = 'nexusai.stim56-runtime-acceptance/v1'
  generated_utc = (Get-Date).ToUniversalTime().ToString('o')
  collection_id = $CollectionId
  localai_url = $baseUrl
  database_mutated = $false
  models_changed = $false
  # Windows PowerShell 5.1 can throw "Argument types do not match" when its
  # array-subexpression binder expands a generic List[object] inside a hashtable.
  # Materialize the list explicitly so the acceptance report is portable.
  checks = $results.ToArray()
  passed = $failures.Count -eq 0
}

if (-not $ReportPath) {
  $repoRoot = Split-Path -Parent $PSScriptRoot
  $reportRoot = Join-Path $repoRoot 'reports\runtime-activation-stim56'
  New-Item -ItemType Directory -Force -Path $reportRoot | Out-Null
  $ReportPath = Join-Path $reportRoot ("stim56-runtime-acceptance-{0}.json" -f (Get-Date).ToUniversalTime().ToString('yyyyMMddTHHmmssZ'))
}
$reportPath = [System.IO.Path]::GetFullPath($ReportPath)
$reportParent = Split-Path -Parent $reportPath
if ($reportParent) { New-Item -ItemType Directory -Force -Path $reportParent | Out-Null }
$report | ConvertTo-Json -Depth 30 | Set-Content -Encoding utf8 -LiteralPath $reportPath
$report | ConvertTo-Json -Depth 30

if ($failures.Count -gt 0) {
  throw "STIM-5/6 runtime acceptance failed $($failures.Count) check(s); report=$reportPath"
}
Write-Host "STIM56RuntimeAcceptance=PASS Checks=$($results.Count) Failures=0 DatabaseMutated=false ModelsChanged=false Report=$reportPath"
