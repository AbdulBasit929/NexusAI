# LIVE WINDOW, plates (tools/media_data/PLATES_GO_LIVE.md): recreates ONLY the forensic-records worker on the plates overlay image, with exactly the configuration it has now plus
# docker-compose.forensic-records.plates.yaml (plate role ON, engine nexus, model files pinned by SHA-256). Default is a DRY RUN (the compose hash of today's four files must equal the
# running container's label; nothing changes).
#   -Apply     tags the running :latest as :before-<Tag>, tags :<Tag> as :latest and recreates the worker with the plates file
#   -Rollback  tags :before-<Tag> as :latest and recreates the worker without the plates file (the plate role is off again)
# Run from PowerShell:  .\tools\dev\live-window\5-recreate-worker-plates.ps1 [-Apply|-Rollback] [-Tag plates-20261110d]
param([switch]$Apply, [switch]$Rollback, [string]$Tag = 'plates-20261110d')
$ErrorActionPreference = 'Stop'
$main = 'C:\Users\sheik\Workspace\Office\Projects\NexusAI'
$lw = (Resolve-Path (Join-Path $PSScriptRoot '..\..\..')).Path
$image = 'nexusai-forensic-records-worker'
$env:NEXUSAI_MEDIA_MODEL_DIR = "$lw\models\media"
$four = @('-f', "$main\docker-compose.forensic-records.yaml", '-f', "$lw\docker-compose.forensic-records.ingest-v2.yaml", '-f', "$lw\docker-compose.forensic-records.media.yaml", '-f', "$lw\docker-compose.forensic-records.families.yaml")
$five = $four + @('-f', "$lw\docker-compose.forensic-records.plates.yaml")
$base = @('compose', '--project-name', 'nexusai', '--project-directory', $main, '--env-file', "$main\.env.forensic-runtime.local")
$container = 'nexusai-forensic-records-worker-1'
$info = @(docker inspect $container | ConvertFrom-Json)[0]
$label = $info.Config.Labels.'com.docker.compose.config-hash'
$hash4 = (docker @base @four config --hash forensic-records-worker) -replace '^\S+\s+', ''
$hash5 = (docker @base @five config --hash forensic-records-worker) -replace '^\S+\s+', ''
Write-Host ("running image {0}; label {1}" -f $info.Image.Substring(7, 12), $label)
$state = if ($label -eq $hash4) { 'without the plates file (plate role off)' } elseif ($label -eq $hash5) { 'WITH the plates file (plate role on)' } else { '' }
Write-Host ("compose hash: four files {0}, five files {1} -> {2}" -f $hash4.Substring(0, 12), $hash5.Substring(0, 12), $(if ($state) { "the worker runs $state" } else { 'NOT reproduced' }))
if (-not $state) { throw 'compose does not reproduce the running worker; stop' }
if (-not $Apply -and -not $Rollback) { Write-Host 'dry run only; nothing changed.'; Remove-Item Env:\NEXUSAI_MEDIA_MODEL_DIR -ErrorAction SilentlyContinue; return }
$inflight = docker exec nexusai-forensic-postgres-1 psql -U localrecall -d localrecall -At -c "select count(*) from forensic.records_ingest_jobs where status::text in ('queued','running')"
if ([int]$inflight -ne 0) { throw "jobs are in flight ($inflight); wait for the worker to be idle" }
$rollbackTag = "before-$Tag"
if ($Apply) {
  if (-not (docker images -q "${image}:${Tag}")) { throw "image ${image}:${Tag} does not exist (Build-PlatesOverlay.ps1)" }
  if (docker images -q "${image}:${rollbackTag}") { throw "${image}:${rollbackTag} already exists; the rollback must not be overwritten" }
  docker tag "${image}:latest" "${image}:${rollbackTag}"
  docker tag "${image}:${Tag}" "${image}:latest"
  $files = $five
} else {
  if (-not (docker images -q "${image}:${rollbackTag}")) { throw "no rollback image ${image}:${rollbackTag}" }
  docker tag "${image}:${rollbackTag}" "${image}:latest"
  $files = $four
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
$sw = @($now.Config.Env | Where-Object { $_ -match '^FORENSIC_ANPR_(ENABLED|ENGINE|ONNX_PROVIDERS)=' })
Write-Host ("after: image {0}, health {1}, {2}" -f $now.Image.Substring(7, 12), $now.State.Health.Status, ($sw -join ' '))
Remove-Item Env:\NEXUSAI_MEDIA_MODEL_DIR -ErrorAction SilentlyContinue
