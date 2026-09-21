# SPDX-License-Identifier: MIT
[CmdletBinding()]
param(
    [switch]$ValidateOnly
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$root = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$configPath = Join-Path $PSScriptRoot 'frozen-config-v1.json'
$config = Get-Content -LiteralPath $configPath -Raw | ConvertFrom-Json
$outputRoot = Join-Path $root 'local-acceptance-models\nxb21-english-functional-qualification'
$dispatchLock = Join-Path $outputRoot 'english-functional-corpus-v1.dispatched.lock'
$run = Join-Path $outputRoot ('run-' + [DateTime]::UtcNow.ToString('yyyyMMddTHHmmssfffZ'))
$evaluatorStarted = $false
$initiallyLoaded = $false
$availableBeforeDispatch = $null
$loadedRam = $null
$evaluatorPeakMiB = 0.0

function Resolve-RepoPath([string]$Relative) {
    return [IO.Path]::GetFullPath((Join-Path $root $Relative))
}

function Get-SHA256([string]$Path) {
    return (Get-FileHash -Algorithm SHA256 -LiteralPath $Path).Hash.ToLowerInvariant()
}

function Write-JSON([string]$Path, $Value) {
    $parent = Split-Path -Parent $Path
    if (-not (Test-Path -LiteralPath $parent)) {
        New-Item -ItemType Directory -Force -Path $parent | Out-Null
    }
    [IO.File]::WriteAllText($Path, ($Value | ConvertTo-Json -Depth 100) + "`n", [Text.UTF8Encoding]::new($false))
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

function Get-Containers {
    $names = @('nexusai-api-1', 'nexusai-forensic-records-api-1', 'nexusai-forensic-records-worker-1', 'nexusai-forensic-postgres-1', 'nexusai-forensic-nats-1')
    $rows = Invoke-Native docker (@('inspect') + $names) | ConvertFrom-Json
    $map = [ordered]@{}
    foreach ($row in $rows) {
        $name = $row.Name.TrimStart('/')
        $healthProperty = $row.State.PSObject.Properties['Health']
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

function Assert-HealthyContainers($Containers) {
    foreach ($name in $Containers.Keys) {
        $container = $Containers[$name]
        if ($container.status -cne 'running' -or $container.oom_killed -or $container.health -eq 'unhealthy') {
            throw "CONTAINER_UNHEALTHY:$name"
        }
    }
}

function Get-RuntimeState {
    $containers = Get-Containers
    Assert-HealthyContainers $containers
    $postgres = $containers.'nexusai-forensic-postgres-1'.id
    $tupleSQL = "SELECT (SELECT count(*) FROM forensic.evidence_items)||(chr(124))||(SELECT count(*) FROM forensic.evidence_versions)||(chr(124))||(SELECT count(*) FROM forensic.records_ingest_jobs)||(chr(124))||(SELECT count(*) FROM forensic.records)||(chr(124))||(SELECT count(*) FROM forensic.derived_artifacts)||(chr(124))||(SELECT count(*) FROM forensic.kb_collection_assets);"
    $jobsSQL = "SELECT count(*) FROM forensic.records_ingest_jobs WHERE status NOT IN ('completed','dead_letter');"
    $activitySQL = 'SELECT count(*) FROM public.agent_analysis_history;'
    return [ordered]@{
        containers = $containers
        retained_tuple = Invoke-Native docker @('exec', $postgres, 'psql', '-U', 'localrecall', '-d', 'localrecall', '-Atc', $tupleSQL)
        active_jobs = [int](Invoke-Native docker @('exec', $postgres, 'psql', '-U', 'localrecall', '-d', 'localrecall', '-Atc', $jobsSQL))
        activity_count = [int](Invoke-Native docker @('exec', $postgres, 'psql', '-U', 'localrecall', '-d', 'localrecall', '-Atc', $activitySQL))
        analyst_index_sha256 = Get-AnalystIndexSHA256
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

function Get-LoadedState {
    $system = Invoke-RestMethod -Method Get -Uri ([string]$config.runtime.localai_url + '/system') -TimeoutSec 20
    foreach ($loaded in @($system.loaded_models)) {
        if ([string]$loaded.id -ceq [string]$config.model.id) { return $loaded }
    }
    return $null
}

function Invoke-UnloadCurrentModel {
    $body = @{model = [string]$config.model.id} | ConvertTo-Json -Compress
    Invoke-RestMethod -Method Post -Uri ([string]$config.runtime.localai_url + '/backend/shutdown') -ContentType 'application/json' -Body $body -TimeoutSec 30 | Out-Null
}

function Invoke-GovernedCleanCacheReclaim {
    $dirty = Invoke-Native wsl.exe @('--distribution', 'docker-desktop', '--user', 'root', '--exec', 'grep', '^Dirty:', '/proc/meminfo')
    $writeback = Invoke-Native wsl.exe @('--distribution', 'docker-desktop', '--user', 'root', '--exec', 'grep', '^Writeback:', '/proc/meminfo')
    if ($dirty -notmatch '^Dirty:\s+([0-9]+)\s+kB$') { throw 'CLEAN_CACHE_RECLAIM_DIRTY_STATE_UNAVAILABLE' }
    $dirtyKiB = [int64]$Matches[1]
    if ($writeback -notmatch '^Writeback:\s+([0-9]+)\s+kB$') { throw 'CLEAN_CACHE_RECLAIM_WRITEBACK_STATE_UNAVAILABLE' }
    $writebackKiB = [int64]$Matches[1]
    if ($writebackKiB -ne 0) { throw "CLEAN_CACHE_RECLAIM_BLOCKED_WRITEBACK:$writebackKiB" }
    $result = Invoke-Native wsl.exe @('--distribution', 'docker-desktop', '--user', 'root', '--exec', '/sbin/sysctl', '-w', 'vm.drop_caches=1')
    if ($result -notmatch 'vm.drop_caches\s*=\s*1') { throw 'CLEAN_CACHE_RECLAIM_FAILED' }
    Write-Host "CLEAN_FILE_CACHE_RECOVERY=PASS dirty_kib=$dirtyKiB writeback_kib=0 dirty_pages_preserved=true sync=false"
}

function Wait-ForRAMFloor([double]$Floor, [string]$Stage, [int]$TimeoutSeconds) {
    $deadline = [DateTime]::UtcNow.AddSeconds($TimeoutSeconds)
    do {
        $samples = @()
        for ($index = 0; $index -lt 3; $index++) {
            $samples += Get-FreeRAMGiB
            if ($index -lt 2) { Start-Sleep -Seconds 5 }
        }
        $minimum = [double](@($samples) | Measure-Object -Minimum).Minimum
        Write-Host "RAM_GATE stage=$Stage samples_gib=$($samples -join ',') required_gib=$Floor"
        if ($minimum -ge $Floor) { return [math]::Round($minimum, 3) }
        if ([DateTime]::UtcNow -lt $deadline) { Start-Sleep -Seconds 5 }
    } while ([DateTime]::UtcNow -lt $deadline)
    throw "RAM_TOO_LOW:$Stage minimum_gib=$minimum required_gib=$Floor"
}

function Get-ContractDigest {
    $ordered = [ordered]@{}
    $names = @($config.contract.files.PSObject.Properties.Name)
    [Array]::Sort($names, [StringComparer]::Ordinal)
    foreach ($name in $names) {
        $actual = Get-SHA256 (Resolve-RepoPath $name)
        $expected = [string]$config.contract.files.PSObject.Properties[$name].Value
        if ($actual -cne $expected) { throw "CONTRACT_FILE_DRIFT:$name" }
        $ordered[$name] = $actual
    }
    $json = $ordered | ConvertTo-Json -Compress
    $sha = [Security.Cryptography.SHA256]::Create()
    try {
        return ([BitConverter]::ToString($sha.ComputeHash([Text.Encoding]::UTF8.GetBytes($json)))).Replace('-', '').ToLowerInvariant()
    }
    finally { $sha.Dispose() }
}

function Assert-FrozenInputs {
    foreach ($property in $config.files.PSObject.Properties) {
        $actual = Get-SHA256 (Resolve-RepoPath $property.Name)
        if ($actual -cne [string]$property.Value) { throw "FROZEN_INPUT_DRIFT:$($property.Name)" }
    }
    $contractDigest = Get-ContractDigest
    if ($contractDigest -cne [string]$config.contract.sha256) { throw 'CONTRACT_DIGEST_DRIFT' }
    $binaryHash = Get-SHA256 (Resolve-RepoPath ([string]$config.evaluator.path))
    if ($binaryHash -cne [string]$config.evaluator.sha256) { throw 'EVALUATOR_DRIFT' }
}

function Assert-ModelArtifacts($Before) {
    $api = $Before.containers.'nexusai-api-1'.id
    $artifact = ((Invoke-Native docker @('exec', $api, 'sha256sum', [string]$config.model.artifact_path)) -split '\s+')[0]
    $profile = ((Invoke-Native docker @('exec', $api, 'sha256sum', [string]$config.model.profile_path)) -split '\s+')[0]
    $bytes = [int64](Invoke-Native docker @('exec', $api, 'stat', '-c', '%s', [string]$config.model.artifact_path))
    if ($artifact -cne [string]$config.model.artifact_sha256 -or $profile -cne [string]$config.model.profile_sha256 -or $bytes -ne [int64]$config.model.artifact_bytes) {
        throw 'MODEL_OR_PROFILE_DRIFT'
    }
    $models = @((Invoke-RestMethod -Method Get -Uri ([string]$config.runtime.localai_url + '/v1/models') -TimeoutSec 20).data.id)
    if ($models -notcontains [string]$config.model.id) { throw 'CURRENT_MODEL_NOT_REGISTERED' }
}

function Assert-RuntimeUnchanged($Before, $After) {
    if ($Before.retained_tuple -cne $After.retained_tuple) { throw 'RETAINED_TUPLE_DRIFT' }
    if ($Before.activity_count -ne $After.activity_count) { throw 'ACTIVITY_DRIFT' }
    if ($After.active_jobs -ne 0) { throw "ACTIVE_JOBS:$($After.active_jobs)" }
    if ($Before.analyst_index_sha256 -cne $After.analyst_index_sha256) { throw 'PREMIUM_UI_DRIFT' }
    foreach ($name in $Before.containers.Keys) {
        $a = $Before.containers[$name]
        $b = $After.containers[$name]
        if ($a.id -cne $b.id -or $a.image -cne $b.image -or $a.restart_count -ne $b.restart_count) {
            throw "CONTAINER_DRIFT:$name"
        }
    }
}

function Invoke-FreshnessCheck([string]$OutputPath) {
    $arguments = @(
        (Resolve-RepoPath 'scripts/nxb21-english-functional-qualification/validate_freshness.py'),
        '--repo', $root,
        '--corpus', (Resolve-RepoPath ([string]$config.corpus.path)),
        '--oracle', (Resolve-RepoPath ([string]$config.oracle.path)),
        '--certification', (Resolve-RepoPath 'api/forensic_records/contracts/operation-certification-v1.json')
    )
    $raw = Invoke-Native python $arguments
    $freshness = $raw | ConvertFrom-Json
    Write-JSON $OutputPath $freshness
    if ([string]$freshness.status -cne 'PASS') { throw 'FRESHNESS_CHECK_FAILED' }
    return $freshness
}

function Write-ResourceSample([string]$Path, $Process) {
    $processMiB = 0.0
    try {
        $Process.Refresh()
        $processMiB = [math]::Round($Process.WorkingSet64 / 1MB, 3)
    }
    catch {}
    $statsObject = $null
    try {
        $stats = Invoke-Native docker @('stats', '--no-stream', '--format', '{{json .}}', 'nexusai-api-1')
        $statsObject = $stats | ConvertFrom-Json
    }
    catch {}
    $sample = [ordered]@{
        utc = [DateTime]::UtcNow.ToString('o')
        available_ram_gib = Get-FreeRAMGiB
        evaluator_working_set_mib = $processMiB
        api_docker_stats = $statsObject
    }
    [IO.File]::AppendAllText($Path, ($sample | ConvertTo-Json -Depth 10 -Compress) + "`n", [Text.UTF8Encoding]::new($false))
    return $processMiB
}

New-Item -ItemType Directory -Force -Path $run | Out-Null
$receiptPath = Join-Path $run 'operator-receipt-v1.json'
$resultPath = Join-Path $run 'qualification-intermediate-v1.json'
$resourcePath = Join-Path $run 'resource-observations.ndjson'
$stdoutPath = Join-Path $run 'evaluator.stdout.log'
$stderrPath = Join-Path $run 'evaluator.stderr.log'
$before = $null
$after = $null
$freshness = $null
$status = 'FAIL'
$errorMessage = ''

try {
    if (Test-Path -LiteralPath $dispatchLock) { throw 'QUALIFICATION_CORPUS_ALREADY_CONSUMED' }
    Assert-FrozenInputs
    $branch = Invoke-Native git @('branch', '--show-current')
    $head = Invoke-Native git @('rev-parse', 'HEAD')
    if ($branch -cne [string]$config.source.branch -or $head -cne [string]$config.source.head) { throw 'SOURCE_BRANCH_OR_HEAD_DRIFT' }
    $statusLines = @((Invoke-Native git @('status', '--short')) -split '\r?\n' | Where-Object { $_ -ne '' })
    $before = Get-RuntimeState
    if ($before.active_jobs -ne 0) { throw "ACTIVE_JOBS:$($before.active_jobs)" }
    if ($before.retained_tuple -cne [string]$config.runtime.retained_tuple -or $before.activity_count -ne [int]$config.runtime.activity_count -or $before.analyst_index_sha256 -cne [string]$config.runtime.analyst_index_sha256) {
        throw 'FROZEN_RUNTIME_STATE_DRIFT'
    }
    Assert-ModelArtifacts $before
    $freshness = Invoke-FreshnessCheck (Join-Path $run 'freshness-v1.json')
    $loadedState = Get-LoadedState
    $initiallyLoaded = $null -ne $loadedState

    $freeze = [ordered]@{
        contract_version = 'nexusai.english-functional-freeze/v1'
        frozen_at = [DateTime]::UtcNow.ToString('o')
        corpus_id = [string]$config.corpus.id
        corpus_sha256 = [string]$config.corpus.sha256
        oracle_sha256 = [string]$config.oracle.sha256
        model_sha256 = [string]$config.model.artifact_sha256
        profile_sha256 = [string]$config.model.profile_sha256
        evaluator_sha256 = [string]$config.evaluator.sha256
        contract_sha256 = [string]$config.contract.sha256
        branch = $branch
        head = $head
        git_status_entries = $statusLines.Count
        freshness = $freshness
        runtime_before = $before
        current_model_initially_loaded = $initiallyLoaded
        qwen3_8b_state = 'CLOSED_RESOURCE_INSUFFICIENT'
        qualification_corpus = 'NOT_CONSUMED'
    }
    $freezePath = Join-Path $run 'freeze-v1.json'
    Write-JSON $freezePath $freeze
    [IO.File]::WriteAllText($freezePath + '.sha256', (Get-SHA256 $freezePath) + "`n", [Text.UTF8Encoding]::new($false))

    if ($ValidateOnly) {
        $after = Get-RuntimeState
        Assert-RuntimeUnchanged $before $after
        $status = 'PASS'
        Write-Host "QUALIFICATION_PREFLIGHT_VALIDATION=PASS run=$run corpus_consumed=false"
        return
    }

    if ($initiallyLoaded) {
        $currentLoadedRAM = Get-FreeRAMGiB
        Write-Host "CURRENT_MODEL_STATE=LOADED available_ram_gib=$currentLoadedRAM required_loaded_gib=4"
        if ($currentLoadedRAM -lt 4) {
            Write-Host 'CURRENT_MODEL_UNLOAD=REQUIRED_FOR_GOVERNED_PRELOAD_GATE'
            Invoke-UnloadCurrentModel
            Start-Sleep -Seconds 5
            if ($null -ne (Get-LoadedState)) { throw 'CURRENT_MODEL_UNLOAD_FAILED' }
            Invoke-GovernedCleanCacheReclaim
            $availableBeforeDispatch = Wait-ForRAMFloor 6 'before_loading_unloaded_current_model' 120
        }
        else {
            $availableBeforeDispatch = $currentLoadedRAM
            $loadedRam = $currentLoadedRAM
        }
    }
    else {
        Invoke-GovernedCleanCacheReclaim
        $availableBeforeDispatch = Wait-ForRAMFloor 6 'before_loading_unloaded_current_model' 120
    }

    Write-Host "QUALIFICATION_FREEZE=PASS run=$run"
    Write-Host "PRELOAD_RAM_GIB=$availableBeforeDispatch"
    $env:NEXUSAI_ENGLISH_FUNCTIONAL_LIVE = '1'
    $env:NEXUSAI_ENGLISH_CORPUS = Resolve-RepoPath ([string]$config.corpus.path)
    $env:NEXUSAI_ENGLISH_ORACLE = Resolve-RepoPath ([string]$config.oracle.path)
    $env:NEXUSAI_ENGLISH_RUN_DIR = $run
    $env:NEXUSAI_ENGLISH_DISPATCH_LOCK = $dispatchLock
    $env:NEXUSAI_ENGLISH_MODEL = [string]$config.model.id
    $env:NEXUSAI_ENGLISH_LOCALAI_URL = [string]$config.runtime.localai_url
    $env:NEXUSAI_ENGLISH_RAM_CHECK = Resolve-RepoPath 'scripts/nxb21-english-functional-qualification/check_loaded_ram.ps1'

    $arguments = @('-test.run', '^TestEnglishFunctionalQualificationOneShot$', '-test.v', '-test.timeout', '3h')
    $process = Start-Process -FilePath (Resolve-RepoPath ([string]$config.evaluator.path)) -ArgumentList $arguments -WorkingDirectory $root -NoNewWindow -PassThru -RedirectStandardOutput $stdoutPath -RedirectStandardError $stderrPath
    $evaluatorStarted = $true
    Write-Host "QUALIFICATION_CORPUS=CONSUMED evaluator_pid=$($process.Id)"
    do {
        $sampleMiB = Write-ResourceSample $resourcePath $process
        if ($sampleMiB -gt $evaluatorPeakMiB) { $evaluatorPeakMiB = $sampleMiB }
        if (-not $process.HasExited) { Start-Sleep -Seconds 10 }
    } while (-not $process.HasExited)
    $process.WaitForExit()
    if ($process.ExitCode -ne 0) {
        $tail = if (Test-Path -LiteralPath $stdoutPath) { (Get-Content -LiteralPath $stdoutPath -Tail 20) -join ' | ' } else { '' }
        throw "EVALUATOR_FAILED:$($process.ExitCode):$tail"
    }
    $log = Get-Content -LiteralPath $stdoutPath -Raw
    if ($log -match 'MODEL_LOADED_RAM available_gib=([0-9.]+)') { $loadedRam = [double]$Matches[1] }
    if ($null -eq $loadedRam) { $loadedRam = Get-FreeRAMGiB }
    if ($loadedRam -lt 4) { throw "LOADED_RAM_TOO_LOW:$loadedRam" }
    if (-not (Test-Path -LiteralPath $resultPath)) { throw 'QUALIFICATION_RESULT_MISSING' }
    $result = Get-Content -LiteralPath $resultPath -Raw | ConvertFrom-Json
    if ([string]$result.gate -cne 'PASS') { throw 'QUALIFICATION_GATE_FAILED' }

    if (-not $initiallyLoaded) {
        Invoke-UnloadCurrentModel
        Start-Sleep -Seconds 5
        if ($null -ne (Get-LoadedState)) { throw 'POST_QUALIFICATION_MODEL_UNLOAD_FAILED' }
    }
    $after = Get-RuntimeState
    Assert-RuntimeUnchanged $before $after
    $status = 'PASS'
}
catch {
    $errorMessage = $_.Exception.Message
    Write-Host "ENGLISH_FUNCTIONAL_QUALIFICATION=FAIL error=$errorMessage"
    if ($errorMessage -like 'RAM_TOO_LOW:*' -and -not (Test-Path -LiteralPath $dispatchLock)) {
        Write-Host 'QUALIFICATION_CORPUS=NOT_CONSUMED'
        Write-Host "STANDALONE_RERUN_COMMAND=powershell -NoProfile -ExecutionPolicy Bypass -File `"$PSCommandPath`""
    }
}
finally {
    if ($evaluatorStarted -and $null -ne (Get-LoadedState) -and (Get-FreeRAMGiB) -lt 4) {
        try {
            Invoke-UnloadCurrentModel
            Start-Sleep -Seconds 5
            Write-Host 'CURRENT_MODEL_SAFE_UNLOAD=PASS reason=loaded_ram_below_floor'
        }
        catch { Write-Host "CURRENT_MODEL_SAFE_UNLOAD=FAIL error=$($_.Exception.Message)" }
    }
    if ($null -eq $after) {
        try { $after = Get-RuntimeState } catch {}
    }
    $receipt = [ordered]@{
        contract_version = 'nexusai.english-functional-operator-receipt/v1'
        status = $status
        error = $errorMessage
        completed_at = [DateTime]::UtcNow.ToString('o')
        run_directory = $run
        corpus_consumed = Test-Path -LiteralPath $dispatchLock
        preload_ram_gib = $availableBeforeDispatch
        loaded_ram_gib = $loadedRam
        evaluator_peak_working_set_mib = $evaluatorPeakMiB
        build_peak_memory = 'NOT_OBSERVED'
        live_harness_overhead_mib = $evaluatorPeakMiB
        runtime_before = $before
        runtime_after = $after
        result_path = $(if (Test-Path -LiteralPath $resultPath) { $resultPath } else { $null })
        stdout_path = $stdoutPath
        stderr_path = $stderrPath
        resource_observations_path = $resourcePath
    }
    Write-JSON $receiptPath $receipt
    [IO.File]::WriteAllText($receiptPath + '.sha256', (Get-SHA256 $receiptPath) + "`n", [Text.UTF8Encoding]::new($false))
    Write-Host "OPERATOR_RECEIPT=$receiptPath"
}

if ($status -cne 'PASS') { exit 1 }
Write-Host "ENGLISH_FUNCTIONAL_QUALIFICATION=PASS result=$resultPath"
