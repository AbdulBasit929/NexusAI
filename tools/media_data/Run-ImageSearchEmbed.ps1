<#
.SYNOPSIS
  Runs the M1 embedding stage in a memory-capped container and restarts it until it reports completion (the Docker VM on this laptop sometimes refuses an allocation; finished checkpoints are kept).

.EXAMPLE
  powershell -ExecutionPolicy Bypass -File .\tools\media_data\Run-ImageSearchEmbed.ps1 -Stage embed-nd -Threads 4
#>
[CmdletBinding()]
param(
  [ValidateSet('embed-nd', 'embed-sem')][string]$Stage = 'embed-nd',
  [int]$Threads = 4,
  [int]$MemoryMB = 1800,
  [int]$MaxRuns = 12,
  [string]$Data = 'C:/tmp/nexusai-media-data'
)
$ErrorActionPreference = 'Continue'
$Repo = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$Log = "$Data/work-faces/m1-$Stage.log"
for ($run = 1; $run -le $MaxRuns; $run++) {
  "$(Get-Date -Format 'HH:mm:ss') run $run" | Out-File -FilePath $Log -Append -Encoding utf8
  & docker run --rm --network none --memory "${MemoryMB}m" -e "IMG_THREADS=$Threads" -v "${Data}:/data:ro" -v "${Data}/work-faces:/work" -v "${Repo}:/repo:ro" -v "${Repo}/models/media:/models/media:ro" `
    --entrypoint python nexusai-media-eval-face:20261119 /repo/tools/media_data/image_search_m1.py $Stage 2>&1 | Out-File -FilePath $Log -Append -Encoding utf8
  if (Select-String -Path $Log -Pattern "$Stage complete|siglip1 done" -Quiet) { "$(Get-Date -Format 'HH:mm:ss') complete after $run run(s)" | Out-File -FilePath $Log -Append -Encoding utf8; break }
  Start-Sleep -Seconds 20
}
