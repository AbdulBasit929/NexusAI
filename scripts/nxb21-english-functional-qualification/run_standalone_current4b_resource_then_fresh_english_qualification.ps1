# SPDX-License-Identifier: MIT
[CmdletBinding()]
param(
    [switch]$ValidateOnly
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$root = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$configPath = Join-Path $PSScriptRoot 'standalone-current4b-r2-freeze-v1.json'
$config = Get-Content -LiteralPath $configPath -Raw | ConvertFrom-Json
$privateRoot = Join-Path $root 'local-acceptance-models\nxb21-english-functional-qualification-r2'
$resourceLock = Join-Path $privateRoot 'current4b-resource-smoke-r2.dispatched.lock'
$qualificationLock = Join-Path $privateRoot 'english-functional-corpus-r2.dispatched.lock'
$run = Join-Path $privateRoot ('run-' + [DateTime]::UtcNow.ToString('yyyyMMddTHHmmssfffZ'))
$receiptPath = Join-Path $run 'standalone-operator-receipt-v1.json'
$resourcePath = Join-Path $run 'resource-observations.ndjson'
$stdoutPath = Join-Path $run 'evaluator.stdout.log'
$stderrPath = Join-Path $run 'evaluator.stderr.log'
$qualificationResultPath = Join-Path $run 'qualification-intermediate-v1.json'
$resourceLockCreated = $false
$modelOwned = $false
$qualificationStarted = $false
$qualificationRAMFailure = $false
$finalState = 'PREPARATION_VALIDATION'
$exitCode = 40
$errorMessage = ''
$before = $null
$after = $null
$recoverySamples = @()
$preloadSamples = @()
$qualifyingThresholdGiB = $null
$qualifyingPolicy = $null
$recoveryBestStableMinGiB = $null
$predictedLoadedGiB = $null
$postLoadSamples = @()
$postSoakSamples = @()
$transportCalls = @()
$apiMemory = $null
$backendMemory = @()
$vmmemWorkingSetMiB = $null
$evaluatorPeakMiB = 0.0

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
    [IO.File]::AppendAllText($Path, ($Value | ConvertTo-Json -Depth 20 -Compress) + "`n", [Text.UTF8Encoding]::new($false))
}

function Invoke-Native([string]$File, [string[]]$Arguments) {
    Push-Location $root
    try {
        $output = @(& $File @Arguments 2>&1)
        $code = $LASTEXITCODE
    }
    finally { Pop-Location }
    if ($code -ne 0) { throw "$File failed ($code): $($output -join [Environment]::NewLine)" }
    return ($output -join [Environment]::NewLine).Trim()
}

function Get-FreeRAMGiB {
    $os = Get-CimInstance Win32_OperatingSystem
    return [math]::Round(([int64]$os.FreePhysicalMemory * 1024) / 1GB, 3)
}

function Get-HighMemoryProcesses {
    return @(Get-Process -ErrorAction SilentlyContinue |
        Sort-Object WorkingSet64 -Descending |
        Select-Object -First 15 |
        ForEach-Object { [ordered]@{process_name=$_.ProcessName; pid=$_.Id; working_set_mib=[math]::Round($_.WorkingSet64/1MB,1)} })
}

function Write-HighMemoryReport {
    Write-Host 'HIGH_MEMORY_PROCESS_REPORT=READ_ONLY'
    foreach ($process in Get-HighMemoryProcesses) {
        Write-Host "PROCESS_NAME=$($process.process_name) PID=$($process.pid) WORKING_SET_MIB=$($process.working_set_mib)"
    }
}

function Get-Containers {
    $names = @('nexusai-api-1', 'nexusai-forensic-records-api-1', 'nexusai-forensic-records-worker-1', 'nexusai-forensic-postgres-1', 'nexusai-forensic-nats-1')
    $rows = Invoke-Native docker (@('inspect') + $names) | ConvertFrom-Json
    $map = [ordered]@{}
    foreach ($row in $rows) {
        $healthProperty = $row.State.PSObject.Properties['Health']
        $name = $row.Name.TrimStart('/')
        $map[$name] = [ordered]@{
            id = [string]$row.Id
            image = [string]$row.Image
            restart_count = [int]$row.RestartCount
            status = [string]$row.State.Status
            health = $(if ($null -ne $healthProperty -and $null -ne $healthProperty.Value) { [string]$healthProperty.Value.Status } else { 'none' })
            oom_killed = [bool]$row.State.OOMKilled
        }
    }
    return $map
}

function Assert-ContainersHealthy($Containers) {
    foreach ($name in $Containers.Keys) {
        $container = $Containers[$name]
        if ($container.status -cne 'running' -or $container.oom_killed -or $container.health -eq 'unhealthy') {
            throw "CONTAINER_UNHEALTHY:$name"
        }
    }
}

function Get-AnalystIndexSHA256 {
    $client = New-Object System.Net.WebClient
    try {
        $client.Headers['Accept'] = 'text/html'
        $bytes = $client.DownloadData([string]$config.runtime.analyst_url)
    }
    finally { $client.Dispose() }
    $sha = [Security.Cryptography.SHA256]::Create()
    try { return ([BitConverter]::ToString($sha.ComputeHash($bytes))).Replace('-', '').ToLowerInvariant() }
    finally { $sha.Dispose() }
}

