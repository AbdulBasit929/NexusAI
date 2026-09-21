# NexusAI live demo/proof script.
# Run section by section, not all at once.

cd "C:\Users\sheik\Workspace\Office\Projects\NexusAI"

# ============================================================
# SETUP — run once
# ============================================================
$envContent = Get-Content ".env.forensic-runtime.local" | Where-Object { $_ -match "^FORENSIC_RECORDS_API_KEY=" }
$apiKey = ($envContent -split "=", 2)[1]
$headers = @{
  "Authorization" = "Bearer $apiKey"
  "X-Forensic-Tenant-ID" = "default"
  "X-Forensic-Actor-ID" = "demo-analyst"
  "X-Forensic-Subject-ID" = "demo-analyst"
  "X-Forensic-Actor-Role" = "user"
  "X-Forensic-Collection-ID" = "nexusai-forensic-demo"
}

function Ask-NexusAI($question) {
  $body = @{ query = $question; collection_id = "nexusai-forensic-demo"; limit = 20; max_kb_results = 8 } | ConvertTo-Json
  $start = Get-Date
  $r = Invoke-RestMethod -Uri "http://localhost:8091/query/hybrid" -Method Post -ContentType "application/json" -Headers $headers -Body $body
  $elapsed = ((Get-Date) - $start).TotalSeconds

  Write-Host "`n> $question" -ForegroundColor Cyan
  Write-Host "  ($elapsed s, $($r.answer.records_row_count) row(s))" -ForegroundColor DarkGray

  $summaryText = if ($r.answer.llm_summary) { $r.answer.llm_summary } else { $r.answer.records_summary }
  Write-Host $summaryText -ForegroundColor Green

  # The actual data values, not just the summary sentence
  if ($r.records.source_native_results) {
    $r.records.source_native_results | Select-Object * -ExcludeProperty metadata | Format-Table -AutoSize
  } elseif ($r.records.call_type_breakdown) {
    $r.records.call_type_breakdown | Format-Table call_type, direction, event_count, first_seen, last_seen -AutoSize
  } elseif ($r.records.entity_activity) {
    $r.records.entity_activity | Format-Table -AutoSize
  } elseif ($r.records.image_metadata) {
    $r.records.image_metadata | Format-Table -AutoSize
  } elseif ($r.records.audio_metadata) {
    $r.records.audio_metadata | Format-Table -AutoSize
  } elseif ($r.records.video_metadata) {
    $r.records.video_metadata | Format-Table -AutoSize
  } elseif ($r.records.document_metadata) {
    $r.records.document_metadata | Format-Table -AutoSize
  }
  return $r
}

# ============================================================
# PART 1 — model responds, and produces a correct, grounded answer
# from real case data when the request is small.
# ============================================================

$body = @{
  model = "qwen3-4b-instruct-2507-q4km-nxb21d-dev"
  temperature = 0.7
  max_tokens = 60
  messages = @(@{ role = "user"; content = "Hello! Introduce yourself in one sentence." })
} | ConvertTo-Json -Depth 5
$start = Get-Date
$response = Invoke-RestMethod -Uri "http://localhost:8080/v1/chat/completions" -Method Post -ContentType "application/json" -Body $body
Write-Host "`n[$(((Get-Date) - $start).TotalSeconds) s]" -ForegroundColor Cyan
Write-Host $response.choices[0].message.content -ForegroundColor Green

$body = @{
  model = "qwen3-4b-instruct-2507-q4km-nxb21d-dev"
  temperature = 0
  max_tokens = 60
  messages = @(
    @{ role = "system"; content = "You are a forensic analyst. Summarize the fact in one sentence, citing its fact ID. Do not invent information." }
    @{ role = "user"; content = "Fact:`n[F1] This case contains 2,500 IPDR network sessions.`n`nQuestion: How many IPDR sessions are in this case?" }
  )
  response_format = @{
    type = "json_schema"
    json_schema = @{
      name = "narrative"; strict = $true
      schema = @{
        type = "object"
        properties = @{ summary = @{ type = "string" }; fact_id = @{ type = "string"; enum = @("F1") } }
        required = @("summary","fact_id"); additionalProperties = $false
      }
    }
  }
} | ConvertTo-Json -Depth 10
$start = Get-Date
$response = Invoke-RestMethod -Uri "http://localhost:8080/v1/chat/completions" -Method Post -ContentType "application/json" -Body $body
Write-Host "`n[$(((Get-Date) - $start).TotalSeconds) s]" -ForegroundColor Cyan
Write-Host $response.choices[0].message.content -ForegroundColor Green

# ============================================================
# PART 2 — the real system, exact answers, real values
# ============================================================

Ask-NexusAI "How many IPDR sessions are in this case?" | Out-Null
Ask-NexusAI "How many CDR records do we have in this case?" | Out-Null
Ask-NexusAI "How many ANPR sightings do we have?" | Out-Null
Ask-NexusAI "How many access log entries are there?" | Out-Null
Ask-NexusAI "Show call type breakdown for 923461678183" | Out-Null

# ============================================================
# PART 3 — media-type queries (video/image/audio/document counts).
# NOTE: these exercise a fix made today that is still being deployed
# as of this writing. Run Part 2 first; if these still return an
# unrelated/wrong count instead of the real number, the deploy hasn't
# finished yet — re-run this section after confirming the container
# was rebuilt (ask for that confirmation before presenting this part).
# Best run against the multimodal workspace, which has real media files.
# ============================================================

$headers["X-Forensic-Collection-ID"] = "nexusai-multimodal-product-acceptance"
function Ask-NexusAI-Multimodal($question) {
  $body = @{ query = $question; collection_id = "nexusai-multimodal-product-acceptance"; limit = 20; max_kb_results = 8 } | ConvertTo-Json
  $start = Get-Date
  $r = Invoke-RestMethod -Uri "http://localhost:8091/query/hybrid" -Method Post -ContentType "application/json" -Headers $headers -Body $body
  Write-Host "`n> $question" -ForegroundColor Cyan
  Write-Host "  ($(((Get-Date) - $start).TotalSeconds) s)" -ForegroundColor DarkGray
  Write-Host $r.answer.records_summary -ForegroundColor Green
  foreach ($f in @("image_metadata","audio_metadata","video_metadata","document_metadata")) {
    if ($r.records.$f) { $r.records.$f | Format-Table -AutoSize }
  }
  return $r
}

Ask-NexusAI-Multimodal "How many video files do we have in this case?" | Out-Null
Ask-NexusAI-Multimodal "How many image files do we have in this case?" | Out-Null
Ask-NexusAI-Multimodal "How many audio files do we have in this case?" | Out-Null

# reset back to the main demo collection for anything after this
$headers["X-Forensic-Collection-ID"] = "nexusai-forensic-demo"
