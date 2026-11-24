<#
.SYNOPSIS
  Provisions the two models of the image search (DINOv2 base and SigLIP 2 base) into models/media/image-search/ after checking their SHA-256.

.DESCRIPTION
  Copies the model folders that the 2026-11-19 bake-off downloaded (tools/media_data/fetch_retrieval_models_20261119.py) into the media model folder that the worker and the image-search service mount read-only.
  Weights (model.safetensors) are verified against the digests the compose overlay pins; a mismatch stops the copy. Only the files the encoders read are copied (no .bin, .h5, .msgpack, .onnx or .tflite duplicates).
  Both models are Apache-2.0 (model cards read 2026-11-19; docs/data/face-image-search-results-20261119.md section 9). Nothing is downloaded here and nothing is started.

.EXAMPLE
  powershell -ExecutionPolicy Bypass -File .\tools\media_data\Install-ImageSearchModels.ps1
#>
[CmdletBinding()]
param(
  [string]$Source = 'C:\tmp\nexusai-media-data\models\image',
  [string]$Target = ''
)
$ErrorActionPreference = 'Stop'
$Repo = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
if (-not $Target) { $Target = Join-Path $Repo 'models\media\image-search' }
$Models = @(
  @{ Name = 'dinov2-base'; Sha256 = 'd73036b56966966d07975d696bde331762f37297e2f095de8cea0040c3aa0841' },
  @{ Name = 'siglip2-base-patch16-224'; Sha256 = '612923381c76ec5a9bed335d1c48827e3f2e506ac31b044b63b2031fadee6a0b' }
)
foreach ($model in $Models) {
  $from = Join-Path $Source $model.Name
  $to = Join-Path $Target $model.Name
  $weights = Join-Path $from 'model.safetensors'
  if (-not (Test-Path -LiteralPath $weights)) { throw "missing $weights" }
  $actual = (Get-FileHash -Algorithm SHA256 -LiteralPath $weights).Hash.ToLower()
  if ($actual -ne $model.Sha256) { throw "$($model.Name): model.safetensors is $actual, expected $($model.Sha256)" }
  New-Item -ItemType Directory -Force -Path $to | Out-Null
  Get-ChildItem -LiteralPath $from -File -Recurse | Where-Object { $_.Extension -notin '.bin', '.h5', '.msgpack', '.onnx', '.tflite', '.git' -and $_.FullName -notmatch '\\\.' } | ForEach-Object {
    $relative = $_.FullName.Substring($from.Length).TrimStart('\')
    $destination = Join-Path $to $relative
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $destination) | Out-Null
    Copy-Item -LiteralPath $_.FullName -Destination $destination -Force
  }
  Write-Host ("ok  {0}  safetensors verified {1}" -f $model.Name, $actual.Substring(0, 12))
}
Write-Host "provisioned under $Target"
