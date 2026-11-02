<#
STAGING API: the API of this checkout, built as a binary, run on THIS machine against the restored copy of the live database (rehearse-evidence.sh with KEEP=1) and the real local model, on
127.0.0.1:8191. It carries the live API's own settings (every FORENSIC_* switch is copied from the running container), with four differences: the database (the restored copy), the model address
(this machine's :8080, not host.docker.internal), the listen address, and no API key (the port is bound to 127.0.0.1 only). Nothing here touches the live database, the live API or its image.

  powershell -File tools\dev\live-window\run-staging-api.ps1 [-Stamp 20261031T103450Z]      starts it (leaves it running); the process id and the log are printed
  Stop it with:  Stop-Process -Id <pid>
Ask it with tools/showcase/ask_case.py --url http://127.0.0.1:8191 (no key is needed: the file's reader still sends the live key, which this API ignores).
#>
param([string]$Stamp = "20261031T103450Z", [string]$Binary = "C:\tmp\nexusai-g4\api-staging.exe")
$ErrorActionPreference = "Stop"
$repo = Split-Path (Split-Path (Split-Path $PSScriptRoot))          # the checkout: tools\dev\live-window -> its root
$projects = Split-Path $repo                                          # the main checkout sits beside it and holds the git-ignored backups
$backups = Join-Path (Join-Path $projects "NexusAI") ".phase2-backups"
$conn = Get-Content (Join-Path $backups ".rehearsal-evidence-$Stamp.conn")
$pw, $port = $conn -split " "
$liveEnv = docker inspect nexusai-forensic-records-api-1 --format "{{range .Config.Env}}{{println .}}{{end}}"
$settings = @{}
foreach ($line in $liveEnv) { if ($line -match "^([A-Z0-9_]+)=(.*)$") { $settings[$Matches[1]] = $Matches[2] } }
$work = "C:\tmp\nexusai-g4\staging-api"
New-Item -ItemType Directory -Force "$work\spool", "$work\plans" | Out-Null
$settings["FORENSIC_DATABASE_URL"] = "postgres://forensic_runtime:$pw@127.0.0.1:$port/localrecall?sslmode=disable"
$settings["LOCALAI_KB_URL"] = "http://127.0.0.1:8080"
$settings["FORENSIC_API_ADDR"] = "127.0.0.1:8191"
$settings["FORENSIC_API_KEY"] = ""
$settings["FORENSIC_API_AUTH_REQUIRED"] = "false"
$settings["FORENSIC_SPOOL_DIR"] = "$work\spool"
$settings["FORENSIC_PLAN_CACHE_DIR"] = "$work\plans"
$settings["FORENSIC_KB_MIRROR_ENABLED"] = "false"
# the API needs a message bus to start; it gets its own, never the live one (a stray consumer on the live bus could take an ingest job)
if (-not (docker ps -q -f "name=nx-staging-nats")) { docker run --rm -d --name nx-staging-nats -p 127.0.0.1:4223:4222 nats:2.11-alpine -js | Out-Null; Start-Sleep -Seconds 2 }
$settings["NATS_URL"] = "nats://127.0.0.1:4223"
foreach ($name in $settings.Keys) { if ($name -ne "PATH" -and $name -ne "SSL_CERT_FILE") { Set-Item -Path "Env:$name" -Value $settings[$name] } }
$proc = Start-Process -FilePath $Binary -PassThru -WindowStyle Hidden -RedirectStandardOutput "$work\out.log" -RedirectStandardError "$work\err.log"
Start-Sleep -Seconds 4
"staging API pid $($proc.Id), log $work\err.log"
try { (Invoke-WebRequest -Uri "http://127.0.0.1:8191/healthz" -UseBasicParsing -TimeoutSec 8).Content } catch { "healthz: " + $_.Exception.Message }
