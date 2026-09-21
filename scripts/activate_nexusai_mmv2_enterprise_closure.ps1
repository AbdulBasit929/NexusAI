[CmdletBinding()]
param(
    [switch]$PreflightOnly
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

$RepositoryRoot = 'C:\Users\sheik\Workspace\Office\Projects\NexusAI'
$OutputPath = 'reports\mmv2-anpr-image-video-20260824\mmv2-enterprise-closure-local-activation-output.txt'
$ExpectedBranch = 'codex/forensic-hybrid-checkpoint-20260723'
$ExpectedHead = '40717b83510c08db25dc26b9d6674bf46db363ac'
$RequiredFreeGiB = 6.0
$PreferredFreeGiB = 6.5
$ComposeProject = 'nexusai'
$RuntimeEnvFile = '.env.forensic-runtime.local'
$CheckpointPath = 'reports\mmv2-anpr-image-video-20260824\final-p1-live-certification-20260825.json'
$LocalAIImage = 'nexusai/localai-forensic:phase3-runtime'
$ForensicApiImage = 'nexusai/forensic-records-api:phase3-runtime'
$WorkerImage = 'nexusai/forensic-records-worker:phase3-runtime'
$LocalAIRollbackTag = 'nexusai/localai-forensic:rollback-before-mmv2-enterprise-closure-20260825'
$ForensicApiRollbackTag = 'nexusai/forensic-records-api:rollback-before-mmv2-enterprise-closure-20260825'
$ExpectedLocalAIRunningImageId = 'sha256:13b37779c63c9a453115c8d0e4b13a12e68258b285b5b588a4ac2d118b7f2d51'
$ExpectedForensicApiRunningImageId = 'sha256:abce6567f187af6700784fe5b558115c50f361c4a662f3ce2f258a42ada1e6d5'
$ExpectedWorkerRunningImageId = 'sha256:be13dbec4bd9bb62c824140df3777b898d7ad2801a0e68cd512c00dafaf4f9ef'
$MediaModelDirectory = 'C:\Users\sheik\.cache'

$SidecarComposeFiles = @(
    'docker-compose.forensic-records.yaml',
    'docker-compose.forensic-records.runtime.yaml',
    'docker-compose.forensic-records.asr-small.yaml'
)
$LocalAIComposeFiles = @(
    'docker-compose.yaml',
    'docker-compose.forensic-runtime.localai.yaml'
)
$ServiceNames = [ordered]@{
    localai = 'api'
    forensic_api = 'forensic-records-api'
    worker = 'forensic-records-worker'
    nats = 'forensic-nats'
    postgres = 'forensic-postgres'
}
$ExpectedContainerIds = [ordered]@{
    localai = '0898847fda2874c71721afbda5bdaa969119b87e1ac01cc09c4abe7d132bb987'
    forensic_api = '38a5040f82f389f4fd7f248ac18c6470a1fd0dc15529dc1c7b5982cd6ae321c3'
    worker = '1846f558941f583467976ad8e74a44e9063c8a1a50e870951c7e479ecce3fb66'
    nats = '5c49e70d132ac01b4adf86f27b1735d4d5cf6cc205d848c0881164f1f200eb04'
    postgres = 'f66e05a3b17978c9912aa911177f22e57d69c17de87881a33f23bcba228ad361'
}
$ExpectedRetainedCounts = [ordered]@{
    evidence = 51L
    versions = 51L
    jobs = 64L
    active_jobs = 0L
    canonical_records = 22207L
    artifacts = 441L
    kb_assets = 47L
}
$RequiredVolumes = @(
    'nexusai_models',
    'nexusai_backends',
    'nexusai_configuration',
    'nexusai_data',
    'nexusai_images',
    'nexusai_forensic_spool',
    'nexusai_forensic_postgres_data',
    'nexusai_forensic_nats_data',
    'nexusai_r8_paddlex_model_cache'
)
$ExpectedHashes = [ordered]@{
    'api\forensic_records\query.go' = 'e1a7ce5dd83f33d308091ccdb7900034c1dd5fac4b733e9fa40614871dbda08c'
    'api\forensic_records\video_anpr.go' = '05074b5b6350ff2fe27df2981eac05e7344345b688c0e453975605506a2b25da'
    'api\forensic_records\cases.go' = '192034755191482ffd9645e365edfe504a0e709ae556e11fb30a02cad4229fbf'
    'api\forensic_records\platform_contracts.go' = '83fbb42c68c492fb134734526068d811a1e4b4122afdff5d2657c4a323e06c49'
    'api\forensic_records\stim_fact_packet.go' = '2cc7db288465fd83b5b7761f34e5c08652e7d6f63a0b2c802a25f56c9906002a'
    'api\forensic_records\mmv2_query_test.go' = 'fc4d61ba9c4e9175b65ce528444a318e4d6eee9a4dd20679e88741f4d294e4c7'
    'api\forensic_records\Dockerfile' = '3a54a1fa363db2ca14e00fb03cdb42b80c1bbdb493e53191c9b185c98fa69a1d'
    'core\services\agents\forensic_presentation.go' = 'ca4778904c1418f9ad3cdebaf78b2ce09210931e0fb34fdd46f221e7b400fc64'
    'core\services\agents\forensic_presentation_test.go' = '7785149faebc75ed0e4037645ff890c80531352ca2efefc4ebb62c16b27f77af'
    'core\http\react-ui\src\pages\AgentChat.jsx' = '0ce908036028f59f917f8d80e7b2fbe5cb3df7f9ea16efc3c3218235c9ed3bbe'
    'core\http\react-ui\src\analyst\AnalystHistory.jsx' = 'aafe288016d38e9dd69e6f066d6113bd2d79262a962b44b6b78a40a95cce6a8c'
    'Dockerfile' = '167f93165728a31b554751c2a100120eca34ffdcc8509ecb3ac5e393efe48a94'
    'docker-compose.forensic-records.yaml' = 'b0f724ea34bea80bfadf8491d03bf2ddc9a8cfcbc0f2982d1a5374b968e75dd6'
    'docker-compose.forensic-records.runtime.yaml' = '8793dff4a5980dc0d8dd6a453967cb6512daf13301f22039c770ab9d4410d923'
    'docker-compose.forensic-records.asr-small.yaml' = '4f39dfe6d572d1ffcddbe81339389167534f4b0f8b20e1d5915b0532827c11f0'
    'docker-compose.yaml' = '0ef74ce958237dbe66552dcb3e36a02def568eb1a179bcd04828cf54af4dc72d'
    'docker-compose.forensic-runtime.localai.yaml' = 'a4a10aca68a9291e9696d86f392a33e53df64992a31cca896c7c99492ce5ec5b'
    '.env' = 'e6e974b1c197e56722c03fd7092acaeb1ef9a98e37aff8197bb88904007c024d'
    '.env.forensic-runtime.local' = '82bb4660ef9615f821f3ef3157652054d6b338fa5324cb894f53ea6e9e5de726'
    'reports\mmv2-anpr-image-video-20260824\final-p1-live-certification-20260825.json' = 'fdba8acd87c626616b3ceaa6ace7222495d789c7602fd3d80f68611d77c8b0aa'
}

$script:BeforeIds = $null
$script:BeforeImages = $null
$script:BeforeCounts = $null
$script:BeforeMounts = $null
$script:BeforeVolumes = $null
$script:PgUser = $null
$script:PgDatabase = $null
$script:HistoryDatabaseUrl = $null
$script:RecreationStarted = $false
$script:TranscriptStarted = $false
$script:ExitCode = 0

function Write-Section([string]$Title) {
    Write-Host ''
    Write-Host "=== $Title ===" -ForegroundColor Cyan
}

function Invoke-External {
    param([string]$FilePath, [string[]]$ArgumentList, [string]$Description)
    $previousPreference = $ErrorActionPreference
    $exitCode = 0
    try {
        $ErrorActionPreference = 'Continue'
        & $FilePath @ArgumentList 2>&1 | ForEach-Object { Write-Host ([string]$_) }
        $exitCode = $LASTEXITCODE
    }
    finally { $ErrorActionPreference = $previousPreference }
    if ($exitCode -ne 0) { throw "$Description failed with exit code $exitCode." }
}

function Invoke-ExternalText {
    param([string]$FilePath, [string[]]$ArgumentList, [string]$Description)
    $previousPreference = $ErrorActionPreference
    $exitCode = 0
    $output = @()
    try {
        $ErrorActionPreference = 'Continue'
        $output = & $FilePath @ArgumentList 2>&1
        $exitCode = $LASTEXITCODE
    }
    finally { $ErrorActionPreference = $previousPreference }
    if ($exitCode -ne 0) {
        throw "$Description failed with exit code $exitCode.`n$($output -join [Environment]::NewLine)"
    }
    return ($output -join "`n").Trim()
}

function Invoke-DisplayOnly {
    param([string]$FilePath, [string[]]$ArgumentList, [string]$Description)
    try {
        $previousPreference = $ErrorActionPreference
        $ErrorActionPreference = 'Continue'
        & $FilePath @ArgumentList 2>&1 | ForEach-Object { Write-Host ([string]$_) }
        $displayExit = $LASTEXITCODE
        $ErrorActionPreference = $previousPreference
        if ($displayExit -ne 0) { Write-Warning "$Description returned exit code $displayExit; substantive verification already passed." }
    }
    catch { Write-Warning "$Description could not be displayed: $($_.Exception.Message)" }
}

function Get-FreePhysicalMemoryGiB {
    $os = Get-CimInstance -ClassName Win32_OperatingSystem
    return [math]::Round(([double]$os.FreePhysicalMemory / 1MB), 2)
}

function Assert-FreePhysicalMemory {
    $freeGiB = Get-FreePhysicalMemoryGiB
    Write-Host ("FreePhysicalMemoryGiB={0:N2}" -f $freeGiB)
    Write-Host ("RequiredFreePhysicalMemoryGiB={0:N2}" -f $RequiredFreeGiB)
    Write-Host ("PreferredLaunchFreePhysicalMemoryGiB={0:N2}" -f $PreferredFreeGiB)
    if ($freeGiB -lt $RequiredFreeGiB) {
        throw ("MMV2EnterpriseClosureActivation=SAFE_STOP_RAM_GATE Free physical RAM {0:N2} GiB is below mandatory {1:N2} GiB; no Docker mutation was performed." -f $freeGiB, $RequiredFreeGiB)
    }
    if ($freeGiB -lt $PreferredFreeGiB) {
        Write-Warning ("Mandatory RAM gate passed, but free RAM {0:N2} GiB is below the preferred {1:N2} GiB launch margin." -f $freeGiB, $PreferredFreeGiB)
    } else {
        Write-Host 'PreferredLaunchRam=PASS'
    }
}

function Get-ComposeContainerId([string]$Service) {
    $text = Invoke-ExternalText 'docker' @('ps', '-q', '--no-trunc', '--filter', "label=com.docker.compose.project=$ComposeProject", '--filter', "label=com.docker.compose.service=$Service") "Resolve running container for $Service"
    $ids = @($text -split "`r?`n" | Where-Object { $_.Trim() })
    if ($ids.Count -ne 1) { throw "Expected one running container for Compose service '$Service'; found $($ids.Count)." }
    return $ids[0].Trim()
}

function Get-AllServiceIds {
    $result = [ordered]@{}
    foreach ($key in $ServiceNames.Keys) { $result[$key] = Get-ComposeContainerId $ServiceNames[$key] }
    return $result
}

function Get-ContainerImageId([string]$ContainerId) {
    return Invoke-ExternalText 'docker' @('inspect', $ContainerId, '--format', '{{.Image}}') "Inspect image for $ContainerId"
}

function Get-AllServiceImages([System.Collections.IDictionary]$Ids) {
    $result = [ordered]@{}
    foreach ($key in $Ids.Keys) { $result[$key] = Get-ContainerImageId $Ids[$key] }
    return $result
}

function Get-ContainerEnvironmentValue([string]$ContainerId, [string]$Name) {
    $text = Invoke-ExternalText 'docker' @('inspect', $ContainerId, '--format', '{{range .Config.Env}}{{println .}}{{end}}') "Inspect environment for $ContainerId"
    $prefix = "$Name="
    $matches = @($text -split "`r?`n" | Where-Object { $_.StartsWith($prefix, [System.StringComparison]::Ordinal) })
    if ($matches.Count -ne 1) { throw "Expected one '$Name' environment entry in container $ContainerId." }
    return $matches[0].Substring($prefix.Length)
}

function ConvertTo-ComparableMountSource([string]$Type, [string]$Source) {
    if ($Type -eq 'volume') { return '' }
    $normalized = $Source -replace '\\', '/'
    if ($normalized -match '^/run/desktop/mnt/host/([A-Za-z])/(.*)$') { $normalized = "$($Matches[1]):/$($Matches[2])" }
    elseif ($normalized -match '^/host_mnt/([A-Za-z])/(.*)$') { $normalized = "$($Matches[1]):/$($Matches[2])" }
    return $normalized.TrimEnd('/').ToLowerInvariant()
}

function Get-ContainerMountContract([string]$ContainerId) {
    $json = Invoke-ExternalText 'docker' @('inspect', $ContainerId, '--format', '{{json .Mounts}}') "Inspect mounts for $ContainerId"
    $parsed = $json | ConvertFrom-Json
    $items = @()
    foreach ($mount in $parsed) { $items += $mount }
    return @($items | ForEach-Object {
        $name = if ($null -ne $_.PSObject.Properties['Name']) { [string]$_.Name } else { '' }
        $source = if ($null -ne $_.PSObject.Properties['Source']) { [string]$_.Source } else { '' }
        [pscustomobject]@{
            Destination = [string]$_.Destination
            Type = [string]$_.Type
            Name = $name
            Source = ConvertTo-ComparableMountSource ([string]$_.Type) $source
            RW = [bool]$_.RW
        }
    } | Sort-Object Destination | ConvertTo-Json -Compress)
}

function Assert-SameMount([string]$Before, [string]$After, [string]$Name) {
    if ($Before -cne $After) { throw "$Name mount contract changed.`nBefore=$Before`nAfter=$After" }
}

function Get-RequiredVolumeContract {
    $result = [ordered]@{}
    foreach ($volume in $RequiredVolumes) {
        $result[$volume] = Invoke-ExternalText 'docker' @('volume', 'inspect', $volume, '--format', '{{.Name}}|{{.Driver}}|{{json .Options}}|{{json .Labels}}') "Inspect required volume $volume"
    }
    return $result
}

function Assert-SameVolumeContract([System.Collections.IDictionary]$Before, [System.Collections.IDictionary]$After) {
    foreach ($name in $Before.Keys) {
        if ($Before[$name] -cne $After[$name]) { throw "Required volume '$name' changed unexpectedly." }
    }
    Write-Host 'RequiredVolumesPreserved=true'
}

function Wait-HttpOk {
    param([string]$Uri, [string]$Name, [int]$TimeoutSeconds = 240, [hashtable]$Headers = @{})
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        try {
            $response = Invoke-WebRequest -UseBasicParsing -Uri $Uri -Headers $Headers -TimeoutSec 10
            if ([int]$response.StatusCode -eq 200) { Write-Host "$Name=200"; return }
        } catch { Start-Sleep -Seconds 2 }
    } while ((Get-Date) -lt $deadline)
    throw "$Name did not return HTTP 200 within $TimeoutSeconds seconds: $Uri"
}

