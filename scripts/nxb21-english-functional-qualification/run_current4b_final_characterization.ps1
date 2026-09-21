# SPDX-License-Identifier: MIT
[CmdletBinding()]
param([switch]$ValidateOnly)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$root = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$configPath = Join-Path $PSScriptRoot 'current4b-final-characterization-freeze.json'
$config = Get-Content -LiteralPath $configPath -Raw | ConvertFrom-Json
$privateRoot = Join-Path $root 'local-acceptance-models\nxb21-current4b-final-characterization'
$dispatchLock = Join-Path $privateRoot 'resource-lifecycle-soak.dispatched.lock'
$run = Join-Path $privateRoot ('run-' + [DateTime]::UtcNow.ToString('yyyyMMddTHHmmssfffZ'))
$observationsPath = Join-Path $run 'resource-lifecycle-observations.ndjson'
$receiptPath = Join-Path $run 'resource-lifecycle-receipt-v1.json'
$modelOwned = $false
$dispatchCreated = $false
$before = $null
$after = $null
$observations = @()
$batchResults = @()
$shapedResults = @()
$loadCount = 0
$reloadCount = 0
$finalState = 'VALIDATION_ONLY'
$exitCode = 40
$errorMessage = ''
$requestCount = 0
$callResults = @()
$unloadState = 'NOT_REQUIRED'
$integrityState = 'NOT_CHECKED'
$pagingSince = $null
$baselineSwapKiB = $null
$experimentTimer = [Diagnostics.Stopwatch]::StartNew()

function Resolve-RepoPath([string]$Relative) {
    return [IO.Path]::GetFullPath((Join-Path $root $Relative))
}

function Get-SHA256([string]$Path) {
    return (Get-FileHash -Algorithm SHA256 -LiteralPath $Path).Hash.ToLowerInvariant()
}

function Write-JSON([string]$Path, $Value) {
    $parent = Split-Path -Parent $Path
    if (-not (Test-Path -LiteralPath $parent)) { New-Item -ItemType Directory -Force -Path $parent | Out-Null }
    [IO.File]::WriteAllText($Path, ($Value | ConvertTo-Json -Depth 100) + "`n", [Text.UTF8Encoding]::new($false))
}

function Append-JSONLine([string]$Path, $Value) {
    [IO.File]::AppendAllText($Path, ($Value | ConvertTo-Json -Depth 30 -Compress) + "`n", [Text.UTF8Encoding]::new($false))
}

function Invoke-Native([string]$File, [string[]]$Arguments) {
    Push-Location $root
    try { $output = @(& $File @Arguments 2>&1); $code = $LASTEXITCODE }
    finally { Pop-Location }
    if ($code -ne 0) { throw "$File failed ($code): $($output -join [Environment]::NewLine)" }
    return ($output -join [Environment]::NewLine).Trim()
}

function Get-FreeRAMGiB {
    $os = Get-CimInstance Win32_OperatingSystem
    return [math]::Round(([int64]$os.FreePhysicalMemory * 1024) / 1GB, 3)
}

function Convert-MemoryToMiB([string]$Value) {
    if ($Value -notmatch '^\s*([0-9.]+)\s*([KMG]iB)\s*$') { return $null }
    $number = [double]$Matches[1]
    switch ($Matches[2]) { 'KiB' { return [math]::Round($number / 1024, 3) }; 'MiB' { return [math]::Round($number, 3) }; 'GiB' { return [math]::Round($number * 1024, 3) } }
}

function Get-APIContainerMemoryMiB {
    $stats = (Invoke-Native docker @('stats','--no-stream','--format','{{json .}}','nexusai-api-1')) | ConvertFrom-Json
    $used = ([string]$stats.MemUsage -split '/')[0].Trim()
    return Convert-MemoryToMiB $used
}

function Get-BackendMemory {
    $api = [string]$before.containers.'nexusai-api-1'.id
    $raw = Invoke-Native docker @('exec',$api,'ps','-eo','pid=,rss=,args=')
    $rows = @()
    foreach ($line in ($raw -split '\r?\n')) {
        if ($line -notmatch '^\s*([0-9]+)\s+([0-9]+)\s+(.+)$') { continue }
        $pidValue = [int]$Matches[1]; $rssKiB = [int64]$Matches[2]; $command = [string]$Matches[3]
        if ($command -notmatch 'llama-cpp-cpu-all') { continue }
        $rows += [ordered]@{pid=$pidValue;rss_mib=[math]::Round($rssKiB/1024,3);command=$command}
    }
    return @($rows)
}

