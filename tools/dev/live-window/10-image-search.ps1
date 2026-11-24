# LIVE WINDOW, image search (module M1; docs/roadmap/image-search-m1-handoff-20261120.md): adds the image-search service container and recreates ONLY the forensic-records worker and the forensic-records API, each with exactly the
# configuration it has now plus docker-compose.forensic-records.image-search.yaml. Nothing else is touched (no database change here: migration 038 is applied first, by hand, with its preflight and verify).
#   (no switch)   DRY RUN: checks the preconditions and shows, for the worker, the API and the service, what compose would render against what runs now (names added or changed); nothing changes.
#   -Apply        1) the service container, 2) the API (image :image-search-20261123 becomes :latest, the old one is kept as :before-image-search-20261123; the Ask step switch is set from -Ask), 3) the worker (same pattern; the
#                 descriptors of new images are switched on from -Descriptors). The shared key is generated once and written to the stack's secrets file (.env.forensic-runtime.local, git-ignored); it is never printed.
#   -Descriptors on|off|keep   FORENSIC_IMAGE_SEARCH_ENABLED of the worker (default on at the first apply)
#   -Ask on|off|keep           FORENSIC_IMAGE_SEARCH_ASK of the API (default on at the first apply)
#   -Rollback     the worker and the API go back to :before-image-search-20261123 with the files they had, and the service container is removed. The database tables stay (rollback.sql of migration 038 is separate).
# Needs: migration 038 applied (ops/038_image_search/verify.sql passes), the two model folders (tools/media_data/Install-ImageSearchModels.ps1), the images built by tools/media_data/build_image_search_overlay.py (worker) and the API overlay.
# After an apply, restart the interface workspace (wslrelay keeps a dead connection to the old API container):  .\tools\dev\Restart-Workspace.ps1 -Port 4181 -Tree lane   (and -Port 4182 -Tree ui)
param([switch]$Apply, [switch]$Rollback, [ValidateSet('keep', 'on', 'off')][string]$Descriptors = 'on', [ValidateSet('keep', 'on', 'off')][string]$Ask = 'on',
      [string]$Tag = 'image-search-20261123', [switch]$IgnoreInFlight, [switch]$ApiOnly)
$ErrorActionPreference = 'Stop'
$main = 'C:\Users\sheik\Workspace\Office\Projects\NexusAI'
$lw = (Resolve-Path (Join-Path $PSScriptRoot '..\..\..')).Path
. (Join-Path $lw 'evaluation\question_factory\Run-Arms.ps1')
$ErrorActionPreference = 'Stop'
$script:AskedSecrets['LOCALAI_API_KEY'] = $true
$env:NEXUSAI_MEDIA_MODEL_DIR = "$lw\models\media"
$env:FORENSIC_IMAGE_SEARCH_SERVICE_IMAGE = "nexusai-forensic-records-worker:$Tag"   # the service runs the overlay image from its first start, before :latest is moved
$workerImage = 'nexusai-forensic-records-worker'
$apiImage = 'nexusai-forensic-records-api'
$overlay = "$lw\docker-compose.forensic-records.image-search.yaml"
$secretsFile = "$main\.env.forensic-runtime.local"
$workerContainer = 'nexusai-forensic-records-worker-1'
$serviceContainer = 'nexusai-forensic-image-search-1'
$rollbackTag = "before-$Tag"

# the file lists of the running worker (the same eight as 9-recreate-worker-easyocr.ps1) and the overlay
$four = @('-f', "$main\docker-compose.forensic-records.yaml", '-f', "$lw\docker-compose.forensic-records.ingest-v2.yaml", '-f', "$lw\docker-compose.forensic-records.media.yaml", '-f', "$lw\docker-compose.forensic-records.families.yaml")
$eight = $four + @('-f', "$lw\docker-compose.forensic-records.plates.yaml", '-f', "$lw\docker-compose.forensic-records.video.yaml", '-f', "$lw\docker-compose.forensic-records.speech-routes.yaml", '-f', "$lw\docker-compose.forensic-records.easyocr.yaml")
$nine = $eight + @('-f', $overlay)
$base = @('compose', '--project-name', 'nexusai', '--project-directory', $main, '--env-file', $secretsFile)