function Wait-ContainerHealthy([string]$ContainerId, [string]$Name, [int]$TimeoutSeconds = 240) {
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        $state = Invoke-ExternalText 'docker' @('inspect', $ContainerId, '--format', '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}') "Inspect health for $Name"
        if ($state -eq 'healthy' -or $state -eq 'running') { Write-Host "$Name=$state"; return }
        Start-Sleep -Seconds 2
    } while ((Get-Date) -lt $deadline)
    throw "$Name did not become healthy/running; last state '$state'."
}

function Assert-PostgresReady([string]$ContainerId) {
    Invoke-External 'docker' @('exec', $ContainerId, 'pg_isready', '-U', $script:PgUser, '-d', $script:PgDatabase) 'PostgreSQL readiness'
    Write-Host 'PostgreSQL=ready'
}

function Get-RetainedCounts([string]$PostgresContainerId) {
    $sql = "SELECT (SELECT count(*) FROM forensic.evidence_items),(SELECT count(*) FROM forensic.evidence_versions),(SELECT count(*) FROM forensic.records_ingest_jobs),(SELECT count(*) FROM forensic.records_ingest_jobs WHERE status IN ('queued','running')),(SELECT count(*) FROM forensic.records),(SELECT count(*) FROM forensic.derived_artifacts),(SELECT count(*) FROM forensic.kb_collection_assets);"
    $line = Invoke-ExternalText 'docker' @('exec', $PostgresContainerId, 'psql', '-U', $script:PgUser, '-d', $script:PgDatabase, '-At', '-F', '|', '-c', $sql) 'Read retained counts'
    $parts = @($line.Trim() -split '\|')
    if ($parts.Count -ne 7) { throw "Unexpected retained-count result: '$line'" }
    return [ordered]@{
        evidence = [long]$parts[0]
        versions = [long]$parts[1]
        jobs = [long]$parts[2]
        active_jobs = [long]$parts[3]
        canonical_records = [long]$parts[4]
        artifacts = [long]$parts[5]
        kb_assets = [long]$parts[6]
    }
}

