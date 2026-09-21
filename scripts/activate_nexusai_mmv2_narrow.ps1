[CmdletBinding()]
param(
    [switch]$PreflightOnly
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

$RepositoryRoot = 'C:\Users\sheik\Workspace\Office\Projects\NexusAI'
$RequiredFreeGiB = 6.0
$ComposeProject = 'nexusai'
$RuntimeEnvFile = '.env.forensic-runtime.local'
$PreflightPath = 'reports\mmv2-anpr-image-video-20260824\activation-preflight-20260824.json'
$ReportDirectory = 'reports\mmv2-anpr-image-video-20260824'
$LocalAIImage = 'nexusai/localai-forensic:phase3-runtime'
$WorkerImage = 'nexusai/forensic-records-worker:phase3-runtime'
$ForensicApiImage = 'nexusai/forensic-records-api:phase3-runtime'
$LocalAIRollbackTag = 'nexusai/localai-forensic:rollback-before-mmv2-activation-20260824'
$WorkerRollbackTag = 'nexusai/forensic-records-worker:rollback-before-mmv2-activation-20260824'
$ExpectedLocalAIRollbackId = 'sha256:afc6ac795e0c16eca293c765c23caf08fa768b961c774409324664ad320fd7b6'
$ExpectedWorkerRollbackId = 'sha256:4b345f8f3a503a9e54c32a07089a2313b19a368ab4120edb610648d592f8764f'
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

$ExpectedHashes = [ordered]@{
    'ingestion\forensic_records\media_pipeline.py' = '76abd76a8d6cfc1322d90088a4705010920e7338bbc277c7a857d3e9bc8a8c4e'
    'ingestion\forensic_records\worker.py' = '05ed93fc366662bb8c12c6b4dc32130d363d7a46a6f68197335d85ed81a735e0'
    'ingestion\forensic_records\image_intelligence.py' = 'db4f29c555cfa690c3881bffafbae78bd6365108eebdc8637dcb91892858ab6a'
    'ingestion\forensic_records\forensic_contracts.py' = 'a94e67076b70e454c6f43aea0d9189531634bcd1cd3f77aac7c6cd6674e092e3'
    'core\services\agents\forensic_direct.go' = '50f875377ddc0cb68f90c5596cc847a41838ebbbb5a2cf700383fde4caf4cb6e'
    'core\services\agents\forensic_presentation.go' = 'e06db4575b46d4695b084ed72862ba241212b4742397df17363cd8465b7dc0d2'
    'core\services\agents\records_tools.go' = 'c100a51c0acca03fdc1e11c29a102c83d2d561fca4948bcbdfc0045c6db1838b'
    'core\services\agents\analysis_history.go' = '180f0fbf87f41d875ad5f2e58ed04edf684ca50e5de92c5c7cb4ade8cfff0055'
    'core\services\agents\config.go' = '0b2d596349c973b5369ee7f24c3d91e9f25f43c24d1cb384f58df44dcecee471'
    'core\services\agents\dispatcher.go' = '2e8fadf092eafdf4b73d8bbef11e338d9d84b71d4bd7500e8d576a2da5bae59c'
    'core\services\agents\executor.go' = 'ef540d16ada46e20626169c25a102d9fdc79adeea8b540a7cb8b4f62df4aba64'
    'core\services\agents\store.go' = '2ae779b6328942fb28ee6ea956824f100b8ebdcb28a461ceb8afb7cd91d57042'
    'core\http\app.go' = '9c82c6191320d30dc606a5bd5d419df10ac4dbc5906e0f1f452e521b1412cadd'
    'core\http\endpoints\localai\forensic_cases.go' = '45624de127f110bd7a6678d40a3e47c92727465d498281927a40d10ef404c91a'
    'core\http\endpoints\localai\agent_history.go' = '7b3d4c2571fc541c4e634a8a93560c9ccbf60d9e9222dff6b39828d0e8f1ae7a'
    'core\http\endpoints\localai\agents.go' = '34c481706dd6af47ec36a75d020c80ca9e0b5440cba1d00d474ac7b3ceb8f00a'
    'core\http\routes\localai.go' = 'a963a2145bbb68377db79858371948f712f9636fab040af6dd263402dba81149'
    'core\http\routes\agents.go' = '3e031a5cf983342671ac69963c7da1723dd16c0abb6ff49270fa3dae98e63122'
    'core\http\routes\records.go' = 'e399e041b448e8bd4edcc997d624655597856af568a68bb1e6800657b0c2e2dc'
    'core\schema\localai.go' = 'f9ea5546870a599b6879107ccccb2828e687811d0d57500722fb0767ce5c83c3'
    'core\http\react-ui\src\App.jsx' = '29ee6c96150413ab64aa1ce17216a21ce8dc51c5b1a721839f3b1c6acf2c74df'
    'core\http\react-ui\src\router.jsx' = 'd5bc080c362fab3bcece1fd38dec74a9c5d743579655a5f033b57039e9532883'
    'core\http\react-ui\src\utils\api.js' = '53bfc7b027134567d985b5125a2b9ecb3110eaa3f133e40a327f5c68d679886d'
    'core\http\react-ui\src\pages\CaseWorkspace.jsx' = '00d3a54df9a45b1c34af288df14bd5140b88967796c8a428b2c95fe629212711'
    'core\http\react-ui\src\components\evidence-workspace\EvidenceWorkspace.jsx' = '9987ac54290a8d5f5b66bfd3f26e19ad5ec0f8f4bfd39b0cf45422a2769ea124'
    'core\http\react-ui\src\analyst\AnalystAddData.jsx' = '1fb5826e4af5bffb27de9c187b6556418364af6dbfc020bc33e1657e92dc5bb7'
    'core\http\react-ui\src\analyst\AnalystAsk.jsx' = 'fc1d6b1f041612def796927eb3dcae49448e1286064c5b48044a2f4d41386626'
    'core\http\react-ui\src\analyst\AnalystData.jsx' = 'e9b7edd7de4e0b03e147383b2dddcfc8254eb01802404977092ff595eab3c390'
    'core\http\react-ui\src\analyst\AnalystHistory.jsx' = 'c4f3ab7c15da43a582fbec16926f61b39fc21263b8521e72dde170bf145276bf'
    'core\http\react-ui\src\analyst\AnalystHome.jsx' = '2600dfff72b3491258bb4546037caea8263409dd5739614347bbc90663cc8552'
    'core\http\react-ui\src\analyst\analystMediaPresentation.js' = '556d06fa54525a83d2118446f1cc124c32447f85ce77a29d242d34f29119ffdc'
    'core\http\react-ui\src\analyst\analystMediaPresentation.test.js' = '9a430415056f9f95ceb54bd931c47d940a41cf6ffa9c09b697593ff5804a67de'
    'core\http\react-ui\src\analyst\AnalystPortal.css' = 'ef413bd82b7cbcb4a68a1b951dd325118ea6a940d6d14caf5607b9173a1449ac'
    'core\http\react-ui\src\analyst\AnalystPortalLayout.jsx' = 'f57228c92696913fc780866fdbfc8330a78d434ba87982ebcf96fb06b4c57538'
    'core\http\react-ui\src\analyst\analystPresentation.js' = '972de507b34a2321162c273a86bb566f3c03312fa795ea64d8870394abf3ab87'
    'ingestion\forensic_records\Dockerfile' = '6d77c22c44c1bcc09753fcacd3da097845d46db5334e08ef41efdd39b28164d9'
    'Dockerfile' = '167f93165728a31b554751c2a100120eca34ffdcc8509ecb3ac5e393efe48a94'
    'docker-compose.forensic-records.yaml' = 'b0f724ea34bea80bfadf8491d03bf2ddc9a8cfcbc0f2982d1a5374b968e75dd6'
    'docker-compose.forensic-records.runtime.yaml' = '8793dff4a5980dc0d8dd6a453967cb6512daf13301f22039c770ab9d4410d923'
    'docker-compose.forensic-records.asr-small.yaml' = '4f39dfe6d572d1ffcddbe81339389167534f4b0f8b20e1d5915b0532827c11f0'
    'docker-compose.yaml' = '0ef74ce958237dbe66552dcb3e36a02def568eb1a179bcd04828cf54af4dc72d'
    'docker-compose.forensic-runtime.localai.yaml' = 'a4a10aca68a9291e9696d86f392a33e53df64992a31cca896c7c99492ce5ec5b'
    '.env' = 'e6e974b1c197e56722c03fd7092acaeb1ef9a98e37aff8197bb88904007c024d'
    '.env.forensic-runtime.local' = '82bb4660ef9615f821f3ef3157652054d6b338fa5324cb894f53ea6e9e5de726'
    'reports\mmv2-anpr-image-video-20260824\activation-preflight-20260824.json' = '96962362d878dfa1c76e9714c119903af9e8f6f7d4d9bc77ab9984b502c87710'
}

$ExpectedPreActivationShortIds = [ordered]@{
    localai = 'd4207db67d15'
    forensic_api = 'a97e545b8356'
    worker = '4aaf9b56ca22'
    nats = '5c49e70d132a'
    postgres = 'f66e05a3b179'
}

$ServiceNames = [ordered]@{
    localai = 'api'
    forensic_api = 'forensic-records-api'
    worker = 'forensic-records-worker'
    nats = 'forensic-nats'
    postgres = 'forensic-postgres'
}

$script:TranscriptStarted = $false
$script:RecreationStarted = $false
$script:ServicesStopped = $false
$script:BeforeIds = $null
$script:BeforeCounts = $null
$script:BeforeMounts = $null
$script:PgUser = $null
$script:PgDatabase = $null
$script:HistoryDatabaseUrl = $null
$script:TranscriptPath = $null

function Write-Section([string]$Title) {
    Write-Host ''
    Write-Host "=== $Title ===" -ForegroundColor Cyan
}

function Invoke-External {
    param(
        [Parameter(Mandatory)][string]$FilePath,
        [Parameter(Mandatory)][string[]]$ArgumentList,
        [Parameter(Mandatory)][string]$Description
    )
    & $FilePath @ArgumentList
    if ($LASTEXITCODE -ne 0) {
        throw "$Description failed with exit code $LASTEXITCODE."
    }
}

function Invoke-ExternalText {
    param(
        [Parameter(Mandatory)][string]$FilePath,
        [Parameter(Mandatory)][string[]]$ArgumentList,
        [Parameter(Mandatory)][string]$Description
    )
    $output = & $FilePath @ArgumentList 2>&1
    if ($LASTEXITCODE -ne 0) {
        throw "$Description failed with exit code $LASTEXITCODE.`n$($output -join [Environment]::NewLine)"
    }
    return ($output -join "`n").Trim()
}

function Get-FreePhysicalMemoryGiB {
    $os = Get-CimInstance -ClassName Win32_OperatingSystem
    return [math]::Round(([double]$os.FreePhysicalMemory / 1MB), 2)
}

function Assert-FreePhysicalMemory {
    $freeGiB = Get-FreePhysicalMemoryGiB
    Write-Host ("FreePhysicalMemoryGiB={0:N2}" -f $freeGiB)
    Write-Host ("RequiredFreePhysicalMemoryGiB={0:N2}" -f $RequiredFreeGiB)
    if ($freeGiB -lt $RequiredFreeGiB) {
        throw ("MMV2NarrowActivation=SAFE_STOP_RAM_GATE Free physical RAM {0:N2} GiB is below mandatory {1:N2} GiB; no mutation was performed." -f $freeGiB, $RequiredFreeGiB)
    }
}

function Wait-FreePhysicalMemoryAfterStop([int]$TimeoutSeconds = 45) {
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    $bestGiB = 0.0
    do {
        $freeGiB = Get-FreePhysicalMemoryGiB
        if ($freeGiB -gt $bestGiB) { $bestGiB = $freeGiB }
        Write-Host ("PostStopFreePhysicalMemoryGiB={0:N2}" -f $freeGiB)
        if ($freeGiB -ge $RequiredFreeGiB) { return }
        Start-Sleep -Seconds 3
    } while ((Get-Date) -lt $deadline)
    throw ("Post-stop RAM guard failed: best observed {0:N2} GiB is below {1:N2} GiB." -f $bestGiB, $RequiredFreeGiB)
}

function Get-ComposeContainerId([string]$Service) {
    $id = Invoke-ExternalText -FilePath 'docker' -ArgumentList @(
        'ps', '-q',
        '--filter', "label=com.docker.compose.project=$ComposeProject",
        '--filter', "label=com.docker.compose.service=$Service"
    ) -Description "Resolve running container for $Service"
    $ids = @($id -split "`n" | Where-Object { $_.Trim() })
    if ($ids.Count -ne 1) {
        throw "Expected exactly one running container for compose service '$Service'; found $($ids.Count)."
    }
    return $ids[0].Trim()
}

function Get-ProtectedContainerIds {
    $ids = [ordered]@{}
    foreach ($key in $ServiceNames.Keys) {
        $ids[$key] = Get-ComposeContainerId -Service $ServiceNames[$key]
    }
    return $ids
}

function Assert-ExpectedPreActivationIds([System.Collections.IDictionary]$Ids) {
    foreach ($key in $ExpectedPreActivationShortIds.Keys) {
        if (-not $Ids[$key].StartsWith($ExpectedPreActivationShortIds[$key], [System.StringComparison]::OrdinalIgnoreCase)) {
            throw "Container '$key' is $($Ids[$key]); expected the verified preflight container starting with $($ExpectedPreActivationShortIds[$key])."
        }
    }
}

function Get-ContainerEnvironmentValue([string]$ContainerId, [string]$Name) {
    # Windows PowerShell 5.1 returns a JSON array from ConvertFrom-Json as one
    # nested Object[] instead of enumerating its string entries. Ask Docker for
    # one entry per line so secret values remain captured but are never printed.
    $text = Invoke-ExternalText -FilePath 'docker' -ArgumentList @('inspect', $ContainerId, '--format', '{{range .Config.Env}}{{println .}}{{end}}') -Description "Inspect environment for $ContainerId"
    $items = @($text -split "`r?`n" | Where-Object { $_ })
    $prefix = "$Name="
    $match = @($items | Where-Object { $_.StartsWith($prefix, [System.StringComparison]::Ordinal) })
    if ($match.Count -ne 1) {
        throw "Expected exactly one '$Name' environment entry in container $ContainerId."
    }
    return $match[0].Substring($prefix.Length)
}

function ConvertTo-ComparableMountSource([string]$Type, [string]$Source) {
    if ($Type -eq 'volume') {
        # The named volume is identified by Name; its engine-internal source
        # path is an implementation detail and is not a preservation boundary.
        return ''
    }
    $normalized = $Source -replace '\\', '/'
    if ($normalized -match '^/run/desktop/mnt/host/([A-Za-z])/(.*)$') {
        $normalized = "$($Matches[1]):/$($Matches[2])"
    }
    elseif ($normalized -match '^/host_mnt/([A-Za-z])/(.*)$') {
        $normalized = "$($Matches[1]):/$($Matches[2])"
    }
    return $normalized.TrimEnd('/').ToLowerInvariant()
}

function Get-ContainerMountContract([string]$ContainerId) {
    $json = Invoke-ExternalText -FilePath 'docker' -ArgumentList @('inspect', $ContainerId, '--format', '{{json .Mounts}}') -Description "Inspect mounts for $ContainerId"
    $parsedMounts = $json | ConvertFrom-Json
    $mounts = @()
    foreach ($mount in $parsedMounts) {
        $mounts += $mount
    }
    return @($mounts | ForEach-Object {
        $mountName = if ($null -ne $_.PSObject.Properties['Name']) { [string]$_.Name } else { '' }
        $rawMountSource = if ($null -ne $_.PSObject.Properties['Source']) { [string]$_.Source } else { '' }
        $mountSource = ConvertTo-ComparableMountSource -Type ([string]$_.Type) -Source $rawMountSource
        [pscustomobject]@{
            Destination = [string]$_.Destination
            Type = [string]$_.Type
            Name = $mountName
            Source = $mountSource
            RW = [bool]$_.RW
        }
    } | Sort-Object Destination | ConvertTo-Json -Compress)
}

function Assert-SameMountContract([string]$Expected, [string]$Actual, [string]$Name) {
    if ($Expected -cne $Actual) {
        throw "$Name mount contract changed unexpectedly.`nBefore=$Expected`nAfter=$Actual"
    }
}

function Wait-HttpOk {
    param(
        [Parameter(Mandatory)][string]$Uri,
        [Parameter(Mandatory)][string]$Name,
        [int]$TimeoutSeconds = 180,
        [hashtable]$Headers = @{}
    )
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        try {
            $response = Invoke-WebRequest -UseBasicParsing -Uri $Uri -Headers $Headers -TimeoutSec 10
            if ([int]$response.StatusCode -eq 200) {
                Write-Host "$Name=200"
                return
            }
        }
        catch {
            Start-Sleep -Seconds 2
        }
    } while ((Get-Date) -lt $deadline)
    throw "$Name did not return HTTP 200 within $TimeoutSeconds seconds: $Uri"
}

