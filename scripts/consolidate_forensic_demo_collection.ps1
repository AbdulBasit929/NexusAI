param(
  [string]$CollectionId = 'nexusai-forensic-demo',
  [string]$LocalAIUrl = 'http://localhost:8080',
  [string]$ForensicAPIUrl = 'http://localhost:8091',
  [switch]$Apply
)

$ErrorActionPreference = 'Stop'
$requiredCollection = 'nexusai-forensic-demo'
if ($CollectionId -ne $requiredCollection) {
  throw "This guarded consolidation only retains $requiredCollection"
}

function Invoke-JsonRequest {
  param(
    [Parameter(Mandatory=$true)][string]$Method,
    [Parameter(Mandatory=$true)][string]$Uri,
    [string]$Body = ''
  )
  $parameters = @{
    Method = $Method
    Uri = $Uri
    TimeoutSec = 30
  }
  if ($Body) {
    $parameters.ContentType = 'application/json'
    $parameters.Body = $Body
  }
  Invoke-RestMethod @parameters
}

$ready = Invoke-WebRequest -UseBasicParsing -TimeoutSec 10 -Uri "$($LocalAIUrl.TrimEnd('/'))/readyz"
if ($ready.StatusCode -ne 200) { throw 'LocalAI is not ready' }
$forensicHealth = Invoke-WebRequest -UseBasicParsing -TimeoutSec 10 -Uri "$($ForensicAPIUrl.TrimEnd('/'))/healthz"
if ($forensicHealth.StatusCode -ne 200) { throw 'Forensic records API is not ready' }

$status = Invoke-JsonRequest -Method Get -Uri "$($LocalAIUrl.TrimEnd('/'))/api/records/forensic/status?collection_id=$([uri]::EscapeDataString($CollectionId))&limit=20"
if ([int64]$status.summary.evidence_total -ne 8 -or
    [int64]$status.summary.completed_jobs -ne 8 -or
    [int64]$status.summary.accepted_rows -ne 9272 -or
    [int64]$status.summary.failed_jobs -ne 0 -or
    [int64]$status.summary.evidence_in_flight -ne 0) {
  throw 'Retained demo collection does not match the accepted 8-evidence/9272-row completion contract'
}

$collectionResponse = Invoke-JsonRequest -Method Get -Uri "$($LocalAIUrl.TrimEnd('/'))/api/agents/collections"
$collections = @($collectionResponse.collections | ForEach-Object { [string]$_ } | Sort-Object -CaseSensitive -Unique)
if ($CollectionId -notin $collections) { throw "Retained collection is absent from LocalAI: $CollectionId" }
$removeCollections = @($collections | Where-Object { $_ -ne $CollectionId })

$dbInventorySQL = @"
SELECT collection_id, count(*) AS evidence_count, coalesce(sum(size_bytes), 0) AS evidence_bytes
FROM forensic.evidence_items
GROUP BY collection_id
ORDER BY collection_id;
SELECT collection_id, count(*) AS job_count, coalesce(sum(accepted_rows), 0) AS accepted_rows,
       coalesce(sum(duplicate_rows), 0) AS duplicate_rows, coalesce(sum(rejected_rows), 0) AS rejected_rows
FROM forensic.records_ingest_jobs
GROUP BY collection_id
ORDER BY collection_id;
"@
$dbInventory = & docker exec nexusai-forensic-postgres-1 psql -U localrecall -d localrecall -v ON_ERROR_STOP=1 -P pager=off -c $dbInventorySQL 2>&1
if ($LASTEXITCODE -ne 0) { throw 'Could not inventory forensic database collections' }

$reportRoot = Join-Path (Split-Path -Parent $PSScriptRoot) 'reports\runtime-activation-20260804'
New-Item -ItemType Directory -Force -Path $reportRoot | Out-Null
$inventoryPath = Join-Path $reportRoot 'phase7.2-demo-collection-consolidation.json'
$inventory = [ordered]@{
  phase = '7.2'
  generated_utc = (Get-Date).ToUniversalTime().ToString('o')
  apply_requested = [bool]$Apply
  retained_collection = $CollectionId
  retained_contract = [ordered]@{
    evidence_total = [int64]$status.summary.evidence_total
    completed_jobs = [int64]$status.summary.completed_jobs
    accepted_rows = [int64]$status.summary.accepted_rows
    duplicate_rows = [int64]$status.summary.duplicate_rows
    rejected_rows = [int64]$status.summary.rejected_rows
  }
  kb_collections_before = $collections
  kb_collections_to_remove = $removeCollections
  database_inventory_before = ($dbInventory -join "`n")
  outcome = if ($Apply) { 'in_progress' } else { 'dry_run_pass' }
}
$inventory | ConvertTo-Json -Depth 8 | Set-Content -Encoding ascii -LiteralPath $inventoryPath