function Write-RetainedCounts([System.Collections.IDictionary]$Counts, [string]$Prefix) {
    foreach ($key in $Counts.Keys) { Write-Host "$Prefix.$key=$($Counts[$key])" }
    Write-Host "$Prefix.Tuple=$($Counts.evidence)|$($Counts.versions)|$($Counts.jobs)|$($Counts.active_jobs)|$($Counts.canonical_records)|$($Counts.artifacts)|$($Counts.kb_assets)"
}

function Assert-ExpectedRetainedCounts([System.Collections.IDictionary]$Counts) {
    foreach ($key in $ExpectedRetainedCounts.Keys) {
        if ([long]$Counts[$key] -ne [long]$ExpectedRetainedCounts[$key]) {
            throw "Unauthorized retained state for '$key': expected $($ExpectedRetainedCounts[$key]), got $($Counts[$key])."
        }
    }
    Write-Host 'ExpectedRetainedTuple=51|51|64|0|22207|441|47'
}

function Assert-SameRetainedCounts([System.Collections.IDictionary]$Before, [System.Collections.IDictionary]$After) {
    foreach ($key in $Before.Keys) {
        if ([long]$Before[$key] -ne [long]$After[$key]) { throw "Unauthorized retained-data mutation: $key changed from $($Before[$key]) to $($After[$key])." }
    }
}