function Get-RuntimeState {
    $containers = Get-Containers
    Assert-ContainersHealthy $containers
    $postgres = $containers['nexusai-forensic-postgres-1'].id
    $tupleSQL = "SELECT (SELECT count(*) FROM forensic.evidence_items)||(chr(124))||(SELECT count(*) FROM forensic.evidence_versions)||(chr(124))||(SELECT count(*) FROM forensic.records_ingest_jobs)||(chr(124))||(SELECT count(*) FROM forensic.records)||(chr(124))||(SELECT count(*) FROM forensic.derived_artifacts)||(chr(124))||(SELECT count(*) FROM forensic.kb_collection_assets);"
    $jobsSQL = "SELECT count(*) FROM forensic.records_ingest_jobs WHERE status NOT IN ('completed','dead_letter');"
    return [ordered]@{
        containers = $containers
        retained_tuple = Invoke-Native docker @('exec', $postgres, 'psql', '-U', 'localrecall', '-d', 'localrecall', '-Atc', $tupleSQL)
        activity_count = [int](Invoke-Native docker @('exec', $postgres, 'psql', '-U', 'localrecall', '-d', 'localrecall', '-Atc', 'SELECT count(*) FROM public.agent_analysis_history;'))
        active_jobs = [int](Invoke-Native docker @('exec', $postgres, 'psql', '-U', 'localrecall', '-d', 'localrecall', '-Atc', $jobsSQL))
        analyst_index_sha256 = Get-AnalystIndexSHA256
    }
}

function Assert-RuntimeBaseline($State) {
    if ($State.retained_tuple -cne [string]$config.runtime.retained_tuple) { throw "RETAINED_TUPLE_DRIFT:$($State.retained_tuple)" }
    if ($State.activity_count -ne [int]$config.runtime.activity_count) { throw "ACTIVITY_DRIFT:$($State.activity_count)" }
    if ($State.active_jobs -ne 0) { throw "ACTIVE_JOBS:$($State.active_jobs)" }
    if ($State.analyst_index_sha256 -cne [string]$config.runtime.analyst_index_sha256) { throw 'ANALYST_UI_DRIFT' }
}

function Assert-RuntimeUnchanged($Before, $After) {
    if ($Before.retained_tuple -cne $After.retained_tuple) { throw 'RETAINED_TUPLE_CHANGED' }
    if ($Before.activity_count -ne $After.activity_count) { throw 'ACTIVITY_CHANGED' }
    if ($After.active_jobs -ne 0) { throw "ACTIVE_JOBS:$($After.active_jobs)" }
    if ($Before.analyst_index_sha256 -cne $After.analyst_index_sha256) { throw 'ANALYST_UI_CHANGED' }
    foreach ($name in $Before.containers.Keys) {
        $a = $Before.containers[$name]
        $b = $After.containers[$name]
        if ($a.id -cne $b.id -or $a.image -cne $b.image -or $a.restart_count -ne $b.restart_count) { throw "CONTAINER_CHANGED:$name" }
    }
}

function Assert-FrozenFiles {
    foreach ($property in $config.files.PSObject.Properties) {
        $actual = Get-SHA256 (Resolve-RepoPath $property.Name)
        if ($actual -cne [string]$property.Value) { throw "FROZEN_FILE_DRIFT:$($property.Name)" }
    }
    $runnerHash = Get-SHA256 $PSCommandPath
    if ($runnerHash -cne [string]$config.runner_sha256) { throw 'RUNNER_HASH_DRIFT' }
    $freshness = Get-Content -LiteralPath (Resolve-RepoPath ([string]$config.freshness.path)) -Raw | ConvertFrom-Json
    if ([string]$freshness.status -cne 'PASS' -or [int]$freshness.question_overlap -ne 0 -or [int]$freshness.normalized_literal_overlap -ne 0 -or [int]$freshness.prior_corpus_overlap -ne 0) { throw 'FROZEN_FRESHNESS_RECEIPT_INVALID' }
}

function Assert-ModelIdentity($State) {
    $api = $State.containers['nexusai-api-1'].id
    $artifactHash = ((Invoke-Native docker @('exec', $api, 'sha256sum', [string]$config.model.artifact_path)) -split '\s+')[0]
    $profileHash = ((Invoke-Native docker @('exec', $api, 'sha256sum', [string]$config.model.profile_path)) -split '\s+')[0]
    $artifactBytes = [int64](Invoke-Native docker @('exec', $api, 'stat', '-c', '%s', [string]$config.model.artifact_path))
    if ($artifactHash -cne [string]$config.model.artifact_sha256 -or $profileHash -cne [string]$config.model.profile_sha256 -or $artifactBytes -ne [int64]$config.model.artifact_bytes) { throw 'MODEL_IDENTITY_DRIFT' }
    $registered = @((Invoke-RestMethod -Uri ([string]$config.runtime.localai_url + '/v1/models') -TimeoutSec 20).data.id)
    if ($registered -notcontains [string]$config.model.id) { throw 'MODEL_NOT_REGISTERED' }
}

function Get-LoadedState {
    $system = Invoke-RestMethod -Uri ([string]$config.runtime.localai_url + '/system') -TimeoutSec 20
    foreach ($loaded in @($system.loaded_models)) {
        if ([string]$loaded.id -ceq [string]$config.model.id) { return $loaded }
    }
    return $null
}

function Invoke-UnloadCurrentModel {
    $body = @{model=[string]$config.model.id} | ConvertTo-Json -Compress
    Invoke-RestMethod -Method Post -Uri ([string]$config.runtime.localai_url + '/backend/shutdown') -ContentType 'application/json' -Body $body -TimeoutSec 30 | Out-Null
}

