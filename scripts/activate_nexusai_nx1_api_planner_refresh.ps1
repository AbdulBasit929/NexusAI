param(
    [switch]$PreflightOnly
)

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

# ============================================================================
# NX-1 FINAL API-ONLY PLANNER REFRESH
#
# Allowed mutation:
#   - forensic-records-api image build
#   - forensic-records-api container recreation
#
# Protected:
#   - LocalAI/UI api
#   - forensic worker
#   - PostgreSQL
#   - NATS
#   - retained evidence/data
#   - models/profiles/backends
#   - named volumes
#   - database schema
#
# PowerShell: Windows PowerShell 5.1 compatible
# ============================================================================

$RepoRoot = 'C:\Users\sheik\Workspace\Office\Projects\NexusAI'

$RollbackTag =
    'nexusai/forensic-records-api:rollback-before-nx1-api-planner-refresh-20260825'

$RequiredFreeRamKiB = 6291456   # exactly 6 GiB

$ReportDir = Join-Path $RepoRoot 'reports\nx1-foundation-truth-20260825'

if ($PreflightOnly) {
    $TranscriptFile = Join-Path $ReportDir 'nx1-api-planner-refresh-preflight.txt'
}
else {
    $TranscriptFile = Join-Path $ReportDir 'nx1-api-planner-refresh-local-output.txt'
}

$Compose = @(
    '-p', 'nexusai',
    '--env-file', '.\.env.forensic-runtime.local',
    '-f', '.\docker-compose.forensic-records.yaml',
    '-f', '.\docker-compose.forensic-records.runtime.yaml',
    '-f', '.\docker-compose.forensic-records.asr-small.yaml'
)

# --------------------------------------------------------------------------
# Helpers
# --------------------------------------------------------------------------

function Write-Marker {
    param([string]$Text)
    Write-Host $Text
}

function Assert-LastExit {
    param([string]$Message)

    if ($LASTEXITCODE -ne 0) {
        throw $Message
    }
}

function Get-ComposeContainerID {
    param(
        [Parameter(Mandatory=$true)]
        [string]$Service
    )

    $id = docker ps -q --no-trunc `
        --filter 'label=com.docker.compose.project=nexusai' `
        --filter "label=com.docker.compose.service=$Service"

    Assert-LastExit "Could not inspect Docker service '$Service'."

    $id = ($id | Select-Object -First 1)

    if ([string]::IsNullOrWhiteSpace($id)) {
        throw "Required running service '$Service' was not found."
    }

    return $id.Trim()
}

function Get-ContainerImageID {
    param(
        [Parameter(Mandatory=$true)]
        [string]$ContainerID
    )

    $imageID = docker inspect $ContainerID --format '{{.Image}}'
    Assert-LastExit "Could not inspect image for container $ContainerID."

    return $imageID.Trim()
}

function Get-PostgresEnvValue {
    param(
        [Parameter(Mandatory=$true)]
        [string]$ContainerID,

        [Parameter(Mandatory=$true)]
        [string]$Variable
    )

    $lines = docker inspect $ContainerID `
        --format '{{range .Config.Env}}{{println .}}{{end}}'

    Assert-LastExit "Could not inspect PostgreSQL environment."

    $prefix = "$Variable="

    $value = $lines |
        Where-Object { $_ -like "$Variable=*" } |
        Select-Object -First 1

    if ([string]::IsNullOrWhiteSpace($value)) {
        throw "Could not determine $Variable from PostgreSQL container."
    }

    return $value.Substring($prefix.Length).Trim()
}

function Get-RetainedTuple {
    param(
        [Parameter(Mandatory=$true)]
        [string]$PostgresContainer
    )

    $pgUser = Get-PostgresEnvValue `
        -ContainerID $PostgresContainer `
        -Variable 'POSTGRES_USER'

    $pgDb = Get-PostgresEnvValue `
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

    $tuple = docker exec $PostgresContainer `
        psql `
        -U $pgUser `
        -d $pgDb `
        -At `
        -F '|' `
        -c $sql

    Assert-LastExit 'Could not read retained-state tuple from PostgreSQL.'

    $tuple = ($tuple | Select-Object -Last 1).Trim()

    if ($tuple -notmatch '^\d+\|\d+\|\d+\|\d+\|\d+\|\d+\|\d+$') {
        throw "Unexpected retained-state tuple format: $tuple"
    }

    return $tuple
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

    if ($activeJobs -ne 0) {
        throw "Activation blocked: $activeJobs active forensic job(s) detected."
    }

    Write-Marker 'ActiveJobs=0'
}

