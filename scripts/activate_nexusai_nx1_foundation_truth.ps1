[CmdletBinding()]
param(
    [switch]$PreflightOnly
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

$RepositoryRoot = 'C:\Users\sheik\Workspace\Office\Projects\NexusAI'
$ReportDirectory = 'reports\nx1-foundation-truth-20260825'
$PreflightOutputPath = Join-Path $ReportDirectory 'nx1-final-source-closure-preflight-output.txt'
$ActivationOutputPath = Join-Path $ReportDirectory 'nx1-final-source-closure-activation-output.txt'
$ExpectedBranch = 'codex/forensic-hybrid-checkpoint-20260723'
$ExpectedHead = '40717b83510c08db25dc26b9d6674bf46db363ac'
$CheckpointPath = 'reports\nexusai-nx1-activation-manifest-20260825.json'
$RequiredFreeRAMKiB = 6291456L
$PreferredFreeRAMGiB = 6.5
$ComposeProject = 'nexusai'
$RuntimeEnvFile = '.env.forensic-runtime.local'
$BaseEnvFile = '.env'
$LocalAIImage = 'nexusai/localai-forensic:phase3-runtime'
$ForensicApiImage = 'nexusai/forensic-records-api:phase3-runtime'
$WorkerImage = 'nexusai/forensic-records-worker:phase3-runtime'
$LocalAIRollbackTag = 'nexusai/localai-forensic:rollback-before-nx1-final-source-closure-20260825'
$ForensicApiRollbackTag = 'nexusai/forensic-records-api:rollback-before-nx1-final-source-closure-20260825'
$ExpectedLocalAIRunningImageId = 'sha256:11549bad5bf378cb3cfee20fcbc7a384767a98eab2d07dd490c970fddc68cb2f'
$ExpectedForensicApiRunningImageId = 'sha256:4eb2db0af60d80a1ea75733cc5db559d508197cd2504006f77ccad4ff708e14b'
$ExpectedWorkerRunningImageId = 'sha256:be13dbec4bd9bb62c824140df3777b898d7ad2801a0e68cd512c00dafaf4f9ef'
$ExpectedNatsRunningImageId = 'sha256:e4bf19f15fd3218814a4e3c9e0064e1334bd8aa20d5984b9f1a0afd084f8cc00'
$ExpectedPostgresRunningImageId = 'sha256:61f891691050da6032023c01ea885730eeeba06b7c17b403e7d0b9c49c37dfe9'
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
    localai = '77d0f172f0ca56bd40d8dd2d8a3777a1984997c3bf09899faab4709e6c708aa9'
    forensic_api = '1bdf4f77087ebf8870a640704d95fea36e152bf198175f782bd451bcb526b074'
    worker = '1846f558941f583467976ad8e74a44e9063c8a1a50e870951c7e479ecce3fb66'
    nats = '5c49e70d132ac01b4adf86f27b1735d4d5cf6cc205d848c0881164f1f200eb04'
    postgres = 'f66e05a3b17978c9912aa911177f22e57d69c17de87881a33f23bcba228ad361'
}
$ExpectedImages = [ordered]@{
    localai = $ExpectedLocalAIRunningImageId
    forensic_api = $ExpectedForensicApiRunningImageId
    worker = $ExpectedWorkerRunningImageId
    nats = $ExpectedNatsRunningImageId
    postgres = $ExpectedPostgresRunningImageId
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
    'core\services\agents\forensic_direct.go' = '300fba20e33ce70c124e4cbddc3998dc5a5aa56f29a38f9ca6d9a05cbdcfabef'
    'core\services\agents\forensic_direct_test.go' = '8dcf736e937577efce35e99435567555f4652a1a6f04f01785abbcdda1a42c5e'
    'core\services\agents\analysis_history.go' = '180f0fbf87f41d875ad5f2e58ed04edf684ca50e5de92c5c7cb4ade8cfff0055'
    'core\services\agents\analysis_history_test.go' = '4914e5372190da790d48e155c6cdcac1c6efa2489e196d5d0759f32f2a38acc3'
    'api\forensic_records\query.go' = 'a92691a9505c1c850cc89cf40208a823322912a223ecabd142f4cba625c1557b'
    'api\forensic_records\cases.go' = '9886c1bfe682353833cbe66236c79df31f367656dd381e182a0ea71b33b6321c'
    'api\forensic_records\mmv2_query_test.go' = '768ba5a877d4236979fca23277d428f04367ec1e69376035566af4367d20c9a4'
    'core\services\agents\forensic_presentation.go' = 'ca4778904c1418f9ad3cdebaf78b2ce09210931e0fb34fdd46f221e7b400fc64'
    'core\services\agents\forensic_presentation_test.go' = '7785149faebc75ed0e4037645ff890c80531352ca2efefc4ebb62c16b27f77af'
    'core\http\react-ui\src\pages\AgentChat.jsx' = '0ce908036028f59f917f8d80e7b2fbe5cb3df7f9ea16efc3c3218235c9ed3bbe'
    'core\http\react-ui\src\analyst\AnalystHistory.jsx' = 'aafe288016d38e9dd69e6f066d6113bd2d79262a962b44b6b78a40a95cce6a8c'
    'Dockerfile' = '167f93165728a31b554751c2a100120eca34ffdcc8509ecb3ac5e393efe48a94'
    'api\forensic_records\Dockerfile' = '3a54a1fa363db2ca14e00fb03cdb42b80c1bbdb493e53191c9b185c98fa69a1d'
    'docker-compose.yaml' = '0ef74ce958237dbe66552dcb3e36a02def568eb1a179bcd04828cf54af4dc72d'
    'docker-compose.forensic-runtime.localai.yaml' = 'a4a10aca68a9291e9696d86f392a33e53df64992a31cca896c7c99492ce5ec5b'
    'docker-compose.forensic-records.yaml' = 'b0f724ea34bea80bfadf8491d03bf2ddc9a8cfcbc0f2982d1a5374b968e75dd6'
    'docker-compose.forensic-records.runtime.yaml' = '8793dff4a5980dc0d8dd6a453967cb6512daf13301f22039c770ab9d4410d923'
    'docker-compose.forensic-records.asr-small.yaml' = '4f39dfe6d572d1ffcddbe81339389167534f4b0f8b20e1d5915b0532827c11f0'
    '.env' = 'e6e974b1c197e56722c03fd7092acaeb1ef9a98e37aff8197bb88904007c024d'
    '.env.forensic-runtime.local' = '82bb4660ef9615f821f3ef3157652054d6b338fa5324cb894f53ea6e9e5de726'
    'reports\nexusai-nx1-activation-manifest-20260825.json' = '6907d1dac803e4eb6906bb0d94e0eb1626bf7e23a4c94b54e82ef8105dd64d46'
}