function Get-VmmemWorkingSetMiB {
    $process = Get-Process -ErrorAction SilentlyContinue | Where-Object { $_.ProcessName -in @('vmmemWSL','vmmem') } | Sort-Object WorkingSet64 -Descending | Select-Object -First 1
    if ($null -eq $process) { return $null }
    return [math]::Round($process.WorkingSet64/1MB,3)
}

function Get-Telemetry([string]$Stage, [int]$CallIndex, [int]$BatchIndex) {
    $backend = @(Get-BackendMemory)
    # OrderedDictionary keys are not Measure-Object properties in Windows
    # PowerShell 5.1. Explicit accumulation also handles the unloaded state.
    $rssTotal = 0.0
    foreach ($row in $backend) { $rssTotal += [double]$row['rss_mib'] }
    $memory = Get-CimInstance Win32_PerfFormattedData_PerfOS_Memory
    $pagefile = @(Get-CimInstance Win32_PageFileUsage | ForEach-Object {
        [ordered]@{allocated_mib=$_.AllocatedBaseSize;used_mib=$_.CurrentUsage;peak_mib=$_.PeakUsage}
    })
    $stats = Invoke-Native docker @('stats','--no-stream','--format','{{json .}}')
    $serviceStats = @($stats -split '\r?\n' | ForEach-Object { $_ | ConvertFrom-Json } | Where-Object { $_.Name -in @($before.containers.Keys) })
    $linuxMemory = Invoke-Native wsl.exe @('--distribution','docker-desktop','--exec','cat','/proc/meminfo')
    $swapTotal = $null; $swapFree = $null
    if ($linuxMemory -match '(?m)^SwapTotal:\s+(\d+)') { $swapTotal = [int64]$Matches[1] }
    if ($linuxMemory -match '(?m)^SwapFree:\s+(\d+)') { $swapFree = [int64]$Matches[1] }
    if ($null -eq $swapTotal -or $null -eq $swapFree) { throw 'TELEMETRY_SWAP_UNAVAILABLE' }
    $compression = @(Get-Process -Name 'Memory Compression' -ErrorAction SilentlyContinue)
    $record = [ordered]@{
        utc = [DateTime]::UtcNow.ToString('o'); stage = $Stage; call_index = $CallIndex; batch_index = $BatchIndex
        available_ram_gib = Get-FreeRAMGiB; api_container_memory_mib = Get-APIContainerMemoryMiB
        vmmemwsl_working_set_mib = Get-VmmemWorkingSetMiB; backend_processes = $backend
        backend_rss_total_mib = [math]::Round($rssTotal, 3)
        committed_bytes = [int64]$memory.CommittedBytes; commit_limit_bytes = [int64]$memory.CommitLimit
        commit_reserve_gib = [math]::Round(($memory.CommitLimit - $memory.CommittedBytes)/1GB,3)
        pages_input_per_second = [int64]$memory.PagesInputPersec; pages_output_per_second = [int64]$memory.PagesOutputPersec
        pagefile = $pagefile; wsl_swap_used_kib = $swapTotal - $swapFree
        compression_working_set_mib = $(if($compression.Count -gt 0){[math]::Round($compression[0].WorkingSet64/1MB,3)}else{$null})
        service_stats = $serviceStats
        historical_4gib_guard_met = (Get-FreeRAMGiB) -ge 4.0
    }
    Append-JSONLine $observationsPath $record
    return $record
}

function Assert-ExperimentalSafety($Sample) {
    if ($experimentTimer.Elapsed.TotalSeconds -ge 1200) { throw 'EXPERIMENT_ABORT_DURATION_LIMIT' }
    # Experiment abort guards are not an approved operating envelope. The
    # existing isolated-evaluator headroom supplies the physical abort guard.
    if ($Sample.available_ram_gib -le 1.5) { throw 'EXPERIMENT_ABORT_PHYSICAL_RESERVE' }
    if ($Sample.commit_limit_bytes -le 0 -or $Sample.commit_reserve_gib -le 2.0) { throw 'EXPERIMENT_ABORT_COMMIT_RESERVE' }
    if ($null -eq $script:baselineSwapKiB) { $script:baselineSwapKiB = $Sample.wsl_swap_used_kib }
    if (($Sample.wsl_swap_used_kib - $script:baselineSwapKiB) -gt 262144) { throw 'EXPERIMENT_ABORT_SWAP_GROWTH' }
    if (($Sample.pages_input_per_second + $Sample.pages_output_per_second) -ge 1024) {
        if ($null -eq $script:pagingSince) { $script:pagingSince = [DateTime]::UtcNow }
        if (([DateTime]::UtcNow - $script:pagingSince).TotalSeconds -ge 15) { throw 'EXPERIMENT_ABORT_SUSTAINED_PAGING' }
    } else { $script:pagingSince = $null }
    $containers = Get-Containers
    foreach ($name in $containers.Keys) {
        $current = $containers[$name]; $original = $before.containers[$name]
        if ($current.oom_killed -or $current.status -ne 'running' -or $current.health -eq 'unhealthy' -or $current.id -ne $original.id -or $current.restart_count -ne $original.restart_count) { throw "EXPERIMENT_ABORT_SERVICE_STATE:$name" }
    }
}

