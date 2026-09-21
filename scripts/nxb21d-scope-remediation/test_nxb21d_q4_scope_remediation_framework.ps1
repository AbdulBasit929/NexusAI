# SPDX-License-Identifier: MIT
[CmdletBinding()]
param()
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$root = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
. (Join-Path $root 'scripts\nxb21d-offline\common.ps1')
. (Join-Path $root 'scripts\nxb21d-development\byte_safe_transport.ps1')
. (Join-Path $root 'scripts\nxb21d-remediation\exact_identifier_authority.ps1')
. (Join-Path $root 'scripts\nxb21d-phrase-remediation\phrase_literal_authority.ps1')
. (Join-Path $PSScriptRoot 'explicit_scope_authority.ps1')

$passed = 0
function Assert-True([bool]$Condition, [string]$Name) {
    if (-not $Condition) { throw "ASSERTION_FAILED:$Name" }
    $script:passed++
}

$corpusPath = Join-Path $PSScriptRoot 'nxb21d-q4-scope-remediation-16case-development-corpus-v1.json'
$corpus = Read-NxDStrictUtf8Text $corpusPath | ConvertFrom-Json
Assert-True (@($corpus.cases).Count -eq 16) 'corpus-count'
foreach ($case in @($corpus.cases)) {
    $expected = $case.expected
    if ([string]$expected.query_capability_id -ne 'image.plate') {
        $scope = Resolve-NxDExplicitScopeAuthority ([string]$case.question) ([string]$expected.scope)
        Assert-True ([bool]$scope.applicable) "scope-applicable:$($case.id)"
        Assert-True ([string]$scope.resolution_status -ceq 'RESOLVED_DETERMINISTIC_AUTHORITY') "scope-resolved:$($case.id)"
        Assert-True ([string]$scope.source_scope -ceq [string]$expected.scope) "scope-source:$($case.id)"
        Assert-True ([string]$scope.final_scope -ceq [string]$expected.scope) "scope-final:$($case.id)"
        Assert-True (-not [bool]$scope.reconciled) "scope-unchanged:$($case.id)"
        $phrase = Resolve-NxDExplicitPhraseLiteral ([string]$case.question) ([string]$expected.query_capability_id) ([string]$expected.semantic) ([string]$expected.literal_text) $false
        Assert-True ([bool]$phrase.applicable) "phrase-applicable:$($case.id)"
        Assert-True ([string]$phrase.final_literal -ceq [string]$expected.literal_text) "phrase-final:$($case.id)"
    }
}

$observed = Resolve-NxDExplicitScopeAuthority 'tamam transcript files mein "pichla rasta band rakho" search karo' 'selected'
Assert-True ([bool]$observed.reconciled) 'observed-scope-reconciled'
Assert-True ([string]$observed.model_scope -ceq 'selected') 'observed-model-scope-preserved'
Assert-True ([string]$observed.final_scope -ceq 'workspace') 'observed-workspace-wins'
Assert-True ([string]$observed.model_proposal_status -ceq 'FAIL_SCOPE') 'observed-model-failure-preserved'

$quoted = Resolve-NxDExplicitScopeAuthority 'Search this document for "all transcript files".' 'selected'
Assert-True ([string]$quoted.final_scope -ceq 'selected') 'quoted-scope-words-ignored'
$ambiguous = Resolve-NxDExplicitScopeAuthority 'Search this recording across the whole investigation.' 'selected'
Assert-True ([string]$ambiguous.resolution_status -ceq 'REJECTED_REQUIRES_CLARIFICATION') 'contradictory-scope-rejected'
$implicit = Resolve-NxDExplicitScopeAuthority 'Find the copied phrase.' 'workspace'
Assert-True (-not [bool]$implicit.applicable) 'implicit-scope-not-invented'

$plate = Resolve-NxDSingleExactIdentifier 'Find plate ISB-5907 in this selected image.' 'image.plate' 'EXACT_VALUE' 'ISB-5907' @('ISB-590', '5907')
Assert-True ([string]$plate.final_literal_text -ceq 'ISB-5907') 'plate-literal-regression'
Assert-True (@($plate.final_entities).Count -eq 1 -and [string]$plate.final_entities[0] -ceq 'ISB-5907') 'plate-atomic-regression'
$phraseControl = Resolve-NxDExplicitPhraseLiteral 'تمام دستاویزات میں "فہرست مکمل کر دی" تلاش کریں۔' 'document.phrase' 'PHRASE_CONTAINS' '' $true
Assert-True ([string]$phraseControl.final_literal -ceq 'فہرست مکمل کر دی') 'phrase-authority-regression'
Assert-True (-not [bool]$phraseControl.final_clarification) 'phrase-clarification-regression'

