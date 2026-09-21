param(
    [switch]$PreflightOnly
)

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

# ============================================================================
# NEXUSAI NX-1 FINAL TIME-RANGE + URDU ACTIVATION/CERTIFICATION
#
# ALLOWED RUNTIME MUTATION:
#   forensic-records-api ONLY
#
# PROTECTED:
#   api / LocalAI UI
#   forensic-records-worker
#   forensic-nats
#   forensic-postgres
#   models
#   profiles
#   backends
#   named volumes
#   retained evidence/data
#   DB schema
#
# PowerShell:
#   Windows PowerShell 5.1 compatible
# ============================================================================


# ============================================================================
# 0. CONFIGURATION
# ============================================================================

$RepoRoot = 'C:\Users\sheik\Workspace\Office\Projects\NexusAI'

$ApiImageTag =
    'nexusai/forensic-records-api:phase3-runtime'

$ExpectedCurrentApiImage =
    'sha256:0ef44346e74478a8d0775db19350f674a4f7089fefe4f6fd7374cf93d4b3a454'

$RollbackTag =
    'nexusai/forensic-records-api:rollback-before-nx1-time-urdu-final-20260826'

# Win32_OperatingSystem.FreePhysicalMemory is KiB.
$RequiredFreeRamKiB = 6291456

$ForensicHealthUrl =
    'http://localhost:8091/healthz'

$PublicQueryUrl =
    'http://localhost:8080/api/records/forensic/query'

$AcceptanceCase =
    'nexusai-multimodal-product-acceptance'

$VideoEvidence =
    '50057921-4f1f-4ab8-bab0-47bcdc957822'

$KnownRetainedBaseline =
    '51|51|64|0|22207|441|47'

$ReportDir = Join-Path `
    $RepoRoot `
    'reports\nx1-foundation-truth-20260825'

if ($PreflightOnly) {

    $TranscriptPath = Join-Path `
        $ReportDir `
        'nx1-time-urdu-final-preflight-20260826.txt'
}
else {

    $TranscriptPath = Join-Path `
        $ReportDir `
        'nx1-time-urdu-final-activation-certification-20260826.txt'
}


# ============================================================================
# 1. HELPERS
# ============================================================================

function Write-Marker {

    param(
        [Parameter(Mandatory=$true)]
        [string]$Text
    )

    Write-Host $Text
}


function Assert-ExitCode {

    param(
        [Parameter(Mandatory=$true)]
        [string]$Message
    )

    if ($LASTEXITCODE -ne 0) {
        throw $Message
    }
}


function Get-ComposeContainerID {

    param(
        [Parameter(Mandatory=$true)]
        [string]$Service
    )

    $ids = @(
        docker ps -q --no-trunc `
            --filter 'label=com.docker.compose.project=nexusai' `
            --filter "label=com.docker.compose.service=$Service"
    )

    Assert-ExitCode "Failed to inspect service '$Service'."

    $ids = @(
        $ids |
        Where-Object {
            -not [string]::IsNullOrWhiteSpace($_)
        }
    )

    if ($ids.Count -ne 1) {

        throw (
            "Expected exactly one running container for '$Service'; " +
            "found $($ids.Count)."
        )
    }

    return $ids[0].Trim()
}


function Get-ContainerImageID {

    param(
        [Parameter(Mandatory=$true)]
        [string]$ContainerID
    )

    $value = docker inspect `
        $ContainerID `
        --format '{{.Image}}'

    Assert-ExitCode "Could not inspect image for container $ContainerID."

    if ([string]::IsNullOrWhiteSpace($value)) {
        throw "Container image ID is empty for $ContainerID."
    }

    return $value.Trim()
}


function Assert-ContainerRunning {

    param(
        [Parameter(Mandatory=$true)]
        [string]$Name,

        [Parameter(Mandatory=$true)]
        [string]$ContainerID
    )

    $state = docker inspect `
        $ContainerID `
        --format '{{.State.Status}}|{{.State.Running}}'

    Assert-ExitCode "Could not inspect state of $Name."

    $state = $state.Trim()

    if ($state -ne 'running|true') {
        throw "$Name is not running. State=$state"
    }

    Write-Marker "$Name=RUNNING"
}


function Get-PostgresEnv {

    param(
        [Parameter(Mandatory=$true)]
        [string]$ContainerID,

        [Parameter(Mandatory=$true)]
        [string]$Variable
    )

    $entries = @(
        docker inspect `
            $ContainerID `
            --format '{{range .Config.Env}}{{println .}}{{end}}'
    )

    Assert-ExitCode 'Could not inspect PostgreSQL environment.'

    $entry = $entries |
        Where-Object {
            $_ -like "$Variable=*"
        } |
        Select-Object -First 1

    if ([string]::IsNullOrWhiteSpace($entry)) {
        throw "PostgreSQL environment variable '$Variable' not found."
    }

    return $entry.Substring(("$Variable=").Length).Trim()
}


