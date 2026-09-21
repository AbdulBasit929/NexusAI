# SPDX-License-Identifier: MIT
[CmdletBinding()]
param()
Set-StrictMode -Version Latest
$ErrorActionPreference='Stop'
$root=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
. (Join-Path $root 'scripts\nxb21d-offline\common.ps1')
. (Join-Path $root 'scripts\nxb21d-development\byte_safe_transport.ps1')
. (Join-Path $root 'scripts\nxb21d-q4-qualification\qualification_guards.ps1')

function Assert-Equal([string]$Name,[string]$Actual,[string]$Expected){if($Actual-cne$Expected){throw "INTEGRITY_MISMATCH:$Name expected=$Expected actual=$Actual"}}
function Assert-Framework($Config){foreach($p in $Config.framework_files.PSObject.Properties){$path=Resolve-NxDPath $p.Name;if(-not(Test-Path -LiteralPath $path -PathType Leaf)){throw "PREFLIGHT_MISSING:$($p.Name)"};Assert-Equal "source:$($p.Name)" (Get-NxDHash $path) ([string]$p.Value)}}
function Get-DevelopmentFreezeIdentity($Config){
    $lines=@(
        "freeze_id=$($Config.development_freeze.freeze_id)",
        "artifact_sha256=$($Config.model.artifact_sha256)",
        "q4_profile_sha256=$($Config.model.profile_sha256)",
        "q8_profile_sha256=$($Config.model.q8_profile_sha256)",
        "query_language_sha256=$($Config.framework_files.'api/forensic_records/query_language_assistance.go')",
        "exact_authority_sha256=$($Config.framework_files.'scripts/nxb21d-remediation/exact_identifier_authority.ps1')",
        "phrase_authority_sha256=$($Config.framework_files.'scripts/nxb21d-phrase-remediation/phrase_literal_authority.ps1')",
        "scope_authority_sha256=$($Config.framework_files.'scripts/nxb21d-scope-remediation/explicit_scope_authority.ps1')",
        "contracts_sha256=$($Config.framework_files.'api/forensic_records/query_intelligence_contracts.go')",
        "capability_refs_sha256=$($Config.framework_files.'api/forensic_records/contracts/query-capability-references-v1.json')",
        "transport_sha256=$($Config.framework_files.'scripts/nxb21d-development/byte_safe_transport.ps1')",
        "corpus_sha256=$($Config.corpus.sha256)",
        "evaluator_sha256=$($Config.evaluator.sha256)",
        "runner_sha256=$($Config.runner.sha256)",
        "completion_budget=$($Config.development_freeze.completion_budget)"
    )
    $bytes=[Text.UTF8Encoding]::new($false).GetBytes(($lines-join"`n")+"`n");$sha=[Security.Cryptography.SHA256]::Create();try{$hash=$sha.ComputeHash($bytes)}finally{$sha.Dispose()}
    return ([BitConverter]::ToString($hash)).Replace('-','').ToLowerInvariant()
}
function Assert-Baseline($Config){
    $containers=Get-NxDContainers
    foreach($p in $Config.runtime.containers.PSObject.Properties){$a=$containers[$p.Name];$e=$p.Value;if($null-eq$a-or-not(Test-NxDContainerHealthy $a)){throw "RUNTIME_UNHEALTHY:$($p.Name)"};Assert-Equal "$($p.Name).id" ([string]$a.Id) ([string]$e.id);Assert-Equal "$($p.Name).image" ([string]$a.Image) ([string]$e.image);if([int]$a.RestartCount-ne[int]$e.restarts){throw "RUNTIME_DRIFT:$($p.Name).restarts"}}
    Assert-NxDHTTPHealth $Config
    $jobs=Get-NxDActiveJobs $containers;if($jobs-ne0){throw "ACTIVE_JOBS:$jobs"}
    $tuple=Get-NxDRetainedTuple $containers;Assert-Equal 'retained_tuple' $tuple ([string]$Config.runtime.retained_tuple)
    $activity=Get-NxDActivityCount $containers;if($activity-ne[int]$Config.runtime.activity_count){throw "RETAINED_ACTIVITY_DRIFT:$activity"}
    return [pscustomobject]@{containers=$containers;retained_tuple=$tuple;activity_count=$activity;active_jobs=$jobs}
}

