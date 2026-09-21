param(
  [string]$CollectionId = 'nexusai-multimodal-product-acceptance',
  [string]$ReportPath = 'reports/unified-multimodal-product-acceptance-20260823/product-acceptance-api.json'
)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
. (Join-Path $PSScriptRoot 'forensic_runtime_common.ps1')
$runtime = Get-ForensicRuntimeEnvironment (Join-Path $repoRoot '.env.forensic-runtime.local')
$recordsURL = 'http://localhost:8091'
$localAIURL = 'http://localhost:8080'
$actor = 'nexusai-breadth-acceptance-operator'
$headers = @{
  Authorization = "Bearer $($runtime.FORENSIC_RECORDS_API_KEY)"
  'X-Forensic-Tenant-ID' = 'default'
  'X-Forensic-Actor-ID' = $actor
  'X-Forensic-Subject-ID' = $actor
  'X-Forensic-Actor-Role' = 'admin'
  'X-Forensic-Collection-ID' = $CollectionId
  'X-Forensic-Case-ID' = $CollectionId
}

function Invoke-Status([string]$Uri, $RequestHeaders = $headers) {
  $response = Invoke-WebRequest -SkipHttpErrorCheck -TimeoutSec 30 -Headers $RequestHeaders -Uri $Uri
  [ordered]@{
    status = [int]$response.StatusCode
    content_type = [string]$response.Headers.'Content-Type'
    content_disposition = [string]$response.Headers.'Content-Disposition'
    content_range = [string]$response.Headers.'Content-Range'
    evidence_sha256 = [string]$response.Headers.'X-Evidence-SHA256'
    bytes = [int]$response.RawContentStream.Length
  }
}

function Invoke-Range([string]$EvidenceId) {
  $rangeHeaders = $headers.Clone()
  $rangeHeaders.Range = 'bytes=0-31'
  $uri = "$recordsURL/evidence/$EvidenceId/content?tenant_id=default&collection_id=$CollectionId&case_id=$CollectionId"
  Invoke-Status $uri $rangeHeaders
}

function Get-Detail([string]$EvidenceId) {
  Invoke-RestMethod -TimeoutSec 30 -Headers $headers -Uri "$recordsURL/evidence/${EvidenceId}?limit=25"
}

function Get-Artifact($Detail, [string]$Type) {
  @($Detail.derived_artifacts | Where-Object artifact_type -eq $Type) | Select-Object -First 1
}

$inventory = Invoke-RestMethod -TimeoutSec 30 -Headers $headers -Uri "$recordsURL/evidence?collection_id=$CollectionId&limit=100"
if ([int]$inventory.pagination.evidence_total -ne 21) { throw "Expected 21 evidence items, got $($inventory.pagination.evidence_total)" }
$details = @{}
$artifactCounts = @{}
$jobCounts = @{ completed = 0; dead_letter = 0; active = 0; total = 0 }
foreach ($item in @($inventory.items)) {
  $detail = Get-Detail ([string]$item.evidence_id)
  $details[[string]$item.evidence_id] = $detail
  foreach ($artifact in @($detail.derived_artifacts)) {
    $type = [string]$artifact.artifact_type
    if (-not $artifactCounts.ContainsKey($type)) { $artifactCounts[$type] = 0 }
    $artifactCounts[$type]++
  }
  foreach ($job in @($detail.ingest_jobs)) {
    $jobCounts.total++
    if ($job.status -eq 'completed') { $jobCounts.completed++ }
    elseif ($job.status -eq 'dead_letter') { $jobCounts.dead_letter++ }
    elseif ($job.status -in @('queued','processing','retrying')) { $jobCounts.active++ }
  }
}

$ids = [ordered]@{
  plate = 'a0ff7b18-129f-4a8a-82cf-48887f98bd9e'
  road = 'd4b9daf1-da54-4c66-923c-c725bec4400c'
  face_a = 'c23c3e10-a7a6-435a-ae64-c411a7cd0e14'
  face_pose = '3fff8296-ee69-4283-a498-7a0000887072'
  face_blur = '769f6124-5b06-4c73-9122-39ef4423e0c4'
  face_b = '273bc132-74b5-4633-b8f9-bf0a812ef3ff'
  no_face = '3ffc0eae-f030-4e62-835f-100682b99b1c'
  audio = '2097c6df-c0ad-4886-b0e7-ea4f4856a220'
  video = '50057921-4f1f-4ab8-bab0-47bcdc957822'
  txt = 'ee48bd0b-acde-4b51-b8c6-d9194278efda'
  pdf = '88eedb87-658a-4f3e-a89c-8b5740e4a248'
  docx = 'b7608931-046a-4b29-9eef-241c7315f38f'
}

