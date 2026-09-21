# SPDX-License-Identifier: MIT
[CmdletBinding()]
param()
Set-StrictMode -Version Latest
$ErrorActionPreference='Stop'
$root=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
. (Join-Path $root 'scripts\nxb21d-offline\common.ps1')
. (Join-Path $root 'scripts\nxb21d-development\byte_safe_transport.ps1')
. (Join-Path $root 'scripts\nxb21d-q4-qualification\qualification_guards.ps1')
$passed=0
function Assert-True([bool]$Condition,[string]$Name){if(-not$Condition){throw "ASSERTION_FAILED:$Name"};$script:passed++}
function Get-Identity($Config){$lines=@("freeze_id=$($Config.development_freeze.freeze_id)","artifact_sha256=$($Config.model.artifact_sha256)","q4_profile_sha256=$($Config.model.profile_sha256)","q8_profile_sha256=$($Config.model.q8_profile_sha256)","query_language_sha256=$($Config.framework_files.'api/forensic_records/query_language_assistance.go')","exact_authority_sha256=$($Config.framework_files.'scripts/nxb21d-remediation/exact_identifier_authority.ps1')","phrase_authority_sha256=$($Config.framework_files.'scripts/nxb21d-phrase-remediation/phrase_literal_authority.ps1')","scope_authority_sha256=$($Config.framework_files.'scripts/nxb21d-scope-remediation/explicit_scope_authority.ps1')","contracts_sha256=$($Config.framework_files.'api/forensic_records/query_intelligence_contracts.go')","capability_refs_sha256=$($Config.framework_files.'api/forensic_records/contracts/query-capability-references-v1.json')","transport_sha256=$($Config.framework_files.'scripts/nxb21d-development/byte_safe_transport.ps1')","corpus_sha256=$($Config.corpus.sha256)","evaluator_sha256=$($Config.evaluator.sha256)","runner_sha256=$($Config.runner.sha256)","completion_budget=$($Config.development_freeze.completion_budget)");$bytes=[Text.UTF8Encoding]::new($false).GetBytes(($lines-join"`n")+"`n");$sha=[Security.Cryptography.SHA256]::Create();try{return ([BitConverter]::ToString($sha.ComputeHash($bytes))).Replace('-','').ToLowerInvariant()}finally{$sha.Dispose()}}

$cfg=Read-NxDStrictUtf8Text (Join-Path $PSScriptRoot 'nxb21d_q4_interface_remediation_v2_config.json')|ConvertFrom-Json
foreach($p in $cfg.framework_files.PSObject.Properties){Assert-True ((Get-NxDHash (Resolve-NxDPath $p.Name))-ceq[string]$p.Value) "framework-hash:$($p.Name)"}
Assert-True ((Get-Identity $cfg)-ceq[string]$cfg.development_freeze.identity_sha256) 'freeze-identity'
Assert-True ([int]$cfg.development_freeze.completion_budget-eq512) 'completion-budget-512'
Assert-True ([string]$cfg.development_freeze.state-ceq'FROZEN_NOT_EXECUTED') 'development-not-executed'
Assert-NxQ4HoldoutUnconsumed (Resolve-NxDPath $cfg.corpus.path) (Resolve-NxDPath $cfg.consumed_holdout_registry);$passed++

$validation=& python (Resolve-NxDPath $cfg.corpus.validator) --repo $root --corpus (Resolve-NxDPath $cfg.corpus.path)
Assert-True ($LASTEXITCODE-eq0) 'corpus-validator-exit'
$validated=($validation-join[Environment]::NewLine)|ConvertFrom-Json
Assert-True ([string]$validated.status-ceq'PASS') 'corpus-validator-status'
Assert-True ([int]$validated.cases-eq24-and[int]$validated.model_cases-eq20-and[int]$validated.critical_cases-eq4) 'corpus-composition'
foreach($language in @('en','ur','roman_ur','mixed')){Assert-True ([int]$validated.languages.$language-eq6) "language:$language"}
Assert-True ([string]$validated.sha256-ceq[string]$cfg.corpus.sha256) 'corpus-reproducible-hash'
Assert-True (@($validated.failures).Count-eq0) 'freshness-no-overlap'

