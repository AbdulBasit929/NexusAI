# LIVE WINDOW, plates in video (docker-compose.forensic-records.video.yaml): recreates ONLY the forensic-records worker on the video overlay image, with exactly the configuration it has now plus
# docker-compose.forensic-records.video.yaml (video plate processor V4 ON, longer job lease, wall-clock budget). Default is a DRY RUN (the compose hash of today's five files must equal the
# running container's label; nothing changes).
#   -Apply     tags the running :latest as :before-<Tag>, tags :<Tag> as :latest and recreates the worker with the video file
#   -Rollback  tags :before-<Tag> as :latest and recreates the worker without the video file (V4 is off again)
# Run from PowerShell:  .\tools\dev\live-window\6-recreate-worker-video.ps1 [-Apply|-Rollback] [-Tag video-v4-20261111]
param([switch]$Apply, [switch]$Rollback, [string]$Tag = 'video-v4-20261111', [switch]$IgnoreInFlight)  # -IgnoreInFlight: queued jobs stay in the queue and are picked up by the new worker (use only with the worker stopped)
$ErrorActionPreference = 'Stop'
$main = 'C:\Users\sheik\Workspace\Office\Projects\NexusAI'
$lw = (Resolve-Path (Join-Path $PSScriptRoot '..\..\..')).Path
$image = 'nexusai-forensic-records-worker'
$env:NEXUSAI_MEDIA_MODEL_DIR = "$lw\models\media"
$four = @('-f', "$main\docker-compose.forensic-records.yaml", '-f', "$lw\docker-compose.forensic-records.ingest-v2.yaml", '-f', "$lw\docker-compose.forensic-records.media.yaml", '-f', "$lw\docker-compose.forensic-records.families.yaml")
$five = $four + @('-f', "$lw\docker-compose.forensic-records.plates.yaml")
$six = $five + @('-f', "$lw\docker-compose.forensic-records.video.yaml")
$base = @('compose', '--project-name', 'nexusai', '--project-directory', $main, '--env-file', "$main\.env.forensic-runtime.local")
$container = 'nexusai-forensic-records-worker-1'
$info = @(docker inspect $container | ConvertFrom-Json)[0]
$label = $info.Config.Labels.'com.docker.compose.config-hash'
$hash5 = (docker @base @five config --hash forensic-records-worker) -replace '^\S+\s+', ''
$hash6 = (docker @base @six config --hash forensic-records-worker) -replace '^\S+\s+', ''
Write-Host ("running image {0}; label {1}" -f $info.Image.Substring(7, 12), $label)
$state = if ($label -eq $hash5) { 'without the video file (V4 off)' } elseif ($label -eq $hash6) { 'WITH the video file (V4 on)' } else { '' }
Write-Host ("compose hash: five files {0}, six files {1} -> {2}" -f $hash5.Substring(0, 12), $hash6.Substring(0, 12), $(if ($state) { "the worker runs $state" } else { 'NOT reproduced' }))
if (-not $state) { throw 'compose does not reproduce the running worker; stop' }
if (-not $Apply -and -not $Rollback) { Write-Host 'dry run only; nothing changed.'; Remove-Item Env:\NEXUSAI_MEDIA_MODEL_DIR -ErrorAction SilentlyContinue; return }
$inflight = docker exec nexusai-forensic-postgres-1 psql -U localrecall -d localrecall -At -c "select count(*) from forensic.records_ingest_jobs where status::text in ('queued','running')"
if ([int]$inflight -ne 0 -and -not $IgnoreInFlight) { throw "jobs are in flight ($inflight); wait for the worker to be idle" }
$rollbackTag = "before-$Tag"
if ($Apply) {
  if (-not (docker images -q "${image}:${Tag}")) { throw "image ${image}:${Tag} does not exist (tools\media_data\Build-VideoV4Overlay.ps1)" }
  if (docker images -q "${image}:${rollbackTag}") { throw "${image}:${rollbackTag} already exists; the rollback must not be overwritten" }
  docker tag "${image}:latest" "${image}:${rollbackTag}"
  docker tag "${image}:${Tag}" "${image}:latest"
  $files = $six
} else {
  if (-not (docker images -q "${image}:${rollbackTag}")) { throw "no rollback image ${image}:${rollbackTag}" }
  docker tag "${image}:${rollbackTag}" "${image}:latest"
  $files = $five
}
Write-Host ("APPLY: {0}:latest is now {1}" -f $image, ((@(docker image inspect "${image}:latest" | ConvertFrom-Json)[0]).Id.Substring(7, 12)))
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
$sw = @($now.Config.Env | Where-Object { $_ -match '^FORENSIC_(ANPR_(ENABLED|ENGINE)|VIDEO_ANPR_V4_ENABLED|WORKER_LEASE_SECONDS)=' })
Write-Host ("after: image {0}, health {1}, {2}" -f $now.Image.Substring(7, 12), $now.State.Health.Status, ($sw -join ' '))
Remove-Item Env:\NEXUSAI_MEDIA_MODEL_DIR -ErrorAction SilentlyContinue
