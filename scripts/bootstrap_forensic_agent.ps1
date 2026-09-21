param(
  [string]$LocalAIUrl = "http://localhost:8080",
  [string]$AgentName = "Forensic_Records_Analyst",
  [string]$Model = "auto",
  [string]$CollectionId = "records-demo",
  [string]$TenantId = "default",
  [string]$ForensicRecordsApiUrl = "http://host.docker.internal:8091",
  [int]$KnowledgeBaseResults = 5,
  [string]$ProfilePath = "",
  [switch]$Force
)

$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($ProfilePath)) {
  $ProfilePath = Join-Path $PSScriptRoot "..\configuration\forensic_agent_profiles.json"
}
$resolvedProfilePath = (Resolve-Path -LiteralPath $ProfilePath).Path
$profileCatalog = Get-Content -LiteralPath $resolvedProfilePath -Raw | ConvertFrom-Json
$profile = @($profileCatalog.profiles | Where-Object { $_.agent_name -eq $AgentName }) | Select-Object -First 1
if ($null -eq $profile) {
  $availableProfiles = @($profileCatalog.profiles | ForEach-Object { $_.agent_name }) -join ", "
  throw "No forensic agent profile is defined for '$AgentName' in $resolvedProfilePath. Available profiles: $availableProfiles"
}
if ([string]::IsNullOrWhiteSpace([string]$profile.system_prompt) -or [string]::IsNullOrWhiteSpace([string]$profile.permanent_goal)) {
  throw "Forensic agent profile '$AgentName' must define system_prompt and permanent_goal."
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
    $params.Body = ($Body | ConvertTo-Json -Depth 20)
  }
  Invoke-RestMethod @params
}

function Get-AgentConfigUrl {
  param([string]$BaseUrl, [string]$Name)
  $escapedName = [Uri]::EscapeDataString($Name)
  "$BaseUrl/api/agents/$escapedName/config"
}

function Get-AgentUrl {
  param([string]$BaseUrl, [string]$Name)
  $escapedName = [Uri]::EscapeDataString($Name)
  "$BaseUrl/api/agents/$escapedName"
}

