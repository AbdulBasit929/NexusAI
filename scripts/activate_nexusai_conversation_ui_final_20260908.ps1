param(
    [switch]$ValidateOnly,
    [switch]$Rollback
)

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

$RepoRoot = 'C:\Users\sheik\Workspace\Office\Projects\NexusAI'
$ContextDir = Join-Path $RepoRoot 'reports\nxb21\investigation-workspace-conversation-ui-final-build-context-20260908\context'
$ManifestPath = Join-Path $RepoRoot 'reports\nxb21\investigation-workspace-conversation-ui-final-deployment-manifest-20260908.json'
$ManifestHashPath = "$ManifestPath.sha256"
$ReceiptDir = Join-Path $RepoRoot 'reports\nxb21\investigation-workspace-conversation-ui-final-activation-20260908'
$ReceiptPath = Join-Path $ReceiptDir 'activation-receipt.json'
$ReceiptHashPath = "$ReceiptPath.sha256"

$BaseCompose = Join-Path $RepoRoot 'local-acceptance-models\nxmmr\private-activation-video-result\20260903T063719719Z\activation-compose.json'
$ActivationOverride = Join-Path $ContextDir 'compose.ui-only.yaml'
$RollbackOverride = Join-Path $ContextDir 'compose.rollback.yaml'

$BaseImage = 'sha256:bed2eec364ca8c65151625685f79f5ab7a606a8223af88d1696504751b4a03a1'
$ActivationImage = 'nexusai/localai-forensic:conversation-ui-final-20260908'
$RollbackImage = 'nexusai/localai-forensic:rollback-before-conversation-ui-final-20260908'
$RequiredFreeRamKiB = 6291456
$ReadyUrl = 'http://localhost:8080/readyz'
$AnalystUrl = 'http://localhost:8080/analyst'
$RerunCommand = 'powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\activate_nexusai_conversation_ui_final_20260908.ps1'

$ProtectedServices = @(
    'forensic-records-api',
    'forensic-records-worker',
    'forensic-nats',
    'forensic-postgres'
)

function Write-Marker {
    param([Parameter(Mandatory=$true)][string]$Text)
    Write-Host $Text
}

function Assert-ExitCode {
    param([Parameter(Mandatory=$true)][string]$Message)
    if ($LASTEXITCODE -ne 0) {
        throw $Message
    }
}

function Get-ServiceContainerID {
    param([Parameter(Mandatory=$true)][string]$Service)

    $ids = @(
        docker ps -q --no-trunc `
            --filter 'label=com.docker.compose.project=nexusai' `
            --filter "label=com.docker.compose.service=$Service" |
        Where-Object { -not [string]::IsNullOrWhiteSpace($_) }
    )
    Assert-ExitCode "Could not inspect Compose service '$Service'."

    if ($ids.Count -ne 1) {
        throw "Expected exactly one running '$Service' container; found $($ids.Count)."
    }
    return $ids[0].Trim()
}

function Get-ContainerImageID {
    param([Parameter(Mandatory=$true)][string]$ContainerID)

    $value = docker inspect $ContainerID --format '{{.Image}}'
    Assert-ExitCode "Could not inspect image for container $ContainerID."
    if ([string]::IsNullOrWhiteSpace($value)) {
        throw "Container image ID is empty for $ContainerID."
    }
    return $value.Trim()
}

function Get-ContainerRestartCount {
    param([Parameter(Mandatory=$true)][string]$ContainerID)
    $value = docker inspect $ContainerID --format '{{.RestartCount}}'
    Assert-ExitCode "Could not inspect restart count for container $ContainerID."
    return [int]$value
}

function Get-ProtectedSnapshot {
    $snapshot = [ordered]@{}
    foreach ($service in $ProtectedServices) {
        $container = Get-ServiceContainerID $service
        $snapshot[$service] = [ordered]@{
            container_id = $container
            image_id = Get-ContainerImageID $container
            restart_count = Get-ContainerRestartCount $container
        }
    }
    return $snapshot
}

