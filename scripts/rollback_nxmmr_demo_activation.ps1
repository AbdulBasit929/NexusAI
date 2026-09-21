# SPDX-License-Identifier: MIT
[CmdletBinding()]
param([string]$RunDirectory='', [switch]$Internal)
Set-StrictMode -Version Latest
$ErrorActionPreference='Stop'
$mutex=$null; $run=''
try {
    . (Join-Path $PSScriptRoot 'nxmmr_demo_operator_common.ps1')
    if(-not $Internal){$mutex=Enter-NxOperatorLock}
    $run=Get-NxRunDirectory $RunDirectory
    $receipt=Get-Content -LiteralPath (Join-Path $run 'rollback.json') -Raw | ConvertFrom-Json
    $file=Join-Path $run 'rollback-compose.json'
    if((Get-NxHash $file) -cne $receipt.compose_sha256){throw 'Rollback configuration integrity failure.'}
    $compose=Get-Content -LiteralPath $file -Raw | ConvertFrom-Json
    if(@($compose.services.PSObject.Properties).Count -ne 1 -or -not $compose.services.PSObject.Properties['forensic-records-worker']){throw 'Rollback must contain exactly the worker.'}
    if($compose.services.'forensic-records-worker'.image -cne $receipt.original_image){throw 'Rollback image mismatch.'}
    $null=Invoke-NxNative docker @('image','inspect',$receipt.original_image,'--format','{{.Id}}')
    $before=Get-Content -LiteralPath (Join-Path $run 'container-before.json') -Raw | ConvertFrom-Json
    $current=Get-NxContainers -AllowMissingWorker; Assert-NxProtected $before $current
    if($current.Contains('forensic-records-worker')) {
        if($current['forensic-records-worker'].Id -eq $receipt.original_container){Write-Host 'ROLLBACK=NOT_NEEDED original worker is unchanged.'; return}
        $state=Get-Content -LiteralPath (Join-Path $run 'state.json') -Raw | ConvertFrom-Json
        if($state.PSObject.Properties['expected_image'] -and $current['forensic-records-worker'].Image -cne $state.expected_image){throw 'Current worker is not this activation result; rollback refused.'}
    }
    if((Get-NxJobs $current) -ne 0){throw 'Active jobs prevent rollback; preserve logs and request assistance.'}
    $null=Invoke-NxReceiptCompose $file @('config','--quiet')
    $null=Invoke-NxReceiptCompose $file @('up','-d','--no-deps','--no-build','--pull','never','--force-recreate','forensic-records-worker') 180 (Join-Path $run 'rollback.log')
    $restored=Wait-NxHealth; Assert-NxProtected $before $restored
    $worker=$restored['forensic-records-worker']; if($worker.Image -cne $receipt.original_image){throw 'Rollback image verification failed.'}
    $envMap=@{}; foreach($entry in $worker.Config.Env){$p=$entry -split '=',2;$envMap[$p[0]]=$p[1]}
    foreach($p in $compose.services.'forensic-records-worker'.environment.PSObject.Properties){if($envMap[$p.Name] -cne $p.Value){throw "Rollback environment mismatch: $($p.Name)"}}
    if((Get-NxJobs $restored) -ne 0){throw 'Unexpected jobs after rollback.'}
    Write-NxJson (Join-Path $run 'rollback-verification.json') @{state='PASS';image=$worker.Image;container=$worker.Id;prior_configuration_restored=$true;models_removed=$false;protected_services_unchanged=$true}
    Write-NxJson (Join-Path $run 'state.json') @{state='ROLLED_BACK';recreation_started=$true}
    Write-Host "ROLLBACK=PASS. Exact prior image and role configuration restored. Receipt: $run"
    Write-Host 'Newly copied model files remain inactive; no evidence, database or volume was cleaned.'
} catch {
    Write-Host "ROLLBACK=FAILED reason=$($_.Exception.Message). Preserve logs; do not guess."
    if($run -and (Test-Path -LiteralPath $run)){Write-NxJson (Join-Path $run 'rollback-failure.json') @{state='FAILED';reason=$_.Exception.Message;utc=[DateTime]::UtcNow.ToString('o')}}
    if($Internal){throw}; exit 5
}
finally {if($mutex){$mutex.ReleaseMutex();$mutex.Dispose()}}
