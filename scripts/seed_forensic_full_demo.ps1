param(
  [string]$CollectionId = 'nexusai-forensic-demo',
  [string]$LocalAIUrl = 'http://localhost:8080',
  [string]$RecordsApiUrl = 'http://localhost:8091',
  [int]$TimeoutSeconds = 600
)

$ErrorActionPreference = 'Stop'
$RepoRoot = Split-Path -Parent $PSScriptRoot
$RuntimeEnvPath = Join-Path $RepoRoot '.env.forensic-runtime.local'

if ($CollectionId -ne 'nexusai-forensic-demo') {
  throw 'The clean consolidated seed is intentionally restricted to nexusai-forensic-demo'
}
if (-not (Test-Path -LiteralPath $RuntimeEnvPath)) {
  throw "Runtime environment is missing: $RuntimeEnvPath"
}

$runtime = ((Get-Content -LiteralPath $RuntimeEnvPath |
  Where-Object { $_ -and -not $_.StartsWith('#') }) -join "`n") |
  ConvertFrom-StringData
if ($runtime.FORENSIC_RECORDS_API_KEY -notmatch '^[0-9a-f]{64}$') {
  throw 'Runtime forensic API key has an invalid shape'
}

$recordFiles = @(
  @{ Path = Join-Path $RepoRoot 'fixtures\forensic_seed\records-demo\seed_cdr_large.csv'; Type = 'cdr' },
  @{ Path = Join-Path $RepoRoot 'fixtures\forensic_seed\records-demo\seed_anpr.csv'; Type = 'anpr' },
  @{ Path = Join-Path $RepoRoot 'fixtures\forensic_seed\records-demo\seed_ipdr.csv'; Type = 'ipdr' },
  @{ Path = Join-Path $RepoRoot 'fixtures\forensic_seed\records-demo\seed_access_log.csv'; Type = 'access_log' },
  @{ Path = Join-Path $RepoRoot 'fixtures\forensic_seed\records-demo\seed_transactions.csv'; Type = 'transaction' },
  @{ Path = Join-Path $RepoRoot 'ingestion\forensic_records\tests\fixtures\pakistan_subscriber_populated_acceptance_v1.jsonl'; Type = 'subscriber' },
  @{ Path = Join-Path $RepoRoot 'ingestion\forensic_records\tests\fixtures\pakistan_tower_sector_messy_synthetic.csv'; Type = 'tower_location' },
  @{ Path = Join-Path $RepoRoot 'ingestion\forensic_records\tests\fixtures\pakistan_cdr_tsv_synthetic.tsv'; Type = 'cdr' }
)
$kbFiles = @(
  (Join-Path $RepoRoot 'fixtures\forensic_seed\records-demo\policy_records_handling.md'),
  (Join-Path $RepoRoot 'fixtures\forensic_seed\records-demo\case_notes_records_demo.txt'),
  (Join-Path $RepoRoot 'fixtures\forensic_seed\records-demo\identity_case_notes.md')
)
foreach ($path in @($recordFiles.Path) + $kbFiles) {
  if (-not (Test-Path -LiteralPath $path)) { throw "Required demo fixture is missing: $path" }
}

function Invoke-CurlJSON {
  param([Parameter(Mandatory=$true)][string[]]$Arguments)
  $raw = & curl.exe --fail-with-body -sS @Arguments
  if ($LASTEXITCODE -ne 0) { throw "curl failed with exit $LASTEXITCODE" }
  return $raw | ConvertFrom-Json
}

function Invoke-CurlText {
  param([Parameter(Mandatory=$true)][string[]]$Arguments)
  $raw = (& curl.exe --fail-with-body -sS @Arguments) -join "`n"
  if ($LASTEXITCODE -ne 0) { throw "curl failed with exit $LASTEXITCODE" }
  return $raw
}

