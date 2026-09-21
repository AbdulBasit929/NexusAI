# SPDX-License-Identifier: MIT
[CmdletBinding()]
param([switch]$Json)
Set-StrictMode -Version Latest
$ErrorActionPreference='Stop'
try {
    . (Join-Path $PSScriptRoot 'nxmmr_four_p1_operator_common.ps1')
    $receipt=Invoke-NxP1Preflight
    if($Json){$receipt|ConvertTo-Json -Depth 50}else{Show-NxP1Preflight $receipt}
    if($receipt.status -ne 'PASS'){exit 2}
    exit 0
} catch {Write-Host "P1_CORRECTION_ACTIVATION_PREFLIGHT=BLOCKED reason=$($_.Exception.Message)";exit 2}
