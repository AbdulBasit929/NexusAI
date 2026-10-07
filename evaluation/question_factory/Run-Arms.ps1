# GOVERNED SQL LANE: THE THREE ARMS (the owner runs this; gates are in reports/governed-sql-20261005/PREREGISTRATION.md)
#
# Run it from a CLEAN checkout of the branch (not from your main working copy, so nothing of yours is touched):
#
#   cd C:\Users\sheik\Workspace\Office\Projects\NexusAI-localwork
#   git fetch origin claude/gifted-pasteur-4kpx0r
#   git checkout -B lane-arms origin/claude/gifted-pasteur-4kpx0r
#   . .\evaluation\question_factory\Run-Arms.ps1
#   Show-Stack          # read-only: what is running, which switches it has
#   Build-LaneImage     # docker build of THIS checkout, tagged with the name the running API already uses; the old image is kept
#   Run-Arm A           # new image, both switches off  (the control: must equal arm-baseline-v1)
#   Run-Arm C           # lane only after the existing path declines
#   Run-Arm B           # lane before the existing path
#   Compare-Arms        # A vs baseline, A vs C, A vs B
#   Run-Regression A ; Run-Regression C     # gates G2 and G3: the 103-question corpus and the pre-flight
#   Restore-Stack       # puts the original image and switches back
#
# questions-demo.json and arm-baseline-v1 are copied from your main checkout (..\NexusAI\evaluation\question_factory) the first time.
# Each Run-Arm recreates ONLY the API container, with the environment it already had plus the two lane switches, and checks the switch
# state from `docker inspect` before asking a question. It never touches postgres, the worker, nats or LocalAI.
# About 90 minutes per arm for the 80-question spread (--per-intent 1). Run the same command again to continue (--resume).
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
  # The running stack was started from the main checkout, whose compose file does not declare the lane switches. The same file
  # of THIS checkout does (and differs from it only by those declarations), so use it when it exists. The API service has only
  # named volumes, so nothing depends on the directory.
  $workDir = $Stack.WorkDir
  foreach ($f in $Stack.Files) {
    $mine = Join-Path $script:Repo (Split-Path $f -Leaf)
    if (Test-Path -LiteralPath $mine) { $f = $mine; $workDir = $script:Repo }
    $a += @('-f', $f)
  }
  if ($Stack.EnvFile) { $a += @('--env-file', $Stack.EnvFile) }
  $a += $ComposeArgs
  Write-Host ("docker " + ($a -join ' ')) -ForegroundColor DarkGray
  Push-Location $workDir
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

