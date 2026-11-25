# LIVE WINDOW, video plate fix round 2 (docs/data/video-plates-recall-round2-20261125.md; image built by tools/media_data/Build-VideoV41Overlay.ps1): recreates ONLY the forensic-records worker on the
# overlay image, with the configuration it has now plus ONE change: the length limit of a video for plate reading (FORENSIC_VIDEO_ANPR_V4_MAX_DURATION_SECONDS) goes from 900 s to 14400 s (the new default of
# docker-compose.forensic-records.video.yaml). Default is a DRY RUN: with the limit forced back to 900 the compose hash of the nine files must equal the running container's label, which proves that the limit is the
# only change in the configuration; nothing is changed.
#   -Apply     tags the running :latest as :before-<Tag>, tags :<Tag> as :latest and recreates the worker (limit 14400)
#   -Rollback  tags :before-<Tag> as :latest and recreates the worker with the limit back at 900
# Run from PowerShell:  .\tools\dev\live-window\12-recreate-worker-video-v41.ps1 [-Apply|-Rollback] [-Tag video-v41-20261125]
# The owner's word is needed before -Apply (it changes the live worker). Queued jobs wait; the script refuses while a job is running unless -IgnoreInFlight is given.
param([switch]$Apply, [switch]$Rollback, [string]$Tag = 'video-v41-20261125', [switch]$IgnoreInFlight)
$ErrorActionPreference = 'Stop'
$main = 'C:\Users\sheik\Workspace\Office\Projects\NexusAI'
$lw = (Resolve-Path (Join-Path $PSScriptRoot '..\..\..')).Path
$image = 'nexusai-forensic-records-worker'
$secretsFile = "$main\.env.forensic-runtime.local"
$env:NEXUSAI_MEDIA_MODEL_DIR = "$lw\models\media"
$keyLine = @(Get-Content -LiteralPath $secretsFile | Where-Object { $_ -match '^FORENSIC_IMAGE_SEARCH_SERVICE_KEY=.+' } | Select-Object -First 1)
if ($keyLine.Count -gt 0) { $env:FORENSIC_IMAGE_SEARCH_SERVICE_KEY = $keyLine[0].Substring('FORENSIC_IMAGE_SEARCH_SERVICE_KEY='.Length).Trim() }   # read from the secrets file, never printed
$four = @('-f', "$main\docker-compose.forensic-records.yaml", '-f', "$lw\docker-compose.forensic-records.ingest-v2.yaml", '-f', "$lw\docker-compose.forensic-records.media.yaml", '-f', "$lw\docker-compose.forensic-records.families.yaml")
$nine = $four + @('-f', "$lw\docker-compose.forensic-records.plates.yaml", '-f', "$lw\docker-compose.forensic-records.video.yaml", '-f', "$lw\docker-compose.forensic-records.speech-routes.yaml", '-f', "$lw\docker-compose.forensic-records.easyocr.yaml", '-f', "$lw\docker-compose.forensic-records.image-search.yaml")
$base = @('compose', '--project-name', 'nexusai', '--project-directory', $main, '--env-file', $secretsFile)
$container = 'nexusai-forensic-records-worker-1'
$info = @(docker inspect $container | ConvertFrom-Json)[0]
# the compose files interpolate the image-search and EasyOCR switches and the service image tag, which their deployments set in their own environments: take them from the running containers (the key is read above)
foreach ($e in $info.Config.Env) { if ($e -match '^(FORENSIC_IMAGE_SEARCH_(ENABLED|ASK|TILES|TIMEOUT_SECONDS)|FORENSIC_EASYOCR_[A-Z_]+|FORENSIC_OCR_EASYOCR_ENABLED)=(.*)$') { Set-Item -Path ("Env:\" + $Matches[1]) -Value $Matches[3] } }
$svc = @(docker inspect nexusai-forensic-image-search-1 | ConvertFrom-Json)[0]
if ($svc) { $env:FORENSIC_IMAGE_SEARCH_SERVICE_IMAGE = $svc.Config.Image }
$label = $info.Config.Labels.'com.docker.compose.config-hash'
$env:FORENSIC_VIDEO_ANPR_V4_MAX_DURATION_SECONDS = '900'
$hash = (docker @base @nine config --hash forensic-records-worker) -replace '^\S+\s+', ''
Remove-Item Env:\FORENSIC_VIDEO_ANPR_V4_MAX_DURATION_SECONDS -ErrorAction SilentlyContinue
Write-Host ("running image {0}; label {1}; nine files with the limit at 900 s: {2} -> {3}" -f $info.Image.Substring(7, 12), $label.Substring(0, 12), $hash.Substring(0, 12), $(if ($label -eq $hash) { 'the configuration is reproduced' } else { 'NOT reproduced' }))
if ($label -ne $hash) { throw 'compose does not reproduce the running worker; stop' }
if (-not $Apply -and -not $Rollback) { Write-Host 'dry run only; nothing changed.'; Remove-Item Env:\NEXUSAI_MEDIA_MODEL_DIR, Env:\FORENSIC_IMAGE_SEARCH_SERVICE_KEY -ErrorAction SilentlyContinue; return }
$inflight = docker exec nexusai-forensic-postgres-1 psql -U localrecall -d localrecall -At -c "select count(*) from forensic.records_ingest_jobs where status::text in ('queued','running')"
if ([int]$inflight -ne 0 -and -not $IgnoreInFlight) { throw "jobs are in flight ($inflight); wait for the worker to be idle" }
$rollbackTag = "before-$Tag"
if ($Apply) {
  if (-not (docker images -q "${image}:${Tag}")) { throw "image ${image}:${Tag} does not exist (tools\media_data\Build-VideoV41Overlay.ps1)" }
  if (docker images -q "${image}:${rollbackTag}") { throw "${image}:${rollbackTag} already exists; the rollback must not be overwritten" }
  docker tag "${image}:latest" "${image}:${rollbackTag}"
  docker tag "${image}:${Tag}" "${image}:latest"
} else {
  if (-not (docker images -q "${image}:${rollbackTag}")) { throw "no rollback image ${image}:${rollbackTag}" }
  docker tag "${image}:${rollbackTag}" "${image}:latest"
  $env:FORENSIC_VIDEO_ANPR_V4_MAX_DURATION_SECONDS = '900'
}
Write-Host ("{0}: {1}:latest is now {2}" -f $(if ($Apply) { 'APPLY' } else { 'ROLLBACK' }), $image, ((@(docker image inspect "${image}:latest" | ConvertFrom-Json)[0]).Id.Substring(7, 12)))
$ErrorActionPreference = 'Continue'   # compose prints a harmless warning about an orphan container on stderr; the exit code is what counts
docker @base @nine up -d --no-deps --no-build --force-recreate forensic-records-worker 2>&1 | Out-Host
$rc = $LASTEXITCODE
$ErrorActionPreference = 'Stop'
if ($rc -ne 0) { throw "compose up failed (exit $rc)" }
for ($i = 0; $i -lt 40; $i++) {
  $s = (@(docker inspect $container | ConvertFrom-Json)[0]).State
  if ($s.Health.Status -eq 'healthy') { break }
  Start-Sleep -Seconds 3
}
$now = @(docker inspect $container | ConvertFrom-Json)[0]
$sw = @($now.Config.Env | Where-Object { $_ -match '^FORENSIC_(ANPR_(ENABLED|ENGINE)|VIDEO_ANPR_V4_[A-Z_]+|WORKER_LEASE_SECONDS)=' })
Write-Host ("after: image {0}, health {1}, {2}" -f $now.Image.Substring(7, 12), $now.State.Health.Status, ($sw -join ' '))
Remove-Item Env:\NEXUSAI_MEDIA_MODEL_DIR, Env:\FORENSIC_IMAGE_SEARCH_SERVICE_KEY, Env:\FORENSIC_VIDEO_ANPR_V4_MAX_DURATION_SECONDS -ErrorAction SilentlyContinue
