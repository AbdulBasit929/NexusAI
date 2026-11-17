<#
.SYNOPSIS
  Builds the worker image with the EasyOCR Urdu and mixed-text role (OCR router v5) as a NEW TAG on top of the running worker image. It does not touch `:latest`, the running container or any switch.

.DESCRIPTION
  Plan: docs/roadmap/ocr-easyocr-integration-plan-20261117.md. What the image gets: the packages EasyOCR needs (installed WITHOUT its dependency list so torch and OpenCV of the image are not replaced:
  easyocr 1.7.2, torchvision 0.21.0 CPU, scikit-image, python-bidi, Shapely, pyclipper, ninja, PyYAML; the recipe of tools/media_data/Dockerfile.eval-easyocr, about 40 MB), and two files over /app:
  easyocr_ocr_processor.py (new) and ocr_router.py (v5; the image's own file is v4). The model files are NOT in the image: they are mounted read-only from the media model folder and pinned by SHA-256.
  The role is OFF by default (FORENSIC_OCR_EASYOCR_ENABLED=false in docker-compose.forensic-records.easyocr.yaml); a worker recreated without that file runs the router exactly as before.

  Building needs the network once (pip). After the build the image is smoke-tested WITHOUT network: the packages import, the two files compile, and the unit tests of the router and the processor pass inside the image.
  Going live is a separate, deliberate step that needs the owner's word (tools/dev/live-window; dry run first).

.EXAMPLE
  .\tools\media_data\Build-EasyOcrOverlay.ps1 -Tag easyocr-20261117
#>
param(
  [Parameter(Mandatory = $true)][string]$Tag,
  [string]$BaseTag = 'latest'
)
$ErrorActionPreference = 'Stop'
$repo = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$source = Join-Path $repo 'ingestion\forensic_records'
$image = 'nexusai-forensic-records-worker'
$output = "$($image):$Tag"
$base = "$($image):$BaseTag"
if (-not (& docker images -q $base 2>$null)) { throw "base image $base does not exist" }
if (& docker images -q $output 2>$null) { throw "$output already exists; pick a new tag or remove it on purpose" }

function Invoke-Docker { param([string[]]$Arguments)
  $previous = $ErrorActionPreference; $ErrorActionPreference = 'Continue'
  & docker @Arguments 2>&1 | Out-Host
  $code = $LASTEXITCODE; $ErrorActionPreference = $previous
  if ($code -ne 0) { throw "docker $($Arguments -join ' ') failed with exit code $code" }
}

$context = Join-Path ([System.IO.Path]::GetTempPath()) ("easyocr-overlay-" + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $context | Out-Null
try {
  $files = @('easyocr_ocr_processor.py', 'ocr_router.py')
  foreach ($name in $files) {
    $path = Join-Path $source $name
    if (-not (Test-Path -LiteralPath $path)) { throw "missing source file: $path" }
    Copy-Item -LiteralPath $path -Destination (Join-Path $context $name)
  }
  $user = (& docker inspect -f '{{.Config.User}}' $base 2>$null | Out-String).Trim()
  if (-not $user) { $user = '65534:65534' }
  $lines = @(
    "FROM $base",
    'USER root',
    'RUN pip install --no-cache-dir --no-deps easyocr==1.7.2 \',
    ' && pip install --no-cache-dir --no-deps torchvision==0.21.0 --index-url https://download.pytorch.org/whl/cpu \',
    ' && pip install --no-cache-dir scikit-image python-bidi Shapely pyclipper ninja PyYAML'
  ) + ($files | ForEach-Object { "COPY $_ /app/$_" }) + @("USER $user")
  Set-Content -LiteralPath (Join-Path $context 'Dockerfile') -Value $lines -Encoding ascii
  Invoke-Docker @('build', '-t', $output, $context)
  Write-Host "built $output on top of $base ($($files -join ', ') and the EasyOCR packages); :latest and the running worker are unchanged"
  # no quotes inside the -c strings (Windows PowerShell 5.1 strips them)
  # the image runs as an unprivileged user that cannot write bytecode into /app, so the files are parsed rather than compiled to disk
  Invoke-Docker @('run', '--rm', '--network', 'none', '--entrypoint', 'python', $output, '-B', '-c', 'import ast,sys;[ast.parse(open(p).read()) for p in sys.argv[1:]]', '/app/ocr_router.py', '/app/easyocr_ocr_processor.py')
  Invoke-Docker @('run', '--rm', '--network', 'none', '--entrypoint', 'python', $output, '-c', 'import easyocr,torchvision,skimage,bidi,shapely,pyclipper,torch,numpy,cv2;print(easyocr.__version__,torch.__version__,numpy.__version__,cv2.__version__)')
  Invoke-Docker @('run', '--rm', '--network', 'none', '--entrypoint', 'python', $output, '-c', 'import ocr_router,easyocr_ocr_processor;print(ocr_router.PROCESSOR_REVISION,easyocr_ocr_processor.PROCESSOR_REVISION)')
  # the unit tests run inside the image against the repository's copy (read-only mount), which imports the same code as /app
  Invoke-Docker @('run', '--rm', '--network', 'none', '-v', "${repo}:/repo:ro", '-w', '/repo', '--entrypoint', 'python', $output, '-B', '-m', 'unittest', 'ingestion.forensic_records.tests.test_ocr_router', 'ingestion.forensic_records.tests.test_easyocr_ocr_processor')
} finally {
  Remove-Item -LiteralPath $context -Recurse -Force -ErrorAction SilentlyContinue
}