$exitCode=21;$cfg=$null;$run=$null;$transcript=$false;$monitor=$null;$modelMayBeLoaded=$false;$summaryPath=$null
$temporaryTest=Resolve-NxDPath 'api/forensic_records/nxb21d_q4_interface_remediation_test.go'
try{
    $cfg=Read-NxDStrictUtf8Text (Join-Path $PSScriptRoot 'nxb21d_q4_interface_remediation_config.json')|ConvertFrom-Json
    Assert-Framework $cfg
    Assert-Equal 'development_freeze_identity' (Get-DevelopmentFreezeIdentity $cfg) ([string]$cfg.development_freeze.identity_sha256)
    if([int]$cfg.development_freeze.completion_budget-ne512){throw 'INTEGRITY_MISMATCH:completion_budget'}
    $corpusPath=Resolve-NxDPath $cfg.corpus.path
    Assert-Equal 'corpus' (Get-NxDHash $corpusPath) ([string]$cfg.corpus.sha256)
    Assert-NxQ4HoldoutUnconsumed $corpusPath (Resolve-NxDPath $cfg.consumed_holdout_registry)
    $validation=& python (Resolve-NxDPath $cfg.corpus.validator) --repo $root --corpus $corpusPath
    if($LASTEXITCODE-ne0){throw "CORPUS_VALIDATION_FAILED:$($validation-join' ')"}
    Write-Host "CORPUS_FROZEN=PASS cases=24 sha256=$($cfg.corpus.sha256) development_only=true"

    $before=Assert-Baseline $cfg
    $artifact=Resolve-NxDPath $cfg.model.artifact_path;Assert-Equal 'q4_artifact' (Get-NxDHash $artifact) ([string]$cfg.model.artifact_sha256)
    if((Get-Item -LiteralPath $artifact).Length-ne[int64]$cfg.model.artifact_bytes){throw 'INTEGRITY_MISMATCH:q4_artifact_bytes'}
    Assert-Equal 'q4_profile_host' (Get-NxDHash (Resolve-NxDPath $cfg.model.profile_path)) ([string]$cfg.model.profile_sha256)
    Assert-Equal 'q4_artifact_container' ((Invoke-NxDNative docker @('exec',$cfg.runtime.container,'sha256sum',$cfg.model.artifact_container_path))-split'\s+')[0] ([string]$cfg.model.artifact_sha256)
    Assert-Equal 'q4_profile_container' ((Invoke-NxDNative docker @('exec',$cfg.runtime.container,'sha256sum',$cfg.model.profile_container_path))-split'\s+')[0] ([string]$cfg.model.profile_sha256)
    Assert-Equal 'q8_profile_container' ((Invoke-NxDNative docker @('exec',$cfg.runtime.container,'sha256sum',$cfg.model.q8_profile_container_path))-split'\s+')[0] ([string]$cfg.model.q8_profile_sha256)
    $models=@((Invoke-RestMethod -Uri ($cfg.runtime.localai_url+'/v1/models') -TimeoutSec 15).data.id);if($models-cnotcontains[string]$cfg.model.id){throw 'Q4_REGISTRATION_MISSING'}
    Write-Host "Q4_REGISTRATION=PASS loaded=$($null-ne(Get-NxDLoadedState $cfg))"

    $loaded=$null-ne(Get-NxDLoadedState $cfg);$floor=if($loaded){[double]$cfg.resource.minimum_loaded_gib}else{[double]$cfg.resource.minimum_before_load_gib}
    $ramCfg=[pscustomobject]@{gates=[pscustomobject]@{minimum_available_ram_gib=$floor}}
    try{$null=Assert-NxDRam $ramCfg 'before_first_development_inference'}catch{if($_.Exception.Message-like'RAM_TOO_LOW*'){$null=Invoke-NxDSafeLinuxCleanCache;$null=Wait-NxDRamAfterCleanCache $ramCfg 'before_first_development_inference' 180}else{throw}}

    $runRoot=Resolve-NxDPath $cfg.output_root;$run=Join-Path $runRoot ('run-'+[DateTime]::UtcNow.ToString('yyyyMMddTHHmmssfffZ'));New-Item -ItemType Directory -Force -Path $run|Out-Null
    Start-Transcript -LiteralPath (Join-Path $run 'operator.log') -Force|Out-Null;$transcript=$true
    $summaryPath=Join-Path $run 'interface-remediation-24case-development-summary-v1.json'
    Write-Host "NXB21D_Q4_INTERFACE_REMEDIATION_24CASE=START run=$run"
    Write-Host "DEVELOPMENT_FREEZE=PASS id=$($cfg.development_freeze.freeze_id) identity_sha256=$($cfg.development_freeze.identity_sha256) corpus_sha256=$($cfg.corpus.sha256) completion_budget=512"

    if(Test-Path -LiteralPath $temporaryTest){throw 'SOURCE_DRIFT:temporary evaluator exists'}
    Copy-Item -LiteralPath (Resolve-NxDPath $cfg.evaluator.path) -Destination $temporaryTest
    $resourcePath=Join-Path $run 'resource-samples.jsonl'
    $monitor=Start-Job -ArgumentList $resourcePath,$cfg.runtime.container -ScriptBlock {param($path,$container);while($true){try{$os=Get-CimInstance Win32_OperatingSystem;$ram=[math]::Round(([int64]$os.FreePhysicalMemory*1024)/1GB,3);$stats=& docker stats --no-stream --format '{{json .}}' $container 2>$null;(@{utc=[DateTime]::UtcNow.ToString('o');available_ram_gib=$ram;docker_stats=$stats}|ConvertTo-Json -Compress)|Add-Content -LiteralPath $path -Encoding UTF8}catch{};Start-Sleep 5}}
    $env:NXB21D_DEV_CORPUS=$corpusPath;$env:NXB21D_DEV_RESULT=$summaryPath;$env:NXB21D_DEV_RUN_DIR=$run;$env:NXB21D_LOCALAI_URL=$cfg.runtime.localai_url;$env:NXB21D_MODEL=$cfg.model.id
    $env:GOCACHE=Join-Path $run 'go-cache';$env:GOTMPDIR=Join-Path $run 'go-tmp';New-Item -ItemType Directory -Force -Path $env:GOCACHE,$env:GOTMPDIR|Out-Null
    $modelMayBeLoaded=$true
    Invoke-NxDNativeStreaming go @('test','-v','./api/forensic_records','-run','TestForensicRecordsSynthesis','-count=1','-timeout','2h','-args','--ginkgo.focus=NX-B2[.]1D Q4 production interface 24-case development','--ginkgo.v','--ginkgo.timeout=2h') (Join-Path $run 'go-evaluator.log')
    if(-not(Test-Path -LiteralPath $summaryPath)){throw 'DEVELOPMENT_INVALID:no aggregate result'}
    $result=Read-NxDStrictUtf8Text $summaryPath|ConvertFrom-Json
    if([int]$result.cases_completed-ne24-or[int]$result.model_calls-ne20){throw 'DEVELOPMENT_INVALID:incomplete'}
    Invoke-NxDUnload $cfg;$modelMayBeLoaded=$false;Start-Sleep 3;if(Get-NxDLoadedState $cfg){throw 'MODEL_UNLOAD_FAILED'}
    $after=Assert-Baseline $cfg
    Write-Host "RUNTIME_INTEGRITY=PASS retained_tuple=$($after.retained_tuple) activity_count=$($after.activity_count) active_jobs=$($after.active_jobs) model_unloaded=true"
    $result|Add-Member -NotePropertyName runtime_postcheck -NotePropertyValue 'PASS'
    $result|Add-Member -NotePropertyName retained_tuple -NotePropertyValue $after.retained_tuple
    $result|Add-Member -NotePropertyName activity_count -NotePropertyValue $after.activity_count
    $result|Add-Member -NotePropertyName model_unloaded -NotePropertyValue $true
    Write-NxDJson $summaryPath $result
    [IO.File]::WriteAllText($summaryPath+'.sha256',(Get-NxDHash $summaryPath)+'  '+(Split-Path $summaryPath -Leaf)+"`n",[Text.UTF8Encoding]::new($false))
    if([string]$result.development_gate-ceq'PASS'){$exitCode=0;Write-Host 'NXB21D_Q4_INTERFACE_REMEDIATION_24CASE=PASS'}else{$exitCode=22;Write-Host 'NXB21D_Q4_INTERFACE_REMEDIATION_24CASE=DEVELOPMENT_FAILURE'}
    Write-Host "DEVELOPMENT_RECEIPT=$summaryPath";Write-Host "DEVELOPMENT_RECEIPT_SHA256=$(Get-NxDHash $summaryPath)"
    Write-Host 'EXACT_NEXT_ACTION=Preserve the run and reopen Codex with the receipt. Do not generate a qualification holdout or activate.'
}catch{
    $message=$_.Exception.Message
    if($message-like'RAM_TOO_LOW*'){$exitCode=10}elseif($message-like'*DRIFT*'-or$message-like'INTEGRITY_*'-or$message-like'PREFLIGHT_*'-or$message-like'CORPUS_*'-or$message-like'HOLDOUT_*'){$exitCode=11}elseif($message-like'RUNTIME_*'-or$message-like'ACTIVE_JOBS*'){$exitCode=12}elseif($message-like'MODEL_*'-or$message-like'Q4_REGISTRATION*'){$exitCode=20}
    Write-Host "NXB21D_Q4_INTERFACE_REMEDIATION_24CASE=FAIL error=$message exit_code=$exitCode"
    Write-Host 'EXACT_NEXT_ACTION=Preserve the run if created and reopen Codex with the terminal error. Do not retry after integrity or transport corruption.'
}finally{
    if($monitor){Stop-Job $monitor -ErrorAction SilentlyContinue;Remove-Job $monitor -Force -ErrorAction SilentlyContinue}
    if(Test-Path -LiteralPath $temporaryTest){Remove-Item -LiteralPath $temporaryTest -Force}
    try{if($null-ne$cfg-and($modelMayBeLoaded-or(Get-NxDLoadedState $cfg))){Invoke-NxDUnload $cfg;Start-Sleep 3}}catch{Write-Host "SAFE_UNLOAD_ERROR=$($_.Exception.Message)"}
    Remove-Item Env:NXB21D_DEV_CORPUS,Env:NXB21D_DEV_RESULT,Env:NXB21D_DEV_RUN_DIR,Env:NXB21D_LOCALAI_URL,Env:NXB21D_MODEL,Env:GOCACHE,Env:GOTMPDIR -ErrorAction SilentlyContinue
    if($transcript){Stop-Transcript -ErrorAction SilentlyContinue|Out-Null}
}
exit $exitCode
