# SPDX-License-Identifier: MIT
[CmdletBinding()]
param([switch]$OwnerApproved)
Set-StrictMode -Version Latest
$ErrorActionPreference='Stop'
$root=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
. (Join-Path $root 'scripts\nxb21d-offline\common.ps1')
. (Join-Path $PSScriptRoot 'qualification_guards.ps1')

function Assert-Q4Equal([string]$Name,[string]$Actual,[string]$Expected){if($Actual-cne$Expected){throw "ACTIVATION_PREREQUISITE_FAILED:${Name} expected=$Expected actual=$Actual"}}
function Wait-Q4HTTPHealth($Config,[int]$TimeoutSeconds=180){$deadline=[DateTime]::UtcNow.AddSeconds($TimeoutSeconds);do{try{Assert-NxDHTTPHealth $Config;return}catch{Write-Host "ACTIVATION_HEALTH_WAIT error=$($_.Exception.Message)"};Start-Sleep 5}while([DateTime]::UtcNow-lt$deadline);throw 'ACTIVATION_VERIFICATION_FAILED:http_health_timeout'}
function Assert-Q4Framework($Config){foreach($property in $Config.framework_files.PSObject.Properties){Assert-Q4Equal "source_hash:$($property.Name)" (Get-NxDHash (Resolve-NxDPath $property.Name)) ([string]$property.Value)}}
function Assert-Q4Baseline($Config){
    $containers=Get-NxDContainers
    foreach($property in $Config.runtime.containers.PSObject.Properties){
        $actual=$containers[$property.Name];$expected=$property.Value
        if($null-eq$actual-or-not(Test-NxDContainerHealthy $actual)){throw "ACTIVATION_PREREQUISITE_FAILED:runtime:$($property.Name)"}
        Assert-Q4Equal "$($property.Name).id" ([string]$actual.Id) ([string]$expected.id)
        Assert-Q4Equal "$($property.Name).image" ([string]$actual.Image) ([string]$expected.image)
        if([int]$actual.RestartCount-ne[int]$expected.restarts){throw "ACTIVATION_PREREQUISITE_FAILED:$($property.Name).restarts"}
    }
    Assert-NxDHTTPHealth $Config
    if((Get-NxDActiveJobs $containers)-ne0){throw 'ACTIVATION_PREREQUISITE_FAILED:active_jobs'}
    $tuple=Get-NxDRetainedTuple $containers;Assert-Q4Equal 'retained_tuple' $tuple ([string]$Config.runtime.retained_tuple)
    $activity=Get-NxDActivityCount $containers
    if($activity-ne[int]$Config.runtime.activity_count){throw 'ACTIVATION_PREREQUISITE_FAILED:activity_count'}
    return [pscustomobject]@{containers=$containers;retained_tuple=$tuple;activity_count=$activity}
}
function Assert-Q4SourceManifest($Config){
    $path=Resolve-NxDPath $Config.activation.source_manifest;$sidecar=$path+'.sha256'
    if(-not(Test-Path -LiteralPath $path)-or-not(Test-Path -LiteralPath $sidecar)){throw 'ACTIVATION_PREREQUISITE_FAILED:source_manifest_missing'}
    Assert-Q4Equal 'source_manifest_sha256' (Get-NxDHash $path) ([string]$Config.activation.source_manifest_sha256)
    Assert-Q4Equal 'source_manifest_sidecar' (((Get-Content $sidecar -Raw).Trim()-split'\s+')[0]) ([string]$Config.activation.source_manifest_sha256)
    $manifest=Get-Content -LiteralPath $path -Raw|ConvertFrom-Json
    if(($manifest.services-join',')-cne'api,forensic-records-api'-or[bool]$manifest.worker_required){throw 'ACTIVATION_PREREQUISITE_FAILED:source_manifest_scope'}
    $names=@($manifest.files.PSObject.Properties.Name);[Array]::Sort($names,[StringComparer]::Ordinal);$ordered=[ordered]@{}
    foreach($name in $names){$expected=[string]$manifest.files.PSObject.Properties[$name].Value;Assert-Q4Equal "activation_source:$name" (Get-NxDHash (Resolve-NxDPath $name)) $expected;$ordered[$name]=$expected}
    $bytes=[Text.Encoding]::UTF8.GetBytes(($ordered|ConvertTo-Json -Compress));$sha=[Security.Cryptography.SHA256]::Create()
    try{$digest=([BitConverter]::ToString($sha.ComputeHash($bytes))).Replace('-','').ToLowerInvariant()}finally{$sha.Dispose()}
    Assert-Q4Equal 'source_digest' $digest ([string]$manifest.source_digest_sha256)
    return $manifest
}

