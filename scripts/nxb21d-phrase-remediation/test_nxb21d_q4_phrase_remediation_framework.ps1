# SPDX-License-Identifier: MIT
[CmdletBinding()]
param()
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$root = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
. (Join-Path $root 'scripts\nxb21d-offline\common.ps1')
. (Join-Path $root 'scripts\nxb21d-development\byte_safe_transport.ps1')
. (Join-Path $root 'scripts\nxb21d-remediation\exact_identifier_authority.ps1')
. (Join-Path $PSScriptRoot 'phrase_literal_authority.ps1')

$passed = 0
function Assert-True([bool]$Condition, [string]$Name) {
    if (-not $Condition) { throw "ASSERTION_FAILED:$Name" }
    $script:passed++
}

$corpusPath = Join-Path $PSScriptRoot 'nxb21d-q4-phrase-remediation-20case-development-corpus-v1.json'
$corpus = Read-NxDStrictUtf8Text $corpusPath | ConvertFrom-Json
$phraseCases = @($corpus.cases | Where-Object { $_.expected.query_capability_id -ne 'image.plate' })
foreach ($case in $phraseCases) {
    $expected = $case.expected
    $result = Resolve-NxDExplicitPhraseLiteral ([string]$case.question) ([string]$expected.query_capability_id) ([string]$expected.semantic) ([string]$expected.literal_text) $false
    Assert-True ([bool]$result.applicable) "applicable:$($case.id)"
    Assert-True ([string]$result.resolution_status -ceq 'RESOLVED_DETERMINISTIC_AUTHORITY') "resolved:$($case.id)"
    Assert-True ([string]$result.source_literal -ceq [string]$expected.literal_text) "source:$($case.id)"
    Assert-True ([string]$result.final_literal -ceq [string]$expected.literal_text) "final:$($case.id)"
    Assert-True (-not [bool]$result.reconciled) "unchanged:$($case.id)"
    Assert-True (@($result.source_codepoints).Count -eq ([string]$expected.literal_text).Length) "codepoints:$($case.id)"
}

$urdu = $phraseCases | Where-Object language -eq 'urdu-script' | Select-Object -First 1
$source = [string]$urdu.expected.literal_text
$corrupted = $source.Substring(0, 1) + [char]::ConvertFromUtf32(0x0430) + [char]::ConvertFromUtf32(0x043C) + $source.Substring(3)
$homoglyph = Resolve-NxDExplicitPhraseLiteral ([string]$urdu.question) ([string]$urdu.expected.query_capability_id) 'PHRASE_CONTAINS' $corrupted $false
Assert-True ([bool]$homoglyph.reconciled) 'homoglyph-reconciled'
Assert-True ([bool]$homoglyph.script_family_mismatch) 'homoglyph-script-mismatch'
Assert-True (@($homoglyph.mismatch_reasons) -contains 'model_script_family_mismatch') 'homoglyph-recorded'
Assert-True ([string]$homoglyph.final_literal -ceq $source) 'homoglyph-source-wins'
Assert-True ([string]$homoglyph.model_literal -ceq $corrupted) 'homoglyph-model-preserved'

$omitted = Resolve-NxDExplicitPhraseLiteral ([string]$urdu.question) ([string]$urdu.expected.query_capability_id) 'PHRASE_CONTAINS' '' $true
Assert-True ([bool]$omitted.reconciled) 'omitted-reconciled'
Assert-True (@($omitted.mismatch_reasons) -contains 'unjustified_model_clarification') 'clarification-recorded'
Assert-True (-not [bool]$omitted.final_clarification) 'clarification-cleared'
Assert-True ([string]$omitted.final_literal -ceq $source) 'omitted-source-wins'

$multiple = Resolve-NxDExplicitPhraseLiteral 'Search this document for "alpha" or "beta".' 'document.phrase' 'PHRASE_CONTAINS' 'alpha' $false
Assert-True ([string]$multiple.resolution_status -ceq 'REJECTED_REQUIRES_CLARIFICATION') 'multiple-rejected'
$unquoted = Resolve-NxDExplicitPhraseLiteral 'Search this document for alpha beta.' 'document.phrase' 'PHRASE_CONTAINS' 'alpha beta' $false
Assert-True (-not [bool]$unquoted.applicable) 'unquoted-not-captured'
$contradictory = Resolve-NxDExplicitPhraseLiteral 'Search this document for "alpha beta".' 'transcript.phrase' 'PHRASE_CONTAINS' 'alpha beta' $false
Assert-True ([string]$contradictory.reason -ceq 'PHRASE_FAMILY_ABSENT_OR_CONTRADICTORY') 'family-contradiction'
$punctuation = Resolve-NxDExplicitPhraseLiteral 'Search this document for "alpha, beta!".' 'document.phrase' 'PHRASE_CONTAINS' 'wrong' $false
Assert-True ([string]$punctuation.final_literal -ceq 'alpha, beta!') 'quote-punctuation-excluded'

