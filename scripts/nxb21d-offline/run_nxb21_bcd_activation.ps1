# SPDX-License-Identifier: MIT
[CmdletBinding()]
param([switch]$OwnerApproved)
. (Join-Path $PSScriptRoot 'common.ps1')
$exitCode=40;$run=New-NxDRun 'activation';Start-Transcript -LiteralPath (Join-Path $run 'operator.log') -Force|Out-Null
try {
  if(-not$OwnerApproved){throw 'ACTIVATION_PREREQUISITE_FAILED: -OwnerApproved is required after Codex review'}
  $cfg=Get-NxDConfig;try{$null=Assert-NxDRam $cfg 'activation_preflight'}catch{if($_.Exception.Message-like'RAM_TOO_LOW:*'){$exitCode=41};throw}
  $receiptPath=Resolve-NxDPath 'reports/nxb21/d-model-evaluation-receipt-v1.json';$hashPath=$receiptPath+'.sha256';$sealPath=Resolve-NxDPath 'reports/nxb21/d-activation-source-seal-v1.json'
  Assert-NxDActivationArtifacts $receiptPath $hashPath $sealPath
  $expectedHash=((Get-Content $hashPath -Raw).Trim()-split'\s+')[0];if((Get-NxDHash $receiptPath)-cne$expectedHash){throw 'ACTIVATION_PREREQUISITE_FAILED:receipt integrity'}
  $receipt=Get-Content $receiptPath -Raw|ConvertFrom-Json
  Assert-NxDActivationReceipt $receipt
  $seal=Get-Content $sealPath -Raw|ConvertFrom-Json;if($seal.D_evaluation_receipt_sha256-cne$expectedHash-or$seal.source_digest-cne[string]$cfg.d_source_digest-or($seal.services-join',')-cne'api,forensic-records-api'-or[bool]$seal.worker_required){throw 'ACTIVATION_PREREQUISITE_FAILED:source seal'}
  $before=Assert-NxDBaseline $cfg
  foreach($p in $seal.files.PSObject.Properties){if((Get-NxDHash (Resolve-NxDPath $p.Name))-cne[string]$p.Value){throw "ACTIVATION_PREREQUISITE_FAILED:source hash $($p.Name)"}}
  if(Get-NxDLoadedState $cfg){throw 'ACTIVATION_PREREQUISITE_FAILED:Qwen load is a separate governed boundary'}

  . (Resolve-NxDPath 'scripts/nxmmr_stt_query_operator_common.ps1')
  $script:NxSttChanged=@('api','forensic-records-api');$script:NxSttProtected=@('forensic-records-worker','forensic-postgres','forensic-nats')
  $containers=Get-NxContainers;$oldImages=[ordered]@{api=$containers['api'].Image;'forensic-records-api'=$containers['forensic-records-api'].Image}
  $tag=[DateTime]::UtcNow.ToString('yyyyMMddHHmmss');$newImages=[ordered]@{api="nexusai/localai-nxb21-bcd:$tag";'forensic-records-api'="nexusai/forensic-records-api-nxb21-bcd:$tag"}
  $manifest=[pscustomobject]@{services=[pscustomobject]@{dockerfiles=[pscustomobject]@{api='Dockerfile';'forensic-records-api'='api/forensic_records/Dockerfile'}}}
  $build=New-NxSttCompose $containers $newImages ([ordered]@{}) $manifest -Build
  $build.services.api.build.network='default';$build.services.api.build.args=[ordered]@{LOCALAI_BUILD_GOMAXPROCS='4';LOCALAI_BUILD_GOFLAGS='-p=2'};$build.services['forensic-records-api'].build.network='default'
  $compose=Join-Path $run 'candidate-compose.json';Write-NxDJson $compose $build
  $rollback=New-NxSttCompose $containers $oldImages ([ordered]@{}) $manifest;$rollbackPath=Join-Path $run 'rollback-compose.json';Write-NxDJson $rollbackPath $rollback
  $null=Invoke-NxDNative docker @('compose','-f',$compose,'config','--quiet');$null=Invoke-NxDNative docker @('compose','-f',$rollbackPath,'config','--quiet')
  try{$null=Assert-NxDRam $cfg 'immediately_before_first_build'}catch{if($_.Exception.Message-like'RAM_TOO_LOW:*'){$exitCode=41};throw}
  Write-Host 'ACTIVATION_MUTATION=BEGIN services=api,forensic-records-api worker=false'
  try{$null=Invoke-NxDNative docker @('compose','-f',$compose,'build','--pull=false','api');$null=Invoke-NxDNative docker @('compose','-f',$compose,'build','--pull=false','forensic-records-api')}catch{$exitCode=42;throw}
  try{$null=Invoke-NxDNative docker @('compose','-f',$compose,'up','-d','--no-deps','api','forensic-records-api')}catch{$exitCode=43;throw}
  Start-Sleep -Seconds 15;$after=Get-NxDContainers
  try{
    $expectedImages=@{api=Get-NxDImageId $newImages.api;'forensic-records-api'=Get-NxDImageId $newImages['forensic-records-api']}
    Assert-NxDServiceImages $before.containers $after $expectedImages $true
    Assert-NxDProtectedServices $before.containers $after
    Assert-NxDHTTPHealth $cfg
    if((Get-NxDActiveJobs $after)-ne0-or(Get-NxDRetainedTuple $after)-cne$before.retained_tuple-or(Get-NxDActivityCount $after)-ne$before.activity_count){throw 'ACTIVATION_VERIFICATION_FAILED:retained state'}
    if(Get-NxDLoadedState $cfg){throw 'ACTIVATION_VERIFICATION_FAILED:model state changed'}
  }catch{$exitCode=44;throw}
  $activation=[ordered]@{schema_version='nexusai.nxb21-bcd-activation-receipt/v1';receipt_id=[DateTime]::UtcNow.ToString('yyyyMMddTHHmmssfffZ');state='VERIFIED';D_source_digest=$cfg.d_source_digest;D_evaluation_receipt_sha256=$expectedHash;services=@('api','forensic-records-api');worker_activated=$false;before_images=$oldImages;after_images=@{api=$after.api.Image;'forensic-records-api'=$after['forensic-records-api'].Image};protected_services_unchanged=$true;retained_tuple=$before.retained_tuple;activity_count_unchanged=$true;database_migration=$false;volumes_changed=$false;model_state_changed=$false;rollback_compose=$rollbackPath}
  Write-NxDJson (Join-Path $run 'activation-receipt.json') $activation;Write-NxDJson (Resolve-NxDPath 'reports/nxb21/bcd-activation-receipt-latest.json') $activation
  $exitCode=0;Write-Host "NXB21_BCD_ACTIVATION=PASS receipt=$($activation.receipt_id)";Write-Host 'EXACT_NEXT_ACTION=Reopen Codex and provide reports/nxb21/bcd-activation-receipt-latest.json for independent review before F.'
} catch {if($exitCode-eq40-and$_.Exception.Message-like'*build*'){$exitCode=42}elseif($exitCode-eq40-and$_.Exception.Message-like'*up*'){$exitCode=43};Write-Host "NXB21_BCD_ACTIVATION=FAIL exit_code=$exitCode error=$($_.Exception.Message)";Write-Host 'EXACT_NEXT_ACTION=Do not broaden service scope. Reopen Codex with this run log; use rollback only if a live service was recreated.'}
finally{Stop-Transcript -ErrorAction SilentlyContinue|Out-Null;$global:LASTEXITCODE=$exitCode}
exit $exitCode