function Assert-ProtectedSnapshot {
    param([Parameter(Mandatory=$true)]$Before)

    foreach ($service in $ProtectedServices) {
        $container = Get-ServiceContainerID $service
        $image = Get-ContainerImageID $container
        if ($container -ne $Before[$service].container_id) {
            throw "Protected service '$service' container changed."
        }
        if ($image -ne $Before[$service].image_id) {
            throw "Protected service '$service' image changed."
        }
        $restartCount = Get-ContainerRestartCount $container
        if ($restartCount -ne [int]$Before[$service].restart_count) {
            throw "Protected service '$service' restart count changed."
        }
    }
    Write-Marker 'PROTECTED_SERVICES_PRESERVED=true'
}

function Get-ContainerEnvValue {
    param(
        [Parameter(Mandatory=$true)][string]$ContainerID,
        [Parameter(Mandatory=$true)][string]$Name
    )

    $lines = docker inspect $ContainerID --format '{{range .Config.Env}}{{println .}}{{end}}'
    Assert-ExitCode "Could not inspect environment for $ContainerID."
    $entry = $lines | Where-Object { $_ -like "$Name=*" } | Select-Object -First 1
    if ([string]::IsNullOrWhiteSpace($entry)) {
        throw "Required container environment variable '$Name' is unavailable."
    }
    return $entry.Substring($Name.Length + 1)
}

function Get-RetainedTuple {
    $postgres = Get-ServiceContainerID 'forensic-postgres'
    $pgUser = Get-ContainerEnvValue -ContainerID $postgres -Name 'POSTGRES_USER'
    $pgDb = Get-ContainerEnvValue -ContainerID $postgres -Name 'POSTGRES_DB'

    $sql = @"
SELECT
  (SELECT count(*) FROM forensic.evidence_items),
  (SELECT count(*) FROM forensic.evidence_versions),
  (SELECT count(*) FROM forensic.records_ingest_jobs),
  (SELECT count(*) FROM forensic.records),
  (SELECT count(*) FROM forensic.derived_artifacts),
  (SELECT count(*) FROM forensic.kb_collection_assets);
"@

    $output = docker exec $postgres psql -U $pgUser -d $pgDb -At -F '|' -c $sql
    Assert-ExitCode 'Could not read the retained-state tuple.'
    $tuple = $output | Where-Object { $_ -match '^\d+\|\d+\|\d+\|\d+\|\d+\|\d+$' } | Select-Object -Last 1
    if ([string]::IsNullOrWhiteSpace($tuple)) {
        throw 'Could not parse the retained-state tuple.'
    }
    return $tuple.Trim()
}

function Get-ScalarQuery {
    param([Parameter(Mandatory=$true)][string]$Sql)
    $postgres = Get-ServiceContainerID 'forensic-postgres'
    $pgUser = Get-ContainerEnvValue -ContainerID $postgres -Name 'POSTGRES_USER'
    $pgDb = Get-ContainerEnvValue -ContainerID $postgres -Name 'POSTGRES_DB'
    $value = docker exec $postgres psql -U $pgUser -d $pgDb -At -c $Sql
    Assert-ExitCode 'Could not read an authoritative forensic scalar.'
    $parsed = $value | Where-Object { $_ -match '^\d+$' } | Select-Object -Last 1
    if ([string]::IsNullOrWhiteSpace($parsed)) { throw 'Could not parse an authoritative forensic scalar.' }
    return [int]$parsed
}

function Assert-ZeroActiveJobs {
    $count = Get-ScalarQuery "SELECT count(*) FROM forensic.records_ingest_jobs WHERE status NOT IN ('completed','dead_letter');"
    Write-Marker "ACTIVE_JOBS=$count"
    if ($count -ne 0) { throw "UI activation is blocked while $count forensic job(s) are active." }
    return $count
}

function Get-ActivityCount {
    return Get-ScalarQuery 'SELECT count(*) FROM public.agent_analysis_history;'
}

function Wait-ForHttp200 {
    param(
        [Parameter(Mandatory=$true)][string]$Url,
        [int]$TimeoutSeconds = 300,
        [string]$Accept = '*/*'
    )

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        try {
            $response = Invoke-WebRequest -UseBasicParsing -TimeoutSec 15 -Headers @{Accept=$Accept} -Uri $Url
            if ([int]$response.StatusCode -eq 200) {
                return $response
            }
        }
        catch {
            # The API may still be starting.
        }
        Start-Sleep -Seconds 3
    }
    throw "Timed out waiting for HTTP 200 from $Url."
}

