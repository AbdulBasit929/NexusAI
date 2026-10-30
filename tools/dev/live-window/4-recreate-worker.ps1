# LIVE WINDOW, step 4 (docs/data/migrations-027-031-runbook.md section 7): recreates ONLY the forensic-records worker on the families overlay image, with exactly the configuration it has now plus the two
# switches of docker-compose.forensic-records.families.yaml. Default is a DRY RUN (the compose hash of today's three files must equal the running container's label; the rendered change is listed).
#   -Apply     retags nexusai-forensic-records-worker:families-20261030 as :latest and recreates the worker
#   -Rollback  retags :before-families-20261030 (the image that ran before) as :latest and recreates the worker without the families file
# Run from PowerShell:  .\tools\dev\live-window\4-recreate-worker.ps1 [-Apply|-Rollback]
param([switch]$Apply, [switch]$Rollback, [string]$To = 'families-20261030')
$ErrorActionPreference = 'Stop'
$main = 'C:\Users\sheik\Workspace\Office\Projects\NexusAI'
$lw = (Resolve-Path (Join-Path $PSScriptRoot '..\..\..')).Path
$image = 'nexusai-forensic-records-worker'
$env:NEXUSAI_MEDIA_MODEL_DIR = "$lw\models\media"
$today = @('-f', "$main\docker-compose.forensic-records.yaml", '-f', "$lw\docker-compose.forensic-records.ingest-v2.yaml", '-f', "$lw\docker-compose.forensic-records.media.yaml")
$families = $today + @('-f', "$lw\docker-compose.forensic-records.families.yaml")
$base = @('compose', '--project-name', 'nexusai', '--project-directory', $main, '--env-file', "$main\.env.forensic-runtime.local")
$container = 'nexusai-forensic-records-worker-1'
$info = @(docker inspect $container | ConvertFrom-Json)[0]
$label = $info.Config.Labels.'com.docker.compose.config-hash'
$computed = (docker @base @today config --hash forensic-records-worker) -replace '^\S+\s+', ''
Write-Host ("running image {0}; label {1}" -f $info.Image.Substring(7, 12), $label)
Write-Host ("compose hash of today's three files: {0} -> {1}" -f $computed, $(if ($computed -eq $label) { 'IDENTICAL: a recreate reproduces the running configuration' } else { 'DIFFERENT: stop, do not apply' }))
if ($computed -ne $label) { throw 'compose does not reproduce the running worker; stop' }
if (-not $Apply -and -not $Rollback) { Write-Host 'dry run only; nothing changed.'; return }
$inflight = docker exec nexusai-forensic-postgres-1 psql -U localrecall -d localrecall -At -c "select count(*) from forensic.records_ingest_jobs where status::text in ('queued','running')"
if ([int]$inflight -ne 0) { throw "jobs are in flight ($inflight); wait for the worker to be idle" }
$tag = if ($Rollback) { 'before-families-20261030' } else { $To }
$id = (@(docker image inspect "${image}:${tag}" | ConvertFrom-Json)[0]).Id
Write-Host ("APPLY: {0}:latest <- {0}:{1} ({2})" -f $image, $tag, $id.Substring(7, 12))
docker tag "${image}:${tag}" "${image}:latest"
$files = if ($Rollback) { $today } else { $families }
$ErrorActionPreference = 'Continue'   # compose prints a harmless warning about the orphan container nexusai-api-1-prev-gbnf on stderr; the exit code is what counts
docker @base @files up -d --no-deps --no-build --force-recreate forensic-records-worker 2>&1 | Out-Host
$rc = $LASTEXITCODE
$ErrorActionPreference = 'Stop'
if ($rc -ne 0) { throw "compose up failed (exit $rc)" }
for ($i = 0; $i -lt 40; $i++) {
  $state = (@(docker inspect $container | ConvertFrom-Json)[0]).State
  if ($state.Health.Status -eq 'healthy') { break }
  Start-Sleep -Seconds 3
}
$now = @(docker inspect $container | ConvertFrom-Json)[0]
$switches = @($now.Config.Env | Where-Object { $_ -match '^FORENSIC_(INGEST_V2|DOCUMENT_MENTIONS|DOCUMENT_GAZETTEER)=' })
Write-Host ("after: image {0}, health {1}, switches: {2}" -f $now.Image.Substring(7, 12), $now.State.Health.Status, ($switches -join ' '))
Remove-Item Env:\NEXUSAI_MEDIA_MODEL_DIR -ErrorAction SilentlyContinue