if (-not $Apply) {
  Write-Host "DemoCollectionConsolidation=DRY_RUN Retain=$CollectionId RemoveKB=$($removeCollections.Count) Inventory=$inventoryPath"
  return
}

foreach ($collection in $removeCollections) {
  $escaped = [uri]::EscapeDataString($collection)
  Invoke-JsonRequest -Method Post -Uri "$($LocalAIUrl.TrimEnd('/'))/api/agents/collections/$escaped/reset" | Out-Null
}

$remainingResponse = Invoke-JsonRequest -Method Get -Uri "$($LocalAIUrl.TrimEnd('/'))/api/agents/collections"
$remaining = @($remainingResponse.collections | ForEach-Object { [string]$_ } | Sort-Object -CaseSensitive -Unique)
if ($remaining.Count -ne 1 -or $remaining[0] -ne $CollectionId) {
  throw "LocalAI collection cleanup did not converge to exactly $CollectionId"
}

$cleanupSQL = @"
BEGIN;
SET LOCAL session_replication_role = replica;
DELETE FROM forensic.processing_events WHERE collection_id <> '$CollectionId';
DELETE FROM forensic.evidence_custody_events WHERE collection_id <> '$CollectionId';
DELETE FROM forensic.derived_artifacts WHERE collection_id <> '$CollectionId';
DELETE FROM forensic.processing_runs WHERE collection_id <> '$CollectionId';
DELETE FROM forensic.evidence_source_links WHERE collection_id <> '$CollectionId';
UPDATE forensic.evidence_items SET current_version_id = NULL WHERE collection_id <> '$CollectionId';
UPDATE forensic.evidence_versions SET previous_version_id = NULL WHERE collection_id <> '$CollectionId';
DELETE FROM forensic.records_ingest_errors WHERE collection_id <> '$CollectionId';
DELETE FROM forensic.record_entities WHERE collection_id <> '$CollectionId';
DELETE FROM forensic.records WHERE collection_id <> '$CollectionId';
DELETE FROM forensic.cdr_records WHERE collection_id <> '$CollectionId';
DELETE FROM forensic.generic_records WHERE collection_id <> '$CollectionId';
DELETE FROM forensic.kb_active_metadata WHERE collection_id <> '$CollectionId';
DELETE FROM forensic.kb_collection_assets WHERE collection_id <> '$CollectionId';
DELETE FROM forensic.records_ingest_jobs WHERE collection_id <> '$CollectionId';
DELETE FROM forensic.records_audit_log WHERE collection_id IS NOT NULL AND collection_id <> '$CollectionId';
DELETE FROM forensic.evidence_versions WHERE collection_id <> '$CollectionId';
DELETE FROM forensic.evidence_storage_objects WHERE collection_id <> '$CollectionId';
DELETE FROM forensic.evidence_items WHERE collection_id <> '$CollectionId';
SET LOCAL session_replication_role = origin;
COMMIT;
"@
$cleanupOutput = & docker exec nexusai-forensic-postgres-1 psql -U localrecall -d localrecall -v ON_ERROR_STOP=1 -P pager=off -c $cleanupSQL 2>&1
if ($LASTEXITCODE -ne 0) { throw 'Scoped forensic database cleanup failed and was rolled back' }

$dbRemainingSQL = "SELECT DISTINCT collection_id FROM forensic.evidence_items UNION SELECT DISTINCT collection_id FROM forensic.records_ingest_jobs ORDER BY collection_id;"
$dbRemaining = & docker exec nexusai-forensic-postgres-1 psql -U localrecall -d localrecall -v ON_ERROR_STOP=1 -At -c $dbRemainingSQL 2>&1
if ($LASTEXITCODE -ne 0) { throw 'Could not verify forensic database cleanup' }
$dbRemainingCollections = @($dbRemaining | Where-Object { $_ -and $_ -notmatch '^\(' })
if ($dbRemainingCollections.Count -ne 1 -or $dbRemainingCollections[0] -ne $CollectionId) {
  throw "Forensic database cleanup did not converge to exactly $CollectionId"
}

$inventory.completed_utc = (Get-Date).ToUniversalTime().ToString('o')
$inventory.kb_collections_after = $remaining
$inventory.database_collections_after = $dbRemainingCollections
$inventory.database_cleanup_output = ($cleanupOutput -join "`n")
$inventory.outcome = 'pass'
$inventory | ConvertTo-Json -Depth 8 | Set-Content -Encoding ascii -LiteralPath $inventoryPath
Write-Host "DemoCollectionConsolidation=PASS Retain=$CollectionId RemovedKB=$($removeCollections.Count) DatabaseCollections=1 Inventory=$inventoryPath"