function Get-LoadedModels {
    $system = Invoke-RestMethod -Method Get -Uri 'http://localhost:8080/system' -TimeoutSec 20
    return @($system.loaded_models.id | Where-Object { -not [string]::IsNullOrWhiteSpace($_) } | Sort-Object -Unique)
}

function Assert-NoActiveDModelSelection {
    $runnerPattern = '(?i)(run_nxb21d_.*(?:development|qualification)\.ps1|run_(?:fresh|final)_local_selection12_development\.ps1|NXB21D_(?:LOWMEM|Q4))'
    $hostRunners = @(Get-CimInstance Win32_Process | Where-Object {
        $_.ProcessId -ne $PID -and $_.CommandLine -and $_.CommandLine -match $runnerPattern
    })
    if ($hostRunners.Count -gt 0) {
        Write-Marker 'UI_ACTIVATION=BLOCKED_ACTIVE_MODEL_SELECTION'
        throw 'An NXB21D model-selection runner is active.'
    }

    $api = Get-ServiceContainerID 'api'
    $apiProcesses = docker top $api -eo pid,args
    Assert-ExitCode 'Could not inspect API container processes.'
    $ownedBackend = @($apiProcesses | Where-Object { $_ -match '(?i)qwen3[-_/ ]?8b|nxb21d' })
    $loaded = @(Get-LoadedModels)
    $ownedLoaded = @($loaded | Where-Object { $_ -match '(?i)qwen3[-_/ ]?8b|nxb21d' })
    if ($ownedBackend.Count -gt 0 -or $ownedLoaded.Count -gt 0) {
        Write-Marker 'UI_ACTIVATION=BLOCKED_ACTIVE_MODEL_SELECTION'
        throw 'The model-selection-owned Qwen3 8B backend is loaded or executing.'
    }
    Write-Marker 'D_MODEL_SELECTION_ACTIVE=false'
    Write-Marker 'D_OWNED_MODEL_EXECUTING=false'
    return $loaded
}

function Get-ByteHash {
    param([Parameter(Mandatory=$true)][byte[]]$Bytes)

    $sha = [System.Security.Cryptography.SHA256]::Create()
    try {
        return (($sha.ComputeHash($Bytes) | ForEach-Object { $_.ToString('x2') }) -join '')
    }
    finally {
        $sha.Dispose()
    }
}