function Invoke-GovernedCleanCacheReclaim {
    $dirty = Invoke-Native wsl.exe @('--distribution', 'docker-desktop', '--user', 'root', '--exec', 'grep', '^Dirty:', '/proc/meminfo')
    $writeback = Invoke-Native wsl.exe @('--distribution', 'docker-desktop', '--user', 'root', '--exec', 'grep', '^Writeback:', '/proc/meminfo')
    if ($dirty -notmatch '^Dirty:\s+([0-9]+)\s+kB$') { throw 'CACHE_DIRTY_STATE_UNAVAILABLE' }
    $dirtyKiB = [int64]$Matches[1]
    if ($writeback -notmatch '^Writeback:\s+([0-9]+)\s+kB$') { throw 'CACHE_WRITEBACK_STATE_UNAVAILABLE' }
    $writebackKiB = [int64]$Matches[1]
    if ($writebackKiB -ne 0) { throw "CACHE_RECLAIM_BLOCKED_WRITEBACK:$writebackKiB" }
    $result = Invoke-Native wsl.exe @('--distribution', 'docker-desktop', '--user', 'root', '--exec', '/sbin/sysctl', '-w', 'vm.drop_caches=1')
    if ($result -notmatch 'vm.drop_caches\s*=\s*1') { throw 'CACHE_RECLAIM_FAILED' }
    Write-Host "CLEAN_FILE_CACHE_RECOVERY=PASS dirty_kib=$dirtyKiB writeback_kib=0 dirty_pages_preserved=true sync=false"
}

function Get-StableRAMSamples([string]$Stage, [double]$Floor, [int]$Count=5, [int]$SpacingSeconds=5) {
    $samples = @()
    for ($index = 0; $index -lt $Count; $index++) {
        $value = Get-FreeRAMGiB
        $samples += $value
        Write-Host "RAM_SAMPLE stage=$Stage index=$($index+1) available_gib=$value required_gib=$Floor"
        Append-JSONLine $resourcePath ([ordered]@{utc=[DateTime]::UtcNow.ToString('o');stage=$Stage;sample=$index+1;available_ram_gib=$value;required_gib=$Floor})
        if ($index -lt ($Count - 1)) { Start-Sleep -Seconds $SpacingSeconds }
    }
    return @($samples)
}

function Assert-SampleFloor([double[]]$Samples, [double]$Floor, [string]$Stage) {
    $minimum = [double]($Samples | Measure-Object -Minimum).Minimum
    if ($minimum -lt $Floor) { throw "RAM_FLOOR_FAILED:$Stage minimum_gib=$minimum required_gib=$Floor" }
}

function Test-SampleTrend([double[]]$Samples, [double]$MaximumDeclineGiB) {
    if ($Samples.Count -lt 2) { return $true }
    $decline = [double]$Samples[0] - [double]$Samples[$Samples.Count - 1]
    return $decline -le $MaximumDeclineGiB
}

function Assert-StableSampleWindow([double[]]$Samples, [double]$Floor, [string]$Stage, [double]$MaximumDeclineGiB) {
    Assert-SampleFloor $Samples $Floor $Stage
    if (-not (Test-SampleTrend $Samples $MaximumDeclineGiB)) {
        $decline = [math]::Round(([double]$Samples[0] - [double]$Samples[$Samples.Count - 1]), 3)
        throw "RAM_TREND_FAILED:$Stage decline_gib=$decline maximum_decline_gib=$MaximumDeclineGiB"
    }
}

function Get-BestStableWindowMinimum([double[]]$Samples, [int]$Count, [double]$MaximumDeclineGiB) {
    if ($Samples.Count -lt $Count) { return $null }
    $best = $null
    for ($start = 0; $start -le ($Samples.Count - $Count); $start++) {
        $window = @($Samples[$start..($start + $Count - 1)])
        if (-not (Test-SampleTrend $window $MaximumDeclineGiB)) { continue }
        $minimum = [double]($window | Measure-Object -Minimum).Minimum
        if ($null -eq $best -or $minimum -gt [double]$best) { $best = $minimum }
    }
    return $best
}

function Get-RecoveryRAMSamples([int]$MaximumWaitSeconds, [int]$SpacingSeconds) {
    $samples = @()
    $timer = [Diagnostics.Stopwatch]::StartNew()
    $index = 0
    while ($true) {
        $index++
        $value = Get-FreeRAMGiB
        $samples += $value
        $elapsed = [math]::Round($timer.Elapsed.TotalSeconds, 1)
        Write-Host "RAM_RECOVERY_SAMPLE index=$index elapsed_seconds=$elapsed available_gib=$value diagnostic_only=true"
        Append-JSONLine $resourcePath ([ordered]@{utc=[DateTime]::UtcNow.ToString('o');stage='preload_recovery';sample=$index;elapsed_seconds=$elapsed;available_ram_gib=$value;diagnostic_only=$true})
        if ($samples.Count -ge [int]$config.resource.state_machine.qualifying_sample_count) {
            $best = Get-BestStableWindowMinimum $samples ([int]$config.resource.state_machine.qualifying_sample_count) ([double]$config.resource.state_machine.maximum_window_decline_gib)
            if ($null -ne $best -and [double]$best -ge [double]$config.resource.empirical_advisory_preload_target_gib) {
                Write-Host "RECOVERY_ADVISORY_TRIGGER=PASS best_stable_min_gib=$best"
                break
            }
        }
        if ($timer.Elapsed.TotalSeconds -ge $MaximumWaitSeconds) { break }
        $remaining = $MaximumWaitSeconds - $timer.Elapsed.TotalSeconds
        Start-Sleep -Seconds ([math]::Max(1, [math]::Min($SpacingSeconds, [math]::Ceiling($remaining))))
    }
    $timer.Stop()
    return @($samples)
}