$faceArtifact = Get-Artifact $details[$ids.face_a] 'forensics.face-observation/v1'
$imageArtifact = Get-Artifact $details[$ids.face_a] 'forensics.image-embedding-observation/v1'
if (-not $faceArtifact -or -not $imageArtifact) { throw 'Required face or image query observation is unavailable.' }
$faceCandidates = @($ids.face_pose, $ids.face_blur, $ids.face_b)
$candidateQuery = ($faceCandidates | ForEach-Object { 'candidate_evidence_id=' + [uri]::EscapeDataString($_) }) -join '&'
$scopeQuery = "tenant_id=default&collection_id=$CollectionId&case_id=$CollectionId"
$faceURL = "$recordsURL/faces/similar?authorization=explicit_case_evidence_scope&query_face_observation_id=$($faceArtifact.metadata.observation_id)&$candidateQuery&top_k=3&$scopeQuery"
$faceResult = Invoke-RestMethod -TimeoutSec 30 -Headers $headers -Uri $faceURL
$imageCandidates = @($ids.face_pose, $ids.face_blur, $ids.face_b, $ids.no_face, $ids.road)
$imageCandidateQuery = ($imageCandidates | ForEach-Object { 'candidate_evidence_id=' + [uri]::EscapeDataString($_) }) -join '&'
$imageURL = "$recordsURL/images/similar?authorization=explicit_case_evidence_scope&query_image_observation_id=$($imageArtifact.metadata.observation_id)&$imageCandidateQuery&top_k=5&$scopeQuery"
$imageResult = Invoke-RestMethod -TimeoutSec 30 -Headers $headers -Uri $imageURL
$compareAPose = Invoke-RestMethod -TimeoutSec 30 -Headers $headers -Uri "$recordsURL/evidence/compare?evidence_id_a=$($ids.face_a)&evidence_id_b=$($ids.face_pose)&$scopeQuery"
$compareAB = Invoke-RestMethod -TimeoutSec 30 -Headers $headers -Uri "$recordsURL/evidence/compare?evidence_id_a=$($ids.face_a)&evidence_id_b=$($ids.face_b)&$scopeQuery"

$mediaChecks = [ordered]@{}
foreach ($name in @('plate','audio','video','txt','pdf','docx')) {
  $id = $ids[$name]
  $uri = "$recordsURL/evidence/$id/content?tenant_id=default&collection_id=$CollectionId&case_id=$CollectionId"
  $mediaChecks[$name] = [ordered]@{ full = Invoke-Status $uri; range = Invoke-Range $id }
}

$missingAuthorizationURL = "$recordsURL/faces/similar?query_face_observation_id=$($faceArtifact.metadata.observation_id)&candidate_evidence_id=$($ids.face_pose)&$scopeQuery"
$missingCandidateURL = "$recordsURL/images/similar?authorization=explicit_case_evidence_scope&query_image_observation_id=$($imageArtifact.metadata.observation_id)&$scopeQuery"
$crossTenantURL = "$recordsURL/evidence/$($ids.plate)/content?tenant_id=other&collection_id=$CollectionId&case_id=$CollectionId"
$wrongCaseHeaders = $headers.Clone(); $wrongCaseHeaders['X-Forensic-Case-ID'] = 'wrong-case'
$wrongCaseURL = "$recordsURL/evidence/$($ids.plate)/content?tenant_id=default&collection_id=$CollectionId&case_id=wrong-case"
$security = [ordered]@{
  face_missing_explicit_authorization = Invoke-Status $missingAuthorizationURL
  image_missing_candidate_scope = Invoke-Status $missingCandidateURL
  cross_tenant_query_mismatch = Invoke-Status $crossTenantURL
  wrong_case = Invoke-Status $wrongCaseURL $wrongCaseHeaders
  missing_evidence = Invoke-Status "$recordsURL/evidence/00000000-0000-0000-0000-000000000000/content?tenant_id=default&collection_id=$CollectionId&case_id=$CollectionId"
}

