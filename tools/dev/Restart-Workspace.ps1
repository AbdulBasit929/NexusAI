<#
.SYNOPSIS
  Restarts the workspace dev server (Vite) on a port, from the lane tree or the main checkout, with the API key read from the env file. Needed after every API recreate.

.DESCRIPTION
  After the API container is recreated, a running dev server keeps its connection to the OLD container and every request through its /api proxy hangs until it is restarted
  (seen 2026-10-19: a live check waited 15 minutes). This script stops whatever listens on the port, starts `vite` again in the chosen tree and checks that the proxy reaches the API.

  The key is read from `.env.forensic-runtime.local` (git-ignored, in the MAIN checkout's root) and put in the environment of the new process only; it is never printed or written.

.EXAMPLE
  .\tools\dev\Restart-Workspace.ps1 -Port 4190 -Tree lane          # the lane tree (NexusAI-localwork), the committed workspace
  .\tools\dev\Restart-Workspace.ps1 -Port 4181 -Tree main          # the main checkout (what http://localhost:4181 has served so far)
#>
param(
  [Parameter(Mandatory = $true)][int]$Port,
  [ValidateSet('lane', 'main')][string]$Tree = 'lane',
  [string]$Collection = 'ingest-v2-shadow-synthetic-20261013'
)
$ErrorActionPreference = 'Stop'
$lane = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)          # .../NexusAI-localwork
$main = Join-Path (Split-Path -Parent $lane) 'NexusAI'
$root = if ($Tree -eq 'main') { $main } else { $lane }
$workspace = Join-Path $root 'apps\investigation-workspace'
if (-not (Test-Path (Join-Path $workspace 'node_modules\vite\bin\vite.js'))) { throw "no vite in $workspace (run npm install there)" }

$envFile = Join-Path $main '.env.forensic-runtime.local'
$line = Get-Content $envFile | Where-Object { $_ -match '^FORENSIC_RECORDS_API_KEY=' } | Select-Object -First 1
$key = ($line -replace '^FORENSIC_RECORDS_API_KEY=', '').Trim().Trim('"')
if (-not $key) { throw "FORENSIC_RECORDS_API_KEY is not in $envFile" }

$old = Get-NetTCPConnection -State Listen -LocalPort $Port -ErrorAction SilentlyContinue | Select-Object -First 1 -ExpandProperty OwningProcess
if ($old) { Stop-Process -Id $old -Force; Write-Host "stopped the listener on $Port (pid $old)"; Start-Sleep -Seconds 2 }

$out = Join-Path ([IO.Path]::GetTempPath()) "vite-$Port.out"
$err = Join-Path ([IO.Path]::GetTempPath()) "vite-$Port.err"
$env:FORENSIC_RECORDS_API_KEY = $key
# the API by its IPv4 address: 'localhost' can resolve to ::1 first and Docker's IPv6 relay can stall (seen 2026-10-31)
if (-not $env:NEXUSAI_API_URL) { $env:NEXUSAI_API_URL = 'http://127.0.0.1:8091' }
try {
  $p = Start-Process -FilePath 'node' -ArgumentList 'node_modules\vite\bin\vite.js', '--port', $Port, '--strictPort' -WorkingDirectory $workspace -WindowStyle Hidden -PassThru -RedirectStandardOutput $out -RedirectStandardError $err
} finally { Remove-Item Env:\FORENSIC_RECORDS_API_KEY -ErrorAction SilentlyContinue }
Write-Host "started the $Tree workspace on port $Port (pid $($p.Id)); log: $out"
Start-Sleep -Seconds 8

# the proxy must reach the API: a read of the connections endpoint with the five scope headers
$headers = @{ 'X-Forensic-Tenant-ID' = 'default'; 'X-Forensic-Collection-ID' = $Collection; 'X-Forensic-Actor-ID' = 'investigation-workspace'; 'X-Forensic-Subject-ID' = 'investigation-workspace'; 'X-Forensic-Actor-Role' = 'user' }
try {
  $r = Invoke-WebRequest -UseBasicParsing -Headers $headers -TimeoutSec 60 "http://127.0.0.1:$Port/api/analysis/connections?per_subject=3&tenant_id=default&collection_id=$Collection"
  Write-Host "the proxy reaches the API: HTTP $($r.StatusCode)"
} catch {
  $code = if ($_.Exception.Response) { $_.Exception.Response.StatusCode.value__ } else { 'no answer' }
  throw "the proxy did not reach the API (HTTP $code); see $err"
}