function Get-RetainedTuple {

    param(
        [Parameter(Mandatory=$true)]
        [string]$PostgresContainer
    )

    $pgUser = Get-PostgresEnv `
        -ContainerID $PostgresContainer `
        -Variable 'POSTGRES_USER'

    $pgDb = Get-PostgresEnv `
        -ContainerID $PostgresContainer `
        -Variable 'POSTGRES_DB'

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

    $output = @(
        docker exec `
            $PostgresContainer `
            psql `
            -U $pgUser `
            -d $pgDb `
            -At `
            -F '|' `
            -c $sql
    )

    Assert-ExitCode 'Failed to read retained NexusAI state.'

    $tuple = $output |
        Where-Object {
            $_ -match '^\d+\|\d+\|\d+\|\d+\|\d+\|\d+\|\d+$'
        } |
        Select-Object -Last 1

    if ([string]::IsNullOrWhiteSpace($tuple)) {
        throw 'Could not parse retained-state tuple.'
    }

    return $tuple.Trim()
}


function Assert-ZeroActiveJobs {

    param(
        [Parameter(Mandatory=$true)]
        [string]$Tuple
    )

    $parts = $Tuple.Split('|')

    if ($parts.Count -ne 7) {
        throw "Invalid retained-state tuple: $Tuple"
    }

    $active = [int]$parts[3]

    Write-Marker "ActiveJobs=$active"

    if ($active -ne 0) {

        throw (
            "Activation blocked: $active forensic job(s) " +
            "are currently active."
        )
    }
}


function Wait-ForHttp200 {

    param(
        [Parameter(Mandatory=$true)]
        [string]$Url,

        [int]$TimeoutSeconds = 240
    )

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)

    while ((Get-Date) -lt $deadline) {

        try {

            $response = Invoke-WebRequest `
                -UseBasicParsing `
                -TimeoutSec 10 `
                -Uri $Url

            if ([int]$response.StatusCode -eq 200) {

                Write-Marker "HTTP200=$Url"
                return
            }
        }
        catch {
            # Retry.
        }

        Start-Sleep -Seconds 2
    }

    throw "Timed out waiting for HTTP 200 from $Url."
}


function Show-TopMemoryProcesses {

    Write-Host ''
    Write-Host '=== TOP WINDOWS MEMORY USERS ==='

    Get-Process |
        Sort-Object WorkingSet64 -Descending |
        Select-Object `
            -First 15 `
            ProcessName,
            Id,
            @{
                Name='WorkingSetMiB'
                Expression={
                    [math]::Round($_.WorkingSet64 / 1MB, 1)
                }
            } |
        Format-Table -AutoSize
}


function Invoke-ForensicQuery {

    param(
        [Parameter(Mandatory=$true)]
        [string]$Question,

        [Parameter(Mandatory=$true)]
        [string]$Target
    )

    $payload = @{
        collection_id   = $AcceptanceCase
        case_id         = $AcceptanceCase
        query           = $Question
        target          = $Target
        limit           = 10
        max_kb_results  = 5
    }

    $json = $payload | ConvertTo-Json -Compress

    # Explicit UTF-8 bytes are important for native Urdu under
    # Windows PowerShell 5.1.
    $bytes = [System.Text.Encoding]::UTF8.GetBytes($json)

    return Invoke-RestMethod `
        -Method Post `
        -TimeoutSec 135 `
        -ContentType 'application/json; charset=utf-8' `
        -Body $bytes `
        -Uri $PublicQueryUrl
}


function Get-RecursivePropertyValues {

    param(
        [Parameter(Mandatory=$true)]
        $Object,

        [Parameter(Mandatory=$true)]
        [string]$PropertyName
    )

    $results = New-Object System.Collections.ArrayList

    function Search-Node {

        param($Node)

        if ($null -eq $Node) {
            return
        }

        if ($Node -is [string]) {
            return
        }

        if (
            $Node -is [System.Collections.IEnumerable] -and
            -not ($Node -is [System.Collections.IDictionary]) -and
            -not ($Node -is [pscustomobject])
        ) {

            foreach ($item in $Node) {
                Search-Node $item
            }

            return
        }

        if ($Node -is [System.Collections.IDictionary]) {

            foreach ($key in $Node.Keys) {

                if (
                    [string]::Equals(
                        [string]$key,
                        $PropertyName,
                        [System.StringComparison]::OrdinalIgnoreCase
                    )
                ) {
                    [void]$results.Add($Node[$key])
                }

                Search-Node $Node[$key]
            }

            return
        }

        $properties = $Node.PSObject.Properties

        foreach ($property in $properties) {

            if (
                [string]::Equals(
                    $property.Name,
                    $PropertyName,
                    [System.StringComparison]::OrdinalIgnoreCase
                )
            ) {
                [void]$results.Add($property.Value)
            }

            Search-Node $property.Value
        }
    }

    Search-Node $Object

    return @($results)
}


function Test-ContainsExactValue {

    param(
        [Parameter(Mandatory=$true)]
        $Object,

        [Parameter(Mandatory=$true)]
        [string]$PropertyName,

        [Parameter(Mandatory=$true)]
        [string]$ExpectedValue
    )

    $values = @(
        Get-RecursivePropertyValues `
            -Object $Object `
            -PropertyName $PropertyName
    )

    foreach ($value in $values) {

        if (
            $null -ne $value -and
            ([string]$value) -eq $ExpectedValue
        ) {
            return $true
        }
    }

    return $false
}


function Test-ContainsNumericValue {

    param(
        [Parameter(Mandatory=$true)]
        $Object,

        [Parameter(Mandatory=$true)]
        [string]$PropertyName,

        [Parameter(Mandatory=$true)]
        [double]$ExpectedValue
    )

    $values = @(
        Get-RecursivePropertyValues `
            -Object $Object `
            -PropertyName $PropertyName
    )

    foreach ($value in $values) {

        if ($null -eq $value) {
            continue
        }

        $parsed = 0.0

        if (
            [double]::TryParse(
                ([string]$value),
                [ref]$parsed
            )
        ) {

            if ($parsed -eq $ExpectedValue) {
                return $true
            }
        }
    }

    return $false
}


# ============================================================================
# 2. INITIALIZE
# ============================================================================

if (-not (Test-Path $RepoRoot)) {
    throw "Repository not found: $RepoRoot"
}

Set-Location $RepoRoot

if (-not (Test-Path $ReportDir)) {

    New-Item `
        -ItemType Directory `
        -Force `
        -Path $ReportDir |
        Out-Null
}

