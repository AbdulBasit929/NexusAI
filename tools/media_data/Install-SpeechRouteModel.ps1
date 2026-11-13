<#
.SYNOPSIS
  Puts the converted Omnilingual CTC model where the worker reads it: models\media\speech\omni-ctc-300m-hf in this checkout (git-ignored, mounted read-only at /models/media in the worker).

.DESCRIPTION
  The converted folder is made by `bash tools/media_data/run_speech_baseline.sh omni-convert` (tools/media_data/omni_ctc.py; 423 of 423 tensors placed, vocabulary equal to the CTC outputs) and lives in
  C:\tmp\nexusai-media-data\models\asr\omni-ctc-300m-hf. This copies model.safetensors, config.json, pieces.json and SOURCE.txt, and verifies the SHA-256 of the copy against the original. Nothing is
  changed in the running system; the worker reads the folder only when the speech routes are switched on (docker-compose.forensic-records.speech-routes.yaml).
#>
param([string]$Source = 'C:\tmp\nexusai-media-data\models\asr\omni-ctc-300m-hf')
$ErrorActionPreference = 'Stop'
$repo = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$target = Join-Path $repo 'models\media\speech\omni-ctc-300m-hf'
New-Item -ItemType Directory -Force -Path $target | Out-Null
foreach ($name in @('model.safetensors', 'config.json', 'pieces.json', 'SOURCE.txt')) {
  $from = Join-Path $Source $name
  if (-not (Test-Path -LiteralPath $from)) { throw "missing $from" }
  Copy-Item -LiteralPath $from -Destination (Join-Path $target $name) -Force
}
$a = (Get-FileHash -Algorithm SHA256 -LiteralPath (Join-Path $Source 'model.safetensors')).Hash
$b = (Get-FileHash -Algorithm SHA256 -LiteralPath (Join-Path $target 'model.safetensors')).Hash
if ($a -ne $b) { throw 'the copy does not match the original' }
Write-Host "installed $target (model.safetensors sha256 $a)"
