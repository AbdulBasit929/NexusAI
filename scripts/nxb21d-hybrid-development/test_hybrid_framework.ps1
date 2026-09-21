# SPDX-License-Identifier: MIT
[CmdletBinding()]
param()
$ErrorActionPreference='Stop'
$root=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
. (Join-Path $root 'scripts/nxb21d-offline/common.ps1')
. (Join-Path $root 'scripts/nxb21d-development/byte_safe_transport.ps1')
$runner=Join-Path $PSScriptRoot 'run_nxb21d_q4_hybrid_32case_development.ps1'
foreach($file in Get-ChildItem -LiteralPath $PSScriptRoot -Filter '*.ps1'){
    $tokens=$null;$errors=$null
    $null=[Management.Automation.Language.Parser]::ParseFile($file.FullName,[ref]$tokens,[ref]$errors)
    if($errors.Count){throw "SYNTAX_FAILED:$($file.Name):$errors"}
}
$tokens=$null;$errors=$null
$ast=[Management.Automation.Language.Parser]::ParseFile($runner,[ref]$tokens,[ref]$errors)
foreach($name in @('Assert-Equal','Assert-Framework','Get-DevelopmentFreezeIdentity')){
    $fn=$ast.Find({param($node) $node -is [Management.Automation.Language.FunctionDefinitionAst] -and $node.Name -eq $name},$true)
    if(-not $fn){throw "FUNCTION_MISSING:$name"}
    Invoke-Expression $fn.Extent.Text
}
$cfg=Get-Content -LiteralPath (Join-Path $PSScriptRoot 'hybrid_config.json') -Raw|ConvertFrom-Json
Assert-Framework $cfg
Assert-Equal 'identity' (Get-DevelopmentFreezeIdentity $cfg) $cfg.development_freeze.identity_sha256
$cfg.framework_files.'api/forensic_records/query.go'='tampered'
$rejected=$false
try{Assert-Framework $cfg}catch{if($_.Exception.Message -like 'INTEGRITY_MISMATCH:*'){$rejected=$true}else{throw}}
if(-not $rejected){throw 'SOURCE_TAMPER_NOT_REJECTED'}
if((Get-DevelopmentFreezeIdentity $cfg) -eq $cfg.development_freeze.identity_sha256){throw 'IDENTITY_TAMPER_NOT_REJECTED'}
& powershell -NoProfile -ExecutionPolicy Bypass -File $runner -ValidateOnly
if($LASTEXITCODE -ne 0){throw "VALIDATE_ONLY_FAILED:$LASTEXITCODE"}
Write-Host 'HYBRID_FRAMEWORK_TEST=PASS syntax=true source_tamper_rejected=true identity_tamper_rejected=true live_inference=false'
