[CmdletBinding()]
param(
    [switch]$PreflightOnly
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

$RepositoryRoot = 'C:\Users\sheik\Workspace\Office\Projects\NexusAI'
$ExpectedBranch = 'codex/forensic-hybrid-checkpoint-20260723'
$ExpectedHead = '40717b83510c08db25dc26b9d6674bf46db363ac'
$RequiredFreeGiB = 6.0
$PreferredFreeGiB = 6.5
$ComposeProject = 'nexusai'
$RuntimeEnvFile = '.env.forensic-runtime.local'
$CheckpointPath = 'reports\mmv2-anpr-image-video-20260824\final-closure-predeployment-20260824.json'
$LocalAIImage = 'nexusai/localai-forensic:phase3-runtime'
$ForensicApiImage = 'nexusai/forensic-records-api:phase3-runtime'
$WorkerImage = 'nexusai/forensic-records-worker:phase3-runtime'
$LocalAIRollbackTag = 'nexusai/localai-forensic:rollback-before-mmv2-final-p1-20260825'
$ForensicApiRollbackTag = 'nexusai/forensic-records-api:rollback-before-mmv2-final-p1-20260825'
$ExpectedLocalAIRunningImageId = 'sha256:f08bb832d18eacfd679855d9dc4ea72a611a13fda88ad813135f7221edbe26e8'
$ExpectedForensicApiRunningImageId = 'sha256:9814cdf0e215070eafc640cedcc78ff0482dcb7b990369180fdba9334a420516'
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
$ExpectedContainerPrefixes = [ordered]@{
    localai = '0405fef3a5c5'
    forensic_api = 'a97e545b8356'
    worker = '1846f558941f'
    nats = '5c49e70d132a'
    postgres = 'f66e05a3b179'
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
    'api\forensic_records\query.go' = 'b618f550cc3f64d00f191b73ffd2cce3123ae1f60ab42842e565bce379e531df'
    'api\forensic_records\query_language_normalization.go' = 'fb9d75bbf56694e601b8b898b25876aaf17f6a809d3ba25402ac4cb9fe08e6cd'
    'api\forensic_records\video_anpr.go' = 'e1cd583f7a900fc32c906d4eaaafd5e329f686481f02bf9d95ffc586d4646e07'
    'api\forensic_records\cases.go' = '910b659a55cc86ece10764213224c531f5a4dcd8d5e4e54875d6c698bfd21b2a'
    'api\forensic_records\capabilities.go' = 'a45d0b4a8e8b7dbe50e01b729ef929863a120b828db5f8f07b78b6a48dc5bbd2'
    'api\forensic_records\main.go' = '21e469ce1974d976fff7ca0afcea36851ff31db85b109c6d53dcc67353c4376f'
    'api\forensic_records\mmv2_query_test.go' = '9461b674055ed984768116ff021726fd56ea99b011bf50164dd3e99b7863a3f6'
    'api\forensic_records\contracts\operation-certification-v1.json' = 'b61893faa296c13bd6d7a2bbe6ebc214d0769a456de7797d2d0eb0c633ffc1ab'
    'api\forensic_records\contracts\query-variant-ledger-v1.json' = '12f594345f45a44f1f2f697adc9fbb0bd65ce72a73f7359ce37e668f744bfb4f'
    'api\forensic_records\Dockerfile' = '3a54a1fa363db2ca14e00fb03cdb42b80c1bbdb493e53191c9b185c98fa69a1d'
    'core\services\agents\forensic_direct.go' = '0e5d5332c4fc640ff30c414ddf863bfb3d2c306ee274cc53a190db7b0113f25b'
    'core\services\agents\forensic_direct_test.go' = '363c128947814e4a5668c6a8eb2760ef0d5a025d773435dc9a34efe3e306f8ad'
    'core\services\agents\records_tools.go' = '0dfaf844ea9a60b5e12b7b306e7c43b2981d8818638be307afe4cbd097647ac1'
    'core\http\react-ui\src\analyst\AnalystData.jsx' = 'e9b7edd7de4e0b03e147383b2dddcfc8254eb01802404977092ff595eab3c390'
    'core\http\react-ui\src\analyst\AnalystAsk.jsx' = 'fc1d6b1f041612def796927eb3dcae49448e1286064c5b48044a2f4d41386626'
    'core\http\react-ui\src\analyst\analystMediaPresentation.js' = '308751c8af643a8d401a88ab66a085bf9b4b5555987ef6c158d58c68ef761249'
    'core\http\react-ui\src\analyst\analystMediaPresentation.test.js' = 'b06338e158325d4bbf2c546c623464358b658e723fdef03b938c08bbb2500e12'
    'Dockerfile' = '167f93165728a31b554751c2a100120eca34ffdcc8509ecb3ac5e393efe48a94'
    'docker-compose.forensic-records.yaml' = 'b0f724ea34bea80bfadf8491d03bf2ddc9a8cfcbc0f2982d1a5374b968e75dd6'
    'docker-compose.forensic-records.runtime.yaml' = '8793dff4a5980dc0d8dd6a453967cb6512daf13301f22039c770ab9d4410d923'
    'docker-compose.forensic-records.asr-small.yaml' = '4f39dfe6d572d1ffcddbe81339389167534f4b0f8b20e1d5915b0532827c11f0'
    'docker-compose.yaml' = '0ef74ce958237dbe66552dcb3e36a02def568eb1a179bcd04828cf54af4dc72d'
    'docker-compose.forensic-runtime.localai.yaml' = 'a4a10aca68a9291e9696d86f392a33e53df64992a31cca896c7c99492ce5ec5b'
    '.env' = 'e6e974b1c197e56722c03fd7092acaeb1ef9a98e37aff8197bb88904007c024d'
    '.env.forensic-runtime.local' = '82bb4660ef9615f821f3ef3157652054d6b338fa5324cb894f53ea6e9e5de726'
    'reports\mmv2-anpr-image-video-20260824\final-closure-predeployment-20260824.json' = '882ced1ce89edfc2ab47b775ca849a95847562e715ddf312b85c99178b942e23'
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

function Write-Section([string]$Title) {
    Write-Host ''
    Write-Host "=== $Title ===" -ForegroundColor Cyan
}

function Invoke-External {
    param([string]$FilePath, [string[]]$ArgumentList, [string]$Description)
    # BuildKit writes normal progress to stderr even when the build succeeds.
    # Merge and stringify both native streams inside this PowerShell process so
    # Windows PowerShell 5.1 does not promote progress into NativeCommandError
    # when the whole activation is piped through Tee-Object. Exit status remains
    # the authoritative failure signal.
    $previousPreference = $ErrorActionPreference
    $exitCode = 0
    try {
        $ErrorActionPreference = 'Continue'
        & $FilePath @ArgumentList 2>&1 | ForEach-Object { Write-Host ([string]$_) }
        $exitCode = $LASTEXITCODE
    }
    finally {
        $ErrorActionPreference = $previousPreference
    }
    if ($exitCode -ne 0) { throw "$Description failed with exit code $exitCode." }
}

function Invoke-ExternalText {
    param([string]$FilePath, [string[]]$ArgumentList, [string]$Description)
    $previousPreference = $ErrorActionPreference
    $exitCode = 0
    try {
        $ErrorActionPreference = 'Continue'
        $output = & $FilePath @ArgumentList 2>&1
        $exitCode = $LASTEXITCODE
    }
    finally {
        $ErrorActionPreference = $previousPreference
    }
    if ($exitCode -ne 0) {
        throw "$Description failed with exit code $exitCode.`n$($output -join [Environment]::NewLine)"
    }
    return ($output -join "`n").Trim()
}

function Invoke-DisplayOnly {
    param([string]$FilePath, [string[]]$ArgumentList, [string]$Description)
    try { & $FilePath @ArgumentList }
    catch { Write-Warning "$Description could not be displayed: $($_.Exception.Message)" }
    if ($LASTEXITCODE -ne 0) { Write-Warning "$Description returned exit code $LASTEXITCODE; substantive verification already passed." }
}

function Get-FreePhysicalMemoryGiB {
    $os = Get-CimInstance -ClassName Win32_OperatingSystem
    return [math]::Round(([double]$os.FreePhysicalMemory / 1MB), 2)
}

function Write-LargestMemoryConsumers {
    Write-Host 'LargestWindowsMemoryConsumers:'
    Get-Process -ErrorAction SilentlyContinue |
        Group-Object ProcessName |
        ForEach-Object {
            [pscustomobject]@{
                Process = $_.Name
                Instances = $_.Count
                WorkingSetMiB = [math]::Round((($_.Group | Measure-Object WorkingSet64 -Sum).Sum / 1MB), 1)
            }
        } |
        Sort-Object WorkingSetMiB -Descending |
        Select-Object -First 15 |
        Format-Table -AutoSize | Out-Host
}

function Assert-FreePhysicalMemory {
    $freeGiB = Get-FreePhysicalMemoryGiB
    Write-Host ("FreePhysicalMemoryGiB={0:N2}" -f $freeGiB)
    Write-Host ("RequiredFreePhysicalMemoryGiB={0:N2}" -f $RequiredFreeGiB)
    Write-Host ("PreferredLaunchFreePhysicalMemoryGiB={0:N2}" -f $PreferredFreeGiB)
    Write-LargestMemoryConsumers
    if ($freeGiB -lt $RequiredFreeGiB) {
        throw ("MMV2FinalP1Activation=SAFE_STOP_RAM_GATE Free physical RAM {0:N2} GiB is below mandatory {1:N2} GiB; no mutation was performed." -f $freeGiB, $RequiredFreeGiB)
    }
    if ($freeGiB -lt $PreferredFreeGiB) {
        Write-Warning ("Mandatory RAM gate passed, but free RAM {0:N2} GiB is below the preferred {1:N2} GiB launch margin." -f $freeGiB, $PreferredFreeGiB)
    }
}

function Get-ComposeContainerId([string]$Service) {
    $text = Invoke-ExternalText 'docker' @('ps', '-q', '--filter', "label=com.docker.compose.project=$ComposeProject", '--filter', "label=com.docker.compose.service=$Service") "Resolve running container for $Service"
    $ids = @($text -split "`r?`n" | Where-Object { $_.Trim() })
    if ($ids.Count -ne 1) { throw "Expected one running container for compose service '$Service'; found $($ids.Count)." }
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
        evidence = [long]$parts[0]; versions = [long]$parts[1]; jobs = [long]$parts[2]
        active_jobs = [long]$parts[3]; canonical_records = [long]$parts[4]
        artifacts = [long]$parts[5]; kb_assets = [long]$parts[6]
    }
}

function Write-RetainedCounts([System.Collections.IDictionary]$Counts, [string]$Prefix) {
    foreach ($key in $Counts.Keys) { Write-Host "$Prefix.$key=$($Counts[$key])" }
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
    if ($checkpoint.contract_version -ne 'nexusai.mmv2.final-closure-predeployment/v1') { throw 'Unexpected MMV-2 final closure checkpoint contract.' }
    if (($checkpoint.deployment_gate.required_services -join ',') -ne 'forensic-records-api,LocalAI/UI') { throw 'Checkpoint deployment scope changed.' }
    if ([bool]$checkpoint.deployment_gate.worker_rebuild_required) { throw 'Checkpoint unexpectedly requires worker rebuild.' }
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
    foreach ($key in $ExpectedContainerPrefixes.Keys) {
        if (-not $Ids[$key].StartsWith($ExpectedContainerPrefixes[$key], [System.StringComparison]::OrdinalIgnoreCase)) {
            throw "Container '$key' changed from checkpoint: $($Ids[$key])."
        }
    }
    if ($Images.localai -ne $ExpectedLocalAIRunningImageId) { throw "Current LocalAI image is $($Images.localai), expected $ExpectedLocalAIRunningImageId." }
    if ($Images.forensic_api -ne $ExpectedForensicApiRunningImageId) { throw "Current forensic API image is $($Images.forensic_api), expected $ExpectedForensicApiRunningImageId." }
    if ($Images.worker -ne $ExpectedWorkerRunningImageId) { throw "Current worker image is $($Images.worker), expected $ExpectedWorkerRunningImageId." }
}

function Assert-AllRuntimeHealth([System.Collections.IDictionary]$Ids, [switch]$RequireAnalystPortal) {
    Wait-ContainerHealthy $Ids.localai 'LocalAIContainer'
    Wait-ContainerHealthy $Ids.forensic_api 'ForensicApiContainer'
    Wait-ContainerHealthy $Ids.worker 'WorkerContainer'
    Wait-ContainerHealthy $Ids.nats 'NatsContainer'
    Wait-ContainerHealthy $Ids.postgres 'PostgreSQLContainer'
    Wait-HttpOk 'http://localhost:8080/readyz' 'LocalAIReadyz'
    Wait-HttpOk 'http://localhost:8091/healthz' 'ForensicApiHealthz'
    Wait-HttpOk 'http://localhost:9109/metrics' 'WorkerMetrics'
    Wait-HttpOk 'http://localhost:8222/healthz' 'NatsHealthz'
    Wait-HttpOk 'http://localhost:8080/' 'AnalystPortalRoot'
    if ($RequireAnalystPortal) {
        Wait-HttpOk 'http://localhost:8080/analyst/home?case=nexusai-multimodal-product-acceptance' 'AnalystPortalHttp' 240 @{ Accept = 'text/html' }
    }
    Assert-PostgresReady $Ids.postgres
}

function Assert-ProtectedIdsUnchanged([System.Collections.IDictionary]$Ids) {
    foreach ($key in @('worker', 'postgres', 'nats')) {
        if ($Ids[$key] -ne $script:BeforeIds[$key]) { throw "Protected service '$key' container changed: before=$($script:BeforeIds[$key]) after=$($Ids[$key])." }
    }
    Write-Host 'ProtectedServicesPreserved=true'
}

function Ensure-RollbackTag([string]$Tag, [string]$ExpectedId, [string]$Name) {
    # A missing-tag `docker image inspect` writes to stderr. Under Windows
    # PowerShell 5.1 plus ErrorActionPreference=Stop that becomes a terminating
    # NativeCommandError before its exit code can be handled. `image ls` is an
    # exact-reference, read-only existence probe that returns empty with exit 0.
    $existingId = Invoke-ExternalText 'docker' @('image', 'ls', '--quiet', '--no-trunc', $Tag) "Probe $Name rollback tag"
    if (-not [string]::IsNullOrWhiteSpace($existingId)) {
        if ($existingId -ne $ExpectedId) { throw "$Name rollback tag '$Tag' already exists with unexpected ID $existingId; refusing to overwrite it." }
        Write-Host "$Name rollback tag already valid: $Tag $existingId"
    } else {
        Invoke-External 'docker' @('image', 'tag', $ExpectedId, $Tag) "Create $Name rollback tag"
    }
    $verified = Invoke-ExternalText 'docker' @('image', 'inspect', $Tag, '--format', '{{.Id}}') "Verify $Name rollback tag"
    if ($verified -ne $ExpectedId) { throw "$Name rollback tag verification failed: $verified" }
    Write-Host "${Name}Rollback=$Tag $verified"
}

function Assert-RollbackTagsExist {
    $localId = Invoke-ExternalText 'docker' @('image', 'inspect', $LocalAIRollbackTag, '--format', '{{.Id}}') 'Inspect LocalAI rollback tag'
    $apiId = Invoke-ExternalText 'docker' @('image', 'inspect', $ForensicApiRollbackTag, '--format', '{{.Id}}') 'Inspect forensic API rollback tag'
    if ($localId -ne $ExpectedLocalAIRunningImageId) { throw 'LocalAI rollback tag no longer resolves to its exact protected image.' }
    if ($apiId -ne $ExpectedForensicApiRunningImageId) { throw 'Forensic API rollback tag no longer resolves to its exact protected image.' }
    Write-Host 'RollbackImages=PASS'
}

function Invoke-GuardedRollback([string]$FailureMessage) {
    Write-Section 'AUTOMATIC GUARDED ROLLBACK'
    Write-Warning $FailureMessage
    Assert-RollbackTagsExist
    $env:NEXUSAI_FORENSIC_API_IMAGE = $ForensicApiRollbackTag
    Invoke-External 'docker' (Get-SidecarComposeArguments @('up', '-d', '--no-deps', '--force-recreate', 'forensic-records-api')) 'Rollback only forensic API'
    $env:NEXUSAI_LOCALAI_RUNTIME_IMAGE = $LocalAIRollbackTag
    Invoke-External 'docker' (Get-LocalAIComposeArguments @('up', '-d', '--no-deps', '--force-recreate', 'api')) 'Rollback only LocalAI/UI'
    $rollbackIds = Get-AllServiceIds
    Assert-ProtectedIdsUnchanged $rollbackIds
    if ((Get-ContainerImageId $rollbackIds.localai) -ne $ExpectedLocalAIRunningImageId) { throw 'Rolled-back LocalAI image mismatch.' }
    if ((Get-ContainerImageId $rollbackIds.forensic_api) -ne $ExpectedForensicApiRunningImageId) { throw 'Rolled-back forensic API image mismatch.' }
    Assert-SameMount $script:BeforeMounts.localai (Get-ContainerMountContract $rollbackIds.localai) 'Rolled-back LocalAI'
    Assert-SameMount $script:BeforeMounts.forensic_api (Get-ContainerMountContract $rollbackIds.forensic_api) 'Rolled-back forensic API'
    Assert-SameMount $script:BeforeMounts.worker (Get-ContainerMountContract $rollbackIds.worker) 'Protected worker'
    Assert-AllRuntimeHealth $rollbackIds -RequireAnalystPortal
    $counts = Get-RetainedCounts $rollbackIds.postgres
    Assert-SameRetainedCounts $script:BeforeCounts $counts
    Assert-SameVolumeContract $script:BeforeVolumes (Get-RequiredVolumeContract)
    Write-RetainedCounts $counts 'RollbackRetained'
    Write-Host 'MMV2FinalP1Activation=FAILED_ROLLED_BACK' -ForegroundColor Yellow
    Write-Host 'RetainedDataMutated=false'
    Write-Host 'ModelsChanged=false'
    Write-Host 'DatabaseMigration=false'
}

Set-Location -LiteralPath $RepositoryRoot

try {
    Write-Section '1-2 REPOSITORY AND DOCKER READINESS'
    Write-Host "Repository=$((Get-Location).Path)"
    Invoke-External 'docker' @('version') 'Docker client and engine readiness'
    $dockerOs = Invoke-ExternalText 'docker' @('info', '--format', '{{.OSType}}') 'Docker engine OS check'
    if ($dockerOs -ne 'linux') { throw "Expected Docker Desktop Linux engine, got '$dockerOs'." }
    Write-Host 'DockerLinuxEngine=ready'

    Write-Section '3-8 SOURCE STATE AND MANDATORY RAM GATE'
    Assert-FreePhysicalMemory
    Assert-ExpectedSourceState

    Write-Section '9-10 PROTECTED RUNTIME AND BEFORE STATE'
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
    Assert-AllRuntimeHealth $script:BeforeIds -RequireAnalystPortal
    $script:BeforeCounts = Get-RetainedCounts $script:BeforeIds.postgres
    Write-RetainedCounts $script:BeforeCounts 'BeforeRetained'
    if ([long]$script:BeforeCounts.active_jobs -ne 0) { throw "MMV2FinalP1Activation=SAFE_STOP_ACTIVE_JOBS Active jobs=$($script:BeforeCounts.active_jobs); no mutation was performed." }
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

    Write-Section '11 ROLLBACK-TAG PLAN'
    Write-Host "LocalAIRollbackPlanned=$LocalAIRollbackTag $ExpectedLocalAIRunningImageId"
    Write-Host "ForensicApiRollbackPlanned=$ForensicApiRollbackTag $ExpectedForensicApiRunningImageId"

    if ($PreflightOnly) {
        Write-Host 'MMV2FinalP1Preflight=PASS' -ForegroundColor Green
        Write-Host 'ExactRebuildServices=forensic-records-api,api'
        Write-Host 'MutationPerformed=false'
        exit 0
    }

    Write-Section '11 CREATE AND VERIFY EXACT ROLLBACK TAGS'
    Ensure-RollbackTag $LocalAIRollbackTag $ExpectedLocalAIRunningImageId 'LocalAI'
    Ensure-RollbackTag $ForensicApiRollbackTag $ExpectedForensicApiRunningImageId 'ForensicApi'
    Assert-RollbackTagsExist

    Write-Section '12-15 FOCUSED SOURCE SMOKES AND EXACT CACHED BUILDS'
    Invoke-External 'go' @('test', './api/forensic_records', '-run', '^TestMMV2', '-count=1') 'Focused MMV-2 forensic API tests'
    Invoke-External 'go' @('test', './core/services/agents', '-run', '^(TestDeterministicForensicRouteCoverage|TestAllAcceptedForensicTemplatesAreExplicitlyRoutableFromAgentChat)$', '-count=1') 'Focused MMV-2 agent routing tests'
    Invoke-External 'node' @('--test', 'core\http\react-ui\src\analyst\analystMediaPresentation.test.js') 'Focused Analyst media presentation tests'
    Invoke-External 'docker' (Get-SidecarComposeArguments @('--progress', 'plain', 'build', 'forensic-records-api')) 'Build only forensic-records-api with cache'
    Invoke-External 'docker' (Get-LocalAIComposeArguments @('--progress', 'plain', 'build', 'api')) 'Build only LocalAI/UI with cache'
    $builtApiId = Invoke-ExternalText 'docker' @('image', 'inspect', $ForensicApiImage, '--format', '{{.Id}}') 'Inspect built forensic API image'
    $builtLocalAIId = Invoke-ExternalText 'docker' @('image', 'inspect', $LocalAIImage, '--format', '{{.Id}}') 'Inspect built LocalAI image'
    if ($builtApiId -eq $ExpectedForensicApiRunningImageId) { throw 'Forensic API build did not produce a new image ID.' }
    if ($builtLocalAIId -eq $ExpectedLocalAIRunningImageId) { throw 'LocalAI/UI build did not produce a new image ID.' }
    Write-Host "BuiltForensicApiImage=$builtApiId"
    Write-Host "BuiltLocalAIImage=$builtLocalAIId"
    $apiEntrypoint = Invoke-ExternalText 'docker' @('image', 'inspect', $ForensicApiImage, '--format', '{{json .Config.Entrypoint}}') 'Inspect forensic API image entrypoint'
    if ($apiEntrypoint -cne '["/forensic-records-api"]') { throw "Unexpected forensic API entrypoint: $apiEntrypoint" }
    Invoke-External 'docker' @('run', '--rm', '--network', 'none', '--entrypoint', '/local-ai', $LocalAIImage, '--version') 'Built LocalAI binary smoke'
    Write-Host 'PreReplacementSmokeChecks=PASS'

    Write-Section '16-20 NARROW RECREATION AND READINESS'
    $script:RecreationStarted = $true
    Invoke-External 'docker' (Get-SidecarComposeArguments @('up', '-d', '--no-deps', '--force-recreate', 'forensic-records-api')) 'Recreate only forensic-records-api'
    Wait-HttpOk 'http://localhost:8091/healthz' 'ForensicApiHealthz'
    Invoke-External 'docker' (Get-LocalAIComposeArguments @('up', '-d', '--no-deps', '--force-recreate', 'api')) 'Recreate only LocalAI/UI'
    $afterIds = Get-AllServiceIds
    Assert-ProtectedIdsUnchanged $afterIds
    Assert-AllRuntimeHealth $afterIds -RequireAnalystPortal

    Write-Section '21-29 IMAGE, MOUNT, VOLUME, AND RETAINED-STATE VERIFICATION'
    $afterImages = Get-AllServiceImages $afterIds
    if ($afterImages.forensic_api -ne $builtApiId) { throw "Active forensic API image $($afterImages.forensic_api) does not match built image $builtApiId." }
    if ($afterImages.localai -ne $builtLocalAIId) { throw "Active LocalAI image $($afterImages.localai) does not match built image $builtLocalAIId." }
    if ($afterImages.worker -ne $script:BeforeImages.worker) { throw 'Protected worker image changed.' }
    Assert-SameMount $script:BeforeMounts.localai (Get-ContainerMountContract $afterIds.localai) 'LocalAI'
    Assert-SameMount $script:BeforeMounts.forensic_api (Get-ContainerMountContract $afterIds.forensic_api) 'Forensic API'
    Assert-SameMount $script:BeforeMounts.worker (Get-ContainerMountContract $afterIds.worker) 'Worker'
    Assert-SameMount $script:BeforeMounts.postgres (Get-ContainerMountContract $afterIds.postgres) 'PostgreSQL'
    Assert-SameMount $script:BeforeMounts.nats (Get-ContainerMountContract $afterIds.nats) 'NATS'
    Assert-SameVolumeContract $script:BeforeVolumes (Get-RequiredVolumeContract)
    $afterCounts = Get-RetainedCounts $afterIds.postgres
    Write-RetainedCounts $afterCounts 'AfterRetained'
    Assert-SameRetainedCounts $script:BeforeCounts $afterCounts

    Write-Section '30 FINAL EVIDENCE DISPLAY'
    Invoke-DisplayOnly 'docker' @('ps', '--no-trunc') 'docker ps'
    Invoke-DisplayOnly 'docker' @('image', 'inspect', $LocalAIImage, $ForensicApiImage, $WorkerImage, $LocalAIRollbackTag, $ForensicApiRollbackTag, '--format', '{{.Id}}|{{json .RepoTags}}') 'Relevant images and tags'
    foreach ($key in $afterIds.Keys) {
        Write-Host "ContainerBefore.$key=$($script:BeforeIds[$key])"
        Write-Host "ContainerAfter.$key=$($afterIds[$key])"
        Write-Host "FinalImage.$key=$($afterImages[$key])"
    }
    Assert-AllRuntimeHealth $afterIds -RequireAnalystPortal
    Write-RetainedCounts $afterCounts 'FinalRetained'

    Write-Section '31 FINAL SUCCESS MARKERS'
    Write-Host 'MMV2FinalP1Activation=PASS' -ForegroundColor Green
    Write-Host 'ProtectedServicesPreserved=true'
    Write-Host 'RetainedDataMutated=false'
    Write-Host 'ModelsChanged=false'
    Write-Host 'DatabaseMigration=false'
}
catch {
    $failure = $_.Exception.Message
    Write-Host "ERROR: $failure" -ForegroundColor Red
    if (-not $PreflightOnly -and $script:RecreationStarted -and $null -ne $script:BeforeIds) {
        try { Invoke-GuardedRollback $failure }
        catch {
            Write-Host "AUTOMATIC ROLLBACK FAILED: $($_.Exception.Message)" -ForegroundColor Red
            Write-Host 'MMV2FinalP1Activation=FAILED_MANUAL_INTERVENTION_REQUIRED' -ForegroundColor Red
        }
    } elseif (-not $PreflightOnly) {
        Write-Host 'MMV2FinalP1Activation=FAILED_NO_RECREATION' -ForegroundColor Yellow
        Write-Host 'RuntimeServiceMutation=false'
    }
    exit 1
}
