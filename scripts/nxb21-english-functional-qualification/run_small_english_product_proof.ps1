# SPDX-License-Identifier: MIT
[CmdletBinding()]
param([switch]$ValidateOnly)
Set-StrictMode -Version Latest
$ErrorActionPreference='Stop'
$root=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$configPath=Join-Path $PSScriptRoot 'small-english-product-freeze-v1.json'
$config=Get-Content -LiteralPath $configPath -Raw | ConvertFrom-Json
$library=Join-Path $PSScriptRoot 'run_current4b_final_characterization.ps1'
if((Get-FileHash -Algorithm SHA256 -LiteralPath $library).Hash.ToLowerInvariant() -cne 'ec0387052e0de7041799f869341000469211ccaaeda53dbf6bbcbbba53b2470b'){throw 'FROZEN_ENVELOPE_LIBRARY_DRIFT'}
# Definitions only: importing the measured guard does not rerun characterization.
$tokens=$null;$parseErrors=$null
$ast=[Management.Automation.Language.Parser]::ParseFile($library,[ref]$tokens,[ref]$parseErrors)
if($parseErrors.Count -ne 0){throw 'ENVELOPE_LIBRARY_PARSE_FAILED'}
foreach($definition in $ast.FindAll({param($node) $node -is [Management.Automation.Language.FunctionDefinitionAst]},$false)){
    . ([scriptblock]::Create($definition.Extent.Text))
}
$privateRoot=Join-Path $root 'local-acceptance-models\nxb21-small-english-product-proof-v1'
$dispatchLock=Join-Path $privateRoot 'product-proof.dispatched.lock'
$run=Join-Path $privateRoot ('run-'+[DateTime]::UtcNow.ToString('yyyyMMddTHHmmssfffZ'))
$observationsPath=Join-Path $run 'runtime-observations.ndjson'
$receiptPath=Join-Path $run 'product-proof-receipt.json'
$before=$null;$after=$null;$modelOwned=$false;$dispatchCreated=$false
$unloadState='NOT_REQUIRED';$integrityState='NOT_CHECKED';$finalState='PREFLIGHT';$errorMessage='';$exitCode=10
$pagingSince=$null;$baselineSwapKiB=$null;$experimentTimer=[Diagnostics.Stopwatch]::StartNew()
$requestCount=0;$loadCount=0;$reloadCount=0;$callResults=@();$observations=@();$child=$null

function Assert-ProofFiles {
    if((Get-SHA256 $configPath) -cne (Get-Content -LiteralPath ($configPath+'.sha256') -Raw).Trim()){throw 'PROOF_FREEZE_DRIFT'}
    if((Get-SHA256 $PSCommandPath) -cne [string]$config.runner_sha256){throw 'PROOF_RUNNER_DRIFT'}
    foreach($property in $config.files.PSObject.Properties){
        if((Get-SHA256 (Resolve-RepoPath $property.Name)) -cne [string]$property.Value){throw "PROOF_FILE_DRIFT:$($property.Name)"}
    }
    if(Test-Path -LiteralPath $dispatchLock){throw 'SMALL_ENGLISH_PRODUCT_PROOF_ALREADY_CONSUMED'}
}

