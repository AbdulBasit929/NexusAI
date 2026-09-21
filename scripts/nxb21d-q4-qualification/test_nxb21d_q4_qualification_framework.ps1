# SPDX-License-Identifier: MIT
[CmdletBinding()]
param()
Set-StrictMode -Version Latest
$ErrorActionPreference='Stop'
$root=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
. (Join-Path $root 'scripts\nxb21d-offline\common.ps1')
. (Join-Path $root 'scripts\nxb21d-development\byte_safe_transport.ps1')
. (Join-Path $PSScriptRoot 'qualification_guards.ps1')
$passed=0
function Assert-True([bool]$Condition,[string]$Name){if(-not$Condition){throw "ASSERTION_FAILED:$Name"};$script:passed++}
function Assert-Throws([scriptblock]$Action,[string]$Pattern,[string]$Name){$message='';try{&$Action}catch{$message=$_.Exception.Message};Assert-True ($message-like$Pattern) $Name}

$config=Read-NxDStrictUtf8Text (Join-Path $PSScriptRoot 'nxb21d_q4_qualification_config.json')|ConvertFrom-Json
$registryPath=Resolve-NxDPath $config.consumed_holdout_registry
$registryHash=Get-NxDHash $registryPath
$registrySidecar=((Get-Content -LiteralPath ($registryPath+'.sha256') -Raw).Trim()-split'\s+')[0]
Assert-True ($registryHash-ceq$registrySidecar) 'consumed-registry-sidecar'
$registry=Read-NxDStrictUtf8Text $registryPath|ConvertFrom-Json
$entry=@($registry.entries|Where-Object holdout_sha256 -CEQ ([string]$config.corpus.sha256))
Assert-True ($entry.Count-eq1) 'consumed-hash-registered-once'
Assert-True (-not[bool]$entry[0].reusable-and-not[bool]$entry[0].remaining_cases_may_run-and-not[bool]$entry[0].copied_bytes_may_run) 'consumed-hash-permanently-blocked'

$incidentPath=Resolve-NxDPath $entry[0].incident_receipt
Assert-True ((Get-NxDHash $incidentPath)-ceq[string]$entry[0].incident_receipt_sha256) 'incident-receipt-integrity'
Assert-True ((Get-NxDHash $incidentPath)-ceq(((Get-Content -LiteralPath ($incidentPath+'.sha256') -Raw).Trim()-split'\s+')[0])) 'incident-sidecar'
$incident=Read-NxDStrictUtf8Text $incidentPath|ConvertFrom-Json
Assert-True ([string]$incident.qualification_state-ceq'invalid_incomplete_consumed') 'incident-state'
Assert-True ([int]$incident.run.cases_completed-eq2-and[int]$incident.run.completed_model_calls-eq2-and[int]$incident.run.requests_dispatched-eq3) 'incident-counts'
Assert-Throws {Assert-NxQ4QualificationReceipt $incident} '*Q4 qualification gate' 'activation-rejects-incident'

$copy=Join-Path ([IO.Path]::GetTempPath()) ('nxb21d-consumed-copy-'+[guid]::NewGuid().ToString('N')+'.json')
try{
    Copy-Item -LiteralPath (Resolve-NxDPath $config.corpus.path) -Destination $copy
    Assert-True ((Get-NxDHash $copy)-ceq[string]$config.corpus.sha256) 'copied-holdout-same-hash'
    Assert-Throws {Assert-NxQ4HoldoutUnconsumed $copy $registryPath} 'HOLDOUT_ALREADY_CONSUMED_INVALID_INCOMPLETE:*' 'copied-hash-refused'
}finally{if(Test-Path -LiteralPath $copy){Remove-Item -LiteralPath $copy -Force}}
Assert-Throws {Assert-NxQ4HoldoutUnconsumed (Resolve-NxDPath $config.corpus.path) $registryPath} 'HOLDOUT_ALREADY_CONSUMED_INVALID_INCOMPLETE:*' 'configured-holdout-refused'

$valid=[pscustomobject]@{schema_version='nexusai.nxb21d-q4-final-qualification-receipt/v1';qualification_state='complete_pass';holdout_consumed=$true;cases_completed=168;DExitDecision='PASS';critical=[pscustomobject]@{percent=100};suitability='SUITABLE';runtime_postcheck='PASS';model_unloaded=$true}
Assert-NxQ4QualificationReceipt $valid;$passed++

$runner=Read-NxDStrictUtf8Text (Join-Path $PSScriptRoot 'run_nxb21d_q4_final_qualification.ps1')
Assert-True ($runner.Contains('Assert-NxQ4HoldoutUnconsumed')) 'runner-hash-guard'
Assert-True ($runner.IndexOf('Assert-NxQ4HoldoutUnconsumed')-lt$runner.IndexOf('$runRoot=')) 'guard-before-run-directory'
Assert-True ($runner.Contains("qualification_state='complete_pass'")) 'pass-receipt-explicit-state'
foreach($forbidden in @('docker compose down','docker system prune')){Assert-True (-not$runner.Contains($forbidden)) "runner-forbidden:$forbidden"}

$activation=Read-NxDStrictUtf8Text (Join-Path $PSScriptRoot 'run_nxb21_b1c_q4_activation.ps1')
Assert-True ($activation.Contains('Assert-NxQ4QualificationReceipt $receipt')) 'activation-uses-receipt-guard'
Assert-True (-not(Test-Path -LiteralPath (Resolve-NxDPath $config.public_receipt))) 'no-normal-pass-receipt'
foreach($forbidden in @('docker system prune','docker image prune','docker volume prune','compose down','--remove-orphans')){Assert-True (-not$activation.Contains($forbidden)) "activation-forbidden:$forbidden"}

foreach($script in @('run_nxb21d_q4_final_qualification.ps1','run_nxb21_b1c_q4_activation.ps1','rollback_nxb21_b1c_q4_activation.ps1','qualification_guards.ps1')){
    $tokens=$null;$errors=$null
    [Management.Automation.Language.Parser]::ParseFile((Join-Path $PSScriptRoot $script),[ref]$tokens,[ref]$errors)|Out-Null
    Assert-True (@($errors).Count-eq0) "syntax:$script"
}

Write-Host "NXB21D_Q4_QUALIFICATION_FRAMEWORK=PASS assertions=$passed"