function Get-Containers {
    $names = @('nexusai-api-1','nexusai-forensic-records-api-1','nexusai-forensic-records-worker-1','nexusai-forensic-postgres-1','nexusai-forensic-nats-1')
    $rows = Invoke-Native docker (@('inspect') + $names) | ConvertFrom-Json
    $map = [ordered]@{}
    foreach ($row in $rows) {
        $health = $row.State.PSObject.Properties['Health']
        $map[$row.Name.TrimStart('/')] = [ordered]@{id=[string]$row.Id;image=[string]$row.Image;restart_count=[int]$row.RestartCount;status=[string]$row.State.Status;health=$(if($null -ne $health -and $null -ne $health.Value){[string]$health.Value.Status}else{'none'});oom_killed=[bool]$row.State.OOMKilled}
    }
    return $map
}

function Get-AnalystIndexSHA256 {
    $client = New-Object System.Net.WebClient
    try { $client.Headers['Accept']='text/html'; $bytes=$client.DownloadData([string]$config.runtime.analyst_url) }
    finally { $client.Dispose() }
    $sha=[Security.Cryptography.SHA256]::Create(); try { return ([BitConverter]::ToString($sha.ComputeHash($bytes))).Replace('-','').ToLowerInvariant() } finally { $sha.Dispose() }
}

function Get-RuntimeState {
    $containers=Get-Containers
    foreach($name in $containers.Keys){$c=$containers[$name];if($c.status -cne 'running' -or $c.oom_killed -or $c.health -eq 'unhealthy'){throw "CONTAINER_UNHEALTHY:$name"}}
    $postgres=[string]$containers.'nexusai-forensic-postgres-1'.id
    $tupleSQL="SELECT (SELECT count(*) FROM forensic.evidence_items)||(chr(124))||(SELECT count(*) FROM forensic.evidence_versions)||(chr(124))||(SELECT count(*) FROM forensic.records_ingest_jobs)||(chr(124))||(SELECT count(*) FROM forensic.records)||(chr(124))||(SELECT count(*) FROM forensic.derived_artifacts)||(chr(124))||(SELECT count(*) FROM forensic.kb_collection_assets);"
    $system=Invoke-RestMethod -Uri ([string]$config.runtime.localai_url+'/system') -TimeoutSec 20
    return [ordered]@{containers=$containers;loaded_model_ids=@($system.loaded_models | ForEach-Object { [string]$_.id } | Sort-Object);retained_tuple=Invoke-Native docker @('exec',$postgres,'psql','-U','localrecall','-d','localrecall','-Atc',$tupleSQL);activity_count=[int](Invoke-Native docker @('exec',$postgres,'psql','-U','localrecall','-d','localrecall','-Atc','SELECT count(*) FROM public.agent_analysis_history;'));active_jobs=[int](Invoke-Native docker @('exec',$postgres,'psql','-U','localrecall','-d','localrecall','-Atc',"SELECT count(*) FROM forensic.records_ingest_jobs WHERE status NOT IN ('completed','dead_letter');"));analyst_index_sha256=Get-AnalystIndexSHA256}
}

function Assert-RuntimeBaseline($State) {
    if($State.retained_tuple -cne [string]$config.runtime.retained_tuple -or $State.activity_count -ne [int]$config.runtime.activity_count -or $State.active_jobs -ne 0 -or $State.analyst_index_sha256 -cne [string]$config.runtime.analyst_index_sha256){throw 'RUNTIME_BASELINE_DRIFT'}
}

function Assert-RuntimeUnchanged($A,$B) {
    if (($A.loaded_model_ids -join '|') -cne ($B.loaded_model_ids -join '|')) { throw 'UNRELATED_LOADED_MODEL_STATE_CHANGED' }
    if($A.retained_tuple -cne $B.retained_tuple -or $A.activity_count -ne $B.activity_count -or $B.active_jobs -ne 0 -or $A.analyst_index_sha256 -cne $B.analyst_index_sha256){throw 'RUNTIME_STATE_CHANGED'}
    foreach($name in $A.containers.Keys){$x=$A.containers[$name];$y=$B.containers[$name];if($x.id -cne $y.id -or $x.image -cne $y.image -or $x.restart_count -ne $y.restart_count){throw "CONTAINER_CHANGED:$name"}}
}