$encoded=ConvertTo-NxDJsonUtf8NoBOMBytes ([ordered]@{question='اس منتخب ماخذ میں عبارت تلاش کریں';literal='زمردی رسید'})
Assert-True (-not$encoded.Utf8BOM-and-not(Test-NxDUtf8BOM $encoded.Bytes)) 'byte-safe-request-no-bom'
Assert-True ((ConvertFrom-NxDStrictUtf8Bytes $encoded.Bytes)-like'*زمردی رسید*') 'urdu-byte-round-trip'
$invalid=ConvertFrom-NxDPlannerResponseBytes ([byte[]](0xC3,0x28))
Assert-True (-not$invalid.strict_utf8_decode-and[string]$invalid.error_classification-ceq'INVALID_UTF8_RESPONSE') 'invalid-utf8-rejected'

$evaluator=Read-NxDStrictUtf8Text (Resolve-NxDPath $cfg.evaluator.path)
foreach($required in @('resolveWithLanguageAssistance','dynamicPlannerMaxCompletionTokens','FinishReason','CompletionUtilizationPercent','InnerPlannerJSONBytes','inner-model-proposal.json','case-result-v2.json','TRUNCATED_LENGTH','VALIDATION_REJECTED','production_contract_validated','four_consecutive_model_failures','above_448','holdout_consumed": false')){Assert-True ($evaluator.Contains($required)) "evaluator:$required"}
Assert-True (-not$evaluator.Contains('nxb21d_planner_proposal')) 'no-six-field-substitute'

$runner=Read-NxDStrictUtf8Text (Resolve-NxDPath $cfg.runner.path)
foreach($required in @('Assert-Framework','Get-DevelopmentFreezeIdentity','Assert-NxQ4HoldoutUnconsumed','minimum_before_load_gib','minimum_loaded_gib','Invoke-NxDSafeLinuxCleanCache','Assert-Baseline','Invoke-NxDUnload','DEVELOPMENT_RECEIPT_SHA256')){Assert-True ($runner.Contains($required)) "runner:$required"}
foreach($forbidden in @('docker compose down','docker system prune','docker image prune','docker volume prune','--remove-orphans','run_nxb21_b1c_q4_activation')){Assert-True (-not$runner.Contains($forbidden)) "runner-forbidden:$forbidden"}
foreach($script in @($cfg.runner.path,'scripts/nxb21d-q4-qualification/qualification_guards.ps1')){$tokens=$null;$errors=$null;[Management.Automation.Language.Parser]::ParseFile((Resolve-NxDPath $script),[ref]$tokens,[ref]$errors)|Out-Null;Assert-True (@($errors).Count-eq0) "syntax:$script"}

$freezePath=Resolve-NxDPath 'reports/nxb21/d-q4-production-interface-schema-bound-development-freeze-v2.json'
$freeze=Read-NxDStrictUtf8Text $freezePath|ConvertFrom-Json
Assert-True ((Get-NxDHash $freezePath)-ceq(((Get-Content -LiteralPath ($freezePath+'.sha256') -Raw).Trim()-split'\s+')[0])) 'freeze-sidecar'
Assert-True ([string]$freeze.freeze_id-ceq[string]$cfg.development_freeze.freeze_id) 'freeze-id'
Assert-True ([string]$freeze.identity_sha256-ceq[string]$cfg.development_freeze.identity_sha256) 'freeze-report-identity'
Assert-True ([string]$freeze.program_state.development_diagnostic_24_v2-ceq'FROZEN_NOT_EXECUTED') 'freeze-state'
Assert-True ([string]$freeze.retired_qualification_attempt.holdout_sha256-ceq'aaa8517983ea4523e235c4b3de31e5562bfaeada21ef24a7b23065a567d31d13') 'consumed-q4-preserved'

Write-Host "NXB21D_Q4_INTERFACE_REMEDIATION_FRAMEWORK=PASS assertions=$passed"
