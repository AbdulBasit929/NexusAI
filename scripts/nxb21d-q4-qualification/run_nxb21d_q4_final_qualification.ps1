# SPDX-License-Identifier: MIT
[CmdletBinding()]
param([switch]$PreparationDryRun)
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$root = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
. (Join-Path $root 'scripts\nxb21d-offline\common.ps1')
. (Join-Path $root 'scripts\nxb21d-development\byte_safe_transport.ps1')
. (Join-Path $PSScriptRoot 'qualification_guards.ps1')

function Assert-Equal([string]$Name, [string]$Actual, [string]$Expected) {
    if ($Actual -cne $Expected) { throw "INTEGRITY_MISMATCH:$Name expected=$Expected actual=$Actual" }
}
function Wait-ContainerHealthy([string]$Name, [int]$TimeoutSeconds = 180) {
    $deadline = [DateTime]::UtcNow.AddSeconds($TimeoutSeconds)
    do {
        $row = (Invoke-NxDNative docker @('inspect', $Name) | ConvertFrom-Json)[0]
        if (Test-NxDContainerHealthy $row) { return $row }
        if ([string]$row.State.Status -in @('dead','exited','removing')) { throw "RUNTIME_UNHEALTHY:${Name}:$($row.State.Status)" }
        Write-Host "RUNTIME_HEALTH_WAIT container=$Name status=$($row.State.Status)"
        Start-Sleep 5
    } while ([DateTime]::UtcNow -lt $deadline)
    throw "RUNTIME_HEALTH_TIMEOUT:$Name"
}
function Assert-Baseline($Config) {
    $containers = Get-NxDContainers
    foreach ($property in $Config.runtime.containers.PSObject.Properties) {
        $actual = $containers[$property.Name]; $expected = $property.Value
        if ($null -eq $actual) { throw "RUNTIME_MISSING:$($property.Name)" }
        if (-not (Test-NxDContainerHealthy $actual)) { $null = Wait-ContainerHealthy ($actual.Name.TrimStart('/')); $containers = Get-NxDContainers; $actual = $containers[$property.Name] }
        Assert-Equal "$($property.Name).id" ([string]$actual.Id) ([string]$expected.id)
        Assert-Equal "$($property.Name).image" ([string]$actual.Image) ([string]$expected.image)
        if ([int]$actual.RestartCount -ne [int]$expected.restarts) { throw "RUNTIME_DRIFT:$($property.Name).restarts" }
    }
    Assert-NxDHTTPHealth $Config
    $jobs = Get-NxDActiveJobs $containers
    if ($jobs -ne 0) { throw "ACTIVE_JOBS:$jobs" }
    $retained = Get-NxDRetainedTuple $containers
    Assert-Equal 'retained_tuple' $retained ([string]$Config.runtime.retained_tuple)
    $activity = Get-NxDActivityCount $containers
    if ($activity -ne [int]$Config.runtime.activity_count) { throw "RETAINED_ACTIVITY_DRIFT:$activity" }
    return [pscustomobject]@{containers=$containers; retained_tuple=$retained; activity_count=$activity; active_jobs=$jobs}
}
function Assert-Framework($Config) {
    foreach ($property in $Config.framework_files.PSObject.Properties) {
        $path = Resolve-NxDPath $property.Name
        if (-not (Test-Path -LiteralPath $path -PathType Leaf)) { throw "PREFLIGHT_MISSING:$($property.Name)" }
        Assert-Equal $property.Name (Get-NxDHash $path) ([string]$property.Value)
    }
}
function Get-CandidateIdentity($Config) {
    $lines = @(
        "candidate_id=$($Config.candidate.candidate_id)",
        "artifact_sha256=$($Config.model.artifact_sha256)",
        "profile_sha256=$($Config.model.profile_sha256)",
        "q8_profile_sha256=$($Config.model.q8_profile_sha256)",
        "query_language_sha256=$($Config.framework_files.'api/forensic_records/query_language_assistance.go')",
        "scope_planner_sha256=$($Config.framework_files.'api/forensic_records/d_dynamic_planner.go')",
        "shared_scope_sha256=$($Config.framework_files.'pkg/forensictext/query.go')",
        "query_bridge_sha256=$($Config.framework_files.'api/forensic_records/query.go')",
        "contracts_sha256=$($Config.framework_files.'api/forensic_records/query_intelligence_contracts.go')",
        "capability_refs_sha256=$($Config.framework_files.'api/forensic_records/contracts/query-capability-references-v1.json')",
        "evaluator_sha256=$($Config.evaluator.sha256)",
        "corpus_sha256=$($Config.corpus.sha256)",
        "temperature=$($Config.candidate.request_temperature)",
        "effective_context=$($Config.candidate.effective_context)"
    )
    $bytes = [Text.UTF8Encoding]::new($false).GetBytes(($lines -join "`n") + "`n")
    $sha = [Security.Cryptography.SHA256]::Create()
    try { $hash = $sha.ComputeHash($bytes) } finally { $sha.Dispose() }
    return ([BitConverter]::ToString($hash)).Replace('-', '').ToLowerInvariant()
}

