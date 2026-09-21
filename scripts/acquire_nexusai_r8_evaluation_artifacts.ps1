param(
  [string]$OutputRoot = 'C:\NexusAI-Evaluation\R8\Artifacts\1.0.0',
  [string[]]$Candidate = @(
    'omz-vehicle-license-plate-detection-barrier-0106-fp32',
    'omz-vehicle-license-plate-detection-barrier-0123-source',
    'parseq-tiny-v1.0.0',
    'easyocr-arabic-g1-v1.7.2'
  ),
  [switch]$PreflightOnly
)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
$manifestPath = Join-Path $repoRoot 'configuration\nexusai_r8_evaluation_artifacts.json'
$manifest = Get-Content -Raw -LiteralPath $manifestPath | ConvertFrom-Json
$mutex = New-Object System.Threading.Mutex($false, 'Local\NexusAIR8EvaluationArtifactAcquisition')
if (-not $mutex.WaitOne(0)) {
  $mutex.Dispose()
  throw 'Another NexusAI R8 artifact acquisition is already running'
}

function Assert-R8ArtifactFile {
  param(
    [Parameter(Mandatory=$true)][string]$Path,
    [Parameter(Mandatory=$true)]$Artifact
  )
  $file = Get-Item -LiteralPath $Path
  if ($file.Length -ne [int64]$Artifact.size_bytes) {
    throw "Size mismatch for $($Artifact.name): expected=$($Artifact.size_bytes) actual=$($file.Length)"
  }
  if (-not [string]::IsNullOrWhiteSpace([string]$Artifact.checksum_algorithm)) {
    $actual = (Get-FileHash -LiteralPath $Path -Algorithm ([string]$Artifact.checksum_algorithm)).Hash.ToLowerInvariant()
    $expected = ([string]$Artifact.checksum).ToLowerInvariant()
    if ($actual -ne $expected) {
      throw "Checksum mismatch for $($Artifact.name): algorithm=$($Artifact.checksum_algorithm)"
    }
  }
}

function Invoke-R8Download {
  param(
    [Parameter(Mandatory=$true)][string]$Url,
    [Parameter(Mandatory=$true)][string]$Destination,
    [Parameter(Mandatory=$true)][int64]$ArtifactSize
  )
  $partial = "$Destination.partial"
  $downloadResult = & python (Join-Path $repoRoot 'scripts\download_nexusai_r8_artifact.py') --url $Url --destination $partial --expected-size $ArtifactSize
  if ($LASTEXITCODE -ne 0) { throw "Resumable download failed: $Url" }
  Write-Host "R8ArtifactDownload=$downloadResult"
  return $partial
}

