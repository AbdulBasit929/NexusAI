param(
    [switch]$PreflightOnly
)

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

# ============================================================================
# NexusAI NX-1 P1-3 FINAL API-ONLY ACTIVATION
#
# ALLOWED:
#   - build forensic-records-api
#   - recreate forensic-records-api
#
# PROTECTED:
#   - api / LocalAI UI
#   - forensic-records-worker
#   - forensic-nats
#   - forensic-postgres
#   - models
#   - profiles
#   - backends
#   - volumes
#   - retained evidence/data
#   - DB schema
#
# Compatible with Windows PowerShell 5.1.
# ============================================================================

$RepoRoot = 'C:\Users\sheik\Workspace\Office\Projects\NexusAI'

$ApiImageTag = 'nexusai/forensic-records-api:phase3-runtime'

$RollbackTag =
    'nexusai/forensic-records-api:rollback-before-nx1-p13-final-20260826'

# 6 GiB expressed in Win32_OperatingSystem FreePhysicalMemory KiB.
$RequiredFreeRamKiB = 6291456

$ForensicHealthUrl = 'http://localhost:8091/healthz'

$ReportDir = Join-Path `
    $RepoRoot `
    'reports\nx1-foundation-truth-20260825'

if ($PreflightOnly) {
    $TranscriptPath = Join-Path `
        $ReportDir `
        'nx1-p13-final-preflight-20260826.txt'
}
else {
    $TranscriptPath = Join-Path `
        $ReportDir `
        'nx1-p13-final-activation-20260826.txt'
}

# ============================================================================
# Helpers
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
            "Expected exactly one running container for service '$Service', " +
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

    Assert-ExitCode "Failed to inspect container image for $ContainerID."

    if ([string]::IsNullOrWhiteSpace($value)) {
        throw "Container image ID is empty for $ContainerID."
    }

    return $value.Trim()
}

function Get-ContainerState {
    param(
        [Parameter(Mandatory=$true)]
        [string]$ContainerID
    )

    $state = docker inspect `
        $ContainerID `
        --format '{{.State.Status}}|{{.State.Running}}'

    Assert-ExitCode "Could not inspect container state for $ContainerID."

    return $state.Trim()
}

