param(
  [string]$PilotRoot = 'C:\NexusAI-Evaluation\R8\T3Pilot\CCPD-demo-02aaea15',
  [string]$ArtifactRoot = 'C:\NexusAI-Evaluation\R8\Artifacts\1.0.0',
  [string]$OutputRoot = 'C:\NexusAI-Evaluation\R8\Results\fastplate-real-pilot\1.0.0',
  [switch]$SkipImageBuild,
  [switch]$PreflightOnly
)

$ErrorActionPreference='Stop'
Set-StrictMode -Version Latest
$repoRoot=Split-Path -Parent $PSScriptRoot
$image='nexusai/r8-fastplate-eval:1.1.0-cpu'
$candidateRoot=Join-Path $ArtifactRoot 'fast-plate-ocr-cct-s-v2-global-v1.1.0'
$model=Join-Path $candidateRoot 'cct_s_v2_global.onnx'
$config=Join-Path $candidateRoot 'cct_s_v2_global_plate_config.yaml'
$contract=Join-Path $repoRoot 'configuration\nexusai_r8_t3_diagnostic_pilot.json'
$receipt=Join-Path $PilotRoot 'acquisition-receipt.json'

function Get-R8ContainerId([string]$Name){$id=& docker inspect --format '{{.Id}}' $Name 2>$null;if($LASTEXITCODE-ne 0-or[string]::IsNullOrWhiteSpace(($id-join''))){throw "Required production container missing: $Name"};return($id-join'').Trim()}
$mutex=[Threading.Mutex]::new($false,'Local\NexusAIR8FastPlateDiagnosticPilot')
if(-not $mutex.WaitOne(0)){$mutex.Dispose();throw 'Another FastPlateOCR diagnostic pilot is running'}
try{
  foreach($path in @($model,$config,$contract,$receipt)){if(-not(Test-Path -LiteralPath $path -PathType Leaf)){throw "Required file missing: $path"}}
  if((Get-FileHash $model -Algorithm SHA256).Hash.ToLowerInvariant()-ne'384bbbd2cea3ef54761d3df70822ef3a349ee1a112aeafddbe0e3ba06bc6e47b'){throw 'Model integrity failed'}
  if((Get-FileHash $config -Algorithm SHA256).Hash.ToLowerInvariant()-ne'0335c74a305173bb6f393efed0fde03cadeaa0b649ed8e19f431016d8232d0a6'){throw 'Config integrity failed'}
  $before=@{};foreach($name in @('nexusai-api-1','nexusai-forensic-records-api-1','nexusai-forensic-records-worker-1')){$before[$name]=Get-R8ContainerId $name}
  if($PreflightOnly){Write-Host 'R8FastPlateDiagnosticPreflight=PASS Images=5 Authority=diagnostic_only ProductionMutation=false';return}
  New-Item -ItemType Directory -Force -Path $OutputRoot|Out-Null
  if(-not $SkipImageBuild){& docker build --progress plain -f tools/r8_challenger_eval/Dockerfile.fastplate -t $image .;if($LASTEXITCODE-ne 0){throw 'FastPlateOCR evaluator build failed'}}else{& docker image inspect $image *> $null;if($LASTEXITCODE-ne 0){throw "Evaluator image missing: $image"}}
  $resolvedPilot=(Resolve-Path $PilotRoot).Path;$resolvedCandidate=(Resolve-Path $candidateRoot).Path;$resolvedOutput=(Resolve-Path $OutputRoot).Path;$resolvedRepo=(Resolve-Path $repoRoot).Path
  & docker run --rm --network none --cpus 2 --memory 2g `
    --mount "type=bind,source=$resolvedPilot,target=/pilot,readonly" `
    --mount "type=bind,source=$resolvedCandidate,target=/model,readonly" `
    --mount "type=bind,source=$resolvedOutput,target=/results" `
    --mount "type=bind,source=$resolvedRepo\configuration,target=/contracts,readonly" `
    -e NEXUSAI_R8_OFFLINE=1 --entrypoint python $image /opt/nexusai-r8/run_fastplate_pilot.py `
    --pilot-root /pilot --contract /contracts/nexusai_r8_t3_diagnostic_pilot.json --receipt /pilot/acquisition-receipt.json `
    --model /model/cct_s_v2_global.onnx --config /model/cct_s_v2_global_plate_config.yaml --output /results/diagnostic.json
  if($LASTEXITCODE-ne 0){throw 'FastPlateOCR real-image diagnostic failed'}
  foreach($name in $before.Keys){if($before[$name]-ne(Get-R8ContainerId $name)){throw "Production container changed: $name"}}
  Write-Host "R8FastPlateDiagnostic=PASS Result=$(Join-Path $OutputRoot 'diagnostic.json') AccuracyAuthority=false ProductionMutation=false"
}finally{$mutex.ReleaseMutex();$mutex.Dispose()}
