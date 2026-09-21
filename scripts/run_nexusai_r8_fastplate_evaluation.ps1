param(
  [string]$ArtifactRoot = 'C:\NexusAI-Evaluation\R8\Artifacts\1.0.0',
  [string]$T2VRoot = 'C:\NexusAI-Evaluation\R8\T2V\1.0.0',
  [string]$OutputRoot = 'C:\NexusAI-Evaluation\R8\Results\fastplate-cct-s-v2-global\1.0.0',
  [switch]$SkipImageBuild,
  [switch]$PreflightOnly
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repoRoot = Split-Path -Parent $PSScriptRoot
$image = 'nexusai/r8-fastplate-eval:1.1.0-cpu'
$candidateRoot = Join-Path $ArtifactRoot 'fast-plate-ocr-cct-s-v2-global-v1.1.0'
$model = Join-Path $candidateRoot 'cct_s_v2_global.onnx'
$config = Join-Path $candidateRoot 'cct_s_v2_global_plate_config.yaml'
$manifest = Join-Path $T2VRoot 'manifest.json'
$contract = Join-Path $repoRoot 'configuration\nexusai_r8_t2v_contract.json'
$gate = Join-Path $repoRoot 'configuration\forensic_r8_model_approval_gate.json'
$taxonomy = Join-Path $repoRoot 'tests\fixtures\forensic_modalities\r8_image_fixture_taxonomy_v1.json'

function Get-R8ContainerId([string]$Name) {
  $id = & docker inspect --format '{{.Id}}' $Name 2>$null
  if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace(($id -join ''))) { throw "Required production container missing: $Name" }
  return ($id -join '').Trim()
}

$mutex = [Threading.Mutex]::new($false, 'Local\NexusAIR8FastPlateEvaluation')
if (-not $mutex.WaitOne(0)) { $mutex.Dispose(); throw 'Another FastPlateOCR evaluation is running' }
try {
  foreach ($path in @($model,$config,$manifest,$contract,$gate,$taxonomy)) {
    if (-not (Test-Path -LiteralPath $path -PathType Leaf)) { throw "Required file missing: $path" }
  }
  if ((Get-FileHash $model -Algorithm SHA256).Hash.ToLowerInvariant() -ne '384bbbd2cea3ef54761d3df70822ef3a349ee1a112aeafddbe0e3ba06bc6e47b') { throw 'FastPlateOCR model integrity failed' }
  if ((Get-FileHash $config -Algorithm SHA256).Hash.ToLowerInvariant() -ne '0335c74a305173bb6f393efed0fde03cadeaa0b649ed8e19f431016d8232d0a6') { throw 'FastPlateOCR config integrity failed' }
  & python (Join-Path $repoRoot 'scripts\validate_nexusai_r8_t2v_pack.py') --pack-root $T2VRoot --contract $contract
  if ($LASTEXITCODE -ne 0) { throw 'T2-V pack validation failed' }
  $dockerVersion = & docker info --format '{{.ServerVersion}}' 2>$null
  if ($LASTEXITCODE -ne 0) { throw 'Docker Desktop Linux engine unavailable' }
  $before=@{}
  foreach($name in @('nexusai-api-1','nexusai-forensic-records-api-1','nexusai-forensic-records-worker-1')) { $before[$name]=Get-R8ContainerId $name }
  if ($PreflightOnly) {
    Write-Host "R8FastPlatePreflight=PASS Docker=$dockerVersion T2VValidated=true ModelIntegrity=true Provider=CPUExecutionProvider Threads=2 ProductionMutation=false"
    return
  }

  New-Item -ItemType Directory -Force -Path $OutputRoot | Out-Null
  if (-not $SkipImageBuild) {
    & docker build --progress plain -f tools/r8_challenger_eval/Dockerfile.fastplate -t $image .
    if ($LASTEXITCODE -ne 0) { throw 'FastPlateOCR evaluator build failed' }
  } else {
    & docker image inspect $image *> $null
    if ($LASTEXITCODE -ne 0) { throw "Evaluator image missing: $image" }
  }
  $imageDigest = (& docker image inspect --format '{{.Id}}' $image).Trim()
  $resolvedT2V=(Resolve-Path $T2VRoot).Path
  $resolvedCandidate=(Resolve-Path $candidateRoot).Path
  $resolvedOutput=(Resolve-Path $OutputRoot).Path
  $prediction=Join-Path $OutputRoot 'predictions.json'
  & docker run --rm --network none --cpus 2 --memory 2g `
    --mount "type=bind,source=$resolvedT2V,target=/fixtures,readonly" `
    --mount "type=bind,source=$resolvedCandidate,target=/model,readonly" `
    --mount "type=bind,source=$resolvedOutput,target=/results" `
    -e NEXUSAI_R8_OFFLINE=1 $image --fixtures /fixtures/manifest.json `
    --model /model/cct_s_v2_global.onnx --config /model/cct_s_v2_global_plate_config.yaml `
    --threads 2 --output /results/predictions.json
  if ($LASTEXITCODE -ne 0) { throw 'Offline FastPlateOCR inference failed' }
  foreach($partition in @('development','validation','sealed_holdout')) {
    & python (Join-Path $repoRoot 'scripts\benchmark_r8_anpr_models.py') --fixtures $manifest --predictions $prediction --output (Join-Path $OutputRoot "score-$partition.json") --taxonomy $taxonomy --gate-config $gate --partition $partition --evaluation-mode benchmark
    if ($LASTEXITCODE -ne 0) { throw "Scoring failed: $partition" }
  }
  foreach($name in $before.Keys) {
    if($before[$name] -ne (Get-R8ContainerId $name)) { throw "Production container changed: $name" }
  }
  $scores=@{}
  foreach($partition in @('development','validation','sealed_holdout')) { $scores[$partition]=Get-Content -Raw (Join-Path $OutputRoot "score-$partition.json") | ConvertFrom-Json }
  $bundle=Join-Path $OutputRoot 'evaluation-bundle.json'
  [ordered]@{
    contract_version='nexusai.r8-challenger-evaluation-bundle/v1'
    evaluated_at_utc=(Get-Date).ToUniversalTime().ToString('o')
    candidate='fast-plate-ocr-cct-s-v2-global-v1.1.0'
    hardware_profile_id='D0-20260813-i7-1260P-docker-cpu'
    execution_provider='CPUExecutionProvider'
    evaluator_image_digest=$imageDigest
    offline=$true
    production_containers_unchanged=$true
    production_role_assignment=$false
    automatic_promotion=$false
    predictions_sha256=(Get-FileHash $prediction -Algorithm SHA256).Hash.ToLowerInvariant()
    scores=$scores
  } | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath $bundle -Encoding UTF8
  Write-Host "R8FastPlateEvaluation=PASS Bundle=$bundle Offline=true ProductionMutation=false AutomaticPromotion=false"
} finally {
  $mutex.ReleaseMutex()
  $mutex.Dispose()
}