$tokens = $null
$parseErrors = $null
foreach ($path in @('run_nxb21d_q4_scope_remediation_16case_development.ps1', 'explicit_scope_authority.ps1')) {
    [Management.Automation.Language.Parser]::ParseFile((Join-Path $PSScriptRoot $path), [ref]$tokens, [ref]$parseErrors) | Out-Null
    Assert-True (@($parseErrors).Count -eq 0) "powershell-syntax:$path"
}
$runner = [IO.File]::ReadAllText((Join-Path $PSScriptRoot 'run_nxb21d_q4_scope_remediation_16case_development.ps1'))
Assert-True (-not $runner.Contains('Invoke-WebRequest')) 'no-invoke-webrequest'
Assert-True (-not $runner.Contains('run_nxb21d_qwen_evaluation')) 'no-consumed-holdout-runner'
Assert-True ($runner.Contains('model_proposal_result') -and $runner.Contains('final_typed_plan_result')) 'dual-result-receipt'
Assert-True ($runner.Contains('explicit_scope_authority')) 'scope-audit-receipt'
Assert-True ($runner.Contains('SCOPE_CANDIDATE_REFREEZE=PASS')) 'refreeze-marker'
Assert-True ($runner.Contains('Wait-NxDContainerHealthy')) 'bounded-container-health-wait'
Assert-True ($runner.Contains('LOCALAI_CONTAINER_HEALTH_TIMEOUT')) 'container-health-timeout-marker'
Assert-True ($runner.Contains('Wait-DevelopmentRAM')) 'bounded-preload-ram-wait'
Assert-True ($runner.Contains('Invoke-NxDSafeLinuxCleanCache')) 'safe-clean-cache-recovery'
Assert-True ($runner.Contains('RAM_RECLAIM_WAIT')) 'ram-reclaim-wait-marker'
Assert-True ($runner.Contains('Get-NxDResumeState')) 'receipt-verified-resume'
Assert-True ($runner.Contains('RESUME_INTEGRITY_EVIDENCE_HASH')) 'resume-evidence-hash-gate'
Assert-True ($runner.Contains('DEVELOPMENT_RESUME=PASS')) 'resume-pass-marker'
Assert-True ($runner.Contains('preserved=True')) 'completed-case-skip-marker'
Assert-True ($runner.Contains('Wait-DevelopmentRAM $required ([string]$case.id) 180')) 'per-case-ram-wait'
Assert-True (-not $runner.Contains("-File -Recurse")) 'resume-does-not-descend-into-case-evidence'
Assert-True ($runner.Contains("-like 'RESUME_*'")) 'resume-error-classification'

$definitionEnd = $runner.IndexOf('$exitCode = 21', [StringComparison]::Ordinal)
Assert-True ($definitionEnd -gt 0) 'runner-function-boundary'
$definitionSource = $runner.Substring(0, $definitionEnd).Replace('$PSScriptRoot', "'$($PSScriptRoot.Replace("'", "''"))'")
Invoke-Expression $definitionSource
$resumeProbe = Get-NxDResumeState `
    (Resolve-NxDPath 'local-acceptance-models/nxb21-d/challengers/qwen3-4b-instruct-2507-q4km/localai-dev-results/16case-scope-remediation') `
    $corpus `
    (Read-NxDStrictUtf8Text (Join-Path $PSScriptRoot 'nxb21d_q4_scope_remediation_16case_config.json') | ConvertFrom-Json) `
    (Join-Path $env:TEMP 'nxb21d-resume-probe-not-a-run')
Assert-True (@($resumeProbe.results).Count -ge 1) 'resume-discovers-consumed-prefix'
Assert-True ([string]$resumeProbe.results[0].case_id -ceq 'scope16-en-tr-workspace-001') 'resume-prefix-first-case'
Assert-True ([string]$resumeProbe.results[0].raw_request_sha256 -ceq 'eecaf4ca61726e964d4665eca8e44f73fab0ffdfcfce1a1787c9e90f932147cb') 'resume-request-evidence-hash'
Assert-True ([string]$resumeProbe.results[0].raw_response_sha256 -ceq '8d0c0c83056016f3192d7215770dd88a0f5a5e47b5b27146f07e0f4656c50a74') 'resume-response-evidence-hash'

$configPath = Join-Path $PSScriptRoot 'nxb21d_q4_scope_remediation_16case_config.json'
if (Test-Path -LiteralPath $configPath) {
    $config = Read-NxDStrictUtf8Text $configPath | ConvertFrom-Json
    foreach ($property in $config.framework_files.PSObject.Properties) {
        Assert-True ((Get-NxDHash (Resolve-NxDPath $property.Name)) -ceq [string]$property.Value) "framework-hash:$($property.Name)"
    }
    $identityLines = @(
        "candidate_id=$($config.candidate.candidate_id)",
        "artifact_sha256=$($config.candidate.artifact_sha256)",
        "profile_sha256=$($config.candidate.profile_sha256)",
        "q8_profile_sha256=$($config.candidate.q8_profile_sha256)",
        "prompt_sha256=$($config.candidate.prompt_sha256)",
        "schema_sha256=$($config.candidate.schema_sha256)",
        "implementation_sha256=$($config.candidate.implementation_sha256)",
        "bridge_sha256=$($config.candidate.bridge_sha256)",
        "authority_sha256=$($config.candidate.authority_sha256)",
        "phrase_authority_sha256=$($config.candidate.phrase_authority_sha256)",
        "shared_scope_sha256=$($config.candidate.shared_scope_sha256)",
        "api_scope_sha256=$($config.candidate.api_scope_sha256)",
        "agent_scope_sha256=$($config.candidate.agent_scope_sha256)",
        "scope_authority_sha256=$($config.candidate.scope_authority_sha256)",
        "transport_sha256=$($config.candidate.transport_sha256)",
        "corpus_sha256=$($config.corpus.sha256)",
        "request_temperature=$($config.candidate.request_temperature)",
        "effective_context=$($config.candidate.effective_context)"
    )
    $bytes = [Text.UTF8Encoding]::new($false).GetBytes(($identityLines -join "`n") + "`n")
    $sha = [Security.Cryptography.SHA256]::Create()
    try { $identity = ([BitConverter]::ToString($sha.ComputeHash($bytes))).Replace('-', '').ToLowerInvariant() }
    finally { $sha.Dispose() }
    Assert-True ($identity -ceq [string]$config.candidate.identity_sha256) 'candidate-identity'
}

Write-Host "NXB21D_SCOPE_REMEDIATION_FRAMEWORK_TESTS=PASS assertions=$passed"
