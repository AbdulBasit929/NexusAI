[CmdletBinding()]
param([switch]$ValidateOnly)
Set-StrictMode -Version Latest
$ErrorActionPreference='Stop'

function Get-LatencySummary($Rows) {
 $values=@($Rows|ForEach-Object{[int64]$_.latency_ms}|Sort-Object)
 if($values.Count -eq 0){return [ordered]@{p50_ms=$null;p95_ms=$null;max_ms=$null}}
 $p50=[math]::Max(0,[math]::Ceiling(0.50*$values.Count)-1)
 $p95=[math]::Max(0,[math]::Ceiling(0.95*$values.Count)-1)
 return [ordered]@{p50_ms=$values[$p50];p95_ms=$values[$p95];max_ms=$values[-1]}
}
function Get-DevelopmentOutcome($InferenceStarted,$AdmissionStarted,$DriverExitCode,$Results,$ErrorMessage,$Integrity,$Unload) {
 if($Integrity -eq 'FAIL' -or $Unload -eq 'FAIL'){return 'RUNTIME_INTEGRITY_FAILURE'}
 if(-not $InferenceStarted){if($AdmissionStarted){return 'RESOURCE_ADMISSION_FAILURE_NO_INFERENCE'};return 'PREFLIGHT_FAILURE_NO_INFERENCE'}
 if($null -eq $DriverExitCode -or $DriverExitCode -ne 0 -or $ErrorMessage -ne '' -or $null -eq $Results){return 'DEVELOPMENT_DRIVER_ERROR_AFTER_INFERENCE'}
 if(-not $Results.complete -or @($Results.registered).Count -ne 5 -or @($Results.dynamic).Count -ne 2 -or @($Results.terminal).Count -ne 3 -or @($Results.synthesis).Count -ne 3){return 'DEVELOPMENT_DRIVER_ERROR_AFTER_INFERENCE'}
 if(@($Results.registered|Where-Object{-not $_.correct}).Count -gt 0 -or @($Results.dynamic|Where-Object{-not $_.correct}).Count -gt 0 -or @($Results.terminal|Where-Object{-not $_.correct}).Count -gt 0 -or @($Results.synthesis|Where-Object{-not $_.validated}).Count -gt 0){return 'DEVELOPMENT_COMPLETED_FUNCTIONAL_FAILURE'}
 $semantic=Get-LatencySummary @($Results.registered);$dynamic=Get-LatencySummary @($Results.dynamic);$synthesis=Get-LatencySummary @($Results.synthesis)
 if($semantic.p95_ms -gt 30000 -or $semantic.max_ms -gt 180000 -or $dynamic.p95_ms -gt 60000 -or $dynamic.max_ms -gt 180000 -or $synthesis.p95_ms -gt 60000 -or $synthesis.max_ms -gt 180000){return 'DEVELOPMENT_COMPLETED_FUNCTIONAL_FAILURE'}
 return 'DEVELOPMENT_COMPLETED_PASS'
}
function Get-CorrectCount($Rows,[string]$Property='correct'){return @($Rows|Where-Object{$_.$Property -eq $true}).Count}

$root=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$configPath=Join-Path $PSScriptRoot 'current4b-final-characterization-freeze.json'
$config=Get-Content -LiteralPath $configPath -Raw|ConvertFrom-Json
$library=Join-Path $PSScriptRoot 'run_current4b_final_characterization.ps1'
if((Get-FileHash -LiteralPath $library -Algorithm SHA256).Hash.ToLowerInvariant() -cne 'ec0387052e0de7041799f869341000469211ccaaeda53dbf6bbcbbba53b2470b'){throw 'FROZEN_RESOURCE_LIBRARY_DRIFT'}
$tokens=$null;$parseErrors=$null;$ast=[Management.Automation.Language.Parser]::ParseFile($library,[ref]$tokens,[ref]$parseErrors)
if($parseErrors.Count -ne 0){throw 'RESOURCE_LIBRARY_PARSE_FAILED'}
foreach($definition in $ast.FindAll({param($node)$node -is [Management.Automation.Language.FunctionDefinitionAst]},$false)){. ([scriptblock]::Create($definition.Extent.Text))}

