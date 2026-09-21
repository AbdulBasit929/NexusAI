# SPDX-License-Identifier: MIT
[CmdletBinding()]
param([switch]$ValidateOnly)
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$root = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..\..'))
. (Join-Path $root 'scripts\nxb21d-offline\common.ps1')
. (Join-Path $root 'scripts\nxb21d-development\byte_safe_transport.ps1')
. (Join-Path $root 'scripts\nxb21d-hybrid-development\immutable_receipt.ps1')

function Assert-Equal([string]$Name, [string]$Actual, [string]$Expected) {
    if ($Actual -cne $Expected) { throw "INTEGRITY_MISMATCH:$Name expected=$Expected actual=$Actual" }
}
function Get-LoadedModels($Config) {
    return @((Invoke-RestMethod -Uri ($Config.runtime.localai_url + '/system') -TimeoutSec 15).loaded_models.id | Sort-Object)
}
function Get-LiveIndexSHA($Config) {
    $client = New-Object Net.WebClient
    try { $client.Headers['Accept'] = 'text/html'; return Get-NxDBytesSHA256 ($client.DownloadData($Config.runtime.localai_url + '/analyst')) }
    finally { $client.Dispose() }
}
function Assert-Runtime($Config) {
    $containers = Get-NxDContainers
    foreach ($entry in $Config.runtime.containers.PSObject.Properties) {
        $actual = $containers[$entry.Name]; $expected = $entry.Value
        if ($null -eq $actual -or -not (Test-NxDContainerHealthy $actual)) { throw "RUNTIME_UNHEALTHY:$($entry.Name)" }
        Assert-Equal "$($entry.Name).id" ([string]$actual.Id) ([string]$expected.id)
        Assert-Equal "$($entry.Name).image" ([string]$actual.Image) ([string]$expected.image)
        if ([int]$actual.RestartCount -ne [int]$expected.restarts) { throw "RUNTIME_DRIFT:$($entry.Name).restarts" }
    }
    Assert-NxDHTTPHealth $Config
    $jobs = Get-NxDActiveJobs $containers; if ($jobs -ne 0) { throw "ACTIVE_JOBS:$jobs" }
    $tuple = Get-NxDRetainedTuple $containers; Assert-Equal 'retained_tuple' $tuple ([string]$Config.runtime.retained_tuple)
    $activity = Get-NxDActivityCount $containers; if ($activity -ne [int]$Config.runtime.activity_count) { throw "RETAINED_ACTIVITY_DRIFT:$activity" }
    $loaded = @(Get-LoadedModels $Config)
    if ($loaded -contains [string]$Config.model.id) { throw 'LOWMEM_MODEL_ALREADY_LOADED' }
    if (@(Compare-Object @($Config.runtime.loaded_models) $loaded).Count -ne 0) { throw 'LOADED_MODEL_BASELINE_DRIFT' }
    Assert-Equal 'analyst_index' (Get-LiveIndexSHA $Config) ([string]$Config.runtime.analyst_index_sha256)
    return [pscustomobject]@{containers=$containers;retained_tuple=$tuple;activity_count=$activity;active_jobs=$jobs;loaded_models=$loaded}
}
function Assert-Frozen($Config) {
    foreach ($entry in $Config.frozen_files.PSObject.Properties) {
        $path = Resolve-NxDPath $entry.Name
        if (-not (Test-Path -LiteralPath $path -PathType Leaf)) { throw "PREFLIGHT_MISSING:$($entry.Name)" }
        Assert-Equal "file:$($entry.Name)" (Get-NxDHash $path) ([string]$entry.Value)
    }
    $artifact = Resolve-NxDPath $Config.model.artifact_path
    if ((Get-Item -LiteralPath $artifact).Length -ne [int64]$Config.model.artifact_bytes) { throw 'ARTIFACT_BYTES_MISMATCH' }
    Assert-Equal 'artifact' (Get-NxDHash $artifact) ([string]$Config.model.artifact_sha256)
    Assert-Equal 'profile' (Get-NxDHash (Resolve-NxDPath $Config.model.profile_path)) ([string]$Config.model.profile_sha256)
    Assert-Equal 'corpus' (Get-NxDHash (Resolve-NxDPath $Config.corpus.path)) ([string]$Config.corpus.sha256)
    Assert-Equal 'bindings' (Get-NxDHash (Resolve-NxDPath $Config.corpus.bindings_path)) ([string]$Config.corpus.bindings_sha256)
    Assert-Equal 'freshness' (Get-NxDHash (Resolve-NxDPath $Config.corpus.freshness_path)) ([string]$Config.corpus.freshness_sha256)
    Assert-Equal 'prebuilt_evaluator' (Get-NxDHash (Resolve-NxDPath $Config.evaluator.path)) ([string]$Config.evaluator.sha256)
    $validation = & python (Resolve-NxDPath $Config.corpus.validator) --repo $root --corpus (Resolve-NxDPath $Config.corpus.path)
    if ($LASTEXITCODE -ne 0) { throw "CORPUS_VALIDATION_FAILED:$($validation -join ' ')" }
    $freshness = Read-NxDStrictUtf8Text (Resolve-NxDPath $Config.corpus.freshness_path) | ConvertFrom-Json
    if ($freshness.status -cne 'PASS' -or [int]$freshness.question_overlap -ne 0 -or [int]$freshness.target_value_overlap -ne 0 -or [int]$freshness.normalized_literal_overlap -ne 0) { throw 'FRESHNESS_RECEIPT_INVALID' }
}
function Assert-StablePreload($Config) {
    $deadline = [DateTime]::UtcNow.AddSeconds([int]$Config.resource.preload_settling_limit_seconds)
    $consecutive = 0; $samples = @(); $recoveryAttempted = $false
    do {
        $available = Get-NxDRamGiB
        if ($available -ge [double]$Config.resource.minimum_before_load_gib) { $consecutive++ } else { $consecutive = 0 }
        $sample = [pscustomobject][ordered]@{ordinal=$samples.Count+1;available_gib=$available;required_gib=[double]$Config.resource.minimum_before_load_gib;consecutive=$consecutive;captured_at_utc=[DateTime]::UtcNow.ToString('o')}
        $samples += $sample
        Write-Host "PRELOAD_STABLE_SAMPLE_$($sample.ordinal) available_gib=$available required_gib=$($sample.required_gib) consecutive=$consecutive"
        if ($consecutive -ge [int]$Config.resource.preload_consecutive_samples) { return [pscustomobject]@{passed=$true;samples=$samples;preload_ram_gib=$available} }
        if (-not $recoveryAttempted -and $available -lt [double]$Config.resource.minimum_before_load_gib) { $null=Invoke-NxDSafeLinuxCleanCache; $recoveryAttempted=$true }
        if ([DateTime]::UtcNow -lt $deadline) { Start-Sleep -Seconds ([int]$Config.resource.preload_sample_interval_seconds) }
    } while ([DateTime]::UtcNow -lt $deadline)
    return [pscustomobject]@{passed=$false;samples=$samples;preload_ram_gib=$samples[-1].available_gib}
}
function Get-EffectiveSettings($Config, [datetime]$Since) {
    $raw = Invoke-NxDNative docker @('logs','--since',$Since.ToUniversalTime().ToString('o'),$Config.runtime.container)
    $clean = [regex]::Replace($raw, [char]27 + '\[[0-9;?]*[ -/]*[@-~]', '')
    $line = @($clean -split '\r?\n' | Where-Object { $_ -like "*effective runtime tuning*modelID=`"$($Config.model.id)`"*" } | Select-Object -Last 1)
    if ($line.Count -ne 1 -or $line[0] -notmatch 'context=([0-9]+).*n_batch=([0-9]+).*n_gpu_layers=([0-9]+).*parallel="([0-9]+)"') { throw 'EFFECTIVE_RUNTIME_SETTINGS_INVALID' }
    return [pscustomobject][ordered]@{context=[int]$Matches[1];requested_batch=[int]$Config.model.requested_batch;effective_n_batch=[int]$Matches[2];threads=[int]$Config.model.threads;parallel=[int]$Matches[4];gpu_layers=[int]$Matches[3];raw_log_line=$line[0]}
}
function Invoke-PrebuiltEvaluator([string]$Path,[string]$LogPath) {
    Push-Location $root; $code=1
    try {
        & $Path '-test.v' '-test.run=TestForensicRecordsSynthesis' '-test.timeout=2h' '--ginkgo.focus=NXB21D final controlled local Qwen3 8B selection12 standalone development' '--ginkgo.v' '--ginkgo.timeout=2h' 2>&1 | Tee-Object -FilePath $LogPath | ForEach-Object { Write-Host $_ }
        $code=$LASTEXITCODE
    } finally { Pop-Location }
    if ($code -ne 0) { throw "PREBUILT_EVALUATOR_FAILED:$code" }
}

$exitCode=21; $cfg=$null; $run=$null; $summaryPath=$null; $intermediatePath=$null; $result=$null; $modelMayBeLoaded=$false; $after=$null; $effective=$null; $postcheck='NOT_RUN'; $transcript=$false; $stable=$null; $preflightFailure=$false
try {
    $configPath=Join-Path $PSScriptRoot 'final-local-selection12-config.json'
    Assert-Equal 'operator_config' (Get-NxDHash $configPath) ((Read-NxDStrictUtf8Text ($configPath+'.sha256')).Trim().Split(' ')[0])
    $cfg=Read-NxDStrictUtf8Text $configPath | ConvertFrom-Json
    $runtime=Read-NxDStrictUtf8Text (Resolve-NxDPath $cfg.runtime_path) | ConvertFrom-Json
    $cfg | Add-Member -Force -NotePropertyName runtime -NotePropertyValue $runtime
    Assert-Frozen $cfg
    $before=Assert-Runtime $cfg
    if ($ValidateOnly) {
        Write-Host "FRESH_GATE_ID=$($cfg.gate_id)"; Write-Host "FRESH_CORPUS_SHA256=$($cfg.corpus.sha256)"; Write-Host "FRESHNESS_FILES_SCANNED=$($cfg.corpus.files_scanned)"
        Write-Host "CURRENT_RETAINED_TUPLE=$($before.retained_tuple)"; Write-Host "CURRENT_ACTIVITY_COUNT=$($before.activity_count)"; Write-Host 'ACTIVE_JOBS=0'
        Write-Host 'FINAL_LOCAL_SELECTION12_VALIDATE_ONLY=PASS live_inference=false model_load=false'; exit 0
    }
    $runRoot=Resolve-NxDPath $cfg.output_root; $lock=Join-Path $runRoot 'development-dispatched.lock'
    if (Test-Path -LiteralPath $lock) { throw 'FINAL_LOCAL_SELECTION12_ALREADY_DISPATCHED' }
    $stable=Assert-StablePreload $cfg
    if (-not $stable.passed) { $preflightFailure=$true; $exitCode=10; throw 'RESOURCE_PREFLIGHT_FAIL' }
    $run=Join-Path $runRoot ('run-'+[DateTime]::UtcNow.ToString('yyyyMMddTHHmmssfffZ')); New-Item -ItemType Directory -Force -Path $run | Out-Null
    Start-Transcript -LiteralPath (Join-Path $run 'operator.log') -Force | Out-Null; $transcript=$true
    $summaryPath=Join-Path $run 'final-local-selection12-development-summary-v1.json'; $intermediatePath=Join-Path $run 'selection12-development-intermediate-v1.json'; $started=[DateTime]::UtcNow
    $env:NXB21D_FINAL_LOCAL12_LIVE='1'; $env:NXB21D_FINAL_LOCAL12_RUN_DIR=$run; $env:NXB21D_FINAL_LOCAL12_CORPUS=Resolve-NxDPath $cfg.corpus.path
    $env:NXB21D_FINAL_LOCAL12_BINDINGS=Resolve-NxDPath $cfg.corpus.bindings_path; $env:NXB21D_FINAL_LOCAL12_RAM_SCRIPT=Resolve-NxDPath $cfg.resource.measure_script
    $env:NXB21D_FINAL_LOCAL12_GOVERNED_RAM_SCRIPT=Resolve-NxDPath $cfg.resource.governed_ram_script; $env:NXB21D_FINAL_LOCAL12_DISPATCH_LOCK=$lock
    $env:NXB21D_LOCALAI_URL=$cfg.runtime.localai_url; $env:NXB21D_MODEL=$cfg.model.id
    $modelMayBeLoaded=$true
    Write-Host "NXB21D_FINAL_LOCAL_SELECTION12=START run=$run preload_ram_gib=$($stable.preload_ram_gib)"
    Invoke-PrebuiltEvaluator (Resolve-NxDPath $cfg.evaluator.path) (Join-Path $run 'prebuilt-evaluator.log')
    if (-not (Test-Path -LiteralPath $intermediatePath)) { throw 'DEVELOPMENT_INVALID:no aggregate result' }
    $result=Read-NxDStrictUtf8Text $intermediatePath | ConvertFrom-Json; $effective=Get-EffectiveSettings $cfg $started
    Write-Host "EFFECTIVE_RUNTIME context=$($effective.context) requested_batch=$($effective.requested_batch) n_batch=$($effective.effective_n_batch) threads=$($effective.threads) parallel=$($effective.parallel) gpu_layers=$($effective.gpu_layers)"
    Invoke-NxDUnload $cfg; Start-Sleep 3; if (Get-NxDLoadedState $cfg) { throw 'MODEL_UNLOAD_FAILED' }; $modelMayBeLoaded=$false
    $after=Assert-Runtime $cfg; $postcheck='PASS'; $counts=$result.acceptance_counts
    $accepted=[int]$result.cases_completed -eq 12 -and [int]$result.model_calls -eq 12 -and [int]$result.raw_tuple_correct -eq 12 -and [int]$result.passed -eq 12
    foreach($name in @('http_success','strict_utf8','schema_valid','finish_reason_stop')) { $accepted = $accepted -and [int]$counts.$name -eq 12 }
    foreach($name in @('malformed','unknown_decision','truncation','fact_override','scope_override','authorization_override')) { $accepted = $accepted -and [int]$counts.$name -eq 0 }
    $exitCode = if ($accepted) { 0 } else { 22 }
} catch {
    Write-Host "NXB21D_FINAL_LOCAL_SELECTION12=FAIL error=$($_.Exception.Message)"
    if ($preflightFailure) { Write-Host 'RESOURCE_PREFLIGHT_FAIL'; Write-Host 'CORPUS_STATE=NOT_CONSUMED'; Write-Host 'QWEN3_8B_MODEL_QUALITY=INSUFFICIENT_EVIDENCE' }
} finally {
    Remove-Item Env:NXB21D_FINAL_LOCAL12_LIVE,Env:NXB21D_FINAL_LOCAL12_RUN_DIR,Env:NXB21D_FINAL_LOCAL12_CORPUS,Env:NXB21D_FINAL_LOCAL12_BINDINGS,Env:NXB21D_FINAL_LOCAL12_RAM_SCRIPT,Env:NXB21D_FINAL_LOCAL12_GOVERNED_RAM_SCRIPT,Env:NXB21D_FINAL_LOCAL12_DISPATCH_LOCK,Env:NXB21D_LOCALAI_URL,Env:NXB21D_MODEL -ErrorAction SilentlyContinue
    try { if ($null -ne $cfg -and $modelMayBeLoaded) { Invoke-NxDUnload $cfg; Start-Sleep 3; $modelMayBeLoaded=[bool](Get-NxDLoadedState $cfg) } } catch { Write-Host "SAFE_UNLOAD_ERROR=$($_.Exception.Message)"; $exitCode=20 }
    if ($run) {
        try { if ($postcheck-ne'PASS') { $after=Assert-Runtime $cfg; $postcheck='PASS' } } catch { $postcheck='FAIL'; $exitCode=12; Write-Host "RUNTIME_POSTCHECK_ERROR=$($_.Exception.Message)" }
        try {
            if ($null -eq $result) { $result=if(Test-Path -LiteralPath $intermediatePath){Read-NxDStrictUtf8Text $intermediatePath|ConvertFrom-Json}else{[pscustomobject]@{development_gate='FAIL';cases_completed=0}} }
            if ($exitCode -ne 0) { $result|Add-Member -Force -NotePropertyName development_gate -NotePropertyValue 'FAIL' }
            $dispatched=Test-Path -LiteralPath $lock
            $resourceFailure=$dispatched -and @($result.results|Where-Object{$_.state -in @('RAM_GATE_FAILED','MODEL_LOADED_RAM_GATE_FAILED')}).Count -gt 0
            $result|Add-Member -Force -NotePropertyName gate_id -NotePropertyValue $cfg.gate_id
            $result|Add-Member -Force -NotePropertyName corpus_state -NotePropertyValue $(if($dispatched){'CONSUMED'}else{'NOT_CONSUMED'})
            $result|Add-Member -Force -NotePropertyName preload_ram_gib -NotePropertyValue $stable.preload_ram_gib
            $result|Add-Member -Force -NotePropertyName preload_stable_samples -NotePropertyValue $stable.samples
            $result|Add-Member -Force -NotePropertyName effective_runtime -NotePropertyValue $effective
            $result|Add-Member -Force -NotePropertyName runtime_postcheck -NotePropertyValue $postcheck
            $result|Add-Member -Force -NotePropertyName model_unload -NotePropertyValue $(if (-not $modelMayBeLoaded) {'PASS'} else {'FAIL'})
            $result|Add-Member -Force -NotePropertyName retained_tuple -NotePropertyValue $(if ($null -ne $after) {$after.retained_tuple} else {$null})
            $result|Add-Member -Force -NotePropertyName activity_count -NotePropertyValue $(if ($null -ne $after) {$after.activity_count} else {$null})
            $result|Add-Member -Force -NotePropertyName active_jobs -NotePropertyValue $(if ($null -ne $after) {$after.active_jobs} else {$null})
            $result|Add-Member -Force -NotePropertyName local_qwen3_8b_final_attempt -NotePropertyValue $(if ($exitCode -eq 0) {'PASS'} elseif ($resourceFailure) {'RESOURCE_FAILURE'} else {'FAIL'})
            $result|Add-Member -Force -NotePropertyName local_qwen3_8b_path -NotePropertyValue $(if ($resourceFailure) {'CLOSED_RESOURCE_INSUFFICIENT'} else {'FINAL_ATTEMPT_CONSUMED'})
            $result|Add-Member -Force -NotePropertyName D_status -NotePropertyValue 'OPEN'; $result|Add-Member -Force -NotePropertyName activation -NotePropertyValue 'BLOCKED'
            $seal=Seal-NxHybridReceipt $summaryPath $result; Write-NxHybridReceiptOutput $seal
            Write-Host $(if ($exitCode -eq 0) {'QWEN3_8B_MODEL_SELECTION=PASS'} else {'QWEN3_8B_MODEL_SELECTION=FAIL'})
        } catch { Write-Host "FINAL_RECEIPT_SEAL_FAILED=$($_.Exception.Message)"; $exitCode=21 }
    }
    if($transcript){Stop-Transcript -ErrorAction SilentlyContinue|Out-Null}
}
exit $exitCode
