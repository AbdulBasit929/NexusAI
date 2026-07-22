param(
  [string]$LocalAIUrl = "http://localhost:8080",
  [string]$AgentName = "Forensic_Records_Analyst",
  [string]$CollectionId = "records-demo",
  [string]$TenantId = "default",
  [string]$ForensicRecordsApiUrl = "http://host.docker.internal:8091",
  [string]$Model = "auto",
  [string]$ModelId = "",
  [string]$ModelUrl = "",
  [string]$ModelName = "",
  [int]$KnowledgeBaseResults = 5,
  [int]$TimeoutMinutes = 45,
  [switch]$ConfigureAgent,
  [switch]$SkipInstall,
  [switch]$ForceAgent
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

function Select-ChatModelId {
  param([string[]]$ModelIds, [string]$PreferredName = "")
  $nonChatPattern = "(?i)(embed|embedding|rerank|whisper|tts|stt|audio|vad|vision|clip)"
  $preferredPattern = "(?i)(instruct|chat|qwen|llama|mistral|gemma|phi|granite|deepseek|hermes|orca)"

  if (-not [string]::IsNullOrWhiteSpace($PreferredName)) {
    $exact = @($ModelIds | Where-Object { $_ -eq $PreferredName -and $_ -notmatch $nonChatPattern })
    if ($exact.Count -gt 0) {
      return $exact[0]
    }
    $contains = @($ModelIds | Where-Object { $_ -like "*$PreferredName*" -and $_ -notmatch $nonChatPattern })
    if ($contains.Count -gt 0) {
      return $contains[0]
    }
  }

  $preferred = @($ModelIds | Where-Object { $_ -notmatch $nonChatPattern -and $_ -match $preferredPattern })
  if ($preferred.Count -gt 0) {
    return $preferred[0]
  }

  $fallback = @($ModelIds | Where-Object { $_ -notmatch $nonChatPattern })
  if ($fallback.Count -gt 0) {
    return $fallback[0]
  }

  return ""
}

function Wait-ForLocalAI {
  param([string]$BaseUrl)
  Write-Host "Waiting for LocalAI at $BaseUrl ..."
  for ($attempt = 1; $attempt -le 60; $attempt++) {
    try {
      Invoke-RestMethod -Method Get -Uri (Join-Url $BaseUrl "/v1/models") | Out-Null
      return
    } catch {
      Start-Sleep -Seconds 2
    }
  }
  throw "LocalAI did not become ready at $BaseUrl"
}

function Install-Model {
  param([string]$BaseUrl, [string]$GalleryId, [string]$ConfigUrl, [string]$Name)

  if (-not [string]::IsNullOrWhiteSpace($GalleryId)) {
    $body = [ordered]@{ id = $GalleryId }
  } elseif (-not [string]::IsNullOrWhiteSpace($ConfigUrl)) {
    $body = [ordered]@{ url = $ConfigUrl }
    if (-not [string]::IsNullOrWhiteSpace($Name)) {
      $body.name = $Name
    }
  } else {
    return $null
  }

  Write-Host "Starting LocalAI model install via /models/apply ..."
  $response = Invoke-JsonRequest -Method Post -Uri (Join-Url $BaseUrl "/models/apply") -Body $body
  $response | ConvertTo-Json -Depth 8
  return $response
}

function Wait-ForModelJob {
  param([string]$StatusUrl, [datetime]$Deadline)

  if ([string]::IsNullOrWhiteSpace($StatusUrl)) {
    return
  }

  Write-Host "Waiting for model install job: $StatusUrl"
  while ((Get-Date) -lt $Deadline) {
    try {
      $status = Invoke-RestMethod -Method Get -Uri $StatusUrl
      $status | ConvertTo-Json -Depth 8
      if ($status.processed -eq $true -or $status.status -match "(?i)(complete|completed|success|failed|error)") {
        return
      }
    } catch {
      Write-Warning "Could not read model job yet: $($_.Exception.Message)"
    }
    Start-Sleep -Seconds 10
  }
}

function Resolve-ChatModel {
  param(
    [string]$BaseUrl,
    [string]$RequestedModel,
    [string]$PreferredName,
    [datetime]$Deadline
  )

  while ((Get-Date) -lt $Deadline) {
    $modelIds = Get-AvailableModelIds -BaseUrl $BaseUrl
    if (-not [string]::IsNullOrWhiteSpace($RequestedModel) -and $RequestedModel -ne "auto") {
      if ($modelIds -contains $RequestedModel) {
        return $RequestedModel
      }
    } else {
      $selected = Select-ChatModelId -ModelIds $modelIds -PreferredName $PreferredName
      if (-not [string]::IsNullOrWhiteSpace($selected)) {
        return $selected
      }
    }
    Start-Sleep -Seconds 10
  }

  return ""
}

$LocalAIUrl = $LocalAIUrl.TrimEnd("/")
$deadline = (Get-Date).AddMinutes($TimeoutMinutes)
Wait-ForLocalAI -BaseUrl $LocalAIUrl

$resolvedModel = ""
$availableModels = Get-AvailableModelIds -BaseUrl $LocalAIUrl
if (-not [string]::IsNullOrWhiteSpace($Model) -and $Model -ne "auto") {
  if ($availableModels -contains $Model) {
    $resolvedModel = $Model
  } else {
    Write-Warning "Requested model '$Model' is not currently installed."
  }
} else {
  $resolvedModel = Select-ChatModelId -ModelIds $availableModels -PreferredName $ModelName
}

if ([string]::IsNullOrWhiteSpace($resolvedModel) -and -not $SkipInstall) {
  if ([string]::IsNullOrWhiteSpace($ModelId) -and [string]::IsNullOrWhiteSpace($ModelUrl)) {
    Write-Warning "No chat/instruct model is installed, and no -ModelId or -ModelUrl was supplied. I will not silently download a large model."
  } else {
    $job = Install-Model -BaseUrl $LocalAIUrl -GalleryId $ModelId -ConfigUrl $ModelUrl -Name $ModelName
    if ($null -ne $job -and -not [string]::IsNullOrWhiteSpace($job.status)) {
      Wait-ForModelJob -StatusUrl $job.status -Deadline $deadline
    } elseif ($null -ne $job -and -not [string]::IsNullOrWhiteSpace($job.statusURL)) {
      Wait-ForModelJob -StatusUrl $job.statusURL -Deadline $deadline
    }
    $resolvedModel = Resolve-ChatModel -BaseUrl $LocalAIUrl -RequestedModel $Model -PreferredName $ModelName -Deadline $deadline
  }
}

Write-Host ""
if ([string]::IsNullOrWhiteSpace($resolvedModel)) {
  Write-Warning "No usable chat/instruct model is available. Deterministic forensic tools can still answer exact records questions after the updated code is rebuilt, but open-ended LLM reasoning will fail until a chat model is installed."
  Write-Host "Current models:"
  Get-AvailableModelIds -BaseUrl $LocalAIUrl | ForEach-Object { Write-Host " - $_" }
  exit 2
}

Write-Host "Selected chat/instruct model: $resolvedModel"

if ($ConfigureAgent) {
  $bootstrapPath = Join-Path $PSScriptRoot "bootstrap_forensic_agent.ps1"
  if (!(Test-Path -LiteralPath $bootstrapPath)) {
    throw "bootstrap_forensic_agent.ps1 not found at $bootstrapPath"
  }

  $bootstrapArgs = @{
    LocalAIUrl = $LocalAIUrl
    AgentName = $AgentName
    Model = $resolvedModel
    CollectionId = $CollectionId
    TenantId = $TenantId
    ForensicRecordsApiUrl = $ForensicRecordsApiUrl
    KnowledgeBaseResults = $KnowledgeBaseResults
  }
  if ($ForceAgent) {
    $bootstrapArgs.Force = $true
  }

  Write-Host "Configuring forensic agent '$AgentName' with model '$resolvedModel' ..."
  & $bootstrapPath @bootstrapArgs
}

Write-Host ""
Write-Host "Next checks:"
Write-Host "  curl.exe $LocalAIUrl/v1/models"
Write-Host "  curl.exe $LocalAIUrl/api/agents"
Write-Host "  Open: $LocalAIUrl/app/agents/$AgentName/chat"
