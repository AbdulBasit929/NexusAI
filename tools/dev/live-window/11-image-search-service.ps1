# Recreates ONLY the image-search service container from an overlay image tag (nothing else is touched: not the API, not the worker, not the database, not the language model).
# Used when image_search_service.py or the service block of docker-compose.forensic-records.image-search.yaml changes.
#
#   .\tools\dev\live-window\11-image-search-service.ps1 -Tag image-search-svc-20261123b
#
# The image is built beforehand with a two-line Dockerfile (FROM the running overlay; COPY the new image_search_service.py to /app/). The shared key is read from the secrets file and never printed.
param([string]$Tag = 'image-search-svc-20261123b')
$ErrorActionPreference = 'Stop'
$main = 'C:\Users\sheik\Workspace\Office\Projects\NexusAI'
$lw = (Resolve-Path (Join-Path $PSScriptRoot '..\..\..')).Path
$secretsFile = "$main\.env.forensic-runtime.local"
$serviceContainer = 'nexusai-forensic-image-search-1'
if (-not (docker images -q "nexusai-forensic-records-worker:$Tag")) { throw "image nexusai-forensic-records-worker:$Tag does not exist" }
$keyLine = @(Get-Content -LiteralPath $secretsFile | Where-Object { $_ -match '^FORENSIC_IMAGE_SEARCH_SERVICE_KEY=.+' } | Select-Object -First 1)
if ($keyLine.Count -eq 0) { throw 'no FORENSIC_IMAGE_SEARCH_SERVICE_KEY in the secrets file; run 10-image-search.ps1 first' }
$env:FORENSIC_IMAGE_SEARCH_SERVICE_KEY = $keyLine[0].Substring('FORENSIC_IMAGE_SEARCH_SERVICE_KEY='.Length).Trim()
$env:NEXUSAI_MEDIA_MODEL_DIR = "$lw\models\media"
$env:FORENSIC_IMAGE_SEARCH_SERVICE_IMAGE = "nexusai-forensic-records-worker:$Tag"
$four = @('-f', "$main\docker-compose.forensic-records.yaml", '-f', "$lw\docker-compose.forensic-records.ingest-v2.yaml", '-f', "$lw\docker-compose.forensic-records.media.yaml", '-f', "$lw\docker-compose.forensic-records.families.yaml")
$nine = $four + @('-f', "$lw\docker-compose.forensic-records.plates.yaml", '-f', "$lw\docker-compose.forensic-records.video.yaml", '-f', "$lw\docker-compose.forensic-records.speech-routes.yaml", '-f', "$lw\docker-compose.forensic-records.easyocr.yaml", '-f', "$lw\docker-compose.forensic-records.image-search.yaml")
$base = @('compose', '--project-name', 'nexusai', '--project-directory', $main, '--env-file', $secretsFile)
$ErrorActionPreference = 'Continue'   # compose prints a harmless warning about orphan containers on stderr; the exit code is what counts
docker @base @nine up -d --no-deps --no-build --force-recreate forensic-image-search 2>&1 | Out-Host
if ($LASTEXITCODE -ne 0) { throw "compose up failed for the service (exit $LASTEXITCODE)" }
for ($i = 0; $i -lt 100; $i++) {
  $st = (@(docker inspect $serviceContainer | ConvertFrom-Json)[0]).State
  if ($st.Health -and $st.Health.Status -eq 'healthy') { Write-Host "service healthy on image $Tag"; Remove-Item Env:\NEXUSAI_MEDIA_MODEL_DIR, Env:\FORENSIC_IMAGE_SEARCH_SERVICE_IMAGE, Env:\FORENSIC_IMAGE_SEARCH_SERVICE_KEY -ErrorAction SilentlyContinue; return }
  if (-not $st.Running) { throw "the service container stopped: docker logs $serviceContainer" }
  Start-Sleep -Seconds 3
}
throw "the service container did not become healthy within 5 minutes: docker logs $serviceContainer"
