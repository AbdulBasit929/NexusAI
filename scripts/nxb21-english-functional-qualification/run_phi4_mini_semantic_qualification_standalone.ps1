# SPDX-License-Identifier: MIT
[CmdletBinding()]
param([switch]$VerifyOnly)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$root = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$runner = Join-Path $PSScriptRoot 'run_phi4_mini_semantic_qualification.ps1'
$required = @(
    'nexusai-forensic-postgres-1',
    'nexusai-forensic-nats-1',
    'nexusai-api-1',
    'nexusai-forensic-records-api-1',
    'nexusai-forensic-records-worker-1'
)

function Test-DockerEngine {
    $null = & docker info --format '{{.ServerVersion}}' 2>$null
    return $LASTEXITCODE -eq 0
}

if (-not (Test-DockerEngine)) {
    $desktop = Join-Path $env:LOCALAPPDATA 'Programs\DockerDesktop\Docker Desktop.exe'
    if (-not (Test-Path -LiteralPath $desktop)) { throw 'DOCKER_DESKTOP_EXECUTABLE_NOT_FOUND' }
    Start-Process -FilePath $desktop -WindowStyle Hidden
    $deadline = [DateTime]::UtcNow.AddMinutes(3)
    do {
        Start-Sleep -Seconds 5
        if (Test-DockerEngine) { break }
    } while ([DateTime]::UtcNow -lt $deadline)
    if (-not (Test-DockerEngine)) { throw 'DOCKER_ENGINE_START_TIMEOUT' }
}

$decodedStates = & docker inspect @required 2>$null | ConvertFrom-Json
if ($LASTEXITCODE -ne 0) { throw 'NEXUSAI_REQUIRED_CONTAINERS_MISSING' }
$states = @()
foreach ($decodedState in $decodedStates) { $states += $decodedState }
if ($states.Count -ne $required.Count) { throw 'NEXUSAI_REQUIRED_CONTAINERS_MISSING' }
$stopped = @($states | Where-Object { $_.State.Status -ne 'running' } | ForEach-Object { $_.Name.TrimStart('/') })
if ($stopped.Count -gt 0) {
    $null = & docker start @stopped
    if ($LASTEXITCODE -ne 0) { throw 'NEXUSAI_CONTAINER_START_FAILED' }
}

$deadline = [DateTime]::UtcNow.AddMinutes(4)
do {
    $decodedStates = & docker inspect @required | ConvertFrom-Json
    $states = @()
    foreach ($decodedState in $decodedStates) { $states += $decodedState }
    $ready = $states.Count -eq $required.Count
    foreach ($state in $states) {
        $stateHealthProperty = $state.State.PSObject.Properties['Health']
        $configHealthProperty = $state.Config.PSObject.Properties['Healthcheck']
        $hasHealthcheck = $null -ne $configHealthProperty -and $null -ne $configHealthProperty.Value
        $health = if ($null -ne $stateHealthProperty -and $null -ne $stateHealthProperty.Value) { [string]$stateHealthProperty.Value.Status } else { 'none' }
        if ($state.State.Status -ne 'running' -or $state.State.OOMKilled -or $health -eq 'unhealthy' -or ($hasHealthcheck -and $health -ne 'healthy')) {
            $ready = $false
        }
    }
    if ($ready) { break }
    Start-Sleep -Seconds 10
} while ([DateTime]::UtcNow -lt $deadline)
if (-not $ready) { throw 'NEXUSAI_REQUIRED_SERVICES_NOT_HEALTHY' }

# The embedding backend is demand-loaded and is not used by this frozen
# semantic qualification. Unload it before admission so it cannot consume the
# physical reserve; the next embedding request can load it again normally.
$system = Invoke-RestMethod -Uri 'http://127.0.0.1:8080/system' -TimeoutSec 20
if (@($system.loaded_models | Where-Object { [string]$_.id -ceq 'qwen3-embedding-0.6b' }).Count -gt 0) {
    $body = @{ model = 'qwen3-embedding-0.6b' } | ConvertTo-Json -Compress
    Invoke-RestMethod -Method Post -Uri 'http://127.0.0.1:8080/backend/shutdown' -ContentType 'application/json' -Body $body -TimeoutSec 30 | Out-Null
    Start-Sleep -Seconds 8
    $system = Invoke-RestMethod -Uri 'http://127.0.0.1:8080/system' -TimeoutSec 20
    if (@($system.loaded_models | Where-Object { [string]$_.id -ceq 'qwen3-embedding-0.6b' }).Count -gt 0) { throw 'UNRELATED_EMBEDDING_MODEL_UNLOAD_FAILED' }
    Write-Host 'UNRELATED_EMBEDDING_MODEL_UNLOAD=PASS'
}

