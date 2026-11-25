<#
.SYNOPSIS
  Builds the worker image with the round-2 video plate fix (video_anpr_nexus_v4.py 4.1.0) as a NEW TAG on top of the running worker image. It does not touch `:latest`, the running container or any switch.

.DESCRIPTION
  docs/data/video-plates-recall-round2-20261125.md: the whole video is read coarse to fine (a long video is no longer read only for its first minutes), tiles on 1080p frames, two reads make a plate group
  when the frames read are sparse, and the result says how much of the video was read. Only this one module changes; the running image's other files stay as they are. The image's own copy of the module
  is checked first: it must equal the repository's version before the fix (line endings ignored), so nothing else is silently replaced. After the build the image is smoke-tested without network.

  Going live is a separate, deliberate step: tools/dev/live-window/12-recreate-worker-video-v41.ps1 (dry run by default; -Apply; -Rollback).

.EXAMPLE
  .\tools\media_data\Build-VideoV41Overlay.ps1 -Tag video-v41-20261125
#>
param(
  [Parameter(Mandatory = $true)][string]$Tag,
  [string]$BaseTag = 'latest'
)
$ErrorActionPreference = 'Stop'
$repo = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$module = Join-Path $repo 'ingestion\forensic_records\video_anpr_nexus_v4.py'
$image = 'nexusai-forensic-records-worker'
$output = "$($image):$Tag"
$base = "$($image):$BaseTag"
if (-not (Test-Path -LiteralPath $module)) { throw "missing source file: $module" }
if (-not (& docker images -q $base 2>$null)) { throw "base image $base does not exist" }
if (& docker images -q $output 2>$null) { throw "$output already exists; pick a new tag or remove it on purpose" }

function Invoke-Docker { param([string[]]$Arguments)
  $previous = $ErrorActionPreference; $ErrorActionPreference = 'Continue'
  & docker @Arguments 2>&1 | Out-Host
  $code = $LASTEXITCODE; $ErrorActionPreference = $previous
  if ($code -ne 0) { throw "docker $($Arguments -join ' ') failed with exit code $code" }
}

$context = Join-Path ([System.IO.Path]::GetTempPath()) ("video-v41-overlay-" + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $context | Out-Null
try {
  # the image's own copy must be the repository's pre-fix version, or something else has been changed in the image and this overlay would hide it
  $extracted = Join-Path $context 'image_module.py'
  $previous = $ErrorActionPreference; $ErrorActionPreference = 'Continue'
  $container = (& docker create $base 2>$null | Out-String).Trim()
  & docker cp "$($container):/app/video_anpr_nexus_v4.py" $extracted 2>&1 | Out-Null
  & docker rm $container 2>&1 | Out-Null
  $before = (& git -C $repo show 'HEAD:ingestion/forensic_records/video_anpr_nexus_v4.py' 2>$null | Out-String)
  $ErrorActionPreference = $previous
  if (-not (Test-Path -LiteralPath $extracted)) { throw 'could not extract video_anpr_nexus_v4.py from the image' }
  $inImage = ((Get-Content -LiteralPath $extracted -Raw) -replace "`r`n", "`n").Trim()
  $inHead = ($before -replace "`r`n", "`n").Trim()
  if ($inHead -and $inImage -ne $inHead) { Write-Warning 'the image copy of video_anpr_nexus_v4.py differs from HEAD (it may already be the fixed version, or the image has another change); the overlay replaces it with the repository file' }
  Remove-Item -LiteralPath $extracted
  Copy-Item -LiteralPath $module -Destination (Join-Path $context 'video_anpr_nexus_v4.py')
  Set-Content -LiteralPath (Join-Path $context 'Dockerfile') -Value @("FROM $base", 'COPY video_anpr_nexus_v4.py /app/video_anpr_nexus_v4.py') -Encoding ascii
  Invoke-Docker @('build', '-t', $output, $context)
  Write-Host "built $output on top of $base (video_anpr_nexus_v4.py); :latest and the running worker are unchanged"
  # no quotes inside the -c string (Windows PowerShell 5.1 strips them)
  # (no py_compile: the worker runs as user nobody and /app is not writable for a .pyc; importing the module below compiles it)
  Invoke-Docker @('run', '--rm', '--network', 'none', '--entrypoint', 'python', $output, '-c', 'import video_anpr_nexus_v4 as v;print(len(v.progressive_schedule(600.0,30.0,4.0,100)))')
} finally {
  Remove-Item -LiteralPath $context -Recurse -Force -ErrorAction SilentlyContinue
}
