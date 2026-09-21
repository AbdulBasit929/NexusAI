param(
  [string]$CollectionId = 'nexusai-forensic-demo',
  [string]$LocalAIUrl = 'http://localhost:8080',
  [int]$TimeoutSeconds = 210,
  [string]$AgentFilter = '',
  [string]$ReportPath = ''
)

$ErrorActionPreference = 'Stop'
if ($CollectionId -ne 'nexusai-forensic-demo') {
  throw 'The model-assisted Agent Chat matrix is restricted to nexusai-forensic-demo'
}

$tests = @(
  @{ Agent='Forensic_Records_Analyst'; Template='data_quality'; Prompt='Explain the review workflow supported by deterministic data quality analysis. Do not restate values or invent causes.' },
  @{ Agent='Communications_CDR_Analyst'; Template='frequent_contacts'; Prompt='Explain how an analyst should use the deterministic frequent contacts analysis for 923001110001. Do not restate values or infer relationships or communication content.' },
  @{ Agent='Network_IPDR_Capture_Analyst'; Template='ipdr_protocol_breakdown'; Prompt='Explain the review workflow supported by the deterministic network protocol breakdown. Do not restate values or infer payload content or maliciousness.' },
  @{ Agent='Vehicle_ANPR_Geospatial_Analyst'; Template='anpr_camera_activity'; Prompt='Explain the review workflow supported by deterministic ANPR camera activity. Do not restate values or infer a driver, owner, or route.' },
  @{ Agent='Subscriber_Identity_Analyst'; Template='subscriber_status_summary'; Prompt='Explain how an analyst should use the deterministic subscriber status summary when reviewing supplied sources. Do not restate values.' },
  @{ Agent='Tower_Location_Reference_Analyst'; Template='tower_status_summary'; Prompt='Explain the review workflow supported by the deterministic tower status summary. Do not restate values or infer RF coverage or device presence.' }
)
if ($AgentFilter) {
  $tests = @($tests | Where-Object { $_.Agent -eq $AgentFilter })
  if ($tests.Count -ne 1) { throw "Agent filter did not match exactly one configured model-assisted test: $AgentFilter" }
}

$reportRoot = Join-Path (Split-Path -Parent $PSScriptRoot) 'reports\runtime-activation-20260811'
New-Item -ItemType Directory -Force -Path $reportRoot | Out-Null
$reportName = if ($AgentFilter) { 'r6.6-predeployment-agent-chat-llm-regression.json' } else { 'r6.6-predeployment-agent-chat-llm-matrix.json' }
$reportPath = if ($ReportPath) { [System.IO.Path]::GetFullPath($ReportPath) } else { Join-Path $reportRoot $reportName }
$results = @()
foreach ($test in $tests) {
  Write-Host "Testing model-assisted Agent Chat: $($test.Agent)"
  $agent = [uri]::EscapeDataString($test.Agent)
  $sseURL = "$($LocalAIUrl.TrimEnd('/'))/api/agents/$agent/sse"
  $sseJob = Start-Job -ScriptBlock {
    param($url, $timeout)
    curl.exe -sS -N --max-time $timeout $url
  } -ArgumentList $sseURL, ($TimeoutSeconds + 10)

  $started = Get-Date
  $answer = $null
  $ack = $null
  try {
    Start-Sleep -Seconds 2
    $body = @{
      message = $test.Prompt
      case_id = $CollectionId
      collection_id = $CollectionId
    } | ConvertTo-Json -Compress
    $ack = Invoke-RestMethod -Method Post -TimeoutSec 15 `
      -Uri "$($LocalAIUrl.TrimEnd('/'))/api/agents/$agent/chat" `
      -ContentType 'application/json' -Body $body
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
      Start-Sleep -Seconds 5
      $lines = @(Receive-Job -Job $sseJob -Keep)
      foreach ($line in $lines) {
        if ($line -notlike 'data: *') { continue }
        try { $event = $line.Substring(6) | ConvertFrom-Json } catch { continue }
        $eventID = [string]$event.message_id
        $baseID = $eventID -replace '-agent$',''
        # The Agent Chat event contract now carries the originating request ID
        # on every final message. Ignore replayed/stale events from older chats.
        if ($event.sender -eq 'agent' -and $event.content -and $baseID -eq [string]$ack.message_id) {
          $answer = $event
          break
        }
      }
    } while (-not $answer -and (Get-Date) -lt $deadline)
  } finally {
    Stop-Job $sseJob -ErrorAction SilentlyContinue
    Remove-Job $sseJob -Force -ErrorAction SilentlyContinue
  }

  $content = if ($answer) { [string]$answer.content } else { '' }
  $result = [pscustomobject]@{
    agent = $test.Agent
    template = $test.Template
    prompt = $test.Prompt
    message_id = if ($ack) { [string]$ack.message_id } else { '' }
    completed = [bool]$answer
    latency_ms = [math]::Round(((Get-Date) - $started).TotalMilliseconds, 1)
    content_length = $content.Length
    has_deterministic_findings = $content -match '(?m)^### Answer\s*$'
    has_model_interpretation = $content -match '(?m)^### Plain-language interpretation\s*$'
    has_safe_model_fallback = $content -match 'Model note:|LLM synthesis rejected by forensic policy|defaulted to deterministic engine'
    selected_template_visible = $content -match [regex]::Escape($test.Template)
	internal_template_hidden = $content -notmatch '\*\*Template:\*\*'
	professional_heading_visible = $content -match '(?m)^##\s+\S'
    answer = $content
  }
  $results += $result
  # Persist progress after every agent so a host timeout cannot erase completed
  # evidence from a long CPU-only model matrix.
  $results | ConvertTo-Json -Depth 8 | Set-Content -Encoding utf8 -LiteralPath $reportPath
}

$failures = @($results | Where-Object {
  -not $_.completed -or -not $_.has_deterministic_findings -or
  (-not $_.has_model_interpretation -and -not $_.has_safe_model_fallback) -or
  -not $_.internal_template_hidden -or -not $_.professional_heading_visible
})
if ($failures.Count -gt 0) {
  $failedAgents = ($failures.agent -join ',')
  throw "Model-assisted Agent Chat matrix failed for: $failedAgents"
}
Write-Host "AgentChatLLMMatrix=PASS Agents=$($results.Count) Report=$reportPath"