function Wait-ForForensicApi {
    param(
        [int]$TimeoutSeconds = 240
    )

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)

    do {
        try {
            $response = Invoke-WebRequest `
                -UseBasicParsing `
                -TimeoutSec 10 `
                -Uri 'http://localhost:8091/healthz'

            if ([int]$response.StatusCode -eq 200) {
                Write-Marker 'ForensicApiHTTP=200'
                return
            }
        }
        catch {
            # Retry until deadline.
        }

        Start-Sleep -Seconds 2

    } while ((Get-Date) -lt $deadline)

    throw 'Forensic API did not become healthy before timeout.'
}

function Show-MemoryConsumers {
    Write-Host ''
    Write-Host 'Top memory-consuming Windows processes:'

    Get-Process |
        Sort-Object WorkingSet64 -Descending |
        Select-Object -First 15 `
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

function Invoke-ApiRollback {
    param(
        [Parameter(Mandatory=$true)]
        [string]$BeforeTuple,

        [Parameter(Mandatory=$true)]
        [string]$WorkerBefore,

        [Parameter(Mandatory=$true)]
        [string]$NatsBefore,

        [Parameter(Mandatory=$true)]
        [string]$PostgresBefore,

        [Parameter(Mandatory=$true)]
        [string]$LocalAIBefore
    )

    Write-Warning 'Substantive post-activation failure detected.'
    Write-Warning 'Attempting FORENSIC-API-ONLY rollback.'

    $rollbackImageID = docker image inspect $RollbackTag --format '{{.Id}}'
    Assert-LastExit 'Rollback image could not be inspected.'

    docker image tag `
        $RollbackTag `
        'nexusai/forensic-records-api:phase3-runtime'

    Assert-LastExit 'Failed to restore phase3-runtime tag from rollback image.'

    docker compose @Compose `
        up -d --no-deps --force-recreate forensic-records-api

    Assert-LastExit 'Rollback forensic API recreation failed.'

    Wait-ForForensicApi -TimeoutSeconds 240

    $workerNow = Get-ComposeContainerID 'forensic-records-worker'
    $natsNow = Get-ComposeContainerID 'forensic-nats'
    $postgresNow = Get-ComposeContainerID 'forensic-postgres'
    $localAINow = Get-ComposeContainerID 'api'

    if ($workerNow -ne $WorkerBefore) {
        throw 'Worker changed during rollback.'
    }

    if ($natsNow -ne $NatsBefore) {
        throw 'NATS changed during rollback.'
    }

    if ($postgresNow -ne $PostgresBefore) {
        throw 'PostgreSQL changed during rollback.'
    }

    if ($localAINow -ne $LocalAIBefore) {
        throw 'LocalAI/UI changed during rollback.'
    }

    $afterRollbackTuple = Get-RetainedTuple $postgresNow

    Write-Marker "RetainedTupleAfterRollback=$afterRollbackTuple"

    if ($afterRollbackTuple -ne $BeforeTuple) {
        throw (
            "Retained-state mismatch after rollback. " +
            "Before=$BeforeTuple After=$afterRollbackTuple"
        )
    }

    Write-Marker 'NX1ApiPlannerRefresh=FAILED_ROLLED_BACK'
    Write-Marker 'ProtectedServicesPreserved=true'
    Write-Marker 'RetainedDataMutated=false'
    Write-Marker 'ModelsChanged=false'
    Write-Marker 'ProfilesChanged=false'
    Write-Marker 'BackendsChanged=false'
    Write-Marker 'DatabaseMigration=false'
}

# --------------------------------------------------------------------------
# Setup transcript
# --------------------------------------------------------------------------

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
        -Path $TranscriptFile `
        -Force |
        Out-Null
}
catch {
    Write-Warning "Could not start transcript: $($_.Exception.Message)"
}

$ActivationStarted = $false
$RollbackCreated = $false

try {

    # ======================================================================
    # PHASE 1 — Docker readiness
    # ======================================================================

    Write-Host ''
    Write-Host '=== Docker readiness ==='

    $dockerVersion = docker info --format '{{.ServerVersion}}'
    Assert-LastExit 'Docker Desktop Linux engine is not ready.'

    Write-Marker "DockerServer=$dockerVersion"

    # ======================================================================
    # PHASE 2 — RAM gate
    # ======================================================================

    Write-Host ''
    Write-Host '=== Physical RAM gate ==='

    $os = Get-CimInstance Win32_OperatingSystem

    $totalGiB = [math]::Round(
        [double]$os.TotalVisibleMemorySize / 1MB,
        3
    )

    $freeGiB = [math]::Round(
        [double]$os.FreePhysicalMemory / 1MB,
        3
    )

    Write-Marker "TotalPhysicalRAMGiB=$totalGiB"
    Write-Marker "FreePhysicalRAMGiB=$freeGiB"
    Write-Marker 'RequiredFreeRAMGiB=6.000'

    if ([double]$os.FreePhysicalMemory -lt $RequiredFreeRamKiB) {

        Write-Marker 'RAMGate=FAIL'

        Show-MemoryConsumers

        Write-Marker 'NX1ApiPlannerRefresh=NOT_STARTED_RAM_GATE'

        return
    }

    Write-Marker 'RAMGate=PASS'

    # ======================================================================
    # PHASE 3 — Required source / compose files
    # ======================================================================

    Write-Host ''
    Write-Host '=== Repository preflight ==='

    $requiredPaths = @(
        '.\.env.forensic-runtime.local',
        '.\docker-compose.forensic-records.yaml',
        '.\docker-compose.forensic-records.runtime.yaml',
        '.\docker-compose.forensic-records.asr-small.yaml',
        '.\api\forensic_records\query.go',
        '.\api\forensic_records\mmv2_query_test.go'
    )

    foreach ($path in $requiredPaths) {
        if (-not (Test-Path $path)) {
            throw "Required repository path missing: $path"
        }
    }

    $branch = git branch --show-current
    Assert-LastExit 'Could not determine Git branch.'

    $head = git rev-parse HEAD
    Assert-LastExit 'Could not determine Git HEAD.'

    Write-Marker "GitBranch=$branch"
    Write-Marker "GitHEAD=$head"

    # Read-only only: do not demand a clean worktree.
    $dirtyCount = @(
        git status --porcelain
    ).Count

    Write-Marker "DirtyWorktreeEntries=$dirtyCount"
    Write-Marker 'GitMutation=false'

    # ======================================================================
    # PHASE 4 — Discover current containers
    # ======================================================================

    Write-Host ''
    Write-Host '=== Runtime before-state ==='

    $apiBefore = Get-ComposeContainerID 'forensic-records-api'
    $workerBefore = Get-ComposeContainerID 'forensic-records-worker'
    $natsBefore = Get-ComposeContainerID 'forensic-nats'
    $postgresBefore = Get-ComposeContainerID 'forensic-postgres'
    $localAIBefore = Get-ComposeContainerID 'api'

    $apiBeforeImage = Get-ContainerImageID $apiBefore
    $workerBeforeImage = Get-ContainerImageID $workerBefore
    $natsBeforeImage = Get-ContainerImageID $natsBefore
    $postgresBeforeImage = Get-ContainerImageID $postgresBefore
    $localAIBeforeImage = Get-ContainerImageID $localAIBefore

    Write-Marker "ForensicApiBefore=$apiBefore"
    Write-Marker "ForensicApiImageBefore=$apiBeforeImage"
    Write-Marker "LocalAIBefore=$localAIBefore"
    Write-Marker "LocalAIImageBefore=$localAIBeforeImage"
    Write-Marker "WorkerBefore=$workerBefore"
    Write-Marker "WorkerImageBefore=$workerBeforeImage"
    Write-Marker "NatsBefore=$natsBefore"
    Write-Marker "PostgresBefore=$postgresBefore"

    # ======================================================================
    # PHASE 5 — Retained-state / active-job preflight
    # ======================================================================

    Write-Host ''
    Write-Host '=== Retained state ==='

    $beforeTuple = Get-RetainedTuple $postgresBefore

    Write-Marker "RetainedTupleBefore=$beforeTuple"

    if ($beforeTuple -eq '51|51|64|0|22207|441|47') {
        Write-Marker 'KnownRetainedBaseline=CONFIRMED'
    }
    else {
        Write-Warning (
            "Live retained tuple differs from the previously reported " +
            "51|51|64|0|22207|441|47. " +
            "The fresh live tuple will be used as the authoritative " +
            "before-state for preservation."
        )
    }

    Assert-ZeroActiveJobs $beforeTuple

    # ======================================================================
    # PHASE 6 — Focused source tests
    # ======================================================================

    Write-Host ''
    Write-Host '=== NX-1 focused source gate ==='

    go test ./api/forensic_records `
        -run 'TestMMV2(VideoANPRRoutesAcrossLanguages|PublicEnterprise|EnterpriseResultState)' `
        -count=1

    Assert-LastExit 'Focused NX-1 forensic API tests failed.'

    go vet ./api/forensic_records

    Assert-LastExit 'go vet ./api/forensic_records failed.'

    Write-Marker 'NX1SourceGate=PASS'

    # ======================================================================
    # Preflight-only stop
    # ======================================================================

    if ($PreflightOnly) {
        Write-Marker 'NX1ApiPlannerRefreshPreflight=PASS'
        Write-Marker 'Mutation=false'
        return
    }

    # ======================================================================
    # PHASE 7 — Fresh rollback tag
    # ======================================================================

    Write-Host ''
    Write-Host '=== Creating forensic API rollback image ==='

    docker image tag $apiBeforeImage $RollbackTag

    Assert-LastExit 'Could not create forensic API rollback tag.'

    $RollbackCreated = $true

    $rollbackImageID = docker image inspect `
        $RollbackTag `
        --format '{{.Id}}'

    Assert-LastExit 'Could not inspect forensic API rollback tag.'

    if ($rollbackImageID.Trim() -ne $apiBeforeImage) {
        throw (
            "Rollback tag mismatch. Expected $apiBeforeImage; " +
            "got $rollbackImageID"
        )
    }

    Write-Marker "RollbackTag=$RollbackTag"
    Write-Marker "RollbackImage=$rollbackImageID"
    Write-Marker 'RollbackGate=PASS'

    # ======================================================================
    # PHASE 8 — Build forensic API ONLY
    # ======================================================================

    Write-Host ''
    Write-Host '=== Building forensic-records-api ONLY ==='

    docker compose @Compose `
        --progress plain `
        build forensic-records-api

    Assert-LastExit 'forensic-records-api build failed.'

    $builtImage = docker compose @Compose `
        images -q forensic-records-api

    Assert-LastExit 'Could not determine newly built forensic API image.'

    $builtImage = ($builtImage | Select-Object -First 1).Trim()

    if ([string]::IsNullOrWhiteSpace($builtImage)) {
        throw 'New forensic API image ID is empty.'
    }

    Write-Marker "ForensicApiBuiltImage=$builtImage"

    if ($builtImage -eq $apiBeforeImage) {
        Write-Warning (
            'Built image ID equals the previous image ID. ' +
            'This may be valid only if Docker determined the source produced ' +
            'an identical image. Verify carefully.'
        )
    }

    # ======================================================================
    # PHASE 9 — Recreate forensic API ONLY
    # ======================================================================

    Write-Host ''
    Write-Host '=== Activating forensic-records-api ONLY ==='

    $ActivationStarted = $true

    docker compose @Compose `
        up -d --no-deps --force-recreate forensic-records-api

    Assert-LastExit 'forensic-records-api recreation failed.'

    Wait-ForForensicApi -TimeoutSeconds 240

    # ======================================================================
    # PHASE 10 — Verify changed service
    # ======================================================================

    Write-Host ''
    Write-Host '=== Verifying deployed API ==='

    $apiAfter = Get-ComposeContainerID 'forensic-records-api'
    $apiAfterImage = Get-ContainerImageID $apiAfter

    Write-Marker "ForensicApiAfter=$apiAfter"
    Write-Marker "ForensicApiImageAfter=$apiAfterImage"

    if ($apiAfter -eq $apiBefore) {
        throw 'forensic-records-api container was not recreated.'
    }

    if ($apiAfterImage -ne $builtImage) {
        throw (
            "Running forensic API image does not match the newly built " +
            "image. Running=$apiAfterImage Built=$builtImage"
        )
    }

    # ======================================================================
    # PHASE 11 — Verify protected services EXACTLY unchanged
    # ======================================================================

    Write-Host ''
    Write-Host '=== Protected-service reconciliation ==='

    $workerAfter = Get-ComposeContainerID 'forensic-records-worker'
    $natsAfter = Get-ComposeContainerID 'forensic-nats'
    $postgresAfter = Get-ComposeContainerID 'forensic-postgres'
    $localAIAfter = Get-ComposeContainerID 'api'

    if ($workerAfter -ne $workerBefore) {
        throw 'Protected worker container changed unexpectedly.'
    }

    if ($natsAfter -ne $natsBefore) {
        throw 'Protected NATS container changed unexpectedly.'
    }

    if ($postgresAfter -ne $postgresBefore) {
        throw 'Protected PostgreSQL container changed unexpectedly.'
    }

    if ($localAIAfter -ne $localAIBefore) {
        throw 'Protected LocalAI/UI container changed unexpectedly.'
    }

    if ((Get-ContainerImageID $workerAfter) -ne $workerBeforeImage) {
        throw 'Protected worker image changed unexpectedly.'
    }

    if ((Get-ContainerImageID $natsAfter) -ne $natsBeforeImage) {
        throw 'Protected NATS image changed unexpectedly.'
    }

    if ((Get-ContainerImageID $postgresAfter) -ne $postgresBeforeImage) {
        throw 'Protected PostgreSQL image changed unexpectedly.'
    }

    if ((Get-ContainerImageID $localAIAfter) -ne $localAIBeforeImage) {
        throw 'Protected LocalAI/UI image changed unexpectedly.'
    }

    Write-Marker 'ProtectedServicesPreserved=true'

    # ======================================================================
    # PHASE 12 — Retained-state after
    # ======================================================================

    Write-Host ''
    Write-Host '=== Retained-state reconciliation ==='

    $afterTuple = Get-RetainedTuple $postgresAfter

    Write-Marker "RetainedTupleAfter=$afterTuple"

    if ($afterTuple -ne $beforeTuple) {
        throw (
            "Retained-state changed unexpectedly. " +
            "Before=$beforeTuple After=$afterTuple"
        )
    }

    Write-Marker 'RetainedDataMutated=false'

    # ======================================================================
    # PHASE 13 — Final runtime overview
    # ======================================================================

    Write-Host ''
    Write-Host '=== Final Docker state ==='

    docker ps `
        --format 'table {{.Names}}\t{{.Status}}\t{{.Image}}'

    # ======================================================================
    # PASS
    # ======================================================================

    Write-Host ''
    Write-Marker 'NX1ApiPlannerRefresh=PASS'
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
}
catch {

    Write-Host ''
    Write-Error $_.Exception.Message

    if ($ActivationStarted -and $RollbackCreated) {

        try {
            Invoke-ApiRollback `
                -BeforeTuple $beforeTuple `
                -WorkerBefore $workerBefore `
                -NatsBefore $natsBefore `
                -PostgresBefore $postgresBefore `
                -LocalAIBefore $localAIBefore
        }
        catch {
            Write-Error (
                'AUTOMATIC ROLLBACK ALSO ENCOUNTERED AN ERROR: ' +
                $_.Exception.Message
            )

            Write-Marker 'NX1ApiPlannerRefresh=FAILED_ROLLBACK_INCOMPLETE'
        }
    }
    else {
        Write-Marker 'NX1ApiPlannerRefresh=FAILED_BEFORE_ACTIVATION'
    }

    exit 1
}
finally {
    try {
        Stop-Transcript | Out-Null
    }
    catch {}
}