function Get-PreloadAdmission {
    $formalFloor = [double]$config.resource.preload_formal_floor_gib
    $loadedFloor = [double]$config.resource.loaded_formal_floor_gib
    $loadDelta = [double]$config.resource.observed_load_delta_gib
    $advisoryTarget = [double]$config.resource.empirical_advisory_preload_target_gib
    $maximumDecline = [double]$config.resource.state_machine.maximum_window_decline_gib
    $sampleCount = [int]$config.resource.state_machine.qualifying_sample_count
    $recovery = @(Get-RecoveryRAMSamples ([int]$config.resource.state_machine.maximum_recovery_wait_seconds) ([int]$config.resource.state_machine.recovery_sample_spacing_seconds))
    $bestStableMinimum = Get-BestStableWindowMinimum $recovery $sampleCount $maximumDecline
    $script:recoverySamples = @($recovery)
    $script:recoveryBestStableMinGiB = $bestStableMinimum
    Write-Host "RECOVERY_SAMPLES_GIB=$($recovery -join ',')"
    Write-Host "RECOVERY_BEST_STABLE_MIN_GIB=$(if ($null -eq $bestStableMinimum) { 'NONE' } else { $bestStableMinimum })"
    Write-Host "FORMAL_PRELOAD_FLOOR_GIB=$formalFloor"
    Write-Host "FORMAL_LOADED_FLOOR_GIB=$loadedFloor"
    Write-Host "OBSERVED_MODEL_LOAD_DELTA_GIB=$loadDelta"
    Write-Host "EMPIRICAL_ADVISORY_PRELOAD_TARGET_GIB=$advisoryTarget"
    if ($null -eq $bestStableMinimum) {
        throw 'PREDICTED_RESOURCE_INSUFFICIENT:no_stable_recovery_window'
    }

    $credibleThreshold = [math]::Round([math]::Max($formalFloor, ($loadedFloor + $loadDelta)), 3)
    $policy = 'PREFERRED_EMPIRICAL_ADMISSION'
    $threshold = $advisoryTarget
    if ([double]$bestStableMinimum -lt $advisoryTarget) {
        $predictedFromBest = [math]::Round(([double]$bestStableMinimum - $loadDelta), 3)
        $script:predictedLoadedGiB = $predictedFromBest
        if ([double]$bestStableMinimum -lt $formalFloor -or $predictedFromBest -lt $loadedFloor) {
            throw "PREDICTED_RESOURCE_INSUFFICIENT:best_stable_preload_gib=$bestStableMinimum observed_load_delta_gib=$loadDelta predicted_loaded_gib=$predictedFromBest required_loaded_gib=$loadedFloor"
        }
        $policy = 'CREDIBLE_PREDICTED_LOADED_FLOOR'
        $threshold = $credibleThreshold
    }

    $script:qualifyingPolicy = $policy
    $script:qualifyingThresholdGiB = $threshold
    Write-Host "QUALIFYING_POLICY=$policy"
    Write-Host "QUALIFYING_THRESHOLD_GIB=$threshold"
    $qualifying = @(Get-StableRAMSamples 'preload_qualifying' $threshold $sampleCount ([int]$config.resource.state_machine.qualifying_sample_spacing_seconds))
    $script:preloadSamples = @($qualifying)
    Assert-StableSampleWindow $qualifying $threshold 'preload_qualifying' $maximumDecline
    $qualifyingMinimum = [double]($qualifying | Measure-Object -Minimum).Minimum
    $predictedLoaded = [math]::Round(($qualifyingMinimum - $loadDelta), 3)
    $script:predictedLoadedGiB = $predictedLoaded
    if ($predictedLoaded -lt $loadedFloor) {
        throw "PREDICTED_RESOURCE_INSUFFICIENT:qualifying_min_gib=$qualifyingMinimum observed_load_delta_gib=$loadDelta predicted_loaded_gib=$predictedLoaded required_loaded_gib=$loadedFloor"
    }
    return [ordered]@{
        recovery_samples = $recovery
        recovery_best_stable_min_gib = [double]$bestStableMinimum
        qualifying_policy = $policy
        qualifying_threshold_gib = [double]$threshold
        qualifying_samples = $qualifying
        qualifying_min_gib = $qualifyingMinimum
        predicted_loaded_gib = $predictedLoaded
    }
}

function Assert-ResourceStateMachineContract {
    $maximumDecline = [double]$config.resource.state_machine.maximum_window_decline_gib
    $count = [int]$config.resource.state_machine.qualifying_sample_count
    $transient = @(4.304, 4.301, 5.441, 7.233, 8.418)
    $transientBest = Get-BestStableWindowMinimum $transient $count $maximumDecline
    if ([math]::Round(([double]$transientBest - [double]$config.resource.observed_load_delta_gib), 3) -ge [double]$config.resource.loaded_formal_floor_gib) { throw 'STATE_MACHINE_TRANSIENT_SEQUENCE_WRONGLY_ADMITTED' }
    $advisory = @(9.31, 9.28, 9.25, 9.22, 9.20)
    if ((Get-BestStableWindowMinimum $advisory $count $maximumDecline) -lt [double]$config.resource.empirical_advisory_preload_target_gib) { throw 'STATE_MACHINE_ADVISORY_SEQUENCE_REJECTED' }
    $declining = @(9.80, 9.60, 9.40, 9.25, 9.15)
    if (Test-SampleTrend $declining $maximumDecline) { throw 'STATE_MACHINE_DECLINING_SEQUENCE_ADMITTED' }
}

