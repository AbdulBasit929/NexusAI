# LIVE WINDOW, EasyOCR routed OCR (tools/media_data/Build-EasyOcrOverlay.ps1, plan docs/roadmap/ocr-easyocr-integration-plan-20261117.md): recreates ONLY the forensic-records worker on the overlay image, with exactly the
# configuration it has now plus the easyocr compose file. Default is a DRY RUN (the compose hash of the seven current files must equal the running container's label; nothing changes).
#   -Apply            tags the running :latest as :before-<Tag>, tags :<Tag> as :latest and recreates the worker with the eight files; the role is OFF (FORENSIC_OCR_EASYOCR_ENABLED=false): the router behaves as before
#   -Apply -Enable    the same with the role ON (a separate decision; needs the owner's word, one role at a time)
#   -Enable           (alone) switches the role on in the already-recreated worker: the same image, the eight files, FORENSIC_OCR_EASYOCR_ENABLED=true
#   -Rollback         tags :before-<Tag> as :latest and recreates the worker with the seven files (the router is v4 again, EasyOCR is gone)
# Needs the model folder models\media\easyocr\model (tools\media_data\Install-EasyOcrModels.ps1) with the two pinned files; the dry run checks it. The worker refuses to start with a file that does not match its pin.
# Run from PowerShell:  .\tools\dev\live-window\9-recreate-worker-easyocr.ps1 [-Apply] [-Enable] [-Rollback] [-Tag easyocr-20261117]
# After an apply, restart the workspace if the API became unreachable (wslrelay), as after every recreate.
param([switch]$Apply, [switch]$Enable, [switch]$Rollback, [string]$Tag = 'easyocr-20261117', [switch]$IgnoreInFlight)
$ErrorActionPreference = 'Stop'
$main = 'C:\Users\sheik\Workspace\Office\Projects\NexusAI'
$lw = (Resolve-Path (Join-Path $PSScriptRoot '..\..\..')).Path
$image = 'nexusai-forensic-records-worker'
$env:NEXUSAI_MEDIA_MODEL_DIR = "$lw\models\media"
$four = @('-f', "$main\docker-compose.forensic-records.yaml", '-f', "$lw\docker-compose.forensic-records.ingest-v2.yaml", '-f', "$lw\docker-compose.forensic-records.media.yaml", '-f', "$lw\docker-compose.forensic-records.families.yaml")
$six = $four + @('-f', "$lw\docker-compose.forensic-records.plates.yaml", '-f', "$lw\docker-compose.forensic-records.video.yaml")
$seven = $six + @('-f', "$lw\docker-compose.forensic-records.speech-routes.yaml")
$eight = $seven + @('-f', "$lw\docker-compose.forensic-records.easyocr.yaml")
$base = @('compose', '--project-name', 'nexusai', '--project-directory', $main, '--env-file', "$main\.env.forensic-runtime.local")
$container = 'nexusai-forensic-records-worker-1'
$models = @("$lw\models\media\easyocr\model\craft_mlt_25k.pth", "$lw\models\media\easyocr\model\arabic.pth")
$info = @(docker inspect $container | ConvertFrom-Json)[0]
$label = $info.Config.Labels.'com.docker.compose.config-hash'
function Get-ConfigHash([string[]]$Files, [string]$EnabledValue) {
  if ($EnabledValue) { $env:FORENSIC_OCR_EASYOCR_ENABLED = $EnabledValue } else { Remove-Item Env:\FORENSIC_OCR_EASYOCR_ENABLED -ErrorAction SilentlyContinue }
  $h = (docker @base @Files config --hash forensic-records-worker) -replace '^\S+\s+', ''
  Remove-Item Env:\FORENSIC_OCR_EASYOCR_ENABLED -ErrorAction SilentlyContinue
  return $h
}
$hash7 = Get-ConfigHash $seven ''
$hash8off = Get-ConfigHash $eight 'false'
$hash8on = Get-ConfigHash $eight 'true'
$state = if ($label -eq $hash7) { 'seven' } elseif ($label -eq $hash8off) { 'eight-off' } elseif ($label -eq $hash8on) { 'eight-on' } else { '' }
Write-Host ("running image {0}; label {1}" -f $info.Image.Substring(7, 12), $label)
Write-Host ("compose hash: seven files {0}, eight files off {1}, eight files on {2} -> {3}" -f $hash7.Substring(0, 12), $hash8off.Substring(0, 12), $hash8on.Substring(0, 12), $(if ($state) { "the worker runs: $state" } else { 'NOT reproduced' }))
$present = ($models | Where-Object { Test-Path -LiteralPath $_ }).Count
Write-Host ("model files: {0} of 2 present" -f $present)
if (-not $state) { throw 'compose does not reproduce the running worker; stop' }
if (-not $Apply -and -not $Rollback -and -not $Enable) { Write-Host 'dry run only; nothing changed.'; Remove-Item Env:\NEXUSAI_MEDIA_MODEL_DIR -ErrorAction SilentlyContinue; return }
if (($Apply -or $Enable) -and $present -ne 2) { throw "the EasyOCR model files are missing (Install-EasyOcrModels.ps1): $($models -join ', ')" }
$inflight = docker exec nexusai-forensic-postgres-1 psql -U localrecall -d localrecall -At -c "select count(*) from forensic.records_ingest_jobs where status::text in ('queued','running')"
if ([int]$inflight -ne 0 -and -not $IgnoreInFlight) { throw "jobs are in flight ($inflight); wait for the worker to be idle" }
$rollbackTag = "before-$Tag"
if ($Apply) {
  if ($state -ne 'seven') { throw "the worker already runs the EasyOCR files ($state); use -Enable or -Rollback" }
  if (-not (docker images -q "${image}:${Tag}")) { throw "image ${image}:${Tag} does not exist (tools\media_data\Build-EasyOcrOverlay.ps1)" }
  if (docker images -q "${image}:${rollbackTag}") { throw "${image}:${rollbackTag} already exists; the rollback must not be overwritten" }
  docker tag "${image}:latest" "${image}:${rollbackTag}"
  docker tag "${image}:${Tag}" "${image}:latest"
  $files = $eight; $useEight = $true
} elseif ($Enable) {
  if ($state -eq 'seven') { throw 'the worker has not been recreated with the EasyOCR files yet (-Apply first)' }
  $files = $eight; $useEight = $true
} else {
  if (-not (docker images -q "${image}:${rollbackTag}")) { throw "no rollback image ${image}:${rollbackTag}" }
  docker tag "${image}:${rollbackTag}" "${image}:latest"
  $files = $seven; $useEight = $false
}
if ($Enable) { $env:FORENSIC_OCR_EASYOCR_ENABLED = 'true' } elseif ($useEight) { $env:FORENSIC_OCR_EASYOCR_ENABLED = 'false' }
Write-Host ("{0}: {1}:latest is now {2}; role {3}" -f $(if ($Rollback) { 'ROLLBACK' } else { 'APPLY' }), $image, ((@(docker image inspect "${image}:latest" | ConvertFrom-Json)[0]).Id.Substring(7, 12)), $(if ($Rollback) { 'gone' } elseif ($Enable) { 'ON' } else { 'OFF' }))
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
$sw = @($now.Config.Env | Where-Object { $_ -match '^FORENSIC_(OCR_EASYOCR_ENABLED|EASYOCR_THREADS|EASYOCR_MAX_SIDE|OCR_ROUTER_URDU_SHARE|ASR_ROUTES_ENABLED)' })
Write-Host ("after: image {0}, health {1}, {2}" -f $now.Image.Substring(7, 12), $now.State.Health.Status, ($sw -join ' '))
Remove-Item Env:\FORENSIC_OCR_EASYOCR_ENABLED -ErrorAction SilentlyContinue
Remove-Item Env:\NEXUSAI_MEDIA_MODEL_DIR -ErrorAction SilentlyContinue
