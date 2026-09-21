[CmdletBinding()]
param(
  [string]$BaseUri = 'http://127.0.0.1:8088',
  [string]$Model = 'face-detect-yunet-sface',
  [string]$FixtureRoot = (Join-Path (Split-Path -Parent $PSScriptRoot) 'tests\fixtures\faces\nexusai-synthetic'),
  [string]$OutputPath = (Join-Path (Split-Path -Parent $PSScriptRoot) 'reports\post-bfa-multimodal-20260823\face-benchmark.json'),
  [string]$ContainerName = 'nexusai-face-benchmark'
)

$ErrorActionPreference = 'Stop'

$fixtureDefinitions = @(
  [ordered]@{ file = 'subject-a-frontal.png'; purpose = 'single clear frontal face and positive-pair anchor'; expected_faces = 1; truth = 'SAME_FIXTURE_SUBJECT:A'; derivative = $null },
  [ordered]@{ file = 'subject-a-pose.png'; purpose = 'moderate pose and same-subject positive'; expected_faces = 1; truth = 'SAME_FIXTURE_SUBJECT:A'; derivative = 'ImageGen edit of subject-a-frontal.png: approximately 20 degree pose, slight smile, softer focus' },
  [ordered]@{ file = 'subject-a-blur.png'; purpose = 'moderate blur'; expected_faces = 1; truth = 'SAME_FIXTURE_SUBJECT:A'; derivative = 'cv2.GaussianBlur(input, (21,21), 7)' },
  [ordered]@{ file = 'subject-b-frontal.png'; purpose = 'different-subject comparison'; expected_faces = 1; truth = 'DIFFERENT_FIXTURE_SUBJECT:B'; derivative = $null },
  [ordered]@{ file = 'group-four.png'; purpose = 'multiple faces including one smaller face'; expected_faces = 4; truth = 'SYNTHETIC_GROUP_COUNT:4'; derivative = $null },
  [ordered]@{ file = 'no-face-street.png'; purpose = 'no-face negative'; expected_faces = 0; truth = 'NO_FACE'; derivative = $null }
)

function ConvertTo-DataUri([string]$Path) {
  $bytes = [IO.File]::ReadAllBytes($Path)
  return 'data:image/png;base64,' + [Convert]::ToBase64String($bytes)
}

