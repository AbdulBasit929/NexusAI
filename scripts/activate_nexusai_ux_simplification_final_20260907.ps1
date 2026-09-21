param(
    [switch]$ValidateOnly,
    [switch]$Rollback
)

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

$RepoRoot = 'C:\Users\sheik\Workspace\Office\Projects\NexusAI'
$ContextDir = Join-Path $RepoRoot 'reports\nxb21\investigation-workspace-ux-simplification-final-build-context-20260907\context'
$ManifestPath = Join-Path $RepoRoot 'reports\nxb21\investigation-workspace-ux-simplification-final-deployment-manifest-20260907.json'
$ManifestHashPath = "$ManifestPath.sha256"
$ReceiptDir = Join-Path $RepoRoot 'reports\nxb21\investigation-workspace-ux-simplification-final-activation-20260907'
$ReceiptPath = Join-Path $ReceiptDir 'activation-receipt.json'
$ReceiptHashPath = "$ReceiptPath.sha256"

$BaseCompose = Join-Path $RepoRoot 'local-acceptance-models\nxmmr\private-activation-video-result\20260903T063719719Z\activation-compose.json'
$ActivationOverride = Join-Path $ContextDir 'compose.ui-only.yaml'
$RollbackOverride = Join-Path $ContextDir 'compose.rollback.yaml'

$BaseImage = 'sha256:0e47f2d689e796cf231e08b33f23e470999d3a80000ef852d49d8c93eb05827d'
$ActivationImage = 'nexusai/localai-forensic:ux-simplification-final-20260907'
$RollbackImage = 'nexusai/localai-forensic:rollback-before-ux-simplification-final-20260907'
$RequiredFreeRamKiB = 6291456
$ReadyUrl = 'http://localhost:8080/readyz'
$AnalystUrl = 'http://localhost:8080/analyst'

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

