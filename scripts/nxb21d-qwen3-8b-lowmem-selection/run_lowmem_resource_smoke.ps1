# SPDX-License-Identifier: MIT
[CmdletBinding()]
param()
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$root = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
. (Join-Path $root 'scripts\nxb21d-offline\common.ps1')

$model = 'qwen3-8b-q4km-nxb21d-selection-lowmem-dev'
$container = 'nexusai-api-1'
$url = 'http://127.0.0.1:8080'
$artifact = Join-Path $root 'local-acceptance-models\nxb21-d\challengers\qwen3-8b-q4km\Qwen3-8B-Q4_K_M.gguf'
$profile = Join-Path $root 'local-acceptance-models\nxb21-d\challengers\qwen3-8b-q4km\qwen3-8b-q4km-nxb21d-selection-lowmem-dev.yaml'
$outputRoot = Join-Path $root 'local-acceptance-models\nxb21-d\qwen3-8b-lowmem-resource-smoke'
$lock = Join-Path $outputRoot 'resource-smoke-dispatched.lock'
$run = Join-Path $outputRoot ('run-' + [DateTime]::UtcNow.ToString('yyyyMMddTHHmmssfffZ'))
$modelMayBeLoaded = $false

function Get-BackendMemory {
    $raw = Invoke-NxDNative docker @('exec', $container, 'sh', '-lc', 'ps -eo pid=,rss=,args= | grep llama-cpp-cpu-all | grep -v grep')
    $rows = @()
    foreach ($line in ($raw -split '\r?\n')) {
        if ($line -match '^\s*([0-9]+)\s+([0-9]+)\s+(.+)$') {
            $rows += [ordered]@{pid = [int]$Matches[1]; rss_mib = [math]::Round(([int64]$Matches[2]) / 1024, 1); command = $Matches[3]}
        }
    }
    return @($rows)
}

function Get-APIContainerMemory {
    $raw = Invoke-NxDNative docker @('stats', '--no-stream', '--format', '{{json .}}', $container)
    return $raw | ConvertFrom-Json
}

function Assert-AvailableRAM([double]$Floor, [string]$Stage) {
    $cfg = [pscustomobject]@{gates = [pscustomobject]@{minimum_available_ram_gib = $Floor}}
    try { return Assert-NxDRam $cfg $Stage } catch {
        if ($_.Exception.Message -notlike 'RAM_TOO_LOW*') { throw }
        $null = Invoke-NxDSafeLinuxCleanCache
        return Wait-NxDRamAfterCleanCache $cfg $Stage 180
    }
}

function Get-RuntimeState {
    $containers = Get-NxDContainers
    return [ordered]@{
        retained_tuple = Get-NxDRetainedTuple $containers
        activity_count = Get-NxDActivityCount $containers
        active_jobs = Get-NxDActiveJobs $containers
    }
}

function Test-Loaded {
    $system = Invoke-RestMethod -Uri ($url + '/system') -TimeoutSec 15
    return @($system.loaded_models.id) -contains $model
}

