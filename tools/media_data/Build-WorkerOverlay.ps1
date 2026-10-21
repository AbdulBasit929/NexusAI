<#
.SYNOPSIS
  Layers the new OCR modules onto the running worker image (the "worker overlay" method used for Ingest v2), after tagging the current image as the rollback.

.DESCRIPTION
  The worker image is built from ingestion/forensic_records/Dockerfile, which needs the network for pip and apt; the overlay avoids that and changes only what is named. The image in production
  is the older build plus Ingest v2, so its media_pipeline.py is NOT the repository's file: this script extracts the image's own media_pipeline.py, applies the two small edits of
  patch_image_media_pipeline.py to it, and copies that (plus the new modules) over /app. The repository's worker.py, roman_urdu.py and the video modules are not touched.

  The previous image stays available under `before-<Tag>`; rolling back is
    docker tag nexusai-forensic-records-worker:before-<Tag> nexusai-forensic-records-worker:latest
  and recreating the worker (the command is in docker-compose.forensic-records.media.yaml).

.EXAMPLE
  .\tools\media_data\Build-WorkerOverlay.ps1 -Tag media-ocr-20261021
#>
param(
  [Parameter(Mandatory = $true)][string]$Tag,
  [string[]]$NewFiles = @('onnx_text_ocr.py', 'onnx_ocr_processor.py', 'ocr_router.py', 'multilingual_ocr.py')
)
$ErrorActionPreference = 'Stop'
$repo = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$source = Join-Path $repo 'ingestion\forensic_records'
$image = 'nexusai-forensic-records-worker'
$rollback = "$($image):before-$Tag"

function Invoke-Docker { param([string[]]$Arguments)
  # docker writes progress to stderr, which PowerShell 5.1 treats as an error under -ErrorAction Stop: judge by the exit code
  $previous = $ErrorActionPreference; $ErrorActionPreference = 'Continue'
  & docker @Arguments 2>&1 | Out-Host
  $code = $LASTEXITCODE; $ErrorActionPreference = $previous
  if ($code -ne 0) { throw "docker $($Arguments -join ' ') failed with exit code $code" }
}

# the rollback tag is made only once: a second run must not overwrite it with the already-patched image
$existing = & docker images -q $rollback 2>$null
if ($existing) { throw "$rollback already exists; pick a new tag or remove it on purpose" }
Invoke-Docker @('tag', "$($image):latest", $rollback)
$context = Join-Path ([System.IO.Path]::GetTempPath()) ("worker-overlay-" + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $context | Out-Null
try {
  foreach ($name in $NewFiles) {
    $path = Join-Path $source $name
    if (-not (Test-Path -LiteralPath $path)) { throw "missing source file: $path" }
    Copy-Item -LiteralPath $path -Destination (Join-Path $context $name)
  }
  # the image's own media_pipeline.py, patched in two places
  $extracted = Join-Path $context 'image_media_pipeline.py'
  $previous = $ErrorActionPreference; $ErrorActionPreference = 'Continue'
  $container = (& docker create $rollback 2>$null | Out-String).Trim()
  & docker cp "$($container):/app/media_pipeline.py" $extracted 2>&1 | Out-Null
  & docker rm $container 2>&1 | Out-Null
  $ErrorActionPreference = $previous
  if (-not (Test-Path -LiteralPath $extracted)) { throw 'could not extract media_pipeline.py from the image' }
  & python -X utf8 (Join-Path $PSScriptRoot 'patch_image_media_pipeline.py') $extracted (Join-Path $context 'media_pipeline.py')
  if ($LASTEXITCODE -ne 0) { throw 'the image media_pipeline.py did not match the expected text; nothing was built' }
  Remove-Item -LiteralPath $extracted
  $files = @('media_pipeline.py') + $NewFiles
  $lines = @("FROM $rollback") + ($files | ForEach-Object { "COPY $_ /app/$_" })
  Set-Content -LiteralPath (Join-Path $context 'Dockerfile') -Value $lines -Encoding ascii
  Invoke-Docker @('build', '-t', "$($image):latest", $context)
  Write-Host "built $($image):latest on top of $rollback ($($files -join ', '))"
} finally {
  Remove-Item -LiteralPath $context -Recurse -Force -ErrorAction SilentlyContinue
}
