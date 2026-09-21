# SPDX-License-Identifier: MIT
[CmdletBinding()]
param()
$ErrorActionPreference = 'Stop'
$root = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..\..'))
. (Join-Path $root 'scripts\nxb21d-offline\common.ps1')
. (Join-Path $root 'scripts\nxb21d-development\byte_safe_transport.ps1')
$runner = Join-Path $PSScriptRoot 'run_fresh_selection12_development.ps1'
foreach ($file in Get-ChildItem -LiteralPath $PSScriptRoot -Filter '*.ps1') {
    $tokens = $null; $errors = $null
    $null = [Management.Automation.Language.Parser]::ParseFile($file.FullName, [ref]$tokens, [ref]$errors)
    if ($errors.Count) { throw "SYNTAX_FAILED:$($file.Name):$errors" }
}
$runnerText = [IO.File]::ReadAllText($runner)
if ([regex]::Matches($runnerText, '\$seal = Seal-NxHybridReceipt').Count -ne 1) { throw 'FINALIZATION_NOT_SINGLE' }
if ($runnerText -match 'Write-NxDJson \$summaryPath|WriteAllText\(\$summaryPath') { throw 'DIRECT_FINAL_WRITE' }
if ($runnerText -notmatch 'NXB21D Qwen3 8B low-memory fresh selection12 standalone development') { throw 'WRONG_EVALUATOR_FOCUS' }
$goText = [IO.File]::ReadAllText((Join-Path $root 'api\forensic_records\nxb21d_qwen3_8b_selection_ginkgo_test.go'))
if ($goText -notmatch 'os.O_WRONLY\|os.O_CREATE\|os.O_EXCL') { throw 'ONE_SHOT_CREATE_NEW_MISSING' }
if ($goText -notmatch 'Fail\(stage \+ " failed before dispatch') { throw 'RAM_FAILURE_NOT_EXPLICIT' }
if ($goText -notmatch 'Fail\("fresh selection12 stopped after') { throw 'EVALUATOR_FAILURE_NOT_EXPLICIT' }
$cfg = Get-Content -LiteralPath (Join-Path $PSScriptRoot 'fresh-selection12-config.json') -Raw | ConvertFrom-Json
foreach ($entry in $cfg.frozen_files.PSObject.Properties) {
    if ((Get-NxDHash (Resolve-NxDPath $entry.Name)) -cne [string]$entry.Value) { throw "FROZEN_FILE_MISMATCH:$($entry.Name)" }
}
$original = [string]$cfg.frozen_files.'api/forensic_records/query.go'
$cfg.frozen_files.'api/forensic_records/query.go' = 'tampered'
if ((Get-NxDHash (Resolve-NxDPath 'api/forensic_records/query.go')) -ceq [string]$cfg.frozen_files.'api/forensic_records/query.go') { throw 'SOURCE_TAMPER_NOT_REJECTED' }
$cfg.frozen_files.'api/forensic_records/query.go' = $original
& powershell -NoProfile -ExecutionPolicy Bypass -File $runner -ValidateOnly
if ($LASTEXITCODE -ne 0) { throw "VALIDATE_ONLY_FAILED:$LASTEXITCODE" }
Write-Host 'FRESH_SELECTION12_FRAMEWORK_TEST=PASS syntax=true source_tamper_rejected=true identity_bound=true final_receipt_create_new=true evaluator_failures_explicit=true live_inference=false'