function Get-ContainerEnv([string]$Name) {
  $h = @{}
  foreach ($line in @((@(docker inspect $Name | ConvertFrom-Json)[0]).Config.Env)) { $i = $line.IndexOf('='); if ($i -gt 0) { $h[$line.Substring(0, $i)] = $line.Substring($i + 1) } }
  return $h
}
function Hash12([string]$Value) { $sha = [Security.Cryptography.SHA256]::Create(); return ([BitConverter]::ToString($sha.ComputeHash([Text.Encoding]::UTF8.GetBytes($Value))) -replace '-', '').Substring(0, 12).ToLower() }
function Show-Diff([string]$Label, $Rendered, $Running) {
  $added = @($Rendered.Keys | Where-Object { -not $Running.ContainsKey($_) } | Sort-Object)
  $changed = @($Rendered.Keys | Where-Object { $Running.ContainsKey($_) -and $Running[$_] -ne $Rendered[$_] } | Sort-Object)
  $gone = @($Running.Keys | Where-Object { $_ -match '^(FORENSIC|NEXUSAI|NATS|LOCALAI)_' -and -not $Rendered.ContainsKey($_) } | Sort-Object)
  Write-Host ("{0}: compose renders {1} variables; ADDED [{2}]; CHANGED [{3}]; would be LOST [{4}]" -f $Label, $Rendered.Count, ($added -join ', '), ($changed -join ', '), ($gone -join ', '))
  return [pscustomobject]@{ Added = $added; Changed = $changed; Lost = $gone }
}
function Get-Rendered([string[]]$Files, [string]$Service) {
  $json = (& docker @base @Files config --format json $Service) | ConvertFrom-Json
  $h = @{}
  foreach ($p in $json.services.$Service.environment.PSObject.Properties) { $h[$p.Name] = [string]$p.Value }
  return $h
}

