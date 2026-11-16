<#
.SYNOPSIS
  Starts (or restarts) the NEW interface on http://localhost:4182 and leaves it running after you close this window or quit the Claude app.

.DESCRIPTION
  The new design lives in the interface worktree  C:\Users\sheik\Workspace\Office\Projects\NexusAI-ui  (branch ui/v2-redesign, app in apps\investigation-workspace) and is served by Vite on port 4182.
  Port 4181 is the OLD design and is not touched.

  Why it was "down": Vite is an ordinary process. A process started from inside the Claude app is stopped when the app or the session is restarted (the same happens to Docker started from there).
  This script starts Vite through WMI (Win32_Process.Create), so it belongs to no window, no terminal and no app session, and keeps running until you stop it or restart Windows.

  What it needs (it checks each one and tells you what to do):
    * Docker Desktop running with the stack up (the API answers on http://127.0.0.1:8091/healthz). Vite only proxies /api to it.
    * Node on the PATH and the packages installed in apps\investigation-workspace (node_modules\vite).
    * The API key file  C:\Users\sheik\Workspace\Office\Projects\NexusAI\.env.forensic-runtime.local  (read here, put in the environment of the Vite process only; never printed or written anywhere).

  Run it AGAIN after every time the API container is recreated: the dev server keeps its connection to the old container and its /api calls hang until it is restarted.

.EXAMPLE
  .\Start-UI-4182.ps1                 start or restart on 4182 (Orion case)
  .\Start-UI-4182.ps1 -Status         what is running (Docker, API, model, 4182, 4181)
  .\Start-UI-4182.ps1 -Stop           stop the server on 4182
  .\Start-UI-4182.ps1 -Port 4190      another port (for a second copy)
#>
param(
  [int]$Port = 4182,
  [string]$Collection = 'orion-lahore-reference-20261030',
  [string]$ApiUrl = 'http://127.0.0.1:8091',
  [switch]$Stop,
  [switch]$Status,
  [switch]$Detached       # internal: the second step, run by WMI outside any window or app
)
$ErrorActionPreference = 'Stop'
$here = Split-Path -Parent $MyInvocation.MyCommand.Path
$lane = Split-Path -Parent (Split-Path -Parent $here)            # ...\NexusAI-localwork
$projects = Split-Path -Parent $lane                              # ...\Projects
$mainCheckout = Join-Path $projects 'NexusAI'
$ui = Join-Path $projects 'NexusAI-ui\apps\investigation-workspace'
$vite = Join-Path $ui 'node_modules\vite\bin\vite.js'
$envFile = Join-Path $mainCheckout '.env.forensic-runtime.local'
$out = Join-Path ([IO.Path]::GetTempPath()) "vite-$Port.out"
$err = Join-Path ([IO.Path]::GetTempPath()) "vite-$Port.err"

function Get-Listener([int]$p) { Get-NetTCPConnection -State Listen -LocalPort $p -ErrorAction SilentlyContinue | Select-Object -First 1 }
function Test-Url([string]$url, [int]$seconds = 8) { try { (Invoke-WebRequest -UseBasicParsing -Uri $url -TimeoutSec $seconds).StatusCode } catch { 0 } }
function Stop-Port([int]$p) {
  $l = Get-Listener $p
  if ($l) { Stop-Process -Id $l.OwningProcess -Force -ErrorAction SilentlyContinue; Write-Host "stopped the process on port $p (pid $($l.OwningProcess))"; Start-Sleep -Seconds 2 }
}

if ($Status) {
  $docker = if (Get-Command docker -ErrorAction SilentlyContinue) { (& docker info --format '{{.ServerVersion}}' 2>$null) } else { $null }
  Write-Host ("Docker engine : {0}" -f $(if ($docker) { "running ($docker)" } else { 'NOT running (start Docker Desktop, then the containers)' }))
  Write-Host ("API   :8091   : {0}" -f $(if ((Test-Url "$ApiUrl/healthz") -eq 200) { 'answers' } else { 'does not answer' }))
  Write-Host ("Model :8080   : {0}" -f $(if ((Test-Url 'http://127.0.0.1:8080/readyz' 5) -eq 200) { 'ready' } else { 'not ready (the first question can fail until it has loaded)' }))
  foreach ($p in 4182, 4181) {
    $l = Get-Listener $p
    Write-Host ("UI    :{0}   : {1}" -f $p, $(if ($l) { "listening (pid $($l.OwningProcess)), page answers: $(Test-Url "http://127.0.0.1:$p/" 6)" } else { 'nothing listens' }))
  }
  return
}
if ($Stop) { Stop-Port $Port; if (-not (Get-Listener $Port)) { Write-Host "port $Port is free" }; return }

if ($Detached) {
  # step 2 (outside any window or app): stop the old server, start Vite with the key in ITS environment only, and finish; Vite goes on running
  $line = Get-Content $envFile | Where-Object { $_ -match '^FORENSIC_RECORDS_API_KEY=' } | Select-Object -First 1
  $key = ($line -replace '^FORENSIC_RECORDS_API_KEY=', '').Trim().Trim('"')
  if (-not $key) { throw "FORENSIC_RECORDS_API_KEY is not in $envFile" }
  Stop-Port $Port
  $env:FORENSIC_RECORDS_API_KEY = $key
  $env:NEXUSAI_API_URL = $ApiUrl       # by IPv4 address: 'localhost' can resolve to ::1 first and Docker's IPv6 relay can stall
  try {
    Start-Process -FilePath 'node' -ArgumentList 'node_modules\vite\bin\vite.js', '--port', $Port, '--strictPort' -WorkingDirectory $ui -WindowStyle Hidden -RedirectStandardOutput $out -RedirectStandardError $err | Out-Null
  } finally { Remove-Item Env:\FORENSIC_RECORDS_API_KEY -ErrorAction SilentlyContinue }
  return
}

# ---- step 1: check everything, then hand over to step 2
Write-Host "== NexusAI interface on port $Port"
$problems = @()
if (-not (Get-Command node -ErrorAction SilentlyContinue)) { $problems += 'Node is not on the PATH (install Node 20 or newer).' }
if (-not (Test-Path -LiteralPath $vite)) { $problems += "Vite is missing: run  npm install  in  $ui" }
if (-not (Test-Path -LiteralPath $envFile)) { $problems += "The key file is missing: $envFile" }
if ((Test-Url "$ApiUrl/healthz") -ne 200) { $problems += "The API does not answer on $ApiUrl. Start Docker Desktop and the stack first (the page would load but every request would hang)." }
if ($problems.Count) { $problems | ForEach-Object { Write-Host "  PROBLEM: $_" -ForegroundColor Red }; throw 'not started: fix the problem above and run this script again' }
Write-Host "  API answers on $ApiUrl; interface folder: $ui"

$command = 'powershell.exe -NoProfile -ExecutionPolicy Bypass -File "{0}" -Port {1} -Collection {2} -ApiUrl {3} -Detached' -f $MyInvocation.MyCommand.Path, $Port, $Collection, $ApiUrl
$started = $false
try {
  $r = Invoke-CimMethod -ClassName Win32_Process -MethodName Create -Arguments @{ CommandLine = $command }
  $started = ($r.ReturnValue -eq 0)
} catch { $started = $false }
if (-not $started) {
  Write-Host '  WMI start was not possible: starting it from this window instead (it will stop if this window or the app that owns it is closed).' -ForegroundColor Yellow
  & $MyInvocation.MyCommand.Path -Port $Port -Collection $Collection -ApiUrl $ApiUrl -Detached
}

$ok = $false
for ($i = 0; $i -lt 45; $i++) { if ((Test-Url "http://127.0.0.1:$Port/" 4) -eq 200) { $ok = $true; break }; Start-Sleep -Seconds 1 }
if (-not $ok) { Write-Host "  The server did not answer within 45 seconds. Log: $err" -ForegroundColor Red; Get-Content $err -Tail 15 -ErrorAction SilentlyContinue; throw 'not started' }
$proxy = Test-Url "http://127.0.0.1:$Port/api/healthz" 15
$l = Get-Listener $Port
Write-Host ("  page answers (200), the /api proxy reaches the API: {0}, pid {1}" -f $(if ($proxy -eq 200) { 'yes' } else { "NO (HTTP $proxy): see $err" }), $l.OwningProcess)
Write-Host "  open:  http://localhost:$Port/"
Write-Host "         http://localhost:$Port/cases/$Collection/evidence"
Write-Host "  logs:  $out  and  $err"
Write-Host "  stop:  .\Start-UI-4182.ps1 -Stop      status:  .\Start-UI-4182.ps1 -Status"
