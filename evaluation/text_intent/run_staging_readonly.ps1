<#
STAGING API FOR THE TEXT-INTENT MEASUREMENT: the API of this checkout, built as a binary, run on THIS machine on 127.0.0.1:8191 with FORENSIC_TEXT_INTENT=true, against the LIVE database through a
READ-ONLY session (default_transaction_read_only=on: any write the code attempts fails instead of landing) and the real local model on :8080. It carries the live API's own switches (copied from the
running container) with these differences: the database address (this machine's mapped port) and the read-only session, the model address, the listen address, no API key (the port is bound to
127.0.0.1 only), its own message bus, and the switch under test. The live API, its image and its data are not touched.

  powershell -File evaluation\text_intent\run_staging_readonly.ps1 [-Binary C:\tmp\nexusai-g4\api-staging.exe] [-TextIntent true]     starts it; prints the process id and the log
  Stop it with:  Stop-Process -Id <pid>
Build the binary first:  go build -p 2 -o C:\tmp\nexusai-g4\api-staging.exe ./api/forensic_records   (from the checkout root)
#>
param([string]$Binary = "C:\tmp\nexusai-g4\api-staging.exe", [string]$TextIntent = "true")
$ErrorActionPreference = "Stop"
$liveEnv = docker inspect nexusai-forensic-records-api-1 --format "{{range .Config.Env}}{{println .}}{{end}}"
$settings = @{}
foreach ($line in $liveEnv) { if ($line -match "^([A-Z0-9_]+)=(.*)$") { $settings[$Matches[1]] = $Matches[2] } }
$work = "C:\tmp\nexusai-g4\staging-api-ro"
New-Item -ItemType Directory -Force "$work\spool", "$work\plans" | Out-Null
# the live URL names the database container; the same server is reachable on this machine at the mapped port 5433
$url = $settings["FORENSIC_DATABASE_URL"] -replace "@[^/@]+/", "@127.0.0.1:5433/"
if ($url -notmatch "\?") { $url += "?" } else { $url += "&" }
$settings["FORENSIC_DATABASE_URL"] = $url + "default_transaction_read_only=on"
$settings["LOCALAI_KB_URL"] = "http://127.0.0.1:8080"
$settings["FORENSIC_API_ADDR"] = "127.0.0.1:8191"
$settings["FORENSIC_API_KEY"] = ""
$settings["FORENSIC_API_AUTH_REQUIRED"] = "false"
$settings["FORENSIC_SPOOL_DIR"] = "$work\spool"
$settings["FORENSIC_PLAN_CACHE_DIR"] = "$work\plans"
$settings["FORENSIC_KB_MIRROR_ENABLED"] = "false"
$settings["FORENSIC_TEXT_INTENT"] = $TextIntent
if (-not (docker ps -q -f "name=nx-staging-nats")) { docker run --rm -d --name nx-staging-nats -p 127.0.0.1:4223:4222 nats:2.11-alpine -js | Out-Null; Start-Sleep -Seconds 2 }
$settings["NATS_URL"] = "nats://127.0.0.1:4223"
foreach ($name in $settings.Keys) { if ($name -ne "PATH" -and $name -ne "SSL_CERT_FILE") { Set-Item -Path "Env:$name" -Value $settings[$name] } }
$proc = Start-Process -FilePath $Binary -PassThru -WindowStyle Hidden -RedirectStandardOutput "$work\out.log" -RedirectStandardError "$work\err.log"
Start-Sleep -Seconds 4
"staging API pid $($proc.Id), log $work\err.log"
try { (Invoke-WebRequest -Uri "http://127.0.0.1:8191/healthz" -UseBasicParsing -TimeoutSec 8).Content } catch { "healthz: " + $_.Exception.Message }