function Invoke-FacePost([string]$Route, [hashtable]$Payload) {
  $body = $Payload | ConvertTo-Json -Depth 8 -Compress
  $timer = [Diagnostics.Stopwatch]::StartNew()
  try {
    $response = Invoke-RestMethod -Uri ($BaseUri.TrimEnd('/') + $Route) -Method Post `
      -ContentType 'application/json' -Body $body -TimeoutSec 240
    $timer.Stop()
    return [ordered]@{ latency_ms = $timer.ElapsedMilliseconds; response = $response }
  } catch {
    $timer.Stop()
    $detail = $_.Exception.Message
    if ($_.ErrorDetails.Message) { $detail = $_.ErrorDetails.Message }
    throw "$Route failed after $($timer.ElapsedMilliseconds) ms: $detail"
  }
}

function Get-CosineSimilarity([double[]]$Left, [double[]]$Right) {
  if ($Left.Count -ne $Right.Count -or $Left.Count -eq 0) { throw 'Embedding dimensions do not match' }
  $dot = 0.0
  $leftNorm = 0.0
  $rightNorm = 0.0
  for ($i = 0; $i -lt $Left.Count; $i++) {
    $dot += $Left[$i] * $Right[$i]
    $leftNorm += $Left[$i] * $Left[$i]
    $rightNorm += $Right[$i] * $Right[$i]
  }
  if ($leftNorm -eq 0 -or $rightNorm -eq 0) { throw 'Zero-length embedding is invalid' }
  return $dot / ([Math]::Sqrt($leftNorm) * [Math]::Sqrt($rightNorm))
}

function Get-ContainerMemory {
  if (-not $ContainerName) { return $null }
  try {
    return (docker stats --no-stream --format '{{.MemUsage}}|{{.MemPerc}}' $ContainerName 2>$null).Trim()
  } catch { return $null }
}

$fixtureResults = @()
$embeddings = @{}
$memoryBefore = Get-ContainerMemory

foreach ($definition in $fixtureDefinitions) {
  $path = Join-Path $FixtureRoot $definition.file
  if (-not (Test-Path -LiteralPath $path -PathType Leaf)) { throw "Missing fixture: $path" }
  $dataUri = ConvertTo-DataUri $path
  $analysis = Invoke-FacePost '/v1/face/analyze' @{ model = $Model; img = $dataUri; actions = @() }
  $faces = @($analysis.response.faces)
  $embeddingResult = $null
  if ($definition.expected_faces -eq 1) {
    $embeddingResult = Invoke-FacePost '/v1/face/embed' @{ model = $Model; img = $dataUri }
    $embedding = [double[]]@($embeddingResult.response.embedding)
    $embeddings[$definition.file] = $embedding
  }
  $fixtureResults += [ordered]@{
    filename = $definition.file
    source = 'OpenAI ImageGen synthetic project fixture'
    url = $null
    publisher = 'OpenAI ImageGen'
    license = 'synthetic user-authorized project output; not third-party natural evidence'
    attribution = 'Synthetic fixture generated for NexusAI model acceptance; never represent as natural evidence.'
    sha256 = (Get-FileHash -Algorithm SHA256 -LiteralPath $path).Hash.ToLowerInvariant()
    bytes = (Get-Item -LiteralPath $path).Length
    purpose = $definition.purpose
    ground_truth = $definition.truth
    derivative_command = $definition.derivative
    expected_face_count = $definition.expected_faces
    detected_face_count = $faces.Count
    detection_count_pass = ($faces.Count -eq $definition.expected_faces)
    detection_latency_ms = $analysis.latency_ms
    faces = @($faces | ForEach-Object {
      [ordered]@{
        bbox = [ordered]@{ x = $_.region.x; y = $_.region.y; width = $_.region.w; height = $_.region.h }
        bbox_positive = ($_.region.w -gt 0 -and $_.region.h -gt 0 -and $_.region.x -ge 0 -and $_.region.y -ge 0)
        confidence = $_.face_confidence
      }
    })
    embedding = if ($embeddingResult) { [ordered]@{ generated = $true; dimension = $embeddingResult.response.dim; model = $embeddingResult.response.model; latency_ms = $embeddingResult.latency_ms } } else { [ordered]@{ generated = $false; reason = 'not applicable to multi-face/no-face fixture' } }
  }
}

$sameCosine = Get-CosineSimilarity $embeddings['subject-a-frontal.png'] $embeddings['subject-a-pose.png']
$blurCosine = Get-CosineSimilarity $embeddings['subject-a-frontal.png'] $embeddings['subject-a-blur.png']
$differentCosine = Get-CosineSimilarity $embeddings['subject-a-frontal.png'] $embeddings['subject-b-frontal.png']
$sameVerify = Invoke-FacePost '/v1/face/verify' @{
  model = $Model
  img1 = ConvertTo-DataUri (Join-Path $FixtureRoot 'subject-a-frontal.png')
  img2 = ConvertTo-DataUri (Join-Path $FixtureRoot 'subject-a-pose.png')
}
$differentVerify = Invoke-FacePost '/v1/face/verify' @{
  model = $Model
  img1 = ConvertTo-DataUri (Join-Path $FixtureRoot 'subject-a-frontal.png')
  img2 = ConvertTo-DataUri (Join-Path $FixtureRoot 'subject-b-frontal.png')
}

$allCountsPass = @($fixtureResults | Where-Object { -not $_.detection_count_pass }).Count -eq 0
$allBboxesPositive = @($fixtureResults.faces | Where-Object { -not $_.bbox_positive }).Count -eq 0
$allEmbeddings128 = @($fixtureResults | Where-Object { $_.expected_face_count -eq 1 -and $_.embedding.dimension -ne 128 }).Count -eq 0
$rankingPass = $sameCosine -gt $differentCosine

$report = [ordered]@{
  contract_version = 'nexusai.face-model-benchmark/v1'
  generated_utc = (Get-Date).ToUniversalTime().ToString('o')
  retained_mutation = $false
  natural_evidence = $false
  model = [ordered]@{
    id = $Model
    role = 'CPU face detection and evidence-scoped candidate visual similarity'
    embedding_dimension_expected = 128
    identity_claims_authorized = $false
    demographics_authorized = $false
    global_gallery_authorized = $false
  }
  fixtures = $fixtureResults
  similarity = [ordered]@{
    label = 'candidate visual similarity; not identity'
    same_fixture_subject_cosine = $sameCosine
    blurred_same_fixture_subject_cosine = $blurCosine
    different_fixture_subject_cosine = $differentCosine
    relative_ranking_pass = $rankingPass
    same_fixture_verify = [ordered]@{ verified = $sameVerify.response.verified; distance = $sameVerify.response.distance; threshold = $sameVerify.response.threshold; confidence = $sameVerify.response.confidence; latency_ms = $sameVerify.latency_ms }
    different_fixture_verify = [ordered]@{ verified = $differentVerify.response.verified; distance = $differentVerify.response.distance; threshold = $differentVerify.response.threshold; confidence = $differentVerify.response.confidence; latency_ms = $differentVerify.latency_ms }
  }
  runtime = [ordered]@{ container = $ContainerName; memory_before = $memoryBefore; memory_after = Get-ContainerMemory }
  acceptance = [ordered]@{
    detection_counts_pass = $allCountsPass
    returned_bboxes_positive = $allBboxesPositive
    embeddings_are_128d = $allEmbeddings128
    same_subject_ranks_above_different_subject = $rankingPass
    verdict = if ($allCountsPass -and $allBboxesPositive -and $allEmbeddings128 -and $rankingPass) { 'M1_ACCEPTABLE_LIMITED' } else { 'NOT_ACCEPTABLE' }
    limitations = @(
      'Synthetic fixtures test only basic M1 behavior and do not establish operational biometric accuracy.',
      'Similarity is candidate visual similarity, never identity.',
      'Bounding boxes require analyst visual review; this gate checks counts and positive coordinates only.'
    )
  }
}

$parent = Split-Path -Parent $OutputPath
if ($parent) { New-Item -ItemType Directory -Force -Path $parent | Out-Null }
$report | ConvertTo-Json -Depth 12 | Set-Content -Encoding utf8 -LiteralPath $OutputPath
$report | ConvertTo-Json -Depth 12
