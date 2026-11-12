<#
.SYNOPSIS
  Builds the worker image with version-scoped media artifact ids as a NEW TAG on top of the running worker image. It does not touch `:latest`, the running container or any switch.

.DESCRIPTION
  Defect (found 2026-11-12): a media reading's id is derived from the SHA-256 of the file and the reading's position, and the worker stores with ON CONFLICT (artifact_id) DO NOTHING, so the second copy of
  the same bytes in another case lost every plate read, group and summary (ingestion/forensic_records/media_artifact_scope.py). The production image is an older build plus overlays, so the repository's
  worker.py cannot be copied over it: the image's own worker.py is extracted, patched by patch_worker_artifact_scope.py (seven edits; it refuses unless every anchor occurs once, and it keeps the
  mixed line endings) and copied with media_artifact_scope.py over /app. After the build the image is smoke-tested without network: the patched worker must parse and the module must import.

  Going live is a separate, deliberate step: tools/dev/live-window/7-recreate-worker-artifact-scope.ps1 (dry run by default; -Apply; -Rollback). No configuration changes.

.EXAMPLE
  .\tools\media_data\Build-ArtifactScopeOverlay.ps1 -Tag artifact-scope-20261112
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

$context = Join-Path ([System.IO.Path]::GetTempPath()) ("artifact-scope-overlay-" + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $context | Out-Null
try {
  $module = Join-Path $source 'media_artifact_scope.py'
  if (-not (Test-Path -LiteralPath $module)) { throw "missing source file: $module" }
  Copy-Item -LiteralPath $module -Destination (Join-Path $context 'media_artifact_scope.py')
  $extracted = Join-Path $context 'image_worker.py'
  $previous = $ErrorActionPreference; $ErrorActionPreference = 'Continue'
  $container = (& docker create $base 2>$null | Out-String).Trim()
  & docker cp "$($container):/app/worker.py" $extracted 2>&1 | Out-Null
  & docker rm $container 2>&1 | Out-Null
  $ErrorActionPreference = $previous
  if (-not (Test-Path -LiteralPath $extracted)) { throw 'could not extract worker.py from the image' }
  & python -X utf8 (Join-Path $PSScriptRoot 'patch_worker_artifact_scope.py') $extracted (Join-Path $context 'worker.py')
  if ($LASTEXITCODE -ne 0) { throw 'the image worker.py did not match the expected text (or is already patched); nothing was built' }
  Remove-Item -LiteralPath $extracted
  $files = @('worker.py', 'media_artifact_scope.py')
  $lines = @("FROM $base") + ($files | ForEach-Object { "COPY $_ /app/$_" })
  Set-Content -LiteralPath (Join-Path $context 'Dockerfile') -Value $lines -Encoding ascii
  Invoke-Docker @('build', '-t', $output, $context)
  Write-Host "built $output on top of $base ($($files -join ', ')); :latest and the running worker are unchanged"
  # no quotes inside the -c string (Windows PowerShell 5.1 strips them): the worker is compiled by py_compile, the module imported by a quote-free command
  Invoke-Docker @('run', '--rm', '--network', 'none', '--entrypoint', 'python', $output, '-m', 'py_compile', '/app/worker.py')
  Invoke-Docker @('run', '--rm', '--network', 'none', '--entrypoint', 'python', $output, '-c', 'import media_artifact_scope;print(7)')
} finally {
  Remove-Item -LiteralPath $context -Recurse -Force -ErrorAction SilentlyContinue
}