$kb = [ordered]@{}
foreach ($term in @('NEXUS-DEMO-2026','03001234567','case notes')) {
  $body = @{ query = $term; max_results = 5 } | ConvertTo-Json -Compress
  $result = Invoke-RestMethod -Method Post -TimeoutSec 30 -ContentType 'application/json' -Body $body -Uri "$localAIURL/api/agents/collections/$CollectionId/search"
  $kb[$term] = [ordered]@{ count = @($result.results).Count; top_source = [string]$result.results[0].metadata.file_name; top_content = [string]$result.results[0].content }
}
$retrievalBody = @{
  collection_id = $CollectionId; case_id = $CollectionId
  query = 'Find records and source passages mentioning NEXUS-DEMO-2026'
  target = 'NEXUS-DEMO-2026'; limit = 10; max_kb_results = 5
} | ConvertTo-Json -Compress
$retrieval = Invoke-RestMethod -Method Post -TimeoutSec 135 -ContentType 'application/json' -Body $retrievalBody -Uri "$localAIURL/api/records/forensic/query"

$history = Invoke-RestMethod -TimeoutSec 30 -Uri "$localAIURL/api/agents/Forensic_Records_Analyst/history?case_id=$CollectionId&collection_id=$CollectionId&limit=10"
$historySummary = @($history.items | ForEach-Object {
  [ordered]@{
    analysis_id = [string]$_.analysis_id
    status = [string]$_.status
    question = [string]$_.question
    citation_count = @($_.answer_metadata.presentation.citations).Count
    answer_preview = ([string]$_.answer).Substring(0, [Math]::Min(300, ([string]$_.answer).Length))
  }
})

$structured = Get-Content -Raw (Join-Path $repoRoot 'reports/unified-multimodal-product-acceptance-20260823/structured-65-operation-matrix.json') | ConvertFrom-Json
$evidenceIdentities = @($inventory.items | ForEach-Object {
  $detail = $details[[string]$_.evidence_id]
  $versionId = [string]$detail.derived_artifacts[0].version_id
  if (-not $versionId) { $versionId = [string]$detail.records_preview[0].version_id }
  if (-not $versionId) { $versionId = [string]$detail.ingest_jobs[0].metadata.version_id }
  if (-not $versionId) { throw "No current version ID is exposed for evidence $($_.evidence_id)" }
  [ordered]@{
    evidence_id = [string]$_.evidence_id
    version_id = $versionId
    original_filename = [string]$_.original_filename
    sha256 = [string]$_.sha256
    modality = [string]$_.modality
    detected_type = [string]$_.detected_type
    processing_status = [string]$_.processing_status
  }
})
$report = [ordered]@{
  contract_version = 'nexusai.unified-multimodal-product-acceptance/v1'
  recorded_at = (Get-Date).ToUniversalTime().ToString('o')
  scope = @{ tenant_id='default'; collection_id=$CollectionId; case_id=$CollectionId; actor_id=$actor }
  inventory = @{
    evidence_count=[int]$inventory.pagination.evidence_total
    ready_count=@($inventory.items | Where-Object processing_status -eq 'completed').Count
    jobs=$jobCounts
    authoritative_reconciliation=@{ evidence=21; versions=21; jobs=34; active_jobs=0; completed_jobs=33; dead_letter_jobs=1; canonical_records=9274; derived_artifacts=372; kb_assets=19 }
    authoritative_artifact_counts=@{
      'forensics.anpr-observation/v1'=1; 'forensics.audio-roman-urdu-segment/v1'=6; 'forensics.audio-timestamp-segment/v1'=6
      'forensics.document-native-text-passage/v1'=114; 'forensics.face-observation/v1'=5; 'forensics.image-embedding-observation/v1'=9
      'forensics.image-fingerprint/v1'=7; 'forensics.image-observation/v1'=5; 'forensics.image-ocr-observation/v1'=219
    }
    detail_preview_artifact_counts=$artifactCounts
    items=$evidenceIdentities
  }
  structured_operations = @{ total=@($structured.results).Count; passed=@($structured.results | Where-Object outcome -in @('answered','accepted_no_result')).Count; report='structured-65-operation-matrix.json' }
  image_intelligence = @{ face_similarity=$faceResult; siglip_similarity=$imageResult; comparison_same_subject=$compareAPose; comparison_different_subject=$compareAB }
  source_access = $mediaChecks
  security = $security
  knowledge_base = @{ direct_search=$kb; governed_retrieval=$retrieval }
  analyst_history = @{ count=@($history.items).Count; items=$historySummary }
  browser_acceptance = @{
    workspace_selected=$true; home_ready_sources=21; home_available_areas=15; data_catalog_sources=21
    image_preview_and_regions=$true; ask_surface=$true; history_reopen=$true
    audio_preview='browser_reports_unsupported'; video_preview='browser_reports_unsupported'
    observed_ui_inconsistencies=@('Audio detail reports capability unavailable although timestamped Urdu and Roman Urdu artifacts are visible.', 'Audio/video elements fall back to unsupported in the in-app browser while authenticated full and range source endpoints are operational.', 'Earlier immutable processing notes remain visible alongside current readiness and can look like current failures without careful reading.')
  }
  known_limitations = @(
    'Candidate face similarity is not identity.',
    'SigLIP similarity is candidate semantic visual similarity, not evidence identity or fact.',
    'No exact-duplicate or threshold-positive near-duplicate pair exists in the approved 21-source manifest; no positive claim is fabricated.',
    'General OCR and media observations remain review-required.',
    'Spoken plate punctuation/format preservation remains an M2 limitation.',
    'The retained case contains one historical dead-letter job and 33 completed jobs; no latest job is active or failed.'
  )
}

