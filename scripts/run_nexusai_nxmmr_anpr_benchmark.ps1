[CmdletBinding()]
param(
    [string]$ImageRoot = (Join-Path $env:USERPROFILE 'Downloads\archive\Pakistani License Number Plates Data'),
    [string]$Video = (Join-Path $env:USERPROFILE 'Downloads\sample.mp4'),
    [string]$WorkerImage = 'nexusai/forensic-records-worker:phase3-runtime'
)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
$oracleRoot = Join-Path $repoRoot 'local-acceptance-models\nxmmr\private-ground-truth\human-verification'
$modelRoot = Join-Path $repoRoot 'local-acceptance-models\nxmmr'
$resultRoot = Join-Path $repoRoot 'local-acceptance-models\nxmmr\private-benchmarks\anpr-human-gold'
$evaluator = Join-Path $PSScriptRoot 'benchmark_nexusai_nxmmr_anpr.py'
$developmentResult = Join-Path $resultRoot 'image-development.json'
$holdoutResult = Join-Path $resultRoot 'image-sealed-holdout.json'
$videoResult = Join-Path $resultRoot 'sample-mp4-video.json'
$expectedGoldDigest = '88c6827c23fbed467f097843404ed09c947bf7f27432e5cb5fa8427be9f48567'
$expectedVideoDigest = 'd470773444dd23bc4b8d446e3fb1e057923e9902b2a2a93e9fafc83762d137ee'
$modelDigests = @{
    'yolo-v9-t-384-license-plates-end2end.onnx' = '888397b96d761c89db40bc9c305838e8652660f5e282c2cadebbe8d2951a77a8'
    'cct_xs_v2_global.onnx' = '8031afb5fdc6b4d80462c9d542f1284ebd2cfddf5dbacd62609848d7e2855f44'
    'cct_xs_v2_global_plate_config.yaml' = '0335c74a305173bb6f393efed0fde03cadeaa0b649ed8e19f431016d8232d0a6'
}

function Assert-Hash([string]$Path, [string]$Expected) {
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        throw "Required bounded benchmark input is missing: $Path"
    }
    $actual = (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($actual -ne $Expected) {
        throw "Integrity mismatch for $Path"
    }
}

function Invoke-IsolatedBenchmark([string[]]$Command) {
    $name = 'nxmmr-anpr-' + [Guid]::NewGuid().ToString('N')
    $arguments = @(
        'run', '--rm', '--name', $name, '--network', 'none', '--read-only',
        '--cpus', '4', '--memory', '2g', '--pids-limit', '512',
        '--tmpfs', '/tmp:rw,noexec,nosuid,size=1g',
        '--workdir', '/app',
        '--env', 'PYTHONPATH=/app',
        '--env', 'FORENSIC_ANPR_DETECTOR_MODEL_PATH=/models/nxmmr/yolo-v9-t-384-license-plates-end2end.onnx',
        '--env', 'FORENSIC_ANPR_OCR_MODEL_PATH=/models/nxmmr/cct_xs_v2_global.onnx',
        '--env', 'FORENSIC_ANPR_OCR_CONFIG_PATH=/models/nxmmr/cct_xs_v2_global_plate_config.yaml',
        '--env', 'FORENSIC_ANPR_ONNX_PROVIDERS=OpenVINOExecutionProvider,CPUExecutionProvider',
        '--env', 'FORENSIC_ANPR_MIN_DETECTION_CONFIDENCE=0.75',
        '--env', 'NXMMR_CPU_LIMIT=4', '--env', 'NXMMR_MEMORY_LIMIT=2g',
        '--mount', "type=bind,src=$evaluator,dst=/benchmark/benchmark_nexusai_nxmmr_anpr.py,readonly",
        '--mount', "type=bind,src=$oracleRoot,dst=/oracle,readonly",
        '--mount', "type=bind,src=$modelRoot,dst=/models/nxmmr,readonly",
        '--mount', "type=bind,src=$ImageRoot,dst=/evidence/images,readonly",
        '--mount', "type=bind,src=$Video,dst=/evidence/sample.mp4,readonly",
        '--mount', "type=bind,src=$resultRoot,dst=/results",
        '--entrypoint', 'python', $WorkerImage,
        '/benchmark/benchmark_nexusai_nxmmr_anpr.py'
    ) + $Command
    & docker @arguments
    if ($LASTEXITCODE -ne 0) {
        throw "The isolated benchmark container failed with exit code $LASTEXITCODE"
    }
}

if (-not (Test-Path -LiteralPath $ImageRoot -PathType Container)) {
    throw "Approved image directory was not found: $ImageRoot"
}
Assert-Hash -Path $Video -Expected $expectedVideoDigest
$validation = Get-Content -LiteralPath (Join-Path $oracleRoot 'ground-truth-validation.json') -Raw | ConvertFrom-Json
if ($validation.GroundTruthValidation -ne 'PASS' -or $validation.status -ne 'PASS' -or $validation.gold_digest -ne $expectedGoldDigest) {
    throw 'The independent human gold pack is not validated and locked to the authorized digest.'
}
foreach ($entry in $modelDigests.GetEnumerator()) {
    Assert-Hash -Path (Join-Path $modelRoot $entry.Key) -Expected $entry.Value
}

docker image inspect $WorkerImage *> $null
if ($LASTEXITCODE -ne 0) {
    throw "The pre-existing incumbent worker image is unavailable; this runner will not build or download it: $WorkerImage"
}
$availableGiB = [math]::Round((Get-CimInstance Win32_OperatingSystem).FreePhysicalMemory / 1MB, 3)
if ($availableGiB -lt 3.5) {
    throw "Resource preflight failed: $availableGiB GiB free; 3.5 GiB is required."
}
$productionBefore = @(& docker ps --format '{{.ID}} {{.Names}}' | Sort-Object)
if ($LASTEXITCODE -ne 0) { throw 'Docker preflight failed.' }

New-Item -ItemType Directory -Path $resultRoot -Force | Out-Null
$mutex = [Threading.Mutex]::new($false, 'Global\NexusAI-NXMMR-ANPR')
if (-not $mutex.WaitOne(0)) { throw 'Another NX-MMR ANPR benchmark is already running.' }
try {
    foreach ($priorResult in @($developmentResult, $holdoutResult, $videoResult)) {
        if (Test-Path -LiteralPath $priorResult -PathType Leaf) {
            throw "A prior benchmark receipt already exists; refusing to rescore or overwrite it: $priorResult"
        }
    }
    Invoke-IsolatedBenchmark -Command @(
        'images', '--oracle-root', '/oracle', '--image-root', '/evidence/images',
        '--split', 'development', '--output', '/results/image-development.json'
    )
    Invoke-IsolatedBenchmark -Command @(
        'images', '--oracle-root', '/oracle', '--image-root', '/evidence/images',
        '--split', 'sealed_holdout', '--development-receipt', '/results/image-development.json',
        '--output', '/results/image-sealed-holdout.json'
    )
    Invoke-IsolatedBenchmark -Command @(
        'video', '--oracle-root', '/oracle', '--video', '/evidence/sample.mp4',
        '--output', '/results/sample-mp4-video.json'
    )
}
finally {
    $mutex.ReleaseMutex()
    $mutex.Dispose()
}

$productionAfter = @(& docker ps --format '{{.ID}} {{.Names}}' | Sort-Object)
if (Compare-Object -ReferenceObject $productionBefore -DifferenceObject $productionAfter) {
    throw 'Live container membership changed during the benchmark; results require investigation.'
}
Write-Host "NX-MMR isolated ANPR scoring complete: $resultRoot"
