<#
.SYNOPSIS
  Copies the two EasyOCR model files the router needs into the media model folder, checking each against the SHA-256 pin of docker-compose.forensic-records.easyocr.yaml. It changes nothing else.

.DESCRIPTION
  Source: the evaluation data folder, where tools/media_data/fetch_easyocr.py put the files (and recorded the hashes of the downloads in manifest-20261117-easyocr.jsonl). Target:
  <media model folder>\easyocr\model (mounted read-only into the worker at /models/media). A file that is already there with another hash is NOT overwritten: the script stops and says so.
  The English recogniser (english_g2.pth) is not needed by the product route (Latin pages are read by the PP-OCRv5 engine) and is not copied.
  Ledger MOD-047 and MOD-048: the licence of these weights is unconfirmed; copying them is not a decision to use them.

.EXAMPLE
  .\tools\media_data\Install-EasyOcrModels.ps1 -MediaModelDir C:\Users\sheik\Workspace\Office\Projects\NexusAI-localwork\models\media
#>
param(
  [Parameter(Mandatory = $true)][string]$MediaModelDir,
  [string]$Source = 'C:\tmp\nexusai-media-data\models\ocr\easyocr\model'
)
$ErrorActionPreference = 'Stop'
$pins = [ordered]@{
  'craft_mlt_25k.pth' = '4a5efbfb48b4081100544e75e1e2b57f8de3d84f213004b14b85fd4b3748db17'
  'arabic.pth'        = '2a9afd42c374deb98aed0b53c9b77d75e1d00d4e0501f3b0276c54190c89b1a8'
}
$target = Join-Path $MediaModelDir 'easyocr\model'
New-Item -ItemType Directory -Force -Path $target | Out-Null
foreach ($name in $pins.Keys) {
  $from = Join-Path $Source $name
  if (-not (Test-Path -LiteralPath $from)) { throw "missing source file: $from" }
  $hash = (Get-FileHash -Algorithm SHA256 -LiteralPath $from).Hash.ToLower()
  if ($hash -ne $pins[$name]) { throw "$name does not match its pin ($hash)" }
  $to = Join-Path $target $name
  if (Test-Path -LiteralPath $to) {
    $existing = (Get-FileHash -Algorithm SHA256 -LiteralPath $to).Hash.ToLower()
    if ($existing -ne $pins[$name]) { throw "$to exists with another hash ($existing); it was not overwritten" }
    Write-Host "$name already in place, hash matches"
    continue
  }
  Copy-Item -LiteralPath $from -Destination $to
  Write-Host "copied $name ($hash)"
}
