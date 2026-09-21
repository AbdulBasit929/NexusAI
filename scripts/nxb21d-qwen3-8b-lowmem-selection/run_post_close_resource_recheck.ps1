# SPDX-License-Identifier: MIT
[CmdletBinding()]
param([switch]$ValidateOnly)
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$root = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
. (Join-Path $root 'scripts\nxb21d-offline\common.ps1')

$model = 'qwen3-8b-q4km-nxb21d-selection-lowmem-dev'
$container = 'nexusai-api-1'
$url = 'http://127.0.0.1:8080'
$artifact = Join-Path $root 'local-acceptance-models\nxb21-d\challengers\qwen3-8b-q4km\Qwen3-8B-Q4_K_M.gguf'
$profile = Join-Path $root 'local-acceptance-models\nxb21-d\challengers\qwen3-8b-q4km\qwen3-8b-q4km-nxb21d-selection-lowmem-dev.yaml'
$artifactSHA = 'd98cdcbd03e17ce47681435b5150e34c1417f50b5c0019dd560e4882c5745785'
$profileSHA = '672f8e2c29f162cdc74a8a8dfe7129955436c771db454b0a3ebb20a3bf9b560a'
$outputRoot = Join-Path $root 'local-acceptance-models\nxb21-d\qwen3-8b-lowmem-resource-recheck'
$dispatchLock = Join-Path $outputRoot 'resource-recheck-dispatched.lock'

function Get-LoadedModelIDs {
    return @((Invoke-RestMethod -Uri ($url + '/system') -TimeoutSec 15).loaded_models.id | Sort-Object)
}

function Get-BackendRows {
    $raw = Invoke-NxDNative docker @('exec', $container, 'sh', '-lc', 'ps -eo pid=,rss=,args= | grep llama-cpp-cpu-all | grep -v grep || true')
    $rows = @()
    foreach ($line in ($raw -split '\r?\n')) {
        if ($line -match '^\s*([0-9]+)\s+([0-9]+)\s+(.+)$') {
            $rows += [pscustomobject][ordered]@{
                pid = [int]$Matches[1]
                rss_mib = [math]::Round(([int64]$Matches[2]) / 1024, 1)
                command = $Matches[3]
            }
        }
    }
    return @($rows)
}

function Get-APIContainerStats {
    return (Invoke-NxDNative docker @('stats', '--no-stream', '--format', '{{json .}}', $container)) | ConvertFrom-Json
}

function Convert-MemoryToMiB([string]$Usage) {
    $value = ($Usage -split '/')[0].Trim()
    if ($value -notmatch '^([0-9.]+)(KiB|MiB|GiB|B)$') { return $null }
    $number = [double]$Matches[1]
    $mib = switch ($Matches[2]) {
        'GiB' { $number * 1024 }
        'MiB' { $number }
        'KiB' { $number / 1024 }
        default { $number / 1MB }
    }
    return [math]::Round($mib, 1)
}

function Get-RuntimeState {
    $containers = Get-NxDContainers
    return [pscustomobject][ordered]@{
        containers = $containers
        retained_tuple = Get-NxDRetainedTuple $containers
        activity_count = Get-NxDActivityCount $containers
        active_jobs = Get-NxDActiveJobs $containers
        loaded_models = @(Get-LoadedModelIDs)
    }
}

function Assert-SameRuntime($Before, $After) {
    foreach ($name in $Before.containers.Keys) {
        $a = $After.containers[$name]
        $b = $Before.containers[$name]
        if ($null -eq $a -or $a.Id -cne $b.Id -or $a.Image -cne $b.Image -or [int]$a.RestartCount -ne [int]$b.RestartCount) {
            throw "RUNTIME_CONTAINER_DRIFT:$name"
        }
        if (-not (Test-NxDContainerHealthy $a)) { throw "RUNTIME_UNHEALTHY:$name" }
    }
    if ($After.retained_tuple -cne $Before.retained_tuple) { throw 'RETAINED_TUPLE_DRIFT' }
    if ($After.activity_count -ne $Before.activity_count) { throw 'ACTIVITY_DRIFT' }
    if ($After.active_jobs -ne 0) { throw "ACTIVE_JOBS:$($After.active_jobs)" }
    if (@(Compare-Object $Before.loaded_models $After.loaded_models).Count -ne 0) { throw 'LOADED_MODEL_SET_DRIFT' }
}

