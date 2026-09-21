# SPDX-License-Identifier: MIT
[CmdletBinding()]
param([string]$RunDirectory='',[switch]$Internal)
Set-StrictMode -Version Latest
$ErrorActionPreference='Stop'
$run=''
try {
    . (Join-Path $PSScriptRoot 'nxmmr_stt_query_operator_common.ps1')
    $run=Get-NxSttRunDirectory $RunDirectory;$manifest=Get-NxSttManifest
    $state=Get-Content -LiteralPath (Join-Path $run 'state.json') -Raw|ConvertFrom-Json
    if($state.state -notin @('RECREATION_STARTED','VERIFIED')){throw 'No admitted three-service recreation exists to verify.'}
    if($state.manifest_sha256 -cne (Get-NxHash $script:NxSttManifestPath)){throw 'Manifest differs from the activation receipt.'}
    $before=Get-Content -LiteralPath (Join-Path $run 'container-before.json') -Raw|ConvertFrom-Json
    $containers=Wait-NxHealth $manifest.resource_gates.health_timeout_seconds;Assert-NxSttProtected $before $containers
    foreach($name in $script:NxSttChanged){if($containers[$name].Image -cne [string]$state.expected_images.$name -or $containers[$name].RestartCount -ne 0){throw "Activated service image/restart mismatch: $name"}}
    $worker=$containers['forensic-records-worker'];$envMap=@{};foreach($entry in $worker.Config.Env){$pair=$entry -split '=',2;$envMap[$pair[0]]=$pair[1]}
    foreach($property in $manifest.worker_environment.PSObject.Properties){if($envMap[$property.Name] -cne [string]$property.Value){throw "Worker role mismatch: $($property.Name)"}}
    $null=Get-NxSttAssets $manifest -DestinationRequired;$null=Get-NxSttModelInventory $manifest
    if((Get-NxJobs $containers) -ne 0){throw 'Unexpected active jobs after activation.'}
    Start-Sleep -Seconds 10
    $stable=Get-NxSttContainers;Assert-NxHealth $stable;Assert-NxSttProtected $before $stable
    foreach($name in $script:NxSttChanged){if($stable[$name].Id -cne $containers[$name].Id -or $stable[$name].RestartCount -ne 0){throw "Activated service was unstable: $name"}}
    Write-NxJson (Join-Path $run 'container-after.json') (Get-NxSummary $stable)
    Write-NxJson (Join-Path $run 'state.json') @{state='VERIFIED';recreation_started=$true;expected_images=$state.expected_images;manifest_sha256=$state.manifest_sha256;live_acceptance='PENDING'}
    Write-Host "STT_QUERY_ACTIVATION_VERIFICATION=PASS receipt=$run"
} catch {Write-Host "STT_QUERY_ACTIVATION_VERIFICATION=FAILED reason=$($_.Exception.Message)";if($run){Write-NxJson (Join-Path $run 'verification-failure.json') @{state='FAILED';reason=$_.Exception.Message;utc=[DateTime]::UtcNow.ToString('o')}};if($Internal){throw};exit 4}