$privateRoot=Join-Path $root 'local-acceptance-models\nxb21-product-convergence-development'
$binary=Join-Path $privateRoot 'development.test.exe'
$run=Join-Path $privateRoot ('run-'+[DateTime]::UtcNow.ToString('yyyyMMddTHHmmssfffZ')+'-'+[guid]::NewGuid().ToString('N').Substring(0,8))
$observationsPath=Join-Path $run 'runtime-observations.ndjson'
$reportPath=Join-Path $root 'reports\nxb21\product-convergence-development-20260915.json'
$manifestPath=Join-Path $run 'development-source-manifest.json'
$before=$null;$after=$null;$modelOwned=$false;$child=$null;$lease=$null
$unloadState='NOT_REQUIRED';$integrityState='NOT_CHECKED';$state='PREFLIGHT_FAILURE_NO_INFERENCE';$errorMessage='';$exitCode=10
$pagingSince=$null;$baselineSwapKiB=$null;$experimentTimer=[Diagnostics.Stopwatch]::StartNew()
$requestCount=0;$childRequestCount=0;$loadCount=0;$reloadCount=0;$callResults=@();$observations=@()
$admissionStarted=$false;$liveInference=$false;$driverExitCode=$null;$results=$null
$ramAdmission='NOT_ATTEMPTED';$modelLoad='NOT_ATTEMPTED';$sourceRegression='NOT_RUN'
$retrievalPrecheck='NOT_RUN';$variantRecall='NOT_MEASURED';$manifestSHA='';$binarySHA='';$runnerSHA=''
$scratchName='';$scratchPassword='';$scratchRoleCreated=$false;$scratchDBCreated=$false