function Wait-ContainerHealthy([string]$ContainerId, [string]$Name, [int]$TimeoutSeconds = 180) {
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        $status = Invoke-ExternalText -FilePath 'docker' -ArgumentList @('inspect', $ContainerId, '--format', '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}') -Description "Inspect health for $Name"
        if ($status -eq 'healthy' -or $status -eq 'running') {
            Write-Host "$Name=$status"
            return
        }
        Start-Sleep -Seconds 2
    } while ((Get-Date) -lt $deadline)
    throw "$Name did not become healthy/running within $TimeoutSeconds seconds; last status was '$status'."
}

function Assert-PostgresReady([string]$ContainerId) {
    Invoke-External -FilePath 'docker' -ArgumentList @('exec', $ContainerId, 'pg_isready', '-U', $script:PgUser, '-d', $script:PgDatabase) -Description 'PostgreSQL readiness'
    Write-Host 'PostgreSQL=ready'
}

function Get-RetainedCounts([string]$PostgresContainerId) {
    $sql = "SELECT (SELECT count(*) FROM forensic.evidence_items),(SELECT count(*) FROM forensic.evidence_versions),(SELECT count(*) FROM forensic.records_ingest_jobs),(SELECT count(*) FROM forensic.records_ingest_jobs WHERE status IN ('queued','running')),(SELECT count(*) FROM forensic.records),(SELECT count(*) FROM forensic.derived_artifacts),(SELECT count(*) FROM forensic.kb_collection_assets);"
    $line = Invoke-ExternalText -FilePath 'docker' -ArgumentList @('exec', $PostgresContainerId, 'psql', '-U', $script:PgUser, '-d', $script:PgDatabase, '-At', '-F', '|', '-c', $sql) -Description 'Read retained counts'
    $parts = @($line.Trim() -split '\|')
    if ($parts.Count -ne 7) {
        throw "Unexpected retained-count result: '$line'"
    }
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
    foreach ($key in $Counts.Keys) {
        Write-Host "$Prefix.$key=$($Counts[$key])"
    }
}

