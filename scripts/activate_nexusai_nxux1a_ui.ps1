param(
    [switch]$PreflightOnly
)

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

# ============================================================================
# NexusAI / Investigation Workspace
# NX-UX1A — UI-ONLY ACTIVATION
#
# ALLOWED:
#   - build Compose service "api"
#   - recreate Compose service "api"
#
# PROTECTED:
#   - forensic-records-api
#   - forensic-records-worker
#   - forensic-nats
#   - forensic-postgres
#   - models
#   - profiles
#   - backends
#   - named volumes
#   - retained evidence/data
#   - database schema
#
# Windows PowerShell 5.1 compatible.
# ============================================================================


# ============================================================================
# 0. CONFIGURATION
# ============================================================================

$RepoRoot =
    'C:\Users\sheik\Workspace\Office\Projects\NexusAI'

$RequiredFreeRamKiB = 6291456   # 6 GiB

$RollbackTag =
    'nexusai/localai-forensic:rollback-before-nxux1a-20260826'

$ReadyUrl =
    'http://localhost:8080/readyz'

$AnalystHomeUrl =
    'http://localhost:8080/analyst/home'

$ReportDir = Join-Path `
    $RepoRoot `
    'reports\nxux1-investigation-workspace-20260826'

if ($PreflightOnly) {

    $TranscriptPath = Join-Path `
        $ReportDir `
        'nxux1a-ui-preflight.txt'
}
else {

    $TranscriptPath = Join-Path `
        $ReportDir `
        'nxux1a-ui-activation.txt'
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

    Assert-ExitCode "Could not inspect service '$Service'."

    $ids = @(
        $ids |
        Where-Object {
            -not [string]::IsNullOrWhiteSpace($_)
        }
    )

    if ($ids.Count -ne 1) {

        throw (
            "Expected exactly one running '$Service' container; " +
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

    Assert-ExitCode "Could not inspect image for $ContainerID."

    if ([string]::IsNullOrWhiteSpace($value)) {
        throw "Image ID is empty for $ContainerID."
    }

    return $value.Trim()
}


function Get-ContainerConfiguredImage {

    param(
        [Parameter(Mandatory=$true)]
        [string]$ContainerID
    )

    $value = docker inspect `
        $ContainerID `
        --format '{{.Config.Image}}'

    Assert-ExitCode (
        "Could not inspect configured image for $ContainerID."
    )

    if ([string]::IsNullOrWhiteSpace($value)) {
        throw "Configured image reference is empty for $ContainerID."
    }

    return $value.Trim()
}


function Assert-Running {

    param(
        [Parameter(Mandatory=$true)]
        [string]$Name,

        [Parameter(Mandatory=$true)]
        [string]$ContainerID
    )

    $state = docker inspect `
        $ContainerID `
        --format '{{.State.Status}}|{{.State.Running}}'

    Assert-ExitCode "Could not inspect $Name."

    if ($state.Trim() -ne 'running|true') {
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

    Assert-ExitCode 'Could not inspect PostgreSQL environment.'

    $entry = $values |
        Where-Object {
            $_ -like "$Variable=*"
        } |
        Select-Object -First 1

    if ([string]::IsNullOrWhiteSpace($entry)) {
        throw "Could not resolve PostgreSQL variable $Variable."
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

    Assert-ExitCode 'Could not read retained-state tuple.'

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
            "UI activation stopped because $activeJobs " +
            "forensic job(s) are active."
        )
    }
}


function Wait-ForHttp200 {

    param(
        [Parameter(Mandatory=$true)]
        [string]$Url,

        [int]$TimeoutSeconds = 300,

        [string]$Accept = '*/*'
    )

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)

    while ((Get-Date) -lt $deadline) {

        try {

            $headers = @{
                Accept = $Accept
            }

            $response = Invoke-WebRequest `
                -UseBasicParsing `
                -TimeoutSec 15 `
                -Headers $headers `
                -Uri $Url

            if ([int]$response.StatusCode -eq 200) {

                Write-Marker "HTTP200=$Url"
                return $response
            }
        }
        catch {
            # Retry.
        }

        Start-Sleep -Seconds 3
    }

    throw "Timed out waiting for HTTP 200 from $Url."
}


function Show-MemoryConsumers {

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

function Get-ComposeDiscovery {

    param(
        [Parameter(Mandatory=$true)]
        [string]$ContainerID
    )

    $inspectJson = docker inspect $ContainerID

    Assert-ExitCode 'Could not inspect container for Compose discovery.'

    if ([string]::IsNullOrWhiteSpace($inspectJson)) {
        throw 'docker inspect returned no data.'
    }

    $inspect = $inspectJson | ConvertFrom-Json

    if ($null -eq $inspect -or $inspect.Count -lt 1) {
        throw 'Could not parse docker inspect result.'
    }

    $labels = $inspect[0].Config.Labels

    if ($null -eq $labels) {
        throw 'Container has no Docker Compose labels.'
    }

    $workingDir =
        $labels.'com.docker.compose.project.working_dir'

    $configFilesRaw =
        $labels.'com.docker.compose.project.config_files'

    if ([string]::IsNullOrWhiteSpace($workingDir)) {
        throw 'Compose working directory label is empty.'
    }

    if ([string]::IsNullOrWhiteSpace($configFilesRaw)) {
        throw 'Compose config-files label is empty.'
    }

    $files = @(
        $configFilesRaw -split ',' |
        ForEach-Object {
            $_.Trim()
        } |
        Where-Object {
            -not [string]::IsNullOrWhiteSpace($_)
        }
    )

    if ($files.Count -eq 0) {
        throw 'No Compose configuration files were discovered.'
    }

    return @{
        WorkingDir = $workingDir.Trim()
        ConfigFiles = $files
    }
}


function Get-ContainerEnvValue {

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

    Assert-ExitCode "Could not inspect environment for $ContainerID."

    $entry = $entries |
        Where-Object {
            $_ -like "$Variable=*"
        } |
        Select-Object -First 1

    if ([string]::IsNullOrWhiteSpace($entry)) {
        return $null
    }

    return $entry.Substring(("$Variable=").Length)
}
function Import-RequiredComposeEnvironment {

    param(
        [Parameter(Mandatory=$true)]
        [string]$ContainerID,

        [Parameter(Mandatory=$true)]
        [string[]]$ComposeFiles
    )

    # ------------------------------------------------------------------------
    # Read the effective environment of the already-running API container.
    # Do not print secret values.
    # ------------------------------------------------------------------------

    $containerEnvLines = @(
        docker inspect `
            $ContainerID `
            --format '{{range .Config.Env}}{{println .}}{{end}}'
    )

    Assert-ExitCode 'Could not inspect current api environment.'

    $containerEnv = @{}

    foreach ($line in $containerEnvLines) {

        if ([string]::IsNullOrWhiteSpace($line)) {
            continue
        }

        $parts = $line -split '=', 2

        if ($parts.Count -ne 2) {
            continue
        }

        $containerEnv[$parts[0]] = $parts[1]
    }


    # ------------------------------------------------------------------------
    # Discover variables explicitly required by Compose expressions such as:
    #
    #   ${VAR:?message}
    #   ${VAR?message}
    #
    # We do NOT import arbitrary container environment variables.
    # ------------------------------------------------------------------------

    $requiredNames = New-Object `
        'System.Collections.Generic.HashSet[string]' `
        ([System.StringComparer]::Ordinal)

    foreach ($composeFile in $ComposeFiles) {

        if (-not (Test-Path $composeFile)) {
            throw "Compose file does not exist: $composeFile"
        }

        $content = Get-Content `
            -Raw `
            -LiteralPath $composeFile

        $patterns = @(
            '\$\{([A-Za-z_][A-Za-z0-9_]*):\?[^}]*\}',
            '\$\{([A-Za-z_][A-Za-z0-9_]*)\?[^}]*\}'
        )

        foreach ($pattern in $patterns) {

            $matches = [regex]::Matches(
                $content,
                $pattern
            )

            foreach ($match in $matches) {

                [void]$requiredNames.Add(
                    $match.Groups[1].Value
                )
            }
        }
    }


    # ------------------------------------------------------------------------
    # Known compatibility mapping:
    #
    # Compose requires:
    #   NEXUSAI_AGENT_HISTORY_DATABASE_URL
    #
    # Current running API exposes:
    #   LOCALAI_AGENT_POOL_DATABASE_URL
    #
    # Preserve the current runtime value exactly.
    # ------------------------------------------------------------------------

    if (
        $requiredNames.Contains(
            'NEXUSAI_AGENT_HISTORY_DATABASE_URL'
        )
    ) {

        $existing =
            [Environment]::GetEnvironmentVariable(
                'NEXUSAI_AGENT_HISTORY_DATABASE_URL',
                'Process'
            )

        if ([string]::IsNullOrWhiteSpace($existing)) {

            if (
                $containerEnv.ContainsKey(
                    'NEXUSAI_AGENT_HISTORY_DATABASE_URL'
                )
            ) {

                [Environment]::SetEnvironmentVariable(
                    'NEXUSAI_AGENT_HISTORY_DATABASE_URL',
                    $containerEnv[
                        'NEXUSAI_AGENT_HISTORY_DATABASE_URL'
                    ],
                    'Process'
                )
            }
            elseif (
                $containerEnv.ContainsKey(
                    'LOCALAI_AGENT_POOL_DATABASE_URL'
                )
            ) {

                [Environment]::SetEnvironmentVariable(
                    'NEXUSAI_AGENT_HISTORY_DATABASE_URL',
                    $containerEnv[
                        'LOCALAI_AGENT_POOL_DATABASE_URL'
                    ],
                    'Process'
                )
            }
        }
    }


    # ------------------------------------------------------------------------
    # Recover same-name required variables from the running API container.
    # Do NOT overwrite a process variable that is already explicitly supplied.
    # ------------------------------------------------------------------------

    foreach ($name in $requiredNames) {

        $current =
            [Environment]::GetEnvironmentVariable(
                $name,
                'Process'
            )

        if (-not [string]::IsNullOrWhiteSpace($current)) {

            Write-Marker (
                "ComposeRequiredEnv:$name=ALREADY_AVAILABLE"
            )

            continue
        }


        if ($containerEnv.ContainsKey($name)) {

            [Environment]::SetEnvironmentVariable(
                $name,
                $containerEnv[$name],
                'Process'
            )

            Write-Marker (
                "ComposeRequiredEnv:$name=RECOVERED"
            )

            continue
        }


        throw (
            "Required Compose environment variable '$name' " +
            "is unavailable both in the current process and " +
            "the running api container. Stop rather than guessing."
        )
    }


    Write-Marker 'RequiredComposeEnvironment=PASS'
}
# ============================================================================
# 2. INITIALIZE
# ============================================================================

if (-not (Test-Path $RepoRoot)) {
    throw "Repository does not exist: $RepoRoot"
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


$RuntimeChanged = $false
$RollbackCreated = $false

$beforeUi = $null
$beforeUiImage = $null
$uiConfiguredImage = $null

$beforeForensic = $null
$beforeWorker = $null
$beforeNats = $null
$beforePostgres = $null

$beforeForensicImage = $null
$beforeWorkerImage = $null
$beforeNatsImage = $null
$beforePostgresImage = $null

$beforeTuple = $null

$ComposeArgs = $null
$ComposeWorkingDir = $null


try {

    # ========================================================================
    # 3. DOCKER READY
    # ========================================================================

    Write-Host ''
    Write-Host '=== 3. DOCKER READINESS ==='

    $dockerVersion = docker info --format '{{.ServerVersion}}'

    Assert-ExitCode 'Docker Desktop Linux engine is not ready.'

    Write-Marker "DockerServerVersion=$dockerVersion"


    # ========================================================================
    # 4. RAM GATE
    # ========================================================================

    Write-Host ''
    Write-Host '=== 4. RAM GATE ==='

    $os = Get-CimInstance Win32_OperatingSystem

    $totalGiB = [math]::Round(
        ([double]$os.TotalVisibleMemorySize / 1MB),
        3
    )

    $freeGiB = [math]::Round(
        ([double]$os.FreePhysicalMemory / 1MB),
        3
    )

    Write-Marker "TotalPhysicalRAMGiB=$totalGiB"
    Write-Marker "FreePhysicalRAMGiB=$freeGiB"
    Write-Marker 'RequiredFreeRAMGiB=6.000'

    if ([double]$os.FreePhysicalMemory -lt $RequiredFreeRamKiB) {

        Write-Marker 'RAMGate=FAIL'

        Show-MemoryConsumers

        Write-Marker 'NXUX1AActivation=NOT_STARTED_RAM_GATE'

        return
    }

    Write-Marker 'RAMGate=PASS'


    # ========================================================================
    # 5. REPOSITORY STATE
    # ========================================================================

    Write-Host ''
    Write-Host '=== 5. REPOSITORY STATE ==='

    $branch = git branch --show-current
    Assert-ExitCode 'Could not determine Git branch.'

    $head = git rev-parse HEAD
    Assert-ExitCode 'Could not determine Git HEAD.'

    $dirtyCount = @(
        git status --porcelain
    ).Count

    Write-Marker "GitBranch=$branch"
    Write-Marker "GitHEAD=$head"
    Write-Marker "DirtyWorktreeEntries=$dirtyCount"
    Write-Marker 'GitMutation=false'


    # ========================================================================
    # 6. CURRENT UI + PROTECTED RUNTIME
    # ========================================================================

    Write-Host ''
    Write-Host '=== 6. CURRENT RUNTIME ==='

    $beforeUi =
        Get-ComposeContainerID 'api'

    $beforeForensic =
        Get-ComposeContainerID 'forensic-records-api'

    $beforeWorker =
        Get-ComposeContainerID 'forensic-records-worker'

    $beforeNats =
        Get-ComposeContainerID 'forensic-nats'

    $beforePostgres =
        Get-ComposeContainerID 'forensic-postgres'


    $beforeUiImage =
        Get-ContainerImageID $beforeUi

    $uiConfiguredImage =
        Get-ContainerConfiguredImage $beforeUi

    $beforeForensicImage =
        Get-ContainerImageID $beforeForensic

    $beforeWorkerImage =
        Get-ContainerImageID $beforeWorker

    $beforeNatsImage =
        Get-ContainerImageID $beforeNats

    $beforePostgresImage =
        Get-ContainerImageID $beforePostgres


    Write-Marker "UIContainerBefore=$beforeUi"
    Write-Marker "UIImageBefore=$beforeUiImage"
    Write-Marker "UIConfiguredImage=$uiConfiguredImage"

    Write-Marker "ForensicApiBefore=$beforeForensic"
    Write-Marker "WorkerBefore=$beforeWorker"
    Write-Marker "NatsBefore=$beforeNats"
    Write-Marker "PostgresBefore=$beforePostgres"


    Assert-Running `
        -Name 'UIBefore' `
        -ContainerID $beforeUi

    Assert-Running `
        -Name 'ForensicApiBefore' `
        -ContainerID $beforeForensic

    Assert-Running `
        -Name 'WorkerBefore' `
        -ContainerID $beforeWorker

    Assert-Running `
        -Name 'NatsBefore' `
        -ContainerID $beforeNats

    Assert-Running `
        -Name 'PostgresBefore' `
        -ContainerID $beforePostgres


    # ========================================================================
    # 7. DISCOVER EXACT COMPOSE CONFIG
    # ========================================================================

    Write-Host ''
    Write-Host '=== 7. COMPOSE DISCOVERY ==='

    $composeDiscovery =
        Get-ComposeDiscovery $beforeUi

    $ComposeWorkingDir =
        $composeDiscovery.WorkingDir

    $configFiles =
        $composeDiscovery.ConfigFiles

    Write-Marker "ComposeWorkingDir=$ComposeWorkingDir"

    foreach ($file in $configFiles) {
        Write-Marker "ComposeConfigFile=$file"
    }


    $ComposeArgs = @(
        '-p',
        'nexusai'
    )

    foreach ($file in $configFiles) {

        $ComposeArgs += @(
            '-f',
            $file
        )
    }


    if (-not (Test-Path $ComposeWorkingDir)) {
        throw "Compose working directory does not exist: $ComposeWorkingDir"
    }

    Set-Location $ComposeWorkingDir
Import-RequiredComposeEnvironment `
    -ContainerID $beforeUi `
    -ComposeFiles $configFiles

    $composeServices = @(
        docker compose @ComposeArgs config --services
    )

    Assert-ExitCode 'Compose configuration validation failed.'

    if ($composeServices -notcontains 'api') {
        throw 'Compose configuration does not define service api.'
    }

    Write-Marker 'ComposeConfig=PASS'


    # ========================================================================
    # 8. RETAINED STATE
    # ========================================================================

    Write-Host ''
    Write-Host '=== 8. RETAINED STATE ==='

    $beforeTuple =
        Get-RetainedTuple $beforePostgres

    Write-Marker "RetainedTupleBefore=$beforeTuple"

    Assert-ZeroActiveJobs $beforeTuple

    if ($beforeTuple -eq '51|51|64|0|22207|441|47') {

        Write-Marker 'KnownRetainedBaseline=CONFIRMED'
    }
    else {

        Write-Warning (
            'The current retained tuple differs from the latest ' +
            'known NX-1/NX-UX1A baseline.'
        )

        throw (
            'Stop before UI activation so retained-state divergence ' +
            'can be reviewed.'
        )
    }


    # ========================================================================
    # 9. READ-ONLY CURRENT UI CHECK
    # ========================================================================

    Write-Host ''
    Write-Host '=== 9. CURRENT UI HEALTH ==='

    Wait-ForHttp200 `
        -Url $ReadyUrl `
        -TimeoutSeconds 60 |
        Out-Null

    Write-Marker 'CurrentUIReady=PASS'


    # ========================================================================
    # 10. PREFLIGHT-ONLY STOP
    # ========================================================================

    if ($PreflightOnly) {

        Write-Host ''
        Write-Host '============================================================'
        Write-Host 'NX-UX1A UI PREFLIGHT SUCCESS'
        Write-Host '============================================================'

        Write-Marker 'NXUX1APreflight=PASS'
        Write-Marker 'MutationPerformed=false'
        Write-Marker 'NextAction=RUN_NORMAL_ACTIVATION'

        return
    }


    # ========================================================================
    # 11. CREATE ROLLBACK TAG
    # ========================================================================

    Write-Host ''
    Write-Host '=== 11. CREATE UI ROLLBACK TAG ==='

    docker image tag `
        $beforeUiImage `
        $RollbackTag

    Assert-ExitCode 'Could not create UI rollback tag.'

    $RollbackCreated = $true


    $rollbackImage = docker image inspect `
        $RollbackTag `
        --format '{{.Id}}'

    Assert-ExitCode 'Could not inspect UI rollback tag.'

    $rollbackImage = $rollbackImage.Trim()

    Write-Marker "RollbackTag=$RollbackTag"
    Write-Marker "RollbackImage=$rollbackImage"

    if ($rollbackImage -ne $beforeUiImage) {

        throw (
            'UI rollback tag does not point to the exact ' +
            'currently running image.'
        )
    }

    Write-Marker 'RollbackTagVerified=PASS'


    # ========================================================================
    # 12. BUILD ONLY API
    # ========================================================================

    Write-Host ''
    Write-Host '=== 12. BUILD UI/API SERVICE ONLY ==='

    docker compose @ComposeArgs `
        --progress plain `
        build api

    Assert-ExitCode 'NX-UX1A api build failed.'

    Write-Marker 'NXUX1ABuild=PASS'


    # Resolve the image tag used by the running service.
    # This is more reliable than assuming a hardcoded repository/tag.
    $builtImage = docker image inspect `
        $uiConfiguredImage `
        --format '{{.Id}}'

    Assert-ExitCode (
        "Could not inspect built configured image $uiConfiguredImage."
    )

    $builtImage = $builtImage.Trim()

    Write-Marker "UIImageBefore=$beforeUiImage"
    Write-Marker "UIImageBuilt=$builtImage"

    if ([string]::IsNullOrWhiteSpace($builtImage)) {
        throw 'Built UI image ID is empty.'
    }

    if ($builtImage -eq $beforeUiImage) {

        throw (
            'Build completed but the configured api image ID did not ' +
            'change. Stop and verify whether NX-UX1A source was included.'
        )
    }

    Write-Marker 'BuiltUIImageVerification=PASS'


    # ========================================================================
    # 13. RECREATE ONLY API
    # ========================================================================

    Write-Host ''
    Write-Host '=== 13. ACTIVATE UI/API SERVICE ONLY ==='

    $RuntimeChanged = $true

    docker compose @ComposeArgs `
        up -d `
        --no-deps `
        --force-recreate `
        api

    Assert-ExitCode 'NX-UX1A api recreation failed.'


    # ========================================================================
    # 14. HEALTH
    # ========================================================================

    Write-Host ''
    Write-Host '=== 14. UI HEALTH ==='

    Wait-ForHttp200 `
        -Url $ReadyUrl `
        -TimeoutSeconds 300 |
        Out-Null

    Write-Marker 'UIReady=PASS'


    # Analyst route may return HTML.
    $analystResponse = Wait-ForHttp200 `
        -Url $AnalystHomeUrl `
        -TimeoutSeconds 120 `
        -Accept 'text/html'

    Write-Marker (
        "AnalystHomeHTTP=$($analystResponse.StatusCode)"
    )

    Write-Marker 'AnalystHomeRoute=PASS'


    # ========================================================================
    # 15. VERIFY NEW UI CONTAINER/IMAGE
    # ========================================================================

    Write-Host ''
    Write-Host '=== 15. VERIFY UI ACTIVATION ==='

    $afterUi =
        Get-ComposeContainerID 'api'

    $afterUiImage =
        Get-ContainerImageID $afterUi

    Write-Marker "UIContainerBefore=$beforeUi"
    Write-Marker "UIContainerAfter=$afterUi"

    Write-Marker "UIImageBefore=$beforeUiImage"
    Write-Marker "UIImageAfter=$afterUiImage"

    if ($afterUi -eq $beforeUi) {
        throw 'UI api container was not recreated.'
    }

    if ($afterUiImage -ne $builtImage) {

        throw (
            'Running UI does not use the newly built image. ' +
            "Running=$afterUiImage Built=$builtImage"
        )
    }

    Write-Marker 'UIActivationImage=PASS'


    # ========================================================================
    # 16. VERIFY PROTECTED SERVICES
    # ========================================================================

    Write-Host ''
    Write-Host '=== 16. PROTECTED SERVICES ==='

    $afterForensic =
        Get-ComposeContainerID 'forensic-records-api'

    $afterWorker =
        Get-ComposeContainerID 'forensic-records-worker'

    $afterNats =
        Get-ComposeContainerID 'forensic-nats'

    $afterPostgres =
        Get-ComposeContainerID 'forensic-postgres'


    if ($afterForensic -ne $beforeForensic) {
        throw 'Protected forensic API container changed.'
    }

    if ($afterWorker -ne $beforeWorker) {
        throw 'Protected worker container changed.'
    }

    if ($afterNats -ne $beforeNats) {
        throw 'Protected NATS container changed.'
    }

    if ($afterPostgres -ne $beforePostgres) {
        throw 'Protected PostgreSQL container changed.'
    }


    if (
        (Get-ContainerImageID $afterForensic) -ne
        $beforeForensicImage
    ) {
        throw 'Protected forensic API image changed.'
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

    Write-Marker 'ProtectedServicesPreserved=true'


    # ========================================================================
    # 17. RETAINED STATE AFTER
    # ========================================================================

    Write-Host ''
    Write-Host '=== 17. RETAINED STATE AFTER ACTIVATION ==='

    $afterTuple =
        Get-RetainedTuple $afterPostgres

    Write-Marker "RetainedTupleBefore=$beforeTuple"
    Write-Marker "RetainedTupleAfter=$afterTuple"

    if ($afterTuple -ne $beforeTuple) {

        throw (
            'Retained state changed unexpectedly during UI activation. ' +
            "Before=$beforeTuple After=$afterTuple"
        )
    }

    Write-Marker 'RetainedDataMutated=false'


    # Deployment safety passed.
    # Do not automatically rollback for later cosmetic/manual UX concerns.
    $RuntimeChanged = $false


    # ========================================================================
    # 18. FINAL SERVICE STATUS
    # ========================================================================

    Write-Host ''
    Write-Host '=== 18. FINAL DOCKER STATUS ==='

    docker ps `
        --format 'table {{.Names}}\t{{.Status}}\t{{.Image}}'


    # ========================================================================
    # 19. SUCCESS
    # ========================================================================

    Write-Host ''
    Write-Host '============================================================'
    Write-Host 'NX-UX1A UI ACTIVATION SUCCESS'
    Write-Host '============================================================'

    Write-Marker 'NXUX1AActivation=PASS'
    Write-Marker 'UIServiceOnly=true'

    Write-Marker 'ForensicApiPreserved=true'
    Write-Marker 'WorkerPreserved=true'
    Write-Marker 'NatsPreserved=true'
    Write-Marker 'PostgresPreserved=true'

    Write-Marker 'RetainedDataMutated=false'
    Write-Marker 'ModelsChanged=false'
    Write-Marker 'ProfilesChanged=false'
    Write-Marker 'BackendsChanged=false'
    Write-Marker 'NamedVolumesPreserved=true'
    Write-Marker 'DatabaseMigration=false'

    Write-Marker 'InvestigationWorkspaceURL=http://localhost:8080/analyst/home'
    Write-Marker 'NextAction=VISUALLY_REVIEW_NXUX1A'
}
catch {

    $failure = $_.Exception.Message

    Write-Host ''
    Write-Host '============================================================'
    Write-Host 'NX-UX1A ACTIVATION ERROR'
    Write-Host '============================================================'

    Write-Error $failure


    # ========================================================================
    # AUTOMATIC ROLLBACK ONLY FOR A POST-RECREATE DEPLOYMENT-SAFETY FAILURE
    # ========================================================================

    if (
        $RuntimeChanged -and
        $RollbackCreated
    ) {

        Write-Warning (
            'Deployment-safety failure detected. ' +
            'Attempting UI/API-only rollback.'
        )

        try {

            # Restore the image reference originally used by the API service.
            docker image tag `
                $RollbackTag `
                $uiConfiguredImage

            Assert-ExitCode (
                'Could not restore UI configured image from rollback tag.'
            )


            docker compose @ComposeArgs `
                up -d `
                --no-deps `
                --force-recreate `
                api

            Assert-ExitCode 'UI rollback recreation failed.'


            Wait-ForHttp200 `
                -Url $ReadyUrl `
                -TimeoutSeconds 300 |
                Out-Null


            $rollbackUi =
                Get-ComposeContainerID 'api'

            $rollbackUiImage =
                Get-ContainerImageID $rollbackUi

            if ($rollbackUiImage -ne $beforeUiImage) {

                throw (
                    'Rollback UI is not running the original ' +
                    'pre-activation image.'
                )
            }


            # Protected service verification after rollback.
            $rollbackForensic =
                Get-ComposeContainerID 'forensic-records-api'

            $rollbackWorker =
                Get-ComposeContainerID 'forensic-records-worker'

            $rollbackNats =
                Get-ComposeContainerID 'forensic-nats'

            $rollbackPostgres =
                Get-ComposeContainerID 'forensic-postgres'


            if ($rollbackForensic -ne $beforeForensic) {
                throw 'Forensic API changed during rollback.'
            }

            if ($rollbackWorker -ne $beforeWorker) {
                throw 'Worker changed during rollback.'
            }

            if ($rollbackNats -ne $beforeNats) {
                throw 'NATS changed during rollback.'
            }

            if ($rollbackPostgres -ne $beforePostgres) {
                throw 'PostgreSQL changed during rollback.'
            }


            $rollbackTuple =
                Get-RetainedTuple $rollbackPostgres

            if ($rollbackTuple -ne $beforeTuple) {

                throw (
                    'Retained state differs after rollback. ' +
                    "Before=$beforeTuple After=$rollbackTuple"
                )
            }


            Write-Marker 'NXUX1AActivation=FAILED_ROLLED_BACK'
            Write-Marker 'ProtectedServicesPreserved=true'
            Write-Marker 'RetainedDataMutated=false'
        }
        catch {

            Write-Error (
                'ROLLBACK ALSO ENCOUNTERED AN ERROR: ' +
                $_.Exception.Message
            )

            Write-Marker (
                'NXUX1AActivation=FAILED_ROLLBACK_INCOMPLETE'
            )
        }
    }
    else {

        Write-Marker 'NXUX1AActivation=FAILED_BEFORE_RUNTIME_CHANGE'
        Write-Marker 'RuntimeMutationPerformed=false'
    }

    exit 1
}
finally {

    try {
        Stop-Transcript | Out-Null
    }
    catch {
        # Transcript might not have started.
    }
}