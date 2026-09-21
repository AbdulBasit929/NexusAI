# SPDX-License-Identifier: MIT
[CmdletBinding()]
param(
    [Parameter(Mandatory=$true)][ValidateSet('PRELOAD_RAM','MODEL_LOADED_RAM','PRECALL_RAM','POSTCALL_RAM')][string]$Stage,
    [Parameter(Mandatory=$true)][int]$CompletedCalls,
    [Parameter(Mandatory=$true)][string]$GovernedRamScript
)
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
if ($Stage -eq 'POSTCALL_RAM') {
    $os = Get-CimInstance Win32_OperatingSystem
    $available = [math]::Round(([int64]$os.FreePhysicalMemory * 1024) / 1GB, 3)
    Write-Host "$Stage available_gib=$available required_gib=4 measurement_only=true"
    exit 0
}
$governedCompletedCalls = if ($Stage -eq 'PRELOAD_RAM') { 0 } else { [math]::Max(1, $CompletedCalls) }
$output = & powershell -NoProfile -ExecutionPolicy Bypass -File $GovernedRamScript -CompletedCalls $governedCompletedCalls 2>&1
$code = $LASTEXITCODE
foreach ($line in @($output)) { Write-Host "$Stage $line" }
exit $code