function Assert-ExpectedSourceState {
    $branch = Invoke-ExternalText 'git' @('branch', '--show-current') 'Read Git branch'
    $head = Invoke-ExternalText 'git' @('rev-parse', 'HEAD') 'Read Git HEAD'
    if ($branch -cne $ExpectedBranch) { throw "Expected branch '$ExpectedBranch', got '$branch'." }
    if ($head -cne $ExpectedHead) { throw "Expected HEAD '$ExpectedHead', got '$head'." }
    $gitDir = Invoke-ExternalText 'git' @('rev-parse', '--git-dir') 'Resolve Git directory'
    foreach ($marker in @('MERGE_HEAD', 'CHERRY_PICK_HEAD', 'REVERT_HEAD')) {
        if (Test-Path -LiteralPath (Join-Path $gitDir $marker)) { throw "Unsafe in-progress Git operation: $marker" }
    }
    $statusText = Invoke-ExternalText 'git' @('status', '--porcelain=v1', '--untracked-files=all') 'Read worktree state'
    $dirty = @($statusText -split "`r?`n" | Where-Object { $_ })
    if ($dirty.Count -eq 0) { throw 'Expected the checkpointed materially dirty worktree, but the worktree is clean.' }
    Write-Host "GitBranch=$branch"
    Write-Host "GitHead=$head"
    Write-Host "DirtyWorktreeEntries=$($dirty.Count)"
    foreach ($entry in $ExpectedHashes.GetEnumerator()) {
        if (-not (Test-Path -LiteralPath $entry.Key -PathType Leaf)) { throw "Missing activation input: $($entry.Key)" }
        $actual = (Get-FileHash -Algorithm SHA256 -LiteralPath $entry.Key).Hash.ToLowerInvariant()
        if ($actual -cne $entry.Value) { throw "Checkpoint hash mismatch for '$($entry.Key)': expected $($entry.Value), got $actual." }
        Write-Host "SHA256 $actual  $($entry.Key)"
    }
    $checkpoint = Get-Content -LiteralPath $CheckpointPath -Raw | ConvertFrom-Json
    if ($checkpoint.contract_version -ne 'nexusai.mmv2.final-p1-live-certification/v1') { throw 'Unexpected MMV-2 live-certification checkpoint contract.' }
    if (($checkpoint.source_correction_20260825.services_to_build -join ',') -cne 'forensic-records-api,api') { throw 'Checkpoint deployment scope changed.' }
    if ([bool]$checkpoint.source_correction_20260825.worker_rebuild_required) { throw 'Checkpoint unexpectedly requires worker rebuild.' }
    if ([int]$checkpoint.source_correction_20260825.source_open_foundational_p1 -ne 0) { throw 'Checkpoint source closure is no longer complete.' }
    Write-Host 'CheckpointState=PASS'
    Write-Host 'ExactRebuildServices=forensic-records-api,api'
}