function Assert-SameRetainedCounts([System.Collections.IDictionary]$Before, [System.Collections.IDictionary]$After) {
    foreach ($key in $Before.Keys) {
        if ([long]$Before[$key] -ne [long]$After[$key]) {
            throw "Unauthorized retained-data mutation: '$key' changed from $($Before[$key]) to $($After[$key])."
        }
    }
}

function Assert-ExpectedFileState {
    foreach ($entry in $ExpectedHashes.GetEnumerator()) {
        if (-not (Test-Path -LiteralPath $entry.Key -PathType Leaf)) {
            throw "Required activation input is missing: $($entry.Key)"
        }
        $actual = (Get-FileHash -Algorithm SHA256 -LiteralPath $entry.Key).Hash.ToLowerInvariant()
        if ($actual -cne $entry.Value) {
            throw "Checkpoint mismatch for '$($entry.Key)': expected $($entry.Value), got $actual."
        }
        Write-Host "SHA256 $actual  $($entry.Key)"
    }

    $gitDir = Invoke-ExternalText -FilePath 'git' -ArgumentList @('rev-parse', '--git-dir') -Description 'Resolve Git directory'
    foreach ($marker in @('MERGE_HEAD', 'CHERRY_PICK_HEAD', 'REVERT_HEAD')) {
        if (Test-Path -LiteralPath (Join-Path $gitDir $marker)) {
            throw "Unsafe in-progress Git operation detected: $marker"
        }
    }

    $preflight = Get-Content -LiteralPath $PreflightPath -Raw | ConvertFrom-Json
    if ($preflight.contract_version -ne 'nexusai.mmv2.activation-preflight/v1') {
        throw "Unexpected activation preflight contract: $($preflight.contract_version)"
    }
    if ($preflight.verdict -ne 'SAFE_STOP_RAM_GATE_NOT_MET_NO_BUILD') {
        throw "Unexpected prior preflight verdict: $($preflight.verdict)"
    }
    if ([bool]$preflight.forensic_api_rebuild_needed) {
        throw 'Preflight unexpectedly requires a forensic API rebuild.'
    }
    $scope = @($preflight.rebuild_scope_after_source_diff_reverification)
    if (($scope -join ',') -ne 'forensic-records-worker,localai-ui') {
        throw "Unexpected MMV-2 rebuild scope: $($scope -join ',')"
    }
    if ($preflight.rollback_images.localai.tag -ne $LocalAIRollbackTag -or $preflight.rollback_images.localai.image_id -ne $ExpectedLocalAIRollbackId) {
        throw 'LocalAI rollback image in checkpoint does not match the guarded procedure.'
    }
    if ($preflight.rollback_images.worker.tag -ne $WorkerRollbackTag -or $preflight.rollback_images.worker.image_id -ne $ExpectedWorkerRollbackId) {
        throw 'Worker rollback image in checkpoint does not match the guarded procedure.'
    }
    Write-Host 'CheckpointState=PASS'
    Write-Host 'ExactRebuildServices=forensic-records-worker,api'
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
    Invoke-External -FilePath 'docker' -ArgumentList (Get-SidecarComposeArguments @('config', '--quiet')) -Description 'Forensic sidecar compose validation'
    Invoke-External -FilePath 'docker' -ArgumentList (Get-LocalAIComposeArguments @('config', '--quiet')) -Description 'LocalAI compose validation'
    Write-Host 'ComposeConfiguration=PASS'
}

