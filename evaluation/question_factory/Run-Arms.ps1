# GOVERNED SQL LANE: THE THREE ARMS (the owner runs this; gates are in reports/governed-sql-20261005/PREREGISTRATION.md)
#
#   cd <repo root>
#   git fetch origin claude/gifted-pasteur-4kpx0r
#   git checkout origin/claude/gifted-pasteur-4kpx0r -- api semantic_layer go.mod go.sum docker-compose.forensic-records.yaml evaluation/question_factory reports/governed-sql-20261005
#   . .\evaluation\question_factory\Run-Arms.ps1
#   Show-Stack          # read-only: what is running, which switches it has
#   Build-LaneImage     # builds the forensic API image with the lane in it (switches stay off); remembers the old image
#   Run-Arm A           # new image, both switches off  (the control: must equal arm-baseline-v1)
#   Run-Arm C           # lane only after the existing path declines
#   Run-Arm B           # lane before the existing path
#   Compare-Arms        # A vs baseline, A vs B, A vs C
#   Restore-Stack       # puts the original image and switches back
#
# Each Run-Arm recreates ONLY the API container, with the environment it already had plus the two lane switches, and checks the
# switch state from `docker inspect` before asking a question. It never touches postgres, the worker, nats or LocalAI.
# Needs about 90 minutes per arm for the 80-question spread (--per-intent 1). Re-run the same command to continue (--resume).
$ErrorActionPreference = "Continue"
$script:Here = Split-Path -Parent $MyInvocation.MyCommand.Path
$script:Repo = (Resolve-Path (Join-Path $script:Here "..\..")).Path
$script:StateFile = Join-Path $env:TEMP "nexusai-lane-arms-state.json"

function Get-Stack {
  $names = @(docker ps --format "{{.Names}}")
  $api = $names | Where-Object { $_ -match 'forensic.*api' -and $_ -notmatch 'worker' } | Select-Object -First 1
  $pg = $names | Where-Object { $_ -match 'postgres' } | Select-Object -First 1
  if (-not $api -or -not $pg) { throw "The stack is not running (found api=[$api] postgres=[$pg]). Start it first." }
  $info = @(docker inspect $api | ConvertFrom-Json)[0]
  $labels = $info.Config.Labels
  $envFile = $labels.'com.docker.compose.project.environment_file'
  if ($envFile) { $envFile = ($envFile -split ',')[0] }
  if (-not $envFile) { $candidate = Join-Path $script:Repo ".env.forensic-runtime.local"; if (Test-Path $candidate) { $envFile = $candidate } }
  $workDir = $labels.'com.docker.compose.project.working_dir'
  if (-not $workDir) { $workDir = $script:Repo }
  [pscustomobject]@{
    Api = $api; Postgres = $pg; Project = $labels.'com.docker.compose.project'; Service = $labels.'com.docker.compose.service'
    Files = @($labels.'com.docker.compose.project.config_files' -split ','); EnvFile = $envFile; WorkDir = $workDir
    ImageName = $info.Config.Image; ImageId = $info.Image; Env = @($info.Config.Env)
  }
}

function Get-ContainerSwitch($Stack, $Name) {
  $info = @(docker inspect $Stack.Api | ConvertFrom-Json)[0]
  $line = @($info.Config.Env) | Where-Object { $_ -like "$Name=*" } | Select-Object -First 1
  if ($line) { return $line.Substring($Name.Length + 1) } else { return "<unset>" }
}

function Show-Stack {
  $s = Get-Stack
  "api container:   $($s.Api)"; "postgres:        $($s.Postgres)"; "compose project: $($s.Project)  service: $($s.Service)"
  "compose files:   $($s.Files -join '; ')"; "env file:        $($s.EnvFile)"; "image name:      $($s.ImageName)"; "image id:        $($s.ImageId)"
  foreach ($n in 'FORENSIC_GOVERNED_SQL', 'FORENSIC_GOVERNED_SQL_FIRST', 'FORENSIC_CONVERSATION_FRONT_DOOR') { "{0,-34} {1}" -f $n, (Get-ContainerSwitch $s $n) }
}

