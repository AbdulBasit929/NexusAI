param(
  [string]$LocalAIUrl = "http://localhost:8080",
  [string]$CollectionId = "records-demo",
  [string]$TenantId = "default",
  [string]$Model = "auto",
  [string]$ForensicRecordsApiUrl = "http://host.docker.internal:8091",
  [int]$KnowledgeBaseResults = 5,
  [switch]$Force
)

$ErrorActionPreference = "Stop"
$bootstrap = Join-Path $PSScriptRoot "bootstrap_forensic_agent.ps1"
$profiles = @(
  "Forensic_Records_Analyst",
  "Communications_CDR_Analyst",
  "Network_IPDR_Capture_Analyst",
  "Vehicle_ANPR_Geospatial_Analyst",
  "Subscriber_Identity_Analyst",
  "Tower_Location_Reference_Analyst"
)

foreach ($agentName in $profiles) {
  Write-Host ""
  Write-Host "=== Applying forensic profile: $agentName ==="
  $arguments = @{
    LocalAIUrl = $LocalAIUrl
    AgentName = $agentName
    Model = $Model
    CollectionId = $CollectionId
    TenantId = $TenantId
    ForensicRecordsApiUrl = $ForensicRecordsApiUrl
    KnowledgeBaseResults = $KnowledgeBaseResults
  }
  if ($Force) {
    $arguments.Force = $true
  }
  & $bootstrap @arguments
}

Write-Host ""
Write-Host "Forensic specialist profiles applied: $($profiles.Count)"
Write-Host "All specialists are bound to collection '$CollectionId' and tenant '$TenantId'."