$exitCode=40;$run=$null;$transcript=$false
try{
    $run=New-NxDRun 'q4-activation';Start-Transcript -LiteralPath (Join-Path $run 'operator.log') -Force|Out-Null;$transcript=$true
    if(-not$OwnerApproved){throw 'ACTIVATION_PREREQUISITE_FAILED:-OwnerApproved required after qualification receipt review'}
    $cfg=Get-Content -LiteralPath (Join-Path $PSScriptRoot 'nxb21d_q4_qualification_config.json') -Raw|ConvertFrom-Json
    Assert-Q4Framework $cfg
    try{try{$null=Assert-NxDRam $cfg 'activation_preflight'}catch{if($_.Exception.Message-like'RAM_TOO_LOW:*'){$null=Invoke-NxDSafeLinuxCleanCache;$null=Wait-NxDRamAfterCleanCache $cfg 'activation_preflight' 180}else{throw}}}catch{if($_.Exception.Message-like'RAM_TOO_LOW:*'){$exitCode=41};throw}
    $receiptPath=Resolve-NxDPath $cfg.public_receipt;$receiptSidecar=$receiptPath+'.sha256';$sealPath=Resolve-NxDPath $cfg.activation.source_seal;$sealSidecar=$sealPath+'.sha256'
    if(-not(Test-Path -LiteralPath $receiptPath)-or-not(Test-Path -LiteralPath $receiptSidecar)-or-not(Test-Path -LiteralPath $sealPath)-or-not(Test-Path -LiteralPath $sealSidecar)){throw 'ACTIVATION_PREREQUISITE_FAILED:qualification_receipt_or_seal_missing'}
    $receiptHash=((Get-Content $receiptSidecar -Raw).Trim()-split'\s+')[0];Assert-Q4Equal 'qualification_receipt_integrity' (Get-NxDHash $receiptPath) $receiptHash
    $receipt=Get-Content -LiteralPath $receiptPath -Raw|ConvertFrom-Json
    Assert-NxQ4QualificationReceipt $receipt
    Assert-Q4Equal 'candidate_identity' ([string]$receipt.candidate.identity_sha256) ([string]$cfg.candidate.identity_sha256)
    Assert-Q4Equal 'holdout_identity' ([string]$receipt.holdout_sha256) ([string]$cfg.corpus.sha256)
    $manifest=Assert-Q4SourceManifest $cfg
    Assert-Q4Equal 'source_seal_integrity' (Get-NxDHash $sealPath) (((Get-Content $sealSidecar -Raw).Trim()-split'\s+')[0])
    $seal=Get-Content -LiteralPath $sealPath -Raw|ConvertFrom-Json
    Assert-Q4Equal 'seal_receipt' ([string]$seal.D_evaluation_receipt_sha256) $receiptHash
    Assert-Q4Equal 'seal_manifest' ([string]$seal.source_manifest_sha256) ([string]$cfg.activation.source_manifest_sha256)
    Assert-Q4Equal 'seal_digest' ([string]$seal.source_digest_sha256) ([string]$manifest.source_digest_sha256)
    $before=Assert-Q4Baseline $cfg
    if(Get-NxDLoadedState $cfg){throw 'ACTIVATION_PREREQUISITE_FAILED:Q4_model_loaded'}

    . (Resolve-NxDPath 'scripts/nxmmr_stt_query_operator_common.ps1')
    $script:NxSttChanged=@('api','forensic-records-api');$script:NxSttProtected=@('forensic-records-worker','forensic-postgres','forensic-nats')
    $containers=Get-NxContainers;$oldImages=[ordered]@{api=$containers.api.Image;'forensic-records-api'=$containers['forensic-records-api'].Image}
    $tag=[DateTime]::UtcNow.ToString('yyyyMMddHHmmss');$newImages=[ordered]@{api="nexusai/localai-nxb21-b1c-q4:$tag";'forensic-records-api'="nexusai/forensic-records-api-nxb21-b1c-q4:$tag"}
    $buildManifest=[pscustomobject]@{services=[pscustomobject]@{dockerfiles=[pscustomobject]@{api='Dockerfile';'forensic-records-api'='api/forensic_records/Dockerfile'}}}
    $build=New-NxSttCompose $containers $newImages ([ordered]@{}) $buildManifest -Build
    $build.services.api.build.network='default';$build.services.api.build.args=[ordered]@{LOCALAI_BUILD_GOMAXPROCS='4';LOCALAI_BUILD_GOFLAGS='-p=2'};$build.services['forensic-records-api'].build.network='default'
    $compose=Join-Path $run 'candidate-compose.json';Write-NxDJson $compose $build
    $rollback=New-NxSttCompose $containers $oldImages ([ordered]@{}) $buildManifest;$rollbackPath=Join-Path $run 'rollback-compose.json';Write-NxDJson $rollbackPath $rollback
    $null=Invoke-NxDNative docker @('compose','-f',$compose,'config','--quiet');$null=Invoke-NxDNative docker @('compose','-f',$rollbackPath,'config','--quiet')
    try{$null=Invoke-NxDSafeLinuxCleanCache;$null=Wait-NxDRamAfterCleanCache $cfg 'immediately_before_first_build' 180}catch{if($_.Exception.Message-like'RAM_TOO_LOW:*'){$exitCode=41};throw}
    Write-Host 'ACTIVATION_MUTATION=BEGIN services=api,forensic-records-api worker=false'
    try{$null=Invoke-NxDNative docker @('compose','-f',$compose,'build','--pull=false','api');$null=Invoke-NxDNative docker @('compose','-f',$compose,'build','--pull=false','forensic-records-api')}catch{$exitCode=42;throw}
    try{$null=Invoke-NxDNative docker @('compose','-f',$compose,'up','-d','--no-deps','api','forensic-records-api')}catch{$exitCode=43;throw}
    Wait-Q4HTTPHealth $cfg 180;$after=Get-NxDContainers
    try{
        $expectedImages=@{api=Get-NxDImageId $newImages.api;'forensic-records-api'=Get-NxDImageId $newImages['forensic-records-api']}
        Assert-NxDServiceImages $before.containers $after $expectedImages $true;Assert-NxDProtectedServices $before.containers $after;Assert-NxDHTTPHealth $cfg
        if((Get-NxDActiveJobs $after)-ne0-or(Get-NxDRetainedTuple $after)-cne$before.retained_tuple-or(Get-NxDActivityCount $after)-ne$before.activity_count){throw 'ACTIVATION_VERIFICATION_FAILED:retained_state'}
        if(Get-NxDLoadedState $cfg){throw 'ACTIVATION_VERIFICATION_FAILED:model_state_changed'}
    }catch{$exitCode=44;throw}
    $activation=[ordered]@{schema_version='nexusai.nxb21-b1c-q4-activation-receipt/v1';receipt_id=[DateTime]::UtcNow.ToString('yyyyMMddTHHmmssfffZ');state='VERIFIED';candidate_identity_sha256=$cfg.candidate.identity_sha256;D_evaluation_receipt_sha256=$receiptHash;source_digest_sha256=$manifest.source_digest_sha256;services=@('api','forensic-records-api');worker_activated=$false;before_images=$oldImages;after_images=@{api=$after.api.Image;'forensic-records-api'=$after['forensic-records-api'].Image};protected_services_unchanged=$true;retained_tuple=$before.retained_tuple;activity_count_unchanged=$true;database_migration=$false;volumes_changed=$false;model_state_changed=$false;rollback_compose=$rollbackPath}
    Write-NxDJson (Join-Path $run 'activation-receipt.json') $activation;Write-NxDJson (Resolve-NxDPath 'reports/nxb21/b1c-q4-activation-receipt-latest.json') $activation
    $exitCode=0;Write-Host "NXB21_B1C_Q4_ACTIVATION=PASS receipt=$($activation.receipt_id)";Write-Host 'EXACT_NEXT_ACTION=Reopen Codex and provide reports/nxb21/b1c-q4-activation-receipt-latest.json for independent review before E/F.'
}catch{if($exitCode-eq40-and$_.Exception.Message-like'*build*'){$exitCode=42}elseif($exitCode-eq40-and$_.Exception.Message-like'*up*'){$exitCode=43};Write-Host "NXB21_B1C_Q4_ACTIVATION=FAIL exit_code=$exitCode error=$($_.Exception.Message)";Write-Host 'EXACT_NEXT_ACTION=Do not broaden service scope. Reopen Codex with this run log; use the captured rollback only if a live service was recreated.'}
finally{if($transcript){Stop-Transcript -ErrorAction SilentlyContinue|Out-Null}}
exit $exitCode
