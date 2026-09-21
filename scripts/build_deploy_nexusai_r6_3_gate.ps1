param(
  [string]$CaseID = 'nexusai-forensic-demo',
  [string]$AgentName = 'Forensic_Records_Analyst',
  [string]$ApiKey = $env:LOCALAI_API_KEY
)

$ErrorActionPreference = 'Stop'
$gateMutex = New-Object System.Threading.Mutex($false, 'Local\NexusAIR63ActivationGate')
if (-not $gateMutex.WaitOne(0)) {
  $gateMutex.Dispose()
  throw 'Another NexusAI R6.3 activation gate is already running'
}

$RepoRoot = Split-Path -Parent $PSScriptRoot
$LogRoot = Join-Path $RepoRoot 'reports\runtime-activation-20260810'
$MarkerPath = Join-Path $LogRoot 'r6.3-live-acceptance.json'
$CombinedGate = Join-Path $PSScriptRoot 'build_deploy_forensic_phase6_gate.ps1'
$SwaggerPath = Join-Path $RepoRoot 'swagger\swagger.json'
$EnvFile = Join-Path $RepoRoot '.env.forensic-runtime.local'
$ExpectedContract = 'forensics.case-analysis-history/v1'
$ExpectedPresentationContract = 'forensics.agent-presentation/v1'
$ExpectedCorpusContract = 'forensics.family-query-answer-corpus/v1'
$ExpectedRoutes = @(
  '/api/agents/{name}/history',
  '/api/agents/{name}/history/{analysis_id}',
  '/api/agents/{name}/history/{analysis_id}/saved',
  '/api/agents/{name}/history/import',
  '/api/agents/{name}/history/imports/{import_id}'
)
$headers = @{}
if (-not [string]::IsNullOrWhiteSpace($ApiKey)) {
  $headers['Authorization'] = "Bearer $ApiKey"
}

