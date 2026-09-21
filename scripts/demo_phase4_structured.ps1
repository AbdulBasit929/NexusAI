param(
  [string]$CdrPath = "$env:USERPROFILE\Downloads\923461678183.csv",
  [string]$PythonPath = "$env:LOCALAPPDATA\Programs\Python\Python313\python.exe",
  [string]$GoPath = "C:\Program Files\Go\bin\go.exe",
  [switch]$FullVerification,
  [switch]$LiveSmoke,
  [string]$LocalAIUrl = "http://localhost:8080",
  [string]$RecordsApiUrl = "http://localhost:8091",
  [string]$CollectionId = "records-demo"
)

$ErrorActionPreference = "Stop"
$RepoRoot = Split-Path -Parent $PSScriptRoot
$WorkerPath = Join-Path $RepoRoot "ingestion\forensic_records\worker.py"
$FixtureRoot = Join-Path $RepoRoot "ingestion\forensic_records\tests\fixtures"

function Assert-DemoCondition {
  param([bool]$Condition, [string]$Message)
  if (-not $Condition) {
    throw $Message
  }
}

function Invoke-StructuredAudit {
  param([string]$Path, [string]$RecordType)
  $json = & $PythonPath $WorkerPath audit-source $Path `
    --record-type $RecordType `
    --source-timezone Asia/Karachi `
    --jurisdiction PK
  if ($LASTEXITCODE -ne 0) {
    throw "Structured audit failed for $Path"
  }
  return $json | ConvertFrom-Json
}

Assert-DemoCondition (Test-Path -LiteralPath $PythonPath) "Python 3.13 was not found at $PythonPath"

Write-Host ""
Write-Host "NexusAI Phase 4 structured-intelligence demonstration"
Write-Host "Safety: audits are read-only and never emit raw rows or identifiers."

if (Test-Path -LiteralPath $CdrPath) {
  Write-Host ""
  Write-Host "1. Supplied real-format CDR CSV - privacy-safe offline audit"
  $cdr = Invoke-StructuredAudit -Path $CdrPath -RecordType "auto"
  Assert-DemoCondition ($cdr.routing.detected_record_type -eq "cdr") "The supplied CSV did not route to the CDR adapter."
  Assert-DemoCondition ($cdr.accounting.row_accounting_complete -eq $true) "CDR row accounting is incomplete."
  Assert-DemoCondition ($cdr.privacy.raw_rows_emitted -eq $false) "The audit unexpectedly emitted raw rows."
  [pscustomobject]@{
    Format = $cdr.source.extension
    Adapter = $cdr.routing.detected_record_type
    Rows = $cdr.accounting.total_rows
    Accepted = $cdr.accounting.accepted_rows
    Rejected = $cdr.accounting.rejected_rows
    ExactDuplicates = $cdr.accounting.exact_duplicate_rows
    UniqueNormalized = $cdr.accounting.unique_normalized_rows
    Encoding = $cdr.schema.encoding
    Delimiter = $cdr.schema.delimiter
    Timezone = $cdr.routing.source_timezone
  } | Format-Table -AutoSize
} else {
  Write-Warning "Supplied CDR was not found at $CdrPath; the synthetic family matrix will still run."
}

Write-Host ""
Write-Host "2. Pakistan-oriented structured-family acceptance matrix"
$goldenPath = Join-Path $FixtureRoot "pakistan_phase4_structured_goldens_v1.json"
$families = (Get-Content -Raw -LiteralPath $goldenPath | ConvertFrom-Json).acceptance_matrix
$labels = @{
  cdr = "CDR"
  ipdr = "IPDR"
  anpr = "ANPR"
  transaction = "Transactions"
  subscriber = "Subscriber"
  tower_location = "Tower/location"
  access_log = "Access/security"
}

$matrix = foreach ($family in $families) {
  $name = $labels[$family.record_type]
  $fixture = Join-Path $FixtureRoot $family.path
  $audit = Invoke-StructuredAudit -Path $fixture -RecordType $family.record_type
  Assert-DemoCondition ($audit.routing.detected_record_type -eq $family.record_type) "$name adapter mismatch."
  Assert-DemoCondition ($audit.accounting.total_rows -eq $family.total_rows) "$name total-row mismatch."
  Assert-DemoCondition ($audit.accounting.accepted_rows -eq $family.accepted_rows) "$name accepted-row mismatch."
  Assert-DemoCondition ($audit.accounting.rejected_rows -eq $family.rejected_rows) "$name rejected-row mismatch."
  Assert-DemoCondition ($audit.accounting.exact_duplicate_rows -eq $family.duplicate_rows) "$name duplicate-row mismatch."
  Assert-DemoCondition ($audit.accounting.unique_normalized_rows -eq $family.unique_normalized_rows) "$name unique-row mismatch."
  Assert-DemoCondition ($audit.accounting.row_accounting_complete -eq $true) "$name row accounting is incomplete."
  [pscustomobject]@{
    Family = $name
    Format = $audit.source.extension
    Total = $audit.accounting.total_rows
    Accepted = $audit.accounting.accepted_rows
    Rejected = $audit.accounting.rejected_rows
    Duplicates = $audit.accounting.exact_duplicate_rows
    Accounting = "PASS"
  }
}
$matrix | Format-Table -AutoSize

Write-Host "3. Demonstration points"
Write-Host "- CSV is a first-class CDR/IPDR/ANPR/tower source; XLSX is an additional adapter, not a replacement."
Write-Host "- Exact facts use deterministic adapters and SQL; models may only explain bounded returned evidence."
Write-Host "- Source hashes, rows, rejects, duplicates, timezones and locators stay auditable."
Write-Host "- Unsupported OCR/STT/video operations remain visibly pending until their own acceptance gates pass."

if ($FullVerification) {
  Write-Host ""
  Write-Host "4. Full source verification"
  & $PythonPath -m unittest `
    ingestion.forensic_records.tests.test_worker `
    ingestion.forensic_records.tests.test_phase2_adapters `
    ingestion.forensic_records.tests.test_phase4_xlsx `
    ingestion.forensic_records.tests.test_phase4_structured
  if ($LASTEXITCODE -ne 0) { throw "Python Phase 4 regression failed." }

  if (Test-Path -LiteralPath $GoPath) {
    & $GoPath test ./api/forensic_records -count=1
    if ($LASTEXITCODE -ne 0) { throw "Go forensic API regression failed." }
  } else {
    Write-Warning "Go was not found at $GoPath; Go verification was skipped."
  }

  Push-Location (Join-Path $RepoRoot "core\http\react-ui")
  try {
    & "C:\Program Files\nodejs\npm.cmd" run build
    if ($LASTEXITCODE -ne 0) { throw "React production build failed." }
  } finally {
    Pop-Location
  }
}

if ($LiveSmoke) {
  Write-Host ""
  Write-Host "5. Approved deployed-runtime smoke (synthetic/read-only queries only)"
  & (Join-Path $RepoRoot "scripts\smoke_forensic_records.ps1") `
    -LocalAIUrl $LocalAIUrl `
    -RecordsApiUrl $RecordsApiUrl `
    -CollectionId $CollectionId
  if ($LASTEXITCODE -ne 0) { throw "Live forensic smoke failed." }
}

Write-Host ""
Write-Host "Phase 4 structured demonstration completed successfully."
if (-not $LiveSmoke) {
  Write-Host "No source file was uploaded and no service/database state was changed."
}
