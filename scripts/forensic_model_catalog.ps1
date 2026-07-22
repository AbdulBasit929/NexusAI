param(
  [string]$LocalAIUrl = "http://localhost:8080",
  [string]$CatalogPath = "",
  [ValidateSet("all", "structured_records", "knowledge_base_text", "documents_ocr", "audio_speech", "audio_events_voice", "images_vision", "video", "unknown_mixed")]
  [string]$EvidenceType = "all",
  [ValidateSet("cpu", "balanced", "accuracy", "throughput", "multilingual")]
  [string]$Profile = "balanced",
  [ValidateSet("plan", "status", "install")]
  [string]$Action = "plan",
  [int]$Limit = 0,
  [switch]$AssumeYes
)

$ErrorActionPreference = "Stop"

function Join-Url {
  param([string]$Base, [string]$Path)
  return $Base.TrimEnd("/") + "/" + $Path.TrimStart("/")
}

function Invoke-JsonRequest {
  param(
    [Parameter(Mandatory = $true)][string]$Method,
    [Parameter(Mandatory = $true)][string]$Uri,
    [object]$Body = $null
  )

  $params = @{
    Method = $Method
    Uri = $Uri
  }
  if ($null -ne $Body) {
    $params.ContentType = "application/json"
    $params.Body = ($Body | ConvertTo-Json -Depth 20 -Compress)
  }
  Invoke-RestMethod @params
}

function Get-DefaultCatalogPath {
  $root = Split-Path -Parent $PSScriptRoot
  return Join-Path $root "configuration\forensic_evidence_model_catalog.json"
}

function Get-AvailableModelIds {
  param([string]$BaseUrl)
  try {
    $models = Invoke-RestMethod -Method Get -Uri (Join-Url $BaseUrl "/v1/models")
    if ($null -eq $models.data) {
      return @()
    }
    return @($models.data | ForEach-Object { $_.id } | Where-Object { -not [string]::IsNullOrWhiteSpace($_) })
  } catch {
    Write-Warning "Could not read $BaseUrl/v1/models: $($_.Exception.Message)"
    return @()
  }
}

function Test-CandidateInstalled {
  param([object]$Candidate, [string[]]$Installed)
  if ($Installed.Count -eq 0) {
    return $false
  }
  $localai = $Candidate.localai
  if ($null -ne $localai -and -not [string]::IsNullOrWhiteSpace($localai.gallery_id)) {
    if ($Installed -contains [string]$localai.gallery_id) {
      return $true
    }
  }
  if ($null -ne $localai -and $null -ne $localai.match_terms) {
    foreach ($modelId in $Installed) {
      $allTerms = $true
      foreach ($term in @($localai.match_terms)) {
        if ($modelId -notlike "*$term*") {
          $allTerms = $false
          break
        }
      }
      if ($allTerms) {
        return $true
      }
    }
  }
  return $false
}

function Test-ProfileMatch {
  param([object]$Candidate, [string]$ProfileName)
  if ($null -eq $Candidate.profiles) {
    return $true
  }
  return @($Candidate.profiles) -contains $ProfileName
}

function Get-CatalogRecommendations {
  param([object]$Catalog, [string]$EvidenceTypeName, [string]$ProfileName)

  $items = @()
  foreach ($property in $Catalog.evidence_types.PSObject.Properties) {
    if ($EvidenceTypeName -ne "all" -and $property.Name -ne $EvidenceTypeName) {
      continue
    }
    $type = $property.Value
    $candidates = @($type.recommended_models | Where-Object { Test-ProfileMatch -Candidate $_ -ProfileName $ProfileName })
    if ($Limit -gt 0) {
      $candidates = @($candidates | Sort-Object priority | Select-Object -First $Limit)
    } else {
      $candidates = @($candidates | Sort-Object priority)
    }
    $items += [pscustomobject]@{
      EvidenceType = $property.Name
      Label = $type.label
      Pipeline = @($type.primary_pipeline)
      BenchmarkTasks = @($type.benchmark_tasks)
      Candidates = $candidates
    }
  }
  return $items
}

function Install-Candidate {
  param([string]$BaseUrl, [object]$Candidate)

  $galleryId = ""
  if ($null -ne $Candidate.localai) {
    $galleryId = [string]$Candidate.localai.gallery_id
  }
  if ([string]::IsNullOrWhiteSpace($galleryId)) {
    Write-Warning "No LocalAI gallery_id is configured for '$($Candidate.name)'. Install/configure its adapter manually, then rerun -Action status."
    return
  }

  if (-not $AssumeYes) {
    $answer = Read-Host "Install '$($Candidate.name)' from LocalAI gallery id '$galleryId'? This may download large model files. Type YES to continue"
    if ($answer -ne "YES") {
      Write-Host "Skipped $($Candidate.name)"
      return
    }
  }

  Write-Host "Installing $galleryId via /models/apply ..."
  $response = Invoke-JsonRequest -Method Post -Uri (Join-Url $BaseUrl "/models/apply") -Body @{ id = $galleryId }
  $response | ConvertTo-Json -Depth 8
}

if ([string]::IsNullOrWhiteSpace($CatalogPath)) {
  $CatalogPath = Get-DefaultCatalogPath
}
$CatalogPath = [System.IO.Path]::GetFullPath($CatalogPath)
if (!(Test-Path -LiteralPath $CatalogPath)) {
  throw "Catalog not found: $CatalogPath"
}

$LocalAIUrl = $LocalAIUrl.TrimEnd("/")
$catalog = Get-Content -LiteralPath $CatalogPath -Raw | ConvertFrom-Json
$installed = Get-AvailableModelIds -BaseUrl $LocalAIUrl
$recommendationGroups = Get-CatalogRecommendations -Catalog $catalog -EvidenceTypeName $EvidenceType -ProfileName $Profile

Write-Host "Forensic evidence model catalog: $CatalogPath"
Write-Host "Profile: $Profile"
Write-Host "Action: $Action"
Write-Host "LocalAI: $LocalAIUrl"
Write-Host "Installed models detected: $($installed.Count)"
Write-Host ""

foreach ($group in $recommendationGroups) {
  Write-Host "## $($group.Label) [$($group.EvidenceType)]"
  Write-Host "Pipeline: $($group.Pipeline -join ' -> ')"
  foreach ($candidate in $group.Candidates) {
    $isInstalled = Test-CandidateInstalled -Candidate $candidate -Installed $installed
    $state = if ($isInstalled) { "installed/matched" } else { "not installed" }
    $galleryId = ""
    if ($null -ne $candidate.localai) {
      $galleryId = [string]$candidate.localai.gallery_id
    }
    $displayGallery = if ([string]::IsNullOrWhiteSpace($galleryId)) { "manual/adapter" } else { $galleryId }
    Write-Host (" - ({0}) {1}: {2} [{3}]" -f $candidate.priority, $candidate.name, $candidate.role, $state)
    Write-Host ("   backend: {0}; gallery_id: {1}" -f $candidate.backend, $displayGallery)
    if (-not [string]::IsNullOrWhiteSpace($candidate.notes)) {
      Write-Host "   note: $($candidate.notes)"
    }
    if ($Action -eq "install" -and -not $isInstalled) {
      Install-Candidate -BaseUrl $LocalAIUrl -Candidate $candidate
    }
  }
  Write-Host "Benchmark focus: $($group.BenchmarkTasks -join '; ')"
  Write-Host ""
}

Write-Host "Suggested benchmark command:"
Write-Host "  python scripts\benchmark_forensic_models.py --localai-url $LocalAIUrl --catalog $CatalogPath --profile $Profile --output reports\forensic-model-benchmark.json"
