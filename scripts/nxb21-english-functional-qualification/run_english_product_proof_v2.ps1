# SPDX-License-Identifier: MIT
[CmdletBinding()]
param([switch]$ValidateOnly)
Set-StrictMode -Version Latest
$ErrorActionPreference='Stop'
$root=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$configPath=Join-Path $PSScriptRoot 'english-product-proof-v2-freeze.json'
$config=Get-Content -LiteralPath $configPath -Raw | ConvertFrom-Json
$library=Join-Path $PSScriptRoot 'run_current4b_final_characterization.ps1'
if((Get-FileHash -Algorithm SHA256 -LiteralPath $library).Hash.ToLowerInvariant() -cne 'ec0387052e0de7041799f869341000469211ccaaeda53dbf6bbcbbba53b2470b'){throw 'FROZEN_ENVELOPE_LIBRARY_DRIFT'}
$tokens=$null;$parseErrors=$null
$ast=[Management.Automation.Language.Parser]::ParseFile($library,[ref]$tokens,[ref]$parseErrors)
if($parseErrors.Count -ne 0){throw 'ENVELOPE_LIBRARY_PARSE_FAILED'}
foreach($definition in $ast.FindAll({param($node) $node -is [Management.Automation.Language.FunctionDefinitionAst]},$false)){. ([scriptblock]::Create($definition.Extent.Text))}
$privateRoot=Join-Path $root 'local-acceptance-models\nxb21-english-product-proof-v2-20260911'
$dispatchLock=Join-Path $privateRoot 'product-proof.dispatched.lock'
$executionLease=Join-Path $privateRoot 'operator.active'
$run=Join-Path $privateRoot ('run-'+[DateTime]::UtcNow.ToString('yyyyMMddTHHmmssfffZ')+'-'+[guid]::NewGuid().ToString('N').Substring(0,8))
$observationsPath=Join-Path $run 'runtime-observations.ndjson'
$receiptPath=Join-Path $run 'product-proof-receipt.json'
$before=$null;$after=$null;$modelOwned=$false;$dispatchCreated=$false;$lease=$null
$unloadState='NOT_REQUIRED';$integrityState='NOT_CHECKED';$finalState='PREFLIGHT_FAILURE_NO_INFERENCE';$errorMessage='';$exitCode=10
$pagingSince=$null;$baselineSwapKiB=$null;$experimentTimer=[Diagnostics.Stopwatch]::StartNew()
$requestCount=0;$loadCount=0;$reloadCount=0;$callResults=@();$observations=@();$child=$null
$identities='NOT_CHECKED';$memoryState='NOT_CHECKED';$results=$null