function Set-RequiredComposeEnvironment {
    $env:NEXUSAI_LOCALAI_RUNTIME_IMAGE = $LocalAIImage
    $env:NEXUSAI_FORENSIC_API_IMAGE = $ForensicApiImage
    $env:NEXUSAI_FORENSIC_WORKER_IMAGE = $WorkerImage
    $env:NEXUSAI_MEDIA_MODEL_DIR = $MediaModelDirectory
    $env:FORENSIC_ANPR_ENABLED = 'true'
    $env:FORENSIC_FACE_ENABLED = 'true'
    $env:FORENSIC_IMAGE_EMBEDDING_ENABLED = 'true'
    $env:FORENSIC_OCR_ENABLED = 'true'
    $env:NEXUSAI_AGENT_HISTORY_DATABASE_URL = $script:HistoryDatabaseUrl
}

function Get-SidecarComposeArguments([string[]]$Tail) {
    $args = @('compose', '-p', $ComposeProject, '--env-file', $RuntimeEnvFile)
    foreach ($file in $SidecarComposeFiles) { $args += @('-f', $file) }
    return $args + $Tail
}

function Get-LocalAIComposeArguments([string[]]$Tail) {
    $args = @('compose', '-p', $ComposeProject, '--env-file', $RuntimeEnvFile)
    foreach ($file in $LocalAIComposeFiles) { $args += @('-f', $file) }
    return $args + $Tail
}

function Assert-ComposeConfiguration {
    Invoke-External 'docker' (Get-SidecarComposeArguments @('config', '--quiet')) 'Forensic sidecar Compose validation'
    Invoke-External 'docker' (Get-LocalAIComposeArguments @('config', '--quiet')) 'LocalAI Compose validation'
    Write-Host 'ComposeConfiguration=PASS'
}

function Assert-ExpectedInitialRuntime([System.Collections.IDictionary]$Ids, [System.Collections.IDictionary]$Images) {
    foreach ($key in $ExpectedContainerIds.Keys) {
        if ($Ids[$key] -cne $ExpectedContainerIds[$key]) { throw "Container '$key' changed from checkpoint: expected $($ExpectedContainerIds[$key]), got $($Ids[$key])." }
    }
    if ($Images.localai -cne $ExpectedLocalAIRunningImageId) { throw "Current LocalAI image is $($Images.localai), expected $ExpectedLocalAIRunningImageId." }
    if ($Images.forensic_api -cne $ExpectedForensicApiRunningImageId) { throw "Current forensic API image is $($Images.forensic_api), expected $ExpectedForensicApiRunningImageId." }
    if ($Images.worker -cne $ExpectedWorkerRunningImageId) { throw "Current worker image is $($Images.worker), expected $ExpectedWorkerRunningImageId." }
}

function Assert-AllRuntimeHealth([System.Collections.IDictionary]$Ids) {
    Wait-ContainerHealthy $Ids.localai 'LocalAIContainer'
    Wait-ContainerHealthy $Ids.forensic_api 'ForensicApiContainer'
    Wait-ContainerHealthy $Ids.worker 'WorkerContainer'
    Wait-ContainerHealthy $Ids.nats 'NatsContainer'
    Wait-ContainerHealthy $Ids.postgres 'PostgreSQLContainer'
    Wait-HttpOk 'http://localhost:8080/readyz' 'LocalAIReadyz'
    Wait-HttpOk 'http://localhost:8080/' 'AnalystPortalRoot'
    Wait-HttpOk 'http://localhost:8080/analyst/home?case=nexusai-multimodal-product-acceptance' 'AnalystPortalHttp' 240 @{ Accept = 'text/html' }
    Wait-HttpOk 'http://localhost:8091/healthz' 'ForensicApiHealthz'
    Wait-HttpOk 'http://localhost:9109/metrics' 'WorkerMetrics'
    Wait-HttpOk 'http://localhost:8222/healthz' 'NatsHealthz'
    Assert-PostgresReady $Ids.postgres
}

function Assert-ProtectedIdsUnchanged([System.Collections.IDictionary]$Ids) {
    foreach ($key in @('worker', 'postgres', 'nats')) {
        if ($Ids[$key] -cne $script:BeforeIds[$key]) { throw "Protected service '$key' container changed: before=$($script:BeforeIds[$key]) after=$($Ids[$key])." }
    }
    Write-Host 'ProtectedServicesPreserved=true'
}

function Ensure-RollbackTag([string]$Tag, [string]$ExpectedId, [string]$Name) {
    $existingId = Invoke-ExternalText 'docker' @('image', 'ls', '--quiet', '--no-trunc', $Tag) "Probe $Name rollback tag"
    if (-not [string]::IsNullOrWhiteSpace($existingId)) {
        if ($existingId -cne $ExpectedId) { throw "$Name rollback tag '$Tag' already exists with unexpected ID $existingId; refusing to overwrite it." }
        Write-Host "$Name rollback tag already valid: $Tag $existingId"
    } else {
        Invoke-External 'docker' @('image', 'tag', $ExpectedId, $Tag) "Create $Name rollback tag"
    }
    $verified = Invoke-ExternalText 'docker' @('image', 'inspect', $Tag, '--format', '{{.Id}}') "Verify $Name rollback tag"
    if ($verified -cne $ExpectedId) { throw "$Name rollback tag verification failed: $verified" }
    Write-Host "${Name}Rollback=$Tag $verified"
}