function Assert-RollbackImages {
    $localId = Invoke-ExternalText -FilePath 'docker' -ArgumentList @('image', 'inspect', $LocalAIRollbackTag, '--format', '{{.Id}}') -Description 'Inspect LocalAI rollback image'
    $workerId = Invoke-ExternalText -FilePath 'docker' -ArgumentList @('image', 'inspect', $WorkerRollbackTag, '--format', '{{.Id}}') -Description 'Inspect worker rollback image'
    if ($localId -ne $ExpectedLocalAIRollbackId) { throw "LocalAI rollback ID mismatch: $localId" }
    if ($workerId -ne $ExpectedWorkerRollbackId) { throw "Worker rollback ID mismatch: $workerId" }
    Write-Host "LocalAIRollback=$LocalAIRollbackTag $localId"
    Write-Host "WorkerRollback=$WorkerRollbackTag $workerId"
    Write-Host 'RollbackImages=PASS'
}

function Assert-AllRuntimeHealth {
    param(
        [Parameter(Mandatory)][System.Collections.IDictionary]$Ids,
        [switch]$RequireAnalystPortalRoute
    )
    Wait-HttpOk -Uri 'http://localhost:8080/readyz' -Name 'LocalAIReadyz'
    Wait-HttpOk -Uri 'http://localhost:8091/healthz' -Name 'ForensicApiHealthz'
    Wait-HttpOk -Uri 'http://localhost:9109/metrics' -Name 'WorkerMetrics'
    Wait-HttpOk -Uri 'http://localhost:8222/healthz' -Name 'NatsHealthz'
    Wait-HttpOk -Uri 'http://localhost:8080/' -Name 'LocalAIUiRootHttp'
    if ($RequireAnalystPortalRoute) {
        Wait-HttpOk -Uri 'http://localhost:8080/analyst/home?case=nexusai-multimodal-product-acceptance' -Name 'AnalystPortalHttp' -Headers @{ Accept = 'text/html' }
    }
    Assert-PostgresReady -ContainerId $Ids.postgres
}