function Assert-FrozenIdentity {
    if((Get-SHA256 $PSCommandPath) -cne [string]$config.runner_sha256){throw 'RUNNER_HASH_DRIFT'}
    foreach($property in $config.files.PSObject.Properties){if((Get-SHA256 (Resolve-RepoPath $property.Name)) -cne [string]$property.Value){throw "FROZEN_FILE_DRIFT:$($property.Name)"}}
    $r2Lock=Resolve-RepoPath ([string]$config.consumed_r2.dispatch_lock)
    if(-not (Test-Path -LiteralPath $r2Lock) -or (Get-SHA256 $r2Lock) -cne [string]$config.consumed_r2.dispatch_lock_sha256){throw 'R2_CONSUMED_LOCK_DRIFT'}
}

function Assert-ModelIdentity {
    $api=[string]$before.containers.'nexusai-api-1'.id
    $artifact=((Invoke-Native docker @('exec',$api,'sha256sum',[string]$config.model.artifact_path))-split '\s+')[0]
    $profile=((Invoke-Native docker @('exec',$api,'sha256sum',[string]$config.model.profile_path))-split '\s+')[0]
    $bytes=[int64](Invoke-Native docker @('exec',$api,'stat','-c','%s',[string]$config.model.artifact_path))
    if($artifact -cne [string]$config.model.artifact_sha256 -or $profile -cne [string]$config.model.profile_sha256 -or $bytes -ne [int64]$config.model.artifact_bytes){throw 'MODEL_IDENTITY_DRIFT'}
}

function Get-LoadedState {
    $system=Invoke-RestMethod -Uri ([string]$config.runtime.localai_url+'/system') -TimeoutSec 20
    foreach($loaded in @($system.loaded_models)){if([string]$loaded.id -ceq [string]$config.model.id){return $loaded}}
    return $null
}

function Invoke-UnloadOwnedModel {
    if(-not $modelOwned){return}
    if ($null -eq (Get-LoadedState)) { $script:modelOwned=$false; $script:unloadState='ALREADY_UNLOADED'; return }
    $body=@{model=[string]$config.model.id}|ConvertTo-Json -Compress
    Invoke-RestMethod -Method Post -Uri ([string]$config.runtime.localai_url+'/backend/shutdown') -ContentType 'application/json' -Body $body -TimeoutSec 30|Out-Null
    Start-Sleep -Seconds 8
    if($null -ne (Get-LoadedState)){throw 'OWNED_MODEL_UNLOAD_FAILED'}
    $script:modelOwned=$false
    $script:unloadState='PASS'
}

function Invoke-GovernedCleanCacheReclaim {
    $dirty=Invoke-Native wsl.exe @('--distribution','docker-desktop','--user','root','--exec','grep','^Dirty:','/proc/meminfo')
    $writeback=Invoke-Native wsl.exe @('--distribution','docker-desktop','--user','root','--exec','grep','^Writeback:','/proc/meminfo')
    if($dirty -notmatch '^Dirty:\s+([0-9]+)\s+kB$'){throw 'CACHE_DIRTY_STATE_UNAVAILABLE'};$dirtyKiB=[int64]$Matches[1]
    if($writeback -notmatch '^Writeback:\s+([0-9]+)\s+kB$'){throw 'CACHE_WRITEBACK_STATE_UNAVAILABLE'};$writebackKiB=[int64]$Matches[1]
    if($writebackKiB -ne 0){throw "CACHE_RECLAIM_BLOCKED_WRITEBACK:$writebackKiB"}
    $result=Invoke-Native wsl.exe @('--distribution','docker-desktop','--user','root','--exec','/sbin/sysctl','-w','vm.drop_caches=1')
    if($result -notmatch 'vm.drop_caches\s*=\s*1'){throw 'CACHE_RECLAIM_FAILED'}
    Write-Host "CLEAN_FILE_CACHE_RECOVERY=PASS dirty_kib=$dirtyKiB writeback_kib=0 dirty_pages_preserved=true sync=false"
}

function Get-StableRAMWindow([string]$Stage,[double]$Floor,[int]$MaximumWaitSeconds=120) {
    $window=@();$timer=[Diagnostics.Stopwatch]::StartNew();$index=0
    while($timer.Elapsed.TotalSeconds -le $MaximumWaitSeconds){
        $index++;$value=Get-FreeRAMGiB;Write-Host "RAM_SAMPLE stage=$Stage index=$index available_gib=$value required_gib=$Floor"
        Append-JSONLine $observationsPath ([ordered]@{utc=[DateTime]::UtcNow.ToString('o');stage=$Stage;sample=$index;available_ram_gib=$value;required_gib=$Floor})
        if($value -ge $Floor){$window+=@($value);if($window.Count -gt 5){$window=@($window[($window.Count-5)..($window.Count-1)])}}else{$window=@()}
        if($window.Count -eq 5 -and ([double]$window[0]-[double]$window[4]) -le 0.5){return @($window)}
        Start-Sleep -Seconds 5
    }
    throw "RAM_STABLE_WINDOW_FAILED:$Stage required_gib=$Floor"
}