function Assert-RollbackTagsExist {
    $localId = Invoke-ExternalText 'docker' @('image', 'inspect', $LocalAIRollbackTag, '--format', '{{.Id}}') 'Inspect LocalAI rollback tag'
    $apiId = Invoke-ExternalText 'docker' @('image', 'inspect', $ForensicApiRollbackTag, '--format', '{{.Id}}') 'Inspect forensic API rollback tag'
    if ($localId -cne $ExpectedLocalAIRunningImageId) { throw 'LocalAI rollback tag does not resolve to the exact protected image.' }
    if ($apiId -cne $ExpectedForensicApiRunningImageId) { throw 'Forensic API rollback tag does not resolve to the exact protected image.' }
    Write-Host 'RollbackImages=PASS'
}

function Invoke-GuardedRollback([string]$FailureMessage) {
    Write-Section 'AUTOMATIC GUARDED ROLLBACK'
    Write-Warning $FailureMessage
    Assert-RollbackTagsExist
    $env:NEXUSAI_FORENSIC_API_IMAGE = $ForensicApiRollbackTag
    Invoke-External 'docker' (Get-SidecarComposeArguments @('up', '-d', '--no-deps', '--force-recreate', 'forensic-records-api')) 'Rollback only forensic-records-api'
    $env:NEXUSAI_LOCALAI_RUNTIME_IMAGE = $LocalAIRollbackTag
    Invoke-External 'docker' (Get-LocalAIComposeArguments @('up', '-d', '--no-deps', '--force-recreate', 'api')) 'Rollback only LocalAI/UI api'
    $rollbackIds = Get-AllServiceIds
    Assert-ProtectedIdsUnchanged $rollbackIds
    if ((Get-ContainerImageId $rollbackIds.localai) -cne $ExpectedLocalAIRunningImageId) { throw 'Rolled-back LocalAI image mismatch.' }
    if ((Get-ContainerImageId $rollbackIds.forensic_api) -cne $ExpectedForensicApiRunningImageId) { throw 'Rolled-back forensic API image mismatch.' }
    Assert-SameMount $script:BeforeMounts.localai (Get-ContainerMountContract $rollbackIds.localai) 'Rolled-back LocalAI'
    Assert-SameMount $script:BeforeMounts.forensic_api (Get-ContainerMountContract $rollbackIds.forensic_api) 'Rolled-back forensic API'
    Assert-SameMount $script:BeforeMounts.worker (Get-ContainerMountContract $rollbackIds.worker) 'Protected worker'
    Assert-AllRuntimeHealth $rollbackIds
    $rollbackCounts = Get-RetainedCounts $rollbackIds.postgres
    Assert-SameRetainedCounts $script:BeforeCounts $rollbackCounts
    Assert-ExpectedRetainedCounts $rollbackCounts
    Assert-SameVolumeContract $script:BeforeVolumes (Get-RequiredVolumeContract)
    Write-RetainedCounts $rollbackCounts 'RollbackRetained'
    Write-Host 'MMV2EnterpriseClosureActivation=FAILED_ROLLED_BACK' -ForegroundColor Yellow
    Write-Host 'ProtectedServicesPreserved=true'
    Write-Host 'RetainedDataMutated=false'
    Write-Host 'ModelsChanged=false'
    Write-Host 'DatabaseMigration=false'
}