function Invoke-DevelopmentSQL([string]$Statement) {
 $output=@($Statement|& docker exec -i nexusai-forensic-postgres-1 psql -X -v ON_ERROR_STOP=1 -U localrecall -d localrecall -At)
 if($LASTEXITCODE -ne 0){throw 'DISPOSABLE_DEVELOPMENT_SQL_FAILED'}
 return ($output -join "`n").Trim()
}
function New-DevelopmentScratch {
 $script:scratchName='nxb21_dev_'+[guid]::NewGuid().ToString('N').Substring(0,16)
 $script:scratchPassword=[guid]::NewGuid().ToString('N')+[guid]::NewGuid().ToString('N')
 if($script:scratchName -cnotmatch '^nxb21_dev_[a-f0-9]{16}$'){throw 'UNSAFE_SCRATCH_ID'}
 if((Invoke-DevelopmentSQL "SELECT count(*) FROM pg_database WHERE datname='$script:scratchName';") -ne '0'){throw 'SCRATCH_DATABASE_EXISTS'}
 if((Invoke-DevelopmentSQL "SELECT count(*) FROM pg_roles WHERE rolname='$script:scratchName';") -ne '0'){throw 'SCRATCH_ROLE_EXISTS'}
 $null=Invoke-DevelopmentSQL "CREATE ROLE $script:scratchName LOGIN PASSWORD '$script:scratchPassword' NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE;";$script:scratchRoleCreated=$true
 $null=Invoke-DevelopmentSQL "CREATE DATABASE $script:scratchName OWNER $script:scratchName;";$script:scratchDBCreated=$true
 $env:NXB21_SOURCE_NATIVE_TEST_DATABASE_NAME=$script:scratchName
 $env:NXB21_SOURCE_NATIVE_TEST_DATABASE_URL="postgres://$($script:scratchName):$($script:scratchPassword)@127.0.0.1:5433/$($script:scratchName)?sslmode=disable"
}
function Remove-DevelopmentScratch {
 [Environment]::SetEnvironmentVariable('NXB21_SOURCE_NATIVE_TEST_DATABASE_URL',$null,'Process')
 [Environment]::SetEnvironmentVariable('NXB21_SOURCE_NATIVE_TEST_DATABASE_NAME',$null,'Process')
 if($script:scratchDBCreated){
  if((Invoke-DevelopmentSQL "SELECT pg_get_userbyid(datdba) FROM pg_database WHERE datname='$script:scratchName';") -cne $script:scratchName){throw 'SCRATCH_OWNERSHIP_CHANGED'}
  $null=Invoke-DevelopmentSQL "DROP DATABASE $script:scratchName;";$script:scratchDBCreated=$false
 }
 if($script:scratchRoleCreated){$null=Invoke-DevelopmentSQL "DROP ROLE $script:scratchName;";$script:scratchRoleCreated=$false}
}
function New-SourceManifest {
 $template='{{if not .Standard}}{{.Dir}}|{{range .GoFiles}}{{.}};{{end}}|{{range .CgoFiles}}{{.}};{{end}}|{{range .CFiles}}{{.}};{{end}}|{{range .CXXFiles}}{{.}};{{end}}|{{range .MFiles}}{{.}};{{end}}|{{range .HFiles}}{{.}};{{end}}|{{range .SFiles}}{{.}};{{end}}|{{range .SysoFiles}}{{.}};{{end}}|{{range .EmbedFiles}}{{.}};{{end}}|{{range .TestGoFiles}}{{.}};{{end}}|{{range .XTestGoFiles}}{{.}};{{end}}{{end}}'
 $rows=Invoke-Native go @('list','-deps','-test','-f',$template,'./api/forensic_records')
 $paths=@('go.mod','go.sum','scripts/nxb21-english-functional-qualification/run_product_convergence_development.ps1','scripts/nxb21-english-functional-qualification/run_current4b_final_characterization.ps1','scripts/nxb21-english-functional-qualification/current4b-final-characterization-freeze.json','scripts/nxb21-english-functional-qualification/test_development_runner_state.ps1')
 foreach($line in @($rows -split '\r?\n')){
  if([string]::IsNullOrWhiteSpace($line)){continue};$parts=$line -split '\|',-1;$dir=[string]$parts[0]
  if(-not $dir.StartsWith($root,[StringComparison]::OrdinalIgnoreCase)){continue}
  foreach($part in @($parts|Select-Object -Skip 1)){foreach($name in @($part -split ';')){
   if([string]::IsNullOrWhiteSpace($name)){continue}
   $fullName=if([IO.Path]::IsPathRooted($name)){[IO.Path]::GetFullPath($name)}else{[IO.Path]::GetFullPath((Join-Path $dir $name))}
   if($fullName.StartsWith($root,[StringComparison]::OrdinalIgnoreCase) -and (Test-Path -LiteralPath $fullName)){$paths+=($fullName.Substring($root.Length).TrimStart('\').Replace('\','/'))}
  }}
 }
 $entries=@()
 foreach($relative in @($paths|Sort-Object -Unique)){
  $full=Join-Path $root $relative;if(-not(Test-Path -LiteralPath $full)){throw "SOURCE_MANIFEST_FILE_MISSING:$relative"}
  $entries+=[ordered]@{path=$relative;sha256=Get-SHA256 $full;bytes=(Get-Item -LiteralPath $full).Length}
 }
 $branch=Invoke-Native git @('branch','--show-current');$head=Invoke-Native git @('rev-parse','HEAD');$dirty=Invoke-Native git @('status','--short','--untracked-files=all')
 $dirtyLines=@($dirty -split '\r?\n'|Where-Object{$_ -ne ''});$statusTemp=Join-Path $run 'git-dirty-status.txt'
 [IO.File]::WriteAllText($statusTemp,($dirtyLines -join "`n")+"`n",[Text.UTF8Encoding]::new($false))
 $manifest=[ordered]@{contract_version='nexusai.development-source-manifest/v1';branch=$branch;head=$head;dirty_summary=[ordered]@{entries=$dirtyLines.Count;sha256=Get-SHA256 $statusTemp};go_version=(Invoke-Native go @('version'));go_env=[ordered]@{GOVERSION=(Invoke-Native go @('env','GOVERSION'));GOOS=(Invoke-Native go @('env','GOOS'));GOARCH=(Invoke-Native go @('env','GOARCH'))};files=$entries}
 Write-JSON $manifestPath $manifest;return $manifest
}
function Get-DevelopmentUIIndexSHA256 {
 $client=New-Object System.Net.WebClient
 try{$client.Headers['Accept']='text/html';$bytes=$client.DownloadData(([string]$config.runtime.localai_url).TrimEnd('/')+'/')}
 finally{$client.Dispose()}
 $sha=[Security.Cryptography.SHA256]::Create();try{return ([BitConverter]::ToString($sha.ComputeHash($bytes))).Replace('-','').ToLowerInvariant()}finally{$sha.Dispose()}
}
function Get-DevelopmentRuntimeState {
 $containers=Get-Containers
 foreach($name in $containers.Keys){$c=$containers[$name];if($c.status-cne'running' -or $c.oom_killed -or $c.health-eq'unhealthy'){throw "CONTAINER_UNHEALTHY:$name"}}
 $postgres=[string]$containers.'nexusai-forensic-postgres-1'.id
 $tupleSQL="SELECT (SELECT count(*) FROM forensic.evidence_items)||(chr(124))||(SELECT count(*) FROM forensic.evidence_versions)||(chr(124))||(SELECT count(*) FROM forensic.records_ingest_jobs)||(chr(124))||(SELECT count(*) FROM forensic.records)||(chr(124))||(SELECT count(*) FROM forensic.derived_artifacts)||(chr(124))||(SELECT count(*) FROM forensic.kb_collection_assets);"
 $system=Invoke-RestMethod -Uri ([string]$config.runtime.localai_url+'/system') -TimeoutSec 20
 return [ordered]@{containers=$containers;loaded_model_ids=@($system.loaded_models|ForEach-Object{[string]$_.id}|Sort-Object);retained_tuple=Invoke-Native docker @('exec',$postgres,'psql','-U','localrecall','-d','localrecall','-Atc',$tupleSQL);activity_count=[int](Invoke-Native docker @('exec',$postgres,'psql','-U','localrecall','-d','localrecall','-Atc','SELECT count(*) FROM public.agent_analysis_history;'));active_jobs=[int](Invoke-Native docker @('exec',$postgres,'psql','-U','localrecall','-d','localrecall','-Atc',"SELECT count(*) FROM forensic.records_ingest_jobs WHERE status NOT IN ('completed','dead_letter');"));analyst_index_sha256=Get-DevelopmentUIIndexSHA256}
}
function Assert-DevelopmentRuntimeBaseline($State) {
 if([string]::IsNullOrWhiteSpace([string]$State.retained_tuple) -or $State.active_jobs-ne0){throw 'RUNTIME_BASELINE_UNSAFE'}
}
function Write-AdjudicationReport {
 $registered=@();$dynamic=@();$terminal=@();$synthesis=@()
 if($null-ne$results){$registered=@($results.registered);$dynamic=@($results.dynamic);$terminal=@($results.terminal);$synthesis=@($results.synthesis)}
 $registeredCorrect=Get-CorrectCount $registered;$dynamicCorrect=Get-CorrectCount $dynamic;$terminalCorrect=Get-CorrectCount $terminal;$synthesisValidated=Get-CorrectCount $synthesis 'validated'
 $fallback=@($synthesis|Where-Object{$_.fallback-eq$true}).Count
 $authority=@($registered|Where-Object{$_.authority_safe-ne$true}).Count+@($dynamic|Where-Object{$_.authority_safe-ne$true}).Count
 $critical=@($synthesis|Where-Object{$_.critical_value_safe-ne$true}).Count;$factRefs=@($synthesis|Where-Object{$_.fact_refs_safe-ne$true}).Count;$citations=@($synthesis|Where-Object{$_.citation_refs_safe-ne$true}).Count
 $semanticRole=if($registered.Count-ne5){'UNADJUDICATED'}elseif($registeredCorrect-eq5){'KEEP'}elseif($registeredCorrect-eq4){'ONE_BOUNDED_REVIEW_REQUIRED'}else{'REPLACE_REQUIRED'}
 $synthesisRole=if($synthesis.Count-ne3){'UNADJUDICATED'}elseif($synthesisValidated-eq3){'KEEP'}else{'REPLACE_REQUIRED'}
 $replacement=if($semanticRole-eq'REPLACE_REQUIRED' -or $synthesisRole-eq'REPLACE_REQUIRED'){$true}elseif($semanticRole-eq'ONE_BOUNDED_REVIEW_REQUIRED'){'PENDING_BOUNDED_ROOT_CAUSE'}elseif($semanticRole-eq'UNADJUDICATED' -or $synthesisRole-eq'UNADJUDICATED'){'UNDETERMINED'}else{$false}
 if($state-eq'RESOURCE_ADMISSION_FAILURE_NO_INFERENCE' -or $errorMessage-match 'PHYSICAL_RESERVE|RAM_STABLE_WINDOW_FAILED'){$next='Close Codex and unused applications, keep Docker running, then run: powershell.exe -NoProfile -ExecutionPolicy Bypass -File "C:\Users\sheik\Workspace\Office\Projects\NexusAI\scripts\nxb21-english-functional-qualification\run_product_convergence_development.ps1"'}
 elseif($state-eq'DEVELOPMENT_COMPLETED_PASS'){$next='Proceed to Work Item 5: compact P1 all-family functional baseline, five representative end-to-end analyst flows, and deployed Overview/Evidence/Ask/Activity acceptance. Do not create V3 yet.'}
 elseif($liveInference){$next='STOP. Prepare model-replacement candidate research only. Do not download anything without explicit approval.'}
 else{$next='STOP. Repair the reported source or runtime preflight failure before any live adjudication.'}
 $manifest=$null;if(Test-Path $manifestPath){$manifest=Get-Content $manifestPath -Raw|ConvertFrom-Json}
 $report=[ordered]@{
  CURRENT_BRANCH=$(if($null-ne$manifest){$manifest.branch}else{''});CURRENT_HEAD=$(if($null-ne$manifest){$manifest.head}else{''})
  SOURCE_MANIFEST=$manifestPath;SOURCE_MANIFEST_SHA256=$manifestSHA;DEVELOPMENT_BINARY=$binary;DEVELOPMENT_BINARY_SHA256=$binarySHA;RUNNER_SHA256=$runnerSHA
  RETRIEVAL_PRECHECK=$retrievalPrecheck;VARIANT_LEDGER_TOP5_RECALL=$variantRecall;SOURCE_REGRESSION=$sourceRegression
  RAM_ADMISSION=$ramAdmission;MODEL_LOAD=$modelLoad;MODEL_UNLOAD=$unloadState;RUNTIME_INTEGRITY=$integrityState
  REGISTERED_CASES=$registered.Count;REGISTERED_CORRECT=$registeredCorrect;DYNAMIC_CASES=$dynamic.Count;DYNAMIC_CORRECT=$dynamicCorrect;TERMINAL_CASES=$terminal.Count;TERMINAL_CORRECT=$terminalCorrect
  SYNTHESIS_CASES=$synthesis.Count;SYNTHESIS_VALIDATED=$synthesisValidated;SYNTHESIS_FALLBACK=$fallback
  AUTHORITY_VIOLATIONS=$authority;CRITICAL_VALUE_FAILURES=$critical;FACT_REF_FAILURES=$factRefs;CITATION_FAILURES=$citations
  SEMANTIC_P50=(Get-LatencySummary $registered).p50_ms;SEMANTIC_P95=(Get-LatencySummary $registered).p95_ms;SEMANTIC_MAX=(Get-LatencySummary $registered).max_ms
  DYNAMIC_P50=(Get-LatencySummary $dynamic).p50_ms;DYNAMIC_P95=(Get-LatencySummary $dynamic).p95_ms;DYNAMIC_MAX=(Get-LatencySummary $dynamic).max_ms
  SYNTHESIS_P50=(Get-LatencySummary $synthesis).p50_ms;SYNTHESIS_P95=(Get-LatencySummary $synthesis).p95_ms;SYNTHESIS_MAX=(Get-LatencySummary $synthesis).max_ms
  CURRENT_4B_SEMANTIC_ROLE=$semanticRole;CURRENT_4B_SYNTHESIS_ROLE=$synthesisRole;MODEL_REPLACEMENT_REQUIRED=$replacement
  DEVELOPMENT_FINAL_STATE=$state;REPORT_COMPLETENESS=$(if($null-ne$results -and $results.complete){'COMPLETE'}elseif($null-ne$results){'PARTIAL_DRIVER_ABORT'}else{'NO_CASE_RESULTS'});LIVE_INFERENCE=$liveInference;ANY_REQUEST_DISPATCHED=$liveInference;RUNNER_RESOURCE_REQUEST_COUNT=$requestCount;CHILD_MODEL_REQUEST_COUNT=$childRequestCount;CHILD_PROCESS_ID=$(if($null-ne$child){$child.Id}else{$null});CHILD_EXIT_CODE=$driverExitCode
  NEW_FORMAL_PROOF_CREATED=$false;NX_B21_D='OPEN';EXACT_NEXT_ACTION=$next
  MODEL_IDENTITY=$config.model;RUNTIME_ENVELOPE=[ordered]@{path=$configPath;sha256=Get-SHA256 $configPath;admission_floor_gib=$config.resource.admission_floor_gib}
  CASE_RESULTS=[ordered]@{registered=$registered;dynamic=$dynamic;terminal=$terminal;synthesis=$synthesis};ERROR=$errorMessage
 }
 Write-JSON $reportPath $report;Write-JSON (Join-Path $run 'development-adjudication.json') $report
}

New-Item -ItemType Directory -Path $run -Force|Out-Null
try {
 $lease=[IO.File]::Open((Join-Path $privateRoot 'development.active'),[IO.FileMode]::CreateNew,[IO.FileAccess]::Write,[IO.FileShare]::None)
 $env:GOCACHE=Join-Path $root '.tmp-go-cache';$manifest=New-SourceManifest;$manifestSHA=Get-SHA256 $manifestPath;$runnerSHA=Get-SHA256 $PSCommandPath
 Write-Host "SOURCE_MANIFEST=$manifestPath";Write-Host "SOURCE_MANIFEST_SHA256=$manifestSHA"
 $regressionLog=Join-Path $run 'source-regression.log'
 $checks=@(@('go',@('test','./api/forensic_records','-count=1')),@('go',@('test','./core/services/agents','-count=1')),@('go',@('vet','./api/forensic_records','./core/services/agents')),@('powershell.exe',@('-NoProfile','-ExecutionPolicy','Bypass','-File',(Join-Path $PSScriptRoot 'test_development_runner_state.ps1'))))
 foreach($check in $checks){$line=Invoke-Native ([string]$check[0]) ([string[]]$check[1]);[IO.File]::AppendAllText($regressionLog,$line+"`n",[Text.UTF8Encoding]::new($false))}
 $recall=Invoke-Native go @('test','./api/forensic_records','-run','^TestRetrievalFirstSemanticRecall$','-count=1','-v');[IO.File]::AppendAllText($regressionLog,$recall+"`n",[Text.UTF8Encoding]::new($false))
 if($recall -match 'DEVELOPMENT_EXPECTED_OPS_TOP5='){throw 'RETRIEVAL_PRECHECK_FAILED'};$retrievalPrecheck='PASS 5/5'
 if($recall -notmatch 'RETRIEVAL_TOP5=96\.35% \(132/137\)'){throw 'VARIANT_LEDGER_RECALL_DRIFT'};$variantRecall='96.35% (132/137)';$sourceRegression='PASS'
 Write-Host 'SOURCE_REGRESSION=PASS';Write-Host "RETRIEVAL_PRECHECK=$retrievalPrecheck";Write-Host "VARIANT_LEDGER_TOP5_RECALL=$variantRecall"
 $null=Invoke-Native go @('test','-c','-o',$binary,'./api/forensic_records');$binarySHA=Get-SHA256 $binary;[IO.File]::WriteAllText($binary+'.sha256',$binarySHA+"`n",[Text.UTF8Encoding]::new($false))
 Write-Host "DEVELOPMENT_BINARY=$binary";Write-Host "DEVELOPMENT_BINARY_SHA256=$binarySHA"
 $before=Get-DevelopmentRuntimeState;Assert-DevelopmentRuntimeBaseline $before;Assert-ModelIdentity
 if($null-ne(Get-LoadedState)){throw 'CURRENT_4B_MUST_START_UNLOADED'};$sample=Get-Telemetry 'preflight' 0 0;Assert-ExperimentalSafety $sample
 if($ValidateOnly){$state='PREFLIGHT_FAILURE_NO_INFERENCE';$errorMessage='VALIDATE_ONLY_COMPLETED_NO_INFERENCE';$exitCode=0}
 else {
  $admissionStarted=$true;Invoke-GovernedCleanCacheReclaim;$null=Get-StableRAMWindow 'initial_preload' ([double]$config.resource.admission_floor_gib);$ramAdmission='PASS'
  New-DevelopmentScratch;$experimentTimer=[Diagnostics.Stopwatch]::StartNew();Start-OwnedModel $false $true;$liveInference=$requestCount-gt0;$modelLoad='PASS';$state='DEVELOPMENT_DRIVER_ERROR_AFTER_INFERENCE'
  $env:NXB21_DEVELOPMENT_LIVE='1';$env:NXB21_DEVELOPMENT_RUN=$run;$env:NXB21_DEVELOPMENT_URL=[string]$config.runtime.localai_url
  $child=Start-Process -FilePath $binary -ArgumentList @('-test.run=^TestForensicRecordsSynthesis$','"-ginkgo.focus=Product convergence development adjudication"','-ginkgo.no-color','-ginkgo.succinct','-test.timeout=20m') -WorkingDirectory $root -RedirectStandardOutput (Join-Path $run 'stdout.log') -RedirectStandardError (Join-Path $run 'stderr.log') -PassThru -WindowStyle Hidden
  $null=$child.Handle;$lastProgress=''
  while(-not$child.HasExited){
   Start-Sleep -Seconds 2;$sample=Get-Telemetry 'development_inflight' 0 1;Assert-ExperimentalSafety $sample;$child.Refresh();$progressPath=Join-Path $run 'development-results.json'
   if(Test-Path $progressPath){try{$progress=Get-Content $progressPath -Raw|ConvertFrom-Json;$message="registered=$(@($progress.registered).Count)/5 dynamic=$(@($progress.dynamic).Count)/2 terminal=$(@($progress.terminal).Count)/3 synthesis=$(@($progress.synthesis).Count)/3";if($message-cne$lastProgress){Write-Host "DEVELOPMENT_COMPLETED $message";$lastProgress=$message}}catch{}}
  }
  $child.WaitForExit();$driverExitCode=$child.ExitCode
  if($null-eq$driverExitCode){throw 'DEVELOPMENT_DRIVER_EXIT_CODE_UNAVAILABLE'};if($driverExitCode-ne0){throw "DEVELOPMENT_DRIVER_FAILED:$driverExitCode"}
  $results=Get-Content -LiteralPath (Join-Path $run 'development-results.json') -Raw|ConvertFrom-Json;if(-not$results.complete){throw 'DEVELOPMENT_INCOMPLETE'};$exitCode=0
 }
} catch {
 $errorMessage=$_.Exception.Message;if(($admissionStarted -or $errorMessage-match 'PHYSICAL_RESERVE|RAM_STABLE_WINDOW_FAILED') -and -not$liveInference -and $ramAdmission-ne'PASS'){$ramAdmission='FAIL'}
 $liveInference=$liveInference -or $requestCount-gt0 -or (Test-Path (Join-Path $run 'model-requests.ndjson'));if($liveInference -and $modelLoad-eq'NOT_ATTEMPTED'){$modelLoad='FAIL'};Write-Host "ERROR=$errorMessage"
} finally {
 if($null-ne$child -and -not$child.HasExited){$child.Kill();$child.WaitForExit();$driverExitCode=$child.ExitCode}
 $resultsPath=Join-Path $run 'development-results.json'
 if($null-eq$results -and (Test-Path -LiteralPath $resultsPath)){try{$results=Get-Content -LiteralPath $resultsPath -Raw|ConvertFrom-Json}catch{}}
 $childRequestsPath=Join-Path $run 'model-requests.ndjson'
 if(Test-Path -LiteralPath $childRequestsPath){$childRequestCount=@([IO.File]::ReadLines($childRequestsPath)|Where-Object{-not[string]::IsNullOrWhiteSpace($_)}).Count}
 try{Remove-DevelopmentScratch}catch{$integrityState='FAIL';$exitCode=40;$errorMessage=$_.Exception.Message}
 try{Invoke-UnloadOwnedModel}catch{$unloadState='FAIL';$exitCode=40;$errorMessage=$_.Exception.Message}
 if($null-ne$before){try{$after=Get-DevelopmentRuntimeState;Assert-RuntimeUnchanged $before $after;if($integrityState-ne'FAIL'){$integrityState='PASS'}}catch{$integrityState='FAIL';$exitCode=40;$errorMessage=$_.Exception.Message}}
 $liveInference=$liveInference -or $requestCount-gt0 -or (Test-Path (Join-Path $run 'model-requests.ndjson'))
 if(-not$ValidateOnly -or $integrityState-eq'FAIL'){$state=Get-DevelopmentOutcome $liveInference $admissionStarted $driverExitCode $results $errorMessage $integrityState $unloadState}
 if($state-eq'DEVELOPMENT_COMPLETED_FUNCTIONAL_FAILURE'){$exitCode=20};if($state-eq'DEVELOPMENT_DRIVER_ERROR_AFTER_INFERENCE' -and $exitCode-eq0){$exitCode=30}
 Write-AdjudicationReport
 Write-JSON (Join-Path $run 'development-runtime-receipt.json') ([ordered]@{purpose='DEVELOPMENT_NOT_CERTIFICATION';final_state=$state;error=$errorMessage;live_inference=$liveInference;request_count=($requestCount+$childRequestCount);runner_resource_request_count=$requestCount;child_model_request_count=$childRequestCount;driver_process_id=$(if($null-ne$child){$child.Id}else{$null});driver_exit_code=$driverExitCode;new_formal_proof_created=$false;source_manifest=$manifestPath;source_manifest_sha256=$manifestSHA;binary=$binary;binary_sha256=$binarySHA;runner_sha256=$runnerSHA;runtime_before=$before;runtime_after=$after;unload=$unloadState;integrity=$integrityState;exit_code=$exitCode})
 foreach($name in @('NXB21_DEVELOPMENT_LIVE','NXB21_DEVELOPMENT_RUN','NXB21_DEVELOPMENT_URL','NXB21_SOURCE_NATIVE_TEST_DATABASE_URL','NXB21_SOURCE_NATIVE_TEST_DATABASE_NAME')){[Environment]::SetEnvironmentVariable($name,$null,'Process')}
 if($null-ne$lease){$lease.Dispose();[IO.File]::Delete((Join-Path $privateRoot 'development.active'))}
 Write-Host "DEVELOPMENT_REPORT=$reportPath";Write-Host "DEVELOPMENT_RUN=$run";Write-Host "MODEL_UNLOAD=$unloadState";Write-Host "RUNTIME_INTEGRITY=$integrityState";Write-Host "LIVE_INFERENCE=$liveInference";Write-Host "FINAL_STATE=$state"
}
exit $exitCode