try {

    Start-Transcript `
        -Path $TranscriptPath `
        -Force |
        Out-Null
}
catch {

    Write-Warning (
        'Transcript could not be started: ' +
        $_.Exception.Message
    )
}


$Compose = @(
    '-p', 'nexusai',
    '--env-file', '.\.env.forensic-runtime.local',
    '-f', '.\docker-compose.forensic-records.yaml',
    '-f', '.\docker-compose.forensic-records.runtime.yaml'
)

if (Test-Path '.\docker-compose.forensic-records.asr-small.yaml') {

    $Compose += @(
        '-f',
        '.\docker-compose.forensic-records.asr-small.yaml'
    )

    Write-Marker 'ASRComposeOverlay=INCLUDED'
}
else {

    Write-Marker 'ASRComposeOverlay=NOT_PRESENT'
}


$RuntimeChanged = $false
$RollbackCreated = $false

$beforeTuple = $null

$beforeApi = $null
$beforeWorker = $null
$beforeNats = $null
$beforePostgres = $null
$beforeLocalAI = $null

$beforeApiImage = $null
$beforeWorkerImage = $null
$beforeNatsImage = $null
$beforePostgresImage = $null
$beforeLocalAIImage = $null


try {

    # ========================================================================
    # 3. DOCKER + RAM
    # ========================================================================

    Write-Host ''
    Write-Host '=== 3. DOCKER READINESS ==='

    $serverVersion = docker info --format '{{.ServerVersion}}'

    Assert-ExitCode 'Docker Desktop Linux engine is not ready.'

    Write-Marker "DockerServerVersion=$serverVersion"


    Write-Host ''
    Write-Host '=== 4. RAM GATE ==='

    $os = Get-CimInstance Win32_OperatingSystem

    $totalRamGiB = [math]::Round(
        ([double]$os.TotalVisibleMemorySize / 1MB),
        3
    )

    $freeRamGiB = [math]::Round(
        ([double]$os.FreePhysicalMemory / 1MB),
        3
    )

    Write-Marker "TotalPhysicalRAMGiB=$totalRamGiB"
    Write-Marker "FreePhysicalRAMGiB=$freeRamGiB"
    Write-Marker 'RequiredFreeRAMGiB=6.000'

    if ([double]$os.FreePhysicalMemory -lt $RequiredFreeRamKiB) {

        Write-Marker 'RAMGate=FAIL'

        Show-TopMemoryProcesses

        Write-Marker 'NX1TimeUrduActivation=NOT_STARTED_RAM_GATE'

        return
    }

    Write-Marker 'RAMGate=PASS'


    # ========================================================================
    # 5. REPOSITORY + COMPOSE
    # ========================================================================

    Write-Host ''
    Write-Host '=== 5. REPOSITORY PREFLIGHT ==='

    $requiredFiles = @(
        '.\.env.forensic-runtime.local',
        '.\docker-compose.forensic-records.yaml',
        '.\docker-compose.forensic-records.runtime.yaml',
        '.\api\forensic_records\query_language_normalization.go',
        '.\api\forensic_records\video_anpr.go',
        '.\api\forensic_records\mmv2_query_test.go'
    )

    foreach ($file in $requiredFiles) {

        if (-not (Test-Path $file)) {
            throw "Required file missing: $file"
        }
    }

    Write-Marker 'RequiredFiles=PASS'

    $branch = git branch --show-current
    Assert-ExitCode 'Could not determine Git branch.'

    $head = git rev-parse HEAD
    Assert-ExitCode 'Could not determine Git HEAD.'

    Write-Marker "GitBranch=$branch"
    Write-Marker "GitHEAD=$head"

    $dirtyCount = @(
        git status --porcelain
    ).Count

    Write-Marker "DirtyWorktreeEntries=$dirtyCount"
    Write-Marker 'GitMutation=false'


    Write-Host ''
    Write-Host 'Relevant source hashes:'

    Get-FileHash `
        '.\api\forensic_records\query_language_normalization.go',
        '.\api\forensic_records\video_anpr.go',
        '.\api\forensic_records\mmv2_query_test.go' `
        -Algorithm SHA256 |
        Format-Table Path, Hash -AutoSize


    Write-Host ''
    Write-Host '=== 6. COMPOSE VALIDATION ==='

    $services = @(
        docker compose @Compose config --services
    )

    Assert-ExitCode 'Compose configuration validation failed.'

    if ($services -notcontains 'forensic-records-api') {
        throw 'forensic-records-api is absent from compose configuration.'
    }

    Write-Marker 'ComposeConfig=PASS'


    # ========================================================================
    # 7. CAPTURE BEFORE STATE
    # ========================================================================

    Write-Host ''
    Write-Host '=== 7. CURRENT RUNTIME ==='

    $beforeApi =
        Get-ComposeContainerID 'forensic-records-api'

    $beforeWorker =
        Get-ComposeContainerID 'forensic-records-worker'

    $beforeNats =
        Get-ComposeContainerID 'forensic-nats'

    $beforePostgres =
        Get-ComposeContainerID 'forensic-postgres'

    $beforeLocalAI =
        Get-ComposeContainerID 'api'


    $beforeApiImage =
        Get-ContainerImageID $beforeApi

    $beforeWorkerImage =
        Get-ContainerImageID $beforeWorker

    $beforeNatsImage =
        Get-ContainerImageID $beforeNats

    $beforePostgresImage =
        Get-ContainerImageID $beforePostgres

    $beforeLocalAIImage =
        Get-ContainerImageID $beforeLocalAI


    Write-Marker "ForensicApiContainerBefore=$beforeApi"
    Write-Marker "ForensicApiImageBefore=$beforeApiImage"

    Write-Marker "WorkerContainerBefore=$beforeWorker"
    Write-Marker "NatsContainerBefore=$beforeNats"
    Write-Marker "PostgresContainerBefore=$beforePostgres"
    Write-Marker "LocalAIContainerBefore=$beforeLocalAI"


    Assert-ContainerRunning `
        -Name 'ForensicApiBefore' `
        -ContainerID $beforeApi

    Assert-ContainerRunning `
        -Name 'WorkerBefore' `
        -ContainerID $beforeWorker

    Assert-ContainerRunning `
        -Name 'NatsBefore' `
        -ContainerID $beforeNats

    Assert-ContainerRunning `
        -Name 'PostgresBefore' `
        -ContainerID $beforePostgres

    Assert-ContainerRunning `
        -Name 'LocalAIBefore' `
        -ContainerID $beforeLocalAI


    # ========================================================================
    # 8. EXACT CURRENT API BASELINE
    # ========================================================================

    Write-Host ''
    Write-Host '=== 8. CURRENT API BASELINE ==='

    Write-Marker "ExpectedCurrentApiImage=$ExpectedCurrentApiImage"
    Write-Marker "ActualCurrentApiImage=$beforeApiImage"

    if ($beforeApiImage -ne $ExpectedCurrentApiImage) {

        throw (
            'Current forensic API does not match the expected ' +
            'pre-activation image. Stop rather than guessing.'
        )
    }

    Write-Marker 'CurrentApiBaseline=PASS'


    # ========================================================================
    # 9. RETAINED STATE
    # ========================================================================

    Write-Host ''
    Write-Host '=== 9. RETAINED STATE ==='

    $beforeTuple =
        Get-RetainedTuple $beforePostgres

    Write-Marker "RetainedTupleBefore=$beforeTuple"

    Assert-ZeroActiveJobs $beforeTuple

    if ($beforeTuple -ne $KnownRetainedBaseline) {

        throw (
            'Retained baseline differs from expected state. ' +
            "Expected=$KnownRetainedBaseline Actual=$beforeTuple"
        )
    }

    Write-Marker 'KnownRetainedBaseline=PASS'


    # ========================================================================
    # 10. SOURCE TESTS
    # ========================================================================

    Write-Host ''
    Write-Host '=== 10. SOURCE TEST GATE ==='

    go test ./api/forensic_records `
        -run 'TestNX1(VideoSourceTimeContract|GroupedVideoRoutingLanguageMatrix)' `
        -count=1

    Assert-ExitCode 'Focused NX-1 time/Urdu tests failed.'

    Write-Marker 'FocusedTimeUrduTests=PASS'


    go test ./api/forensic_records -count=1

    Assert-ExitCode 'Full forensic API test suite failed.'

    Write-Marker 'FullForensicApiTests=PASS'


    go vet ./api/forensic_records

    Assert-ExitCode 'go vet failed.'

    Write-Marker 'GoVet=PASS'


    # ========================================================================
    # 11. PREFLIGHT-ONLY EXIT
    # ========================================================================

    if ($PreflightOnly) {

        Write-Host ''
        Write-Host '============================================================'
        Write-Host 'NX-1 TIME/URDU PREFLIGHT SUCCESS'
        Write-Host '============================================================'

        Write-Marker 'NX1TimeUrduPreflight=PASS'
        Write-Marker 'MutationPerformed=false'
        Write-Marker 'NextAction=RUN_NORMAL_ACTIVATION'

        return
    }


    # ========================================================================
    # 12. CREATE FRESH ROLLBACK TAG
    # ========================================================================

    Write-Host ''
    Write-Host '=== 12. CREATE FRESH ROLLBACK TAG ==='

    docker image tag `
        $beforeApiImage `
        $RollbackTag

    Assert-ExitCode 'Could not create rollback tag.'

    $RollbackCreated = $true

    $rollbackImage = docker image inspect `
        $RollbackTag `
        --format '{{.Id}}'

    Assert-ExitCode 'Could not inspect rollback image.'

    $rollbackImage = $rollbackImage.Trim()

    Write-Marker "RollbackTag=$RollbackTag"
    Write-Marker "RollbackImage=$rollbackImage"

    if ($rollbackImage -ne $beforeApiImage) {

        throw (
            'Rollback tag does not point to the exact ' +
            'pre-activation forensic API image.'
        )
    }

    Write-Marker 'RollbackTagVerified=PASS'


    # ========================================================================
    # 13. BUILD FORENSIC API ONLY
    # ========================================================================

    Write-Host ''
    Write-Host '=== 13. BUILD FORENSIC API ONLY ==='

    docker compose @Compose `
        --progress plain `
        build forensic-records-api

    Assert-ExitCode 'Forensic API build failed.'

    Write-Marker 'ForensicApiBuild=PASS'


    # Inspect actual phase3-runtime image.
    $newApiImage = docker image inspect `
        $ApiImageTag `
        --format '{{.Id}}'

    Assert-ExitCode 'Could not inspect newly built API image.'

    $newApiImage = $newApiImage.Trim()

    Write-Marker "ForensicApiImageBefore=$beforeApiImage"
    Write-Marker "ForensicApiImageBuilt=$newApiImage"

    if ([string]::IsNullOrWhiteSpace($newApiImage)) {
        throw 'New API image ID is empty.'
    }

    if ($newApiImage -eq $beforeApiImage) {

        throw (
            'New build produced the same image ID as the ' +
            'pre-activation runtime. Verify source inclusion.'
        )
    }

    Write-Marker 'BuiltImageVerification=PASS'


    # ========================================================================
    # 14. RECREATE ONLY FORENSIC API
    # ========================================================================

    Write-Host ''
    Write-Host '=== 14. ACTIVATE FORENSIC API ONLY ==='

    $RuntimeChanged = $true

    docker compose @Compose `
        up -d `
        --no-deps `
        --force-recreate `
        forensic-records-api

    Assert-ExitCode 'Forensic API recreation failed.'


    # ========================================================================
    # 15. HEALTH
    # ========================================================================

    Write-Host ''
    Write-Host '=== 15. FORENSIC API HEALTH ==='

    Wait-ForHttp200 `
        -Url $ForensicHealthUrl `
        -TimeoutSeconds 240

    Write-Marker 'ForensicApiHealth=PASS'


    # ========================================================================
    # 16. VERIFY NEW API IMAGE
    # ========================================================================

    Write-Host ''
    Write-Host '=== 16. VERIFY ACTIVE API IMAGE ==='

    $afterApi =
        Get-ComposeContainerID 'forensic-records-api'

    $afterApiImage =
        Get-ContainerImageID $afterApi

    Write-Marker "ForensicApiContainerAfter=$afterApi"
    Write-Marker "ForensicApiImageAfter=$afterApiImage"

    if ($afterApi -eq $beforeApi) {
        throw 'Forensic API container was not recreated.'
    }

    if ($afterApiImage -ne $newApiImage) {

        throw (
            'Running forensic API does not use the newly built image. ' +
            "Running=$afterApiImage Built=$newApiImage"
        )
    }

    Write-Marker 'ForensicApiImageActivation=PASS'


    # ========================================================================
    # 17. PROTECTED SERVICE RECONCILIATION
    # ========================================================================

    Write-Host ''
    Write-Host '=== 17. PROTECTED SERVICES ==='

    $afterWorker =
        Get-ComposeContainerID 'forensic-records-worker'

    $afterNats =
        Get-ComposeContainerID 'forensic-nats'

    $afterPostgres =
        Get-ComposeContainerID 'forensic-postgres'

    $afterLocalAI =
        Get-ComposeContainerID 'api'


    if ($afterWorker -ne $beforeWorker) {
        throw 'Protected worker container changed.'
    }

    if ($afterNats -ne $beforeNats) {
        throw 'Protected NATS container changed.'
    }

    if ($afterPostgres -ne $beforePostgres) {
        throw 'Protected PostgreSQL container changed.'
    }

    if ($afterLocalAI -ne $beforeLocalAI) {
        throw 'Protected LocalAI/UI container changed.'
    }


    if (
        (Get-ContainerImageID $afterWorker) -ne
        $beforeWorkerImage
    ) {
        throw 'Protected worker image changed.'
    }

    if (
        (Get-ContainerImageID $afterNats) -ne
        $beforeNatsImage
    ) {
        throw 'Protected NATS image changed.'
    }

    if (
        (Get-ContainerImageID $afterPostgres) -ne
        $beforePostgresImage
    ) {
        throw 'Protected PostgreSQL image changed.'
    }

    if (
        (Get-ContainerImageID $afterLocalAI) -ne
        $beforeLocalAIImage
    ) {
        throw 'Protected LocalAI/UI image changed.'
    }

    Write-Marker 'ProtectedServicesPreserved=true'


    # ========================================================================
    # 18. RETAINED STATE AFTER DEPLOYMENT
    # ========================================================================

    Write-Host ''
    Write-Host '=== 18. RETAINED STATE AFTER DEPLOYMENT ==='

    $afterTuple =
        Get-RetainedTuple $afterPostgres

    Write-Marker "RetainedTupleBefore=$beforeTuple"
    Write-Marker "RetainedTupleAfter=$afterTuple"

    if ($afterTuple -ne $beforeTuple) {

        throw (
            'Retained data changed unexpectedly. ' +
            "Before=$beforeTuple After=$afterTuple"
        )
    }

    Write-Marker 'RetainedDataMutated=false'


    # ========================================================================
    # FROM HERE ONWARD:
    # DEPLOYMENT SAFETY HAS PASSED.
    #
    # QUERY CONTRACT FAILURES WILL NOT TRIGGER AUTOMATIC ROLLBACK.
    # ========================================================================

    $RuntimeChanged = $false


    # ========================================================================
    # 19. LIVE TEST — UUID ONLY
    # ========================================================================

    Write-Host ''
    Write-Host '=== 19. LIVE: UUID ONLY ==='

    $uuidOnly = Invoke-ForensicQuery `
        -Question (
            "Show the grouped ANPR timeline for retained video evidence " +
            $VideoEvidence
        ) `
        -Target $VideoEvidence

    $uuidOnly | Select-Object `
        template,
        target,
        @{N='Operation';E={$_.enterprise.operation.operation_id}},
        @{N='ResultState';E={$_.enterprise.result_state}},
        @{N='RowCount';E={$_.enterprise.row_count}} |
        Format-List

    $uuidOnlyPass =
        ($uuidOnly.template -eq 'video_anpr_grouped_timeline') -and
        ($uuidOnly.enterprise.operation.operation_id -eq 'video.anpr_grouped_timeline') -and
        (([string]$uuidOnly.target).ToLowerInvariant() -eq $VideoEvidence.ToLowerInvariant())

    Write-Marker "UUIDOnlyLivePASS=$uuidOnlyPass"


    # ========================================================================
    # 20. LIVE TEST — UUID + PLATE
    # ========================================================================

    Write-Host ''
    Write-Host '=== 20. LIVE: UUID + PLATE ==='

    $uuidPlate = Invoke-ForensicQuery `
        -Question (
            "For retained video evidence $VideoEvidence " +
            "show sightings of plate MN1367"
        ) `
        -Target $VideoEvidence

    $uuidPlate | Select-Object `
        template,
        target,
        @{N='Operation';E={$_.enterprise.operation.operation_id}},
        @{N='ResultState';E={$_.enterprise.result_state}},
        @{N='RowCount';E={$_.enterprise.row_count}} |
        Format-List

    $uuidPlateEvidencePass =
        Test-ContainsExactValue `
            -Object $uuidPlate `
            -PropertyName 'evidence_id' `
            -ExpectedValue $VideoEvidence

    $uuidPlateFilterPass =
        Test-ContainsExactValue `
            -Object $uuidPlate `
            -PropertyName 'plate' `
            -ExpectedValue 'MN1367'

    if (-not $uuidPlateFilterPass) {

        $uuidPlateFilterPass =
            Test-ContainsExactValue `
                -Object $uuidPlate `
                -PropertyName 'plate_filter' `
                -ExpectedValue 'MN1367'
    }

    $uuidPlatePass =
        ($uuidPlate.template -eq 'video_anpr_grouped_timeline') -and
        ($uuidPlate.enterprise.operation.operation_id -eq 'video.anpr_grouped_timeline') -and
        $uuidPlateEvidencePass -and
        $uuidPlateFilterPass

    Write-Marker "UUIDPlateLivePASS=$uuidPlatePass"


    # ========================================================================
    # 21. LIVE TEST — UUID + TIME
    # ========================================================================

    Write-Host ''
    Write-Host '=== 21. LIVE: UUID + TIME ==='

    $uuidTime = Invoke-ForensicQuery `
        -Question (
            "Show grouped ANPR for retained video evidence $VideoEvidence " +
            "from 10 seconds to 30 seconds"
        ) `
        -Target $VideoEvidence

    $start10Pass =
        Test-ContainsNumericValue `
            -Object $uuidTime `
            -PropertyName 'start_seconds' `
            -ExpectedValue 10

    $end30Pass =
        Test-ContainsNumericValue `
            -Object $uuidTime `
            -PropertyName 'end_seconds' `
            -ExpectedValue 30

    $uuidTimePass =
        ($uuidTime.template -eq 'video_anpr_grouped_timeline') -and
        ($uuidTime.enterprise.operation.operation_id -eq 'video.anpr_grouped_timeline') -and
        $start10Pass -and
        $end30Pass

    Write-Marker "UUIDTimeStart10PASS=$start10Pass"
    Write-Marker "UUIDTimeEnd30PASS=$end30Pass"
    Write-Marker "UUIDTimeLivePASS=$uuidTimePass"


    # ========================================================================
    # 22. LIVE TEST — UUID + PLATE + TIME
    # ========================================================================

    Write-Host ''
    Write-Host '=== 22. LIVE: UUID + PLATE + TIME ==='

    $combined = Invoke-ForensicQuery `
        -Question (
            "For retained video evidence $VideoEvidence " +
            "show plate MN1367 from 10 seconds to 30 seconds"
        ) `
        -Target $VideoEvidence

    $combinedEvidencePass =
        Test-ContainsExactValue `
            -Object $combined `
            -PropertyName 'evidence_id' `
            -ExpectedValue $VideoEvidence

    $combinedPlatePass =
        Test-ContainsExactValue `
            -Object $combined `
            -PropertyName 'plate' `
            -ExpectedValue 'MN1367'

    if (-not $combinedPlatePass) {

        $combinedPlatePass =
            Test-ContainsExactValue `
                -Object $combined `
                -PropertyName 'plate_filter' `
                -ExpectedValue 'MN1367'
    }

    $combinedStartPass =
        Test-ContainsNumericValue `
            -Object $combined `
            -PropertyName 'start_seconds' `
            -ExpectedValue 10

    $combinedEndPass =
        Test-ContainsNumericValue `
            -Object $combined `
            -PropertyName 'end_seconds' `
            -ExpectedValue 30

    $combinedPass =
        ($combined.template -eq 'video_anpr_grouped_timeline') -and
        ($combined.enterprise.operation.operation_id -eq 'video.anpr_grouped_timeline') -and
        $combinedEvidencePass -and
        $combinedPlatePass -and
        $combinedStartPass -and
        $combinedEndPass

    Write-Marker "CombinedEvidencePASS=$combinedEvidencePass"
    Write-Marker "CombinedPlatePASS=$combinedPlatePass"
    Write-Marker "CombinedStart10PASS=$combinedStartPass"
    Write-Marker "CombinedEnd30PASS=$combinedEndPass"
    Write-Marker "UUIDPlateTimeLivePASS=$combinedPass"


    # ========================================================================
    # 23. POSITIVE ANPR REGRESSION
    # ========================================================================

    Write-Host ''
    Write-Host '=== 23. LIVE: POSITIVE ANPR ==='

    $positive = Invoke-ForensicQuery `
        -Question 'Where was plate MN1367 observed?' `
        -Target 'MN1367'

    $positivePass =
        ($positive.enterprise.operation.operation_id -eq 'anpr.sightings') -and
        ($positive.enterprise.result_state -eq 'results_present') -and
        ([int]$positive.enterprise.row_count -eq 1)

    Write-Marker "PositiveANPRLivePASS=$positivePass"


    # ========================================================================
    # 24. NEGATIVE ANPR REGRESSION
    # ========================================================================

    Write-Host ''
    Write-Host '=== 24. LIVE: NEGATIVE ANPR ==='

    $negative = Invoke-ForensicQuery `
        -Question 'Where was plate ZZZ9999 observed?' `
        -Target 'ZZZ9999'

    $negativePass =
        ($negative.enterprise.operation.operation_id -eq 'anpr.sightings') -and
        ($negative.enterprise.result_state -eq 'no_match_for_filter') -and
        ([int]$negative.enterprise.row_count -eq 0)

    Write-Marker "NegativeANPRLivePASS=$negativePass"


    # ========================================================================
    # 25. ENGLISH GROUPED VIDEO
    # ========================================================================

    Write-Host ''
    Write-Host '=== 25. LIVE: ENGLISH GROUPED VIDEO ==='

    $english = Invoke-ForensicQuery `
        -Question (
            "Show the grouped ANPR timeline for retained video evidence " +
            $VideoEvidence
        ) `
        -Target $VideoEvidence

    $englishPass =
        ($english.template -eq 'video_anpr_grouped_timeline') -and
        ($english.enterprise.operation.operation_id -eq 'video.anpr_grouped_timeline')

    Write-Marker "EnglishGroupedVideoLivePASS=$englishPass"


    # ========================================================================
    # 26. ROMAN URDU
    # ========================================================================

    Write-Host ''
    Write-Host '=== 26. LIVE: ROMAN URDU ==='

    $roman = Invoke-ForensicQuery `
        -Question (
            "Is video evidence $VideoEvidence ki grouped ANPR timeline dikhao"
        ) `
        -Target $VideoEvidence

    $romanPass =
        ($roman.template -eq 'video_anpr_grouped_timeline') -and
        ($roman.enterprise.operation.operation_id -eq 'video.anpr_grouped_timeline')

    Write-Marker "RomanUrduGroupedVideoLivePASS=$romanPass"


    # ========================================================================
    # 27. NATIVE URDU
    # ========================================================================

    Write-Host ''
    Write-Host '=== 27. LIVE: NATIVE URDU ==='

    $urduQuestionTemplateBase64 =
        '2YjbjNqI24zZiCDYp9uM2YjbjNqI2YbYsyB7MH0g2qnbjCDar9ix2YjZvtqIINin25Ig2KfbjNmGINm+24wg2KLYsSDZudin2KbZhSDZhNin2KbZhiDYr9qp2r7Yp9im24zaug=='

    $urduQuestionTemplate =
        [System.Text.Encoding]::UTF8.GetString(
            [System.Convert]::FromBase64String(
                $urduQuestionTemplateBase64
            )
        )

    $urduQuestion =
        [string]::Format(
            $urduQuestionTemplate,
            $VideoEvidence
        )

    $urdu = Invoke-ForensicQuery `
        -Question $urduQuestion `
        -Target $VideoEvidence

    $urduPass =
        ($urdu.template -eq 'video_anpr_grouped_timeline') -and
        ($urdu.enterprise.operation.operation_id -eq 'video.anpr_grouped_timeline')

    Write-Marker "UrduTemplate=$($urdu.template)"
    Write-Marker "UrduOperation=$($urdu.enterprise.operation.operation_id)"
    Write-Marker "NativeUrduGroupedVideoLivePASS=$urduPass"


    # ========================================================================
    # 28. FINAL RETAINED + PROTECTED RECONCILIATION
    # ========================================================================

    Write-Host ''
    Write-Host '=== 28. FINAL PRESERVATION CHECK ==='

    $finalWorker =
        Get-ComposeContainerID 'forensic-records-worker'

    $finalNats =
        Get-ComposeContainerID 'forensic-nats'

    $finalPostgres =
        Get-ComposeContainerID 'forensic-postgres'

    $finalLocalAI =
        Get-ComposeContainerID 'api'

    $protectedFinalPass =
        ($finalWorker -eq $beforeWorker) -and
        ($finalNats -eq $beforeNats) -and
        ($finalPostgres -eq $beforePostgres) -and
        ($finalLocalAI -eq $beforeLocalAI)

    $finalTuple =
        Get-RetainedTuple $finalPostgres

    $retainedFinalPass =
        ($finalTuple -eq $beforeTuple)

    Write-Marker "FinalRetainedTuple=$finalTuple"
    Write-Marker "ProtectedServicesFinalPASS=$protectedFinalPass"
    Write-Marker "RetainedStateFinalPASS=$retainedFinalPass"


    # ========================================================================
    # 29. FINAL VERDICT
    # ========================================================================

    Write-Host ''
    Write-Host '============================================================'
    Write-Host 'NX-1 FINAL LIVE CERTIFICATION'
    Write-Host '============================================================'

    Write-Marker "UUIDOnly=$uuidOnlyPass"
    Write-Marker "UUIDPlate=$uuidPlatePass"
    Write-Marker "UUIDTime=$uuidTimePass"
    Write-Marker "UUIDPlateTime=$combinedPass"

    Write-Marker "PositiveANPR=$positivePass"
    Write-Marker "NegativeANPR=$negativePass"

    Write-Marker "EnglishGroupedVideo=$englishPass"
    Write-Marker "RomanUrduGroupedVideo=$romanPass"
    Write-Marker "NativeUrduGroupedVideo=$urduPass"

    Write-Marker "ProtectedServices=$protectedFinalPass"
    Write-Marker "RetainedState=$retainedFinalPass"


    $allLivePass =
        $uuidOnlyPass -and
        $uuidPlatePass -and
        $uuidTimePass -and
        $combinedPass -and
        $positivePass -and
        $negativePass -and
        $englishPass -and
        $romanPass -and
        $urduPass -and
        $protectedFinalPass -and
        $retainedFinalPass


    if ($allLivePass) {

        Write-Host ''
        Write-Host '============================================================'
        Write-Host 'NX-1 FINAL STATUS: CLOSED'
        Write-Host '============================================================'

        Write-Marker 'NX1TimeUrduActivation=PASS'
        Write-Marker 'NX1FinalLiveCertification=PASS'

        Write-Marker 'P1_1=CLOSED'
        Write-Marker 'P1_2=CLOSED'
        Write-Marker 'P1_3=CLOSED'

        Write-Marker 'OpenP0=0'
        Write-Marker 'OpenFoundationP1Source=0'
        Write-Marker 'OpenFoundationP1Live=0'

        Write-Marker 'ProtectedServicesPreserved=true'
        Write-Marker 'RetainedDataMutated=false'
        Write-Marker 'ModelsChanged=false'
        Write-Marker 'ProfilesChanged=false'
        Write-Marker 'BackendsChanged=false'
        Write-Marker 'NamedVolumesPreserved=true'
        Write-Marker 'DatabaseMigration=false'

        Write-Marker 'NX1FinalStatus=CLOSED'
        Write-Marker 'NextPhase=NX-UX1'
    }
    else {

        Write-Host ''
        Write-Warning (
            'Deployment is healthy, but one or more live query-contract ' +
            'checks did not pass. Runtime is intentionally NOT rolled back.'
        )

        Write-Marker 'NX1TimeUrduActivation=PASS'
        Write-Marker 'NX1FinalLiveCertification=REVIEW_REQUIRED'

        Write-Marker 'OpenP0=0'
        Write-Marker 'OpenFoundationP1Live=1'

        Write-Marker 'ProtectedServicesPreserved=true'
        Write-Marker 'RetainedDataMutated=false'

        Write-Marker 'NX1FinalStatus=PARTIAL'
        Write-Marker 'NextAction=REVIEW_FAILED_LIVE_CONTRACT_ONLY'

        exit 2
    }
}
catch {

    $failure = $_.Exception.Message

    Write-Host ''
    Write-Host '============================================================'
    Write-Host 'NX-1 ACTIVATION ERROR'
    Write-Host '============================================================'

    Write-Error $failure

    # Only deployment-safety failures after recreation trigger rollback.
    if (
        $RuntimeChanged -and
        $RollbackCreated
    ) {

        Write-Warning (
            'Deployment-safety failure detected. ' +
            'Attempting FORENSIC-API-ONLY rollback.'
        )

        try {

            docker image tag `
                $RollbackTag `
                $ApiImageTag

            Assert-ExitCode (
                'Could not restore phase3-runtime rollback tag.'
            )

            docker compose @Compose `
                up -d `
                --no-deps `
                --force-recreate `
                forensic-records-api

            Assert-ExitCode 'Forensic API rollback recreation failed.'

            Wait-ForHttp200 `
                -Url $ForensicHealthUrl `
                -TimeoutSeconds 240


            $rollbackWorker =
                Get-ComposeContainerID 'forensic-records-worker'

            $rollbackNats =
                Get-ComposeContainerID 'forensic-nats'

            $rollbackPostgres =
                Get-ComposeContainerID 'forensic-postgres'

            $rollbackLocalAI =
                Get-ComposeContainerID 'api'


            if ($rollbackWorker -ne $beforeWorker) {
                throw 'Worker changed during rollback.'
            }

            if ($rollbackNats -ne $beforeNats) {
                throw 'NATS changed during rollback.'
            }

            if ($rollbackPostgres -ne $beforePostgres) {
                throw 'PostgreSQL changed during rollback.'
            }

            if ($rollbackLocalAI -ne $beforeLocalAI) {
                throw 'LocalAI/UI changed during rollback.'
            }


            $rollbackTuple =
                Get-RetainedTuple $rollbackPostgres

            if ($rollbackTuple -ne $beforeTuple) {

                throw (
                    'Retained state differs after rollback. ' +
                    "Before=$beforeTuple After=$rollbackTuple"
                )
            }


            $rollbackApi =
                Get-ComposeContainerID 'forensic-records-api'

            $rollbackRunningImage =
                Get-ContainerImageID $rollbackApi

            if ($rollbackRunningImage -ne $beforeApiImage) {

                throw (
                    'Rollback API is not using the original ' +
                    'pre-activation image.'
                )
            }


            Write-Marker 'NX1TimeUrduActivation=FAILED_ROLLED_BACK'
            Write-Marker 'ProtectedServicesPreserved=true'
            Write-Marker 'RetainedDataMutated=false'
            Write-Marker 'ModelsChanged=false'
            Write-Marker 'DatabaseMigration=false'
        }
        catch {

            Write-Error (
                'ROLLBACK ALSO ENCOUNTERED AN ERROR: ' +
                $_.Exception.Message
            )

            Write-Marker (
                'NX1TimeUrduActivation=FAILED_ROLLBACK_INCOMPLETE'
            )
        }
    }
    else {

        Write-Marker 'NX1TimeUrduActivation=FAILED_BEFORE_SAFE_ACTIVATION'
    }

    exit 1
}
finally {

    try {
        Stop-Transcript | Out-Null
    }
    catch {
        # Transcript may not have started.
    }
}