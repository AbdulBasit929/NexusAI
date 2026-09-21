# Read-only follow-up to the owner's bounded speech/demo acceptance.
param([ValidateSet('evidence','history')][string]$Mode = 'evidence')
$ErrorActionPreference = 'Stop'
$repo = Split-Path -Parent $PSScriptRoot
. (Join-Path $PSScriptRoot 'forensic_runtime_common.ps1')
$runtime = Get-ForensicRuntimeEnvironment (Join-Path $repo '.env.forensic-runtime.local')
$collection = 'nexusai-multimodal-product-acceptance'
$headers = @{
  Authorization = "Bearer $($runtime.FORENSIC_RECORDS_API_KEY)"
  'X-Forensic-Tenant-ID' = 'default'
  'X-Forensic-Actor-ID' = 'nexusai-speech-acceptance-operator'
  'X-Forensic-Subject-ID' = 'nexusai-speech-acceptance-operator'
  'X-Forensic-Actor-Role' = 'admin'
  'X-Forensic-Collection-ID' = $collection
  'X-Forensic-Case-ID' = $collection
}
$base = 'http://localhost:8091'
$private = Join-Path $repo 'local-acceptance-models/nxmmr/private-benchmarks/speech-breadth-v1/fleurs'
git check-ignore "$private/live-api.json" | Out-Null
if ($LASTEXITCODE -ne 0) { throw 'Private receipt must be Git ignored' }
if ($Mode -eq 'history') {
  if (Test-Path -LiteralPath "$private/live-history.json") { throw 'Immutable history receipt already exists' }
  $history = Invoke-RestMethod -TimeoutSec 30 -Uri "http://localhost:8080/api/agents/Forensic_Records_Analyst/history?case_id=$collection&collection_id=$collection&limit=20"
  $expected = @('72c3ec8c-d7c2-4828-8333-65b39409039e','dafbc2a1-724c-4190-ba5b-f3cd78b9c926','97bc6791-e420-488c-b52d-4c2e685920d3','e7baf87f-2e63-4e9d-a664-54e9866cbbc7','1b9165fd-185e-4683-930c-ff3c25eace8c')
  $items = @($history.items | Where-Object { $_.analysis_id -in $expected })
  if ($items.Count -ne 5) { throw 'Expected five bounded STT analysis receipts' }
  $items | ConvertTo-Json -Depth 60 | Set-Content -LiteralPath "$private/live-history.json" -Encoding utf8
  $items | Select-Object analysis_id,status,answer | ConvertTo-Json -Depth 4
  exit
}
if (Test-Path -LiteralPath "$private/live-api.json") { throw 'Immutable live receipt already exists' }
$ids = @{
  english = '22a595a9-bd51-4da0-8cde-dd892b55842e'
  urdu = '2097c6df-c0ad-4886-b0e7-ea4f4856a220'
  face_a = 'c23c3e10-a7a6-435a-ae64-c411a7cd0e14'
}
$details = @{}
$ranges = @{}
foreach ($name in $ids.Keys) {
  $details[$name] = Invoke-RestMethod -TimeoutSec 30 -Headers $headers -Uri "$base/evidence/$($ids[$name])?limit=50"
  if ($name -ne 'face_a') {
    $rangeHeaders = $headers.Clone()
    $rangeHeaders.Range = 'bytes=0-31'
    $response = Invoke-WebRequest -TimeoutSec 30 -Headers $rangeHeaders -Uri "$base/evidence/$($ids[$name])/content?tenant_id=default&collection_id=$collection&case_id=$collection"
    $ranges[$name] = @{ status=[int]$response.StatusCode; range=$response.Headers.'Content-Range'; bytes=$response.RawContentStream.Length }
  }
}
$rankings = @{}
$candidates = @('3fff8296-ee69-4283-a498-7a0000887072','769f6124-5b06-4c73-9122-39ef4423e0c4','273bc132-74b5-4633-b8f9-bf0a812ef3ff')
foreach ($kind in @('face','image')) {
  $type = if ($kind -eq 'face') { 'forensics.face-observation/v1' } else { 'forensics.image-embedding-observation/v1' }
  $artifact = @($details.face_a.derived_artifacts | Where-Object artifact_type -eq $type)[0]
  if (-not $artifact) { throw "Existing $kind reference observation unavailable" }
  $candidateQuery = ($candidates | ForEach-Object { "candidate_evidence_id=$_" }) -join '&'
  $uri = "$base/${kind}s/similar?authorization=explicit_case_evidence_scope&query_${kind}_observation_id=$($artifact.metadata.observation_id)&$candidateQuery&top_k=3&tenant_id=default&collection_id=$collection&case_id=$collection"
  $rankings[$kind] = Invoke-RestMethod -TimeoutSec 30 -Headers $headers -Uri $uri
}
$receipt = @{ recorded_at=(Get-Date).ToUniversalTime().ToString('o'); collection=$collection; details=$details; ranges=$ranges; rankings=$rankings; inference_performed=$false; retained_evidence_write=$false }
$receipt | ConvertTo-Json -Depth 50 | Set-Content -LiteralPath "$private/live-api.json" -Encoding utf8
foreach ($name in @('english','urdu')) {
  $artifacts = @($details[$name].derived_artifacts)
  [pscustomobject]@{ source=$name; evidence_id=$ids[$name]; status=$details[$name].item.processing_status; transcript_count=@($artifacts | Where-Object artifact_type -eq 'forensics.audio-timestamp-segment/v1').Count; roman_count=@($artifacts | Where-Object artifact_type -eq 'forensics.audio-roman-urdu-segment/v1').Count; range=$ranges[$name] } | ConvertTo-Json -Depth 4
}
foreach ($kind in $rankings.Keys) { $rankings[$kind] | Select-Object -Property * -ExcludeProperty query | ConvertTo-Json -Depth 10 }
