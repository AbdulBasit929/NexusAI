param(
  [string]$LocalAIUrl = "http://localhost:8080",
  [string]$RecordsApiUrl = "http://localhost:8091",
  [string]$CollectionId = "records-demo",
  [string]$TenantId = "default",
  [string]$Target = "923461678183",
  [switch]$SkipReport
)

$ErrorActionPreference = "Stop"

function Invoke-JsonPost {
  param(
    [string]$Uri,
    [hashtable]$Body
  )
  Invoke-RestMethod -Method Post -Uri $Uri -ContentType "application/json" -Body ($Body | ConvertTo-Json -Depth 12 -Compress)
}

function Assert-True {
  param(
    [bool]$Condition,
    [string]$Message
  )
  if (-not $Condition) {
    throw $Message
  }
}

Write-Host "Checking forensic records API health..."
$health = Invoke-RestMethod -Uri "$RecordsApiUrl/healthz"
Assert-True ($null -ne $health) "Forensic records API health check returned no response."

Write-Host "Checking LocalAI collections endpoint..."
$collections = Invoke-RestMethod -Uri "$LocalAIUrl/api/agents/collections"
Assert-True ($null -ne $collections) "LocalAI collections endpoint returned no response."

Write-Host "Checking deterministic template registry..."
$templates = Invoke-RestMethod -Uri "$RecordsApiUrl/query/templates"
$templateNames = @($templates.templates | ForEach-Object { $_.name })
foreach ($required in @("frequent_contacts", "shortest_call", "source_file_audit", "case_readiness", "executive_case_brief", "limitations_and_data_quality")) {
  Assert-True ($templateNames -contains $required) "Missing expected template: $required"
}

$queries = @(
  @{ name = "collection overview"; query = "which files were ingested"; expected_template = "source_file_audit"; expect_records = $true },
  @{ name = "frequent contacts"; query = "who are the frequent contacts?"; expected_template = "frequent_contacts"; expect_records = $true },
  @{ name = "shortest call"; query = "shortest call duration of $Target"; expected_template = "shortest_call"; expect_records = $true },
  @{ name = "mixed phone format"; query = "shortest call for +92 346 167 8183"; expected_template = "shortest_call"; expect_records = $true },
  @{ name = "case readiness"; query = "is this case ready for production use?"; expected_template = "case_readiness"; expect_records = $true; expect_readiness = $true },
  @{ name = "clarification"; query = "where was this number most often observed?"; expected_template = "top_locations"; expect_clarification = $true }
)

foreach ($item in $queries) {
  Write-Host "Running hybrid query smoke: $($item.name)"
  $body = @{
    tenant_id = $TenantId
    collection_id = $CollectionId
    query = $item.query
    limit = 10
    max_kb_results = 3
  }
  $result = Invoke-JsonPost -Uri "$RecordsApiUrl/query/hybrid" -Body $body
  Assert-True ($result.template -eq $item.expected_template) "Query '$($item.name)' routed to '$($result.template)', expected '$($item.expected_template)'."
  if ($item.expect_clarification) {
    Assert-True ($result.intent -eq "clarification") "Query '$($item.name)' should require clarification."
    Assert-True ($result.answer.clarification_required -eq $true) "Query '$($item.name)' did not return clarification metadata."
  }
  if ($item.expect_records) {
    Assert-True (($result.route -contains "records_sql") -or ($result.intent -eq "records")) "Query '$($item.name)' did not use records_sql."
    Assert-True ($null -ne $result.answer.records_status) "Query '$($item.name)' did not include records_status."
  }
  if ($item.expect_readiness) {
    Assert-True ($null -ne $result.records.readiness) "Query '$($item.name)' did not include readiness summary."
    Assert-True ($null -ne $result.records.readiness_checks) "Query '$($item.name)' did not include readiness checks."
  }
}

if (-not $SkipReport) {
  Write-Host "Running report generation smoke..."
  $report = Invoke-JsonPost -Uri "$RecordsApiUrl/reports/generate" -Body @{
    tenant_id = $TenantId
    collection_id = $CollectionId
    target = $Target
    include_evidence = $true
  }
  Assert-True ($report.markdown -match "Forensic Intelligence Report") "Report markdown missing title."
  Assert-True ($report.markdown -match "Key Findings") "Report markdown missing Key Findings."
  Assert-True ($report.markdown -match "Case Readiness") "Report markdown missing Case Readiness."
  Assert-True ($report.markdown -match "Recommended Next Steps") "Report markdown missing next steps."
}

Write-Host "Forensic records smoke checks completed successfully."