$script:BeforeIds = $null
$script:BeforeImages = $null
$script:RuntimeImageTags = $null
$script:BeforeCounts = $null
$script:BeforeMounts = $null
$script:BeforeVolumes = $null
$script:BeforeModels = $null
$script:BeforeProfiles = $null
$script:PgUser = $null
$script:PgDatabase = $null
$script:HistoryDatabaseUrl = $null
$script:RecreationStarted = $false
$script:RamGateFailed = $false
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

function Get-StringSha256([string]$Value) {
    $sha = [System.Security.Cryptography.SHA256]::Create()
    try {
        $bytes = [System.Text.Encoding]::UTF8.GetBytes($Value)
        return (($sha.ComputeHash($bytes) | ForEach-Object { $_.ToString('x2') }) -join '')
    }
    finally { $sha.Dispose() }
}

function Get-PhysicalMemoryState {
    $os = Get-CimInstance -ClassName Win32_OperatingSystem
    return [pscustomobject]@{
        TotalKiB = [long]$os.TotalVisibleMemorySize
        FreeKiB = [long]$os.FreePhysicalMemory
        TotalGiB = [math]::Round(([double]$os.TotalVisibleMemorySize / 1MB), 3)
        FreeGiB = [math]::Round(([double]$os.FreePhysicalMemory / 1MB), 3)
        RequiredGiB = [math]::Round(([double]$RequiredFreeRAMKiB / 1MB), 3)
    }
}

