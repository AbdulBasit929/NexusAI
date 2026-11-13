<#
.SYNOPSIS
  Builds the worker image with the speech routes as a NEW TAG on top of the running worker image. It does not touch `:latest`, the running container or any switch.

.DESCRIPTION
  The speech routes (ingestion/forensic_records/speech_routes.py) let the worker read Pashto, Pakistani Punjabi and the other languages of its route table in its own process. The production image is an
  older build plus overlays, so the repository's media_pipeline.py cannot be copied over it: the image's own media_pipeline.py is extracted, patched by patch_image_media_pipeline_speech_routes.py
  (one edit; it refuses unless its anchor occurs exactly once) and copied with speech_routes.py and speech_routes.json over /app. After the build the image is smoke-tested without network: the patched file
  must compile and the route module must import (which proves that it finds media_pipeline next to it, as in the worker) and load its table.

  Going live is a separate, deliberate step: tools/dev/live-window/8-recreate-worker-speech-routes.ps1 (dry run by default; -Apply; -Rollback). The routes stay OFF until the compose file turns them on.

.EXAMPLE
  .\tools\media_data\Build-SpeechRoutesOverlay.ps1 -Tag speech-routes-20261113
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

$context = Join-Path ([System.IO.Path]::GetTempPath()) ("speech-routes-overlay-" + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $context | Out-Null
try {
  foreach ($name in @('speech_routes.py', 'speech_routes.json')) {
    $file = Join-Path $source $name
    if (-not (Test-Path -LiteralPath $file)) { throw "missing source file: $file" }
    Copy-Item -LiteralPath $file -Destination (Join-Path $context $name)
  }
  $extracted = Join-Path $context 'image_media_pipeline.py'
  $previous = $ErrorActionPreference; $ErrorActionPreference = 'Continue'
  $container = (& docker create $base 2>$null | Out-String).Trim()
  & docker cp "$($container):/app/media_pipeline.py" $extracted 2>&1 | Out-Null
  & docker rm $container 2>&1 | Out-Null
  $ErrorActionPreference = $previous
  if (-not (Test-Path -LiteralPath $extracted)) { throw 'could not extract media_pipeline.py from the image' }
  & python -X utf8 (Join-Path $PSScriptRoot 'patch_image_media_pipeline_speech_routes.py') $extracted (Join-Path $context 'media_pipeline.py')
  if ($LASTEXITCODE -ne 0) { throw 'the image media_pipeline.py did not match the expected text (or is already patched); nothing was built' }
  Remove-Item -LiteralPath $extracted
  $files = @('media_pipeline.py', 'speech_routes.py', 'speech_routes.json')
  $lines = @("FROM $base") + ($files | ForEach-Object { "COPY $_ /app/$_" })
  Set-Content -LiteralPath (Join-Path $context 'Dockerfile') -Value $lines -Encoding ascii
  Invoke-Docker @('build', '-t', $output, $context)
  Write-Host "built $output on top of $base ($($files -join ', ')); :latest and the running worker are unchanged"
  # no quotes inside the -c strings (Windows PowerShell 5.1 strips them): compile the patched file, import the route module from /app and load its table
  Invoke-Docker @('run', '--rm', '--network', 'none', '--entrypoint', 'python', $output, '-m', 'py_compile', '/app/media_pipeline.py')
  Invoke-Docker @('run', '--rm', '--network', 'none', '--workdir', '/app', '--entrypoint', 'python', $output, '-c', 'import speech_routes as s;print(sorted(s.load_routes()))')
} finally {
  Remove-Item -LiteralPath $context -Recurse -Force -ErrorAction SilentlyContinue
}