function Start-UnchangedFoundation {
    Invoke-External -FilePath 'docker' -ArgumentList @('start', $script:BeforeIds.postgres, $script:BeforeIds.nats, $script:BeforeIds.forensic_api) -Description 'Start unchanged PostgreSQL, NATS, and forensic API containers'
    Wait-ContainerHealthy -ContainerId $script:BeforeIds.postgres -Name 'PostgreSQLContainer'
    Wait-ContainerHealthy -ContainerId $script:BeforeIds.nats -Name 'NatsContainer'
    Wait-ContainerHealthy -ContainerId $script:BeforeIds.forensic_api -Name 'ForensicApiContainer'
    Wait-HttpOk -Uri 'http://localhost:8222/healthz' -Name 'NatsHealthz'
    Wait-HttpOk -Uri 'http://localhost:8091/healthz' -Name 'ForensicApiHealthz'
    Assert-PostgresReady -ContainerId $script:BeforeIds.postgres
}

function Assert-UnchangedProtectedIds([System.Collections.IDictionary]$AfterIds) {
    foreach ($key in @('postgres', 'nats', 'forensic_api')) {
        if ($AfterIds[$key] -ne $script:BeforeIds[$key]) {
            throw "Unaffected protected container '$key' changed: before=$($script:BeforeIds[$key]) after=$($AfterIds[$key])"
        }
    }
    Write-Host 'ProtectedServicesPreserved=true'
}

