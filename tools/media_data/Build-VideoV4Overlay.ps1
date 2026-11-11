<#
.SYNOPSIS
  Builds the worker image with the video plate processor V4 as a NEW TAG on top of the running worker image. It does not touch `:latest`, the running container or any switch.

.DESCRIPTION
  Same overlay method as Build-PlatesOverlay.ps1: the production image is an older build plus overlays, so the repository's media_pipeline.py cannot be copied over it. The image's own
  media_pipeline.py is extracted, patched by patch_image_media_pipeline_video_v4.py (four edits; it refuses unless every anchor occurs once) and copied with the four video modules over /app:
  video_anpr_nexus_v4.py (V4), video_anpr_parity_adapter.py (scheduler, grouping, contracts), video_anpr_product_v2.py (the precision gates) and video_anpr_onnx_v3.py (V4 takes one helper from it).
  The default engine of V4's second reader and its model files are already in the worker (FORENSIC_ANPR_DETECTOR_MODEL_PATH and the cct-xs reader); the M5 engine is in the plates overlay.
  After the build the image is smoke-tested without network: the pipeline and V4 must import.

  Going live is a separate, deliberate step: tools/dev/live-window/6-recreate-worker-video.ps1 (dry run by default; -Apply; -Rollback), which adds docker-compose.forensic-records.video.yaml.

.EXAMPLE
  .\tools\media_data\Build-VideoV4Overlay.ps1 -Tag video-v4-20261111
#>
param(
  [Parameter(Mandatory = $true)][string]$Tag,
  [string]$BaseTag = 'latest',
  [string[]]$NewFiles = @('video_anpr_nexus_v4.py', 'video_anpr_parity_adapter.py', 'video_anpr_product_v2.py', 'video_anpr_onnx_v3.py')
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

$context = Join-Path ([System.IO.Path]::GetTempPath()) ("video-v4-overlay-" + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $context | Out-Null
try {
  foreach ($name in $NewFiles) {
    $path = Join-Path $source $name
    if (-not (Test-Path -LiteralPath $path)) { throw "missing source file: $path" }
    Copy-Item -LiteralPath $path -Destination (Join-Path $context $name)
  }
  $extracted = Join-Path $context 'image_media_pipeline.py'
  $previous = $ErrorActionPreference; $ErrorActionPreference = 'Continue'
  $container = (& docker create $base 2>$null | Out-String).Trim()
  & docker cp "$($container):/app/media_pipeline.py" $extracted 2>&1 | Out-Null
  & docker rm $container 2>&1 | Out-Null
  $ErrorActionPreference = $previous
  if (-not (Test-Path -LiteralPath $extracted)) { throw 'could not extract media_pipeline.py from the image' }
  & python -X utf8 (Join-Path $PSScriptRoot 'patch_image_media_pipeline_video_v4.py') $extracted (Join-Path $context 'media_pipeline.py')
  if ($LASTEXITCODE -ne 0) { throw 'the image media_pipeline.py did not match the expected text; nothing was built' }
  Remove-Item -LiteralPath $extracted
  $files = @('media_pipeline.py') + $NewFiles
  $lines = @("FROM $base") + ($files | ForEach-Object { "COPY $_ /app/$_" })
  Set-Content -LiteralPath (Join-Path $context 'Dockerfile') -Value $lines -Encoding ascii
  Invoke-Docker @('build', '-t', $output, $context)
  Write-Host "built $output on top of $base ($($files -join ', ')); :latest and the running worker are unchanged"
  Invoke-Docker @('run', '--rm', '--network', 'none', '--entrypoint', 'python', $output, '-c', 'import media_pipeline,video_anpr_nexus_v4;print(7)')
} finally {
  Remove-Item -LiteralPath $context -Recurse -Force -ErrorAction SilentlyContinue
}
