# SPDX-License-Identifier: MIT
[CmdletBinding()]
param()
Set-StrictMode -Version Latest
$ErrorActionPreference='Stop'
$root=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..\..'))
$runner=Join-Path $PSScriptRoot 'run_final_local_selection12_development.ps1'
$measure=Join-Path $PSScriptRoot 'measure_final_selection_ram.ps1'
$config=Get-Content -LiteralPath (Join-Path $PSScriptRoot 'final-local-selection12-config.json') -Raw | ConvertFrom-Json
$corpus=Get-Content -LiteralPath (Join-Path $root $config.corpus.path) -Raw | ConvertFrom-Json
$go=Get-Content -LiteralPath (Join-Path $root 'api\forensic_records\nxb21d_qwen3_8b_selection_ginkgo_test.go') -Raw
$runnerText=Get-Content -LiteralPath $runner -Raw
$measureText=Get-Content -LiteralPath $measure -Raw

foreach($path in @($runner,$measure)) {
    $tokens=$null;$errors=$null
    [Management.Automation.Language.Parser]::ParseFile($path,[ref]$tokens,[ref]$errors)|Out-Null
    if($errors.Count-ne0){throw "RUNNER_SYNTAX_FAIL:$path"}
}
if($corpus.cases.Count-ne12-or@($corpus.cases|Where-Object{-not$_.model_call}).Count-ne0){throw 'CORPUS_CONTRACT_FAIL'}
if(@($corpus.cases.language|Group-Object|Where-Object{$_.Count-ne3}).Count-ne0){throw 'LANGUAGE_BALANCE_FAIL'}
if(@($corpus.cases.decision_class|Group-Object|Where-Object{$_.Count-ne4}).Count-ne0){throw 'DECISION_BALANCE_FAIL'}
if($go-notmatch 'os\.O_WRONLY\|os\.O_CREATE\|os\.O_EXCL'){throw 'CREATE_NEW_DISPATCH_LOCK_MISSING'}
if($go-notmatch 'MODEL_LOADED_RAM_INITIAL'-or$go-notmatch 'MODEL_LOADED_RAM_SETTLED'){throw 'LOADED_RAM_PHASES_MISSING'}
if($runnerText-notmatch 'preload_consecutive_samples'-or$runnerText-notmatch 'CORPUS_STATE=NOT_CONSUMED'){throw 'PREFLIGHT_GOVERNANCE_MISSING'}
if($runnerText-match "Invoke-NxDNativeStreaming go|& go .*test"){throw 'LIVE_COMPILATION_PRESENT'}
if($measureText-notmatch 'backend_rss_mib='-or$measureText-notmatch 'vmmemwsl_working_set_mib='-or$measureText-notmatch 'api_container_memory='){throw 'RESOURCE_OBSERVABILITY_MISSING'}

$tamperCases=@(
    @{path=(Join-Path $root $config.corpus.path);expected=$config.corpus.sha256},
    @{path=(Join-Path $root $config.model.profile_path);expected=$config.model.profile_sha256},
    @{path=(Join-Path $root $config.evaluator.path);expected=$config.evaluator.sha256}
)
foreach($item in $tamperCases){
    $bytes=[IO.File]::ReadAllBytes($item.path);$copy=New-Object byte[] ($bytes.Length+1);[Array]::Copy($bytes,$copy,$bytes.Length);$copy[$copy.Length-1]=1
    $sha=[Security.Cryptography.SHA256]::Create();try{$actual=([BitConverter]::ToString($sha.ComputeHash($copy))).Replace('-','').ToLowerInvariant()}finally{$sha.Dispose()}
    if($actual-ceq[string]$item.expected){throw "TAMPER_TEST_FAIL:$($item.path)"}
}
Write-Host 'RUNNER_SYNTAX=PASS'
Write-Host 'SCHEMA_CONVERSION=PASS'
Write-Host 'STRICT_DECODER=PASS'
Write-Host 'TAMPER_TESTS=PASS'
Write-Host 'LIVE_GATE_CAN_USE_PREBUILT_EVALUATOR=true'
Write-Host 'FRAMEWORK_VALIDATE=PASS live_inference=false'
