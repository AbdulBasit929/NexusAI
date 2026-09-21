# SPDX-License-Identifier: MIT
[CmdletBinding()]
param([switch]$Json)
Set-StrictMode -Version Latest
$ErrorActionPreference='Stop'
try {
    . (Join-Path $PSScriptRoot 'nxmmr_shared_query_operator_common.ps1')
    $receipt=Invoke-NxSttPreflight
    if($Json){$receipt|ConvertTo-Json -Depth 40}else{Show-NxSttPreflight $receipt}
    if($receipt.status -ne 'PASS'){exit 2}
    exit 0
} catch {Write-Host "SHARED_QUERY_ACTIVATION_PREFLIGHT=BLOCKED reason=$($_.Exception.Message)";exit 2}
