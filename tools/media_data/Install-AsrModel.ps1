<#
.SYNOPSIS
  Installs a hash-checked faster-whisper (CTranslate2) model into the LocalAI container's model folder and registers it, without restarting anything.

.DESCRIPTION
  The worker's ASR role calls LocalAI's /v1/audio/transcriptions with a model name (FORENSIC_ASR_MODEL). This copies the model files named in the fetch manifest into /models/<name>/ of the
  LocalAI container (docker cp), writes /models/<name>.yaml (backend faster-whisper, the pinned model folder), and prints the SHA-256 of model.bin so the choice is recorded. LocalAI picks up
  a new model file on its next request; nothing is restarted and no other model is touched. Rollback: docker exec <container> rm -rf /models/<name> /models/<name>.yaml.

.EXAMPLE
  .\tools\media_data\Install-AsrModel.ps1 -Folder faster-whisper-large-v3-turbo-ct2 -Name faster-whisper-turbo-ur -Licence 'MIT (hub tag)'
#>
param(
  [Parameter(Mandatory = $true)][string]$Folder,   # a folder under C:\tmp\nexusai-media-data\models\asr
  [Parameter(Mandatory = $true)][string]$Name,     # the model name the worker will request
  [string]$Container = 'nexusai-api-1',
  [string]$Threads = '8',
  [string]$Licence = 'MIT (hub tag)',
  [string]$DataRoot = 'C:\tmp\nexusai-media-data'
)
$ErrorActionPreference = 'Stop'
$source = Join-Path $DataRoot "models\asr\$Folder"
if (-not (Test-Path (Join-Path $source 'model.bin'))) { throw "no model.bin in $source" }
$sha = (Get-FileHash -Algorithm SHA256 (Join-Path $source 'model.bin')).Hash.ToLower()
$manifestLine = Get-Content (Join-Path $DataRoot 'manifest.jsonl') | Where-Object { $_ -match [regex]::Escape("models/asr/$Folder/model.bin") } | Select-Object -Last 1
if (-not $manifestLine) { throw "models/asr/$Folder/model.bin is not in the fetch manifest" }
if (($manifestLine | ConvertFrom-Json).sha256 -ne $sha) { throw 'model.bin differs from the fetch manifest; refusing to install' }
$previous = $ErrorActionPreference; $ErrorActionPreference = 'Continue'
& docker exec $Container sh -c "test ! -e /models/$Name && mkdir -p /models/$Name" 2>&1 | Out-Null
if ($LASTEXITCODE -ne 0) { $ErrorActionPreference = $previous; throw "/models/$Name already exists in $Container; choose another name or remove it on purpose" }
foreach ($file in Get-ChildItem $source -File) { & docker cp $file.FullName "$($Container):/models/$Name/$($file.Name)" 2>&1 | Out-Null }
$yaml = @"
name: $Name
backend: faster-whisper
threads: $Threads
parameters:
  model: $Name
description: >-
  Pinned CTranslate2 Whisper model from the media activation programme (docs/data/media-activation-programme-20261021.md). model.bin sha256 $sha.
license: $Licence
"@
$yaml | & docker exec -i $Container sh -c "cat > /models/$Name.yaml" 2>&1 | Out-Null
$ErrorActionPreference = $previous
Write-Host "installed /models/$Name in $Container (model.bin sha256 $sha)"
& docker exec $Container sh -c "ls -la /models/$Name | head -12; cat /models/$Name.yaml | head -6"