function Invoke-Compose($Stack, [string[]]$ComposeArgs) {
  $a = @('compose', '-p', $Stack.Project)
  foreach ($f in $Stack.Files) { $a += @('-f', $f) }
  if ($Stack.EnvFile) { $a += @('--env-file', $Stack.EnvFile) }
  $a += $ComposeArgs
  Write-Host ("docker " + ($a -join ' ')) -ForegroundColor DarkGray
  Push-Location $Stack.WorkDir
  try { & docker @a | Out-Host } finally { Pop-Location }
  return [int]$LASTEXITCODE
}

# Every FORENSIC_/NEXUSAI_ variable the running API already has goes back into the process environment, so compose recreates the
# container with EXACTLY the configuration it had; then the caller overrides the two lane switches.
function Import-ContainerEnv($Stack) {
  $saved = @{}
  foreach ($line in $Stack.Env) {
    $i = $line.IndexOf('=')
    if ($i -lt 1) { continue }
    $name = $line.Substring(0, $i)
    if ($name -match '^(FORENSIC|NEXUSAI)_') {
      $saved[$name] = [Environment]::GetEnvironmentVariable($name, 'Process')
      [Environment]::SetEnvironmentVariable($name, $line.Substring($i + 1), 'Process')
    }
  }
  return $saved
}

function Restore-ProcessEnv($Saved) { foreach ($k in $Saved.Keys) { [Environment]::SetEnvironmentVariable($k, $Saved[$k], 'Process') } }

function Wait-Healthy {
  for ($i = 0; $i -lt 60; $i++) {
    try { if ((Invoke-WebRequest -UseBasicParsing -TimeoutSec 3 "http://127.0.0.1:8091/healthz").StatusCode -eq 200) { return $true } } catch {}
    Start-Sleep -Seconds 3
  }
  return $false
}

function Build-LaneImage {
  $s = Get-Stack
  if (-not (Test-Path $script:StateFile)) {
    # keep the old image reachable under a second name, in case the build re-tags the first
    docker tag $s.ImageId "nexusai-lane-arms-original:before" | Out-Null
    @{ ImageName = $s.ImageName; ImageId = $s.ImageId; GovernedSql = (Get-ContainerSwitch $s 'FORENSIC_GOVERNED_SQL'); GovernedSqlFirst = (Get-ContainerSwitch $s 'FORENSIC_GOVERNED_SQL_FIRST') } |
      ConvertTo-Json | Set-Content -Encoding ascii $script:StateFile
    Write-Host "saved the original image and switch state to $script:StateFile"
  }
  $rc = Invoke-Compose $s @('build', $s.Service)
  if ($rc -ne 0) { Write-Host "BUILD FAILED (exit $rc): nothing was changed; the running API is untouched" -ForegroundColor Red; return }
  Write-Host "built. Next: Run-Arm A" -ForegroundColor Green
}

function Set-Arm {
  param([ValidateSet('A', 'B', 'C')][string]$Arm)
  $s = Get-Stack
  $want = @{ 'A' = @('false', 'false'); 'B' = @('true', 'true'); 'C' = @('true', 'false') }[$Arm]
  $saved = Import-ContainerEnv $s
  try {
    $env:FORENSIC_GOVERNED_SQL = $want[0]; $env:FORENSIC_GOVERNED_SQL_FIRST = $want[1]
    $rc = Invoke-Compose $s @('up', '-d', '--no-deps', '--force-recreate', $s.Service)
  } finally { Restore-ProcessEnv $saved; Remove-Item Env:\FORENSIC_GOVERNED_SQL, Env:\FORENSIC_GOVERNED_SQL_FIRST -ErrorAction SilentlyContinue }
  if ($rc -ne 0) { throw "compose up failed (exit $rc)" }
  if (-not (Wait-Healthy)) { throw "the API did not become healthy within 3 minutes" }
  $s = Get-Stack
  $got = @((Get-ContainerSwitch $s 'FORENSIC_GOVERNED_SQL'), (Get-ContainerSwitch $s 'FORENSIC_GOVERNED_SQL_FIRST'))
  Write-Host ("arm {0}: container reports FORENSIC_GOVERNED_SQL={1} FORENSIC_GOVERNED_SQL_FIRST={2}" -f $Arm, $got[0], $got[1])
  if ($got[0] -ne $want[0] -or $got[1] -ne $want[1]) { throw "the container's switch state is not what arm $Arm needs; stop and send this output" }
  return $s
}