function Invoke-GuardedRollback([string]$FailureMessage) {
    Write-Section 'GUARDED ROLLBACK'
    Write-Warning $FailureMessage

    Invoke-External -FilePath 'docker' -ArgumentList @('start', $script:BeforeIds.postgres, $script:BeforeIds.nats, $script:BeforeIds.forensic_api) -Description 'Restore unchanged foundation for rollback'
    Wait-ContainerHealthy -ContainerId $script:BeforeIds.postgres -Name 'RollbackPostgreSQL'
    Wait-ContainerHealthy -ContainerId $script:BeforeIds.nats -Name 'RollbackNATS'
    Wait-ContainerHealthy -ContainerId $script:BeforeIds.forensic_api -Name 'RollbackForensicAPI'

    $env:NEXUSAI_LOCALAI_RUNTIME_IMAGE = $LocalAIRollbackTag
    Invoke-External -FilePath 'docker' -ArgumentList (Get-LocalAIComposeArguments @('up', '-d', '--no-deps', '--force-recreate', 'api')) -Description 'Rollback LocalAI/UI container'
    $env:NEXUSAI_FORENSIC_WORKER_IMAGE = $WorkerRollbackTag
    Invoke-External -FilePath 'docker' -ArgumentList (Get-SidecarComposeArguments @('up', '-d', '--no-deps', '--force-recreate', 'forensic-records-worker')) -Description 'Rollback worker container'

    $rollbackIds = Get-ProtectedContainerIds
    $runningLocalImage = Invoke-ExternalText -FilePath 'docker' -ArgumentList @('inspect', $rollbackIds.localai, '--format', '{{.Image}}') -Description 'Inspect rolled-back LocalAI image'
    $runningWorkerImage = Invoke-ExternalText -FilePath 'docker' -ArgumentList @('inspect', $rollbackIds.worker, '--format', '{{.Image}}') -Description 'Inspect rolled-back worker image'
    if ($runningLocalImage -ne $ExpectedLocalAIRollbackId) { throw "Rollback LocalAI image mismatch: $runningLocalImage" }
    if ($runningWorkerImage -ne $ExpectedWorkerRollbackId) { throw "Rollback worker image mismatch: $runningWorkerImage" }
    Assert-UnchangedProtectedIds -AfterIds $rollbackIds
    Assert-SameMountContract -Expected $script:BeforeMounts.localai -Actual (Get-ContainerMountContract -ContainerId $rollbackIds.localai) -Name 'Rolled-back LocalAI'
    Assert-SameMountContract -Expected $script:BeforeMounts.worker -Actual (Get-ContainerMountContract -ContainerId $rollbackIds.worker) -Name 'Rolled-back worker'
    Assert-AllRuntimeHealth -Ids $rollbackIds
    $rollbackCounts = Get-RetainedCounts -PostgresContainerId $rollbackIds.postgres
    Assert-SameRetainedCounts -Before $script:BeforeCounts -After $rollbackCounts
    Write-RetainedCounts -Counts $rollbackCounts -Prefix 'RollbackRetained'
    Write-Host 'MMV2NarrowActivation=FAILED_ROLLED_BACK' -ForegroundColor Yellow
    Write-Host 'RetainedDataMutated=false'
    Write-Host 'ModelsChanged=false'
    Write-Host 'DatabaseMigration=false'
}

Set-Location -LiteralPath $RepositoryRoot

