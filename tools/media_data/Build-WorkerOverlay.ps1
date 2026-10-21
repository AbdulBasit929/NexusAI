<#
.SYNOPSIS
  Layers changed worker source files onto the running worker image (the "worker overlay" method used for Ingest v2), after tagging the current image as the rollback.

.DESCRIPTION
  The worker image is built from ingestion/forensic_records/Dockerfile, which needs the network for pip and apt; the overlay avoids that: it copies only the named files over /app of the image the
  worker runs today. The previous image stays available under `before-<Tag>`; rolling back is `docker tag nexusai-forensic-records-worker:before-<Tag> nexusai-forensic-records-worker:latest` and
  recreating the worker (see docker-compose.forensic-records.media.yaml for the command).

.EXAMPLE
  .\tools\media_data\Build-WorkerOverlay.ps1 -Tag media-ocr-20261021
#>
param(
  [Parameter(Mandatory = $true)][string]$Tag,
  [string[]]$Files = @('onnx_text_ocr.py', 'onnx_ocr_processor.py', 'media_pipeline.py', 'multilingual_ocr.py')
)
$ErrorActionPreference = 'Stop'
$repo = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$source = Join-Path $repo 'ingestion\forensic_records'
$image = 'nexusai-forensic-records-worker'
$rollback = "$($image):before-$Tag"
docker tag "$($image):latest" $rollback
if ($LASTEXITCODE -ne 0) { throw 'could not tag the rollback image' }
$context = Join-Path ([System.IO.Path]::GetTempPath()) ("worker-overlay-" + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $context | Out-Null
try {
  foreach ($name in $Files) {
    $path = Join-Path $source $name
    if (-not (Test-Path -LiteralPath $path)) { throw "missing source file: $path" }
    Copy-Item -LiteralPath $path -Destination (Join-Path $context $name)
  }
  $lines = @("FROM $rollback") + ($Files | ForEach-Object { "COPY $_ /app/$_" })
  Set-Content -LiteralPath (Join-Path $context 'Dockerfile') -Value $lines -Encoding ascii
  docker build -t "$($image):latest" $context
  if ($LASTEXITCODE -ne 0) { throw 'overlay build failed; the running worker is untouched' }
  Write-Host "built $($image):latest on top of $rollback ($($Files -join ', '))"
} finally {
  Remove-Item -LiteralPath $context -Recurse -Force -ErrorAction SilentlyContinue
}