function Get-ReloadAdmissionGiB([double]$MeasuredDelta,[double]$Reserve,[double]$Margin) {
    if ($MeasuredDelta -le 0 -or $Reserve -le 0 -or $Margin -le 0) { throw 'RELOAD_ENVELOPE_INVALID' }
    return [math]::Ceiling(($MeasuredDelta+$Reserve+$Margin)*10)/10
}

function Test-ReloadSample($Sample) {
    $envelope=$config.reload_envelope
    $floor=Get-ReloadAdmissionGiB $envelope.measured_peak_delta_gib $envelope.selected_reserve_gib $envelope.load_margin_gib
    return ($Sample.available_ram_gib -ge $floor -and
        $Sample.commit_reserve_gib -ge $envelope.reload_commit_reserve_gib -and
        ($Sample.pages_input_per_second+$Sample.pages_output_per_second) -le $envelope.quiet_paging_pages_per_second -and
        ($Sample.wsl_swap_used_kib-$baselineSwapKiB) -le 262144 -and
        $Sample.backend_rss_total_mib -le ($envelope.unloaded_backend_rss_mib+256))
}

function Test-ReloadWindow($Window) {
    if (@($Window).Count -ne 5) { return $false }
    foreach ($sample in $Window) { if (-not (Test-ReloadSample $sample)) { return $false } }
    return (($Window[0].available_ram_gib-$Window[4].available_ram_gib) -le $config.reload_envelope.load_margin_gib -and
        ($Window[0].commit_reserve_gib-$Window[4].commit_reserve_gib) -le $config.reload_envelope.load_margin_gib)
}

function Get-StableReloadWindow {
    $window=@(); $timer=[Diagnostics.Stopwatch]::StartNew()
    $envelope=$config.reload_envelope
    $floor=Get-ReloadAdmissionGiB $envelope.measured_peak_delta_gib $envelope.selected_reserve_gib $envelope.load_margin_gib
    if ($floor -ne [double]$envelope.reload_admission_gib) { throw 'RELOAD_ENVELOPE_CALCULATION_DRIFT' }
    while ($timer.Elapsed.TotalSeconds -le 120) {
        $sample=Get-Telemetry 'pre_reload_envelope' 0 ($reloadCount+1)
        Assert-ExperimentalSafety $sample
        if ($null -ne (Get-LoadedState)) { throw 'RELOAD_REQUIRES_CLEAN_UNLOAD' }
        Write-Host "RELOAD_SAMPLE available_gib=$($sample.available_ram_gib) required_gib=$floor commit_reserve_gib=$($sample.commit_reserve_gib)"
        if (Test-ReloadSample $sample) {
            $window+=@($sample)
            if ($window.Count -gt 5) { $window=@($window[($window.Count-5)..($window.Count-1)]) }
        } else { $window=@() }
        if (Test-ReloadWindow $window) {
            $state=Get-RuntimeState
            Assert-RuntimeUnchanged $before $state
            return $window
        }
        Start-Sleep -Seconds 5
    }
    throw "RELOAD_ENVELOPE_ADMISSION_FAILED:required_gib=$floor"
}

function New-ExclusiveLock([string]$Path,[string]$Content) {
    $parent=Split-Path -Parent $Path;if(-not(Test-Path -LiteralPath $parent)){New-Item -ItemType Directory -Force -Path $parent|Out-Null}
    $stream=[IO.File]::Open($Path,[IO.FileMode]::CreateNew,[IO.FileAccess]::Write,[IO.FileShare]::Read)
    try{$bytes=[Text.Encoding]::UTF8.GetBytes($Content);$stream.Write($bytes,0,$bytes.Length);$stream.Flush($true)}finally{$stream.Dispose()}
}

