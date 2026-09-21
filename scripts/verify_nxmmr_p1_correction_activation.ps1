# SPDX-License-Identifier: MIT
[CmdletBinding()]
param([string]$RunDirectory='',[switch]$Internal)
Set-StrictMode -Version Latest
$ErrorActionPreference='Stop'
$run=''
try {
    . (Join-Path $PSScriptRoot 'nxmmr_p1_correction_operator_common.ps1')
    $run=Get-NxP1RunDirectory $RunDirectory;$manifest=Get-NxP1Manifest
    $state=Get-Content -LiteralPath (Join-Path $run 'state.json') -Raw|ConvertFrom-Json
    if($state.state -notin @('STARTING_CANDIDATE_SERVICES','VERIFIED')){throw 'No admitted P1 candidate recreation exists to verify.'}
    if($state.manifest_sha256 -cne (Get-NxHash $script:NxSttManifestPath) -or $state.source_seal_sha256 -cne (Get-NxHash $script:NxSttIntegrityPath)){throw 'Source seal differs from the activation receipt.'}
    $before=Get-Content -LiteralPath (Join-Path $run 'container-before.json') -Raw|ConvertFrom-Json
    $private=Get-Content -LiteralPath (Join-Path $run 'private-container-snapshot.json') -Raw|ConvertFrom-Json
    $containers=Wait-NxHealth $manifest.resource_gates.health_timeout_seconds;Assert-NxSttProtected $before $containers
    foreach($name in $script:NxSttChanged){
        if($containers[$name].Image -cne [string]$state.expected_images.$name -or $containers[$name].RestartCount -ne 0){throw "Activated image/restart mismatch: $name"}
        Assert-NxP1RuntimeParity (Get-NxP1SnapshotContainer $private $name) $containers[$name] $name
    }
    Assert-NxP1WorkerRoles $containers['forensic-records-worker'] $manifest
    $null=Get-NxSttModelInventory $manifest
    Assert-NxP1RetainedState $containers ([string]$state.retained_counts_before)
    Start-Sleep -Seconds 10
    $stable=Get-NxSttContainers;Assert-NxHealth $stable;Assert-NxSttProtected $before $stable
    foreach($name in $script:NxSttChanged){if($stable[$name].Id -cne $containers[$name].Id -or $stable[$name].RestartCount -ne 0){throw "Activated service was unstable: $name"}}
    Assert-NxP1RetainedState $stable ([string]$state.retained_counts_before)
    Write-NxJson (Join-Path $run 'container-after.json') (Get-NxSummary $stable)
    Set-NxP1ReceiptOutcome $state 'VERIFIED';Write-NxJson (Join-Path $run 'state.json') $state
    Write-Host "P1_CORRECTION_ACTIVATION_VERIFICATION=PASS receipt=$run"
} catch {Write-Host "P1_CORRECTION_ACTIVATION_VERIFICATION=FAILED reason=$($_.Exception.Message)";if($run){Write-NxJson (Join-Path $run 'verification-failure.json') @{state='FAILED';reason=$_.Exception.Message;utc=[DateTime]::UtcNow.ToString('o')}};if($Internal){throw};exit 4}