$collections = Invoke-CurlJSON -Arguments @('-X', 'GET', "$($LocalAIUrl.TrimEnd('/'))/api/agents/collections")
$collectionExists = $CollectionId -in @($collections.collections)
if (-not $collectionExists) {
  [void](Invoke-RestMethod -Method Post -TimeoutSec 30 `
    -Uri "$($LocalAIUrl.TrimEnd('/'))/api/agents/collections" `
    -ContentType 'application/json' `
    -Body (@{ name = $CollectionId } | ConvertTo-Json -Compress))
} else {
  Write-Host "Validating existing clean demo collection without duplicating evidence: $CollectionId"
}

$escapedCollection = [uri]::EscapeDataString($CollectionId)
if (-not $collectionExists) {
  foreach ($path in $kbFiles) {
    Write-Host "Adding governed demo context: $(Split-Path -Leaf $path)"
    [void](Invoke-CurlJSON -Arguments @(
      '-X', 'POST',
      '-F', "file=@$path",
      '-F', 'skip_forensic_records=true',
      "$($LocalAIUrl.TrimEnd('/'))/api/agents/collections/$escapedCollection/upload"
    ))
  }
}

$uploaded = @()
$evidenceURL = "$($LocalAIUrl.TrimEnd('/'))/api/records/forensic/evidence?collection_id=$escapedCollection&limit=100"
foreach ($record in $recordFiles) {
  $filename = Split-Path -Leaf $record.Path
  $encodedFilename = [uri]::EscapeDataString($filename)
  $existingRaw = Invoke-CurlText -Arguments @('-X', 'GET', "$evidenceURL&q=$encodedFilename")
  if ($existingRaw -match ('"original_filename":"' + [regex]::Escape($filename) + '"')) {
    $evidenceMatch = [regex]::Match($existingRaw, '"evidence_id":"([0-9a-f-]{36})"')
    $uploaded += [pscustomobject]@{
      filename = $filename
      record_type = $record.Type
      evidence_id = $evidenceMatch.Groups[1].Value
      action = 'already_present'
    }
    continue
  }
  Write-Host "Registering $filename as $($record.Type)"
  $response = Invoke-CurlJSON -Arguments @(
    '-X', 'POST',
    '-H', "Authorization: Bearer $($runtime.FORENSIC_RECORDS_API_KEY)",
    '-H', "X-Forensic-Tenant-ID: $($runtime.FORENSIC_RECORDS_TENANT_ID)",
    '-H', 'X-Forensic-Actor-ID: nexusai-demo-seeder',
    '-H', 'X-Forensic-Subject-ID: nexusai-demo-seeder',
    '-H', 'X-Forensic-Actor-Role: admin',
    '-H', "X-Forensic-Collection-ID: $CollectionId",
    '-H', "X-Forensic-Case-ID: $CollectionId",
    '-F', "tenant_id=$($runtime.FORENSIC_RECORDS_TENANT_ID)",
    '-F', "collection_id=$CollectionId",
    '-F', "case_id=$CollectionId",
    '-F', "record_type=$($record.Type)",
    '-F', 'jurisdiction=PK',
    '-F', 'source_timezone=Asia/Karachi',
    '-F', "file=@$($record.Path)",
    "$($RecordsApiUrl.TrimEnd('/'))/webhooks/records/upload"
  )
  $uploaded += [pscustomobject]@{
    filename = $filename
    record_type = $record.Type
    evidence_id = $response.evidence_id
    job_id = $response.job_id
    action = 'uploaded'
  }
}

$requiredNames = @($recordFiles | ForEach-Object { Split-Path -Leaf $($_.Path) })
$deadline = (Get-Date).AddSeconds($TimeoutSeconds)
do {
  $completedNames = @()
  foreach ($filename in $requiredNames) {
    $encodedFilename = [uri]::EscapeDataString($filename)
    $itemRaw = Invoke-CurlText -Arguments @('-X', 'GET', "$evidenceURL&q=$encodedFilename")
    if ($itemRaw -match ('"original_filename":"' + [regex]::Escape($filename) + '"') -and
        $itemRaw -match '"processing_status":"completed"') {
      $completedNames += $filename
    }
  }
  if ($completedNames.Count -eq $requiredNames.Count) { break }
  Start-Sleep -Seconds 2
} until ((Get-Date) -gt $deadline)

if ($completedNames.Count -ne $requiredNames.Count) {
  $missing = @($requiredNames | Where-Object { $_ -notin $completedNames })
  throw "Demo seed did not complete within $TimeoutSeconds seconds; incomplete: $($missing -join ', ')"
}

$acceptedRows = 0
$duplicateRows = 0
$rejectedRows = 0
foreach ($filename in $requiredNames) {
  $encodedFilename = [uri]::EscapeDataString($filename)
  $itemRaw = Invoke-CurlText -Arguments @('-X', 'GET', "$evidenceURL&q=$encodedFilename")
  $acceptedMatch = [regex]::Match($itemRaw, '"accepted_rows":(\d+)')
  $duplicateMatch = [regex]::Match($itemRaw, '"duplicate_rows":(\d+)')
  $rejectedMatch = [regex]::Match($itemRaw, '"rejected_rows":(\d+)')
  if ($acceptedMatch.Success) { $acceptedRows += [int]$acceptedMatch.Groups[1].Value }
  if ($duplicateMatch.Success) { $duplicateRows += [int]$duplicateMatch.Groups[1].Value }
  if ($rejectedMatch.Success) { $rejectedRows += [int]$rejectedMatch.Groups[1].Value }
}
$summary = [ordered]@{
  collection_id = $CollectionId
  evidence_ids = $uploaded
  structured_files = $completedNames.Count
  accepted_rows = $acceptedRows
  duplicate_rows = $duplicateRows
  rejected_rows = $rejectedRows
  failed = 0
  generated_at = (Get-Date).ToUniversalTime().ToString('o')
}
$summary | ConvertTo-Json -Depth 8
Write-Host "FullDemoSeed=PASS Collection=$CollectionId Files=$($completedNames.Count) AcceptedRows=$acceptedRows DuplicateRows=$duplicateRows RejectedRows=$rejectedRows"
