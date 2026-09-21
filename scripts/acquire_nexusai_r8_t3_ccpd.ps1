param(
  [ValidateSet('ccpd-2019-zenodo-15647076','ccpd-2020-green-zenodo-15647076')]
  [string]$ArtifactId = 'ccpd-2019-zenodo-15647076',
  [string]$OutputRoot = 'C:\NexusAI-Evaluation\R8\T3\CCPD',
  [switch]$ExecuteDownload,
  [switch]$Extract
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repoRoot = Split-Path -Parent $PSScriptRoot
$contractPath = Join-Path $repoRoot 'configuration\nexusai_r8_t3_acquisition.json'
$contract = Get-Content -Raw -LiteralPath $contractPath | ConvertFrom-Json
$artifact = @($contract.artifacts | Where-Object artifact_id -eq $ArtifactId)
if ($artifact.Count -ne 1) { throw "Unknown or ambiguous artifact: $ArtifactId" }
$artifact = $artifact[0]

if ($contract.production_mutation -or $contract.retained_case_ingest -or $contract.automatic_promotion) {
  throw 'T3 contract attempts to authorize a prohibited action'
}
if (-not [IO.Path]::IsPathRooted($OutputRoot)) { throw 'OutputRoot must be absolute' }
$fullRoot = [IO.Path]::GetFullPath($OutputRoot)
$fullRepo = [IO.Path]::GetFullPath($repoRoot)
if ($fullRoot.StartsWith($fullRepo, [StringComparison]::OrdinalIgnoreCase)) {
  throw 'Real T3 evidence must never be stored inside the repository'
}

$driveRoot = Split-Path $fullRoot -Qualifier
$drive = [IO.DriveInfo]::new($driveRoot)
$requiredFree = [int64]$artifact.minimum_free_disk_gib_for_download * 1GB
if ($drive.AvailableFreeSpace -lt $requiredFree) {
  throw "Insufficient free disk for $ArtifactId; require at least $($artifact.minimum_free_disk_gib_for_download) GiB"
}

$destinationRoot = Join-Path $fullRoot $ArtifactId
$destination = Join-Path $destinationRoot ([string]$artifact.filename)
$partial = "$destination.partial"
Write-Host "R8T3AcquisitionPreflight=PASS Artifact=$ArtifactId Bytes=$($artifact.size_bytes) FreeGiB=$([math]::Round($drive.AvailableFreeSpace/1GB,2)) Destination=$destination Privacy=isolated_evaluation_only ProductionMutation=false"

if (-not $ExecuteDownload) {
  Write-Host 'R8T3Acquisition=NOT_STARTED Reason=ExecuteDownload_not_supplied'
  return
}

$mutex = [Threading.Mutex]::new($false, 'Local\NexusAIR8T3CCPDAcquisition')
if (-not $mutex.WaitOne(0)) { $mutex.Dispose(); throw 'Another NexusAI R8 T3 acquisition is running' }
try {
  New-Item -ItemType Directory -Force -Path $destinationRoot | Out-Null
  if (Test-Path -LiteralPath $destination) {
    $file = Get-Item -LiteralPath $destination
    if ($file.Length -ne [int64]$artifact.size_bytes) { throw 'Existing final artifact has the wrong byte size' }
    $md5 = (Get-FileHash -LiteralPath $destination -Algorithm MD5).Hash.ToLowerInvariant()
    if ($md5 -ne ([string]$artifact.checksum).ToLowerInvariant()) { throw 'Existing final artifact fails publisher MD5' }
  } else {
    & python (Join-Path $repoRoot 'scripts\download_nexusai_r8_artifact.py') `
      --url ([string]$artifact.url) --destination $partial --expected-size ([int64]$artifact.size_bytes) `
      --attempts 1000 --read-timeout-seconds 120 --max-retry-delay-seconds 60 --progress-interval-mib 64
    if ($LASTEXITCODE -ne 0) { throw "Resumable download failed; retain partial file: $partial" }
    $file = Get-Item -LiteralPath $partial
    if ($file.Length -ne [int64]$artifact.size_bytes) { throw "Size mismatch: expected=$($artifact.size_bytes) actual=$($file.Length)" }
    $md5 = (Get-FileHash -LiteralPath $partial -Algorithm MD5).Hash.ToLowerInvariant()
    if ($md5 -ne ([string]$artifact.checksum).ToLowerInvariant()) { throw 'Downloaded artifact fails publisher MD5; partial retained for investigation' }
    Move-Item -LiteralPath $partial -Destination $destination
  }

  $sha256 = (Get-FileHash -LiteralPath $destination -Algorithm SHA256).Hash.ToLowerInvariant()
  $receipt = [ordered]@{
    contract_version = 'nexusai.r8-t3-artifact-receipt/v1'
    artifact_id = $ArtifactId
    acquired_at_utc = (Get-Date).ToUniversalTime().ToString('o')
    source_url = [string]$artifact.url
    filename = [string]$artifact.filename
    size_bytes = (Get-Item -LiteralPath $destination).Length
    publisher_md5 = ([string]$artifact.checksum).ToLowerInvariant()
    sha256 = $sha256
    scope = 'isolated_offline_benchmark_only'
    safe_for_git = $false
    safe_for_demo = $false
    training_allowed = $false
    production_mutation = $false
  }
  $receiptPath = Join-Path $destinationRoot 'acquisition-receipt.json'
  [IO.File]::WriteAllText($receiptPath, (($receipt | ConvertTo-Json -Depth 6) + [Environment]::NewLine), [Text.UTF8Encoding]::new($false))
  Write-Host "R8T3Acquisition=PASS Artifact=$ArtifactId Receipt=$receiptPath SHA256=$sha256 ProductionMutation=false AutomaticPromotion=false"

  if ($Extract) {
    $extractRoot = Join-Path $destinationRoot 'extracted'
    if (Test-Path -LiteralPath $extractRoot) { throw "Extraction target already exists; refusing overwrite: $extractRoot" }
    $archiveValidator = Join-Path $repoRoot 'scripts\validate_nexusai_r8_t3_archive.py'
    & python $archiveValidator --archive $destination --expected-bytes ([int64]$artifact.size_bytes)
    if ($LASTEXITCODE -ne 0) { throw 'Hostile-archive validation failed; extraction not started' }
    New-Item -ItemType Directory -Path $extractRoot | Out-Null
    & tar.exe -xf $destination -C $extractRoot
    if ($LASTEXITCODE -ne 0) { throw 'Archive extraction failed; inspect isolated extraction directory' }
    Write-Host "R8T3Extraction=PASS Root=$extractRoot Validation=pending_manifest_and_privacy_checks"
  }
} finally {
  $mutex.ReleaseMutex()
  $mutex.Dispose()
}
