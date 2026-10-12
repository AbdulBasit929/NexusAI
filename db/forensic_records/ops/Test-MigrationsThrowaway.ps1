<#
.SYNOPSIS
  Proves migrations 012 to 015 on a THROWAWAY PostgreSQL container. It never touches the NexusAI stack.

.DESCRIPTION
  Starts a disposable container from an image that is already on this machine (it never pulls), applies migrations 001 to 011
  and the runtime-role grants as a fresh database would see them, then runs for 012 to 015: preflight, apply, verify, smoke,
  a second apply (idempotency), the runtime-role grants again, a gated rollback in reverse order (including a check that the
  gate refuses without its setting), re-application from the rolled-back state, and a final comparison that nothing created
  by migrations 001 to 011 changed. The container and its data are removed at the end.

  The container has no published port and a random password that is never printed.

.EXAMPLE
  powershell -ExecutionPolicy Bypass -File .\db\forensic_records\ops\Test-MigrationsThrowaway.ps1
#>
[CmdletBinding()]
param(
  [string]$Image = 'timescale/timescaledb:latest-pg16',
  [int]$MemoryMB = 1536,
  [switch]$KeepContainer,
  # A copy of db/forensic_records to test instead of the checked-in one (used to prove the checks fail when a rule is broken).
  [string]$MigrationDir = '',
  # Also load this many synthetic call events (and half as many internet sessions) and print load time, sizes and query timings.
  [int]$VolumeRows = 0
)

$ErrorActionPreference = 'Stop'
$RepoRoot = Split-Path -Parent (Split-Path -Parent (Split-Path -Parent $PSScriptRoot))
. (Join-Path $RepoRoot 'scripts\forensic_runtime_common.ps1')

if (-not $MigrationDir) { $MigrationDir = Join-Path $RepoRoot 'db\forensic_records' }
if (-not (Test-Path -LiteralPath (Join-Path $MigrationDir '012_typed_families.sql'))) { throw "No migrations found in $MigrationDir" }
$Container = 'nx-migtest-' + ([guid]::NewGuid().ToString('N').Substring(0, 8))
$Password = [guid]::NewGuid().ToString('N')
$Results = New-Object System.Collections.Generic.List[object]
$Remote = '/tmp/mig'

$BaseMigrations = @(
  '001_phase1_records.sql', '002_job_row_counters.sql', '003_phase2_kb_assets_and_entities.sql', '004_dynamic_records.sql',
  '005_backfill_canonical_records.sql', '006_canonical_normalized_fields.sql', '007_evidence_items.sql',
  '008_phase3_evidence_control_plane.sql', '009_phase3_queue_lifecycle.sql', '010_runtime_rls_policies.sql',
  '011_r5_custody_chain_tail.sql'
)
$Typed = @(
  @{ Id = '012'; Name = '012_typed_families'; Gate = 'app.typed_families_allow_destructive_rollback'; Tables = @('comm_event', 'data_session', 'app_usage', 'plate_sighting', 'subscriber_registration', 'subscriber_identity', 'layout_versions') },
  @{ Id = '013'; Name = '013_reference_tables'; Gate = 'app.reference_tables_allow_destructive_rollback'; Tables = @('cell_site', 'camera_site', 'device_tac', 'ip_range_reference') },
  @{ Id = '014'; Name = '014_entity_relationship_layer'; Gate = 'app.entity_layer_allow_destructive_rollback'; Tables = @('derivation', 'entity', 'entity_mention', 'relationship', 'relationship_evidence', 'ip_assignment') },
  @{ Id = '015'; Name = '015_vectors_and_earth'; Gate = 'app.vectors_allow_destructive_rollback'; Tables = @('content_embedding') }
)