function New-GenericBody([string]$Marker,[bool]$PayloadShaped,[bool]$SynthesisShaped=$false) {
    $choices=@($Marker)
    $user='Emit the fixed readiness marker now.'
    if($PayloadShaped){
        $choices=@(1..69|ForEach-Object{"CHOICE_$($_.ToString('000'))"})
        $marker=$choices[0]
        $items=@(1..66|ForEach-Object{[ordered]@{id="ITEM_$($_.ToString('000'))";group="GROUP_$((($_-1)%12)+1)";meaning=('Generic non-semantic resource descriptor '+('x'*85));requires=@('field_a','field_b')}})
        $user='Generic resource descriptors only; select the first issued choice. '+($items|ConvertTo-Json -Depth 5 -Compress)
    }
    $schema=[ordered]@{type='object';additionalProperties=$false;required=@('status');properties=[ordered]@{status=[ordered]@{type='string';enum=$choices}}}
    $maxTokens=24
    if ($SynthesisShaped) {
        $maxTokens=256
        $schema.required=@('status','summary')
        $schema.properties['summary']=[ordered]@{type='string'}
        $user+=' Also provide a 100-word neutral summary of these dummy inventory descriptors in the summary field. No real evidence is present.'
    }
    $body=[ordered]@{model=[string]$config.model.id;temperature=0;max_tokens=$maxTokens;response_format=[ordered]@{type='json_schema';json_schema=[ordered]@{name='resource_readiness';strict=$true;schema=$schema}};messages=@([ordered]@{role='system';content='This is a generic resource test. Return the requested JSON fields.'},[ordered]@{role='user';content=$user})}|ConvertTo-Json -Depth 20 -Compress
    return [ordered]@{marker=$marker;allowed=$choices;body=$body;request_bytes=[Text.Encoding]::UTF8.GetByteCount($body)}
}

function Invoke-ResourceCall([string]$Stage,[int]$CallIndex,[int]$BatchIndex,[bool]$PayloadShaped) {
    $request=New-GenericBody "READY_$CallIndex" $PayloadShaped ($PayloadShaped -and $CallIndex -eq 3)
    $pre=Get-Telemetry ($Stage+'_before') $CallIndex $BatchIndex
    Assert-ExperimentalSafety $pre
    $timer=[Diagnostics.Stopwatch]::StartNew()
    Add-Type -AssemblyName System.Net.Http
    $handler = New-Object System.Net.Http.HttpClientHandler
    $handler.AllowAutoRedirect = $false
    $client = New-Object System.Net.Http.HttpClient($handler)
    $client.Timeout = [TimeSpan]::FromSeconds(180)
    $content = New-Object System.Net.Http.StringContent([string]$request.body, [Text.Encoding]::UTF8, 'application/json')
    $cancellation = New-Object System.Threading.CancellationTokenSource
    $httpResponse = $null
    try {
        $script:modelOwned = $true
        $script:requestCount++
        Append-JSONLine $observationsPath ([ordered]@{stage='request_dispatched';call_index=$CallIndex;batch_index=$BatchIndex;request_bytes=$request.request_bytes;utc=[DateTime]::UtcNow.ToString('o')})
        $task = $client.PostAsync(([string]$config.runtime.localai_url+'/v1/chat/completions'), $content, $cancellation.Token)
        while (-not $task.IsCompleted) {
            Start-Sleep -Seconds 2
            $sample = Get-Telemetry ($Stage+'_inflight') $CallIndex $BatchIndex
            Assert-ExperimentalSafety $sample
            if ($timer.Elapsed.TotalSeconds -ge 180) { throw 'RESOURCE_CALL_TIMEOUT' }
        }
        $httpResponse = $task.GetAwaiter().GetResult()
        $null = $httpResponse.EnsureSuccessStatusCode()
        $rawResponse = $httpResponse.Content.ReadAsStringAsync().GetAwaiter().GetResult()
        $response = $rawResponse | ConvertFrom-Json
    } catch {
        $cancellation.Cancel()
        Append-JSONLine $observationsPath ([ordered]@{stage='request_failed';call_index=$CallIndex;latency_ms=$timer.ElapsedMilliseconds;error=$_.Exception.Message;utc=[DateTime]::UtcNow.ToString('o')})
        throw
    } finally {
        if ($null -ne $httpResponse) { $httpResponse.Dispose() }
        $content.Dispose(); $client.Dispose(); $handler.Dispose(); $cancellation.Dispose()
    }
    $timer.Stop()
    if(@($response.choices).Count -ne 1 -or [string]$response.choices[0].finish_reason -cne 'stop'){throw "RESOURCE_CALL_INCOMPLETE:$Stage"}
    $decoded=[string]$response.choices[0].message.content|ConvertFrom-Json
    if([string]$decoded.status -cne [string]$request.marker){throw "RESOURCE_CALL_INVALID:$Stage"}
    if ($PayloadShaped -and $CallIndex -eq 3 -and [string]::IsNullOrWhiteSpace([string]$decoded.summary)) { throw 'SYNTHESIS_SHAPED_OUTPUT_EMPTY' }
    $post=Get-Telemetry ($Stage+'_after') $CallIndex $BatchIndex
    Assert-ExperimentalSafety $post
    $result=[ordered]@{stage=$Stage;call_index=$CallIndex;batch_index=$BatchIndex;payload_shaped=$PayloadShaped;request_bytes=$request.request_bytes;latency_ms=$timer.ElapsedMilliseconds;pre=$pre;post=$post;ram_drift_gib=[math]::Round(([double]$post.available_ram_gib-[double]$pre.available_ram_gib),3);api_memory_drift_mib=[math]::Round(([double]$post.api_container_memory_mib-[double]$pre.api_container_memory_mib),3);backend_rss_drift_mib=[math]::Round(([double]$post.backend_rss_total_mib-[double]$pre.backend_rss_total_mib),3);vmmem_drift_mib=[math]::Round(([double]$post.vmmemwsl_working_set_mib-[double]$pre.vmmemwsl_working_set_mib),3)}
    $usage=$response.PSObject.Properties['usage']
    $result['token_usage']=$(if($null -ne $usage){$usage.Value}else{$null})
    Write-Host "RESOURCE_CALL=PASS stage=$Stage call=$CallIndex batch=$BatchIndex request_bytes=$($request.request_bytes) latency_ms=$($timer.ElapsedMilliseconds) ram_after_gib=$($post.available_ram_gib)"
    $script:callResults += @($result)
    Append-JSONLine $observationsPath ([ordered]@{stage='request_completed';result=$result})
    return $result
}

