param(
  [ValidateSet('BOS', 'HuggingFace')][string]$ModelSource = 'BOS',
  [switch]$SkipImageBuild,
  [switch]$SkipModelDownload,
  [switch]$PreflightOnly
)

$ErrorActionPreference = 'Stop'
$gateMutex = New-Object System.Threading.Mutex($false, 'Local\NexusAIR8ModelEvaluation')
if (-not $gateMutex.WaitOne(0)) {
  $gateMutex.Dispose()
  throw 'Another NexusAI R8 model evaluation is already running'
}

$repoRoot = Split-Path -Parent $PSScriptRoot
$image = 'nexusai/r8-ocr-eval:3.7.0-cpu'
$cacheVolume = 'nexusai_r8_paddlex_model_cache'
$workRoot = Join-Path $repoRoot '.tmp\r8-model-evaluation'
$fixtureRoot = Join-Path $workRoot 'fixtures'
$resultRoot = Join-Path $workRoot 'results'
$reportRoot = Join-Path $repoRoot 'reports\runtime-evaluation-r8'
$candidates = @('tesseract', 'paddle-detector', 'paddle-en', 'paddle-arabic')

function Invoke-R8Docker {
  param([Parameter(Mandatory=$true)][string[]]$Arguments)
  & docker @Arguments
  if ($LASTEXITCODE -ne 0) { throw "Docker failed: docker $($Arguments -join ' ')" }
}

function Get-R8ContainerState {
  param([Parameter(Mandatory=$true)][string]$Name)
  $inspectionText = & docker inspect $Name 2>$null
  if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace(($inspectionText -join ''))) {
    throw "Required production container $Name is missing"
  }
  $inspection = ($inspectionText -join "`n") | ConvertFrom-Json
  $health = if ($null -ne $inspection[0].State.Health) { [string]$inspection[0].State.Health.Status } else { 'not_configured' }
  return [pscustomobject]@{
    ID = [string]$inspection[0].Id
    Status = [string]$inspection[0].State.Status
    Health = $health
    ExitCode = [int]$inspection[0].State.ExitCode
  }
}

function Assert-R8ContainerReady {
  param([Parameter(Mandatory=$true)][string]$Name)
  $state = Get-R8ContainerState $Name
  if ($state.Status -ne 'running' -or $state.Health -notin @('healthy', 'not_configured')) {
    throw "Production container $Name is not ready: status=$($state.Status) health=$($state.Health) exit_code=$($state.ExitCode)"
  }
  return $state
}

function Get-R8MountArguments {
  $resolvedWorkRoot = (Resolve-Path $workRoot).Path
  return @(
    '--mount', "type=bind,source=$resolvedWorkRoot,target=/work",
    '--mount', "type=volume,source=$cacheVolume,target=/root/.paddlex"
  )
}