function Invoke-Psql {
  param(
    [Parameter(Mandatory = $true)][string]$RemoteFile,
    [string[]]$Before = @(),
    [string[]]$Variables = @(),
    [switch]$AllowFailure
  )
  $psqlArgs = @('-v', 'ON_ERROR_STOP=1', '-U', 'localrecall', '-d', 'localrecall')
  foreach ($variable in $Variables) { $psqlArgs += @('-v', $variable) }
  foreach ($statement in $Before) { $psqlArgs += @('-c', $statement) }
  $psqlArgs += @('-f', "$Remote/$RemoteFile")
  $lines = @()
  $exit = 0
  $saved = $ErrorActionPreference
  try {
    $ErrorActionPreference = 'Continue'
    $lines = @(& docker exec $Container psql @psqlArgs 2>&1 | ForEach-Object {
        if ($_ -is [System.Management.Automation.ErrorRecord]) { $_.Exception.Message } else { "$_" }
      })
    $exit = $LASTEXITCODE
  } finally {
    $ErrorActionPreference = $saved
  }
  if ($exit -ne 0 -and -not $AllowFailure) {
    $lines | Select-Object -Last 25 | ForEach-Object { Write-Host "  psql> $_" }
    throw "$RemoteFile failed (psql exit $exit)"
  }
  return [pscustomobject]@{ Exit = $exit; Output = ($lines -join "`n") }
}

function Invoke-Scalar {
  param([Parameter(Mandatory = $true)][string]$Sql)
  $value = & docker exec $Container psql -v ON_ERROR_STOP=1 -U localrecall -d localrecall -tA -c $Sql
  if ($LASTEXITCODE -ne 0) { throw "query failed: $Sql" }
  return (($value | Out-String).Trim())
}

function Step {
  param([string]$Label, [scriptblock]$Body)
  $watch = [System.Diagnostics.Stopwatch]::StartNew()
  & $Body
  $watch.Stop()
  $Results.Add([pscustomobject]@{ Step = $Label; Seconds = [math]::Round($watch.Elapsed.TotalSeconds, 1); Result = 'ok' })
  Write-Host ("  ok  {0,-62} {1,6:N1}s" -f $Label, $watch.Elapsed.TotalSeconds)
}

function Assert-Contains {
  param([string]$Text, [string]$Expected, [string]$Label)
  if ($Text -notmatch [regex]::Escape($Expected)) { throw "$Label did not print '$Expected'" }
}

function Get-CatalogDigest {
  # A digest of everything migrations 001 to 011 created in schema forensic (tables, columns, constraints, policies, functions),
  # excluding the objects migrations 012 to 015 add.
  $newTables = "'layout_versions','comm_event','data_session','app_usage','plate_sighting','subscriber_identity','subscriber_registration','cell_site','camera_site','device_tac','ip_range_reference','derivation','entity','entity_mention','relationship','relationship_evidence','ip_assignment','content_embedding'"
  $sql = @"
SELECT md5(string_agg(x, E'\n' ORDER BY x)) FROM (
  SELECT 'col ' || c.relname || '.' || a.attname || ' ' || format_type(a.atttypid, a.atttypmod) || ' ' || a.attnotnull AS x
    FROM pg_attribute a JOIN pg_class c ON c.oid = a.attrelid
    WHERE c.relnamespace = 'forensic'::regnamespace AND c.relkind IN ('r','p') AND a.attnum > 0 AND NOT a.attisdropped
      AND c.relname NOT IN ($newTables)
  UNION ALL
  SELECT 'con ' || c.relname || '.' || k.conname || ' ' || pg_get_constraintdef(k.oid)
    FROM pg_constraint k JOIN pg_class c ON c.oid = k.conrelid
    WHERE c.relnamespace = 'forensic'::regnamespace AND c.relname NOT IN ($newTables)
      AND NOT (pg_get_constraintdef(k.oid) ~ 'REFERENCES forensic\.(layout_versions|comm_event|data_session|app_usage|plate_sighting|subscriber_identity|subscriber_registration|cell_site|camera_site|derivation|entity|relationship)')
  UNION ALL
  SELECT 'pol ' || tablename || '.' || policyname || ' ' || coalesce(qual, '') || ' ' || coalesce(with_check, '')
    FROM pg_policies WHERE schemaname = 'forensic' AND tablename NOT IN ($newTables)
  UNION ALL
  SELECT 'fn ' || p.proname || ' ' || md5(pg_get_functiondef(p.oid))
    FROM pg_proc p WHERE p.pronamespace = 'forensic'::regnamespace
      AND p.proname NOT IN ('normalize_plate','make_cell_key','norm_cell_part')
) t
"@
  return (Invoke-Scalar $sql)
}

