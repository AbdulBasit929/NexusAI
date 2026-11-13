# LIVE WINDOW, speech routes (tools/media_data/Build-SpeechRoutesOverlay.ps1): recreates ONLY the forensic-records worker on the overlay image, with exactly the configuration it has now plus the speech-routes compose
# file: an image swap and four settings (FORENSIC_ASR_ROUTES_ENABLED and its path, threads, time budget). Default is a DRY RUN (the compose hash of the six current files must equal the running container's label; nothing changes).
#   -Apply     tags the running :latest as :before-<Tag>, tags :<Tag> as :latest and recreates the worker with the seven files
#   -Rollback  tags :before-<Tag> as :latest and recreates the worker with the six files (the routes are off again, every language goes to the Whisper service as before)
# Needs the model folder models\media\speech\omni-ctc-300m-hf in this checkout (tools\media_data\Install-SpeechRouteModel.ps1); the dry run checks it.
# Run from PowerShell:  .\tools\dev\live-window\8-recreate-worker-speech-routes.ps1 [-Apply|-Rollback] [-Tag speech-routes-20261113]
# After an apply, restart the workspace if the API became unreachable (wslrelay), as after every recreate.
param([switch]$Apply, [switch]$Rollback, [string]$Tag = 'speech-routes-20261113', [switch]$IgnoreInFlight)  # -IgnoreInFlight: queued jobs stay in the queue and are picked up by the new worker (use only with the worker stopped)
$ErrorActionPreference = 'Stop'
$main = 'C:\Users\sheik\Workspace\Office\Projects\NexusAI'
$lw = (Resolve-Path (Join-Path $PSScriptRoot '..\..\..')).Path
$image = 'nexusai-forensic-records-worker'
$env:NEXUSAI_MEDIA_MODEL_DIR = "$lw\models\media"
$four = @('-f', "$main\docker-compose.forensic-records.yaml", '-f', "$lw\docker-compose.forensic-records.ingest-v2.yaml", '-f', "$lw\docker-compose.forensic-records.media.yaml", '-f', "$lw\docker-compose.forensic-records.families.yaml")
$six = $four + @('-f', "$lw\docker-compose.forensic-records.plates.yaml", '-f', "$lw\docker-compose.forensic-records.video.yaml")
$seven = $six + @('-f', "$lw\docker-compose.forensic-records.speech-routes.yaml")
$base = @('compose', '--project-name', 'nexusai', '--project-directory', $main, '--env-file', "$main\.env.forensic-runtime.local")
$container = 'nexusai-forensic-records-worker-1'
$model = "$lw\models\media\speech\omni-ctc-300m-hf\model.safetensors"
$info = @(docker inspect $container | ConvertFrom-Json)[0]
$label = $info.Config.Labels.'com.docker.compose.config-hash'
$hash6 = (docker @base @six config --hash forensic-records-worker) -replace '^\S+\s+', ''
$hash7 = (docker @base @seven config --hash forensic-records-worker) -replace '^\S+\s+', ''
$state = if ($label -eq $hash6) { 'the six files (routes off)' } elseif ($label -eq $hash7) { 'the seven files (routes on)' } else { '' }
Write-Host ("running image {0}; label {1}" -f $info.Image.Substring(7, 12), $label)
Write-Host ("compose hash: six files {0}, seven files {1} -> {2}" -f $hash6.Substring(0, 12), $hash7.Substring(0, 12), $(if ($state) { "the worker runs $state" } else { 'NOT reproduced' }))
Write-Host ("model folder: {0}" -f $(if (Test-Path -LiteralPath $model) { 'present' } else { 'MISSING (Install-SpeechRouteModel.ps1)' }))
if (-not $state) { throw 'compose does not reproduce the running worker; stop' }
if (-not $Apply -and -not $Rollback) { Write-Host 'dry run only; nothing changed.'; Remove-Item Env:\NEXUSAI_MEDIA_MODEL_DIR -ErrorAction SilentlyContinue; return }
if ($Apply -and -not (Test-Path -LiteralPath $model)) { throw "the model folder is missing: $model" }
$inflight = docker exec nexusai-forensic-postgres-1 psql -U localrecall -d localrecall -At -c "select count(*) from forensic.records_ingest_jobs where status::text in ('queued','running')"
if ([int]$inflight -ne 0 -and -not $IgnoreInFlight) { throw "jobs are in flight ($inflight); wait for the worker to be idle" }
$rollbackTag = "before-$Tag"
if ($Apply) {
  if (-not (docker images -q "${image}:${Tag}")) { throw "image ${image}:${Tag} does not exist (tools\media_data\Build-SpeechRoutesOverlay.ps1)" }
  if (docker images -q "${image}:${rollbackTag}") { throw "${image}:${rollbackTag} already exists; the rollback must not be overwritten" }
  docker tag "${image}:latest" "${image}:${rollbackTag}"
  docker tag "${image}:${Tag}" "${image}:latest"
  $files = $seven
} else {
  if (-not (docker images -q "${image}:${rollbackTag}")) { throw "no rollback image ${image}:${rollbackTag}" }
  docker tag "${image}:${rollbackTag}" "${image}:latest"
  $files = $six
}
Write-Host ("{0}: {1}:latest is now {2}" -f $(if ($Apply) { 'APPLY' } else { 'ROLLBACK' }), $image, ((@(docker image inspect "${image}:latest" | ConvertFrom-Json)[0]).Id.Substring(7, 12)))
$ErrorActionPreference = 'Continue'   # compose prints a harmless warning about an orphan container on stderr; the exit code is what counts
docker @base @files up -d --no-deps --no-build --force-recreate forensic-records-worker 2>&1 | Out-Host
$rc = $LASTEXITCODE
$ErrorActionPreference = 'Stop'
if ($rc -ne 0) { throw "compose up failed (exit $rc)" }
for ($i = 0; $i -lt 40; $i++) {
  $s = (@(docker inspect $container | ConvertFrom-Json)[0]).State
  if ($s.Health.Status -eq 'healthy') { break }
  Start-Sleep -Seconds 3
}
$now = @(docker inspect $container | ConvertFrom-Json)[0]
$sw = @($now.Config.Env | Where-Object { $_ -match '^FORENSIC_(ASR_ROUTES_ENABLED|ASR_ROUTE_|ASR_ENABLED|VIDEO_ANPR_V4_ENABLED|WORKER_LEASE_SECONDS)' })
Write-Host ("after: image {0}, health {1}, {2}" -f $now.Image.Substring(7, 12), $now.State.Health.Status, ($sw -join ' '))
Remove-Item Env:\NEXUSAI_MEDIA_MODEL_DIR -ErrorAction SilentlyContinue
