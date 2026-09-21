[CmdletBinding()]
param(
    [string]$ImageRoot = (Join-Path $env:USERPROFILE 'Downloads\archive\Pakistani License Number Plates Data'),
    [string]$Video = (Join-Path $env:USERPROFILE 'Downloads\sample.mp4')
)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
$helper = Join-Path $PSScriptRoot 'nxmmr_human_verification.py'

if (-not (Test-Path -LiteralPath $ImageRoot -PathType Container)) {
    throw "Approved image directory was not found: $ImageRoot"
}
if (-not (Test-Path -LiteralPath $Video -PathType Leaf)) {
    throw "Approved video was not found: $Video"
}

Push-Location $repoRoot
try {
    python $helper --image-root $ImageRoot --video $Video
}
finally {
    Pop-Location
}