function Get-AvailableModelIds {
  param([string]$BaseUrl)
  try {
    $models = Invoke-RestMethod -Method Get -Uri "$BaseUrl/v1/models"
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
  param([string[]]$ModelIds)
  $nonChatPattern = "(?i)(embed|embedding|rerank|whisper|tts|stt|audio|vad|vision|clip)"
  $preferredPattern = "(?i)(instruct|chat|qwen|llama|mistral|gemma|phi|granite|deepseek)"

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

$LocalAIUrl = $LocalAIUrl.TrimEnd("/")
$createUrl = "$LocalAIUrl/api/agents"
$agentUrl = Get-AgentUrl -BaseUrl $LocalAIUrl -Name $AgentName
$configUrl = Get-AgentConfigUrl -BaseUrl $LocalAIUrl -Name $AgentName

Write-Host "Waiting for LocalAI agents API at $LocalAIUrl ..."
$ready = $false
for ($attempt = 1; $attempt -le 30; $attempt++) {
  try {
    Invoke-RestMethod -Method Get -Uri "$LocalAIUrl/api/agents" | Out-Null
    $ready = $true
    break
  } catch {
    Start-Sleep -Seconds 2
  }
}
if (-not $ready) {
  throw "LocalAI agents API did not become ready at $LocalAIUrl/api/agents"
}

$availableModels = Get-AvailableModelIds -BaseUrl $LocalAIUrl
$resolvedModel = $Model
if ([string]::IsNullOrWhiteSpace($resolvedModel) -or $resolvedModel -eq "auto") {
  $preferredModel = [string]$profileCatalog.model_policy.preferred_model
  if (-not [string]::IsNullOrWhiteSpace($preferredModel) -and $availableModels -contains $preferredModel) {
    $resolvedModel = $preferredModel
  } else {
    $resolvedModel = Select-ChatModelId -ModelIds $availableModels
  }
  if ([string]::IsNullOrWhiteSpace($resolvedModel)) {
    Write-Warning "No chat/instruct model was found in $LocalAIUrl/v1/models. Deterministic forensic prompts will still work after the current code is rebuilt, but general LLM answers require installing a chat model and rerunning this script with -Model <model-id>."
    $resolvedModel = ""
  } else {
    Write-Host "Using detected chat model: $resolvedModel"
  }
} elseif ($resolvedModel -match "(?i)(embed|embedding|rerank|whisper|tts|stt|audio|vad|vision|clip)") {
  throw "Configured model '$resolvedModel' is not a chat/instruct synthesis model. Choose a chat model; deterministic forensic operations do not require an LLM."
} elseif ($availableModels.Count -gt 0 -and $availableModels -notcontains $resolvedModel) {
  Write-Warning "Configured model '$resolvedModel' is not currently listed by $LocalAIUrl/v1/models. Deterministic forensic prompts can still run, but general LLM answers will fail until this model is installed or the agent is reconfigured."
}

$cancelPrevious = $true
$systemPrompt = [string]$profile.system_prompt
$permanentGoal = [string]$profile.permanent_goal
$userPromptContract = $profile.user_prompt_contract | ConvertTo-Json -Depth 10 -Compress

$agentConfig = [ordered]@{
  name = $AgentName
  description = [string]$profile.description
  model = $resolvedModel
  # The governed records agents are text-only. Assigning the chat model to the
  # multimodal slot falsely advertises image capability and can route binary
  # evidence to a model that was never approved for vision work.
  multimodal_model = ""

  enable_kb = $true
  kb_mode = "both"
  kb_results = $KnowledgeBaseResults
  kb_auto_search = $true
  kb_as_tools = $true
  enable_kb_compaction = $false
  kb_compaction_interval = "daily"
  kb_compaction_summarize = $true
  long_term_memory = $false
  summary_long_term_memory = $false
  conversation_storage_mode = "user_only"

  enable_forensic_records = $true
  forensic_records_api_url = $ForensicRecordsApiUrl
  forensic_records_api_key = ""
  forensic_tenant_id = $TenantId
  forensic_collection_id = $CollectionId

  system_prompt = $systemPrompt
  permanent_goal = $permanentGoal
  skills_prompt = "Analyst request contract (guidance only; never invent missing inputs): $userPromptContract"
  inner_monologue_template = ""
  scheduler_task_template = ""

  strip_thinking_tags = $true
  enable_auto_compaction = $false
  auto_compaction_threshold = 4096
  standalone_job = $false
  initiate_conversations = $false
  enable_planning = $false
  cancel_previous_on_new_message = $cancelPrevious
  loop_detection = 5
  can_stop_itself = $false
  scheduler_poll_interval = "30s"
  enable_reasoning = $false
  enable_reasoning_tool = $true
  enable_reasoning_for_instruct = $true
  enable_guided_tools = $false
  enable_skills = $false
  skills_mode = "prompt"
  parallel_jobs = 2
  disable_sink_state = $false
  enable_evaluation = $false
  max_evaluation_loops = 2
  max_attempts = 1
  last_message_duration = "5m"
  max_iterations = 4

  connectors = @()
  actions = @()
  dynamic_prompts = @()
  mcp_servers = @()
  mcp_stdio_servers = @{}
  filters = @()
}

$existing = $null
try {
  $existing = Invoke-RestMethod -Method Get -Uri $configUrl
} catch {
  $existing = $null
}

if ($null -ne $existing -and $Force) {
  Write-Host "Deleting existing agent '$AgentName' because -Force was supplied ..."
  Invoke-RestMethod -Method Delete -Uri $agentUrl | Out-Null
  $existing = $null
}

if ($null -ne $existing) {
  Write-Host "Updating existing agent '$AgentName' ..."
  Invoke-JsonRequest -Method Put -Uri $agentUrl -Body $agentConfig | Out-Null
} else {
  Write-Host "Creating agent '$AgentName' ..."
  Invoke-JsonRequest -Method Post -Uri $createUrl -Body $agentConfig | Out-Null
}

Write-Host ""
Write-Host "Verification:"
$agents = Invoke-RestMethod -Method Get -Uri "$LocalAIUrl/api/agents"
$agents | ConvertTo-Json -Depth 8

Write-Host ""
Write-Host "Profile: $($profile.agent_name) v$($profile.version) ($resolvedProfilePath)"
Write-Host "Families: $(@($profile.family_ids) -join ', ')"
Write-Host "Model role: $($profileCatalog.model_policy.role); deterministic operations remain authoritative"
Write-Host ""
if ([string]::IsNullOrWhiteSpace($resolvedModel)) {
  Write-Host "Model mode: deterministic forensic mode only. Install a chat/instruct model later for open-ended LLM answers."
} else {
  Write-Host "Model mode: $resolvedModel"
}

Write-Host ""
Write-Host "Open this chat page:"
Write-Host "$LocalAIUrl/app/agents/$AgentName/chat"