# The compose file reads some settings from the SHELL that runs compose (for example FORENSIC_API_KEY comes from
# ${FORENSIC_RECORDS_API_KEY}), not from the container. A new window does not have them, and compose then recreates the API
# with an empty key, which the API refuses to start with. So: copy each from the running container where it is non-empty,
# else ask for it (hidden input, never printed, never written to disk). Enter on LOCALAI_API_KEY skips it.
$script:AskedSecrets = @{}
function Ensure-ComposeSecrets($Stack) {
  $map = [ordered]@{ 'FORENSIC_RECORDS_API_KEY' = 'FORENSIC_API_KEY'; 'FORENSIC_PII_ALIAS_SECRET' = 'FORENSIC_PII_ALIAS_SECRET'; 'LOCALAI_API_KEY' = $null }
  # The stack's own secrets file (gitignored, in the main checkout) is the first place to look.
  $fileValues = @{}
  foreach ($dir in @((Join-Path (Split-Path $script:Repo -Parent) 'NexusAI'), $script:Repo)) {
    $file = Join-Path $dir '.env.forensic-runtime.local'
    if (Test-Path -LiteralPath $file) {
      foreach ($l in (Get-Content -LiteralPath $file)) {
        if ($l -match '^\s*([A-Z][A-Z0-9_]*)=(.*)$' -and -not $fileValues.ContainsKey($Matches[1])) { $fileValues[$Matches[1]] = $Matches[2].Trim() }
      }
    }
  }
  foreach ($host_var in $map.Keys) {
    if ([Environment]::GetEnvironmentVariable($host_var, 'Process')) { continue }
    $value = ''
    if ($fileValues.ContainsKey($host_var)) { $value = $fileValues[$host_var] }
    $inside = $map[$host_var]
    if (-not $value -and $inside) {
      $line = @($Stack.Env) | Where-Object { $_ -like "$inside=*" } | Select-Object -First 1
      if ($line) { $value = $line.Substring($inside.Length + 1) }
    }
    if (-not $value -and -not $script:AskedSecrets[$host_var]) {
      $script:AskedSecrets[$host_var] = $true
      $secure = Read-Host -AsSecureString "$host_var is not set in this window; paste it (hidden), or press Enter to leave it empty"
      $ptr = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($secure)
      try { $value = [Runtime.InteropServices.Marshal]::PtrToStringBSTR($ptr) } finally { [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($ptr) }
    }
    if ($value) { [Environment]::SetEnvironmentVariable($host_var, $value, 'Process') }
  }
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
  Write-Host "building $($s.ImageName) from $script:Repo (the running API is not touched until Run-Arm)" -ForegroundColor DarkGray
  Push-Location $script:Repo
  try { docker build -t $s.ImageName -f api/forensic_records/Dockerfile . | Out-Host; $rc = [int]$LASTEXITCODE } finally { Pop-Location }
  if ($rc -ne 0) { Write-Host "BUILD FAILED (exit $rc): nothing was changed; the running API is untouched" -ForegroundColor Red; return }
  Write-Host "built. Next: Run-Arm A" -ForegroundColor Green
}

# The baseline's question file and results live in the main checkout; the arms must use the SAME questions.
function Import-BaselineData {
  param([string]$From = (Join-Path (Split-Path $script:Repo -Parent) "NexusAI\evaluation\question_factory"))
  foreach ($name in 'questions-demo.json', 'arm-baseline-v1') {
    $src = Join-Path $From $name; $dst = Join-Path $script:Here $name
    if ((Test-Path $src) -and -not (Test-Path $dst)) { Copy-Item $src $dst -Recurse; Write-Host "copied $name from $From" }
  }
}

function Set-Arm {
  param([ValidateSet('A', 'B', 'C')][string]$Arm)
  $s = Get-Stack
  $want = @{ 'A' = @('false', 'false'); 'B' = @('true', 'true'); 'C' = @('true', 'false') }[$Arm]
  $saved = Import-ContainerEnv $s
  Ensure-ComposeSecrets $s
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
  param([ValidateSet('A', 'B', 'C')][string]$Arm, [int]$BudgetMinutes = 110, [string]$Questions = 'questions-demo.json', [string]$Tag = '')
  Import-BaselineData
  $s = Set-Arm -Arm $Arm
  Add-Type -Namespace W -Name P -MemberDefinition '[System.Runtime.InteropServices.DllImport("kernel32.dll")] public static extern uint SetThreadExecutionState(uint f);' -ErrorAction SilentlyContinue
  [void][W.P]::SetThreadExecutionState([uint32]2147483649)
  $keyLine = $s.Env | Where-Object { $_ -like 'FORENSIC_RECORDS_API_KEY=*' -or $_ -like 'FORENSIC_API_KEY=*' } | Select-Object -First 1
  if (-not $keyLine) { throw "the API container has no FORENSIC_API_KEY" }
  $env:FORENSIC_RECORDS_API_KEY = $keyLine.Substring($keyLine.IndexOf('=') + 1)
  $psql = "docker exec -i $($s.Postgres) psql -U localrecall -d localrecall -At -F '|'"
  Push-Location $script:Here
  try {
    if (-not (Test-Path $Questions)) { throw "$Questions is missing: use the file the baseline used (or New-QuestionSet for a new one), do not regenerate it" }
    python factory.py run --questions $Questions --arm "arm-lane-$Arm$Tag" --per-intent 1 --budget-minutes $BudgetMinutes --resume --psql $psql
  } finally { Pop-Location; Remove-Item Env:\FORENSIC_RECORDS_API_KEY -ErrorAction SilentlyContinue }
  Write-Host "If it stopped at the time budget, run the same command again to continue." -ForegroundColor Yellow
}

# Ask ONE question again against the running API and print the answer key, the answer, the lane header and the query the lane ran.
# A NEW question set with a different seed (read-only against the case database). Questions never seen while the lane was built.
function New-QuestionSet {
  param([int]$Seed = 2, [string]$Name = 'questions-demo-v2.json')
  $s = Get-Stack
  $psql = "docker exec -i $($s.Postgres) psql -U localrecall -d localrecall -At -F '|'"
  Push-Location $script:Here
  try { python factory.py generate --psql $psql --collection nexusai-forensic-demo --seed $Seed --out $Name } finally { Pop-Location }
}

# Everything the activation decision still needs, in one go, saved to files next to the factory: arm A on the unseen set, the two corpus
# comparisons, and the queries behind the remaining wrong answers. Takes about an hour; start it and leave the window alone.
# The confirmation round after a fix: the unseen set on arm B (kept as -v3), the old corpus and the pre-flight on C and B, and the
# comparisons against A saved next to the factory. Build-LaneImage first. Takes about two hours; start it and leave the window alone.
# STAGE 1 of the activation plan: the lane runs only AFTER the existing path withholds, so it cannot change an answer already given.
# This recreates only the API container with FORENSIC_GOVERNED_SQL=true (and _FIRST=false), asserts it, and leaves it running.
# To keep it after a full restart of the stack, set FORENSIC_GOVERNED_SQL=true in the environment that starts the stack.
function Enable-LaneStage1 { [void](Set-Arm -Arm C); Show-Stack }
# Back to the lane off, exactly as shipped (the same recreate with both switches false). Use this to roll Stage 1 back.
function Disable-Lane { [void](Set-Arm -Arm A); Show-Stack }

# Ask the running API one or more questions the way the UI does, and show the answer, which path produced it (the X-Governed-SQL header)
# and the query the lane ran. Read-only. Example:  Ask-Case "How many calls did 923001110001 make?"
function Ask-Case {
  param([Parameter(Mandatory, ValueFromRemainingArguments)][string[]]$Question, [string]$Collection = 'nexusai-forensic-demo')
  $s = Get-Stack
  $keyLine = $s.Env | Where-Object { $_ -like 'FORENSIC_RECORDS_API_KEY=*' -or $_ -like 'FORENSIC_API_KEY=*' } | Select-Object -First 1
  $key = $keyLine.Substring($keyLine.IndexOf('=') + 1)
  $headers = @{ 'Authorization' = "Bearer $key"; 'X-Forensic-Tenant-ID' = 'default'; 'X-Forensic-Collection-ID' = $Collection; 'X-Forensic-Actor-ID' = 'investigation-workspace'; 'X-Forensic-Subject-ID' = 'investigation-workspace'; 'X-Forensic-Actor-Role' = 'user' }
  foreach ($q in $Question) {
    $body = @{ tenant_id = 'default'; collection_id = $Collection; query = $q } | ConvertTo-Json
    $started = Get-Date
    try {
      $r = Invoke-WebRequest -UseBasicParsing -Uri 'http://127.0.0.1:8091/query/hybrid' -Method Post -ContentType 'application/json' -Headers $headers -Body $body -TimeoutSec 900
    } catch { Write-Host "FAILED: $($_.Exception.Message)" -ForegroundColor Red; continue }
    $blob = $r.Content | ConvertFrom-Json
    $ent = $blob.enterprise
    Write-Host ""
    Write-Host "Q: $q" -ForegroundColor Cyan
    Write-Host ("path    : " + ($blob.route -join ', ') + "   (" + [math]::Round(((Get-Date) - $started).TotalSeconds, 1) + " s)")
    Write-Host ("lane    : " + $r.Headers['X-Governed-SQL'])
    Write-Host ("answer  : " + $ent.executive_answer)
    if ($ent.derivation -and $ent.derivation.sql) { Write-Host ("query   : " + $ent.derivation.sql) }
    if ($ent.limitations) { foreach ($l in $ent.limitations) { Write-Host ("note    : " + $l) -ForegroundColor DarkGray } }
  }
}

function Verify-Round {
  Run-Arm -Arm B -Questions questions-demo-v2.json -Tag -v3
  Run-Regression C
  Run-Regression B
  $out = $script:Here
  Push-Location (Join-Path $script:Repo "reports\free-question-baseline-20261002")
  try {
    python replay_corpus.py compare replay-lane-A replay-lane-C | Out-File -Encoding utf8 (Join-Path $out 'verify-compare-A-C.txt')
    python replay_corpus.py compare replay-lane-A replay-lane-B | Out-File -Encoding utf8 (Join-Path $out 'verify-compare-A-B.txt')
  } finally { Pop-Location }
  Push-Location $out
  try { python factory.py compare arm-lane-A-v2 arm-lane-B-v3 | Out-File -Encoding utf8 (Join-Path $out 'verify-compare-v2-A-B.txt') } finally { Pop-Location }
  Write-Host "done. Send me the three summaries above, then run:  Select-String -Path .\evaluation\question_factory\verify-compare-A-C.txt -Pattern '^--- |^  ON ' | ForEach-Object { `$_.Line.Substring(0, [Math]::Min(240, `$_.Line.Length)) }   and   Select-String -Path .\evaluation\question_factory\verify-compare-v2-A-B.txt -Pattern 'WORSE' -Context 0,4" -ForegroundColor Green
}

function Finish-Round {
  param([string[]]$Explain = @('ACCE-top_group-01', 'ACCE-count_distinct-01', 'CDR-avg-01', 'CDR-max-01'))
  Run-Arm -Arm A -Questions questions-demo-v2.json -Tag -v2
  $out = $script:Here
  Push-Location (Join-Path $script:Repo "reports\free-question-baseline-20261002")
  try {
    python replay_corpus.py compare replay-lane-A replay-lane-C | Out-File -Encoding utf8 (Join-Path $out 'round-compare-A-C.txt')
    python replay_corpus.py compare replay-lane-A replay-lane-B | Out-File -Encoding utf8 (Join-Path $out 'round-compare-A-B.txt')
  } finally { Pop-Location }
  Push-Location $out
  try { python factory.py compare arm-lane-A-v2 arm-lane-B-v2 | Out-File -Encoding utf8 (Join-Path $out 'round-compare-v2-A-B.txt') } finally { Pop-Location }
  [void](Set-Arm -Arm B)
  Explain-Question -Id $Explain -Questions questions-demo-v2.json | Out-File -Encoding utf8 (Join-Path $out 'round-explain.txt')
  Write-Host "done. Send me: the summary printed above, round-compare-v2-A-B.txt, round-explain.txt, round-compare-A-C.txt and round-compare-A-B.txt (all in $out)" -ForegroundColor Green
}

function Explain-Question {
  param([Parameter(Mandatory)][string[]]$Id, [string]$Questions = 'questions-demo.json')
  $s = Get-Stack
  $keyLine = $s.Env | Where-Object { $_ -like 'FORENSIC_RECORDS_API_KEY=*' -or $_ -like 'FORENSIC_API_KEY=*' } | Select-Object -First 1
  $env:FORENSIC_RECORDS_API_KEY = $keyLine.Substring($keyLine.IndexOf('=') + 1)
  Push-Location $script:Here
  try { foreach ($one in $Id) { python factory.py explain --questions $Questions --id $one } } finally { Pop-Location; Remove-Item Env:\FORENSIC_RECORDS_API_KEY -ErrorAction SilentlyContinue }
}

# Re-read saved results with the current number matcher (a correctly rounded rendition counts). Writes results-rejudged.json beside each.
function Rejudge-Arms {
  param([string[]]$Arm = @('arm-baseline-v1', 'arm-lane-A', 'arm-lane-C-run1', 'arm-lane-C', 'arm-lane-B'))
  Push-Location $script:Here
  try { $have = @($Arm | Where-Object { Test-Path (Join-Path $_ 'results.json') }); python factory.py rejudge @have } finally { Pop-Location }
}

function Compare-Arms {
  Import-BaselineData
  Push-Location $script:Here
  try {
    foreach ($pair in @(@('arm-baseline-v1', 'arm-lane-A'), @('arm-lane-A', 'arm-lane-C'), @('arm-lane-A', 'arm-lane-B'))) {
      if ((Test-Path (Join-Path $pair[0] 'results.json')) -and (Test-Path (Join-Path $pair[1] 'results.json'))) { python factory.py compare $pair[0] $pair[1]; "" } else { "skipped $($pair -join ' vs ') (results missing)" }
    }
  } finally { Pop-Location }
}

# Gate G2: the 103-question corpus and the 38-question pre-flight, with the lane off (A), after the old path (C) and first (B). Read the DIFFERENCES that
# compare prints: every question the existing path answered must have identical text.
function Run-Regression {
  param([ValidateSet('A', 'B', 'C')][string]$Arm)
  $s = Set-Arm -Arm $Arm
  $keyLine = $s.Env | Where-Object { $_ -like 'FORENSIC_RECORDS_API_KEY=*' -or $_ -like 'FORENSIC_API_KEY=*' } | Select-Object -First 1
  $env:FORENSIC_RECORDS_API_KEY = $keyLine.Substring($keyLine.IndexOf('=') + 1)
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
  Ensure-ComposeSecrets $s
  try {
    foreach ($pair in @(@('FORENSIC_GOVERNED_SQL', $state.GovernedSql), @('FORENSIC_GOVERNED_SQL_FIRST', $state.GovernedSqlFirst))) {
      if ($pair[1] -eq '<unset>') { Remove-Item "Env:\$($pair[0])" -ErrorAction SilentlyContinue } else { [Environment]::SetEnvironmentVariable($pair[0], $pair[1], 'Process') }
    }
    $rc = Invoke-Compose $s @('up', '-d', '--no-deps', '--force-recreate', $s.Service)
  } finally { Restore-ProcessEnv $saved }
  if ($rc -eq 0) { Wait-Healthy | Out-Null; Show-Stack } else { Write-Host "restore failed (exit $rc)" -ForegroundColor Red }
}