try {
    Write-Section 'A-B REPOSITORY AND DOCKER READINESS'
    Write-Host "Repository=$((Get-Location).Path)"
    Invoke-External -FilePath 'docker' -ArgumentList @('version') -Description 'Docker client/engine readiness'
    $dockerOs = Invoke-ExternalText -FilePath 'docker' -ArgumentList @('info', '--format', '{{.OSType}}') -Description 'Docker engine OS verification'
    if ($dockerOs -ne 'linux') { throw "Expected Docker Linux engine, got '$dockerOs'." }
    Write-Host 'DockerLinuxEngine=ready'

    Write-Section 'C-D MANDATORY PHYSICAL RAM GATE'
    Assert-FreePhysicalMemory

    if (-not $PreflightOnly) {
        $timestamp = Get-Date -Format 'yyyyMMdd-HHmmss'
        $script:TranscriptPath = Join-Path $ReportDirectory "manual-narrow-activation-$timestamp.log"
        Start-Transcript -LiteralPath $script:TranscriptPath -Force | Out-Null
        $script:TranscriptStarted = $true
        Write-Host "ActivationTranscript=$((Resolve-Path -LiteralPath $script:TranscriptPath).Path)"
        Write-Host 'DockerLinuxEngine=ready'
        Write-Host ("FreePhysicalMemoryGiBAtGate={0:N2}" -f (Get-FreePhysicalMemoryGiB))
        Write-Host ("RequiredFreePhysicalMemoryGiB={0:N2}" -f $RequiredFreeGiB)
    }

    Write-Section 'E SOURCE, WORKTREE, AND CHECKPOINT CONTRACT'
    Assert-ExpectedFileState
    if (-not (Test-Path -LiteralPath $MediaModelDirectory -PathType Container)) {
        throw "Required existing read-only media model directory is missing: $MediaModelDirectory"
    }

    Write-Section 'F-H PROTECTED CONTAINERS, HEALTH, AND ACTIVE JOB GATE'
    $script:BeforeIds = Get-ProtectedContainerIds
    Assert-ExpectedPreActivationIds -Ids $script:BeforeIds
    foreach ($key in $script:BeforeIds.Keys) { Write-Host "BeforeContainer.$key=$($script:BeforeIds[$key])" }

    $script:PgUser = Get-ContainerEnvironmentValue -ContainerId $script:BeforeIds.postgres -Name 'POSTGRES_USER'
    $script:PgDatabase = Get-ContainerEnvironmentValue -ContainerId $script:BeforeIds.postgres -Name 'POSTGRES_DB'
    $script:HistoryDatabaseUrl = Get-ContainerEnvironmentValue -ContainerId $script:BeforeIds.localai -Name 'LOCALAI_AGENT_POOL_DATABASE_URL'
    Set-RequiredComposeEnvironment
    Assert-ComposeConfiguration
    Assert-AllRuntimeHealth -Ids $script:BeforeIds

    Write-Section 'I RETAINED COUNTS BEFORE ACTIVATION'
    $script:BeforeCounts = Get-RetainedCounts -PostgresContainerId $script:BeforeIds.postgres
    Write-RetainedCounts -Counts $script:BeforeCounts -Prefix 'BeforeRetained'
    if ([long]$script:BeforeCounts.active_jobs -ne 0) {
        throw "MMV2NarrowActivation=SAFE_STOP_ACTIVE_JOBS Active forensic ingest jobs=$($script:BeforeCounts.active_jobs); no service recreation was performed."
    }
    Write-Host 'ActiveForensicIngestJobs=0'

    $script:BeforeMounts = [ordered]@{
        localai = Get-ContainerMountContract -ContainerId $script:BeforeIds.localai
        worker = Get-ContainerMountContract -ContainerId $script:BeforeIds.worker
    }

    Write-Section 'J EXISTING ROLLBACK IMAGES'
    Assert-RollbackImages

    if ($PreflightOnly) {
        Write-Host 'MMV2NarrowPreflight=PASS' -ForegroundColor Green
        Write-Host 'ExactRebuildServices=forensic-records-worker,api'
        Write-Host 'MutationPerformed=false'
        exit 0
    }

    Write-Section 'K-L EXACT BUILDS AND PRE-REPLACEMENT CHECKS'
    Invoke-External -FilePath 'python' -ArgumentList @('-c', "import ast,pathlib; ast.parse(pathlib.Path(r'ingestion\forensic_records\media_pipeline.py').read_text(encoding='utf-8'))") -Description 'Worker Python parse smoke'
    Invoke-External -FilePath 'node' -ArgumentList @('--test', 'core\http\react-ui\src\analyst\analystMediaPresentation.test.js') -Description 'Analyst media presentation unit test'

    Invoke-External -FilePath 'docker' -ArgumentList @('stop', $script:BeforeIds.localai, $script:BeforeIds.worker, $script:BeforeIds.forensic_api, $script:BeforeIds.nats, $script:BeforeIds.postgres) -Description 'Reversibly stop exact verified runtime containers for guarded build'
    $script:ServicesStopped = $true

    Wait-FreePhysicalMemoryAfterStop

    Invoke-External -FilePath 'docker' -ArgumentList (Get-SidecarComposeArguments @('--progress', 'plain', 'build', 'forensic-records-worker')) -Description 'Build exact MMV-2 worker service'
    Invoke-External -FilePath 'docker' -ArgumentList (Get-LocalAIComposeArguments @('--progress', 'plain', 'build', 'api')) -Description 'Build exact MMV-2 LocalAI/UI service'

    $builtWorkerId = Invoke-ExternalText -FilePath 'docker' -ArgumentList @('image', 'inspect', $WorkerImage, '--format', '{{.Id}}') -Description 'Inspect built worker image'
    $builtLocalAIId = Invoke-ExternalText -FilePath 'docker' -ArgumentList @('image', 'inspect', $LocalAIImage, '--format', '{{.Id}}') -Description 'Inspect built LocalAI image'
    if ($builtWorkerId -eq $ExpectedWorkerRollbackId) { throw 'Worker build did not produce a new image ID.' }
    if ($builtLocalAIId -eq $ExpectedLocalAIRollbackId) { throw 'LocalAI/UI build did not produce a new image ID.' }
    Write-Host "BuiltWorkerImage=$builtWorkerId"
    Write-Host "BuiltLocalAIImage=$builtLocalAIId"

    $insideWorkerHash = Invoke-ExternalText -FilePath 'docker' -ArgumentList @('run', '--rm', '--network', 'none', '--entrypoint', 'sha256sum', $WorkerImage, '/app/media_pipeline.py') -Description 'Verify worker source inside built image'
    if (-not $insideWorkerHash.StartsWith($ExpectedHashes['ingestion\forensic_records\media_pipeline.py'], [System.StringComparison]::OrdinalIgnoreCase)) {
        throw "Built worker contains unexpected media pipeline: $insideWorkerHash"
    }
    Invoke-External -FilePath 'docker' -ArgumentList @('run', '--rm', '--network', 'none', '--entrypoint', 'python', $WorkerImage, '-c', "import media_pipeline; assert media_pipeline.VIDEO_ANPR_GROUP_CONTRACT == 'forensics.video-anpr-plate-group/v1'") -Description 'Built worker MMV-2 import smoke'
    Invoke-External -FilePath 'docker' -ArgumentList @('run', '--rm', '--network', 'none', '--entrypoint', '/local-ai', $LocalAIImage, '--version') -Description 'Built LocalAI binary smoke'
    Write-Host 'PreReplacementSmokeChecks=PASS'

    Write-Section 'M-O NARROW RECREATION AND HEALTH WAIT'
    Start-UnchangedFoundation

    $script:RecreationStarted = $true
    Invoke-External -FilePath 'docker' -ArgumentList (Get-LocalAIComposeArguments @('up', '-d', '--no-deps', '--force-recreate', 'api')) -Description 'Recreate only LocalAI/UI service'
    Wait-HttpOk -Uri 'http://localhost:8080/readyz' -Name 'LocalAIReadyz'
    Invoke-External -FilePath 'docker' -ArgumentList (Get-SidecarComposeArguments @('up', '-d', '--no-deps', '--force-recreate', 'forensic-records-worker')) -Description 'Recreate only worker service'

    $afterIds = Get-ProtectedContainerIds
    Wait-ContainerHealthy -ContainerId $afterIds.worker -Name 'WorkerContainer'
    Wait-HttpOk -Uri 'http://localhost:9109/metrics' -Name 'WorkerMetrics'

    Write-Section 'P-Q COMPLETE HEALTH AND PROTECTED-ID VERIFICATION'
    Assert-UnchangedProtectedIds -AfterIds $afterIds
    Assert-AllRuntimeHealth -Ids $afterIds -RequireAnalystPortalRoute
    $runningLocalImage = Invoke-ExternalText -FilePath 'docker' -ArgumentList @('inspect', $afterIds.localai, '--format', '{{.Image}}') -Description 'Inspect active LocalAI image'
    $runningWorkerImage = Invoke-ExternalText -FilePath 'docker' -ArgumentList @('inspect', $afterIds.worker, '--format', '{{.Image}}') -Description 'Inspect active worker image'
    if ($runningLocalImage -ne $builtLocalAIId) { throw "Active LocalAI image $runningLocalImage does not equal built image $builtLocalAIId." }
    if ($runningWorkerImage -ne $builtWorkerId) { throw "Active worker image $runningWorkerImage does not equal built image $builtWorkerId." }
    Assert-SameMountContract -Expected $script:BeforeMounts.localai -Actual (Get-ContainerMountContract -ContainerId $afterIds.localai) -Name 'LocalAI'
    Assert-SameMountContract -Expected $script:BeforeMounts.worker -Actual (Get-ContainerMountContract -ContainerId $afterIds.worker) -Name 'Worker'

    Write-Section 'R RETAINED-COUNT RECONCILIATION'
    $afterCounts = Get-RetainedCounts -PostgresContainerId $afterIds.postgres
    Write-RetainedCounts -Counts $afterCounts -Prefix 'AfterRetained'
    Assert-SameRetainedCounts -Before $script:BeforeCounts -After $afterCounts
    Write-Host 'RetainedDataMutated=false'

    Write-Section 'S FINAL RUNTIME EVIDENCE'
    Invoke-External -FilePath 'docker' -ArgumentList @('ps', '--no-trunc') -Description 'Display Docker runtime state'
    # Avoid Go-template string literals here: Windows PowerShell 5.1 strips the
    # nested quotes when forwarding native arguments. Docker's json helper is
    # quote-free at the command boundary and preserves every repository tag.
    Invoke-External -FilePath 'docker' -ArgumentList @('image', 'inspect', $LocalAIImage, $WorkerImage, $ForensicApiImage, $LocalAIRollbackTag, $WorkerRollbackTag, '--format', '{{.Id}}|{{json .RepoTags}}') -Description 'Display relevant image IDs and tags'
    foreach ($key in $afterIds.Keys) {
        $health = Invoke-ExternalText -FilePath 'docker' -ArgumentList @('inspect', $afterIds[$key], '--format', '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}') -Description "Display $key health"
        Write-Host "FinalHealth.$key=$health"
    }

    Write-Section 'T FINAL SUCCESS MARKERS'
    Write-Host 'MMV2NarrowActivation=PASS' -ForegroundColor Green
    Write-Host 'ProtectedServicesPreserved=true'
    Write-Host 'RetainedDataMutated=false'
    Write-Host 'ModelsChanged=false'
    Write-Host 'DatabaseMigration=false'
    Write-Host "ActivationTranscript=$((Resolve-Path -LiteralPath $script:TranscriptPath).Path)"
}
catch {
    $failure = $_.Exception.Message
    Write-Host "ERROR: $failure" -ForegroundColor Red
    if (-not $PreflightOnly -and $null -ne $script:BeforeIds) {
        try {
            if ($script:RecreationStarted) {
                Invoke-GuardedRollback -FailureMessage $failure
            }
            elseif ($script:ServicesStopped) {
                Write-Section 'RESTORE WITHOUT RECREATION'
                Invoke-External -FilePath 'docker' -ArgumentList @('start', $script:BeforeIds.postgres, $script:BeforeIds.nats, $script:BeforeIds.forensic_api, $script:BeforeIds.worker, $script:BeforeIds.localai) -Description 'Restart exact original containers after pre-recreation failure'
                Assert-AllRuntimeHealth -Ids $script:BeforeIds
                $restoredCounts = Get-RetainedCounts -PostgresContainerId $script:BeforeIds.postgres
                Assert-SameRetainedCounts -Before $script:BeforeCounts -After $restoredCounts
                Write-Host 'MMV2NarrowActivation=FAILED_NO_RECREATION_ORIGINAL_CONTAINERS_RESTORED' -ForegroundColor Yellow
                Write-Host 'RetainedDataMutated=false'
                Write-Host 'ModelsChanged=false'
                Write-Host 'DatabaseMigration=false'
            }
        }
        catch {
            Write-Host "AUTOMATIC RECOVERY FAILED: $($_.Exception.Message)" -ForegroundColor Red
            Write-Host 'MMV2NarrowActivation=FAILED_MANUAL_INTERVENTION_REQUIRED' -ForegroundColor Red
        }
    }
    exit 1
}
finally {
    if ($script:TranscriptStarted) {
        Stop-Transcript | Out-Null
    }
}
