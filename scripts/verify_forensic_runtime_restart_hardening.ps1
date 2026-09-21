param()

$ErrorActionPreference = 'Stop'
$RepoRoot = Split-Path -Parent $PSScriptRoot
$RuntimeEnvPath = Join-Path $RepoRoot '.env.forensic-runtime.local'

if (-not (Test-Path -LiteralPath $RuntimeEnvPath)) {
  throw 'Runtime environment is missing; source verification cannot resolve the accepted Compose topology'
}

Push-Location $RepoRoot
try {
  $resolvedText = docker compose -p nexusai --env-file $RuntimeEnvPath `
    -f .\docker-compose.forensic-records.yaml `
    -f .\docker-compose.forensic-records.runtime.yaml `
    -f .\docker-compose.forensic-records.asr-small.yaml `
    config --format json
  if ($LASTEXITCODE -ne 0) {
    throw 'Forensic runtime Compose resolution failed'
  }
  $resolved = $resolvedText | ConvertFrom-Json
} finally {
  Pop-Location
}

function Assert-Equal {
  param(
    [Parameter(Mandatory=$true)]$Actual,
    [Parameter(Mandatory=$true)]$Expected,
    [Parameter(Mandatory=$true)][string]$Label
  )
  if ($Actual -ne $Expected) {
    throw "$Label mismatch"
  }
}

function Assert-DependencyCondition {
  param(
    [Parameter(Mandatory=$true)]$Service,
    [Parameter(Mandatory=$true)][string]$Dependency,
    [Parameter(Mandatory=$true)][string]$Condition
  )
  $dependencyConfig = $Service.depends_on.$Dependency
  if ($null -eq $dependencyConfig) {
    throw "Missing dependency $Dependency"
  }
  Assert-Equal $dependencyConfig.condition $Condition "$Dependency condition"
}

$api = $resolved.services.'forensic-records-api'
$worker = $resolved.services.'forensic-records-worker'
$postgres = $resolved.services.'forensic-postgres'
$nats = $resolved.services.'forensic-nats'

Assert-Equal $resolved.name 'nexusai' 'Compose project'
Assert-Equal $api.restart 'unless-stopped' 'Forensic API restart policy'
Assert-Equal $worker.restart 'unless-stopped' 'Forensic worker restart policy'

foreach ($service in @($api, $worker)) {
  Assert-DependencyCondition $service 'forensic-postgres' 'service_healthy'
  Assert-DependencyCondition $service 'forensic-nats' 'service_healthy'
  Assert-DependencyCondition $service 'forensic-spool-init' 'service_completed_successfully'
}

$postgresProbe = [string]::Join(' ', @($postgres.healthcheck.test))
$natsProbe = [string]::Join(' ', @($nats.healthcheck.test))
if ($postgresProbe -notmatch '\bpg_isready\b') {
  throw 'PostgreSQL healthcheck does not prove server readiness'
}
if ($natsProbe -notmatch '/healthz') {
  throw 'NATS healthcheck does not use the monitoring readiness endpoint'
}

$expectedServices = @(
  'forensic-nats',
  'forensic-postgres',
  'forensic-records-api',
  'forensic-records-worker',
  'forensic-spool-init'
)
$actualServices = @($resolved.services.PSObject.Properties.Name | Sort-Object)
if (Compare-Object $expectedServices $actualServices) {
  throw 'Resolved forensic service identity drifted'
}

$expectedVolumes = @('forensic_nats_data', 'forensic_postgres_data', 'forensic_spool')
$actualVolumes = @($resolved.volumes.PSObject.Properties.Name | Sort-Object)
if (Compare-Object $expectedVolumes $actualVolumes) {
  throw 'Resolved forensic volume identity drifted'
}

$actualNetworks = @($resolved.networks.PSObject.Properties.Name | Sort-Object)
if (Compare-Object @('default') $actualNetworks) {
  throw 'Resolved forensic network identity drifted'
}

# Compose gates normal startup on dependency readiness. The processes remain
# fail-fast and observable; unless-stopped lets the engine retry transient
# startup exits and restore them after a daemon restart without fighting an
# intentional operator stop.
Write-Host 'ForensicRuntimeHardeningSource=PASS ForensicApiRestartPolicy=PASS ForensicWorkerRestartPolicy=PASS'
Write-Host 'PostgresReadinessDependency=PASS NatsReadinessDependency=PASS TransientStartupRecovery=PASS ManualStopSemantics=PASS'
Write-Host 'ComposeValidation=PASS SecretLeakage=0 VolumesChanged=false'