function Assert-ContainerRunning {
    param(
        [Parameter(Mandatory=$true)]
        [string]$Name,

        [Parameter(Mandatory=$true)]
        [string]$ContainerID
    )

    $state = Get-ContainerState $ContainerID

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

    $values = @(
        docker inspect `
            $ContainerID `
            --format '{{range .Config.Env}}{{println .}}{{end}}'
    )

    Assert-ExitCode 'Failed to inspect PostgreSQL environment.'

    $entry = $values |
        Where-Object {
            $_ -like "$Variable=*"
        } |
        Select-Object -First 1

    if ([string]::IsNullOrWhiteSpace($entry)) {
        throw "Could not resolve PostgreSQL environment variable $Variable."
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

    Assert-ExitCode 'Failed to query retained NexusAI state.'

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
        throw "Invalid retained tuple: $Tuple"
    }

    $activeJobs = [int]$parts[3]

    Write-Marker "ActiveJobs=$activeJobs"

    if ($activeJobs -ne 0) {
        throw (
            "Activation blocked because $activeJobs forensic job(s) " +
            "are active."
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

# ============================================================================
# Repository / report initialization
# ============================================================================

if (-not (Test-Path $RepoRoot)) {
    throw "Repository does not exist: $RepoRoot"
}

Set-Location $RepoRoot

if (-not (Test-Path $ReportDir)) {
    New-Item `
        -ItemType Directory `
        -Path $ReportDir `
        -Force |
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

# Runtime values needed for rollback.
$ActivationStarted = $false
$RollbackCreated = $false
$RollbackAttempted = $false

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

# ============================================================================
# Compose configuration
# ============================================================================

$Compose = @(
    '-p', 'nexusai',
    '--env-file', '.\.env.forensic-runtime.local',
    '-f', '.\docker-compose.forensic-records.yaml',
    '-f', '.\docker-compose.forensic-records.runtime.yaml'
)

# Use the ASR overlay if this repository/runtime still has it.
# It should not alter the API-only scope, but including the existing overlay
# keeps compose resolution consistent with the current forensic runtime.
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

try {

    # ========================================================================
    # 1. Docker readiness
    # ========================================================================

    Write-Host ''
    Write-Host '=== 1. DOCKER READINESS ==='

    $serverVersion = docker info --format '{{.ServerVersion}}'

    Assert-ExitCode 'Docker Desktop Linux engine is not ready.'

    Write-Marker "DockerServerVersion=$serverVersion"

    # ========================================================================
    # 2. RAM gate
    # ========================================================================

    Write-Host ''
    Write-Host '=== 2. PHYSICAL RAM GATE ==='

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

        Write-Marker 'NX1P13Activation=NOT_STARTED_RAM_GATE'

        return
    }

    Write-Marker 'RAMGate=PASS'

    # ========================================================================
    # 3. Required repository files
    # ========================================================================

    Write-Host ''
    Write-Host '=== 3. REPOSITORY PREFLIGHT ==='

    $requiredFiles = @(
        '.\.env.forensic-runtime.local',
        '.\docker-compose.forensic-records.yaml',
        '.\docker-compose.forensic-records.runtime.yaml',
        '.\api\forensic_records\query.go',
        '.\api\forensic_records\query_language_normalization.go',
        '.\api\forensic_records\query_intelligence_contracts.go',
        '.\api\forensic_records\query_intelligence_composition.go',
        '.\api\forensic_records\query_intelligence_planner.go',
        '.\api\forensic_records\mmv2_query_test.go'
    )

    foreach ($file in $requiredFiles) {

        if (-not (Test-Path $file)) {
            throw "Required file missing: $file"
        }
    }

    Write-Marker 'RequiredRepositoryFiles=PASS'

    # Read-only Git inspection.
    $branch = git branch --show-current

    Assert-ExitCode 'Could not read Git branch.'

    $head = git rev-parse HEAD

    Assert-ExitCode 'Could not read Git HEAD.'

    $dirtyEntries = @(
        git status --porcelain
    ).Count

    Write-Marker "GitBranch=$branch"
    Write-Marker "GitHEAD=$head"
    Write-Marker "DirtyWorktreeEntries=$dirtyEntries"
    Write-Marker 'GitMutation=false'

    # Log source hashes without enforcing stale values.
    Write-Host ''
    Write-Host 'Source hashes:'

    Get-FileHash `
        '.\api\forensic_records\query.go',
        '.\api\forensic_records\query_language_normalization.go',
        '.\api\forensic_records\query_intelligence_contracts.go',
        '.\api\forensic_records\query_intelligence_composition.go',
        '.\api\forensic_records\query_intelligence_planner.go',
        '.\api\forensic_records\mmv2_query_test.go' `
        -Algorithm SHA256 |
        Format-Table Path, Hash -AutoSize

    # ========================================================================
    # 4. Compose validation
    # ========================================================================

    Write-Host ''
    Write-Host '=== 4. COMPOSE VALIDATION ==='

    $composeServices = @(
        docker compose @Compose config --services
    )

    Assert-ExitCode 'Compose configuration validation failed.'

    if ($composeServices -notcontains 'forensic-records-api') {
        throw 'Compose config does not define forensic-records-api.'
    }

    Write-Marker 'ComposeConfig=PASS'

    # ========================================================================
    # 5. Discover current runtime
    # ========================================================================

    Write-Host ''
    Write-Host '=== 5. CURRENT RUNTIME ==='

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
    Write-Marker "WorkerImageBefore=$beforeWorkerImage"

    Write-Marker "NatsContainerBefore=$beforeNats"
    Write-Marker "NatsImageBefore=$beforeNatsImage"

    Write-Marker "PostgresContainerBefore=$beforePostgres"
    Write-Marker "PostgresImageBefore=$beforePostgresImage"

    Write-Marker "LocalAIContainerBefore=$beforeLocalAI"
    Write-Marker "LocalAIImageBefore=$beforeLocalAIImage"

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
    # 6. Verify currently reported rollback baseline
    # ========================================================================

    Write-Host ''
    Write-Host '=== 6. CURRENT API BASELINE ==='

    $ReportedExpectedApiBaseline =
        'sha256:74c07cce1225d0694ca8342419af9f1994801aeacdb1c048a54ef9af854ed8a7'

    Write-Marker (
        "ReportedExpectedApiBaseline=" +
        $ReportedExpectedApiBaseline
    )

    Write-Marker (
        "ActualCurrentApiBaseline=" +
        $beforeApiImage
    )

    if ($beforeApiImage -ne $ReportedExpectedApiBaseline) {

        throw (
            'Current forensic API image does not match the latest ' +
            'reported NX-1 rollback baseline. ' +
            'Do not guess or replace it manually.'
        )
    }

    Write-Marker 'CurrentApiBaseline=PASS'

    # ========================================================================
    # 7. Retained data + active jobs
    # ========================================================================

    Write-Host ''
    Write-Host '=== 7. RETAINED STATE ==='

    $beforeTuple =
        Get-RetainedTuple $beforePostgres

    Write-Marker "RetainedTupleBefore=$beforeTuple"

    Assert-ZeroActiveJobs $beforeTuple

    if ($beforeTuple -eq '51|51|64|0|22207|441|47') {
        Write-Marker 'KnownRetainedBaseline=CONFIRMED'
    }
    else {
        Write-Warning (
            'The live retained tuple differs from the previously ' +
            'reported 51|51|64|0|22207|441|47. ' +
            'Deployment is stopped so the difference can be reviewed.'
        )

        throw 'Unexpected retained baseline.'
    }

    # ========================================================================
    # 8. Source verification
    # ========================================================================

    Write-Host ''
    Write-Host '=== 8. SOURCE TEST GATE ==='

    go test ./api/forensic_records `
        -run 'TestMMV2(VideoANPRRoutesAcrossLanguages|PublicEnterprise|EnterpriseResultState)' `
        -count=1

    Assert-ExitCode 'Focused NX-1 API regression suite failed.'

    Write-Marker 'FocusedApiTests=PASS'

    # Run the full package because this is intended to close NX-1.
    go test ./api/forensic_records -count=1

    Assert-ExitCode 'Full forensic API package test failed.'

    Write-Marker 'FullForensicApiTests=PASS'

    go vet ./api/forensic_records

    Assert-ExitCode 'go vet failed for forensic API.'

    Write-Marker 'GoVet=PASS'

    # ========================================================================
    # 9. Preflight-only exit
    # ========================================================================

    if ($PreflightOnly) {

        Write-Host ''

        Write-Marker 'NX1P13Preflight=PASS'
        Write-Marker 'MutationPerformed=false'
        Write-Marker 'NextAction=RUN_NORMAL_ACTIVATION'

        return
    }

    # ========================================================================
    # 10. Create fresh rollback tag
    # ========================================================================

    Write-Host ''
    Write-Host '=== 10. CREATE ROLLBACK TAG ==='

    docker image tag `
        $beforeApiImage `
        $RollbackTag

    Assert-ExitCode 'Could not create forensic API rollback tag.'

    $RollbackCreated = $true

    $rollbackImageID = docker image inspect `
        $RollbackTag `
        --format '{{.Id}}'

    Assert-ExitCode 'Could not inspect rollback tag.'

    $rollbackImageID = $rollbackImageID.Trim()

    Write-Marker "RollbackTag=$RollbackTag"
    Write-Marker "RollbackImageID=$rollbackImageID"

    if ($rollbackImageID -ne $beforeApiImage) {
        throw (
            'Rollback tag does not point to the exact running ' +
            'pre-activation API image.'
        )
    }

    Write-Marker 'RollbackTagVerified=PASS'

    # ========================================================================
    # 11. Build forensic-records-api ONLY
    # ========================================================================

    Write-Host ''
    Write-Host '=== 11. BUILD FORENSIC API ONLY ==='

    docker compose @Compose `
        --progress plain `
        build forensic-records-api

    Assert-ExitCode 'Forensic API build failed.'

    Write-Marker 'ForensicApiBuild=PASS'

    # IMPORTANT:
    # Inspect the actual tagged image after build.
    # Do NOT use `docker compose images -q` for verification because the
    # previous activation showed it could return an identifier representation
    # that did not match the running BuildKit manifest image.

    $newApiImage = docker image inspect `
        $ApiImageTag `
        --format '{{.Id}}'

    Assert-ExitCode (
        "Could not inspect newly built image $ApiImageTag."
    )

    $newApiImage = $newApiImage.Trim()

    if ([string]::IsNullOrWhiteSpace($newApiImage)) {
        throw 'New forensic API image ID is empty.'
    }

    Write-Marker "ForensicApiImageBefore=$beforeApiImage"
    Write-Marker "ForensicApiImageBuilt=$newApiImage"

    if ($newApiImage -eq $beforeApiImage) {
        throw (
            'The build completed but phase3-runtime still points to the ' +
            'same pre-activation image. Stop and verify whether the ' +
            'P1-3 source correction was actually incorporated.'
        )
    }

    Write-Marker 'BuiltImageVerification=PASS'

    # ========================================================================
    # 12. Recreate ONLY forensic-records-api
    # ========================================================================

    Write-Host ''
    Write-Host '=== 12. ACTIVATE FORENSIC API ONLY ==='

    $ActivationStarted = $true

    docker compose @Compose `
        up -d `
        --no-deps `
        --force-recreate `
        forensic-records-api

    Assert-ExitCode 'Forensic API recreation failed.'

    # ========================================================================
    # 13. Wait for API health
    # ========================================================================

    Write-Host ''
    Write-Host '=== 13. FORENSIC API HEALTH ==='

    Wait-ForHttp200 `
        -Url $ForensicHealthUrl `
        -TimeoutSeconds 240

    Write-Marker 'ForensicApiHealth=PASS'

    # ========================================================================
    # 14. Verify new running API image
    # ========================================================================

    Write-Host ''
    Write-Host '=== 14. VERIFY ACTIVE API IMAGE ==='

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
            'Running forensic API image does not match the actual ' +
            'new phase3-runtime image. ' +
            "Running=$afterApiImage Built=$newApiImage"
        )
    }

    Write-Marker 'ForensicApiImageActivation=PASS'

    # ========================================================================
    # 15. Verify protected services
    # ========================================================================

    Write-Host ''
    Write-Host '=== 15. PROTECTED SERVICE RECONCILIATION ==='

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

    Assert-ContainerRunning `
        -Name 'WorkerAfter' `
        -ContainerID $afterWorker

    Assert-ContainerRunning `
        -Name 'NatsAfter' `
        -ContainerID $afterNats

    Assert-ContainerRunning `
        -Name 'PostgresAfter' `
        -ContainerID $afterPostgres

    Assert-ContainerRunning `
        -Name 'LocalAIAfter' `
        -ContainerID $afterLocalAI

    Write-Marker 'ProtectedServicesPreserved=true'

    # ========================================================================
    # 16. Verify retained state
    # ========================================================================

    Write-Host ''
    Write-Host '=== 16. RETAINED STATE RECONCILIATION ==='

    $afterTuple =
        Get-RetainedTuple $afterPostgres

    Write-Marker "RetainedTupleBefore=$beforeTuple"
    Write-Marker "RetainedTupleAfter=$afterTuple"

    if ($afterTuple -ne $beforeTuple) {
        throw (
            'Retained state changed unexpectedly. ' +
            "Before=$beforeTuple After=$afterTuple"
        )
    }

    Write-Marker 'RetainedDataMutated=false'

    # ========================================================================
    # 17. Final Docker status
    # ========================================================================

    Write-Host ''
    Write-Host '=== 17. FINAL RUNTIME ==='

    docker ps `
        --format 'table {{.Names}}\t{{.Status}}\t{{.Image}}'

    # ========================================================================
    # 18. SUCCESS
    # ========================================================================

    Write-Host ''
    Write-Host '============================================================'
    Write-Host 'NX-1 P1-3 API-ONLY ACTIVATION SUCCESS'
    Write-Host '============================================================'

    Write-Marker 'NX1P13Activation=PASS'
    Write-Marker 'ForensicApiOnly=true'
    Write-Marker 'LocalAIPreserved=true'
    Write-Marker 'WorkerPreserved=true'
    Write-Marker 'NatsPreserved=true'
    Write-Marker 'PostgresPreserved=true'
    Write-Marker 'RetainedDataMutated=false'
    Write-Marker 'ModelsChanged=false'
    Write-Marker 'ProfilesChanged=false'
    Write-Marker 'BackendsChanged=false'
    Write-Marker 'NamedVolumesPreserved=true'
    Write-Marker 'DatabaseMigration=false'
    Write-Marker 'NextAction=RUN_NX1_P13_LIVE_REPLAY'
}
catch {

    $failureMessage = $_.Exception.Message

    Write-Host ''
    Write-Host '============================================================'
    Write-Host 'NX-1 P1-3 ACTIVATION ERROR'
    Write-Host '============================================================'

    Write-Error $failureMessage

    # ========================================================================
    # Rollback only if the running service was already recreated.
    # ========================================================================

    if (
        $ActivationStarted -and
        $RollbackCreated
    ) {

        $RollbackAttempted = $true

        Write-Warning (
            'Post-activation failure detected. ' +
            'Attempting forensic-API-only rollback.'
        )

        try {

            # Restore the phase3-runtime tag to the exact pre-activation
            # rollback image.
            docker image tag `
                $RollbackTag `
                $ApiImageTag

            Assert-ExitCode (
                'Failed to restore phase3-runtime from rollback tag.'
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

            # Verify protected services after rollback.
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

            Write-Marker "RetainedTupleAfterRollback=$rollbackTuple"

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

            Write-Marker 'NX1P13Activation=FAILED_ROLLED_BACK'
            Write-Marker 'ProtectedServicesPreserved=true'
            Write-Marker 'RetainedDataMutated=false'
            Write-Marker 'ModelsChanged=false'
            Write-Marker 'DatabaseMigration=false'
        }
        catch {

            Write-Error (
                'ROLLBACK ENCOUNTERED AN ERROR: ' +
                $_.Exception.Message
            )

            Write-Marker (
                'NX1P13Activation=FAILED_ROLLBACK_INCOMPLETE'
            )
        }
    }
    else {

        Write-Marker 'NX1P13Activation=FAILED_BEFORE_RUNTIME_CHANGE'
        Write-Marker 'RuntimeMutationPerformed=false'
    }

    exit 1
}
finally {

    try {
        Stop-Transcript | Out-Null
    }
    catch {
        # Transcript may never have started.
    }
}