function Assert-Preflight {
    if (-not (Test-Path -LiteralPath $artifact -PathType Leaf) -or (Get-Item -LiteralPath $artifact).Length -ne 5027783488) { throw 'ARTIFACT_IDENTITY_MISMATCH' }
    if ((Get-NxDHash $artifact) -cne $artifactSHA) { throw 'ARTIFACT_HASH_MISMATCH' }
    if ((Get-NxDHash $profile) -cne $profileSHA) { throw 'PROFILE_HASH_MISMATCH' }
    $containerArtifactSHA = ((Invoke-NxDNative docker @('exec', $container, 'sha256sum', '/models/Qwen3-8B-Q4_K_M.gguf')) -split '\s+')[0]
    $containerProfileSHA = ((Invoke-NxDNative docker @('exec', $container, 'sha256sum', '/models/qwen3-8b-q4km-nxb21d-selection-lowmem-dev.yaml')) -split '\s+')[0]
    if ($containerArtifactSHA -cne $artifactSHA) { throw 'CONTAINER_ARTIFACT_HASH_MISMATCH' }
    if ($containerProfileSHA -cne $profileSHA) { throw 'CONTAINER_PROFILE_HASH_MISMATCH' }
    if (@((Invoke-RestMethod -Uri ($url + '/v1/models') -TimeoutSec 15).data.id) -notcontains $model) { throw 'MODEL_REGISTRATION_MISSING' }
    if (@(Get-LoadedModelIDs) -contains $model) { throw 'LOWMEM_MODEL_ALREADY_LOADED' }
}

