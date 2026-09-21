# SPDX-License-Identifier: MIT
[CmdletBinding()]
param([ValidateRange(1,120)][int]$WaitForRamMinutes=120)
Set-StrictMode -Version Latest
$ErrorActionPreference='Stop'
try {
    . (Join-Path $PSScriptRoot 'nxmmr_p1_correction_operator_common.ps1')
    $receipt=Invoke-NxP1Preflight;Show-NxP1Preflight $receipt
    $nonRamFailures=@($receipt.failures|Where-Object {$_.gate -ne 'ram_gib'})
    if($receipt.errors.Count -gt 0 -or $nonRamFailures.Count -gt 0){exit 2}
    if($receipt.status -ne 'PASS'){
        Write-Host "Only the RAM gate is pending. Waiting up to $WaitForRamMinutes minutes. Close Codex, ChatGPT, browsers, editors and optional apps; keep Docker Desktop and this terminal running."
        $manifest=Get-NxP1Manifest
        $null=Wait-NxSttRam $manifest $WaitForRamMinutes -RecoverBuildCache
        $receipt=Invoke-NxP1Preflight;Show-NxP1Preflight $receipt
        if($receipt.status -ne 'PASS'){exit 2}
    }
    Write-Host 'The worker, PostgreSQL and NATS are protected. Builds are sequential and source-sealed, with network access for approved Dockerfile dependencies only. No forced base refresh; runtime startup is no-build/no-pull. No model downloads or runtime package installation. A guarded two-service drain is allowed only after both candidates and rollback are complete.'
    $answer=Read-Host 'Activate ONLY api and forensic-records-api now? Type YES or NO'
    if($answer -cne 'YES'){Write-Host 'CANCELLED. No activation mutation.';exit 1}
    & powershell.exe -NoProfile -ExecutionPolicy Bypass -File (Join-Path $PSScriptRoot 'activate_nxmmr_p1_correction.ps1') -WaitForRamMinutes $WaitForRamMinutes -AllowGuardedDrain
    exit $LASTEXITCODE
} catch {Write-Host "P1_CORRECTION_ACTIVATION=FAILED $($_.Exception.Message)";exit 3}
