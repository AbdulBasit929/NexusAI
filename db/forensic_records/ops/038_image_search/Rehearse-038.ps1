<#
.SYNOPSIS
  Rehearses migration 038 (image search) on a THROWAWAY PostgreSQL container. It never touches the NexusAI stack.

.DESCRIPTION
  Starts a disposable container from an image already on this machine (it never pulls), applies migrations 001 to 011, the runtime-role grants and 012 to 015 as a fresh database would see them,
  then for 038: preflight, apply, verify, smoke, a second apply (idempotency), the gate check of the rollback (it must refuse without its setting), the rollback, re-application, verify again.
  The container has no published port and a random password that is never printed. With -KeepContainer it is left running (name printed) for further tests such as the latency run; otherwise it is removed
  together with its data volume.

.EXAMPLE
  powershell -ExecutionPolicy Bypass -File .\db\forensic_records\ops\038_image_search\Rehearse-038.ps1
#>
[CmdletBinding()]
param(
  [string]$Image = 'timescale/timescaledb:latest-pg16',
  [int]$MemoryMB = 1536,
  [switch]$KeepContainer,
  # Publish the database on 127.0.0.1 at this port (for the Go acceptance tests); 0 = not published.
  [int]$PublishPort = 0
)

$ErrorActionPreference = 'Stop'
$MigrationDir = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$Container = 'nx-mig038-' + ([guid]::NewGuid().ToString('N').Substring(0, 8))
$Password = [guid]::NewGuid().ToString('N')
$Remote = '/tmp/mig'
$Base = @('001_phase1_records.sql', '002_job_row_counters.sql', '003_phase2_kb_assets_and_entities.sql', '004_dynamic_records.sql', '005_backfill_canonical_records.sql',
  '006_canonical_normalized_fields.sql', '007_evidence_items.sql', '008_phase3_evidence_control_plane.sql', '009_phase3_queue_lifecycle.sql', '010_runtime_rls_policies.sql',
  '011_r5_custody_chain_tail.sql')
$Typed = @('012_typed_families.sql', '013_reference_tables.sql', '014_entity_relationship_layer.sql', '015_vectors_and_earth.sql')

function Invoke-Docker {
  param([Parameter(Mandatory = $true)][string[]]$DockerArgs)
  $saved = $ErrorActionPreference
  try {
    $ErrorActionPreference = 'Continue'
    $lines = @(& docker @DockerArgs 2>&1 | ForEach-Object { if ($_ -is [System.Management.Automation.ErrorRecord]) { $_.Exception.Message } else { "$_" } })
    return [pscustomobject]@{ Exit = $LASTEXITCODE; Output = ($lines -join "`n") }
  } finally { $ErrorActionPreference = $saved }
}

function Invoke-Psql {
  param([Parameter(Mandatory = $true)][string]$File, [string[]]$Before = @(), [switch]$AllowFailure)
  $psqlArgs = @('exec', $Container, 'psql', '-v', 'ON_ERROR_STOP=1', '-U', 'localrecall', '-d', 'localrecall')
  foreach ($statement in $Before) { $psqlArgs += @('-c', $statement) }
  $psqlArgs += @('-f', "$Remote/$File")
  $result = Invoke-Docker $psqlArgs
  if ($result.Exit -ne 0 -and -not $AllowFailure) {
    ($result.Output -split "`n") | Select-Object -Last 25 | ForEach-Object { Write-Host "  psql> $_" }
    throw "$File failed (psql exit $($result.Exit))"
  }
  return $result
}

function Step {
  param([string]$Label, [scriptblock]$Body)
  $watch = [System.Diagnostics.Stopwatch]::StartNew()
  & $Body
  $watch.Stop()
  Write-Host ("  ok  {0,-64} {1,6:N1}s" -f $Label, $watch.Elapsed.TotalSeconds)
}