function Assert-ContextManifest {
    if (-not (Test-Path -LiteralPath $ManifestPath)) {
        throw "Deployment manifest is missing: $ManifestPath"
    }
    if (-not (Test-Path -LiteralPath $ManifestHashPath)) {
        throw "Deployment manifest checksum is missing: $ManifestHashPath"
    }
    if (-not (Test-Path -LiteralPath $ContextDir)) {
        throw "Build context is missing: $ContextDir"
    }

    $expectedManifestHash = ((Get-Content -Raw -LiteralPath $ManifestHashPath).Trim() -split '\s+')[0].ToLowerInvariant()
    $actualManifestHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $ManifestPath).Hash.ToLowerInvariant()
    if ($expectedManifestHash -ne $actualManifestHash) {
        throw 'Deployment manifest checksum mismatch.'
    }

    $manifest = Get-Content -Raw -LiteralPath $ManifestPath | ConvertFrom-Json
    if ($manifest.base_image_id -ne $BaseImage) {
        throw 'Deployment manifest is not bound to the accepted live base image.'
    }
    if ($manifest.d_unqualified_source_included -ne $false) {
        throw 'Deployment manifest does not fail closed for D-unqualified source.'
    }

    $actualFiles = @(Get-ChildItem -LiteralPath $ContextDir -Recurse -File)
    if ($actualFiles.Count -ne $manifest.context_files.Count) {
        throw "Build context file count mismatch. Expected $($manifest.context_files.Count), found $($actualFiles.Count)."
    }

    $allowed = New-Object 'System.Collections.Generic.HashSet[string]' ([System.StringComparer]::OrdinalIgnoreCase)
    foreach ($entry in $manifest.context_files) {
        $relative = $entry.path.Replace('/', [IO.Path]::DirectorySeparatorChar)
        $full = Join-Path $ContextDir $relative
        if (-not (Test-Path -LiteralPath $full -PathType Leaf)) {
            throw "Manifest-owned build-context file is missing: $($entry.path)"
        }
        $hash = (Get-FileHash -Algorithm SHA256 -LiteralPath $full).Hash.ToLowerInvariant()
        if ($hash -ne $entry.sha256.ToLowerInvariant()) {
            throw "Build-context checksum mismatch: $($entry.path)"
        }
        if ((Get-Item -LiteralPath $full).Length -ne [long]$entry.bytes) {
            throw "Build-context size mismatch: $($entry.path)"
        }
        [void]$allowed.Add($entry.path)
    }

    foreach ($file in $actualFiles) {
        $relative = $file.FullName.Substring($ContextDir.Length + 1).Replace('\', '/')
        if (-not $allowed.Contains($relative)) {
            throw "Unowned file exists in the sealed build context: $relative"
        }
        if ($relative -match '^(api|core|configuration|scripts|reports)/') {
            throw "Production source directory is forbidden in the UI-only build context: $relative"
        }
    }

    $textFiles = @($actualFiles | Where-Object {
        $_.Name -eq 'Dockerfile' -or $_.Extension -in @('.html','.css','.js','.json','.yaml','.yml','.txt')
    })
    $forbidden = Select-String -LiteralPath $textFiles.FullName -Pattern 'nxb21d|q4_full_plan_role|d_dynamic_planner|decision8|qwen3-8b|qwen/qwen3' -CaseSensitive:$false
    if ($forbidden) {
        throw 'A forbidden D-development marker exists in the sealed UI-only context.'
    }

    Write-Marker 'DEPLOYMENT_MANIFEST=PASS'
    Write-Marker 'CONTEXT_INTEGRITY=PASS'
    Write-Marker 'D_UNQUALIFIED_SOURCE_INCLUDED=NO'
    return $manifest
}

function Assert-ComposeOverlay {
    param([Parameter(Mandatory=$true)][string]$OverridePath)

    $json = docker compose -p nexusai -f $BaseCompose -f $OverridePath config --format json
    Assert-ExitCode "Compose validation failed for $OverridePath."
    $config = $json | ConvertFrom-Json
    if ($null -eq $config.services.api) {
        throw 'Merged Compose configuration does not define api.'
    }

    $api = $config.services.api
    $expectedTarget = 8088
    $expectedEntrypoint = '/usr/local/bin/nexusai-ui-wrapper'
    $published = @($api.ports | Where-Object { [string]$_.published -eq '8080' -and [int]$_.target -eq $expectedTarget })
    if ($published.Count -ne 1) {
        throw "Merged Compose configuration does not publish host 8080 to target $expectedTarget exactly once."
    }

    $entrypoint = @($api.entrypoint) -join ' '
    if ($entrypoint -ne $expectedEntrypoint) {
        throw "Unexpected merged API entrypoint: $entrypoint"
    }
    Write-Marker "COMPOSE_OVERLAY_$expectedTarget=PASS"
}

function Get-AvailableRamGiB {
    $os = Get-CimInstance Win32_OperatingSystem
    return [math]::Round(([double]$os.FreePhysicalMemory / 1MB), 3)
}

function Invoke-BoundedCleanCacheRecovery {
    $writebackLine = & "$env:WINDIR\System32\wsl.exe" --distribution docker-desktop --user root --exec grep '^Writeback:' /proc/meminfo
    if ($LASTEXITCODE -ne 0 -or $writebackLine -notmatch '^Writeback:\s+0\s+kB$') {
        Write-Marker 'CLEAN_FILE_CACHE_RECOVERY=SKIPPED reason=writeback_nonzero_or_unavailable sync=false'
        return
    }
    & "$env:WINDIR\System32\wsl.exe" --distribution docker-desktop --user root --exec /sbin/sysctl -w vm.compact_memory=1 | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Bounded WSL memory compaction failed.' }
    & "$env:WINDIR\System32\wsl.exe" --distribution docker-desktop --user root --exec /sbin/sysctl -w vm.drop_caches=1 | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Bounded clean-cache recovery failed.' }
    Write-Marker 'CLEAN_FILE_CACHE_RECOVERY=PASS writeback_kib=0 dirty_pages_preserved=true sync=false'
}

