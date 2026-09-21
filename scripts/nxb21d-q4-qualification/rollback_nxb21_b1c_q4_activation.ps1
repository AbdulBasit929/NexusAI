# SPDX-License-Identifier: MIT
[CmdletBinding()]
param([Parameter(Mandatory=$true)][string]$RunDirectory)
Set-StrictMode -Version Latest
$ErrorActionPreference='Stop'
$root=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
. (Join-Path $root 'scripts\nxb21d-offline\common.ps1')
$privateRoot=[IO.Path]::GetFullPath($script:NxPrivateRoot)
$run=[IO.Path]::GetFullPath($RunDirectory)
if(-not$run.StartsWith($privateRoot,[StringComparison]::OrdinalIgnoreCase)){throw 'ROLLBACK_PREREQUISITE_FAILED:run directory outside private operator root'}
$compose=Join-Path $run 'rollback-compose.json'
if(-not(Test-Path -LiteralPath $compose -PathType Leaf)){throw 'ROLLBACK_PREREQUISITE_FAILED:rollback compose receipt missing'}
$cfg=Get-Content -LiteralPath (Join-Path $PSScriptRoot 'nxb21d_q4_qualification_config.json') -Raw|ConvertFrom-Json
$before=Get-NxDContainers;$beforeTuple=Get-NxDRetainedTuple $before;$beforeActivity=Get-NxDActivityCount $before
if((Get-NxDActiveJobs $before)-ne0){throw 'ROLLBACK_PREREQUISITE_FAILED:active_jobs'}
if(Get-NxDLoadedState $cfg){throw 'ROLLBACK_PREREQUISITE_FAILED:Q4_model_loaded'}
$null=Invoke-NxDNative docker @('compose','-f',$compose,'config','--quiet')
$null=Invoke-NxDNative docker @('compose','-f',$compose,'up','-d','--no-deps','api','forensic-records-api')
$deadline=[DateTime]::UtcNow.AddSeconds(180);$healthy=$false
do{try{Assert-NxDHTTPHealth $cfg;$healthy=$true;break}catch{Write-Host "ROLLBACK_HEALTH_WAIT error=$($_.Exception.Message)"};Start-Sleep 5}while([DateTime]::UtcNow-lt$deadline)
if(-not$healthy){throw 'ROLLBACK_VERIFICATION_FAILED:http_health_timeout'}
$after=Get-NxDContainers
$expected=@{api=[string]$cfg.runtime.containers.api.image;'forensic-records-api'=[string]$cfg.runtime.containers.'forensic-records-api'.image}
Assert-NxDServiceImages $before $after $expected $false
Assert-NxDProtectedServices $before $after
if((Get-NxDActiveJobs $after)-ne0-or(Get-NxDRetainedTuple $after)-cne$beforeTuple-or(Get-NxDActivityCount $after)-ne$beforeActivity){throw 'ROLLBACK_VERIFICATION_FAILED:retained_state'}
if(Get-NxDLoadedState $cfg){throw 'ROLLBACK_VERIFICATION_FAILED:model_state_changed'}
Write-NxDJson (Join-Path $run 'rollback-receipt.json') ([ordered]@{schema_version='nexusai.nxb21-b1c-q4-rollback-receipt/v1';state='ROLLED_BACK';completed_at=[DateTime]::UtcNow.ToString('o');services=@('api','forensic-records-api');protected_services_unchanged=$true;retained_tuple=$beforeTuple;activity_count_unchanged=$true;database_migration=$false;volumes_changed=$false;model_state_changed=$false})
Write-Host 'NXB21_B1C_Q4_ROLLBACK=PASS'