try {
    Assert-ProofFiles
    $before=Get-RuntimeState;Assert-RuntimeBaseline $before;Assert-ModelIdentity
    foreach($name in $before.containers.Keys){
        $current=$before.containers[$name];$accepted=$config.accepted_runtime.containers.$name
        if($current.id -cne $accepted.id -or $current.image -cne $accepted.image -or $current.restart_count -ne $accepted.restart_count){throw "ACCEPTED_SERVICE_DRIFT:$name"}
    }
    if($null -ne (Get-LoadedState)){throw 'CURRENT_4B_MUST_START_UNLOADED'}
    New-Item -ItemType Directory -Force -Path $run | Out-Null
    $sample=Get-Telemetry 'preflight' 0 0;Assert-ExperimentalSafety $sample
    if($ValidateOnly){Write-JSON (Join-Path $run 'validation-only.json') ([ordered]@{state='PASS';live_inference=$false;runtime=$before;telemetry=$sample});Write-Host 'SMALL_ENGLISH_PRODUCT_PROOF_VALIDATION=PASS';Write-Host 'LIVE_INFERENCE=false';exit 0}
    Invoke-GovernedCleanCacheReclaim
    $null=Get-StableRAMWindow 'initial_preload' ([double]$config.resource.admission_floor_gib)
    # Consumption begins before even the first generic model-load request.
    New-ExclusiveLock $dispatchLock ([DateTime]::UtcNow.ToString('o'))
    $dispatchCreated=$true
    Start-OwnedModel $false $true
    $env:NEXUSAI_SMALL_ENGLISH_LIVE='1'
    $env:NEXUSAI_SMALL_ENGLISH_CORPUS=Resolve-RepoPath ([string]$config.corpus)
    $env:NEXUSAI_SMALL_ENGLISH_RUN=$run
    $env:NEXUSAI_SMALL_ENGLISH_LOCK=Join-Path $privateRoot 'evaluator.dispatched.lock'
    $env:NEXUSAI_SMALL_ENGLISH_MODEL=[string]$config.model.id
    $env:NEXUSAI_SMALL_ENGLISH_URL=[string]$config.runtime.localai_url
    $binary=Resolve-RepoPath ([string]$config.evaluator)
    $child=Start-Process -FilePath $binary -ArgumentList @('-test.run=^TestForensicRecordsSynthesis$','"-ginkgo.focus=Fresh English product proof"','-ginkgo.v','-test.timeout=20m') -WorkingDirectory $root -RedirectStandardOutput (Join-Path $run 'evaluator.stdout.log') -RedirectStandardError (Join-Path $run 'evaluator.stderr.log') -PassThru -WindowStyle Hidden
    $lastCount=-1
    while(-not $child.HasExited){
        Start-Sleep -Seconds 2
        $sample=Get-Telemetry 'product_proof_inflight' 0 1;Assert-ExperimentalSafety $sample
        $resultsPath=Join-Path $run 'product-proof-results.json'
        if(Test-Path -LiteralPath $resultsPath){
            # A concurrent file replacement may be between reads; telemetry still runs.
            try{$result=Get-Content -LiteralPath $resultsPath -Raw | ConvertFrom-Json;$count=@($result.results).Count;if($count -ne $lastCount){Write-Host "PRODUCT_PROOF_COMPLETED_CASES=$count planned=45";$lastCount=$count}}catch{}
        }
        $child.Refresh()
    }
    $child.WaitForExit()
    $results=Get-Content -LiteralPath (Join-Path $run 'product-proof-results.json') -Raw | ConvertFrom-Json
    if($child.ExitCode -ne 0 -or @($results.results).Count -ne 45 -or $results.automated_failures -ne 0){throw 'PRODUCT_PROOF_AUTOMATED_GATE_FAILED'}
    $finalState='ENGLISH_PRODUCT_PROOF_COMPLETED_CONTENT_ADJUDICATION_REQUIRED';$exitCode=0
} catch {
    $errorMessage=$_.Exception.Message
    $finalState=$(if($dispatchCreated){'ENGLISH_PRODUCT_PROOF_INCOMPLETE_OR_FAILED'}else{'PREFLIGHT_FAILURE_NO_INFERENCE'})
    Write-Host "ERROR=$errorMessage"
} finally {
    if(-not $ValidateOnly -or $dispatchCreated){
        # Only the child created by this runner may be stopped.
        if($null -ne $child -and -not $child.HasExited){$child.Kill();$child.WaitForExit()}
        try{Invoke-UnloadOwnedModel}catch{$unloadState='FAIL';$exitCode=40;$finalState='UNLOAD_FAILURE';$errorMessage=$_.Exception.Message}
        if($null -ne $before){
            try{$after=Get-RuntimeState;Assert-RuntimeUnchanged $before $after;$integrityState='PASS'}catch{$integrityState='FAIL';$exitCode=40;$finalState='RUNTIME_INTEGRITY_FAIL';$errorMessage=$_.Exception.Message}
        }
        $receipt=[ordered]@{contract_version='nexusai.small-english-product-proof-receipt/v1';final_state=$finalState;error=$errorMessage;dispatch_created=$dispatchCreated;runtime_before=$before;runtime_after=$after;unload_state=$unloadState;runtime_integrity=$integrityState;resource_track='CLOSED';strict_certification='PENDING';content_review='REQUIRED';source_only_evaluator=$true;runtime_observations=$observationsPath;exit_code=$exitCode}
        Write-JSON $receiptPath $receipt
        [IO.File]::WriteAllText($receiptPath+'.sha256',(Get-SHA256 $receiptPath)+"`n",[Text.UTF8Encoding]::new($false))
        Write-Host "CURRENT_4B_MODEL_UNLOAD=$unloadState";Write-Host "RUNTIME_INTEGRITY=$integrityState";Write-Host "PRODUCT_PROOF_RECEIPT=$receiptPath"
    }
    $env:NEXUSAI_SMALL_ENGLISH_LIVE=$null
}
Write-Host "FINAL_STATE=$finalState"
exit $exitCode
