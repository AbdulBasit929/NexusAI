# SPDX-License-Identifier: MIT
[CmdletBinding()]
param()
Set-StrictMode -Version Latest
$ErrorActionPreference='Stop'
try {
    . (Join-Path $PSScriptRoot 'nxmmr_demo_operator_common.ps1')
    $receipt=Invoke-NxPreflight; Show-NxPreflight $receipt
    if($receipt.status -ne 'PASS'){exit 2}
    $answer=Read-Host 'Activate ONLY the demo worker now? Type YES or NO'
    if($answer -cne 'YES'){Write-Host 'CANCELLED. No activation mutation.';exit 1}
    & powershell.exe -NoProfile -ExecutionPolicy Bypass -File (Join-Path $PSScriptRoot 'activate_nxmmr_demo.ps1')
    exit $LASTEXITCODE
} catch {Write-Host "ACTIVATION=FAILED $($_.Exception.Message)";exit 3}
