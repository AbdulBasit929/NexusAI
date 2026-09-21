param(
  [string]$ArtifactRoot = 'C:\NexusAI-Evaluation\R8\Artifacts\1.0.0',
  [string]$T2VRoot = 'C:\NexusAI-Evaluation\R8\T2V\1.0.0',
  [string]$OutputRoot = 'C:\NexusAI-Evaluation\R8\Results\easyocr-arabic-g1\1.0.0',
  [switch]$SkipImageBuild,
  [switch]$PreflightOnly
)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
$image = 'nexusai/r8-easyocr-eval:1.7.2-cpu'
$candidateRoot = Join-Path $ArtifactRoot 'easyocr-arabic-g1-v1.7.2'
$archive = Join-Path $candidateRoot 'arabic.zip'
$manifest = Join-Path $T2VRoot 'manifest.json'
$contract = Join-Path $repoRoot 'configuration\nexusai_r8_t2v_contract.json'
$gate = Join-Path $repoRoot 'configuration\forensic_r8_model_approval_gate.json'
$taxonomy = Join-Path $repoRoot 'tests\fixtures\forensic_modalities\r8_image_fixture_taxonomy_v1.json'

function Get-R8ContainerId([string]$Name) {
  $id = & docker inspect --format '{{.Id}}' $Name 2>$null
  if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace(($id -join ''))) { throw "Required production container missing: $Name" }
  return ($id -join '').Trim()
}

$mutex = New-Object System.Threading.Mutex($false, 'Local\NexusAIR8EasyOCREvaluation')
if (-not $mutex.WaitOne(0)) { $mutex.Dispose(); throw 'Another EasyOCR evaluation is running' }
try {
  foreach ($path in @($archive, $manifest, $contract, $gate, $taxonomy)) {
    if (-not (Test-Path -LiteralPath $path -PathType Leaf)) { throw "Required file missing: $path" }
  }
  if ((Get-Item -LiteralPath $archive).Length -ne 199916962) { throw 'EasyOCR archive size failed' }
  & python (Join-Path $repoRoot 'scripts\validate_nexusai_r8_archive.py') --archive $archive --member arabic.pth --size 215400714 --algorithm MD5 --checksum 993074555550e4e06a6077d55ff0449a
  if ($LASTEXITCODE -ne 0) { throw 'EasyOCR archive member integrity failed' }
  & python (Join-Path $repoRoot 'scripts\validate_nexusai_r8_t2v_pack.py') --pack-root $T2VRoot --contract $contract
  if ($LASTEXITCODE -ne 0) { throw 'T2-V pack validation failed' }
  $dockerVersion = & docker info --format '{{.ServerVersion}}' 2>$null
  if ($LASTEXITCODE -ne 0) { throw 'Docker Desktop Linux engine is unavailable' }
  $before = @{}
  foreach ($name in @('nexusai-api-1','nexusai-forensic-records-api-1','nexusai-forensic-records-worker-1')) { $before[$name] = Get-R8ContainerId $name }
  if ($PreflightOnly) {
    Write-Host "R8EasyOCRPreflight=PASS Docker=$dockerVersion T2VValidated=true ModelIntegrity=true ProductionMutation=false"
    return
  }
  New-Item -ItemType Directory -Force -Path $OutputRoot | Out-Null
  if (-not $SkipImageBuild) {
    & docker build --progress plain -f tools/r8_challenger_eval/Dockerfile.easyocr -t $image .
    if ($LASTEXITCODE -ne 0) { throw 'EasyOCR evaluator image build failed' }
  } else {
    & docker image inspect $image *> $null
    if ($LASTEXITCODE -ne 0) { throw "Evaluator image missing: $image" }
  }
  $resolvedT2V = (Resolve-Path $T2VRoot).Path
  $resolvedCandidate = (Resolve-Path $candidateRoot).Path
  $resolvedOutput = (Resolve-Path $OutputRoot).Path
  $prediction = Join-Path $OutputRoot 'predictions.json'
  & docker run --rm --network none --cpus 4 --memory 4g `
    --mount "type=bind,source=$resolvedT2V,target=/fixtures,readonly" `
    --mount "type=bind,source=$resolvedCandidate,target=/model,readonly" `
    --mount "type=bind,source=$resolvedOutput,target=/results" `
    -e NEXUSAI_R8_OFFLINE=1 $image `
    --fixtures /fixtures/manifest.json --model-archive /model/arabic.zip --output /results/predictions.json
  if ($LASTEXITCODE -ne 0) { throw 'Offline EasyOCR inference failed' }
  foreach ($partition in @('development','validation','sealed_holdout')) {
    & python (Join-Path $repoRoot 'scripts\benchmark_r8_anpr_models.py') `
      --fixtures $manifest --predictions $prediction --output (Join-Path $OutputRoot "score-$partition.json") `
      --taxonomy $taxonomy --gate-config $gate --partition $partition --evaluation-mode benchmark
    if ($LASTEXITCODE -ne 0) { throw "Scoring failed: $partition" }
  }
  foreach ($name in $before.Keys) { if ($before[$name] -ne (Get-R8ContainerId $name)) { throw "Production container changed: $name" } }
  $scores = @{}
  foreach ($partition in @('development','validation','sealed_holdout')) { $scores[$partition] = Get-Content -Raw (Join-Path $OutputRoot "score-$partition.json") | ConvertFrom-Json }
  $bundle = Join-Path $OutputRoot 'evaluation-bundle.json'
  [ordered]@{
    contract_version = 'nexusai.r8-challenger-evaluation-bundle/v1'
    evaluated_at_utc = (Get-Date).ToUniversalTime().ToString('o')
    candidate = 'easyocr-arabic-g1-v1.7.2'
    offline = $true
    production_containers_unchanged = $true
    production_role_assignment = $false
    automatic_promotion = $false
    predictions_sha256 = (Get-FileHash -LiteralPath $prediction -Algorithm SHA256).Hash.ToLowerInvariant()
    scores = $scores
  } | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath $bundle -Encoding UTF8
  Write-Host "R8EasyOCREvaluation=PASS Bundle=$bundle Offline=true ProductionMutation=false AutomaticPromotion=false"
} finally {
  $mutex.ReleaseMutex()
  $mutex.Dispose()
}