function Write-TopMemoryProcesses {
    Write-Host 'TopWindowsProcessesByWorkingSet:'
    Get-Process | Sort-Object WorkingSet64 -Descending | Select-Object -First 15 `
        @{Name='Process';Expression={$_.ProcessName}},
        @{Name='PID';Expression={$_.Id}},
        @{Name='WorkingSetGiB';Expression={[math]::Round(([double]$_.WorkingSet64 / 1GB), 3)}} |
        Format-Table -AutoSize | Out-String | Write-Host
}

function Assert-FreePhysicalMemory {
    $memory = Get-PhysicalMemoryState
    Write-Host ("TotalPhysicalRAMGiB={0:N3}" -f $memory.TotalGiB)
    Write-Host ("FreePhysicalRAMGiB={0:N3}" -f $memory.FreeGiB)
    Write-Host ("RequiredFreeRAMGiB={0:N3}" -f $memory.RequiredGiB)
    Write-Host ("PreferredLaunchFreeRAMGiB={0:N1}" -f $PreferredFreeRAMGiB)
    if ($memory.FreeKiB -lt $RequiredFreeRAMKiB) {
        $script:RamGateFailed = $true
        Write-Host 'RAMGate=FAIL' -ForegroundColor Red
        Write-TopMemoryProcesses
        throw ("Free physical RAM {0:N3} GiB is below mandatory {1:N3} GiB." -f $memory.FreeGiB, $memory.RequiredGiB)
    }
    Write-Host 'RAMGate=PASS' -ForegroundColor Green
    if ($memory.FreeGiB -lt $PreferredFreeRAMGiB) {
        Write-Warning ("Mandatory RAM gate passed, but free RAM {0:N3} GiB is below the preferred {1:N1} GiB margin." -f $memory.FreeGiB, $PreferredFreeRAMGiB)
    } else {
        Write-Host 'PreferredLaunchRAM=PASS'
    }
}

function Write-DiskState {
    $driveName = (Split-Path -Qualifier $RepositoryRoot).TrimEnd(':')
    $drive = Get-PSDrive -Name $driveName
    $freeGiB = [math]::Round(([double]$drive.Free / 1GB), 2)
    $usedGiB = [math]::Round(([double]$drive.Used / 1GB), 2)
    Write-Host "BuildDrive=$driveName`:"
    Write-Host "BuildDriveFreeGiB=$freeGiB"
    Write-Host "BuildDriveUsedGiB=$usedGiB"
    Write-Host 'DiskPolicy=No mandatory deployment minimum is defined by the current guarded activation policy.'
    if ($drive.Free -le 0) { throw 'Build drive reports no available free space.' }
    Write-Host 'DiskGate=PASS_CURRENT_PROJECT_POLICY'
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
    Write-Host 'NamedVolumesPreserved=true'
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
    $stateText = ''
    do {
        $stateJson = Invoke-ExternalText 'docker' @('inspect', $ContainerId, '--format', '{{json .State}}') "Inspect health for $Name"
        $state = $stateJson | ConvertFrom-Json
        $stateText = [string]$state.Status
        if ($null -ne $state.PSObject.Properties['Health']) { $stateText = [string]$state.Health.Status }
        if ($stateText -eq 'healthy' -or $stateText -eq 'running') { Write-Host "$Name=$stateText"; return }
        Start-Sleep -Seconds 2
    } while ((Get-Date) -lt $deadline)
    throw "$Name did not become healthy/running; last state '$stateText'."
}

function Assert-PostgresReady([string]$ContainerId) {
    Invoke-External 'docker' @('exec', $ContainerId, 'pg_isready', '-U', $script:PgUser, '-d', $script:PgDatabase) 'PostgreSQL readiness'
    Write-Host 'PostgreSQL=ready'
}