Push-Location $repoRoot
try {
  $dockerVersion = & docker info --format '{{.ServerVersion}}' 2>$null
  if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($dockerVersion)) {
    throw 'Docker Desktop Linux engine is not ready'
  }
  $apiBefore = Assert-R8ContainerReady 'nexusai-forensic-records-api-1'
  $workerBefore = Assert-R8ContainerReady 'nexusai-forensic-records-worker-1'
  $localAIBefore = Assert-R8ContainerReady 'nexusai-api-1'
  $memoryCounter = New-Object System.Diagnostics.PerformanceCounter('Memory', 'Available MBytes')
  $availableMemoryGiB = [math]::Round($memoryCounter.NextValue() / 1024, 2)
  $memoryCounter.Dispose()
  $availableDiskGiB = [math]::Round((Get-PSDrive -Name (Split-Path $repoRoot -Qualifier).TrimEnd(':')).Free / 1GB, 2)
  if ($availableMemoryGiB -lt 3) {
    throw "R8 evaluation requires at least 3 GiB free RAM; available=$availableMemoryGiB GiB. Close memory-heavy applications and rerun."
  }
  if ($availableDiskGiB -lt 12) {
    throw "R8 evaluation requires at least 12 GiB free workspace-drive capacity; available=$availableDiskGiB GiB."
  }
  if ($PreflightOnly) {
    Write-Host "R8ModelEvaluationPreflight=PASS Docker=$dockerVersion FreeRAMGiB=$availableMemoryGiB FreeDiskGiB=$availableDiskGiB ProductionContainersResolved=true Mutation=false"
    return
  }

  New-Item -ItemType Directory -Force -Path $fixtureRoot, $resultRoot, $reportRoot | Out-Null
  $existingCacheVolume = docker volume ls --quiet --filter "name=^$cacheVolume`$"
  if ($LASTEXITCODE -ne 0) { throw 'Could not inspect Docker volumes for the isolated R8 model cache' }
  if ([string]::IsNullOrWhiteSpace(($existingCacheVolume -join ''))) {
    Invoke-R8Docker @('volume', 'create', '--label', 'nexusai.phase=R8-evaluation', $cacheVolume)
  }

  if (-not $SkipImageBuild) {
    Write-Host 'R8Stage=EvaluatorImageBuild Status=STARTED This stage can take several minutes'
    Invoke-R8Docker @('build', '--progress', 'plain', '-f', 'tools/r8_ocr_eval/Dockerfile', '-t', $image, '.')
  } else {
    Invoke-R8Docker @('image', 'inspect', $image)
  }

  $mounts = Get-R8MountArguments
  # Fixture creation and the Tesseract baseline are network-disabled.
  Invoke-R8Docker (@('run', '--rm', '--network', 'none') + $mounts + @('-e', 'NEXUSAI_R8_OFFLINE=1', $image, '--generate', '--candidate', 'tesseract'))

  if (-not $SkipModelDownload) {
    $sourceValue = if ($ModelSource -eq 'BOS') { 'BOS' } else { 'HUGGINGFACE' }
    # This is the only network-enabled stage. It populates only the isolated R8
    # cache volume and does not mount NexusAI evidence or production volumes.
    Write-Host "R8Stage=OfficialModelDownload Status=STARTED Source=$ModelSource This stage can take several minutes"
    Invoke-R8Docker (@('run', '--rm') + $mounts + @('-e', "PADDLE_PDX_MODEL_SOURCE=$sourceValue", $image, '--download-only'))
  }

  foreach ($candidate in @('paddle-detector', 'paddle-en', 'paddle-arabic')) {
    Write-Host "R8Stage=OfflineBenchmark Status=STARTED Candidate=$candidate"
    Invoke-R8Docker (@('run', '--rm', '--network', 'none') + $mounts + @('-e', 'NEXUSAI_R8_OFFLINE=1', $image, '--candidate', $candidate))
  }

  foreach ($candidate in $candidates) {
    $prediction = "/work/results/$candidate-predictions.json"
    $score = "/work/results/$candidate-score.json"
    Invoke-R8Docker (@('run', '--rm', '--network', 'none') + $mounts + @(
      '--entrypoint', 'python', $image, '/opt/nexusai-r8/score_benchmark.py',
      '--fixtures', '/work/fixtures/manifest.json', '--predictions', $prediction,
      '--output', $score, '--iou-threshold', '0.5',
      '--taxonomy', '/opt/nexusai-r8/image_fixture_taxonomy.json',
      '--gate-config', '/opt/nexusai-r8/model_approval_gate.json'
    ))
  }

  $stamp = (Get-Date).ToUniversalTime().ToString('yyyyMMddTHHmmssZ')
  $bundle = Join-Path $reportRoot "r8-model-evaluation-$stamp.json"
  $candidateResults = foreach ($candidate in $candidates) {
    $scorePath = Join-Path $resultRoot "$candidate-score.json"
    [ordered]@{
      candidate = $candidate
      predictions_sha256 = (Get-FileHash -Algorithm SHA256 (Join-Path $resultRoot "$candidate-predictions.json")).Hash.ToLowerInvariant()
      score_sha256 = (Get-FileHash -Algorithm SHA256 $scorePath).Hash.ToLowerInvariant()
      score = Get-Content -Raw $scorePath | ConvertFrom-Json
    }
  }
  $apiAfter = Assert-R8ContainerReady 'nexusai-forensic-records-api-1'
  $workerAfter = Assert-R8ContainerReady 'nexusai-forensic-records-worker-1'
  $localAIAfter = Assert-R8ContainerReady 'nexusai-api-1'
  if ($apiBefore.ID -ne $apiAfter.ID -or $workerBefore.ID -ne $workerAfter.ID -or $localAIBefore.ID -ne $localAIAfter.ID) {
    throw 'A production NexusAI container changed during the isolated model evaluation'
  }
  [ordered]@{
    contract_version = 'forensics.r8-model-evaluation-bundle/v1'
    evaluated_at_utc = (Get-Date).ToUniversalTime().ToString('o')
    image = $image
    model_source = $ModelSource
    offline_benchmark = $true
    fixtures_sha256 = (Get-FileHash -Algorithm SHA256 (Join-Path $fixtureRoot 'manifest.json')).Hash.ToLowerInvariant()
    production_containers_unchanged = $true
    production_role_assignment = $null
    decision = 'measured_results_require_review_no_automatic_promotion'
    candidates = @($candidateResults)
  } | ConvertTo-Json -Depth 20 | Set-Content -Encoding utf8 -LiteralPath $bundle
  $reviewBundle = Join-Path $reportRoot "r8-model-evaluation-review-$stamp.json"
  & python (Join-Path $repoRoot 'scripts\finalize_nexusai_r8_model_review.py') `
    --original-bundle $bundle --results $resultRoot --fixtures (Join-Path $fixtureRoot 'manifest.json') --output $reviewBundle
  if ($LASTEXITCODE -ne 0) { throw 'R8 corrected review-bundle generation failed' }
  Write-Host "R8ModelEvaluation=PASS Bundle=$bundle ProductionContainersUnchanged=true OfflineBenchmark=true AutomaticPromotion=false"
} finally {
  Pop-Location
  $gateMutex.ReleaseMutex()
  $gateMutex.Dispose()
}