function Assert-RamGate {
    $freeGiB = Get-AvailableRamGiB
    Write-Marker "CURRENT_AVAILABLE_RAM_GIB=$freeGiB"
    Write-Marker 'RAM_REQUIRED_GIB=6'

    if (($freeGiB * 1GB / 1KB) -lt $RequiredFreeRamKiB) {
        Invoke-BoundedCleanCacheRecovery
        $deadline = (Get-Date).AddSeconds(60)
        do {
            Start-Sleep -Seconds 5
            $freeGiB = Get-AvailableRamGiB
            Write-Marker "RAM_RECOVERY_AVAILABLE_GIB=$freeGiB"
            if (($freeGiB * 1GB / 1KB) -ge $RequiredFreeRamKiB) { break }
        } while ((Get-Date) -lt $deadline)
    }

    if (($freeGiB * 1GB / 1KB) -lt $RequiredFreeRamKiB) {
        Write-Marker 'RAM_GATE=FAIL'
        Write-Host 'LARGEST_PROCESSES_READ_ONLY:'
        Get-Process |
            Sort-Object WorkingSet64 -Descending |
            Select-Object -First 12 ProcessName, Id, @{Name='WorkingSetMiB';Expression={[math]::Round($_.WorkingSet64 / 1MB, 1)}} |
            Format-Table -AutoSize
        Write-Marker 'ACTIVATION=PENDING_RAM'
        Write-Marker 'UI_ACTIVATION=NOT_STARTED'
        Write-Marker 'MUTATION_PERFORMED=false'
        Write-Marker "RERUN_COMMAND=$RerunCommand"
        throw 'ACTIVATION=PENDING_RAM. Close applications manually and rerun the printed command.'
    }
    Write-Marker 'RAM_GATE=PASS'
}

function Invoke-ComposeRecreate {
    param([Parameter(Mandatory=$true)][string]$OverridePath)

    docker compose -p nexusai -f $BaseCompose -f $OverridePath up -d --no-deps --force-recreate --no-build --pull never api
    Assert-ExitCode 'The API-only Compose recreate failed.'
}

function Assert-LiveUIIndex {
    $expected = (Get-FileHash -Algorithm SHA256 -LiteralPath (Join-Path $ContextDir 'ui-dist\index.html')).Hash.ToLowerInvariant()
    $client = New-Object System.Net.WebClient
    try {
        $client.Headers['Accept'] = 'text/html'
        $bytes = $client.DownloadData($AnalystUrl)
    }
    finally {
        $client.Dispose()
    }
    $actual = Get-ByteHash $bytes
    if ($actual -ne $expected) {
        throw "The live /analyst route is not serving the sealed UI index. Expected=$expected Actual=$actual"
    }
    Write-Marker 'LIVE_ANALYST_INDEX=PASS'
    return $actual
}

function Get-LiveUIIndexHash {
    $client = New-Object System.Net.WebClient
    try {
        $client.Headers['Accept'] = 'text/html'
        return Get-ByteHash $client.DownloadData($AnalystUrl)
    }
    finally { $client.Dispose() }
}

