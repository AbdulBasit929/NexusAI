# SPDX-License-Identifier: MIT
[CmdletBinding()]
param([string]$RunDirectory='',[switch]$Internal)
Set-StrictMode -Version Latest
$ErrorActionPreference='Stop'
$mutex=$null;$run=''
try {
    . (Join-Path $PSScriptRoot 'nxmmr_p1_correction_operator_common.ps1')
    if(-not $Internal){$mutex=Enter-NxP1OperatorLock}
    $run=Get-NxP1RunDirectory $RunDirectory;$manifest=Get-NxP1Manifest
    $state=Get-Content -LiteralPath (Join-Path $run 'state.json') -Raw|ConvertFrom-Json
    if(-not $state.mutation_started){throw 'This receipt has no live mutation to roll back.'}
    $file=Join-Path $run 'rollback-compose.json';if((Get-NxHash $file) -cne [string]$state.rollback_compose_sha256){throw 'Rollback compose integrity failure.'}
    $compose=Get-Content -LiteralPath $file -Raw|ConvertFrom-Json
    if((@($compose.services.PSObject.Properties.Name) -join ',') -cne ($script:NxSttChanged -join ',')){throw 'Rollback scope differs from the admitted two services.'}
    $before=Get-Content -LiteralPath (Join-Path $run 'container-before.json') -Raw|ConvertFrom-Json
    $private=Get-Content -LiteralPath (Join-Path $run 'private-container-snapshot.json') -Raw|ConvertFrom-Json
    $current=Get-NxSttContainers;Assert-NxSttProtected $before $current;Assert-NxP1RetainedState $current ([string]$state.retained_counts_before)
    foreach($name in $script:NxSttChanged){
        $original=[string]$before.$name.image;$candidate=[string]$state.expected_images.$name
        $null=Invoke-NxNative docker @('image','inspect',$original,'--format','{{.Id}}')
        if($current[$name].Image -cnotin @($original,$candidate)){throw "Unexpected current image prevents rollback: $name"}
    }
    $null=Invoke-NxReceiptCompose $file @('config','--quiet')
    $null=Invoke-NxReceiptCompose $file @('up','-d','--no-deps','--no-build','--pull','never','--force-recreate') 300 (Join-Path $run 'rollback.log') -StreamOutput
    $restored=Wait-NxHealth $manifest.resource_gates.health_timeout_seconds;Assert-NxSttProtected $before $restored
    foreach($name in $script:NxSttChanged){
        if($restored[$name].Image -cne [string]$before.$name.image){throw "Rollback image mismatch: $name"}
        Assert-NxP1RuntimeParity (Get-NxP1SnapshotContainer $private $name) $restored[$name] $name
    }
    Assert-NxP1WorkerRoles $restored['forensic-records-worker'] $manifest;Assert-NxP1RetainedState $restored ([string]$state.retained_counts_before)
    Write-NxJson (Join-Path $run 'rollback-verification.json') @{state='PASS';services=$script:NxSttChanged;protected_services_unchanged=$true;database_migration=$false;volumes_changed=$false;retained_state_mutated=$false}
    Set-NxP1ReceiptOutcome $state 'ROLLED_BACK';Write-NxJson (Join-Path $run 'state.json') $state
    Write-Host "P1_CORRECTION_ROLLBACK=PASS receipt=$run"
} catch {Write-Host "P1_CORRECTION_ROLLBACK=FAILED reason=$($_.Exception.Message)";if($Internal){throw};exit 5} finally {if($mutex){$mutex.ReleaseMutex();$mutex.Dispose()}}
