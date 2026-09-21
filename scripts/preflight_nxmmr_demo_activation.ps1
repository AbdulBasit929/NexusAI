# SPDX-License-Identifier: MIT
[CmdletBinding()]
param([switch]$Json)
Set-StrictMode -Version Latest
$ErrorActionPreference='Stop'
try {
    . (Join-Path $PSScriptRoot 'nxmmr_demo_operator_common.ps1')
    $receipt=Invoke-NxPreflight
    if($Json){$receipt | ConvertTo-Json -Depth 30}else{Show-NxPreflight $receipt}
    if($receipt.status -ne 'PASS'){exit 2}
    if(-not $Json){Write-Host 'Next: .\scripts\activate_nxmmr_demo.ps1'}
    exit 0
} catch {Write-Host "ACTIVATION_PREFLIGHT=BLOCKED reason=$($_.Exception.Message) measured=unverified required=verified"; exit 2}