function Get-RetainedCounts([string]$PostgresContainerId) {
    $sql = "SELECT (SELECT count(*) FROM forensic.evidence_items),(SELECT count(*) FROM forensic.evidence_versions),(SELECT count(*) FROM forensic.records_ingest_jobs),(SELECT count(*) FROM forensic.records_ingest_jobs WHERE status::text IN ('queued','running','processing')),(SELECT count(*) FROM forensic.records),(SELECT count(*) FROM forensic.derived_artifacts),(SELECT count(*) FROM forensic.kb_collection_assets);"
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

function Assert-SameRetainedCounts([System.Collections.IDictionary]$Before, [System.Collections.IDictionary]$After) {
    foreach ($key in $Before.Keys) {
        if ([long]$Before[$key] -ne [long]$After[$key]) { throw "Unauthorized retained-data mutation: $key changed from $($Before[$key]) to $($After[$key])." }
    }
    Write-Host 'RetainedDataMutated=false'
}

function Get-ModelContract {
    $response = Invoke-WebRequest -UseBasicParsing -TimeoutSec 30 -Uri 'http://localhost:8080/v1/models'
    if ([int]$response.StatusCode -ne 200) { throw 'Model inventory endpoint did not return HTTP 200.' }
    $payload = $response.Content | ConvertFrom-Json
    $models = @($payload.data | Sort-Object id | ForEach-Object {
        $objectValue = if ($null -ne $_.PSObject.Properties['object']) { [string]$_.object } else { '' }
        $ownerValue = if ($null -ne $_.PSObject.Properties['owned_by']) { [string]$_.owned_by } else { '' }
        [ordered]@{ id = [string]$_.id; object = $objectValue; owned_by = $ownerValue }
    })
    $json = ConvertTo-Json -InputObject $models -Compress -Depth 5
    return [ordered]@{ count = $models.Count; sha256 = Get-StringSha256 $json; json = $json }
}

function Assert-SameModelContract([System.Collections.IDictionary]$Before, [System.Collections.IDictionary]$After) {
    if ($Before.count -ne $After.count -or $Before.sha256 -cne $After.sha256) {
        throw "Model inventory changed: before=$($Before.count)/$($Before.sha256) after=$($After.count)/$($After.sha256)."
    }
    Write-Host 'ModelsChanged=false'
}

function Get-ProfileContract {
    $files = @($BaseEnvFile, $RuntimeEnvFile) + $SidecarComposeFiles + $LocalAIComposeFiles
    $result = [ordered]@{}
    foreach ($file in $files) {
        $result[$file] = (Get-FileHash -Algorithm SHA256 -LiteralPath $file).Hash.ToLowerInvariant()
    }
    return $result
}

function Assert-SameProfileContract([System.Collections.IDictionary]$Before, [System.Collections.IDictionary]$After) {
    foreach ($file in $Before.Keys) {
        if ($Before[$file] -cne $After[$file]) { throw "Environment/profile input changed: $file" }
    }
    Write-Host 'ProfilesChanged=false'
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
    if ($dirty.Count -eq 0) { throw 'Expected the checkpointed dirty worktree, but the worktree is clean.' }
    Write-Host "GitBranch=$branch"
    Write-Host "GitHead=$head"
    Write-Host "DirtyWorktreeEntries=$($dirty.Count)"
    foreach ($entry in $ExpectedHashes.GetEnumerator()) {
        if (-not (Test-Path -LiteralPath $entry.Key -PathType Leaf)) { throw "Missing activation input: $($entry.Key)" }
        $actual = (Get-FileHash -Algorithm SHA256 -LiteralPath $entry.Key).Hash.ToLowerInvariant()
        if ($actual -cne $entry.Value) { throw "Checkpoint hash mismatch for '$($entry.Key)': expected $($entry.Value), got $actual." }
        Write-Host "SourceHash=PASS $($entry.Key) $actual"
    }
    $checkpoint = Get-Content -LiteralPath $CheckpointPath -Raw | ConvertFrom-Json
    if ($checkpoint.contract_version -ne 'nexusai.nx1-activation-manifest/v1') { throw 'Unexpected NX-1 activation checkpoint contract.' }
    if ($checkpoint.status -ne 'final_source_closure_pass_activation_not_approved') { throw "Unexpected checkpoint status '$($checkpoint.status)'." }
    if (($checkpoint.deployment_scope.rebuild -join ',') -cne 'forensic-records-api,api (LocalAI/UI)') { throw 'Checkpoint deployment scope changed.' }
    if (($checkpoint.deployment_scope.must_not_rebuild -join ',') -cne 'forensic-records-worker,forensic-nats,forensic-postgres') { throw 'Checkpoint protected-service scope changed.' }
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
        if ($Ids[$key] -cne $ExpectedContainerIds[$key]) { throw "Container '$key' differs from the verified NX-1 checkpoint: expected $($ExpectedContainerIds[$key]), got $($Ids[$key]). Reverify before activation." }
        if ($Images[$key] -cne $ExpectedImages[$key]) { throw "Image '$key' differs from the verified NX-1 checkpoint: expected $($ExpectedImages[$key]), got $($Images[$key]). Reverify before activation." }
    }
    $localTagId = Invoke-ExternalText 'docker' @('image', 'inspect', $LocalAIImage, '--format', '{{.Id}}') 'Inspect current LocalAI runtime tag'
    $apiTagId = Invoke-ExternalText 'docker' @('image', 'inspect', $ForensicApiImage, '--format', '{{.Id}}') 'Inspect current forensic API runtime tag'
    $script:RuntimeImageTags = [ordered]@{
        localai = $localTagId
        forensic_api = $apiTagId
    }
    if ($localTagId -cne $Images.localai) {
        $rollbackId = Invoke-ExternalText 'docker' @('image', 'inspect', $LocalAIRollbackTag, '--format', '{{.Id}}') 'Inspect LocalAI rollback tag for interrupted-build resume'
        if ($rollbackId -cne $Images.localai) { throw 'Current LocalAI runtime tag does not resolve to the running pre-NX-1 image and rollback tag does not prove the pre-activation image.' }
        Write-Host "CurrentRuntimeTag.localai=RESUME_BUILT_IMAGE $localTagId"
    }
    if ($apiTagId -cne $Images.forensic_api) {
        $rollbackId = Invoke-ExternalText 'docker' @('image', 'inspect', $ForensicApiRollbackTag, '--format', '{{.Id}}') 'Inspect forensic API rollback tag for interrupted-build resume'
        if ($rollbackId -cne $Images.forensic_api) { throw 'Current forensic API runtime tag does not resolve to the running pre-NX-1 image and rollback tag does not prove the pre-activation image.' }
        Write-Host "CurrentRuntimeTag.forensic_api=RESUME_BUILT_IMAGE $apiTagId"
    }
    Write-Host 'CurrentRuntimeImages=PASS'
}