Write-Host "Throwaway migration test: image $Image, container $Container"
$started = $false
try {
  Step 'image is present locally (never pulled)' { Assert-ForensicImage $Image }

  Step 'start disposable container' {
    & docker run -d --rm --name $Container --label nexusai.purpose=migration-test --memory "${MemoryMB}m" `
      -e POSTGRES_USER=localrecall -e POSTGRES_DB=localrecall -e "POSTGRES_PASSWORD=$Password" $Image | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'docker run failed' }
    $script:started = $true
    $deadline = (Get-Date).AddSeconds(180)
    $ready = $false
    do {
      Start-Sleep -Seconds 2
      $saved = $ErrorActionPreference
      try {
        $ErrorActionPreference = 'Continue'
        $log = (@(& docker logs $Container 2>&1 | ForEach-Object { "$_" }) -join "`n")
      } finally {
        $ErrorActionPreference = $saved
      }
      if ($log -match 'init process complete; ready for start up') {
        & docker exec $Container pg_isready -h 127.0.0.1 -U localrecall -d localrecall | Out-Null
        if ($LASTEXITCODE -eq 0) { $ready = $true }
      }
    } until ($ready -or (Get-Date) -gt $deadline)
    if (-not $ready) { throw 'container did not become ready in 180 seconds' }
  }

  Step 'copy migration files into the container' {
    & docker cp $MigrationDir "${Container}:$Remote" | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'docker cp failed' }
  }

  Step 'server facts (version, encoding, extensions available)' {
    $facts = Invoke-Scalar "SELECT version() || ' | ' || current_setting('server_encoding') || ' | vector=' || coalesce((SELECT default_version FROM pg_available_extensions WHERE name='vector'),'none') || ' | postgis=' || coalesce((SELECT default_version FROM pg_available_extensions WHERE name='postgis'),'none')"
    Write-Host "      $facts"
  }

  foreach ($file in $BaseMigrations) {
    Step "base migration $file" { [void](Invoke-Psql $file) }
  }
  Step 'runtime role grants (before 012 to 015)' { [void](Invoke-Psql 'runtime_role.grants.sql') }
  Step 'existing 010 verify passes before the new migrations' { [void](Invoke-Psql '010_runtime_rls_policies.verify.sql') }

  $digestBefore = Get-CatalogDigest
  $legacyRows = Invoke-Scalar "SELECT (SELECT count(*) FROM forensic.records) || ',' || (SELECT count(*) FROM forensic.cdr_records) || ',' || (SELECT count(*) FROM forensic.evidence_items)"

  # Forward pass.
  foreach ($m in $Typed) {
    $dir = "ops/$($m.Name)"
    Step "$($m.Id) preflight" { [void](Invoke-Psql "$dir/preflight.sql") }
    Step "$($m.Id) apply" { [void](Invoke-Psql "$($m.Name).sql") }
    Step "$($m.Id) verify" { $r = Invoke-Psql "$dir/verify.sql"; Assert-Contains $r.Output "migration_$($m.Id)_verified" "$($m.Id) verify" }
    Step "$($m.Id) smoke (rolled back)" { $r = Invoke-Psql "$dir/smoke.sql"; Assert-Contains $r.Output "migration_$($m.Id)_smoke_passed" "$($m.Id) smoke" }
  }

  # Idempotency: every migration a second time, then verify again.
  foreach ($m in $Typed) {
    Step "$($m.Id) apply a second time (idempotent)" { [void](Invoke-Psql "$($m.Name).sql") }
  }
  Step 'runtime role grants again, after 012 to 015' { [void](Invoke-Psql 'runtime_role.grants.sql') }
  foreach ($m in $Typed) {
    Step "$($m.Id) verify after re-run and grants" { $r = Invoke-Psql "ops/$($m.Name)/verify.sql"; Assert-Contains $r.Output "migration_$($m.Id)_verified" "$($m.Id) verify" }
    Step "$($m.Id) smoke with the runtime role present" { $r = Invoke-Psql "ops/$($m.Name)/smoke.sql"; Assert-Contains $r.Output "migration_$($m.Id)_smoke_passed" "$($m.Id) smoke" }
  }

  Step 'existing 010 verify and runtime RLS probe still pass' {
    [void](Invoke-Psql '010_runtime_rls_policies.verify.sql')
    [void](Invoke-Psql 'runtime_role.rls.verify.sql')
  }
  Step 'nothing from migrations 001 to 011 changed' {
    $digestAfter = Get-CatalogDigest
    if ($digestAfter -ne $digestBefore) { throw "catalog digest changed: $digestBefore -> $digestAfter" }
    $legacyAfter = Invoke-Scalar "SELECT (SELECT count(*) FROM forensic.records) || ',' || (SELECT count(*) FROM forensic.cdr_records) || ',' || (SELECT count(*) FROM forensic.evidence_items)"
    if ($legacyAfter -ne $legacyRows) { throw "legacy row counts changed: $legacyRows -> $legacyAfter" }
  }

  if ($VolumeRows -gt 0) {
    Step "volume probe ($VolumeRows call events, $([int]($VolumeRows / 2)) sessions)" {
      $r = Invoke-Psql 'ops/volume_probe.sql' -Variables @("rows=$VolumeRows")
      $r.Output -split "`n" | Where-Object { $_ -match '^(===|---|Time:|\s+\S+\s+\|\s|\s*reload)' } |
        ForEach-Object { Write-Host "      $_" }
      Assert-Contains $r.Output 'reload_inserted_nothing' 'the volume probe re-load'
    }
  }

  # Rollback pass, reverse order, with the gate refusing first.
  $reverse = @($Typed)
  [array]::Reverse($reverse)
  foreach ($m in $reverse) {
    $dir = "ops/$($m.Name)"
    Step "$($m.Id) rollback refuses without its gate" {
      $r = Invoke-Psql "$dir/rollback.sql" -AllowFailure
      if ($r.Exit -eq 0) { throw "$($m.Id) rollback ran without the gate setting" }
      Assert-Contains $r.Output 'destructive' "$($m.Id) gate message"
    }
    Step "$($m.Id) rollback with the gate" {
      [void](Invoke-Psql "$dir/rollback.sql" -Before @("SET $($m.Gate) = 'true'"))
      foreach ($t in $m.Tables) {
        $present = Invoke-Scalar "SELECT to_regclass('forensic.$t') IS NOT NULL"
        if ($present -ne 'f') { throw "forensic.$t still exists after the $($m.Id) rollback" }
      }
    }
  }
  Step 'the 012 rollback refuses while relationship_evidence exists' {
    [void](Invoke-Psql '013_reference_tables.sql')
    [void](Invoke-Psql '014_entity_relationship_layer.sql')
    [void](Invoke-Psql '012_typed_families.sql')
    $r = Invoke-Psql 'ops/012_typed_families/rollback.sql' -AllowFailure -Before @("SET app.typed_families_allow_destructive_rollback = 'true'")
    if ($r.Exit -eq 0) { throw 'the 012 rollback ran while the relationship layer was present' }
    Assert-Contains $r.Output 'roll back migrations 015, 014 and 013 before 012' '012 ordering guard'
  }

  # Re-apply everything from the rolled-back state (014 and 013 and 012 were just re-created; add 015).
  Step '015 re-apply and full verify of 012 to 015 from the rolled-back state' {
    [void](Invoke-Psql '015_vectors_and_earth.sql')
    [void](Invoke-Psql 'runtime_role.grants.sql')
    foreach ($m in $Typed) {
      $r = Invoke-Psql "ops/$($m.Name)/verify.sql"
      Assert-Contains $r.Output "migration_$($m.Id)_verified" "$($m.Id) verify after re-application"
    }
  }
  Step 'nothing from migrations 001 to 011 changed after rollback and re-application' {
    $digestFinal = Get-CatalogDigest
    if ($digestFinal -ne $digestBefore) { throw "catalog digest changed: $digestBefore -> $digestFinal" }
  }
} finally {
  if ($started -and -not $KeepContainer) {
    & docker stop $Container | Out-Null
    Write-Host "Container $Container stopped and removed (it ran with --rm)."
  } elseif ($started) {
    Write-Host "Container $Container left running (-KeepContainer). Remove it with: docker stop $Container"
  }
}

$total = [math]::Round(($Results | Measure-Object -Property Seconds -Sum).Sum, 1)
Write-Host "MigrationThrowawayTest=PASS Steps=$($Results.Count) Seconds=$total Image=$Image"