function New-ExclusiveLock([string]$Path, [string]$Content) {
    $parent = Split-Path -Parent $Path
    if (-not (Test-Path -LiteralPath $parent)) { New-Item -ItemType Directory -Force -Path $parent | Out-Null }
    $stream = [IO.File]::Open($Path, [IO.FileMode]::CreateNew, [IO.FileAccess]::Write, [IO.FileShare]::Read)
    try {
        $bytes = [Text.Encoding]::UTF8.GetBytes($Content)
        $stream.Write($bytes, 0, $bytes.Length)
        $stream.Flush($true)
    }
    finally { $stream.Dispose() }
}

function Invoke-GenericTransportRequest([string]$Marker, [string]$Stage) {
    $schema = [ordered]@{
        type = 'object'; additionalProperties = $false; required = @('status')
        properties = [ordered]@{status=[ordered]@{type='string'; enum=@($Marker)}}
    }
    $body = [ordered]@{
        model = [string]$config.model.id
        temperature = 0
        max_tokens = 24
        response_format = [ordered]@{type='json_schema';json_schema=[ordered]@{name='runtime_readiness';strict=$true;schema=$schema}}
        messages = @(
            [ordered]@{role='system';content='This is a generic runtime transport check. Return only the requested JSON readiness marker.'},
            [ordered]@{role='user';content='Emit the fixed readiness marker now.'}
        )
    } | ConvertTo-Json -Depth 20 -Compress
    $timer = [Diagnostics.Stopwatch]::StartNew()
    try {
        $response = Invoke-RestMethod -Method Post -Uri ([string]$config.runtime.localai_url + '/v1/chat/completions') -ContentType 'application/json; charset=utf-8' -Body ([Text.Encoding]::UTF8.GetBytes($body)) -TimeoutSec 180
    }
    catch { throw "GENERIC_TRANSPORT_FAILED:${Stage}:$($_.Exception.Message)" }
    $timer.Stop()
    if (@($response.choices).Count -ne 1 -or [string]$response.choices[0].finish_reason -cne 'stop') { throw "GENERIC_RESPONSE_INCOMPLETE:$Stage" }
    $decoded = [string]$response.choices[0].message.content | ConvertFrom-Json
    if ([string]$decoded.status -cne $Marker) { throw "GENERIC_RESPONSE_INVALID:$Stage" }
    $record = [ordered]@{stage=$Stage;latency_ms=$timer.ElapsedMilliseconds;finish_reason=[string]$response.choices[0].finish_reason;status=[string]$decoded.status;ram_after_gib=Get-FreeRAMGiB}
    Append-JSONLine $resourcePath ([ordered]@{utc=[DateTime]::UtcNow.ToString('o');stage=$Stage;latency_ms=$record.latency_ms;available_ram_gib=$record.ram_after_gib;finish_reason=$record.finish_reason})
    $loadedFloor = [double]$config.resource.loaded_formal_floor_gib
    if ($record.ram_after_gib -lt $loadedFloor) { throw "RAM_FLOOR_FAILED:$Stage minimum_gib=$($record.ram_after_gib) required_gib=$loadedFloor" }
    return $record
}

function Get-APIContainerMemory {
    return (Invoke-Native docker @('stats', '--no-stream', '--format', '{{json .}}', 'nexusai-api-1')) | ConvertFrom-Json
}

function Get-BackendMemory($State) {
    $api = $State.containers['nexusai-api-1'].id
    $raw = Invoke-Native docker @('exec', $api, 'ps', '-eo', 'pid=,rss=,args=')
    $rows = @()
    foreach ($line in ($raw -split '\r?\n')) {
        if ($line -match '^\s*([0-9]+)\s+([0-9]+)\s+(.+)$' -and $Matches[3] -match 'llama-cpp-cpu-all') {
            $rows += [ordered]@{pid=[int]$Matches[1];rss_mib=[math]::Round(([int64]$Matches[2])/1024,1);command=$Matches[3]}
        }
    }
    return @($rows)
}

function Get-VmmemWorkingSetMiB {
    $process = Get-Process -ErrorAction SilentlyContinue | Where-Object { $_.ProcessName -in @('vmmemWSL','vmmem') } | Sort-Object WorkingSet64 -Descending | Select-Object -First 1
    if ($null -eq $process) { return $null }
    return [math]::Round($process.WorkingSet64/1MB,1)
}

function Write-QualificationResourceSample($Process, [string]$Stage) {
    $workingSet = 0.0
    try { $Process.Refresh(); $workingSet = [math]::Round($Process.WorkingSet64/1MB,3) } catch {}
    $ram = Get-FreeRAMGiB
    Append-JSONLine $resourcePath ([ordered]@{utc=[DateTime]::UtcNow.ToString('o');stage=$Stage;available_ram_gib=$ram;evaluator_working_set_mib=$workingSet})
    return [ordered]@{ram=$ram;working_set_mib=$workingSet}
}