function Assert-AllRuntimeHealth([System.Collections.IDictionary]$Ids) {
    Wait-ContainerHealthy $Ids.localai 'LocalAIContainer'
    Wait-ContainerHealthy $Ids.forensic_api 'ForensicApiContainer'
    Wait-ContainerHealthy $Ids.worker 'WorkerContainer'
    Wait-ContainerHealthy $Ids.nats 'NatsContainer'
    Wait-ContainerHealthy $Ids.postgres 'PostgreSQLContainer'
    Wait-HttpOk 'http://localhost:8080/readyz' 'LocalAIReadyz'
    Wait-HttpOk 'http://localhost:8080/analyst/home?case=nexusai-multimodal-product-acceptance' 'AnalystUtilityHTML' 240 @{ Accept = 'text/html' }
    Wait-HttpOk 'http://localhost:8091/healthz' 'ForensicApiHealthz'
    Wait-HttpOk 'http://localhost:9109/metrics' 'WorkerMetrics'
    Wait-HttpOk 'http://localhost:8222/healthz' 'NatsHealthz'
    Assert-PostgresReady $Ids.postgres
}

function Assert-ProtectedIdsUnchanged([System.Collections.IDictionary]$Ids) {
    foreach ($key in @('worker', 'postgres', 'nats')) {
        if ($Ids[$key] -cne $script:BeforeIds[$key]) { throw "Protected service '$key' container changed: before=$($script:BeforeIds[$key]) after=$($Ids[$key])." }
        if ((Get-ContainerImageId $Ids[$key]) -cne $script:BeforeImages[$key]) { throw "Protected service '$key' image changed." }
    }
    Write-Host 'ProtectedServicesPreserved=true'
}

function Ensure-RollbackTag([string]$Tag, [string]$ExpectedId, [string]$Name) {
    $existingId = Invoke-ExternalText 'docker' @('image', 'ls', '--quiet', '--no-trunc', $Tag) "Probe $Name rollback tag"
    if (-not [string]::IsNullOrWhiteSpace($existingId)) {
        if ($existingId -cne $ExpectedId) { throw "$Name rollback tag '$Tag' already exists with unexpected ID $existingId; refusing to overwrite it." }
        Write-Host "$Name rollback tag already resolves correctly: $Tag $existingId"
    } else {
        Invoke-External 'docker' @('image', 'tag', $ExpectedId, $Tag) "Create $Name rollback tag"
    }
    $verified = Invoke-ExternalText 'docker' @('image', 'inspect', $Tag, '--format', '{{.Id}}') "Verify $Name rollback tag"
    if ($verified -cne $ExpectedId) { throw "$Name rollback tag verification failed: expected $ExpectedId got $verified" }
    Write-Host "${Name}Rollback=$Tag $verified"
}

function Assert-RollbackTagsExist {
    $localId = Invoke-ExternalText 'docker' @('image', 'inspect', $LocalAIRollbackTag, '--format', '{{.Id}}') 'Inspect LocalAI rollback tag'
    $apiId = Invoke-ExternalText 'docker' @('image', 'inspect', $ForensicApiRollbackTag, '--format', '{{.Id}}') 'Inspect forensic API rollback tag'
    if ($localId -cne $script:BeforeImages.localai) { throw 'LocalAI rollback tag does not resolve to the exact pre-activation image.' }
    if ($apiId -cne $script:BeforeImages.forensic_api) { throw 'Forensic API rollback tag does not resolve to the exact pre-activation image.' }
    Write-Host 'RollbackImages=PASS'
}

function Invoke-GuardedRollback([string]$FailureMessage) {
    Write-Section 'AUTOMATIC GUARDED TWO-SERVICE ROLLBACK'
    Write-Warning $FailureMessage
    Assert-RollbackTagsExist
    $env:NEXUSAI_FORENSIC_API_IMAGE = $ForensicApiRollbackTag
    Invoke-External 'docker' (Get-SidecarComposeArguments @('up', '-d', '--no-deps', '--force-recreate', 'forensic-records-api')) 'Rollback only forensic-records-api'
    $env:NEXUSAI_LOCALAI_RUNTIME_IMAGE = $LocalAIRollbackTag
    Invoke-External 'docker' (Get-LocalAIComposeArguments @('up', '-d', '--no-deps', '--force-recreate', 'api')) 'Rollback only LocalAI/UI api'
    $rollbackIds = Get-AllServiceIds
    Assert-ProtectedIdsUnchanged $rollbackIds
    if ((Get-ContainerImageId $rollbackIds.localai) -cne $script:BeforeImages.localai) { throw 'Rolled-back LocalAI image mismatch.' }
    if ((Get-ContainerImageId $rollbackIds.forensic_api) -cne $script:BeforeImages.forensic_api) { throw 'Rolled-back forensic API image mismatch.' }
    Assert-SameMount $script:BeforeMounts.localai (Get-ContainerMountContract $rollbackIds.localai) 'Rolled-back LocalAI'
    Assert-SameMount $script:BeforeMounts.forensic_api (Get-ContainerMountContract $rollbackIds.forensic_api) 'Rolled-back forensic API'
    Assert-SameMount $script:BeforeMounts.worker (Get-ContainerMountContract $rollbackIds.worker) 'Protected worker'
    Assert-AllRuntimeHealth $rollbackIds
    $rollbackCounts = Get-RetainedCounts $rollbackIds.postgres
    Assert-SameRetainedCounts $script:BeforeCounts $rollbackCounts
    Assert-SameModelContract $script:BeforeModels (Get-ModelContract)
    Assert-SameProfileContract $script:BeforeProfiles (Get-ProfileContract)
    Assert-SameVolumeContract $script:BeforeVolumes (Get-RequiredVolumeContract)
    Write-RetainedCounts $rollbackCounts 'RollbackRetained'
    Write-Host 'NX1Activation=FAILED_ROLLED_BACK' -ForegroundColor Yellow
    Write-Host 'ProtectedServicesPreserved=true'
    Write-Host 'RetainedDataMutated=false'
    Write-Host 'ModelsChanged=false'
    Write-Host 'ProfilesChanged=false'
    Write-Host 'NamedVolumesPreserved=true'
    Write-Host 'DatabaseMigration=false'
}

