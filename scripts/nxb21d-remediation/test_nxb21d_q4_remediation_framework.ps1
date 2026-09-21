# SPDX-License-Identifier: MIT
[CmdletBinding()]
param()
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$root = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
. (Join-Path $root 'scripts\nxb21d-offline\common.ps1')
. (Join-Path $root 'scripts\nxb21d-development\byte_safe_transport.ps1')
. (Join-Path $PSScriptRoot 'exact_identifier_authority.ps1')

$passed = 0
function Assert-True([bool]$Condition, [string]$Name) {
    if (-not $Condition) { throw "ASSERTION_FAILED:$Name" }
    $script:passed++
}

function Assert-Resolved([string]$Question, [string]$Identifier, [string]$ModelLiteral, [string[]]$ModelEntities, [bool]$ExpectedReconciled) {
    $result = Resolve-NxDSingleExactIdentifier $Question 'image.plate' 'EXACT_VALUE' $ModelLiteral $ModelEntities
    Assert-True ([bool]$result.applicable) "applicable:$Identifier"
    Assert-True ([string]$result.resolution_status -ceq 'RESOLVED_DETERMINISTIC_AUTHORITY') "resolved:$Identifier"
    Assert-True ([string]$result.final_literal_text -ceq $Identifier) "literal:$Identifier"
    Assert-True (@($result.final_entities).Count -eq 1 -and [string]$result.final_entities[0] -ceq $Identifier) "atomic:$Identifier"
    Assert-True ([bool]$result.reconciled -eq $ExpectedReconciled) "reconciled:$Identifier"
}

Assert-Resolved 'Find plate ISB-5907 in the selected evidence.' 'ISB-5907' 'ISB-5907' @('ISB-590', '5907') $true
Assert-Resolved 'Find ABC-123 in the selected evidence.' 'ABC-123' 'ABC-123' @('ABC-123') $false
Assert-Resolved 'Find ABC-1234 in the selected evidence.' 'ABC-1234' 'ABC-123' @('ABC-1234') $true
Assert-Resolved 'Find LM-305K in the selected evidence.' 'LM-305K' 'LM305K' @('LM-305K') $true
Assert-Resolved 'Find UV-8801 in the selected evidence.' 'UV-8801' 'UV-8801' @('UV-880') $true

$invented = Resolve-NxDSingleExactIdentifier 'Find PQ-4829 in selected evidence.' 'image.plate' 'EXACT_VALUE' 'PQ-4829' @('PQ-4829', 'ZZ-999')
Assert-True ([bool]$invented.reconciled) 'invented-second-reconciled'
Assert-True (@($invented.mismatch_reasons) -contains 'entity_count_not_one') 'invented-second-recorded'

$multiple = Resolve-NxDSingleExactIdentifier 'Compare ABC-123 with XYZ-987.' 'image.plate' 'EXACT_VALUE' 'ABC-123' @('ABC-123')
Assert-True ([string]$multiple.resolution_status -ceq 'REJECTED_REQUIRES_CLARIFICATION') 'multiple-rejected'
Assert-True ([string]$multiple.reason -ceq 'MULTIPLE_IDENTIFIER_SPANS_UNSUPPORTED') 'multiple-reason'
$none = Resolve-NxDSingleExactIdentifier 'Find the vehicle mentioned near the gate.' 'image.plate' 'EXACT_VALUE' '' @()
Assert-True ([string]$none.reason -ceq 'NO_BOUNDED_IDENTIFIER_SPAN') 'ordinary-prose-rejected'
$phrase = Resolve-NxDSingleExactIdentifier 'Find exact phrase north gate reopened.' 'document.phrase' 'EXACT_PHRASE' 'north gate reopened' @('north gate reopened')
Assert-True (-not [bool]$phrase.applicable -and [string]$phrase.resolution_status -ceq 'NOT_APPLICABLE') 'phrase-unaffected'

$corpusPath = Join-Path $PSScriptRoot 'nxb21d-q4-remediation-16case-development-corpus-v1.json'
$corpus = Read-NxDStrictUtf8Text $corpusPath | ConvertFrom-Json
foreach ($case in @($corpus.cases | Where-Object { $_.expected.query_capability_id -ceq 'image.plate' })) {
    $identifier = [string]$case.expected.literal_text
    Assert-Resolved ([string]$case.question) $identifier $identifier @($identifier) $false
    Assert-True ([string]$case.expected.scope -ceq 'selected') "plate-selected-only:$($case.id)"
}

$tokens = $null
$parseErrors = $null
[Management.Automation.Language.Parser]::ParseFile((Join-Path $PSScriptRoot 'run_nxb21d_q4_remediation_16case_development.ps1'), [ref]$tokens, [ref]$parseErrors) | Out-Null
Assert-True (@($parseErrors).Count -eq 0) 'runner-powershell-syntax'
[Management.Automation.Language.Parser]::ParseFile((Join-Path $PSScriptRoot 'exact_identifier_authority.ps1'), [ref]$tokens, [ref]$parseErrors) | Out-Null
Assert-True (@($parseErrors).Count -eq 0) 'authority-powershell-syntax'

$runner = [IO.File]::ReadAllText((Join-Path $PSScriptRoot 'run_nxb21d_q4_remediation_16case_development.ps1'))
Assert-True (-not $runner.Contains('Invoke-WebRequest')) 'no-invoke-webrequest'
Assert-True (-not $runner.Contains('run_nxb21d_qwen_evaluation')) 'no-consumed-holdout-runner'
Assert-True ($runner.Contains('model_proposal_result') -and $runner.Contains('final_typed_plan_result')) 'dual-result-receipt'
Assert-True ($runner.Contains('CANDIDATE_REFREEZE=PASS')) 'refreeze-marker'

$configPath = Join-Path $PSScriptRoot 'nxb21d_q4_remediation_16case_config.json'
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

Write-Host "NXB21D_REMEDIATION_FRAMEWORK_TESTS=PASS assertions=$passed"
