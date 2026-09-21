# SPDX-License-Identifier: MIT
[CmdletBinding()]
param([switch]$PreparationDryRun)
. (Join-Path $PSScriptRoot 'common.ps1')

$exitCode=21;$run=New-NxDRun 'evaluation';$log=Join-Path $run 'operator.log';$cfg=$null;$modelMayBeLoaded=$false;$monitor=$null;$temporaryTest=Resolve-NxDPath 'api/forensic_records/nxb21d_offline_evaluator_test.go'
Start-Transcript -LiteralPath $log -Force|Out-Null
try {
  $cfg=Get-NxDConfig
  Write-Host "NXB21D_EVALUATION=START run=$run dry_run=$PreparationDryRun"
  if(-not$PreparationDryRun){Assert-NxDHoldoutNotConsumed}
  try{$initialRam=Assert-NxDRam $cfg 'initial'}catch{if($_.Exception.Message-like'RAM_TOO_LOW:*'){$exitCode=10;Write-Host $_.Exception.Message;Write-Host 'EXACT_NEXT_ACTION=Close optional applications manually and rerun. Leave Docker Desktop running.';exit 10}else{throw}}
  $before=Assert-NxDBaseline $cfg
  if(Get-NxDLoadedState $cfg){throw 'BASELINE_DRIFT:Qwen is already loaded'}
  $artifact=Get-NxDModelArtifactReceipt $cfg $before.containers
  $cacheRecovered=Invoke-NxDSafeLinuxCleanCache
  if($PreparationDryRun){$preloadRam=Wait-NxDRamAfterCleanCache $cfg 'dry_run_immediately_before_model_load' 60;Write-NxDJson (Join-Path $run 'dry-run-receipt.json') ([ordered]@{status='PASS';initial_ram_gib=$initialRam;preload_ram_gib=$preloadRam;source_digest=$before.source_digest;clean_file_cache_recovered=$cacheRecovered;runtime_mutated=$false;holdout_consumed=$false});$exitCode=0;Write-Host 'NXB21D_EVALUATION_DRY_RUN=PASS';return}

  $contractFiles=@($cfg.holdout,$cfg.capability_contract,'api/forensic_records/query_language_assistance.go','api/forensic_records/d_dynamic_planner.go','api/forensic_records/query_intelligence_contracts.go','scripts/nxb21d-offline/d_model_evaluator_test.go','scripts/nxb21d-offline/nxb21d_operator_config.json')
  $hashes=[ordered]@{};foreach($f in $contractFiles){$hashes[$f]=Get-NxDHash (Resolve-NxDPath $f)}
  $manifest=[ordered]@{schema_version='nexusai.nxb21d-model-evaluation-manifest/v1';frozen_at=[DateTime]::UtcNow.ToString('o');source_digest=$before.source_digest;holdout_cases=168;language_counts=@{english=65;urdu=37;roman_urdu=36;mixed=30};files=$hashes;model=$cfg.model;model_artifact=$artifact;thresholds=$cfg.gates;holdout_may_be_used_for_tuning=$false}
  $manifestPath=Join-Path $run 'd-model-evaluation-manifest-v1.json';Write-NxDJson $manifestPath $manifest;$manifestHash=Get-NxDHash $manifestPath
  Write-Host "EVALUATION_CONTRACT_FROZEN=PASS manifest_sha256=$manifestHash holdout_sha256=$($hashes[$cfg.holdout])"
  $null=Wait-NxDRamAfterCleanCache $cfg 'immediately_before_model_load' 60

  if(Test-Path -LiteralPath $temporaryTest){throw 'SOURCE_DRIFT:temporary evaluator path already exists'}
  Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'd_model_evaluator_test.go') -Destination $temporaryTest
  $resultPath=Join-Path $run 'd-qwen-168-holdout-results-v1.json'
  $resourcePath=Join-Path $run 'resource-samples.jsonl'
  $monitor=Start-Job -ArgumentList $resourcePath,$cfg.runtime.localai_url,$cfg.model.id -ScriptBlock {param($path,$url,$model);while($true){try{$os=Get-CimInstance Win32_OperatingSystem;$ram=[math]::Round(([int64]$os.FreePhysicalMemory*1024)/1GB,3);$stats=(& docker stats --no-stream --format '{{json .}}' nexusai-api-1 2>$null);$backend=$null;try{$backend=Invoke-RestMethod -Uri ($url+'/backend/monitor?model='+[uri]::EscapeDataString($model))-TimeoutSec 5}catch{};(@{utc=[DateTime]::UtcNow.ToString('o');available_ram_gib=$ram;docker_stats=$stats;backend=$backend}|ConvertTo-Json -Compress -Depth 20)|Add-Content -LiteralPath $path -Encoding UTF8}catch{};Start-Sleep -Seconds 5}}
  $env:NXB21D_HOLDOUT=Resolve-NxDPath $cfg.holdout;$env:NXB21D_RESULT=$resultPath;$env:NXB21D_LOCALAI_URL=$cfg.runtime.localai_url;$env:NXB21D_MODEL=$cfg.model.id
  $env:GOCACHE=Join-Path $run 'go-cache';$env:GOTMPDIR=Join-Path $run 'go-tmp';New-Item -ItemType Directory -Force -Path $env:GOCACHE,$env:GOTMPDIR|Out-Null
  Write-Host 'HOLDOUT_CONSUMPTION=BEGIN one_shot=true'
  $modelMayBeLoaded=$true
  Invoke-NxDNativeStreaming go @('test','-v','./api/forensic_records','-run','TestForensicRecordsSynthesis','-ginkgo.focus','NX-B2.1D one-shot current-model holdout','-ginkgo.v','-ginkgo.timeout=4h','-count=1','-timeout','4h') (Join-Path $run 'go-evaluator.log')
  if(-not(Test-Path -LiteralPath $resultPath)){throw 'EVALUATION_FAILED:no result receipt'}
  $result=Get-Content -LiteralPath $resultPath -Raw|ConvertFrom-Json
  if([int]$result.cases_completed-ne 168-or-not[bool]$result.holdout_consumed){throw 'EVALUATION_INVALID:holdout incomplete'}
  Write-Host "QWEN_SUITABILITY=$($result.suitability) critical_percent=$($result.critical.percent) overall_percent=$($result.overall.percent)"

  if($modelMayBeLoaded-or(Get-NxDLoadedState $cfg)){Invoke-NxDUnload $cfg;$modelMayBeLoaded=$false};Start-Sleep -Seconds 2
  if(Get-NxDLoadedState $cfg){$exitCode=24;throw 'MODEL_UNLOAD_FAILED'}
  $after=Assert-NxDBaseline $cfg
  if($after.retained_tuple-cne$before.retained_tuple-or$after.activity_count-ne$before.activity_count){$exitCode=30;throw 'POSTCHECK_FAILED:retained state changed'}
  $samples=@();if(Test-Path $resourcePath){$samples=@(Get-Content $resourcePath|ForEach-Object{$_|ConvertFrom-Json})}
  $minRam=if($samples.Count){[double](($samples|Measure-Object available_ram_gib -Minimum).Minimum)}else{$null}
  $sanitized=[ordered]@{schema_version='nexusai.nxb21d-model-evaluation-receipt/v1';completed_at=[DateTime]::UtcNow.ToString('o');source_digest=$before.source_digest;evaluation_manifest_sha256=$manifestHash;holdout_sha256=$hashes[$cfg.holdout];model=$cfg.model;holdout_consumed=$true;cases_completed=168;overall=$result.overall;language=$result.language;structured_valid=$result.structured_valid;critical=$result.critical;breakdowns=$result.breakdowns;latency=$result.latency;suitability=$result.suitability;DExitDecision=$result.D_exit_decision;runtime_postcheck='PASS';retained_tuple=$after.retained_tuple;activity_count_unchanged=$true;model_unloaded=$true;resources=@{sample_count=$samples.Count;minimum_available_ram_gib=$minRam;clean_file_cache_recovered=$cacheRecovered;detailed_samples_private=$true};new_model_downloaded=$false;deployment_performed=$false}
  $public=Resolve-NxDPath 'reports/nxb21/d-model-evaluation-receipt-v1.json';Write-NxDJson $public $sanitized;[IO.File]::WriteAllText($public+'.sha256',(Get-NxDHash $public)+"  d-model-evaluation-receipt-v1.json`n",[Text.UTF8Encoding]::new($false))
  if($result.suitability-eq'SUITABLE'){$sourceManifest=Get-Content -LiteralPath (Resolve-NxDPath $cfg.d_source_manifest) -Raw|ConvertFrom-Json;Write-NxDJson (Resolve-NxDPath 'reports/nxb21/d-activation-source-seal-v1.json') ([ordered]@{schema_version='nexusai.nxb21d-activation-source-seal/v1';sealed_at=[DateTime]::UtcNow.ToString('o');D_evaluation_receipt_sha256=(Get-NxDHash $public);source_digest=$before.source_digest;file_count=$sourceManifest.file_count;files=$sourceManifest.files;services=@('api','forensic-records-api');worker_required=$false})}
  Write-NxDJson (Join-Path $run 'd-qwen-model-fitness-v1.json') $sanitized
  if($result.suitability-eq'SUITABLE'){$exitCode=0;Write-Host 'NXB21D_EVALUATION=PASS'}elseif([double]$result.critical.percent-lt 100){$exitCode=23;Write-Host 'NXB21D_EVALUATION=CRITICAL_SAFETY_FAILURE'}else{$exitCode=22;Write-Host 'NXB21D_EVALUATION=MODEL_INSUFFICIENT'}
  Write-Host 'EXACT_NEXT_ACTION=Reopen Codex and provide reports/nxb21/d-model-evaluation-receipt-v1.json for review. Do not run activation without approval.'
} catch {
  Write-Host "NXB21D_EVALUATION=FAIL error=$($_.Exception.Message)"
  if($exitCode-eq21){if($_.Exception.Message-like'RAM_TOO_LOW:*'){$exitCode=10}elseif($_.Exception.Message-like'SOURCE_DRIFT*'-or$_.Exception.Message-like'BASELINE_DRIFT*'-or$_.Exception.Message-like'RUNTIME_DRIFT*'-or$_.Exception.Message-like'RETAINED_DRIFT*'){$exitCode=11}elseif($_.Exception.Message-like'ACTIVE_JOBS*'-or$_.Exception.Message-like'RUNTIME_UNHEALTHY*'-or$_.Exception.Message-like'RUNTIME_HTTP_UNHEALTHY*'){$exitCode=12}elseif($_.Exception.Message-like'MODEL_*'){$exitCode=20}}
  Write-Host 'EXACT_NEXT_ACTION=Preserve this run directory, reopen Codex, and report the error. Do not retry the holdout automatically.'
} finally {
  if($monitor){Stop-Job $monitor -ErrorAction SilentlyContinue;Remove-Job $monitor -Force -ErrorAction SilentlyContinue}
  if(Test-Path -LiteralPath $temporaryTest){Remove-Item -LiteralPath $temporaryTest -Force}
  try{
    if($null-ne$cfg-and$modelMayBeLoaded){Invoke-NxDUnload $cfg;Start-Sleep -Seconds 2}
    if($null-ne$cfg-and(Get-NxDLoadedState $cfg)){Write-Host 'SAFE_UNLOAD_ERROR=MODEL_UNLOAD_FAILED: model still present in /system loaded_models'}
  }catch{Write-Host "SAFE_UNLOAD_ERROR=$($_.Exception.Message)"}
  Remove-Item Env:NXB21D_HOLDOUT,Env:NXB21D_RESULT,Env:NXB21D_LOCALAI_URL,Env:NXB21D_MODEL,Env:GOCACHE,Env:GOTMPDIR -ErrorAction SilentlyContinue
  Stop-Transcript -ErrorAction SilentlyContinue|Out-Null
  $global:LASTEXITCODE=$exitCode
}
exit $exitCode