function Assert-V2Identity {
 if((Get-SHA256 $configPath) -cne (Get-Content -LiteralPath ($configPath+'.sha256') -Raw).Trim()){throw 'V2_FREEZE_DRIFT'}
 if([string]$config.proof_id -cne 'nxb21-english-product-proof-v2-20260911' -or [int]$config.cases -ne 48){throw 'V2_IDENTITY_MISMATCH'}
 if((Get-SHA256 $PSCommandPath) -cne [string]$config.runner_sha256){throw 'V2_RUNNER_DRIFT'}
 foreach($property in $config.files.PSObject.Properties){if((Get-SHA256 (Resolve-RepoPath $property.Name)) -cne [string]$property.Value){throw "V2_FILE_DRIFT:$($property.Name)"}}
 foreach($lockPath in @($dispatchLock,(Join-Path $privateRoot 'evaluator.dispatched.lock'))){if(Test-Path -LiteralPath $lockPath){throw 'V2_PROOF_ALREADY_CONSUMED'}}
 $branch=Invoke-Native git @('-C',$root,'branch','--show-current');$head=Invoke-Native git @('-C',$root,'rev-parse','HEAD')
 if($branch -cne [string]$config.branch -or $head -cne [string]$config.head){throw 'V2_SOURCE_HEAD_DRIFT'}
}
function Set-V2DiscoveryEnvironment {
 # Secrets stay in process memory and child environment, never in proof logs.
 $container=Invoke-Native docker @('inspect','nexusai-forensic-records-api-1') | ConvertFrom-Json
 foreach($entry in $container.Config.Env){
  $pair=$entry -split '=',2
  if($pair[0] -ceq 'FORENSIC_API_KEY'){$env:NXB21_V2_DISCOVERY_KEY=$pair[1]}
  if($pair[0] -ceq 'FORENSIC_TRUSTED_TENANT_ID'){$env:NXB21_V2_DISCOVERY_TENANT=$pair[1]}
 }
 if([string]::IsNullOrWhiteSpace($env:NXB21_V2_DISCOVERY_KEY) -or [string]::IsNullOrWhiteSpace($env:NXB21_V2_DISCOVERY_TENANT)){throw 'DISCOVERY_SCOPE_UNAVAILABLE'}
}
function Invoke-V2OfflineChecks {
 $binary=Resolve-RepoPath ([string]$config.evaluator)
 $env:NXB21_V2_PACKAGE=$PSScriptRoot
 $env:NXB21_V2_LIVE=$null
 $env:NXB21_V2_CONTEXT_OUT=Join-Path $run 'current-product-help-context.txt'
 & $binary '-test.run=^TestForensicRecordsSynthesis$' '-ginkgo.focus=English product proof V2' '-ginkgo.no-color' '-test.timeout=90s' *> (Join-Path $run 'offline-checks.log')
 if($LASTEXITCODE -ne 0){throw 'V2_OFFLINE_EVALUATOR_CHECK_FAILED'}
 if((Get-SHA256 $env:NXB21_V2_CONTEXT_OUT) -cne [string]$config.product_context_sha256){throw 'V2_PRODUCT_DISCOVERY_DRIFT'}
 $env:NXB21_V2_CONTEXT_OUT=$null
}
try {
 Assert-V2Identity
 New-Item -ItemType Directory -Force -Path $run | Out-Null
 # The transient lease prevents two unconsumed runners from admitting together.
 # It is not a proof-consumption marker and is removed only by its owner.
 $lease=[IO.File]::Open($executionLease,[IO.FileMode]::CreateNew,[IO.FileAccess]::Write,[IO.FileShare]::None)
 $before=Get-RuntimeState;Assert-RuntimeBaseline $before;Assert-ModelIdentity
 foreach($name in $before.containers.Keys){$current=$before.containers[$name];$accepted=$config.accepted_runtime.containers.$name;if($current.id -cne $accepted.id -or $current.image -cne $accepted.image -or $current.restart_count -ne $accepted.restart_count){throw "ACCEPTED_SERVICE_DRIFT:$name"}}
 if(($before.loaded_model_ids -join '|') -cne (@($config.accepted_runtime.loaded_model_ids) -join '|')){throw 'ACCEPTED_LOADED_MODEL_STATE_DRIFT'}
 if($null -ne (Get-LoadedState)){throw 'CURRENT_4B_MUST_START_UNLOADED'}
 Set-V2DiscoveryEnvironment
 Invoke-V2OfflineChecks
 $identities='PASS'
 $sample=Get-Telemetry 'preflight_no_inference' 0 0
 $memoryState=$(if([double]$sample.available_ram_gib -lt [double]$config.resource.admission_floor_gib){'OPERATOR_MEMORY_CLEANUP_REQUIRED'}else{'COLD_ADMISSION_STILL_REQUIRED_AT_DISPATCH'})
 if($ValidateOnly){$finalState='VALIDATE_ONLY_PASS_NO_INFERENCE';$exitCode=0}
 else {
  $finalState='RESOURCE_ADMISSION_FAILURE_NO_INFERENCE'
  Assert-ExperimentalSafety $sample
  Invoke-GovernedCleanCacheReclaim
  $null=Get-StableRAMWindow 'initial_preload' ([double]$config.resource.admission_floor_gib)
  $sample=Get-Telemetry 'admitted_before_dispatch' 0 0;Assert-ExperimentalSafety $sample
  # The permanent lock precedes the first model request, including warm-load.
  New-ExclusiveLock $dispatchLock ("proof_id=$($config.proof_id)`nfreeze_sha256=$(Get-SHA256 $configPath)`nstarted=$([DateTime]::UtcNow.ToString('o'))`n")
  $dispatchCreated=$true;$finalState='PROOF_RUNTIME_ABORT_CONSUMED'
  Start-OwnedModel $false $true
  $env:NXB21_V2_LIVE='1';$env:NXB21_V2_RUN=$run;$env:NXB21_V2_MODEL=[string]$config.model.id;$env:NXB21_V2_URL=[string]$config.runtime.localai_url;$env:NXB21_V2_CONTEXT_SHA=[string]$config.product_context_sha256
  $binary=Resolve-RepoPath ([string]$config.evaluator)
  $child=Start-Process -FilePath $binary -ArgumentList @('-test.run=^TestForensicRecordsSynthesis$','"-ginkgo.focus=English product proof V2 executes"','-ginkgo.v','-ginkgo.no-color','-test.timeout=20m') -WorkingDirectory $root -RedirectStandardOutput (Join-Path $run 'evaluator.stdout.log') -RedirectStandardError (Join-Path $run 'evaluator.stderr.log') -PassThru -WindowStyle Hidden
  $lastCount=-1
  while(-not $child.HasExited){
   Start-Sleep -Seconds 2
   $sample=Get-Telemetry 'proof_inflight' 0 1;Assert-ExperimentalSafety $sample
   $path=Join-Path $run 'product-proof-results.json'
   if(Test-Path -LiteralPath $path){try{$snapshot=Get-Content -LiteralPath $path -Raw | ConvertFrom-Json;$count=@($snapshot.results).Count;if($count -ne $lastCount){Write-Host "V2_COMPLETED_CASES=$count planned=48";$lastCount=$count}}catch{}}
   $child.Refresh()
  }
  $child.WaitForExit()
  $results=Get-Content -LiteralPath (Join-Path $run 'product-proof-results.json') -Raw | ConvertFrom-Json
  if(-not $results.complete -or @($results.results).Count -ne 48){throw 'V2_INCOMPLETE_CONSUMED'}
  if($results.automated_gate_pass -and $child.ExitCode -eq 0){$finalState='PROOF_COMPLETED_AUTOMATED_GATE_PASS_CONTENT_REVIEW_REQUIRED';$exitCode=0}
  else{$finalState='PROOF_COMPLETED_FUNCTIONAL_GATE_FAILED';$exitCode=20}
 }
} catch {$errorMessage=$_.Exception.Message;Write-Host "ERROR=$errorMessage"}
finally {
 if($null -ne $child -and -not $child.HasExited){$child.Kill();$child.WaitForExit()}
 try{Invoke-UnloadOwnedModel}catch{$unloadState='FAIL';$exitCode=40;$finalState='RUNTIME_INTEGRITY_FAILURE';$errorMessage=$_.Exception.Message}
 if($null -ne $before){try{$after=Get-RuntimeState;Assert-RuntimeUnchanged $before $after;$integrityState='PASS'}catch{$integrityState='FAIL';$exitCode=40;$finalState='RUNTIME_INTEGRITY_FAILURE';$errorMessage=$_.Exception.Message}}
 $receipt=[ordered]@{contract_version='nexusai.english-product-proof-receipt/v2';proof_id=[string]$config.proof_id;final_state=$finalState;error=$errorMessage;identities=$identities;memory_state=$memoryState;dispatch_created=$dispatchCreated;live_inference=$dispatchCreated;proof_consumed=(Test-Path -LiteralPath $dispatchLock);runtime_before=$before;runtime_after=$after;unload_state=$unloadState;runtime_integrity=$integrityState;content_review='REQUIRED';resource_track='CLOSED';nx_b21d='OPEN';strict_certification='PENDING';results=$results;exit_code=$exitCode}
 Write-JSON $receiptPath $receipt
 [IO.File]::WriteAllText($receiptPath+'.sha256',(Get-SHA256 $receiptPath)+"`n",[Text.UTF8Encoding]::new($false))
 foreach($name in @('NXB21_V2_LIVE','NXB21_V2_RUN','NXB21_V2_MODEL','NXB21_V2_URL','NXB21_V2_CONTEXT_SHA','NXB21_V2_CONTEXT_OUT','NXB21_V2_PACKAGE','NXB21_V2_DISCOVERY_KEY','NXB21_V2_DISCOVERY_TENANT')){[Environment]::SetEnvironmentVariable($name,$null,'Process')}
 if($null -ne $lease){$lease.Dispose();[IO.File]::Delete($executionLease)}
 Write-Host "V2_IDENTITIES=$identities";Write-Host "MEMORY_STATE=$memoryState";Write-Host "DISPATCH_CREATED=$dispatchCreated";Write-Host "CURRENT_4B_MODEL_UNLOAD=$unloadState";Write-Host "RUNTIME_INTEGRITY=$integrityState";Write-Host "PRODUCT_PROOF_RECEIPT=$receiptPath"
}
Write-Host "FINAL_STATE=$finalState"
exit $exitCode