try {
    Set-Location -LiteralPath $RepositoryRoot
    $reportDirectory = Split-Path -Parent $OutputPath
    if (-not (Test-Path -LiteralPath $reportDirectory -PathType Container)) { throw "Required report directory is missing: $reportDirectory" }
    Start-Transcript -Path $OutputPath -Force | Out-Null
    $script:TranscriptStarted = $true

    Write-Section '1-3 REPOSITORY, DOCKER, BRANCH, AND WORKTREE'
    Write-Host "Repository=$((Get-Location).Path)"
    Invoke-External 'docker' @('version') 'Docker client and engine readiness'
    $dockerOs = Invoke-ExternalText 'docker' @('info', '--format', '{{.OSType}}') 'Docker engine OS check'
    if ($dockerOs -cne 'linux') { throw "Expected Docker Desktop Linux engine, got '$dockerOs'." }
    Write-Host 'DockerLinuxEngine=ready'
    Assert-ExpectedSourceState

    Write-Section '4-6 EXACT RUNTIME, HEALTH, AND BEFORE STATE'
    $script:BeforeIds = Get-AllServiceIds
    $script:BeforeImages = Get-AllServiceImages $script:BeforeIds
    Assert-ExpectedInitialRuntime $script:BeforeIds $script:BeforeImages
    foreach ($key in $script:BeforeIds.Keys) {
        Write-Host "BeforeContainer.$key=$($script:BeforeIds[$key])"
        Write-Host "BeforeImage.$key=$($script:BeforeImages[$key])"
    }
    $script:PgUser = Get-ContainerEnvironmentValue $script:BeforeIds.postgres 'POSTGRES_USER'
    $script:PgDatabase = Get-ContainerEnvironmentValue $script:BeforeIds.postgres 'POSTGRES_DB'
    $script:HistoryDatabaseUrl = Get-ContainerEnvironmentValue $script:BeforeIds.localai 'LOCALAI_AGENT_POOL_DATABASE_URL'
    Set-RequiredComposeEnvironment
    Assert-ComposeConfiguration
    Assert-AllRuntimeHealth $script:BeforeIds
    $script:BeforeCounts = Get-RetainedCounts $script:BeforeIds.postgres
    Write-RetainedCounts $script:BeforeCounts 'BeforeRetained'
    Assert-ExpectedRetainedCounts $script:BeforeCounts
    if ([long]$script:BeforeCounts.active_jobs -ne 0) { throw "MMV2EnterpriseClosureActivation=SAFE_STOP_ACTIVE_JOBS Active jobs=$($script:BeforeCounts.active_jobs); no Docker mutation was performed." }
    Write-Host 'ActiveForensicProcessingJobs=0'
    $script:BeforeMounts = [ordered]@{
        localai = Get-ContainerMountContract $script:BeforeIds.localai
        forensic_api = Get-ContainerMountContract $script:BeforeIds.forensic_api
        worker = Get-ContainerMountContract $script:BeforeIds.worker
        postgres = Get-ContainerMountContract $script:BeforeIds.postgres
        nats = Get-ContainerMountContract $script:BeforeIds.nats
    }
    $script:BeforeVolumes = Get-RequiredVolumeContract
    if (-not (Test-Path -LiteralPath $MediaModelDirectory -PathType Container)) { throw "Existing media model bind directory is missing: $MediaModelDirectory" }
    Write-Host 'ModelsAndVolumesPresent=PASS'

    Write-Section '7-9 MANDATORY PHYSICAL RAM GATE'
    Assert-FreePhysicalMemory

    Write-Section '10 ROLLBACK-TAG PLAN'
    Write-Host "LocalAIRollbackPlanned=$LocalAIRollbackTag $ExpectedLocalAIRunningImageId"
    Write-Host "ForensicApiRollbackPlanned=$ForensicApiRollbackTag $ExpectedForensicApiRunningImageId"

    if ($PreflightOnly) {
        Write-Host 'MMV2EnterpriseClosurePreflight=PASS' -ForegroundColor Green
        Write-Host 'ExactRebuildServices=forensic-records-api,api'
        Write-Host 'MutationPerformed=false'
    } else {
        Write-Section '10 CREATE AND VERIFY EXACT ROLLBACK TAGS'
        Ensure-RollbackTag $LocalAIRollbackTag $ExpectedLocalAIRunningImageId 'LocalAI'
        Ensure-RollbackTag $ForensicApiRollbackTag $ExpectedForensicApiRunningImageId 'ForensicApi'
        Assert-RollbackTagsExist

        Write-Section '11-13 FOCUSED SOURCE SMOKES AND EXACT CACHED BUILDS'
        Invoke-External 'go' @('test', './api/forensic_records', '-run', '^TestMMV2', '-count=1') 'Focused MMV-2 forensic API tests'
        Invoke-External 'go' @('test', './core/services/agents', '-run', '^TestAgents$', '-ginkgo.focus', 'preserves typed result state', '-count=1') 'Focused typed presentation test'
        Assert-FreePhysicalMemory
        Invoke-External 'docker' (Get-SidecarComposeArguments @('--progress', 'plain', 'build', 'forensic-records-api')) 'Build only forensic-records-api with Docker cache'
        Invoke-External 'docker' (Get-LocalAIComposeArguments @('--progress', 'plain', 'build', 'api')) 'Build only LocalAI/UI api with Docker cache'
        $builtApiId = Invoke-ExternalText 'docker' @('image', 'inspect', $ForensicApiImage, '--format', '{{.Id}}') 'Inspect built forensic API image'
        $builtLocalAIId = Invoke-ExternalText 'docker' @('image', 'inspect', $LocalAIImage, '--format', '{{.Id}}') 'Inspect built LocalAI image'
        if ($builtApiId -ceq $ExpectedForensicApiRunningImageId) { throw 'Forensic API build did not produce a new image ID.' }
        if ($builtLocalAIId -ceq $ExpectedLocalAIRunningImageId) { throw 'LocalAI/UI build did not produce a new image ID.' }
        Write-Host "BuiltForensicApiImage=$builtApiId"
        Write-Host "BuiltLocalAIImage=$builtLocalAIId"
        $apiEntrypoint = Invoke-ExternalText 'docker' @('image', 'inspect', $ForensicApiImage, '--format', '{{json .Config.Entrypoint}}') 'Inspect forensic API image entrypoint'
        if ($apiEntrypoint -cne '["/forensic-records-api"]') { throw "Unexpected forensic API entrypoint: $apiEntrypoint" }
        Invoke-External 'docker' @('run', '--rm', '--network', 'none', '--entrypoint', '/local-ai', $LocalAIImage, '--version') 'Built LocalAI binary smoke'
        Write-Host 'PreReplacementSmokeChecks=PASS'

        $preReplaceIds = Get-AllServiceIds
        Assert-ProtectedIdsUnchanged $preReplaceIds
        if ($preReplaceIds.localai -cne $script:BeforeIds.localai) { throw 'LocalAI/UI container changed during the guarded build; refusing recreation.' }
        if ($preReplaceIds.forensic_api -cne $script:BeforeIds.forensic_api) { throw 'Forensic API container changed during the guarded build; refusing recreation.' }
        $preReplaceCounts = Get-RetainedCounts $preReplaceIds.postgres
        Assert-SameRetainedCounts $script:BeforeCounts $preReplaceCounts
        Assert-ExpectedRetainedCounts $preReplaceCounts
        if ([long]$preReplaceCounts.active_jobs -ne 0) { throw "MMV2EnterpriseClosureActivation=SAFE_STOP_ACTIVE_JOBS Active jobs=$($preReplaceCounts.active_jobs); no service recreation was performed." }
        Write-Host 'PreRecreationActiveForensicProcessingJobs=0'

        Write-Section '14-18 NARROW RECREATION AND READINESS'
        $script:RecreationStarted = $true
        Invoke-External 'docker' (Get-SidecarComposeArguments @('up', '-d', '--no-deps', '--force-recreate', 'forensic-records-api')) 'Recreate only forensic-records-api'
        Wait-HttpOk 'http://localhost:8091/healthz' 'ForensicApiHealthz'
        Invoke-External 'docker' (Get-LocalAIComposeArguments @('up', '-d', '--no-deps', '--force-recreate', 'api')) 'Recreate only LocalAI/UI api'
        $afterIds = Get-AllServiceIds
        Assert-ProtectedIdsUnchanged $afterIds
        if ($afterIds.localai -ceq $script:BeforeIds.localai) { throw 'LocalAI/UI container was not recreated.' }
        if ($afterIds.forensic_api -ceq $script:BeforeIds.forensic_api) { throw 'Forensic API container was not recreated.' }
        Assert-AllRuntimeHealth $afterIds

        Write-Section '19-23 IMAGE, MOUNT, VOLUME, AND RETAINED-STATE VERIFICATION'
        $afterImages = Get-AllServiceImages $afterIds
        if ($afterImages.forensic_api -cne $builtApiId) { throw "Active forensic API image $($afterImages.forensic_api) does not match built image $builtApiId." }
        if ($afterImages.localai -cne $builtLocalAIId) { throw "Active LocalAI image $($afterImages.localai) does not match built image $builtLocalAIId." }
        if ($afterImages.worker -cne $script:BeforeImages.worker) { throw 'Protected worker image changed.' }
        Assert-SameMount $script:BeforeMounts.localai (Get-ContainerMountContract $afterIds.localai) 'LocalAI'
        Assert-SameMount $script:BeforeMounts.forensic_api (Get-ContainerMountContract $afterIds.forensic_api) 'Forensic API'
        Assert-SameMount $script:BeforeMounts.worker (Get-ContainerMountContract $afterIds.worker) 'Worker'
        Assert-SameMount $script:BeforeMounts.postgres (Get-ContainerMountContract $afterIds.postgres) 'PostgreSQL'
        Assert-SameMount $script:BeforeMounts.nats (Get-ContainerMountContract $afterIds.nats) 'NATS'
        Assert-SameVolumeContract $script:BeforeVolumes (Get-RequiredVolumeContract)
        $afterCounts = Get-RetainedCounts $afterIds.postgres
        Write-RetainedCounts $afterCounts 'AfterRetained'
        Assert-SameRetainedCounts $script:BeforeCounts $afterCounts
        Assert-ExpectedRetainedCounts $afterCounts

        Write-Section '24 FINAL EVIDENCE DISPLAY'
        Invoke-DisplayOnly 'docker' @('ps', '--no-trunc') 'docker ps'
        Invoke-DisplayOnly 'docker' @('image', 'inspect', $LocalAIImage, $ForensicApiImage, $WorkerImage, $LocalAIRollbackTag, $ForensicApiRollbackTag, '--format', '{{.Id}}|{{json .RepoTags}}') 'Relevant images and tags'
        foreach ($key in $afterIds.Keys) {
            Write-Host "ContainerBefore.$key=$($script:BeforeIds[$key])"
            Write-Host "ContainerAfter.$key=$($afterIds[$key])"
            Write-Host "FinalImage.$key=$($afterImages[$key])"
        }
        Write-RetainedCounts $afterCounts 'FinalRetained'

        Write-Section '25 FINAL SUCCESS MARKERS'
        Write-Host 'MMV2EnterpriseClosureActivation=PASS' -ForegroundColor Green
        Write-Host 'ProtectedServicesPreserved=true'
        Write-Host 'RetainedDataMutated=false'
        Write-Host 'ModelsChanged=false'
        Write-Host 'DatabaseMigration=false'
    }
}
catch {
    $failure = $_.Exception.Message
    Write-Host "ERROR: $failure" -ForegroundColor Red
    $script:ExitCode = 1
    if (-not $PreflightOnly -and $script:RecreationStarted -and $null -ne $script:BeforeIds) {
        try { Invoke-GuardedRollback $failure }
        catch {
            Write-Host "AUTOMATIC ROLLBACK FAILED: $($_.Exception.Message)" -ForegroundColor Red
            Write-Host 'MMV2EnterpriseClosureActivation=FAILED_ROLLBACK_INCOMPLETE' -ForegroundColor Red
        }
    } elseif (-not $PreflightOnly) {
        Write-Host 'MMV2EnterpriseClosureActivation=FAILED_NO_RECREATION' -ForegroundColor Yellow
        Write-Host 'RuntimeServiceMutation=false'
    }
}
finally {
    if ($script:TranscriptStarted) {
        try { Stop-Transcript | Out-Null } catch { }
    }
}

exit $script:ExitCode