$exitCode=21; $cfg=$null; $run=$null; $transcript=$false; $modelMayBeLoaded=$false; $qualificationStarted=$false; $monitor=$null
$temporaryTest=Resolve-NxDPath 'api/forensic_records/nxb21d_q4_qualification_test.go'
try {
    $cfgPath=Join-Path $PSScriptRoot 'nxb21d_q4_qualification_config.json'
    $cfg=Read-NxDStrictUtf8Text $cfgPath|ConvertFrom-Json
    Assert-NxQ4HoldoutUnconsumed (Resolve-NxDPath $cfg.corpus.path) (Resolve-NxDPath $cfg.consumed_holdout_registry)
    Assert-Framework $cfg
    Assert-Equal 'candidate_identity' (Get-CandidateIdentity $cfg) ([string]$cfg.candidate.identity_sha256)
    $publicReceipt=Resolve-NxDPath $cfg.public_receipt
    if (Test-Path -LiteralPath $publicReceipt) { throw 'HOLDOUT_ALREADY_CONSUMED:new Q4 qualification receipt exists' }
    $runRoot=Resolve-NxDPath $cfg.output_root; $run=Join-Path $runRoot ('qualification-'+[DateTime]::UtcNow.ToString('yyyyMMddTHHmmssfffZ'))
    New-Item -ItemType Directory -Force -Path $run|Out-Null
    Start-Transcript -LiteralPath (Join-Path $run 'operator.log') -Force|Out-Null; $transcript=$true
    Write-Host "NXB21D_Q4_FINAL_QUALIFICATION=START run=$run"

    $validation=& python (Resolve-NxDPath $cfg.corpus.validator) --repo $root --corpus (Resolve-NxDPath $cfg.corpus.path)
    if ($LASTEXITCODE -ne 0) { throw "CORPUS_VALIDATION_FAILED:$($validation -join ' ')" }
    Assert-Equal 'corpus' (Get-NxDHash (Resolve-NxDPath $cfg.corpus.path)) ([string]$cfg.corpus.sha256)
    Write-Host "INDEPENDENT_HOLDOUT_FROZEN=PASS cases=168 sha256=$($cfg.corpus.sha256)"

    $before=Assert-Baseline $cfg
    $artifactPath=Resolve-NxDPath $cfg.model.artifact_path
    Assert-Equal 'q4_artifact' (Get-NxDHash $artifactPath) ([string]$cfg.model.artifact_sha256)
    if ((Get-Item $artifactPath).Length -ne [int64]$cfg.model.artifact_bytes) { throw 'INTEGRITY_MISMATCH:q4_artifact_bytes' }
    Assert-Equal 'q4_profile_host' (Get-NxDHash (Resolve-NxDPath $cfg.model.profile_path)) ([string]$cfg.model.profile_sha256)
    Assert-Equal 'q4_artifact_container' ((Invoke-NxDNative docker @('exec',$cfg.runtime.container,'sha256sum',$cfg.model.artifact_container_path))-split'\s+')[0] ([string]$cfg.model.artifact_sha256)
    Assert-Equal 'q4_profile_container' ((Invoke-NxDNative docker @('exec',$cfg.runtime.container,'sha256sum',$cfg.model.profile_container_path))-split'\s+')[0] ([string]$cfg.model.profile_sha256)
    Assert-Equal 'q8_profile_container' ((Invoke-NxDNative docker @('exec',$cfg.runtime.container,'sha256sum',$cfg.model.q8_profile_container_path))-split'\s+')[0] ([string]$cfg.model.q8_profile_sha256)
    $models=@((Invoke-RestMethod -Uri ($cfg.runtime.localai_url+'/v1/models') -TimeoutSec 15).data.id)
    if ($models -cnotcontains [string]$cfg.model.id) { throw 'MODEL_MISSING:Q4 registration' }

    if ($PreparationDryRun) {
        $dryReceipt=Join-Path $run 'qualification-preparation-dry-run-v1.json'
        Write-NxDJson $dryReceipt ([ordered]@{schema_version='nexusai.nxb21d-q4-qualification-preparation/v1';status='PASS';candidate_identity_sha256=$cfg.candidate.identity_sha256;holdout_sha256=$cfg.corpus.sha256;baseline=[ordered]@{retained_tuple=$before.retained_tuple;activity_count=$before.activity_count;active_jobs=$before.active_jobs};runtime_mutated=$false;model_loaded_or_unloaded_by_dry_run=$false;holdout_consumed=$false})
        $exitCode=0
        Write-Host "NXB21D_Q4_QUALIFICATION_PREPARATION=PASS receipt=$dryReceipt sha256=$(Get-NxDHash $dryReceipt)"
        return
    }

    $qualificationStarted=$true
    if (Get-NxDLoadedState $cfg) { Invoke-NxDUnload $cfg; $modelMayBeLoaded=$false; Start-Sleep 3 }
    if (Get-NxDLoadedState $cfg) { throw 'MODEL_UNLOAD_FAILED:prequalification' }
    $null=Invoke-NxDSafeLinuxCleanCache
    $null=Wait-NxDRamAfterCleanCache $cfg 'immediately_before_q4_qualification' 180

    $manifest=[ordered]@{schema_version='nexusai.nxb21d-q4-final-qualification-manifest/v1';frozen_at=[DateTime]::UtcNow.ToString('o');candidate=$cfg.candidate;model=$cfg.model;corpus_sha256=$cfg.corpus.sha256;cases=168;language_counts=[ordered]@{en=65;ur=37;roman_ur=36;mixed=30};critical_cases=80;thresholds=$cfg.gates;framework_files=$cfg.framework_files;development_receipt=$cfg.development_receipt;holdout_may_be_used_for_tuning=$false}
    $manifestPath=Join-Path $run 'qualification-manifest-v1.json'; Write-NxDJson $manifestPath $manifest
    Write-Host "QUALIFICATION_CONTRACT_FROZEN=PASS manifest_sha256=$(Get-NxDHash $manifestPath)"

    if (Test-Path -LiteralPath $temporaryTest) { throw 'SOURCE_DRIFT:temporary evaluator exists' }
    Copy-Item -LiteralPath (Resolve-NxDPath $cfg.evaluator.path) -Destination $temporaryTest
    $resultPath=Join-Path $run 'q4-independent-qualification-results-v1.json'
    $resourcePath=Join-Path $run 'resource-samples.jsonl'
    $monitor=Start-Job -ArgumentList $resourcePath,$cfg.runtime.container -ScriptBlock {param($path,$container);while($true){try{$os=Get-CimInstance Win32_OperatingSystem;$ram=[math]::Round(([int64]$os.FreePhysicalMemory*1024)/1GB,3);$stats=& docker stats --no-stream --format '{{json .}}' $container 2>$null;(@{utc=[DateTime]::UtcNow.ToString('o');available_ram_gib=$ram;docker_stats=$stats}|ConvertTo-Json -Compress)|Add-Content -LiteralPath $path -Encoding UTF8}catch{};Start-Sleep 5}}
    $env:NXB21D_HOLDOUT=Resolve-NxDPath $cfg.corpus.path; $env:NXB21D_RESULT=$resultPath; $env:NXB21D_RUN_DIR=$run
    $env:NXB21D_LOCALAI_URL=$cfg.runtime.localai_url; $env:NXB21D_MODEL=$cfg.model.id
    $env:GOCACHE=Join-Path $run 'go-cache';$env:GOTMPDIR=Join-Path $run 'go-tmp';New-Item -ItemType Directory -Force $env:GOCACHE,$env:GOTMPDIR|Out-Null
    Write-Host 'HOLDOUT_CONSUMPTION=BEGIN one_shot=true independent=true'
    $modelMayBeLoaded=$true
    Invoke-NxDNativeStreaming go @('test','-v','./api/forensic_records','-run','TestForensicRecordsSynthesis','-count=1','-timeout','4h','-args','--ginkgo.focus=NX-B2[.]1D one-shot Q4 independent qualification holdout','--ginkgo.v','--ginkgo.timeout=4h') (Join-Path $run 'go-evaluator.log')
    if (-not (Test-Path -LiteralPath $resultPath)) { throw 'EVALUATION_INVALID:no result' }
    $result=Read-NxDStrictUtf8Text $resultPath|ConvertFrom-Json
    if (-not [bool]$result.holdout_consumed -or [int]$result.cases_completed -ne 168) { throw 'EVALUATION_INVALID:incomplete' }

    Invoke-NxDUnload $cfg; $modelMayBeLoaded=$false; Start-Sleep 3
    if (Get-NxDLoadedState $cfg) { throw 'MODEL_UNLOAD_FAILED:postqualification' }
    $after=Assert-Baseline $cfg
    $sanitized=[ordered]@{schema_version='nexusai.nxb21d-q4-final-qualification-receipt/v1';qualification_state='complete_pass';completed_at=[DateTime]::UtcNow.ToString('o');candidate=$cfg.candidate;model=$cfg.model;qualification_manifest_sha256=Get-NxDHash $manifestPath;holdout_sha256=$cfg.corpus.sha256;holdout_consumed=$true;cases_completed=168;model_calls=$result.model_calls;structured_valid=$result.structured_valid;overall=$result.overall;language=$result.language;critical=$result.critical;breakdowns=$result.breakdowns;latency=$result.latency;suitability=$result.suitability;DExitDecision=$result.D_exit_decision;runtime_postcheck='PASS';retained_tuple=$after.retained_tuple;activity_count=$after.activity_count;model_unloaded=$true;private_result_sha256=Get-NxDHash $resultPath;runtime_impact=[ordered]@{deployment=$false;database_mutation=$false;retained_evidence_mutation=$false;retained_activity_mutation=$false;model_download=$false}}
    Write-NxDJson $publicReceipt $sanitized; [IO.File]::WriteAllText($publicReceipt+'.sha256',(Get-NxDHash $publicReceipt)+'  '+(Split-Path $publicReceipt -Leaf)+"`n",[Text.UTF8Encoding]::new($false))
    if ($result.suitability -eq 'SUITABLE') {
        $sourceManifestPath=Resolve-NxDPath $cfg.activation.source_manifest
        $sourceManifest=Read-NxDStrictUtf8Text $sourceManifestPath|ConvertFrom-Json
        Assert-Equal 'activation_source_manifest' (Get-NxDHash $sourceManifestPath) ([string]$cfg.activation.source_manifest_sha256)
        Assert-Equal 'activation_source_digest' ([string]$sourceManifest.source_digest_sha256) ([string]$cfg.activation.source_digest_sha256)
        $sourceSealPath=Resolve-NxDPath $cfg.activation.source_seal
        Write-NxDJson $sourceSealPath ([ordered]@{schema_version='nexusai.nxb21d-q4-activation-source-seal/v1';sealed_at=[DateTime]::UtcNow.ToString('o');candidate_identity_sha256=$cfg.candidate.identity_sha256;D_evaluation_receipt_sha256=Get-NxDHash $publicReceipt;source_manifest_sha256=$cfg.activation.source_manifest_sha256;source_digest_sha256=$cfg.activation.source_digest_sha256;services=$cfg.activation.services;worker_required=$cfg.activation.worker_required})
        [IO.File]::WriteAllText($sourceSealPath+'.sha256',(Get-NxDHash $sourceSealPath)+'  '+(Split-Path $sourceSealPath -Leaf)+"`n",[Text.UTF8Encoding]::new($false))
        $exitCode=0; Write-Host 'NXB21D_Q4_FINAL_QUALIFICATION=PASS'
    }
    elseif ([double]$result.critical.percent -lt 100) { $exitCode=23; Write-Host 'NXB21D_Q4_FINAL_QUALIFICATION=CRITICAL_SAFETY_FAILURE' }
    else { $exitCode=22; Write-Host 'NXB21D_Q4_FINAL_QUALIFICATION=MODEL_INSUFFICIENT' }
    Write-Host "QUALIFICATION_RECEIPT=$publicReceipt"
    Write-Host "QUALIFICATION_RECEIPT_SHA256=$(Get-NxDHash $publicReceipt)"
    Write-Host 'EXACT_NEXT_ACTION=Reopen Codex and provide the qualification receipt. Do not run activation.'
} catch {
    $message=$_.Exception.Message
    if ($message -like 'RAM_TOO_LOW*') {$exitCode=10} elseif ($message -like '*DRIFT*' -or $message -like 'INTEGRITY_*' -or $message -like 'PREFLIGHT_*' -or $message -like 'CORPUS_*' -or $message -like 'HOLDOUT_ALREADY*') {$exitCode=11} elseif ($message -like 'ACTIVE_JOBS*' -or $message -like 'RUNTIME_*') {$exitCode=12} elseif ($message -like 'MODEL_*') {$exitCode=20}
    Write-Host "NXB21D_Q4_FINAL_QUALIFICATION=FAIL error=$message exit_code=$exitCode"
    Write-Host 'EXACT_NEXT_ACTION=Preserve the run directory and reopen Codex. Never restart the holdout automatically.'
} finally {
    if($monitor){Stop-Job $monitor -ErrorAction SilentlyContinue;Remove-Job $monitor -Force -ErrorAction SilentlyContinue}
    if(Test-Path -LiteralPath $temporaryTest){Remove-Item -LiteralPath $temporaryTest -Force}
    try{if($qualificationStarted-and$null-ne$cfg-and($modelMayBeLoaded-or(Get-NxDLoadedState $cfg))){Invoke-NxDUnload $cfg;Start-Sleep 3}}catch{Write-Host "SAFE_UNLOAD_ERROR=$($_.Exception.Message)"}
    Remove-Item Env:NXB21D_HOLDOUT,Env:NXB21D_RESULT,Env:NXB21D_RUN_DIR,Env:NXB21D_LOCALAI_URL,Env:NXB21D_MODEL,Env:GOCACHE,Env:GOTMPDIR -ErrorAction SilentlyContinue
    if($transcript){Stop-Transcript -ErrorAction SilentlyContinue|Out-Null}
}
exit $exitCode
