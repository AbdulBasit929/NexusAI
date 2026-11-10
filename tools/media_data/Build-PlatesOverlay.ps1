<#
.SYNOPSIS
  Builds the worker image with the plate engine (FORENSIC_ANPR_ENGINE=nexus) as a NEW TAG on top of the running worker image. It does not touch `:latest`, the running container or any switch.

.DESCRIPTION
  Same overlay method as Build-WorkerOverlay.ps1 (the production image is an older build plus overlays, so the repository's media_pipeline.py cannot be copied over it): the image's own
  media_pipeline.py is extracted, patched by patch_image_media_pipeline_plates.py (three edits; refuses unless every anchor occurs once), and copied with the plate modules and the format registry
  over /app. The model files are not in the image: the worker already mounts models/media at /models/media, and the plate models are staged in models/media/nexus-plates/.

  Going live is a separate, deliberate step (docs/data/plates-m5-preregistration-20261110.md, Amendment 5 and tools/media_data/PLATES_GO_LIVE.md): tag this image as latest (the previous
  latest is kept as the rollback), set FORENSIC_ANPR_ENABLED=true and FORENSIC_ANPR_ENGINE=nexus, recreate the worker.

.EXAMPLE
  .\tools\media_data\Build-PlatesOverlay.ps1 -Tag plates-20261110
#>
param(
  [Parameter(Mandatory = $true)][string]$Tag,
  [string[]]$NewFiles = @('plate_processor_nexus.py', 'plate_detector_onnx.py', 'plate_formats.py', 'plate_format_registry_v1.json')
)
$ErrorActionPreference = 'Stop'
$repo = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$source = Join-Path $repo 'ingestion\forensic_records'
$image = 'nexusai-forensic-records-worker'
$output = "$($image):$Tag"
if (& docker images -q $output 2>$null) { throw "$output already exists; pick a new tag or remove it on purpose" }

function Invoke-Docker { param([string[]]$Arguments)
  $previous = $ErrorActionPreference; $ErrorActionPreference = 'Continue'
  & docker @Arguments 2>&1 | Out-Host
  $code = $LASTEXITCODE; $ErrorActionPreference = $previous
  if ($code -ne 0) { throw "docker $($Arguments -join ' ') failed with exit code $code" }
}

$context = Join-Path ([System.IO.Path]::GetTempPath()) ("plates-overlay-" + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $context | Out-Null
try {
  foreach ($name in $NewFiles) {
    $path = Join-Path $source $name
    if (-not (Test-Path -LiteralPath $path)) { throw "missing source file: $path" }
    Copy-Item -LiteralPath $path -Destination (Join-Path $context $name)
  }
  $extracted = Join-Path $context 'image_media_pipeline.py'
  $previous = $ErrorActionPreference; $ErrorActionPreference = 'Continue'
  $container = (& docker create "$($image):latest" 2>$null | Out-String).Trim()
  & docker cp "$($container):/app/media_pipeline.py" $extracted 2>&1 | Out-Null
  & docker rm $container 2>&1 | Out-Null
  $ErrorActionPreference = $previous
  if (-not (Test-Path -LiteralPath $extracted)) { throw 'could not extract media_pipeline.py from the image' }
  & python -X utf8 (Join-Path $PSScriptRoot 'patch_image_media_pipeline_plates.py') $extracted (Join-Path $context 'media_pipeline.py')
  if ($LASTEXITCODE -ne 0) { throw 'the image media_pipeline.py did not match the expected text; nothing was built' }
  Remove-Item -LiteralPath $extracted
  $files = @('media_pipeline.py') + $NewFiles
  $lines = @("FROM $($image):latest") + ($files | ForEach-Object { "COPY $_ /app/$_" })
  Set-Content -LiteralPath (Join-Path $context 'Dockerfile') -Value $lines -Encoding ascii
  Invoke-Docker @('build', '-t', $output, $context)
  Write-Host "built $output on top of $($image):latest ($($files -join ', ')); :latest and the running worker are unchanged"
} finally {
  Remove-Item -LiteralPath $context -Recurse -Force -ErrorAction SilentlyContinue
}
