# SPDX-License-Identifier: MIT
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$script:NxRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$script:NxConfigPath = Join-Path $PSScriptRoot 'nxb21d_operator_config.json'
$script:NxPrivateRoot = Join-Path $script:NxRoot 'local-acceptance-models\nxb21-d\offline-runs'

function Resolve-NxDPath([string]$Relative) { return [IO.Path]::GetFullPath((Join-Path $script:NxRoot $Relative)) }
function Get-NxDHash([string]$Path) { return (Get-FileHash -Algorithm SHA256 -LiteralPath $Path).Hash.ToLowerInvariant() }
function Write-NxDJson([string]$Path, $Value) {
    $parent=Split-Path -Parent $Path; if(-not(Test-Path -LiteralPath $parent)){New-Item -ItemType Directory -Force -Path $parent|Out-Null}
    [IO.File]::WriteAllText($Path,($Value|ConvertTo-Json -Depth 100)+"`n",[Text.UTF8Encoding]::new($false))
}
function Assert-NxDBundleIntegrity {
    $path=Join-Path $PSScriptRoot 'nxb21d_bundle_integrity.json';if(-not(Test-Path $path)){throw 'BUNDLE_INTEGRITY_MISSING'};$seal=Get-Content $path -Raw|ConvertFrom-Json
    foreach($p in $seal.files.PSObject.Properties){if((Get-NxDHash (Resolve-NxDPath $p.Name))-cne[string]$p.Value){throw "BUNDLE_INTEGRITY_FAILED:$($p.Name)"}}
}
function Get-NxDConfig { Assert-NxDBundleIntegrity; return Get-Content -LiteralPath $script:NxConfigPath -Raw | ConvertFrom-Json }
function Get-NxDRamGiB {
    $os=Get-CimInstance Win32_OperatingSystem
    return [math]::Round(([int64]$os.FreePhysicalMemory*1024)/1GB,3)
}
function Get-NxDHighMemoryProcesses {
    return @(Get-Process -ErrorAction SilentlyContinue|Sort-Object WorkingSet64 -Descending|Select-Object -First 10|ForEach-Object{[pscustomobject]@{name=$_.ProcessName;pid=$_.Id;working_set_mib=[math]::Round($_.WorkingSet64/1MB,1)}})
}
function Invoke-NxDNative([string]$File,[string[]]$Arguments) {
    Push-Location $script:NxRoot
    try {$output=@(& $File @Arguments 2>&1);$code=$LASTEXITCODE} finally {Pop-Location}
    if($code-ne 0){throw "$File failed ($code): $($output -join [Environment]::NewLine)"};return ($output -join [Environment]::NewLine).Trim()
}
function Invoke-NxDNativeStreaming([string]$File,[string[]]$Arguments,[string]$LogPath) {
    $code=1;Push-Location $script:NxRoot
    try{& $File @Arguments 2>&1|Tee-Object -FilePath $LogPath|ForEach-Object{Write-Host $_};$code=$LASTEXITCODE}finally{Pop-Location}
    if($code-ne0){throw "$File failed ($code); inspect $LogPath"}
}
function New-NxDRun([string]$Kind) {
    $dir=Join-Path $script:NxPrivateRoot ($Kind+'-'+[DateTime]::UtcNow.ToString('yyyyMMddTHHmmssfffZ'));New-Item -ItemType Directory -Force -Path $dir|Out-Null
    Write-NxDJson (Join-Path $script:NxPrivateRoot "latest-$Kind.json") ([ordered]@{run_directory=$dir;created_at=[DateTime]::UtcNow.ToString('o')});return $dir
}
function Assert-NxDRam($Config,[string]$Stage) {
    $available=Get-NxDRamGiB;$required=[double]$Config.gates.minimum_available_ram_gib
    Write-Host "RAM_GATE stage=$Stage available_gib=$available required_gib=$required"
    if($available-lt$required){foreach($p in Get-NxDHighMemoryProcesses){Write-Host "HIGH_MEMORY_PROCESS name=$($p.name) pid=$($p.pid) working_set_mib=$($p.working_set_mib)"};throw [InvalidOperationException]::new("RAM_TOO_LOW:$available")};return $available
}
function Wait-NxDRamAfterCleanCache($Config,[string]$Stage,[int]$WaitSeconds=60) {
    $required=[double]$Config.gates.minimum_available_ram_gib;$deadline=[DateTime]::UtcNow.AddSeconds($WaitSeconds)
    do{
        $available=Get-NxDRamGiB
        Write-Host "RAM_RECLAIM_WAIT stage=$Stage available_gib=$available required_gib=$required"
        if($available-ge$required){return $available}
        if([DateTime]::UtcNow-ge$deadline){break}
        Start-Sleep -Seconds 5
    }while([DateTime]::UtcNow-lt$deadline)
    return Assert-NxDRam $Config $Stage
}
function Invoke-NxDWslMemoryCommand([ValidateSet('dirty','writeback','drop-clean-cache')][string]$Command) {
    $arguments=switch($Command){
        'dirty' {'--distribution docker-desktop --user root --exec grep ^Dirty: /proc/meminfo'}
        'writeback' {'--distribution docker-desktop --user root --exec grep ^Writeback: /proc/meminfo'}
        'drop-clean-cache' {'--distribution docker-desktop --user root --exec /sbin/sysctl -w vm.drop_caches=1'}
    }
    $info=New-Object Diagnostics.ProcessStartInfo
    $info.FileName=Join-Path $env:WINDIR 'System32\wsl.exe';$info.Arguments=$arguments;$info.WorkingDirectory=$script:NxRoot
    $info.UseShellExecute=$false;$info.CreateNoWindow=$true;$info.RedirectStandardOutput=$true;$info.RedirectStandardError=$true
    $process=New-Object Diagnostics.Process;$process.StartInfo=$info;$started=$false
    try{
        if(-not$process.Start()){throw 'Could not start fixed WSL memory command.'};$started=$true
        $stdoutTask=$process.StandardOutput.ReadToEndAsync();$stderrTask=$process.StandardError.ReadToEndAsync()
        if(-not$process.WaitForExit(30000)){$process.Kill();$process.WaitForExit();throw 'Fixed WSL memory command timed out.'}
        if($process.ExitCode-ne0){throw "Fixed WSL memory command failed: $Command ($($stderrTask.Result.Trim()))"}
        return $stdoutTask.Result.Trim()
    }finally{if($started-and-not$process.HasExited){$process.Kill()};$process.Dispose()}
}
function Invoke-NxDSafeLinuxCleanCache {
    for($attempt=1;$attempt-le6;$attempt++){
        $dirtyLine=Invoke-NxDWslMemoryCommand dirty;$writebackLine=Invoke-NxDWslMemoryCommand writeback;$values=@{}
        foreach($line in ("$dirtyLine`n$writebackLine"-split'\r?\n')){if($line-match'^(Dirty|Writeback):\s+([0-9]+)\s+kB$'){$values[$Matches[1]]=[int64]$Matches[2]}}
        if($values.Count-eq2-and$values.Writeback-eq0){
            $null=Invoke-NxDWslMemoryCommand drop-clean-cache
            Write-Host "CLEAN_FILE_CACHE_RECOVERY=PASS dirty_kib=$($values.Dirty) writeback_kib=0 dirty_pages_preserved=true sync=false"
            return $true
        }
        $dirty=if($values.ContainsKey('Dirty')){$values.Dirty}else{'unknown'};$writeback=if($values.ContainsKey('Writeback')){$values.Writeback}else{'unknown'}
        Write-Host "CLEAN_FILE_CACHE_RECOVERY=WAIT attempt=$attempt dirty_kib=$dirty writeback_kib=$writeback"
        if($attempt-lt6){Start-Sleep -Seconds 5}
    }
    Write-Host 'CLEAN_FILE_CACHE_RECOVERY=SKIPPED reason=writeback_nonzero_or_meminfo_unavailable sync=false'
    return $false
}
function Get-NxDContainers {
    $rows=Invoke-NxDNative docker @('inspect','nexusai-api-1','nexusai-forensic-records-api-1','nexusai-forensic-records-worker-1','nexusai-forensic-postgres-1','nexusai-forensic-nats-1')|ConvertFrom-Json
    $map=@{};foreach($r in $rows){$name=$r.Name.TrimStart('/').Replace('nexusai-','').Replace('-1','');$map[$name]=$r};return $map
}
function Test-NxDContainerHealthy($Container) {
    if($Container.State.Status-cne'running'-or[bool]$Container.State.OOMKilled){return $false}
    $healthProperty=$Container.State.PSObject.Properties['Health']
    if($null-ne$healthProperty-and$null-ne$healthProperty.Value){return [string]$healthProperty.Value.Status-ceq'healthy'}
    return $true
}
function Get-NxDRetainedTuple($Containers) {
    $sql='SELECT (SELECT count(*) FROM forensic.evidence_items)||(chr(124))||(SELECT count(*) FROM forensic.evidence_versions)||(chr(124))||(SELECT count(*) FROM forensic.records_ingest_jobs)||(chr(124))||(SELECT count(*) FROM forensic.records)||(chr(124))||(SELECT count(*) FROM forensic.derived_artifacts)||(chr(124))||(SELECT count(*) FROM forensic.kb_collection_assets);'
    return Invoke-NxDNative docker @('exec',$Containers['forensic-postgres'].Id,'psql','-U','localrecall','-d','localrecall','-Atc',$sql)
}
function Get-NxDActiveJobs($Containers) {
    return [int](Invoke-NxDNative docker @('exec',$Containers['forensic-postgres'].Id,'psql','-U','localrecall','-d','localrecall','-Atc',"SELECT count(*) FROM forensic.records_ingest_jobs WHERE status NOT IN ('completed','dead_letter');"))
}
function Get-NxDActivityCount($Containers) {
    return [int](Invoke-NxDNative docker @('exec',$Containers['forensic-postgres'].Id,'psql','-U','localrecall','-d','localrecall','-Atc','SELECT count(*) FROM public.agent_analysis_history;'))
}
function Assert-NxDHTTPHealth($Config) {
    $probes=@(
        'http://127.0.0.1:8091/healthz',
        ($Config.runtime.localai_url+'/readyz'),
        ($Config.runtime.localai_url+'/'),
        'http://127.0.0.1:9109/metrics',
        'http://127.0.0.1:8222/healthz'
    )
    foreach($uri in $probes){
        try{$response=Invoke-WebRequest -UseBasicParsing -Method Get -Uri $uri -TimeoutSec 15}
        catch{throw "RUNTIME_HTTP_UNHEALTHY:$uri $($_.Exception.Message)"}
        if([int]$response.StatusCode-lt 200-or[int]$response.StatusCode-ge 400){throw "RUNTIME_HTTP_UNHEALTHY:$uri status=$($response.StatusCode)"}
    }
}
function Get-NxDImageId([string]$Image) {
    return (Invoke-NxDNative docker @('image','inspect','--format','{{.Id}}',$Image)).Trim()
}
function Assert-NxDServiceImages($Before,$After,$ExpectedImages,[bool]$RequireReplacement) {
    foreach($name in @('api','forensic-records-api')){
        if(-not(Test-NxDContainerHealthy $After[$name])){throw "ACTIVATION_VERIFICATION_FAILED:$name unhealthy"}
        if($After[$name].Image-cne[string]$ExpectedImages[$name]){throw "ACTIVATION_VERIFICATION_FAILED:$name image"}
        if($RequireReplacement-and$After[$name].Id-ceq$Before[$name].Id){throw "ACTIVATION_VERIFICATION_FAILED:$name was not recreated"}
    }
}
function Assert-NxDProtectedServices($Before,$After) {
    foreach($name in @('forensic-records-worker','forensic-postgres','forensic-nats')){
        $a=$After[$name];$b=$Before[$name]
        if($a.Id-cne$b.Id-or$a.Image-cne$b.Image-or[int]$a.RestartCount-ne[int]$b.RestartCount){throw "PROTECTED_SERVICE_CHANGED:$name"}
        if(-not(Test-NxDContainerHealthy $a)){throw "PROTECTED_SERVICE_UNHEALTHY:$name"}
    }
}
function Get-NxDModelArtifactReceipt($Config,$Containers) {
    $path='/models/'+[string]$Config.model.artifact;$size=[int64](Invoke-NxDNative docker @('exec',$Containers['api'].Id,'stat','-c','%s',$path));$hash=((Invoke-NxDNative docker @('exec',$Containers['api'].Id,'sha256sum',$path))-split'\s+')[0]
    if($size-ne[int64]$Config.model.artifact_bytes-or$hash-cne[string]$Config.model.artifact_sha256){throw 'MODEL_ARTIFACT_DRIFT'};return [pscustomobject]@{path=$path;bytes=$size;sha256=$hash}
}
function Test-NxDSourceManifest($Config) {
    $m=Get-Content -LiteralPath (Resolve-NxDPath $Config.d_source_manifest) -Raw|ConvertFrom-Json
    if($m.source_digest_sha256-cne[string]$Config.d_source_digest){throw 'SOURCE_DRIFT:manifest_digest'}
    $ordered=[ordered]@{};$names=@($m.files.PSObject.Properties.Name);[Array]::Sort($names,[StringComparer]::Ordinal);foreach($name in $names){$value=[string]$m.files.PSObject.Properties[$name].Value;if((Get-NxDHash (Resolve-NxDPath $name))-cne$value){throw "SOURCE_DRIFT:$name"};$ordered[$name]=$value}
    $json=$ordered|ConvertTo-Json -Compress; $bytes=[Text.Encoding]::UTF8.GetBytes($json);$sha=[Security.Cryptography.SHA256]::Create();try{$digest=([BitConverter]::ToString($sha.ComputeHash($bytes))).Replace('-','').ToLowerInvariant()}finally{$sha.Dispose()}
    if($digest-cne[string]$Config.d_source_digest){throw 'SOURCE_DRIFT:aggregate_digest'};return $digest
}
function Get-NxDLoadedState($Config) {
    $system=Invoke-RestMethod -Method Get -Uri ($Config.runtime.localai_url+'/system') -TimeoutSec 15
    foreach($loaded in @($system.loaded_models)){
        if([string]$loaded.id-ceq[string]$Config.model.id){return $loaded}
    }
    return $null
}
function Invoke-NxDUnload($Config) {
    $body=@{model=$Config.model.id}|ConvertTo-Json -Compress;Invoke-RestMethod -Method Post -Uri ($Config.runtime.localai_url+'/backend/shutdown') -ContentType 'application/json' -Body $body -TimeoutSec 30|Out-Null
}
function Assert-NxDHoldoutNotConsumed {
    $receipt=Resolve-NxDPath 'reports/nxb21/d-model-evaluation-receipt-v1.json'
    if(-not(Test-Path -LiteralPath $receipt)){return}
    $state=Get-Content -LiteralPath $receipt -Raw|ConvertFrom-Json
    if([bool]$state.holdout_consumed){throw 'HOLDOUT_ALREADY_CONSUMED:freeze a new independent holdout before another evaluation'}
}
function Assert-NxDBaseline($Config) {
    $digest=Test-NxDSourceManifest $Config;$c=Get-NxDContainers
    foreach($p in $Config.runtime.containers.PSObject.Properties){$a=$c[$p.Name];$e=$p.Value;if($a.Id-cne[string]$e.id-or$a.Image-cne[string]$e.image-or[int]$a.RestartCount-ne[int]$e.restarts){throw "RUNTIME_DRIFT:$($p.Name)"};if(-not(Test-NxDContainerHealthy $a)){throw "RUNTIME_UNHEALTHY:$($p.Name)"}}
    Assert-NxDHTTPHealth $Config
    $jobs=Get-NxDActiveJobs $c;if($jobs-ne 0){throw "ACTIVE_JOBS:$jobs"};$tuple=Get-NxDRetainedTuple $c;if($tuple-cne[string]$Config.runtime.retained_tuple){throw "RETAINED_DRIFT:$tuple"}
    $models=(Invoke-RestMethod -Method Get -Uri ($Config.runtime.localai_url+'/v1/models') -TimeoutSec 15).data.id;if($models-cnotcontains[string]$Config.model.id){throw 'MODEL_MISSING'}
    return [pscustomobject]@{source_digest=$digest;containers=$c;retained_tuple=$tuple;active_jobs=$jobs;activity_count=(Get-NxDActivityCount $c);models=@($models);ram_gib=Get-NxDRamGiB}
}
function Assert-NxDActivationReceipt($Receipt) {
    if(-not[bool]$Receipt.holdout_consumed-or[int]$Receipt.cases_completed-ne168-or$Receipt.DExitDecision-cne'PASS'-or[double]$Receipt.critical.percent-ne100-or$Receipt.suitability-cne'SUITABLE'-or$Receipt.runtime_postcheck-cne'PASS'){throw 'ACTIVATION_PREREQUISITE_FAILED:D evaluation gate'}
}
function Assert-NxDActivationArtifacts([string]$ReceiptPath,[string]$HashPath,[string]$SealPath) {
    if(-not(Test-Path -LiteralPath $ReceiptPath)-or-not(Test-Path -LiteralPath $HashPath)-or-not(Test-Path -LiteralPath $SealPath)){throw 'ACTIVATION_PREREQUISITE_FAILED:accepted D receipt/seal missing'}
}