function Write-ActivationReceipt {
    param(
        [Parameter(Mandatory=$true)]$Manifest,
        [Parameter(Mandatory=$true)][string]$BeforeContainer,
        [Parameter(Mandatory=$true)][string]$BeforeImage,
        [Parameter(Mandatory=$true)][int]$BeforeRestartCount,
        [Parameter(Mandatory=$true)][string]$BeforeIndexHash,
        [Parameter(Mandatory=$true)][string]$AfterContainer,
        [Parameter(Mandatory=$true)][string]$AfterImage,
        [Parameter(Mandatory=$true)][int]$AfterRestartCount,
        [Parameter(Mandatory=$true)][string]$BuiltImage,
        [Parameter(Mandatory=$true)][string]$BeforeTuple,
        [Parameter(Mandatory=$true)][string]$AfterTuple,
        [Parameter(Mandatory=$true)]$ProtectedBefore,
        [Parameter(Mandatory=$true)][int]$ActivityCount,
        [Parameter(Mandatory=$true)]$LoadedModelsBefore,
        [Parameter(Mandatory=$true)]$LoadedModelsAfter,
        [Parameter(Mandatory=$true)][string]$LiveIndexHash
    )

    if (-not (Test-Path -LiteralPath $ReceiptDir)) {
        New-Item -ItemType Directory -Force -Path $ReceiptDir | Out-Null
    }
    $receipt = [ordered]@{
        schema_version = 'nexusai.investigation-workspace-conversation-ui-final-activation-receipt/v1'
        activated_at_utc = (Get-Date).ToUniversalTime().ToString('o')
        status = 'LIVE_VERIFIED'
        canonical_route = '/analyst'
        source_manifest_sha256 = (Get-FileHash -Algorithm SHA256 -LiteralPath $ManifestPath).Hash.ToLowerInvariant()
        source_digest_sha256 = $Manifest.source_digest_sha256
        base_image_id = $BaseImage
        api_container_before = $BeforeContainer
        api_image_before = $BeforeImage
        api_restart_count_before = $BeforeRestartCount
        analyst_index_before_sha256 = $BeforeIndexHash
        api_container_after = $AfterContainer
        api_image_after = $AfterImage
        api_restart_count_after = $AfterRestartCount
        built_image_id = $BuiltImage
        activation_image = $ActivationImage
        rollback_image = $RollbackImage
        retained_tuple_before = $BeforeTuple
        retained_tuple_after = $AfterTuple
        activity_count_before = $ActivityCount
        activity_count_after = $ActivityCount
        protected_services_before = $ProtectedBefore
        protected_services_preserved = $true
        retained_data_mutated = $false
        database_migration = $false
        loaded_models_before = @($LoadedModelsBefore)
        loaded_models_after = @($LoadedModelsAfter)
        transient_model_runtime_reset_expected = $true
        d_owned_model_state_preserved_unloaded = $true
        configured_models_changed = $false
        d_unqualified_source_included = $false
        d_activation = 'BLOCKED'
        live_index_sha256 = $LiveIndexHash
        live_route_acceptance = 'PASS'
        live_backend_acceptance = 'PASS'
    }
    $receipt | ConvertTo-Json -Depth 12 | Set-Content -LiteralPath $ReceiptPath -Encoding UTF8
    $hash = (Get-FileHash -Algorithm SHA256 -LiteralPath $ReceiptPath).Hash.ToLowerInvariant()
    "$hash  activation-receipt.json" | Set-Content -LiteralPath $ReceiptHashPath -Encoding ASCII
    Write-Marker "ACTIVATION_RECEIPT=$ReceiptPath"
    Write-Marker "ACTIVATION_RECEIPT_SHA256=$hash"
}

if ($ValidateOnly -and $Rollback) {
    throw 'Choose either -ValidateOnly or -Rollback, not both.'
}
if (-not (Test-Path -LiteralPath $RepoRoot)) {
    throw "Repository is unavailable: $RepoRoot"
}
if (-not (Test-Path -LiteralPath $BaseCompose)) {
    throw "Accepted live Compose file is unavailable: $BaseCompose"
}

Set-Location $RepoRoot
$runtimeChanged = $false
$rollbackCreated = $false