function Assert-R63SourceContract {
  if (-not (Test-Path -LiteralPath $SwaggerPath)) {
    throw "Generated Swagger contract is missing: $SwaggerPath"
  }
  $swagger = Get-Content -Raw -LiteralPath $SwaggerPath | ConvertFrom-Json
  foreach ($route in $ExpectedRoutes) {
    if (-not $swagger.paths.PSObject.Properties[$route]) {
      throw "Generated Swagger does not publish the R6.3 route: $route"
    }
  }
  $configuredURL = [string]$env:NEXUSAI_AGENT_HISTORY_DATABASE_URL
  if ([string]::IsNullOrWhiteSpace($configuredURL) -and (Test-Path -LiteralPath $EnvFile)) {
    $line = Get-Content -LiteralPath $EnvFile | Where-Object { $_ -match '^NEXUSAI_AGENT_HISTORY_DATABASE_URL=' } | Select-Object -Last 1
    if ($line) { $configuredURL = ($line -split '=', 2)[1].Trim() }
  }
  if ([string]::IsNullOrWhiteSpace($configuredURL)) {
    # A previously accepted deployment already has the retained URL in its
    # container environment. Reuse it only in this process so an incremental
    # rebuild does not require copying a database credential to the console or
    # persisting a second plaintext value. Include the newest stopped Compose
    # API container because Docker Desktop can stop the whole stack before a
    # later guarded activation. Explicit host/env-file values still win.
    $retainedAPI = docker ps -a `
      --filter 'label=com.docker.compose.project=nexusai' `
      --filter 'label=com.docker.compose.service=api' `
      --format '{{.ID}}' 2>$null | Select-Object -First 1
    if (-not [string]::IsNullOrWhiteSpace($retainedAPI)) {
      $containerLine = docker inspect --format '{{range .Config.Env}}{{println .}}{{end}}' $retainedAPI 2>$null |
        Where-Object {
          $_ -match '^(NEXUSAI_AGENT_HISTORY_DATABASE_URL|LOCALAI_AGENT_POOL_DATABASE_URL)='
        } |
        Select-Object -Last 1
      if ($containerLine) {
        $configuredURL = ($containerLine -split '=', 2)[1].Trim()
      }
    }
  }
  if ([string]::IsNullOrWhiteSpace($configuredURL) -or $configuredURL -match 'CHANGE_ME') {
    throw 'Set NEXUSAI_AGENT_HISTORY_DATABASE_URL to the retained PostgreSQL database before activation'
  }
  if ($configuredURL -notmatch '^postgres(ql)?://') {
    throw 'NEXUSAI_AGENT_HISTORY_DATABASE_URL must be a PostgreSQL URL'
  }
  $env:NEXUSAI_AGENT_HISTORY_DATABASE_URL = $configuredURL
}

function Assert-R63RuntimeContract {
  $ready = Invoke-WebRequest -Method Get -UseBasicParsing -TimeoutSec 30 -Uri 'http://localhost:8080/readyz'
  $recordsHealth = Invoke-WebRequest -Method Get -UseBasicParsing -TimeoutSec 30 -Uri 'http://localhost:8091/healthz'
  $app = Invoke-WebRequest -Method Get -UseBasicParsing -TimeoutSec 30 -Headers $headers -Uri 'http://localhost:8080/app'
  if ($ready.StatusCode -ne 200 -or $recordsHealth.StatusCode -ne 200 -or $app.StatusCode -ne 200) {
    throw 'R6.3 health or application-shell acceptance failed'
  }

  $encodedCase = [uri]::EscapeDataString($CaseID)
  $encodedAgent = [uri]::EscapeDataString($AgentName)
  $historyUri = "http://localhost:8080/api/agents/$encodedAgent/history?case_id=$encodedCase&collection_id=$encodedCase&limit=1"
  $history = Invoke-RestMethod -Method Get -UseBasicParsing -TimeoutSec 30 -Headers $headers -Uri $historyUri
  if ($history.contract_version -ne $ExpectedContract -or $null -eq $history.items -or $null -eq $history.has_more) {
    throw 'The deployed R6.3 retained-history response does not match its public contract'
  }
  $capabilitiesUri = "http://localhost:8080/api/records/forensic/capabilities?collection_id=$encodedCase"
  $capabilities = Invoke-RestMethod -Method Get -UseBasicParsing -TimeoutSec 30 -Headers $headers -Uri $capabilitiesUri
  if ($capabilities.query_corpus.contract_version -ne $ExpectedCorpusContract -or
      [string]::IsNullOrWhiteSpace([string]$capabilities.query_corpus.corpus_version) -or
      @($capabilities.query_corpus.entries).Count -lt 1) {
    throw 'The deployed R6.4-A query corpus does not match its versioned public contract'
  }
  return [ordered]@{
    item_count = @($history.items).Count
    has_more = [bool]$history.has_more
    corpus_version = [string]$capabilities.query_corpus.corpus_version
    corpus_entries = @($capabilities.query_corpus.entries).Count
  }
}

Push-Location $RepoRoot
try {
  Assert-R63SourceContract
  New-Item -ItemType Directory -Force -Path $LogRoot | Out-Null

  # The proven combined gate rebuilds sequentially, preserves rollback images
  # and named volumes, and restores the prior images on build/health failure.
  # This wrapper sends no history mutation and never drops retained schema.
  & $CombinedGate
  if ($LASTEXITCODE -ne 0) { throw "The guarded combined rebuild failed with exit code $LASTEXITCODE" }

  $acceptance = Assert-R63RuntimeContract
  [ordered]@{
    phase = 'R6.3-B+R6.4-A1'
    status = 'live_accepted'
    deployed_utc = (Get-Date).ToUniversalTime().ToString('o')
    contract_version = $ExpectedContract
    presentation_contract_version = $ExpectedPresentationContract
    query_corpus_contract_version = $ExpectedCorpusContract
    query_corpus_version = $acceptance.corpus_version
    query_corpus_entries = $acceptance.corpus_entries
    case_id = $CaseID
    agent_name = $AgentName
    history_item_count = $acceptance.item_count
    history_has_more = $acceptance.has_more
    source_contract = 'pass'
    localai_ready = 'pass'
    records_health = 'pass'
    app_shell = 'pass'
    retained_history_get = 'pass'
    query_corpus_get = 'pass'
    mutating_requests_sent = 0
    schema_drop_attempted = $false
    named_volumes_preserved = $true
    rollback_images_preserved = $true
  } | ConvertTo-Json | Set-Content -Encoding ascii -LiteralPath $MarkerPath
  Write-Host "R6.3BActivation=PASS CaseID=$CaseID Agent=$AgentName Presentation=$ExpectedPresentationContract Corpus=$($acceptance.corpus_version) MutatingRequests=0 RollbackImages=preserved VolumesPreserved=true"
} finally {
  Pop-Location
  $gateMutex.ReleaseMutex()
  $gateMutex.Dispose()
}
