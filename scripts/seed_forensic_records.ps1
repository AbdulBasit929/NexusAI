param(
  [string]$SeedDir = "fixtures/forensic_seed/records-demo",
  [string]$TenantId = "default",
  [string]$CollectionId = "records-demo",
  [string]$RecordsApiUrl = "http://localhost:8091",
  [string]$LocalAIUrl = "http://localhost:8080",
  [switch]$Force,
  [switch]$SkipKbDocuments
)

$ErrorActionPreference = "Stop"

function Join-Url([string]$Base, [string]$Path) {
  return $Base.TrimEnd("/") + "/" + $Path.TrimStart("/")
}

function Upload-RecordFile([string]$Path, [string]$RecordType) {
  $args = @(
    "-sS",
    "-X", "POST", (Join-Url $RecordsApiUrl "/webhooks/records/upload"),
    "-F", "tenant_id=$TenantId",
    "-F", "collection_id=$CollectionId",
    "-F", "record_type=$RecordType"
  )
  if ($Force) {
    $args += @("-F", "force=true")
  }
  $args += @("-F", "file=@$Path")
  Write-Host "Uploading records: $Path as $RecordType"
  $raw = & curl.exe @args
  try {
    $raw | ConvertFrom-Json
  } catch {
    $raw
  }
}

function Upload-KbDocument([string]$Path) {
  Write-Host "Uploading KB document: $Path"
  & curl.exe -sS -X POST (Join-Url $LocalAIUrl "/api/agents/collections/$([uri]::EscapeDataString($CollectionId))/upload") -F "file=@$Path"
}

if (!(Test-Path -LiteralPath $SeedDir)) {
  throw "Seed directory not found: $SeedDir. Run scripts/generate_forensic_seed_data.py first."
}

$recordFiles = @(
  @{ Path = "seed_cdr_large.csv"; Type = "cdr" },
  @{ Path = "seed_anpr.csv"; Type = "anpr" },
  @{ Path = "seed_ipdr.csv"; Type = "ipdr" },
  @{ Path = "seed_access_log.csv"; Type = "access_log" }
)

$responses = @()
foreach ($item in $recordFiles) {
  $path = Join-Path $SeedDir $item.Path
  if (Test-Path -LiteralPath $path) {
    $responses += Upload-RecordFile -Path $path -RecordType $item.Type
  }
}

if (!$SkipKbDocuments) {
  foreach ($doc in @("policy_records_handling.md", "case_notes_records_demo.txt")) {
    $path = Join-Path $SeedDir $doc
    if (Test-Path -LiteralPath $path) {
      Upload-KbDocument -Path $path | Out-Null
    }
  }
}

Write-Host ""
Write-Host "Seed upload summary"
$responses | ConvertTo-Json -Depth 8