try {
  Write-Host "starting $Container from $Image"
  $portArgs = @()
  if ($PublishPort -gt 0) { $portArgs = @('-p', "127.0.0.1:${PublishPort}:5432") }
  $run = Invoke-Docker (@('run', '-d', '--name', $Container, '--memory', "${MemoryMB}m") + $portArgs + @('-e', 'POSTGRES_USER=localrecall', '-e', "POSTGRES_PASSWORD=$Password", '-e', 'POSTGRES_DB=localrecall', $Image))
  if ($run.Exit -ne 0) { throw "could not start the container: $($run.Output)" }
  $deadline = (Get-Date).AddSeconds(120)
  do {
    Start-Sleep -Seconds 2
    $ready = Invoke-Docker @('exec', $Container, 'pg_isready', '-h', '127.0.0.1', '-U', 'localrecall', '-d', 'localrecall')
    $log = (Invoke-Docker @('logs', $Container)).Output
  } while (-not ($ready.Exit -eq 0 -and $log -match 'init process complete; ready for start up') -and (Get-Date) -lt $deadline)
  if ($ready.Exit -ne 0) { throw 'the database did not become ready' }

  $copy = Invoke-Docker @('cp', $MigrationDir, "${Container}:$Remote")
  if ($copy.Exit -ne 0) { throw "could not copy the migrations: $($copy.Output)" }

  Step 'migrations 001 to 011' { foreach ($f in $Base) { [void](Invoke-Psql $f) } }
  Step 'runtime-role grants (before the typed migrations)' { [void](Invoke-Psql 'runtime_role.grants.sql') }
  Step 'migrations 012 to 015' { foreach ($f in $Typed) { [void](Invoke-Psql $f) } }
  Step 'runtime-role grants (again, as the compose file does)' { [void](Invoke-Psql 'runtime_role.grants.sql') }

  Step '038 preflight' { $r = Invoke-Psql 'ops/038_image_search/preflight.sql'; if ($r.Output -notmatch 'preflight passed') { throw 'preflight did not report success' } }
  Step '038 apply' { [void](Invoke-Psql '038_image_search.sql') }
  Step '038 verify' { $r = Invoke-Psql 'ops/038_image_search/verify.sql'; if ($r.Output -notmatch 'migration_038_verified') { throw 'verify did not report success' } }
  Step '038 smoke (synthetic, rolled back)' { $r = Invoke-Psql 'ops/038_image_search/smoke.sql'; if ($r.Output -notmatch 'migration_038_smoke_passed') { throw 'smoke did not report success' } }
  Step '038 apply again (idempotency)' { [void](Invoke-Psql '038_image_search.sql') }
  Step '038 verify after the second apply' { $r = Invoke-Psql 'ops/038_image_search/verify.sql'; if ($r.Output -notmatch 'migration_038_verified') { throw 'verify failed after reapply' } }
  Step 'rollback refuses without its setting' {
    $r = Invoke-Psql 'ops/038_image_search/rollback.sql' -AllowFailure
    if ($r.Exit -eq 0 -or $r.Output -notmatch 'destructive') { throw 'the rollback ran without its gate setting' }
  }
  Step 'rollback with the setting' {
    [void](Invoke-Psql 'ops/038_image_search/rollback.sql' -Before @("SET app.image_search_allow_destructive_rollback = 'true'"))
    $left = Invoke-Docker @('exec', $Container, 'psql', '-U', 'localrecall', '-d', 'localrecall', '-tA', '-c', "SELECT to_regclass('forensic.image_hash') IS NULL AND to_regclass('forensic.image_search_log') IS NULL AND to_regprocedure('forensic.image_index_state(text,text)') IS NULL")
    if ($left.Output.Trim() -ne 't') { throw 'the rollback left objects behind' }
    $kept = Invoke-Docker @('exec', $Container, 'psql', '-U', 'localrecall', '-d', 'localrecall', '-tA', '-c', "SELECT to_regclass('forensic.content_embedding') IS NOT NULL")
    if ($kept.Output.Trim() -ne 't') { throw 'the rollback dropped content_embedding' }
  }
  Step 'apply after rollback, verify' {
    [void](Invoke-Psql '038_image_search.sql')
    $r = Invoke-Psql 'ops/038_image_search/verify.sql'
    if ($r.Output -notmatch 'migration_038_verified') { throw 'verify failed after re-application' }
  }
  Write-Host 'migration 038 rehearsal passed'
  if ($KeepContainer) {
    Write-Host "container kept: $Container (remove with: docker rm -f -v $Container); the password is in its environment (docker inspect), not printed here"
    if ($PublishPort -gt 0) { Write-Host "published on 127.0.0.1:$PublishPort, database localrecall, user localrecall" }
  }
} finally {
  if (-not $KeepContainer) { [void](Invoke-Docker @('rm', '-f', '-v', $Container)) }
}
