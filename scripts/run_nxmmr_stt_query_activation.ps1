# SPDX-License-Identifier: MIT
[CmdletBinding()]
param()
Set-StrictMode -Version Latest
$ErrorActionPreference='Stop'
try {
    . (Join-Path $PSScriptRoot 'nxmmr_stt_query_operator_common.ps1')
    $receipt=Invoke-NxSttPreflight;Show-NxSttPreflight $receipt
    if($receipt.status -ne 'PASS'){exit 2}
    Write-Host 'RAM policy: the 6 GiB floor remains mandatory. After each build, the operator waits up to 30 minutes, may release verified-clean Docker Linux page cache, and seals completed images for retry.'
    $answer=Read-Host 'Activate ONLY api, forensic-records-api, and forensic-records-worker now? Type YES or NO'
    if($answer -cne 'YES'){Write-Host 'CANCELLED. No activation mutation.';exit 1}
    & powershell.exe -NoProfile -ExecutionPolicy Bypass -File (Join-Path $PSScriptRoot 'activate_nxmmr_stt_query.ps1')
    exit $LASTEXITCODE
} catch {Write-Host "STT_QUERY_ACTIVATION=FAILED $($_.Exception.Message)";exit 3}