try {
  & python (Join-Path $repoRoot 'scripts\validate_nexusai_r8_evaluation_artifacts.py') --manifest $manifestPath
  if ($LASTEXITCODE -ne 0) { throw 'R8 acquisition manifest validation failed' }

  if ($manifest.production_role_assignment -or $manifest.production_runtime_mutation -or $manifest.retained_case_ingest -or $manifest.automatic_promotion) {
    throw 'The acquisition contract attempts to authorize a prohibited production action'
  }
  $known = @($manifest.candidates | ForEach-Object { $_.candidate_id })
  foreach ($id in $Candidate) {
    if ($id -notin $known) { throw "Unknown or unapproved candidate: $id" }
  }
  $selected = @($manifest.candidates | Where-Object { $_.candidate_id -in $Candidate })
  $requiredBytes = [int64](($selected.artifacts | Measure-Object -Property size_bytes -Sum).Sum)
  $driveRoot = Split-Path $OutputRoot -Qualifier
  $driveInfo = New-Object System.IO.DriveInfo($driveRoot)
  $freeBytes = [int64]$driveInfo.AvailableFreeSpace
  if ($freeBytes -lt ($requiredBytes + 2GB)) {
    throw "Insufficient free disk: require downloaded bytes plus a 2 GiB safety margin"
  }
  if ($PreflightOnly) {
    Write-Host "R8ArtifactAcquisitionPreflight=PASS Candidates=$($selected.Count) DownloadBytes=$requiredBytes OutputRoot=$OutputRoot ProductionMutation=false"
    return
  }

  New-Item -ItemType Directory -Force -Path $OutputRoot | Out-Null
  $receiptCandidates = @()
  foreach ($entry in $selected) {
    $candidateRoot = Join-Path $OutputRoot ([string]$entry.candidate_id)
    New-Item -ItemType Directory -Force -Path $candidateRoot | Out-Null
    $receiptArtifacts = @()
    foreach ($artifact in $entry.artifacts) {
      $destination = Join-Path $candidateRoot ([string]$artifact.name)
      if (Test-Path -LiteralPath $destination) {
        Assert-R8ArtifactFile -Path $destination -Artifact $artifact
      } else {
        $partial = Invoke-R8Download -Url ([string]$artifact.url) -Destination $destination -ArtifactSize ([int64]$artifact.size_bytes)
        Assert-R8ArtifactFile -Path $partial -Artifact $artifact
        Move-Item -LiteralPath $partial -Destination $destination
      }
      if ($null -ne $artifact.expected_archive_members) {
        foreach ($member in @($artifact.expected_archive_members)) {
          $archiveResult = & python (Join-Path $repoRoot 'scripts\validate_nexusai_r8_archive.py') `
            --archive $destination --member ([string]$member.name) --size ([int64]$member.size_bytes) `
            --algorithm ([string]$member.checksum_algorithm) --checksum ([string]$member.checksum)
          if ($LASTEXITCODE -ne 0) { throw "Archive member validation failed for $($artifact.name)" }
          Write-Host "R8ArchiveValidation=$archiveResult"
        }
      }
      $receiptArtifacts += [ordered]@{
        name = [string]$artifact.name
        source_url = [string]$artifact.url
        size_bytes = (Get-Item -LiteralPath $destination).Length
        sha256 = (Get-FileHash -LiteralPath $destination -Algorithm SHA256).Hash.ToLowerInvariant()
        publisher_checksum_algorithm = $artifact.checksum_algorithm
        publisher_checksum = $artifact.checksum
      }
    }
    $licensePath = Join-Path $candidateRoot 'LICENSE.txt'
    if (-not (Test-Path -LiteralPath $licensePath)) {
      $licensePartial = "$licensePath.partial"
      & curl.exe --fail --location --connect-timeout 30 --speed-limit 1024 --speed-time 60 --retry 5 --retry-delay 3 --output $licensePartial ([string]$entry.license_url)
      if ($LASTEXITCODE -ne 0) { throw "License download failed for $($entry.candidate_id)" }
      Move-Item -LiteralPath $licensePartial -Destination $licensePath
    }
    $noticeSha256 = $null
    if (-not [string]::IsNullOrWhiteSpace([string]$entry.notice_url)) {
      $noticePath = Join-Path $candidateRoot 'NOTICE.txt'
      if (-not (Test-Path -LiteralPath $noticePath)) {
        $noticePartial = "$noticePath.partial"
        & curl.exe --fail --location --connect-timeout 30 --speed-limit 1024 --speed-time 60 --retry 5 --retry-delay 3 --output $noticePartial ([string]$entry.notice_url)
        if ($LASTEXITCODE -ne 0) { throw "Notice download failed for $($entry.candidate_id)" }
        Move-Item -LiteralPath $noticePartial -Destination $noticePath
      }
      $noticeSha256 = (Get-FileHash -LiteralPath $noticePath -Algorithm SHA256).Hash.ToLowerInvariant()
    }
    $receiptCandidates += [ordered]@{
      candidate_id = [string]$entry.candidate_id
      revision = [string]$entry.revision
      role = [string]$entry.role
      license = [string]$entry.license
      license_sha256 = (Get-FileHash -LiteralPath $licensePath -Algorithm SHA256).Hash.ToLowerInvariant()
      notice_sha256 = $noticeSha256
      artifacts = $receiptArtifacts
      status = 'acquired_for_isolated_evaluation_not_accepted_not_promoted'
    }
  }
  $receiptPath = Join-Path $OutputRoot 'acquisition-receipt.json'
  [ordered]@{
    contract_version = 'nexusai.r8-evaluation-artifact-receipt/v1'
    acquired_at_utc = (Get-Date).ToUniversalTime().ToString('o')
    source_manifest_sha256 = (Get-FileHash -LiteralPath $manifestPath -Algorithm SHA256).Hash.ToLowerInvariant()
    scope = 'isolated_offline_evaluation_only'
    production_role_assignment = $false
    production_runtime_mutation = $false
    retained_case_ingest = $false
    automatic_promotion = $false
    candidates = $receiptCandidates
  } | ConvertTo-Json -Depth 12 | Set-Content -LiteralPath $receiptPath -Encoding UTF8
  Write-Host "R8ArtifactAcquisition=PASS Candidates=$($receiptCandidates.Count) Receipt=$receiptPath ProductionMutation=false AutomaticPromotion=false"
} finally {
  $mutex.ReleaseMutex()
  $mutex.Dispose()
}