function Get-EffectiveSettings([datetime]$Since) {
    $raw = Invoke-NxDNative docker @('logs', '--since', $Since.ToUniversalTime().ToString('o'), $container)
    $clean = [regex]::Replace($raw, [char]27 + '\[[0-9;?]*[ -/]*[@-~]', '')
    $line = @($clean -split '\r?\n' | Where-Object { $_ -like "*effective runtime tuning*modelID=`"$model`"*" } | Select-Object -Last 1)
    if ($line.Count -ne 1) { throw 'EFFECTIVE_RUNTIME_SETTINGS_NOT_FOUND' }
    if ($line[0] -notmatch 'context=([0-9]+).*n_batch=([0-9]+).*n_gpu_layers=([0-9]+).*parallel="([0-9]+)".*flash_attention="([^"]+)".*f16=([^ ]+)') {
        throw 'EFFECTIVE_RUNTIME_SETTINGS_UNPARSEABLE'
    }
    return [pscustomobject][ordered]@{
        context_size = [int]$Matches[1]
        n_batch = [int]$Matches[2]
        gpu_layers = [int]$Matches[3]
        parallel = [int]$Matches[4]
        flash_attention = $Matches[5]
        f16 = $Matches[6]
        raw_log_line = $line[0]
    }
}

Assert-Preflight
$validationState = Get-RuntimeState
if ($validationState.active_jobs -ne 0) { throw "ACTIVE_JOBS:$($validationState.active_jobs)" }
Assert-NxDHTTPHealth ([pscustomobject]@{runtime = [pscustomobject]@{localai_url = $url}})
if ($ValidateOnly) {
    Write-Host "PROFILE=$model"
    Write-Host "PROFILE_SHA256=$profileSHA"
    Write-Host "ARTIFACT_SHA256=$artifactSHA"
    Write-Host 'MODEL_STATE=UNLOADED'
    Write-Host 'REQUESTED_BATCH=128'
    Write-Host 'KNOWN_PRIOR_EFFECTIVE_N_BATCH=512'
    Write-Host "CURRENT_RETAINED_TUPLE=$($validationState.retained_tuple)"
    Write-Host "CURRENT_ACTIVITY_COUNT=$($validationState.activity_count)"
    Write-Host 'ACTIVE_JOBS=0'
    Write-Host 'LOWMEM_RESOURCE_RECHECK_VALIDATE_ONLY=PASS live_inference=false model_load=false'
    exit 0
}

if (Test-Path -LiteralPath $dispatchLock) { throw 'LOWMEM_RESOURCE_RECHECK_ALREADY_DISPATCHED' }
$firstFloor = [pscustomobject]@{gates = [pscustomobject]@{minimum_available_ram_gib = 6}}
try { $availableBefore = Assert-NxDRam $firstFloor 'post_close_before_load' } catch {
    if ($_.Exception.Message -notlike 'RAM_TOO_LOW*') { throw }
    $null = Invoke-NxDSafeLinuxCleanCache
    $availableBefore = Wait-NxDRamAfterCleanCache $firstFloor 'post_close_before_load' 180
}

$run = Join-Path $outputRoot ('run-' + [DateTime]::UtcNow.ToString('yyyyMMddTHHmmssfffZ'))
New-Item -ItemType Directory -Force -Path $run | Out-Null
[IO.File]::WriteAllText($dispatchLock, $run + "`n", [Text.UTF8Encoding]::new($false))
$before = $validationState
$backendBefore = @(Get-BackendRows)
$beforePIDs = @($backendBefore.pid)
$loaded = $false
$status = 'FAIL'
$failure = ''
$samples = @()
$effective = $null
$loadMilliseconds = $null
$postcheck = 'NOT_RUN'
$after = $null
$started = [DateTime]::UtcNow.AddSeconds(-2)

try {
    Write-Host "LOWMEM_RESOURCE_RECHECK=START run=$run available_before_load_gib=$availableBefore"
    $tokenizeBody = @{model = $model; content = 'NEXUSAI_RESOURCE_ADMISSION_RECHECK_ONLY'} | ConvertTo-Json -Compress
    $timer = [Diagnostics.Stopwatch]::StartNew()
    $tokenized = Invoke-RestMethod -Method Post -Uri ($url + '/v1/tokenize') -ContentType 'application/json; charset=utf-8' -Body ([Text.Encoding]::UTF8.GetBytes($tokenizeBody)) -TimeoutSec 180
    $timer.Stop()
    $loadMilliseconds = $timer.ElapsedMilliseconds
    if ($null -eq $tokenized.tokens) { throw 'TOKENIZER_LOAD_RESPONSE_INVALID' }
    $loaded = $true
    $effective = Get-EffectiveSettings $started
    Write-Host "EFFECTIVE_RUNTIME context=$($effective.context_size) requested_batch=128 n_batch=$($effective.n_batch) gpu_layers=$($effective.gpu_layers) parallel=$($effective.parallel)"

    Start-Sleep -Seconds 15
    $cleanAttempted = $false
    $deadline = [DateTime]::UtcNow.AddSeconds(180)
    $consecutive = 0
    do {
        $available = Get-NxDRamGiB
        $backends = @(Get-BackendRows)
        $owned = @($backends | Where-Object { $beforePIDs -notcontains $_.pid })
        $stats = Get-APIContainerStats
        $samples += [pscustomobject][ordered]@{
            utc = [DateTime]::UtcNow.ToString('o')
            available_ram_gib = $available
            owned_backend = $owned
            api_memory_usage = $stats.MemUsage
            api_memory_mib = Convert-MemoryToMiB $stats.MemUsage
        }
        Write-Host "LOADED_RAM_SAMPLE available_gib=$available required_gib=4 consecutive=$consecutive"
        if ($available -ge 4) { $consecutive++ } else { $consecutive = 0 }
        if ($consecutive -ge 3) { break }
        if (-not $cleanAttempted -and $available -lt 4) {
            $null = Invoke-NxDSafeLinuxCleanCache
            $cleanAttempted = $true
        }
        Start-Sleep -Seconds 5
    } while ([DateTime]::UtcNow -lt $deadline)
    if ($consecutive -lt 3) { throw "LOADED_RAM_FLOOR_FAILED:$($samples[-1].available_ram_gib)" }
    $status = 'PASS'
} catch {
    $failure = $_.Exception.Message
    Write-Host "LOWMEM_RESOURCE_RECHECK=FAIL error=$failure"
} finally {
    if ($loaded) {
        try {
            Invoke-NxDUnload ([pscustomobject]@{model = [pscustomobject]@{id = $model}; runtime = [pscustomobject]@{localai_url = $url}})
            Start-Sleep -Seconds 5
            if (@(Get-LoadedModelIDs) -contains $model) { throw 'MODEL_UNLOAD_FAILED' }
            $loaded = $false
        } catch {
            $status = 'FAIL'
            $failure = (($failure + ';MODEL_UNLOAD:' + $_.Exception.Message).Trim(';'))
        }
    }
    try {
        $after = Get-RuntimeState
        Assert-SameRuntime $before $after
        $postcheck = 'PASS'
    } catch {
        $status = 'FAIL'
        $postcheck = 'FAIL'
        $failure = (($failure + ';POSTCHECK:' + $_.Exception.Message).Trim(';'))
    }

    $ownedSamples = @($samples | ForEach-Object { @($_.owned_backend) })
    $backendRSS = if ($ownedSamples.Count) { [math]::Round(($ownedSamples.rss_mib | Measure-Object -Maximum).Maximum, 1) } else { $null }
    $apiPeak = if ($samples.Count) { [math]::Round(($samples.api_memory_mib | Measure-Object -Maximum).Maximum, 1) } else { $null }
    $availableAfterLoad = if ($samples.Count) { [double]$samples[-1].available_ram_gib } else { $null }
    $minimumAvailableLoaded = if ($samples.Count) { [math]::Round(($samples.available_ram_gib | Measure-Object -Minimum).Minimum, 3) } else { $null }
    $receipt = [ordered]@{
        schema_version = 'nexusai.nxb21d-qwen3-8b-lowmem-post-close-resource-recheck/v1'
        status = $status
        resource_admission_only = $true
        live_inference = $false
        benchmark_questions_used = 0
        model = $model
        artifact_sha256 = $artifactSHA
        profile_sha256 = $profileSHA
        requested_batch = 128
        effective_runtime = $effective
        preload_floor_gib = 6
        loaded_floor_gib = 4
        available_before_load_gib = $availableBefore
        available_after_load_gib = $availableAfterLoad
        minimum_available_while_loaded_gib = $minimumAvailableLoaded
        model_backend_peak_rss_mib = $backendRSS
        api_container_peak_mib = $apiPeak
        load_and_tokenize_latency_ms = $loadMilliseconds
        settling_limit_seconds = 180
        required_consecutive_loaded_samples = 3
        samples = $samples
        model_unload = $(if (-not $loaded) { 'PASS' } else { 'FAIL' })
        runtime_postcheck = $postcheck
        retained_tuple_before = $before.retained_tuple
        retained_tuple_after = $(if ($null -ne $after) { $after.retained_tuple } else { $null })
        activity_count_before = $before.activity_count
        activity_count_after = $(if ($null -ne $after) { $after.activity_count } else { $null })
        active_jobs_before = $before.active_jobs
        active_jobs_after = $(if ($null -ne $after) { $after.active_jobs } else { $null })
        loaded_models_before = $before.loaded_models
        loaded_models_after = $(if ($null -ne $after) { $after.loaded_models } else { $null })
        error = $failure
        D_status = 'OPEN'
        activation = 'BLOCKED'
    }
    $receiptPath = Join-Path $run 'lowmem-post-close-resource-recheck-v1.json'
    Write-NxDJson $receiptPath $receipt
    [IO.File]::WriteAllText($receiptPath + '.sha256', (Get-NxDHash $receiptPath) + "`n", [Text.UTF8Encoding]::new($false))
    Write-Host "LOWMEM_RESOURCE_RECHECK=$status receipt=$receiptPath"
    Write-Host "LOWMEM_RESOURCE_RECHECK_RECEIPT_SHA256=$(Get-NxDHash $receiptPath)"
}

if ($status -eq 'PASS') { exit 0 }
exit 1