$reportFullPath = Join-Path $repoRoot $ReportPath
$report | ConvertTo-Json -Depth 40 | Set-Content -LiteralPath $reportFullPath -Encoding utf8
$checks = @(
  ($report.inventory.evidence_count -eq 21)
  ($report.inventory.ready_count -eq 21)
  ($report.inventory.jobs.active -eq 0)
  (@($faceResult.results).Count -eq 3)
  (@($imageResult.results).Count -eq 5)
  ($security.face_missing_explicit_authorization.status -eq 403)
  ($security.image_missing_candidate_scope.status -eq 400)
  ($security.cross_tenant_query_mismatch.status -eq 403)
  ($security.wrong_case.status -eq 404)
  ($security.missing_evidence.status -eq 404)
  ($mediaChecks.txt.full.status -eq 200)
  ($mediaChecks.txt.range.status -eq 206)
  ($retrieval.route -contains 'kb_rag')
  ([int]$retrieval.answer.evidence_count -gt 0)
  (@($history.items).Count -ge 4)
)
if ($checks -contains $false) { throw 'One or more unified product acceptance checks failed. See the generated report.' }
$checksumFiles = @(
  'local-acceptance-inputs/NEXUSAI_MULTIMODAL_PRODUCT_ACCEPTANCE_MANIFEST.json'
  'reports/unified-multimodal-product-acceptance-20260823/P1_MEDIA_ROLE_RECOVERY_APPROVAL_MANIFEST.json'
  'reports/unified-multimodal-product-acceptance-20260823/media-role-reprocess-results.json'
  'reports/unified-multimodal-product-acceptance-20260823/media-role-reprocess-verdict.json'
  'reports/unified-multimodal-product-acceptance-20260823/structured-65-operation-matrix.json'
  'reports/unified-multimodal-product-acceptance-20260823/product-acceptance-api.json'
  'docs/demo/nexusai-breadth-multimodal-demo-guide.md'
  'reports/nexusai-unified-multimodal-product-acceptance-20260823.md'
)
$fileChecksums = @($checksumFiles | ForEach-Object {
  $path = Join-Path $repoRoot $_
  if (Test-Path -LiteralPath $path -PathType Leaf) {
    [ordered]@{ path=$_; bytes=(Get-Item -LiteralPath $path).Length; sha256=(Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash.ToLowerInvariant() }
  }
})
$export = [ordered]@{
  contract_version='nexusai.unified-multimodal-export-checksum/v1'
  recorded_at=(Get-Date).ToUniversalTime().ToString('o')
  scope=@{tenant_id='default';collection_id=$CollectionId;case_id=$CollectionId;actor_id=$actor}
  retained_state=@{evidence=21;versions=21;jobs=34;active_jobs=0;completed_jobs=33;dead_letter_jobs=1;canonical_records=9274;derived_artifacts=372;kb_assets=19}
  evidence=$evidenceIdentities
  files=$fileChecksums
  retention='review hold; deletion and cleanup require separate explicit approval'
}
$export | ConvertTo-Json -Depth 12 | Set-Content -LiteralPath (Join-Path $repoRoot 'reports/unified-multimodal-product-acceptance-20260823/export-checksum-manifest.json') -Encoding utf8
Write-Host "UnifiedProductAcceptance=PASS_WITH_RECORDED_LIMITATIONS Report=$ReportPath"
