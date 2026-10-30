# LIVE WINDOW, step 3 (docs/data/migrations-027-031-runbook.md section 7): recreates ONLY the forensic-records API container (the Ask engine) on the image tagged families-20261030, with exactly the
# configuration it has now. Directly after step 2: until then the running engine reads the new, empty views from the database. Needs the Run-Arms.ps1 helpers (secrets are copied from the running
# container or the secrets file, never printed).
#   (no switch)  DRY RUN: compose's config hash must equal the running container's label; the rendered environment is compared; nothing changes
#   -Apply       retags nexusai-forensic-records-api:families-20261030 as :latest and recreates the API; compares the environment before and after
#   -Rollback    retags :before-families-20261030 (the image that ran before) as :latest and recreates the API
# Run from PowerShell:  .\tools\dev\live-window\3-recreate-api.ps1 [-Apply|-Rollback]     Afterwards restart the interface workspace:  .\tools\dev\Restart-Workspace.ps1 -Port 4181 -Tree lane
param([switch]$Apply, [switch]$Rollback, [string]$To = 'families-20261030')
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot '..\..\..\evaluation\question_factory\Run-Arms.ps1')
$ErrorActionPreference = 'Stop'
$script:AskedSecrets['LOCALAI_API_KEY'] = $true

function Get-EnvFingerprint($Stack) {
  $h = @{}
  foreach ($line in $Stack.Env) {
    $i = $line.IndexOf('=')
    if ($i -lt 1) { continue }
    $sha = [Security.Cryptography.SHA256]::Create()
    $h[$line.Substring(0, $i)] = ([BitConverter]::ToString($sha.ComputeHash([Text.Encoding]::UTF8.GetBytes($line.Substring($i + 1)))) -replace '-', '').Substring(0, 12).ToLower()
  }
  return $h
}

$s = Get-Stack
$s.Files = @($s.Files | Select-Object -Unique)          # the label of the running container lists the same three files many times; once is enough
$image = 'nexusai-forensic-records-api'
Write-Host ("running container {0}: image {1} ({2}); project {3}, service {4}; compose files: {5}" -f $s.Api, $s.ImageName, $s.ImageId.Substring(7, 12), $s.Project, $s.Service, (($s.Files | ForEach-Object { Split-Path $_ -Leaf }) -join ', '))
$before = Get-EnvFingerprint $s
Write-Host ("environment: {0} variables" -f $before.Count)

$saved = Import-ContainerEnv $s
Ensure-ComposeSecrets $s
try {
  Write-Host "--- what compose would render (hash) against the running container's label"
  $label = (@(docker inspect $s.Api | ConvertFrom-Json)[0]).Config.Labels.'com.docker.compose.config-hash'
  Write-Host ("running label: {0}" -f $label)
  [void](Invoke-Compose $s @('config', '--hash', $s.Service))
  if (-not $Apply -and -not $Rollback) {
    Write-Host "--- dry run only. Rendered environment against the running one:"
    $a = @('compose', '-p', $s.Project)
    foreach ($f in $s.Files) { $mine = Join-Path $script:Repo (Split-Path $f -Leaf); if (Test-Path -LiteralPath $mine) { $f = $mine }; $a += @('-f', $f) }
    if ($s.EnvFile) { $a += @('--env-file', $s.EnvFile) }
    $a += @('config', '--format', 'json', $s.Service)
    Push-Location $script:Repo
    try { $cfg = (& docker @a) | ConvertFrom-Json } finally { Pop-Location }
    $want = @{}
    foreach ($p in $cfg.services.($s.Service).environment.PSObject.Properties) { $sha = [Security.Cryptography.SHA256]::Create(); $want[$p.Name] = ([BitConverter]::ToString($sha.ComputeHash([Text.Encoding]::UTF8.GetBytes([string]$p.Value))) -replace '-', '').Substring(0, 12).ToLower() }
    $onlyWant = @($want.Keys | Where-Object { -not $before.ContainsKey($_) })
    $differ = @($want.Keys | Where-Object { $before.ContainsKey($_) -and $before[$_] -ne $want[$_] })
    Write-Host ("compose renders {0} variables; would ADD: [{1}]; DIFFER from the container: [{2}]" -f $want.Count, ($onlyWant -join ', '), ($differ -join ', '))
    Write-Host ("service image in compose: {0}; build section: {1}" -f $cfg.services.($s.Service).image, [bool]$cfg.services.($s.Service).build)
    return
  }
  $tag = if ($Rollback) { 'before-families-20261030' } else { $To }
  $id = (@(docker image inspect "${image}:${tag}" | ConvertFrom-Json)[0]).Id
  if (-not $id) { throw "no image ${image}:${tag}" }
  Write-Host ("--- APPLY: {0}:latest <- {0}:{1} ({2})" -f $image, $tag, $id.Substring(7, 12))
  docker tag "${image}:${tag}" "${image}:latest"
  $ErrorActionPreference = 'Continue'   # compose prints a harmless warning about the orphan container nexusai-api-1-prev-gbnf on stderr; the exit code is what counts
  $rc = Invoke-Compose $s @('up', '-d', '--no-deps', '--no-build', '--force-recreate', $s.Service)
  $ErrorActionPreference = 'Stop'
  if ($rc -ne 0) { throw "compose up failed (exit $rc)" }
} finally { Restore-ProcessEnv $saved }

if (-not (Wait-Healthy)) { throw "the API did not become healthy within 3 minutes: roll back with -Rollback" }
$after = Get-Stack
$fa = Get-EnvFingerprint $after
$changed = @(($before.Keys + $fa.Keys) | Select-Object -Unique | Where-Object { $before[$_] -ne $fa[$_] })
Write-Host ("after: container {0}, image {1}; environment {2} -> {3} variables; names added/removed/changed: [{4}]" -f $after.Api, $after.ImageId.Substring(7, 12), $before.Count, $fa.Count, ($changed -join ', '))