try {
    if (Test-Path -LiteralPath $lock) { throw 'LOWMEM_RESOURCE_SMOKE_ALREADY_DISPATCHED' }
    if ((Get-Item -LiteralPath $artifact).Length -ne 5027783488) { throw 'ARTIFACT_BYTES_MISMATCH' }
    if ((Get-NxDHash $artifact) -cne 'd98cdcbd03e17ce47681435b5150e34c1417f50b5c0019dd560e4882c5745785') { throw 'ARTIFACT_HASH_MISMATCH' }
    $profileHash = Get-NxDHash $profile
    if ($profileHash -cne '672f8e2c29f162cdc74a8a8dfe7129955436c771db454b0a3ebb20a3bf9b560a') { throw 'PROFILE_HASH_MISMATCH' }
    $containerProfileHash = ((Invoke-NxDNative docker @('exec', $container, 'sha256sum', '/models/qwen3-8b-q4km-nxb21d-selection-lowmem-dev.yaml')) -split '\s+')[0]
    if ($containerProfileHash -cne $profileHash) { throw 'CONTAINER_PROFILE_HASH_MISMATCH' }
    if (@((Invoke-RestMethod -Uri ($url + '/v1/models') -TimeoutSec 15).data.id) -notcontains $model) { throw 'MODEL_REGISTRATION_MISSING' }
    if (Test-Loaded) { throw 'LOWMEM_MODEL_ALREADY_LOADED' }
    $before = Get-RuntimeState
    if ($before.active_jobs -ne 0) { throw "ACTIVE_JOBS:$($before.active_jobs)" }
    $availableBefore = Assert-AvailableRAM 6 'lowmem_before_load'

    New-Item -ItemType Directory -Force -Path $run | Out-Null
    New-Item -ItemType Directory -Force -Path $outputRoot | Out-Null
    [IO.File]::WriteAllText($lock, $run + "`n", [Text.UTF8Encoding]::new($false))
    Write-Host "LOWMEM_RESOURCE_SMOKE=START run=$run available_before_load_gib=$availableBefore"

    $profiler = Join-Path $PSScriptRoot 'measure_prompt_tokens.py'
    $corpus = Join-Path $root 'scripts\nxb21d-qwen3-8b-selection\qwen3-8b-selection-12case-corpus-v1.json'
    $bindings = Join-Path $root 'scripts\nxb21d-qwen3-8b-selection\schema-bindings-v1.json'
    $profileOutput = @(& python $profiler --corpus $corpus --bindings $bindings --model $model --url $url 2>&1)
    if ($LASTEXITCODE -ne 0) { throw "PROMPT_PROFILING_FAILED:$($profileOutput -join ' ')" }
    $promptProfile = ($profileOutput -join "`n") | ConvertFrom-Json
    Write-NxDJson (Join-Path $run 'prompt-token-profile.json') $promptProfile
    $modelMayBeLoaded = $true

    Start-Sleep -Seconds 15
    $availableAfterLoad = Assert-AvailableRAM 4 'lowmem_postload_steady_state'
    $backendAfterLoad = @(Get-BackendMemory)
    $apiAfterLoad = Get-APIContainerMemory
    $loadUpperBound = [int]$promptProfile.rows[0].prompt_tokenize_ms
    Write-Host "LOWMEM_LOAD=PASS available_after_load_gib=$availableAfterLoad load_upper_bound_ms=$loadUpperBound"

    $body = [ordered]@{
        model = $model
        temperature = 0
        max_tokens = 512
        messages = @(
            [ordered]@{role = 'system'; content = 'Synthetic resource smoke only. Return the single allowed decision and no other fields.'},
            [ordered]@{role = 'user'; content = 'Select RESOURCE_SMOKE_OK.'}
        )
        response_format = [ordered]@{
            type = 'json_schema'
            json_schema = [ordered]@{
                name = 'nxb21d_lowmem_resource_smoke'
                strict = $true
                schema = [ordered]@{
                    type = 'object'
                    additionalProperties = $false
                    required = @('decision')
                    properties = [ordered]@{decision = [ordered]@{type = 'string'; enum = @('RESOURCE_SMOKE_OK')}}
                }
            }
        }
    } | ConvertTo-Json -Depth 20 -Compress
    $timer = [Diagnostics.Stopwatch]::StartNew()
    $response = Invoke-RestMethod -Method Post -Uri ($url + '/v1/chat/completions') -ContentType 'application/json; charset=utf-8' -Body ([Text.Encoding]::UTF8.GetBytes($body)) -TimeoutSec 180
    $timer.Stop()
    $content = [string]$response.choices[0].message.content
    $decoded = $content | ConvertFrom-Json
    if ([string]$decoded.decision -cne 'RESOURCE_SMOKE_OK') { throw 'GENERIC_SMOKE_DECISION_INVALID' }
    if ([string]$response.choices[0].finish_reason -cne 'stop') { throw 'GENERIC_SMOKE_FINISH_REASON_INVALID' }
    $availableAfterCall = Get-NxDRamGiB
    if ($availableAfterCall -lt 4) { throw "RAM_TOO_LOW_AFTER_GENERIC_CALL:$availableAfterCall" }
    $backendAfterCall = @(Get-BackendMemory)
    $apiAfterCall = Get-APIContainerMemory

    Invoke-RestMethod -Method Post -Uri ($url + '/backend/shutdown') -ContentType 'application/json' -Body (@{model = $model} | ConvertTo-Json -Compress) -TimeoutSec 30 | Out-Null
    Start-Sleep -Seconds 5
    if (Test-Loaded) { throw 'MODEL_UNLOAD_FAILED' }
    $modelMayBeLoaded = $false
    $after = Get-RuntimeState
    if ($after.retained_tuple -cne $before.retained_tuple -or $after.activity_count -ne $before.activity_count -or $after.active_jobs -ne 0) { throw 'RUNTIME_INTEGRITY_FAILED' }

    $maxRSS = [math]::Round((@($backendAfterCall.rss_mib) | Measure-Object -Maximum).Maximum, 1)
    $receipt = [ordered]@{
        schema_version = 'nexusai.nxb21d-qwen3-8b-lowmem-resource-smoke/v1'
        status = 'PASS'
        development_only = $true
        benchmark_quality_evidence = $false
        model = $model
        artifact_sha256 = 'd98cdcbd03e17ce47681435b5150e34c1417f50b5c0019dd560e4882c5745785'
        profile_sha256 = $profileHash
        context_size = 1536
        n_batch = 128
        threads = 4
        parallel = 1
        mmap = $true
        mlock = $false
        gpu_layers = 0
        completion_budget = 512
        host_available_before_load_gib = $availableBefore
        host_available_after_load_gib = $availableAfterLoad
        host_available_after_generic_call_gib = $availableAfterCall
        model_load_time_upper_bound_ms = $loadUpperBound
        generic_call_latency_ms = $timer.ElapsedMilliseconds
        model_backend_rss_mib = $maxRSS
        backend_processes_after_load = $backendAfterLoad
        backend_processes_after_call = $backendAfterCall
        api_container_after_load = $apiAfterLoad
        api_container_after_call = $apiAfterCall
        prompt_profile = $promptProfile
        generic_response = [ordered]@{content = $content; finish_reason = $response.choices[0].finish_reason; usage = $response.usage}
        model_unload = 'PASS'
        runtime_integrity = 'PASS'
        retained_tuple = $after.retained_tuple
        activity_count = $after.activity_count
        active_jobs = $after.active_jobs
    }
    $receiptPath = Join-Path $run 'lowmem-resource-smoke-v1.json'
    Write-NxDJson $receiptPath $receipt
    [IO.File]::WriteAllText($receiptPath + '.sha256', (Get-NxDHash $receiptPath) + "`n", [Text.UTF8Encoding]::new($false))
    Write-Host "LOWMEM_RESOURCE_SMOKE=PASS receipt=$receiptPath"
} catch {
    Write-Host "LOWMEM_RESOURCE_SMOKE=FAIL error=$($_.Exception.Message)"
    if (Test-Path -LiteralPath $run) {
        $failurePath = Join-Path $run 'lowmem-resource-smoke-failure-v1.json'
        Write-NxDJson $failurePath ([ordered]@{status = 'FAIL'; error = $_.Exception.Message; model = $model; profile_sha256 = (Get-NxDHash $profile)})
        [IO.File]::WriteAllText($failurePath + '.sha256', (Get-NxDHash $failurePath) + "`n", [Text.UTF8Encoding]::new($false))
    }
    exit 1
} finally {
    if ($modelMayBeLoaded) {
        try {
            Invoke-RestMethod -Method Post -Uri ($url + '/backend/shutdown') -ContentType 'application/json' -Body (@{model = $model} | ConvertTo-Json -Compress) -TimeoutSec 30 | Out-Null
            Start-Sleep -Seconds 5
        } catch { Write-Host "LOWMEM_SAFE_UNLOAD_ERROR=$($_.Exception.Message)" }
    }
}
