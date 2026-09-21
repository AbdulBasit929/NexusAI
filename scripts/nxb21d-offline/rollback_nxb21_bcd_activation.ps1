# SPDX-License-Identifier: MIT
[CmdletBinding()]
param([Parameter(Mandatory=$true)][string]$RunDirectory)
. (Join-Path $PSScriptRoot 'common.ps1')
$root=[IO.Path]::GetFullPath($script:NxPrivateRoot);$run=[IO.Path]::GetFullPath($RunDirectory);if(-not$run.StartsWith($root,[StringComparison]::OrdinalIgnoreCase)){throw 'Rollback run directory is outside the private operator root.'}
$compose=Join-Path $run 'rollback-compose.json';if(-not(Test-Path $compose)){throw 'Rollback compose receipt is missing.'}
$cfg=Get-NxDConfig;$before=Get-NxDContainers;$beforeTuple=Get-NxDRetainedTuple $before;$beforeActivity=Get-NxDActivityCount $before
if((Get-NxDActiveJobs $before)-ne0){throw 'ROLLBACK_PREREQUISITE_FAILED:active jobs'}
if(Get-NxDLoadedState $cfg){throw 'ROLLBACK_PREREQUISITE_FAILED:Qwen is loaded'}
$null=Invoke-NxDNative docker @('compose','-f',$compose,'up','-d','--no-deps','api','forensic-records-api');Start-Sleep 15;$after=Get-NxDContainers
$expected=@{api=[string]$cfg.runtime.containers.api.image;'forensic-records-api'=[string]$cfg.runtime.containers.'forensic-records-api'.image}
Assert-NxDServiceImages $before $after $expected $false;Assert-NxDProtectedServices $before $after;Assert-NxDHTTPHealth $cfg
if((Get-NxDActiveJobs $after)-ne0-or(Get-NxDRetainedTuple $after)-cne$beforeTuple-or(Get-NxDActivityCount $after)-ne$beforeActivity){throw 'ROLLBACK_VERIFICATION_FAILED:retained state'}
if(Get-NxDLoadedState $cfg){throw 'ROLLBACK_VERIFICATION_FAILED:model state changed'}
Write-NxDJson (Join-Path $run 'rollback-receipt.json') ([ordered]@{state='ROLLED_BACK';completed_at=[DateTime]::UtcNow.ToString('o');services=@('api','forensic-records-api');protected_services_unchanged=$true;retained_tuple=$beforeTuple;activity_count_unchanged=$true;database_migration=$false;volumes_changed=$false;model_state_changed=$false})
Write-Host 'NXB21_BCD_ROLLBACK=PASS'