Assert-FrozenFiles
$before = Get-RuntimeState
Assert-RuntimeBaseline $before
Assert-ModelIdentity $before
if (Test-Path -LiteralPath $qualificationLock) { throw 'FRESH_QUALIFICATION_CORPUS_ALREADY_CONSUMED' }
if ($ValidateOnly) {
    if (Test-Path -LiteralPath $resourceLock) { throw 'RESOURCE_SMOKE_ALREADY_DISPATCHED' }
    Assert-ResourceStateMachineContract
    Write-Host 'STANDALONE_PREPARATION_VALIDATION=PASS'
    Write-Host 'RESOURCE_STATE_MACHINE_STATIC_TESTS=PASS'
    Write-Host 'RESOURCE_SMOKE=NOT_STARTED'
    Write-Host 'QUALIFICATION_CORPUS=UNCONSUMED'
    Write-Host 'LIVE_INFERENCE=false'
    Write-Host 'AUTO_KILL_CODEX=NO'
    Write-Host 'AUTO_KILL_CHROME=NO'
    Write-Host 'AUTO_KILL_VSCODE=NO'
    Write-Host 'AUTO_KILL_EXPLORER=NO'
    Write-Host 'AUTO_KILL_VMMEMWSL=NO'
    Write-Host 'AUTO_KILL_RANDOM_USER_PROCESS=NO'
    exit 0
}

