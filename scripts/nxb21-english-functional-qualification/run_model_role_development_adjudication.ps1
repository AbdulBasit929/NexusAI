# SPDX-License-Identifier: MIT
[CmdletBinding()]
param(
    [switch]$ValidateOnly,
    [switch]$CloseMemoryHeavyApps,
    [switch]$TemporarilyUnloadEmbedding,
    [switch]$ReclaimDockerFileCache,
    [ValidatePattern('^[A-Za-z0-9][A-Za-z0-9._-]{0,31}$')]
    [string]$AttemptId
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$root = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$library = Join-Path $PSScriptRoot 'run_current4b_final_characterization.ps1'
$modelRoleTest = Join-Path $root 'api\forensic_records\model_role_development_ginkgo_test.go'
$corpus = Join-Path $root 'api\forensic_records\contracts\model-role-development-corpus-v1.json'
$goCache = Join-Path $root '.tmp-go-cache'
$attemptSuffix = if ([string]::IsNullOrWhiteSpace($AttemptId)) { '' } else { '-' + $AttemptId }
$observationsPath = Join-Path $root "reports\nxb21\model-role-ram-admission${attemptSuffix}-20260916.ndjson"
$receiptPath = Join-Path $root "reports\nxb21\model-role-development-adjudication${attemptSuffix}-20260916.json"
$evidenceDir = Join-Path $root "reports\nxb21\model-role-development-evidence${attemptSuffix}-20260916"
$modelID = 'qwen_qwen3-4b-instruct-2507'
$embeddingModelID = 'qwen3-embedding-0.6b'
$apiBaseURL = 'http://127.0.0.1:8080'

$expectedHashes = [ordered]@{
    $library = 'EC0387052E0DE7041799F869341000469211CCAAEDA53DBF6BBCBBBA53B2470B'
    $modelRoleTest = '115284131F7166457539AA7E6ABA0CFE8B9071E1E8D543CB20A0703154129E70'
    $corpus = 'CB2A78BD8BCE28AABF15E17A2630879857F4CC0F9A62BD2B68FEF64222A8BB45'
}

foreach ($entry in $expectedHashes.GetEnumerator()) {
    if (-not (Test-Path -LiteralPath $entry.Key -PathType Leaf)) {
        throw "REQUIRED_FILE_MISSING:$($entry.Key)"
    }
    $actual = (Get-FileHash -LiteralPath $entry.Key -Algorithm SHA256).Hash
    if ($actual -cne $entry.Value) {
        throw "FROZEN_FILE_HASH_MISMATCH:$($entry.Key):expected=$($entry.Value):actual=$actual"
    }
}

if (-not (Test-Path -LiteralPath $goCache -PathType Container)) {
    throw "GO_CACHE_MISSING:$goCache"
}

$parseTokens = $null
$parseErrors = $null
$libraryAST = [Management.Automation.Language.Parser]::ParseFile(
    $library,
    [ref]$parseTokens,
    [ref]$parseErrors
)
if (@($parseErrors).Count -gt 0) {
    throw ('RESOURCE_LIBRARY_PARSE_FAILED:' + (($parseErrors | ForEach-Object Message) -join '; '))
}

$requiredContainers = @(
    'nexusai-api-1',
    'nexusai-forensic-records-api-1',
    'nexusai-forensic-records-worker-1',
    'nexusai-forensic-postgres-1',
    'nexusai-forensic-nats-1'
)
foreach ($container in $requiredContainers) {
    $state = (& docker inspect $container --format '{{.State.Running}}|{{.State.OOMKilled}}').Trim()
    if ($LASTEXITCODE -ne 0 -or $state -cne 'true|false') {
        throw "CONTAINER_NOT_READY:${container}:$state"
    }
}

$system = Invoke-RestMethod -Uri "$apiBaseURL/system" -TimeoutSec 20
$loadedModels = @($system.loaded_models | ForEach-Object { [string]$_.id })
if ($loadedModels -contains $modelID) {
    throw "TARGET_MODEL_MUST_START_UNLOADED:$modelID"
}

$registered = Invoke-RestMethod -Uri "$apiBaseURL/v1/models" -TimeoutSec 20
$registeredModels = @($registered.data | ForEach-Object { [string]$_.id })
if ($registeredModels -notcontains $modelID) {
    throw "TARGET_MODEL_NOT_REGISTERED:$modelID"
}

$artifactPath = '/models/Qwen_Qwen3-4B-Instruct-2507-Q8_0.gguf'
$profilePath = '/models/qwen_qwen3-4b-instruct-2507.yaml'
$artifactBytes = (& docker exec nexusai-api-1 stat -c '%s' $artifactPath).Trim()
if ($LASTEXITCODE -ne 0 -or $artifactBytes -cne '4280405216') {
    throw "MODEL_ARTIFACT_SIZE_MISMATCH:$artifactBytes"
}
$artifactHash = ((& docker exec nexusai-api-1 sha256sum $artifactPath) -split '\s+')[0]
if ($LASTEXITCODE -ne 0 -or $artifactHash -cne '260b5b5b6ad73e44df81a43ea1f5c11c37007b6bac18eb3cd2016e8667c19662') {
    throw "MODEL_ARTIFACT_HASH_MISMATCH:$artifactHash"
}
$profileHash = ((& docker exec nexusai-api-1 sha256sum $profilePath) -split '\s+')[0]
if ($LASTEXITCODE -ne 0 -or $profileHash -cne 'ef036e0a02cc5e8331dc4dff41c0e0cb68e6e0bda71d92b92f47f673472e666f') {
    throw "MODEL_PROFILE_HASH_MISMATCH:$profileHash"
}

Write-Host "MODEL_ROLE_PREFLIGHT=PASS model=$modelID artifact_bytes=$artifactBytes"

if ($ValidateOnly) {
    Write-Host ('MODEL_ROLE_VALIDATE_ONLY=PASS loaded_models=' + ($loadedModels -join ','))
    exit 0
}

if (Test-Path -LiteralPath $observationsPath) {
    throw "RAM_ADMISSION_ARTIFACT_ALREADY_EXISTS:$observationsPath"
}
if (Test-Path -LiteralPath $receiptPath) {
    throw "MODEL_ROLE_RECEIPT_ALREADY_EXISTS:$receiptPath"
}

$definitions = $libraryAST.FindAll(
    { param($node) $node -is [Management.Automation.Language.FunctionDefinitionAst] },
    $false
)
foreach ($definition in $definitions) {
    . ([scriptblock]::Create($definition.Extent.Text))
}

$embeddingWasUnloaded = $false
$embeddingRestoreFailure = $null
$goExitCode = $null
try {
    if ($CloseMemoryHeavyApps) {
        $closeTargets = @(
            'ChatGPT', 'Codex', 'codex', 'chrome', 'msedge', 'firefox', 'brave',
            'opera', 'Code', 'Teams', 'ms-teams', 'OUTLOOK', 'WINWORD', 'EXCEL',
            'POWERPNT', 'OneDrive', 'Slack', 'Discord', 'WhatsApp', 'Spotify',
            'Acrobat', 'AcroRd32', 'PowerToys'
        )
        Get-Process -ErrorAction SilentlyContinue |
            Where-Object {
                $_.MainWindowHandle -ne 0 -and
                $closeTargets -contains $_.ProcessName
            } |
            ForEach-Object {
                Write-Host "CLOSING_APP process=$($_.ProcessName) pid=$($_.Id)"
                [void]$_.CloseMainWindow()
            }
    }

    if ($TemporarilyUnloadEmbedding -and $loadedModels -contains $embeddingModelID) {
        $shutdownBody = @{ model = $embeddingModelID } | ConvertTo-Json -Compress
        Invoke-RestMethod `
            -Method Post `
            -Uri "$apiBaseURL/backend/shutdown" `
            -ContentType 'application/json' `
            -Body $shutdownBody `
            -TimeoutSec 30 | Out-Null
        $shutdownDeadline = [DateTime]::UtcNow.AddSeconds(30)
        do {
            Start-Sleep -Seconds 1
            $shutdownState = Invoke-RestMethod -Uri "$apiBaseURL/system" -TimeoutSec 20
            $shutdownLoaded = @($shutdownState.loaded_models | ForEach-Object { [string]$_.id })
        } while ($shutdownLoaded -contains $embeddingModelID -and [DateTime]::UtcNow -lt $shutdownDeadline)
        if ($shutdownLoaded -contains $embeddingModelID) {
            throw "EMBEDDING_MODEL_UNLOAD_TIMEOUT:$embeddingModelID"
        }
        $embeddingWasUnloaded = $true
        Write-Host "EMBEDDING_MEMORY_RECLAIM=PASS model=$embeddingModelID"
    }

    if ($CloseMemoryHeavyApps -or $embeddingWasUnloaded) {
        Start-Sleep -Seconds 20
    }

    if ($ReclaimDockerFileCache) {
        Invoke-GovernedCleanCacheReclaim
        Start-Sleep -Seconds 5
    }

    $window = @(Get-StableRAMWindow 'model_role_development' 8.627 120)
    Write-Host ('RAM_ADMISSION=PASS samples=' + ($window -join ','))

    $env:GOCACHE = (Resolve-Path -LiteralPath $goCache).Path
    $env:NXB21_MODEL_ROLE_LIVE = '1'
    $env:NXB21_MODEL_ROLE_OUTPUT = $receiptPath
    $env:NXB21_MODEL_ROLE_EVIDENCE_DIR = $evidenceDir

    Push-Location $root
    try {
        & go test ./api/forensic_records `
            '-run=^TestForensicRecordsSynthesis$' `
            '-ginkgo.focus=runs one opt-in installed-model development adjudication' `
            '-count=1' `
            '-timeout=15m'
        $goExitCode = $LASTEXITCODE
    }
    finally {
        Pop-Location
    }

    if ($goExitCode -ne 0) {
        throw "MODEL_ROLE_GO_TEST_FAILED:exit_code=$goExitCode"
    }
    if (-not (Test-Path -LiteralPath $receiptPath -PathType Leaf)) {
        throw "MODEL_ROLE_RECEIPT_MISSING_AFTER_TEST:$receiptPath"
    }
}
finally {
    if ($embeddingWasUnloaded) {
        try {
            $restoreBody = @{
                model = $embeddingModelID
                input = @('NexusAI transient runtime restoration probe')
            } | ConvertTo-Json -Compress
            Invoke-RestMethod `
                -Method Post `
                -Uri "$apiBaseURL/v1/embeddings" `
                -ContentType 'application/json' `
                -Body $restoreBody `
                -TimeoutSec 120 | Out-Null
            $restoreState = Invoke-RestMethod -Uri "$apiBaseURL/system" -TimeoutSec 20
            $restoreLoaded = @($restoreState.loaded_models | ForEach-Object { [string]$_.id })
            if ($restoreLoaded -notcontains $embeddingModelID) {
                throw 'embedding endpoint returned without a resident model'
            }
            Write-Host "EMBEDDING_RUNTIME_RESTORE=PASS model=$embeddingModelID"
        }
        catch {
            $embeddingRestoreFailure = $_.Exception.Message
            Write-Warning "EMBEDDING_RUNTIME_RESTORE=FAIL error=$embeddingRestoreFailure"
        }
    }
}

if ($null -ne $embeddingRestoreFailure) {
    throw "EMBEDDING_RUNTIME_RESTORE_FAILED:$embeddingRestoreFailure"
}

Write-Host "MODEL_ROLE_ADJUDICATION_EXECUTION=PASS receipt=$receiptPath"
exit 0