function Start-OwnedModel([bool]$IsReload,[bool]$PreloadAlreadyAdmitted) {
    if(-not $PreloadAlreadyAdmitted){
        Invoke-GovernedCleanCacheReclaim
        $null=Get-StableReloadWindow
    }
    $script:loadCount++
    if($IsReload){$script:reloadCount++}
    $null=Invoke-ResourceCall 'model_load' (1000+$loadCount) $loadCount $false
    if($null -eq (Get-LoadedState)){throw 'CURRENT_4B_MODEL_NOT_LOADED'}
    Start-Sleep -Seconds 10
    $sample=Get-Telemetry 'post_reload' 0 $loadCount
    Assert-ExperimentalSafety $sample
}

try {
    Assert-FrozenIdentity
    $before=Get-RuntimeState
    Assert-RuntimeBaseline $before
    Assert-ModelIdentity
    if($null -ne (Get-LoadedState)){throw 'CURRENT_4B_MUST_BE_UNLOADED_BEFORE_SOAK'}
} catch {
    Write-JSON $receiptPath ([ordered]@{contract_version='nexusai.current4b-runtime-characterization/v2';final_state='PREFLIGHT_FAILURE';error=$_.Exception.Message;request_count=0;quality_evidence='NONE'})
    Write-Host "PREFLIGHT_FAILURE_RECEIPT=$receiptPath"
    exit 10
}
if($ValidateOnly){
    if(Test-Path -LiteralPath $dispatchLock){throw 'RESOURCE_LIFECYCLE_SOAK_ALREADY_DISPATCHED'}
    New-Item -ItemType Directory -Force -Path $run | Out-Null
    $sample=Get-Telemetry 'validation_only_no_inference' 0 0
    Assert-ExperimentalSafety $sample
    Write-JSON (Join-Path $run 'validation-only.json') ([ordered]@{state='PASS';live_inference=$false;runtime=$before;telemetry=$sample})
    Write-Host 'DISPOSABLE_RESOURCE_SOAK_VALIDATION=PASS';Write-Host 'LIVE_INFERENCE=false';Write-Host 'R2_CORPUS=CONSUMED_DO_NOT_RERUN';Write-Host 'R3_CORPUS=NOT_CREATED';Write-Host 'AUTO_KILL_USER_OR_SYSTEM_PROCESS=NO';exit 0
}

