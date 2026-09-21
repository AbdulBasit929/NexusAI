# SPDX-License-Identifier: MIT
[CmdletBinding()]
param(
    [ValidateRange(1,120)][int]$WaitForRamMinutes=120
)
Set-StrictMode -Version Latest
$ErrorActionPreference='Stop'
try {
    . (Join-Path $PSScriptRoot 'nxmmr_shared_query_operator_common.ps1')
    $receipt=Invoke-NxSttPreflight;Show-NxSttPreflight $receipt
    $nonRamFailures=@($receipt.failures|Where-Object {$_.gate -ne 'ram_gib'})
    if($receipt.errors.Count -gt 0 -or $nonRamFailures.Count -gt 0){exit 2}
    if($receipt.status -ne 'PASS'){
        Write-Host "Only the RAM gate is pending. Waiting up to $WaitForRamMinutes minutes; close unnecessary apps while Docker and this terminal remain open."
        $manifest=Get-NxSttManifest
        $null=Wait-NxSttRam $manifest $WaitForRamMinutes -RecoverBuildCache:$true
        $receipt=Invoke-NxSttPreflight;Show-NxSttPreflight $receipt
        if($receipt.status -ne 'PASS'){exit 2}
    }
    Write-Host 'RAM policy: the 6 GiB floor is mandatory. Builds are sequential, resumable, and may release only verified-clean Docker Linux page cache.'
    $answer=Read-Host 'Activate ONLY api, forensic-records-api, and forensic-records-worker now? Type YES or NO'
    if($answer -cne 'YES'){Write-Host 'CANCELLED. No activation mutation.';exit 1}
    & powershell.exe -NoProfile -ExecutionPolicy Bypass -File (Join-Path $PSScriptRoot 'activate_nxmmr_shared_query.ps1') -WaitForRamMinutes $WaitForRamMinutes
    exit $LASTEXITCODE
} catch {Write-Host "SHARED_QUERY_ACTIVATION=FAILED $($_.Exception.Message)";exit 3}
