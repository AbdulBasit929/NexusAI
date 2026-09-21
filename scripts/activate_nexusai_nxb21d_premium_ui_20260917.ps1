# SPDX-License-Identifier: MIT
[CmdletBinding()]
param(
    [switch]$ValidateOnly,
    [switch]$TemporarilyUnloadEmbedding,
    [switch]$ReclaimDockerFileCache,
    [ValidateRange(1, 30)]
    [int]$WaitForRamMinutes = 5
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

$RepoRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$BundleRoot = Join-Path $RepoRoot 'reports\nxb21\nxb21d-premium-ui-activation-20260917'
$ContextDir = Join-Path $BundleRoot 'build-context'
$ManifestPath = Join-Path $BundleRoot 'deployment-manifest.json'
$ManifestHashPath = "$ManifestPath.sha256"
$ReceiptPath = Join-Path $BundleRoot 'activation-receipt.json'
$ReceiptHashPath = "$ReceiptPath.sha256"
$BaseCompose = Join-Path $RepoRoot 'local-acceptance-models\nxmmr\private-activation-video-result\20260903T063719719Z\activation-compose.json'
$ActivationCompose = Join-Path $ContextDir 'compose.ui-only.yaml'
$RollbackCompose = Join-Path $ContextDir 'compose.rollback.yaml'
$UISource = Join-Path $RepoRoot 'core\http\react-ui'

$BaseImageID = 'sha256:91620e4c932764c46931dd4eecb47f9f72408e2387da3189f8fcd4ab4d7506be'
$ActivationImage = 'nexusai/localai-forensic:nxb21d-premium-ui-20260917'
$RollbackImage = 'nexusai/localai-forensic:rollback-before-nxb21d-premium-ui-20260917'
$EmbeddingModelID = 'qwen3-embedding-0.6b'
$Q8ModelID = 'qwen_qwen3-4b-instruct-2507'
$RequiredFreeRamGiB = 6.0
$APIBaseURL = 'http://127.0.0.1:8080'

$ProtectedServices = @(
    'forensic-records-api',
    'forensic-records-worker',
    'forensic-postgres',
    'forensic-nats'
)

function Write-Marker([string]$Text) {
    Write-Host $Text
}

function Invoke-Native {
    param(
        [Parameter(Mandatory=$true)][string]$File,
        [Parameter(Mandatory=$true)][string[]]$Arguments
    )

    $priorPreference = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try {
        $output = @(& $File @Arguments 2>&1)
        $exitCode = $LASTEXITCODE
    }
    finally {
        $ErrorActionPreference = $priorPreference
    }
    if ($exitCode -ne 0) {
        throw "$File failed with exit code $exitCode`: $($output -join [Environment]::NewLine)"
    }
    return ($output -join [Environment]::NewLine).Trim()
}

function Get-FileHashLower([string]$Path) {
    return (Get-FileHash -Algorithm SHA256 -LiteralPath $Path).Hash.ToLowerInvariant()
}

function Get-TreeDigest {
    param(
        [Parameter(Mandatory=$true)][string]$Root,
        [switch]$UISourceRules
    )

    $resolved = (Resolve-Path -LiteralPath $Root).Path
    $files = @(Get-ChildItem -LiteralPath $resolved -Recurse -File | Where-Object {
        if (-not $UISourceRules) { return $true }
        return $_.FullName -notmatch '\\(node_modules|dist|reports|test-results|playwright-report)\\' -and
            $_.Name -notlike '*.log'
    })
    $pathMap = @{}
    foreach ($file in $files) {
        $relative = $file.FullName.Substring($resolved.Length + 1).Replace('\', '/')
        $pathMap[$relative] = $file.FullName
    }
    [string[]]$relativePaths = @($pathMap.Keys)
    [Array]::Sort($relativePaths, [StringComparer]::Ordinal)
    $lines = @($relativePaths | ForEach-Object { "$(Get-FileHashLower $pathMap[$_])  $_" })
    $bytes = [Text.Encoding]::UTF8.GetBytes(($lines -join "`n") + "`n")
    $sha = [Security.Cryptography.SHA256]::Create()
    try {
        $digest = ([BitConverter]::ToString($sha.ComputeHash($bytes))).Replace('-', '').ToLowerInvariant()
    }
    finally {
        $sha.Dispose()
    }
    return [pscustomobject]@{ Count = $files.Count; Digest = $digest }
}

function Get-ServiceContainerID([string]$Service) {
    $raw = Invoke-Native docker @(
        'ps', '-q', '--no-trunc',
        '--filter', 'label=com.docker.compose.project=nexusai',
        '--filter', "label=com.docker.compose.service=$Service"
    )
    $ids = @($raw -split "`r?`n" | Where-Object { -not [string]::IsNullOrWhiteSpace($_) })
    if ($ids.Count -ne 1) {
        throw "Expected exactly one running '$Service' container; found $($ids.Count)."
    }
    return $ids[0].Trim()
}

function Get-ContainerInspect([string]$ContainerID) {
    $value = Invoke-Native docker @('inspect', $ContainerID) | ConvertFrom-Json
    if ($null -eq $value -or @($value).Count -ne 1) {
        throw "Could not parse Docker inspection for $ContainerID."
    }
    return @($value)[0]
}

function Get-ServiceSnapshot([string[]]$Services) {
    $snapshot = [ordered]@{}
    foreach ($service in $Services) {
        $id = Get-ServiceContainerID $service
        $inspect = Get-ContainerInspect $id
        if (-not [bool]$inspect.State.Running -or [bool]$inspect.State.OOMKilled) {
            throw "Service '$service' is not in a safe running state."
        }
        $healthProperty = $inspect.State.PSObject.Properties['Health']
        if ($null -ne $healthProperty -and [string]$healthProperty.Value.Status -cne 'healthy') {
            throw "Service '$service' health is '$($healthProperty.Value.Status)', not healthy."
        }
        $snapshot[$service] = [ordered]@{
            container_id = [string]$inspect.Id
            image_id = [string]$inspect.Image
            restart_count = [int]$inspect.RestartCount
            health = if ($null -ne $healthProperty) { [string]$healthProperty.Value.Status } else { 'not_configured' }
        }
    }
    return $snapshot
}

function Assert-ServiceSnapshot($Before) {
    foreach ($service in $ProtectedServices) {
        $id = Get-ServiceContainerID $service
        $inspect = Get-ContainerInspect $id
        if ([string]$inspect.Id -cne [string]$Before[$service].container_id -or
            [string]$inspect.Image -cne [string]$Before[$service].image_id -or
            [int]$inspect.RestartCount -ne [int]$Before[$service].restart_count) {
            throw "Protected service '$service' changed during UI activation."
        }
        if (-not [bool]$inspect.State.Running -or [bool]$inspect.State.OOMKilled) {
            throw "Protected service '$service' is not healthy after UI activation."
        }
        $healthProperty = $inspect.State.PSObject.Properties['Health']
        if ($null -ne $healthProperty -and [string]$healthProperty.Value.Status -cne 'healthy') {
            throw "Protected service '$service' health is '$($healthProperty.Value.Status)' after UI activation."
        }
    }
    Write-Marker 'PROTECTED_SERVICES_PRESERVED=true'
}

function Get-ContainerEnvValue($Inspect, [string]$Name) {
    $entry = @($Inspect.Config.Env | Where-Object { $_ -like "$Name=*" } | Select-Object -First 1)
    if ($entry.Count -ne 1) {
        throw "Container environment variable '$Name' is unavailable."
    }
    return ([string]$entry[0]).Substring($Name.Length + 1)
}

function Get-RetainedState {
    $postgresID = Get-ServiceContainerID 'forensic-postgres'
    $inspect = Get-ContainerInspect $postgresID
    $user = Get-ContainerEnvValue $inspect 'POSTGRES_USER'
    $database = Get-ContainerEnvValue $inspect 'POSTGRES_DB'
    $sql = "SELECT (SELECT count(*) FROM forensic.evidence_items),(SELECT count(*) FROM forensic.evidence_versions),(SELECT count(*) FROM forensic.records_ingest_jobs),(SELECT count(*) FROM forensic.records_ingest_jobs WHERE status NOT IN ('completed','dead_letter')),(SELECT count(*) FROM forensic.records),(SELECT count(*) FROM forensic.derived_artifacts),(SELECT count(*) FROM forensic.kb_collection_assets);"
    $raw = Invoke-Native docker @('exec', $postgresID, 'psql', '-U', $user, '-d', $database, '-At', '-F', '|', '-c', $sql)
    $tuple = @($raw -split "`r?`n" | Where-Object { $_ -match '^\d+\|\d+\|\d+\|\d+\|\d+\|\d+\|\d+$' } | Select-Object -Last 1)
    if ($tuple.Count -ne 1) { throw 'Could not parse the retained-state tuple.' }

    $activityRaw = Invoke-Native docker @('exec', $postgresID, 'psql', '-U', $user, '-d', $database, '-At', '-c', 'SELECT count(*) FROM public.agent_analysis_history;')
    $activity = @($activityRaw -split "`r?`n" | Where-Object { $_ -match '^\d+$' } | Select-Object -Last 1)
    if ($activity.Count -ne 1) { throw 'Could not parse the Activity count.' }

    $parts = $tuple[0].Split('|')
    return [pscustomobject]@{
        tuple = $tuple[0]
        active_jobs = [int]$parts[3]
        activity_count = [int]$activity[0]
    }
}

function Get-LoadedModels {
    $system = Invoke-RestMethod -Method Get -Uri "$APIBaseURL/system" -TimeoutSec 20
    return @($system.loaded_models | ForEach-Object { [string]$_.id } | Where-Object { $_ } | Sort-Object -Unique)
}

function Wait-ForHTTP200([string]$URL, [int]$TimeoutSeconds, [string]$Accept = '*/*') {
    $deadline = [DateTime]::UtcNow.AddSeconds($TimeoutSeconds)
    do {
        try {
            $response = Invoke-WebRequest -UseBasicParsing -Uri $URL -Headers @{Accept=$Accept} -TimeoutSec 15
            if ([int]$response.StatusCode -eq 200) { return }
        }
        catch {
            # The API may still be starting after the bounded recreate.
        }
        Start-Sleep -Seconds 3
    } while ([DateTime]::UtcNow -lt $deadline)
    throw "Timed out waiting for HTTP 200 from $URL."
}

function Get-RemoteHash([string]$URL) {
    $client = New-Object Net.WebClient
    try {
        $client.Headers['Accept'] = 'text/html'
        $bytes = $client.DownloadData($URL)
    }
    finally {
        $client.Dispose()
    }
    $sha = [Security.Cryptography.SHA256]::Create()
    try {
        return ([BitConverter]::ToString($sha.ComputeHash($bytes))).Replace('-', '').ToLowerInvariant()
    }
    finally {
        $sha.Dispose()
    }
}

function Get-FreeRAMGiB {
    $os = Get-CimInstance Win32_OperatingSystem
    return [math]::Round(([int64]$os.FreePhysicalMemory * 1024) / 1GB, 3)
}

function Wait-StableRAM {
    $deadline = [DateTime]::UtcNow.AddMinutes($WaitForRamMinutes)
    $window = @()
    $sample = 0
    do {
        $sample++
        $value = Get-FreeRAMGiB
        Write-Marker "RAM_SAMPLE stage=ui_activation index=$sample available_gib=$value required_gib=$RequiredFreeRamGiB"
        if ($value -ge $RequiredFreeRamGiB) {
            $window += $value
            if ($window.Count -gt 3) { $window = @($window[($window.Count - 3)..($window.Count - 1)]) }
        }
        else {
            $window = @()
        }
        if ($window.Count -eq 3) {
            Write-Marker "RAM_GATE=PASS samples=$($window -join ',')"
            return @($window)
        }
        Start-Sleep -Seconds 5
    } while ([DateTime]::UtcNow -lt $deadline)
    throw "RAM_GATE_FAILED: three consecutive samples did not reach $RequiredFreeRamGiB GiB."
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
    Write-Marker "CLEAN_FILE_CACHE_RECOVERY=PASS dirty_kib=$dirtyKiB writeback_kib=0 dirty_pages_preserved=true sync=false"
}

function Stop-EmbeddingModel {
    $body = @{ model = $EmbeddingModelID } | ConvertTo-Json -Compress
    Invoke-RestMethod -Method Post -Uri "$APIBaseURL/backend/shutdown" -ContentType 'application/json' -Body $body -TimeoutSec 30 | Out-Null
    $deadline = [DateTime]::UtcNow.AddSeconds(30)
    do {
        Start-Sleep -Seconds 1
        $loaded = @(Get-LoadedModels)
    } while ($loaded -contains $EmbeddingModelID -and [DateTime]::UtcNow -lt $deadline)
    if ($loaded -contains $EmbeddingModelID) { throw 'EMBEDDING_MODEL_UNLOAD_TIMEOUT' }
    Write-Marker "EMBEDDING_MEMORY_RECLAIM=PASS model=$EmbeddingModelID"
}

function Restore-EmbeddingModel {
    $body = @{ model = $EmbeddingModelID; input = @('NexusAI UI activation runtime restoration probe') } | ConvertTo-Json -Compress
    Invoke-RestMethod -Method Post -Uri "$APIBaseURL/v1/embeddings" -ContentType 'application/json' -Body $body -TimeoutSec 180 | Out-Null
    $loaded = @(Get-LoadedModels)
    if ($loaded -notcontains $EmbeddingModelID) { throw 'EMBEDDING_RUNTIME_RESTORE_FAILED' }
    Write-Marker "EMBEDDING_RUNTIME_RESTORE=PASS model=$EmbeddingModelID"
}

function Invoke-APIRecreate([string]$Overlay) {
    Invoke-Native docker @(
        'compose', '-p', 'nexusai', '-f', $BaseCompose, '-f', $Overlay,
        'up', '-d', '--no-deps', '--force-recreate', '--no-build', '--pull', 'never', 'api'
    ) | ForEach-Object { if ($_ ) { Write-Host $_ } }
}

function Assert-Bundle {
    foreach ($path in @($ManifestPath, $ManifestHashPath, $BaseCompose, $ActivationCompose, $RollbackCompose, (Join-Path $ContextDir 'Dockerfile'), (Join-Path $ContextDir 'nexusai-ui-wrapper'), (Join-Path $ContextDir 'ui-dist\index.html'))) {
        if (-not (Test-Path -LiteralPath $path -PathType Leaf)) { throw "Required activation file is missing: $path" }
    }

    $expectedManifestHash = ((Get-Content -LiteralPath $ManifestHashPath -Raw).Trim() -split '\s+')[0].ToLowerInvariant()
    if ((Get-FileHashLower $ManifestPath) -cne $expectedManifestHash) { throw 'DEPLOYMENT_MANIFEST_HASH_MISMATCH' }
    $manifest = Get-Content -LiteralPath $ManifestPath -Raw | ConvertFrom-Json
    if ([string]$manifest.base_image_id -cne $BaseImageID) { throw 'DEPLOYMENT_BASE_IMAGE_MISMATCH' }

    $source = Get-TreeDigest -Root $UISource -UISourceRules
    if ($source.Count -ne [int]$manifest.ui_source_file_count -or $source.Digest -cne [string]$manifest.ui_source_digest_sha256) {
        throw "UI_SOURCE_DIGEST_MISMATCH:actual_count=$($source.Count):expected_count=$($manifest.ui_source_file_count):actual_digest=$($source.Digest):expected_digest=$($manifest.ui_source_digest_sha256)"
    }
    $dist = Get-TreeDigest -Root (Join-Path $ContextDir 'ui-dist')
    if ($dist.Count -ne [int]$manifest.ui_dist_file_count -or $dist.Digest -cne [string]$manifest.ui_dist_digest_sha256) {
        throw "UI_DIST_DIGEST_MISMATCH:actual_count=$($dist.Count):expected_count=$($manifest.ui_dist_file_count):actual_digest=$($dist.Digest):expected_digest=$($manifest.ui_dist_digest_sha256)"
    }

    $checks = [ordered]@{
        (Join-Path $ContextDir 'ui-dist\index.html') = [string]$manifest.built_index_sha256
        (Join-Path $ContextDir 'nexusai-ui-wrapper') = [string]$manifest.wrapper_sha256
        (Join-Path $ContextDir 'Dockerfile') = [string]$manifest.dockerfile_sha256
        $ActivationCompose = [string]$manifest.activation_compose_sha256
        $RollbackCompose = [string]$manifest.rollback_compose_sha256
    }
    foreach ($entry in $checks.GetEnumerator()) {
        if ((Get-FileHashLower $entry.Key) -cne $entry.Value) { throw "ACTIVATION_FILE_HASH_MISMATCH:$($entry.Key)" }
    }

    Invoke-Native docker @('compose', '-p', 'nexusai', '-f', $BaseCompose, '-f', $ActivationCompose, 'config', '--quiet') | Out-Null
    Invoke-Native docker @('compose', '-p', 'nexusai', '-f', $BaseCompose, '-f', $RollbackCompose, 'config', '--quiet') | Out-Null
    Write-Marker 'UI_ACTIVATION_BUNDLE=PASS'
    return $manifest
}

Set-Location $RepoRoot
$runtimeChanged = $false
$rollbackReady = $false
$embeddingWasUnloaded = $false
$protectedBefore = $null
$retainedBefore = $null
$apiBeforeID = $null

try {
    $serverVersion = Invoke-Native docker @('info', '--format', '{{.ServerVersion}}')
    Write-Marker "DOCKER_SERVER_VERSION=$serverVersion"
    $manifest = Assert-Bundle

    $allServices = @('api') + $ProtectedServices
    $allBefore = Get-ServiceSnapshot $allServices
    $protectedBefore = [ordered]@{}
    foreach ($service in $ProtectedServices) { $protectedBefore[$service] = $allBefore[$service] }
    $apiBeforeID = [string]$allBefore.api.container_id
    if ([string]$allBefore.api.image_id -cne $BaseImageID) {
        throw "LIVE_BASE_IMAGE_MISMATCH: expected=$BaseImageID actual=$($allBefore.api.image_id)"
    }

    $retainedBefore = Get-RetainedState
    if ($retainedBefore.active_jobs -ne 0) { throw "ACTIVE_JOBS_BLOCK_ACTIVATION:$($retainedBefore.active_jobs)" }
    Wait-ForHTTP200 "$APIBaseURL/readyz" 60
    Wait-ForHTTP200 "$APIBaseURL/api/v1/forensics/cases" 60
    $loadedBefore = @(Get-LoadedModels)
    if ($loadedBefore -contains $Q8ModelID) { throw 'Q8_MODEL_MUST_START_UNLOADED' }

    Write-Marker "API_CONTAINER_BEFORE=$apiBeforeID"
    Write-Marker "API_IMAGE_BEFORE=$($allBefore.api.image_id)"
    Write-Marker "RETAINED_TUPLE_BEFORE=$($retainedBefore.tuple)"
    Write-Marker "ACTIVITY_COUNT_BEFORE=$($retainedBefore.activity_count)"
    Write-Marker "ACTIVE_JOBS_BEFORE=$($retainedBefore.active_jobs)"
    Write-Marker "LOADED_MODELS_BEFORE=$($loadedBefore -join ',')"
    Write-Marker "RAM_AVAILABLE_GIB=$(Get-FreeRAMGiB)"

    if ($ValidateOnly) {
        Write-Marker 'NXB21D_PREMIUM_UI_PREFLIGHT=PASS'
        Write-Marker 'RAM_GATE=DEFERRED_TO_LIVE_RUN'
        Write-Marker 'MUTATION_PERFORMED=false'
        exit 0
    }

    if (Test-Path -LiteralPath $ReceiptPath) { throw "ACTIVATION_RECEIPT_ALREADY_EXISTS:$ReceiptPath" }

    if ($TemporarilyUnloadEmbedding -and $loadedBefore -contains $EmbeddingModelID) {
        Stop-EmbeddingModel
        $embeddingWasUnloaded = $true
        Start-Sleep -Seconds 20
    }
    if ($ReclaimDockerFileCache) {
        Invoke-GovernedCleanCacheReclaim
        Start-Sleep -Seconds 5
    }

    $ramSamples = @(Wait-StableRAM)

    # Recheck every mutable precondition immediately before the first mutation.
    $apiCurrent = Get-ServiceSnapshot @('api')
    if ([string]$apiCurrent.api.container_id -cne $apiBeforeID -or [string]$apiCurrent.api.image_id -cne $BaseImageID) {
        throw 'API_STATE_CHANGED_DURING_PREFLIGHT'
    }
    Assert-ServiceSnapshot $protectedBefore
    $retainedCurrent = Get-RetainedState
    if ($retainedCurrent.tuple -cne $retainedBefore.tuple -or $retainedCurrent.activity_count -ne $retainedBefore.activity_count -or $retainedCurrent.active_jobs -ne 0) {
        throw 'RETAINED_STATE_CHANGED_DURING_PREFLIGHT'
    }

    Invoke-Native docker @('image', 'tag', $BaseImageID, $RollbackImage) | Out-Null
    $rollbackID = Invoke-Native docker @('image', 'inspect', '--format', '{{.Id}}', $RollbackImage)
    if ($rollbackID.Trim() -cne $BaseImageID) { throw 'ROLLBACK_TAG_ID_MISMATCH' }
    $rollbackReady = $true
    Write-Marker "ROLLBACK_IMAGE=$RollbackImage"

    Invoke-Native docker @('build', '--network', 'none', '--progress', 'plain', '--build-arg', "BASE_IMAGE=$RollbackImage", '-t', $ActivationImage, $ContextDir) | ForEach-Object { if ($_ ) { Write-Host $_ } }
    $builtID = (Invoke-Native docker @('image', 'inspect', '--format', '{{.Id}}', $ActivationImage)).Trim()
    $builtInspect = Get-ContainerInspect $ActivationImage
    if ([string]$builtInspect.Config.Labels.'nexusai.ui-source-digest' -cne [string]$manifest.ui_source_digest_sha256 -or
        [string]$builtInspect.Config.Labels.'nexusai.base-image' -cne $BaseImageID -or
        [string]$builtInspect.Config.Labels.'nexusai.d-unqualified-source-included' -cne 'false') {
        throw 'BUILT_IMAGE_LABEL_MISMATCH'
    }
    Write-Marker "BUILT_IMAGE_ID=$builtID"
    Write-Marker 'UI_ONLY_IMAGE_BUILD=PASS'

    $runtimeChanged = $true
    Invoke-APIRecreate $ActivationCompose
    Wait-ForHTTP200 "$APIBaseURL/readyz" 300
    Wait-ForHTTP200 "$APIBaseURL/analyst" 180 'text/html'
    Wait-ForHTTP200 "$APIBaseURL/api/v1/forensics/cases" 60

    $apiAfterID = Get-ServiceContainerID 'api'
    $apiAfterInspect = Get-ContainerInspect $apiAfterID
    if ($apiAfterID -ceq $apiBeforeID) { throw 'API_CONTAINER_WAS_NOT_RECREATED' }
    if ([string]$apiAfterInspect.Image -cne $builtID) { throw 'ACTIVATED_IMAGE_ID_MISMATCH' }
    Assert-ServiceSnapshot $protectedBefore

    $retainedAfter = Get-RetainedState
    if ($retainedAfter.tuple -cne $retainedBefore.tuple -or
        $retainedAfter.activity_count -ne $retainedBefore.activity_count -or
        $retainedAfter.active_jobs -ne 0) {
        throw 'RETAINED_STATE_CHANGED_DURING_ACTIVATION'
    }

    $expectedIndex = [string]$manifest.built_index_sha256
    $analystHash = Get-RemoteHash "$APIBaseURL/analyst"
    $rootHash = Get-RemoteHash "$APIBaseURL/"
    if ($analystHash -cne $expectedIndex -or $rootHash -cne $expectedIndex) {
        throw "LIVE_UI_INDEX_HASH_MISMATCH: expected=$expectedIndex analyst=$analystHash root=$rootHash"
    }

    if ($embeddingWasUnloaded) {
        Restore-EmbeddingModel
        $embeddingWasUnloaded = $false
    }
    $loadedAfter = @(Get-LoadedModels)
    if ($loadedAfter -contains $Q8ModelID) { throw 'Q8_MODEL_LOADED_UNEXPECTEDLY' }

    $receipt = [ordered]@{
        contract_version = 'nexusai.nxb21d-premium-ui-activation-receipt/v1'
        activated_at_utc = [DateTime]::UtcNow.ToString('o')
        status = 'LIVE_VERIFIED'
        route = '/analyst'
        manifest_sha256 = Get-FileHashLower $ManifestPath
        ui_source_digest_sha256 = [string]$manifest.ui_source_digest_sha256
        ui_dist_digest_sha256 = [string]$manifest.ui_dist_digest_sha256
        live_index_sha256 = $analystHash
        api_container_before = $apiBeforeID
        api_image_before = $BaseImageID
        api_container_after = $apiAfterID
        api_image_after = [string]$apiAfterInspect.Image
        activation_image = $ActivationImage
        rollback_image = $RollbackImage
        ram_gate_samples_gib = @($ramSamples)
        retained_tuple_before = $retainedBefore.tuple
        retained_tuple_after = $retainedAfter.tuple
        activity_count_before = $retainedBefore.activity_count
        activity_count_after = $retainedAfter.activity_count
        active_jobs_before = $retainedBefore.active_jobs
        active_jobs_after = $retainedAfter.active_jobs
        protected_services_preserved = $true
        loaded_models_before = @($loadedBefore)
        loaded_models_after = @($loadedAfter)
        embedding_runtime_restored = ($loadedBefore -notcontains $EmbeddingModelID -or $loadedAfter -contains $EmbeddingModelID)
        q8_model_remained_unloaded = $true
        retained_data_mutated = $false
        database_migration = $false
        backend_source_activated = $false
        model_or_profile_changed = $false
    }
    $receipt | ConvertTo-Json -Depth 12 | Set-Content -LiteralPath $ReceiptPath -Encoding UTF8
    $receiptHash = Get-FileHashLower $ReceiptPath
    "$receiptHash  activation-receipt.json" | Set-Content -LiteralPath $ReceiptHashPath -Encoding ASCII

    $runtimeChanged = $false
    Write-Marker "ACTIVATION_RECEIPT=$ReceiptPath"
    Write-Marker "ACTIVATION_RECEIPT_SHA256=$receiptHash"
    Write-Marker 'NXB21D_PREMIUM_UI=LIVE_VERIFIED'
    Write-Marker 'UI_ACTIVATION=PASS'
    Write-Marker 'RETAINED_DATA_MUTATED=false'
}
catch {
    $activationError = $_.Exception.Message
    Write-Host "ACTIVATION_ERROR=$activationError" -ForegroundColor Red
    if ($runtimeChanged -and $rollbackReady) {
        try {
            Write-Warning 'Restoring the exact pre-activation API image.'
            Invoke-APIRecreate $RollbackCompose
            Wait-ForHTTP200 "$APIBaseURL/readyz" 300
            $rolledBackID = Get-ServiceContainerID 'api'
            $rolledBackInspect = Get-ContainerInspect $rolledBackID
            if ([string]$rolledBackInspect.Image -cne $BaseImageID) { throw 'ROLLBACK_IMAGE_ID_MISMATCH' }
            if ($null -ne $protectedBefore) { Assert-ServiceSnapshot $protectedBefore }
            if ($null -ne $retainedBefore) {
                $rollbackRetained = Get-RetainedState
                if ($rollbackRetained.tuple -cne $retainedBefore.tuple -or $rollbackRetained.activity_count -ne $retainedBefore.activity_count) {
                    throw 'RETAINED_STATE_CHANGED_AFTER_ROLLBACK'
                }
            }
            $runtimeChanged = $false
            Write-Marker 'UI_ACTIVATION=FAILED_ROLLED_BACK'
        }
        catch {
            Write-Host "AUTOMATIC_ROLLBACK_INCOMPLETE=$($_.Exception.Message)" -ForegroundColor Red
            Write-Marker 'UI_ACTIVATION=FAILED_ROLLBACK_INCOMPLETE'
        }
    }
    else {
        Write-Marker 'UI_ACTIVATION=NOT_STARTED'
        Write-Marker 'SERVICE_RECREATION_PERFORMED=false'
        Write-Marker 'RETAINED_DATA_MUTATED=false'
    }

    if ($embeddingWasUnloaded) {
        try {
            Restore-EmbeddingModel
            $embeddingWasUnloaded = $false
        }
        catch {
            Write-Warning "EMBEDDING_RUNTIME_RESTORE=FAIL error=$($_.Exception.Message)"
        }
    }
    exit 1
}
