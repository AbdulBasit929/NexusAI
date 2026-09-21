# SPDX-License-Identifier: MIT
[CmdletBinding()]
param([switch]$ValidateOnly)
Set-StrictMode -Version Latest
$ErrorActionPreference='Stop'
$root=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
. (Join-Path $root 'scripts\nxb21d-offline\common.ps1')
. (Join-Path $root 'scripts\nxb21d-development\byte_safe_transport.ps1')
. (Join-Path $root 'scripts\nxb21d-q4-qualification\qualification_guards.ps1')
. (Join-Path $root 'scripts\nxb21d-hybrid-development\immutable_receipt.ps1')

function Assert-Equal([string]$Name,[string]$Actual,[string]$Expected){if($Actual-cne$Expected){throw "INTEGRITY_MISMATCH:$Name expected=$Expected actual=$Actual"}}
function Assert-Framework($Config){foreach($p in $Config.framework_files.PSObject.Properties){$path=Resolve-NxDPath $p.Name;if(-not(Test-Path -LiteralPath $path -PathType Leaf)){throw "PREFLIGHT_MISSING:$($p.Name)"};Assert-Equal "source:$($p.Name)" (Get-NxDHash $path) ([string]$p.Value)}}
function Get-DevelopmentFreezeIdentity($Config){
    $lines=@("freeze_id=$($Config.development_freeze.freeze_id)","artifact_sha256=$($Config.model.artifact_sha256)","q4_profile_sha256=$($Config.model.profile_sha256)","q8_profile_sha256=$($Config.model.q8_profile_sha256)","runtime_sha256=$($Config.runtime_sha256)","completion_budget=512")
    [string[]]$names=@($Config.framework_files.PSObject.Properties.Name)
    [Array]::Sort($names,[StringComparer]::Ordinal)
    foreach($name in $names){$lines += "${name}=$($Config.framework_files.$name)"}
    return Get-NxDBytesSHA256 ([Text.UTF8Encoding]::new($false).GetBytes(($lines-join"`n")+"`n"))
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

$exitCode=21;$cfg=$null;$run=$null;$transcript=$false;$monitor=$null;$modelMayBeLoaded=$false;$summaryPath=$null;$intermediatePath=$null;$result=$null;$postState='NOT_RUN';$after=$null

try{
    $configPath=Join-Path $PSScriptRoot 'decision8_config.json'
    Assert-Equal 'operator_config' (Get-NxDHash $configPath) ((Read-NxDStrictUtf8Text ($configPath+'.sha256')).Trim().Split(' ')[0])
    $cfg=Read-NxDStrictUtf8Text $configPath|ConvertFrom-Json
    Assert-Framework $cfg
    $runtimePath=Join-Path $PSScriptRoot 'runtime-baseline.json'
    Assert-Equal 'runtime_baseline' (Get-NxDHash $runtimePath) ([string]$cfg.runtime_sha256)
    $cfg.runtime=Read-NxDStrictUtf8Text $runtimePath|ConvertFrom-Json
    Assert-Equal 'development_freeze_identity' (Get-DevelopmentFreezeIdentity $cfg) ([string]$cfg.development_freeze.identity_sha256)
    if([int]$cfg.development_freeze.completion_budget-ne512){throw 'INTEGRITY_MISMATCH:completion_budget'}
    $corpusPath=Resolve-NxDPath $cfg.corpus.path
    Assert-Equal 'corpus' (Get-NxDHash $corpusPath) ([string]$cfg.corpus.sha256)
    Assert-NxQ4HoldoutUnconsumed $corpusPath (Resolve-NxDPath $cfg.consumed_holdout_registry)
    $validation=& python (Resolve-NxDPath $cfg.corpus.validator) --repo $root --corpus $corpusPath
    if($LASTEXITCODE-ne0){throw "CORPUS_VALIDATION_FAILED:$($validation-join' ')"}
    Write-Host "CORPUS_FROZEN=PASS cases=8 sha256=$($cfg.corpus.sha256) development_only=true"

    if($ValidateOnly){Write-Host 'DECISION8_FRAMEWORK=PASS live_inference=false';exit 0}
    Set-Location -LiteralPath $root
    if(Test-Path -LiteralPath (Join-Path (Resolve-NxDPath $cfg.output_root) 'development-dispatched.lock')){throw 'DEVELOPMENT_ALREADY_DISPATCHED'}
    $before=Assert-Baseline $cfg
    $artifact=Resolve-NxDPath $cfg.model.artifact_path;Assert-Equal 'q4_artifact' (Get-NxDHash $artifact) ([string]$cfg.model.artifact_sha256)
    if((Get-Item -LiteralPath $artifact).Length-ne[int64]$cfg.model.artifact_bytes){throw 'INTEGRITY_MISMATCH:q4_artifact_bytes'}
    Assert-Equal 'q4_profile_host' (Get-NxDHash (Resolve-NxDPath $cfg.model.profile_path)) ([string]$cfg.model.profile_sha256)
    Assert-Equal 'q4_artifact_container' ((Invoke-NxDNative docker @('exec',$cfg.runtime.container,'sha256sum',$cfg.model.artifact_container_path))-split'\s+')[0] ([string]$cfg.model.artifact_sha256)
    Assert-Equal 'q4_profile_container' ((Invoke-NxDNative docker @('exec',$cfg.runtime.container,'sha256sum',$cfg.model.profile_container_path))-split'\s+')[0] ([string]$cfg.model.profile_sha256)
    Assert-Equal 'q8_profile_container' ((Invoke-NxDNative docker @('exec',$cfg.runtime.container,'sha256sum',$cfg.model.q8_profile_container_path))-split'\s+')[0] ([string]$cfg.model.q8_profile_sha256)
    $models=@((Invoke-RestMethod -Uri ($cfg.runtime.localai_url+'/v1/models') -TimeoutSec 15).data.id);if($models-cnotcontains[string]$cfg.model.id){throw 'Q4_REGISTRATION_MISSING'}
    if(Get-NxDLoadedState $cfg){throw 'MODEL_ALREADY_LOADED:unload Q4 before starting this standalone gate'}
    Write-Host "Q4_REGISTRATION=PASS loaded=$($null-ne(Get-NxDLoadedState $cfg))"

    $loaded=$null-ne(Get-NxDLoadedState $cfg);$floor=if($loaded){[double]$cfg.resource.minimum_loaded_gib}else{[double]$cfg.resource.minimum_before_load_gib}
    $ramCfg=[pscustomobject]@{gates=[pscustomobject]@{minimum_available_ram_gib=$floor}}
    try{$null=Assert-NxDRam $ramCfg 'before_first_development_inference'}catch{if($_.Exception.Message-like'RAM_TOO_LOW*'){$null=Invoke-NxDSafeLinuxCleanCache;$null=Wait-NxDRamAfterCleanCache $ramCfg 'before_first_development_inference' 180}else{throw}}

    $runRoot=Resolve-NxDPath $cfg.output_root;$run=Join-Path $runRoot ('run-'+[DateTime]::UtcNow.ToString('yyyyMMddTHHmmssfffZ'));New-Item -ItemType Directory -Force -Path $run|Out-Null
    Start-Transcript -LiteralPath (Join-Path $run 'operator.log') -Force|Out-Null;$transcript=$true
    $summaryPath=Join-Path $run 'decision8-development-summary-v1.json';$intermediatePath=Join-Path $run 'decision8-development-intermediate-v1.json'
    Write-Host "NXB21D_DECISION8=START run=$run"
    Write-Host "DEVELOPMENT_FREEZE=PASS id=$($cfg.development_freeze.freeze_id) identity_sha256=$($cfg.development_freeze.identity_sha256) corpus_sha256=$($cfg.corpus.sha256) completion_budget=512"

    $resourcePath=Join-Path $run 'resource-samples.jsonl'
    $monitor=Start-Job -ArgumentList $resourcePath,$cfg.runtime.container -ScriptBlock {param($path,$container);while($true){try{$os=Get-CimInstance Win32_OperatingSystem;$ram=[math]::Round(([int64]$os.FreePhysicalMemory*1024)/1GB,3);$stats=& docker stats --no-stream --format '{{json .}}' $container 2>$null;(@{utc=[DateTime]::UtcNow.ToString('o');available_ram_gib=$ram;docker_stats=$stats}|ConvertTo-Json -Compress)|Add-Content -LiteralPath $path -Encoding UTF8}catch{};Start-Sleep 5}}
    $env:NXB21D_DECISION8_CORPUS=$corpusPath;$env:NXB21D_DECISION8_RUN_DIR=$run;$env:NXB21D_DECISION8_LIVE='1';$env:NXB21D_DECISION8_RAM_SCRIPT=Join-Path $root 'scripts\nxb21d-hybrid-development\check_hybrid_ram.ps1';$env:NXB21D_DECISION8_BINDINGS=Join-Path $PSScriptRoot 'schema-bindings-v1.json';$env:NXB21D_LOCALAI_URL=$cfg.runtime.localai_url;$env:NXB21D_MODEL=$cfg.model.id
    $lock=Join-Path $runRoot 'development-dispatched.lock'
    $claim=[IO.File]::Open($lock,[IO.FileMode]::CreateNew,[IO.FileAccess]::Write,[IO.FileShare]::None)
    try{$bytes=[Text.UTF8Encoding]::new($false).GetBytes($run);$claim.Write($bytes,0,$bytes.Length)}finally{$claim.Dispose()}
    $modelMayBeLoaded=$true
    Invoke-NxDNativeStreaming go @('test','-v','./api/forensic_records','-run','TestForensicRecordsSynthesis','-count=1','-timeout','2h','-args','--ginkgo.focus=NXB21D decision8 standalone development','--ginkgo.v','--ginkgo.timeout=2h') (Join-Path $run 'go-evaluator.log')
    if(-not(Test-Path -LiteralPath $intermediatePath)){throw 'DEVELOPMENT_INVALID:no aggregate result'}
    $result=Read-NxDStrictUtf8Text $intermediatePath|ConvertFrom-Json
    Invoke-NxDUnload $cfg;Start-Sleep 3;if(Get-NxDLoadedState $cfg){throw 'MODEL_UNLOAD_FAILED'};$modelMayBeLoaded=$false
    $after=Assert-Baseline $cfg
    Write-Host "RUNTIME_INTEGRITY=PASS retained_tuple=$($after.retained_tuple) activity_count=$($after.activity_count) active_jobs=$($after.active_jobs) model_unloaded=true"
    $postState='PASS'
    if([string]$result.development_gate-ceq'PASS'-and[int]$result.cases_completed-eq8-and[int]$result.model_calls-eq8-and[int]$result.raw_tuple_correct-eq8){$exitCode=0}else{$exitCode=22}
    Write-Host 'EXACT_NEXT_ACTION=Preserve the run and reopen Codex with the receipt. Do not generate a qualification holdout or activate.'
}catch{
    $message=$_.Exception.Message
    if($message-like'RAM_TOO_LOW*'){$exitCode=10}elseif($message-like'*DRIFT*'-or$message-like'INTEGRITY_*'-or$message-like'PREFLIGHT_*'-or$message-like'CORPUS_*'-or$message-like'HOLDOUT_*'){$exitCode=11}elseif($message-like'RUNTIME_*'-or$message-like'ACTIVE_JOBS*'){$exitCode=12}elseif($message-like'MODEL_*'-or$message-like'Q4_REGISTRATION*'){$exitCode=20}
    Write-Host "NXB21D_DECISION8=FAIL error=$message exit_code=$exitCode"
    if($run){$failurePath=Join-Path $run 'operator-failure.json';Write-NxDJson $failurePath @{development_gate='FAIL';error=$message;run=$run;D_status='OPEN';activation='BLOCKED'};[IO.File]::WriteAllText($failurePath+'.sha256',(Get-NxDHash $failurePath)+"`n",[Text.UTF8Encoding]::new($false))}
    Write-Host 'EXACT_NEXT_ACTION=Preserve the run if created and reopen Codex with the terminal error. Do not retry after integrity or transport corruption.'
}finally{
    if($monitor){Stop-Job $monitor -ErrorAction SilentlyContinue;Remove-Job $monitor -Force -ErrorAction SilentlyContinue}

    try{if($null-ne$cfg-and$modelMayBeLoaded){Invoke-NxDUnload $cfg;Start-Sleep 3;if(Get-NxDLoadedState $cfg){throw 'MODEL_UNLOAD_FAILED'};$modelMayBeLoaded=$false}}catch{$exitCode=20;Write-Host "SAFE_UNLOAD_ERROR=$($_.Exception.Message)"}
    Remove-Item Env:NXB21D_DECISION8_CORPUS,Env:NXB21D_DECISION8_RUN_DIR,Env:NXB21D_DECISION8_LIVE,Env:NXB21D_DECISION8_RAM_SCRIPT,Env:NXB21D_DECISION8_BINDINGS,Env:NXB21D_LOCALAI_URL,Env:NXB21D_MODEL -ErrorAction SilentlyContinue
    if($run){
        if(-not $summaryPath){$summaryPath=Join-Path $run 'decision8-development-summary-v1.json'}
        if($postState -ne 'PASS'){
            try{$after=Assert-Baseline $cfg;$postState='PASS'}catch{$postState='FAIL';$exitCode=12;Write-Host "RUNTIME_POSTCHECK_ERROR=$($_.Exception.Message)"}
        }
        try{
            if($null-eq$result){
                $result=if($intermediatePath-and(Test-Path -LiteralPath $intermediatePath)){Read-NxDStrictUtf8Text $intermediatePath|ConvertFrom-Json}else{[pscustomobject]@{cases_completed=0;development_gate='FAIL'}}
            }
            if($exitCode-ne0){$result|Add-Member -Force -NotePropertyName development_gate -NotePropertyValue 'FAIL'}
            $result|Add-Member -Force -NotePropertyName runtime_postcheck -NotePropertyValue $postState
            $result|Add-Member -Force -NotePropertyName activation -NotePropertyValue 'BLOCKED'
            if($null-ne$after){
                $result|Add-Member -Force -NotePropertyName retained_tuple -NotePropertyValue $after.retained_tuple
                $result|Add-Member -Force -NotePropertyName activity_count -NotePropertyValue $after.activity_count
                $result|Add-Member -Force -NotePropertyName active_jobs -NotePropertyValue $after.active_jobs
            }
            $result|Add-Member -Force -NotePropertyName model_unloaded -NotePropertyValue (-not $modelMayBeLoaded)
            $seal=Seal-NxHybridReceipt $summaryPath $result
            Write-NxHybridReceiptOutput $seal
            if($exitCode-eq0){Write-Host 'NXB21D_DECISION8=PASS'}else{Write-Host 'NXB21D_DECISION8=DEVELOPMENT_FAILURE'}
        }catch{$exitCode=21;Write-Host "FINAL_RECEIPT_SEAL_FAILED:$($_.Exception.Message)"}
    }
    if($transcript){Stop-Transcript -ErrorAction SilentlyContinue|Out-Null}
}
exit $exitCode