# ---- the shared key: generated once, kept in the stack's secrets file, never printed ----
function Ensure-Key {
  $line = @(Get-Content -LiteralPath $secretsFile -ErrorAction SilentlyContinue | Where-Object { $_ -match '^FORENSIC_IMAGE_SEARCH_SERVICE_KEY=.+' } | Select-Object -First 1)
  if ($line.Count -gt 0) { return $true }
  if (-not $Apply) { return $false }
  $bytes = New-Object byte[] 32
  [Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($bytes)
  $key = ([BitConverter]::ToString($bytes) -replace '-', '').ToLower()
  Add-Content -LiteralPath $secretsFile -Value "FORENSIC_IMAGE_SEARCH_SERVICE_KEY=$key" -Encoding ascii
  Write-Host 'a new shared key was written to the secrets file (not shown)'
  return $true
}

# ---- preconditions ----
$problems = @()
$verify = Get-Content -Raw -LiteralPath (Join-Path $lw 'db\forensic_records\ops\038_image_search\verify.sql') | docker exec -i nexusai-forensic-postgres-1 psql -v ON_ERROR_STOP=1 -U localrecall -d localrecall -At -f - 2>&1
if (($verify -join "`n") -notmatch 'migration_038_verified') { $problems += 'migration 038 is not applied (or its verify fails): apply db/forensic_records/038_image_search.sql after ops/038_image_search/preflight.sql' }
foreach ($t in @($(if (-not $ApiOnly) { "${workerImage}:$Tag" }), "${apiImage}:$Tag")) { if ($t -and -not (docker images -q $t)) { $problems += "image $t does not exist" } }
foreach ($m in @('dinov2-base', 'siglip2-base-patch16-224')) { if (-not (Test-Path -LiteralPath "$lw\models\media\image-search\$m\model.safetensors")) { $problems += "model folder models\media\image-search\$m is missing (Install-ImageSearchModels.ps1)" } }
if (-not (Test-Path -LiteralPath $secretsFile)) { $problems += "secrets file $secretsFile is missing" }
$inflight = docker exec nexusai-forensic-postgres-1 psql -U localrecall -d localrecall -At -c "select count(*) from forensic.records_ingest_jobs where status::text in ('queued','running')"
if ([int]$inflight -ne 0 -and -not $IgnoreInFlight -and ($Apply -or $Rollback)) { $problems += "jobs are in flight ($inflight); wait for the worker to be idle (or -IgnoreInFlight)" }
Write-Host ("preconditions: {0}" -f $(if ($problems.Count) { 'NOT MET' } else { 'met' }))
$problems | ForEach-Object { Write-Host "  - $_" }
if ($problems.Count -and ($Apply -or $Rollback)) { throw 'preconditions not met; nothing was changed' }

$s = Get-Stack
$s.Files = @($s.Files | Select-Object -Unique)
$apiRunning = Get-ContainerEnv $s.Api
$workerRunning = Get-ContainerEnv $workerContainer
Write-Host ("running: API {0} ({1}); worker {2}" -f $s.Api, $s.ImageId.Substring(7, 12), ((@(docker inspect $workerContainer | ConvertFrom-Json)[0]).Image.Substring(7, 12)))

$keyPresent = Ensure-Key
$keyLine = @(Get-Content -LiteralPath $secretsFile -ErrorAction SilentlyContinue | Where-Object { $_ -match '^FORENSIC_IMAGE_SEARCH_SERVICE_KEY=.+' } | Select-Object -First 1)
if ($keyLine.Count -gt 0) { $env:FORENSIC_IMAGE_SEARCH_SERVICE_KEY = $keyLine[0].Substring('FORENSIC_IMAGE_SEARCH_SERVICE_KEY='.Length).Trim() }
if (-not $keyPresent) { Write-Host 'dry run: no shared key exists yet; one will be created in the secrets file at -Apply' ; $env:FORENSIC_IMAGE_SEARCH_SERVICE_KEY = 'dry-run-placeholder' }

# ---- what each recreate would change (a dry run, and the check before every apply) ----
$workerSwitch = if ($Descriptors -eq 'keep') { ($workerRunning['FORENSIC_IMAGE_SEARCH_ENABLED']) } elseif ($Descriptors -eq 'on') { 'true' } else { 'false' }
if (-not $workerSwitch) { $workerSwitch = 'false' }
$askSwitch = if ($Ask -eq 'keep') { ($apiRunning['FORENSIC_IMAGE_SEARCH_ASK']) } elseif ($Ask -eq 'on') { 'true' } else { 'false' }
if (-not $askSwitch) { $askSwitch = 'false' }
foreach ($name in 'FORENSIC_OCR_EASYOCR_ENABLED', 'FORENSIC_EASYOCR_LATIN_REREAD', 'FORENSIC_EASYOCR_TABLE_ROWS') { if ($workerRunning.ContainsKey($name)) { Set-Item -Path "Env:\$name" -Value $workerRunning[$name] } }
$env:FORENSIC_IMAGE_SEARCH_ENABLED = $workerSwitch
$renderedWorker = Get-Rendered $nine 'forensic-records-worker'
$diffWorker = Show-Diff 'worker' $renderedWorker $workerRunning
$renderedService = Get-Rendered $nine 'forensic-image-search'
Write-Host ("service: compose renders {0} variables for the new container {1}" -f $renderedService.Count, $serviceContainer)
# the API: the files of the running container plus the overlay, with the environment copied from the container (Run-Arms helpers) and the Ask switch set
$apiFiles = @()
foreach ($f in $s.Files) { $mine = Join-Path $lw (Split-Path $f -Leaf); if (Test-Path -LiteralPath $mine) { $f = $mine }; $apiFiles += @('-f', $f) }
$apiFiles += @('-f', $overlay)
$apiSaved = Import-ContainerEnv $s
$env:FORENSIC_IMAGE_SEARCH_ASK = $askSwitch
Ensure-ComposeSecrets $s
$apiBase = @('compose', '--project-name', $s.Project, '--project-directory', $s.WorkDir)
if ($s.EnvFile) { $apiBase += @('--env-file', $s.EnvFile) }
$jsonApi = (& docker @apiBase @apiFiles config --format json $s.Service) | ConvertFrom-Json
$renderedApi = @{}
foreach ($p in $jsonApi.services.($s.Service).environment.PSObject.Properties) { $renderedApi[$p.Name] = [string]$p.Value }
$diffApi = Show-Diff 'API' $renderedApi $apiRunning
Restore-ProcessEnv $apiSaved
if (($Apply -or $Rollback) -and ($renderedApi.Count -lt 20 -or $renderedWorker.Count -lt 20 -or $diffApi.Changed.Count -gt 0 -or $diffWorker.Changed.Count -gt 0 -or $diffApi.Lost.Count -gt 0 -or $diffWorker.Lost.Count -gt 0)) {
  throw 'compose would not reproduce the running containers (too few variables rendered, or settings changed or lost); nothing was changed'
}
if (-not $Apply -and -not $Rollback) {
  $unexpected = @(@($diffWorker.Added) + @($diffApi.Added) | Where-Object { $_ -notmatch '^FORENSIC_IMAGE_SEARCH_' })
  Write-Host ("dry run only; nothing changed. Names added by compose that are not image-search settings: [{0}]; settings that would be lost: worker [{1}] API [{2}]" -f ($unexpected -join ', '), ($diffWorker.Lost -join ', '), ($diffApi.Lost -join ', '))
  Remove-Item Env:\NEXUSAI_MEDIA_MODEL_DIR, Env:\FORENSIC_IMAGE_SEARCH_ENABLED, Env:\FORENSIC_IMAGE_SEARCH_ASK -ErrorAction SilentlyContinue
  return
}

# :latest becomes the new image and the old one is kept under the rollback tag; run twice, it does nothing the second time (the rollback tag is never overwritten)
function Move-Latest([string]$Image) {
  $latest = (@(docker image inspect "${Image}:latest" | ConvertFrom-Json)[0]).Id
  $new = (@(docker image inspect "${Image}:$Tag" | ConvertFrom-Json)[0]).Id
  if ($latest -eq $new) { Write-Host "${Image}:latest already is ${Image}:$Tag"; return }
  if (docker images -q "${Image}:$rollbackTag") { throw "${Image}:$rollbackTag already exists and :latest is another image; resolve by hand" }
  docker tag "${Image}:latest" "${Image}:$rollbackTag"
  docker tag "${Image}:$Tag" "${Image}:latest"
}
function Wait-Container([string]$Name, [int]$Seconds = 240) {
  for ($i = 0; $i -lt ($Seconds / 3); $i++) {
    $st = (@(docker inspect $Name | ConvertFrom-Json)[0]).State
    if ($st.Health -and $st.Health.Status -eq 'healthy') { return $true }
    if (-not $st.Running) { return $false }
    Start-Sleep -Seconds 3
  }
  return $false
}
$ErrorActionPreference = 'Continue'   # compose prints a harmless warning about orphan containers on stderr; exit codes are what count

if ($Rollback) {
  foreach ($pair in @(@($workerImage), @($apiImage))) {
    $img = $pair[0]
    if (-not (docker images -q "${img}:$rollbackTag")) { throw "no rollback image ${img}:$rollbackTag" }
    docker tag "${img}:$rollbackTag" "${img}:latest"
  }
  Remove-Item Env:\FORENSIC_IMAGE_SEARCH_ENABLED, Env:\FORENSIC_IMAGE_SEARCH_ASK -ErrorAction SilentlyContinue
  docker @base @eight up -d --no-deps --no-build --force-recreate forensic-records-worker 2>&1 | Out-Host
  $apiSaved = Import-ContainerEnv $s
  Ensure-ComposeSecrets $s
  $apiOld = @(); foreach ($f in $s.Files) { if ((Split-Path $f -Leaf) -ne 'docker-compose.forensic-records.image-search.yaml') { $mine = Join-Path $lw (Split-Path $f -Leaf); if (Test-Path -LiteralPath $mine) { $f = $mine }; $apiOld += @('-f', $f) } }
  docker @apiBase @apiOld up -d --no-deps --no-build --force-recreate $s.Service 2>&1 | Out-Host
  Restore-ProcessEnv $apiSaved
  docker rm -f $serviceContainer 2>&1 | Out-Host
  Write-Host 'ROLLBACK done: worker and API are on the previous images with the previous files; the service container is removed; the database tables of migration 038 are untouched'
  return
}

# ---- apply: service, API, worker ----
if (-not $ApiOnly) {
  Write-Host '--- 1. the service container'
  docker @base @nine up -d --no-deps --no-build forensic-image-search 2>&1 | Out-Host
  if (-not (Wait-Container $serviceContainer 300)) { throw "the service container did not become healthy: docker logs $serviceContainer" }
  Write-Host 'service healthy'
}

Write-Host '--- 2. the API'
Move-Latest $apiImage
$apiSaved = Import-ContainerEnv $s
$env:FORENSIC_IMAGE_SEARCH_ASK = $askSwitch
Ensure-ComposeSecrets $s
docker @apiBase @apiFiles up -d --no-deps --no-build --force-recreate $s.Service 2>&1 | Out-Host
$rc = $LASTEXITCODE
Restore-ProcessEnv $apiSaved
if ($rc -ne 0) { throw "compose up failed for the API (exit $rc): run with -Rollback" }
if (-not (Wait-Healthy)) { throw 'the API did not become healthy within 3 minutes: run with -Rollback' }
$s2 = Get-Stack
Write-Host ("API now: image {0}; Ask step {1}" -f $s2.ImageId.Substring(7, 12), $askSwitch)

if ($ApiOnly) {
  Remove-Item Env:\NEXUSAI_MEDIA_MODEL_DIR, Env:\FORENSIC_IMAGE_SEARCH_ENABLED, Env:\FORENSIC_IMAGE_SEARCH_ASK -ErrorAction SilentlyContinue
  Write-Host 'done (API only). Restart the workspace now: .\tools\dev\Restart-Workspace.ps1 -Port 4181 -Tree main  (and -Port 4182 -Tree ui)'
  return
}
Write-Host '--- 3. the worker'
Move-Latest $workerImage
$env:FORENSIC_IMAGE_SEARCH_ENABLED = $workerSwitch
docker @base @nine up -d --no-deps --no-build --force-recreate forensic-records-worker 2>&1 | Out-Host
$rc = $LASTEXITCODE
if ($rc -ne 0) { throw "compose up failed for the worker (exit $rc): run with -Rollback" }
if (-not (Wait-Container $workerContainer 240)) { throw 'the worker did not become healthy: run with -Rollback' }
$w = @(docker inspect $workerContainer | ConvertFrom-Json)[0]
Write-Host ("worker now: image {0}; descriptors {1}; memory limit {2} MB" -f $w.Image.Substring(7, 12), $workerSwitch, [int]($w.HostConfig.Memory / 1MB))
Remove-Item Env:\NEXUSAI_MEDIA_MODEL_DIR, Env:\FORENSIC_IMAGE_SEARCH_ENABLED, Env:\FORENSIC_IMAGE_SEARCH_ASK -ErrorAction SilentlyContinue
Write-Host 'done. Restart the workspace now: .\tools\dev\Restart-Workspace.ps1 -Port 4181 -Tree lane  (and -Port 4182 -Tree ui)'