function Get-ProtectedSnapshot {
    $snapshot = [ordered]@{}
    foreach ($service in $ProtectedServices) {
        $container = Get-ServiceContainerID $service
        $snapshot[$service] = [ordered]@{
            container_id = $container
            image_id = Get-ContainerImageID $container
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
  (SELECT count(*) FROM forensic.records_ingest_jobs
      WHERE status::text IN ('queued','running','processing')),
  (SELECT count(*) FROM forensic.records),
  (SELECT count(*) FROM forensic.derived_artifacts),
  (SELECT count(*) FROM forensic.kb_collection_assets);
"@

    $output = docker exec $postgres psql -U $pgUser -d $pgDb -At -F '|' -c $sql
    Assert-ExitCode 'Could not read the retained-state tuple.'
    $tuple = $output | Where-Object { $_ -match '^\d+\|\d+\|\d+\|\d+\|\d+\|\d+\|\d+$' } | Select-Object -Last 1
    if ([string]::IsNullOrWhiteSpace($tuple)) {
        throw 'Could not parse the retained-state tuple.'
    }
    return $tuple.Trim()
}

function Assert-ZeroActiveJobs {
    param([Parameter(Mandatory=$true)][string]$Tuple)

    $parts = $Tuple.Split('|')
    if ($parts.Count -ne 7) {
        throw "Invalid retained-state tuple: $Tuple"
    }
    Write-Marker "ACTIVE_JOBS=$($parts[3])"
    if ([int]$parts[3] -ne 0) {
        throw "UI activation is blocked while $($parts[3]) forensic job(s) are active."
    }
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

function Assert-RamGate {
    $os = Get-CimInstance Win32_OperatingSystem
    $freeGiB = [math]::Round(([double]$os.FreePhysicalMemory / 1MB), 3)
    Write-Marker "CURRENT_AVAILABLE_RAM_GIB=$freeGiB"
    Write-Marker 'RAM_REQUIRED_GIB=6'

    if ([double]$os.FreePhysicalMemory -lt $RequiredFreeRamKiB) {
        Write-Marker 'RAM_GATE=FAIL'
        Write-Host 'LARGEST_PROCESSES_READ_ONLY:'
        Get-Process |
            Sort-Object WorkingSet64 -Descending |
            Select-Object -First 12 ProcessName, Id, @{Name='WorkingSetMiB';Expression={[math]::Round($_.WorkingSet64 / 1MB, 1)}} |
            Format-Table -AutoSize
        throw 'ACTIVATION=PENDING_RAM. Close applications manually and rerun this same command.'
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

function Write-ActivationReceipt {
    param(
        [Parameter(Mandatory=$true)]$Manifest,
        [Parameter(Mandatory=$true)][string]$BeforeContainer,
        [Parameter(Mandatory=$true)][string]$BeforeImage,
        [Parameter(Mandatory=$true)][string]$AfterContainer,
        [Parameter(Mandatory=$true)][string]$AfterImage,
        [Parameter(Mandatory=$true)][string]$BuiltImage,
        [Parameter(Mandatory=$true)][string]$BeforeTuple,
        [Parameter(Mandatory=$true)][string]$AfterTuple,
        [Parameter(Mandatory=$true)]$ProtectedBefore,
        [Parameter(Mandatory=$true)][string]$LiveIndexHash
    )

    if (-not (Test-Path -LiteralPath $ReceiptDir)) {
        New-Item -ItemType Directory -Force -Path $ReceiptDir | Out-Null
    }
    $receipt = [ordered]@{
        schema_version = 'nexusai.investigation-workspace-ux-simplification-activation-receipt/v1'
        activated_at_utc = (Get-Date).ToUniversalTime().ToString('o')
        status = 'LIVE_VERIFIED'
        canonical_route = '/analyst'
        source_manifest_sha256 = (Get-FileHash -Algorithm SHA256 -LiteralPath $ManifestPath).Hash.ToLowerInvariant()
        source_digest_sha256 = $Manifest.source_digest_sha256
        base_image_id = $BaseImage
        api_container_before = $BeforeContainer
        api_image_before = $BeforeImage
        api_container_after = $AfterContainer
        api_image_after = $AfterImage
        built_image_id = $BuiltImage
        activation_image = $ActivationImage
        rollback_image = $RollbackImage
        retained_tuple_before = $BeforeTuple
        retained_tuple_after = $AfterTuple
        protected_services_before = $ProtectedBefore
        protected_services_preserved = $true
        retained_data_mutated = $false
        database_migration = $false
        models_changed = $false
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
    $protectedBefore = Get-ProtectedSnapshot
    $tupleBefore = Get-RetainedTuple
    Assert-ZeroActiveJobs $tupleBefore
    Wait-ForHttp200 -Url $ReadyUrl -TimeoutSeconds 180 | Out-Null

    Write-Marker "API_CONTAINER_BEFORE=$apiBefore"
    Write-Marker "API_IMAGE_BEFORE=$apiImageBefore"
    Write-Marker "RETAINED_TUPLE_BEFORE=$tupleBefore"

    if ($ValidateOnly) {
        if ($apiImageBefore -ne $BaseImage) {
            throw "ValidateOnly expected accepted base image $BaseImage; running image is $apiImageBefore."
        }
        Write-Marker 'UX_SIMPLIFICATION_PREFLIGHT=PASS'
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
    $liveIndexHash = Assert-LiveUIIndex

    Write-ActivationReceipt `
        -Manifest $manifest `
        -BeforeContainer $apiBefore `
        -BeforeImage $apiImageBefore `
        -AfterContainer $apiAfter `
        -AfterImage $apiImageAfter `
        -BuiltImage $builtImage.Trim() `
        -BeforeTuple $tupleBefore `
        -AfterTuple $tupleAfter `
        -ProtectedBefore $protectedBefore `
        -LiveIndexHash $liveIndexHash

    $runtimeChanged = $false
    Write-Marker 'UX_SIMPLIFICATION=LIVE_VERIFIED'
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
