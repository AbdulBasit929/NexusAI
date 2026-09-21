# SPDX-License-Identifier: MIT
[CmdletBinding()]
param(
    [Parameter(Mandatory=$true)][ValidateSet('PRELOAD_RAM','MODEL_LOADED_RAM_INITIAL','MODEL_LOADED_RAM_SETTLED','PRECALL_RAM','POSTCALL_RAM')][string]$Stage,
    [Parameter(Mandatory=$true)][int]$CompletedCalls,
    [Parameter(Mandatory=$true)][string]$GovernedRamScript
)
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

function Write-ResourceSnapshot([string]$Label) {
    $os = Get-CimInstance Win32_OperatingSystem
    $available = [math]::Round(([int64]$os.FreePhysicalMemory * 1024) / 1GB, 3)
    $vmmem = [math]::Round(((@(Get-Process -Name 'vmmemWSL' -ErrorAction SilentlyContinue) | Measure-Object WorkingSet64 -Sum).Sum / 1MB), 1)
    $containerMemory = (& docker stats --no-stream --format '{{.MemUsage}}' nexusai-api-1 2>$null | Select-Object -First 1)
    $backendRSS = 0.0
    try {
        $rows = @(& docker top nexusai-api-1 -eo rss,args 2>$null)
        foreach ($row in $rows) {
            if ($row -match 'qwen3-8b-q4km-nxb21d-selection-lowmem-dev' -and $row -match '^\s*([0-9]+)\s+') {
                $backendRSS += [double]$Matches[1] / 1024
            }
        }
    } catch { $backendRSS = 0.0 }
    Write-Host "$Label available_gib=$available backend_rss_mib=$([math]::Round($backendRSS,1)) vmmemwsl_working_set_mib=$vmmem api_container_memory=$containerMemory"
}

Write-ResourceSnapshot $Stage
if ($Stage -in @('MODEL_LOADED_RAM_INITIAL','POSTCALL_RAM')) {
    Write-Host "$Stage measurement_only=true"
    exit 0
}
$governedCompletedCalls = if ($Stage -eq 'PRELOAD_RAM') { 0 } else { [math]::Max(1, $CompletedCalls) }
$output = & powershell -NoProfile -ExecutionPolicy Bypass -File $GovernedRamScript -CompletedCalls $governedCompletedCalls 2>&1
$code = $LASTEXITCODE
foreach ($line in @($output)) { Write-Host "$Stage $line" }
Write-ResourceSnapshot ($Stage + '_FINAL')
exit $code