try {
    Set-Location -LiteralPath $RepositoryRoot
    if (-not (Test-Path -LiteralPath $ReportDirectory -PathType Container)) {
        New-Item -ItemType Directory -Path $ReportDirectory -Force | Out-Null
    }
    $outputPath = if ($PreflightOnly) { $PreflightOutputPath } else { $ActivationOutputPath }
    Start-Transcript -Path $outputPath -Force | Out-Null
    $script:TranscriptStarted = $true

    Write-Section '1 REPOSITORY AND DOCKER DESKTOP'
    Write-Host "Repository=$((Get-Location).Path)"
    Write-Host "PowerShellVersion=$($PSVersionTable.PSVersion)"
    Invoke-External 'docker' @('version') 'Docker client and engine readiness'
    $dockerOs = Invoke-ExternalText 'docker' @('info', '--format', '{{.OSType}}') 'Docker engine OS check'
    if ($dockerOs -cne 'linux') { throw "Expected Docker Desktop Linux engine, got '$dockerOs'." }
    Write-Host 'DockerLinuxEngine=ready'
    Assert-ExpectedSourceState

    Write-Section '2 CURRENT RUNTIME AND BEFORE STATE'
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

    Write-Section '3 RETAINED DATA, MODELS, PROFILES, MOUNTS, AND VOLUMES'
    $script:BeforeCounts = Get-RetainedCounts $script:BeforeIds.postgres
    Write-RetainedCounts $script:BeforeCounts 'BeforeRetained'
    if ([long]$script:BeforeCounts.active_jobs -ne 0) { throw "Active forensic processing jobs=$($script:BeforeCounts.active_jobs); no Docker mutation is allowed." }
    Write-Host 'ActiveForensicProcessingJobs=0'
    $script:BeforeModels = Get-ModelContract
    Write-Host "BeforeModels.Count=$($script:BeforeModels.count)"
    Write-Host "BeforeModels.SHA256=$($script:BeforeModels.sha256)"
    $script:BeforeProfiles = Get-ProfileContract
    foreach ($file in $script:BeforeProfiles.Keys) { Write-Host "BeforeProfileHash.$file=$($script:BeforeProfiles[$file])" }
    $script:BeforeMounts = [ordered]@{
        localai = Get-ContainerMountContract $script:BeforeIds.localai
        forensic_api = Get-ContainerMountContract $script:BeforeIds.forensic_api
        worker = Get-ContainerMountContract $script:BeforeIds.worker
        postgres = Get-ContainerMountContract $script:BeforeIds.postgres
        nats = Get-ContainerMountContract $script:BeforeIds.nats
    }
    $script:BeforeVolumes = Get-RequiredVolumeContract
    if (-not (Test-Path -LiteralPath $MediaModelDirectory -PathType Container)) { throw "Existing media model bind directory is missing: $MediaModelDirectory" }
    Write-Host 'ModelsProfilesVolumes=CAPTURED'

    Write-Section '4 DISK AND MANDATORY PHYSICAL RAM GATE'
    Write-DiskState
    Assert-FreePhysicalMemory

    Write-Section '5 ROLLBACK PLAN'
    Write-Host "LocalAIRollbackPlanned=$LocalAIRollbackTag $($script:BeforeImages.localai)"
    Write-Host "ForensicApiRollbackPlanned=$ForensicApiRollbackTag $($script:BeforeImages.forensic_api)"

    if ($PreflightOnly) {
        Write-Host 'NX1Preflight=PASS' -ForegroundColor Green
        Write-Host 'NX1Activation=NOT_STARTED_PREFLIGHT_ONLY'
        Write-Host 'ExactRebuildServices=forensic-records-api,api'
        Write-Host 'RuntimeServiceMutation=false'
        Write-Host 'RetainedDataMutated=false'
    } else {
        Write-Section '6 CREATE AND VERIFY FRESH ROLLBACK TAGS'
        Ensure-RollbackTag $LocalAIRollbackTag $script:BeforeImages.localai 'LocalAI'
        Ensure-RollbackTag $ForensicApiRollbackTag $script:BeforeImages.forensic_api 'ForensicApi'
        Assert-RollbackTagsExist

        Write-Section '7 SOURCE TESTS AND TWO CACHED BUILDS'
        Invoke-External 'go' @('test', './core/services/agents', '-run', '^Test(DeterministicForensicRouteCoverage|GroupedVideoUUIDIsNotExtractedAsPlateParameter|GroupedVideoExplicitPlateRemainsSeparateFromEvidenceUUID|NaturalGroupedVideoRoutingPreservesTypedOperationParameters)$', '-count=1') 'Focused UUID, plate, time-range, and scope routing tests'
        Invoke-External 'go' @('test', './core/services/agents', '-run', '^TestAgents$', '-count=1', '-args', '--ginkgo.focus=preserves public row count and result state through retained History reopen') 'Focused retained History contract test'
        Invoke-External 'go' @('test', './api/forensic_records', '-run', '^TestMMV2', '-count=1') 'Focused positive/zero/no-match API tests'
        Invoke-External 'go' @('vet', './api/forensic_records', './core/services/agents') 'Changed-package vet checks'
        Assert-FreePhysicalMemory
        if ($script:RuntimeImageTags.forensic_api -cne $script:BeforeImages.forensic_api) {
            Write-Host "ForensicApiBuild=REUSING_INTERRUPTED_BUILD $($script:RuntimeImageTags.forensic_api)"
        } else {
            Invoke-External 'docker' (Get-SidecarComposeArguments @('--progress', 'plain', 'build', 'forensic-records-api')) 'Build only forensic-records-api with Docker cache'
        }
        Assert-FreePhysicalMemory
        if ($script:RuntimeImageTags.localai -cne $script:BeforeImages.localai) {
            Write-Host "LocalAIBuild=REUSING_INTERRUPTED_BUILD $($script:RuntimeImageTags.localai)"
        } else {
            Invoke-External 'docker' (Get-LocalAIComposeArguments @('--progress', 'plain', 'build', 'api')) 'Build only LocalAI/UI api with Docker cache'
        }
        $builtApiId = Invoke-ExternalText 'docker' @('image', 'inspect', $ForensicApiImage, '--format', '{{.Id}}') 'Inspect built forensic API image'
        $builtLocalAIId = Invoke-ExternalText 'docker' @('image', 'inspect', $LocalAIImage, '--format', '{{.Id}}') 'Inspect built LocalAI image'
        if ($builtApiId -ceq $script:BeforeImages.forensic_api) { throw 'Forensic API build did not produce a new image ID.' }
        if ($builtLocalAIId -ceq $script:BeforeImages.localai) { throw 'LocalAI/UI build did not produce a new image ID.' }
        Write-Host "BuiltForensicApiImage=$builtApiId"
        Write-Host "BuiltLocalAIImage=$builtLocalAIId"
        $apiEntrypoint = Invoke-ExternalText 'docker' @('image', 'inspect', $ForensicApiImage, '--format', '{{json .Config.Entrypoint}}') 'Inspect forensic API image entrypoint'
        if ($apiEntrypoint -cne '["/forensic-records-api"]') { throw "Unexpected forensic API entrypoint: $apiEntrypoint" }
        Invoke-External 'docker' @('run', '--rm', '--network', 'none', '--entrypoint', '/local-ai', $LocalAIImage, '--version') 'Built LocalAI binary smoke'
        Write-Host 'PreReplacementSmokeChecks=PASS'

        $preReplaceIds = Get-AllServiceIds
        Assert-ProtectedIdsUnchanged $preReplaceIds
        if ($preReplaceIds.localai -cne $script:BeforeIds.localai) { throw 'LocalAI/UI container changed during build; refusing recreation.' }
        if ($preReplaceIds.forensic_api -cne $script:BeforeIds.forensic_api) { throw 'Forensic API container changed during build; refusing recreation.' }
        $preReplaceCounts = Get-RetainedCounts $preReplaceIds.postgres
        Assert-SameRetainedCounts $script:BeforeCounts $preReplaceCounts
        if ([long]$preReplaceCounts.active_jobs -ne 0) { throw "Active jobs=$($preReplaceCounts.active_jobs); no service recreation was performed." }

        Write-Section '8 RECREATE ONLY FORENSIC API AND LOCALAI/UI'
        $script:RecreationStarted = $true
        Invoke-External 'docker' (Get-SidecarComposeArguments @('up', '-d', '--no-deps', '--force-recreate', 'forensic-records-api')) 'Recreate only forensic-records-api'
        Wait-HttpOk 'http://localhost:8091/healthz' 'ForensicApiHealthz'
        Invoke-External 'docker' (Get-LocalAIComposeArguments @('up', '-d', '--no-deps', '--force-recreate', 'api')) 'Recreate only LocalAI/UI api'

        Write-Section '9 HEALTH AND PRESERVATION VERIFICATION'
        $afterIds = Get-AllServiceIds
        Assert-ProtectedIdsUnchanged $afterIds
        if ($afterIds.localai -ceq $script:BeforeIds.localai) { throw 'LocalAI/UI container was not recreated.' }
        if ($afterIds.forensic_api -ceq $script:BeforeIds.forensic_api) { throw 'Forensic API container was not recreated.' }
        Assert-AllRuntimeHealth $afterIds
        $afterImages = Get-AllServiceImages $afterIds
        if ($afterImages.forensic_api -cne $builtApiId) { throw "Active forensic API image $($afterImages.forensic_api) does not match built image $builtApiId." }
        if ($afterImages.localai -cne $builtLocalAIId) { throw "Active LocalAI image $($afterImages.localai) does not match built image $builtLocalAIId." }
        Assert-SameMount $script:BeforeMounts.localai (Get-ContainerMountContract $afterIds.localai) 'LocalAI'
        Assert-SameMount $script:BeforeMounts.forensic_api (Get-ContainerMountContract $afterIds.forensic_api) 'Forensic API'
        Assert-SameMount $script:BeforeMounts.worker (Get-ContainerMountContract $afterIds.worker) 'Worker'
        Assert-SameMount $script:BeforeMounts.postgres (Get-ContainerMountContract $afterIds.postgres) 'PostgreSQL'
        Assert-SameMount $script:BeforeMounts.nats (Get-ContainerMountContract $afterIds.nats) 'NATS'
        Assert-SameVolumeContract $script:BeforeVolumes (Get-RequiredVolumeContract)
        Assert-SameProfileContract $script:BeforeProfiles (Get-ProfileContract)
        Assert-SameModelContract $script:BeforeModels (Get-ModelContract)
        $afterCounts = Get-RetainedCounts $afterIds.postgres
        Write-RetainedCounts $afterCounts 'AfterRetained'
        Assert-SameRetainedCounts $script:BeforeCounts $afterCounts

        Write-Section '10 FINAL EVIDENCE AND SUCCESS MARKERS'
        Invoke-DisplayOnly 'docker' @('ps', '--no-trunc') 'docker ps'
        Invoke-DisplayOnly 'docker' @('image', 'inspect', $LocalAIImage, $ForensicApiImage, $WorkerImage, $LocalAIRollbackTag, $ForensicApiRollbackTag, '--format', '{{.Id}}|{{json .RepoTags}}') 'Relevant image tags'
        foreach ($key in $afterIds.Keys) {
            Write-Host "ContainerBefore.$key=$($script:BeforeIds[$key])"
            Write-Host "ContainerAfter.$key=$($afterIds[$key])"
            Write-Host "FinalImage.$key=$($afterImages[$key])"
        }
        Write-Host 'NX1Activation=PASS' -ForegroundColor Green
        Write-Host 'NX1ServicesChanged=forensic-records-api,api'
        Write-Host 'ProtectedServicesPreserved=true'
        Write-Host 'RetainedDataMutated=false'
        Write-Host 'ModelsChanged=false'
        Write-Host 'ProfilesChanged=false'
        Write-Host 'NamedVolumesPreserved=true'
        Write-Host 'DatabaseMigration=false'
        Write-Host 'NX1LiveCertification=REQUIRED_AFTER_CODEX_REOPENS'
    }
}
catch {
    $failure = $_.Exception.Message
    Write-Host "ERROR: $failure" -ForegroundColor Red
    $script:ExitCode = 1
    if ($script:RamGateFailed -and -not $script:RecreationStarted) {
        Write-Host 'NX1Activation=NOT_STARTED_RAM_GATE' -ForegroundColor Yellow
        Write-Host 'RuntimeServiceMutation=false'
    } elseif (-not $PreflightOnly -and $script:RecreationStarted -and $null -ne $script:BeforeIds) {
        try { Invoke-GuardedRollback $failure }
        catch {
            Write-Host "AUTOMATIC ROLLBACK FAILED: $($_.Exception.Message)" -ForegroundColor Red
            Write-Host 'NX1Activation=FAILED_ROLLBACK_INCOMPLETE' -ForegroundColor Red
        }
    } elseif (-not $PreflightOnly) {
        Write-Host 'NX1Activation=FAILED_NO_RECREATION' -ForegroundColor Yellow
        Write-Host 'RuntimeServiceMutation=false'
    } else {
        Write-Host 'NX1Preflight=FAIL' -ForegroundColor Red
        Write-Host 'RuntimeServiceMutation=false'
    }
}
finally {
    if ($script:TranscriptStarted) {
        try { Stop-Transcript | Out-Null } catch { }
    }
}

exit $script:ExitCode
