param(
  [switch]$Approved,
  [string]$CollectionId = 'nexusai-runtime-acceptance-20260730',
  [string]$CaseId = 'runtime-acceptance-20260730'
)

$ErrorActionPreference = 'Stop'
if (-not $Approved) {
  throw 'Retained synthetic ingest requires explicit approval. Re-run with -Approved only after the bundled STIM-3/STIM-4 activation is authorized.'
}

$repoRoot = Split-Path -Parent $PSScriptRoot
$runtimeEnvPath = Join-Path $repoRoot '.env.forensic-runtime.local'
$fixtureRoot = Join-Path $repoRoot 'ingestion\forensic_records\tests\fixtures\stim4_runtime_acceptance'
. (Join-Path $PSScriptRoot 'forensic_runtime_common.ps1')
$runtime = Get-ForensicRuntimeEnvironment $runtimeEnvPath

$headers = @{
  Authorization = "Bearer $($runtime.FORENSIC_RECORDS_API_KEY)"
  'X-Forensic-Tenant-ID' = $runtime.FORENSIC_RECORDS_TENANT_ID
  'X-Forensic-Actor-ID' = 'nexusai-stim4-acceptance-operator'
  'X-Forensic-Subject-ID' = 'nexusai-stim4-acceptance-operator'
  'X-Forensic-Actor-Role' = 'admin'
  'X-Forensic-Collection-ID' = $CollectionId
  'X-Forensic-Case-ID' = $CaseId
}

$fixtures = @(
  [pscustomobject]@{ Name='cdr_alpha.csv'; Family='cdr'; Total=4; Accepted=3; Duplicate=1; Rejected=0 },
  [pscustomobject]@{ Name='cdr_bravo.csv'; Family='cdr'; Total=2; Accepted=2; Duplicate=0; Rejected=0 },
  [pscustomobject]@{ Name='cdr_charlie.tsv'; Family='cdr'; Total=2; Accepted=2; Duplicate=0; Rejected=0 },
  [pscustomobject]@{ Name='ipdr.csv'; Family='ipdr'; Total=3; Accepted=3; Duplicate=0; Rejected=0 },
  [pscustomobject]@{ Name='subscriber.jsonl'; Family='subscriber'; Total=4; Accepted=4; Duplicate=0; Rejected=0 },
  [pscustomobject]@{ Name='anpr_east.csv'; Family='anpr'; Total=3; Accepted=3; Duplicate=0; Rejected=0 },
  [pscustomobject]@{ Name='anpr_west.csv'; Family='anpr'; Total=3; Accepted=3; Duplicate=0; Rejected=0 }
)

function Invoke-AcceptanceUpload {
  param(
    [Parameter(Mandatory=$true)]$Fixture,
    [Parameter(Mandatory=$true)][string]$Path,
    [Parameter(Mandatory=$true)][bool]$ForceUpload
  )

  $curlArgs = @(
    '-sS', '--max-time', '60', '-X', 'POST', 'http://localhost:8091/webhooks/records/upload',
    '-H', "Authorization: Bearer $($runtime.FORENSIC_RECORDS_API_KEY)",
    '-H', "X-Forensic-Tenant-ID: $($runtime.FORENSIC_RECORDS_TENANT_ID)",
    '-H', 'X-Forensic-Actor-ID: nexusai-stim4-acceptance-operator',
    '-H', 'X-Forensic-Subject-ID: nexusai-stim4-acceptance-operator',
    '-H', 'X-Forensic-Actor-Role: admin',
    '-H', "X-Forensic-Collection-ID: $CollectionId",
    '-H', "X-Forensic-Case-ID: $CaseId",
    '-F', "tenant_id=$($runtime.FORENSIC_RECORDS_TENANT_ID)",
    '-F', "collection_id=$CollectionId",
    '-F', "case_id=$CaseId",
    '-F', 'user_id=nexusai-stim4-acceptance-operator',
    '-F', "record_type=$($Fixture.Family)",
    '-F', 'skip_kb_mirror=true',
    '-F', 'jurisdiction=PK',
    '-F', 'source_timezone=Asia/Karachi',
    '-F', "file=@$Path",
    '-w', "`n%{http_code}"
  )
  if ($ForceUpload) {
    $curlArgs += @('-F', 'force=true')
  }
  @(& curl.exe @curlArgs)
}