New-Item -ItemType Directory -Force -Path $run | Out-Null
try {
    if (Test-Path -LiteralPath $resourceLock) { throw 'RESOURCE_SMOKE_ALREADY_DISPATCHED' }
    $initiallyLoaded = $null -ne (Get-LoadedState)
    if ($initiallyLoaded) {
        Invoke-UnloadCurrentModel
        Start-Sleep -Seconds 5
        if ($null -ne (Get-LoadedState)) { throw 'PRELOAD_MODEL_UNLOAD_FAILED' }
    }
    Invoke-GovernedCleanCacheReclaim
    $admission = Get-PreloadAdmission
    $recoverySamples = @($admission.recovery_samples)
    $recoveryBestStableMinGiB = [double]$admission.recovery_best_stable_min_gib
    $qualifyingPolicy = [string]$admission.qualifying_policy
    $qualifyingThresholdGiB = [double]$admission.qualifying_threshold_gib
    $preloadSamples = @($admission.qualifying_samples)
    $predictedLoadedGiB = [double]$admission.predicted_loaded_gib

    $preloadMin = [double]($preloadSamples | Measure-Object -Minimum).Minimum
    $preloadMax = [double]($preloadSamples | Measure-Object -Maximum).Maximum
    Write-Host "QUALIFYING_SAMPLES_GIB=$($preloadSamples -join ',')"
    Write-Host "QUALIFYING_MIN_GIB=$preloadMin"
    Write-Host "PREDICTED_LOADED_GIB=$predictedLoadedGiB"
    Write-Host 'PRELOAD_STABLE=PASS'
    Write-Host "PRELOAD_MIN_GIB=$preloadMin"
    Write-Host "PRELOAD_MAX_GIB=$preloadMax"
    Write-Host "PRELOAD_ADVISORY_TARGET_GIB=$($config.resource.empirical_advisory_preload_target_gib)"
    Write-Host "ADVISORY_TARGET_MET=$($preloadMin -ge [double]$config.resource.empirical_advisory_preload_target_gib)"

    New-ExclusiveLock $resourceLock ("resource_smoke_id=$($config.resource.smoke_id)`ndispatched_at=$([DateTime]::UtcNow.ToString('o'))`nquality_evidence=NONE`n")
    $resourceLockCreated = $true
    $modelOwned = $true
    $transportCalls += Invoke-GenericTransportRequest 'READY_A' 'model_load_resource_smoke'
    if ($null -eq (Get-LoadedState)) { throw 'CURRENT_4B_MODEL_NOT_LOADED_AFTER_SMOKE' }
    Invoke-GovernedCleanCacheReclaim
    Start-Sleep -Seconds 15
    $postLoadSamples = @(Get-StableRAMSamples 'post_load' ([double]$config.resource.loaded_formal_floor_gib) ([int]$config.resource.state_machine.qualifying_sample_count) ([int]$config.resource.state_machine.qualifying_sample_spacing_seconds))
    Assert-StableSampleWindow $postLoadSamples ([double]$config.resource.loaded_formal_floor_gib) 'post_load' ([double]$config.resource.state_machine.maximum_window_decline_gib)
    $apiMemory = Get-APIContainerMemory
    $backendMemory = @(Get-BackendMemory $before)
    $vmmemWorkingSetMiB = Get-VmmemWorkingSetMiB
    Write-Host 'RESOURCE_SMOKE=PASS'

    $transportCalls += Invoke-GenericTransportRequest 'READY_B' 'transport_soak_1'
    $transportCalls += Invoke-GenericTransportRequest 'READY_C' 'transport_soak_2'
    Write-Host 'HTTP_TRANSPORT=PASS'
    Write-Host 'MODEL_RESPONSE=PASS'
    Write-Host 'NO_TIMEOUT=PASS'
    Write-Host 'STRICT_RESPONSE_COMPLETION=PASS'
    Start-Sleep -Seconds 10
    $postSoakSamples = @(Get-StableRAMSamples 'post_soak' ([double]$config.resource.loaded_formal_floor_gib) ([int]$config.resource.state_machine.qualifying_sample_count) ([int]$config.resource.state_machine.qualifying_sample_spacing_seconds))
    Assert-StableSampleWindow $postSoakSamples ([double]$config.resource.loaded_formal_floor_gib) 'post_soak' ([double]$config.resource.state_machine.maximum_window_decline_gib)
    Write-Host 'TRANSPORT_SOAK=PASS'
    Write-Host 'POST_LOAD_RAM=PASS'

    Assert-FrozenFiles
    $env:NEXUSAI_ENGLISH_FUNCTIONAL_LIVE = '1'
    $env:NEXUSAI_ENGLISH_CORPUS = Resolve-RepoPath ([string]$config.corpus.path)
    $env:NEXUSAI_ENGLISH_ORACLE = Resolve-RepoPath ([string]$config.oracle.path)
    $env:NEXUSAI_ENGLISH_RUN_DIR = $run
    $env:NEXUSAI_ENGLISH_DISPATCH_LOCK = $qualificationLock
    $env:NEXUSAI_ENGLISH_MODEL = [string]$config.model.id
    $env:NEXUSAI_ENGLISH_LOCALAI_URL = [string]$config.runtime.localai_url
    $env:NEXUSAI_ENGLISH_RAM_CHECK = Resolve-RepoPath 'scripts/nxb21-english-functional-qualification/check_loaded_ram.ps1'

    $process = Start-Process -FilePath (Resolve-RepoPath ([string]$config.evaluator.path)) -ArgumentList @('-test.run','^TestEnglishFunctionalQualificationOneShot$','-test.v','-test.timeout','3h') -WorkingDirectory $root -NoNewWindow -PassThru -RedirectStandardOutput $stdoutPath -RedirectStandardError $stderrPath
    $qualificationStarted = $true
    Write-Host 'QUALIFICATION_DISPATCH=STARTING_AFTER_ALL_RESOURCE_GATES'
    do {
        $sample = Write-QualificationResourceSample $process 'qualification'
        if ($sample.working_set_mib -gt $evaluatorPeakMiB) { $evaluatorPeakMiB = $sample.working_set_mib }
        if ($sample.ram -lt 4 -and -not $process.HasExited) {
            $qualificationRAMFailure = $true
            Stop-Process -Id $process.Id -Force
            break
        }
        if (-not $process.HasExited) { Start-Sleep -Seconds 5 }
    } while (-not $process.HasExited)
    $process.WaitForExit()
    $process.Refresh()
    if ($qualificationRAMFailure) { throw 'QUALIFICATION_RESOURCE_FLOOR_BREACH' }
    if (-not (Test-Path -LiteralPath $qualificationLock)) { throw 'QUALIFICATION_DISPATCH_LOCK_MISSING' }
    if (-not (Test-Path -LiteralPath $qualificationResultPath)) { throw 'QUALIFICATION_RESULT_MISSING' }
    $qualificationResult = Get-Content -LiteralPath $qualificationResultPath -Raw | ConvertFrom-Json
    if ($process.ExitCode -ne 0 -or [string]$qualificationResult.gate -cne 'PASS') { throw "QUALIFICATION_GATE_FAILED:exit=$($process.ExitCode)" }
    $finalState = 'CURRENT_4B_ENGLISH_FUNCTIONAL_BASELINE'
    $exitCode = 0
}
catch {
    $errorMessage = $_.Exception.Message
    if ($errorMessage -like 'PREDICTED_RESOURCE_INSUFFICIENT*' -or $errorMessage -like 'RAM_FLOOR_FAILED:preload_qualifying*' -or $errorMessage -like 'RAM_TREND_FAILED:preload_qualifying*') {
        $finalState = 'CURRENT_4B_PREDICTED_RESOURCE_INSUFFICIENT'; $exitCode = 11
        Write-HighMemoryReport
        Write-Host 'RESOURCE_SMOKE=NOT_STARTED'
        Write-Host 'QUALIFICATION_CORPUS=UNCONSUMED'
        Write-Host 'MODEL_DISPATCHED_FOR_QUALIFICATION=false'
        Write-Host 'MUTATION_PERFORMED=false'
        Write-Host 'CURRENT_QWEN3_4B_LOCAL_FUNCTIONAL_RUNTIME=RESOURCE_INSUFFICIENT_ON_CURRENT_HOST'
        Write-Host 'NEXT_ACTION=SMALLER_MODEL_SELECTION_SEPARATE_APPROVAL_REQUIRED'
    }
    elseif (-not $qualificationStarted -and ($errorMessage -like 'RAM_FLOOR_FAILED:post_load*' -or $errorMessage -like 'RAM_TREND_FAILED:post_load*' -or $errorMessage -like 'RAM_FLOOR_FAILED:model_load*' -or $errorMessage -like 'CURRENT_4B_MODEL_NOT_LOADED*')) {
        $finalState = 'CURRENT_4B_RESOURCE_SMOKE_FAIL'; $exitCode = 20
        Write-Host 'CURRENT_QWEN3_4B_LOCAL_FUNCTIONAL_RUNTIME=RESOURCE_INSUFFICIENT_ON_CURRENT_HOST'
    }
    elseif (-not $qualificationStarted -and ($errorMessage -like 'GENERIC_*' -or $errorMessage -like '*transport*' -or $errorMessage -like 'RAM_FLOOR_FAILED:post_soak*' -or $errorMessage -like 'RAM_TREND_FAILED:post_soak*')) {
        $finalState = 'CURRENT_4B_TRANSPORT_SMOKE_FAIL'; $exitCode = 21
        Write-Host 'CURRENT_QWEN3_4B_LOCAL_FUNCTIONAL_RUNTIME=TRANSPORT_OR_USABILITY_INSUFFICIENT_ON_CURRENT_HOST'
    }
    elseif ($qualificationStarted) {
        $finalState = $(if ($qualificationRAMFailure) { 'INCOMPLETE_RESOURCE_FAILURE' } else { 'ENGLISH_FUNCTIONAL_QUALIFICATION_FAIL' })
        $exitCode = 30
    }
    else { $finalState = 'PREQUALIFICATION_INTEGRITY_FAIL'; $exitCode = 40 }
    Write-Host "FINAL_STATE=$finalState"
    Write-Host "ERROR=$errorMessage"
}
finally {
    $ownedModelLoaded = $false
    if ($modelOwned) { try { $ownedModelLoaded = $null -ne (Get-LoadedState) } catch {} }
    if ($ownedModelLoaded) {
        try {
            Invoke-UnloadCurrentModel
            Start-Sleep -Seconds 5
            Write-Host 'CURRENT_4B_MODEL_UNLOAD=PASS'
        }
        catch { Write-Host "CURRENT_4B_MODEL_UNLOAD=FAIL error=$($_.Exception.Message)" }
    }
    try {
        $after = Get-RuntimeState
        Assert-RuntimeUnchanged $before $after
        Write-Host 'RUNTIME_INTEGRITY=PASS'
    }
    catch {
        if ($errorMessage -eq '') { $errorMessage = $_.Exception.Message; $finalState = 'RUNTIME_INTEGRITY_FAIL'; $exitCode = 40 }
        Write-Host "RUNTIME_INTEGRITY=FAIL error=$($_.Exception.Message)"
    }
    $receipt = [ordered]@{
        contract_version = 'nexusai.current4b-resource-then-english-qualification/v1'
        resource_smoke_id = [string]$config.resource.smoke_id
        corpus_id = [string]$config.corpus.id
        final_state = $finalState
        exit_code = $exitCode
        error = $errorMessage
        completed_at = [DateTime]::UtcNow.ToString('o')
        resource_smoke_quality_evidence = 'NONE'
        resource_lock_created = $resourceLockCreated
        corpus_consumed = Test-Path -LiteralPath $qualificationLock
        qualification_started = $qualificationStarted
        recovery_samples_gib = $recoverySamples
        recovery_best_stable_min_gib = $recoveryBestStableMinGiB
        maximum_recovery_wait_seconds = [int]$config.resource.state_machine.maximum_recovery_wait_seconds
        qualifying_policy = $qualifyingPolicy
        qualifying_threshold_gib = $qualifyingThresholdGiB
        qualifying_samples_gib = $preloadSamples
        qualifying_min_gib = $(if ($preloadSamples.Count) { [double]($preloadSamples | Measure-Object -Minimum).Minimum } else { $null })
        maximum_window_decline_gib = [double]$config.resource.state_machine.maximum_window_decline_gib
        predicted_loaded_gib = $predictedLoadedGiB
        preload_samples_gib = $preloadSamples
        preload_min_gib = $(if ($preloadSamples.Count) { [double]($preloadSamples | Measure-Object -Minimum).Minimum } else { $null })
        preload_max_gib = $(if ($preloadSamples.Count) { [double]($preloadSamples | Measure-Object -Maximum).Maximum } else { $null })
        preload_advisory_target_gib = [double]$config.resource.empirical_advisory_preload_target_gib
        advisory_target_met = $(if ($preloadSamples.Count) { [double]($preloadSamples | Measure-Object -Minimum).Minimum -ge [double]$config.resource.empirical_advisory_preload_target_gib } else { $false })
        post_load_samples_gib = $postLoadSamples
        post_load_min_gib = $(if ($postLoadSamples.Count) { [double]($postLoadSamples | Measure-Object -Minimum).Minimum } else { $null })
        post_load_max_gib = $(if ($postLoadSamples.Count) { [double]($postLoadSamples | Measure-Object -Maximum).Maximum } else { $null })
        post_load_stable_gib = $(if ($postLoadSamples.Count) { [double]($postLoadSamples | Measure-Object -Minimum).Minimum } else { $null })
        post_soak_samples_gib = $postSoakSamples
        post_soak_min_gib = $(if ($postSoakSamples.Count) { [double]($postSoakSamples | Measure-Object -Minimum).Minimum } else { $null })
        host_headroom_gib = $(if ($postSoakSamples.Count) { [math]::Round(([double]($postSoakSamples | Measure-Object -Minimum).Minimum - 4), 3) } else { $null })
        transport_calls = $transportCalls
        api_container_memory = $apiMemory
        vmmemwsl_working_set_mib = $vmmemWorkingSetMiB
        model_backend_memory = $backendMemory
        evaluator_peak_working_set_mib = $evaluatorPeakMiB
        runtime_before = $before
        runtime_after = $after
        qualification_result_path = $(if (Test-Path -LiteralPath $qualificationResultPath) { $qualificationResultPath } else { $null })
        evaluator_stdout_path = $stdoutPath
        evaluator_stderr_path = $stderrPath
        resource_observations_path = $resourcePath
        auto_kill_codex = 'NO'
        auto_kill_chrome = 'NO'
        auto_kill_vscode = 'NO'
        auto_kill_explorer = 'NO'
        auto_kill_vmmemwsl = 'NO'
        auto_kill_random_user_process = 'NO'
    }
    Write-JSON $receiptPath $receipt
    [IO.File]::WriteAllText($receiptPath + '.sha256', (Get-SHA256 $receiptPath) + "`n", [Text.UTF8Encoding]::new($false))
    Write-Host "OPERATOR_RECEIPT=$receiptPath"
}

if ($exitCode -eq 0) { Write-Host 'FINAL_STATE=CURRENT_4B_ENGLISH_FUNCTIONAL_BASELINE' }
exit $exitCode
