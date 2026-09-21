$ErrorActionPreference = 'Stop'

$RepoRoot = 'C:\Users\sheik\Workspace\Office\Projects\NexusAI'
$ContextDir = Join-Path $RepoRoot 'reports\nxb21\investigation-workspace-conversation-ui-final-build-context-20260908\context'
$ManifestPath = Join-Path $RepoRoot 'reports\nxb21\investigation-workspace-conversation-ui-final-deployment-manifest-20260908.json'
$OwnerPaths = @(
    'core/http/react-ui/e2e/analyst-portal.spec.js',
    'core/http/react-ui/src/analyst/AnalystPortal.css',
    'core/http/react-ui/src/analyst/AnalystPortalLayout.jsx',
    'core/http/react-ui/src/pages/AgentChat.jsx'
)

function Get-OwnedFile {
    param([Parameter(Mandatory=$true)][string]$Root, [Parameter(Mandatory=$true)][string]$FullPath)
    $item = Get-Item -LiteralPath $FullPath
    return [ordered]@{
        path = $item.FullName.Substring($Root.Length + 1).Replace('\', '/')
        bytes = $item.Length
        sha256 = (Get-FileHash -Algorithm SHA256 -LiteralPath $item.FullName).Hash.ToLowerInvariant()
    }
}

if (-not (Test-Path -LiteralPath $ContextDir -PathType Container)) {
    throw "Build context is unavailable: $ContextDir"
}

$contextFiles = @(Get-ChildItem -LiteralPath $ContextDir -Recurse -File | Sort-Object FullName | ForEach-Object {
    Get-OwnedFile -Root $ContextDir -FullPath $_.FullName
})
$ownerFiles = @($OwnerPaths | ForEach-Object {
    Get-OwnedFile -Root $RepoRoot -FullPath (Join-Path $RepoRoot $_)
})
$digestInput = ($ownerFiles | ForEach-Object { "$($_.path)|$($_.bytes)|$($_.sha256)" }) -join "`n"
$digestBytes = [Text.Encoding]::UTF8.GetBytes($digestInput)
$sha = [Security.Cryptography.SHA256]::Create()
try {
    $sourceDigest = (($sha.ComputeHash($digestBytes) | ForEach-Object { $_.ToString('x2') }) -join '')
}
finally {
    $sha.Dispose()
}

$manifest = [ordered]@{
    schema_version = 'nexusai.investigation-workspace-conversation-ui-final-deployment-manifest/v1'
    created_at_utc = (Get-Date).ToUniversalTime().ToString('o')
    status = 'SOURCE_VALIDATED_ACTIVATION_READY'
    git_branch = 'codex/forensic-hybrid-checkpoint-20260723'
    git_head = '40717b83510c08db25dc26b9d6674bf46db363ac'
    canonical_route = '/analyst'
    primary_path = 'Add data -> Ask -> Answer -> Evidence'
    architecture = 'opaque accepted live API image plus sealed UI-only conversational refinement'
    base_image_id = 'sha256:bed2eec364ca8c65151625685f79f5ab7a606a8223af88d1696504751b4a03a1'
    base_compose = 'local-acceptance-models/nxmmr/private-activation-video-result/20260903T063719719Z/activation-compose.json'
    accepted_backend_rebuilt = $false
    required_service = 'api only'
    ram_required_gib = 6
    source_digest_sha256 = $sourceDigest
    source_owner_files = $ownerFiles
    context_files = $contextFiles
    d_unqualified_source_included = $false
    d_activation = 'BLOCKED'
}
$manifest | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath $ManifestPath -Encoding UTF8
$manifestHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $ManifestPath).Hash.ToLowerInvariant()
"$manifestHash  $([IO.Path]::GetFileName($ManifestPath))" | Set-Content -LiteralPath "$ManifestPath.sha256" -Encoding ASCII

Write-Host "CONTEXT_FILES=$($contextFiles.Count)"
Write-Host "SOURCE_DIGEST_SHA256=$sourceDigest"
Write-Host "MANIFEST_SHA256=$manifestHash"