try {
    $dockerVersion = docker info --format '{{.ServerVersion}}'
    Assert-ExitCode 'Docker Desktop Linux engine is not ready.'
    Write-Marker "DOCKER_SERVER_VERSION=$dockerVersion"

    $manifest = Assert-ContextManifest
    Assert-ComposeOverlay -OverridePath $ActivationOverride
    Assert-ComposeOverlay -OverridePath $RollbackOverride

    $apiBefore = Get-ServiceContainerID 'api'
    $apiImageBefore = Get-ContainerImageID $apiBefore
    $apiRestartBefore = Get-ContainerRestartCount $apiBefore
    $protectedBefore = Get-ProtectedSnapshot
    $tupleBefore = Get-RetainedTuple
    $activeJobsBefore = Assert-ZeroActiveJobs
    $activityBefore = Get-ActivityCount
    Wait-ForHttp200 -Url $ReadyUrl -TimeoutSeconds 180 | Out-Null
    $indexBefore = Get-LiveUIIndexHash
    $loadedModelsBefore = @(Assert-NoActiveDModelSelection)

    Write-Marker "API_CONTAINER_BEFORE=$apiBefore"
    Write-Marker "API_IMAGE_BEFORE=$apiImageBefore"
    Write-Marker "API_RESTART_COUNT_BEFORE=$apiRestartBefore"
    Write-Marker "ANALYST_INDEX_BEFORE_SHA256=$indexBefore"
    Write-Marker "RETAINED_TUPLE_BEFORE=$tupleBefore"
    Write-Marker "ACTIVITY_COUNT_BEFORE=$activityBefore"
    Write-Marker "LOADED_MODELS_BEFORE=$($loadedModelsBefore -join ',')"

    if ($ValidateOnly) {
        if ($apiImageBefore -ne $BaseImage) {
            throw "ValidateOnly expected accepted base image $BaseImage; running image is $apiImageBefore."
        }
        Write-Marker 'CONVERSATION_UI_FINAL_PREFLIGHT=PASS'
        Write-Marker 'MUTATION_PERFORMED=false'
        Write-Marker 'UI_ACTIVATION=READY'
        return
    }

    if ($Rollback) {
        $rollbackID = docker image inspect $RollbackImage --format '{{.Id}}'
        Assert-ExitCode "Rollback image is unavailable: $RollbackImage"
        Assert-RamGate
        $runtimeChanged = $true
        Invoke-ComposeRecreate -OverridePath $RollbackOverride
        Wait-ForHttp200 -Url $ReadyUrl -TimeoutSeconds 300 | Out-Null
        $apiAfterRollback = Get-ServiceContainerID 'api'
        if ((Get-ContainerImageID $apiAfterRollback) -ne $rollbackID.Trim()) {
            throw 'Rollback container is not using the sealed rollback image.'
        }
        Assert-ProtectedSnapshot -Before $protectedBefore
        $tupleAfterRollback = Get-RetainedTuple
        if ($tupleAfterRollback -ne $tupleBefore) {
            throw 'Retained state changed during UI rollback.'
        }
        $runtimeChanged = $false
        Write-Marker 'UI_ROLLBACK=PASS'
        Write-Marker 'RETAINED_DATA_MUTATED=false'
        return
    }

    if ($apiImageBefore -ne $BaseImage) {
        throw "Activation expected accepted base image $BaseImage; running image is $apiImageBefore."
    }

    # The governed RAM gate is deliberately last: every read-only and source
    # preflight above completes before this check, and no mutation precedes it.
    $null = Assert-NoActiveDModelSelection
    $null = Assert-ZeroActiveJobs
    Assert-RamGate

    docker image tag $BaseImage $RollbackImage
    Assert-ExitCode 'Could not create the exact UI rollback tag.'
    $rollbackCreated = $true
    $rollbackID = docker image inspect $RollbackImage --format '{{.Id}}'
    Assert-ExitCode 'Could not verify the UI rollback tag.'
    if ($rollbackID.Trim() -ne $BaseImage) {
        throw 'Rollback tag does not resolve to the accepted live image.'
    }
    Write-Marker "ROLLBACK_IMAGE=$RollbackImage"

    # BuildKit resolves FROM references by name, not by a bare local image ID.
    # The preceding check proves this local tag resolves to the exact base ID.
    docker build --progress plain --build-arg "BASE_IMAGE=$RollbackImage" -t $ActivationImage $ContextDir
    Assert-ExitCode 'The sealed UI-only Docker image build failed.'
    $builtImage = docker image inspect $ActivationImage --format '{{.Id}}'
    Assert-ExitCode 'Could not inspect the built UI-only image.'
    $imageInspection = docker image inspect $ActivationImage | ConvertFrom-Json
    Assert-ExitCode 'Could not inspect the D-isolation image label.'
    if ($null -eq $imageInspection -or $imageInspection.Count -ne 1) {
        throw 'Could not parse the built UI-only image inspection.'
    }
    $dLabel = $imageInspection[0].Config.Labels.'nexusai.d-unqualified-source-included'
    if ($dLabel -ne 'false') {
        throw 'Built image does not carry the required D-isolation label.'
    }
    Write-Marker "BUILT_IMAGE_ID=$($builtImage.Trim())"
    Write-Marker 'SEALED_UI_IMAGE_BUILD=PASS'

    $runtimeChanged = $true
    Invoke-ComposeRecreate -OverridePath $ActivationOverride
    Wait-ForHttp200 -Url $ReadyUrl -TimeoutSeconds 300 | Out-Null
    Wait-ForHttp200 -Url $AnalystUrl -TimeoutSeconds 180 -Accept 'text/html' | Out-Null

    $apiAfter = Get-ServiceContainerID 'api'
    $apiImageAfter = Get-ContainerImageID $apiAfter
    $apiRestartAfter = Get-ContainerRestartCount $apiAfter
    if ($apiAfter -eq $apiBefore) {
        throw 'API container was not recreated.'
    }
    if ($apiImageAfter -ne $builtImage.Trim()) {
        throw 'The recreated API container is not using the sealed UI-only image.'
    }

    Assert-ProtectedSnapshot -Before $protectedBefore
    $tupleAfter = Get-RetainedTuple
    if ($tupleAfter -ne $tupleBefore) {
        throw "Retained state changed during UI activation. Before=$tupleBefore After=$tupleAfter"
    }
    $activeJobsAfter = Assert-ZeroActiveJobs
    $activityAfter = Get-ActivityCount
    if ($activityAfter -ne $activityBefore) {
        throw "Activity count changed during UI activation. Before=$activityBefore After=$activityAfter"
    }
    $loadedModelsAfter = @(Assert-NoActiveDModelSelection)
    $liveIndexHash = Assert-LiveUIIndex

    Write-ActivationReceipt `
        -Manifest $manifest `
        -BeforeContainer $apiBefore `
        -BeforeImage $apiImageBefore `
        -BeforeRestartCount $apiRestartBefore `
        -BeforeIndexHash $indexBefore `
        -AfterContainer $apiAfter `
        -AfterImage $apiImageAfter `
        -AfterRestartCount $apiRestartAfter `
        -BuiltImage $builtImage.Trim() `
        -BeforeTuple $tupleBefore `
        -AfterTuple $tupleAfter `
        -ProtectedBefore $protectedBefore `
        -ActivityCount $activityBefore `
        -LoadedModelsBefore $loadedModelsBefore `
        -LoadedModelsAfter $loadedModelsAfter `
        -LiveIndexHash $liveIndexHash

    $runtimeChanged = $false
    Write-Marker 'CONVERSATION_UI_FINAL=LIVE_VERIFIED'
    Write-Marker 'PRIMARY_PATH=ADD_DATA_ASK_ANSWER_EVIDENCE'
    Write-Marker 'UI_ACTIVATION=PASS'
    Write-Marker 'RETAINED_DATA_MUTATED=false'
    Write-Marker 'NX-B2.1D=OPEN'
    Write-Marker 'D_ACTIVATION=BLOCKED'
}
catch {
    Write-Host "ACTIVATION_ERROR=$($_.Exception.Message)" -ForegroundColor Red

    if ($runtimeChanged -and $rollbackCreated) {
        Write-Warning 'Activation safety check failed after recreation; restoring the exact pre-activation UI image.'
        try {
            Invoke-ComposeRecreate -OverridePath $RollbackOverride
            Wait-ForHttp200 -Url $ReadyUrl -TimeoutSeconds 300 | Out-Null
            $rollbackContainer = Get-ServiceContainerID 'api'
            if ((Get-ContainerImageID $rollbackContainer) -ne $BaseImage) {
                throw 'Automatic rollback did not restore the accepted base image.'
            }
            Assert-ProtectedSnapshot -Before $protectedBefore
            $rollbackTuple = Get-RetainedTuple
            if ($rollbackTuple -ne $tupleBefore) {
                throw 'Retained state differs after automatic UI rollback.'
            }
            Write-Marker 'UI_ACTIVATION=FAILED_ROLLED_BACK'
            Write-Marker 'RETAINED_DATA_MUTATED=false'
        }
        catch {
            Write-Host "AUTOMATIC_ROLLBACK_INCOMPLETE=$($_.Exception.Message)" -ForegroundColor Red
            Write-Marker 'UI_ACTIVATION=FAILED_ROLLBACK_INCOMPLETE'
        }
    }
    else {
        Write-Marker 'UI_ACTIVATION=NOT_STARTED'
        Write-Marker 'MUTATION_PERFORMED=false'
    }
    exit 1
}
