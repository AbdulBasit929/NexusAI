# Warm the model before a demonstration (PowerShell). A model that has been idle is reloaded on the first request, which takes about a minute and can make the first question fail.
# Run it once, wait for "ready", then ask one real question in the workspace. Nothing here changes any data; it sends one tiny completion and one cheap read.
#   .\docs\ux\v2\demo-warmup.ps1
param([string]$Model = 'qwen3-4b-instruct-2507-q4km-nxb21d-dev', [string]$Url = 'http://127.0.0.1:8080/v1/chat/completions')
$body = @{ model = $Model; messages = @(@{ role = 'user'; content = 'Reply with the single word: ready' }); max_tokens = 4; temperature = 0 } | ConvertTo-Json -Depth 4
$clock = [Diagnostics.Stopwatch]::StartNew()
try {
  $answer = Invoke-RestMethod -Uri $Url -Method Post -ContentType 'application/json' -Body $body -TimeoutSec 240
  $clock.Stop()
  "model: $($answer.choices[0].message.content.Trim()) in $([math]::Round($clock.Elapsed.TotalSeconds, 1)) s"
  if ($clock.Elapsed.TotalSeconds -gt 8) { 'It was cold. Run this again; the second answer should take under a second.' } else { 'Warm. Ask one real question now so the engine path is warm too.' }
} catch { "The model did not answer: $($_.Exception.Message). Check that the model container is running (docker ps)." }
Get-CimInstance Win32_Processor | Measure-Object -Property LoadPercentage -Average | ForEach-Object { "CPU load: $($_.Average) %" }
$os = Get-CimInstance Win32_OperatingSystem
'Free memory: {0:N1} GB' -f ($os.FreePhysicalMemory / 1MB)
if (Test-Path 'C:\tmp\nexusai-locks\heavy.lock') { 'A heavy job holds the lock: ' + (Get-Content 'C:\tmp\nexusai-locks\heavy.lock') + '. Do not demonstrate until it is released.' }
