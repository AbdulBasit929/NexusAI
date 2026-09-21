[CmdletBinding()]
param([int]$CompletedCalls=0)
Set-StrictMode -Version Latest
$ErrorActionPreference='Stop'
. (Join-Path $PSScriptRoot '..\nxb21d-offline\common.ps1')
$floor=if($CompletedCalls -eq 0){6}else{4}
$cfg=[pscustomobject]@{gates=[pscustomobject]@{minimum_available_ram_gib=$floor}}
try {
    try {$null=Assert-NxDRam $cfg 'before_residual_inference'} catch {
        if($_.Exception.Message -notlike 'RAM_TOO_LOW*'){throw}
        $null=Invoke-NxDSafeLinuxCleanCache
        $null=Wait-NxDRamAfterCleanCache $cfg 'before_residual_inference' 180
    }
    exit 0
} catch {Write-Host $_.Exception.Message;exit 10}