# These optional Windows shell companions are disposable and restart on demand.
# Protected applications and Docker/WSL processes are deliberately excluded.
Get-Process -Name 'OneDrive','Widgets','Microsoft.CmdPal.UI','LockApp','SearchHost','StartMenuExperienceHost','ShellExperienceHost','TextInputHost' -ErrorAction SilentlyContinue |
    Stop-Process -Force -ErrorAction SilentlyContinue
Get-Process -Name 'PowerToys*' -ErrorAction SilentlyContinue |
    Stop-Process -Force -ErrorAction SilentlyContinue

$memory = Get-CimInstance Win32_OperatingSystem
Write-Host ("PRE_RUN_AVAILABLE_RAM_GIB={0:N3}" -f (($memory.FreePhysicalMemory * 1KB) / 1GB))

$identity = @(& docker exec nexusai-api-1 sh -lc "stat -c '%s' /models/microsoft_Phi-4-mini-instruct-Q4_K_M.gguf; sha256sum /models/microsoft_Phi-4-mini-instruct-Q4_K_M.gguf /models/phi4-mini-instruct-nxb21d-semantic.yaml")
if ($LASTEXITCODE -ne 0 -or $identity.Count -ne 3) { throw 'PHI_MODEL_IDENTITY_UNAVAILABLE' }
if ([int64]$identity[0] -ne 2491874688) { throw 'PHI_ARTIFACT_SIZE_MISMATCH' }
if (($identity[1] -split '\s+')[0] -cne '01999f17c39cc3074afae5e9c539bc82d45f2dd7faa3917c66cbef76fce8c0c2') { throw 'PHI_ARTIFACT_SHA256_MISMATCH' }
if (($identity[2] -split '\s+')[0] -cne 'c4a409b6d456cf86731c3f204f193871139260e9f36cc6a15c93a765ff2a1a74') { throw 'PHI_PROFILE_SHA256_MISMATCH' }

# Confirm that LocalAI parsed and registered the profile before the governed
# runner is allowed to dispatch a model-load request. A profile copied into a
# live model volume may require an explicit config refresh.
$profileName = 'phi4-mini-instruct-nxb21d-semantic'
$profileUri = "http://127.0.0.1:8080/api/models/config-json/$profileName"
$profileConfig = $null
try {
    $profileConfig = Invoke-RestMethod -Method Get -Uri $profileUri -TimeoutSec 20
} catch {
    $reload = Invoke-RestMethod -Method Post -Uri 'http://127.0.0.1:8080/models/reload' -TimeoutSec 60
    $reloadSuccessProperty = $reload.PSObject.Properties['success']
    if ($null -eq $reloadSuccessProperty -or $reloadSuccessProperty.Value -ne $true) { throw 'PHI_PROFILE_RELOAD_FAILED' }
    $profileConfig = Invoke-RestMethod -Method Get -Uri $profileUri -TimeoutSec 20
}
$profileOptions = @($profileConfig.options)
if ([string]$profileConfig.name -cne $profileName -or [string]$profileConfig.backend -cne 'llama-cpp') { throw 'PHI_PROFILE_REGISTRATION_IDENTITY_MISMATCH' }
if ($profileOptions -notcontains 'use_jinja:true' -or $profileOptions -notcontains 'parallel:1') { throw 'PHI_PROFILE_REGISTRATION_OPTIONS_MISMATCH' }
if ([string]$profileConfig.parameters.model -cne 'microsoft_Phi-4-mini-instruct-Q4_K_M.gguf') { throw 'PHI_PROFILE_REGISTRATION_ARTIFACT_MISMATCH' }

Write-Host 'DOCKER_ENGINE=PASS'
Write-Host 'NEXUSAI_REQUIRED_SERVICES=PASS'
Write-Host 'PHI_ARTIFACT_IDENTITY=PASS'
Write-Host 'PHI_PROFILE_IDENTITY=PASS'
Write-Host 'PHI_PROFILE_REGISTRATION=PASS'

if ($VerifyOnly) {
    Write-Host 'STANDALONE_WRAPPER_VERIFICATION=PASS'
    Write-Host 'LIVE_INFERENCE=false'
    exit 0
}

& powershell.exe -NoProfile -ExecutionPolicy Bypass -File $runner -ReuseVerifiedBinary
exit $LASTEXITCODE