$results = @()
foreach ($fixture in $fixtures) {
  $path = Join-Path $fixtureRoot $fixture.Name
  if (-not (Test-Path -LiteralPath $path -PathType Leaf)) {
    throw "Synthetic acceptance fixture is missing: $path"
  }
  Write-Host "STIM4AcceptanceUpload=$($fixture.Name) Family=$($fixture.Family)"
  $rawWithStatus = Invoke-AcceptanceUpload -Fixture $fixture -Path $path -ForceUpload $false
  if ($LASTEXITCODE -ne 0 -or $rawWithStatus.Count -lt 2) {
    throw "Upload transport failed for $($fixture.Name)"
  }
  $statusCode = [int]$rawWithStatus[-1]
  if ($statusCode -lt 200 -or $statusCode -ge 300) {
    throw "Upload HTTP $statusCode for $($fixture.Name)"
  }
  $payload = ($rawWithStatus[0..($rawWithStatus.Count - 2)] -join "`n") | ConvertFrom-Json
  $evidenceId = if ($payload.evidence_id) { $payload.evidence_id } else { $payload.existing.evidence_id }
  if (-not $evidenceId) { throw "Upload returned no evidence identity for $($fixture.Name)" }

  $forcedReplacement = $false
  while ($true) {
    $deadline = (Get-Date).AddMinutes(5)
    $lastPoll = Get-Date
    $retryAfterForcedUpload = $false
    do {
      Start-Sleep -Seconds 2
      if (((Get-Date) - $lastPoll).TotalSeconds -ge 15) {
        Write-Host "STIM4AcceptanceWait=$($fixture.Name) EvidenceId=$evidenceId"
        $lastPoll = Get-Date
      }
      $detailUri = 'http://localhost:8091/evidence/{0}?limit=1&include_records_preview=false&view=accounting' -f [uri]::EscapeDataString([string]$evidenceId)
      $detail = Invoke-RestMethod -TimeoutSec 20 -Headers $headers -Uri $detailUri
      $completed = @($detail.ingest_jobs | Where-Object { $_.status -eq 'completed' -and [int]$_.total_rows -eq $fixture.Total })
      $dead = @($detail.ingest_jobs | Where-Object { $_.status -eq 'dead_letter' })
      if ($dead.Count -gt 0 -and $completed.Count -eq 0) { throw "$($fixture.Name) dead-lettered" }
      if ([string]$payload.status -eq 'duplicate' -and -not $forcedReplacement -and @($detail.ingest_jobs).Count -eq 0) {
        Write-Host "STIM4AcceptanceDuplicateWithoutIngestJob=$($fixture.Name) ForceReplacement=true"
        $rawWithStatus = Invoke-AcceptanceUpload -Fixture $fixture -Path $path -ForceUpload $true
        if ($LASTEXITCODE -ne 0 -or $rawWithStatus.Count -lt 2) {
          throw "Forced upload transport failed for $($fixture.Name)"
        }
        $statusCode = [int]$rawWithStatus[-1]
        if ($statusCode -lt 200 -or $statusCode -ge 300) {
          throw "Forced upload HTTP $statusCode for $($fixture.Name)"
        }
        $payload = ($rawWithStatus[0..($rawWithStatus.Count - 2)] -join "`n") | ConvertFrom-Json
        $evidenceId = if ($payload.evidence_id) { $payload.evidence_id } else { $payload.existing.evidence_id }
        if (-not $evidenceId) { throw "Forced upload returned no evidence identity for $($fixture.Name)" }
        $forcedReplacement = $true
        $retryAfterForcedUpload = $true
        break
      }
    } until ($completed.Count -gt 0 -or (Get-Date) -gt $deadline)
    if ($retryAfterForcedUpload) { continue }
    if ($completed.Count -gt 0) { break }
    break
  }
  if ($completed.Count -eq 0) { throw "Timed out waiting for $($fixture.Name)" }
  $job = $completed | Select-Object -First 1
  if ([string]$job.record_type -ne $fixture.Family) {
    throw "Classification mismatch for $($fixture.Name): expected $($fixture.Family), got $($job.record_type)"
  }
  if ([int]$job.accepted_rows -ne $fixture.Accepted -or
      [int]$job.duplicate_rows -ne $fixture.Duplicate -or
      [int]$job.rejected_rows -ne $fixture.Rejected) {
    throw "Accounting mismatch for $($fixture.Name)"
  }
  $fullDetailUri = 'http://localhost:8091/evidence/{0}?limit=1&include_records_preview=false' -f [uri]::EscapeDataString([string]$evidenceId)
  $fullDetail = Invoke-RestMethod -TimeoutSec 20 -Headers $headers -Uri $fullDetailUri
  $results += [pscustomobject]@{
    source_file = $fixture.Name
    expected_family = $fixture.Family
    evidence_id = $evidenceId
    version_id = $fullDetail.item.metadata.version_id
    total_rows = [int]$job.total_rows
    accepted_rows = [int]$job.accepted_rows
    duplicate_rows = [int]$job.duplicate_rows
    rejected_rows = [int]$job.rejected_rows
  }
}

$results | ConvertTo-Json -Depth 5
