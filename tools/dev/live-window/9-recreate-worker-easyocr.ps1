# LIVE WINDOW, EasyOCR routed OCR (tools/media_data/Build-EasyOcrOverlay.ps1, plan docs/roadmap/ocr-easyocr-integration-plan-20261117.md): recreates ONLY the forensic-records worker, with exactly the
# configuration it has now plus the easyocr compose file. Default is a DRY RUN (the compose hash of the current files must be one this script knows; nothing changes).
#   -Apply [-Tag t]       swap the image: tags the running :latest as :before-<Tag>, tags :<Tag> as :latest, then recreates the worker with the eight files
#   -Role on|off|keep     the EasyOCR role (FORENSIC_OCR_EASYOCR_ENABLED); keep = as now (off when coming from the seven files)
#   -Reread on|off|keep   Latin and digit boxes read again with the PP-OCRv5 Latin recogniser (FORENSIC_EASYOCR_LATIN_REREAD); keep = as now (off when coming from the seven files)
#   -Rollback [-Tag t]    tags :before-<Tag> as :latest and recreates the worker with the seven files (the router is v4 again, EasyOCR is gone)
#   -Enable               same as -Role on (kept for the first deploy's command line)
# Without -Apply, a -Role or -Reread that differs from the current state recreates the worker on the image it already runs (a switch flip, no image swap).
# Needs the model folder models\media\easyocr\model (tools\media_data\Install-EasyOcrModels.ps1) with the two pinned files; the dry run checks it. The worker refuses to start with a file that does not match its pin.
# Run from PowerShell:  .\tools\dev\live-window\9-recreate-worker-easyocr.ps1 [-Apply -Tag easyocr-reread-20261117] [-Role on] [-Reread on] [-Rollback]
# After an apply, restart the workspace if the API became unreachable (wslrelay), as after every recreate.
param([switch]$Apply, [switch]$Enable, [ValidateSet('keep', 'on', 'off')][string]$Role = 'keep', [ValidateSet('keep', 'on', 'off')][string]$Reread = 'keep', [switch]$Rollback, [string]$Tag = 'easyocr-20261117', [switch]$IgnoreInFlight)
$ErrorActionPreference = 'Stop'
if ($Enable) { $Role = 'on' }
$main = 'C:\Users\sheik\Workspace\Office\Projects\NexusAI'
$lw = (Resolve-Path (Join-Path $PSScriptRoot '..\..\..')).Path
$image = 'nexusai-forensic-records-worker'
$env:NEXUSAI_MEDIA_MODEL_DIR = "$lw\models\media"
$four = @('-f', "$main\docker-compose.forensic-records.yaml", '-f', "$lw\docker-compose.forensic-records.ingest-v2.yaml", '-f', "$lw\docker-compose.forensic-records.media.yaml", '-f', "$lw\docker-compose.forensic-records.families.yaml")
$six = $four + @('-f', "$lw\docker-compose.forensic-records.plates.yaml", '-f', "$lw\docker-compose.forensic-records.video.yaml")
$seven = $six + @('-f', "$lw\docker-compose.forensic-records.speech-routes.yaml")
$eight = $seven + @('-f', "$lw\docker-compose.forensic-records.easyocr.yaml")
# the first deploy (commit 1a59b9f) used the easyocr compose file before the re-read switch existed; its hash is reproduced from git so that worker is still recognised
$legacyFile = Join-Path ([System.IO.Path]::GetTempPath()) 'docker-compose.forensic-records.easyocr.v1.yaml'
git -C $lw show 1a59b9f:docker-compose.forensic-records.easyocr.yaml | Set-Content -LiteralPath $legacyFile -Encoding utf8
$eightV1 = $seven + @('-f', $legacyFile)
$base = @('compose', '--project-name', 'nexusai', '--project-directory', $main, '--env-file', "$main\.env.forensic-runtime.local")
$container = 'nexusai-forensic-records-worker-1'
$models = @("$lw\models\media\easyocr\model\craft_mlt_25k.pth", "$lw\models\media\easyocr\model\arabic.pth")
$info = @(docker inspect $container | ConvertFrom-Json)[0]
$label = $info.Config.Labels.'com.docker.compose.config-hash'
function Get-ConfigHash([string[]]$Files, [string]$Enabled, [string]$RereadValue) {
  foreach ($pair in @(@('FORENSIC_OCR_EASYOCR_ENABLED', $Enabled), @('FORENSIC_EASYOCR_LATIN_REREAD', $RereadValue))) {
    if ($pair[1]) { Set-Item -Path ("Env:\" + $pair[0]) -Value $pair[1] } else { Remove-Item ("Env:\" + $pair[0]) -ErrorAction SilentlyContinue }
  }
  $h = (docker @base @Files config --hash forensic-records-worker) -replace '^\S+\s+', ''
  Remove-Item Env:\FORENSIC_OCR_EASYOCR_ENABLED, Env:\FORENSIC_EASYOCR_LATIN_REREAD -ErrorAction SilentlyContinue
  return $h
}
$states = [ordered]@{
  'seven'            = @{ hash = (Get-ConfigHash $seven '' ''); role = 'off'; reread = 'off' }
  'v1 role off'      = @{ hash = (Get-ConfigHash $eightV1 'false' ''); role = 'off'; reread = 'off' }
  'v1 role on'       = @{ hash = (Get-ConfigHash $eightV1 'true' ''); role = 'on'; reread = 'off' }
  'role off'         = @{ hash = (Get-ConfigHash $eight 'false' 'false'); role = 'off'; reread = 'off' }
  'role on'          = @{ hash = (Get-ConfigHash $eight 'true' 'false'); role = 'on'; reread = 'off' }
  'role on + reread' = @{ hash = (Get-ConfigHash $eight 'true' 'true'); role = 'on'; reread = 'on' }
}
$state = ''
foreach ($name in $states.Keys) { if ($states[$name].hash -eq $label) { $state = $name; break } }
Write-Host ("running image {0}; label {1}" -f $info.Image.Substring(7, 12), $label.Substring(0, 12))
Write-Host ("known states: " + (($states.Keys | ForEach-Object { "{0} {1}" -f $_, $states[$_].hash.Substring(0, 8) }) -join '; '))
$present = ($models | Where-Object { Test-Path -LiteralPath $_ }).Count
Write-Host ("-> the worker runs: {0}; model files: {1} of 2 present" -f $(if ($state) { $state } else { 'NOT reproduced' }), $present)
if (-not $state) { throw 'compose does not reproduce the running worker; stop' }
$roleNow, $rereadNow = $states[$state].role, $states[$state].reread
$roleWant = if ($Role -eq 'keep') { $roleNow } else { $Role }
$rereadWant = if ($Reread -eq 'keep') { $rereadNow } else { $Reread }
if ($roleWant -eq 'off' -and $rereadWant -eq 'on') { throw 'the re-read belongs to the EasyOCR role: -Reread on needs the role on' }
$changesSwitch = ($roleWant -ne $roleNow) -or ($rereadWant -ne $rereadNow)
if (-not $Apply -and -not $Rollback -and -not $changesSwitch) { Write-Host 'dry run only; nothing changed.'; Remove-Item Env:\NEXUSAI_MEDIA_MODEL_DIR -ErrorAction SilentlyContinue; return }
if (($Apply -or $changesSwitch) -and -not $Rollback -and $present -ne 2) { throw "the EasyOCR model files are missing (Install-EasyOcrModels.ps1): $($models -join ', ')" }
$inflight = docker exec nexusai-forensic-postgres-1 psql -U localrecall -d localrecall -At -c "select count(*) from forensic.records_ingest_jobs where status::text in ('queued','running')"
if ([int]$inflight -ne 0 -and -not $IgnoreInFlight) { throw "jobs are in flight ($inflight); wait for the worker to be idle" }
$rollbackTag = "before-$Tag"
if ($Apply) {
  if (-not (docker images -q "${image}:${Tag}")) { throw "image ${image}:${Tag} does not exist (tools\media_data\Build-EasyOcrOverlay.ps1)" }
  if (docker images -q "${image}:${rollbackTag}") { throw "${image}:${rollbackTag} already exists; the rollback must not be overwritten" }
  docker tag "${image}:latest" "${image}:${rollbackTag}"
  docker tag "${image}:${Tag}" "${image}:latest"
  $files = $eight; $useSeven = $false
} elseif ($Rollback) {
  if (-not (docker images -q "${image}:${rollbackTag}")) { throw "no rollback image ${image}:${rollbackTag}" }
  docker tag "${image}:${rollbackTag}" "${image}:latest"
  $files = $seven; $useSeven = $true; $roleWant = 'gone'; $rereadWant = 'gone'
} else {
  $files = $eight; $useSeven = $false
}
if (-not $useSeven) {
  $env:FORENSIC_OCR_EASYOCR_ENABLED = if ($roleWant -eq 'on') { 'true' } else { 'false' }
  $env:FORENSIC_EASYOCR_LATIN_REREAD = if ($rereadWant -eq 'on') { 'true' } else { 'false' }
}
Write-Host ("{0}: {1}:latest is now {2}; role {3}; Latin re-read {4}" -f $(if ($Rollback) { 'ROLLBACK' } elseif ($Apply) { 'APPLY' } else { 'SWITCH' }), $image, ((@(docker image inspect "${image}:latest" | ConvertFrom-Json)[0]).Id.Substring(7, 12)), $roleWant, $rereadWant)
$ErrorActionPreference = 'Continue'   # compose prints a harmless warning about an orphan container on stderr; the exit code is what counts
docker @base @files up -d --no-deps --no-build --force-recreate forensic-records-worker 2>&1 | Out-Host
$rc = $LASTEXITCODE
$ErrorActionPreference = 'Stop'
if ($rc -ne 0) { throw "compose up failed (exit $rc)" }
for ($i = 0; $i -lt 60; $i++) {
  $s = (@(docker inspect $container | ConvertFrom-Json)[0]).State
  if ($s.Health.Status -eq 'healthy') { break }
  Start-Sleep -Seconds 3
}
$now = @(docker inspect $container | ConvertFrom-Json)[0]
$sw = @($now.Config.Env | Where-Object { $_ -match '^FORENSIC_(OCR_EASYOCR_ENABLED|EASYOCR_THREADS|EASYOCR_MAX_SIDE|EASYOCR_LATIN_REREAD|EASYOCR_LATIN_MODE|OCR_ROUTER_URDU_SHARE|ASR_ROUTES_ENABLED)' })
Write-Host ("after: image {0}, health {1}, {2}" -f $now.Image.Substring(7, 12), $now.State.Health.Status, ($sw -join ' '))
Remove-Item Env:\FORENSIC_OCR_EASYOCR_ENABLED, Env:\FORENSIC_EASYOCR_LATIN_REREAD, Env:\NEXUSAI_MEDIA_MODEL_DIR -ErrorAction SilentlyContinue
Remove-Item -LiteralPath $legacyFile -ErrorAction SilentlyContinue