$decomposed = 'cafe' + [char]0x0301
$composed = $decomposed.Normalize([Text.NormalizationForm]::FormC)
Assert-True ((Get-NxDNormalizedPhrase $decomposed) -ceq $composed) 'normalization-contract'
Assert-True ((Get-NxDCodepoints $decomposed).Count -eq 5) 'codepoint-diagnostics'

$plate = Resolve-NxDSingleExactIdentifier 'Find plate ISB-5907 in this selected image.' 'image.plate' 'EXACT_VALUE' 'ISB-5907' @('ISB-590', '5907')
Assert-True ([string]$plate.final_literal_text -ceq 'ISB-5907') 'plate-literal-regression'
Assert-True (@($plate.final_entities).Count -eq 1 -and [string]$plate.final_entities[0] -ceq 'ISB-5907') 'plate-atomic-regression'
Assert-True ([bool]$plate.reconciled) 'plate-audit-regression'

$tokens = $null
$parseErrors = $null
[Management.Automation.Language.Parser]::ParseFile((Join-Path $PSScriptRoot 'run_nxb21d_q4_phrase_remediation_20case_development.ps1'), [ref]$tokens, [ref]$parseErrors) | Out-Null
Assert-True (@($parseErrors).Count -eq 0) 'runner-powershell-syntax'
[Management.Automation.Language.Parser]::ParseFile((Join-Path $PSScriptRoot 'phrase_literal_authority.ps1'), [ref]$tokens, [ref]$parseErrors) | Out-Null
Assert-True (@($parseErrors).Count -eq 0) 'authority-powershell-syntax'

$runner = [IO.File]::ReadAllText((Join-Path $PSScriptRoot 'run_nxb21d_q4_phrase_remediation_20case_development.ps1'))
Assert-True (-not $runner.Contains('Invoke-WebRequest')) 'no-invoke-webrequest'
Assert-True (-not $runner.Contains('run_nxb21d_qwen_evaluation')) 'no-consumed-holdout-runner'
Assert-True ($runner.Contains('model_proposal_result') -and $runner.Contains('final_typed_plan_result')) 'dual-result-receipt'
Assert-True ($runner.Contains('phrase_literal_authority')) 'phrase-audit-receipt'
Assert-True ($runner.Contains('PHRASE_CANDIDATE_REFREEZE=PASS')) 'refreeze-marker'

$configPath = Join-Path $PSScriptRoot 'nxb21d_q4_phrase_remediation_20case_config.json'
if (Test-Path -LiteralPath $configPath) {
    $config = Read-NxDStrictUtf8Text $configPath | ConvertFrom-Json
    foreach ($property in $config.framework_files.PSObject.Properties) {
        $path = Resolve-NxDPath $property.Name
        Assert-True ((Get-NxDHash $path) -ceq [string]$property.Value) "framework-hash:$($property.Name)"
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
        "transport_sha256=$($config.candidate.transport_sha256)",
        "corpus_sha256=$($config.corpus.sha256)",
        "request_temperature=$($config.candidate.request_temperature)",
        "effective_context=$($config.candidate.effective_context)"
    )
    $identityBytes = [Text.UTF8Encoding]::new($false).GetBytes(($identityLines -join "`n") + "`n")
    $identitySha = [Security.Cryptography.SHA256]::Create()
    try { $actualIdentity = ([BitConverter]::ToString($identitySha.ComputeHash($identityBytes))).Replace('-', '').ToLowerInvariant() }
    finally { $identitySha.Dispose() }
    Assert-True ($actualIdentity -ceq [string]$config.candidate.identity_sha256) 'candidate-identity'
}

Write-Host "NXB21D_PHRASE_REMEDIATION_FRAMEWORK_TESTS=PASS assertions=$passed"
