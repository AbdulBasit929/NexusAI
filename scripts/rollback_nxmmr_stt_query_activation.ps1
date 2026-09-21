# SPDX-License-Identifier: MIT
[CmdletBinding()]
param([string]$RunDirectory='',[switch]$Internal)
Set-StrictMode -Version Latest
$ErrorActionPreference='Stop'
$mutex=$null;$run=''
try {
    . (Join-Path $PSScriptRoot 'nxmmr_stt_query_operator_common.ps1')
    if(-not $Internal){$mutex=Enter-NxSttOperatorLock}
    $run=Get-NxSttRunDirectory $RunDirectory;$receipt=Get-Content -LiteralPath (Join-Path $run 'rollback.json') -Raw|ConvertFrom-Json
    $file=Join-Path $run 'rollback-compose.json';if((Get-NxHash $file) -cne $receipt.compose_sha256){throw 'Rollback compose integrity failure.'}
    $compose=Get-Content -LiteralPath $file -Raw|ConvertFrom-Json
    if((@($compose.services.PSObject.Properties.Name) -join ',') -cne ($script:NxSttChanged -join ',')){throw 'Rollback service scope differs from the admitted three services.'}
    foreach($name in $script:NxSttChanged){$null=Invoke-NxNative docker @('image','inspect',[string]$receipt.original_images.$name,'--format','{{.Id}}')}
    $before=Get-Content -LiteralPath (Join-Path $run 'container-before.json') -Raw|ConvertFrom-Json;$current=Get-NxSttContainers;Assert-NxSttProtected $before $current
    if((Get-NxJobs $current) -ne 0){throw 'Active jobs prevent rollback.'}
    $null=Invoke-NxReceiptCompose $file @('config','--quiet')
    $null=Invoke-NxReceiptCompose $file @('up','-d','--no-deps','--no-build','--pull','never','--force-recreate') 300 (Join-Path $run 'rollback.log')
    $restored=Wait-NxHealth;Assert-NxSttProtected $before $restored
    foreach($name in $script:NxSttChanged){if($restored[$name].Image -cne [string]$receipt.original_images.$name){throw "Rollback image mismatch: $name"}}
    if((Get-NxJobs $restored) -ne 0){throw 'Unexpected jobs after rollback.'}
    Write-NxJson (Join-Path $run 'rollback-verification.json') @{state='PASS';services=$script:NxSttChanged;protected_services_unchanged=$true;volumes_removed=$false;evidence_removed=$false}
    Write-NxJson (Join-Path $run 'state.json') @{state='ROLLED_BACK';recreation_started=$true}
    Write-Host "STT_QUERY_ROLLBACK=PASS receipt=$run"
} catch {Write-Host "STT_QUERY_ROLLBACK=FAILED reason=$($_.Exception.Message)";if($Internal){throw};exit 5} finally {if($mutex){$mutex.ReleaseMutex();$mutex.Dispose()}}
