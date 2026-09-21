param(
  [string]$CaseID = 'nexusai-forensic-demo',
  [string]$AgentName = 'Forensic_Records_Analyst',
  [string]$ApiKey = $env:LOCALAI_API_KEY,
  [string]$ExpectedCorpusVersion = '2026-08-11.r6.4',
  [string]$AcceptancePhase = 'R6.4+R6.5-A1',
  [string]$MarkerName = 'r6.4-live-acceptance.json',
  [string]$AcceptanceLogRoot = ''
)

$ErrorActionPreference = 'Stop'
$gateMutex = New-Object System.Threading.Mutex($false, 'Local\NexusAIR64ActivationGate')
if (-not $gateMutex.WaitOne(0)) {
  $gateMutex.Dispose()
  throw 'Another NexusAI R6.4 activation gate is already running'
}

$RepoRoot = Split-Path -Parent $PSScriptRoot
$LogRoot = if ($AcceptanceLogRoot) { [System.IO.Path]::GetFullPath($AcceptanceLogRoot) } else { Join-Path $RepoRoot 'reports\runtime-activation-20260810' }
$MarkerPath = Join-Path $LogRoot $MarkerName
$BaseGate = Join-Path $PSScriptRoot 'build_deploy_nexusai_r6_3_gate.ps1'
$ExpectedCatalogContract = 'forensics.query-template-catalog/v1'
$ExpectedCorpusContract = 'forensics.family-query-answer-corpus/v1'
$ExpectedPresentationContract = 'forensics.agent-presentation/v1'
$ExpectedTemplateCount = 65
$headers = @{}
if (-not [string]::IsNullOrWhiteSpace($ApiKey)) {
  $headers['Authorization'] = "Bearer $ApiKey"
}

function Assert-R64RuntimeContract {
  $encodedCase = [uri]::EscapeDataString($CaseID)
  $templates = Invoke-RestMethod -Method Get -UseBasicParsing -TimeoutSec 30 -Headers $headers `
    -Uri 'http://localhost:8080/api/records/forensic/templates'
  if ($templates.contract_version -ne $ExpectedCatalogContract -or
      [int]$templates.template_count -ne $ExpectedTemplateCount -or
      @($templates.templates).Count -ne $ExpectedTemplateCount) {
    throw 'The deployed R6.4 query-template catalog is not the accepted 65-operation contract'
  }
  foreach ($template in @($templates.templates)) {
    if ([string]::IsNullOrWhiteSpace([string]$template.name) -or
        [string]::IsNullOrWhiteSpace([string]$template.operation_id) -or
        [string]::IsNullOrWhiteSpace([string]$template.family_id) -or
        @('records', 'kb', 'hybrid') -notcontains [string]$template.route) {
      throw "Incomplete R6.4 operation metadata for template '$($template.name)'"
    }
    if ($ExpectedCorpusVersion -eq '2026-08-11.r6.6' -and
        (@('case_wide', 'case_or_target', 'target_required') -notcontains [string]$template.scope_mode -or
         @('bounded_records_table', 'cited_evidence_results', 'executive_brief_with_sources') -notcontains [string]$template.presentation)) {
      throw "Incomplete R6.6 scope/presentation metadata for template '$($template.name)'"
    }
  }

  $capabilities = Invoke-RestMethod -Method Get -UseBasicParsing -TimeoutSec 30 -Headers $headers `
    -Uri "http://localhost:8080/api/records/forensic/capabilities?collection_id=$encodedCase"
  $corpus = $capabilities.query_corpus
  $coveredTemplates = @($corpus.entries | ForEach-Object { [string]$_.expected_template } | Sort-Object -Unique)
  $suggestedEntries = @($corpus.entries | Where-Object { $_.suggested -eq $true })
  if ($corpus.contract_version -ne $ExpectedCorpusContract -or
      $corpus.corpus_version -ne $ExpectedCorpusVersion -or
      $coveredTemplates.Count -ne $ExpectedTemplateCount -or
      $suggestedEntries.Count -lt 1 -or
      @($corpus.scenarios).Count -lt 7) {
    throw 'The deployed R6.4 family corpus is incomplete or not at the accepted version'
  }

  return [ordered]@{
    template_count = @($templates.templates).Count
    corpus_entry_count = @($corpus.entries).Count
    covered_template_count = $coveredTemplates.Count
    suggested_entry_count = $suggestedEntries.Count
    scenario_count = @($corpus.scenarios).Count
  }
}

Push-Location $RepoRoot
try {
  New-Item -ItemType Directory -Force -Path $LogRoot | Out-Null

  # Reuse the proven sequential, rollback-safe combined rebuild. The delegated
  # gate and this acceptance layer use GET requests only and preserve volumes.
  & $BaseGate -CaseID $CaseID -AgentName $AgentName -ApiKey $ApiKey
  if ($LASTEXITCODE -ne 0) { throw "The guarded R6.3 base gate failed with exit code $LASTEXITCODE" }

  $acceptance = Assert-R64RuntimeContract
  [ordered]@{
    phase = $AcceptancePhase
    status = 'live_accepted'
    deployed_utc = (Get-Date).ToUniversalTime().ToString('o')
    presentation_contract_version = $ExpectedPresentationContract
    query_template_contract_version = $ExpectedCatalogContract
    query_corpus_contract_version = $ExpectedCorpusContract
    query_corpus_version = $ExpectedCorpusVersion
    case_id = $CaseID
    agent_name = $AgentName
    template_count = $acceptance.template_count
    corpus_entry_count = $acceptance.corpus_entry_count
    covered_template_count = $acceptance.covered_template_count
    suggested_entry_count = $acceptance.suggested_entry_count
    governance_scenario_count = $acceptance.scenario_count
    template_catalog_get = 'pass'
    query_corpus_get = 'pass'
    mutating_requests_sent = 0
    schema_drop_attempted = $false
    named_volumes_preserved = $true
    rollback_images_preserved = $true
  } | ConvertTo-Json | Set-Content -Encoding ascii -LiteralPath $MarkerPath
  Write-Host "R6.4Activation=PASS CaseID=$CaseID Templates=$($acceptance.template_count) Corpus=$ExpectedCorpusVersion Scenarios=$($acceptance.scenario_count) MutatingRequests=0 RollbackImages=preserved VolumesPreserved=true"
} finally {
  Pop-Location
  $gateMutex.ReleaseMutex()
  $gateMutex.Dispose()
}