New-Item -ItemType Directory -Force -Path $run|Out-Null
try{
    if(Test-Path -LiteralPath $dispatchLock){throw 'RESOURCE_LIFECYCLE_SOAK_ALREADY_DISPATCHED'}
    Invoke-GovernedCleanCacheReclaim
    $null=Get-StableRAMWindow 'initial_preload' ([double]$config.resource.admission_floor_gib)
    New-ExclusiveLock $dispatchLock ("soak_id=$($config.soak_id)`ndispatched_at=$([DateTime]::UtcNow.ToString('o'))`nquality_evidence=NONE`n")
    $dispatchCreated=$true
    $globalCall=0;$batchIndex=0
    foreach($batchSize in @($config.batch_plan)){
        $batchIndex++
        if($batchIndex -eq 1){Start-OwnedModel $false $true}else{Invoke-UnloadOwnedModel;Start-OwnedModel $true $false}
        $startRAM=Get-FreeRAMGiB
        $calls=@()
        for($i=1;$i -le [int]$batchSize;$i++){$globalCall++;$calls+=@(Invoke-ResourceCall 'tiny' $globalCall $batchIndex $false)}
        $endRAM=Get-FreeRAMGiB
        $batchResults+=@([ordered]@{batch_index=$batchIndex;batch_size=[int]$batchSize;start_ram_gib=$startRAM;end_ram_gib=$endRAM;drift_gib=[math]::Round(($endRAM-$startRAM),3);calls=$calls})
    }
    Invoke-UnloadOwnedModel;Start-OwnedModel $true $false
    for($i=1;$i -le [int]$config.payload_shaped_calls;$i++){$shapedResults+=@(Invoke-ResourceCall 'payload_shaped' $i ($batchIndex+1) $true)}
    $finalState='CHARACTERIZATION_COMPLETED_ADJUDICATION_REQUIRED';$exitCode=0
}
catch{
    $errorMessage=$_.Exception.Message
    $finalState=$(if($requestCount -gt 0){'CHARACTERIZATION_INCOMPLETE'}else{'HARNESS_OR_PRECONDITION_FAILURE_NO_INFERENCE'})
    $exitCode=$(if($dispatchCreated){20}else{10})
    Write-Host "FINAL_STATE=$finalState";Write-Host "ERROR=$errorMessage"
}
finally{
    try{Invoke-UnloadOwnedModel;Write-Host "CURRENT_4B_MODEL_UNLOAD=$unloadState"}catch{$unloadState='FAIL';$exitCode=40;$finalState='UNLOAD_FAILURE';Write-Host "CURRENT_4B_MODEL_UNLOAD=FAIL error=$($_.Exception.Message)"}
    try {
        $recovery=Get-Telemetry 'final_unload_recovery' 0 $reloadCount
        Assert-ExperimentalSafety $recovery
    } catch { $exitCode=40; $finalState='FINAL_RECOVERY_CHECK_FAILED'; $errorMessage=$_.Exception.Message }
    try{$after=Get-RuntimeState;Assert-RuntimeUnchanged $before $after;$integrityState='PASS';Write-Host 'RUNTIME_INTEGRITY=PASS'}catch{$integrityState='FAIL';Write-Host "RUNTIME_INTEGRITY=FAIL error=$($_.Exception.Message)";$exitCode=40;$finalState='RUNTIME_INTEGRITY_FAIL'}
    $receipt=[ordered]@{contract_version='nexusai.current4b-resource-lifecycle-soak/v1';soak_id=[string]$config.soak_id;final_state=$finalState;exit_code=$exitCode;error=$errorMessage;completed_at=[DateTime]::UtcNow.ToString('o');quality_evidence='NONE';r2_corpus_state='CONSUMED_DO_NOT_RERUN';r3_corpus_state='NOT_CREATED';dispatch_created=$dispatchCreated;batch_plan=@($config.batch_plan);tiny_calls_planned=[int]$config.tiny_calls;payload_shaped_calls_planned=[int]$config.payload_shaped_calls;load_count=$loadCount;reload_count=$reloadCount;batch_results=$batchResults;payload_shaped_results=$shapedResults;runtime_before=$before;runtime_after=$after;observations_path=$observationsPath;auto_kill_user_or_system_process='NO'}
    $receipt.contract_version='nexusai.current4b-runtime-characterization/v2'
    $receipt['request_count']=$requestCount; $receipt['completed_calls']=$callResults
    $receipt['unload_state']=$unloadState; $receipt['runtime_integrity']=$integrityState
    $receipt['safe_resource_envelope']='NOT_ADOPTED_REQUIRES_MEASUREMENT_ADJUDICATION'
    $receipt['reload_envelope']=$config.reload_envelope
    $receipt['final_attempt_policy']='PASS_KEEP_RUNTIME_BASELINE_STRICT_PENDING_OR_FAIL_REJECT_NO_FURTHER_4B_RESOURCE_ATTEMPT'
    $receipt['model_functional_baseline']='NOT_QUALIFIED_BY_NONSEMANTIC_WORKLOAD'
    Write-JSON $receiptPath $receipt;[IO.File]::WriteAllText($receiptPath+'.sha256',(Get-SHA256 $receiptPath)+"`n",[Text.UTF8Encoding]::new($false));Write-Host "RESOURCE_SOAK_RECEIPT=$receiptPath"
}
if($exitCode -eq 0){Write-Host "FINAL_STATE=$finalState"}
exit $exitCode