function Run-Arm {
  param([ValidateSet('A', 'B', 'C')][string]$Arm, [int]$BudgetMinutes = 110)
  $s = Set-Arm -Arm $Arm
  Add-Type -Namespace W -Name P -MemberDefinition '[System.Runtime.InteropServices.DllImport("kernel32.dll")] public static extern uint SetThreadExecutionState(uint f);' -ErrorAction SilentlyContinue
  [void][W.P]::SetThreadExecutionState([uint32]2147483649)
  $keyLine = $s.Env | Where-Object { $_ -like 'FORENSIC_RECORDS_API_KEY=*' } | Select-Object -First 1
  if (-not $keyLine) { throw "the API container has no FORENSIC_RECORDS_API_KEY" }
  $env:FORENSIC_RECORDS_API_KEY = $keyLine.Substring('FORENSIC_RECORDS_API_KEY='.Length)
  $psql = "docker exec -i $($s.Postgres) psql -U localrecall -d localrecall -At -F '|'"
  Push-Location $script:Here
  try {
    if (-not (Test-Path questions-demo.json)) { throw "questions-demo.json is missing: use the file the baseline used, do not regenerate it" }
    python factory.py run --questions questions-demo.json --arm "arm-lane-$Arm" --per-intent 1 --budget-minutes $BudgetMinutes --resume --psql $psql
  } finally { Pop-Location; Remove-Item Env:\FORENSIC_RECORDS_API_KEY -ErrorAction SilentlyContinue }
  Write-Host "If it stopped at the time budget, run the same command again to continue." -ForegroundColor Yellow
}

function Compare-Arms {
  Push-Location $script:Here
  try {
    foreach ($pair in @(@('arm-baseline-v1', 'arm-lane-A'), @('arm-lane-A', 'arm-lane-C'), @('arm-lane-A', 'arm-lane-B'))) {
      if ((Test-Path (Join-Path $pair[0] 'results.json')) -and (Test-Path (Join-Path $pair[1] 'results.json'))) { python factory.py compare $pair[0] $pair[1]; "" } else { "skipped $($pair -join ' vs ') (results missing)" }
    }
  } finally { Pop-Location }
}

# Gate G2: the 103-question corpus and the 38-question pre-flight, with the lane off (A) and fallback (C). Read the DIFFERENCES that
# compare prints: every question the existing path answered must have identical text.
function Run-Regression {
  param([ValidateSet('A', 'C')][string]$Arm)
  $s = Set-Arm -Arm $Arm
  $keyLine = $s.Env | Where-Object { $_ -like 'FORENSIC_RECORDS_API_KEY=*' } | Select-Object -First 1
  $env:FORENSIC_RECORDS_API_KEY = $keyLine.Substring('FORENSIC_RECORDS_API_KEY='.Length)
  Push-Location (Join-Path $script:Repo "reports\free-question-baseline-20261002")
  try {
    python replay_corpus.py run "replay-lane-$Arm"
    $out = Join-Path (Get-Location) "preflight-lane-$Arm"; New-Item -ItemType Directory -Force $out | Out-Null
    $env:NEXUSAI_OUT = $out; python preflight.py; Remove-Item Env:\NEXUSAI_OUT -ErrorAction SilentlyContinue
  } finally { Pop-Location; Remove-Item Env:\FORENSIC_RECORDS_API_KEY -ErrorAction SilentlyContinue }
}

function Restore-Stack {
  if (-not (Test-Path $script:StateFile)) { Write-Host "no saved state: nothing to restore"; return }
  $state = Get-Content $script:StateFile -Raw | ConvertFrom-Json
  $s = Get-Stack
  docker tag $state.ImageId $state.ImageName | Out-Null
  $saved = Import-ContainerEnv $s
  try {
    foreach ($pair in @(@('FORENSIC_GOVERNED_SQL', $state.GovernedSql), @('FORENSIC_GOVERNED_SQL_FIRST', $state.GovernedSqlFirst))) {
      if ($pair[1] -eq '<unset>') { Remove-Item "Env:\$($pair[0])" -ErrorAction SilentlyContinue } else { [Environment]::SetEnvironmentVariable($pair[0], $pair[1], 'Process') }
    }
    $rc = Invoke-Compose $s @('up', '-d', '--no-deps', '--force-recreate', $s.Service)
  } finally { Restore-ProcessEnv $saved }
  if ($rc -eq 0) { Wait-Healthy | Out-Null; Show-Stack } else { Write-Host "restore failed (exit $rc)" -ForegroundColor Red }
}
